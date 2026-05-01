# Phase 1k Stage F-oct-postfix Production Fix Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Phase 1k Stage F-oct-prelim-5-4 (`8f693b7`) §6 단일 결정 — `(a) postfilter production fix cycle` 를 실행. `internal/postfilter/tilt.go` 의 γ_t 선택 분기 (line 65~68) 가 ITU-T G.729 (06/2012) §A.4.2.3 spec 정의에서 이탈한 *Phase 1g 임시 proxy* (`pf.agcGainPrev == 0`) 를 사용 — frame 0 sf0 cold start 에서 `agcGainPrev == 0` 이 항상 inactive branch 로 강제 진입시켜 ALGTHM frame 0 sf0 sample 5..7 부호가 chain `[+1,+1,+1]` 로 출력되는 결함의 production fix. 본 cycle 은 **production 변경 1 함수 + 호출부 1 라인** scope 로 한정 — 다른 stage / package 변경 0 라인. ALGTHM PST want sample 5..7 = `[-1,-1,-1]` 정합 또는 |Δ| 감소 입증을 본 cycle 종결 게이트로 정의.

**Architecture:** 5-task fix cycle (TDD 패턴 — RED → GREEN → 회귀 → 확장 → 종합). Task F-oct-postfix-1 = failing test 작성 + RED 확인 (production 변경 0). Task F-oct-postfix-2 = `computeTiltMu` γ_t 선택 분기 production fix + spec docstring 갱신 (production 변경 ≤ 1 함수 + 호출부). Task F-oct-postfix-3 = 회귀 게이트 14 항목 + ITU contract test + go vet + go test ./... -race 누적 PASS 검증, 비-contract diagnostic 3건 상태 변화 측정. Task F-oct-postfix-4 = ALGTHM 외 frame 1+ + PITCH/FIXED frame 0 부호 정합 보조 검증 (production 변경 0). Task F-oct-postfix-5 = 종합 보고서 + Phase 1k 종결 평가. Task 1 의 RED → Task 2 의 GREEN 사이 production fix 가 결함을 spec 정합으로 해소하는지가 본 cycle 의 단일 입증 의무.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §A.4.2.3 (Annex A tilt compensation, p.43) + §4.2.3 (main spec tilt compensation, p.29) + 기존 F-quart/F-sext/F-sept/F-oct-prelim 진단 하니스 (회귀 게이트). 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) **0건 참조** (E4).

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-oct-postfix 진입 시점, post-`8f693b7`)

| 경로 | 상태 | F-oct-postfix 변경? |
|------|------|---------------------|
| `internal/lsp/lsp_lp.go` | committed via `02bf785` | **No** (다른 영역) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis baseline | **No** (보존 — 본 cycle 어떤 commit 도 add 금지) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed | **No** |
| `internal/decoder/stagef_sext_diagnostic_test.go` | committed `6f1c841` | **No** |
| `internal/decoder/stagef_sept_diagnostic_test.go` | committed `48265cd`/`d61497d`/`353398d` | **No** |
| `internal/decoder/foct_prelim_diagnostic_test.go` | committed `5832294`/`94ef154`/`51e74e2` | **No** |
| `internal/decoder/stagef_octprelim5_diagnostic_test.go` | committed `445c72d`/`9f27f74`/`9a749b0` | **No** |
| `internal/postfilter/tilt.go` | F-sept 시점 그대로 (Phase 1g proxy 잔존) | **YES** — Task F-oct-postfix-2 (γ_t 선택 분기 fix + docstring 갱신) |
| `internal/postfilter/postfilter.go` | F-sept 시점 그대로 | **MAYBE** — Task F-oct-postfix-2 호출부 1 라인 (computeTiltMu signature 변경 시) |
| 그 외 production 파일 | 미변경 | **No** |

F-oct-postfix 신규 파일:
- (Task F-oct-postfix-1) `internal/decoder/stagef_octpostfix_regression_test.go` — failing regression test 단일 파일.
- (Task F-oct-postfix-1) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-1-report.md`
- (Task F-oct-postfix-2) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md`
- (Task F-oct-postfix-3) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-3-report.md`
- (Task F-oct-postfix-4) (선택) `internal/decoder/stagef_octpostfix_extra_test.go` — frame 1+ 보조 검증 + `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-4-report.md`
- (Task F-oct-postfix-5) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-5-report.md`

본 cycle 의 production 변경 범위 = **`internal/postfilter/tilt.go` 단일 함수 (`computeTiltMu`) + `internal/postfilter/postfilter.go` 호출 1 라인** (Task F-oct-postfix-2 한정). 다른 production 파일 1 라인이라도 변경 시 즉시 E5 발동.

`internal/decoder/stagef_bis_diagnostic_test.go` untracked 보존: 본 cycle 5 task 어떤 commit 도 본 파일을 add 하지 않는다. 사후 working tree 의 `?? internal/decoder/stagef_bis_diagnostic_test.go` 가 F-oct-prelim cycle 시점과 동일하게 유지됨을 각 task §0 보고서에서 확인.

### Phase 0.2 회귀 게이트 명세

각 task commit 직후 *반드시* 실행 (총 14 게이트 — F-oct-prelim-5 시점의 14 그대로 + 본 cycle 의 신규 regression 1):

1. **Stage D 17 contract test** (`internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/`). 본 cycle 회귀 0 의무.
2. **Stage D-bis 3 contract test**. 회귀 0.
3. **Phase 1i sample 0 가드** (`TestDecode_Frame0Sample0_MatchesALGTHM`): `1c00385` 후 PASS. 본 cycle 모든 commit 직후 PASS 의무.
4. **F-quart-3 reference cross-check** (`TestDiagnostic_FquartGainReferenceCrossCheck`).
5. **F-quart-1 alignment harness** (`TestDiagnostic_FquartGainImap_Sf0Sample0to7`).
6. **F-sext-1 chain trace** (`TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7`).
7. **F-sept-1 excitation 분해** (`TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5`).
8. **F-sept-2 LP cross-check** (`TestDiagnostic_FseptLPReferenceCrossCheck`).
9. **F-sept-3 synth IIR trace** (`TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7`).
10. **F-oct-prelim-1 PST format** (`TestDiagnostic_FoctPrelimPSTFormat`).
11. **F-oct-prelim-2 frame alignment** (`TestDiagnostic_FoctPrelimFrameAlignment`).
12. **F-oct-prelim-3 multi-vector** (`TestDiagnostic_FoctPrelimMultiVectorScan`).
13. **F-oct-prelim-5 진단 4건** (`TestDiagnostic_FoctPrelim5PSTSourceVerbatim`, `TestDiagnostic_FoctPrelim5BitVectorCompare`, `TestDiagnostic_FoctPrelim5HpFilterInitState`, `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism`).
14. **본 cycle 신규** (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`): Task 1 commit 시점 RED, Task 2 commit 후 GREEN. 이후 commit 모두 GREEN 유지 의무.

비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 **본 cycle 진입 시점 FAIL 유지** (F-quint-3 §4.6 plan-허용). Task F-oct-postfix-3 §3 에서 fix 가 본 3건의 PASS/FAIL 상태를 변화시키는지 측정 — 변화 발생 시 보고서에 정량 기록만, *contract 의무 신설 금지* (자동 promotion 금지 동상).

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.2 의 1~14 중 임의, Task 1 commit 의 항목 14 RED 는 *예외 — 의도된 RED*) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | Task F-oct-postfix-1 spec § 인용이 PDF §A.4.2.3 verbatim grep 결과와 불일치 (= 휴리스틱 fit) | 즉시 측정 폐기 + spec § 식 PDF 직접 재발췌 + Task 1 §0 에 도출 과정 정량 기록 |
| **E3** | Task F-oct-postfix-2 의 fix 후에도 항목 14 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 가 RED 잔존 — 즉 γ_t 선택 fix 가 sample 5..7 부호를 spec want = `-1` 로 전환시키지 못함 | 즉시 `git revert HEAD` + 보고서에 fix 후 sample 5..7 raw 출력 정량 기록 + Task 5 의 다음 cycle 권고 갱신 (Phase 1l 또는 Annex A binary 행동 추적 cycle) |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | 본 cycle 의 production 변경이 Phase 0.1 의 scope (`internal/postfilter/tilt.go` + `internal/postfilter/postfilter.go` 호출 1 라인) 를 1 라인이라도 초과 | 즉시 `git revert HEAD` + commit 재구성 (scope 내로 축소) |

각 보고서 (F-oct-postfix-1/2/3/4/5) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 회피 의무 (forced-fit avoidance)

본 cycle 은 *production fix* 이며 강압-적합 위험이 진단 cycle 보다 높다. 다음 패턴을 적극 회피:

1. **spec § 인용 우회 fit 금지**: F-oct-prelim-5-4 §3.6 표의 spec 인용 ("γ_t = 0.9 if long-term postfilter active (g_l > 0)") 은 **PDF 원문 검증 필수** (Task 1 Step 1). PDF §A.4.2.3 verbatim 원문은 본 plan §"Spec § 인용 (Task F-oct-postfix-1)" 에 인용되어 있다 — 그 인용이 ground-truth.
2. **constant 값 강압-적합 금지**: 현 production constant `gammaTiltActiveQ14 = 14746` (≈0.9), `gammaTiltInactiveQ14 = 3277` (≈0.2) 는 main spec §4.2.3 정의에 일치하나 **Annex A spec §A.4.2.3 정의 (0.8 / 0)** 와는 다름. 본 cycle 은 **분기 조건 fix** 만 scope 로 한정 — constant 값 변경은 별도 cycle 영역. 단, Task 5 §3 에 잔여 보류 항목으로 명시 의무.
3. **fix 후 PASS 강압 회피**: Task 2 fix 후 항목 14 가 GREEN 으로 전환되면, "다른 회귀 게이트가 PASS 라는 사실만으로" fix 의 spec 정합성을 결론짓지 않는다. Task 3 §4 에 fix 의 spec § 인용 ↔ 변경된 코드 라인 ↔ ALGTHM frame 0 sample 5..7 부호 변화의 *3-way mapping* 명시 의무.
4. **scope crawl 금지**: Task 2 의 production 변경이 `computeTiltMu` 외 함수로 확장되거나, F-oct-postfix-3/4/5 가 추가 production fix 를 임의 도입하면 즉시 E5 발동.

---

## Spec § 인용 (Task F-oct-postfix-1 의 spec ground-truth)

**(인용 1)** ITU-T G.729 (06/2012) PDF page 43, §A.4.2.3 *Tilt compensation*, verbatim (PDF text extraction `pdftotext -layout`):

```
A.4.2.3       Tilt compensation
The filter Ht(z) compensates for the tilt in the short-term postfilter Hf(z) and is given by:
                                               H t (z ) = 1 + γ t k1′ z −1                                                (A.13)

where γ t k1′ is a tilt factor, k1′ being the first reflection coefficient calculated by:

                                           r (1)          21−i
                                    k1′ = − h ; rh (i ) =  h f ( j )h f ( j + i )                                        (A.14)
                                           rh (0)          j =0


where hf(n) is the truncated impulse response of the filter Aˆ (z / γ n ) / Aˆ (z / γ d ) . The value of γt = 0.8 is
used if k1′ < 0 and γt is set to zero if k1′ ≥ 0 . The gain factor gt which is used in clause 4.2.3 is
eliminated.
```

**(인용 2)** ITU-T G.729 (06/2012) PDF page 29, §4.2.3 *Tilt compensation* (main spec, 비교용), verbatim:

```
4.2.3   Tilt compensation
... [중략] ...
The gain term gt = 1 – | γ t k1′ | compensates for the decreasing effect of gf in Hf(z). Furthermore, it has
been shown that the product filter Hf (z)Ht(z) has generally no gain. Two values for γt are used
depending on the sign of k1′ . If k1′ is negative, γt = 0.9, and if k1′ is positive, γt = 0.2.
```

**(인용 3)** 현 production `internal/postfilter/tilt.go:23-25`, verbatim:

```go
// γ_t selection follows Annex A's voicing-dependent rule; for Phase 1g
// we consult pf.agcGainPrev as a proxy for "long-term active" (non-zero)
// vs "inactive" (zero).
```

및 `internal/postfilter/tilt.go:65-68`, verbatim:

```go
gammaTQ14 := gammaTiltActiveQ14
if pf.agcGainPrev == 0 {
    gammaTQ14 = gammaTiltInactiveQ14
}
```

### 핵심 spec 해석 — strict reading vs F-oct-prelim-5-4 §3.6 해석

**Strict PDF reading (인용 1, 2)**: 두 spec 모두 γ_t 선택 분기 = **k1' 의 부호** (`k1' < 0` ↔ active). `g_l` (long-term postfilter gain) 또는 voicing 상태와의 명시적 연결은 §A.4.2.3 본문에 *부재*.

**F-oct-prelim-5-4 §3.6 해석**: spec 을 "γ_t = 0.9 if long-term postfilter active (g_l > 0), else 0.2 (§A.4.2.3)" 로 인용 — 이는 PDF 원문에 *직접 등장하지 않는다*. F-oct-prelim-5-4 보고서가 §A.4.2.2 (long-term postfilter 의 `g_l = clamp(R(T)/E(T), 0, γ_l)` 결과로 active/inactive 를 결정) 와 §A.4.2.3 (γ_t 분기) 를 *해석적으로 결합* 한 것으로 추정.

**본 cycle 의 ground-truth 결정**: Task F-oct-postfix-1 Step 1 에서 PDF §A.4.2.3 raw 인용 + F-oct-prelim-5-4 §3.6 해석 비교를 명시. *strict reading (k1' 부호) 채택* 을 default 로 하되, F-oct-prelim-5-4 §3.6 해석이 spec 외 출처 (ITU 참조 C / bcg729 등 — 그러나 E4 invariant 로 인용 금지) 에서 도출되었다면 그 사실을 명시하고 strict reading 으로 fix 진행. 단, Phase 0.4 §1 "spec § 인용 우회 fit 금지" 에 따라 **F-oct-prelim-5-4 §3.6 해석은 본 cycle 에서 spec 인용으로 사용하지 않는다**.

→ **fix 방향 결정**: γ_t 선택 분기 조건을 `pf.agcGainPrev == 0` (Phase 1g proxy) 에서 **`k1 >= 0`** (k1' 의 부호) 로 변경. 이 경우 **signature 변경 불필요** — `k1` 는 `computeTiltMu` 의 local 변수 (line 60).

→ **scope 최소화 결과**: production 변경 = `internal/postfilter/tilt.go` 의 1 함수 내 분기 조건 1 라인 + docstring 갱신 (line 23-25). 호출부 (`postfilter.go:44`) 변경 **0 라인**. F-oct-prelim-5-4 §3.6 의 "computeTiltMu signature 확장" 추정과는 달리 *최소 침투* fix 가 가능.

→ **g_l 전달 가능성 평가** (보조): 만약 strict reading 이 후속 cycle 에서 반증되어 g_l-based 분기로 pivot 이 필요하면, scope 확장 옵션 = `computeLongTermGain` 의 g_l (또는 g1 Q14) 을 `Postfilter` state field 로 영속화 + `computeTiltMu` 가 read. signature 변경 없이 state 1 field 추가 + write 1 라인 (`longterm.go:48-90` 내) + read 1 라인 (`tilt.go:65-68`). 본 옵션은 *Task 5 의 잔여 보류 항목* 으로만 기록.

---

## Task F-oct-postfix-1: failing regression test 작성 — RED 확인

**Goal:** ALGTHM frame 0 sf0 sample 5..7 의 PST want = `[-1, -1, -1]` 와 production 출력 `[+1, +1, +1]` (또는 양수) 의 mismatch 를 assertion 으로 표현하는 regression test 1 개 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 를 작성. 본 task commit 시점 항목 14 회귀 게이트 = **RED 의무** (Task 2 fix 후 GREEN 전환 입증의 baseline). 그 외 14 - 1 = 13 회귀 게이트는 PASS 유지 의무.

**Files:**
- Create: `internal/decoder/stagef_octpostfix_regression_test.go`
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-1-report.md`

### Spec § 인용

본 plan 상단 "Spec § 인용 (Task F-oct-postfix-1 의 spec ground-truth)" §전체 — 인용 1/2/3 + strict reading 결정.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
8f693b7 docs(plans): add Stage F-oct-prelim-5 synthesis report + F-oct decision
```

`internal/lsp/lsp_lp.go` modified 잔존 시 즉시 사용자 통보 (별도 cycle 영역).

Run (회귀 게이트 baseline, 13 항목):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|oct_prelim|OctPrelim5)" -v
go test ./internal/postfilter/ -v -run Contract
go test ./internal/synth/ -v -run Contract
go vet ./...
```

Expected: 13 항목 모두 PASS + `go vet` clean. 출력 요약을 보고서 §2 에 인용.

- [ ] **Step 2: PDF §A.4.2.3 raw 인용 grep 재확인 (strict reading 의 ground-truth)**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B2 -A 22 "A.4.2.3       Tilt compensation"
```

Expected output: 본 plan 상단 "인용 1" 과 byte-level 일치. 일치하지 않을 경우 즉시 E2 발동 + 본 plan 의 인용 갱신.

PDF page (Rec. ITU-T G.729 (06/2012) p.43) 명시. F-oct-prelim-5-4 §3.6 표의 spec 인용 ("g_l > 0") 과 PDF 원문 (sign of k1') 의 *불일치* 를 보고서 §3 에 정량 기록.

- [ ] **Step 3: failing test 작성 — `stagef_octpostfix_regression_test.go` 신규**

`internal/decoder/stagef_octpostfix_regression_test.go` 신규 작성:

```go
package decoder

import "testing"

// TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput is the
// Phase 1k Stage F-oct-postfix RED→GREEN regression. ALGTHM
// frame 0 sf0 sample 5..7 must equal the ITU reference's PST
// want sample 5..7 (= [-1, -1, -1] per F-oct-prelim-5-4 §3.2
// raw measurement). Pre-fix production output is [+1, +1, +1]
// (positive); post-fix (Task F-oct-postfix-2) must be
// [-1, -1, -1] (or signs match).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.4.2.3 (PDF p.43)
// — γ_t = 0.8 if k1' < 0 else 0 (Annex A); main §4.2.3 (PDF p.29)
// — γ_t = 0.9 if k1' < 0 else 0.2. Production constants currently
// match main §4.2.3 (0.9 / 0.2). The defect being repaired is the
// *branch condition*, not the constants.
//
// Pre-Task-2 commit: this test must be RED (intentional baseline).
// Post-Task-2 commit: this test must be GREEN (fix verification).
func TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput(t *testing.T) {
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

	for n := 5; n <= 7; n++ {
		got, want := out[n], wantFrames[0][n]
		if got != want {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				n, got, want, int32(got)-int32(want))
		}
	}
}
```

Helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `readPSTFrames`, `frameSamples`) 는 기존 `decoder` package test 에서 이미 정의됨 — 신규 helper 0.

- [ ] **Step 4: test 컴파일 + RED 측정**

Run:
```
go build ./...
go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
```

Expected: build PASS, test FAIL (RED) — 출력에 `frame 0 sample 5: got=1 want=-1 (Δ=2)` (또는 동상) 3개 라인 등장. raw 출력을 보고서 §3 에 verbatim 인용.

만약 RED 가 아닌 GREEN 이라면 (= 현 production 이 이미 spec want 를 만족) 즉시 E1 발동 (premise 붕괴) + 보고서에 sample 5..7 raw 출력 + F-oct-prelim-5-4 §3.2 측정값과의 비교 + 본 cycle 의 premise 재평가 의무 명시.

- [ ] **Step 5: 13 회귀 게이트 PASS 재확인**

Run: Step 1 의 13 항목 + 본 신규 test = 14 항목 중 13 항목 PASS (신규 1건만 RED).

13 PASS 의무. 1+ FAIL 시 E1 발동 (test 추가만으로 다른 회귀 발생 = 의도되지 않은 부작용).

- [ ] **Step 6: 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-1-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix-1 보고서 — RED 확인

**작성일**: 2026-05-01
**범위**: failing regression test 작성 + RED 의무 입증.
**산출물**: `stagef_octpostfix_regression_test.go` 신규 1 파일 + RED 출력 verbatim 인용.
**준수**: F-oct-prelim-5-4 §6 결정 (a) production fix cycle 진입.
**production 변경**: 0 라인. **테스트 변경**: 1 신규 파일 (~40 라인).

## 0. Working tree 상태 + escape hatch 평가 (E1–E5)
## 1. 회귀 게이트 baseline (13 항목 PASS)
## 2. PDF §A.4.2.3 raw 인용 + strict reading 채택 근거
## 3. RED 출력 verbatim (sample 5..7)
## 4. F-oct-prelim-5-4 §3.6 spec 해석과 PDF 원문의 불일치 정량 기록
## 5. Task F-oct-postfix-2 진입 의무 항목 (γ_t 선택 분기 fix scope)
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_octpostfix_regression_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-1-report.md
```

```bash
git add internal/decoder/stagef_octpostfix_regression_test.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-postfix-1 ALGTHM sf0 sample 5..7 regression (RED)

F-oct-prelim-5-4 §6 결정 (a) production fix cycle 의 RED→GREEN
baseline. ALGTHM frame 0 sf0 sample 5..7 가 PST want = [-1,-1,-1]
와 일치해야 함을 assertion 으로 표현. 현 production 출력 [+1,+1,+1]
(F-oct-prelim-5-4 §3.2 측정 동상) → 본 commit 시점 RED 의무.

Task F-oct-postfix-2 의 internal/postfilter/tilt.go γ_t 선택 분기
fix 후 GREEN 전환 입증의 baseline.

production 변경 0 라인. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix-2: γ_t 선택 분기 production fix — GREEN 전환

**Goal:** `internal/postfilter/tilt.go:65-68` 의 γ_t 선택 분기 조건을 spec §A.4.2.3 strict reading 정합으로 수정. fix scope = (a) 분기 조건 1 라인: `pf.agcGainPrev == 0` → `k1 >= 0`, (b) docstring (line 23-25) 의 "Phase 1g proxy" 자기 인정 문구 제거 + spec § 정합 docstring 으로 교체. signature 변경 0 — `computeTiltMu` 호출부 (`postfilter.go:44`) 변경 0 라인.

**Files:**
- Modify: `internal/postfilter/tilt.go` (≤ 단일 함수 + 상수/문서 영역)
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md`

### Spec § 인용

본 plan 상단 "Spec § 인용" §전체. 특히 인용 1 의 "γt = 0.8 is used if k1′ < 0 and γt is set to zero if k1′ ≥ 0" 가 분기 조건의 ground-truth.

### scope 세부

**현 코드 (`tilt.go:23-25, 65-68`)**:
```go
// γ_t selection follows Annex A's voicing-dependent rule; for Phase 1g
// we consult pf.agcGainPrev as a proxy for "long-term active" (non-zero)
// vs "inactive" (zero).
...
gammaTQ14 := gammaTiltActiveQ14
if pf.agcGainPrev == 0 {
    gammaTQ14 = gammaTiltInactiveQ14
}
```

**fix 후 코드 (제안)**:
```go
// γ_t selection follows ITU-T G.729 (06/2012) §A.4.2.3 (Annex A
// p.43) and §4.2.3 (main p.29): γ_t depends on the sign of k1'.
// If k1' < 0, the postfilter is "active" (γ_t = 0.9 in our Q14
// constants, matching main §4.2.3); if k1' ≥ 0, γ_t = 0.2
// (inactive, matching main §4.2.3). The Annex A spec §A.4.2.3
// uses 0.8 / 0 for the active/inactive constants; the difference
// vs. our 0.9 / 0.2 is tracked as a follow-up cycle (see Stage
// F-oct-postfix-5 §3 잔여 보류 항목).
...
gammaTQ14 := gammaTiltActiveQ14
if k1 >= 0 {
    gammaTQ14 = gammaTiltInactiveQ14
}
```

**변경 라인 수**: docstring 3 라인 (교체 ~6 라인) + 분기 조건 1 라인 (`pf.agcGainPrev == 0` → `k1 >= 0`). 다른 함수 0 라인. signature 변경 0. import 변경 0.

- [ ] **Step 1: 사전 조건 확인**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
<F-oct-postfix-1 commit hash> test(decoder): add Stage F-oct-postfix-1 ALGTHM sf0 sample 5..7 regression (RED)
```

Run (Task F-oct-postfix-1 의 RED 재확인): `go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v`

Expected: FAIL (RED 잔존 — fix 미적용 시점).

- [ ] **Step 2: production fix 적용**

`internal/postfilter/tilt.go` 의 docstring (line 23-25) + 분기 조건 (line 65-68) 을 위 "fix 후 코드 (제안)" 으로 교체. 다른 라인 변경 0.

`internal/postfilter/postfilter.go:44` (`muQ15 := pf.computeTiltMu(&aNum, &aDen)`) 변경 0 라인 — signature 동일.

`internal/postfilter/types.go` 의 `agcGainPrev` field 변경 0 라인 (다른 stage — AGC §A.4.2.4 — 에서 사용 잔존).

- [ ] **Step 3: GREEN 입증**

Run: `go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v`

Expected: PASS (GREEN). 출력 verbatim 을 보고서 §3 에 인용.

만약 FAIL 잔존 (sample 5..7 부호가 여전히 spec want 와 불일치) 시 즉시 E3 발동:
- Step 4 진입 금지.
- `git checkout -- internal/postfilter/tilt.go` 로 fix 되돌리기.
- 보고서 §3 에 fix 적용 후 sample 5..7 raw 출력 정량 기록 (예: `got=2 want=-1`).
- Task F-oct-postfix-5 의 다음 cycle 권고를 "γ_t 분기 fix 가 결함 해소 미입증 → 추가 진단 cycle (예: pitch synthesis IIR memory 또는 Annex A binary 행동 추적)" 으로 갱신.
- E3 발동 시 Task 3/4 skip + Task 5 만 실행 (cycle 미해소 종합 보고).

- [ ] **Step 4: 14 회귀 게이트 PASS 재확인**

Run (Phase 0.2 의 14 항목 전체):
```
go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5)" -v
go test ./internal/postfilter/ -v
go test ./internal/synth/ -v
go vet ./...
```

Expected: 14 항목 PASS + `go vet` clean.

Phase 1i sample 0 가드 (`TestDecode_Frame0Sample0_MatchesALGTHM`) FAIL 시 즉시 E1 발동 — sample 0 회귀는 Phase 1k spec §7.2 의 절대 invariant.

- [ ] **Step 5: 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix-2 보고서 — γ_t 분기 production fix (GREEN)

**작성일**: 2026-05-01
**범위**: internal/postfilter/tilt.go γ_t 선택 분기 fix.
**산출물**: tilt.go 단일 함수 변경 + 회귀 게이트 14 항목 PASS.
**준수**: spec §A.4.2.3 strict reading (k1' 부호) + Phase 0.4 강압-적합 회피.
**production 변경**: ≤ 7 라인 (docstring 6 + 분기 조건 1).

## 0. Working tree 상태 + escape hatch 평가 (E1–E5)
## 1. 변경 diff verbatim
## 2. spec § 인용 ↔ 변경 라인 ↔ ALGTHM sample 5..7 부호 변화 3-way mapping
## 3. GREEN raw 출력 인용
## 4. 14 회귀 게이트 PASS 결과 요약
## 5. (Annex A 0.8/0 vs main 0.9/0.2 constant 차이) 잔여 보류 항목 명시
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
 M internal/postfilter/tilt.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md
```

```bash
git add internal/postfilter/tilt.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md
git commit -m "$(cat <<'EOF'
fix(postfilter): select γ_t by sign of k1' per §A.4.2.3 (drop Phase 1g proxy)

ITU-T G.729 (06/2012) §A.4.2.3 (Annex A p.43): "γt = 0.8 is used if
k1' < 0 and γt is set to zero if k1' ≥ 0". 본 디코더는 main §4.2.3
(p.29) 의 0.9 / 0.2 constant 를 답습 — 분기 *조건* 만 spec 정합으로
교체 (constant 차이는 잔여 보류 항목).

기존 분기 조건 `pf.agcGainPrev == 0` 는 Phase 1g 임시 proxy 로
docstring 자체 인정 (`tilt.go:23-25`). frame 0 sf0 cold start 시
agcGainPrev = 0 zero-value → 항상 inactive branch (γ_t = 0.2) 진입
→ tilt μ 약화 → ALGTHM sample 5..7 부호 [+1,+1,+1] (PST want
[-1,-1,-1] 와 반전). 본 fix 로 sample 5..7 spec 정합 입증
(TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput GREEN).

scope: tilt.go 단일 함수 + docstring. signature/호출부 변경 0.
회귀 게이트 14 항목 PASS. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix-3: 회귀 게이트 누적 검증 + 비-contract 3건 상태 측정

**Goal:** Task 2 의 fix 가 본 plan §0.2 의 14 회귀 게이트 + ITU contract test 전체 + go vet + `go test ./... -race` 모두 PASS 를 입증. 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 의 상태 변화 정량 측정 — γ_t fix 가 PASS 로 전환시키는지 확인 (단, *contract 의무 신설 금지*; 측정-only).

**Files:**
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-3-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task — 측정 + 보고만).

- [ ] **Step 1: 누적 회귀 게이트 실행 — 14 + race + vet**

Run:
```
go test ./... -race -timeout 5m
go vet ./...
```

Expected: `go test ./... -race` 결과에서 14 회귀 게이트 항목 모두 PASS. 비-contract diagnostic 3건의 상태는 측정만 — PASS/FAIL 정량 기록.

전체 출력을 보고서 §1 에 요약 (각 package: PASS/FAIL count + 본 cycle 회귀 0 의무 항목).

- [ ] **Step 2: 비-contract 3건 상태 변화 측정**

Run (각 개별):
```
go test ./internal/decoder/ -run TestDiagnostic_SinglePulseChain -v
go test ./internal/decoder/ -run TestDecode_LowEnergyCodebookIsSmooth -v
go test ./internal/decoder/ -run TestDecode_SucceedsAcrossAllGainIndices -v
```

직전 시점 (F-oct-prelim-5 종결) 의 PASS/FAIL 상태와 본 cycle Task 2 fix 후 상태를 표 형태로 비교. 변화 발생 시 (예: F-oct-prelim-5 시점 FAIL → fix 후 PASS):
- 보고서 §2 에 정량 raw 출력 인용.
- *contract 의무 신설 금지* (자동 promotion 금지) — Task 5 §3 잔여 보류 항목으로만 기록.

변화 없음 (모두 FAIL 잔존) 도 정상 — 본 fix 는 ALGTHM sf0 sample 5..7 spec 정합이 단일 의무.

- [ ] **Step 3: spec § ↔ 코드 ↔ 출력 3-way mapping**

본 task 보고서 §3 에:

| spec § 인용 | 변경 라인 | 출력 변화 (ALGTHM sf0 sample 5..7) |
|-------------|-----------|------------------------------------|
| §A.4.2.3 "γt = 0.8 if k1' < 0, else 0" | `tilt.go:67` `if k1 >= 0 {` | got=`[+1,+1,+1]` (pre-fix) → `[-1,-1,-1]` (post-fix) |
| §A.4.2.3 docstring 갱신 | `tilt.go:23-25` Phase 1g proxy 제거 | (정량 변화 없음 — 문서) |

- [ ] **Step 4: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-3-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix-3 보고서 — 회귀 게이트 누적 검증

**작성일**: 2026-05-01
**범위**: 14 회귀 게이트 + go test ./... -race + go vet 누적 PASS 입증.
        비-contract diagnostic 3건 상태 변화 측정 (PASS 전환 가능성).
**산출물**: 측정 표 + 3-way mapping + 잔여 보류 항목 갱신.
**준수**: production 변경 0, 외부 G.729 0 참조.

## 0. escape hatch 평가
## 1. 14 회귀 게이트 결과
## 2. 비-contract 3건 상태 변화
## 3. spec § ↔ 코드 ↔ 출력 3-way mapping
## 4. 잔여 보류 항목 (Task F-oct-postfix-4 / 5 진입 의무)
```

```bash
git add docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-3-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-oct-postfix-3 regression gate verification report

Task F-oct-postfix-2 의 γ_t 분기 fix 가 14 회귀 게이트 + ITU contract
test 전체 + go vet + go test ./... -race PASS 를 유지하는지 누적 검증.
비-contract diagnostic 3건 (SinglePulseChain / LowEnergyCodebook /
SucceedsAcrossAllGainIndices) 의 상태 변화 정량 측정 — γ_t fix 의
spec 정합 효과를 다른 stage 출력에서 cross-check.

production 변경 0. 측정-only. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix-4: frame 1+ / PITCH / FIXED 부호 정합 보조 검증

**Goal:** Task 2 fix 의 효과가 ALGTHM frame 0 sf0 sample 5..7 만의 *국지적 효과* 가 아니라 *spec 정합 generalization* 임을 보조 검증. ALGTHM frame 1..3 sf0/sf1 sample 0..7 부호 + PITCH/FIXED frame 0 sf0 sample 5..7 부호 측정. 측정 결과 부호 정합 다수 발견 시 본 cycle 의 spec 정합 입증 강화 — 부호 mismatch 잔존 시 잔여 결함 식별 + Task 5 의 다음 cycle 권고 입력.

**Files:**
- Create: `internal/decoder/stagef_octpostfix_extra_test.go` (보조 진단 test 1 파일 — 측정-only, assertion 0)
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-4-report.md`

production 변경 0 라인. test 변경 = 신규 1 파일.

### Spec § 인용

§A.4.2.3 (Task 1 spec ground-truth 동상). 본 task 는 spec 추가 인용 없이 *fix 일반성 검증* 만 수행.

- [ ] **Step 1: 사전 조건 확인 + Task 2/3 commit hash 인용**

Run: `git log --oneline -3`

Expected: Task 3 + Task 2 + Task 1 commit hash 순. Task 2 commit 이 fix(postfilter) prefix 이어야 함.

- [ ] **Step 2: 보조 진단 test 작성 — `stagef_octpostfix_extra_test.go`**

```go
package decoder

import "testing"

// TestDiagnostic_FoctPostfixExtraVectors: γ_t fix 의 일반성 검증.
// ALGTHM frame 1..3 + PITCH/FIXED frame 0 sample 0..7 부호 정합
// 측정. assertion 0 — 본 task 는 측정 + 정량 기록.
func TestDiagnostic_FoctPostfixExtraVectors(t *testing.T) {
	cases := []struct{ name, bit, pst string }{
		{"ALGTHM", "ALGTHM.BIT", "ALGTHM.PST"},
		{"PITCH", "PITCH.BIT", "PITCH.PST"},
		{"FIXED", "FIXED.BIT", "FIXED.PST"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			bitPath := vectorPath(c.bit)
			pstPath := vectorPath(c.pst)
			ensureTestdataPresent(t, bitPath, pstPath)
			frames, bads := readG192Frames(t, bitPath)
			wantFrames := readPSTFrames(t, pstPath)

			var d Decoder
			maxFrames := 4
			if len(frames) < maxFrames {
				maxFrames = len(frames)
			}
			for f := 0; f < maxFrames; f++ {
				var out [frameSamples]int16
				if err := d.Decode(frames[f], bads[f], out[:]); err != nil {
					t.Fatalf("Decode frame %d: %v", f, err)
				}
				match5to7 := 0
				for n := 5; n <= 7; n++ {
					if out[n] == wantFrames[f][n] {
						match5to7++
					}
				}
				t.Logf("[%s frame %d] sample 5..7 got=[%d %d %d] want=[%d %d %d] match=%d/3",
					c.name, f,
					out[5], out[6], out[7],
					wantFrames[f][5], wantFrames[f][6], wantFrames[f][7],
					match5to7)
			}
		})
	}
}
```

- [ ] **Step 3: 측정 + 정량 기록**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPostfixExtraVectors -v`

Expected: PASS (측정-only). raw 출력 12 라인 (3 vector × 4 frame) 을 보고서 §2 에 인용.

ALGTHM frame 0 sample 5..7 = `[-1, -1, -1]` 정합 입증 (Task 2 GREEN 동상). PITCH/FIXED frame 0 sample 5..7 도 spec 정합 (= `[-1, -1, -1]` PST want 와 일치) 시 fix 의 일반성 강화. mismatch 잔존 시 잔여 결함 식별 + Task 5 §3 잔여 보류 항목 추가.

- [ ] **Step 4: 14 회귀 게이트 + 신규 보조 PASS 재확인**

Run: Phase 0.2 의 14 항목 + 본 신규 보조 = 15 항목 PASS 의무.

- [ ] **Step 5: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-4-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix-4 보고서 — frame 1+ / PITCH / FIXED 보조 검증

**작성일**: 2026-05-01
**범위**: γ_t fix 의 generalization 측정.
**산출물**: 보조 test 1 파일 + 측정 표 + 잔여 결함 식별.
**준수**: production 변경 0, 외부 G.729 0 참조.

## 0. escape hatch 평가
## 1. 측정 raw 출력 (3 vector × 4 frame)
## 2. 부호 정합 표 (sample 5..7)
## 3. 잔여 결함 식별 (있을 시)
## 4. Task F-oct-postfix-5 진입 의무 항목
```

```bash
git add internal/decoder/stagef_octpostfix_extra_test.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-4-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-postfix-4 frame 1+ / PITCH / FIXED 보조 측정

Task F-oct-postfix-2 의 γ_t 분기 fix 가 ALGTHM frame 0 sf0 sample
5..7 외 frame 1..3 + PITCH/FIXED 의 sample 5..7 부호 정합도 강화하는지
측정. assertion 0 (측정-only). 잔여 결함 발견 시 Task F-oct-postfix-5
의 다음 cycle 권고 입력.

production 변경 0. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-postfix-5: 종합 보고서 + Phase 1k 종결 평가

**Goal:** Task 1~4 결과 결합 분석 — γ_t 분기 fix 의 spec 정합 입증 (Task 2 GREEN + Task 3 회귀 게이트 PASS + Task 4 generalization 측정) + Phase 1k 종결 가능 여부 단일 결정. 종결 가능 시 Phase 1k closure declaration. 종결 불가 시 다음 cycle (Phase 1l 또는 Phase 0c 잔여) 권고.

**Files:**
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-5-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task — 종합 보고만).

- [ ] **Step 1: cycle commit 요약**

Run: `git log --oneline -6` — 본 cycle 5 commit + 직전 `8f693b7` 까지.

Expected:
```
<5 hash> docs(plans): add Stage F-oct-postfix-5 ...
<4 hash> test(decoder): add Stage F-oct-postfix-4 ...
<3 hash> docs(plans): add Stage F-oct-postfix-3 ...
<2 hash> fix(postfilter): select γ_t by sign of k1' ...
<1 hash> test(decoder): add Stage F-oct-postfix-1 ...
8f693b7 docs(plans): add Stage F-oct-prelim-5 synthesis report ...
```

- [ ] **Step 2: 결합 분석 표 작성**

| Task | 의무 | 결과 | spec 정합 입증 기여 |
|------|------|------|----------------------|
| F-oct-postfix-1 | RED 의무 | (Task 1 결과 인용) | baseline 정량 |
| F-oct-postfix-2 | GREEN 의무 + 14 게이트 PASS | (Task 2 결과 인용) | spec §A.4.2.3 정합 단일 입증 |
| F-oct-postfix-3 | 회귀 게이트 누적 + 비-contract 측정 | (Task 3 결과 인용) | 회귀 0 입증 + 부수 효과 측정 |
| F-oct-postfix-4 | generalization 측정 | (Task 4 결과 인용) | 일반성 또는 잔여 결함 식별 |

- [ ] **Step 3: 잔여 보류 항목 갱신**

F-oct-prelim-5-4 §5 의 9 + 1 = 10 항목을 본 cycle 결과에 따라 갱신:

| # | 항목 | 직전 상태 | 본 cycle 갱신 |
|---|------|-----------|--------------|
| 1 | F-oct (production fix / plan-end / 추가 진단) | F-oct-prelim-5-4 §6 (a) | **종결** (본 cycle 실행 완료) |
| 2 | filterSubframe ÷4/×4 | F-quint-3 §4.1 동상 | 미갱신 |
| 3 | β init = 0.2 | F-quint-3 §4.2 동상 | 미갱신 |
| 4 | frame 1+ 잔여 | Task 4 측정 결과 반영 | (Task 4 결과 인용) |
| 5 | 회귀 가드 promotion | F-oct-prelim-3 §5 동상 | 미갱신 |
| 6 | 비-contract diagnostic 3건 | F-quint-3 §4.6 동상 | (Task 3 측정 결과 반영) |
| 7 | F-sext-2 / F-sext-3 reactivate | F-oct-prelim-5-4 종결 | 변경 없음 |
| 8 | lsp_lp.go uncommitted | `02bf785` 정식화 완료 | 미갱신 |
| 9 | stagef_bis_diagnostic_test.go untracked | 보존 유지 | 미갱신 (별도 cycle 검토) |
| 10 | tilt.go γ_t proxy | Task 2 fix 완료 | **종결** |

신규 보류 항목 (본 task 산출):

| # | 항목 | 비고 |
|---|------|------|
| 11 | Annex A constant (0.8/0) vs main constant (0.9/0.2) 차이 | 본 cycle 은 분기 *조건* 만 fix. constant 값 spec 정합 (Annex A strict) 은 별도 cycle 영역. ITU contract test PASS 유지가 우선. |
| 12 | (Task 4 결과에 따라) PITCH/FIXED frame 0 또는 ALGTHM frame 1+ 잔여 mismatch | 발생 시 Phase 1l / Phase 0c 잔여 cycle 으로 분기 |

- [ ] **Step 4: Phase 1k 종결 평가**

다음 결정 트리:

| 조건 | 결정 |
|------|------|
| Task 2 GREEN + Task 3 14 게이트 PASS + Task 4 ALGTHM frame 1..3 PST 정합 다수 + PITCH/FIXED frame 0 정합 | **Phase 1k 종결 권고**. 다음 cycle = Phase 1l (full ITU contract test stage V 진입). |
| Task 2 GREEN + Task 3 PASS + Task 4 잔여 mismatch (frame 1+ 또는 다른 vector) | **Phase 1k 종결 보류**. 다음 cycle = 잔여 mismatch 진단 cycle (Phase 1k Stage F-non / 가칭) — single-frame 부호 정합이 multi-frame state propagation 에서 깨지는지 추적. |
| Task 2 E3 발동 (fix 후 RED 잔존) | **Phase 1k 종결 불가**. 다음 cycle = 추가 진단 cycle (예: synth IIR pitch memory 또는 Annex A binary 행동 추적, F-oct-prelim-5-4 §7.3 의 P-SRC-2 검토). |

본 task 의 §4 결정은 **단일** 이며 Phase 0.4 강압-적합 회피 의무 준수.

- [ ] **Step 5: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-5-report.md`:

```markdown
# Phase 1k Stage F-oct-postfix 종합 보고서 + Phase 1k 종결 평가

**작성일**: 2026-05-01
**범위**: F-oct-postfix-1/2/3/4 결합 분석 + Phase 1k 종결 평가.
**산출물**: cycle 결산 + 결정 트리 + 다음 cycle 권고.
**준수**: F-oct-postfix-1~4 + F-oct-prelim-5-4 + spec §A.4.2.3 인용.
**production 변경**: 0 라인 (메타 task).

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)
## 1. F-oct-postfix cycle commit 요약
## 2. 결합 분석 표
## 3. 잔여 보류 항목 갱신 (10 + 신규 1~2)
## 4. Phase 1k 종결 평가 (단일 결정)
## 5. 다음 cycle 권고 (Phase 1l / Phase 0c / 추가 진단)
## 6. 결론
```

```bash
git add docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-5-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-oct-postfix synthesis + Phase 1k closure decision

F-oct-postfix cycle (Task 1 RED, Task 2 γ_t fix GREEN, Task 3 회귀
게이트 누적, Task 4 generalization) 의 결합 분석 + Phase 1k 종결
평가. Task 2 의 internal/postfilter/tilt.go 분기 fix 가 ALGTHM
frame 0 sf0 sample 5..7 spec 정합을 입증한 후 cycle 결산.

Phase 1k 종결 가능 시 Phase 1l 진입 권고. 잔여 mismatch 발견 시
다음 진단 cycle 권고. production 변경 0 (메타 task). 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

### 1. Spec coverage

F-oct-prelim-5-4 §6 결정 (a) production fix cycle + §7.2 다음 cycle 권고 매핑:

- §7.2 "tilt.go γ_t 선택 분기를 spec §A.4.2.3 정합으로 production fix" → **Task F-oct-postfix-2** 단일 fix.
- §7.2 "회귀 게이트 본 cycle 14 게이트 + postfilter package contract test + ALGTHM frame 0 sample 5..7 부호 측정" → **Task F-oct-postfix-3** 누적 검증.
- §7.2 "fix 후 spec want 정합 또는 |Δ| 감소 확인" → **Task F-oct-postfix-1** RED + **Task F-oct-postfix-2** GREEN 단일 입증.
- §7.2 "중단 조건: fix 후 양수 잔존 또는 회귀 신규 FAIL → revert + 추가 진단 cycle 환원" → **E1 / E3** 발동 + **Task F-oct-postfix-5** 다음 cycle 권고.
- §7.3 "Phase 1k 종결 재평가" → **Task F-oct-postfix-5** §4 단일 결정.

5 항목 모두 task 매핑 완료. 누락 0.

### 2. Placeholder scan

본 plan 검토 — "TBD"/"TODO"/"implement later"/"fill in details" 검색:

- Task 1 의 Step 3 에 *완전한 test 코드* 제시 (signature, helper 인용 포함).
- Task 2 의 fix scope 가 *완전한 변경 라인* 으로 제시 (현 코드 + fix 후 코드 + 변경 라인 수 정량).
- Task 4 의 Step 2 에 *완전한 보조 test 코드* 제시.
- 각 task 의 Step N 보고서 outline 에 *각 § 명시* (placeholder 없이).
- 각 commit 메시지 *완전한 한국어 본문* + co-author trailer.

placeholder 0 확인. 단 *Task 4 Step 2 의 측정 결과* 는 본 plan 작성 시점 미확정 (Task 2 fix 후에야 측정 가능) — 이는 Task 자체 실행 결과에 의한 것으로 placeholder 가 아님. 보고서 §2 outline 이 "(Task N 결과 인용)" 으로 명시.

### 3. Type consistency

- Task 1 신규 test `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`: helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `readPSTFrames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0.
- Task 4 신규 test `TestDiagnostic_FoctPostfixExtraVectors`: 동일 helper 재사용.
- Task 2 production fix: `computeTiltMu` signature 변경 0 — 호출부 (`postfilter.go:44`) 변경 0. `pf.agcGainPrev` field 유지 (다른 stage 의 AGC §A.4.2.4 에서 사용).
- 회귀 게이트 14 항목의 test 이름이 Phase 0.2 ↔ 각 task Step 에서 일관 (예: `TestDiagnostic_FoctPrelim5HpFilterInitState` 동일 명칭).
- 본 cycle 의 `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` 가 Task 1 commit 시점 RED, 이후 모든 commit GREEN 의무 — 회귀 게이트 항목 14 으로 명시.

type consistency clean.

### 4. Spec § 인용 정합성 (특별 검토)

본 plan 의 spec 인용은 PDF `pdftotext -layout` 결과의 verbatim — F-oct-prelim-5-4 §3.6 의 "γ_t = 0.9 if long-term postfilter active (g_l > 0)" 해석과 *불일치*. 본 plan 은 PDF 원문 (sign of k1') 을 ground-truth 로 채택 + Phase 0.4 §1 강압-적합 회피 의무 명시 + Task 1 Step 2 에 grep 재확인 의무 명시. 본 결정은 본 plan 자체 의 산출물이며, *F-oct-prelim-5-4 보고서의 spec 해석을 정정* 하는 부수 효과를 가진다 (Task 1 §4 보고서에 정정 사실 명시 의무).

이는 fix 의 *최소 침투 scope* (signature 변경 불필요) 라는 부수적 이점도 가져옴.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, F-oct-prelim / F-sept / F-sext 패턴 답습. 각 task 완료 후 main agent 가 다음 task 진입 권고를 사용자에게 게이트.

**2. Inline Execution** — batch execution. Task 1 RED commit → 사용자 게이트 → Task 2 production fix commit → 사용자 게이트 → Task 3/4/5 batch.

**Recommended user gate before Task F-oct-postfix-1**: 사용자가 본 plan 의 Phase 0.4 §1 (spec § 인용 우회 fit 금지 = PDF 원문 채택) + Task 2 의 fix scope (signature 변경 불필요, 분기 조건 1 라인 + docstring 6 라인) 을 검토 후 진입 승인.

**Which approach?**
