package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3qFFmpegGainVariantAudit_SPEECH compares local gain reconstruction
// variants against FFmpeg executable black-box decode of the same SPEECH.BIT
// payload. FFmpeg is used only as an external process; no implementation
// source is inspected.
func TestPhase3qFFmpegGainVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FFMPEG_GAIN_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FFMPEG_GAIN_AUDIT=1 to run local-vs-ffmpeg gain audit")
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

	variants := []phase3jVariant{
		{name: "production"},
		{name: "gain_mirror_default", mode: phase3jGainMirrorDefault},
		{name: "gain_loggain_sat16", mode: phase3jGainLogGainSat16},
		{name: "gain_legacy_q12_build", mode: phase3jGainLegacyQ12Build},
		{name: "gain_ec_q26", mode: phase3jGainECQ26},
		{name: "gain_ec_q25", mode: phase3jGainECQ25},
		{name: "gain_ec_q27", mode: phase3jGainECQ27},
		{name: "gain_ec_q13", mode: phase3jGainECQ13},
		{name: "gain_gamma_q12", mode: phase3jGainGammaQ12},
		{name: "gain_gamma_q14", mode: phase3jGainGammaQ14},
		{name: "gain_ec25_gamma14", mode: phase3jGainEC25GammaQ14},
		{name: "gain_ec25_gamma12", mode: phase3jGainEC25GammaQ12},
		{name: "gain_ignore_gamma_log", mode: phase3jGainIgnoreGammaLog},
		{name: "gain_loggain_i32", mode: phase3jGainLogGainI32},
		{name: "gain_pred_i32", mode: phase3jGainPredictedI32},
		{name: "gain_pred_i32_ec27", mode: phase3jGainPredictedI32EC27},
		{name: "gain_update_10log", mode: phase3jGainUpdate10Log},
		{name: "gain_update_default", mode: phase3jGainUpdateDefault},
	}

	production := decodeVariant(t, bitData, frames, nil, nil)
	mirror := phase3jDecodeVariant(t, bitData, frames, variants[1])
	if !phase3eEqualPCM(production, mirror) {
		t.Fatalf("phase3q gain mirror diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3q FFmpeg gain variant audit - SPEECH.BIT (%d frames)", frames)
	t.Logf("baseline production vs FFmpeg: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR,
		prodEnv.ratioMedian, prodEnv.lowRatioFrames, prodEnv.lowCorrFrames)
	t.Logf("")
	t.Logf("%-26s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3", "bestSNR")
	t.Logf("%-26s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3jGainProduction {
			out = phase3jDecodeVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, row{name: v.name, m: m, env: env})
		t.Logf("%-26s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d %9.2f",
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
	t.Logf("verdict: %s", phase3qGainFFmpegVerdict(rows[0], best, bestCorr, bestEnvelope))

	for _, vector := range []string{"TAME.BIT", "FIXED.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
		path := vectorPath(vector)
		ensureTestdataPresent(t, path)
		vecData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", vector, err)
		}
		vecFrames := len(vecData) / bitstream.G192FrameBytes
		if vecFrames == 0 {
			continue
		}
		vecRef := phase3uFFmpegDecodeG192(t, vecData, vecFrames, vector)
		phase3qReportGainVariants(t, vector, vecRef,
			decodeVariant(t, vecData, vecFrames, nil, nil),
			func(v phase3jVariant) []int16 {
				return phase3jDecodeVariant(t, vecData, vecFrames, v)
			},
			variants)
	}

	astRawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(astRawPath)
	if err != nil {
		t.Logf("Asterisk gain variant audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, astRawPath, astFrames, "asterisk")
	phase3qReportGainVariants(t, "Asterisk", astRef,
		phase3tDecodeRawVariant(t, raw, astFrames, phase3eVariant{name: "production"}),
		func(v phase3jVariant) []int16 {
			return phase3qDecodeRawGainVariant(t, raw, astFrames, v)
		},
		variants)
}

func phase3qReportGainVariants(t *testing.T, label string, ref, production []int16, decode func(phase3jVariant) []int16, variants []phase3jVariant) {
	t.Helper()
	prodMetrics := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)

	type row struct {
		name string
		m    blackboxMetrics
		env  phase3pEnvelopeSummary
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3q FFmpeg gain variant audit - %s", label)
	t.Logf("baseline production vs FFmpeg: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodEnv.ratioMedian, prodEnv.lowRatioFrames, prodEnv.lowCorrFrames)
	t.Logf("%-26s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3", "bestSNR")
	t.Logf("%-26s %9s %7s %10s %10s %8s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3jGainProduction {
			out = decode(v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, row{name: v.name, m: m, env: env})
		t.Logf("%-26s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d %9.2f",
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
	t.Logf("best by gSNR@0: %s %.2f dB (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("best by low-ratio count: %s %d (delta=%+d)", bestEnvelope.name, bestEnvelope.env.lowRatioFrames, bestEnvelope.env.lowRatioFrames-prodEnv.lowRatioFrames)
	t.Logf("verdict: %s", phase3qGainFFmpegVerdict(rows[0], best, bestCorr, bestEnvelope))
}

func phase3qDecodeRawGainVariant(t *testing.T, raw []byte, frames int, variant phase3jVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3jGainDecoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.decodeFramePhase3jVariant(raw[start:start+bitstream.FrameBytes], out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3jVariant[%s] raw frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func phase3qGainFFmpegVerdict(prod, best, bestCorr, bestEnvelope struct {
	name string
	m    blackboxMetrics
	env  phase3pEnvelopeSummary
}) string {
	if best.name != prod.name && best.name != "gain_mirror_default" && best.m.globalSNR-prod.m.globalSNR > 1.0 {
		return "gain variant materially improves FFmpeg-referenced SNR; inspect " + best.name
	}
	if bestCorr.name != prod.name && bestCorr.name != "gain_mirror_default" && bestCorr.m.corr-prod.m.corr > 0.05 {
		return "gain variant materially improves FFmpeg-referenced correlation; inspect " + bestCorr.name
	}
	if bestEnvelope.name != prod.name && bestEnvelope.name != "gain_mirror_default" &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 &&
		bestEnvelope.m.globalSNR >= prod.m.globalSNR-0.25 &&
		bestEnvelope.m.corr >= prod.m.corr-0.02 {
		return "gain variant materially reduces low-envelope frames; inspect " + bestEnvelope.name
	}
	if bestEnvelope.name != prod.name && bestEnvelope.name != "gain_mirror_default" &&
		prod.env.lowRatioFrames-bestEnvelope.env.lowRatioFrames > prod.env.activeFrames/10 {
		return "envelope-only gain variant reduces low-ratio frames but worsens waveform metrics; do not use as production fix"
	}
	return "no tested gain variant beats production enough against FFmpeg black-box; continue coupled pitch/gain envelope diagnostics"
}
