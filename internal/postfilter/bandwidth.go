package postfilter

// expandBandwidth computes a_scaled[i] = γ^i · a[i] for i ∈ [0, 10]
// per ITU-T G.729 §3.10.1 / §A.4.2.1.
//
// Q-formats:
//
//	a        [11]int16, Q12; a[0] = 4096
//	gammaQ15 Word16, Q15 (γ = gammaQ15 / 32768)
//	out      [11]int16, Q12; out[0] = 4096 (filter normalisation preserved)
//
// Implementation: iteratively accumulate γ^i in gammaPow (Q15) and
// multiply each a[i] by gammaPow. The product a[i]·gammaPow at Q12·Q15
// is rounded back to Q12 with (·+2^14)>>15.
func expandBandwidth(a *[11]int16, gammaQ15 int16, out *[11]int16) {
	out[0] = a[0]
	gammaPow := gammaQ15
	for i := 1; i <= 10; i++ {
		prod := int32(a[i]) * int32(gammaPow)
		out[i] = int16((prod + (1 << 14)) >> 15)

		gp := int32(gammaPow) * int32(gammaQ15)
		gammaPow = int16((gp + (1 << 14)) >> 15)
	}
}
