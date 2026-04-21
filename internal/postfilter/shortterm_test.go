package postfilter

import "testing"

// Zero LPC tail → s_st = r' (identity).
func TestApplyShortTerm_ZeroTail(t *testing.T) {
	var pf Postfilter
	aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var rIn, sOut [subframeLen]int16
	for i := range rIn {
		rIn[i] = int16(500 + i*7)
	}

	pf.applyShortTerm(&aDen, &rIn, &sOut)

	for i := range sOut {
		if sOut[i] != rIn[i] {
			t.Errorf("sOut[%d] = %d, want %d (zero-tail identity)",
				i, sOut[i], rIn[i])
		}
	}
}

// State carry: two consecutive calls with identity filter pass through r'.
func TestApplyShortTerm_StateCarriesAcrossSubframes(t *testing.T) {
	var pf Postfilter
	aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var r1, r2, s1, s2 [subframeLen]int16
	for i := range r1 {
		r1[i] = int16(100 + i)
		r2[i] = int16(200 + i)
	}

	pf.applyShortTerm(&aDen, &r1, &s1)
	pf.applyShortTerm(&aDen, &r2, &s2)

	for i := range s1 {
		if s1[i] != r1[i] {
			t.Errorf("s1[%d] = %d, want %d", i, s1[i], r1[i])
		}
		if s2[i] != r2[i] {
			t.Errorf("s2[%d] = %d, want %d", i, s2[i], r2[i])
		}
	}
}

func TestApplyShortTerm_UpdatesPastSynthPost(t *testing.T) {
	var pf Postfilter
	aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var rIn, sOut [subframeLen]int16
	for i := range rIn {
		rIn[i] = int16(i + 10)
	}

	pf.applyShortTerm(&aDen, &rIn, &sOut)

	for i := 0; i < lpcOrder; i++ {
		want := sOut[subframeLen-lpcOrder+i]
		if pf.pastSynthPost[i] != want {
			t.Errorf("pastSynthPost[%d] = %d, want %d",
				i, pf.pastSynthPost[i], want)
		}
	}
}
