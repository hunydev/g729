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

func TestSynthesize_EndToEndIdentity(t *testing.T) {
var synth Synthesizer
var v, c, s [40]int16
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

for i := range v {
v[i] = int16(500 + i*10)
}

synth.Synthesize(&a, &v, &c, 16384, 0, &s)

for i := range s {
if s[i] != v[i] {
t.Errorf("s[%d] = %d, want %d", i, s[i], v[i])
}
}
}

func TestSynthesize_MatchesPiecewiseComposition(t *testing.T) {
var v, c [40]int16
a := [11]int16{4096, 1500, -800, 300, -100, 0, 0, 0, 0, 0, 0}

for i := range v {
v[i] = int16(i * 17)
c[i] = int16((i - 20) * 400)
}

var synthRef Synthesizer
var uRef, sRef [40]int16
BuildExcitation(10000, 2000, &v, &c, &uRef)
synthRef.filterSubframe(&a, &uRef, &sRef)

var synthUUT Synthesizer
var sUUT [40]int16
synthUUT.Synthesize(&a, &v, &c, 10000, 2000, &sUUT)

for i := range sRef {
if sRef[i] != sUUT[i] {
t.Errorf("s[%d] = %d, want %d", i, sUUT[i], sRef[i])
}
}
for i := range synthRef.pastSynth {
if synthRef.pastSynth[i] != synthUUT.pastSynth[i] {
t.Errorf("pastSynth[%d] mismatch", i)
}
}
}
