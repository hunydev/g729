# Phase 1k Stage F-oct-postfix2-prelim-2 보고서 — M5 excitation pre-postfilter 부호 trace

**작성일**: 2026-05-02
**범위**: M5 가설 (excitation pre-postfilter 부호 결함) 측정.
**산출물**: 측정 함수 1 추가 + 3-point sign trace + M5 가설 평가.
**준수**: production 변경 0, 외부 G.729 0 참조, F-sept sample 5..7 한정 미수행 보완.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§A.4.1, §A.4.2)

| 게이트 | 평가 | 근거 |
|--------|------|------|
| E1 (회귀 게이트 1+ FAIL) | non-trigger | 14 contract PASS + item 15 RED 잔존 (의무) + 비-contract 3 FAIL (plan §0.3 허용) |
| E2 (spec § 인용 ↔ PDF grep 불일치) | **trigger 부분** — citation 정정 적용 | plan 추정 "§A.3.5 Excitation reconstruction" 가 PDF grep 결과 §A.3.5 = "Computation of the impulse response" 로 불일치. 정정: §A.4.1 ("Same as described in clause 4.1") → §4.1.5 "Decoding of the adaptive and fixed-codebook gains" (excitation reconstruction 본 정의). 측정 함수 doc-comment 에 정정 사실 명시. |
| E3 (4 가설 중 2+ 잔존) | not yet evaluable (Task 5 결합 분석 후 판단) | — |
| E4 (외부 G.729 인용/대조) | non-trigger | PDF + repo PST 만 사용 |
| E5 (production 변경 > 0) | non-trigger | `git diff --stat` = test 1 파일만 변경 |
| Phase 0.5 (working tree 보존) | OK | `internal/decoder/stagef_bis_diagnostic_test.go` untracked 동일 |

### Spec § PDF verbatim 인용

ITU-T G.729 (06/2012) PDF §A.4.1 (PDF p.42):

```
A.4.1     Parameter decoding procedure
Same as described in clause 4.1.
```

ITU-T G.729 (06/2012) PDF §4.1.5 (PDF p.27, Annex A 가 동상 인계):

```
4.1.5   Decoding of the adaptive and fixed-codebook gains
The received gain-codebook index gives the adaptive-codebook gain ĝ_p
and the fixed-codebook gain correction factor γ̂. ... The fixed-codebook
vector is obtained from the product of the quantized gain correction
factor with this predicted gain equation (74). The adaptive-codebook
gain is reconstructed using equation (73).
```

ITU-T G.729 (06/2012) PDF §A.4.2 (PDF p.42-43, postfilter cascade):

```
A.4.2     Post-processing
The post-processing is the same as described in clause 4.2 except for
some simplification in the adaptive postfilter.

The adaptive postfilter is the cascade of three filters: a long-term
postfilter Hp(z), a short-term postfilter Hf(z) and a tilt compensation
filter Ht(z), followed by an adaptive gain control procedure.
```

## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인

`go test ./internal/decoder/ -run '<14 contract names>|TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput|TestDiagnostic_FoctPostfix2PrelimChainDump|TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace' -v` 결과:

- 14 contract: 전부 `--- PASS`.
- 항목 15: `--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` — `frame 0 sample 5/6/7: got=2 want=-1 (Δ=3)` (RED 잔존 의무 충족).
- 신규 측정: `--- PASS: TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace`.
- 비-contract 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) FAIL — plan §0.3 본문 동상 허용.
- `go vet ./...` clean.

신규 회귀 0건.

## 2. M5 측정 raw 출력 (sample 5..7 3-point sign trace)

`go test ./internal/decoder/ -run TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace -v` (verbatim):

```
-------- M5 excitation pre-postfilter sign trace (ALGTHM frame 0 sf0) --------
PST want sample 5..7 = [-1 -1 -1]  (signs=[− − −])
[M5 sample 5] excitation u[5]=    +0  syn[5]=    +1  pre-post[5]=    +1  postfilter sPf[5]=    +1  sign chain=[0,+,+,+]
[M5 sample 6] excitation u[6]=    +0  syn[6]=    +1  pre-post[6]=    +1  postfilter sPf[6]=    +1  sign chain=[0,+,+,+]
[M5 sample 7] excitation u[7]=    +0  syn[7]=    +1  pre-post[7]=    +1  postfilter sPf[7]=    +1  sign chain=[0,+,+,+]
──────── M5 sign-transition decision ────────
  stage 1 excitation u : [    +0     +0     +0]  signs=[0 0 0]
  stage 2 syn IIR s    : [    +1     +1     +1]  signs=[+ + +]
  stage 3 pre-post in  : [    +1     +1     +1]  signs=[+ + +]
  stage 4 postfilter   : [    +1     +1     +1]  signs=[+ + +]
  stage 5 PST want     : [    -1     -1     -1]  signs=[− − −]
  >>> sign flip between stage 1 (excitation u) and stage 2 (syn IIR s   ) at sample(s): [5 6 7]
  >>> sign flip between stage 4 (postfilter  ) and stage 5 (PST want    ) at sample(s): [5 6 7]
verdict: M5 REFUTED — excitation u[5..7] = 0 (부호 무 ; sign 발생은 chain 후단 합성 IIR 단계). M5 가설 (excitation 자체 부호 결함) 미해당.
```

### 9-값 표 (excitation / syn / pre-postfilter × sample 5/6/7)

| 단계 \ sample | 5 | 6 | 7 |
|---------------|---|---|---|
| excitation u[n] | +0 (zero) | +0 (zero) | +0 (zero) |
| synth IIR syn[n] | +1 | +1 | +1 |
| pre-postfilter (= postfilter.Filter 입력) | +1 | +1 | +1 |
| (보조) postfilter sPf[n] | +1 | +1 | +1 |
| (보조) PST want | −1 | −1 | −1 |

## 3. M5 가설 평가 (반증 / 유력 / 부분)

**M5 = REFUTED.**

근거:
1. excitation u[5..7] 가 *모두 0* — sample-uniform 0 (부호 결함 불가능).
2. PST want = `[−1,−1,−1]` 와의 mismatch 의 sign-determining 단계는 excitation 외부 (= 합성 IIR 단계 또는 그 후단). chain 의 sign-transition 측정에서 stage 1→2 (excitation→syn IIR) 가 0→+ 로 부호 *생성*; stage 4→5 (postfilter→PST want) 가 +→− 로 부호 *반전*. stage 2→3 (syn→pre-post) / stage 3→4 (pre-post→postfilter) 는 부호 보존 (sign flip 0).
3. plan §Step 4 표 의 "excitation u[5..7] 부호 = `[+,+,+]` (PST want 와 반전) → M5 유력" 조건 *미해당* (excitation = 0).
4. plan §Step 4 표 의 "excitation u[5..7] 부호 = `[−,−,−]` (PST want 정합) but post-IIR / post-postfilter 부호 = `[+,+,+]` → M5 반증" 조건 *부분 부합* — excitation 이 PST want 와 **부호상 정합 아닌 부호 무 (0)** 이지만, 결함 단계가 chain 후단인 점 동상.

### sample-uniform 패턴 부합 여부

부합 — sample 5/6/7 모두 동일 단계 (stage 1→2) 에서 부호 *생성* + 동일 단계 (stage 4→5) 에서 *반전*. 9 값이 sample-uniform (각 단계의 [v5,v6,v7] 가 동일) → F-oct-postfix-1 RED 의 Δ=+3 sample-uniform 패턴 (`[+2,+2,+2]` vs want `[−1,−1,−1]`) 와 일관.

### 부호 결정 (sign-determining) 단계 식별

1. **부호 *생성*: 합성 IIR (synth.Filter)** — excitation 0 → syn +1 (sample 5/6/7 동상). 즉 +1 의 출처는 합성 IIR 의 *과거 메모리* (영-입력 시 IIR 의 전이/잔여 응답) 또는 LP 계수 상호작용. 본 측정 시점의 `synth.Synthesizer` 는 zero-state 로 reset 되었으므로 (`syn.Reset()` 명시 호출 부재 — 변수 zero-init), 0 입력 → 0 출력이 spec 기대. 그러나 production decoder `d.syn` 도 frame 0 sf0 진입 시 zero state (Decoder zero value) — *그럼에도* +1 출력은 실제 frame 0 sf0 의 production path (`subframe.go:42 d.syn.Filter(sfA, &u, &s)`) 와 정합 (chain dump `[+2,+2,+2]` 후 ×2 scale 전).

   → **M3 (synth IIR memory propagation) 가설 의 잠재적 trigger** — Task 4 의 M3 정밀 측정으로 인계 (LP 계수 a[1..10] 상호작용 또는 saturation/round 효과 검증).

2. **부호 *반전*: PST want 와 chain 출력 사이 (stage 4→5)** — postfilter sPf = +1 vs PST want = −1. 이 단계는 chain 의 *외부* (PST 파일 측) — **M6 (PST want 데이터 부호) 가설 의 잠재적 trigger** — Task 3 의 M6 정밀 측정으로 인계.

## 4. F-sept-1/-3 측정 결과와의 cross-check

- **F-sept-1** (`TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5`) sample 5 단독: 본 측정의 sample 5 와 정합 (excitation u[5]=0).
- **F-sept-3** (`TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7`) sample 0..7: 본 측정의 syn[5..7] = [+1,+1,+1] 와 정합 (synth.Filter 의 production path 동일 호출).
- 본 task 신규 기여: sample 5..7 한정 *3-point sign trace* (excitation/syn/pre-post 동시 비교) + 부호 전환 단계 명시적 식별 (stage 1→2 생성, stage 4→5 반전).

## 5. Task 3 진입 의무 (M6 측정 baseline)

Task 3 (M6 = PST want 데이터 부호 검증) 진입 시 본 task 의 다음 정량 자료를 ground-truth 로 인계:
- chain 종단 (postfilter sPf) = `[+1,+1,+1]` (×2 scale 전), post-hpfilter (chain dump baseline `ff5534a`) = `[+2,+2,+2]`.
- PST want = `[−1,−1,−1]` 의 출처가 (a) ALGTHM.PST byte-level encoding 결함 / (b) READMETV.txt format 해석 결함 / (c) ITU 원본 vector 자체 의 어느 것인지 byte 검증.
- M6 STRONG 시 → M3 / M1' 측정 우선순위 하향 가능. M6 REFUTED 시 → Task 4 의 M3 (synth IIR memory) + M1' (postfilter 외 분기) 정밀 측정으로 chain 후단 결함 단계 추가 식별.
