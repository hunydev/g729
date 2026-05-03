package lsp

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// Phase 2a-VQ-3 step 1: failing test for searchL2.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 889–892 — using the
// selected first-stage vector L1, the lower-half second-stage entry
// L2 ∈ [0, 32) is searched by minimizing the weighted MSE on the
// first five reconstructed LSF coefficients ω̂[0..4] after the
// J=0.0012 pair-rearrangement (lsfRearrJ1, line 890).
//
// Synthetic oracle: with the MA predictor memory set to zero and unit
// weights, the partial reconstructed ω̂[0..4] is a deterministic
// linear function of L1[l1][0..4] + L2[row][0..4]. Computing that
// function for a chosen winning row and presenting the result as the
// search target ω forces the closed-form winner to be that row with
// zero weighted partial MSE.
func TestSearchL2_PartialOracle(t *testing.T) {
	const (
		l1     uint8 = 5
		wantL2 uint8 = 7
		sel    uint8 = 0
	)

	var mem [4][10]int16 // zero predictor memory

	// Build oracle target ω from the candidate winner: combine, then
	// run the same predictor + partial J1 rearrangement the search
	// will perform.
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[wantL2][i]
	}
	var omega [10]int16
	applyPredictorWithMemory(sel, &mem, &residual, &omega)
	for i := 1; i < 5; i++ {
		if omega[i]-omega[i-1] < lsfRearrJ1 {
			sum := int32(omega[i]) + int32(omega[i-1])
			omega[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
			omega[i] = int16((sum + int32(lsfRearrJ1)) / 2)
		}
	}

	var weights [10]int16
	for i := 0; i < 10; i++ {
		weights[i] = lsfQ11One
	}

	gotIdx, gotMSE := searchL2(l1, sel, &mem, &omega, &weights)
	if gotIdx != int(wantL2) {
		t.Fatalf("searchL2 index = %d, want %d", gotIdx, wantL2)
	}
	if gotMSE != 0 {
		t.Fatalf("searchL2 mse = %d, want 0 for oracle target", gotMSE)
	}
}
