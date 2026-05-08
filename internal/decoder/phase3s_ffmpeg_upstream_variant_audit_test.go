package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3sFFmpegUpstreamVariantAudit_SPEECH repeats the upstream local
// decoder variants against FFmpeg executable black-box decode of SPEECH.BIT.
// FFmpeg is used only as an external process; no implementation source is
// inspected.
func TestPhase3sFFmpegUpstreamVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FFMPEG_UPSTREAM_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FFMPEG_UPSTREAM_AUDIT=1 to run local-vs-ffmpeg upstream audit")
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

	variants := []phase3eVariant{
		{name: "production"},
		{name: "zero_adaptive_contribution", zeroAdaptive: true},
		{name: "zero_fixed_contribution", zeroFixed: true},
		{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true},
		{name: "pitch_gain_half", pitchScaleNum: 1, pitchScaleDen: 2},
		{name: "pitch_gain_double", pitchScaleNum: 2, pitchScaleDen: 1},
		{name: "fixed_gain_half", fixedExpDelta: -1},
		{name: "fixed_gain_double", fixedExpDelta: +1},
		{name: "fixed_gain_quad", fixedExpDelta: +2},
		{name: "fractional_acb_interpolation", useFractionalACB: true},
		{name: "force_pitch_frac_zero", forceTFracZero: true},
		{name: "flip_pitch_frac_sign", flipTFracSign: true},
		{name: "synth_identity_hp_x2", synthMode: phase3eSynthIdentityHP},
		{name: "synth_identity_pf_hp_x2", synthMode: phase3eSynthIdentityPFHP},
		{name: "reset_synth_each_frame", resetSynthEachFrame: true},
		{name: "reset_past_exc_each_frame", resetPastExcEachFrame: true},
		{name: "reset_gain_each_frame", resetGainEachFrame: true},
		{name: "reset_lsp_each_frame", resetLSPEachFrame: true},
	}

	production := phase3eDecodeVariant(t, bitData, frames, phase3eVariant{name: "production"})
	productionViaDecode := decodeVariant(t, bitData, frames, nil, nil)
	if !phase3eEqualPCM(production, productionViaDecode) {
		t.Fatalf("phase3s production variant diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3s FFmpeg upstream variant audit - SPEECH.BIT (%d frames)", frames)
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
			out = phase3eDecodeVariant(t, bitData, frames, v)
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
	t.Logf("verdict: %s", phase3sUpstreamFFmpegVerdict(rows[0], best, bestCorr, bestEnvelope))
}

func phase3sUpstreamFFmpegVerdict(prod, best, bestCorr, bestEnvelope struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}) string {
	if best.name != prod.name && best.m.globalSNR-prod.m.globalSNR > 1.0 {
		return "upstream variant materially improves FFmpeg-referenced SNR; inspect " + best.name
	}
	if bestCorr.name != prod.name && bestCorr.m.corr-prod.m.corr > 0.05 {
		return "upstream variant materially improves FFmpeg-referenced correlation; inspect " + bestCorr.name
	}
	if bestEnvelope.name != prod.name &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 &&
		bestEnvelope.m.globalSNR >= prod.m.globalSNR-0.25 &&
		bestEnvelope.m.corr >= prod.m.corr-0.02 {
		return "upstream variant materially reduces low-envelope frames; inspect " + bestEnvelope.name
	}
	if bestEnvelope.name != prod.name &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 {
		return "envelope-only upstream variant reduces low-ratio frames but worsens waveform metrics; do not use as production fix"
	}
	return "no simple upstream scale/reset/removal variant beats production against FFmpeg black-box"
}
