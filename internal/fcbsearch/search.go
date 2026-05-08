package fcbsearch

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

// SearchDepthFirst performs the focused ACELP pulse-position search of
// ITU-T G.729 Annex A §A.3.8.1 (G729E.txt lines 2185–2188):
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
// OQ-A38-DEPTH disposition (Phase 2d sub-plan §9 line 457). §A.3.8.1
// gives no explicit candidate count, depth ordering, threshold, or
// budget. The pinned default per the sub-plan is: constant 8 × 8 × 8 ×
// 16 = 8192 iterations (the full A-priori), depth order T0 → T1 → T2 →
// T3, lower-position-first tie-break, and no early-exit branch. This
// is the "Annex A fixed-complexity" interpretation: the depth-first
// nesting amortizes the partial sums (C, E partials carried across
// loop levels) but does not prune the search space. INT-1a slot 1/5 is
// reserved to revisit ordering / tie-break / pruning if the FCB byte-EQ
// rate underperforms the Phase 2c P1 baseline.
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

// ratioGreater reports whether c1²/e1 > c2²/e2 for non-negative C
// values and positive E values. Use float64 only for the ordering step:
// the previous int64 C*C path could overflow before the ratio comparison,
// corrupting the pulse search on large-correlation inputs.
func ratioGreater(c1, e1, c2, e2 int64) bool {
	return (float64(c1)*float64(c1))/float64(e1) >
		(float64(c2)*float64(c2))/float64(e2)
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
