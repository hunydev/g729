package postfilter

import "testing"

func TestNoAllocationInFilter(t *testing.T) {
	var pf Postfilter
	a := [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, 0, 0}
	var s, sPf [subframeLen]int16
	for i := range s {
		s[i] = int16(200 + i*5)
	}

	allocs := testing.AllocsPerRun(100, func() {
		pf.Filter(&a, 40, &s, &sPf)
	})
	if allocs != 0 {
		t.Errorf("Filter allocs = %v, want 0", allocs)
	}
}

func TestNoAllocationInReset(t *testing.T) {
	var pf Postfilter
	pf.pastS[0] = 1
	pf.pastResidual[0] = 1
	pf.pastSynthPost[0] = 1
	pf.pastTiltInput = 1
	pf.agcGainPrev = 1

	allocs := testing.AllocsPerRun(100, func() {
		pf.Reset()
	})
	if allocs != 0 {
		t.Errorf("Reset allocs = %v, want 0", allocs)
	}
}
