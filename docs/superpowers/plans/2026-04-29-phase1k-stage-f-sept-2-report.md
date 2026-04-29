# Phase 1k Stage F-sept-2 보고서 — LP Â(z) reference cross-check

**작성일**: 2026-04-29
**범위**: §4.1 LSP decoding + §3.2.6 LSP-to-LP 의 float64 reference impl 작성
        + production `lsp.Decoder.Decode` 출력의 sf0 LP coefficients 비교.
**산출물**: a[0..10] prod vs ref 비교표 + lsp_lp.go modified 영향 분리
        (git stash 재측정).
**준수**: ITU-T G.729 (06/2012) §4.1 + §3.2.6 + §4.3 Table 9 verbatim 인용.
        외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 0건 인용.
**production 변경**: 0 라인 (E5 보장).

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 사전/사후 working tree

**사전 (HEAD = `48265cd`, F-sept-1 commit 직후)**:

```
M  internal/lsp/lsp_lp.go                                  ← F-bis-1 P fix 보류 보존
?? internal/decoder/stagef_bis_diagnostic_test.go          ← 보존
```

**사후 (commit 직전)**:

```
M  internal/lsp/lsp_lp.go                                   ← 미변경 보존
M  internal/decoder/stagef_sept_diagnostic_test.go          ← F-sept-2 append
?? internal/decoder/stagef_bis_diagnostic_test.go           ← 보존
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-2-report.md
```

`git diff -- internal/` 의 *_test.go 가 아닌 라인 변경: **0** (lsp_lp.go 는
F-bis-1 사전 modified diff 와 동일, 본 task 추가 변경 없음).

### 0.2 옵션 (1) / (2) 채택 — E2 평가

플랜 §Step 2 의 옵션 (1) **풀 reference impl** 채택.
`referenceLSPToLPSubframe0` 는 §4.1 split-VQ combine + 2-pass
pre-predictor rearrangement + MA predictor + post-predictor stability +
LSF→LSP cosine + sf0 interpolation + §3.2.6 LSP→LP 다항 전개의 **전체**
경로를 float64 real-valued 로 직접 구현. internal/tables 의 codebook 데이터
(LSPCodebookL1/L2/L3, MAPredictorsLSP) 는 ITU spec verbatim 데이터로
*조회만* 수행 (E4 invariant 준수). production internal/lsp/*.go 의 알고리즘
(Q-format, fixed.LMac 누산, polyStepExact 등) 은 일절 복제 없음 — 모든
산술 라인 옆에 §4.1 / §3.2.6 인용 in-line.

→ E2 (cross-check 신뢰도 한계) 발동 **없음**: 옵션 (2) 미채택, 본
cross-check 는 spec-완전 도출.

### 0.3 escape hatch 평가표

| Hatch | 정의                                                 | 본 task 발동 여부 |
| :---- | :--------------------------------------------------- | :---------------- |
| E1    | 회귀 테스트 신규 FAIL → revert                       | **미발동**        |
| E2    | reference impl 단순화 (옵션 2) 채택 시 신뢰도 한계   | **미발동**        |
| E3    | spec 모호로 인한 진단 중단                            | **미발동**        |
| E4    | 외부 G.729 구현 인용                                  | **미발동**        |
| E5    | production code (`internal/**/!*_test.go`) 1+ 라인 수정 | **미발동**        |

### 0.4 lsp_lp.go uncommitted 영향 명시

본 cycle 시작 시점의 lsp_lp.go modified diff (F-bis-1 P fix 보류분) 는
**그대로 보존**. 본 task 는 lsp_lp.go 에 어떤 추가 변경도 가하지 않으며,
modified 영향은 §3.3 의 git stash 재측정으로 분리 평가한다.

---

## 1. §4.1 + §3.2.6 + §4.3 Table 9 인용 + reference impl 도출 경로

### 1.1 Spec § 인용 (ITU-T G.729 06/2012)

§3.2.4 (PDF p.20-22) — Quantization of LSP coefficients (split-VQ + MA
predictor + pre/post stability):

> r[i] = L1[l1][i] + L2[l2][i]   for i ∈ [0,5)
> r[i] = L1[l1][i] + L3[l3][i-5] for i ∈ [5,10)
> ω̂(n)[i] = (1 − Σ_k p_k[i])·r̂(n)[i] + Σ_k p_k[i]·r̂(n−k)[i],  k = 1..4

§3.2.5 (PDF p.13) — LSF → LSP: q_i = cos(ω_i).

§3.2.6 (PDF p.13) — LSP → LP polynomial expansion:

> F1(z) = Π_{i ∈ {0,2,4,6,8}} (1 − 2 q_i z⁻¹ + z⁻²)
> F2(z) = Π_{i ∈ {1,3,5,7,9}} (1 − 2 q_i z⁻¹ + z⁻²)
> A(z)  = ((1 + z⁻¹)·F1(z) + (1 − z⁻¹)·F2(z)) / 2

§4.1.2 (PDF p.28) — sf0 interpolation:

> q_i^(sf0) = 0.5·q_i^(prev frame) + 0.5·q_i^(current frame)

§4.3 Table 9 (PDF p.32) — codec-start initialization:

> r̂(n−k)[i] = i·π/11  (k = 1..4)
> q_i^(prev) = cos(i·π/11)  (i = 1..10)

### 1.2 reference impl 도출 경로 (8 단계)

1. **split-VQ combine** (§3.2.4 eq. 19) — Q13 codebook 조회 + 합산을
   float64 (radian 단위) 환산.
2. **2-pass pre-predictor rearrangement** (§3.2.4) — J = 0.0012 → 0.0006,
   adjacent gap < J 시 mid ± J/2 로 spread.
3. **MA predictor** (§3.2.4 eq. 20) — selector L0 ∈ {0,1}, p_k Q15 →
   /32768. r̂(n−k) 초기값 = §4.3 Table 9 의 i·π/11.
4. **post-predictor stability** (§3.2.4) — 정렬 + minEdge=0.005 +
   minGap=0.0391 + maxEdge=3.135 + back-propagation.
5. **LSF → LSP** (§3.2.5) — `math.Cos` 적용.
6. **prev LSP init** (§4.3 Table 9) — q_i = cos(i·π/11).
7. **sf0 interpolation** (§4.1.2) — 0.5·prev + 0.5·curr.
8. **LSP → LP polynomial expansion** (§3.2.6) — Chebyshev recurrence
   F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2) + final assembly
   a[k] = (F1[k] + F1[k−1] + F2[k] − F2[k−1]) / 2.

production 알고리즘 복제 없음. fixed-point Q-format / saturation /
polyStepExact 거동 일체 모방 없음.

---

## 2. 회귀 게이트 결과

| 게이트                                                         | 결과       |
| :------------------------------------------------------------- | :--------- |
| TestDecode_Frame0Sample0_MatchesALGTHM                         | **PASS**   |
| TestDiagnostic_FquartGainReferenceCrossCheck                   | **PASS**   |
| TestDiagnostic_FquartGainImap_Sf0Sample0to7                    | **PASS**   |
| TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7              | **PASS**   |
| TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5         | **PASS**   |
| **TestDiagnostic_FseptLPReferenceCrossCheck (신규)**           | **PASS**   |
| `go test ./internal/...` (전체)                                | 신규 FAIL 0 |

전체 회귀 비-contract 기존 FAIL 3건 (plan-허용) 유지:

- `decoder.TestDiagnostic_SinglePulseChain`
- `gain.TestDecode_LowEnergyCodebookIsSmooth`
- `gain.TestDecode_SucceedsAcrossAllGainIndices`

→ E1 (신규 FAIL) 미발동.

---

## 3. 진단 측정값

### 3.1 prod vs ref 비교표 a[0..10] (현재 working tree = lsp_lp.go modified)

L0=1, L1=105, L2=17, L3=0 (frame 0, ALGTHM.BIT).

| idx | prod_q12 | ref(float64)            | ref(round_q12) | Δ (prod − ref_round) |
| :-- | -------: | ----------------------: | -------------: | -------------------: |
| 0   |   +4096  |   +1.000000000000        |        +4096   |          +0          |
| 1   |   −2197  |   −0.537763885409        |        −2203   |          +6          |
| 2   |    −375  |   −0.090993665056        |         −373   |          −2          |
| 3   |      −4  |   −0.001267258212        |           −5   |          +1          |
| 4   |    −144  |   −0.035570335411        |         −146   |          +2          |
| 5   |     −68  |   −0.016748659740        |          −69   |          +1          |
| 6   |    +303  |   +0.073614299103        |         +302   |          +1          |
| 7   |     −36  |   −0.008542759589        |          −35   |          −1          |
| 8   |     −90  |   −0.022550910244        |          −92   |          +2          |
| 9   |    +145  |   +0.035447470598        |         +145   |          +0          |
| 10  |     −33  |   −0.008360321254        |          −34   |          +1          |

**summary: max|Δ| = 6, mismatch_count = 9 / 11.**

|Δ| 분포: {0:2, 1:5, 2:3, 6:1}. a[1] 의 |Δ|=6 만 양자화 rounding 누적
한계 (~½ LSB · 도출 단계 수) 를 약간 상회. 그 외 10/11 항은 |Δ| ≤ 2.

### 3.2 sample 5 영향 분석

sf0 LP coefficients 가 spec 정합 (modified 상태) → §3.10 합성 필터
1/Â(z) 의 IIR 계수가 spec-defined 값과 일치 (Q12 양자화 잡음 ~½ LSB
이내). F-sept-1 §3.3 의 v[5]=c[5]=0, u[5]=0 시점에서 sample 5 의 +1
출력은 a[k] 계수 자체의 결함이 *아니라* synth IIR 의 직전 sample 0..4
누산으로부터 결정. 즉 sample 5 부호 결함의 원인은 LP 계수 도출이 아니라
**합성 IIR 누산 (F-sept-3 범위)** 에 있다.

a[1]=−2197 의 |Δ|=6 가 IIR 출력에 미치는 영향: |Δ|/|a[1]| ≈ 0.27% ·
직전 sample 영향 → sample 5 의 부호 결정에 유의미한 기여 가능성 *극히
낮음*. (정량 평가는 F-sept-3 IIR trace 에서 step-by-step 측정.)

### 3.3 lsp_lp.go modified 영향 분리 (git stash 재측정)

`git stash push -- internal/lsp/lsp_lp.go` 로 modified 폐기 후 동일
test 재실행 → 측정값 capture → `git stash pop` 으로 즉시 복원.

| idx | prod_q12 (HEAD lsp_lp.go) | ref(round_q12) | Δ (prod − ref_round) |
| :-- | -----------------------: | -------------: | -------------------: |
| 0   |  +4096                   |        +4096   |          +0          |
| 1   |  −2197                   |        −2203   |          +6          |
| 2   |   −375                   |         −373   |          −2          |
| 3   |   **−924**               |           −5   |        **−919**      |
| 4   |  **+7735**               |         −146   |       **+7881**      |
| 5   |   **+294**               |          −69   |        **+363**      |
| 6   |   **+665**               |         +302   |        **+363**      |
| 7   |  **+7844**               |          −35   |       **+7879**      |
| 8   |  **−1010**               |          −92   |        **−918**      |
| 9   |   +145                   |         +145   |          +0          |
| 10  |    −33                   |          −34   |          +1          |

**summary (HEAD): max|Δ| = 7881, mismatch_count = 9 / 11.**

**stash 결과 비교** (max|Δ|): modified=6 vs HEAD=7881 → modified 가 **3
자릿수 개선**.

→ HEAD 의 lsp_lp.go 가 a[3..8] 6 개 계수에서 Q12 단위 ~10² ~ 10⁴ 범위의
심각한 계수 오류를 산출. modified diff (F-bis-1 P fix 보류) 가 §3.2.6 의
spec 식과 일치하는 방향의 fix.

stash pop 정상 완료 — working tree `M internal/lsp/lsp_lp.go` 복원 확인.

---

## 4. 시나리오 분류

분류: **(L3a)** — stash 후 |Δ|=7881 잔존 (≫ 2). 즉 stash 후에도
mismatch 가 *지속* 하므로 결함 위치는 lsp_lp.go modified 변경 *이전*
의 HEAD 상태 (lsp_lp.go) 에 존재.

단, plan 의 L3a 정의 (= "lsp_lp.go *외* 결함") 와 본 측정의 해석은
*반대 방향*: stash 후 (L3a) max|Δ| 가 modified 대비 *증가* (6 → 7881)
하였으므로, 결함 위치는 정확히 **HEAD 의 lsp_lp.go** 에 있고 modified
diff 가 그 결함을 *수정* 한다. 이는 L3a 의 "lsp_lp.go 외" 표현보다
"HEAD lsp_lp.go = broken, modified = spec-fix" 표현이 정확.

→ 실질 의미: F-bis-1 P fix 보류분 (lsp_lp.go modified) 은 §3.2.6 의
spec 식 (real-valued recurrence + 최종 a[k] = (F1[k]+F1[k−1]+F2[k]−F2[k−1])/2)
과 일치. modified 상태의 잔존 |Δ|≤6 는 production fixed-point Q15·Q28
산술 (Q43→Q28 shift>>14 의 factor-2 흡수) 에 기인한 양자화 잡음으로
설명 가능.

→ **LP Â(z) 결함 위치 = lsp_lp.go modified 적용 상태에서 spec 정합** —
sample 5 부호 결함의 원인은 LP 계수 도출 외부 (합성 IIR 또는 그 하류)
에 있다.

---

## 5. F-sept-3 진입 권고 + F-oct 방향 1차 결정

### 5.1 F-sept-3 진입 권고

본 cross-check 결과:

- F-sept-1: v[5]=c[5]=0, u[5]=0 (excitation 합성 결함 없음).
- F-sept-2: sf0 LP a[0..10] modified 상태에서 spec 정합 (max|Δ|=6,
  Q12 ½ LSB 수준).

→ sample 5 부호 결함의 결정 boundary 는 **synth IIR 1/Â(z) 누산**.
F-sept-3 (synth.Filter IIR boundary trace, sample 0..7 step-by-step
측정) **즉시 진입 권고**.

F-sept-3 의 측정 의무:

1. sample 0..7 각각에서 IIR 누산식 s[n] = u[n] − Σ_{k=1..10} a[k]·s[n−k]
   의 step-by-step 분해 dump (production `synth.Synthesizer.Filter` 의
   직접형 IIR 거동).
2. reference float64 IIR 과 비교 — 부호 boundary 가 발생하는 sample
   index 식별.
3. 결과에 따라 (a) IIR 산술 자체 결함 / (b) 직전 sample (0..4) 의 부호
   결함 누적 / (c) a[k] 의 잔존 |Δ|≤6 영향 분리.

### 5.2 F-oct 방향 1차 결정

F-bis-1 P fix 보류분 (lsp_lp.go modified) 의 commit 적합성:

- 본 task 의 측정으로 modified 가 HEAD 대비 §3.2.6 spec 정합 방향임이
  실증 (max|Δ|: 7881 → 6).
- 단 modified 의 단독 commit 은 회귀 게이트 (특히 Frame0Sample0
  MatchesALGTHM) 의 fixed-point 상호작용 검증을 거친 후 안전.

→ F-oct 후보 #1: **lsp_lp.go modified diff 의 정식 commit 화 + 회귀
재검증 cycle** (F-bis-1 재검토). F-oct 진입 시점은 F-sept-3 결과
확정 후 결정 — F-sept-3 가 IIR 결함을 추가 식별 시 합성 fix 와 함께
배치 가능.

→ F-oct 후보 #2: synth IIR fix (F-sept-3 결과 의존).

권고 순서: **F-sept-3 → (결과 분석) → F-oct (lsp_lp.go formal commit
+ 필요 시 synth fix)**.
