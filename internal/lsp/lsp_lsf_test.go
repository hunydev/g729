package lsp

import (
	"testing"

	"github.com/hunydev/g729/internal/tables"
)

// TestLSPToLSFInvertsLSFToLSP verifies that lspToLSF remains a close
// inverse of lsfToLSP across the full Q13 LSF domain. The forward path
// scales by 20861 in Q15 into a 14-bit table coordinate, so nominal
// π/64 cell boundaries are not all exactly representable; the ±8 LSB
// tolerance covers that endpoint quantization.
func TestLSPToLSFInvertsLSFToLSP(t *testing.T) {
	for k := int32(0); k <= 64; k++ {
		omega := k * lspStep
		if omega > lspMaxOmega {
			omega = lspMaxOmega
		}
		q := lsfToLSP(int16(omega))
		got := lspToLSF(q)
		diff := int32(got) - omega
		if diff < 0 {
			diff = -diff
		}
		const tol = 8
		if diff > tol {
			t.Errorf("k=%d ω=%d → q=%d → ω'=%d (diff=%d, want ≤%d)", k, omega, q, got, diff, tol)
		}
	}
}

// TestLSPToLSFInterior pins the inverse on Q13 LSF arguments at
// fractional positions inside CosLSP cells. The tolerance is
// ⌈lspStep/|Δc|⌉ + 1 LSB: the forward map's integer division loses
// up to that many ω-LSBs at the most-tilted cells (idx 0 and 63
// near cos ≈ ±1 have |Δc| ≈ 38, giving tol ≈ 12; interior cells
// with |Δc| ≈ 1500 give tol ≈ 1).
func TestLSPToLSFInterior(t *testing.T) {
	for k := int32(0); k < 64; k++ {
		omega := k*lspStep + lspStep/2
		q := lsfToLSP(int16(omega))
		got := lspToLSF(q)
		diff := int32(got) - omega
		if diff < 0 {
			diff = -diff
		}
		dc := int32(tables.CosLSP[k]) - int32(tables.CosLSP[k+1])
		if dc <= 0 {
			dc = 1
		}
		tol := lspStep/dc + 1
		if diff > tol {
			t.Errorf("k=%d ω=%d → q=%d → ω'=%d (diff=%d, want ≤%d)", k, omega, q, got, diff, tol)
		}
	}
}

func TestLSPToLSFBoundaryClamps(t *testing.T) {
	if got := lspToLSF(32767); got != 0 {
		t.Errorf("lspToLSF(+max) = %d, want 0", got)
	}
	if got := lspToLSF(-32768); got != int16(lspMaxOmega) {
		t.Errorf("lspToLSF(-max) = %d, want %d", got, lspMaxOmega)
	}
}

// TestLSPToLSFMonotonic asserts that lspToLSF is monotone
// non-increasing in q (matching the CosLSP table's monotone
// non-increasing shape over [0, π]).
func TestLSPToLSFMonotonic(t *testing.T) {
	prev := int16(-1)
	for q := int32(32767); q >= -32768; q -= 1024 {
		got := lspToLSF(int16(q))
		if prev >= 0 && got < prev {
			t.Errorf("non-monotone at q=%d: got %d < prev %d", q, got, prev)
		}
		prev = got
	}
}
