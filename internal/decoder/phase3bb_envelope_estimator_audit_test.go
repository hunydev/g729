package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3bbEnvelopeEstimatorAudit checks whether runtime-available local
// decoder taps can predict the missing frame envelope without using an
// oracle at decode time. FFmpeg is used only as an executable black-box to
// provide numeric training/evaluation labels for this diagnostic.
func TestPhase3bbEnvelopeEstimatorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ENVELOPE_ESTIMATOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ENVELOPE_ESTIMATOR_AUDIT=1 to run envelope estimator audit")
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
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	speechLocal, speechTaps := decodeG192WithTapsForEnvelopeAudit(t, bitData, speechFrames)
	speechSamples := phase3bbSamples(speechRef, speechLocal, speechTaps)

	train, holdout := phase3bbSplitSamples(speechSamples)
	model := phase3bbFitRidge(t, train, 1e-3)

	t.Logf("Phase 3bb envelope estimator audit")
	t.Logf("model trained from SPEECH.BIT first-half active frames: n=%d coeff=%s", len(train), phase3bbFormatCoefficients(model))
	phase3bbLogDataset(t, "SPEECH.BIT train", speechRef, speechLocal, speechTaps, train, model)
	phase3bbLogDataset(t, "SPEECH.BIT holdout", speechRef, speechLocal, speechTaps, holdout, model)
	phase3bbLogOutput(t, "SPEECH.BIT full", speechRef, speechLocal, speechTaps, model)
	phase3bbLogGrid(t, "SPEECH.BIT full", speechRef, speechLocal, speechTaps, model)

	for _, vector := range []string{"FIXED.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
		path := vectorPath(vector)
		ensureTestdataPresent(t, path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", vector, err)
		}
		frames := len(data) / bitstream.G192FrameBytes
		ref := phase3uFFmpegDecodeG192(t, data, frames, vector)
		local, taps := decodeG192WithTapsForEnvelopeAudit(t, data, frames)
		phase3bbLogOutput(t, vector, ref, local, taps, model)
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk envelope estimator audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	astLocal, astTaps := phase3ajDecodeRawWithTaps(t, raw, astFrames)
	astSamples := phase3bbSamples(astRef, astLocal, astTaps)
	phase3bbLogDataset(t, "Asterisk external", astRef, astLocal, astTaps, astSamples, model)
	phase3bbLogOutput(t, "Asterisk external", astRef, astLocal, astTaps, model)
	phase3bbLogGrid(t, "Asterisk external", astRef, astLocal, astTaps, model)

	astTrain, astHoldout := phase3bbSplitSamples(astSamples)
	combinedTrain := append(append([]phase3bbSample{}, train...), astTrain...)
	combinedModel := phase3bbFitRidge(t, combinedTrain, 1e-3)
	t.Logf("combined SPEECH.BIT+Asterisk model: train=%d holdout=%d coeff=%s",
		len(combinedTrain), len(holdout)+len(astHoldout), phase3bbFormatCoefficients(combinedModel))
	phase3bbLogOutput(t, "combined model SPEECH.BIT full", speechRef, speechLocal, speechTaps, combinedModel)
	phase3bbLogGrid(t, "combined model SPEECH.BIT full", speechRef, speechLocal, speechTaps, combinedModel)
	phase3bbLogDataset(t, "combined model Asterisk holdout", astRef, astLocal, astTaps, astHoldout, combinedModel)
	phase3bbLogOutput(t, "combined model Asterisk external", astRef, astLocal, astTaps, combinedModel)
	phase3bbLogGrid(t, "combined model Asterisk external", astRef, astLocal, astTaps, combinedModel)
}

type phase3bbSample struct {
	frame    int
	refRMS   float64
	localRMS float64
	features []float64
}

func phase3bbSamples(ref, local []int16, taps []Phase3DiagFrameTaps) []phase3bbSample {
	frames := len(ref) / frameSamples
	if lf := len(local) / frameSamples; lf < frames {
		frames = lf
	}
	if tf := len(taps); tf < frames {
		frames = tf
	}
	out := make([]phase3bbSample, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		refRMS := envelopeRMS(ref[off : off+frameSamples])
		localRMS := envelopeRMS(local[off : off+frameSamples])
		if refRMS < 500 || localRMS < 1 {
			continue
		}
		out = append(out, phase3bbSample{
			frame:    frame,
			refRMS:   refRMS,
			localRMS: localRMS,
			features: phase3bbFeatures(taps[frame]),
		})
	}
	return out
}

func phase3bbSplitSamples(samples []phase3bbSample) (train, holdout []phase3bbSample) {
	for _, s := range samples {
		if s.frame%2 == 0 {
			train = append(train, s)
		} else {
			holdout = append(holdout, s)
		}
	}
	return train, holdout
}

func phase3bbFeatures(tap Phase3DiagFrameTaps) []float64 {
	m := envelopeStageSummary(tap)
	hasGA036 := 0.0
	if phase3alHasGA036(tap.Frame) {
		hasGA036 = 1
	}
	hasShortPitch := 0.0
	if tap.Sub[0].TInt < 40 || tap.Sub[1].TInt < 40 {
		hasShortPitch = 1
	}
	anyFrac := 0.0
	if tap.Sub[0].TFrac != 0 || tap.Sub[1].TFrac != 0 {
		anyFrac = 1
	}
	bothFrac := 0.0
	if tap.Sub[0].TFrac != 0 && tap.Sub[1].TFrac != 0 {
		bothFrac = 1
	}
	l0High := 0.0
	if tap.Frame.L0 != 0 {
		l0High = 1
	}
	return []float64{
		1,
		math.Log1p(m.outRMS),
		math.Log1p(m.uRMS),
		math.Log1p(m.sRMS),
		math.Log1p(m.spfRMS),
		math.Log1p(m.hpRMS),
		math.Log1p(m.gpMax),
		math.Log1p(m.gcMax),
		safeRatioFloat64(m.fixedRMS, m.uRMS),
		safeRatioFloat64(m.pitchRMS, m.uRMS),
		safeRatioFloat64(m.sRMS, m.uRMS),
		safeRatioFloat64(m.spfRMS, m.sRMS),
		safeRatioFloat64(m.hpRMS, m.spfRMS),
		m.predictedAvgQ10 / 1024.0,
		m.logGainAvgQ10 / 1024.0,
		hasGA036,
		hasShortPitch,
		anyFrac,
		bothFrac,
		l0High,
	}
}

func phase3bbFitRidge(t *testing.T, samples []phase3bbSample, lambda float64) []float64 {
	t.Helper()
	if len(samples) == 0 {
		t.Fatalf("no samples for envelope estimator")
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

func phase3bbSolve(t *testing.T, a [][]float64) []float64 {
	t.Helper()
	n := len(a)
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(a[row][col]) > math.Abs(a[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(a[pivot][col]) < 1e-12 {
			t.Fatalf("singular estimator matrix at column %d", col)
		}
		a[col], a[pivot] = a[pivot], a[col]
		den := a[col][col]
		for j := col; j <= n; j++ {
			a[col][j] /= den
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			f := a[row][col]
			for j := col; j <= n; j++ {
				a[row][j] -= f * a[col][j]
			}
		}
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = a[i][n]
	}
	return out
}

func phase3bbPredictScale(model, features []float64) float64 {
	return phase3bbPredictScaleWithBounds(model, features, 0.25, 3.0)
}

func phase3bbPredictScaleWithBounds(model, features []float64, minScale, maxScale float64) float64 {
	var y float64
	for i := range model {
		y += model[i] * features[i]
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

func phase3bbApplyModel(local []int16, taps []Phase3DiagFrameTaps, model []float64) []int16 {
	return phase3bbApplyModelWithParams(local, taps, model, 500, 0.25, 3.0)
}

func phase3bbApplyModelWithParams(local []int16, taps []Phase3DiagFrameTaps, model []float64, minRMS, minScale, maxScale float64) []int16 {
	out := append([]int16(nil), local...)
	frames := len(out) / frameSamples
	if tf := len(taps); tf < frames {
		frames = tf
	}
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if envelopeRMS(out[off:off+frameSamples]) < minRMS {
			continue
		}
		scale := phase3bbPredictScaleWithBounds(model, phase3bbFeatures(taps[frame]), minScale, maxScale)
		for i := off; i < off+frameSamples; i++ {
			out[i] = phase3bbScaleSample(out[i], scale)
		}
	}
	return out
}

func phase3bbScaleSample(v int16, scale float64) int16 {
	x := math.Round(float64(v) * scale)
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return int16(x)
}

func phase3bbLogDataset(t *testing.T, label string, ref, local []int16, taps []Phase3DiagFrameTaps, samples []phase3bbSample, model []float64) {
	t.Helper()
	var absLogErr, sqLogErr float64
	var under05, over15 int
	for _, s := range samples {
		pred := phase3bbPredictScale(model, s.features)
		actual := s.refRMS / s.localRMS
		err := math.Log(pred / actual)
		absLogErr += math.Abs(err)
		sqLogErr += err * err
		ratioAfter := s.localRMS * pred / s.refRMS
		if ratioAfter < 0.5 {
			under05++
		}
		if ratioAfter > 1.5 {
			over15++
		}
	}
	if len(samples) == 0 {
		t.Logf("%s estimator: no active samples", label)
		return
	}
	t.Logf("%s estimator: n=%d meanAbsLogErr=%.3f rmsLogErr=%.3f postRatio<0.5=%d postRatio>1.5=%d",
		label, len(samples), absLogErr/float64(len(samples)), math.Sqrt(sqLogErr/float64(len(samples))), under05, over15)
	_ = ref
	_ = local
	_ = taps
}

func phase3bbLogOutput(t *testing.T, label string, ref, local []int16, taps []Phase3DiagFrameTaps, model []float64) {
	t.Helper()
	scaled := phase3bbApplyModel(local, taps, model)
	base := blackboxMeasure(ref, local, 40)
	baseEnv := phase3pEnvelopeCompare(ref, local)
	est := blackboxMeasure(ref, scaled, 40)
	estEnv := phase3pEnvelopeCompare(ref, scaled)
	t.Logf("%s output: production gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d clipped=%d",
		label, base.globalSNR, base.segSNR, base.corr, baseEnv.ratioMedian, baseEnv.lowRatioFrames, phase3xCountClipped(local))
	t.Logf("%s output: estimated  gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d clipped=%d",
		label, est.globalSNR, est.segSNR, est.corr, estEnv.ratioMedian, estEnv.lowRatioFrames, phase3xCountClipped(scaled))
}

func phase3bbLogGrid(t *testing.T, label string, ref, local []int16, taps []Phase3DiagFrameTaps, model []float64) {
	t.Helper()
	t.Logf("%s grid:", label)
	t.Logf("  %-8s %-6s %8s %8s %7s %8s %8s %7s", "minRMS", "max", "gSNR", "seg", "corr", "ratio", "low<.5", "clip")
	for _, minRMS := range []float64{100, 300, 500, 800} {
		for _, maxScale := range []float64{3, 4, 5, 6} {
			scaled := phase3bbApplyModelWithParams(local, taps, model, minRMS, 0.25, maxScale)
			m := blackboxMeasure(ref, scaled, 40)
			env := phase3pEnvelopeCompare(ref, scaled)
			t.Logf("  %-8.0f %-6.1f %8.2f %8.2f %7.3f %8.3f %8d %7d",
				minRMS, maxScale, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(scaled))
		}
	}
}

func phase3bbFormatCoefficients(coeff []float64) string {
	const names = "1,out,u,s,gc,fix/u,pit/u,s/u,ga036"
	_ = names
	out := ""
	for i, c := range coeff {
		if i > 0 {
			out += " "
		}
		out += phase3bbItoa(i) + "=" + phase3bbFormatFloat(c)
	}
	return out
}

func phase3bbItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func phase3bbFormatFloat(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	scaled := int64(math.Round(v * 1000))
	return sign + phase3bbItoa(int(scaled/1000)) + "." +
		string(byte('0'+(scaled/100)%10)) +
		string(byte('0'+(scaled/10)%10)) +
		string(byte('0'+scaled%10))
}
