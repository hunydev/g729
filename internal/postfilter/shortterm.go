package postfilter

import "github.com/exedev/g729/internal/fixed"

// applyShortTerm runs the short-term postfilter IIR
//
//	s_st(n) = r'(n) − Σ_{i=1..10} aDen[i] · s_st(n−i)
//
// per ITU-T G.729 §A.4.2.1, using pastSynthPost as the 10-tap IIR memory.
//
// Arithmetic parallels synth.filterSubframe: Q13 accumulator, LShl by 3,
// Round to Q0 Word16. Different state array and different coefficients.
func (pf *Postfilter) applyShortTerm(aDen *[11]int16, rIn *[subframeLen]int16, sOut *[subframeLen]int16) {
	var work [lpcOrder + subframeLen]int16
	copy(work[:lpcOrder], pf.pastSynthPost[:])

	for n := 0; n < subframeLen; n++ {
		lTemp := fixed.LMult(rIn[n], aDen[0])
		for i := 1; i <= lpcOrder; i++ {
			lTemp = fixed.LMsu(lTemp, aDen[i], work[lpcOrder+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[lpcOrder+n] = fixed.Round(lTemp)
	}

	copy(sOut[:], work[lpcOrder:])
	copy(pf.pastSynthPost[:], work[subframeLen:])
}
