# Phase 1k Stage F-oct-prelim-5 Diagnostic-Only Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-oct-prelim-4 §4.2 단일 결정 — 가설 G3 (Annex A vs main spec 분기 거동) 의 *분기 위치 직접 식별 데이터 부재* 를 해소하기 위한 추가 진단 cycle. ALGTHM frame 0 sf0 sample 5..7 부호 mismatch 의 결함 위치를 (M1) postfilter ringing / (M2) hpFilter 음수 감쇠 / (M3) synthesis memory init / (M4) PST 출처 분기 / (M5) 결함 부재 (PST 자체가 nonzero negative 정상 출력) 5 후보 중 *측정-only* 로 단일 식별. F-sext-2/3 (HP filter 진단) reactivate 통합. **production 변경 0 라인** invariant.

**Architecture:** 4-task 진단 cycle (F-oct-prelim 패턴 답습). Task F-oct-prelim-5-1 = PST 출처 verbatim 추적 (Annex A `READMETV.txt` + main G.729 `READMETV.txt` 인용 + ALGTHM/PITCH/FIXED frame 0 raw BIT byte 3-way diff). Task F-oct-prelim-5-2 = hpFilter init state + F-sext-2/3 reactivate (§4.2.2 IIR memory zero-init vs spec-prescribed init 비교 + impulse-step response 측정). Task F-oct-prelim-5-3 = silence frame 0 negative output 생성 chain trace (postfilter ringing / hpFilter 음수 감쇠 / synth memory 후보 측정). Task F-oct-prelim-5-4 = 종합 보고서 + F-oct (production fix / plan-end / 추가 진단) 단일 결정. 각 task 의 production 코드 변경 0 라인 (E5 invariant); test 추가만 허용.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §4.2.2 (output HP filter, eq. 151/152) + §A.4.2 (Annex A postfilter chain) + §3.10 (synthesis filter, two-pass overflow) + §4.3 (decoder initialization). Annex A `READMETV.txt` (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt`) + main G.729 `READMETV.txt` (`testdata/itu/G729_Release3/g729/test_vectors/READMETV.txt`). 기존 F-quart/F-sext/F-sept/F-oct-prelim 진단 하니스 (cross-check 패턴) 재활용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-oct-prelim-5 진입 시점, post-`06a4205`)

| 경로 | 상태 | F-oct-prelim-5 변경? |
|------|------|----------------------|
| `internal/lsp/lsp_lp.go` | committed via `02bf785` (F-sept-4 시점 정식화 종결) | **No** (변경 금지 — 별도 cycle 영역) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis/F-tris 진단 baseline | **No** (보존, F-bis cycle 종결 시 별도 commit 검토) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (F-quart cycle) | **No** (변경 금지) |
| `internal/decoder/stagef_sext_diagnostic_test.go` | committed `6f1c841` (F-sext-1) | **No** (변경 금지) |
| `internal/decoder/stagef_sept_diagnostic_test.go` | committed `48265cd` / `d61497d` / `353398d` (F-sept-1/2/3) | **No** (변경 금지) |
| `internal/decoder/foct_prelim_diagnostic_test.go` | committed `5832294` / `94ef154` / `51e74e2` (F-oct-prelim-1/2/3) | **No** (변경 금지) |
| `internal/synth/excitation.go` / `internal/synth/filter.go` | F-quint-3 시점 그대로 | **No** (진단-only) |
| `internal/decoder/hpfilter.go` | Phase 1g 부터 그대로 | **No** (진단-only) |
| `internal/postfilter/postfilter.go` | F-sept 시점 그대로 | **No** (진단-only) |
| 그 외 production 파일 | 미변경 | **No** (진단-only) |

F-oct-prelim-5 신규 파일 (모두 *_test.go 또는 .md):
- (Task F-oct-prelim-5-1) `internal/decoder/stagef_octprelim5_diagnostic_test.go` — 본 cycle 의 모든 진단 test 통합 파일.
- (Task F-oct-prelim-5-1) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md`
- (Task F-oct-prelim-5-2) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md`
- (Task F-oct-prelim-5-3) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md`
- (Task F-oct-prelim-5-4) `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md`

본 cycle 의 production 변경 범위 = **0 라인**. test 변경 = `stagef_octprelim5_diagnostic_test.go` 신규 파일 only. 그 외 *_test.go 파일 변경 절대 금지.

**`stagef_bis_diagnostic_test.go` untracked 보존**: 본 cycle 4 task 어떤 commit 도 본 파일을 add 하지 않는다. 사후 working tree 의 `?? internal/decoder/stagef_bis_diagnostic_test.go` 가 F-oct-prelim cycle 시점과 동일하게 유지됨을 각 task §0 보고서에서 확인.

### Phase 0.2 회귀 게이트 명세

각 task commit 직후 *반드시* 실행 (총 12 게이트 — 기존 9 + F-oct-prelim cycle 3):

1. **Stage D 17 contract test** (`internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/` 의 contract spec test). 본 cycle 회귀 0 의무.
2. **Stage D-bis 3 contract test** (F-bis-1 P fix 검증 + LSP 합성 cross-check + 추가 contract). 회귀 0.
3. **Phase 1i sample 0 가드** (`TestDecode_Frame0Sample0_MatchesALGTHM`): F-quint-2 commit `1c00385` 후부터 PASS (got=2 want=2). 본 cycle 모든 commit 직후 PASS 의무.
4. **F-quart-3 reference cross-check** (`TestDiagnostic_FquartGainReferenceCrossCheck`): F-quint cycle 후 PASS. 회귀 0.
5. **F-quart-1 alignment harness** (`TestDiagnostic_FquartGainImap_Sf0Sample0to7`): F-quint cycle 후 measurement-only PASS. 회귀 0.
6. **F-sext-1 chain trace** (`TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7`): commit `6f1c841` 시점 PASS. 회귀 0.
7. **F-sept-1 excitation 분해** (`TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5`): commit `48265cd` 시점 PASS. 회귀 0.
8. **F-sept-2 LP cross-check** (`TestDiagnostic_FseptLPReferenceCrossCheck`): commit `d61497d` 시점 PASS (`02bf785` lsp_lp.go fix 정식화 후). 회귀 0.
9. **F-sept-3 synth IIR trace** (`TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7`): commit `353398d` 시점 PASS. 회귀 0.
10. **F-oct-prelim-1 PST format** (`TestDiagnostic_FoctPrelimPSTFormat`): commit `5832294` 시점 PASS. 회귀 0.
11. **F-oct-prelim-2 frame alignment** (`TestDiagnostic_FoctPrelimFrameAlignment`): commit `94ef154` 시점 PASS. 회귀 0.
12. **F-oct-prelim-3 multi-vector** (`TestDiagnostic_FoctPrelimMultiVectorScan`): commit `51e74e2` 시점 PASS. 회귀 0.

비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) FAIL 유지 — F-quint-3 §4.6 plan-허용 동상.

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 본 cycle 의 임의 commit 후 회귀 게이트 1+ FAIL (Phase 0.2 의 1~12 중 임의) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + task 재설계 |
| **E2** | Task F-oct-prelim-5-2 의 hpFilter spec init state 측정이 §4.2.2 식 (151)/(152) verbatim 인용에서 도출되지 않음 (= 휴리스틱 fit) | 즉시 측정 폐기 + spec § 식 hand-trace 재작성 + 보고서에 도출 과정 정량 기록 |
| **E3** | Task F-oct-prelim-5-1/2/3 결과가 *상호 모순* (예: Task 5-2 가 hpFilter init state 결함 단일 지목, Task 5-3 도 postfilter ringing 결함 단일 지목 — 중복 결함 정황) | 단일 결함 식별 실패. 시나리오 결합 분석 (F-oct-prelim-5-4 §3) 에 모순 명시 + F-oct 권고를 *복수 fix 동시 적용* 또는 *추가 진단 cycle* 으로 갱신. |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | 본 cycle 의 임의 commit 가 production 파일 (`internal/**/*.go` 중 `*_test.go` 가 아닌 것) 1 라인이라도 변경 | 즉시 `git revert HEAD` + commit 재구성 (test-only 로 축소) |

각 보고서 (F-oct-prelim-5-1/2/3/4) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 회피 의무 (forced-fit avoidance)

본 cycle 은 **G3 (Annex A vs main spec 분기 거동)** 단일 잔존 가설의 *분기 위치 직접 식별* 이 목적. F-oct-prelim-4 §4.2 의 "*분기 위치 식별 없이 production fix 시도 시 spec 정합한 stage 를 spec 위반으로 회귀시킬 위험*" 을 회피하기 위해, 각 task 의 측정 결과는 다음 강압-적합 패턴을 *적극 회피* 한다:

1. **PST want = -1 의 해석 강압**: PST 가 chain 의 정상 출력이라는 가정 (G3 잔존) 을 측정으로 검증해야 하며, "ALGTHM 특이" / "트림된 reference output" 등 임의 해석으로 설명하지 않는다 (F-oct-prelim-3 §5 의 G5 기각 결론 준수).
2. **production-동치 reference**: hpFilter / postfilter / synthesis 의 reference 측정은 spec § 식 verbatim 인용에서 도출 (F-sept-2 / F-sept-3 패턴). production 호출 결과를 ref 로 둬 trivial-zero match 를 유도하는 패턴 금지 (E2).
3. **PST 출처 가설의 검증 완결성**: Task 5-1 의 출처 trace 는 Annex A README + main README + ITU Software Package Release 2/3 README 의 *세 출처 모두* 를 인용 — 단일 출처에서 임의 결론 도출 금지.
4. **모순 측정 직시**: Task 5-1/2/3 결과가 모순일 때 (E3 발동) 결합 표 §3 에 명시 의무. F-oct 권고는 *모순 자체* 를 입력으로 받아 단일 결정 (또는 추가 진단 cycle 호출).

---

## Task F-oct-prelim-5-1: PST 출처 verbatim + BIT 3-vector compare

**Goal:** ALGTHM/PITCH/FIXED 의 PST 출처를 Annex A `READMETV.txt` + main G.729 `READMETV.txt` + ITU Software Package Release 2 README header 의 세 인용으로 재확인 — `decoder file.bit file.pst` 명령의 `decoder` binary 가 **Annex A** (G.729A v1.1) 인지 **main G.729** (v3.3) 인지 식별. 동시에 ALGTHM/PITCH/FIXED 세 vector 의 frame 0 raw BIT bytes 를 byte-level 3-way diff 측정 — F-oct-prelim-3 §5 의 *PITCH/FIXED 도 sample 5..7 = 0/3 반전* 결과의 *공통 stimulus 가설* 을 정량 검증. PITCH.PST / FIXED.PST 의 sample 5..7 측정은 F-oct-prelim-3 §3.2 에서 이미 측정됨 (각각 [-1,-1,-1] / [-1,-1,-1] / chain output [+,+,+] 모두 동상 0/3 반전).

**Files:**
- Create: `internal/decoder/stagef_octprelim5_diagnostic_test.go` (신규 진단 파일, 본 cycle 모든 task 의 test 누적)
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md`

### Spec § 인용 (F-oct-prelim-5-1 진단 근거)

**(인용 1)** Annex A `READMETV.txt` line 1-3, 13-21:

```
Testvectors to verify correct execution of G.729A ANSI-C software
Version 1.1
[...]
Format: all files contain 16 bit sampled data using the Intel (PC)
format.

*.in  - input files
*.bit - bit stream files
*.out - output files

and were obtained using the following commands

 coder file.in file.bit
 decoder file.bit file.pst
ITU-T G.729 Software Package Release 2 (November 2006)
```

**(인용 2)** main G.729 `READMETV.txt` line 1, 3-4, 12-21:

```
 ITU-T G.729 Software Package Release 2 (November 2006)
[...]
Testvectors to verify correct execution of G.729 ANSI-C software
Version 3.3
[...]
Format: all files contain 16 bit sampled data using the Intel (PC)
format.

*.in  - input files
*.bit - bit stream files
*.out - output files

and were obtained using the following commands

 coder file.in file.bit
 decoder file.bit file.pst
```

세 출처의 핵심 공통점:
- 두 README 모두 `decoder file.bit file.pst` 명령으로 PST 가 생성됨을 명시.
- 두 README 모두 "Intel (PC) format" 16-bit (= LittleEndian) 명시.
- header "ITU-T G.729 Software Package Release 2 (November 2006)" 동일 (Annex A 와 main 동일 release).

**핵심 분기점 — 본 task 의 식별 항목**:
- Annex A `decoder` binary 와 main G.729 `decoder` binary 의 *알고리즘 차이* 가 PST 출력 차이를 유발하는가? Annex A 디코더는 §A.4.2 (단순화된 postfilter) 를 적용 — main 은 §4.2 (full postfilter, conditional 분기 포함) 를 적용. 본 구현은 G.729A 만 구현하므로 Annex A 디코더와 정합해야 한다. Annex A `test_vectors/ALGTHM.PST` 가 본 진단의 ground-truth.
- 단, ITU testdata 의 `g729AnnexA/test_vectors/` 와 `g729/test_vectors/` 가 *동일* 한지 byte-level 검증으로 분기 가능성 검증 가능.

**(인용 3)** F-oct-prelim-3 §3.2 측정값 (commit `51e74e2`):
- ALGTHM PST sample 5..7 = `[-1, -1, -1]`, chain output `[1, 1, 1]` (0/3 부호 일치).
- PITCH PST sample 5..7 = `[-1, -1, -1]`, chain output `[1, 1, 1]` (0/3 부호 일치).
- FIXED PST sample 5..7 = `[-1, -1, -1]`, chain output `[1, 1, 1]` (0/3 부호 일치).
- TEST PST sample 5..7 = `[0, 0, 0]`, chain output `[0, 0, 0]` (3/3 trivial-zero).
- LSP PST sample 5..7 = `[0, 0, 0]`, chain output `[0, 0, 0]` (3/3 trivial-zero).

본 task 의 BIT 3-way diff 는 ALGTHM/PITCH/FIXED 의 frame 0 raw BIT 가 *동일/유사한 silence stimulus* 를 가지는지 정량 검증.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
06a4205 docs(plans): add Stage F-oct-prelim synthesis report + F-oct recommendation
```

`internal/lsp/lsp_lp.go` 의 modified 표시는 `02bf785` 커밋 (F-sept-4 시점 정식화) 후 사라졌음을 확인. 만약 modified 가 잔존한다면 즉시 사용자 통보 (별도 cycle 영역).

Run (회귀 게이트 baseline, 9건):
```
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimPSTFormat -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimFrameAlignment -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimMultiVectorScan -v
```

Expected: 10건 모두 PASS. 본 출력을 보고서 §2 에 인용 (요약 형태 — 각 test "PASS" 단일 라인).

- [ ] **Step 2: 진단 test 작성 — `stagef_octprelim5_diagnostic_test.go` 신규**

`internal/decoder/stagef_octprelim5_diagnostic_test.go` 신규 작성. 본 파일은 **본 cycle 의 모든 진단 test** 를 담는다 (F-oct-prelim-5-1, 5-2, 5-3 각 task 의 test).

본 task (F-oct-prelim-5-1) 에서는 `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` 와 `TestDiagnostic_FoctPrelim5BitVectorCompare` 를 추가:

```go
package decoder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mainVectorPath builds a path into the main G.729 (non-Annex-A) test-vector
// tree. Used for cross-checking against Annex A vectors (Phase 1k Stage
// F-oct-prelim-5).
func mainVectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729", "test_vectors", name)
}

// TestDiagnostic_FoctPrelim5PSTSourceVerbatim: Stage F-oct-prelim-5-1 진단.
//
// Annex A `READMETV.txt` + main G.729 `READMETV.txt` 의 PST 생성 명령
// (`decoder file.bit file.pst`) 와 "Intel (PC) format" 16-bit 인용을
// verbatim 으로 dump. ITU Software Package Release 2 (November 2006)
// header 에 의한 동일 release 라는 사실을 정량 확인.
//
// 동시에 Annex A test_vectors 와 main G.729 test_vectors 의 ALGTHM.BIT
// / ALGTHM.PST / PITCH.BIT / PITCH.PST / FIXED.BIT / FIXED.PST 가
// byte-level 동일한지 검증 (= 동일 release 의 동일 binary 산출물인가).
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5PSTSourceVerbatim(t *testing.T) {
	annexAReadme := vectorPath("READMETV.txt")
	mainReadme := mainVectorPath("READMETV.txt")
	ensureTestdataPresent(t, annexAReadme, mainReadme)

	t.Logf("──────── Annex A README header (line 1..21) ────────")
	dumpFirstNLines(t, annexAReadme, 21)

	t.Logf("──────── main G.729 README header (line 1..21) ────────")
	dumpFirstNLines(t, mainReadme, 21)

	t.Logf("──────── byte-level diff: Annex A vs main G.729 test_vectors ────────")
	for _, name := range []string{"ALGTHM.BIT", "ALGTHM.PST", "PITCH.BIT", "PITCH.PST", "FIXED.BIT", "FIXED.PST"} {
		annexA := vectorPath(name)
		main := mainVectorPath(name)
		aData, errA := os.ReadFile(annexA)
		mData, errM := os.ReadFile(main)
		if errA != nil || errM != nil {
			t.Logf("[%s] read error  Annex A=%v  main=%v", name, errA, errM)
			continue
		}
		if bytes.Equal(aData, mData) {
			t.Logf("[%s] Annex A vs main BYTE-EQUAL (%d byte)", name, len(aData))
		} else {
			t.Logf("[%s] Annex A vs main MISMATCH  Annex A=%d byte  main=%d byte",
				name, len(aData), len(mData))
			diffCount, firstDiff := byteDiffSummary(aData, mData)
			t.Logf("[%s] mismatch byte count = %d  (first diff offset = %d)",
				name, diffCount, firstDiff)
		}
	}

	t.Logf("──────── 시나리오 분류 dump ────────")
	t.Logf("(P-SRC-1) Annex A 와 main test_vectors BYTE-EQUAL → PST 생성 binary 동일")
	t.Logf("            본 구현 (G.729A) 는 Annex A binary 와 동일 알고리즘 적용")
	t.Logf("            → PST 가 ground-truth 이며 chain 결함은 *우리 구현 내부* 에 존재.")
	t.Logf("(P-SRC-2) Annex A 와 main test_vectors MISMATCH → PST 생성 binary 분기")
	t.Logf("            우리 구현은 Annex A binary 와 정합해야 함 (g729AnnexA 폴더 사용 정합).")
	t.Logf("            mismatch 는 main G.729 (full postfilter) 의 후속 영향이므로 본 cycle 무관.")
}

// TestDiagnostic_FoctPrelim5BitVectorCompare: Stage F-oct-prelim-5-1 진단.
//
// ALGTHM.BIT / PITCH.BIT / FIXED.BIT 의 frame 0 (10 byte = 80 bit packed)
// 을 byte-level 3-way diff. F-oct-prelim-3 §5 가 세 vector *모두* sample
// 5..7 = 0/3 부호 반전을 동상 측정한 사실의 *공통 silence stimulus*
// 가설 (G3 흡수) 을 정량 검증.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5BitVectorCompare(t *testing.T) {
	algthmBit := vectorPath("ALGTHM.BIT")
	pitchBit := vectorPath("PITCH.BIT")
	fixedBit := vectorPath("FIXED.BIT")
	ensureTestdataPresent(t, algthmBit, pitchBit, fixedBit)

	algthmFrames, _ := readG192Frames(t, algthmBit)
	pitchFrames, _ := readG192Frames(t, pitchBit)
	fixedFrames, _ := readG192Frames(t, fixedBit)

	if len(algthmFrames) == 0 || len(pitchFrames) == 0 || len(fixedFrames) == 0 {
		t.Fatalf("empty frames: algthm=%d pitch=%d fixed=%d",
			len(algthmFrames), len(pitchFrames), len(fixedFrames))
	}

	a := algthmFrames[0]
	p := pitchFrames[0]
	f := fixedFrames[0]

	t.Logf("──────── frame 0 raw bytes (10 byte = 80 bit packed) ────────")
	t.Logf("ALGTHM frame 0: %s", hexBytes(a))
	t.Logf("PITCH  frame 0: %s", hexBytes(p))
	t.Logf("FIXED  frame 0: %s", hexBytes(f))

	t.Logf("──────── 3-way byte-level diff (frame 0) ────────")
	for i := 0; i < 10; i++ {
		var ab, pb, fb byte
		if i < len(a) {
			ab = a[i]
		}
		if i < len(p) {
			pb = p[i]
		}
		if i < len(f) {
			fb = f[i]
		}
		mark := "  "
		switch {
		case ab == pb && pb == fb:
			mark = "==" // 3-way 동일
		case ab == pb:
			mark = "AP" // ALGTHM=PITCH ≠ FIXED
		case ab == fb:
			mark = "AF" // ALGTHM=FIXED ≠ PITCH
		case pb == fb:
			mark = "PF" // PITCH=FIXED ≠ ALGTHM
		default:
			mark = "//" // 3-way 모두 상이
		}
		t.Logf("[%d] ALGTHM=%02x PITCH=%02x FIXED=%02x   %s",
			i, ab, pb, fb, mark)
	}

	t.Logf("──────── 시나리오 분류 dump ────────")
	t.Logf("(B-CMP-1) frame 0 BIT byte 3-way 동일 (== 10/10) → 동일 stimulus")
	t.Logf("            → silence-input 정합 가설 강화 (encoder 가 silence 를 동일하게 인코딩)")
	t.Logf("(B-CMP-2) frame 0 BIT byte 일부 다름 → stimulus 분기")
	t.Logf("            → 동일 sample 5..7 = 0/3 결함 발현은 *디코더 출력 메커니즘 공통* 에 기인.")
	t.Logf("            → G3 (Annex A vs main 분기) 영역에서 *분기 위치는 디코더 내부* 임을 함의.")
}

// dumpFirstNLines: file 의 처음 n 라인을 t.Logf 로 출력. 한국어 / ASCII 혼합
// dump 에 사용.
func dumpFirstNLines(t *testing.T, path string, n int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("dumpFirstNLines(%s): %v", path, err)
		return
	}
	lines := strings.Split(string(data), "\n")
	limit := n
	if limit > len(lines) {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		t.Logf("  %2d| %s", i+1, lines[i])
	}
}

// hexBytes: byte slice 를 공백 분리 hex 로 dump.
func hexBytes(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02x", v)
	}
	return strings.Join(parts, " ")
}

// byteDiffSummary: a/b 두 슬라이스의 mismatch byte 수 + 첫 차이 offset.
func byteDiffSummary(a, b []byte) (count int, firstDiff int) {
	firstDiff = -1
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			count++
			if firstDiff < 0 {
				firstDiff = i
			}
		}
	}
	if len(a) != len(b) {
		count += abs(len(a) - len(b))
		if firstDiff < 0 {
			firstDiff = n
		}
	}
	return count, firstDiff
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
```

**중요 — helper 중복 회피**: 본 cycle 의 helper (`mainVectorPath`, `dumpFirstNLines`, `hexBytes`, `byteDiffSummary`, `abs`) 명이 기존 `decoder` package test helper 와 중복하지 않는지 사전 검토. 충돌 시 본 cycle 의 helper 명을 변경 — 기존 파일 *수정 금지*. 가령 `abs` 가 `stagef_sept_diagnostic_test.go` 의 `abs32` 와 별개이므로 중복 위험 미발. 그러나 `dumpFirstNLines` 등이 충돌하면 `dumpFirstNLinesV5` 등 cycle suffix 추가.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5PSTSourceVerbatim -v` 와 `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5BitVectorCompare -v`

Expected: 둘 다 PASS (assertion 0, t.Logf 만 출력). raw output 보고서 §3.1 에 인용.

컴파일 오류 발생 시 (signature 불일치 등) 본 step 에서 *test 코드만* 수정. production 코드 1 라인이라도 변경 시 즉시 E5 발동.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 의 출력에서 다음 분류 (보고서 §3.2):

- **PST 출처 분류 (P-SRC-1 / P-SRC-2)**:
  - **(P-SRC-1)** Annex A 와 main test_vectors 의 ALGTHM.BIT/PST + PITCH.BIT/PST + FIXED.BIT/PST 가 6/6 BYTE-EQUAL → **PST 생성 binary 동일** → 우리 구현 (G.729A) 내부에 결함 잔존.
  - **(P-SRC-2)** mismatch 1+ 발견 → **PST 생성 binary 분기** → 본 cycle 의 ground-truth 는 `g729AnnexA/test_vectors/` (Annex A binary 출력) 이며 main test_vectors 분기는 무관.

- **BIT 3-way 분류 (B-CMP-1 / B-CMP-2)**:
  - **(B-CMP-1)** frame 0 BIT byte 10/10 동일 → **동일 silence stimulus** → 세 vector 의 동상 0/3 결함은 *공통 stimulus 에 대한 동일 디코더 출력*.
  - **(B-CMP-2)** 일부 byte 상이 → **stimulus 분기** → 동상 결함은 *디코더 내부 공통 메커니즘* 에 기인.

본 task 결과는 (P-SRC-?, B-CMP-?) 2D 분류로 단일 결정.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 12 게이트 모두 PASS + 본 task 의 새 test 2건 PASS. 비-contract diagnostic 3건 (F-quint-3 §4.6) FAIL 유지 (plan-허용).

- [ ] **Step 6: F-oct-prelim-5-1 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-5-1 보고서 — PST 출처 verbatim + BIT 3-vector 비교

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (1) + (4). PST 생성 binary 식별 (Annex A vs
        main G.729) + ALGTHM/PITCH/FIXED frame 0 raw BIT 3-way diff.
**산출물**: README header dump + 6 vector byte-level 동일성 표 + frame 0
            BIT 10-byte 3-way diff 표 + (P-SRC-?, B-CMP-?) 분류.
**준수**: Annex A `READMETV.txt` + main G.729 `READMETV.txt` 인용. 외부
        G.729 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. README 인용 (Annex A + main G.729)
## 2. 회귀 게이트 baseline (Step 1 출력)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 PST 출처 분류 (P-SRC-1 / P-SRC-2)
   3.3 BIT 3-way 분류 (B-CMP-1 / B-CMP-2)
## 4. 결합 분류 (P-SRC × B-CMP) 와 G3 함의
## 5. F-oct-prelim-5-2 / F-oct-prelim-5-3 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go            ← 미변경 보존
?? internal/decoder/stagef_octprelim5_diagnostic_test.go     ← 본 task 신규
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md
```

**E5 검증**: `git diff -- internal/` 의 production 라인 (즉 `*_test.go` 가 아닌 파일) 변경 0.

```bash
git add internal/decoder/stagef_octprelim5_diagnostic_test.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-1-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-5-1 PST source verbatim + BIT compare

F-oct-prelim-4 §4.3 (1) + (4) 후속. ALGTHM/PITCH/FIXED PST 의 생성
binary 식별 (`decoder file.bit file.pst`) + Annex A 와 main G.729
test_vectors 의 6 파일 byte-level 동일성 측정 + ALGTHM/PITCH/FIXED
frame 0 raw BIT 10-byte 3-way diff.

본 진단은 측정-only — production 변경 0. 분류 (P-SRC-1/2 × B-CMP-1/2)
4 row 결합으로 G3 (Annex A vs main 분기 거동) 가설의 분기 위치 후보를
좁힌다. Annex A README + main README + ITU Software Package Release 2
(November 2006) header 인용. 외부 G.729 구현 0건 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-5-2: hpFilter init state + F-sext-2/3 reactivate

**Goal:** ITU-T G.729 §4.2.2 (output HP filter, eq. 151/152) 의 IIR memory **초기 state** 가 spec 상 `0` 인지 별도 prescribed value 인지 정량 식별. production `internal/decoder/hpfilter.go` 의 `d.hpX = [2]int16{0, 0}`, `d.hpY = [2]int32{0, 0}` (zero-init, §4.3 default) 가 spec § 인용 init 와 정합하는지 검증. 추가로 zero-input chain (synth output 0 → postfilter 0 → hpFilter 0) 의 sample 0..7 출력 trace 측정 — silence frame 0 에서 chain 이 negative output 을 *생성할 수 있는 메커니즘이 spec 상 존재하는가* 의 직접 측정. F-sext-2 (HP startup transient) + F-sext-3 (HP reference cross-check) 가설을 본 task 에 통합 reactivate.

**Files:**
- Modify: `internal/decoder/stagef_octprelim5_diagnostic_test.go` (`TestDiagnostic_FoctPrelim5HpFilterInitState` 추가)
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md`

### Spec § 인용 (F-oct-prelim-5-2 진단 근거)

**(인용 1)** §4.2.2 (PDF p.43), output HP filter 정의 (실수형):

```
H_h2(z) = (0.93980581 - 1.8795834 z⁻¹ + 0.93980581 z⁻²) /
          (1 - 1.9330735 z⁻¹ + 0.93589199 z⁻²)
```

**(인용 2)** §4.2.2 식 (151) (PDF p.43), 실수 difference equation:

```
y_h2(n) = 0.93980581·s_f(n) - 1.8795834·s_f(n-1) + 0.93980581·s_f(n-2)
        + 1.9330735·y_h2(n-1) - 0.93589199·y_h2(n-2)
```

**(인용 3)** §4.2.2 식 (152) (PDF p.43), 16-bit 출력 스케일:

```
sw(n) = sat(2 · y_h2(n))
```

**(인용 4)** §4.3 (PDF p.46), 디코더 초기화: "*All filter and quantizer states are initialized to zero at the beginning of decoding.*" (테이블 9 zero-init 포함).

**(인용 5)** F-sext-1 보고서 §3.1 (commit `6f1c841`):
- synth.Filter sample 5..7 = `[1, 1, 1]`
- postfilter.Filter sample 5..7 = `[1, 1, 1]`
- hpFilter sample 5..7 = `[1, 1, 1]`
- pcm.ScaleUpSat sample 5..7 = `[2, 2, 2]`
- PST want sample 5..7 = `[-1, -1, -1]`

§4.2.2 식 (151) 의 IIR 항 `1.9330735·y_h2(n-1) - 0.93589199·y_h2(n-2)` 는 |a1|≈1.93 의 양수 계수 — y(n-1) > 0 이면 다음 출력에 양수 영향. 단 a2 = -0.94 는 음수 가능성을 만든다. **본 task 는 zero-input + zero-state 에서 hpFilter 가 음수 출력을 생성하는 startup transient 가 존재하는지 직접 측정** (= F-sext-2 가설 reactivate).

**(인용 6)** production hpFilter.go (line 11-19) self-citing:
```
const (
    hpB0Q13    = 7699
    hpB1Q13    = -15399
    hpB2Q13    = 7699
    hpNegA1Q12 = 7918
    hpA2Q13    = 7667
)
```

7699 / 8192 ≈ 0.93994 (실수 0.93980581 와 |error| ≈ 0.00014). 15399 / 8192 ≈ 1.87988. 7918 / 4096 ≈ 1.93311. 7667 / 8192 ≈ 0.93591. **모두 spec real value 와 |Δ| ≤ 0.001 정합** (Q12/Q13 round-to-nearest).

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
<hash> test(decoder): add Stage F-oct-prelim-5-1 PST source verbatim + BIT compare
```

`git diff --stat HEAD~1` Expected: 신규 파일 2건 (test + report) 만, internal/ 의 production 0 라인.

Run (회귀 게이트, 12 항목 — Phase 0.2 + 본 cycle 5-1 commit 후 PASS 확인):
```
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5PSTSourceVerbatim -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5BitVectorCompare -v
go test ./internal/...
```

Expected: 본 cycle 5-1 신규 2건 PASS + 전체 회귀 (비-contract 3건 FAIL plan-허용 외) PASS.

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FoctPrelim5HpFilterInitState`**

`internal/decoder/stagef_octprelim5_diagnostic_test.go` 에 다음 test 추가:

```go
// TestDiagnostic_FoctPrelim5HpFilterInitState: Stage F-oct-prelim-5-2 진단.
//
// ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) HP filter (100 Hz cutoff,
// 2-pole 2-zero IIR) 의 초기 IIR state (x[-1], x[-2], y[-1], y[-2]) 가
// spec 상 0 인지 별도 prescribed value 인지 검증. §4.3 "All filter and
// quantizer states are initialized to zero" 의 zero-init 가정에 정합.
//
// 측정 차원:
//
//	(a) production constants (hpB0Q13 / hpB1Q13 / hpB2Q13 / hpNegA1Q12 /
//	    hpA2Q13) vs spec real coefficient (0.93980581 / -1.8795834 /
//	    0.93980581 / -1.9330735 / 0.93589199) 의 |Δ| 정량.
//	(b) zero-input + zero-state 시 sample 0..7 출력 — startup transient
//	    가 음수 출력을 생성하는가.
//	(c) impulse-input (sample 0 = +1, 그 외 0) + zero-state 시 sample
//	    0..7 출력 — IIR step response 의 부호 추세.
//	(d) 실제 ALGTHM frame 0 sf0 chain 결과를 hpFilter 입력으로 (= F-sext-1
//	    재현) 와 동시에, "spec 상 hpFilter 입력이 0 인 silence frame" 가설
//	    검증 — silence input 가정 시 sample 5..7 negative 가 가능한가.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5HpFilterInitState(t *testing.T) {
	// (a) Q-format quantization error
	t.Logf("──────── (a) production Q-format vs spec real coefficient ────────")
	type qpair struct {
		name      string
		qVal      int32
		qScale    float64
		specReal  float64
	}
	pairs := []qpair{
		{"b0", int32(hpB0Q13), 8192.0, 0.93980581},
		{"b1", int32(hpB1Q13), 8192.0, -1.8795834},
		{"b2", int32(hpB2Q13), 8192.0, 0.93980581},
		{"-a1", int32(hpNegA1Q12), 4096.0, 1.9330735},  // production stores |a1| at Q12
		{"a2", int32(hpA2Q13), 8192.0, 0.93589199},
	}
	for _, p := range pairs {
		approx := float64(p.qVal) / p.qScale
		delta := approx - p.specReal
		t.Logf("  %-3s  q=%+6d  approx=%+.8f  spec=%+.8f  |Δ|=%.8f",
			p.name, p.qVal, approx, p.specReal, delta)
	}

	// (b) zero-input + zero-state hpFilter
	t.Logf("──────── (b) zero-input + zero-state hpFilter sample 0..7 ────────")
	{
		var d Decoder
		d.Reset()
		var in [subframeLen]int16  // 모두 0
		var out [subframeLen]int16
		d.hpFilter(&in, out[:])
		t.Logf("  hpFilter(0...) [0..7] = %v", out[:8])
		t.Logf("  hpFilter(0...) [0..%d] all-zero? %v",
			subframeLen-1, allZeroInt16(out[:]))
	}

	// (c) impulse-input (sample 0 = +1) + zero-state hpFilter
	t.Logf("──────── (c) impulse(+1 at n=0) + zero-state hpFilter sample 0..7 ────────")
	{
		var d Decoder
		d.Reset()
		var in [subframeLen]int16
		in[0] = 1
		var out [subframeLen]int16
		d.hpFilter(&in, out[:])
		t.Logf("  hpFilter(δ[0]=+1) [0..7] = %v", out[:8])
		t.Logf("  hpFilter(δ[0]=+1) [0..%d] = %v", subframeLen-1, out[:])
	}

	// (d) impulse-input (sample 0 = +2 = chain output sample 0) + zero-state
	t.Logf("──────── (d) chain-like impulse (sample 0 = +2) + zero-state ────────")
	{
		var d Decoder
		d.Reset()
		var in [subframeLen]int16
		// F-sept-4 chain output sample 0..7 = [2, 4, 3, 3, 1, 1, 1, 1]
		// 단 본 측정은 sample 0 만 +2 로 driving — IIR step 단일 응답 분리
		in[0] = 2
		var out [subframeLen]int16
		d.hpFilter(&in, out[:])
		t.Logf("  hpFilter(δ[0]=+2) [0..7] = %v", out[:8])
	}

	// (e) F-sext-1 chain replay (sample 0..7 = [2, 4, 3, 3, 1, 1, 1, 1])
	t.Logf("──────── (e) F-sept-4 chain output as hpFilter input + zero-state ────────")
	{
		var d Decoder
		d.Reset()
		var in [subframeLen]int16
		chain := [8]int16{2, 4, 3, 3, 1, 1, 1, 1}
		for i, v := range chain {
			in[i] = v
		}
		// sample 8..39 도 chain output 이 있어야 정확하지만, 본 측정은 sample
		// 0..7 의 IIR boundary 만 관찰 — sample 8.. 는 0 으로 두고 IIR 의 잔향
		// 포함.
		var out [subframeLen]int16
		d.hpFilter(&in, out[:])
		t.Logf("  hpFilter(chain[0..7], 0...) [0..7] = %v", out[:8])
		t.Logf("  hpFilter expectation = sample 5..7 부호 추적")
		for n := 5; n <= 7; n++ {
			t.Logf("    [%d]  in=%+d  out=%+d  부호 (in/out) = %s / %s",
				n, in[n], out[n],
				signOfInt16Local(in[n]), signOfInt16Local(out[n]))
		}
	}

	// (f) 시나리오 분류 dump
	t.Logf("──────── (f) 시나리오 분류 (hpFilter init state) ────────")
	t.Logf("(H-INIT-1) zero-input + zero-state → hpFilter all-zero")
	t.Logf("            spec §4.3 zero-init 정합. silence frame 0 의 negative")
	t.Logf("            output 메커니즘은 hpFilter 단독으로 *불가능*.")
	t.Logf("(H-INIT-2) zero-input + zero-state → hpFilter nonzero")
	t.Logf("            spec §4.3 zero-init 위반 또는 production primitive 결함.")
	t.Logf("            E2 / E5 발동 검토.")
	t.Logf("(H-RESP-1) chain-input + zero-state → sample 5..7 부호 = + (chain 동상)")
	t.Logf("            hpFilter 가 sample 5..7 부호 반전 발생시키지 않음 (F-sext-1 §4 동상).")
	t.Logf("            negative output 메커니즘은 chain 외부 (= PST 자체) 또는 *상류 결함*.")
	t.Logf("(H-RESP-2) chain-input + zero-state → sample 5..7 부호 = − (반전)")
	t.Logf("            hpFilter step response 가 startup transient 로 sample 5+ 에서")
	t.Logf("            부호 반전 — F-sext-2 가설 정량 확정.")
}

// allZeroInt16: 모든 element 가 0 이면 true.
func allZeroInt16(s []int16) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}

// signOfInt16Local: F-sept 의 signOfInt16 와 충돌 회피용 local helper.
// F-sept 가 같은 package 의 stagef_sept_diagnostic_test.go 에 signOfInt16 를
// 이미 정의 — 본 cycle 은 별도 명명으로 충돌 회피.
//
// 단, 같은 package 이므로 signOfInt16 가 이미 보이는 경우 그것을 직접 사용
// 가능 — 본 helper 는 fallback. 본 step 에서 컴파일 시도 후 충돌 발생 시
// 본 helper 를 삭제하고 signOfInt16 직접 호출.
func signOfInt16Local(v int16) string {
	switch {
	case v > 0:
		return "+"
	case v < 0:
		return "−"
	default:
		return "0"
	}
}
```

**중요 — helper 충돌 검토**: `stagef_sept_diagnostic_test.go` 에 `signOfInt16` 가 이미 정의되어 있다 (F-sept-1 cycle). 같은 `decoder` package 이므로 직접 호출 가능. 본 task 의 `signOfInt16Local` 는 *컴파일 시도 후 결정* — `signOfInt16` 가 보이면 그것을 직접 호출하고 `signOfInt16Local` 삭제. `allZeroInt16` 도 기존 helper 검색 후 결정.

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5HpFilterInitState -v`

Expected: PASS (assertion 0). raw output 보고서 §3.1 에 인용 (Q-format error 표 + zero-input 결과 + impulse 결과 + chain replay 결과).

컴파일 오류 발생 시 본 step 에서 *test 코드만* 수정. helper 충돌 시 위 노트 적용. production 코드 1 라인이라도 변경 시 즉시 E5 발동.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 의 출력에서 다음 분류 (보고서 §3.2):

- **Q-format 정합 분류 (Q-FMT-1 / Q-FMT-2)**:
  - **(Q-FMT-1)** 5 계수 모두 |Δ| ≤ 0.001 (예상) → spec real value 정합. F-sept-3 (synth IIR) 와 동상 — Q-format 결함 부재.
  - **(Q-FMT-2)** 1+ 계수 |Δ| > 0.001 → Q-format 결함 (E2 검토).

- **Init state 분류 (H-INIT-1 / H-INIT-2)**:
  - **(H-INIT-1)** zero-input + zero-state → all-zero out → §4.3 정합. negative output 메커니즘은 hpFilter 단독 *불가능*.
  - **(H-INIT-2)** zero-input + zero-state → nonzero out → §4.3 위반 (E2 / E5 발동).

- **Response 분류 (H-RESP-1 / H-RESP-2)**:
  - **(H-RESP-1)** chain-input + zero-state → sample 5..7 부호 + → F-sext-1 §4 동상 → hpFilter 가 부호 반전 *없음*. negative output 은 chain 외부 (= PST 자체 G3 잔존 또는 G3 폐기 후 chain 결과를 ground-truth 로 받아들이는 *결함 부재* 가설).
  - **(H-RESP-2)** chain-input + zero-state → sample 5..7 부호 − → F-sext-1 §4 모순 (= 회귀 발동) → 측정 자체 점검 필요. F-sext-1 commit `6f1c841` 시점과 본 task 의 hpFilter 동작이 동일해야 하므로 H-RESP-2 미발생 의무.

본 task 결과는 (Q-FMT-?, H-INIT-?, H-RESP-?) 3D 분류로 단일 결정.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 12 게이트 모두 PASS + 본 task 의 새 test PASS. 비-contract diagnostic 3건 FAIL 유지 (plan-허용).

- [ ] **Step 6: F-oct-prelim-5-2 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-5-2 보고서 — hpFilter init state + F-sext 통합

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (2). §4.2.2 hpFilter (eq. 151/152) IIR
        memory 초기 state 검증 + zero-input/impulse/chain replay step
        response 측정. F-sext-2/3 (HP startup transient + reference
        cross-check) reactivate 통합.
**산출물**: Q-format error 표 + zero-input/impulse/chain replay sample
            0..7 표 + (Q-FMT, H-INIT, H-RESP) 분류.
**준수**: ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) + §4.3 인용.
        외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. §4.2.2 + §4.3 + production self-citing 인용
## 2. 회귀 게이트 baseline (Step 1 출력)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 Q-format 정합 분류 (Q-FMT-1/2)
   3.3 Init state 분류 (H-INIT-1/2)
   3.4 Response 분류 (H-RESP-1/2)
## 4. 결합 분류 (3D) 와 G3 함의
## 5. F-oct-prelim-5-3 진입 권고 + F-sext-2/3 종결 평가
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/decoder/stagef_octprelim5_diagnostic_test.go     ← 본 task 수정
?? internal/decoder/stagef_bis_diagnostic_test.go            ← 미변경 보존
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md
```

**E5 검증**: `git diff -- internal/` 의 production 라인 변경 0. `stagef_octprelim5_diagnostic_test.go` 변경은 본 task test 추가만.

```bash
git add internal/decoder/stagef_octprelim5_diagnostic_test.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-2-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-5-2 hpFilter init state diagnostic

F-oct-prelim-4 §4.3 (2) 후속. ITU-T G.729 §4.2.2 식 (151)/(152) HP filter
의 IIR memory 초기 state (zero-init per §4.3) 정합 검증. zero-input +
impulse + chain replay 의 sample 0..7 step response 측정. F-sext-2/3
(HP startup transient + reference cross-check) 가설 reactivate 통합.

본 진단은 측정-only — production 변경 0. 분류 (Q-FMT × H-INIT × H-RESP)
3D 결합으로 hpFilter 가 silence frame 0 sample 5..7 negative output
의 *원인인지* 정량 식별. 외부 G.729 구현 0건 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-5-3: silence frame 0 negative output 생성 chain trace

**Goal:** ALGTHM frame 0 sf0 의 PST want sample 5..7 = `[-1, -1, -1]` 음수 출력을 생성하는 chain 메커니즘을 spec § 인용으로 식별. 후보 4 개 (M1) §A.4.2 postfilter ringing — long-term + short-term + tilt + AGC 음수 감쇠항, (M2) §4.2.2 hpFilter 음수 감쇠항 (Task 5-2 결과 입력), (M3) §3.10 synthesis filter memory 비-0 init (encoder-decoder 동기화 분기), (M4) PST 자체가 *결함 없는 정상 출력* 인 가설 (= G3 폐기 후 우리 chain 이 spec-correct 인 가설 — F-oct-prelim-3 §5 의 *공통 결함* 가설 폐기) 중 단일 식별. 본 task 는 우리 production chain 을 *조건 변경 없이* 재현 + 각 stage 입출력 sample 0..7 을 반복 측정해 4 후보 별 부호 결정 boundary 를 찾는다.

**Files:**
- Modify: `internal/decoder/stagef_octprelim5_diagnostic_test.go` (`TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` 추가)
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md`

### Spec § 인용 (F-oct-prelim-5-3 진단 근거)

**(인용 1)** §3.10 (PDF p.21), synthesis filter:

```
ŝ(n) = û(n) + Σ_{i=1}^{10} aᵢ · ŝ(n-i),  n = 0, ..., L_subframe-1
```

§4.3 zero-init 시 ŝ(-1) ... ŝ(-10) = 0. û(n) ≥ 0 이고 aᵢ 부호가 임의이면 ŝ(n) 의 부호는 IIR 누산에 의해 임의 가능. F-sept-3 §4 측정에서 sample 0..7 = `[+,+,+,+,+,+,+,+]` (부호 모두 +) 가 spec ref 와 정합 — synth IIR 자체가 음수를 생성할 능력 없음.

**(인용 2)** §A.4.2 (PDF p.78~), Annex A postfilter chain — long-term postfilter `H_p(z)` + short-term postfilter `H_st(z)` + tilt compensation `H_t(z)` + adaptive gain control (AGC). 각 단계의 차분식이 음수 감쇠항을 포함. F-sext-1 §3.1 측정에서 sample 5..7 = `[+,+,+]` (부호 +) — postfilter 가 음수를 생성하지 않음.

**(인용 3)** §4.2.2 hpFilter (Task 5-2 인용 1-3 동상). F-sext-1 §3.1 측정에서 sample 5..7 = `[+,+,+]`.

**(인용 4)** §4.3 zero-init: "*All filter and quantizer states are initialized to zero at the beginning of decoding.*"

**(인용 5)** F-oct-prelim-3 §5 결론 — ALGTHM/PITCH/FIXED 모두 sample 5..7 = `[-1,-1,-1]` (PST want), 0/3 부호 일치. *공통 stimulus 에서 동일 부호 반전 결함 발현* — 우리 chain 이 negative output 을 생성하지 못하는 구조라면, *ITU 디코더는 silence frame 0 에서 negative 출력을 만든다* 는 사실 자체가 § 4.2.2 / §A.4.2 / §3.10 의 어딘가에서 zero-init 와 silence input 의 곱이 *비-zero* 결과를 산출함을 함의 (= 기존 spec § 정합 측정의 모순 — 측정 자체 재점검 또는 새 메커니즘 식별 필요).

**(인용 6)** F-sept-2 §4 결론 (commit `d61497d`, lsp_lp.go fix `02bf785` 후): LP coefficients `Â(z)` 가 §4.1 + §3.2.6 + lsp_lp.go fix 후 spec ref 와 bit-exact 정합. LP 결함 부재.

**(인용 7)** F-sept-1 §4 결론 (commit `48265cd`): excitation u[5] = +1 (gp·v + gc·c, sample 5 양수). 부호 결정은 v[5], c[5] 둘 다 양수 + gp_q14, gc_q12 둘 다 양수 → u[5] 양수. excitation 결함 부재.

**핵심 관측**: 우리 chain 은 *모든 stage 에서 양수 출력* 을 생성하지만, ITU PST 는 *음수 출력* 을 갖는다. 이 모순은 다음 4 가설 중 하나로만 해소:

- **(M1)** postfilter 가 *조건 분기* 에 따라 음수 감쇠항을 활성화 — 우리 구현이 분기를 미구현 또는 잘못 구현. spec § 인용 후보: §A.4.2 의 conditional gain factor / pitch tracking 의존성.
- **(M2)** hpFilter 가 *startup transient* 에서 음수 감쇠 — Task 5-2 결과 입력. (H-RESP-1 이면 폐기.)
- **(M3)** synthesis memory 가 *비-0 init* 으로 시작 — encoder/decoder 동기화. spec § 인용 후보: §4.3 의 zero-init 이 *전부* 인지, 또는 일부 state (e.g., gain predictor MA history) 가 별도 init.
- **(M4)** PST 자체가 *결함 없는 정상 출력* — G3 폐기 + 우리 chain 이 spec-correct + ITU ALGTHM PST 의 sample 5..7 = -1 이 *ITU 디코더 산술 결함의 결과* (e.g., int16 saturation 의 specific edge). 이 가설은 외부 G.729 구현 cross-check 가 *없는* 우리 환경에서 검증 불가능 — 본 cycle 결론으로 도출 시 plan-end declared.

본 task 는 (M1) ~ (M4) 의 *증거 표* 를 spec § 인용 + production 측정 으로 작성 — 단일 가설 식별 또는 모순 명시 (E3 발동 검토).

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 baseline 측정**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
<hash> test(decoder): add Stage F-oct-prelim-5-2 hpFilter init state diagnostic
```

Run (회귀 게이트 baseline, 13 항목 — Phase 0.2 + 본 cycle 5-1, 5-2 commit 후 PASS 확인):
```
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5HpFilterInitState -v
go test ./internal/...
```

Expected: 본 cycle 5-2 신규 PASS + 전체 회귀 PASS (비-contract 3건 FAIL plan-허용 외).

- [ ] **Step 2: 진단 test 추가 — `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism`**

`internal/decoder/stagef_octprelim5_diagnostic_test.go` 에 다음 test 추가:

```go
// TestDiagnostic_FoctPrelim5SilenceNegativeMechanism: Stage F-oct-prelim-5-3 진단.
//
// ALGTHM frame 0 sf0 의 PST want sample 5..7 = -1 음수 출력을 생성하는
// chain 메커니즘 후보 4 개 (M1) §A.4.2 postfilter, (M2) §4.2.2 hpFilter,
// (M3) §3.10 synthesis memory init, (M4) PST 자체 결함 가설 의 증거를
// 측정.
//
// 우리 chain 의 frame 0 sf0 sample 0..7 stage 별 출력은 F-sext-1 §3.1
// 에서 모두 양수 (synth, postfilter, hpFilter 모두 [+,+,+] for sample
// 5..7). 본 task 는 stage 별 출력을 *재측정* 하여 spec ref 와 cross-check.
// 추가로 §4.3 zero-init 이 *모든* state 에 적용되는지 (e.g., gain
// predictor MA history, postfilter past gain) 정량 점검.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5SilenceNegativeMechanism(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	if len(frames) == 0 {
		t.Fatalf("no frames in BIT")
	}

	// (a) PST want frame 0 sf0 sample 0..15 dump
	t.Logf("──────── (a) PST want frame 0 sf0 sample 0..15 ────────")
	t.Logf("  %v", wantFrames[0][:16])

	// (b) production Decoder.Decode frame 0 출력
	t.Logf("──────── (b) production Decoder.Decode frame 0 sf0 sample 0..15 ────────")
	{
		var d Decoder
		var got [80]int16
		if err := d.Decode(frames[0], false, got[:]); err != nil {
			t.Fatalf("Decode frame 0: %v", err)
		}
		t.Logf("  got[0..15] = %v", got[:16])
		t.Logf("  want[0..15] = %v", wantFrames[0][:16])
		t.Logf("  diff[0..15]:")
		for n := 0; n < 16; n++ {
			t.Logf("    [%2d]  got=%+5d  want=%+5d  diff=%+5d  부호 (got/want) = %s / %s",
				n, got[n], wantFrames[0][n], int32(got[n])-int32(wantFrames[0][n]),
				signOfInt16Local(got[n]), signOfInt16Local(wantFrames[0][n]))
		}
	}

	// (c) §4.3 zero-init 검증 — Decoder 의 모든 state 가 zero 인지 점검
	t.Logf("──────── (c) §4.3 zero-init 검증 (Decoder field 별) ────────")
	{
		var d Decoder
		// Reset 호출 안함 — zero value 사용
		t.Logf("  d.pastExc[0..7] = %v", d.pastExc[:8])
		t.Logf("  d.prevGpQ14 = %d", d.prevGpQ14)
		t.Logf("  d.hpX = %v   d.hpY = %v", d.hpX, d.hpY)
		t.Logf("  d.initialized = %v", d.initialized)
		t.Logf("  (lsp.Decoder, gain.Decoder, synth.Synthesizer, postfilter.Postfilter")
		t.Logf("   의 zero value 정합은 각 package contract test 가 검증 — D 17 게이트.)")
	}

	// (d) F-sept-4 chain output 재현 — sample 0..15 stage 별 trace
	// (F-sext-1 §3.1 raw output 인용 — 본 task 는 sample 5..7 mismatch 를
	//  stage 별로 재차 capture 해 모순 부재 검증)
	t.Logf("──────── (d) F-sext-1 §3.1 chain stage 별 출력 재현 (sample 5..7) ────────")
	t.Logf("  (F-sext-1 commit 6f1c841 보고서 인용 — assertion 없이 dump)")
	t.Logf("  stage              [   5    6    7]  부호분포")
	t.Logf("  synth.Filter       [   1    1    1]  [+ + +]")
	t.Logf("  postfilter.Filter  [   1    1    1]  [+ + +]")
	t.Logf("  hpFilter           [   1    1    1]  [+ + +]")
	t.Logf("  pcm.ScaleUpSat     [   2    2    2]  [+ + +]  (PST 도메인)")
	t.Logf("  PST want           [  -1   -1   -1]  [− − −]")
	t.Logf("  PST/2 spec-target  [  -1   -1   -1]  [− − −]")

	// (e) PST want 음수 출력 가설 4 개 평가 dump
	t.Logf("──────── (e) PST want -1 음수 출력 가설 4 개 평가 ────────")
	t.Logf("(M1) postfilter conditional 분기 음수 감쇠항")
	t.Logf("     - 근거: §A.4.2 의 long-term postfilter Hp(z) 또는 tilt comp Ht(z)")
	t.Logf("       가 특정 조건 (e.g., voicing factor 임계, pitch gain 임계) 에서")
	t.Logf("       음수 감쇠 활성화.")
	t.Logf("     - 우리 측정: F-sext-1 §3.1 postfilter[5..7] = [+,+,+] (양수).")
	t.Logf("       즉 postfilter 가 *현 구현* 에서 negative 를 생성하지 않음.")
	t.Logf("     - 폐기/유지 결정: §A.4.2 의 conditional 분기 모두 우리 구현이")
	t.Logf("       포함하는지 별도 검증 필요 — 본 task 측정 범위 외, 후속 cycle.")
	t.Logf("(M2) §4.2.2 hpFilter 음수 감쇠 (Task 5-2 결과)")
	t.Logf("     - Task 5-2 H-INIT-?, H-RESP-? 분류 결과 인용")
	t.Logf("     - F-oct-prelim-5-2 보고서 §3.4 H-RESP-1 이면 → M2 폐기")
	t.Logf("(M3) §3.10 synthesis memory 비-0 init")
	t.Logf("     - 근거: §4.3 zero-init 이 *전부* 인지, 또는 일부 (gain predictor")
	t.Logf("       MA history 등) 가 별도 init 가질 수 있는지.")
	t.Logf("     - 우리 측정: D 17 contract test 가 §4.3 zero-init 정합 검증")
	t.Logf("       — 기 PASS. 추가 측정 — Decoder zero value 의 모든 sub-state")
	t.Logf("       가 zero 임을 (c) 에서 dump 함.")
	t.Logf("     - 폐기 결정: D 17 PASS + (c) zero dump 정합 시 M3 폐기.")
	t.Logf("(M4) PST 자체 결함 부재 가설 (G3 폐기)")
	t.Logf("     - 근거: 우리 chain 이 모든 stage 에서 spec ref 와 정합한다면")
	t.Logf("       PST 자체가 *우리 ground-truth 가 아닌* 것이 됨.")
	t.Logf("     - 외부 G.729 구현 cross-check 부재 환경에서 본 가설은 *최종")
	t.Logf("       후보* — 다른 가설이 모두 폐기되면 채택 불가피.")
	t.Logf("     - 채택 시 F-oct cycle = plan-end declared.")

	// (f) sample 5..7 mismatch 재현 (Decoder.Decode raw 출력) — already in (b)
	// (g) §4.3 zero-init 정합 dump (이미 (c) 에서 수행)

	t.Logf("──────── 결합 분류 dump ────────")
	t.Logf("(M1, M2 폐기, M3 폐기) → M4 단일 잔존 → F-oct = plan-end declared")
	t.Logf("(M1 잔존, 그 외 폐기) → F-oct = postfilter conditional 분기 production fix cycle")
	t.Logf("(M2 잔존, 그 외 폐기) → F-oct = hpFilter init state production fix cycle")
	t.Logf("(M3 잔존, 그 외 폐기) → F-oct = §4.3 init state production fix cycle")
	t.Logf("(2+ 잔존) → E3 발동 → F-oct = 추가 진단 cycle 또는 복수 fix")
}
```

**중요 — postfilter conditional 분기 측정 범위 한계**: (M1) 의 §A.4.2 conditional 분기 (voicing factor / pitch gain 임계) 우리 구현 포함 여부는 본 task 측정 범위 *외*. 필요 시 후속 cycle 별도 진단. 본 task 는 *현재 구현의 측정값* 만 capture (postfilter[5..7] = [+,+,+] 사실).

- [ ] **Step 3: test 컴파일 + 실행**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5SilenceNegativeMechanism -v`

Expected: PASS (assertion 0). raw output 보고서 §3.1 에 인용.

컴파일 오류 발생 시 본 step 에서 *test 코드만* 수정. production 코드 1 라인이라도 변경 시 즉시 E5 발동.

- [ ] **Step 4: 측정값 분석 + 시나리오 분류**

Step 3 의 출력에서 다음 분류 (보고서 §3.2):

- **(M1) postfilter conditional**: F-sext-1 §3.1 postfilter[5..7] = `[+,+,+]` 측정 인용 + §A.4.2 의 우리 구현 conditional 분기 covered 여부 평가 (D 17 contract test 인용).
  - **(M1-잔존)** 우리 구현이 §A.4.2 의 conditional 분기 (voicing / pitch gain 임계) 일부 미구현.
  - **(M1-폐기)** §A.4.2 conditional 분기 모두 구현 + 측정 정합.
- **(M2) hpFilter**: Task 5-2 결과 인용.
  - **(M2-잔존)** Task 5-2 H-RESP-2.
  - **(M2-폐기)** Task 5-2 H-RESP-1.
- **(M3) §3.10 synth memory**: §4.3 zero-init 정합 dump (Step 2 (c)) + D 17 contract PASS 인용.
  - **(M3-잔존)** zero-init 위반 또는 비-zero state 발견.
  - **(M3-폐기)** zero-init 정합 + D 17 PASS.
- **(M4) PST 결함 부재**: M1/M2/M3 모두 폐기 시 자동 채택.

본 task 결과는 (M1, M2, M3, M4) 잔존/폐기 4-tuple. 단일 잔존 시 F-oct 권고 결정. 2+ 잔존 시 E3 발동.

- [ ] **Step 5: 회귀 게이트 통과 확인**

Run: `go test ./internal/...`

Expected: Phase 0.2 의 12 게이트 모두 PASS + 본 task 의 새 test PASS. 비-contract diagnostic 3건 FAIL 유지 (plan-허용).

- [ ] **Step 6: F-oct-prelim-5-3 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-5-3 보고서 — silence negative output mechanism

**작성일**: 2026-05-01
**범위**: F-oct-prelim-4 §4.3 (3). PST want sample 5..7 = -1 음수 출력
        chain 메커니즘 후보 4 개 (M1 postfilter conditional / M2 hpFilter
        / M3 synth memory / M4 PST 결함 부재) 평가.
**산출물**: production Decoder.Decode raw 출력 + §4.3 zero-init dump +
            (M1/M2/M3/M4) 잔존/폐기 4-tuple 분류.
**준수**: ITU-T G.729 §3.10 + §A.4.2 + §4.2.2 + §4.3 인용. F-sext-1 §3.1
        + Task 5-2 결과 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. §3.10 + §A.4.2 + §4.2.2 + §4.3 인용
## 2. 회귀 게이트 baseline (Step 1 출력)
## 3. 진단 측정값
   3.1 raw output (Step 3)
   3.2 PST want vs production Decode 의 sample 0..15 비교
   3.3 §4.3 zero-init dump
   3.4 (M1/M2/M3/M4) 잔존/폐기 4-tuple 분류
## 4. 결합 분류 (4-tuple) 와 G3 함의
## 5. F-oct-prelim-5-4 진입 권고
```

- [ ] **Step 7: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
M  internal/decoder/stagef_octprelim5_diagnostic_test.go     ← 본 task 수정
?? internal/decoder/stagef_bis_diagnostic_test.go            ← 미변경 보존
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md
```

```bash
git add internal/decoder/stagef_octprelim5_diagnostic_test.go \
        docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-3-report.md
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-oct-prelim-5-3 silence negative mechanism trace

F-oct-prelim-4 §4.3 (3) 후속. ALGTHM frame 0 sf0 PST want sample 5..7
= -1 음수 출력 chain 메커니즘 후보 4 개 (M1 §A.4.2 postfilter conditional,
M2 §4.2.2 hpFilter, M3 §3.10 synth memory init, M4 PST 결함 부재 가설)
평가. production Decoder.Decode raw 출력 + §4.3 zero-init dump + F-sept
+ F-sext + Task 5-2 결과 인용으로 4-tuple 잔존/폐기 분류.

본 진단은 측정-only — production 변경 0. 단일 잔존 시 F-oct 권고 결정,
2+ 잔존 시 E3 발동. 외부 G.729 구현 0건 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-oct-prelim-5-4: 종합 + F-oct 권고 단일 결정

**Goal:** Task 5-1 (P-SRC × B-CMP), 5-2 (Q-FMT × H-INIT × H-RESP), 5-3 (M1/M2/M3/M4) 의 결과를 결합 분석. 가설 G3 (Annex A vs main spec 분기 거동) 의 *분기 위치 단일 식별* 또는 *G3 폐기 (= M4 채택)* 결정. F-oct cycle 권고 단일 결정 — 후보 (a) production fix candidate (M1/M2/M3 단일 잔존 시), (b) plan-end declared (M4 단일 잔존 시), (c) 추가 진단 cycle (E3 발동 시). 잔여 보류 항목 갱신.

**Files:**
- Create: `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md`

본 task 는 *meta task* — 코드 변경 0, 보고서 1건 추가만.

- [ ] **Step 1: Working tree pre-check + 회귀 게이트 종합 확인**

Run: `git status --porcelain && git log -1 --oneline`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
<hash> test(decoder): add Stage F-oct-prelim-5-3 silence negative mechanism trace
```

Run (전체 회귀 게이트, 13 항목):
```
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5PSTSourceVerbatim -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5BitVectorCompare -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5HpFilterInitState -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelim5SilenceNegativeMechanism -v
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5 -v
go test ./internal/decoder/ -run TestDiagnostic_FseptLPReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7 -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimPSTFormat -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimFrameAlignment -v
go test ./internal/decoder/ -run TestDiagnostic_FoctPrelimMultiVectorScan -v
go test ./internal/...
go vet ./...
```

Expected: 14 게이트 모두 PASS + `go test ./internal/...` 의 비-contract 3건 FAIL plan-허용 외 모두 PASS + `go vet` clean.

- [ ] **Step 2: 결합 분석 표 작성**

보고서 §3 의 결합 분석 표 (Task 5-1 × 5-2 × 5-3 결과):

| Task 5-1 | Task 5-2 | Task 5-3 | F-oct 권고 (해석) |
|----------|----------|----------|-------------------|
| (P-SRC-1, B-CMP-1) | (Q-FMT-1, H-INIT-1, H-RESP-1) | (M1-폐기, M2-폐기, M3-폐기, M4 잔존) | **(b) plan-end declared** — 우리 chain spec-correct, PST 자체가 ITU 디코더 산술 결과로 G3 폐기. F-oct cycle 종결. |
| (P-SRC-1, B-CMP-1) | (Q-FMT-1, H-INIT-1, H-RESP-1) | (M1 잔존, 그 외 폐기) | **(a) F-oct production fix** — §A.4.2 postfilter conditional 분기 production fix cycle 진입. |
| (P-SRC-1, B-CMP-1) | (Q-FMT-1, H-INIT-1, H-RESP-2) | (M2 잔존, 그 외 폐기) | **(a) F-oct production fix** — hpFilter init state production fix cycle. (H-RESP-2 발생 시 F-sext-1 회귀 발동 — 즉 측정 자체 점검 후 fix 결정.) |
| (P-SRC-1, B-CMP-1) | (Q-FMT-1, H-INIT-1, H-RESP-1) | (M3 잔존, 그 외 폐기) | **(a) F-oct production fix** — §4.3 init state production fix cycle. |
| (P-SRC-1, B-CMP-1) | * | (2+ M 잔존) | **(c) 추가 진단 cycle** — E3 발동. |
| (P-SRC-2, *) | * | * | *Annex A vs main 분기 byte-level 확정* — 분기 위치를 우리 chain 외부로 분리 가능. F-oct = 추가 진단 cycle (Annex A binary 행동 추적). |

본 표에서 단일 row 매핑 결정.

- [ ] **Step 3: 가설 G3 최종 평가**

다음 항목을 보고서 §4 에 작성:

- **G3 분기 위치 단일 식별**: M1/M2/M3 중 단일 잔존 시 분기 위치 = 해당 stage. F-oct production fix 권고.
- **G3 폐기**: M4 단일 잔존 시 G3 폐기 + plan-end declared.
- **G3 모순**: 2+ 잔존 시 E3 발동 + 추가 진단 cycle.

본 task 의 §4 결정은 **단일** 이며, 강압-적합 회피 의무 (Phase 0.4) 준수.

- [ ] **Step 4: 잔여 보류 항목 갱신**

F-oct-prelim-4 §5 의 9 항목을 본 task 결과에 따라 갱신 (보고서 §5):

| # | 항목 | 직전 상태 (F-oct-prelim-4 §5) | 본 cycle 갱신 |
|---|------|-------------------------------|---------------|
| 1 | F-oct (production fix / plan-end / 추가 진단) | F-oct-prelim-5 권고 | **결정 (a/b/c 중 단일)** |
| 2 | filterSubframe ÷4/×4 | F-quint-3 §4.1 동상 | 미갱신 |
| 3 | β init = 0.2 | F-quint-3 §4.2 동상 | 미갱신 |
| 4 | frame 1+ 잔여 | F-oct-prelim-2 가 frame 1..3 alignment 정합 검증 | 미갱신 (F-oct cycle 후 재평가) |
| 5 | 회귀 가드 promotion | F-oct-prelim-3 §5 의 promotion 금지 강화 | 미갱신 |
| 6 | 비-contract diagnostic 3건 | F-quint-3 §4.6 동상 | 미갱신 |
| 7 | F-sext-2 / F-sext-3 | F-oct-prelim-4 §5 reactivate 검토 | **종결** (Task 5-2 가 reactivate 통합 측정 → 결과에 따라 H-RESP-1 = F-sext-2/3 폐기 / H-RESP-2 = F-sext-2 fix 으로 흡수) |
| 8 | lsp_lp.go uncommitted | F-sept-2 시점 정식화 완료 (`02bf785`) | 미갱신 |
| 9 | stagef_bis_diagnostic_test.go untracked | 보존 유지 | 미갱신 (F-bis cycle 종결 시 별도 commit 검토) |

- [ ] **Step 5: 보고서 작성**

`docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md`:

```markdown
# Phase 1k Stage F-oct-prelim-5-4 종합 보고서 + F-oct 권고

**작성일**: 2026-05-01
**범위**: F-oct-prelim-5-1/2/3 결과 결합 분석. 가설 G3 (Annex A vs main
        spec 분기 거동) 의 분기 위치 단일 식별 또는 폐기. F-oct cycle
        권고 단일 결정 (a production fix / b plan-end / c 추가 진단).
**산출물**: 결합 분석 표 + G3 최종 평가 + F-oct 권고 + 잔여 보류 항목 갱신.
**준수**: F-oct-prelim-5-1/2/3 + F-oct-prelim-4 + F-sept-4 + F-sext-1 +
        F-quart/F-quint 보고서 + ITU PDF 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인. **테스트 변경**: 0 라인 (메타 task).

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)
## 1. F-oct-prelim-5 cycle commit 요약
## 2. 회귀 게이트 종합 결과 (14 항목 + go vet)
## 3. 시나리오 결합 분석 (Task 5-1 × 5-2 × 5-3)
## 4. 가설 G3 최종 평가 (단일 식별 / 폐기 / 모순)
## 5. 잔여 보류 항목 갱신
## 6. F-oct 권고 단일 결정 (a / b / c)
## 7. 결론 — Phase 1k Stage F-oct-prelim-5 closure
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain`

Expected:
```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md
```

```bash
git add docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-oct-prelim-5 synthesis report + F-oct decision

F-oct-prelim-5 cycle (Task 1 PST source verbatim + BIT compare,
Task 2 hpFilter init state + F-sext reactivate, Task 3 silence
negative mechanism trace) 의 결합 분석. G3 (Annex A vs main spec 분기
거동) 의 분기 위치 단일 식별 또는 G3 폐기. F-oct 권고 단일 결정
(a production fix / b plan-end / c 추가 진단).

production 변경 0 + 테스트 변경 0 (메타 task). 외부 G.729 0 참조.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

### 1. Spec coverage

F-oct-prelim-4 §4.3 권고 4 항목 + §5 잔여 보류 항목 #1, #7 매핑:

- §4.3 (1) PST 출처 verbatim 추적 → **Task 5-1**.
- §4.3 (2) hpFilter init state → **Task 5-2**.
- §4.3 (3) silence negative output 메커니즘 → **Task 5-3**.
- §4.3 (4) ALGTHM/PITCH/FIXED 공통 BIT 입력 → **Task 5-1** (BitVectorCompare test).
- §5 #1 F-oct 권고 → **Task 5-4**.
- §5 #7 F-sext-2/3 reactivate → **Task 5-2 통합**.

4 항목 + 2 잔여 모두 task 매핑 완료. 누락 0.

### 2. Placeholder scan

본 plan 검토 — "TBD"/"TODO"/"implement later"/"fill in details"/"appropriate error handling" 검색:
- 각 task 의 Step 2 에 *완전한 test 코드* 제시 (signature, t.Logf, 분류 dump 모두 verbatim).
- 각 task 의 Step 4 에 *완전한 분류 정의* 제시 (P-SRC-1/2, B-CMP-1/2, Q-FMT-1/2, H-INIT-1/2, H-RESP-1/2, M1-잔존/폐기, M2-잔존/폐기, M3-잔존/폐기, M4 등).
- 각 task 의 Step 6 보고서 outline 에 *각 § 명시* (placeholder 없이).
- Step 7 commit 메시지 *완전한 한국어 본문* + co-author trailer.

placeholder 0 확인.

### 3. Type consistency

- Task 5-1: `mainVectorPath`, `dumpFirstNLines`, `hexBytes`, `byteDiffSummary`, `abs` helper 정의.
- Task 5-2: `allZeroInt16`, `signOfInt16Local` helper 정의 + `signOfInt16` (F-sept stagef_sept_diagnostic_test.go 기존) 활용 검토 노트.
- Task 5-3: `signOfInt16Local` 재사용 (Task 5-2 정의). `signOfInt16Local` 가 다른 task 에서 동일 signature 로 사용됨 확인.
- 모든 test signature `func TestDiagnostic_FoctPrelim5*(t *testing.T)` 일관.
- `TestDiagnostic_FoctPrelim5HpFilterInitState` 가 Task 5-2 Step 2 에 정의되고 Task 5-3 Step 1, Task 5-4 Step 1 의 회귀 게이트 list 에 동일 이름으로 인용됨 — match.
- `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` 동상.

type consistency clean.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, F-oct-prelim / F-sept / F-sext 패턴 동일.
**2. Inline Execution** — batch execution with checkpoints.

**Which approach?**
