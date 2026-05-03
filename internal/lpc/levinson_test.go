package lpc

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLevinsonDurbin_KroneckerR0 asserts the §3.2.2 recursion on the
// degenerate input r' = [1, 0, 0, ..., 0] yields the trivial all-pole
// filter A(z) = 1 (a[0] = 4096 in Q12, a[1..10] = 0). With every
// reflection coefficient k_i = 0, the prediction error stays at
// E[i] = E[0] = 1 throughout.
func TestLevinsonDurbin_KroneckerR0(t *testing.T) {
	var r [11]int32
	r[0] = 1
	var a [11]int16
	levinsonDurbin(&r, &a)
	if a[0] != 4096 {
		t.Fatalf("a[0] = %d, want 4096 (Q12 one)", a[0])
	}
	for j := 1; j <= 10; j++ {
		if a[j] != 0 {
			t.Errorf("a[%d] = %d, want 0", j, a[j])
		}
	}
}

// TestLevinsonDurbin_AR1Pole asserts that a pole-0.5 AR(1) process
// (whose true autocorrelation decays as R(k) = R(0)·0.5^|k|) recovers
// a[1] = -0.5 (Q12: -2048) with a[2..10] all zero. The AR(1) model
// A(z) = 1 - 0.5 z^-1 is the unique order-10 predictor for this
// input, so all higher reflection coefficients must vanish.
func TestLevinsonDurbin_AR1Pole(t *testing.T) {
	var r [11]int32
	r[0] = 1 << 24
	for k := 1; k <= 10; k++ {
		r[k] = r[k-1] >> 1
	}
	var a [11]int16
	levinsonDurbin(&r, &a)
	if a[0] != 4096 {
		t.Fatalf("a[0] = %d, want 4096", a[0])
	}
	// a[1] should be -2048 ± 1 LSB (Q12 quantization).
	if a[1] < -2049 || a[1] > -2047 {
		t.Errorf("a[1] = %d, want -2048 ± 1 (Q12 -0.5)", a[1])
	}
	for j := 2; j <= 10; j++ {
		if a[j] < -1 || a[j] > 1 {
			t.Errorf("a[%d] = %d, want 0 ± 1", j, a[j])
		}
	}
}

// TestLevinsonDurbin_StabilityKnown asserts that for a synthetic but
// strictly stable autocorrelation (Toeplitz-PSD), every reflection
// coefficient |k_i| < 1, which manifests as the prediction error
// E[i] remaining strictly positive across all 10 stages. We exercise
// that property indirectly by asserting the returned a[] coefficients
// are bounded — for this benign input none should saturate to
// ±32767. Input is the AR(1) above.
func TestLevinsonDurbin_StabilityKnown(t *testing.T) {
	var r [11]int32
	r[0] = 1 << 24
	for k := 1; k <= 10; k++ {
		r[k] = r[k-1] >> 1
	}
	var a [11]int16
	levinsonDurbin(&r, &a)
	for j := 0; j <= 10; j++ {
		if a[j] == 32767 || a[j] == -32768 {
			t.Errorf("a[%d] = %d saturated; benign AR(1) input must not saturate", j, a[j])
		}
	}
}

// TestLevinsonDurbin_Frame0Characterisation captures, for the first
// 240-sample analysis frame of the canonical LSP.IN test vector,
// the LP coefficients produced by AC-1 + AC-2 + LD-1. The values are
// pinned by direct §3.2.2 arithmetic in this codebase (no external
// implementation has been consulted). The capture is logged via
// t.Logf at this stage; it will be promoted to a hard assertion in
// Task 2a-INT-1 once L1/L2/L3 gate-equality is established on the
// same frame.
func TestLevinsonDurbin_Frame0Characterisation(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729AnnexA", "test_vectors", "LSP.IN")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("LSP.IN unavailable: %v", err)
	}
	if len(raw) < 480 {
		t.Skipf("LSP.IN too short: %d bytes", len(raw))
	}
	var speech [240]int16
	for n := 0; n < 240; n++ {
		speech[n] = int16(uint16(raw[2*n]) | uint16(raw[2*n+1])<<8)
	}
	var windowed [240]int16
	windowSpeech(&speech, &windowed)
	var r [11]int32
	_ = autocorrelate(&windowed, &r)
	applyLagWindow(&r)
	var a [11]int16
	levinsonDurbin(&r, &a)
	t.Logf("LSP.IN frame-0 LP coefficients (Q12):")
	for j := 0; j <= 10; j++ {
		t.Logf("  a[%2d] = %6d", j, a[j])
	}
}

// TestLevinsonDurbin_ZeroAllocation gates I4: the recursion must
// allocate no heap memory in steady state.
func TestLevinsonDurbin_ZeroAllocation(t *testing.T) {
	var r [11]int32
	r[0] = 1 << 24
	for k := 1; k <= 10; k++ {
		r[k] = r[k-1] >> 1
	}
	var a [11]int16
	allocs := testing.AllocsPerRun(50, func() {
		levinsonDurbin(&r, &a)
	})
	if allocs != 0 {
		t.Errorf("levinsonDurbin allocs/run = %v, want 0", allocs)
	}
}
