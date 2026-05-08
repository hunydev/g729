package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3oFFmpegEnvelopeAudit_SPEECH compares the local decoder with
// FFmpeg executable black-box decode on the same SPEECH.BIT payload, then
// attaches local DecodeWithTaps gain/stage metrics to the worst active frames.
//
// FFmpeg is used only as an external executable. No external implementation
// source is inspected.
func TestPhase3oFFmpegEnvelopeAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FFMPEG_ENVELOPE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FFMPEG_ENVELOPE_AUDIT=1 to run local-vs-ffmpeg envelope audit")
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
	frames := len(bitData) / bitstream.G192FrameBytes
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}

	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, "speech-bit.g729")
	ffPath := filepath.Join(tmp, "speech-bit.ffmpeg.s16le")
	writeG192RawForEnvelopeAudit(t, bitData, frames, rawPath)
	ffmpegDecodeRawForEnvelopeAudit(t, rawPath, ffPath)

	ff := readPCM16LEForEnvelopeAudit(t, ffPath)
	if len(ff) > frames*frameSamples {
		ff = ff[:frames*frameSamples]
	}
	if len(ff) < frames*frameSamples {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(ff), frames*frameSamples)
	}

	local, taps := decodeG192WithTapsForEnvelopeAudit(t, bitData, frames)

	rows := make([]envelopeAuditFrame, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		ffFrame := ff[off : off+frameSamples]
		localFrame := local[off : off+frameSamples]
		ffRMS := envelopeRMS(ffFrame)
		localRMS := envelopeRMS(localFrame)
		ratio := 0.0
		if ffRMS > 0 {
			ratio = localRMS / ffRMS
		}
		rows = append(rows, envelopeAuditFrame{
			frame:    frame,
			ffRMS:    ffRMS,
			localRMS: localRMS,
			ratio:    ratio,
			snrVsFF:  envelopeSNRDB(ffFrame, localFrame),
			corrVsFF: envelopeCorr(ffFrame, localFrame),
			stages:   envelopeStageSummary(taps[frame]),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		ai := rows[i].ffRMS >= 500
		aj := rows[j].ffRMS >= 500
		if ai != aj {
			return ai
		}
		if rows[i].snrVsFF == rows[j].snrVsFF {
			return rows[i].ffRMS > rows[j].ffRMS
		}
		return rows[i].snrVsFF < rows[j].snrVsFF
	})

	summary := summarizeEnvelopeAudit(rows)
	t.Logf("Phase 3o FFmpeg envelope audit - SPEECH.BIT (%d frames)", frames)
	t.Logf("active frames ffRMS>=500: %d ; ratio median=%.3f mean=%.3f p10=%.3f p90=%.3f low<0.5=%d high>1.5=%d corr<0=%d corr<0.3=%d",
		summary.activeFrames, summary.ratioMedian, summary.ratioMean, summary.ratioP10, summary.ratioP90,
		summary.lowRatioFrames, summary.highRatioFrames, summary.negativeCorrFrames, summary.lowCorrFrames)
	t.Logf("active stage ratios: pitch/u=%.2f fixed/u=%.2f s/u=%.2f spf/s=%.2f hp/spf=%.2f out/hp=%.2f",
		summary.pitchToExcRatio, summary.fixedToExcRatio,
		summary.synthToExcRatio, summary.postfilterToSynthRatio, summary.hpToPostfilterRatio, summary.outputToHPRatio)
	t.Logf("%5s %9s %9s %8s %9s %8s %9s %9s %9s %9s %9s %9s %9s %7s %7s %9s %9s",
		"frame", "ffRMS", "localRMS", "ratio", "snrVsFF", "corr",
		"pRMS", "fRMS", "uRMS", "sRMS", "spfRMS", "hpRMS", "outRMS",
		"gpMax", "gcMax", "predAvg", "logGain")
	limit := 12
	if limit > len(rows) {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		r := rows[i]
		t.Logf("%5d %9.1f %9.1f %8.3f %9.2f %8.3f %9.1f %9.1f %9.1f %9.1f %9.1f %9.1f %9.1f %7.3f %7.3f %9.1f %9.1f",
			r.frame, r.ffRMS, r.localRMS, r.ratio, r.snrVsFF, r.corrVsFF,
			r.stages.pitchRMS, r.stages.fixedRMS, r.stages.uRMS,
			r.stages.sRMS, r.stages.spfRMS, r.stages.hpRMS, r.stages.outRMS,
			r.stages.gpMax, r.stages.gcMax, r.stages.predictedAvgQ10/1024.0, r.stages.logGainAvgQ10/1024.0)
	}
}

type envelopeAuditFrame struct {
	frame    int
	ffRMS    float64
	localRMS float64
	ratio    float64
	snrVsFF  float64
	corrVsFF float64
	stages   envelopeStageMetrics
}

type envelopeStageMetrics struct {
	pitchRMS        float64
	fixedRMS        float64
	uRMS            float64
	sRMS            float64
	spfRMS          float64
	hpRMS           float64
	outRMS          float64
	gpMax           float64
	gcMax           float64
	predictedAvgQ10 float64
	logGainAvgQ10   float64
}

type envelopeAuditSummary struct {
	activeFrames           int
	ratioMean              float64
	ratioMedian            float64
	ratioP10               float64
	ratioP90               float64
	lowRatioFrames         int
	highRatioFrames        int
	negativeCorrFrames     int
	lowCorrFrames          int
	pitchToExcRatio        float64
	fixedToExcRatio        float64
	synthToExcRatio        float64
	postfilterToSynthRatio float64
	hpToPostfilterRatio    float64
	outputToHPRatio        float64
}

func summarizeEnvelopeAudit(rows []envelopeAuditFrame) envelopeAuditSummary {
	var summary envelopeAuditSummary
	var ratios []float64
	var ratioSum float64
	var pitchSum, fixedSum, uSum, sSum, spfSum, hpSum, outSum float64
	for _, r := range rows {
		if r.ffRMS < 500 {
			continue
		}
		summary.activeFrames++
		ratios = append(ratios, r.ratio)
		ratioSum += r.ratio
		if r.ratio < 0.5 {
			summary.lowRatioFrames++
		}
		if r.ratio > 1.5 {
			summary.highRatioFrames++
		}
		if r.corrVsFF < 0 {
			summary.negativeCorrFrames++
		}
		if r.corrVsFF < 0.3 {
			summary.lowCorrFrames++
		}
		pitchSum += r.stages.pitchRMS
		fixedSum += r.stages.fixedRMS
		uSum += r.stages.uRMS
		sSum += r.stages.sRMS
		spfSum += r.stages.spfRMS
		hpSum += r.stages.hpRMS
		outSum += r.stages.outRMS
	}
	if summary.activeFrames == 0 {
		return summary
	}
	sort.Float64s(ratios)
	summary.ratioMean = ratioSum / float64(summary.activeFrames)
	summary.ratioMedian = envelopePercentile(ratios, 0.5)
	summary.ratioP10 = envelopePercentile(ratios, 0.1)
	summary.ratioP90 = envelopePercentile(ratios, 0.9)
	summary.pitchToExcRatio = safeRatioFloat64(pitchSum, uSum)
	summary.fixedToExcRatio = safeRatioFloat64(fixedSum, uSum)
	summary.synthToExcRatio = safeRatioFloat64(sSum, uSum)
	summary.postfilterToSynthRatio = safeRatioFloat64(spfSum, sSum)
	summary.hpToPostfilterRatio = safeRatioFloat64(hpSum, spfSum)
	summary.outputToHPRatio = safeRatioFloat64(outSum, hpSum)
	return summary
}

func envelopePercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func safeRatioFloat64(num, den float64) float64 {
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func writeG192RawForEnvelopeAudit(t *testing.T, g192 []byte, frames int, path string) {
	t.Helper()
	var raw bytes.Buffer
	r := bytes.NewReader(g192)
	var packed [bitstream.FrameBytes]byte
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, err)
		}
		raw.Write(packed[:])
	}
	if err := os.WriteFile(path, raw.Bytes(), 0o600); err != nil {
		t.Fatalf("write raw: %v", err)
	}
}

func ffmpegDecodeRawForEnvelopeAudit(t *testing.T, inPath, outPath string) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "g729", "-i", inPath,
		"-f", "s16le", "-ar", "8000", "-ac", "1", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg decode: %v\n%s", err, out)
	}
}

func readPCM16LEForEnvelopeAudit(t *testing.T, path string) []int16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pcm: %v", err)
	}
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i : 2*i+2]))
	}
	return out
}

func decodeG192WithTapsForEnvelopeAudit(t *testing.T, g192 []byte, frames int) ([]int16, []Phase3DiagFrameTaps) {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	taps := make([]Phase3DiagFrameTaps, frames)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(g192)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, err)
		}
		frameTaps, err := dec.DecodeWithTaps(packed[:])
		if err != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", f, err)
		}
		taps[f] = frameTaps
		copy(out[f*frameSamples:(f+1)*frameSamples], frameTaps.Output[:])
	}
	return out, taps
}

func envelopeStageSummary(taps Phase3DiagFrameTaps) envelopeStageMetrics {
	var pitchE, fixedE, uE, sE, spfE, hpE, outE float64
	var gpMax, gcMax float64
	var predSum, logGainSum float64
	for sf := 0; sf < 2; sf++ {
		sub := &taps.Sub[sf]
		gp := math.Abs(float64(sub.GpQ14) / 16384.0)
		gcSigned := float64(sub.GainTaps.GcMantQ14) * math.Exp2(float64(sub.GainTaps.GcExp)-14)
		gc := math.Abs(gcSigned)
		if gp > gpMax {
			gpMax = gp
		}
		if gc > gcMax {
			gcMax = gc
		}
		predSum += float64(sub.GainTaps.Predicted)
		logGainSum += float64(sub.GainTaps.LogGainDbQ10)
		for n := 0; n < subframeLen; n++ {
			pitchPart := float64(sub.GpQ14) * float64(sub.V[n]) / 16384.0
			fixedPart := gcSigned * float64(sub.C[n]) / 8192.0
			pitchE += pitchPart * pitchPart
			fixedE += fixedPart * fixedPart
			uE += squareFloat64(sub.U[n])
			sE += squareFloat64(sub.S[n])
			spfE += squareFloat64(sub.SPf[n])
			hpE += squareFloat64(sub.HpOut[n])
		}
	}
	for n := 0; n < frameSamples; n++ {
		outE += squareFloat64(taps.Output[n])
	}
	return envelopeStageMetrics{
		pitchRMS:        math.Sqrt(pitchE / frameSamples),
		fixedRMS:        math.Sqrt(fixedE / frameSamples),
		uRMS:            math.Sqrt(uE / frameSamples),
		sRMS:            math.Sqrt(sE / frameSamples),
		spfRMS:          math.Sqrt(spfE / frameSamples),
		hpRMS:           math.Sqrt(hpE / frameSamples),
		outRMS:          math.Sqrt(outE / frameSamples),
		gpMax:           gpMax,
		gcMax:           gcMax,
		predictedAvgQ10: predSum / 2,
		logGainAvgQ10:   logGainSum / 2,
	}
}

func squareFloat64(v int16) float64 {
	x := float64(v)
	return x * x
}

func envelopeRMS(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var e float64
	for _, v := range s {
		e += squareFloat64(v)
	}
	return math.Sqrt(e / float64(len(s)))
}

func envelopeSNRDB(ref, test []int16) float64 {
	if len(ref) != len(test) || len(ref) == 0 {
		return math.NaN()
	}
	var sigE, errE float64
	for i := range ref {
		s := float64(ref[i])
		e := s - float64(test[i])
		sigE += s * s
		errE += e * e
	}
	if sigE < 1 {
		return math.NaN()
	}
	if errE < 1 {
		return math.Inf(+1)
	}
	return 10 * math.Log10(sigE/errE)
}

func envelopeCorr(a, b []int16) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.NaN()
	}
	var meanA, meanB float64
	for i := range a {
		meanA += float64(a[i])
		meanB += float64(b[i])
	}
	meanA /= float64(len(a))
	meanB /= float64(len(b))
	var num, denA, denB float64
	for i := range a {
		da := float64(a[i]) - meanA
		db := float64(b[i]) - meanB
		num += da * db
		denA += da * da
		denB += db * db
	}
	den := math.Sqrt(denA * denB)
	if den <= 0 {
		return math.NaN()
	}
	return num / den
}
