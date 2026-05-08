package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3anProxyOutputGainAudit applies a final PCM gain only on frames
// selected by the safer Phase 3al runtime proxies. Unlike the fixed-
// contribution experiments, this does not alter excitation or decoder state.
// It tests whether a bounded post-decode envelope recovery is safer.
func TestPhase3anProxyOutputGainAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_PROXY_OUTPUT_GAIN_AUDIT") != "1" {
		t.Skip("set G729_DECODER_PROXY_OUTPUT_GAIN_AUDIT=1 to run proxy output gain audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	selectors := []phase3alSelector{
		{name: "u>=35_su<6_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 6, 0.55) && phase3alPred(m, 30)
		}},
		{name: "u>=35_su<8_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alPred(m, 30)
		}},
		{name: "u>=35_su<10_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 10, 0.55) && phase3alPred(m, 30)
		}},
	}
	scales := []struct {
		name string
		num  int
		den  int
	}{
		{name: "x5/4", num: 5, den: 4},
		{name: "x3/2", num: 3, den: 2},
		{name: "x7/4", num: 7, den: 4},
		{name: "x2", num: 2, den: 1},
	}

	t.Logf("Phase 3an proxy output gain audit")
	t.Logf("%-12s %-34s %-5s %7s %7s %10s %10s %8s %9s %9s %8s",
		"vector", "selector", "gain", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %-34s %-5s %7s %7s %10s %10s %8s %9s %9s %8s",
		"------", "--------", "----", "------", "-------", "------", "-----", "------", "--------", "-------", "-------")

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
		phase3anLog(t, vector, "production", "-", frames, 0, ref, prod)
		for _, sel := range selectors {
			flags := phase3alSelectorFlags(taps, sel)
			for _, scale := range scales {
				out := phase3anScaleSelectedFrames(prod, flags, scale.num, scale.den)
				phase3anLog(t, vector, sel.name, scale.name, frames, phase3ahCountFlags(flags), ref, out)
			}
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk proxy output gain audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	phase3anLog(t, "Asterisk", "production", "-", frames, 0, ref, prod)
	for _, sel := range selectors {
		flags := phase3alSelectorFlags(taps, sel)
		for _, scale := range scales {
			out := phase3anScaleSelectedFrames(prod, flags, scale.num, scale.den)
			phase3anLog(t, "Asterisk", sel.name, scale.name, frames, phase3ahCountFlags(flags), ref, out)
		}
	}
}

func phase3anScaleSelectedFrames(in []int16, flags []bool, num, den int) []int16 {
	out := append([]int16(nil), in...)
	frames := len(out) / frameSamples
	if len(flags) < frames {
		frames = len(flags)
	}
	for frame := 0; frame < frames; frame++ {
		if !flags[frame] {
			continue
		}
		off := frame * frameSamples
		scaled := phase3xScale(out[off:off+frameSamples], num, den)
		copy(out[off:off+frameSamples], scaled)
	}
	return out
}

func phase3anLog(t *testing.T, vector, selector, gain string, frames, applied int, ref, out []int16) {
	t.Helper()
	m := blackboxMeasure(ref, out, 40)
	env := phase3pEnvelopeCompare(ref, out)
	t.Logf("%-12s %-34s %-5s %7d %7d %10.2f %10.2f %8.3f %9.3f %9d %8d",
		vector, selector, gain, frames, applied, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
}
