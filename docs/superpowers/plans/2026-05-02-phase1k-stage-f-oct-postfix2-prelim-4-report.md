# Phase 1k Stage F-oct-postfix2-prelim-4 보고서 — M1' + M3 정밀 측정

**작성일**: 2026-05-02
**범위**: M1' (postfilter 외 분기) + M3 (synth IIR 재진입) 측정.
**산출물**: postfilter / synth white-box 측정 2 파일 + stage 별 부호 trace.
**준수**: F-oct-prelim-5-3 §3.3 M3 폐기 결정의 sample 5..7 한정 재평가.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§A.4.2.1/2/4 + §A.4.1)

- **E1 (test 우선 진단)**: 충족. 본 task 산출 = 측정-only test 2 파일, assertion 0.
- **E2 (spec § verbatim)**: PDF grep 으로 §A.4.2.1 Long-term postfilter `H_p(z) = 1/(1+γ_p g_l)·(1+γ_p g_l z^−T)` (A.11) 확인. §A.4.1 = "Same as described in clause 4.1." (encoder 측 cross-ref). §A.4.2.4 AGC = §4.2.4 절차 + g_target one-pole α-smoothing. test 파일 doc-comment 에 verbatim 등재.
- **E3 (단일 가설 결정 회피)**: 본 task 는 측정-only — 가설 결정은 Task 5 종합에서 수행.
- **E4 (외부 G.729 0 참조)**: Annex A binary 미사용. ITU PDF + READMETV.txt (testdata) 만 사용.
- **E5 (production 0 라인)**: `internal/postfilter/*.go` + `internal/synth/*.go` 모두 본 task 변경 없음 (`git diff` 검증).

**F-oct-prelim-5-3 §3.3 인용 재독**: "synth IIR memory init = zero dump → M3 폐기" 의 측정 영역은 frame 0 sf0 *codec-start* 에 한정 (pastSynth=[0;10] §4.3 Table 9). 본 task 는 동일 fixture 를 sample 5..7 까지 propagation 하여 memory pre-sample-5 / post-sample-7 의 *상태 변화* 와 *선형성* (forced sign-flip) 을 측정.

**Phase 0.5 사전 보유 working tree 보존**: `?? internal/decoder/stagef_bis_diagnostic_test.go` 미변경 (commit 0 적용).

## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인

```
ok  internal/bitstream
ok  internal/decoder      (단, item 15 = TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput RED 잔존 — 의무)
ok  internal/fcb / fixed / lsp / pcm / pitch / tables
ok  internal/postfilter   (+ TestDiagnostic_FoctPostfix2PrelimM1Prime PASS — 신규 측정)
ok  internal/synth        (+ TestDiagnostic_FoctPostfix2PrelimM3IIRMemory PASS — 신규 측정)
```

게이트 #1~#14 = PASS, #15 = RED 잔존 (다음 fix cycle GREEN gate). 비-contract 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) baseline 과 동일 FAIL — production 0 라인 의무로 자동 보장.

`go vet ./...` clean.

## 2. M1' 측정 raw 출력 (postfilter stage 별 sample 5..7)

`go test ./internal/postfilter/ -run TestDiagnostic_FoctPostfix2PrelimM1Prime -v` 발췌:

```
PST want sample 5..7              = [-1 -1 -1]
excitation u[5..7]                = [+0 +0 +0]
synth IIR s[5..7] (pre-postfilter)= [+1 +1 +1]   signs=[+ + +]
tInt = 20  (refinePitch range = {19,20,21} ∩ [20,143])

aNum (γ_n·a Q12)   = [4096 -1208 -113 -1 -13 -3 8 -1 -1 1 0]
aDen (γ_d·a Q12)   = [4096 -1538 -184 -1 -35 -11 36 -3 -5 6 -1]
residual r[5..7]   = [+1 +1 +1]
refinePitch T      = 20
computeLongTermGain branch=compute  R=20 E=22  → g0=10922 g1=5461 (Q14)
longterm  rOut[5..7] = [+1 +1 +1]
shortterm sSt[5..7]  = [+1 +1 +1]
computeTiltMu branch=inactive(γ_t=3277)  μ_Q15=-558
tilt      sTilt[5..7]= [+1 +1 +1]
computeAGCTargetGain g_target_Q14=18364
applyAGC branch=init-seed(agcGainPrev←gTargetQ24)  pre=0 → post=18804736
AGC       sPf[5..7]  = [+1 +1 +1]

replicated chain == production Postfilter.Filter ? true
```

Sign-chain (sample 5..7):

| stage | signs |
|-------|-------|
| input s | [+ + +] |
| residual r | [+ + +] |
| longterm rOut | [+ + +] |
| shortterm sSt | [+ + +] |
| tilt sTilt | [+ + +] |
| AGC sPf | [+ + +] |
| **PST want** | **[− − −]** |

**분기 cover**: longterm = `compute` 분기 (R=20>0, E=22≠0 → clamp 분기 미진입), tilt = `inactive(γ_t=3277)` (codec-start agcGainPrev=0), AGC = `init-seed` (initialized=false → agcGainPrev seeded from g_target).

## 3. M3 측정 raw 출력 (IIR memory pre/post sample 5..7)

`go test ./internal/synth/ -run TestDiagnostic_FoctPostfix2PrelimM3IIRMemory -v` 발췌:

```
LP a[0..10] (Q12) = [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]
excitation u[0..7] = [+1 +1 +1 +1 +0 +0 +0 +0]
PST want sample 5..7 = [-1 -1 -1]   signs=[− − −]
pastSynth (codec-start) = [0 0 0 0 0 0 0 0 0 0]   (§4.3 Table 9 zero invariant)

[M3 IIR memory pre-sample-5]  mem[0..9] = [1 2 2 2 1 0 0 0 0 0]
[M3 IIR memory post-sample-7] mem[0..9] = [1 1 1 1 2 2 2 1 0 0]
syn[0..7] (replayed Pass-1) = [1 2 2 2 1 1 1 1]
Pass-1 fixed.Overflow = false
syn[0..7] (production) = [1 2 2 2 1 1 1 1]    (replay == production)

memory sign changes (pre-5 vs post-7, non-zero pairs only) = 0 / 10

──── forced sign-flip stimulus (-u) ────
syn(-u) sample 0..7 = [-1 -2 -2 -2 -1 -1 -1 -1]
syn(-u) == -syn(+u) for sample 0..7 ? true   (Pass-1 linear IIR invariant)
```

## 4. M1' 가설 평가 + cover 결손 분기 ID

**시나리오 M1'-D 적중** — input `s[5..7]` = [+ + +] = sPf[5..7]; postfilter chain 의 모든 stage 가 부호 보존 (sign-chain 표 6 행 모두 [+ + +]). 모든 분기 (longterm `compute` / tilt `inactive` / AGC `init-seed`) 가 의도된 codec-start 경로로 활성화 — *cover 결손 분기 0건*.

→ **M1' 반증.** F-oct-prelim-5-3 의 "postfilter sign-preserving" 결정 재확인. 결함 위치 = postfilter *상류* (synth IIR / excitation / LP).

## 5. M3 가설 평가 + F-oct-prelim-5-3 §3.3 결정 재평가

**시나리오 M3-C 적중** — 측정 3건 모두 "spec 정합":

1. **memory zero init**: codec-start pastSynth = [0;10] (§4.3 Table 9).
2. **memory propagation 부호 변화 = 0/10**: pre-sample-5 mem [1 2 2 2 1 0 0 0 0 0] → post-sample-7 mem [1 1 1 1 2 2 2 1 0 0] — 모두 비음수 (zero element 만 변동, sign flip 0).
3. **선형 IIR invariant**: `syn(-u) == -syn(+u)` for sample 0..7 (Pass-1 only, fixed.Overflow=false → Pass-2 미발동).

→ **M3 반증** (F-oct-prelim-5-3 §3.3 의 sample 0..4 한정 결정을 sample 5..7 까지 확장 — 결정 *유지*). IIR 산술 §3.10 spec 정합. 결함 = excitation 입력 u[] (M5 영역) 또는 LP a[] 변환.

핵심 관찰: u[5..7] = [+0 +0 +0] (zero excitation) 이지만 syn[5..7] = [+1 +1 +1] — 즉 **부호 *생성* 단계 = IIR 의 직전 비-zero 메모리 (u[0..4]=[1 1 1 1 0] → syn[0..4]=[1 2 2 2 1]) 에 의한 자기-피드백**. memory 자체는 양수 (codec-start zero 에서 양 excitation 만 받았으므로 자연스러운 spec 거동), 따라서 syn[5..7] 양수 = §3.10 spec 거동.

→ "syn[5..7] 양수 = spec" 가 측정 결과로 **공식 확립**. PST want [-1 -1 -1] 와의 부호 불일치는 *상류 결함* 으로만 설명 가능.

## 6. Task 5 진입 의무 (4 가설 비교 + 다음 cycle 결정)

본 cycle 4 가설 측정 종합 (F-oct-postfix2-prelim 1~4 commit 누적):

| 가설 | task | 결과 |
|------|------|------|
| M5 (excitation pre-postfilter sign defect) | Task 2 (`6dc851e`) | 측정 데이터로 §M5 평가 (보고서 §3) |
| M6 (PST want byte parsing defect) | Task 3 (`cb9529d`) | 반증 — PST want = [-1 -1 -1] 검증, 9 vector 분포 [−,−,−] |
| M1' (postfilter 외 분기 cover 결손) | Task 4 (본) | **반증** — 모든 분기 spec 경로, sign-chain 6 stage [+ + +] |
| M3 (synth IIR memory propagation) | Task 4 (본) | **반증** — zero init / sign change 0 / 선형 invariant |

→ Task 5 §2 4 가설 비교표 입력 ready. M5 단독 잔존 가능성 vs 4 가설 모두 반증 (= spec 영역 확장 진단 cycle 권고) 의 결정은 Task 5 종합에서 수행.
