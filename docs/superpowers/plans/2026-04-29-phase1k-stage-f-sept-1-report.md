# Phase 1k Stage F-sept-1 보고서 — excitation u[5] 부호 분해 진단

**작성일**: 2026-04-29
**범위**: F-sext-1 §4 시나리오 (i) (`synth.Filter[5..7] = [+1,+1,+1]` vs PST want `[−1,−1,−1]`, 4 stage 모두 부호 유지) 후속. ALGTHM frame 0 sf0 sample 5 의 excitation `u[5] = ĝ_p · v[5] + ĝ_c · c[5]` 합성에서 두 항 (lPitch, lCode), sum (lSum), 최종 (u[5]) 의 부호 + 절대값 + saturation 거동 측정.
**산출물**: lPitch / lCode / lSum / u[5] 단계별 부호 분포 표 + v[5] / c[5] 부호 사전 trace.
**준수**: ITU-T G.729 (06/2012) §4.1.6 eq. (75) verbatim 인용. 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 0건 참조.
**production 변경**: 0 라인 (E5 invariant 충족).
**lsp_lp.go uncommitted 영향**: 본 task 는 sf0 LP coefficient 를 *호출* 만 하며 (`lsp.Decoder.Decode` 의 sf1 반환값) 측정 trace 의 한 항으로 dump 한다. lsp_lp.go modified 상태가 sf0 LP 의 *수치 정확도* 에 미치는 영향은 본 task 에서 분리 측정하지 않으며, F-sept-2 cross-check 에서 분리 책임 위임.

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 Working tree pre-check (Step 1, F-sept-1 진입 시점, post-`078b172`)

```
$ git status --porcelain
 M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go

$ git diff --stat -- internal/
 internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
 1 file changed, 54 insertions(+), 54 deletions(-)
```

Plan §Phase 0.1 의 pre-state 와 동일.

### 0.2 Working tree post-state (Step 7, F-sept-1 commit 직전)

```
 M internal/lsp/lsp_lp.go                                      ← 미변경 보존 (F-bis-1 P fix uncommitted)
?? internal/decoder/stagef_bis_diagnostic_test.go              ← 미변경 보존 (F-bis 진단 baseline)
?? internal/decoder/stagef_sept_diagnostic_test.go             ← 본 task 신규
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-1-report.md  ← 본 task 신규
```

`git diff --stat -- internal/` 는 pre 와 동일 (lsp_lp.go 만, 54 ± 54). production 변경 0.

### 0.3 Escape hatch 평가

| 해치 | 발동 조건 | 본 task 발동? | 근거 |
|------|---------|---------------|------|
| **E1** | commit 후 회귀 게이트 1+ FAIL | **No** | Step 5 의 4 게이트 모두 PASS, 비-contract diagnostic 3건 (SinglePulseChain, LowEnergyCodebookIsSmooth, SucceedsAcrossAllGainIndices) 사전 FAIL 유지 (plan-허용). |
| **E2** | reference impl 의 휴리스틱 fit | **N/A** | 본 task 는 reference impl 작성 task 가 아님 (production 함수 호출 + spec eq. (75) 산술 분해 *재현* 만). |
| **E3** | 후속 task 와의 상호 모순 | **N/A** | F-sept-2/3 미실시 — 본 task 단독 결과만. |
| **E4** | 외부 G.729 구현 인용/대조 흔적 | **No** | 인용원: ITU-T G.729 (06/2012) §4.1.6 eq. (75) + `internal/synth/excitation.go:7-26` self-citing docstring 단 2건. |
| **E5** | production 파일 1+ 라인 변경 | **No** | `git diff -- internal/` 의 production 변경 = 기존 lsp_lp.go uncommitted only (본 task 무관). 신규 파일은 `*_test.go` + `.md` 만. |

### 0.4 lsp_lp.go uncommitted 영향 명시

본 task 의 sf0 LP coefficient (Step 3 출력 `[4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]`) 는 *modified working tree 상태* 에 대한 측정. 본 task 의 시나리오 분류는 `u[5]` 부호 결정 boundary 가 excitation 합성 *상류 또는 하류* 인지만 판정하며, sf0 LP 의 정확도 자체는 검증 대상이 아니다 → lsp_lp.go modified 영향 분리 책임은 F-sept-2 로 위임.

---

## 1. §4.1.6 eq. (75) 인용 + production self-citing

ITU-T G.729 (06/2012) §4.1.6 eq. (75) (PDF p.30):

> u(n) = ĝ_p · v(n) + ĝ_c · c(n),    n = 0, …, L_subframe − 1

`internal/synth/excitation.go:7-26` production self-citing docstring (verbatim):

```go
// BuildExcitation composes the per-subframe excitation
//
//	u(n) = g_p · v(n) + g_c · c(n)
//
// per ITU-T G.729 §4.1.6 eq. (75), using ITU saturation arithmetic.
//
// Q-formats:
//	gpQ14  Q14 Word16 (adaptive codebook gain)
//	gcQ12  Q12 Word16 (fixed codebook gain, from internal/gain)
//	v      Q0  Word16 × 40 (adaptive codebook vector)
//	c      Q13 Word16 × 40 (fixed codebook vector)
//	u      Q0  Word16 × 40 (output excitation, saturated)
//
// Per sample:
//	lPitch = LMult(gpQ14, v[n])              // Q15 (= 2·gp·v)
//	lCode  = LShr(LMult(gcQ12, c[n]), 11)    // Q26 → Q15
//	lSum   = LAdd(lPitch, lCode)             // Q15
//	u[n]   = Round(LShl(lSum, 1))            // Q15 → Q16 → Q0
```

본 task 는 위 4-단계 산술을 *test 내부에서 재현* (production `BuildExcitation` 호출이 아닌 step-by-step capture) 하여 lPitch / lCode / lSum / u 의 부호 + 절대값 + Q15 saturation 거동을 측정.

---

## 2. 회귀 게이트 baseline (Step 1 출력)

```
$ go test ./internal/decoder/ -run 'TestDecode_Frame0Sample0_MatchesALGTHM|TestDiagnostic_FquartGainReferenceCrossCheck|TestDiagnostic_FquartGainImap_Sf0Sample0to7|TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7' -v
...
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM
--- PASS: TestDiagnostic_FquartGainImap_Sf0Sample0to7
--- PASS: TestDiagnostic_FquartGainReferenceCrossCheck
--- PASS: TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7
PASS
ok  	github.com/exedev/g729/internal/decoder	0.004s
```

4 게이트 모두 PASS — F-sept-1 진입 baseline 확정.

---

## 3. 진단 측정값

### 3.1 Step 3 raw output

```
$ go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v
=== RUN   TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5
    PST want sample 5 = -1 (PST/2 spec-target = -1)
    sf0 LP coefficients (Q12, a[0]=4096): [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]
    pitch sf0: tInt=20 tFrac=0 (P1=2)
    v[] sample 0..7 = [   +0    +0    +0    +0    +0    +0    +0    +0]
    c[] sample 0..7 = [+8192 +8192 +8192 +8192    +0    +0    +0    +0]
    gain sf0: gp_q14=1995 gc_q12=4153 (beta_q14=3277, GA1=5 GB1=6)
    ──────── excitation u[0..7] 분해 trace (§4.1.6 eq. 75) ────────
    [ n]   v       c        lPitch=LMult(gp,v)   lCode=LShr(LMult(gc,c),11)   lSum         u
    [ 0]    +0  +8192             +0               +33224                    +33224     +1
    [ 1]    +0  +8192             +0               +33224                    +33224     +1
    [ 2]    +0  +8192             +0               +33224                    +33224     +1
    [ 3]    +0  +8192             +0               +33224                    +33224     +1
    [ 4]    +0     +0             +0                   +0                        +0     +0
    [ 5]    +0     +0             +0                   +0                        +0     +0
    [ 6]    +0     +0             +0                   +0                        +0     +0
    [ 7]    +0     +0             +0                   +0                        +0     +0
    ──────── sample 5 부호 결정 분석 ────────
    v[5]                              = +0 (부호 0)
    c[5]                              = +0 (부호 0)
    lPitch = LMult(gp_q14, v[5])      = +0 (부호 0, |절대값| 0)
    lCode  = LShr(LMult(gc_q12,c[5]),11) = +0 (부호 0, |절대값| 0)
    lSum   = lPitch + lCode           = +0 (부호 0)
    u[5]   = Round(LShl(lSum, 1))     = +0 (부호 0)
    PST want sample 5                 = -1 (부호 −)
    PST/2  spec-target sample 5       = -1 (부호 −)
    Q15 saturation 발생? false  (|lPitch|=0, |lCode|=0, threshold=32767)
    ──────── F-sept-1 시나리오 분류 ────────
    u[5] 부호 = 0, PST want 부호 = −
    (시나리오 A') excitation u[5] = 0 (v[5]=0, c[5]=0). sample 5 출력은 전적으로
       IIR 누산 (직전 비-zero u[0..4] 의 1/Â(z) 피드백) 으로 결정됨.
       → 부호 결정 boundary = synth IIR 또는 LP Â(z). 합성 입력 결함 가능성 제외.
       결함 위치 후보 = LP Â(z) (F-sept-2) 또는 synth IIR 1/Â(z) (F-sept-3).
--- PASS: TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 (0.00s)
```

### 3.2 sample 5 분해 표

| 항목 | 값 | 부호 | 비고 |
|------|----|------|------|
| `gp_q14` (Q14) | 1995 | + | F-sext-1 §3.2 와 일치 (zero-init gain VQ). |
| `gc_q12` (Q12) | 4153 | + | F-sext-1 §3.2 와 일치. |
| `tInt / tFrac` | 20 / 0 | — | pitch sf0, P1=2. |
| `beta_q14` (β init) | 3277 | + | `fcb.ClampPitchGainForEnhancement(0)` zero-init 결과. |
| `v[5]` (Q0) | **0** | **0** | adaptive codebook 출력 — sample 5 contribution 없음. |
| `c[5]` (Q13) | **0** | **0** | fixed codebook 출력 — sample 5 pulse 없음 (pulse 위치 = 0,1,2,3 만). |
| `lPitch = LMult(gp_q14, v[5])` (Q15) | 0 | 0 | v[5]=0 → 자명 0. |
| `lCode = LShr(LMult(gc_q12, c[5]), 11)` (Q15) | 0 | 0 | c[5]=0 → 자명 0. |
| `lSum = lPitch + lCode` (Q15) | 0 | 0 | — |
| `u[5] = Round(LShl(lSum, 1))` (Q0) | **0** | **0** | excitation sample 5 = 0. |
| **PST want sample 5** | **−1** | **−** | spec-target. |
| **PST/2 sample 5** | **−1** | **−** | half scale (`>>1`). |
| Q15 saturation | **false** | — | 두 항 모두 0 → saturation 산술 무관. |

### 3.3 sample 0..7 보조 dump (excitation 전체 sf0 sample 5 주변 컨텍스트)

| n | v[n] | c[n] | lPitch (Q15) | lCode (Q15) | lSum (Q15) | u[n] (Q0) |
|---|------|------|--------------|-------------|------------|-----------|
| 0 |  0   | +8192 | 0           | +33224      | +33224     | **+1**    |
| 1 |  0   | +8192 | 0           | +33224      | +33224     | **+1**    |
| 2 |  0   | +8192 | 0           | +33224      | +33224     | **+1**    |
| 3 |  0   | +8192 | 0           | +33224      | +33224     | **+1**    |
| 4 |  0   |  0    | 0           |  0          |  0         |  0        |
| 5 |  0   |  0    | 0           |  0          |  0         |  0        |
| 6 |  0   |  0    | 0           |  0          |  0         |  0        |
| 7 |  0   |  0    | 0           |  0          |  0         |  0        |

**핵심 관찰**:
1. `v[]` 전체 = 0 (sample 0..7) — adaptive codebook 가 zero-init `pastExc` 에서 sf0 첫 호출 시 0 출력 (정상 거동).
2. `c[]` 의 비-zero pulse 4개가 sample **0..3** 에 모두 위치 (모두 +8192 = +1.0 Q13). sample 5..7 위치 pulse 없음.
3. excitation `u[]` 가 sample 0..3 = +1, sample 4..7 = 0 으로 *unit-impulse-train* 형태.
4. F-sext-1 §3.1 `synth.Filter sample 5..7 = [+1, +1, +1]` 의 +1 은 직접 입력 (`u[5..7] = 0`) 이 아닌 **IIR 피드백** (직전 sample 0..3 의 +1 입력에 대한 `1/Â(z)` 응답 꼬리) 에서 발생.
5. 두 항 모두 0 → Q15 saturation 발생 가능성 0 → 시나리오 (B4) 자동 배제.

---

## 4. 시나리오 분류

본 task 의 단일 분류: **시나리오 (A)** (변형 A' — 강 sub-form).

**근거 1 (B 계열 일괄 배제)**: `u[5] = 0` 이며 `v[5] = c[5] = 0`. 따라서:
- (B1) adaptive codebook 결함 가설 — `v[5] ≠ 0` 의 부호가 lPitch 부호를 결정해야 성립. v[5]=0 → 적용 불가.
- (B2) fixed codebook 결함 가설 — `c[5] ≠ 0` 의 부호가 lCode 부호를 결정해야 성립. c[5]=0 → 적용 불가.
- (B3) gain decode 잔여 결함 가설 — 두 항의 절대값 ratio 가 PST 와 모순해야 성립. 두 항 모두 0 → ratio 정의 불가 → 적용 불가.
- (B4) saturation 결함 가설 — `|lPitch|` 또는 `|lCode|` > 32767. 두 항 모두 0 → 적용 불가.

**근거 2 (A 계열 적합)**: F-sext-1 측정 (`synth.Filter[5] = +1`) + 본 task 측정 (`u[5] = 0`) 의 조합은:
- excitation sample 5 직접 contribution = 0.
- synth.Filter sample 5 출력 +1 = `Σ_{i=1..10} (−a_i) · ŝ(5−i)` 의 IIR 누산 결과 (§3.10 eq.).
- 따라서 sample 5 의 부호 결정 boundary 는 **excitation 합성 *외부*** — 즉 (i) LP coefficient `Â(z)` 의 부호/계수 정확도, 또는 (ii) synth IIR `1/Â(z)` 의 직접형 산술 거동.

**시나리오 (A) 단정 — 강압-적합 검증**:
- 측정 결과는 lPitch = lCode = 0 으로 *결정적*. 정성 해석 0, 측정값 그대로 분류.
- 시나리오 (A) 가 PST want 부호 (−) 와 직접 일치하지 않음 (`u[5] = 0 ≠ −`) 에도 불구하고 분류가 강제됨 — 절대값 magnitude 차이 (1 LSB) 는 IIR 피드백 잔여로 자명 설명 가능 (synth.Filter[5] = +1 ≠ −1 의 1 LSB 부호 반전이 IIR 또는 LP 결함의 기대 출력).

**모순 항목 (강압-적합 회피 의무)**: 측정값 모순 0건. 본 task 결과는 F-sept-2/3 의 결과와 결합 시 모순 발생 가능 (예: F-sept-2 가 LP `Â(z)` 정합 + F-sept-3 가 synth IIR 정합 단정 시 → E3 발동 후보). 본 task 단독으로는 모순 없음.

---

## 5. F-sept-2 / F-sept-3 진입 권고

**(R1) F-sept-2 (LP `Â(z)` reference cross-check) 진입 우선순위 = 높음.**
- 근거: 본 task 의 sf0 LP coeff dump `[4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]` 의 정확도가 미검증.
- `lsp_lp.go` uncommitted modified 상태가 a[1..10] 부호/크기에 미치는 영향이 sample 5 IIR 출력 부호를 좌우할 수 있음 (§3.10 식의 `−a_i · ŝ(n−i)` 항).
- F-sept-2 cross-check + git stash 분리 측정 (plan §Step 4 L3a/L3b) 로 lsp_lp.go modified 가 LP 정확도 손상 여부를 판정 의무.

**(R2) F-sept-3 (synth IIR boundary trace) 진입 우선순위 = 중간 (F-sept-2 결과에 의존).**
- F-sept-2 가 (L1) prod = ref 로 LP 정합 확인 시 → F-sept-3 가 sample 5 부호 반전의 단일 결함 후보 (synth IIR 직접형 산술 또는 two-pass overflow path 분기).
- F-sept-2 가 (L3) prod ≠ ref 로 LP 결함 식별 시 → F-sept-3 의 IIR 측정도 reference float64 IIR 비교를 *prod LP* 와 *ref LP* 두 입력 모두 수행해 결함 분리 의무.

**(R3) F-sept-4 종합 분석 입력**:
- F-sept-1 단정: excitation 합성 결함 **제외**.
- F-sept-1 + F-sept-2/3 결합 결과로 단일 결함 위치 확정 → F-oct production fix cycle 권고 방향 결정.
- 만약 F-sept-2 + F-sept-3 모두 결함 단정 시 → E3 발동 → 복수 fix 동시 적용 권고.

**(R4) lsp_lp.go uncommitted 분리 의무**: F-sept-2 §Step 4 의 git stash 측정 (modified vs unmodified) 결과는 본 task 의 단정 (시나리오 A) 의 *부호* 자체에는 영향 없음 (v[5]=c[5]=0 은 LSP 와 무관한 fcb/pitch 출력) — 그러나 sample 5 IIR 출력 부호 (+1 vs −1) 의 결정 메커니즘은 LP modified 상태에 직접 의존 가능. F-sept-2 분리 측정 결과를 본 보고서 §3.3 보조 해석에 *역참조* 권고.
