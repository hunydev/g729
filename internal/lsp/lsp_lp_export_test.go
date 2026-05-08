package lsp_test

import (
	"testing"

	"github.com/hunydev/g729/internal/lsp"
)

// TestLSPToLPExported pins the QA-1 contract: lsp.LSPToLP must be a
// public symbol so the encoder can reconstruct quantized Â(z) from
// quantized LSP outside the package. The functional correctness is
// covered by in-package tests; this black-box test only guards the
// export surface and the documented signature/Q-format (a[0] = 4096
// in Q12 per §3.2.6).
func TestLSPToLPExported(t *testing.T) {
	var q [10]int16
	var a [11]int16
	lsp.LSPToLP(&q, &a)
	if a[0] != 4096 {
		t.Fatalf("LSPToLP: a[0] = %d, want 4096 (1.0 in Q12)", a[0])
	}
}
