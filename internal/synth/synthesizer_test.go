package synth

import (
	"testing"
)

func TestSynthesizer_ZeroValueIsReset(t *testing.T) {
	var synth Synthesizer
	for i, v := range synth.pastSynth {
		if v != 0 {
			t.Errorf("pastSynth[%d] = %d, want 0", i, v)
		}
	}
}

func TestSynthesizer_ResetZerosState(t *testing.T) {
	var synth Synthesizer
	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(100 + i)
	}
	synth.Reset()
	for i, v := range synth.pastSynth {
		if v != 0 {
			t.Errorf("after Reset, pastSynth[%d] = %d, want 0", i, v)
		}
	}
}
