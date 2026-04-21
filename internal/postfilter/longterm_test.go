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
