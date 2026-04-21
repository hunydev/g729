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

func TestFilter_PastStateContributes(t *testing.T) {
var synth Synthesizer
synth.pastSynth[9] = 1000

var u, s [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

synth.filterSubframe(&a, &u, &s)

want := []int16{-500, 250, -125, 62, -31}
for i, w := range want {
if s[i] != w && s[i] != w+1 && s[i] != w-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], w)
}
}
}

func TestFilter_StateUpdate(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

for i := range u {
u[i] = int16(1000 + i)
}

synth.filterSubframe(&a, &u, &s)

for i := 0; i < 10; i++ {
want := u[30+i]
if synth.pastSynth[i] != want {
t.Errorf("pastSynth[%d] = %d, want %d", i, synth.pastSynth[i], want)
}
}
}

func TestFilter_TwoSubframeContinuity(t *testing.T) {
var synth Synthesizer
var u1, u2, s1, s2 [40]int16
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

for i := range u1 {
u1[i] = int16(100 + i)
u2[i] = int16(200 + i)
}

synth.filterSubframe(&a, &u1, &s1)
synth.filterSubframe(&a, &u2, &s2)

for i := range s1 {
if s1[i] != u1[i] {
t.Errorf("s1[%d] = %d, want %d", i, s1[i], u1[i])
}
if s2[i] != u2[i] {
t.Errorf("s2[%d] = %d, want %d", i, s2[i], u2[i])
}
}
}

func TestFilter_IIRDecayAcrossBoundary(t *testing.T) {
var synth Synthesizer
synth.pastSynth[9] = 4000

var u, s1, s2 [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

synth.filterSubframe(&a, &u, &s1)

if s1[0] != -2000 && s1[0] != -1999 && s1[0] != -2001 {
t.Errorf("s1[0] = %d, want -2000 ±1", s1[0])
}

synth.filterSubframe(&a, &u, &s2)

for i := range s2 {
if s2[i] > 2 || s2[i] < -2 {
t.Errorf("s2[%d] = %d, expected |·| ≤ 2", i, s2[i])
}
}
}
