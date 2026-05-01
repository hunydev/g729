# Phase 1k Stage F-oct-postfix 종합 보고서 — E3 발동 cycle 결산 + 다음 cycle 결정

**작성일**: 2026-05-01
**범위**: Task F-oct-postfix-1 (RED) + Task F-oct-postfix-2 (γ_t 분기 fix 시도 → E3 발동) 결합 분석. plan §398 "E3 발동 시 Task 3/4 skip + Task 5 직진" protocol 적용. Task 3/4 미실행.
**산출물**: 본 보고서 1건 + F-oct-postfix-2 보고서 commit 합본. production 변경 0 라인.
**준수**: spec §A.4.2.3 strict reading + Phase 0.4 강압-적합 회피 + plan §69 E3 protocol + E5 production-clean.
**결론 미리**: G3 (γ_t 선택 분기 단독 결함) 가설 **반증**. 다음 cycle = **후보 ② Annex A binary 행동 추적** 단일 권고 (근거 §3, §4).

---

## §0 Working tree 상태 + escape hatch 평가표 (E1-E5)

### 0.1 working tree pre/post

**진입 시점** (본 task 진입):
```
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md
?? internal/decoder/stagef_bis_diagnostic_test.go
56caa72 (HEAD -> main) test(decoder): add Stage F-oct-postfix-1 ALGTHM sf0 sample 5..7 regression (RED)
```

**완료 시점** (본 보고서 commit 직후):
```
?? internal/decoder/stagef_bis_diagnostic_test.go
<new HEAD> docs(plans): F-oct-postfix synthesis + cycle decision (E3)
56caa72 test(decoder): add Stage F-oct-postfix-1 ALGTHM sf0 sample 5..7 regression (RED)
```

production 변경 0 라인 (E5 invariant 충족). `internal/postfilter/tilt.go` = `56caa72` HEAD 와 byte 동상 (postfix-2 의 fix 시도 후 `git checkout -- internal/postfilter/tilt.go` 로 revert 됨; postfix-2-report §3 raw 출력으로 입증). 사전 보유 untracked diagnostic (`stagef_bis_diagnostic_test.go`) 변경 0 (보존 유지).

### 0.2 escape hatch 종합 평가

| Hatch | trigger 조건 (plan) | 본 cycle 발동 여부 | 근거 |
|-------|---------------------|----------------------|------|
| **E1** | Phase 1i sample 0 invariant 회귀 (`TestDecode_Frame0Sample0_MatchesALGTHM` FAIL) | ❌ 미발동 | 본 cycle 종료 시점 PASS 재확인 (본 보고서 commit 직전 sanity gate). |
| **E2** | spec § 인용이 PDF verbatim grep 결과와 불일치 | ❌ 미발동 | postfix-1 Step 1 grep 재확인으로 strict reading (k1' 부호) 채택 — plan §136 strict PDF reading 정합. |
| **E3** | Task 2 fix 후에도 `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` RED 잔존 | ✅ **발동** | postfix-2-report §3: pre-fix `got=[+2,+2,+2]` ↔ post-fix `got=[+2,+2,+2]` Δ=0. 분기 flip 이 sample 5..7 부호/크기 무영향 입증. |
| **E4** | 외부 G.729 구현 참조 발견 | ❌ 미발동 | postfix-1/postfix-2 양 cycle 모두 ITU-T G.729 (06/2012) PDF + READMETV.txt 만 인용. |
| **E5** | production 변경 라인 > 0 (메타 task 의무) | ❌ 미발동 | 본 task 는 보고서 only. tilt.go = HEAD 동상 입증 §0.1. |

E3 단독 발동. Task 3/4 skip + Task 5 직진 (plan §398 protocol).

---

## §1 Cycle premise (G3 식별 → 1 라인 fix scope) ↔ 실측 괴리 정량

### 1.1 cycle 진입 premise (F-oct-prelim-5-4 §3.6 + plan §134-§144)

F-oct-prelim-5-4 §3.6 표 row C3: γ_t 선택 분기 (`tilt.go:65-68`) 가 spec §A.4.2.3 voicing-dependent rule 의 **단독 결손 분기**로 식별. cover 가설 = G3 (postfilter conditional 분기 cover 결손, M1 단일 후보).

plan §134-§144 의 strict reading 채택으로 fix scope 최소화:
- 분기 조건 1 라인: `pf.agcGainPrev == 0` → `k1 >= 0` (`tilt.go:67`).
- docstring 6 라인 (Phase 1g proxy 자기 인정 제거 + spec §A.4.2.3 정합 인용).
- signature 변경 0 — 호출부 (`postfilter.go:44`) 변경 0.
- 예측 효과 (plan §518 표): ALGTHM frame 0 sf0 sample 5..7 부호 `[+1,+1,+1]` (pre-fix) → `[-1,-1,-1]` (post-fix, spec want 정합).

### 1.2 spec verbatim ground-truth (Phase 0.4 의무)

ITU-T G.729 (06/2012) §A.4.2.3 (PDF p.43, `pdftotext -layout`) verbatim:

> *"The value of γt = 0.8 is used if k1′ < 0 and γt is set to zero if k1′ ≥ 0. The gain factor gt which is used in clause 4.2.3 is eliminated."*

(plan §88-§104 인용 1 동상.) 분기 조건의 ground-truth = **k1' 의 부호** 단독. `g_l` 또는 voicing 상태와의 명시적 결합은 §A.4.2.3 본문에 부재.

### 1.3 실측 괴리 정량 (postfix-2-report §2-§3 인용)

| 측정 항목 | plan 예측 (§518 표) | 실측 (postfix-2-report §3 raw) | 괴리 정량 |
|-----------|----------------------|---------------------------------|------------|
| pre-fix sample 5 부호 | `+1` | `+2` (want=-1, Δ=3) | 크기 1 차이; 부호 일치 (+) |
| pre-fix sample 6 부호 | `+1` | `+2` (want=-1, Δ=3) | 크기 1 차이; 부호 일치 |
| pre-fix sample 7 부호 | `+1` | `+2` (want=-1, Δ=3) | 크기 1 차이; 부호 일치 |
| post-fix sample 5 부호 | `-1` (정합) | `+2` (want=-1, Δ=3) | **부호 반대 + 크기 무변화** |
| post-fix sample 6 부호 | `-1` (정합) | `+2` (want=-1, Δ=3) | **부호 반대 + 크기 무변화** |
| post-fix sample 7 부호 | `-1` (정합) | `+2` (want=-1, Δ=3) | **부호 반대 + 크기 무변화** |
| pre→post Δ (실측) | `±2` 부호 전환 | `Δ=0` (전 sample) | **분기 flip 영향 = 0** |

→ **G3 가설 (γ_t 선택 분기 단독 결함) 반증**. 분기 조건을 spec §A.4.2.3 strict reading 정합으로 flip 했음에도 ALGTHM frame 0 sf0 sample 5..7 의 부호/크기 모두 무변화. 결함 위치는 γ_t 선택 분기 *외부*.

---

## §2 G3 가설 반증의 함의 — F-oct-prelim-5-4 §3.6 결정 재평가 의무

### 2.1 F-oct-prelim-5-4 §3.6 의 spec 인용 vs PDF 원문

F-oct-prelim-5-4 §3.6 row C3 spec 인용 verbatim (`prelim-5-4-report.md:190`):

> *"γ_t = 0.9 if long-term postfilter active (g_l > 0), else 0.2 (tilt comp voicing-dependent gain, §A.4.2.3)"*

이 인용은 plan §138 의 검증 결과 **PDF §A.4.2.3 본문에 직접 등장하지 않음**. F-oct-prelim-5-4 보고서가 §A.4.2.2 (long-term postfilter `g_l = clamp(R(T)/E(T), 0, γ_l)`) 와 §A.4.2.3 (γ_t 분기) 를 *해석적으로 결합* 한 것으로 추정 (plan §138). 본 cycle 은 strict reading (k1' 부호) 을 채택하고 §A.4.2.3 해석을 fix 의 ground-truth 에서 배제 (plan §140).

### 2.2 strict reading 채택의 측정 기반 근거 재검토

plan §140 의 strict reading 채택 결정은 다음 근거의 *조합* 이었다:

| 근거 | 종류 | 본 cycle 측정 후 평가 |
|------|------|------------------------|
| (A) PDF §A.4.2.3 verbatim 원문 = "k1' 의 부호" 단독 | spec 인용 | 유지 (PDF 원문 무변화) |
| (B) F-oct-prelim-5-4 §3.6 의 g_l-based 인용은 §A.4.2.2 결합 해석 | spec 해석 추정 | 유지 (반증 측정 없음) |
| (C) strict reading 으로 fix 시 sample 5..7 부호가 spec want `-1` 로 전환 | 미측정 가설 (plan 예측치) | **반증** — postfix-2-report §3 측정으로 무변화 입증 |

**근거 (C) 의 반증 함의**: strict reading 의 *정확성* 자체는 (A) 로 유지되나, *fix 효과의 충분성* 가설은 본 cycle 측정으로 반증. 즉 spec §A.4.2.3 strict reading 정합 fix 가 **결함 해소를 보장하지 않는다** — 결함의 실제 위치가 γ_t 선택 분기 외부에 있기 때문.

### 2.3 F-oct-prelim-5-4 §3.6 결정 재평가 — strict reading 채택은 *유지*, 충분성 가설은 *철회*

본 cycle 은 다음을 명시한다 (Phase 0.4 강압-적합 회피 의무):

1. F-oct-prelim-5-4 §3.6 row C3 의 spec 인용 ("g_l > 0") 의 §A.4.2.3 출처 부재 사실은 본 cycle plan §138 에서 *spec 인용 확인 부재* 로 정량 입증.
2. strict reading (k1' 부호) 채택은 PDF 원문 (인용 1) 으로 유지 — 본 cycle 측정으로 *반증 없음*.
3. 단, F-oct-prelim-5-4 §6 결정 (a) 의 **암묵적 가정** = "γ_t 분기 fix 가 ALGTHM sample 5..7 부호 결함의 *충분 조건*" 은 본 cycle 로 **반증**. 결함 cover 모델은 G3 단독 → G3 + Gx (잔여 결함) 다중 cover 로 갱신 의무.
4. M1 (postfilter conditional 분기) 단독 채택 결정 (`prelim-5-4-report.md:209`) 도 *부분적 반증* — γ_t 분기는 cover 결손이지만 sample 5..7 결함의 *주요* 또는 *유일* 원인이 아니다.

### 2.4 잔여 결함 위치 후보 영역 (측정 기반)

postfix-2-report §3 raw 출력에서 pre-fix `got=2`, post-fix `got=2` (Δ=0) 의 무변화는 다음 함의를 가진다:

- γ_t 값 선택이 sample 5..7 출력 (16-bit signed, scale `±32767`) 의 LSB 단위 (`Δ=2-(-1)=3`) 에 영향 없음 → tilt μ Q15 의 곱셈 결과가 이 sample 단위에서 *지배적 항이 아님*.
- 부호 자체가 `+` (got=2) ↔ spec want `-` (want=-1) 이 무변화 → **부호 결정 항 (sign-determining term)** 이 tilt compensation 외부에 위치.
- 후보 위치: (i) 합성 LP filter 출력 (synth IIR / `internal/synth`), (ii) long-term postfilter (`internal/postfilter/longterm.go`) 의 g_l 적용 단계, (iii) AGC (`internal/postfilter` agcGainPrev update path), (iv) high-pass filter (post-AGC, `internal/postfilter/hpfilter`).

각 후보의 우선순위는 §3 후보 비교에서 다룬다.

---

## §3 다음 cycle 후보 3건 비교 평가표

postfix-2-report §5 의 3 후보를 *측정 데이터* (postfix-2 실측 + F-oct-prelim 1-cycle / 5-cycle 누적 측정) 만으로 비교 — Phase 0.4 §1 강압-적합 회피 의무 준수 (임의 선호 금지).

### 3.1 후보 명세

| ID | 후보명 (postfix-2-report §5 ID) | 핵심 의무 | scope (production 변경 추정) |
|----|----------------------------------|-----------|--------------------------------|
| ① | g_l 영속화 cycle (longterm.go state field 추가 + tilt.go read) | F-oct-prelim-5-4 §3.6 의 g_l-based 분기 가설을 측정 검증. plan §146 의 보조 옵션 정합. | state 1 field + write 1 라인 + read 1 라인 = 3 라인 |
| ② | Annex A binary 행동 추적 cycle | ITU 참조 binary 의 frame 0 sf0 sample 5..7 중간 단계 (μ, γ_t, k1', s_st, s_tilt, g_l, agcGain) trace 측정. plan §69 E3 phrasing "Phase 1l 또는 Annex A binary 행동 추적 cycle" 정합. E4 invariant 와 충돌 가능성 검토 의무. | production 0 라인 (측정-only) |
| ③ | Stage F-sept/F-oct-prelim 회귀 재진단 (M1 외 후보 재진입) | F-oct-prelim-5-4 §6 결정 (a) 의 G3 단독 가설이 본 cycle 로 반증된 후, F-sept/F-oct-prelim 의 잔여 측정 데이터를 G3+Gx 다중 cover 가설로 재해석. 측정 데이터 재발굴. | production 0 라인 (재해석-only) |

### 3.2 비교 평가표 (4 차원)

| 차원 | ① g_l 영속화 | ② Annex A binary trace | ③ M1 외 후보 재진입 |
|------|---------------|--------------------------|------------------------|
| **priority (측정 효율)** | 中 — g_l 가설 자체가 §2.2 (B) 처럼 *spec 해석 추정* 단계. 측정 전 production 변경 의무 → 강압-적합 위험 中. | **高** — 측정 = ground-truth 직접 도출. F-oct-prelim 5-cycle (5-1 / 5-2 / 5-3 / 5-4) 누적 측정에서 항상 *간접* (spec 해석 + 한국어 추론) 만 사용 → binary trace 도입이 spec 해석 의존 단절. | 中 — 재해석 cycle 은 신규 측정 0. F-oct-prelim 5 cycle 의 진단 한계 (간접 측정만) 를 넘기 어려움. |
| **risk (E2/E4/E5 발동)** | E5 risk (production 변경 3 라인). E2 risk 中 — g_l 가설의 §A.4.2.3 명시 부재 (§2.1) → spec 인용 우회 fit 위험. | E4 risk **검토 의무** — Annex A binary = ITU 참조 C source 로 추정; plan §9 의 E4 invariant ("외부 G.729 구현 0건 참조") 와 정면 충돌 가능. *trace 측정 결과 인용* 만으로 한정 + spec 절대 우선 + 본 cycle 진입 전 사용자 게이트 필수. | E2 risk 低 (재해석-only). 단 측정 0 → priority 한계 상존. |
| **spec-grounding** | 弱 — §A.4.2.3 본문에 g_l-based 분기 부재 (plan §136). §A.4.2.2 와 결합 해석 단계. | 中 — Annex A binary 자체는 ITU 표준 구현; 단 본 cycle 의 strict reading 채택 (§2.2 (A)) 은 PDF 원문 우선이므로 binary 가 spec 해석을 대체할 수 없다. binary = *단계별 ground-truth 측정 도구* 로만 한정. | 中 — F-oct-prelim 보고서 (`8f693b7` synthesis + 5-1/5-2/5-3/5-4) spec 인용 누적; 단 §3.6 의 spec 인용 부재 (§2.1) 가 재해석 신뢰도 한계. |
| **cost (cycle 수)** | 1 cycle (state 추가 + 측정 + 회귀 검증) | 2-3 cycle (binary 도입 + trace 측정 + spec 정합 검증) — E4 게이트 통과 필수 | 1 cycle (재해석 + 신규 진단 plan 산출) |

### 3.3 측정 기반 단일 결정 매트릭스

| 후보 | 측정 충분성 | spec 정합 안정성 | E-hatch 안전성 | **종합** |
|------|--------------|-------------------|------------------|----------|
| ① | 中 (g_l 측정 도입) | 弱 (g_l 가설 §A.4.2.3 부재) | 中 (E5 발동 + 강압-적합 risk) | △ |
| ② | **高** (단계별 trace ground-truth) | 中 (binary = 측정 도구로만 한정 시 안정) | 中 (E4 사용자 게이트 통과 시 안전) | **○** |
| ③ | 弱 (신규 측정 0) | 中 (spec 인용 누적 활용) | 高 (production 0 + 재해석-only) | △ |

**측정 충분성** 이 본 cycle 결과 (postfix-2 의 G3 반증) 의 직접 후속 의무이므로 *최우선 차원*. ① 은 production fix 가 측정 이전이라 강압-적합 risk 中. ③ 은 신규 측정 0 으로 본 cycle 의 함의 (잔여 결함 위치 식별) 를 해소할 수 없다. ② 가 단독 우위.

---

## §4 권고 단일 결정 + 다음 cycle plan task 분해 outline

### 4.1 권고 단일 결정

**채택**: 후보 ② **Annex A binary 행동 추적 cycle**.

**근거 1문장** (§3.3 종합):

> postfix-2 의 G3 반증 (sample 5..7 Δ=0) 이후 *잔여 결함 위치 식별* 이 다음 cycle 의 단일 의무이며, F-oct-prelim 5-cycle 누적 진단의 *spec 해석 의존 한계* 를 단절할 유일한 방법은 단계별 ground-truth 직접 측정 (Annex A binary trace) — ① 은 측정 전 production 변경 강압-적합 risk, ③ 은 측정 0 으로 결함 위치 미식별 limit 잔존.

### 4.2 다음 cycle plan task 분해 outline (Phase 1k Stage F-non / 가칭)

본 cycle 종료 후 사용자 게이트 통과 시 다음 plan 의 task 분해:

| # | Task | 의무 | scope |
|---|------|------|-------|
| 1 | E4 invariant 재검토 + Annex A binary 도입 game plan | "외부 G.729 구현 0건 참조" 의 *trace 측정 도구 인용* 예외 정의 (또는 E4 유지 시 `pdftotext` + 수동 trace 만 허용 — fallback path 명시). 사용자 게이트 1차. | 본 cycle 의 다음 plan 산출. 측정 0. production 0. |
| 2 | binary build + frame 0 sf0 trace harness | ITU 참조 C source (G.729 Annex A reference, ITU-T 공식 source code) build + Phase 1i 의 frame 0 sf0 sample 0..79 step-by-step trace (μ, γ_t, k1', s_st, s_tilt, g_l, agcGain, hp_in, hp_out) dump. | binary harness 만 — go production 0. |
| 3 | trace 측정 결과 vs go decoder 측정 결과 단계별 비교 | sample 5..7 의 *부호 결정 단계* 를 trace 측정에서 식별 (어느 단계에서 부호가 `+` ↔ `-` divergence). 결함 위치 단일 식별. | 측정-only. production 0. |
| 4 | Task 3 결과의 spec § 매핑 + 결함 cover 가설 갱신 | F-oct-prelim-5-4 §3.6 의 G3 단독 → 다중 cover 가설로 갱신. M1 외 후보 (synth IIR / hpFilter init / AGC) 의 spec § 결손 점검. | 보고서 only. production 0. |
| 5 | 다음 production fix cycle 의 plan 작성 | Task 3 식별 결함의 spec 정합 fix 의 scope / risk / 회귀 게이트 정의. **본 plan 자체 의 종결 의무는 postfix-1 RED test 의 GREEN 전환** (다음 cycle 의 입증 contract 승계). | plan only. fix 미실행. |

총 **5 task**. 핵심 입증 의무 = postfix-1 RED test (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 의 GREEN 전환 — 본 cycle 에서 해소 미입증된 contract 를 승계.

---

## §5 사용자 게이트 항목 (다음 cycle 진입 승인 사항)

다음 cycle plan 작성 dispatch 전 사용자 결정 필수 항목:

| # | 게이트 항목 | 결정 옵션 | default 권고 |
|---|--------------|-------------|----------------|
| G1 | **E4 invariant 재검토** — Annex A binary (ITU 참조 C source) 인용 허용 여부 | (a) 허용 + 측정 도구 한정 (spec 인용 우회 금지) / (b) E4 유지 + 수동 PDF trace 만 / (c) 후보 ① 또는 ③ 으로 pivot | (a) — 측정 도구 한정 + Phase 0.4 §1 강압-적합 회피 의무 명시 |
| G2 | **postfix-1 RED test 잔존 처리** — `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` 다음 cycle 의 입증 contract 승계 | (a) 승계 / (b) skip directive 추가 후 다음 cycle 진입 / (c) 임시 commenting 후 다음 cycle 진입 | (a) — RED test 가 다음 cycle 의 GREEN gate. skip / commenting 은 회귀 안전망 약화. |
| G3 | **잔여 untracked `stagef_bis_diagnostic_test.go` 처리** | (a) 보존 유지 / (b) 다음 cycle 에서 정식 commit / (c) 삭제 | (a) — F-oct-prelim 5-cycle 누적 산출물; 다음 cycle 의 trace harness 와 충돌 가능성 검토 후 결정. |
| G4 | **F-oct-prelim-5-4 §6 결정 (a) 정정 commit 의무** | (a) 별도 정정 보고서 commit / (b) 다음 plan 의 Self-Review 섹션 인용으로 갈음 / (c) 정정 불요 (본 보고서 §2.3 으로 충분) | (c) — 본 보고서 §2.3 이 정정 사실 명시; 별도 commit 은 plan-end 산출물 중복. |
| G5 | **Phase 1k 종결 평가** | (a) Phase 1k 종결 보류 (다음 cycle 후 재평가) / (b) Phase 1k 종결 + 신규 Phase 1l 로 다음 cycle 분리 | (a) — postfix-1 RED contract 가 Phase 1k 정의 의무이므로 종결 보류. plan §747 결정 트리 row 3 정합. |
| G6 | **다음 plan 작성 dispatch 승인** | (a) 본 task 종료 후 즉시 dispatch / (b) G1-G5 결정 후 dispatch / (c) 보류 | (b) — G1 (E4 재검토) 결과가 plan 의 Task 1 scope 결정 |

---

## 부록 A: 본 task 산출물 commit 정보

- commit 1건 (보고서 only): `docs(plans): F-oct-postfix synthesis + cycle decision (E3)`
- 포함 파일: 본 보고서 + `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md` (postfix-2 cycle 산출물 합본)
- production 변경 0 라인. test 변경 0 라인. plan 문서 변경 0 라인.
- co-author trailer 포함.

## 부록 B: 사후 sanity gate (commit 직전 측정)

```
$ go vet ./...
(clean)

$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
=== RUN   TestDecode_Frame0Sample0_MatchesALGTHM
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)
PASS

$ go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)
FAIL
```

세 항목 모두 expected 와 일치 (vet clean + sample0 invariant PASS + RED contract 잔존).
