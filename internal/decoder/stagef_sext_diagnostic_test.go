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
	"fmt"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7: Stage F-sext-1 진단.
//
// ITU-T G.729 (06/2012) §4 + §A.4.2: postfilter chain sequence
//
//	synth.Filter → postfilter.Filter → hpFilter → pcm.ScaleUpSat
//
// F-quint-3 §3.3 측정으로 ALGTHM frame 0 sf0 hpFilter[5..7] = [1, 1, 1] vs
// PST/2 [-1, -1, -1] 부호 반전 (|Δ|=2) 잔존 확인. 본 진단은 chain 의 각
// 단계에서 sample 5..7 의 *원본 부호 + 절대값* 을 측정해 부호 반전
// boundary 를 식별한다. production 코드 0-수정 (측정-only).
//
// Decoder instance 우회 — 4 stage instance 를 zero-init 으로 직접
// 생성 + Reset 호출. hpFilter 는 hpFilterStandalone (test 내부 wrapper)
// 으로 호출하여 Decoder.hpX/hpY 의존성 제거.
func TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// PST/2 spec-target sample 5..7
	var pstHalf [subframeLen]int16
	for n := 0; n < subframeLen; n++ {
		pstHalf[n] = int16(int32(wantFrames[0][n]) >> 1)
	}
	t.Logf("PST/2 sample 5..7 = [%d %d %d]", pstHalf[5], pstHalf[6], pstHalf[7])

	// LSP 디코딩 → frame 0 sf0 LP coefficients (sf1 = first subframe in
	// decoder의 Decode signature; sf0 in our terminology).
	var lspDec lsp.Decoder
	lspDec.Reset()
	sfA, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	// pitch (sf0 → DecodeDelaySubframe1 in pitch package nomenclature)
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))

	// fcb pitch enhancement β with prevGpQ14 = 0 (zero-init)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)

	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)

	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcMant_gcQ12, gcExp_gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	gcQ12 := gain.LegacyGcQ12FromMantExp(gcMant_gcQ12, gcExp_gcQ12)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)

	// 4 chain stage capture
	var syn synth.Synthesizer
	syn.Reset()
	var sStage [subframeLen]int16
	syn.Filter(&sfA, &u, &sStage)

	var pst postfilter.Postfilter
	pst.Reset()
	var pfStage [subframeLen]int16
	pst.Filter(&sfA, tInt, &sStage, &pfStage)

	// HP filter 단독 호출 — Decoder instance 우회
	var hpStage [subframeLen]int16
	hpFilterStandalone(&pfStage, &hpStage)

	var pcmStage [subframeLen]int16
	pcm.ScaleUpSat(hpStage[:], pcmStage[:])

	// sample 5..7 비교표
	t.Logf("──────── sample 5..7 chain trace ────────")
	t.Logf("stage              [   5    6    7]  부호분포")
	t.Logf("synth.Filter       %s  %s", fmtSamples3(sStage[5:8]), signs3(sStage[5:8]))
	t.Logf("postfilter.Filter  %s  %s", fmtSamples3(pfStage[5:8]), signs3(pfStage[5:8]))
	t.Logf("hpFilter           %s  %s", fmtSamples3(hpStage[5:8]), signs3(hpStage[5:8]))
	t.Logf("pcm.ScaleUpSat     %s  %s  (PST 도메인)", fmtSamples3(pcmStage[5:8]), signs3(pcmStage[5:8]))
	t.Logf("PST want sample 5..7         = [%d %d %d]", wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("PST/2 spec-target sample 5..7 = [%d %d %d]", pstHalf[5], pstHalf[6], pstHalf[7])

	// boundary 식별 — sample 5 단독 dump
	stageNames := []string{"synth.Filter", "postfilter.Filter", "hpFilter", "pcm.ScaleUpSat"}
	stageOuts := [4][subframeLen]int16{sStage, pfStage, hpStage, pcmStage}
	t.Logf("──────── sample 5 부호 boundary ────────")
	for i, name := range stageNames {
		s5 := stageOuts[i][5]
		t.Logf("%-18s sample 5 부호 = %s (값 %d)", name, signOf(s5), s5)
	}
	t.Logf("PST want sample 5 부호 = %s (값 %d)", signOf(wantFrames[0][5]), wantFrames[0][5])
	t.Logf("PST/2  sample 5 부호 = %s (값 %d)", signOf(pstHalf[5]), pstHalf[5])

	// 보조: gain VQ 출력 (cross-check 용)
	t.Logf("gain VQ: gp_q14=%d gc_q12=%d   tInt=%d tFrac=%d   beta_q14=%d",
		gpQ14, gcQ12, tInt, tFrac, betaQ14)
}

// hpFilterStandalone: F-sext 진단용 wrapper. Decoder.hpFilter 와 동일
// 알고리즘이나 state 를 zero-init 으로 시작. production 코드 변경 0
// — 본 함수는 *_test.go 내부.
func hpFilterStandalone(in *[subframeLen]int16, out *[subframeLen]int16) {
	var hpX [2]int16
	var hpY [2]int32
	x1, x2 := hpX[0], hpX[1]
	y1, y2 := hpY[0], hpY[1]
	for n := 0; n < subframeLen; n++ {
		xn := in[n]
		ff := int32(hpB0Q13)*int32(xn) +
			int32(hpB1Q13)*int32(x1) +
			int32(hpB2Q13)*int32(x2)
		ff >>= 1
		fb := int64(hpNegA1Q12) * int64(y1)
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(y2)) >> 13
		acc := int64(ff) + fb
		yn := (acc + (1 << 11)) >> 12
		if yn > 32767 {
			yn = 32767
		} else if yn < -32768 {
			yn = -32768
		}
		out[n] = int16(yn)
		x2, x1 = x1, xn
		y2, y1 = y1, int32(acc)
	}
}

func fmtSamples3(s []int16) string {
	return fmt.Sprintf("[%4d %4d %4d]", s[0], s[1], s[2])
}

func signs3(s []int16) string {
	return fmt.Sprintf("[%s %s %s]", signOf(s[0]), signOf(s[1]), signOf(s[2]))
}

func signOf(v int16) string {
	switch {
	case v > 0:
		return "+"
	case v < 0:
		return "−"
	default:
		return "0"
	}
}
