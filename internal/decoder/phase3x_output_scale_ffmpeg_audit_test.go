package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3xOutputScaleFFmpegAudit checks whether changing only final PCM
// scale improves agreement with FFmpeg executable black-box decode. This is
// diagnostic-only and does not alter decoder internals.
func TestPhase3xOutputScaleFFmpegAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_OUTPUT_SCALE_FFMPEG_AUDIT") != "1" {
		t.Skip("set G729_DECODER_OUTPUT_SCALE_FFMPEG_AUDIT=1 to run output scale audit")
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
	phase3xReport(t, "SPEECH.BIT", speechRef, decodeVariant(t, bitData, speechFrames, nil, nil))

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk output-scale audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3xReport(t, "Asterisk", astRef, phase3tDecodeRawVariant(t, raw, astFrames, phase3eVariant{name: "production"}))
}

func phase3xReport(t *testing.T, label string, ref, production []int16) {
	t.Helper()
	type scale struct {
		name string
		num  int
		den  int
	}
	scales := []scale{
		{name: "x1/4", num: 1, den: 4},
		{name: "x1/2", num: 1, den: 2},
		{name: "x3/4", num: 3, den: 4},
		{name: "x7/8", num: 7, den: 8},
		{name: "x1", num: 1, den: 1},
		{name: "x5/4", num: 5, den: 4},
		{name: "x3/2", num: 3, den: 2},
		{name: "x7/4", num: 7, den: 4},
		{name: "x2", num: 2, den: 1},
		{name: "x5/2", num: 5, den: 2},
		{name: "x3", num: 3, den: 1},
	}

	base := blackboxMeasure(ref, production, 40)
	baseEnv := phase3pEnvelopeCompare(ref, production)
	best := phase3xRow{name: "x1", m: base, env: baseEnv}
	bestSeg := best
	bestCorr := best
	bestEnv := best

	t.Logf("Phase 3x output scale FFmpeg audit - %s", label)
	t.Logf("%-8s %9s %7s %10s %10s %8s %9s %9s %9s",
		"scale", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-8s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-----", "---", "----", "------", "-----", "------", "--------", "-------", "-------")
	for _, s := range scales {
		out := production
		if s.num != s.den {
			out = phase3xScale(production, s.num, s.den)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		row := phase3xRow{name: s.name, m: m, env: env}
		clipped := phase3xCountClipped(out)
		t.Logf("%-8s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			s.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, clipped)
		if row.m.globalSNR > best.m.globalSNR {
			best = row
		}
		if row.m.segSNR > bestSeg.m.segSNR {
			bestSeg = row
		}
		if row.m.corr > bestCorr.m.corr {
			bestCorr = row
		}
		if row.env.lowRatioFrames < bestEnv.env.lowRatioFrames {
			bestEnv = row
		}
	}
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-base.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-base.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-base.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnv.name, bestEnv.env.lowRatioFrames, bestEnv.env.lowRatioFrames-baseEnv.lowRatioFrames)
	t.Logf("verdict: %s", phase3xOutputScaleVerdict(phase3xRow{name: "x1", m: base, env: baseEnv}, best, bestSeg, bestCorr, bestEnv))
}

type phase3xRow struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}

func phase3xScale(in []int16, num, den int) []int16 {
	out := make([]int16, len(in))
	for i, s := range in {
		v := int64(s) * int64(num)
		if v >= 0 {
			v += int64(den / 2)
		} else {
			v -= int64(den / 2)
		}
		v /= int64(den)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}

func phase3xCountClipped(samples []int16) int {
	var clipped int
	for _, s := range samples {
		if s == 32767 || s == -32768 {
			clipped++
		}
	}
	return clipped
}

func phase3xOutputScaleVerdict(base, best, bestSeg, bestCorr, bestEnv phase3xRow) string {
	if best.name != base.name && best.m.globalSNR-base.m.globalSNR > 1.0 && best.m.segSNR >= base.m.segSNR-0.25 && best.m.corr >= base.m.corr-0.02 {
		return "final output scale materially improves global SNR without damaging segmental/correlation; inspect " + best.name
	}
	if bestSeg.name != base.name && bestSeg.m.segSNR-base.m.segSNR > 1.0 && bestSeg.m.globalSNR >= base.m.globalSNR-0.25 && bestSeg.m.corr >= base.m.corr-0.02 {
		return "final output scale materially improves segmental SNR without damaging global/correlation; inspect " + bestSeg.name
	}
	if bestCorr.name != base.name && bestCorr.m.corr-base.m.corr > 0.05 && bestCorr.m.globalSNR >= base.m.globalSNR-0.25 {
		return "final output scale materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnv.name != base.name && base.env.lowRatioFrames-bestEnv.env.lowRatioFrames > base.env.activeFrames/10 {
		return "output-scale-only variant reduces low-ratio frames but is not waveform-safe"
	}
	return "no final output scale variant is production-safe"
}
