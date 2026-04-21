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

// End-to-end smoke: Filter with zero input must produce zero output.
func TestFilter_ZeroInputZeroOutput(t *testing.T) {
	var pf Postfilter
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, sPf [subframeLen]int16

	pf.Filter(&a, 40, &s, &sPf)

	for i := range sPf {
		if sPf[i] != 0 {
			t.Errorf("sPf[%d] = %d, want 0 (zero input)", i, sPf[i])
		}
	}
}

func TestFilter_ZeroLPCIsApproximateIdentity(t *testing.T) {
	var pf Postfilter
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, sPf [subframeLen]int16
	for i := range s {
		s[i] = int16(500 + i*3)
	}

	// AGC time constant ≈ 100 samples (α ≈ 0.99), so several hundred
	// samples (≈ 25+ subframes) are needed before g_pf settles within
	// 10% of g_target = 1.0.
	for k := 0; k < 50; k++ {
		pf.Filter(&a, 40, &s, &sPf)
	}

	for i := range sPf {
		want := int(s[i])
		got := int(sPf[i])
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		tol := want / 10
		if tol < 5 {
			tol = 5
		}
		if diff > tol {
			t.Errorf("sPf[%d] = %d, want ≈ %d (tol %d)", i, got, want, tol)
		}
	}
}
