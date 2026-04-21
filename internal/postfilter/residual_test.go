package postfilter

import "testing"

// Zero-coefficient numerator (except a[0]): residual = s directly.
func TestComputeResidual_ZeroTail(t *testing.T) {
	var pf Postfilter
	aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, r [subframeLen]int16
	for i := range s {
		s[i] = int16(100 + i*10)
	}

	pf.computeResidual(&aNum, &s, &r)

	for i := range r {
		if r[i] != s[i] {
			t.Errorf("r[%d] = %d, want %d (zero-tail residual is identity)",
				i, r[i], s[i])
		}
	}
}

// First-tap only: aNum = [4096, 2048, 0, ...] → r[n] = s(n) + 0.5·s(n-1).
func TestComputeResidual_FirstTapOnly(t *testing.T) {
	var pf Postfilter
	aNum := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, r [subframeLen]int16
	for i := range s {
		s[i] = 200
	}

	pf.computeResidual(&aNum, &s, &r)

	if r[0] < 199 || r[0] > 201 {
		t.Errorf("r[0] = %d, want 200 (±1)", r[0])
	}
	for i := 1; i < subframeLen; i++ {
		if r[i] < 299 || r[i] > 301 {
			t.Errorf("r[%d] = %d, want 300 (±1)", i, r[i])
		}
	}
}

func TestComputeResidual_PastSContributes(t *testing.T) {
	var pf Postfilter
	pf.pastS[9] = 1000
	aNum := [11]int16{4096, 4096, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, r [subframeLen]int16

	pf.computeResidual(&aNum, &s, &r)

	if r[0] < 999 || r[0] > 1001 {
		t.Errorf("r[0] = %d, want 1000 (from pastS[9])", r[0])
	}
	if r[1] != 0 && r[1] != 1 && r[1] != -1 {
		t.Errorf("r[1] = %d, want 0 (no active input)", r[1])
	}
}

func TestComputeResidual_UpdatesPastS(t *testing.T) {
	var pf Postfilter
	aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s, r [subframeLen]int16
	for i := range s {
		s[i] = int16(500 + i)
	}

	pf.computeResidual(&aNum, &s, &r)

	for i := 0; i < lpcOrder; i++ {
		want := s[subframeLen-lpcOrder+i]
		if pf.pastS[i] != want {
			t.Errorf("pastS[%d] = %d, want %d (last 10 of s)",
				i, pf.pastS[i], want)
		}
	}
}
