package lpc

// autocorrelate computes the §3.2.1 eq. 5 autocorrelation
//
//	r(k) = Σ_{n=k}^{239} s'(n) · s'(n-k)    for k = 0..10
//
// of a 240-sample windowed speech buffer (Q0 int16, post-§3.2.1
// eq. 4). All eleven values are returned in a shared Word32 (Q0)
// representation; the returned scale is the right-shift in bits
// applied to the input samples prior to accumulation. A caller that
// needs the unshifted magnitude must left-shift each r[k] by
// 2*scale (equivalently, the input was scaled by 2^-scale, so the
// product was scaled by 2^-2scale).
//
// Overflow-recovery policy. §3.2.1 line 691 ("to avoid arithmetic
// problems") leaves the normalization unspecified. Worst case
// 240 · 32767² ≈ 2.58·10¹¹ exceeds 2³¹−1 (≈2.15·10⁹), so for
// high-energy frames an in-place right shift on s'(n) is required.
// We pick the minimal shared shift such that r(0), the largest of
// the eleven values (Cauchy-Schwarz: |r(k)| ≤ r(0)), fits in
// Word32. The same shift is applied symmetrically to both factors,
// giving a uniform Q-format across all r[k] for the downstream
// lag-window / Levinson stages.
//
// Zero allocation: caller owns both buffers; the routine uses only
// a stack-resident scratch [240]int32 for the pre-shifted samples.
func autocorrelate(windowed *[240]int16, r *[11]int32) (scale int) {
	const maxWord32 = int64(1<<31 - 1)

	var sumSq int64
	for n := 0; n < 240; n++ {
		v := int64(windowed[n])
		sumSq += v * v
	}

	// Choose shift so that floor(sumSq / 4^scale) ≤ MaxInt32. Each
	// shift on the inputs divides every product by 4. For a true DC
	// max input (32767) this terminates after at most 4 iterations
	// (4^4 = 256 > 240·32767² / 2³¹ ≈ 120).
	for sumSq > maxWord32 {
		sumSq >>= 2
		scale++
	}

	if scale == 0 {
		// Fast path: accumulate directly in Word32 without a
		// temporary. r(0) is known to fit (sumSq ≤ MaxInt32) and
		// |r(k)| ≤ r(0) by Cauchy-Schwarz, so all eleven products
		// stay in range.
		for k := 0; k <= 10; k++ {
			var acc int32
			for n := k; n < 240; n++ {
				acc += int32(windowed[n]) * int32(windowed[n-k])
			}
			r[k] = acc
		}
		return 0
	}

	var s [240]int32
	for n := 0; n < 240; n++ {
		s[n] = int32(windowed[n]) >> uint(scale)
	}
	for k := 0; k <= 10; k++ {
		var acc int32
		for n := k; n < 240; n++ {
			acc += s[n] * s[n-k]
		}
		r[k] = acc
	}
	return scale
}
