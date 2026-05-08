package closedloop

import "github.com/hunydev/g729/internal/fixed"

// SubframeLen is the §A.3.5 / §A.3.7 subframe length in samples.
const SubframeLen = 40

// gammaPow holds γⁱ for i = 0..10 in Q15 with γ = 0.75 (Annex A).
// The values match internal/pitch/openloop.gammaPow; they are
// duplicated here rather than exported because the openloop helper
// is package-private. A future refactor (see plan §HI-1 step 3) may
// promote a shared helper if TG-1 / CL-1 require it.
//
// Spec: ITU-T G.729 Annex A §A.3.3 line 2063 (γ = 0.75).
var gammaPow = [11]int16{
	32767, // γ⁰ saturated to Q15 cap (1.0)
	24576, // γ¹ = 0.75
	18432, // γ²
	13824, // γ³
	10368, // γ⁴
	7776,  // γ⁵
	5832,  // γ⁶
	4374,  // γ⁷
	3281,  // γ⁸
	2460,  // γ⁹
	1845,  // γ¹⁰
}

// ImpulseResponse computes the 40-tap impulse response h(n) of the
// weighted synthesis filter 1/Â(z/γ) with γ = 0.75, per ITU-T G.729
// Annex A §A.3.5 (G729E.txt lines 2114–2117):
//
//	"The impulse response h(n) ... is computed for each subframe by
//	 filtering a signal consisting of a unit sample extended by zeros
//	 through the filter 1/Â(z/γ)."
//
// Inputs and outputs are Q12. aHatQ12[0] must equal 4096 (= 1.0 in
// Q12) per ITU LP-polynomial convention; the recurrence assumes
// aw[0] = 4096 and divides by it via an arithmetic shift.
//
// Filter form. Annex A's perceptual weighting in §A.3.3 applies
// (1 − 0.7z⁻¹) only to the residual / target chain (eq. A.2); the
// impulse-response filter in §A.3.5 is the bare all-pole 1/Â(z/γ),
// not 1/[Â(z/γ)·(1 − 0.7z⁻¹)]. This is the documented difference
// between the Annex A reduced-complexity weighting and the §3.7
// base-codec form Â(z/γ₁)/[Â(z)·Â(z/γ₂)].
//
// Algorithm (ITU Syn_filt convention, Q12 in / Q12 out):
//
//	aw[i] = γⁱ · aHatQ12[i]            // Q12, leading tap 4096
//	h[0]  = 1.0 (= 4096 in Q12)        // δ[0] divided by aw[0]
//	h[n]  = -Σ_{i=1..min(n,10)} aw[i]·h[n-i] / aw[0]   (n ≥ 1)
//
// The fixed-point recurrence accumulates aw[i]·h[n-i] in Q25 via
// LMsu (which doubles the Q12·Q12 product), shifts the accumulator
// left by 3 to Q28, then rounds and extracts the high half to land
// in Q12. This matches the standard ITU Syn_filt scaling.
//
// I3 / I4: pure (writes only through h), zero allocation. A static
// 11-element gamma-weighted coefficient buffer is held on the stack.
func ImpulseResponse(aHatQ12 *[11]int16, h *[SubframeLen]int16) {
	var aw [11]int16
	aw[0] = aHatQ12[0]
	for i := 1; i <= 10; i++ {
		aw[i] = fixed.Mult(aHatQ12[i], gammaPow[i])
	}

	for n := 0; n < SubframeLen; n++ {
		var acc fixed.Word32
		if n == 0 {
			// Input is the unit sample: x[0] = 1.0 in Q12.
			// L_mult(4096, 4096) = 2 · 4096 · 4096 in Q25.
			acc = fixed.LMult(4096, aw[0])
		}
		limit := n
		if limit > 10 {
			limit = 10
		}
		for i := 1; i <= limit; i++ {
			acc = fixed.LMsu(acc, aw[i], h[n-i])
		}
		// Q25 → Q28 (×8) so Round's >>16 lands in Q12.
		acc = fixed.LShl(acc, 3)
		h[n] = fixed.Round(acc)
	}
}
