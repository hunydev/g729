package openloop

// Search returns the open-loop pitch lag T_op ∈ [20, 143] for the
// 80-sample current frame of low-pass weighted speech sw, given the
// 223-sample composite wsp buffer assembled by §A.3.3 (143-sample
// history followed by the 80-sample fresh sw).
//
// The implementation realises the full §A.3.4 (G729E.txt lines
// 2089-2114) decimated three-range search:
//
//  1. Per §A.3.4 lines 2094-2097 + 2113-2114 the wsp buffer is
//     scanned over the three delay regions [80,143], [40,79], [20,39]
//     by pickBestInRange, which evaluates eq. A.5 R(k)²/E(k) at the
//     stride dictated by each region (full stride for the two short
//     regions, even-only + ±1 refinement for [80,143]).
//  2. The three (lag, R, E) triples are merged by mergeThreeRanges
//     per §A.3.4 lines 2109-2111, which biases the winner toward the
//     lower delay range via the OQ-1 sub-multiple lift.
//
// I3 / I4: pure (reads only wsp) and zero-allocation on every path.
func Search(wsp *[223]int16) int16 {
	r3 := pickBestInRangeWide(wsp, 80, 143)
	r2 := pickBestInRangeWide(wsp, 40, 79)
	r1 := pickBestInRangeWide(wsp, 20, 39)
	return mergeThreeRangesWide(r1, r2, r3)
}
