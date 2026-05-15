package tables

// CosLSP is the cosine lookup table used for LSF → LSP conversion
// per ITU-T G.729 §3.2.5. The table covers the full half-period
// [0, π] in 64 uniform steps (65 entries including both endpoints).
// Entry 0 corresponds to cos(0) = +1.0 and entry 64 corresponds to
// cos(π) = -1.0; the table is therefore monotonically non-increasing
// over its full extent.
//
// Entries are Q15 Word16. A linear interpolation between adjacent
// entries provides cos(ω) for any ω ∈ [0, π]; symmetry covers the
// remaining range when needed.
//
// The numerical values that satisfy the G.729 standard are part of
// the standard itself (single allowed values for bitstream
// interoperability) and are reproduced here under the merger
// doctrine. No algorithmic source code from any G.729
// implementation was consulted.
var CosLSP = [65]int16{
	32767, 32729, 32610, 32413, 32138, 31786, 31357, 30853,
	30274, 29622, 28899, 28106, 27246, 26320, 25330, 24279,
	23170, 22006, 20788, 19520, 18205, 16846, 15447, 14010,
	12540, 11039, 9512, 7962, 6393, 4808, 3212, 1608,
	0, -1608, -3212, -4808, -6393, -7962, -9512, -11039,
	-12540, -14010, -15447, -16846, -18205, -19520, -20788, -22006,
	-23170, -24279, -25330, -26320, -27246, -28106, -28899, -29622,
	-30274, -30853, -31357, -31786, -32138, -32413, -32610, -32729,
	-32768,
}

// CosLSPSlope is the Q15 interpolation slope table paired with CosLSP
// for LSF → LSP conversion. The value at index i is multiplied by the
// 8-bit intra-cell fraction and shifted right by 12 before being added
// to CosLSP[i].
var CosLSPSlope = [64]int16{
	-631, -1893, -3150, -4399, -5638, -6863, -8072, -9261,
	-10428, -11570, -12684, -13767, -14817, -15832, -16808, -17744,
	-18637, -19486, -20287, -21039, -21741, -22390, -22986, -23526,
	-24009, -24435, -24801, -25108, -25354, -25540, -25664, -25726,
	-25726, -25664, -25540, -25354, -25108, -24801, -24435, -24009,
	-23526, -22986, -22390, -21741, -21039, -20287, -19486, -18637,
	-17744, -16808, -15832, -14817, -13767, -12684, -11570, -10428,
	-9261, -8072, -6863, -5638, -4399, -3150, -1893, -631,
}
