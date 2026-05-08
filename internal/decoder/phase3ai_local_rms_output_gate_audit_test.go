package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3aiLocalRMSOutputGateAudit tries a runtime-available proxy:
// apply post-decode PCM scaling only when the unmodified local frame RMS is
// below a threshold and either subframe uses transmitted GA in {0,3,6}. This
// does not alter decoder state; it is an audibility/output-compensation probe,
// not a canonical decoder fix.
func TestPhase3aiLocalRMSOutputGateAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_LOCAL_RMS_OUTPUT_GATE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_LOCAL_RMS_OUTPUT_GATE_AUDIT=1 to run local RMS output gate audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	t.Logf("Phase 3ai local-RMS output gate audit")
	t.Logf("%-12s %-14s %7s %7s %10s %10s %8s %9s %9s %9s",
		"vector", "gate", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %-14s %7s %7s %10s %10s %8s %9s %9s %9s",
		"------", "----", "------", "-------", "------", "-----", "------", "--------", "-------", "-------")

	for _, vector := range []string{"SPEECH.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
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
		prod := decodeVariant(t, bitData, frames, nil, nil)
		gaFlags := phase3aiReadG192GA036Flags(t, bitData, frames)
		phase3aiLog(t, vector, "production", frames, 0, ref, prod)
		for _, gate := range phase3aiOutputGates() {
			out, applied := phase3aiApplyOutputGate(prod, gaFlags, gate)
			phase3aiLog(t, vector, gate.name, frames, applied, ref, out)
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk local RMS output gate audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	gaFlags := phase3aiReadRawGA036Flags(t, raw, frames)
	phase3aiLog(t, "Asterisk", "production", frames, 0, ref, prod)
	for _, gate := range phase3aiOutputGates() {
		out, applied := phase3aiApplyOutputGate(prod, gaFlags, gate)
		phase3aiLog(t, "Asterisk", gate.name, frames, applied, ref, out)
	}
}

type phase3aiGate struct {
	name      string
	threshold float64
	num       int
	den       int
}

func phase3aiOutputGates() []phase3aiGate {
	return []phase3aiGate{
		{name: "rms<500_x3/2", threshold: 500, num: 3, den: 2},
		{name: "rms<750_x3/2", threshold: 750, num: 3, den: 2},
		{name: "rms<1000_x3/2", threshold: 1000, num: 3, den: 2},
		{name: "rms<1500_x5/4", threshold: 1500, num: 5, den: 4},
		{name: "rms<1500_x3/2", threshold: 1500, num: 3, den: 2},
		{name: "rms<2000_x5/4", threshold: 2000, num: 5, den: 4},
		{name: "rms<2000_x3/2", threshold: 2000, num: 3, den: 2},
	}
}

func phase3aiReadG192GA036Flags(t *testing.T, bitData []byte, frames int) []bool {
	t.Helper()
	flags := make([]bool, frames)
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[GA flags] frame %d: %v", frame, err)
		}
		flags[frame] = phase3aiFrameHasGA036(t, frame, packed[:])
	}
	return flags
}

func phase3aiReadRawGA036Flags(t *testing.T, raw []byte, frames int) []bool {
	t.Helper()
	flags := make([]bool, frames)
	for frame := 0; frame < frames; frame++ {
		packed := raw[frame*bitstream.FrameBytes : (frame+1)*bitstream.FrameBytes]
		flags[frame] = phase3aiFrameHasGA036(t, frame, packed)
	}
	return flags
}

func phase3aiFrameHasGA036(t *testing.T, frame int, packed []byte) bool {
	t.Helper()
	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		t.Fatalf("Unpack[GA flags] frame %d: %v", frame, err)
	}
	return phase3aiIsGA036(uint8(f.GA1)) || phase3aiIsGA036(uint8(f.GA2))
}

func phase3aiIsGA036(ga uint8) bool {
	return ga == 0 || ga == 3 || ga == 6
}

func phase3aiApplyOutputGate(prod []int16, gaFlags []bool, gate phase3aiGate) ([]int16, int) {
	out := append([]int16(nil), prod...)
	frames := len(out) / frameSamples
	if len(gaFlags) < frames {
		frames = len(gaFlags)
	}
	var applied int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if !gaFlags[frame] {
			continue
		}
		if envelopeRMS(out[off:off+frameSamples]) >= gate.threshold {
			continue
		}
		scaled := phase3xScale(out[off:off+frameSamples], gate.num, gate.den)
		copy(out[off:off+frameSamples], scaled)
		applied++
	}
	return out, applied
}

func phase3aiLog(t *testing.T, vector, gate string, frames, applied int, ref, out []int16) {
	t.Helper()
	m := blackboxMeasure(ref, out, 40)
	env := phase3pEnvelopeCompare(ref, out)
	t.Logf("%-12s %-14s %7d %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
		vector, gate, frames, applied, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
}
