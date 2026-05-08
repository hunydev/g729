package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3pFFmpegACBVariantAudit_SPEECH compares local adaptive-codebook
// variants against FFmpeg executable black-box decode of the same SPEECH.BIT
// payload. FFmpeg is used only as an external process; no implementation
// source is inspected.
func TestPhase3pFFmpegACBVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FFMPEG_ACB_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FFMPEG_ACB_AUDIT=1 to run local-vs-ffmpeg ACB audit")
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
	ref := readPCM16LEForEnvelopeAudit(t, ffPath)
	if len(ref) > frames*frameSamples {
		ref = ref[:frames*frameSamples]
	}
	if len(ref) < frames*frameSamples {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(ref), frames*frameSamples)
	}

	variants := []phase3hVariant{
		{name: "production"},
		{name: "acb_fractional_interpolation", mode: phase3hACBFractionalInterpolation},
		{name: "acb_delay_minus_1", mode: phase3hACBDelayMinus1},
		{name: "acb_delay_plus_1", mode: phase3hACBDelayPlus1},
		{name: "acb_frac_sign_flip", mode: phase3hACBFracSignFlip},
		{name: "acb_frac_ignore_integer", mode: phase3hACBFracIgnoreInteger},
		{name: "acb_frac_phase_swap", mode: phase3hACBFracPhaseSwap},
		{name: "acb_frac_neg_no_k_adj", mode: phase3hACBFracNegNoKAdjust},
		{name: "acb_frac_pos_k_minus_1", mode: phase3hACBFracPosKMinus1},
		{name: "acb_short_no_periodic", mode: phase3hACBShortNoPeriodic},
	}

	production := phase3hDecodeVariant(t, bitData, frames, variants[0])
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3p FFmpeg ACB variant audit - SPEECH.BIT (%d frames)", frames)
	t.Logf("baseline production vs FFmpeg: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR,
		prodEnv.ratioMedian, prodEnv.lowRatioFrames, prodEnv.lowCorrFrames)
	t.Logf("")
	t.Logf("%-28s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3", "bestSNR")
	t.Logf("%-28s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3hACBProduction {
			out = phase3hDecodeVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, row{name: v.name, m: m, env: env})
		t.Logf("%-28s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d %9.2f",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames, m.bestSNR)
	}

	best := rows[0]
	bestCorr := rows[0]
	bestEnvelope := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
		if r.env.lowRatioFrames < bestEnvelope.env.lowRatioFrames {
			bestEnvelope = r
		}
	}
	t.Logf("")
	t.Logf("best by gSNR@0: %s %.2f dB (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("best by low-ratio count: %s %d (delta=%+d)", bestEnvelope.name, bestEnvelope.env.lowRatioFrames, bestEnvelope.env.lowRatioFrames-prodEnv.lowRatioFrames)
	t.Logf("verdict: %s", phase3pACBFFmpegVerdict(rows[0], best, bestCorr, bestEnvelope))
}

// TestPhase3pFFmpegACBVariantAudit_Asterisk applies the same ACB variant grid
// to the external Asterisk-origin raw payload. This specifically covers the
// nonzero pitch-fraction surface that the current local encoder SPEECH corpus
// does not exercise.
func TestPhase3pFFmpegACBVariantAudit_Asterisk(t *testing.T) {
	if os.Getenv("G729_DECODER_FFMPEG_ACB_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FFMPEG_ACB_AUDIT=1 to run local-vs-ffmpeg ACB audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Skipf("Asterisk payload unavailable: %v", err)
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes

	tmp := t.TempDir()
	ffPath := filepath.Join(tmp, "asterisk.ffmpeg.s16le")
	ffmpegDecodeRawForEnvelopeAudit(t, rawPath, ffPath)
	ref := readPCM16LEForEnvelopeAudit(t, ffPath)
	if len(ref) > frames*frameSamples {
		ref = ref[:frames*frameSamples]
	}
	if len(ref) < frames*frameSamples {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(ref), frames*frameSamples)
	}

	variants := phase3pACBVariants()
	production := phase3hDecodeRawVariant(t, raw, frames, variants[0])
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3p FFmpeg ACB variant audit - Asterisk (%d frames)", frames)
	t.Logf("baseline production vs FFmpeg: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR,
		prodEnv.ratioMedian, prodEnv.lowRatioFrames, prodEnv.lowCorrFrames)
	t.Logf("")
	t.Logf("%-28s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3", "bestSNR")
	t.Logf("%-28s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3hACBProduction {
			out = phase3hDecodeRawVariant(t, raw, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, row{name: v.name, m: m, env: env})
		t.Logf("%-28s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d %9.2f",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames, m.bestSNR)
	}

	best := rows[0]
	bestCorr := rows[0]
	bestEnvelope := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
		if r.env.lowRatioFrames < bestEnvelope.env.lowRatioFrames {
			bestEnvelope = r
		}
	}
	t.Logf("")
	t.Logf("best by gSNR@0: %s %.2f dB (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("best by low-ratio count: %s %d (delta=%+d)", bestEnvelope.name, bestEnvelope.env.lowRatioFrames, bestEnvelope.env.lowRatioFrames-prodEnv.lowRatioFrames)
	t.Logf("verdict: %s", phase3pACBFFmpegVerdict(rows[0], best, bestCorr, bestEnvelope))
}

func phase3pACBVariants() []phase3hVariant {
	return []phase3hVariant{
		{name: "production"},
		{name: "acb_fractional_interpolation", mode: phase3hACBFractionalInterpolation},
		{name: "acb_delay_minus_1", mode: phase3hACBDelayMinus1},
		{name: "acb_delay_plus_1", mode: phase3hACBDelayPlus1},
		{name: "acb_frac_sign_flip", mode: phase3hACBFracSignFlip},
		{name: "acb_frac_ignore_integer", mode: phase3hACBFracIgnoreInteger},
		{name: "acb_frac_phase_swap", mode: phase3hACBFracPhaseSwap},
		{name: "acb_frac_phase_minus_1", mode: phase3hACBFracPhaseMinus1},
		{name: "acb_frac_phase_plus_1", mode: phase3hACBFracPhasePlus1},
		{name: "acb_frac_neg_no_k_adj", mode: phase3hACBFracNegNoKAdjust},
		{name: "acb_frac_pos_k_minus_1", mode: phase3hACBFracPosKMinus1},
		{name: "acb_frac_forward_arm", mode: phase3hACBFracForwardArm},
		{name: "acb_short_no_periodic", mode: phase3hACBShortNoPeriodic},
	}
}

func phase3hDecodeRawVariant(t *testing.T, raw []byte, frames int, variant phase3hVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.decodeFramePhase3hVariant(raw[start:start+bitstream.FrameBytes], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3hVariant[%s] raw frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

type phase3pEnvelopeSummary struct {
	activeFrames   int
	ratioMedian    float64
	lowRatioFrames int
	lowCorrFrames  int
	negativeCorr   int
	meanFrameSNRDB float64
	medianFrameSNR float64
}

func phase3pEnvelopeCompare(ref, got []int16) phase3pEnvelopeSummary {
	frames := len(ref) / frameSamples
	if gf := len(got) / frameSamples; gf < frames {
		frames = gf
	}
	var summary phase3pEnvelopeSummary
	var ratios []float64
	var snrs []float64
	var snrSum float64
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		r := ref[off : off+frameSamples]
		g := got[off : off+frameSamples]
		refRMS := envelopeRMS(r)
		if refRMS < 500 {
			continue
		}
		gotRMS := envelopeRMS(g)
		ratio := gotRMS / refRMS
		corr := envelopeCorr(r, g)
		snr := envelopeSNRDB(r, g)
		summary.activeFrames++
		ratios = append(ratios, ratio)
		snrs = append(snrs, snr)
		snrSum += snr
		if ratio < 0.5 {
			summary.lowRatioFrames++
		}
		if corr < 0.3 {
			summary.lowCorrFrames++
		}
		if corr < 0 {
			summary.negativeCorr++
		}
	}
	if summary.activeFrames == 0 {
		return summary
	}
	sort.Float64s(ratios)
	sort.Float64s(snrs)
	summary.ratioMedian = envelopePercentile(ratios, 0.5)
	summary.meanFrameSNRDB = snrSum / float64(summary.activeFrames)
	summary.medianFrameSNR = envelopePercentile(snrs, 0.5)
	return summary
}

func phase3pACBFFmpegVerdict(prod, best, bestCorr, bestEnvelope struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}) string {
	if best.name != prod.name && best.m.globalSNR-prod.m.globalSNR > 1.0 {
		return "ACB variant materially improves FFmpeg-referenced SNR; inspect " + best.name
	}
	if bestCorr.name != prod.name && bestCorr.m.corr-prod.m.corr > 0.05 {
		return "ACB variant materially improves FFmpeg-referenced correlation; inspect " + bestCorr.name
	}
	if bestEnvelope.name != prod.name && prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > int(math.Ceil(float64(prod.env.activeFrames)*0.1)) {
		return "ACB variant materially reduces low-envelope frames; inspect " + bestEnvelope.name
	}
	return "no tested ACB variant beats production against FFmpeg black-box; continue gain/envelope and pitch-state coupling audits"
}
