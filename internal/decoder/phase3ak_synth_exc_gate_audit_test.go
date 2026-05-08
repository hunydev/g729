package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3akSynthExcGateAudit gates GA036_x3/2 using a local stage proxy:
// production synthesis RMS divided by excitation RMS. The flags are computed
// from an unmodified production pass, so this is still diagnostic; it tests
// whether s/u can replace the FFmpeg oracle ratio as a selector signal.
func TestPhase3akSynthExcGateAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_SYNTH_EXC_GATE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_SYNTH_EXC_GATE_AUDIT=1 to run synth/exc gate audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	candidate := phase3afFindScale("GA036_x3/2")
	thresholds := []float64{6, 8, 10, 12, 15}
	t.Logf("Phase 3ak synth/excitation gate audit - %s", candidate.name)
	t.Logf("%-12s %-8s %7s %7s %10s %10s %8s %9s %9s",
		"vector", "s/u", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5")
	t.Logf("%-12s %-8s %7s %7s %10s %10s %8s %9s %9s",
		"------", "---", "------", "-------", "------", "-----", "------", "--------", "-------")

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
		phase3ahLog(t, vector, "prod", frames, 0, ref, prod)
		for _, th := range thresholds {
			flags := phase3akSynthExcFlags(taps, th)
			out := phase3ahDecodeG192ConditionalScale(t, bitData, frames, flags, candidate)
			phase3ahLog(t, vector, phase3akGateName(th), frames, phase3ahCountFlags(flags), ref, out)
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk synth/exc gate audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	phase3ahLog(t, "Asterisk", "prod", frames, 0, ref, prod)
	for _, th := range thresholds {
		flags := phase3akSynthExcFlags(taps, th)
		out := phase3ahDecodeRawConditionalScale(t, raw, frames, flags, candidate)
		phase3ahLog(t, "Asterisk", phase3akGateName(th), frames, phase3ahCountFlags(flags), ref, out)
	}
}

func phase3akSynthExcFlags(taps []Phase3DiagFrameTaps, threshold float64) []bool {
	flags := make([]bool, len(taps))
	for frame, tap := range taps {
		stages := envelopeStageSummary(tap)
		if stages.uRMS <= 0 {
			continue
		}
		flags[frame] = stages.sRMS/stages.uRMS < threshold
	}
	return flags
}

func phase3akGateName(threshold float64) string {
	switch threshold {
	case 6:
		return "s/u<6"
	case 8:
		return "s/u<8"
	case 10:
		return "s/u<10"
	case 12:
		return "s/u<12"
	case 15:
		return "s/u<15"
	default:
		return "s/u<?>"
	}
}
