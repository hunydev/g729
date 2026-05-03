package lsp

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/tables"
)

// TestINT1D1ClosedForm — Phase 2a-INT-1-d1 (diagnostic 1) closed-form
// measurement battery for the frame-0 L2/L3 divergence.
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-03-phase2a-int1-d1-diagnostic-plan.md
//
// Cycle context: the integration gate TestEncode_LSPVectorBitExact
// (root) is RED at frame 0 with got=(L0=0,L1=120,L2=2,L3=11) vs
// want=(0,120,10,10). This test reproduces the frame-0 lpcStep
// boundary values WITHOUT modifying any production code (I6 freeze)
// and emits, via t.Logf only, every quantity required to localise
// the first-divergence boundary in closed form, per the §0.4
// boundary-trace protocol of the parent plan and the Phase 1o D-3
// pattern.
//
// ABSOLUTE CONSTRAINTS (parent plan §0.4 + Phase 2a-INT-1 d1):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 source.
//     Only spec arithmetic from G729E.{pdf,txt} §3.2.4.
//   - I6 binding: zero production-file changes. This file is _test.go.
//   - measurement-only: no t.Errorf / t.Fatalf for numeric divergence;
//     hard-asserts are restricted to structural sanity (file sizes,
//     L1 winner == 120, sel chosen by closed form == got).
//
// Subtest map (under TestINT1D1ClosedForm):
//
//	S0_LPCAQ12               — a[0..10] Q12 after lpc.Analyze.
//	S1_OmegaQ13              — ω[0..9] Q13 after LPToLSP+LSPToLSF.
//	S2_TargetLSF             — l_i[0..9] Q13 (sel=0 and sel=1) per eq. 23.
//	S3_Weights               — w[0..9] Q11 per eq. 22.
//	S4_L1Winner              — searchL1 winner (expect 120).
//	S5_L2PerRowCost_sel0     — partial WMSE for all 32 L2 rows, sel=0.
//	S6_L3PerRowCost_sel0     — partial WMSE for all 32 L3 rows, sel=0,
//	                           given L1=120 and L2 = production winner.
//	S7_LSBGap_L2Row2VsRow10  — head-to-head decomposition: residual,
//	                           ω̂, post-rearrange ω̂, per-coefficient
//	                           weighted-square contribution, total.
//	S8_LSBGap_L3Row11VsRow10 — same for L3 row 11 (got) vs row 10 (want).
func TestINT1D1ClosedForm(t *testing.T) {
	if testing.Short() {
		t.Skip("d1 closed-form battery; -short")
	}

	// --- Frame-0 input reconstruction ----------------------------------

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	const samplesPerFrame = 80
	if len(inData) < 2*samplesPerFrame {
		t.Fatalf("LSP.IN truncated: %d bytes", len(inData))
	}

	var pcmFrame0 [samplesPerFrame]int16
	for i := 0; i < samplesPerFrame; i++ {
		pcmFrame0[i] = int16(binary.LittleEndian.Uint16(
			inData[2*i : 2*i+2]))
	}

	// Mirror the production lpcStep pipeline (encoder.go) on frame 0
	// without running the encoder, so we can dump intermediates.
	// The Encoder buffer convention at frame 0 is:
	//   oldSpeech[0..159] = 0  (first frame, no past)
	//   oldSpeech[160..239] = preprocessed(pcmFrame0[0..79])
	var pp pcm.PreProcessor
	var processed [samplesPerFrame]int16
	pp.Process(pcmFrame0[:], processed[:])

	var oldSpeech [240]int16
	for i := 0; i < 80; i++ {
		oldSpeech[160+i] = processed[i]
	}

	// --- S0: LPC a[0..10] Q12 ------------------------------------------
	var aQ12 [lpc.LPCOrder + 1]int16
	var an lpc.Analyzer
	if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
		t.Fatalf("lpc.Analyze: %v", err)
	}

	t.Run("S0_LPCAQ12", func(t *testing.T) {
		t.Logf("a[0..10] (Q12) = %v", aQ12)
	})

	// --- S1: ω[0..9] Q13 -----------------------------------------------
	var qQ15 [10]int16
	if err := LPToLSP(&aQ12, &qQ15); err != nil {
		t.Fatalf("LPToLSP: %v", err)
	}
	var omega [10]int16
	LSPToLSF(&qQ15, &omega)

	t.Run("S1_OmegaQ13", func(t *testing.T) {
		t.Logf("q[0..9]    (Q15) = %v", qQ15)
		t.Logf("omega[0..9](Q13) = %v", omega)
		t.Logf("(decoder cold-start initialPastResidual reference) = %v",
			initialPastResidual)
	})

	// Seed freqPrev exactly as Encoder does on cold start.
	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)

	// --- S2: target l_i Q13 (eq. 23) for sel=0 and sel=1 ---------------
	var target0, target1 [10]int16
	computeTargetLSF(0, &freqPrev, &omega, &target0)
	computeTargetLSF(1, &freqPrev, &omega, &target1)
	t.Run("S2_TargetLSF", func(t *testing.T) {
		t.Logf("target sel=0 (Q13) = %v", target0)
		t.Logf("target sel=1 (Q13) = %v", target1)
	})

	// --- S3: weights w[0..9] Q11 ---------------------------------------
	var weights [10]int16
	weightsLSF(&omega, &weights)
	t.Run("S3_Weights", func(t *testing.T) {
		t.Logf("weights[0..9] (Q11) = %v", weights)
	})

	// --- S4: L1 winner (sel-independent — searchL1 takes target) -------
	// searchL1 takes the per-selector target, so winners may differ.
	l1Sel0, costL1Sel0 := searchL1(&target0)
	l1Sel1, costL1Sel1 := searchL1(&target1)
	t.Run("S4_L1Winner", func(t *testing.T) {
		t.Logf("sel=0: searchL1 winner = %d, sumSqDiff(Q26) = %d",
			l1Sel0, costL1Sel0)
		t.Logf("sel=1: searchL1 winner = %d, sumSqDiff(Q26) = %d",
			l1Sel1, costL1Sel1)
		t.Logf("integration-gate observation: got L0=0 L1=120; "+
			"sel=0 reproduces L1=%d here", l1Sel0)
	})

	// Pin the L1 winner to the production / want value 120 for the
	// subsequent L2/L3 measurements (both got and want agree on
	// L1=120 at frame 0; the divergence is downstream).
	const wantL1 uint8 = 120
	const gotL2 = 2
	const wantL2 = 10
	const gotL3 = 11
	const wantL3 = 10

	// --- S5: L2 per-row partial WMSE for sel=0 -------------------------
	// Exactly mirrors searchL2 inline so we can dump per-row totals.
	t.Run("S5_L2PerRowCost_sel0", func(t *testing.T) {
		costs := computeL2PerRowCost(wantL1, 0, &freqPrev, &omega, &weights)
		var argmin int
		for r := 0; r < 32; r++ {
			if costs[r] < costs[argmin] {
				argmin = r
			}
			t.Logf("  L2 sel=0 row=%2d  partialWMSE=%d", r, costs[r])
		}
		t.Logf("argmin row = %d (cost=%d)", argmin, costs[argmin])
		t.Logf("got row = %d (cost=%d)", gotL2, costs[gotL2])
		t.Logf("want row = %d (cost=%d)", wantL2, costs[wantL2])
		t.Logf("delta(got - want) = %d", costs[gotL2]-costs[wantL2])
	})

	// --- S6: L3 per-row partial WMSE for sel=0, given L1=120, L2=got ---
	t.Run("S6_L3PerRowCost_sel0_givenL2Got", func(t *testing.T) {
		costs := computeL3PerRowCost(wantL1, gotL2, 0,
			&freqPrev, &omega, &weights)
		var argmin int
		for r := 0; r < 32; r++ {
			if costs[r] < costs[argmin] {
				argmin = r
			}
			t.Logf("  L3 sel=0 row=%2d  partialWMSE=%d", r, costs[r])
		}
		t.Logf("argmin row = %d (cost=%d)", argmin, costs[argmin])
		t.Logf("got row = %d (cost=%d)", gotL3, costs[gotL3])
		t.Logf("want row = %d (cost=%d)", wantL3, costs[wantL3])
		t.Logf("delta(got - want) = %d", costs[gotL3]-costs[wantL3])
	})

	// Counterfactual: if the encoder had picked L2=10 (want), would
	// L3=10 win?
	t.Run("S6b_L3PerRowCost_sel0_givenL2Want", func(t *testing.T) {
		costs := computeL3PerRowCost(wantL1, wantL2, 0,
			&freqPrev, &omega, &weights)
		var argmin int
		for r := 0; r < 32; r++ {
			if costs[r] < costs[argmin] {
				argmin = r
			}
		}
		t.Logf("argmin row = %d (cost=%d)", argmin, costs[argmin])
		t.Logf("want row = %d (cost=%d)", wantL3, costs[wantL3])
	})

	// --- S7: head-to-head L2 row 2 (got) vs row 10 (want) --------------
	t.Run("S7_LSBGap_L2Row2VsRow10_sel0", func(t *testing.T) {
		dumpL2Pair(t, wantL1, gotL2, wantL2, 0, &freqPrev, &omega, &weights)
	})

	// --- S8: head-to-head L3 row 11 (got) vs row 10 (want) -------------
	t.Run("S8_LSBGap_L3Row11VsRow10_sel0_givenL2Got", func(t *testing.T) {
		dumpL3Pair(t, wantL1, gotL2, gotL3, wantL3, 0,
			&freqPrev, &omega, &weights)
	})
	t.Run("S8b_LSBGap_L3Row11VsRow10_sel0_givenL2Want", func(t *testing.T) {
		dumpL3Pair(t, wantL1, wantL2, gotL3, wantL3, 0,
			&freqPrev, &omega, &weights)
	})
}

// computeL2PerRowCost replicates searchL2 inline and returns the
// partial WMSE for every L2 row at fixed (l1, selector). int64 to
// match production's accumulator width.
func computeL2PerRowCost(l1, selector uint8, mem *[4][10]int16,
	omega, weights *[10]int16) [32]int64 {

	var out [32]int64
	var residual, omegaHat [10]int16

	for row := 0; row < 32; row++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] +
				tables.LSPCodebookL2[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)

		// Partial-only J1 rearrange on indices 1..4 (matches
		// production searchL2 exactly).
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

// computeL3PerRowCost replicates searchL3 inline.
func computeL3PerRowCost(l1, l2, selector uint8, mem *[4][10]int16,
	omega, weights *[10]int16) [32]int64 {

	var out [32]int64
	var residual, omegaHat [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[l1][i] +
			tables.LSPCodebookL2[l2][i]
	}
	for row := 0; row < 32; row++ {
		for i := 0; i < 5; i++ {
			residual[5+i] = tables.LSPCodebookL1[l1][5+i] +
				tables.LSPCodebookL3[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
		rearrangeAdjacent(&omegaHat, lsfRearrJ1)

		var mse int64
		for i := 5; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			mse += int64(weights[i]) * d * d
		}
		out[row] = mse
	}
	return out
}

// dumpL2Pair logs a side-by-side decomposition of two L2 candidates.
func dumpL2Pair(t *testing.T, l1, rowA, rowB, selector uint8,
	mem *[4][10]int16, omega, weights *[10]int16) {

	for _, row := range []uint8{rowA, rowB} {
		var residual, omegaHat [10]int16
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] +
				tables.LSPCodebookL2[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
		var preRearr [10]int16 = omegaHat
		for i := 1; i < 5; i++ {
			if omegaHat[i]-omegaHat[i-1] < lsfRearrJ1 {
				sum := int32(omegaHat[i]) + int32(omegaHat[i-1])
				omegaHat[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
				omegaHat[i] = int16((sum + int32(lsfRearrJ1)) / 2)
			}
		}

		t.Logf("--- L2 row %d (selector=%d) ---", row, selector)
		t.Logf("  L1[%d][0..4]      = %v",
			l1, tables.LSPCodebookL1[l1][:5])
		t.Logf("  L2[%d][0..4]      = %v",
			row, tables.LSPCodebookL2[row])
		t.Logf("  residual[0..4]    = %v", residual[:5])
		t.Logf("  omegaHat preRearr = %v", preRearr[:5])
		t.Logf("  omegaHat postJ1   = %v", omegaHat[:5])
		t.Logf("  omega target      = %v", omega[:5])
		var total int64
		for i := 0; i < 5; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			contrib := int64(weights[i]) * d * d
			total += contrib
			t.Logf("    i=%d  w=%d  d=%d  w*d^2=%d", i,
				weights[i], d, contrib)
		}
		t.Logf("  TOTAL partial WMSE = %d", total)
	}
}

// dumpL3Pair logs a side-by-side decomposition of two L3 candidates.
func dumpL3Pair(t *testing.T, l1, l2, rowA, rowB, selector uint8,
	mem *[4][10]int16, omega, weights *[10]int16) {

	for _, row := range []uint8{rowA, rowB} {
		var residual, omegaHat [10]int16
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] +
				tables.LSPCodebookL2[l2][i]
			residual[5+i] = tables.LSPCodebookL1[l1][5+i] +
				tables.LSPCodebookL3[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
		var preRearr [10]int16 = omegaHat
		rearrangeAdjacent(&omegaHat, lsfRearrJ1)

		t.Logf("--- L3 row %d (l1=%d, l2=%d, selector=%d) ---",
			row, l1, l2, selector)
		t.Logf("  L3[%d][0..4]      = %v",
			row, tables.LSPCodebookL3[row])
		t.Logf("  residual[5..9]    = %v", residual[5:10])
		t.Logf("  omegaHat preRearr = %v", preRearr)
		t.Logf("  omegaHat postJ1   = %v", omegaHat)
		t.Logf("  omega target[5..9]= %v", omega[5:10])
		var total int64
		for i := 5; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			contrib := int64(weights[i]) * d * d
			total += contrib
			t.Logf("    i=%d  w=%d  d=%d  w*d^2=%d", i,
				weights[i], d, contrib)
		}
		t.Logf("  TOTAL partial WMSE (i=5..9) = %d", total)
	}
}
