package postfilter

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FnonCgammaRevisit1PostfilterSubStageTrace — Phase 1k
// Stage F-non-Cgamma-revisit-1 (Task 1, G-1 postfilter sub-stage trace).
//
// Plan: docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-plan.md
// (commit c743116, Phase 2 §Task 1).
//
// 측정 의무 (1줄): §4.2.1/2/3/4/5 4 sub-stage 의 sample 5..7 한정
// 출력 4-tuple + sub-stage 별 spec 정합 EQ/NE 판정.
//
// 선행 측정 보완: F-oct-postfix2-prelim-4 (commit f04ec88) — sPf[5..7]
// (postfilter chain 최종 출력) 만 측정, 4 sub-stage 분리 출력 부재.
// 본 task = sub-stage 별 (long-term / short-term / tilt / AGC + HP)
// 분리 + spec 정합 EQ/NE 판정.
//
// Spec polarity expectation (PDF verbatim 인용 — ITU-T G.729 (06/2012)):
//
//	§4.2.1 Long-term postfilter, p.28 line 1565..1572 (eq 78):
//	  "Hp(z) = (1/(1+γp·gl)) · (1 + γp·gl·z^-T)"
//	  "gl is bounded by 1, and it is set to zero if the long-term
//	  prediction gain is less than 3 dB."
//	  → Hp(z) = (1/(1+γp·gl))·1  + (γp·gl/(1+γp·gl))·z^-T  ;  γp=0.5.
//	  coefficients g0 = 1/(1+γp·gl), g1 = γp·gl/(1+γp·gl) — both ≥ 0
//	  for any 0 ≤ gl ≤ 1.  Polarity preserve when r̂(n) and r̂(n-T)
//	  share sign (or when gl=0 → output = r̂(n) exactly).
//
//	§4.2.2 Short-term postfilter, p.28 line 1626..1644 (eq 84):
//	  "Hf(z) = (1/gf) · (Â(z/γn)/Â(z/γd))"
//	  IIR with LP-derived denominator coefficients aˆ_i (γn=0.55,
//	  γd=0.7). Polarity preserve under stable IIR with sign-equal
//	  coefficients (F-non-prelim-1: a[0..10] sign 11/11 == reference).
//
//	§4.2.3 Tilt compensation, p.29 line 1646..1667 (eq 86, 87):
//	  "Ht(z) = (1/gt)·(1 + γt·k1' · z^-1)"
//	  "Two values for γt are used depending on the sign of k1'.
//	  If k1' is negative, γt = 0.9, and if k1' is positive, γt = 0.2."
//	  One-tap FIR: s_tilt(n) = s_st(n) + (γt·k1') · s_st(n-1).
//	  Polarity preserve when s_st sample 5..7 dominate over μ·s_st(n-1)
//	  contribution (μ = γt·k1', |μ| < 1 typical).
//
//	§4.2.4 Adaptive gain control, p.29 line 1669..1686 (eq 88, 89, 90):
//	  "G = Σ ŝ(n) / Σ sf(n)"   (main spec; Annex A §A.4.2.4 = Σŝ²/Σsf²)
//	  "sf'(n) = g(n)·sf(n)"
//	  "g(n) = 0.85·g(n-1) + 0.15·G"   (main spec; Annex A = 0.9/0.1)
//	  g(n) is a positive scalar gain → strict polarity preserve.
//
//	§4.2.5 High-pass filtering and upscaling, p.29 line 1687..1693
//	  (eq 91):
//	  "H_h2(z) = (0.93980581 − 1.8795834·z^-1 + 0.93980581·z^-2) /
//	             (1 − 1.9330735·z^-1 + 0.93589199·z^-2)"
//	  "The filtered signal is multiplied by a factor 2 to restore the
//	  input signal level."
//	  Linear-phase 2-pole 2-zero IIR + ×2 scale. Polarity preserve at
//	  steady-state DC-removed input; at sample 0..7 (first samples
//	  after zero state init) impulse response is +b0·×2 ≈ +1.879
//	  applied to first input → first sample sign = sign(input[0]).
//
// 종합 polarity expectation: syn[5..7]=[+1,+1,+1] (F-non-prelim-1
// 측정값) + 4 sub-stage 모두 polarity preserve (spec 인용 위) →
// 4 sub-stage 출력 부호 sample 5..7 = "+" 기대.
//
// 강압-적합 회피 (Phase 0.4): EQ = 출력 부호 = "+" (spec 정합),
// NE = 출력 부호 = "-" (spec 외부 mechanism 식별 후보).
// "0" = degenerate (sub-stage 출력이 0; sample magnitude 미달 →
// inconclusive, spec polarity preserve 와 모순 아님).
//
// Production 변경 0 (E5/E2). assertion 0 (측정-only).
// 외부 G.729 구현 0 참조 (E1/E4): spec source = ITU-T G.729 (06/2012)
// PDF verbatim 인용 only (위 §4.2.* paragraph). Annex A binary 0 사용.
func TestDiagnostic_FnonCgammaRevisit1PostfilterSubStageTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) sf0 LP coefficients (Q12, a[0]=4096) per §4.1.
	var lspDec lsp.Decoder
	lspDec.Reset()
	sfA, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	// (2) Excitation u[] = g_p·v + g_c·c per §3.8/3.9.
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	const pastExcLen = 153 // pitchMax(143) + 10
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcMant_gcQ12, gcExp_gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)

	// (3) Synth IIR — pre-postfilter input s[] per §4.1.6.
	var syn synth.Synthesizer
	syn.Reset()
	var s [subframeLen]int16
	syn.Filter(&sfA, &u, &s)

	t.Logf("──────── F-non-Cgamma-revisit-1 fixture (ALGTHM frame 0 sf0) ────────")
	t.Logf("PST want sample 5..7              = [%+d %+d %+d]   signs=[%s %s %s]",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7],
		signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))
	t.Logf("synth IIR s[5..7] (postfilter in) = [%+d %+d %+d]   signs=[%s %s %s]",
		s[5], s[6], s[7],
		signOfInt16(s[5]), signOfInt16(s[6]), signOfInt16(s[7]))

	// (4) Postfilter chain — replicate Postfilter.Filter() with per-
	//     sub-stage capture. Mirrors stagef_octpostfix2_prelim_diagnostic_test.go
	//     M1' chain replication (production unchanged).
	var pf Postfilter // zero value = codec-start

	var aNum, aDen [11]int16
	expandBandwidth(&sfA, gammaNumQ15, &aNum)
	expandBandwidth(&sfA, gammaDenQ15, &aDen)

	var r [subframeLen]int16
	pf.computeResidual(&aNum, &s, &r)

	// Past-residual update — exactly mirrors postfilter.Filter().
	copy(pf.pastResidual[:pitchMax], pf.pastResidual[subframeLen:])
	copy(pf.pastResidual[pitchMax:], r[:])

	T := pf.refinePitch(&r, tInt)

	// (a) §4.2.1 long-term postfilter output (lt[]).
	g0LT, g1LT := pf.computeLongTermGain(&r, T)
	ltBranch := "active(g_l>0)"
	if g1LT == 0 {
		ltBranch = "inactive(g_l=0; r̂·r̂_T ≤ 0 or E=0)"
	}
	var lt [subframeLen]int16
	pf.applyLongTerm(&r, T, &lt)

	// (b) §4.2.2 short-term postfilter output (st[]).
	var st [subframeLen]int16
	pf.applyShortTerm(&aDen, &lt, &st)

	// (c) §4.2.3 tilt compensation output (tc[]). γ_t branch detection:
	//     production gates on agcGainPrev (codec-start = 0 → inactive,
	//     γ_t=3277=0.2). Spec §4.2.3 gates on sign(k1'); we record both.
	tiltBranchProd := "active(γ_t=14746=0.9)"
	tiltGammaQ14 := gammaTiltActiveQ14
	if pf.agcGainPrev == 0 {
		tiltBranchProd = "inactive(γ_t=3277=0.2)"
		tiltGammaQ14 = gammaTiltInactiveQ14
	}
	muQ15 := pf.computeTiltMu(&aNum, &aDen)
	// Spec gating on sign(k1'): spec §4.2.3 "If k1' is negative,
	// γt = 0.9, and if k1' is positive, γt = 0.2."  Reproduce k1'
	// sign for spec-vs-production gating verdict.
	var hh [tiltLen]int32
	for n := 0; n < tiltLen; n++ {
		var hNum int32
		switch {
		case n == 0:
			hNum = int32(aNum[0])
		case n <= lpcOrder:
			hNum = int32(aNum[n])
		default:
			hNum = 0
		}
		acc := hNum << 12
		for k := 1; k <= lpcOrder && k <= n; k++ {
			acc -= int32(aDen[k]) * hh[n-k]
		}
		hh[n] = (acc + (1 << 11)) >> 12
	}
	var rh0, rh1 int64
	for n := 0; n < tiltLen; n++ {
		rh0 += int64(hh[n]) * int64(hh[n])
	}
	for n := 0; n < tiltLen-1; n++ {
		rh1 += int64(hh[n]) * int64(hh[n+1])
	}
	k1Sign := "0"
	if rh0 > 0 {
		k1 := -(rh1 << 15) / rh0
		switch {
		case k1 > 0:
			k1Sign = "+"
		case k1 < 0:
			k1Sign = "−"
		}
	}
	tiltBranchSpec := "γt=0.2 (k1'>0)"
	if k1Sign == "−" {
		tiltBranchSpec = "γt=0.9 (k1'<0)"
	} else if k1Sign == "0" {
		tiltBranchSpec = "γt=0   (k1'=0)"
	}
	var tc [subframeLen]int16
	pf.applyTiltWithMu(&st, muQ15, &tc)

	// (d) §4.2.4 AGC output (agc[]) — gain-scaled postfiltered signal.
	gTarget := pf.computeAGCTargetGain(&s, &tc)
	agcInitBefore := pf.initialized
	var agcOut [subframeLen]int16
	pf.applyAGC(&tc, gTarget, &agcOut)
	agcBranch := "steady-state"
	if !agcInitBefore {
		agcBranch = "init-seed(g(-1) ← g_target per §4.2.4 init)"
	}

	// (e) §4.2.5 HP filter + ×2 upscale (hp[]).
	//     Mirrors internal/decoder/hpfilter.go (§4.2.5 / §A.4.2.5).
	//     PDF verbatim coefficients (eq 91):
	//       b0=+0.93980581, b1=-1.8795834, b2=+0.93980581
	//       a1=-1.9330735,  a2=+0.93589199
	//     Fixed-point quantization (mirrors decoder/hpfilter.go):
	//       b0/b1/b2 at Q13; -a1 at Q12 (|a1|>1); a2 at Q13.
	//     Zero state (codec-start; ITU-T §4.3 "All static encoder and
	//     decoder variables should be initialized to zero").
	const (
		hpB0Q13    int32 = 7699
		hpB1Q13    int32 = -15399
		hpB2Q13    int32 = 7699
		hpNegA1Q12 int32 = 7918
		hpA2Q13    int32 = 7667
	)
	var (
		hpX1, hpX2 int16
		hpY1, hpY2 int32
	)
	var hp [subframeLen]int16
	for n := 0; n < subframeLen; n++ {
		xn := agcOut[n]

		ff := hpB0Q13*int32(xn) + hpB1Q13*int32(hpX1) + hpB2Q13*int32(hpX2) // Q13
		ff >>= 1                                                            // Q12

		fb := int64(hpNegA1Q12) * int64(hpY1) // Q24
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(hpY2)) >> 13

		acc := int64(ff) + fb // Q12
		yn := (acc + (1 << 11)) >> 12
		if yn > 32767 {
			yn = 32767
		} else if yn < -32768 {
			yn = -32768
		}
		hp[n] = int16(yn)

		hpX2 = hpX1
		hpX1 = xn
		hpY2 = hpY1
		hpY1 = int32(acc)
	}
	// Note: ×2 upscale (eq 91 last sentence) is applied by the
	// downstream pcm.ScaleUpSat stage; the §4.2.5 sub-stage output is
	// hp[] (pre-upscale). polarity == sign(hp[n]) since ×2 is sign-
	// preserving. We log both pre-upscale hp[] and the ×2-scaled hpX2[]
	// for completeness.
	var hpX2Out [3]int32
	for i, n := range []int{5, 6, 7} {
		hpX2Out[i] = int32(hp[n]) << 1
	}

	// (5) Cross-check: replicated chain == production Postfilter.Filter.
	var pfRef Postfilter
	var sPfRef [subframeLen]int16
	pfRef.Filter(&sfA, tInt, &s, &sPfRef)
	matchAll := true
	for n := 0; n < subframeLen; n++ {
		if agcOut[n] != sPfRef[n] {
			matchAll = false
			break
		}
	}

	t.Logf("──────── per sub-stage sample 5..7 출력 ────────")
	t.Logf("residual r[5..7]               = [%+d %+d %+d]   signs=[%s %s %s]",
		r[5], r[6], r[7],
		signOfInt16(r[5]), signOfInt16(r[6]), signOfInt16(r[7]))
	t.Logf("refinePitch T=%d (tInt=%d, tFrac=%d)", T, tInt, tFrac)

	t.Logf("(a) §4.2.1 long-term  lt[5..7]  = [%+d %+d %+d]   signs=[%s %s %s]   branch=%s  g0=%d g1=%d (Q14)",
		lt[5], lt[6], lt[7],
		signOfInt16(lt[5]), signOfInt16(lt[6]), signOfInt16(lt[7]),
		ltBranch, g0LT, g1LT)
	t.Logf("(b) §4.2.2 short-term st[5..7]  = [%+d %+d %+d]   signs=[%s %s %s]   IIR aDen[1..10]=%v",
		st[5], st[6], st[7],
		signOfInt16(st[5]), signOfInt16(st[6]), signOfInt16(st[7]),
		aDen[1:11])
	t.Logf("(c) §4.2.3 tilt       tc[5..7]  = [%+d %+d %+d]   signs=[%s %s %s]   prod-branch=%s  spec-branch=%s  μ_Q15=%d  k1'sign=%s",
		tc[5], tc[6], tc[7],
		signOfInt16(tc[5]), signOfInt16(tc[6]), signOfInt16(tc[7]),
		tiltBranchProd, tiltBranchSpec, muQ15, k1Sign)
	_ = tiltGammaQ14 // documented in branch label; unused beyond log.
	t.Logf("(d) §4.2.4 AGC        agc[5..7] = [%+d %+d %+d]   signs=[%s %s %s]   branch=%s  g_target_Q14=%d",
		agcOut[5], agcOut[6], agcOut[7],
		signOfInt16(agcOut[5]), signOfInt16(agcOut[6]), signOfInt16(agcOut[7]),
		agcBranch, gTarget)
	t.Logf("(e) §4.2.5 HP+×2      hp[5..7]  = [%+d %+d %+d]   signs=[%s %s %s]   ×2_scaled=[%+d %+d %+d]",
		hp[5], hp[6], hp[7],
		signOfInt16(hp[5]), signOfInt16(hp[6]), signOfInt16(hp[7]),
		hpX2Out[0], hpX2Out[1], hpX2Out[2])

	t.Logf("replication chain == production Postfilter.Filter ? %v (sub-stage agcOut == sPf)", matchAll)
	if !matchAll {
		t.Logf("WARNING: chain replication mismatch — measurement still valid (per-stage capture only).")
	}

	// (6) Sub-stage classifier — sample-5..7 sign vs spec polarity
	//     expectation (= "+", since syn[5..7]=[+,+,+] and PST chain is
	//     polarity-preserving per spec §4.2.* quotes above).
	type subStageResult struct {
		label      string
		sec        string
		v5, v6, v7 int16
	}
	results := []subStageResult{
		{"long-term", "§4.2.1", lt[5], lt[6], lt[7]},
		{"short-term", "§4.2.2", st[5], st[6], st[7]},
		{"tilt", "§4.2.3", tc[5], tc[6], tc[7]},
		{"AGC+HP", "§4.2.4+§4.2.5", agcOut[5], agcOut[6], agcOut[7]},
	}
	expectedSign := signOfInt16(s[5]) // "+" for ALGTHM frame 0 sf0.

	t.Logf("──────── sub-stage EQ/NE 판정 (expected sign=%s, spec polarity-preserve) ────────", expectedSign)
	overallVerdict := "EQ_ALL"
	flipDetected := false
	for _, rs := range results {
		verdict := classifyCgammaPostfilterSubStage(expectedSign, rs.v5, rs.v6, rs.v7)
		t.Logf("  %-10s (%s) signs=[%s %s %s]  verdict=%s",
			rs.label, rs.sec,
			signOfInt16(rs.v5), signOfInt16(rs.v6), signOfInt16(rs.v7),
			verdict)
		switch verdict {
		case "NE":
			overallVerdict = "NE_AT_LEAST_ONE"
			flipDetected = true
		case "INCONCLUSIVE":
			if overallVerdict == "EQ_ALL" {
				overallVerdict = "INCONCLUSIVE"
			}
		}
	}
	// HP sub-stage (e) is part of §4.2.5; report independently for
	// completeness since plan §Task 1 enumerates 4 sub-stage but
	// distinguishes (d)=AGC and (e)=HP within §4.2.5.
	hpVerdict := classifyCgammaPostfilterSubStage(expectedSign, hp[5], hp[6], hp[7])
	t.Logf("  %-10s (%s) signs=[%s %s %s]  verdict=%s   (sub-substage; AGC+HP combined verdict above used for G-1)",
		"HP", "§4.2.5",
		signOfInt16(hp[5]), signOfInt16(hp[6]), signOfInt16(hp[7]),
		hpVerdict)
	if hpVerdict == "NE" {
		flipDetected = true
		overallVerdict = "NE_AT_LEAST_ONE"
	}

	// (7) G-1 hypothesis evaluation.
	t.Logf("──────── G-1 hypothesis evaluation ────────")
	t.Logf("postfilter chain input  s[5..7] signs = [%s %s %s]",
		signOfInt16(s[5]), signOfInt16(s[6]), signOfInt16(s[7]))
	t.Logf("postfilter chain output sPf[5..7] signs = [%s %s %s]",
		signOfInt16(agcOut[5]), signOfInt16(agcOut[6]), signOfInt16(agcOut[7]))
	t.Logf("PST want sample 5..7 signs = [%s %s %s]",
		signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))
	t.Logf("4 sub-stage overall verdict = %s   (flip in chain detected? %v)",
		overallVerdict, flipDetected)

	switch overallVerdict {
	case "NE_AT_LEAST_ONE":
		t.Logf("[G-1 verdict] (Cγ-postfilter) TRIGGER — postfilter chain 내 ≥1 sub-stage 에서 spec polarity-preserve 위반. mechanism 후보 식별. → F-non-fix-postfilter cycle 진입 권고 (plan §Task 3 (Cγ-postfilter)).")
	case "EQ_ALL":
		t.Logf("[G-1 verdict] (Cγ-postfilter) REFUTE — postfilter 4 sub-stage 모두 spec polarity-preserve 정합 (sample 5..7 부호=expected). G-1 폐기. 잔여 mechanism 후보 = G-2 (synth IIR memory + Y magnitude). → F-non-Cgamma-revisit-2 (Task 2) 진입 권고.")
	default:
		t.Logf("[G-1 verdict] INCONCLUSIVE — sub-stage 출력에 0 등 degenerate sample 포함. spec polarity-preserve 와 모순 아님. → Task 2 진입 후 G-2 verdict 와 결합 평가.")
	}
}

// classifyCgammaPostfilterSubStage returns the EQ/NE binary verdict for
// a postfilter sub-stage's sample 5..7 output vs the spec-derived
// expected sign (per §4.2.* polarity-preserve property).
//
// Verdict semantics (Phase 0.4 강압-적합 회피, plan §0.4):
//
//	"EQ"           — sample 5..7 모두 expected sign (spec 정합).
//	"NE"           — ≥1 sample 이 expected sign 의 반대 부호
//	                 (spec polarity-preserve 위반, mechanism 후보).
//	"INCONCLUSIVE" — 0 sample 포함 + 나머지 EQ (degenerate, spec
//	                 와 모순 아님; magnitude-bound 미달).
//
// "거의 정합" / "범위 내 변동" 등 모호 verdict 금지 (plan §0.4 ③).
func classifyCgammaPostfilterSubStage(expectedSign string, v5, v6, v7 int16) string {
	signs := [3]string{signOfInt16(v5), signOfInt16(v6), signOfInt16(v7)}
	hasNE := false
	hasZero := false
	for _, s := range signs {
		switch {
		case s == "0":
			hasZero = true
		case s != expectedSign:
			hasNE = true
		}
	}
	switch {
	case hasNE:
		return "NE"
	case hasZero:
		return "INCONCLUSIVE"
	default:
		return "EQ"
	}
}
