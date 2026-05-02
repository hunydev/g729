# Phase 1k Stage F-non-Cgamma-revisit — Plan

**Cycle ID**: `F-non-Cgamma-revisit` (Phase 1k 9번째 cycle)
**작성일**: 2026-05-04
**선행 cycle**: `F-non-prelim-X-split` (synthesis commit `aa9dcf9`, 권고 = (R1) Cγ 재진입 단독 cycle, 3 task)
**사용자 승인**: G-XS2 = "(A) Cγ 재진입"
**선행 plan 양식**: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-plan.md` (commit `49fac32`)

---

## Phase 0 — Context, Invariant, Cumulative Catalog

### 0.1 직전 cycle 정리 (F-non-prelim-X-split)

3 task 완수, 결함 0건:

| task | commit | 측정 대상 | 결과 verdict |
|------|--------|-----------|--------------|
| F-non-prelim-X-split-1 | `fd0b381` | Cα = fcb pulse trace (sample 5..7 한정 §3.8) | spec 정합 — 4 pulse position/sign 모두 reference 일치, c[5..7] 부호 = `[+1,+1,+1]` |
| F-non-prelim-X-split-2 | `4cd25e1` | Cβ = gain g_c trace (sample 5..7 한정 §3.9) | spec 정합 — γ_ga/γ_gb sum, predicted gain, g_c product 모두 reference 일치, g_c=+4153 |
| F-non-prelim-X-split-3 | `aa9dcf9` | synthesis (cumulative + Phase 1k 종결 평가) | (R1) Cγ 재진입 권고 |

**Cα 폐기** (sample 5..7 fcb 입력 정합), **Cβ 폐기** (sample 5..7 gain 정합) → 잔여 sub-hypothesis = **Cγ (postfilter / synthesis IIR memory / Y magnitude 영향)**.

### 0.2 8 cycle 누적 verdict (Phase 1k 1번째 ~ 8번째)

| cycle | sub-hypothesis | verdict | 폐기 sub-stage |
|-------|---------------|---------|----------------|
| F-oct-postfix-1 | M1 (LP→postfilter chain bias) | spec-측정 RED 잔존 (단 mechanism = 외부 미식별) | — |
| F-oct-postfix-2 | M2 (postfilter coefficient quantization) | spec 정합 폐기 | M2 |
| F-oct-postfix2-prelim-1..6 | M1', M3, M5, M6, Cα, Cβ + Z (PST chain) + Y (a[0..10] sign) | 전부 spec 정합 폐기 | M1', M3, M5, M6, Cα, Cβ, Z |
| F-non-prelim-1..2 | Y (a[0..10] sign-equal 11/11), forced (-u) → syn(-u)=-syn(+u) (linear) | spec 정합 (sign), Y magnitude max\|Δ\|=6 잔존 | Y(sign), forced-flip linearity |
| F-non-prelim-X-split-1..2 | Cα/Cβ sample 5..7 한정 재측정 | spec 정합 | Cα(s5..7), Cβ(s5..7) |

**누적 결함 0건** (8 cycle 14 sub-hypothesis 폐기). **잔여 후보 공간**:
- (Cγ-postfilter) §4.2 postfilter sub-stage (long-term / short-term / tilt / AGC+HP) sample 5..7 한정 미측정.
- (Cγ-synth) §4.1.6 synthesis IIR memory sample 5..7 한정 pre/post 미측정.
- (Cγ-Y-mag) Y magnitude (a[1..10] +6 변화) → syn[5..7] 부호 변화 미측정 (F-non-prelim-2 forced-flip 은 large perturbation only).

### 0.3 ALGTHM frame 0 sf0 누적 측정값

| 항목 | 값 | 출처 cycle |
|------|------|-----------|
| g_p (Q14) | +1995 | F-non-prelim-X-split-2 |
| g_c (Q1) | +4153 | F-non-prelim-X-split-2 |
| v[0..4] (adaptive cb) | 0 | F-non-prelim-1 |
| c[0..3] (fcb pulse, Q13) | +8192 each | F-non-prelim-X-split-1 |
| u[0..3] (excitation) | +1 each | F-non-prelim-1 |
| u[4..7] | 0 | F-non-prelim-1 |
| syn[5..7] (synthesis output) | `[+1, +1, +1]` | F-non-prelim-1 |
| sPf[5..7] (postfiltered) | `[+1, +1, +1]` | F-oct-postfix2-prelim-4 |
| post-HP (PST) | `[+2, +2, +2]` | F-oct-postfix2-prelim-4 |
| **want (ALGTHM.PST)** | `[−1, −1, −1]` | F-oct-postfix-1 |
| a[0..10] sign vs reference | 11/11 일치 | F-non-prelim-1 |
| a[1..10] magnitude max\|Δ\| | 6 (F-sept-2 L3 잔존) | F-sept-2 |
| forced (-u) → syn 부호 | 완전 반전 (linearity 입증) | F-non-prelim-2 |
| Z (PST chain) | post-AGC + post-HP + post-×2 (§4.2 + §A.4.2.5) | F-oct-postfix2-prelim-6 |

### 0.4 Invariant E1-E5 재확인 (escape hatch + 강압-적합 회피)

- **E1**: 외부 G.729 구현 0건 참조. ITU-T G.729 (06/2012) PDF + READMETV.txt 만 spec source. **Annex A binary 사용 금지** (G1 결정).
- **E2**: production 변경 0 라인 (측정 only). 본 cycle 3 task 모두 진단 test 추가만.
- **E3**: F-oct-postfix-1 RED (항목 17) 영구 잔존 — fix 시점 = mechanism 식별 후 별도 cycle.
- **E4**: 측정값과 spec 비교 시 **PDF verbatim 인용 의무**. 우리 결과를 정당화하는 paragraph 만 cherry-pick 금지. spec 미측정 sub-stage 식별 시 §번호 + 인용 line 명시.
- **E5**: 자동 promotion 0 — 측정-only test 는 회귀 게이트 자동 등재 금지. synthesis (Task 3) 의 명시 결정 후 promotion.

**강압-적합 회피 절차** (Phase 0.4):
1. spec 미측정 sub-stage 식별 시 § number + PDF page 인용 (예: "§4.2.4 tilt compensation, p.27 line N").
2. 측정 결과가 spec 와 mismatch 일 때 = production bug 후보. mismatch 가 spec scope 밖일 때 = sub-stage 폐기 (Cγ-X 폐기).
3. **금지**: "거의 정합" / "범위 내 변동" / "magnitude tolerance 내" 같은 모호한 verdict. 모든 verdict = `EQ` / `NE` 이진.
4. **금지**: spec 가 모호한 부분 (예: rounding 방향 미명시) 을 우리 구현 정당화로 사용. 모호 지점 = 별도 sub-hypothesis 로 분리.

### 0.5 누적 contract test gate (19건)

| # | gate | 상태 | 출처 |
|---|------|------|------|
| 1..16 | (Phase 1a~1j 누적 16건; F-non-prelim-X-split plan §0.5 참조) | PASS | 누적 |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 | **RED 잔존** | F-oct-postfix-1 |
| 18 | F-non-prelim-X-split measurement bundle (Cα fcb + Cβ gain) | PASS | F-non-prelim-X-split (commit `aa9dcf9` 시점 promotion) |
| 19 | F-non-Cgamma-revisit measurement bundle (G-1 + G-2) | **pending** (Task 3 synthesis 결정 후 promotion 여부 판단; 자동 promotion 금지 — E5) | 본 cycle |

회귀 게이트 commit 직후 검증:
- `go vet ./...` clean.
- 누적 18 gate PASS/FAIL dump (1..16 + 18 PASS, 17 RED 잔존).
- 19번 = pending (Task 3 verdict 에 따라 promote 또는 폐기).

---

## Phase 1 — Hypothesis Tree (Cγ 재진입)

```
Cγ (sample 5..7 잔여 mechanism 후보)
├── G-1 (postfilter sub-stage 미측정)
│   ├── §4.2.1 long-term postfilter
│   ├── §4.2.2 short-term postfilter
│   ├── §4.2.4 tilt compensation
│   └── §4.2.5 adaptive gain control + HP filter
├── G-2 (synthesis IIR memory + Y magnitude 통합)
│   ├── §4.1.6 IIR memory pre/post sample 5..7
│   └── Y magnitude small perturbation (a[1..10] +6 → syn 부호 변화 측정)
└── Synthesis (Task 3) — 3-시나리오 결정 트리
    ├── (Cγ-postfilter) G-1 mismatch 식별 → F-non-fix-postfilter 진입 (best 2cy 종결)
    ├── (Cγ-synth) G-2 mismatch 식별 → F-non-fix-synth 진입 (best 2cy 종결)
    └── (Cγ-refute) G-1 + G-2 모두 spec 정합 → spec-외부 mechanism 0 입증 → Phase 1k 잠정 종결 + alternative path 진입
```

**기대 entropy** (사전):
- (Cγ-postfilter) ≈ 40% — postfilter sub-stage 은 가장 미측정 영역.
- (Cγ-synth) ≈ 20% — IIR memory sample 5..7 차이는 작을 가능성 (linearity 입증됨).
- (Cγ-refute) ≈ 40% — 8 cycle 누적 결함 0건 base rate.

---

## Phase 2 — Task 분해 (3 task, TDD)

### Task 1: F-non-Cgamma-revisit-1 (G-1 postfilter sample 5..7 sub-stage)

**목적**: §4.2 postfilter 4 sub-stage (long-term / short-term / tilt / AGC+HP) 의 sample 5..7 한정 출력 측정 + 분기 결정.

**선행 측정 보완**: F-oct-postfix2-prelim-4 (commit `f04ec88`) — 당시 sPf[5..7] 만 측정 (4 sub-stage 분리 측정 부재). 본 task = sub-stage 별 분리.

**TDD 절차**:
1. **RED**: `internal/postfilter/stagef_fnoncgamma_revisit_diagnostic_test.go` 신규 — `TestDiagnostic_FnonCgammaRevisit1PostfilterSubStageTrace`.
   - frame 0 sf0 sample 5..7 한정.
   - 4 sub-stage 출력:
     - (a) long-term postfilter 출력 `lt[5..7]` (§4.2.1).
     - (b) short-term postfilter 출력 `st[5..7]` (§4.2.2).
     - (c) tilt compensation 출력 `tc[5..7]` (§4.2.4).
     - (d) AGC + HP 출력 `agc[5..7]`, `hp[5..7]` (§4.2.5).
   - 각 sub-stage 부호 + magnitude dump.
   - reference comparison (READMETV.txt ALGTHM 또는 PDF spec verbatim) — sub-stage 별 expected sign / magnitude bound.
   - sub-stage classifier `classifyCgammaPostfilterSubStage()` — `EQ`/`NE` 이진 verdict.
2. **GREEN**: production 변경 0 (E2). test = 측정 only.
3. **dump 확인**: 4 sub-stage 출력 + verdict.
4. **commit**: `test(postfilter): add Stage F-non-Cgamma-revisit-1 G-1 postfilter sub-stage trace` + Co-authored-by.

**측정 의무** (1줄): §4.2.1/2/4/5 4 sub-stage 의 sample 5..7 한정 출력 4-tuple + sub-stage 별 spec 정합 EQ/NE 판정.

**polarity expectation** (PDF 인용):
- §4.2 PST chain = positive polarity preserve (tilt + AGC scalar gain only).
- 따라서 syn[5..7]=`[+1,+1,+1]` → 모든 sub-stage 출력 부호 = `+`. 어느 sub-stage 에서 `−` 가 나오면 mechanism 후보 식별.

**escape hatch**: 4 sub-stage 모두 부호 = `+` (즉 mismatch 없음) 인 경우 = G-1 폐기 → Task 2 의 G-2 만 mechanism 후보. plan abandon 없이 Task 2 진행.

---

### Task 2: F-non-Cgamma-revisit-2 (G-2 synth IIR memory + Y magnitude)

**목적**: §4.1.6 synthesis IIR memory pre/post sample 5..7 + Y magnitude small perturbation (a[1..10] +6 변화 시 syn[5..7] 부호 변화) 통합 측정.

**선행 측정 보완**: F-non-prelim-2 (commit `d1a4f2d`) — forced (-u) large perturbation 만 측정 (linearity 입증). 본 task = small magnitude perturbation (Y magnitude max\|Δ\|=6 한정 변화 시 syn 부호 변화).

**TDD 절차**:
1. **RED**: `internal/synth/stagef_fnoncgamma_revisit_diagnostic_test.go` 신규 — 2 sub-test:
   - **Sub-test A**: `TestDiagnostic_FnonCgammaRevisit2SynthIIRMemoryTrace`
     - frame 0 sf0 sample 5..7 IIR memory pre-state (mem_syn[0..9]) + post-state (각 sample 처리 후) dump.
     - reference: a[0..10] reference coefficient 사용 시 동일 mem_syn 기댓값.
     - verdict: pre-state EQ vs reference, post-state EQ vs reference.
   - **Sub-test B**: `TestDiagnostic_FnonCgammaRevisit2YMagnitudePerturbationTrace`
     - a[1..10] 의 +6 magnitude perturbation 적용 (sign 보존, magnitude only) → syn[5..7] 부호 측정.
     - 비교 baseline = unperturbed syn[5..7] = `[+1,+1,+1]`.
     - verdict: perturbed syn[5..7] 부호 = `[+1,+1,+1]` 면 magnitude perturbation = mechanism 아님 (G-2-Y-mag 폐기). 부호 = `[−1,−1,−1]` 면 magnitude = mechanism 후보 식별.
   - sub-stage classifier `classifyCgammaSynthSubStage()` + `classifyCgammaYMagSubStage()`.
2. **GREEN**: production 변경 0 (E2).
3. **dump 확인**: IIR memory 10 entry + Y magnitude perturbation syn[5..7] 부호.
4. **commit**: `test(synth): add Stage F-non-Cgamma-revisit-2 G-2 synth IIR memory + Y magnitude trace` + Co-authored-by.

**측정 의무** (1줄): §4.1.6 IIR memory pre/post sample 5..7 EQ/NE + Y magnitude +6 perturbation 적용 시 syn[5..7] 부호 EQ/NE.

**polarity expectation**:
- IIR memory pre-state: 직전 sub-frame state, reference 와 EQ 기대.
- IIR memory post-state: a[0..10] sign 정합 (11/11) + g_p/g_c/u 정합 → mem_syn EQ 기대.
- Y magnitude perturbation: linearity (forced-flip 입증) 하에서 small magnitude 변화 → 부호 변화 없을 가능성 ≈ 80% (sign-determined-by-input 가설).

**escape hatch**: sub-test A + B 모두 EQ → G-2 폐기 → Task 3 에서 (Cγ-refute) 시나리오 확정.

---

### Task 3: F-non-Cgamma-revisit-3 (synthesis + 3-시나리오 결정 트리)

**목적**: G-1 + G-2 verdict 결합 + 3-시나리오 결정 트리 + Phase 1k 종결 평가 + alternative path 권고 (음성 시).

**TDD 절차** (분석 + synthesis report only, test 추가 없음):
1. G-1 (Task 1) verdict 4-tuple 인용.
2. G-2 (Task 2) verdict (IIR + Y-mag) 인용.
3. 3-시나리오 결정 트리 적용:

| 시나리오 | 조건 | 다음 cycle | 누적 cycle 추정 (Phase 1k 종결까지) |
|---------|------|-----------|-----------------------------------|
| **(Cγ-postfilter)** | G-1 4 sub-stage 중 ≥1 NE | F-non-fix-postfilter (1 fix cycle) | 본 cycle + fix = 2 cy 종결 (best) |
| **(Cγ-synth)** | G-1 전부 EQ + G-2 (IIR 또는 Y-mag) NE | F-non-fix-synth (1 fix cycle) | 본 cycle + fix = 2 cy 종결 (best) |
| **(Cγ-refute)** | G-1 + G-2 전부 EQ | Phase 1k 잠정 종결 + alternative path | 본 cycle 종결 + alternative path 별도 |

4. **(Cγ-refute) 시 alternative path 권고** (G-XS2 합의 의무):
   - (a) Phase 0c (PCM/IO) 재진입 + want 도메인 재해석 cycle.
   - (b) Phase 1g (decoder integration) 재진입 + multi-frame state 진단.
   - (c) ITU corrigendum / 추가 spec source 검색 (단 G1 = Annex A binary 거부 유지).
5. 누적 19 gate 상태 dump + 본 cycle 측정 bundle promotion 여부 (시나리오 별):
   - (Cγ-postfilter) / (Cγ-synth): 측정 bundle = mechanism 식별 evidence → promotion (item 19 PASS 등재, fix cycle 후).
   - (Cγ-refute): 측정 bundle = sub-hypothesis 폐기 evidence → promotion 검토 (E5 자동 promotion 금지 → 명시 합의 후).
6. **Synthesis report**: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-synthesis-report.md`.
7. **commit**: `docs(plans): F-non-Cgamma-revisit synthesis + Phase 1k decision` + Co-authored-by.

**측정 의무** (1줄): G-1 + G-2 verdict 결합 + 3-시나리오 결정 트리 적용 + Phase 1k 종결 (best 2cy / 잠정 종결) verdict + (음성 시) alternative path 권고.

---

## Phase 3 — 회귀 게이트 (각 commit 직후)

각 task commit 직후 실행:
1. `go vet ./...` — clean 필수.
2. 누적 18 gate dump:
   - 1..16 PASS (변동 없음).
   - 17 RED 잔존 (F-oct-postfix-1, fix scope 외).
   - 18 PASS (F-non-prelim-X-split bundle).
3. 신규 19번 = pending (Task 3 synthesis 후 결정).
4. test 실행 명령:
   - Task 1: `go test ./internal/postfilter/ -run FnonCgammaRevisit1 -v`
   - Task 2: `go test ./internal/synth/ -run FnonCgammaRevisit2 -v`
   - 누적: `go test ./...` (RED 17 잔존 확인).

---

## Phase 4 — Escape hatch E1-E5

| code | 발동 조건 | 행동 |
|------|----------|------|
| E1 | 외부 G.729 구현 참조 유혹 (ITU reference C, Annex A binary, 3rd party fork) | 즉시 차단. spec source = PDF + READMETV.txt only. |
| E2 | production 변경 유혹 (측정 중 fix 욕구) | 즉시 차단. fix = 별도 cycle (F-non-fix-postfilter / F-non-fix-synth). |
| E3 | RED 17 잔존을 task failure 로 간주하려는 유혹 | 차단. 17 = mechanism 식별 후 별도 fix cycle. |
| E4 | spec 모호 paragraph 를 우리 구현 정당화로 cherry-pick 하려는 유혹 | 차단. 모호 지점 = 별도 sub-hypothesis 로 분리. PDF verbatim 인용 + page/line 명시. |
| E5 | 측정-only test 자동 promotion 유혹 (Task 1/2 commit 시 회귀 게이트 등재) | 차단. promotion = Task 3 synthesis 결정 후. |

---

## Phase 5 — Self-review (작성자)

- ✅ Phase 0 (이전 cycle 정리 + invariant + 8 cycle 누적 catalog) 포함.
- ✅ Phase 0.4 강압-적합 회피 절차 (PDF verbatim + cherry-pick 금지).
- ✅ Task 3개 분해 (G-1 / G-2 / synthesis).
- ✅ 각 task TDD (RED→GREEN→commit, production 0).
- ✅ commit 메시지 양식 + Co-authored-by trailer.
- ✅ 회귀 게이트 19건 (18 + 신규 1) 관리.
- ✅ Escape hatch E1-E5.
- ✅ `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 명시.
- ✅ 외부 G.729 구현 0건 참조 (E1).
- ✅ production 변경 0 라인 (E2).

**위험 요소**:
- (R-A) G-1/G-2 모두 EQ → (Cγ-refute) → Phase 1k 잠정 종결. 가능성 ≈ 40% (8 cycle base rate). → alternative path 3개 사전 명시 (Phase 2 Task 3 §4) 로 mitigate.
- (R-B) G-1 sub-stage 분리 측정 시 production 호출 표면 부재 (long-term / short-term / tilt / AGC 의 internal output 접근). → 기존 `internal/postfilter/` 의 export 함수 (`Longterm`, `Shortterm`, `Tilt`, `AGC`, `HP`) 직접 호출 + sample 5..7 한정 slice. 호출 가능 여부 사전 검증 의무 (Task 1 첫 단계).
- (R-C) Y magnitude +6 perturbation 의 reference 부재 (PDF 에 명시 안됨). → baseline = unperturbed syn[5..7]=`[+1,+1,+1]`, perturbation = production 함수 직접 입력 변경 (a[1..10] 만 +6). reference 가 아니라 우리 구현의 sensitivity 측정 (mechanism 식별 evidence).

---

## Phase 6 — Execution Handoff

**다음 dispatch**: `F-non-Cgamma-revisit-1` (Task 1, G-1 postfilter sub-stage trace).

**선행 의무 (dispatch 직전)**:
1. Phase 0.5 19 gate baseline 확인 (`go test ./...` + 17 RED 잔존 확인).
2. `internal/postfilter/` export 표면 점검 (long-term / short-term / tilt / AGC+HP 직접 호출 가능성).
3. Phase 1 hypothesis tree + Phase 2 Task 1 측정 의무 1줄 재확인.

**완료 trigger**: Task 3 synthesis commit + 3-시나리오 verdict 명시 + (Cγ-postfilter / Cγ-synth) → 다음 fix cycle dispatch / (Cγ-refute) → Phase 1k 잠정 종결 + alternative path G-XS 합의.

---

**Plan 종료.** 본 commit = F-non-Cgamma-revisit cycle 0번째 (plan-only) commit. 다음 commit = Task 1 (`test(postfilter): add Stage F-non-Cgamma-revisit-1 G-1 postfilter sub-stage trace`).

---

## Task 진행 status

- [x] Task 1 — F-non-Cgamma-revisit-1 (G-1 postfilter sub-stage trace) — commit `a4120f9`. Verdict: EQ_ALL (4 sub-stage 모두 polarity-preserve). G-1 (Cγ-postfilter) REFUTE.
- [x] Task 2 — F-non-Cgamma-revisit-2 (G-2 synth IIR memory + Y magnitude trace) — Sub-test A verdict: EQ (4 state 모두; pre-sample-5 / post-5 / post-6 / post-7 production==reference). Sub-test B verdict: EQ (perturbed syn[5..7] sign == baseline [+,+,+]). G-2-IIR + G-2-Y-mag 모두 폐기 후보.
- [ ] Task 3 — F-non-Cgamma-revisit-3 (synthesis + 3-시나리오 결정 트리).
