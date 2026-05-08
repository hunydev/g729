package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3tAsteriskVariantAudit compares simple local decoder upstream
// variants against FFmpeg executable black-box decode on an external
// Asterisk-origin raw G.729 payload. FFmpeg is used only as an external
// process; no implementation source is inspected.
func TestPhase3tAsteriskVariantAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ASTERISK_VARIANT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ASTERISK_VARIANT_AUDIT=1 to run Asterisk variant audit")
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
	totalSamples := frames * frameSamples
	if len(ref) > totalSamples {
		ref = ref[:totalSamples]
	}
	if len(ref) < totalSamples {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(ref), totalSamples)
	}

	variants := []phase3eVariant{
		{name: "production"},
		{name: "zero_adaptive_contribution", zeroAdaptive: true},
		{name: "zero_fixed_contribution", zeroFixed: true},
		{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true},
		{name: "pitch_gain_half", pitchScaleNum: 1, pitchScaleDen: 2},
		{name: "pitch_gain_double", pitchScaleNum: 2, pitchScaleDen: 1},
		{name: "fixed_gain_half", fixedExpDelta: -1},
		{name: "fixed_gain_half_no_fcb_enh", fixedExpDelta: -1, noFCBEnhancement: true},
		{name: "fixed_gain_half_reset_synth", fixedExpDelta: -1, resetSynthEachFrame: true},
		{name: "fixed_gain_half_reset_gain", fixedExpDelta: -1, resetGainEachFrame: true},
		{name: "fixed_gain_half_reset_lsp", fixedExpDelta: -1, resetLSPEachFrame: true},
		{name: "fixed_gain_double", fixedExpDelta: +1},
		{name: "fixed_gain_quad", fixedExpDelta: +2},
		{name: "fractional_acb_interpolation", useFractionalACB: true},
		{name: "force_pitch_frac_zero", forceTFracZero: true},
		{name: "flip_pitch_frac_sign", flipTFracSign: true},
		{name: "reset_synth_each_frame", resetSynthEachFrame: true},
		{name: "reset_past_exc_each_frame", resetPastExcEachFrame: true},
		{name: "reset_gain_each_frame", resetGainEachFrame: true},
		{name: "reset_lsp_each_frame", resetLSPEachFrame: true},
	}

	production := phase3tDecodeRawVariant(t, raw, frames, variants[0])
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3t Asterisk upstream variant audit (%d frames)", frames)
	t.Logf("baseline production vs FFmpeg: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR,
		prodEnv.ratioMedian, prodEnv.lowRatioFrames, prodEnv.lowCorrFrames)
	t.Logf("")
	t.Logf("%-30s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3", "bestSNR")
	t.Logf("%-30s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------", "-------")
	for _, v := range variants {
		out := production
		if v.name != "production" {
			out = phase3tDecodeRawVariant(t, raw, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, row{name: v.name, m: m, env: env})
		t.Logf("%-30s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d %9.2f",
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
	t.Logf("verdict: %s", phase3tAsteriskVariantVerdict(rows[0], best, bestCorr, bestEnvelope))
}

func phase3tDecodeRawVariant(t *testing.T, raw []byte, frames int, variant phase3eVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		if variant.resetSynthEachFrame {
			dec.syn.Reset()
		}
		if variant.resetPastExcEachFrame {
			dec.pastExc = [pastExcLen]int16{}
			dec.prevGpQ14 = 0
		}
		if variant.resetGainEachFrame {
			dec.gn.Reset()
		}
		if variant.resetLSPEachFrame {
			dec.lsp.Reset()
		}
		start := f * bitstream.FrameBytes
		if err := dec.decodeFramePhase3eVariant(raw[start:start+bitstream.FrameBytes], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3eVariant[%s] raw frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func phase3tAsteriskVariantVerdict(prod, best, bestCorr, bestEnvelope struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}) string {
	if best.name != prod.name && best.m.globalSNR-prod.m.globalSNR > 1.0 {
		return "Asterisk variant materially improves FFmpeg-referenced SNR; inspect " + best.name
	}
	if bestCorr.name != prod.name && bestCorr.m.corr-prod.m.corr > 0.05 {
		return "Asterisk variant materially improves FFmpeg-referenced correlation; inspect " + bestCorr.name
	}
	if bestEnvelope.name != prod.name &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 &&
		bestEnvelope.m.globalSNR >= prod.m.globalSNR-0.25 &&
		bestEnvelope.m.corr >= prod.m.corr-0.02 {
		return "Asterisk variant materially reduces low-envelope frames; inspect " + bestEnvelope.name
	}
	if bestEnvelope.name != prod.name &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 {
		return "envelope-only Asterisk variant reduces low-ratio frames but worsens waveform metrics; do not use as production fix"
	}
	return "no simple Asterisk scale/reset/removal variant beats production against FFmpeg black-box"
}
