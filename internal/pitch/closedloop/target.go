package closedloop

import "github.com/hunydev/g729/internal/fixed"

// TargetSignal computes the 40-sample adaptive-codebook target
// signal x(n) per ITU-T G.729 Annex A §A.3.6 (G729E.txt lines
// 2119–2125):
//
//	"The target signal x(n) for the adaptive-codebook search is
//	 computed by filtering of the LP residual signal r(n) through
//	 the weighted synthesis filter 1/Â(z/γ)."
//
// Spec form chosen. Annex A's §A.3.6 explicitly picks the
// "filter the residual through 1/Â(z/γ)" formulation as the
// equivalent — and simpler — reformulation of the base-codec §3.6
// "subtract zero-input response of W(z)/Â(z) from sw(n)" definition
// (G729E.txt lines 1058–1066). Annex A's perceptual weighting in
// §A.3.3 collapses the base codec's W(z) = A(z/γ₁)/A(z/γ₂) to a
// single 1/Â(z/γ) all-pole stage with γ = 0.75; both forms produce
// the same numerical x(n) when memories are aligned. The all-pole
// reformulation is the one HI-1 already mirrors for h(n), so the
// closed-loop search and TG-1 share the same arithmetic primitives.
//
// Algorithm. With aw[i] = γⁱ·â[i] (i = 1..10) and aw[0] = â[0] =
// 4096 (Q12 LP-polynomial leading-tap convention), the all-pole
// recurrence
//
//	x(n) = r(n) − Σ_{i=1..10} aw[i] · x(n − i),   n = 0,...,39
//
// produces x(n) one sample at a time. Past output samples
// x(n − i) for i > n resolve to swMem[10 + n − i], which holds
// the trailing 10 samples of the previous subframe's x(n) (or
// zero on cold-start). This mirrors the synthesis-filter Q12
// convention: accumulate products with LMult / LMsu, shift left by 3,
// then Round so aw[0] = 4096 behaves as an exact unity coefficient.
//
// Q-format. r and x are int16 in the same scale (Q0 in the encoder
// pipeline; the absolute scale is irrelevant to TargetSignal — it
// simply propagates whatever Q the residual carries). aHatQ12 is
// Q12 with the leading 1.0 stored as 4096 at index 0; gamma
// weighting is applied via fixed.Mult(aHat[i], gammaPow[i]) in Q12.
// Since aw[0] = 4096 is the implicit leading-coefficient identity,
// the recurrence does not divide by aw[0] (mirroring the
// lowpassWeightedSpeech precedent in openloop/lowpass.go).
//
// Memory contract (I3). swMem is treated as read-only inside this
// function. The encoder driver is responsible for advancing it once
// per subframe via the §A.3.10 weighted-error update (eq. A.10):
//
//	ew(n) = x(n) − ĝ_p · y(n) − ĝ_c · z(n),  n = 30,...,39
//
// (G729E.txt lines 2202–2215). Holding swMem read-only here keeps
// TargetSignal pure with respect to the per-subframe state machine;
// see plan §TG-1 step 3 and Phase 2c invariant I3.
//
// I4. Zero allocation: a static 11-element gamma-weighted coefficient
// buffer lives on the stack; no slices are taken or returned.
func TargetSignal(aHatQ12 *[11]int16, residual *[SubframeLen]int16, swMem *[10]int16, x *[SubframeLen]int16) {
	var aw [11]int16
	aw[0] = aHatQ12[0]
	for i := 1; i <= 10; i++ {
		aw[i] = fixed.Mult(aHatQ12[i], gammaPow[i])
	}

	for n := 0; n < SubframeLen; n++ {
		acc := fixed.LMult(residual[n], aw[0])
		for i := 1; i <= 10; i++ {
			var xni int16
			if n-i >= 0 {
				xni = x[n-i]
			} else {
				xni = swMem[10+n-i]
			}
			acc = fixed.LMsu(acc, aw[i], xni)
		}
		x[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}
