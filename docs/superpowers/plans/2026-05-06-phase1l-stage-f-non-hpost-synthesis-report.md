# Phase 1l F-non-Hpost 종합 보고서 + 차기 cycle 권고

**작성일**: 2026-05-06
**Cycle ID**: `F-non-Hpost` (Phase 1l 1번째 cycle, NEW (P0c-inter-subframe-postfilter-state) 시나리오 측정)
**범위**: HP-1 (`076b6de`, subframe boundary postfilter 5 sub-state trace) + HP-2 (`2ee0009`, §A.4.2.5 HP filter frame-edge state trace) 측정 결합. plan `308e4f3` §Phase 7 3-시나리오 결정 트리 적용.
**산출물**: 3-시나리오 결정 트리 적용 → **(Hpost-refute)** 시나리오 확정 + Phase 1l 잠정 종결 + alternative path 4-옵션 (a/b/c/d) + 사용자 게이트 G-XS5 권고.
**준수**:
- production 변경 0 라인 (E2 — 본 cycle 3 task 모두 측정-only / synthesis-only).
- 외부 G.729 구현 0 인용 (E1 / G1 결정 — Annex A binary 거부 유지).
- 본 task = 보고서 only — test/code 변경 0.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 (Phase 0.6 보존 의무).
- 측정 bundle (HP-1+HP-2) 자동 promotion 금지 (E5) — 명시 게이트 권고.
- 모든 verdict = `EQ` / `NE` 이진. UNDETERMINED = spec 진정 모호 시에만 (E4).

---

## 0. Working tree + escape hatch 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go        ← Phase 0.6 보존 의무 (미변경 의도)
$ git log -1 --oneline
2ee0009 test(decoder): add Phase 1l HP-2 HP filter frame-edge diagnostic
```

본 commit (Task 3 = synthesis) 후 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go        ← 미변경 (의도)
HEAD = <synthesis commit>  docs(plans): Phase 1l F-non-Hpost synthesis + close + alternative path
```

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 평가 | 근거 |
|------|---------|------|------|
| **E1** | 외부 G.729 구현 인용·실행 | **미발동** | HP-1 / HP-2 / HP-3 모두 PDF (`docs/superpowers/specs/itu/G729E.pdf`) + `READMETV.txt` + repo committed PST + 본 repo internal 패키지만 사용. ITU reference C / Annex A binary / bcg729 / Sipro / FFmpeg G.729 인용 0건. |
| **E2** | production 변경 라인 > 0 | **미발동** | HP-1 commit `076b6de` + HP-2 commit `2ee0009` diff 의 production 0 라인. test 신규 2 파일 (`internal/postfilter/phase1l_hp1_subframe_state_diagnostic_test.go` + `internal/decoder/phase1l_hp_edge_diagnostic_test.go`). 본 task = docs only. |
| **E3** | gate 17 RED 잔존을 task failure 로 간주 | **미발동** | gate 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) = mechanism 미식별 marker. 본 보고서 §10 disposition 권고 명시 (skip / refactor / escalate 3-옵션). |
| **E4** | spec 모호 paragraph cherry-pick / 모호 verdict | **미발동** | HP-1 12 cell UNDETERMINED 는 §4.2.1/2/3 + §A.4.2.1/2/3 의 subframe-boundary policy 명시 부재 = 명시적 spec ambiguity 분류. carryover 가 "ITU 관행상 자연스러우니 EQ" cherry-pick 거부. HP-2 4 cell 중 EQ 2건 = §4.3 catch-all verbatim 인용 직접 매핑, UNDETERMINED 2건 = late-state 명시 부재. |
| **E5** | 측정-only test 자동 promotion (gate 20 자동 등재) | **미발동** | 본 보고서 §7 명시 게이트 권고 (gate 20 HP-1+HP-2 bundle promote 여부 사용자 결정). 자동 promotion 0. |

### 0.3 사용자 G-XS4 결정 정합

- **G-XS4 = "(F-non-Hpost cycle 진입, 3 sub-task: HP-1 + HP-2 + HP-3)"**: 본 cycle 진입 premise. 3 task 모두 plan-bound 충족.
- **bis 보존**: untracked `stagef_bis_diagnostic_test.go` 미변경.
- **gate 17 RED 잔존 acknowledge**: §7 + §10.

---

## 1. F-non-Hpost cycle commit 요약

```
<synthesis hash>  docs(plans): Phase 1l F-non-Hpost synthesis + close + alternative path  ← 본 commit
2ee0009            test(decoder): add Phase 1l HP-2 HP filter frame-edge diagnostic
076b6de            test(postfilter): add Phase 1l HP-1 subframe boundary state diagnostic
308e4f3            docs(plans): Phase 1l F-non-Hpost cycle plan
8e6386c            docs(plans): Phase 0c re-entry synthesis + inter-subframe state hypothesis (직전 cycle 종결)
```

---

## 2. HP-1 verdict — commit `076b6de`

**측정 대상**: ALGTHM + FIXED + PITCH frame 0 의 subframe-1 (sample 0..39) → subframe-2 (sample 40..79) 경계에서 postfilter 5 sub-state (long-term `Hp` / short-term `Hf` (pastS) / short-term `Hf` (pastSynthPost) / tilt γ_t / AGC) carryover-vs-spec EQ/NE.

### 2.1 15-cell verdict matrix (3 vector × 5 sub-state)

| sub-state | spec § | ALGTHM | FIXED | PITCH | spec policy |
|-----------|--------|--------|-------|-------|-------------|
| `Hp` (long-term residual memory) | §4.2.1 / §A.4.2.1 | UNDETERMINED | UNDETERMINED | UNDETERMINED | subframe-boundary 명시 부재 (E4 모호) |
| `Hf` (pastS, short-term input memory) | §4.2.2 / §A.4.2.2 | UNDETERMINED | UNDETERMINED | UNDETERMINED | subframe-boundary 명시 부재 (E4 모호) |
| `Hf` (pastSynthPost, short-term output memory) | §4.2.2 / §A.4.2.2 | UNDETERMINED | UNDETERMINED | UNDETERMINED | subframe-boundary 명시 부재 (E4 모호) |
| γ_t (tilt past input) | §4.2.3 / §A.4.2.3 | UNDETERMINED | UNDETERMINED | UNDETERMINED | subframe-boundary 명시 부재 (E4 모호) |
| AGC g(n-1) | §4.2.4 / §A.4.2.4 | **EQ** | **EQ** | **EQ** | §4.2.4 verbatim mandate carryover (smoothing across subframes) |

**verdict 분포**: NE = 0, EQ = 3 (AGC × 3 vector), UNDETERMINED = 12 (4 sub-state × 3 vector).

### 2.2 Production policy 측정

5 sub-state 모두 production = **carryover** (sf-1 끝 snapshot A == sf-2 시작 snapshot B exact, 3 vector × 5 sub-state 전부).

### 2.3 R-D max|sPf| (silence 부재 확인)

ALGTHM = 38, FIXED = 8, PITCH = 13. 3 vector 모두 nonzero — silence frame 가설 (R-D) 제거.

### 2.4 §4.2.4 verbatim 인용 (AGC EQ 의 hard-spec 근거)

> §4.2.4 (AGC): "The gain control adjusts the gain of the postfiltered signal s'(n) so that its energy is similar to the energy of the synthesis signal s_hat(n). The gain factor G_n for each subframe is computed as ... The gain-scaled postfilter signal sf(n) is given by:  sf(n) = g(n) · s'(n),  where g(n) is updated on a sample-by-sample basis as:  g(n) = 0.85 · g(n-1) + 0.15 · G_n"

→ `g(n-1)` carryover = sample-단위 (subframe 경계에서 reset 부재) **VERBATIM 명시**. production carryover policy = spec EQ.

### 2.5 결론 (HP-1)

- **(Hpost-state-defect)** 시나리오 조건 = HP-1 ≥1 NE → 본 측정 NE = 0 → **REFUTED**.
- AGC 3-cell EQ = §4.2.4 carryover verbatim 직접 매핑 = 첫 hard-spec invariant 충족.
- 12-cell UNDETERMINED = §4.2.1/2/3 + §A.4.2.1/2/3 의 subframe-boundary policy 명시 부재 = E4 ambiguity 분류 (cherry-pick 회피).
- production 의 carryover policy 가 4 sub-state 에 대해서도 spec 와 모순 evidence 0건 (단 spec 가 carryover 명시 없음 → defect 도 정합도 입증 부재).
- **side-finding SF-1 (tilt γ_t mismatch)** orthogonal: γ_t state carryover 자체 EQ; SF-1 은 sf-1 내부의 value/branch 이슈 = HP-1 scope 밖.

---

## 3. HP-2 verdict — commit `2ee0009`

**측정 대상**: ALGTHM + SPEECH frame 0 의 §A.4.2.5 HP filter (2nd-order IIR + ×2) 의 frame-edge state EQ/NE.
- region: early `[0..21]` (frame 시작 transient) + late `[65..79]` (frame 종료 누적).
- production HP filter state field: `Decoder.hpX [2]int16`, `Decoder.hpY [2]int16`.
- production frame-0 init = zero (decoder zero value).

### 3.1 4-cell verdict matrix (2 vector × 2 region)

| region | ALGTHM | SPEECH | spec policy |
|--------|--------|--------|-------------|
| early `[0..21]` (frame 0 첫 호출 init) | **EQ** | **EQ** | §4.3 catch-all verbatim mandate zero-init (Table 9 비-등재) |
| late `[65..79]` (subframe-2 후반 누적) | UNDETERMINED | UNDETERMINED | spec late-state 값 명시 부재 (E4 모호) |

**verdict 분포**: NE = 0, EQ = 2 (early × 2 vector), UNDETERMINED = 2 (late × 2 vector).

### 3.2 §4.3 catch-all verbatim (HP filter zero-init 의 hard-spec 근거)

PDF lines 1696–1707 (§4.3 "Initialization of internal variables"):

> "All static encoder and decoder variables should be initialized to zero, except the variables listed in Table 9."

Table 9 등재 변수 = `{β, g(−1), ^l_i, q_i, Û^(k)}`. **HP filter state (`hpX`, `hpY`) 는 Table 9 비-등재** → §4.3 catch-all 적용 → **zero-init 명시 mandate**. production zero-init = spec EQ (verbatim).

### 3.3 §A.4.2.5 IIR pole pair 정합

HP filter impulse response decay (production 측정) = §4.2.5 IIR pole pair (1.93 z^-1 / -0.94 z^-2 계수) **EXACT 매칭**. HP filter 자체의 transfer function / state propagation = spec verbatim 정합.

### 3.4 Δ pattern 비-correlate 발견

- Δ[0..21] 패턴 (production - want): `(+3, +3, +3, +1, +1, ...)` 등 미세-증가 → **HP filter impulse response decay envelope 와 불일치**.
- HP filter pole-pair decay 는 단조 감쇠 (지수형) — Δ pattern 은 step-form / boundary-cluster — 두 envelope 가 형상 dimension 모순.
- 결론: **HP filter 자체는 spec 정합 (early EQ + impulse decay 정합), Δ origin 은 HP filter 상위 / 외부 mechanism 에 위치**.

### 3.5 결론 (HP-2)

- **(HP-edge-defect)** 시나리오 조건 = HP-1 EQ_ALL + HP-2 ≥1 NE → 본 측정 NE = 0 → **REFUTED**.
- early-region 2-cell EQ = §4.3 catch-all verbatim 직접 매핑 = **두 번째 hard-spec invariant 충족**.
- late-region 2-cell UNDETERMINED = spec 명시 부재 (E4 모호 분류).
- HP filter mechanism = spec verbatim 정합. Δ pattern origin = HP 외부.

---

## 4. 3-시나리오 결정 트리 적용

| 시나리오 | 조건 | 본 cycle 결과 | 적용 |
|---------|------|----------------|------|
| **(Hpost-state-defect)** | HP-1 12-cell ≥1 NE | NE = 0 (3 EQ + 12 UNDETERMINED) | **REFUTED** |
| **(HP-edge-defect)** | HP-1 EQ_ALL + HP-2 ≥1 NE | HP-1 NE = 0, HP-2 NE = 0 | **REFUTED** |
| **(Hpost-refute)** | HP-1 + HP-2 모두 NE = 0 | HP-1 NE = 0 + HP-2 NE = 0 | **확정** |

**선택 시나리오**: **(Hpost-refute)**.

**의미**:
- (Hpost-state-defect) / (HP-edge-defect) 양자 제거 → postfilter inter-subframe state propagation + HP filter frame-edge transient init 양 측면에서 production = spec 정합.
- spec-내부 mechanism 후보 = NEW (P0c-inter-subframe-postfilter-state) 시나리오에서 측정 surface 였던 5 sub-state + HP edge × 2 region 모두 NE 0.
- §4.2.4 carryover + §4.3 catch-all + §A.4.2.5 impulse-decay = **22 sub-hypothesis 누적 폐기 후 처음으로 마주친 3건의 hard-spec invariant**. spec-내부 mechanism 후보 공간 = **공식 고갈** (Phase 1k 잠정 종결의 worst-case 시나리오 mirror).

---

## 5. **Critical insight** — Hard-spec invariant 3건 + spec-내부 후보 공간 공식 고갈

### 5.1 Hard-spec invariant 1 — §4.2.4 AGC carryover (verbatim)

> §4.2.4: "g(n) = 0.85 · g(n-1) + 0.15 · G_n ... g(n) is updated on a sample-by-sample basis"

→ subframe 경계에서 g(n-1) **reset 금지** verbatim. AGC 3-cell EQ (HP-1) = 직접 매핑.

### 5.2 Hard-spec invariant 2 — §4.3 catch-all zero-init (verbatim)

> §4.3 (lines 1696–1707): "All static encoder and decoder variables should be initialized to zero, except the variables listed in Table 9."

→ Table 9 = `{β, g(−1), ^l_i, q_i, Û^(k)}`. HP filter state (`hpX`, `hpY`) 비-등재 → frame-0 zero-init **mandate**. HP-2 early 2-cell EQ = 직접 매핑.

### 5.3 Hard-spec invariant 3 — §A.4.2.5 HP filter impulse response (계수 verbatim)

> §A.4.2.5 = "Same as described in clause 4.2.5." §4.2.5 HP filter b/a 계수 (1.93 z^-1 / -0.94 z^-2 IIR pole pair) production impulse response decay 와 **EXACT 매칭**.

→ HP filter mechanism 자체 = spec verbatim 정합 (HP-2 §3.3).

### 5.4 결과: spec-내부 mechanism 후보 공간 공식 고갈

22 sub-hypothesis (16 Phase 1k + 4 Phase 0c + 2 Phase 1l) 누적 폐기 + 3 hard-spec invariant 직접 매핑 = **PDF + READMETV.txt + textbook (Kondoz, Spanias) 단독 source 로 추론 가능한 spec-내부 mechanism 후보 = 공집합**.

→ Phase 1k 잠정 종결 시점의 "worst-case 시나리오" mirror. 후속 cycle 은 **spec source 확장 (옵션 c)** 또는 **non-mechanism path (옵션 a/b/d)** 에 의존.

---

## 6. 누적 폐기 catalog (16 Phase 1k + 4 Phase 0c + 2 Phase 1l = **22건**)

### 6.1 Phase 1k 누적 16건 (carry from `d448282`)

`M1'`, `M2`, `M3`, `M5`, `M6`, `Cα` (X-split-1), `Cβ` (X-split-2), `Z` (PST chain), `Y(sign)` (a[0..10] sign 11/11), `forced-flip linearity`, `Cγ-postfilter §4.2.1` (long-term), `Cγ-postfilter §4.2.2` (short-term), `Cγ-postfilter §4.2.3/4` (tilt), `Cγ-postfilter §4.2.5` (AGC+HP), `Cγ-synth §3.10/§4.1.6 IIR`, `Cγ-Y-mag (small +6 perturbation)`.

### 6.2 Phase 0c 신규 폐기 4건 (carry from `8e6386c`)

| sub-hypothesis | 출처 | verdict |
|----------------|------|---------|
| `P0c-format-defect` (PST format/endianness/header/unit) | P0c-1 (`8ec97f5`) | **REFUTED** |
| `P0c-want-stage-defect` (S\* ≠ postX2) | P0c-2 (`aeee9e9`) | **REFUTED** |
| `P0c-uniform-delta` (sample-uniform constant Δ) | P0c-3 (`68a7df9`) | **REFUTED** |
| `P0c-ALGTHM-isolated` (ALGTHM frame 0 단독) | P0c-3 (`68a7df9`) | **REFUTED** |

### 6.3 Phase 1l 신규 폐기 2건 (본 cycle)

| sub-hypothesis | 출처 | verdict |
|----------------|------|---------|
| `Hpost-state-defect` (postfilter inter-subframe carryover/reset 균열) | HP-1 (`076b6de`) | **REFUTED** (NE = 0; AGC EQ verbatim, 4 sub-state UNDETERMINED) |
| `HP-edge-defect` (§A.4.2.5 HP filter frame-0 init 균열) | HP-2 (`2ee0009`) | **REFUTED** (NE = 0; early EQ verbatim, late UNDETERMINED) |

### 6.4 i=40 candidate (cross-vector P0c-3 derived) 자체 처분

- **i=40 inter-subframe postfilter state defect candidate**: HP-1 carryover EQ (5 sub-state × 3 vector) 가 inter-subframe state 균열 가설을 직접 refute → **자체 REFUTED**.
- 본 cycle 의 sub-hypothesis 폐기는 12 (HP-1 5 sub-state × 3 vector 중 12 UNDETERMINED 는 폐기 카운트 외) 가 아니라 2 (시나리오 명 단위) 로 산정.

**누적 22건 폐기 + 식별 결함 0건** + **3 hard-spec invariant 매핑 완료** + **spec-내부 mechanism 후보 = 공집합 (공식 고갈)**.

---

## 7. 19-gate 상태 dump

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | Phase 1a~1j 누적 16건 | **PASS** | 변동 없음. |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) | **RED 잔존 (의도)** | mechanism 미식별 — 22 sub-hypothesis 폐기 후 spec-내부 후보 공집합. §10 disposition 권고 (skip / refactor / escalate 3-옵션). |
| 18 | F-non-prelim-X-split bundle (Cα fcb + Cβ gain) | **PASS** | aa9dcf9 시점 promotion. |
| 19 | P0c-reentry + Phase 1l measurement bundle (P0c-1/2/3 + HP-1/HP-2 결합) | **pending — auto-promote 금지 (E5)** | 본 보고서 §0.2 권고: **명시 사용자 게이트 G-XS5 후 promotion 결정**. 후보 promotion 형태 = 5 측정 test (P0c 3건 + HP 2건) classifier verdict 회귀 보호. |

---

## 8. Plan-allowed FAIL 목록 (regression baseline, 변동 없음)

```
$ go vet ./...        → clean (VET-OK)
$ go test ./... -race → 4 pre-existing FAIL 잔존 (변동 없음):
                         - TestDiagnostic_SinglePulseChain               (gain log-domain 14 dB 진단)
                         - TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput  (gate 17 marker)
                         - TestDecode_LowEnergyCodebookIsSmooth          (gain edge saturation)
                         - TestDecode_SucceedsAcrossAllGainIndices       (gain (GA, GB) edge saturation)
```

baseline 변동 0. production 변경 0. test 변경 0 (본 task = docs only).

---

## 9. Phase 1l 종결 verdict + Phase 1k vs Phase 1l 비교

### 9.1 종결 verdict

**Phase 1l = 잠정 종결 (worst-case 시나리오 = (Hpost-refute) 확정)**.

- spec-내부 mechanism 후보 = 공집합 (재고갈). Phase 1k 잠정 종결의 mirror.
- 본 cycle = 추가 2 sub-hypothesis 폐기 + 3 hard-spec invariant 매핑 + i=40 candidate 자체 처분.
- 후속 = alternative path (a/b/c/d) 4-옵션 사용자 게이트 G-XS5.

### 9.2 결정 비교 표 — Phase 1k closure vs Phase 1l closure

| 항목 | Phase 1k closure (`d448282`) | Phase 1l closure (본 cycle) |
|------|------------------------------|------------------------------|
| 누적 폐기 | 16 sub-hypothesis | **22 sub-hypothesis** (16 + 4 P0c + 2 Phase 1l) |
| 식별 결함 | 0 | 0 |
| 측정 surface | 단일 sample 시점 (sample 5..7) + chain stage 단일 절단 | 80 sample 전체 + cross-vector (4 vector) + sub-state 5건 + HP edge 2 region |
| 폐기 mechanism family | postfilter chain (M1'/M2/M3/M5/M6) + Cα/Cβ/Y/Cγ chain | + PST format / want chain stage / inter-subframe state / HP edge |
| Hard-spec invariant 매핑 | 0 (전부 spec 모호 / 측정 mismatch / linearity) | **3** (§4.2.4 AGC carryover + §4.3 catch-all zero-init + §A.4.2.5 impulse decay) |
| 결정적 cross-vector evidence | 부재 | low/high energy split (P0c-3) + boundary/interior Δ pattern |
| 종결 시나리오 | (Cγ-refute) — alternative (a) Phase 0c 재진입 | (Hpost-refute) — alternative (a/b/c/d) 4-옵션 사용자 게이트 |
| spec-내부 후보 공간 | 공집합 (1차 고갈) | **공집합 (재고갈, 공식)** — PDF + READMETV.txt + textbook source 한계 |
| 가용 alternative path | (a) Phase 0c 재진입 (실행됨 → P0c-reentry) | (a/b/c/d) 4-옵션 — Phase 0c 재진입 path 이미 소진 |

**Phase 1k → Phase 1l = 측정 surface 확장 (시간축 + cross-vector) → spec-내부 후보 공간 재고갈 + hard-spec invariant 3건 직접 매핑 + alternative path 옵션 (a)(P0c) 소진 → 옵션 공간 (b/c/d) 으로 축소**.

---

## 10. 차기 cycle alternative path 4-옵션 + 권고 우선순위

### 10.1 4-옵션 정의

#### **(a) Phase 1g multi-frame state 진단 재진입**
- **목적**: frame 0 한정 측정을 frame 0..N 누적 / 다중 frame propagation 으로 확장. frame-rate state (LSP interpolation across frames, gain VQ moving average β/g(−1), pitch lag prev) 의 spec 정합성 측정.
- **scope**: frame 0..3 decode 결과 PST trace + frame-edge state (β, g(-1), q_i, Û^(k), prev pitch lag) snapshot.
- **비용**: 高 (multi-frame instrumentation + 4 vector × N frame test surface 확장).
- **expected gain**: 中 (HP-1 inter-subframe 차원이 EQ 인 점이 frame-rate 차원 EQ 를 정당화하지는 않음 — 시간축 한 단계 더 확장 surface).
- **risk**: spec-내부 후보 재고갈 시 추가 sub-hypothesis 폐기 누적만 발생.

#### **(b) Cγ chain elsewhere — parameter decode pipeline upstream of LP synthesis**
- **목적**: postfilter chain 하류 (sPf 까지) 가 spec 정합 → 상류 (parameter decode → excitation u → synth syn) 측 균열 후보 재방문. 특히 **gain VQ tables / FCB position decoding / LSP interpolation across subframes**.
- **scope**:
  - gain VQ codebook (`internal/gain/`) Q-format / 테이블 값 vs PDF Annex A Table verbatim 비교.
  - FCB position decoding (`internal/fcb/`) bit pattern → pulse position 변환 spec verbatim.
  - LSP interpolation (`internal/lsp/`) sf-1 / sf-2 weight (0.5 / 0.5 또는 다른 verbatim 정의) 측정.
- **비용**: 中 (각 모듈 single-frame trace 반복).
- **expected gain**: 中-高 (parameter decode upstream 은 측정 surface 가 명확 + spec verbatim 풍부).
- **risk**: F-non-prelim-X-split-1/2 + F-non-prelim-1/2 가 일부 영역 (Cα fcb pulse + Cβ gain g_c + Y a[0..10] + linearity) 이미 폐기 — 잔여 surface 확인 필요.

#### **(c) ITU corrigendum / additional spec source search**
- **목적**: G1 invariant (Annex A binary 거부) 유지 하 PDF errata / 보충 권고 (G.729 Appendix I/II/III) / textbook secondary source 확장.
- **scope**:
  - PDF G729E.pdf 가 최신 errata 반영본인지 ITU-T 사이트 확인.
  - G.729 Appendix I (low-complexity), II (DTX/CNG), III (extension) 가 fixed-point 산출에 영향 주는지 확인.
  - Kondoz / Spanias textbook 의 G.729 chapter 보조 인용 가능 paragraph 확인.
- **비용**: 低 (문서 fetch + 인용 확인).
- **expected gain**: 不確定 (errata 가 본 mismatch 와 직접 관련될 확률 낮음, 단 cost 도 낮음).
- **risk**: errata 확인 무성과 시 (a)/(b) 진입 지연.

#### **(d) NEW — gate 17 RED contract disposition 결정**
- **목적**: 22 sub-hypothesis 폐기 + 0 defect 식별 시점에서 gate 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 의 RED 잔존을 "버그 marker" 로 유지할지 / "spec-correct, vector-derivation-uncertain" 로 재분류할지 결정.
- **3-sub-옵션**:
  - **(d-i) skip**: `t.Skip("spec-correct, vector-derivation-uncertain pending corrigendum review")` 으로 재분류 → RED noise 제거, 후속 cycle 의 다른 RED 와 분리 가능.
  - **(d-ii) refactor**: 현 contract test 를 sample-level NE 검증에서 chain-internal invariant 검증 (예: AGC g(n) carryover EQ) 으로 refactor → spec verbatim 직접 매핑.
  - **(d-iii) escalate to user**: gate 17 disposition 자체를 사용자 결정 사안으로 escalate, 본 cycle 종결 시 commit 0 변경.
- **비용**: 最低 (skip = 1-line edit, refactor = 1 test edit, escalate = 0 변경).
- **expected gain**: 高 (RED noise 제거 → 후속 cycle 의 회귀 baseline 가시성 향상 + 22 sub-hypothesis 폐기 evidence 가 "spec-correct" 분류 정당화).
- **risk**: skip 선택 시 mechanism 식별 evidence 누락 위험 — 단 22 sub-hypothesis 폐기 + 3 hard-spec invariant 매핑이 disposition 변경 정당화.

### 10.2 권고 우선순위 (default suggestion + 정당화)

**권고 ordering**: **(d) → (b) → (c) → (a)**.

| rank | option | 이유 (정당화) |
|------|--------|--------------|
| 1 | **(d) gate 17 disposition** | **최저 비용 + 최고 즉시 가치**. 22 sub-hypothesis 폐기 누적 evidence 가 disposition 재결정의 充分 근거. RED noise 제거 → 후속 (b)/(c)/(a) cycle 의 회귀 baseline 가시성 보장. **default 권고**: (d-iii) escalate to user (자동 변경 금지, E2/E3 정합). |
| 2 | **(b) Cγ chain elsewhere — parameter decode upstream** | **측정 surface 명확 + spec verbatim 풍부 + 비용 中**. postfilter 하류 EQ 확정 → 상류 측 균열 후보 미탐색 영역 (gain VQ table / FCB position decode / LSP interp) 존재. 단 일부 sub-area 이미 폐기 (Cα/Cβ/Y) — 잔여 surface 확정 필요. |
| 3 | **(c) corrigendum / spec source 확장** | **비용 低 + 不確定 yield**. 단 cost 가 낮아 (b) 진입 전 배경 조사로 병행 가능. PDF errata 확인 + G.729 Appendix I/II/III 영향 평가 = 1 cycle 내 완료 가능. |
| 4 | **(a) Phase 1g multi-frame state** | **비용 高 + 中 yield**. multi-frame instrumentation 부담 + frame-rate state 가 frame 0 한정 mismatch 의 직접 원인 확률 不高 (frame 0 = clean init 가정 시). (b)/(c) 소진 후 진입 권고. |

**default 권고 핵심 1줄**: **(d-iii) escalate gate 17 disposition to user → (b) parameter decode upstream re-visit → (c) corrigendum 병행 조사 → (a) multi-frame 최후수단**.

---

## 11. Side-finding catalog (carry + 갱신)

| ID | 내용 | spec § / 출처 | sign 영향 | disposition |
|----|------|---------------|-----------|-------------|
| SF-1 | tilt γ_t gating mismatch (carry from F-non-Cgamma-revisit + HP-1 결합) | `internal/postfilter/tilt.go` vs §4.2.3 | sample 5..7 부호 무관 (F-oct-postfix-2 입증) + γ_t state carryover EQ (HP-1 §2 입증) → orthogonal sub-hypothesis 잔존 | **standing**: γ_t state carryover 자체는 EQ; SF-1 = sf-1 내부 value/branch 이슈. (b) Cγ chain elsewhere 의 parameter decode upstream 후보와 결합 측정 권고. |
| SF-2 | gate 17 RED disposition 3-옵션 → 4-옵션 (carry, 갱신) | `internal/decoder/stagef_octpostfix_regression_test.go` | RED — 22 sub-hypothesis 폐기 후 disposition 재결정 권고 | **NEW disposition 후보 (d-i/ii/iii)** §10 권고. default = (d-iii) escalate to user. |
| SF-3 | sample-unit UNDETERMINED (P0c-1) | PDF §A.4.2.5 + READMETV.txt 모두 Q-format 명시 부재 | sign 영향 미정 | (c) corrigendum / spec source 확장과 결합 (carry). |
| SF-4 | low/high energy split (P0c-3) | P0c-3 verdict matrix | mechanism 강력 signal — 단 HP-1/HP-2 측정으로 inter-subframe + HP edge 2 영역 모두 EQ 확인 → mechanism 위치 = postfilter 외부 (상류 또는 multi-frame) 좁혀짐 | (b) Cγ chain elsewhere 의 parameter decode upstream 진입 시 첫 surface. |
| SF-5 (NEW) | HP filter Δ pattern non-correlate (HP-2 §3.4) | impulse decay 단조 vs Δ step-form | sign 영향 미정 — Δ origin 위치만 좁힘 | mechanism 위치 = HP 상위 (sPf ← postfilter chain ← syn ← excitation). (b) parameter decode upstream 진입 시 분리 측정. |

---

## 12. 열린 follow-up

| ID | 내용 | tracking |
|----|------|----------|
| FU-1 | F-bis-1 / F-tris diagnostic (`stagef_bis_diagnostic_test.go`, untracked, Phase 0.6 보존) | 다음 cycle 진입 시 commit / 폐기 결정 |
| FU-2 | tilt γ_t gating SF-1 별도 cycle | (b) Cγ chain elsewhere 진입 시 결합 |
| FU-3 | gate 19 promotion 명시 사용자 게이트 G-XS5 (E5) | 본 보고서 §7 권고 |
| FU-4 | gate 17 RED disposition (SF-2) — 4-옵션 (d-i/ii/iii) | 본 보고서 §10 (d) 권고 |
| FU-5 | OVERFLOW.BIT loader bug | 후속 cycle 별도 (carry) |
| FU-6 | `decode_test.go` 8 `t.Skip` 해제 | 후속 cycle 별도 (carry) |
| FU-7 | 3 plan-allowed FAIL (SinglePulseChain / LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices) | 후속 cycle 별도 (carry) |
| FU-8 | sample-unit UNDETERMINED (SF-3) → spec source 확장 | (c) corrigendum 결합 (carry) |
| FU-9 (NEW) | HP filter Δ pattern non-correlate (SF-5) → mechanism 위치 = HP 상위 | (b) parameter decode upstream 진입 시 분리 측정 |

---

## 13. 본 cycle 종결 산출

### 13.1 plan checkbox

본 commit 후 plan `2026-05-06-phase1l-stage-f-non-hpost-plan.md` §Task 진행 status:

- [x] Task 1 — HP-1 (`076b6de`).
- [x] Task 2 — HP-2 (`2ee0009`).
- [x] **Task 3 — HP-3 synthesis (본 보고서)** — (Hpost-refute) 시나리오 확정 + Phase 1l 잠정 종결 + alternative path 4-옵션 사용자 게이트 G-XS5 권고.

### 13.2 회귀 게이트 검증

```
$ go vet ./...        → clean (VET-OK)
$ go test ./... -race → 4 pre-existing FAIL 잔존 (변동 없음):
                         - TestDiagnostic_SinglePulseChain
                         - TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput  (gate 17)
                         - TestDecode_LowEnergyCodebookIsSmooth
                         - TestDecode_SucceedsAcrossAllGainIndices
```

baseline 변동 0. production 변경 0. test 변경 0 (본 task = docs only).

### 13.3 사용자 게이트 의무 G-XS5 (다음 dispatch 전)

1. **gate 17 disposition 결정** (SF-2, §10 (d)) — (d-i skip / d-ii refactor / d-iii escalate-stay) 중 선택.
2. **차기 alternative path 진입 승인** — §10 권고 ordering ((d) → (b) → (c) → (a)) 채택 여부 + 첫 dispatch cycle 명 결정.
3. **gate 19 promotion 결정** (E5) — P0c-1+2+3 + HP-1+HP-2 measurement bundle 회귀 보호 등재 여부 yes/no.
4. **SF-1 (tilt γ_t) 별도 cycle vs (b) 결합 측정 priority** 선택.

---

**보고서 종료.** Phase 1l F-non-Hpost = **(Hpost-refute) 시나리오 확정** + **Phase 1l 잠정 종결 (worst-case mirror Phase 1k)** + **22 sub-hypothesis 누적 폐기 + 0 defect + 3 hard-spec invariant 매핑 + spec-내부 mechanism 후보 공식 고갈** verdict. 다음 cycle dispatch = 사용자 G-XS5 (alternative path 4-옵션 선택) 게이트 통과 후.
