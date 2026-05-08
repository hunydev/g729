package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3ahGainClusterOracleGateAudit applies GA036_x3/2 only on frames
// where an FFmpeg black-box oracle says the production local envelope is low.
// This is not production-implementable; it bounds the benefit of a perfect
// selector and checks state coupling under conditional application.
func TestPhase3ahGainClusterOracleGateAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CLUSTER_ORACLE_GATE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_CLUSTER_ORACLE_GATE_AUDIT=1 to run gain-cluster oracle gate audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	candidate := phase3afFindScale("GA036_x3/2")
	thresholds := []float64{0.50, 0.75, 1.00}
	t.Logf("Phase 3ah gain-cluster oracle gate audit - %s", candidate.name)
	t.Logf("%-12s %-10s %7s %7s %10s %10s %8s %9s %9s",
		"vector", "gate", "frames", "applied", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5")
	t.Logf("%-12s %-10s %7s %7s %10s %10s %8s %9s %9s",
		"------", "----", "------", "-------", "------", "-----", "------", "--------", "-------")

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
		prod := decodeVariant(t, bitData, frames, nil, nil)
		phase3ahLog(t, vector, "production", frames, 0, ref, prod)
		for _, th := range thresholds {
			flags := phase3ahOracleFlags(ref, prod, th)
			out := phase3ahDecodeG192ConditionalScale(t, bitData, frames, flags, candidate)
			phase3ahLog(t, vector, phase3ahGateName(th), frames, phase3ahCountFlags(flags), ref, out)
		}
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk oracle gate audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	phase3ahLog(t, "Asterisk", "production", frames, 0, ref, prod)
	for _, th := range thresholds {
		flags := phase3ahOracleFlags(ref, prod, th)
		out := phase3ahDecodeRawConditionalScale(t, raw, frames, flags, candidate)
		phase3ahLog(t, "Asterisk", phase3ahGateName(th), frames, phase3ahCountFlags(flags), ref, out)
	}
}

func phase3ahOracleFlags(ref, prod []int16, threshold float64) []bool {
	frames := len(ref) / frameSamples
	if pf := len(prod) / frameSamples; pf < frames {
		frames = pf
	}
	flags := make([]bool, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		refRMS := envelopeRMS(ref[off : off+frameSamples])
		if refRMS < 500 {
			continue
		}
		prodRMS := envelopeRMS(prod[off : off+frameSamples])
		flags[frame] = prodRMS/refRMS < threshold
	}
	return flags
}

func phase3ahDecodeG192ConditionalScale(t *testing.T, bitData []byte, frames int, flags []bool, candidate phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	production := phase3adClusterScale{name: "production", num: 1, den: 1, mask: func(uint8) bool { return false }}
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[oracle gate] frame %d: %v", f, err)
		}
		scale := production
		if f < len(flags) && flags[f] {
			scale = candidate
		}
		if err := dec.decodeFramePhase3adScale(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, scale); err != nil {
			t.Fatalf("decodeFramePhase3adScale[oracle gate] frame %d: %v", f, err)
		}
	}
	return out
}

func phase3ahDecodeRawConditionalScale(t *testing.T, raw []byte, frames int, flags []bool, candidate phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	production := phase3adClusterScale{name: "production", num: 1, den: 1, mask: func(uint8) bool { return false }}
	for f := 0; f < frames; f++ {
		scale := production
		if f < len(flags) && flags[f] {
			scale = candidate
		}
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3adScale(packed, out[f*frameSamples:(f+1)*frameSamples], &gd, scale); err != nil {
			t.Fatalf("decodeFramePhase3adScale[oracle gate] frame %d: %v", f, err)
		}
	}
	return out
}

func phase3ahLog(t *testing.T, vector, gate string, frames, applied int, ref, out []int16) {
	t.Helper()
	m := blackboxMeasure(ref, out, 40)
	env := phase3pEnvelopeCompare(ref, out)
	t.Logf("%-12s %-10s %7d %7d %10.2f %10.2f %8.3f %9.3f %9d",
		vector, gate, frames, applied, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames)
}

func phase3ahGateName(threshold float64) string {
	switch threshold {
	case 0.50:
		return "ratio<0.5"
	case 0.75:
		return "ratio<0.75"
	case 1.00:
		return "ratio<1.0"
	default:
		return "ratio<?>"
	}
}

func phase3ahCountFlags(flags []bool) int {
	var count int
	for _, f := range flags {
		if f {
			count++
		}
	}
	return count
}
