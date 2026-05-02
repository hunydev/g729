# Phase 1k Stage F-non-prelim-X-split Diagnostic Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-non-prelim 종합 보고서 (`e867f5e`, `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-synthesis-report.md`) §3.1 = 단일 source `g_c · c[n]` 곱 부호 식별 + §4.1 권고 = "Cα + Cβ hybrid (단일 source 의 두 fix layer 분리 미수행)". 사용자 G-S2 (hybrid 결정 승인) + G-S3 (F-non-prelim-X-split 진입 승인) = "진행". 본 cycle = `g_c · c[n]` 곱 부호의 sub-source 분리 진단 cycle. ALGTHM frame 0 sf0 sample 0..3 의 `g_c=+4153 (Q12)` + `c[0..3]=+8192 (Q13)` → `+33224 (Q15)` → Round → `u[0..3]=+1` 의 양 입력 origin 을 (Cα) `fcb.Decode` 의 c[n] 부호 결정 vs (Cβ) `gain.Decoder.Decode` 의 g_c 부호 결정 두 후보로 분리 측정. **production 변경 0 라인** 진단 cycle. 다음 cycle (F-non-fix-{fcb,gain} 가칭) 의 RED gate = `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` (`56caa72`) 승계.

**Architecture:** 3-task 진단 cycle (TDD 패턴 — failing/측정 test → dump → commit). Task F-non-prelim-X-split-1 = Cα 측정 (`fcb.Decode` 내 sub-stage 분리: 4 pulse positions, 4 signs idx.Signs=0xf, raw c_raw[0..39] = ±PulseAmplitude, β·c[n−t] pitch enhancement 적용 차분 → c[0..3]=+8192 의 origin = raw placement vs enhancement vs spec 정합 측정). Task F-non-prelim-X-split-2 = Cβ 측정 (`gain.Decoder.Decode` 내 sub-stage 분리: GA[5]+GB[6] VQ table entry 직접 lookup, MA predictor (frame 0 = zero predictor state) 영향, 합산 g_c 부호 결정 → g_c=+4153 의 origin = VQ table entry vs predictor/sign-mask vs spec 정합 측정). Task F-non-prelim-X-split-3 = 종합 (Cα/Cβ verdict 결합 + 결정 트리 → 단일 식별→F-non-fix-{fcb,gain} / hybrid 잔존→추가 진단 / 둘 다 spec 정합→Cγ 재진입 또는 Y magnitude follow-up).

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.8 (4-pulse ACELP fixed-codebook) + §3.8.2 (pitch enhancement β·c[n−t]) + §3.9 (gain VQ — GA / GB tables) + §3.9.2 (MA predictor for gain) + §A.3.8 (Annex A fcb decoding) + §A.3.9 (Annex A gain decoding) + §4.1.5 / §4.1.6 (Decoding of fixed/adaptive codebook gains, eq.(75) excitation) + §4.3 Table 9 (state init, including past_err = MIN_GAIN_PRED_DB) + READMETV.txt + 기존 F-quart/F-sext/F-sept/F-oct-prelim/F-oct-prelim-5/F-oct-postfix-1/F-oct-postfix2-prelim/F-non-prelim 진단 하니스 (회귀 게이트 17건). **외부 G.729 구현 0건 참조** (E4) — 사용자 G1 결정 ("(c) Annex A binary 거부") 유지. 이미 repo committed 인 PST 파일 (`testdata/itu/test_vectors/`) 은 입력 stimulus 로 계속 사용 가능. **Annex A binary (예: `coder.exe` / `decoder.exe` / 표준 별첨 ZIP) 사용 금지** — verbatim PDF spec + READMETV.txt 만 ground-truth.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 직전 cycle 의 결정 / 측정 정리 (누적 진단 컨텍스트)

**직전 cycle = F-non-prelim (`e867f5e` synthesis)**:

- F-non-prelim-1 (`f82893d`) — Task 1 (X 측정): `TestDiagnostic_FnonPrelimXExcitationSubterms` 신규. **verdict X-fcb (단일 식별)**. ALGTHM frame 0 sf0 raw 측정:
  - g_p (Q14) = +1995 (≈ +0.122)
  - g_c (Q12) = +4153 (≈ +1.014)
  - v[0..4] (Q0, pitch contribution) = `[0,0,0,0,0]` (zero past_exc, tInt=20, 첫 frame artefact)
  - c[0..4] (Q13, fcb contribution) = `[+8192,+8192,+8192,+8192,0]`
  - g_p · v[0..4] (Q15 pre-Round) = `[0,0,0,0,0]`
  - g_c · c[0..4] (Q15 pre-Round) = `[+33224,+33224,+33224,+33224,0]`
  - u[0..4] (Q0, composite) = `[+1,+1,+1,+1,0]`
  - Round 식 검증: `(33224·2 + 32768) >> 16 = 1` ✓
  - replication match (40 sample) = true
- F-non-prelim-2 (`d1a4f2d`) — Task 2 (Y 측정): `TestDiagnostic_FnonPrelimYLpACheck`. **verdict Y-magnitude (X-fcb 정합)**. a[0..10] sign-equal=11/11 vs §3.2.6 reference, forced a[1..10] sign-flip → syn[5..7] 부호 flip 0/3, magnitude만 변화 (+1 → 0). max\|Δ\|=6 (a[1]: −2197 vs −2203) magnitude gap 잔존, 부호 source 와 직교 — 본 cycle 외 처리 의무 인계.
- F-non-prelim-3 (`dd4e21a`) — Task 3 (Z 측정 보고서 only): spec § verbatim catalog (§A.4.2.* + §4.2 + §4.2.5 + §4.3 Table 9 + READMETV.txt). **verdict Z-confirm (반증)**. chain order 7항 production 정합 7/7, PST 비교 도메인 8 의문 spec verbatim 정합 8/8. PST = post-AGC + post-HP + post-×2 입증.
- F-non-prelim-4 (`e867f5e`) — Task 4 (synthesis): 단일 source = fcb codebook c[0..3] 의 양 입력 (= `g_c · c[n]` 곱) 식별. 4 후보 (Cα/Cβ/Cγ/Cδ) 평가:
  - Cα `fcb.Decode` (c[n] 부호 결정) — HIGH priority, MID risk, 측정 미수행 (sub-decomposition 한계)
  - Cβ `gain.Decoder.Decode` (g_c 부호 결정) — HIGH priority, MID risk, ROM cross-check 미수행 (한계)
  - Cγ spec 정합 (추가 mechanism) — LOW priority (Task 2/3 측정으로 dismiss 정당)
  - Cδ stimulus 의문 — DISMISSED (M6 `cb9529d` byte-level + 9 vector 분포 반증)
  - 권고 = **Cα + Cβ hybrid 진단 cycle (= F-non-prelim-X-split, 본 plan)**. 사용자 G-S2 = hybrid 승인, G-S3 = 본 cycle 진입 승인.

**누적 측정 사실 (본 cycle 진입 premise)**:

| 사실 | 출처 | 비고 |
|------|------|------|
| ALGTHM frame 0 sf0 sample 0..3 production u = `+1` (Q0, sample-uniform) | `f82893d` §2 | X-fcb verdict 의 최종 출력 |
| `g_c · c[n]` (Q15 pre-Round) = `+33224` for n=0..3 (sample-uniform) | `f82893d` §2 | 곱 부호 origin 측정 대상 |
| g_p · v[n] (Q15 pre-Round) = `0` for n=0..4 | `f82893d` §2 | pitch contribution 부호 source 아님 (X-pitch 자동 반증) |
| Round 식: `(33224·2 + 32768) >> 16 = 1` ✓ | `f82893d` §2.1 | spec eq.(75) (§4.1.6) 정합 |
| g_c (Q12) = +4153 single int16 | `f82893d` §2 | sub-source 미상 (VQ table vs predictor) |
| c[0..3] (Q13) = +8192 sample-uniform | `f82893d` §2 | sub-source 미상 (raw placement vs β enhancement) |
| pitch lag tInt = 20 (sf0) | `f82893d` §2 | β·c[n−t] enhancement 는 n≥20 영역만 영향 (sample 0..3 영역 enhancement off — 추정, Task 1 §2 측정 의무) |
| GA1 = 5, GB1 = 6 (gain VQ indices, ALGTHM frame 0 sf0) | `f82893d` §2 + spec §3.9 | Task 2 sub-stage 분리 baseline |
| frame 0 = codec-start, MA predictor past_err state = MIN_GAIN_PRED_DB (§4.3 Table 9) | spec §4.3 Table 9 | predictor 효과 zero-state |
| F-sept-2 L3 magnitude gap (max\|Δ\|=6) 잔존 | `d1a4f2d` §3.1 | 부호 source 와 직교, 본 cycle 외 |
| 합성 결함 (item 17 RED) | got=`[+2,+2,+2]`, want=`[−1,−1,−1]`, Δ=3 | `56caa72` (F-oct-postfix-1) — 다음 fix cycle GREEN gate |

**핵심 추론** (synthesis §3.1 + §4.1): `g_c · c[n]` 곱이 양수 = (a) c[n] 양수 × g_c 양수 (둘 다 spec 정합, mismatch 외부) 또는 (b) c[n] 결함 양수 × g_c 정합 양수 (= Cα 단독) 또는 (c) c[n] 정합 양수 × g_c 결함 양수 (= Cβ 단독) 또는 (d) 둘 다 결함 (hybrid). 본 cycle = Cα 의 c[n] sub-source 측정 (raw pulse placement / sign 처리 / β enhancement) + Cβ 의 g_c sub-source 측정 (VQ table entry / MA predictor / sign 처리) 으로 (a)~(d) 식별. 단일 식별 시 다음 fix cycle scope (1~3 라인) 자동 결정.

### Phase 0.2 invariant 재확인 (E1-E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.3 의 1~16 PASS 항목, 단 항목 17 = postfix-1 RED 는 *예외 — 의도된 RED 잔존*) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | 본 cycle 의 임의 task 의 spec § 인용이 PDF verbatim grep 결과와 불일치 (= 휴리스틱 fit) | 즉시 측정 폐기 + spec § PDF 직접 재발췌 + 보고서 §0 에 도출 과정 정량 기록 |
| **E3** | 본 cycle Task 3 종합에서 Cα/Cβ 중 2+ 가 잔존 (단일 식별 불가, hybrid 잔존) | Task 3 §4 다음 cycle 권고 = "추가 진단 cycle 또는 hybrid fix cycle (Cα+Cβ 동시 fix)". 단일 fix cycle 진입 강요 금지 (Phase 0.4 §1 — 측정 데이터 기반). |
| **E4** | 외부 G.729 구현 (ITU 참조 C / Annex A binary / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조/실행 흔적 발견. **사용자 G1 결정 = Annex A binary 거부** (black-box 행동 추적 포함). | 즉시 작업 중단 + 사용자 통보 + 해당 인용/binary 제거 후 재시작 |
| **E5** | 본 cycle 의 production 변경 라인 > 0 (메타 의무 — 진단 cycle) | 즉시 `git revert HEAD` + commit 재구성 (production 변경 제거) |

### Phase 0.3 회귀 게이트 명세 (17건 — 누적 contract test 16 + F-non-prelim Task 1 의 X measurement promotion 1)

각 task commit 직후 *반드시* 실행 (총 17 게이트 — F-non-prelim 종결 시점의 16 PASS + 항목 17 = F-oct-postfix-1 RED 의무; 본 cycle Task 1/2 의 신규 측정 harness 는 *측정-only* — 자동 promotion 하지 *않음*, Task 3 §3 의 "잔여 보류 항목" 으로 처리):

| # | 그룹 | test | 의무 |
|---|------|------|------|
| 1 | F-quart | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 2 | F-quart | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 3 | F-sext | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 4 | F-sept | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 5 | F-sept | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 6 | F-sept | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 7 | F-oct-prelim | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 8 | F-oct-prelim | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |
| 9 | F-oct-prelim | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS |
| 10 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` | PASS |
| 11 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5BitVectorCompare` | PASS |
| 12 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5HpFilterInitState` | PASS |
| 13 | F-oct-prelim-5 | `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` | PASS |
| 14 | ITU contract | `TestDecode_Frame0Sample0_MatchesALGTHM` (Phase 1i sample 0 invariant) | PASS |
| 15 | F-oct-postfix2-prelim | `TestDiagnostic_FoctPostfix2PrelimChainDump` (+ `M5ExcitationSignTrace`, `M6PSTSignVerify`, `M3IIRMemory` 동상 PASS) | PASS |
| 16 | F-non-prelim | `TestDiagnostic_FnonPrelimXExcitationSubterms` + `TestDiagnostic_FnonPrelimYLpACheck` (X + Y measurement 묶음, 직전 cycle 신규) | PASS |
| 17 | F-oct-postfix-1 RED | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **RED 잔존 의무** (다음 fix cycle 의 GREEN gate) |

추가 sanity:
- `go vet ./...` clean (각 task commit 직후).
- 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 본 cycle 진입 시점 FAIL 유지. 본 cycle 어떤 task 도 본 3건의 상태를 변화시키지 *않아야* 함 (production 변경 0 라인 의무로 자동 보장).

본 cycle Task 1/2 의 신규 측정 harness (`TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace`, `TestDiagnostic_FnonPrelimXSplit2GainGcTrace`) 는 *측정-only* (assertion 0 또는 PASS 의무) — 회귀 게이트 18/19번째 항목으로 자동 promotion 하지 *않는다* (자동 promotion 금지, F-non-prelim §0.3 동상 패턴). Task 3 §3 잔여 보류 항목으로 처리.

### Phase 0.4 강압-적합 회피 의무 (forced-fit avoidance)

본 cycle 은 *진단 cycle* 이며 직전 cycle (단일 source 식별 + Cα/Cβ hybrid 잔존) 의 측정 데이터를 누적 ground-truth 로 사용. Phase 0.4 의무가 production fix cycle 보다 *더 엄격*. 다음 패턴을 적극 회피:

1. **Cα vs Cβ 임의 우선 결정 금지**: Task 1 (Cα — `fcb.Decode` sub-stage) 와 Task 2 (Cβ — `gain.Decoder.Decode` sub-stage) 의 측정 결과가 도출되기 전 임의 우선 결정 금지. 두 task 모두 raw 값 + 부호 + Q-format + ROM/spec cross-ref 를 dump 후 측정 데이터로만 부호 결정 sub-source 식별. spec 인용 또는 직관적 "fcb 가 가장 가능성 높음" 식의 추론 금지. Cα 와 Cβ 는 직전 cycle synthesis §3.2 에서 둘 다 priority HIGH / risk MID — 우선 순위 차이 없음.
2. **spec § 인용 우회 fit 금지**: Task 1 (§3.8 + §3.8.2 + §A.3.8 + §4.1.5/6) / Task 2 (§3.9 + §3.9.2 + §A.3.9 + §4.1.5/6 + §4.3 Table 9) 의 spec 인용은 모두 PDF `pdftotext -layout` verbatim grep 으로 채택. 결합 해석 또는 *간접 증거* 는 보고서 §0 에서 명시. F-non-prelim Task 1 §0.2 의 "§A.3.5 → §4.1.5/6 정정" 패턴 (plan 추정 인용과 PDF grep 불일치 시 즉시 정정) 답습.
3. **음성 결과 (Cα/Cβ 둘 다 spec 정합) 도 결과로 인정**: 본 cycle 측정 결과가 "c[n] 도 spec 정합 + g_c 도 spec 정합" (= Cα/Cβ 둘 다 반증) 일 경우도 *유효한 측정 결과*. Task 3 §3 결정 트리에 따라 Cγ 재진입 (synthesis §3.4 Cγ LOW priority dismiss 의 재고) 또는 Y magnitude follow-up cycle (max\|Δ\|=6) 의 다음 cycle 권고 자동 도출.
4. **scope crawl 금지**: 본 cycle 모든 task 의 production 변경 = 0 라인. test 변경 = 측정 harness 신규/추가만. helper 신규 0 (기존 `decoder` / `synth` / `gain` / `fcb` / `pitch` package helper 재사용). spec 인용은 §3.8.* + §3.9.* + §A.3.8/9 + §4.1.5/6 + §4.3 Table 9 영역 한정.
5. **g_l 영속화 후보 ① 영구 제외**: 사용자 G1 결정 = "후보 ③ pivot" — 본 cycle 도 ① (g_l state 영속화 + tilt.go read) 와 관련된 측정/fix 일체 도입 금지. (본 cycle 은 g_l 보다 상류 — fcb/gain decoding sub-stage — 측정이므로 자연 회피.)
6. **hybrid 결정 강요 금지**: Task 3 §3 결정 트리에서 측정 결과가 (Cα 단독 결함) 또는 (Cβ 단독 결함) 또는 (둘 다 결함 = hybrid) 또는 (둘 다 정합) 모두 명시 — *측정 데이터에 의해 자동 결정*. 직전 cycle synthesis §4.1 의 "hybrid 권고" 가 본 cycle 에서 *선험적으로* hybrid 결정을 강요하지 않음. Cα 또는 Cβ 단독 식별 시 hybrid 권고를 *측정 우선* 으로 재고.
7. **Cδ 재진입 절대 금지**: M6 (`cb9529d`) byte-level + 9 vector 분포 반증 + synthesis §3.3 Cδ DISMISSED 결정 → 본 cycle 어떤 측정도 Cδ (PST want / test vector) 재진입 트리거 금지. 본 cycle Cα/Cβ 둘 다 정합 시에도 Cγ 재진입 또는 Y follow-up 만 권고 (Phase 0.4 §3 정합).

### Phase 0.5 사전 보유 working tree 보존 의무

`internal/decoder/stagef_bis_diagnostic_test.go` (untracked, F-bis baseline 잔존) 는 본 cycle 3 task 어떤 commit 도 add 하지 않는다. 사후 working tree 의 `?? internal/decoder/stagef_bis_diagnostic_test.go` 가 F-non-prelim synthesis 시점 (`e867f5e`) 과 동일하게 유지됨을 각 task §0 보고서에서 확인.

---

## Spec § 인용 (본 cycle 의 ground-truth 공통)

각 task §0 에서 PDF `pdftotext -layout` verbatim grep 으로 재확인 의무. 본 §은 plan 작성 시점의 추정 인용 — task 실행 시점 grep 결과와 불일치 시 즉시 E2 발동.

**(인용 1)** ITU-T G.729 (06/2012) PDF §3.8 *Fixed codebook: structure and search* (Cα 의 직접 spec ground-truth):
- 4-pulse ACELP codebook 구조: 4 tracks × 8 positions (i0/i1/i2 ∈ {0,5,10,...,35}, i3 ∈ {3,8,...,38}). 각 pulse 의 부호 = ±1 (sign mask, idx.Signs).
- spec 정의: `c[n] = Σ_k s_k · δ(n − m_k)` (m_k = pulse position, s_k = ±1 sign). Q13 amplitude = ±8192.
- Task 1 진입 시 PDF verbatim grep + 4 pulse position / sign 매핑 확인.

**(인용 2)** ITU-T G.729 (06/2012) PDF §3.8.2 *Pre-emphasis / pitch sharpening filter* (Cα 의 β·c[n−t] enhancement ground-truth):
- spec 정의: `c[n] = c_raw[n] + β · c[n − T]` for n ≥ T (T = pitch lag, β ≈ Q14 3277 ≈ 0.2).
- 첫 frame sf0 에서 tInt=20 → enhancement 는 n ≥ 20 영역만 영향. sample 0..3 (n < 20) → enhancement off (raw c_raw[n] 만).
- Task 1 진입 시 PDF verbatim grep + β / T 영향 영역 cross-ref.

**(인용 3)** ITU-T G.729 (06/2012) PDF §3.9 *Quantization of the gains* (Cβ 의 직접 spec ground-truth):
- gain VQ: 2-stage (GA 8 entries × 5-dim, GB 16 entries × 5-dim — 또는 spec verbatim 수치 grep 확인). g_p / g_c correction factor (γ̂) lookup.
- spec 정의: `g_c = γ̂ · g_c̃` (g_c̃ = predicted gain, γ̂ = VQ correction).
- Task 2 진입 시 PDF verbatim grep + GA/GB table dimension 확인.

**(인용 4)** ITU-T G.729 (06/2012) PDF §3.9.2 *Gain prediction (MA predictor)* (Cβ 의 g_c̃ predictor ground-truth):
- spec 정의: `Ē[n] = Σ_i b_i · U[n−i]` (4-tap MA predictor on past quantized prediction error).
- frame 0 (codec-start) → past_err state 초기화 (§4.3 Table 9 = MIN_GAIN_PRED_DB ≈ −14 dB).
- Task 2 진입 시 PDF verbatim grep + Table 9 init value 확인.

**(인용 5)** ITU-T G.729 (06/2012) PDF §A.3.8 *Decoding of fixed-codebook vector* + §A.3.9 *Decoding of gains* (Annex A 8 kbps decoder side cross-ref):
- §A.3.8 = encoder §3.8 의 decoder side (codeword → pulse positions/signs → c[n]).
- §A.3.9 = encoder §3.9 의 decoder side (codeword GA/GB → γ̂ → g_c).
- Task 1 / Task 2 모두 진입 시 PDF verbatim grep.

**(인용 6)** ITU-T G.729 (06/2012) PDF §4.1.5 *Decoding of the adaptive and fixed-codebook gains* + §4.1.6 *Computation of the reconstructed speech* eq.(75) (X-fcb verdict 의 곱 부호 합성 spec):
- eq.(75): `u[n] = Round( g_p · v[n] + g_c · c[n] )` (Q15 → Q0 round).
- F-non-prelim Task 1 §0.2 정정 인용 — `§A.3.5` (plan 추정) 가 PDF grep 결과 §4.1.5/6 으로 정정. 본 cycle 도 동일 정정 패턴 답습.

**(인용 7)** ITU-T G.729 (06/2012) PDF §4.3 Table 9 *Initial state values* (Cβ predictor zero-state ground-truth + Cα past_exc=0 cross-ref):
- past_err[i] = MIN_GAIN_PRED_DB (i = 0..3) — gain MA predictor init.
- past_exc[n] = 0 (n = 0..L_FRAME+L_INTERPOL) — adaptive codebook init.
- Task 1/2 모두 인용.

**(인용 8)** READMETV.txt: PST 파일 format = Intel PC = 16-bit signed little-endian, frame = 80 sample (M6 cross-ref ground-truth, Cδ 영구 dismiss 정합).

각 task 는 본 §의 인용을 baseline 으로 채택. 추가 spec 인용 시 해당 task §0 에 PDF page + verbatim 추가.

---

## Task F-non-prelim-X-split-1: Cα 측정 — `fcb.Decode` c[n] 부호 sub-stage 분리

**Goal:** **후보 Cα** = ALGTHM frame 0 sf0 의 fcb codebook 출력 c[0..3]=+8192 (Q13) 의 부호 결정 sub-stage 식별. spec §3.8 (4-pulse ACELP structure) + §3.8.2 (β·c[n−t] pitch enhancement) + §A.3.8 (Annex A decoder side) 에 따라 sub-stage 4개 (idx.Positions / idx.Signs / raw pulse placement c_raw[0..39] / β·c[n−t] 적용 후 c[0..39]) 를 sample 0..3 한정으로 분리 dump. c[0..3]=+8192 의 origin 이 (a) raw pulse placement (idx.Signs=0xf → 4-pulse 모두 + 부호) 인지 (b) β enhancement 누적 (단, tInt=20 → n=0..3 영역 영향 0 추정 — 측정으로 입증) 인지 (c) spec 정합 (codeword 자체가 spec-canonical 양 부호 출력) 인지 식별. 단일 sub-stage 식별 시 다음 fix cycle (F-non-fix-fcb) 의 production fix scope 자동 결정.

**Files:**
- Create: `internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go`
- Create: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-1-report.md`

production 변경 0 라인. test 변경 = 신규 1 파일.

### Spec § 인용

본 plan 상단 §"Spec § 인용" 인용 1 (§3.8) + 인용 2 (§3.8.2) + 인용 5 (§A.3.8) + 인용 6 (§4.1.5/6) + 인용 7 (§4.3 Table 9 past_exc=0). Task 진입 시 PDF verbatim grep 으로 확정.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
e867f5e docs(plans): F-non-prelim synthesis + cycle decision
```

(본 plan commit 후에는 plan commit 이 HEAD; X-split-1 진입 시점에는 plan commit 이 HEAD.)

기타 파일 modified 잔존 시 즉시 사용자 통보.

Run (회귀 게이트 baseline, Phase 0.3 의 16 PASS 항목 + 항목 17 RED 잔존):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5|OctPostfix2Prelim|nonPrelim)" -v
go test ./internal/postfilter/ ./internal/synth/ -v -run Contract
go vet ./...
```

Expected: 16 PASS + 항목 17 RED + `go vet` clean. 출력 요약을 보고서 §1 에 인용.

- [ ] **Step 2: spec § PDF verbatim grep**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 50 "3.8 Fixed"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "3.8.2"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.8"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 20 "Table 9"
```

verbatim 인용 (특히 §3.8 의 4-pulse position/sign 매핑 + §3.8.2 의 β·c[n−t] enhancement 식 + n≥T 조건) 을 보고서 §0 spec 인용 섹션에 기록. 인용 grep 결과와 본 plan §"Spec § 인용" 추정이 불일치 시 즉시 E2 발동 + 정정 (F-non-prelim Task 1 §0.2 의 §A.3.5→§4.1.5/6 정정 패턴 답습).

- [ ] **Step 3: production code 의 fcb sub-stage 측정 점 식별**

기존 production code 에서 sub-stage 측정 점 식별:

```
grep -n "func \|Positions\|Signs\|Pulse" internal/fcb/decode.go internal/fcb/types.go 2>/dev/null
grep -n "fcb\." internal/decoder/subframe.go
```

dump 대상 sub-stage (frame 0 sf0 sample 0..39 한정, 분석 focus = sample 0..3):

| sub-stage | 측정 점 | dump 형식 | spec § |
|-----------|---------|-----------|--------|
| (a) idx.Positions (4 pulse positions) | `fcb.Indices.Positions` (codeword 디코딩 결과) | hex int + decoded {m0,m1,m2,m3} | §3.8 / §A.3.8 |
| (b) idx.Signs (4-pulse sign mask) | `fcb.Indices.Signs` (4-bit mask, expected 0xf) | hex int + decoded {s0,s1,s2,s3} ∈ {+1,−1} | §3.8 / §A.3.8 |
| (c) c_raw[0..39] (raw pulse placement, 4 nonzero entries) | `fcb.Decode` 의 β=0 stub 호출 (또는 동일 path 복제 with β=0) | int16[40] + nonzero indices + sign | §3.8 |
| (d) β·c[n−t] enhancement 적용 후 c[0..39] | `fcb.Decode` 의 production 호출 (β=Q14 3277, t=20) | int16[40] + enhancement Δ = c − c_raw | §3.8.2 |
| (e) c[0..3] sample-uniform check | (d) 의 sample 0..3 verbatim | `+8192` 확인 (X-fcb 합치) | (cross-check) |

**측정 점이 production API 로 노출되지 않는 경우** (예: c_raw 가 중간 buffer 만): test 가 `fcb.Decode` 를 *β=0 stub* 인자로 호출하여 raw placement 만 추출 (spec §3.8.2 의 β=0 시 enhancement off 정합) + production β=Q14 3277 호출의 차분으로 enhancement 효과 분리. 두 호출 의 동치성은 c[0..3]=+8192 일치 + Δ[0..3] = 0 (enhancement off 영역) 로 검증 (replicated chain == production 의무, F-non-prelim Task 1 §2.2 의 replication match 패턴 답습).

- [ ] **Step 4: dump harness test 작성 — `stagef_fnonprelim_xsplit_diagnostic_test.go`**

`internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go` 신규 작성 (구체 outline):

```go
package decoder

import "testing"

// TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace decomposes the ALGTHM
// frame 0 sf0 fcb codebook output c[0..3] = +8192 (Q13) into spec §3.8
// + §3.8.2 sub-stages: (a) idx.Positions (4-pulse positions), (b) idx.Signs
// (4-bit sign mask, expected 0xf), (c) raw pulse placement c_raw[0..39]
// (β=0 stub), (d) production β·c[n−t] enhancement (β=Q14 3277, t=20).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §3.8 (4-pulse ACELP) +
//   §3.8.2 (pitch enhancement c[n] = c_raw[n] + β·c[n−T] for n ≥ T)
// + §A.3.8 (Annex A decoder side fcb decoding) + §4.1.5/6 eq.(75).
//
// F-non-prelim synthesis (e867f5e) §3.1 identifies single source =
// `g_c · c[n]` product positive. §4.1 recommends Cα + Cβ hybrid split.
// This test = Cα half (c[n] sub-source identification).
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace(t *testing.T) {
	// 1) decode ALGTHM frame 0 → reach subframe 0 dispatch
	// 2) intercept fcb.Indices (Positions, Signs)
	// 3) call fcb.Decode with β=0 stub → record c_raw[0..39]
	// 4) call fcb.Decode with production β=Q14 3277, t=20 → record c[0..39]
	// 5) compute Δ[0..39] = c − c_raw → record enhancement contribution
	// 6) cross-check c[0..3] = +8192 (X-fcb verdict 합치)
	// 7) t.Logf each sub-stage raw value + sign + Q-format
}
```

helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0. fcb.Decode 호출은 production exported API (`fcb.Decode(idx, t, betaQ14, c)` signature 답습).

측정 출력 형식 (예시):
```
[Cα idx.Positions]   raw=<hex>  decoded m=[<m0>,<m1>,<m2>,<m3>]  tracks={i0:0/5/.../35, i1:..., i2:..., i3:3/8/.../38}
[Cα idx.Signs]       raw=<hex>  expected=0xf  decoded s=[<s0>,<s1>,<s2>,<s3>]  ∈{+1,−1}
[Cα c_raw[0..39]]    nonzero@m=[<m0>:+8192, <m1>:..., <m2>:..., <m3>:...]  zeros elsewhere
[Cα c[0..39] (β=Q14 3277, t=20)]  full int16[40] dump or nonzero indices
[Cα Δ[0..39] = c − c_raw]   nonzero@n≥20 (enhancement off for n<20 expected)
[Cα c[0..3] sample-uniform]  c=[+8192,+8192,+8192,+8192]  X-fcb-match=<true|false>
[Cα 결정] sign-determining sub-stage = <raw-placement | β-enhancement | spec-canonical | hybrid | undetermined>
[Cα verdict] Cα = <단독 결함 | spec 정합 | 부분 결함>
```

- [ ] **Step 5: 측정 + Cα sub-stage 부호 결정성 평가**

Run:
```
go build ./...
go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace -v
```

Expected: build PASS, test PASS, t.Logf 출력에 sub-stage별 raw 값 + 부호. 출력 verbatim 을 보고서 §2 에 인용.

Cα sub-stage 평가 (보고서 §3, Phase 0.4 §1 — 측정 데이터만):

| sub-stage 측정 결과 | Cα 평가 |
|--------------------|---------|
| idx.Signs=0xf (4-pulse 모두 +) + idx.Positions 중 m_k ∈ {0,1,2,3} (sample 0..3 영역) + c_raw[0..3]=+8192 + Δ[0..3]=0 → raw placement 단독 양수 결정 | **raw placement 단독 결정** — c[n] 양 부호 = 4-pulse sign 0xf decoding 결과 (정합 또는 결함은 §3.8 sign decoding spec verbatim cross-ref 로 판단) |
| idx.Signs ≠ 0xf (혼합) + c_raw[0..3] 중 일부 음수 + Δ[0..3] 누적으로 c[0..3]=+8192 양수화 | **β-enhancement 결정** — 단, sample 0..3 영역 enhancement off (n<T=20) 추정과 모순 → spec §3.8.2 verbatim 재확인 + 결함 가능성 |
| idx.Signs=0xf + idx.Positions 중 m_k ∉ {0,1,2,3} → c_raw[0..3] = 0 → enhancement 만 c[0..3]=+8192 | **spec 위반** — sample 0..3 영역에 raw pulse 부재인데 +8192 출력 = §3.8.2 enhancement off 위반 (= Cα 결함) |
| c[0..3] ≠ +8192 (replicated path 와 production path 불일치) | **replication 결함** — 복제 path 검토 의무 + 보고서 §0 에 한계 명시 |
| idx.Signs=0xf + raw placement 정합 + spec §3.8 sign decoding rule 정합 | **Cα 반증 (spec 정합)** — c[n] 양 부호는 spec-canonical, 결함 없음 → Cβ 단독 fix scope 후보 강화 |

- [ ] **Step 6: 17 회귀 게이트 재확인**

Run: 16 PASS + 항목 17 RED + 신규 측정 `TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace` PASS.

1+ FAIL 시 (단 항목 17 제외) E1 발동.

- [ ] **Step 7: 보고서 작성**

`docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-1-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-X-split-1 보고서 — Cα fcb c[n] sub-stage 분리

**작성일**: 2026-05-04
**범위**: 후보 Cα (`fcb.Decode` 의 c[0..3]=+8192 부호 결정) sub-stage (idx.Positions / idx.Signs / raw placement / β enhancement) 분리 측정.
**산출물**: 측정 함수 1 신규 파일 + sub-stage별 raw + 부호 결정성 평가 + Cα verdict (단독 결함 / spec 정합 / 부분 결함).
**준수**: production 변경 0, 외부 G.729 0 참조 (Annex A binary 거부 — G1 결정 정합), F-non-prelim Task 1 의 g_c·c 곱 측정 baseline 인계.

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G-S2/G-S3 결정 정합성
## 1. 회귀 게이트 baseline (16 PASS + 항목 17 RED + 신규 PASS)
## 2. sub-stage raw 출력 (sample 0..39, focus 0..3)
## 3. Cα sub-stage 부호 결정성 평가 (raw placement / β enhancement / spec 정합 / replication 결함)
## 4. F-non-prelim Task 1 §2 의 c[0..4] dump 와의 cross-check
## 5. Task 2 (Cβ) 진입 의무 항목
```

- [ ] **Step 8: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go
?? docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-1-report.md
```

```bash
git add internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go \
        docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-non-prelim-X-split-1 Cα fcb pulse trace

F-non-prelim 종합 (e867f5e) §3.1 단일 source = g_c·c[n] 곱 양수 +
§4.1 Cα+Cβ hybrid 권고 — 사용자 G-S2 hybrid 승인 + G-S3 진입 승인.
본 task = Cα 절반 측정. §3.8 (4-pulse ACELP) + §3.8.2 (β·c[n−T] pitch
enhancement) + §A.3.8 에 따라 ALGTHM frame 0 sf0 c[0..3]=+8192 (Q13)
의 sub-stage (idx.Positions / idx.Signs=0xf / raw placement c_raw /
β enhancement Δ) 분리 dump. raw placement (sign 0xf) 단독 / β
enhancement 누적 / spec-canonical 식별로 다음 fix cycle scope 결정.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조
(Annex A binary 사용 금지 — G1 결정).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-non-prelim-X-split-2: Cβ 측정 — `gain.Decoder.Decode` g_c sub-stage 분리

**Goal:** **후보 Cβ** = ALGTHM frame 0 sf0 의 gain VQ 출력 g_c=+4153 (Q12) 의 부호 결정 sub-stage 식별. spec §3.9 (gain VQ — GA / GB tables) + §3.9.2 (MA predictor) + §A.3.9 (Annex A decoder side) + §4.3 Table 9 (past_err init = MIN_GAIN_PRED_DB) 에 따라 sub-stage 3개 (GA[5]+GB[6] VQ table entry 직접 lookup, MA predictor (frame 0 = zero predictor state) 영향, 합산 g_c 부호 결정 산출) 를 분리 dump. g_c=+4153 의 origin 이 (a) VQ table entry 자체 (ROM cross-check vs PDF Table) 인지 (b) MA predictor 효과 (frame 0 zero-state) 인지 (c) sign 처리 / γ̂ 결합 산출 로직 인지 식별. 단일 sub-stage 식별 시 다음 fix cycle (F-non-fix-gain) 의 production fix scope 자동 결정.

**Files:**
- Modify: `internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go` (Cβ 측정 함수 1개 추가, 기존 Cα test 옆)
- Create: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-2-report.md`

production 변경 0 라인. test 변경 = 기존 파일에 함수 1 추가.

### Spec § 인용

본 plan 상단 §"Spec § 인용" 인용 3 (§3.9) + 인용 4 (§3.9.2) + 인용 5 (§A.3.9) + 인용 6 (§4.1.5/6) + 인용 7 (§4.3 Table 9 past_err). Task 진입 시 PDF verbatim grep.

- [ ] **Step 1: 사전 조건 + Task 1 commit hash 인용**

Run: `git log --oneline -3`

Expected: Task 1 commit + plan commit + `e867f5e`.

- [ ] **Step 2: spec § PDF verbatim grep**

Run:
```
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 40 "3.9 Quantization of the gains"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "3.9.2"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 30 "A.3.9"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 40 "4.1.5"
pdftotext -layout docs/superpowers/specs/itu/G729E.pdf - | grep -B1 -A 20 "Table 9"
```

verbatim 인용 (특히 §3.9 의 GA/GB table dimension + §3.9.2 의 4-tap MA predictor 식 + §4.3 Table 9 의 past_err init 값) 을 보고서 §0 에 기록. 인용 grep 결과와 본 plan §"Spec § 인용" 추정이 불일치 시 즉시 E2 발동 + 정정 (F-non-prelim Task 1 §0.2 패턴 답습).

- [ ] **Step 3: Cβ 측정 함수 추가 — `TestDiagnostic_FnonPrelimXSplit2GainGcTrace`**

기존 `stagef_fnonprelim_xsplit_diagnostic_test.go` 에 함수 1 추가. dump 대상 sub-stage:

| sub-stage | 측정 점 | dump 형식 | spec § |
|-----------|---------|-----------|--------|
| (a) idx.GA1=5, idx.GB1=6 (gain VQ indices) | `gain.Indices.GA1, GB1` (codeword) | int + decoded entry | §3.9 / §A.3.9 |
| (b) GA[5] table entry | production ROM lookup (예: `internal/gain/tables.go` GA array index 5) | (g_p_ga, g_c_ga) 또는 spec-canonical correction 쌍 | §3.9 (PDF Table) |
| (c) GB[6] table entry | production ROM lookup (예: `internal/gain/tables.go` GB array index 6) | (g_p_gb, g_c_gb) 또는 spec-canonical correction 쌍 | §3.9 (PDF Table) |
| (d) γ̂ correction factor (= GA[5]+GB[6] 합산 또는 spec식) | production 합산 결과 | int16 + Q-format | §3.9 / §4.1.5 |
| (e) g_c̃ predicted gain (MA predictor output) | `gain.Decoder` 내 predictor 결과 (frame 0 → past_err = MIN_GAIN_PRED_DB) | int16 / int32 + Q-format | §3.9.2 / §4.3 Table 9 |
| (f) 합산 g_c (Q12) = γ̂ · g_c̃ (또는 spec eq.) | `gain.Decoder.Decode` 반환값 | int16=+4153 (Q12) confirm | §4.1.5 / §4.1.6 |
| (g) sample 0..3 cross-check | (f) × c[n] (Task 1 측정값) → Q15 → Round → +1 | u[0..3]=+1 합치 | §4.1.6 eq.(75) |

helper 신규 0 — gain.Decoder.Decode + production ROM table 직접 access. (production export 미상 시 white-box test 로 same-package 직접 호출.)

측정 출력 형식 (예시):
```
[Cβ idx.GA1, GB1]   GA1=5, GB1=6  (codeword)
[Cβ GA[5] entry]    (g_p_ga=<int>, g_c_ga=<int>)  Q-format=<spec>
[Cβ GB[6] entry]    (g_p_gb=<int>, g_c_gb=<int>)  Q-format=<spec>
[Cβ γ̂ correction]   computed=<int16>  spec-eq-cross-ref=<value>  match=<true|false>
[Cβ MA predictor]   past_err=[MIN_GAIN_PRED_DB×4]  g_c̃ predicted=<int>  Q-format=<spec>
[Cβ g_c (Q12)]      computed=+4153  X-fcb-match=true  spec-canonical=<true|false>
[Cβ ROM cross-ref]  GA[5]+GB[6] vs PDF §3.9 Table verbatim entry  diff=<...>
[Cβ 결정] sign-determining sub-stage = <VQ-table | predictor | sign-mask/processing | hybrid | spec-canonical>
[Cβ verdict] Cβ = <단독 결함 | spec 정합 | 부분 결함>
```

- [ ] **Step 4: 측정 + Cβ sub-stage 부호 결정성 평가**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimXSplit2GainGcTrace -v`

Expected: PASS, t.Logf 출력 verbatim 을 보고서 §2 에 인용.

Cβ sub-stage 평가 (보고서 §3):

| 측정 결과 | Cβ 평가 |
|-----------|---------|
| GA[5]+GB[6] ROM entry = PDF §3.9 Table verbatim 일치 + g_c̃ predictor 정합 + 합산 g_c=+4153 spec-canonical 양수 | **Cβ 반증 (spec 정합)** — g_c 양 부호는 spec 정의 산출, 결함 없음 → Cα 단독 fix scope 후보 강화 |
| GA[5] 또는 GB[6] ROM entry ≠ PDF Table | **VQ table 결함 (Cβ 단독)** — fix scope = `internal/gain/tables.go` ROM 정정 |
| MA predictor 결과 ≠ spec §3.9.2 식 (frame 0 zero-state past_err 가산 결함) | **predictor 결함 (Cβ 단독)** — fix scope = `internal/gain/decode.go` predictor 로직 |
| sign-mask / γ̂ 결합 산출 로직 결함 (예: 음수 입력의 부호 누락) | **sign 처리 결함 (Cβ 단독)** — fix scope = `internal/gain/decode.go` sign 처리 |
| Cα + Cβ 둘 다 spec 정합 (둘 다 반증) | **hybrid 반증** — Cγ 재진입 또는 Y magnitude follow-up 권고 (Phase 0.4 §3) |

- [ ] **Step 5: 17 회귀 게이트 재확인**

Run: 16 PASS + 항목 17 RED + 신규 측정 2건 (Task 1 의 Cα + Task 2 의 Cβ) PASS.

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-2-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-X-split-2 보고서 — Cβ gain g_c sub-stage 분리

**작성일**: 2026-05-04
**범위**: 후보 Cβ (`gain.Decoder.Decode` 의 g_c=+4153 부호 결정) sub-stage (GA/GB VQ table entry / MA predictor / γ̂ 결합 / sign 처리) 분리 측정.
**산출물**: 측정 함수 1 추가 + sub-stage별 raw + ROM cross-ref + Cβ verdict.
**준수**: F-non-prelim Task 1 의 g_c=+4153 raw 측정 baseline 인계, F-non-prelim-X-split-1 (Cα) verdict 와의 cross-check.

## 0. escape hatch 평가 + spec § PDF verbatim 인용 (§3.9 + §3.9.2 + §A.3.9 + §4.3 Table 9)
## 1. 16 회귀 게이트 PASS + 항목 17 RED + 신규 Cα+Cβ PASS 재확인
## 2. Cβ 측정 raw 출력 (GA[5]/GB[6] entry + predictor + g_c)
## 3. Cβ 후보 평가 (단독 결함 / spec 정합 / 부분 결함)
## 4. F-non-prelim Task 1 §2 의 g_c=+4153 raw 와의 cross-check + ROM Table verbatim cross-ref
## 5. Task 3 (synthesis) 진입 의무
```

```bash
git add internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go \
        docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-non-prelim-X-split-2 Cβ gain g_c trace

후보 Cβ (gain.Decoder.Decode 의 g_c=+4153 (Q12) 부호 결정) 의
sub-stage (GA[5]+GB[6] VQ table entry / MA predictor (frame 0
past_err=MIN_GAIN_PRED_DB) / γ̂ 결합 / sign 처리) 분리 측정.
§3.9 (gain VQ) + §3.9.2 (MA predictor) + §A.3.9 + §4.3 Table 9
verbatim 기반 ROM cross-ref + spec-canonical 식 정합 측정. VQ table
entry 결함 / predictor 결함 / sign 처리 결함 / spec 정합 식별로
Cα verdict (X-split-1) 와의 결합 결정 트리 입력.

assertion 0 (측정-only). production 변경 0 라인. 외부 G.729 0 참조
(Annex A binary 거부 — G1 결정).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-non-prelim-X-split-3: 종합 + 결정 트리 + 다음 cycle outline

**Goal:** Task 1 (Cα) + Task 2 (Cβ) 측정 결과 결합 분석 — Cα/Cβ 비교표 (Phase 0.4 §1 강압-적합 회피 의무 준수) + 다음 cycle 단일 결정 (production fix cycle 진입 / hybrid fix cycle / Cγ 재진입 / Y magnitude follow-up). 단일 후보 식별 시 fix scope outline 작성. Cα+Cβ 둘 다 결함 잔존 시 hybrid fix cycle 권고. Cα+Cβ 둘 다 spec 정합 (반증) 시 Cγ 재진입 또는 Y follow-up 권고. **Cδ 절대 재진입 금지** (Phase 0.4 §7 의무).

**Files:**
- Create: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-synthesis-report.md`

production 변경 0 라인. test 변경 0 라인 (메타 task — 종합 보고만).

- [ ] **Step 1: cycle commit 요약**

Run: `git log --oneline -6`

Expected:
```
<3 hash> docs(plans): F-non-prelim-X-split synthesis ...
<2 hash> test(decoder): F-non-prelim-X-split-2 ...
<1 hash> test(decoder): F-non-prelim-X-split-1 ...
<plan hash> docs(plans): add Phase 1k Stage F-non-prelim-X-split plan
e867f5e docs(plans): F-non-prelim synthesis + cycle decision
dd4e21a docs(plans): F-non-prelim-3 ...
```

- [ ] **Step 2: Cα/Cβ 비교표 (단일 표)**

Task 1~2 측정 결과를 단일 표로 결합 (Phase 0.4 §1 — 측정 데이터만):

| 후보 | 측정 출처 | 결과 | 평가 (단독 결함 / spec 정합 / 부분) | spec § 인용 |
|------|-----------|------|------------------------------------|--------------|
| **Cα** (`fcb.Decode` c[n] sub-stage) | Task 1 §2 + §3 | (Task 1 결과) | (Task 1 결과) | §3.8, §3.8.2, §A.3.8, §4.1.5/6 |
| **Cβ** (`gain.Decoder.Decode` g_c sub-stage) | Task 2 §2 + §3 | (Task 2 결과) | (Task 2 결과) | §3.9, §3.9.2, §A.3.9, §4.1.5/6, §4.3 Table 9 |

- [ ] **Step 3: 결정 트리 (단일 식별 / hybrid 결함 / 둘 다 정합)**

| 시나리오 | 결정 | 다음 cycle 명 |
|----------|------|--------------|
| Cα 단독 "결함" + Cβ "spec 정합" | **단일 식별 (Cα)** — production fix cycle 진입 | F-non-fix-fcb (production 1~3 라인 fix in `internal/fcb/decode.go` 또는 `internal/fcb/tables.go` — Task 1 §3 의 식별 sub-stage) |
| Cβ 단독 "결함" + Cα "spec 정합" | **단일 식별 (Cβ)** — production fix cycle 진입 | F-non-fix-gain (production 1~3 라인 fix in `internal/gain/decode.go` 또는 `internal/gain/tables.go` — Task 2 §3 의 식별 sub-stage) |
| Cα + Cβ 둘 다 "결함" 잔존 (= hybrid 결함) | **hybrid fix cycle** — production fix in fcb + gain 두 layer 동시 | F-non-fix-hybrid (production 2~6 라인 fix; 단 측정 데이터로 둘 다 결함 입증 시에만, Phase 0.4 §6 정합) |
| Cα + Cβ 둘 다 "spec 정합" (둘 다 반증) | **Cγ 재진입 또는 Y follow-up** — F-non-prelim synthesis §3.4 Cγ LOW priority dismiss 의 측정-기반 재고 + Y magnitude gap (max\|Δ\|=6, `d1a4f2d` §3.1) follow-up | F-non-Cgamma-revisit 또는 F-non-Y-magnitude-followup (Phase 0.4 §3 정합, Cδ 재진입 절대 금지) |
| 측정 도구 결함 (replication mismatch / Q-format 부정합 / ROM lookup 실패) | **E2 발동** — 측정 reset + 재실행 | (해당 task 재실행) |

본 §3 결정은 *측정 데이터에 의해 자동 결정* — 임의 선택 금지 (Phase 0.4 §1, §6).

- [ ] **Step 4: 다음 cycle 권고 outline (식별 후보별)**

식별된 후보에 따라:

| 식별 후보 | 다음 cycle 명 | scope outline | 예상 fix 라인 수 |
|-----------|--------------|---------------|------------------|
| Cα (raw placement 단독) | F-non-fix-fcb-pulse | `fcb.Decode` 의 sign 처리 또는 pulse placement | 1~3 |
| Cα (β enhancement 단독) | F-non-fix-fcb-enhance | β·c[n−T] enhancement 의 n<T 영역 boundary 처리 | 1~3 |
| Cβ (VQ table 단독) | F-non-fix-gain-rom | `internal/gain/tables.go` GA/GB ROM entry 정정 | 1~5 |
| Cβ (predictor 단독) | F-non-fix-gain-predictor | MA predictor 식 정정 또는 past_err init | 1~3 |
| Cβ (sign 처리 단독) | F-non-fix-gain-sign | g_c sign mask 또는 γ̂ 결합 부호 처리 | 1~3 |
| Cα + Cβ hybrid | F-non-fix-hybrid | 양 layer 동시 fix | 2~6 |
| 둘 다 spec 정합 | F-non-Cgamma-revisit / F-non-Y-magnitude-followup | (synthesis §4 권고) | 0 (진단) ~ ? (fix) |

각 fix cycle 의 GREEN gate = 항목 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) — RED → GREEN 전환 의무.

- [ ] **Step 5: 잔여 보류 항목 갱신**

F-non-prelim synthesis (`e867f5e`) §5 사용자 게이트 G-S1~G-S5 의 본 cycle 결과 반영 + 신규 보류 항목 추가:

| # | 항목 | 본 cycle 갱신 |
|---|------|---------------|
| 1 | F-non-prelim-X-split cycle 자체 | 본 cycle 종결 |
| 2 | Cα/Cβ 단일 식별 / hybrid / 둘 다 정합 | (Step 3 결과) |
| 3 | `stagef_bis_diagnostic_test.go` untracked | 보존 유지 (변경 0) — Phase 0.5 충족 |
| 4 | 다음 fix cycle 의 RED gate | 항목 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 승계 |
| 5 | Cγ 재진입 (synthesis §3.4 LOW priority) | (Step 3 결과 — Cα+Cβ 둘 다 정합 시에만 진입) |
| 6 | Y magnitude follow-up (max\|Δ\|=6, `d1a4f2d` §3.1) | (Step 3 결과 — 부호 fix 후 또는 둘 다 정합 시 진입) |
| 7 | Cδ DISMISSED 유지 (M6 반증) | 본 cycle 어떤 task 도 Cδ 재진입 0 — Phase 0.4 §7 충족 |
| 8 | F-non-prelim synthesis G-S1~G-S5 결정 | 본 cycle 진입 정합 (G-S2 hybrid 승인 + G-S3 본 cycle 진입 승인 채택) |

- [ ] **Step 6: 보고서 작성 + commit**

`docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-synthesis-report.md`:

```markdown
# Phase 1k Stage F-non-prelim-X-split 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-04
**범위**: F-non-prelim-X-split-1 (Cα) + F-non-prelim-X-split-2 (Cβ) 결합 분석 + Cα/Cβ 비교 + 다음 cycle 단일 결정 (fix / hybrid / Cγ 재진입 / Y follow-up).
**산출물**: cycle 결산 + Cα/Cβ 비교표 + 결정 트리 + 다음 cycle plan outline + 잔여 보류 항목 + Phase 1k 진척 평가.
**준수**: Phase 0.4 강압-적합 회피 (측정 데이터만), Phase 0.4 §7 Cδ 재진입 절대 금지, 사용자 G1 결정 (Annex A binary 거부), production 변경 0 라인 (메타 task), 사전 보유 working tree 보존.

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)
## 1. F-non-prelim-X-split cycle commit 요약 + cycle premise (G-S2 hybrid 승인)
## 2. Cα/Cβ 비교표 (단일 표, 측정 데이터만)
## 3. 단일 식별 결정 (또는 hybrid fix / Cγ 재진입 / Y follow-up)
## 4. 다음 cycle 권고 (production fix / 추가 진단 / follow-up)
## 5. 잔여 보류 항목 갱신 + 사용자 게이트
## 6. Phase 1k 진척 평가
```

```bash
git add docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-synthesis-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-non-prelim-X-split synthesis + next cycle decision

F-non-prelim-X-split cycle (Task 1 Cα fcb pulse trace, Task 2 Cβ gain
g_c trace) 의 결합 분석 + Cα/Cβ 비교표 + 다음 cycle 단일 결정.
F-non-prelim 종합 (e867f5e) §4.1 hybrid 권고 + 사용자 G-S2/G-S3 진입
승인 후 측정 데이터만으로 단일 식별 또는 hybrid fix 또는 Cγ 재진입
또는 Y magnitude follow-up 결정. Cδ 재진입 절대 금지 (Phase 0.4 §7).

production 변경 0 (메타 task). 외부 G.729 0 참조 (G1 결정 정합).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

### 1. Spec coverage

F-non-prelim synthesis (`e867f5e`) §3.1 단일 source + §3.2 4 후보 평가표 + §4.1 hybrid 권고 + §4.2 3-task outline + §5 사용자 게이트 G-S1~G-S5 결정 + 본 plan task 매핑:

- 단일 source = `g_c · c[n]` 곱 → Task 1 (Cα = c[n]) + Task 2 (Cβ = g_c) 분리 측정.
- Cα HIGH priority + spec §3.8 / §3.8.2 / §A.3.8 → Task 1 § 인용 catalog.
- Cβ HIGH priority + spec §3.9 / §3.9.2 / §A.3.9 / §4.3 Table 9 → Task 2 § 인용 catalog.
- Cγ LOW priority (Y/Z 측정으로 dismiss) → Task 3 §3 결정 트리의 "둘 다 정합 시에만 재진입" 조건 + Phase 0.4 §3 / §6 의무.
- Cδ DISMISSED (M6 반증) → Task 3 §3 결정 트리에 *부재* + Phase 0.4 §7 절대 금지 의무.
- synthesis §4.2 의 3-task outline (C-fcb-pulse-trace / C-gain-gc-trace / synthesis) → 본 plan Task 1/2/3 매핑 정합.
- 사용자 G-S2 (hybrid 결정 승인) → 본 plan 진입 premise.
- 사용자 G-S3 (F-non-prelim-X-split 진입 승인) → 본 plan 자체 정합.
- 사용자 G-S4 (bis 보존) → Phase 0.5 의무.
- 사용자 G-S5 (항목 17 RED 잔존 acknowledge) → Phase 0.3 항목 17 RED 의무 승계.

5 항목 모두 매핑 완료. 누락 0.

### 2. Placeholder scan

- 각 task 의 Step N 보고서 outline 에 *각 § 명시* (placeholder 없이).
- Task 1/2 의 Step 4 또는 §3 에 *test outline 코드* 또는 *측정 출력 형식* 제시 (signature, 측정 점, dump 형식).
- Task 3 의 보고서 outline 은 *완전한 측정 출력 형식* + spec § 인용 + 후보 평가 표 제시. 구체 측정 결과는 task 실행 시점 결정 (dump 값은 측정에 의해 도출되므로 placeholder 가 아님 — 보고서에 "(Task N 결과)" 로 명시).
- 각 commit 메시지 *완전한 한국어 본문* + co-author trailer.

placeholder 0 확인.

### 3. Type consistency

- Task 1 신규 test `TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace`: helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`, `frameSamples`) 모두 기존 `decoder` package test 정의 — 신규 helper 0. fcb.Decode 호출 signature = `Decode(idx Indices, t int, betaQ14 int16, c *[40]int16)` (production exported API).
- Task 2 측정 함수 `TestDiagnostic_FnonPrelimXSplit2GainGcTrace`: gain.Decoder.Decode 호출 signature = `(d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14, gcQ12 int16)` + ROM table 직접 access (`internal/gain/tables.go` 또는 동일 package white-box).
- Task 3 = 보고서 only — type 변경 0.
- 회귀 게이트 17 항목의 test 이름 Phase 0.3 ↔ 각 task Step 에서 일관.
- production 변경 0 라인 의무 (E5 + Phase 0.4 §4) — 본 cycle 3 task 모두 test/docs 변경만.

type consistency clean.

### 4. Spec § 인용 정합성 (특별 검토)

본 plan 의 spec 인용은 PDF `pdftotext -layout` verbatim. 각 task 진입 시 Step 2 등에서 grep 재확인 의무 명시. F-oct-prelim-5-4 §3.6 의 "g_l > 0" 결합 해석 또는 F-oct-postfix-2 의 "γ_t 분기 strict reading" 결합 해석은 본 cycle 의 spec 인용으로 사용하지 *않는다* (Phase 0.4 §2). Cα (§3.8 + §3.8.2 + §A.3.8) / Cβ (§3.9 + §3.9.2 + §A.3.9 + §4.3 Table 9) 의 인용 출처가 후보별 분리 — 결합 해석 위험 0. 공통 인용 (§4.1.5/6 eq.(75)) 은 X-fcb verdict 합치 cross-check 용 (F-non-prelim Task 1 §0.2 정정 패턴 답습).

### 5. 사용자 G1 결정 정합성 특별 검토

G1 (c) = "Annex A binary 거부 + 후보 ③ pivot". 본 plan:

- Phase 0.2 E4: 외부 G.729 구현 (Annex A binary 포함) 0건 인용. 본 cycle 어떤 측정도 외부 binary 동작 비교 금지.
- Phase 0.4 §5: g_l 영속화 (후보 ①) 관련 측정 / fix 도입 금지 — 본 cycle 은 g_l 보다 상류 (fcb/gain decoding sub-stage) 측정이므로 자연 회피.
- Task 1~3 모든 측정: PDF (`docs/superpowers/specs/itu/G729E.pdf`) + READMETV.txt + repo committed PST 파일 + 본 repo internal 패키지 (production exported API + ROM table) 만 사용. Annex A binary trace 의 ground-truth 대체 불가 — 측정 한계는 보고서 §0 명시.
- 본 cycle 은 후보 ③ 의 spec scope 확장 (§3.8 / §3.9 / §A.3.* 추가) — 사용자 G-N5 (a) "G1 spec scope 한계 인정 + 다음 cycle 진입" 정합.

G1 결정 정합 100%.

### 6. 회귀 게이트 17 항목 정합성

Phase 0.3 의 17 항목:
- F-quart 2 (항목 1, 2)
- F-sext 1 (항목 3)
- F-sept 3 (항목 4, 5, 6)
- F-oct-prelim 3 (항목 7, 8, 9)
- F-oct-prelim-5 4 (항목 10, 11, 12, 13)
- ITU contract 1 (항목 14)
- F-oct-postfix2-prelim 1 (항목 15 — 4 측정 함수 묶음)
- F-non-prelim 1 (항목 16 — Task 1 X + Task 2 Y measurement 묶음)
- F-oct-postfix-1 RED 1 (항목 17)

합계 = 2+1+3+3+4+1+1+1+1 = **17**. 사용자 task 명세 ("누적 contract test 18건 (17 + 신규 1)") — 본 cycle Task 1 의 신규 측정 promotion 시 18건. 단 자동 promotion 금지 (Phase 0.3) — 본 cycle 종결 후 별도 게이트 결정에서 promotion 여부 판단 (synthesis §5 잔여 보류).

> 본 cycle Task 1/2 의 신규 측정 (`TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace`, `TestDiagnostic_FnonPrelimXSplit2GainGcTrace`) 자체는 회귀 게이트 18/19번째 항목으로 *자동 promotion 하지 않는다* (Phase 0.3 자동 promotion 금지). 항목 16 의 "F-non-prelim measurement 묶음" 은 직전 cycle 에서 만들어진 X+Y measurement test 묶음을 가리킴.

### 7. Phase 0.4 강압-적합 회피 의무 (본 cycle 특별 강조)

본 cycle 은 *직전 cycle 단일 source 식별 + hybrid 권고* 후의 후속 진단. 강압-적합 위험이 가장 높음 (synthesis §4.1 hybrid 권고가 본 cycle 측정 결과를 hybrid 채택 방향으로 해석할 유혹). 회피 의무 7 항목 (Phase 0.4 §1-7) 각 task Step 에 명시 — 특히:

- Task 1 §5 의 sub-stage 평가표가 5 시나리오 (raw placement 단독 / β-enhancement 단독 / spec 위반 / replication 결함 / Cα 반증) 모두 명시 — "raw placement 단독" 강압적 채택 회피.
- Task 2 §4 의 sub-stage 평가표가 5 시나리오 (Cβ 반증 / VQ table 결함 / predictor 결함 / sign 처리 결함 / hybrid 반증) 모두 명시 — "Cβ 반증" 강압적 채택 회피.
- Task 3 §3 결정 트리가 5 시나리오 (Cα 단독 / Cβ 단독 / 둘 다 결함 hybrid / 둘 다 정합 / 측정 도구 결함) 모두 명시 — synthesis hybrid 권고가 *측정 결과를 좌우하지 않음*.
- Phase 0.4 §6 (hybrid 결정 강요 금지) — Cα 또는 Cβ 단독 식별 시 synthesis hybrid 권고를 *측정 우선* 으로 재고.
- Phase 0.4 §7 (Cδ 재진입 절대 금지) — Task 3 §3 결정 트리에 Cδ 부재.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-prelim-x-split-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, F-non-prelim 패턴 답습. 각 task 완료 후 main agent 가 다음 task 진입 권고를 사용자에게 게이트.

**2. Inline Execution** — batch execution. Task 1 Cα fcb sub-stage 측정 → 사용자 게이트 → Task 2 Cβ gain sub-stage 측정 → 사용자 게이트 → Task 3 종합.

**Recommended user gate before Task F-non-prelim-X-split-1 dispatch**: 사용자가 본 plan 의 Phase 0.4 §1 (Cα vs Cβ 임의 우선 결정 금지) + §6 (hybrid 결정 강요 금지) + §7 (Cδ 재진입 절대 금지) + Phase 0.5 (bis test 보존) + Phase 0.3 회귀 게이트 17 항목 (특히 항목 17 RED 잔존 의무) 을 검토 후 진입 승인.

**Which approach?**
