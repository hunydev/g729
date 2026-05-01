# Phase 1k Stage F-oct-prelim-3 보고서 — 6 ITU vector cross-check

**작성일**: 2026-04-30
**범위**: ALGTHM / TEST / SPEECH / LSP / PITCH / FIXED 의 frame 0 sf0
        sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7
        부호 비교. ALGTHM 특이성 (G5) vs 일반 결함 (G1/G3/G4) 분리.
**산출물**: 6 vector × sample 5..7 매칭 표 + 시나리오 (V3 mixed) 분류.
**준수**: ITU-T G.729 (06/2012) §6 + Annex A README 만 인용. 외부 구현 0건.
**production 변경**: 0 라인.

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 Working tree pre/post

Pre (Task F-oct-prelim-2 commit `94ef154` 직후):
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

Post (본 task 작업 후, commit 직전):
```
 M internal/decoder/foct_prelim_diagnostic_test.go
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md
?? internal/decoder/stagef_bis_diagnostic_test.go
```

`stagef_bis_diagnostic_test.go` 사전 보유 untracked 파일 — **미변경 보존 확인**.

### 0.2 Escape hatch 평가표

| 코드 | 게이트 | 평가 |
|------|--------|------|
| E1 | 회귀 게이트 1건이라도 FAIL | **미발동** — 9/9 PASS |
| E2 | 외부 G.729 구현 참조 필요 | **미발동** — ITU PDF + Annex A README 만 인용 |
| E3 | spec § 위반 | **미발동** — 측정-only |
| E4 | 외부 구현 0 위반 | **미발동** |
| E5 | production 변경 발생 | **미발동** — 0 라인 |

---

## 1. §6 + README 인용 (vector 의도 정의)

ITU Annex A test vector README (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt`)
verbatim:

```
Testvectors to verify correct execution of G.729A ANSI-C software
Version 1.1
...
Format: all files contain 16 bit sampled data using the Intel (PC) format.

*.in  - input files
*.bit - bit stream files
*.out - output files

algthm          - conditional parts of the algorithm
erasure         - frame erasure recovery
fixed           - fixed codebook search
lsp             - lsp quantization
overflow        - overflow detection in synthesizer
parity          - parity check
pitch           - pitch search
speech          - generic speech file
tame            - taming procedure
```

→ vector 의도:
- **algthm**: conditional 분기 (silence frame 0 포함 추정 — F-oct-prelim-1/2 측정과 정합)
- **test**: README 미언급 (auxiliary)
- **speech**: 일반 음성
- **lsp**: LSP 양자화
- **pitch**: pitch 탐색 stress
- **fixed**: fixed codebook 탐색 stress

ITU-T G.729 (06/2012) §6 "Test vectors" — PDF 본문에서 sample-level 정합 기준은 §6 일반 기술뿐이며 vector-별 의도는 Annex A README 가 단일 출처. (외부 구현 무참조).

PST 포맷: README 명시 — "16 bit sampled data using the Intel (PC) format" → LittleEndian, 80 sample/frame × 2 byte = 160 byte/frame. F-oct-prelim-1 측정과 정합.

`TEST.pst` lowercase 확장자 — Annex A 배포 그대로 보존; 본 진단은 `os.Stat` fallback 으로 양쪽 case 처리.

---

## 2. 회귀 게이트 결과 (9건)

| # | Test | 결과 |
|---|------|------|
| 1 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| 2 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 3 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 4 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 5 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 6 | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 7 | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 8 | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 9 | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |

추가:
- `go test ./internal/...` — 신규 FAIL 0. 기존 plan-허용 비-contract diagnostic 3건 FAIL 유지:
  `TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`,
  `TestDecode_SucceedsAcrossAllGainIndices`.
- `go vet ./...` — clean (출력 0 라인).

---

## 3. 진단 측정값

### 3.1 Raw output (`go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimMultiVectorScan -v`)

```
=== RUN   TestDiagnostic_FoctPrelimMultiVectorScan
    foct_prelim_diagnostic_test.go:308: ──────── (a) 6 vector × frame 0 sf0 sample 0..7 ────────
    foct_prelim_diagnostic_test.go:310: [ALGTHM]
    foct_prelim_diagnostic_test.go:311:   prod = [2 2 2 2 0 2 2 2]
    foct_prelim_diagnostic_test.go:312:   want = [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:313:   sign = [= = = = ≠ ≠ ≠ ≠]  (sample 5..7 일치 0/3)
    foct_prelim_diagnostic_test.go:310: [TEST]
    foct_prelim_diagnostic_test.go:311:   prod = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:312:   want = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:313:   sign = [= = = = = = = =]  (sample 5..7 일치 3/3)
    foct_prelim_diagnostic_test.go:310: [SPEECH]
    foct_prelim_diagnostic_test.go:311:   prod = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:312:   want = [0 2 0 0 0 -2 0 0]
    foct_prelim_diagnostic_test.go:313:   sign = [≠ = ≠ ≠ = ≠ = =]  (sample 5..7 일치 2/3)
    foct_prelim_diagnostic_test.go:310: [LSP]
    foct_prelim_diagnostic_test.go:311:   prod = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:312:   want = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:313:   sign = [= = = = = = = =]  (sample 5..7 일치 3/3)
    foct_prelim_diagnostic_test.go:310: [PITCH]
    foct_prelim_diagnostic_test.go:311:   prod = [2 6 4 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:312:   want = [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:313:   sign = [= = = = ≠ ≠ ≠ ≠]  (sample 5..7 일치 0/3)
    foct_prelim_diagnostic_test.go:310: [FIXED]
    foct_prelim_diagnostic_test.go:311:   prod = [2 2 2 2 0 0 0 0]
    foct_prelim_diagnostic_test.go:312:   want = [2 4 2 1 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:313:   sign = [= = = = ≠ ≠ ≠ ≠]  (sample 5..7 일치 0/3)
    foct_prelim_diagnostic_test.go:316: ──────── (b) sample 5..7 부호 일치 분포 요약 ────────
    foct_prelim_diagnostic_test.go:317: vector       sample5  sample6  sample7  match5..7
    foct_prelim_diagnostic_test.go:321:   ALGTHM       ≠        ≠        ≠        0/3
    foct_prelim_diagnostic_test.go:321:   TEST         =        =        =        3/3
    foct_prelim_diagnostic_test.go:321:   SPEECH       ≠        =        =        2/3
    foct_prelim_diagnostic_test.go:321:   LSP          =        =        =        3/3
    foct_prelim_diagnostic_test.go:321:   PITCH        ≠        ≠        ≠        0/3
    foct_prelim_diagnostic_test.go:321:   FIXED        ≠        ≠        ≠        0/3
    foct_prelim_diagnostic_test.go:331: ──────── F-oct-prelim-3 시나리오 분류 ────────
    foct_prelim_diagnostic_test.go:340: (V3) mixed — 일부 vector 정합 / 일부 반전
    foct_prelim_diagnostic_test.go:341:      ALGTHM-specific 거동 (G5 발현) 가능성 + vector-specific 분포 분석 의무
--- PASS: TestDiagnostic_FoctPrelimMultiVectorScan (0.01s)
PASS
```

### 3.2 6 vector × sample 5..7 매칭 표

| Vector | prod[0..7]              | want[0..7]                   | sample5 | sample6 | sample7 | 5..7 매치 | 분류 |
|--------|-------------------------|------------------------------|---------|---------|---------|-----------|------|
| ALGTHM | `[2 2 2 2 0 2 2 2]`     | `[2 4 3 3 1 -1 -1 -1]`       | ≠       | ≠       | ≠       | **0/3**   | 반전 |
| TEST   | `[2 2 2 2 0 0 0 0]`     | `[2 2 2 2 0 0 0 0]`          | =       | =       | =       | **3/3**   | 정합 |
| SPEECH | `[2 2 2 2 0 0 0 0]`     | `[0 2 0 0 0 -2 0 0]`         | ≠       | =       | =       | **2/3**   | 부분정합 |
| LSP    | `[2 2 2 2 0 0 0 0]`     | `[2 2 2 2 0 0 0 0]`          | =       | =       | =       | **3/3**   | 정합 |
| PITCH  | `[2 6 4 2 0 0 0 0]`     | `[2 4 3 3 1 -1 -1 -1]`       | ≠       | ≠       | ≠       | **0/3**   | 반전 |
| FIXED  | `[2 2 2 2 0 0 0 0]`     | `[2 4 2 1 1 -1 -1 -1]`       | ≠       | ≠       | ≠       | **0/3**   | 반전 |

### 3.3 정합 / 반전 vector 그룹 공통점 보조 분석

**정합 그룹 (TEST, LSP — 5..7 매치 3/3)**:
- 두 vector 의 want sample 0..7 이 모두 `[2 2 2 2 0 0 0 0]` — 즉 frame 0 자체가 *near-zero* low-energy 출력. prod 도 동일 `[2 2 2 2 0 0 0 0]`.
- 결론: 부호가 "정합" 되는 이유는 **prod 와 want 가 모두 0 이므로 부호도 모두 0**. 비정보적 정합 (trivial match).
- 즉 chain 은 silence frame 출력만 동일하게 0 으로 수렴. 실제 신호 추적은 검증 안 됨.

**반전 그룹 (ALGTHM, PITCH, FIXED — 5..7 매치 0/3)**:
- 모두 want 의 sample 5..7 이 음수 (`-1, -1, -1` 또는 `-1, -1, -1`).
- prod 는 모두 0 (또는 +양수).
- 즉 production 은 frame 0 sf0 의 sample 5..7 에서 음수를 생성하지 못함. ALGTHM 만의 특이성이 아님 — PITCH/FIXED 도 동일 결함.

**부분정합 (SPEECH — 2/3)**:
- want sample 0..7 = `[0 2 0 0 0 -2 0 0]` — 매우 sparse, 소수 nonzero.
- prod 의 sample 5 = 0, want = -2 → 부호 불일치 (≠).
- sample 6, 7 모두 0/0 → 부호 정합 (= trivially).
- match 2/3 도 마찬가지로 trivial-zero 가 절반 기여.

**핵심 통찰**:
- **정합 (3/3) 으로 분류된 TEST/LSP 는 want 자체가 silence (0) 이라 정보 없음**.
- **want 가 nonzero 음수인 vector 3건 (ALGTHM, PITCH, FIXED) 모두 0/3 반전** — 일관된 결함.
- 즉 vector-specific 거동이 아니라 **want 가 negative 한 vector 에서 production 이 negative 를 못 만드는 일관 결함**.

---

## 4. 시나리오 분류 (V1 / V2 / V3)

표면 분류: **V3 (mixed) — 일부 vector 정합 / 일부 반전**.

그러나 §3.3 보조 분석에 따라 *해석된* 분류:
- 정합으로 보이는 TEST/LSP 는 want=0 의 trivial match. **정보적 정합 0건**.
- want 가 sample 5..7 에서 nonzero negative 를 갖는 vector 3건 (ALGTHM, PITCH, FIXED) 은 **모두 0/3 반전** — V2 와 등가.

→ **본질적 시나리오: V2 (일반 결함)**. V3 는 silence-saturation 으로 인한 위장.

---

## 5. 가설 G5 평가 + Task 4 진입 권고

### G5 (ALGTHM silence frame 0 특이) 평가

**기각**. ALGTHM 만의 특이성이 아님:
- want 가 nonzero negative 인 3 vector (ALGTHM, PITCH, FIXED) 모두 동일 0/3 반전.
- ALGTHM 만 아니라 PITCH (pitch search stress) 와 FIXED (fixed codebook stress) 의 frame 0 sf0 sample 5..7 도 동일 결함.
- G5 는 *vector-specific* 가설이었으나 측정은 *일반적* 일관 결함을 시사.

### G3 (Annex A vs main) 평가

**보류 / 약한 가능성**. 본 측정만으로는 결정 불가:
- production decode 는 G.729A (Annex A) 기준이나 ITU 분석은 main G.729 (Annex E PDF) 도 참조.
- frame 0 silence 입력에서 음수 출력을 만드는 쪽이 어디인지 (main vs Annex A) 직접 비교 안 됨.
- F-oct-prelim-4 에서 BIT raw bits + ITU C reference (외부 참조 0 위반) 비교 대신, **PST 자체의 ITU 생성 절차**가 main 인지 Annex A 인지 README/§6 에서 재확인 필요.

### F-oct-prelim-4 진입 권고

**권고**: 진입 승인. 다만 가설 G3 단독이 아닌 다음 3축 동시 추적:

1. **silence-input 디코딩에서 음수 출력 생성 메커니즘 추적** —
   sf0 sample 5..7 에서 want 가 음수가 되는 chain 단계 (postfilter ringing /
   hpFilter startup transient / synthesis filter memory) 확인.
   특히 hpFilter 의 zero-input 응답이 음수 감쇠항을 생성하는지 측정.

2. **PST 생성 절차 verbatim 추적** — Annex A README 의 `decoder file.bit file.pst`
   가 어느 디코더 binary 인지 (Annex A 인지 main 인지) 재확인.
   → ITU-T G.729 Software Package Release 2/3 README (testdata 디렉토리 외부)
   에서 단일 출처 확인.

3. **회귀 가드 promotion 기각** — TEST/LSP 의 정합 (3/3) 은 trivial-zero 일치
   이므로 회귀 가드로 promotion 시 **위양성 검증력**을 만들 위험. promotion 금지.

---

**작성 완료**. 다음 단계: F-oct-prelim-4 (3축 추적) 또는 plan §Task 4 진입.
