package lsp

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/tables"
)

// TestINT1D2Frame5ClosedForm — Phase 2a-INT-1-d2 (diagnostic 2)
// closed-form measurement battery.
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-03-phase2a-int1-d1-diagnostic-plan.md
//	(d2 §6 / §7 appended)
//
// d1 measured frame 0 (silent input) and refuted H-A/H-B/H-C/H-G/H-H/H-I/H-K
// while leaving H-D (silent-input LP convention), H-E (codebook row
// indexing), and H-F (MA-predictor table indexing) live. d2 broadens
// the sample to the first non-silent frame (5) and three later
// frames (10, 50, 100) so the divergence pattern can be compared
// across the silent → speech transition; in addition it dumps raw
// table-shape inspection values so an out-of-band cross-check
// against the spec PDF can refute or confirm H-E / H-F.
//
// ABSOLUTE CONSTRAINTS (parent plan §0.4 + Phase 2a-INT-1 d1 §0):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 source.
//     Spec source = G729E.{pdf,txt} only.
//   - I6 binding: zero production-file changes. This file is _test.go.
//   - measurement-only: no t.Errorf for numeric divergence; only
//     t.Fatalf for missing test vectors and structural sanity, and
//     t.Logf for every measurement.
//
// Subtest map (under TestINT1D2Frame5ClosedForm):
//
//	Frame5_*   — frame index 5 closed-form battery
//	Frame10_*  — frame index 10 closed-form battery
//	Frame15_*  — frame index 15 closed-form battery
//	Frame25_*  — frame index 25 closed-form battery
//
// Note: frames 50 and 100 from the dispatch brief are NOT in scope
// because production's LPToLSP guard fires at frame 29 of LSP.IN
// ("fewer than 5 sign changes in F1 or F2 — LP filter not stable")
// — the integration gate stops there too. The substituted frames
// (15, 25) keep the four-sample shape requested by the brief while
// staying inside the frame range production can actually reach.
//
//	TableShape_LSPCodebookL1 — first/middle/last rows + range stats
//	TableShape_LSPCodebookL2 — first/last rows + range stats
//	TableShape_LSPCodebookL3 — first/last rows + range stats
//	TableShape_MAPredictorsLSP — selector 0/1 × tap 0..3 row dumps + sums
//	Permutation_Frame0_LSPMA_TapReverse — recompute L2 winner with reversed taps
//	Permutation_Frame0_LSPMA_SelectorSwap — recompute L2 winner with swapped selector
//	Permutation_Frame0_L2_BitReverseRow  — recompute L2 winner with bit-reversed row index
func TestINT1D2Frame5ClosedForm(t *testing.T) {
	if testing.Short() {
		t.Skip("d2 closed-form battery; -short")
	}

	// --- I/O preconditions --------------------------------------------
	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164 // G.192 framed: 1 sync + 1 len + 80 data, ×2 bytes each
		minFramesNeeded  = 28  // we drive 0..27 inclusive (frame 29 hits LPToLSP guard)
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
	if len(inData) < minFramesNeeded*bytesPerInFrame {
		t.Fatalf("LSP.IN too short: %d bytes, need ≥%d",
			len(inData), minFramesNeeded*bytesPerInFrame)
	}
	if len(bitData) < minFramesNeeded*bytesPerBitFrame {
		t.Fatalf("LSP.BIT too short: %d bytes, need ≥%d",
			len(bitData), minFramesNeeded*bytesPerBitFrame)
	}

	// --- Sequential lpcStep mirror up to frame 100 --------------------
	//
	// Mirrors encoder.lpcStep exactly: pre-process PCM, slide
	// oldSpeech, run lpc.Analyze, LPToLSP, LSPToLSF, Quantize.
	// Quantize advances freqPrev internally via commitPredictorMemory,
	// so freqPrev tracks production state frame-by-frame.

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)

	targetFrames := map[int]bool{5: true, 10: true, 15: true, 25: true}

	for f := 0; f <= 27; f++ {
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
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}
		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			t.Fatalf("frame %d: LPToLSP: %v", f, err)
		}
		var omega [10]int16
		LSPToLSF(&qQ15, &omega)

		if targetFrames[f] {
			memSnap := freqPrev
			wantL0, wantL1, wantL2, wantL3 := extractLSPFieldsFromG192d2(
				bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
			t.Run(framedSubtestName(f, "Boundary"), func(t *testing.T) {
				dumpFrameBoundary(t, f, &aQ12, &qQ15, &omega, &memSnap,
					wantL0, wantL1, wantL2, wantL3)
			})
		}

		_ = Quantize(&omega, &freqPrev)
	}

	// --- Step 2: table-shape sanity (no production change). -----------
	t.Run("TableShape_LSPCodebookL1", func(t *testing.T) {
		dumpL1Shape(t)
	})
	t.Run("TableShape_LSPCodebookL2", func(t *testing.T) {
		dumpL2Shape(t)
	})
	t.Run("TableShape_LSPCodebookL3", func(t *testing.T) {
		dumpL3Shape(t)
	})
	t.Run("TableShape_MAPredictorsLSP", func(t *testing.T) {
		dumpMAPredShape(t)
	})

	// --- Step 3: permutation sanity for frame 0 -----------------------
	t.Run("Permutation_Frame0", func(t *testing.T) {
		dumpFrame0Permutations(t, inData)
	})
}

func framedSubtestName(f int, suffix string) string {
	switch f {
	case 5:
		return "Frame5_" + suffix
	case 10:
		return "Frame10_" + suffix
	case 15:
		return "Frame15_" + suffix
	case 25:
		return "Frame25_" + suffix
	}
	return "FrameUnknown_" + suffix
}

// extractLSPFieldsFromG192d2 mirrors the root-level helper but lives
// in the lsp package so this test does not depend on root.
func extractLSPFieldsFromG192d2(g192Frame []byte) (l0, l1, l2, l3 uint8) {
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

// top3 returns the three lowest-cost (idx, cost) pairs in ascending
// cost order. Used to log the closest contenders for a stage so we
// can visualise the gap to the "want" row.
func top3(costs []int64) []struct {
	Idx  int
	Cost int64
} {
	type pair struct {
		Idx  int
		Cost int64
	}
	pairs := make([]pair, len(costs))
	for i, c := range costs {
		pairs[i] = pair{i, c}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Cost < pairs[j].Cost })
	out := make([]struct {
		Idx  int
		Cost int64
	}, 3)
	for i := 0; i < 3 && i < len(pairs); i++ {
		out[i] = struct {
			Idx  int
			Cost int64
		}(pairs[i])
	}
	return out
}

// computeL1PerRowCost replicates searchL1's unweighted MSE per row,
// returning all 128 costs so we can extract a top-3 contender list.
func computeL1PerRowCost(target *[10]int16) [128]int64 {
	var out [128]int64
	for row := 0; row < 128; row++ {
		var sum int64
		for i := 0; i < 10; i++ {
			d := int64(target[i]) - int64(tables.LSPCodebookL1[row][i])
			sum += d * d
		}
		out[row] = sum
	}
	return out
}

// dumpFrameBoundary logs the boundary table required by the d1 plan
// §0.4 protocol, evaluated at one target frame, for both selectors.
func dumpFrameBoundary(t *testing.T, f int,
	aQ12 *[lpc.LPCOrder + 1]int16, qQ15, omega *[10]int16,
	memSnap *[4][10]int16,
	wantL0, wantL1, wantL2, wantL3 uint8) {

	t.Logf("=== Frame %d closed-form boundary trace ===", f)
	t.Logf("a (Q12)        = %v", aQ12)
	t.Logf("q (Q15)        = %v", qQ15)
	t.Logf("omega (Q13)    = %v", omega)
	t.Logf("freqPrev[0]    = %v", memSnap[0])
	t.Logf("freqPrev[1]    = %v", memSnap[1])
	t.Logf("freqPrev[2]    = %v", memSnap[2])
	t.Logf("freqPrev[3]    = %v", memSnap[3])
	t.Logf("want indices   : L0=%d L1=%d L2=%d L3=%d",
		wantL0, wantL1, wantL2, wantL3)

	var weights [10]int16
	weightsLSF(omega, &weights)
	t.Logf("weights (Q11)  = %v", weights)

	for sel := uint8(0); sel < 2; sel++ {
		var target [10]int16
		computeTargetLSF(sel, memSnap, omega, &target)
		t.Logf("--- selector=%d ---", sel)
		t.Logf("target (Q13)  = %v", target)

		// L1 contender list
		l1Costs := computeL1PerRowCost(&target)
		l1Top := top3(l1Costs[:])
		t.Logf("L1 top3       = %v", l1Top)
		t.Logf("L1 want=%d cost = %d", wantL1, l1Costs[wantL1])

		// Pick the "got" L1 (argmin) for downstream stages so the
		// search reproduces what production would do at this sel.
		gotL1 := uint8(l1Top[0].Idx)

		// L2 contender list at gotL1
		l2Costs := computeL2PerRowCost(gotL1, sel, memSnap, omega, &weights)
		l2Top := top3(l2Costs[:])
		t.Logf("L2 (l1=%d) top3 = %v", gotL1, l2Top)
		if int(wantL2) < len(l2Costs) {
			t.Logf("L2 want=%d cost = %d", wantL2, l2Costs[wantL2])
		}

		gotL2 := uint8(l2Top[0].Idx)

		// L3 contender list at (gotL1, gotL2) and at (gotL1, wantL2)
		l3GotCosts := computeL3PerRowCost(gotL1, gotL2, sel,
			memSnap, omega, &weights)
		l3Top := top3(l3GotCosts[:])
		t.Logf("L3 (l1=%d,l2=%d-got) top3 = %v", gotL1, gotL2, l3Top)
		if int(wantL3) < len(l3GotCosts) {
			t.Logf("L3 want=%d cost (with got L2)  = %d",
				wantL3, l3GotCosts[wantL3])
		}
		// counterfactual: what if we had picked the want L1 + want L2?
		l1WantCosts := computeL1PerRowCost(&target)
		_ = l1WantCosts
		l3CFCosts := computeL3PerRowCost(wantL1, wantL2, sel,
			memSnap, omega, &weights)
		l3CFTop := top3(l3CFCosts[:])
		t.Logf("L3 (l1=%d-want,l2=%d-want) top3 = %v",
			wantL1, wantL2, l3CFTop)
	}
}

// dumpL1Shape prints the first row, three middle rows, and last row
// of LSPCodebookL1, plus a per-column min/max range across all 128
// rows.  Spec: §3.2.4 Table 7 (L1 codebook, 128 entries × 10 cols, Q13).
func dumpL1Shape(t *testing.T) {
	t.Logf("LSPCodebookL1 dim = %d × %d (want 128 × 10)",
		len(tables.LSPCodebookL1), len(tables.LSPCodebookL1[0]))
	for _, r := range []int{0, 60, 120, 127} {
		t.Logf("L1[%3d] = %v", r, tables.LSPCodebookL1[r])
	}
	// monotonic-increase sanity on first row
	row0 := tables.LSPCodebookL1[0]
	monotone := true
	for i := 1; i < 10; i++ {
		if row0[i] <= row0[i-1] {
			monotone = false
		}
	}
	t.Logf("L1[0] strictly increasing across cols? %v", monotone)

	var colMin, colMax [10]int16
	for j := 0; j < 10; j++ {
		colMin[j] = tables.LSPCodebookL1[0][j]
		colMax[j] = tables.LSPCodebookL1[0][j]
	}
	for r := 0; r < 128; r++ {
		for j := 0; j < 10; j++ {
			v := tables.LSPCodebookL1[r][j]
			if v < colMin[j] {
				colMin[j] = v
			}
			if v > colMax[j] {
				colMax[j] = v
			}
		}
	}
	t.Logf("L1 col min = %v", colMin)
	t.Logf("L1 col max = %v", colMax)
}

func dumpL2Shape(t *testing.T) {
	t.Logf("LSPCodebookL2 dim = %d × %d (want 32 × 5)",
		len(tables.LSPCodebookL2), len(tables.LSPCodebookL2[0]))
	for _, r := range []int{0, 1, 2, 10, 16, 31} {
		t.Logf("L2[%2d] = %v", r, tables.LSPCodebookL2[r])
	}
	var colMin, colMax [5]int16
	for j := 0; j < 5; j++ {
		colMin[j] = tables.LSPCodebookL2[0][j]
		colMax[j] = tables.LSPCodebookL2[0][j]
	}
	for r := 0; r < 32; r++ {
		for j := 0; j < 5; j++ {
			v := tables.LSPCodebookL2[r][j]
			if v < colMin[j] {
				colMin[j] = v
			}
			if v > colMax[j] {
				colMax[j] = v
			}
		}
	}
	t.Logf("L2 col min = %v", colMin)
	t.Logf("L2 col max = %v", colMax)
}

func dumpL3Shape(t *testing.T) {
	t.Logf("LSPCodebookL3 dim = %d × %d (want 32 × 5)",
		len(tables.LSPCodebookL3), len(tables.LSPCodebookL3[0]))
	for _, r := range []int{0, 1, 10, 11, 16, 31} {
		t.Logf("L3[%2d] = %v", r, tables.LSPCodebookL3[r])
	}
	var colMin, colMax [5]int16
	for j := 0; j < 5; j++ {
		colMin[j] = tables.LSPCodebookL3[0][j]
		colMax[j] = tables.LSPCodebookL3[0][j]
	}
	for r := 0; r < 32; r++ {
		for j := 0; j < 5; j++ {
			v := tables.LSPCodebookL3[r][j]
			if v < colMin[j] {
				colMin[j] = v
			}
			if v > colMax[j] {
				colMax[j] = v
			}
		}
	}
	t.Logf("L3 col min = %v", colMin)
	t.Logf("L3 col max = %v", colMax)
}

// dumpMAPredShape prints MAPredictorsLSP[selector][tap][i] for both
// selectors and all four taps, plus per-tap row sums.  The spec
// publishes nominal weights such that Σ_k P[selector][k][i] ≈ 0.7
// (so 1−ΣP ≈ 0.3, i.e. the "30%" weight on the new residual; see
// §3.2.4 eq. 19/20 and the textbook reading of an order-4 MA
// predictor with ~0.30 / 0.30 / 0.20 / 0.20-style mixing). We log
// the per-i sum across taps so it can be cross-checked by
// inspection against the spec values.
func dumpMAPredShape(t *testing.T) {
	for sel := 0; sel < 2; sel++ {
		t.Logf("--- MAPredictorsLSP[selector=%d] ---", sel)
		for tap := 0; tap < 4; tap++ {
			row := tables.MAPredictorsLSP[sel][tap]
			var rowSum int64
			for i := 0; i < 10; i++ {
				rowSum += int64(row[i])
			}
			t.Logf("  P[%d][tap=%d] = %v   rowSum(Q15)=%d (≈ %.4f real)",
				sel, tap, row, rowSum, float64(rowSum)/32768.0)
		}
		// Per-coefficient sum across all four taps. (1 − ΣP) is what
		// applyPredictorWithMemory uses as the "compensator" weight on
		// the new residual; this should be a small positive Q15 value
		// across all 10 coefficients.
		var sumPerCoef [10]int64
		for tap := 0; tap < 4; tap++ {
			for i := 0; i < 10; i++ {
				sumPerCoef[i] += int64(tables.MAPredictorsLSP[sel][tap][i])
			}
		}
		t.Logf("  ΣP[%d][·][i]    = %v", sel, sumPerCoef)
		var compPerCoef [10]int64
		for i := 0; i < 10; i++ {
			compPerCoef[i] = 32767 - sumPerCoef[i]
		}
		t.Logf("  (1−ΣP) Q15      = %v", compPerCoef)
	}
}

// dumpFrame0Permutations runs three table-permutation experiments on
// frame 0 (selector=0). For each permutation we recompute the L1 /
// L2 winner and report whether the (got=2, want=10) divergence at L2
// changes. A permutation that turns argmin from 2 into 10 is a
// strong signal that production's table-read order is wrong.
func dumpFrame0Permutations(t *testing.T, inData []byte) {
	const samplesPerFrame = 80

	// Reconstruct frame-0 inputs (all-zero PCM).
	var pcmFrame0 [samplesPerFrame]int16
	for i := 0; i < samplesPerFrame; i++ {
		pcmFrame0[i] = int16(binary.LittleEndian.Uint16(
			inData[2*i : 2*i+2]))
	}
	var pp pcm.PreProcessor
	var processed [samplesPerFrame]int16
	pp.Process(pcmFrame0[:], processed[:])
	var oldSpeech [240]int16
	for i := 0; i < 80; i++ {
		oldSpeech[160+i] = processed[i]
	}
	var an lpc.Analyzer
	var aQ12 [lpc.LPCOrder + 1]int16
	if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
		t.Fatalf("lpc.Analyze: %v", err)
	}
	var qQ15 [10]int16
	if err := LPToLSP(&aQ12, &qQ15); err != nil {
		t.Fatalf("LPToLSP: %v", err)
	}
	var omega [10]int16
	LSPToLSF(&qQ15, &omega)

	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)
	var weights [10]int16
	weightsLSF(&omega, &weights)

	const wantL1 uint8 = 120
	const wantL2 = 10
	const gotL2Production = 2

	// --- Permutation A: reverse MAPredictorsLSP tap order -------------
	// Hypothesis: tap k in production reads tables.MAPredictorsLSP[sel][k]
	// but the table may have been authored with tap k = 4-k order, so
	// the right pairing would be preds[3-k] · mem[k].
	t.Run("LSPMA_TapReverse_sel0", func(t *testing.T) {
		costs := l2CostsWithTapReverse(wantL1, 0, &freqPrev, &omega, &weights)
		argmin := argmin64(costs[:])
		t.Logf("L2 argmin under reversed taps = %d (cost=%d)",
			argmin, costs[argmin])
		t.Logf("  cost @ got(2)  = %d", costs[gotL2Production])
		t.Logf("  cost @ want(10)= %d", costs[wantL2])
		if argmin == wantL2 {
			t.Logf("  *** SIGNAL: tap-reversal produces want=10 ***")
		} else {
			t.Logf("  no rescue; tap-reversal does not flip argmin to want")
		}
	})

	// --- Permutation B: swap MAPredictorsLSP selector 0↔1 -------------
	// Hypothesis: selector indexing inverted at table-author time.
	t.Run("LSPMA_SelectorSwap", func(t *testing.T) {
		// Use selector=1 as if it were selector=0.
		costs := computeL2PerRowCost(wantL1, 1, &freqPrev, &omega, &weights)
		argmin := argmin64(costs[:])
		t.Logf("L2 (sel=1 stand-in for sel=0) argmin = %d (cost=%d)",
			argmin, costs[argmin])
		t.Logf("  cost @ got(2)   = %d", costs[gotL2Production])
		t.Logf("  cost @ want(10) = %d", costs[wantL2])
		if argmin == wantL2 {
			t.Logf("  *** SIGNAL: selector-swap produces want=10 ***")
		}
	})

	// --- Permutation C: bit-reverse the L2 row index ------------------
	// Hypothesis: production search returns row r, but the bitstream
	// transmits bitrev5(r); the underlying winner is the same but the
	// codepoint emitted differs.
	t.Run("L2_BitReverseRowIndex", func(t *testing.T) {
		bitrev5 := func(r int) int {
			rev := 0
			for b := 0; b < 5; b++ {
				if r&(1<<b) != 0 {
					rev |= 1 << (4 - b)
				}
			}
			return rev
		}
		costs := computeL2PerRowCost(wantL1, 0, &freqPrev, &omega, &weights)
		argmin := argmin64(costs[:])
		t.Logf("plain argmin = %d (=%05b)", argmin, argmin)
		t.Logf("bitrev5(argmin) = %d (=%05b)",
			bitrev5(argmin), bitrev5(argmin))
		t.Logf("bitrev5(want=10=%05b) = %d (=%05b)",
			wantL2, bitrev5(wantL2), bitrev5(wantL2))
		if bitrev5(argmin) == wantL2 {
			t.Logf("  *** SIGNAL: bit-reversal of argmin yields want ***")
		}
	})

	// --- Permutation D: bit-reverse the L1 row index (7-bit) ----------
	t.Run("L1_BitReverseRowIndex", func(t *testing.T) {
		bitrev7 := func(r int) int {
			rev := 0
			for b := 0; b < 7; b++ {
				if r&(1<<b) != 0 {
					rev |= 1 << (6 - b)
				}
			}
			return rev
		}
		var target [10]int16
		computeTargetLSF(0, &freqPrev, &omega, &target)
		costs := computeL1PerRowCost(&target)
		argmin := 0
		for r := 1; r < 128; r++ {
			if costs[r] < costs[argmin] {
				argmin = r
			}
		}
		t.Logf("plain L1 argmin = %d (=%07b)", argmin, argmin)
		t.Logf("bitrev7(argmin) = %d (=%07b)",
			bitrev7(argmin), bitrev7(argmin))
		t.Logf("bitrev7(want=120=%07b) = %d (=%07b)",
			wantL1, bitrev7(int(wantL1)), bitrev7(int(wantL1)))
		// Sanity: production agrees with want on L1=120 already, so the
		// bitrev signal here should be that NO permutation is needed
		// (i.e. plain argmin == want).
	})
}

// l2CostsWithTapReverse mirrors computeL2PerRowCost but invokes a
// tap-reversed predictor application: pred-tap k is paired with
// memory tap (3-k). All other arithmetic (Q-format, rearrangeAdjacent
// pre-rearrange J1, partial WMSE on i=0..4) is identical to
// production's searchL2.
func l2CostsWithTapReverse(l1, selector uint8, mem *[4][10]int16,
	omega, weights *[10]int16) [32]int64 {

	var out [32]int64
	var residual, omegaHat [10]int16

	for row := 0; row < 32; row++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] +
				tables.LSPCodebookL2[row][i]
		}
		applyPredictorTapReversed(selector, mem, &residual, &omegaHat)
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
		out[row] = mse
	}
	return out
}

// applyPredictorTapReversed is a measurement-only mirror of
// applyPredictorWithMemory with preds[k] swapped for preds[3-k] in
// the LMac chain. Used by Permutation A only.
func applyPredictorTapReversed(selector uint8, mem *[4][10]int16,
	residual, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]
	for i := 0; i < 10; i++ {
		var sumP int32
		for k := 0; k < 4; k++ {
			sumP += int32(preds[k][i])
		}
		comp := int32(32767) - sumP
		// emulate fixed.LMac: acc += a*b<<1 (Q15·Q13 with rounding).
		acc := int64(comp) * int64(residual[i]) << 1
		// reversed pairing
		acc += int64(preds[3][i]) * int64(mem[0][i]) << 1
		acc += int64(preds[2][i]) * int64(mem[1][i]) << 1
		acc += int64(preds[1][i]) * int64(mem[2][i]) << 1
		acc += int64(preds[0][i]) * int64(mem[3][i]) << 1
		// round-to-nearest, then cast to int16.
		acc = (acc + (1 << 15)) >> 16
		if acc > 32767 {
			acc = 32767
		}
		if acc < -32768 {
			acc = -32768
		}
		out[i] = int16(acc)
	}
}

func argmin64(xs []int64) int {
	bi := 0
	for i := 1; i < len(xs); i++ {
		if xs[i] < xs[bi] {
			bi = i
		}
	}
	return bi
}
