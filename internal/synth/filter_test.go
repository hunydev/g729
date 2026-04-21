package synth

import (
	"testing"
)

// With a = [4096, 0, 0, ..., 0] (i.e. A(z) = 1, synthesis is identity),
// the filter should reproduce u in s, regardless of pastSynth.
func TestFilter_ZeroLPCIsIdentity(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range u {
		u[i] = int16(1000 + i*37)
	}

	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(9000 - i*100)
	}

	synth.filterSubframe(&a, &u, &s)

	for i := range s {
		if s[i] != u[i] {
			t.Errorf("s[%d] = %d, want %d (zero LPC is identity)", i, s[i], u[i])
		}
	}
}

func TestFilter_FirstOrderImpulseResponse(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

u[0] = 4000

synth.filterSubframe(&a, &u, &s)

expected := []int16{4000, -2000, 1000, -500, 250, -125, 62, -31, 16, -8}
for i, want := range expected {
if s[i] != want && s[i] != want+1 && s[i] != want-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
}
}
if s[20] > 2 || s[20] < -2 {
t.Errorf("s[20] = %d, want |·| ≤ 2 after decay", s[20])
}
}

func TestFilter_FirstOrderPositiveFeedback(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

u[0] = 4000

synth.filterSubframe(&a, &u, &s)

expected := []int16{4000, 2000, 1000, 500, 250, 125}
for i, want := range expected {
if s[i] != want && s[i] != want+1 && s[i] != want-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
}
}
}
