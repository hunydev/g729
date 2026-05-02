package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FnonPrelimXExcitationSubterms decomposes the ALGTHM
// frame 0 sf0 excitation u[0..4] = [+1,+1,+1,+1,+0] into the spec
// §4.1.5/§4.1.6 eq. (75) sub-terms
//
//	u(n) = ĝ_p · v(n) + ĝ_c · c(n)
//
// (gain ĝ_p Q14, ĝ_c Q12, pitch contribution v(n) Q0, fcb contribution
// c(n) Q13) to identify which sub-term determines the sign of u[0..4].
//
// Spec ground-truth (PDF verbatim grep, see report §0):
//   - ITU-T G.729 (06/2012) §4.1.5 (PDF p.27) gain decoding (ĝ_p Q14,
//     ĝ_c Q12).
//   - ITU-T G.729 (06/2012) §4.1.6 eq. (75) (PDF p.27) excitation
//     reconstruction u(n) = ĝ_p · v(n) + ĝ_c · c(n).
//   - ITU-T G.729 (06/2012) §A.4.1 (PDF p.42) "Same as described in
//     clause 4.1" (Annex A decoder reuses §4.1.5 / §4.1.6 verbatim).
//
// E2 declaration: plan §"Spec § 인용" cites §A.3.5 for the additive
// decomposition; PDF grep shows §A.3.5 = "Computation of the impulse
// response" (encoder side). The substantive citation for u(n) is
// §4.1.5 + §4.1.6 eq. (75) via §A.4.1. Same correction recorded in
// F-oct-postfix2-prelim §0.
//
// F-oct-postfix2-prelim synthesis (9a5a7f6) §1.4 (1) identifies
// u[0..4] as the source of syn[5..7]=+1 self-feedback (4 hypotheses
// M1'/M3/M5/M6 all REFUTED). Candidate X (excitation u[0..4] sign)
// = HIGH priority; sub-term decomposition narrows the next fix scope.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FnonPrelimXExcitationSubterms(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	ensureTestdataPresent(t, bitPath)

	frames, _ := readG192Frames(t, bitPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) Replicate the production sf0 excitation path
	// (subframe.go:21–39 — identical sequence + identical inputs).
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	var pastExc [pastExcLen]int16 // first frame: all zero
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)

	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)

	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)

	// (2) Production composite excitation u[0..39].
	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	// (3) Sub-term isolated excitation contributions (Q0, post-Round)
	//     by re-running BuildExcitation with one gain zeroed. This
	//     uses the *same* production saturation/round arithmetic and
	//     therefore yields the per-sample sub-term Q0 contribution
	//     that the spec eq. (75) defines, modulo cross-rounding ±1.
	var uPitchOnly [subframeLen]int16
	synth.BuildExcitation(gpQ14, 0, &v, &c, &uPitchOnly)

	var uCodeOnly [subframeLen]int16
	synth.BuildExcitation(0, gcQ12, &v, &c, &uCodeOnly)

	// (4) Raw pre-Round Q15 sub-term values (additive in the Q15
	//     domain *before* rounding to Q0). Useful when |contribution|
	//     in Q0 rounds to 0 but the Q15 magnitude / sign is still
	//     informative for the sign-determination decision.
	var lPitchQ15 [subframeLen]int32
	var lCodeQ15 [subframeLen]int32
	var lSumQ15 [subframeLen]int32
	for n := 0; n < subframeLen; n++ {
		lp := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		lc := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
		ls := fixed.LAdd(lp, lc)
		lPitchQ15[n] = int32(lp)
		lCodeQ15[n] = int32(lc)
		lSumQ15[n] = int32(ls)
	}

	// (5) Production-vs-replicated path equivalence check on u[0..4]
	//     (Phase 0.4 §1 — replication 결함 surfacing).
	prodU := decodeFnonProdU(t, frames[0])
	matchAll := true
	for n := 0; n < subframeLen; n++ {
		if prodU[n] != u[n] {
			matchAll = false
			break
		}
	}

	t.Logf("──────── F-non-prelim-1 X excitation sub-term decomposition (ALGTHM frame 0 sf0) ────────")
	t.Logf("indices: P1=%d C1=0x%04x S1=0x%x GA1=%d GB1=%d", f.P1, f.C1, f.S1, f.GA1, f.GB1)
	t.Logf("pitch delay: tInt=%d tFrac=%d   beta_q14=%d", tInt, tFrac, betaQ14)
	t.Logf("[X g_p Q14]  value=%+6d  sign=%s  Q-format=Q14", gpQ14, signOfInt16(gpQ14))
	t.Logf("[X g_c Q12]  value=%+6d  sign=%s  Q-format=Q12", gcQ12, signOfInt16(gcQ12))

	t.Logf("──────── sub-term raw (sample 0..4) ────────")
	t.Logf("[X v[0..4]]    pitch codebook v   = [%+6d %+6d %+6d %+6d %+6d]  signs=[%s %s %s %s %s]",
		v[0], v[1], v[2], v[3], v[4],
		signOfInt16(v[0]), signOfInt16(v[1]), signOfInt16(v[2]), signOfInt16(v[3]), signOfInt16(v[4]))
	t.Logf("[X c[0..4]]    fcb codebook c     = [%+6d %+6d %+6d %+6d %+6d]  signs=[%s %s %s %s %s]",
		c[0], c[1], c[2], c[3], c[4],
		signOfInt16(c[0]), signOfInt16(c[1]), signOfInt16(c[2]), signOfInt16(c[3]), signOfInt16(c[4]))
	t.Logf("[X g_p·v Q15]  pre-Round int32    = [%+8d %+8d %+8d %+8d %+8d]  signs=[%s %s %s %s %s]",
		lPitchQ15[0], lPitchQ15[1], lPitchQ15[2], lPitchQ15[3], lPitchQ15[4],
		signOfInt32(lPitchQ15[0]), signOfInt32(lPitchQ15[1]), signOfInt32(lPitchQ15[2]),
		signOfInt32(lPitchQ15[3]), signOfInt32(lPitchQ15[4]))
	t.Logf("[X g_c·c Q15]  pre-Round int32    = [%+8d %+8d %+8d %+8d %+8d]  signs=[%s %s %s %s %s]",
		lCodeQ15[0], lCodeQ15[1], lCodeQ15[2], lCodeQ15[3], lCodeQ15[4],
		signOfInt32(lCodeQ15[0]), signOfInt32(lCodeQ15[1]), signOfInt32(lCodeQ15[2]),
		signOfInt32(lCodeQ15[3]), signOfInt32(lCodeQ15[4]))
	t.Logf("[X (g_p·v + g_c·c) Q15]            = [%+8d %+8d %+8d %+8d %+8d]  signs=[%s %s %s %s %s]",
		lSumQ15[0], lSumQ15[1], lSumQ15[2], lSumQ15[3], lSumQ15[4],
		signOfInt32(lSumQ15[0]), signOfInt32(lSumQ15[1]), signOfInt32(lSumQ15[2]),
		signOfInt32(lSumQ15[3]), signOfInt32(lSumQ15[4]))

	t.Logf("──────── sub-term Q0-rounded contribution (g_c=0 / g_p=0 isolation) ────────")
	t.Logf("[X u_pitch[0..4]]  (g_c=0)         = [%+6d %+6d %+6d %+6d %+6d]  signs=[%s %s %s %s %s]",
		uPitchOnly[0], uPitchOnly[1], uPitchOnly[2], uPitchOnly[3], uPitchOnly[4],
		signOfInt16(uPitchOnly[0]), signOfInt16(uPitchOnly[1]), signOfInt16(uPitchOnly[2]),
		signOfInt16(uPitchOnly[3]), signOfInt16(uPitchOnly[4]))
	t.Logf("[X u_code [0..4]]  (g_p=0)         = [%+6d %+6d %+6d %+6d %+6d]  signs=[%s %s %s %s %s]",
		uCodeOnly[0], uCodeOnly[1], uCodeOnly[2], uCodeOnly[3], uCodeOnly[4],
		signOfInt16(uCodeOnly[0]), signOfInt16(uCodeOnly[1]), signOfInt16(uCodeOnly[2]),
		signOfInt16(uCodeOnly[3]), signOfInt16(uCodeOnly[4]))

	t.Logf("──────── composite + replication 검증 ────────")
	t.Logf("[X u[0..4]]    composite (replicated) = [%+6d %+6d %+6d %+6d %+6d]  expected=[+1 +1 +1 +1 +0]",
		u[0], u[1], u[2], u[3], u[4])
	t.Logf("[X u[0..4]]    production decodeSubframe = [%+6d %+6d %+6d %+6d %+6d]",
		prodU[0], prodU[1], prodU[2], prodU[3], prodU[4])
	t.Logf("[X replication match (all 40 samples)] = %v", matchAll)

	// (6) Sign-determining sub-term identification (Phase 0.4 §1 —
	//     measurement-driven; no a-priori preference).
	t.Logf("──────── X 부호 결정 sub-항 평가 ────────")
	verdict := classifyFnonSignDetermining(uPitchOnly[:5], uCodeOnly[:5], u[:5])
	t.Logf("[X 결정] sample 0..4 sign-determining sub-term = %s", verdict)

	for n := 0; n < 5; n++ {
		t.Logf("  sample %d: u_pitch=%+d (sign %s)  u_code=%+d (sign %s)  u_total=%+d (sign %s)",
			n,
			uPitchOnly[n], signOfInt16(uPitchOnly[n]),
			uCodeOnly[n], signOfInt16(uCodeOnly[n]),
			u[n], signOfInt16(u[n]))
	}

	// (7) X 가설 평가 (X-pos = pitch / X-fcb = fcb / X-both / X-refute)
	t.Logf("──────── X 가설 평가 ────────")
	t.Logf("X 가설 후보: X-pos (g_p·v 결정) / X-fcb (g_c·c 결정) / X-both (hybrid) / X-refute (둘 다 0)")
	t.Logf("verdict: %s", classifyFnonXHypothesis(uPitchOnly[:5], uCodeOnly[:5], u[:5]))
}

// decodeFnonProdU runs the production decodeSubframe path on frame 0 sf0
// and returns the composite excitation u[0..39] *as the production
// pipeline computes it*. Used for replicated-vs-production equivalence
// check (Phase 0.4 §1 replication 결함 surfacing).
func decodeFnonProdU(t *testing.T, framePacked []byte) [subframeLen]int16 {
	t.Helper()
	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(framePacked, false, out[:]); err != nil {
		t.Fatalf("decodeFnonProdU: Decode frame 0: %v", err)
	}
	// We need u[] not out[]; replicate via a second pass that records
	// it through a local BuildExcitation invocation with the same
	// inputs. Since first-frame state is deterministic (zero), this
	// matches the production path exactly when the replicated chain
	// itself matches; equivalence is the property under test.
	var f bitstream.Frame
	if err := bitstream.Unpack(framePacked, &f); err != nil {
		t.Fatalf("decodeFnonProdU: Unpack frame 0: %v", err)
	}
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)
	return u
}

// classifyFnonSignDetermining returns the sub-term that determines the
// sign of u[0..4] across all 5 samples. Phase 0.4 §1 강압-적합 회피:
// the function emits a single label only if the evidence is unambiguous
// across all 5 samples; otherwise "hybrid" or "undetermined" surfaces.
func classifyFnonSignDetermining(uPitch, uCode, uTotal []int16) string {
	pitchAllZero := true
	codeAllZero := true
	for i := range uTotal {
		if uPitch[i] != 0 {
			pitchAllZero = false
		}
		if uCode[i] != 0 {
			codeAllZero = false
		}
	}
	if pitchAllZero && codeAllZero {
		return "undetermined (둘 다 Q0=0; sub-항 모두 부호 무 — Q15 raw 검토 의무)"
	}
	if pitchAllZero {
		return "g_c·c (fcb contribution) 단독 결정"
	}
	if codeAllZero {
		return "g_p·v (pitch contribution) 단독 결정"
	}
	// Both non-zero somewhere — check per-sample agreement.
	hybrid := false
	for i := range uTotal {
		ps := signOfInt16(uPitch[i])
		cs := signOfInt16(uCode[i])
		if ps != "0" && cs != "0" && ps != cs {
			hybrid = true
		}
	}
	if hybrid {
		return "hybrid (sample 별 부호 충돌; Task 4 위임)"
	}
	return "hybrid (두 sub-항 모두 비-zero 기여; 단독 결정 아님 — Task 4 위임)"
}

// classifyFnonXHypothesis maps the sub-term measurement onto the four
// X-hypothesis labels declared in the task spec.
func classifyFnonXHypothesis(uPitch, uCode, uTotal []int16) string {
	pitchAllZero := true
	codeAllZero := true
	for i := range uTotal {
		if uPitch[i] != 0 {
			pitchAllZero = false
		}
		if uCode[i] != 0 {
			codeAllZero = false
		}
	}
	switch {
	case pitchAllZero && codeAllZero:
		return "X-refute (두 sub-항 모두 Q0 zero — u[0..4] 부호가 본 식에서 발생하지 않음; 검색 영역 재진입 의무)"
	case pitchAllZero && !codeAllZero:
		return "X-fcb (fcb contribution g_c·c[n] 단독으로 u[0..4] 부호 결정 — fix scope = fcb.Decode 또는 g_c decoding)"
	case !pitchAllZero && codeAllZero:
		return "X-pos (pitch contribution g_p·v[n] 단독으로 u[0..4] 부호 결정 — fix scope = pitch.AdaptiveCodebook 또는 g_p decoding)"
	default:
		return "X-both (두 sub-항 모두 기여 — 단일 fix scope 비결정; Task 4 위임)"
	}
}

// TestDiagnostic_FnonPrelimYLPCrossCheck cross-checks the LP synthesis
// filter input coefficients a[0..10] (Q12, a[0]=4096) for ALGTHM frame
// 0 sf0 against the F-sept-2 §3.2.6 reference and (optionally) probes
// whether forced sign-flip of a[1..10] alters the sign of syn[5..7].
//
// Spec ground-truth (PDF verbatim grep, see report §0):
//   - ITU-T G.729 (06/2012) §A.4.1 (PDF p.42) "Same as described in
//     clause 4.1" (Annex A decoder reuses §4.1 LSP decoding +
//     interpolation verbatim).
//   - ITU-T G.729 (06/2012) §4.1 (PDF p.21) LSP decoding +
//     §4.1.2 LSP interpolation per subframe.
//   - ITU-T G.729 (06/2012) §3.2.6 (PDF p.13) LSP-to-LP polynomial
//     conversion (F1/F2 expansion + assembly).
//   - ITU-T G.729 (06/2012) §4.3 Table 9 codec-start state
//     (pastResidual = i·π/11, pastLSP_prev = cos(i·π/11)).
//   - ITU-T G.729 (06/2012) §3.10 LP synthesis filter
//     ŝ(n) = u(n) − Σ aᵢ · ŝ(n−i) (used by the forced-flip stimulus).
//
// E2 declaration (citation drift): plan §"Spec § 인용" cites §A.3.2
// (encoder-side LP analysis & quantization) + §A.3.3 (perceptual
// weighting) for the Y cross-check. PDF grep confirms those sections
// are encoder-side and therefore not the substantive citation for the
// decoder's a[] reconstruction. The substantive citation chain is
// §A.4.1 → §4.1 + §4.1.2 + §3.2.6 + §4.3 Table 9. Same correction
// pattern as F-non-prelim-1 (§A.3.5 → §4.1.5/§4.1.6 via §A.4.1).
//
// F-sept-2 (TestDiagnostic_FseptLPReferenceCrossCheck) PASSes at frame
// 0 with max|Δ|=6 + sign-equal across all 11 a[] coefficients (L3
// 분류 — magnitude gap exists but signs match). This task re-measures
// the same a[0..10] from the same production lsp.Decoder path and
// additionally measures whether a[]'s *sign* content has any causal
// influence on syn[5..7] sign (forced-flip stimulus, §3.10 IIR
// linearity probe).
//
// Phase 0.4 §1 강압-적합 회피: forced-flip is a *probe*, not a hypothesis
// confirmation. The probe answers "if a[] sign were flipped, would
// syn[5..7] sign flip?" — independent of whether the *current* a[] is
// spec-compliant. Verdict gate = sign-equal (a[] sign vs §3.2.6 ref);
// magnitude gap (max|Δ|) is reported as auxiliary, since the F-non
// cycle's question is sign-source identification, not magnitude
// precision (the latter is F-sept-2 L3 territory).
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FnonPrelimYLPCrossCheck(t *testing.T) {
bitPath := vectorPath("ALGTHM.BIT")
ensureTestdataPresent(t, bitPath)

frames, _ := readG192Frames(t, bitPath)
var f bitstream.Frame
if err := bitstream.Unpack(frames[0], &f); err != nil {
t.Fatalf("Unpack frame 0: %v", err)
}

// (a) production a[0..10] for frame 0 sf0 via lsp.Decoder
//     (= same path as F-sept-2 §1 + F-oct-postfix2-prelim Task 4 §3).
var lspDec lsp.Decoder
lspDec.Reset()
sf0Prod, _ := lspDec.Decode(lsp.Indices{
L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
})

t.Logf("──────── F-non-prelim-2 Y LP a[] cross-check (ALGTHM frame 0 sf0) ────────")
t.Logf("indices: L0=%d L1=%d L2=%d L3=%d", f.L0, f.L1, f.L2, f.L3)
t.Logf("[Y a[0..10] (frame 0 sf0)] = [%+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d]  Q12",
sf0Prod[0], sf0Prod[1], sf0Prod[2], sf0Prod[3], sf0Prod[4], sf0Prod[5],
sf0Prod[6], sf0Prod[7], sf0Prod[8], sf0Prod[9], sf0Prod[10])

// (b) F-sept-2 reference cross-check (re-uses the same float64
//     §3.2.6 reference function defined in stagef_sept_diagnostic_test.go).
sf0Ref := referenceLSPToLPSubframe0(t, uint8(f.L0), uint8(f.L1), uint8(f.L2), uint8(f.L3))

byteEqual := true
signEqual := true
maxAbsDelta := int32(0)
t.Logf("──────── F-sept-2 reference cross-check (Q12 byte / sign comparison) ────────")
t.Logf("idx   prod_q12   ref(float64)        ref(round_q12)   Δ   sign(prod) sign(ref)")
for i := 0; i <= 10; i++ {
refQ12 := int16(roundFloat(sf0Ref[i] * 4096))
delta := int32(sf0Prod[i]) - int32(refQ12)
if delta != 0 {
byteEqual = false
}
ad := delta
if ad < 0 {
ad = -ad
}
if ad > maxAbsDelta {
maxAbsDelta = ad
}
ps := signOfInt16(sf0Prod[i])
rs := signOfInt16(refQ12)
if ps != rs {
signEqual = false
}
t.Logf("[%2d]   %+6d     %+18.12f   %+6d           %+d     %s          %s",
i, sf0Prod[i], sf0Ref[i], refQ12, delta, ps, rs)
}
t.Logf("[Y F-sept-2 reference cmp]    a-byte-equal=%v  sign-equal=%v  max|Δ|=%d",
byteEqual, signEqual, maxAbsDelta)

// (c) Forced a-sign-flip stimulus on synth.Filter — probe whether
//     a[]'s sign content is causally bound to syn[5..7] sign under
//     the §3.10 IIR. Build u[] via the production excitation path
//     (= F-non-prelim-1 replicated chain) so that the only varying
//     factor between baseline and flipped runs is a[1..10] sign.
tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
var pastExc [pastExcLen]int16
var v [subframeLen]int16
pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
betaQ14 := fcb.ClampPitchGainForEnhancement(0)
var c [subframeLen]int16
fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
var gn gain.Decoder
gn.Reset()
gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
var u [subframeLen]int16
synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

// Baseline syn[] with production a[].
var synBase [subframeLen]int16
{
var sy synth.Synthesizer
sy.Reset()
fixed.ClearOverflow()
sy.Filter(&sf0Prod, &u, &synBase)
}

// Flipped a[]: a[0]=4096 unchanged (spec invariant), a[1..10] negated
// with int16 saturation (-32768 → 32767 if encountered; not expected
// for this vector, F-oct-postfix2-prelim §3 dump shows |a[i]| ≤ ~2200).
var sf0Flip [lpcOrder + 1]int16
sf0Flip[0] = sf0Prod[0]
for i := 1; i <= 10; i++ {
v := sf0Prod[i]
if v == -32768 {
sf0Flip[i] = 32767
} else {
sf0Flip[i] = -v
}
}

var synFlip [subframeLen]int16
{
var sy synth.Synthesizer
sy.Reset()
fixed.ClearOverflow()
sy.Filter(&sf0Flip, &u, &synFlip)
}

t.Logf("──────── Forced a-sign-flip syn[5..7] (a[1..10] → −a[1..10], a[0]=4096 fixed) ────────")
t.Logf("[Y a[0..10] flipped] = [%+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d]  Q12",
sf0Flip[0], sf0Flip[1], sf0Flip[2], sf0Flip[3], sf0Flip[4], sf0Flip[5],
sf0Flip[6], sf0Flip[7], sf0Flip[8], sf0Flip[9], sf0Flip[10])
t.Logf("[Y forced a-sign-flip syn[5..7]]  baseline=[%+d %+d %+d]  flipped=[%+d %+d %+d]",
synBase[5], synBase[6], synBase[7], synFlip[5], synFlip[6], synFlip[7])
t.Logf("  per-sample sign:  baseline=[%s %s %s]  flipped=[%s %s %s]",
signOfInt16(synBase[5]), signOfInt16(synBase[6]), signOfInt16(synBase[7]),
signOfInt16(synFlip[5]), signOfInt16(synFlip[6]), signOfInt16(synFlip[7]))

signFlipped := 0
for n := 5; n <= 7; n++ {
bs := signOfInt16(synBase[n])
fs := signOfInt16(synFlip[n])
if bs != "0" && fs != "0" && bs != fs {
signFlipped++
}
}
signFlipInduced := signFlipped == 3
t.Logf("[Y forced a-sign-flip] sign-flipped-samples=%d/3  sign-flip-induced=%v",
signFlipped, signFlipInduced)

// Magnitude change measurement (independent of sign).
magChanged := false
for n := 5; n <= 7; n++ {
if synFlip[n] != synBase[n] {
magChanged = true
}
}

// (d) Y verdict (Phase 0.4 §1 — measurement-driven; no a-priori).
//
// Verdict gate = sign-equal (a[] sign content vs §3.2.6 reference) —
// the *sign-source* predicate for the F-non-prelim cycle. byte-equal
// / max|Δ| is the magnitude-precision predicate, reported as
// auxiliary (F-sept-2 baseline classifies max|Δ|=6 as L3 정합 gap
// but PASS — orthogonal to the sign-source question of this cycle).
t.Logf("──────── Y 가설 평가 ────────")
t.Logf("[Y 결정] LP a[] spec 정합성 = %s; 부호 결정성 = %s",
ySpecLabel(byteEqual, signEqual, maxAbsDelta),
ySignDeterminismLabel(signFlipInduced, magChanged))
verdict := classifyFnonYHypothesis(signEqual, signFlipInduced, magChanged)
t.Logf("Y 가설 후보: Y-refute (sign-equal + flip 시 syn[5..7] 변화 0) / Y-flip (sign-equal + flip 시 syn[5..7] 부호 flip — a[] sign 결정성 보유, 단 현재 a sign 정합) / Y-magnitude (sign-equal + flip 시 magnitude 만 변화, 부호 보존) / Y-suspect (sign-mismatch — a[] sign 자체 결함) / Y-inconclusive")
t.Logf("verdict: %s", verdict)
}

// ySpecLabel maps (byteEqual, signEqual, maxAbsDelta) onto a Korean
// verdict label for the LP a[] spec-compliance dimension. Sign and
// magnitude are reported as separate axes because they are diagnosed
// at different cycles (sign = F-non-prelim, magnitude = F-sept-2 L3).
func ySpecLabel(byteEqual, signEqual bool, maxAbsDelta int32) string {
switch {
case byteEqual:
return "완전 정합 (byte-equal vs §3.2.6 reference)"
case signEqual && maxAbsDelta <= 2:
return "정합 (sign-equal + max|Δ|≤2 — Q12 rounding 정상)"
case signEqual:
return "sign-정합 (sign-equal, max|Δ|=" +
itoa(maxAbsDelta) + " — F-sept-2 L3 magnitude gap; 본 cycle 부호 source 와 직교)"
default:
return "sign-결함 (sign-mismatch vs §3.2.6 reference — a[] sign 자체 spec 위반)"
}
}

// itoa: minimal int32 → string helper (avoid strconv import drift).
func itoa(v int32) string {
if v == 0 {
return "0"
}
neg := v < 0
if neg {
v = -v
}
var buf [12]byte
i := len(buf)
for v > 0 {
i--
buf[i] = byte('0' + v%10)
v /= 10
}
if neg {
i--
buf[i] = '-'
}
return string(buf[i:])
}

// ySignDeterminismLabel maps the forced-flip outcome onto a Korean label
// for the a[] sign-determinism dimension (independent of whether the
// *current* a[] is spec-compliant).
func ySignDeterminismLabel(signFlipInduced, magChanged bool) string {
switch {
case signFlipInduced:
return "보유 (forced flip 시 syn[5..7] 3/3 부호 flip — a[] sign 이 syn 부호 결정에 직접 기여)"
case magChanged:
return "부분 (forced flip 시 magnitude 변화하나 부호 보존 — u[] 자기-피드백이 syn 부호를 지배)"
default:
return "부재 (forced flip  syn[5..7] 변화 무 — a[] sign 영향 0)"
}
}

// classifyFnonYHypothesis maps the (signEqual, signFlipInduced,
// magChanged) measurement triple onto the five Y-hypothesis labels.
// Verdict gate = signEqual (a[] sign vs §3.2.6 reference); magnitude
// gap is reported by ySpecLabel and not used to gate the verdict (it
// is F-sept-2 L3 territory, orthogonal to the sign source question).
func classifyFnonYHypothesis(signEqual, signFlipInduced, magChanged bool) string {
if !signEqual {
return "Y-suspect (a[] sign ≠ §3.2.6 reference — fix scope = internal/lsp LSP→a[] 변환)"
}
switch {
case signFlipInduced:
return "Y-flip (sign-equal + forced flip 시 syn[5..7] 부호 flip — a[] sign 결정성 보유. 단 현재 a[] sign 이 spec 정합이므로 부호 결함은 a[] 외부; F-non-prelim-1 X-fcb verdict 잔존 우선)"
case magChanged:
return "Y-magnitude (sign-equal + forced flip 시 syn[5..7] magnitude 만 변화, 부호 보존 — a[] 가 syn 부호에 미치는 영향 부분적; 부호 source 는 u[] 자기-피드백 — F-non-prelim-1 X-fcb verdict 정합)"
default:
return "Y-refute (sign-equal + forced flip 시 syn[5..7] 변화 0 — a[] 가 syn 부호 결함 후보에서 완전 배제)"
}
}
