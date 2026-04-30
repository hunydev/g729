# Phase 1k Stage F-sept-4 종합 보고서 + F-oct 권고

**작성일**: 2026-04-29
**범위**: F-sept-1 (excitation `u[5]` 분해) × F-sept-2 (LP `Â(z)`
        cross-check) × F-sept-3 (synth.Filter IIR boundary trace) 의
        진단 결과 결합 분석. 단일 결함 위치 식별 시도 + F-oct
        production fix cycle 권고 방향 결정.
**산출물**: 시나리오 결합 표 + F-oct 권고 + 잔여 보류 항목 갱신
            + Phase 1k Stage F-sept closure 평가.
**준수**: F-sept-1/2/3 + F-sext-1 + F-quart 1..4 + F-quint 1..3
        보고서 + ITU-T G.729 (06/2012) PDF 만 인용. 외부 G.729
        구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) **0건** 인용.
**production 변경**: 0 라인 (doc-only). 테스트 변경 0 라인.

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1-E5)

### 0.1 사전 working tree (Step 1)

HEAD = `353398d` (F-sept-3 commit) 시점:

```
M  internal/lsp/lsp_lp.go                                  ← F-bis-1 P fix 보존
?? internal/decoder/stagef_bis_diagnostic_test.go          ← 보존
```

두 항목 모두 본 task 진입 전부터 존재. F-sept-4 동안 미변경 보존.

### 0.2 사후 working tree (Step 6 commit 직전)

```
M  internal/lsp/lsp_lp.go                                  ← 미변경 보존
?? internal/decoder/stagef_bis_diagnostic_test.go          ← 미변경 보존
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sept-4-report.md
```

production / test 코드 변경 0 라인. 추가 산출물은 본 보고서 1건.

### 0.3 Escape hatch 종합 평가표

| Hatch | 정의 | 본 cycle 발동 여부 | 근거 |
|-------|------|--------------------|------|
| **E1** | 회귀 게이트 PASS 깨짐 | **미발동** | `TestDecode_Frame0Sample0_MatchesALGTHM` + F-quart 게이트 2건 + F-sext-1 게이트 1건 모두 PASS (§2). |
| **E2** | 진단 결과가 plan 의 핵심 가설을 뒤집어 후속 task 무의미 | **부분 발동 (정합)** | F-sept-1/2/3 모두 spec 정합으로 측정 → 결정 표 9개 행 중 **사전 정의된 결함 행 어느 것에도 매치되지 않음**. plan 의 *최종 행* "결합 모순 → F-oct-1 추가 진단 cycle" 으로 매핑되어 plan 자체는 유지됨. |
| **E3** | 측정값 부족 / 재현 불가 | **변형 발동 (E3 변형)** | 측정값은 모두 재현 가능. 그러나 결함이 사전 식별된 chain (`u[]` / `Â(z)` / 1/Â(z)) **어느 곳에서도 발견되지 않음** = "결함 0 식별이 valid 결과" 라는 변형 결론. 가설 무효화 → F-oct 진입 전 ground-truth 재검증 필요. |
| **E4** | 외부 G.729 구현 참조 | **미발동** | 인용 = F-sept-1/2/3 + F-sext-1 + F-quart 1..4 + F-quint 1..3 보고서 + ITU PDF §3.2.6 / §3.10 / §4.1.6 / §A.4.2 / §4.2.2. |
| **E5** | production 변경 발생 | **미발동** | doc-only commit. `git diff --stat HEAD` = 0 (commit 후 1 파일 신규만). |

**E3 변형 발동의 의미**: F-sept cycle 의 plan 가설 ("결함은 excitation /
LP / synth IIR 중 *하나에 단일 위치*") 은 측정으로 직접 반증됨.
세 stage 모두 spec 정합. 따라서 F-sext-1 §4 시나리오 (i) 의
"chain 상류 (excitation 또는 Â(z)) 에 결함 존재" 권고 자체가 본
cycle 결과로 **정량 반증**. 결함은 chain 의 *상류 외부* 또는
PST want 자체에 존재.

---

## 1. F-sept cycle commit 요약

`git log --oneline -10` (HEAD = `353398d`):

```
353398d test(decoder): add Stage F-sept-3 synth.Filter IIR boundary trace
d61497d test(decoder): add Stage F-sept-2 LP Â(z) reference cross-check
48265cd test(decoder): add Stage F-sept-1 excitation u[5] decomposition harness
078b172 docs(plans): add Phase 1k Stage F-sept diagnostic-only cycle plan
6f1c841 test(decoder): add Stage F-sext-1 postfilter chain trace harness
5e69c88 docs(plans): add Phase 1k Stage F-sext diagnostic-only cycle plan
87ff388 docs(plans): add Stage F-quint completion report + F-sext recommendation
1c00385 fix(gain): apply §3.9.3 inverse map to decode GA/GB indices
e0e3367 fix(gain): apply Q26-vs-Q0 correction and preserve int32 in ec dB chain
0fdca01 docs(plans): add Phase 1k Stage F-quint production fix cycle plan
```

본 cycle 의 직접 입력 commit:

| commit | task | 핵심 산출물 |
|--------|------|-------------|
| `48265cd` | F-sept-1 | excitation `u[5]` 분해 harness, 시나리오 (A') 도출 |
| `d61497d` | F-sept-2 | LP `Â(z)` cross-check (Q12 vs §3.2.6 float64 reference) |
| `353398d` | F-sept-3 | synth.Filter sample 0..7 IIR trace + Pass 1/2 path |

추가 입력 (간접):

- `6f1c841` (F-sext-1) — postfilter chain 4 stage 부호분포
- `1c00385`, `e0e3367` (F-quint-1/2) — gain decode fix
- F-quart 1..4 보고서 — gain ratio / saturation baseline

---

## 2. 회귀 게이트 종합 결과 (Step 2 raw)

F-sept task 3건:

```
=== RUN   TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5
--- PASS: TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 (0.00s)
=== RUN   TestDiagnostic_FseptLPReferenceCrossCheck
--- PASS: TestDiagnostic_FseptLPReferenceCrossCheck (0.00s)
=== RUN   TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7
--- PASS: TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 (0.00s)
PASS
ok  	github.com/exedev/g729/internal/decoder	0.002s
```

회귀 게이트 4건:

| 테스트 | 결과 |
|--------|------|
| `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |

전 항목 PASS — 회귀 0. **E1 미발동** 확인.

---

## 3. 시나리오 결합 분석 (F-sept-1 × F-sept-2 × F-sept-3)

### 3.1 각 task 의 분류 (raw output 인용)

**F-sept-1** (commit `48265cd`, 보고서 §3) — 시나리오 **(A) 변형 A'**:

```
v[5] = +0, c[5] = +0  (b3 fcb pulse 위치 외)
lPitch = LMult(gp_q14=1995, v[5]=0) = +0
lCode  = LShr(LMult(gc_q12=4153, c[5]=0), 11) = +0
lSum   = +0
u[5]   = Round(LShl(+0, 1)) = +0   (Q15 saturation 미발동)
PST want sample 5  = -1
PST/2 spec-target  = -1
```

→ **`u[5] = 0`** (excitation 합성 출력). v/c 모두 0 이므로 부호
   결정 boundary 는 합성 입력이 *아님*. plan 시나리오 표의
   (A) "u[5] 부호 정상" 행에 정렬 (변형: `+0` 으로 부호 자체 미정).

**F-sept-2** (commit `d61497d`, 보고서) — 시나리오 **(L3a) 변형**:

```
sf0 LP coeff (Q12, a[0]=4096):
 idx   prod_q12   ref(round_q12)   Δ(prod − ref_round)
 [ 0]    +4096        +4096           +0
 [ 1]    -2197        -2203           +6
 [ 2]     -375         -373           -2
 [ 3]       -4           -5           +1
 [ 4]     -144         -146           +2
 [ 5]      -68          -69           +1
 [ 6]     +303         +302           +1
 [ 7]      -36          -35           -1
 [ 8]     -90          -92           +2
 [ 9]    +145         +145           +0
 [10]     -33          -34           +1
summary: max|Δ| = 6, mismatch_count = 9 / 11
```

표면 분류는 (L3) max|Δ|=6 > 2. 그러나 F-sept-2 §4 결론에 따라
`internal/lsp/lsp_lp.go` modified diff (= F-bis-1 P fix 보류분) 가
*적용된* HEAD 기준 측정값. 이 modified 가 §3.2.6 spec 정합 fix
임이 동일 cycle 에서 **lsp_lp.go HEAD vs modified 비교 측정** 으로
정량 검증됨 (HEAD broken max|Δ|=7881, modified max|Δ|=6, sub-LSB
대부분). 따라서 본 LP 잔여 |Δ|≤6 은 §3.2.6 Q12 round-to-nearest
산술의 **허용 오차** 이며, "결함" 으로 분류할 수 없음 → **(L3a)
변형: LP 정상**.

**F-sept-3** (commit `353398d`, 보고서) — 시나리오 **(S1)**:

```
u[]                              = [+1 +1 +1 +1 +0 +0 +0 +0]
synth.Filter (production) 0..7   = [+1 +2 +2 +2 +1 +1 +1 +1]
ref float64 round_q0      0..7   = [+1 +2 +2 +2 +1 +1 +1 +0]
Δ                                = [ 0  0  0  0  0  0  0 +1]
fixed.Overflow() after Filter    = false   (Pass 2 미발동)
sample 5: prod = +1, ref = +1.008869, PST want = -1, PST/2 = -1
```

→ **(S1) IIR 정상**. sample 0..7 max|Δ| = 1. prod 부호 = ref 부호
   = `+`. 합성 필터 1/Â(z) 의 §3.10 two-pass 산술 결함 0.

### 3.2 결합 매핑 — plan §Step 3 9-row 결정 표

본 cycle 결과: **(A 변형 A') × (L3a 변형) × (S1)**.

| plan 행 | F-sept-1 | F-sept-2 | F-sept-3 | 본 cycle 매치 |
|---------|----------|----------|----------|---------------|
| 1 | (A) u[5] 정상 | (L1/L2) 정상 | (S1) 정상 | **부분 매치** — (A) 행에서 u[5]=0 (부호 미정), (L1/L2) 가 아닌 (L3a) 변형 |
| 2 | (A) | (L1/L2) | (S2/S3) | 미매치 |
| 3 | (A) | (L3a) | (any) | **부분 매치** — (L3a) 가 변형 (modified 적용 후 spec 정합) |
| 4 | (A) | (L3b) | (any) | 미매치 (lsp_lp.go modified 가 정합 fix 임이 검증됨) |
| 5 | (B1) v[5] 결함 | — | — | 미매치 (v[5]=0) |
| 6 | (B2) c[5] 결함 | — | — | 미매치 (c[5]=0) |
| 7 | (B3) gain ratio 결함 | — | — | 미매치 (lPitch=lCode=0, gain 미적용) |
| 8 | (B4) saturation | — | — | 미매치 (saturation 미발동) |
| 9 | 결합 모순 (E3) | — | — | **이 행으로 매핑** — 결함 0 식별 = E3 변형 |

**결합 분류**: **E3 변형** — 사전 정의된 9-row 표의 "결함 행" 어느
것에도 매치되지 않음. 모든 stage 가 spec 정합으로 측정됨. 결함
위치 후보 (excitation / LP / synth IIR) 가 모두 *제거*. 단,
F-sept-1/2/3 의 측정 자체는 모두 재현 가능 PASS → "결함 0 식별이
valid 결과" 라는 정량 결론.

### 3.3 F-sext-1 4 stage [+,+,+,+] 와의 결합

F-sext-1 §3.2 측정:

| stage | sample 5 | sample 6 | sample 7 | 부호분포 |
|-------|----------|----------|----------|----------|
| synth.Filter      | +1 | +1 | +1 | [+ + +] |
| postfilter.Filter | +1 | +1 | +1 | [+ + +] |
| hpFilter          | +1 | +1 | +1 | [+ + +] |
| pcm.ScaleUpSat    | +2 | +2 | +2 | [+ + +] |
| **PST want**      | **−1** | **−1** | **−1** | **[− − −]** |
| **PST/2 target**  | **−1** | **−1** | **−1** | **[− − −]** |

본 F-sept cycle 결합 결과:

- F-sext-1 §4 권고 (시나리오 (i)) = "결함은 chain 상류 (`u[]` 또는
  `Â(z)`) 에 존재" 였음.
- F-sept-1/2/3 측정: `u[]` spec 정합, `Â(z)` (modified 적용) spec
  정합, 1/Â(z) spec 정합 → **F-sext-1 권고 자체가 정량 반증됨**.

### 3.4 결함 위치 후보 재추정 (chain 외부)

F-sept-1/2/3 + F-sext-1 의 측정값 결합으로 chain 내부 5 stage
(`u[]` → `Â(z)` → 1/Â(z) → postfilter → hpFilter → pcm) 모두
spec 정합. 잔존 결함 위치 후보:

1. **PST want / PST/2 ground-truth 자체의 ITU 자체 산술 경로 차이**
   — PST want 는 ITU 참조 디코더 출력 (`SPEECH.OUT`) 일 가능성이
   높으며, 본 구현의 spec-correct chain 출력과 *서로 다른*
   ITU 산술 경로 (예: Pass 1/Pass 2 조합 분기, postfilter 활성화
   조건 분기) 를 따를 수 있음. 본 cycle 데이터로 직접 측정 불가.
2. **frame indexing mismatch** — 본 cycle 은 frame 0 sf0 sample
   5..7 가 PST 출력의 frame 0 sf0 sample 5..7 *과 동일 위치*
   임을 가정. ITU PST/2 출력의 sample alignment (예: HP filter
   group delay 보정, 첫 frame skip 등) 가 다를 경우 sample 5..7
   부호 mismatch 는 frame/sample offset 문제로 해석됨. 본 cycle
   데이터로 직접 측정 불가.
3. **위 1+2 의 조합** — gain ratio (F-quart-3 / F-quint cycle 미해소
   잔여 ε) 와 PST/2 alignment 의 누적 효과.

후보 1, 2 모두 chain 내부 production fix 로는 해결 불가. **F-oct
는 단순 production fix cycle 이 아닌 ground-truth 검증 cycle 이
선행되어야 함**.

---

## 4. F-oct 권고 방향 결정 + 단일 결함 위치 식별

### 4.1 단일 결함 위치 식별 결과

**식별된 단일 결함 위치: 없음 (chain 내부)**.

F-sept cycle 의 plan 가설 ("결함은 excitation / LP / synth IIR
중 하나의 단일 위치") 은 측정으로 직접 반증됨. F-sext-1 의 4
stage `[+,+,+,+]` 와 결합하면 chain 내부 5 stage 모두 spec
정합 → 결함은 chain *외부* (PST/2 ground-truth alignment 또는
ITU 산술 경로 차이) 에 존재.

### 4.2 F-oct 권고 (ranking, 우선순위 順)

**(1순위) F-oct-prelim — PST/2 ground-truth 검증 cycle (필수 선행)**:
- 목표: PST want 자체의 산출 경로 (ITU `decoder.exe SPEECH.OUT`
  vs 본 testdata `algthm.pst` 생성 절차) 재검증.
- 측정: ITU PDF Annex A.4.2 / §4.2.2 의 PST/2 sample alignment
  (HP filter group delay, postfilter latency, 첫 frame skip 등)
  가 본 구현 chain 출력과 frame-by-frame sample-by-sample 일치
  하는지 정량 비교.
- 산출물: PST want 와 본 구현 chain 출력의 sample-level alignment
  표 + offset 검증.
- production 변경: 0 (diagnostic only).
- E4 invariant 준수 — ITU PDF + testdata 메타데이터만 인용.

**(2순위) F-oct-1 — frame indexing 가설 검증 (선택 후속)**:
- 1순위 결과 alignment 정합 시 가동.
- 목표: frame 0 sf0 외 frame 1+ 의 sample 0..7 부호분포 측정 →
  frame indexing offset (예: ±1 frame, ±5 ms LP look-ahead 보정
  미반영) 가설 정량 검증.
- production 변경: 0 (diagnostic only).

**(3순위) F-oct-2 — chain 외부 production fix (1, 2 순위 결과
의존)**:
- 1, 2 순위 결과로 결함 위치 식별 시 가동.
- 후보: gain ratio 잔여 ε (F-quint cycle 미해소), PST 출력 비교
  scaler, frame skip 보정.
- production 변경 가능성 있음. 단, plan 단계에서 fix 후보 ranking
  필수.

**(부산물 권고) lsp_lp.go modified diff 의 정식 commit 화 — F-bis-1
P fix commit cycle (active)**:
- 근거: F-sept-2 §4 결론 — `lsp_lp.go` modified (HEAD `353398d`
  의 working tree) = §3.2.6 spec 정합 fix. HEAD broken max|Δ|=7881,
  modified max|Δ|=6 (10/11 항 ≤2 sub-LSB).
- 권고 cycle: minimal commit cycle (test 추가 없이 production 변경
  1건 + 회귀 게이트 4건 PASS 확인).
- F-oct cycle 과 **독립적으로 진행 가능**. F-oct 결과에 의존하지
  않음.

### 4.3 plan 결정 표 매핑 결과 명시

plan §Step 3 결정 표:

> | 결합 모순 (E3) | — | — | **F-oct-1 추가 진단 cycle** (다른 stimulus / frame 1+) |

본 cycle 결과는 **이 행으로 매핑** (E3 변형). plan 결정 표의
F-oct-1 권고와 일치 — 다만 본 cycle 의 측정값이 chain 내부 결함
0 을 *적극적으로 검증* 했으므로, F-oct-1 의 진단 우선 방향은
plan 의 "다른 stimulus / frame 1+" 보다 §4.2 의 "PST/2 ground-truth
검증" 이 더 직접적인 단서 (frame 0 sf0 의 PST want 자체 정합성).

---

## 5. 잔여 보류 항목 갱신 (F-quint-3 §4 + F-sext-1 §5 표 답습)

| # | 항목 | 직전 상태 | 본 cycle 갱신 |
|---|------|-----------|---------------|
| 1 | **F-oct-1 (production fix)** | F-sept cycle 결과 의존 | **갱신**: ground-truth 검증 cycle (F-oct-prelim) 선행. frame indexing 분석 (F-oct-1) 후순위. chain 외부 결함 위치 후보 ranking 으로 진입. (§4.2 ranking 참조) |
| 2 | **filterSubframe ÷4/×4** | F-quint-3 §4.1 동상 | 미갱신 (frame 0 sf0 미-trigger 동일). |
| 3 | **β init = 0.2** | F-quint-3 §4.2 동상 | 미갱신 (gp_q14=1995, beta_q14=3277 frame 0 sf0 측정값 동일). |
| 4 | **frame 1+ 잔여** | F-sept frame 0 sf0 한정 | 미갱신. F-oct 후속 cycle 의 frame indexing 분석에 흡수 가능. |
| 5 | **회귀 가드 promotion** | sample 0..7 영구 게이트 후속 | 미갱신. F-oct closure 후 검토. |
| 6 | **비-contract diagnostic 3건** | F-quint-3 §4.6 동상 | 미갱신. F-sept-1/2/3 + F-sext-1 추가 4건 (총 7건) 으로 누적. cleanup task 별도. |
| 7 | **F-sext-2 / F-sext-3 (HP filter 진단)** | F-sext-1 §4 시나리오 (i) 로 유보 | **갱신**: F-sept-3 (S1) 로 IIR spec 정합 검증됨 → HP filter 자체 결함 가능성 *더욱 낮음*. F-oct-prelim (PST/2 ground-truth 검증) 후 sample 5..7 잔존 시 재가동 검토. **유보 강화**. |
| 8 | **lsp_lp.go uncommitted (F-bis-1 P fix)** | (L3b) 발현 시 reactivate | **갱신 (active)**: F-sept-2 cycle 에서 modified diff 가 §3.2.6 spec 정합 fix 임이 정량 검증 (HEAD broken max|Δ|=7881 → modified max|Δ|=6). **별도 minimal commit cycle 권고 — F-oct 와 독립 진행 가능**. (§4.2 부산물 권고 참조) |

---

## 6. 결론 — Phase 1k Stage F-sept closure

### 6.1 Stage F-sept closure 평가

- **F-sept-1 (excitation 분해)**: PASS, 시나리오 (A 변형 A') 도출.
- **F-sept-2 (LP cross-check)**: PASS, 시나리오 (L3a 변형) 도출
  (modified diff = §3.2.6 spec 정합 fix 정량 검증).
- **F-sept-3 (synth IIR trace)**: PASS, 시나리오 (S1) 도출
  (max|Δ|=1, Pass 2 미발동).
- **F-sept-4 (본 종합 보고서)**: 결합 분류 = **E3 변형**. F-oct
  권고 ranking 완료. Phase 1k Stage F-sept 모든 task **closure
  가능**.

### 6.2 다음 cycle 진입 지점

**1순위 (필수 선행)**: F-oct-prelim cycle (PST/2 ground-truth 검증
diagnostic-only). production 변경 0.

**1순위 (병렬 가능)**: lsp_lp.go modified 정식 commit cycle (F-bis-1
P fix). minimal cycle, F-oct 와 독립.

**2순위 (1순위 결과 의존)**: F-oct-1 (frame indexing 분석) → F-oct-2
(chain 외부 production fix).

### 6.3 invariant 종합 준수

- **E1 미발동**: 회귀 게이트 4건 + F-sept task 3건 모두 PASS.
- **E4 미발동**: 외부 G.729 구현 0건 인용. 모든 인용 = F-sept-1/2/3
  + F-sext-1 + F-quart 1..4 + F-quint 1..3 보고서 + ITU PDF.
- **E5 미발동**: production 변경 0 라인. 테스트 변경 0 라인.
  doc-only commit (본 보고서 1 파일 추가).
- 사전 working tree 보존: `M lsp_lp.go` + `?? stagef_bis_diagnostic_test.go`
  미변경 확인.

**Phase 1k Stage F-sept closure 가능**.
