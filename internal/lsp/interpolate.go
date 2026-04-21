package lsp

// interpolateLSP produces the per-subframe LSP vectors for one frame
// per ITU-T G.729 §4.1.2:
//
//	sf1[i] = (prev[i] + curr[i]) / 2
//	sf2[i] = curr[i]
//
// The midpoint is computed in 32-bit before the shift so the average
// of two near-full-scale Q15 operands does not saturate prematurely.
// The result is always within Word16 range, so no explicit saturation
// is needed after the shift.
func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
	for i := 0; i < 10; i++ {
		sf1[i] = int16((int32(prev[i]) + int32(curr[i])) >> 1)
		sf2[i] = curr[i]
	}
}
