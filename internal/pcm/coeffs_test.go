package pcm

import (
	"math"
	"testing"
)

// TestCoefficientValues asserts that the integer Q-format coefficients
// defined in coeffs.go, when converted back to real numbers, agree
// with the ITU-T G.729 §3.1.1 filter specification within one unit of
// least precision (ULP) of the chosen Q-format.
func TestCoefficientValues(t *testing.T) {
	cases := []struct {
		name    string
		got     int16
		want    float64
		qFormat uint
	}{
		{"A1", A1, 1.9059465, A1Q},
		{"A2", A2, -0.9114024, A2Q},
		{"B0", B0, 0.46363718, BQ},
		{"B1", B1, -0.92724705, BQ},
		{"B2", B2, 0.46363718, BQ},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scale := float64(int64(1) << tc.qFormat)
			actual := float64(tc.got) / scale
			tol := 1.0 / scale
			if math.Abs(actual-tc.want) > tol {
				t.Errorf("%s = %d (Q%d, ≈ %.8f), want %.8f ± %.8f",
					tc.name, tc.got, tc.qFormat, actual, tc.want, tol)
			}
		})
	}
}
