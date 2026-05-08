package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3beLagEstimatorAudit checks whether the per-frame lag oracle
// headroom can be recovered from runtime-available local features. FFmpeg is
// used only as an executable black-box decoder to produce numeric labels.
func TestPhase3beLagEstimatorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_LAG_ESTIMATOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_LAG_ESTIMATOR_AUDIT=1 to run lag estimator audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	speech := phase3beLoadG192LagDataset(t, "SPEECH.BIT")
	train, holdout := phase3beSplitLagSamples(speech.samples)
	model := phase3beFitRidge(t, train, 1e-2)

	t.Logf("Phase 3be lag estimator audit")
	t.Logf("SPEECH.BIT model: train=%d holdout=%d coeff=%s", len(train), len(holdout), phase3bbFormatCoefficients(model))
	phase3beLogDataset(t, "SPEECH.BIT train", train, model)
	phase3beLogDataset(t, "SPEECH.BIT holdout", holdout, model)
	phase3beLogOutput(t, "SPEECH.BIT", speech, model)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk lag estimator audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	asterisk := phase3beLoadRawLagDataset(t, "Asterisk", rawPath)
	phase3beLogDataset(t, "Asterisk external with SPEECH model", asterisk.samples, model)
	phase3beLogOutput(t, "Asterisk external with SPEECH model", asterisk, model)

	astTrain, astHoldout := phase3beSplitLagSamples(asterisk.samples)
	combinedTrain := append(append([]phase3beLagSample{}, train...), astTrain...)
	combinedModel := phase3beFitRidge(t, combinedTrain, 1e-2)
	t.Logf("combined SPEECH.BIT+Asterisk lag model: train=%d coeff=%s", len(combinedTrain), phase3bbFormatCoefficients(combinedModel))
	phase3beLogDataset(t, "combined model Asterisk holdout", astHoldout, combinedModel)
	phase3beLogOutput(t, "combined model SPEECH.BIT", speech, combinedModel)
	phase3beLogOutput(t, "combined model Asterisk", asterisk, combinedModel)
}

type phase3beLagDataset struct {
	label    string
	ref      []int16
	enhanced []int16
	taps     []Phase3DiagFrameTaps
	samples  []phase3beLagSample
}

type phase3beLagSample struct {
	frame    int
	lag      int
	features []float64
}

func phase3beLoadG192LagDataset(t *testing.T, name string) phase3beLagDataset {
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
	return phase3beMakeLagDataset(name, ref, enhanced, taps)
}

func phase3beLoadRawLagDataset(t *testing.T, label, rawPath string) phase3beLagDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, label)
	enhanced := phase3rDecodeRawEnhanced(t, rawPath, frames)
	_, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	return phase3beMakeLagDataset(label, ref, enhanced, taps)
}

func phase3beMakeLagDataset(label string, ref, enhanced []int16, taps []Phase3DiagFrameTaps) phase3beLagDataset {
	lags := phase3rBestFrameLags(ref, enhanced, 20)
	frames := len(lags)
	if tf := len(taps); tf < frames {
		frames = tf
	}
	samples := make([]phase3beLagSample, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if off+frameSamples > len(ref) || off+frameSamples > len(enhanced) {
			break
		}
		if envelopeRMS(ref[off:off+frameSamples]) < 500 || envelopeRMS(enhanced[off:off+frameSamples]) < 500 {
			continue
		}
		samples = append(samples, phase3beLagSample{
			frame:    frame,
			lag:      phase3beClampLag(lags[frame], -4, 4),
			features: phase3beLagFeatures(taps[frame], enhanced[off:off+frameSamples]),
		})
	}
	return phase3beLagDataset{label: label, ref: ref, enhanced: enhanced, taps: taps, samples: samples}
}

func phase3beLagFeatures(tap Phase3DiagFrameTaps, frame []int16) []float64 {
	f := phase3bbFeatures(tap)
	out := make([]float64, 0, len(f)+5)
	out = append(out, f...)
	out = append(out,
		phase3beZeroCrossRate(frame),
		phase3beBoundarySlope(frame),
		phase3beFrameSkew(frame),
		phase3beSubframeRMSRatio(frame),
		phase3beSignedPeakRatio(frame),
	)
	return out
}

func phase3beSplitLagSamples(samples []phase3beLagSample) (train, holdout []phase3beLagSample) {
	for _, s := range samples {
		if s.frame%2 == 0 {
			train = append(train, s)
		} else {
			holdout = append(holdout, s)
		}
	}
	return train, holdout
}

func phase3beFitRidge(t *testing.T, samples []phase3beLagSample, lambda float64) []float64 {
	t.Helper()
	if len(samples) == 0 {
		t.Fatalf("no samples for lag estimator")
	}
	n := len(samples[0].features)
	xtx := make([][]float64, n)
	for i := range xtx {
		xtx[i] = make([]float64, n+1)
	}
	for _, s := range samples {
		y := float64(s.lag)
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

func phase3bePredictLag(model, features []float64) int {
	var y float64
	for i, c := range model {
		y += c * features[i]
	}
	return phase3beClampLag(int(math.Round(y)), -4, 4)
}

func phase3beApplyLagModel(ds phase3beLagDataset, model []float64) []int16 {
	out := append([]int16(nil), ds.enhanced...)
	frames := len(out) / frameSamples
	if tf := len(ds.taps); tf < frames {
		frames = tf
	}
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if off+frameSamples > len(out) {
			break
		}
		features := phase3beLagFeatures(ds.taps[frame], out[off:off+frameSamples])
		lag := phase3bePredictLag(model, features)
		copy(out[off:off+frameSamples], phase3rShiftFrame(out[off:off+frameSamples], lag))
	}
	return out
}

func phase3beLogDataset(t *testing.T, label string, samples []phase3beLagSample, model []float64) {
	t.Helper()
	var exact, within1 int
	var absErr float64
	var nonzeroPred int
	for _, s := range samples {
		pred := phase3bePredictLag(model, s.features)
		if pred == s.lag {
			exact++
		}
		if absInt(pred-s.lag) <= 1 {
			within1++
		}
		if pred != 0 {
			nonzeroPred++
		}
		absErr += float64(absInt(pred - s.lag))
	}
	if len(samples) == 0 {
		t.Logf("%s lag estimator: no samples", label)
		return
	}
	t.Logf("%s lag estimator: n=%d exact=%.1f%% within1=%.1f%% meanAbsErr=%.2f nonzeroPred=%.1f%%",
		label, len(samples),
		100*float64(exact)/float64(len(samples)),
		100*float64(within1)/float64(len(samples)),
		absErr/float64(len(samples)),
		100*float64(nonzeroPred)/float64(len(samples)))
}

func phase3beLogOutput(t *testing.T, label string, ds phase3beLagDataset, model []float64) {
	t.Helper()
	base := blackboxMeasure(ds.ref, ds.enhanced, 40)
	lagged := phase3beApplyLagModel(ds, model)
	m := blackboxMeasure(ds.ref, lagged, 40)
	oracle := phase3rFrameLagOracle(ds.ref, ds.enhanced, 20)
	oracleM := blackboxMeasure(ds.ref, oracle, 40)
	t.Logf("%s output: base gSNR=%.2f seg=%.2f corr=%.3f ; estimated lag gSNR=%.2f seg=%.2f corr=%.3f ; oracle lag gSNR=%.2f seg=%.2f corr=%.3f",
		label, base.globalSNR, base.segSNR, base.corr,
		m.globalSNR, m.segSNR, m.corr,
		oracleM.globalSNR, oracleM.segSNR, oracleM.corr)
}

func phase3beClampLag(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func phase3beZeroCrossRate(samples []int16) float64 {
	if len(samples) < 2 {
		return 0
	}
	var count int
	prev := samples[0] >= 0
	for _, s := range samples[1:] {
		cur := s >= 0
		if cur != prev {
			count++
		}
		prev = cur
	}
	return float64(count) / float64(len(samples)-1)
}

func phase3beBoundarySlope(samples []int16) float64 {
	if len(samples) < 2 {
		return 0
	}
	return float64(samples[len(samples)-1]-samples[0]) / math.Max(1, envelopeRMS(samples))
}

func phase3beFrameSkew(samples []int16) float64 {
	r := math.Max(1, envelopeRMS(samples))
	var sum float64
	for _, s := range samples {
		x := float64(s) / r
		sum += x * x * x
	}
	return sum / float64(len(samples))
}

func phase3beSubframeRMSRatio(samples []int16) float64 {
	if len(samples) < frameSamples {
		return 1
	}
	return envelopeRMS(samples[:subframeLen]) / math.Max(1, envelopeRMS(samples[subframeLen:frameSamples]))
}

func phase3beSignedPeakRatio(samples []int16) float64 {
	var pos, neg int
	for _, s := range samples {
		if int(s) > pos {
			pos = int(s)
		}
		if -int(s) > neg {
			neg = -int(s)
		}
	}
	return float64(pos-neg) / math.Max(1, float64(pos+neg))
}
