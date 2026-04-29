package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
)

// TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5: Stage F-sept-1 진단.
//
// ITU-T G.729 (06/2012) §4.1.6 eq. (75): u(n) = ĝ_p · v(n) + ĝ_c · c(n).
//
// F-sext-1 §4 시나리오 (i) 후속 — synth.Filter[5..7] = [+1,+1,+1] vs PST want
// [−1,−1,−1] (4 stage 모두 부호 유지) 의 *상류 결함 위치* 를 식별. 본 진단은
// excitation u[5] 합성에서 두 항 (gp·v[5], gc·c[5]) 의 부호 + 절대값 +
// saturation 거동을 측정한다.
//
// 측정-only — 산술 분해는 production BuildExcitation 의 LMult/LShr/LAdd/Round
// 단계를 *test 내부 재현* 으로 capture (production 코드 0-수정, E5 보장).
//
// 시나리오 분류 (Step 4):
//   - (A)  u[5] 부호 = PST want 부호 → IIR 또는 LP 결함 (F-sept-2/3)
//   - (B1) u[5] 부호 ≠ PST want, v[5] 부호 ≠ expected → adaptive codebook 결함
//   - (B2) u[5] 부호 ≠ PST want, c[5] 부호 ≠ expected → fixed codebook 결함
//   - (B3) v/c 부호 정상이나 두 항 절대값 ratio 결함 → gain decode 잔여
//   - (B4) lPitch/lCode saturation → fixed primitives 결함
func TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	pstWant5 := wantFrames[0][5]
	pstHalf5 := int16(int32(pstWant5) >> 1)
	t.Logf("PST want sample 5 = %d (PST/2 spec-target = %d)", pstWant5, pstHalf5)

	// (a) LSP → frame 0 sf0 LP coefficients (sf1 in lsp.Decoder.Decode 명명).
	var lspDec lsp.Decoder
	lspDec.Reset()
	sfA, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})
	t.Logf("sf0 LP coefficients (Q12, a[0]=4096): %v", sfA[:])

	// (b) pitch sf0 → tInt / tFrac
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	t.Logf("pitch sf0: tInt=%d tFrac=%d (P1=%d)", tInt, tFrac, f.P1)

	// (c) adaptive codebook v[]
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	t.Logf("v[] sample 0..7 = [%+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d]",
		v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7])

	// (d) fixed codebook c[] with β from prevGpQ14=0 (zero-init).
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	t.Logf("c[] sample 0..7 = [%+5d %+5d %+5d %+5d %+5d %+5d %+5d %+5d]",
		c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7])

	// (e) gain → gp_q14, gc_q12
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	t.Logf("gain sf0: gp_q14=%d gc_q12=%d (beta_q14=%d, GA1=%d GB1=%d)",
		gpQ14, gcQ12, betaQ14, f.GA1, f.GB1)

	// (f) excitation u[0..7] 분해 trace — production BuildExcitation 알고리즘 재현.
	t.Logf("──────── excitation u[0..7] 분해 trace (§4.1.6 eq. 75) ────────")
	t.Logf("[ n]   v       c        lPitch=LMult(gp,v)   lCode=LShr(LMult(gc,c),11)   lSum         u")
	var u [subframeLen]int16
	for n := 0; n <= 7; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
		lSum := fixed.LAdd(lPitch, lCode)
		u[n] = int16(fixed.Round(fixed.LShl(lSum, 1)))
		t.Logf("[%2d] %+5d  %+5d   %+12d         %+12d              %+12d  %+5d",
			n, v[n], c[n], int32(lPitch), int32(lCode), int32(lSum), u[n])
	}

	// (g) sample 5 집중 분석.
	lPitch5 := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[5]))
	lCode5 := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[5])), 11)
	lSum5 := fixed.LAdd(lPitch5, lCode5)
	u5 := int16(fixed.Round(fixed.LShl(lSum5, 1)))

	t.Logf("──────── sample 5 부호 결정 분석 ────────")
	t.Logf("v[5]                              = %+d (부호 %s)", v[5], signOfInt16(v[5]))
	t.Logf("c[5]                              = %+d (부호 %s)", c[5], signOfInt16(c[5]))
	t.Logf("lPitch = LMult(gp_q14, v[5])      = %+d (부호 %s, |절대값| %d)",
		int32(lPitch5), signOfInt32(int32(lPitch5)), abs32(int32(lPitch5)))
	t.Logf("lCode  = LShr(LMult(gc_q12,c[5]),11) = %+d (부호 %s, |절대값| %d)",
		int32(lCode5), signOfInt32(int32(lCode5)), abs32(int32(lCode5)))
	t.Logf("lSum   = lPitch + lCode           = %+d (부호 %s)",
		int32(lSum5), signOfInt32(int32(lSum5)))
	t.Logf("u[5]   = Round(LShl(lSum, 1))     = %+d (부호 %s)",
		u5, signOfInt16(u5))
	t.Logf("PST want sample 5                 = %+d (부호 %s)",
		pstWant5, signOfInt16(pstWant5))
	t.Logf("PST/2  spec-target sample 5       = %+d (부호 %s)",
		pstHalf5, signOfInt16(pstHalf5))

	// (h) saturation 점검 — Q15 도메인 |값| > 32767 검출.
	const q15Sat = int32(32767)
	saturated := abs32(int32(lPitch5)) > q15Sat || abs32(int32(lCode5)) > q15Sat
	t.Logf("Q15 saturation 발생? %v  (|lPitch|=%d, |lCode|=%d, threshold=32767)",
		saturated, abs32(int32(lPitch5)), abs32(int32(lCode5)))

	// (i) 시나리오 분류 dump.
	t.Logf("──────── F-sept-1 시나리오 분류 ────────")
	uSign := signOfInt16(u5)
	wantSign := signOfInt16(pstWant5)
	t.Logf("u[5] 부호 = %s, PST want 부호 = %s", uSign, wantSign)
	switch {
	case v[5] == 0 && c[5] == 0:
		t.Logf("(시나리오 A') excitation u[5] = 0 (v[5]=0, c[5]=0). sample 5 출력은 전적으로")
		t.Logf("   IIR 누산 (직전 비-zero u[0..4] 의 1/Â(z) 피드백) 으로 결정됨.")
		t.Logf("   → 부호 결정 boundary = synth IIR 또는 LP Â(z). 합성 입력 결함 가능성 제외.")
		t.Logf("   결함 위치 후보 = LP Â(z) (F-sept-2) 또는 synth IIR 1/Â(z) (F-sept-3).")
	case uSign == wantSign:
		t.Logf("(시나리오 A) u[5] 부호 = PST want 부호 → excitation 합성 정상.")
		t.Logf("   결함 위치 후보 = LP Â(z) (F-sept-2) 또는 synth IIR 1/Â(z) (F-sept-3).")
	case saturated:
		t.Logf("(시나리오 B4) Q15 saturation 발생 → internal/fixed primitives 결함 의심.")
	case signOfInt16(v[5]) == "+" && abs32(int32(lPitch5)) > abs32(int32(lCode5)):
		t.Logf("(시나리오 B1) lPitch 가 lSum 부호 결정. v[5]=%s 가 expected '−' 와 불일치 시 → pitch.AdaptiveCodebook 결함.",
			signOfInt16(v[5]))
	case signOfInt16(c[5]) == "+" && abs32(int32(lCode5)) > abs32(int32(lPitch5)):
		t.Logf("(시나리오 B2) lCode 가 lSum 부호 결정. c[5]=%s 가 expected '−' 와 불일치 시 → fcb.Decode 결함.",
			signOfInt16(c[5]))
	default:
		t.Logf("(시나리오 B3) 두 항 부호 정상이나 절대값 ratio 가 PST 와 모순 → gain decode 잔여 결함.")
	}
}

// signOfInt16 / signOfInt32 / abs32 — F-sept 진단 helper.
// (F-sext-1 의 signOf(int16) 와 별도 명명 — F-sext 파일 변경 금지 invariant.)
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

func signOfInt32(v int32) string {
	switch {
	case v > 0:
		return "+"
	case v < 0:
		return "−"
	default:
		return "0"
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
