package tables

// Pow2Table is the fixed-point pow2 approximation LUT per ITU-T
// G.729 §3.9 (the tabpow[] data array in tab_ld8a.c).
//
// 33 entries representing 2^(i/32) for i ∈ [0, 32] in Q14:
//
//	Pow2Table[0]  = 16384  (= 2^0       · 2^14)
//	Pow2Table[32] = 32767  (≈ 2^1       · 2^14, saturated)
//
// Used by pow2Fixed for forward lookup with linear interpolation.
//
// Values transcribed from tab_ld8a.c data-array initializer under
// the merger-doctrine exception (see scratch-from-spec policy).
var Pow2Table = [33]int16{
	16384, 16743, 17109, 17484, 17867, 18258, 18658, 19066, 19484, 19911,
	20347, 20792, 21247, 21713, 22188, 22674, 23170, 23678, 24196, 24726,
	25268, 25821, 26386, 26964, 27554, 28158, 28774, 29405, 30048, 30706,
	31379, 32066, 32767,
}

// Log2Table is the fixed-point log2 fractional-part LUT per ITU-T
// G.729 §3.9 (the tablog[] data array in tab_ld8a.c).
//
// 33 entries representing log2(1 + i/32) for i ∈ [0, 32] in Q15:
//
//	Log2Table[0]  = 0      (= log2(1))
//	Log2Table[32] = 32767  (≈ log2(2) in Q15, saturated)
//
// Used by log2Fixed to interpolate the fractional component once
// the input has been normalized (via fixed.NormL) to a [1, 2)
// mantissa.
//
// Values transcribed from tab_ld8a.c data-array initializer under
// the merger-doctrine exception.
var Log2Table = [33]int16{
	0, 1455, 2866, 4236, 5568, 6863, 8124, 9352, 10549, 11716,
	12855, 13967, 15054, 16117, 17156, 18172, 19167, 20142, 21097, 22033,
	22951, 23852, 24735, 25603, 26455, 27291, 28113, 28922, 29716, 30497,
	31266, 32023, 32767,
}
