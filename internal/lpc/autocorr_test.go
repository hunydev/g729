package lpc

import (
	"math"
	"testing"
)

// TestAutocorrelate_AllZeroInputAllZeroOutput asserts that a fully
// silent windowed buffer yields r[0..10] = 0 with scale = 0
// (§3.2.1 eq. 5).
func TestAutocorrelate_AllZeroInputAllZeroOutput(t *testing.T) {
	var windowed [240]int16
	var r [11]int32
	scale := autocorrelate(&windowed, &r)
	if scale != 0 {
		t.Fatalf("scale = %d, want 0", scale)
	}
	for k := 0; k <= 10; k++ {
		if r[k] != 0 {
			t.Fatalf("r[%d] = %d, want 0", k, r[k])
		}
	}
}

// TestAutocorrelate_DCInputClosedForm asserts that for s'(n) = 1024,
// r(k) = (240 - k) * 1024^2 for k = 0..10 (§3.2.1 eq. 5 directly).
// 240 * 1024^2 = 251_658_240 fits in Word32 with no scaling, so the
// returned scale must be 0.
func TestAutocorrelate_DCInputClosedForm(t *testing.T) {
	var windowed [240]int16
	for n := 0; n < 240; n++ {
		windowed[n] = 1024
	}
	var r [11]int32
	scale := autocorrelate(&windowed, &r)
	if scale != 0 {
		t.Fatalf("scale = %d, want 0 (DC 1024 fits without normalization)", scale)
	}
	for k := 0; k <= 10; k++ {
		want := int32(240-k) * 1024 * 1024
		if r[k] != want {
			t.Fatalf("r[%d] = %d, want %d", k, r[k], want)
		}
	}
}

// TestAutocorrelate_PeriodicSineHasPeriodPeak asserts that for a
// 1 kHz sine sampled at 8 kHz (period = 8 samples) the autocorrelation
// at lag k=8 is within 1% of r[0] (the period-aligned self-similarity
// peak) and that r[0] > 0. This validates the cross-product accumulation
// across all 240 - k taps.
func TestAutocorrelate_PeriodicSineHasPeriodPeak(t *testing.T) {
	var windowed [240]int16
	const amp = 16384.0
	for n := 0; n < 240; n++ {
		v := amp * math.Sin(2*math.Pi*float64(n)/8.0)
		windowed[n] = int16(math.Round(v))
	}
	var r [11]int32
	_ = autocorrelate(&windowed, &r)
	if r[0] <= 0 {
		t.Fatalf("r[0] = %d, want > 0", r[0])
	}
	// k=8 spans n=8..239 (232 taps) vs k=0 spanning 240 taps; ratio
	// ~232/240 ≈ 0.967. Allow 5 % slack to absorb int16 rounding.
	lo := int64(r[0]) * 92 / 100
	if int64(r[8]) < lo {
		t.Fatalf("r[8] = %d, want >= %d (period-aligned peak)", r[8], lo)
	}
	// Off-period (k=4, half period) must be deeply negative.
	if r[4] >= 0 {
		t.Fatalf("r[4] = %d, want < 0 (anti-correlated half-period)", r[4])
	}
}

// TestAutocorrelate_OverflowTriggersScaling asserts that an input near
// the int16 limit (which would push 240·v² past 2³¹−1) triggers the
// shared right-shift normalization (scale > 0) and that r[0] remains
// in Word32 range, with r[0] decreasing by ~4× per scale bit applied
// to the inputs.
func TestAutocorrelate_OverflowTriggersScaling(t *testing.T) {
	var windowed [240]int16
	for n := 0; n < 240; n++ {
		windowed[n] = 30000
	}
	var r [11]int32
	scale := autocorrelate(&windowed, &r)
	if scale == 0 {
		t.Fatalf("scale = 0, want > 0 (240 * 30000^2 = 2.16e11 overflows Word32)")
	}
	// After right-shifting inputs by `scale` bits, each sample is
	// floor(30000 / 2^scale). r[0] is the sum of squares of those.
	shifted := int32(30000) >> uint(scale)
	want := int32(240) * shifted * shifted
	if r[0] != want {
		t.Fatalf("r[0] = %d, want %d (240 * (30000>>%d)^2)", r[0], want, scale)
	}
}
