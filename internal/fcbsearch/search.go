package fcbsearch

import "math/bits"

// ACELP track positions per ITU-T G.729 §3.8 Table 7 (G729E.txt around
// line 1313): four interleaved tracks of 8 positions plus a 16-position
// fourth track (jx selector). Track-3 ordering is sorted ascending so
// that the tie-break "lower position index wins" pin (OQ-A38-DEPTH per
// Phase 2d sub-plan §9 line 457) is honored when iterating.
var (
	track0 = [8]int8{0, 5, 10, 15, 20, 25, 30, 35}
	track1 = [8]int8{1, 6, 11, 16, 21, 26, 31, 36}
	track2 = [8]int8{2, 7, 12, 17, 22, 27, 32, 37}
	track3 = [16]int8{3, 4, 8, 9, 13, 14, 18, 19, 23, 24, 28, 29, 33, 34, 38, 39}
)

const SearchTopKMax = 32

// SearchThresholdScanDefaultLimit is the G.729 §3.8.1 frame-level cap on
// fourth-loop entries across the two subframes: K3=0.4 thresholding, with at
// most 180 fourth-loop entries over the two subframes.
const SearchThresholdScanDefaultLimit = 180

// SearchDepthFirst performs an exhaustive ACELP pulse-position search using
// the same depth-first loop ordering as ITU-T G.729 Annex A §A.3.8.1
// (G729E.txt lines 2185–2188):
//
//	"Instead of the nested-loop search approach, an iterative
//	 depth-first, tree search approach is used. In this new approach
//	 a smaller number of pulse position combinations is tested and it
//	 has fixed complexity."
//
// The criterion (§3.8 eq. 53–58, lines 1265–1290) is to maximize
//
//	T = C² / E
//
// where, after the sign decomposition of §3.8.1 eq. 56–58:
//
//	C   = |d(m0)| + |d(m1)| + |d(m2)| + |d(m3)|              (eq. 58)
//	E/2 = Σ φ′(mᵢ,mᵢ) + Σ_{i<j} φ′(mᵢ,mⱼ)                    (eq. 59)
//
// with the diagonal of φ′ pre-scaled by 0.5 per eq. 57 (PhiPrime above)
// so the right-hand side of eq. 59 is computed as a plain sum.
//
// OQ-A38-DEPTH disposition (Phase 2d sub-plan §9 line 457). This exhaustive
// helper tests the full 8 × 8 × 8 × 16 candidate space with deterministic
// T0 → T1 → T2 → T3 ordering and lower-position-first tie-breaks. Production
// Core uses the threshold-scan helper below; this exhaustive helper remains
// the deterministic fallback for degenerate threshold inputs and a
// PDF-visible C²/E audit baseline.
//
// Q-format. dAbs is int32 Q12 (CB-3 SignsFromD output); φ′ is int32
// Q24 (PhiPrime output). C is int64 Q12 (sum of four Q12 values), so
// C²/E is dimensionless (Q0). The selected C is kept unsquared during
// search because valid large-correlation inputs can exceed int64 before
// any wider comparison helper gets a chance to run.
//
// Tie-break. Iteration order is T0 → T1 → T2 → T3, each track in
// ascending position order. Updates fire only on strict improvement
// (mul128Cmp == 1), so the first candidate visited at any given ratio
// wins — i.e. lower position indices win ties.
//
// Degenerate cases. If E ≤ 0 for the candidate under test (impossible
// for a real impulse response h since Φ = HᵀH is positive semi-
// definite, but defensible against caller error / OQ-A38-SIGNTIE
// pathological inputs), that candidate is skipped — never overwriting
// a well-defined best. If every candidate has E ≤ 0 (e.g. phi is
// all-zero, the §A.3.8.1 plan-§6.1 "impossible h" guard), the result
// is the lower-position default {0, 1, 2, 3}.
//
// I3 / I4: pure (writes only through positions and sumOut), zero
// allocation. The 6.4 KB φ′ matrix is caller-owned (see PhiPrime).
//
// On return:
//   - positions[i] is the absolute pulse position mᵢ ∈ [0, 40) on track i
//   - sumOut[0] is the best C² in Q24 when representable, otherwise
//     MaxInt64 as a diagnostic saturation marker
//   - sumOut[1] is the best E in Q24 (int64), with the eq. 57 0.5
//     factor already absorbed in φ′ — i.e. this is the eq. 59 "E/2".
func SearchDepthFirst(
	dAbs *[SubframeLen]int32,
	phi *[SubframeLen][SubframeLen]int32,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	// Default fallback per §A.3.8.1 zero-Φ guard.
	bestPos := [4]int8{0, 1, 2, 3}
	var bestC, bestE int64
	found := false

	for _, m0 := range track0 {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1 {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2 {
				d012 := d01 + int64(dAbs[m2])
				e012 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3 {
					C := d012 + int64(dAbs[m3])
					E := e012 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if E <= 0 {
						continue
					}
					if !found {
						found = true
						bestC, bestE = C, E
						bestPos = [4]int8{m0, m1, m2, m3}
						continue
					}
					if ratioGreater(C, E, bestC, bestE) {
						bestC, bestE = C, E
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}

	*positions = bestPos
	sumOut[0] = squareSaturatingInt64(bestC)
	sumOut[1] = bestE
}

// SearchDepthFirstThresholdScan is the G.729 §3.8.1 focused fixed-codebook
// search. It computes the K3=0.4 first-three-pulse correlation threshold,
// then evaluates at most limit first-three prefixes that exceed that threshold,
// scanning all legal fourth-track positions for each accepted prefix.
//
// This deliberately remains scalar and zero-allocation. The threshold is based
// only on dAbs, so it is derived from the local search surface rather than an
// external implementation oracle.
func SearchDepthFirstThresholdScan(
	dAbs *[SubframeLen]int32,
	phi *[SubframeLen][SubframeLen]int32,
	positions *[4]int8,
	sumOut *[2]int64,
	limit int,
) {
	SearchDepthFirstThresholdScanEntered(dAbs, phi, positions, sumOut, limit)
}

// SearchDepthFirstThresholdScanEntered is SearchDepthFirstThresholdScan plus
// the number of accepted first-three-pulse prefixes whose fourth-track loop
// was entered. It lets the encoder audit frame-level Annex A budgets without
// duplicating the focused-search implementation.
func SearchDepthFirstThresholdScanEntered(
	dAbs *[SubframeLen]int32,
	phi *[SubframeLen][SubframeLen]int32,
	positions *[4]int8,
	sumOut *[2]int64,
	limit int,
) int {
	if limit <= 0 {
		SearchDepthFirst(dAbs, phi, positions, sumOut)
		return 0
	}

	var sumC, maxC int64
	var first3Count int64
	for _, m0 := range track0 {
		d0 := int64(dAbs[m0])
		for _, m1 := range track1 {
			d01 := d0 + int64(dAbs[m1])
			for _, m2 := range track2 {
				c := d01 + int64(dAbs[m2])
				sumC += c
				first3Count++
				if c > maxC {
					maxC = c
				}
			}
		}
	}
	if first3Count == 0 {
		SearchDepthFirst(dAbs, phi, positions, sumOut)
		return 0
	}
	avgC := sumC / first3Count
	threshold := avgC + (4*(maxC-avgC))/10

	bestPos := [4]int8{0, 1, 2, 3}
	var bestC, bestE int64
	found := false
	entered := 0
scan:
	for _, m0 := range track0 {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1 {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2 {
				c3 := d01 + int64(dAbs[m2])
				if c3 <= threshold {
					continue
				}
				if entered >= limit {
					break scan
				}
				entered++
				e3 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3 {
					C := c3 + int64(dAbs[m3])
					E := e3 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if E <= 0 {
						continue
					}
					if !found {
						found = true
						bestC, bestE = C, E
						bestPos = [4]int8{m0, m1, m2, m3}
						continue
					}
					if ratioGreater(C, E, bestC, bestE) {
						bestC, bestE = C, E
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}
	if !found {
		SearchDepthFirst(dAbs, phi, positions, sumOut)
		return entered
	}

	*positions = bestPos
	sumOut[0] = squareSaturatingInt64(bestC)
	sumOut[1] = bestE
	return entered
}

// SearchTopK returns the top fixed-codebook position candidates ranked by
// the same C²/E criterion and lower-position tie-break as SearchDepthFirst.
//
// The caller supplies `positions` scratch; at most SearchTopKMax entries are
// written. The returned count is always at least 1. This helper lets the
// encoder cheaply rerank a small candidate set after quantized gain search
// without allocating.
func SearchTopK(
	dAbs *[SubframeLen]int32,
	phi *[SubframeLen][SubframeLen]int32,
	positions *[SearchTopKMax][4]int8,
	limit int,
) int {
	if limit < 1 {
		limit = 1
	}
	if limit > SearchTopKMax {
		limit = SearchTopKMax
	}

	var bestC, bestE [SearchTopKMax]int64
	count := 0
	for _, m0 := range track0 {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1 {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2 {
				d012 := d01 + int64(dAbs[m2])
				e012 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3 {
					C := d012 + int64(dAbs[m3])
					E := e012 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if E <= 0 {
						continue
					}
					if count == limit && !ratioGreater(C, E, bestC[count-1], bestE[count-1]) {
						continue
					}

					pos := count
					for pos > 0 && ratioGreater(C, E, bestC[pos-1], bestE[pos-1]) {
						pos--
					}
					if count < limit {
						count++
					}
					for i := count - 1; i > pos; i-- {
						positions[i] = positions[i-1]
						bestC[i] = bestC[i-1]
						bestE[i] = bestE[i-1]
					}
					positions[pos] = [4]int8{m0, m1, m2, m3}
					bestC[pos] = C
					bestE[pos] = E
				}
			}
		}
	}

	if count == 0 {
		positions[0] = [4]int8{0, 1, 2, 3}
		return 1
	}
	return count
}

// ratioGreater reports whether c1²/e1 > c2²/e2 for non-negative C
// values and positive E values. Compare by exact integer cross-product:
//
//	c1²/e1 > c2²/e2  <=>  c1²·e2 > c2²·e1
//
// C is the sum of four int32 correlations and E is the sum of ten int32
// phi terms, so the valid search-domain cross-products fit comfortably
// in uint128. The saturating multiply keeps malformed caller inputs from
// wrapping while preserving the exact Annex A ordering in the production
// domain.
func ratioGreater(c1, e1, c2, e2 int64) bool {
	if c1 < 0 {
		c1 = -c1
	}
	if c2 < 0 {
		c2 = -c2
	}
	left := mul128By64Saturating(mul64(uint64(c1), uint64(c1)), uint64(e2))
	right := mul128By64Saturating(mul64(uint64(c2), uint64(c2)), uint64(e1))
	return cmp128(left, right) > 0
}

type uint128 struct {
	hi uint64
	lo uint64
}

func mul64(a, b uint64) uint128 {
	hi, lo := bits.Mul64(a, b)
	return uint128{hi: hi, lo: lo}
}

func mul128By64Saturating(x uint128, y uint64) uint128 {
	hiLo, lo := bits.Mul64(x.lo, y)
	hiHi, loHi := bits.Mul64(x.hi, y)
	if hiHi != 0 {
		return uint128{hi: ^uint64(0), lo: ^uint64(0)}
	}
	hi, carry := bits.Add64(hiLo, loHi, 0)
	if carry != 0 {
		return uint128{hi: ^uint64(0), lo: ^uint64(0)}
	}
	return uint128{hi: hi, lo: lo}
}

func cmp128(a, b uint128) int {
	if a.hi > b.hi {
		return 1
	}
	if a.hi < b.hi {
		return -1
	}
	if a.lo > b.lo {
		return 1
	}
	if a.lo < b.lo {
		return -1
	}
	return 0
}

func squareSaturatingInt64(v int64) int64 {
	if v < 0 {
		v = -v
	}
	const maxSqrtInt64 int64 = 3037000499
	if v > maxSqrtInt64 {
		return 1<<63 - 1
	}
	return v * v
}
