package gainquant

import (
	"math/bits"

	"github.com/exedev/g729/internal/tables"
)

// SearchConjugate performs the §3.9.2 conjugate-structure two-stage
// (GA 3-bit + GB 4-bit) gain VQ search per ITU-T G.729.
//
// Inputs (all 40-sample subframe vectors, caller-owned):
//
//   - x  : target signal x(n) per §3.6                    (Q0)
//   - y  : filtered adaptive-codebook vector per eq. (44) (Q0)
//   - z  : filtered fixed-codebook vector per eq. (64)    (Q0)
//   - gpcPredQ12 : predicted fixed-codebook gain g'c per eq. (71), Q12,
//     held NON-saturated as int32 (IMPL-3 representation change — see
//     PredictedGcQ12 docstring; the §3.9.2 cost search must evaluate
//     the spec eq. (74) value, not an int16-clipped surrogate).
//
// Outputs:
//
//   - ga : *physical* GBK1 entry index (0..7).  ENC-1's PackGains
//     applies the §3.9.3 forward map (GainMap1) to obtain the
//     transmitted bit pattern.
//   - gb : *physical* GBK2 entry index (0..15); see GainMap2.
//   - gpQ14     : ĝp = GBK1[ga][0] + GBK2[gb][0] per eq. (73), Q14.
//   - gammaCQ13 : γ̂_c = GBK1[ga][1] + GBK2[gb][1] per eq. (74), Q13.
//     The caller (encoder) feeds this into DequantGc together with
//     log2GcPredQ10 to obtain the native (gcMantQ14, gcExp) g_c
//     representation that mirrors gain.Decoder.Decode bit-for-bit.
//
// Algorithm (§3.9.2 lines 1382–1407):
//  1. Compute the inner products A = ⟨y,y⟩, B = ⟨z,z⟩, C = ⟨y,z⟩,
//     D = ⟨x,y⟩, F = ⟨x,z⟩ (all int64, exact for 40 int16² accums).
//  2. Solve the 2×2 system from ∂E/∂gp = ∂E/∂gc = 0 of eq. (63):
//     [A C][gp] = [D]   ⇒  det = A·B − C²,
//     [C B][gc]   [F]      gp_opt = (D·B − F·C) / det,
//     gc_opt = (F·A − D·C) / det.
//     Degenerate cases (det = 0) handled by 1-D fallback.  Optimum
//     pair is the *unquantized* reference for preselect (eq. 63 line
//     1389).
//  3. Pre-select 4-of-8 GA entries on min |γ̂_GA·g'c − gc_opt|
//     (linear Q12; OQ-GA-PRESELECT-METRIC pinned to L1).
//  4. Pre-select 8-of-16 GB entries on min |gp_GB − gp_opt| (Q14).
//  5. Exhaustive 4×8 = 32 cost minimization of eq. (63):
//     J(ĝp,ĝc) = ĝp²·A + ĝc²·B + 2·ĝp·ĝc·C − 2·ĝp·D − 2·ĝc·F.
//
// Implementation notes:
//
//   - All correlations are right-shifted by `nshift` so the largest
//     magnitude fits in 14 bits.  This guarantees the cost-term
//     int64 multiplications never overflow (worst-case ~2^54).
//   - The same `nshift` is reused for the optimum-gain solve (det =
//     A·B − C² ≤ 2^28; numerators ≤ 2^28; numerator<<14 ≤ 2^42).
//   - All 40 inner products are computed in a single pass.
//
// I4 (zero allocation): all scratch buffers are fixed-size local
// arrays.
func SearchConjugate(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	// 1. Correlations (int64, exact: 40 · (2^15)² < 2^36).
	var A, B, C, D, F int64
	for i := 0; i < 40; i++ {
		xi := int64(x[i])
		yi := int64(y[i])
		zi := int64(z[i])
		A += yi * yi
		B += zi * zi
		C += yi * zi
		D += xi * yi
		F += xi * zi
	}

	// 2. Normalize so max |corr| ≤ 2^14 — keeps both the optimum-solve
	// products (≤ 2^28) and the cost-term products (≤ 2^54) inside int64.
	maxAbs := absI64(A)
	if v := absI64(B); v > maxAbs {
		maxAbs = v
	}
	if v := absI64(C); v > maxAbs {
		maxAbs = v
	}
	if v := absI64(D); v > maxAbs {
		maxAbs = v
	}
	if v := absI64(F); v > maxAbs {
		maxAbs = v
	}
	var nshift uint
	if maxAbs > 0 {
		blen := uint(bits.Len64(uint64(maxAbs)))
		if blen > 14 {
			nshift = blen - 14
		}
	}
	if nshift > 0 {
		A = signedRsh(A, nshift)
		B = signedRsh(B, nshift)
		C = signedRsh(C, nshift)
		D = signedRsh(D, nshift)
		F = signedRsh(F, nshift)
	}

	// 3. Optimum (gp_opt Q14, gc_opt Q12) via 2×2 solve.
	var gpOptQ14, gcOptQ12 int64
	det := A*B - C*C // ≥ 0 by Cauchy–Schwarz
	switch {
	case det > 0:
		numGp := D*B - F*C
		numGc := F*A - D*C
		gpOptQ14 = (numGp << 14) / det
		gcOptQ12 = (numGc << 12) / det
	case A > 0:
		// y nondegenerate, z degenerate or y∝z → gc undefined;
		// fall back to the 1-D pitch-only optimum.
		gpOptQ14 = (D << 14) / A
		gcOptQ12 = 0
	case B > 0:
		gpOptQ14 = 0
		gcOptQ12 = (F << 12) / B
	default:
		gpOptQ14 = 0
		gcOptQ12 = 0
	}
	// Codebook entries are non-negative; clamp to ≥ 0 for proximity
	// preselect.  Upper clamp is implicit via codebook envelope.
	if gpOptQ14 < 0 {
		gpOptQ14 = 0
	}
	if gcOptQ12 < 0 {
		gcOptQ12 = 0
	}

	// 4. GA preselect: 4-of-8 minimising |γ̂_GA·g'c − gc_opt|  (Q12).
	//    γ̂_GA·g'c at Q12 = (GainGBK1[i][1] · gpcPredQ12) >> 13.
	var gaIdx [8]uint8
	var gaDist [8]int64
	for i := 0; i < 8; i++ {
		gaIdx[i] = uint8(i)
		cand := (int64(tables.GainGBK1[i][1]) * int64(gpcPredQ12)) >> 13
		d := cand - gcOptQ12
		if d < 0 {
			d = -d
		}
		gaDist[i] = d
	}
	sortByDist8(&gaIdx, &gaDist)
	gaCands := gaIdx[:4]

	// 5. GB preselect: 8-of-16 minimising |gp_GB − gp_opt|  (Q14).
	var gbIdx [16]uint8
	var gbDist [16]int64
	for j := 0; j < 16; j++ {
		gbIdx[j] = uint8(j)
		d := int64(tables.GainGBK2[j][0]) - gpOptQ14
		if d < 0 {
			d = -d
		}
		gbDist[j] = d
	}
	sortByDist16(&gbIdx, &gbDist)
	gbCands := gbIdx[:8]

	// 6. Exhaustive 4×8 cost minimisation of eq. (63).
	//    Common Q for cost: Q28 (after shifts).
	const costInit int64 = 1 << 62
	bestCost := costInit
	var bestGA, bestGB uint8
	var bestGp int32
	var bestGam int32
	for _, gai := range gaCands {
		gp1 := int64(tables.GainGBK1[gai][0])
		gam1 := int64(tables.GainGBK1[gai][1])
		for _, gbi := range gbCands {
			gp2 := int64(tables.GainGBK2[gbi][0])
			gam2 := int64(tables.GainGBK2[gbi][1])
			gpQ := gp1 + gp2                       // Q14
			gam := gam1 + gam2                     // Q13
			gcQ := (gam * int64(gpcPredQ12)) >> 13 // Q12

			cost := gpQ * gpQ * A           // Q28
			cost += (gcQ * gcQ * B) << 4    // Q24<<4 = Q28
			cost += (2 * gpQ * gcQ * C) << 2
			cost -= (2 * gpQ * D) << 14
			cost -= (2 * gcQ * F) << 16

			if cost < bestCost {
				bestCost = cost
				bestGA = gai
				bestGB = gbi
				bestGp = int32(gpQ)
				bestGam = int32(gam)
			}
		}
	}

	gpQ14 = int16(bestGp) // sum ≤ 22215, fits Word16
	gammaCQ13 = int16(bestGam)
	ga = bestGA
	gb = bestGB
	return
}

// absI64 returns the absolute value of x without branching on the
// sign bit (avoids pathological behaviour at math.MinInt64; our
// correlations are bounded well below that).
func absI64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// signedRsh implements arithmetic right-shift on int64 (Go's >> on
// signed integers is already arithmetic, but this wrapper makes the
// intent explicit at the call site).
func signedRsh(x int64, n uint) int64 {
	return x >> n
}

// sortByDist8 sorts (idx, dist) pairs in-place by ascending dist.
// Insertion sort, branch-light, allocation-free for fixed N=8.
func sortByDist8(idx *[8]uint8, dist *[8]int64) {
	for i := 1; i < 8; i++ {
		di := dist[i]
		ii := idx[i]
		j := i - 1
		for j >= 0 && dist[j] > di {
			dist[j+1] = dist[j]
			idx[j+1] = idx[j]
			j--
		}
		dist[j+1] = di
		idx[j+1] = ii
	}
}

// sortByDist16 — same as sortByDist8 for N=16.
func sortByDist16(idx *[16]uint8, dist *[16]int64) {
	for i := 1; i < 16; i++ {
		di := dist[i]
		ii := idx[i]
		j := i - 1
		for j >= 0 && dist[j] > di {
			dist[j+1] = dist[j]
			idx[j+1] = idx[j]
			j--
		}
		dist[j+1] = di
		idx[j+1] = ii
	}
}
