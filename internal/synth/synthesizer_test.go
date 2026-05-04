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

synth.Synthesize(&a, &v, &c, 16384, 0, 0, &s)

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
// gcQ12=2000 → mant=8000, exp=0 (preserves intent: g_c ≈ 0.488).
BuildExcitation(10000, 8000, 0, &v, &c, &uRef)
synthRef.filterSubframe(&a, &uRef, &sRef)

var synthUUT Synthesizer
var sUUT [40]int16
synthUUT.Synthesize(&a, &v, &c, 10000, 8000, 0, &sUUT)

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

func TestSynthesize_ResetRestoresZeroValueDeterminism(t *testing.T) {
var v, c [40]int16
a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}

for i := range v {
v[i] = int16(i * 13)
c[i] = int16((i - 10) * 200)
}

var synthRef Synthesizer
var sRef [40]int16
// gcQ12=1500 → mant=6000, exp=0 (preserves intent: g_c ≈ 0.366).
synthRef.Synthesize(&a, &v, &c, 12000, 6000, 0, &sRef)

var synthUUT Synthesizer
for i := range synthUUT.pastSynth {
synthUUT.pastSynth[i] = int16(5000 - i*50)
}
synthUUT.Reset()

var sUUT [40]int16
synthUUT.Synthesize(&a, &v, &c, 12000, 6000, 0, &sUUT)

for i := range sRef {
if sRef[i] != sUUT[i] {
t.Errorf("s[%d] = %d, want %d", i, sUUT[i], sRef[i])
}
}
}

func TestSynthesize_StatePropagatesAcrossSubframes(t *testing.T) {
var synth Synthesizer
var v1, v2, c, s1, s2 [40]int16
a := [11]int16{4096, 4000, 0, 0, 0, 0, 0, 0, 0, 0, 0}

v1[0] = 4000

synth.Synthesize(&a, &v1, &c, 16384, 0, 0, &s1)
synth.Synthesize(&a, &v2, &c, 0, 0, 0, &s2)

anyNonZero := false
for i := range s2 {
if s2[i] != 0 {
anyNonZero = true
break
}
}
if !anyNonZero {
t.Error("s2 is all zero — state did not propagate")
}
}

func TestFilter_MatchesSynthesize(t *testing.T) {
a := [11]int16{4096, 1000, -500, 200, 0, 0, 0, 0, 0, 0, 0}
var v, c [40]int16
for i := range v {
v[i] = int16(100 + 3*i)
c[i] = int16(50 - 2*i)
}
gpQ14 := int16(8192)
// gcQ12=2048 → mant=8192, exp=0 (g_c=0.5).
gcMantQ14 := int16(8192)
gcExp := int8(0)

var sRef [40]int16
var synRef Synthesizer
synRef.Synthesize(&a, &v, &c, gpQ14, gcMantQ14, gcExp, &sRef)

var u, sSplit [40]int16
BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)
var synSplit Synthesizer
synSplit.Filter(&a, &u, &sSplit)

if sRef != sSplit {
t.Fatalf("Filter(a, u, s) did not match Synthesize(a, v, c, gp, gc, s):\n ref=%v\n got=%v", sRef, sSplit)
}
if synRef != synSplit {
t.Fatalf("state mismatch: ref=%+v got=%+v", synRef, synSplit)
}
}

func TestFilter_ZeroExcitationIsZero(t *testing.T) {
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
var u, s [40]int16
var syn Synthesizer
syn.Filter(&a, &u, &s)
var zero [40]int16
if s != zero {
t.Fatalf("zero excitation produced non-zero output: %v", s)
}
}
