package decoder

import (
	"math"
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

// TestPhase3bjEnhancedTapSubframeEstimatorAudit repeats the subframe envelope
// estimator using taps captured from the same enhanced decode path that feeds
// DecodeEnvelopeRecovered. FFmpeg is used only as an executable black-box
// decoder for numeric training/evaluation labels.
func TestPhase3bjEnhancedTapSubframeEstimatorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ENHANCED_TAP_SUBFRAME_ESTIMATOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ENHANCED_TAP_SUBFRAME_ESTIMATOR_AUDIT=1 to run enhanced-tap subframe estimator audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	speech := phase3bjLoadG192Dataset(t, "SPEECH.BIT")
	train, holdout := phase3bjSplitSamples(speech.samples)
	model := phase3bjFitRidge(t, train, 1e-3)

	t.Logf("Phase 3bj enhanced-tap subframe envelope estimator audit")
	t.Logf("SPEECH.BIT model: train=%d holdout=%d coeff=%s", len(train), len(holdout), phase3bbFormatCoefficients(model))
	phase3bjLogDataset(t, "SPEECH.BIT holdout", holdout, model)
	phase3bjLogOutput(t, "SPEECH.BIT full", speech, model)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk enhanced-tap subframe estimator audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	asterisk := phase3bjLoadRawDataset(t, "Asterisk", rawPath)
	phase3bjLogDataset(t, "Asterisk external with SPEECH model", asterisk.samples, model)
	phase3bjLogOutput(t, "Asterisk external with SPEECH model", asterisk, model)

	astTrain, astHoldout := phase3bjSplitSamples(asterisk.samples)
	combinedTrain := append(append([]phase3bjSample{}, train...), astTrain...)
	combinedModel := phase3bjFitRidge(t, combinedTrain, 1e-3)
	t.Logf("combined SPEECH.BIT+Asterisk model: train=%d holdout=%d coeff=%s",
		len(combinedTrain), len(holdout)+len(astHoldout), phase3bbFormatCoefficients(combinedModel))
	phase3bjLogOutput(t, "combined model SPEECH.BIT full", speech, combinedModel)
	phase3bjLogDataset(t, "combined model Asterisk holdout", astHoldout, combinedModel)
	phase3bjLogOutput(t, "combined model Asterisk external", asterisk, combinedModel)
}

type phase3bjDataset struct {
	label    string
	ref      []int16
	enhanced []int16
	taps     []phase3bjFrameTaps
	samples  []phase3bjSample
}

type phase3bjFrameTaps struct {
	frame bitstream.Frame
	sub   [2]phase3bjSubframeTaps
}

type phase3bjSubframeTaps struct {
	tInt     int
	tFrac    int
	gp       float64
	gc       float64
	pitchRMS float64
	fixedRMS float64
	uRMS     float64
	sRMS     float64
	spfRMS   float64
	hpRMS    float64
}

type phase3bjSample struct {
	frame    int
	sf       int
	refRMS   float64
	localRMS float64
	features []float64
}

func phase3bjLoadG192Dataset(t *testing.T, name string) phase3bjDataset {
	t.Helper()
	path := vectorPath(name)
	ensureTestdataPresent(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	frames := len(data) / bitstream.G192FrameBytes
	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, name+".g729")
	writeG192RawForEnvelopeAudit(t, data, frames, rawPath)
	ref := phase3uFFmpegDecodeG192(t, data, frames, name)
	enhanced, taps := phase3bjDecodeEnhancedWithTaps(t, rawPath, frames)
	return phase3bjMakeDataset(name, ref, enhanced, taps)
}

func phase3bjLoadRawDataset(t *testing.T, label, rawPath string) phase3bjDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, label)
	enhanced, taps := phase3bjDecodeEnhancedWithTaps(t, rawPath, frames)
	return phase3bjMakeDataset(label, ref, enhanced, taps)
}

func phase3bjMakeDataset(label string, ref, enhanced []int16, taps []phase3bjFrameTaps) phase3bjDataset {
	samples := phase3bjSamples(ref, enhanced, taps)
	return phase3bjDataset{label: label, ref: ref, enhanced: enhanced, taps: taps, samples: samples}
}

func phase3bjDecodeEnhancedWithTaps(t *testing.T, rawPath string, frames int) ([]int16, []phase3bjFrameTaps) {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw g729 payload: %v", err)
	}
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}
	out := make([]int16, frames*frameSamples)
	taps := make([]phase3bjFrameTaps, frames)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		packed := raw[frame*bitstream.FrameBytes : (frame+1)*bitstream.FrameBytes]
		frameOut := out[frame*frameSamples : (frame+1)*frameSamples]
		frameTaps, err := dec.decodeEnhancedFrameWithTaps(packed, frameOut)
		if err != nil {
			t.Fatalf("decodeEnhancedFrameWithTaps frame %d: %v", frame, err)
		}
		taps[frame] = frameTaps
	}
	return out, taps
}

func (d *Decoder) decodeEnhancedFrameWithTaps(packed []byte, out []int16) (phase3bjFrameTaps, error) {
	var taps phase3bjFrameTaps
	if len(packed) < bitstream.FrameBytes {
		return taps, ErrShortInput
	}
	if len(out) < frameSamples {
		return taps, ErrShortOutput
	}
	if err := bitstream.Unpack(packed, &taps.frame); err != nil {
		return taps, err
	}
	f := taps.frame
	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0),
		L1: uint8(f.L1),
		L2: uint8(f.L2),
		L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	var stats envelopeRecoveryStats
	stats.hasGA036 = envelopeRecoveryHasGA036(uint8(f.GA1)) || envelopeRecoveryHasGA036(uint8(f.GA2))
	d.decodeEnhancedSubframeWithTaps(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen], &stats, &taps.sub[0])
	d.decodeEnhancedSubframeWithTaps(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples], &stats, &taps.sub[1])

	scaleDecoderOutputForEnvelopeRecovery(out[:frameSamples])
	applyEnvelopeRecovery(out[:frameSamples], &stats)
	return taps, nil
}

func (d *Decoder) decodeEnhancedSubframeWithTaps(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	stats *envelopeRecoveryStats,
	taps *phase3bjSubframeTaps,
) {
	taps.tInt = tInt
	taps.tFrac = tFrac
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.DecodeWithLogCorrections(gain.Indices{GA: GA, GB: GB}, &c, 26, 14)
	gcSigned := float64(gcMant) * math.Exp2(float64(gcExp)-14)
	gcAbs := math.Abs(gcSigned)
	if gcAbs > stats.gcMax {
		stats.gcMax = gcAbs
	}
	taps.gp = math.Abs(float64(gpQ14) / 16384.0)
	taps.gc = gcAbs

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	var pitchE, fixedE float64
	for n := 0; n < subframeLen; n++ {
		pitchPart := float64(gpQ14) * float64(v[n]) / 16384.0
		fixedPart := gcSigned * float64(c[n]) / 8192.0
		pitchE += pitchPart * pitchPart
		fixedE += fixedPart * fixedPart
		stats.pitchE += pitchPart * pitchPart
		stats.fixedE += fixedPart * fixedPart
		stats.uE += float64(u[n]) * float64(u[n])
		stats.sE += float64(s[n]) * float64(s[n])
	}
	taps.pitchRMS = math.Sqrt(pitchE / subframeLen)
	taps.fixedRMS = math.Sqrt(fixedE / subframeLen)
	taps.uRMS = envelopeRMS(u[:])
	taps.sRMS = envelopeRMS(s[:])
	taps.spfRMS = envelopeRMS(sPf[:])
	taps.hpRMS = envelopeRMS(hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func phase3bjSamples(ref, enhanced []int16, taps []phase3bjFrameTaps) []phase3bjSample {
	frames := len(ref) / frameSamples
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	if tf := len(taps); tf < frames {
		frames = tf
	}
	out := make([]phase3bjSample, 0, frames*2)
	for frame := 0; frame < frames; frame++ {
		for sf := 0; sf < 2; sf++ {
			off := frame*frameSamples + sf*subframeLen
			refSub := ref[off : off+subframeLen]
			enhSub := enhanced[off : off+subframeLen]
			refRMS := envelopeRMS(refSub)
			localRMS := envelopeRMS(enhSub)
			if refRMS < 500 || localRMS < 1 {
				continue
			}
			out = append(out, phase3bjSample{
				frame:    frame,
				sf:       sf,
				refRMS:   refRMS,
				localRMS: localRMS,
				features: phase3bjSubframeFeatures(taps[frame], sf, enhSub),
			})
		}
	}
	return out
}

func phase3bjSubframeFeatures(tap phase3bjFrameTaps, sf int, enhancedSub []int16) []float64 {
	sub := tap.sub[sf]
	ga := tap.frame.GA1
	if sf == 1 {
		ga = tap.frame.GA2
	}
	hasGA036 := 0.0
	if ga == 0 || ga == 3 || ga == 6 {
		hasGA036 = 1
	}
	fracNeg, fracZero, fracPos := 0.0, 0.0, 0.0
	switch sub.tFrac {
	case -1:
		fracNeg = 1
	case 0:
		fracZero = 1
	case 1:
		fracPos = 1
	}
	shortPitch := 0.0
	if sub.tInt < subframeLen {
		shortPitch = 1
	}
	return []float64{
		1,
		float64(sf),
		math.Log1p(envelopeRMS(enhancedSub)),
		math.Log1p(sub.uRMS),
		math.Log1p(sub.sRMS),
		math.Log1p(sub.spfRMS),
		math.Log1p(sub.hpRMS),
		math.Log1p(sub.gp),
		math.Log1p(sub.gc),
		safeRatioFloat64(sub.pitchRMS, sub.uRMS),
		safeRatioFloat64(sub.fixedRMS, sub.uRMS),
		safeRatioFloat64(sub.sRMS, sub.uRMS),
		safeRatioFloat64(sub.spfRMS, sub.sRMS),
		safeRatioFloat64(sub.hpRMS, sub.spfRMS),
		shortPitch,
		fracNeg,
		fracZero,
		fracPos,
		hasGA036,
	}
}

func phase3bjSplitSamples(samples []phase3bjSample) (train, holdout []phase3bjSample) {
	for _, s := range samples {
		if (s.frame*2+s.sf)%2 == 0 {
			train = append(train, s)
		} else {
			holdout = append(holdout, s)
		}
	}
	return train, holdout
}

func phase3bjFitRidge(t *testing.T, samples []phase3bjSample, lambda float64) []float64 {
	t.Helper()
	if len(samples) == 0 {
		t.Fatalf("no samples for enhanced-tap subframe envelope estimator")
	}
	n := len(samples[0].features)
	xtx := make([][]float64, n)
	for i := range xtx {
		xtx[i] = make([]float64, n+1)
	}
	for _, s := range samples {
		y := math.Log(s.refRMS / s.localRMS)
		for i, xi := range s.features {
			xtx[i][n] += xi * y
			for j, xj := range s.features {
				xtx[i][j] += xi * xj
			}
		}
	}
	for i := 1; i < n; i++ {
		xtx[i][i] += lambda
	}
	return phase3bbSolve(t, xtx)
}

func phase3bjLogDataset(t *testing.T, label string, samples []phase3bjSample, model []float64) {
	t.Helper()
	var absLogErr, absRatioErr float64
	for _, s := range samples {
		want := s.refRMS / s.localRMS
		pred := phase3bjPredictScale(model, s.features, 0.25, 3.0)
		absLogErr += math.Abs(math.Log(pred) - math.Log(want))
		absRatioErr += math.Abs(pred - want)
	}
	if len(samples) == 0 {
		t.Logf("%s estimator: no samples", label)
		return
	}
	t.Logf("%s estimator: n=%d meanAbsLogErr=%.3f meanAbsScaleErr=%.3f",
		label, len(samples), absLogErr/float64(len(samples)), absRatioErr/float64(len(samples)))
}

func phase3bjLogOutput(t *testing.T, label string, ds phase3bjDataset, model []float64) {
	t.Helper()
	base := blackboxMeasure(ds.ref, ds.enhanced, 40)
	baseEnv := phase3pEnvelopeCompare(ds.ref, ds.enhanced)
	t.Logf("%s output: baseline gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d clipped=%d",
		label, base.globalSNR, base.segSNR, base.corr, baseEnv.ratioMedian, baseEnv.lowRatioFrames, phase3xCountClipped(ds.enhanced))
	for _, cfg := range []struct {
		name     string
		minRMS   float64
		minScale float64
		maxScale float64
	}{
		{name: "enhsub_min50_b0.25_3.0", minRMS: 50, minScale: 0.25, maxScale: 3.0},
		{name: "enhsub_min200_b0.25_3.0", minRMS: 200, minScale: 0.25, maxScale: 3.0},
		{name: "enhsub_min500_b0.25_3.0", minRMS: 500, minScale: 0.25, maxScale: 3.0},
		{name: "enhsub_min200_b0.50_2.0", minRMS: 200, minScale: 0.50, maxScale: 2.0},
	} {
		out := phase3bjApplyModel(ds.enhanced, ds.taps, model, cfg.minRMS, cfg.minScale, cfg.maxScale)
		m := blackboxMeasure(ds.ref, out, 40)
		env := phase3pEnvelopeCompare(ds.ref, out)
		t.Logf("%s output: %-24s gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d clipped=%d deltaG=%+.2f deltaS=%+.2f",
			label, cfg.name, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames,
			phase3xCountClipped(out), m.globalSNR-base.globalSNR, m.segSNR-base.segSNR)
	}
}

func phase3bjApplyModel(enhanced []int16, taps []phase3bjFrameTaps, model []float64, minRMS, minScale, maxScale float64) []int16 {
	out := append([]int16(nil), enhanced...)
	frames := len(out) / frameSamples
	if tf := len(taps); tf < frames {
		frames = tf
	}
	for frame := 0; frame < frames; frame++ {
		for sf := 0; sf < 2; sf++ {
			off := frame*frameSamples + sf*subframeLen
			sub := out[off : off+subframeLen]
			if envelopeRMS(sub) < minRMS {
				continue
			}
			scale := phase3bjPredictScale(model, phase3bjSubframeFeatures(taps[frame], sf, sub), minScale, maxScale)
			for i, sample := range sub {
				out[off+i] = phase3bbScaleSample(sample, scale)
			}
		}
	}
	return out
}

func phase3bjPredictScale(model, features []float64, minScale, maxScale float64) float64 {
	var y float64
	for i, c := range model {
		y += c * features[i]
	}
	scale := math.Exp(y)
	if scale < minScale {
		return minScale
	}
	if scale > maxScale {
		return maxScale
	}
	return scale
}
