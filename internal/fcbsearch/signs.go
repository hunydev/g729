package fcbsearch

// SignsFromD performs the sign decomposition of ITU-T G.729 §3.8.1
// (G729E.txt lines 1296–1300):
//
//	"the signal d(n) is decomposed into two parts: its absolute value
//	 |d(n)| and its sign sign[d(n)]"
//
// d(n) is the backward-filtered correlation (CB-1, eq. 52, Q12). The
// signs are extracted *once* and applied during the depth-first focused
// ACELP search (CB-2 φ′ matrix, eq. 56–57); CB-4 reapplies them when
// the final fixed-codebook excitation c(n) is built (eq. 58).
//
// Sign convention: signs[n] ∈ {−1, +1}. The spec is silent on the
// d(n) == 0 tie; the Phase 2d sub-plan pins this to +1 (logged as
// OQ-A38-SIGNTIE in §9; line 458 of the plan).
//
// I3 / I4: pure (writes only through signs and dAbs), zero allocation.
func SignsFromD(d *[SubframeLen]int32, signs *[SubframeLen]int16, dAbs *[SubframeLen]int32) {
	for n := 0; n < SubframeLen; n++ {
		v := d[n]
		if v >= 0 {
			signs[n] = +1
			dAbs[n] = v
		} else {
			signs[n] = -1
			dAbs[n] = -v
		}
	}
}
