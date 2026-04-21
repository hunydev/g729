package tables

// GainGBK1 is the first-stage codebook of the conjugate-structure
// gain VQ per ITU-T G.729 §3.9 eq. (73)-(74). 8 entries, each a
// pair (g_p component, γ̂ component).
//
// Q-format:
//
//	[i][0] — adaptive-codebook gain g_p component, Q14
//	[i][1] — fixed-codebook gain correction γ̂ component, Q13
//
// (The Q14/Q13 split follows the ITU reference distribution's data
// tables; the spec text fixes only the algorithm, not the chosen
// fixed-point format.)
//
// Per §3.9.3 the transmitted index GA is *mapped* — the decoder
// must apply GainImap1[GA] before indexing this table.
//
// Numerical values transcribed from the ITU reference distribution's
// tab_ld8a.c data-array initializer under the merger-doctrine
// exception (see repository scratch-from-spec policy). No
// algorithmic source code from any G.729 implementation was
// consulted; only the bit-exact data-table content required by the
// standard for interoperability.
var GainGBK1 = [8][2]int16{
	{1, 1516},
	{1551, 2425},
	{1831, 5022},
	{57, 5404},
	{1921, 9291},
	{3242, 9949},
	{356, 14756},
	{2678, 27162},
}

// GainMap1 maps a physical GBK1 entry index to the bit-pattern that
// the encoder transmits for that entry (per §3.9.3 — error-spread
// reduction). Encoder-side: `transmitted_GA = GainMap1[best_idx]`.
var GainMap1 = [8]uint8{5, 1, 4, 7, 3, 0, 6, 2}

// GainImap1 is the inverse of GainMap1: given the transmitted GA
// bit pattern, returns the physical GBK1 entry index. Decoder-side:
// `entry = GainGBK1[GainImap1[GA]]`.
var GainImap1 = [8]uint8{5, 1, 7, 4, 2, 0, 6, 3}
