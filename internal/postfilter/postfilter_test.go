package postfilter

import "testing"

func TestPostfilter_ZeroValueIsReset(t *testing.T) {
	var pf Postfilter

	for i, v := range pf.pastS {
		if v != 0 {
			t.Errorf("pastS[%d] = %d, want 0", i, v)
		}
	}
	for i, v := range pf.pastResidual {
		if v != 0 {
			t.Errorf("pastResidual[%d] = %d, want 0", i, v)
		}
	}
	for i, v := range pf.pastSynthPost {
		if v != 0 {
			t.Errorf("pastSynthPost[%d] = %d, want 0", i, v)
		}
	}
	if pf.pastTiltInput != 0 {
		t.Errorf("pastTiltInput = %d, want 0", pf.pastTiltInput)
	}
	if pf.agcGainPrev != 0 {
		t.Errorf("agcGainPrev = %d, want 0", pf.agcGainPrev)
	}
}

func TestPostfilter_ResetZerosState(t *testing.T) {
	var pf Postfilter
	pf.pastS[0] = 123
	pf.pastResidual[50] = -456
	pf.pastSynthPost[9] = 789
	pf.pastTiltInput = 42
	pf.agcGainPrev = 1234

	pf.Reset()

	for i, v := range pf.pastS {
		if v != 0 {
			t.Errorf("after Reset, pastS[%d] = %d, want 0", i, v)
		}
	}
	for i, v := range pf.pastResidual {
		if v != 0 {
			t.Errorf("after Reset, pastResidual[%d] = %d, want 0", i, v)
		}
	}
	for i, v := range pf.pastSynthPost {
		if v != 0 {
			t.Errorf("after Reset, pastSynthPost[%d] = %d, want 0", i, v)
		}
	}
	if pf.pastTiltInput != 0 {
		t.Errorf("after Reset, pastTiltInput = %d, want 0", pf.pastTiltInput)
	}
	if pf.agcGainPrev != 0 {
		t.Errorf("after Reset, agcGainPrev = %d, want 0", pf.agcGainPrev)
	}
}
