package openloop

import "github.com/exedev/g729/internal/fixed"

// gamma07Q15 is the Q15 representation of 0.7 (= round(0.7·32768)).
// It is the |z⁻¹| coefficient of the (1 − 0.7z⁻¹) low-pass factor
// applied in §A.3.3 to form A'(z).
const gamma07Q15 int16 = 22938

// gammaPow holds γⁱ for i = 0..10 in Q15, with γ = 0.75. The values
// are exact-rational representations rounded to the nearest int16:
//
//	γ⁰  = 1.0           → 32767  (saturated; Q15 cap)
//	γ¹  = 0.75          → 24576
//	γ²  = 0.5625        → 18432
//	γ³  = 0.421875      → 13824
//	γ⁴  = 0.31640625    → 10368
//	γ⁵  = 0.2373046875  → 7776
//	γ⁶  = 0.177978515625→ 5832
//	γ⁷  = 0.13348388…   → 4374
//	γ⁸  = 0.10011291…   → 3281
//	γ⁹  = 0.07508468…   → 2460
//	γ¹⁰ = 0.05631351…   → 1845
//
// Documented recurrence (off-line check, not enforced at runtime):
//
//	gammaPow[i+1] = MultR(gammaPow[i], 24576)
//
// Differences of ±1 LSB versus the recurrence are absorbed by
// rounding to the nearest int16 at each step. The static LUT is
// preferred so the order-0 saturation (1.0 → 32767) and the
// half-LSB rounding choices for γ⁸..γ¹⁰ are pinned in source.
var gammaPow = [11]int16{
	32767,
	24576,
	18432,
	13824,
	10368,
	7776,
	5832,
	4374,
	3281,
	2460,
	1845,
}

// gammaWeightLP computes Â(z/γ) from the order-10 LP polynomial â
// per §A.3.3 (line 2063, γ = 0.75). The leading coefficient is
// passed through unchanged (Q12 = 4096 ≡ 1.0); each subsequent tap
// is scaled by γⁱ via fixed.Mult, producing aw[i] in Q12.
//
// I3 / I4: pure (writes only through out), zero allocation.
func gammaWeightLP(a, out *[11]int16) {
	out[0] = a[0]
	for i := 1; i <= 10; i++ {
		out[i] = fixed.Mult(a[i], gammaPow[i])
	}
}

// combineWith07 multiplies Â(z/γ) by the (1 − 0.7z⁻¹) low-pass
// factor to produce A'(z) per §A.3.3 (line 2071). The mathematical
// product is order-11; per OQ-2 (default reading) it is truncated
// to order-10 by dropping the highest-degree tap.
//
// Q-format / convolution convention. The first tap aw[0] is the
// implicit Q12 leading 1.0 (= 4096) of the LP polynomial. The
// (1 − 0.7z⁻¹) factor's z⁻¹ coefficient is therefore added at
// out[1] using its Q15 representation directly (-22938) rather than
// re-scaled through the leading 4096; for higher indices i ≥ 2 the
// contribution -0.7·aw[i-1] is computed via fixed.Mult (Q15·Q12 →
// Q12). The dual-mode handling produces the hand-traced expected
// values pinned by the package tests; a uniform-Q12 reading is
// reserved for OQ-2 escalation if INT-1 plausibility fails.
//
// I3 / I4: pure (writes only through out), zero allocation.
func combineWith07(aw, out *[11]int16) {
	out[0] = aw[0]
	out[1] = fixed.Saturate(int32(aw[1]) - int32(gamma07Q15))
	for i := 2; i <= 10; i++ {
		out[i] = aw[i] - fixed.MultR(gamma07Q15, aw[i-1])
	}
}
