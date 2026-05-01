# Phase 1k Stage F-oct-prelim-5-3 보고서 — silence negative output mechanism

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (3). PST want sample 5..7 = -1 음수 출력 chain
        메커니즘 후보 4 개 (M1 postfilter conditional / M2 hpFilter / M3 synth
        memory / M4 PST 결함 부재) 평가.
**산출물**: production Decoder.Decode raw 출력 + §4.3 zero-init dump +
            (M1/M2/M3/M4) 잔존/폐기 4-tuple 분류.
**준수**: ITU-T G.729 (06/2012) §3.10 + §A.4.2 + §4.2.2 + §4.3 인용. F-sext-1
        §3.1 + F-sept §4 + Task 5-2 보고서 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)

### Working tree pre (Step 1)

```
?? internal/decoder/stagef_bis_diagnostic_test.go
HEAD: 9f27f74 test(decoder): add Stage F-oct-prelim-5-2 hpFilter init state diagnostic
```

### Working tree post (Step 7 직전)

```
M  internal/decoder/stagef_octprelim5_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md
```

`internal/decoder/stagef_bis_diagnostic_test.go` (untracked, 사전 보유) **미변경** 보존.

### Escape hatch 평가표

| Hatch | 항목                        | 본 task 발동 여부 | 비고                                               |
|-------|-----------------------------|------------------|----------------------------------------------------|
| E1    | regression FAIL → revert    | 미발동            | Phase 0.2 13 게이트 + 본 신규 test PASS 유지       |
| E2    | spec § 인용 부재             | 미발동            | §3.10 / §A.4.2 / §4.2.2 / §4.3 + F-sext-1 / F-sept |
| E3    | 가설 2+ 잔존 → 추가 진단     | 미발동            | (M1) 단일 잔존 — §4 결합 분류 참조                  |
| E4    | 외부 G.729 구현 참조         | 미발동            | ITU-T spec PDF + 사내 보고서만 인용                  |
| E5    | production 변경              | 미발동            | 0 라인 — diagnostic test 1 함수 추가만             |

## 1. §3.10 + §A.4.2 + §4.2.2 + §4.3 인용

- (인용 1) §3.10 synthesis filter ŝ(n) = û(n) + Σ aᵢ·ŝ(n-i). zero-init + û(n)≥0
  + aᵢ 부호 임의 → ŝ 부호는 IIR 누산 가능. F-sept-3 §4 측정 sample 0..7 = [+×8].
- (인용 2) §A.4.2 Annex A postfilter chain (long-term Hp(z) + short-term Hst(z)
  + tilt Ht(z) + AGC). F-sext-1 §3.1: postfilter[5..7] = [+,+,+].
- (인용 3) §4.2.2 hpFilter — Task 5-2 인용 1-3 동상. F-sext-1 §3.1: hpFilter[5..7]
  = [+,+,+].
- (인용 4) §4.3 zero-init: "All filter and quantizer states are initialized to
  zero at the beginning of decoding."
- (인용 5) F-oct-prelim-3 §5: ALGTHM/PITCH/FIXED PST sample 5..7 = [-1,-1,-1]
  (3 vector 동상). 우리 chain 양수 vs spec ref 음수 — 모순.
- (인용 6) F-sept-2 §4: LP Â(z) bit-exact spec 정합. LP 결함 부재.
- (인용 7) F-sept-1 §4: u[5] = +1 (gp·v + gc·c). excitation 결함 부재.
- (인용 8) F-oct-prelim-5-2 §3.4 (commit 9f27f74): hpFilter `H-INIT-1` +
  `H-RESP-1` 확정. **M2 폐기 입력**. Q-FMT-1 도 동시 확정.

## 2. 회귀 게이트 baseline (Step 1 출력)

- `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5HpFilterInitState -v`
  → **PASS**.
- `go test ./internal/...` → Phase 0.2 13 게이트 PASS + 비-contract diagnostic 3건
  FAIL (plan-허용 baseline):
  - `TestDiagnostic_SinglePulseChain` (decoder)
  - `TestDecode_LowEnergyCodebookIsSmooth` (gain)
  - `TestDecode_SucceedsAcrossAllGainIndices` (gain)
- 본 cycle 5-1 + 5-2 commit 후 신규 FAIL 0건.

## 3. 진단 측정값

### 3.1 raw output (Step 3 — `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism`)

**(a) PST want frame 0 sf0 sample 0..15**

```
[2 4 3 3 1 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1]
```

**(b) production `Decoder.Decode` frame 0 sf0 sample 0..15**

```
got [0..15]  = [2 2 2 2 0 2 2 2 2 0 0 0 0 0 0 0]
want[0..15]  = [2 4 3 3 1 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1 -1]
```

| n  | got | want | diff | 부호 (got/want) |
|----|-----|------|------|----------------|
|  0 |  +2 |   +2 |   +0 |  +  /  +       |
|  1 |  +2 |   +4 |   -2 |  +  /  +       |
|  2 |  +2 |   +3 |   -1 |  +  /  +       |
|  3 |  +2 |   +3 |   -1 |  +  /  +       |
|  4 |  +0 |   +1 |   -1 |  0  /  +       |
|  5 |  +2 |   -1 |   +3 |  +  /  −       |
|  6 |  +2 |   -1 |   +3 |  +  /  −       |
|  7 |  +2 |   -1 |   +3 |  +  /  −       |
|  8 |  +2 |   -1 |   +3 |  +  /  −       |
|  9 |  +0 |   -1 |   +1 |  0  /  −       |
| 10 |  +0 |   -1 |   +1 |  0  /  −       |
| 11 |  +0 |   -1 |   +1 |  0  /  −       |
| 12 |  +0 |   -1 |   +1 |  0  /  −       |
| 13 |  +0 |   -1 |   +1 |  0  /  −       |
| 14 |  +0 |   -1 |   +1 |  0  /  −       |
| 15 |  +0 |   -1 |   +1 |  0  /  −       |

**부호 반전 boundary**: sample 4 → 5 (got 양수/0 유지, want 음수 유입).

### 3.2 PST want vs production Decode 의 sample 0..15 비교

- sample 0 = 정합 (+2).
- sample 1..4 = 동부호 (+ vs +) 또는 0 vs +, |diff| ≤ 2.
- sample 5..15 = **부호 반전 일관 (got + 또는 0, want = -1)**.
- 누적 diff sample 0..15 = `[0,-2,-1,-1,-1,+3,+3,+3,+3,+1,+1,+1,+1,+1,+1,+1]`.
- 우리 chain 의 sample 5..15 출력은 *2 또는 0* — 음수 생성 불능.

### 3.3 §4.3 zero-init dump

```
d.pastExc[0..7]   = [0 0 0 0 0 0 0 0]
d.pastExc all-zero? true
d.prevGpQ14       = 0
d.hpX             = [0 0]
d.hpY             = [0 0]
d.initialized     = false
```

`Decoder` zero value 의 모든 노출 field가 zero. `lsp.Decoder` / `gain.Decoder`
/ `synth.Synthesizer` / `postfilter.Postfilter` sub-state 의 zero-value 정합은
각 package contract test (D 17 게이트) 가 검증 — Phase 0.2 PASS 유지.

### 3.4 (M1/M2/M3/M4) 잔존/폐기 4-tuple 분류

| 가설 | 근거                                                      | 측정/인용                                                        | 분류        |
|------|-----------------------------------------------------------|------------------------------------------------------------------|-------------|
| M1   | §A.4.2 postfilter conditional 음수 감쇠 분기              | F-sext-1 §3.1 postfilter[5..7]=[+,+,+]; conditional 분기 cover 별도 검증 필요 (본 task 측정 외) | **잔존**    |
| M2   | §4.2.2 hpFilter 음수 감쇠                                 | Task 5-2 §3.4 H-INIT-1 + H-RESP-1                                | **폐기**    |
| M3   | §3.10 synthesis memory 비-0 init                          | §4.3 zero-init dump 모두 zero + D 17 contract PASS               | **폐기**    |
| M4   | PST 자체 결함 부재 가설 (G3 폐기)                          | M1 잔존이므로 자동 채택 불가                                       | **잔존-보류**|

**4-tuple = (M1 잔존, M2 폐기, M3 폐기, M4 잔존-보류)**.

엄밀히는 M4 도 *반증되지 않은* 가설로 잔존 형태이나, M1 의 §A.4.2 conditional
분기 (voicing factor / pitch gain 임계) 우리 구현 cover 측정이 본 task 범위
*외* 인 한, M1 단일을 우선 후보로 채택. F-oct-prelim-5-4 종합에서 M1 cover
측정 결과에 따라 (M1 폐기 → M4 단일 채택) 또는 (M1 production fix cycle) 분기.

## 4. 결합 분류 (4-tuple) 와 G3 함의

- (M1 잔존 / M2 폐기 / M3 폐기 / M4 보류) → **단일 잔존 후보 = M1**.
- 권고 cycle: **F-oct = postfilter conditional 분기 production fix cycle**
  preliminary, 단 5-4 에서 M1 cover 측정 후 확정.
- 가설 G3 (Annex A vs main spec 분기 거동): M2/M3 폐기로 hpFilter / synth memory
  분기는 부재 확정. 잔여 분기 위치는 §A.4.2 postfilter chain 내부 conditional
  분기 (long-term gain factor / tilt comp / AGC 임계) 로 좁혀짐.
- 2+ 잔존 (E3) 발동 부재 — M2/M3 polarity 명확 폐기.

## 5. F-oct-prelim-5-4 진입 권고

- **진입**: 즉시 F-oct-prelim-5-4 (meta task — 코드 변경 0, 보고서 1건).
- **5-4 의무 측정**: §A.4.2 의 conditional 분기 (Annex A pitch postfilter Hp(z)
  의 `g_l` long-term gain factor, tilt comp Ht(z) 의 `k1` 부호 분기, AGC 의
  scaling) 를 우리 `internal/postfilter` 구현이 cover 하는지 spec § 인용 + 코드
  symbol grep 으로 점검. cover 정합이면 M1 도 폐기 → M4 단일 채택 → F-oct =
  plan-end declared. cover 결손이면 M1 production fix cycle 권고.
- **5-4 산출물**: Task 5-1 / 5-2 / 5-3 의 결과 결합 + G3 분기 위치 단일 식별
  또는 G3 폐기 (= M4 채택) 의 F-oct 단일 권고.
- **잔여 보류 항목**: §A.4.2 conditional 분기 cover 측정 (5-4), 비-contract
  diagnostic 3건 FAIL 의 후속 cycle 처리 (Phase 1k Stage 외).
