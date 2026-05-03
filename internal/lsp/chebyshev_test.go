package lsp

import (
	"math"
	"testing"
)

// TestChebyshevC evaluates §3.2.3 eq. 17 C(x) via the back-recursion
// of lines 794–799 against analytical Chebyshev polynomial values.
//
// With f(1..5) all zero (and the unused f(0) arbitrary), eq. 17 reduces
// to C(x) = T_5(x) = cos(5·arccos(x)). With f(5) = 2·1.0 (Q24 = 2^25)
// the constant f(5)/2 = 1.0 is added. With f(1) = 2·1.0 a 2·T_4(x)
// term is added. These three families pin both the recursion path
// (b[k] update with f(5−k)) and the closing term (f(5)/2).
func TestChebyshevC(t *testing.T) {
	const oneQ24 int32 = 1 << 24
	// Q15 sample points: x = cos(θ) for θ ∈ {π/8, π/4, π/2, 3π/4, 7π/8}.
	// Endpoints θ=0,π give x=±1 which is unrepresentable in Q15 (would
	// need 2^15) and trigger systematic 6e-5 quantization error per
	// recursion step that accumulates linearly with |f|; interior points
	// stay safely within the ±2^14 (Q24) tolerance.
	thetas := []float64{math.Pi / 8, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, 7 * math.Pi / 8}

	cases := []struct {
		name string
		f    [6]int32
		// realC returns the analytical C(cos(θ)) value (real, not Q24).
		realC func(theta float64) float64
	}{
		{
			name: "T5_only",
			f:    [6]int32{0, 0, 0, 0, 0, 0},
			realC: func(theta float64) float64 {
				return math.Cos(5 * theta)
			},
		},
		{
			name: "T5_plus_constant_one",
			// f(5) = 2.0 in Q24 → f(5)/2 = 1.0
			f: [6]int32{0, 0, 0, 0, 0, 2 * oneQ24},
			realC: func(theta float64) float64 {
				return math.Cos(5*theta) + 1.0
			},
		},
		{
			name: "T5_plus_2T4",
			// f(1) = 2.0 in Q24 → adds 2·T_4(x) = 2·cos(4θ)
			f: [6]int32{0, 2 * oneQ24, 0, 0, 0, 0},
			realC: func(theta float64) float64 {
				return math.Cos(5*theta) + 2*math.Cos(4*theta)
			},
		},
	}

	const tol int32 = 1 << 14 // ±2^14 in Q24 per plan

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, theta := range thetas {
				xReal := math.Cos(theta)
				// x in Q15. Clamp to ±32767 to avoid int16 overflow at ±1.
				xScaled := math.Round(xReal * 32768)
				if xScaled > 32767 {
					xScaled = 32767
				}
				if xScaled < -32767 {
					xScaled = -32767
				}
				xQ15 := int16(xScaled)

				got := chebyshevC(xQ15, &tc.f)

				wantReal := tc.realC(theta)
				want := int32(math.Round(wantReal * float64(int64(1)<<24)))
				diff := got - want
				if diff < 0 {
					diff = -diff
				}
				if diff > tol {
					t.Errorf("theta=%v x=%v: got C=%d, want %d (real %v), |diff|=%d > %d",
						theta, xReal, got, want, wantReal, diff, tol)
				}
			}
		})
	}
}
