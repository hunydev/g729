# Phase 1k Stage F 부분 보고서 (분기 P 진단 결과 — Escape Hatch 1 발동)

**상태**: 부분 보고서. 분기 P (lsp 모듈) 결함 위치 확정 + 후보 수정 검증 완료. **Phase 1i sample-0 잠금 회귀로 인해 escape hatch 1 발동 → 작업 트리 롤백, 미커밋.** 추가 분석(상류 P fix + 하류 보정 동시 수정) 필요.

**작성일**: 2026-04-27

**준수**: ITU 참조 C, bcg729, FFmpeg G.729 등 일체의 외부 구현물 미참조. ITU-T G.729 §3.2.4 / §3.2.6 + LSP→LP 표준 이론 (Schur–Cohn step-down) 만 사용. ITU 벡터(.BIT/.PST) 블랙-박스 I/O.

---

## 1. 누적 커밋

| SHA | 메시지 | 비고 |
|------|--------|------|
| (없음) | — | F-fix 단일 랜딩 커밋은 escape hatch 1 발동으로 보류. Production 수정/F-prep-1 t.Errorf 승격/sample 40 가드 추가 모두 working tree에서 롤백됨. |

회귀 게이트: 롤백 후 `go test -race ./...` ALL PASS, `go vet ./...` silent. 본 세션 진입 시점과 동일 상태.

---

## 2. 분기 P 진단 — LSP 정렬은 정상, 결함은 `lspToLP` Q28 누산기

### 2.1 LSP 정렬 검사 (사용자 지시 first check)

ALGTHM frame 0 sf0 입력 `L0=1, L1=105, L2=17, L3=0`로 `lsp.Decoder.Decode` 내부 단계를 추적한 결과:

```
residual after combine (Q13):   [-459 467 3418 6122 8937 11082 13677 17808 20112 22812]
residual after rearrange (Q13): (변경 없음)
lsf after applyPredictor (Q13): [1094 2322 4846 7645 10313 12553 15128 18294 20594 23115]
lsf after enforceLSFStability:  (변경 없음)  →  strictly increasing ✓
lsp Q15 (cos(omega)):           [32468 31454 27196 19498 10040 1244 -8941 -20163 -26531 -31105]  →  strictly decreasing ✓
lspSF1 (interpolated):          [31954 29510 24327 16555 7351 -1710 -11277 -20811 -27049 -31273]  →  strictly decreasing ✓
lspSF2:                         (= lsp)  →  strictly decreasing ✓
```

**판정**: §3.2.4 LSP 안정/정렬 규칙은 모두 충족. 분기점은 §3.2.4가 아니라 §3.2.6 (LSP→LP 변환)에 있다.

### 2.2 `lspToLP` Q28 누산기 결함 확인 (§3.2.6 line-by-line walk)

ITU-T G.729 §3.2.6은 F1, F2 다항식을 Chebyshev 점화식

```
F(j) = F(j) − 2·q·F(j−1) + F(j−2)        (exact arithmetic)
```

으로 재귀 구성하도록 규정한다. 본 식은 **수학적 등식**이며 saturating 산술이 아니다.

현 production `internal/lsp/lsp_lp.go::lspToLP` (롤백 전 상태)는 `f1`, `f2`를 `[11]fixed.Word32` (Q28)로 보관하고 `polyStep` 안에서 `fixed.LSub` / `fixed.LAdd`로 누산 — 즉 **Word32 saturating** 누산. ALGTHM frame 0 sf1 (interpolated LSP) 입력으로 단계별 추적:

| step | q1 (real) | F1 (real, 단계 종료 후) — 다른 누산 |
|:---:|:---:|---|
| 0 | +0.97516 | `[+1.0000, -1.9503, +1.0000, …]` (float / fixed 동일) |
| 1 | +0.74240 | `[+1.0000, -3.4351, +4.8958, -3.4351, +1.0000, …]` (동일) |
| 2 | +0.22433 | float: `[…, -3.8838, +7.4371, -9.0669, +7.4371, -3.8838, …]`<br>**fixed: `[…, -3.8838, +7.4371, -8.0000, +7.4371, -3.8838, …]`**  ← **F1[3] = MIN32 / 2^28 = -8.000으로 Word32 underflow 포화** |
| 3 | -0.34415 | 발산 누적: `[…, -3.1955, +5.7639, -6.7649, +8.0000, -6.7649, +5.7639, -3.1955, …]` (F1[4] = MAX32 포화) |
| 4 | -0.82547 | 최종 F1: `[+1.000, -1.545, +1.488, **-1.960**, **+5.764**, **-5.530**, **+5.764**, **-1.960**, +1.488, -1.545, +1.000]` (vs float 기대 `[…, -1.511, +1.468, -1.410, +1.468, -1.511, …]` — middle 4 계수가 비대칭, 포화로 인해 회복 불가) |

**최종 a[] Q12** = `[4096 -2197 -375 -924 7735 294 665 7844 -1010 145 -33]` — F-prep-1이 보고한 값과 동일, |k_7|=1.897.

**스펙 인용 (§3.2.6, F polynomial recurrence, exact arithmetic)**:
> "The polynomials F1(z) and F2(z) are computed by the following recursive relation:
> F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2)"

본 점화식에 saturation을 부과하라는 규정은 없다. F1, F2의 중간 단계 계수는 |F| ≤ ~9.07까지 도달할 수 있고, 이는 Q28 Word32 표현 한계 |F| ≤ ~7.999를 초과 → **production이 §3.2.6 exact-arithmetic 가정을 위반**.

### 2.3 후보 수정 (검증 후 롤백)

**위치**: `internal/lsp/lsp_lp.go:16-71` (`lspToLP` + `polyStep`)

**수정 내용**: `f1`, `f2` 백킹 스토어를 `[11]fixed.Word32` → `[11]int64`. `polyStep`도 `int64` exact-arithmetic (`f - prod + fPrev2`, saturation 제거; `prod = (q*fPrev1) >> 14`는 동일). 최종 Q28 → Q12 Word16 단계만 saturation 유지.

**검증 결과 (post-fix, working tree)**:
- `TestALGTHMFrame0SF0_AzStability`: **PASS**. 모든 m에서 |k_m| < 1:
  ```
  m=10: k=-0.008057    m=9 : k=+0.031081    m=8 : k=-0.006054
  m=7 : k=-0.009197    m=6 : k=+0.068325    m=5 : k=+0.020135
  m=4 : k=-0.017413    m=3 : k=-0.011058    m=2 : k=-0.096825
  m=1 : k=-0.595293
  ```
  Schur–Cohn step-down 모두 통과 → A(z) minimum-phase 확정. **§3.2.6 준수.**
- `TestLSPToLPLeadingCoefficient`, `TestLSPToLPAllZeroLSPProducesSymmetric`: PASS.
- 그러나 ↓

### 2.4 Phase 1i sample-0 잠금 회귀 (Escape hatch 1 발동)

`TestDecode_Frame0Sample0_MatchesALGTHM` 결과 (post-fix):
```
frame 0 sample 0: got=4 want=2 (Δ=2)   ← FAIL (Phase 1i 잠금 got=2 want=2 회귀)
```

`TestDecode_Frame0SF0AllSamples_MatchesALGTHM` (Stage F sample 40 가드, 추가 후) 결과 발췌:
```
sample 0..5 : Δ ∈ {+2, +3, +3, +3, +3}
sample 6..21: Δ ∈ {+1, +3, +3, +1, -1, -1, -1, -1, …, -1}
sample 22..28: Δ = -2 (일정)
…
```

발산 폭은 Δ ≤ 3로 *극적으로 축소* (롤백 전: |s|=32767 포화 + 부호 진동, 14 dB 이상). 그러나 Phase 1i 잠금 (got=2 want=2)이 깨졌다.

**해석**:
1. F-prep 가설("§3.2.6 위반이 14 dB 발산의 주원인")은 **확증**. A(z) minimum-phase가 회복되면서 |Δ|가 ~32767 → ≤ 3로 줄었다.
2. 그러나 **하류에 추가로 보정 결함이 존재** — Phase 1i 잠금 시점(`736beba` synth §3.10 ½)에 sample 0 = 2가 만들어졌던 경로는, 깨진 §3.2.6의 결함이 만든 잘못된 a[] 값과 결합한 결과였다. §3.2.6을 spec 준수로 고치면 그 결합이 깨지고 잠재되어 있던 하류 (synth/postfilter/scale) 결함이 노출된다.
3. 롤백 전의 Δ=+2 ~ +3 패턴은 sample-by-sample 등비/등차가 아닌, "거의-DC + 작은 변동" 형태. synth `pastSynth` 초기 상태 또는 §3.10 두-패스 스케일링(F-prep-2가 노출한 ÷2+×2 vs ÷4+×4 spec 위반)이 하류 결함 후보.

본 세션 사용자 지시:
> "ALGTHM frame 0 sample 0 regresses (locked at got=2 want=2, Phase 1i `736beba`) — **immediate rollback**, no force-fit"

→ working tree 전체를 `git checkout --` 으로 복원. 미커밋. 회귀 게이트 다시 ALL PASS.

---

## 3. F-prep-1 |k_m| 비교 (수정 전 vs 수정 후 — *후보 수정 측정값*, 미커밋)

| step m | 수정 전 \|k_m\| | 수정 후 \|k_m\| | 판정 |
|:---:|---:|---:|:---:|
| 10 | 0.008057 | 0.008057 | < 1 |
| 9  | 0.031081 | 0.031081 | < 1 |
| 8  | 0.230895 | 0.006054 | < 1 |
| 7  | **1.897114** ✗ | 0.009197 | < 1 |
| 6  | (도달 불가) | 0.068325 | < 1 |
| 5  | (도달 불가) | 0.020135 | < 1 |
| 4  | (도달 불가) | 0.017413 | < 1 |
| 3  | (도달 불가) | 0.011058 | < 1 |
| 2  | (도달 불가) | 0.096825 | < 1 |
| 1  | (도달 불가) | 0.595293 | < 1 |

수정 후 모든 |k_m| < 1 → §3.2.6 minimum-phase 만족 (수정 자체는 spec-correct).

---

## 4. 발견된 결함 위치 — 의도와 다른가?

본 세션 진입 직전 가설: 분기 P = `lspToLP::polyStep` Q-format chain 결함. **결과: 가설 적중 (위치는 정확히 일치).**

- 정확한 결함 위치: `internal/lsp/lsp_lp.go::polyStep` + `lspToLP` 내 `f1, f2 [11]fixed.Word32` 보관. **§3.2.6 exact-arithmetic 점화식에 Word32 saturation을 강제 → 중간 단계 F polynomial 계수가 Q28 표현 범위를 초과하면 비대칭 포화로 잘못된 a[] 산출.**
- 다만 **이 fix 단독으로는 sf0 비트-정확이 불가능**하다는 추가 사실 노출 — 하류에 또 다른 결함이 보정으로 작용하고 있었음.

---

## 5. Task 4 V1 (frame 0 80-sample) 도달 못함

Task 3 단계에서 escape hatch 1 발동 → Task 4 미진입.
sf0 sample 40 sample-by-sample 가드도 미커밋 상태.

---

## 6. Escape Hatch 평가

| 해치 | 발동 여부 | 비고 |
|------|----------|------|
| 1 ALGTHM f0 s0 회귀 (got=2 want=2 lock) | **발동** | 후보 fix 적용 시 got=4 want=2 → 즉시 롤백 |
| 2 Phase 1i 다른 fix 회귀 | 미발동 (롤백으로 보존) | — |
| 3 Stage D 17 + D-bis 3 어서션 회귀 | 미발동 (롤백으로 보존) | — |
| 4 Task 4 sf1 FAIL | **도달 못함** | Task 3에서 정지 |

---

## 7. 권고 후속 조치 (사용자 결정 필요)

분기 P fix는 §3.2.6 spec 준수 측면에서 **필수적이고 정확**하지만, 단독 커밋이 불가능. 다음 중 하나의 경로:

**경로 A — Stage F-bis 신설 ("P + 하류 보정 동시 수정")**:
1. `internal/lsp/lsp_lp.go` int64 누산 fix를 candidate branch로 보관.
2. F-prep-3 추가: 후보 fix 적용 후 sample-by-sample Δ 패턴 (+2/+3/-1/-2 분포) 분석으로 하류 결함 좁히기.
3. 하류 결함 후보:
   - F-prep-2가 이미 노출한 §3.10 ÷2+×2 (`internal/synth/filter.go:31-52`) — spec은 ÷4+×4. P fix 후 발산 폭이 ≤3으로 작아져서 §3.10 두-패스가 trigger 안 되더라도, §3.10 자체 수정과 함께 묶으면 sample 0 = 2가 회복될 가능성.
   - postfilter (Phase 1i `f24add7` AGC seed) 또는 hpFilter 초기화의 입력 의존성.
4. 두 fix를 동일 커밋에 묶어 sample-0 잠금이 새 위치에서 (got=2 want=2 다시) 회복되는지 확인.

**경로 B — 분기 P 단독 커밋 + Phase 1i 잠금 갱신 (사용자 명시 승인 필요)**:
- "Phase 1i 잠금 = 우연히 일치하던 값이며 spec 준수가 더 우선" 으로 판단 시, sample-0 잠금을 일시적으로 비활성화하고 P fix만 커밋. ALGTHM 전체 비트-정확은 후속 단계 작업.
- **현 세션은 escape hatch 절대 준수 지시로 인해 본 경로 자가 선택 불가.** 사용자가 명시적으로 잠금 갱신을 승인해야 진행 가능.

**경로 C — F-prep-1 Errorf 승격만 단독 커밋 + 분기 P 미수정**:
- |k_m|≥1 노출 어서션을 `t.Errorf`로 승격하면 production은 그대로 둔 채 회귀 게이트가 빨개진다 (CI red). 의도적 red-bar는 작업 진행 시 명시적으로 깨짐을 알리는 신호.
- 단점: 회귀 게이트가 모든 후속 작업을 차단.

본 부분 보고서는 결정에 필요한 데이터만 제공. F-fix 진입 여부와 경로 선택은 사용자 권한.

---

## 8. 작업 트리 상태

```
$ git status
On branch main
nothing to commit, working tree clean

$ go test -race ./...
ok  github.com/hunydev/g729/internal/decoder    (cached)
ok  github.com/hunydev/g729/internal/fcb        (cached)
ok  github.com/hunydev/g729/internal/fixed      (cached)
ok  github.com/hunydev/g729/internal/gain       (cached)
ok  github.com/hunydev/g729/internal/lsp        (cached)
ok  github.com/hunydev/g729/internal/pcm        (cached)
ok  github.com/hunydev/g729/internal/pitch      (cached)
ok  github.com/hunydev/g729/internal/postfilter (cached)
ok  github.com/hunydev/g729/internal/synth      (cached)
ok  github.com/hunydev/g729/internal/tables     (cached)
```

세션 진입 직전과 동일한 상태로 복원됨.
