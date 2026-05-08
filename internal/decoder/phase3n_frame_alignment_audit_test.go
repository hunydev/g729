package decoder

import (
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3nFrameAlignmentAudit_SPEECH checks whether the poor global metric
// is explained by a larger sample or frame offset than the ±40-sample sweep
// used in earlier diagnostics.
func TestPhase3nFrameAlignmentAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FRAME_ALIGNMENT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FRAME_ALIGNMENT_AUDIT=1 to run frame alignment audit")
	}

	bitPath := vectorPath("SPEECH.BIT")
	pstPath := vectorPath("SPEECH.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)
	packed := phase3mReadPackedFrames(t, bitData, frames)
	production := decodeVariant(t, bitData, frames, nil, nil)

	rows := []struct {
		name string
		out  []int16
	}{
		{name: "production", out: production},
		{name: "decode_input_prev_frame", out: phase3nDecodeFrameShift(t, packed, -1)},
		{name: "decode_input_next_frame", out: phase3nDecodeFrameShift(t, packed, +1)},
	}

	t.Logf("Phase 3n frame alignment audit - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("%-24s %9s %7s %10s %8s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "corr@0", "lagCorr", "corrBest", "gSNRBest")
	t.Logf("%-24s %9s %7s %10s %8s %9s %10s %10s",
		"-------", "---", "----", "------", "------", "-------", "--------", "---------")
	for _, r := range rows {
		m := blackboxMeasure(ref, r.out, 400)
		t.Logf("%-24s %9.2f %7d %10.2f %8.3f %9d %10.3f %10.2f",
			r.name, m.rms, m.peak, m.globalSNR, m.corr,
			m.bestCorrLag, m.bestCorr, m.bestSNR)
	}
	t.Logf("verdict: %s", phase3nAlignmentVerdict(blackboxMeasure(ref, production, 400)))
}

func phase3nDecodeFrameShift(t *testing.T, packed [][bitstream.FrameBytes]byte, frameOffset int) []int16 {
	t.Helper()
	out := make([]int16, len(packed)*frameSamples)
	var dec Decoder
	for f := range packed {
		src := f + frameOffset
		if src < 0 {
			src = 0
		} else if src >= len(packed) {
			src = len(packed) - 1
		}
		var buf [bitstream.FrameBytes]byte
		copy(buf[:], packed[src][:])
		if err := dec.Decode(buf[:], false, out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("Decode[phase3n] frame %d source %d: %v", f, src, err)
		}
	}
	return out
}

func phase3nAlignmentVerdict(prod blackboxMetrics) string {
	if absInt(prod.bestCorrLag) >= frameSamples-5 && prod.bestCorr-prod.corr > 0.2 {
		return "best correlation is near a frame-sized lag; inspect frame alignment"
	}
	if absInt(prod.bestCorrLag) > 40 && prod.bestCorr-prod.corr > 0.05 {
		return "larger-than-40 sample lag improves correlation; inspect sample alignment"
	}
	return "large sample/frame lag does not rescue global correlation"
}
