package gainquant

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// dequantize composes the codebook entry pair into (ĝp Q14, ĝc Q12)
// for a given physical (ga, gb) pair and predicted g'c (Q12), per
// §3.9 eq. (73)-(74). Used by the round-trip / bound assertions.
//
// IMPL-3: gcQ12 is held as int32 to match the unsaturated form
// SearchConjugate now uses internally (γ̂_c · g'c can exceed int16
// in practice; see PredictedGcQ12 docstring).
func dequantize(ga, gb uint8, gpcPredQ12 int32) (gpQ14 int16, gcQ12 int32) {
	gpQ14 = tables.GainGBK1[ga][0] + tables.GainGBK2[gb][0]
	gammaCQ13 := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
	gcQ12 = (gammaCQ13 * gpcPredQ12) >> 13
	return
}

// gcFromGamma reconstructs ĝc Q12 (int32, unsaturated) from the
// (gammaCQ13, g'c Q12) pair returned by SearchConjugate, mirroring
// the encoder's downstream dequant before the §A.3.10 commit.
func gcFromGamma(gammaCQ13 int16, gpcPredQ12 int32) int32 {
	return (int32(gammaCQ13) * gpcPredQ12) >> 13
}

// TestSearchConjugate_ZeroInputsReturnsValidIndices pins the §3.9.2
// degenerate edge case: with zero target/adaptive/fixed contributions
// the cost (eq. 63) is constant in (ĝp, ĝc); the search must still
// return a valid (ga, gb) pair whose decoded gains match the
// codebook-summed entry, without panicking.
func TestSearchConjugate_ZeroInputsReturnsValidIndices(t *testing.T) {
	var x, y, z [40]int16
	ga, gb, gp, gamma := SearchConjugate(&x, &y, &z, 4096)
	if ga >= 8 {
		t.Fatalf("ga = %d, want < 8", ga)
	}
	if gb >= 16 {
		t.Fatalf("gb = %d, want < 16", gb)
	}
	wantGp, wantGc := dequantize(ga, gb, 4096)
	gc := gcFromGamma(gamma, 4096)
	if gp != wantGp {
		t.Fatalf("gp = %d, want %d (codebook-derived)", gp, wantGp)
	}
	if gc != wantGc {
		t.Fatalf("gc = %d, want %d (codebook-derived)", gc, wantGc)
	}
}

// TestSearchConjugate_RoundTripCodebookEntry pins the §3.9.2 search:
// when (x, y, z) are constructed such that the optimal (gp_opt, gc_opt)
// solving the eq. 63 partial-derivative system land *exactly* on a
// codebook combo (ga, gb), the search must return that combo.
//
// Construction: y, z orthogonal nonzero impulses; x = ĝp·y/2^14
// + ĝc·z/2^12 with (ĝp, ĝc) = the dequantization of (ga=4, gb=7) at
// g'c = 4096 (1.0 in Q12). The amplitude 16384 is chosen so the
// fixed-point shifts in x[i] = (ĝ·amp) >> Q are exact (no rounding).
func TestSearchConjugate_RoundTripCodebookEntry(t *testing.T) {
	const targetGA, targetGB uint8 = 4, 7
	const gpcPred int32 = 4096 // 1.0 in Q12
	wantGp, wantGc := dequantize(targetGA, targetGB, gpcPred)

	var x, y, z [40]int16
	const amp int16 = 16384
	y[0] = amp
	z[1] = amp
	x[0] = int16((int32(wantGp) * int32(amp)) >> 14)
	x[1] = int16((wantGc * int32(amp)) >> 12)

	ga, gb, gp, gamma := SearchConjugate(&x, &y, &z, gpcPred)
	gc := gcFromGamma(gamma, gpcPred)
	if ga != targetGA || gb != targetGB {
		t.Fatalf("indices: got (%d,%d) want (%d,%d); gp=%d gc=%d", ga, gb, targetGA, targetGB, gp, gc)
	}
	if gp != wantGp {
		t.Fatalf("gp = %d, want %d", gp, wantGp)
	}
	if gc != wantGc {
		t.Fatalf("gc = %d, want %d", gc, wantGc)
	}
}

// TestSearchConjugate_PurePitchOptimumNearOne pins x=y, z=0:
// gp_opt = D/A = 1.0, gc_opt = 0. The search should pick (ga, gb)
// whose ĝp is close to 1.0 and ĝc small (the codebook has no
// (1.0, 0) entry, so we just check ranges).
func TestSearchConjugate_PurePitchOptimumNearOne(t *testing.T) {
	var x, y, z [40]int16
	for i := 0; i < 40; i++ {
		y[i] = 1000
		x[i] = 1000
	}
	ga, gb, gp, gamma := SearchConjugate(&x, &y, &z, 4096)
	gc := gcFromGamma(gamma, 4096)
	if ga >= 8 || gb >= 16 {
		t.Fatalf("indices out of range: (%d,%d)", ga, gb)
	}
	// ĝp should be roughly within ±0.3 of 1.0 (Q14 = 16384) given the
	// codebook quantization granularity.
	if gp < 11000 || gp > 22000 {
		t.Fatalf("gp = %d, want roughly 16384 (1.0 Q14) ±5000", gp)
	}
	// gc should be small: best codebook entry for ĝc=0 is bounded by
	// the smallest γ̂ sum (~= GainGBK1[0][1] + GainGBK2[1][1] = 1516).
	// Allow a generous bound.
	if gc < 0 || gc > 6000 {
		t.Fatalf("gc = %d, want small (≤ 6000 Q12)", gc)
	}
}

// TestSearchConjugate_PureInnovationGcAccurate pins x=z with y small
// (orthogonal-to-z impulse).  In this regime the eq. 63 cost is
// dominated by the (ĝc - gc_opt)² · B term (B = ⟨z,z⟩ ≫ A = ⟨y,y⟩),
// so the search must pick ĝc close to gc_opt = 1.0 (Q12 = 4096).
// ĝp is *not* tightly constrained because ĝp²·A is dwarfed by the
// gc-mismatch cost (so we don't pin it).
func TestSearchConjugate_PureInnovationGcAccurate(t *testing.T) {
	var x, y, z [40]int16
	y[0] = 100
	for i := 1; i < 40; i++ {
		z[i] = 1000
		x[i] = 1000
	}
	ga, gb, _, gamma := SearchConjugate(&x, &y, &z, 4096)
	gc := gcFromGamma(gamma, 4096)
	if ga >= 8 || gb >= 16 {
		t.Fatalf("indices out of range: (%d,%d)", ga, gb)
	}
	// ĝc should be roughly within ±25%% of 4096 (gc_opt = 1.0 Q12).
	if gc < 3000 || gc > 5500 {
		t.Fatalf("gc = %d, want roughly 4096 (1.0 Q12) ±25%%", gc)
	}
}

// TestSearchConjugate_BoundsRespectCodebookEnvelope pins the §3.9.2
// implicit clamp: when gp_opt explodes (x = 10·y), the quantized ĝp
// is bounded by the codebook envelope (max GainGBK1[i][0] + max
// GainGBK2[j][0] = 3242 + 18973 = 22215). Phase 2c's *unquantized*
// 1.2 clamp does NOT apply post-VQ (the largest codebook combo
// exceeds 1.2 by design); we only assert the search produces a valid
// codebook entry.
func TestSearchConjugate_BoundsRespectCodebookEnvelope(t *testing.T) {
	var x, y, z [40]int16
	for i := 0; i < 40; i++ {
		y[i] = 1000
		x[i] = 10000
	}
	ga, gb, gp, _ := SearchConjugate(&x, &y, &z, 4096)
	if ga >= 8 || gb >= 16 {
		t.Fatalf("indices out of range: (%d,%d)", ga, gb)
	}
	const maxCodebookGp = int16(3242 + 18973) // 22215
	if gp > maxCodebookGp {
		t.Fatalf("gp = %d, exceeds codebook envelope %d", gp, maxCodebookGp)
	}
	// And it should be near the upper end (≥ ~0.8 Q14).
	if gp < 13000 {
		t.Fatalf("gp = %d, want ≥ 13000 (search should saturate near codebook ceiling)", gp)
	}
}

// TestSearchConjugate_PureFunction asserts the search reads its
// inputs without mutation.
func TestSearchConjugate_PureFunction(t *testing.T) {
	var x, y, z [40]int16
	y[0] = 1000
	z[1] = 1000
	x[0] = 668
	x[1] = 1354
	xo, yo, zo := x, y, z
	_, _, _, _ = SearchConjugate(&x, &y, &z, 4096)
	if x != xo {
		t.Errorf("x mutated")
	}
	if y != yo {
		t.Errorf("y mutated")
	}
	if z != zo {
		t.Errorf("z mutated")
	}
}

// TestSearchConjugate_DecoderRoundTrip cross-checks the encoder
// search against the decoder gain VQ: passing the encoder-returned
// (ga, gb) through the §3.9.3 forward map (GainMap1/GainMap2) and
// then through the decoder's inverse map (GainImap1/GainImap2)
// recovers the same physical entry, and the dequantized (ĝp, ĝc)
// match the search's returned (gp, gc) bit-for-bit.
func TestSearchConjugate_DecoderRoundTrip(t *testing.T) {
	const targetGA, targetGB uint8 = 4, 7
	const gpcPred int32 = 4096
	wantGp, wantGc := dequantize(targetGA, targetGB, gpcPred)

	var x, y, z [40]int16
	const amp int16 = 16384
	y[0] = amp
	z[1] = amp
	x[0] = int16((int32(wantGp) * int32(amp)) >> 14)
	x[1] = int16((wantGc * int32(amp)) >> 12)

	ga, gb, gp, gamma := SearchConjugate(&x, &y, &z, gpcPred)
	gc := gcFromGamma(gamma, gpcPred)

	transmittedGA := tables.GainMap1[ga]
	transmittedGB := tables.GainMap2[gb]
	physGA := tables.GainImap1[transmittedGA]
	physGB := tables.GainImap2[transmittedGB]
	if physGA != ga {
		t.Fatalf("GA round-trip: %d → %d → %d", ga, transmittedGA, physGA)
	}
	if physGB != gb {
		t.Fatalf("GB round-trip: %d → %d → %d", gb, transmittedGB, physGB)
	}
	gpDecoded := tables.GainGBK1[physGA][0] + tables.GainGBK2[physGB][0]
	gammaDecoded := int32(tables.GainGBK1[physGA][1]) + int32(tables.GainGBK2[physGB][1])
	gcDecoded := (gammaDecoded * gpcPred) >> 13
	if gpDecoded != gp {
		t.Fatalf("decoder gp = %d, encoder gp = %d", gpDecoded, gp)
	}
	if gcDecoded != gc {
		t.Fatalf("decoder gc = %d, encoder gc = %d", gcDecoded, gc)
	}
}

// TestSearchConjugate_ZeroAlloc pins the encoder hot-path budget:
// the conjugate-codebook search runs once per subframe (twice per
// frame) and must not allocate.
func TestSearchConjugate_ZeroAlloc(t *testing.T) {
	var x, y, z [40]int16
	for i := 0; i < 40; i++ {
		y[i] = int16(100 + i)
		z[i] = int16(50 + i*2)
		x[i] = int16(200 + i)
	}
	allocs := testing.AllocsPerRun(64, func() {
		_, _, _, _ = SearchConjugate(&x, &y, &z, 4096)
	})
	if allocs != 0 {
		t.Fatalf("allocs/op = %.2f, want 0", allocs)
	}
}
