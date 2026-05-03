package lsp

import (
	"math"
	"testing"
)

// expectedWeightsLSF is the floating-point oracle for ITU-T G.729
// §3.2.4 equation (22), evaluated in Q11. Used as an independent
// reference for the fixed-point assertions on weightsLSF.
func expectedWeightsLSF(omega *[10]int16) [10]int16 {
	var om [10]float64
	for i := range omega {
		om[i] = float64(omega[i]) / 8192.0
	}
	var w [10]float64

	branch := func(arg float64) float64 {
		if arg > 0 {
			return 1.0
		}
		return 1.0 / (10*arg*arg + 1.0)
	}

	w[0] = branch(om[1] - 0.04*math.Pi - 1.0)
	for i := 1; i <= 8; i++ {
		w[i] = branch(om[i+1] - om[i-1] - 1.0)
	}
	w[9] = branch(-om[8] + 0.92*math.Pi - 1.0)

	w[4] *= 1.2
	w[5] *= 1.2

	var out [10]int16
	for i, v := range w {
		r := math.Round(v * 2048.0)
		switch {
		case r > 32767:
			out[i] = 32767
		case r < -32768:
			out[i] = -32768
		default:
			out[i] = int16(r)
		}
	}
	return out
}

func assertWeightsClose(t *testing.T, name string, got, want [10]int16, tol int) {
	t.Helper()
	for i := 0; i < 10; i++ {
		if d := int(got[i]) - int(want[i]); d < -tol || d > tol {
			t.Errorf("%s: i=%d got=%d want=%d (diff=%d, tol=±%d)", name, i, got[i], want[i], d, tol)
		}
	}
}

// TestWeightsLSF_UniformSpacing exercises a uniform LSF grid
// ω_i = (i+1)·π/11 (Q13, 1-based "ω_i = i·π/11"). All adjacent
// 1-based gaps ω_{i+1}-ω_{i-1} = 2π/11 ≈ 0.5712 < 1.0, so every
// branch of eq. (22) takes the "otherwise" arm; w_5 and w_6
// (1-based) receive the ×1.2 boost on top.
func TestWeightsLSF_UniformSpacing(t *testing.T) {
	const piQ13 = 25736
	var omega [10]int16
	for i := 0; i < 10; i++ {
		omega[i] = int16(((i+1)*piQ13 + 5) / 11)
	}

	var got [10]int16
	weightsLSF(&omega, &got)
	want := expectedWeightsLSF(&omega)
	assertWeightsClose(t, "uniform", got, want, 3)
}

// TestWeightsLSF_ClusterAtFive constructs a synthetic ω where the
// gap that drives w_5 (omega[5]-omega[3] in 0-based; ω_6 − ω_4 in
// 1-based) sits just under 1.0, forcing the eq. (22) "otherwise"
// branch but with a near-unity raw weight. The ×1.2 boost on w_5
// (line 882) then visibly pushes the Q11 result past 2048.
func TestWeightsLSF_ClusterAtFive(t *testing.T) {
	const piQ13 = 25736
	var omega [10]int16
	for i := 0; i < 10; i++ {
		omega[i] = int16(((i+1)*piQ13 + 5) / 11)
	}
	// Set omega[3] and omega[5] so that omega[5]-omega[3] = 0.95 rad
	// (Q13 = 7782) → arg = -0.05 → raw w_5 ≈ 0.976 (Q11 1999) →
	// boosted ≈ 1.171 (Q11 2399).
	omega[3] = 8000
	omega[5] = 8000 + 7782
	// Mirror for omega[4] and omega[6] so w_6 (omega[6]-omega[4])
	// also lands in (0.83, 1.0).
	omega[4] = 9000
	omega[6] = 9000 + 7782

	var got [10]int16
	weightsLSF(&omega, &got)
	want := expectedWeightsLSF(&omega)
	assertWeightsClose(t, "cluster", got, want, 3)

	if got[4] <= 2048 {
		t.Errorf("expected boosted w_5 (got[4]) > 2048 (Q11 1.0); got %d", got[4])
	}
	if got[5] <= 2048 {
		t.Errorf("expected boosted w_6 (got[5]) > 2048 (Q11 1.0); got %d", got[5])
	}
}

// TestWeightsLSF_SmallOmega2 forces the w_1 (1-based) edge case
// where ω_2 < 0.04π, sending the (ω_2 − 0.04π − 1) test deep into
// the otherwise branch. Verified by hand:
//
//	omega[1] = 500 → 500/8192 − 0.04π − 1 ≈ −1.0647
//	w_1 ≈ 1/(10·1.0647² + 1) ≈ 0.0811 → Q11 ≈ 166
func TestWeightsLSF_SmallOmega2(t *testing.T) {
	const piQ13 = 25736
	var omega [10]int16
	for i := 0; i < 10; i++ {
		omega[i] = int16(((i+1)*piQ13 + 5) / 11)
	}
	omega[1] = 500

	var got [10]int16
	weightsLSF(&omega, &got)
	want := expectedWeightsLSF(&omega)
	assertWeightsClose(t, "smallOmega2", got, want, 3)

	if got[0] < 150 || got[0] > 180 {
		t.Errorf("expected w_1 ≈ 166 (Q11), got %d", got[0])
	}
}
