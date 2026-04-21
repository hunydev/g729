package synth

import (
"testing"
)

func TestBuildExcitation_PitchIdentity(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = int16(100 + i*3)
}
BuildExcitation(16384, 0, &v, &c, &u)
for i := range u {
if u[i] != v[i] {
t.Errorf("u[%d] = %d, want %d (g_p = 1.0)", i, u[i], v[i])
}
}
}

func TestBuildExcitation_PitchHalfGain(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = 200
}
BuildExcitation(8192, 0, &v, &c, &u)
for i := range u {
if u[i] != 100 {
t.Errorf("u[%d] = %d, want 100", i, u[i])
}
}
}

func TestBuildExcitation_ZeroGains(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = int16(i * 50)
}
BuildExcitation(0, 0, &v, &c, &u)
for i := range u {
if u[i] != 0 {
t.Errorf("u[%d] = %d, want 0", i, u[i])
}
}
}
