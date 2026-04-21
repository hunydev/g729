package postfilter

import "testing"

func TestRefinePitch_LocksToTruePeriod(t *testing.T) {
	var pf Postfilter

	for i := range pf.pastResidual {
		if (i/15)%2 == 0 {
			pf.pastResidual[i] = 1000
		} else {
			pf.pastResidual[i] = -1000
		}
	}
	var r [subframeLen]int16
	for i := range r {
		if (i/15)%2 == 0 {
			r[i] = 1000
		} else {
			r[i] = -1000
		}
	}
	copy(pf.pastResidual[pitchMax:], r[:])

	bestT := pf.refinePitch(&r, 30)
	if bestT != 30 {
		t.Errorf("bestT = %d, want 30", bestT)
	}
}

func TestRefinePitch_ZeroSignalFallsBackToTInt(t *testing.T) {
	var pf Postfilter
	var r [subframeLen]int16

	bestT := pf.refinePitch(&r, 55)
	if bestT != 55 {
		t.Errorf("bestT = %d, want 55 (fallback to t_int)", bestT)
	}
}

func TestRefinePitch_ClampsAtLowerEdge(t *testing.T) {
	var pf Postfilter
	var r [subframeLen]int16

	_ = pf.refinePitch(&r, 20)
}

func TestRefinePitch_ClampsAtUpperEdge(t *testing.T) {
	var pf Postfilter
	var r [subframeLen]int16

	_ = pf.refinePitch(&r, 143)
}

// With g_l = 0 (zero correlation), long-term postfilter is identity.
func TestApplyLongTerm_ZeroGainIsIdentity(t *testing.T) {
	var pf Postfilter
	var r, rOut [subframeLen]int16
	for i := range r {
		r[i] = int16(100 + i)
	}
	// Leave pastResidual zero so R = 0 → g_l = 0.
	copy(pf.pastResidual[pitchMax:], r[:])

	pf.applyLongTerm(&r, 40, &rOut)

	for i := range rOut {
		if rOut[i] != r[i] {
			t.Errorf("rOut[%d] = %d, want %d (zero gain is identity)",
				i, rOut[i], r[i])
		}
	}
}

func TestApplyLongTerm_PeriodicSignalPreserved(t *testing.T) {
	var pf Postfilter
	const T = 30
	// Period-T square wave aligned with r's frame-of-reference: index n
	// (relative to the current subframe start) maps to mod = ((n%T)+T)%T.
	// Fill past samples (n ∈ [-pitchMax, 0)) and current r (n ∈ [0,
	// subframeLen)) using the same formula so r(n) ≡ r(n-T) holds.
	fill := func(n int) int16 {
		mod := ((n % T) + T) % T
		if mod < 15 {
			return -1000
		}
		return 1000
	}
	for i := range pf.pastResidual {
		pf.pastResidual[i] = fill(i - pitchMax)
	}
	var r [subframeLen]int16
	for i := range r {
		r[i] = fill(i)
	}
	copy(pf.pastResidual[pitchMax:], r[:])

	var rOut [subframeLen]int16
	pf.applyLongTerm(&r, T, &rOut)

	for i := range rOut {
		if rOut[i] < r[i]-1 || rOut[i] > r[i]+1 {
			t.Errorf("rOut[%d] = %d, want %d (±1)", i, rOut[i], r[i])
		}
	}
}

func sign(x int) int16 {
	if x >= 0 {
		return 1
	}
	return -1
}
