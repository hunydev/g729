# Phase 1k Stage F-quart 진단-only 사이클 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-tris-2 가 식별한 *확정 spec-위반 후보* (gain.decodeVQ 의 GainImap1/GainImap2 inverse-map 누락) 가 ALGTHM frame 0 sample 0..7 PST/2 정렬에 *충분조건*인지 *부분 조건*인지를 production 코드 0-수정으로 측정·확정한다. 동시에 §3.7 / §3.8 / §3.9 비선형 체인 잔여 결함을 line-by-line spec 인용으로 검증해, 단일 fix vs 다중 fix 결정에 필요한 정보를 일괄 확보한다.

**Architecture:** 4 task 모두 *진단-only* (production 코드 수정 0). Task F-quart-1·F-quart-3 은 신규 진단 test 가 production 함수를 *호출하면서도* GainImap 적용/미적용 두 분기 결과를 동시 비교하기 위해 *test 코드 자체에서 inverse-map 적용한 평행 fork* 를 정의한다. Task F-quart-2 는 spec § 인용 + line-by-line 코드 대조 보고만 (코드/테스트 변경 0). Task F-quart-4 는 1·2·3 결과를 통합해 단일 fix 후보 또는 다중 fix 후보 ranking 을 산출한 *F-quint 권고서*를 작성한다.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) + 기존 stagef diagnostic 하니스 패턴. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — Working tree 사전점검 + 사이클 입구 invariant

### Phase 0.1 Working tree 상태 명시

F-quart 진입 시점의 working tree 는 F-tris-2 종료 직후 상태:

| 경로 | 상태 | F-quart 변경 허가? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 | **No** (보존) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) — F-bis-1/F-tris-1 진단 하니스 | **No** (보존) |

F-quart 신규 파일 (허가):
- `internal/decoder/stagef_quart_diagnostic_test.go` (Task F-quart-1·F-quart-3 측정 하니스, *new uncommitted* 또는 commit 가능)
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md` (Task F-quart-1 보고서, *staged → committed*)
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md` (Task F-quart-2 보고서, *staged → committed*)
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-3-report.md` (Task F-quart-3 보고서, *staged → committed*)
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-4-report.md` (Task F-quart-4 종합 보고서 + F-quint 권고, *staged → committed*)

Production 코드 (`internal/lsp/`·`internal/synth/`·`internal/postfilter/`·`internal/pcm/`·`internal/gain/`·`internal/fcb/`·`internal/pitch/`·`internal/decoder/decode.go`·`internal/decoder/subframe.go` 등) 의 **수정 절대 금지**. 각 task 시작·종료 시 `git diff --stat -- internal/` 실행해 production 변경 0 라인 검증.

### Phase 0.2 Escape hatch (E1·E2·E3·E4) 사전합의

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | 진단 결과 1·2·3순위 모든 후보가 spec § 인용 line-by-line 일치 (= 결함 위치 0건 식별) | 즉시 모든 보고서 commit + Stage F-quart **부정** + Stage F-quint plan 권고에 "외부 정합 모델 (frame 1+ 의존, codec state-machine 외부 결함, ALGTHM 자체 오류 가능성) 진단 cycle" 명시 |
| **E2** | 단일 fix (GainImap inverse-map 적용) 진단 결과가 sample 0..7 의 |Δ|≤1 LSB 정렬을 *전부* 회복 | F-quint plan 권고에 "단일-commit fix 직진 가능" 명시. 그러나 *F-quart-3 비선형 체인 잔여 spec-위반*이 1건 이상이면 *복합 fix*로 ranking. |
| **E3** | F-quart-1 두 분기 비교 결과가 *기대 방향과 정반대* (= GainImap inverse-map 적용 후 정렬도가 *악화*) | 즉시 F-quart-1 보고서에 결과 명기 + F-quart-2 / F-quart-3 진단 *계속 진행* (단일 fix 부정 ≠ 다중 fix 부정) + 종합 보고서 §의 ranking 갱신 |
| **E4** | 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro, FFmpeg) 1건이라도 의도치 않게 인용/대조에 사용된 흔적 발견 | 즉시 모든 작업 중단. 사용자에게 통보. 해당 인용 제거 후 재시작. |

각 보고서는 §0 에 *해치 평가표* 포함 의무.

### Phase 0.3 진단 데이터 baseline 인용

F-tris-1 보고 §0.2 의 stage-by-stage 출력 (sample 0..7) 은 본 사이클의 **불변 baseline**:

```
u[0..7] = [2 2 2 2 0 0 0 0]
a[] (Q12) = [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]

PST want sf0      [ 0.. 7]     2     4     3     3     1    -1    -1    -1
PST/2 spec-target [ 0.. 7]     1     2     1     1     0    -1    -1    -1
synth.Filter      [ 0.. 7]     2     3     4     4     3     2     1     1
postfilter.Filter [ 0.. 7]     2     2     3     4     3     2     1     2
hpFilter          [ 0.. 7]     2     2     3     3     2     1     0     1

Match count vs PST/2 (|Δ|≤1 LSB):
  synth.Filter:      33 / 40
  postfilter.Filter: 32 / 40
  hpFilter:          34 / 40
```

또한 F-tris-2 보고 §4.4 의 두 분기 g_p / γ̂_c 표:

| 인덱싱 정책 | g_p (Q14) | γ̂_c (Q13) |
|-------------|----------:|----------:|
| Production (`GBK[bits]`) | 13815 (0.843 real) | 12915 (1.577 real) |
| Spec §3.9.3 (`GBK[Imap[bits]]`) | 1995 (0.122 real) | 1516 (0.185 real) |

F-quart-1 진단 출력은 위 두 표 모두와 *동시에 비교* 가능해야 한다.

---

## Task F-quart-1: GainImap inverse-map 단일-fix 정렬도 측정

**Goal:** *Production 코드 수정 0 라인*으로 GainImap inverse-map 을 *test 코드에서 평행 fork* 적용해 두 분기 (production / spec-fix) 의 sample 0..7 PST/2 정렬도를 직접 비교한다. 단일 fix 가 sample 0..7 결함의 충분조건인지 측정해 §0.2 의 E2/E3 해치 평가에 필요한 데이터를 산출한다.

**Files:**
- Create: `internal/decoder/stagef_quart_diagnostic_test.go` (신규 진단 test, working tree 미커밋 또는 별도 commit 모두 허용)
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md` (보고서, staged → committed)
- **Modify: 없음** (production 코드 0-수정)

**전체 프로토콜 — Production 호출 없이 두 분기 평행 시뮬레이션**:

`gain.Decoder.Decode` 는 unexported 필드 (`pastErrors`, `initialized`, `prevGpQ14`) 를 갖는다. Test 가 이를 *교체-인덱스 인자로 호출*하면 production 코드 수정 0 으로 spec-fix 분기를 시뮬레이션 가능. 핵심: **test 가 받는 `idx` 의 GA/GB 필드를 *test 코드에서* GainImap1/GainImap2 로 변환한 새 Indices 를 만들어 `gain.Decoder.Decode` 에 전달**한다. Production `decodeVQ` 는 받은 GA/GB 로 직접 GBK 인덱싱 → spec-fix 분기는 test 가 미리 inverse map 한 GA'/GB' 을 넘기므로 production 입장에선 같은 코드 경로지만 *결과적으로* `GBK[Imap[bits]]` 와 동일.

이 방식은:
- Production `decodeVQ` 코드 수정 0.
- `Decoder.Decode` 호출 1회 = 1 회 `decodeVQ` 호출 + pastErrors FIFO 1회 update + 비선형 체인 1회 통과. 따라서 두 분기를 비교하려면 *별도 Decoder instance 2개* 필요 (pastErrors 가 분기별로 발산).
- pitch / fcb / synth / postfilter / pcm / hpFilter 도 분기별 별도 instance 필요 (synth.pastSynth, postfilter.agcGainPrev, decoder.pastExc 등 상태 분기).

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected: F-tris-2 종료 시점 그대로 (lsp_lp.go modified + stagef_bis_diagnostic_test.go new + production diff 0 라인). 다른 production 변경이 보이면 즉시 작업 중단 후 사용자 통보.

- [ ] **Step 2: F-quart-1 진단 test 작성 — 두 분기 평행 디코딩**

`internal/decoder/stagef_quart_diagnostic_test.go` 를 다음 골격으로 작성. `bitstream` / `decoder` / `tables` / `gain` 패키지의 *exported* API 만 사용. `tables.GainImap1` / `tables.GainImap2` 는 이미 exported (`gain_gbk1.go:44`, `gain_gbk2.go:38`).

Frame 0 의 GA1=5 / GB1=6 (sf0) 와 GA2 / GB2 (sf1) 는 ALGTHM bit-stream 에서 추출해야 한다. 본 step 의 test 는 frame 0 sf0 sample 0..7 만 비교 대상이므로 *sf0 의 idx 만* inverse map 적용; sf1 은 production 그대로 (또는 inverse map 적용 후 비교 — sf0 영향 없음. 단순화 위해 sf0 만 mapping).

```go
package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/tables"
	"github.com/exedev/g729/internal/decoder/itu" // ALGTHM 로더
)

// remapSf0Gain returns a copy of frame's bit-stream payload with sf0
// gain indices (GA1, GB1) replaced by GainImap1[GA1] / GainImap2[GB1].
// All other bits are preserved verbatim.
func remapSf0Gain(rawBits []byte) []byte { /* ... */ }

func TestDiagnostic_FquartGainImap_Sf0Sample0to7(t *testing.T) {
	// 1. Load ALGTHM frame 0 bitstream (raw 80 bits).
	rawFrame0 := itu.MustLoadAlgthmFrame(t, 0)

	// 2. Branch A — production: decode rawFrame0 verbatim through
	//    decoder.Decoder. Capture sample 0..7 at four boundaries
	//    (synth, postfilter, hpFilter, scaled).
	branchA := decodeWithBoundaryCapture(t, rawFrame0)

	// 3. Branch B — spec-fix: build remapped bits with sf0 GA/GB
	//    inverse-mapped, then decode through a fresh Decoder instance.
	remapped := remapSf0Gain(rawFrame0)
	branchB := decodeWithBoundaryCapture(t, remapped)

	// 4. PST/2 spec target.
	pstHalf := loadPSTHalfSf0Frame0(t)

	// 5. Side-by-side report (t.Logf).
	t.Logf("Boundary           [ 0..  7]  matches/40")
	for _, b := range []boundaryDump{branchA, branchB} {
		for _, stage := range b.stages {
			t.Logf("  %-22s %s  %d/40",
				stage.name, fmtSamples(stage.samples[:8]),
				stage.matchCount(pstHalf))
		}
	}
}
```

`decodeWithBoundaryCapture` 는 4 boundary (synth.Filter / postfilter.Filter / hpFilter / pcm.ScaleUpSat) 출력 [40]int16 와 sample 0..7 슬라이스, 그리고 |Δ|≤1 LSB match count (40 샘플 전체 vs PST/2) 를 반환.

- [ ] **Step 3: `remapSf0Gain` 구현 — bit-stream packing 정확도**

ITU-T G.729 §4 (디코더 입력 비트 순서) 에 따라 sf0 의 gain indices 위치를 식별. `internal/bitstream/decode.go` 의 unpacking 코드를 *읽어서* (수정 X) 비트 위치를 추론. GA1 (3 bit) + GB1 (4 bit) = 7 bit 를 sf0 대상 위치에서 inverse map 후 *재패킹*.

```go
func remapSf0Gain(rawBits []byte) []byte {
	out := make([]byte, len(rawBits))
	copy(out, rawBits)
	// Decode existing GA1, GB1 from out[].
	ga1 := readBits(out, gA1BitOffset, 3)
	gb1 := readBits(out, gB1BitOffset, 4)
	// Apply inverse map per §3.9.3.
	ga1Mapped := uint32(tables.GainImap1[ga1])
	gb1Mapped := uint32(tables.GainImap2[gb1])
	writeBits(out, gA1BitOffset, 3, ga1Mapped)
	writeBits(out, gB1BitOffset, 4, gb1Mapped)
	return out
}
```

`gA1BitOffset` / `gB1BitOffset` 는 `internal/bitstream` 의 frame layout 상수에서 차용 (또는 layout 표를 spec § 4.1 / §4.2 에서 직접 인용해 결정). *Production bitstream 패키지 수정 0*.

- [ ] **Step 4: ALGTHM frame 0 verbatim 디코딩 (branch A) sanity check**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v`
Expected: branch A 출력의 synth.Filter sample 0..7 = `[2 3 4 4 3 2 1 1]` (= F-tris-1 baseline). 일치하지 않으면 즉시 중단 (test infra 결함).

- [ ] **Step 5: branch B 출력 측정 + 비교표 작성**

Test output 의 t.Logf 출력에서 branch B (spec-fix) 의 4 boundary sample 0..7 + 40-sample match count 추출. 다음 비교표 형식으로 `2026-04-28-phase1k-stage-f-quart-1-report.md` 의 본문에 인용:

```
Branch       Boundary           [ 0..  7]                       matches/40
A (prod)     synth.Filter       [ 2  3  4  4  3  2  1  1]       33/40
A (prod)     postfilter.Filter  [ 2  2  3  4  3  2  1  2]       32/40
A (prod)     hpFilter           [ 2  2  3  3  2  1  0  1]       34/40
A (prod)     pcm.ScaleUpSat     [ 4  4  6  6  4  2  0  2]       (PST 도메인)
B (spec)     synth.Filter       [ ?  ?  ?  ?  ?  ?  ?  ?]       ?/40
B (spec)     postfilter.Filter  [ ?  ?  ?  ?  ?  ?  ?  ?]       ?/40
B (spec)     hpFilter           [ ?  ?  ?  ?  ?  ?  ?  ?]       ?/40
B (spec)     pcm.ScaleUpSat     [ ?  ?  ?  ?  ?  ?  ?  ?]       (PST 도메인)
```

`?` 위치를 실측값으로 채운다.

- [ ] **Step 6: 정렬 시나리오 분류**

Branch B 의 hpFilter sample 0..7 을 PST/2 = `[1 2 1 1 0 -1 -1 -1]` 와 *직접 비교* 후 다음 셋 중 하나로 분류:

- **시나리오 (S1) 충분조건**: branch B 의 hpFilter sample 0..7 *전부* 가 PST/2 와 |Δ|≤1 LSB. 또한 40-sample match count = 40/40. → E2 *부분-발동* (단일 fix 시 sample 0..7 회복). F-quart-2 / F-quart-3 진행해 잔여 spec-위반 검출.
- **시나리오 (S2) 부분조건**: branch B sample 0..7 일부만 PST/2 일치 (예: sample 0 일치, sample 1 불일치). → E2/E3 미발동 (단일 fix 부분 회복). F-quart-2/F-quart-3 필수 — 다중 fix 후보 존재.
- **시나리오 (S3) 악화**: branch B 의 sample 0..7 정렬도가 branch A 보다 *나쁨*. → E3 발동. 단, F-quart-2/F-quart-3 *계속 진행* (단일 fix 부정 ≠ 다중 fix 부정).

각 시나리오에서 보고서 §의 권고 문구가 다르므로, Step 5 측정값 확인 직후 시나리오 분류 명시.

- [ ] **Step 7: §3.9.3 spec 인용 + docstring spec-위반 명시**

보고서 §1 (spec 인용) 에 PDF p.22 §3.9.3 verbatim 인용:

> "To reduce the impact of single bit errors, the GA and GB indices are reordered before transmission. The mapping tables are given in Annex C/D."

또한 §3.9 의 인코더 측 코드북 검색 절차 (PDF p.21-22) 인용 — 인코더가 reorder 적용 시 디코더의 inverse map 의무.

추가로 production `internal/gain/vq.go:14-17` 의 docstring:

> "The codebooks are indexed directly by the received bits (GA, GB); the optional reorder tables (Map/Imap) live in tables for the encoder search routine and play no role at the decoder."

이 문장이 §3.9.3 와 *모순*임을 명시 (= docstring 자체에 spec-위반 정당화가 박혀있음을 보고서에 기록). docstring 자체도 F-quint 사이클에서 수정 대상.

- [ ] **Step 8: 진단 test 실행 결과 수집 + 보고서 작성**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md` 를 다음 골격으로 작성:

```markdown
# Phase 1k Stage F-quart-1 진단 노트 — GainImap inverse-map 단일-fix 정렬도 측정

**작성일**: 2026-04-28
**범위**: F-tris-2 §4 발견 (decodeVQ GainImap 누락) 의 단일-fix 적용 시 ALGTHM frame 0 sample 0..7 PST/2 정렬도 측정.
**산출물**: 단일 fix 충분조건 / 부분조건 / 악화 분류 + F-quart-2/F-quart-3 진입 가이드.
**준수**: ITU-T G.729 (06/2012) PDF §3.9 / §3.9.3 / §4.1.6 / §4.2.4 만 인용. 외부 구현 미참조.

## 0. Working tree 상태 + escape hatch 평가
## 1. §3.9.3 spec 인용 + docstring 모순 명시
## 2. F-quart-1 진단 test 설명 (production 코드 0-수정 평행 시뮬레이션)
## 3. Branch A (production) sample 0..7 + 40-sample match count
## 4. Branch B (spec-fix) sample 0..7 + 40-sample match count
## 5. 비교표 + 시나리오 분류 (S1/S2/S3)
## 6. F-quart-2 / F-quart-3 진입 가이드
```

- [ ] **Step 9: Working tree post-check + commit**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected:
- Production 변경 0 라인 확인.
- `internal/decoder/stagef_quart_diagnostic_test.go` *new* 상태 (커밋 선택).
- `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md` *new staged*.

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md
# 진단 test 는 commit 권고 (재현성). 단, working tree 미커밋 유지도 허용.
git add internal/decoder/stagef_quart_diagnostic_test.go
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-quart-1 GainImap diagnostic harness + report

Stage F-quart-1 measures ALGTHM frame 0 sample 0..7 PST/2 alignment
under two parallel decoding branches (production vs §3.9.3 spec-fix
inverse-mapped GA/GB) without modifying production code.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-quart-2: 3순위 spec 인용 — pitch / fcb / pitch enhancement line-by-line

**Goal:** F-tris-2 §5 가 *deferred* 한 3순위 (pitch.AdaptiveCodebook / fcb.Decode / pitch enhancement) 를 §3.7 / §3.7.1 / §3.8 / §A.3.7 / §A.3.8 line-by-line spec 인용 + Q-format 검증으로 진단한다. Production 코드 수정 0, 신규 진단 test 0 (코드 대조 only). frame 0 sf0 의 stimulus 에서 본 단계가 spec-correct 인지를 결정.

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md` (보고서, staged → committed)
- **Modify: 없음**, **신규 test: 없음**

본 task 는 *코드 읽기 + spec 인용 + Q-format 검증표* 만 수행. F-tris-2 §2·§3 가 1·2순위에 대해 수행한 것과 동일 방법론을 3순위에 확장.

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected: F-quart-1 종료 후 상태 (production diff 0 라인 + 진단 하니스 커밋된 또는 미커밋).

- [ ] **Step 2: pitch.AdaptiveCodebook §3.7 / §3.7.1 / §A.3.7 인용**

PDF p.13-15, §3.7 (pitch period decoding):
- 식 (33)·(34): pitch lag 의 정수/분수 부분 분해.
- 식 (40) (PDF p.15, §3.7.1): adaptive codebook FIR interpolation —

> "v(n) = Σ_{i=0..9} u(n − k − i)·b30(t + 3i) + Σ_{i=0..9} u(n − k + 1 + i)·b30(3 − t + 3i)        n = 0,...,39"

§A.3.7 (PDF p.42): G.729A short pitch — "When tInt < 40, the adaptive codebook vector is constructed by replicating the past excitation with period tInt." (정확 verbatim 인용은 PDF 직접 확인 후 보고서에 기록.)

- [ ] **Step 3: pitch.AdaptiveCodebook line-by-line 검증**

`internal/pitch/adaptive.go:39-67` (이 plan §0.3 와 별도로 보고서 본문에 코드 인용) 와 식 (40) + §A.3.7 short-pitch periodicity 의 일치 여부:

| 분기 | 조건 | spec 출처 | production 코드 (line) | 일치 |
|------|------|-----------|---------------------|---|
| Fast path | tFrac=0 ∧ tInt≥40 | §3.7 정수 lag 단순 복사 | `adaptive.go:40-46` | ? |
| FIR interpolation | tFrac≠0 ∧ tInt≥40 | 식 (40) | `adaptive.go:48-50` + `firInterpolate` (`adaptive.go:72-106`) | ? |
| Short pitch tFrac=0 | tFrac=0 ∧ tInt<40 | §A.3.7 short-pitch periodicity | `adaptive.go:56-60` + `adaptive.go:64-66` | ? |
| Short pitch tFrac≠0 | tFrac≠0 ∧ tInt<40 | §A.3.7 + 식 (40) interpolation | `adaptive.go:62-63` + `adaptive.go:64-66` | ? |

각 row 에서 production 코드 동작이 spec 식·문장과 일치하는지 verbatim 비교. ALGTHM frame 0 sf0 의 (tInt=20, tFrac=0) 는 Short pitch tFrac=0 분기 → 본 분기를 깊이 검증. (다른 분기는 짧은 sanity check.)

- [ ] **Step 4: pitch.AdaptiveCodebook hand-trace (frame 0 sf0)**

frame 0 sf0: tInt=20, tFrac=0, pastExc 전부 0 (decoder 초기화 직후).

Production 분기 진입: `adaptive.go:56-60` (short pitch, tFrac=0 분기) → base = len(pastExc) - 20 → v[0..19] = pastExc[base+0..base+19] = 0. → `adaptive.go:64-66` 의 periodicity extension → v[n] = v[n−20] for n=20..39 → 0.

→ v[0..39] = 0 전체. F-tris-1 출력 `gp·v=0` 정합.

Spec 식 (40) + §A.3.7: 동일 출력 (pastExc=0 → 모든 항 0). **결함 위치 아님 (frame 0 sf0 stimulus 한정)**.

본 §의 결과는 *frame 0 sf0 한정*임을 명시. 다른 frame (특히 frame 1+ 의 voiced sf) 에서는 b30 FIR 계수 (`tables.PitchInterpFIR`) 의 spec 일치 여부가 별도 검증 대상이지만, 본 사이클 범위 외.

- [ ] **Step 5: fcb.Decode §3.8 / §4.1.5 인용**

PDF p.17-19, §3.8 (algebraic codebook) + §4.1.5 (디코더 측 동일):
- §3.8 식 (61) — algebraic codebook 4-pulse 위치 인덱싱.
- §3.8 표 7 — pulse 위치 후보 set (i₀∈{0,5,10,...,35}, i₁∈{1,6,...,36}, 등).
- §3.8 식 (62) — sign decoding.
- §3.8.1 식 (64) — pitch enhancement: c'(n) = c(n) + β·c(n − T) for n ≥ T, where T = subframe pitch lag, β = clamped pitch gain.

PDF §A.3.8: G.729A algebraic codebook — 동일 4-pulse 구조, search 만 단순화 (디코더는 영향 없음).

- [ ] **Step 6: fcb.Decode line-by-line 검증**

`internal/fcb/decode.go:20-24` (Decode entry):

```go
func Decode(idx Indices, t int, betaQ14 int16, c *[40]int16) {
	positions := decodePositions(idx.Positions)
	placePulses(positions, idx.Signs, c)
	applyPitchEnhancement(c, t, betaQ14)
}
```

3 단계가 §3.8 의 (1) 위치 디코딩 → (2) sign 적용 → (3) pitch enhancement 와 정합. 각 helper 의 line-by-line 검증:

| 단계 | spec 출처 | production 파일·함수 | 검증 항목 |
|------|-----------|--------------------|---------|
| 1 | §3.8 식 (61) + 표 7 | `internal/fcb/positions.go::decodePositions` | 13-bit → 4 pulse 위치 매핑 (각 pulse track 이 표 7 의 후보 집합과 일치) |
| 2 | §3.8 식 (62) | `internal/fcb/signs.go::placePulses` | 4-bit signs → ±PulseAmplitude (Q13) 4 pulse 배치 |
| 3 | §3.8.1 식 (64) | `internal/fcb/enhancement.go::applyPitchEnhancement` (또는 동등 파일) | β·c(n−T) loop 의 Q-format (β Q14, c Q13 → β·c Q14+13+1=Q28, shift to Q13) + boundary (n ≥ T 만 적용) |

각 단계가 spec 일치하면 *결함 위치 아님*; 위반 발견 시 해당 line 인용 + spec § 인용 비교.

- [ ] **Step 7: fcb.Decode hand-trace (frame 0 sf0, C1=0, S1=15)**

Frame 0 sf0 의 idx.Positions = C1 = 0 (13-bit 0). idx.Signs = S1 = 15 = 0b1111 (4-bit, 4 pulse 모두 + sign).

§3.8 표 7 + C1=0 → 4 pulse 위치 = (i₀=0, i₁=1, i₂=2, i₃=3) ← *정확값은 `decodePositions` 의 13-bit 분해 추적 후 결정*. 가정: 4 pulse 모두 위치 0..3.

placePulses(positions=[0,1,2,3], signs=15, c) → c[0]=c[1]=c[2]=c[3]=+8192 (Q13 = +1.0), c[4..39]=0.

applyPitchEnhancement(c, t=20, betaQ14): β = ClampPitchGainForEnhancement(d.prevGpQ14=0). prevGpQ14 zero-value 의 clamp 결과는 §3.8.1 의 lower-bound (= 0.2 = 3277 Q14) 가정. n ≥ 20 이면 c[n] += β·c[n−20] → n=20: c[20] += 3277·8192/16384 = 1638 (Q13). 등등. n<20 변경 없음.

→ c[0..3] = +8192, c[4..19] = 0, c[20..23] ≈ +1638. 본 hand-trace 결과를 production diagnostic test 가 c[] 를 dump 하도록 (이미 진단 하니스에 c[] dump 가 있는 경우) 비교. (없으면 본 hand-trace 만 보고서에 기록 + 후속 진단 하니스 권고.)

- [ ] **Step 8: pitch enhancement β clamp §3.8.1 인용**

PDF §3.8.1 (pitch sharpening clamp): β 의 lower / upper bound 값 verbatim 인용. `internal/fcb/enhancement.go::ClampPitchGainForEnhancement` (또는 파일명) 의 clamp 상수와 spec 일치 여부 검증. frame 0 sf0 의 prevGpQ14=0 처리가 spec 의 "the first subframe of the first frame initializes β to ..." 와 일치하는지 확인.

- [ ] **Step 9: 보고서 작성 + commit**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md` 를 다음 골격으로 작성:

```markdown
# Phase 1k Stage F-quart-2 진단 노트 — pitch / fcb / pitch enhancement spec 인용

**작성일**: 2026-04-28
**범위**: F-tris-2 §5 deferred — 3순위 spec 인용 + Q-format line-by-line.
**산출물**: 3순위 후보 spec 일치/위반 분류 + 잔여 spec-위반 후보 ranking 갱신.
**준수**: ITU-T G.729 (06/2012) PDF §3.7 / §3.7.1 / §3.8 / §3.8.1 / §A.3.7 / §A.3.8 만 인용.

## 0. Working tree 상태 + escape hatch 평가
## 1. pitch.AdaptiveCodebook (§3.7 / §A.3.7)
   1.1 spec 인용
   1.2 line-by-line 검증표
   1.3 frame 0 sf0 hand-trace (tInt=20, tFrac=0, pastExc=0)
   1.4 결론
## 2. fcb.Decode (§3.8 / §4.1.5)
   2.1 spec 인용
   2.2 line-by-line 검증표 (decodePositions / placePulses / applyPitchEnhancement)
   2.3 frame 0 sf0 hand-trace (C1=0, S1=15)
   2.4 결론
## 3. pitch enhancement β clamp (§3.8.1)
   3.1 spec 인용
   3.2 ClampPitchGainForEnhancement 값 검증
   3.3 frame 0 sf0 prevGpQ14=0 처리
## 4. 종합 — 3순위 spec 일치/위반 분류
```

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-quart-2 pitch/fcb/pitch-enhancement spec note

Stage F-quart-2 verifies §3.7 / §3.7.1 / §3.8 / §3.8.1 / §A.3.7 / §A.3.8
line-by-line against pitch.AdaptiveCodebook, fcb.Decode, and pitch
enhancement, completing the 3순위 deferral from F-tris-2.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-quart-3: gain.Decode 비선형 체인 검증

**Goal:** `gain.Decode` 의 비선형 체인 (predictedLogGain → log2Fixed(ecEnergy) → ecBarDbQ10 → log2GcQ10 → pow2Fixed → gc0Q14 → gcQ12 → MA predictor FIFO update) 을 §3.9 / §4.1.6 line-by-line 인용 + Q-format 검증 + frame 0 sf0 hand-trace 로 검증한다. F-tris-2 §6.5 의 "비선형 체인 의존으로 hand-calc 정렬 예측 불가" 가 *체인 자체의 spec-위반* 인지 *정상 체인* 인지를 결정.

**Files:**
- Modify: `internal/decoder/stagef_quart_diagnostic_test.go` (F-quart-1 의 진단 하니스에 gain.Decode 중간값 dump 추가, *production 코드 0-수정*)
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-3-report.md` (보고서)

`gain.Decoder` 의 unexported 필드 dump 가 필요할 수 있다. 그 경우 *test-internal accessor* 로 처리 (test 가 `internal/gain` 패키지의 *_test.go 가 아니므로 unexported 접근 불가*). 대안: `internal/gain` 에 *test-only export* 함수 추가 (예: `internal/gain/export_test.go` — production binary 에 영향 0). 그러나 본 사이클은 production 패키지 변경 0 이 원칙 → **test-only export 도 production 패키지에 추가 금지**. 대신 *gain.Decoder 의 input/output (idx, c, gpQ14, gcQ12) 만* 측정. 중간값은 spec 식에 따라 *test 코드에서 재계산* (production decoder 와 별개 mock 으로).

**검증 전략 변경**: production gain.Decoder 의 중간값을 직접 dump 하는 대신,
- (i) test 가 spec § 식을 *test 코드에서 처음부터 재구현* (= reference impl) → 동일 입력 (idx, c) 에 대한 reference impl 출력 (gpQ14, gcQ12) 을 production gain.Decode 출력과 *비교*.
- (ii) reference impl 은 §3.9 / §4.1.6 식에 *직접* 따라 작성. 외부 구현 0건 참조.
- (iii) 두 출력 일치 → production gain.Decode 의 비선형 체인은 *입출력 동치 spec-correct*. 불일치 → spec-위반 위치를 reference impl 의 단계별 dump 와 production input/output 비교로 좁힘.

이 전략은 F-tris-2 §2·§3 의 line-by-line + hand-trace 방법론과 동일한 effective coverage 를 제공하면서 production 패키지에 export 추가 0을 보장.

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected: F-quart-2 종료 후 상태 (보고서 commit + production diff 0 라인).

- [ ] **Step 2: §3.9 / §4.1.6 비선형 체인 spec 인용**

PDF §3.9 (gain quantization, p.20-22) + §4.1.6 (디코더 측 동일):
- 식 (66) — predicted log-gain Ê(m) = Σ_{i=0..3} b_i · Û(m − i − 1), b = MA predictor 계수.
- 식 (68) — E̅_c = 10·log10(E_c / 40) dB, E_c = Σ_{n=0..39} c²(n).
- 식 (69) — log gain 변환식.
- 식 (70) — g_c0 = 2^((Ê + E̅_c) / 20).
- 식 (71) — g_c = γ̂_c · g_c0.
- 식 (74) — U(m) = 20·log10(γ̂_c) (MA predictor FIFO 입력).

§A.3.9: G.729A inheritance — 식 동일.

각 식을 보고서 §1 에 verbatim 인용 (PDF 직접 인용).

- [ ] **Step 3: pastErrors default value §3.9 인용 + production 비교**

§3.9: "The initial value of each entry in the MA predictor's tap line is set to −14 dB Q10 = −14336" (verbatim 확인 필요). `internal/gain/decode.go:9` `pastErrorsDefault int16 = -14336` 이 spec 일치 여부 검증.

또한 `internal/gain/decode.go:38-44` 의 lazy init (`if !d.initialized { ... }`) 패턴이 spec 의 "각 frame 의 첫 sf0 진입 시 초기화" 와 일치하는지 검증.

- [ ] **Step 4: log2Fixed / pow2Fixed §-인용 검증**

PDF §3.9 의 log2/pow2 fixed-point 함수 인용. ITU 사양은 통상 표 lookup + interpolation (§3.9 표 8 / 9 / 10 — `tables.Pow2Table` / `tables.Log2Table` 등). `internal/gain/decode.go::log2Fixed` / `pow2Fixed` 구현 (별도 helper 파일 위치 확인) line-by-line 비교.

각 helper 의:
- 입력 Q-format
- 표 lookup index 추출 방식
- interpolation 수식
- 출력 Q-format
모두 spec § 인용과 일치하는지 검증표.

- [ ] **Step 5: dB conversion 상수 검증**

`internal/gain/decode.go:18-23`:
- `dbPerLog2Q13 = 24660` ≈ 10·log10(2)·2¹³
- `tenLog10_40Q10 = 16405` ≈ 10·log10(40)·2¹⁰
- `invDbScaleQ15 = 5443` ≈ 1/(20·log10(2))·2¹⁵
- `dbPerLog2Q10 = 6165` ≈ 20·log10(2)·2¹⁰

각 상수의 *수학적 진짜 값*을 13/14자리 정밀도로 계산해 비교:
- 10·log10(2) = 3.010299957... → ×2¹³ = 24660.36 → 정수 round = 24660 ✓ (또는 24661 — 1-LSB diff 가능)
- 10·log10(40) = 16.020599913... → ×2¹⁰ = 16405.09 → 16405 ✓
- 1/(20·log10(2)) = 0.166096... → ×2¹⁵ = 5443.4 → 5443 ✓
- 20·log10(2) = 6.020599913... → ×2¹⁰ = 6165.09 → 6165 ✓

각 상수의 정수 round 가 *최단거리* 인지 확인. 1-LSB 차이 발견 시 보고서에 명시 (spec 위반 vs 단순 round 선택 차이는 §3.9 의 정수 표 부재 시 모호 — 본 case 는 외부 구현 미참조이므로 *수학적 진짜 값과 가장 가까운 정수* 가 spec-correct 라고 판정).

- [ ] **Step 6: zero-energy guard §-인용 검증**

`internal/gain/decode.go:46-65` 의 `if ecEnergy <= 0 { ... }` 분기:
- `gpQ14` 만 decodeVQ 결과로 설정.
- `gcQ12 = 0`.
- pastErrors FIFO 에 *long-term default* (`pastErrorsDefault = -14336`) 삽입.

§3.9 / §A.3.9 가 이 guard 를 명시하는지 확인 (verbatim 인용 검색). 명시 없으면 *production 자체 추가 안전망* 으로 분류 → frame 0 sf0 의 c[] 가 0 이 아니므로 본 분기 미진입 (= 본 stimulus 영향 없음).

- [ ] **Step 7: Test reference impl 작성**

`internal/decoder/stagef_quart_diagnostic_test.go` 에 다음 함수 추가:

```go
// referenceGainDecode is a hand-coded reimplementation of gain.Decoder.Decode
// directly from ITU-T G.729 (06/2012) §3.9 / §4.1.6 equations (66)/(68)-(71)/(74),
// without any reference to existing G.729 implementations. Used to cross-check
// the production gain.Decoder.Decode output for ALGTHM frame 0 sf0/sf1.
type referenceGainState struct {
	pastErrors  [4]int16
	initialized bool
	prevGpQ14   int16 // not used in this reference; mirrors production for parity
}

func (r *referenceGainState) decode(idxGA, idxGB uint8, c *[40]int16) (gpQ14, gcQ12 int16) {
	// 1. Decode quantized gains by §3.9 conjugate-structure VQ.
	// 2. Compute E_c = Σ c²(n) and E̅_c = 10·log10(E_c/40) Q10 dB.
	// 3. Predict Ê(m) = Σ b_i · pastErrors[i] per §3.9 eq (66) MA predictor.
	// 4. Compute g_c0 = 2^((Ê + E̅_c)/20) per §3.9 eq (70).
	// 5. Compute g_c = γ̂_c · g_c0 per §3.9 eq (71).
	// 6. Update pastErrors FIFO with U(m) = 20·log10(γ̂_c) per §3.9 eq (74).
	// All steps use only ITU-T G.729 (06/2012) PDF §3.9 / §4.1.6 equations.
	// ... actual implementation per spec ...
}
```

본 reference impl 은 spec § 인용에서 직접 도출 — 외부 구현 0건 참조. MA predictor 계수 b_i 는 §3.9 (PDF p.21) 에서 직접 인용 (보고서에 verbatim 기록).

- [ ] **Step 8: Reference impl vs production gain.Decode 비교**

frame 0 sf0 의 (idx_GA=5, idx_GB=6, c=production fcb output) 으로:
- production: `prodDec := &gain.Decoder{}; gp, gc := prodDec.Decode(idx, &c)` → (gp_prod, gc_prod).
- reference: `refState := &referenceGainState{}; refState.initInternalState(); gp_ref, gc_ref := refState.decode(idx.GA, idx.GB, &c)`

비교:
- `gp_prod == gp_ref`? (Q14 비트 동일)
- `gc_prod == gc_ref`? (Q12 비트 동일)
- `prodDec.pastErrors == refState.pastErrors`? (FIFO 동일)

본 비교를 *두 분기* (production indexing / spec-fix indexing) 모두 수행. spec-fix 분기에서도 *reference impl* 동일 결과 (즉 비선형 체인은 두 분기 모두 정상이지만 indexing 만 다름) 이면, F-quart-1 단일-fix 가 *비선형 체인 결함* 동시 해결 불필요.

- [ ] **Step 9: 비교 결과별 결론 분기**

| 결과 | 의미 | 보고서 §결론 |
|------|------|-----------|
| 두 분기 모두 prod=ref (gp/gc/FIFO 비트 동일) | 비선형 체인 결함 0 | 단일 fix 충분조건 가능 (F-quart-1 시나리오 S1·S2 와 직교) |
| 두 분기 모두 prod≠ref | 비선형 체인 결함 1+개 (indexing 무관) | 다중 fix 필수, F-quint plan 에 비선형 체인 fix 포함 |
| 한 분기만 prod=ref, 다른 분기 prod≠ref | 의외 — 분석 필요 | 단계별 dump 로 1-LSB diff 위치 좁힘 |

- [ ] **Step 10: 보고서 작성 + commit**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-3-report.md`:

```markdown
# Phase 1k Stage F-quart-3 진단 노트 — gain.Decode 비선형 체인

**작성일**: 2026-04-28
**범위**: gain.Decoder.Decode 의 §3.9 / §4.1.6 비선형 체인 (predictedLogGain → ecEnergy → log2 → pow2 → MA FIFO) line-by-line 인용 + reference impl 평행 검증.
**산출물**: 비선형 체인 결함 후보 식별 또는 부정.
**준수**: ITU-T G.729 (06/2012) PDF §3.9 / §A.3.9 / §4.1.6 만 인용. Reference impl 은 spec 식 직접 도출 — 외부 구현 0건 참조.

## 0. Working tree 상태 + escape hatch 평가
## 1. §3.9 식 (66)/(68)/(69)/(70)/(71)/(74) verbatim 인용
## 2. pastErrorsDefault / log2Fixed / pow2Fixed / dB 상수 검증
## 3. zero-energy guard 분류 (자체 안전망 vs spec 명시)
## 4. Reference impl 사양 (spec § 식 직접 구현)
## 5. Production vs reference 두 분기 비교 결과
## 6. 결론 — 비선형 체인 결함 후보 ranking
```

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-3-report.md \
        internal/decoder/stagef_quart_diagnostic_test.go
git commit -m "$(cat <<'EOF'
test(decoder): add Stage F-quart-3 gain.Decode reference cross-check

Stage F-quart-3 implements a §3.9-direct reference of gain.Decoder.Decode
(equations 66/68-71/74, MA predictor, log2/pow2 chain) without referencing
any existing G.729 implementation, then cross-checks production gain.Decode
output against the reference for ALGTHM frame 0 sf0/sf1 under both
production and §3.9.3 inverse-mapped GA/GB indexing branches.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-quart-4: 종합 분석 + F-quint plan 권고

**Goal:** F-quart-1 / F-quart-2 / F-quart-3 의 모든 측정값과 spec § 인용을 통합해, 단일 fix vs 다중 fix 결정 근거가 되는 ranking 표를 산출한다. 그 결과로 *F-quint plan 권고* (spec 위반 후보 적용 순서, 각 후보 적용 후 예상 정렬도, 강압-적합 회피를 위한 진단 obligation) 를 작성한다.

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-4-report.md` (보고서)
- **Modify: 없음, 신규 test: 없음**

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected: F-quart-3 종료 후 상태 (3 보고서 + 진단 하니스 commit + production diff 0 라인).

- [ ] **Step 2: 3 보고서 핵심 발견 통합**

각 보고서의 §0 (escape hatch 평가) 와 §결론 을 발췌해 단일 표로 통합:

| 후보 | F-quart 보고서 | spec 위반? | 본 stimulus 영향? | 단독-fix 정렬 시나리오 |
|------|---------|---------|--------------|---------------|
| GainImap inverse-map 누락 | F-quart-1 | ✓ §3.9.3 | ? (실측) | S1/S2/S3 (실측 분류) |
| pitch.AdaptiveCodebook | F-quart-2 | ? | frame 0 sf0 = pastExc=0 ⇒ 0 영향 | n/a |
| fcb.Decode (decodePositions/placePulses/applyPitchEnhancement) | F-quart-2 | ? | C1=0 / S1=15 / β=lower-bound | ? |
| pitch enhancement β clamp | F-quart-2 | ? | prevGpQ14=0 → lower-bound | ? |
| gain.Decode 비선형 체인 | F-quart-3 | ? | reference cross-check | ? |
| filterSubframe ÷2/×2 (F-tris-2 부산물) | F-tris-2 §3.4 | ✓ §3.10/§A.3.10 (÷4/×4) | 미-trigger (overflow 0) | n/a |

`?` 위치를 F-quart-1/2/3 결과 측정값으로 채운다.

- [ ] **Step 3: 단일/다중 fix 시나리오 결정**

§2 표 채우기 결과로:

- **시나리오 A — 단일 fix 충분조건**: F-quart-1 시나리오 S1 (40/40 정렬) + F-quart-2/3 잔여 spec-위반 0 (현재 stimulus 영향 0 이거나 spec 일치). → F-quint plan = *단일-commit fix* (gain.decodeVQ 의 GainImap 적용 + docstring 수정).

- **시나리오 B — 다중 fix 필수**: F-quart-1 시나리오 S2 (부분 정렬) 또는 F-quart-2/3 추가 spec-위반 1+ 발견. → F-quint plan = *복합-commit fix* (각 위반을 *순차* 적용 + 각 적용 직후 정렬도 측정).

- **시나리오 C — 단일 fix 부정**: F-quart-1 시나리오 S3 (악화). → F-quint plan = *근본 재진단 cycle* (정렬 악화 원인 = pastErrors / pow2 / β 의 cross-coupling 의심) + docstring 자체-spec-위반 별도 분리 fix.

- [ ] **Step 4: Strict ordering 결정 — 다중 fix 시 적용 순서**

시나리오 B 의 경우, fix 들 사이의 *적용 순서*가 정렬도에 영향. 본 단계는 dependency graph 그리기:

- (1) GainImap inverse-map: idx → g_p/γ̂_c → gain.Decode → u → s 영향. *최상류*.
- (2) pitch enhancement β clamp: c → u → s 영향. (1) 과 *독립*.
- (3) fcb.Decode 위치/sign: c 영향. (1)/(2) 와 *독립*.
- (4) gain.Decode 비선형 체인: gc 영향. (1) 과 *coupled* (γ̂_c 입력).
- (5) filterSubframe ÷2/×2: 본 stimulus 미-trigger.

순서: (3) → (2) → (1) → (4). 각 fix 적용 직후 sample 0..7 정렬도 측정 의무.

- [ ] **Step 5: F-quint plan 권고서 본문 작성**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-4-report.md`:

```markdown
# Phase 1k Stage F-quart-4 종합 노트 + F-quint plan 권고

**작성일**: 2026-04-28
**범위**: F-quart-1/2/3 종합 + 단일/다중 fix 결정 + 적용 순서.
**산출물**: F-quint plan 권고서 (단일-commit / 복합-commit / 재진단 cycle 중 하나).
**준수**: 본 보고서는 F-quart-1/2/3 보고서만 인용 (spec § 인용은 각 보고서에서 단일 출처로 유지).

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4 최종)
## 1. F-quart-1/2/3 핵심 발견 통합표
## 2. 단일/다중 fix 시나리오 분류 (A/B/C)
## 3. (시나리오 B 인 경우) 다중 fix 적용 순서 + 각 단계 예상 정렬도
## 4. F-quint plan 권고서
   4.1 사이클 입구 invariant
   4.2 Task ranking + 사용자 결정 요청 항목
   4.3 강압-적합 회피 obligation (각 fix 직후 정렬도 측정 + spec § 재인용)
## 5. 미해결 항목 (frame 1+ 의존, codec state-machine 외부 결함, ALGTHM 자체 오류 가능성) — F-quint 범위 외
```

- [ ] **Step 6: 사용자 결정 요청 항목 명시**

§4.2 에 *명시적 사용자 결정 항목* 작성:
- (a) F-quint plan 진입 (시나리오 A/B/C 중 식별된 분기로).
- (b) Stage F 부정 + 외부 정합 모델 진단 cycle 진입 (E1 발동 시).
- (c) 본 사이클 일시 정지 + frame 1+ 진단 사이클 우선 (시나리오 C 또는 본 stimulus 한정 결함 가설 검증 필요 시).

- [ ] **Step 7: Working tree post-check + commit**

Run: `git status --porcelain && git diff --stat -- internal/`
Expected: production diff 0 라인. 5 파일 (4 보고서 + 1 진단 하니스) commit 또는 미커밋 (계획서 종료).

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-4-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-quart-4 synthesis + F-quint plan recommendation

Stage F-quart-4 integrates F-quart-1/2/3 findings into a single ranking
table, classifies the fix landscape as A (single-commit) / B (multi-fix)
/ C (root re-diagnosis), and produces an F-quint plan recommendation
with a strict fix-application ordering plus per-step alignment-check
obligations to prevent forced fitting.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

Plan을 다시 읽고 다음을 확인:

**1. Spec coverage**:
- ✓ §3.7 / §3.7.1 (pitch.AdaptiveCodebook): F-quart-2 §1.
- ✓ §3.8 / §3.8.1 (fcb.Decode + pitch enhancement): F-quart-2 §2 / §3.
- ✓ §3.9 / §3.9.3 / §4.1.6 (gain VQ + 비선형 체인): F-quart-1 §1 + F-quart-3 §1·§2.
- ✓ §A.3.7 / §A.3.8 / §A.3.9: F-quart-2 §1 / §2 + F-quart-3 §1.
- ✓ Production code 0-수정 invariant: 모든 task §0 working tree pre-check + §9/§10 post-check 검증.
- ✓ Escape hatch E1/E2/E3/E4: Phase 0.2 + 모든 task §0.

**2. Placeholder scan**:
- F-quart-1 Step 5 의 "?" 는 *측정 결과로 채울 자리* (실측-driven). 측정 후 실제 값으로 교체 의무가 명시되어 있으므로 placeholder 가 아니라 실측 슬롯.
- F-quart-2 Step 6 의 검증표 "?" 도 동일.
- F-quart-3 Step 7 의 "... actual implementation per spec ..." 은 reference impl 의 *spec § 식별 + 직접 구현 의무*임을 명시 — 외부 구현 미참조 + spec § 인용에서 직접 도출 명시되어 있으므로 placeholder 아님 (구현 자유도가 의도된 것).
- F-quart-4 §3 의 "예상 정렬도" 는 시나리오 B 한정 + F-quart-1 실측값 기반 산출이므로 placeholder 아님.

**3. Type consistency**:
- `gpQ14` (int16, Q14): 4 task 모두 동일.
- `gcQ12` / `gammaCQ13`: F-quart-1·F-quart-3 동일.
- `idx.GA` (uint8, 3-bit) / `idx.GB` (uint8, 4-bit): F-quart-1 Step 3 + F-quart-3 Step 7 동일.
- `tables.GainImap1` (`[8]uint8`) / `tables.GainImap2` (`[16]uint8`): F-quart-1 Step 3·Step 7 일관.
- F-quart-1 의 "branch A/B" 와 F-quart-3 의 "production/spec-fix 분기" 는 동일 개념 — 본 plan 에서 *동일 명칭 통일*: F-quart-1 §의 branch A=production / branch B=spec-fix; F-quart-3 §의 production/spec-fix 도 동일 매핑.

**4. 본 plan 의 외부 구현 참조 0**: 모든 spec 인용은 ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) 단일 출처. F-quart-3 Step 7 의 reference impl 은 *spec § 식 직접 구현 + 외부 구현 미참조* 명시. ✓

**5. 강압-적합 회피**: F-quart-1 시나리오 분류 (S1/S2/S3) + F-quart-4 시나리오 분류 (A/B/C) + 다중 fix 적용 순서 + 각 단계 직후 정렬도 측정 의무 = 강압-적합 위험을 *실측 게이트*로 차단. F-quint 가 시나리오 B 인 경우, 첫 fix 적용 직후 정렬도 *악화* 면 즉시 revert + 재진단.

**6. Co-author trailer**: 4 commit 모두 `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` 포함.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-plan.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
