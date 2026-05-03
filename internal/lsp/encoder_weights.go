package lsp

import (
	"github.com/exedev/g729/internal/fixed"
)

// Q-format constants for the eq. (22) adaptive-weight piecewise.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 863–882.
//
//	0.04·π ≈ 0.125663706 → Q13 = 1029   (line 864)
//	0.92·π ≈ 2.890265303 → Q13 = 23676  (line 880)
//	1.0                  → Q13 = 8192   (gap threshold, lines 866/873/881)
//	1.0                  → Q11 = 2048   (eq. 22 unity weight, line 877)
//	1.2                  → Q11 = 2458   (×1.2 boost on w_5/w_6, line 882)
const (
	lsfQ13Pi04   = 1029
	lsfQ13Pi92   = 23676
	lsfQ13One    = 8192
	lsfQ11One    = 2048
	lsfQ11OneTwo = 2458
)

// weightsLSF computes the 10 adaptive distortion weights w_i used by
// the LSF VQ search per ITU-T G.729 §3.2.4 equation (22). Inputs are
// the unquantized LSF coefficients ω_i in Q13 (radians); outputs are
// the weights in Q11 (so 1.0 == 2048).
//
// Branch structure mirrors the spec verbatim (lines 863–882):
//
//	w_1   uses the argument (ω_2 − 0.04π − 1)
//	w_i   (1-based 2..9) uses (ω_{i+1} − ω_{i-1} − 1)
//	w_10  uses (−ω_9 + 0.92π − 1)
//
// In each case, if the argument is > 0 the weight is 1.0; otherwise
// w = 1 / (10·arg² + 1). After all ten weights are formed, w_5 and
// w_6 (1-based; 0-based indices 4 and 5) are multiplied by 1.2 per
// line 882.
//
// Allocation contract (I4): the routine reads omega via pointer,
// writes w via pointer, and uses only stack scalars in between.
func weightsLSF(omega *[10]int16, w *[10]int16) {
	// 1-based i=1: arg = ω_2 − 0.04π − 1, using omega[1].
	w[0] = weightFromArg(fixed.Sub(fixed.Sub(omega[1], lsfQ13Pi04), lsfQ13One))

	// 1-based i = 2..9 → 0-based 1..8: arg = ω_{i+1} − ω_{i-1} − 1.
	for i := 1; i <= 8; i++ {
		gap := fixed.Sub(omega[i+1], omega[i-1])
		w[i] = weightFromArg(fixed.Sub(gap, lsfQ13One))
	}

	// 1-based i=10: arg = −ω_9 + 0.92π − 1, using omega[8].
	w[9] = weightFromArg(fixed.Sub(fixed.Sub(lsfQ13Pi92, omega[8]), lsfQ13One))

	// Per spec line 882: "the weights w_5 and w_6 are each
	// multiplied by 1.2".
	w[4] = boost12(w[4])
	w[5] = boost12(w[5])
}

// weightFromArg evaluates the eq. (22) piecewise on a single Q13
// argument (gap − 1.0 in radians). The "if … > 0" arm returns the
// Q11 unity 2048; the "otherwise" arm returns 1 / (10·arg² + 1) in
// Q11 via a Q26 squared-and-shifted denominator.
//
// Numerics: argQ13 ∈ [-32768, 0] in the otherwise branch (the
// caller guarantees the > 0 branch is filtered out). Squaring lifts
// to Q26 in int64; the additive 1.0 baseline is (1 << 26). The
// final reciprocal is (1 << 37) / d_q26, which gives the Q11
// weight directly:
//
//	w_real  = 1 / d_real           where d_real = d_q26 / 2^26
//	w_q11   = w_real · 2^11        = 2^37 / d_q26
//
// We use plain int64 arithmetic instead of fixed.DivS because the
// numerator (2^37) does not fit DivS's Word16 num<=den precondition;
// the int64 quotient is bounded by [186, 2048] for argQ13 in
// [-8192, 0], well inside Word16.
func weightFromArg(argQ13 int16) int16 {
	if argQ13 > 0 {
		return lsfQ11One
	}
	a := int64(argQ13)
	sqQ26 := a * a
	dQ26 := 10*sqQ26 + (int64(1) << 26)
	wQ11 := (int64(1) << 37) / dQ26
	if wQ11 > 32767 {
		return 32767
	}
	return int16(wQ11)
}

// boost12 multiplies a Q11 weight by 1.2 in place semantics, used
// only for w_5 and w_6 per §3.2.4 line 882. The product is computed
// in Q22 (Q11·Q11) then shifted right by 11 to land back in Q11.
func boost12(wQ11 int16) int16 {
	v := (int32(wQ11) * lsfQ11OneTwo) >> 11
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
