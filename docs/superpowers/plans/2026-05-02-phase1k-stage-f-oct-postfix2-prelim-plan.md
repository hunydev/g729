# Phase 1k Stage F-oct-postfix2-prelim Diagnostic Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Phase 1k Stage F-oct-postfix synthesis (`8907847`) §3 의 후보 ③ ("M1 외 후보 재진입") 을 채택. 사용자 G1 결정 = "(c) Annex A binary 거부 + 후보 ③ pivot". F-oct-postfix-2 의 G3 (γ_t 선택 분기 단독 결함) 가설이 측정 (Δ=0) 으로 *반증* 된 후, F-oct-prelim-5-4 §3.6 의 "M1 (postfilter conditional 분기) 단독 채택" 결정을 *postfix-2 의 측정 데이터* 로 재평가. 새로운 4 가설 후보군 — **M1'** (postfilter 외 분기: agcGain / longterm / shortterm 의 다른 분기), **M3** (synth IIR §3.10 chain — F-oct-prelim-5-3 폐기 결정 재평가), **M5** (excitation pre-postfilter 부호 결함 — F-sept 에서 cross-check 하였으나 sample 5..7 한정 미수행), **M6** (PST want 데이터 자체 부호 결함 — F-oct-prelim-5-1 의 P-SRC-2 분류 재해석) — 을 *측정 데이터만으로* 비교하여 다음 fix cycle 의 단일 위치를 식별. 본 cycle 은 **production 변경 0 라인** 진단 cycle. 잔존 RED contract = `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` (`56caa72`) 가 다음 fix cycle 의 GREEN gate 로 승계.

**Architecture:** 5-task 진단 cycle (TDD 패턴 — failing/측정 test → dump → commit). Task F-oct-postfix2-prelim-1 = 5-stage chain dump harness (단일 frame 0 sf0 sample 5..7 의 chain stage 출력을 일괄 dump, 4 가설 비교의 *공통 ground-truth*). Task F-oct-postfix2-prelim-2 = M5 정밀 측정 (sample 5..7 한정 excitation 부호 + pre-postfilter chain trace). Task F-oct-postfix2-prelim-3 = M6 정밀 측정 (PST want 데이터의 byte-level 부호 + Annex A spec sample format 재검증). Task F-oct-postfix2-prelim-4 = M1' / M3 정밀 측정 (postfilter 의 longterm/agc/shortterm 분기 cover 점검 + synth IIR memory propagation 재진단). Task F-oct-postfix2-prelim-5 = 종합 + 4 가설 비교표 + 다음 cycle 결정 (production fix cycle 또는 추가 진단 cycle).

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §A.4.2.1-5 (Annex A postfilter chain) + §A.3.* (Annex A decoder excitation / synthesis) + READMETV.txt (PST file format) + 기존 F-quart/F-sext/F-sept/F-oct-prelim/F-oct-prelim-5/F-oct-postfix-1 진단 하니스 (회귀 게이트 15건). **외부 G.729 구현 0건 참조** (E4) — 사용자 G1 결정으로 **Annex A reference C binary 사용 금지** (black-box 행동 추적 포함). 단 이미 repo committed 인 PST 파일 (`testdata/itu/test_vectors/`) 은 입력 stimulus 로 계속 사용 가능.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 직전 cycle 의 결정 / 측정 정리

**직전 cycle = F-oct-postfix (`8907847` synthesis)**:

- F-oct-postfix-1 (`56caa72`): RED test `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` 추가 — ALGTHM frame 0 sf0 sample 5..7 의 production 출력 `[+2,+2,+2]` ↔ PST want `[-1,-1,-1]` (Δ=3 each) mismatch 를 assertion 으로 표현. 본 cycle 진입 시점 RED 잔존 (다음 fix cycle 의 GREEN gate).
- F-oct-postfix-2 (revert): `internal/postfilter/tilt.go:67` 의 γ_t 선택 분기 조건을 `pf.agcGainPrev == 0` (Phase 1g proxy) → `k1 >= 0` (spec §A.4.2.3 strict reading) 으로 fix 시도. fix 적용 후 sample 5..7 출력 = pre-fix `[+2,+2,+2]` 와 byte-동상 (Δ=0). 분기 flip 이 sample 5..7 부호/크기 모두 무영향 입증 → **G3 (γ_t 분기 단독 결함) 가설 반증**. tilt.go 는 `git checkout -- internal/postfilter/tilt.go` 로 revert (synthesis report §0.1 입증).
- F-oct-postfix synthesis (`8907847`): E3 발동 + Task 3/4 skip + Task 5 직진. §3 의 3 후보 (① g_l 영속화 / ② Annex A binary trace / ③ M1 외 후보 재진입) 비교 + 사용자 G1 결정 = "(c) Annex A binary 거부 + 후보 ③ pivot".

**G3 반증의 함의** (synthesis §2.4):

- γ_t 값 선택이 sample 5..7 출력 (16-bit signed) 의 LSB 단위 (Δ=3) 에 영향 없음 → tilt μ Q15 곱셈 결과가 sample 5..7 단위에서 *지배적 항이 아님*.
- 부호 자체가 `+` (got=2) ↔ spec want `-` (want=-1) 무변화 → **부호 결정 항** 이 tilt compensation **외부** 에 위치.
- 후보 위치: (i) 합성 LP filter 출력 (synth IIR), (ii) long-term postfilter (`longterm.go`) g_l 적용 단계, (iii) AGC (`agc.go`) update path, (iv) high-pass filter (`hpfilter.go`) post-AGC, (v) excitation 자체 (synth 입력), (vi) PST want 데이터 자체의 부호 재해석.

**F-oct-prelim-5-4 §3.6 의 M1 단독 채택 결정** (synthesis §2.3):

- M1 (postfilter conditional 분기) 단독 채택 결정의 *암묵적 가정* = "γ_t 분기 fix 가 sample 5..7 부호 결함의 충분 조건" 은 본 cycle 로 반증.
- M1 → M1' (postfilter 외 분기) / M3 재진입 / M5 (excitation 부호) / M6 (PST want 부호) 의 4 가설 후보군으로 갱신.
- Phase 0.4 강압-적합 회피 의무에 따라 *측정 데이터만으로* 비교.

### Phase 0.2 invariant 재확인 (E1-E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.3 의 1~14 PASS 항목, 단 항목 15 = postfix-1 RED 는 *예외 — 의도된 RED 잔존*) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | 본 cycle 임의 task 의 spec § 인용이 PDF verbatim grep 결과와 불일치 (= 휴리스틱 fit) | 즉시 측정 폐기 + spec § PDF 직접 재발췌 + 보고서 §0 에 도출 과정 정량 기록 |
| **E3** | 본 cycle Task 5 종합에서 4 가설 중 2+ 가 잔존 (단일 식별 불가) | Task 5 §4 다음 cycle 권고 = "추가 진단 cycle (각 잔존 가설별 분리 측정)". 단일 fix cycle 진입 금지. |
| **E4** | 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조/실행 흔적 발견. **사용자 G1 결정 = Annex A binary 거부** (black-box 행동 추적 포함). | 즉시 작업 중단 + 사용자 통보 + 해당 인용/binary 제거 후 재시작 |
| **E5** | 본 cycle 의 production 변경 라인 > 0 (메타 의무 — 진단 cycle) | 즉시 `git revert HEAD` + commit 재구성 (production 변경 제거) |

### Phase 0.3 회귀 게이트 명세 (15건)

각 task commit 직후 *반드시* 실행 (총 15 게이트 — F-oct-postfix-1 시점의 14 + 본 cycle 의 신규 측정 harness 1; 단 항목 15 = postfix-1 RED 는 *항상 RED 의무* — 다음 fix cycle 의 GREEN gate):

| # | 그룹 | test | 의무 |
|---|------|------|------|
| 1 | F-quart | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 2 | F-quart | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 3 | F-sext | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 4 | F-sept | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 5 | F-sept | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 6 | F-sept | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 7 | F-oct-prelim | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 8 | F-oct-prelim | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |
| 9 | F-oct-prelim | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS |
| 10 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` | PASS |
| 11 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5BitVectorCompare` | PASS |
| 12 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5HpFilterInitState` | PASS |
| 13 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` | PASS |
| 14 | ITU contract | `TestDecode_Frame0Sample0_MatchesALGTHM` (Phase 1i sample 0 invariant) | PASS |
| 15 | F-oct-postfix-1 RED | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **RED 잔존 의무** (다음 fix cycle 의 GREEN gate) |

추가 sanity:
- `go vet ./...` clean (각 task commit 직후).
- 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 본 cycle 진입 시점 FAIL 유지. 본 cycle 어떤 task 도 본 3건의 상태를 변화시키지 *않아야* 함 (production 변경 0 라인 의무로 자동 보장).

본 cycle Task 1 의 신규 측정 harness (`TestDiagnostic_FoctPostfix2PrelimChainDump`) 는 *측정-only* (assertion 0 또는 PASS 의무) — 회귀 게이트 16번째 항목으로 자동 승격되지 *않는다* (자동 promotion 금지, F-quint-3 §4.6 동상). Task 5 §3 잔여 보류 항목 처리.

### Phase 0.4 강압-적합 회피 의무 (forced-fit avoidance)

본 cycle 은 *진단 cycle* 이며 Phase 0.4 의무가 production fix cycle 보다 *더 엄격*. 다음 패턴을 적극 회피:

1. **가설별 측정 분리 의무**: 4 가설 (M1' / M3 / M5 / M6) 비교 시 *측정 데이터만* 사용. spec 인용 또는 직관적 추론으로 가설 우선순위를 결정하지 않는다. 각 task 의 측정 결과를 단일 표 (Task 5 §2 4 가설 비교표) 로 결합하여 단일 식별.
2. **spec § 인용 우회 fit 금지**: F-oct-prelim-5-4 §3.6 의 spec 인용 ("γ_t = 0.9 if g_l > 0") 이 PDF 원문 부재로 §A.4.2.2 와 §A.4.2.3 결합 해석으로 도출되었음 (synthesis §2.1 입증). 본 cycle 의 M1' / M3 / M5 / M6 spec 인용은 PDF verbatim grep 결과로만 채택. 결합 해석 또는 *간접 증거* 는 보고서 §0 에서 명시.
3. **음성 결과 (가설 반증) 도 결과로 인정**: 4 가설 중 0건 식별 (= 모두 반증) 도 *유효한 측정 결과*. 이 경우 Task 5 §4 다음 cycle 권고 = "추가 spec 영역 확장 진단 cycle (예: §A.3.* Annex A decoder 의 excitation generation, 또는 §A.4.1 LP synthesis filter)".
4. **scope crawl 금지**: 본 cycle 모든 task 의 production 변경 = 0 라인. test 변경 = 측정 harness 신규만. helper 신규 0 (기존 `decoder` package helper 재사용). spec 인용은 §A.* 영역 한정.
5. **g_l 영속화 후보 ① 제외 의무**: 사용자 G1 결정 = "후보 ③ pivot" — 따라서 본 cycle 은 ① (g_l state 영속화 + tilt.go read) 와 관련된 측정/fix 를 일체 도입하지 *않는다*. 단 M1' (postfilter 외 분기) 측정에서 longterm.go 분기 cover 점검 시 g_l 의 *측정값* (raw output) 은 dump 가능 (state 영속화 변경 없음).
6. **G3 반증 사실 정정 의무**: F-oct-prelim-5-4 §3.6 row C3 의 "M1 단독 채택" 결정이 본 cycle 진입 premise 로 *반증* 됨을 각 task §0 보고서에 명시. 정정 commit 별도 작성 의무는 synthesis report §5 G4 = (c) "본 보고서 §2.3 으로 정정 사실 명시 충분" 에 따라 본 plan 자체로 갈음.

### Phase 0.5 사전 보유 working tree 보존 의무

`internal/decoder/stagef_bis_diagnostic_test.go` (untracked, F-bis baseline 잔존) 는 본 cycle 5 task 어떤 commit 도 add 하지 않는다. 사후 working tree 의 `?? internal/decoder/stagef_bis_diagnostic_test.go` 가 F-oct-postfix synthesis 시점 (`8907847`) 과 동일하게 유지됨을 각 task §0 보고서에서 확인.

---

## Spec § 인용 (본 cycle 의 ground-truth 공통)

**(인용 1)** ITU-T G.729 (06/2012) PDF page 43, §A.4.2 *Postfilter*, verbatim (chain 의 stage 순서):

```
A.4.2   Postfilter
The postfilter consists of the following parts:
        - long-term postfilter Hp(z) (clause A.4.2.2)
        - short-term postfilter Hf(z) (clause A.4.2.1)
        - tilt-compensation filter Ht(z) (clause A.4.2.3)
        - adaptive gain control AGC (clause A.4.2.4)
```

**(인용 2)** ITU-T G.729 (06/2012) PDF page 43, §A.4.2.3 (postfix-1/2 ground-truth 동상):

```
A.4.2.3       Tilt compensation
... The value of γ_t = 0.8 is used if k1' < 0 and γ_t is set to zero
if k1' ≥ 0. The gain factor gt which is used in clause 4.2.3 is eliminated.
```

**(인용 3)** ITU-T G.729 (06/2012) PDF §A.3 *Decoder* (Annex A decoder 의 excitation / LP synthesis chain — Task 1/2 ground-truth). PDF page 39-42, §A.3.1-§A.3.5 의 excitation reconstruction + LP synthesis 단계.

**(인용 4)** READMETV.txt (PST 파일 format ground-truth): F-oct-prelim-1/-5-1 시점 인용 동상 — 16-bit little-endian signed PCM, frame = 80 sample, ALGTHM/PITCH/FIXED 등 vector category 별 별도 파일. (Task 3 M6 측정의 ground-truth.)

각 task 는 본 §의 인용을 baseline 으로 채택. 추가 spec 인용 시 해당 task §0 에 PDF page + verbatim 추가.

---

## Task F-oct-postfix2-prelim-1: 5-stage chain dump harness — 4 가설 공통 ground-truth

**Goal:** Annex A postfilter chain 의 5 stage (excitation → synth IIR → long-term postfilter → short-term postfilter → tilt → AGC → hpfilter) 출력을 frame 0 sf0 sample 5..7 한정으로 *일괄 dump* 하는 측정-only 진단 test 1 개 (`TestDiagnostic_FoctPostfix2PrelimChainDump`) 를 추가. 본 dump 가 Task 2 (M5) / Task 3 (M6) / Task 4 (M1' + M3) 의 *공통 ground-truth* — 각 가설 측정의 baseline 정량 자료. assertion 0 (PASS 의무, t.Logf 측정만).

**Files:**
- Create: `internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go`
- Create: `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-1-report.md`

production 변경 0 라인. test 변경 = 신규 1 파일.

### Spec § 인용

본 plan 상단 "Spec § 인용" 인용 1 (chain stage 순서) + §A.3 (excitation / synth — Task 의무에 따라 PDF page verbatim 인용 추가).

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
8907847 docs(plans): F-oct-postfix synthesis + cycle decision (E3)
```

기타 파일 modified 잔존 시 즉시 사용자 통보.

Run (회귀 게이트 baseline, Phase 0.3 의 14 PASS 항목 + 항목 15 RED 잔존):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5)" -v
go test ./internal/postfilter/ -v -run Contract
go test ./internal/synth/ -v -run Contract
go vet ./...
```

Expected: 14 PASS + 항목 15 RED + `go vet` clean. 출력 요약을 보고서 §1 에 인용.

- [ ] **Step 2: chain stage 식별 + dump 점 결정**

본 plan 상단 인용 1 의 stage 순서 + 기존 production 코드 (`internal/postfilter/postfilter.go`, `internal/synth/synthesizer.go`, `internal/decoder/decode.go`) 를 grep 으로 stage boundary 식별:

```
grep -n "func " internal/synth/synthesizer.go internal/postfilter/postfilter.go internal/decoder/subframe.go
```

dump 점 결정 (frame 0 sf0 sample 5..7 한정):

| stage | 이름 | dump 위치 | dump 형식 |
|-------|------|-----------|-----------|
| 1 | excitation u[n] (Q0 / int32) | synth.Filter 호출 직전 | `excitation[5..7]` |
| 2 | synth IIR 출력 syn[n] (Q0) | synth.Filter 호출 직후 | `syn[5..7]` |
| 3 | long-term postfilter 출력 (Q0) | postfilter.Filter 내부 longterm 단계 직후 | (test 에서 직접 dump 불가 시 §0 에 한계 명시) |
| 4 | short-term postfilter 출력 (Q0) | shortterm 단계 직후 | (동상) |
| 5 | tilt 출력 (Q0) | tilt 단계 직후 | (동상) |
| 6 | AGC 출력 (Q0) | AGC 단계 직후 | (동상) |
| 7 | hpfilter 출력 (Q0, int16) | decoder.Decode 의 hpfilter 호출 직후 | `out[5..7]` |

내부 stage (3-6) 가 test 에서 직접 측정 불가 시 (production API 가 stage 별 출력을 노출하지 않는 경우), test 는 *외부 관찰 가능* 한 stage (1, 2, 7) 만 dump + 나머지는 Task 4 의 M1' 측정에서 별도 harness 로 추가. 본 한계를 보고서 §0 에 명시.

- [ ] **Step 3: dump harness test 작성 — `stagef_octpostfix2_prelim_diagnostic_test.go`**

`internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go` 신규 작성 (구체 코드 — Step 2 측정 가능 stage 한정):

```go
package decoder

import "testing"

// TestDiagnostic_FoctPostfix2PrelimChainDump dumps the Annex A postfilter
// chain stage outputs for ALGTHM frame 0 sf0 sample 5..7 — the common
// ground-truth for Tasks F-oct-postfix2-prelim-2/3/4 (M5/M6/M1'+M3
// hypothesis differential measurement).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.4.2 (PDF p.43) chain
// order = long-term → short-term → tilt → AGC. F-oct-postfix synthesis
// (8907847) §2.4 identifies the sign-determining term as residing
// *outside* tilt compensation (Δ=0 measurement); this dump enables
// stage-by-stage sign tracing.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FoctPostfix2PrelimChainDump(t *testing.T) {
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

	t.Logf("ALGTHM frame 0 sf0 sample 5..7 (PST want = [%d %d %d])",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("  decoded out[5..7] (post-hpfilter)            = [%d %d %d]",
		out[5], out[6], out[7])
	t.Logf("  delta vs PST want                            = [%d %d %d]",
		int32(out[5])-int32(wantFrames[0][5]),
		int32(out[6])-int32(wantFrames[0][6]),
		int32(out[7])-int32(wantFrames[0][7]))
	// Additional stage dumps (excitation, synth IIR, postfilter chain)
	// are added in Tasks 2/4 via stage-specific harnesses or Decoder
	// instrumentation hooks if exposed; this baseline records the
	// externally observable terminal output for cross-reference.
}
```

내부 stage (3-6) dump 가 가능한 경우 (예: `Decoder` 가 debug hook 또는 stage별 trace 를 노출), 본 test 에 추가 stage 의 raw 값 t.Logf. 노출 부재 시 Step 2 한계 명시 + Task 4 의 M1' 측정에서 `internal/postfilter` package 내부 white-box test 로 보완.

helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `readPSTFrames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0.

- [ ] **Step 4: 측정 + 정량 기록**

Run:
```
go build ./...
go test ./internal/decoder/ -run TestDiagnostic_FoctPostfix2PrelimChainDump -v
```

Expected: build PASS, test PASS, t.Logf 출력에 sample 5..7 의 stage별 raw 값 + Δ 정량. 출력 verbatim 을 보고서 §2 에 인용.

- [ ] **Step 5: 14 회귀 게이트 PASS + 항목 15 RED 재확인**

Run: Phase 0.3 의 14 PASS + 항목 15 RED 잔존.

14 PASS 의무. 1+ FAIL 시 E1 발동.

- [ ] **Step 6: 보고서 작성**

`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-1-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix2-prelim-1 보고서 — 5-stage chain dump baseline

**작성일**: 2026-05-02
**범위**: Annex A postfilter chain stage dump harness 추가 (측정-only).
**산출물**: `stagef_octpostfix2_prelim_diagnostic_test.go` 신규 1 파일 + dump raw 출력 verbatim.
**준수**: G3 반증 후 4 가설 (M1'/M3/M5/M6) 측정의 공통 baseline.
**production 변경**: 0 라인. **테스트 변경**: 1 신규 파일.

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G1 결정 정합성
## 1. 회귀 게이트 baseline (14 PASS + 항목 15 RED)
## 2. chain stage dump raw 출력 (sample 5..7)
## 3. F-oct-postfix synthesis §2.4 의 "tilt 외 부호 결정 항" 식별 정량 baseline
## 4. Task 2/3/4 진입 의무 항목 (4 가설 측정의 공통 ground-truth 인계)
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go
?? docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-1-report.md
```

```bash
git add internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-postfix2-prelim-1 chain dump baseline

F-oct-postfix synthesis (8907847) §3 후보 ③ pivot — 사용자 G1 결정
= "(c) Annex A binary 거부 + 후보 ③ pivot". G3 (γ_t 분기 단독 결함)
가설 반증 후 4 가설 후보 (M1'/M3/M5/M6) 측정의 공통 ground-truth 로
ALGTHM frame 0 sf0 sample 5..7 의 chain stage 출력을 일괄 dump.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조
(Annex A binary 사용 금지 — G1 결정).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix2-prelim-2: M5 정밀 측정 — excitation pre-postfilter 부호 trace

**Goal:** **M5 가설** = excitation pre-postfilter signal 의 부호 결함이 sample 5..7 부호 mismatch 의 원인. F-sept 에서 cross-check 했으나 sample 5..7 한정 미수행 (F-sept-1 = sample 5 단독 분해, F-sept-3 = synth IIR 0..7 trace 이나 부호 결정 단계 미식별). 본 task = sample 5..7 한정 excitation 부호 + synth IIR 입력/출력 부호 + post-IIR pre-postfilter 부호의 *3-point sign trace* 측정. M5 가설 채택/반증의 단일 입증.

**Files:**
- Modify: `internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go` (M5 측정 함수 1개 추가 — 기존 baseline test 옆에 병기)
- Create: `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-2-report.md`

production 변경 0 라인. test 변경 = 기존 파일에 함수 1 추가.

### Spec § 인용

ITU-T G.729 (06/2012) §A.3.5 *Excitation reconstruction* (PDF page ~41) + §A.4.1 *LP synthesis filter* (PDF page ~42-43) — excitation u[n] 생성 + IIR 합성 단계의 spec 정의. Task 진입 시 PDF verbatim grep 으로 인용 확정.

- [ ] **Step 1: 사전 조건 + Task 1 commit hash 인용**

Run: `git log --oneline -2`

Expected: Task 1 commit + `8907847`.

- [ ] **Step 2: spec § PDF verbatim grep**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.5"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.4.1"
```

verbatim 인용을 보고서 §0 spec 인용 섹션에 기록. 인용 grep 결과와 본 plan 의 §A.3.5 / §A.4.1 추정이 불일치 시 즉시 E2 발동.

- [ ] **Step 3: M5 측정 함수 추가 — `TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace`**

기존 `stagef_octpostfix2_prelim_diagnostic_test.go` 에 함수 1 추가 — sample 5..7 한정 excitation u[5..7] 의 부호 + synth IIR 직후 syn[5..7] 부호 + pre-postfilter (postfilter.Filter 입력) 부호 측정. F-sept-1 의 excitation 분해 helper 재사용 가능 시 재사용 (예: subframe 의 adaptive + fixed contribution 분해).

측정 출력 형식 (예시):
```
[M5 sample 5] excitation u[5]=<int>  syn[5]=<int>  pre-post[5]=<int>  sign chain=[+|-,+|-,+|-]
[M5 sample 6] ... (동상)
[M5 sample 7] ... (동상)
[M5 결정] 부호 전환 단계: stage <N> at sample <M>; sign-determining term identified at <stage>
```

내부 stage 가 production API 로 노출되지 않는 경우, `internal/synth` package 내 white-box test 또는 `internal/decoder/subframe.go` 의 trace hook 활용. helper 신규 0 의무.

- [ ] **Step 4: 측정 + 정량 기록 + M5 가설 평가**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace -v`

Expected: PASS, t.Logf 출력 verbatim 을 보고서 §2 에 인용.

M5 가설 평가 (보고서 §3):

| 측정 결과 | M5 가설 평가 |
|-----------|--------------|
| excitation u[5..7] 부호 = `[-,-,-]` (PST want 정합) but post-IIR / post-postfilter 부호 = `[+,+,+]` | M5 **반증** — excitation 자체는 spec 정합, 결함은 chain 후단 |
| excitation u[5..7] 부호 = `[+,+,+]` (PST want 와 반전) | M5 **유력** — excitation 자체 부호 결함, fix scope = `internal/synth/excitation.go` 또는 `internal/fcb` / `internal/pitch` |
| excitation = mixed (예: `[-,+,-]`) | M5 **부분** — sample 별 분리 분석 필요, Task 5 종합으로 인계 |

- [ ] **Step 5: 15 회귀 게이트 재확인**

Run: 14 PASS + 항목 15 RED + 신규 측정 PASS.

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-2-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix2-prelim-2 보고서 — M5 excitation pre-postfilter 부호 trace

**작성일**: 2026-05-02
**범위**: M5 가설 (excitation pre-postfilter 부호 결함) 측정.
**산출물**: 측정 함수 1 추가 + 3-point sign trace + M5 가설 평가.
**준수**: production 변경 0, 외부 G.729 0 참조, F-sept sample 5..7 한정 미수행 보완.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§A.3.5, §A.4.1)
## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인
## 2. M5 측정 raw 출력 (sample 5..7 3-point sign trace)
## 3. M5 가설 평가 (반증 / 유력 / 부분)
## 4. F-sept-1/-3 측정 결과와의 cross-check
## 5. Task 3 진입 의무 (M6 측정 baseline)
```

```bash
git add internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-postfix2-prelim-2 M5 excitation sign trace

M5 가설 (excitation pre-postfilter signal 부호 결함) 의 sample 5..7
한정 측정. F-sept 에서 cross-check 하였으나 sample 5..7 한정 미수행 →
본 task 로 보완. excitation u[5..7] / synth IIR 직후 syn[5..7] /
pre-postfilter 부호 3-point trace 로 부호 결정 단계 식별.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix2-prelim-3: M6 정밀 측정 — PST want 데이터 부호 검증

**Goal:** **M6 가설** = ALGTHM.PST 의 sample 5..7 want 값 (`[-1,-1,-1]`) 자체가 P-SRC-2 (PST source / format) 결함으로 잘못 해석된 결과 — 즉 우리 production 출력 `[+2,+2,+2]` 가 실제 spec want 와 정합하고, mismatch 가 PST 파일 *읽기* 측에서 발생. F-oct-prelim-5-1 의 P-SRC-2 분류 (PST source verbatim 측정) 를 sample 5..7 한정으로 재해석. 본 task = ALGTHM.PST 파일의 sample 5..7 byte-level 검증 + endianness 재확인 + READMETV.txt format 정의 재발췌 + 다른 PST vector (PITCH/FIXED/etc.) 의 frame 0 sf0 sample 5..7 부호 분포 측정.

**Files:**
- Modify: `internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go` (M6 측정 함수 1개 추가)
- Create: `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-3-report.md`

production 변경 0 라인. test 변경 = 기존 파일에 함수 1 추가.

### Spec § 인용

READMETV.txt (testdata/itu/test_vectors/ 디렉토리 또는 명세된 위치) — PST 파일 format 정의. Task 진입 시 grep 으로 verbatim 재발췌.

- [ ] **Step 1: 사전 조건 + Task 2 commit hash 인용**

Run: `git log --oneline -3`

- [ ] **Step 2: READMETV.txt + PST byte 검증**

Run:
```
find . -name "READMETV.txt" -exec cat {} \;
xxd testdata/itu/test_vectors/ALGTHM.PST | head -20
```

(파일 경로는 본 repo 의 실제 위치로 조정 — `find` 으로 식별.)

frame 0 의 sample 5..7 byte offset 계산 (16-bit little-endian signed; sample 5 offset = 5×2 = 10 bytes from frame 0 start; frame 0 start = 0 if 단일 vector):

```
sample 5 byte offset = 10..11
sample 6 byte offset = 12..13
sample 7 byte offset = 14..15
```

xxd 출력에서 해당 byte 의 verbatim hex + 정규 little-endian 해석:
- byte sequence `FF FF FF FF FF FF` → int16 little-endian = `[-1, -1, -1]` (M6 가설 반증 — PST 부호 정합)
- byte sequence `02 00 02 00 02 00` → int16 little-endian = `[+2, +2, +2]` (M6 가설 유력 — PST 부호가 우리 production 출력과 동상)
- 그 외 → 별도 해석

- [ ] **Step 3: M6 측정 함수 추가 — `TestDiagnostic_FoctPostfix2PrelimM6PSTSignVerify`**

기존 `stagef_octpostfix2_prelim_diagnostic_test.go` 에 함수 1 추가 — 다음 측정:

1. ALGTHM.PST byte offset 10..15 의 raw hex (Step 2 cross-check 의 in-test 자동화).
2. F-oct-prelim-5-1 의 PSTSourceVerbatim helper 재사용 (있을 경우) 또는 `os.ReadFile` + binary.LittleEndian.Uint16.
3. 다른 PST vector (PITCH.PST, FIXED.PST, LSP.PST, SPEECH.PST 등 repo 내 가용 파일) 의 frame 0 sf0 sample 5..7 부호 분포 측정. 다수 vector 가 부호 `[-,-,-]` 분포 → spec want 부호의 *반복 패턴* (M6 반증). 다수 vector 가 부호 `[+,+,+]` ↔ ALGTHM 만 `[-,-,-]` → ALGTHM 단독 anomaly (M6 부분 유력).

```go
func TestDiagnostic_FoctPostfix2PrelimM6PSTSignVerify(t *testing.T) {
	// (a) ALGTHM byte-level verification
	pstPath := vectorPath("ALGTHM.PST")
	raw, err := os.ReadFile(pstPath)
	if err != nil { t.Fatalf("ReadFile: %v", err) }
	t.Logf("ALGTHM.PST byte offset 10..15 = % x", raw[10:16])
	for n := 5; n <= 7; n++ {
		off := n * 2
		v := int16(binary.LittleEndian.Uint16(raw[off : off+2]))
		t.Logf("  sample %d (offset %d..%d): hex=% x → int16 LE = %d",
			n, off, off+1, raw[off:off+2], v)
	}

	// (b) Multi-vector sample 5..7 sign distribution
	vectors := []string{"ALGTHM.PST", "PITCH.PST", "FIXED.PST", /* ... */}
	for _, v := range vectors {
		path := vectorPath(v)
		if _, err := os.Stat(path); err != nil { continue }
		// dump sample 5..7
	}
}
```

- [ ] **Step 4: 측정 + M6 가설 평가**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPostfix2PrelimM6PSTSignVerify -v`

Expected: PASS, raw 출력 verbatim 을 보고서 §2 에 인용.

M6 가설 평가 (보고서 §3):

| 측정 결과 | M6 가설 평가 |
|-----------|--------------|
| ALGTHM.PST byte 10..15 = little-endian int16 `[-1,-1,-1]` 정합 + 다른 vector 도 `[-,-,-]` 분포 다수 | M6 **반증** — PST want 부호 자체는 정상, 결함은 production 출력측 |
| ALGTHM.PST byte 10..15 = `[+2,+2,+2]` 또는 endianness 불일치 발견 | M6 **유력** — PST 읽기 코드 (`readPSTFrames`) 또는 format 해석 결함, fix scope = test helper |
| ALGTHM.PST 단독 `[-,-,-]` ↔ PITCH/FIXED 등 `[+,+,+]` 분포 | M6 **부분** — ALGTHM vector 자체의 anomaly, F-oct-prelim-5-1 P-SRC-2 재해석 필요 |

- [ ] **Step 5: 15 회귀 게이트 재확인**

Run: 14 PASS + 항목 15 RED + 신규 측정 PASS.

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-3-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix2-prelim-3 보고서 — M6 PST want 부호 검증

**작성일**: 2026-05-02
**범위**: M6 가설 (PST want 데이터 부호 결함, P-SRC-2 재해석) 측정.
**산출물**: 측정 함수 1 추가 + ALGTHM.PST byte-level + multi-vector 분포 + M6 평가.
**준수**: F-oct-prelim-5-1 P-SRC-2 분류 재해석 + READMETV.txt verbatim.

## 0. escape hatch 평가 + READMETV.txt verbatim 재발췌
## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인
## 2. ALGTHM.PST byte 10..15 raw hex + int16 LE 해석
## 3. multi-vector frame 0 sf0 sample 5..7 부호 분포
## 4. M6 가설 평가 (반증 / 유력 / 부분)
## 5. F-oct-prelim-5-1 P-SRC-2 분류 재해석 결론
## 6. Task 4 진입 의무 (M1' + M3 측정 baseline)
```

```bash
git add internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-3-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-postfix2-prelim-3 M6 PST want sign verification

M6 가설 (PST want 데이터 부호 자체 결함, P-SRC-2 재해석) 의 byte-level
검증. ALGTHM.PST byte offset 10..15 의 raw hex + int16 little-endian
해석 + multi-vector frame 0 sf0 sample 5..7 부호 분포 측정. F-oct-prelim-5-1
의 P-SRC-2 분류 재해석.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix2-prelim-4: M1' + M3 정밀 측정 — postfilter 외 분기 + synth IIR 재진단

**Goal:** **M1' 가설** = postfilter 의 *γ_t 외* 분기 (longterm.go 의 g_l clamp / agc.go 의 α-smoothing / shortterm.go 의 분기) 가 sample 5..7 부호 결함의 cover 결손. **M3 가설 재진입** = synth IIR 의 memory propagation (F-oct-prelim-5-3 §3.3 의 "zero dump → M3 폐기" 결정 재평가, sample 5..7 한정으로). 본 task = 두 가설을 단일 측정 함수로 결합 — postfilter 내부 stage 별 출력 dump (white-box, `internal/postfilter` package 내 test 추가) + synth IIR memory state pre/post sample 5..7 dump.

**Files:**
- Create: `internal/postfilter/stagef_octpostfix2_prelim_diagnostic_test.go` (M1' white-box 측정)
- Create: `internal/synth/stagef_octpostfix2_prelim_diagnostic_test.go` (M3 IIR memory 측정)
- Create: `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-4-report.md`

production 변경 0 라인. test 변경 = 신규 2 파일.

### Spec § 인용

ITU-T G.729 (06/2012) §A.4.2.1 *Short-term postfilter* + §A.4.2.2 *Long-term postfilter* + §A.4.2.4 *AGC* + §A.4.1 *LP synthesis filter* (Task 진입 시 PDF verbatim grep).

- [ ] **Step 1: 사전 조건 + Task 3 commit hash 인용**

Run: `git log --oneline -4`

- [ ] **Step 2: spec § PDF verbatim grep + F-oct-prelim-5-3 §3.3 인용 재독**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 25 "A.4.2.1"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 25 "A.4.2.2"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 20 "A.4.2.4"
```

F-oct-prelim-5-3 §3.3 의 M3 폐기 근거 ("synth IIR memory init = zero dump") 재독 + 본 task 의 sample 5..7 한정 재진단의 정당화 근거 (synthesis §2.4 "후보 위치 (i) 합성 LP filter 출력") 를 보고서 §0 에 명시.

- [ ] **Step 3: M1' 측정 — `internal/postfilter/stagef_octpostfix2_prelim_diagnostic_test.go`**

postfilter package 내 white-box test 추가 — sample 5..7 한정 longterm 출력 / agc 출력 / shortterm 출력 / tilt 출력 의 raw 값 + 부호. 각 stage 의 분기 cover (예: longterm.go 의 `R<=0||E==0 → return 16384,0` 분기 진입 여부, agc.go 의 α-smoothing 분기) 도 측정.

`internal/postfilter` package 내부 함수 (`computeLongTermGain`, `applyAGC`, `applyShortTerm`, `computeTiltMu`) 직접 호출 가능 — input = F-sept-3 dump 의 frame 0 sf0 sample 0..7 excitation / syn 값을 reproducer 로 fixture 화. fixture 부재 시 별도 helper 작성 (test 만, production 0).

측정 출력 형식 (예시):
```
[M1' sample 5] longterm out=<int> branch=(active|inactive) g_l_Q14=<int>
              agc out=<int> alpha_branch=(...) agcGainPrev=<int>
              shortterm out=<int>
              tilt out=<int> γ_t_Q14=<int>
[M1' sample 6] ... (동상)
[M1' sample 7] ... (동상)
[M1' 결정] 부호 결정 stage = <stage>; cover 결손 분기 = <분기 ID 또는 부재>
```

- [ ] **Step 4: M3 측정 — `internal/synth/stagef_octpostfix2_prelim_diagnostic_test.go`**

synth package 내 white-box test 추가 — sample 5..7 한정 IIR memory state pre (= sample 4 직후) / post (= sample 7 직후) dump. F-oct-prelim-5-3 §3.3 의 "zero dump" 가 frame 0 sf0 sample 0..4 한정이었음을 cross-check + sample 5..7 의 IIR memory propagation pattern 측정.

측정 출력 형식 (예시):
```
[M3 IIR memory pre-sample-5]  mem[0..9] = [...]
[M3 IIR memory post-sample-7] mem[0..9] = [...]
[M3 결정] sample 5..7 동안 memory 부호 변화 = <yes|no>; M3 가설 평가 = <유력|반증>
```

- [ ] **Step 5: 측정 + M1' / M3 가설 평가**

Run:
```
go test ./internal/postfilter/ -run TestDiagnostic_FoctPostfix2PrelimM1Prime -v
go test ./internal/synth/ -run TestDiagnostic_FoctPostfix2PrelimM3IIRMemory -v
```

가설 평가 표 (보고서 §3):

| 가설 | 측정 결과 → 평가 |
|------|-------------------|
| M1' (postfilter 외 분기 cover 결손) | longterm/agc/shortterm 의 분기 중 sample 5..7 부호를 결정하는 분기 식별 → M1' 유력 + 분기 ID 명시 / 식별 부재 → M1' 반증 |
| M3 (synth IIR memory propagation) | sample 5..7 동안 memory 부호 변화 → M3 유력 / 변화 없음 (zero memory propagate) → M3 반증 (F-oct-prelim-5-3 결정 유지) |

- [ ] **Step 6: 15 회귀 게이트 재확인**

Run: 14 PASS + 항목 15 RED + 신규 측정 2건 PASS.

- [ ] **Step 7: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-4-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix2-prelim-4 보고서 — M1' + M3 정밀 측정

**작성일**: 2026-05-02
**범위**: M1' (postfilter 외 분기) + M3 (synth IIR 재진입) 측정.
**산출물**: postfilter / synth white-box 측정 2 파일 + stage 별 부호 trace.
**준수**: F-oct-prelim-5-3 §3.3 M3 폐기 결정의 sample 5..7 한정 재평가.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§A.4.2.1/2/4 + §A.4.1)
## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인
## 2. M1' 측정 raw 출력 (postfilter stage 별 sample 5..7)
## 3. M3 측정 raw 출력 (IIR memory pre/post sample 5..7)
## 4. M1' 가설 평가 + cover 결손 분기 ID
## 5. M3 가설 평가 + F-oct-prelim-5-3 §3.3 결정 재평가
## 6. Task 5 진입 의무 (4 가설 비교 + 다음 cycle 결정)
```

```bash
git add internal/postfilter/stagef_octpostfix2_prelim_diagnostic_test.go \
        internal/synth/stagef_octpostfix2_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-4-report.md
git commit -m "$(cat <<'EOF'
test(postfilter,synth): add Stage F-oct-postfix2-prelim-4 M1' + M3 measurement

M1' 가설 (postfilter 의 γ_t 외 분기 cover 결손) + M3 가설 재진입
(synth IIR memory propagation, F-oct-prelim-5-3 §3.3 폐기 결정의
sample 5..7 한정 재평가) 의 white-box 측정. postfilter stage 별
출력 부호 + IIR memory pre/post sample 5..7 dump.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix2-prelim-5: 4 가설 비교 + 다음 cycle 결정

**Goal:** Task 1~4 측정 결과 결합 분석 — 4 가설 (M1' / M3 / M5 / M6) 비교표 (단일 표 + Phase 0.4 §1 강압-적합 회피 의무 준수) + 다음 cycle 단일 결정 (production fix cycle 진입 또는 추가 진단 cycle). 단일 가설 식별 시 fix scope outline 작성. 2+ 가설 잔존 시 E3 발동 (추가 진단 cycle). 0 가설 식별 (모두 반증) 시 spec 영역 확장 진단 cycle 권고.

**Files:**
- Create: `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-5-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task — 종합 보고만).

- [ ] **Step 1: cycle commit 요약**

Run: `git log --oneline -6`

Expected:
```
<5 hash> docs(plans): F-oct-postfix2-prelim synthesis ...
<4 hash> test(postfilter,synth): F-oct-postfix2-prelim-4 ...
<3 hash> test(decoder): F-oct-postfix2-prelim-3 ...
<2 hash> test(decoder): F-oct-postfix2-prelim-2 ...
<1 hash> test(decoder): F-oct-postfix2-prelim-1 ...
8907847 docs(plans): F-oct-postfix synthesis + cycle decision (E3)
```

- [ ] **Step 2: 4 가설 비교표 (단일 표)**

Task 2~4 의 측정 결과를 단일 표로 결합 (Phase 0.4 §1 — 측정 데이터만):

| 가설 | 측정 출처 | 결과 | 평가 (반증/유력/부분) | spec § 인용 |
|------|-----------|------|----------------------|--------------|
| M1' (postfilter 외 분기) | Task 4 §2 + §4 | (Task 4 결과) | (Task 4 결과) | §A.4.2.1/2/4 |
| M3 (synth IIR memory 재진입) | Task 4 §3 + §5 | (Task 4 결과) | (Task 4 결과) | §A.4.1 |
| M5 (excitation pre-postfilter 부호) | Task 2 §2 + §3 | (Task 2 결과) | (Task 2 결과) | §A.3.5, §A.4.1 |
| M6 (PST want 데이터 부호) | Task 3 §2 + §3 + §4 | (Task 3 결과) | (Task 3 결과) | READMETV.txt |

- [ ] **Step 3: 단일 식별 결정 트리**

| 시나리오 | 결정 |
|----------|------|
| 4 가설 중 1건 "유력" + 나머지 "반증" | **단일 식별** — 다음 cycle = production fix cycle (식별 가설의 fix scope outline 작성) |
| 2+ 가설 "유력" 또는 "부분" | **E3 발동** — 다음 cycle = 추가 진단 cycle (각 잔존 가설별 분리 측정 plan) |
| 4 가설 모두 "반증" | spec 영역 확장 — 다음 cycle = §A.3.* / §A.4.1 외 영역 (예: §A.5 *Bit allocation* 또는 §A.6 *Concealment*) 진단 cycle |

본 §3 결정은 *측정 데이터에 의해 자동 결정* — 임의 선택 금지 (Phase 0.4 §1).

- [ ] **Step 4: 다음 cycle 권고 outline**

식별된 가설에 따라:

| 식별 가설 | 다음 cycle 명 | scope outline |
|-----------|----------------|----------------|
| M1' (특정 분기 ID) | F-oct-postfix3 (production fix) | 식별 분기의 spec 정합 fix (1~3 라인) + RED→GREEN gate = 항목 15 |
| M3 (IIR memory) | F-oct-postfix3-synth (production fix) | synth IIR memory init / propagation fix |
| M5 (excitation 부호) | F-oct-postfix3-excit (production fix) | `internal/synth/excitation.go` 또는 `internal/fcb`/`internal/pitch` fix |
| M6 (PST 읽기) | F-oct-postfix3-pst (test helper fix) | `readPSTFrames` helper 의 endianness / format 해석 fix; production 변경 0 가능 |
| 모두 반증 | F-non (가칭, 추가 진단) | spec §A.3.* / §A.5 / §A.6 영역 cover 점검 |
| 2+ 잔존 | F-non-prelim (가칭, 분리 측정) | 각 잔존 가설별 단일 frame 정밀 측정 cycle |

- [ ] **Step 5: 잔여 보류 항목 갱신**

F-oct-postfix synthesis (`8907847`) §5 사용자 게이트 G1-G6 의 본 cycle 결과 반영 + 신규 보류 항목 추가:

| # | 항목 | 본 cycle 갱신 |
|---|------|---------------|
| F-oct-postfix2-prelim cycle 자체 | 본 cycle 종결 |
| 4 가설 단일 식별 | (Step 3 결과) |
| `stagef_bis_diagnostic_test.go` untracked | 보존 유지 (변경 0) |
| F-oct-prelim-5-4 §3.6 M1 단독 결정 정정 | 본 plan §0.4 §6 으로 갈음 (별도 commit 불요) |
| 다음 fix cycle 의 RED gate | 항목 15 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 승계 |

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-5-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix2-prelim 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-02
**범위**: F-oct-postfix2-prelim-1/2/3/4 결합 분석 + 4 가설 비교 + 다음 cycle 단일 결정.
**산출물**: cycle 결산 + 4 가설 비교표 + 결정 트리 + 다음 cycle plan outline.
**준수**: Phase 0.4 강압-적합 회피 (측정 데이터만), 사용자 G1 결정 (Annex A binary 거부).
**production 변경**: 0 라인 (메타 task).

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)
## 1. F-oct-postfix2-prelim cycle commit 요약
## 2. 4 가설 비교표 (단일 표, 측정 데이터만)
## 3. 단일 식별 결정 (또는 E3 발동 / 0 식별 처리)
## 4. 다음 cycle 권고 (production fix / 추가 진단 / spec 영역 확장)
## 5. 잔여 보류 항목 갱신
## 6. 결론
```

```bash
git add docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-5-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-oct-postfix2-prelim synthesis + next cycle decision

F-oct-postfix2-prelim cycle (Task 1 chain dump baseline, Task 2 M5
excitation sign trace, Task 3 M6 PST want 부호 검증, Task 4 M1' + M3
postfilter/synth white-box 측정) 의 결합 분석 + 4 가설 비교표 + 다음
cycle 단일 결정. F-oct-prelim-5-4 §3.6 의 M1 단독 채택 결정이 G3
반증으로 갱신된 후 측정 데이터만으로 단일 식별 또는 E3 발동.

production 변경 0 (메타 task). 외부 G.729 0 참조 (G1 결정 정합).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

### 1. Spec coverage

F-oct-postfix synthesis (`8907847`) §3 후보 ③ + §5 사용자 게이트 G1 (c) 결정 + 본 plan task 매핑:

- 후보 ③ "M1 외 후보 재진입" → Task 1 baseline + Task 2 (M5) + Task 3 (M6) + Task 4 (M1' + M3) 4 가설 분리 측정.
- G1 (c) "Annex A binary 거부" → Phase 0.2 E4 강화 + Phase 0.4 §5 Annex A binary 측정 도구 사용 금지.
- G2 (a) "postfix-1 RED test 다음 cycle 의 GREEN gate 승계" → Phase 0.3 항목 15 RED 잔존 의무 + Task 5 §5 잔여 보류 항목.
- G3 (a) "stagef_bis_diagnostic_test.go 보존 유지" → Phase 0.5 의무.
- G4 (c) "F-oct-prelim-5-4 §6 (a) 정정 본 보고서 §2.3 으로 갈음" → 본 plan Phase 0.4 §6 으로 갈음 (별도 commit 불요).
- G5 (a) "Phase 1k 종결 보류" → 본 cycle 은 Phase 1k 내 진단 cycle 잔존.
- G6 (b) "G1-G5 결정 후 dispatch" → 본 plan 작성 = G1-G5 결정 반영 후.

7 항목 모두 매핑 완료. 누락 0.

### 2. Placeholder scan

본 plan 검토 — "TBD" / "TODO" / "implement later" / "fill in details" 검색:

- 각 task 의 Step N 보고서 outline 에 *각 § 명시* (placeholder 없이).
- Task 1 의 Step 3 에 *완전한 test 코드* 제시 (signature, helper 인용, 본문 t.Logf).
- Task 2/3/4 의 측정 함수는 *완전한 측정 출력 형식* + spec § 인용 + 가설 평가 표 제시. 구체 코드 윤곽 + 측정 결과는 task 실행 시점 결정 (dump 값은 측정에 의해 도출되므로 placeholder 가 아님 — 보고서에 "(Task N 결과)" 로 명시).
- 각 commit 메시지 *완전한 한국어 본문* + co-author trailer.

placeholder 0 확인.

### 3. Type consistency

- Task 1 신규 test `TestDiagnostic_FoctPostfix2PrelimChainDump`: helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `readPSTFrames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0.
- Task 2/3 측정 함수 동일 helper 재사용 — Task 3 의 `os.ReadFile` + `binary.LittleEndian.Uint16` 만 표준 lib 추가.
- Task 4 의 postfilter / synth white-box test: 각 package 내 기존 test helper 재사용 + production unexported 함수 직접 호출 가능 (같은 package). 신규 helper 0.
- 회귀 게이트 15 항목의 test 이름 Phase 0.3 ↔ 각 task Step 에서 일관.
- production 변경 0 라인 의무 (E5 + Phase 0.4 §4) — 본 cycle 5 task 모두 test/docs 변경만.

type consistency clean.

### 4. Spec § 인용 정합성 (특별 검토)

본 plan 의 spec 인용은 PDF `pdftotext -layout` verbatim. 각 task 진입 시 Step 2 등에서 grep 재확인 의무 명시. F-oct-prelim-5-4 §3.6 의 "g_l > 0" 결합 해석은 본 cycle 의 spec 인용으로 사용하지 *않는다* (synthesis §2.1 + Phase 0.4 §2). M5 (§A.3.5) / M3 (§A.4.1) / M1' (§A.4.2.1/2/4) / M6 (READMETV.txt) 의 인용 출처가 가설별 분리 — 결합 해석 위험 0.

### 5. 사용자 G1 결정 정합성 특별 검토

G1 (c) = "Annex A binary 거부 + 후보 ③ pivot". 본 plan:

- Phase 0.2 E4: 외부 G.729 구현 (Annex A binary 포함) 0건 인용.
- Phase 0.4 §5: g_l 영속화 (후보 ①) 관련 측정 / fix 도입 금지.
- Task 1~4 모든 측정: PDF + READMETV.txt + repo committed PST 파일만 사용 (Annex A binary trace 의 ground-truth 대체 불가 — 측정 한계는 보고서 §0 명시).

G1 결정 정합 100%.

### 6. 회귀 게이트 15 항목 정합성

Phase 0.3 의 15 항목:
- F-quart 2 (항목 1, 2)
- F-sext 1 (항목 3)
- F-sept 3 (항목 4, 5, 6)
- F-oct-prelim 3 (항목 7, 8, 9)
- F-oct-prelim-5 4 (항목 10, 11, 12, 13)
- ITU contract 1 (항목 14 = `TestDecode_Frame0Sample0_MatchesALGTHM`)
- F-oct-postfix-1 RED 1 (항목 15)

합계 = 2+1+3+3+4+1+1 = **15**. 사용자 task 명세와 정합.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, F-oct-prelim-5 / F-oct-postfix 패턴 답습. 각 task 완료 후 main agent 가 다음 task 진입 권고를 사용자에게 게이트.

**2. Inline Execution** — batch execution. Task 1 chain dump → 사용자 게이트 → Task 2~4 가설별 측정 → 사용자 게이트 → Task 5 종합.

**Recommended user gate before Task F-oct-postfix2-prelim-1**: 사용자가 본 plan 의 Phase 0.4 §1 (4 가설 분리 측정 의무) + §5 (g_l 후보 ① 제외 의무) + §6 (F-oct-prelim-5-4 §3.6 정정 의무) + Phase 0.5 (bis test 보존) + Phase 0.3 회귀 게이트 15 항목 (특히 항목 15 RED 잔존 의무) 을 검토 후 진입 승인.

**Which approach?**
