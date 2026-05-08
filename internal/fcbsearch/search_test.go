package fcbsearch_test

import (
	"testing"

	"github.com/hunydev/g729/internal/fcbsearch"
)

// CB-2 RED tests for §A.3.8.1 (G729E.txt lines 2185–2188) — depth-first
// focused ACELP search. The pulse-track structure (§3.8 Table 7) is:
//   T0: {0, 5, 10, 15, 20, 25, 30, 35}    (8 positions, 3 bits)
//   T1: {1, 6, 11, 16, 21, 26, 31, 36}    (8 positions, 3 bits)
//   T2: {2, 7, 12, 17, 22, 27, 32, 37}    (8 positions, 3 bits)
//   T3: {3, 8, 13, 18, 23, 28, 33, 38}
//      ∪ {4, 9, 14, 19, 24, 29, 34, 39}   (16 positions, 4 bits incl. jx)
//
// Search criterion (§3.8 eq. 53–58, lines 1265–1290):
//   maximize T = C²/E where
//     C = |d(m0)| + |d(m1)| + |d(m2)| + |d(m3)|     (eq. 58)
//     E/2 = Σ φ′(mi,mi) + Σ_{i<j} φ′(mi,mj)         (eq. 59)
//
// Tie-break (OQ-A38-DEPTH default): lower position index wins.

func TestSearchDepthFirst_DeltaSpike(t *testing.T) {
	// Spike |d| at positions {0, 1, 2, 3} (one per track, all in the first
	// slot of their track). Phi diagonal = small constant, off-diagonal = 0.
	// The maximizer must trivially pick {0,1,2,3}.
	var dAbs [40]int32
	dAbs[0] = 1 << 20
	dAbs[1] = 1 << 20
	dAbs[2] = 1 << 20
	dAbs[3] = 1 << 20

	var phi [40][40]int32
	for i := 0; i < 40; i++ {
		phi[i][i] = 1 << 18 // small positive denominator
	}

	var positions [4]int8
	var sums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sums)

	want := [4]int8{0, 1, 2, 3}
	if positions != want {
		t.Fatalf("positions=%v want %v", positions, want)
	}
}

func TestSearchDepthFirst_Track3HighSlot(t *testing.T) {
	// Force track-3 winner into the second-half (jx=1) range: position 4.
	// Tracks 0,1,2 each have one dominant slot; track 3 has dAbs[4] as the
	// only nonzero, so the search must select position 4.
	var dAbs [40]int32
	dAbs[5] = 1 << 22 // T0 winner
	dAbs[6] = 1 << 22 // T1 winner
	dAbs[7] = 1 << 22 // T2 winner
	dAbs[4] = 1 << 22 // T3 winner — only nonzero in T3

	var phi [40][40]int32
	for i := 0; i < 40; i++ {
		phi[i][i] = 1 << 18
	}

	var positions [4]int8
	var sums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sums)

	want := [4]int8{5, 6, 7, 4}
	if positions != want {
		t.Fatalf("positions=%v want %v (track3 jx=1 selector)", positions, want)
	}
}

func TestSearchDepthFirst_TieBreakLowerPosition(t *testing.T) {
	// Uniform dAbs across all 40 positions and uniform phi diagonal with
	// zero off-diagonal: every (m0,m1,m2,m3) combination yields the same
	// C²/E. The tie-break rule (lower position index wins) must select
	// the first slot of each track: {0, 1, 2, 3}.
	var dAbs [40]int32
	for n := range dAbs {
		dAbs[n] = 12345
	}
	var phi [40][40]int32
	for i := 0; i < 40; i++ {
		phi[i][i] = 1 << 16
	}

	var positions [4]int8
	var sums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sums)

	want := [4]int8{0, 1, 2, 3}
	if positions != want {
		t.Fatalf("positions=%v want %v (tie-break lower-position-first)", positions, want)
	}
}

func TestSearchDepthFirst_ExhaustiveCrossCheck(t *testing.T) {
	// Cross-check the depth-first implementation against an independent
	// brute-force exhaustive scan over all 8 × 8 × 8 × 16 = 8192 pulse
	// position combinations (this *is* the same combinatorial space, but
	// computed by an in-test reference implementation that also pulls the
	// tie-break by reverse update — i.e. only strict-greater wins, lower
	// index iterated first). dAbs and phi are seeded by a deterministic
	// PRNG so the test is reproducible.
	var dAbs [40]int32
	var phi [40][40]int32
	seed := uint64(0xC0FFEE_42)
	rng := func() uint32 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return uint32(seed >> 32)
	}
	for n := 0; n < 40; n++ {
		dAbs[n] = int32(rng()&0x000F_FFFF) + 1 // small non-negative Q12
	}
	for i := 0; i < 40; i++ {
		// Diagonal: positive (energy-like, in Q24 with 0.5 factor).
		phi[i][i] = int32(rng()&0x0007_FFFF) + (1 << 12)
		for j := i + 1; j < 40; j++ {
			// Small signed off-diagonal.
			v := int32(rng()&0x0001_FFFF) - (1 << 16)
			phi[i][j] = v
			phi[j][i] = v
		}
	}

	var got [4]int8
	var gotSums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &got, &gotSums)

	want, wC2, wE := bruteForceACELP(&dAbs, &phi)
	if got != want {
		t.Fatalf("depth-first positions=%v want %v (brute-force)", got, want)
	}
	if gotSums[0] != wC2 || gotSums[1] != wE {
		t.Fatalf("sums=(C²=%d, E=%d) want (C²=%d, E=%d)",
			gotSums[0], gotSums[1], wC2, wE)
	}
}

func TestSearchDepthFirst_LargeCorrelationDoesNotOverflowC2(t *testing.T) {
	// Large valid correlations can push C above sqrt(MaxInt64). The
	// search must still rank C²/E correctly instead of overflowing
	// C*C before comparison.
	var dAbs [40]int32
	for n := range dAbs {
		dAbs[n] = 1
	}
	dAbs[5] = 1_100_000_000
	dAbs[6] = 1_100_000_000
	dAbs[7] = 1_100_000_000
	dAbs[4] = 1_100_000_000

	var phi [40][40]int32
	for i := 0; i < 40; i++ {
		phi[i][i] = 1 << 18
	}

	var got [4]int8
	var sums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &got, &sums)

	want := [4]int8{5, 6, 7, 4}
	if got != want {
		t.Fatalf("positions=%v want %v for large-C overflow guard", got, want)
	}
	if sums[0] != 1<<63-1 {
		t.Fatalf("C² diagnostic sum=%d want saturated MaxInt64", sums[0])
	}
}

func TestSearchDepthFirst_CB1CB3Trace(t *testing.T) {
	// End-to-end trace through CB-1 + CB-3 + CB-2: feed a hand-chosen x, y,
	// gp, h through AdjustedTarget → CorrelationD → SignsFromD →
	// PhiPrime → SearchDepthFirst, then verify the depth-first search
	// agrees with the in-test brute-force reference.
	var x, y, h [40]int16
	for n := range x {
		x[n] = int16(-1500 + 113*n)
		y[n] = int16(800 - 19*n)
		h[n] = int16(2048 - 37*n)
	}
	const gp = int16(8000) // Q14 ≈ 0.488

	var xPrime [40]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)

	var d [40]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [40][40]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	var got [4]int8
	var gotSums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &got, &gotSums)

	want, wC2, wE := bruteForceACELP(&dAbs, &phi)
	if got != want {
		t.Fatalf("CB-1→CB-3→CB-2 trace: positions=%v want %v", got, want)
	}
	if gotSums[0] != wC2 || gotSums[1] != wE {
		t.Fatalf("CB-1→CB-3→CB-2 trace: sums=(C²=%d, E=%d) want (C²=%d, E=%d)",
			gotSums[0], gotSums[1], wC2, wE)
	}
}

func TestSearchDepthFirst_ZeroPhi(t *testing.T) {
	// Pathological case: phi diagonal is zero everywhere (impossible h,
	// but defensible: ensure the search does not divide by zero or panic
	// and falls back to the lower-position default {0,1,2,3}).
	var dAbs [40]int32
	for n := range dAbs {
		dAbs[n] = int32(n + 1)
	}
	var phi [40][40]int32 // all zeros

	var positions [4]int8
	var sums [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sums)

	want := [4]int8{0, 1, 2, 3}
	if positions != want {
		t.Fatalf("positions=%v want %v (zero-phi fallback)", positions, want)
	}
}

func TestSearchDepthFirst_NoAlloc(t *testing.T) {
	var dAbs [40]int32
	for n := range dAbs {
		dAbs[n] = int32(1000 + 17*n)
	}
	var phi [40][40]int32
	for i := 0; i < 40; i++ {
		phi[i][i] = int32(2048 + i)
	}
	var positions [4]int8
	var sums [2]int64
	if got := testing.AllocsPerRun(8, func() {
		fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sums)
	}); got != 0 {
		t.Fatalf("SearchDepthFirst allocations/op = %v, want 0 (caller-owned scratch)", got)
	}
}

// bruteForceACELP is an in-test reference: exhaustively scans all
// 8×8×8×16 = 8192 pulse position combinations, returns the (positions,
// C², E) maximizing T = C²/E with lower-position-first tie-break.
func bruteForceACELP(dAbs *[40]int32, phi *[40][40]int32) ([4]int8, int64, int64) {
	t0 := []int8{0, 5, 10, 15, 20, 25, 30, 35}
	t1 := []int8{1, 6, 11, 16, 21, 26, 31, 36}
	t2 := []int8{2, 7, 12, 17, 22, 27, 32, 37}
	t3 := []int8{3, 4, 8, 9, 13, 14, 18, 19, 23, 24, 28, 29, 33, 34, 38, 39}

	var best [4]int8
	var bestC, bestE int64
	first := true
	for _, m0 := range t0 {
		for _, m1 := range t1 {
			for _, m2 := range t2 {
				for _, m3 := range t3 {
					C := int64(dAbs[m0]) + int64(dAbs[m1]) +
						int64(dAbs[m2]) + int64(dAbs[m3])
					E := int64(phi[m0][m0]) + int64(phi[m1][m1]) +
						int64(phi[m2][m2]) + int64(phi[m3][m3]) +
						int64(phi[m0][m1]) + int64(phi[m0][m2]) + int64(phi[m0][m3]) +
						int64(phi[m1][m2]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if first {
						first = false
						bestC, bestE = C, E
						best = [4]int8{m0, m1, m2, m3}
						continue
					}
					if ratioGreater(C, E, bestC, bestE) {
						bestC, bestE = C, E
						best = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}
	return best, squareSaturatingInt64(bestC), bestE
}

// ratioGreater reports whether a²/b > c²/d treating any non-positive
// denominator as the worst possible ratio (so the well-defined side wins).
func ratioGreater(a, b, c, d int64) bool {
	if d <= 0 && b <= 0 {
		return false
	}
	if d <= 0 {
		return true
	}
	if b <= 0 {
		return false
	}
	return (float64(a)*float64(a))/float64(b) >
		(float64(c)*float64(c))/float64(d)
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
