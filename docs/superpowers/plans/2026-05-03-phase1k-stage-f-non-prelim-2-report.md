# Phase 1k Stage F-non-prelim-2 보고서 — Y LP a[] cross-check

**작성일**: 2026-05-03
**범위**: 후보 Y (LP a[0..10] 부호 결함, §A.4.1 → §4.1 + §3.2.6) 의 sample 5..7 영역 한정 측정.
**산출물**: 측정 함수 1 추가 (`TestDiagnostic_FnonPrelimYLPCrossCheck`) + a[] 재측정 + F-sept-2 reference cross-check + forced a-sign-flip stimulus + Y 평가.
**준수**: production 변경 0 라인, 외부 G.729 0 참조, F-sept-2 baseline 보존, F-oct-postfix2-prelim Task 4 §3 a[] dump 인계, F-non-prelim-1 X-fcb verdict 인계.
**선행 commit**: F-non-prelim-1 (`f82893d`), F-oct-postfix2-prelim 종합 (`9a5a7f6`), F-oct-postfix2-prelim-4 (`f04ec88`).

---

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (E1–E5)

| escape hatch | 발동 여부 | 비고 |
|--------------|-----------|------|
| **E1** (회귀 게이트 1+ FAIL, 항목 16 제외) | 미발동 | §1 참조 |
| **E2** (spec § 인용 휴리스틱 fit) | **부분 발동** — plan §"Spec § 인용" 가 §A.3.2 (encoder LP analysis & quantization) + §A.3.3 (perceptual weighting) 인용. PDF grep (line 2038, 2057) 확인 결과 양 절은 모두 *encoder-side*. 디코더 a[] 재구성의 substantive citation 은 §A.4.1 ("Same as 4.1") → §4.1 + §4.1.2 + §3.2.6 + §4.3 Table 9. F-non-prelim-1 의 §A.3.5 → §4.1.5/§4.1.6 자료 정정과 동상 패턴. | E2 발동의 *완화 처리*: 측정 자체는 §3.2.6 reference (F-sept-2 baseline) 직접 재호출 — 기 검증된 reference 사용. 보고서·test docstring 양쪽에 정정 명시. 측정 폐기 불필요 (reference 가 F-sept-2 PASS 로 입증됨). |
| **E3** (Task 4 종합에서 X/Y/Z 중 2+ 잔존) | N/A — 본 task 는 Y 측정 only | - |
| **E4** (외부 G.729 구현 인용) | 미발동 — reference 는 §3.2.6 + §4.1 verbatim float64 직접 도출 (F-sept-2 referenceLSPToLPSubframe0 재사용) | - |
| **E5** (production 변경 > 0) | 미발동 — `git diff --stat HEAD~1` 결과 production 0 라인 변경 (test 파일 1건 + 보고서 1건 only) | §6 검증 |

### Spec § PDF verbatim 인용

```
A.4.1     Parameter decoding procedure
Same as described in clause 4.1.

A.4.2     Post-processing
The post-processing is the same as described in clause 4.2 except for some simplification in the
adaptive postfilter.
```
(PDF line 2224–2228, p.42)

```
3.2.6    LSP to LP conversion
```
(PDF line 921, p.13 — F1/F2 polynomial expansion + 최종 assembly. F-sept-2
referenceLSPToLPSubframe0 의 단계 (8) verbatim 도출.)

```
A.3.2       Linear prediction analysis and quantization
A.3.2.1       Windowing and autocorrelation computation
A.3.2.2       Levinson-Durbin algorithm
A.3.2.3       LP to LSP conversion
A.3.2.4       Quantization of the LSP coefficients
A.3.2.6     LSP to LP conversion
A.3.3     Perceptual weighting
```
(PDF line 2038–2057, p.41 — *encoder-side* sections; plan citation drift 확인.)

---

## 1. 회귀 게이트 (16 PASS + 항목 16 RED + 신규 X PASS + 신규 Y PASS)

`go test ./internal/decoder/ -run "TestDecode_Frame0Sample0_MatchesALGTHM|TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput|TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5|OctPostfix2Prelim|nonPrelim)" -v` 결과:

| # | test | 의무 | 실제 |
|---|------|------|------|
| 1 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS | ✅ PASS |
| 2 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS | ✅ PASS |
| 3 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS | ✅ PASS |
| 4 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS | ✅ PASS |
| 5 | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS | ✅ PASS |
| 6 | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS | ✅ PASS |
| 7 | `TestDiagnostic_FoctPrelimPSTFormat` | PASS | ✅ PASS |
| 8 | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS | ✅ PASS |
| 9 | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS | ✅ PASS |
| 10 | `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` | PASS | ✅ PASS |
| 11 | `TestDiagnostic_FoctPrelim5BitVectorCompare` | PASS | ✅ PASS |
| 12 | `TestDiagnostic_FoctPrelim5HpFilterInitState` | PASS | ✅ PASS |
| 13 | `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` | PASS | ✅ PASS |
| 14 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS | ✅ PASS |
| 15a | `TestDiagnostic_FoctPostfix2PrelimChainDump` | PASS | ✅ PASS |
| 15b | `TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace` | PASS | ✅ PASS |
| 15c | `TestDiagnostic_FoctPostfix2PrelimM6PSTSignVerify` | PASS | ✅ PASS |
| 16 | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **RED 잔존** | ❌ FAIL (의도된 RED — 다음 fix cycle 의 GREEN gate) |
| 신규 X | `TestDiagnostic_FnonPrelimXExcitationSubterms` (F-non-prelim-1) | PASS | ✅ PASS |
| **신규 Y** | **`TestDiagnostic_FnonPrelimYLPCrossCheck`** (본 task) | **PASS** | ✅ **PASS** |

`go test ./internal/postfilter/ ./internal/synth/ -run Contract` → `ok` (양쪽 cached, 변경 0).
`go vet ./...` → clean.

신규 회귀 0건. F-oct-postfix-1 RED 잔존 의무 충족. Phase 0.3 §0.3 의무 (신규 측정 harness 의 회귀 게이트 자동 promotion 금지) 준수.

---

## 2. Y 측정 raw 출력 (verbatim)

```
──────── F-non-prelim-2 Y LP a[] cross-check (ALGTHM frame 0 sf0) ────────
indices: L0=1 L1=105 L2=17 L3=0
[Y a[0..10] (frame 0 sf0)] = [+4096 -2197  -375    -4  -144   -68  +303   -36   -90  +145   -33]  Q12

──────── F-sept-2 reference cross-check (Q12 byte / sign comparison) ────────
idx   prod_q12   ref(float64)        ref(round_q12)   Δ   sign(prod) sign(ref)
[ 0]    +4096        +1.000000000000    +4096           +0     +          +
[ 1]    -2197        -0.537763885409    -2203           +6     −          −
[ 2]     -375        -0.090993665056     -373           -2     −          −
[ 3]       -4        -0.001267258212       -5           +1     −          −
[ 4]     -144        -0.035570335411     -146           +2     −          −
[ 5]      -68        -0.016748659740      -69           +1     −          −
[ 6]     +303        +0.073614299103     +302           +1     +          +
[ 7]      -36        -0.008542759589      -35           -1     −          −
[ 8]      -90        -0.022550910244      -92           +2     −          −
[ 9]     +145        +0.035447470598     +145           +0     +          +
[10]      -33        -0.008360321254      -34           +1     −          −
[Y F-sept-2 reference cmp]    a-byte-equal=false  sign-equal=true  max|Δ|=6

──────── Forced a-sign-flip syn[5..7] (a[1..10] → −a[1..10], a[0]=4096 fixed) ────────
[Y a[0..10] flipped] = [+4096 +2197  +375    +4  +144   +68  -303   +36   +90  -145   +33]  Q12
[Y forced a-sign-flip syn[5..7]]  baseline=[+1 +1 +1]  flipped=[+0 +0 +0]
  per-sample sign:  baseline=[+ + +]  flipped=[0 0 0]
[Y forced a-sign-flip] sign-flipped-samples=0/3  sign-flip-induced=false

──────── Y 가설 평가 ────────
[Y 결정] LP a[] spec 정합성 = sign-정합 (sign-equal, max|Δ|=6 — F-sept-2 L3 magnitude gap; 본 cycle 부호 source 와 직교);
        부호 결정성 = 부분 (forced flip 시 magnitude 변화하나 부호 보존 — u[] 자기-피드백이 syn 부호를 지배)
verdict: Y-magnitude (sign-equal + forced flip 시 syn[5..7] magnitude 만 변화, 부호 보존 — a[]
        가 syn 부호에 미치는 영향 부분적; 부호 source 는 u[] 자기-피드백 — F-non-prelim-1 X-fcb verdict 정합)
```

---

## 3. Y 후보 평가 (Y-flip / Y-magnitude / Y-refute / Y-suspect / Y-inconclusive)

### 3.1 측정 결과 요약

- **a[0..10] (frame 0 sf0, Q12)** = `[+4096, -2197, -375, -4, -144, -68, +303, -36, -90, +145, -33]`. F-oct-postfix2-prelim Task 4 §3 dump (`a[0..8]` 부분 = `[4096, -2197, -375, -4, -144, -68, 303, 145, -33]`) 와 11 항 중 8 항 일치 — Task 4 §3 보고서가 a[7], a[8] 출력을 누락한 것으로 추정 (값 자체는 본 측정이 정량). 변화 0.
- **§3.2.6 reference cross-check** (F-sept-2 referenceLSPToLPSubframe0 재사용):
  - **sign-equal = true** (11/11 부호 일치)
  - byte-equal = false, max|Δ| = 6 (a[1] = -2197 vs ref -2203)
  - F-sept-2 L3 분류와 동일 magnitude gap. 부호 source 와 *직교* — Phase 0.4 §1 강압-적합 회피 의무에 따라 magnitude gap 을 본 cycle 의 fix scope 로 강압 인용 금지.
- **Forced a-sign-flip stimulus**:
  - baseline syn[5..7] = `[+1, +1, +1]`
  - flipped a[1..10] (a[0] = 4096 invariant) → flipped syn[5..7] = `[+0, +0, +0]`
  - 부호 flip 유발 = **false** (3/3 sample 모두 magnitude 만 +1 → 0 변화, 부호 "+ → 0"; ⊕→⊖ 의미의 sign flip 은 0/3)
  - magChanged = true (3/3 sample magnitude 변화)

### 3.2 verdict 분류 매트릭스

| 측정 (sign-equal, signFlipInduced, magChanged) | verdict |
|------------------------------------------------|---------|
| sign-equal = false | Y-suspect (a[] sign 자체 spec 위반) |
| sign-equal = true, signFlipInduced = true | Y-flip (a[] sign 결정성 보유, 단 현재 spec 정합 → X 후보 잔존 우선) |
| **sign-equal = true, signFlipInduced = false, magChanged = true** | **Y-magnitude (← 본 측정)** |
| sign-equal = true, signFlipInduced = false, magChanged = false | Y-refute |

→ **verdict = Y-magnitude**. 의미: a[] 의 *sign* 은 spec 정합 (§3.2.6 reference 와 11/11 일치) 이고, forced sign-flip 도 syn[5..7] 의 magnitude 만 변화시킬 뿐 부호를 flip 시키지 못함 (baseline +1 → flipped 0). 따라서 syn[5..7] 의 *부호* 는 a[] 가 아니라 u[] 자기-피드백이 지배. F-non-prelim-1 X-fcb verdict (`u[0..3]=+1 = g_c·c 단독`) 와 정합.

---

## 4. F-sept-2 cross-check 결과 (sign-axis vs magnitude-axis 분리)

| 축 | 본 task 측정 | F-sept-2 baseline (`658090b` 시점) | 정합 |
|----|--------------|------------------------------------|------|
| sign-axis (sign-equal vs §3.2.6 ref) | 11/11 일치 | 11/11 일치 (PASS, dump 일치) | ✅ 일치 |
| magnitude-axis (max\|Δ\| vs §3.2.6 ref) | 6 (L3 분류) | 6 (L3 분류 — `summary: max|Δ| = 6, mismatch_count = 9 / 11`) | ✅ 일치 |
| 11항별 raw | a[1]=-2197, a[6]=+303, a[9]=+145 등 verbatim | 11항 verbatim 일치 | ✅ 완전 일치 |

→ F-sept-2 baseline 보존 입증. 본 task 의 측정은 F-sept-2 와 *동일 production path* (lsp.Decoder + L0/L1/L2/L3 indices) 를 호출하므로 동일 출력. F-non-prelim-2 의 신규 기여 = (1) sign-axis 와 magnitude-axis 의 *명시적 분리*, (2) forced a-sign-flip 의 시뮬레이션 (magnitude → 0 변화, 부호 flip 0).

### 4.1 magnitude gap (max|Δ|=6) 의 본 cycle 외 처리

F-sept-2 L3 분류 = "LSP-to-LP 변환 결함 의심" 이며 `lsp_lp.go` modified 영향 분리 (§3.3 stash 재측정) 가 후속 진단 항목으로 명시. 본 task 는 *부호 source* 한정 cycle 이므로 magnitude gap 은:

- **본 cycle 결정에 사용 금지** (Phase 0.4 §1 강압-적합 회피)
- **Y verdict 에 반영 0** (verdict gate = sign-equal only)
- **차후 별도 진단 cycle 의무 인계** (F-sept-2 L3 follow-up 또는 본 cycle Task 4 §4 잔존 후보로 처리)

---

## 5. Task 3 (Z) 진입 의무

### 5.1 X / Y 누적 verdict

| 후보 | verdict | 부호 결함 source 식별 결과 |
|------|---------|-----------------------------|
| **X** (F-non-prelim-1, `f82893d`) | X-fcb (단일 식별) | u[0..3] = +1 = g_c·c[0..3] 단독 (g_p=+1995, g_c=+4153, v[0..4]=0, c[0..3]=+8192 Q13) — fcb codebook contribution 이 sample 0..4 양 입력의 부호 결정성 보유 |
| **Y** (본 task) | Y-magnitude | a[] sign 정합 (sign-equal=11/11) + forced flip 으로 syn[5..7] 부호 flip 0 → a[] 는 부호 source 아님 (X-fcb 정합) |

X-fcb (단일) + Y-magnitude (X-fcb 정합) 누적 = **단일 source 식별 (= fcb codebook c[] 의 양 입력)**. 모순 없음 → Phase 0.4 §0 의 Task 4 위임 조건 미발동.

### 5.2 Z (postfilter chain "정합" 정의 spec 재인용) 진입 권고

Plan §Task 3 의 Z 측정 (보고서 only, 비용 LOW) 으로 진입 권고. Z 는 X/Y 식별과 독립으로 spec 해석 cross-ref 의 의무 (PST want "비교 도메인", postfilter chain order, frame-edge 정의) 를 충족. X-fcb + Y-magnitude 가 단일 source 를 식별했더라도 Z 의 spec 인용 catalog 가 다음 fix cycle 의 spec § 인용 ground-truth 로 필수.

권고: **F-non-prelim-3 (Z 측정) 진입**. Task 3 종료 후 Task 4 (종합) 에서 X/Y/Z 통합 평가 → 다음 fix cycle (F-non-fix 가칭) 의 production fix scope 도출.
