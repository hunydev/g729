# Phase 1k Stage F-oct-prelim-4 종합 보고서 + F-oct 권고

**작성일**: 2026-04-30
**범위**: F-oct-prelim-1 (PST file format), F-oct-prelim-2 (BIT↔PST
        frame indexing), F-oct-prelim-3 (6 ITU vector cross-check)
        의 진단 결과 결합 분석. 가설 G1~G5 중 결정적 위치 식별 +
        F-oct cycle (production fix / plan-end / 추가 진단) 권고.
**산출물**: 측정값 결합 표 + 가설 G1~G5 최종 평가 + F-oct 권고
            단일 결정 + 잔여 보류 항목 갱신.
**준수**: ITU-T G.729 (06/2012) PDF + Annex A `READMETV.txt` +
        F-oct-prelim-1/2/3 + F-sept-4 + F-sext-1 + F-quart/F-quint
        보고서만 인용. **외부 G.729 구현 (참조 C / bcg729 /
        Sipro Lab / FFmpeg) 0건 인용**.
**production 변경**: 0 라인. **테스트 변경**: 0 라인 (메타 task —
        본 보고서 1 파일만 추가).

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 Working tree pre/post

**Pre-task** (HEAD = `51e74e2`, F-oct-prelim-3 commit 직후):

```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

**Post-task (commit 직전)**:

```
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md
?? internal/decoder/stagef_bis_diagnostic_test.go
```

`stagef_bis_diagnostic_test.go` (사전 보유 untracked) — **미변경 보존
확인**. 본 task 신규 산출물 = 보고서 1건. production / test 코드
0 라인 변경.

### 0.2 Escape hatch 종합 평가표 (F-oct-prelim cycle 전체)

| Hatch | 정의 | F-oct-prelim cycle 발동 여부 | 근거 |
|-------|------|------------------------------|------|
| **E1** | 회귀 게이트 PASS 깨짐 | **미발동** | F-oct-prelim-1/2/3 + 본 task 모두 회귀 게이트 PASS. 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) FAIL 은 plan-허용 (F-quint-3 §4.6 동상). |
| **E2** | 진단 결과가 plan 핵심 가설을 뒤집어 후속 task 무의미 | **부분 발동 (재정의)** | F-oct-prelim-3 의 V3 표면 분류는 §3.3 보조 분석으로 V2 본질 재해석 (silence-saturation trivial 위장). plan §Step 3 결정 표의 (P1, F1, V2) row 매핑 — plan 자체 유지. |
| **E3** | 측정값 부족 / 재현 불가 | **변형 발동 (E3 변형, F-sept-4 와 동일)** | 측정 모두 재현 가능 PASS. 단 가설 G5 (ALGTHM 특이) 가 PITCH/FIXED 동일 발현으로 *적극 반증*. 기 식별 가설 (G1/G2/G5) 모두 반증 — G3 만 잔존. |
| **E4** | 외부 G.729 구현 참조 | **미발동** | 인용 = ITU PDF + Annex A `READMETV.txt` + F-oct-prelim-1/2/3 + F-sept-4 + F-sext-1 + F-quart/F-quint 보고서. 외부 구현 0건. |
| **E5** | production 변경 발생 | **미발동** | F-oct-prelim cycle 전체 production 0 라인. 본 task doc-only. |

---

## 1. F-oct-prelim cycle commit 요약 (3 commit + 본 commit)

`git log --oneline -10` (HEAD = `51e74e2`):

```
51e74e2 test(decoder): add Stage F-oct-prelim-3 multi-vector mismatch scan
94ef154 test(decoder): add Stage F-oct-prelim-2 BIT↔PST frame indexing scan
5832294 test(decoder): add Stage F-oct-prelim-1 PST file format verification
8decbf8 docs(plans): add Phase 1k Stage F-oct-prelim PST/2 ground-truth verification plan
02bf785 fix(lsp): keep §3.2.6 F polynomial recurrence in exact int64 arithmetic
d6834b0 docs(plans): add Stage F-sept synthesis report + F-oct recommendation
353398d test(decoder): add Stage F-sept-3 synth.Filter IIR boundary trace
d61497d test(decoder): add Stage F-sept-2 LP Â(z) reference cross-check
48265cd test(decoder): add Stage F-sept-1 excitation u[5] decomposition harness
078b172 docs(plans): add Phase 1k Stage F-sept diagnostic-only cycle plan
```

본 cycle 의 직접 입력 commit 및 산출물:

| commit | task | 핵심 산출물 | 시나리오 분류 |
|--------|------|-------------|---------------|
| `5832294` | F-oct-prelim-1 | `TestDiagnostic_FoctPrelimPSTFormat` (LE / BE / scaling 검증) | **P1 (LE 정상)** — endian/scaling/stride 정상, G1 반증 |
| `94ef154` | F-oct-prelim-2 | `TestDiagnostic_FoctPrelimFrameAlignment` (BIT[0..3]↔PST[0..3] 4×4 매칭) | **F1 (alignment 정상)** — frame 0 만 결함, G2 반증 |
| `51e74e2` | F-oct-prelim-3 | `TestDiagnostic_FoctPrelimMultiVectorScan` (6 vector × sample 5..7) | **V3 표면 / V2 본질** — ALGTHM/PITCH/FIXED 일관 0/3 반전, G5 (ALGTHM 특이) 기각 |
| 본 task | F-oct-prelim-4 | 종합 보고서 + F-oct 권고 | (메타 task) |

---

## 2. 회귀 게이트 종합 결과 (9건 + `go vet`)

| # | Test | 결과 |
|---|------|------|
| 1 | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 2 | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |
| 3 | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS |
| 4 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| 5 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 6 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 7 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 8 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 9 | `TestDiagnostic_FseptLPReferenceCrossCheck` + `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |

추가:
- `go test ./internal/...` — 신규 FAIL **0**. 기존 plan-허용 비-contract
  diagnostic 3건 FAIL 유지 (`TestDiagnostic_SinglePulseChain`,
  `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`).
- `go vet ./...` — clean (출력 0 라인).

---

## 3. 시나리오 결합 분석 (Task 1 × Task 2 × Task 3) + 가설 G1~G5 매핑

### 3.1 본 cycle 측정값 좌표

- **Task 1 (PST format)**: **P1 (LE 정상)** 단일 확정 (F-oct-prelim-1 §4).
- **Task 2 (frame alignment)**: **F1 (alignment 정상)** 단일 확정 — frame 0
  만 결함, frame 1..3 정합 (F-oct-prelim-2 §4).
- **Task 3 (multi-vector)**: 표면 **V3 (mixed)**, 본질 **V2-등가** —
  TEST/LSP 의 정합 (3/3) 은 want=`[2 2 2 2 0 0 0 0]` silence trivial
  match. *want 가 nonzero negative 인 3 vector (ALGTHM, PITCH, FIXED)
  모두 0/3 반전 일관 결함* (F-oct-prelim-3 §3.3 / §4).

### 3.2 plan §Step 3 결정 표 매핑

plan 표 (line 928~939) row 직접 매칭:

| Task 1 | Task 2 | Task 3 | F-oct 권고 (plan 정의) |
|--------|--------|--------|------------------------|
| (P1) endian/scaling 정상 | (F1) alignment 정상 | (V3) mixed | **G5 우세 — ALGTHM stress test** → ALGTHM 우회 + 정합 vector 회귀 가드 + ALGTHM 의도 거동 별도 추구 |
| (P1) endian/scaling 정상 | (F1) alignment 정상 | (V2) 모든 vector 반전 | **G3 우세 — Annex A vs main spec hpFilter 거동 차이** → §4.2.2 hpFilter startup transient 재검토 |

본 cycle 측정은 **표면 V3** 이지만 §3.3 의 보조 분석 (silence-saturation
trivial 위장 + want 가 nonzero negative 인 3 vector 일관 0/3 반전) 으로
**본질 V2-등가** 로 재해석. 따라서 plan 표 (P1, F1, V2) row =
**G3 우세** 적용.

동시에 표면 V3 의 잔여 가능성 — TEST/LSP 의 trivial-zero 정합이 위장임을
인정한 후에도, 실제 silence-saturation 외 ALGTHM stress test 의도
거동은 `algthm` README 정의 ("conditional parts of the algorithm") 와
정합 — 본 보고서 §4 권고 결정 시 동시 고려.

### 3.3 가설 G1~G5 최종 평가

| 가설 | 정의 | 본 cycle 평가 | 근거 |
|------|------|---------------|------|
| **G1** | `readPSTFrames` PST 파일 해석 결함 (endian / stride / scaling) | **반증** | F-oct-prelim-1 §4: P1 (LE 정상) 단일 확정. README "Intel (PC) format" 인용 + LE / BE 양쪽 해석 + chain bit-exact 일치로 검증 (sample 0..4 LE = chain). |
| **G2** | BIT↔PST frame indexing offset (preroll / skip) | **반증** | F-oct-prelim-2 §4: F1 (alignment 정상). BIT[0]↔PST[0] best 매칭 + frame 1..3 정합. 일관 off-by-k 부재. |
| **G3** | Annex A vs main spec 분기 거동 차이 (hpFilter startup transient / 기타) | **잔존 — 약한 우세** | F-oct-prelim-3 §3.3 + §4: ALGTHM/PITCH/FIXED *모두* sample 5..7 nonzero negative 생성 실패. F-sept-3 (synth IIR) + F-sext-1 (4 stage [+,+,+,+]) 모두 spec 정합 — chain 내부 결함 0. 잔존 분기 후보 = hpFilter startup transient / postfilter init / pitch tracking init. **단, 본 cycle 데이터로 분기 위치 직접 식별 불가** (G3 자체 약한 우세). |
| **G4** | PST = 2·decode (PST/2) 산술 차이 | **반증 (sample 0..4 한정)** | F-oct-prelim-1 §4 (P4): PST/2 가설은 sample 4 floor mismatch + sample 0..3 값 단순 chain=PST 가 더 단순한 적합 → 약화. sample 5..7 잔여는 G3 영역으로 이관. |
| **G5** | ALGTHM vector 특이 거동 (silence frame 0 stress test) | **기각** | F-oct-prelim-3 §5: PITCH (pitch search stress) + FIXED (fixed codebook stress) 도 동일 sample 5..7 = 0/3 반전. ALGTHM-특이 0건. **G5 의 *vector-specific 가설* 부분 기각**. 단, "silence-input 에서 음수 출력 생성 메커니즘 부재" 라는 *공통 결함* 가설은 G3 로 흡수. |

**결론**: G1, G2, G5 (vector-specific) **반증/기각**. G4 sample 0..4 한정
**반증**, sample 5..7 잔여 → G3 흡수. **G3 단일 잔존**, 단 *분기 위치
직접 식별은 본 cycle 데이터로 불가능*.

---

## 4. F-oct 권고 방향 결정 (단일 결정 — 강압-적합 회피)

### 4.1 후보 비교

| 후보 | 정의 | 평가 |
|------|------|------|
| **(a) F-oct production fix candidate** | G3 추적 — §4.2.2 hpFilter startup transient + Annex A vs main 분기 후보 (post-filter init / pitch tracking init) 진단 + production fix | **부적합 — 강압-적합 위험**. F-sept-3 (synth IIR spec 정합) + F-sext-1 (hpFilter 출력 [+,+,+,+] spec 정합) 으로 chain 내부 5 stage 모두 정합 검증 완료. 분기 위치 식별 없이 production fix 시도 시 spec 정합한 stage 를 spec 위반으로 회귀시킬 위험. |
| **(b) plan-end declared** "결함 0 — ALGTHM stress test 가설 채택" | PST 가 reference encoder loopback 이므로 chain 외부 결함 위치 부재 인정 → plan-end | **부적합 — 측정 반증됨**. F-oct-prelim-3 가 *ALGTHM 특이 가설을 기각* (PITCH/FIXED 동일 발현) — "ALGTHM stress test" 명목으로 plan-end 시 multi-vector 일관 결함을 무시하는 강압-적합. |
| **(c) F-oct-prelim-5 추가 진단** | silence frame 0 의 reference encoder 출력 trace + hpFilter init state 측정 + Annex A vs main 디코더 binary 식별 | **적합**. G3 의 분기 위치 직접 식별을 위한 *측정 부족* 이 §3.3 의 핵심 결론. 추가 진단 없이 (a) 진입 시 fix 후보 ranking 불가 — production 변경 정당화 미달. |

### 4.2 단일 결정

**(c) F-oct-prelim-5 추가 진단** 채택.

**근거**:
1. G3 단일 잔존 가설이지만 *분기 위치 직접 식별 데이터 부재* (§3.3
   결론). production fix candidate (a) 진입 시 chain 5 stage spec 정합
   회귀 위험 + 강압-적합 risk.
2. (b) plan-end 는 multi-vector 일관 결함 (ALGTHM/PITCH/FIXED 모두
   0/3 반전) 을 ALGTHM-특이로 강압 해석 — F-oct-prelim-3 §5 가
   *적극 반증*.
3. F-oct-prelim-5 의 측정 항목 (silence input → reference encoder PST
   trace + hpFilter init state) 은 모두 ITU PDF + Annex A README 내에서
   spec 인용 가능 → E4 invariant 준수 가능.

### 4.3 F-oct-prelim-5 측정 항목 권고 (plan 작성 시 출발점)

본 보고서는 plan 자체를 작성하지 않으나, 다음 cycle plan 작성 시
출발점:

1. **silence-input PST 출처 verbatim 추적** — Annex A `READMETV.txt`
   의 "*.out - output files" 가 어떤 디코더 binary (`decoder.exe` —
   Annex A 인지 main G.729 인지) 출력인지 ITU Software Package
   Release 2/3 README (testdata 외부) 단일 출처로 재확인.
2. **hpFilter init state 측정** — frame 0 sf0 진입 시 hpFilter 의
   IIR memory 초기 state (production 0 init vs spec/Annex A 비-0 init
   가능성) 를 spec § 인용으로 정량 검증.
3. **silence frame 0 의 negative output 생성 메커니즘** — want 의
   sample 5..7 = `-1, -1, -1` 을 만드는 chain 단계 식별 (postfilter
   ringing / hpFilter 음수 감쇠항 / synthesis filter memory 비-0
   init). spec § 인용 우선.
4. **ALGTHM/PITCH/FIXED 공통 BIT 입력 분석** — frame 0 의 BIT raw
   bits 가 세 vector 에서 동일/유사한지 (silence equivalent stimulus
   가 셋 모두에 적용되는지) 측정.

위 4 항목은 모두 production 0 변경 진단으로 수행 가능.

---

## 5. 잔여 보류 항목 갱신 (F-sept-4 §5 + 본 cycle 결과)

| # | 항목 | 직전 상태 (F-sept-4 §5) | 본 cycle 갱신 |
|---|------|-------------------------|---------------|
| 1 | **F-oct (production fix 또는 plan-end)** | ground-truth 검증 cycle (F-oct-prelim) 선행 | **갱신**: F-oct-prelim cycle 완료 — G1/G2/G5 반증, G4 sample 0..4 반증, **G3 단일 잔존 (약한 우세)**. F-oct production fix candidate 진입은 G3 분기 위치 식별 데이터 부족으로 강압-적합 위험 → **F-oct-prelim-5 추가 진단 cycle 권고** (§4.2). |
| 2 | **filterSubframe ÷4/×4** | F-quint-3 §4.1 동상 | 미갱신 (frame 0 sf0 미-trigger 동일). |
| 3 | **β init = 0.2** | F-quint-3 §4.2 동상 | 미갱신 (gp_q14=1995, beta_q14=3277 frame 0 sf0 동일). |
| 4 | **frame 1+ 잔여** | F-sept frame 0 sf0 한정 | **갱신**: F-oct-prelim-2 가 frame 1..3 alignment 정합 검증 — frame 1+ 의 sample 5..7 부호 잔여 결함 측정은 F-oct-prelim-5 또는 후속 cycle 에 흡수. |
| 5 | **회귀 가드 promotion** | sample 0..7 영구 게이트 후속 | **갱신**: F-oct-prelim-3 §5 — TEST/LSP 정합 (3/3) 은 silence trivial-zero 일치이므로 sample 5..7 영구 게이트 promotion 시 **위양성 검증력** 위험. **promotion 금지 강화**. |
| 6 | **비-contract diagnostic 3건** | F-quint-3 §4.6 동상 | 미갱신. F-oct-prelim cycle 추가 3건 (`TestDiagnostic_FoctPrelimPSTFormat/FrameAlignment/MultiVectorScan` — 모두 PASS 로 회귀 게이트 기여). cleanup task 별도. |
| 7 | **F-sext-2 / F-sext-3 (HP filter 진단)** | F-sept-3 (S1) IIR 정합으로 유보 강화 | **갱신 (재가동 검토)**: G3 단일 잔존 + hpFilter startup transient 가 §4.2.2 분기 후보로 재부상. F-oct-prelim-5 의 측정 항목 2 (hpFilter init state) 와 통합 가능. **F-sext-2/3 reactivate 검토 — F-oct-prelim-5 plan 작성 시 통합 결정**. |
| 8 | **lsp_lp.go uncommitted (F-bis-1 P fix)** | F-sept-2 시점 정식화 완료 (`02bf785`) | 미갱신 (F-sept-4 시점 종결). |
| 9 | **stagef_bis_diagnostic_test.go untracked** | 보존 유지 | 미갱신. F-oct-prelim cycle 4 task 모두 untracked 보존 확인. F-bis cycle 종결 시 commit 검토. |

---

## 6. 모순 발현 평가 (해당 없음)

plan §Step 3 의 모순 시나리오 (P1, F1, V1) row — "F-sept-4 회귀 의심"
**미발현**. 본 cycle 측정은 V1 (모든 vector 정합) 미관찰 → V3 표면
관찰. 회귀 게이트 9/9 PASS 로 F-sept-4 회귀 부재 직접 검증. E1/E3
회귀 발동 사유 0.

---

## 7. 결론 — Phase 1k Stage F-oct-prelim closure

### 7.1 F-oct-prelim cycle closure 평가

- **F-oct-prelim-1 (PST format)**: PASS, P1 단일 확정 (G1 반증).
- **F-oct-prelim-2 (frame alignment)**: PASS, F1 단일 확정 (G2 반증).
- **F-oct-prelim-3 (multi-vector)**: PASS, V3 표면 / V2 본질 (G5
  vector-specific 부분 기각, G4 sample 5..7 잔여 → G3 흡수).
- **F-oct-prelim-4 (본 종합 보고서)**: 가설 G1/G2/G5 반증, G4 부분
  반증, **G3 단일 잔존 (약한 우세)**. F-oct cycle 권고 = **(c)
  F-oct-prelim-5 추가 진단**.

Phase 1k Stage F-oct-prelim 모든 task **closure 가능**.

### 7.2 다음 cycle 진입 권고 (단일 결정)

**F-oct-prelim-5 추가 진단 cycle plan 작성** —

- diagnostic only, production 변경 0
- 측정 항목 = §4.3 의 4 항목 (PST 출처 verbatim, hpFilter init state,
  silence negative output 생성 메커니즘, ALGTHM/PITCH/FIXED 공통 BIT
  입력)
- F-sext-2/3 reactivate 통합 검토 (잔여 항목 #7)
- E4 invariant 준수 — 외부 G.729 구현 0 인용 유지

F-oct production fix cycle 또는 plan-end declared 는 F-oct-prelim-5
결과 산출 후 재평가.

### 7.3 invariant 종합 준수 (F-oct-prelim cycle 전체)

- **E1 미발동**: 회귀 게이트 9건 + F-oct-prelim task 3건 모두 PASS,
  비-contract diagnostic 3건 FAIL plan-허용 유지.
- **E4 미발동**: 외부 G.729 구현 0 인용. 모든 인용 = ITU PDF +
  Annex A README + 기 작성 보고서.
- **E5 미발동**: F-oct-prelim cycle 전체 production 0 라인. 본 task
  doc-only.
- 사전 working tree 보존: `?? stagef_bis_diagnostic_test.go` 4 task
  내내 미변경 보존 확인.

**Phase 1k Stage F-oct-prelim closure**.
