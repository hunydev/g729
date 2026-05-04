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

func TestFilter_ResetRestoresZeroValueDeterminism(t *testing.T) {
	a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}
	var s [subframeLen]int16
	for i := range s {
		s[i] = int16(200 + i*5)
	}

	var pfRef Postfilter
	var sRef [subframeLen]int16
	pfRef.Filter(&a, 40, &s, &sRef)

	var pfUUT Postfilter
	var dummy [subframeLen]int16
	pfUUT.Filter(&a, 60, &dummy, &dummy)
	pfUUT.Reset()

	var sUUT [subframeLen]int16
	pfUUT.Filter(&a, 40, &s, &sUUT)

	for i := range sRef {
		if sRef[i] != sUUT[i] {
			t.Errorf("sPf[%d] = %d, want %d (Reset non-deterministic)",
				i, sUUT[i], sRef[i])
		}
	}
}

func TestFilter_StatePropagatesAcrossSubframes(t *testing.T) {
	a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}
	var s1, s2 [subframeLen]int16
	for i := range s1 {
		s1[i] = int16(200 + i*5)
		s2[i] = int16(500 - i*3)
	}

	var pfA Postfilter
	var a1, a2 [subframeLen]int16
	pfA.Filter(&a, 40, &s1, &a1)
	pfA.Filter(&a, 40, &s2, &a2)

	var pfB Postfilter
	var b2 [subframeLen]int16
	pfB.Filter(&a, 40, &s2, &b2)

	different := false
	for i := range a2 {
		if a2[i] != b2[i] {
			different = true
			break
		}
	}
	if !different {
		t.Error("a2 == b2 — postfilter state did not carry across subframes")
	}
}

// TestFilter_ImpulseResponse_FirstSampleNonZero asserts that a smooth
// unit-magnitude synth input does NOT produce a zero-valued first
// output sample. The §A.4.2 postfilter is a cascade of bandwidth-
// expanded FIR / long-term / IIR / tilt / AGC; none of these stages
// introduces algorithmic group delay, so the first output sample must
// reflect the first input sample scaled by the AGC gain.
func TestFilter_ImpulseResponse_FirstSampleNonZero(t *testing.T) {
	var pf Postfilter

	var a [11]int16
	a[0] = 4096

	var s [subframeLen]int16
	for i := range s {
		s[i] = 100
	}

	var sPf [subframeLen]int16
	pf.Filter(&a, 40, &s, &sPf)

	if sPf[0] == 0 {
		t.Fatalf("Filter output sample 0 is 0; expected non-zero (input was 100 flat). "+
			"Postfilter introduced a startup delay. sPf[:8]=%v", sPf[:8])
	}
	if sPf[0] < 50 || sPf[0] > 150 {
		t.Fatalf("Filter output sample 0 = %d; expected ≈100 (input was 100 flat, AGC should pass unity).", sPf[0])
	}
}

// TestFilter_SmoothPositiveInput_PreservesPolarity asserts that a
// monotonically-positive synth input produces a predominantly-
// positive postfilter output (at least 75 % of samples must be
// non-negative).
func TestFilter_SmoothPositiveInput_PreservesPolarity(t *testing.T) {
	var pf Postfilter

	var a [11]int16
	a[0] = 4096
	a[1] = -2048
	a[2] = 1024

	var s [subframeLen]int16
	for i := range s {
		s[i] = int16(500 + i*10)
	}

	var sPf [subframeLen]int16
	pf.Filter(&a, 40, &s, &sPf)

	negCount := 0
	for _, v := range sPf {
		if v < 0 {
			negCount++
		}
	}
	if negCount > subframeLen/4 {
		t.Fatalf("Postfilter inverted %d/%d samples on a monotonically-positive input. sPf=%v",
			negCount, subframeLen, sPf[:])
	}
}
