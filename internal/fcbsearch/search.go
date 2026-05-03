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
// C² is int64 Q24 and E is int64 Q24 — the same scale, so the ratio
// C²/E is dimensionless (Q0). Cross-product comparison C₁²·E₂ vs
// C₂²·E₁ uses 128-bit unsigned multiplication via math/bits.Mul64 to
// avoid the int64 overflow that would otherwise occur for worst-case
// dAbs/φ′ magnitudes (~2⁶² · 2³³ = 2⁹⁵).
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
//   - sumOut[0] is the best C² in Q24 (int64)
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
	var bestC2, bestE int64
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
					C2 := C * C
					if !found {
						found = true
						bestC2, bestE = C2, E
						bestPos = [4]int8{m0, m1, m2, m3}
						continue
					}
					if ratioGreater(C2, E, bestC2, bestE) {
						bestC2, bestE = C2, E
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}

	*positions = bestPos
	sumOut[0] = bestC2
	sumOut[1] = bestE
}

// ratioGreater reports whether a/b > c/d for non-negative a, c and
// positive b, d, using a 128-bit cross-product to stay overflow-safe.
func ratioGreater(a, b, c, d int64) bool {
	hi1, lo1 := bits.Mul64(uint64(a), uint64(d))
	hi2, lo2 := bits.Mul64(uint64(c), uint64(b))
	if hi1 != hi2 {
		return hi1 > hi2
	}
	return lo1 > lo2
}
