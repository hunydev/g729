package fcb

import "testing"

func TestClampPitchGainForEnhancement(t *testing.T) {
	const (
		lowerQ14 = 3277
		upperQ14 = 13107
	)
	cases := []struct {
		name  string
		input int16
		want  int16
	}{
		{"below lower bound (zero)", 0, lowerQ14},
		{"at lower bound", lowerQ14, lowerQ14},
		{"just above lower bound", lowerQ14 + 1, lowerQ14 + 1},
		{"mid-range 0.5", 8192, 8192},
		{"just below upper bound", upperQ14 - 1, upperQ14 - 1},
		{"at upper bound", upperQ14, upperQ14},
		{"above upper bound (1.0 Q14)", 16384, upperQ14},
		{"max int16", 32767, upperQ14},
		{"negative clamps to lower", -1000, lowerQ14},
		{"most negative int16", -32768, lowerQ14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClampPitchGainForEnhancement(tc.input)
			if got != tc.want {
				t.Errorf("ClampPitchGainForEnhancement(%d) = %d, want %d",
					tc.input, got, tc.want)
			}
		})
	}
}

func TestApplyPitchEnhancement_IdentityAtBetaZero(t *testing.T) {
var c [40]int16
c[5] = PulseAmplitude
c[10] = -PulseAmplitude
original := c
applyPitchEnhancement(&c, 20, 0)
if c != original {
t.Fatalf("β=0 must leave c unchanged; got %v, want %v", c, original)
}
}

func TestApplyPitchEnhancement_BelowLagUnchanged(t *testing.T) {
var c [40]int16
c[5] = PulseAmplitude
c[10] = -PulseAmplitude
applyPitchEnhancement(&c, 20, 8192)
for n := 0; n < 20; n++ {
want := int16(0)
if n == 5 {
want = PulseAmplitude
} else if n == 10 {
want = -PulseAmplitude
}
if c[n] != want {
t.Errorf("c[%d] = %d, want %d (unchanged below lag)", n, c[n], want)
}
}
}

func TestApplyPitchEnhancement_SinglePulsePropagationAtBetaHalf(t *testing.T) {
var c [40]int16
c[0] = PulseAmplitude
applyPitchEnhancement(&c, 20, 8192)
if c[0] != PulseAmplitude {
t.Errorf("c[0] = %d, want %d (source unchanged)", c[0], PulseAmplitude)
}
if diff := c[20] - 4096; diff > 1 || diff < -1 {
t.Errorf("c[20] = %d, want ≈4096 (±1 for rounding)", c[20])
}
for n := 21; n < 40; n++ {
if c[n] != 0 {
t.Errorf("c[%d] = %d, want 0 (no further propagation for T=20)", n, c[n])
}
}
}

func TestApplyPitchEnhancement_CascadeAtBeta08(t *testing.T) {
var c [40]int16
c[0] = PulseAmplitude
applyPitchEnhancement(&c, 10, 13107)

if diff := c[10] - 6554; diff > 2 || diff < -2 {
t.Errorf("c[10] = %d, want ≈6554", c[10])
}
if diff := c[20] - 5243; diff > 3 || diff < -3 {
t.Errorf("c[20] = %d, want ≈5243", c[20])
}
if diff := c[30] - 4194; diff > 3 || diff < -3 {
t.Errorf("c[30] = %d, want ≈4194", c[30])
}
}

func TestApplyPitchEnhancement_InPlaceIIRNotFIR(t *testing.T) {
var c [40]int16
c[0] = PulseAmplitude
applyPitchEnhancement(&c, 5, 8192)

want := []int16{0, 0, 0, 0, 0, 4096, 0, 0, 0, 0, 2048, 0, 0, 0, 0, 1024}
for n := 0; n < len(want); n++ {
if n == 0 {
continue
}
if diff := c[n] - want[n]; diff > 1 || diff < -1 {
t.Errorf("c[%d] = %d, want ≈%d (IIR cascade)", n, c[n], want[n])
}
}
}
