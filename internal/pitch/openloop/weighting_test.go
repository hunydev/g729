package openloop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/lpc"
)

// TestGammaWeightLP_Identity covers the trivial input where â = 1
// (Q12 = 4096) and all higher taps are zero. γⁱ·0 = 0 for i ≥ 1 so
// the output must be the same identity polynomial.
func TestGammaWeightLP_Identity(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	want := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var got [11]int16
	gammaWeightLP(&a, &got)
	if got != want {
		t.Fatalf("gammaWeightLP(identity) = %v, want %v", got, want)
	}
}

// TestGammaWeightLP_SingleTap verifies the first-tap scaling: with
// a[1] = -2048 (Q12, = -0.5) and γ = 0.75 the result is -1536
// (Q12, = -0.375) per fixed.Mult (Q15·Q12 → Q12).
func TestGammaWeightLP_SingleTap(t *testing.T) {
	a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	want := [11]int16{4096, -1536, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var got [11]int16
	gammaWeightLP(&a, &got)
	if got != want {
		t.Fatalf("gammaWeightLP(single-tap) = %v, want %v", got, want)
	}
}

// TestCombineWith07_Identity covers the case where Â(z/γ) = 1: the
// resulting A'(z) reduces to (1 − 0.7z⁻¹). The leading tap is the
// Q12 unit 4096 and the z⁻¹ tap is -0.7 in Q12.
func TestCombineWith07_Identity(t *testing.T) {
	aw := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	want := [11]int16{4096, -2867, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var got [11]int16
	combineWith07(&aw, &got)
	if got != want {
		t.Fatalf("combineWith07(identity) = %v, want %v", got, want)
	}
}

// TestCombineWith07_SingleTap verifies the two-tap convolution
// against a hand-traced expected output. With aw = [4096, -1536, 0,
// …, 0] (the gammaWeightLP image of [4096, -2048, 0, …, 0]) the
// expected A'(z) coefficients are [4096, -4403, 1075, 0, …, 0].
func TestCombineWith07_SingleTap(t *testing.T) {
	aw := [11]int16{4096, -1536, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	want := [11]int16{4096, -4403, 1075, 0, 0, 0, 0, 0, 0, 0, 0}
	var got [11]int16
	combineWith07(&aw, &got)
	if got != want {
		t.Fatalf("combineWith07(single-tap) = %v, want %v", got, want)
	}
}

// TestCombineWith07_Frame0Characterisation logs A'(z) for the
// canonical PITCH.IN frame-0 input. The coefficients are not
// asserted at WS-1 (the test is an observational dump); Task 2b-INT-1
// will promote the log to a hard assertion once the chain is wired
// through encoder.go.
func TestCombineWith07_Frame0Characterisation(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "itu",
		"G729_Release3", "g729AnnexA", "test_vectors", "PITCH.IN")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("PITCH.IN unavailable: %v", err)
	}
	if len(raw) < 2*lpc.LPCWindowSamples {
		t.Skipf("PITCH.IN too short: %d bytes", len(raw))
	}
	var speech [lpc.LPCWindowSamples]int16
	for n := 0; n < lpc.LPCWindowSamples; n++ {
		speech[n] = int16(uint16(raw[2*n]) | uint16(raw[2*n+1])<<8)
	}
	var an lpc.Analyzer
	var aQ12 [lpc.LPCOrder + 1]int16
	if err := an.Analyze(&speech, &aQ12); err != nil {
		t.Fatalf("lpc.Analyze: %v", err)
	}
	var aw, aPrime [11]int16
	gammaWeightLP(&aQ12, &aw)
	combineWith07(&aw, &aPrime)
	t.Logf("PITCH.IN frame-0 unweighted â (Q12):")
	for i := 0; i <= 10; i++ {
		t.Logf("  a[%2d]      = %6d", i, aQ12[i])
	}
	t.Logf("PITCH.IN frame-0 Â(z/γ) aw (Q12):")
	for i := 0; i <= 10; i++ {
		t.Logf("  aw[%2d]     = %6d", i, aw[i])
	}
	t.Logf("PITCH.IN frame-0 A'(z) aPrime (Q12 leading; see weighting.go for tap-1 convention):")
	for i := 0; i <= 10; i++ {
		t.Logf("  aPrime[%2d] = %6d", i, aPrime[i])
	}
}

// TestGammaWeightLP_ZeroAllocation gates I4 for the γ-weighting
// step.
func TestGammaWeightLP_ZeroAllocation(t *testing.T) {
	a := [11]int16{4096, -2048, 1024, -512, 256, -128, 64, -32, 16, -8, 4}
	var out [11]int16
	allocs := testing.AllocsPerRun(128, func() {
		gammaWeightLP(&a, &out)
	})
	if allocs != 0 {
		t.Fatalf("gammaWeightLP allocs/run = %v, want 0", allocs)
	}
}

// TestCombineWith07_ZeroAllocation gates I4 for the (1 − 0.7z⁻¹)
// convolution step.
func TestCombineWith07_ZeroAllocation(t *testing.T) {
	aw := [11]int16{4096, -1536, 768, -384, 192, -96, 48, -24, 12, -6, 3}
	var out [11]int16
	allocs := testing.AllocsPerRun(128, func() {
		combineWith07(&aw, &out)
	})
	if allocs != 0 {
		t.Fatalf("combineWith07 allocs/run = %v, want 0", allocs)
	}
}
