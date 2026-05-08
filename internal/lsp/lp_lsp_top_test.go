package lsp

import (
	"testing"

	"github.com/hunydev/g729/internal/tables"
)

// TestLPToLSP_RoundTripCodebookL1 sweeps the 128 entries of
// tables.LSPCodebookL1 (each row is a sorted Q13 LSF prototype per
// ITU-T G.729 §3.2.4) and verifies the chain
//
//	LSF → LSP (lsfToLSP) → LP (LSPToLP) → LSP (LPToLSP)
//
// recovers the input LSP within the algorithm's intrinsic tolerance.
//
// Tolerance: the plan's "±4 LSB Q15" target is unreachable with the
// I11-binding (60-point grid, 4 binary subdivisions) configuration;
// see the deviation note on Task 2a-LP-3 in
// docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md and the
// sin(ω)·Δω derivation in TestFindLSPRoots_RecoversChosenLSPs
// (lp_lsp_roots_test.go). The (60, 4) constraint dominates the
// numerical floor at ≈109 Q15 LSBs; we use 256 to additionally
// absorb chebyshevC Q24 truncation and the LSPToLP forward error.
func TestLPToLSP_RoundTripCodebookL1(t *testing.T) {
	const tol int32 = 256

	for row := 0; row < len(tables.LSPCodebookL1); row++ {
		var lsp [10]int16
		for i := 0; i < 10; i++ {
			lsp[i] = lsfToLSP(tables.LSPCodebookL1[row][i])
		}

		var a [11]int16
		LSPToLP(&lsp, &a)

		var q [10]int16
		if err := LPToLSP(&a, &q); err != nil {
			t.Fatalf("row=%d: LPToLSP returned error: %v", row, err)
		}

		for i := 0; i < 10; i++ {
			d := int32(q[i]) - int32(lsp[i])
			if d < 0 {
				d = -d
			}
			if d > tol {
				t.Errorf("row=%d q[%d]=%d want≈%d (Δ=%d > tol=%d)",
					row, i, q[i], lsp[i], d, tol)
			}
		}

		// Recovered LSPs must remain strictly decreasing in cosine
		// domain (≡ strictly increasing in ω).
		for i := 1; i < 10; i++ {
			if q[i] >= q[i-1] {
				t.Errorf("row=%d: q not strictly decreasing at i=%d: %d >= %d",
					row, i, q[i], q[i-1])
			}
		}
	}
}

// TestLPToLSP_ZeroAlloc guards I4: the wrapper must not allocate in
// steady state. The two scratch buffers (f1, f2) live on the stack.
func TestLPToLSP_ZeroAlloc(t *testing.T) {
	var lsp [10]int16
	for i := 0; i < 10; i++ {
		lsp[i] = lsfToLSP(tables.LSPCodebookL1[0][i])
	}
	var a [11]int16
	LSPToLP(&lsp, &a)

	var q [10]int16
	avg := testing.AllocsPerRun(64, func() {
		_ = LPToLSP(&a, &q)
	})
	if avg != 0 {
		t.Fatalf("LPToLSP allocates %.2f objects/run; want 0", avg)
	}
}
