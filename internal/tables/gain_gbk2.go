package tables

// GainGBK2 is the second-stage codebook of the conjugate-structure
// gain VQ per ITU-T G.729 §3.9 eq. (73)-(74). 16 entries, each a
// pair (g_p component Q14, γ̂ component Q13).
//
// Per §3.9.3 the transmitted index GB is *mapped* — the decoder
// must apply GainImap2[GB] before indexing this table.
//
// Numerical values transcribed from the ITU reference distribution's
// tab_ld8a.c data-array initializer under the merger-doctrine
// exception. No algorithmic source code consulted.
var GainGBK2 = [16][2]int16{
	{826, 2005},
	{1994, 0},
	{5142, 592},
	{6160, 2395},
	{8091, 4861},
	{9120, 525},
	{10573, 2966},
	{11569, 1196},
	{13260, 3256},
	{14194, 1630},
	{15132, 4914},
	{15161, 14276},
	{15434, 237},
	{16112, 3392},
	{17299, 1861},
	{18973, 5935},
}

// GainMap2 maps a physical GBK2 entry index to the transmitted GB
// bit pattern (§3.9.3 — error-spread reduction).
var GainMap2 = [16]uint8{4, 6, 0, 2, 12, 14, 8, 10, 15, 11, 9, 13, 7, 3, 1, 5}

// GainImap2 inverts GainMap2: decoder uses
// `entry = GainGBK2[GainImap2[GB]]`.
var GainImap2 = [16]uint8{2, 14, 3, 13, 0, 15, 1, 12, 6, 10, 7, 9, 4, 11, 5, 8}
