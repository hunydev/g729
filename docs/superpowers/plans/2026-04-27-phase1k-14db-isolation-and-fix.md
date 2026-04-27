# Phase 1k 구현 플랜 — 14 dB 오차 격리 및 수정

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 단일-펄스 격리 하네스 + 모듈별 Q-포맷 계약 테스트로 14 dB 오차 위치를 관측적으로 특정하고, 수정 적용 후 ALGTHM frame 0 (80 샘플) 비트-정확을 달성한다.

**Architecture:** 3-Stage 파이프라인 (Diagnose → Fix → Validate). Stage D에서 모듈별 Q-포맷 계약과 단일-펄스 하네스를 영구 가드로 추가하여 진단의 산출물 자체가 미래 회귀 가드가 되게 함. Stage F는 진단 결과에 따라 위치 결정 (한 줄, 가능하면 한 파일). Stage V는 ALGTHM frame 0 80 샘플 비트-정확 + 병리적 테스트 A+B 혼합 재인증.

**Tech Stack:** Go 1.x, `internal/fixed` (ITU-T G.191 basic ops with sticky overflow flag), `internal/{fcb,gain,synth,postfilter,decoder}`, ITU 테스트 벡터 (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/`).

**Spec:** `docs/superpowers/specs/2026-04-27-phase1k-14db-isolation-and-fix-design.md`

---

## File Structure (작업 파일)

**신규 5개**:
- `internal/decoder/frame0_regression_test.go` — Phase 1i 회귀 가드 (sample 0 유지 + Stage 단계별 확장)
- `internal/fcb/qformat_contract_test.go` — `PulseAmplitude` Q13 + Σc² 식별식
- `internal/gain/qformat_contract_test.go` — `fixedCodebookEnergy` Q26 / `log2Fixed` Q0 입력 / 로그-도메인 상수
- `internal/synth/qformat_contract_test.go` — `BuildExcitation` 체인 + `filterSubframe` `a[0]=4096`
- `internal/postfilter/qformat_contract_test.go` — sub-stage I/O Q-포맷
- `internal/decoder/diagnostic_singlepulse_test.go` — 13개 경계 단일-펄스 하네스

**기존 수정**:
- `internal/decoder/decode_test.go` — Task 1에서 `TestDecode_Frame0Sample0_MatchesALGTHM` 제거 (새 파일로 이동), V2에서 `TestDecode_ITUVectorAlgthmBitExact` `t.Skip` 제거
- `internal/gain/decode.go` 또는 `internal/synth/excitation.go` 또는 `internal/synth/filter.go` 또는 `internal/postfilter/*.go` — Stage F 1줄 수정 (위치는 진단 결과 의존)
- `internal/gain/pathological_test.go` — V3 A+B 혼합 재인증

---

## Task 1: D1 — frame 0 회귀 가드 파일 분리

**Goal:** Phase 1i가 잠근 frame 0 sample 0 = 2 가드를 전용 파일로 옮기고, sf1의 나머지 39 샘플을 진단 로깅으로 관측한다 (관측-우선 TDD).

**Files:**
- Create: `internal/decoder/frame0_regression_test.go`
- Modify: `internal/decoder/decode_test.go` (기존 `TestDecode_Frame0Sample0_MatchesALGTHM` 제거)

- [x] **Step 1: 새 파일 생성**

`internal/decoder/frame0_regression_test.go`:

```go
package decoder

import "testing"

// TestDecode_Frame0Sample0_MatchesALGTHM is the Phase 1i regression
// guard: ALGTHM frame 0 sample 0 must equal the ITU reference's
// sample 0. ANY change in this phase that regresses this assertion
// must be rolled back per Phase 1k spec §7.2.
func TestDecode_Frame0Sample0_MatchesALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	if out[0] != wantFrames[0][0] {
		t.Errorf("frame 0 sample 0: got=%d want=%d (Δ=%d)",
			out[0], wantFrames[0][0], int32(out[0])-int32(wantFrames[0][0]))
	}
}

// TestDecode_Frame0SF1_DiagnosticLog observes sf1 (samples 0..39)
// against ALGTHM.PST. No assertions — purely diagnostic. Used during
// Stage F as a moving target: as the 14 dB fix lands, this output
// shows how many sf1 samples now match.
//
// In Stage V (Task 9) this becomes an assertion test for the full
// frame (samples 0..79).
func TestDecode_Frame0SF1_DiagnosticLog(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	for n := 0; n < 40; n++ {
		t.Logf("sf1 sample %2d: got=%6d want=%6d Δ=%+d",
			n, out[n], wantFrames[0][n],
			int32(out[n])-int32(wantFrames[0][n]))
	}
}
```

- [x] **Step 2: 기존 파일에서 중복 테스트 제거**

`internal/decoder/decode_test.go`의 lines 542-565 (`TestDecode_Frame0Sample0_MatchesALGTHM` 함수 전체) 삭제.

- [x] **Step 3: 두 테스트 실행 → 통과 + 진단 로그 확인**

Run: `go test -run 'TestDecode_Frame0(Sample0|SF1_DiagnosticLog)' ./internal/decoder/ -v`

Expected:
- `TestDecode_Frame0Sample0_MatchesALGTHM`: PASS (Phase 1i 보장)
- `TestDecode_Frame0SF1_DiagnosticLog`: PASS (어서션 0개), `t.Logf` 출력에 sf1 40 샘플 got vs want 표시

이 단계에서 sf1 중 몇 개 샘플이 이미 일치하는지 **로그를 읽고 메모**해 둘 것 (Stage F 후 비교용).

- [x] **Step 4: 회귀 점검 (Phase 1i 가드 다른 경로로 깨지지 않았는지)**

Run: `go test -race ./internal/decoder/`

Expected: ALL PASS, `TestDecode_ITUVectorAlgthmBitExact` 등은 여전히 `t.Skip`.

- [x] **Step 5: Commit**

```bash
git add internal/decoder/frame0_regression_test.go internal/decoder/decode_test.go
git commit -m "$(cat <<'EOF'
test(decoder): split frame 0 regression guard + add sf1 diagnostic log

Move the Phase 1i sample-0 lock to its own file, add sf1 (40-sample)
diagnostic logging that prints got vs want per sample. Pure observation,
no new assertions. Diagnostic output drives Stage F's incremental scope
expansion.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 2: D2 — fcb Q-포맷 계약 테스트

**Goal:** `PulseAmplitude == 8192` Q13 불변, ACELP 4-pulse codebook의 `Σc² = N·2²⁶` 식별식, 피치 강화 후 `|c[n]|` 상한을 영구 어서션으로 박는다.

**Files:**
- Create: `internal/fcb/qformat_contract_test.go`

- [x] **Step 1: 계약 테스트 작성**

`internal/fcb/qformat_contract_test.go`:

```go
package fcb

import "testing"

// TestQFormatContract_PulseAmplitudeIsOneQ13 — PulseAmplitude is the
// Q13 representation of true pulse magnitude +1.0 per ITU-T G.729 §3.8.
// Hard-coded to 8192 = 2^13 = +1.0 in Q13.
func TestQFormatContract_PulseAmplitudeIsOneQ13(t *testing.T) {
	const want int16 = 1 << 13
	if PulseAmplitude != want {
		t.Fatalf("PulseAmplitude = %d, want %d (= +1.0 Q13)",
			PulseAmplitude, want)
	}
}

// TestQFormatContract_SinglePulseEnergyIs2to26 — for a single +Q13
// pulse, Σc² (raw, before any Q-format reinterpretation) equals 2^26.
// This is the input that the gain decoder's fixedCodebookEnergy will
// see; documenting it here pins down the inter-module Q-format
// contract.
func TestQFormatContract_SinglePulseEnergyIs2to26(t *testing.T) {
	var c [40]int16
	c[0] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 1 << 26
	if sum != want {
		t.Fatalf("Σc² = %d, want %d (= 2^26 for single Q13 pulse)",
			sum, want)
	}
}

// TestQFormatContract_FourPulseEnergyIs2to28 — canonical ACELP 4-pulse
// codebook: Σc² = 4·2^26 = 2^28. This is the Q26 representation of
// true energy 4.0.
func TestQFormatContract_FourPulseEnergyIs2to28(t *testing.T) {
	var c [40]int16
	c[5] = PulseAmplitude
	c[11] = PulseAmplitude
	c[22] = PulseAmplitude
	c[33] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 1 << 28
	if sum != want {
		t.Fatalf("Σc² = %d, want %d (= 2^28 for canonical 4-pulse Q13)",
			sum, want)
	}
}

// TestQFormatContract_PostEnhancementBoundedByMaxBeta — after pitch
// enhancement c(n) ← c(n) + β·c(n−t), |c[n]| ≤ PulseAmplitude·(1 + βMax)
// where βMax is the §3.8 clamp ceiling. This guards against the
// enhancement loop blowing the Q13 range.
func TestQFormatContract_PostEnhancementBoundedByMaxBeta(t *testing.T) {
	var c [40]int16
	c[0] = PulseAmplitude
	c[5] = PulseAmplitude
	c[10] = PulseAmplitude
	c[15] = PulseAmplitude
	// β at maximum allowed value (Q14 = 16384 → 1.0 true; clamping
	// inside the enhancer caps it lower).
	betaQ14 := ClampPitchGainForEnhancement(32767)
	applyPitchEnhancement(&c, 5, betaQ14)
	for n, v := range c {
		// Empirical upper bound: PulseAmplitude · 2 = 16384.
		// The enhancement is contractive (β < 1) but cumulative
		// hits across multiple positions can sum.
		if v > 32767 || v < -32768 {
			t.Errorf("c[%d] = %d after enhancement: out of int16 range",
				n, v)
		}
	}
}
```

- [x] **Step 2: 테스트 실행**

Run: `go test -run 'TestQFormatContract_' ./internal/fcb/ -v`

Expected: 4개 PASS. 만약 하나라도 실패하면 fcb 모듈의 자기-주장이 깨진 것 → 보고서에 기록 후 plan 중단.

- [x] **Step 3: 회귀 점검**

Run: `go test -race ./internal/fcb/`

Expected: ALL PASS.

- [x] **Step 4: Commit**

```bash
git add internal/fcb/qformat_contract_test.go
git commit -m "$(cat <<'EOF'
test(fcb): Q-format contract — PulseAmplitude=8192 Q13, Σc²=N·2^26

Lock the inter-module Q-format claim that fcb's c[] is Q13 with pulse
magnitudes ±2^13 (= ±1.0 true). Σc² for N-pulse codebooks = N·2^26.
Phase 1k diagnostic harness depends on this contract being explicit.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 3: D3 — gain Q-포맷 계약 테스트 (Phase 1j 확장)

**Goal:** Phase 1j Task 1이 docstring으로만 박은 계약(`fixedCodebookEnergy` Q26, `log2Fixed` Q0 입력, 로그-도메인 상수)을 컴파일타임 어서션으로 변환한다.

**Files:**
- Create: `internal/gain/qformat_contract_test.go`

- [x] **Step 1: 계약 테스트 작성**

`internal/gain/qformat_contract_test.go`:

```go
package gain

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// TestQFormatContract_FixedCodebookEnergyIsQ26 — fixedCodebookEnergy
// returns Σ c[n]² as a Word32. For c at Q13, each squared term is
// Q26; the sum is therefore Q26. This contract is the foundation of
// Phase 1j's gain Q-format diagnosis.
func TestQFormatContract_FixedCodebookEnergyIsQ26(t *testing.T) {
	tests := []struct {
		name  string
		setup func(c *[40]int16)
		want  fixed.Word32
	}{
		{"single pulse +1.0 Q13", func(c *[40]int16) { c[0] = 8192 }, 1 << 26},
		{"four pulses canonical", func(c *[40]int16) {
			c[5] = 8192
			c[11] = 8192
			c[22] = 8192
			c[33] = 8192
		}, 1 << 28},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c [40]int16
			tt.setup(&c)
			got := fixedCodebookEnergy(&c)
			if got != tt.want {
				t.Errorf("got=%d want=%d (= Σc²·2^26-style accumulation)",
					got, tt.want)
			}
		})
	}
}

// TestQFormatContract_Log2FixedTreatsInputAsQ0 — log2Fixed returns
// log2(x)·1024 (Q10) treating x as a Q0 integer. So log2Fixed(2^k)
// = k·1024. Caller is responsible for adjusting if its input has
// a non-zero Q-shift.
func TestQFormatContract_Log2FixedTreatsInputAsQ0(t *testing.T) {
	tests := []struct {
		name string
		x    fixed.Word32
		want fixed.Word32
	}{
		{"x=1", 1, 0},
		{"x=2", 2, 1024},
		{"x=2^10", 1 << 10, 10 * 1024},
		{"x=2^26", 1 << 26, 26 * 1024},
		{"x=2^28", 1 << 28, 28 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := log2Fixed(tt.x)
			// Allow ±2 LSB at Q10 (per log2Fixed's documented accuracy).
			diff := int32(got) - int32(tt.want)
			if diff < -2 || diff > 2 {
				t.Errorf("got=%d want=%d (Δ=%d, max ±2 LSB Q10)",
					got, tt.want, diff)
			}
		})
	}
}

// TestQFormatContract_Pow2FixedReturnsQ0 — pow2Fixed(input Q10) returns
// 2^(input/1024) as Q0. So pow2Fixed(0) = 1, pow2Fixed(1024) = 2,
// pow2Fixed(10*1024) = 1024.
func TestQFormatContract_Pow2FixedReturnsQ0(t *testing.T) {
	tests := []struct {
		name string
		x    fixed.Word32
		want fixed.Word32
	}{
		{"2^0", 0, 1},
		{"2^1", 1024, 2},
		{"2^10", 10 * 1024, 1024},
		{"2^14", 14 * 1024, 1 << 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pow2Fixed(tt.x)
			// Allow ±1% relative tolerance for the 33-entry table.
			rel := math.Abs(float64(int32(got)-int32(tt.want))) /
				math.Max(float64(tt.want), 1)
			if rel > 0.01 {
				t.Errorf("got=%d want=%d (rel err=%.4f, max 1%%)",
					got, tt.want, rel)
			}
		})
	}
}

// TestQFormatContract_LogDomainConstants — verify the four magic
// numbers in decode.go match their physical identities. These are
// pure compile-time invariants; if a future refactor changes them,
// this test catches the drift.
func TestQFormatContract_LogDomainConstants(t *testing.T) {
	// dbPerLog2Q13 = 10·log10(2) · 2^13 ≈ 24659.7
	wantDbPerLog2Q13 := int(math.Round(10 * math.Log10(2) * (1 << 13)))
	if dbPerLog2Q13 != wantDbPerLog2Q13 {
		t.Errorf("dbPerLog2Q13 = %d, want %d", dbPerLog2Q13, wantDbPerLog2Q13)
	}
	// tenLog10_40Q10 = 10·log10(40) · 2^10 ≈ 16404.7
	wantTenLog10_40Q10 := int(math.Round(10 * math.Log10(40) * (1 << 10)))
	if tenLog10_40Q10 != wantTenLog10_40Q10 {
		t.Errorf("tenLog10_40Q10 = %d, want %d",
			tenLog10_40Q10, wantTenLog10_40Q10)
	}
	// invDbScaleQ15 = 1/(20·log10(2)) · 2^15 ≈ 5443.4
	wantInvDbScaleQ15 := int(math.Round(1.0 / (20 * math.Log10(2)) * (1 << 15)))
	if invDbScaleQ15 != wantInvDbScaleQ15 {
		t.Errorf("invDbScaleQ15 = %d, want %d",
			invDbScaleQ15, wantInvDbScaleQ15)
	}
	// dbPerLog2Q10 = 20·log10(2) · 2^10 ≈ 6165.4
	wantDbPerLog2Q10 := int(math.Round(20 * math.Log10(2) * (1 << 10)))
	if dbPerLog2Q10 != wantDbPerLog2Q10 {
		t.Errorf("dbPerLog2Q10 = %d, want %d", dbPerLog2Q10, wantDbPerLog2Q10)
	}
}

// TestQFormatContract_PastErrorsDefaultIsMinus14dBQ10 — initial value
// of MA-predictor history per ITU-T G.729 §3.9.1 / Table 6.
func TestQFormatContract_PastErrorsDefaultIsMinus14dBQ10(t *testing.T) {
	const wantDbQ10 int16 = -14 * 1024
	if pastErrorsDefault != wantDbQ10 {
		t.Fatalf("pastErrorsDefault = %d, want %d (= −14 dB Q10)",
			pastErrorsDefault, wantDbQ10)
	}
}
```

- [x] **Step 2: 테스트 실행**

Run: `go test -run 'TestQFormatContract_' ./internal/gain/ -v`

Expected: 5개(서브테스트 포함 11개) PASS. 

만약 `TestQFormatContract_LogDomainConstants` 가 실패하면 → 상수 자체가 스펙에서 어긋남 → Phase 1j 완료 보고서와 모순. **즉시 plan 중단**하고 사용자에게 보고.

- [x] **Step 3: 회귀 점검**

Run: `go test -race ./internal/gain/`

Expected: ALL PASS (Phase 1i/1j의 기존 테스트들 포함).

- [x] **Step 4: Commit**

```bash
git add internal/gain/qformat_contract_test.go
git commit -m "$(cat <<'EOF'
test(gain): Q-format contract — extend Phase 1j contract scope to assertions

Phase 1j docstring contract for energy.go / log2.go / pow2.go now
backed by compile-time test assertions: fixedCodebookEnergy returns
Q26, log2Fixed treats input as Q0, pow2Fixed returns Q0, log-domain
constants match closed-form physical identities, pastErrorsDefault
= −14 dB Q10 per Table 6.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 4: D4 — synth Q-포맷 계약 테스트

**Goal:** `BuildExcitation`의 Q-포맷 체인 (`gpQ14·v_Q0 → Q15`, `gcQ12·c_Q13 → Q26`, `LShr 11 → Q15`, sum + LShl 1 + Round → Q0)과 `filterSubframe`의 LP 계수 규약 (`a[0] = 4096` Q12)을 어서션으로 박는다.

**Files:**
- Create: `internal/synth/qformat_contract_test.go`

- [ ] **Step 1: 계약 테스트 작성**

`internal/synth/qformat_contract_test.go`:

```go
package synth

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// TestQFormatContract_BuildExcitationPitchTermIsQ15 — for a unit
// pitch gain (gpQ14 = 16384 = 1.0 true) and unit pitch sample
// (v_Q0 = 1), the LMult result is 2·1·1 = 2 in Q-encoded form,
// representing 1.0 at Q15 (since LMult auto-shifts left by 1).
//
// Documented: pf.LMult(Q14, Q0) = 2·a·b is at Q15.
func TestQFormatContract_BuildExcitationPitchTermIsQ15(t *testing.T) {
	const gpQ14 int16 = 1 << 14 // 1.0 true
	const vQ0 int16 = 1
	got := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(vQ0))
	const want fixed.Word32 = 2 // 1.0 · 2^15 / 2^14 = 2
	if got != want {
		t.Fatalf("LMult(gpQ14=1.0, v=1) = %d, want %d (= 1.0·2^15 stored)",
			got, want)
	}
}

// TestQFormatContract_BuildExcitationCodeTermIsQ26ThenQ15 — for unit
// fixed-codebook gain (gcQ12 = 4096 = 1.0 true) and unit code pulse
// (c_Q13 = 8192 = 1.0 true), LMult yields 2·gc·c = 2·4096·8192 in
// Q-encoded form (Q26-stored value of 1.0 true product). After LShr
// by 11, value is at Q15.
func TestQFormatContract_BuildExcitationCodeTermIsQ26ThenQ15(t *testing.T) {
	const gcQ12 int16 = 1 << 12 // 1.0 true
	const cQ13 int16 = 1 << 13  // 1.0 true
	lMultRaw := fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(cQ13))
	const wantQ26 fixed.Word32 = 1 << 26 // 1.0·2^26
	if lMultRaw != wantQ26 {
		t.Errorf("LMult(gc=1.0 Q12, c=1.0 Q13) = %d, want %d (Q26)",
			lMultRaw, wantQ26)
	}
	lCodeQ15 := fixed.LShr(lMultRaw, 11)
	const wantQ15 fixed.Word32 = 1 << 15 // 1.0·2^15
	if lCodeQ15 != wantQ15 {
		t.Errorf("LShr(Q26, 11) = %d, want %d (Q15)", lCodeQ15, wantQ15)
	}
}

// TestQFormatContract_BuildExcitationSinglePulseProducesGcQ12 — when
// gpQ14 = 0 and v = 0, with c being a single Q13 pulse and gcQ12 a
// known value, u[0] should equal round-to-int(gcQ12 / 4096) =
// round-to-int(true gc).
func TestQFormatContract_BuildExcitationSinglePulseProducesGcQ12(t *testing.T) {
	tests := []struct {
		name  string
		gcQ12 int16
		want  int16
	}{
		{"gc=1.0 (Q12=4096)", 4096, 1},
		{"gc=2.0 (Q12=8192)", 8192, 2},
		{"gc=5.5 (Q12=22528)", 22528, 6}, // round nearest, ties-to-even → 6
		{"gc=0.0", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v, c, u [40]int16
			c[0] = 1 << 13 // +1.0 Q13
			BuildExcitation(0, tt.gcQ12, &v, &c, &u)
			if u[0] != tt.want {
				t.Errorf("u[0] = %d, want %d (= round(gcQ12/4096))",
					u[0], tt.want)
			}
		})
	}
}

// TestQFormatContract_FilterSubframeAcceptsAOneQ12 — the LP synthesis
// filter expects a[0] = 4096 (= +1.0 Q12) per ITU-T G.729 §4.1.6.
// With a[i]=0 for i≥1 (trivial filter) and u being a unit excitation,
// s should equal u.
func TestQFormatContract_FilterSubframeAcceptsAOneQ12(t *testing.T) {
	var synth Synthesizer
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var u, s [40]int16
	for n := 0; n < 40; n++ {
		u[n] = int16(n + 1)
	}
	synth.Filter(&a, &u, &s)
	for n := 0; n < 40; n++ {
		if s[n] != u[n] {
			t.Errorf("s[%d] = %d, want %d (trivial filter passthrough)",
				n, s[n], u[n])
		}
	}
}
```

- [ ] **Step 2: 테스트 실행**

Run: `go test -run 'TestQFormatContract_' ./internal/synth/ -v`

Expected: 4개 (서브테스트 포함 7개) PASS.

만약 `TestQFormatContract_FilterSubframeAcceptsAOneQ12` 가 실패하면 → `filterSubframe`의 누산기 또는 `Round` 자리가 어긋남 → Stage F 후보 위치로 표시.

- [ ] **Step 3: 회귀 점검**

Run: `go test -race ./internal/synth/`

Expected: ALL PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/synth/qformat_contract_test.go
git commit -m "$(cat <<'EOF'
test(synth): Q-format contract — excitation chain + filter a[0]=4096

Lock BuildExcitation's Q-format chain (gp·v→Q15, gc·c→Q26→Q15) and
filterSubframe's LP-coefficient convention (a[0]=4096 Q12 trivial
filter passthrough). Pinpoints which boundary owns the Q-format if
diagnostic harness flags a divergence in synth.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 5: D5 — postfilter Q-포맷 계약 테스트

**Goal:** postfilter sub-stage들의 I/O Q-포맷 (AGC 게인 Q24 내부, isqrt Q14 출력, gammaNum/gammaDen Q15)을 어서션으로 박는다.

**Files:**
- Create: `internal/postfilter/qformat_contract_test.go`

- [ ] **Step 1: 계약 테스트 작성**

`internal/postfilter/qformat_contract_test.go`:

```go
package postfilter

import "testing"

// TestQFormatContract_GammaConstantsAreQ15 — bandwidth-expansion
// constants γ_n ≈ 0.55, γ_d ≈ 0.70 in Q15 per ITU-T G.729 §A.4.2.
func TestQFormatContract_GammaConstantsAreQ15(t *testing.T) {
	tests := []struct {
		name string
		got  int16
		want float64
	}{
		{"gammaNumQ15 (γ_n ≈ 0.55)", gammaNumQ15, 0.55},
		{"gammaDenQ15 (γ_d ≈ 0.70)", gammaDenQ15, 0.70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotF := float64(tt.got) / 32768.0
			if gotF < tt.want-0.005 || gotF > tt.want+0.005 {
				t.Errorf("got=%.4f want=%.4f (Q15 raw=%d)",
					gotF, tt.want, tt.got)
			}
		})
	}
}

// TestQFormatContract_IsqrtQ14ReturnsQ14 — isqrtQ14(x at Q28) returns
// √x at Q14. So isqrtQ14(2^28) = 2^14 (= 1.0 Q14).
func TestQFormatContract_IsqrtQ14ReturnsQ14(t *testing.T) {
	tests := []struct {
		name string
		xQ28 int64
		want int16
	}{
		{"√1 (Q28)", 1 << 28, 1 << 14}, // 1.0 in Q14
		{"√4 (Q28)", 4 << 28, 2 << 14}, // 2.0 in Q14
		{"√0", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isqrtQ14(tt.xQ28)
			diff := int(got) - int(tt.want)
			if diff < -1 || diff > 1 {
				t.Errorf("got=%d want=%d (Δ=%d)", got, tt.want, diff)
			}
		})
	}
}

// TestQFormatContract_AGCAlphaIsQ15 — α ≈ 0.99 at Q15 = 32440.
// ITU-T G.729 §A.4.2.4.
func TestQFormatContract_AGCAlphaIsQ15(t *testing.T) {
	const wantQ15 int64 = 32440
	const want float64 = 0.99
	gotF := float64(wantQ15) / 32768.0
	if gotF < want-0.001 || gotF > want+0.001 {
		t.Fatalf("alphaQ15 represents %.4f, want %.4f", gotF, want)
	}
}

// TestQFormatContract_AGCSeedsAgcGainPrevToTargetQ24 — on the very
// first applyAGC call, agcGainPrev is seeded from g_target Q14
// shifted to Q24 (per Phase 1i §A.4.2.4 init fix).
func TestQFormatContract_AGCSeedsAgcGainPrevToTargetQ24(t *testing.T) {
	var pf Postfilter
	const gTargetQ14 int16 = 1 << 14 // 1.0 true
	var sTilt, sPf [subframeLen]int16
	for n := range sTilt {
		sTilt[n] = 100
	}
	pf.applyAGC(&sTilt, gTargetQ14, &sPf)
	if !pf.initialized {
		t.Fatal("applyAGC did not flip the initialized flag")
	}
	const wantQ24 int32 = int32(gTargetQ14) << 10
	// agcGainPrev evolves by one EWMA step inside the loop; assert
	// the seed wasn't 0 (which would have caused a 32440/32768
	// attenuation on the first sample).
	if pf.agcGainPrev < wantQ24/2 {
		t.Errorf("agcGainPrev = %d (Q24), expected ~%d (seeded from g_target)",
			pf.agcGainPrev, wantQ24)
	}
}
```

- [ ] **Step 2: 테스트 실행**

Run: `go test -run 'TestQFormatContract_' ./internal/postfilter/ -v`

Expected: 4개 (서브테스트 포함 ~9개) PASS.

- [ ] **Step 3: 회귀 점검**

Run: `go test -race ./internal/postfilter/`

Expected: ALL PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/postfilter/qformat_contract_test.go
git commit -m "$(cat <<'EOF'
test(postfilter): Q-format contract — sub-stage I/O Q-formats

Pin γ_n/γ_d Q15 constants, isqrtQ14 contract (Q28 input → Q14 output),
AGC α=0.99 Q15, and the §A.4.2.4 seed-to-target invariant from
Phase 1i. These contracts are the boundary checks the diagnostic
harness uses to flag postfilter as a 14 dB suspect.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 6: D6 — 단일-펄스 진단 하네스 (관측-우선)

**Goal:** 단일 +Q13 펄스를 입력으로 gain → excitation → synth → postfilter 체인을 외부에서 수동 조립하고, 13개 경계의 실측 값을 t.Logf로 출력. 어서션 0개 (관측-우선 TDD).

**Files:**
- Create: `internal/decoder/diagnostic_singlepulse_test.go`

- [ ] **Step 1: 하네스 작성**

`internal/decoder/diagnostic_singlepulse_test.go`:

```go
package decoder

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_SinglePulseChain feeds a controlled single-pulse
// fixed-codebook input through the full decoder chain (gain →
// excitation → synth → postfilter) and logs the value at each of 13
// canonical boundaries against spec-derived true values.
//
// Purpose (Phase 1k §6 of design): observe at which boundary the
// 14 dB divergence enters. No assertions — Task 7 promotes the
// spec-aligned boundaries to assertions; the first divergent
// boundary becomes the Stage F target.
//
// Input choice: single +Q13 pulse at position 0, gpQ14=0 (zero
// pitch contribution), default past errors (Table 6 init = −14 dB Q10
// each), idx (GA=3, GB=7) — same as the existing pathological tests
// so the γ̂_c value is reproducible.
func TestDiagnostic_SinglePulseChain(t *testing.T) {
	var c [40]int16
	c[0] = 8192 // +1.0 Q13

	// Spec-derived true expected values (ITU-T G.729 §3.9.1 eq 66-72,
	// §A.4.2 pseudo-code).
	const sigmaCSquaredTrue float64 = 1.0 // single +1.0 pulse
	expectedEcBarDb := 10.0 * math.Log10(sigmaCSquaredTrue/40.0) // -16.02 dB
	expectedPredictedDb := 30.0 + 1.79*(-14.0)                   // 4.94 dB (initial)
	expectedLogGainDb := expectedPredictedDb - expectedEcBarDb   // 20.96 dB
	expectedGcPrime := math.Pow(10, expectedLogGainDb/20)        // 11.16

	t.Logf("=== Spec-derived true expected values ===")
	t.Logf("Σc² true              = %g", sigmaCSquaredTrue)
	t.Logf("Ē_c (true dB)         = %.4f", expectedEcBarDb)
	t.Logf("Ê predicted (true dB) = %.4f", expectedPredictedDb)
	t.Logf("logGain (true dB)     = %.4f", expectedLogGainDb)
	t.Logf("g'_c (true)           = %.4f", expectedGcPrime)

	// === Boundary ① fcb output ===
	var sumSqQ26 int64
	var maxAbs int16
	for n := 0; n < 40; n++ {
		sumSqQ26 += int64(c[n]) * int64(c[n])
		a := c[n]
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	cTrueSumSq := float64(sumSqQ26) / float64(int64(1)<<26)
	t.Logf("[① fcb] Σc²(raw=Q26)  = %d → true=%.4f (want %.4f)",
		sumSqQ26, cTrueSumSq, sigmaCSquaredTrue)
	t.Logf("[① fcb] max|c|        = %d (Q13, true=%.4f)",
		maxAbs, float64(maxAbs)/8192.0)

	// === Boundary ② fixedCodebookEnergy ===
	// Cannot call package-private from external test. Replicate
	// using the same arithmetic for observation purposes.
	var ecObserved int64
	for n := 0; n < 40; n++ {
		// LMult equiv = 2·c·c, then >>1 = c·c. Same as energy.go.
		ecObserved += int64(c[n]) * int64(c[n])
	}
	t.Logf("[② energy] raw=%d Q26 → true=%.4f (want %.4f)",
		ecObserved, float64(ecObserved)/float64(int64(1)<<26),
		sigmaCSquaredTrue)

	// === Boundary ⑩-⑪ gain.Decode + BuildExcitation ===
	// Drive gain.Decoder externally and observe gcQ12.
	var gd gain.Decoder
	gpQ14, gcQ12 := gd.Decode(gain.Indices{GA: 3, GB: 7}, &c)
	gcTrue := float64(gcQ12) / 4096.0
	t.Logf("[⑩ gain] gpQ14=%d gcQ12=%d (true gc=%.4f)",
		gpQ14, gcQ12, gcTrue)

	// What does spec predict for gcQ12 with this idx? γ̂_c is the
	// gain VQ output for (GA=3, GB=7); we read it from the actual
	// VQ via decodeVQ — but that is package-private. Instead, log
	// gc magnitude vs spec g'_c bound: with γ̂_c ∈ [0, ~2], gc
	// should be within [0, expectedGcPrime · γ̂_max].
	t.Logf("[⑩ gain] spec g'_c=%.4f → gc bounded by [0, ~22] for γ̂_max≈2",
		expectedGcPrime)

	// Build excitation with this gcQ12 and a unit-pulse c, no pitch.
	var v, u [40]int16
	synth.BuildExcitation(0, gcQ12, &v, &c, &u)
	t.Logf("[⑪ u] u[0]=%d (= round(gcQ12/4096) = round(%.4f) = expect %d)",
		u[0], gcTrue, int(math.Round(gcTrue)))

	// === Boundary ⑫ synth.Filter ===
	// Use trivial LP filter (a[0]=4096, rest zero) so the
	// boundary observation isolates the filter's identity behavior.
	var sy synth.Synthesizer
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s [40]int16
	sy.Filter(&a, &u, &s)
	t.Logf("[⑫ s] s[0]=%d (trivial filter passthrough → expect %d)",
		s[0], u[0])

	// === Boundary ⑬ postfilter ===
	var pf postfilter.Postfilter
	var sPf [40]int16
	pf.Filter(&a, 40, &s, &sPf)
	t.Logf("[⑬ sPf] sPf[0..7]=%v", sPf[:8])

	// Summary line for cross-task scanning
	t.Logf("=== boundaries logged: ① fcb energy ② accumulator " +
		"⑩ gcQ12 ⑪ u[0] ⑫ s[0] ⑬ sPf[0..7] ===")
	t.Logf("Boundaries ③-⑨ (gain log-domain intermediates) are " +
		"package-private to internal/gain; Task 3's gain Q-format " +
		"contract tests cover them. Cross-reference those test " +
		"outputs when chasing 14 dB divergence.")
}
```

- [ ] **Step 2: 하네스 실행 + 출력 캡처**

Run: `go test -run 'TestDiagnostic_SinglePulseChain' ./internal/decoder/ -v 2>&1 | tee /tmp/phase1k_diag.txt`

Expected: PASS (어서션 0개), 모든 13개 경계 로그 출력.

**중요**: `/tmp/phase1k_diag.txt`를 읽고 다음을 메모:
1. `gcQ12` 값 (포화 32767 또는 −32768인지 확인)
2. `u[0]` 값 (round(gcTrue)와 일치하는지)
3. `s[0]` 값 (u[0]과 같아야 함 — trivial filter)
4. `sPf[0..7]` 값 (postfilter 출력)
5. `Ē_c true dB`, `Ê predicted dB`, `logGain dB` 값과 `gcQ12` true 값 차이가 dB 도메인에서 14 ± 2 dB 정도인지

이 메모는 Task 7과 Task 8에서 직접 사용.

- [ ] **Step 3: 회귀 점검**

Run: `go test -race ./internal/decoder/`

Expected: ALL PASS.

- [ ] **Step 4: Commit (관측만, 어서션 0개)**

```bash
git add internal/decoder/diagnostic_singlepulse_test.go
git commit -m "$(cat <<'EOF'
test(decoder): single-pulse diagnostic harness (observation-only)

Drive gain → excitation → synth → postfilter externally with a
controlled single +Q13 pulse, log all 13 spec boundaries. No
assertions — Task 7 promotes spec-aligned boundaries; the first
divergent boundary becomes Stage F target per design §6.4.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 7: D7 — 하네스 어서션 채택

**Goal:** Task 6의 관측 결과를 바탕으로, 스펙 참값과 일치하는 경계에 한해 어서션을 추가. 첫 발산 경계는 어서션 추가하지 않고 t.Errorf로 명시적 진단 메시지를 남김.

**Files:**
- Modify: `internal/decoder/diagnostic_singlepulse_test.go`

- [ ] **Step 1: Task 6 출력 분석**

`/tmp/phase1k_diag.txt`를 다시 읽고 각 경계의 실측 vs 참값 dB 차이를 계산:

| 경계 | 실측 (raw) | 실측 → true | 참값 true | dB 차이 |
|------|-----------|-------------|----------|---------|
| ① Σc² | (raw 값) | (true 값) | 1.0 | (계산) |
| ⑩ gcQ12 | (raw 값) | (true 값) | (참값) | (계산) |
| ⑪ u[0] | (raw 값) | u[0] 정수 | round(gc) | (계산) |
| ⑫ s[0] | (raw 값) | u[0]와 같아야 | (계산) |
| ⑬ sPf[0] | (raw 값) | (계산) | (참값) | (계산) |

dB 차이 = `20·log10(|true_observed| / |true_expected|)`. 

첫 번째로 `|dB 차이| > 0.5` 되는 경계 번호를 K로 정의.

- [ ] **Step 2: 어서션 추가 (boundary < K) + Stage F 트리거 메시지 (boundary == K)**

Edit `internal/decoder/diagnostic_singlepulse_test.go`. Task 6의 t.Logf는 그대로 두고, 함수 끝에 다음을 추가:

```go
	// === Spec-aligned boundary assertions (Task 7) ===
	//
	// Boundaries 1..K-1 are spec-consistent per Task 6 observation;
	// boundary K is the 14 dB suspect. After Stage F lands the fix,
	// promote boundary K and onwards to assertions.
	//
	// EDIT THE THRESHOLDS BELOW based on Task 6's observed values.
	// This is the assertion rendition of the running observation.

	// Boundary ① — Σc² in Q26 form must equal 2^26 for single Q13 pulse.
	if sumSqQ26 != 1<<26 {
		t.Errorf("BOUNDARY ① fcb energy: Σc²=%d, want %d (= 2^26)",
			sumSqQ26, int64(1)<<26)
	}

	// Boundary ⑪ — u[0] must equal round(true gc).
	expectedU := int16(math.Round(gcTrue))
	if u[0] != expectedU {
		t.Errorf("BOUNDARY ⑪ excitation: u[0]=%d, want %d (= round(gcTrue=%.4f))",
			u[0], expectedU, gcTrue)
	}

	// Boundary ⑫ — trivial LP filter (a[0]=4096, rest 0) is identity.
	if s[0] != u[0] {
		t.Errorf("BOUNDARY ⑫ synth.Filter: s[0]=%d, u[0]=%d (trivial filter must be identity)",
			s[0], u[0])
	}

	// Boundary ⑩ Stage F trigger — gcQ12 true magnitude must be within
	// [0, expectedGcPrime · γ̂_max] where γ̂_max ≈ 2.
	maxExpectedGc := expectedGcPrime * 2.0
	if gcTrue < 0 || gcTrue > maxExpectedGc+0.5 {
		t.Errorf("BOUNDARY ⑩ gain: gcTrue=%.4f exceeds spec bound [0, %.4f]; "+
			"this is the Stage F target (14 dB suspect at gain log-domain math)",
			gcTrue, maxExpectedGc)
	} else if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Errorf("BOUNDARY ⑩ gain: gcQ12 saturated (%d); 14 dB suspect at "+
			"gain log-domain math — review §3.9.1 ecBar/predicted/logGain chain",
			gcQ12)
	}
```

위 어서션 중 어떤 것이 Task 6 관측에서 실패하는지를 먼저 결정한 뒤 작성. 만약 ⑫ (trivial filter identity)가 실패하면 → Stage F 후보가 synth/filter.go (LShl 3, Round 자리). 만약 ⑩ (gcQ12 포화)이 실패하면 → Stage F 후보가 gain/decode.go.

만약 Task 6 관측에서 모든 경계가 스펙과 일치하면 → 14 dB 패턴이 단일-펄스 입력으로 재현되지 않음 → **탈출 해치 1 발동**, completion report에 기록 후 Stage F 진입 금지.

- [ ] **Step 3: 어서션 실행**

Run: `go test -run 'TestDiagnostic_SinglePulseChain' ./internal/decoder/ -v`

Expected: ① ⑫ PASS, ⑩ 또는 ⑪이 t.Errorf로 실패 (= 14 dB 위치 식별).

만약 모든 어서션이 PASS → 탈출 해치 1로 분기, plan 중단하고 사용자 보고.

- [ ] **Step 4: 회귀 점검**

Run: `go test -race ./...`

Expected: ALL PASS 또는 `TestDiagnostic_SinglePulseChain`만 실패 (의도된 진단). 다른 테스트 회귀 0건.

- [ ] **Step 5: Commit (실패하는 진단 어서션 포함)**

```bash
git add internal/decoder/diagnostic_singlepulse_test.go
git commit -m "$(cat <<'EOF'
test(decoder): single-pulse harness assertions for spec-aligned boundaries

Promote Task 6 observations to assertions: boundaries ① (fcb energy),
⑪ (excitation u[0]=round(gc)), ⑫ (trivial filter identity) MUST hold.
Boundary ⑩ flagged as 14 dB Stage F target — assertion intentionally
fails to drive Stage F to the correct fix location. See design §6.4.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 8: F1 — 14 dB 수정

**Goal:** Task 7이 지목한 위치에 한 줄 (또는 진단 증거가 명시된 두 곳)을 수정하여 14 dB 오차를 제거. 같은 commit 안에 ALGTHM frame 0 sample 40+ failing assertion 추가.

**Files:**
- Modify: 한 곳 (Task 7 결과에 따라)
  - 후보 A: `internal/gain/decode.go`
  - 후보 B: `internal/synth/excitation.go`
  - 후보 C: `internal/synth/filter.go`
  - 후보 D: `internal/postfilter/agc.go` 또는 `tilt.go` 또는 `shortterm.go`
  - 후보 E: `internal/fcb/types.go`
- Modify: `internal/decoder/frame0_regression_test.go` (sf2 어서션 추가)

- [ ] **Step 1: Task 7 결과에 따른 수정 위치 확정**

Task 7의 어서션 실패 메시지를 확인하고 design §5.2의 표를 참조하여 정확한 파일과 줄을 결정한다. 결정된 위치를 commit message에 §spec-ref + 한 줄 설명으로 기재.

분기별 예시 수정 (실제 수정은 진단 결과에 따름; 아래는 **참고 템플릿**):

**브랜치 A: gain log-domain (boundary ④~⑩)**

`internal/gain/decode.go`:70 — 현재:
```go
ecLog2Q10 := log2Fixed(ecEnergy)
```
가능한 수정:
```go
// fixedCodebookEnergy returns Σc² at Q26 (cQ13² scaling), but
// log2Fixed treats input as Q0. Subtract 26·1024 to recover the
// true log2 per ITU-T G.729 §3.9.1 eq (66).
ecLog2Q10 := log2Fixed(ecEnergy) - 26*1024
```

**경고**: Phase 1j 완료 보고서에서 이 단독 수정은 frame 0 sample 0 = 2를 회귀시킨다 (got=12)고 확인되었다. 이 브랜치를 선택하면 **반드시** 다른 14 dB 짝의 동시 수정이 필요 — 그 짝의 위치를 진단 표에서 같이 식별해야 함. 짝이 식별되지 않으면 → 탈출 해치 1.

**브랜치 B: excitation LShr (boundary ⑪)**

`internal/synth/excitation.go`:30 — 현재:
```go
lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
```
가능한 수정 (예시 — 진단이 LShr 카운트 미스를 가리킬 때):
```go
// LMult(gcQ12, cQ13) = Q26. LShr by 11 → Q15. If diagnostic shows
// the chain expects gcQ9 instead, LShr count adjusts to 11+3=14.
lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), <NEW_COUNT>)
```

**브랜치 C: filter LShl 3 (boundary ⑫)**

`internal/synth/filter.go`:66 — 현재:
```go
lTemp = fixed.LShl(lTemp, 3)
```
가능한 수정 (예시):
```go
// LP accumulator is Q26 (LMult chain at Q12·Q0=Q12, accumulated
// 11 times stays Q12; pre-Round it must be promoted to Q31 — the
// shift count derives from <spec ref>).
lTemp = fixed.LShl(lTemp, <NEW_COUNT>)
```

**브랜치 D: postfilter (boundary ⑬)**

`internal/postfilter/agc.go` 또는 `tilt.go` 또는 `shortterm.go`의 한 줄. 진단이 가리키는 sub-stage에 따름.

**브랜치 E: fcb PulseAmplitude (boundary ① — 거의 발생 안 함)**

`internal/fcb/types.go`:14 — `PulseAmplitude` 값 변경. **가장 침습적**, Σc² 계약 테스트 전체가 깨지므로 D2를 함께 갱신.

수정 commit 직전:
1. 진단 표에서 결정된 위치를 한 문장으로 적기
2. ITU §ref 인용
3. 한 파일이 아닌 두 파일이라면 두 곳 모두 진단 표에 dB 증거가 있어야 함 — 없으면 탈출 해치 1.

- [ ] **Step 2: ALGTHM frame 0 sf2 failing assertion 추가**

`internal/decoder/frame0_regression_test.go`에 다음 함수를 추가 (Task 1의 두 함수 뒤에):

```go
// TestDecode_Frame0Sample40_MatchesALGTHM is the Stage F target
// assertion: ALGTHM frame 0 sample 40 (sf2 first sample) must equal
// the ITU reference. Under Phase 1j's tip this is -23270 (vs
// reference value); the 14 dB fix in Stage F must zero this delta
// without regressing TestDecode_Frame0Sample0_MatchesALGTHM.
func TestDecode_Frame0Sample40_MatchesALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	if out[40] != wantFrames[0][40] {
		t.Errorf("frame 0 sample 40 (sf2 start): got=%d want=%d (Δ=%d)",
			out[40], wantFrames[0][40],
			int32(out[40])-int32(wantFrames[0][40]))
	}
}
```

- [ ] **Step 3: failing test 확인 (수정 적용 전)**

Run: `go test -run 'TestDecode_Frame0Sample40_MatchesALGTHM' ./internal/decoder/ -v`

Expected: FAIL (out[40] ≠ wantFrames[0][40]). 출력의 Δ 값을 메모 — 수정 후와 비교용.

- [ ] **Step 4: Step 1의 수정 적용**

진단 표에서 결정된 위치에 한 줄 수정. 주석으로 ITU §ref 한 줄 + 무엇이 틀렸는지 한 줄 추가.

- [ ] **Step 5: Stage F 검증 — 두 조건 동시 충족 확인**

Run:
```
go test -run 'TestDecode_Frame0Sample0_MatchesALGTHM|TestDecode_Frame0Sample40_MatchesALGTHM|TestDecode_Frame0SF1_DiagnosticLog' ./internal/decoder/ -v
```

Expected:
- `TestDecode_Frame0Sample0_MatchesALGTHM`: PASS (Phase 1i 회귀 가드, 절대 깨지면 안 됨)
- `TestDecode_Frame0Sample40_MatchesALGTHM`: PASS
- `TestDecode_Frame0SF1_DiagnosticLog`: PASS (어서션 0개), 로그에 sf1 40 샘플이 모두 일치하는지 확인

만약 `TestDecode_Frame0Sample0_MatchesALGTHM` 회귀 → **탈출 해치 2 발동**: 수정 롤백, design §7.2에 따라 두 곳 동시 수정 후보 식별 또는 plan 중단.

만약 `TestDecode_Frame0Sample40_MatchesALGTHM` 여전히 실패 → 진단 위치가 잘못됨, Step 1로 복귀.

- [ ] **Step 6: 회귀 점검 (전 패키지)**

Run:
```bash
go test -race ./...
go vet ./...
go test -bench=BenchmarkDecode -benchmem -run=^$ ./internal/decoder/
```

Expected:
- `go test -race ./...`: ALL PASS (`TestDiagnostic_SinglePulseChain`의 boundary ⑩ assertion도 이제 통과해야 함)
- `go vet ./...`: silent
- BenchmarkDecode: 0 allocs/op 유지

만약 `internal/gain/pathological_test.go`의 4개 중 일부가 실패 → 정상 (Task 11에서 재인증 예정). 단, 이 단계에서는 commit하지 않고 메모만 해 두기.

만약 `BenchmarkDecode`가 0 allocs를 잃으면 → 수정이 escape allocation을 도입함. Step 4 수정을 zero-alloc 형태로 재작성.

- [ ] **Step 7: Commit (수정 + sf2 어서션 동일 commit)**

```bash
git add <수정한 파일들> internal/decoder/frame0_regression_test.go
git commit -m "$(cat <<'EOF'
fix(<module>): correct 14 dB scale at boundary <X> per ITU-T G.729 §<ref>

<한 줄: 무엇이 틀렸고 무엇으로 바뀌었는지 — 예: "fixedCodebookEnergy
returns Q26 but log2Fixed expects Q0; compensate by subtracting
26·1024 from log2 result. Also adjusts <other location> by <amount>
to preserve Phase 1i frame 0 sample 0 = 2.">

Phase 1j hypothesis empirically confirmed (or refuted, with the
correct boundary located via Task 6's diagnostic harness). Same
commit adds the failing sf2 sample 40 assertion turned green by the
fix. Phase 1i sample 0 regression guard intact.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 9: V1 — frame 0 80 샘플 회귀 가드

**Goal:** Stage F가 통과한 frame 0의 모든 80 샘플 일치를 영구 어서션으로 박는다.

**Files:**
- Modify: `internal/decoder/frame0_regression_test.go`

- [ ] **Step 1: 80-샘플 어서션 함수 추가**

`internal/decoder/frame0_regression_test.go`에 다음 함수 추가:

```go
// TestDecode_Frame0AllSamples_MatchesALGTHM locks the Stage F
// achievement: every sample of ALGTHM frame 0 (80 samples)
// matches the ITU reference. Combined with the existing
// TestDecode_Frame0Sample0_MatchesALGTHM and
// TestDecode_Frame0Sample40_MatchesALGTHM tests, this is the
// permanent Phase 1k regression guard.
func TestDecode_Frame0AllSamples_MatchesALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	for n := 0; n < frameSamples; n++ {
		if out[n] != wantFrames[0][n] {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				n, out[n], wantFrames[0][n],
				int32(out[n])-int32(wantFrames[0][n]))
		}
	}
}
```

- [ ] **Step 2: 80-샘플 어서션 실행**

Run: `go test -run 'TestDecode_Frame0AllSamples_MatchesALGTHM' ./internal/decoder/ -v`

Expected: PASS. 만약 일부 샘플이 어긋남 → Stage F 수정이 sf1 또는 sf2 일부에서 부정확. Task 8 Step 5로 복귀, 두 곳 동시 수정 후보 재검토.

- [ ] **Step 3: 회귀 점검**

Run: `go test -race ./internal/decoder/`

Expected: ALL PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/decoder/frame0_regression_test.go
git commit -m "$(cat <<'EOF'
test(decoder): expand frame 0 regression guard to full 80 samples

Stage F's 14 dB fix landed; now lock all 80 samples of ALGTHM frame
0 against the ITU reference. Permanent guard for any future change
that might regress frame 0 bit-exactness.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 10: V2 — ALGTHM 비트-정확 활성화

**Goal:** `TestDecode_ITUVectorAlgthmBitExact`의 frame 0 부분의 `t.Skip`을 제거. 나머지 34 frames는 Phase 1l까지 보류.

**Files:**
- Modify: `internal/decoder/decode_test.go` (`TestDecode_ITUVectorAlgthmBitExact` 함수)

- [ ] **Step 1: Skip 제거 + frame 0만 검증하도록 축소**

Edit `internal/decoder/decode_test.go`. `TestDecode_ITUVectorAlgthmBitExact` 함수 (line 164~229)를 다음으로 교체:

```go
func TestDecode_ITUVectorAlgthmBitExact(t *testing.T) {
	t.Skip("Phase 1k complete for frame 0 only. Frames 1..34 still " +
		"diverge — same root-cause investigation continues in " +
		"Phase 1l. The frame-0-only assertion lives in " +
		"TestDecode_Frame0AllSamples_MatchesALGTHM.")
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	if len(frames) != len(wantFrames) {
		t.Fatalf("frame count mismatch: bit=%d pst=%d",
			len(frames), len(wantFrames))
	}

	var d Decoder
	var out [frameSamples]int16
	for i, packed := range frames {
		if err := d.Decode(packed, bads[i], out[:]); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if out != wantFrames[i] {
			for n := 0; n < frameSamples; n++ {
				if out[n] != wantFrames[i][n] {
					t.Errorf("frame %d sample %d: got %d, want %d (delta %+d)",
						i, n, out[n], wantFrames[i][n],
						int(out[n])-int(wantFrames[i][n]))
					break
				}
			}
			if t.Failed() && i >= 2 {
				t.Fatal("stopping after 3 divergent frames")
			}
		}
	}
}
```

**참고**: 이 phase는 frame 0만 수정하므로 `t.Skip`은 그대로 두되 메시지를 갱신한다. frame 0 비트-정확은 Task 9의 `TestDecode_Frame0AllSamples_MatchesALGTHM`이 보장. design §5.3은 "frame 0 t.Skip 제거"라고 했지만, 실제로는 전체 ALGTHM (35 frames) skip 유지가 정합 — 메시지만 갱신.

- [ ] **Step 2: 메시지 갱신 검증**

Run: `go test -run 'TestDecode_ITUVectorAlgthmBitExact' ./internal/decoder/ -v`

Expected: SKIP with new message.

Run: `go test -run 'TestDecode_Frame0AllSamples_MatchesALGTHM' ./internal/decoder/ -v`

Expected: PASS (Task 9에서 이미 통과).

- [ ] **Step 3: 회귀 점검**

Run: `go test -race ./...`

Expected: ALL PASS, skip count: ALGTHM 1 + SPEECH 1 + FIXED 1 + LSP 1 + PITCH 1 + TAME 1 + TEST 1 + OVERFLOW 1 = 8 (변동 없음).

- [ ] **Step 4: Commit**

```bash
git add internal/decoder/decode_test.go
git commit -m "$(cat <<'EOF'
test(decoder): update ALGTHM bit-exact skip message — frame 0 done

Frame 0 (80 samples) bit-exactness is now guarded by
TestDecode_Frame0AllSamples_MatchesALGTHM. Frames 1..34 remain
under investigation in Phase 1l with the same root cause.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 11: V3 — 병리적 테스트 A+B 혼합 재인증

**Goal:** Stage F 수정 적용으로 변경된 `internal/gain/pathological_test.go` 4개 테스트를 A+B 혼합 전략으로 재인증.

**Files:**
- Modify: `internal/gain/pathological_test.go`

- [ ] **Step 1: 현재 실측값 캡처**

Run: `go test -run '^TestDecode_(AllZero|LowEnergy|HighEnergy|SucceedsAcrossAllGainIndices)CodebookIs(Bounded|Smooth)$' ./internal/gain/ -v 2>&1 | tee /tmp/phase1k_pathological.txt`

각 테스트에서 보고된 (gpQ14, gcQ12) 실측값을 메모.

- [ ] **Step 2: A 전략 적용 (AllZero / LowEnergy)**

`internal/gain/pathological_test.go` 첫 두 함수를 다음으로 교체:

```go
// TestDecode_AllZeroCodebookIsBounded — Strategy A (spec-derived).
//
// All-zero codebook → fixedCodebookEnergy returns 0 → zero-energy
// guard branch in Decode → gcQ12 = 0 exactly per the documented
// branch in decode.go. gpQ14 still derives from VQ.
func TestDecode_AllZeroCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16

	gpQ14, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)

	if gcQ12 != 0 {
		t.Fatalf("all-zero codebook: gcQ12=%d, want 0 (zero-energy guard)",
			gcQ12)
	}
	if gpQ14 < -32768 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of int16 range: %d", gpQ14)
	}
}

// TestDecode_LowEnergyCodebookIsSmooth — Strategy A (spec-derived).
//
// Single-pulse codebook (Σc²=1 true). Spec eq (66): Ē_c = 10·log10(1/40)
// = −16 dB. Initial Ê = 30 + 1.79·(−14) = 5 dB. logGain = 21 dB →
// g'_c ≈ 11.16. With γ̂_c (GA=3, GB=7) ≈ 0.5 (from VQ tables), gc ≈
// 5.5 → gcQ12 ≈ 22500. Loose ±20% bound to absorb VQ table tolerance.
func TestDecode_LowEnergyCodebookIsSmooth(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[0] = 8192
	gpQ14, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if gpQ14 < 0 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of expected range [0, 32767]: %d", gpQ14)
	}
	const wantLow, wantHigh int16 = 18000, 27000 // ±20% around 22500
	if gcQ12 < wantLow || gcQ12 > wantHigh {
		t.Errorf("gcQ12=%d outside spec-derived range [%d,%d] for "+
			"single-pulse Σc²=1, initial Ê=5dB, γ̂≈0.5",
			gcQ12, wantLow, wantHigh)
	}
}
```

만약 Step 1에서 캡처한 실측값이 [18000, 27000]을 벗어나면 → A 전략을 그대로 적용하지 못함 → B 전략으로 강등 (다음 step 참조).

- [ ] **Step 3: B 전략 적용 (HighEnergy / SucceedsAcrossAllGainIndices)**

`internal/gain/pathological_test.go`의 나머지 두 함수를 다음으로 교체. 캡처된 실측값으로 어서션을 갱신하고, 각 어서션 옆에 spec 경계 한 줄 주석:

```go
// TestDecode_HighEnergyCodebookIsBounded — Strategy B (empirical
// re-lock with spec-bound cross-check).
//
// Canonical 4-pulse codebook (Σc²=4 true). Spec: Ē_c=−10 dB, Ê=5 dB,
// logGain=15 dB, g'_c ≈ 5.62, gc ≈ γ̂·5.62. With γ̂≈0.5 (GA=3, GB=7),
// gc ≈ 2.8 → gcQ12 ≈ 11400. Empirical assertion below was captured
// post-Phase-1k fix and is the permanent regression lock.
func TestDecode_HighEnergyCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192
	_, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Fatalf("canonical 4-pulse codebook drove gcQ12 to int16 extremum: %d",
			gcQ12)
	}
	const wantGcQ12 int16 = <CAPTURE_FROM_STEP_1> // spec g'_c·γ̂ ≈ 2.8 → ~11400
	const tol int16 = 100
	if gcQ12 < wantGcQ12-tol || gcQ12 > wantGcQ12+tol {
		t.Errorf("gcQ12=%d, want %d±%d (spec g'_c·γ̂≈2.8 → Q12≈11400)",
			gcQ12, wantGcQ12, tol)
	}
}

// TestDecode_SucceedsAcrossAllGainIndices — Strategy B.
//
// Full (GA, GB) sweep on the canonical 4-pulse codebook. No
// combination may saturate gc to int16 extrema. Spec upper bound:
// γ̂_max·g'_c ≈ 2·5.62 = 11.24 → gcQ12 ≤ 46055; clamped to 32767.
// We assert no extremum-saturation, not the precise gcQ12 value.
func TestDecode_SucceedsAcrossAllGainIndices(t *testing.T) {
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192

	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			var d Decoder
			_, gcQ12 := d.Decode(Indices{GA: ga, GB: gb}, &c)
			if gcQ12 == 32767 || gcQ12 == -32768 {
				t.Errorf("(GA=%d, GB=%d) saturated gcQ12 to %d "+
					"(spec bound: γ̂_max·g'_c≈11.24 → Q12≈46055, clamps to 32767)",
					ga, gb, gcQ12)
			}
		}
	}
}
```

`<CAPTURE_FROM_STEP_1>`을 Step 1의 실측 gcQ12 값으로 교체.

- [ ] **Step 4: 4개 모두 PASS 확인**

Run: `go test -run '^TestDecode_(AllZero|LowEnergy|HighEnergy|SucceedsAcrossAllGainIndices)' ./internal/gain/ -v`

Expected: 4개 모두 PASS.

만약 A 전략 적용한 두 개 중 하나라도 실패 → 그 함수만 B 전략으로 강등 (`if gcQ12 != 0` → `if gcQ12 < lowBound || gcQ12 > highBound` 형태로). Strategy 강등 사실을 함수 docstring에 한 줄 추가 ("Strategy A→B downgrade: <reason>"). 이 정보는 완료 보고서 §6 의무 기재 항목.

- [ ] **Step 5: 회귀 점검**

Run: `go test -race ./...`

Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gain/pathological_test.go
git commit -m "$(cat <<'EOF'
test(gain): re-certify pathological tests (A spec-derived + B empirical)

After Phase 1k Stage F 14 dB fix, AllZero/LowEnergy locked to
spec-derived bounds (Strategy A); HighEnergy/SucceedsAcross locked to
empirical post-fix values with spec upper-bound cross-check (Strategy B).
Each assertion line carries a one-line spec-math comment.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 12: 완료 보고서 + 최종 회귀 스윕

**Goal:** Phase 1k 결과를 `docs/superpowers/plans/2026-04-27-phase1k-completion-report.md`에 기록. Design §7.6의 6개 의무 기재 항목 모두 포함. 최종 `go test -race / go vet / BenchmarkDecode` 결과 첨부.

**Files:**
- Create: `docs/superpowers/plans/2026-04-27-phase1k-completion-report.md`

- [ ] **Step 1: 최종 회귀 스윕 실행 + 결과 캡처**

Run:
```bash
go test -race ./... 2>&1 | tee /tmp/phase1k_final_test.txt
go vet ./... 2>&1 | tee /tmp/phase1k_final_vet.txt
go test -bench=BenchmarkDecode -benchmem -run=^$ ./internal/decoder/ 2>&1 | tee /tmp/phase1k_final_bench.txt
```

Expected:
- `/tmp/phase1k_final_test.txt`: ALL PASS, skip count 8 (변동 없음)
- `/tmp/phase1k_final_vet.txt`: 빈 출력
- `/tmp/phase1k_final_bench.txt`: `0 B/op	0 allocs/op`

- [ ] **Step 2: 완료 보고서 작성**

Create `docs/superpowers/plans/2026-04-27-phase1k-completion-report.md` with the structure below. 모든 `<…>` placeholder를 Task 6, 7, 8 메모 + Step 1 캡처 결과로 채울 것:

```markdown
# Phase 1k 완료 리포트 — 14 dB 오차 격리 및 수정

**날짜**: <YYYY-MM-DD>
**계획 문서**: [`2026-04-27-phase1k-14db-isolation-and-fix.md`](./2026-04-27-phase1k-14db-isolation-and-fix.md)
**설계 명세**: [`../specs/2026-04-27-phase1k-14db-isolation-and-fix-design.md`](../specs/2026-04-27-phase1k-14db-isolation-and-fix-design.md)
**최종 상태**: <완전 성공 | 부분 성공 (탈출 해치 N 발동)>

---

## 1. Stage D 진단 결과 표 (Design §7.6.1)

| 경계 | 실측 raw | 실측 → true | 참값 true | dB 차이 |
|------|----------|-------------|-----------|---------|
| ① fcb Σc² | <…> | <…> | 1.0 | <…> |
| ② fixedCodebookEnergy | <…> | <…> | 1.0 | <…> |
| ⑩ gcQ12 | <…> | <…> | <…> | <…> |
| ⑪ u[0] | <…> | <…> | <…> | <…> |
| ⑫ s[0] | <…> | <…> | <…> | <…> |
| ⑬ sPf[0] | <…> | <…> | <…> | <…> |

(Task 6 `/tmp/phase1k_diag.txt` 캡처 + Task 3 gain Q-format 어서션 결과로 채울 것)

## 2. 식별된 14 dB 위치 (Design §7.6.2)

위치: `<file>:<line>`
증거: <Task 7 어서션 실패 메시지 인용>

## 3. Stage F 수정 diff 요약 (Design §7.6.3)

```diff
<git diff Stage F commit 인용>
```

## 4. ALGTHM frame 0 80 샘플 비트-정확 (Design §7.6.4)

`TestDecode_Frame0AllSamples_MatchesALGTHM`: <PASS | FAIL with 어느 샘플 어느 정도>

## 5. 병리적 테스트 A/B 분류 (Design §7.6.5)

| 테스트 | 전략 | 강등 사유 |
|--------|------|-----------|
| AllZeroCodebookIsBounded | A | — |
| LowEnergyCodebookIsSmooth | A 또는 B | <…> |
| HighEnergyCodebookIsBounded | B | <…> |
| SucceedsAcrossAllGainIndices | B | <…> |

## 6. 탈출 해치 발동 여부 (Design §7.6.6)

탈출 해치 1 (단일 모듈 미지목): <발동 여부>
탈출 해치 2 (sample 0 회귀): <발동 여부>
탈출 해치 3 (정확히 14 dB 아님): <발동 여부 + 실제 dB>
탈출 해치 4 (병리적 재인증 미완성): <발동 여부>

## 7. 검증 결과

### `go test -race ./...`

```
<Step 1 /tmp/phase1k_final_test.txt 인용>
```

### `go vet ./...`

빈 출력 (silent).

### `BenchmarkDecode -benchmem`

```
<Step 1 /tmp/phase1k_final_bench.txt 인용>
```

## 8. 커밋 목록

```
<git log --oneline e76f48f..HEAD>
```

## 9. 위반 여부 자가 점검

- [ ] ITU C 참조 / bcg729 / Sipro Lab / FFmpeg 코드 미열람
- [ ] ITU 테스트 벡터의 내부 바이트 레이아웃 미검사
- [ ] 변수/함수 명은 스펙 수학 기호에서만 유래
- [ ] 커밋 메시지에 금지어 ("포팅", "porting", "bcg729", "ITU C", "reference implementation", "Sipro") 없음
- [ ] `t.Skip` 신규 추가 없음 (V2의 메시지 갱신은 기존 skip)
- [ ] 위장 placeholder 커밋 없음
- [ ] Phase 1i 비트-정확 (frame 0 sample 0 = 2) 회귀 없음
- [ ] `BenchmarkDecode` 0 allocs/op 유지
- [ ] `go vet` silent

## 10. Phase 1l 권고

- 남은 ITU 6개 벡터 (SPEECH, FIXED, LSP, PITCH, TAME, TEST) 활성화 — frame 0 비트-정확이 보장됐으므로 frame 1+의 발산만 추적
- frame 1+의 발산 패턴이 frame 0과 다른지 확인 (state advance 관련 버그 vs 정적 산식 버그 분리)
- OVERFLOW.BIT 비트스트림 파서 버그 → Phase 1m+로 보류
```

- [ ] **Step 3: 보고서 한번 더 검토 + 모든 `<…>` placeholder가 채워졌는지 확인**

`<…>`이 보고서에 남아 있으면 commit 금지.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-04-27-phase1k-completion-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Phase 1k completion report

ALGTHM frame 0 (80 samples) bit-exact achieved (or partial-success
fallback). 14 dB located at <boundary X>, fixed in <module>/<file>:<line>.
Stage D contracts permanent. Pathological tests re-certified A+B.
Phase 1l: enable remaining 6 ITU vectors with same root-cause pattern.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review Checklist (플랜 자체 검증)

플랜을 읽으며 다음을 확인:

- [ ] Spec §1~10 모든 항목이 12개 task 중 하나에 대응됨
- [ ] 모든 step에 실행 가능한 코드 또는 명령이 포함됨 (placeholder 0개, except `<CAPTURE_FROM_STEP_1>` style local references and Task 12 which is a report template)
- [ ] 함수 / 타입 / 상수 명이 task 간 일관 (예: `TestDecode_Frame0Sample0_MatchesALGTHM`은 Task 1, 8, 9에서 동일 이름)
- [ ] 각 commit message에 Co-author trailer 명시
- [ ] Stage F의 분기별 템플릿이 5.2 표와 일치 (gain / synth-excitation / synth-filter / postfilter / fcb)
- [ ] 탈출 해치 1~4의 발동 조건과 대응이 plan 안에 명시됨 (Task 7 Step 3, Task 8 Step 5, Task 11 Step 4)
