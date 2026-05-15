package gain

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
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

	fracQ14 := int32(pow2FracQ14(frac))

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

// Pow2FracQ14 is the exported form of pow2FracQ14: returns 2^(frac/1024)
// at Q14 (an int16 in [16384, 32767] representing [1.0, 2.0)). Used by
// the encoder-side dequantizer (internal/gainquant) to mirror the
// decoder mantissa split bit-for-bit per REF-1 §2. `frac` MUST be in
// [0, 1024); see pow2FracQ14 for the table-lookup contract.
func Pow2FracQ14(frac int32) int16 {
	return pow2FracQ14(frac)
}

func Pow2FracQ14FromQ15(frac int32) int16 {
	return pow2FracQ14FromQ15(frac)
}

func LogGainToLog2Q15(logGainDbQ10 int32) int32 {
	return logGainToLog2Q15(logGainDbQ10)
}

func FixedGainQ14FromLog2Gamma(log2GcQ15 int32, gammaCQ13 int32) int64 {
	return fixedGainQ14FromLog2Gamma(log2GcQ15, gammaCQ13)
}

// QuantizeFixedGainQ1 mirrors the decoder's final fixed-codebook gain
// quantization before the mantissa/exponent split.
func QuantizeFixedGainQ1(gainQ14 int64) int64 {
	return quantizeFixedGainQ1(gainQ14)
}

func SplitGainQ14(gainQ14 int64) (mant int16, exp int8) {
	return splitGainQ14(gainQ14)
}

// pow2FracQ14 returns 2^(frac/1024) at Q14 — i.e. an int16 in
// [16384, 32767] representing a value in [1.0, 2.0). `frac` MUST be in
// [0, 1024); callers obtain it as the low 10 bits of a Q10 exponent.
//
// This is the table lookup + 5-bit linear interpolation that was
// previously inlined in pow2Fixed; both pow2Fixed and Decode (mantissa
// path) now share it so the Pow2Table semantics are pinned in one
// place.
func pow2FracQ14(frac int32) int16 {
	idx := frac >> 5
	a := frac & 0x1F
	t0 := int32(tables.Pow2Table[idx])
	t1 := int32(tables.Pow2Table[idx+1])
	return int16(t0 + ((t1-t0)*a)>>5)
}

func pow2FracQ14FromQ15(frac int32) int16 {
	idx := frac >> 10
	a := frac & 0x3FF
	t0 := int32(tables.Pow2Table[idx])
	t1 := int32(tables.Pow2Table[idx+1])
	return int16(t0 + ((t1-t0)*a)>>10)
}

func logGainToLog2Q15(logGainDbQ10 int32) int32 {
	return (logGainDbQ10 * invDbScaleQ15) >> 10
}

func fixedGainQ14FromLog2Gamma(log2GcQ15 int32, gammaCQ13 int32) int64 {
	intPart := log2GcQ15 >> 15
	fracQ15 := log2GcQ15 - (intPart << 15)
	gc0Q14 := int64(pow2FracQ14FromQ15(fracQ15))
	baseQ14 := (int64(gammaCQ13) * gc0Q14) >> 13
	if intPart >= 0 {
		if intPart >= 62 {
			return int64(0x7FFFFFFFFFFFFFFF)
		}
		return baseQ14 << uint(intPart)
	}
	shift := -intPart
	if shift >= 63 {
		return 0
	}
	return baseQ14 >> uint(shift)
}

func quantizeFixedGainQ1(gainQ14 int64) int64 {
	if gainQ14 <= 0 {
		return 0
	}
	gainQ1 := gainQ14 >> 13
	if gainQ1 > 32767 {
		gainQ1 = 32767
	}
	return gainQ1 << 13
}

func splitGainQ14(gainQ14 int64) (mant int16, exp int8) {
	if gainQ14 <= 0 {
		return 0, 0
	}
	var e int
	for gainQ14 > 32767 && e < 127 {
		gainQ14 >>= 1
		e++
	}
	for gainQ14 < 16384 && e > -128 {
		gainQ14 <<= 1
		e--
	}
	return int16(gainQ14), int8(e)
}
