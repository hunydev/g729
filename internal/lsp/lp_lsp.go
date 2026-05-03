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
