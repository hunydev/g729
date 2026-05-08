package postfilter

// Annex A postfilter constants per ITU-T G.729 §A.4.2.
const (
	gammaNumQ15 int16 = 18022 // γ_n ≈ 0.55
	gammaDenQ15 int16 = 22938 // γ_d ≈ 0.70
)

// Filter runs the full Annex A postfilter chain on one subframe per
// ITU-T G.729 §A.4.2.
//
// Inputs:
//
//	a    — LP filter coefficients for this subframe (Q12, a[0] = 4096)
//	tInt — integer pitch delay decoded by internal/pitch
//	s    — pre-postfilter synthesis from internal/synth (Q0)
//
// Output:
//
//	sPf  — postfiltered samples (Q0)
//
// Updates all Postfilter state fields; zero-allocation.
func (pf *Postfilter) Filter(a *[11]int16, tInt int, s *[subframeLen]int16, sPf *[subframeLen]int16) {
	var aNum, aDen [11]int16
	expandBandwidth(a, gammaNumQ15, &aNum)
	expandBandwidth(a, gammaDenQ15, &aDen)

	var r [subframeLen]int16
	pf.computeResidual(&aNum, s, &r)

	// Slide pastResidual left by subframeLen, write current r at the tail
	// so refinePitch and applyLongTerm can index r(n-T) for T ∈ [20, 143].
	copy(pf.pastResidual[:pitchMax], pf.pastResidual[subframeLen:])
	copy(pf.pastResidual[pitchMax:], r[:])

	T := pf.refinePitch(&r, tInt)

	var rOut [subframeLen]int16
	g0, g1 := pf.computeLongTermGain(&r, T)
	pf.applyLongTermWithGains(T, g0, g1, &rOut)

	var sSt [subframeLen]int16
	pf.applyShortTerm(&aDen, &rOut, &sSt)

	muQ15 := pf.computeTiltMuForLongTerm(&aNum, &aDen, g1 != 0)
	var sTilt [subframeLen]int16
	pf.applyTiltWithMu(&sSt, muQ15, &sTilt)

	gTarget := pf.computeAGCTargetGain(s, &sTilt)
	pf.applyAGC(&sTilt, gTarget, sPf)
}
