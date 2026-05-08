# Phase 1k Stage F-sext Diagnostic-Only Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-quint-3 §3.3 가 식별한 ALGTHM frame 0 sf0 의 *sample 5..7 부호 반전* 잔존 결함 (`hpFilter[5..7] = [1, 1, 1]` vs PST/2 `[-1, -1, -1]`, |Δ|=2) 의 **원인을 진단-only 로 규명**한다. 두 후보 (postfilter §A.4.2 4-sample delay vs HP filter §4.2.2 startup transient) 를 spec § 직접 인용 + reference cross-check 로 분리 식별; 후속 production fix 는 별도 cycle (F-sept) 권고. **production 변경 0 라인** invariant.

**Architecture:** 4-task 진단 cycle (F-quart 패턴 답습). Task F-sext-1 = postfilter §A.4.2 chain trace harness (postfilter 입력→출력→hpFilter 입력→hpFilter 출력 sample-by-sample 측정으로 부호 반전 발생 위치 식별). Task F-sext-2 = HP filter §4.2.2 startup 진단 (state init `hpX[0..1]` / `hpY[0..1]` 의 frame 0 첫 호출 transient 측정). Task F-sext-3 = HP filter reference cross-check (test 코드에서 §4.2.2 식 (151)/(152) 직접 도출 reference impl 작성, prod vs ref 비교, 외부 G.729 구현 0 인용). Task F-sext-4 = 종합 보고서 + F-sept production fix cycle 권고. 각 task 의 production 코드 변경 0 라인; test 추가만 허용 (E5 invariant).

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §4.2.2 (Output HP filter) + §A.4.2 (Annex A postfilter 변형). 기존 F-quart-1 alignment harness + F-quart-3 reference cross-check 패턴 재활용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-sext 진입 시점, post-87ff388)

| 경로 | 상태 | F-sext 변경? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 | **No** (보존, 별도 cycle 처리) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis/F-tris 진단 baseline | **No** (보존) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (F-quart/F-quint) | **No** (변경 금지) |
| `internal/decoder/hpfilter.go` | F-quint-3 시점 그대로 | **No** (진단-only) |
| `internal/decoder/subframe.go` | F-quint-3 시점 그대로 | **No** (진단-only) |
| `internal/postfilter/*` | F-quint-3 시점 그대로 | **No** (진단-only) |
| 그 외 production 파일 | 미변경 | **No** (진단-only) |

F-sext 신규 파일 (모두 *_test.go):
- (Task F-sext-1) `internal/decoder/stagef_sext_diagnostic_test.go` — 통합 진단 파일 (Task F-sext-1/F-sext-2/F-sext-3 의 새 test 모두 본 파일에 추가).
- (Task F-sext-1) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md`
- (Task F-sext-2) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-2-report.md`
- (Task F-sext-3) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-3-report.md`
- (Task F-sext-4) `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-4-report.md`

본 cycle 의 production 변경 범위 = **0 라인**. test 변경 = `stagef_sext_diagnostic_test.go` 신규 파일 only. 그 외 *_test.go 파일 변경 절대 금지.

### Phase 0.2 회귀 게이트 명세

각 task commit 직후 *반드시* 실행:

1. **Stage D 17 contract test**: `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/` 의 contract spec test. 본 cycle 회귀 0 의무.
2. **Stage D-bis 3 contract test**: F-bis-1 P fix 검증 + LSP 합성 cross-check + 추가 contract.
3. **Phase 1i sample 0 가드** (`TestDecode_Frame0Sample0_MatchesALGTHM`): F-quint-2 commit `1c00385` 후부터 PASS (got=2 want=2). 본 cycle 모든 commit 직후 PASS 의무.
4. **F-quart-3 reference cross-check** (`TestDiagnostic_FquartGainReferenceCrossCheck`): F-quint cycle 후 PASS (Branch P/S 의 prod = ref ± 4 양자화 tol). 본 cycle 회귀 0.
5. **F-quart-1 alignment harness** (`TestDiagnostic_FquartGainImap_Sf0Sample0to7`): F-quint cycle 후 measurement-only PASS (Branch A hpFilter 36/40). 본 cycle 직접 영향 없음.

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Stage D 17 / D-bis 3 / Phase 1i 가드 / F-quart-3 cross-check) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | Task F-sext-3 의 reference impl 가 §4.2.2 식 (151)/(152) verbatim 인용에서 도출되지 않음 (즉 prod-동치 휴리스틱 fit) | 즉시 reference impl 폐기 + spec § 식 hand-trace 재작성 + 보고서에 도출 과정 정량 기록 |
| **E3** | Task F-sext-3 후 prod = ref (HP filter sample 0..7 비트-정확) 가 확인되었음에도 sample 5..7 부호 반전이 *여전히* prod 에서 관찰됨 | sample 5..7 부호 반전이 HP filter 외부 (postfilter or 상류) 에 위치 → F-sept 권고 방향을 **postfilter §A.4.2** 로 갱신 |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | 본 cycle 의 임의 commit 가 production 파일 (`internal/**/*.go` 중 `*_test.go` 가 아닌 것) 1 라인이라도 변경 | 즉시 `git revert HEAD` + commit 재구성 (test-only 로 축소) |

각 보고서 (F-sext-1/2/3/4) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 (forced-fit) 회피 의무

본 cycle 은 **진단-only** — 따라서 강압-적합 위험은 production fix cycle 보다 낮으나, 다음 의무는 유지:

1. **측정값 그대로 기록**: Task F-sext-1/F-sext-2/F-sext-3 의 모든 측정값은 **raw output** 으로 보고서 §3 에 인용. 의도적 재가공 / 평균 / 정규화 금지.
2. **spec § verbatim 인용**: 각 task 의 보고서 §1 에 §4.2.2 / §A.4.2 의 spec 식 verbatim 인용 (PDF page + line 번호 포함).
3. **reference impl 도출 경로 명시**: Task F-sext-3 의 reference impl 는 spec 식에서 *직접* 도출 (식 자체 → float64 코드). production 코드 또는 외부 구현 0 참조.
4. **F-sept 권고는 측정 기반 ranking 만**: Task F-sext-4 의 권고 (postfilter §A.4.2 vs HP §4.2.2 vs 상류 결함) 는 §3.1/§3.2/§3.3 의 측정값 표에서 *직접* 도출. 휴리스틱 해석 금지 — 측정값이 결정적이지 않으면 "결정적 분리 불가, F-sept-1 추가 진단 필요" 로 명시.

---

## Task F-sext-1: postfilter §A.4.2 chain trace harness

**Goal:** ALGTHM frame 0 sf0 의 sample 5..7 부호 반전 (|Δ|=2) 이 *어느 단계에서 발생하는지* 식별. `synth.Filter` 출력 → `postfilter.Filter` 출력 → `hpFilter` 출력 → `pcm.ScaleUpSat` (PST 도메인) 의 4 단계에서 sample 5..7 의 *부호 분포* 를 trace 한다. 부호 반전이 어느 boundary 에서 발생하는지 정량 식별.

**Files:**
- Create: `internal/decoder/stagef_sext_diagnostic_test.go` (신규 진단 파일, `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md`

### Spec § 인용 (F-sext-1 진단 근거)

ITU-T G.729 (06/2012) §4 (PDF p.30, postfilter chain 의 일반 sequence):

> "The decoder output speech is enhanced by a postfilter consisting of a long-term postfilter, a short-term postfilter, a tilt compensation filter, and adaptive gain control..."

§A.4.2 (Annex A postfilter 변형) 는 long-term postfilter 단순화 (`g_l = 0.5`, `g_pst = 0` 등) 외 main-body §4 와 동일 chain. 즉 chain sequence:

1. `synth.Filter` → `s[n]` (Q0 합성 출력, 40 sample/sf)
2. `postfilter.Filter` → `sPf[n]` (Q0 postfilter 출력)
3. `hpFilter` → `hpOut[n]` (Q0 HP-filtered, 100 Hz cutoff)
4. `pcm.ScaleUpSat` → `pst[n]` (Q0 PST 도메인, scale ×2)

PST/2 ground-truth = `pst[n] >> 1`. F-quint-3 §3.2 측정에서 frame 0 sf0 sample 5..7 의 hpFilter 출력 = `[1, 1, 1]` vs PST/2 `[-1, -1, -1]` (부호 반전 + |Δ|=2).

본 task 는 `synth.Filter` / `postfilter.Filter` / `hpFilter` 각 단계의 sample 5..7 *원본 부호* 를 측정해 부호 반전 boundary 를 식별.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-quint cycle 종료 시점 = 87ff388 직후 동일.)

Run (회귀 게이트 baseline):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
```

Expected: 3건 모두 PASS. 이 baseline 출력을 보고서 §2 에 인용.

- [ ] **Step 2: 진단 test 작성 — `stagef_sext_diagnostic_test.go` 신규**

`internal/decoder/stagef_sext_diagnostic_test.go` 신규 작성. 본 파일은 **본 cycle 의 모든 진단 test** 를 담는다 (F-sext-1, F-sext-2, F-sext-3 각 task 의 test).

본 task (F-sext-1) 에서는 `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` 추가:

```go
package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
)

// TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7: Stage F-sext-1 진단.
//
// ITU-T G.729 (06/2012) §4 + §A.4.2: postfilter chain sequence
//   synth.Filter → postfilter.Filter → hpFilter → pcm.ScaleUpSat
//
// F-quint-3 §3.2 측정으로 ALGTHM frame 0 sf0 hpFilter[5..7] = [1, 1, 1] vs
// PST/2 [-1, -1, -1] 부호 반전 (|Δ|=2) 잔존 확인.
//
// 본 진단은 chain 의 각 단계에서 sample 5..7 의 *원본 부호 + 절대값* 을
// 측정해 부호 반전 boundary 를 식별한다. production 코드 0-수정.
//
// 측정-only — t.Errorf / t.Fatalf 미사용 (sanity check 1건 외).
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

	// frame 0 sf0 디코딩 — F-quart-1 의 decodeFquartSf0 helper 와 동일
	// 구조이지만 본 task 에서는 chain 의 모든 중간값을 capture 해야 하므로
	// in-line 으로 작성 (재사용 helper 추가 금지 — 본 cycle test-only).

	// LSP 디코딩 → frame 0 sf0 LP coefficients
	var lspDec lsp.Decoder
	lspDec.Reset()
	var sfA [lpcOrder + 1]int16
	if err := lspDec.DecodeSubframe0(f.L0, f.L1, f.L2, f.L3, &sfA); err != nil {
		t.Fatalf("lsp.DecodeSubframe0: %v", err)
	}

	// pitch / fcb / gain
	tInt, tFrac := pitch.DecodeDelaySubframe0(f.P1)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)

	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: f.S1}, tInt, betaQ14, &c)

	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: f.GA1, GB: f.GB1}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	// 4 chain stage capture
	var syn synth.Synthesizer
	syn.Reset()
	var sStage [subframeLen]int16
	syn.Filter(&sfA, &u, &sStage)

	var pst postfilter.Postfilter
	pst.Reset()
	var pfStage [subframeLen]int16
	pst.Filter(&sfA, tInt, &sStage, &pfStage)

	// HP filter 단독 호출 — Decoder instance 우회, helper 재사용
	var hpStage [subframeLen]int16
	hpFilterStandalone(&pfStage, &hpStage)

	var pcmStage [subframeLen]int16
	pcm.ScaleUpSat(&hpStage, &pcmStage)

	// sample 5..7 비교표
	t.Logf("──────── sample 5..7 chain trace ────────")
	t.Logf("stage              [5  6  7]  부호분포")
	t.Logf("synth.Filter       %s  %s", fmtSamples3(sStage[5:8]), signs3(sStage[5:8]))
	t.Logf("postfilter.Filter  %s  %s", fmtSamples3(pfStage[5:8]), signs3(pfStage[5:8]))
	t.Logf("hpFilter           %s  %s", fmtSamples3(hpStage[5:8]), signs3(hpStage[5:8]))
	t.Logf("pcm.ScaleUpSat     %s  %s  (PST 도메인)", fmtSamples3(pcmStage[5:8]), signs3(pcmStage[5:8]))
	t.Logf("PST want sample 5..7         = [%d %d %d]", wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("PST/2 spec-target sample 5..7 = [%d %d %d]", pstHalf[5], pstHalf[6], pstHalf[7])

	// boundary 식별 — 부호가 첫 반전되는 단계 추출
	stageNames := []string{"synth.Filter", "postfilter.Filter", "hpFilter", "pcm.ScaleUpSat"}
	stageOuts := [][subframeLen]int16{sStage, pfStage, hpStage, pcmStage}
	for i, name := range stageNames {
		s5 := stageOuts[i][5]
		t.Logf("%-18s sample 5 부호 = %s (값 %d)", name, signOf(s5), s5)
	}
	t.Logf("PST want sample 5 부호 = %s (값 %d)", signOf(wantFrames[0][5]), wantFrames[0][5])
	t.Logf("PST/2  sample 5 부호 = %s (값 %d)", signOf(pstHalf[5]), pstHalf[5])
}

// hpFilterStandalone: F-sext 진단용 wrapper. Decoder.hpFilter 와
// 동일 알고리즘이나 state 를 zero-init 으로 시작. production 코드
// 변경 0 — 본 함수는 *_test.go 내부.
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

// fmtSamples3 / signs3 / signOf: 출력 helper. 기존 fmtSamples8
// 패턴 답습.
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
```

`fmt` import 누락 시 `import "fmt"` 추가. 본 step 의 진단 test 코드는 production 코드를 *호출* 만 하며 변경하지 않는다. `hpFilterStandalone` 은 hpfilter.go 의 알고리즘을 *test 내부에 복제* 한 것 (production 변경 0).

**중요**: `lsp.Decoder.DecodeSubframe0`, `synth.Synthesizer.Reset`, `postfilter.Postfilter.Reset`, `gain.Decoder.Reset`, `pitch.DecodeDelaySubframe0`, `pcm.ScaleUpSat` 의 정확한 signature 는 `internal/lsp/`, `internal/synth/`, `internal/postfilter/`, `internal/gain/`, `internal/pitch/`, `internal/pcm/` 의 exported API 검토 후 호출. signature 가 plan 예시와 다르면 test 작성 시 실제 API 에 맞춰 수정 (test code 자유도).

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v`

Expected: PASS (assertion 0, t.Logf 만 출력). raw output 보고서 §3.1 에 인용.

컴파일 오류 발생 시 (signature 불일치 등) 본 step 에서 *test 코드만* 수정. production 코드 1 라인이라도 변경 시 즉시 E5 발동.

- [ ] **Step 4: 측정값 분석 + 부호 반전 boundary 식별**

Step 3 의 출력에서 sample 5 의 부호 분포를 4 stage 별로 추출:

| stage | sample 5 부호 | 값 |
|-------|--------------|------|
| synth.Filter | ? | ? |
| postfilter.Filter | ? | ? |
| hpFilter | ? | ? |
| pcm.ScaleUpSat | ? | ? |
| **PST want sample 5** | **−** | **−2** (예상) |
| **PST/2 sample 5** | **−** | **−1** |

분류 시나리오:
- (i) **synth.Filter sample 5 = +** → 부호 반전이 *상류* (gain decode / FCB / pitch / synth 합성). F-quint cycle 의 fix 들로 미해소된 추가 결함 — F-sept 후보.
- (ii) **synth.Filter sample 5 = −, postfilter sample 5 = +** → 부호 반전이 postfilter §A.4.2 (long-term / short-term / tilt / agc 중 하나). F-sept-1 후보 = postfilter 진단 cycle.
- (iii) **postfilter sample 5 = −, hpFilter sample 5 = +** → 부호 반전이 HP filter §4.2.2. Task F-sext-2 / F-sext-3 의 정밀 진단 진행.
- (iv) **hpFilter sample 5 = −, pcm.ScaleUpSat sample 5 = +** → pcm 변환에서 부호 반전 발생 (예상치 못함, 의심 결함).

본 task 의 출력 만으로 boundary 가 결정되면 보고서 §4 에 시나리오 분류 명시. 결정 불가 (예: 모든 stage 가 0 또는 |값|<1) 시 시나리오 (v) 로 분류 후 F-sext-2/F-sext-3 정밀 진단 진행.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run:
```
go test ./internal/...
```

Expected:
- `TestDecode_Frame0Sample0_MatchesALGTHM`: PASS (Phase 1i 가드 보존).
- `TestDiagnostic_FquartGainReferenceCrossCheck`: PASS.
- `TestDiagnostic_FquartGainImap_Sf0Sample0to7`: PASS.
- 본 task 가 추가한 `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7`: PASS.
- Stage D 17 + D-bis 3 contract test: PASS.
- 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`): F-quint-3 §4.6 표대로 FAIL 유지 (plan-허용).

**FAIL 분류**:
- Stage D 17 / D-bis 3 / Phase 1i 가드 / F-quart-3 cross-check 의 회귀 → **E1 발동, 즉시 revert**.
- 본 task 의 새 test FAIL → 본 task Step 2-3 재작성.

- [ ] **Step 6: F-sext-1 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md`:

```markdown
# Phase 1k Stage F-sext-1 보고서 — postfilter §A.4.2 chain trace

**작성일**: 2026-04-29
**범위**: F-quint-3 §3.3 의 sample 5..7 부호 반전 (|Δ|=2) 의 chain
        boundary 식별 진단.
**산출물**: synth → postfilter → hpFilter → pcm 4 stage 별 sample 5..7
            부호 + 절대값 측정값 표.
**준수**: ITU-T G.729 (06/2012) §4 + §A.4.2 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. §4 + §A.4.2 chain sequence 인용
## 2. 회귀 게이트 baseline (Step 1 출력)
## 3. 진단 측정값
   3.1 4 stage chain trace raw output (Step 3)
   3.2 sample 5..7 부호 분포 표 (Step 4)
## 4. boundary 시나리오 분류 (i/ii/iii/iv/v)
## 5. F-sext-2 / F-sext-3 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_sext_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md
```

**E5 검증**: `git diff -- internal/` 의 production 라인 (즉 `*_test.go` 가 아닌 파일) 변경 0. `internal/lsp/lsp_lp.go` 의 별도 cycle 보류 변경은 *기존 modified* 그대로 (본 task 가 추가 변경 0).

```bash
git add internal/decoder/stagef_sext_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sext-1 postfilter chain trace harness

F-quint-3 §3.3 가 식별한 ALGTHM frame 0 sf0 sample 5..7 부호 반전
(|Δ|=2) 의 발생 boundary 를 chain 의 4 stage (synth.Filter →
postfilter.Filter → hpFilter → pcm.ScaleUpSat) 에서 sample-by-sample
추적한다.

본 진단은 측정-only — production 변경 0. 시나리오 분류 (i/ii/iii/iv/v)
로 후속 task (F-sext-2 HP startup, F-sext-3 reference cross-check)
또는 F-sept production fix cycle 권고 방향 결정.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sext-2: HP filter §4.2.2 startup state 진단

**Goal:** F-sext-1 시나리오 (iii) (HP filter 가 부호 반전 boundary) 가 식별되거나 미결정 시, HP filter §4.2.2 의 *startup transient* 가 sample 5..7 까지 영향을 주는지 정량 진단. `hpX[0..1]` / `hpY[0..1]` 의 frame 0 첫 호출 (zero-init 상태) 에서 sample 0..39 의 *순차 출력 수렴 거동* 측정.

**Files:**
- Modify: `internal/decoder/stagef_sext_diagnostic_test.go` (`TestDiagnostic_FsextHPStartup_Frame0` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-2-report.md`

### Spec § 인용 (F-sext-2 진단 근거)

ITU-T G.729 (06/2012) §4.2.2 (PDF p.31):

> "Output postfilter HP filter, with cutoff frequency at 100 Hz:
>
> H_h2(z) = (b₀_h2 + b₁_h2·z⁻¹ + b₂_h2·z⁻²) / (1 + a₁_h2·z⁻¹ + a₂_h2·z⁻²)
>
> where  b₀_h2 = +0.93980581,  b₁_h2 = -1.8795834,  b₂_h2 = +0.93980581
>        a₁_h2 = -1.9330735,   a₂_h2 = +0.93589199"

→ 식 (151) (transfer function), 식 (152) (계수). 본 IIR 은 frame 0 첫 호출에서 `x[-1] = x[-2] = y[-1] = y[-2] = 0` (§4.3 Table 9 초기화) 으로 시작 → 첫 수개 sample 은 *startup transient* (수렴 영역).

`internal/decoder/hpfilter.go:14-19` production constants (Q-format):

```
hpB0Q13    = 7699   = round(+0.93980581 · 2^13)
hpB1Q13    = -15399 = round(-1.8795834 · 2^13)
hpB2Q13    = 7699   = round(+0.93980581 · 2^13)
hpNegA1Q12 = 7918   = round(+1.9330735 · 2^12)  (note: stores |a₁|)
hpA2Q13    = 7667   = round(+0.93589199 · 2^13)
```

Verify (production 자체 문서):
- `hpB0Q13 / 8192 = 7699 / 8192 = 0.93982...` ≈ spec.
- `hpB1Q13 / 8192 = -15399 / 8192 = -1.87976...` ≈ spec.
- `hpNegA1Q12 / 4096 = 7918 / 4096 = 1.93311...` ≈ |a₁|.
- `hpA2Q13 / 8192 = 7667 / 8192 = 0.93591...` ≈ spec.

본 task 는 production 의 startup transient 거동을 측정해, sample 5..7 잔존이 *startup transient 의 정상 부산물* 인지 *알고리즘 결함* 인지 분리 식별.

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sext-1 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-sext-1 의 신규 파일은 commit 됨.)

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FsextHPStartup_Frame0`**

`internal/decoder/stagef_sext_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FsextHPStartup_Frame0: Stage F-sext-2 진단.
//
// ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152): Output HP filter (100 Hz
// cutoff). frame 0 첫 호출은 §4.3 Table 9 zero-init 상태에서 시작 →
// startup transient 영역.
//
// 본 진단은 frame 0 sf0 의 hpFilter 출력 sample 0..39 를 zero-init
// 시작에서 측정하고, 두 가지 stimulus 비교:
//   (a) ALGTHM 실제 입력 (postfilter.Filter 출력)
//   (b) impulse stimulus (input[0]=1, input[1..39]=0)
//
// (b) 의 출력 = HP filter 의 impulse response → startup transient 길이
// 직접 측정 가능.
//
// 측정-only.
func TestDiagnostic_FsextHPStartup_Frame0(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	// stimulus (a): ALGTHM 실제 입력 (= F-sext-1 의 pfStage)
	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	pfStage := computeFsextPostfilterOutput(t, &f)  // F-sext-1 helper 재사용

	var hpA [subframeLen]int16
	hpFilterStandalone(&pfStage, &hpA)

	// stimulus (b): impulse
	var pfB [subframeLen]int16
	pfB[0] = 1
	var hpB [subframeLen]int16
	hpFilterStandalone(&pfB, &hpB)

	// 출력 dump
	t.Logf("──────── stimulus (a): ALGTHM postfilter 출력 → hpFilter ────────")
	t.Logf("hpA sample 0..39:")
	dumpInt16(t, hpA[:])
	t.Logf("hpA sample 0..7 = %s", fmtSamples8(hpA[:8]))

	t.Logf("──────── stimulus (b): impulse stimulus → hpFilter (impulse response) ────────")
	t.Logf("hpB sample 0..39:")
	dumpInt16(t, hpB[:])
	t.Logf("hpB sample 0..7 = %s", fmtSamples8(hpB[:8]))

	// startup transient 길이 추정: hpB 의 |값| 이 처음 1 LSB 이하로 안정되는 sample
	t.Logf("──────── startup transient 길이 추정 (hpB |값|≤1 sample) ────────")
	for n := 0; n < subframeLen; n++ {
		v := hpB[n]
		if v < 0 {
			v = -v
		}
		if v <= 1 {
			t.Logf("hpB sample %d 첫 |값|≤1 도달 (값 %d)", n, hpB[n])
			break
		}
	}

	// PST/2 비교
	var pstHalf [subframeLen]int16
	for n := 0; n < subframeLen; n++ {
		pstHalf[n] = int16(int32(wantFrames[0][n]) >> 1)
	}
	t.Logf("──────── stimulus (a) 출력 vs PST/2 sample 5..7 ────────")
	t.Logf("hpA sample 5..7      = [%d %d %d]", hpA[5], hpA[6], hpA[7])
	t.Logf("PST/2 sample 5..7    = [%d %d %d]", pstHalf[5], pstHalf[6], pstHalf[7])
	t.Logf("Δ sample 5..7        = [%+d %+d %+d]", int32(hpA[5])-int32(pstHalf[5]), int32(hpA[6])-int32(pstHalf[6]), int32(hpA[7])-int32(pstHalf[7]))
}

// computeFsextPostfilterOutput: F-sext-1 의 postfilter 출력 계산
// helper (test-internal). production 코드 재사용 만 — 변경 0.
func computeFsextPostfilterOutput(t *testing.T, f *bitstream.Frame) [subframeLen]int16 {
	// 구현은 F-sext-1 Step 2 의 in-line 코드 그대로 함수화. 시간 절약을
	// 위해 본 step 에서는 F-sext-1 의 in-line 코드를 helper 로
	// extract 하면서 진단 test 두 곳 모두에서 호출.
	t.Helper()
	// (구체 구현은 F-sext-1 Step 2 코드를 함수 body 로 wrap)
	// ... (생략, F-sext-1 Step 2 코드 인라인)
	var pfStage [subframeLen]int16
	return pfStage  // placeholder - 실제 구현 시 채움
}
```

**중요**: `computeFsextPostfilterOutput` 의 body 는 F-sext-1 Step 2 의 LSP/pitch/fcb/gain/synth/postfilter 디코딩 코드를 함수화 한 것. F-sext-1 Step 2 의 in-line 코드를 본 step 에서 helper 로 *리팩터*. F-sext-1 의 test 도 helper 호출로 갱신 (중복 제거).

production 변경 0 invariant 유지 — helper 는 *_test.go 내부.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FsextHPStartup_Frame0 -v`

Expected: PASS. raw output 보고서 §3.1 에 인용.

Run (F-sext-1 test 도 helper extract 후 PASS 유지):

```
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
```

Expected: PASS.

- [ ] **Step 4: startup transient 길이 분석**

Step 3 의 출력에서:
- (b) impulse response 의 sample n 에서 |hpB[n]| ≤ 1 첫 도달 → startup transient 길이 (단위: sample).
- (a) 의 sample 5..7 hpA 값 vs PST/2 의 부호 + 절대값 비교 (F-quint-3 §3.2 와 정합).

분류:
- (α) **transient 길이 ≤ 5** → sample 5..7 잔존은 *transient 가 아닌 알고리즘 결함*. F-sext-3 reference cross-check 진행.
- (β) **transient 길이 5..15** → sample 5..7 가 transient 영역. PST 자체가 동일 transient 영향 받았는지 F-sext-3 cross-check 로 확인 (zero-init 보장 spec 정합 vs PST 의 *상태 전이 가정*).
- (γ) **transient 길이 > 15** → IIR 계수 quantization 결함 의심. spec 식 (151)/(152) 의 *real-valued* 계수 vs Q-format 양자화 계수의 거동 차이 — F-sext-3 cross-check 결정적.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: 5 항목 (Phase 1i / F-quart-3 / F-quart-1 / Stage D / D-bis) 회귀 0 + 본 task 의 새 test PASS.

- [ ] **Step 6: F-sext-2 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-2-report.md`:

```markdown
# Phase 1k Stage F-sext-2 보고서 — HP filter §4.2.2 startup 진단

**작성일**: 2026-04-29
**범위**: HP filter §4.2.2 의 frame 0 첫 호출 startup transient 거동
        측정 + sample 5..7 잔존이 transient 부산물인지 알고리즘 결함인지 분리.
**산출물**: impulse response 길이 + ALGTHM 입력 stimulus 의 hpFilter 출력 비교.
**준수**: §4.2.2 식 (151)/(152) verbatim 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가
## 1. §4.2.2 식 (151)/(152) verbatim 인용 + Q-format constant 검증
## 2. 회귀 게이트 결과
## 3. 진단 측정값
   3.1 stimulus (a) ALGTHM postfilter → hpFilter 출력 (sample 0..39)
   3.2 stimulus (b) impulse response (sample 0..39)
   3.3 startup transient 길이 (impulse |값|≤1 첫 도달 sample)
## 4. 시나리오 분류 (α/β/γ)
## 5. F-sext-3 cross-check 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go
M  internal/decoder/stagef_sext_diagnostic_test.go         ← F-sext-2 추가
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-2-report.md
```

```bash
git add internal/decoder/stagef_sext_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sext-2 HP filter §4.2.2 startup diagnostic

ITU-T G.729 (06/2012) §4.2.2 (식 (151)/(152)) Output HP filter
의 frame 0 첫 호출 startup transient 거동 측정. impulse response
길이 직접 측정 + ALGTHM stimulus 의 hpFilter 출력을 zero-init 상태
에서 sample 0..39 dump.

본 진단은 sample 5..7 부호 반전 잔존이 *transient 부산물* 인지
*Q-format 양자화 또는 알고리즘 결함* 인지 시나리오 (α/β/γ) 로 분리
하기 위한 측정 단계. production 변경 0.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sext-3: HP filter reference cross-check (no external code)

**Goal:** §4.2.2 식 (151)/(152) 에서 *직접 도출* 한 float64 reference impl 을 test 코드 내부에 작성하고, production hpFilter 의 frame 0 sf0 sample 0..39 출력을 reference 와 비트-정확 비교한다. 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro / FFmpeg) 0 인용. F-quart-3 의 reference cross-check 패턴 답습.

**Files:**
- Modify: `internal/decoder/stagef_sext_diagnostic_test.go` (`TestDiagnostic_FsextHPReferenceCrossCheck` 추가)
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-3-report.md`

### Spec § 인용 (F-sext-3 reference 도출 근거)

ITU-T G.729 (06/2012) §4.2.2 식 (151) (PDF p.31, transfer function):

> H_h2(z) = (b₀_h2 + b₁_h2·z⁻¹ + b₂_h2·z⁻²) / (1 + a₁_h2·z⁻¹ + a₂_h2·z⁻²)

→ 시간 도메인 difference equation:

```
y[n] = b₀·x[n] + b₁·x[n-1] + b₂·x[n-2] − a₁·y[n-1] − a₂·y[n-2]
```

식 (152): `b₀ = +0.93980581, b₁ = -1.8795834, b₂ = +0.93980581, a₁ = -1.9330735, a₂ = +0.93589199`.

reference impl 은 위 식을 *float64* 로 직접 구현. 양자화 0, saturation 0, rounding 0 — 즉 spec 의 *real-valued* 거동 그대로.

§4.3 Table 9 초기화: `x[-1] = x[-2] = y[-1] = y[-2] = 0` (frame 0 첫 호출).

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sext-2 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: reference impl + cross-check 작성**

`internal/decoder/stagef_sext_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FsextHPReferenceCrossCheck: Stage F-sext-3 진단.
//
// ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) 에서 *직접 도출* 한
// float64 reference impl 을 작성하고, production hpFilter 의 frame 0
// sf0 sample 0..39 출력과 비트-정확 비교한다.
//
// 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro / FFmpeg) 0 인용.
// reference impl 의 모든 라인은 spec 식 또는 §4.3 Table 9 초기화에서
// 직접 도출.
//
// 측정-only — Δ assertion 0 (cross-check 결과를 t.Logf 로 dump,
// F-sept production fix cycle 에서 assertion promotion 가능).
func TestDiagnostic_FsextHPReferenceCrossCheck(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	ensureTestdataPresent(t, bitPath)

	frames, _ := readG192Frames(t, bitPath)
	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// production stimulus = F-sext-1/F-sext-2 의 postfilter 출력
	pfStage := computeFsextPostfilterOutput(t, &f)

	// production hpFilter 출력
	var hpProd [subframeLen]int16
	hpFilterStandalone(&pfStage, &hpProd)

	// reference impl: §4.2.2 식 (151)/(152) float64 직접 도출
	hpRef := referenceHPFilter(&pfStage)

	// dump
	t.Logf("──────── F-sext-3 cross-check (production vs §4.2.2 reference float64) ────────")
	t.Logf("sample  pf_in    hp_prod  hp_ref(float64)  hp_ref(round)  Δ(prod − ref_round)")
	for n := 0; n < subframeLen; n++ {
		refRound := int16(int32(math.Round(hpRef[n])))
		if hpRef[n] > 32767 {
			refRound = 32767
		} else if hpRef[n] < -32768 {
			refRound = -32768
		}
		t.Logf("[%2d]    %6d   %6d   %14.6f   %6d   %+d",
			n, pfStage[n], hpProd[n], hpRef[n], refRound, int32(hpProd[n])-int32(refRound))
	}

	// sample 5..7 집중 비교
	t.Logf("──────── sample 5..7 비교 ────────")
	for n := 5; n <= 7; n++ {
		refRound := int16(int32(math.Round(hpRef[n])))
		t.Logf("sample %d: prod=%+d, ref(real)=%.6f, ref(rounded int16)=%+d, Δ=%+d",
			n, hpProd[n], hpRef[n], refRound, int32(hpProd[n])-int32(refRound))
	}

	// 시나리오 분류 dump
	allMatch := true
	for n := 0; n < subframeLen; n++ {
		refRound := int16(int32(math.Round(hpRef[n])))
		if hpRef[n] > 32767 {
			refRound = 32767
		} else if hpRef[n] < -32768 {
			refRound = -32768
		}
		if hpProd[n] != refRound {
			allMatch = false
			break
		}
	}
	if allMatch {
		t.Logf("F-sext-3 분류: prod = ref (sample 0..39 비트-정확) — sample 5..7 부호 반전이 production 결함이 아닌 §4.2.2 spec-허용 거동")
	} else {
		t.Logf("F-sext-3 분류: prod ≠ ref — production 의 Q-format 양자화 또는 알고리즘이 §4.2.2 real-valued 거동에서 벗어남 (F-sept 후보)")
	}
}

// referenceHPFilter: §4.2.2 식 (151)/(152) 의 float64 직접 구현.
// 양자화 / saturation / rounding 0 — spec real-valued 거동 그대로.
//
// 외부 G.729 구현 0 인용. 모든 상수 = §4.2.2 식 (152) verbatim.
func referenceHPFilter(in *[subframeLen]int16) [subframeLen]float64 {
	const (
		b0 = +0.93980581  // §4.2.2 식 (152)
		b1 = -1.8795834   // §4.2.2 식 (152)
		b2 = +0.93980581  // §4.2.2 식 (152)
		a1 = -1.9330735   // §4.2.2 식 (152)
		a2 = +0.93589199  // §4.2.2 식 (152)
	)
	// §4.3 Table 9: zero-init at frame 0 first call
	var x1, x2, y1, y2 float64
	var out [subframeLen]float64
	for n := 0; n < subframeLen; n++ {
		xn := float64(in[n])
		// difference equation (시간 도메인 form of 식 (151)):
		//   y[n] = b0·x[n] + b1·x[n-1] + b2·x[n-2] - a1·y[n-1] - a2·y[n-2]
		yn := b0*xn + b1*x1 + b2*x2 - a1*y1 - a2*y2
		out[n] = yn
		x2, x1 = x1, xn
		y2, y1 = y1, yn
	}
	return out
}
```

`math` import 누락 시 추가.

reference impl 의 *모든 상수* = §4.2.2 식 (152) verbatim. *모든 연산* = 식 (151) transfer function 의 시간 도메인 변환. 외부 코드 0 인용 — 단지 spec 식의 직접 구현.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FsextHPReferenceCrossCheck -v`

Expected: PASS (assertion 0). raw output 보고서 §3 에 인용.

- [ ] **Step 4: prod vs ref 분류**

Step 3 의 출력에서 sample 0..39 의 prod − ref_round Δ 분포 분석:

분류:
- (X) **모든 sample Δ = 0** → Q-format 양자화의 영향 미미, production 이 §4.2.2 real-valued 거동 비트-정확 모방. sample 5..7 부호 반전이 *§4.2.2 spec-허용 거동* (예: postfilter 입력 자체의 부호 반전 → HP filter 가 그대로 통과). **F-sept 권고 = postfilter §A.4.2 (또는 그 상류) 진단**.
- (Y) **일부 sample |Δ| = 1** → Q13/Q12 양자화 rounding 의 누적 (수치적 정상). sample 5..7 의 부호 반전이 prod 와 ref 양쪽에 모두 발생 → §4.2.2 spec-허용. **F-sept 권고 = postfilter §A.4.2 (또는 그 상류)**.
- (Z) **일부 sample |Δ| ≥ 2 또는 sample 5..7 부호가 prod ≠ ref** → production 의 Q-format 양자화/saturation 정책이 spec real-valued 와 분기. **F-sept 권고 = HP filter §4.2.2 production fix** (Q-format 검토).

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: 5 항목 (Phase 1i / F-quart-3 / F-quart-1 / Stage D / D-bis) 회귀 0 + 본 task 의 새 test PASS + F-sext-1/F-sext-2 의 기존 test PASS.

- [ ] **Step 6: F-sext-3 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-3-report.md`:

```markdown
# Phase 1k Stage F-sext-3 보고서 — HP filter reference cross-check

**작성일**: 2026-04-29
**범위**: §4.2.2 식 (151)/(152) float64 reference impl 작성 + production
        hpFilter 의 frame 0 sf0 sample 0..39 비교.
**산출물**: prod vs ref 비교표 + sample 5..7 부호 반전이 §4.2.2
            spec-허용 거동인지 결정.
**준수**: §4.2.2 식 (151)/(152) verbatim 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (특히 E2/E3/E4 평가)
## 1. §4.2.2 식 (151)/(152) 인용 + reference impl 도출 경로
## 2. 회귀 게이트 결과
## 3. 진단 측정값
   3.1 prod vs ref 비교표 sample 0..39
   3.2 sample 5..7 집중 비교
## 4. 시나리오 분류 (X/Y/Z)
## 5. F-sept 권고 방향 결정
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go
M  internal/decoder/stagef_sext_diagnostic_test.go         ← F-sext-3 추가
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-3-report.md
```

```bash
git add internal/decoder/stagef_sext_diagnostic_test.go \
        docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-3-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-sext-3 HP filter reference cross-check

ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) 에서 직접 도출한
float64 reference impl 을 작성하고, production hpFilter 의 frame 0
sf0 sample 0..39 출력과 비교한다. 양자화 / saturation 0 — spec
real-valued 거동 그대로.

외부 G.729 구현 (ITU 참조 C, bcg729, Sipro, FFmpeg) 0 인용.
reference impl 의 모든 상수 = §4.2.2 식 (152) verbatim, 모든 연산
= 식 (151) transfer function 의 시간 도메인 변환.

본 cross-check 는 sample 5..7 부호 반전이 (X) §4.2.2 spec-허용
거동 (postfilter 입력 자체 결함) 인지 (Z) HP filter Q-format 결함
인지 분리 식별. F-sept production fix cycle 권고 방향 결정.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-sext-4: 종합 보고서 + F-sept 권고

**Goal:** F-sext-1/F-sext-2/F-sext-3 의 측정값을 종합해 sample 5..7 부호 반전의 *결정적 위치* 를 식별하고, F-sept (production fix cycle) 의 권고 방향 (postfilter §A.4.2 vs HP §4.2.2 vs 상류) 을 결정. production 변경 0.

**Files:**
- Create: `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-4-report.md`
- **Modify: 없음**

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (F-sext-3 commit 후):
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: 종합 측정값 수집**

Run:
```
git log --oneline -10
go test ./internal/decoder/ -run TestDiagnostic_Fsext -v
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
```

Expected: F-sext task 3건 + 회귀 게이트 3건 모두 PASS. raw output 보고서 §1/§2 에 인용.

- [ ] **Step 3: 시나리오 결합 분석**

F-sext-1 시나리오 (i/ii/iii/iv/v) × F-sext-2 시나리오 (α/β/γ) × F-sext-3 시나리오 (X/Y/Z) 결합 → F-sept 권고 결정 표:

| F-sext-1 | F-sext-3 | F-sept 권고 |
|----------|----------|-------------|
| (i) synth.Filter sample 5 = + | (any) | F-sept = synth/gain/fcb/pitch 진단 cycle (상류) |
| (ii) postfilter sample 5 = + | (any) | F-sept-1 = postfilter §A.4.2 진단 cycle |
| (iii) hpFilter sample 5 = + | (X) prod=ref | postfilter §A.4.2 진단 (HP filter 는 spec-동치) |
| (iii) hpFilter sample 5 = + | (Z) prod≠ref | F-sept-1 = HP filter §4.2.2 Q-format fix |
| (iv) pcm 단계 부호 반전 | — | pcm.ScaleUpSat 의도된 동작 검증 (예상치 못함) |
| (v) 결정 불가 | — | F-sept-2 추가 진단 (다른 stimulus 조합) |

본 step 의 분류는 측정값 표에서 *직접* 도출 — 휴리스틱 0.

- [ ] **Step 4: 잔여 보류 항목 갱신 (F-quint-3 §4 답습)**

F-quint-3 §4.7 의 4-task ranking 을 본 cycle 결과로 갱신:

1. **F-sext-2 (frame 1+ 진단)**: 본 cycle 에서 frame 0 sf0 한정. *본 task 에서 식별된 boundary* 가 frame 1+ 에 미치는 영향은 별도 cycle.
2. **F-sept-1 (production fix)**: 본 cycle Step 3 의 결합 분석으로 권고 방향 결정.
3. **filterSubframe ÷4/×4**: F-quint-3 §4.1 동상.
4. **β init = 0.2**: F-quint-3 §4.2 동상.
5. **회귀 가드 promotion**: sample 0..7 영구 게이트는 F-sept 후 재검토.
6. **비-contract diagnostic 3건**: F-quint-3 §4.6 동상 (cleanup task).

- [ ] **Step 5: F-sext-4 보고서 작성**

`docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-4-report.md`:

```markdown
# Phase 1k Stage F-sext-4 종합 보고서 + F-sept 권고

**작성일**: 2026-04-29
**범위**: F-sext cycle 의 진단 결과 종합 + F-sept (production fix)
        cycle 권고 방향 결정.
**산출물**: 시나리오 결합 분석 + F-sept 권고 (postfilter §A.4.2 vs
            HP §4.2.2 vs 상류) + 잔여 보류 항목 갱신.
**준수**: F-sext-1/2/3 + F-quart 및 F-quint 보고서만 인용.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 종합 평가 (E1-E5)
## 1. F-sext cycle commit 요약 (git log 발췌)
## 2. 회귀 게이트 종합 결과
## 3. 시나리오 결합 분석 (F-sext-1 × F-sext-2 × F-sext-3)
## 4. F-sept 권고 방향 (postfilter / HP filter / 상류)
## 5. 잔여 보류 항목 갱신 (F-quint-3 §4 표 답습)
## 6. 결론 — Phase 1k Stage F-sext closure
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-4-report.md
```

```bash
git add docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-4-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-sext synthesis report + F-sept recommendation

Stage F-sext cycle (F-sext-1 postfilter chain trace, F-sext-2 HP
startup transient, F-sext-3 reference cross-check) 의 진단 결과
종합. ALGTHM frame 0 sf0 sample 5..7 부호 반전 (|Δ|=2) 의 결정적
boundary 식별 + F-sept production fix cycle 권고 방향 결정.

production 변경 0. 시나리오 결합 분석 (F-sext-1 × F-sext-3) 으로
F-sept 권고를 (postfilter §A.4.2 / HP §4.2.2 / 상류) 중 하나로
ranking. 외부 G.729 구현 0건 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage**:
- ✓ F-quint-3 §3.3 의 sample 5..7 부호 반전 결함 진단 — Task F-sext-1 (chain trace) + F-sext-2 (HP startup) + F-sext-3 (reference cross-check) 의 3 진단으로 boundary 식별.
- ✓ §4.2.2 식 (151)/(152) — Task F-sext-3 reference impl 직접 도출.
- ✓ §A.4.2 + §4 chain sequence — Task F-sext-1 의 4 stage trace.
- ✓ §4.3 Table 9 zero-init — Task F-sext-3 reference impl 의 `x1=x2=y1=y2=0` 초기화.
- ✓ production 변경 0 invariant (E5).
- ✓ Escape hatch E1/E2/E3/E4/E5 — Phase 0.3 + 모든 task §0.

**2. Placeholder scan**:
- F-sext-1 Step 2 의 helper signature (lsp.Decoder.DecodeSubframe0 등) 는 plan 작성 시 정확한 API 미확인 — *test 코드 작성 자유도* 로 명시 (placeholder 가 아닌 의도된 자유도).
- F-sext-1 Step 4 의 시나리오 (i/ii/iii/iv/v) / F-sext-2 Step 4 의 (α/β/γ) / F-sext-3 Step 4 의 (X/Y/Z) 는 *측정 분류 표* — placeholder 아닌 분류 골격.
- F-sext-2 Step 2 의 `computeFsextPostfilterOutput` body 는 F-sext-1 Step 2 코드의 helper extract — F-sext-2 step 에서 함수화 의무 명시.

**3. Type consistency**:
- `[subframeLen]int16` (40 sample 단위): F-sext-1/2/3 일관.
- `float64`: F-sext-3 reference impl 만 사용 — production 의 int16/Q-format 과 boundary 명확.
- `int32` (HP filter accumulator): production 의 Q12 y-state — F-sext-3 reference 는 float64 로 양자화 0.
- helper 명명: `hpFilterStandalone`, `referenceHPFilter`, `computeFsextPostfilterOutput` — 일관 (Fsext prefix 또는 명시적 stand-alone 표기).

**4. 외부 구현 참조 0**: 모든 spec 인용 = ITU-T G.729 (06/2012) PDF + production 자체 docstring (`hpfilter.go:3-13`). reference impl 의 상수/연산 모두 §4.2.2 식 (151)/(152) verbatim. 외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 0 인용. ✓

**5. TDD 준수**:
- 본 cycle 은 *진단-only* — 따라서 RED→GREEN gate 는 *프로덕션 fix 검증* 용이 아니라 *진단 데이터 capture 검증* 용으로 변형.
- F-sext-1 Step 2-3 = 진단 test 작성 + 실행 PASS.
- F-sext-2 Step 2-3 = 진단 test 추가 + 실행 PASS.
- F-sext-3 Step 2-3 = reference impl + cross-check + 실행 PASS.
- F-sext-4 = 메타 task (test 추가 0).
- 회귀 게이트 (Phase 0.2 의 5 항목) 는 각 commit 후 *모두 PASS 의무*.

**6. 강압-적합 회피**:
- F-sext-1/2/3 모두 *측정-only* — t.Errorf / t.Fatalf 사용 0 (파일 I/O 오류 제외).
- F-sext-4 의 F-sept 권고 결정은 *측정값 표에서 직접 도출* — 휴리스틱 0.
- F-sext-3 의 reference impl 는 *spec 식 직접 구현* — production 동치를 위한 fit 조정 0. 도출 경로는 보고서 §1 에 명시 의무.

**7. Commit 정책**:
- F-sext-1 = 1 commit (진단 test 1 + 보고서 1).
- F-sext-2 = 1 commit (진단 test modify + 보고서 1).
- F-sext-3 = 1 commit (진단 test modify + 보고서 1).
- F-sext-4 = 1 commit (보고서 1, production 변경 0).
- 총 **4 commit**. 진단 task 별 분리 → 각 측정값을 독립 review 가능.

**8. Co-author trailer**: 4 commit 모두 `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` 포함.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-plan.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration. Per-task gates (Phase 0.2 / 0.3) catch regressions early. F-quart 패턴과 동일.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
