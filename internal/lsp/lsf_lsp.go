package lsp

import "github.com/hunydev/g729/internal/tables"

// LSF → LSP conversion per ITU-T G.729 §3.2.5: q_i = cos(ω_i).
//
// The cosine LUT (tables.CosLSP) covers the full range [0, π] with 65
// uniformly-spaced endpoints, so no quadrant folding is needed: a
// monotone increasing ω yields a monotone decreasing q across the
// whole interval. Each LUT interval is π/64 wide, which in Q13 is
// 25736/64 = 402.125 — we use floor (lspStep = 402) and absorb the
// sub-unit residual in the linear interpolation.
const (
	lspStep     int32 = 402 // floor(π_Q13 / 64)
	lspNumCells int32 = 64
	lspMaxOmega int32 = lspStep * lspNumCells // 25728, just under π_Q13 = 25736
)

// lsfToLSP converts one LSF value ω (Q13, 0 ≤ ω < π) into its LSP
// value cos(ω) (Q15) using tables.CosLSP + linear interpolation.
func lsfToLSP(omega int16) int16 {
	w := int32(omega)
	if w < 0 {
		w = 0
	}
	if w > lspMaxOmega {
		w = lspMaxOmega
	}

	idx := w / lspStep
	if idx >= lspNumCells {
		idx = lspNumCells - 1
	}
	frac := w - idx*lspStep

	c0 := int32(tables.CosLSP[idx])
	c1 := int32(tables.CosLSP[idx+1])
	interp := c0 + ((c1-c0)*frac)/lspStep

	if interp > 32767 {
		interp = 32767
	} else if interp < -32768 {
		interp = -32768
	}
	return int16(interp)
}
