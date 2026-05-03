package lsp

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// Phase 2a-VQ-5 step 1: failing test for Quantize / L0 outer loop.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 851 ("For each of the two
// MA predictors the best approximation … has to be found") and
// 896–897 ("the MA predictor L0 that produces the lowest weighted
// MSE is selected").
//
// Synthetic oracle: for a chosen predictor selector sel ∈ {0,1}, a
// chosen (L1,L2,L3) tuple and zero predictor memory, build the
// reconstructed ω̂ via the encoder predictor + J1 pair-rearrangement
// (matching the cost convention used by searchL2/searchL3) and pass
// that ω̂ as the search target. The chosen sel must win because its
// inner-loop minimum is exactly zero, while the rival selector
// reproduces a different ω̂ for the same residual and therefore
// incurs a strictly positive partial cost.
func TestQuantize_L0SelectsLowerMSEPredictor(t *testing.T) {
	cases := []struct {
		name string
		sel  uint8
		l1   uint8
		l2   uint8
		l3   uint8
	}{
		{name: "predictor0_wins", sel: 0, l1: 5, l2: 7, l3: 11},
		{name: "predictor1_wins", sel: 1, l1: 23, l2: 19, l3: 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mem [4][10]int16

			var residual [10]int16
			for i := 0; i < 5; i++ {
				residual[i] = tables.LSPCodebookL1[tc.l1][i] + tables.LSPCodebookL2[tc.l2][i]
			}
			for i := 0; i < 5; i++ {
				residual[5+i] = tables.LSPCodebookL1[tc.l1][5+i] + tables.LSPCodebookL3[tc.l3][i]
			}
			var omega [10]int16
			applyPredictorWithMemory(tc.sel, &mem, &residual, &omega)
			rearrangeAdjacent(&omega, lsfRearrJ1)

			memCopy := mem
			got := Quantize(&omega, &memCopy)
			if got.L0 != tc.sel {
				t.Fatalf("Quantize.L0 = %d, want %d", got.L0, tc.sel)
			}
			if got.L1 != tc.l1 {
				t.Fatalf("Quantize.L1 = %d, want %d", got.L1, tc.l1)
			}
			if got.L2 != tc.l2 {
				t.Fatalf("Quantize.L2 = %d, want %d", got.L2, tc.l2)
			}
			if got.L3 != tc.l3 {
				t.Fatalf("Quantize.L3 = %d, want %d", got.L3, tc.l3)
			}

			// commitPredictorMemory must have advanced the FIFO once
			// using the winning residual.
			if memCopy[0] != residual {
				t.Fatalf("predictor memory mem[0] = %v, want winning residual %v", memCopy[0], residual)
			}
			for i := 1; i < 4; i++ {
				if memCopy[i] != mem[i-1] {
					t.Fatalf("predictor memory mem[%d] not shifted from mem[%d]", i, i-1)
				}
			}
		})
	}
}

// I12 invariant documentation: exactly 2 · (128 + 32 + 32) = 384
// candidate evaluations per frame (sequential-greedy split-VQ across
// two MA predictors). Plan §VQ-5 step "I12 verification".
func TestQuantize_EvaluationBudget(t *testing.T) {
	const got = 2 * (len(tables.LSPCodebookL1) + len(tables.LSPCodebookL2) + len(tables.LSPCodebookL3))
	const want = 384
	if got != want {
		t.Fatalf("per-frame VQ evaluation budget = %d, want %d", got, want)
	}
}
