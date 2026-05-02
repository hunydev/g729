# Phase 1n R-C-empirical 종합 보고서 + 차기 cycle 권고

**작성일**: 2026-05-08
**Cycle ID**: `R-C-empirical` (Phase 1n 1번째 cycle, CE-4 권고 ranked path
**(c-R-C-empirical)** — Phase 1m R-C inventory 항목의 empirical disposition
by branch-test).
**범위**: RC-1 (`a47f03f`, sf-1 LSP interpolation rounding-mode branch-test
on ALGTHM frame 0 sample 5..7) + RC-2 (`b1412d4`, pitch pre-emphasis
residue-0 sub-measurement) 결합. plan `ea844d6` §Task RC-3 3-시나리오 결정
트리 적용.
**산출물**: 3-시나리오 결정 트리 적용 → **Refute** 시나리오 확정 + Phase 1n
RC-3 cycle-end **mandatory knob retirement** 실행 (E2' commitment) + 누적
catalog 25 → 30 sub-hypothesis (Phase 1m 3 + Phase 1n 2) + I-5 hard-spec
invariant 신규 후보 (§3.8 eq.(48) `n = T,...,39` 루프 바운드) + R-C
deprioritized (sample 5..7 mechanism 후보에서 empirically 제외) + gate 17
reactivation map 갱신 ((c) corrigendum trigger 가 §3.10 / §A.4.* specific
target 보유) + 차기 cycle 권고 ordering + `(d-final)` 사용자 게이트 후보
발의.

**선행 commit**:
- `b1412d4` — Phase 1n RC-2 pitch pre-emphasis residue-0 diagnostic.
- `a47f03f` — Phase 1n RC-1 sf-1 LSP interp rounding branch-test
  (REFUTE_unchanged).
- `ea844d6` — Phase 1n Stage R-C-empirical cycle plan.
- `21894d3` — Phase 1m F-Cγ-elsewhere synthesis + ambiguous verdict.
- `9ab1c91` — gate 17 disposition (Phase 1l alt-path d-i, t.Skip).

**준수**:
- Production line 변경 = 0 (cycle-진입 시점과 byte-EQ). RC-1 commit 의
  knob/branch/setter/SPEC-doc 추가는 본 cycle (RC-3) 에서 mandatory
  retirement 수행 → `internal/lsp/interpolate.go` 의 expression 은
  cycle-진입 시점 (`a47f03f` parent) 의 single-line floor `>>1` 로 원복.
  SPEC verbatim doc-comment block 은 가치 있는 spec 문서로 유지하되
  prose 만 RC-1 empirical 결과 기록으로 갱신.
- 외부 G.729 구현 0 인용 (E1) — RC-1/RC-2 모두 PDF + repo internal
  패키지만 사용.
- RC-1 측정 test 는 knob 부재 시 컴파일 불가 → Option A (delete) 채택,
  본 commit 에서 `git rm`. 측정 record 는 `a47f03f` 의 commit history
  + 본 보고서 §3 의 raw 표로 보존.
- RC-2 측정 test (`phase1n_rc2_pitch_preemphasis_diagnostic_test.go`) 는
  production touch 0 (measurement-only) → 그대로 유지.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경
  (Phase 0.5 보존 의무).
- 측정 bundle (RC-1/RC-2) 자동 promotion 금지 (E5).
- 모든 verdict = `EQ` / `NE` / `REFUTE_unchanged` / `assertion-pass`.

---

## 0. Working tree + escape hatch 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go  ← Phase 0.5 보존 의무
$ git log -1 --oneline
b1412d4 test(decoder): add Phase 1n RC-2 pitch pre-emphasis residue-0 sub-measurement
```

본 commit (RC-3 = synthesis + knob retirement + gate 17 docstring) 후
working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go  ← 미변경 (의도)
HEAD = <RC-3 commit>  docs(plans),refactor(lsp): Phase 1n RC-3 synthesis +
                                                knob retirement (REFUTE)
```

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 평가 | 근거 |
|------|-----------|------|------|
| **E1** | 외부 G.729 구현 인용·실행 | **미발동** | RC-1/RC-2 모두 PDF (`G729E.pdf` §3.2.5 / §3.2.6 / §3.8 eq.(48)) + repo internal 패키지만 사용. ITU reference C / Annex A binary / bcg729 / Sipro / FFmpeg G.729 인용 0건. |
| **E2** | production 변경 라인 > 0 (cycle 진입↔종료 비교) | **미발동** | RC-1 의 knob/branch/setter는 본 RC-3 commit 에서 retire → cycle-진입 시점 (`a47f03f` parent) 과 `interpolate.go` byte-EQ. SPEC doc-comment block 은 추가되었으나 (R-C ambiguity + Phase 1n empirical 결과 기록의 valuable doc), behavior 변경 0. E2' relaxation 의 cycle-end commitment 충족. |
| **E3** | gate 17 즉시 reactivation 강행 | **미발동** | REFUTE → reactivation 미발의 (E2' / RC-3 §3 commitment). gate 17 t.Skip 본문 무변경, docstring 만 갱신 (R-C 트리거를 REFUTED 로 강등 + (c) corrigendum specific narrowing). |
| **E4** | spec 모호 paragraph cherry-pick / 모호 verdict | **미발동** | RC-1 = REFUTE_unchanged (3/3 sample byte-EQ across mode flip — 모호 0). RC-2 = assertion-pass (T1=20 ∉ {5,6,7} → mechanism deterministic 제거). I-5 후보는 §3.8 eq.(48) verbatim "n = T,...,39" 인용에 기반. |
| **E5** | 측정-only test 자동 promotion (gate 21 자동 등재) | **미발동** | RC-1 test 는 knob retirement 와 함께 삭제 (Option A). RC-2 test 는 measurement-only 상태로 유지 — gate promotion 발의 0. |

### 0.3 사용자 G-XS6 결정 정합

- **G-XS6 = "(c-R-C-empirical) — empirical branch-test for R-C
  ambiguity"**: 본 cycle 진입 premise. 2 task (RC-1 mandatory + RC-2
  fold-in) 모두 plan-bound 충족.
- **bis 보존**: untracked `stagef_bis_diagnostic_test.go` 미변경.
- **gate 17 SKIP 유지**: REFUTE 로 인해 reactivation 미발의 — t.Skip
  본문 무변경. docstring 갱신은 R-C 트리거 강등 + (c) specific target
  추가 + 누적 카운트 30 update 한정.

---

## 1. R-C-empirical cycle commit 요약

```
<RC-3 hash>  docs(plans),refactor(lsp): Phase 1n RC-3 synthesis +
                                       knob retirement (REFUTE)   ← 본 commit
b1412d4      test(decoder): add Phase 1n RC-2 pitch pre-emphasis
                            residue-0 sub-measurement
a47f03f      test(decoder): add Phase 1n RC-1 sf-1 LSP interp
                            rounding branch-test
ea844d6      docs(plans): add Phase 1n Stage R-C-empirical plan
21894d3      docs(plans): Phase 1m F-Cγ-elsewhere synthesis +
                          ambiguous verdict
```

---

## 2. RC-1 verdict — sf-1 LSP interpolation rounding-mode branch-test (commit `a47f03f`)

### 2.1 측정 raw values (ALGTHM frame 0 sample 5..7)

| n | floor `>>1` (production cycle-entry) | symmetric round (`+1>>1`) | Δ |
|---|--------------------------------------|---------------------------|---|
| 5 | +2 | +2 | 0 |
| 6 | +2 | +2 | 0 |
| 7 | +2 | +2 | 0 |

(post-`ScaleUpSat ×2` int16 PCM domain; cf. §6 plan-slip 항.)

→ **3/3 sample 부호+magnitude 무변동**.

### 2.2 Verdict

**REFUTE_unchanged** (plan §1 결정 트리 "Refute" branch — `sample 5..7
부호 무변동 또는 want 반대 방향`).

### 2.3 Mechanistic 보강

RC-1 측정은 LSP-domain 에서 i=1 / i=5 두 cell 만 +1 LSB drift 가 발생함을
확인한다 (CE-3 finding 재현). 이는 §3.2.6 Chebyshev 전개 후 **a[8..a[10]]
LP 계수만** 변동시킨다 (저-차수 a[1..a[7]] 은 영향 0). 그러나 frame 0 의
synth filter past state 는 zero-init (§4.3 catch-all I-2 보장) → a[8..10]
이 곱하는 모든 항이 0 인 sample 0..9 구간에서 LP 계수 변동은 출력에
도달할 수 없다. 따라서 sample 5..7 은 floor↔symmetric 양 모드에서
mechanistically 동일.

→ **R-C 가 sample 5..7 mechanism 의 후보로 잔존하던 PLAUSIBLE-but-not-
PROVABLE 지위는 본 cycle 로 mechanistically 제거**. R-C 자체는 verbatim
documentation issue 로 잔존하지만, gate 17 와의 인과 link 는 단절.

---

## 3. RC-2 verdict — pitch pre-emphasis residue-0 sub-measurement (commit `b1412d4`)

### 3.1 측정 raw values (ALGTHM frame 0 sf0)

| 변수 | 측정 |
|------|------|
| T1 (integer pitch lag, sf0) | **20** |
| β1 (gain pitch Q14, sf0) | **3277** (= 0.2 Q14) |
| c[5..7] (FCB residue) before pre-emphasis | `[0, 0, 0]` |
| c[5..7] after pitch pre-emphasis loop | `[0, 0, 0]` |

### 3.2 Verdict

**assertion-pass** (plan §Task RC-2 — disjunction `T1 ∉ {5,6,7} ∨ β1 = 0`
의 첫번째 항이 참).

### 3.3 Mechanistic 보강

§3.8 eq.(48) verbatim:

> "c'(n) = c(n) + β·c'(n−T)        n = T, ..., 39"

T1=20 이므로 루프 바운드 `n = T,...,39` 는 n ∈ {20, 21, ..., 39} 만
visit → sample 5..7 은 루프 바운드 외부 → pitch pre-emphasis 가 c[5..7] 에
기여할 mechanism 0. 측정값 c[5..7] (before/after) `[0,0,0]` 가 mechanism
과 byte-EQ 로 부합.

→ **CE-2 c[5..7]=0 측정값의 mechanistic confirmation 완료**. pitch
pre-emphasis 는 sample 5..7 mechanism 후보에서 deterministically 제거.

### 3.4 I-5 hard-spec invariant 신규 후보 promote

§3.8 eq.(48) 의 verbatim `n = T,...,39` 루프 바운드는:
- **§3.8 eq.(48) 본문에 verbatim 명시** (PDF p. 38, R-blocking 부재).
- **production `internal/fcb/enhance.go` 의 루프 바운드와 byte-EQ**
  (RC-2 측정 시 cross-check 완료).
- **항상 frame-0 sample n < T 영역을 mechanism 으로부터 격리** —
  이는 단순한 measurement 가 아니라 spec verbatim 보호 invariant.

→ **본 보고서로 I-5 promote 결정**: §3.8 eq.(48) `n = T,...,39` pitch
pre-emphasis loop bound (+ c[n] = c(n) for n < T implicit by loop
omission). 누적 hard-spec invariant 4 → **5건**.

---

## 4. 3-시나리오 결정 트리 적용 (plan §Task RC-3)

| 시나리오 | 조건 | 본 cycle 결과 | 적용 |
|----------|------|----------------|------|
| **Defect-confirmed** | 3/3 sample 부호 = want `[−1, −1, −1]` (또는 ≥ 2/3) | sym output = floor output (3/3 unchanged) | **NOT APPLICABLE** |
| **Refute** | sample 5..7 부호 무변동 | 3/3 byte-EQ across mode flip | **확정** |
| **Partial** | 1/3 sign change with want partial match | 0/3 sign change | **NOT APPLICABLE** |

**선택 시나리오**: **Refute**.

**의미**:
- Defect 시나리오 deterministically 제거 → R-C 가 sample 5..7 mechanism
  이라는 가설은 empirically 거짓.
- E2' commitment (cycle-end knob retirement) 자동 발동 — 본 RC-3 commit
  에서 수행.
- gate 17 reactivation 미발의 — R-C empirical 트리거는 REFUTED 로 강등.
- 잔존 live lead = (c) corrigendum / Appendix search (이제는 §3.10
  synth.Filter / §A.4.* 구체적 target 보유).

---

## 5. Mechanistic exhaustion 분석 (sample 5..7 sign mismatch, ALGTHM frame 0 sf0)

본 cycle 은 sample 5..7 부호 불일치의 **모든 직접-경로 mechanism 후보**
를 systematic 하게 거짓으로 reduce 한다. 다음 7 경로가 frame-0 sample
5..7 에 도달할 수 있는 모든 결정론적 surface 이다:

| # | mechanism | refuted by | finding |
|---|-----------|------------|---------|
| (i) | FCB direct (c[n] for n=5..7) | Phase 1m CE-2 (`0d58ca6`) | PROD/SPEC 양쪽 c[5]=c[6]=c[7]=0; ALGTHM frame 0 sf0 의 4 pulse position 셋 ∩ {5,6,7} = ∅. |
| (ii) | Pitch pre-emphasis (`c'(n) = c(n) + β·c'(n−T)`) | **Phase 1n RC-2 (`b1412d4`)** | T1=20 → 루프 `n=T..39` 가 n ∈ {20..39} 만 visit; 5..7 비방문. I-5 invariant 후보. |
| (iii) | LSP rounding ripple (sf-1 floor↔symmetric) | **Phase 1n RC-1 (`a47f03f`)** | a[1..a[7]] 무변동 (i=1,5 cell 만 LSP +1 LSB drift), a[8..10] 변동분은 frame-0 zero past-state 곱셈으로 흡수 → sample 5..7 영향 0. |
| (iv) | Postfilter / HP downstream chain | Phase 1k F-* (16) + Phase 1l HP-1/HP-2 (2) | 22 sub-hypothesis 폐기 across `d448282`/`f902bd9`. AGC carryover, IIR pole-pair, catch-all zero-init invariant 모두 byte-EQ. |
| (v) | gain VQ Imap+GBK (`ĝ_p Q14 = +1995`) | Phase 1l carry + Phase 1m CE-1 (`f3df272`) | F-non-prelim-X-split-2 측정값 `g_p=+1995, g_c=+4153` spec 정합; Imap-applied indexing 존재 + eq.(73)-(74) saturation 검증. R-A 부재 cell 6건 surface 좁음. |
| (vi) | FCB sign-bit ordering (R-B 모호 surface) | Phase 1m CE-2 (`0d58ca6`) | hard Table 7 track-residue invariant 가 양 해석에서 동일 → c[5..7]=0 결정. R-B verbatim 부재이지만 sample 5..7 영향 0. |
| (vii) | LSP MA predictor + init constants | Phase 1m CE-3 (`5232411`) | 63 cell byte-EQ; I-4 hard-spec invariant `l̂_i = i·π/11` Q13 + `q_i = cos(i·π/11)` Q15. |

→ **frame-0 결정론 surface 7 경로 전부 mechanistically 제거 / byte-EQ
확인**. Spec-internal 직접-경로 후보 공간 = 빈집합.

---

## 6. R-blocking 인벤토리 갱신 (Phase 1m → Phase 1n)

| ID | 위치 (spec §) | Phase 1m 후 status | Phase 1n 후 status (본 cycle) |
|----|---------------|---------------------|-------------------------------|
| **R-A** | §3.9.3 (Imap reorder map values) | confirmed-blocking, sample 5..7 surface 좁음 | **unchanged** — 직접 측정 부재. |
| **R-B** | §3.8.2 (sign+position bit-string layout) | confirmed-blocking, sample 5..7 영향 0 (CE-2) | **unchanged** — 영향 0 재확인. |
| **R-C** | §3.2.5 eq.(24) sf-1 rounding mode | confirmed-blocking, PLAUSIBLE-but-not-PROVABLE sample 5..7 mechanism 후보 | **EMPIRICALLY DISPROVEN** as sample 5..7 mechanism (RC-1 `a47f03f`). R-C verbatim 부재 자체는 잔존하지만 gate 17 와의 인과 link 단절 → **deprioritized**. |

---

## 7. Hard-spec invariant 누적 (4 → 5)

| # | invariant | 출처 § / verbatim | 출처 cycle |
|---|-----------|-------------------|-----------|
| I-1 | postfilter `agcGainPrev` 의 subframe 경계 carryover | §4.2.4 | HP-1 (`076b6de`) |
| I-2 | §4.3 catch-all zero-init (Table 9 비-등재 변수) | §4.3 lines 1696..1707 | HP-2 (`2ee0009`) |
| I-3 | §A.4.2.5 IIR pole-pair impulse decay (1.93/-0.94) | §4.2.5 / §A.4.2.5 | HP-2 (`2ee0009`) |
| I-4 | §3.2.4 init formulas (`l̂_i = i·π/11` Q13 + `q_i = cos(i·π/11)` Q15) | §3.2.4 + §4.3 + eq.(18) | CE-3 (`5232411`) |
| **I-5 (NEW)** | §3.8 eq.(48) pitch pre-emphasis loop bound `n = T,...,39` (n < T 격리: sample 5..7 with T≥8 mechanism 0) | §3.8 eq.(48), PDF p. 38 verbatim | **RC-2 (`b1412d4`)** |

I-5 promotion 정당화: (a) verbatim 인용 모호 0건 (eq. 번호 + 루프 바운드
양쪽 spec 명시), (b) production `enhance.go` 와 byte-EQ cross-check 완료,
(c) RC-2 측정에서 결정론적 mechanism elimination 입증.

→ **5 hard-spec invariants 누적, 모두 byte-EQ 직접 매핑**.

---

## 8. 누적 폐기 catalog (25 + 2 Phase 1n = **30 sub-hypothesis**)

### 8.1 Phase 1l + Phase 1m carry (25건)

- Phase 1k F-* (16) — `d448282`.
- Phase 0c P0c-1/2/3 (4) — `8e6386c`.
- Phase 1l HP-1 / HP-2 (2) — `f902bd9`.
- Phase 1m CE-1 / CE-2 / CE-3 (3) — `21894d3`.

### 8.2 Phase 1n 신규 폐기 2건 (본 cycle)

| sub-hypothesis | 출처 | verdict |
|----------------|------|---------|
| `RC-1-LSP-interp-rounding-defect` (sf-1 LSP floor vs symmetric round 에 의한 sample 5..7 부호 mechanism) | RC-1 (`a47f03f`) | **REFUTED** (3/3 sample byte-EQ across mode flip; mechanistic argument: a[8..10] 곱셈 흡수). |
| `RC-2-pitch-preemphasis-residue-defect` (pitch pre-emphasis 가 c[5..7] 에 기여하여 sample 5..7 부호 변경) | RC-2 (`b1412d4`) | **REFUTED** (T1=20 → loop bound `n=T..39` 가 5..7 비방문; I-5 invariant 후보 도출). |

> **누적 (Phase 1l/1m 25 + Phase 1n 2)**: **30 sub-hypothesis 폐기,
> defect = 0, hard-spec invariant 매핑 = 5건 (I-1..I-5)**.

---

## 9. 19-gate 상태 dump

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | Phase 1a~1j 누적 16건 | **PASS** | 변동 없음. |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 | **SKIP** (`9ab1c91` t.Skip 유지) | docstring 갱신 (§10) — R-C 트리거 REFUTED 강등 + (c) corrigendum specific 추가 + 누적 30 update. t.Skip 본문 + 테스트 본문 무변경. |
| 18 | F-non-prelim-X-split bundle | **PASS** | 변동 없음. |
| 19 | P0c + Phase 1l measurement bundle | **pending** (E5 미수행) | 변동 없음. |
| 20 (잠정) | Phase 1m CE-1/2/3 measurement bundle | **pending — auto-promote 금지 (E5)** | G-XS6 후 promotion 결정 — 본 cycle 미발의. |
| 21 (잠정) | Phase 1n RC-1 / RC-2 measurement bundle | **N/A (RC-1 deleted, RC-2 retained as measurement-only)** | RC-1 test = knob retirement 와 함께 삭제 (Option A); RC-2 test = retained (production touch 0). gate 21 promotion 미발의. |

---

## 10. Gate 17 reactivation map 갱신

본 cycle 결과로 `9ab1c91` 의 REACTIVATION TRIGGERS 섹션이:

| 원본 트리거 (Phase 1m 후) | Phase 1n 후 update | 정당화 |
|---------------------------|---------------------|--------|
| (c) corrigendum / Appendix I/II/III (R-A/R-B/R-C 3 specific target) | **(c) corrigendum / Appendix search yields a §3.10 synth.Filter or §A.4.* clarification** (R-C deprioritized → 남은 specific target = §3.10 synth.Filter rounding / §A.4.* simplifications) | RC-1 이 R-C 를 sample 5..7 mechanism 에서 empirical 제거. 잔존 live lead 는 §3.10 / §A.4.* (mechanistic exhaustion 후 유일한 외부 source 후보). |
| (a) Phase 1g multi-frame state | **structurally inapplicable to gate 17** (frame 0 only) | gate 17 는 frame 0 sample 5..7 사양 — multi-frame state 는 frame ≥1 만 활성. pre-frame-0 state 는 cold-start 에서 정의 불가. → effectively dead lead. |
| 새 spec source 도입 | 변동 없음 | 일반 trigger 유지. |
| R-C empirical disposition (Phase 1m 신설) | **REFUTED 로 강등** (Phase 1n RC-1) | 본 cycle 결과로 자동 강등. |

**Docstring 갱신** (본 commit 1 hunk, behavior 무변경):
`internal/decoder/stagef_octpostfix_regression_test.go` 의 REACTIVATION
TRIGGERS comment block:
- R-C 4번째 bullet 을 REFUTED 로 강등하고 mechanistic 설명 추가.
- 5번째 bullet (c) corrigendum specific narrowing 추가.
- 누적 카운트 라인 `Cumulative refutations: 30 (was 22 at gate 17
  disposition; +3 Phase 1m, +2 Phase 1n).` 추가.

`t.Skip` 본문 + 테스트 본문 + import + behavior 무변경.

---

## 11. Plan slip 조정 (`[+1,+1,+1]` vs `[+2,+2,+2]`)

RC-1 측정 시점에 plan §1 prose `[+1,+1,+1], Δ=+3` 표기가 RC-1 가
실제로 캡처한 baseline `[+2,+2,+2]` 와 1 LSB 차이로 불일치하는 점이
관찰되었다. 원인:

- **gate 17 docstring + Phase 1l want 추출** 은 pre-`ScaleUpSat ×2`
  PCM domain 의 1 LSB scale 에서 작성 (Phase 1l F-oct-prelim-5-4
  raw 측정 시점).
- **RC-1 캡처** 는 `Decoder.Decode` 의 최종 출력 (post-`ScaleUpSat ×2`
  int16 PCM domain) 에서 측정 → 모든 sample 이 정확히 ×2.

→ 두 표기는 **factor-of-2 consistent**, 결함 아님. 향후 docs / plan 은
domain (pre/post ScaleUpSat) 을 명시 인용 권장. 본 cycle verdict 에는
영향 0 (REFUTE 는 sample 부호 무변동 기준 — 양 domain 동일).

---

## 12. Mechanistic exhaustion 후 잔존 lead (차기 cycle 후보)

§5 의 7 직접-경로 mechanism 모두 거짓 → spec-internal 직접 후보 공간
빈집합. 잔존 lead 는 다음 3 경로로 축약:

| ID | path | 상태 | 비용/정보-수익 |
|----|------|------|----------------|
| **(c-corrigendum-search)** | ITU public corrigendum / Appendix I/II/III for §3.10 synth.Filter rounding 또는 §A.4.* simplifications 의 verbatim clarification | **highest-information remaining lead** | 비용 LOW (외부 PDF 1~2개 fetch + verbatim 검색); 수익 HIGH (specific R-target 보유 — §3.10 synth.Filter rounding 또는 §A.4.x). |
| (a) Phase 1g multi-frame state propagation | gate 17 = frame 0 only — pre-frame-0 cannot exist (cold-start) → **structurally inapplicable** | DEAD for gate 17 (active for 다른 sample, e.g. frame ≥1) | 비용 N/A (gate 17 에 inapplicable). |
| **(d-final)** gate 17 disposition revisit | true mechanistic exhaustion 도달 → t.Skip 영구 유지 + docstring archive 강화, 또는 ITU-T study group 에 escalate, 또는 (필요 시) want-side spec-domain 재해석 | **사용자 게이트 후보** | RC-3 본 commit 에서는 발의만, 결정은 G-XS7 사용자 결정. |

### 12.1 권고 ordering (default)

1. **(c-corrigendum-search)** — 유일한 live spec-internal lead.
   §3.10 synth.Filter / §A.4.* 에 Annex / corrigendum 이 verbatim 부재
   ambiguity 를 해소하는지 확인. 비용 최저, 정보 수익 최고.
2. **(d-final) 사용자 게이트 G-XS7** — (c) 가 NULL result 일 경우, gate
   17 의 영구 disposition 결정 (permanent t.Skip + archived docstring,
   또는 ITU escalation, 또는 want-domain 재해석). 사용자 결정 사안.

(a) 는 gate 17 한정 inapplicable 로 분류 — 다른 sample / frame 의 결함
조사 시 활성화 가능.

---

## 13. Anti-goals (본 cycle 명시 비-수행)

- production code 의 cycle-진입 시점 byte-EQ 외 변경 0 (RC-1 의 knob 은
  본 cycle 에서 retire). SPEC doc-comment block 추가는 documentation
  only — behavior 변경 0.
- 외부 G.729 구현 (ITU C / Annex A binary / bcg729 / Sipro / FFmpeg)
  인용·실행 0건.
- Annex A binary 사용 0건.
- gate 17 reactivation 미발의 (REFUTE 시나리오 — t.Skip 본문 무변경,
  docstring 만 갱신).
- 측정-only test (RC-2) 자동 promotion 0 (E5 유지).

---

## 14. Plan-allowed FAIL 목록 (regression baseline, 변동 없음)

```
--- FAIL: TestDiagnostic_SinglePulseChain          (internal/decoder)
--- FAIL: TestDecode_LowEnergyCodebookIsSmooth     (internal/gain)
--- FAIL: TestDecode_SucceedsAcrossAllGainIndices  (internal/gain)
```

(Phase 1l 시점부터 plan-allowed; 본 cycle 변동 0.)

---

## 15. 본 cycle 종결 산출

### 15.1 plan checkbox

- [x] RC-1 verdict 분류 → **Refute** (REFUTE_unchanged).
- [x] RC-2 verdict 기록 → T1=20, β1=3277 Q14, c[5..7]=[0,0,0]; assertion-pass.
- [x] **knob retirement (E2' commitment)** 실행:
  - [x] `internal/lsp/interpolate.go`: knob 변수 + switch 분기 + mode-1
        브랜치 삭제, 표현식을 cycle-진입 single-line `int16((int32(prev[i])
        + int32(curr[i])) >> 1)` 로 원복. SPEC verbatim doc-comment block
        은 valuable spec doc 으로 유지하되 prose 갱신 (RC-1 empirical
        결과 기록).
  - [x] `internal/lsp/interp_testhook.go` 삭제 (`git rm`).
  - [x] `internal/decoder/phase1n_rc1_lspinterp_branch_diagnostic_test.go`
        삭제 (`git rm`, Option A — knob 부재 시 컴파일 불가).
  - [x] `internal/decoder/phase1n_rc2_pitch_preemphasis_diagnostic_test.go`
        유지 (production touch 0).
- [x] gate 17 docstring 갱신 (R-C bullet REFUTED 강등 + (c) specific
      narrowing + 누적 30).
- [x] synthesis report 작성 (본 문서).

### 15.2 회귀 게이트 검증

```
$ go vet ./...                          # clean
$ go test ./... -race                   # baseline = 3 plan-allowed FAILs
                                        # (TestDiagnostic_SinglePulseChain,
                                        #  TestDecode_LowEnergyCodebookIsSmooth,
                                        #  TestDecode_SucceedsAcrossAllGainIndices)
                                        # gate 17 SKIP 유지
```

cycle-진입 시점 (`b1412d4`) 의 baseline 과 byte-EQ.

### 15.3 사용자 게이트 의무

- **G-XS7 (Defect-confirmed branch)**: **미발의** (REFUTE 시나리오 —
  G-XS7 발의 조건 미충족).
- **사용자 결정 후보 (G-XS7-final)**: §12 (d-final) gate 17 영구
  disposition — (c-corrigendum-search) 결과 후 결정 권고. 본 commit
  은 발의만, 결정은 사용자.

---

## 16. 차기 cycle 권고 ordering (Phase 1n 종결, Phase 1o 진입 전)

1. **(c-corrigendum-search)** — ITU public corrigendum / Appendix
   I/II/III 에서 §3.10 synth.Filter rounding 또는 §A.4.* simplifications
   에 대한 verbatim clarification 검색. 비용 LOW, 수익 HIGH (mechanistic
   exhaustion 후 유일한 live spec-internal lead).
2. **(d-final) 사용자 게이트 G-XS7-final** — (c) 결과에 따라 gate 17 의
   영구 disposition 결정 (permanent t.Skip + archive, ITU escalation,
   또는 want-domain 재해석). 사용자 결정 사안.

---

**보고서 끝**.
