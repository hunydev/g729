# Phase 1k Stage F-oct-prelim-5-1 보고서 — PST 출처 verbatim + BIT 3-vector 비교

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (1) + (4). PST 생성 binary 식별 (Annex A vs main G.729) + ALGTHM/PITCH/FIXED frame 0 raw BIT 3-way diff.
**산출물**: README header dump + 6 vector byte-level 동일성 표 + frame 0 BIT 10-byte 3-way diff 표 + (P-SRC-?, B-CMP-?) 분류.
**준수**: Annex A `READMETV.txt` + main G.729 `READMETV.txt` 인용. ITU Software Package Release 2 (November 2006) header 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

---

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)

### 0.1 Working tree pre-state (Step 1 직전)

```
?? internal/decoder/stagef_bis_diagnostic_test.go
HEAD = 8af40ff docs(plans): add Phase 1k Stage F-oct-prelim-5 diagnostic-only cycle plan
```

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) — 미변경 보존 ✅
- `internal/lsp/lsp_lp.go` — `git diff --stat` 결과 변경 0, `git status` 미표시 ✅ (commit `02bf785` 후 정식화 상태 유지)
- 기 committed `*_test.go` (stagef_quart/sext/sept/foct_prelim) — 변경 0 ✅

### 0.2 Working tree post-state (Step 7 직전, commit 직전)

```
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/stagef_octprelim5_diagnostic_test.go
```

- 신규 untracked 2건 (본 task 산출물) + 사전 보존 1건 (stagef_bis) = 정확히 3건
- `git diff -- internal/` production diff = 비어있음 (0 라인) ✅

### 0.3 Escape hatch 평가표

| Hatch | 정의 | 본 task 발동 여부 | 근거 |
|-------|------|------------------|------|
| E1 | 회귀 게이트 신규 FAIL 발생 시 즉시 revert | ❌ 미발동 | 회귀 게이트 baseline 10건 + 신규 2건 모두 PASS. 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 만 plan-허용 FAIL 유지. |
| E2 | 측정 데이터로 plan 가설 반증 시 plan 수정 | ❌ 미발동 | 본 task 는 측정 only — 가설 분기 결정만 수행, plan 변경 없음. |
| E3 | 신규 진단 test 가 production 코드 의존 시 helper 추출 보류 | ❌ 미발동 | 신규 test 2건 모두 testdata I/O + 기존 testdata helper (`vectorPath`, `ensureTestdataPresent`, `readG192Frames`) 만 사용. |
| E4 | 외부 G.729 구현 인용 시 즉시 중단 | ❌ 미발동 | ITU Annex A `READMETV.txt`, main G.729 `READMETV.txt`, ITU Software Package Release 2 (2006-11) header 만 인용. |
| E5 | production 변경 1+ 라인 시 즉시 revert | ❌ 미발동 | `git diff -- internal/` 결과 변경 0 라인. 신규 파일은 `*_test.go` 한 건. |

---

## 1. README 인용 (Annex A + main G.729)

### 1.1 Annex A `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` (line 1..21)

```
 1| Testvectors to verify correct execution of G.729A ANSI-C software
 2| Version 1.1
 3|
 4| This directory contains testvectors to validate the correct execution
 5| of the G.729A ANSI-C software (version 1.1). NOTE that these vectors
 6| are not part of a validation procedure. It is very difficult to design
 7| an exhaustive set of test vectors. Hence passing these vectors should
 8| be viewed as a minimum requirement, and is not a guarantee that the
 9| implementation is correct for every possible input signal.
10|
11| Format: all files contain 16 bit sampled data using the Intel (PC)
12| format.
13|
14| *.in  - input files
15| *.bit - bit stream files
16| *.out - output files
17|
18| and were obtained using the following commands
19|
20|  coder file.in file.bit
21|  decoder file.bit file.pst
```

### 1.2 main G.729 `testdata/itu/G729_Release3/g729/test_vectors/READMETV.txt` (line 1..21)

```
 1|  ITU-T G.729 Software Package Release 2 (November 2006)
 2| Testvectors to verify correct execution of G.729 ANSI-C software
 3| Version 3.3
 4|
 5| This directory contains testvectors to validate the correct execution
 6| of the G.729 ANSI-C software (version 3.3). NOTE that these vectors
 7| are not part of a validation procedure. It is very difficult to design
 8| an exhaustive set of test vectors. Hence passing these vectors should
 9| be viewed as a minimum requirement, and is not a guarantee that the
10| implementation is correct for every possible input signal.
11|
11| Format: all files contain 16 bit sampled data using the Intel (PC)
12| format.
13|
14| *.in  - input files
15| *.bit - bit stream files
16| *.out - output files
17|
18|
19| and were obtained using the following commands
20|
21|  coder file.in file.bit
```

(주: Annex A README 의 "Software Package Release 2 (November 2006)" header 는 line 22 (본 dump 범위 외) 에 위치 — verbatim test 출력 line 22 동일성 확인 완료. main README 는 동일 header 가 line 1 에 위치. 두 README 모두 동일 release 산출물임을 명시.)

### 1.3 세 출처의 핵심 공통점 (인용 종합)

- 두 README 모두 **`decoder file.bit file.pst`** 명령으로 PST 가 생성됨을 명시 (Annex A line 21, main line 22 — 본 dump 외).
- 두 README 모두 **"Intel (PC) format" 16-bit** 명시 (Annex A line 11, main line 11).
- 두 README 모두 **"ITU-T G.729 Software Package Release 2 (November 2006)"** header 동일 (동일 release 의 Annex A binary 와 main G.729 binary).
- Annex A `decoder` binary 는 §A.4.2 simplified postfilter 적용. main `decoder` binary 는 §4.2 full postfilter 적용. 본 구현 (G.729A only) 의 ground-truth 는 `g729AnnexA/test_vectors/`.

---

## 2. 회귀 게이트 baseline (Step 1 출력)

10건 모두 `PASS` (cached run, `0.014s`):

| # | Test | Result |
|---|------|--------|
| 1 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| 2 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 3 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 4 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 5 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 6 | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 7 | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 8 | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 9 | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |
| 10 | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS |

---

## 3. 진단 측정값

### 3.1 raw output (Step 3)

#### 3.1.1 `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` — byte-level diff 표

```
[ALGTHM.BIT] Annex A vs main MISMATCH  Annex A=5740 byte  main=5740 byte
[ALGTHM.BIT] mismatch byte count = 643   (first diff offset = 40)
[ALGTHM.PST] Annex A vs main MISMATCH  Annex A=5600 byte  main=5600 byte
[ALGTHM.PST] mismatch byte count = 4662  (first diff offset = 142)
[PITCH.BIT]  Annex A vs main MISMATCH  Annex A=300940 byte  main=300940 byte
[PITCH.BIT]  mismatch byte count = 40334  (first diff offset = 44)
[PITCH.PST]  Annex A vs main MISMATCH  Annex A=293600 byte  main=293600 byte
[PITCH.PST]  mismatch byte count = 251135 (first diff offset = 82)
[FIXED.BIT]  Annex A vs main MISMATCH  Annex A=19680 byte  main=19680 byte
[FIXED.BIT]  mismatch byte count = 1562  (first diff offset = 46)
[FIXED.PST]  Annex A vs main MISMATCH  Annex A=19200 byte  main=19200 byte
[FIXED.PST]  mismatch byte count = 11436 (first diff offset = 80)
```

요약: **6/6 MISMATCH** (BYTE-EQUAL = 0/6). 두 tree 의 동일 파일명 6건 모두 길이는 같으나 내용은 분기. 첫 차이 offset 은 모두 40~142 범위 (G.192 frame 1 의 sync/length header 직후 첫 frame body 위치 부근).

#### 3.1.2 `TestDiagnostic_FoctPrelim5BitVectorCompare` — frame 0 raw bytes + 3-way diff

```
ALGTHM frame 0: e9 88 00 a0 00 fa c2 bf b7 e2
PITCH  frame 0: a9 2e 8a 60 00 fa dd 15 76 2c
FIXED  frame 0: 18 48 8a 60 00 fa dd 0f c4 2d

[0] ALGTHM=e9 PITCH=a9 FIXED=18   //
[1] ALGTHM=88 PITCH=2e FIXED=48   //
[2] ALGTHM=00 PITCH=8a FIXED=8a   PF
[3] ALGTHM=a0 PITCH=60 FIXED=60   PF
[4] ALGTHM=00 PITCH=00 FIXED=00   ==
[5] ALGTHM=fa PITCH=fa FIXED=fa   ==
[6] ALGTHM=c2 PITCH=dd FIXED=dd   PF
[7] ALGTHM=bf PITCH=15 FIXED=0f   //
[8] ALGTHM=b7 PITCH=76 FIXED=c4   //
[9] ALGTHM=e2 PITCH=2c FIXED=2d   //
```

집계:

| mark | 의미 | count |
|------|------|-------|
| `==` | 3-way 동일 | **2/10** (offset 4, 5) |
| `AP` | ALGTHM=PITCH ≠ FIXED | 0/10 |
| `AF` | ALGTHM=FIXED ≠ PITCH | 0/10 |
| `PF` | PITCH=FIXED ≠ ALGTHM | **3/10** (offset 2, 3, 6) |
| `//` | 3-way 모두 상이 | **5/10** (offset 0, 1, 7, 8, 9) |

(주: G.192 packed-bitstream byte 단위 비교 — 각 byte 는 8 bit packed parameter field 의 일부. byte 단위 동일성은 *해당 packed 영역 전체* 의 parameter equivalence 를 의미하지만, 단일 bit 차이도 byte mismatch 로 나타나므로 byte-level 동일성은 충분조건.)

### 3.2 PST 출처 분류 (P-SRC-1 / P-SRC-2)

→ **(P-SRC-2)** 채택. 6/6 vector 모두 Annex A 와 main MISMATCH. PST 생성 binary 가 분기. 본 구현 (G.729A) 의 ground-truth 는 `g729AnnexA/test_vectors/` (Annex A binary 출력). main G.729 의 분기는 *full postfilter (§4.2)* 의 후속 영향이며 본 cycle 영역 외.

### 3.3 BIT 3-way 분류 (B-CMP-1 / B-CMP-2)

→ **(B-CMP-2)** 채택. 3-way 동일 byte = 2/10 만 (offset 4, 5 — packed 영역의 일부 zero/공통 bit-field 추정). 5/10 byte 가 3-way 모두 상이. 세 vector 의 frame 0 BIT 는 상이한 stimulus 임이 정량 확인. 따라서 F-oct-prelim-3 §5 의 *세 vector 동상 sample 5..7 = 0/3 부호 반전* 은 **공통 silence stimulus 가설로는 설명 불가**. 동상 결함은 *디코더 내부의 공통 메커니즘* 에 기인. G3 (Annex A vs main 분기 거동) 영역에서 분기 위치는 **디코더 내부** 임을 함의.

---

## 4. 결합 분류 (P-SRC × B-CMP) 와 G3 함의

**결합 분류**: **(P-SRC-2, B-CMP-2)** — 단일 row, 측정 모순 없음.

함의:

1. **(P-SRC-2)**: ground-truth = Annex A `test_vectors/`. 본 구현이 `g729AnnexA/test_vectors/ALGTHM.PST` 와 mismatch 를 보일 경우 책임은 *우리 디코더 내부* (Annex A binary 와 동일 산출물을 내야 함). main G.729 test vector 와의 mismatch 는 G3 의 직접 증거가 *아니다* — 두 binary 가 다른 알고리즘 경로 (full vs simplified postfilter) 를 사용하기 때문. 따라서 G3 ("Annex A vs main 분기 거동") 가설은 *우리 구현이 어느 binary 의 거동에 더 가까운가* 의 비교가 아니라, *우리 구현이 Annex A binary 의 거동과 어디에서 분기하는가* 의 위치 식별 문제로 재정의된다.

2. **(B-CMP-2)**: ALGTHM/PITCH/FIXED 의 frame 0 BIT 가 7/10 byte 에서 분기 (3-way 동일 = 2/10, PF 만 일치 = 3/10). 따라서 세 vector 가 sample 5..7 에서 *동상 0/3 부호 반전* 을 보이는 현상은 stimulus 우연 일치로 설명되지 않는다. **공통 메커니즘 후보**:
   - (a) postfilter (§A.4.2 LT/ST/AGC chain) 의 초기 상태 (state vector 미초기화 혹은 hpFilter init) — F-oct-prelim-5-2 의 hpFilter init 가설과 정합.
   - (b) 디코더 첫 frame 의 LSP/excitation 초기 history (zero-state) 처리 분기.
   - (c) ALGTHM/PITCH/FIXED encoder 가 *frame 0 에 한해* parity/silence 영역의 동일 bit-field 를 채택 (offset 4, 5 의 `00 fa` 3-way 일치 + offset 2, 3, 6 의 PF 일치) — 가능하나 전체 frame 일치는 아님.

3. **G3 잔존 가설 정량 약화**: F-oct-prelim cycle (commit `06a4205`) 종결 시점의 G3 "약한 우세" 는 본 측정으로 **분기 위치를 디코더 내부로 좁힘**. 외부 stimulus (BIT) 분기가 확인되었음에도 출력 측 동상 결함이 발현한다는 사실은, 디코더가 *입력 차이를 흡수하는 내부 기제* (예: 첫 frame state init zero-flush, postfilter LT lag clamp, AGC gain ramp) 를 가지며 그 기제가 결함의 진원지임을 시사. 단, 본 task 단독으로 (a)/(b)/(c) 후보를 구분하지 못하므로 후속 task 로 위임.

---

## 5. F-oct-prelim-5-2 / F-oct-prelim-5-3 진입 권고

### 5.1 F-oct-prelim-5-2 즉시 진입 권고 (우선순위 1)

**근거**: §4 함의 (a) — hpFilter init state 가설은 plan §F-oct-prelim-5-2 의 기존 가설과 정합. 본 task 의 (B-CMP-2) 측정은 *frame 0 에서 디코더 출력 측 공통 결함이 stimulus 분기에도 불구하고 발현* 함을 정량 확인하므로, hpFilter init state (혹은 등가 첫 frame zero-history flush) 가 가장 유력한 단일 후보.

→ F-oct-prelim-5-2 진입 시 본 보고서 §3.1.2 의 frame 0 BIT byte 표를 *baseline* 으로 인용하여 hpFilter init 변경 전/후의 sample 5..7 거동 차이 측정 권고.

### 5.2 F-oct-prelim-5-3 (LP/excitation init zero-history 분기 식별) 후속 진입 권고 (우선순위 2)

**근거**: §4 함의 (b). hpFilter 가설이 (5-2 에서) 부분 반증될 경우 LP/excitation 초기 history 처리 분기 (synth IIR zero-state vs ITU 참조 spec 의 ringing residue) 가 차순위 후보.

### 5.3 본 task 단독 결론 (G3 가설의 위치)

- G1, G2, G5: F-oct-prelim cycle 에서 반증 (commit `06a4205`).
- G3: **약한 우세 → "분기 위치 = 디코더 내부 공통 메커니즘"** 으로 강화 (본 task 측정).
- G4: 부분 반증 유지.

본 task 는 G3 를 *반증* 하지 않고 *재정의* — 분기 위치를 디코더 내부로 좁히는 첫 정량 증거를 제공.

---

**End of report.**
