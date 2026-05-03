package lsp

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// Phase 2a-VQ-2 step 1: failing test for searchL1.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 887–888 — first-stage
// quantizer L1 selects the entry that minimizes the (unweighted)
// mean-squared error between the target vector and a row of
// LSPCodebookL1.
func TestSearchL1_ExactRowMatch(t *testing.T) {
	const want = 17
	var target [10]int16
	for i := 0; i < 10; i++ {
		target[i] = tables.LSPCodebookL1[want][i]
	}
	got, mse := searchL1(&target)
	if got != want {
		t.Fatalf("searchL1 index = %d, want %d", got, want)
	}
	if mse != 0 {
		t.Fatalf("searchL1 mse = %d, want 0 for exact-row target", mse)
	}
}
