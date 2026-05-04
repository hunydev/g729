package postfilter

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FoctPostfix2PrelimM1Prime: Stage F-oct-postfix2-prelim-4
// Step 3 (M1' hypothesis: postfilter 의 γ_t 외 분기 — longterm.go 의 g_l
// clamp 분기 / agc.go 의 α-smoothing initialization 분기 / shortterm.go
// IIR — 가 sample 5..7 부호 결함의 cover 결손인가?).
//
// Approach: white-box (postfilter package 내부). production
// postfilter.Postfilter.Filter() 의 chain 을 replicate 하되 각 stage
// 출력을 sample 5..7 한정으로 dump + 분기 활성/비활성 식별 metric 동반.
//
// Stages (per ITU-T G.729 (06/2012) §A.4.2 cascade — verbatim PDF §A.4.2.1
// short-term / §A.4.2.2 long-term / §A.4.2.3 tilt / §A.4.2.4 AGC; plan
// §"Spec § 인용" cites the chain order long-term → short-term → tilt → AGC
// as recorded in Postfilter.Filter):
//
//  1. expandBandwidth      → aNum (γ_n=0.55), aDen (γ_d=0.70)   §A.4.2.1
//  2. computeResidual      → r(n)                                §A.4.2.1
//  3. refinePitch          → T  ∈ {tInt-1, tInt, tInt+1}         §A.4.2.2
//  4. computeLongTermGain  → g0, g1   (branch: R<=0||E==0)       §A.4.2.2
//  5. applyLongTerm        → r'(n)                               §A.4.2.2
//  6. applyShortTerm       → s_st(n)                             §A.4.2.1
//  7. computeTiltMu        → μ        (γ_t branch: active|inactive) §A.4.2.3
//  8. applyTiltWithMu      → s_tilt(n)                           §A.4.2.3
//  9. computeAGCTargetGain → g_target                            §A.4.2.4
//  10. applyAGC            → sPf(n)   (branch: initialized seed)  §A.4.2.4
//
// codec-start (frame 0 sf0): Postfilter zero value — pastResidual = 0,
// pastSynthPost = 0, pastTiltInput = 0, agcGainPrev = 0, initialized = false.
//
// Production 변경 0. assertion 0 (측정-only).
func TestDiagnostic_FoctPostfix2PrelimM1Prime(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) sf0 LP coefficients (Q12, a[0]=4096)
	var lspDec lsp.Decoder
	lspDec.Reset()
	sfA, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	// (2) excitation u[]
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

	// (3) synth IIR — pre-postfilter input s[]
	var syn synth.Synthesizer
	syn.Reset()
	var s [subframeLen]int16
	syn.Filter(&sfA, &u, &s)

	t.Logf("──────── M1' fixture (ALGTHM frame 0 sf0) ────────")
	t.Logf("PST want sample 5..7              = [%+d %+d %+d]",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("excitation u[5..7]                = [%+d %+d %+d]",
		u[5], u[6], u[7])
	t.Logf("synth IIR s[5..7] (pre-postfilter)= [%+d %+d %+d]  signs=[%s %s %s]",
		s[5], s[6], s[7],
		signOfInt16(s[5]), signOfInt16(s[6]), signOfInt16(s[7]))
	t.Logf("tInt = %d  (refinePitch range = {%d,%d,%d} ∩ [20,143])",
		tInt, tInt-1, tInt, tInt+1)

	// (4) Reproduce postfilter.Filter chain with per-stage capture.
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

	// computeLongTermGain branch detection: re-evaluate (R, E) so we can
	// identify the "R <= 0 || E == 0" early-return branch.
	var ltR, ltE int64
	for n := 0; n < subframeLen; n++ {
		rn := int64(pf.pastResidual[pitchMax+n])
		rnT := int64(pf.pastResidual[pitchMax+n-T])
		ltR += rn * rnT
		ltE += rnT * rnT
	}
	g0, g1 := pf.computeLongTermGain(&r, T)
	ltBranch := "compute"
	if ltR <= 0 || ltE == 0 {
		ltBranch = "clamp_zero(g0=16384,g1=0)"
	}

	var rOut [subframeLen]int16
	pf.applyLongTerm(&r, T, &rOut)

	var sSt [subframeLen]int16
	pf.applyShortTerm(&aDen, &rOut, &sSt)

	// computeTiltMu γ_t branch detection — depends on agcGainPrev BEFORE
	// applyAGC runs.  Codec-start (frame 0 sf0): agcGainPrev = 0 →
	// γ_t = inactive (3277, =0.2).  Subsequent subframes: γ_t = active
	// (14746, =0.9) iff long-term active in prior sf.
	tiltBranch := "active(γ_t=14746)"
	tiltGammaQ14 := gammaTiltActiveQ14
	if pf.agcGainPrev == 0 {
		tiltBranch = "inactive(γ_t=3277)"
		tiltGammaQ14 = gammaTiltInactiveQ14
	}
	muQ15 := pf.computeTiltMu(&aNum, &aDen)

	var sTilt [subframeLen]int16
	pf.applyTiltWithMu(&sSt, muQ15, &sTilt)

	gTarget := pf.computeAGCTargetGain(&s, &sTilt)

	// applyAGC initialization branch detection — first-call seed of
	// agcGainPrev from g_target (per §A.4.2.4 Annex A first-frame init).
	agcInitBefore := pf.initialized
	agcGainPrevBefore := pf.agcGainPrev
	var sPf [subframeLen]int16
	pf.applyAGC(&sTilt, gTarget, &sPf)
	agcBranch := "steady-state"
	if !agcInitBefore {
		agcBranch = "init-seed(agcGainPrev←gTargetQ24)"
	}

	// (5) Per-stage sample-5..7 dump.
	t.Logf("──────── M1' per-stage dump sample 5..7 ────────")
	t.Logf("aNum (γ_n·a Q12)   = %v", aNum)
	t.Logf("aDen (γ_d·a Q12)   = %v", aDen)
	t.Logf("residual r[5..7]   = [%+d %+d %+d]   signs=[%s %s %s]",
		r[5], r[6], r[7],
		signOfInt16(r[5]), signOfInt16(r[6]), signOfInt16(r[7]))
	t.Logf("refinePitch T      = %d  (tInt=%d)", T, tInt)
	t.Logf("computeLongTermGain branch=%s  R=%d E=%d  → g0=%d g1=%d (Q14)",
		ltBranch, ltR, ltE, g0, g1)
	t.Logf("longterm  rOut[5..7] = [%+d %+d %+d]  signs=[%s %s %s]",
		rOut[5], rOut[6], rOut[7],
		signOfInt16(rOut[5]), signOfInt16(rOut[6]), signOfInt16(rOut[7]))
	t.Logf("shortterm sSt[5..7]  = [%+d %+d %+d]  signs=[%s %s %s]",
		sSt[5], sSt[6], sSt[7],
		signOfInt16(sSt[5]), signOfInt16(sSt[6]), signOfInt16(sSt[7]))
	t.Logf("computeTiltMu branch=%s γ_t_Q14=%d  μ_Q15=%d",
		tiltBranch, tiltGammaQ14, muQ15)
	t.Logf("tilt      sTilt[5..7]= [%+d %+d %+d]  signs=[%s %s %s]",
		sTilt[5], sTilt[6], sTilt[7],
		signOfInt16(sTilt[5]), signOfInt16(sTilt[6]), signOfInt16(sTilt[7]))
	t.Logf("computeAGCTargetGain g_target_Q14=%d",
		gTarget)
	t.Logf("applyAGC branch=%s  agcGainPrev pre=%d → post=%d (Q24)  initialized pre=%v",
		agcBranch, agcGainPrevBefore, pf.agcGainPrev, agcInitBefore)
	t.Logf("AGC       sPf[5..7]  = [%+d %+d %+d]  signs=[%s %s %s]",
		sPf[5], sPf[6], sPf[7],
		signOfInt16(sPf[5]), signOfInt16(sPf[6]), signOfInt16(sPf[7]))

	// (6) Cross-check vs production Postfilter.Filter — ensure replicated
	// chain matches the public Filter() output (replication invariant).
	var pfRef Postfilter
	var sPfRef [subframeLen]int16
	pfRef.Filter(&sfA, tInt, &s, &sPfRef)
	matchAll := true
	for n := 0; n < subframeLen; n++ {
		if sPf[n] != sPfRef[n] {
			matchAll = false
			break
		}
	}
	t.Logf("replicated chain == production Postfilter.Filter ? %v", matchAll)
	if !matchAll {
		t.Logf("WARNING: replication mismatch — M1' per-stage dump may not reflect production (still measurement-only).")
	}

	// (7) Sign-transition decision — identify the stage that *creates* the
	// sample 5..7 sign mismatch vs PST want.
	stages := []struct {
		name string
		v5   int16
		v6   int16
		v7   int16
	}{
		{"input s        ", s[5], s[6], s[7]},
		{"residual r     ", r[5], r[6], r[7]},
		{"longterm rOut  ", rOut[5], rOut[6], rOut[7]},
		{"shortterm sSt  ", sSt[5], sSt[6], sSt[7]},
		{"tilt sTilt     ", sTilt[5], sTilt[6], sTilt[7]},
		{"AGC sPf        ", sPf[5], sPf[6], sPf[7]},
		{"PST want       ", wantFrames[0][5], wantFrames[0][6], wantFrames[0][7]},
	}
	t.Logf("──────── M1' sign-chain ────────")
	for i, st := range stages {
		t.Logf("  stage %d %s  signs=[%s %s %s]   raw=[%+d %+d %+d]",
			i+1, st.name,
			signOfInt16(st.v5), signOfInt16(st.v6), signOfInt16(st.v7),
			st.v5, st.v6, st.v7)
	}

	// (8) M1' 가설 평가.
	wantSigns := [3]string{
		signOfInt16(wantFrames[0][5]),
		signOfInt16(wantFrames[0][6]),
		signOfInt16(wantFrames[0][7]),
	}
	sIn := [3]string{signOfInt16(s[5]), signOfInt16(s[6]), signOfInt16(s[7])}
	sOut := [3]string{signOfInt16(sPf[5]), signOfInt16(sPf[6]), signOfInt16(sPf[7])}

	t.Logf("──────── M1' hypothesis evaluation ────────")
	t.Logf("input s[5..7] signs    = [%s %s %s]", sIn[0], sIn[1], sIn[2])
	t.Logf("output sPf[5..7] signs = [%s %s %s]", sOut[0], sOut[1], sOut[2])
	t.Logf("PST want signs         = [%s %s %s]", wantSigns[0], wantSigns[1], wantSigns[2])

	// Check whether postfilter chain *flips* any of sample 5..7 sign.
	flipsInChain := 0
	for i := 0; i < 3; i++ {
		if sIn[i] != "0" && sOut[i] != "0" && sIn[i] != sOut[i] {
			flipsInChain++
		}
	}
	matchInputToWant := sIn == wantSigns
	matchOutputToWant := sOut == wantSigns

	switch {
	case matchOutputToWant:
		t.Logf("(시나리오 M1'-A) postfilter sPf[5..7] sign == PST want")
		t.Logf("   → postfilter 정상. M1' 반증. 결함 = postfilter 상류 (synth IIR / excitation / LP).")
	case flipsInChain > 0:
		t.Logf("(시나리오 M1'-B) postfilter chain 내부에서 부호 flip 발생 (%d/3 sample)", flipsInChain)
		t.Logf("   → M1' 유력. flip 을 일으키는 stage 식별을 §M1' sign-chain 표에서 수행.")
	case matchInputToWant && !matchOutputToWant:
		t.Logf("(시나리오 M1'-C) 입력 s[5..7] sign == PST want 이나 sPf 가 다름")
		t.Logf("   → M1' 유력. postfilter chain 이 spec 와 다른 부호 변환을 도입.")
	default:
		t.Logf("(시나리오 M1'-D) 입력 s 가 이미 PST want 와 부호 불일치, postfilter 가 부호 보존")
		t.Logf("   → M1' 반증. F-oct-prelim-5-3 의 'postfilter sign-preserving' 결정 재확인.")
		t.Logf("   결함 위치 = postfilter 상류 (synth IIR — M3 가설로 진단).")
	}

	t.Logf("[M1' 결정] 부호 결정 stage = (위 sign-chain 표에서 첫 sign 변화 row)")
	t.Logf("[M1' cover 결손 분기] longterm.computeLongTermGain branch=%s, tilt γ_t branch=%s, agc init branch=%s",
		ltBranch, tiltBranch, agcBranch)
}

// --- helpers (test-only, package-local; mirrors decoder/testdata_helpers_test.go) ---

func vectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}

func ensureTestdataPresent(tb testing.TB, paths ...string) {
	tb.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			tb.Skipf("missing test vector %s: %v", p, err)
		}
	}
}

func readG192Frames(tb testing.TB, path string) ([][]byte, []bool) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readG192Frames: %v", err)
	}
	frames, bads, err := bitstream.ReadG192File(bytes.NewReader(data))
	if err != nil {
		tb.Fatalf("ReadG192File(%s): %v", path, err)
	}
	return frames, bads
}

func readPSTFrames(tb testing.TB, path string) [][80]int16 {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readPSTFrames: %v", err)
	}
	const frameSamples = 80
	if len(data)%(frameSamples*2) != 0 {
		tb.Fatalf("readPSTFrames(%s): size %d not multiple of %d",
			path, len(data), frameSamples*2)
	}
	nFrames := len(data) / (frameSamples * 2)
	out := make([][80]int16, nFrames)
	for i := 0; i < nFrames; i++ {
		for n := 0; n < frameSamples; n++ {
			off := (i*frameSamples + n) * 2
			out[i][n] = int16(binary.LittleEndian.Uint16(data[off : off+2]))
		}
	}
	return out
}

func signOfInt16(v int16) string {
	switch {
	case v > 0:
		return "+"
	case v < 0:
		return "−"
	default:
		return "0"
	}
}
