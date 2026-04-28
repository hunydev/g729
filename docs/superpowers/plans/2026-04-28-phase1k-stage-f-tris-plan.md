# Phase 1k Stage F-tris Implementation Plan (P fix + 상류 진폭 2× 과대 보정 동시 수정)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-bis-2 보고서가 escape hatch 발동으로 노출한 결함 — `pcm.ScaleUpSat`는 §4.2.5 ×2 spec-correct 이며, 진정한 결함은 *상류*에서 sample-의존적으로 진폭을 2× 부풀리는 단계 — 를 PST/2 (spec-mandated pre-×2 도메인) 비교 기준으로 식별하고, P fix와 단일 커밋으로 동시 수정. ALGTHM frame 0 sf0 (40 샘플) 비트-정확 + Phase 1i sample 0 잠금 자연 회복.

**Architecture:** F-bis-2 보고서가 확보한 4개 사실에서 출발한다.
- (사실 1) `pcm.ScaleUpSat` (`internal/pcm/scale.go:17-25`)는 ITU-T G.729 §4.2.5 / §A.4.2.5 "multiplied by a factor 2 to restore the input signal level" 와 line-by-line 일치. **변경 대상 아님.**
- (사실 2) F-bis 플랜의 *§3.10 ÷2* 인용은 *작성 오류*. 실제 §3.10는 메모리 갱신 절차로 출력 스케일링과 무관. 출력 스케일링 규정은 §4.2.5 ×2 단일 조항.
- (사실 3) F-bis-1 stage-by-stage 측정 데이터를 PST/2 도메인 기준으로 재해석:
  - sample 0/2/3: hpFilter 출력이 spec 기대 (PST/2 = 1, 1.5, 1.5)의 **2× 과대** (production 2, 3, 3)
  - sample 1: hpFilter 출력이 spec 기대 (= 2)와 **정상 일치** (production 2)
  - 즉 상류 *어느 단계*에서 **sample-의존적으로 0/2/3은 2× 부풀고 1은 정상**인 결함이 발생.
- (사실 4) Phase 1i sample 0 잠금 통과는 두 결함 (P 결함이 진폭을 약 1/2 축소 + 상류 결함이 진폭을 sample 0 위치에서 2× 확대) 의 *우연한 상쇄* 결과. P fix 단독 적용 시 첫 결함만 제거되어 두 번째 결함이 sample 0에 노출 (got=4 want=2).

본 플랜은 (i) 진단 비교 기준을 PST → PST/2로 변경한 확장 stage-by-stage 진단 (sample 0..39 전체)로 상류 결함 단계를 식별, (ii) §4.2.4 / §A.4.2.4 / §3.10 / §3.4 등 후보 § 인용 line-by-line 분석으로 결함 위치 + 방향 확정, (iii) P fix와 상류 fix를 단일 커밋으로 묶어 sf0 40 샘플 비트-정확 달성 + Phase 1i 잠금 자연 회복.

**Tech Stack:** Go 1.22+, `internal/{lsp,synth,postfilter,decoder}` 모듈 수정 가능. `internal/pcm/scale.go`는 변경 금지 (spec-correct 확정).

**Scratch-from-spec discipline:** ITU 참조 C, bcg729, Sipro Lab, FFmpeg 절대 참조 금지. ITU-T G.729 (06/2012) §3.x / §4.x / §A.4.x + LSP-LP 표준 이론(Schur–Cohn step-down)만 사용. spec PDF: `docs/superpowers/specs/itu/G729E.pdf`.

**Co-author trailer for every commit:**
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`

**플랜 인용 오류 정정** (F-bis 플랜 무효화 사항):
- F-bis 플랜의 "§3.10 The output speech is finally divided by 2 with saturation control" 인용은 spec에 존재하지 않는 문장 — F-bis 플랜 작성자가 §4.2.5 ×2를 ÷2로 오인용. 본 플랜은 §4.2.5 / §A.4.2.5 ×2 spec-correct 기준으로 모든 분석을 재출발.
- F-bis-1 진단 보고서 §3.2 "결함 위치 = pcm.ScaleUpSat" 결론은 *경계 식별*은 정확하나 *방향 라벨링*이 잘못. 본 플랜에서 "결함 *경계* = ScaleUpSat 직전, 결함 *방향* = 상류 진폭 2× 과대 (sample 의존적)" 로 재라벨링.

---

## File Structure

| 파일 | 역할 | 상태 |
|------|------|------|
| `internal/lsp/lsp_lp.go` | F-bis-1 P fix 그대로 보존 (int64 exact 누산) | working tree에 유지 → F-tris-3 commit |
| `internal/decoder/stagef_bis_diagnostic_test.go` | F-bis-1 진단 하니스. 비교 기준을 PST/2로 변경 + sample 0..39 전체 capture로 확장. 파일명도 갱신 검토. | 수정 (F-tris-1) |
| `internal/postfilter/agc.go` 또는 `internal/synth/filter.go` 또는 기타 상류 단계 | F-tris-1이 식별한 상류 결함 위치 | 수정 (F-tris-3) |
| `internal/decoder/frame0_regression_test.go` | sample 40 가드 추가 | 수정 (F-tris-3) |
| `internal/decoder/frame0_regression_test.go` | sample 80 가드 (V1) | 수정 (V1) |
| `internal/decoder/decode_test.go` | ALGTHM skip 메시지 갱신 | 수정 (V2) |
| `internal/gain/pathological_test.go` | A+B 병리 재인증 | 수정 (V3) |
| `docs/superpowers/plans/2026-04-28-phase1k-stage-f-tris-completion-report.md` | 완료 보고서 (인용 정정 + 누적 결과) | 신규 |

`internal/pcm/scale.go`는 본 플랜의 모든 태스크에서 **변경 대상 아님** — F-bis-2가 line-by-line spec 일치 확정.

---

## Spec-derived Reference Values

본 단계 전체에서 공통으로 사용할 손계산 참값. 모든 인용은 `docs/superpowers/specs/itu/G729E.pdf` (ITU-T G.729 (06/2012) 본문 + Annex 통합본) 기준.

### §3.1.1 인코더 입력 ÷2 / §4.2.5 디코더 출력 ×2 대칭 (spec-mandated)

PDF p.5, §3.1.1 "Preprocessing":
> "The scaling consists of dividing the input by a factor 2 to reduce the possibility of overflows in the fixed-point implementation. ... Both the scaling and high-pass filtering are combined by dividing the coefficients at the numerator of this filter by 2."

PDF p.29, §4.2.5 "High-pass filtering and upscaling":
> "The filtered signal is multiplied by a factor 2 to restore the input signal level."

PDF p.43, §A.4.2.5 "High-pass filtering and upscaling":
> "Same as described in clause 4.2.5."

→ **인코더 ÷2 ↔ 디코더 ×2** 대칭. 디코더 합성/postfilter/HP 단계는 *반-진폭 도메인* (PST/2)에서 동작해야 하며, ScaleUpSat ×2가 PST 도메인으로 복원.

### §A.4.2.4 (G.729A) AGC seed + α 변경

PDF p.43, §A.4.2.4 "Adaptive gain control":
> "The adaptive gain control is given by:
>
>   sf'(n) = g(n) · sf(n)        (A.40)
>   g(n) = α · g(n−1) + (1−α) · g_t(n)   (A.41)
>
> where g_t(n) is the gain factor and α is the smoothing factor (α = 0.9). The initial value of g(−1) = 1.0 is used. Then for each new subframe, g(−1) is set equal to g(39) of the previous subframe."

키 인용:
- **α = 0.9** (G.729A; G.729 본문 §4.2.4는 α = 0.99 명시. Annex A가 α만 변경.)
- **초기값 g(−1) = 1.0** (= 16384 Q14, = 16777216 Q24)
- 이후 subframe: g(−1) = 직전 subframe의 g(39)

### F-bis-1 데이터 (PST/2 도메인 재해석, F-bis-2 §1.7 표 인용)

| sample | PST want | spec-mandated pre-×2 (= PST/2) | production hpFilter 출력 | 상류 진폭 비율 (production / spec) |
|---:|---:|---:|---:|---:|
| 0 | 2 | 1 | 2 | **2× 과대** |
| 1 | 4 | 2 | 2 | 1× (정상) |
| 2 | 3 | 1 또는 2 | 3 | **2× 과대** (1.5의 2×) |
| 3 | 3 | 1 또는 2 | 3 | **2× 과대** (1.5의 2×) |

→ 상류 결함은 단일 진폭 상수 ×2가 아니라 *sample-의존적* 패턴. AGC g(n)이 첫 호출 시 잘못된 seed로 시작하면 평활화 식 (A.41)에서 sample 0~39 동안 점진적으로 g_target에 수렴 — 이 과도구간 동안 sample-의존 진폭 왜곡이 발생할 수 있다.

### Schur-Cohn step-down (F-bis-1 검증, P fix 후 |k_m|<1 모두 통과)

```
k_10=-0.008057  k_9=+0.031081  k_8=-0.006054  k_7=-0.009197
k_6=+0.068325   k_5=+0.020135  k_4=-0.017413  k_3=-0.011058
k_2=-0.096825   k_1=-0.595293
```

§3.2.6 minimum-phase 회복 — P fix 정확. 본 플랜에서 P fix를 그대로 보존하여 F-tris-3에서 단일 커밋.

### ALGTHM frame 0 sf0 입력 (Stage D-bis Task 3 인용)

```
LSP indices: L0=1, L1=105, L2=17, L3=0
Pitch sf0:   tInt=20, tFrac=+0
FCB sf0:     C1=0, S1=15  (4-pulse, 모든 +Q13, 위치 0~3 + 피치-증강 20~23 β=0.2)
Gain sf0:    GA1=5, GB1=6 → gpQ14=13815, gcQ12=6844
```

---

## Bite-Sized Task Granularity

각 태스크 패턴:
1. (필요 시) 진단 하니스 추가/수정 또는 분석 노트 산출
2. 어서션/측정 실행 → 분석
3. 분석 결과로 다음 태스크의 진입 조건 결정
4. F-tris-3에서만 단일 커밋 — 이전 단계는 진단/측정 또는 분석-only

---

### Task 1 (F-tris-1): 진단 하니스 PST/2 비교 기준 + sample 0..39 전체 확장

**Files:**
- Modify (uncommitted, working tree only): `internal/decoder/stagef_bis_diagnostic_test.go`
- Preserve (uncommitted): `internal/lsp/lsp_lp.go` (F-bis-1 P fix 그대로 유지)

**Why:** F-bis-1 진단 하니스는 PST 도메인 직접 비교로 sample 0의 ×2 *경계* 식별에 성공했지만 *방향 라벨링*이 잘못 (F-bis-2 §1.7). 본 태스크는 비교 기준을 PST/2 (= spec-mandated pre-×2 도메인)로 변경하여 상류 단계 (synth.Filter / postfilter.Filter / hpFilter)에서 진폭 2× 과대가 *어느 단계*에서 *어느 sample*에 진입하는지 측정한다.

**중요 (escape hatch 1 절대 준수):**
- working tree는 RED 상태로 진행 (P fix 단독 적용 → `TestDecode_Frame0Sample0_MatchesALGTHM` got=4 want=2 FAIL).
- 본 태스크는 진단-only. **커밋 없음.**

- [ ] **Step 1: working tree 사전 점검**

```bash
git status
go test -v -run "TestALGTHMFrame0SF0_AzStability" ./internal/lsp/
go test -v -run "TestDecode_Frame0Sample0_MatchesALGTHM" ./internal/decoder/
```

Expected:
- `git status`: `internal/lsp/lsp_lp.go` modified + `internal/decoder/stagef_bis_diagnostic_test.go` untracked
- `TestALGTHMFrame0SF0_AzStability`: PASS (F-bis-1 §2 인용)
- `TestDecode_Frame0Sample0_MatchesALGTHM`: FAIL (got=4 want=2) — 의도된 RED

만약 working tree가 위 상태와 다르면 **즉시 멈춤** + 사용자에게 "F-bis-1 working tree 손실" 보고 → P fix 재적용 안내.

- [ ] **Step 2: 진단 하니스를 PST/2 비교 + sample 0..39 전체 capture로 확장**

`internal/decoder/stagef_bis_diagnostic_test.go`를 다음과 같이 수정 (기존 파일을 *대체*; 보조 함수는 신규 추가):

```go
package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace: Stage F-tris-1 진단.
//
// 후보 P fix(`internal/lsp/lsp_lp.go` int64 누산)가 working tree에
// 적용된 상태에서 ALGTHM frame 0 sf0 의 sample 0..39 *전체* 를
// 4개 stage 경계 (synth.Filter → postfilter.Filter → hpFilter →
// pcm.ScaleUpSat) 에서 측정하고, 비교 기준을 spec-mandated
// **PST/2 도메인** (§4.2.5 / §A.4.2.5 의 pre-×2 단계) 으로 갱신.
//
// 본 테스트는 진단-only(t.Logf). escape hatch 1 준수: P fix 단독
// working tree에서는 RED 상태이므로 t.Errorf 미사용.
//
// F-bis-2 §1.7 재해석 결과:
//   - synth/postfilter/hpFilter 출력은 PST/2 도메인이어야 spec-correct.
//   - F-bis-1 측정 (sample 0..3): hpFilter 출력은 sample 0/2/3에서
//     PST/2의 2× 과대, sample 1만 정상.
//   - 본 진단은 위 패턴이 sample 4..39까지 어떻게 진화하는지, 그리고
//     2× 과대가 합성 단계 (synth.Filter) 에서 이미 발생하는지
//     postfilter/hpFilter에서 누적되는지 식별한다.
//
// 출력 형태:
//   - 각 stage 경계의 sample 0..39 dump (4 column)
//   - PST want / PST/2 spec-target / Δ (각 stage vs PST/2) sample 0..39
//   - Stage-진입-위치 식별: synth.Filter ↔ PST/2 일치 sample 수 vs
//     postfilter.Filter ↔ PST/2 일치 sample 수 비교로 결함 진입 단계
//     좁히기.
func TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	var d Decoder

	sf1A, _ := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))

	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt1, tFrac1, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt1, betaQ14, &c)

	gpQ14, gcQ12 := d.gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	t.Logf("u[0..7] = %v", u[:8])
	t.Logf("a[] (Q12) = %v", sf1A)

	// Boundary 1: synth.Filter
	var sRaw [subframeLen]int16
	d.syn.Filter(&sf1A, &u, &sRaw)

	// Boundary 2: postfilter.Filter
	var sPf [subframeLen]int16
	d.pst.Filter(&sf1A, tInt1, &sRaw, &sPf)

	// Boundary 3: hpFilter
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])

	// PST want / PST/2 spec-target
	var want [subframeLen]int16
	var spec [subframeLen]int16 // PST/2, ½-LSB 라운딩 허용
	for n := 0; n < subframeLen; n++ {
		want[n] = wantFrames[0][n]
		// PST/2: round half-to-even or simple >>1 — 본 진단은
		// |Δ|≤1 LSB 까지 일치로 간주.
		spec[n] = int16(int32(want[n]) >> 1)
	}

	t.Logf("PST want sf0 (sample 0..39):")
	dumpInt16(t, want[:])
	t.Logf("PST/2 spec-target (= want >> 1, sample 0..39):")
	dumpInt16(t, spec[:])

	t.Logf("BOUNDARY synth.Filter sf0 (sample 0..39):")
	dumpInt16(t, sRaw[:])
	t.Logf("BOUNDARY postfilter.Filter sf0 (sample 0..39):")
	dumpInt16(t, sPf[:])
	t.Logf("BOUNDARY hpFilter sf0 (sample 0..39):")
	dumpInt16(t, hpOut[:])

	// Δ tables vs PST/2 (1 LSB 허용 일치 카운트)
	cntSynth := matchCount(sRaw[:], spec[:], 1)
	cntPst := matchCount(sPf[:], spec[:], 1)
	cntHp := matchCount(hpOut[:], spec[:], 1)
	t.Logf("Match count vs PST/2 (|Δ|≤1 LSB):")
	t.Logf("  synth.Filter:      %d / 40", cntSynth)
	t.Logf("  postfilter.Filter: %d / 40", cntPst)
	t.Logf("  hpFilter:          %d / 40", cntHp)

	// Stage-진입 분석:
	// - synth.Filter cnt가 거의 40이면 합성 단계는 spec-correct, 결함은
	//   postfilter 또는 hpFilter (이는 hpFilter spec-correct가 가장
	//   가능성 높으므로 postfilter 1순위).
	// - synth.Filter cnt가 낮으면 합성 단계 자체에 진폭 결함 — synth
	//   Q-format 또는 saturation recovery 결함.
	// - 모든 stage cnt가 비슷하게 낮으면 경계 식별 불가 → 다른 진단 필요.

	t.Logf("Per-sample Δ (production stage vs PST/2 spec):")
	for n := 0; n < subframeLen; n++ {
		t.Logf("  n=%2d: want=%5d spec=%5d  synth=%5d (Δ=%+d)  postfilter=%5d (Δ=%+d)  hpFilter=%5d (Δ=%+d)",
			n, want[n], spec[n],
			sRaw[n], int32(sRaw[n])-int32(spec[n]),
			sPf[n], int32(sPf[n])-int32(spec[n]),
			hpOut[n], int32(hpOut[n])-int32(spec[n]),
		)
	}

	// 추가 진단: AGC g_target, agcGainPrev seed/end, 그리고 §A.4.2.4
	// 평활화 식 g(n) = α·g(n-1) + (1-α)·g_t(n) 의 첫 호출 시 거동.
	//
	// 주의: 이 정보는 Postfilter 내부 상태이므로 export 어댑터 또는
	// 디버그 콜백이 필요. 본 진단은 *측정-only*; 어댑터 추가가 필요하면
	// 진단 hook을 internal/postfilter에 임시로 추가 (untracked, F-tris-2
	// 분석 종료 시 제거).
}

// dumpInt16 prints v as 5 rows of 8 columns, each %5d wide. Used for
// human-readable sample dump in t.Logf.
func dumpInt16(t *testing.T, v []int16) {
	t.Helper()
	for r := 0; r < 5; r++ {
		base := r * 8
		t.Logf("  [%2d..%2d] %5d %5d %5d %5d %5d %5d %5d %5d",
			base, base+7,
			v[base+0], v[base+1], v[base+2], v[base+3],
			v[base+4], v[base+5], v[base+6], v[base+7],
		)
	}
}

// matchCount returns the number of indices i where |a[i]-b[i]| <= tol.
func matchCount(a, b []int16, tol int32) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	c := 0
	for i := 0; i < n; i++ {
		d := int32(a[i]) - int32(b[i])
		if d < 0 {
			d = -d
		}
		if d <= tol {
			c++
		}
	}
	return c
}
```

**가드레일:**
- 기존 `TestDiagnostic_FbisStageBoundaries_Sample0Trace` 함수는 *삭제하지 말고 유지* — F-bis-1 보고서 §3 인용을 재현 가능 상태로 보존. 본 태스크는 새 함수를 추가.
- `dumpInt16`, `matchCount` 보조 함수는 동일 파일 또는 별도 *_test.go에 신규.
- `d.syn`, `d.pst`, `d.hpFilter`, `d.pastExc`, `d.prevGpQ14`, `d.gn`은 internal/decoder 패키지 내부 가시성. 시그니처가 F-bis-1 진단 하니스와 다르면 그 형태에 맞춰 조정.

- [ ] **Step 3: 진단 실행**

Run:
```bash
go test -v -run TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace ./internal/decoder/ 2>&1 | tee /tmp/ftris1.log
```

Expected log: PST want / PST/2 spec / 3개 stage 출력 + sample 0..39 Δ 표 + match count.

- [ ] **Step 4: 진단 결과 분석 (보고서 §2 작성용)**

다음 표를 채울 데이터를 `/tmp/ftris1.log`에서 추출:

| 항목 | 값 | 해석 |
|------|---:|------|
| `synth.Filter` ↔ PST/2 match count (sample 0..39) | _N₁_ | synth 단계 spec-correct? |
| `postfilter.Filter` ↔ PST/2 match count | _N₂_ | postfilter 진입 시 진폭 변화? |
| `hpFilter` ↔ PST/2 match count | _N₃_ | hpFilter 통과 후 잔여 결함? |
| 첫 미스매치 sample | _n_first_ | 결함 시작 위치 |
| 미스매치 패턴 (every-N? 처음-N? 무작위?) | _pattern_ | sample-의존성 형태 |

판정 가이드:
1. **N₁ ≈ N₂ ≈ N₃ ≈ 40 (모든 stage spec-correct)**: F-bis-1 측정과 모순 → 진단 하니스 자체 결함 + escape hatch.
2. **N₁ 낮음, N₂/N₃은 N₁과 비슷**: 결함은 *synth.Filter* (또는 그 직전 BuildExcitation, gain.Decode, fcb.Decode 등). F-tris-2 1순위 = synth 모듈 §3.10/§3.4 분석.
3. **N₁ ≈ 40, N₂ 낮음, N₃ ≈ N₂**: 결함은 *postfilter*. F-tris-2 1순위 = `postfilter/agc.go` (§A.4.2.4 seed) 또는 short-term postfilter γ_n/γ_d (§3.8/§A.4.2.3).
4. **N₁ ≈ N₂ ≈ 40, N₃ 낮음**: 결함은 *hpFilter*. F-tris-2 1순위 = `decoder/decode.go::hpFilter` (§4.2.5 H_h2 계수 / 초기 상태).
5. **위 어떤 패턴에도 부합 안함**: 결함이 multi-stage cancellation. F-tris-2 진입 보류 + 사용자 보고.

**중요**: F-bis-2 §2.4의 1순위 후보 (postfilter AGC seed)는 **시나리오 3**에 정확히 해당. 본 진단의 결과가 시나리오 3이면 F-tris-2가 그 가설을 spec § 인용으로 검증할 수 있다.

- [ ] **Step 5: 진단 종료 — 미커밋 유지**

본 태스크는 측정만. **커밋 없음.** working tree:
- `internal/lsp/lsp_lp.go`: P fix (uncommitted, F-bis-1에서 보존)
- `internal/decoder/stagef_bis_diagnostic_test.go`: 신규 함수 추가됨 (uncommitted)
- 그 외 production: 변경 없음

다음 태스크(F-tris-2)에서 분석 노트 산출 후 F-tris-3 단일 커밋으로 landing.

**Escape hatch 1 재확인**: F-tris-3 단일 커밋이 sample 0 잠금을 회복하지 못하면 즉시 working tree 롤백 (`git checkout -- internal/lsp/lsp_lp.go && rm internal/decoder/stagef_bis_diagnostic_test.go`) + 사용자 보고.

---

### Task 2 (F-tris-2): 식별된 상류 단계의 spec-인용 line-by-line 분석

**Files:**
- (분석-only, 코드 수정 없음)

**Why:** F-tris-1 시나리오 결과에 따라 1순위 후보 단계의 ITU-T G.729 § 인용 + production 코드 line-by-line 비교 + 결함 위치(파일경로:라인) 확정. 강압-적합 금지 — F-tris-1 stage 식별만으로 임의 수정 금지.

- [ ] **Step 1: F-tris-1 시나리오에 따른 1순위 spec § 결정**

**(나) postfilter 진입 시 결함** (1순위 — F-bis-2 §2.4 권고):

**§A.4.2.4 (G.729A AGC) 인용** (PDF p.43):
> "g(n) = α · g(n−1) + (1−α) · g_t(n)
> where g_t(n) is the gain factor and α is the smoothing factor (α = 0.9). **The initial value of g(−1) = 1.0 is used.** Then for each new subframe, g(−1) is set equal to g(39) of the previous subframe."

비교 위치: `internal/postfilter/agc.go:49-75`. 두 결함 후보:
- (가) **seed**: `agc.go:53-56` `if !pf.initialized { pf.agcGainPrev = int32(gTargetQ24); pf.initialized = true }` → 첫 호출 시 g(−1) = g_target. spec은 g(−1) = 1.0 = 16384 Q14 = 16777216 Q24.
- (나) **α 값**: `agc.go:50` `const alphaQ15 int64 = 32440 // ≈ 0.99`. spec §A.4.2.4: α = 0.9. 0.9 Q15 = round(0.9 × 32768) = 29491. **G.729 본문 §4.2.4는 α=0.99이지만 G.729A는 α=0.9.** production 0.99 (= 32440 Q15)는 G.729A spec 위반.

**(다) synth 진입 시 결함** (2순위):

**§4.2.4 / §A.4.2.4 LP synthesis filter** (PDF p.28-29):
> "s(n) = u(n) − Σ_{i=1..10} a_i · s(n−i)"

비교 위치: `internal/synth/filter.go:60-69` (`onePass`). 후보 결함:
- LMult/LMsu/LShl 시프트 폭이 a[0]=4096 Q12 정규화와 어긋남
- Round 위치 또는 LMac 부호 처리 (G.729 §A.4.2 인용)

**§3.10 / §A.3.10 saturation recovery** (PDF p.24):
> "When overflow occurs, the speech samples and the filter memory are divided by 4 and the filtering is re-done. The output is multiplied by 4 with saturation."

비교 위치: `internal/synth/filter.go:31-52`. 현 ÷2+×2는 spec ÷4+×4와 어긋남 — **F-prep-2가 이미 노출**.

**(라) hpFilter 진입 시 결함** (3순위):

**§4.2.5 / §A.4.2.5 HP filter coefficients** (PDF p.29):
> "H_h2(z) = (0.93980581 − 1.8795834·z⁻¹ + 0.93980581·z⁻²) / (1 − 1.9330735·z⁻¹ + 0.93589199·z⁻²)        (91)"

비교 위치: `internal/decoder/decode.go::hpFilter` 또는 `internal/decoder/hp.go`. 후보 결함:
- 계수 값 (b_0, b_1, b_2, a_1, a_2)
- 초기 상태 (`hpX[0]=hpX[1]=hpY[0]=hpY[1]=0`)
- Q-포맷 정규화 (계수 Q-format이 입력 도메인과 일치하는지)

- [ ] **Step 2: production 코드 line-by-line 대조**

식별된 단계의 production 코드를 읽고, Step 1에서 인용한 spec 식과 line-by-line 매핑.

체크리스트 (강압-적합 금지):
- 각 식의 *부호* 일치 (덧셈 vs 뺄셈)
- 각 변수의 *Q-포맷* 일치 (Q12, Q14, Q15, Q24, Q28 등)
- 각 시프트(LShl/LShr)의 폭 정확
- 각 saturation 위치 (중간 단계 saturation 금지 vs 최종 단계 saturation 허용)
- 각 상수의 값 (예: α = 0.9, b_0 = 0.93980581 Q15, …) 일치

- [ ] **Step 3: hand-calc — 후보 fix가 F-tris-1 측정값을 PST/2에 정렬시키는지 검증**

본 태스크가 강압-적합 방지의 핵심. 식별된 결함의 fix 후보를 *코드 적용 없이* hand-calc로 first-order 검증:

후보 (나-가) AGC seed 1.0:
- g(−1) = 16777216 (Q24, = 1.0 Q14 << 10)
- g_target = (F-tris-1 진단에서 capture한 값)
- g(0) = 0.99·1.0 + 0.01·g_target  (현 production α=0.99)
  = 0.99 + 0.01·g_target_normalized
- 또는 §A.4.2.4 α=0.9 적용 시 g(0) = 0.9·1.0 + 0.1·g_target

production 시드 (g(−1) = g_target) 대비, *spec seed (g(−1) = 1.0) + spec α (= 0.9)*가 sample 0..3의 진폭을 PST/2 도메인에 가깝게 정렬시키는지 산술 검증:
- sample 0의 sTilt[0] (postfilter.Filter 입력) 값 capture
- spec seed + spec α로 g(0) 계산
- spec g(0) × sTilt[0] ÷ 2^24 round → spec sPf[0] 후보
- 이 값이 PST/2[0] (= 1)과 비트-정확 또는 ±1 LSB 일치하는지 확인

본 hand-calc가 PASS이면 후보 (나-가) + (나-나)는 결함 위치로 강한 후보. FAIL이면 다른 단계 재고려.

- [ ] **Step 4: 결함 위치 + spec § 인용 + 손계산 기대값 분석 노트 산출**

본 태스크는 코드 수정/커밋 없음. 다음 형태의 분석 노트 (F-tris-3 커밋 메시지에 그대로 사용):

```
결함 위치 1: <파일경로:라인>  (예: internal/postfilter/agc.go:53-56)
spec § 인용: ITU-T G.729 §A.4.2.4 "<직접 인용>"
현 production: <라인 그대로 복사>
어긋남: <어떤 부호/Q-포맷/시프트/상수가 spec과 다른지>
수정안: <spec § 인용에 맞춘 코드 형태>

결함 위치 2 (있을 시): <파일경로:라인>  (예: internal/postfilter/agc.go:50)
spec § 인용: ITU-T G.729 §A.4.2.4 "α = 0.9"
현 production: const alphaQ15 int64 = 32440 // ≈ 0.99
어긋남: G.729A α는 0.9 (= 29491 Q15), production은 G.729 본문 0.99
수정안: const alphaQ15 int64 = 29491 // = 0.9 Q15, ITU-T G.729 §A.4.2.4

sample 0..3 영향 hand-calc: <Step 3 결과 표>
F-tris-1 N₁/N₂/N₃ 매칭 카운트 변화 예측: <표>
```

**Escape hatch:**
- Step 2에서 식별된 단계의 production 코드가 spec § 인용과 *완전히 일치* (F-bis-2의 ScaleUpSat 사례 재발 가능성)하면 즉시 멈춤 + 사용자에게 "F-tris-1 식별 stage와 F-tris-2 spec 대조 충돌" 보고.
- Step 3 hand-calc가 후보 fix로 PST/2 정렬을 *예측 불가*하면 (= 다른 결함이 더 있음), F-tris-3 진입 보류 + 사용자에게 "후보 fix만으로 sf0 비트-정확 도달 불확실, 추가 진단 필요 (F-quart 사이클)" 보고.

---

### Task 3 (F-tris-3): P fix + 상류 fix 단일 커밋 + sample 40 가드

**Files:**
- Modify: `internal/lsp/lsp_lp.go` (F-bis-1 P fix 그대로 landing)
- Modify: F-tris-2가 식별한 상류 단계 파일 (1순위: `internal/postfilter/agc.go`)
- Modify: `internal/decoder/frame0_regression_test.go` (sample 40 가드)
- (옵션) Add or Remove: `internal/decoder/stagef_bis_diagnostic_test.go` — 진단 하니스 영구 보존 가치 판단

**Why:** F-bis-1 P fix와 F-tris-2 분석 노트의 상류 fix를 spec § 인용에 따라 동시 수정. Phase 1i sample 0 잠금이 새 spec-준수 경로에서 자연 회복(got=2 want=2). sf0 sample 40까지 비트-정확 가드 추가.

- [ ] **Step 1: F-tris-2 분석 노트에 따라 상류 fix 적용**

분석 노트 그대로 production 코드 수정. **임의 수정 금지** — 분석 노트에 명시되지 않은 라인 변경은 본 커밋 범위 외.

가장 가능성 높은 시나리오 ((나) postfilter AGC) 의 수정 형태 예시 (실제 형태는 F-tris-2 분석 노트 기준으로 결정):

`internal/postfilter/agc.go`의 두 결함 동시 수정:

```go
// Spec correction (ITU-T G.729 §A.4.2.4):
//   - α = 0.9 (G.729A; G.729 본문 §4.2.4 α=0.99에서 변경)
//   - 첫 호출 시 g(−1) = 1.0; 이후 subframe에서 g(−1) = 직전 g(39)
const alphaQ15 int64 = 29491 // = 0.9 · 2^15, ITU-T G.729 §A.4.2.4

func (pf *Postfilter) applyAGC(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16) {
	gTargetQ24 := int64(gTargetQ14) << 10
	if !pf.initialized {
		// §A.4.2.4: g(−1) = 1.0 = 16384 Q14 = 16777216 Q24.
		pf.agcGainPrev = 16777216
		pf.initialized = true
	}

	g := int64(pf.agcGainPrev)
	for n := 0; n < subframeLen; n++ {
		g = (alphaQ15*g + (32768-alphaQ15)*gTargetQ24 + (1 << 14)) >> 15
		prod := g * int64(sTilt[n])
		v := (prod + (1 << 23)) >> 24
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		sPf[n] = int16(v)
	}
	if g < 0 {
		g = 0
	}
	pf.agcGainPrev = int32(g)
}
```

- [ ] **Step 2: 진단 하니스 보존/삭제 결정**

`internal/decoder/stagef_bis_diagnostic_test.go`를 다음 두 형태 중 하나:

**(가) 영구 가드로 보존 (권장):**
- t.Logf 출력 그대로 유지 (sample 0..39 stage trace는 향후 회귀 시 즉시 stage 식별 가능)
- 파일명 갱신 검토: `stagef_diagnostic_test.go` 또는 그대로 유지
- 본 커밋에 포함

**(나) 진단 종료, 삭제:**
- 보존 가치 < 비용 판단 시 `git rm internal/decoder/stagef_bis_diagnostic_test.go`

권장 (가). 단 t.Logf만 — assertion 승격 시 stage 출력값이 향후 spec 개선 시 변경 가능하므로 hard assertion 위험.

- [ ] **Step 3: sample 40 가드 추가**

`internal/decoder/frame0_regression_test.go`에 다음 테스트 추가:

```go
// TestDecode_Frame0SF0AllSamples_MatchesALGTHM: Stage F-tris 가드.
// Phase 1i sample 0 잠금에 더해 sf0 전체(40 samples)가 ALGTHM.PST
// frame 0과 비트-정확함을 영구 보장.
//
// Phase 1k Stage F-tris: §3.2.6 (LSP→LP int64 exact) +
// §<A.4.2.4 또는 식별된 §>의 spec-correct fix를 단일 커밋으로 landing한 결과.
func TestDecode_Frame0SF0AllSamples_MatchesALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	for n := 0; n < 40; n++ {
		if out[n] != wantFrames[0][n] {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				n, out[n], wantFrames[0][n], int32(out[n])-int32(wantFrames[0][n]))
		}
	}
}
```

- [ ] **Step 4: 회귀 게이트 + 영구 어서션 회귀 검사**

Run:
```bash
go test -race ./... 2>&1 | tee /tmp/ftris3-fix.log
go vet ./...
```

검증 항목 (모두 통과 필수):
- 신규: `TestDecode_Frame0SF0AllSamples_MatchesALGTHM` PASS
- 회복: `TestDecode_Frame0Sample0_MatchesALGTHM` PASS (got=2 want=2)
- 회복: `TestALGTHMFrame0SF0_AzStability` PASS
- 영구: Stage D 17개 + D-bis 3개 + F-prep-1 + F-prep-2 어서션 모두 PASS
- 영구: `internal/fixed` 0 allocs/op

**Escape hatch 1 (절대 준수):**
- `TestDecode_Frame0Sample0_MatchesALGTHM` FAIL → **Step 5 커밋 금지**, working tree 롤백, 사용자에게 "F-tris-2 식별 단계 또는 fix 형태 부정확 — 추가 분석 필요" 보고.
- `TestDecode_Frame0SF0AllSamples_MatchesALGTHM` sample 1~39 어디서든 FAIL → sf0 *내부*에 추가 결함. 즉시 멈춤 + 사용자 결정 (즉시 멈춤 vs F-quart 사이클). **자동 재시도 금지**.

**병리 케이스 회귀 점검:**
- `TestPathological*` (gain) 4건 PASS 확인. 본 fix는 postfilter (또는 다른 상류) 변경이므로 gain 모듈 영향 없음 예상. FAIL 시 Step 6 V3에서 처리.

- [ ] **Step 5: 단일 커밋 (P fix + 상류 fix + sample 40 가드 + 진단 하니스)**

```bash
git add internal/lsp/lsp_lp.go <F-tris-2 식별 단계 파일> internal/decoder/frame0_regression_test.go internal/decoder/stagef_bis_diagnostic_test.go
git commit -m "$(cat <<'EOF'
fix(lsp+<상류 모듈>): ALGTHM frame 0 sf0 bit-exact via §3.2.6 exact arithmetic + §<X.X> spec-correct path

§3.2.6 LSP→LP recurrence requires exact arithmetic; saturating the
intermediate F1, F2 polynomials in Word32 (Q28) caused asymmetric
a[] coefficients (|k_7|=1.897) at ALGTHM frame 0 sf0. Replace
[11]fixed.Word32 with [11]int64 in lspToLP/polyStep; final Q12
Word16 saturation only at output stage.

§<X.X> at <파일:라인>: <인용된 spec 식> requires <수정된 거동>;
production used <어긋난 거동>. Two defects (P fix exposing upstream
amplitude doubling) cancelled at Phase 1i sample 0 lock; both
spec-correct fixes land in a single commit.

Adds sf0 sample-by-sample regression guard (40 samples). pcm.ScaleUpSat
remains spec-correct per §4.2.5/§A.4.2.5 ("multiplied by a factor 2
to restore the input signal level") — not modified.

Stage F partial:    2026-04-27-phase1k-stage-f-partial-report.md
Stage F-bis-1:      2026-04-27-phase1k-stage-f-bis-1-report.md
Stage F-bis-2:      2026-04-27-phase1k-stage-f-bis-2-report.md
Stage F-tris plan:  2026-04-28-phase1k-stage-f-tris-plan.md

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

- [ ] **Step 6: 회귀 게이트 재실행 (커밋 후)**

Run: `go test -race ./... && go vet ./...`
Expected: ALL PASS, vet silent.

---

### Task 4 (V1): 프레임 0 80-sample 비트-정확 가드

**Files:**
- Modify: `internal/decoder/frame0_regression_test.go`

**Why:** Stage F 플랜 Task 4와 동일. ALGTHM 프레임 0 전체(sf0+sf1) 80-sample 일치 영구 어서션화.

- [ ] **Step 1: 80-sample 가드 추가**

```go
func TestDecode_Frame0AllSamples_MatchesALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}
	for n := 0; n < frameSamples; n++ {
		if out[n] != wantFrames[0][n] {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				n, out[n], wantFrames[0][n], int32(out[n])-int32(wantFrames[0][n]))
		}
	}
}
```

- [ ] **Step 2: 실행**

Run: `go test -v -run TestDecode_Frame0AllSamples_MatchesALGTHM ./internal/decoder/`

판단:
- **PASS** → V1 완료. Task 5(V2)로 진행.
- **FAIL on sf1 (sample 40~79)** → Stage F-tris 수정이 sf0만 고치고 sf1은 미해결. 가능성:
  - sf0 수정이 pastSynth/pastExc/agcGainPrev 상태에 잘못된 값을 남김
  - sf1 LP 계수가 별도 안정성 문제
  - sf1 자극(C2, S2, GA2, GB2)이 별도 분기점 활성화
- FAIL 시 즉시 멈추고 보고서에 기록. 추가 사이클 또는 escape hatch 4 발동.

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`

- [ ] **Step 4: 커밋**

```bash
git add internal/decoder/frame0_regression_test.go
git commit -m "$(cat <<'EOF'
test(decoder): Stage V1 frame 0 80-sample bit-exact regression guard

Promotes the Stage F-tris sf0 sample-40 guard to full-frame (samples
0..79) coverage. Phase 1i sample-0 lock + Stage F-tris sf0 fix + sf1
combine to make this guard unconditional.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

### Task 5 (V2): ALGTHM skip 메시지 갱신

**Files:**
- Modify: `internal/decoder/decode_test.go`

**Why:** Stage F 플랜 Task 5와 동일.

- [ ] **Step 1: 스킵 메시지 갱신**

`internal/decoder/decode_test.go`의 `TestDecode_ITUVectorAlgthmBitExact` t.Skip 메시지를 다음으로 변경:

```go
	t.Skip("Phase 1k Stage F-tris: frame 0 bit-exact via Frame0AllSamples regression " +
		"guard. Frames 1-34 require pastExc/pastSynth/agcGainPrev state evolution + " +
		"multi-frame diagnostics; deferred to Phase 1l (subagent vectors SPEECH/FIXED) " +
		"or Phase 1m (multi-frame ALGTHM).")
```

- [ ] **Step 2: 실행**

Run: `go test -v -run TestDecode_ITUVectorAlgthmBitExact ./internal/decoder/`
Expected: SKIP, 메시지 갱신 확인.

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`

- [ ] **Step 4: 커밋**

```bash
git add internal/decoder/decode_test.go
git commit -m "$(cat <<'EOF'
test(decoder): V2 update ALGTHM skip message — frame 0 done, 1-34 deferred

Stage F-tris achieved frame 0 bit-exact (covered by Frame0AllSamples).
The full-vector test stays skipped because frames 1-34 require
multi-frame state evolution diagnostics, deferred to Phase 1l/1m.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

### Task 6 (V3): 병리 케이스 A+B 하이브리드 재인증

**Files:**
- Modify: `internal/gain/pathological_test.go`

**Why:** Stage F 플랜 Task 6와 동일. F-tris-3가 lsp + 상류 단계를 변경했으므로 gain 모듈 4개 병리 테스트 영향 없음 확인.

A 전략: spec-유도 가능(`AllZeroCodebookIsBounded`, `LowEnergyCodebookIsSmooth`)는 어서션 유지.
B 전략: spec-유도 어려운 경계(`HighEnergyCodebookIsBounded`, `SucceedsAcrossAllGainIndices`)는 측정값 갱신 + §A.4.2 인용 주석.

- [ ] **Step 1: 4개 병리 테스트 회귀 확인**

Run: `go test -v -run TestPathological ./internal/gain/`

예상: 모두 PASS (lsp + postfilter 변경은 gain 모듈 무관).

- [ ] **Step 2: B 전략 — 측정값 재산출 (필요 시)**

만약 1개 이상 FAIL이면 임계값 갱신 + 다음 주석:
```go
// Updated post-Stage F-tris (Phase 1k §3.2.6 fix + §<X.X> <상류> fix):
// empirical boundary reflects post-fix gain VQ behavior. Spec §A.4.2
// only guarantees gpQ14 ∈ [0, 3·Q14], gcQ12 ∈ [0, +Q15]; the tighter
// bound here is empirical regression-guard, not a spec claim.
```

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`

- [ ] **Step 4: 커밋 (변경 시만)**

```bash
git add internal/gain/pathological_test.go
git commit -m "$(cat <<'EOF'
test(gain): V3 pathological re-cert post-Stage F-tris (empirical bounds refresh)

Stage F-tris lsp + <상류> fix did not affect gain VQ behavior; A+B
hybrid strategy: AllZero/LowEnergy assertions unchanged (spec-derived);
HighEnergy/SucceedsAcross thresholds refreshed empirically with
spec §A.4.2 caveat comment.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

### Task 7: 완료 보고서

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-tris-completion-report.md`

**Why:** Phase 1k 누적(Stage D + D-bis + F-prep + F-partial + F-bis-1 + F-bis-2 + F-tris + V) 결과 + F-bis 플랜 §3.10 인용 오류 정정 명시 + 상류 결함 식별 결과를 한 문서에 정리.

- [ ] **Step 1: 보고서 작성**

```markdown
# Phase 1k Stage F-tris 완료 보고서

**작성일**: 2026-04-XX
**범위**: Phase 1k 전체 (Stage D + D-bis + F-prep + F-partial + F-bis-1 + F-bis-2 + F-tris + V) 누적 결과.
**핵심 결론**: ALGTHM frame 0 80 샘플 비트-정확 달성. Stage F partial이 노출하고 F-bis-2가 정정한 두 결함 (§3.2.6 LSP→LP saturation + §<X.X> <상류> 진폭 결함) 을 단일 커밋으로 동시 수정. `pcm.ScaleUpSat`는 §4.2.5 ×2 spec-correct로 *변경 없음*.

---

## 0. F-bis 플랜 인용 오류 정정 (서론)

본 보고서는 다음 F-bis 플랜의 작성 오류를 명시 정정한다:
- F-bis 플랜 (`2026-04-27-phase1k-stage-f-bis-plan.md` line 49-71)의 *§3.10 "The output speech is finally divided by 2 with saturation control"* 인용은 spec에 존재하지 않는 문장.
- 실제 §3.10는 "Memory update" — 합성/가중 필터의 메모리 갱신 절차로 출력 스케일링과 무관.
- 출력 스케일링 규정은 **§4.2.5 / §A.4.2.5** "**multiplied by a factor 2** to restore the input signal level" 단일 조항.
- F-bis-1 보고서 §3.2 "결함 위치 = pcm.ScaleUpSat" 결론은 *경계 식별*은 정확하나 *방향 라벨링*이 잘못. 실제 결함은 ScaleUpSat의 *상류*.

본 정정 사실은 F-bis-2 보고서 §1.1-1.5에서 확립.

---

## 1. 누적 커밋

| Stage | # | SHA | 메시지 |
|-------|---|-----|--------|
| D | 1 | c275a12 | test(decoder): split frame 0 regression guard + sf1 diagnostic log |
| D | 2 | 4e7b254 | test(fcb): Q-format contract |
| D | 3 | 520019e | test(gain): Q-format contract |
| D | 4 | cd12df4 | test(synth): Q-format contract |
| D | 5 | f312865 | test(postfilter): Q-format contract |
| D | 6 | f4f3bd2 | test(decoder): single-pulse diagnostic harness |
| D | 7 | 9c33178 | test(decoder): single-pulse harness assertions |
| D-bis | 1 | a36a335 | test(decoder): D-bis-1 4-pulse canonical |
| D-bis | 2 | 1a983c0 | test(decoder): D-bis-1 4-pulse assertions |
| D-bis | 3 | 4854bd6 | test(decoder): D-bis-2 pitch-active |
| D-bis | 4 | daa9fcd | test(decoder): D-bis-3 ALGTHM replay |
| F-prep | 1 | <sha> | test(lsp): F-prep-1 A(z) stability |
| F-prep | 2 | <sha> | test(synth): F-prep-2 closed-form + saturation |
| F-tris | 3 | <sha> | fix(lsp+<상류>): §3.2.6 exact + §<X.X> <상류> spec-correct |
| V | 1 | <sha> | test(decoder): V1 frame 0 80-sample guard |
| V | 2 | <sha> | test(decoder): V2 ALGTHM skip message |
| V | 3 | <sha> | test(gain): V3 pathological re-cert (변경 시) |

회귀 게이트: 마지막 커밋 시점 `go test -race ./...` ALL PASS, `go vet ./...` silent, `internal/fixed` 0 allocs/op.

---

## 2. F-tris-1 PST/2 도메인 진단 결과

진단 하니스: `internal/decoder/stagef_bis_diagnostic_test.go::TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace`

P fix만 적용한 working tree에서 측정한 sf0 sample 0..39 (PST/2 도메인 비교):

| 경계 | match count vs PST/2 (\|Δ\|≤1 LSB) | 해석 |
|------|---:|------|
| `synth.Filter` 직후 | <N₁> / 40 | <synth 단계 평가> |
| `postfilter.Filter` 직후 | <N₂> / 40 | <postfilter 평가> |
| `hpFilter` 직후 | <N₃> / 40 | <hpFilter 평가> |

판정: **결함 진입 단계 = <식별된 단계>**. 시나리오 = <(가)/(나)/(다)/(라)>.

---

## 3. F-tris-2 spec-인용 분석

결함 위치: `<파일경로:라인>`
spec § 인용: ITU-T G.729 §<X.X>
> "<직접 인용>"

이전 거동(어긋남):
```
<production 코드 라인>
```

수정 후 거동(spec-correct):
```
<수정된 코드 라인>
```

sample 0..3 영향 hand-calc: <표>

(해당 시 추가 결함:)
결함 위치 2: `<파일경로:라인>`
spec § 인용: ITU-T G.729 §<X.X>
...

---

## 4. 결합 fix가 sample 0 잠금을 자연 회복하는 메커니즘

§3.2.6 결함만 수정 (P fix 단독): a[] 정확 → 상류 진폭 2× 결함 노출 → ScaleUpSat ×2 후 sample 0 = 4 (want=2).

§<X.X> 결함만 수정 (상류 fix 단독): a[] 비대칭 → 14 dB sf0 발산 (Stage D-bis 측정).

§3.2.6 + §<X.X> 동시 수정: a[] 정확 + 상류 진폭 spec-correct → ScaleUpSat ×2 후 sample 0 = 2 (want=2). Phase 1i 잠금 자연 회복.

이는 Phase 1j 가설("두 14 dB 오차가 sample 0에서 상쇄")의 정량적 확증 — 한 결함이 다른 결함의 잘못된 진폭을 우연히 정정하던 형태. F-bis-2 §1.8에서 이 메커니즘 확립.

---

## 5. 검증 결과

- ALGTHM frame 0 80 샘플 비트-정확: ✅ (`TestDecode_Frame0AllSamples_MatchesALGTHM`)
- Phase 1i sample 0 잠금 자연 회복 (got=2 want=2): ✅
- A(z) minimum-phase (§3.2.6 spec 준수): ✅ (`TestALGTHMFrame0SF0_AzStability`)
- Stage D 17개 컨트랙트 어서션: ✅ 무회귀
- Stage D-bis 3개 어서션: ✅ 무회귀
- F-prep 신규 어서션 2개: ✅ PASS
- 병리 케이스 4개: ✅ (변경 N개)
- 0 allocs/op `internal/fixed` 벤치: ✅

---

## 6. 영구 가드

- `TestDecode_Frame0Sample0_MatchesALGTHM` (Phase 1i)
- `TestDecode_Frame0SF0AllSamples_MatchesALGTHM` (Stage F-tris)
- `TestDecode_Frame0AllSamples_MatchesALGTHM` (Stage V1)
- `TestALGTHMFrame0SF0_AzStability` (lsp F-prep-1)
- `TestFilter_ImpulseResponse_OnePoleClosedForm` (synth F-prep-2)
- `TestFilter_SaturationRecovery_ScalingFactorMatchesSpec` (synth F-prep-2)
- `TestDiagnostic_FbisStageBoundaries_Sample0Trace` (F-bis-1, t.Logf 보존 시)
- `TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace` (F-tris-1, t.Logf 보존 시)
- Phase 1k Stage D 17개 컨트랙트 + D-bis 3개 = 20개

총 신규 영구 가드 ≈ 28개 (Phase 1k 누적).

---

## 7. 다음 단계 후보

- **Phase 1l**: SPEECH/FIXED ITU 벡터 활성화. ALGTHM frames 1-34는 multi-frame state 진단 필요.
- **Phase 1m**: ALGTHM frames 1-34 비트-정확 — pastExc/pastSynth/MA predictor/agcGainPrev 다중 프레임 진화 진단.
- **Phase 1n**: 인코더 시작.

---

## 8. 탈출 해치 발동 평가 (전 사이클 누적)

| 해치 | 발동 시점 | 결과 |
|------|----------|------|
| 1 (sample 0 회귀) | F-partial | F-bis-1로 흡수 |
| 1 (production = spec ScaleUpSat) | **F-bis-2** | **F-tris로 흡수** |
| 2 (다른 Phase 1i fix 회귀) | 미발동 | — |
| 3 (Stage D + D-bis 어서션 회귀) | 미발동 | — |
| 4 (Stage 전체 비결정) | 미발동 | F-tris 단일 커밋으로 80-sample 달성 |

본 phase의 두 escape hatch 발동은 모두 강압-적합 방지에 성공 — production을 spec과 다르게 임의 변경하지 않고, 분석을 글로 명시한 후에만 진입.
```

- [ ] **Step 2: 보고서 커밋**

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-tris-completion-report.md
git commit -m "$(cat <<'EOF'
docs(plans): Phase 1k Stage F-tris completion report — ALGTHM frame 0 80-sample bit-exact

Combined-fix landing at §3.2.6 + §<X.X> <상류> identified by
F-tris-1 PST/2-domain stage trace and F-tris-2 spec-citation
analysis. Two defects mutually cancelled at Phase 1i sample 0;
both spec-correct fixes land in a single commit. pcm.ScaleUpSat
remains spec-correct per §4.2.5/§A.4.2.5. ~28 permanent regression
guards accumulated. Phase 1l/1m entry conditions met.

Documents F-bis plan §3.10 citation error correction.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

- [ ] **Step 3: 최종 회귀 스윕**

Run:
```bash
go test -race ./...
go vet ./...
go test -bench=. -benchmem -run=^$ ./internal/fixed/
```

Expected: ALL PASS, vet silent, 0 allocs/op for `Add`/`LMult`/`LMac`/`DivS`/`NormL`.

---

## Self-Review Checklist (플랜 작성자용)

- **Spec coverage:** F-bis-2 §2.4의 후보 3개 (postfilter AGC seed, synth saturation recovery, synth Q-format) 가 본 플랜에서 각각 F-tris-2 시나리오 (나)/(다)/(다)에 매핑. F-bis-2 §3 권고 4개 항목 (인용 정정, 결론 재라벨링, 새 진단 사이클, PST/2 비교 기준) 이 본 플랜의 서론 + Task 1 + 완료 보고서 §0에 모두 반영.
- **Placeholder scan:** F-tris-3 Step 1/5와 완료 보고서 §1/§2/§3에서 `<F-tris-2 식별 단계 파일>`, `<X.X>`, `<상류>` placeholder 사용 — F-tris-1/2 결과로 채워질 부분만. 그 외 모든 코드 블록 완성.
- **Type consistency:** `lsp.Decoder.Decode`, `synth.Synthesizer.Filter`, `pst.Filter`, `hpFilter`, `pcm.ScaleUpSat`, `Postfilter.applyAGC` 시그니처는 기존 파일과 일치 가정 (Task 1 Step 2에 시그니처 점검 가드레일 명시).
- **Escape hatch alignment:** 4개 hatch가 본 플랜에서도 작동:
  - F-tris-1 Step 5: 진단 종료 시 RED 미커밋
  - F-tris-2 Step 4 escape hatch: production이 spec과 일치하면 ScaleUpSat 사례 재발 — 즉시 멈춤
  - F-tris-3 Step 4: sample 0 잠금 미회복 시 커밋 금지 + 롤백
  - F-tris-3 Step 4: sf0 sample 1~39 FAIL 시 즉시 멈춤
- **Scratch-from-spec:** 모든 인용이 ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) 의 §3.x / §4.x / §A.4.x 범위 내. 외부 참조 코드 0건.
- **No force-fit:** F-tris-2가 *분석 노트* 단계를 별도 태스크로 분리한 이유는 강압-적합 금지 — F-tris-1의 stage 식별만으로 임의 수정 금지, spec § 인용 ⇒ production 어긋남 ⇒ hand-calc로 fix가 PST/2 정렬을 예측 ⇒ 수정안 사슬을 글로 명시한 후에만 F-tris-3 진입.
- **Citation-error guard:** F-tris-2 Step 3 hand-calc 단계가 강압-적합 + 인용 오류 둘 다에 대한 방어선. spec § 인용이 잘못이면 hand-calc 결과가 PST/2 정렬을 예측 못함 → 즉시 멈춤.

---

## Execution Handoff

**1. Subagent-Driven (recommended)** — 태스크별 새 서브에이전트 디스패치, F-tris-1 결과 보고 후 사용자가 F-tris-2 진입 승인. F-tris-2 분석 노트 종료 후 사용자가 F-tris-3 단일 커밋 승인.

**2. Inline Execution** — `superpowers:executing-plans`로 배치 실행 + F-tris-3 직전 체크포인트.

작성자가 사용자에게 두 옵션 중 하나 선택 요청. **F-tris-1 / F-tris-2 / F-tris-3 사이의 모든 분기 결정은 사용자 결정 권고** (Phase 1j 강압-적합 재발 방지, F-bis-2가 escape hatch를 모범적으로 발동한 사례를 본 플랜에서도 보존).
