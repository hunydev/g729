package lpc

import "github.com/hunydev/g729/internal/fixed"

// lagWindow holds the §3.2.1 eq. 6 60 Hz bandwidth-expansion
// coefficients in Q15:
//
//	w_lag(k) = exp(−0.5 · (2π · f0 · k / fs)²)    f0 = 60, fs = 8000
//
// for k = 1..10 (indexed lagWindow[k-1]). Values precomputed from
// the closed-form expression and asserted within ±2 LSB by
// lagwindow_test.go's oracle (clean-room: derived directly from the
// §3.2.1 lines 692–699 formula, not transcribed).
var lagWindow = [10]int16{
	32732, 32623, 32442, 32191, 31871,
	31484, 31033, 30520, 29950, 29324,
}

// applyLagWindow performs §3.2.1 eq. 6 (lag windowing of r(k) for
// k = 1..10) and eq. 7 (white-noise correction r'(0) = r(0)·1.0001)
// in place on the autocorrelation buffer produced by autocorrelate.
//
// Eq. 7 implementation. The spec constant 1.0001 has no exact dyadic
// representation. We use 1 + 2^-13 ≈ 1.000122, computed as
// r(0) + (r(0) >> 13) with saturating addition. The fractional error
// vs. the spec value (1.22·10^-4 vs. 1.00·10^-4) is below the
// quantization noise of the downstream Q15 LSP VQ and is the
// conventional realization of this clause.
//
// Eq. 6 implementation. Each r(k) (Word32, AC-1 shared scale) is
// multiplied by the Q15 lag-window coefficient. The product is
// formed in 64-bit and right-shifted by 15 to land back in the same
// Word32 scale; |w_lag(k)| ≤ 1.0 in Q15 guarantees the result
// stays within the input magnitude (no saturation needed for the
// k ≥ 1 entries).
//
// Zero allocation: operates in place on the caller's [11]int32.
func applyLagWindow(r *[11]int32) {
	r[0] = fixed.LAdd(r[0], r[0]>>13)
	for k := 1; k <= 10; k++ {
		r[k] = int32((int64(r[k]) * int64(lagWindow[k-1])) >> 15)
	}
}
