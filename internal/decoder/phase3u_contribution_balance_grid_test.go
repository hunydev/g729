package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3uContributionBalanceGrid audits pitch/fixed contribution balance
// variants against FFmpeg executable black-box decode. It is diagnostic-only:
// the grid is intentionally coarse and must not be treated as a production
// tuning surface by itself.
func TestPhase3uContributionBalanceGrid(t *testing.T) {
	if os.Getenv("G729_DECODER_CONTRIBUTION_BALANCE_GRID") != "1" {
		t.Skip("set G729_DECODER_CONTRIBUTION_BALANCE_GRID=1 to run contribution balance grid")
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
	if speechFrames <= 0 {
		t.Fatalf("SPEECH frames reconciled to %d", speechFrames)
	}
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	phase3uRunGrid(t, "SPEECH.BIT", speechRef, func(v phase3eVariant) []int16 {
		return phase3eDecodeVariant(t, bitData, speechFrames, v)
	})

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk contribution grid skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3uRunGrid(t, "Asterisk", astRef, func(v phase3eVariant) []int16 {
		return phase3tDecodeRawVariant(t, raw, astFrames, v)
	})
}

type phase3uScale struct {
	name string
	num  int
	den  int
}

type phase3uRow struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}

func phase3uRunGrid(t *testing.T, label string, ref []int16, decode func(phase3eVariant) []int16) {
	t.Helper()
	pitchScales := []phase3uScale{
		{name: "p1/2", num: 1, den: 2},
		{name: "p3/4", num: 3, den: 4},
		{name: "p1", num: 1, den: 1},
		{name: "p5/4", num: 5, den: 4},
		{name: "p3/2", num: 3, den: 2},
	}
	fixedExpDeltas := []struct {
		name  string
		delta int
	}{
		{name: "f1/2", delta: -1},
		{name: "f1", delta: 0},
		{name: "f2", delta: +1},
		{name: "f4", delta: +2},
	}

	production := decode(phase3eVariant{name: "production"})
	prod := phase3uRow{
		name: "production",
		m:    blackboxMeasure(ref, production, 40),
		env:  phase3pEnvelopeCompare(ref, production),
	}
	best := prod
	bestCorr := prod
	bestSeg := prod
	bestEnvelope := prod

	t.Logf("Phase 3u contribution balance grid - %s", label)
	t.Logf("baseline: rms=%.2f gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d",
		prod.m.rms, prod.m.globalSNR, prod.m.segSNR, prod.m.corr, prod.env.ratioMedian, prod.env.lowRatioFrames)
	t.Logf("%-14s %9s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "bestSNR")
	t.Logf("%-14s %9s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "------", "-----", "------", "--------", "-------", "-------")
	for _, ps := range pitchScales {
		for _, fs := range fixedExpDeltas {
			name := ps.name + "_" + fs.name
			v := phase3eVariant{name: name}
			if ps.num != ps.den {
				v.pitchScaleNum = ps.num
				v.pitchScaleDen = ps.den
			}
			if fs.delta != 0 {
				v.fixedExpDelta = fs.delta
			}
			out := production
			if name != "p1_f1" {
				out = decode(v)
			}
			row := phase3uRow{
				name: name,
				m:    blackboxMeasure(ref, out, 40),
				env:  phase3pEnvelopeCompare(ref, out),
			}
			t.Logf("%-14s %9.2f %10.2f %10.2f %8.3f %9.3f %9d %9.2f",
				row.name, row.m.rms, row.m.globalSNR, row.m.segSNR, row.m.corr,
				row.env.ratioMedian, row.env.lowRatioFrames, row.m.bestSNR)
			if row.m.globalSNR > best.m.globalSNR {
				best = row
			}
			if row.m.corr > bestCorr.m.corr {
				bestCorr = row
			}
			if row.m.segSNR > bestSeg.m.segSNR {
				bestSeg = row
			}
			if row.env.lowRatioFrames < bestEnvelope.env.lowRatioFrames {
				bestEnvelope = row
			}
		}
	}
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prod.m.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-prod.m.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prod.m.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnvelope.name, bestEnvelope.env.lowRatioFrames, bestEnvelope.env.lowRatioFrames-prod.env.lowRatioFrames)
	t.Logf("verdict: %s", phase3uContributionVerdict(prod, best, bestSeg, bestCorr, bestEnvelope))
}

func phase3uContributionVerdict(prod, best, bestSeg, bestCorr, bestEnvelope phase3uRow) string {
	if best.name != prod.name && best.m.globalSNR-prod.m.globalSNR > 1.0 && best.m.segSNR >= prod.m.segSNR-0.25 && best.m.corr >= prod.m.corr-0.02 {
		return "contribution balance variant materially improves global SNR without damaging segmental/correlation; inspect " + best.name
	}
	if bestSeg.name != prod.name && bestSeg.m.segSNR-prod.m.segSNR > 1.0 && bestSeg.m.globalSNR >= prod.m.globalSNR-0.25 && bestSeg.m.corr >= prod.m.corr-0.02 {
		return "contribution balance variant materially improves segmental SNR without damaging global/correlation; inspect " + bestSeg.name
	}
	if bestCorr.name != prod.name && bestCorr.m.corr-prod.m.corr > 0.05 && bestCorr.m.globalSNR >= prod.m.globalSNR-0.25 {
		return "contribution balance variant materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnvelope.name != prod.name && prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 {
		return "envelope-only contribution balance variant reduces low-ratio frames but is not waveform-safe"
	}
	return "no coarse pitch/fixed contribution balance variant is production-safe"
}

func phase3uFFmpegDecodeG192(t *testing.T, g192 []byte, frames int, label string) []int16 {
	t.Helper()
	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, label+".g729")
	ffPath := filepath.Join(tmp, label+".ffmpeg.s16le")
	writeG192RawForEnvelopeAudit(t, g192, frames, rawPath)
	ffmpegDecodeRawForEnvelopeAudit(t, rawPath, ffPath)
	return phase3uReadFFmpegPCM(t, ffPath, frames)
}

func phase3uFFmpegDecodeRaw(t *testing.T, rawPath string, frames int, label string) []int16 {
	t.Helper()
	tmp := t.TempDir()
	ffPath := filepath.Join(tmp, label+".ffmpeg.s16le")
	ffmpegDecodeRawForEnvelopeAudit(t, rawPath, ffPath)
	return phase3uReadFFmpegPCM(t, ffPath, frames)
}

func phase3uReadFFmpegPCM(t *testing.T, ffPath string, frames int) []int16 {
	t.Helper()
	total := frames * frameSamples
	out := readPCM16LEForEnvelopeAudit(t, ffPath)
	if len(out) > total {
		out = out[:total]
	}
	if len(out) < total {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(out), total)
	}
	return out
}
