package decoder

// PHASE 1o D-3.ter DISPOSITION — KEEP-WITH-NOTE.
//
// The hypothesis investigated by this diagnostic file has been closed by
// the gate 17 PSTdomain demotion (Phase 1o D-1b, commit 6633b28) and/or
// the Phase 1o D-3 state-bearing root-cause cycle (commits aa27ad1,
// 0428df7, bd37512, da089b5, be80eaf, c81645b — closure c81645b/this-cycle).
// Retained as evidence-trail and a verification-path demonstrator that
// future Phase-2 encoder cross-reference work may want to re-walk; do NOT
// extend this file — open a new dated diagnostic file instead. See
// session-state checkpoints 011..020 for the gate 17 / 28-cycle history,
// and docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md
// §3 D-3.ter for the housekeeping decision rationale.

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/tables"
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

// TestDiagnostic_FnonPrelimXSplit2GainGcTrace decomposes the ALGTHM
// frame 0 sf0 gain VQ output g_c=+4153 (Q12) into spec §3.9 + §3.9.1
// + §3.9.2 + §A.3.9 + §4.3 Table 9 sub-stages: (a) idx.GA1=5 / GB1=6
// raw codewords, (b) GainImap1[5] / GainImap2[6] inverse-permutation
// to physical GBK entry indices, (c) GainGBK1[entry] (g_p, γ̂_c) Q14/Q13
// pair lookup, (d) GainGBK2[entry] (g_p, γ̂_c) Q14/Q13 pair lookup,
// (e) γ̂ = GBK1[*][1] + GBK2[*][1] (eq. (74) right factor, Q13), (f) MA
// predictor Ê(m) = E̅ + Σ b_i·Û(m−i) using past_err init =
// MIN_GAIN_PRED_DB = -14336 Q10 ×4 (§4.3 Table 9 — "All static encoder
// and decoder variables should be initialized to zero, except the
// variables listed in Table 9"; gain.pastErrorsDefault), (g) production
// gain.Decoder.Decode end-to-end g_c (Q12) confirm against X-fcb verdict
// +4153, (h) sign-determining sub-stage classification.
//
// Spec ground-truth (verbatim, ITU-T G.729 (06/2012) PDF):
//   §3.9 eq. (65):  g_c = γ · g_c'
//   §3.9.1 eq. (69): Ẽ(m) = Σ_{i=1..4} b_i · Û(m−i)
//                    [b1 b2 b3 b4] = [0.68 0.58 0.34 0.19]
//   §3.9.1 eq. (71): g_c' = 10^((Ẽ(m) + Ē − E)/20),  Ē = 30 dB
//   §3.9.2 eq. (73): ĝ_p = GA1(GA) + GB1(GB)
//   §3.9.2 eq. (74): ĝ_c = g_c' · γ̂ = g_c' (GA2(GA) + GB2(GB))
//   §A.3.9        : "Same as described in clause 3.9."
//   §4.3 Table 9  : non-zero initialization variables — gain MA
//                   predictor past_err[0..3] = MIN_GAIN_PRED_DB.
//
// production wiring (§Spec §3.9 cross-ref):
//   gain.decodeVQ                  : entry = GainGBK*[GainImap*[idx]]
//   gain.predictedLogGain          : Round(LShl(LMac chain, 2)) + Ē Q10
//   gain.Decoder.Decode            : returns (gpQ14, gcQ12)
//   gain.pastErrorsDefault = -14336 (Q10) = MIN_GAIN_PRED_DB
//   tables.GainMAPredictor         = [5571, 4751, 2785, 1556] (Q13)
//   tables.GainMeanEnergyQ10       = 30720 (Ē = 30 dB Q10)
//
// F-non-prelim synthesis (e867f5e) §3.1: single source =
// `g_c · c[n]` product positive (Q15 pre-Round = +33224); §4.1
// recommends Cα + Cβ hybrid split. Task 1 (fd0b381) verdict =
// Cα-refute (c[0..3]=+8192 spec-canonical). This test = Cβ half
// (g_c=+4153 sub-source identification: VQ table γ̂ / MA predictor
// state / γ̂·g_c' composition / sign processing).
//
// Phase 0.4 §1 / §3: no a-priori preference between sub-stages —
// sign-origin is decided by the measured ROM-table / predictor / g_c
// values only. "Cβ-refute (둘 다 정합)" is a valid outcome.
//
// production 변경 0. assertion 0 (measurement-only, t.Logf dump).
func TestDiagnostic_FnonPrelimXSplit2GainGcTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	ensureTestdataPresent(t, bitPath)

	frames, _ := readG192Frames(t, bitPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// Reproduce the production sf0 fixed-codebook vector c[40] so the
	// gain decoder consumes an identical input to the regular decode
	// path (the fcb output drives gain.fixedCodebookEnergy, which
	// determines E̅ in dB and thus the predicted-gain branch).
	tInt, _ := pitch.DecodeDelaySubframe1(uint8(f.P1))
	betaQ14Prod := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14Prod, &c)

	// ──── (a) GA1 / GB1 raw codewords (bitstream side) ────
	ga := uint8(f.GA1)
	gb := uint8(f.GB1)

	// ──── (b) GainImap*[GA] / [GB] → physical GBK entry index ────
	//          (§3.9.3 inverse permutation; production gain.decodeVQ
	//          performs the same lookup verbatim.)
	gaEntry := tables.GainImap1[ga]
	gbEntry := tables.GainImap2[gb]

	// ──── (c) GainGBK1[entry] = (g_p_ga Q14, γ̂_ga Q13) ────
	gpGaQ14 := tables.GainGBK1[gaEntry][0]
	gammaGaQ13 := tables.GainGBK1[gaEntry][1]

	// ──── (d) GainGBK2[entry] = (g_p_gb Q14, γ̂_gb Q13) ────
	gpGbQ14 := tables.GainGBK2[gbEntry][0]
	gammaGbQ13 := tables.GainGBK2[gbEntry][1]

	// ──── (e) eq. (73) / (74) summation (Word16 saturating) ────
	gpSumQ14 := int16(fixed.Add(gpGaQ14, gpGbQ14))
	gammaSumQ13 := int16(fixed.Add(gammaGaQ13, gammaGbQ13))

	// ──── (f) MA predictor (frame 0 zero-state / Table 9 init) ────
	//          replicates gain.predictedLogGain verbatim using the
	//          public tables; past_err[0..3] = MIN_GAIN_PRED_DB =
	//          -14336 (Q10) per §4.3 Table 9 + gain.pastErrorsDefault.
	const minGainPredDbQ10 int16 = -14336
	pastErr := [4]int16{minGainPredDbQ10, minGainPredDbQ10, minGainPredDbQ10, minGainPredDbQ10}
	var acc fixed.Word32
	for i := 0; i < 4; i++ {
		acc = fixed.LMac(acc, tables.GainMAPredictor[i], pastErr[i])
	}
	predictedRaw := fixed.Round(fixed.LShl(acc, 2))
	predictedQ10 := int16(fixed.Add(tables.GainMeanEnergyQ10, predictedRaw))

	// ──── (g) end-to-end g_c via production gain.Decoder.Decode ────
	//          — spec eq. (65) g_c = γ̂·g_c'. zero-state Decoder; first
	//          call seeds pastErrors with pastErrorsDefault internally.
	var gn gain.Decoder
	gpQ14Prod, gcMant_gcQ12Prod, gcExp_gcQ12Prod := gn.Decode(gain.Indices{GA: ga, GB: gb}, &c)
	gcQ12Prod := gain.LegacyGcQ12FromMantExp(gcMant_gcQ12Prod, gcExp_gcQ12Prod)

	// X-fcb verdict cross-ref (F-non-prelim Task 1 §2 raw measurement).
	const xFcbGcQ12 int16 = +4153
	gcMatch := gcQ12Prod == xFcbGcQ12
	gpComposeMatch := gpQ14Prod == gpSumQ14

	t.Logf("──────── F-non-prelim-X-split-2 Cβ gain g_c sub-stage trace (ALGTHM frame 0 sf0) ────────")
	t.Logf("indices: P1=%d  C1=0x%04x  S1=0x%x  GA1=%d  GB1=%d", f.P1, f.C1, f.S1, f.GA1, f.GB1)

	// ──── (a) GA1 / GB1 raw codewords ────
	t.Logf("[Cβ idx.GA1, GB1]    GA1=%d (3 bits)  GB1=%d (4 bits)   (codewords from bitstream §4.2 Table 8)", ga, gb)

	// ──── (b) Inverse-permutation to physical entry ────
	t.Logf("[Cβ Imap]            GainImap1[%d]=%d  GainImap2[%d]=%d   (§3.9.3 inverse permutation; production gain/vq.go)", ga, gaEntry, gb, gbEntry)

	// ──── (c) GA entry ────
	t.Logf("[Cβ GA[%d] entry]    GainGBK1[%d] = (g_p_ga=%+d Q14, γ̂_ga=%+d Q13)   (PDF §3.9.2 eq.(73)/(74); tables/gain_gbk1.go)",
		gaEntry, gaEntry, gpGaQ14, gammaGaQ13)

	// ──── (d) GB entry ────
	t.Logf("[Cβ GB[%d] entry]    GainGBK2[%d] = (g_p_gb=%+d Q14, γ̂_gb=%+d Q13)   (PDF §3.9.2 eq.(73)/(74); tables/gain_gbk2.go)",
		gbEntry, gbEntry, gpGbQ14, gammaGbQ13)

	// ──── (e) γ̂ correction (eq. (74)) + ĝ_p sum (eq. (73)) ────
	t.Logf("[Cβ γ̂ correction]   eq.(74) γ̂ = γ̂_ga + γ̂_gb = %+d + %+d = %+d (Q13)", gammaGaQ13, gammaGbQ13, gammaSumQ13)
	t.Logf("[Cβ ĝ_p compose]     eq.(73) ĝ_p = g_p_ga + g_p_gb = %+d + %+d = %+d (Q14)   gain.Decode return ĝ_p=%+d  match=%v",
		gpGaQ14, gpGbQ14, gpSumQ14, gpQ14Prod, gpComposeMatch)

	// ──── (f) MA predictor (frame 0 init) ────
	t.Logf("[Cβ MA predictor]    past_err init=[%+d %+d %+d %+d] (Q10, all = MIN_GAIN_PRED_DB per §4.3 Table 9)",
		pastErr[0], pastErr[1], pastErr[2], pastErr[3])
	t.Logf("                     b=[%+d %+d %+d %+d] (Q13; spec §3.9.1 [0.68 0.58 0.34 0.19]; tables.GainMAPredictor)",
		tables.GainMAPredictor[0], tables.GainMAPredictor[1], tables.GainMAPredictor[2], tables.GainMAPredictor[3])
	t.Logf("                     LMac acc(Q24)=%+d  Round(LShl(acc,2))=%+d (Q10)  Ẽ contribution",
		acc, predictedRaw)
	t.Logf("                     Ê(m) = Ē + Σb·Û = %+d + %+d = %+d (Q10 dB)   (Ē=GainMeanEnergyQ10=30720 = 30 dB Q10)",
		tables.GainMeanEnergyQ10, predictedRaw, predictedQ10)

	// ──── (g) Σc² + production end-to-end g_c ────
	var sumSqQ26 int64
	for n := 0; n < subframeLen; n++ {
		sumSqQ26 += int64(c[n]) * int64(c[n])
	}
	t.Logf("[Cβ fcb energy]      Σc² (Q26) = %d   (input to gain.fixedCodebookEnergy → log2 → Ē̄ in eq.(66))", sumSqQ26)
	t.Logf("[Cβ g_c (Q12)]       gain.Decode → (ĝ_p=%+d Q14, ĝ_c=%+d Q12)   X-fcb verdict +4153 match=%v",
		gpQ14Prod, gcQ12Prod, gcMatch)

	// ──── (h) ROM cross-ref vs PDF Table (verbatim numeric) ────
	//          (Tables in tables/gain_gbk*.go are bit-exact from the
	//          ITU reference data array under the merger-doctrine
	//          exception per the file headers; the PDF itself does
	//          NOT print the numeric GBK table — §3.9.2 only gives
	//          dimensions 8×2 / 16×2. This row records the dimension
	//          + sign-of-γ̂ check rather than per-entry verbatim.)
	t.Logf("[Cβ ROM cross-ref]   GainGBK1 dim=%d×2 (spec §3.9.2: 3-bit GA → 8 entries × 2-dim)  GainGBK2 dim=%d×2 (4-bit GB → 16 × 2-dim)  γ̂_ga sign=%s  γ̂_gb sign=%s  γ̂ sum sign=%s",
		len(tables.GainGBK1), len(tables.GainGBK2),
		signOfInt16(gammaGaQ13), signOfInt16(gammaGbQ13), signOfInt16(gammaSumQ13))

	// ──── (i) Cβ sub-stage 부호 결정성 평가 ────
	t.Logf("──────── Cβ sub-stage 부호 결정성 평가 ────────")
	verdict := classifyCbetaSubStage(gammaSumQ13, predictedQ10, gcQ12Prod, gcMatch, gpComposeMatch)
	t.Logf("[Cβ 결정] sign-determining sub-stage = %s", verdict)
	t.Logf("[Cβ verdict] %s", classifyCbetaHypothesis(gammaGaQ13, gammaGbQ13, gammaSumQ13, predictedQ10, gcQ12Prod, gcMatch, gpComposeMatch))
}

// classifyCbetaSubStage decides which gain sub-stage determines the
// sign of g_c=+4153 for ALGTHM frame 0 sf0 by applying plan §Task 2
// Step 4 의 decision table verbatim against the measured VQ-table /
// predictor / composition values. Phase 0.4 §1 — measurement-driven
// only.
func classifyCbetaSubStage(gammaSumQ13, predictedQ10, gcQ12Prod int16, gcMatch, gpComposeMatch bool) string {
	gammaPositive := gammaSumQ13 > 0
	predictedFinite := predictedQ10 > -32000 && predictedQ10 < 32000
	gcPositive := gcQ12Prod > 0

	switch {
	case gcMatch && gpComposeMatch && gammaPositive && predictedFinite && gcPositive:
		return "VQ-table-γ̂ (γ̂ = GBK1[ga][1] + GBK2[gb][1] = " + itoa(int32(gammaSumQ13)) + " > 0 (Q13); MA predictor zero-state finite Ê(m)=" + itoa(int32(predictedQ10)) + " (Q10); g_c = γ̂·g_c' inherits γ̂ sign — single source = positive VQ entry, no sign-mask logic involved)"
	case gcMatch && !gammaPositive && gcPositive:
		return "spec-violation (γ̂ sum ≤ 0 yet g_c > 0 — sign inversion in γ̂·g_c' composition)"
	case gcMatch && gammaPositive && !gcPositive:
		return "spec-violation (γ̂ sum > 0 yet g_c ≤ 0 — sign loss in γ̂·g_c' composition)"
	case !gcMatch:
		return "replication-mismatch (production gain.Decode g_c=" + itoa(int32(gcQ12Prod)) + " ≠ X-fcb verdict +4153 — X-fcb baseline drift; investigate)"
	default:
		return "undetermined (sub-stage values do not fit known pattern)"
	}
}

// classifyCbetaHypothesis maps the sub-stage decomposition to one of
// the Cβ verdict labels per plan §Task 2 Step 4: Cβ-vq / Cβ-pred /
// Cβ-refute / Cβ-inconclusive. Phase 0.4 §3 — "둘 다 spec 정합" is a
// valid outcome (Cβ-refute), routing to Cγ re-entry or Y magnitude
// follow-up per Task 3 의 결정 트리.
func classifyCbetaHypothesis(gammaGaQ13, gammaGbQ13, gammaSumQ13, predictedQ10, gcQ12Prod int16, gcMatch, gpComposeMatch bool) string {
	gammaGaOk := gammaGaQ13 >= 0
	gammaGbOk := gammaGbQ13 >= 0
	gammaPositive := gammaSumQ13 > 0
	predFinite := predictedQ10 > -32000 && predictedQ10 < 32000
	gcPositive := gcQ12Prod > 0

	switch {
	case gcMatch && gpComposeMatch && gammaGaOk && gammaGbOk && gammaPositive && predFinite && gcPositive:
		// All sub-stages match spec §3.9.* verbatim:
		// - GBK entry pair = production ROM (no replication drift).
		// - γ̂ sum = positive Q13 (eq. (74) RHS).
		// - MA predictor frame-0 zero-state per §4.3 Table 9.
		// - Composition g_c = γ̂·g_c' preserves γ̂ sign.
		// - Final g_c = +4153 = X-fcb baseline.
		// → No defect in gain.Decoder.Decode; sign of g_c is
		//   spec-canonical positive.
		return "Cβ-refute (g_c=+4153 양 부호 = §3.9 spec-canonical: γ̂ = +" + itoa(int32(gammaSumQ13)) + " (Q13) > 0 × frame-0 zero-state predictor (Ê(m)=" + itoa(int32(predictedQ10)) + " Q10) × eq.(65) g_c=γ̂·g_c' 부호 보존. gain.Decoder.Decode 정합 — Cβ 결함 없음. **Cα + Cβ 둘 다 정합** (hybrid 반증) → Task 3 결정 트리 §3 권고: Cγ 재진입 또는 Y magnitude follow-up.)"
	case gcMatch && (!gammaGaOk || !gammaGbOk):
		return "Cβ-vq (GBK ROM entry sign 결함 — fix scope = internal/tables/gain_gbk*.go)"
	case gcMatch && !predFinite:
		return "Cβ-pred (MA predictor 결과 saturated/invalid — fix scope = internal/gain/predictor.go 또는 pastErrorsDefault)"
	case gcMatch && gammaPositive && gcPositive && !gpComposeMatch:
		return "Cβ-pred (ĝ_p sum mismatch — composition layer 결함; fix scope = internal/gain/vq.go)"
	default:
		return "Cβ-inconclusive (sub-stage 측정 데이터로 단일 sub-source 식별 불가 — Task 3 종합 §3 결정 트리에서 hybrid 평가)"
	}
}
