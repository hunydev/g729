package lsp

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

func TestCombineAllZeroIndices(t *testing.T) {
	var r [10]int16
	combineResidual(0, 0, 0, &r)

	for i := 0; i < 5; i++ {
		want := tables.LSPCodebookL1[0][i] + tables.LSPCodebookL2[0][i]
		if r[i] != want {
			t.Errorf("r[%d] = %d, want %d (L1[0][%d]+L2[0][%d])",
				i, r[i], want, i, i)
		}
	}
	for i := 5; i < 10; i++ {
		want := tables.LSPCodebookL1[0][i] + tables.LSPCodebookL3[0][i-5]
		if r[i] != want {
			t.Errorf("r[%d] = %d, want %d (L1[0][%d]+L3[0][%d])",
				i, r[i], want, i, i-5)
		}
	}
}

func TestCombineDifferentIndicesDiffer(t *testing.T) {
	var a, b [10]int16
	combineResidual(0, 0, 0, &a)
	combineResidual(1, 0, 0, &b)
	if a == b {
		t.Fatalf("combineResidual did not vary with L1: %v == %v", a, b)
	}
}
