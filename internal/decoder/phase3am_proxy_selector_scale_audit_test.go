package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3amProxySelectorScaleAudit checks whether the safer Phase 3al
// local selectors can tolerate stronger GA036 contribution scaling. This is
// diagnostic-only: the selector is based on an unmodified local pass, and the
// acceptance bar is multivector improvement without LSP/ALGTHM/TEST damage.
func TestPhase3amProxySelectorScaleAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_PROXY_SELECTOR_SCALE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_PROXY_SELECTOR_SCALE_AUDIT=1 to run proxy selector scale audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	selectors := []phase3alSelector{
		{name: "u>=35_su<6_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 6, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<8_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<10_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 10, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
	}
	scales := []phase3adClusterScale{
		phase3afFindScale("GA036_x5/4"),
		phase3afFindScale("GA036_x3/2"),
		phase3afFindScale("GA036_x7/4"),
		phase3afFindScale("GA036_x2"),
	}

	t.Logf("Phase 3am proxy selector scale audit")
	t.Logf("%-12s %-34s %-11s %7s %7s %10s %10s %8s %9s %9s %8s",
		"vector", "selector", "scale", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %-34s %-11s %7s %7s %10s %10s %8s %9s %9s %8s",
		"------", "--------", "-----", "------", "-------", "------", "-----", "------", "--------", "-------", "-------")

	for _, vector := range []string{"SPEECH.BIT", "FIXED.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
		path := vectorPath(vector)
		ensureTestdataPresent(t, path)
		bitData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", vector, err)
		}
		frames := len(bitData) / bitstream.G192FrameBytes
		if frames == 0 {
			continue
		}
		ref := phase3uFFmpegDecodeG192(t, bitData, frames, vector)
		prod, taps := decodeG192WithTapsForEnvelopeAudit(t, bitData, frames)
		phase3amLog(t, vector, "production", "-", frames, 0, ref, prod)
		for _, sel := range selectors {
			flags := phase3alSelectorFlags(taps, sel)
			for _, scale := range scales {
				out := phase3ahDecodeG192ConditionalScale(t, bitData, frames, flags, scale)
				phase3amLog(t, vector, sel.name, scale.name, frames, phase3ahCountFlags(flags), ref, out)
			}
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk proxy selector scale audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	phase3amLog(t, "Asterisk", "production", "-", frames, 0, ref, prod)
	for _, sel := range selectors {
		flags := phase3alSelectorFlags(taps, sel)
		for _, scale := range scales {
			out := phase3ahDecodeRawConditionalScale(t, raw, frames, flags, scale)
			phase3amLog(t, "Asterisk", sel.name, scale.name, frames, phase3ahCountFlags(flags), ref, out)
		}
	}
}

func phase3amLog(t *testing.T, vector, selector, scale string, frames, applied int, ref, out []int16) {
	t.Helper()
	m := blackboxMeasure(ref, out, 40)
	env := phase3pEnvelopeCompare(ref, out)
	t.Logf("%-12s %-34s %-11s %7d %7d %10.2f %10.2f %8.3f %9.3f %9d %8d",
		vector, selector, scale, frames, applied, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
}
