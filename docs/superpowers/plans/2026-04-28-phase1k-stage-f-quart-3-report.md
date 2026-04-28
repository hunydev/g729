# Phase 1k Stage F-quart-3 진단 노트 — gain.Decode 비선형 체인 reference cross-check

**작성일**: 2026-04-28
**범위**: `gain.Decoder.Decode` 의 §3.9 / §4.1.6 비선형 체인 (predictedLogGain → log2Fixed(ecEnergy) → ecBarDbQ10 → log2GcQ10 → pow2Fixed → gc0Q14 → gcQ12 → MA predictor FIFO update) 을 ITU-T G.729 (06/2012) PDF §3.9 (식 (66)–(74)) 와 §4.3 Table 9 에서 직접 도출한 reference impl 로 평행 검증.
**산출물**: 비선형 체인의 **2 spec-위반 식별** (Q-format 보정 누락 + int16 silent overflow). F-quart-1 의 GainImap fix 만으로는 정렬 불가.
**준수**: ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.9 / §4.1.6 / §4.3 만 인용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**. Reference impl 은 spec 식에서 *처음부터* 작성 (float64 dB / linear units, Q-format 환산은 마지막 단계).

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 Working tree 상태

| 경로 | 상태 | F-quart-3 변경? |
|------|------|---|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 보존 | No |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) — F-bis-1/F-tris-1 진단 하니스 보존 | No |
| `internal/decoder/stagef_quart_diagnostic_test.go` | **modified (committed by 본 task)** — F-quart-3 reference impl 추가 | Yes (test 코드만) |
| `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-3-report.md` | **new (committed by 본 task)** | Yes (신규) |

`git diff --stat -- internal/` (task 시작·종료 양쪽):

```
internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
1 file changed, 54 insertions(+), 54 deletions(-)
```

Production 코드 (`internal/lsp/lsp_lp.go` 의 F-bis-1 P fix 외) **0 라인** 변경. `internal/synth`, `internal/postfilter`, `internal/pcm`, `internal/gain`, `internal/fcb`, `internal/pitch`, `internal/decoder/decode.go`, `internal/decoder/subframe.go` 모두 미변경. `internal/gain` 에 `export_test.go` 등 test-only export 추가 0 (플랜 §419 명시).

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 본 task 발동? |
|------|---------|---|
| **E1** (production = spec) | reference 일치 → 비선형 체인 결함 0 | **No** — Branch P sf0 Δgc_q12 = -25923, Branch S sf0 Δgc_q12 = -3348, sf1 도 양 분기 다중 차이. |
| **E2** (spec § 모호) | reference 작성 시 spec 식 모호 | **No** — 식 (66)–(74) 와 §4.3 Table 9 모두 verbatim 명시; reference impl 은 직접 도출 가능. |
| **E3** (외부 구현 인용) | ITU C / bcg729 / Sipro / FFmpeg 코드 1건이라도 인용 | **No** — PDF 단일 출처. |
| **본 task의 fix 적용** | F-quart-3 은 측정-only | **No** — production 0-수정 유지. |

E1 미발동 → **§3.9.3 GainImap fix 외에 다중 비선형 체인 fix 필수** 가 본 task 의 1차 결론 (§6 참조).

---

## 1. §3.9 식 verbatim 인용 (PDF p.23)

본 절은 `docs/superpowers/specs/itu/G729E.pdf` §3.9 (Quantization of the gains, p.22-24) 본문에서 추출한 verbatim 인용이다.

### 1.1 식 (66) — 평균 fixed-codebook 에너지

> The mean energy of the fixed-codebook contribution is given by:
> $$\bar{E} = 10 \log\!\left(\frac{1}{40} \sum_{n=0}^{39} c(n)^2\right) \quad (66)$$

(보고서 본문에서는 *명확성을 위해* 이 양을 `E̅_c` 로 부른다 — spec 의 `E̅` 기호와 후술하는 `E̅` (= 30 dB 평균) 와의 충돌을 피하기 위함이다.)

### 1.2 식 (67) — mean-removed scaled codebook energy

> Let E(m) be the mean-removed energy (in dB) of the (scaled) fixed-codebook contribution at the subframe m, given by:
> $$E(m) = 20 \log g_c + \bar{E}_c - \bar{E} \quad (67)$$

(여기서 `Ē` = 30 dB — 다음 줄 verbatim: "where Ē = 30 dB is the mean energy of the fixed-codebook excitation".)

### 1.3 식 (68) — g_c 표현

> The gain g_c can be expressed as a function of E(m), Ē and Ē_c by:
> $$g_c = 10^{(E(m) + \bar{E} - \bar{E}_c) / 20} \quad (68)$$

### 1.4 식 (69) — MA predictor

> The 4th order MA prediction is done as follows. The predicted energy is given by:
> $$\tilde{E}(m) = \sum_{i=1}^{4} b_i \, \hat{U}(m-i) \quad (69)$$
>
> where [b1 b2 b3 b4] = [0.68 0.58 0.34 0.19] are the MA prediction coefficients, and Û(m) is the quantized version of the prediction error U(m) at subframe m.

### 1.5 식 (70) — 예측 오차

> $$U(m) = E(m) - \tilde{E}(m) \quad (70)$$

### 1.6 식 (71) — predicted gain g_c'

> The predicted gain g_c' is found by replacing E(m) by its predicted value in equation (68).
> $$g_c' = 10^{(\tilde{E}(m) + \bar{E} - \bar{E}_c) / 20} \quad (71)$$

### 1.7 식 (72) — correction factor 와 prediction error 의 관계

> The correction factor γ is related to the gain-prediction error by:
> $$U(m) = E(m) - \tilde{E}(m) = 20 \log(\gamma) \quad (72)$$

### 1.8 식 (73) — pitch gain VQ

> $$\hat{g}_p = GA_1(GA) + GB_1(GB) \quad (73)$$

### 1.9 식 (74) — fixed-codebook gain VQ

> $$\hat{g}_c = g_c' \, \hat{\gamma} = g_c' \, (GA_2(GA) + GB_2(GB)) \quad (74)$$

### 1.10 §4.3 Table 9 — Û 초기값

> | Variable | Reference | Initial value |
> |----------|-----------|---------------|
> | Û(k)     | 3.9.1     | −14           |

(verbatim, PDF p.30, 표 9 마지막 행.)

---

## 2. 상수 line-by-line 검증

### 2.1 `pastErrorsDefault` (`internal/gain/decode.go:9`)

```go
const pastErrorsDefault int16 = -14336
```

§4.3 Table 9 의 Û(k) = −14 dB 를 Q10 으로 표현하면 −14·1024 = −14336. **spec-correct ✓**.

### 2.2 dB 변환 상수 4개 (`internal/gain/decode.go:18-23`)

| 상수 | 정의 | 수학적 진짜 값 (float64) | round | production | Δ |
|------|-----|-------------------|-------|----------|---|
| `dbPerLog2Q13` | 10·log₁₀(2) · 2¹³ | 24660.377244793 | 24660 | 24660 | 0 |
| `tenLog10_40Q10` | 10·log₁₀(40) · 2¹⁰ | 16405.094311198 | 16405 | 16405 | 0 |
| `invDbScaleQ15` | 1 / (20·log₁₀(2)) · 2¹⁵ | 5442.646990663 | 5443 | 5443 | 0 |
| `dbPerLog2Q10` | 20·log₁₀(2) · 2¹⁰ | 6165.094311198 | 6165 | 6165 | 0 |

각 상수의 production 값 = 수학적 진짜 값의 round-to-nearest. **4건 모두 1-LSB 이내, spec-correct ✓** (외부 구현 0 인용으로 *수학 진짜 값과 가장 가까운 정수* 를 spec-correct 기준으로 채택).

### 2.3 MA predictor 계수 b_i (`internal/tables/gain_ma.go:12`)

```go
var GainMAPredictor = [4]int16{5571, 4751, 2785, 1556}
```

§3.9 verbatim: `[b1 b2 b3 b4] = [0.68 0.58 0.34 0.19]`. Q13 변환:

| i | spec | × 2¹³ | round | production | Δ |
|---|------|-------|-------|----------|---|
| 1 | 0.68 | 5570.56 | 5571 | 5571 | 0 |
| 2 | 0.58 | 4751.36 | 4751 | 4751 | 0 |
| 3 | 0.34 | 2785.28 | 2785 | 2785 | 0 |
| 4 | 0.19 | 1556.48 | 1556 | 1556 | 0 |

**4건 모두 spec-correct ✓**.

### 2.4 Mean log-energy `GainMeanEnergyQ10` (`internal/tables/gain_ma.go:17`)

```go
const GainMeanEnergyQ10 int16 = 30720
```

§3.9 (식 (67) 다음 verbatim: "where Ē = 30 dB"). 30·1024 = 30720. **spec-correct ✓**.

### 2.5 `Pow2Table` / `Log2Table` (`internal/tables/gain_pow2.go`)

33-entry LUT 각각:
- `Pow2Table[i]` = round(2^(i/32) · 2¹⁴), i ∈ [0,32]
- `Log2Table[i]` = round(log₂(1 + i/32) · 2¹⁵), i ∈ [0,32]

본 task 는 LUT 값을 한정-검증 (endpoints + 4 mid-points spot-check):

| i | Pow2 spec | Pow2 prod | Log2 spec | Log2 prod |
|---|-----------|-----------|-----------|-----------|
| 0 | 16384.000 | 16384 | 0.000 | 0 |
| 8 | 19483.989 | 19484 | 11716.422 | 11716 |
| 16 | 23170.475 | 23170 | 19167.000 | 19167 |
| 24 | 27554.099 | 27554 | 26455.482 | 26455 |
| 32 | 32768.000 | 32767 (sat) | 32768 | 32767 (sat) |

5/5 spot-check ≤ 1-LSB. (전체 33-entry 검증은 `internal/tables/gain_tables_test.go::TestPow2TableMatchesAnalytic` 등이 별도로 수행하며 본 task 는 그 결과를 가정.) **spec-correct ✓**.

---

## 3. Zero-energy guard 분류

`internal/gain/decode.go:46-65` 의 `if ecEnergy <= 0 { ... }` 분기:

§3.9 (PDF p.22-24) 어디에도 *codebook energy 가 0 일 때의 동작* 은 명시되지 않는다. §4.4 (frame erasure concealment) 도 본 분기 와 무관. 식 (66) 의 log₁₀(0) 은 수학적으로 −∞ 이며 spec 은 이를 명시적으로 처리하지 않는다.

**분류**: **production 자체 추가 안전망** (spec § 의 침묵을 robustness-fill).

본 stimulus (frame 0 sf0/sf1) 는 c[] 에 비-0 펄스 ≥1 개 → ecEnergy > 0 → **본 guard 미진입** (= F-quart-3 결과 영향 0). 추후 frame erasure / pathological c=0 stimulus 테스트 시에만 중요.

---

## 4. Reference impl 사양

### 4.1 설계 원칙

- **언어**: Go (test-only, `internal/decoder/stagef_quart_diagnostic_test.go` 내부 추가).
- **수치 표현**: float64 dB / linear units (정밀도 보존). 마지막 단계에서만 Q14/Q12 로 round-to-nearest 환산.
- **외부 구현 0 참조**: 식 (66)/(68)–(74) + §4.3 Table 9 + GainGBK1/GainGBK2 데이터 테이블만 사용. production 의 알고리즘 helper (decodeVQ, log2Fixed, pow2Fixed, predictedLogGain, fixedCodebookEnergy) 0 호출.
- **MA predictor 계수**: §3.9 verbatim `[b1 b2 b3 b4] = [0.68 0.58 0.34 0.19]` (소수점 표현 그대로 사용; Q13 환산 미경유).

### 4.2 알고리즘 (`referenceGainState.referenceDecode`)

```text
Step 0: 첫 호출 시 pastErrorsDb[i] = -14.0 (i=0..3)              (§4.3 Table 9)
Step 1: (gp_q14, gamma_q13) = GBK1[ga] + GBK2[gb]               (식 (73)/(74) VQ part)
Step 2: gamma_true = gamma_q13 / 2^13                             (Q13 → 무차원)
Step 3: ec_int = Σ c[n]² (n=0..39)                                (Q26: c는 Q13)
Step 4: ec_true = ec_int / 2^26                                   (true linear E_c)
Step 5: ec_bar_db = 10·log₁₀(ec_true / 40)                        (식 (66))
Step 6: e_tilde_db = Σ b_i · pastErrorsDb[i]                      (식 (69))
Step 7: predicted_db = e_tilde_db + 30                            (식 (71) 의 Ẽ + Ē)
Step 8: log10_gc0_db = predicted_db − ec_bar_db                   (식 (71) 의 분자)
Step 9: gc0_true = 10^(log10_gc0_db / 20)                         (식 (71))
Step 10: gc_true = gamma_true · gc0_true                          (식 (74))
Step 11: gc_q12 = sat_int16(round(gc_true · 4096))                (Q12 환산)
Step 12: U(m) = 20·log₁₀(gamma_true)                              (식 (72))
Step 13: pastErrorsDb FIFO shift; pastErrorsDb[0] = U(m)
```

각 step 의 수식은 §1 의 verbatim 인용에서 직접 도출됨 — 알고리즘적 외부 참조 0.

### 4.3 Reference impl 코드 위치

`internal/decoder/stagef_quart_diagnostic_test.go::referenceGainState.referenceDecode` (≈ 100 LOC). 본 task commit 에 포함.

---

## 5. Production vs reference 비교 결과

`go test -run TestDiagnostic_FquartGainReferenceCrossCheck -v ./internal/decoder/` 실측치 (ALGTHM frame 0):

### 5.1 Branch P (production verbatim indexing)

#### sf0: GA=5, GB=6

| 항목 | reference (spec) | production | Δ |
|------|----------------|------------|---|
| ecBarDb | **−9.8295 dB** | (간접: 4.4375 dB — §5.3 분석) | **+14.27 dB** |
| predicted_db | 4.9400 dB | (간접: 4.9400 dB) | 0 |
| log10_gc0_db | **14.7695 dB** | (간접: 0.5025 dB) | **−14.27 dB** |
| gc_true | 8.633 | 1.671 | **÷5.17** (= 10^(−14.27/20)) |
| **gp_q14** | **13815** | **13815** | **0** |
| **gc_q12** | **32767 (saturated, ideal=35367)** | **6844** | **−25923** |
| FIFO[0] post-update Q10 | 4049 | (sf1 일치 검증 — §5.1 sf1) | — |
| U(m) ref | 3.9541 dB | — | — |

#### sf1: GA=6, GB=2 (β=clamp(prod gp_sf0=13815))

| 항목 | reference | production | Δ |
|------|----------|------------|---|
| ecBarDb (ref) | −10.0000 dB | (간접) | — |
| predicted_db (ref) | 17.1488 dB | — | — |
| log10_gc0_db (ref) | 27.1488 dB | — | — |
| gc_true (ref) | 42.668 (saturating) | — | — |
| **gp_q14** | **5498** | **5498** | **0** |
| **gc_q12** | **32767 (saturated)** | **32767 (saturated)** | **0** ← saturation 일치 |

→ Branch P sf0 prod ≠ ref (Δgc_q12 = −25923). sf1 일치는 **양측 모두 32767 saturation** 의 결과 — FIFO 동치성을 의미하지 않음.

### 5.2 Branch S (§3.9.3 inverse-mapped indexing)

#### sf0: GA=0 (= GainImap1[5]), GB=1 (= GainImap2[6])

| 항목 | reference | production | Δ |
|------|----------|------------|---|
| ecBarDb (ref) | −9.8295 dB | (간접: 4.4375 dB) | +14.27 dB |
| predicted_db (ref) | 4.9400 dB | — | — |
| log10_gc0_db (ref) | 14.7695 dB | — | — |
| gc_true (ref) | 1.013 | 0.196 | ÷5.17 |
| **gp_q14** | **1995** | **1995** | **0** |
| **gc_q12** | **4151** | **803** | **−3348** |
| FIFO[0] ref Q10 | −15006 | — | — |
| U(m) ref | −14.6538 dB | — | — |

#### sf1: GA=6, GB=3 (β=clamp(prod gp_sf0=1995))

| 항목 | reference | production | Δ |
|------|----------|------------|---|
| ecBarDb (ref) | −10.0000 dB | — | — |
| predicted_db (ref) | 4.4954 dB | — | — |
| log10_gc0_db (ref) | 14.4954 dB | — | — |
| **gp_q14** | **6516** | **6516** | **0** |
| **gc_q12** | **32767 (saturated, ideal≈45494)** | **8805** | **−23962** |

→ Branch S sf0 prod ≠ ref (Δgc_q12 = −3348). sf1 prod ≠ ref (Δgc_q12 = −23962, **간접 FIFO 검증**: production sf1 gc_q12 = 8805 ≠ reference 32767, 즉 sf0 의 FIFO 업데이트 가 production-reference 동치가 아닌 *비선형 체인 자체가 다른 dB-domain 에서 동작* 함을 시사).

### 5.3 Production ecBarDbQ10 +14.27 dB 오류의 정량 분석

production 의 `ecLog2Q10 → ecDbQ10 → ecBarDbQ10` 체인을 수치 트레이스 (sf0 stimulus, c[] = `[8192,8192,8192,8192,0,...,1639,1639,1639,1639,...]`):

```
ec_int = 4·8192² + 4·1639² = 279,180,740
log2(ec_int) = 28.0566 → ecLog2Q10 = 28730  (Q10)
ecLog2Q10 · 24660 = 708,479,800
+ 4096 = 708,483,896
>> 13 = 86,485                              ← 논리적 ecDbQ10 (= 84.46 dB)
int16(86485) = 20949                         ← int16 truncation: 86485 mod 65536 = 20949
ecBarDbQ10 = 20949 − 16405 = 4544           (= 4.4375 dB)
```

spec answer:
```
ec_true = ec_int / 2^26 = 4.16
ec_bar_spec = 10·log10(4.16/40) = −9.8295 dB
```

차이: **+14.267 dB** = **−78.268 dB (lost) + 64.000 dB (overflow recovery)**

- **−78.268 dB** = `−26·10·log10(2)` = `internal/gain/energy.go:18-22` docstring 이 명시한 *Q26-vs-Q0 Q-format 보정* 의 누락. docstring verbatim:
  > "Callers must ALSO apply a Q-format correction to account for the Q26-vs-Q0 mismatch against the spec's log2 of a Q0 sum: see the comment in decode.go at the `ecLog2Q10 = ... - 26*1024` line."

  그러나 `internal/gain/decode.go:70-72` 의 caller 에는 `- 26*1024` 보정이 **부재**:
  ```go
  ecLog2Q10 := log2Fixed(ecEnergy)
  ecDbQ10 := int16((int32(ecLog2Q10)*dbPerLog2Q13 + (1 << 12)) >> 13)
  ecBarDbQ10 := fixed.Sub(ecDbQ10, tenLog10_40Q10)
  ```

  **확정 spec-위반 #1**: docstring 이 약속한 Q-format 보정 미적용.

- **+64.000 dB** = int16(86485) 의 silent truncation. 86485 > 32767 이므로 int16 캐스트가 low 16-bit 만 보존하고 64 dB Q10 (= 65536 Q10) 가 silent-discard. F-tris-2 §3.9.3 분석에서 식별되지 않은 **새 spec-위반**:

  **확정 spec-위반 #2**: `int16(...)` truncation 으로 silent overflow.

이 두 결함이 *부분적으로 상쇄* 하여 production gc_q12 가 *겉보기에 합리적인* 1.67 (= 6844 / 4096) 을 산출하지만, spec true gc = 8.63 → 환산값은 ÷5.17 (= 0.193x = 10^(−14.267/20)) 만큼 작다.

### 5.4 Step 9 표 분류

| 결과 | 본 측정 | 의미 |
|------|------|------|
| 두 분기 모두 prod=ref | ✗ | (해당 없음) |
| **두 분기 모두 prod≠ref** | **✓ 본 case** | **비선형 체인 결함 1+개 (indexing 무관) → 다중 fix 필수** |
| 한 분기만 prod=ref | ✗ | (해당 없음) |

**시나리오: 비선형 체인 결함 다중**.

---

## 6. 결론 — 비선형 체인 결함 후보 ranking

### 6.1 본 task 가 확정한 사실

1. **§3.9 식 (66)/(68)/(69)/(70)/(71)/(72)/(73)/(74) 모두 verbatim 인용 가능**, 모호 0 (E2 미발동).
2. **dB 변환 상수 4건 (24660 / 16405 / 5443 / 6165) 모두 1-LSB 이내 spec-correct**.
3. **MA predictor b_i (5571/4751/2785/1556) + GainMeanEnergyQ10 (30720) + pastErrorsDefault (−14336) 모두 spec-correct**.
4. **`fixedCodebookEnergy` 의 docstring 이 명시한 Q26-vs-Q0 보정 (−26·1024) 이 caller 에서 누락**. 이는 `internal/gain/decode.go:70-72` 의 확정 spec-위반.
5. **`ecDbQ10` 계산에 `int16(...)` truncation 이 silent overflow 를 유발**. ALGTHM frame 0 sf0 input (ec_int = 2.79e8) 에서 86485 → 20949 로 64 dB 손실. 확정 spec-위반.
6. 두 결함이 *부분 상쇄* 하여 net +14.267 dB 오류 → gc 가 spec 의 0.193x. **F-quart-1 의 §3.9.3 GainImap fix 만으로는 sample 1 |Δ|=2 잔존을 설명**: spec-fix branch 에서도 gc 는 spec 의 0.193x → synth.Filter 출력이 spec 보다 작아 근접 0 으로 수렴 → 일부 sample 만 우연히 matching.
7. `Pow2Table` / `Log2Table` 자체는 spec-correct (5/5 spot-check ≤ 1 LSB).
8. zero-energy guard 는 production 자체 추가 안전망 (spec § 침묵) — frame 0 sf0/sf1 에서는 미진입.

### 6.2 비선형 체인 결함 후보 ranking

| 순위 | 결함 | 위치 | 근거 | 영향 (frame 0 sf0) |
|------|------|------|------|------|
| **1** | **`ecDbQ10` int16 silent overflow** | `internal/gain/decode.go:71` | §5.3 수치 트레이스 (86485 → 20949, +64 dB silent recovery) | gc 가 spec 의 약 0.193x (단독 영향 측정 예상치) |
| **2** | **Q26-vs-Q0 보정 누락 (`-26*1024`)** | `internal/gain/decode.go:70-72` (caller side) | `internal/gain/energy.go:18-22` docstring verbatim 이 *명시적으로 약속하나 caller 미적용* | gc 가 spec 의 1/2^13 (= 단독 영향 시) |
| 3 | (잔여) §3.9.3 GainImap inverse-map 누락 | `internal/gain/vq.go::decodeVQ` | F-quart-1 §3 식별, F-quart-3 §5.2 의 Branch S 도 (1)+(2) 결함 영향 받음 | gc 의 codebook entry 자체 변경 |

### 6.3 Fix sequence 권고 (F-quint plan 상세 추후)

순위 (1)+(2) 는 동시에 fix 해야 한다 — 각각 단독 fix 시 ÷64 dB 또는 ×8192 의 큰 편차 도입. 동시 fix 시:

- `ecLog2Q10 := log2Fixed(ecEnergy) - 26*1024` (Q-format 보정 적용)
- `ecDbQ10 := int32(ecLog2Q10 * dbPerLog2Q13 + (1 << 12)) >> 13` (int32 보존 — 후속 `fixed.Sub` 도 int32 또는 saturating int16 변환 필요)
- 이후 `tenLog10_40Q10` 차감 시 saturate 가 발생하지 않는지 재검증.

순위 (3) 의 GainImap fix 는 (1)+(2) 와 직교 — 별도 fix.

세 결함 *동시* fix 후 frame 0 sf0/sf1 의 PST/2 정렬도 재측정 (F-quint plan).

### 6.4 F-quart-4 진입 권고

본 task 의 결론으로 F-quart-1 의 시나리오 S2 (단일 fix sample 1 |Δ|=2 잔존) 가 **순위 (1)+(2) 결함의 상쇄적 합** 으로 *완전 설명* 됨. F-quart-4 (종합 분석 + F-quint plan 권고) 진입 가능. F-quart-4 에서 필요한 추가 진단 0 — 본 task 가 충분.

---

## 7. 부록 — go test 출력 raw

```
=== RUN   TestDiagnostic_FquartGainReferenceCrossCheck
[P] sf0  GA=5 GB=6
[P] sf0  E̅_c=  -9.8295 dB   Ê(m)=   4.9400 dB   log10(g_c0)·20=  14.7695 dB
[P] sf0  PROD: gp_q14= 13815  gc_q12=  6844
[P] sf0  REF : gp_q14= 13815  gc_q12= 32767   (gc_true=8.633396)
[P] sf0  Δgp_q14 = +0   Δgc_q12 = -25923
[P] sf0  REF post-update FIFO Q10 = [4049 -14336 -14336 -14336]   U(m)=3.9541 dB
[P] sf1  PROD: gp_q14=  5498  gc_q12= 32767
[P] sf1  REF : gp_q14=  5498  gc_q12= 32767   (gc_true=42.667862)
[P] sf1  Δgp_q14 = +0   Δgc_q12 = +0       ← 양측 saturation 우연 일치
[S] sf0  GA=0 GB=1
[S] sf0  PROD: gp_q14=  1995  gc_q12=   803
[S] sf0  REF : gp_q14=  1995  gc_q12=  4151   (gc_true=1.013413)
[S] sf0  Δgp_q14 = +0   Δgc_q12 = -3348
[S] sf0  REF post-update FIFO Q10 = [-15006 -14336 -14336 -14336]   U(m)=-14.6538 dB
[S] sf1  PROD: gp_q14=  6516  gc_q12=  8805
[S] sf1  REF : gp_q14=  6516  gc_q12= 32767   (gc_true=11.108868)
[S] sf1  Δgp_q14 = +0   Δgc_q12 = -23962
--- PASS: TestDiagnostic_FquartGainReferenceCrossCheck (0.00s)
```

(생략: dump 의 fixed-codebook c[] 출력 — `go test -v` 로 재현 가능.)
