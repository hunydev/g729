package decoder

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/tables"
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

// TestDiagnostic_FseptLPReferenceCrossCheck: Stage F-sept-2 진단.
//
// ITU-T G.729 (06/2012) §4.1 (LSP decoding) + §3.2.6 (LSP-to-LP) 에서
// 직접 도출한 float64 reference impl 을 작성하고, production
// lsp.Decoder.Decode 출력의 sf0 LP coefficients (a[0..10] Q12, a[0]=4096)
// 와 비교한다. 양자화 / saturation 0 — spec real-valued 거동 그대로.
//
// 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 0 인용.
// reference impl 의 모든 라인은 §4.1 + §3.2.6 + §4.3 Table 9 식 verbatim
// 도출. internal/tables 의 codebook 데이터 (LSPCodebookL1/L2/L3,
// MAPredictorsLSP) 는 ITU spec 정의 verbatim 이므로 *조회만* 수행하며,
// production internal/lsp/*.go 의 알고리즘 (Q-format, saturation,
// fixed.LMac 누산 등) 은 일절 복제하지 않는다.
//
// 측정-only — Δ assertion 0 (t.Logf only).
//
// 주의: 본 cycle 시작 시 lsp_lp.go 가 uncommitted modified 상태였다
// (F-bis-1 P fix 보류). production 측정값은 modified 상태 기준이며,
// ref ≠ prod 인 경우 modified 영향 분리는 보고서 §3.3 (git stash 재측정)
// 으로 수행한다.
func TestDiagnostic_FseptLPReferenceCrossCheck(t *testing.T) {
bitPath := vectorPath("ALGTHM.BIT")
ensureTestdataPresent(t, bitPath)

frames, _ := readG192Frames(t, bitPath)
var f bitstream.Frame
if err := bitstream.Unpack(frames[0], &f); err != nil {
t.Fatalf("Unpack frame 0: %v", err)
}

// production: lsp.Decoder.Decode 의 sf1 (= subframe 0 의 LP coefficients).
// §4.1 / §3.2.6 명명에서 "sf0" 에 해당. Q12, a[0]=4096.
var lspDec lsp.Decoder
lspDec.Reset()
sf0Prod, _ := lspDec.Decode(lsp.Indices{
L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
})

// reference: §4.1 + §3.2.6 직접 도출 float64 impl (subframe 0 only).
sf0Ref := referenceLSPToLPSubframe0(t, uint8(f.L0), uint8(f.L1), uint8(f.L2), uint8(f.L3))

// dump
t.Logf("──────── F-sept-2 cross-check (production Q12 vs §3.2.6 reference float64) ────────")
t.Logf("L0=%d L1=%d L2=%d L3=%d", f.L0, f.L1, f.L2, f.L3)
t.Logf("idx   prod_q12   ref(float64)        ref(round_q12)   Δ(prod − ref_round)")
maxAbsDelta := int32(0)
mismatch := 0
for i := 0; i <= 10; i++ {
refQ12 := int16(roundFloat(sf0Ref[i] * 4096))
delta := int32(sf0Prod[i]) - int32(refQ12)
if delta < 0 {
if -delta > maxAbsDelta {
maxAbsDelta = -delta
}
} else if delta > maxAbsDelta {
maxAbsDelta = delta
}
if delta != 0 {
mismatch++
}
t.Logf("[%2d]   %+6d     %+18.12f   %+6d           %+d",
i, sf0Prod[i], sf0Ref[i], refQ12, delta)
}
t.Logf("summary: max|Δ| = %d, mismatch_count = %d / 11", maxAbsDelta, mismatch)

// 시나리오 분류 (L1 / L2 / L3) — Step 4 의 stash 재측정은 수동 진단
// 단계로 별도 수행. 본 함수는 측정 dump + 1 차 분류만 출력.
switch {
case maxAbsDelta == 0:
t.Logf("F-sept-2 분류: (L1) prod = ref (sf0 LP coeff 11 항 비트-정확) — LP 변환 spec 정합.")
case maxAbsDelta <= 2:
t.Logf("F-sept-2 분류: (L2) max|Δ|=%d ∈ {1,2} — Q12 양자화 rounding 누적 (수치 정상). LP 결함 제외.", maxAbsDelta)
default:
t.Logf("F-sept-2 분류: (L3) max|Δ|=%d > 2 — LSP-to-LP 변환 결함 의심.", maxAbsDelta)
t.Logf("   하위 진단 의무: lsp_lp.go modified 상태 영향 분리 ( §3.3, git stash 재측정).")
t.Logf("   stash 후 잔존 → (L3a) lsp_lp.go 외 결함; stash 후 |Δ|≤1 → (L3b) lsp_lp.go modified 가 LP 손상.")
}
}

// referenceLSPToLPSubframe0: §4.1 LSP decoding + interpolation + §3.2.6
// LSP-to-LP 의 float64 직접 구현. frame 0 sf0 의 LP coefficients (a[0..10],
// a[0]=1.0 real) 반환. 양자화 / saturation 0.
//
// 외부 G.729 구현 0 인용. 모든 산술 라인은 ITU-T G.729 (06/2012) §4.1 +
// §3.2.6 + §4.3 Table 9 의 spec 식에서 직접 도출.
//
// internal/tables 의 LSPCodebookL1/L2/L3 (Q13 int16) + MAPredictorsLSP
// (Q15 int16) 은 ITU spec 의 verbatim 데이터이므로 조회만 수행. 모든 산술은
// float64 real-valued 으로 (radian 단위) 환산 후 진행한다.
func referenceLSPToLPSubframe0(t *testing.T, l0, l1, l2, l3 uint8) [lpcOrder + 1]float64 {
t.Helper()

// (1) §3.2.4 / §4.1 split-VQ residual combine (eq. 19).
//     r[i] = L1[l1][i] + L2[l2][i]       for i ∈ [0,5)
//     r[i] = L1[l1][i] + L3[l3][i-5]     for i ∈ [5,10)
//     Q13 → radians: divide by 2^13 = 8192.
const q13 = 8192.0
var r [10]float64
for i := 0; i < 5; i++ {
r[i] = (float64(tables.LSPCodebookL1[l1][i]) +
float64(tables.LSPCodebookL2[l2][i])) / q13
}
for i := 5; i < 10; i++ {
r[i] = (float64(tables.LSPCodebookL1[l1][i]) +
float64(tables.LSPCodebookL3[l3][i-5])) / q13
}

// (2) §3.2.4 pre-predictor pair-rearrangement, two passes:
//     J = 0.0012 then J = 0.0006. For each adjacent pair with
//     gap < J, spread to (mid − J/2, mid + J/2).
for _, J := range [2]float64{0.0012, 0.0006} {
for i := 1; i < 10; i++ {
if r[i]-r[i-1] < J {
mid := (r[i] + r[i-1]) * 0.5
r[i-1] = mid - J*0.5
r[i] = mid + J*0.5
}
}
}

// (3) §3.2.4 / §4.1 MA predictor reconstruction (eq. 20):
//     ω̂(n)[i] = (1 − Σ_k p_k[i]) · r̂(n)[i] + Σ_k p_k[i] · r̂(n−k)[i]
//     for k = 1..4, with predictor selector L0 ∈ {0,1}.
//     Q15 → real: divide by 2^15 = 32768.
const q15 = 32768.0
preds := &tables.MAPredictorsLSP[l0]

// §4.3 Table 9 codec-start initialization: r̂(n−k)[i] = i·π/11
// (radians, real-valued) for all k = 1..4 on the first decoded frame.
var pastResidual [4][10]float64
for k := 0; k < 4; k++ {
for i := 0; i < 10; i++ {
pastResidual[k][i] = float64(i+1) * math.Pi / 11.0
}
}

var omega [10]float64
for i := 0; i < 10; i++ {
var sumP float64
for k := 0; k < 4; k++ {
sumP += float64(preds[k][i]) / q15
}
acc := (1.0 - sumP) * r[i]
for k := 0; k < 4; k++ {
acc += (float64(preds[k][i]) / q15) * pastResidual[k][i]
}
omega[i] = acc
}

// (4) §3.2.4 post-predictor stability:
//     1) sort ascending; 2) ω_1 ≥ 0.005; 3) ω_{i+1} − ω_i ≥ 0.0391;
//     4) ω_10 ≤ 3.135 (spec radian thresholds verbatim).
const (
minEdge = 0.005
minGap  = 0.0391
maxEdge = 3.135
)
// insertion sort (n=10).
for i := 1; i < 10; i++ {
v := omega[i]
j := i - 1
for j >= 0 && omega[j] > v {
omega[j+1] = omega[j]
j--
}
omega[j+1] = v
}
if omega[0] < minEdge {
omega[0] = minEdge
}
for i := 1; i < 10; i++ {
if omega[i] < omega[i-1]+minGap {
omega[i] = omega[i-1] + minGap
}
}
if omega[9] > maxEdge {
omega[9] = maxEdge
for i := 9; i > 0; i-- {
if omega[i-1] > omega[i]-minGap {
omega[i-1] = omega[i] - minGap
}
}
}

// (5) §3.2.5 LSF → LSP per coordinate: q_i = cos(ω_i).
var lspCurr [10]float64
for i := 0; i < 10; i++ {
lspCurr[i] = math.Cos(omega[i])
}

// (6) §4.1 / §4.3 Table 9 codec-start prev-frame LSP init:
//     q_i^(prev) = cos(i·π/11), i = 1..10.
var lspPrev [10]float64
for i := 0; i < 10; i++ {
lspPrev[i] = math.Cos(float64(i+1) * math.Pi / 11.0)
}

// (7) §4.1.2 per-subframe LSP interpolation for sf0:
//     q_i^(sf0) = 0.5 · q_i^(prev) + 0.5 · q_i^(curr)
var lspSF0 [10]float64
for i := 0; i < 10; i++ {
lspSF0[i] = 0.5*lspPrev[i] + 0.5*lspCurr[i]
}

// (8) §3.2.6 LSP → LP polynomial expansion:
//     F1(z) = Π_{i ∈ {0,2,4,6,8}} (1 − 2 q_i z^-1 + z^-2)
//     F2(z) = Π_{i ∈ {1,3,5,7,9}} (1 − 2 q_i z^-1 + z^-2)
//     A(z)  = ((1 + z^-1)·F1(z) + (1 − z^-1)·F2(z)) / 2
//
//     §3.2.6 recurrence (j = 1..top):
//         F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2)
//     with F_i(0) = 1, F_i(j<0) = 0.
var f1, f2 [11]float64
f1[0] = 1.0
f2[0] = 1.0
for step := 0; step < 5; step++ {
q1 := lspSF0[2*step]
q2 := lspSF0[2*step+1]
top := 2*step + 2
for j := top; j >= 2; j-- {
f1[j] = f1[j] - 2.0*q1*f1[j-1] + f1[j-2]
f2[j] = f2[j] - 2.0*q2*f2[j-1] + f2[j-2]
}
f1[1] = f1[1] - 2.0*q1
f2[1] = f2[1] - 2.0*q2
}

// §3.2.6 final assembly:
//   a[k] = (F1[k] + F1[k-1] + F2[k] − F2[k-1]) / 2,  k = 0..10
//   (with F1[-1] = F2[-1] = 0).
var aRef [lpcOrder + 1]float64
for k := 0; k <= 10; k++ {
var prev1, prev2 float64
if k > 0 {
prev1 = f1[k-1]
prev2 = f2[k-1]
}
aRef[k] = 0.5 * (f1[k] + prev1 + f2[k] - prev2)
}
return aRef
}

// roundFloat: float64 → nearest int32 (half-away-from-zero). F-sept-2
// helper for projecting reference float a[k] into Q12 for prod 비교.
func roundFloat(f float64) int32 {
if f >= 0 {
return int32(f + 0.5)
}
return int32(f - 0.5)
}
