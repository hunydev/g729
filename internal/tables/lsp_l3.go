package tables

// LSPCodebookL3 is the 5-bit second-stage upper-split LSF quantizer
// codebook from ITU-T G.729 §3.2.4. Each of the 32 rows is a
// 5-element LSF residual vector covering the upper half of the
// 10-LSF vector (LSFs 6..10), in Q13 Word16.
//
// The numerical values that satisfy the G.729 standard are part of
// the standard itself (single allowed values for bitstream
// interoperability) and are reproduced here under the merger
// doctrine. No algorithmic source code from any G.729
// implementation was consulted.
var LSPCodebookL3 = [32][5]int16{
	{   582,  -1201,    829,     86,    385},
	{  1450,     72,   -231,    864,    661},
	{  -163,   -526,   -754,  -1633,    267},
	{   573,    796,   -169,   -631,    816},
	{   519,    291,    159,   -640,  -1296},
	{  1549,    715,    527,   -714,   -193},
	{  -457,    612,   -283,  -1381,   -741},
	{  -344,   1341,   1087,   -654,   -569},
	{  -543,  -1752,   -195,    -98,   -276},
	{  -235,   -728,    949,   1517,    895},
	{   502,   -362,   -960,   -483,   1386},
	{   450,   -466,   -108,   1010,   2223},
	{   -28,   -378,    744,  -1005,    240},
	{   271,    -15,    909,   -259,   1688},
	{ -1011,    581,    -53,   -747,    878},
	{  -498,  -1377,     18,   -444,   1483},
	{  1015,   -222,    443,    372,   -354},
	{   669,    659,   1640,    932,    534},
	{  1385,   -182,   -907,   -721,   -262},
	{   569,   1247,    337,    416,   -121},
	{   369,  -1003,   -507,   -587,   -904},
	{    72,   -141,   1465,     63,   -785},
	{   208,    301,   -882,    117,   -404},
	{  -912,    623,    -76,    276,   -440},
	{  -267,   -525,    140,    882,   -139},
	{  -697,    865,   1060,    413,    446},
	{   581,  -1037,   -895,    669,    297},
	{     3,    692,   -292,   1050,    782},
	{ -1061,   -484,    362,   -597,   -852},
	{ -1182,   -744,   1340,    262,     63},
	{  -774,   -483,  -1247,    -70,     98},
	{ -1125,   -265,   -242,    724,    934},
}
