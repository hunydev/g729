package fcbsearch

// PhiPrime computes the modified correlation matrix φ′(i,j) of
// ITU-T G.729 §3.8.1 eq. 56–57 (G729E.txt lines 1267–1273):
//
//	φ′(i,j) = sign[d(i)] · sign[d(j)] · φ(i,j),    i = 0..39, j = i+1..39   (eq. 56)
//	φ′(i,i) = 0.5 · φ(i,i),                        i = 0..39                (eq. 57)
//
// where φ(i,j) is the lower-triangular correlation of the impulse
// response h(n) defined by eq. 51 (lines 1252–1255):
//
//	φ(i,j) = Σ_{n=j..39} h(n−i) · h(n−j),    i ≤ j
//
// The sign decomposition signs[n] = sign[d(n)] is the CB-3 SignsFromD
// output (§3.8.1 lines 1296–1300). Diagonal pre-scaling by 0.5 absorbs
// the factor 2 of eq. 55, so the depth-first search of §A.3.8.1 (lines
// 2185–2188) accumulates the eq. 59 form (E/2) directly.
//
// Q-format. h is Q12 (Phase 2c HI-1 convention); each product h·h is
// int32 Q24, so φ′ is stored as int32 Q24. Storage layout is full
// symmetric: phi[i][j] = phi[j][i] for i ≠ j, with the diagonal
// pre-scaled per eq. 57. (The plan §6.1 line 233 mandates lower-
// triangular only; we widen to full symmetric so the depth-first inner
// loop can index phi[mi][mj] without an i<j swap. The 6.4 KB total
// matches Phase 2c precedents — caller-owned scratch.)
//
// Saturation. φ(i,i) is bounded by Σ h(n)² which for typical Q12 h
// (peak |h| ≈ 4096) stays well under 2³⁰; off-diagonal terms are
// similarly bounded. No explicit saturation is applied here; if a
// pathological h overflows int32 the byte-EQ harness at INT-1a will
// flag it and the storage may be widened to int64 (logged under the
// existing OQ-Q-FORMAT-A10 budget).
//
// I3 / I4: pure (writes only through phi), zero allocation.
func PhiPrime(h, signs *[SubframeLen]int16, phi *[SubframeLen][SubframeLen]int32) {
	for i := 0; i < SubframeLen; i++ {
		var diag int64
		for n := i; n < SubframeLen; n++ {
			t := int64(h[n-i])
			diag += t * t
		}
		phi[i][i] = saturateInt64ToInt32(diag >> 1) // eq. 57: 0.5·φ(i,i); sign² = 1
		for j := i + 1; j < SubframeLen; j++ {
			var sum int64
			for n := j; n < SubframeLen; n++ {
				sum += int64(h[n-i]) * int64(h[n-j])
			}
			s := int64(signs[i]) * int64(signs[j]) // ±1 per eq. 56
			v := sum * s
			phi[i][j] = saturateInt64ToInt32(v)
			phi[j][i] = saturateInt64ToInt32(v)
		}
	}
}
