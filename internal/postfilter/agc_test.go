package postfilter

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// Energy-neutral case: s and sTilt have equal energies → g_target ≈ 1.0 Q14.
func TestComputeAGCTargetGain_EqualEnergy(t *testing.T) {
	var pf Postfilter
	var s, sTilt [subframeLen]int16
	for i := range s {
		s[i] = int16(1000 - i*10)
		sTilt[i] = int16(1000 - i*10)
	}

	g := pf.computeAGCTargetGain(&s, &sTilt)
	if g < 1628 || g > 1644 {
		t.Errorf("g = %d, want ≈ 1636 (equal energies target increment)", g)
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

func TestAGCTargetEnergySaturatesWord32(t *testing.T) {
	var x [subframeLen]int16
	for i := range x {
		x[i] = fixed.Max16
	}

	if got := agcTargetEnergy(&x); got != int64(fixed.Max32) {
		t.Fatalf("agcTargetEnergy(max subframe) = %d; want Word32 saturation %d", got, fixed.Max32)
	}
}

func TestApplyAGC_SmoothingDoesNotOvershoot(t *testing.T) {
	var pf Postfilter
	var sTilt, sPf [subframeLen]int16
	for i := range sTilt {
		sTilt[i] = 1000
	}
	const gTargetQ14 int16 = 819

	pf.applyAGC(&sTilt, gTargetQ14, &sPf)

	// gTargetQ14 is an increment; steady-state gain is roughly 10x it.
	if pf.agcGainPrev <= 8192<<10 || pf.agcGainPrev >= 1<<24 {
		t.Errorf("agcGainPrev = %d, want between target and unity", pf.agcGainPrev)
	}
	for k := 0; k < 200; k++ {
		pf.applyAGC(&sTilt, gTargetQ14, &sPf)
	}
	want := int32(2040) << 12
	if pf.agcGainPrev < want-(4<<12) || pf.agcGainPrev > want+(4<<12) {
		t.Errorf("after convergence, agcGainPrev = %d, want ≈ %d (±2 LSB at Q14)",
			pf.agcGainPrev, want)
	}
}

// TestApplyAGC_FirstCallUsesSeededGain asserts that the first-ever
// applyAGC call (Postfilter zero value) uses the current g_target as
// the smoother's initial state, NOT zero.
func TestApplyAGC_FirstCallUsesSeededGain(t *testing.T) {
	var pf Postfilter

	var sTilt [subframeLen]int16
	for i := range sTilt {
		sTilt[i] = 1000
	}
	gTargetQ14 := int16(1636) // unity target increment

	var sPf [subframeLen]int16
	pf.applyAGC(&sTilt, gTargetQ14, &sPf)

	for i := range sPf {
		if sPf[i] < 900 || sPf[i] > 1100 {
			t.Fatalf("sPf[%d] = %d; expected ≈1000 (g_target=1.0, input=1000). "+
				"AGC startup transient is corrupting output. sPf[:8]=%v",
				i, sPf[i], sPf[:8])
		}
	}
}
