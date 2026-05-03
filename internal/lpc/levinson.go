package lpc

import "math"

// levinsonDurbin solves the §3.2.2 Yule–Walker normal equations
//
//	Σ_{j=0..10} a_j · r'(|i-j|) = 0    i = 1..10,    a_0 = 1
//
// for the order-10 LP coefficients via the Levinson recursion of
// §3.2.2 lines 717–736:
//
//	E[0] = r'(0)
//	for i = 1..10:
//	    k_i  = -(Σ_{j=0..i-1} a^{i-1}_j · r'(i-j)) / E^{i-1}
//	    a^{i}_i = k_i
//	    for j = 1..i-1: a^{i}_j = a^{i-1}_j + k_i · a^{i-1}_{i-j}
//	    E^{i} = (1 - k_i²) · E^{i-1}
//
// Q-format pinning. r' is Word32 in the AC-1 shared scale (the same
// uniform exponent across r'(0..10) returned by autocorrelate +
// applyLagWindow). The output a[] is Q12 with a[0] = 4096 — chosen
// so the consumer side (internal/lsp/lsp_lp.go's lspToLP) sees the
// same numeric format. Reflection coefficients k_i are kept in Q15
// during the recursion; the prediction error E[i] inherits the r'
// scale (no Q-promotion), since (1 - k²) is dimensionless.
//
// Sum scaling. The accumulator Σ a^{i-1}_j · r'(i-j) carries Q12·rscale
// units. To divide cleanly into a Q15 reflection coefficient we
// pre-shift by 3 bits before the int64 division by E[i-1] (Q15 =
// Q12 << 3). The shifted sum is bounded by 11·2^12·2^31 ≈ 2^46, well
// within int64 range; no intermediate saturation is required.
//
// Stability guard. The spec recursion assumes a positive-definite
// Toeplitz r'(), which guarantees E[i] > 0 and |k_i| < 1. When the
// caller has not (yet) supplied such an input — e.g. an all-zero
// frame producing E[0] = 0 — we abort the i-th stage by setting
// k_i = 0, leaving a[] unchanged and propagating E[i] = E[i-1].
// This keeps the function total without panicking (I3) and yields
// the trivial all-pole filter A(z) = 1 for silence inputs.
//
// Zero allocation: scratch state lives in stack-resident arrays.
func levinsonDurbin(r *[11]int32, a *[11]int16) {
	const order = 10
	const q12one = int32(4096)

	var aWork [order + 1]int32
	var aPrev [order + 1]int32
	aWork[0] = q12one

	e := int64(r[0])

	for i := 1; i <= order; i++ {
		var sum int64
		sum = int64(aWork[0]) * int64(r[i])
		for j := 1; j < i; j++ {
			sum += int64(aWork[j]) * int64(r[i-j])
		}

		var kQ15 int32
		if e > 0 {
			num := -(sum << 3)
			q := num / e
			switch {
			case q > math.MaxInt16:
				kQ15 = math.MaxInt16
			case q < math.MinInt16:
				kQ15 = math.MinInt16
			default:
				kQ15 = int32(q)
			}
		}

		copy(aPrev[:i], aWork[:i])
		for j := 1; j < i; j++ {
			upd := int64(aPrev[j]) + ((int64(kQ15) * int64(aPrev[i-j])) >> 15)
			aWork[j] = saturateInt32(upd)
		}
		aWork[i] = kQ15 >> 3

		kSq := int64(kQ15) * int64(kQ15)
		if kSq > (int64(1) << 30) {
			kSq = int64(1) << 30
		}
		oneMinusKSq := (int64(1) << 30) - kSq
		e = (e * oneMinusKSq) >> 30
	}

	a[0] = 4096
	for j := 1; j <= order; j++ {
		a[j] = saturateInt16(aWork[j])
	}
}

func saturateInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func saturateInt16(v int32) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	if v < math.MinInt16 {
		return math.MinInt16
	}
	return int16(v)
}
