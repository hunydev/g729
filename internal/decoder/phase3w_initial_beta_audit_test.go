package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3wInitialBetaAudit checks the stream-initial fixed-codebook pitch
// enhancement state. Table-9 clean-room notes in the older diagnostics identify
// β as a non-zero initialization variable. This audit measures the impact of
// initializing the decoder's previous pitch-gain state to the upper β value
// before the first subframe. FFmpeg is used only as an external process.
func TestPhase3wInitialBetaAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_INITIAL_BETA_AUDIT") != "1" {
		t.Skip("set G729_DECODER_INITIAL_BETA_AUDIT=1 to run initial beta audit")
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
	speechFrames := len(bitData) / bitstream.G192FrameBytes
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	phase3wReport(t, "SPEECH.BIT", speechRef,
		decodeVariant(t, bitData, speechFrames, nil, nil),
		phase3wDecodeG192InitialBeta(t, bitData, speechFrames))

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk initial beta audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3wReport(t, "Asterisk", astRef,
		phase3tDecodeRawVariant(t, raw, astFrames, phase3eVariant{name: "production"}),
		phase3wDecodeRawInitialBeta(t, raw, astFrames))
}

func phase3wReport(t *testing.T, label string, ref, production, betaUpperInit []int16) {
	t.Helper()
	prod := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)
	init := blackboxMeasure(ref, betaUpperInit, 40)
	initEnv := phase3pEnvelopeCompare(ref, betaUpperInit)

	t.Logf("Phase 3w initial beta audit - %s", label)
	t.Logf("%-18s %9s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "bestSNR")
	t.Logf("%-18s %9s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "------", "-----", "------", "--------", "-------", "-------")
	t.Logf("%-18s %9.2f %10.2f %10.2f %8.3f %9.3f %9d %9.2f",
		"production", prod.rms, prod.globalSNR, prod.segSNR, prod.corr, prodEnv.ratioMedian, prodEnv.lowRatioFrames, prod.bestSNR)
	t.Logf("%-18s %9.2f %10.2f %10.2f %8.3f %9.3f %9d %9.2f",
		"initial_beta_0.8", init.rms, init.globalSNR, init.segSNR, init.corr, initEnv.ratioMedian, initEnv.lowRatioFrames, init.bestSNR)
	t.Logf("delta init-production: gSNR=%+.4f seg=%+.4f corr=%+.5f low<0.5=%+d",
		init.globalSNR-prod.globalSNR, init.segSNR-prod.segSNR, init.corr-prod.corr, initEnv.lowRatioFrames-prodEnv.lowRatioFrames)
}

func phase3wDecodeG192InitialBeta(t *testing.T, bitData []byte, frames int) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	dec.prevGpQ14 = 13107
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame initial beta frame %d: %v", f, err)
		}
		if err := dec.Decode(packed[:], false, out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("Decode initial beta frame %d: %v", f, err)
		}
	}
	return out
}

func phase3wDecodeRawInitialBeta(t *testing.T, raw []byte, frames int) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	dec.prevGpQ14 = 13107
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.Decode(raw[start:start+bitstream.FrameBytes], false, out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("Decode raw initial beta frame %d: %v", f, err)
		}
	}
	return out
}
