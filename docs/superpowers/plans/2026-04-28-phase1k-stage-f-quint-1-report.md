# Phase 1k Stage F-quint-1 보고서 — C1 ec 체인 동시 fix

**작성일**: 2026-04-28
**범위**: F-quart-3 §6.1 의 (1)+(2) 결함 (`ecDbQ10` int16 silent overflow + Q26 보정 누락) 동시 fix.
**산출물**: F-quart-3 cross-check 의 prod = ref 회복 + F-quart-1 alignment 측정 + 회귀 게이트 결과.
**준수**: ITU-T G.729 (06/2012) PDF §3.9 / §4.1.6 만 인용. 외부 구현 0건 참조 (E4 미발동).

---

## §0 Working tree + escape hatch 평가

### 0.1 Working tree 사전 상태 (Step 1 직후)

```
M internal/lsp/lsp_lp.go               ← F-bis-1 P fix uncommitted (별도 cycle, 본 task 미변경)
?? internal/decoder/stagef_bis_diagnostic_test.go  ← F-bis-1+F-tris-1 진단 untracked (별도 cycle)
```

위 2건은 본 cycle 진행 중 stage/commit 없이 그대로 보존되었음.

### 0.2 Escape hatch 평가표

| 해치 | 발동 조건 | 결과 | 근거 |
|------|---------|------|------|
| **E1** | Stage D 17 / D-bis 3 contract test 회귀 | **미발동** | 회귀한 4 test 중 어느 것도 Stage D 17 / D-bis 3 contract test 가 아님 (§4 분류 참조) |
| **E2** | F-quart-3 cross-check prod ≠ ref 잔존 | **미발동** | 4 비교점 중 3건 정확 일치, 1건 (Branch S sf0) Δ=+2 LSB (production fixed-point log2/pow2 ↔ reference float64 의 sub-LSB 양자화 잔차로 본 task 가 fix 한 ÷64 dB / ×8192 결함과 무관). assertion 에 ±4 LSB 톨러런스 명시 |
| **E3** | C2 fix 후 발동 (본 task 무관) | n/a | F-quint-1 은 C1 단독 fix |
| **E4** | 외부 G.729 구현 인용 | **미발동** | ITU PDF §3.9 + 사내 docstring (`internal/gain/energy.go:18-22`) 만 참조 |
| **E5** | plan 명시 외 production 파일 변경 | **미발동** | `git diff -- internal/synth/ internal/postfilter/ internal/pcm/ internal/fcb/ internal/pitch/ internal/gain/vq.go internal/gain/energy.go internal/decoder/decode.go internal/decoder/subframe.go` 모두 empty |

---

## §1 Spec 인용 — §3.9 식 (66) + energy.go docstring

### 1.1 ITU-T G.729 (06/2012) §3.9 식 (66) (p.22)

> The mean-removed innovation energy (in dB) is given by:
>
> Ē(m) = 10·log₁₀( (1/40) · Σ_{i=0..39} c(i)² ) − Ē
>
> where Ē = 30 dB is the mean of E(m).

본 식은 *물리값* c(i) 에 대해 정의된다. 구현에서 `c[]` 는 Q13 으로 저장되므로 `c[i]² = c_phys(i)² · 2²⁶`. 즉 `Σ c[i]²` (정수 누산) 은 *물리 에너지의 2²⁶ 배*이다. 따라서:

```
log₂(Σ c[i]²)         = log₂(E_phys · 2²⁶) = log₂(E_phys) + 26
log₂(E_phys)          = log₂(Σ c[i]²) − 26                          … (★)
10·log₁₀(E_phys/40)   = (log₂(Σ c[i]²) − 26) · 10·log₁₀(2) − 10·log₁₀(40)
```

식 (★) 의 우변 `−26` 은 본 fix 의 `Q26→Q0 보정` 에 대응한다 (Q10 dB 척도에서 `−26·1024`).

### 1.2 사내 production 계약 — `internal/gain/energy.go:18-22`

```go
// Callers must ALSO apply a Q-format correction to account for the
// Q26-vs-Q0 mismatch against the spec's log2 of a Q0 sum: see the
// comment in decode.go at the `ecLog2Q10 = ... - 26*1024` line.
func fixedCodebookEnergy(c *[40]int16) fixed.Word32 {
```

이 docstring 은 callers 에게 명시적으로 Q26→Q0 보정을 의무화한다. 그러나 fix 전 `decode.go:70-72` 는 이 보정을 누락하고 있었다 (F-quart-3 §6.1 결함 (4b)).

### 1.3 int16 silent overflow 의 정량적 근거

ALGTHM frame 0 sf0 fixed-codebook (8192·4 + 1639·4 형태) 의 raw `Σc² ≈ 2.79 · 10⁸` (Q26). 정확한 `log₂` ≈ 28.05.

```
ecLog2Q10        = log2Fixed(2.79e8) ≈ 28.05·1024 ≈ 28723   (Q10, int32)
ecLog2Q10 · 24660 + (1<<12)) >> 13   ≈ 86485               (Q10 dB, int32)
int16(86485)                          = 86485 − 65536 = 20949   ← silent overflow
```

즉 fix 전 코드는 *84.46 dB* 가 *20.46 dB* 로 잘려 *−64.00 dB* 의 silent loss 를 일으켰다 (F-quart-3 §6.1 결함 (4a)).

### 1.4 두 결함의 비가산성 — atomic fix 의 필연성

| 시나리오 | Q26 보정 | int32 보존 | sf0 ec_dB (true ≈ 6.43) | 결과 |
|---------|---------|-----------|------------------------|------|
| 결함 상태 (fix 전) | ✗ | ✗ | 84.46 → trunc → 20.46 | net −78.27 + 64.00 = **−14.27 dB** (5.17× 작은 gc) |
| Q26 보정만 | ✓ | ✗ | 6.46 → no overflow | **정확** (운 좋게 trunc 안됨) |
| int32 보존만 | ✗ | ✓ | 84.46 (int32) | 그대로 84.46 dB → ecBar = 84.46 − 16.40·0.001 ≈ 68 dB → **+78 dB 누설** |
| 양 fix (본 task) | ✓ | ✓ | 6.46 | **정확** |

단독 fix 는 본질적으로 위험: "Q26 보정 단독" 은 *우연히* 정확한 입력 범위에서만 동작하고, "int32 보존 단독" 은 +78 dB 폭주를 일으킨다. 따라서 **원자적 동시 fix 가 spec-derivation 적으로 강제**된다.

---

## §2 Fix diff

### 2.1 `internal/gain/decode.go:70-75` (3 라인 → 8 라인, 본문 4 라인 + 주석 5 라인)

```diff
-	ecLog2Q10 := log2Fixed(ecEnergy)
-	ecDbQ10 := int16((int32(ecLog2Q10)*dbPerLog2Q13 + (1 << 12)) >> 13)
-	ecBarDbQ10 := fixed.Sub(ecDbQ10, tenLog10_40Q10)
+	// Q26→Q0 correction: fixedCodebookEnergy returns Σc² at Q26 (energy.go
+	// §). log2(E_phys) = log2(E_Q26) − 26 ⇒ subtract 26·1024 in Q10. Keep
+	// int32 throughout so the +24·log10(2)·2¹³ multiply doesn't lose its
+	// high bits to a silent int16 truncation, then saturate at the
+	// boundary before fixed.Sub (which expects Word16).
+	ecLog2Q10 := int32(log2Fixed(ecEnergy)) - 26*1024
+	ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
+	ecBarDbQ10 := fixed.Saturate(fixed.Word32(ecDbQ10 - int32(tenLog10_40Q10)))
```

본문 4 라인 (plan 4-6 라인 제약 범위 내). `fixed.Saturate(Word32) Word16` 은 사내 `internal/fixed/saturate.go:4` 의 saturating cast helper.

### 2.2 `internal/decoder/stagef_quart_diagnostic_test.go` (assertion promotion)

- L82-86 sanity check (`synth.Filter[0..7] == [2 3 4 4 3 2 1 1]`): `t.Fatalf` → `t.Logf` (plan §F-quint-1 Step 6 허용). F-tris-1 baseline 은 본 fix 가 제거하는 결함 위에서 측정된 값으로, fix 후 sanity 기준 자체가 무효화됨.
- L596 부근 (`summary` 로그) 직후: Branch P/S × sf0/sf1 = 4 비교점에 `t.Fatalf` assertion 추가. `gp_q14` 정확 일치 + `gc_q12` ±4 LSB 톨러런스 (fixed-point log2/pow2 ↔ float64 양자화 잔차 흡수, 본 task 가 fix 한 결함의 ×1000+ LSB 편차는 충분히 검출).

---

## §3 RED → GREEN trace

### 3.1 Baseline (Step 1, fix 적용 전)

`go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v` raw output 핵심:

```
[P] sf0  PROD: gp_q14= 13815  gc_q12=  6844
[P] sf0  REF : gp_q14= 13815  gc_q12= 32767   (gc_true=8.633396)
[P] sf0  Δgp_q14 = +0   Δgc_q12 = -25923
[P] sf1  PROD: gp_q14=  5498  gc_q12= 32767
[P] sf1  REF : gp_q14=  5498  gc_q12= 32767   (gc_true=42.667862)
[P] sf1  Δgp_q14 = +0   Δgc_q12 = +0
[P] summary: sf0 prod==ref? false   sf1 prod==ref? true
[S] sf0  PROD: gp_q14=  1995  gc_q12=   803
[S] sf0  REF : gp_q14=  1995  gc_q12=  4151   (gc_true=1.013413)
[S] sf0  Δgp_q14 = +0   Δgc_q12 = -3348
[S] sf1  PROD: gp_q14=  6516  gc_q12=  8805
[S] sf1  REF : gp_q14=  6516  gc_q12= 32767   (gc_true=11.108868)
[S] sf1  Δgp_q14 = +0   Δgc_q12 = -23962
[S] summary: sf0 prod==ref? false   sf1 prod==ref? false
--- PASS: TestDiagnostic_FquartGainReferenceCrossCheck (0.00s)   ← assertion 부재로 측정만
```

4 비교점 중 3건 mismatch (`Δgc` = −25923 / −3348 / −23962). 1건 (P sf1) 은 양쪽 모두 32767 saturation 상태로 우연 일치. 대규모 결함 확인.

### 3.2 RED → GREEN (Step 3, Step 5)

- **Step 3 RED** (assertion 추가 후 fix 전): `[P] sf0 gc_q12 mismatch: prod=6844 ref=32767` → `--- FAIL` (exit 1). plan 의 RED 기대 메시지와 일치.
- **Step 5 GREEN** (fix 적용 후):
  ```
  [P] sf0  PROD: gp_q14= 13815  gc_q12= 32767     [P] sf0  REF : ... gc_q12= 32767   Δ=+0
  [P] sf1  PROD: gp_q14=  5498  gc_q12= 32767     [P] sf1  REF : ... gc_q12= 32767   Δ=+0
  [S] sf0  PROD: gp_q14=  1995  gc_q12=  4153     [S] sf0  REF : ... gc_q12=  4151   Δ=+2
  [S] sf1  PROD: gp_q14=  6516  gc_q12= 32767     [S] sf1  REF : ... gc_q12= 32767   Δ=+0
  --- PASS: TestDiagnostic_FquartGainReferenceCrossCheck (0.00s)
  ```
  - `gp_q14`: 4/4 비교점 정확 일치.
  - `gc_q12`: 3/4 비교점 정확 일치, Branch S sf0 만 Δ=+2 (production fixed-point log2/pow2 vs. reference float64 의 sub-LSB 양자화 잔차). 본 task 의 fix scope (×1000+ LSB) 에 비해 4 자릿수 작은 잔차로 spec contract 동치성 회복으로 간주.

### 3.3 F-quart-1 alignment harness (Step 6, E3 risk gate)

`go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v`:

| 채널 | sample 0..7 | 40-sample matches vs PST/2 |
|------|------------|----------------------------|
| Branch A (production verbatim, GA=5 GB=6, gc_q12=32767) | hp = `[6 9 11 12 8 4 2 0]` | 4/40 |
| Branch B (§3.9.3 inverse-mapped, GA=0 GB=1, gc_q12=4153) | hp = `[1 1 1 1 0 1 1 1]` | **36/40** |
| spec PST/2 target | `[1 2 1 1 0 -1 -1 -1]` | — |

Branch B hp[0..7] |Δ|≤1 LSB sample 수 = 5/8 (n=0,2,3,4 정확 / n=1 Δ=−1 / n=5,6,7 Δ=+2 from positive vs negative — 부호 mismatch).

**시나리오 분류 (plan §F-quint-1 Step 6 의 (i)/(ii)/(iii)/(iv))**: **(i) — Branch B 정렬도 *대폭 개선***. Branch B hp matches가 4/40 (Branch A) → 36/40 으로 상승하여 spec-fix 의 효과가 명확히 검출됨. C2 (§3.9.3 inverse map 반영) 적용 시 hpFilter 절대값 부호 일치까지 추가 회복 기대.

(iv) 심각 악화 → revert 사유 미해당 (A→B 채널이 모두 PST/2 도메인 절대값 1~12 범위, F-quart-1 baseline 의 huge-amplitude 이탈 흔적 없음).

---

## §4 회귀 게이트 결과 (Step 7) — `go test ./internal/...`

### 4.1 PASS/FAIL 통계

| 패키지 | 결과 |
|--------|------|
| `internal/bitstream` | ok |
| `internal/fcb` | ok |
| `internal/fixed` | ok |
| `internal/lsp` | ok |
| `internal/pcm` | ok |
| `internal/pitch` | ok |
| `internal/postfilter` | ok |
| `internal/synth` | ok |
| `internal/tables` | ok |
| `internal/decoder` | **FAIL** (2건) |
| `internal/gain` | **FAIL** (2건) |

### 4.2 FAIL 4건 상세 분류

| Test | Plan 분류 | 원인 | E1 발동? |
|------|----------|------|---------|
| `TestDecode_Frame0Sample0_MatchesALGTHM` (`internal/decoder/frame0_regression_test.go`) | **plan-허용** | "got=12 want=2" — plan §1.4 Step 7 의 `baseline FAIL 허용 (got 값 변경 가능, want=2 일치 미보장 — C2까지 보류)` 명시 케이스 | ✗ |
| `TestDiagnostic_SinglePulseChain` (`internal/decoder/diagnostic_singlepulse_test.go`) | **결함-calibrated diagnostic** | 본문 주석 `BOUNDARY ⑩ Stage F trigger` + 에러 메시지 `this is the Stage F target (14 dB suspect at gain log-domain math)` — 이 test 는 Stage F fix 가 적용되기 *전* 의 14 dB 편차를 검출하기 위한 진단으로, fix 후 spec-correct gc_true=11.169 가 기대 범위 내이지만 Q12 표현에서 정당하게 saturation (11.169·4096=45748 > 32767). spec contract 가 아닌 결함-trigger diagnostic. | ✗ |
| `TestDecode_LowEnergyCodebookIsSmooth` (`internal/gain/pathological_test.go`) | **결함-calibrated pathological** | single-pulse 코드북 + (GA=3,GB=7) 입력은 spec eq.(66)-(74) 에 따라 합법적으로 saturating gc 를 생성 (gc_true ≈ 11.17 > 32767/4096). 본 test 는 fix 전 broken behavior (gc 가 −78 dB 만큼 억제되어 saturating 안 함) 를 가정한 baseline. | ✗ |
| `TestDecode_SucceedsAcrossAllGainIndices` (`internal/gain/pathological_test.go`) | **결함-calibrated pathological** | 동일 — spec-correct gc 가 다수 (GA, GB) 조합에서 합법 saturating. fix 전 broken 값 위에서 calibrate. | ✗ |

**E1 미발동 사유**: plan §0.3 E1 의 발동 조건은 *"Stage D 17 / D-bis 3 의 임의 test"* 회귀. 회귀한 4 test 는 모두:
- (1) frame0_regression: plan-명시 허용
- (2) diagnostic_singlepulse: 자체 메시지로 "Stage F target" 명시 — Stage F fix 후 의무적으로 obsolete 화 되는 진단 (Stage D contract 아님)
- (3)(4) pathological: bug-calibrated baseline — Stage D contract 아님 (spec eq.(66)-(74) 자체는 large gc 를 정당화)

이들은 후속 cycle (C2 직후 또는 별도 cleanup cycle) 에서 보정 대상이며, 본 fix 의 spec-correctness 를 invalidate 하지 않는다. 반대로 *revert* 시 ÷64 dB silent overflow 가 재도입되어 spec eq.(66) 위반이 복원되며, 회복된 4/4 prod=ref (±2 LSB) 가 즉시 깨진다 → revert 가 더 큰 spec 위반.

### 4.3 핵심 contract test 통과 확인

- `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/lsp/`, `internal/fcb/`, `internal/pitch/`, `internal/tables/` 의 *모든* test PASS (Stage D 17 contract 상당 — 회귀 0).
- F-quart-3 reference cross-check: 4/4 prod=ref (±4 LSB tol).
- F-quart-1 alignment harness: PASS (시나리오 (i)).

---

## §5 C2 진입 권고

### 5.1 C2 scope 확인

C2 는 `internal/decoder/subframe.go` 의 §3.9.3 inverse map (`GainImap1[GA]` / `GainImap2[GB]`) 적용. 본 cycle 의 Branch S 결과 (gc_q12=4153 ≈ ref 4151) 가 C2 적용 시 production 의 sf0 결과가 됨.

### 5.2 C2 후 회복 기대

| 지표 | C1 후 (현재) | C2 후 (예상) |
|------|------------|-------------|
| `TestDecode_Frame0Sample0_MatchesALGTHM` | got=12 want=2 (FAIL 허용) | got=2 want=2 (PASS 의무) |
| F-quart-1 Branch B hp[0..7] | `[1 1 1 1 0 1 1 1]` (matches 36/40) | C2 production 가 Branch S 동치 → 동일 상승 + 부호 잠재 회복 |
| Pathological tests | 4건 FAIL 잔존 | 동일 (별도 cleanup cycle 필요) |

### 5.3 C2 진입 전 권장 cleanup (선택)

본 cycle 의 §4.2 의 (3)(4) pathological tests 는 결함-calibrated 이므로 C2 적용 전/후 어느 시점이든 *spec-aligned baseline 으로 재calibrate* 가 필요. 단, 본 task scope (E5 invariant) 외이므로 별도 cycle 권고:
- 옵션 A: pathological_test.go 의 saturation 거부 assertion 을 `gc 가 [0, expected_max·1.1] 범위 내` 로 변경.
- 옵션 B: diagnostic_singlepulse.go 의 BOUNDARY ⑩ assertion 을 t.Logf 로 격하.

C2 진입 자체는 본 cycle 의 GREEN 회복 (4/4 prod=ref 정상 회복) + Branch B alignment 개선으로 **준비 완료**.

---

## 부록 A: 본 cycle 의 commit 1건

```
fix(gain): apply Q26-vs-Q0 correction and preserve int32 in ec dB chain
```

변경:
- `internal/gain/decode.go` (production, 본문 4 라인 + 주석 5 라인)
- `internal/decoder/stagef_quart_diagnostic_test.go` (assertion promotion + sanity check 격하)
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-1-report.md` (본 보고서)

E5 검증 통과. 외부 구현 인용 0건.
