package lsp

import "github.com/hunydev/g729/internal/tables"

// LSF → LSP conversion per ITU-T G.729 §3.2.5: q_i = cos(ω_i).
//
// The cosine LUT (tables.CosLSP) covers the full range [0, π] with 65
// uniformly-spaced endpoints, so no quadrant folding is needed: a
// monotone increasing ω yields a monotone decreasing q across the
// whole interval. Interpolation keeps the full π_Q13 numerator instead
// of replacing π/64 with floor(25736/64), because that floor error is
// visible in the decoder_tame_lsp_pipeline numeric oracle.
const (
	lspPiQ13    int32 = 25736
	lspStep     int32 = lspPiQ13 / lspNumCells
	lspNumCells int32 = 64
	lspMaxOmega int32 = lspPiQ13
)

// lsfToLSP converts one LSF value ω (Q13, 0 ≤ ω < π) into its LSP
// value cos(ω) (Q15) using tables.CosLSP + linear interpolation.
func lsfToLSP(omega int16) int16 {
	w := int32(omega)
	if w < 0 {
		w = 0
	}
	if w > lspPiQ13 {
		w = lspPiQ13
	}

	pos := w * lspNumCells
	idx := pos / lspPiQ13
	if idx >= lspNumCells {
		idx = lspNumCells - 1
	}
	frac := pos - idx*lspPiQ13

	c0 := int32(tables.CosLSP[idx])
	c1 := int32(tables.CosLSP[idx+1])
	interp := c0 + ((c1-c0)*frac)/lspPiQ13

	if interp > 32767 {
		interp = 32767
	} else if interp < -32768 {
		interp = -32768
	}
	return int16(interp)
}
