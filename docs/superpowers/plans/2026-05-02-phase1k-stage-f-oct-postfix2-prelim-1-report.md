# Phase 1k Stage F-oct-postfix2-prelim-1 보고서 — 5-stage chain dump baseline

**작성일**: 2026-05-02
**범위**: Annex A postfilter chain stage dump harness 추가 (측정-only).
**산출물**: `internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go` 신규 1 파일 + dump raw 출력 verbatim.
**준수**: G3 반증 후 4 가설 (M1'/M3/M5/M6) 측정의 공통 baseline.
**production 변경**: 0 라인. **테스트 변경**: 1 신규 파일.

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G1 결정 정합성

Working tree pre-check (`git status --porcelain && git log -1 --oneline`):

```
?? internal/decoder/stagef_bis_diagnostic_test.go
118446e (HEAD -> main) docs(plans): add Phase 1k Stage F-oct-postfix2-prelim plan
```

- Plan §Step 1 의 expected `8907847 docs(plans): F-oct-postfix synthesis + cycle decision (E3)` 와 차이 = HEAD 가 plan commit (`118446e`) 으로 1단계 앞서 있음. `8907847` 은 직전 cycle synthesis commit 으로 `git log` 상 존재 (118446e 의 직계 부모). plan 의 `Expected` 는 plan commit 이전 시점 기준 — 실제 작업 시점에서 plan commit 이 추가된 것은 *정합* (plan 자체가 본 cycle 의 Task 1 진입 직전 커밋된 source-of-truth).
- 사전 보유 untracked file `internal/decoder/stagef_bis_diagnostic_test.go` 보존 — 본 task 어떤 파일도 수정하지 않음.

| Hatch | 평가 |
|-------|------|
| **E1** | 미발동. 회귀 게이트 14건 PASS + 항목 15 RED 잔존 (다음 fix cycle GREEN gate). 신규 회귀 0건. |
| **E2** | 미발동. spec § 인용 = `§A.4.2 (PDF p.43) chain order = long-term → short-term → tilt → AGC` 만 사용 (test 파일 doc comment) — F-sext / F-oct-postfix 누적 인용과 동일, plan 상단 인용 1 과 일치. 신규 PDF verbatim 인용 0 (Step 2 한계 기반 — 외부 관찰 가능 stage 만 dump). |
| **E3** | 본 task 적용 외 (Task 5 종합 시점에서 평가). |
| **E4** | 미발동. 외부 G.729 구현 0 참조. ITU-T G.729 (06/2012) PDF + READMETV.txt + 본 repo 의 commit 된 PST 입력 stimulus 만 사용. Annex A binary 사용 0 (G1 결정 준수). |
| **E5** | 미발동. production 변경 0 라인. test 변경 = 신규 1 파일 (`internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go`, 45 라인). |

**G1 결정 정합성**: 사용자 결정 = "(c) Annex A binary 거부 + 후보 ③ pivot". 본 task 의 dump harness 는 4 가설 (M1'/M3/M5/M6) 비교의 공통 ground-truth 만 제공 — Annex A binary 실행 / black-box trace / 외부 reference 0건. 정합.

**Step 2 한계 명시 (plan §Step 2 후미)**: 본 baseline test 는 외부 관찰 가능 stage (chain stage 7 = post-hpfilter `out[5..7]`) 만 dump. 내부 stage 1, 2 (excitation, synth IIR), 3-6 (longterm / shortterm / tilt / AGC) 는 production API 가 stage 별 출력을 노출하지 *않으므로* 본 test 에서 직접 측정 불가. 해당 stage 의 raw 측정은 Task 2 (M5: excitation + synth IIR sign trace, `internal/synth` 또는 `internal/decoder/subframe.go` white-box) 와 Task 4 (M1' + M3: `internal/postfilter` + `internal/synth` package 내부 white-box test) 로 분리 인계.

## 1. 회귀 게이트 baseline (14 PASS + 항목 15 RED)

| # | Test | 결과 |
|---|------|------|
| 1 | TestDiagnostic_FquartGainImap_Sf0Sample0to7 | PASS |
| 2 | TestDiagnostic_FquartGainReferenceCrossCheck | PASS |
| 3 | TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 | PASS |
| 4 | TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 | PASS |
| 5 | TestDiagnostic_FseptLPReferenceCrossCheck | PASS |
| 6 | TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 | PASS |
| 7 | TestDiagnostic_FoctPrelimPSTFormat | PASS |
| 8 | TestDiagnostic_FoctPrelimFrameAlignment | PASS |
| 9 | TestDiagnostic_FoctPrelimMultiVectorScan | PASS |
| 10 | TestDiagnostic_FoctPrelim5PSTSourceVerbatim | PASS |
| 11 | TestDiagnostic_FoctPrelim5BitVectorCompare | PASS |
| 12 | TestDiagnostic_FoctPrelim5HpFilterInitState | PASS |
| 13 | TestDiagnostic_FoctPrelim5SilenceNegativeMechanism | PASS |
| 14 | TestDecode_Frame0Sample0_MatchesALGTHM | PASS |
| 15 | TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput | **FAIL** (의도된 RED — 다음 fix cycle GREEN gate) |
| Contract | `internal/postfilter` Q-format contract 4건 | PASS |
| Contract | `internal/synth` Q-format contract 4건 | PASS |
| Sanity | `go vet ./...` | clean |
| 신규 | TestDiagnostic_FoctPostfix2PrelimChainDump | PASS (측정-only) |

비-contract 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 본 cycle 진입 시점 FAIL 유지 — production 변경 0 라인 의무로 자동 보장, 본 task 어떤 파일도 변화시키지 않음 (plan 허용).

## 2. chain stage dump raw 출력 (sample 5..7)

`go test ./internal/decoder/ -run TestDiagnostic_FoctPostfix2PrelimChainDump -v` verbatim:

```
=== RUN   TestDiagnostic_FoctPostfix2PrelimChainDump
    stagef_octpostfix2_prelim_diagnostic_test.go:31: ALGTHM frame 0 sf0 sample 5..7 (PST want = [-1 -1 -1])
    stagef_octpostfix2_prelim_diagnostic_test.go:33:   decoded out[5..7] (post-hpfilter)            = [2 2 2]
    stagef_octpostfix2_prelim_diagnostic_test.go:35:   delta vs PST want                            = [3 3 3]
--- PASS: TestDiagnostic_FoctPostfix2PrelimChainDump (0.00s)
```

| stage | 출처 (이미 측정된 누적 진단) | sample 5 | sample 6 | sample 7 | 부호 |
|-------|------------------------------|---------:|---------:|---------:|------|
| 1. excitation u[n] | F-sept-1 (`stagef_sept_diagnostic_test.go:530`) | +0 | (Task 2 측정) | (Task 2 측정) | 0 / Task 2 인계 |
| 2. synth IIR syn[n] | F-sext / F-sept-3 | +1 | +1 | +1 | [+ + +] |
| 3. long-term postfilter | (production API 미노출 — Task 4 인계) | — | — | — | — |
| 4. short-term postfilter | (동상) | — | — | — | — |
| 5. tilt | (동상) | — | — | — | — |
| 6. AGC | (동상) | — | — | — | — |
| 7. post-hpfilter out[n] | **본 task baseline** | **+2** | **+2** | **+2** | **[+ + +]** |
| Δ vs PST want | 본 task (want = [-1 -1 -1]) | **+3** | **+3** | **+3** | want=[− − −] |

stage 2 / stage 7 cross-reference (F-sext `stagef_sext_diagnostic_test.go:99-105`): stage 2 syn = [1 1 1], postfilter.Filter = [1 1 1], hpFilter = [1 1 1], `pcm.ScaleUpSat` (PST 도메인) = [2 2 2]. 본 task baseline 의 `out[5..7] = [2 2 2]` 와 정합 — chain stage 7 (post-hpfilter) 출력은 PST 도메인 scaling 후 값.

## 3. F-oct-postfix synthesis §2.4 의 "tilt 외 부호 결정 항" 식별 정량 baseline

F-oct-postfix synthesis (`8907847`) §2.4 의 G3 반증:
- γ_t 분기 flip (k1 ≥ 0 strict reading) 적용 후 Δ=0 — sample 5..7 의 16-bit signed LSB 단위에서 *tilt 분기는 부호 결정 항이 아님*.
- 본 task baseline 정량: `out[5..7] = [+2 +2 +2]`, Δ vs PST want = [+3 +3 +3] 모두 *동일 부호* + *동일 크기* (frame 0 sf0 의 sample 5..7 mismatch 가 sample 별 변동 0). 부호 결정 항이 sample-uniform — sample-specific (per-tap) 이 아니라 *subframe-level* 또는 *전역 상수 (gain / sign flip)* 일 가능성 높음.
- 후보 위치 (synthesis §2.4):
  - (i) 합성 LP filter 출력 (synth IIR) — F-sept-3 측정에서 sample 5 에서 prod = ref (둘 다 +1) 로 확인, Task 4 M3 재진단 대상.
  - (ii) long-term postfilter g_l 적용 단계 — Task 4 M1' 측정 대상.
  - (iii) AGC update path — Task 4 M1' 측정 대상.
  - (iv) high-pass filter post-AGC — F-oct-prelim-5 hpFilter init state PASS, *부호 flip* 과 무관할 가능성 높으나 Task 4 측정에서 보조 확인.
  - (v) excitation 자체 — Task 2 M5 sample 5..7 한정 측정 대상.
  - (vi) PST want 데이터 자체 — Task 3 M6 측정 대상.

본 baseline 의 *sample-uniform* + *부호 단일* 특성은 Task 5 종합 시 4 가설 비교의 *분류축* 으로 사용:
- 가설별 측정에서 sample 5/6/7 의 부호/크기 일관성 (uniform) 을 동일하게 재현하는 항이 단일 식별 후보.
- 만약 가설별 측정이 sample-별 변동 (예: sample 5 만 부호 flip) 만 보이면, 해당 가설은 본 baseline 의 uniform 패턴과 *불정합* → 반증.

## 4. Task 2/3/4 진입 의무 항목 (4 가설 측정의 공통 ground-truth 인계)

| Task | 가설 | 본 baseline 인계 자료 | 측정 대상 stage |
|------|------|------------------------|-----------------|
| Task 2 | M5 (excitation 부호) | sample 5 u[5]=+0 (F-sept-1), sample 6/7 미측정 | stage 1 (excitation u), stage 2 (synth IIR syn), pre-postfilter |
| Task 3 | M6 (PST want 부호) | PST want = [-1 -1 -1], byte-level / endianness Task 3 검증 | PST 파일 byte stream |
| Task 4 | M1' (postfilter 외 분기) | stage 3-6 production API 미노출 → white-box test 의무 | stage 3 (longterm), 4 (shortterm), 6 (AGC) |
| Task 4 | M3 (synth IIR chain) | F-sept-3 (sample 5 prod=ref) + 본 baseline (out=+2) | stage 2 + stage 7 cross-check |

**baseline 정합성 확인**: 모든 후속 task 측정은 본 baseline 의 `out[5..7] = [+2 +2 +2]` / Δ = [+3 +3 +3] / sample-uniform 특성과 cross-check 의무. 임의 측정이 본 baseline 과 *불정합* (예: 동일 입력에서 out 가 다른 값) 시 즉시 측정 폐기 + 환경 재진단 (E1 발동 검토).

---

**다음 task**: F-oct-postfix2-prelim-2 (M5 excitation pre-postfilter 부호 trace).
