package openloop

import "testing"

// TestLPResidual_Zero pins eq. A.3 trivial case: zero speech and zero
// 10-sample input history must yield r(n) = 0 for all 80 samples.
func TestLPResidual_Zero(t *testing.T) {
	var s [80]int16
	var aHat [11]int16
	aHat[0] = 4096 // Q12 leading 1.0; not consumed in the i=1..10 sum.
	var mem [10]int16
	var r [80]int16
	lpResidual(&s, &aHat, &mem, &r)
	for n := 0; n < 80; n++ {
		if r[n] != 0 {
			t.Fatalf("lpResidual(zero) r[%d] = %d, want 0", n, r[n])
		}
	}
}

// TestLPResidual_ImpulseIdentity pins eq. A.3 for an identity Â
// (â[1..10] = 0) and an impulse input s = [4096, 0, …, 0]. Since the
// i = 1..10 sum is zero (all â taps zero), r(n) = s(n).
func TestLPResidual_ImpulseIdentity(t *testing.T) {
	var s [80]int16
	s[0] = 4096
	var aHat [11]int16
	aHat[0] = 4096
	var mem [10]int16
	var r [80]int16
	lpResidual(&s, &aHat, &mem, &r)
	if r[0] != 4096 {
		t.Fatalf("lpResidual(impulse) r[0] = %d, want 4096", r[0])
	}
	for n := 1; n < 80; n++ {
		if r[n] != 0 {
			t.Fatalf("lpResidual(impulse) r[%d] = %d, want 0", n, r[n])
		}
	}
}

// TestLPResidual_NoAlloc enforces I4 on the hot path.
func TestLPResidual_NoAlloc(t *testing.T) {
	var s [80]int16
	var aHat [11]int16
	aHat[0] = 4096
	var mem [10]int16
	var r [80]int16
	allocs := testing.AllocsPerRun(100, func() {
		lpResidual(&s, &aHat, &mem, &r)
	})
	if allocs != 0 {
		t.Fatalf("lpResidual allocates %v/op, want 0", allocs)
	}
}

// TestLowpassWeightedSpeech_Zero pins eq. A.2 trivial case: zero
// residual and zero 10-sample sw history must yield sw(n) = 0.
func TestLowpassWeightedSpeech_Zero(t *testing.T) {
	var r [80]int16
	var aPrime [11]int16
	aPrime[0] = 4096
	var mem [10]int16
	var sw [80]int16
	lowpassWeightedSpeech(&r, &aPrime, &mem, &sw)
	for n := 0; n < 80; n++ {
		if sw[n] != 0 {
			t.Fatalf("lowpassWeightedSpeech(zero) sw[%d] = %d, want 0", n, sw[n])
		}
	}
}

// TestLowpassWeightedSpeech_ImpulseIdentity pins eq. A.2 for an
// identity A' (a'[1..10] = 0) and an impulse residual r = [4096, 0,
// …, 0]. The i = 1..10 recursive sum is zero so sw(n) = r(n).
func TestLowpassWeightedSpeech_ImpulseIdentity(t *testing.T) {
	var r [80]int16
	r[0] = 4096
	var aPrime [11]int16
	aPrime[0] = 4096
	var mem [10]int16
	var sw [80]int16
	lowpassWeightedSpeech(&r, &aPrime, &mem, &sw)
	if sw[0] != 4096 {
		t.Fatalf("lowpassWeightedSpeech(impulse) sw[0] = %d, want 4096", sw[0])
	}
	for n := 1; n < 80; n++ {
		if sw[n] != 0 {
			t.Fatalf("lowpassWeightedSpeech(impulse) sw[%d] = %d, want 0", n, sw[n])
		}
	}
}

// TestLowpassWeightedSpeech_NoAlloc enforces I4 on the hot path.
func TestLowpassWeightedSpeech_NoAlloc(t *testing.T) {
	var r [80]int16
	var aPrime [11]int16
	aPrime[0] = 4096
	var mem [10]int16
	var sw [80]int16
	allocs := testing.AllocsPerRun(100, func() {
		lowpassWeightedSpeech(&r, &aPrime, &mem, &sw)
	})
	if allocs != 0 {
		t.Fatalf("lowpassWeightedSpeech allocates %v/op, want 0", allocs)
	}
}

// TestSlideOldWspeech_RampPin is the I-2b-2 binding pin: load the
// 143-element history with [0,1,…,142], slide in a fresh
// [200,201,…,279] block, and verify the resulting layout is
// old[0:63]   = [80,81,…,142]      (previous-frame's old[80:143])
// old[63:143] = [200,201,…,279]    (current-frame fresh sw samples)
func TestSlideOldWspeech_RampPin(t *testing.T) {
	var old [143]int16
	for i := 0; i < 143; i++ {
		old[i] = int16(i)
	}
	var fresh [80]int16
	for i := 0; i < 80; i++ {
		fresh[i] = int16(200 + i)
	}
	slideOldWspeech(&old, &fresh)
	for i := 0; i < 63; i++ {
		want := int16(80 + i)
		if old[i] != want {
			t.Fatalf("slideOldWspeech old[%d] = %d, want %d", i, old[i], want)
		}
	}
	for i := 0; i < 80; i++ {
		want := int16(200 + i)
		if old[63+i] != want {
			t.Fatalf("slideOldWspeech old[%d] = %d, want %d", 63+i, old[63+i], want)
		}
	}
}

// TestSlideOldWspeech_NoAlloc enforces I4 on the per-frame slide.
func TestSlideOldWspeech_NoAlloc(t *testing.T) {
	var old [143]int16
	var fresh [80]int16
	allocs := testing.AllocsPerRun(100, func() {
		slideOldWspeech(&old, &fresh)
	})
	if allocs != 0 {
		t.Fatalf("slideOldWspeech allocates %v/op, want 0", allocs)
	}
}
