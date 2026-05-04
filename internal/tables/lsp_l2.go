package tables

// LSPCodebookL2 is the 5-bit second-stage lower-split LSF quantizer
// codebook from ITU-T G.729 §3.2.4. Each of the 32 rows is a
// 5-element LSF residual vector covering the lower half of the
// 10-LSF vector (LSFs 1..5), in Q13 Word16.
//
// The numerical values that satisfy the G.729 standard are part of
// the standard itself (single allowed values for bitstream
// interoperability) and are reproduced here under the merger
// doctrine. No algorithmic source code from any G.729
// implementation was consulted.
var LSPCodebookL2 = [32][5]int16{
	{-435, -815, -742, 1033, -518},
	{-833, -891, 463, -8, -1251},
	{-1021, 231, -306, 321, -220},
	{57, -198, -339, -33, -1468},
	{171, -350, 294, 1660, 453},
	{-701, -842, -58, 950, 892},
	{584, 31, -289, 356, -333},
	{-109, -808, 231, 77, -87},
	{-859, 1236, 550, 854, 714},
	{-877, -954, -1248, -299, 212},
	{-77, 344, -620, 763, 413},
	{-314, -307, -256, -1260, -429},
	{711, 693, 521, 650, 1305},
	{-112, -271, -500, 946, 1733},
	{575, -10, -468, -199, 1101},
	{145, -285, -1280, -398, 36},
	{-1133, -835, 1350, 1284, -95},
	{-1459, -1237, 416, -213, 466},
	{-15, 66, 468, 1019, -748},
	{-338, 148, 1445, 75, -760},
	{389, 239, 1568, 981, 113},
	{-312, -98, 949, 31, 1104},
	{1127, 584, 835, 277, -1159},
	{539, -114, 856, -493, 223},
	{2197, 2337, 1268, 670, 304},
	{-1596, 550, 801, -456, -56},
	{1154, 593, -77, 1237, -31},
	{397, 558, 203, -797, -919},
	{334, 1475, 632, -80, 48},
	{-545, -330, -429, -680, 1133},
	{1320, 827, -398, -576, 341},
	{-163, 674, -11, -886, 531},
}
