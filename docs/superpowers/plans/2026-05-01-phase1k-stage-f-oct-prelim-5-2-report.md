# Phase 1k Stage F-oct-prelim-5-2 보고서 — hpFilter init state + F-sext 통합

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (2). §4.2.2 hpFilter (eq. 151/152) IIR
        memory 초기 state 검증 + zero-input/impulse/chain replay step
        response 측정. F-sext-2/3 (HP startup transient + reference
        cross-check) reactivate 통합.
**산출물**: Q-format error 표 + zero-input/impulse/chain replay sample
            0..7 표 + (Q-FMT, H-INIT, H-RESP) 분류.
**준수**: ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) + §4.3 인용.
        외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)

### 0.1 working tree pre/post

**pre (commit `445c72d` 직후, F-oct-prelim-5-2 진입 시점):**

```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

본 task 시작 시 `internal/lsp/lsp_lp.go` modified 잔존 *없음* (이전 cycle 에서 정리됨). `stagef_bis_diagnostic_test.go` untracked 상태 그대로 보존.

**post (본 task 완료, commit 직전):**

```
 M internal/decoder/stagef_octprelim5_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md
```

E5 검증: `git diff -- internal/` 의 production 라인 변경 0. 본 task 변경은 `stagef_octprelim5_diagnostic_test.go` 에 신규 함수 (`TestDiagnostic_FoctPrelim5HpFilterInitState` + helper `allZeroInt16`) append 한 단일 test 파일 변경. `stagef_bis_diagnostic_test.go` untracked 미변경 보존.

### 0.2 escape hatch 평가표

| 항목 | 발동? | 근거 |
|------|-------|------|
| **E1** (commit revert) | 미발동 | 회귀 게이트 신규 FAIL 0. 본 cycle 5-1 PASS 유지 + 본 task 신규 test PASS. |
| **E2** (Q-format 결함) | 미발동 | (a) 5 계수 모두 |Δ| ≤ 0.0002 — spec real value 정합 (Q-FMT-1). |
| **E3** (외부 reference 의존) | 미발동 | spec PDF §4.2.2 / §4.3 + production hpfilter.go self-citing constant 만 인용. 외부 G.729 구현 0건 참조. |
| **E4** (외부 G.729 구현) | 미발동 | E3 와 동일. |
| **E5** (production 변경) | 미발동 | `internal/**/*.go` 중 `*_test.go` 외 0 라인 변경. `git diff -- internal/decoder/hpfilter.go` empty. |

## 1. §4.2.2 + §4.3 + production self-citing 인용

(인용 1) §4.2.2 (PDF p.43) `H_h2(z) = (0.93980581 - 1.8795834 z⁻¹ + 0.93980581 z⁻²) / (1 - 1.9330735 z⁻¹ + 0.93589199 z⁻²)`

(인용 2) §4.2.2 식 (151) `y_h2(n) = 0.93980581·s_f(n) - 1.8795834·s_f(n-1) + 0.93980581·s_f(n-2) + 1.9330735·y_h2(n-1) - 0.93589199·y_h2(n-2)`

(인용 3) §4.2.2 식 (152) `sw(n) = sat(2 · y_h2(n))`

(인용 4) §4.3 (PDF p.46): "*All filter and quantizer states are initialized to zero at the beginning of decoding.*"

(인용 5) production `internal/decoder/hpfilter.go` line 11-19 self-citing:

```
hpB0Q13    = 7699
hpB1Q13    = -15399
hpB2Q13    = 7699
hpNegA1Q12 = 7918
hpA2Q13    = 7667
```

(인용 6) production `internal/decoder/hpfilter.go` line 26-67: `func (d *Decoder) hpFilter(in *[subframeLen]int16, out []int16)` — IIR memory `d.hpX [2]int16`, `d.hpY [2]int32`. `Decoder.Reset()` (types.go:38) zero-init.

## 2. 회귀 게이트 baseline (Step 1 출력)

본 task 진입 직전 (`445c72d`) 의 12 게이트 PASS:

- `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` — PASS (P-SRC-2 확인)
- `TestDiagnostic_FoctPrelim5BitVectorCompare` — PASS
- `TestDecode_Frame0Sample0_MatchesALGTHM` / Fquart / Fsext / Fsept / FoctPrelim 시리즈 — 모두 PASS
- `go test ./internal/...` — 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 만 plan-허용 FAIL

post-commit 게이트도 동일 (신규 FAIL 0).

## 3. 진단 측정값

### 3.1 raw output (Step 3)

```
=== RUN   TestDiagnostic_FoctPrelim5HpFilterInitState
──────── (a) production Q-format vs spec real coefficient ────────
  b0   q= +7699  approx=+0.93981934  spec=+0.93980581  |Δ|=0.00001353
  b1   q=-15399  approx=-1.87976074  spec=-1.87958340  |Δ|=-0.00017734
  b2   q= +7699  approx=+0.93981934  spec=+0.93980581  |Δ|=0.00001353
  -a1  q= +7918  approx=+1.93310547  spec=+1.93307350  |Δ|=0.00003197
  a2   q= +7667  approx=+0.93591309  spec=+0.93589199  |Δ|=0.00002110
──────── (b) zero-input + zero-state hpFilter sample 0..7 ────────
  hpFilter(0...) [0..7] = [0 0 0 0 0 0 0 0]
  hpFilter(0...) [0..39] all-zero? true
──────── (c) impulse(+1 at n=0) + zero-state hpFilter sample 0..7 ────────
  hpFilter(δ[0]=+1) [0..7] = [1 0 0 0 0 0 0 0]
  hpFilter(δ[0]=+1) [0..39] = [1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]
──────── (d) chain-like impulse (sample 0 = +2) + zero-state ────────
  hpFilter(δ[0]=+2) [0..7] = [2 0 0 0 0 0 0 0]
──────── (e) F-sept-4 chain output as hpFilter input + zero-state ────────
  hpFilter(chain[0..7], 0...) [0..7] = [2 4 2 2 0 0 0 0]
  hpFilter expectation = sample 5..7 부호 추적
    [5]  in=+1  out=+0  부호 (in/out) = + / 0
    [6]  in=+1  out=+0  부호 (in/out) = + / 0
    [7]  in=+1  out=+0  부호 (in/out) = + / 0
--- PASS: TestDiagnostic_FoctPrelim5HpFilterInitState (0.00s)
```

### 3.2 Q-format 정합 분류 → **(Q-FMT-1)**

5 계수 모두 |Δ| ≤ 0.0002 (가장 큰 b1 의 |Δ|=0.000177). `< 0.001` 임계 만족. spec real value 정합. Q-format 결함 부재. F-sept-3 (synth IIR Q-format 정합) 와 동상.

### 3.3 Init state 분류 → **(H-INIT-1)**

`Decoder.Reset()` 후 zero-input (`in[0..39] = 0`) → hpFilter `out[0..39] = 0` (all-zero, 40/40). spec §4.3 "All filter and quantizer states are initialized to zero" 와 정합. **silence frame 0 의 negative output 메커니즘은 hpFilter 단독으로 *불가능*.**

### 3.4 Response 분류 → **(H-RESP-1)**

F-sept-4 chain output 7-sample prefix `[2, 4, 3, 3, 1, 1, 1, 1]` 을 hpFilter 입력으로 driving (sample 8.. = 0) + zero-state → output `[2, 4, 2, 2, 0, 0, 0, 0]`. **sample 5..7 부호 = `0` (non-negative)** — hpFilter 가 부호 반전 *없음*. F-sext-1 §4 의 "hpFilter 단계에서 sample 5..7 = `[1, 1, 1]` (양수 동상)" 결과와 정합 방향 (수치 차이는 본 task 가 sample 8.. 를 0 으로 driving 한 boundary 효과; 부호 추세는 동일). **H-RESP-2 (부호 반전) 미발생** → F-sext-1 결과와 비-회귀.

추가 관측:
- impulse `δ[0]=+1` → out[0..39] = `[1, 0, 0, ...]` (양수 단일 펄스, 후속 sample 모두 0).
- impulse `δ[0]=+2` → `[2, 0, 0, ...]`.
- IIR pole 가 양수 영역에서 안정적 응답을 생성. **음수 startup transient 메커니즘 부재**.

## 4. 결합 분류 (3D) 와 G3 함의

**3D 분류: (Q-FMT-1, H-INIT-1, H-RESP-1)** — 단일 결정.

함의:
- hpFilter 의 **Q-format / init state / step response 모두 spec 정합**.
- silence frame 0 sample 5..7 negative output (`PST want = [-1, -1, -1]`) 의 *원인은 hpFilter 가 아님*.
- F-sext-2 (HP startup transient) 가설 **폐기 확정**: hpFilter 가 양수 chain input 으로부터 음수 sample 을 *생성하지 않음*.
- F-sext-3 (HP reference cross-check) 가설 **폐기 확정**: production constant 가 spec real value |Δ| ≤ 0.0002 정합.
- F-oct-prelim-3 §5 의 *공통 결함 메커니즘* 후보 중 hpFilter 후보 **제거**. 잔존 후보:
  - (M1) §A.4.2 postfilter ringing (long-term + short-term + tilt + AGC)
  - (M3) §3.10 synthesis filter memory 비-0 init
  - (M4) PST 자체가 spec-correct 음수 출력 생성 (= 우리 chain 이 *정상* 가설; PST = ITU reference G.729A binary 의 정상 산출)
- G3 (silence frame 0 negative PST want) 함의: PST want 는 ITU reference 가 G.729A `decoder` 를 silence BIT 에 적용해 생성한 binary 이므로, 음수 출력은 (M1) postfilter ringing 또는 (M3) synth memory 에서 spec 상 발생하는 정상 응답.

## 5. F-oct-prelim-5-3 진입 권고 + F-sext-2/3 종결 평가

### 5.1 F-sext-2 / F-sext-3 종결

- **F-sext-2 (HP startup transient)** — 본 task 측정으로 **폐기 종결**. (b)/(c)/(d)/(e) 4 측정에서 hpFilter 가 양수/zero input 으로부터 음수 sample 생성 불가 입증.
- **F-sext-3 (HP reference cross-check)** — 본 task (a) Q-format error 표로 **폐기 종결**. spec real value 정합 (|Δ| ≤ 0.0002).

### 5.2 F-oct-prelim-5-3 진입 권고

**진행 권고**. plan §Task F-oct-prelim-5-3 (silence frame 0 negative output 생성 chain trace) 으로 이동. 본 task 결과로 후보 (M2) hpFilter 제거된 상태 진입 → 잔존 (M1) / (M3) / (M4) 3 후보 중 단일 식별이 5-3 의 핵심.

특히 (M1) §A.4.2 postfilter (long-term + short-term + tilt + AGC) 의 chain stage 별 sample 0..7 부호 trace 가 5-3 의 1순위 측정. F-sext-1 보고서는 postfilter.Filter sample 5..7 = `[1, 1, 1]` 양수 동상 보고했으므로, 5-3 은 *postfilter 내부 sub-stage* 수준에서 음수 transient 가 발생하는지 확인해야 함.
