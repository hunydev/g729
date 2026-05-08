package lsp

import (
	"testing"

	"github.com/hunydev/g729/internal/tables"
)

// Phase 2a-VQ-4 step 1: failing test for searchL3.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 893–895 — using the
// selected first-stage vector L1 and lower-half second-stage entry
// L2, the upper-half second-stage entry L3 ∈ [0, 32) is searched by
// minimizing the weighted MSE on the upper five reconstructed LSF
// coefficients ω̂[5..9]. Per line 893, the J=0.0012 pair-
// rearrangement (lsfRearrJ1) is applied "again" — this time across
// the full reconstructed [0..9] vector before the partial cost is
// summed.
//
// Synthetic oracle: with zero predictor memory and unit weights, the
// reconstructed ω̂ is a deterministic linear function of the combined
// residual. Building the search target ω from a chosen winning row
// using the same predictor + full J1 rearrangement forces that row
// to win with zero partial weighted MSE.
func TestSearchL3_PartialOracle(t *testing.T) {
	const (
		l1     uint8 = 5
		l2     uint8 = 7
		wantL3 uint8 = 11
		sel    uint8 = 0
	)

	var mem [4][10]int16 // zero predictor memory

	// Build oracle target ω from the candidate winner: combine
	// (lower = L1+L2, upper = L1+L3), run the predictor, then apply
	// J1 rearrangement on the full vector.
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
	}
	for i := 0; i < 5; i++ {
		residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[wantL3][i]
	}
	var omega [10]int16
	applyPredictorWithMemory(sel, &mem, &residual, &omega)
	rearrangeAdjacent(&omega, lsfRearrJ1)

	var weights [10]int16
	for i := 0; i < 10; i++ {
		weights[i] = lsfQ11One
	}

	gotIdx, gotMSE := searchL3(l1, l2, sel, &mem, &omega, &weights)
	if gotIdx != int(wantL3) {
		t.Fatalf("searchL3 index = %d, want %d", gotIdx, wantL3)
	}
	if gotMSE != 0 {
		t.Fatalf("searchL3 mse = %d, want 0 for oracle target", gotMSE)
	}
}
