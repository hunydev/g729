package lpc

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// TestWindowSpeech_DCInputMatchesLUT asserts windowSpeech applies the
// §3.2.1 eq. 4 product s'(n) = w_lp(n)·s(n) for every n ∈ [0,239].
// Using a DC input s(n) = 1024 (Q0), each output sample must equal
// fixed.Mult(1024, w_lp(n)).
func TestWindowSpeech_DCInputMatchesLUT(t *testing.T) {
	var speech [240]int16
	for n := 0; n < 240; n++ {
		speech[n] = 1024
	}
	var windowed [240]int16
	windowSpeech(&speech, &windowed)
	for n := 0; n < 240; n++ {
		want := fixed.Mult(1024, lpAnalysisWindow[n])
		if windowed[n] != want {
			t.Fatalf("windowed[%d] = %d, want %d", n, windowed[n], want)
		}
	}
}

// TestWindowSpeech_ZeroInputZeroOutput asserts zero input yields zero
// output across all 240 taps (sanity for the multiplicative form of
// §3.2.1 eq. 4).
func TestWindowSpeech_ZeroInputZeroOutput(t *testing.T) {
	var speech [240]int16
	var windowed [240]int16
	for n := 0; n < 240; n++ {
		windowed[n] = 0x5A5A
	}
	windowSpeech(&speech, &windowed)
	for n := 0; n < 240; n++ {
		if windowed[n] != 0 {
			t.Fatalf("windowed[%d] = %d, want 0", n, windowed[n])
		}
	}
}

// TestWindowSpeech_ZeroAllocation gates I4 zero-allocation in steady
// state for the per-sample window application.
func TestWindowSpeech_ZeroAllocation(t *testing.T) {
	var speech [240]int16
	var windowed [240]int16
	for n := 0; n < 240; n++ {
		speech[n] = int16(n - 120)
	}
	allocs := testing.AllocsPerRun(100, func() {
		windowSpeech(&speech, &windowed)
	})
	if allocs != 0 {
		t.Fatalf("windowSpeech allocs/run = %v, want 0", allocs)
	}
}
