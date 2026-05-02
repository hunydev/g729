# Phase 1k Stage F-non-prelim-3 보고서 — Z spec 해석 재검토

**작성일**: 2026-05-03
**범위**: 후보 Z (postfilter chain "정합" 정의 spec 재인용) — 비용 LOW 보고서 only.
**산출물**: spec § verbatim 인용 catalog + production cross-ref 표 + PST 비교 도메인 평가.
**준수**: production 변경 0, test 변경 0, 외부 G.729 0 참조 (E4 invariant).

---

## 0. escape hatch 평가 + 사전 조건

- **E1 (회귀)**: 본 task 코드 변경 0 → 자동 보존.
- **E2 (spec 인용 mismatch)**: §2 grep 결과와 plan 상단 §"Spec § 인용" 인용 4·5 추정 정합 확인 (mismatch 0).
- **E3 (3 후보 모두 반증)**: Task 4 (synthesis) 에서 X+Y+Z 종합 평가 후 결정.
- **E4 (외부 G.729 참조)**: 본 task 인용 출처 = ITU-T G.729 (06/2012) PDF + READMETV.txt 2건 만. 외부 0건.
- **E5 (Annex A binary 사용)**: 본 task 측정 0건 (보고서 only) → 적용 무관.
- **사전 보유 working tree 보존**: `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 (`git status --porcelain` 확인).

**사전 조건 (Step 1) — `git log --oneline -4`**:

```
d1a4f2d test(decoder): add Stage F-non-prelim-2 Y LP a[] cross-check
f82893d test(decoder): add Stage F-non-prelim-1 X excitation sub-term decomposition
658090b docs(plans): add Phase 1k Stage F-non-prelim plan
9a5a7f6 docs(plans): F-oct-postfix2-prelim synthesis + cycle decision
```

→ Task 2 (`d1a4f2d`) + Task 1 (`f82893d`) + plan baseline (`658090b`) + 직전 cycle synthesis (`9a5a7f6`) — plan §Step 1 expected 와 정합.

---

## 1. 16 회귀 게이트 PASS 재확인 (자동 보존)

코드 변경 0 → 게이트 자동 보존.

- `go vet ./...` clean.
- `TestDecode_Frame0Sample0_MatchesALGTHM` PASS (item 1).
- `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` FAIL — `got=[+2 +2 +2] want=[-1 -1 -1] Δ=3` (item 16 RED 잔존, F-oct-postfix-1 명세대로).
- 본 cycle 신규 측정 (`TestDiagnostic_FnonPrelimXExcitationSubterms`, `TestDiagnostic_FnonPrelimYLPCrossCheck`) Task 1·2 commit 시 PASS 확정 (회귀 무).

---

## 2. spec § PDF verbatim 인용 catalog (§A.4.2.* + §4.2 + §4.3 Table 9)

PDF: `docs/superpowers/specs/itu/G729E.pdf` (ITU-T G.729 06/2012).
추출 수단: `pdftotext -layout`. 행 번호는 추출 텍스트의 1-indexed 줄.

### 2.1 §4.2 Post-processing (PDF p.28, 추출 line 1553–1564) — **본 cycle 의 main-body ground-truth**

> 4.2     Post-processing
> Post-processing consists of three functions: adaptive postfiltering, high-pass filtering and signal upscaling. The adaptive postfilter is the cascade of three filters: a long-term postfilter Hp(z), a short-term postfilter Hf(z) and a tilt compensation filter Ht(z), followed by an adaptive gain control procedure. The postfilter coefficients are updated every 5 ms subframe. The postfiltering process is organized as follows. First, the reconstructed speech ŝ(n) is inverse filtered through Â(z/γn) to produce the residual signal r̂(n). This signal is used to compute the delay T and gain gt of the long-term postfilter Hp(z). The signal r̂(n) is then filtered through the long-term postfilter Hp(z) and the synthesis filter 1/[gf Â(z/γd)]. Finally, the output signal of the synthesis filter 1/[gf Â(z/γd)] is passed through the tilt compensation filter Ht(z) to generate the postfiltered reconstructed speech signal sf(n). Adaptive gain control is then applied to sf(n) to **match the energy of ŝ(n)**. The resulting signal sf′(n) is **high-pass filtered and scaled to produce the output signal of the decoder**.

**핵심 Z-구속 (verbatim 마지막 문장)**: 디코더의 **output signal** = AGC 출력 sf′(n) → HP filter → scale. 즉 **PST = post-HP, post-scale 도메인**.

### 2.2 §4.2.5 High-pass filtering and upscaling (PDF p.29, 추출 line 1687–1693)

> 4.2.5    High-pass filtering and upscaling
> A high-pass filter with a cut-off frequency of 100 Hz is applied to the reconstructed postfiltered speech sf′(n). The filter is given by:
>
>  H_h2(z) = (0.93980581 − 1.8795834 z⁻¹ + 0.93980581 z⁻²) / (1 − 1.9330735 z⁻¹ + 0.93589199 z⁻²)
>
> The filtered signal is **multiplied by a factor 2 to restore the input signal level**.

**핵심 Z-구속**: HP 필터 직후 ×2 scale 이 디코더 출력 chain 의 **종점** — PST 비교 도메인 = ×2 scaled.

### 2.3 §4.3 / Table 9 Encoder and decoder initialization (PDF p.30, 추출 line 1695–1708)

> 4.3      Encoder and decoder initialization
> All static encoder and decoder variables should be initialized to zero, except the variables listed in Table 9.
>
>             Table 9 – Description of parameters with non-zero initialization
>      Variable             Reference          Initial value
>          β                    3.8                 0.8
>          g(–1)                4.2.4               1.0
>          ŵi                   3.2.4              iπ/11
>          qi                   3.2.4              arccos(iπ/11)
>          Û(k)                 3.9.1              –14

**핵심 Z-구속**: AGC g(−1) 만 1.0; 그 외 (HP filter x/y 메모리, postfilter 잔차 메모리, past synthesis) = **zero init**. Frame 0 sf0 시점에서 d.hpX = d.hpY = pst 메모리 = past_exc = 모두 0.

### 2.4 §A.4.2 Post-processing (PDF p.42, 추출 line 2226–2295) — **Annex A 본 cycle ground-truth**

> A.4.2     Post-processing
> The post-processing is the same as described in clause 4.2 except for some simplification in the adaptive postfilter.
> The adaptive postfilter is the cascade of three filters: a long-term postfilter Hp(z), a short-term postfilter Hf(z) and a tilt compensation filter Ht(z), followed by an adaptive gain control procedure.
> The long-term postfilter is simplified by using only integer delays. In the short-term postfilter and the tilt compensation filter, the gain terms gf and gt are not used.
> The postfiltering process is similar to that described in the main body of this Recommendation with the exception that the compensation filtering is performed before synthesis filtering through 1/Â(z/γd).

#### §A.4.2.1 Long-term postfilter (line 2236–2246)

> Hp(z) = 1/(1 + γp gl) · (1 + γp gl z⁻T)    (A.11)
>
> The only difference from clause 4.2.1 is that the long-term delay T is always an integer delay and it is computed by searching the range [Tcl − 3, Tcl + 3], where Tcl is the integer part of the (transmitted) pitch delay in the current subframe bounded by Tcl ≤ 140.

#### §A.4.2.2 Short-term postfilter (line 2249–2262)

> Hf(z) = Â(z/γn) / Â(z/γd) = (1 + Σ γn^i âi z⁻i) / (1 + Σ γd^i âi z⁻i)    (A.12)
>
> γn = 0.55, γd = 0.7. The only difference from clause 4.2.2 is that the gain factor gf is eliminated.

#### §A.4.2.3 Tilt compensation (line 2263–2276)

> Ht(z) = 1 + γt k1′ z⁻¹    (A.13)
>
> k1′ = −rh(1)/rh(0); rh(i) = Σ_{j=0..21−i} hf(j) hf(j+i)    (A.14)
>
> The value of γt = 0.8 is used if k1′ < 0 and γt is set to **zero if k1′ ≥ 0**. The gain factor gt which is used in clause 4.2.3 is eliminated.

#### §A.4.2.4 Adaptive gain control (line 2277–2290)

> Same as described in clause 4.2.4, with the only difference being that the gain scaling factor G for the present subframe is computed by:
>
>  G = √( Σ ŝ²(n) / Σ sf²(n) )    (A.15)
>
> g(n) = 0.9 g(n−1) + 0.1 G,  n = 0,…,39

#### §A.4.2.5 High-pass filtering and upscaling (line 2292–2293)

> **Same as described in clause 4.2.5.**

→ Annex A 의 PST 출력 chain 종점 = §4.2.5 와 동일 = HP filter + ×2 scale.

### 2.5 §2 General description (PDF p.4, 추출 line 384–395) — **sample-rate / frame-size**

> The CS-ACELP coder is based on the code-excited linear prediction (CELP) coding model. The coder operates on speech frames of **10 ms corresponding to 80 samples at a sampling rate of 8 000 samples per second**.

### 2.6 READMETV.txt (Annex A test_vectors, line 8–17 and 21)

> Format: all files contain **16 bit sampled data using the Intel (PC) format**.
>
> *.in  - input files
> *.bit - bit stream files
> *.out - output files
>
> and were obtained using the following commands
>
>  coder file.in file.bit
>  decoder file.bit file.pst
> ITU-T G.729 Software Package Release 2 (November 2006)

> 5600  algthm.bit
> 5600  algthm.pst   ← 5600 byte / 2 byte per sample = 2800 samples = 35 frames × 80 samples

(Main g729 test_vectors/READMETV.txt = 동일 format 문구; PST 정의 동일.)

**핵심 Z-구속**: PST file = `decoder file.bit file.pst` 의 출력 = decoder 의 **최종 출력 stream** (§4.2 / §A.4.2.5 종점) = int16 LE, 80 samples/frame, 8 kHz.

---

## 3. production chain 구조 cross-ref 표

production code 인용 (`grep -n "func "`):

```
internal/postfilter/postfilter.go:23: func (pf *Postfilter) Filter(a, tInt, s, sPf)
internal/postfilter/longterm.go:12 / :104  refinePitch / applyLongTerm
internal/postfilter/shortterm.go:13         applyShortTerm
internal/postfilter/tilt.go:26 / :86        computeTiltMu / applyTiltWithMu
internal/postfilter/agc.go:10 / :49         computeAGCTargetGain / applyAGC
internal/decoder/decode.go:18               Decode (frame-level: 2× decodeSubframe + pcm.ScaleUpSat)
internal/decoder/subframe.go:21             decodeSubframe (per-subframe pipeline)
internal/decoder/hpfilter.go:26             hpFilter
```

`internal/decoder/subframe.go:30–49` 의 호출 순서:

```
pitch.AdaptiveCodebook → fcb.Decode → gn.Decode → synth.BuildExcitation
  → d.syn.Filter (LP synthesis 1/Â(z))
  → d.pst.Filter (long-term Hp → short-term Hf → tilt Ht → AGC; postfilter.go:23–50)
  → d.hpFilter (§4.2.5 HP)
```

`internal/decoder/decode.go:47`: `pcm.ScaleUpSat(out[:frameSamples], out[:frameSamples])` (×2 scale, frame-level after both subframes).

| spec § | spec stage / 정의 | production 구현 | 일치 / 차이 |
|--------|-------------------|-----------------|-------------|
| §A.4.2 chain order | Hp (long-term) → Hf (short-term) → Ht (tilt) → AGC | `postfilter.go:23–49` `Filter` 내 호출 순서: `applyLongTerm → applyShortTerm → applyTiltWithMu → applyAGC` | **일치** |
| §A.4.2.1 Hp | 정수 delay T ∈ [Tcl−3, Tcl+3], Tcl ≤ 140; gl 0..1; Hp(z) = (1 + γp gl z⁻T) / (1 + γp gl) | `longterm.go:12 refinePitch + :58 computeLongTermGain + :104 applyLongTerm` | **일치** (F-oct-prelim/F-sept measurement 정합) |
| §A.4.2.2 Hf | γn = 0.55, γd = 0.7; gf 제거 | `shortterm.go:13 applyShortTerm` + `postfilter.go:5–6` `gammaNumQ15 = 18022 ≈ 0.55`, `gammaDenQ15 = 22938 ≈ 0.70` | **일치** |
| §A.4.2.3 Ht | γt = 0.8 if k1′ < 0; **γt = 0 if k1′ ≥ 0**; gt 제거 | `tilt.go:26 computeTiltMu + :86 applyTiltWithMu` | F-oct-postfix-2 revert (cycle G–O) 후 strict reading 정합 (Δ=0 측정 확인). |
| §A.4.2.4 AGC | G = √(Σŝ² / Σsf²); g(n) = 0.9·g(n−1) + 0.1·G | `agc.go:10 computeAGCTargetGain + :49 applyAGC` (`isqrtQ14`) | **일치** |
| §A.4.2.5 / §4.2.5 HP + ×2 | 100 Hz HP filter (b = +0.93980581, −1.8795834, +0.93980581; a = 1, −1.9330735, +0.93589199); ×2 scale | `hpfilter.go:14–20` Q12/Q13 계수 정합; `decode.go:47 pcm.ScaleUpSat` | **일치** |
| §4.2 parent | post-processing = postfilter + HP + ×2 → decoder output | `subframe.go:42–49` (LP synth → pst → hpFilter) + `decode.go:47` (×2) | **일치** |
| §4.3 Table 9 | g(−1) = 1.0; 그 외 모두 zero init | `Postfilter.Reset()` (g_prev = Q14 16384 = 1.0); decoder hpX/hpY = 0; past_exc = 0; pst 잔차 메모리 = 0 | **일치** (F-oct-prelim-5-3 §3.3 zero-dump cross-ref) |

→ §A.4.2.* + §4.2 + §4.3 Table 9 의 7개 항목 모두 production 정합. **차이 0건**.

---

## 4. PST 비교 도메인 재검토 (READMETV.txt + PDF §4.2 + §A.4.2.5)

| 의문 | spec 답 (verbatim 출처) | 우리 가정과 정합? |
|------|--------------------------|-------------------|
| PST 의 sample-rate? | PDF §2 line 386: "10 ms corresponding to **80 samples at a sampling rate of 8 000 samples per second**" → PST = 8 kHz. | **정합** — 우리 `subframeLen = 40`, 두 subframe = 80 sample/frame, 8 kHz. |
| PST 의 frame size? | PDF §2 line 386 + READMETV.txt: `algthm.pst = 5600 byte` ÷ 2 byte/sample = 2800 sample = 35 frame × 80 sample/frame. | **정합** — 우리 `frameSamples = 80`. |
| PST file format (byte-level)? | READMETV.txt line 11: "16 bit sampled data using the Intel (PC) format" = int16 little-endian. | **정합** — F-oct-postfix2-prelim Task 3 §2 byte-level 정합 입증 (Q-format 일치). |
| PST 출력의 chain 종점? | PDF §4.2 line 1564: "The resulting signal sf′(n) is **high-pass filtered and scaled to produce the output signal of the decoder**." + §A.4.2.5 line 2293: "Same as described in clause 4.2.5." | **정합** — 우리 chain = postfilter → HP → ×2 (`subframe.go:45–48` + `decode.go:47`). PST = post-HP, post-×2 도메인. |
| PST 비교 단위 (sample-by-sample)? | PDF §A.4.2 cascade 종점 = AGC 출력 sf′(n) → §A.4.2.5 HP+×2 → decoder output. 단위 = 단일 16-bit signed sample stream, no decimation, no time-shift, no sub-band split. | **정합** — sample-by-sample int16 비교 가능. |
| Decimation / sub-band / time-shift? | PDF §4.2 + §A.4.2 전체에 decimation / sub-band / time-shift 언급 **無**. §A.4.2.5 = HP + ×2 only. | **정합** — 우리 chain decimation/sub-band/time-shift 0. |
| PST 가 pre-HP 출력일 가능성? | §4.2 line 1564 "The resulting signal sf′(n) is high-pass filtered and scaled **to produce the output signal of the decoder**" — output = post-HP. pre-HP 가능성 spec 부정. | **정합** — 우리 PST 비교 = post-HP 가정 = spec verbatim. |
| PST 가 pre-×2 출력일 가능성? | §4.2.5 line 1693: "The filtered signal is **multiplied by a factor 2 to restore the input signal level**." 즉 ×2 = output 종점 직전. | **정합** — `decode.go:47 pcm.ScaleUpSat` ×2 scale 후 비교 = spec verbatim. |

→ 8개 의문 모두 spec verbatim 답이 우리 가정과 **정합**. 모순 0건. spec 명시 부재 0건.

---

## 5. Z 후보 평가 (반증 / 유력 / 부분)

§3 (production cross-ref 차이 0건) + §4 (PST 비교 도메인 모든 의문 정합) → plan §Step 5 평가표 첫 행 적용:

> spec § verbatim ↔ production cross-ref 모두 일치 + PST 비교 도메인 정합 → **Z 반증** — spec 해석 자체에 결함 0.

### Z 결론

**Z 반증** (Z-confirm 변형: "우리 chain 정합" 가설 입증).

- §A.4.2.* (chain order + Hp/Hf/Ht/AGC stage 정의) — 모두 production 정합.
- §4.2 (post-processing parent — postfilter+HP+×2 종점 정의) — production 정합.
- §4.2.5 / §A.4.2.5 (HP+×2 종점) — production 정합.
- §4.3 Table 9 (state init) — production 정합.
- READMETV.txt (PST file format / 80 sample / 8 kHz) — production 정합.

**우리 현 가정과의 모순**: **0건**. "PST = post-HP, post-×2 출력" 가정이 §4.2 line 1564 + §4.2.5 line 1693 + §A.4.2.5 line 2293 의 verbatim 으로 직접 입증.

→ frame 0 sf0 sample 5..7 의 `got=[+2,+2,+2] vs want=[-1,-1,-1]` Δ=3 결함의 root cause 는 **spec 해석 / chain 종점 / 비교 도메인 측의 결함이 아님** 이 본 task 로 확정. 결함 source 는 Task 1 (X) 가 식별한 excitation u 부호 결정 sub-항 또는 Task 2 (Y) 가 식별한 LP a[] cross-check 결과 중 하나로 한정.

---

## 6. Task 4 (synthesis) 진입 의무

본 task 산출:

1. **Z 반증 확정** — postfilter chain 정합 결함 가설 제거.
2. 잔존 후보: **X (excitation u 부호 source) + Y (LP a[] cross-check)** 2건. Task 4 결정 트리 입력.
3. plan §Task F-non-prelim-4 §2 비교표 작성 시 Z 행은 "**반증 — spec verbatim 정합 입증, 결함 0**" 으로 채움.
4. 결정 트리:
   - Task 1 (X) 가 단일 sub-항 식별 + Task 2 (Y) 도 단일 식별 → 두 식별이 동일 origin 인지 cross-check.
   - Task 1 단일 + Task 2 반증 → X 단독 fix scope.
   - Task 1 반증 + Task 2 단일 → Y 단독 fix scope.
   - Task 1·2 모두 반증 + Z 반증 (본 task) → E3 발동 / W 후보 (PST 출처 / test vector 자체) 재진입.

본 보고서 = Task 4 진입 입력 1/3 완료. Task 1·2 보고서 commit (`f82893d`, `d1a4f2d`) 와 결합 시 Task 4 단일 표 작성 가능.

---

## 부록 A. 회귀 게이트 결과 (commit 직전, 코드 변경 0)

```
$ go vet ./...
(clean)

$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
=== RUN   TestDecode_Frame0Sample0_MatchesALGTHM
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)

$ go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)  ← item 16 RED 잔존 (F-oct-postfix-1 명세대로)
```

→ 게이트 정합 (15 PASS + item 16 RED 잔존). 코드 변경 0 확인.
