# Phase 1k Stage F-sept-3 보고서 — synth.Filter IIR boundary trace

**작성일**: 2026-04-29
**범위**: ITU-T G.729 (06/2012) §3.10 합성 필터 1/Â(z) 의 sample 0..7
        IIR 누산 측정 + reference float64 IIR 비교 + §3.10 two-pass
        overflow recovery Pass 1/2 path 측정.
**산출물**: prod vs ref 비교표 sample 0..7 + Pass 1/2 발동 여부 +
            sample 5 부호 분기 분석.
**준수**: §3.10 + §4.3 Table 9 verbatim 인용. 외부 G.729 구현
          (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 0건 인용.
**production 변경**: 0 라인 (E5 invariant 보존).

---

## 0. Working tree 상태 + escape hatch 평가

본 cycle 시작 시 (HEAD = `d61497d`, F-sept-2 commit):

```
M  internal/lsp/lsp_lp.go                                ← F-bis-1 P fix 보존 (F-sept-2 §4 결론)
?? internal/decoder/stagef_bis_diagnostic_test.go        ← 보존
```

본 task 동안 `internal/lsp/lsp_lp.go` 및 `stagef_bis_diagnostic_test.go`
모두 미변경. 추가된 산출물:

```
M  internal/decoder/stagef_sept_diagnostic_test.go       ← TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 추가
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-3-report.md
```

production source (`internal/**/*.go`, non-test) 0 라인 변경.

| Escape hatch | 발동 | 비고 |
| --- | --- | --- |
| E1 (회귀 게이트 fail) | No | 6 게이트 PASS + 사전 허용된 비-contract diagnostic 3건만 FAIL 유지. |
| E2 (API 부재) | No | `fixed.ClearOverflow()` / `fixed.Overflow()` 모두 존재 — Pass 1/2 측정 수행. |
| E3 (data 부재) | No | ALGTHM.BIT / ALGTHM.PST 정상. |
| E4 (외부 구현 인용) | No | reference impl 모든 라인 §3.10 + §4.3 Table 9 verbatim. |
| E5 (production 변경) | No | non-test source 0 라인 변경. |

---

## 1. §3.10 식 인용 + reference impl 도출 경로

ITU-T G.729 (06/2012) §3.10 (PDF p.24) verbatim:

> The reconstructed speech is obtained by passing the LP excitation u(n)
> through the LP synthesis filter:
>   ŝ(n) = u(n) − Σᵢ₌₁¹⁰ aᵢ · ŝ(n−i),    n = 0, …, L_subframe−1

§4.3 Table 9 codec-start: pastSynth(n) = 0 (n = −1, …, −10).

reference float64 impl 도출 (`referenceSynthFilter`, 측정-only):

1. `a[i] (Q12 int16) → a_real[i] = a[i] / 4096.0`. a[0] = 4096 → 1.0.
2. `pastSynth[0..9] = 0` (§4.3 Table 9).
3. `for n in 0..39: ŝ(n) = u[n] − Σᵢ₌₁¹⁰ a_real[i] · ŝ_or_past[n−i]` (real arithmetic).
4. saturation / Q-format / two-pass recovery 모두 0.

production 측정 경로 (`synth.Synthesizer.Filter`, `internal/synth/filter.go`
self-citing): direct-form `LMult/LMsu/LShl(<<3)/Round` + `fixed.Overflow()`
guard + Pass 2 (¼-scale / re-run / ×2-saturate). Pass 1/2 path 는
production Filter 호출 *직전* `fixed.ClearOverflow()` / 직후
`fixed.Overflow()` 로 측정.

외부 구현 0 인용 (E4 invariant). reference impl 의 모든 산술은 §3.10
식 verbatim 도출.

---

## 2. 회귀 게이트 결과

```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM            -v   PASS
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck      -v   PASS
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7       -v   PASS
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v   PASS
go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v PASS
go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck         -v   PASS
go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7   -v   PASS  ← 본 task 신규
go test ./internal/...                                                                  FAIL (사전 허용 3건만)
```

`go test ./internal/...` 잔존 FAIL 항목 (모두 본 task 이전 cycle 부터
존재 — F-sept-2 commit 시점 동일):

- `internal/decoder` `TestDiagnostic_SinglePulseChain` (비-contract diagnostic).
- `internal/gain` `TestDecode_LowEnergyCodebookIsSmooth` (비-contract diagnostic).
- `internal/gain` `TestDecode_SucceedsAcrossAllGainIndices` (비-contract diagnostic).

신규 FAIL 0 → E1 미발동.

---

## 3. 진단 측정값

### 3.1 prod vs ref 비교표 sample 0..7

```
idx   u[n]   prod_q0   ref(float64)        ref(round_q0)   Δ(prod − ref_round)
[ 0]    +1     +1         1.000000           +1              +0
[ 1]    +1     +2         1.536377           +2              +0
[ 2]    +1     +2         1.915630           +2              +0
[ 3]    +1     +2         2.169136           +2              +0
[ 4]    +0     +1         1.375512           +1              +0
[ 5]    +0     +1         1.008869           +1              +0
[ 6]    +0     +1         0.688062           +1              +0
[ 7]    +0     +1         0.465966           +0              +1
summary: max|Δ| (sample 0..7) = 1
```

- sample 0..6: `prod = ref_round`, |Δ|=0.
- sample 7: `prod=+1`, `ref=0.466 → round +0`, |Δ|=1 (rounding boundary).
- 8 sample 모두 부호 일치 (+).

### 3.2 sample 5 IIR boundary 분석

| 항목 | 값 | 부호 |
| --- | --- | --- |
| `u[5]` | +0 | 0 |
| `prod synth.Filter[5]` | +1 | + |
| `ref synth.Filter[5]` (float64) | +1.008869 | + |
| `PST want sample 5` | −1 | − |
| `PST/2 spec-target` | −1 | − |

- `u[5] = 0` (F-sept-1 시나리오 A' 와 일관). production output `+1` 은
  순수 IIR 피드백 (`−Σ aᵢ · ŝ(n−i)`, i=1..10) 결과.
- reference float64 IIR 도 `+1.008869` (round → +1). 부호 + 와 절대값
  ≈1 모두 정합.
- `prod` = `ref_round` 이며 두 값 모두 PST want `−1` 과 부호 반대.

### 3.3 sample 0..7 raw output 발췌

`go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v`:

```
u[] sample 0..7 = [+1 +1 +1 +1 +0 +0 +0 +0]
synth.Filter (production) sample 0..7 = [+1 +2 +2 +2 +1 +1 +1 +1]
fixed.Overflow() after Filter = false
```

### 3.4 §3.10 two-pass overflow Pass 1/2 발동 측정

API 가용성:
- `fixed.ClearOverflow()` — `internal/fixed/overflow.go:18` (존재).
- `fixed.Overflow()` — `internal/fixed/overflow.go:24` (존재).

측정 절차: production `syn.Filter(&sfA, &u, &sProd)` 호출 *직전*
`fixed.ClearOverflow()`, 호출 *직후* `fixed.Overflow()` 조회.

결과: `fixed.Overflow() = false` (Pass 1 단독 종료, Pass 2 미발동).

해석: ALGTHM frame 0 sf0 의 `u[]` 진폭 (sample 0..7 ∈ {0, +1}) 은
direct-form 누산에서 saturation 을 유발하지 않음. 따라서 sample 0..7
의 production 출력은 §3.10 직접형 IIR 의 Pass 1 결과 그대로이며,
`internal/synth/filter.go:31..54` 의 ¼-scale 회복 경로는 본 cycle 의
sample 5 부호 결정과 무관함을 확정.

(주: `Filter` 내부에서 Pass 2 가 발동되면 외부에서 보는
`fixed.Overflow()` 는 두 번째 onePass 의 saturation 만 반영. 본 cycle
은 Pass 1 자체가 overflow 미발생이므로 외부 `false` 가 곧 "Pass 1
clean" 을 의미.)

---

## 4. 시나리오 분류 (S1 / S2a / S2b / S3)

분류 결정값:
- sample 0..7 max|Δ| = 1 (≤ 1 임계).
- sample 5: prod 부호 = ref 부호 (둘 다 +).
- Pass 2 미발동 (`fixed.Overflow() = false`).

→ **시나리오 S1**: production §3.10 IIR 산술이 spec real-valued IIR
   과 정합 (sample 0..7 |Δ|≤1, sample 5 부호 일치). 합성 필터 자체는
   sample 5 부호 반전의 *원인이 아님*.

배제된 시나리오:
- (S2a) Pass 2 영향 — Pass 2 미발동으로 배제.
- (S2b) Pass 1 단독 IIR 산술 결함 — prod = ref (round) 로 배제.
- (S3) Q-format sub-LSB boundary — sample 5 에서 prod 부호 = ref
       부호 이며, max|Δ|=1 은 sample 7 의 0.466 round 에 한정.
       sample 5 부호는 boundary 로부터 충분히 떨어져 있음
       (ref ≈ +1.009, → round +1). Q-format widening 권고 근거 없음.

---

## 5. F-sept-4 종합 진입 + F-oct 권고 방향 결정

### F-sept-3 결론

ALGTHM frame 0 sf0 sample 5 의 production 출력 `+1` (PST want `−1`,
부호 반전) 는 §3.10 합성 필터 IIR 산술의 결함이 *아님*. production
direct-form 누산은 Pass 1 단독으로 종료되며 (overflow 미발동), real-
valued IIR 과 비교 시 sample 0..7 max|Δ|=1, sample 5 절대값 1 / 부호
+ 모두 일치.

### F-sept-1/2/3 종합 (F-sept-4 요지 미리보기)

| Cycle | 측정 대상 | 결론 |
| --- | --- | --- |
| F-sept-1 | excitation u[5] (§4.1.6 eq. 75) | u[5]=0 (v[5]=c[5]=0). sample 5 출력은 IIR 피드백 단독 결정. |
| F-sept-2 | LP Â(z) sf0 (§3.2.6) | HEAD lsp_lp.go max\|Δ\|=7881 broken / modified max\|Δ\|=6 spec 정합. modified 보존이 §3.2.6 정합 fix. |
| F-sept-3 | synth.Filter 1/Â(z) (§3.10) | Pass 1 단독, prod = ref (round), sample 5 부호 일치. **IIR 결함 없음**. |

→ sample 5 의 부호 반전은 합성 필터 *후단* 또는 *PST want 자체* 의
   해석 boundary 에서 결정. 합성 필터 (synth + LP) 영역에서는 spec
   정합 거동.

### F-sept-4 / F-oct 권고

1. **F-sept-4 (종합 분석)**: F-sept-1/2/3 측정값을 종합하여 sample 5
   부호 반전 책임 stage 를 *합성 필터 후단* (postfilter / hpFilter /
   pcm) 또는 *PST 비교 spec 의 PST/2 경로* (F-sext-1 의 PST/2 spec-
   target = −1 vs PST want = −1, 즉 reference encoder 의 sample 5
   값이 본질적으로 sub-LSB rounding boundary 일 가능성) 로 좁힐 것.
2. **F-oct (production fix)**:
   - 합성 필터 (synth + LP) 영역 fix 권고 *없음* — 본 cycle 까지의
     모든 측정이 §3.10 + §3.2.6 spec 정합을 확인.
   - lsp_lp.go modified (F-bis-1 P fix) 는 §3.2.6 spec 정합 fix 로
     확정 → F-oct 에서 official commit 권고 (F-sept-2 §4 결론 유지).
   - sample 5 부호 boundary 는 F-sept-4 종합 후 stage 식별 결과에
     따라 fix 범위 결정 (현 시점 `synth.Filter` 변경 권고는 0).
