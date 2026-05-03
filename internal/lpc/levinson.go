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
// so the consumer side (internal/lsp/lsp_lp.go's LSPToLP) sees the
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
// Internal aWork precision (FIX-1B). The intermediate predictor state
// aWork/aPrev is carried in Q24 (int64) — 12 extra fractional bits
// above the Q12 produced at write-out. The inner update
//
//	aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j]) >> 15
//
// is Q24 + (Q15 · Q24) >> 15 = Q24, preserving fractional precision
// the prior Q12 storage truncated. The numerator sum is computed at
// the production Q12 width by round-shifting aWork (Q24) → Q12 just
// before each multiply, keeping the divide arithmetic and reflection
// coefficient bit-exact with the historical fixed-point recursion.
// Final write-out a[j] = saturate(round(aWork[j] >> 12)) yields Q12.
// See docs/superpowers/plans/2026-05-04-phase2a-int1-d4-pinpoint-plan.md
// §13–14 for the d5 validation that motivated this widening.
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
	const oneQ24 = int64(1) << 24

	var aWork [order + 1]int64
	var aPrev [order + 1]int64
	aWork[0] = oneQ24

	e := int64(r[0])

	for i := 1; i <= order; i++ {
		// Sum at production Q12 width. Round-shift aWork (Q24) → Q12
		// just before each multiply so the divide arithmetic remains
		// bit-identical to the historical fixed-point recursion.
		var sum int64
		sum = q24ToQ12Round(aWork[0]) * int64(r[i])
		for j := 1; j < i; j++ {
			sum += q24ToQ12Round(aWork[j]) * int64(r[i-j])
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
		// Inner update at full Q24 precision:
		//   aPrev[j] (Q24) + (kQ15 (Q15) · aPrev[i-j] (Q24)) >> 15 = Q24.
		for j := 1; j < i; j++ {
			aWork[j] = aPrev[j] + (int64(kQ15)*aPrev[i-j])>>15
		}
		// New aWork[i] = k_i in Q24 (kQ15 << 9).
		aWork[i] = int64(kQ15) << 9

		kSq := int64(kQ15) * int64(kQ15)
		if kSq > (int64(1) << 30) {
			kSq = int64(1) << 30
		}
		oneMinusKSq := (int64(1) << 30) - kSq
		e = (e * oneMinusKSq) >> 30
	}

	a[0] = 4096
	for j := 1; j <= order; j++ {
		a[j] = saturateInt16(q24ToQ12Round(aWork[j]))
	}
}

// q24ToQ12Round round-shifts a Q24 int64 down to Q12 with signed
// half-away-from-zero rounding to avoid systematic bias toward
// either rail in the sum accumulator and the final write-out.
func q24ToQ12Round(v int64) int64 {
	if v >= 0 {
		return (v + (1 << 11)) >> 12
	}
	return -((-v + (1 << 11)) >> 12)
}

func saturateInt16(v int64) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	if v < math.MinInt16 {
		return math.MinInt16
	}
	return int16(v)
}
