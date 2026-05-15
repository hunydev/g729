package lsp

import "github.com/hunydev/g729/internal/tables"

// LSF → LSP conversion per ITU-T G.729 §3.2.5: q_i = cos(ω_i).
//
// The cosine LUT (tables.CosLSP) covers the full range [0, π] with 65
// uniformly-spaced endpoints, so no quadrant folding is needed: a
// monotone increasing ω yields a monotone decreasing q across the
// whole interval. The LSF angle is first scaled by 20861 in Q15 to a
// 14-bit table coordinate; the high six bits select the table cell and
// the low eight bits are the interpolation fraction, as pinned by the
// decoder_tame_lsf_to_lsp numeric oracle.
const (
	lspPiQ13            int32 = 25736
	lspStep             int32 = lspPiQ13 / lspNumCells
	lspNumCells         int32 = 64
	lspMaxOmega         int32 = lspPiQ13
	lspFracBits         int32 = 8
	lspLSFToCosScaleQ15 int32 = 20861
)

func lsfToLSP(omega int16) int16 {
	_, _, _, _, out := lsfToLSPParts(omega)
	return out
}

// lsfToLSPParts converts one LSF value ω (Q13, 0 ≤ ω ≤ π) into its LSP
// value cos(ω) (Q15) and returns the interpolation components used by
// the fixed-point path. The parts are kept unexported but allow oracle
// tests to compare the numeric stage directly.
func lsfToLSPParts(omega int16) (index, frac int16, base, slope, out int16) {
	w := int32(omega)
	if w < 0 {
		w = 0
	}

	q := (w * lspLSFToCosScaleQ15) >> 15
	maxQ := (lspNumCells << lspFracBits) - 1
	if q > maxQ {
		q = maxQ
	}

	idx := q >> lspFracBits
	f := q & ((1 << lspFracBits) - 1)

	c0 := int32(tables.CosLSP[idx])
	m := int32(tables.CosLSPSlope[idx])
	interp := c0 + ((m * f) >> 12)

	if interp > 32767 {
		interp = 32767
	} else if interp < -32768 {
		interp = -32768
	}
	return int16(idx), int16(f), int16(c0), int16(m), int16(interp)
}
