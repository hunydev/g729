package synth

import (
"testing"
)

func TestBuildExcitation_PitchIdentity(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = int16(100 + i*3)
}
BuildExcitation(16384, 0, 0, &v, &c, &u)
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
BuildExcitation(8192, 0, 0, &v, &c, &u)
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
BuildExcitation(0, 0, 0, &v, &c, &u)
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
// gcQ12=4096 (g_c=1.0) → mant=16384, exp=0 (preserves intent).
BuildExcitation(0, 16384, 0, &v, &c, &u)
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
// gcQ12=4096 (g_c=1.0) → mant=16384, exp=0.
BuildExcitation(0, 16384, 0, &v, &c, &u)
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
// gcQ12=2048 (g_c=0.5) → mant=8192, exp=0.
BuildExcitation(8192, 8192, 0, &v, &c, &u)
for i := range u {
if u[i] < 100 || u[i] > 101 {
t.Errorf("u[%d] = %d, want 100 or 101", i, u[i])
}
}
}

func TestBuildExcitation_SaturatesOnHighPitchGain(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = 32767
}
BuildExcitation(32767, 0, 0, &v, &c, &u)
for i := range u {
if u[i] != 32767 {
t.Errorf("u[%d] = %d, want MAX_16", i, u[i])
}
}
}

func TestBuildExcitation_SaturatesOnNegativeExtreme(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = -32768
}
BuildExcitation(32767, 0, 0, &v, &c, &u)
for i := range u {
if u[i] != -32768 {
t.Errorf("u[%d] = %d, want MIN_16", i, u[i])
}
}
}

func TestBuildExcitation_SaturatesOnBothContributionsHigh(t *testing.T) {
var v, c, u [40]int16
for i := range v {
v[i] = 32767
c[i] = 32767
}
// gcQ12=32767 (g_c≈7.999, max Q12) → mant=32767, exp=2 (preserves saturation intent).
BuildExcitation(32767, 32767, 2, &v, &c, &u)
for i := range u {
if u[i] != 32767 {
t.Errorf("u[%d] = %d, want MAX_16", i, u[i])
}
}
}
