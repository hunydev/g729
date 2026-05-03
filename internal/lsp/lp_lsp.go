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
