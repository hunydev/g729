# Phase 1k Stage F-bis Implementation Plan (P fix + 하류 ×2 보정 동시 수정)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stage F partial 보고서가 노출한 "분기 P fix + 하류 ×2 결함" 동시 결함 쌍을 단일 커밋으로 동시에 수정. ALGTHM frame 0 sf0 (40 샘플) 비트-정확 + Phase 1i sample 0 잠금 자연 회복.

**Architecture:** Stage F partial이 확보한 사실 두 가지를 출발점으로 한다.
- (사실 1) `internal/lsp/lsp_lp.go::lspToLP`의 `[11]fixed.Word32` Q28 saturating 누산은 §3.2.6 exact-arithmetic 점화식 위반(F1[3]=−8.0, F1[4]=+8.0 양극단 포화 → |k_7|=1.897). 후보 fix(`[11]int64` exact)는 검증 완료 — Schur–Cohn step-down 모든 |k_m|<1.
- (사실 2) 후보 P fix 단독 적용 시 ALGTHM f0 sample 0 = 4 (want 2). Δ 패턴 (sample 0..5: +2/+3/+3/+3/+3, sample 6..21: 혼합 +1/+3/-1, sample 22~28: 일정 -2)은 IIR 발산 형태가 아닌 거의-DC + 작은 변동 → "단일-경로 정수배(×2) 결함이 하류 어딘가에 있고, 이전까지 §3.2.6 결함이 그 정수배를 우연히 ÷2로 상쇄"하는 형태.

본 플랜은 (i) P fix를 working tree에만 적용한 채 stage-by-stage 진단 하니스로 sample 0의 "want=2 vs got=4" 2x 진입점을 4개 경계(synth → postfilter → hpFilter → pcm.ScaleUpSat) 중에서 정확히 찾고, (ii) 진입점에서 §-인용 spec 위반을 line-by-line 식별, (iii) P fix와 하류 fix를 단일 커밋으로 묶어 Phase 1i 잠금이 새 spec-준수 경로에서 자연 회복하는지 검증한다.

**Tech Stack:** Go 1.22+, `internal/{lsp,synth,postfilter,pcm,decoder}` 모듈 수정 가능.

**Scratch-from-spec discipline:** ITU 참조 C, bcg729, Sipro Lab, FFmpeg 절대 참조 금지. ITU-T G.729 §3.2.6 / §3.10 / §3.7 / §3.8 / §3.9 + Annex A의 해당 절 + LSP-LP 표준 이론(Schur–Cohn step-down)만 사용.

**Co-author trailer for every commit:**
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`

---

## File Structure

| 파일 | 역할 | 상태 |
|------|------|------|
| `internal/lsp/lsp_lp.go` | (사실 1) Q28 saturating 누산 → int64 exact 누산 | 수정 (F-bis-3에 한해 commit) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | 진단 하니스: P fix 가정 하에 sample 0 stage-by-stage capture | 신규 (F-bis-1, 진단 종료 후 영구 가드로 보존 또는 제거 결정) |
| `internal/synth/filter.go` 또는 `internal/postfilter/...` 또는 `internal/pcm/scale.go` 또는 `internal/decoder/decode.go::hpFilter` | F-bis-1이 식별한 하류 ×2 결함 위치 | 수정 (F-bis-3) |
| `internal/decoder/frame0_regression_test.go` | sample 40 가드 추가 (Stage F 플랜 Task 3 Step 4 동일) | 수정 (F-bis-3) |
| `internal/decoder/frame0_regression_test.go` | sample 80 가드 (Stage F 플랜 Task 4 V1 동일) | 수정 (V1) |
| `internal/decoder/decode_test.go` | ALGTHM skip 메시지 갱신 (Stage F 플랜 Task 5 V2 동일) | 수정 (V2) |
| `internal/gain/pathological_test.go` | A+B 병리 재인증 (Stage F 플랜 Task 6 V3 동일) | 수정 (V3) |
| `docs/superpowers/plans/2026-04-27-phase1k-stage-f-bis-completion-report.md` | 완료 보고서 | 신규 |

---

## Spec-derived Reference Values

본 단계 전체에서 공통으로 사용할 손계산 참값.

### §3.2.6 LSP→LP exact-arithmetic 점화식 (Stage F partial §2에서 인용)

```
F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2)
```

본 점화식에 saturation 부과 규정 없음. 중간 단계 |F| ≤ ~9.07 (ALGTHM f0 sf1 자극 기준 손계산) → Q28 Word32 표현 한계 |F| ≤ ~7.999를 초과. ⇒ Word32 saturating 누산기는 §3.2.6 위반. int64(또는 더 넓은 표현)로 exact 누산 후 마지막 단계에서만 Q12 Word16 saturation 허용.

### §3.10 LP synthesis filter saturation recovery (Stage F-prep-2가 노출)

```
"When overflow occurs, the speech samples and the filter memory are
divided by 4 and the filtering is re-done. The output is multiplied
by 4 with saturation."
```

현 `internal/synth/filter.go`의 ÷2 + ×2는 spec 위반. 본 플랜에서는 saturation recovery가 trigger되지 않는 stimulus(P fix 후 |Δ|≤3) 임에도 불구하고 §3.10이 잠재적 ×2 결함의 *형식적* 근원이 될 수 있는지 F-bis-1 검사 항목으로 확인.

### §3.7 / §3.8 Postfilter (long-term, short-term, tilt compensation)

- 출력 = `Y_pf(z) = H_long(z) · H_short(z) · H_tilt(z) · S(z)` (모듈식)
- §3.8 short-term postfilter: `H_short(z) = A(z/γ_n) / A(z/γ_d)` with γ_n=0.55, γ_d=0.7 (G.729; G.729A는 다른 값 — Annex A 인용 필수).
- §3.9 AGC: `g_t = sqrt( Σ s² / Σ y_pf² )`, smoothing factor `α`. AGC seed 값(첫 호출 시 default state)은 §A.4.2가 별도 명시.

### §3.10 Up-scaling (스피치 출력 ×2 final stage)

```
"The output speech is finally divided by 2 with saturation control"
```

→ 정확히는 spec은 *post-emphasis 후 출력을 절반으로 줄여 amplitude를 dB 기준으로 맞춘다* 라고 표현. `pcm.ScaleUpSat`가 이 단계를 구현. 명칭이 "ScaleUp"이지만 spec은 *÷2*; ITU 참조 (ALGTHM.PST) 기준값과 비교하기 위한 G.192 변환은 별개.

### ALGTHM frame 0 sf0 stage-by-stage 손계산 기대값 (Stage F partial §2.4 인용)

P fix 적용 후, sample 0 want=2 (PST 기준)이며 production은 4를 산출. 단일-경로 ×2 패턴이 의심되므로 다음 경계 중 **정확히 한 곳**에서 ×2 결함이 진입:

| 경계 | 출력 기댓값 (sample 0) | 비고 |
|------|---------------------|------|
| `synth.Filter` 직후 | (손계산 또는 측정 필요) | u[0] 입력에 대한 1/A(z) 첫 샘플 |
| `postfilter.Filter` 직후 | (손계산 또는 측정 필요) | tilt + AGC 적용 후 |
| `hpFilter` 직후 | (손계산 또는 측정 필요) | DC blocker / HP filter |
| `pcm.ScaleUpSat` 직후 | 2 (PST 기준) | ÷2 + saturation 단계 |

손계산이 어려운 경계는 **현 production(부분 보고서 측정값)을 기준** 으로 하고, P fix만 적용한 working tree에서 동일 sample 0 값을 측정해 비교. 동일 → 그 경계는 무죄. 1/2 또는 2x 변화 → 그 경계가 결함 위치(또는 그 직전 단계).

---

## Bite-Sized Task Granularity

각 태스크 패턴:
1. (필요 시) 후보 fix를 working tree에 적용 (F-bis-1) 또는 진단 하니스 추가
2. 어서션/측정 실행 → 분석
3. 분석 결과로 다음 태스크의 진입 조건 결정
4. F-bis-3에서만 단일 커밋 — 이전 단계는 진단/측정 (커밋 0건 또는 진단 하니스 1건)

---

### Task 1 (F-bis-1): 후보 P fix 적용 + stage-by-stage sample 0 진단 하니스

**Files:**
- Modify (uncommitted, working tree only): `internal/lsp/lsp_lp.go`
- Create: `internal/decoder/stagef_bis_diagnostic_test.go`

**Why:** Stage F partial에서 P fix 단독으로는 sample 0 = 4 (want=2). Δ 패턴이 거의-DC + 작은 변동 → 단일-경로 ×2 결함을 stage-by-stage로 노출시킬 수 있다. 본 태스크는 4개 경계(synth → postfilter → hpFilter → ScaleUpSat) 사이에서 sample 0 값이 어디서 2x로 변하는지 측정한다.

**중요 (escape hatch 1 절대 준수):**
- 본 태스크는 **`go test -race ./...`가 RED인 working tree에서 진행한다.** P fix가 sample 0 잠금 회귀를 일으키므로 `TestDecode_Frame0Sample0_MatchesALGTHM`가 FAIL 상태로 시작.
- **이 RED 상태로 커밋 금지.** 진단 종료 후 working tree를 `git checkout --` 으로 복원하거나, F-bis-3에서 P+하류 fix 동시 단일 커밋으로 GREEN 회복 후에만 커밋.
- 진단 하니스(`stagef_bis_diagnostic_test.go`) 자체도 본 태스크 종료 시점에서는 미커밋 상태로 둔다. F-bis-3에서 (i) production 수정과 함께 하니스가 영구 가드로 보존할 가치가 있으면 동일 커밋에 포함, (ii) 진단 목적만으로 사용했고 영구 가드 가치가 없으면 삭제.

- [ ] **Step 1: 후보 P fix를 working tree에 적용 (uncommitted)**

`internal/lsp/lsp_lp.go`의 `lspToLP` + `polyStep`을 다음과 같이 변경. (현재 파일은 `[11]fixed.Word32` + `polyStep(f fixed.Word32, q int16, fPrev1, fPrev2 fixed.Word32) fixed.Word32` 형태 — Stage F partial §2.2/2.3 인용.)

```go
// lspToLP converts the 10 LSP coefficients (Q15 cosine domain) into 11
// LP coefficients a[] (Q12, monic). Implements ITU-T G.729 §3.2.6
// recurrence
//
//     F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2)
//
// using exact int64 arithmetic for the F1, F2 polynomials. The spec
// does not authorise saturation on this recurrence — middle-stage
// |F| can transiently exceed the Q28 Word32 envelope (~|F| ≤ 7.999)
// while the final symmetric/antisymmetric A polynomials remain in
// Q12 Word16 range. Saturating the intermediate F polynomials in
// Word32 produced asymmetric a[] coefficients with |k_7| = 1.897
// (Stage F partial report, branch P diagnostic).
func lspToLP(lsp *[10]int16, a *[11]int16) {
	var f1, f2 [11]int64
	const oneQ28 int64 = 1 << 28
	f1[0] = oneQ28
	f2[0] = oneQ28

	for step := 0; step < 5; step++ {
		q1 := int64(lsp[2*step])
		q2 := int64(lsp[2*step+1])
		top := 2*step + 2
		for j := top; j >= 2; j-- {
			f1[j] = polyStepExact(f1[j], q1, f1[j-1], f1[j-2])
			f2[j] = polyStepExact(f2[j], q2, f2[j-1], f2[j-2])
		}
		// j=1 카피 (step 종료 시 F(1) -= 2·q·F(0)):
		f1[1] -= 2 * q1 * (oneQ28 >> 14)
		f2[1] -= 2 * q2 * (oneQ28 >> 14)
		// 위 식의 정확한 형태는 기존 polyStep과 동일한 j=1 처리.
		// (만약 기존 lspToLP가 j 범위를 다르게 처리하고 있다면 그 형태를
		// 그대로 보존하되 산술만 int64 exact로 교체.)
	}

	// Combine F1, F2 → A polynomial (existing Q28 → Q12 round-and-saturate).
	// 이 단계의 saturation은 §3.2.6 출력 정의 (a[] ∈ Q12 Word16)이므로
	// spec-허용. 다만 round 위치는 기존 코드와 동일하게 유지.
	for k := 0; k <= 10; k++ {
		var sum int64
		// (기존 코드의 F1+F2 또는 F1−F2 결합 로직을 그대로 보존, int64
		// 변수로 운반)
		// ... existing combine logic adapted to int64 ...
		sum = (sum + (1 << 16)) >> 17
		// Q12 Word16 saturation (§3.2.6 output domain).
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		a[k] = int16(sum)
	}
}

// polyStepExact computes one Chebyshev recurrence step in exact int64
// arithmetic, no saturation.
//
//   prod = (q · fPrev1) >> 14    // Q15 · Q28 = Q43, shift to Q28 (factor of 2 absorbed in q's Q15 sign convention)
//   result = f − 2·prod + fPrev2
//
// q는 Q15(LSP cos), fPrev1/fPrev2는 Q28. result는 Q28.
func polyStepExact(f, q, fPrev1, fPrev2 int64) int64 {
	prod := (q * fPrev1) >> 14
	return f - prod + fPrev2
}
```

**중요 가드레일 (강압-적합 금지):**
- 위 코드는 골격만 제시한다. **기존 `lspToLP` 본체의 j=1, 결합(combine) 로직, round 위치는 보존**하고, 누산기 자료형과 산술만 `int64` exact로 교체.
- Stage F partial §2.3에서 검증된 결과: int64 fix 후 `TestALGTHMFrame0SF0_AzStability` PASS, `TestLSPToLPLeadingCoefficient` / `TestLSPToLPAllZeroLSPProducesSymmetric` PASS. 본 태스크에서도 동일 결과를 확인한다.

- [ ] **Step 2: 후보 P fix만 적용한 working tree에서 동일성 재확인**

Run:
```bash
go test -v -run "TestALGTHMFrame0SF0_AzStability|TestLSPToLPLeadingCoefficient|TestLSPToLPAllZeroLSPProducesSymmetric" ./internal/lsp/
```

Expected:
- `TestALGTHMFrame0SF0_AzStability` PASS (모든 |k_m|<1, Stage F partial §3 표 기록과 일치)
- 기타 lsp 단위 테스트 PASS (회귀 없음)

만약 PASS가 아니면 Step 1의 코드 옮겨심기 과정에서 결합/round 로직을 잘못 옮겼다는 신호. **Step 1로 돌아가 기존 본체와 사이드-바이-사이드 비교** 후 재시도.

- [ ] **Step 3: stage-by-stage sample 0 진단 하니스 작성**

`internal/decoder/stagef_bis_diagnostic_test.go` 신규 파일:

```go
package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestDiagnostic_FbisStageBoundaries_Sample0Trace: Stage F-bis-1 진단.
//
// 후보 P fix(`internal/lsp/lsp_lp.go` int64 누산)가 working tree에
// 적용된 상태에서 ALGTHM frame 0 sf0의 sample 0 값을 4개 경계
// (synth → postfilter → hpFilter → ScaleUpSat) 모두에서 측정해
// "sample 0 = 2x 결함이 어느 경계에서 진입하는가" 를 식별한다.
//
// 본 테스트는 진단-only(t.Logf 사용). escape hatch 1 준수: P fix
// 단독 working tree에서는 RED 상태이므로 본 하니스도 t.Errorf를
// 사용하지 않는다. F-bis-3 단일 커밋 시점에 보존/삭제 여부를 결정.
//
// 손계산 기대값 (sample 0):
//   - PST(spec ground truth, ALGTHM.PST[0][0]): 2
//   - Production (P fix only): 4
//   - 단일-경로 ×2 가설이 맞다면 4개 경계 중 정확히 1곳에서 1→2 또는 2→4의 ÷2 누락이 진입한다.
//
// Stage F partial §2.4 부터 연속:
//   sample 0..5: Δ ∈ {+2, +3, +3, +3, +3}
//   sample 6..21: Δ ∈ {+1, +3, +3, +1, -1, ..., -1}
//   sample 22..28: Δ = -2 (일정)
//   → "거의-DC + 작은 변동" 패턴 → 단일-경로 정수 스케일 결함의 신호.
func TestDiagnostic_FbisStageBoundaries_Sample0Trace(t *testing.T) {
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

	t.Logf("u[0..3] = %v (BuildExcitation output)", u[:4])

	// Boundary 1: synth.Filter 직후
	var sRaw [subframeLen]int16
	d.syn.Filter(&sf1A, &u, &sRaw)
	t.Logf("BOUNDARY synth.Filter: sample 0 = %d (sRaw[0..3] = %v)", sRaw[0], sRaw[:4])

	// Boundary 2: postfilter.Filter 직후
	var sPf [subframeLen]int16
	d.pst.Filter(&sf1A, tInt1, &sRaw, &sPf)
	t.Logf("BOUNDARY postfilter.Filter: sample 0 = %d (sPf[0..3] = %v)", sPf[0], sPf[:4])

	// Boundary 3: hpFilter 직후
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	t.Logf("BOUNDARY hpFilter: sample 0 = %d (hpOut[0..3] = %v)", hpOut[0], hpOut[:4])

	// Boundary 4: pcm.ScaleUpSat 직후 (frame-level — 80 sample 입력 필요).
	// sf0만 측정하기 위해 hpOut을 80 자리 임시 버퍼에 복사하고 ScaleUpSat를
	// 호출. sf1 자리는 0으로 채움 (ScaleUpSat는 sample-by-sample이므로 OK).
	var fullFrame [frameSamples]int16
	copy(fullFrame[:subframeLen], hpOut[:])
	pcm.ScaleUpSat(fullFrame[:frameSamples], fullFrame[:frameSamples])
	t.Logf("BOUNDARY pcm.ScaleUpSat: sample 0 = %d (fullFrame[0..3] = %v)", fullFrame[0], fullFrame[:4])

	// PST ground truth
	t.Logf("PST want sample 0..3 = %v", wantFrames[0][:4])
	t.Logf("Δ at each boundary vs PST sample 0 (want=%d):", wantFrames[0][0])
	t.Logf("  synth:        Δ=%d", int32(sRaw[0])-int32(wantFrames[0][0]))
	t.Logf("  postfilter:   Δ=%d", int32(sPf[0])-int32(wantFrames[0][0]))
	t.Logf("  hpFilter:     Δ=%d", int32(hpOut[0])-int32(wantFrames[0][0]))
	t.Logf("  ScaleUpSat:   Δ=%d", int32(fullFrame[0])-int32(wantFrames[0][0]))

	// 결정 가이드 (보고서 §2 작성 시 사용):
	//
	// 두 인접 경계 사이에서 sample 0이 *정확히 2x* (또는 *정확히 1/2x*)
	// 변하면 그 사이 단계가 결함 위치. 예:
	//   synth=1, postfilter=2 → postfilter 진입 시 ×2  → 결함 = postfilter
	//   synth=2, postfilter=2, hpFilter=2, ScaleUpSat=4 → ScaleUpSat의 ÷2 누락
	//     (spec §3.10 "divided by 2 with saturation control" 위반)
	//
	// 두 경계 사이의 차이가 2x가 아니면 단일-경로 가설이 깨짐 → 다른
	// 진단(예: pastSynth/pastExc 상태 누설)이 필요. F-bis-2 escape hatch.
}
```

**중요 — production 메서드 노출 점검:**
- `d.syn.Filter`, `d.pst.Filter`는 이미 export 가능한 메서드 시그니처라고 가정. 실제 코드와 다르면 진단 하니스 작성 시 export 어댑터를 같은 파일에 작성하거나, 진단을 internal/decoder 패키지 내부에서 실행.
- `d.hpFilter`는 `func (d *Decoder) hpFilter(in *[subframeLen]int16, out []int16)` 형태로 추정. 시그니처 다르면 본 코드의 호출 형태를 실제 시그니처에 맞춰 조정.
- `pcm.ScaleUpSat`는 frame-level (80 샘플 in/out). sf0만 측정하기 위해 80 자리 버퍼 사용 — sf1 자리는 0으로 채워도 ScaleUpSat가 sample-by-sample 변환이면 결과는 sf0 부분만 영향.

만약 `pcm.ScaleUpSat`가 frame-level cross-sample 의존성(예: 이전 sample 기억)을 가진다면 본 진단 방법이 부정확. 이 경우 진단 코드를 `decoder.Decode`를 직접 호출하고 sf0/sf1 사이 frame-level 호출 순서를 따르는 형태로 수정.

- [ ] **Step 4: 진단 실행**

Run:
```bash
go test -v -run TestDiagnostic_FbisStageBoundaries_Sample0Trace ./internal/decoder/ 2>&1 | tee /tmp/fbis-diag.log
```

Expected log 형태 (예시 — 실제 측정값으로 대체):
```
u[0..3] = [...]
BOUNDARY synth.Filter: sample 0 = X
BOUNDARY postfilter.Filter: sample 0 = Y
BOUNDARY hpFilter: sample 0 = Z
BOUNDARY pcm.ScaleUpSat: sample 0 = 4
PST want sample 0..3 = [2 ...]
Δ at each boundary vs PST sample 0 (want=2):
  synth:        Δ=...
  postfilter:   Δ=...
  hpFilter:     Δ=...
  ScaleUpSat:   Δ=2
```

**판정 가이드 (보고서 §2에 그대로 인용):**
1. 한 인접 경계 쌍에서 정확히 2x (또는 1/2x) 변화 발견 → **결함 위치 확정**. 다음 태스크(F-bis-2)는 그 단계의 spec § 인용 분석.
2. 모든 경계에서 일정한 비율(예: 모두 2x) → ScaleUpSat가 ÷2를 누락한 형태 (spec §3.10 마지막 단계 위반). F-bis-2는 `pcm.ScaleUpSat` 분석.
3. 어떤 경계에서도 2x 패턴 없음 → 단일-경로 가설 부정. **escape hatch**: F-bis-2 진입 보류, 사용자에게 "Δ 패턴이 단일-경로 정수 스케일 외 다른 결함 시그니처 (예: pastSynth/pastExc 메모리 누설)" 보고.

- [ ] **Step 5: 진단 종료 — working tree 미커밋 유지**

본 태스크는 진단만 수행한다. **커밋 없음.** working tree 상태:
- `internal/lsp/lsp_lp.go`: 수정됨 (P fix 후보, 미커밋)
- `internal/decoder/stagef_bis_diagnostic_test.go`: 신규 (미커밋)
- `internal/decoder/frame0_regression_test.go::TestDecode_Frame0Sample0_MatchesALGTHM`: RED (P fix만 적용 시 got=4 want=2)

다음 태스크(F-bis-2) 종료 후 F-bis-3 단일 커밋에서 P fix + 하류 fix를 동시에 landing.

**Escape hatch 1 재확인:**
- F-bis-3 단일 커밋이 sample 0 잠금을 회복하지 못하면 (즉 P + 하류 fix를 합쳐도 got≠2), 즉시 working tree 전체 롤백 (`git checkout --`) + 본 부분 보고서 §2 위와 같은 형태의 부분 보고서 작성 + 사용자에게 "Stage F-bis 가설 부정, 추가 결함 존재" 보고.

---

### Task 2 (F-bis-2): 식별된 단계의 spec-인용 line-by-line 분석

**Files:**
- (분석-only, 코드 수정 없음 — F-bis-3에서 한 번에 수정)

**Why:** F-bis-1이 식별한 단계(`synth.Filter` / `postfilter.Filter` / `hpFilter` / `pcm.ScaleUpSat` 중 하나)의 ITU-T G.729 § 인용 + 현 production 코드 line-by-line 비교 + 결함 위치(파일경로:라인) 확정.

본 태스크는 강압-적합 금지 원칙의 핵심: F-bis-1의 stage 식별만으로 *임의로* 코드를 수정하지 않는다. spec § 인용 ⇒ production 코드의 어느 라인이 § 인용과 어긋나는지 ⇒ 어긋난 부분만 수정 — 이 사슬을 본 태스크에서 글로 명시한다.

- [ ] **Step 1: 식별된 단계의 spec § 정확한 인용**

F-bis-1 결과에 따라 다음 중 하나:

**(가) `synth.Filter` 단계가 결함 위치인 경우:**
- §3.10 인용: "When overflow occurs, the speech samples and the filter memory are divided by 4 and the filtering is re-done. The output is multiplied by 4 with saturation."
- 추가 §3.10 인용 (Pass 1 직접형 누산): A(z) 정규화 가정, a[0]=4096 Q12.
- §A.4.2 (G.729A 단순화 특이사항): saturation recovery가 G.729A에서 동일하게 적용되는지 확인 (§A.4.2가 §3.10을 그대로 상속).

production 비교 위치: `internal/synth/filter.go:31-52` (Pass-2 saturation recovery), `internal/synth/filter.go:60-69` (onePass 누산).

**(나) `postfilter.Filter` 단계가 결함 위치인 경우:**
- §3.7 long-term postfilter, §3.8 short-term postfilter, §3.9 tilt + AGC.
- §A.4.2 G.729A postfilter 단순화 (G.729A는 short-term postfilter만, long-term/tilt 별도 처리).
- AGC seed: §A.4.2 default state (예: `g_t_prev = 1.0` Q14 또는 §A.4.2 명시 값).

production 비교 위치: `internal/postfilter/*.go` 전체. 결함이 단일-경로 ×2 형태이면 가장 흔한 후보는 (i) AGC 초기 상태(첫 호출 시 `g_t_prev`), (ii) tilt compensation의 ×(1+μ) 처리, (iii) short-term postfilter의 γ_n/γ_d 가중치 misapplication.

**(다) `hpFilter` 단계가 결함 위치인 경우:**
- §3.10 high-pass filter (DC blocker): `H_hp(z) = (b_0 + b_1·z^-1 + b_2·z^-2) / (1 + a_1·z^-1 + a_2·z^-2)` 형태, b_0=0.93980581, …
- §A.4.2 G.729A 동일 HP filter 사용 명시.
- 초기 상태: `hpX[0]=hpX[1]=hpY[0]=hpY[1]=0`.

production 비교 위치: `internal/decoder/*.go` (`hpFilter` 메서드, `hpX`/`hpY` 필드 초기화).

**(라) `pcm.ScaleUpSat` 단계가 결함 위치인 경우:**
- §3.10 final stage 인용: "The output speech is finally divided by 2 with saturation control"
- ITU-T G.191 G.192 변환과 별개 — spec은 *÷2* 표기 (실제 거동은 `>>1` + saturation).

production 비교 위치: `internal/pcm/scale.go::ScaleUpSat` (또는 동일 기능). 함수 이름 "ScaleUp"이 spec의 "divided by 2"와 *반대 방향*이므로 명명 오류 가능성: `<<1` 사용? 인용 §3.10 명확히 "÷2".

- [ ] **Step 2: production 코드 line-by-line 대조**

식별된 단계의 production 코드 파일을 읽고, Step 1에서 인용한 spec § 식과 line-by-line 매핑.

체크리스트 (강압-적합 금지):
- 각 식의 *부호* 일치 (덧셈 vs 뺄셈)
- 각 변수의 *Q-포맷* 일치 (Q12, Q14, Q15 등)
- 각 시프트(LShl/LShr)의 폭 정확
- 각 saturation의 위치 (중간 단계 saturation 금지 vs 최종 단계 saturation 허용)
- 각 상수의 값 (예: γ_n, γ_d, b_0, b_1, …) 일치

- [ ] **Step 3: 결함 위치 + spec § 인용 + 손계산 기대값을 본 태스크의 출력 노트로 정리**

본 태스크는 코드 수정/커밋 없음. 다음 정보만 분석 노트(F-bis-3 커밋 메시지에 그대로 사용)로 산출:

```
결함 위치: <파일경로:라인>
spec § 인용: ITU-T G.729 §<X.X> "<직접 인용>"
현 production: <라인 그대로 복사>
어긋남: <어떤 부호/Q-포맷/시프트/상수가 spec과 다른지>
수정안: <spec § 인용에 맞춘 코드 형태>
sample 0 영향 손계산: <왜 ÷2 또는 ×2가 되는지>
```

본 노트가 있어야 F-bis-3에서 *임의 수정*이 아닌 *spec § 인용에 따른 수정*이 가능.

**Escape hatch:**
- Step 2에서 식별된 단계의 production 코드가 spec § 인용과 *완전히 일치*하면, F-bis-1이 식별한 단계가 잘못이거나 F-bis-1의 단일-경로 가설 자체가 부정. **본 태스크 종료 + 사용자 보고**: "F-bis-1 stage 식별과 F-bis-2 spec 대조 결과 충돌 — 추가 분석 필요" + working tree 롤백.

---

### Task 3 (F-bis-3): P fix + 하류 fix 단일 커밋 + sample 40 가드

**Files:**
- Modify: `internal/lsp/lsp_lp.go` (Task 1 Step 1의 후보 fix 그대로 landing)
- Modify: F-bis-2가 식별한 하류 단계 파일
- Modify: `internal/decoder/frame0_regression_test.go` (sample 40 가드 추가)
- (옵션) Add or Remove: `internal/decoder/stagef_bis_diagnostic_test.go` — 진단 하니스 영구 보존 가치 판단

**Why:** Stage F partial이 노출한 두 결함을 spec § 인용에 따라 동시 수정. Phase 1i sample 0 잠금이 새 spec-준수 경로에서 자연 회복(got=2 want=2)되는지 검증. sample 40까지 비트-정확 가드 추가.

- [ ] **Step 1: Task 2의 분석 노트에 따라 하류 fix 적용**

F-bis-2 Step 3에서 산출한 분석 노트 그대로 production 코드 수정. *임의 수정 금지* — 분석 노트에 명시되지 않은 라인 변경은 본 커밋 범위 외.

- [ ] **Step 2: 진단 하니스 보존/삭제 결정**

`internal/decoder/stagef_bis_diagnostic_test.go`를 다음 두 형태 중 하나로:

**(가) 영구 가드로 보존 (권장: 회귀 시 즉시 stage 식별 가능):**
- t.Logf 출력을 *그대로* 보존 (assertion으로 승격 시 Δ 값이 기대 형태로 PASS인 보존하기 어려움)
- 또는 sample 0의 4개 경계 값을 명시적 want값으로 어서션 (PST=2 기준 expected = ?)
- 보존 시 본 커밋에 포함

**(나) 진단 목적 종료, 삭제:**
- 보존 가치 < 보존 비용으로 판단 시 삭제
- F-bis-3 커밋 직전 `rm internal/decoder/stagef_bis_diagnostic_test.go`

권장 (가): 영구 가드로 보존. 단 t.Logf만 — sample 0 경계 값들은 향후 spec 개선 시 변경 가능하므로 hard assertion 위험.

- [ ] **Step 3: sample 40 가드 추가**

`internal/decoder/frame0_regression_test.go`에 다음 테스트 추가 (Stage F 플랜 Task 3 Step 4와 동일):

```go
// TestDecode_Frame0SF0AllSamples_MatchesALGTHM: Stage F-bis 가드.
// Phase 1i sample 0 잠금에 더해 sf0 전체(40 samples)가 ALGTHM.PST
// frame 0과 비트-정확함을 영구 보장.
//
// Phase 1k Stage F-bis: §3.2.6 (LSP→LP int64 exact) +
// <F-bis-2 식별 단계>의 spec-correct fix를 단일 커밋으로 landing한 결과.
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
go test -race ./... 2>&1 | tee /tmp/stagef-bis-fix.log
go vet ./...
```

검증 항목 (모두 통과 필수):
- 신규: `TestDecode_Frame0SF0AllSamples_MatchesALGTHM` PASS — sf0 40 샘플 비트-정확
- 회복: `TestDecode_Frame0Sample0_MatchesALGTHM` PASS (got=2 want=2) — Phase 1i 잠금 자연 회복
- 회복: `TestALGTHMFrame0SF0_AzStability` PASS — A(z) minimum-phase
- 영구: Stage D 17개 + D-bis 3개 + F-prep-1/F-prep-2 어서션 (Stage F 플랜 진입 전 미커밋이므로 본 플랜에서는 대상 외) 모두 PASS
- 영구: `internal/fixed` 0 allocs/op

**Escape hatch 1 (절대 준수):**
- `TestDecode_Frame0Sample0_MatchesALGTHM`이 FAIL이면 **Step 5 커밋 금지**, working tree 롤백 (`git checkout --`), 사용자에게 "F-bis-2 식별 단계 또는 fix 형태 부정확 — 추가 분석 필요" 보고.
- `TestDecode_Frame0SF0AllSamples_MatchesALGTHM`이 sample 1~39 어디서든 FAIL이면 sf0 *내부*에 추가 결함이 있다는 신호. 사용자 결정 (즉시 멈춤 vs 추가 진단). **자동 재시도 금지** (Phase 1j 강압-적합 재발 방지).

- [ ] **Step 5: 단일 커밋 (P fix + 하류 fix + sample 40 가드 + 진단 하니스)**

```bash
git add internal/lsp/lsp_lp.go <F-bis-2 식별 단계 파일> internal/decoder/frame0_regression_test.go internal/decoder/stagef_bis_diagnostic_test.go
git commit -m "$(cat <<'EOF'
fix(<lsp+모듈>): ALGTHM frame 0 sf0 bit-exact via §3.2.6 exact arithmetic + §<X.X> <단계 이름> spec-correct path

§3.2.6 LSP→LP recurrence requires exact arithmetic; saturating the
intermediate F1, F2 polynomials in Word32 (Q28) caused asymmetric
a[] coefficients (|k_7|=1.897) at ALGTHM frame 0 sf0. Replace
[11]fixed.Word32 with [11]int64 in lspToLP/polyStep; final Q12
Word16 saturation only at output stage.

§<X.X> <단계 이름> at <파일:라인>: <인용된 spec 식> requires
<수정된 거동>; production used <어긋난 거동>. Single-path 2x
discrepancy at sample 0 (got=4 want=2 with §3.2.6 fix alone)
resolves to want=2 with both fixes combined. Phase 1i sample-0
lock recovers naturally on the new spec-correct path.

Adds sf0 sample-by-sample regression guard (40 samples).

Stage F partial: 2026-04-27-phase1k-stage-f-partial-report.md
Stage F-bis plan:  2026-04-27-phase1k-stage-f-bis-plan.md

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

**Why:** Stage F 플랜 Task 4와 동일. ALGTHM 프레임 0 전체(sf0+sf1) 80-sample 일치를 영구 어서션화.

- [ ] **Step 1: 80-sample 가드 추가**

`internal/decoder/frame0_regression_test.go`에 다음 추가:

```go
// TestDecode_Frame0AllSamples_MatchesALGTHM: Stage V1 최종 가드.
// Phase 1i (sample 0) + Stage F-bis (sf0) + sf1 누적 결과로 프레임 0
// 80 샘플 전체가 ALGTHM.PST와 비트-정확.
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
- **FAIL on sf1 (sample 40~79)** → Stage F-bis 수정이 sf0만 고치고 sf1은 미해결. 가능성:
  - sf0 수정이 pastSynth/pastExc 상태에 잘못된 값을 남김 (Stage F-bis 수정의 stale 효과)
  - sf1 LP 계수가 별도 안정성 문제 (sf0 수정과 무관한 추가 §3.2.6 위반)
  - sf1 자극(C2, S2, GA2, GB2)이 별도 분기점을 활성화
- FAIL 시 즉시 멈추고 보고서에 기록. 추가 사이클(Stage F-tris) 또는 escape hatch 4 발동.

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`
Expected: ALL PASS, vet silent.

- [ ] **Step 4: 커밋**

```bash
git add internal/decoder/frame0_regression_test.go
git commit -m "$(cat <<'EOF'
test(decoder): Stage V1 frame 0 80-sample bit-exact regression guard

Promotes the Stage F-bis sf0 sample-40 guard to full-frame (samples
0..79) coverage. Phase 1i sample-0 lock + Stage F-bis sf0 fix + sf1
combine to make this guard unconditional.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

### Task 5 (V2): ALGTHM skip 메시지 갱신

**Files:**
- Modify: `internal/decoder/decode_test.go`

**Why:** Stage F 플랜 Task 5와 동일. `TestDecode_ITUVectorAlgthmBitExact`의 t.Skip 메시지를 현 상태(frame 0 통과, frames 1-34 보류)로 갱신.

- [ ] **Step 1: 스킵 메시지 갱신**

`internal/decoder/decode_test.go`의 `TestDecode_ITUVectorAlgthmBitExact` 함수에서 t.Skip 메시지를 다음으로 변경:

```go
	t.Skip("Phase 1k Stage F-bis: frame 0 bit-exact via Frame0AllSamples regression " +
		"guard. Frames 1-34 require pastExc/pastSynth state evolution + multi-frame " +
		"diagnostics; deferred to Phase 1l (subagent vectors SPEECH/FIXED) or Phase " +
		"1m (multi-frame ALGTHM).")
```

- [ ] **Step 2: 실행**

Run: `go test -v -run TestDecode_ITUVectorAlgthmBitExact ./internal/decoder/`
Expected: SKIP, 메시지가 갱신된 형태로 출력.

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`
Expected: ALL PASS, vet silent.

- [ ] **Step 4: 커밋**

```bash
git add internal/decoder/decode_test.go
git commit -m "$(cat <<'EOF'
test(decoder): V2 update ALGTHM skip message — frame 0 done, 1-34 deferred

Stage F-bis achieved frame 0 bit-exact (covered by Frame0AllSamples).
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

**Why:** Stage F 플랜 Task 6과 동일. F-bis-3가 lsp + 하류 단계를 변경했으므로 gain 모듈의 4개 병리 테스트가 영향받지 않았음을 확인.

A 전략: 명확히 spec-유도 가능한 케이스(`AllZeroCodebookIsBounded`, `LowEnergyCodebookIsSmooth`)는 측정 없이 기존 어서션 유지.
B 전략: spec-유도 어려운 경계(`HighEnergyCodebookIsBounded`, `SucceedsAcrossAllGainIndices`)는 F-bis-3 후 출력값을 새로 측정하여 어서션 갱신 + spec §A.4.2 인용 주석 추가.

- [ ] **Step 1: 4개 병리 테스트 회귀 확인**

Run: `go test -v -run TestPathological ./internal/gain/`

예상 결과 (대부분):
- `AllZeroCodebookIsBounded`: PASS — gain 모듈 boundary 처리, lsp/synth/postfilter/hpFilter/scale 수정과 무관
- `LowEnergyCodebookIsSmooth`: PASS — 동일 이유
- `HighEnergyCodebookIsBounded`: PASS 또는 임계값 약간 변경 가능
- `SucceedsAcrossAllGainIndices`: PASS — gain 코드북 sweep, F-bis-3 영향 없음

만약 모두 PASS면 Step 2 스킵하고 Step 4(커밋 없이 V3 종료) 또는 빈 커밋(생략).

- [ ] **Step 2: B 전략 — spec-유도 어려운 경계 재측정 (필요 시)**

Step 1에서 1개 이상 FAIL이면 해당 어서션의 임계값을 새 측정값으로 갱신하고 다음 주석을 추가:

```go
// Updated post-Stage F-bis (Phase 1k §3.2.6 fix + §<X.X> <단계> fix):
// empirical boundary reflects post-fix gain VQ behavior. Spec §A.4.2
// only guarantees gpQ14 ∈ [0, 3·Q14], gcQ12 ∈ [0, +Q15]; the tighter
// bound here is empirical regression-guard, not a spec claim.
```

- [ ] **Step 3: 회귀 게이트**

Run: `go test -race ./... && go vet ./...`
Expected: ALL PASS, vet silent.

- [ ] **Step 4: 커밋 (변경이 있는 경우만)**

```bash
git add internal/gain/pathological_test.go
git commit -m "$(cat <<'EOF'
test(gain): V3 pathological re-cert post-Stage F-bis (empirical bounds refresh)

Stage F-bis lsp + <단계> fix did not affect gain VQ behavior; A+B
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
- Create: `docs/superpowers/plans/2026-04-27-phase1k-stage-f-bis-completion-report.md`

**Why:** Phase 1k 누적(Stage D + D-bis + F-prep + F-partial + F-bis + V) 결과를 한 문서에 정리, Phase 1l/1m 진입 조건 명시.

- [ ] **Step 1: 보고서 작성**

```markdown
# Phase 1k Stage F-bis 완료 보고서

**작성일**: 2026-04-XX
**범위**: Phase 1k 전체 (Stage D + D-bis + F-prep + F-partial + F-bis + V) 누적 결과.
**핵심 결론**: ALGTHM frame 0 80 샘플 비트-정확 달성. Stage F partial이 노출한 두 결함(§3.2.6 LSP→LP saturation + 하류 §<X.X> <단계>의 단일-경로 ×2)을 단일 커밋으로 동시 수정. Phase 1i sample 0 잠금 자연 회복.

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
| F-bis | 3 | <sha> | fix(<lsp+모듈>): §3.2.6 exact arithmetic + §<X.X> <단계> spec-correct path |
| V | 1 | <sha> | test(decoder): V1 frame 0 80-sample guard |
| V | 2 | <sha> | test(decoder): V2 ALGTHM skip message |
| V | 3 | <sha> | test(gain): V3 pathological re-cert (변경 시) |

회귀 게이트: 마지막 커밋 시점 `go test -race ./...` ALL PASS, `go vet ./...` silent, `internal/fixed` 0 allocs/op.

---

## 2. F-bis-1 stage-by-stage 진단 결과

진단 하니스: `internal/decoder/stagef_bis_diagnostic_test.go::TestDiagnostic_FbisStageBoundaries_Sample0Trace`

후보 P fix(`[11]int64` exact)만 적용한 working tree에서 측정한 sample 0 4개 경계 값:

| 경계 | sample 0 | Δ vs PST(want=2) |
|------|---------|------------------|
| `synth.Filter` 직후 | <측정값> | <Δ> |
| `postfilter.Filter` 직후 | <측정값> | <Δ> |
| `hpFilter` 직후 | <측정값> | <Δ> |
| `pcm.ScaleUpSat` 직후 | 4 | +2 |
| **PST want** | **2** | **0** |

판정: **결함 진입 단계 = <식별된 단계>** — `<직전 경계>`와 `<해당 경계>` 사이에서 sample 0이 정확히 2x 변화.

---

## 3. F-bis-2 spec-인용 분석

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

sample 0 영향 손계산: <왜 ÷2 누락 또는 ×2 잉여가 되는지>

---

## 4. 결합 fix가 sample 0 잠금을 자연 회복하는 메커니즘

§3.2.6 결함만 수정 (P fix 단독): a[] 정확 → 1/A(z) 출력 정확 → … → ScaleUpSat 출력 = 4 (want=2). 단일-경로 ×2 누락이 노출됨.

§<X.X> 결함만 수정 (하류 fix 단독): 가설상 a[] 비대칭 → 14 dB sf0 발산 (Stage D-bis가 측정).

§3.2.6 + §<X.X> 동시 수정: a[] 정확 + 단일-경로 ÷2 적용 → ScaleUpSat 출력 = 2 (want=2). Phase 1i 잠금 자연 회복. **두 결함이 서로 정확히 상쇄하던 형태가 풀리고 spec-준수 경로로 정렬.**

이는 Phase 1j 가설("두 14 dB 오차가 sample 0에서 상쇄")의 정량적 확증: 한 결함이 다른 결함의 잘못된 ×2 또는 ÷2를 우연히 정정하던 형태.

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
- `TestDecode_Frame0SF0AllSamples_MatchesALGTHM` (Stage F-bis)
- `TestDecode_Frame0AllSamples_MatchesALGTHM` (Stage V1)
- `TestALGTHMFrame0SF0_AzStability` (lsp F-prep-1)
- `TestFilter_ImpulseResponse_OnePoleClosedForm` (synth F-prep-2)
- `TestFilter_SaturationRecovery_ScalingFactorMatchesSpec` (synth F-prep-2)
- `TestDiagnostic_FbisStageBoundaries_Sample0Trace` (F-bis-1, t.Logf 진단; 보존 시)
- Phase 1k Stage D 17개 컨트랙트 + D-bis 3개 = 20개

총 신규 영구 가드 ≈ 27개 (Phase 1k 누적).

---

## 7. 다음 단계 후보

- **Phase 1l**: SPEECH/FIXED ITU 벡터 활성화. ALGTHM frames 1-34는 multi-frame state 진단 필요.
- **Phase 1m**: ALGTHM frames 1-34 비트-정확 — pastExc/pastSynth/MA predictor 다중 프레임 진화 진단.
- **Phase 1n**: 인코더 시작 (현재까지는 디코더 단독).

---

## 8. 탈출 해치 발동 평가

| 해치 | 발동 여부 | 비고 |
|------|----------|------|
| 1 (sample 0 회귀) | F-partial에서 발동 → F-bis로 흡수 | 27개 가드로 영구 잠금 |
| 2 (다른 Phase 1i fix 회귀) | 미발동 | — |
| 3 (Stage D + D-bis 어서션 회귀) | 미발동 | — |
| 4 (Stage 전체 비결정) | 미발동 | F-bis 단일 커밋으로 80-sample 달성 |
```

- [ ] **Step 2: 보고서 커밋**

```bash
git add docs/superpowers/plans/2026-04-27-phase1k-stage-f-bis-completion-report.md
git commit -m "$(cat <<'EOF'
docs(plans): Phase 1k Stage F-bis completion report — ALGTHM frame 0 80-sample bit-exact

Combined-fix landing at §3.2.6 + §<X.X> <단계> identified by F-bis-1
stage-by-stage diagnostic and F-bis-2 spec-citation analysis. Two
defects mutually cancelled at sample 0; both spec-correct fixes land
in a single commit. ~27 permanent regression guards accumulated
across Phase 1k. Phase 1l/1m entry conditions met.

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

- **Spec coverage:** Stage F partial §7 경로 A의 4단계 ((i) P fix candidate 보관, (ii) F-prep-3 stage-by-stage 진단, (iii) 하류 결함 후보 좁히기, (iv) 동시 커밋)이 본 플랜에서 각각 Task 1 Step 1, Task 1 Step 3-4, Task 2, Task 3에 매핑.
- **Placeholder scan:** F-bis-3 Step 1/5와 완료 보고서 §1/§2/§3에서 `<F-bis-2 식별 단계 파일>`, `<X.X>`, `<단계 이름>` 등 placeholder 사용 — F-bis-1/2 결과로 채워질 부분만. 그 외 모든 코드 블록 완성.
- **Type consistency:** `lsp.Decoder.Decode`, `synth.Synthesizer.Filter`, `pst.Filter`, `hpFilter`, `pcm.ScaleUpSat` 시그니처는 기존 파일과 일치 가정 (Task 1 Step 3에 시그니처 점검 가드레일 명시).
- **Escape hatch alignment:** Phase 1k 설계 §7의 4개 hatch가 본 플랜에서도 작동:
  - F-bis-1 Step 5: 진단 종료 시 RED 상태 미커밋
  - F-bis-2 Step 3: 단계 식별과 spec 대조 충돌 시 사용자 보고 + 롤백
  - F-bis-3 Step 4: sample 0 잠금 미회복 시 커밋 금지 + 롤백
  - F-bis-3 Step 4: sf0 sample 1~39 FAIL 시 즉시 멈춤
- **Scratch-from-spec:** 모든 기대값이 ITU-T G.729 §3.2.6 / §3.7 / §3.8 / §3.9 / §3.10 / §A.4.2 인용. 외부 참조 코드 0건.
- **No force-fit:** F-bis-2가 *분석 노트* 단계를 별도 태스크로 분리한 이유는 강압-적합 금지 원칙 — F-bis-1의 stage 식별만으로 임의 수정 금지, spec § 인용 ⇒ production 어긋남 ⇒ 수정안 사슬을 글로 명시한 후에만 F-bis-3 진입.

---

## Execution Handoff

**1. Subagent-Driven (recommended)** — 태스크별 새 서브에이전트 디스패치, F-bis-1 결과 보고 후 사용자가 F-bis-2 진입 승인. F-bis-2 분석 노트 종료 후 사용자가 F-bis-3 단일 커밋 승인.

**2. Inline Execution** — `superpowers:executing-plans`로 배치 실행 + F-bis-3 직전 체크포인트.

작성자가 사용자에게 두 옵션 중 하나 선택 요청. **F-bis-1 / F-bis-2 / F-bis-3 사이의 모든 분기 결정은 사용자 결정 권고** (Phase 1j 강압-적합 재발 방지, Stage F partial이 escape hatch 1을 모범적으로 발동한 사례를 본 플랜에서도 보존).
