package gain

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// log2Fixed returns log2(x) in Q10 for x > 0; returns 0 for x <= 0.
//
// Algorithm (per ITU-T G.729 §3.9, computing the binary logarithm of a
// strictly positive Q0 Word32):
//
//  1. Normalize x so its MSB lies at bit position 30 (NormL gives the
//     left-shift count s). Then x·2^s ∈ [2^30, 2^31).
//
//  2. Integer part: log2(x) = (30 - s) + log2(mantissa) with
//     mantissa = (x·2^s)/2^30 ∈ [1, 2).
//
//  3. Express mantissa as 1 + f, f ∈ [0, 1). Split the 30-bit
//     fractional region into a 5-bit table index and a 15-bit
//     interpolation residual:
//
//     tables.Log2Table[i]   ≈ log2(1 + i/32) · 2¹⁵
//     frac_Q15 ≈ Log2Table[i] + (Log2Table[i+1] - Log2Table[i])·a/32768
//
//  4. Combine: result_Q10 = (intPart << 10) + (frac_Q15 >> 5).
//
// Accuracy: ±2 LSB at Q10 across the table interior (verified against
// the closed form), exact at the 33 tabulated breakpoints.
//
// Q-format CONTRACT: this function treats `x` as a Q0 integer and
// returns log2(x) at Q10. If a caller passes a Qk value (k > 0) as
// `x`, the returned log2 is off by k·1024 (log2(value·2^k) = log2(value)
// + k). Callers with a Qk input MUST subtract k·1024 from the result
// to recover the spec-intended log2. See decode.go's ecLog2Q10 handling.
func log2Fixed(x fixed.Word32) fixed.Word32 {
	return log2FixedQ15(x) >> 5
}

// log2FixedQ15 is the higher-precision form of log2Fixed. It returns
// log2(x) in Q15 for x > 0, preserving the table interpolation output before
// the legacy Q10 downshift. The decoder gain path uses this to avoid losing
// low bits before the dB-domain multiply.
func log2FixedQ15(x fixed.Word32) fixed.Word32 {
	if x <= 0 {
		return 0
	}

	s := fixed.NormL(x)
	normX := fixed.LShl(x, s)
	intPart := fixed.Word32(30) - fixed.Word32(s)

	frac30 := int64(normX) - (1 << 30)
	idx := fixed.Word32(frac30 >> 25)
	a := fixed.Word32((frac30 >> 10) & 0x7FFF)

	t0 := fixed.Word32(tables.Log2Table[idx])
	t1 := fixed.Word32(tables.Log2Table[idx+1])
	fracLog2Q15 := t0 + ((t1-t0)*a)>>15

	return (intPart << 15) + fracLog2Q15
}

// Log2Fixed is the exported form of log2Fixed used by the encoder-side
// gain predictor (internal/gainquant). See the unexported form's
// contract: input treated as Q0; output is log2(x) at Q10.
func Log2Fixed(x fixed.Word32) fixed.Word32 {
	return log2Fixed(x)
}

// Log2FixedQ15 is the exported high-precision form used by gain-domain
// diagnostics and encoder-side predictors that need to mirror receiver
// reconstruction without the early Q10 truncation.
func Log2FixedQ15(x fixed.Word32) fixed.Word32 {
	return log2FixedQ15(x)
}
