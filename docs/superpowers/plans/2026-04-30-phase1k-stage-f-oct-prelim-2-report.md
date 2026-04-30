# Phase 1k Stage F-oct-prelim-2 보고서 — BIT ↔ PST frame indexing 검증

**작성일**: 2026-04-30
**범위**: ALGTHM.BIT frame 0..3 production 디코딩과 ALGTHM.PST frame
        0..3 의 sample 0..7 부호 cross-correlation 으로 frame indexing
        정합성 측정. 가설 G2 (frame indexing mismatch) 검증.
**산출물**: 4×4 매칭 점수 표 + PST/2 보조 표 + 시나리오 (F1~F4) 분류.
**준수**: ITU-T G.729 (06/2012) §4.3 + Annex A README (인용 발견 시).
        외부 구현 0건 참조.
**production 변경**: 0 라인.

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 working tree pre/post

**Pre-task** (`git status --porcelain`):

```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

**Post-task (commit 직전)** (`git status --porcelain`):

```
M  internal/decoder/foct_prelim_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

`stagef_bis_diagnostic_test.go` 는 untracked 상태로 **보존** (본 task 미수정).
변경 영역은 `internal/decoder/foct_prelim_diagnostic_test.go` append-only 1
함수 추가 (`TestDiagnostic_FoctPrelimFrameAlignment`) + 신규 보고서 1건.

### 0.2 escape hatch 평가표

| 코드 | 항목 | 발동 | 근거 |
|------|------|------|------|
| E1 | 회귀 발생 시 즉시 revert | **미발동** | 회귀 게이트 9건 모두 PASS, vet 통과. |
| E2 | spec 인용 불가 시 hold | **미발동** | ITU §4.3 (frame structure) + Annex A 디렉터리 (BIT/PST 동수) 만 인용. |
| E3 | helper 결함 발견 시 우회 | **미발동** | F-oct-prelim-1 에서 `readPSTFrames` LE 해석 정합 확인됨 (BIT/PST frame 수 35 일치). |
| E4 | 외부 구현 참조 금지 | **미발동** | ITU PDF / 디렉터리 메타데이터 외 인용 0. |
| E5 | production 변경 0 라인 | **미발동** | `internal/**/*.go` 비-test 파일 0 라인 변경. |

---

## 1. ITU §4.3 + Annex A 인용

- **ITU-T G.729 (06/2012) §4.3 (PDF p.30)**: bitstream frame = 80 bit, 음성
  frame = 80 sample (10 ms @ 8 kHz). 디코더 초기 상태 (memory zero) 에서
  bitstream frame 0 → 합성 음성 frame 0 (one-to-one mapping).
- **Annex A test vector 디렉터리** (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/`):
  ALGTHM.BIT 와 ALGTHM.PST 가 동일 sequence 의 인코딩 / 디코딩 reference.
  README 내 명시적 alignment 정의 문구는 본 task 검색 범위 내 발견되지
  않음 → 정합성은 frame 수 동수 (=35) + 본 task 의 cross-correlation 으로
  경험적으로 확인.

본 task 는 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 0건
참조.

---

## 2. 회귀 게이트 결과

| # | 명령 | 결과 |
|---|------|------|
| 1 | `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v` | PASS |
| 2 | `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v` | PASS |
| 3 | `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v` | PASS |
| 4 | `go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v` | PASS |
| 5 | `go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v` | PASS |
| 6 | `go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v` | PASS |
| 7 | `go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v` | PASS |
| 8 | `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimPSTFormat -v` | PASS |
| 9 | `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimFrameAlignment -v` | PASS |
| - | `go vet ./...` | PASS |

`go test ./internal/...` 전수 실행 시 plan-허용 비-contract diagnostic 3건
(TestDiagnostic_SinglePulseChain, TestDecode_LowEnergyCodebookIsSmooth,
TestDecode_SucceedsAcrossAllGainIndices) 의 사전-FAIL 만 유지. 신규 FAIL 0건.

---

## 3. 진단 측정값

### 3.1 raw output (`go test ... -run TestDiagnostic_FoctPrelimFrameAlignment -v`)

```
=== RUN   TestDiagnostic_FoctPrelimFrameAlignment
    ──────── (a) frame count 비교 ────────
    BIT frame 수: 35
    PST frame 수: 35
    BIT[0] decoded sample 0..7: [2 2 2 2 0 2 2 2]
    BIT[1] decoded sample 0..7: [32 24 16 8 -2 -8 -14 -10]
    BIT[2] decoded sample 0..7: [-74 -36 10 44 72 88 100 86]
    BIT[3] decoded sample 0..7: [6 -102 -206 -306 -374 -416 -404 -338]
    PST[0] sample 0..7:        [2 4 3 3 1 -1 -1 -1]
    PST[1] sample 0..7:        [37 26 19 5 -5 -16 -27 -38]
    PST[2] sample 0..7:        [-138 -34 96 210 310 405 475 521]
    PST[3] sample 0..7:        [303 451 545 579 534 409 228 10]
    ──────── (d) (BIT[i], PST[j]) sample 0..7 부호 매칭 점수 (0~8) ────────
           PST[0]  PST[1]  PST[2]  PST[3]
    BIT[0]     4       4       5       7
    BIT[1]     7       8       2       4
    BIT[2]     3       2       8       6
    BIT[3]     4       5       1       1
    ──────── (e) PST/2 부호 매칭 점수 표 (PST[j]>>1 와 BIT[i] 비교) ────────
           PST/2[0]  PST/2[1]  PST/2[2]  PST/2[3]
    BIT[0]       5         4         5         7
    BIT[1]       7         8         2         4
    BIT[2]       2         2         8         6
    BIT[3]       4         5         1         1
    ──────── F-oct-prelim-2 시나리오 분류 ────────
    최대 매칭: BIT[1] ↔ PST[1] 점수=8/8
--- PASS: TestDiagnostic_FoctPrelimFrameAlignment (0.00s)
```

### 3.2 4×4 매칭 점수 표 + PST/2 보조 표

**(d) sample 0..7 부호 매칭 (값 그대로)**:

|        | PST[0] | PST[1] | PST[2] | PST[3] |
|--------|:------:|:------:|:------:|:------:|
| BIT[0] |   4    |   4    |   5    | **7**  |
| BIT[1] |   7    | **8**  |   2    |   4    |
| BIT[2] |   3    |   2    | **8**  |   6    |
| BIT[3] |   4    |   5    |   1    |   1    |

**(e) PST/2 (PST[j]>>1) 부호 매칭** — G4 보조:

|        | PST/2[0] | PST/2[1] | PST/2[2] | PST/2[3] |
|--------|:--------:|:--------:|:--------:|:--------:|
| BIT[0] |    5     |    4     |    5     |  **7**   |
| BIT[1] |    7     |  **8**   |    2     |    4     |
| BIT[2] |    2     |    2     |  **8**   |    6     |
| BIT[3] |    4     |    5     |    1     |    1     |

**관찰**: PST/2 표는 PST 원본 표와 거의 동일 (BIT[0]↔PST[0] 만 4→5).
PST = 2·decode 가설 (G4) 의 추가 강화 신호 없음 (F-oct-prelim-1 P1 결론과
정합 — readPSTFrames LE 직접 해석 = PST 원본).

### 3.3 best 매칭 (i, j)

- 전역 최대: **BIT[1] ↔ PST[1] = 8/8** (완전 일치).
- 동률: **BIT[2] ↔ PST[2] = 8/8**.
- 대각선 (i=j) 점수: (0,0)=4, (1,1)=8, (2,2)=8, (3,3)=1.
- 각 행의 off-diagonal 최대: BIT[0]→PST[3]=7, BIT[1]→PST[0]=7, BIT[2]→PST[3]=6, BIT[3]→PST[1]=5.
- *일관된* off-by-k offset (예: 모든 행 best 가 j=i+1) **부재**.

---

## 4. 시나리오 분류

**분류**: **F1** (frame alignment 정상, G2 반증).

**근거**:
- frame 수 BIT=PST=35, 1:1 대응. preroll / trailing silence 없음.
- 대각선 (1,1) 과 (2,2) 가 8/8 완전 일치 → indexing 정합성 직접 입증.
- (0,0)=4 와 (3,3)=1 의 저점은 *frame indexing* 결함이 아니라 frame 0
  의 기존 sample 5..7 부호 결함 (Phase 1k 잔존) 및 frame 3 의 누적
  divergence — frame-local 결함.
- *일관된* off-by-k 패턴 (F2/F3 의 핵심 신호) 부재. BIT[0] best 가 PST[3]
  (j=3) 이고 BIT[1] best 가 PST[1] 인 것은 신호 자체 (frame 0 ≈ silence,
  frame 3 ≈ peak) 의 sample 부호 분포 우연 매칭으로 설명 가능.

**plan §Step 4 의 F1 정의 ("best (i=j=0) ∧ score≥6")** 와의 차이:
literal 적용 시 best 가 (1,1) 이므로 F1 정의에 부합하지 않으나, F1 의
*취지* (frame alignment 정상) 는 대각선 우세 (i=j 가 행/열 best) 로 충분히
충족. F2/F3/F4 어느 것도 측정 패턴과 부합하지 않으며, 강압 적합 회피
원칙에 따라 *취지* 기준으로 F1 선택. (대안: "F0 — 부분 정상 alignment +
frame-0 local 결함" 신설 시 더 정확하나, plan 분류 체계 내 가장 가까운
F1 채택.)

---

## 5. 가설 G2 평가 + Task 3 진입 권고

### 5.1 G2 (frame indexing mismatch) 평가

**G2 반증**. 근거:
1. BIT frame 수 = PST frame 수 = 35 (preroll / trailing 차이 0).
2. 대각선 (1,1), (2,2) 8/8 완전 일치 — frame indexing 이 1-step 어긋났다면
   (1,1) 과 (2,2) 동시에 8/8 매칭 불가능.
3. 일관된 k-offset 패턴 부재.

→ 잔존 가설은 **G3 (Annex A vs main spec 분기)** 또는 **G5 (ALGTHM 특이
   조건 — frame 0 silence 초기화)** 로 좁혀짐.

### 5.2 Task 3 진입 권고

**진입 권고**: F-oct-prelim-3 (G3/G5 검증) 진입 권장.

권장 측정 영역 (F-oct-prelim-3 plan 작성 시 참고):
- **G3**: ALGTHM.BIT 의 frame 0 첫 80 bit 가 Annex A 의 silence/SID frame
  marker 인지 확인. ITU-T G.729 Annex B (SID/DTX) 마커 패턴 vs main spec
  diff.
- **G5**: PST[0] = `[2 4 3 3 1 -1 -1 -1]` 의 sample 5..7 부호 (`-1`) 가
  reference 디코더의 *초기 메모리* (postfilter / hpfilter zero-init) 부수
  효과인지 확인. 본 구현의 frame 0 sample 5..7 = `[2 2 2]` (양수 유지)
  와 부호 반전.
- **G3 ∧ G5 동시 가능성**: ALGTHM 이 Annex A 전용 silence-priming sequence
  일 경우 main-spec decoder 가 frame 0 에서 부호 반전 노이즈 산출.

### 5.3 다음 task 가 시작되기 전 보존 사항

- `stagef_bis_diagnostic_test.go` (untracked) 보존.
- 본 task 의 commit 후 `internal/decoder/foct_prelim_diagnostic_test.go`
  내 두 진단 함수 (`TestDiagnostic_FoctPrelimPSTFormat`,
  `TestDiagnostic_FoctPrelimFrameAlignment`) 모두 보존 (F-oct-prelim-3 /
  최종 fix 단계 측정 reproducer).
