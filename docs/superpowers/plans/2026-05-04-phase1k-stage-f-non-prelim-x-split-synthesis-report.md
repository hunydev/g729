# Phase 1k Stage F-non-prelim-X-split 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-04
**범위**: F-non-prelim-X-split-1 (`fd0b381`, Cα fcb pulse trace) + F-non-prelim-X-split-2 (`4cd25e1`, Cβ gain g_c trace) 측정 결과 결합 분석. plan `49fac32` §Task 3 결정 트리 적용.
**산출물**: cycle 결산 + Cα/Cβ 비교표 + 결정 트리 = "둘 다 spec 정합" verdict + 다음 cycle 단일 권고 + Phase 1k 8-cycle 누적 평가 + 사용자 게이트.
**준수**: production / test 변경 0 라인 (메타 task — 보고서만), 외부 G.729 0 인용 (E4 / G1 결정 정합 — Annex A binary 거부), `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 (Phase 0.5), Phase 0.4 강압-적합 회피 (특히 §1 임의 우선 결정 금지, §3 음성 결과 인정, §6 hybrid 강요 금지, §7 Cδ 재진입 절대 금지).

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 진입 시점 working tree

```
$ git status --porcelain
?? internal/decoder/stagef_bis_diagnostic_test.go        ← Phase 0.5 보존 의무, 본 cycle 3 task 모두 미변경
$ git log -1 --oneline
4cd25e1 test(decoder): add Stage F-non-prelim-X-split-2 Cβ gain g_c trace
```

본 commit (Task 3 = synthesis) 후 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go        ← 미변경 (의도)
HEAD = <synthesis commit hash>  docs(plans): F-non-prelim-X-split synthesis + Phase 1k status
```

### 0.2 escape hatch 평가 (E1–E5)

| 해치 | 발동 조건 | 평가 | 근거 |
|------|---------|------|------|
| **E1** | 본 cycle commit 후 회귀 게이트 1+ FAIL (단, 항목 17 RED 의도 잔존 예외) | **미발동** | §1 회귀 게이트 16 PASS + 항목 17 RED 의도 잔존 + 신규 측정 2건 PASS — F-non-prelim 종결 시점과 동일 baseline. |
| **E2** | spec § 인용이 PDF verbatim grep 결과와 불일치 | **미발동** | Task 1 보고서 §2 (§3.8 eq.(45)/(47)/(48), §3.8.2 eq.(61), §A.3.8.2, §4.3 Table 9), Task 2 보고서 §0 (§3.9 eq.(65), §3.9.1 eq.(69)/(71), §3.9.2 eq.(73)/(74), §A.3.9, §4.3 Table 9) 모두 `pdftotext -layout` verbatim grep 채택. 본 task 는 보고서 only — 추가 인용 0. |
| **E3** | Task 3 종합에서 Cα/Cβ 중 2+ 잔존 (단일 식별 불가) | **미발동** | §3 verdict = "둘 다 spec 정합" — 단일 식별 불가가 *결함 잔존* 으로서가 아니라 *결함 0건* 으로 자동 도출. plan §Task 3 Step 3 결정 트리의 "Cα + Cβ 둘 다 spec 정합" 분기 정합. hybrid 결정 강요 금지 (Phase 0.4 §6) 위반 0. |
| **E4** | 외부 G.729 구현 (참조 C / Annex A binary / bcg729 / Sipro / FFmpeg) 인용·실행 | **미발동** | 본 cycle 3 task 모두 PDF (`docs/superpowers/specs/itu/G729E.pdf`) + READMETV.txt + repo committed PST 파일 (`testdata/itu/test_vectors/`) + 본 repo internal 패키지만 사용. Annex A binary trace 0건. 사용자 G1 결정 정합 100%. |
| **E5** | production 변경 라인 > 0 | **미발동** | `git diff HEAD~3 --stat` 결과 production 디렉토리 (`internal/{fcb,gain,decoder,synth,postfilter,...}/!*_test.go`) 변경 0 라인. test 변경 = `internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go` 신규 1 파일 (Task 1+2 누적 측정 함수 2건). 본 task = docs only. |

### 0.3 사용자 G-S 결정 정합

- **G-S2** (hybrid 진단 cycle 승인): 본 cycle 진입 premise. Task 1+2 분리 측정으로 hybrid 의 sub-source 식별 의도 충족 — 결과는 "둘 다 정합" 으로 hybrid 자체 반증.
- **G-S3** (F-non-prelim-X-split 진입 승인): 본 cycle 자체 정합.
- **G-S4** (bis 보존): Phase 0.5 충족.
- **G-S5** (항목 17 RED 잔존 acknowledge): 다음 cycle 의 GREEN gate 로 승계.

---

## 1. F-non-prelim-X-split cycle commit 요약 + cycle premise + 측정 정량

### 1.1 cycle commit

```
<synthesis hash>  docs(plans): F-non-prelim-X-split synthesis + Phase 1k status   ← 본 commit
4cd25e1           test(decoder): add Stage F-non-prelim-X-split-2 Cβ gain g_c trace
fd0b381           test(decoder): add Stage F-non-prelim-X-split-1 Cα fcb pulse trace
49fac32           docs(plans): add Phase 1k Stage F-non-prelim-X-split plan
e867f5e           docs(plans): F-non-prelim synthesis + cycle decision   (직전 cycle 종결)
```

### 1.2 cycle premise (Cα/Cβ hybrid 분리)

직전 cycle (F-non-prelim, `e867f5e`) synthesis §3.1 = 단일 source 식별 = `g_c · c[n]` 곱 부호. §4.1 권고 = "Cα + Cβ hybrid (단일 source 의 두 fix layer 분리 미수행)". 사용자 G-S2/G-S3 승인 후 본 cycle = 두 fix layer 의 *분리 측정*.

분리 디자인:
- **Cα 측정 (Task 1)**: `fcb.Decode` sub-stage 4건 (idx.Positions / idx.Signs / raw c_raw / β·c[n−T] enhancement Δ) 분리 dump.
- **Cβ 측정 (Task 2)**: `gain.Decoder.Decode` sub-stage 7건 (GA/GB codeword + Imap permutation + GBK1/GBK2 ROM entry + γ̂ 합산 + ĝ_p 합산 + MA predictor Ê(m) + g_c 합성) 분리 dump.

### 1.3 측정 정량 (raw, X-fcb verdict 합치 cross-check)

| 측정 점 | Task 1 (Cα, `fd0b381`) | Task 2 (Cβ, `4cd25e1`) | 비고 |
|---------|-------------------------|-------------------------|------|
| 진단 함수 | `TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace` | `TestDiagnostic_FnonPrelimXSplit2GainGcTrace` | 측정-only PASS (회귀 게이트 18/19번째 자동 promotion 0 — Phase 0.3 §94) |
| sub-stage 측정 수 | 5 (a~e: idx.P / idx.S / c_raw / Δ / c_prod) | 8 (a~h: GA/GB / Imap / GBK1 / GBK2 / γ̂ / ĝ_p / 예측 / g_c) | 누적 13 sub-stage measurement |
| 핵심 결과 1 | c_raw[0..3] = `[+8192, +8192, +8192, +8192]` (Q13) | g_c (Q12) = `+4153` | X-fcb verdict (`f82893d`) baseline 와 합치 |
| 핵심 결과 2 | Δ[0..3] = `[0, 0, 0, 0]` (β enhancement off, n<T=20) | γ̂ = `+1516` (Q13) > 0 (단일 양수) | spec eq.(48) n<T 분기 / eq.(74) γ̂ 합 정합 |
| 핵심 결과 3 | Δ[20..23] ≠ 0 (n≥T 분기 enhancement on) | MA predictor Ê(m) = `+5060` (Q10), magnitude only (부호 영향 0) | 부호 source 와 직교 |
| 부호 결정 sub-stage | **raw placement (a+b+c)** — idx.Signs=0xf → s=[+1,+1,+1,+1] | **VQ-table γ̂** (단일) — γ̂_ga=+1516, γ̂_gb=0, sum=+1516 > 0 | 두 cycle 의 부호 source 모두 spec-canonical |
| spec § 정합도 | 5/5 sub-stage 정합 | 8/8 sub-stage 정합 | 결함 0건 |
| verdict | **Cα-refute** (production 결함 0) | **Cβ-refute** (production 결함 0) | hybrid 자체 반증 |

---

## 2. Cα / Cβ 비교표 (단일 표, 측정 데이터만 — Phase 0.4 §1)

plan §Task 3 Step 2 단일 표:

| 후보 | 측정 출처 | 측정 결과 | 평가 | spec § 인용 (PDF verbatim) |
|------|-----------|-----------|------|--------------------------------|
| **Cα** (`fcb.Decode` c[n] sub-stage) | `fd0b381` 보고서 §2 + §3 | idx.Positions=0x0000 → m=[0,1,2,3]; idx.Signs=0xf → s=[+1,+1,+1,+1]; c_raw[0..3]=+8192 (Q13); Δ[0..3]=0 (β-enh off, n<T=20); c_prod[0..3]=+8192. | **Cα-refute (spec 정합)**. c[0..3] 양 부호 source = (a)+(b)+(c) raw 4-pulse placement; (d) β·c[n−T] 기여 0. fcb decoding 결함 0건. | §3.8 eq.(45) `c(n)=Σ s_k δ(n−m_k)`; §3.8.2 eq.(61) `S = s0 + 2s1 + 4s2 + 8s3` (s=1↔+1); §3.8 eq.(48) `c(n)=c(n)` for n<T / `c(n)+βc(n−T)` for n≥T; §A.3.8.2 = §3.8.2; §4.3 Table 9 past_exc=0 + §4.1.5/6 eq.(75). |
| **Cβ** (`gain.Decoder.Decode` g_c sub-stage) | `4cd25e1` 보고서 §2 + §3 | GA1=5, GB1=6; Imap1[5]=0, Imap2[6]=1; GainGBK1[0]=(g_p=+1, γ̂_ga=+1516); GainGBK2[1]=(g_p=+1994, γ̂_gb=0); γ̂=+1516 (Q13) >0; ĝ_p=+1995; MA pred Ê(m)=+5060 (Q10) (magnitude only); g_c=+4153 (Q12). | **Cβ-refute (spec 정합)**. g_c 양 부호 source = 단일 = VQ-table γ̂ (eq.(74) γ̂_ga + γ̂_gb 합산). predictor 는 magnitude 만, 부호 영향 0. sign-mask / 별도 sign 처리 0건. composition (eq.(65) g_c=γ·g_c') 부호 보존 정합. gain decoding 결함 0건. | §3.9 eq.(65) `g_c = γ · g_c'`; §3.9.1 eq.(69)/(71) MA predictor + Ē=30dB; §3.9.2 eq.(73)/(74) ĝ_p / γ̂ 합산; §A.3.9 = §3.9; §4.3 Table 9 (past_err 디폴트 = MIN_GAIN_PRED_DB seed); §4.1.5/6 eq.(75). |

cross-check (X-fcb verdict, `f82893d`):
- `g_c · c[n]` (Q15 pre-Round) = `+4153 (Q12) × +8192 (Q13) >> 13 = +4153 (Q12)` = `+33224 (Q15)` for n=0..3.
- Round 식: `(33224·2 + 32768) >> 16 = 1` → `u[0..3] = +1` (Q0).
- 본 cycle Task 1 c[0..3]=+8192 + Task 2 g_c=+4153 모두 X-fcb baseline 합치 ✓.

---

## 3. "둘 다 정합" verdict 의 함의 — Phase 1k 8 cycle 누적 결함 0건

### 3.1 결정 트리 적용 (plan §Task 3 Step 3)

| 시나리오 | 본 cycle 적용 |
|----------|---------------|
| Cα 단독 결함 + Cβ 정합 → F-non-fix-fcb | **미해당** (Cα-refute) |
| Cβ 단독 결함 + Cα 정합 → F-non-fix-gain | **미해당** (Cβ-refute) |
| Cα + Cβ 둘 다 결함 → F-non-fix-hybrid | **미해당** (둘 다 refute) |
| **Cα + Cβ 둘 다 spec 정합** → Cγ 재진입 또는 Y follow-up | **채택** |
| 측정 도구 결함 (replication mismatch / Q-format 부정합) → E2 | **미해당** (X-fcb baseline cross-check 합치) |

### 3.2 Phase 1k 누적 결함 위치 정량 (8 cycle)

| # | cycle | 진단 대상 | verdict | production 결함 위치 |
|---|-------|-----------|---------|----------------------|
| 1 | F-oct-prelim (5 task) | postfilter chain spec 정합 + bit-vector + hpFilter init + silence negative | spec 정합 5/5 | 0 |
| 2 | F-oct-prelim-5 (재검토 cycle, 5-2/5-3 등) | hpFilter init state + silence negative mechanism | 정합 | 0 |
| 3 | F-oct-postfix (1 task + RED 게이트 등록) | sample 5..7 RED 잔존 등록 | RED 등록만 (진단 결과 0) | 0 (RED 등록만) |
| 4 | F-oct-postfix2-prelim (4 task) | chain dump baseline + M5 excitation sign + M6 PST sign verify + M3 IIR memory | 정합 4/4 (Cδ M6 byte-level 반증) | 0 |
| 5 | F-non-prelim (4 task) | X excitation sub-항 + Y LP a[] cross-check + Z postfilter spec 인용 + synthesis | X-fcb 단일 source 식별 (`g_c·c[n]` 곱), Cα/Cβ hybrid 잔존, Cγ LOW, Cδ DISMISSED | 0 (sub-source 미상) |
| 6 | F-non-prelim-X-split (본 cycle, 3 task) | Cα fcb pulse + Cβ gain g_c 분리 | **Cα-refute + Cβ-refute** | **0** |
| 7 | (참고) F-oct-prelim-1cy 초기 | postfilter prelim | 정합 | 0 |
| 8 | (참고) F-oct-postfix-1 RED 등록 (`56caa72`) | regression gate 등록 | gate 등록 | 0 (RED 자체는 결함 location 식별 0) |

**누적 결함 위치 = 0건** (8 cycle 측정 모두 production 결함 location 식별 실패; 각 cycle 의 측정 대상은 모두 spec verbatim 정합).

### 3.3 함의 (Phase 0.4 §3 — 음성 결과 인정 의무 정합)

본 cycle 의 "둘 다 spec 정합" verdict 는 *측정 데이터 그대로* 인정 (Phase 0.4 §3 + 사용자 task §"Phase 0.4 강압-적합 회피"). hybrid 결정 강요 (Phase 0.4 §6) 0, Cδ 재진입 (Phase 0.4 §7) 0.

production 결함 0건 + PST want 와의 차이 (sample 5..7 got=+2 want=−1, Δ=3 잔존) 의 결합 함의 = 다음 두 가능성 중 하나 (또는 둘 다):

1. **(i) spec 외부 mechanism**: ITU-T G.729 (06/2012) PDF + READMETV.txt + 본 repo 내부 ROM 의 결합으로는 도출되지 않는 mechanism 이 존재 (예: 본 cycle 미측정 영역 = postfilter chain 의 sample 5..7 한정 조건부 동작, 또는 MA predictor history 의 multi-frame 의존성). → Cγ 재진입 (synthesis §3.4 LOW priority dismiss 의 측정-기반 재고).
2. **(ii) want 데이터 해석 자체 의문**: M6 (`cb9529d`) byte-level 반증 + 9 vector 분포 반증으로 Cδ stimulus 자체는 영구 폐기 (Phase 0.4 §7). 그러나 PST 파일의 *해석 도메인* (post-AGC + post-HP + post-×2, F-non-prelim Task 3 (`dd4e21a`) §5 verbatim 입증) 의 단계 매핑이 sample 5..7 한정 영역에서 *측정 미수행* 인 별도 sub-stage 를 포함할 가능성 — 단, 이는 Cδ 의 "stimulus 결함" 가설과는 *다른* "want 도메인 추가 sub-stage" 가설이며, Cγ 카테고리 (spec 외부 mechanism / 추가 sub-stage 식별) 의 일부.

본 보고서는 (i) 와 (ii) 를 *둘 다 가능성으로 명시* (사용자 task §"Phase 0.4 강압-적합 회피" 정합); 단일 가능성 강요 금지.

---

## 4. 잔존 후보 평가표 (Cγ 재진입 / Y follow-up)

Cδ 영구 제외 (Phase 0.4 §7, M6 + 9 vector 반증). 잔존 2 후보:

| # | 후보 | 진단 의도 | priority | risk | spec-grounding | 예상 cost (cycle 수) | 본 cycle 의 직접 trigger |
|---|------|-----------|----------|------|----------------|----------------------|--------------------------|
| **R1** | **Cγ 재진입** = postfilter chain 의 sample 5..7 한정 mechanism 측정 | F-non-prelim Task 3 (`dd4e21a`) Z-confirm 의 chain order 7항 + PST 비교 도메인 8항 정합 입증을 *재고*. sample 5..7 한정 조건부 동작 (예: §4.2 postfilter 의 short-term + long-term + tilt + AGC 의 sample-wise 분기, 또는 §4.2.5 의 AGC gain factor 의 sample-by-sample 갱신 영향) 의 신규 측정 sub-stage 도입. | **HIGH** (본 cycle 의 직접 후속) | MID — Z-confirm 의 7+8 항목은 chain *order* 정합 입증이지 sample-wise *조건 분기* 의 모든 가능성을 cover 하지 않음. spec 미인쇄 sample-wise 동작이 잔존할 수 있음. | §4.2 (postfilter), §4.2.4 (tilt), §4.2.5 (AGC), §4.1.6 (synthesis filter) — 본 cycle 의 spec 영역 (§3.8/§3.9) 외부의 신규 인용 catalog 도입 | 1 진단 cycle (3~4 task: chain sub-stage 분리 + sample 5..7 한정 조건 식별 + verdict) | 둘 다 정합 verdict + (i) spec 외부 mechanism 가능성 |
| **R2** | **Y magnitude follow-up** = F-sept-2 L3 max\|Δ\|=6 잔존 측정 | F-non-prelim Task 2 (`d1a4f2d`) Y-magnitude verdict (max\|Δ\|=6, a[1]: −2197 vs −2203) 의 *부호* 와 직교한 magnitude gap 이 syn[5..7] 에 *간접 영향* 가능성 측정. forced a-flip 결과 magnitude만 변화 (+1 → 0) 이 sample 5..7 에 누적되어 syn 부호에 영향? | **MID** (직전 cycle 에서 직교 입증) | LOW — Y-magnitude 와 부호 source 직교성은 forced-flip 결과로 입증 (`d1a4f2d` §3.1). magnitude 차이가 syn[5..7] 부호에 영향 줄 가능성은 IIR 누적 효과 한정 — 1 cycle 내 측정 가능. | §3.2.6 (LP a[] reference), §4.1.6 (1/Â(z) IIR) | 1 진단 cycle (2~3 task: Y magnitude exact source + IIR 누적 영향 + verdict) | 둘 다 정합 verdict + Y magnitude gap 잔존 |

### 4.1 우선 순위 평가

- **R1 > R2**: R1 은 본 cycle "둘 다 정합" verdict 의 *직접* 후속 (Phase 1k 8 cycle 누적 결함 0건 상태에서 sample 5..7 RED 잔존 → spec 미측정 sub-stage 식별 의무). R2 는 직교 입증 (`d1a4f2d` §3.1 forced-flip 결과 부호 보존, magnitude 만 변화) 으로 priority 낮음.
- **R1 + R2 hybrid** 는 Phase 0.4 §6 (hybrid 강요 금지) 위반 위험 — 단일 cycle 진입 후 R1 결과로 R2 trigger 여부 결정.

---

## 5. 권고 단일 결정 + 다음 cycle outline

### 5.1 권고

**Cγ 재진입 (R1) 단독 진입.** Y follow-up (R2) 은 본 권고에서 제외 — `d1a4f2d` §3.1 forced-flip 의 부호 직교 입증이 본 cycle 측정 (Cα/Cβ 둘 다 정합) 후에도 유효 (부호 source 의 sub-stage 측정이 모두 spec 정합으로 완료됐기 때문에 magnitude → 부호 mechanism 은 IIR 누적 영향 한정 — R1 의 sample 5..7 한정 mechanism 측정에 포함될 가능성이 더 높음).

근거 1문장: 본 cycle "Cα-refute + Cβ-refute" 측정 결과 + Phase 1k 8 cycle 누적 결함 0건 + sample 5..7 RED 잔존 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` Δ=3) 의 결합은 §3.8/§3.9 외부 (postfilter chain §4.2.* 또는 synthesis filter §4.1.6 sample-wise 동작) 의 spec 미측정 sub-stage 식별을 강제 — Phase 0.4 §3 음성 결과 인정 + §6 hybrid 강요 금지 정합.

### 5.2 다음 cycle (F-non-Cgamma-revisit 가칭) outline

| Task | 진단 의도 | spec § 인용 catalog (예상) | 측정 함수명 (가칭) | 성격 |
|------|-----------|----------------------------|--------------------|------|
| **G-1** | postfilter chain (short-term + long-term + tilt + AGC) sample 5..7 한정 sub-stage dump | §4.2.1 (short-term), §4.2.2 (long-term), §4.2.4 (tilt), §4.2.5 (AGC) | `TestDiagnostic_FnonCgammaRevisit1PostfilterSampleSplit` | 측정-only |
| **G-2** | synthesis filter 1/Â(z) sample 5..7 IIR memory contribution dump (Y magnitude follow-up 통합) | §4.1.6 eq.(75) + §A.4.1 | `TestDiagnostic_FnonCgammaRevisit2SynthIIRSampleSplit` | 측정-only |
| **G-3** | 종합 + 다음 결정 (production fix scope 식별 / 추가 진단 cycle / Phase 1k 잠정 종결 권고) | (G-1 + G-2 결합 인용 catalog) | (보고서 only) | 메타 |

**다음 cycle GREEN gate 승계**: 항목 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) — 다음 *fix* cycle (G-3 후 식별 시) 의 RED → GREEN 전환 의무.

**핵심 입증 의무**:
- G-1: postfilter sub-stage 중 sample 5..7 한정 부호-flip mechanism 식별 또는 반증.
- G-2: synthesis filter 가 +1 입력에 대해 sample 5..7 영역에서 음수 출력 가능성을 IIR memory contribution 으로 입증 또는 반증.
- G-3: 식별 시 fix cycle 진입 / 반증 시 Phase 1k 잠정 종결 (alternative path = Phase 0c 재진입 또는 별도 phase) 권고.

---

## 6. 사용자 게이트 + Phase 1k 종결 평가

### 6.1 사용자 게이트 항목

| # | 게이트 | 사용자 결정 옵션 |
|---|--------|------------------|
| **G-XS1** | 본 cycle "둘 다 spec 정합" verdict 승인 | acknowledge / 반박 (측정 재실행 요구 + 구체 항목 지시) |
| **G-XS2** | 권고 = **Cγ 재진입 (R1) 단독 진입** | 채택 / Y follow-up (R2) 우선 / R1+R2 hybrid 강제 / Phase 1k 잠정 종결 진입 (다음 phase 권고) |
| **G-XS3** | 다음 cycle 명 = `F-non-Cgamma-revisit` | 명명 acknowledge / 다른 명명 지시 |
| **G-XS4** | `internal/decoder/stagef_bis_diagnostic_test.go` (untracked, 본 cycle 미변경) 보존 유지 | 유지 / add+commit 결정 / discard 결정 |
| **G-XS5** | 항목 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) RED 잔존 — 다음 cycle GREEN gate 승계 | acknowledge |
| **G-XS6** | F-sept-2 L3 magnitude gap (max\|Δ\|=6) 잔존 — 다음 cycle G-2 에 통합 | acknowledge / 별도 cycle 강제 |
| **G-XS7** | Phase 1k 잠정 종결 옵션 (8 cycle 결함 0건 + sample 5..7 mechanism 미식별 시) | acknowledge — 종결 trigger 조건 (예: G-3 verdict = "spec 외부 mechanism 0건") 합의 |
| **G-XS8** | Cδ 재진입 절대 금지 (Phase 0.4 §7) 본 cycle 후에도 유지 | acknowledge |

### 6.2 Phase 1k 종결 평가 (8 cycle 누적)

본 cycle 종결 시점 정량:
- Phase 1k Stage F 누적 cycle = **8** (F-oct-prelim 1cy + F-oct-prelim-5 5cy + F-oct-postfix + F-oct-postfix2-prelim + F-non-prelim + F-non-prelim-X-split — 사용자 task §"누적 cycle 결과" 정의 정합).
- 누적 결함 위치 = **0건** (각 cycle verdict 모두 spec 정합 또는 sub-source 미상; 본 cycle 에서 sub-source 분리 후에도 결함 0건).
- 누적 RED 항목 = **1** (항목 17, sample 5..7 Δ=3).
- 측정 함수 누적 = 회귀 게이트 17 + 본 cycle 측정-only 2건 (자동 promotion 0).

| 종결 시나리오 | 가능 / 불가 | 추정 잔여 cycle 수 | alternative path |
|--------------|------------|-------------------|------------------|
| **Best case** (다음 cycle Cγ 재진입에서 sample 5..7 mechanism 식별 + 1~3 라인 fix → 항목 17 GREEN) | **가능** | F-non-Cgamma-revisit (1 진단) + F-non-fix-Cgamma (1 fix) = **2 cycle** | — |
| **Mid case** (Cγ 재진입 G-1 후 G-2 추가 분기 → fix scope 식별) | 가능 | 3 cycle (Cγ 재진입 + Y magnitude 별도 분기 + fix) | — |
| **Worst case** (Cγ 재진입에서도 결함 0건 → spec 외부 mechanism 입증 불가) | 가능 (장기) | +3~5 cycle 또는 **Phase 1k 잠정 종결** | (a) Phase 0c (PCM/IO) 재진입 + want 도메인 재해석 cycle, (b) Phase 1g (decoder integration) 재진입 + multi-frame state 진단, (c) 새 spec source 도입 (예: ITU corrigendum 검색) — 단 G1 결정 (Annex A binary 거부) 유지. |

**Phase 1k 완전 종결 가능성 평가**:
- *완전 종결 가능* — 단 다음 cycle (Cγ 재진입) 의 verdict 결과에 의존. 본 cycle 까지의 누적 결함 0건 상태는 종결을 *지연* 시키지 *않음* (각 cycle 의 음성 결과 = 후보 영구 폐기 = 가설 공간 축소).
- *종결 trigger 조건* (G-XS7 합의 의무): (i) Cγ 재진입에서 sample 5..7 mechanism 식별 + fix → 항목 17 GREEN → 완전 종결. (ii) Cγ 재진입에서도 결함 0건 + spec 외부 mechanism 입증 불가 → **잠정 종결** (alternative path 진입, 예: Phase 0c PCM IO 재해석 또는 Phase 1g multi-frame state 진단).

---

**보고서 종료.** 본 commit = F-non-prelim-X-split cycle 3번째 (final) commit. 다음 cycle = `F-non-Cgamma-revisit` (사용자 G-XS1+G-XS2 승인 시) 또는 Phase 1k 잠정 종결 후 alternative path (사용자 G-XS2 = "Phase 1k 잠정 종결 + 다른 phase 진입" 선택 시).
