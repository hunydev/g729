package gain

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// pow2Fixed returns 2^(x/1024) as a Q0 Word32 for x given in Q10.
//
// Per ITU-T G.729 §3.9, the predicted excitation gain is reconstructed by
// raising 2 to a Q10 logarithmic value.  We split x as
//
//	x = intPart·1024 + frac, frac ∈ [0, 1024)
//
// and compute 2^x = 2^intPart · 2^(frac/1024).  The fractional factor is
// looked up in tables.Pow2Table, which stores 2^(i/32) in Q14, with a
// 5-bit interpolation residual:
//
//	idx     = frac >> 5
//	a       = frac & 0x1F
//	frac_Q14 = Pow2Table[idx] + (Pow2Table[idx+1] - Pow2Table[idx])·a / 32
//
// The result is then shifted to absorb the 2^intPart factor; values whose
// magnitude underflows Q0 saturate to 0 (this is the desired behavior for
// gain reconstruction where x < 0 corresponds to a vanishing gain).
//
// Q-format CONTRACT: `x` is interpreted as Q10 and the result is a Q0
// Word32. Callers wanting 2^x at some Qk should pre-add k·1024 to `x`
// before the call (e.g. `pow2Fixed(log2Gc_Q10 + 14*1024)` returns
// 2^log2Gc × 2^14 as a Q0 integer, i.e., the value at Q14 stored in Q0).
func pow2Fixed(x fixed.Word32) fixed.Word32 {
	// Arithmetic shift on Word32 (int32 in Go) gives floor division by 1024.
	intPart := int32(x) >> 10
	frac := int32(x) - (intPart << 10)

	idx := frac >> 5
	a := frac & 0x1F
	t0 := int32(tables.Pow2Table[idx])
	t1 := int32(tables.Pow2Table[idx+1])
	fracQ14 := t0 + ((t1-t0)*a)>>5

	// fracQ14 represents 2^(frac/1024) · 2^14, so result = fracQ14 · 2^(intPart-14).
	shift := intPart - 14
	switch {
	case shift >= 0:
		if shift > 16 {
			return fixed.Word32(0x7FFFFFFF)
		}
		return fixed.Word32(fracQ14 << shift)
	default:
		s := -shift
		if s >= 31 {
			return 0
		}
		return fixed.Word32(fracQ14 >> s)
	}
}

// Pow2Fixed is the exported form of pow2Fixed used by the encoder-side
// gain predictor (internal/gainquant). See the unexported form's
// contract: input is Q10 representing the exponent; output is 2^x as
// Q0 Word32.
func Pow2Fixed(x fixed.Word32) fixed.Word32 {
	return pow2Fixed(x)
}
