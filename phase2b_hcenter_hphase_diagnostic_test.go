package g729

import (
	"math"
	"testing"
)

// TestPhase2bHCenter_HPhaseBoundaryAlignment is a production-free
// diagnostic for the H-PHASE hypothesis. Annex A §A.3.3 eq. A.2/A.3
// define recursive filters over continuous subframe time, so after a
// frame commit the next-frame memories must line up with the tail of
// the same low-pass weighted-speech history searched by §A.3.4.
func TestPhase2bHCenter_HPhaseBoundaryAlignment(t *testing.T) {
	enc := NewEncoder()

	var phase float64
	for f := 0; f < 6; f++ {
		var pcm [FrameSamples]int16
		for i := range pcm {
			pcm[i] = int16(7000 * math.Sin(2*math.Pi*phase/47.0))
			phase++
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		enc.openloopStep()

		for i := 0; i < 10; i++ {
			if got, want := enc.swMem[i], enc.oldWspeech[133+i]; got != want {
				t.Fatalf("frame %d swMem[%d]=%d, want oldWspeech[%d]=%d",
					f, i, got, 133+i, want)
			}
			if got, want := enc.lpResidualMem[i], enc.oldSpeech[230+i]; got != want {
				t.Fatalf("frame %d lpResidualMem[%d]=%d, want oldSpeech[%d]=%d",
					f, i, got, 230+i, want)
			}
		}
	}
}
