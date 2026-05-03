package gain

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// log2Fixed returns log2(x) in Q10 for x > 0; returns 0 for x <= 0.
//
// Algorithm (per ITU-T G.729 §3.9, computing the binary logarithm of a
// strictly positive Q0 Word32):
//
//  1. Normalize x so its MSB lies at bit position 30 (NormL gives the
//     left-shift count s). Then x·2^s ∈ [2^30, 2^31).
//  2. Integer part: log2(x) = (30 - s) + log2(mantissa) with
//     mantissa = (x·2^s)/2^30 ∈ [1, 2).
//  3. Express mantissa as 1 + f, f ∈ [0, 1).  The top 15 bits of the
//     30-bit fractional region give f in Q15.  Split f into a 5-bit
//     index i and a 10-bit interpolation residual a:
//
//         tables.Log2Table[i]   ≈ log2(1 + i/32) · 2¹⁵
//         frac_Q15 ≈ Log2Table[i] + (Log2Table[i+1] - Log2Table[i])·a/1024
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
	if x <= 0 {
		return 0
	}

	s := fixed.NormL(x)
	normX := fixed.LShl(x, s)
	intPart := fixed.Word32(30) - fixed.Word32(s)

	// Top 15 bits of the fractional part of normX (post-leading-1).
	// normX ∈ [2^30, 2^31), so (normX >> 15) ∈ [2^15, 2^16); subtracting
	// 2^15 strips the implicit leading bit, yielding f in Q15.
	frac15 := fixed.Word32((int32(normX) >> 15) & 0x7FFF)
	idx := frac15 >> 10
	a := frac15 & 0x3FF

	t0 := fixed.Word32(tables.Log2Table[idx])
	t1 := fixed.Word32(tables.Log2Table[idx+1])
	fracLog2Q15 := t0 + ((t1-t0)*a)>>10

	return (intPart << 10) + (fracLog2Q15 >> 5)
}

// Log2Fixed is the exported form of log2Fixed used by the encoder-side
// gain predictor (internal/gainquant). See the unexported form's
// contract: input treated as Q0; output is log2(x) at Q10.
func Log2Fixed(x fixed.Word32) fixed.Word32 {
	return log2Fixed(x)
}
