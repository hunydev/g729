package postfilter

import "testing"

// γ = 32767 (≈ 1.0 in Q15): bandwidth expansion should be near-identity
// (modulo 1-LSB Q15 truncation — 32767 is 1.0 − 2^-15).
func TestExpandBandwidth_GammaNearOneIsIdentity(t *testing.T) {
	a := [11]int16{4096, 1000, -500, 300, -100, 50, 25, -10, 5, -2, 1}
	var out [11]int16

	expandBandwidth(&a, 32767, &out)

	if out[0] != 4096 {
		t.Errorf("out[0] = %d, want 4096", out[0])
	}
	for i := 1; i <= 10; i++ {
		diff := int(out[i]) - int(a[i])
		if diff < -2 || diff > 2 {
			t.Errorf("out[%d] = %d, want ≈ %d (γ≈1.0, tol ±2)", i, out[i], a[i])
		}
	}
}

// γ = 0 (Q15): all tail coefficients must be zeroed; a[0] stays 4096.
func TestExpandBandwidth_ZeroGammaZerosTail(t *testing.T) {
	a := [11]int16{4096, 1000, -500, 300, -100, 50, 25, -10, 5, -2, 1}
	var out [11]int16

	expandBandwidth(&a, 0, &out)

	if out[0] != 4096 {
		t.Errorf("out[0] = %d, want 4096", out[0])
	}
	for i := 1; i <= 10; i++ {
		if out[i] != 0 {
			t.Errorf("out[%d] = %d, want 0 (γ = 0)", i, out[i])
		}
	}
}

func TestExpandBandwidth_HalfGammaGeometricDecay(t *testing.T) {
	a := [11]int16{4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096}
	var out [11]int16

	expandBandwidth(&a, 16384, &out)

	if out[0] != 4096 {
		t.Errorf("out[0] = %d, want 4096", out[0])
	}
	for i := 1; i <= 10; i++ {
		prev := int32(out[i-1])
		want := prev / 2
		got := int32(out[i])
		if got < want-2 || got > want+2 {
			t.Errorf("out[%d] = %d, want ≈ %d (half of %d, tol ±2)",
				i, got, want, prev)
		}
	}
}
