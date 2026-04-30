# Phase 1k Stage F-oct-prelim-1 보고서 — PST file format 해석 검증

**작성일**: 2026-04-30
**범위**: ALGTHM.PST binary format 의 endian / scaling / sample stride 정합성 측정. `readPSTFrames` helper 의 LittleEndian, 80 sample/frame 가정 검증. 가설 G1 (PST file 해석 오류) + G4 (PST/2 가설) 1차 데이터.
**산출물**: 파일 크기 / frame 0 raw byte hex / LE·BE 양쪽 해석 / scaling 후보 표 + 시나리오 분류.
**준수**: ITU-T G.729 (06/2012) PDF + ITU testdata `READMETV.txt` 만 인용. 외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 0 인용.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)

진입 시점 working tree:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```
HEAD = `8decbf8` (F-oct-prelim plan).

본 task 신규 파일:
- `internal/decoder/foct_prelim_diagnostic_test.go`
- `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md`

기존 *_test.go 0 변경. production (`internal/**/*.go` non-test) 0 라인 변경 (E5 invariant 준수).

| Escape hatch | 발동 여부 | 근거 |
|---|---|---|
| E1 (회귀) | **미발동** | 7 contract 게이트 PASS, 3 비-contract FAIL 은 plan 게이트 9 허용. |
| E2 (외부 spec 추가) | **미발동** | ITU PDF + Annex A `READMETV.txt` 만 인용. |
| E3 (시간 초과) | **미발동** | 단일 task 내 측정 완료. |
| E4 (외부 G.729 구현 인용) | **미발동** | 외부 구현 0 참조. |
| E5 (production 변경) | **미발동** | `git diff -- internal/` non-test 라인 = 0. |

## 1. PDF + README 검색 결과 (PST format 정의 인용)

### 1.1 ITU-T G.729 (06/2012) PDF
PDF 본문 검색 (`pdftotext G729E.pdf | grep -inE "\.pst|post.*processed|16.bit linear|test.*vector.*format"`) 결과:

| line | 인용 |
|---|---|
| 922 | "sampling it at 8 000 Hz, followed by conversion to 16-bit linear pulse code modulation (PCM) for ..." |
| 925 | "64 kbit/s PCM data, should be converted to 16-bit linear PCM before encoding, or from 16-bit ..." |
| 12587 | "inputfile: 8 kHz sampled data file containing 16-bit linear PCM signal;" |
| 12612 | "outputfile: 8 kHz sampled data file containing 16-bit linear PCM signal;" |

PDF 본문에 `.pst` 확장자 명시 정의 *없음*. encoder/decoder I/O 가 "16-bit linear PCM" 임은 §6 + cover commentary 에서 확인.

### 1.2 ITU testdata Annex A `READMETV.txt` (verbatim, format 정의 부분만)

`testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` 인용 (format 정의 부분만, 알고리즘/post-processing body 인용 0):

> Format: all files contain 16 bit sampled data using the Intel (PC)
> format.

> decoder file.bit file.pst

> Note that for some files only the *.bit and *.pst are available.

파일 인벤토리 (`READMETV.txt` 발췌):
```
    5600  algthm.pst
   48000  erasure.pst
   19200  fixed.pst
  357120  lsp.pst
   61440  overflow.pst
   48000  parity.pst
  293600  pitch.pst
  600000  speech.pst
   20480  tame.pst
```

**해석**:
- "16 bit sampled data using the Intel (PC) format" = **little-endian int16** (Intel 386 native byte order).
- `algthm.pst` 5600 byte = 35 frame × 160 byte = 35 frame × 80 sample × 2 byte. → 80 sample/frame (= 10 ms @ 8 kHz subframe×2) stride 정합.

`readPSTFrames` (testdata_helpers_test.go:28-29) 의 self-citing 주석 "raw 16-bit little-endian PCM file (ITU Annex A .pst format) ... 80-sample frames" 는 README 정의와 정합.

E4: README 인용은 *format 정의* (16-bit Intel PC byte order + 파일 크기) 한정. post-processing 알고리즘 body 인용 0.

## 2. 회귀 게이트 baseline (Step 1 출력) + 9건 결과

### 2.1 Phase 0.2 게이트 1~8 (commit 직전 + 직후 동일)

| # | test/cmd | 결과 |
|---|---|---|
| 1 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| 2 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 3 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 4 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 5 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 6 | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 7 | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 8 | `go vet ./...` | 무출력 |

집계 실행 (`go test ./internal/decoder/ -run '게이트1\|...\|게이트7'`): `ok github.com/exedev/g729/internal/decoder 0.007s`.

### 2.2 게이트 9 (비-contract diagnostic 3건 FAIL plan-허용)

`go test ./internal/...` 전체에서 잔존 FAIL:
- `internal/decoder/diagnostic_singlepulse_test.go` (BOUNDARY ⑩ gain saturate 14 dB suspect)
- `internal/decoder/...` 일부 비-contract diagnostic
- `internal/gain/pathological_test.go` (gcQ12 saturate sweep 진단)

Phase 0.2 게이트 9 정의에 따라 plan-허용. 본 cycle scope 외.

### 2.3 신규 test
- `TestDiagnostic_FoctPrelimPSTFormat`: **PASS** (assertion 0, t.Logf only).

## 3. 진단 측정값

### 3.1 raw output (Step 4)

```
=== RUN   TestDiagnostic_FoctPrelimPSTFormat
    foct_prelim_diagnostic_test.go:35: ──────── (a) ALGTHM.PST 파일 크기 분석 ────────
    foct_prelim_diagnostic_test.go:36: 총 byte: 5600
    foct_prelim_diagnostic_test.go:37: 가정 (80 sample/frame × 2 byte): bytesPerFrame = 160
    foct_prelim_diagnostic_test.go:38: nFrames (가정) = 35 (잉여 byte = 0)
    foct_prelim_diagnostic_test.go:44: ──────── (b) ALGTHM.PST frame 0 raw byte (160 byte = 80 sample × 2) ────────
    foct_prelim_diagnostic_test.go:54: [0000]  02 00 04 00 03 00 03 00 01 00 ff ff ff ff ff ff
    foct_prelim_diagnostic_test.go:54: [0010]  ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff
    foct_prelim_diagnostic_test.go:54: [0020]  ff ff ff ff ff ff ff ff ff ff ff ff 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0030]  00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0040]  00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0050]  00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0060]  00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0070]  00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
    foct_prelim_diagnostic_test.go:54: [0080]  00 00 00 00 00 00 00 00 00 00 0f 00 21 00 33 00
    foct_prelim_diagnostic_test.go:54: [0090]  33 00 34 00 33 00 33 00 2f 00 38 00 32 00 2b 00
    foct_prelim_diagnostic_test.go:57: ──────── (c) endian 양쪽 해석 (frame 0 sample 0..7) ────────
    foct_prelim_diagnostic_test.go:64: LittleEndian (현재 readPSTFrames 가정): [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:65: BigEndian (대안):                       [512 1024 768 768 256 -1 -1 -1]
    foct_prelim_diagnostic_test.go:67: ──────── (d) scaling 후보 (LittleEndian 기준) sample 0..7 ────────
    foct_prelim_diagnostic_test.go:68: 값 그대로 (×1):  [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:74: PST/2 (>>1):     [1 2 1 1 0 -1 -1 -1]
    foct_prelim_diagnostic_test.go:75: PST·2 (<<1):     [4 8 6 6 2 -2 -2 -2]
    foct_prelim_diagnostic_test.go:78: ──────── (e) chain vs PST 해석 부호 비교 (sample 0..7) ────────
    foct_prelim_diagnostic_test.go:79: chain output (F-sept-4): [2 4 3 3 1 1 1 1]
    foct_prelim_diagnostic_test.go:88:   [0]  chain=+2  LE=+2 (부호=)  LE/2=+1 (부호=)  BE=+512 (부호=)
    foct_prelim_diagnostic_test.go:88:   [1]  chain=+4  LE=+4 (부호=)  LE/2=+2 (부호=)  BE=+1024 (부호=)
    foct_prelim_diagnostic_test.go:88:   [2]  chain=+3  LE=+3 (부호=)  LE/2=+1 (부호=)  BE=+768 (부호=)
    foct_prelim_diagnostic_test.go:88:   [3]  chain=+3  LE=+3 (부호=)  LE/2=+1 (부호=)  BE=+768 (부호=)
    foct_prelim_diagnostic_test.go:88:   [4]  chain=+1  LE=+1 (부호=)  LE/2=+0 (부호≠)  BE=+256 (부호=)
    foct_prelim_diagnostic_test.go:88:   [5]  chain=+1  LE=-1 (부호≠)  LE/2=-1 (부호≠)  BE=-1 (부호≠)
    foct_prelim_diagnostic_test.go:88:   [6]  chain=+1  LE=-1 (부호≠)  LE/2=-1 (부호≠)  BE=-1 (부호≠)
    foct_prelim_diagnostic_test.go:88:   [7]  chain=+1  LE=-1 (부호≠)  LE/2=-1 (부호≠)  BE=-1 (부호≠)
    foct_prelim_diagnostic_test.go:96: ──────── (f) readPSTFrames 결과 vs LE 직접 해석 (frame 0 sample 0..7) ────────
    foct_prelim_diagnostic_test.go:97: readPSTFrames frame 0 sample 0..7: [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:98: LE 직접 해석            sample 0..7: [2 4 3 3 1 -1 -1 -1]
    foct_prelim_diagnostic_test.go:107: → readPSTFrames 출력 = LittleEndian 직접 해석 (예상대로)
--- PASS: TestDiagnostic_FoctPrelimPSTFormat (0.00s)
```

### 3.2 endian × scaling 측정 표

| 측정 | 결과 |
|------|------|
| 파일 크기 | 5600 byte |
| nFrames (160 byte stride) | 35 |
| 잉여 byte | 0 |
| LE sample 0..7 | `[+2, +4, +3, +3, +1, −1, −1, −1]` |
| BE sample 0..7 | `[+512, +1024, +768, +768, +256, −1, −1, −1]` |
| LE/2 sample 0..7 | `[+1, +2, +1, +1, 0, −1, −1, −1]` |
| LE·2 sample 0..7 | `[+4, +8, +6, +6, +2, −2, −2, −2]` |
| chain output | `[+2, +4, +3, +3, +1, +1, +1, +1]` |
| readPSTFrames = LE 직접 해석 | **true** |

### 3.3 readPSTFrames vs LE 직접 해석 일치성

frame 0 sample 0..7 의 8 sample 모두 bit-exact 일치. `readPSTFrames` helper 의 binary parse 무결.

### 3.4 부호/값 일치 분석

- **sample 0..3**: LE 해석이 chain 과 *값* 정확 일치 (`+2, +4, +3, +3`). PST/2 는 부호만 일치하되 값 1/2 (`+1, +2, +1, +1`).
- **sample 4**: LE = chain = `+1` (값 일치). PST/2 = `0` (부호 불일치 — `+1>>1 = 0`).
- **sample 5..7**: LE / BE / LE/2 모두 `−1`. chain 은 `+1`. **세 해석 모두 부호 mismatch**.

→ sample 0..4 *값* 일치는 LE int16 직접 해석에서만 발생. PST/2 는 sample 4 floor 로 부호 mismatch.

## 4. 시나리오 분류

**P1 (LE 정상)** 단일 확정.

근거:
- (P1) LE sample 0..4 모두 chain 부호 일치 + LE 가 readPSTFrames 출력과 bit-exact 동일 → endian/scaling/stride 정상. **선택**.
- (P2) BE sample 0..4 도 부호는 일치하지만 *값* 이 비현실적 (`+512, +1024, ...`) — 8 kHz 16-bit linear PCM post-filter 출력 스케일 (sample 0..3 절대값 ≤ 4) 와 부합 않음. 또한 README "Intel (PC) format" 명시 인용으로 **반증**.
- (P3) LE 해석에서 sample 0..4 *모두 값 일치* (LE = chain) — stride 어긋남 가능성 **반증**.
- (P4) PST/2 는 sample 4 부호 mismatch (`+1>>1 = 0` floor) + sample 0..3 값 (`+1,+2,+1,+1`) 이 chain 값 (`+2,+4,+3,+3`) 의 정확 1/2 — *부호* 는 일치하지만 chain 이 PST 의 정확 2 배. PST/2 가설 (chain 이 PST 의 정확 1 배여야 하는데 2 배라면 production 결함) 은 sample 4 floor mismatch 로 약화. PST = chain (sample 0..4) 이 더 단순한 적합.
- (P5) 결정 불가 — 미해당 (P1 단일 확정).

## 5. 가설 G1 / G4 1차 평가 + Task 2 진입 권고

### G1 (PST file 해석 오류): **반증**

`readPSTFrames` 의 LittleEndian + 80 sample/frame + int16 raw 해석이 README "Intel (PC) format" 정의와 정합. frame 0 sample 0..4 가 chain output 과 *값* 정확 일치 (`[+2, +4, +3, +3, +1]`). G1 은 본 cycle 후속 task 에서 추가 추구할 근거 없음.

### G4 (PST/2 가설): **약화 (G1 반증 + sample 4 floor mismatch)**

- LE 직접 해석에서 sample 0..4 가 chain 과 정확 값 일치 → PST 자체가 decode-target. PST/2 (chain = 2·PST) 는 sample 0..4 에서 부합 않음.
- 만약 PST·2 (chain = PST/2, 즉 PST 가 2 배 부풀려진 값) 를 가정하면 sample 0..4 부호는 일치하나 *값* 이 chain 의 2 배 — chain output 이 production 정합 (F-sept-4 검증) 임을 가정할 때 PST·2 는 modeling 만 가능하고 spec 근거 0.
- → G4 (PST/2 그대로 가설) 는 약화. 단 "PST 가 *별도 후처리* (예: gain 정규화 / re-encoding) 를 거친 second-pass output" 이라는 *재정식화된 G4'* 는 본 task 데이터로 배제 불가. Task 3 (다른 vector cross-check) 에서 분리.

### 잔존 mismatch (sample 5..7)

PST sample 5..7 = `−1` vs chain = `+1`. 세 endian/scaling 해석 모두 부호 mismatch. 본 task 범위 외 — **G2 (frame indexing) / G3 (Annex A vs main spec) / G5 (ALGTHM stress 의도) 의 추구 대상**.

특히 sample 4 (`+1`) 와 sample 5 (`−1`) 사이 부호 전환이 PST 에서만 발생 (chain 에서는 sample 4..7 모두 `+1` 일관). 가능 가설:
- (G2) frame 0 = PST 의 다른 frame index (skip/preroll).
- (G5) ALGTHM 은 stress test 로서 sample 5..7 의 부호 반전을 *의도적* 으로 유발 (예: pitch sign / impulse phase).

### Task 2 (F-oct-prelim-2) 진입 권고

**진입 가**. Task 2 의 ALGTHM.BIT ↔ ALGTHM.PST frame indexing / preroll 검증으로 G2 우선 추구. 본 task 결과 (LE 해석 정합 확정, sample 0..4 값 일치) 는 Task 2 의 frame alignment 기준선으로 직접 활용 가능 — frame 0 sample 0..4 = `[+2,+4,+3,+3,+1]` 가 어느 BIT frame index 의 decode 와 매치하는지 sweep.

## A. 보충: PST format 인용 PDF 발견 불가 메모

ITU-T G.729 (06/2012) PDF 본문에서 `.pst` 확장자의 명시 binary format 정의 (endian / sample stride / scaling) 는 발견되지 않음. PDF 는 "16-bit linear PCM" 의 일반 입출력 정의만 명시 (line 922, 925, 12587, 12612). 구체적 byte order 는 ITU testdata distribution 의 `READMETV.txt` "Intel (PC) format" 표기로 보충. README 인용은 format 정의 부분 (16 bit Intel PC + 파일 크기 인벤토리) 한정 — algorithm body 인용 0 (E4 회피).
