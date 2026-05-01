# Phase 1k Stage F-non-prelim Diagnostic Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-oct-postfix2-prelim 종합 보고서 (`9a5a7f6`, `docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-synthesis-report.md`) §2 비교표가 4 가설 (M1' / M3 / M5 / M6) 모두 반증으로 결함 위치 미식별. 동 보고서 §3 의 4 후보 (X excitation u[0..4] 부호 / Y LP a[] / Z spec 해석 / W PST 출처) 중 후보 X 가 측정 §1.4 (1) "syn[5..7]=+1 의 출처 = u[0..4]=[+1,+1,+1,+1,+0] 자기-피드백" + (2) "synth IIR linear invariant `syn(-u) == -syn(+u)`" 두 측정에 의해 가장 강하게 지지. 사용자 G-N1 결정 = "(a) X 우선 정합 → F-non-prelim plan dispatch" (= "진행"). 본 cycle = X 우선 측정 + Y 보조 + Z 보조 + 종합의 4-task 진단 cycle. **production 변경 0 라인** 진단 cycle. 다음 cycle (F-non-fix 가칭) 의 RED gate = `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` (`56caa72`) 승계.

**Architecture:** 4-task 진단 cycle (TDD 패턴 — failing/측정 test → dump → commit). Task F-non-prelim-1 = X 측정 (excitation u[0..4] sub-항 분리: gain g_p Q14 / g_c Q12 / pitch contribution adaptive codebook / fcb contribution fixed codebook → 어느 sub-항이 부호 *결정* 인지 식별). Task F-non-prelim-2 = Y 측정 (LP a[0..10] cross-check, sample 5..7 영역 한정 spec §A.3.5 / §4.1.5 정합). Task F-non-prelim-3 = Z 측정 (spec 해석 재검토, postfilter chain "정합" 정의의 §A.4.2.* 재인용 + chain 구조 cross-ref, 비용 LOW 보고서 only). Task F-non-prelim-4 = 종합 (X/Y/Z 결합 + 결정 트리 — 단일 식별→fix cycle / 2+ 잔존→추가 cycle / 0 식별→W 후보 PST 출처 재진입).

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §A.3.* (Annex A decoder excitation / LP synthesis) + §4.1.5 (Decoding of the adaptive and fixed-codebook gains) + §A.4.1 (LP synthesis filter) + §A.4.2.1-4 (Postfilter cascade) + §3.10 (LP synthesis IIR linearity) + §4.3 Table 9 (state init) + READMETV.txt (PST format) + 기존 F-quart/F-sext/F-sept/F-oct-prelim/F-oct-prelim-5/F-oct-postfix-1/F-oct-postfix2-prelim 진단 하니스 (회귀 게이트 16건). **외부 G.729 구현 0건 참조** (E4) — 사용자 G1 결정 ("(c) Annex A binary 거부") 유지. 단 이미 repo committed 인 PST 파일 (`testdata/itu/test_vectors/`) 은 입력 stimulus 로 계속 사용 가능.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 직전 cycle 의 결정 / 측정 정리 (누적 진단 컨텍스트)

**직전 cycle = F-oct-postfix2-prelim (`9a5a7f6` synthesis)**:

- F-oct-postfix2-prelim-1 (`ff5534a`): chain dump baseline harness 추가 — ALGTHM frame 0 sf0 sample 5..7 의 chain stage 출력 일괄 dump. baseline = `out[5..7]=[+2,+2,+2]`, PST want = `[-1,-1,-1]`, Δ=[+3,+3,+3] (sample-uniform).
- F-oct-postfix2-prelim-2 (`6dc851e`): M5 (excitation pre-postfilter 부호) 측정. **결과 = REFUTED** — u[5..7]=[+0,+0,+0] (zero excitation). syn[5..7]=+1 의 부호 *생성* 단계는 synth IIR 외부 (excitation 자체 부호 결함 0).
- F-oct-postfix2-prelim-3 (`cb9529d`): M6 (PST want 데이터 부호) 측정. **결과 = REFUTED** — ALGTHM.PST byte 10..15 = `ff ff ff ff ff ff` = int16 LE [-1,-1,-1] 정상. 9 vector 분포 `[+,+,+]` = 0 vector (production `[+2,+2,+2]` 정합 PST 부재). PST 데이터 결함 0.
- F-oct-postfix2-prelim-4 (`f04ec88`): M1' (postfilter 외 분기) + M3 (synth IIR memory) 측정. **결과 = 모두 REFUTED** — postfilter 6 stage sign-chain 모두 [+ + +] 보존 (cover 결손 0), IIR memory pre-5 → post-7 sign change 0/10, `syn(-u) == -syn(+u)` linear invariant 확립 (Pass-1 sample 0..7).
- F-oct-postfix2-prelim-5 종합 (`9a5a7f6`): 4 가설 모두 반증 → §3 의 결함 위치 후보 X/Y/Z/W 평가표 도출. **후보 X (excitation u[0..4] 부호) 우선순위 HIGH** — 측정 §1.4 (1)+(2) 가 직접 지지. §4.1 다음 cycle = F-non-prelim, 후보 X 우선 + Y 보조. 사용자 G-N1 = "(a) X 우선 정합 → F-non-prelim plan dispatch" = "진행".

**누적 측정 사실 (본 cycle 진입 premise)**:

| 사실 | 출처 | 비고 |
|------|------|------|
| ALGTHM frame 0 sf0 sample 5..7 PST want = [-1, -1, -1] (PST 도메인) | Task 3 §2 (`cb9529d`) | byte-level 검증 완료 |
| production out[5..7] = [+2, +2, +2] (PST 도메인) → Δ = [+3, +3, +3] sample-uniform | Task 1 §2 (`ff5534a`) | 부호 mismatch 잔존 |
| u[5..7] = [+0, +0, +0] (zero excitation) | Task 2 §2 (`6dc851e`) | M5 반증 근거 |
| u[0..4] = [+1, +1, +1, +1, +0] (양 입력) | Task 4 §3 (`f04ec88`) | **X 후보의 직접 출처** |
| syn[0..4] = [+1, +2, +2, +2, +1], syn[5..7] = [+1, +1, +1] | Task 4 §3 | sample 5..7 부호 = u[0..4] 양 입력의 자기-피드백 |
| pastSynth (codec-start) = [0; 10] | §4.3 Table 9 | IIR memory init 정상 |
| `syn(-u) == -syn(+u)` for sample 0..7, sign-change 0/10 | Task 4 §5 | synth IIR linear invariant 확립 |
| postfilter 6 stage sign-chain 모두 [+ + +] (longterm `compute` R=20>0 / tilt `inactive` γ_t=3277 / AGC `init-seed`) | Task 4 §2 | postfilter 부호 보존, cover 결손 0 |
| 9 PST vector frame 0 sf0 sample 5..7 분포: `[-,-,-]`×3 + `[0,0,0]`×5 + `[-,0,0]`×1 + `[+,+,+]`×0 | Task 3 §3 | spec want 부호 = `−` 또는 `0` 우세, `+` 부재 |

**핵심 추론** (보고서 §1.4 (1)): u[0..4] 가 *음수* 입력이었다면 IIR linear (Task 4 §5) 에 의해 syn[5..7] 도 음수 → postfilter 6 stage 부호 보존 (Task 4 §2) → out[5..7] 도 음수 → PST want [-1,-1,-1] 정합. 따라서 부호 결정 위치 = **u[0..4] 입력의 sub-항 (g_p / g_c / fcb code / pitch contribution / LP a[])** 중 하나. 본 cycle = sub-항 분리 측정.

### Phase 0.2 invariant 재확인 (E1-E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.3 의 1~15 PASS 항목, 단 항목 16 = postfix-1 RED 는 *예외 — 의도된 RED 잔존*) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | 본 cycle 의 임의 task 의 spec § 인용이 PDF verbatim grep 결과와 불일치 (= 휴리스틱 fit) | 즉시 측정 폐기 + spec § PDF 직접 재발췌 + 보고서 §0 에 도출 과정 정량 기록 |
| **E3** | 본 cycle Task 4 종합에서 X/Y/Z 중 2+ 가 잔존 (단일 식별 불가) | Task 4 §4 다음 cycle 권고 = "추가 진단 cycle (각 잔존 후보별 분리 측정)". 단일 fix cycle 진입 금지. |
| **E4** | 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조/실행 흔적 발견. **사용자 G1 결정 = Annex A binary 거부** (black-box 행동 추적 포함). | 즉시 작업 중단 + 사용자 통보 + 해당 인용/binary 제거 후 재시작 |
| **E5** | 본 cycle 의 production 변경 라인 > 0 (메타 의무 — 진단 cycle) | 즉시 `git revert HEAD` + commit 재구성 (production 변경 제거) |

### Phase 0.3 회귀 게이트 명세 (16건 — 누적 contract test 15 + Task 1 신규 baseline)

각 task commit 직후 *반드시* 실행 (총 16 게이트 — F-oct-postfix2-prelim 종결 시점의 15 + 본 cycle Task 1 의 신규 측정 baseline 1; 단 항목 16 = postfix-1 RED 는 *항상 RED 의무* — 다음 fix cycle 의 GREEN gate):

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
| 15 | F-oct-postfix2-prelim | `TestDiagnostic_FoctPostfix2PrelimChainDump` (+ `M5ExcitationSignTrace`, `M6PSTSignVerify`, `M3IIRMemory` 동상 PASS) | PASS |
| 16 | F-oct-postfix-1 RED | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **RED 잔존 의무** (다음 fix cycle 의 GREEN gate) |

추가 sanity:
- `go vet ./...` clean (각 task commit 직후).
- 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 본 cycle 진입 시점 FAIL 유지. 본 cycle 어떤 task 도 본 3건의 상태를 변화시키지 *않아야* 함 (production 변경 0 라인 의무로 자동 보장).

본 cycle Task 1 의 신규 측정 harness (`TestDiagnostic_FnonPrelimXExcitationSubterms`) 는 *측정-only* (assertion 0 또는 PASS 의무) — 회귀 게이트 17번째 항목으로 자동 promotion 하지 *않는다* (자동 promotion 금지, F-oct-postfix2-prelim §0.3 동상). Task 4 §3 잔여 보류 항목으로 처리.

### Phase 0.4 강압-적합 회피 의무 (forced-fit avoidance)

본 cycle 은 *진단 cycle* 이며 직전 cycle (4 가설 모두 반증) 의 측정 데이터를 누적 ground-truth 로 사용. Phase 0.4 의무가 production fix cycle 보다 *더 엄격*. 다음 패턴을 적극 회피:

1. **Sub-항별 측정 분리 의무**: Task 1 의 X 측정에서 sub-항 (g_p / g_c / pitch contribution / fcb contribution) 평가 시 *임의 우선순위 결정 금지*. 4 sub-항 모두 raw 값 + 부호 + Q-format 을 dump 후 측정 데이터로만 부호 결정 sub-항 식별. spec 인용 또는 직관적 "g_c 가 가장 가능성 높음" 식의 추론 금지.
2. **spec § 인용 우회 fit 금지**: Task 1 (§A.3.* + §4.1.5) / Task 2 (§A.3.5 + §4.1.5 + §A.3.3) / Task 3 (§A.4.2.* + §4.2) 의 spec 인용은 모두 PDF `pdftotext -layout` verbatim grep 으로 채택. 결합 해석 또는 *간접 증거* 는 보고서 §0 에서 명시.
3. **음성 결과 (X 반증) 도 결과로 인정**: 본 cycle X 측정 결과가 "u[0..4] sub-항 모두 spec 정합" (= X 반증) 일 경우도 *유효한 측정 결과*. Task 4 §3 결정 트리에 따라 Y 단독 / Z 보조 / W 진입 (M6 cycle 재실행) 의 다음 cycle 권고 자동 도출.
4. **scope crawl 금지**: 본 cycle 모든 task 의 production 변경 = 0 라인. test 변경 = 측정 harness 신규만. helper 신규 0 (기존 `decoder` / `synth` / `gain` / `fcb` / `pitch` package helper 재사용). spec 인용은 §A.* + §3.10 + §4.1.5 + §4.2 + §4.3 Table 9 영역 한정.
5. **g_l 영속화 후보 ① 영구 제외**: 사용자 G1 결정 = "후보 ③ pivot" — 본 cycle 도 ① (g_l state 영속화 + tilt.go read) 와 관련된 측정/fix 일체 도입 금지. (본 cycle 은 g_l 보다 상류 — gain decoding sub-항 — 측정이므로 자연 회피.)
6. **W 후보 강압적 진입/dismiss 동시 금지**: Task 4 §3 결정 트리에서 X/Y/Z 모두 반증 시에만 W (M6 cycle 재실행) 진입. M6 반증 데이터 (`cb9529d` byte-level + 9 vector 분포) 와 명시적 모순 — Task 4 §3 비고에 모순 분석 의무.

### Phase 0.5 사전 보유 working tree 보존 의무

`internal/decoder/stagef_bis_diagnostic_test.go` (untracked, F-bis baseline 잔존) 는 본 cycle 4 task 어떤 commit 도 add 하지 않는다. 사후 working tree 의 `?? internal/decoder/stagef_bis_diagnostic_test.go` 가 F-oct-postfix2-prelim synthesis 시점 (`9a5a7f6`) 과 동일하게 유지됨을 각 task §0 보고서에서 확인.

---

## Spec § 인용 (본 cycle 의 ground-truth 공통)

각 task §0 에서 PDF `pdftotext -layout` verbatim grep 으로 재확인 의무. 본 §은 plan 작성 시점의 추정 인용 — task 실행 시점 grep 결과와 불일치 시 즉시 E2 발동.

**(인용 1)** ITU-T G.729 (06/2012) PDF §4.1.5 *Decoding of the adaptive and fixed-codebook gains* (X 후보의 직접 spec ground-truth — gain decoding 의 부호 항):
- gain prediction predictor `g_c̃` + correction factor `γ̂` → `g_c` 복원 + adaptive codebook gain `g_p` 디코딩의 spec 정의.
- Task 1 진입 시 PDF verbatim grep + Q-format (g_p Q14 / g_c Q12) 확인.

**(인용 2)** ITU-T G.729 (06/2012) PDF §A.3.* *Decoder* (Annex A decoder 의 excitation reconstruction + LP synthesis chain). PDF page 39-42, §A.3.1 (Decoding of LSP) / §A.3.2 (LSP→LP) / §A.3.3 (Decoding of pitch delay) / §A.3.4 (Decoding of fixed-codebook vector) / §A.3.5 (Computing the excitation = `u[n] = g_p · v[n] + g_c · c[n]`).
- **§A.3.5 가 Task 1 의 핵심**: u[n] = pitch contribution (g_p · v[n]) + fcb contribution (g_c · c[n]) 의 가법 분해.

**(인용 3)** ITU-T G.729 (06/2012) PDF §A.4.1 *LP synthesis filter* + §3.10 (LP synthesis IIR 의 linearity 정의 — Task 4 §5 `syn(-u) == -syn(+u)` 측정의 spec ground-truth).

**(인용 4)** ITU-T G.729 (06/2012) PDF §A.4.2 *Postfilter* (chain stage 순서) + §A.4.2.1 (Hf short-term) / §A.4.2.2 (Hp long-term) / §A.4.2.3 (Ht tilt) / §A.4.2.4 (AGC) — Task 3 의 Z 측정 ground-truth.

**(인용 5)** ITU-T G.729 (06/2012) PDF §4.2 *Post-processing* (= post-processing parent) + §4.3 Table 9 (codec-start state init).

**(인용 6)** READMETV.txt: PST 파일 format = Intel PC = 16-bit signed little-endian, frame = 80 sample. (Task 4 의 W 후보 평가 cross-ref ground-truth.)

각 task 는 본 §의 인용을 baseline 으로 채택. 추가 spec 인용 시 해당 task §0 에 PDF page + verbatim 추가.

---

## Task F-non-prelim-1: X 측정 — excitation u[0..4] sub-항 분리 (gain g_p/g_c + pitch/fcb contribution)

**Goal:** **후보 X** = ALGTHM frame 0 sf0 의 excitation u[0..4] = [+1,+1,+1,+1,+0] 의 부호 결정 sub-항 식별. spec §A.3.5 의 가법 분해 `u[n] = g_p · v[n] + g_c · c[n]` 에 따라 sub-항 4개 (gain g_p Q14 / gain g_c Q12 / pitch contribution `g_p · v[n]` / fcb contribution `g_c · c[n]`) 를 sample 0..4 한정으로 분리 dump. 어느 sub-항의 부호가 u[0..4] 양 입력을 *결정* 하는지 식별. 단일 sub-항 식별 시 다음 fix cycle (F-non-fix) 의 production fix scope 자동 결정.

**Files:**
- Create: `internal/decoder/stagef_fnonprelim_diagnostic_test.go`
- Create: `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-1-report.md`

production 변경 0 라인. test 변경 = 신규 1 파일.

### Spec § 인용

본 plan 상단 §"Spec § 인용" 인용 1 (§4.1.5) + 인용 2 (§A.3.5). Task 진입 시 PDF verbatim grep 으로 확정.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
9a5a7f6 docs(plans): F-oct-postfix2-prelim synthesis + cycle decision
```

기타 파일 modified 잔존 시 즉시 사용자 통보.

Run (회귀 게이트 baseline, Phase 0.3 의 15 PASS 항목 + 항목 16 RED 잔존):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5|OctPostfix2Prelim)" -v
go test ./internal/postfilter/ ./internal/synth/ -v -run Contract
go vet ./...
```

Expected: 15 PASS + 항목 16 RED + `go vet` clean. 출력 요약을 보고서 §1 에 인용.

- [ ] **Step 2: spec § PDF verbatim grep**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 40 "4.1.5"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.5"
```

verbatim 인용 (특히 §A.3.5 의 `u[n] = g_p · v[n] + g_c · c[n]` 가법 분해 + Q-format) 을 보고서 §0 spec 인용 섹션에 기록. 인용 grep 결과와 본 plan §"Spec § 인용" 추정이 불일치 시 즉시 E2 발동.

- [ ] **Step 3: production code 의 sub-항 dump 점 식별**

기존 production code 에서 sub-항 측정 점 식별 (test 직접 호출 가능 함수):

```
grep -n "func " internal/gain/decode.go internal/pitch/adaptive.go internal/fcb/decode.go internal/decoder/subframe.go
```

dump 대상 sub-항 (frame 0 sf0 sample 0..4 한정):

| sub-항 | 함수 / 변수 | dump 형식 | spec § |
|--------|------------|-----------|--------|
| g_p (adaptive codebook gain Q14) | `gain.Decoder.Decode(...)` 의 `gpQ14` 반환 | int16 + 부호 | §4.1.5 |
| g_c (fixed codebook gain Q12) | `gain.Decoder.Decode(...)` 의 `gcQ12` 반환 | int16 + 부호 | §4.1.5 |
| pitch contribution v[0..4] (Q0) | `pitch.Adaptive` (또는 `subframe.go` 내 호출 결과) → adaptive codebook output | int16[5] + 부호 | §A.3.5 (g_p · v 항) |
| fcb contribution c[0..4] (Q13) | `fcb.Decode(...)` 의 `c` 출력 | int16[5] + 부호 | §A.3.4, §A.3.5 |
| u[0..4] (sample-별 합성) | `subframe.go` 의 excitation 합산 출력 | int16[5] + 부호 (= [+1,+1,+1,+1,+0] expected) | §A.3.5 |

**측정 점이 production API 로 노출되지 않는 경우** (예: subframe 내부 임시 변수): test 가 `subframe.go` 의 동일 path 를 *복제* (decoder package 내 white-box test, 동일 input 으로 sub-함수 직접 호출) 하여 sub-항 raw 값 도출. 복제 path 와 production path 의 동치성은 u[0..4] 합산 결과가 `[+1,+1,+1,+1,+0]` 로 일치하는지로 검증 (replicated chain == production 의무, F-oct-postfix2-prelim Task 4 §2 동상).

- [ ] **Step 4: dump harness test 작성 — `stagef_fnonprelim_diagnostic_test.go`**

`internal/decoder/stagef_fnonprelim_diagnostic_test.go` 신규 작성 (구체 outline):

```go
package decoder

import "testing"

// TestDiagnostic_FnonPrelimXExcitationSubterms decomposes the ALGTHM
// frame 0 sf0 excitation u[0..4] = [+1,+1,+1,+1,+0] into spec §A.3.5
// sub-terms (g_p, g_c, pitch contribution v[n], fcb contribution c[n])
// to identify which sub-term determines the sign of u[0..4].
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.3.5 (PDF p.41) =
//   u[n] = g_p · v[n] + g_c · c[n]
// + §4.1.5 (PDF p.21) gain decoding (g_p Q14, g_c Q12).
//
// F-oct-postfix2-prelim synthesis (9a5a7f6) §1.4 (1) identifies
// u[0..4] as the source of syn[5..7]=+1 self-feedback (4 hypotheses
// M1'/M3/M5/M6 all REFUTED). Candidate X (excitation u[0..4] sign)
// = HIGH priority; sub-term decomposition narrows fix scope.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FnonPrelimXExcitationSubterms(t *testing.T) {
	// 1) decode ALGTHM frame 0 → reach subframe 0 dispatch
	// 2) intercept gain.Decoder.Decode → record gpQ14, gcQ12
	// 3) intercept pitch adaptive codebook → record v[0..4]
	// 4) intercept fcb.Decode → record c[0..4]
	// 5) compute g_p · v[n] + g_c · c[n] (replicated path) → record per-sample
	// 6) cross-check sum vs production u[0..4] = [+1,+1,+1,+1,+0]
	// 7) t.Logf each sub-term raw value + sign + Q-format
}
```

helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0. gain / fcb / pitch 호출은 production exported API 또는 same-package unexported.

측정 출력 형식 (예시):
```
[X g_p Q14]   value=<int16>  sign=<+|-|0>  Q-format=Q14
[X g_c Q12]   value=<int16>  sign=<+|-|0>  Q-format=Q12
[X v[0..4]]   pitch contrib = [<i> <i> <i> <i> <i>]  signs=[<...>]
[X c[0..4]]   fcb contrib   = [<i> <i> <i> <i> <i>]  signs=[<...>]
[X g_p·v[0..4]]  = [<i> <i> <i> <i> <i>]  signs=[<...>]
[X g_c·c[0..4]]  = [<i> <i> <i> <i> <i>]  signs=[<...>]
[X u[0..4]]   sum = [<i> <i> <i> <i> <i>]  expected=[+1 +1 +1 +1 +0]  match=<true|false>
[X 결정] sign-determining sub-term = <g_p | g_c | v | c | hybrid | undetermined>
```

- [ ] **Step 5: 측정 + sub-항별 부호 결정성 평가**

Run:
```
go build ./...
go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimXExcitationSubterms -v
```

Expected: build PASS, test PASS, t.Logf 출력에 sub-항별 raw 값 + 부호. 출력 verbatim 을 보고서 §2 에 인용.

X 후보 sub-항 평가 (보고서 §3, Phase 0.4 §1 — 측정 데이터만):

| sub-항 측정 결과 | X 평가 |
|------------------|--------|
| g_p > 0 + v[0..4] > 0 + g_c · c[0..4] = 0 → g_p · v 항 단독 양수 결정 | **g_p · v 단독 결정** — fix scope = adaptive codebook (pitch.Adaptive 또는 g_p decoding) |
| g_c > 0 + c[0..4] > 0 + g_p · v[0..4] = 0 → g_c · c 항 단독 양수 결정 | **g_c · c 단독 결정** — fix scope = fcb.Decode 또는 g_c decoding |
| 두 항 모두 비-zero + 합산이 양수 → hybrid | **hybrid** — Task 4 §3 에서 추가 진단 cycle 권고 |
| u[0..4] 합산 ≠ [+1,+1,+1,+1,+0] (replicated path 와 production path 불일치) | **replication 결함** — 복제 path 검토 의무 + 보고서 §0 에 한계 명시 |

- [ ] **Step 6: 16 회귀 게이트 재확인**

Run: 15 PASS + 항목 16 RED + 신규 측정 `TestDiagnostic_FnonPrelimXExcitationSubterms` PASS.

1+ FAIL 시 (단 항목 16 제외) E1 발동.

- [ ] **Step 7: 보고서 작성**

`docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-1-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-1 보고서 — X excitation u[0..4] sub-항 분리

**작성일**: 2026-05-03
**범위**: 후보 X (excitation u[0..4] 부호) 의 sub-항 (g_p / g_c / v / c) 분리 측정.
**산출물**: 측정 함수 1 신규 파일 + sub-항별 raw + 부호 결정성 평가.
**준수**: production 변경 0, 외부 G.729 0 참조, F-oct-postfix2-prelim Task 4 §3 의 u[0..4] dump baseline 인계.

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G-N1 결정 정합성
## 1. 회귀 게이트 baseline (15 PASS + 항목 16 RED + 신규 PASS)
## 2. sub-항 raw 출력 (sample 0..4 한정)
## 3. X 후보 sub-항 부호 결정성 평가 (단일 / hybrid / replication 결함)
## 4. F-oct-postfix2-prelim Task 4 §3 u[0..4] dump 와의 cross-check
## 5. Task 2 (Y) 진입 의무 항목
```

- [ ] **Step 8: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_fnonprelim_diagnostic_test.go
?? docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-1-report.md
```

```bash
git add internal/decoder/stagef_fnonprelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-non-prelim-1 X excitation sub-term decomposition

F-oct-postfix2-prelim 종합 (9a5a7f6) §3 후보 X (excitation u[0..4]
부호) 우선 채택 — 사용자 G-N1 = "(a) X 우선 정합". §A.3.5 의 가법
분해 u[n] = g_p · v[n] + g_c · c[n] 에 따라 ALGTHM frame 0 sf0
sample 0..4 의 sub-항 (g_p Q14 / g_c Q12 / pitch contrib v[n] /
fcb contrib c[n]) 분리 dump. 부호 결정 sub-항 식별로 다음 fix
cycle (F-non-fix) 의 production fix scope 자동 결정.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조
(Annex A binary 사용 금지 — G1 결정).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-non-prelim-2: Y 측정 — LP a[] cross-check (sample 5..7 영역 한정)

**Goal:** **후보 Y** = synth IIR 입력측 LP 계수 a[0..10] 의 spec 정합성 cross-check. F-oct-postfix2-prelim Task 4 §3 dump = `a[0..10] = [4096, -2197, -375, -4, -144, -68, 303, 145, -33] Q12` (값 일부) 의 *spec reference* (§A.3.2 LSP 디코딩 + §A.3.3 LSP→a[] Levinson-Durbin / 또는 §4.1.5 spec 정의) cross-check 측정. F-sept-2 (`TestDiagnostic_FseptLPReferenceCrossCheck`) 가 frame 0 전반 PASS — 본 task = sample 5..7 영역 한정 a[] 변환 정합성 + (선택적) a[] 부호 forced-flip stimulus 시 syn[5..7] 변화 측정.

**Files:**
- Modify: `internal/decoder/stagef_fnonprelim_diagnostic_test.go` (Y 측정 함수 1개 추가, 기존 X test 옆)
- Create: `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-2-report.md`

production 변경 0 라인. test 변경 = 기존 파일에 함수 1 추가.

### Spec § 인용

본 plan 상단 §"Spec § 인용" 인용 2 (§A.3.2 / §A.3.3) + 인용 3 (§A.4.1 / §3.10). Task 진입 시 PDF verbatim grep.

- [ ] **Step 1: 사전 조건 + Task 1 commit hash 인용**

Run: `git log --oneline -3`

Expected: Task 1 commit + `9a5a7f6` + `f04ec88`.

- [ ] **Step 2: spec § PDF verbatim grep**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.2"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.3"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 25 "A.4.1"
```

verbatim 인용 (특히 LSP→LP 변환 §A.3.5 또는 §4.1.5 — task 진입 시 PDF page 확인) 을 보고서 §0 에 기록. 인용 grep 결과와 본 plan §"Spec § 인용" 추정이 불일치 시 즉시 E2 발동.

- [ ] **Step 3: Y 측정 함수 추가 — `TestDiagnostic_FnonPrelimYLPCrossCheck`**

기존 `stagef_fnonprelim_diagnostic_test.go` 에 함수 1 추가. 측정 항목:

(a) **a[0..10] dump (sample 5..7 활용 영역 한정)**: F-oct-postfix2-prelim Task 4 §3 의 frame 0 sf0 a[] dump 재측정 + cross-check (값 변화 0 의무).

(b) **F-sept-2 reference cross-check**: F-sept-2 (`TestDiagnostic_FseptLPReferenceCrossCheck`) 가 사용한 reference (또는 동일 reference 함수 호출) 와 본 task 의 a[0..10] 부호/값 일치 확인. F-sept-2 가 frame 0 전반 PASS → a[] 자체는 spec 정합 baseline.

(c) **(선택적) Forced a[] sign-flip stimulus**: synth.Filter 에 a[1..10] → -a[1..10] 입력 시 syn[5..7] 부호 변화 측정. F-oct-postfix2-prelim Task 4 §5 의 IIR linear invariant (`syn(-u) == -syn(+u)`) 와 평행 — a[] 부호 flip 도 syn 부호 flip 을 *유발* 한다면 a[] 가 부호 결정성 보유. (단 본 측정은 spec 정합 a[] 의 부호가 *현재* 정합인지 식별과는 독립.)

측정 출력 형식 (예시):
```
[Y a[0..10] (frame 0 sf0)]    [<11 int16>]  Q12
[Y F-sept-2 reference cmp]    a-byte-equal=<true|false>  sign-equal=<true|false>
[Y forced a-sign-flip syn[5..7]]  baseline=[<3 int>]  flipped=[<3 int>]  sign-flip-induced=<true|false>
[Y 결정] LP a[] spec 정합성 = <정합|결함|미결정>; 부호 결정성 = <보유|부재|hybrid>
```

helper 신규 0 — F-sept-2 reference 호출 함수 재사용 (or 동일 reference 데이터 직접 비교).

- [ ] **Step 4: 측정 + Y 후보 평가**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimYLPCrossCheck -v`

Expected: PASS, t.Logf 출력 verbatim 을 보고서 §2 에 인용.

Y 후보 평가 (보고서 §3):

| 측정 결과 | Y 평가 |
|-----------|--------|
| a[0..10] = F-sept-2 reference 와 byte-equal + sign-equal | **Y 반증** — LP a[] 자체 spec 정합, 부호 결함은 LP path 외부 |
| a[0..10] reference 와 부호 mismatch (1+ 계수) | **Y 유력** — fix scope = `internal/synth` LSP→a[] 변환 (§A.3.2/3) |
| forced a-sign-flip 가 syn[5..7] 부호 flip 유발 + 현재 a[] 가 spec reference 와 부호 일치 | **Y 부분 정합** — a[] 가 부호 결정성을 보유하나 *현재 값*은 spec 정합 (즉 X 후보가 잔존 우선) |

- [ ] **Step 5: 16 회귀 게이트 재확인**

Run: 15 PASS + 항목 16 RED + 신규 측정 2건 (Task 1 의 X + Task 2 의 Y) PASS.

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-2-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-2 보고서 — Y LP a[] cross-check

**작성일**: 2026-05-03
**범위**: 후보 Y (LP a[0..10] 부호 결함) sample 5..7 영역 한정 측정.
**산출물**: 측정 함수 1 추가 + a[] reference cross-check + (선택적) forced-flip stimulus + Y 평가.
**준수**: F-sept-2 baseline 보존, F-oct-postfix2-prelim Task 4 §3 a[] dump 인계.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§A.3.2/3 + §A.4.1)
## 1. 15 회귀 게이트 PASS + 항목 16 RED 재확인 + 신규 X PASS
## 2. Y 측정 raw 출력 (a[0..10] + reference cmp + forced-flip)
## 3. Y 후보 평가 (반증 / 유력 / 부분 정합)
## 4. F-sept-2 cross-check 결과
## 5. Task 3 (Z) 진입 의무
```

```bash
git add internal/decoder/stagef_fnonprelim_diagnostic_test.go \
        docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-non-prelim-2 Y LP a[] cross-check

후보 Y (LP a[0..10] 부호 결함, §A.3.2/3 LSP→a[] 변환) 의 sample 5..7
영역 한정 cross-check. F-oct-postfix2-prelim Task 4 §3 의 a[] dump
재측정 + F-sept-2 reference cross-check + 선택적 forced a-sign-flip
stimulus 의 syn[5..7] 부호 변화 측정.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-non-prelim-3: Z 측정 — postfilter chain "정합" 정의 spec 재인용 (보고서 only)

**Goal:** **후보 Z** = spec 해석 자체 — postfilter chain "정합" 정의의 ITU-T G.729 PDF §A.4.2.* (postfilter cascade) + §4.2 (post-processing parent) 재인용 + 우리 chain 구조 (`internal/postfilter/postfilter.go`) 와의 cross-ref. PST want 의 *비교 도메인* / *frame-edge 정의* / *sample alignment* spec verbatim 재발췌. 비용 LOW (보고서 only — 측정 코드 0, 측정 test 0). 본 task 의 산출 = spec 인용 catalog + cross-ref 표.

**Files:**
- Create: `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-3-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task).

### Spec § 인용

본 plan 상단 §"Spec § 인용" 인용 4 (§A.4.2.*) + 인용 5 (§4.2 + §4.3 Table 9). Task 진입 시 PDF verbatim grep 으로 확정.

- [ ] **Step 1: 사전 조건 + Task 2 commit hash 인용**

Run: `git log --oneline -4`

Expected: Task 2 + Task 1 + `9a5a7f6` + `f04ec88`.

- [ ] **Step 2: spec § PDF verbatim grep (Z 의 핵심 — 본 task 산출의 ground-truth)**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 50 "A.4.2 "
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.4.2.1"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.4.2.2"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.4.2.3"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.4.2.4"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 40 "4.2 "
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 20 "Table 9"
```

verbatim 인용 catalog (각 § + PDF page) 를 보고서 §2 에 기록.

- [ ] **Step 3: production chain 구조 cross-ref**

Run:
```
grep -n "func " internal/postfilter/postfilter.go internal/postfilter/longterm.go internal/postfilter/shortterm.go internal/postfilter/tilt.go internal/postfilter/agc.go internal/decoder/decode.go internal/decoder/hpfilter.go
```

cross-ref 표 작성 (보고서 §3):

| spec § | spec stage | production 구현 | 일치 / 차이 |
|--------|-----------|-----------------|-------------|
| §A.4.2 chain order | long-term Hp → short-term Hf → tilt Ht → AGC | `postfilter.Filter` 호출 순서 | 일치 / 차이 |
| §A.4.2.1 Hf | short-term postfilter γ_n / γ_d | `shortterm.go` | (인용 ↔ 구현 비교) |
| §A.4.2.2 Hp | long-term postfilter g_l clamp | `longterm.go` | 동상 |
| §A.4.2.3 Ht | tilt compensation γ_t = 0.8 if k1' < 0 / 0 if k1' ≥ 0 | `tilt.go` | F-oct-postfix-2 revert 측정 후 spec strict reading 정합 (Δ=0) |
| §A.4.2.4 AGC | adaptive gain control α-smoothing | `agc.go` | 동상 |
| §4.2 parent | post-processing = postfilter + hpfilter | `decoder.Decode` | hpfilter 직후 PST 도메인 출력 ×2 scale |
| §4.3 Table 9 | codec-start state init | 각 module Reset() | F-oct-prelim-5-3 §3.3 zero dump 정합 |

- [ ] **Step 4: PST want "비교 도메인" 재검토 (Z 의 핵심 측정)**

Z 후보의 핵심 = "PST want sample 5..7 = [-1,-1,-1] 의 *비교 단위* / *비교 도메인* 이 우리 production 출력 도메인과 정합한가". F-oct-prelim-5-1 P-SRC-2 분류 (PST source verbatim) + F-oct-postfix2-prelim Task 3 §2 (byte-level int16 LE) 가 *byte-level* 정합 입증. 그러나 *해석 도메인* (예: PST 가 다른 sample-rate / sub-band / decimation / time-shift 출력일 가능성) 은 미검증.

Run:
```
cat docs/superpowers/specs/itu/READMETV.txt | grep -B 2 -A 10 "PST"
cat docs/superpowers/specs/itu/READMETV.txt | grep -B 2 -A 10 "frame"
```

verbatim 발췌 + 해석:

| 의문 | spec 답 (verbatim 출처) |
|------|-------------------------|
| PST 의 sample-rate? | (READMETV.txt verbatim — "8 kHz" 추정) |
| PST 의 frame size? | (READMETV.txt — "80 samples per frame" 추정) |
| PST 출력의 chain 종점? | (PDF §4.2 / §A.4.2 의 "post-processing output" 정의) |
| PST want 의 *비교 단위* (sample-by-sample)? | (PDF §A.4.2 cascade 종점 = AGC 출력 = post-processing 종점) |

- [ ] **Step 5: Z 후보 평가**

Z 평가 (보고서 §4):

| 측정 결과 | Z 평가 |
|-----------|--------|
| spec § verbatim ↔ production cross-ref 모두 일치 + PST 비교 도메인 정합 | **Z 반증** — spec 해석 자체에 결함 0 |
| 1+ § 인용 ↔ production 차이 식별 | **Z 유력** — fix scope = 식별된 § 의 production 정합화 |
| PST 비교 도메인 (sample-rate / frame size / 종점 chain) 불일치 | **Z 부분** — 비교 단위 재정의 필요 (READMETV.txt 또는 PDF 추가 인용 의무) |

- [ ] **Step 6: 16 회귀 게이트 재확인**

Run: 15 PASS + 항목 16 RED + 본 cycle 신규 측정 (X + Y) PASS. 본 task 는 코드 변경 0 → 게이트 자동 보존.

- [ ] **Step 7: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-3-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-3 보고서 — Z spec 해석 재검토

**작성일**: 2026-05-03
**범위**: 후보 Z (postfilter chain "정합" 정의 spec 재인용) — 비용 LOW 보고서 only.
**산출물**: spec § verbatim 인용 catalog + production cross-ref 표 + PST 비교 도메인 평가.
**준수**: production 변경 0, test 변경 0, 외부 G.729 0 참조.

## 0. escape hatch 평가 + 사전 조건
## 1. 16 회귀 게이트 PASS 재확인 (자동 보존)
## 2. spec § PDF verbatim 인용 catalog (§A.4.2.* + §4.2 + §4.3 Table 9)
## 3. production chain 구조 cross-ref 표
## 4. PST 비교 도메인 재검토 (READMETV.txt + PDF §4.2)
## 5. Z 후보 평가 (반증 / 유력 / 부분)
## 6. Task 4 (synthesis) 진입 의무
```

```bash
git add docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-3-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-non-prelim-3 Z spec interpretation review

후보 Z (postfilter chain "정합" 정의 spec 재인용) 의 LOW-cost 검토 —
ITU-T G.729 PDF §A.4.2.* (postfilter cascade) + §4.2 (post-processing
parent) + §4.3 Table 9 (state init) verbatim 재인용 + production
chain (internal/postfilter, internal/decoder) 와의 cross-ref. PST want
의 비교 도메인 (sample-rate / frame size / chain 종점) 정합성 재검토.

assertion 0 (보고서 only). production 변경 0 라인. test 변경 0.
외부 G.729 0 참조 (G1 결정 정합).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-non-prelim-4: 종합 + 결정 트리 + 다음 cycle outline

**Goal:** Task 1 (X) + Task 2 (Y) + Task 3 (Z) 측정 결과 결합 분석 — 3 후보 비교표 (단일 표 + Phase 0.4 §1 강압-적합 회피 의무 준수) + 다음 cycle 단일 결정 (production fix cycle 진입 / 추가 진단 cycle / W 후보 PST 출처 재진입). 단일 후보 식별 시 fix scope outline 작성. 2+ 후보 잔존 시 E3 발동 (추가 진단 cycle). 0 후보 식별 (모두 반증) 시 W 후보 진입 (M6 cycle 재실행).

**Files:**
- Create: `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-synthesis-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task — 종합 보고만).

- [ ] **Step 1: cycle commit 요약**

Run: `git log --oneline -6`

Expected:
```
<4 hash> docs(plans): F-non-prelim synthesis ...
<3 hash> docs(plans): F-non-prelim-3 ...
<2 hash> test(decoder): F-non-prelim-2 ...
<1 hash> test(decoder): F-non-prelim-1 ...
9a5a7f6 docs(plans): F-oct-postfix2-prelim synthesis + cycle decision
118446e docs(plans): F-oct-postfix2-prelim plan
```

- [ ] **Step 2: 3 후보 비교표 (단일 표)**

Task 1~3 측정 결과를 단일 표로 결합 (Phase 0.4 §1 — 측정 데이터만):

| 후보 | 측정 출처 | 결과 | 평가 (반증/유력/부분) | spec § 인용 |
|------|-----------|------|----------------------|--------------|
| **X** (excitation u[0..4] sub-항) | Task 1 §2 + §3 | (Task 1 결과) | (Task 1 결과) | §A.3.5, §4.1.5 |
| **Y** (LP a[0..10] cross-check) | Task 2 §2 + §3 | (Task 2 결과) | (Task 2 결과) | §A.3.2/3, §A.4.1 |
| **Z** (spec 해석 재검토) | Task 3 §3 + §4 + §5 | (Task 3 결과) | (Task 3 결과) | §A.4.2.*, §4.2 |

- [ ] **Step 3: 결정 트리 (단일 식별 / 2+ 잔존 / 0 식별)**

| 시나리오 | 결정 | 다음 cycle 명 |
|----------|------|--------------|
| X 단독 "유력" + Y/Z "반증" | **단일 식별 (X)** — production fix cycle 진입 | F-non-fix (production 1~3 라인 fix in `internal/gain` 또는 `internal/fcb` 또는 `internal/pitch` — Task 1 §3 의 식별 sub-항) |
| Y 단독 "유력" + X/Z "반증" | **단일 식별 (Y)** — production fix cycle 진입 | F-non-fix-Y (production fix in `internal/synth` LSP→a[] 변환) |
| Z 단독 "유력" + X/Y "반증" | **단일 식별 (Z)** — spec 정합 production fix cycle | F-non-fix-Z (식별 § 의 production 정합화) |
| 2+ "유력" 또는 "부분" 잔존 | **E3 발동** — 추가 진단 cycle | F-non-prelim-2 (가칭 — 잔존 후보별 분리 측정 plan) |
| X/Y/Z 모두 "반증" | **W 후보 진입** (M6 cycle 재실행) | F-non-W (M6 byte-level + 9 vector 분포 재실행 + ITU 외부 별첨 verbatim grep) — Phase 0.4 §6 의무 (W 강압적 진입 금지 — *모두 반증* 조건만 진입) |

본 §3 결정은 *측정 데이터에 의해 자동 결정* — 임의 선택 금지 (Phase 0.4 §1).

- [ ] **Step 4: 다음 cycle 권고 outline (식별 후보별)**

식별된 후보에 따라:

| 식별 후보 | 다음 cycle 명 | scope outline | 예상 fix 라인 수 |
|-----------|--------------|---------------|------------------|
| X (g_p · v 단독) | F-non-fix-X-pitch | adaptive codebook gain 또는 v[n] 부호 fix | 1~3 |
| X (g_c · c 단독) | F-non-fix-X-fcb | fcb code c[n] 또는 g_c 부호 fix | 1~3 |
| X (hybrid g_p+g_c) | F-non-prelim-X-split (추가 진단) | g_p / g_c sub-항 단독 분리 측정 | 0 (진단) |
| Y | F-non-fix-Y-lp | LSP→a[] 변환 부호 fix | 1~5 |
| Z | F-non-fix-Z-spec | 식별 § 의 production 정합화 | 1~10 |
| 모두 반증 (W) | F-non-W (재진단) | M6 byte-level + ITU 별첨 + cross-tree byte 비교 | 0 (진단) |

- [ ] **Step 5: 잔여 보류 항목 갱신**

F-oct-postfix2-prelim synthesis (`9a5a7f6`) §5 사용자 게이트 G-N1~G-N5 의 본 cycle 결과 반영 + 신규 보류 항목 추가:

| # | 항목 | 본 cycle 갱신 |
|---|------|---------------|
| 1 | F-non-prelim cycle 자체 | 본 cycle 종결 |
| 2 | 3 후보 단일 식별 | (Step 3 결과) |
| 3 | `stagef_bis_diagnostic_test.go` untracked | 보존 유지 (변경 0) — Phase 0.5 충족 |
| 4 | 다음 fix cycle 의 RED gate | 항목 16 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 승계 |
| 5 | 후보 W (PST 출처 의문) 재진입 | (Step 3 결과 — 모두 반증 시에만 진입) |
| 6 | F-oct-postfix2-prelim §5 G-N1~G-N5 결정 | 본 cycle 진입 정합 (G-N1 = "(a) X 우선" 채택) |

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-synthesis-report.md`:

```markdown
# Phase 1k Stage F-non-prelim 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-03
**범위**: F-non-prelim-1/2/3 결합 분석 + 3 후보 (X/Y/Z) 비교 + 다음 cycle 단일 결정.
**산출물**: cycle 결산 + 3 후보 비교표 + 결정 트리 + 다음 cycle plan outline + 잔여 보류 항목 + Phase 1k 종결 평가.
**준수**: Phase 0.4 강압-적합 회피 (측정 데이터만), 사용자 G1 결정 (Annex A binary 거부), production 변경 0 라인 (메타 task), 사전 보유 working tree 보존.

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)
## 1. F-non-prelim cycle commit 요약 + cycle premise (G-N1 = X 우선)
## 2. 3 후보 비교표 (단일 표, 측정 데이터만)
## 3. 단일 식별 결정 (또는 E3 발동 / W 진입)
## 4. 다음 cycle 권고 (production fix / 추가 진단 / W 재진단)
## 5. 잔여 보류 항목 갱신 + 사용자 게이트
## 6. Phase 1k 종결 평가
```

```bash
git add docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-synthesis-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-non-prelim synthesis + next cycle decision

F-non-prelim cycle (Task 1 X excitation sub-term, Task 2 Y LP a[]
cross-check, Task 3 Z spec interpretation review) 의 결합 분석 + 3 후보
비교표 + 다음 cycle 단일 결정. F-oct-postfix2-prelim 종합 (9a5a7f6) §3
의 후보 X 우선 채택 (사용자 G-N1) 후 측정 데이터만으로 단일 식별 또는
E3 발동 또는 W 후보 (PST 출처) 재진입 결정.

production 변경 0 (메타 task). 외부 G.729 0 참조 (G1 결정 정합).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

### 1. Spec coverage

F-oct-postfix2-prelim synthesis (`9a5a7f6`) §3 후보 X 우선 + §4 다음 cycle outline + §5 사용자 게이트 G-N1~G-N5 결정 + 본 plan task 매핑:

- 후보 X 우선 (HIGH priority) → Task 1 (X 측정) + Task 4 §3 결정 트리 X 단독 시나리오.
- 후보 Y 보조 (MEDIUM) → Task 2 (Y cross-check).
- 후보 Z 보조 (LOW-MEDIUM, 비용 LOW) → Task 3 (보고서 only).
- 후보 W (LOW, M6 모순) → Task 4 §3 결정 트리의 "모두 반증 시에만 진입" 조건 + Phase 0.4 §6 의무.
- 사용자 G-N1 (a) X 우선 → 본 plan 진입 premise + Task 1 우선순위.
- 사용자 G-N2 (a) sub-항 분리 측정 승인 → Task 1 §3 sub-항 4개 분리 dump.
- 사용자 G-N3 (a) spec 영역 §A.3.* + §4.1.5 확장 → 본 plan §"Spec § 인용" + Task 1/2 §0 spec § 인용.
- 사용자 G-N4 (a) bis 보존 유지 → Phase 0.5 의무.
- 사용자 G-N5 (a) G1 spec scope 한계 인정 → 본 plan 의 spec 영역 (§A.3.* + §4.1.5) 확장으로 갈음.

5 항목 모두 매핑 완료. 누락 0.

### 2. Placeholder scan

- 각 task 의 Step N 보고서 outline 에 *각 § 명시* (placeholder 없이).
- Task 1 의 Step 4 에 *test outline 코드* 제시 (signature, 측정 점, 측정 출력 형식).
- Task 2/3 의 측정 함수 / 보고서 outline 은 *완전한 측정 출력 형식* + spec § 인용 + 후보 평가 표 제시. 구체 코드 윤곽 + 측정 결과는 task 실행 시점 결정 (dump 값은 측정에 의해 도출되므로 placeholder 가 아님 — 보고서에 "(Task N 결과)" 로 명시).
- 각 commit 메시지 *완전한 한국어 본문* + co-author trailer.

placeholder 0 확인.

### 3. Type consistency

- Task 1 신규 test `TestDiagnostic_FnonPrelimXExcitationSubterms`: helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0. gain.Decoder.Decode / fcb.Decode / pitch adaptive 호출은 production exported API.
- Task 2 측정 함수 `TestDiagnostic_FnonPrelimYLPCrossCheck`: F-sept-2 reference 호출 함수 또는 동일 reference 데이터 재사용 — helper 신규 0.
- Task 3 = 보고서 only — type 변경 0.
- 회귀 게이트 16 항목의 test 이름 Phase 0.3 ↔ 각 task Step 에서 일관.
- production 변경 0 라인 의무 (E5 + Phase 0.4 §4) — 본 cycle 4 task 모두 test/docs 변경만.

type consistency clean.

### 4. Spec § 인용 정합성 (특별 검토)

본 plan 의 spec 인용은 PDF `pdftotext -layout` verbatim. 각 task 진입 시 Step 2 등에서 grep 재확인 의무 명시. F-oct-prelim-5-4 §3.6 의 "g_l > 0" 결합 해석 또는 F-oct-postfix-2 의 "γ_t 분기 strict reading" 결합 해석은 본 cycle 의 spec 인용으로 사용하지 *않는다* (Phase 0.4 §2). X (§A.3.5 + §4.1.5) / Y (§A.3.2/3 + §A.4.1) / Z (§A.4.2.* + §4.2 + §4.3 Table 9) 의 인용 출처가 후보별 분리 — 결합 해석 위험 0.

### 5. 사용자 G1 결정 정합성 특별 검토

G1 (c) = "Annex A binary 거부 + 후보 ③ pivot". 본 plan:

- Phase 0.2 E4: 외부 G.729 구현 (Annex A binary 포함) 0건 인용.
- Phase 0.4 §5: g_l 영속화 (후보 ①) 관련 측정 / fix 도입 금지.
- Task 1~4 모든 측정: PDF + READMETV.txt + repo committed PST 파일만 사용 (Annex A binary trace 의 ground-truth 대체 불가 — 측정 한계는 보고서 §0 명시).
- 본 cycle 은 후보 ③ 의 spec scope 확장 (§A.3.* + §4.1.5) — 사용자 G-N5 (a) "G1 spec scope 한계 인정 + 다음 cycle 진입" 정합.

G1 결정 정합 100%.

### 6. 회귀 게이트 16 항목 정합성

Phase 0.3 의 16 항목:
- F-quart 2 (항목 1, 2)
- F-sext 1 (항목 3)
- F-sept 3 (항목 4, 5, 6)
- F-oct-prelim 3 (항목 7, 8, 9)
- F-oct-prelim-5 4 (항목 10, 11, 12, 13)
- ITU contract 1 (항목 14)
- F-oct-postfix2-prelim 1 (항목 15 — 4 측정 함수 묶음)
- F-oct-postfix-1 RED 1 (항목 16)

합계 = 2+1+3+3+4+1+1+1 = **16**. 사용자 task 명세 ("누적 contract test 16건 (15 + Task 1 신규 baseline)") 와 정합.

> 본 cycle Task 1 의 신규 측정 (`TestDiagnostic_FnonPrelimXExcitationSubterms`) 자체는 회귀 게이트 17번째 항목으로 *자동 promotion 하지 않는다* (Phase 0.3 자동 promotion 금지). 항목 15 의 "신규 baseline" 은 직전 cycle (F-oct-postfix2-prelim) 에서 만들어진 4 measurement test 묶음을 가리킴.

### 7. Phase 0.4 강압-적합 회피 의무 (본 cycle 특별 강조)

본 cycle 은 *직전 cycle 4 가설 모두 반증* 후의 후속 진단. 강압-적합 위험이 가장 높음 (X 가 "가장 그럴듯" 하므로 sub-항 측정 결과를 X 채택 방향으로 해석할 유혹). 회피 의무 6 항목 (Phase 0.4 §1-6) 각 task Step 에 명시 — 특히:

- Task 1 §5 의 sub-항 평가표가 4 시나리오 (g_p·v 단독 / g_c·c 단독 / hybrid / replication 결함) 모두 명시 — "g_p·v 단독" 강압적 채택 회피.
- Task 4 §3 결정 트리가 5 시나리오 (X 단독 / Y 단독 / Z 단독 / 2+ 잔존 / 모두 반증) 모두 명시 — X 우선순위 HIGH 가 *측정 결과를 좌우하지 않음*.
- Task 4 §3 W 시나리오는 "X/Y/Z 모두 반증" 조건만 진입 — Phase 0.4 §6 W 강압적 진입 금지 정합.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, F-oct-postfix2-prelim 패턴 답습. 각 task 완료 후 main agent 가 다음 task 진입 권고를 사용자에게 게이트.

**2. Inline Execution** — batch execution. Task 1 X sub-항 측정 → 사용자 게이트 → Task 2~3 Y/Z 측정 → 사용자 게이트 → Task 4 종합.

**Recommended user gate before Task F-non-prelim-1 dispatch**: 사용자가 본 plan 의 Phase 0.4 §1 (sub-항별 분리 측정 의무) + §2 (spec § 결합 해석 금지) + §6 (W 강압적 진입 금지) + Phase 0.5 (bis test 보존) + Phase 0.3 회귀 게이트 16 항목 (특히 항목 16 RED 잔존 의무) 을 검토 후 진입 승인.

**Which approach?**
