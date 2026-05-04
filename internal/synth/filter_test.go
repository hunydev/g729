package synth

import (
	"testing"
)

// With a = [4096, 0, 0, ..., 0] (i.e. A(z) = 1, synthesis is identity),
// the filter should reproduce u in s, regardless of pastSynth.
func TestFilter_ZeroLPCIsIdentity(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range u {
		u[i] = int16(1000 + i*37)
	}

	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(9000 - i*100)
	}

	synth.filterSubframe(&a, &u, &s)

	for i := range s {
		if s[i] != u[i] {
			t.Errorf("s[%d] = %d, want %d (zero LPC is identity)", i, s[i], u[i])
		}
	}
}

func TestFilter_FirstOrderImpulseResponse(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

u[0] = 4000

synth.filterSubframe(&a, &u, &s)

expected := []int16{4000, -2000, 1000, -500, 250, -125, 62, -31, 16, -8}
for i, want := range expected {
if s[i] != want && s[i] != want+1 && s[i] != want-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
}
}
if s[20] > 2 || s[20] < -2 {
t.Errorf("s[20] = %d, want |·| ≤ 2 after decay", s[20])
}
}

func TestFilter_FirstOrderPositiveFeedback(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

u[0] = 4000

synth.filterSubframe(&a, &u, &s)

expected := []int16{4000, 2000, 1000, 500, 250, 125}
for i, want := range expected {
if s[i] != want && s[i] != want+1 && s[i] != want-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
}
}
}

func TestFilter_PastStateContributes(t *testing.T) {
var synth Synthesizer
synth.pastSynth[9] = 1000

var u, s [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

synth.filterSubframe(&a, &u, &s)

want := []int16{-500, 250, -125, 62, -31}
for i, w := range want {
if s[i] != w && s[i] != w+1 && s[i] != w-1 {
t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], w)
}
}
}

func TestFilter_StateUpdate(t *testing.T) {
var synth Synthesizer
var u, s [40]int16
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

for i := range u {
u[i] = int16(1000 + i)
}

synth.filterSubframe(&a, &u, &s)

for i := 0; i < 10; i++ {
want := u[30+i]
if synth.pastSynth[i] != want {
t.Errorf("pastSynth[%d] = %d, want %d", i, synth.pastSynth[i], want)
}
}
}

func TestFilter_TwoSubframeContinuity(t *testing.T) {
var synth Synthesizer
var u1, u2, s1, s2 [40]int16
a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

for i := range u1 {
u1[i] = int16(100 + i)
u2[i] = int16(200 + i)
}

synth.filterSubframe(&a, &u1, &s1)
synth.filterSubframe(&a, &u2, &s2)

for i := range s1 {
if s1[i] != u1[i] {
t.Errorf("s1[%d] = %d, want %d", i, s1[i], u1[i])
}
if s2[i] != u2[i] {
t.Errorf("s2[%d] = %d, want %d", i, s2[i], u2[i])
}
}
}

func TestFilter_IIRDecayAcrossBoundary(t *testing.T) {
var synth Synthesizer
synth.pastSynth[9] = 4000

var u, s1, s2 [40]int16
a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

synth.filterSubframe(&a, &u, &s1)

if s1[0] != -2000 && s1[0] != -1999 && s1[0] != -2001 {
t.Errorf("s1[0] = %d, want -2000 ±1", s1[0])
}

synth.filterSubframe(&a, &u, &s2)

for i := range s2 {
if s2[i] > 2 || s2[i] < -2 {
t.Errorf("s2[%d] = %d, expected |·| ≤ 2", i, s2[i])
}
}
}

// TestFilter_SaturationTriggersTwoPassRecovery: construct an input
// large enough that the single-pass Q13 accumulator saturates, and
// verify that the two-pass recovery produces non-saturated output.
//
// We cannot easily read internal saturation flags from the public API,
// so we assert on output shape: with the two-pass guard, the output
// should not sit at Word16 saturation for more than a handful of
// samples; without the guard, it flat-lines at ±32767 once the
// accumulator clamps.
func TestFilter_SaturationTriggersTwoPassRecovery(t *testing.T) {
// Multi-tap LP with all positive feedback contributions and large
// past-synth state: every LMsu in the pass-1 chain pushes the
// accumulator further past Word32 range, causing the original (non-
// saturating-aware) filter to flat-line at Word16 saturation for the
// entire subframe.  With the §3.10 two-pass guard, the recovery pass
// runs on a 1/4-scaled u and pastSynth and produces output that
// preserves dynamic information instead of collapsing to ±32767.
a := [11]int16{4096, 2000, 1500, 1500, 1500, 1500, 1500, 1500, 1500, 1500, 1500}
var u [40]int16
var s [40]int16
var syn Synthesizer
for i := range syn.pastSynth {
syn.pastSynth[i] = 20000
}
syn.Filter(&a, &u, &s)

nSat := 0
for _, v := range s {
if v == 32767 || v == -32768 {
nSat++
}
}
// Without the guard, all 40 samples saturate.  The recovery pass
// should leave at least 10 unsaturated samples.
if nSat > 30 {
t.Fatalf("saturation recovery missing: %d/40 samples at Word16 saturation — two-pass guard not applied", nSat)
}
}

// TestFilter_NonSaturatingInputIsUnchanged: small inputs that do NOT
// trigger saturation must produce non-saturated output (the guard must
// not perturb the hot path).
func TestFilter_NonSaturatingInputIsUnchanged(t *testing.T) {
a := [11]int16{4096, 2000, -500, 0, 0, 0, 0, 0, 0, 0, 0}
var u [40]int16
for i := range u {
u[i] = int16(100 + 3*i)
}
var s [40]int16
var syn Synthesizer
syn.Filter(&a, &u, &s)
for _, v := range s {
if v == 32767 || v == -32768 {
t.Fatalf("non-pathological input saturated output: %v", s)
}
}
}

// TestFilter_GuardUsesFixedOverflowFlag asserts that the §3.10 guard
// triggers via fixed.Overflow on saturation in the LMsu/LShl/Round
// chain and that the recovery pass actually executes.  With the
// extreme positive feedback below (a[1] = -32768), pass 1 LShl
// saturates at sample 1.  In the recovery pass u and pastSynth are
// scaled by ¼; the first sample becomes Round(LShl(2·4096·4096, 3))
// = 4096, which after the ×4 post-scale equals 16384 (matching the
// original input magnitude).  Without the flag-driven recovery the
// first sample would already be 16384 directly from pass 1 and the
// rest of the subframe would flat-line at ±32767; with recovery, the
// first sample matches pass-2's scaled-and-rescaled outcome.
func TestFilter_GuardUsesFixedOverflowFlag(t *testing.T) {
var syn Synthesizer

var a [11]int16
a[0] = 4096
a[1] = -32768

var u [40]int16
for i := range u {
u[i] = 16384
}

var s [40]int16
syn.Filter(&a, &u, &s)

if s[0] != 16384 {
t.Fatalf("Filter s[0] = %d; want 16384 (post-recovery first sample). "+
"§3.10 overflow guard is not running the recovery pass via fixed.Overflow.", s[0])
}
}

// TestFilter_ImpulseResponse_OnePoleClosedForm: A(z)=1−0.5·z^-1 (Q12
// a[1]=−2048)의 1/A(z) 임펄스 응 0.5^n 폐형식과 일치하는지 검증.
//
// 차분식: s[n] = u[n] + 0.5·s[n-1]
// u[0]=8192, u[1..]=0 → s[n] = 8192·(0.5)^n
//
// 본 어서션이 FAIL이면 onePass의 LMult/LMsu/LShl/Round 누산 또는
// Q-포맷 변환에 결함. Stage F-fix는 filterSubframe.onePass 내부.
func TestFilter_ImpulseResponse_OnePoleClosedForm(t *testing.T) {
var sy Synthesizer
a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
var u, s [40]int16
u[0] = 8192

sy.Filter(&a, &u, &s)

expected := []int16{8192, 4096, 2048, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1}
for n, want := range expected {
got := s[n]
diff := int32(got) - int32(want)
if diff < -1 || diff > 1 {
t.Errorf("s[%d]=%d, want %d (0.5^n closed form, ±1 LSB)", n, got, want)
}
}
t.Logf("s[0..15] = %v", s[:16])
}

// TestFilter_SaturationRecovery_ScalingFactorMatchesSpec: ITU-T G.729
// §3.10 "When overflow occurs, the speech samples and the filter
// memory are divided by 4 and the filtering is re-done. The output
// is multiplied by 4 with saturation."
//
// 자극 설계: A(z)=1−0.99·z^-1 강한 IIR 누적.
// 현 구현 ÷2 + ×2를 적용 → spec ÷4 + ×4 와 불일치.
// 본 어서션은 observation-only (t.Logf). F-fix가 promote.
func TestFilter_SaturationRecovery_ScalingFactorMatchesSpec(t *testing.T) {
var sy Synthesizer
a := [11]int16{4096, -4055, 0, 0, 0, 0, 0, 0, 0, 0, 0}
var u, s [40]int16
for i := 0; i < 40; i++ {
u[i] = 20000
}

sy.Filter(&a, &u, &s)

var nSat int
for _, v := range s {
if v == 32767 || v == -32768 {
nSat++
}
}
t.Logf("Pass-2 saturation count = %d / 40", nSat)
t.Logf("s[36..39] = %v (sample-late overflow region)", s[36:40])
if nSat > 5 {
t.Logf("OBSERVATION (F-prep-2 Q-saturation): Pass-2 saturation count = %d "+
"(samples == ±max). ITU-T G.729 §3.10 specifies divide-by-4 + multiply-by-4 "+
"saturation recovery; current implementation uses ÷2 + ×2 (filter.go:33-51), "+
"which exceeds Word16 range under this stimulus. F-fix promotes to t.Errorf.", nSat)
}
}

// TestALGTHMFrame0SF0_SynthFilter_PerSampleBoundary: F-prep-2 Pass-1/Pass-2
// 경계 진단. Stage D-bis Task 3 확정 입력으로 synth.Filter를 직접 구동하고
// 다음을 측정:
//   - 각 샘플의 Pass-1 누산 overflow 발생 여부 (fixed.Overflow 플래그)
//   - Pass-2 두-패스 가드가 트리거되었는지
//   - sample 0..15에서의 |s[n]| 진폭과 dB 레벨 (vs PST 기대값 |2|)
//
// 입력: ALGTHM f0 sf0 LP a[](Q12), excitation u (gpQ14·v + gcQ12·c),
// past_synth (Phase 1i 잠금  codec-start 0 상태).
//
// 본 진단은 Stage F 분기 (P / Q-saturation / Q-onePass) 결정용.
func TestALGTHMFrame0SF0_SynthFilter_PerSampleBoundary(t *testing.T) {
// Stage D-bis Task 3 확정 LP a[] (Q12). LSP→LP는 Stage F-prep-1
// 에서 이미 unstable로 판명되었으므로, 본 테스트는 그 a[]를 입력으로
// 받아 synth.Filter의 saturation 동작을 격리 측정한다.
a := [11]int16{4096, -2197, -375, -924, 7735, 294, 665, 7844, -1010, 145, -33}

// excitation u: ALGTHM f0 sf0의 4-pulse FCB (S1=15: 모두 +Q13 부호)
// at positions 0,1,2,3 + pitch-augmented 20,21,22,23 (β=0.2).
// gpQ14=13815, gcQ12=6844. v=adaptive codebook (codec-start: zero).
// u[n] = (gp·v[n] + gc·c[n]) — codec-start이므로 v=0, u = gc·c.
// Q13 pulse · Q12 gain → Q25, >>13 → Q12 단순화: u ≈ gcQ12·sign·8192/4096
// 직접 합성하려면 BuildExcitation 호출이 정공법.
var v, c [40]int16
// 4 main pulses (Q13 = 8192) at positions 0..3, all positive (S1=15).
c[0], c[1], c[2], c[3] = 8192, 8192, 8192, 8192
// Pitch-augmented track at 20..23 with β=0.2 (Q14 = 3277 → contribution
// is added during BuildExcitation if pitch lag <40; codec-start tInt=20).
c[20], c[21], c[22], c[23] = 8192, 8192, 8192, 8192

const gpQ14 int16 = 13815
// gcQ12=6844 (g_c≈1.671) → mant=6844*4=27376, exp=0.
const gcMantQ14 int16 = 27376
const gcExp int8 = 0

var u [40]int16
BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)
t.Logf("u[0..15] = %v", u[:16])

var sy Synthesizer
// codec-start: pastSynth = 0.
var s [40]int16
sy.Filter(&a, &u, &s)

t.Logf("s[0..15] = %v", s[:16])
t.Logf("s[16..39] = %v", s[16:])

// PST expected for sample 0 = 2 (Phase 1i lock target — but PST is
// post-postfilter; raw s[0] differs). Stage D-bis observed |s·2|=12
// at sample 5 vs |PST|=1. Log per-sample dB vs a 1-LSB reference.
for n := 0; n < 16; n++ {
mag := int32(s[n])
if mag < 0 {
mag = -mag
}
t.Logf("  s[%2d] = %6d  (|s|=%d)", n, s[n], mag)
}

// Saturation counter: how many samples sit at ±Word16 max.
var nSat int
for _, x := range s {
if x == 32767 || x == -32768 {
nSat++
}
}
t.Logf("OBSERVATION: nSat = %d / 40 (Pass-2 saturation count)", nSat)
}
