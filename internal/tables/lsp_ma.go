package tables

// MAPredictorsLSP holds the two order-4 MA predictor coefficient
// sets used for inter-frame LSF prediction, from ITU-T G.729 §3.2.4.
//
// Indexed as MAPredictorsLSP[selector][tap][lsfIndex]:
//   - selector in {0, 1} chosen by the L0 bit.
//   - tap in {0, 1, 2, 3} selects how far back in residual history
//     (0 = previous residual, 3 = 4th previous).
//   - lsfIndex in {0..9} indexes the LSF dimension.
//
// Values are Q15 Word16 (magnitudes < 1).
//
// The numerical values that satisfy the G.729 standard are part of
// the standard itself (single allowed values for bitstream
// interoperability) and are reproduced here under the merger
// doctrine. No algorithmic source code from any G.729
// implementation was consulted.
var MAPredictorsLSP = [2][4][10]int16{
	{
		{8421, 9109, 9175, 8965, 9034, 9057, 8765, 8775, 9106, 8673},
		{7018, 7189, 7638, 7307, 7444, 7379, 7038, 6956, 6930, 6868},
		{5472, 4990, 5134, 5177, 5246, 5141, 5206, 5095, 4830, 5147},
		{4056, 3031, 2614, 3024, 2916, 2713, 3309, 3237, 2857, 3473},
	},
	{
		{7733, 7880, 8188, 8175, 8247, 8490, 8637, 8601, 8359, 7569},
		{4210, 3031, 2552, 3473, 3876, 3853, 4184, 4154, 3909, 3968},
		{3214, 1930, 1313, 2143, 2493, 2385, 2755, 2706, 2542, 2919},
		{3024, 1592, 940, 1631, 1723, 1579, 2034, 2084, 1913, 2601},
	},
}

// MAPredictorInvSumLSP holds the Q15 complement term used with
// MAPredictorsLSP in the LSF MA reconstruction. It is not recomputed
// as 32767 - Σp at runtime because the fixed-point table values are
// rounded independently; the small per-coordinate differences are
// observable in the decoder_tame_lsp_pipeline numeric oracle.
var MAPredictorInvSumLSP = [2][10]int16{
	{7800, 8447, 8205, 8293, 8126, 8477, 8447, 8703, 9043, 8604},
	{14585, 18333, 19772, 17344, 16426, 16459, 15155, 15220, 16043, 15708},
}
