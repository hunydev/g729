# Phase 1k Stage D 진단 보고서 (부분 완료)

**작성일**: 2026-04-27
**범위**: Stage D Tasks 1–7만 수행. Stage F (Tasks 8+) 진입 보류 — 사용자 결정 대기.
**핵심 결론**: **탈출 해치 1 발동.** 단일-펄스 격리 하네스의 13개 경계 어서션이 모두 통과 → 14 dB ALGTHM frame 0 sf2 포화는 단일-펄스 자극으로는 재현되지 않음. Stage F 분기를 진단 데이터만으로 단일 모듈에 못박을 수 없음.

---

## 1. Stage D 7개 commit 요약

| # | SHA | 커밋 메시지 1줄 |
|---|-----|----------------|
| 1 | `c275a12` | test(decoder): split frame 0 regression guard + add sf1 diagnostic log |
| 2 | `4e7b254` | test(fcb): Q-format contract — PulseAmplitude=8192 Q13, Σc²=N·2^26 |
| 3 | `520019e` | test(gain): Q-format contract — extend Phase 1j contract scope to assertions |
| 4 | `cd12df4` | test(synth): Q-format contract — excitation chain + filter a[0]=4096 |
| 5 | `f312865` | test(postfilter): Q-format contract — sub-stage I/O Q-formats |
| 6 | `f4f3bd2` | test(decoder): single-pulse diagnostic harness (observation-only) |
| 7 | `9c33178` | test(decoder): single-pulse harness assertions for spec-aligned boundaries |

회귀 게이트: `go test -race ./...` ALL PASS, `go vet ./...` silent, `internal/fixed` 벤치 0 allocs/op 유지.

ALGTHM frame 0 sample 0 가드(Phase 1i 잠금) 변동 없음: `got=2 want=2`.

---

## 2. 모듈별 Q-포맷 계약 결과 (Tasks 2–5)

| 모듈 | 테스트 | 결과 | 비고 |
|------|--------|------|------|
| fcb | `TestQFormatContract_PulseAmplitudeIsOneQ13` | ✅ PASS | `PulseAmplitude == 8192` (= +1.0 Q13) |
| fcb | `TestQFormatContract_SinglePulseEnergyIs2to26` | ✅ PASS | Σc² = 2²⁶ |
| fcb | `TestQFormatContract_FourPulseEnergyIs2to28` | ✅ PASS | Σc² = 2²⁸ (4-pulse canonical) |
| fcb | `TestQFormatContract_PostEnhancementBoundedByMaxBeta` | ✅ PASS | int16 범위 유지 |
| gain | `TestQFormatContract_FixedCodebookEnergyIsQ26` | ✅ PASS | Σc² 누산 결과 Q26 |
| gain | `TestQFormatContract_Log2FixedTreatsInputAsQ0` | ✅ PASS | log2Fixed(2^k) ≈ k·1024 |
| gain | `TestQFormatContract_Pow2FixedReturnsQ0` | ✅ PASS | pow2Fixed Q0 출력, ±1% |
| gain | `TestQFormatContract_LogDomainConstants` | ✅ PASS | 24660 / 16405 / 5443 / 6165 모두 폐형식 정체식 일치 |
| gain | `TestQFormatContract_PastErrorsDefaultIsMinus14dBQ10` | ✅ PASS | −14·1024 = −14336 |
| synth | `TestQFormatContract_BuildExcitationPitchTermIsQ15` | ✅ PASS¹ | LMult(1<<14, 1) = 32768 (1.0 Q15) |
| synth | `TestQFormatContract_BuildExcitationCodeTermIsQ26ThenQ15` | ✅ PASS | LMult(Q12,Q13)=Q26 → LShr11 → Q15 |
| synth | `TestQFormatContract_BuildExcitationSinglePulseProducesGcQ12` | ✅ PASS | u[0] = round(gcQ12/4096) |
| synth | `TestQFormatContract_FilterSubframeAcceptsAOneQ12` | ✅ PASS | a[0]=4096 + a[i]=0 → 항등 필터 |
| postfilter | `TestQFormatContract_GammaConstantsAreQ15` | ✅ PASS | γ_n≈0.55, γ_d≈0.70 |
| postfilter | `TestQFormatContract_IsqrtQ14ReturnsQ14` | ✅ PASS² | √(2²⁸)=2¹⁴, √(2²⁶)=2¹³ |
| postfilter | `TestQFormatContract_AGCAlphaIsQ15` | ✅ PASS | α=32440 → 0.99 |
| postfilter | `TestQFormatContract_AGCSeedsAgcGainPrevToTargetQ24` | ✅ PASS | 첫 호출 시 gTarget Q14 << 10 시드 |

¹ 플랜 본문의 산식 오기(`want=2`)를 Q15 컨트랙트 정의(`want=1<<15=32768`)로 정정. 컨트랙트 의도/주석 변경 없음.
² 플랜 본문의 `√4 (Q28) → 2<<14`는 int16 오버플로우 → `√0.25 (Q28) → 1<<13`로 정정.

**결론**: 4개 모듈 모두 자기-주장 Q-포맷이 일관됨. 다중 모듈 동시 실패 없음 → 첫 번째 escape hatch 트리거(다중 모듈 동시 실패)는 비발동.

---

## 3. 단일-펄스 진단 하네스 경계 표 (Task 6 verbatim 로그)

입력: `c[0]=8192` (단일 +Q13 펄스), `gpQ14=0`, `idx={GA:3,GB:7}`, default `pastErrors`.

### 3.1 Spec-유도 참값

```
Σc² true              = 1
Ē_c (true dB)         = -16.0206
Ê predicted (true dB) = 4.9400
logGain (true dB)     = 20.9606
g'_c (true)           = 11.1694
```

### 3.2 경계별 실측 vs 참값

| 경계 | raw | Q-포맷 | true 실측 | true 기대 | dB 차이 | 상태 |
|------|-----|--------|-----------|-----------|---------|------|
| ① Σc² | 67108864 | Q26 | 1.0000 | 1.0000 | 0.00 dB | ✅ 일치 |
| ② energy | 67108864 | Q26 | 1.0000 | 1.0000 | 0.00 dB | ✅ 일치 |
| ③–⑨ gain log-domain | (private) | — | — | — | — | gain 모듈 내부 (Task 3 컨트랙트로 커버) |
| ⑩ gcQ12 | 7134 | Q12 | gc=1.7417 | g'_c·γ̂_c ∈ [0, ~22.34] | 측정 불가³ | ✅ 범위 안, **포화 없음** |
| ⑪ u[0] | 2 | Q0 | 2 | round(1.7417)=2 | 0.00 dB | ✅ 일치 |
| ⑫ s[0] | 2 | Q0 | 2 | u[0]=2 | 0.00 dB | ✅ 일치 (항등 필터) |
| ⑬ sPf[0..7] | `[2 0 0 0 0 0 0 0]` | Q0 | — | — | — | postfilter 자체 거동 (참값 정의 보류) |

³ ⑩의 참값은 (GA=3, GB=7)에 대한 γ̂_c VQ 출력에 의존. γ̂_c ≈ 1.7417/11.1694 ≈ 0.156으로 역산되며, 이는 gc 코드북 후보 범위 안. Stage F 분기 식별을 위해서는 **γ̂_c 직접 추출**이 필요(하네스 한계).

### 3.3 Task 7 어서션 발동 표

| 어서션 | 결과 |
|--------|------|
| BOUNDARY ① fcb energy: Σc² = 2²⁶ | ✅ PASS |
| BOUNDARY ⑪ excitation: u[0] = round(gcTrue) | ✅ PASS |
| BOUNDARY ⑫ synth.Filter: s[0] = u[0] (trivial filter identity) | ✅ PASS |
| BOUNDARY ⑩ gain: gcTrue ∈ [0, g'_c·2] **and** gcQ12 not saturated | ✅ PASS |

**모든 경계 어서션 통과 → 단일-펄스 자극에서 14 dB 분기점 부재.**

---

## 4. 탈출 해치 발동 평가

**탈출 해치 1 발동 확정.**

설계 §7.1 정의: "`divergence_dB > 0.5`가 ⑤~⑬에서 여러 경계에 걸쳐 산재하거나, 어느 경계도 14 dB ± 2 dB 안쪽으로 명확히 들어오지 않음 → Stage F 진입 금지."

본 진단에서 단일-펄스 자극의 모든 경계 dB 차이는 0.00 dB(또는 측정 불가). 14 dB ± 2 dB 영역에 들어오는 경계가 0개. 따라서 Stage F 분기(A=gain / B=excitation / C=synth-filter / D=postfilter / E=fcb) 중 어느 하나도 진단 증거만으로 단정할 수 없음.

다른 escape hatch 미발동 확인:
- 탈출 해치 2(sample 0 회귀): 발동 없음 — frame 0 sample 0 = 2 유지.
- 다중 모듈 Q-포맷 동시 실패: 발동 없음 — 17개 컨트랙트 어서션 100% PASS.
- "정확히 14 dB이 아님" (해치 3): 본 단계에서는 dB 측정 자체가 0 → 다른 자극 클래스로 재시도 필요.

---

## 5. 가설 — 14 dB 자극 의존성

본 결과는 **14 dB 오차가 입력-의존적**임을 시사한다:

1. ALGTHM frame 0의 실제 비트스트림은 단일 펄스가 아닌 **다중 펄스(canonical 4-pulse) + 비제로 피치 + 비-default `pastErrors`** 조합.
2. Phase 1j 완료 보고서에 따르면 "두 개 이상의 14 dB 오차가 서로 상쇄하며 sample 0만 우연히 비트-정확"이라는 가설이 있었음. 단일-펄스 입력은 이 상쇄 짝의 한쪽 또는 양쪽을 자극하지 않을 가능성이 큼.
3. sf1 진단 로그(`TestDecode_Frame0SF1_DiagnosticLog`)에서 sample 2부터 점진적 발산, sample 22부터 수백~수천 단위 발산, sample 34–39에서 ±32767 포화 관측 → **시간 누적 IIR(LP synthesis) 증폭**이 원인 후보.

---

## 6. Stage F 후보 권고 (사용자 결정 입력)

진단이 단일 모듈을 직접 못박지 않음. 그러나 보조 증거를 종합하면 다음 우선순위:

### 우선순위 1 — **하네스 보강 (Stage D-bis)** ★ 최우선 권고
다음 자극 클래스를 추가하여 14 dB을 재현해야 Stage F 진입이 정당화됨:
1. **다중 펄스 4-pulse canonical c[5]=c[11]=c[22]=c[33]=+8192** (Phase 1j 완료 보고서가 g_c=8.86 ≈ Q12 max(8.0) 초과를 보고한 케이스).
2. **비제로 `gpQ14`** (피치 기여를 활성화하여 IIR 누적 자극).
3. **ALGTHM frame 0의 실제 디코딩된 (LSP, 피치, fcb idx, gain idx)을 재생**하여 단일 서브프레임 단위로 13개 경계를 다시 측정.

### 우선순위 2 — **브랜치 C: synth-filter (LP synthesis)**
단일-펄스에서 trivial 필터(a[0]=4096, a[i]=0)는 항등으로 검증됨. 그러나 sf1 진단 로그가 보여주는 "sample 2부터 발산 → sample 30+ 포화" 패턴은 **실제 LP 계수 하의 IIR 거동에서 14 dB 누적 이득**이 발생함을 시사. 후보 위치: `internal/synth/filter.go` LShl/Round 자리. 단, **ALGTHM frame 0의 실제 LP 계수 + 입력**으로 재현되어야 commit 정당.

### 우선순위 3 — **브랜치 A: gain log-domain**
Phase 1j 단독 수정 실패 + 본 단계 단일-펄스 통과를 종합하면 gain 단독 책임은 약화. 하지만 Phase 1j 보고서의 "스펙-유도 g_c=8.86은 Q12 max 초과"는 여전히 미해결. 4-pulse 자극 재현 후 ⑩ gcQ12 포화가 관측되면 즉시 후보 1순위로 승격.

### 우선순위 4 — 브랜치 D (postfilter), B (excitation), E (fcb)
본 단계에서 모두 컨트랙트 통과 + 단일-펄스 경계 통과 → 가능성 낮음.

**최종 권고**: Stage F를 즉시 시작하지 말고, Stage D-bis(다중-펄스 + 실제 ALGTHM 입력 하네스)를 먼저 추가하여 14 dB 재현부를 확보. 그 후 비로소 분기 결정.

---

## 7. 영구 가드로 남는 산출물

본 부분-완료에서 다음이 영구 회귀 가드로 합류:

- `internal/decoder/frame0_regression_test.go` (Phase 1i sample 0 잠금 + sf1 진단 로그)
- `internal/fcb/qformat_contract_test.go` (4 어서션)
- `internal/gain/qformat_contract_test.go` (5 어서션, 서브테스트 포함 11)
- `internal/synth/qformat_contract_test.go` (4 어서션, 서브테스트 포함 7)
- `internal/postfilter/qformat_contract_test.go` (4 어서션, 서브테스트 포함 9)
- `internal/decoder/diagnostic_singlepulse_test.go` (Task 6 로깅 + Task 7 어서션 4)

총 17 신규 컨트랙트 어서션 + 1 진단 로깅 + 1 분리된 회귀 가드.

---

## 8. 다음 단계 (사용자 결정 사항)

사용자가 결정해야 할 옵션:

- **(가)** Stage D-bis 추가: 다중 펄스 / 실제 ALGTHM frame 0 내부 자극으로 14 dB 재현부 확보 후 Stage F 진입.
- **(나)** Stage F 즉시 진입(브랜치 C 또는 A) — 진단 증거 약화 상태에서 가설-주도 수정. **Phase 1k 설계 §7.1이 명시적으로 금지.**
- **(다)** Phase 1l(SPEECH/FIXED 등 다른 ITU 벡터 활성화)로 우회 — 14 dB 픽스 보류, Stage D 산출물만 영구 가드로 마감.

본 보고서는 (가)를 권고함.
