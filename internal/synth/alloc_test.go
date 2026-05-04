package synth

import "testing"

func TestNoAllocationInBuildExcitation(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = int16(i * 13)
c[i] = int16(i * 27)
}

allocs := testing.AllocsPerRun(100, func() {
BuildExcitation(12000, 6000, 0, &v, &c, &u)
})
if allocs != 0 {
t.Errorf("BuildExcitation allocs = %v, want 0", allocs)
}
}

func TestNoAllocationInSynthesize(t *testing.T) {
var synth Synthesizer
var v, c, s [40]int16
a := [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, 0, 0}
for i := range v {
v[i] = int16(i * 11)
c[i] = int16((i - 5) * 200)
}

allocs := testing.AllocsPerRun(100, func() {
synth.Synthesize(&a, &v, &c, 12000, 6000, 0, &s)
})
if allocs != 0 {
t.Errorf("Synthesize allocs = %v, want 0", allocs)
}
}

func TestNoAllocationInReset(t *testing.T) {
var synth Synthesizer
for i := range synth.pastSynth {
synth.pastSynth[i] = int16(i)
}

allocs := testing.AllocsPerRun(100, func() {
synth.Reset()
})
if allocs != 0 {
t.Errorf("Reset allocs = %v, want 0", allocs)
}
}
