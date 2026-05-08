package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3vPostfilterFFmpegAudit checks whether bypassing the local
// postfilter improves agreement with FFmpeg executable black-box decode.
// FFmpeg is used only as an external process; no implementation source is
// inspected.
func TestPhase3vPostfilterFFmpegAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_POSTFILTER_FFMPEG_AUDIT") != "1" {
		t.Skip("set G729_DECODER_POSTFILTER_FFMPEG_AUDIT=1 to run postfilter-vs-ffmpeg audit")
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
	phase3vReport(t, "SPEECH.BIT", speechRef,
		decodeVariant(t, bitData, speechFrames, nil, nil),
		blackboxDecodeNoPostfilter(t, bitData, speechFrames))

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk postfilter audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3vReport(t, "Asterisk", astRef,
		phase3tDecodeRawVariant(t, raw, astFrames, phase3eVariant{name: "production"}),
		phase3vDecodeRawNoPostfilter(t, raw, astFrames))
}

func phase3vReport(t *testing.T, label string, ref, production, noPF []int16) {
	t.Helper()
	prod := blackboxMeasure(ref, production, 40)
	prodEnv := phase3pEnvelopeCompare(ref, production)
	noPFM := blackboxMeasure(ref, noPF, 40)
	noPFEnv := phase3pEnvelopeCompare(ref, noPF)

	t.Logf("Phase 3v postfilter FFmpeg audit - %s", label)
	t.Logf("%-16s %9s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "bestSNR")
	t.Logf("%-16s %9s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "------", "-----", "------", "--------", "-------", "-------")
	t.Logf("%-16s %9.2f %10.2f %10.2f %8.3f %9.3f %9d %9.2f",
		"production", prod.rms, prod.globalSNR, prod.segSNR, prod.corr, prodEnv.ratioMedian, prodEnv.lowRatioFrames, prod.bestSNR)
	t.Logf("%-16s %9.2f %10.2f %10.2f %8.3f %9.3f %9d %9.2f",
		"no_postfilter", noPFM.rms, noPFM.globalSNR, noPFM.segSNR, noPFM.corr, noPFEnv.ratioMedian, noPFEnv.lowRatioFrames, noPFM.bestSNR)
	t.Logf("delta noPF-production: gSNR=%+.2f seg=%+.2f corr=%+.3f low<0.5=%+d",
		noPFM.globalSNR-prod.globalSNR, noPFM.segSNR-prod.segSNR, noPFM.corr-prod.corr, noPFEnv.lowRatioFrames-prodEnv.lowRatioFrames)
	if noPFM.globalSNR-prod.globalSNR > 1.0 || noPFM.corr-prod.corr > 0.05 {
		t.Logf("verdict: postfilter bypass materially improves %s; inspect postfilter path", label)
	} else {
		t.Logf("verdict: postfilter bypass does not materially improve %s", label)
	}
}

func phase3vDecodeRawNoPostfilter(t *testing.T, raw []byte, frames int) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.DecodeFrameNoPostfilter(raw[start:start+bitstream.FrameBytes], out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("DecodeFrameNoPostfilter raw frame %d: %v", f, err)
		}
	}
	return out
}
