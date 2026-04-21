package lsp

import "testing"

func TestLSPToLPLeadingCoefficient(t *testing.T) {
	lsp := [10]int16{31000, 28000, 24000, 20000, 15000, 10000, 5000, -1000, -10000, -20000}
	var a [11]int16
	lspToLP(&lsp, &a)
	if a[0] != 4096 {
		t.Fatalf("a[0] = %d, want 4096 (Q12 1.0)", a[0])
	}
}

func TestLSPToLPAllZeroLSPProducesSymmetric(t *testing.T) {
	var lsp [10]int16
	var a [11]int16
	lspToLP(&lsp, &a)
	for i := 1; i < 11; i += 2 {
		if a[i] != 0 {
			t.Errorf("a[%d] = %d, want 0 with all-zero LSP", i, a[i])
		}
	}
}
