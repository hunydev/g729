package postfilter

// computeTiltMu returns the tilt compensation factor μ (Q15) per
// ITU-T G.729 §A.4.2.3.
//
// PLACEHOLDER IMPLEMENTATION. The exact derivation per §A.4.2.3 (k_1 =
// −r_h(1)/r_h(0) of a 22-tap impulse response of A(z/γ_n)/A(z/γ_d), then
// μ = γ_t · k_1) is deferred to Phase 1g, where ITU bit-exact validation
// will pin down the rounding/scaling conventions. Returning 0 here leaves
// the tilt filter as identity, which structurally preserves the chain.
func (pf *Postfilter) computeTiltMu(aNum, aDen *[11]int16) int16 {
	_ = aNum
	_ = aDen
	return 0
}

// applyTiltWithMu applies the one-tap tilt filter
//
//	s_tilt(n) = s_st(n) + μ · s_st(n-1)
//
// per ITU-T G.729 §A.4.2.3.
//
// pastTiltInput holds s_st(-1) on entry; on return, holds s_st(39).
func (pf *Postfilter) applyTiltWithMu(sIn *[subframeLen]int16, muQ15 int16, sOut *[subframeLen]int16) {
	prev := pf.pastTiltInput
	for n := 0; n < subframeLen; n++ {
		prod := int32(muQ15) * int32(prev)
		contrib := int16((prod + (1 << 14)) >> 15)
		sum := int32(sIn[n]) + int32(contrib)
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		sOut[n] = int16(sum)
		prev = sIn[n]
	}
	pf.pastTiltInput = prev
}
