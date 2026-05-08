package decoder

import (
	"os"
	"os/exec"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3afGainClusterMultiVectorAudit checks the leading GA036 cluster
// candidates across multiple Annex A bitstreams against FFmpeg black-box
// decode. It guards against overfitting the Asterisk payload or SPEECH.BIT.
func TestPhase3afGainClusterMultiVectorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CLUSTER_MULTIVECTOR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_CLUSTER_MULTIVECTOR_AUDIT=1 to run gain-cluster multivector audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	vectors := []string{
		"TAME.BIT",
		"SPEECH.BIT",
		"FIXED.BIT",
		"LSP.BIT",
		"PITCH.BIT",
		"TEST.BIT",
		"ALGTHM.BIT",
	}
	candidates := []phase3adClusterScale{
		phase3afFindScale("GA036_x5/4"),
		phase3afFindScale("GA036_x3/2"),
		phase3afFindScale("GA6_x3/2"),
	}

	t.Logf("Phase 3af gain-cluster multivector audit")
	t.Logf("%-12s %-12s %7s %10s %10s %8s %9s %9s %9s",
		"vector", "candidate", "frames", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %-12s %7s %10s %10s %8s %9s %9s %9s",
		"------", "---------", "------", "------", "-----", "------", "--------", "-------", "-------")

	for _, vector := range vectors {
		path := vectorPath(vector)
		ensureTestdataPresent(t, path)
		bitData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", vector, err)
		}
		frames := len(bitData) / bitstream.G192FrameBytes
		if frames == 0 {
			t.Logf("%-12s skipped: no frames", vector)
			continue
		}
		ref := phase3uFFmpegDecodeG192(t, bitData, frames, vector)
		prod := decodeVariant(t, bitData, frames, nil, nil)
		phase3afLogRow(t, vector, "production", frames, ref, prod)
		for _, c := range candidates {
			out := phase3adDecodeG192Scale(t, bitData, frames, c)
			phase3afLogRow(t, vector, c.name, frames, ref, out)
		}
	}
}

func phase3afFindScale(name string) phase3adClusterScale {
	for _, s := range phase3adScales() {
		if s.name == name {
			return s
		}
	}
	panic("missing phase3ad scale " + name)
}

func phase3afLogRow(t *testing.T, vector, candidate string, frames int, ref, out []int16) {
	t.Helper()
	m := blackboxMeasure(ref, out, 40)
	env := phase3pEnvelopeCompare(ref, out)
	t.Logf("%-12s %-12s %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
		vector, candidate, frames, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
}
