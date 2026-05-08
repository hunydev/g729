package g729

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3Diag_DecoderAmplitudeProfile probes the decoder amplitude
// profile across SPEECH.BIT to surface the near-silence pattern observed
// in TestPhase3RoundTripQuality_SPEECH (pipeline B RMS=65 vs reference
// RMS=2237). Dumps per-frame RMS for the first 10 frames + every 200th
// frame, and the running max |sample| to localise where (or whether)
// the decoder output is non-silence.
func TestPhase3Diag_DecoderAmplitudeProfile(t *testing.T) {
	const bytesPerInFrame = 2 * FrameSamples
	vecDir := "testdata/itu/G729_Release3/g729AnnexA/test_vectors"
	bitPath := filepath.Join(vecDir, "SPEECH.BIT")
	pstPath := filepath.Join(vecDir, "SPEECH.PST")

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(pstData) / bytesPerInFrame
	bReader := bytes.NewReader(bitData)
	dec := NewDecoder()

	var packed [FrameBytes]byte
	out := make([]int16, FrameSamples)
	pst := make([]int16, FrameSamples)

	t.Logf("Per-frame amplitude profile (first 10 + every 200th up to %d)", frames)
	t.Logf("%6s %14s %14s %10s %14s",
		"frame", "ourDec_RMS", "PST_RMS", "ourMaxAbs", "ourFirst8")
	var globalMax int16
	for f := 0; f < frames; f++ {
		if _, rerr := bitstream.ReadG192Frame(bReader, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, rerr)
		}
		if derr := dec.DecodeFrame(packed[:], out); derr != nil {
			t.Fatalf("DecodeFrame frame %d: %v", f, derr)
		}
		base := f * bytesPerInFrame
		for i := 0; i < FrameSamples; i++ {
			pst[i] = int16(binary.LittleEndian.Uint16(pstData[base+2*i : base+2*i+2]))
		}
		var maxAbs int16
		for _, v := range out {
			a := v
			if a < 0 {
				a = -a
			}
			if a > maxAbs {
				maxAbs = a
			}
		}
		if maxAbs > globalMax {
			globalMax = maxAbs
		}
		if f < 10 || f%200 == 0 {
			ourRMS := framesRMS(out)
			pstRMS := framesRMS(pst)
			t.Logf("frame %4d ourRMS=%8.1f pstRMS=%8.1f maxAbs=%5d",
				f, ourRMS, pstRMS, maxAbs)
			t.Logf("  our[0:8] = %v", out[:8])
			t.Logf("  pst[0:8] = %v", pst[:8])
		}
	}
	t.Logf("Global max |our decoder output sample| over %d frames: %d", frames, globalMax)
}

func framesRMS(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var e float64
	for _, v := range s {
		e += float64(v) * float64(v)
	}
	return math.Sqrt(e / float64(len(s)))
}
