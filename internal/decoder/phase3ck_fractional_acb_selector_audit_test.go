package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3ckFractionalACBSelectorAudit localizes which fractional
// adaptive-codebook subsets damage FFmpeg black-box agreement. It does not
// promote a selector: any selector that ignores transmitted fractional delay
// is a diagnostic fallback, not a strict G.729 decoder fix.
func TestPhase3ckFractionalACBSelectorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_FRACTIONAL_ACB_SELECTOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FRACTIONAL_ACB_SELECTOR_AUDIT=1 to run fractional ACB selector audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	bitPath := vectorPath("SPEECH.BIT")
	ensureTestdataPresent(t, bitPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	speechFrames := len(bitData) / bitstream.G192FrameBytes
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "phase3ck-speech-bit")
	phase3ckReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk selector audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "phase3ck-asterisk")
	phase3ckReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3ckSelectorVariant struct {
	name  string
	build func(sub int, tInt, tFrac int, pastExc []int16, v *[subframeLen]int16)
}

func phase3ckSelectorVariants() []phase3ckSelectorVariant {
	return []phase3ckSelectorVariant{
		phase3ckUseSelector("strict_fractional", func(_ int, _ int, _ int) bool { return true }),
		phase3ckUseSelector("integer_lag_only", func(_ int, _ int, tFrac int) bool { return tFrac == 0 }),
		phase3ckUseSelector("sf1_fractional_only", func(sub int, _ int, tFrac int) bool { return sub == 0 || tFrac == 0 }),
		phase3ckUseSelector("sf2_fractional_only", func(sub int, _ int, tFrac int) bool { return sub == 1 || tFrac == 0 }),
		phase3ckUseSelector("frac_neg_only", func(_ int, _ int, tFrac int) bool { return tFrac <= 0 }),
		phase3ckUseSelector("frac_pos_only", func(_ int, _ int, tFrac int) bool { return tFrac >= 0 }),
		phase3ckUseSelector("short_pitch_fractional_only", func(_ int, tInt, tFrac int) bool { return tFrac == 0 || tInt < subframeLen }),
		phase3ckUseSelector("long_pitch_fractional_only", func(_ int, tInt, tFrac int) bool { return tFrac == 0 || tInt >= subframeLen }),
		phase3ckUseSelector("sf1_pos_sf2_neg_fractional", func(sub int, _ int, tFrac int) bool {
			return tFrac == 0 || (sub == 0 && tFrac > 0) || (sub == 1 && tFrac < 0)
		}),
		phase3ckUseSelector("sf1_neg_sf2_pos_fractional", func(sub int, _ int, tFrac int) bool {
			return tFrac == 0 || (sub == 0 && tFrac < 0) || (sub == 1 && tFrac > 0)
		}),
		phase3ckTargetedSelector("pos_strict_neg_no_k_adj", func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			if tFrac < 0 {
				phase3hAdaptiveCodebook(tInt, tFrac, pastExc, v, phase3hACBFracNegNoKAdjust)
				return
			}
			decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
		}),
		phase3ckTargetedSelector("pos_strict_neg_sign_flip", func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			if tFrac < 0 {
				pitch.AdaptiveCodebook(tInt, -tFrac, pastExc, v)
				return
			}
			decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
		}),
		phase3ckTargetedSelector("pos_strict_neg_delay_plus_1", func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			if tFrac < 0 {
				pitch.AdaptiveCodebook(phase3hClampDelay(tInt+1), tFrac, pastExc, v)
				return
			}
			decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
		}),
		phase3ckTargetedSelector("pos_strict_neg_integer", func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			if tFrac < 0 {
				decodeAdaptiveCodebookIntegerLagOnly(tInt, pastExc, v)
				return
			}
			decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
		}),
		phase3ckTargetedSelector("pos_integer_neg_no_k_adj", func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			switch {
			case tFrac < 0:
				phase3hAdaptiveCodebook(tInt, tFrac, pastExc, v, phase3hACBFracNegNoKAdjust)
			case tFrac > 0:
				decodeAdaptiveCodebookIntegerLagOnly(tInt, pastExc, v)
			default:
				decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
			}
		}),
	}
}

func phase3ckUseSelector(name string, use func(sub int, tInt, tFrac int) bool) phase3ckSelectorVariant {
	return phase3ckSelectorVariant{
		name: name,
		build: func(sub int, tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			if use(sub, tInt, tFrac) {
				decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
				return
			}
			decodeAdaptiveCodebookIntegerLagOnly(tInt, pastExc, v)
		},
	}
}

func phase3ckTargetedSelector(name string, build func(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16)) phase3ckSelectorVariant {
	return phase3ckSelectorVariant{
		name: name,
		build: func(_ int, tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
			build(tInt, tFrac, pastExc, v)
		},
	}
}

func phase3ckReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	variants := phase3ckSelectorVariants()
	rows := make([]phase3xRow, 0, len(variants))
	features := phase3ckCollectG192PitchFeatures(t, bitData, frames)
	baseOut := phase3ckDecodeG192Selector(t, bitData, frames, variants[0])
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)

	t.Logf("Phase 3ck fractional ACB selector audit - %s (%d frames)", label, frames)
	t.Logf("%-30s %9s %7s %10s %10s %8s %9s %9s %9s",
		"selector", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	for _, v := range variants {
		out := baseOut
		if v.name != variants[0].name {
			out = phase3ckDecodeG192Selector(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-30s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
		if phase3ckShouldLogLag(v.name) {
			phase3ckLogLagByPitch(t, label+" "+v.name, ref, out, features)
		}
	}
	phase3ckLogBest(t, base, baseEnv, rows)
}

func phase3ckReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	variants := phase3ckSelectorVariants()
	rows := make([]phase3xRow, 0, len(variants))
	features := phase3ckCollectRawPitchFeatures(t, raw, frames)
	baseOut := phase3ckDecodeRawSelector(t, raw, frames, variants[0])
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)

	t.Logf("Phase 3ck fractional ACB selector audit - %s (%d frames)", label, frames)
	t.Logf("%-30s %9s %7s %10s %10s %8s %9s %9s %9s",
		"selector", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	for _, v := range variants {
		out := baseOut
		if v.name != variants[0].name {
			out = phase3ckDecodeRawSelector(t, raw, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-30s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
		if phase3ckShouldLogLag(v.name) {
			phase3ckLogLagByPitch(t, label+" "+v.name, ref, out, features)
		}
	}
	phase3ckLogBest(t, base, baseEnv, rows)
}

func phase3ckShouldLogLag(name string) bool {
	switch name {
	case "strict_fractional", "integer_lag_only", "frac_pos_only", "frac_neg_only", "pos_strict_neg_sign_flip":
		return true
	default:
		return false
	}
}

type phase3ckPitchFeature struct {
	tInt1  int
	tFrac1 int
	tInt2  int
	tFrac2 int
}

func phase3ckCollectG192PitchFeatures(t *testing.T, bitData []byte, frames int) []phase3ckPitchFeature {
	t.Helper()
	features := make([]phase3ckPitchFeature, 0, frames)
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[pitch features] frame %d: %v", frame, err)
		}
		features = append(features, phase3ckPitchFeatureFromPacked(t, packed[:], frame))
	}
	return features
}

func phase3ckCollectRawPitchFeatures(t *testing.T, raw []byte, frames int) []phase3ckPitchFeature {
	t.Helper()
	features := make([]phase3ckPitchFeature, 0, frames)
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		features = append(features, phase3ckPitchFeatureFromPacked(t, raw[start:start+bitstream.FrameBytes], frame))
	}
	return features
}

func phase3ckPitchFeatureFromPacked(t *testing.T, packed []byte, frame int) phase3ckPitchFeature {
	t.Helper()
	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		t.Fatalf("unpack pitch feature frame %d: %v", frame, err)
	}
	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)
	return phase3ckPitchFeature{tInt1: tInt1, tFrac1: tFrac1, tInt2: tInt2, tFrac2: tFrac2}
}

func phase3ckLogLagByPitch(t *testing.T, label string, ref, out []int16, features []phase3ckPitchFeature) {
	t.Helper()
	lags := phase3rBestFrameLags(ref, out, 8)
	lagged := phase3rFrameLagOracle(ref, out, 8)
	laggedM := blackboxMeasure(ref, lagged, 40)
	t.Logf("%s lag8 oracle: GlobalSNR %.2f SegSNR %.2f corr %.3f hist %s",
		label, laggedM.globalSNR, laggedM.segSNR, laggedM.corr,
		phase3rFormatLagHistogram(phase3ckLagCounts(lags), len(lags)))

	type group struct {
		total   int
		nonzero int
		sum     int
		absSum  int
		counts  map[int]int
	}
	groups := map[string]*group{}
	add := func(name string, lag int) {
		g := groups[name]
		if g == nil {
			g = &group{counts: map[int]int{}}
			groups[name] = g
		}
		g.total++
		if lag != 0 {
			g.nonzero++
		}
		g.sum += lag
		if lag < 0 {
			g.absSum -= lag
		} else {
			g.absSum += lag
		}
		g.counts[lag]++
	}
	frames := len(lags)
	if len(features) < frames {
		frames = len(features)
	}
	for frame := 0; frame < frames; frame++ {
		lag := lags[frame]
		f := features[frame]
		add("all", lag)
		if f.tFrac1 == 0 && f.tFrac2 == 0 {
			add("both_zero", lag)
		} else {
			add("any_frac", lag)
		}
		if f.tFrac1 < 0 || f.tFrac2 < 0 {
			add("any_neg", lag)
		}
		if f.tFrac1 > 0 || f.tFrac2 > 0 {
			add("any_pos", lag)
		}
		if f.tFrac1 < 0 {
			add("sf1_neg", lag)
		}
		if f.tFrac2 < 0 {
			add("sf2_neg", lag)
		}
		if f.tFrac1 > 0 {
			add("sf1_pos", lag)
		}
		if f.tFrac2 > 0 {
			add("sf2_pos", lag)
		}
		if f.tInt1 < subframeLen || f.tInt2 < subframeLen {
			add("short_pitch", lag)
		} else {
			add("long_pitch", lag)
		}
	}
	for _, name := range []string{"all", "both_zero", "any_frac", "any_neg", "any_pos", "sf1_neg", "sf2_neg", "sf1_pos", "sf2_pos", "short_pitch", "long_pitch"} {
		g := groups[name]
		if g == nil || g.total == 0 {
			continue
		}
		t.Logf("  %-11s n=%4d nonzero=%5.1f%% mean=%+.2f meanAbs=%.2f hist=%s",
			name, g.total, 100*float64(g.nonzero)/float64(g.total),
			float64(g.sum)/float64(g.total), float64(g.absSum)/float64(g.total),
			phase3rFormatLagHistogram(g.counts, g.total))
	}
}

func phase3ckLagCounts(lags []int) map[int]int {
	counts := map[int]int{}
	for _, lag := range lags {
		counts[lag]++
	}
	return counts
}

func phase3ckLogBest(t *testing.T, base blackboxMetrics, baseEnv phase3pEnvelopeSummary, rows []phase3xRow) {
	t.Helper()
	best := rows[0]
	bestSeg := rows[0]
	bestCorr := rows[0]
	bestEnv := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.segSNR > bestSeg.m.segSNR {
			bestSeg = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
		if r.env.lowRatioFrames < bestEnv.env.lowRatioFrames {
			bestEnv = r
		}
	}
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-base.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-base.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-base.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnv.name, bestEnv.env.lowRatioFrames, bestEnv.env.lowRatioFrames-baseEnv.lowRatioFrames)
	if best.name == "strict_fractional" && bestSeg.name == "strict_fractional" && bestCorr.name == "strict_fractional" && bestEnv.name == "strict_fractional" {
		t.Log("verdict: strict fractional ACB is best across measured selectors; continue gain/envelope/state diagnostics")
		return
	}
	if best.name == "integer_lag_only" || bestSeg.name == "integer_lag_only" || bestCorr.name == "integer_lag_only" || bestEnv.name == "integer_lag_only" {
		t.Log("verdict: every useful improvement collapses transmitted fractional delay; continue looking for the underlying convention/state bug")
		return
	}
	t.Log("verdict: nontrivial fractional selector found; inspect before any production change")
}

func phase3ckDecodeG192Selector(t *testing.T, bitData []byte, frames int, variant phase3ckSelectorVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, frame, err)
		}
		if err := dec.decodeFramePhase3ckSelector(packed[:], out[frame*frameSamples:(frame+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3ckSelector[%s] frame %d: %v", variant.name, frame, err)
		}
	}
	return out
}

func phase3ckDecodeRawSelector(t *testing.T, raw []byte, frames int, variant phase3ckSelectorVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		if err := dec.decodeFramePhase3ckSelector(raw[start:start+bitstream.FrameBytes], out[frame*frameSamples:(frame+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3ckSelector[%s] raw frame %d: %v", variant.name, frame, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3ckSelector(packed []byte, out []int16, variant phase3ckSelectorVariant) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}
	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return err
	}
	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(fr.L0),
		L1: uint8(fr.L1),
		L2: uint8(fr.L2),
		L3: uint8(fr.L3),
	})
	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)
	d.decodeSubframePhase3ckSelector(&sf1A, 0, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3ckSelector(&sf2A, 1, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3ckSelector(
	sfA *[lpcOrder + 1]int16,
	sub int,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	variant phase3ckSelectorVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	variant.build(sub, tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)
	gpQ14, gcMant, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}
