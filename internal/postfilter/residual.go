package postfilter

import "github.com/exedev/g729/internal/fixed"

// computeResidual applies the bandwidth-expanded FIR
//
//	r(n) = Σ_{i=0..10} aNum[i] · s(n−i)
//
// per ITU-T G.729 §A.4.2.1, updating pastS for the next subframe.
//
// Q-formats: aNum Q12, s Q0, r Q0.
//
// State: pastS holds s(n-10..n-1) on entry; on return, pastS holds
// s(30..39) for the next subframe.
func (pf *Postfilter) computeResidual(aNum *[11]int16, s, r *[subframeLen]int16) {
	var work [lpcOrder + subframeLen]int16
	copy(work[:lpcOrder], pf.pastS[:])
	copy(work[lpcOrder:], s[:])

	for n := 0; n < subframeLen; n++ {
		lTemp := fixed.LMult(aNum[0], work[lpcOrder+n])
		for i := 1; i <= lpcOrder; i++ {
			lTemp = fixed.LMac(lTemp, aNum[i], work[lpcOrder+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		r[n] = fixed.Round(lTemp)
	}

	copy(pf.pastS[:], work[subframeLen:])
}
