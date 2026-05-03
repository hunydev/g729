package lsp

// computeF1F2 derives the F1(z) and F2(z) sum/difference polynomial
// coefficients f1(0..5), f2(0..5) from the Q12 LP filter a[0..10] per
// ITU-T G.729 §3.2.3 eq. 9–15:
//
//	f1(0) = f2(0) = 1.0
//	f1(i+1) = a[i+1] + a[10-i] − f1(i)   i = 0..4
//	f2(i+1) = a[i+1] − a[10-i] + f2(i)   i = 0..4
//
// The Q12 input coefficients are promoted to Q24 internally so that
// the additive recursion retains headroom (sums of at most ~16 in real
// magnitude → ~16·2^24 < 2^29, comfortably within Word32). Output
// arrays are filled in place with Q24 Word32 values.
func computeF1F2(a *[11]int16, f1, f2 *[6]int32) {
	const oneQ24 int32 = 1 << 24
	f1[0] = oneQ24
	f2[0] = oneQ24

	for i := 0; i < 5; i++ {
		ai1 := int32(a[i+1]) << 12   // Q12 → Q24
		a10i := int32(a[10-i]) << 12 // Q12 → Q24

		f1[i+1] = ai1 + a10i - f1[i]
		f2[i+1] = ai1 - a10i + f2[i]
	}
}

// chebyshevC evaluates the polynomial C(x) of ITU-T G.729 §3.2.3 eq. 17
//
//	C(x) = T₅(x) + f(1)T₄(x) + f(2)T₃(x) + f(3)T₂(x) + f(4)T₁(x) + f(5)/2
//
// using the back-recursion of §3.2.3 lines 794–799:
//
//	b[5] = 1, b[6] = 0
//	for k = 4 down to 1:  b[k] = 2x·b[k+1] − b[k+2] + f(5−k)
//	C(x) = x·b[1] − b[2] + f(5)/2
//
// Inputs:
//   - x in Q15 (the cosine-domain abscissa, |x| ≤ 1)
//   - f[0..5] in Q24 (the F1 or F2 coefficients from computeF1F2;
//     index 0 is unused by eq. 17)
//
// Output: C(x) in Q24 as a Word32. Pure function, no allocation.
func chebyshevC(x int16, f *[6]int32) int32 {
	const oneQ24 int32 = 1 << 24

	// b[5] = 1.0 in Q24; b[6] = 0. The recursion only needs the
	// rolling pair (b[k+1], b[k+2]) so we keep two scalars.
	bk1 := oneQ24 // b[k+1], starts as b[5]
	bk2 := int32(0) // b[k+2], starts as b[6]

	x32 := int64(x)
	for k := 4; k >= 1; k-- {
		// 2·x·b[k+1]: Q15 * Q24 = Q39, doubled and shifted right 15 → Q24.
		twoXB := int32((2 * x32 * int64(bk1)) >> 15)
		bk := twoXB - bk2 + f[5-k]
		bk2 = bk1
		bk1 = bk
	}

	// C(x) = x·b[1] − b[2] + f(5)/2.
	xB1 := int32((x32 * int64(bk1)) >> 15)
	return xB1 - bk2 + (f[5] >> 1)
}

// grid60 holds the 60 cosine grid abscissae x_k = cos(ω_k) with
// ω_k = k·π/59 for k = 0..59 — i.e. 60 points equally spaced in ω
// across [0, π] per ITU-T G.729 §3.2.3 line 783. Endpoints are
// pinned to the exact Q15 ±full-scale values so the scan begins at
// x = +1 (ω = 0) and ends at x = −1 (ω = π); intermediate samples
// are produced once at package init by reusing the lsfToLSP cosine
// interpolation over tables.CosLSP. Init-time population keeps
// findLSPRoots allocation-free in steady state (I4).
var grid60 [60]int16

func init() {
	// ω_k in Q13 = k · π_Q13 / 59. π_Q13 = 25736, matching the
	// existing lspMaxOmega convention used by lsfToLSP.
	const piQ13 int32 = 25736
	for k := 0; k < 60; k++ {
		omega := (int32(k) * piQ13) / 59
		if omega > piQ13 {
			omega = piQ13
		}
		grid60[k] = lsfToLSP(int16(omega))
	}
	// Pin endpoints to exact ±full-scale Q15. lsfToLSP interpolates
	// over a 64-cell table whose Q13 anchor (25728) is one LSB short
	// of π_Q13 (25736), so the k=59 sample drifts off −32768 by a
	// few LSBs; pinning eliminates that bookkeeping noise without
	// affecting any interior abscissa.
	grid60[0] = 32767
	grid60[59] = -32768
}

// findLSPRoots locates the 5 roots of C_F1 and the 5 roots of C_F2
// on x = cos(ω) ∈ [−1, +1] via the §3.2.3 sign-change scan (lines
// 782–784): C is evaluated on a 60-point grid uniformly spaced in
// ω; each detected sign change is refined by 4 successive binary
// subdivisions; the final root estimate is the midpoint of the last
// surviving sub-interval. Roots are interleaved into q[0..9] in
// strictly increasing-ω (decreasing-x) order — F1 supplies q[0],
// q[2], q[4], q[6], q[8] and F2 supplies q[1], q[3], q[5], q[7],
// q[9], matching the §3.2.3 / §3.2.6 even/odd convention.
//
// Returns ErrLPCNonStable when fewer than 5 sign changes are
// detected for either polynomial (Levinson defect upstream → E8).
//
// I4: zero allocation. I11: 60-point grid + 4 bisections, both
// hard-coded.
func findLSPRoots(f1, f2 *[6]int32, q *[10]int16) error {
	var rootsF1, rootsF2 [5]int16
	var nF1, nF2 int

	xPrev := grid60[0]
	cPrev1 := chebyshevC(xPrev, f1)
	cPrev2 := chebyshevC(xPrev, f2)

	for k := 1; k < 60; k++ {
		x := grid60[k]
		c1 := chebyshevC(x, f1)
		c2 := chebyshevC(x, f2)

		if nF1 < 5 && signsDiffer(cPrev1, c1) {
			rootsF1[nF1] = bisectRoot(xPrev, x, cPrev1, c1, f1)
			nF1++
		}
		if nF2 < 5 && signsDiffer(cPrev2, c2) {
			rootsF2[nF2] = bisectRoot(xPrev, x, cPrev2, c2, f2)
			nF2++
		}

		xPrev = x
		cPrev1 = c1
		cPrev2 = c2
	}

	if nF1 < 5 || nF2 < 5 {
		return ErrLPCNonStable
	}

	// Per §3.2.3: roots of F1 and F2 interlace and the first root in
	// ω order belongs to F1. We scanned k increasing (ω increasing,
	// x decreasing), so rootsF1 / rootsF2 are each already in
	// decreasing-x = increasing-ω order. Interleave directly.
	for i := 0; i < 5; i++ {
		q[2*i] = rootsF1[i]
		q[2*i+1] = rootsF2[i]
	}
	return nil
}

// signsDiffer treats 0 as non-negative; a sign change is reported
// when one value is strictly negative and the other is ≥ 0. This
// matches the §3.2.3 "sign change" criterion and never double-counts
// an exact-zero grid hit.
func signsDiffer(a, b int32) bool { return (a < 0) != (b < 0) }

// bisectRoot performs 4 successive binary subdivisions of the
// interval [xLo, xHi] (with xLo > xHi in cosine domain since ω is
// increasing) on which C changes sign, then returns the midpoint of
// the final sub-interval as the Q15 root estimate. cLo / cHi are
// the cached C values at the interval endpoints; chebyshevC is
// invoked exactly 4 times per call.
func bisectRoot(xLo, xHi int16, cLo, cHi int32, f *[6]int32) int16 {
	for i := 0; i < 4; i++ {
		mid := int16((int32(xLo) + int32(xHi)) >> 1)
		cMid := chebyshevC(mid, f)
		if signsDiffer(cLo, cMid) {
			xHi = mid
			cHi = cMid
		} else {
			xLo = mid
			cLo = cMid
		}
	}
	_ = cHi
	return int16((int32(xLo) + int32(xHi)) >> 1)
}

// LPToLSP converts a quantized 10th-order LP filter a[0..10] in Q12
// into its 10 LSP cosines q[0..9] in Q15 per ITU-T G.729 §3.2.3
// (lines 738–799). The wrapper composes the three building blocks
// of Task family LP:
//
//LP-1: computeF1F2 builds the F1(z)/F2(z) sum/difference
//      polynomial coefficients in Q24 (eq. 13–15).
//LP-2: chebyshevC evaluates each polynomial in the cosine domain
//      via Chebyshev back-recursion (eq. 17, lines 794–799).
//LP-3: findLSPRoots scans a 60-point ω-grid for sign changes and
//      refines each detected root with 4 binary subdivisions
//      (lines 784); the I11-binding (60, 4) configuration.782
//
// Output q is filled in strictly decreasing-x = strictly increasing-ω
// order, with F1 supplying the even-indexed roots q[0,2,4,6,8] and
// F2 the odd-indexed roots q[1,3,5,7,9].
//
// Returns ErrLPCNonStable when either polynomial yields fewer than
// five sign changes on the grid (E8 path; signals a defective LP
// from upstream Levinson).
//
// I4: the f1/f2 scratch buffers live on the caller's stack frame; no
// heap allocation occurs in steady state. I3: pure function, no
// panics, no goroutines, no logging.
func LPToLSP(a *[11]int16, q *[10]int16) error {
var f1, f2 [6]int32
computeF1F2(a, &f1, &f2)
return findLSPRoots(&f1, &f2, q)
}
