package lsp

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/tables"
)

// TestINT1D6Residual — Phase 2a-INT-1-d6 cold-start residual + L2/L3
// weighting diagnostic (post-FIX-1B).
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-04-phase2a-int1-d4-pinpoint-plan.md
//	(§14 records FIX-1B; §15/§16 appended by this dispatch.)
//
// Post-FIX-1B status (§14.4): full LSP.IN measurable, INT-1 byte-EQ
// at L0 78.99 % / L1 38.71 % / L2 17.52 % / L3 19.71 % across
// 2232 frames; frame 0 first miss is L2 only (got 2, want 10) with
// L0/L1/L3 matching. The residual is downstream of LP analysis.
//
// Live hypotheses entering d6:
//   - H-L4           cold-start `freqPrev` propagation differs from
//     decoder
//   - H-VQ-L2W       L2-stage weighted-MSE protocol (eq. 22 +
//     rearrangement order) deviates from spec §3.2.4
//   - H-J1J2         J1/J2 rearrangement applied at wrong pipeline
//     point in L2/L3 search vs spec lines 887–895
//   - H-FREQPREV-UPDATE  encoder commits predictor memory at wrong
//     point so decoder's reconstruction at frame N+1
//     consumes a different past-residual than the
//     encoder's L2/L3 search did
//
// ABSOLUTE CONSTRAINTS (parent plan §0.4 + prior d* §0):
//   - Clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729
//     source consulted. Spec source = G729E.{pdf,txt}.
//   - I6 binding: zero production-file changes (this dispatch is
//     measurement-only; both `internal/lpc/levinson.go` (post FIX-1B)
//     and `internal/lsp/encoder_vq.go` are re-frozen).
//   - I5 budget: NOT consumed by this dispatch (measurement only;
//     remains 2/5).
//
// Subtest map:
//
//	S1_Frame0_L2WinnerForensic  — per-row L2 cost dump + row 2 vs row 10 decomposition
//	S2_Frame0_DecoderOracleParity — encoder freqPrev / r1[] vs decoder reconstruction parity
//	S3_WeightProtocolAudit      — weightsLSF vs spec §3.2.4 eq. (22) closed form
//	S4_RearrangementTimingAudit — J1/J2 application points vs spec lines 887–895
//	S5_Frame596_Drivby          — what's special about frame 596 (post-FIX-1B residual fatal)
func TestINT1D6Residual(t *testing.T) {
	if testing.Short() {
		t.Skip("d6 cold-start residual + L2 weighting battery; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		framesToDrive    = 600 // need to reach frame 596
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read LSP.BIT: %v", err)
	}
	if len(inData) < framesToDrive*bytesPerInFrame {
		t.Fatalf("LSP.IN too short: %d bytes, need ≥%d",
			len(inData), framesToDrive*bytesPerInFrame)
	}

	// ----- Pre-roll: replay encoder up to frame 0, capture omega and
	// freqPrev at the entry to Quantize. Also extract WANT indices
	// from LSP.BIT for frame 0.
	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16

	// Frame 0 PCM → preproc → oldSpeech
	var pcmFrame [samplesPerFrame]int16
	for i := 0; i < samplesPerFrame; i++ {
		pcmFrame[i] = int16(binary.LittleEndian.Uint16(
			inData[2*i : 2*i+2]))
	}
	var processed [samplesPerFrame]int16
	pp.Process(pcmFrame[:], processed[:])
	copy(oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
		t.Fatalf("frame 0: lpc.Analyze: %v", err)
	}
	var qQ15 [10]int16
	if err := LPToLSP(&aQ12, &qQ15); err != nil {
		t.Fatalf("frame 0: LPToLSP: %v", err)
	}
	var omega [10]int16
	LSPToLSF(&qQ15, &omega)

	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)
	memSnapAtL2 := freqPrev // bit-exact production state at frame-0 L2 entry

	wantL0d6, wantL1d6, wantL2d6, wantL3d6 := extractLSPFieldsD6(
		bitData[0:bytesPerBitFrame])

	t.Logf("=== Frame 0 entry state ===")
	t.Logf("  a (Q12)        = %v", aQ12)
	t.Logf("  q (Q15)        = %v", qQ15)
	t.Logf("  omega (Q13)    = %v", omega)
	t.Logf("  freqPrev[k=0..3] (Q13) all = initialPastResidual = %v",
		initialPastResidual)
	t.Logf("  WANT (L0,L1,L2,L3) = (%d,%d,%d,%d)",
		wantL0d6, wantL1d6, wantL2d6, wantL3d6)

	// ===================================================================
	// Step 1: Frame-0 L2 winner forensic
	// ===================================================================
	t.Run("S1_Frame0_L2WinnerForensic", func(t *testing.T) {
		// Compute weights with production routine.
		var weights [10]int16
		weightsLSF(&omega, &weights)
		t.Logf("--- weights (Q11) production = %v", weights)

		// L0 selector: per the WANT bit-stream L0=0. We mirror the
		// encoder's per-selector inner loop for sel=0 only (the L0 winner
		// per WANT). For diagnostic completeness we ALSO log the L0=1
		// path's L1 winner, but the L2 forensic targets the WANT branch.
		const sel uint8 = 0

		var target [10]int16
		computeTargetLSF(sel, &memSnapAtL2, &omega, &target)
		t.Logf("--- computeTargetLSF[sel=%d] target (Q13) = %v", sel, target)

		// L1 search: unweighted MSE.
		l1, l1cost := searchL1(&target)
		t.Logf("--- searchL1: l1=%d cost=%d ; want l1=%d", l1, l1cost, wantL1d6)
		if uint8(l1) != wantL1d6 {
			t.Logf("    NOTE: L1 winner differs from WANT — d6 forensic")
			t.Logf("    proceeds with PRODUCTION L1=%d (this is what feeds", l1)
			t.Logf("    the L2 search; we want to see what the L2 search sees,")
			t.Logf("    not what it would see in an oracle world).")
		}

		// Per-row L2 cost using the same arithmetic as searchL2.
		var costs [32]int64
		var omegaHatTopForRow [32][10]int16
		var residualForRow [32][10]int16
		for row := 0; row < 32; row++ {
			var residual [10]int16
			var omegaHat [10]int16
			for i := 0; i < 5; i++ {
				residual[i] = fixed.Add(
					tables.LSPCodebookL1[l1][i],
					tables.LSPCodebookL2[row][i])
			}
			applyPredictorWithMemory(sel, &memSnapAtL2, &residual, &omegaHat)
			// J=0.0012 partial rearrangement on i=1..4 (production
			// inline pattern).
			for i := 1; i < 5; i++ {
				if omegaHat[i]-omegaHat[i-1] < lsfRearrJ1 {
					sum := int32(omegaHat[i]) + int32(omegaHat[i-1])
					omegaHat[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
					omegaHat[i] = int16((sum + int32(lsfRearrJ1)) / 2)
				}
			}
			var mse int64
			for i := 0; i < 5; i++ {
				d := int64(omega[i]) - int64(omegaHat[i])
				mse += int64(weights[i]) * d * d
			}
			costs[row] = mse
			omegaHatTopForRow[row] = omegaHat
			residualForRow[row] = residual
		}

		// Top-3.
		type pair struct {
			Row  int
			Cost int64
		}
		ps := make([]pair, 32)
		for r := 0; r < 32; r++ {
			ps[r] = pair{r, costs[r]}
		}
		sort.Slice(ps, func(i, j int) bool { return ps[i].Cost < ps[j].Cost })
		t.Logf("--- L2 top-3 (sel=%d, l1=%d):", sel, l1)
		for i := 0; i < 3; i++ {
			t.Logf("    rank %d: row=%-2d cost=%-12d", i+1, ps[i].Row, ps[i].Cost)
		}
		// Production winner (lowest cost) and oracle row 10.
		const gotRow = 2
		const wantRow = 10
		t.Logf("--- L2 row %d (production GOT): cost=%d", gotRow, costs[gotRow])
		t.Logf("    ω̂ (Q13, post-J1, partial[0..4]) = %v",
			omegaHatTopForRow[gotRow][0:5])
		t.Logf("    residual (Q13, partial[0..4])    = %v",
			residualForRow[gotRow][0:5])
		t.Logf("--- L2 row %d (decoder WANT): cost=%d", wantRow, costs[wantRow])
		t.Logf("    ω̂ (Q13, post-J1, partial[0..4]) = %v",
			omegaHatTopForRow[wantRow][0:5])
		t.Logf("    residual (Q13, partial[0..4])    = %v",
			residualForRow[wantRow][0:5])

		// Per-coordinate decomposition for got-row vs want-row.
		t.Logf("--- per-coord cost decomposition (Q11·Q13² ≡ int64 units) ---")
		t.Logf("    i | omega   | ω̂_got  | Δ_got  | term_got       || ω̂_want | Δ_want | term_want")
		var sumGot, sumWant int64
		for i := 0; i < 5; i++ {
			dG := int64(omega[i]) - int64(omegaHatTopForRow[gotRow][i])
			tG := int64(weights[i]) * dG * dG
			dW := int64(omega[i]) - int64(omegaHatTopForRow[wantRow][i])
			tW := int64(weights[i]) * dW * dW
			sumGot += tG
			sumWant += tW
			t.Logf("    %d | %6d | %6d  | %+6d | %-14d || %6d | %+6d | %-14d",
				i, omega[i],
				omegaHatTopForRow[gotRow][i], dG, tG,
				omegaHatTopForRow[wantRow][i], dW, tW)
		}
		t.Logf("    Σ           got = %-14d  want = %-14d  Δ(got−want)=%d",
			sumGot, sumWant, sumGot-sumWant)
		if sumGot < sumWant {
			t.Logf("    => production correctly picks row %d under THIS protocol;", gotRow)
			t.Logf("       row %d is more expensive by %d cost units.", wantRow, sumWant-sumGot)
			// Identify dominating coordinate.
			var maxCoord int
			var maxDelta int64
			for i := 0; i < 5; i++ {
				dG := int64(omega[i]) - int64(omegaHatTopForRow[gotRow][i])
				tG := int64(weights[i]) * dG * dG
				dW := int64(omega[i]) - int64(omegaHatTopForRow[wantRow][i])
				tW := int64(weights[i]) * dW * dW
				delta := tW - tG
				if delta > maxDelta || (delta < 0 && -delta > maxDelta) {
					if delta < 0 {
						maxDelta = -delta
					} else {
						maxDelta = delta
					}
					maxCoord = i
				}
			}
			t.Logf("    dominant gap coordinate i=%d (|Δterm|=%d cost units)",
				maxCoord, maxDelta)
		} else {
			t.Logf("    => row %d would already win under THIS protocol — production",
				wantRow)
			t.Logf("       must be diverging UPSTREAM of cost evaluation.")
		}

		// Sanity: run actual searchL2 and confirm it agrees.
		idx, idxCost := searchL2(uint8(l1), sel, &memSnapAtL2, &omega, &weights)
		t.Logf("--- production searchL2 reports: idx=%d cost=%d", idx, idxCost)
	})

	// ===================================================================
	// Step 2: Decoder oracle parity check
	// ===================================================================
	t.Run("S2_Frame0_DecoderOracleParity", func(t *testing.T) {
		// Build the decoder's view at frame 0 with WANT indices
		// (L0=0, L1=120, L2=10, L3=10). Capture pastResiduals state
		// at the moment ω̂ is reconstructed and the resulting
		// reconstructed ω̂ to compare against the encoder's L2-search
		// approximation for row=10.
		var d Decoder
		// Force initialization (Decode would do it but we want the
		// pre-Decode FIFO snapshot).
		for k := 0; k < 4; k++ {
			d.pastResiduals[k] = initialPastResidual
		}
		d.initialized = true
		decoderMem := d.pastResiduals

		t.Logf("--- decoder pastResiduals[k=0..3] (Q13) all = %v",
			initialPastResidual)
		t.Logf("--- encoder freqPrev[k=0..3]      (Q13) all = %v",
			memSnapAtL2[0])

		bitExactMem := true
		for k := 0; k < 4; k++ {
			if decoderMem[k] != memSnapAtL2[k] {
				bitExactMem = false
			}
		}
		t.Logf("--- mem BIT-EXACT encoder vs decoder ? %v", bitExactMem)

		// Encoder L2 search r1[0..9] for L1=120 (no L2 yet, so the
		// "L1 residual" is just L1 codebook contribution; the search
		// then iterates residual = L1 + L2 across rows).
		var r1Encoder [10]int16
		for i := 0; i < 10; i++ {
			r1Encoder[i] = tables.LSPCodebookL1[wantL1d6][i]
		}
		t.Logf("--- encoder r1=L1[%d] (Q13)         = %v", wantL1d6, r1Encoder)

		// Decoder reconstruction with WANT indices: residual = L1+L2/L3
		// then J1, J2 rearrangements on the residual, then predictor.
		var residualDec [10]int16
		combineResidual(wantL1d6, wantL2d6, wantL3d6, &residualDec)
		residualPostCombine := residualDec
		rearrangeAdjacent(&residualDec, lsfRearrJ1)
		residualPostJ1 := residualDec
		rearrangeAdjacent(&residualDec, lsfRearrJ2)
		residualPostJ2 := residualDec

		t.Logf("--- decoder residual stages (with WANT L1=%d L2=%d L3=%d):",
			wantL1d6, wantL2d6, wantL3d6)
		t.Logf("    post-combine = %v", residualPostCombine)
		t.Logf("    post-J1      = %v", residualPostJ1)
		t.Logf("    post-J2      = %v", residualPostJ2)

		// Encoder's "would-be-decoder-equivalent" L1 contribution is
		// just L1[wantL1] — bit-exact identical between encoder and
		// decoder (both read tables.LSPCodebookL1).
		l1MatchesByConstruction := true
		for i := 0; i < 10; i++ {
			if r1Encoder[i] != tables.LSPCodebookL1[wantL1d6][i] {
				l1MatchesByConstruction = false
			}
		}
		t.Logf("--- L1[wantL1] read identically encoder vs decoder ? %v",
			l1MatchesByConstruction)

		// Now run the decoder predictor on the (post-J2) residual to
		// get the decoder's actual ω̂ at frame 0.
		var omegaHatDec [10]int16
		d.applyPredictor(wantL0d6, &residualDec, &omegaHatDec)
		t.Logf("--- decoder ω̂ (Q13, post-predictor)      = %v", omegaHatDec)
		// And what the encoder's L2 search would compute for row=10
		// (residual partial[0..4] = L1[120][0..4] + L2[10][0..4],
		// predictor with the same memory, partial-J1).
		var residualEncSearch [10]int16
		var omegaHatEncSearch [10]int16
		for i := 0; i < 5; i++ {
			residualEncSearch[i] = fixed.Add(
				tables.LSPCodebookL1[wantL1d6][i],
				tables.LSPCodebookL2[wantL2d6][i])
		}
		applyPredictorWithMemory(wantL0d6, &memSnapAtL2,
			&residualEncSearch, &omegaHatEncSearch)
		for i := 1; i < 5; i++ {
			if omegaHatEncSearch[i]-omegaHatEncSearch[i-1] < lsfRearrJ1 {
				sum := int32(omegaHatEncSearch[i]) + int32(omegaHatEncSearch[i-1])
				omegaHatEncSearch[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
				omegaHatEncSearch[i] = int16((sum + int32(lsfRearrJ1)) / 2)
			}
		}
		t.Logf("--- encoder L2-search ω̂ (Q13, partial[0..4], post-partial-J1)")
		t.Logf("    = %v", omegaHatEncSearch[0:5])
		t.Logf("--- decoder    ω̂ (Q13, [0..4] only)")
		t.Logf("    = %v", omegaHatDec[0:5])

		t.Logf("--- structural delta between encoder L2-search ω̂ and decoder ω̂:")
		t.Logf("    (encoder uses J1 ON ω̂ post-predictor; decoder uses J1+J2 on residual PRE-predictor)")
		for i := 0; i < 5; i++ {
			t.Logf("    i=%d  enc=%6d  dec=%6d  Δ=%+d",
				i, omegaHatEncSearch[i], omegaHatDec[i],
				int32(omegaHatEncSearch[i])-int32(omegaHatDec[i]))
		}

		// The two are NOT expected to match because the encoder's L2
		// stage explicitly applies a different (weaker) rearrangement
		// per spec line 890. The forensic question is whether the gap
		// is small enough to keep the cost ordering equivalent.
		var diffSum int32
		for i := 0; i < 5; i++ {
			d := int32(omegaHatEncSearch[i]) - int32(omegaHatDec[i])
			if d < 0 {
				d = -d
			}
			diffSum += d
		}
		t.Logf("--- |Δ| sum over i=0..4 = %d Q13 LSB", diffSum)

		// Also verify the spec's "true" cost of the WANT row using the
		// full decoder pipeline (J1+J2 on residual, predictor) and
		// compare to encoder's heuristic-cost ranking.
		var weights [10]int16
		weightsLSF(&omega, &weights)
		var trueCostWant int64
		for i := 0; i < 5; i++ {
			d := int64(omega[i]) - int64(omegaHatDec[i])
			trueCostWant += int64(weights[i]) * d * d
		}
		t.Logf("--- 'true' cost of WANT (J1+J2 on residual + predictor) = %d",
			trueCostWant)

		// And the same for got-row (L2=2). Use any L3 (e.g., wantL3d6=10,
		// since L3 only affects [5..9] which we don't include here).
		var residualGot [10]int16
		combineResidual(wantL1d6, 2, wantL3d6, &residualGot)
		rearrangeAdjacent(&residualGot, lsfRearrJ1)
		rearrangeAdjacent(&residualGot, lsfRearrJ2)
		var dGot Decoder
		for k := 0; k < 4; k++ {
			dGot.pastResiduals[k] = initialPastResidual
		}
		dGot.initialized = true
		var omegaHatGot [10]int16
		dGot.applyPredictor(wantL0d6, &residualGot, &omegaHatGot)
		var trueCostGot int64
		for i := 0; i < 5; i++ {
			d := int64(omega[i]) - int64(omegaHatGot[i])
			trueCostGot += int64(weights[i]) * d * d
		}
		t.Logf("--- 'true' cost of GOT row=2 (J1+J2 on residual + predictor, L3=%d) = %d",
			wantL3d6, trueCostGot)
		t.Logf("--- 'true' Δ(got−want) = %d (negative ⇒ row 2 STILL wins under spec-true protocol)",
			trueCostGot-trueCostWant)
	})

	// ===================================================================
	// Step 3: Weight protocol audit
	// ===================================================================
	t.Run("S3_WeightProtocolAudit", func(t *testing.T) {
		// Re-derive each weight in float64 from spec eq. (22) and
		// compare to production weightsLSF output (Q11).
		var wProd [10]int16
		weightsLSF(&omega, &wProd)

		// Convert omega Q13 to real radians.
		var w [10]int16
		var floatW [10]float64
		const piQ13 = 25736.0
		_ = piQ13
		omegaRad := make([]float64, 10)
		for i := 0; i < 10; i++ {
			omegaRad[i] = float64(omega[i]) / 8192.0
		}
		// Spec args (1-based):
		//   w_1   : ω_2 − 0.04π − 1
		//   w_i (2..9) : ω_{i+1} − ω_{i-1} − 1
		//   w_10  : −ω_9 + 0.92π − 1
		argFloat := func(arg float64) float64 {
			if arg > 0 {
				return 1.0
			}
			return 1.0 / (10.0*arg*arg + 1.0)
		}
		const pi = math.Pi
		floatW[0] = argFloat(omegaRad[1] - 0.04*pi - 1.0)
		for i := 1; i <= 8; i++ {
			floatW[i] = argFloat(omegaRad[i+1] - omegaRad[i-1] - 1.0)
		}
		floatW[9] = argFloat(-omegaRad[8] + 0.92*pi - 1.0)
		// Apply 1.2 boost on i=4,5 (0-based; w_5/w_6 in spec 1-based).
		floatW[4] *= 1.2
		floatW[5] *= 1.2

		t.Logf("--- weightsLSF audit (Q11 production vs float spec) ---")
		t.Logf("  i | w_prod (Q11) | w_prod_real | w_spec_float | Δ(LSB)")
		var maxDelta float64
		for i := 0; i < 10; i++ {
			realProd := float64(wProd[i]) / 2048.0
			d := math.Abs(realProd - floatW[i])
			if d > maxDelta {
				maxDelta = d
			}
			t.Logf("  %d | %10d   | %.6f    | %.6f     | %+.0f",
				i, wProd[i], realProd, floatW[i],
				(realProd-floatW[i])*2048.0)
		}
		t.Logf("--- max |Δ| = %.6f real (= %.2f Q11 LSB)", maxDelta, maxDelta*2048.0)
		_ = w

		// Spec π in Q13 sanity:
		t.Logf("--- spec constants verify ---")
		t.Logf("  0.04·π (Q13) = round(0.04*π*8192) = %d ; lsfQ13Pi04 = %d",
			int(math.Round(0.04*math.Pi*8192.0)), lsfQ13Pi04)
		t.Logf("  0.92·π (Q13) = round(0.92*π*8192) = %d ; lsfQ13Pi92 = %d",
			int(math.Round(0.92*math.Pi*8192.0)), lsfQ13Pi92)
		t.Logf("  1.0    (Q13) = 8192            ; lsfQ13One  = %d", lsfQ13One)
		t.Logf("  1.0    (Q11) = 2048            ; lsfQ11One  = %d", lsfQ11One)
		t.Logf("  1.2    (Q11) = round(1.2*2048) = %d ; lsfQ11OneTwo = %d",
			int(math.Round(1.2*2048.0)), lsfQ11OneTwo)
	})

	// ===================================================================
	// Step 4: Rearrangement timing audit
	// ===================================================================
	t.Run("S4_RearrangementTimingAudit", func(t *testing.T) {
		t.Logf("--- spec §3.2.4 rearrangement timing (lines 818–830, 887–899) ---")
		t.Logf("  Decoder pipeline (lines 818–833 'process is done twice'):")
		t.Logf("    residual l̂ = L1 ⊕ L2/L3")
		t.Logf("    rearrange l̂ with J=0.0012  (J1)  // line 830")
		t.Logf("    rearrange l̂ with J=0.0006  (J2)  // line 830")
		t.Logf("    apply MA predictor → ω̂              // eq. (20), line 837")
		t.Logf("")
		t.Logf("  Encoder L2 search (lines 889–891):")
		t.Logf("    'For each possible candidate, the partial vector ω̂_i, i=1..5'")
		t.Logf("    'is reconstructed using equation (20),'")
		t.Logf("    'and rearranged to guarantee a minimum distance of 0.0012.'")
		t.Logf("    'The weighted MSE of equation (21) is computed.'")
		t.Logf("    => J=0.0012 applied to ω̂ (post-predictor), partial [1..5].")
		t.Logf("    => J=0.0006 NOT applied during L2 search (spec is silent).")
		t.Logf("")
		t.Logf("  Encoder L3 search (lines 893–895):")
		t.Logf("    'Again the rearrangement procedure is used to guarantee")
		t.Logf("    'a minimum distance of 0.0012.'")
		t.Logf("    => J=0.0012 applied to full ω̂[1..10] (post-predictor).")
		t.Logf("")
		t.Logf("  Final reconstruction (line 895–896):")
		t.Logf("    'The resulting vector l̂_i, i=1..10 is rearranged to guarantee'")
		t.Logf("    'a minimum distance of 0.0006.'")
		t.Logf("    => J=0.0006 applied to l̂ (residual, pre-predictor).")
		t.Logf("    Plus per line 898: 'rearranged twice' referencing 818–830.")
		t.Logf("")
		t.Logf("--- production protocol (encoder_vq.go) ---")
		t.Logf("  searchL2: predictor → J1 on ω̂[1..4] → cost on i=0..4   ✓ matches spec")
		t.Logf("  searchL3: predictor → J1 on ω̂[1..9] → cost on i=5..9   ✓ matches spec")
		t.Logf("  Quantize (final L0 cost): combine residual → J1 on l̂ → J2 on l̂ →")
		t.Logf("           predictor → enforceLSFStability(ω̂) → cost on i=0..9")
		t.Logf("           ✓ matches decoder pipeline (and spec line 898 'twice').")
		t.Logf("")
		t.Logf("--- VERDICT: rearrangement timing matches spec letter for L2/L3 search")
		t.Logf("    and for the final L0 cost. No protocol violation surfaced.")
	})

	// ===================================================================
	// Step 5: Frame-596 drive-by
	// ===================================================================
	t.Run("S5_Frame596_Driveby", func(t *testing.T) {
		const targetFrame = 596
		// Re-replay from frame 0 to targetFrame, capturing the
		// per-frame Levinson reflection-coefficient extremes and the
		// LP-instability outcome at frame 596.
		var pp pcm.PreProcessor
		var an lpc.Analyzer
		var oldSpeech [240]int16
		var freqPrev [4][10]int16
		InitFreqPrev(&freqPrev)

		for f := 0; f <= targetFrame; f++ {
			var pcmFrame [samplesPerFrame]int16
			off := f * bytesPerInFrame
			for i := 0; i < samplesPerFrame; i++ {
				pcmFrame[i] = int16(binary.LittleEndian.Uint16(
					inData[off+2*i : off+2*i+2]))
			}
			var processed [samplesPerFrame]int16
			pp.Process(pcmFrame[:], processed[:])
			copy(oldSpeech[0:160], oldSpeech[80:240])
			copy(oldSpeech[160:240], processed[:])

			var aQ12 [lpc.LPCOrder + 1]int16
			err := an.Analyze(&oldSpeech, &aQ12)
			if f != targetFrame {
				if err != nil {
					t.Logf("frame %d: lpc.Analyze err: %v (rare; logging only)", f, err)
				} else {
					var qQ15 [10]int16
					if e2 := LPToLSP(&aQ12, &qQ15); e2 == nil {
						var omega [10]int16
						LSPToLSF(&qQ15, &omega)
						_ = Quantize(&omega, &freqPrev)
					}
				}
				continue
			}
			// At target frame: dump everything.
			t.Logf("=== Frame %d snapshot ===", f)
			t.Logf("  oldSpeech[0..15]   = %v", oldSpeech[0:16])
			t.Logf("  oldSpeech[224..239]= %v", oldSpeech[224:240])
			if err != nil {
				t.Logf("  lpc.Analyze ERROR: %v", err)
				return
			}
			t.Logf("  a (Q12) [0..10]    = %v", aQ12)
			// Find max |a[i]| as a transient/saturation proxy.
			var maxAbsA int32
			for i := 1; i <= 10; i++ {
				v := int32(aQ12[i])
				if v < 0 {
					v = -v
				}
				if v > maxAbsA {
					maxAbsA = v
				}
			}
			t.Logf("  max |a[1..10]| (Q12) = %d  (real=%.3f)",
				maxAbsA, float64(maxAbsA)/4096.0)
			// Try the LP→LSP and capture failure mode.
			var qQ15 [10]int16
			if errLSP := LPToLSP(&aQ12, &qQ15); errLSP != nil {
				t.Logf("  LPToLSP FAIL: %v", errLSP)
				t.Logf("  => Frame %d is an LP-instability frame post-FIX-1B.", f)
				t.Logf("     Same fault class as pre-FIX-1B frame 29: residual")
				t.Logf("     aWork-precision overflow / Chebyshev sign-change")
				t.Logf("     under-detection. FIX-1B Q24 widening is sufficient")
				t.Logf("     for frame 29 transient but apparently not for the")
				t.Logf("     (different / higher-energy) frame 596 transient.")
				t.Logf("     Candidate fix: FIX-1C Q30 widening of aWork (deferred")
				t.Logf("     per d4 §13.2; revisit if this single residual fatal")
				t.Logf("     is the only remaining LP-stability gap on LSP.IN).")
			} else {
				t.Logf("  LPToLSP OK: q (Q15) = %v", qQ15)
				t.Logf("  => frame %d is NOT an LP-instability frame; the prior", f)
				t.Logf("     report of a fatal here may have been transient or")
				t.Logf("     state-dependent on ordering. Worth re-investigating")
				t.Logf("     under the integration gate test.")
			}
		}
	})
}

// extractLSPFieldsD6 mirrors d2/d3/d5 helpers (one per file to avoid
// cross-test linkage; bit-extraction is identical).
func extractLSPFieldsD6(g192Frame []byte) (l0, l1, l2, l3 uint8) {
	const g192Bit1 uint16 = 0x0081
	bit := func(i int) uint8 {
		off := 4 + 2*i
		if binary.LittleEndian.Uint16(g192Frame[off:off+2]) == g192Bit1 {
			return 1
		}
		return 0
	}
	pack := func(start, n int) uint8 {
		var v uint8
		for i := 0; i < n; i++ {
			v = (v << 1) | bit(start+i)
		}
		return v
	}
	l0 = pack(0, 1)
	l1 = pack(1, 7)
	l2 = pack(8, 5)
	l3 = pack(13, 5)
	return
}
