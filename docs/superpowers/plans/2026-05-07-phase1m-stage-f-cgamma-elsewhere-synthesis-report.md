# Phase 1m F-Cγ-elsewhere 종합 보고서 + 차기 cycle 권고

**작성일**: 2026-05-07
**Cycle ID**: `F-Cγ-elsewhere` (Phase 1m 1번째 cycle, alternative path **(b)** —
parameter decode pipeline upstream of LP synthesis)
**범위**: CE-1 (`f3df272`, gain VQ Imap+GBK verbatim) + CE-2 (`0d58ca6`, FCB
position+sign verbatim) + CE-3 (`5232411`, LSP init+interp verbatim) 측정
결합. plan `57b877f` §Task CE-4 3-시나리오 결정 트리 적용.
**산출물**: 3-시나리오 결정 트리 적용 → **(CE-ambiguous)** 시나리오 확정 +
Phase 1m 잠정 종결 + R-A / R-B / R-C 3종 spec ambiguity 인벤토리 + gate 17
reactivation map 갱신 (corrigendum trigger 가 specific R-target 보유) + 차기
cycle 권고 ordering + 사용자 게이트 G-XS6.
**선행 commit**:
- `5232411` — Phase 1m CE-3 LSP/LSF init+interp diagnostic.
- `0d58ca6` — Phase 1m CE-2 FCB position+sign diagnostic.
- `f3df272` — Phase 1m CE-1 gain VQ Imap+GBK diagnostic.
- `9ab1c91` — gate 17 disposition (Phase 1l alt-path d-i, t.Skip).
- `f902bd9` — Phase 1l F-non-Hpost synthesis + close + alternative path.
- `57b877f` — Phase 1m F-Cγ-elsewhere cycle plan.

**준수**:
- production 변경 0 라인 (E2 — CE-1/2/3 모두 측정-only test, CE-4 = synthesis
  + 본 commit 내 docstring-only edit on `stagef_octpostfix_regression_test.go`
  reactivation-trigger 부에 R-C 줄 1개 추가, `t.Skip` 본문 무변경).
- 외부 G.729 구현 0 인용 (E1 / G1 결정 — Annex A binary 거부 유지).
- 본 task = 보고서 + 1 docstring 라인 — production code line 0.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경
  (Phase 0.6 보존 의무).
- 측정 bundle (CE-1/2/3) 자동 promotion 금지 (E5) — 명시 게이트 G-XS6 권고.
- 모든 verdict = `EQ` / `NE` / `UNDETERMINED` (R-blocking 표기).

---

## 0. Working tree + escape hatch 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go  ← Phase 0.6 보존 의무
$ git log -1 --oneline
5232411 test(lsp): add Phase 1m CE-3 init+interp verbatim diagnostic
```

본 commit (CE-4 = synthesis + reactivation-map docstring) 후 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go  ← 미변경 (의도)
HEAD = <synthesis commit>  docs(plans): Phase 1m F-Cγ-elsewhere synthesis +
                                          ambiguous verdict
```

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 평가 | 근거 |
|------|-----------|------|------|
| **E1** | 외부 G.729 구현 인용·실행 | **미발동** | CE-1/2/3 모두 PDF (`G729E.pdf` §3.2 / §3.8 / §3.9 / §A.3.x) + repo internal 패키지만 사용. ITU reference C / Annex A binary / bcg729 / Sipro / FFmpeg G.729 인용 0건. CE-1 의 `tables.GainImap*` / `tables.GainGBK*` cross-check 도 PDF Annex A Table verbatim 확인 한도 (R-A blocking 으로 cell F/G 는 enumerate 0). |
| **E2** | production 변경 라인 > 0 | **미발동 (test docstring 1 hunk 제외)** | CE-1/2/3 commit 의 production diff 0 라인. CE-4 synthesis commit = 보고서 신규 + `stagef_octpostfix_regression_test.go` REACTIVATION TRIGGERS 섹션에 R-C 항목 1줄 추가만 — `t.Skip` 본문 + 테스트 본문 + behavior 변경 0. |
| **E3** | gate 17 즉시 reactivation 강행 | **미발동** | (CE-ambiguous) 시나리오 채택 → gate 17 skip 유지. reactivation map 의 (c) corrigendum 슬롯에 specific R-target 추가만 수행 (트리거 자체는 unchanged). |
| **E4** | spec 모호 paragraph cherry-pick / 모호 verdict | **미발동** | R-A (§3.9.3 Imap reorder 값 verbatim 부재) → CE-1 cell F/G UNDETERMINED. R-B (§3.8.2 sign bit ordering vs Table 8 NOTE) → CE-2 cell P/S/C UNDETERMINED. R-C (§3.2.5 eq.(24) sf-1 rounding mode) → CE-3 cell sf-1 R-C UNDETERMINED. 모두 spec verbatim 부재 / 모호 분류. "관행상 자연" cherry-pick 0건. |
| **E5** | 측정-only test 자동 promotion (gate 20 자동 등재) | **미발동** | 본 보고서 §10 G-XS6 권고 (gate 20 = CE-1/2/3 bundle promote 여부 사용자 결정). 자동 promotion 0. |

### 0.3 사용자 G-XS5 결정 정합

- **G-XS5 = "(b) Cγ-elsewhere — parameter decode upstream re-visit"**: 본 cycle
  진입 premise. 3 task (CE-1/2/3) 모두 plan-bound 충족.
- **bis 보존**: untracked `stagef_bis_diagnostic_test.go` 미변경.
- **gate 17 SKIP 유지**: §7 reactivation map 갱신 단, t.Skip 자체 무변경.

---

## 1. F-Cγ-elsewhere cycle commit 요약

```
<synthesis hash>  docs(plans): Phase 1m F-Cγ-elsewhere synthesis +
                                ambiguous verdict                  ← 본 commit
5232411            test(lsp): add Phase 1m CE-3 init+interp verbatim diagnostic
0d58ca6            test(fcb): add Phase 1m CE-2 position+sign bit ordering
                    diagnostic
f3df272            test(gain): add Phase 1m CE-1 gain VQ Imap+GBK verbatim
                    diagnostic
57b877f            docs(plans): add Phase 1m Stage F-Cγ-elsewhere plan
9ab1c91            test(decoder): gate 17 disposition (Phase 1l alt-path d-i)
f902bd9            docs(plans): Phase 1l F-non-Hpost synthesis + close +
                    alternative path  (직전 cycle 종결)
```

---

## 2. CE-1 verdict — gain VQ Imap+GBK (commit `f3df272`)

**측정 대상**: ALGTHM frame 0 sf0 의 gain VQ decode chain — bitstream
GA1/GB1 → `Imap1[GA]` / `Imap2[GB]` 적용 → `GBK1[Imap1[GA]]` /
`GBK2[Imap2[GB]]` 의 `(g_p, γ̂_c)` 추출 → `g_p` 합산 → `g_c = γ̂_c · ĝ_c'`
saturation, vs spec §3.9 / §3.9.3 / §A.3.9 verbatim.

### 2.1 14-cell verdict matrix (요약)

| cell 그룹 | spec § | verdict 분포 |
|-----------|--------|--------------|
| A: Imap-applied indexing 존재 (decoder = `GBK[Imap[idx]]`) | §3.9.3 mandate | **EQ** |
| B/C/D/E: g_p 합산 / γ̂_c 부호 / saturation / Q-format 정합 | §3.9 eq.(73)-(74) | **EQ** × 7 |
| F (R-A): Imap1 / Imap2 테이블 값 vs §3.9.3 verbatim text | §3.9.3 reorder | **UNDETERMINED** (R-A blocking) |
| G (R-A): GBK1 / GBK2 테이블 값 vs §3.9 verbatim text | §3.9 / §A.3.9 | **UNDETERMINED** (R-A-extended) |

**verdict 분포**: NE = 0, EQ = 8, UNDETERMINED = 6 (R-A 6 cells blocking).

### 2.2 R-A 식별 (verbatim 인용)

> §3.9.3 (PDF, Codeword computation for gain quantizer) — 텍스트는 reorder
> mapping 의 **존재** (encoder/decoder side 의 `Imap*` permutation) 만 명시,
> map array 의 numeric values 자체는 §3.9.3 본문에 **verbatim 부재**.

→ `tables.GainImap1` / `tables.GainImap2` / `tables.GainGBK1` /
`tables.GainGBK2` 의 literal 값 vs spec text byte-EQ 검증 = 출처 paragraph
부재로 enumerate 불가 = E4 ambiguity 분류 → cell F/G UNDETERMINED.

### 2.3 결론 (CE-1)

- **(CE-1-defect)** 시나리오 조건 = CE-1 ≥1 NE → 본 측정 NE = 0 → **REFUTED
  (path (b)-i 폐기)**.
- 8 EQ = §3.9.3 mandate (Imap-applied indexing 존재) + §3.9 eq.(73)-(74)
  composition / saturation 직접 매핑.
- 6 UND = R-A 의 §3.9.3 reorder map 값 verbatim 부재 = corrigendum / Annex 의
  보충 source 없이는 폐기 불가능.

---

## 3. CE-2 verdict — FCB position+sign decode (commit `0d58ca6`)

**측정 대상**: ALGTHM frame 0 sf0 의 FCB decode — bitstream C1 (13-bit
position field) → 4 pulse positions + bitstream S1 (4-bit sign field) → 4
pulse signs → c[0..39] 재구성, vs spec §3.8 / §3.8.2 / §A.3.8 verbatim.

### 3.1 24-cell verdict matrix (요약)

| cell 그룹 | spec § | verdict 분포 |
|-----------|--------|--------------|
| Range / Q-format / pulse count 정합 | §3.8 Table 7-8, eq.(45) | **EQ** × 16 |
| P (positions): C1 13-bit 분해 i0..i3 — eq.(62) integer-decomposition 의 bit-string ordering vs Table 8 NOTE "MSB transmitted first" | §3.8.2 eq.(62) + Table 8 NOTE | **UNDETERMINED** × 3 (R-B blocking) |
| S (signs): S1 4-bit 분해 s0..s3 — eq.(61) bit-position vs production `(3-i)` convention | §3.8.2 eq.(61) | **UNDETERMINED** × 3 (R-B blocking) |
| C (c[]): downstream of P+S → R-B 전파 | §3.8.2 | **UNDETERMINED** × 2 |

**verdict 분포**: NE = 0, EQ = 16, UNDETERMINED = 8 (R-B 8 cells blocking).

### 3.2 R-B 식별 (verbatim 인용 + production self-doc)

> §3.8.2 eq.(62) gives the position field as an integer decomposition;
> Table 8 NOTE: "the most significant bit (MSB) is transmitted first" —
> 그러나 어느 bit position 이 i0 / i3 에 매핑되는지 verbatim **부재**.

> production `internal/fcb/signs.go:8-12` 자체 self-doc 주석:
> "the highest-MSB-transmitted bits as the i0 field — this corresponds
>  ... not pinned by the spec."

→ R-B = §3.8.2 sign/position bit-string layout verbatim 부재 + production
self-doc 가 모호 인정. cell P/S/C UNDETERMINED.

### 3.3 핵심 ALGTHM finding — sample 5..7 sign mismatch ≠ FCB-direct

CE-2 는 ALGTHM frame 0 sf0 의 **PROD c[]** 와 (eq.(62)+(61) 어느 reading 에서나
도출 가능한) **SPEC c[]** 모두에서 **c[5] = c[6] = c[7] = 0** 임을 측정 입증
(`fcb` test log line 503). FCB pulse 위치는 ALGTHM frame 0 sf0 에서 sample
5..7 중 어디에도 없음 → sample 5..7 의 부호 mismatch 는 **R-B 의 bit ordering
재해석으로 직접 해소될 수 없음**. 즉 R-B 가 confirmed-blocking 이지만 sample
5..7 mechanism 은 아님.

### 3.4 결론 (CE-2)

- **(CE-2-defect)** 시나리오 조건 = CE-2 ≥1 NE → 본 측정 NE = 0 → **REFUTED
  (path (b)-ii 폐기)**.
- 16 EQ = §3.8 / §A.3.8 의 range / Q-format / pulse count / eq.(45)
  composition 직접 매핑.
- 8 UND = R-B 의 §3.8.2 sign/position bit-string layout 모호 (production
  self-doc 인정).
- ALGTHM 자체 sample 5..7 = c[]=0 이므로 R-B 재해석은 sample 5..7 부호
  mismatch 의 직접 mechanism 후보가 아님 (위치 확인 evidence).

---

## 4. CE-3 verdict — LSP/LSF init + interpolation (commit `5232411`)

**측정 대상**: ALGTHM (+ 추가 sub-vector) frame 0 의 LSP decode chain —
bitstream L0/L1/L2/L3 → §3.2.4 init `pastResiduals` + `prevLSP` byte-EQ →
eq.(20) MA predictor (selector L0) → §3.2.5 eq.(24) sf-1/sf-2 interpolation
→ §3.2.6 LSP→LP a[] (a[0]=4096 contract only), vs spec verbatim.

### 4.1 63-cell verdict matrix (요약)

| cell 그룹 | spec § | verdict 분포 |
|-----------|--------|--------------|
| INVARIANT-A: `initialPastResidual` Q13 = `round(i·π/11 · 8192)` byte-EQ × 4 frame × 10 entry | §3.2.4 + §4.3 Table 9 (`l̂_i = iπ/11`) | **EQ** × 40 |
| INVARIANT-B (selector): L0 dispatch `MAPredictorsLSP[L0]` | §3.2.4 eq.(20) | **EQ** × 1 |
| `initialPrevLSP` Q15 = `round(cos(i·π/11) · 32768)` byte-EQ × 10 entry (Table 9 `q_i = arccos(iπ/11)` typo cross-evidence-disambiguated via eq.(18)) | §3.2.4 eq.(18) + §4.3 Table 9 | **EQ** × 10 |
| INVARIANT-C: sf-2 LSP = current LSP (eq.(24)) | §3.2.5 eq.(24) sf-2 | **EQ** × 5 |
| INVARIANT-D: sf-1 LSP = `(prev+curr)>>1` (eq.(24) sf-1) | §3.2.5 eq.(24) sf-1 | **EQ** × 3, **UNDETERMINED** × 2 (R-C blocking) |
| §3.2.6 a[0] = 4096 contract sf-1 + sf-2 | §3.2.6 | **EQ** × 2 |

**verdict 분포**: NE = 0, EQ = 61, UNDETERMINED = 2 (R-C blocking on sf-1
half-sum cells where odd-(prev+curr) with sum<0 reading distinguishes
floor-toward-neg-inf vs symmetric round).

### 4.2 R-C 식별 (verbatim 인용)

> §3.2.5 eq.(24) (PDF p. 14, lines 901..919):
> "Subframe 1 : q_i^(1) = 0.5 q_i^(previous) + 0.5 q_i^(current)
>                                       i = 1,...,10"

→ "0.5 multiplication" 의 fixed-point **rounding mode 명시 부재**.
production `internal/lsp/interpolate.go:13` 는 `int16((int32(prev[i]) +
int32(curr[i])) >> 1)` = floor-toward-negative-infinity (arithmetic right
shift on 2's-complement). odd `(prev+curr)` with sum<0 cells 에서 symmetric
round-half-away-from-zero 와 **−1 LSB drift**. 본 ambiguity 가 R-C
blocking → cell sf-1 R-C UND × 2.

### 4.3 §3.2.4 init formulas verbatim (NEW hard-spec invariant 4)

CE-3 의 INVARIANT-A + INVARIANT-B + Table 9 cross-evidence-disambiguated
`q_i` 가 결합하여 §3.2.4 init formulas 가 **byte-EQ 직접 매핑** 되는 4번째
hard-spec invariant 를 구성:

> **Invariant I-4 (NEW, §3.2.4 init formulas verbatim)**:
> - `pastResiduals[k][i]` (k=-1..-4, i=1..10) Q13 = `round(i·π/11 · 8192)`
>   ∈ [2340, 4679, 7019, 9359, 11698, 14038, 16377, 18717, 21057, 23396]
>   — `internal/decoder.go:37~48` `initialPastResidual` 와 byte-EQ.
> - `prevLSP[i]` (i=1..10) Q15 = `round(cos(i·π/11) · 32768)` ∈ [31441,
>   27566, 21458, 13612, 4663, -4663, -13612, -21458, -27566, -31441]
>   — `internal/decoder.go:29~32` `initialPrevLSP` 와 byte-EQ
>   (Table 9 row `q_i = arccos(iπ/11)` 의 verbatim 표기는 typo;
>   eq.(18) `ω_i = arccos(q_i)` 와 LSF init `iπ/11` 의 angle-domain
>   결합으로 LSP init = cos(iπ/11) 로 cross-evidence-disambiguated).

### 4.4 R-C plausible mechanism finding (sample 5..7 sign mismatch)

R-C UND × 2 cell 은 sf-1 LSP 에서 **−1 LSB drift** 가능성이 있음. 이 1-LSB
ripple 은 §3.2.6 LSP→LP `a[]` 다항식 expansion 을 거쳐 §3.10 / §A.3.10 IIR
synthesis filter 의 early-sample (sample 0..7) 출력 부호에 propagate 가능.
즉 R-C 는 sample 5..7 sign mismatch 의 **PLAUSIBLE mechanism** — 단
PROVABLE 수준은 아님 (직접 cross-cut 측정 부재).

### 4.5 결론 (CE-3)

- **(CE-3-defect)** 시나리오 조건 = CE-3 ≥1 NE → 본 측정 NE = 0 → **REFUTED
  (path (b)-iii 폐기)**.
- 61 EQ = §3.2.4 init formulas + eq.(20) MA predictor selector + eq.(24)
  sf-2 + §3.2.6 a[0] contract 직접 매핑 → I-4 신규 hard-spec invariant 추가.
- 2 UND = R-C 의 §3.2.5 eq.(24) sf-1 rounding mode verbatim 부재 +
  PLAUSIBLE sample 5..7 mechanism (1-LSB ripple → a[] → synth early sample
  부호) — provable 까지는 미달 (별도 empirical disposition 필요, §10 참조).

---

## 5. 3-시나리오 결정 트리 적용 (plan §Task CE-4)

| 시나리오 | 조건 | 본 cycle 결과 | 적용 |
|----------|------|----------------|------|
| **(CE-defect)** | CE-1/2/3 ≥1 NE | NE = 0 (전 셀) | **REFUTED** |
| **(CE-refute)** | CE-1/2/3 EQ_ALL (UND 0) | UND = 16 (R-A 6 + R-B 8 + R-C 2) | **NOT APPLICABLE** |
| **(CE-ambiguous)** | ≥1 spec ambiguity (E4) + 다른 cell EQ | R-A + R-B + R-C 3종 ambiguity 동시 + 85 EQ + 0 NE | **확정** |

**선택 시나리오**: **(CE-ambiguous)**.

**의미**:
- (CE-defect) 제거 → parameter decode upstream 의 **검증 가능한** 모든 surface
  (Imap-applied indexing 존재, eq.(73)-(74) gain composition / saturation,
  eq.(45)/(61)/(62) FCB pulse 수/Q-format/range, §3.2.4 init formulas
  byte-EQ, eq.(20) selector dispatch, eq.(24) sf-2, §3.2.6 a[0]) = spec
  정합.
- (CE-refute) 미적용 → 16 cell 이 spec verbatim 부재 / 모호로 폐기 불가
  (R-A / R-B / R-C 3종 confirmed-blocking).
- 그 중 **R-C 가 sample 5..7 mechanism 의 PLAUSIBLE-but-not-PROVABLE 후보**
  로 잔존 → 본 cycle 은 mechanism 식별에 도달하지 못했지만 **R-C surface
  는 live** (다음 cycle 의 empirical disposition 후보).

---

## 6. R-blocking 인벤토리 (consolidated, CE-1/2/3)

| ID | 위치 (spec §) | 내용 (verbatim 부재 paragraph) | 영향 cells | sample 5..7 mechanism 후보? |
|----|---------------|--------------------------------|-----------|----------------------------|
| **R-A** | §3.9.3 (Imap reorder map values) | "encoder/decoder side reorder permutation 의 **존재** 명시, map array 의 numeric values 는 §3.9.3 본문에 verbatim 부재" — merger-doctrine data table | CE-1 cell F/G × 6 | 不確定 (Imap/GBK 값이 ALGTHM frame 0 의 g_p / g_c 경로에 영향, 단 §F-non-prelim-X-split-2 가 g_c=+4153 측정 정합 → 실제 영향 surface 좁음) |
| **R-B** | §3.8.2 (sign+position bit-string layout) | eq.(61)+(62) integer-decomposition 의 "어느 transmitted bit 이 i0/s0 에 매핑되는지" verbatim 부재 — production self-doc (`signs.go:8-12`) 가 모호 인정 | CE-2 cell P/S/C × 8 | **NO** (CE-2 §3.3: ALGTHM frame 0 sf0 의 PROD/SPEC c[] 양쪽에서 c[5]=c[6]=c[7]=0 — pulse 위치가 sample 5..7 중 부재) |
| **R-C** | §3.2.5 eq.(24) sf-1 rounding mode | "0.5 q_i^(previous) + 0.5 q_i^(current)" 의 fixed-point rounding mode (floor vs symmetric) verbatim 부재 — production = floor-toward-neg-inf via `(prev+curr)>>1` | CE-3 sf-1 R-C × 2 | **PLAUSIBLE** (1-LSB drift → §3.2.6 a[] → §3.10 / §A.3.10 IIR early-sample ripple, single-LSB 는 sample 5..7 부호 flip 충분 magnitude). PROVABLE 수준 아님 (별도 empirical disposition 필요) |

---

## 7. Hard-spec invariant 누적 (3 → 4)

| # | invariant | 출처 § / verbatim | 출처 cycle |
|---|-----------|-------------------|-----------|
| I-1 | postfilter `agcGainPrev` 의 subframe 경계 carryover (`g(n) = 0.85·g(n-1) + 0.15·G_n`, sample-by-sample) | §4.2.4 | HP-1 (`076b6de`) |
| I-2 | §4.3 catch-all zero-init (Table 9 비-등재 변수 zero 강제) | §4.3 lines 1696..1707 | HP-2 (`2ee0009`) |
| I-3 | §A.4.2.5 IIR pole-pair impulse decay 정확 추적 (1.93 z⁻¹ / -0.94 z⁻²) | §4.2.5 / §A.4.2.5 | HP-2 (`2ee0009`) |
| **I-4 (NEW)** | §3.2.4 init formulas: `pastResiduals` Q13 = `round(i·π/11 · 8192)`, `prevLSP` Q15 = `round(cos(i·π/11) · 32768)` (Table 9 `q_i` typo cross-evidence-disambiguated via eq.(18)) — byte-EQ vs production `initialPastResidual` / `initialPrevLSP` | §3.2.4 + §4.3 Table 9 + eq.(18) | **CE-3 (`5232411`)** |

→ **4 hard-spec invariants 누적, 모두 byte-EQ 직접 매핑**.

---

## 8. 누적 폐기 catalog (22 + 3 Phase 1m = **25 sub-hypothesis**)

### 8.1 Phase 1l carry (22건)

Phase 1l carry 22건 (16 Phase 1k + 4 Phase 0c + 2 Phase 1l) — `f902bd9` §6
참조.

### 8.2 Phase 1m 신규 폐기 3건 (본 cycle)

| sub-hypothesis | 출처 | verdict |
|----------------|------|---------|
| `CE-1-gain-VQ-defect` (gain VQ Imap reorder + GBK composition + saturation 균열) | CE-1 (`f3df272`) | **REFUTED** (NE = 0; 8 EQ verbatim, 6 UND R-A blocking) |
| `CE-2-FCB-position-sign-defect` (FCB position 13-bit / sign 4-bit decode 균열) | CE-2 (`0d58ca6`) | **REFUTED** (NE = 0; 16 EQ verbatim, 8 UND R-B blocking; ALGTHM c[5..7]=0 finding) |
| `CE-3-LSP-init-interp-defect` (LSP init constants / L0 selector / sf-1/sf-2 interpolation 균열) | CE-3 (`5232411`) | **REFUTED** (NE = 0; 61 EQ verbatim, 2 UND R-C blocking; I-4 hard-spec invariant 신규 추가; R-C plausible-but-not-provable sample 5..7 mechanism) |

> **누정 (전 후 합산)**: Phase 1l 직후 22 sub-hypothesis 폐기 + Phase 1m
> 본 cycle 신규 3건 + (CE-1/2/3 의 Imap-applied / eq.(73)-(74) / FCB pulse
> count / §3.2.4 init / eq.(20) selector / eq.(24) sf-2 / §3.2.6 a[0] 의
> 7 sub-cell 묶음) → cumulative **25 sub-hypothesis 폐기, defect = 0,
> hard-spec invariant 매핑 = 4건 (I-1..I-4)**.

---

## 9. 19-gate 상태 dump

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | Phase 1a~1j 누적 16건 | **PASS** | 변동 없음. |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 | **SKIP** (`9ab1c91` t.Skip 유지) | reactivation map 갱신 (§10) — t.Skip 본문 무변경, REACTIVATION TRIGGERS 섹션에 R-C 항목 1줄 추가. |
| 18 | F-non-prelim-X-split bundle | **PASS** | 변동 없음. |
| 19 | P0c + Phase 1l measurement bundle | **pending** (E5 미수행) | 변동 없음. |
| 20 (잠정) | Phase 1m CE-1/2/3 measurement bundle | **pending — auto-promote 금지 (E5)** | G-XS6 후 promotion 결정. |

---

## 10. Gate 17 reactivation map 갱신 (NEW R-targets)

본 cycle 결과로 `9ab1c91` 의 REACTIVATION TRIGGERS 섹션이 **specific
R-target 보유 상태로 진화**. 원본 트리거 (generic) → 본 cycle 후 (specific):

| 원본 트리거 (`9ab1c91`) | Phase 1m 후 update | 정당화 |
|--------------------------|---------------------|--------|
| (c) corrigendum / Appendix I/II/III | **specific R-target 3건 보유**: R-A (§3.9.3 Imap map values), R-B (§3.8.2 sign/position bit-string layout), R-C (§3.2.5 eq.(24) sf-1 rounding mode) | CE-1/2/3 가 verbatim 부재 paragraph 3종을 named 식별 → corrigendum search 가 generic 에서 specific 로 narrowed. |
| (a) Phase 1g multi-frame state | 여전히 generic | Phase 1m 은 frame 0 한정 측정 — multi-frame propagation surface (e.g. CE-1 의 long-term predictor `agcGainPrev`-like state, β / `g(-1)` / `Û^(k)` / pitch lag prev FIFO) 는 미탐색. |
| 새 spec source 도입 | 여전히 generic | 변동 없음. |
| **NEW: R-C empirical disposition** (Phase 1m CE-3 finding) | **신규 트리거** — branch-test `(prev+curr)>>1` (floor) vs symmetric round, ALGTHM sample 5..7 부호 flip 관찰 | CE-3 의 PLAUSIBLE-but-not-PROVABLE 판정 → empirical 1-회 측정 (E5-gated) 으로 PROVABLE 승격 가능. |

**Docstring 갱신** (본 commit 1 hunk, behavior 무변경):
`internal/decoder/stagef_octpostfix_regression_test.go` 의 REACTIVATION
TRIGGERS comment block 에 4번째 bullet 추가:

> "- R-C empirical disposition (Phase 1m CE-3 finding): branch-test
>   sf-1 rounding mode flip on ALGTHM sample 5..7."

`t.Skip` 본문 + 테스트 본문 + import + behavior 무변경.

---

## 11. 차기 cycle 권고 ordering (Phase 1n 진입 전 G-XS6)

### 11.1 4-옵션 정의

#### **(c-R-C-empirical) — Phase 1n NEW (default 권고 rank 1)**
- **목적**: R-C 의 PLAUSIBLE-but-not-PROVABLE 상태를 1-회 empirical
  branch-test 로 disposition. production `interpolate.go:13` 의
  `(prev+curr)>>1` 을 임시로 symmetric round (`(prev+curr+1)>>1` 또는
  부호-aware variant) 로 flip 한 branch 상에서 ALGTHM frame 0 sf0 sample
  5..7 부호 관찰.
- **scope**: 1 file 1-라인 변경 + 1 측정 + revert (또는 fix-cycle
  promotion). E5-gated (production touch — 별도 fix cycle 또는 명시 사용자
  허용 후).
- **비용**: 最低 (1-line + 1-test, ~1 cycle).
- **expected gain**: 最高 (R-C 가 mechanism 이면 → defect identified +
  gate 17 reactivation; R-C 가 mechanism 이 아니면 → R-C path 폐기 +
  잔여 R-A/R-B/(c-corrigendum-search) 로 좁힘).
- **risk**: branch-test 결과 부호 unchanged 시 R-C 폐기 → 잔여 surface
  공식 고갈 가속 (단 이는 정보 yield 자체가 가장 높은 결과 — 후속
  ordering 결정 명료).

#### **(c-corrigendum-search) — rank 2**
- **목적**: ITU public corrigendum / Appendix I/II/III / textbook
  secondary source 에서 R-A (§3.9.3 Imap map values) / R-B (§3.8.2 sign
  bit ordering) / R-C (§3.2.5 sf-1 rounding) 중 하나라도 verbatim
  resolution 확인.
- **비용**: 低 (문서 fetch + 인용 확인).
- **expected gain**: 不確定 (errata 가 본 mismatch 와 직접 관련될 확률
  unclear, 단 cost 도 낮음).
- **risk**: 결과 무성과 시 (a) 진입 지연. (c-R-C-empirical) 와 병행 가능.

#### **(a) Phase 1g multi-frame state 진단** — rank 3
- **목적**: frame 0 한정 측정을 frame 0..N 로 확장 — frame-rate state
  (β, `g(-1)`, `Û^(k)`, pitch lag FIFO, gain VQ MA predictor history,
  CE-1 의 `agcGainPrev`-like long-term predictor) propagation.
- **비용**: 高 (multi-frame instrumentation + 4 vector × N frame).
- **expected gain**: 中 (CE-1/2/3 가 frame 0 spec-정합 확인 → frame-rate
  state 가 frame 0 한정 mismatch 의 원인일 확률 낮음, 단 surface 공식
  미탐색).
- **risk**: 추가 sub-hypothesis 폐기 누적만 발생할 위험.
- **defer**: (c-R-C-empirical) disposition 후 진입 (R-C 가 mechanism 이면
  (a) 불요).

#### **(b-pitch-pre-emphasis) — rank 4 (sub-task uncovered by CE-2)**
- **목적**: CE-2 가 명시적으로 측정하지 않은 §3.8 후반 pitch enhancement
  `β·c(n−T)` 의 residue-0 (lag T 이내 sample) 영역 + pulse-at-position-0
  특수 케이스. ALGTHM frame 0 sf0 의 c[0..3]=+8192 + u[0..3]=+1 측정
  (F-non-prelim-1) 와 결합 시 pitch-FCB cross-term 의 잔여 surface.
- **비용**: 低-中.
- **expected gain**: 低-中 (CE-2 §3.3 가 c[5..7]=0 을 입증 → pitch-FCB
  cross-term 도 sample 5..7 magnitude 영향 미상; 단 신호 chain 검증
  완전성 차원에서 가치).
- **fold**: (c-R-C-empirical) 에 cheap 으로 fold 가능 (동일 ALGTHM frame 0
  sf0 측정 set 재사용).

### 11.2 권고 ordering (default)

**default ordering**: **(c-R-C-empirical) → (c-corrigendum-search) 병행 →
(a) → (b-pitch-pre-emphasis)**.

| rank | option | 핵심 정당화 |
|------|--------|-------------|
| 1 | **(c-R-C-empirical)** | 最低 비용 + 最高 정보 yield. R-C 가 PLAUSIBLE 상태인 한 disposition 미루는 것은 후속 모든 cycle 의 cost 를 비대칭 증대. branch-test 1-회로 R-C path 의 binary 결정 (mechanism / not) 도달. |
| 2 | **(c-corrigendum-search)** | 비용 低 + R-A/B/C 3종 specific R-target 으로 narrowed (Phase 1m 후 처음). (c-R-C-empirical) 와 병행 가능 (서로 독립). |
| 3 | **(a) multi-frame state** | (c) 양자 소진 후 진입. multi-frame 은 비용 高 + frame 0 한정 mismatch 와 직접 인과 약함. |
| 4 | **(b-pitch-pre-emphasis)** | (c-R-C-empirical) 측정 set 와 cheap 으로 fold 가능 — 별도 cycle 로 띄울 가치는 낮음. |

**default 권고 핵심 1줄**: **(c-R-C-empirical) Phase 1n 진입 → R-C
mechanism / not binary 결정 → mechanism 이면 gate 17 reactivation + fix
cycle, not 이면 (c-corrigendum-search) + (a) 잔여 surface 진입**.

---

## 12. Anti-goals (본 cycle 명시 비-수행)

본 cycle 동안 절대 수행 금지 (E1-E5 정합):

1. **production 변경** (E2): CE-1/2/3 + CE-4 모두 production code 0 라인.
   본 commit 의 docstring 1-hunk 추가 = test file 의 REACTIVATION
   TRIGGERS comment block only — `t.Skip` 본문 / 테스트 본문 / import /
   behavior 무변경.
2. **Annex A binary cross-check** (E1 / G1): R-A / R-B / R-C 3종 모두
   ITU-T C reference / bcg729 / Sipro Lab / FFmpeg G.729 / Annex A binary
   동일 input cross-check 시도 0건. R-A/B/C resolution 은 (c)
   corrigendum-search 또는 (c-R-C-empirical) 으로만 진행.
3. **다른 G.729 구현 참조**: 본 cycle 종료까지 0 인용 유지.
4. **R-C guess** (E4): R-C 의 sf-1 rounding mode 가 어느 쪽인지 spec
   verbatim 부재 상태에서 추정 fix 적용 금지. disposition = (c-R-C-empirical)
   branch-test 로만.
5. **gate 17 t.Skip 강행 reactivation** (E3): (CE-ambiguous) 시나리오 채택 →
   skip 유지. reactivation map 의 R-C trigger 추가만 (skip 본문 무변경).
6. **untracked file 변동**: `internal/decoder/stagef_bis_diagnostic_test.go`
   본 cycle 동안 stage / commit / move / delete 0건.
7. **자동 promotion** (E5): CE-1/2/3 측정 test 의 회귀 게이트 자동 등재
   금지. 본 보고서 G-XS6 권고 후 사용자 명시 결정.

---

## 13. Plan-allowed FAIL 목록 (regression baseline, 변동 없음)

```
$ go vet ./...        → clean (VET-OK)
$ go test ./... -race → 3 plan-allowed FAIL 잔존 (변동 없음):
                         - TestDiagnostic_SinglePulseChain
                         - TestDecode_LowEnergyCodebookIsSmooth
                         - TestDecode_SucceedsAcrossAllGainIndices
                       (gate 17 = SKIP, `9ab1c91` t.Skip 유지)
```

baseline 변동 0. production 변경 0. test 변경 = REACTIVATION TRIGGERS
docstring 1 hunk only (behavior 무변경).

---

## 14. Side-finding catalog (carry + 갱신)

| ID | 내용 | spec § / 출처 | sign 영향 | disposition |
|----|------|---------------|-----------|-------------|
| SF-1 | tilt γ_t gating mismatch (carry) | `internal/postfilter/tilt.go` vs §4.2.3 | sample 5..7 부호 무관 (carry) | standing — (a) multi-frame 진입 시 결합 측정. |
| SF-2 | gate 17 RED disposition (carry) | `stagef_octpostfix_regression_test.go` | RED → SKIP 유지 | reactivation map specific R-target 추가 (본 §10). |
| SF-3 | sample-unit UNDETERMINED (P0c-1 carry) | PDF Q-format 명시 부재 | sign 영향 미정 | (c-corrigendum-search) 결합 (carry). |
| SF-4 | low/high energy split (P0c-3 carry) | P0c-3 verdict matrix | mechanism strong signal | (c-R-C-empirical) branch-test 시 cross-vector 재측정 권고. |
| SF-5 | HP filter Δ pattern non-correlate (carry) | impulse decay vs Δ step-form | sign 영향 미정 | mechanism 위치 = HP 상위 — Phase 1m 으로 parameter decode upstream 측 spec 정합 확정 → 잔여 surface 좁아짐 ((c-R-C-empirical) 후 좁힘 가속). |
| **SF-6 (NEW)** | R-A: §3.9.3 Imap reorder map values verbatim 부재 (CE-1) | §3.9.3 paragraph | gain VQ chain 직접 영향 (g_p / g_c 매핑) — F-non-prelim-X-split-2 측정 정합 evidence 로 surface 좁음 | (c-corrigendum-search) target 1. |
| **SF-7 (NEW)** | R-B: §3.8.2 sign/position bit-string layout verbatim 부재 (CE-2) | §3.8.2 + Table 8 NOTE + production self-doc `signs.go:8-12` | ALGTHM frame 0 sf0 의 c[5..7]=0 으로 sample 5..7 mechanism 후보 from FCB 직접 제거 | (c-corrigendum-search) target 2. |
| **SF-8 (NEW)** | R-C: §3.2.5 eq.(24) sf-1 rounding mode verbatim 부재 (CE-3) | §3.2.5 eq.(24) + production `(prev+curr)>>1` floor | 1-LSB drift → §3.2.6 a[] → §3.10 / §A.3.10 IIR early sample 부호 ripple — PLAUSIBLE sample 5..7 mechanism (PROVABLE 미달) | (c-R-C-empirical) branch-test target — Phase 1n default rank 1. |
| **SF-9 (NEW)** | §3.8 후반 pitch enhancement `β·c(n−T)` residue-0 / pos-0 sub-task (CE-2 uncovered) | §3.8 후반 | 不確定 (CE-2 §3.3 c[5..7]=0 finding 와 결합 시 small) | (b-pitch-pre-emphasis) — (c-R-C-empirical) fold 가능. |

---

## 15. 열린 follow-up

| ID | 내용 | tracking |
|----|------|----------|
| FU-1 | F-bis-1 / F-tris diagnostic (`stagef_bis_diagnostic_test.go`, untracked, Phase 0.6 보존) | carry — Phase 1n 진입 시 결정 |
| FU-2 | tilt γ_t gating SF-1 별도 cycle | (a) multi-frame 결합 (carry) |
| FU-3 | gate 19 promotion 명시 사용자 게이트 G-XS5 (E5) | carry |
| FU-4 | gate 17 RED disposition (SF-2) | reactivation map specific R-target 추가 완료 (본 §10) |
| FU-5 | OVERFLOW.BIT loader bug | 별도 cycle (carry) |
| FU-6 | `decode_test.go` 8 `t.Skip` 해제 | 별도 cycle (carry) |
| FU-7 | 3 plan-allowed FAIL (SinglePulseChain / LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices) | 별도 cycle (carry) |
| FU-8 | sample-unit UNDETERMINED (SF-3) | (c-corrigendum-search) 결합 (carry) |
| FU-9 | HP filter Δ pattern non-correlate (SF-5) | (c-R-C-empirical) 후 좁힘 (carry) |
| **FU-10 (NEW)** | gate 20 promotion (CE-1/2/3 measurement bundle, E5) | G-XS6 권고 |
| **FU-11 (NEW)** | (c-R-C-empirical) Phase 1n 진입 — branch-test sf-1 rounding mode flip on ALGTHM sample 5..7 | G-XS6 default rank 1 |
| **FU-12 (NEW)** | (c-corrigendum-search) ITU corrigendum / Appendix I/II/III 가 R-A/R-B/R-C 중 하나라도 resolution 제공하는지 확인 | G-XS6 default rank 2 (병행 가능) |

---

## 16. 본 cycle 종결 산출

### 16.1 plan checkbox

본 commit 후 plan `2026-05-07-phase1m-stage-f-cgamma-elsewhere-plan.md`
§Task 진행 status:

- [x] Task CE-1 — gain VQ Imap+GBK verbatim (`f3df272`).
- [x] Task CE-2 — FCB position+sign verbatim (`0d58ca6`).
- [x] Task CE-3 — LSP/LSF init+interp verbatim (`5232411`).
- [x] **Task CE-4 — synthesis + 3-시나리오 결정 트리 (본 commit)** —
      (CE-ambiguous) 시나리오 확정 + Phase 1m 잠정 종결 + R-A/R-B/R-C
      인벤토리 + gate 17 reactivation map specific R-target 갱신 +
      (c-R-C-empirical) Phase 1n 진입 default 권고 + G-XS6.

### 16.2 회귀 게이트 검증

```
$ go vet ./...        → clean (VET-OK)
$ go test ./... -race → 3 plan-allowed FAIL 잔존 (변동 없음)
                       (gate 17 = SKIP, `9ab1c91`)
```

baseline 변동 0. production 변경 0. test 변경 = `stagef_octpostfix_regression_test.go`
의 REACTIVATION TRIGGERS docstring 1 hunk only (behavior 무변경).

### 16.3 사용자 게이트 의무 G-XS6 (다음 dispatch 전)

1. **(c-R-C-empirical) Phase 1n 진입 승인** (default rank 1) —
   `interpolate.go:13` 의 `(prev+curr)>>1` 을 임시 symmetric round 로 flip
   한 branch-test 의 production-touch 1-회 허용 여부.
2. **(c-corrigendum-search) 병행 진입 승인** (default rank 2) — ITU
   corrigendum / Appendix I/II/III / textbook secondary source fetch.
3. **gate 20 promotion 결정** (E5) — Phase 1m CE-1/2/3 measurement bundle
   회귀 보호 등재 yes/no.
4. **(a) multi-frame / (b-pitch-pre-emphasis) defer 승인** — (c) 양자
   소진 후 진입 ordering 합의.

---

**보고서 종료.** Phase 1m F-Cγ-elsewhere = **(CE-ambiguous) 시나리오 확정**
+ **Phase 1m 잠정 종결** + **누적 25 sub-hypothesis 폐기 + 0 defect + 4
hard-spec invariant 매핑 (I-1..I-4, NEW I-4 = §3.2.4 init formulas)** +
**R-A / R-B / R-C 3종 specific spec ambiguity 인벤토리 + R-C 가 sample 5..7
sign mismatch 의 PLAUSIBLE-but-not-PROVABLE mechanism** + **gate 17
reactivation map specific R-target 갱신 (corrigendum trigger 가 R-A/B/C
3종 named)** verdict. 다음 cycle dispatch = 사용자 G-XS6 ((c-R-C-empirical)
Phase 1n 진입 + (c-corrigendum-search) 병행 + gate 20 promotion 결정) 게이트
통과 후.
