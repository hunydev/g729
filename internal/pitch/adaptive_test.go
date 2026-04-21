package pitch

import "testing"

func TestAdaptiveCodebookIntegerDelay(t *testing.T) {
	// Convention: pastExc[len(pastExc)-1] is u(-1), one sample before
	// the current subframe's v[0]. For integer delay T, v[n] = u(n−T)
	// = pastExc[len − T + n].
	var pastExc [200]int16
	for i := range pastExc {
		pastExc[i] = int16(i)
	}
	var v [40]int16
	AdaptiveCodebook(50, 0, pastExc[:], &v)
	for n := 0; n < 40; n++ {
		want := int16(150 + n) // pastExc[200 − 50 + n] = pastExc[150 + n]
		if v[n] != want {
			t.Errorf("v[%d] = %d, want %d (integer delay 50)", n, v[n], want)
		}
	}
}

func TestAdaptiveCodebookIntegerDelayLargest(t *testing.T) {
	var pastExc [200]int16
	for i := range pastExc {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	AdaptiveCodebook(143, 0, pastExc[:], &v)
	for n := 0; n < 40; n++ {
		want := pastExc[200-143+n]
		if v[n] != want {
			t.Errorf("v[%d] = %d, want %d (integer delay 143)", n, v[n], want)
		}
	}
}
