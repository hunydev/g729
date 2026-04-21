package postfilter

import "testing"

// Energy-neutral case: s and sTilt have equal energies → g_target ≈ 1.0 Q14.
func TestComputeAGCTargetGain_EqualEnergy(t *testing.T) {
	var pf Postfilter
	var s, sTilt [subframeLen]int16
	for i := range s {
		s[i] = int16(1000 - i*10)
		sTilt[i] = int16(1000 - i*10)
	}

	g := pf.computeAGCTargetGain(&s, &sTilt)
	if g < 16380 || g > 16388 {
		t.Errorf("g = %d, want ≈ 16384 (equal energies)", g)
	}
}

func TestComputeAGCTargetGain_ZeroTiltEnergy(t *testing.T) {
	var pf Postfilter
	var s, sTilt [subframeLen]int16
	for i := range s {
		s[i] = int16(1000)
	}

	g := pf.computeAGCTargetGain(&s, &sTilt)
	if g != 0 {
		t.Errorf("g = %d, want 0 (zero sTilt energy)", g)
	}
}

func TestApplyAGC_SmoothingDoesNotOvershoot(t *testing.T) {
	var pf Postfilter
	var sTilt, sPf [subframeLen]int16
	for i := range sTilt {
		sTilt[i] = 1000
	}
	const gTargetQ14 int16 = 8192

	pf.applyAGC(&sTilt, gTargetQ14, &sPf)

	// agcGainPrev is held at Q24 internally; gTargetQ14 = 8192 corresponds
	// to 8192<<10 = 8388608 at Q24.
	if pf.agcGainPrev < 0 || pf.agcGainPrev > 8192<<10 {
		t.Errorf("agcGainPrev = %d, want ∈ (0, %d]", pf.agcGainPrev, 8192<<10)
	}
	for k := 0; k < 200; k++ {
		pf.applyAGC(&sTilt, gTargetQ14, &sPf)
	}
	want := int32(8192) << 10
	if pf.agcGainPrev < want-2048 || pf.agcGainPrev > want+2048 {
		t.Errorf("after convergence, agcGainPrev = %d, want ≈ %d (±2 LSB at Q14)",
			pf.agcGainPrev, want)
	}
}
