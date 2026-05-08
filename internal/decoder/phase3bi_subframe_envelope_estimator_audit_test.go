package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3biSubframeEnvelopeEstimatorAudit checks whether runtime-available
// local decoder taps can predict the subframe envelope headroom identified by
// the subframe lag/RMS oracle. FFmpeg is used only as an executable black-box
// decoder for numeric training/evaluation labels.
func TestPhase3biSubframeEnvelopeEstimatorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_SUBFRAME_ENVELOPE_ESTIMATOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_SUBFRAME_ENVELOPE_ESTIMATOR_AUDIT=1 to run subframe envelope-estimator audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	speech := phase3biLoadG192Dataset(t, "SPEECH.BIT")
	train, holdout := phase3biSplitSamples(speech.samples)
	model := phase3biFitRidge(t, train, 1e-3)

	t.Logf("Phase 3bi subframe envelope estimator audit")
	t.Logf("SPEECH.BIT model: train=%d holdout=%d coeff=%s", len(train), len(holdout), phase3bbFormatCoefficients(model))
	phase3biLogDataset(t, "SPEECH.BIT holdout", holdout, model)
	phase3biLogOutput(t, "SPEECH.BIT full", speech, model)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk subframe envelope estimator audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	asterisk := phase3biLoadRawDataset(t, "Asterisk", rawPath)
	phase3biLogDataset(t, "Asterisk external with SPEECH model", asterisk.samples, model)
	phase3biLogOutput(t, "Asterisk external with SPEECH model", asterisk, model)

	astTrain, astHoldout := phase3biSplitSamples(asterisk.samples)
	combinedTrain := append(append([]phase3biSample{}, train...), astTrain...)
	combinedModel := phase3biFitRidge(t, combinedTrain, 1e-3)
	t.Logf("combined SPEECH.BIT+Asterisk model: train=%d holdout=%d coeff=%s",
		len(combinedTrain), len(holdout)+len(astHoldout), phase3bbFormatCoefficients(combinedModel))
	phase3biLogOutput(t, "combined model SPEECH.BIT full", speech, combinedModel)
	phase3biLogDataset(t, "combined model Asterisk holdout", astHoldout, combinedModel)
	phase3biLogOutput(t, "combined model Asterisk external", asterisk, combinedModel)
}

type phase3biDataset struct {
	label    string
	ref      []int16
	enhanced []int16
	taps     []Phase3DiagFrameTaps
	samples  []phase3biSample
}

type phase3biSample struct {
	frame    int
	sf       int
	refRMS   float64
	localRMS float64
	features []float64
}

func phase3biLoadG192Dataset(t *testing.T, name string) phase3biDataset {
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
	enhanced := phase3rDecodeRawEnhanced(t, rawPath, frames)
	_, taps := decodeG192WithTapsForEnvelopeAudit(t, data, frames)
	return phase3biMakeDataset(name, ref, enhanced, taps)
}

func phase3biLoadRawDataset(t *testing.T, label, rawPath string) phase3biDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, label)
	enhanced := phase3rDecodeRawEnhanced(t, rawPath, frames)
	_, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	return phase3biMakeDataset(label, ref, enhanced, taps)
}

func phase3biMakeDataset(label string, ref, enhanced []int16, taps []Phase3DiagFrameTaps) phase3biDataset {
	samples := phase3biSamples(ref, enhanced, taps)
	return phase3biDataset{label: label, ref: ref, enhanced: enhanced, taps: taps, samples: samples}
}

func phase3biSamples(ref, enhanced []int16, taps []Phase3DiagFrameTaps) []phase3biSample {
	frames := len(ref) / frameSamples
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	if tf := len(taps); tf < frames {
		frames = tf
	}
	out := make([]phase3biSample, 0, frames*2)
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
			out = append(out, phase3biSample{
				frame:    frame,
				sf:       sf,
				refRMS:   refRMS,
				localRMS: localRMS,
				features: phase3biSubframeFeatures(taps[frame], sf, enhSub),
			})
		}
	}
	return out
}

func phase3biSubframeFeatures(tap Phase3DiagFrameTaps, sf int, enhancedSub []int16) []float64 {
	sub := &tap.Sub[sf]
	uRMS := envelopeRMS(sub.U[:])
	sRMS := envelopeRMS(sub.S[:])
	spfRMS := envelopeRMS(sub.SPf[:])
	hpRMS := envelopeRMS(sub.HpOut[:])
	outRMS := envelopeRMS(enhancedSub)

	gp := math.Abs(float64(sub.GpQ14) / 16384.0)
	gcSigned := float64(sub.GainTaps.GcMantQ14) * math.Exp2(float64(sub.GainTaps.GcExp)-14)
	gc := math.Abs(gcSigned)
	var pitchE, fixedE float64
	for n := 0; n < subframeLen; n++ {
		pitchPart := float64(sub.GpQ14) * float64(sub.V[n]) / 16384.0
		fixedPart := gcSigned * float64(sub.C[n]) / 8192.0
		pitchE += pitchPart * pitchPart
		fixedE += fixedPart * fixedPart
	}

	ga := tap.Frame.GA1
	if sf == 1 {
		ga = tap.Frame.GA2
	}
	hasGA036 := 0.0
	if ga == 0 || ga == 3 || ga == 6 {
		hasGA036 = 1
	}
	fracNeg, fracZero, fracPos := 0.0, 0.0, 0.0
	switch sub.TFrac {
	case -1:
		fracNeg = 1
	case 0:
		fracZero = 1
	case 1:
		fracPos = 1
	}
	shortPitch := 0.0
	if sub.TInt < subframeLen {
		shortPitch = 1
	}

	return []float64{
		1,
		float64(sf),
		math.Log1p(outRMS),
		math.Log1p(uRMS),
		math.Log1p(sRMS),
		math.Log1p(spfRMS),
		math.Log1p(hpRMS),
		math.Log1p(gp),
		math.Log1p(gc),
		safeRatioFloat64(math.Sqrt(pitchE/subframeLen), uRMS),
		safeRatioFloat64(math.Sqrt(fixedE/subframeLen), uRMS),
		safeRatioFloat64(sRMS, uRMS),
		safeRatioFloat64(spfRMS, sRMS),
		safeRatioFloat64(hpRMS, spfRMS),
		float64(sub.GainTaps.Predicted) / 1024.0,
		float64(sub.GainTaps.LogGainDbQ10) / 1024.0,
		shortPitch,
		fracNeg,
		fracZero,
		fracPos,
		hasGA036,
	}
}

func phase3biSplitSamples(samples []phase3biSample) (train, holdout []phase3biSample) {
	for _, s := range samples {
		if (s.frame*2+s.sf)%2 == 0 {
			train = append(train, s)
		} else {
			holdout = append(holdout, s)
		}
	}
	return train, holdout
}

func phase3biFitRidge(t *testing.T, samples []phase3biSample, lambda float64) []float64 {
	t.Helper()
	if len(samples) == 0 {
		t.Fatalf("no samples for subframe envelope estimator")
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

func phase3biLogDataset(t *testing.T, label string, samples []phase3biSample, model []float64) {
	t.Helper()
	var absLogErr, absRatioErr float64
	var clippedLow, clippedHigh int
	for _, s := range samples {
		wantLog := math.Log(s.refRMS / s.localRMS)
		pred := phase3biPredictScale(model, s.features, 0.25, 3.0)
		gotLog := math.Log(pred)
		absLogErr += math.Abs(gotLog - wantLog)
		absRatioErr += math.Abs(pred - s.refRMS/s.localRMS)
		if pred <= 0.2501 {
			clippedLow++
		}
		if pred >= 2.999 {
			clippedHigh++
		}
	}
	if len(samples) == 0 {
		t.Logf("%s estimator: no samples", label)
		return
	}
	t.Logf("%s estimator: n=%d meanAbsLogErr=%.3f meanAbsScaleErr=%.3f clippedLow=%d clippedHigh=%d",
		label, len(samples), absLogErr/float64(len(samples)), absRatioErr/float64(len(samples)), clippedLow, clippedHigh)
}

func phase3biLogOutput(t *testing.T, label string, ds phase3biDataset, model []float64) {
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
		{name: "subenv_min50_b0.25_3.0", minRMS: 50, minScale: 0.25, maxScale: 3.0},
		{name: "subenv_min200_b0.25_3.0", minRMS: 200, minScale: 0.25, maxScale: 3.0},
		{name: "subenv_min500_b0.25_3.0", minRMS: 500, minScale: 0.25, maxScale: 3.0},
		{name: "subenv_min200_b0.50_2.0", minRMS: 200, minScale: 0.50, maxScale: 2.0},
	} {
		out := phase3biApplyModel(ds.enhanced, ds.taps, model, cfg.minRMS, cfg.minScale, cfg.maxScale)
		m := blackboxMeasure(ds.ref, out, 40)
		env := phase3pEnvelopeCompare(ds.ref, out)
		t.Logf("%s output: %-24s gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d clipped=%d deltaG=%+.2f deltaS=%+.2f",
			label, cfg.name, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames,
			phase3xCountClipped(out), m.globalSNR-base.globalSNR, m.segSNR-base.segSNR)
	}
}

func phase3biApplyModel(enhanced []int16, taps []Phase3DiagFrameTaps, model []float64, minRMS, minScale, maxScale float64) []int16 {
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
			scale := phase3biPredictScale(model, phase3biSubframeFeatures(taps[frame], sf, sub), minScale, maxScale)
			for i, sample := range sub {
				out[off+i] = phase3bbScaleSample(sample, scale)
			}
		}
	}
	return out
}

func phase3biPredictScale(model, features []float64, minScale, maxScale float64) float64 {
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
