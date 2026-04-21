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

func TestBuildExcitation_CodeIdentity(t *testing.T) {
var v, c, u [40]int16
for i := range c {
c[i] = 8192
}
BuildExcitation(0, 4096, &v, &c, &u)
for i := range u {
if u[i] != 1 {
t.Errorf("u[%d] = %d, want 1", i, u[i])
}
}
}

func TestBuildExcitation_CodeScales(t *testing.T) {
var v, c, u [40]int16
for i := range c {
c[i] = 16384
}
BuildExcitation(0, 4096, &v, &c, &u)
for i := range u {
if u[i] != 2 {
t.Errorf("u[%d] = %d, want 2", i, u[i])
}
}
}

func TestBuildExcitation_PitchAndCodeCombined(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = 200
c[i] = 8192
}
BuildExcitation(8192, 2048, &v, &c, &u)
for i := range u {
if u[i] < 100 || u[i] > 101 {
t.Errorf("u[%d] = %d, want 100 or 101", i, u[i])
}
}
}
