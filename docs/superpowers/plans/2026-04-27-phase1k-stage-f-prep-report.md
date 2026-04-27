# Phase 1k Stage F Prep Report (F-prep-1 + F-prep-2)

**상태**: 부분 보고서 (F-prep-1, F-prep-2 완료, F-fix 미수행 — 사용자 분기 결정 대기).
**일자**: 2026-04-27
**범위**: ITU 참조 C, bcg729, FFmpeg G.729 등 모든 기존 구현물 미참조. ITU-T G.729 §3.2.6 / §3.10 / §A.4.2 + LSP-LP 표준 이론 (Schur–Cohn) 만 사용.

---

## 1. 커밋 요약

| 커밋 | 한 줄 요약 |
|------|-----------|
| `23c5bae` | `test(lsp): F-prep-1 ALGTHM frame 0 sf0 A(z) minimum-phase contract` — Schur–Cohn step-down 어서션(observation-only). |
| `b02d9d3` | `test(synth): F-prep-2 filterSubframe closed-form + saturation recovery contracts` — 임펄스 응답 폐형식 + §3.10 ÷4+×4 컨트랙트 + ALGTHM sf0 per-sample 경계 진단. |

회귀 게이트: `go test -race ./...` PASS, `go vet ./...` silent. `internal/fixed` 벤치 0 allocs/op 유지.

---

## 2. F-prep-1 결과 — A(z) Minimum-Phase 검증

### 입력
ALGTHM.BIT frame 0 sf0의 LSP 인덱스 (Stage D-bis Task 3 확정값):
`L0=1, L1=105, L2=17, L3=0` → `lsp.Decoder.Decode` → `sf1A` (Q12).

### 출력 a[] (Q12 → float)

```
a[] (int16 Q12) = [4096 -2197 -375 -924 7735 294 665 7844 -1010 145 -33]
a[] (float)     = [1, -0.5364, -0.0916, -0.2256, +1.8884, +0.0718, +0.1624, +1.9150, -0.2466, +0.0354, -0.0081]
```

### Schur–Cohn step-down 결과

```
step m=10: k=-0.008057   |k|<1 ✓
step m=9 : k=+0.031081   |k|<1 ✓
step m=8 : k=-0.230895   |k|<1 ✓
step m=7 : k= ──────── |k_7| = 1.897114  ≥ 1   ✗  (FAIL)
```

### 스펙 인용 및 판정

ITU-T G.729 §3.2.6 (LP filter stability)는 양자화된 LP가 안정적이어야 하며, LSP가 strictly ordered 조건 0 < ω₁ < ω₂ < … < ω₁₀ < π를 유지하면 A(z)가 minimum-phase임을 보장한다. 표준 LSP→LP 이론 (Schur–Cohn step-down)에 의해 monic A(z) = 1 + Σ aᵢ z⁻ⁱ가 minimum-phase일 필요충분조건은 모든 reflection coefficient |kₘ| < 1, m=10..1.

**판정: FAIL.** m=7에서 |k_7| = 1.897이 1을 크게 초과 — A(z)는 non-minimum-phase. 즉 1/A(z)는 unit disk 외부 극점을 가지므로 IIR 출력이 발산. `|a[4]|=1.888`, `|a[7]|=1.915`라는 절대값 자체는 일반 LP에서 **합법** 일 수 있으나 (Q12 a_i는 ±2.0 범위 표현 가능), 본 케이스에서는 step-down으로 검증된 실제 불안정성을 동반함.

이 결과는 Stage D-bis §5 위험 노트(LSP→LP unstable 가능성)를 직접 확증한다.

### Verbatim 로그

```
=== RUN   TestALGTHMFrame0SF0_AzStability
    stability_test.go:78: a[] (int16 Q12) = [4096 -2197 -375 -924 7735 294 665 7844 -1010 145 -33]
    stability_test.go:79: a[] (float, Q12-normalized) = [1 -0.536376953125 -0.091552734375 -0.2255859375 1.888427734375 0.07177734375 0.162353515625 1.9150390625 -0.24658203125 0.035400390625 -0.008056640625]
    stability_test.go:103: step m=10: k=-0.008057
    stability_test.go:103: step m=9: k=0.031081
    stability_test.go:103: step m=8: k=-0.230895
    stability_test.go:90: OBSERVATION (F-prep-1): A(z) NOT minimum-phase at step m=7: |k_m|=1.897114 >= 1; Stage F branch points at lsp.* (LSP→LP conversion or decoder bug). Promoted to t.Errorf in F-fix.
--- PASS: TestALGTHMFrame0SF0_AzStability (0.00s)
```

(테스트는 회귀 게이트 보존을 위해 t.Logf로 다운그레이드 — F-fix 단계에서 t.Errorf로 승격 예정.)

---

## 3. F-prep-2 결과 — synth.Filter IIR 경계 컨트랙트

### 3.1 임펄스 응답 폐형식 (TestFilter_ImpulseResponse_OnePoleClosedForm)

A(z) = 1 − 0.5·z⁻¹ (안정), u[0]=8192, u[1..]=0. 폐형식: s[n] = 8192·0.5ⁿ.

```
s[0..15] = [8192 4096 2048 1024 512 256 128 64 32 16 8 4 2 1 1 1]
```

n=0..13까지 ±1 LSB 이내 일치. n=14,15에서 1 LSB 정수 라운딩 잔류(허용 오차 내). **PASS.**

**함의**: `synth.onePass`의 LMult/LMsu/LShl(_, 3)/Round 누산 체인 자체는 안정 A(z)에 대해 정확. 즉 **분기 Q-onePass는 배제됨**.

### 3.2 §3.10 Saturation Recovery 스케일링 인수 (TestFilter_SaturationRecovery_ScalingFactorMatchesSpec)

자극: A(z)=1−0.99·z⁻¹ (a[1]=−4055 Q12), u[*]=20000 일정.

ITU-T G.729 §3.10 인용: *"When overflow occurs, the speech samples and the filter memory are divided by 4 and the filtering is re-done. The output is multiplied by 4 with saturation."*

현재 구현 `internal/synth/filter.go:31-52`는 ÷2 + ×2 (1/2 스케일 후 ×2 복원) 사용 — **스펙 ÷4 + ×4와 명백히 불일치**.

```
Pass-2 saturation count = 39 / 40
s[36..39] = [32767 32767 32767 32767]
OBSERVATION (F-prep-2 Q-saturation): Pass-2 saturation count = 39 (samples == ±max).
ITU-T G.729 §3.10 specifies divide-by-4 + multiply-by-4 saturation recovery; current
implementation uses ÷2 + ×2 (filter.go:33-51), which exceeds Word16 range under this
stimulus. F-fix promotes to t.Errorf.
```

**함의**: 분기 Q-saturation은 **실재하는 spec 위반**. 다만 ALGTHM sf0의 실제 발산이 이 위반에 의해 주도되는지는 §3.3에서 확인.

### 3.3 ALGTHM frame 0 sf0 per-sample 경계 진단 (TestALGTHMFrame0SF0_SynthFilter_PerSampleBoundary)

입력:
- a[] = Stage D-bis 확정값 (위 §2의 unstable A(z))
- excitation u = BuildExcitation(gpQ14=13815, gcQ12=6844, v=0, c=4-pulse{0,1,2,3,20,21,22,23 모두 +Q13})
- pastSynth = 0 (codec-start)

```
u[0..15]  = [2 2 2 2 0 0 0 0 0 0 0 0 0 0 0 0]
s[0..15]  = [2 4 4 4 0 -6 -10 -18 -20 -10 2 32 68 80 88 46]
s[16..39] = [-66 -178 -334 -438 -334 -110 400 1146 1666 1990 1548 -244
             -2692 -5966 -8932 -8840 -5492 3206 17748 31934 32767 32767 15352 -31838]
nSat = 2 / 40   (Pass-2 saturation 발화는 sample 20-21에서만)
```

**경계 분석**:
- sample 0: s=2 (PST=2와 일치 — Phase 1i 잠금 정상)
- sample 1..3: s=4 (PST=2 대비 2배 — 이미 발산 시작; +6 dB)
- sample 5: s=-6 (PST=1 대비 |6|/|1| → +15.6 dB; Stage D-bis 보고서의 "sample 5에서 |s·2|=12 vs |PST|=1 → +21.6 dB"와 정합 — 본 진단의 raw s는 ×2 production scale 이전 값)
- sample 8: |s|=20 → 본격적 지수 발산 진입
- sample 19..23: 천 단위 진입 (1990 → 32767)
- sample 20-21: Word16 saturation (Pass-2 가드 발화)
- 이후 부호 진동 + ±32767 pin

**핵심 관찰**: 발산은 sample 1부터 이미 시작 — saturation 가드는 sample 20에서야 처음 발화. 즉 14 dB 발산의 **주된 원인은 saturation/§3.10 두-패스 처리가 아니라 unstable A(z)에 의한 IIR 자체 발산**. saturation 가드는 단지 발산이 Word16 한계에 닿았을 때만 사후 처리할 뿐.

§3.10 두-패스 가드 발화 동작:
- 발화 빈도: 본 sf0 전체에서 nSat=2 (sample 20-21만 ±32767 도달)
- 발화 후에도 발산 추세는 계속 (sample 22~39의 천~만 단위 진동)
- ÷2+×2 vs ÷4+×4 차이는 본 sf0에서 결정적이지 않음 — Pass-1이 거의 모든 샘플에서 overflow 미발생, Pass-2가 거의 트리거되지 않음. 발산 자체가 unstable A(z)에서 옴.

---

## 4. 분기 추천 (사용자 결정 대기)

### 데이터 매트릭스

| 시그널 | 결과 | 함의 |
|--------|------|------|
| F-prep-1 A(z) Schur–Cohn | **FAIL** at m=7, |k|=1.897 | A(z) non-minimum-phase 확정 |
| F-prep-2 임펄스 응답 폐형식 | PASS | onePass 누산 체인 무죄 → Q-onePass 배제 |
| F-prep-2 §3.10 ÷4+×4 컨트랙트 | **FAIL** (39/40 sat on 합성 자극) | Q-saturation은 실재 spec 위반 |
| F-prep-2 ALGTHM sf0 per-sample | s[1]=4 vs PST=2부터 발산 | 14 dB 주원인은 unstable A(z) (sample 1부터 발산 시작, saturation은 sample 20에서야 첫 발화) |
| Stage D-bis Task 3 sample 5 발산 | 알려진 fact | F-prep-1 + F-prep-3 모두 정합 |

### 추천 (본 결정은 데이터 기반 권고이며, 사용자가 최종 분기 선택)

1. **분기 P (lsp.\*)** — **1순위 추천**.
   - 근거: F-prep-1이 A(z) non-minimum-phase를 직접 증명 (|k_7|=1.897). F-prep-2 per-sample 진단은 발산이 sample 1부터 시작되며 saturation 가드는 sample 20에서야 발화 → 14 dB 발산은 unstable A(z)가 직접 원인.
   - 수정 위치 후보: `internal/lsp/lsp_lp.go::lspToLP`, `internal/lsp/decoder.go::Decode` (단계 7~8: interpolateLSP / lspToLP의 Q-포맷 chain), 또는 §3.2.4의 추가 안정화 후처리.
   - 검증 게이트: 분기 P 적용 후 `TestALGTHMFrame0SF0_AzStability` 가 모든 m에서 |k_m|<1 → t.Errorf로 승격 후 PASS여야 함.

2. **분기 Q-saturation (synth.Filter ÷2+×2 → ÷4+×4)** — **2순위 추천 (분기 P 후속)**.
   - 근거: F-prep-2 saturation 컨트랙트가 명시적 §3.10 인용 위반을 노출. 그 자체로 spec 준수 fix가 필요. 다만 ALGTHM sf0의 14 dB 발산을 해소하는 *주된* 수단은 아님 (saturation 가드 발화 빈도 2/40).
   - 추천 시점: 분기 P로 A(z)를 안정화 → Phase 1i sample-0 잠금 무회귀 확인 → 잔류 차이가 있다면 분기 Q-saturation을 추가. 분기 P가 발산을 완전히 제거하면 Q-saturation은 spec-compliance 작업으로 별도 분리 가능.

3. **분기 Q-onePass** — **배제**.
   - 근거: F-prep-2 임펄스 응답 폐형식 PASS. `onePass`의 Q-포맷/누산 정확성 입증.

### 권고 순서
**P 우선 → V1 (sample 40 가드) → 잔류 시 Q-saturation**. Stage F 플랜 Task 3 Step 1의 분기 결정 규칙 ("Task 1 FAIL → 분기 P") 과 일치.

---

## 5. Escape Hatch 확인

| 항목 | 상태 |
|------|------|
| ALGTHM frame 0 sample 0 잠금 (got=2 want=2, `736beba`) | **유지** (regression test 회귀 없음) |
| Phase 1i 수정 3건 (`f24add7`, `a1244d5`, `3670672`) | **유지** |
| Stage D 17 Q-format 어서션 | **유지** |
| Stage D-bis 3 multi-pulse 어서션 | **유지** |
| `internal/fixed` 벤치 0 allocs/op | **유지** (Add/LMult/LMac/DivS/NormL 모두 0 B/op 0 allocs/op) |
| `go test -race ./...` | PASS |
| `go vet ./...` | silent |

**Escape hatch 발화 없음.** F-prep-2 완료 후 정상 정지.

---

## 6. 다음 단계 (사용자 액션 필요)

1. 사용자는 §4의 추천을 검토 후 분기 P / Q-saturation / Q-onePass 중 하나(혹은 P 우선 + Q-saturation 후속) 선택.
2. 선택된 분기에 대해 Stage F Task 3 (F-fix) 진입.
3. F-fix는 production 수정 + 어서션 t.Errorf 승격 + sample 40 가드 추가를 단일 커밋으로 수행 (Stage F 플랜 Task 3 Step 4 참고).

본 보고서는 분기 결정에 필요한 데이터만 제공하며, F-fix 진행 여부와 분기 선택은 사용자 권한에 둠.
