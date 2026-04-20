package pcm

// High-pass pre-processing filter coefficients from ITU-T G.729 §3.1.1:
//
//	H_h1(z) = (0.46363718 − 0.92724705·z^-1 + 0.46363718·z^-2)
//	          / (1 − 1.9059465·z^-1 + 0.9114024·z^-2)
//
// The 1/2 input scaling mandated by the spec is already folded into
// the numerator coefficients above.
//
// Denominator coefficients (a1, a2) have magnitudes less than 2, so
// Q13 fits with headroom: scaling by 2^13 = 8192 keeps the rounded
// values inside ±Max16.
//
// Numerator coefficients (b0, b1, b2) have magnitudes less than 1, so
// Q13 also fits and matches the denominator precision.
//
// If you change these Q-formats, update coeffs_test.go's expected ULP.
const (
	A1Q = 13 // Q-format exponent for A1
	A2Q = 13 // Q-format exponent for A2
	BQ  = 13 // Q-format exponent for B0, B1, B2
)

// Integer Q-format coefficients. Values are round(real·2^Q):
//
//	A1 = round( 1.9059465 · 2^13) = round( 15613.514) =  15614
//	A2 = round(-0.9114024 · 2^13) = round( -7466.187) =  -7466
//	B0 = round( 0.46363718 · 2^13) = round(  3798.137) =   3798
//	B1 = round(-0.92724705 · 2^13) = round( -7596.009) =  -7596
//	B2 = round( 0.46363718 · 2^13) = round(  3798.137) =   3798
//
// These were derived by hand from the ITU-T G.729 §3.1.1 real-valued
// coefficients; the plan's worked example contained an arithmetic
// slip on A1 and A2 (15615 / -7465 are off by one in their rounding
// step), so the values above are recomputed and the TestCoefficientValues
// ULP guard catches any future regression.
const (
	A1 int16 = 15614
	A2 int16 = -7466

	B0 int16 = 3798
	B1 int16 = -7596
	B2 int16 = 3798
)
