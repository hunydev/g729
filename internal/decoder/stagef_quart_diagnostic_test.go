package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
	"github.com/exedev/g729/internal/tables"
)

// TestDiagnostic_FquartGainImap_Sf0Sample0to7: Stage F-quart-1 진단.
//
// ITU-T G.729 (06/2012) §3.9.3 ("To reduce the impact of single bit errors,
// the GA and GB indices are reordered before transmission. The mapping
// tables are given in Annex C/D.") 에 따른 디코더 측 GainImap1/GainImap2
// inverse-map 적용을 *production 코드 0-수정* 으로 평행 시뮬레이션한다.
//
// Branch A (production verbatim): gain.Decoder.Decode(GA=f.GA1, GB=f.GB1)
// Branch B (spec-fix):             gain.Decoder.Decode(
//                                      GA=GainImap1[f.GA1],
//                                      GB=GainImap2[f.GB1])
//
// production decodeVQ 는 GBK[bits] 인덱싱이므로 Branch B 의 호출 결과는
// 결과적으로 GBK[GainImap[bits]] (= §3.9.3 spec-correct) 와 동일하다.
//
// 두 분기는 별도 Decoder/Synthesizer/Postfilter/Pcm instance 를 갖는다
// (pastErrors / pastSynth / agcGainPrev / pastExc 분기별 분리 보장).
//
// frame 0 sf0 만 측정하므로 모든 instance 는 zero-value (= §4.3 초기화)
// 에서 시작; sf1 은 디코딩하지 않는다.
//
// 본 진단은 측정-only — fix 적용 금지. t.Errorf 미사용.
func TestDiagnostic_FquartGainImap_Sf0Sample0to7(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	gaRaw := uint8(f.GA1)
	gbRaw := uint8(f.GB1)
	gaMap := tables.GainImap1[gaRaw]
	gbMap := tables.GainImap2[gbRaw]
	t.Logf("frame 0 sf0 indices: GA1=%d GB1=%d  →  GainImap1[GA1]=%d  GainImap2[GB1]=%d",
		gaRaw, gbRaw, gaMap, gbMap)

	// PST/2 spec-target (= ITU PST sample >> 1) — F-tris-1 §0.2 도메인.
	var pstHalf [subframeLen]int16
	for n := 0; n < subframeLen; n++ {
		pstHalf[n] = int16(int32(wantFrames[0][n]) >> 1)
	}
	t.Logf("PST want sf0 (sample 0..39):")
	var pstWant [subframeLen]int16
	for n := 0; n < subframeLen; n++ {
		pstWant[n] = wantFrames[0][n]
	}
	dumpInt16(t, pstWant[:])
	t.Logf("PST/2 spec-target (sample 0..39):")
	dumpInt16(t, pstHalf[:])

	// ── Branch A: production verbatim ───────────────────────────────
	branchA := decodeFquartSf0(t, &f, gaRaw, gbRaw)
	t.Logf("──────── Branch A (production verbatim, GA=%d GB=%d) ────────", gaRaw, gbRaw)
	logBranch(t, branchA, pstHalf[:])

	// Sanity check (절대 제약 §4): Branch A synth.Filter sample 0..7
	// 가 F-tris-1 baseline [2 3 4 4 3 2 1 1] 와 일치해야 한다.
	wantSynthA := [8]int16{2, 3, 4, 4, 3, 2, 1, 1}
	for n := 0; n < 8; n++ {
		if branchA.synth[n] != wantSynthA[n] {
			t.Fatalf("Branch A synth.Filter sample %d = %d, want %d (F-tris-1 baseline) — test infra 결함",
				n, branchA.synth[n], wantSynthA[n])
		}
	}
	t.Logf("Branch A sanity check OK: synth.Filter[0..7] == F-tris-1 baseline [2 3 4 4 3 2 1 1].")

	// ── Branch B: §3.9.3 spec-fix (inverse-mapped GA/GB) ───────────
	branchB := decodeFquartSf0(t, &f, gaMap, gbMap)
	t.Logf("──────── Branch B (spec-fix, GA=%d GB=%d) ────────", gaMap, gbMap)
	logBranch(t, branchB, pstHalf[:])

	// ── 종합 비교표 ─────────────────────────────────────────────────
	t.Logf("──────── F-quart-1 비교표 (vs PST/2 = [1 2 1 1 0 -1 -1 -1]) ────────")
	t.Logf("Branch       Boundary           [ 0..  7]                       matches/40")
	t.Logf("A (prod)     synth.Filter       %s  %d/40", fmtSamples8(branchA.synth[:]), matchCount(branchA.synth[:], pstHalf[:], 1))
	t.Logf("A (prod)     postfilter.Filter  %s  %d/40", fmtSamples8(branchA.post[:]), matchCount(branchA.post[:], pstHalf[:], 1))
	t.Logf("A (prod)     hpFilter           %s  %d/40", fmtSamples8(branchA.hp[:]), matchCount(branchA.hp[:], pstHalf[:], 1))
	t.Logf("A (prod)     pcm.ScaleUpSat     %s  (PST 도메인)", fmtSamples8(branchA.pcm[:]))
	t.Logf("B (spec)     synth.Filter       %s  %d/40", fmtSamples8(branchB.synth[:]), matchCount(branchB.synth[:], pstHalf[:], 1))
	t.Logf("B (spec)     postfilter.Filter  %s  %d/40", fmtSamples8(branchB.post[:]), matchCount(branchB.post[:], pstHalf[:], 1))
	t.Logf("B (spec)     hpFilter           %s  %d/40", fmtSamples8(branchB.hp[:]), matchCount(branchB.hp[:], pstHalf[:], 1))
	t.Logf("B (spec)     pcm.ScaleUpSat     %s  (PST 도메인)", fmtSamples8(branchB.pcm[:]))

	// Branch B hpFilter sample 0..7 vs PST/2 절대 차 (시나리오 분류용).
	t.Logf("──────── Branch B hpFilter sample 0..7 |Δ| vs PST/2 ────────")
	allWithin1 := true
	for n := 0; n < 8; n++ {
		d := int32(branchB.hp[n]) - int32(pstHalf[n])
		ad := d
		if ad < 0 {
			ad = -ad
		}
		within := ad <= 1
		if !within {
			allWithin1 = false
		}
		t.Logf("  n=%d: hpB=%d  spec=%d  Δ=%+d  |Δ|≤1? %t", n, branchB.hp[n], pstHalf[n], d, within)
	}
	mB := matchCount(branchB.hp[:], pstHalf[:], 1)
	mA := matchCount(branchA.hp[:], pstHalf[:], 1)
	t.Logf("Branch B hpFilter 40-sample matches vs PST/2: %d/40 (Branch A: %d/40)", mB, mA)

	// 시나리오 분류 (S1/S2/S3) — 측정값에 따라 자동 분류; 보고서 §5 와 일치해야 함.
	scenario := "S?"
	switch {
	case allWithin1 && mB == 40:
		scenario = "S1 (충분조건: sample 0..7 |Δ|≤1 + 40/40 일치)"
	case mB < mA:
		scenario = "S3 (악화: Branch B 정렬도 < Branch A)"
	default:
		scenario = "S2 (부분조건: 일부 sample 만 일치 또는 40-sample 불완전)"
	}
	t.Logf("→ 시나리오 분류: %s", scenario)
}

// fquartBoundary holds the four boundary outputs of one decoding branch.
type fquartBoundary struct {
	gpQ14 int16
	gcQ12 int16
	synth [subframeLen]int16
	post  [subframeLen]int16
	hp    [subframeLen]int16
	pcm   [subframeLen]int16
}

// decodeFquartSf0 runs frame 0 sf0 through a fresh Decoder instance using
// the supplied gain VQ indices (raw or inverse-mapped) and captures all
// four pipeline boundaries. Production code is *not* modified — the test
// reuses the unexported per-stage helpers exactly as decodeSubframe does.
func decodeFquartSf0(t *testing.T, f *bitstream.Frame, ga, gb uint8) fquartBoundary {
	t.Helper()

	var d Decoder

	sfA, _ := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))

	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt1, tFrac1, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt1, betaQ14, &c)

	gpQ14, gcQ12 := d.gn.Decode(gain.Indices{GA: ga, GB: gb}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	var out fquartBoundary
	out.gpQ14 = gpQ14
	out.gcQ12 = gcQ12

	d.syn.Filter(&sfA, &u, &out.synth)
	d.pst.Filter(&sfA, tInt1, &out.synth, &out.post)
	d.hpFilter(&out.post, out.hp[:])
	pcm.ScaleUpSat(out.hp[:], out.pcm[:])

	return out
}

func logBranch(t *testing.T, b fquartBoundary, pstHalf []int16) {
	t.Helper()
	t.Logf("  gain VQ output: g_p (Q14) = %d   γ̂_c (Q12) = %d", b.gpQ14, b.gcQ12)
	t.Logf("  synth.Filter sf0:")
	dumpInt16(t, b.synth[:])
	t.Logf("  postfilter.Filter sf0:")
	dumpInt16(t, b.post[:])
	t.Logf("  hpFilter sf0:")
	dumpInt16(t, b.hp[:])
	t.Logf("  pcm.ScaleUpSat sf0 (PST domain):")
	dumpInt16(t, b.pcm[:])
	t.Logf("  matches vs PST/2 (|Δ|≤1 LSB): synth=%d/40 post=%d/40 hp=%d/40",
		matchCount(b.synth[:], pstHalf, 1),
		matchCount(b.post[:], pstHalf, 1),
		matchCount(b.hp[:], pstHalf, 1),
	)
}

func fmtSamples8(v []int16) string {
	return formatN16(v, 8)
}

func formatN16(v []int16, n int) string {
	out := []byte{'['}
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, []byte(itoaPad(int32(v[i]), 3))...)
	}
	out = append(out, ']')
	return string(out)
}

func itoaPad(x int32, w int) string {
	neg := false
	if x < 0 {
		neg = true
		x = -x
	}
	digits := []byte{}
	if x == 0 {
		digits = append(digits, '0')
	}
	for x > 0 {
		digits = append([]byte{byte('0' + x%10)}, digits...)
		x /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	for len(digits) < w {
		digits = append([]byte{' '}, digits...)
	}
	return string(digits)
}
