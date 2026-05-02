# Phase 0c Re-entry Want-Domain Re-interpret 종합 보고서 + 차기 cycle 권고

**작성일**: 2026-05-05
**Cycle ID**: `P0c-reentry` (Phase 0c 재진입, Phase 1k 잠정 종결 직후 alternative path (a))
**범위**: P0c-1 (`8ec97f5`, ALGTHM.PST format) + P0c-2 (`aeee9e9`, want-stage interpretation) + P0c-3 (`68a7df9`, cross-vector Δ pattern) 측정 결과 결합. plan `82568dd` §Phase 7 4-시나리오 결정 트리 적용 + cross-vector 신규 evidence 기반 NEW 시나리오 도입.
**산출물**: 4-시나리오 결정 트리 적용 → **(P0c-inter-subframe-postfilter-state)** NEW 시나리오 확정 + 차기 cycle `F-non-Hpost` 3-task 윤곽 + 사용자 게이트.
**준수**:
- production 변경 0 라인 (E2 — 본 cycle 4 task 모두 측정-only).
- 외부 G.729 구현 0 인용 (E1 / G1 결정 — Annex A binary 거부 유지).
- 본 task = 보고서 only — test/code 변경 0.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 (Phase 0.5 보존 의무).
- 측정 bundle (P0c-1+2+3) 자동 promotion 금지 (E5) — 명시 게이트 권고.
- 모든 verdict = `EQ` / `NE` 이진.

---

## 0. Working tree + escape hatch 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go        ← Phase 0.5 보존 의무 (미변경 의도)
$ git log -1 --oneline
68a7df9 test(decoder): add Phase 0c-3 cross-vector want pattern diagnostic
```

본 commit (Task 4 = synthesis) 후 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go        ← 미변경 (의도)
HEAD = <synthesis commit>  docs(plans): Phase 0c re-entry synthesis + inter-subframe state hypothesis
```

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 평가 | 근거 |
|------|---------|------|------|
| **E1** | 외부 G.729 구현 인용·실행 | **미발동** | 본 cycle 4 task 모두 PDF (`docs/superpowers/specs/itu/G729E.pdf`) + `READMETV.txt` + repo committed PST + 본 repo internal 패키지만 사용. Annex A binary trace 0건. |
| **E2** | production 변경 라인 > 0 | **미발동** | P0c-1/2/3 commit diff 의 production 0 라인. test 변경 = `internal/decoder/phase0c_pst_format_diagnostic_test.go` + `phase0c_want_stage_diagnostic_test.go` + `phase0c_cross_vector_diagnostic_test.go` 3 신규 파일. 본 task = docs only. |
| **E3** | gate 17 RED 잔존을 task failure 로 간주 | **미발동** | gate 17 = mechanism 식별 후 별도 fix cycle 의 GREEN 대상. 본 cycle 종결 시점에서 RED 의도 잔존 acknowledge. |
| **E4** | spec 모호 paragraph cherry-pick / 모호 verdict ("almost EQ") | **미발동** | P0c-1 verdict 4-tuple (3 EQ + 1 UNDETERMINED) — UNDETERMINED 는 cherry-pick 회피 명시 분류. P0c-2 / P0c-3 모두 `signMatch` integer + `sumAbsDiff` integer + 인덱스 list 이진. 모호 표현 0건. |
| **E5** | 측정-only test 자동 promotion (gate 19/20 자동 등재) | **미발동** | 본 보고서 §6 명시 게이트 권고 (gate 19 P0c bundle promote 여부 사용자 결정). 자동 promotion 0. |

### 0.3 사용자 G-XS3 결정 정합

- **G-XS3 = "(a) Phase 0c 재진입 + want 도메인 재해석 cycle, 3 sub-task"**: 본 cycle 진입 premise. 3 측정 task + 1 ad-hoc synthesis 모두 plan-bound 충족.
- **bis 보존**: untracked `stagef_bis_diagnostic_test.go` 미변경.
- **gate 17 RED 잔존 acknowledge**: §7.

---

## 1. P0c-reentry cycle commit 요약

```
<synthesis hash>  docs(plans): Phase 0c re-entry synthesis + inter-subframe state hypothesis  ← 본 commit
68a7df9            test(decoder): add Phase 0c-3 cross-vector want pattern diagnostic
aeee9e9            test(decoder): add Phase 0c-2 want-stage interpretation diagnostic
8ec97f5            test(decoder): add Phase 0c-1 ALGTHM.PST format diagnostic
82568dd            docs(plans): Phase 0c re-entry want-domain re-interpret plan
d448282            docs(plans): F-non-Cgamma-revisit synthesis + Phase 1k decision (직전 cycle 종결)
```

---

## 2. P0c-1 verdict — commit `8ec97f5`

**측정 대상**: `ALGTHM.PST` 4 차원 (byte-order / header / frame-count / sample unit) format 재검증.

| 차원 | 가정 | 측정 | verdict |
|------|------|------|---------|
| byte-order | Intel little-endian int16 (READMETV "Intel (PC) format") | LE int16 검증 (5600 byte 정수 분해) | **EQ** |
| header | 없음 (5600 = 160 × 35 정수) | 정수 분해 검증, header bytes 부재 | **EQ** |
| frame-count | 35 (160 byte/frame × 35) | 35 정수 검증 | **EQ** |
| sample unit | Q0 raw int16 (PDF §A.4.2.5 "multiplied by a factor 2 to restore the input signal level") | PDF / README 모두 Q-format 명시 부재 | **UNDETERMINED** (E4 모호) |

**4-tuple verdict** = `[byte-order=EQ, header=EQ, frame-count=EQ, unit=UNDETERMINED]`.

**§A.4.2.5 / §4.2.5 verbatim** (P0c-1 commit message):
> §A.4.2.5: "Same as described in clause 4.2.5."
> §4.2.5 (인용 fragment): "multiplied by a factor 2 to restore the input signal level"

→ **(P0c-format-defect) REFUTED** (3 차원 EQ + 1 차원 UNDETERMINED, NE 0건). UNDETERMINED 는 spec-부재 cherry-pick 회피 분류로 별도 sub-hypothesis (`P0c-unit-spec-ambiguity`) 로 분리 가능하나, 현 시점 PST sample unit 가정 (Q0 raw int16, ×2 적용 후) 와 정합성 모순 evidence 0건.

---

## 3. P0c-2 verdict — commit `aeee9e9`

**측정 대상**: ALGTHM.PST frame 0, 80 sample, 4 chain stage (syn / sPf / postHP / postX2) × want.

| Stage  | spec § | signMatch | sumAbsDiff | Δpattern |
|--------|--------|-----------|------------|----------|
| syn    | §4.1.6 | 63/80     | 419        | random   |
| sPf    | §4.2 (long-term + short-term + tilt + AGC) | 63/80 | 387 | random |
| postHP | §A.4.2.5 step 1 (HP filter) | 76/80 | 420 | random |
| postX2 | §A.4.2.5 step 2 (×2 multiplier, 현 production PST) | 76/80 | **314** (argmin) | random (boundary-cluster) |

**S\* (argmin sumAbsDiff) = postX2** ✓ (현재 production "PST = post-AGC+HP+×2" chain-stage 가정 holds).

BUT signMatch[postX2] = 76/80 < 78 escape threshold → **4 sample sign mismatch** (sample 5,6,7 + 1 추가).

**postX2 differ-from-want index 분포** (36 indices):
- cluster A: `[1..21]` (frame 시작 boundary)
- middle: `[22..64]` 완전 EQ
- cluster B: `[65..79]` (frame 종료 boundary)

→ **(P0c-want-stage-defect) REFUTED** (S* = postX2 chain-stage 가정 holds). 단 escape-hatch threshold (signMatch ≥ 78) 미충족 → frame 0 80-sample 전체 polarity-EQ 미입증 → mechanism 후보 = chain stage 식별 외부 (post-stage state init 또는 multi-frame propagation).

---

## 4. P0c-3 verdict — commit `68a7df9` (cross-vector)

**측정 대상**: 4 vector (ALGTHM + SPEECH + FIXED + PITCH) frame 0 sample 0..79 production (postX2) vs want Δ pattern.

| vector  | Δ s5..7 | sign-match s0..79 | differ-cluster (s0..79) |
|---------|---------|-------------------|--------------------------|
| ALGTHM  | (+3,+3,+3) | 76/80 | boundary-only `[1..21] ∪ [65..79]`, middle `[22..64]` 완전 EQ |
| SPEECH  | (+2, 0, 0) | 76/80 | boundary-only `[0, 2, 3, 5, 75..79]`, middle 완전 EQ |
| FIXED   | (+1,+1,+1) | 73/80 | mixed, 인터내 differ `[40..64]` 영역 포함 |
| PITCH   | (+1,+1,+1) | 72/80 | mixed, 인터내 differ `[40..64]` 영역 포함 |

### 4.1 결정적 cross-vector 발견

- (i) **sample-uniform constant Δ across all vectors REFUTED**: ALGTHM Δ s5..7 = (+3,+3,+3) vs SPEECH (+2,0,0) vs FIXED/PITCH (+1,+1,+1) — sample-uniform 단일 상수 mechanism 부재.
- (ii) **ALGTHM-isolated multi-frame state init REFUTED**: 4 vector 모두 frame 0 sf0 NE — ALGTHM 단독 현상 아님 (alternative (b) 단순형 multi-frame state init 가정 — frame 0 = clean init — 제거).
- (iii) **NEW low/high energy split**:
  - **low-energy** (ALGTHM + SPEECH, 2/4): differ 가 **순수 boundary cluster** (frame 시작·종료 sample). middle `[22..64]` 완전 EQ.
  - **high-energy** (FIXED + PITCH, 2/4): differ 가 **interior 영역 `[40..64]` 포함** = subframe-2 onset (sample 40 = subframe boundary).

### 4.2 해석

- (iii) high-energy interior Δ 가 sample 40 (subframe-1 → subframe-2 boundary) 에서 시작 = **subframe 경계에서 postfilter state propagation 의 spec 정합성 균열 가능성** 의 강력 signal.
- (iii) low-energy boundary cluster `[0..21] ∪ [65..79]` = **frame 시작/종료에서 postfilter (특히 HP filter / AGC) state 의 frame-edge 전이 균열** 의 보조 signal.
- middle 영역 `[22..64]` 의 EQ 가 4 vector 중 2 vector 에서 완전 보존 = chain-internal 상태 propagation 의 stable region 존재 → mechanism 이 **state 초기화 / 경계 전이 시점에 국한** 됨을 시사.

→ **(P0c-refute) 시나리오 (단순 spec-내부 mechanism 부재 → alternative (b) Phase 1g 다-frame state) 부분 지지 + 재구성**: multi-frame 보다 **inter-subframe (frame 내 subframe 경계) postfilter state propagation** 이 mechanism 우선 후보.

---

## 5. 4-시나리오 결정 트리 적용

| 시나리오 | 조건 | 본 cycle 결과 | 적용 |
|---------|------|----------------|------|
| **(P0c-format-defect)** | P0c-1 4 차원 중 ≥1 NE | 3 EQ + 1 UNDETERMINED, NE 0건 | **REFUTED** |
| **(P0c-want-stage-defect)** | P0c-1 EQ + P0c-2 S\* ≠ postX2 | S\* = postX2 (argmin sumAbsDiff = 314) | **REFUTED** |
| **(P0c-refute → alternative (b) Phase 1g multi-frame state)** | P0c-1 EQ + P0c-2 S\* = postX2 + P0c-3 cross-vector consistent (sample-uniform 또는 zero) | sample-uniform REFUTED, ALGTHM-isolated REFUTED | **부분 지지 + 재구성** (단순 multi-frame init 가설 제거, inter-subframe propagation 우선 후보) |
| **NEW (P0c-inter-subframe-postfilter-state)** | P0c-3 high-energy vector (FIXED/PITCH) interior Δ 가 sample 40 (subframe boundary) 에서 시작 + low-energy vector (ALGTHM/SPEECH) boundary cluster 만 differ | FIXED/PITCH differ `[40..64]` 포함, ALGTHM/SPEECH boundary-only | **확정 (NEW)** |

**선택 시나리오**: **(P0c-inter-subframe-postfilter-state)** NEW.

**의미**:
- (P0c-format-defect) / (P0c-want-stage-defect) 양자 제거 → want vector 도메인 / chain stage 식별 측면에서 production 가정 holds.
- (P0c-refute) 단순형 (alternative (b) Phase 1g multi-frame state) 제거 → frame 0 자체에서 NE 발생 = multi-frame init 가설 부재 + subframe-내부 state propagation 가설 강화.
- NEW 시나리오 → spec-내부 mechanism 후보 공간 재개 (Phase 1k 잠정 종결 시점의 "spec-내부 후보 고갈" verdict 와 모순 아님 — 본 cycle 측정 surface 가 80-sample × cross-vector 까지 확장되어 새 evidence 영역 발굴).

---

## 6. 누적 폐기 catalog (Phase 1k 16 + Phase 0c 2 = 18건)

### 6.1 Phase 1k 누적 16건 (carry from `d448282`)

`M1'`, `M2`, `M3`, `M5`, `M6`, `Cα` (X-split-1), `Cβ` (X-split-2), `Z` (PST chain), `Y(sign)` (a[0..10] sign 11/11), `forced-flip linearity`, `Cγ-postfilter §4.2.1` (long-term), `Cγ-postfilter §4.2.2` (short-term), `Cγ-postfilter §4.2.3/4` (tilt), `Cγ-postfilter §4.2.5` (AGC+HP), `Cγ-synth §3.10/§4.1.6 IIR`, `Cγ-Y-mag (small +6 perturbation)`.

### 6.2 Phase 0c 신규 폐기 2건

| sub-hypothesis | 출처 | verdict |
|----------------|------|---------|
| `P0c-format-defect` (PST 파일 format / endianness / header / 단위 mismatch) | P0c-1 (`8ec97f5`) | **REFUTED** (3 EQ + 1 UNDETERMINED, NE 0건) |
| `P0c-want-stage-defect` (S\* ≠ postX2 가설) | P0c-2 (`aeee9e9`) | **REFUTED** (S\* = postX2 confirmed) |

### 6.3 보조 폐기 (cross-vector 결과)

| sub-hypothesis | 근거 | verdict |
|----------------|------|---------|
| `P0c-uniform-delta` (sample-uniform constant Δ across all vectors) | P0c-3 (Δ s5..7: ALGTHM (+3,+3,+3) ≠ SPEECH (+2,0,0) ≠ FIXED/PITCH (+1,+1,+1)) | **REFUTED** |
| `P0c-ALGTHM-isolated` (ALGTHM frame 0 단독 현상) | P0c-3 (4/4 vector frame 0 NE) | **REFUTED** |

**누적 18건 폐기 + 식별 결함 0건** (mechanism 후보 = NEW (P0c-inter-subframe-postfilter-state) 측정 cycle 진입 필요).

---

## 7. 19-gate 상태 dump

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | Phase 1a~1j 누적 16건 | **PASS** | 변동 없음. |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) | **RED 잔존 (의도)** | mechanism 미식별 — Phase 1k 잠정 종결 marker. fix scope 외 (E3). |
| 18 | F-non-prelim-X-split bundle (Cα fcb + Cβ gain) | **PASS** | aa9dcf9 시점 promotion. |
| 19 | P0c-reentry measurement bundle (P0c-1 + P0c-2 + P0c-3) | **pending — auto-promote 금지 (E5)** | 본 보고서 §0.2 권고: **명시 사용자 게이트 후 promotion 결정**. 후보 promotion 형태 = 3 phase0c diagnostic test 의 classifier verdict 회귀 보호. |

---

## 8. Plan-allowed FAIL 목록 (regression baseline, 변동 없음)

| FAIL | 위치 | 분류 |
|------|------|------|
| `TestDiagnostic_SinglePulseChain` | `internal/decoder/diagnostic_singlepulse_test.go` | gain log-domain 14 dB 진단 잔존 |
| `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | `internal/decoder/stagef_octpostfix_regression_test.go` | gate 17 = Phase 1k 잠정 종결 marker (의도 RED) |
| `TestDecode_LowEnergyCodebookIsSmooth` | `internal/gain/pathological_test.go` | gain edge saturation |
| `TestDecode_SucceedsAcrossAllGainIndices` | `internal/gain/pathological_test.go` | (GA, GB) edge saturation matrix |

---

## 9. Mechanism 후보 (NEW 시나리오 후속)

### 9.1 Primary mechanism candidate: **inter-subframe postfilter state propagation (sample 40 boundary)**

- **근거**: P0c-3 high-energy vector (FIXED/PITCH) differ-cluster 가 sample 40 (subframe-1 → subframe-2 boundary) 에서 interior 진입.
- **추정 매커니즘**: postfilter chain (long-term `Hp` memory + short-term `Hf` memory + tilt γ_t state + AGC gain state) 의 subframe 경계 (i=39 → i=40) 에서 state carryover / reset policy 가 spec 와 균열.
- **spec 인용 후보** (실제 verbatim 추출은 차기 cycle HP-1 task):
  - §4.2.1 long-term postfilter — pitch lag dependent FIR memory, subframe-by-subframe update.
  - §4.2.2 short-term postfilter — A_q(z) IIR memory, subframe-by-subframe coefficient switch.
  - §4.2.3 tilt compensation — γ_t gating 정책 (SF-1 별도 sub-hypothesis 와 동일 영역).
  - §4.2.4 AGC — gain smoothing memory, subframe-by-subframe.
  - §A.4.2 — Annex A 단순화 buf, init / propagation policy 명시.

### 9.2 Secondary mechanism candidate: **HP filter (§A.4.2.5) frame-edge transient init**

- **근거**: P0c-3 low-energy vector (ALGTHM/SPEECH) differ-cluster 가 frame 시작 `[0..21]` + 종료 `[65..79]` 영역 boundary-only.
- **추정 매커니즘**: §A.4.2.5 HP filter (2nd-order IIR, ×2 multiplier) 의 frame-0 zero-state init 가 ITU 가정 init state (e.g., 추가 warm-up sample, 또는 전 frame DC 누적) 와 균열.
- **spec 인용 후보** (verbatim 추출 차기 cycle HP-2):
  - §A.4.2.5 / §4.2.5 — HP filter 정의 + ×2.

### 9.3 모순 evidence 부재 확인

- middle `[22..64]` 영역 (low-energy vector) 완전 EQ → chain steady-state (frame middle subframe-1 stable region) 가 spec 정합 → mechanism 이 transient / boundary 영역에 국한.
- 본 mechanism 후보 = Phase 1k 16 sub-hypothesis (모두 sample 5..7 단일 시점 + chain stage 단일 절단 측정) 가 측정하지 못한 **시간 축 (subframe / frame boundary) state 전이** 영역에 위치 → 후보 공간 재개 정당화.

---

## 10. 차기 cycle 권고 — `F-non-Hpost` (Phase 1l 진입 후보)

### 10.1 Cycle 명 + 목적

**Cycle ID**: `F-non-Hpost` (Phase 1l 1번째 cycle 후보, NEW (P0c-inter-subframe-postfilter-state) 시나리오 구체화)
**목적**: postfilter chain state vector + HP filter state vector 의 subframe / frame 경계 전이 정책을 spec verbatim 인용 하 측정 + production 정합성 EQ/NE 도출.

### 10.2 3-task 윤곽 (실제 plan 파일은 본 commit 외 별도 dispatch — G-XS4 사용자 게이트 후)

#### **HP-1**: subframe 경계 postfilter state 측정 (i=39 → i=40)

- **scope**: ALGTHM + FIXED + PITCH frame 0 sf-1 (sample 0..39) 과 sf-2 (sample 40..79) 경계에서 postfilter 4 sub-state 의 carryover vs reset 정책 측정.
  - long-term `Hp` memory (pitch lag dependent FIR delay buffer)
  - short-term `Hf` memory (A_q(z) IIR delay buffer)
  - tilt γ_t state (gating decision + previous coefficient)
  - AGC gain state (gain smoothing previous output)
- **dump**:
  - 각 sub-state 의 i=39 시점 vector 와 i=40 시점 vector hex / int dump.
  - reset 가설 (sf 경계에서 zero-state) vs carryover 가설 (sf 경계에서 i=39 state 그대로) 두 reference 와 production 비교.
- **spec verbatim 인용 의무**:
  - §4.2.1 / §4.2.2 / §4.2.3 / §4.2.4 init policy verbatim.
  - §A.4.2 (Annex A 단순화) verbatim.
- **classifier**: `classifyHpostStateBoundary()` → 4 sub-state × {EQ-reset, EQ-carryover, NE} 4-tuple verdict.
- **commit message**:
  ```
  test(decoder): add Phase 1l-Hp1 inter-subframe postfilter state diagnostic
  ```

#### **HP-2**: HP filter frame 경계 state 측정

- **scope**: ALGTHM + SPEECH frame 0 의 HP filter (§A.4.2.5) 의 frame-0 init state (zero-state 가정) 측정.
  - HP filter 2nd-order IIR delay buffer (`y[n-1], y[n-2]` 또는 `mem_pre[2]`) frame 0 초기값 dump.
  - boundary-cluster sample (`[0..21] ∪ [65..79]`) 영역의 HP filter 입력 vs 출력 trace.
  - reference: zero-state init vs warm-up init (가정 — 전 frame 의 DC 누적 또는 ITU spec 의 다른 init 정책) 비교.
- **dump**:
  - frame 0 sample 0..21 의 (sPf input, postHP output, postX2 output) 3-row trace.
  - frame 0 sample 65..79 의 동일 trace.
  - reference inline replay (zero init) 와 production HP state 정합성 EQ/NE.
- **spec verbatim 인용 의무**:
  - §A.4.2.5 / §4.2.5 verbatim ("multiplied by a factor 2 to restore the input signal level" + HP filter coefficients verbatim).
- **classifier**: `classifyHpFrameEdge()` → 2 sample-cluster × {EQ-zeroInit, NE} 2-tuple verdict.
- **commit message**:
  ```
  test(decoder): add Phase 1l-Hp2 HP filter frame-edge state diagnostic
  ```

#### **HP-3**: synthesis + 결정 (fix cycle vs alternative path)

- **scope**: HP-1 + HP-2 verdict 결합 → 3-시나리오 결정 트리:
  - **(Hpost-state-defect)** HP-1 ≥1 NE → postfilter sub-state fix cycle (별도, 1~2 cy).
  - **(HP-edge-defect)** HP-1 EQ_ALL + HP-2 NE → HP filter init policy fix cycle (별도, 1 cy).
  - **(Hpost-refute)** HP-1 + HP-2 모두 EQ → spec-내부 mechanism 후보 재고갈 → alternative path 재선택 (옵션 b Phase 1g 깊이 확장 / 옵션 c spec corrigendum 재검색).
- **commit message**:
  ```
  docs(plans): F-non-Hpost synthesis + Phase 1l decision
  ```

### 10.3 비용 추정

- 본 cycle (P0c-reentry) + F-non-Hpost = **2~4 cycle 누적** 으로 mechanism 식별 또는 후보 공간 재고갈 verdict 확정 가능.
- E2 invariant 유지 — 본 권고 cycle 도 측정-only.

---

## 11. Side-finding catalog (carry + 신규)

| ID | 내용 | spec § / 출처 | sign 영향 | disposition |
|----|------|---------------|-----------|-------------|
| SF-1 | tilt γ_t gating (carry from F-non-Cgamma-revisit synthesis) | `internal/postfilter/tilt.go` vs §4.2.3 | sample 5..7 부호 무관 (F-oct-postfix-2 입증) | F-non-Hpost cycle HP-1 의 tilt γ_t state dump 와 결합 측정 권고 (F-non-Hpost 종결 후 별도 처리). |
| SF-2 | gate 17 RED disposition 3-옵션 (carry) | `internal/decoder/stagef_octpostfix_regression_test.go` | RED — direct reference vector mismatch | **NEW 시나리오 확정 → 옵션 (iii) want 재해석 후 재평가** 의 후속 = F-non-Hpost cycle 종결 시점에서 disposition 재결정 (자동 fix 금지). |
| SF-3 (NEW) | sample-unit UNDETERMINED (P0c-1 4 차원 중 unit 차원) | PDF §A.4.2.5 + READMETV.txt 모두 Q-format 명시 부재 | sign 영향 미정 | F-non-Hpost cycle scope 외. spec source 확장 (옵션 c) 과 결합 검토 권고. |
| SF-4 (NEW) | low/high energy split (P0c-3 cross-vector) | P0c-3 verdict matrix | mechanism 강력 signal | F-non-Hpost cycle HP-1 (high-energy interior) + HP-2 (low-energy boundary) 양 task 분리 측정 핵심 evidence. |

---

## 12. 열린 follow-up

| ID | 내용 | tracking |
|----|------|----------|
| FU-1 | F-bis-1 / F-tris diagnostic (`stagef_bis_diagnostic_test.go`, untracked, Phase 0.5 보존) | F-non-Hpost cycle 진입 시 commit / 폐기 결정 |
| FU-2 | tilt γ_t gating SF-1 별도 cycle | F-non-Hpost HP-1 결합 측정 후 결정 |
| FU-3 | gate 19 promotion 명시 사용자 게이트 (E5) | 본 보고서 §7 권고 |
| FU-4 | gate 17 RED disposition (SF-2) | F-non-Hpost 종결 후 재결정 |
| FU-5 | OVERFLOW.BIT loader bug | Phase 1k post-closure 별도 cycle (carry) |
| FU-6 | `decode_test.go` 8 `t.Skip` 해제 | Phase 1k post-closure (carry) |
| FU-7 | 3 plan-allowed FAIL (SinglePulseChain / LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices) | Phase 1k post-closure (carry) |
| FU-8 (NEW) | sample-unit UNDETERMINED (SF-3) → spec source 확장 (옵션 c) 와 결합 | F-non-Hpost 종결 후 |

---

## 13. 본 cycle 종결 산출

### 13.1 plan checkbox

본 commit 후 plan `2026-05-05-phase0c-reentry-want-domain-reinterpret-plan.md` §Task 진행 status:
- [x] Task 1 — P0c-1 (`8ec97f5`).
- [x] Task 2 — P0c-2 (`aeee9e9`).
- [x] Task 3 — P0c-3 (`68a7df9`).
- [x] **Task 4 — synthesis (본 보고서)** — NEW (P0c-inter-subframe-postfilter-state) 시나리오 확정 + F-non-Hpost cycle 권고.

### 13.2 회귀 게이트 검증

```
$ go vet ./...        → clean
$ go test ./... -race → 4 pre-existing FAIL 잔존 (변동 없음):
                         - TestDiagnostic_SinglePulseChain
                         - TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput  (gate 17)
                         - TestDecode_LowEnergyCodebookIsSmooth
                         - TestDecode_SucceedsAcrossAllGainIndices
```

baseline 변동 0. production 변경 0. test 변경 0 (본 task = docs only).

### 13.3 사용자 게이트 의무 (다음 dispatch 전)

1. **gate 19 promotion 결정** (E5) — P0c-1+2+3 measurement bundle 회귀 보호 등재 여부 yes/no.
2. **F-non-Hpost cycle 진입 승인** (G-XS4) — §10 3-task 윤곽 기준 별도 plan 파일 생성 dispatch 여부.
3. **gate 17 RED disposition 재결정** (SF-2) — F-non-Hpost 종결 시점까지 잔존 유지 권고.
4. **SF-1 (tilt γ_t) HP-1 결합 측정 vs 별도 cycle priority** 선택.

---

**보고서 종료.** Phase 0c re-entry = **(P0c-inter-subframe-postfilter-state) NEW 시나리오 확정** verdict. 다음 cycle dispatch = 사용자 G-XS4 (F-non-Hpost cycle 진입) 게이트 통과 후.
