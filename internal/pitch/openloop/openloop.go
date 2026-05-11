package openloop

import "github.com/hunydev/g729/internal/fixed"

// RangeScore holds one §A.3.4 open-loop pitch range winner.
type RangeScore struct {
	Lag int16
	R   fixed.Word32
	E   fixed.Word32
}

// SearchResult contains the three range winners and the merged T_op.
type SearchResult struct {
	Range1 RangeScore
	Range2 RangeScore
	Range3 RangeScore
	Top    int16
}

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
//     by pickBestInRange, which first retains the eq. A.4 correlation
//     maximum in each region (full stride for the two short regions,
//     even-only + ±1 refinement for [80,143]).
//  2. The three (lag, R, E) triples are merged by mergeThreeRanges
//     per §A.3.4 lines 2109-2111, which biases the winner toward the
//     lower delay range via the OQ-1 sub-multiple lift.
//
// I3 / I4: pure (reads only wsp) and zero-allocation on every path.
func Search(wsp *[223]int16) int16 {
	return SearchWithRanges(wsp).Top
}

// SearchWithRanges is Search plus the per-range winners. It exists for
// encoder-side quality heuristics that need to select among already-standard
// open-loop candidates without changing the core §A.3.4 search primitive.
func SearchWithRanges(wsp *[223]int16) SearchResult {
	lag3, r3, e3 := pickBestInRange(wsp, 80, 143)
	lag2, r2, e2 := pickBestInRange(wsp, 40, 79)
	lag1, r1, e1 := pickBestInRange(wsp, 20, 39)
	return SearchResult{
		Range1: RangeScore{Lag: lag1, R: r1, E: e1},
		Range2: RangeScore{Lag: lag2, R: r2, E: e2},
		Range3: RangeScore{Lag: lag3, R: r3, E: e3},
		Top:    mergeThreeRanges(r1, e1, lag1, r2, e2, lag2, r3, e3, lag3),
	}
}

// SearchWithRangesNormalized is a non-Annex-A quality heuristic that chooses
// each per-range candidate by normalized R(k)²/E(k), then keeps its
// product-tuned pairwise inter-range merge. It is kept separate so Core can
// follow the §A.3.4 raw-correlation "retained maxima" wording and global lower
// sub-multiple checks.
func SearchWithRangesNormalized(wsp *[223]int16) SearchResult {
	lag3, r3, e3 := pickBestInRangeNormalized(wsp, 80, 143)
	lag2, r2, e2 := pickBestInRangeNormalized(wsp, 40, 79)
	lag1, r1, e1 := pickBestInRangeNormalized(wsp, 20, 39)
	return SearchResult{
		Range1: RangeScore{Lag: lag1, R: r1, E: e1},
		Range2: RangeScore{Lag: lag2, R: r2, E: e2},
		Range3: RangeScore{Lag: lag3, R: r3, E: e3},
		Top:    mergeThreeRangesPairwise(r1, e1, lag1, r2, e2, lag2, r3, e3, lag3),
	}
}
