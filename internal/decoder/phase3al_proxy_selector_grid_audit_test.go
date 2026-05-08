package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3alProxySelectorGridAudit tries small runtime-available selector
// combinations after the FFmpeg-oracle gate proved that GA036_x3/2 only helps
// when the local decoder envelope is low. FFmpeg remains an executable-only
// black-box reference; all candidates are diagnostic-only until they pass the
// multivector regression screen.
func TestPhase3alProxySelectorGridAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_PROXY_SELECTOR_GRID_AUDIT") != "1" {
		t.Skip("set G729_DECODER_PROXY_SELECTOR_GRID_AUDIT=1 to run proxy selector grid audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	candidate := phase3afFindScale("GA036_x3/2")
	selectors := phase3alSelectors()
	t.Logf("Phase 3al proxy selector grid audit - %s", candidate.name)
	t.Logf("%-12s %-34s %7s %7s %10s %10s %8s %9s %9s",
		"vector", "selector", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5")
	t.Logf("%-12s %-34s %7s %7s %10s %10s %8s %9s %9s",
		"------", "--------", "------", "-------", "------", "-----", "------", "--------", "-------")

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
		phase3ahLog(t, vector, "production", frames, 0, ref, prod)
		for _, sel := range selectors {
			flags := phase3alSelectorFlags(taps, sel)
			out := phase3ahDecodeG192ConditionalScale(t, bitData, frames, flags, candidate)
			phase3ahLog(t, vector, sel.name, frames, phase3ahCountFlags(flags), ref, out)
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk proxy selector grid audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	phase3ahLog(t, "Asterisk", "production", frames, 0, ref, prod)
	for _, sel := range selectors {
		flags := phase3alSelectorFlags(taps, sel)
		out := phase3ahDecodeRawConditionalScale(t, raw, frames, flags, candidate)
		phase3ahLog(t, "Asterisk", sel.name, frames, phase3ahCountFlags(flags), ref, out)
	}
}

type phase3alSelector struct {
	name string
	fn   func(envelopeStageMetrics) bool
}

func phase3alSelectors() []phase3alSelector {
	return []phase3alSelector{
		{name: "u>=35_su<6_fix<.55", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 6, 0.55) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<8_fix<.55", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<10_fix<.55", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 10, 0.55) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<8_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<10_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 10, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<8_fix<.55_gc<110", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alFrameGain(m, -1, 110)
		}},
		{name: "u>=35_su<10_fix<.55_gc<110", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 10, 0.55) && phase3alFrameGain(m, -1, 110)
		}},
		{name: "u>=35_su<8_fix<.55_pred>=30_gc<110", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 110)
		}},
		{name: "u>=45_su<8_fix<.55_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 45, 8, 0.55) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
		{name: "u>=35_su<8_fix<.50_pred>=30", fn: func(m envelopeStageMetrics) bool {
			return phase3alBase(m, 35, 8, 0.50) && phase3alPred(m, 30) && phase3alFrameGain(m, -1, 1e9)
		}},
	}
}

func phase3alSelectorFlags(taps []Phase3DiagFrameTaps, sel phase3alSelector) []bool {
	flags := make([]bool, len(taps))
	for frame, tap := range taps {
		if !phase3alHasGA036(tap.Frame) {
			continue
		}
		flags[frame] = sel.fn(envelopeStageSummary(tap))
	}
	return flags
}

func phase3alHasGA036(f bitstream.Frame) bool {
	return phase3aiIsGA036(uint8(f.GA1)) || phase3aiIsGA036(uint8(f.GA2))
}

func phase3alBase(m envelopeStageMetrics, minURMS, maxSynthExc, maxFixedExc float64) bool {
	if m.uRMS < minURMS {
		return false
	}
	return safeRatioFloat64(m.sRMS, m.uRMS) < maxSynthExc &&
		safeRatioFloat64(m.fixedRMS, m.uRMS) < maxFixedExc
}

func phase3alPred(m envelopeStageMetrics, minPredDb float64) bool {
	return m.predictedAvgQ10/1024.0 >= minPredDb
}

func phase3alFrameGain(m envelopeStageMetrics, minGC, maxGC float64) bool {
	return m.gcMax >= minGC && m.gcMax < maxGC
}
