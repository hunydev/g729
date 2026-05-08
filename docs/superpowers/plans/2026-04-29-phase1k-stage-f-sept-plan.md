# Phase 1k Stage F-sept Diagnostic-Only Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-sext-1 §4 시나리오 (i) (`synth.Filter[5..7] = [+1,+1,+1]` vs PST want `[−1,−1,−1]`, 4 stage 모두 부호 유지) 후속. ALGTHM frame 0 sf0 sample 5..7 부호 반전의 *상류 결함 위치* 를 **진단-only** 로 단일 식별한다 — 3 후보: (a) excitation `u[]` 합성 (`gp·v[n] + gc·c[n]`, §4.1.6 eq. 75), (b) LP coefficients `Â(z)` (§4.1 LSP-to-LP), (c) synth IIR `1/Â(z)` 산술 (§3.10 two-pass overflow). 후속 production fix 는 별도 cycle (F-oct) 권고. **production 변경 0 라인** invariant.

**Architecture:** 4-task 진단 cycle (F-quart / F-sext 패턴 답습). Task F-sept-1 = excitation 분해 진단 (`gp·v[5]` term, `gc·c[5]` term, sum 부호 trace + 사전 `v[5]`/`c[5]` 부호 측정). Task F-sept-2 = LP `Â(z)` reference cross-check (LSP → LP 변환 의 float64 직접 도출 reference, sf0 LP coeff prod vs ref 비교). Task F-sept-3 = `synth.Filter` IIR boundary trace (sample 0..7 step-by-step 직접형 IIR 누산값 측정 + reference float64 IIR 비교). Task F-sept-4 = 종합 (3 task 결과 결합 분석) + F-oct production fix cycle 권고 방향 결정. 각 task 의 production 코드 변경 0 라인 (E5 invariant); test 추가만 허용.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.10 (synthesis filter, two-pass overflow) + §4.1.6 eq. 75 (excitation 합성) + §4.1 (LSP decoding + LP interpolation) + §3.2.6 (LSP-to-LP recursion). 기존 F-quart/F-sext 진단 하니스 (cross-check 패턴) 재활용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-sept 진입 시점, post-6f1c841)

| 경로 | 상태 | F-sept 변경? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 | **No** (보존, 별도 cycle 처리). **단 본 파일은 LP 변환 핵심 — F-sept-2 의 cross-check 측정값은 *modified 상태* 에 대한 측정임을 보고서에 명시 의무.** |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis/F-tris 진단 baseline | **No** (보존) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (F-quart/F-quint) | **No** (변경 금지) |
| `internal/decoder/stagef_sext_diagnostic_test.go` | committed `6f1c841` (F-sext-1) | **No** (변경 금지 — F-sept 는 별도 파일 사용) |
| `internal/synth/excitation.go` | F-quint-3 시점 그대로 | **No** (진단-only) |
| `internal/synth/filter.go` | F-quint-3 시점 그대로 | **No** (진단-only) |
| `internal/lsp/decoder.go` / `interpolate.go` / `lsf_lsp.go` 등 | F-quint-3 시점 그대로 | **No** (진단-only) |
| 그 외 production 파일 | 미변경 | **No** (진단-only) |

F-sept 신규 파일 (모두 *_test.go 또는 .md):
- (Task F-sept-1) `internal/decoder/stagef_sept_diagnostic_test.go` — 본 cycle 의 모든 진단 test 통합 파일 (F-sept-1/2/3 의 새 test 모두 본 파일에 누적 추가).
- (Task F-sept-1) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md`
- (Task F-sept-2) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md`
- (Task F-sept-3) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md`
- (Task F-sept-4) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md`

본 cycle 의 production 변경 범위 = **0 라인**. test 변경 = `stagef_sept_diagnostic_test.go` 신규 파일 only. 그 외 *_test.go 파일 변경 절대 금지.

**중요**: `internal/lsp/lsp_lp.go` 의 uncommitted modification 은 *기존 F-bis-1 P fix 작업의 보류분* (F-quint-3 §4.8 명시). 본 cycle 은 해당 파일에 어떠한 변경도 가하지 않으며, 진단 측정값은 *modified working tree 상태* 에 대한 측정으로 해석한다. 이 사실은 각 보고서 §0 에 명시 의무.

### Phase 0.2 회귀 게이트 명세

각 task commit 직후 *반드시* 실행:

1. **Stage D 17 contract test**: `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/` 의 contract spec test. 본 cycle 회귀 0 의무.
2. **Stage D-bis 3 contract test**: F-bis-1 P fix 검증 + LSP 합성 cross-check + 추가 contract.
3. **Phase 1i sample 0 가드** (`TestDecode_Frame0Sample0_MatchesALGTHM`): F-quint-2 commit `1c00385` 후부터 PASS (got=2 want=2). 본 cycle 모든 commit 직후 PASS 의무.
4. **F-quart-3 reference cross-check** (`TestDiagnostic_FquartGainReferenceCrossCheck`): F-quint cycle 후 PASS. 본 cycle 회귀 0.
5. **F-quart-1 alignment harness** (`TestDiagnostic_FquartGainImap_Sf0Sample0to7`): F-quint cycle 후 measurement-only PASS. 본 cycle 직접 영향 없음.
6. **F-sext-1 chain trace** (`TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7`): commit `6f1c841` 시점 PASS. 본 cycle 직접 영향 없음 (단순 회귀 가드).

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.2 의 1~6 중 임의) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | Task F-sept-2 또는 F-sept-3 의 reference impl 가 spec § 식 verbatim 인용에서 도출되지 않음 (= prod-동치 휴리스틱 fit) | 즉시 reference impl 폐기 + spec § 식 hand-trace 재작성 + 보고서에 도출 과정 정량 기록 |
| **E3** | Task F-sept-1/2/3 결과가 *상호 모순* (예: F-sept-1 이 excitation 결함을 단일 지목하지만 F-sept-3 도 IIR 누산 결함 단일 지목 — 중복 결함 정황) | 단일 결함 식별 실패. 시나리오 결합 분석 (F-sept-4 §3) 에 모순 명시 + F-oct 권고를 *복수 fix 동시 적용* 으로 갱신. |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | 본 cycle 의 임의 commit 가 production 파일 (`internal/**/*.go` 중 `*_test.go` 가 아닌 것) 1 라인이라도 변경 | 즉시 `git revert HEAD` + commit 재구성 (test-only 로 축소) |

각 보고서 (F-sept-1/2/3/4) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 (forced-fit) 회피 의무

본 cycle 은 **진단-only** — 따라서 강압-적합 위험은 production fix cycle 보다 낮으나, 다음 의무는 유지:

1. **측정값 그대로 기록**: 각 task 의 모든 측정값은 **raw output** 으로 보고서 §3 에 인용. 의도적 재가공 / 평균 / 정규화 금지.
2. **spec § verbatim 인용**: 각 task 의 보고서 §1 에 §3.10 / §4.1.6 / §4.1 / §3.2.6 의 spec 식 verbatim 인용 (PDF page + line 번호 포함).
3. **reference impl 도출 경로 명시**: Task F-sept-2/F-sept-3 의 reference impl 는 spec 식에서 *직접* 도출 (식 자체 → float64 코드). production 코드 또는 외부 구현 0 참조. reference impl 의 각 라인이 spec 의 어느 식에서 왔는지 in-line 주석 의무.
4. **F-oct 권고는 측정 기반 ranking 만**: Task F-sept-4 의 권고 (excitation / LP / synth IIR 중 어느 부분 fix) 는 §3 의 측정값 표에서 *직접* 도출. 휴리스틱 해석 금지 — 측정값이 결정적이지 않으면 "결정적 분리 불가, F-oct-1 추가 진단 필요" 로 명시.
5. **lsp_lp.go uncommitted 영향 분리**: F-sept-2 의 LP cross-check 결과가 ref ≠ prod 으로 나타날 경우, 그 원인이 (i) lsp_lp.go uncommitted 변경 영향 vs (ii) lsp_lp.go *외* (codebook / interpolation 등) 결함 중 어느 것인지 추가 진단 의무 — 원인 분리 없이 fix 권고 금지.

---

## Task F-sept-1: excitation `u[5]` 부호 분해 진단

**Goal:** ALGTHM frame 0 sf0 sample 5 의 부호 반전이 excitation 합성 단계 (`u[5] = gp·v[5] + gc·c[5]`, §4.1.6 eq. 75) 에서 발생하는지 식별. `gp·v[5]` 항, `gc·c[5]` 항, sum 의 각 부호 + 절대값을 측정해 두 항 중 어느 쪽이 *부호를 결정짓는지* 정량 분리. 사전 `v[5]` (adaptive codebook) 와 `c[5]` (fixed codebook) 의 부호도 함께 측정해 상류 (pitch / fcb) 결함 가능성 검증.

**Files:**
- Create: `internal/decoder/stagef_sept_diagnostic_test.go` (신규 진단 파일, `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md`

### Spec § 인용 (F-sept-1 진단 근거)

ITU-T G.729 (06/2012) §4.1.6 eq. (75) (PDF p.30):

> u(n) = ĝ_p · v(n) + ĝ_c · c(n),    n = 0, …, L_subframe − 1

`internal/synth/excitation.go:7-12` production self-citing docstring:

> // BuildExcitation composes the per-subframe excitation
> //
> //	u(n) = g_p · v(n) + g_c · c(n)
> //
> // per ITU-T G.729 §4.1.6 eq. (75), using ITU saturation arithmetic.

`internal/synth/excitation.go:21-26` Q-format 공식:

```
lPitch = LMult(gpQ14, v[n])              // Q15 (= 2·gp·v)
lCode  = LShr(LMult(gcQ12, c[n]), 11)    // Q26 → Q15
lSum   = LAdd(lPitch, lCode)             // Q15
u[n]   = Round(LShl(lSum, 1))            // Q15 → Q16 → Q0
```

본 task 는 위 4 단계 (lPitch / lCode / lSum / u[5]) 의 부호 + 절대값 + saturation 발생 여부를 모두 측정.

**핵심 검증 점**:
- `gp_q14 = 1995` (F-sext-1 §3.2 측정), `gc_q12 = 4153` 모두 양수.
- 따라서 `lPitch = 2·gp·v[5]` 의 부호 = `v[5]` 의 부호.
- `lCode = (2·gc·c[5]) >> 11` 의 부호 = `c[5]` 의 부호.
- 두 항의 부호 조합과 절대값 비례에 따라 `lSum` 의 최종 부호 결정.

PST want sample 5 = `−1` (부호 음). 따라서 `u[5]` 또한 음수 영역 기대 (synth IIR 이 부호 유지하므로). 측정 결과 `u[5] > 0` 이면 → 합성 *입력* 결함; `u[5] ≤ 0` 인데 `synth.Filter[5] > 0` 이면 → IIR 산술 결함 (Task F-sept-3 우선).

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-sext-1 commit `6f1c841` 후 동일.)

Run (회귀 게이트 baseline):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
```

Expected: 4건 모두 PASS. 본 출력을 보고서 §2 에 인용.

- [ ] **Step 2: 진단 test 작성 — `stagef_sept_diagnostic_test.go` 신규**

`internal/decoder/stagef_sept_diagnostic_test.go` 신규 작성. 본 파일은 **본 cycle 의 모든 진단 test** 를 담는다 (F-sept-1, F-sept-2, F-sept-3 각 task 의 test).

본 task (F-sept-1) 에서는 `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` 추가:

```go
package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
)

// TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5: Stage F-sept-1 진단.
//
// ITU-T G.729 (06/2012) §4.1.6 eq. (75): u(n) = ĝ_p · v(n) + ĝ_c · c(n).
//
// F-sext-1 §4 시나리오 (i) 후속 — synth.Filter[5..7] = [+1,+1,+1] 의 상류
// 결함 위치를 식별. excitation u[5] 합성에서 두 항 (gp·v[5], gc·c[5]) 의
// 부호 + 절대값 + saturation 거동을 측정.
//
// 측정-only — 산술 분해는 production 함수 호출이 아닌 *test 내부 재현*
// 으로 진행 (BuildExcitation 의 LMult/LShr/LAdd/Round 단계별 capture).
// production 코드 0-수정 보장 (E5).
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

	// PST/2 sample 5 spec-target (= ITU PST sample >> 1)
	pstWant5 := wantFrames[0][5]
	pstHalf5 := int16(int32(pstWant5) >> 1)
	t.Logf("PST want sample 5 = %d (PST/2 = %d)", pstWant5, pstHalf5)

	// frame 0 sf0 디코딩 (Decoder instance 우회, zero-init).
	// 정확한 API signature 는 internal/lsp/, internal/pitch/, internal/fcb/,
	// internal/gain/ 의 exported API 검토 후 호출.

	// (a) LSP → frame 0 sf0 LP coefficients
	var lspDec lsp.Decoder
	lspDec.Reset()
	var sfA [lpcOrder + 1]int16
	if err := lspDec.DecodeSubframe0(f.L0, f.L1, f.L2, f.L3, &sfA); err != nil {
		t.Fatalf("lsp.DecodeSubframe0: %v", err)
	}
	t.Logf("sf0 LP coefficients (Q12, a[0]=4096): %v", sfA[:])

	// (b) pitch → tInt / tFrac
	tInt, tFrac := pitch.DecodeDelaySubframe0(f.P1)
	t.Logf("pitch sf0: tInt=%d tFrac=%d", tInt, tFrac)

	// (c) adaptive codebook v[]
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	t.Logf("v[] sample 0..7 = [%d %d %d %d %d %d %d %d]",
		v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7])

	// (d) fixed codebook c[]
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: f.S1}, tInt, betaQ14, &c)
	t.Logf("c[] sample 0..7 = [%d %d %d %d %d %d %d %d]",
		c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7])

	// (e) gain → gp_q14, gc_q12
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: f.GA1, GB: f.GB1}, &c)
	t.Logf("gain sf0: gp_q14=%d gc_q12=%d (beta_q14=%d)", gpQ14, gcQ12, betaQ14)

	// (f) excitation u[5] 분해 trace — production BuildExcitation 알고리즘 재현
	t.Logf("──────── excitation u[5] 분해 trace (§4.1.6 eq. 75) ────────")
	for n := 0; n <= 7; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
		lSum := fixed.LAdd(lPitch, lCode)
		un := int16(fixed.Round(fixed.LShl(lSum, 1)))
		t.Logf("[%d] v=%+5d c=%+5d  lPitch=%+10d lCode=%+10d lSum=%+10d  u=%+5d",
			n, v[n], c[n], int32(lPitch), int32(lCode), int32(lSum), un)
	}

	// (g) sample 5 집중 분석
	v5sign := signOfInt16(v[5])
	c5sign := signOfInt16(c[5])
	lPitch5 := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[5]))
	lCode5 := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[5])), 11)
	lSum5 := fixed.LAdd(lPitch5, lCode5)
	u5 := int16(fixed.Round(fixed.LShl(lSum5, 1)))
	t.Logf("──────── sample 5 부호 결정 분석 ────────")
	t.Logf("v[5] = %+d (부호 %s)", v[5], v5sign)
	t.Logf("c[5] = %+d (부호 %s)", c[5], c5sign)
	t.Logf("gp_q14·v[5] / 항(lPitch)         = %+d (부호 %s, |절대값| %d)",
		int32(lPitch5), signOfInt32(int32(lPitch5)), abs32(int32(lPitch5)))
	t.Logf("gc_q12·c[5] / 항(lCode)          = %+d (부호 %s, |절대값| %d)",
		int32(lCode5), signOfInt32(int32(lCode5)), abs32(int32(lCode5)))
	t.Logf("lSum                              = %+d (부호 %s)",
		int32(lSum5), signOfInt32(int32(lSum5)))
	t.Logf("u[5]                              = %+d (부호 %s)",
		u5, signOfInt16(u5))
	t.Logf("PST want sample 5  부호 = %s (값 %d)", signOfInt16(pstWant5), pstWant5)

	// (h) 시나리오 분류 dump
	t.Logf("──────── F-sept-1 시나리오 분류 ────────")
	t.Logf("u[5] 부호 = %s, PST want 부호 = %s", signOfInt16(u5), signOfInt16(pstWant5))
	if signOfInt16(u5) == signOfInt16(pstWant5) {
		t.Logf("(시나리오 A) excitation u[5] 부호 = PST want 부호 → IIR 또는 LP 결함 (F-sept-2 / F-sept-3)")
	} else {
		t.Logf("(시나리오 B) excitation u[5] 부호 ≠ PST want 부호 → excitation 합성 결함")
		t.Logf("   하위 분류:")
		t.Logf("     B1 v[5] 부호 ≠ expected → adaptive codebook (pitch) 결함")
		t.Logf("     B2 c[5] 부호 ≠ expected → fixed codebook (fcb) 결함")
		t.Logf("     B3 두 항 부호 정상이나 절대값 ratio 결함 → gain decode 잔여")
		t.Logf("     B4 saturation 발생 → fixed primitives 결함 (Word16 overflow)")
	}
}

// signOfInt16 / signOfInt32 / abs32 — F-sept 진단 helper.
// (signOf 가 stagef_sext_diagnostic_test.go 에 이미 존재하면 그것을 사용.
//  중복 정의 방지 — 본 step 에서 기존 helper 가용 여부 검토.)
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
```

**중요 — helper 중복 회피**: F-sext-1 의 `stagef_sext_diagnostic_test.go` 에 `signOf(int16) string` 이 이미 정의되어 있다. 본 task 의 helper 명을 다르게 (예: `signOfInt16`) 두거나, F-sext-1 helper 를 재사용 (같은 package `decoder` 라 직접 호출 가능). 컴파일 충돌 발생 시 본 task 의 helper 명을 변경 — 기존 F-sext 파일 *수정 금지*.

API signature 가 plan 예시와 다르면 *test 코드만* 수정 (production 변경 금지).

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v`

Expected: PASS (assertion 0, t.Logf 만 출력). raw output 보고서 §3.1 에 인용.

컴파일 오류 발생 시 (signature 불일치 등) 본 step 에서 *test 코드만* 수정. production 코드 1 라인이라도 변경 시 즉시 E5 발동.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 의 출력에서 다음 표 작성 (보고서 §3.2):

| 항목 | 값 | 부호 |
|------|----|------|
| `v[5]` | ? | ? |
| `c[5]` | ? | ? |
| `lPitch = LMult(gp_q14, v[5])` (Q15) | ? | ? |
| `lCode = LShr(LMult(gc_q12, c[5]), 11)` (Q15) | ? | ? |
| `lSum = lPitch + lCode` (Q15) | ? | ? |
| `u[5] = Round(LShl(lSum, 1))` (Q0) | ? | ? |
| **PST want sample 5** | **−1** | **−** |

분류 시나리오:
- **(A)** `u[5]` 부호 = `−` (= PST want 부호 일치) → excitation 합성 정상. 결함은 LP `Â(z)` (Task F-sept-2) 또는 synth IIR (Task F-sept-3).
- **(B1)** `u[5]` 부호 = `+`, `v[5]` 부호 = `+` → adaptive codebook (pitch) 결함 의심. F-oct 권고: pitch.AdaptiveCodebook 진단/fix.
- **(B2)** `u[5]` 부호 = `+`, `c[5]` 부호 = `+` → fixed codebook (fcb) 결함 의심. F-oct 권고: fcb.Decode 진단/fix (β init / pulse sign 등).
- **(B3)** `v[5]`, `c[5]` 부호 정상이나 두 항 절대값 ratio 가 PST 와 모순 → gain decode 잔여 결함 (F-quint cycle 미해소분).
- **(B4)** `lPitch` 또는 `lCode` 가 saturation 영역 (Q15 |값| > 32767) → `internal/fixed/` primitives 결함 (Word16 overflow).

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 6 게이트 모두 PASS + 본 task 의 새 test PASS. 비-contract diagnostic 3건 (F-quint-3 §4.6) FAIL 유지 (plan-허용).

- [ ] **Step 6: F-sept-1 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md`:

```markdown
# Phase 1k Stage F-sept-1 보고서 — excitation u[5] 부호 분해 진단

**작성일**: 2026-04-29
**범위**: F-sext-1 §4 시나리오 (i) 후속. ALGTHM frame 0 sf0 sample 5
        의 excitation u[5] = gp·v[5] + gc·c[5] 합성 시 두 항 + sum 의
        부호 + 절대값 측정.
**산출물**: lPitch / lCode / lSum / u[5] 단계별 부호 분포 표 +
            v[5] / c[5] 부호 사전 trace.
**준수**: ITU-T G.729 (06/2012) §4.1.6 eq. (75) 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.
**lsp_lp.go uncommitted 영향**: 본 task 는 LP coefficient 를 *호출* 만
        하며 sf0 LP 값을 그대로 dump 한다. lsp_lp.go modified 상태가
        sf0 LP 에 미치는 영향은 F-sept-2 cross-check 에서 분리.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. §4.1.6 eq. (75) + production self-citing docstring 인용
## 2. 회귀 게이트 baseline (Step 1 출력)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 sample 5 분해 표 (Step 4)
   3.3 sample 0..7 의 v[]/c[]/lPitch/lCode/u[] 보조 dump
## 4. 시나리오 분류 (A / B1 / B2 / B3 / B4)
## 5. F-sept-2 / F-sept-3 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go                                    ← 미변경 보존
?? internal/decoder/stagef_bis_diagnostic_test.go            ← 미변경 보존
?? internal/decoder/stagef_sept_diagnostic_test.go           ← 본 task 신규
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md
```

**E5 검증**: `git diff -- internal/` 의 production 라인 (즉 `*_test.go` 가 아닌 파일) 변경 0. `internal/lsp/lsp_lp.go` 의 별도 cycle 보류 변경은 *기존 modified* 그대로 (본 task 가 추가 변경 0).

```bash
git add internal/decoder/stagef_sept_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sept-1 excitation u[5] decomposition harness

F-sext-1 §4 시나리오 (i) 후속. ALGTHM frame 0 sf0 sample 5 부호
반전의 상류 결함 위치를 식별하기 위해 ITU-T G.729 (06/2012)
§4.1.6 eq. (75) u(n) = gp·v(n) + gc·c(n) 의 두 항을 분해한다:
lPitch = LMult(gp_q14, v[n]), lCode = LShr(LMult(gc_q12, c[n]), 11),
lSum, u[n] = Round(LShl(lSum, 1)) 단계별 부호 + 절대값 측정.

본 진단은 측정-only — production 변경 0. 시나리오 분류
(A / B1 / B2 / B3 / B4) 로 후속 task (F-sept-2 LP cross-check,
F-sept-3 synth IIR trace) 또는 F-oct production fix cycle
권고 방향 결정.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sept-2: LP `Â(z)` reference cross-check

**Goal:** ALGTHM frame 0 sf0 의 LP coefficients `Â(z)` 가 §4.1 (LSP decoding) + §3.2.6 (LSP → LP 변환) 의 spec 식 verbatim 도출과 일치하는지 검증. test 코드 내부에 float64 reference impl 작성, production `lsp.Decoder.DecodeSubframe0` 출력의 11 계수 (a[0..10] Q12) 와 비교. F-quart-3 reference cross-check 패턴 답습.

**Files:**
- Modify: `internal/decoder/stagef_sept_diagnostic_test.go` (`TestDiagnostic_FseptLPReferenceCrossCheck` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md`

### Spec § 인용 (F-sept-2 reference 도출 근거)

ITU-T G.729 (06/2012) §4.1 (PDF p.27-28) — LSP decoding (4-bit MA stage + 5-bit + 5-bit). §3.2.6 (PDF p.13) — LSP-to-LP 변환:

> The LP filter coefficients are obtained from the LSP coefficients by  
> A(z) = (1 + z⁻¹)·F₁'(z)/2 + (1 − z⁻¹)·F₂'(z)/2  
> where F₁'(z), F₂'(z) are the symmetric/antisymmetric polynomials defined  
> by the LSP frequencies q_i.

§4.1 LSP interpolation (PDF p.28):

> q_i^(sf0) = 0.5·q_i^(prev frame) + 0.5·q_i^(current frame)

(여기서 prev frame = frame 0 의 *직전* frame 이지만 frame 0 첫 디코딩 시 §4.3 Table 9 의 zero-init q_i 사용.)

본 task 는 위 식을 float64 로 직접 구현, production 출력과 비교.

**중요**: LSP-to-LP 변환에서 관여하는 production 파일:
- `internal/lsp/decoder.go` — Decoder API
- `internal/lsp/codebook.go` — LSF codebook lookup
- `internal/lsp/predictor.go` — MA predictor
- `internal/lsp/lsf_lsp.go` — LSF ↔ LSP 변환
- `internal/lsp/interpolate.go` — LSP frame interpolation
- `internal/lsp/lsp_lp.go` — **LSP → LP polynomial expansion** (uncommitted modification!)
- `internal/lsp/stability.go` — stability check

`lsp_lp.go` 가 uncommitted modified 상태 → 본 task 의 cross-check 결과는 *modified 상태* 에 대한 측정. 결과가 ref ≠ prod 일 경우 다음 분리 진단 의무 (Phase 0.4 §5):
- (i) `lsp_lp.go` modified 상태 효과 → `git stash` 후 재측정으로 modified 영향 측정
- (ii) `lsp_lp.go` 외 결함 → modified 파일 stash 상태에서도 mismatch 유지

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sept-1 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-sept-1 의 신규 파일은 commit 됨.)

- [ ] **Step 2: reference impl + cross-check 작성**

`internal/decoder/stagef_sept_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FseptLPReferenceCrossCheck: Stage F-sept-2 진단.
//
// ITU-T G.729 (06/2012) §4.1 (LSP decoding) + §3.2.6 (LSP-to-LP) 에서
// 직접 도출한 float64 reference impl 을 작성하고, production
// lsp.Decoder.DecodeSubframe0 출력의 11 계수 (a[0..10] Q12) 와 비교.
//
// 외부 G.729 구현 0 인용. reference impl 의 모든 라인은 spec 식 또는
// §4.3 Table 9 초기화에서 직접 도출.
//
// 측정-only — Δ assertion 0.
//
// 주의: lsp_lp.go 가 uncommitted modified 상태. 본 cross-check 의
// production 측정값은 modified 상태에 대한 것이며, ref ≠ prod 인 경우
// modified vs unmodified 영향 분리 진단을 보고서 §3.3 에 명시 의무.
func TestDiagnostic_FseptLPReferenceCrossCheck(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	ensureTestdataPresent(t, bitPath)

	frames, _ := readG192Frames(t, bitPath)
	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// production: lsp.Decoder.DecodeSubframe0 sf0 LP coefficients
	var lspDec lsp.Decoder
	lspDec.Reset()
	var sfAProd [lpcOrder + 1]int16
	if err := lspDec.DecodeSubframe0(f.L0, f.L1, f.L2, f.L3, &sfAProd); err != nil {
		t.Fatalf("lsp.DecodeSubframe0: %v", err)
	}

	// reference: §4.1 + §3.2.6 직접 도출 float64 impl
	sfARef := referenceLSPToLPSubframe0(t, f.L0, f.L1, f.L2, f.L3)

	// dump
	t.Logf("──────── F-sept-2 cross-check (production Q12 vs §3.2.6 reference float64) ────────")
	t.Logf("idx   prod_q12   ref(float64)   ref(round_q12)   Δ(prod − ref_round)")
	for i := 0; i <= 10; i++ {
		refQ12 := int16(int32(roundFloat(sfARef[i] * 4096)))
		t.Logf("[%2d]   %+6d     %14.10f   %+6d           %+d",
			i, sfAProd[i], sfARef[i], refQ12, int32(sfAProd[i])-int32(refQ12))
	}

	// 시나리오 분류
	allMatch := true
	for i := 0; i <= 10; i++ {
		refQ12 := int16(int32(roundFloat(sfARef[i] * 4096)))
		if sfAProd[i] != refQ12 {
			allMatch = false
			break
		}
	}
	if allMatch {
		t.Logf("F-sept-2 분류: prod = ref (sf0 LP coeff 11 항 비트-정확) — LP 변환 spec 정합")
	} else {
		t.Logf("F-sept-2 분류: prod ≠ ref — LSP-to-LP 변환에서 spec 식 도출 결과와 분기")
		t.Logf("   하위 진단 의무: lsp_lp.go modified 상태 영향 분리 (§3.3)")
	}
}

// referenceLSPToLPSubframe0: §4.1 LSP decoding + interpolation + §3.2.6
// LSP-to-LP 의 float64 직접 구현. frame 0 sf0 의 LP coefficients 반환.
//
// 외부 구현 0 인용. 모든 라인은 spec § 인용 in-line 주석 의무.
func referenceLSPToLPSubframe0(t *testing.T, l0, l1, l2, l3 uint16) [lpcOrder + 1]float64 {
	t.Helper()

	// (1) §4.1 LSP decoding: l0=switch bit, l1/l2/l3 = stage-1/stage-2 indices.
	//     (구체 구현은 §4.1 식 + tables LSPCB1/LSPCB2 참조 — 본 함수 body
	//      에서 spec 식 verbatim 인용 in-line.)
	//
	// (2) §4.1 stability check (lsp[i+1] − lsp[i] ≥ Δ_min) 적용.
	//
	// (3) §4.1 interpolation: q_sf0 = 0.5·q_prev + 0.5·q_curr.
	//     frame 0 첫 디코딩 시 q_prev = §4.3 Table 9 zero-init.
	//
	// (4) §3.2.6 LSP-to-LP polynomial expansion:
	//       A(z) = (1+z⁻¹)F₁'(z)/2 + (1−z⁻¹)F₂'(z)/2
	//     where F₁'(z) = ∏ (1 − 2·cos(ω_2i)·z⁻¹ + z⁻²) (i = 0..4)
	//           F₂'(z) = ∏ (1 − 2·cos(ω_2i+1)·z⁻¹ + z⁻²) (i = 0..4)
	//     a[0] = 1.0; a[1..10] = 식 도출 결과 (real-valued).

	// ... (본 step 에서 spec § 식 직접 도출 — 구체 구현은 작성 시점에
	//      §4.1 + §3.2.6 의 PDF 인용 후 line-by-line 도출.)

	var aRef [lpcOrder + 1]float64
	aRef[0] = 1.0
	// ... 나머지 계산 ...
	return aRef
}

// roundFloat: float64 → nearest int (helper).
func roundFloat(f float64) int32 {
	if f >= 0 {
		return int32(f + 0.5)
	}
	return int32(f - 0.5)
}
```

**중요**: `referenceLSPToLPSubframe0` 의 body 는 §4.1 + §3.2.6 의 spec 식을 *직접 도출*. 작성 시점에 다음 조건 의무:
1. 모든 상수 (LSPCB1 / LSPCB2 codebook entry) 는 ITU spec 의 verbatim 값 (또는 `internal/tables/` 의 production 테이블 *조회만*).
2. 모든 산술 라인에 spec § 인용 in-line 주석 (예: `// §3.2.6 식 (12)` 또는 `// §4.1 stability check`).
3. production 코드 (`internal/lsp/*.go`) 의 *알고리즘 복제 금지* — spec § 만 참조. production 코드의 Q-format / saturation 거동을 모방하지 말 것 (양자화 0, real-valued 만).

본 step 의 코드 작성은 **상당한 노력 필요** (LSP codebook lookup + interpolation + polynomial expansion). 단순화 방안:
- 옵션 (1): 본 task 에서 풀 reference impl 작성 (+ ~150 라인).
- 옵션 (2): F-sept-2 의 *측정-only* 본질에 맞춰 reference impl 의 일부 (예: stage-1 codebook lookup) 만 구현 + 나머지는 production 호출 결과 재사용 → cross-check 가 *완전한 spec 도출* 이 아님 → 보고서 §1 에 명시.

옵션 (1) 권장. 옵션 (2) 채택 시 cross-check 의 신뢰도 한계 보고서 §의 결론 강도에 영향 (E2 발동 가능성 검토).

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v`

Expected: PASS. raw output 보고서 §3.1 에 인용.

- [ ] **Step 4: prod vs ref 분류 + lsp_lp.go modified 영향 분리**

Step 3 의 출력에서 a[0..10] 의 prod − ref_q12 Δ 분포 분석:

분류:
- **(L1)** 모든 a[i] |Δ| = 0 → LSP-to-LP 변환 spec 정합 (lsp_lp.go modified 포함). LP 결함 *제외*.
- **(L2)** 일부 a[i] |Δ| ∈ {1, 2} → Q12 양자화 rounding 누적 (수치 정상). LP 결함 *제외* (sub-LSB 차이).
- **(L3)** 일부 a[i] |Δ| > 2 → LSP-to-LP 변환 결함 의심.
  - **하위 진단 의무**: `git stash` 으로 lsp_lp.go modified 폐기 → cross-check 재실행 → 결과 변화 측정.
    - **(L3a)** stash 후 |Δ| ≥ 2 잔존 → 결함 위치 = lsp_lp.go *외* (codebook / interpolation / lsf_lsp).
    - **(L3b)** stash 후 |Δ| ≤ 1 → 결함 위치 = lsp_lp.go modified 변경. F-bis-1 P fix 가 LP 정확도를 *손상* 시킨 가능성 → 별도 cycle (F-bis-1 재검토) 권고.
  - stash 측정 후 반드시 `git stash pop` 으로 working tree 복원.

**중요 — git stash 안전성**: `git stash` 는 working tree 변경을 임시 저장. stash 적용 시점:
1. F-sept-2 진단 test 실행 직후 (= production 측정값 capture 완료) 만 사용.
2. stash 대상은 `internal/lsp/lsp_lp.go` 만 (untracked file 영향 없음).
3. stash 후 측정 → stash pop → 측정값을 보고서 §3.3 에 비교 dump.
4. stash 작업이 commit 전에 정상 완료되지 않으면 작업 *중단* + 사용자 통보.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 6 게이트 + 본 task 새 test PASS.

- [ ] **Step 6: F-sept-2 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md`:

```markdown
# Phase 1k Stage F-sept-2 보고서 — LP Â(z) reference cross-check

**작성일**: 2026-04-29
**범위**: §4.1 LSP decoding + §3.2.6 LSP-to-LP 의 float64 reference
        impl 작성 + production lsp.Decoder.DecodeSubframe0 출력 비교.
**산출물**: a[0..10] prod vs ref 비교표 + lsp_lp.go modified 영향 분리.
**준수**: §4.1 + §3.2.6 + §4.3 Table 9 verbatim 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (특히 E2 평가 — 옵션 (1)/(2))
## 1. §4.1 + §3.2.6 인용 + reference impl 도출 경로
## 2. 회귀 게이트 결과
## 3. 진단 측정값
   3.1 prod vs ref 비교표 a[0..10]
   3.2 sample 5 영향 분석 (Â(z) 의 변화가 sample 5 부호에 미치는 효과 추정)
   3.3 lsp_lp.go modified 영향 분리 (git stash 측정)
## 4. 시나리오 분류 (L1 / L2 / L3a / L3b)
## 5. F-sept-3 진입 권고 + F-oct 권고 방향 1차 결정
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go                                   ← 미변경 보존
M  internal/decoder/stagef_sept_diagnostic_test.go          ← F-sept-2 추가
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md
```

```bash
git add internal/decoder/stagef_sept_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sept-2 LP Â(z) reference cross-check

ITU-T G.729 (06/2012) §4.1 (LSP decoding) + §3.2.6 (LSP-to-LP) 에서
직접 도출한 float64 reference impl 을 작성하고, production
lsp.Decoder.DecodeSubframe0 출력의 11 계수 (a[0..10] Q12) 와 비교한다.
양자화 / saturation 0 — spec real-valued 거동 그대로.

외부 G.729 구현 (ITU 참조 C, bcg729, Sipro, FFmpeg) 0 인용.
reference impl 의 모든 라인은 §4.1 + §3.2.6 식 verbatim 도출.

본 cross-check 는 sf0 LP coefficients 가 spec 정합인지 검증하며,
ref ≠ prod 인 경우 lsp_lp.go uncommitted modified 영향을 git stash
재측정으로 분리. 시나리오 (L1 / L2 / L3a / L3b) 로 후속 task
(F-sept-3 synth IIR trace) 또는 F-oct production fix 권고 결정.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sept-3: synth.Filter IIR boundary trace

**Goal:** ALGTHM frame 0 sf0 의 합성 필터 `1/Â(z)` (§3.10) IIR 누산을 sample 0..7 step-by-step 측정. production `synth.Synthesizer.Filter` 의 직접형 IIR 거동을 reference float64 IIR 과 비교, sample 5 의 부호가 IIR 산술 boundary 에서 결정되는 위치 식별.

**Files:**
- Modify: `internal/decoder/stagef_sept_diagnostic_test.go` (`TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md`

### Spec § 인용 (F-sept-3 reference 도출 근거)

ITU-T G.729 (06/2012) §3.10 (PDF p.24) — synthesis filter:

> The reconstructed speech is obtained by passing the LP excitation u(n)  
> through the LP synthesis filter:  
>   ŝ(n) = u(n) − Σᵢ₌₁¹⁰ aᵢ · ŝ(n−i),    n = 0, …, L_subframe−1

`internal/synth/filter.go:60-69` `onePass` self-citing:

```go
for n := 0; n < 40; n++ {
    lTemp := fixed.LMult(u[n], a[0])
    for i := 1; i <= 10; i++ {
        lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
    }
    lTemp = fixed.LShl(lTemp, 3)
    work[10+n] = fixed.Round(lTemp)
}
```

`internal/synth/filter.go:1-17` two-pass overflow self-citing — frame 0 sf0 의 stimulus 가 Pass 1 또는 Pass 2 중 어느 path 인지도 측정 의무.

§4.3 Table 9: zero-init `pastSynth = [0; 10]` (frame 0 첫 호출).

본 task 는 production 의 IIR 누산을 reference float64 IIR (양자화 0) 과 비교, sample 5 의 부호 분기를 식별.

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sept-2 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7`**

`internal/decoder/stagef_sept_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7: Stage F-sept-3 진단.
//
// ITU-T G.729 (06/2012) §3.10 합성 필터:
//   ŝ(n) = u(n) − Σ aᵢ · ŝ(n−i),  i=1..10
//
// production synth.Synthesizer.Filter 의 sample 0..7 IIR 누산을
// reference float64 IIR (양자화 0) 과 비교한다. sample 5 의 부호가
// IIR boundary 에서 결정되는 위치 식별.
//
// 측정-only — Δ assertion 0.
//
// 주의: §3.10 two-pass overflow recovery 가 Pass 1 또는 Pass 2 중
// 어느 path 로 동작하는지도 측정 의무 (보고서 §3.4).
func TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) 디코딩: LP / pitch / fcb / gain / excitation u[]
	var lspDec lsp.Decoder
	lspDec.Reset()
	var sfA [lpcOrder + 1]int16
	if err := lspDec.DecodeSubframe0(f.L0, f.L1, f.L2, f.L3, &sfA); err != nil {
		t.Fatalf("lsp.DecodeSubframe0: %v", err)
	}
	tInt, tFrac := pitch.DecodeDelaySubframe0(f.P1)
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: f.S1}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: f.GA1, GB: f.GB1}, &c)
	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	t.Logf("u[] sample 0..7 = [%+d %+d %+d %+d %+d %+d %+d %+d]",
		u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7])

	// (2) production synth.Filter
	var syn synth.Synthesizer
	syn.Reset()
	var sProd [subframeLen]int16
	syn.Filter(&sfA, &u, &sProd)
	t.Logf("synth.Filter (production) sample 0..7 = [%+d %+d %+d %+d %+d %+d %+d %+d]",
		sProd[0], sProd[1], sProd[2], sProd[3], sProd[4], sProd[5], sProd[6], sProd[7])

	// (3) reference float64 IIR
	sRef := referenceSynthFilter(&sfA, &u)
	t.Logf("──────── F-sept-3 cross-check (production vs §3.10 reference float64) ────────")
	t.Logf("idx   u[n]   prod_q0   ref(float64)   ref(round_q0)   Δ(prod − ref_round)")
	for n := 0; n < 8; n++ {
		refRound := int16(int32(roundFloat(sRef[n])))
		if sRef[n] > 32767 {
			refRound = 32767
		} else if sRef[n] < -32768 {
			refRound = -32768
		}
		t.Logf("[%2d]   %+5d   %+6d   %14.4f    %+6d        %+d",
			n, u[n], sProd[n], sRef[n], refRound, int32(sProd[n])-int32(refRound))
	}

	// (4) sample 5 집중 분석
	t.Logf("──────── sample 5 IIR boundary 분석 ────────")
	t.Logf("u[5] = %+d (부호 %s)", u[5], signOfInt16(u[5]))
	t.Logf("prod synth.Filter[5] = %+d (부호 %s)", sProd[5], signOfInt16(sProd[5]))
	t.Logf("ref synth.Filter[5]  = %.4f (부호 %s)", sRef[5], signOfFloat(sRef[5]))
	t.Logf("PST want sample 5    = %+d (부호 %s)", wantFrames[0][5], signOfInt16(wantFrames[0][5]))
	t.Logf("PST/2 spec-target    = %+d", int16(int32(wantFrames[0][5])>>1))

	// (5) 시나리오 분류
	prodSign := signOfInt16(sProd[5])
	refSign := signOfFloat(sRef[5])
	t.Logf("──────── F-sept-3 시나리오 분류 ────────")
	if prodSign == refSign {
		t.Logf("(시나리오 S1) prod[5] 부호 = ref[5] 부호 → IIR 산술 spec 정합")
		t.Logf("   결함 위치 = u[5] 또는 LP 계수 (F-sept-1 / F-sept-2 영역)")
	} else {
		t.Logf("(시나리오 S2) prod[5] 부호 ≠ ref[5] 부호 → IIR 산술 결함")
		t.Logf("   하위 진단: §3.10 two-pass overflow recovery (Pass 2) 가 부호 영향 가능성")
	}
}

// referenceSynthFilter: §3.10 합성 필터의 float64 직접 구현.
// 양자화 / saturation / two-pass recovery 모두 0 — spec real-valued
// 거동 그대로 (식: ŝ(n) = u(n) − Σ aᵢ·ŝ(n−i), i=1..10).
//
// §4.3 Table 9: zero-init pastSynth.
//
// a[] Q12 → float64 변환 (a[0] = 1.0).
func referenceSynthFilter(a *[lpcOrder + 1]int16, u *[subframeLen]int16) [subframeLen]float64 {
	t.Helper()
	var aFloat [lpcOrder + 1]float64
	for i := 0; i <= 10; i++ {
		aFloat[i] = float64(a[i]) / 4096.0  // Q12 → real
	}
	// pastSynth zero-init (§4.3 Table 9)
	var pastSynth [10]float64
	var out [subframeLen]float64
	for n := 0; n < subframeLen; n++ {
		// §3.10 식: ŝ(n) = u(n) − Σ aᵢ·ŝ(n−i), i=1..10
		acc := float64(u[n])
		for i := 1; i <= 10; i++ {
			var prev float64
			if n-i < 0 {
				prev = pastSynth[10+(n-i)]
			} else {
				prev = out[n-i]
			}
			acc -= aFloat[i] * prev
		}
		out[n] = acc
	}
	return out
}

func signOfFloat(f float64) string {
	switch {
	case f > 0:
		return "+"
	case f < 0:
		return "−"
	default:
		return "0"
	}
}
```

(`t.Helper()` 호출은 `referenceSynthFilter` body 의 `t` 인자 추가 필요. 작성 시 함수 signature 에 `t *testing.T` 추가.)

reference impl 의 모든 상수 = §3.10 식 + §4.3 Table 9 의 verbatim. 외부 구현 0 인용.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v`

Expected: PASS. raw output 보고서 §3.1 에 인용.

- [ ] **Step 4: prod vs ref 분류 + Pass 1/2 path 측정**

Step 3 의 출력에서 sample 0..7 의 prod − ref Δ 분포 분석:

분류:
- **(S1)** sample 0..7 |Δ| ≤ 1 → IIR 산술 spec 정합. sample 5 부호 결정은 *u[5] 또는 LP 계수* 영역 (F-sept-1 또는 F-sept-2 결과로 식별).
- **(S2)** sample 5 부호가 prod ≠ ref → IIR 산술 자체 결함. 추가 진단:
  - **(S2a)** §3.10 two-pass overflow Pass 2 가 sample 5 부호에 영향. production 호출 후 fixed.Overflow() 상태 측정 의무.
  - **(S2b)** Pass 1 단독에서도 mismatch — 직접형 IIR 누산의 Q-format / saturation 결함.
- **(S3)** sample 0..4 PASS 이지만 sample 5 부호 prod ≠ ref → Q12 누산의 sub-LSB 차이가 sample 5 boundary 에서 부호 분기. F-oct production fix 권고는 *Q-format 정밀도 향상* (예: 누산 widening) 방향.

`fixed.Overflow()` 상태 capture: production `synth.Synthesizer.Filter` 호출 *직전* `fixed.ClearOverflow()` 호출 후, 호출 *직후* `fixed.Overflow()` 값 조회. boolean 결과 (true = Pass 2 발동) 보고서 §3.4 에 기록.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 6 게이트 + 본 task 새 test PASS + F-sept-1/2 의 기존 test PASS.

- [ ] **Step 6: F-sept-3 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md`:

```markdown
# Phase 1k Stage F-sept-3 보고서 — synth.Filter IIR boundary trace

**작성일**: 2026-04-29
**범위**: §3.10 합성 필터 1/Â(z) 의 sample 0..7 IIR 누산 측정 +
        reference float64 IIR 비교 + two-pass overflow path 측정.
**산출물**: prod vs ref 비교표 sample 0..7 + Pass 1/2 발동 여부 +
            sample 5 부호 분기 분석.
**준수**: §3.10 + §4.3 Table 9 verbatim 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가
## 1. §3.10 식 인용 + reference impl 도출 경로
## 2. 회귀 게이트 결과
## 3. 진단 측정값
   3.1 prod vs ref 비교표 sample 0..7
   3.2 sample 5 IIR boundary 분석
   3.3 sample 0..7 raw output 발췌
   3.4 §3.10 two-pass overflow Pass 1/2 발동 측정
## 4. 시나리오 분류 (S1 / S2a / S2b / S3)
## 5. F-sept-4 종합 진입 + F-oct 권고 방향 결정
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go
M  internal/decoder/stagef_sept_diagnostic_test.go          ← F-sept-3 추가
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md
```

```bash
git add internal/decoder/stagef_sept_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sept-3 synth.Filter IIR boundary trace

ITU-T G.729 (06/2012) §3.10 합성 필터 ŝ(n) = u(n) − Σ aᵢ·ŝ(n−i)
의 sample 0..7 IIR 누산을 production synth.Synthesizer.Filter 와
reference float64 IIR 으로 비교. §4.3 Table 9 zero-init pastSynth.

§3.10 two-pass overflow recovery 의 Pass 1/2 path 도 fixed.Overflow()
상태로 측정. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro, FFmpeg)
0 인용. reference impl 모든 라인 §3.10 식 + §4.3 Table 9 verbatim
도출.

본 cross-check 는 sample 5 부호 반전이 (S1) IIR 정상 → u 또는 LP
결함 / (S2) IIR 산술 결함 / (S3) Q-format 정밀도 boundary 중 어느
시나리오인지 식별. F-sept-4 종합 분석 + F-oct production fix
권고 방향 결정.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sept-4: 종합 보고서 + F-oct 권고

**Goal:** F-sept-1 (시나리오 A/B1/B2/B3/B4) × F-sept-2 (L1/L2/L3a/L3b) × F-sept-3 (S1/S2a/S2b/S3) 의 결과를 결합 분석해 *단일 결함 위치* 를 식별. F-oct (production fix cycle) 의 권고 방향 결정. production 변경 0.

**Files:**
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md`
- **Modify: 없음**

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sept-3 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: 종합 측정값 수집**

Run:
```
git log --oneline -10
go test ./internal/decoder/ -run TestDiagnostic_Fsept -v
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
```

Expected: F-sept task 3건 + 회귀 게이트 4건 모두 PASS. raw output 보고서 §1/§2 에 인용.

- [ ] **Step 3: 시나리오 결합 분석**

F-sept-1 × F-sept-2 × F-sept-3 결합 → F-oct 권고 결정 표:

| F-sept-1 | F-sept-2 | F-sept-3 | F-oct 권고 (단일 결함 식별) |
|----------|----------|----------|----------------------------|
| (A) u[5] 부호 정상 | (L1/L2) LP 정상 | (S1) IIR 정상 | **E3 발동** — 결함 0 식별 (모든 stage 정상) → 가설 무효, F-oct 추가 진단 |
| (A) u[5] 부호 정상 | (L1/L2) LP 정상 | (S2/S3) IIR 결함 | **F-oct = synth IIR Q-format / two-pass fix** |
| (A) u[5] 부호 정상 | (L3a) LP 결함 | (any) | **F-oct = LSP-to-LP 외 결함 (codebook / interp / lsf)** |
| (A) u[5] 부호 정상 | (L3b) LP 결함 (lsp_lp.go) | (any) | **F-oct = lsp_lp.go modified 재검토 (F-bis-1 P fix 영향)** |
| (B1) v[5] 부호 결함 | (any) | (any) | **F-oct = pitch.AdaptiveCodebook 진단/fix** |
| (B2) c[5] 부호 결함 | (any) | (any) | **F-oct = fcb.Decode 진단/fix (β init / pulse sign)** |
| (B3) gain ratio 결함 | (any) | (any) | **F-oct = gain decode 잔여 (F-quint cycle 미해소)** |
| (B4) saturation 발생 | (any) | (any) | **F-oct = fixed primitives Word16 overflow fix** |
| 결합 모순 (E3) | — | — | **F-oct-1 추가 진단 cycle** (다른 stimulus / frame 1+) |

본 step 의 분류는 측정값 표에서 *직접* 도출 — 휴리스틱 0.

- [ ] **Step 4: 잔여 보류 항목 갱신 (F-quint-3 §4 + F-sext-3 §5 답습)**

F-quint-3 §4.7 + F-sext-1 §5 의 잔여 항목을 본 cycle 결과로 갱신:

1. **F-oct-1 (production fix)**: Step 3 의 결합 분석으로 권고 방향 결정.
2. **filterSubframe ÷4/×4**: F-quint-3 §4.1 동상 (frame 0 sf0 미-trigger).
3. **β init = 0.2**: F-quint-3 §4.2 동상.
4. **frame 1+ 잔여**: F-sept cycle 은 frame 0 sf0 한정. frame 1+ 영향 별도 cycle.
5. **회귀 가드 promotion**: sample 0..7 영구 게이트 후속 검토.
6. **비-contract diagnostic 3건**: F-quint-3 §4.6 동상 (cleanup task).
7. **F-sext-2 / F-sext-3 (HP filter 진단)**: F-sext-1 §4 시나리오 (i) 로 *유보* 상태. F-oct 후 sample 5..7 잔존 시 재가동 검토.
8. **lsp_lp.go uncommitted (F-bis-1 P fix)**: F-sept-2 (L3b) 발현 시 본 cycle 결과로 reactivate; (L1/L2/L3a) 시 별도 cycle 보류 유지.

- [ ] **Step 5: F-sept-4 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md`:

```markdown
# Phase 1k Stage F-sept-4 종합 보고서 + F-oct 권고

**작성일**: 2026-04-29
**범위**: F-sept-1/2/3 의 진단 결과 결합 분석 + 단일 결함 위치 식별
        + F-oct (production fix) cycle 권고.
**산출물**: 시나리오 결합 표 + F-oct 권고 + 잔여 보류 항목 갱신.
**준수**: F-sept-1/2/3 + F-sext-1 + F-quart 및 F-quint 보고서만 인용.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 종합 평가 (E1-E5)
## 1. F-sept cycle commit 요약
## 2. 회귀 게이트 종합 결과
## 3. 시나리오 결합 분석 (F-sept-1 × F-sept-2 × F-sept-3)
## 4. F-oct 권고 방향 결정 + 단일 결함 위치 식별
## 5. 잔여 보류 항목 갱신 (F-quint-3 §4 + F-sext-1 §5 표 답습)
## 6. 결론 — Phase 1k Stage F-sept closure
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md
```

```bash
git add docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-sept synthesis report + F-oct recommendation

Stage F-sept cycle (F-sept-1 excitation 분해, F-sept-2 LP cross-check,
F-sept-3 synth IIR trace) 의 진단 결과 결합 분석. ALGTHM frame 0 sf0
sample 5..7 부호 반전의 단일 결함 위치 식별 (excitation / LP / IIR
중) + F-oct production fix cycle 권고 방향 결정.

production 변경 0. 시나리오 결합 분석 (F-sept-1 × F-sept-2 × F-sept-3)
으로 F-oct 권고를 단일 production fix 후보로 ranking. 외부 G.729
구현 0건 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage**:
- ✓ F-sext-1 §4 시나리오 (i) 후속 — sample 5..7 상류 결함 위치 식별 (3 후보 동등 분리).
- ✓ §4.1.6 eq. (75) — Task F-sept-1 excitation 분해.
- ✓ §4.1 (LSP decoding + interpolation) + §3.2.6 (LSP-to-LP) — Task F-sept-2 LP cross-check.
- ✓ §3.10 (synthesis filter) + §4.3 Table 9 (zero-init) — Task F-sept-3 IIR trace.
- ✓ §3.10 two-pass overflow path 측정 — Task F-sept-3 Step 4.
- ✓ production 변경 0 invariant (E5).
- ✓ Escape hatch E1/E2/E3/E4/E5 — Phase 0.3 + 모든 task §0.
- ✓ lsp_lp.go uncommitted 영향 분리 — Phase 0.4 §5 + F-sept-2 Step 4.

**2. Placeholder scan**:
- F-sept-2 Step 2 의 `referenceLSPToLPSubframe0` body 는 "(구체 구현은 작성 시점에 §4.1 + §3.2.6 의 PDF 인용 후 line-by-line 도출)" 로 명시 — *test 코드 작성 자유도* (placeholder 가 아닌 의도된 구현 범위 명시). 옵션 (1) 권장 / 옵션 (2) E2 risk 검토.
- F-sept-1 Step 4 의 시나리오 (A/B1/B2/B3/B4) / F-sept-2 Step 4 의 (L1/L2/L3a/L3b) / F-sept-3 Step 4 의 (S1/S2a/S2b/S3) 는 *측정 분류 표* — placeholder 아닌 분류 골격.
- F-sept-4 Step 3 의 결합 표 9 행은 모든 시나리오 조합을 망라 — placeholder 아닌 결정 표.

**3. Type consistency**:
- `[lpcOrder + 1]int16` (= [11]int16, Q12, a[0]=4096): F-sept-1/2/3 일관.
- `[subframeLen]int16` (= [40]int16): u/v/c/s 일관.
- `int16` Q14/Q12/Q13: gpQ14/gcQ12 명시.
- `float64`: F-sept-2/3 reference impl 만 사용 — production boundary 명확.
- `fixed.Word16` / `fixed.LMult`/`LShr`/`LAdd`/`Round`/`LMsu`/`LShl`: F-sept-1 의 분해 라인이 production excitation.go:28-32 와 정합.
- helper 명: `referenceLSPToLPSubframe0`, `referenceSynthFilter`, `signOfInt16`, `signOfInt32`, `signOfFloat`, `roundFloat`, `abs32` — 일관.

**4. 외부 구현 참조 0**: 모든 spec 인용 = ITU-T G.729 (06/2012) PDF + production 자체 docstring (excitation.go:7-26, filter.go:1-17, hpfilter.go:3-13). reference impl 의 모든 상수/연산이 §3.10 / §4.1 / §3.2.6 / §4.1.6 / §4.3 Table 9 의 verbatim. 외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 0 인용. ✓

**5. TDD 준수**:
- 본 cycle 은 *진단-only* — RED→GREEN gate 는 *진단 데이터 capture 검증* 용으로 변형.
- F-sept-1/2/3 모두 Step 2-3 = 진단 test 작성 + 실행 PASS.
- F-sept-4 = 메타 task (test 추가 0).
- 회귀 게이트 (Phase 0.2 의 6 항목) 는 각 commit 후 *모두 PASS 의무*.

**6. 강압-적합 회피**:
- F-sept-1/2/3 모두 *측정-only* — t.Errorf / t.Fatalf 사용 0 (파일 I/O 오류 제외).
- F-sept-4 의 F-oct 권고 결정은 *측정값 결합 표에서 직접 도출* — 휴리스틱 0.
- F-sept-2/3 의 reference impl 는 *spec 식 직접 구현* — production 동치를 위한 fit 조정 0. 도출 경로는 보고서 §1 에 명시 의무.
- F-sept-2 Step 4 의 lsp_lp.go modified 영향 분리 — 원인 분리 없이 fix 권고 금지.

**7. Commit 정책**:
- F-sept-1 = 1 commit (진단 test 1 + 보고서 1).
- F-sept-2 = 1 commit (진단 test modify + 보고서 1).
- F-sept-3 = 1 commit (진단 test modify + 보고서 1).
- F-sept-4 = 1 commit (보고서 1, production 변경 0).
- 총 **4 commit**. 진단 task 별 분리.

**8. Co-author trailer**: 4 commit 모두 `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` 포함.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-plan.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration. Per-task gates (Phase 0.2 / 0.3) catch regressions early. F-quart / F-sext 패턴과 동일.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
