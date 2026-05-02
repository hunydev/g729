package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/pitch"
)

// TestDiagnostic_Phase1nRc2PitchPreemphasisALGTHM: Phase 1n Stage R-C-empirical
// Task RC-2 — pitch pre-emphasis residue-0 sub-measurement (CE-2 mechanistic
// confirmation).
//
// Sub-hypothesis (RC-2):
//
//	ALGTHM frame 0 sf0의 pitch pre-emphasis c'(n) = c(n) + β·c'(n−T) 는
//	sample 5..7 에 직접 기여하지 않는다 (CE-2 c[5..7]=0 evidence 의 mechanistic
//	confirmation): T1 ∉ {5, 6, 7} 또는 β1 = 0 중 하나가 참.
//
// Background: Phase 1m CE-2 (commit 0d58ca6) 는 ALGTHM frame 0 sf0 의 PROD/SPEC
// c[] 양쪽에서 c[5]=c[6]=c[7]=0 을 결정적으로 측정했다. RC-2 는 그 측정값을
// "T1, β1 의 raw 값에서 mechanism 으로도 c[5..7]=0 이 나온다" 로 닫는다.
// 만일 T1 ∈ {5,6,7} 이고 β1 ≠ 0 이면 c[T1] = β·c[0] ≠ 0 (c[0] = ±8192) 이
// 필연; 따라서 관측된 c[5..7]=0 은 분리(disjunction) T1 ∉ {5,6,7} ∨ β1 = 0
// 의 한쪽이 참임을 강제한다.
//
// 본 테스트는 production 0 변경 (E2 유지). 측정-only:
//   - assertion 통과 → CE-2 mechanistic confirmation (mechanism eliminated).
//   - assertion 실패 → NE: CE-2 c[5..7]=0 측정값과 contradict 하는 surprising
//     finding; RC-3 synthesis 가 즉시 escalate.
//
// SPEC quotes (G.729 06/2012, §3.8, pp. 20–21, verbatim via pdftotext):
//
// SPEC §3.8 eq (45):
//
//	c(n) = s0·δ(n − m0) + s1·δ(n − m1) + s2·δ(n − m2) + s3·δ(n − m3),
//	n = 0,...,39
//
// SPEC §3.8 eq (46):
//
//	"the selected codebook vector is filtered through an adaptive
//	pre-filter P(z) that enhances harmonic components to improve the
//	quality of the reconstructed speech. Here the filter:
//	    P(z) = 1 / (1 − β·z^(−T))
//	is used, where T is the integer component of the pitch delay of the
//	current subframe, and β is a pitch gain."
//
// SPEC §3.8 eq (47):
//
//	β = ĝ_p^(m−1)         bounded by 0.2 ≤ β ≤ 0.8
//
// SPEC §3.8 eq (48):
//
//	"For delays less than 40, the codebook c(n) of equation (45) is
//	modified according to:
//	    c(n) = c(n)                          n = 0,...,T−1
//	    c(n) = c(n) + β·c(n − T)             n = T,...,39"
func TestDiagnostic_Phase1nRc2PitchPreemphasisALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (a) T1 raw extraction — pitch.DecodeDelaySubframe1 (sf0 path).
	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))

	// (b) β1 raw extraction — frame 0 sf0 진입 시점 prevGp = 0 (zero-init).
	//     ClampPitchGainForEnhancement(0) → betaLowerQ14 = 3277 (Q14 0.2),
	//     per §3.8 eq (47) lower bound clamp.
	beta1Q14 := fcb.ClampPitchGainForEnhancement(0)

	// (c) "before pre-emphasis" snapshot — fcb.Decode with betaQ14 = 0
	//     forces applyPitchEnhancement to no-op (eq (45) only, no eq (48)).
	var cBefore [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt1, 0, &cBefore)

	// (d) "after pre-emphasis" snapshot — fcb.Decode with the production
	//     beta1Q14 (eq (45) followed by eq (48) in-place IIR).
	var cAfter [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt1, beta1Q14, &cAfter)

	// Cell-matrix dump.
	tInBand := tInt1 >= 5 && tInt1 <= 7
	betaZero := beta1Q14 == 0
	t.Logf("ALGTHM frame 0 sf0: T1=%d  tFrac1=%d  β1Q14=%d  T1∈{5,6,7}?=%v  β1==0?=%v",
		tInt1, tFrac1, beta1Q14, tInBand, betaZero)
	t.Logf("c[0..39] BEFORE pre-emphasis (Q13) = %v", cBefore[:])
	t.Logf("c[5..7]  BEFORE pre-emphasis (Q13) = %v", cBefore[5:8])
	t.Logf("c[5..7]  AFTER  pre-emphasis (Q13) = %v", cAfter[5:8])

	// Cross-check vs CE-2 (commit 0d58ca6): c[5..7] post-pre-emphasis must
	// equal 0 (disjunction-driven mechanistic prediction).
	ce2Confirm := cAfter[5] == 0 && cAfter[6] == 0 && cAfter[7] == 0
	t.Logf("CE2-confirm (c[5..7]==0 after pre-emphasis): %v", ce2Confirm)

	// Verdict line — small cell matrix, single row.
	t.Logf("| T1 | β1Q14 | T1∈{5,6,7}? | β1==0? | c[5] before/after | c[6] before/after | c[7] before/after | verdict |")
	verdict := "EQ (CE-2 mechanistic confirmation: T1 ∉ {5,6,7} ∨ β1 = 0)"
	if tInBand && !betaZero {
		verdict = "NE (T1 ∈ {5,6,7} AND β1 ≠ 0 — contradicts CE-2 c[5..7]=0)"
	}
	t.Logf("| %d | %d | %v | %v | %d/%d | %d/%d | %d/%d | %s |",
		tInt1, beta1Q14, tInBand, betaZero,
		cBefore[5], cAfter[5], cBefore[6], cAfter[6], cBefore[7], cAfter[7], verdict)

	// Assertion (RC-2): T1 ∉ {5, 6, 7} OR β1 == 0.
	if tInBand && !betaZero {
		t.Errorf("RC-2 assertion failed: T1=%d ∈ {5,6,7} AND β1Q14=%d ≠ 0; "+
			"pitch pre-emphasis would write β·c[0] into c[T1], yet CE-2 (commit 0d58ca6) "+
			"measured c[5..7]=0. Surprising NE — escalate to RC-3 synthesis.",
			tInt1, beta1Q14)
	}
}
