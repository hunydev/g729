# Phase 1k Stage F-non-prelim-1 보고서 — X excitation u[0..4] sub-항 분리

**작성일**: 2026-05-03
**범위**: 후보 X (excitation u[0..4] 부호) 의 sub-항 (g_p / g_c / v / c) 분리 측정.
**산출물**: 측정 함수 1 신규 파일 (`internal/decoder/stagef_fnonprelim_diagnostic_test.go`) + sub-항별 raw + 부호 결정성 평가.
**준수**: production 변경 0, 외부 G.729 0 참조, F-oct-postfix2-prelim Task 4 §3 의 u[0..4] dump baseline 인계.

---

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G-N1 결정 정합성

### 0.1 Working tree (Task 진입 직전)

```
?? internal/decoder/stagef_bis_diagnostic_test.go
9a5a7f6 docs(plans): F-oct-postfix2-prelim synthesis + cycle decision
658090b docs(plans): add Phase 1k Stage F-non-prelim plan
```

`stagef_bis_diagnostic_test.go` (untracked, Phase 0.5 보존 의무) 미변경.

### 0.2 escape hatch 평가

- **E1 (회귀 게이트 1+ FAIL)**: 미발동 — 15 PASS + 항목 16 (F-oct-postfix RED 잔존 의무) 만 RED, 신규 측정 PASS.
- **E2 (spec § 인용 불일치)**: **발동** — plan §"Spec § 인용" 이 가법 분해 `u[n] = g_p · v[n] + g_c · c[n]` 의 출처를 §A.3.5 로 인용하나, PDF (G729E.pdf) verbatim grep 결과 §A.3.5 = "Computation of the impulse response" (encoder side). 본 식의 spec 출처는 §4.1.5 (gain decoding) + §4.1.6 eq. (75) (excitation 합성) 이며 Annex A decoder 는 §A.4.1 "Same as described in clause 4.1" 로 §4.1.5/§4.1.6 을 그대로 재사용. 본 보고서 §0.4 의 verbatim 인용으로 정정. 동일 정정은 F-oct-postfix2-prelim §0 에 선행 기록됨.
- **E3 (강압-적합 회피 위반)**: 미발동 — verdict 분류 함수 (`classifyFnonSignDetermining`, `classifyFnonXHypothesis`) 는 우선 가설 없이 `pitchAllZero` / `codeAllZero` boolean 만으로 단독/hybrid/refute 결정.
- **E4 (외부 G.729 구현 참조)**: 미발동 — PDF + READMETV.txt 만 인용, Annex A binary 미참조.
- **E5 (production 변경)**: 미발동 — `git diff` 결과 production 0 라인 변경.

### 0.3 사용자 G-N1 결정 정합성

G-N1 = "(a) X 우선 정합". 본 task 는 X (excitation u[0..4] sub-항 분리) 측정 단독 — Y/Z 측정 또는 W 후보 진단 미혼합. G-N1 정합.

### 0.4 spec § PDF verbatim 인용

§4.1.5 (PDF p.27):
> **4.1.5 Decoding of the adaptive and fixed-codebook gains**
> The received gain-codebook index gives the adaptive-codebook gain ĝ_p and the fixed-codebook gain correction factor γ̂. … The estimated fixed-codebook gain g_c′ is found using equation (71). … The adaptive-codebook gain is reconstructed using equation (73).

§4.1.6 eq. (75) (PDF p.27, 본 task 의 가법 분해 ground-truth):
> subframe is obtained using:
> &nbsp;&nbsp;&nbsp;&nbsp;**u(n) = ĝ_p v(n) + ĝ_c c(n)**, n = 0,…,39 &nbsp;&nbsp;&nbsp;(75)
> where ĝ_p and ĝ_c are the quantized adaptive and fixed-codebook gains, respectively, v(n) is the adaptive-codebook vector (interpolated past excitation), and c(n) is the fixed-codebook vector including harmonic enhancement.

§A.4.1 (PDF p.42):
> **A.4.1 Parameter decoding procedure**
> Same as described in clause 4.1.

Q-format (production 본문 + `internal/synth/excitation.go` doc):
- ĝ_p = Q14 Word16, ĝ_c = Q12 Word16
- v(n) = Q0 Word16, c(n) = Q13 Word16
- 산술: `lPitch = LMult(g_p, v) → Q15`, `lCode = LShr(LMult(g_c, c), 11) → Q15`, `u = Round(LShl(lPitch+lCode, 1))`.

---

## 1. 회귀 게이트 baseline (15 PASS + 항목 16 RED + 신규 PASS)

| # | 명령 | 결과 |
|---|------|------|
| 1–15 | `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v` | PASS |
| | `go test ./internal/decoder/ -run "TestDiagnostic_F(quart\|sext\|sept\|octPrelim\|OctPrelim5\|OctPostfix2Prelim)" -v` | PASS (전건) |
| | `go test ./internal/postfilter/ ./internal/synth/ -v -run Contract` | PASS (전건) |
| | `go vet ./...` | clean |
| 16 | `go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v` | **RED 잔존 (의무)** — `got=2 want=-1 (Δ=3)` × sample 5/6/7 |
| 신규 | `go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimXExcitationSubterms -v` | **PASS** |

E1 미발동.

---

## 2. sub-항 raw 출력 (sample 0..4 한정)

신규 `TestDiagnostic_FnonPrelimXExcitationSubterms` (commit 직전 측정) verbatim 출력:

```
indices: P1=2 C1=0x0000 S1=0xf GA1=5 GB1=6
pitch delay: tInt=20 tFrac=0   beta_q14=3277
[X g_p Q14]  value= +1995  sign=+  Q-format=Q14
[X g_c Q12]  value= +4153  sign=+  Q-format=Q12

──────── sub-term raw (sample 0..4) ────────
[X v[0..4]]    pitch codebook v   = [    +0     +0     +0     +0     +0]  signs=[0 0 0 0 0]
[X c[0..4]]    fcb codebook c     = [ +8192  +8192  +8192  +8192     +0]  signs=[+ + + + 0]
[X g_p·v Q15]  pre-Round int32    = [      +0       +0       +0       +0       +0]  signs=[0 0 0 0 0]
[X g_c·c Q15]  pre-Round int32    = [  +33224   +33224   +33224   +33224       +0]  signs=[+ + + + 0]
[X (g_p·v + g_c·c) Q15]            = [  +33224   +33224   +33224   +33224       +0]  signs=[+ + + + 0]

──────── sub-term Q0-rounded contribution (g_c=0 / g_p=0 isolation) ────────
[X u_pitch[0..4]]  (g_c=0)         = [    +0     +0     +0     +0     +0]  signs=[0 0 0 0 0]
[X u_code [0..4]]  (g_p=0)         = [    +1     +1     +1     +1     +0]  signs=[+ + + + 0]

──────── composite + replication 검증 ────────
[X u[0..4]]    composite (replicated) = [    +1     +1     +1     +1     +0]  expected=[+1 +1 +1 +1 +0]
[X u[0..4]]    production decodeSubframe = [    +1     +1     +1     +1     +0]
[X replication match (all 40 samples)] = true
```

### 2.1 가법 분해 검증 표 (sample 0..4)

| n | g_p (Q14) | g_c (Q12) | v[n] (Q0) | c[n] (Q13) | g_p·v Q15 | g_c·c Q15 | sum Q15 | u[n] (Q0) | round 검증 |
|---|-----------|-----------|-----------|------------|-----------|-----------|---------|-----------|-----------|
| 0 | +1995 | +4153 | +0 | +8192 | +0 | +33224 | +33224 | **+1** | `Round(LShl(33224,1)) = Round(66448) = 1` ✓ |
| 1 | +1995 | +4153 | +0 | +8192 | +0 | +33224 | +33224 | **+1** | ✓ |
| 2 | +1995 | +4153 | +0 | +8192 | +0 | +33224 | +33224 | **+1** | ✓ |
| 3 | +1995 | +4153 | +0 | +8192 | +0 | +33224 | +33224 | **+1** | ✓ |
| 4 | +1995 | +4153 | +0 | +0    | +0 | +0     | +0     | **+0** | ✓ |

`Round(L) = (L + 32768) >> 16` (positive saturation 미발생). `66448 + 32768 = 99216`, `99216 >> 16 = 1`. 가법 분해 + Q-정합 모두 정합.

### 2.2 replication 검증

`production decodeSubframe` (full pipeline) 의 u[0..4] = `[+1,+1,+1,+1,+0]` = replicated path 출력과 정확 일치, 40 sample 전체 (`replication match = true`). Phase 0.4 §1 의 replication 결함 미발생 — 측정 신뢰도 HIGH.

---

## 3. X 후보 sub-항 부호 결정성 평가 (단일 / hybrid / replication 결함)

### 3.1 측정 결과

- **g_p·v[n] (pitch contribution)**: sample 0..4 전체에서 v[n]=0 (이유: ALGTHM frame 0 sf0 의 tInt=20, fractional=0; 첫 frame 의 `pastExc` 는 zero-init → adaptive codebook 출력 v[n] = pastExc[lookup] = 0 ∀n<20). g_p > 0 이지만 v=0 ⇒ 기여 zero.
- **g_c·c[n] (fcb contribution)**: sample 0..3 에서 c[n]=+8192 (Q13 = +1.0; 4-pulse ACELP positions/signs 외부 sample 중 pitch enhancement β·c[n−tInt] 누적 결과인 듯하나 확정은 fcb/decode.go 검토 필요), sample 4 에서 c[4]=0. g_c=+4153 > 0 → 기여 +1 (sample 0..3), +0 (sample 4).
- **합산 u[n]**: `[+1,+1,+1,+1,+0]` — fcb contribution 단독 결정.

### 3.2 verdict (Phase 0.4 §1 측정-driven)

| 평가 항목 | 결과 |
|----------|------|
| sub-항 부호 결정성 분류 함수 출력 | `g_c·c (fcb contribution) 단독 결정` |
| Phase 0.4 §1 (강압-적합 회피) 준수 | ✓ — pitchAllZero=true 단일 조건만으로 결정, 우선 가설 없음 |
| 모순 / 모호 (단일 sub-항이 합성 결과인지 vs 단독인지) | **무** — pitch 측 raw Q15=0 (pre-Round 단계에서 이미 0), 단순 합성이 아님 |

### 3.3 X 가설 평가

| 가설 | 정의 | 본 측정 정합 |
|------|------|-------------|
| **X-pos** | pitch contribution `g_p·v[n]` 단독 결정 | **반증** — v[n]≡0 → 기여 0 |
| **X-fcb** | fcb contribution `g_c·c[n]` 단독 결정 | **유력** — sample 0..3 단독 +1, sample 4 = 0 (c=0 → 0 정합) |
| **X-both** | hybrid (양 sub-항 모두 기여) | **반증** — pitch 측 zero |
| **X-refute** | 두 sub-항 모두 zero | **반증** — fcb 측 비-zero |

**verdict: X-fcb (fcb contribution g_c·c[n] 단독으로 u[0..4] 부호 결정 — fix scope = fcb.Decode 또는 g_c decoding).**

### 3.4 단서 / 한계

- 본 cycle 의 부호 defect 는 syn[5..7]=+1 (PST want=-1) 의 source 로 의심되는 u[0..4] 양수 입력. u[0..4]=`[+1,+1,+1,+1,+0]` 의 부호 source 는 **fcb path 단독** 으로 식별. fix 후보 = (a) `fcb.Decode` 의 pulse sign 처리, (b) `gain.Decoder.Decode` 의 g_c 부호.
- 주의: 본 측정은 *u[0..4] 의 부호 source* 만 식별. u[0..4]=+1 자체가 *결함* 인지 여부는 별도 cross-check 필요 (Y/Z 측정 cycle 에서 LP a[] + postfilter chain 정합성 검증). 부호 source 식별 ≠ 부호 결함 확정.
- v[0..4]=0 은 첫 frame artefact (zero past_exc); pitch contribution X-pos 가설은 본 vector / 본 frame 한정으로 반증, 일반 (mid-stream subframe) 에서는 재평가 필요.

---

## 4. F-oct-postfix2-prelim Task 4 §3 u[0..4] dump 와의 cross-check

F-oct-postfix2-prelim Task 4 (commit `9a5a7f6`) §1.4 (1) 의 u[0..4]=`[+1,+1,+1,+1,+0]` baseline 과 본 측정 결과 정확 일치 (40 sample replication match=true). M5 (excitation 자체 부호 결함) 가설은 동 cycle 에서 REFUTED 였으나 본 cycle 의 task 정의는 "u[0..4] 가 syn[5..7] self-feedback source" 라는 합성 IIR 기반 추론 (M1'/M3 도 REFUTED) 이후 *u[0..4] 의 부호가 어디서 오는가* 라는 한 단계 상류 질문으로 재정의. 본 task 의 답: **fcb path 단독**.

cross-check 정합 — F-oct-postfix2-prelim Task 4 §3 u[0..4] dump = `[+1,+1,+1,+1,+0]` ≡ 본 측정 = `[+1,+1,+1,+1,+0]`.

---

## 5. Task 2 (Y) 진입 의무 항목

- **Y 측정 대상**: LP a[0..10] (sample 5..7 영역 한정) cross-check. spec §A.3.2/3 (LSP→LP 변환) + §A.4.1 (decoder LP 동상). reference cmp + forced-flip.
- **본 task 의 산출 인계**: u[0..4] = `[+1,+1,+1,+1,+0]` (X-fcb 단독 결정) — Y 측정 시 합성 IIR 입력으로 그대로 사용.
- **Phase 0.4 §1 의무 유지**: Y verdict 도 측정-driven 분류만 수행, 우선 가설 (a[5..7] 가 부호 source) 강압 금지.
- **Phase 0.5**: `stagef_bis_diagnostic_test.go` 보존 (Task 2 진입 시 재확인).
- **회귀 게이트**: 16 항목 + 본 cycle 신규 1 항목 = 17 항목 으로 누적; 항목 16 RED 잔존 의무.

X-fcb verdict 는 fix scope 후보 (fcb / g_c) 를 자동 협소화 — Task 4 종합 시 Y/Z 결과와 결합하여 단일 fix cycle (F-non-fix) 진입 또는 추가 진단 cycle 결정.
