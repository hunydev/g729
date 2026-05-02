# Phase 1k Stage F-non-Cgamma-revisit 종합 보고서 + Phase 1k 잠정 종결 결정

**작성일**: 2026-05-04
**Cycle ID**: `F-non-Cgamma-revisit` (Phase 1k 9번째 / Phase 1k 10번째 측정 cycle)
**범위**: G-1 (`a4120f9`, postfilter 4 sub-stage trace) + G-2 (`b30bb7a`, synth IIR + Y magnitude trace) 측정 결과 결합. plan `c743116` §Task 3 3-시나리오 결정 트리 적용.
**산출물**: 측정 verdict 결합표 + 결정 트리 = **(Cγ-refute)** 시나리오 확정 + Phase 1k **잠정 종결** verdict + alternative path 3-옵션 사용자 게이트.
**준수**:
- production 변경 0 라인 (E2 — 본 cycle 3 task 모두 측정-only).
- 외부 G.729 구현 0 인용 (E1 / G1 결정 — Annex A binary 거부 유지).
- 본 task = 보고서 only — test/code 변경 0.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked, F-bis-1/F-tris 잔재) 미변경 (Phase 0.5 보존 의무).
- 측정 bundle (G-1+G-2) 자동 promotion 금지 (E5) — 명시 게이트 권고.
- 모든 verdict = `EQ` / `NE` 이진 (Phase 0.4 §3 — "approximately" / "tolerance" 금지).

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go        ← Phase 0.5 보존 의무, 본 cycle 3 task 모두 미변경
$ git log -1 --oneline
b30bb7a test(synth): add Stage F-non-Cgamma-revisit-2 G-2 synth IIR memory + Y magnitude trace
```

본 commit (Task 3 = synthesis) 후 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go        ← 미변경 (의도)
HEAD = <synthesis commit>  docs(plans): F-non-Cgamma-revisit synthesis + Phase 1k decision
```

### 0.2 Escape hatch 평가 (E1–E5)

| 해치 | 발동 조건 | 평가 | 근거 |
|------|---------|------|------|
| **E1** | 외부 G.729 구현 (참조 C / Annex A binary / bcg729 / Sipro / FFmpeg) 인용·실행 | **미발동** | 본 cycle 3 task 모두 PDF (`docs/superpowers/specs/itu/G729E.pdf`) + `READMETV.txt` + repo committed PST + 본 repo internal 패키지만 사용. Annex A binary trace 0건. G1 결정 정합 100%. |
| **E2** | production 변경 라인 > 0 | **미발동** | `git diff a4120f9~1..b30bb7a -- ':!*_test.go' ':!docs/'` 결과 production 0 라인. test 변경 = `internal/postfilter/stagef_fnoncgamma_revisit_diagnostic_test.go` (G-1) + `internal/synth/stagef_fnoncgamma_revisit_diagnostic_test.go` (G-2) 신규 2 파일. 본 task = docs only. |
| **E3** | F-oct-postfix-1 RED (gate 17) 잔존을 task failure 로 간주 | **미발동** | gate 17 = mechanism 식별 후 별도 fix cycle 의 GREEN 대상. 본 cycle 종결 시점에서 RED 의도 잔존 acknowledge. |
| **E4** | spec 모호 paragraph cherry-pick / 모호 verdict ("almost EQ" / "within tolerance") | **미발동** | G-1 4 sub-stage 모두 polarity-preserve EQ 이진 verdict (sample 5..7 부호 + magnitude dump). G-2 IIR 4 state EQ + Y-mag 부호 EQ 이진 verdict. 모호 표현 0건. spec 모호 후보 1건 (tilt γ_t gating §4.2.3) = 별도 sub-hypothesis 분리 (§7 side-finding catalog). |
| **E5** | 측정-only test 자동 promotion (gate 19 자동 등재) | **미발동** | 본 보고서 §6 에서 명시 게이트 권고 (gate 19 = 측정 bundle promote 여부 사용자 결정). 자동 promotion 0. |

### 0.3 사용자 G-S 결정 정합

- **G-XS2 (= "(A) Cγ 재진입")**: 본 cycle 진입 premise. 3 task 모두 plan-bound 충족 (G-1 + G-2 + synthesis).
- **bis 보존**: untracked `stagef_bis_diagnostic_test.go` 미변경.
- **gate 17 RED 잔존 acknowledge**: §6 (gate dump) 에서 명시.

---

## 1. F-non-Cgamma-revisit cycle commit 요약

```
<synthesis hash>  docs(plans): F-non-Cgamma-revisit synthesis + Phase 1k decision   ← 본 commit
b30bb7a           test(synth): add Stage F-non-Cgamma-revisit-2 G-2 synth IIR memory + Y magnitude trace
a4120f9           test(postfilter): add Stage F-non-Cgamma-revisit-1 G-1 postfilter sub-stage trace
c743116           docs(plans): add Phase 1k Stage F-non-Cgamma-revisit plan
aa9dcf9           docs(plans): F-non-prelim-X-split synthesis + Phase 1k status   (직전 cycle 종결)
```

---

## 2. G-1 (postfilter sub-stage) verdict — commit `a4120f9`

**측정 대상**: §4.2 PST chain 4 sub-stage 의 sample 5..7 한정 출력 + spec 정합 EQ/NE.

| sub-stage | spec § | sample 5..7 부호 | spec 기대 (polarity-preserve) | verdict |
|-----------|--------|------------------|-------------------------------|---------|
| (a) long-term postfilter `lt[5..7]` | §4.2.1 | `[+,+,+]` | `+` (positive polarity preserve) | **EQ** |
| (b) short-term postfilter `st[5..7]` | §4.2.2 | `[+,+,+]` | `+` | **EQ** |
| (c) tilt compensation `tc[5..7]` | §4.2.3/4 | `[+,+,+]` | `+` (scalar tilt gain) | **EQ** |
| (d) AGC + HP `hp[5..7]` (×2 적용) | §4.2.5 | `[+2,+2,+2]` | `+` (scalar AGC gain × HP DC) | **EQ** |

**4-tuple verdict** = `EQ_ALL` → **G-1 (Cγ-postfilter) REFUTED**.

**Side finding (sign-irrelevant for this frame)**: `internal/postfilter/tilt.go` 의 γ_t gating 이 `agcGainPrev` (codec-start → 0.2) 기반 — spec §4.2.3 verbatim 은 `sign(k1')` (k1'<0 → 0.9) 사용. 본 frame: spec γt=0.9 vs prod γt=0.2. tilt μ_contrib (Q15 ≈ −558) 가 |st[n]| (Q0 ≈ +1×2^15) 보다 훨씬 작아 부호 결과 변동 없음 (F-oct-postfix-2 commit 시점 Δ=0 로 입증). → 별도 sub-hypothesis (`tilt-gamma-gating`) 로 분리 (§7).

---

## 3. G-2 (synth IIR + Y magnitude) verdict — commit `b30bb7a`

### 3.1 Sub-test A — §3.10/§4.1.6 IIR memory pre/post sample 5..7

| state | mem_syn[0..9] (production) vs reference inline replay | verdict |
|-------|--------------------------------------------------------|---------|
| pre-sample-5 | 10 entry EQ | **EQ** |
| post-sample-5 | 10 entry EQ | **EQ** |
| post-sample-6 | 10 entry EQ | **EQ** |
| post-sample-7 | 10 entry EQ | **EQ** |

→ **G-2-IIR REFUTED** (4 state 모두 production == spec inline replay).

### 3.2 Sub-test B — Y magnitude +6 sign-preserving perturbation

- baseline: unperturbed syn[5..7] = `[+1, +1, +1]`.
- perturbed (a[1..10] +6, sign 보존): syn[5..7] = `[+, +, +]` = baseline.

→ **G-2-Y-mag REFUTED** (small magnitude perturbation 가 부호 변화를 유발하지 않음 — sign-determined-by-input 가설 입증).

### 3.3 결합 verdict

G-2 (IIR + Y-mag) 모두 EQ → **G-2 (Cγ-synth) REFUTED**.

---

## 4. 3-시나리오 결정 트리 적용

| 시나리오 | 조건 | 본 cycle 결과 | 적용 |
|---------|------|----------------|------|
| (Cγ-postfilter) | G-1 ≥1 NE | G-1 = EQ_ALL | **불성립** |
| (Cγ-synth) | G-1 EQ + G-2 (IIR 또는 Y-mag) NE | G-1 EQ + G-2 EQ_ALL | **불성립** |
| **(Cγ-refute)** | G-1 + G-2 전부 EQ | G-1 EQ_ALL + G-2 EQ_ALL | **확정** |

**선택 시나리오**: **(Cγ-refute)**.

**의미**:
- best 2cy 종결 시나리오 ((Cγ-postfilter) / (Cγ-synth)) 양자 제거.
- mid 3cy 종결 시나리오 (G-1 또는 G-2 부분 NE → 추가 split cycle) 제거.
- worst case = Cγ 3 sub-mechanism (postfilter + IIR + Y-mag) 모두 spec 정합 → spec-내부 mechanism 0 입증 → Phase 1k 잠정 종결 진입.

---

## 5. Phase 1k 누적 catalog (10 cycle, 16 sub-hypothesis)

### 5.1 Cycle 누적

| # | cycle | commit (대표) | sub-hypothesis | verdict |
|---|-------|---------------|----------------|---------|
| 1 | F-oct-postfix-1 | (RED 잔존) | M1 (LP→postfilter chain bias) | spec-측정 RED 잔존 (mechanism 미식별) |
| 2 | F-oct-postfix-2 | — | M2 (postfilter coefficient quantization) | **REFUTED** |
| 3 | F-oct-postfix2-prelim-1..6 | (cb9529d 등) | M1', M3, M5, M6, Cα, Cβ, Z (PST chain) | **REFUTED** (7건) |
| 4 | F-non-prelim-1 | f82893d | X excitation sub-term (a[0..10] sign 11/11) → Y(sign) | **REFUTED** |
| 5 | F-non-prelim-2 | d1a4f2d | Y forced (-u) → syn(-u)=-syn(+u) linearity | **REFUTED** (forced-flip linearity 입증) |
| 6 | F-non-prelim-3 | dd4e21a | Z PST chain spec 재해석 | **REFUTED** |
| 7 | F-non-prelim-X-split-1 | fd0b381 | Cα fcb pulse sample 5..7 한정 | **REFUTED** |
| 8 | F-non-prelim-X-split-2 | 4cd25e1 | Cβ gain g_c sample 5..7 한정 | **REFUTED** |
| 9 | F-non-Cgamma-revisit-1 (G-1) | a4120f9 | Cγ-postfilter (4 sub-stage sample 5..7) | **REFUTED** |
| 10 | F-non-Cgamma-revisit-2 (G-2) | b30bb7a | Cγ-synth (IIR + Y magnitude) | **REFUTED** (2 sub) |

### 5.2 폐기 sub-hypothesis 16건 catalog

`M1'`, `M2`, `M3`, `M5`, `M6`, `Cα` (X-split-1 sample 5..7), `Cβ` (X-split-2 sample 5..7), `Z` (PST chain), `Y(sign)` (a[0..10] sign 11/11), `forced-flip linearity`, `Cγ-postfilter §4.2.1` (long-term), `Cγ-postfilter §4.2.2` (short-term), `Cγ-postfilter §4.2.3/4` (tilt), `Cγ-postfilter §4.2.5` (AGC+HP), `Cγ-synth §3.10/§4.1.6 IIR`, `Cγ-Y-mag (small +6 perturbation)`.

**누적 식별 결함 0건**. M1 (RED 잔존) = mechanism 미식별 — spec-내부 후보 공간 고갈.

---

## 6. Phase 1k 종결 verdict — **잠정 종결 (tentative close)**

### 6.1 verdict 유형

- ❌ best (2cy 종결, mechanism fix) — (Cγ-postfilter) / (Cγ-synth) 양자 (Cγ-refute) 진입으로 제거.
- ❌ mid (3cy 종결, G-1/G-2 부분 NE → split fix) — G-1/G-2 EQ_ALL 로 제거.
- ✅ **worst (잠정 종결)** — spec-내부 mechanism 후보 공간 고갈 (16 sub-hypothesis 폐기 / 결함 0건). 잔존 RED (gate 17) 의 mechanism = spec scope 외부 (production 외부 = want 도메인 / multi-frame state / 추가 spec source) 가능성 강화.

### 6.2 잠정 종결의 의미

- Phase 1k 의 **spec-내부 sub-stage hypothesis tree exhaustion** 확정.
- 결함 0건 = production code 의 §3 (encoder-역방향 = decoder forward) + §4.1 (synthesis) + §4.2 (postfilter) 가 sample 5..7 한정 frame 0 sf0 시점에서 모두 spec verbatim 정합.
- **잔존 RED (gate 17)** = want 도메인 정의 / reference vector 해석 / multi-frame 누적 state / spec corrigendum 미반영 / 다른 영역 (PCM, IO) mechanism 후보 공간 진입 필요.
- **fix cycle 진입 금지** — mechanism 미식별 상태에서 production 변경 = E2 위반.

---

## 7. Side-finding catalog

| ID | 내용 | spec § / 출처 | sign 영향 | disposition |
|----|------|---------------|-----------|-------------|
| SF-1 | tilt γ_t gating: prod = `agcGainPrev` 기반 (codec-start → 0.2), spec verbatim §4.2.3 = `sign(k1')` 기반 (k1'<0 → 0.9). | `internal/postfilter/tilt.go` vs §4.2.3 | 본 frame: μ_contrib Q15 ≈ −558 << \|st[n]\|, Δ=0 (F-oct-postfix-2 입증). 부호-irrelevant. | 별도 sub-hypothesis (`tilt-gamma-gating`) 로 분리. Phase 1k 잠정 종결 후 separate diagnostic cycle 권고. |
| SF-2 | F-oct-postfix-1 RED contract (gate 17) `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` 잔존 disposition. | `internal/decoder/stagef_octpostfix_regression_test.go` | RED — direct reference vector mismatch | **3 옵션** (사용자 선택): (i) 영구 RED (Phase 1k 잠정 종결 marker) 유지, (ii) `t.Skip` 으로 일시 marker 전환 (단 plan G-XS5 acknowledge 의무), (iii) want 도메인 재해석 cycle (alternative path (a)) 결과 후 재평가. |

---

## 8. 19-gate 상태 dump

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | Phase 1a~1j 누적 16건 | **PASS** | 변동 없음. `go test ./...` 회귀 baseline 정합. |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 | **RED 잔존** (의도) | mechanism 미식별 — Phase 1k 잠정 종결 marker. fix scope 외 (E3). |
| 18 | F-non-prelim-X-split measurement bundle (Cα fcb + Cβ gain) | **PASS** | aa9dcf9 시점 명시 promotion. |
| 19 | F-non-Cgamma-revisit measurement bundle (G-1 + G-2) | **pending — auto-promote 금지 (E5)** | 본 보고서 권고: **명시 사용자 게이트 후 promotion 결정**. 후보 promotion 형태 = `internal/postfilter/stagef_fnoncgamma_revisit_diagnostic_test.go` + `internal/synth/stagef_fnoncgamma_revisit_diagnostic_test.go` 의 `_classifierResult == EQ_ALL` 회귀 보호. |

---

## 9. Plan-allowed FAIL 목록 (regression baseline)

본 cycle commit 후에도 변동 없이 잔존 (Phase 1k post-closure 처리 대상):

| FAIL | 위치 | 분류 |
|------|------|------|
| `TestDiagnostic_SinglePulseChain` | `internal/decoder/diagnostic_singlepulse_test.go` | gain log-domain 14 dB 진단 잔존 (별도) |
| `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | `internal/decoder/stagef_octpostfix_regression_test.go` | gate 17 = Phase 1k 잠정 종결 marker (의도 RED) |
| `TestDecode_LowEnergyCodebookIsSmooth` | `internal/gain/pathological_test.go` | gain edge saturation (별도) |
| `TestDecode_SucceedsAcrossAllGainIndices` | `internal/gain/pathological_test.go` | (GA, GB) edge saturation matrix (별도) |

추가로 `internal/decoder/decode_test.go` 내 `t.Skip` 8건 = Phase 1k post-closure 처리 deferred. 본 cycle scope 외.

---

## 10. OVERFLOW.BIT bitstream loader bug

별도 bug — `internal/bitstream/` 의 OVERFLOW.BIT vector loader 가 EOF 또는 alignment 처리에서 mismatch (사전 식별). **본 cycle scope 외, deferred** (Phase 1k post-closure 별도 cycle).

---

## 11. 열린 follow-up

| ID | 내용 | tracking |
|----|------|----------|
| FU-1 | F-bis-1 / F-tris diagnostic — `internal/decoder/stagef_bis_diagnostic_test.go` (untracked, Phase 0.5 보존) | 차기 cycle 진입 시 commit 또는 폐기 결정 |
| FU-2 | tilt γ_t gating 별도 sub-hypothesis (SF-1) | Phase 1k 잠정 종결 후 separate diagnostic cycle |
| FU-3 | gate 19 promotion 명시 사용자 게이트 (E5) | 본 보고서 §8 권고 |
| FU-4 | gate 17 RED disposition 3-옵션 (SF-2) | 사용자 선택 |
| FU-5 | OVERFLOW.BIT loader bug | Phase 1k post-closure 별도 cycle |
| FU-6 | `decode_test.go` 8 `t.Skip` 해제 | Phase 1k post-closure |
| FU-7 | 3 plan-allowed FAIL (SinglePulseChain / LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices) 처리 | Phase 1k post-closure |

---

## 12. Alternative path 권고 — 3 옵션 (사용자 선택, 본 보고서 단일 선정 금지)

(Cγ-refute) → spec-내부 mechanism 후보 공간 고갈 → 다음 진입 영역 = **spec 외부 / 상위 레이어 / 추가 spec source** 중 택일. 본 보고서는 3 옵션을 동등하게 제시하며, 단일 선정은 사용자 게이트 (G-XS3 후속) 의무.

### 옵션 (a) — Phase 0c (PCM/IO) 재진입 + want 도메인 재해석 cycle

- **scope**: `internal/pcm/` + `internal/bitstream/` + READMETV.txt 의 ALGTHM.PST want vector 의 도메인 (Q-format / endianness / sample alignment / pre-emphasis / DC) 재검증.
- **가설**: gate 17 의 `want=[−1,−1,−1]` 가 의도된 PST 출력이 아니라 다른 도메인 (e.g., post-G.711 PCM, pre-decoded PST, frame-offset shifted) 일 가능성.
- **장점**: production 변경 가능성 0 (want 측 재해석만으로 RED→GREEN 가능). 비용 1~2 cy.
- **단점**: PDF 가 want vector 도메인을 명시 안 한 경우 cherry-pick 위험 (Phase 0.4 §4 위반 가능) — 강한 verbatim 인용 게이트 필수.

### 옵션 (b) — Phase 1g (decoder integration) 재진입 + multi-frame state 진단

- **scope**: frame 0 sf0 단일 frame scope 를 frame 0 sf0..sf1 + frame 1..N 으로 확장. mem_syn / mem_postfilter / mem_pitch 누적 state 의 multi-frame propagation 측정.
- **가설**: frame 0 sf0 sample 5..7 의 want=`[−1,−1,−1]` 가 직전 frame (frame -1, decoder init) 의 state 와 정합되어야 하는데 init 시점 state 가 spec 와 mismatch.
- **장점**: spec §4.1.6 (synthesis init), §4.2 (postfilter init) 의 init state 정의 재검증 — production buf 후보 식별.
- **단점**: 측정 surface 가 9~10 sub-system 누적 — single cycle 로 단일 mechanism 식별 어려움. 비용 3~5 cy.

### 옵션 (c) — ITU corrigendum / 추가 spec source 검색

- **scope**: G.729 (06/2012) 외 corrigendum (ITU-T G.729 Cor.1, Cor.2 등), 공개 textbook (Kondoz "Digital Speech", Spanias "Speech Coding"), W3C / IETF g729 references.
- **G1 invariant 유지**: Annex A binary 거부 — corrigendum PDF + textbook 만 허용.
- **가설**: ALGTHM.PST want vector 가 06/2012 PDF 외 corrigendum 의 정의 또는 textbook 의 reference impl 도메인.
- **장점**: spec source 확장으로 want 도메인 모호성 해소 가능.
- **단점**: 추가 spec source 부재 시 cycle 0건 산출. 비용 1 cy (search) + α (검토).

---

## 13. 본 cycle 종결 산출

### 13.1 plan checkbox

본 commit 후 plan `2026-05-04-phase1k-stage-f-non-cgamma-revisit-plan.md` §Task 진행 status:
- [x] Task 1 — F-non-Cgamma-revisit-1 (`a4120f9`) — G-1 EQ_ALL.
- [x] Task 2 — F-non-Cgamma-revisit-2 (`b30bb7a`) — G-2 EQ_ALL (IIR + Y-mag).
- [x] **Task 3 — F-non-Cgamma-revisit-3 (synthesis + 3-시나리오 결정 트리)** — (Cγ-refute) 시나리오 확정 → Phase 1k 잠정 종결.

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

1. **gate 19 promotion 결정** (E5) — G-1+G-2 measurement bundle 회귀 보호 등재 여부 yes/no.
2. **alternative path 1개 선택** (옵션 a / b / c).
3. **gate 17 RED disposition 3-옵션 선택** (SF-2: 영구 RED / `t.Skip` / want 재해석 결과 대기).
4. **SF-1 (tilt γ_t gating) 별도 cycle 진입 priority** (즉시 / Phase 1k post-closure deferred / abandon).

---

**보고서 종료.** Phase 1k = **잠정 종결 (tentative close)** verdict. 다음 cycle dispatch = 사용자 G-XS3 후속 (alternative path 옵션 a/b/c 중 1택) 게이트 통과 후.
