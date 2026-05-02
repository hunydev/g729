package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/pitch"
)

// TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace decomposes the ALGTHM
// frame 0 sf0 fcb codebook output c[0..3] = +8192 (Q13) into spec §3.8
// + §3.8.2 sub-stages: (a) idx.Positions (4-pulse positions m0..m3),
// (b) idx.Signs (4-bit sign mask, expected 0xf), (c) raw pulse
// placement c_raw[0..39] obtained by calling fcb.Decode with the β=0
// stub (spec §3.8.2 enhancement-off path = direct equation (45) only),
// and (d) production β·c[n−T] enhancement Δ obtained by differencing
// the production fcb.Decode (β = clamp(g_p_prev=0) = Q14 3277, T=tInt)
// against c_raw.
//
// Spec ground-truth (verbatim, ITU-T G.729 (06/2012) PDF):
//   §3.8 eq. (45):
//     c(n) = s0 δ(n − m0) + s1 δ(n − m1) + s2 δ(n − m2) + s3 δ(n − m3)
//   §3.8.2 eq. (61):  S = s0 + 2 s1 + 4 s2 + 8 s3   (s = 1 → +1, s = 0 → −1)
//   §3.8 eq. (47):  β = ĝ_p^(m−1)   bounded by 0.2 ≤ β ≤ 0.8
//   §3.8 eq. (48):  c(n) = c(n)              for n = 0..T−1
//                   c(n) = c(n) + β c(n − T)  for n = T..39
//   §A.3.8: decoder-side decoding identical to encoder-side §3.8.2.
//   §4.1.5/6 eq. (75):  u[n] = Round(g_p · v[n] + g_c · c[n])
//   §4.3 Table 9: past_exc[*] = 0, ĝ_p^(−1) = 0 (implicit, frame 0).
//
// F-non-prelim synthesis (e867f5e) §3.1 identifies single source =
// `g_c · c[n]` product positive (Q15 pre-Round = +33224). §4.1
// recommends Cα + Cβ hybrid split. This test = Cα half (c[n] sub-source
// identification: raw-placement / β-enhancement / spec-canonical).
//
// Phase 0.4 §1: no a-priori preference between sub-stages — sign-origin
// is decided by the measured c_raw / Δ values only.
//
// production 변경 0. assertion 0 (measurement-only, t.Logf dump).
func TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	ensureTestdataPresent(t, bitPath)

	frames, _ := readG192Frames(t, bitPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))

	// (a) idx.Positions decoding (replicates fcb.decodePositions per
	//     §3.8 Table 7 layout — bits 12..10 / 9..7 / 6..4 / 3 / 2..0).
	code := f.C1
	i0 := int((code >> 10) & 0x07)
	i1 := int((code >> 7) & 0x07)
	i2 := int((code >> 4) & 0x07)
	jx := int((code >> 3) & 0x01)
	i3 := int(code & 0x07)
	m := [4]int{
		5 * i0,
		5*i1 + 1,
		5*i2 + 2,
		5*i3 + 3 + jx,
	}

	// (b) idx.Signs decoding per §3.8 eq. (45) / placePulses convention
	//     (sign_bit_i = bit (3-i) of S; 1 → +1, 0 → −1).
	signs := uint8(f.S1)
	s := [4]int16{}
	for i := 0; i < 4; i++ {
		if (signs>>(3-uint(i)))&1 == 1 {
			s[i] = +1
		} else {
			s[i] = -1
		}
	}

	// (c) raw pulse placement: β = 0 stub ⇒ §3.8.2 eq. (48) reduces to
	//     eq. (45) only (enhancement off — fcb.applyPitchEnhancement
	//     short-circuits on betaQ14 == 0).
	var cRaw [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: signs}, tInt, 0, &cRaw)

	// (d) production: β = clamp(g_p_prev=0) = betaLowerQ14 = Q14 3277,
	//     T = tInt = 20 (frame 0 sf0 — first-frame artefact on
	//     past_exc=0).
	betaQ14Prod := fcb.ClampPitchGainForEnhancement(0)
	var cProd [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: signs}, tInt, betaQ14Prod, &cProd)

	// Δ[n] = cProd[n] − cRaw[n] = β·c[n−T] enhancement contribution
	//       (Q13 saturating Add; spec §3.8.2 eq. (48) second branch).
	var delta [subframeLen]int32
	for n := 0; n < subframeLen; n++ {
		delta[n] = int32(cProd[n]) - int32(cRaw[n])
	}

	// nonzero indices of c_raw (expect exactly 4 entries at m0..m3).
	rawNonzero := make([]int, 0, 4)
	for n := 0; n < subframeLen; n++ {
		if cRaw[n] != 0 {
			rawNonzero = append(rawNonzero, n)
		}
	}

	t.Logf("──────── F-non-prelim-X-split-1 Cα fcb c[n] sub-stage trace (ALGTHM frame 0 sf0) ────────")
	t.Logf("indices: P1=%d  C1=0x%04x  S1=0x%x  GA1=%d  GB1=%d", f.P1, f.C1, f.S1, f.GA1, f.GB1)
	t.Logf("pitch delay: tInt=%d  tFrac=%d   beta_q14_prod=%d (clamp(g_p_prev=0) → 0.2·2^14)", tInt, tFrac, betaQ14Prod)

	// ──── (a) idx.Positions ────
	t.Logf("[Cα idx.Positions]   raw=0x%04x  decoded fields i0=%d i1=%d i2=%d jx=%d i3=%d  →  m=[%d,%d,%d,%d]",
		f.C1, i0, i1, i2, jx, i3, m[0], m[1], m[2], m[3])
	t.Logf("                     §3.8 Table 7 tracks: m0∈{0,5,..,35}  m1∈{1,6,..,36}  m2∈{2,7,..,37}  m3∈{3,8,..,38}∪{4,9,..,39}")

	// ──── (b) idx.Signs ────
	t.Logf("[Cα idx.Signs]       raw=0x%x   expected-for-X-fcb-positive=0xf   decoded s=[%+d,%+d,%+d,%+d]  ∈{+1,−1}",
		signs, s[0], s[1], s[2], s[3])

	// ──── (c) c_raw[0..39] (β=0 stub) ────
	t.Logf("[Cα c_raw[0..39]]    nonzero @ n=%v  (expected = m=[%d,%d,%d,%d])", rawNonzero, m[0], m[1], m[2], m[3])
	for k, n := range rawNonzero {
		t.Logf("   c_raw[%d] = %+d  (track %d, sign %s, |v|=%d=PulseAmplitude=Q13 +1.0)",
			n, cRaw[n], k, signOfInt16(cRaw[n]), absInt16(cRaw[n]))
	}
	t.Logf("[Cα c_raw[0..3]]     = [%+d %+d %+d %+d]   signs=[%s %s %s %s]",
		cRaw[0], cRaw[1], cRaw[2], cRaw[3],
		signOfInt16(cRaw[0]), signOfInt16(cRaw[1]), signOfInt16(cRaw[2]), signOfInt16(cRaw[3]))

	// ──── (d) production c[0..39] + Δ ────
	t.Logf("[Cα c_prod[0..3]]    = [%+d %+d %+d %+d]   signs=[%s %s %s %s]",
		cProd[0], cProd[1], cProd[2], cProd[3],
		signOfInt16(cProd[0]), signOfInt16(cProd[1]), signOfInt16(cProd[2]), signOfInt16(cProd[3]))
	t.Logf("[Cα Δ[0..3] = c_prod−c_raw]  = [%+d %+d %+d %+d]   (β·c[n−T] enhancement; spec §3.8.2 eq.(48): Δ[n]=0 for n<T=%d)",
		delta[0], delta[1], delta[2], delta[3], tInt)

	// Δ summary across full subframe (enhancement first appears at n≥T).
	deltaNonzero := make([]int, 0, 40)
	for n := 0; n < subframeLen; n++ {
		if delta[n] != 0 {
			deltaNonzero = append(deltaNonzero, n)
		}
	}
	t.Logf("[Cα Δ[0..39] nonzero indices] = %v   (expected first nonzero @ n≥T=%d)", deltaNonzero, tInt)

	// ──── (e) X-fcb verdict cross-check ────
	xFcbMatch := cProd[0] == 8192 && cProd[1] == 8192 && cProd[2] == 8192 && cProd[3] == 8192
	t.Logf("[Cα c[0..3] vs X-fcb verdict (+8192,+8192,+8192,+8192)]  match=%v", xFcbMatch)

	// ──── (f) Cα sub-stage 부호 결정성 평가 ────
	t.Logf("──────── Cα sub-stage 부호 결정성 평가 ────────")
	verdict := classifyCalphaSubStage(s, m, cRaw[:], cProd[:], delta[:], tInt)
	t.Logf("[Cα 결정] sample 0..3 sign-determining sub-stage = %s", verdict)
	t.Logf("[Cα verdict] %s", classifyCalphaHypothesis(s, m, cRaw[:], cProd[:], delta[:], tInt))
}

// classifyCalphaSubStage decides which fcb sub-stage determines the
// sign of c[0..3] for ALGTHM frame 0 sf0, by applying the decision
// table in plan §Task 1 Step 5 verbatim against the measured raw /
// production / Δ values. Phase 0.4 §1 — measurement-driven only.
func classifyCalphaSubStage(s [4]int16, m [4]int, cRaw, cProd []int16, delta []int32, tInt int) string {
	rawAllPositive03 := cRaw[0] == 8192 && cRaw[1] == 8192 && cRaw[2] == 8192 && cRaw[3] == 8192
	deltaZero03 := delta[0] == 0 && delta[1] == 0 && delta[2] == 0 && delta[3] == 0
	rawZero03 := cRaw[0] == 0 && cRaw[1] == 0 && cRaw[2] == 0 && cRaw[3] == 0
	prodPositive03 := cProd[0] == 8192 && cProd[1] == 8192 && cProd[2] == 8192 && cProd[3] == 8192
	signsAllPlus := s[0] == 1 && s[1] == 1 && s[2] == 1 && s[3] == 1
	mInLow := m[0] >= 0 && m[0] < 4 && m[1] >= 0 && m[1] < 4 && m[2] >= 0 && m[2] < 4 && m[3] >= 0 && m[3] < 4

	switch {
	case rawAllPositive03 && deltaZero03 && signsAllPlus && mInLow:
		return "raw-placement (4-pulse positions ∈ {0,1,2,3} × Signs=0xf → c_raw[0..3] = +8192; β·enhancement off — n<T=" + itoa(int32(tInt)) + ")"
	case rawAllPositive03 && deltaZero03:
		return "raw-placement (sign 0xf decoded with positions covering n=0..3; enhancement Δ=0)"
	case !rawAllPositive03 && prodPositive03:
		return "β-enhancement (raw c_raw[0..3] not all +8192 but production c[0..3] = +8192 — spec §3.8.2 violation candidate; T=" + itoa(int32(tInt)) + ")"
	case rawZero03 && prodPositive03:
		return "spec-violation (raw pulses absent in n=0..3 but production c[0..3] ≠ 0 — §3.8.2 eq.(48) n<T branch violated)"
	case !prodPositive03:
		return "replication-mismatch (production c[0..3] ≠ +8192 — replication of decodeSubframe path failed; investigate fcb.Decode or input wiring)"
	default:
		return "undetermined (sub-stage values do not fit known pattern)"
	}
}

// classifyCalphaHypothesis maps the sub-stage decomposition to one of
// the four Cα verdict labels in plan §Task 1 Step 5: Cα-raw / Cα-enh /
// Cα-refute / Cα-inconclusive. Phase 0.4 §3 — "둘 다 spec 정합" is a
// valid outcome (Cα-refute).
func classifyCalphaHypothesis(s [4]int16, m [4]int, cRaw, cProd []int16, delta []int32, tInt int) string {
	rawAllPositive03 := cRaw[0] == 8192 && cRaw[1] == 8192 && cRaw[2] == 8192 && cRaw[3] == 8192
	deltaZero03 := delta[0] == 0 && delta[1] == 0 && delta[2] == 0 && delta[3] == 0
	signsAllPlus := s[0] == 1 && s[1] == 1 && s[2] == 1 && s[3] == 1
	mInLow := m[0] >= 0 && m[0] < 4 && m[1] >= 0 && m[1] < 4 && m[2] >= 0 && m[2] < 4 && m[3] >= 0 && m[3] < 4
	prodPositive03 := cProd[0] == 8192 && cProd[1] == 8192 && cProd[2] == 8192 && cProd[3] == 8192

	switch {
	case rawAllPositive03 && deltaZero03 && signsAllPlus && mInLow && prodPositive03:
		// §3.8 eq.(45) + §3.8.2 eq.(48) n<T branch satisfied verbatim.
		// Sign of c[0..3] is the spec-canonical output of the decoded
		// codeword (S=0xf, positions covering n=0..3). No defect in fcb.Decode.
		return "Cα-refute (c[0..3] 양 부호 = §3.8 spec-canonical: Signs=0xf decoding × positions ∈ {0..3} × β·enhancement off (n<T=20). fcb.Decode 정합 — Cα 결함 없음. Cβ 단독 fix scope 후보 강화.)"
	case rawAllPositive03 && deltaZero03:
		return "Cα-raw (raw placement 단독 결정; signs/positions decoding 의 spec 정합성은 §3.8 verbatim 재확인 필요)"
	case !rawAllPositive03 && prodPositive03:
		return "Cα-enh (β·c[n−T] enhancement 가 c[0..3] 부호 결정 — sample 0..3 영역에 n<T enhancement 누적 = §3.8.2 eq.(48) n<T 분기 위반)"
	default:
		return "Cα-inconclusive (sub-stage 측정 데이터로 단일 sub-source 식별 불가 — 보고서 §0 한계 명시 + Task 2 Cβ 측정으로 hybrid 평가)"
	}
}

func absInt16(v int16) int32 {
	if v < 0 {
		return -int32(v)
	}
	return int32(v)
}
