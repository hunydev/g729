# Phase 1k Stage F-oct-prelim Diagnostic-Only Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-sept-4 종합 결과 (chain 내부 5 stage — excitation `u[5]=0`, LP §3.2.6 정합 `max|Δ|=6`, IIR §3.10 정합 `max|Δ|=1`, postfilter, hpFilter — 모두 spec 정합) 후속. ALGTHM frame 0 sf0 sample 5..7 의 *chain output* `[+1,+1,+1]` 과 *PST want* `[−1,−1,−1]` (PST/2 spec-target) 의 mismatch 가 (G1) PST file 해석 / (G2) frame indexing / (G3) Annex A vs main spec / (G4) PST/2 가설 자체 무효 / (G5) ALGTHM stress test 의도 출력 중 어느 결함 위치에 기인하는지 **진단-only** 로 식별. 후속 production fix (또는 가설 폐기) 는 별도 cycle (F-oct 본편) 권고. **production 변경 0 라인** invariant.

**Architecture:** 4-task 진단 cycle (F-quart / F-sext / F-sept 패턴 답습). Task F-oct-prelim-1 = ALGTHM.PST binary format 해석 검증 (`readPSTFrames` helper 의 endian / scaling / sample stride 정합성). Task F-oct-prelim-2 = ALGTHM.BIT ↔ ALGTHM.PST frame indexing 검증 (frame skip / preroll 식별). Task F-oct-prelim-3 = 다른 ITU vector (TEST / SPEECH / LSP / PITCH / FIXED) cross-check — chain mismatch 패턴이 ALGTHM 특이성인지 일반 결함인지 분리. Task F-oct-prelim-4 = 종합 결합 분석 + F-oct (또는 cycle 종료) 권고 결정. 각 task production 코드 변경 0 라인 (E5 invariant); test 추가만 허용.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §B (Annex A test vector format / 정의) + §6 (test vector 정의) + §3.10 (synthesis filter) + §4.2.2 (postprocessing hpFilter) + Annex A §A.1 (저복잡도 변종 정의). 기존 진단 하니스 (`vectorPath`, `readG192Frames`, `readPSTFrames`, `ensureTestdataPresent` — `internal/decoder/testdata_helpers_test.go`) 재활용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-oct-prelim 진입 시점, post-`02bf785`)

| 경로 | 상태 | F-oct-prelim 변경? |
|------|------|------|
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis/F-tris 진단 baseline | **No** (보존, 별도 cycle) |
| `internal/lsp/lsp_lp.go` | committed `02bf785` — F-bis-1 P fix int64 누산 정식화 | **No** (변경 금지) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (F-quart/F-quint) | **No** (변경 금지) |
| `internal/decoder/stagef_sext_diagnostic_test.go` | committed `6f1c841` (F-sext-1) | **No** (변경 금지) |
| `internal/decoder/stagef_sept_diagnostic_test.go` | committed `d6834b0` (F-sept-1/2/3) | **No** (변경 금지) |
| 그 외 production 파일 | F-sept-4 시점 그대로 | **No** (진단-only) |

F-oct-prelim 신규 파일 (모두 *_test.go 또는 .md):
- (Task F-oct-prelim-1) `internal/decoder/foct_prelim_diagnostic_test.go` — 본 cycle 의 모든 진단 test 통합 파일 (Task 1/2/3 의 새 test 모두 본 파일에 누적 추가).
- (Task F-oct-prelim-1) `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md`
- (Task F-oct-prelim-2) `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-2-report.md`
- (Task F-oct-prelim-3) `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md`
- (Task F-oct-prelim-4) `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md`

본 cycle 의 production 변경 범위 = **0 라인**. test 변경 = `foct_prelim_diagnostic_test.go` 신규 파일 only. 그 외 *_test.go 파일 변경 절대 금지.

### Phase 0.2 회귀 게이트 명세

각 task commit 직후 *반드시* 실행. 9 게이트 PASS 의무:

1. `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v` — Phase 1i sample 0 가드 (F-quint-2 commit `1c00385` 후 PASS).
2. `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v` — F-quart-3 reference cross-check.
3. `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v` — F-quart-1 alignment harness.
4. `go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v` — F-sext-1 chain trace.
5. `go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v` — F-sept-1 excitation 분해.
6. `go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v` — F-sept-2 LP cross-check.
7. `go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v` — F-sept-3 IIR trace.
8. `go vet ./...` — 무출력 (clean).
9. **비-contract diagnostic 3건 FAIL 유지 plan-허용** — F-quint-3 §4.6 으로 정식화된 잔여 진단 (production fix 미완료) 의 FAIL 은 본 cycle 에서 *허용*. 단 게이트 1~7 의 PASS 와 분리해 보고서 §2 에 명시.

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1~8 중 1+ FAIL | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | Task 의 진단 측정값이 spec 식 verbatim 도출에서 분리됨 (= prod-동치 휴리스틱 fit 으로 결론 도출) | 즉시 결론 폐기 + 측정값 raw output 만 보고서에 기록 + 다음 task 로 미해결 이관 |
| **E3** | Task 1/2/3 결과가 *상호 모순* (예: Task 1 = 정상 / Task 3 = 모든 vector 결함 — PST format 정상인데 모든 vector mismatch) | 단일 결함 식별 실패. 시나리오 결합 분석 (Task 4 §3) 에 모순 명시 + F-oct 권고를 *복수 가설 동시 추적* 으로 갱신 |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | 본 cycle 의 임의 commit 가 production 파일 (`internal/**/*.go` 중 `*_test.go` 가 아닌 것) 1 라인이라도 변경 | 즉시 `git revert HEAD` + commit 재구성 (test-only 로 축소) |

각 보고서 (Task 1/2/3/4) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 (forced-fit) 회피 의무

본 cycle 은 **진단-only** + **ground-truth 검증** — 따라서 강압-적합 위험은 production fix cycle 보다 낮으나, 다음 의무는 유지:

1. **측정값 그대로 기록**: 각 task 의 모든 측정값 (byte dump / sample 부호 / cross-correlation 점수) 은 **raw output** 으로 보고서 §3 에 인용. 의도적 재가공 / 평균 / 정규화 금지.
2. **spec § verbatim 인용**: Task 1 보고서 §1 에 §B (PST format) — PDF 에 명시 인용 가능 시. PDF 에서 PST format 명시 인용 *불가* 시 보고서 §A 에 "spec PDF §B 또는 §6.x 에서 PST format 명시 발견되지 않음 — Annex A README / source distribution 내 .pst 정의 인용 시도" 로 명시 (휴리스틱 추정 금지).
3. **production 코드 알고리즘 재현 금지**: Task 진단 test 의 측정 코드는 production helper (`readPSTFrames`, `readG192Frames`) 호출 + raw byte 분석 이외에 production 알고리즘 재현 0.
4. **F-oct 권고는 측정 기반 ranking 만**: Task 4 의 권고 (PST format fix / frame indexing fix / 일반 결함 cycle / ALGTHM 우회 등) 는 §3 의 측정값 표에서 *직접* 도출. 측정값이 결정적이지 않으면 "결정적 분리 불가, F-oct-prelim-5 추가 진단 필요" 로 명시.
5. **PST format 외부 reference 0**: Task 1 의 endian/scaling 검증은 ITU PDF + ITU testdata distribution (README / source 내 정의) 만 인용. 외부 G.729 구현의 .pst writer 코드 0 참조.

### Phase 0.5 baseline F-sept-4 closure 인용 (필수 컨텍스트)

F-sept-4 종합 결과 (commit `d6834b0` 보고서) 가 본 cycle 의 *입력 전제*:

- **chain 내부 5 stage 모두 spec 정합** (정량):
  - excitation `u[5] = 0` (gp·v[5]=0 ∧ gc·c[5]=0; F-sept-1 측정).
  - LP `Â(z)` Q12 11 항: prod vs §3.2.6 reference float64 `max|Δ| = 6 LSB` (Q12 양자화 sub-LSB 누적, spec 정합 분류 L2; F-sept-2 측정).
  - synth IIR `1/Â(z)` sample 0..7: prod vs §3.10 reference float64 `max|Δ| = 1 LSB` (spec 정합 분류 S1; F-sept-3 측정).
  - postfilter chain (4 stage): F-sext-1 측정 모두 부호 보존 = spec-correct.
  - hpFilter §4.2.2: F-sext 시점 거동 일관.
- **잔존 mismatch**: ALGTHM frame 0 sf0 sample 5..7 chain output `[+1,+1,+1]` vs PST want `[+2,+4,+3,+3,+1,−1,−1,−1]` (sample 0..7 중 5..7 부호 반전). PST/2 spec-target sample 5..7 = `[−1,−1,−1]`.
- **결론**: chain 내부 결함 없음 → mismatch 의 결함은 *chain 외부* (PST 해석 / 비교 가설 / vector 정의) 에 위치 가능. 본 F-oct-prelim cycle = 결함 위치를 G1~G5 중 식별.

### Phase 0.6 가설 G1~G5 정의 (검증 대상)

| 가설 | 정의 | 검증 task |
|------|------|----------|
| **G1** PST file 해석 오류 | `readPSTFrames` (`testdata_helpers_test.go:30`) 가 binary format 을 잘못 파싱 — endian (현재 LittleEndian; ITU 정의가 BigEndian 또는 다른 byte order 가능) / scaling (Q15 vs Q14 vs linear) / sample stride (frame=80 sample=160 byte 가정) 결함 | Task 1 |
| **G2** Frame indexing mismatch | ALGTHM.BIT frame 0 ↔ ALGTHM.PST frame 0 정렬 어긋남 — preroll / silence frame / skip 가능성 (BIT[i] decoded ≠ PST[i] 이지만 BIT[i] decoded ≈ PST[i+k] 인 k 존재) | Task 2 |
| **G3** PST 가 G.729 main 출력 (Annex A 가 아님) | ALGTHM.PST 가 main spec hpFilter 거동 가정 — Annex A §4.2.2 hpFilter 의 startup transient 가 main 과 다를 수 있음 (sample 5..7 구간이 startup transient 영역) | Task 4 §3 (Task 1/2 결과로 보조 식별) |
| **G4** PST/2 가설 자체 무효 | F-quart-3 §6.4 의 "PST/2 = decode output" 가설이 잘못. 실제 관계는 PST = 2·decode (Q15→PCM16 의 Q-format 차이) 가 아닌 다른 것 (예: PST = decode 그대로 / PST = decode 후 추가 stage 적용) | Task 1 (scaling 측정) + Task 4 |
| **G5** ALGTHM = stress test vector | sample 5..7 = `[−1,−1,−1]` 이 *의도된 출력* 이며 chain 의 어딘가에 ALGTHM-specific 거동 (예: §3.10 Pass 2 overflow recovery trigger / specific overflow path) 을 *유발* 하기 위한 vector. 일반 vector (SPEECH / TEST 등) 는 정합 가능 | Task 3 |

각 가설의 검증/반증 결정 표 → Task 4 §3 결합 분석.

---

## Task F-oct-prelim-1: PST file format 해석 검증

**Goal:** `internal/decoder/testdata_helpers_test.go:30 readPSTFrames` 가 ALGTHM.PST binary format 을 spec-정합 으로 파싱하는지 검증. endian (LittleEndian 가정 vs BigEndian 대안) / scaling (linear int16 vs Q15 vs Q14) / sample stride (frame = 160 byte 가정) 의 3 차원 측정. PST want sample 0..7 = `[+2, +4, +3, +3, +1, −1, −1, −1]` (현재 readPSTFrames 결과) 가 정확한 해석인지 확인. PST/2 가설 (G4) 에 대한 1차 데이터 제공.

**Files:**
- Create: `internal/decoder/foct_prelim_diagnostic_test.go` (신규 진단 파일, `TestDiagnostic_FoctPrelimPSTFormat` 추가)
- Create: `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md`

### Spec § 인용 (F-oct-prelim-1 진단 근거)

ITU-T G.729 (06/2012) PDF 의 PST format 정의 위치 후보:
- §B (Annex B / Annex A test description) — Annex A test vector 정의 가능성.
- §6 (test vectors) — 일부 release distribution README 에 .pst 정의 명시 가능.

**PDF 검색 의무 (Step 2 작성 전)**: PDF 에서 "pst" / "post-processed" / "16-bit linear" / "test vector format" 등 keyword grep. 발견 시 §/page 명시 인용. **발견 *불가* 시** 보고서 §1 에 다음 명시:

> "ITU-T G.729 (06/2012) PDF 본문에서 .pst binary format 의 명시 정의를 발견하지 못함. ITU testdata distribution (`testdata/itu/G729_Release3/`) 의 `readme.txt` / `disk1readme` / source code 내 .pst writer 정의 (예: `pre_proc.c` / `g729a_decoder.c`) 의 verbatim 인용으로 대체 — 단 source code 인용은 *format 정의 부분 한정* (알고리즘 인용 금지, E4 회피)."

`readPSTFrames` 자체의 self-citing 주석 (`testdata_helpers_test.go:28-29`):

> // readPSTFrames loads a raw 16-bit little-endian PCM file (ITU Annex A
> // .pst format) from path, split into consecutive 80-sample frames.

본 task 는 위 가정 (16-bit LE PCM, 80 sample/frame = 160 byte/frame) 의 모든 차원을 측정.

**핵심 검증 점**:
- ALGTHM.PST 파일 크기 = `nFrames × 160 byte` 확인.
- 처음 160 byte (frame 0) 를 raw byte hex dump.
- LittleEndian / BigEndian 양쪽으로 sample 0..7 해석 → 어느 쪽이 `[+2,+4,+3,+3,+1,−1,−1,−1]` 또는 `[−1,−1,−1, ..., +1, +3, +3, +4, +2]` 산출?
- scaling 후보 (값 그대로 / >>1 / <<1) 각각 sample 0..7 dump → PST/2 가설 (>>1) 의 sample 0..7 = `[+1, +2, +1, +1, 0, −1, −1, −1]` 산출 검증.
- chain output 과 cross-correlation: `[+2, +4, +3, +3, +1, +1, +1, +1]` (chain) vs PST 해석 vs PST/2 해석 → 어느 쪽이 sample 0..4 부호 일치?

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(다른 변경 0. `02bf785` 이후 working tree 청산 상태.)

Run (회귀 게이트 baseline, Phase 0.2 의 게이트 1~8):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v
go vet ./...
```

Expected: 7 test PASS + `go vet` 무출력. 본 출력을 보고서 §2 에 인용.

- [ ] **Step 2: PDF 검색 + ITU testdata README 검토**

본 step 은 *측정 코드 작성 전 spec 인용 근거 확보*:

```bash
# 1. PDF 본문 검색
pdftotext docs/superpowers/specs/itu/G729E.pdf - | grep -inE "\.pst|post.*processed|16.bit linear|test.*vector.*format" | head -40

# 2. ITU testdata distribution README / readme 검토
find testdata/itu -iname "readme*" -o -iname "*.txt" | head -10
# 발견된 README 의 PST format 관련 라인 grep:
grep -iE "\.pst|format|16.bit|byte.order|endian" testdata/itu/**/readme* 2>/dev/null | head -40
```

발견된 인용 (PDF 또는 README) 을 보고서 §1 에 verbatim 기록. 발견 *불가* 시 Phase 0.4 §2 에 따라 명시.

**E4 의무**: README 인용 시 .pst writer 의 *format 정의 부분만* (struct 정의 / fwrite 호출 / 파일 헤더 정의 등). 알고리즘 (post-processing 함수 body) 인용 절대 금지.

- [ ] **Step 3: 진단 test 작성 — `foct_prelim_diagnostic_test.go` 신규**

`internal/decoder/foct_prelim_diagnostic_test.go` 신규 작성. 본 파일은 **본 cycle 의 모든 진단 test** 를 담는다 (Task 1/2/3 의 test 모두 본 파일에 누적 추가).

본 task (F-oct-prelim-1) 에서는 `TestDiagnostic_FoctPrelimPSTFormat` 추가:

```go
package decoder

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

// TestDiagnostic_FoctPrelimPSTFormat: Stage F-oct-prelim-1 진단.
//
// readPSTFrames (testdata_helpers_test.go:30) 의 ALGTHM.PST 해석이
// 16-bit LittleEndian PCM, 80 sample/frame 가정에 정합인지 측정.
//
// 측정 차원:
//   (a) 파일 크기 vs 가정 (nFrames × 160 byte).
//   (b) frame 0 raw byte hex dump (160 byte).
//   (c) LittleEndian vs BigEndian sample 0..7 해석.
//   (d) scaling 후보 (값 그대로 / >>1 / <<1) sample 0..7 dump.
//   (e) chain output [+2, +4, +3, +3, +1, +1, +1, +1] 과 cross-correlation.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimPSTFormat(t *testing.T) {
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, pstPath)

	data, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read PST: %v", err)
	}

	// (a) 파일 크기 vs 가정
	const bytesPerFrame = 160 // 80 sample × 2 byte
	t.Logf("──────── (a) ALGTHM.PST 파일 크기 분석 ────────")
	t.Logf("총 byte: %d", len(data))
	t.Logf("가정 (80 sample/frame × 2 byte): bytesPerFrame = %d", bytesPerFrame)
	t.Logf("nFrames (가정) = %d (잉여 byte = %d)",
		len(data)/bytesPerFrame, len(data)%bytesPerFrame)
	if len(data)%bytesPerFrame != 0 {
		t.Logf("(WARN) 파일 크기가 80-sample frame stride 의 정수배 아님 — G2/G4 의심")
	}

	// (b) frame 0 raw byte hex dump
	t.Logf("──────── (b) ALGTHM.PST frame 0 raw byte (160 byte = 80 sample × 2) ────────")
	for off := 0; off < 160 && off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		var line string
		for i := off; i < end; i++ {
			line += fmt.Sprintf("%02x ", data[i])
		}
		t.Logf("[%04x]  %s", off, line)
	}

	// (c) LittleEndian vs BigEndian sample 0..7 해석
	t.Logf("──────── (c) endian 양쪽 해석 (frame 0 sample 0..7) ────────")
	var leSamples, beSamples [8]int16
	for n := 0; n < 8; n++ {
		off := n * 2
		leSamples[n] = int16(binary.LittleEndian.Uint16(data[off : off+2]))
		beSamples[n] = int16(binary.BigEndian.Uint16(data[off : off+2]))
	}
	t.Logf("LittleEndian (현재 readPSTFrames 가정): %v", leSamples)
	t.Logf("BigEndian (대안):                       %v", beSamples)

	// (d) scaling 후보
	t.Logf("──────── (d) scaling 후보 (LittleEndian 기준) sample 0..7 ────────")
	t.Logf("값 그대로 (×1):  %v", leSamples)
	var leHalf, leDouble [8]int32
	for n := 0; n < 8; n++ {
		leHalf[n] = int32(leSamples[n]) >> 1
		leDouble[n] = int32(leSamples[n]) << 1
	}
	t.Logf("PST/2 (>>1):     %v", leHalf)
	t.Logf("PST·2 (<<1):     %v", leDouble)

	// (e) chain output vs PST 해석 cross-correlation
	chain := [8]int16{+2, +4, +3, +3, +1, +1, +1, +1} // F-sept-4 정합 chain output
	t.Logf("──────── (e) chain vs PST 해석 부호 비교 (sample 0..7) ────────")
	t.Logf("chain output (F-sept-4): %v", chain)
	matchSign := func(a int32, b int16) string {
		signEq := (a > 0 && b > 0) || (a < 0 && b < 0) || (a == 0 && b == 0)
		if signEq {
			return "="
		}
		return "≠"
	}
	for n := 0; n < 8; n++ {
		t.Logf("  [%d]  chain=%+d  LE=%+d (부호%s)  LE/2=%+d (부호%s)  BE=%+d (부호%s)",
			n, chain[n],
			leSamples[n], matchSign(int32(leSamples[n]), chain[n]),
			leHalf[n], matchSign(leHalf[n], chain[n]),
			beSamples[n], matchSign(int32(beSamples[n]), chain[n]))
	}

	// (f) readPSTFrames 호출 결과와 LittleEndian 해석 일치 검증
	want := readPSTFrames(t, pstPath)
	t.Logf("──────── (f) readPSTFrames 결과 vs LE 직접 해석 (frame 0 sample 0..7) ────────")
	t.Logf("readPSTFrames frame 0 sample 0..7: %v", want[0][:8])
	t.Logf("LE 직접 해석            sample 0..7: %v", leSamples)
	identical := true
	for n := 0; n < 8; n++ {
		if want[0][n] != leSamples[n] {
			identical = false
			break
		}
	}
	if identical {
		t.Logf("→ readPSTFrames 출력 = LittleEndian 직접 해석 (예상대로)")
	} else {
		t.Logf("(WARN) readPSTFrames 출력 ≠ LittleEndian 직접 해석 — helper 결함")
	}

	// (g) 시나리오 분류 dump
	t.Logf("──────── F-oct-prelim-1 시나리오 분류 ────────")
	t.Logf("LE sample 0..7 = %v (현재 가정)", leSamples)
	t.Logf("→ scenario hint: sample 0..4 부호 일치 표 + sample 5..7 부호 분포")
	t.Logf("(P1) LE 해석에서 sample 0..4 모두 chain 부호 일치 → endian 정상")
	t.Logf("(P2) BE 해석에서 sample 0..4 모두 chain 부호 일치 → endian 반대 (helper 결함)")
	t.Logf("(P3) 어떤 endian 에서도 sample 0..4 일관 부호 일치 0 → sample stride 어긋남")
	t.Logf("(P4) PST/2 (>>1) sample 0..4 가 chain 과 정확히 일치 (값 동일) → PST = 2·decode 가설 강화")
	t.Logf("(P5) 위 분류로 결정 불가 → Task 2/3 결합 분석 필요")
}
```

`go fmt` + `go vet ./...` 통과 확인. helper 중복 회피: 본 파일은 새 helper 도입 0 (기존 `vectorPath`, `ensureTestdataPresent`, `readPSTFrames` 만 사용).

- [ ] **Step 4: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimPSTFormat -v`

Expected: PASS (assertion 0, t.Logf 만). raw output 보고서 §3.1 에 인용.

- [ ] **Step 5: 측정값 분석 + 시나리오 분류**

Step 4 출력에서 다음 표 작성 (보고서 §3.2):

| 측정 | 결과 |
|------|------|
| 파일 크기 | ? byte |
| nFrames (160 byte stride) | ? |
| 잉여 byte | ? |
| LE sample 0..7 | `[?, ?, ?, ?, ?, ?, ?, ?]` |
| BE sample 0..7 | `[?, ?, ?, ?, ?, ?, ?, ?]` |
| LE/2 sample 0..7 | `[?, ?, ?, ?, ?, ?, ?, ?]` |
| LE·2 sample 0..7 | `[?, ?, ?, ?, ?, ?, ?, ?]` |
| chain output | `[+2, +4, +3, +3, +1, +1, +1, +1]` |
| readPSTFrames = LE 직접 해석 | true / false |

분류 시나리오 (P1~P5 — Phase 0.6 G1, G4 와 매핑):

- **(P1)** LE sample 0..4 부호 모두 chain 일치 (`[+,+,+,+,+]`) + LE 가 readPSTFrames 출력과 동일 → **G1 반증, endian/scaling 정상**. mismatch 는 sample 5..7 한정 → G2/G3/G5 추구.
- **(P2)** BE sample 0..4 부호가 chain 일치 + LE 부호 반대 → **G1 부분 발현 (endian 반대)**. F-oct fix 권고: `readPSTFrames` 를 BigEndian 으로 변경.
- **(P3)** LE / BE 어느 쪽도 sample 0..4 일관 부호 일치 0 → **sample stride 어긋남** (frame 시작 위치가 byte 0 아님). G1 strong. F-oct fix 권고: stride 재계산.
- **(P4)** LE/2 sample 0..4 가 chain 과 *값 정확히 일치* (예: `[+1,+2,+1,+1,0]`) → **G4 가설 강화 (PST = 2·decode 정합 candidate)**. 단 sample 5..7 = `[0,0,0]` (LE/2 floor) vs chain `[+1,+1,+1]` → 여전히 sample 5..7 mismatch. F-oct fix 권고: PST/2 가설 재정식화 + sample 5..7 잔존 결함 별도 추구.
- **(P5)** 위 분류 결정 불가 (모순 분포) → Task 2/3 결합 분석 의무. 본 task 단독 결론 보류.

- [ ] **Step 6: 회귀 게이트 통과 확인**

Run: `go test ./internal/... && go vet ./...`

Expected: Phase 0.2 의 게이트 1~8 모두 PASS + 본 task 새 test PASS. 게이트 9 (비-contract diagnostic 3건 FAIL 유지) plan-허용.

- [ ] **Step 7: F-oct-prelim-1 보고서 작성 + commit**

`docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-1 보고서 — PST file format 해석 검증

**작성일**: 2026-04-30
**범위**: ALGTHM.PST binary format 의 endian / scaling / sample stride
        정합성 측정. readPSTFrames helper 의 LittleEndian, 80 sample/frame
        가정 검증. 가설 G1 (PST file 해석 오류) + G4 (PST/2 가설) 1차 데이터.
**산출물**: 파일 크기 / frame 0 raw byte hex / LE·BE 양쪽 해석 / scaling
        후보 표 + 시나리오 분류 (P1~P5).
**준수**: ITU-T G.729 (06/2012) PDF + ITU testdata README 만 인용.
        외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 0 인용.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. PDF + README 검색 결과 (PST format 정의 인용 — 발견/불가 명시)
## 2. 회귀 게이트 baseline (Step 1 출력) + 9건 결과
## 3. 진단 측정값
   3.1 raw output (Step 4)
   3.2 endian × scaling 측정 표 (Step 5)
   3.3 readPSTFrames vs LE 직접 해석 일치성
## 4. 시나리오 분류 (P1 / P2 / P3 / P4 / P5)
## 5. 가설 G1 / G4 1차 평가 + Task 2 진입 권고
## A. (필요 시) PST format 인용 PDF 발견 불가 보충 메모
```

Working tree 검증:

```bash
git status --porcelain
```

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? internal/decoder/foct_prelim_diagnostic_test.go
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md
```

**E5 검증**: `git diff -- internal/` 의 production 라인 (즉 `*_test.go` 가 아닌 파일) 변경 0.

```bash
git add internal/decoder/foct_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-1 PST file format verification

F-sept-4 종합 (chain 내부 5 stage 모두 spec 정합) 후, ALGTHM frame 0
sf0 sample 5..7 부호 mismatch 의 결함 위치를 식별하기 위해 ALGTHM.PST
binary format 의 endian / scaling / sample stride 정합성을 측정한다.

readPSTFrames (testdata_helpers_test.go:30) 의 16-bit LittleEndian PCM,
80 sample/frame 가정을 (a) 파일 크기 / (b) frame 0 raw byte hex /
(c) LE vs BE / (d) scaling 후보 (×1, /2, ×2) / (e) chain output cross-
correlation 의 5 차원으로 검증. 시나리오 (P1 정상 / P2 endian 반대 /
P3 stride 결함 / P4 PST/2 가설 강화 / P5 결정 불가) 로 분류.

production 변경 0. ITU PDF + ITU testdata README 만 인용. 외부 G.729
구현 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-2: ALGTHM.BIT ↔ ALGTHM.PST frame indexing 검증

**Goal:** ALGTHM.BIT frame 0 ↔ ALGTHM.PST frame 0 정렬 검증 — preroll / silence / skip 가능성 식별. BIT[i] 디코딩 결과 (i ∈ 0..3) 와 PST[j] (j ∈ 0..3) 의 sample 0..7 부호 패턴 cross-correlation. 가장 매칭 높은 (i, j) pair 식별 → 가설 G2 의 검증/반증.

**Files:**
- Modify: `internal/decoder/foct_prelim_diagnostic_test.go` (append-only — `TestDiagnostic_FoctPrelimFrameAlignment` 추가)
- Create: `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-2-report.md`

### Spec § 인용 (F-oct-prelim-2 진단 근거)

ITU-T G.729 (06/2012) §4.3 (PDF p.30) — frame structure + initialization. Annex A README (testdata distribution) 의 BIT ↔ PST frame 정렬 정의 (PDF / README 발견 시 §1 에 인용; 발견 불가 시 Phase 0.4 §2 에 명시).

본 task 는 production decoder 의 frame 0..3 디코딩 출력과 PST frame 0..3 의 sample 0..7 cross-correlation 으로 정렬 측정.

**핵심 검증 점**:
- 4 × 4 = 16 (BIT[i], PST[j]) pair 모두 sample 0..7 부호 매칭 점수 (0~8) 측정.
- diagonal (i=j) 점수 vs off-diagonal 최대 점수 비교 → 어긋남 발현 시 어긋남 폭 (k = j − i) 식별.
- BIT 전체 frame 수 vs PST 전체 frame 수 차이 (프리롤 / 트레일링 확인).

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (Task 1 commit 후):
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(Task 1 의 신규 파일들은 commit 됨.)

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FoctPrelimFrameAlignment`**

`internal/decoder/foct_prelim_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FoctPrelimFrameAlignment: Stage F-oct-prelim-2 진단.
//
// ALGTHM.BIT frame 0..3 production 디코딩 결과 sample 0..7 와
// ALGTHM.PST frame 0..3 sample 0..7 의 cross-correlation 으로 frame
// indexing 정합성 측정. 가설 G2 (frame indexing mismatch) 검증.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimFrameAlignment(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	bitFrames, _ := readG192Frames(t, bitPath)
	pstFrames := readPSTFrames(t, pstPath)

	t.Logf("──────── (a) frame count 비교 ────────")
	t.Logf("BIT frame 수: %d", len(bitFrames))
	t.Logf("PST frame 수: %d", len(pstFrames))
	if len(bitFrames) != len(pstFrames) {
		t.Logf("(WARN) frame 수 불일치 — preroll / trailing silence 가능성 (Δ = %d)",
			len(pstFrames)-len(bitFrames))
	}

	// (b) BIT frame 0..3 production 디코딩 → sample 0..7 capture
	const N = 4
	var bitSamples [N][8]int16
	dec := New() // 새 Decoder instance
	for i := 0; i < N && i < len(bitFrames); i++ {
		var out [80]int16
		// production decoder API: 정확한 method 명은 internal/decoder/ 의
		// exported API 검토 후 호출. (예: dec.DecodeFrame(bitFrames[i], &out)
		// 또는 dec.Decode(packed, false, &out))
		if err := dec.DecodeFrame(bitFrames[i], false, &out); err != nil {
			t.Fatalf("DecodeFrame[%d]: %v", i, err)
		}
		copy(bitSamples[i][:], out[:8])
		t.Logf("BIT[%d] decoded sample 0..7: %v", i, bitSamples[i])
	}

	// (c) PST frame 0..3 sample 0..7
	var pstSamples [N][8]int16
	for j := 0; j < N && j < len(pstFrames); j++ {
		copy(pstSamples[j][:], pstFrames[j][:8])
		t.Logf("PST[%d] sample 0..7:        %v", j, pstSamples[j])
	}

	// (d) 4×4 부호 매칭 점수 표 (0~8)
	t.Logf("──────── (d) (BIT[i], PST[j]) sample 0..7 부호 매칭 점수 (0~8) ────────")
	t.Logf("       PST[0]  PST[1]  PST[2]  PST[3]")
	signMatchScore := func(a, b [8]int16) int {
		score := 0
		for n := 0; n < 8; n++ {
			as := signOfInt16(a[n])
			bs := signOfInt16(b[n])
			if as == bs {
				score++
			}
		}
		return score
	}
	type pair struct{ i, j, score int }
	var best pair
	for i := 0; i < N; i++ {
		row := fmt.Sprintf("BIT[%d]  ", i)
		for j := 0; j < N; j++ {
			s := signMatchScore(bitSamples[i], pstSamples[j])
			row += fmt.Sprintf("%4d    ", s)
			if s > best.score {
				best = pair{i, j, s}
			}
		}
		t.Logf("%s", row)
	}

	// (e) PST/2 cross-correlation (G4 보조 측정)
	t.Logf("──────── (e) PST/2 부호 매칭 점수 표 (PST[j]>>1 와 BIT[i] 비교) ────────")
	t.Logf("       PST/2[0]  PST/2[1]  PST/2[2]  PST/2[3]")
	for i := 0; i < N; i++ {
		row := fmt.Sprintf("BIT[%d]    ", i)
		for j := 0; j < N; j++ {
			var halved [8]int16
			for n := 0; n < 8; n++ {
				halved[n] = int16(int32(pstSamples[j][n]) >> 1)
			}
			s := signMatchScore(bitSamples[i], halved)
			row += fmt.Sprintf("%4d      ", s)
		}
		t.Logf("%s", row)
	}

	// (f) 시나리오 분류
	t.Logf("──────── F-oct-prelim-2 시나리오 분류 ────────")
	t.Logf("최대 매칭: BIT[%d] ↔ PST[%d] 점수=%d/8", best.i, best.j, best.score)
	t.Logf("(F1) best 가 (i=j=0) 이고 score≥6 → 정상 alignment, G2 반증")
	t.Logf("(F2) best 가 (i=0, j=1) → PST 가 1 frame skip / preroll, G2 발현")
	t.Logf("(F3) best 가 |j−i|>1 → multi-frame skip, G2 강하게 발현")
	t.Logf("(F4) 모든 score≤4 → 매칭 0, G2 반증 + G1/G4/G5 우세")
}
```

**API signature 주의**: Step 작성 시 `internal/decoder/decoder.go` 의 exported API (예: `New()`, `DecodeFrame(packed []byte, badFrame bool, out *[80]int16) error`) 를 *grep 으로 확인* 후 호출. signature 가 plan 예시와 다르면 *test 코드만* 수정 (production 변경 금지). Decoder type 이 internal value type 이면 `var dec Decoder; dec.Reset()` 형태 사용.

`signOfInt16` 는 F-sept-1 (`stagef_sept_diagnostic_test.go`) 에 이미 정의됨 — 같은 package `decoder` 라 직접 호출. 본 파일에서 helper 재정의 금지.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimFrameAlignment -v`

Expected: PASS. raw output 보고서 §3.1 에 인용.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 출력에서 4×4 매칭 표 + PST/2 매칭 표 작성 (보고서 §3.2). 분류 시나리오 (F1~F4 — Phase 0.6 G2 와 매핑):

- **(F1)** best (i=j=0) ∧ score ≥ 6/8 → **frame alignment 정상**. G2 반증. mismatch 는 sample 5..7 한정 (PST 와 BIT 정합).
- **(F2)** best (i=0, j=1) → **PST 가 1 frame preroll**. G2 발현. F-oct fix 권고: PST 디코딩 시 frame 1 부터 비교 / BIT 디코딩 시 frame 0 의 *전* 에 silence 1 frame 처리.
- **(F3)** best |j−i| > 1 또는 (i=j) 이지만 j ≠ 0 → **multi-frame skip**. G2 강. F-oct fix 권고: indexing 재계산.
- **(F4)** 모든 score ≤ 4 → **매칭 점수 random 수준**. G2 반증 + G1/G4/G5 우세 (PST 자체가 다른 의미).

PST/2 매칭 표 (보조): PST/2 best 가 PST best 보다 높으면 G4 가설 강화 (Task 1 P4 와 결합 분석).

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/... && go vet ./...`

Expected: Phase 0.2 의 게이트 1~8 모두 PASS + Task 1/2 새 test PASS.

- [ ] **Step 6: F-oct-prelim-2 보고서 작성**

`docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-2-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-2 보고서 — BIT ↔ PST frame indexing 검증

**작성일**: 2026-04-30
**범위**: ALGTHM.BIT frame 0..3 production 디코딩과 ALGTHM.PST frame
        0..3 의 sample 0..7 부호 cross-correlation 으로 frame indexing
        정합성 측정. 가설 G2 (frame indexing mismatch) 검증.
**산출물**: 4×4 매칭 점수 표 + PST/2 보조 표 + 시나리오 (F1~F4) 분류.
**준수**: ITU-T G.729 (06/2012) §4.3 + Annex A README (인용 발견 시).
        외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가
## 1. §4.3 + README 인용 (BIT ↔ PST 정렬 정의 — 발견/불가 명시)
## 2. 회귀 게이트 결과 (9건)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 4×4 매칭 점수 표 + PST/2 보조 표
   3.3 best 매칭 (i, j) 식별
## 4. 시나리오 분류 (F1 / F2 / F3 / F4)
## 5. 가설 G2 평가 + Task 3 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/decoder/foct_prelim_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-2-report.md
```

```bash
git add internal/decoder/foct_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-2 BIT↔PST frame indexing scan

ALGTHM.BIT frame 0..3 production 디코딩 결과 sample 0..7 와
ALGTHM.PST frame 0..3 sample 0..7 부호 cross-correlation 으로
frame indexing 정합성을 측정한다. 4×4 매칭 점수 표 + PST/2 보조
표 (G4 가설 결합) + best (i,j) pair 식별.

시나리오 (F1 정상 / F2 PST 1-frame preroll / F3 multi-frame skip /
F4 매칭 0 = G2 반증) 으로 분류. 가설 G2 검증 + Task 3 진입 결정.

production 변경 0. ITU PDF + ITU testdata README 만 인용. 외부
G.729 구현 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-3: 다른 ITU vector cross-check

**Goal:** ALGTHM 외 5 ITU vector (TEST / SPEECH / LSP / PITCH / FIXED) 에서 동일 chain mismatch 패턴 (sample 5..7 부호 반전) 발생 여부 측정. ALGTHM 특이성 (G5 발현) vs 일반 결함 (G1/G3/G4 발현) 분리. ALGTHM 1건 더 베이스라인으로 포함 → 6 vector × frame 0 sf0 sample 0..7 production 디코딩 vs PST want 부호 비교.

**Files:**
- Modify: `internal/decoder/foct_prelim_diagnostic_test.go` (`TestDiagnostic_FoctPrelimMultiVectorScan` 추가)
- Create: `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md`

### Spec § 인용 (F-oct-prelim-3 진단 근거)

ITU-T G.729 (06/2012) §6 (test vectors — PDF page 발견 시 명시 인용). Annex A test vector distribution 의 vector 의도 정의 (README — 발견 시 §1 에 verbatim).

본 task 는 6 vector × frame 0 sf0 sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7 부호 비교 → mismatch 분포 표 생성.

**핵심 검증 점**:
- 6 vector 중 sample 5..7 모두 부호 일치 → 그 vector 는 정합 (chain 정상).
- 6 vector 중 sample 5..7 모두 부호 반전 → 일반 결함 (G1/G3/G4 발현).
- mixed (일부 vector 정합 / 일부 반전) → vector-specific 거동, ALGTHM 가 stress test (G5) 가능성.

testdata 디렉토리 (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/`) 에서 vector 파일명 확인:
- ALGTHM.BIT / ALGTHM.PST
- TEST.BIT / **TEST.pst** (lowercase 확장자 주의 — `vectorPath` 가 case-sensitive 면 별도 처리 필요)
- SPEECH.BIT / SPEECH.PST
- LSP.BIT / LSP.PST
- PITCH.BIT / PITCH.PST
- FIXED.BIT / FIXED.PST

`vectorPath("TEST.pst")` 가 동작하지 않으면 (대소문자) test 코드에서 양쪽 path 시도 (`os.Stat` fallback). production 변경 0 (E5).

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (Task 2 commit 후):
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FoctPrelimMultiVectorScan`**

`internal/decoder/foct_prelim_diagnostic_test.go` 에 추가:

```go
// TestDiagnostic_FoctPrelimMultiVectorScan: Stage F-oct-prelim-3 진단.
//
// 6 ITU vector (ALGTHM, TEST, SPEECH, LSP, PITCH, FIXED) 의 frame 0
// sf0 sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7
// 부호 비교. ALGTHM 특이성 (G5) vs 일반 결함 (G1/G3/G4) 분리.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimMultiVectorScan(t *testing.T) {
	type vec struct {
		name           string
		bitName, pstName string
	}
	vectors := []vec{
		{"ALGTHM", "ALGTHM.BIT", "ALGTHM.PST"},
		{"TEST", "TEST.BIT", "TEST.pst"}, // lowercase .pst 주의
		{"SPEECH", "SPEECH.BIT", "SPEECH.PST"},
		{"LSP", "LSP.BIT", "LSP.PST"},
		{"PITCH", "PITCH.BIT", "PITCH.PST"},
		{"FIXED", "FIXED.BIT", "FIXED.PST"},
	}

	type result struct {
		name        string
		prod        [8]int16
		want        [8]int16
		matchCount5to7 int // sample 5..7 중 부호 일치 수 (0~3)
		signs       [8]string // "=" or "≠" per sample
	}
	var results []result

	for _, v := range vectors {
		bitPath := vectorPath(v.bitName)
		pstPath := vectorPath(v.pstName)
		// pst 대소문자 fallback
		if _, err := os.Stat(pstPath); err != nil {
			alt := vectorPath(strings.ToUpper(v.pstName))
			if _, err2 := os.Stat(alt); err2 == nil {
				pstPath = alt
			}
		}
		if _, err := os.Stat(bitPath); err != nil {
			t.Logf("vector %s: BIT missing (%v) — skip", v.name, err)
			continue
		}
		if _, err := os.Stat(pstPath); err != nil {
			t.Logf("vector %s: PST missing (%v) — skip", v.name, err)
			continue
		}

		bitFrames, _ := readG192Frames(t, bitPath)
		pstFrames := readPSTFrames(t, pstPath)
		if len(bitFrames) == 0 || len(pstFrames) == 0 {
			t.Logf("vector %s: empty frames — skip", v.name)
			continue
		}

		// production 디코딩 frame 0
		dec := New()
		var out [80]int16
		if err := dec.DecodeFrame(bitFrames[0], false, &out); err != nil {
			t.Logf("vector %s: DecodeFrame error %v — skip", v.name, err)
			continue
		}

		var r result
		r.name = v.name
		copy(r.prod[:], out[:8])
		copy(r.want[:], pstFrames[0][:8])
		for n := 0; n < 8; n++ {
			if signOfInt16(r.prod[n]) == signOfInt16(r.want[n]) {
				r.signs[n] = "="
			} else {
				r.signs[n] = "≠"
			}
		}
		for n := 5; n <= 7; n++ {
			if r.signs[n] == "=" {
				r.matchCount5to7++
			}
		}
		results = append(results, r)
	}

	// 표 출력
	t.Logf("──────── (a) 6 vector × frame 0 sf0 sample 0..7 ────────")
	for _, r := range results {
		t.Logf("[%s]", r.name)
		t.Logf("  prod = %v", r.prod)
		t.Logf("  want = %v", r.want)
		t.Logf("  sign = %v  (sample 5..7 일치 %d/3)", r.signs, r.matchCount5to7)
	}

	// (b) 분포 요약
	t.Logf("──────── (b) sample 5..7 부호 일치 분포 요약 ────────")
	t.Logf("vector       sample5  sample6  sample7  match5..7")
	allMatch := true
	allMismatch := true
	for _, r := range results {
		t.Logf("  %-10s   %s        %s        %s        %d/3",
			r.name, r.signs[5], r.signs[6], r.signs[7], r.matchCount5to7)
		if r.matchCount5to7 < 3 {
			allMatch = false
		}
		if r.matchCount5to7 > 0 {
			allMismatch = false
		}
	}

	// (c) 시나리오 분류
	t.Logf("──────── F-oct-prelim-3 시나리오 분류 ────────")
	switch {
	case allMatch:
		t.Logf("(V1) 모든 vector 가 sample 5..7 부호 정합 → ALGTHM 도 정합 ?")
		t.Logf("     (이 case 가 발현하면 ALGTHM 자체 측정에 모순 — F-sept-4 회귀 의심)")
	case allMismatch:
		t.Logf("(V2) 모든 vector 에서 sample 5..7 부호 반전 → 일반 결함 (G1/G3/G4)")
		t.Logf("     F-oct 권고: chain 외부 결함 추적 (PST format / hpFilter startup / PST/2 가설)")
	default:
		t.Logf("(V3) mixed — 일부 vector 정합 / 일부 반전")
		t.Logf("     ALGTHM-specific 거동 (G5 발현) 가능성 + vector-specific 분포 분석 의무")
	}
}
```

import 추가: `"os"`, `"strings"`. (이미 Task 1 에서 `os`, `fmt` 추가됨. `strings` 신규.)

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimMultiVectorScan -v`

Expected: PASS. raw output 보고서 §3.1 에 인용.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 출력에서 6 vector 의 sample 5..7 부호 매칭 표 작성 (보고서 §3.2). 분류 시나리오 (V1~V3 — Phase 0.6 G5 와 매핑):

- **(V1)** 모든 6 vector sample 5..7 부호 정합 (3/3) → **ALGTHM 도 정합** (모순). F-sept-4 측정 회귀 의심 → Task 4 §3 에서 재검증.
- **(V2)** 모든 6 vector sample 5..7 부호 반전 (0/3) → **일반 결함**. G1/G3/G4 우세. F-oct 권고: chain 외부 결함 추적 (PST format / hpFilter startup transient / PST/2 가설 재정식화).
- **(V3)** mixed (일부 정합 / 일부 반전) → **vector-specific 거동**. G5 발현 가능. F-oct 권고: ALGTHM 우회 + 정합 vector 로 회귀 가드 promotion.

vector-specific 분포 보조 분석 (§3.3):
- 정합 vector 그룹 vs 반전 vector 그룹 의 공통점 (frame 0 의 LSP / pitch / fcb 특성) 측정 — Task 4 §3 에서 결합.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/... && go vet ./...`

Expected: Phase 0.2 의 게이트 1~8 모두 PASS + Task 1/2/3 새 test 모두 PASS.

- [ ] **Step 6: F-oct-prelim-3 보고서 작성**

`docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-3 보고서 — 6 ITU vector cross-check

**작성일**: 2026-04-30
**범위**: ALGTHM / TEST / SPEECH / LSP / PITCH / FIXED 의 frame 0 sf0
        sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7
        부호 비교. ALGTHM 특이성 (G5) vs 일반 결함 (G1/G3/G4) 분리.
**산출물**: 6 vector × sample 5..7 매칭 표 + 시나리오 (V1/V2/V3) 분류.
**준수**: ITU-T G.729 (06/2012) §6 + Annex A README 만 인용. 외부 구현 0건.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가
## 1. §6 + README 인용 (vector 의도 정의 — 발견/불가 명시)
## 2. 회귀 게이트 결과 (9건)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 6 vector × sample 5..7 매칭 표
   3.3 정합/반전 vector 그룹 공통점 보조 분석
## 4. 시나리오 분류 (V1 / V2 / V3)
## 5. 가설 G5 평가 + Task 4 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/decoder/foct_prelim_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md
```

```bash
git add internal/decoder/foct_prelim_diagnostic_test.go \
        docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-3-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-3 multi-vector mismatch scan

ALGTHM 외 5 ITU vector (TEST, SPEECH, LSP, PITCH, FIXED) 의 frame 0
sf0 sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7
부호 비교. ALGTHM 특이성 (가설 G5) vs 일반 결함 (G1/G3/G4) 분리.

시나리오 (V1 모든 vector 정합 / V2 모든 vector 반전 = 일반 결함 /
V3 mixed = vector-specific 거동) 으로 분류. F-oct 권고 방향 (chain
외부 결함 추적 / ALGTHM 우회 / 정합 vector 회귀 가드 promotion)
1차 결정 데이터.

production 변경 0. ITU PDF + ITU testdata README 만 인용. 외부 G.729
구현 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-4: 종합 보고서 + F-oct 권고 갱신

**Goal:** Task 1 (P1~P5) × Task 2 (F1~F4) × Task 3 (V1~V3) 의 결과를 결합 분석해 가설 G1~G5 중 *결정적 위치* 식별. F-oct production fix cycle (또는 plan-end declared "결함 0 — ALGTHM stress test 가설 채택") 권고 결정. production 변경 0.

**Files:**
- Create: `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md`
- **Modify: 없음**

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain`

Expected (Task 3 commit 후):
```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

- [ ] **Step 2: 종합 측정값 수집**

Run:
```
git log --oneline -10
go test ./internal/decoder/ -run "TestDiagnostic_FoctPrelim" -v
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
go test ./internal/decoder/ -run "TestDiagnostic_Fsept" -v
go vet ./...
```

Expected: F-oct-prelim task 3건 PASS + 회귀 게이트 1~8 모두 PASS. raw output 보고서 §1/§2 에 인용.

- [ ] **Step 3: 시나리오 결합 분석 (3 × 5 × 3 × 4 = 60 시나리오 중 핵심 결정 표)**

Task 1 (P1~P5) × Task 2 (F1~F4) × Task 3 (V1~V3) 결합 → F-oct 권고 결정 표:

| Task 1 | Task 2 | Task 3 | F-oct 권고 (단일 결함 식별 또는 가설 폐기) |
|--------|--------|--------|---------------------------------------------|
| (P1) endian/scaling 정상 | (F1) alignment 정상 | (V1) 모든 vector 정합 | **모순 — F-sept-4 회귀 의심** (E1/E3 검토) |
| (P1) endian/scaling 정상 | (F1) alignment 정상 | (V2) 모든 vector 반전 | **G3 우세 — Annex A vs main spec hpFilter 거동 차이** → F-oct = §4.2.2 hpFilter startup transient 재검토 |
| (P1) endian/scaling 정상 | (F1) alignment 정상 | (V3) mixed | **G5 우세 — ALGTHM stress test** → F-oct = ALGTHM 우회 + 정합 vector 로 회귀 가드 + ALGTHM 의도 거동 별도 추구 |
| (P1) endian/scaling 정상 | (F2/F3) preroll/skip | (any) | **G2 발현 — frame indexing fix** → F-oct = `decode_test.go` 비교 시 PST offset 적용 |
| (P1) endian/scaling 정상 | (F4) 매칭 0 | (V2/V3) | **G1/G3/G4 복합** → F-oct = PST 정의 재검토 (Annex A README / source) + 가설 G4 재정식화 |
| (P2) endian 반대 | (any) | (any) | **G1 발현 — readPSTFrames endian fix** → F-oct = helper 를 BigEndian 으로 변경 |
| (P3) sample stride 결함 | (any) | (any) | **G1 발현 — readPSTFrames stride fix** → F-oct = stride 재계산 (header 가능성 검토) |
| (P4) PST/2 가설 강화 | (F1) | (V1/V2) | **G4 발현 — PST = 2·decode 정합 (sample 0..4 만)** → F-oct = sample 5..7 잔존 결함을 G3/G5 추적 |
| (P5) 결정 불가 | (any) | (any) | **F-oct-prelim-5 추가 진단** (PST 다른 frame / 다른 sample 위치 측정 확장) |

본 step 의 분류는 측정값 표에서 *직접* 도출 — 휴리스틱 0.

**중요 — 모순 시나리오 처리**: 표의 (P1, F1, V1) row 발현 시 = ALGTHM frame 0 sf0 sample 5..7 측정값이 F-sept-4 와 본 cycle 사이에 회귀 → E1 발동, working tree 의 untracked file (`stagef_bis_diagnostic_test.go`) 또는 다른 변경 영향 감사. 회귀 trace 보고서 §6 에 기록.

- [ ] **Step 4: 잔여 보류 항목 갱신 (F-sept-4 §5 답습)**

F-sept-4 §5 의 잔여 항목을 본 cycle 결과로 갱신:

1. **F-oct (production fix 또는 plan-end)**: Step 3 의 결합 표로 권고 방향 결정. 표에 없는 새 시나리오 발현 시 plan-end "ALGTHM stress test 가설 채택" 명시.
2. **filterSubframe ÷4/×4**: F-quint-3 §4.1 동상 (frame 0 sf0 미-trigger).
3. **β init = 0.2**: F-quint-3 §4.2 동상.
4. **frame 1+ 잔여**: 본 cycle 의 Task 2 가 frame 0..3 일부 측정 — 추가 frame 영향 별도 cycle.
5. **회귀 가드 promotion**: V1/V2 정합 vector 발견 시 sample 0..7 영구 게이트 promotion 검토.
6. **비-contract diagnostic 3건**: F-quint-3 §4.6 동상 (cleanup task).
7. **F-sext-2 / F-sext-3 (HP filter 진단)**: G3 발현 시 reactivate.
8. **lsp_lp.go uncommitted (F-bis-1 P fix)**: F-sept 시점 정식화 완료 (`02bf785`) — 본 cycle 잔여 0.
9. **stagef_bis_diagnostic_test.go untracked**: 보존 유지. F-bis cycle 종결 시 commit 검토.

- [ ] **Step 5: F-oct-prelim-4 종합 보고서 작성**

`docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-4 종합 보고서 + F-oct 권고

**작성일**: 2026-04-30
**범위**: F-oct-prelim-1/2/3 의 진단 결과 결합 분석 + 가설 G1~G5 중
        결정적 위치 식별 + F-oct (production fix 또는 plan-end) cycle
        권고.
**산출물**: 시나리오 결합 표 (3 × 5 × 4 매트릭스 핵심 발췌) +
            F-oct 권고 + 잔여 보류 항목 갱신.
**준수**: F-oct-prelim-1/2/3 + F-sept-4 + F-sext-1 + F-quart/F-quint
        보고서만 인용. 외부 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 종합 평가 (E1-E5)
## 1. F-oct-prelim cycle commit 요약 (3 commit + 본 commit)
## 2. 회귀 게이트 종합 결과 (9건)
## 3. 시나리오 결합 분석 (Task 1 × Task 2 × Task 3) + 가설 G1~G5 매핑
## 4. F-oct 권고 방향 결정 (production fix / plan-end / 추가 진단)
## 5. 잔여 보류 항목 갱신 (F-sept-4 §5 표 답습)
## 6. (모순 발현 시) 회귀 trace + E1/E3 평가
## 7. 결론 — Phase 1k Stage F-oct-prelim closure
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md
```

```bash
git add docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-4-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-oct-prelim synthesis report + F-oct recommendation

Stage F-oct-prelim cycle (Task 1 PST format, Task 2 frame indexing,
Task 3 multi-vector scan) 의 진단 결과 결합 분석. ALGTHM frame 0 sf0
sample 5..7 부호 mismatch 의 결함 위치 (가설 G1~G5 중 식별) +
F-oct production fix 또는 plan-end (ALGTHM stress test 가설 채택)
권고 결정.

production 변경 0. 시나리오 결합 표 (Task 1 P1~P5 × Task 2 F1~F4 ×
Task 3 V1~V3) 으로 단일 결함 위치 또는 가설 폐기 ranking. 외부 G.729
구현 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage**:
- ✓ F-sept-4 closure 후속 — chain 외부 결함 위치 식별 (G1~G5 동등 분리).
- ✓ §B (PST format) — Task 1 §1 (PDF 인용 발견 / 불가 시 보충 메모 의무).
- ✓ §6 (test vectors) — Task 3 §1.
- ✓ §A (Annex A test vector 정의) — Task 1/3 README 인용.
- ✓ §4.3 frame structure — Task 2 §1.
- ✓ §4.2.2 hpFilter startup transient — Task 4 §3 G3 가설.
- ✓ production 변경 0 invariant (E5).
- ✓ Escape hatch E1/E2/E3/E4/E5 — Phase 0.3 + 모든 task §0.
- ✓ 9 회귀 게이트 — Phase 0.2.

**2. Placeholder scan**:
- Task 1 Step 5 의 측정 표 (?, ?) 는 *측정 후 채울 데이터* — placeholder 아닌 정량 측정 슬롯.
- Task 1/2/3 의 시나리오 분류 (P1~P5 / F1~F4 / V1~V3) 는 *측정 분류 골격* — placeholder 아닌 결정 표.
- Task 4 Step 3 의 9 row 결합 표는 모든 핵심 시나리오 조합 망라 — placeholder 아닌 결정 표.

**3. Type consistency**:
- `[80]int16` (frame sample, PST want): `readPSTFrames` 시그니처 일관.
- `[8]int16` (sample 0..7 capture): Task 2/3 의 chain trace 일관.
- `int16` sample, `byte` raw byte: Task 1 의 hex dump + endian 변환 일관.
- `int32` 누산 (PST/2 시 overflow 회피): Task 1 (d) `int32(leSamples[n]) >> 1`.
- helper 명: `signMatchScore`, `result`, `pair` — local 정의 일관.
- 기존 helper: `vectorPath`, `readPSTFrames`, `readG192Frames`, `ensureTestdataPresent`, `signOfInt16` (F-sept-1 정의) — 재사용.

**4. 외부 구현 참조 0**: 모든 spec 인용 = ITU-T G.729 (06/2012) PDF + ITU testdata distribution README + 본 cycle / F-sept / F-sext / F-quart / F-quint 보고서. 외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 0 인용. ✓ Task 1 Step 2 의 source code 인용 시도 시 *format 정의 부분 한정* (알고리즘 인용 절대 금지) — E4 회피.

**5. TDD 준수**:
- 본 cycle 은 *진단-only / ground-truth 검증* — RED→GREEN gate 는 진단 데이터 capture + 시나리오 분류 결정 의무로 변형.
- Task 1/2/3 모두 Step 3-4 = 진단 test 작성 + 실행 PASS + 분류.
- Task 4 = 메타 task (test 추가 0).
- 9 회귀 게이트 (Phase 0.2) 는 각 commit 후 *모두 PASS 의무* (게이트 9 = plan-허용 FAIL 명시).

**6. 강압-적합 회피**:
- Task 1/2/3 모두 *측정-only* — t.Errorf / t.Fatalf 사용 0 (파일 I/O 오류 제외).
- Task 4 의 F-oct 권고 결정은 *측정값 결합 표에서 직접 도출* — 휴리스틱 0.
- Task 1 의 endian/scaling/stride 검증은 모든 후보 dump → 측정값으로 분류 (예단 0).
- Task 3 의 vector 분류는 6 vector 측정값으로 G5 가설 직접 검증 — vector 선택 bias 0 (ITU Annex A test_vectors 디렉토리의 sample 6건).

**7. Commit 정책**:
- Task 1 = 1 commit (진단 test 신규 + 보고서 1).
- Task 2 = 1 commit (진단 test modify + 보고서 1).
- Task 3 = 1 commit (진단 test modify + 보고서 1).
- Task 4 = 1 commit (보고서 1, production 변경 0).
- 총 **4 commit**. 진단 task 별 분리.

**8. Co-author trailer**: 4 commit 모두 `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` 포함.

**9. ITU PDF 인용 위치 명시**:
- Task 1 §1: §B (Annex A test description) 또는 §6 — 발견 시 PDF page 명시.
- Task 2 §1: §4.3 (PDF p.30) — frame structure.
- Task 3 §1: §6 (test vectors) — PDF page 발견 시 명시.
- Task 4 §3 G3: §4.2.2 hpFilter — PDF page F-sext 보고서 인용.
- 모든 PDF 인용 발견 *불가* 시 Phase 0.4 §2 절차 (보고서 §A 보충 메모) 의무.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-30-phase1k-stage-f-oct-prelim-plan.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — F-quart / F-sext / F-sept 패턴과 동일. task 별 fresh subagent dispatch + 회귀 게이트 catch.

**2. Inline Execution** — Execute tasks in this session, batch execution with checkpoints.

**다음 단계 = F-oct-prelim-1 dispatch 권고** (PST file format 검증으로 G1/G4 1차 데이터 확보 → 후속 task 의 결정 매트릭스 좁히기).
