package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3cDecoderScaleProbe_SPEECH separates a uniform gain error from
// waveform-shape errors on the decoder-only path (SPEECH.BIT -> our decoder).
//
// If a simple scalar produces a large SNR recovery against SPEECH.PST, the
// next production fix should look for a missing Q-format/amplitude factor. If
// the best scalar barely moves SNR, the remaining defect is in the reconstructed
// waveform shape: LSP/LP envelope, adaptive-codebook trajectory, fixed-codebook
// excitation, or gain predictor state.
func TestPhase3cDecoderScaleProbe_SPEECH(t *testing.T) {
	const bytesPerOutFrame = 2 * frameSamples

	vecDir := filepath.Join("..", "..", "testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	bitPath := filepath.Join(vecDir, "SPEECH.BIT")
	pstPath := filepath.Join(vecDir, "SPEECH.PST")
	inPath := filepath.Join(vecDir, "SPEECH.IN")

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}

	frames := len(pstData) / bytesPerOutFrame
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if inf := len(inData) / bytesPerOutFrame; inf < frames {
		frames = inf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	totalSamples := frames * frameSamples

	refPST := readPCM16LEForProbe(t, pstData, totalSamples)
	refIN := readPCM16LEForProbe(t, inData, totalSamples)
	decoded := make([]int16, totalSamples)

	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, rerr)
		}
		if derr := dec.Decode(packed[:], false, decoded[f*frameSamples:(f+1)*frameSamples]); derr != nil {
			t.Fatalf("Decode frame %d: %v", f, derr)
		}
	}

	t.Logf("Phase 3c decoder scale probe — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("RMS: input=%.2f  pst=%.2f  decoded=%.2f", scaleProbeRMS(refIN), scaleProbeRMS(refPST), scaleProbeRMS(decoded))
	logScaleTable(t, "vs SPEECH.PST", refPST, decoded)
	logScaleTable(t, "vs SPEECH.IN", refIN, decoded)
}

func readPCM16LEForProbe(t *testing.T, data []byte, samples int) []int16 {
	t.Helper()
	out := make([]int16, samples)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i : 2*i+2]))
	}
	return out
}

func logScaleTable(t *testing.T, label string, ref, decoded []int16) {
	t.Helper()
	opt := leastSquaresScale(ref, decoded)
	t.Logf("")
	t.Logf("%s", label)
	t.Logf("%8s %10s %10s %10s", "scale", "rms", "GlobalSNR", "SegSNR")
	for _, scale := range []float64{1, 2, 3, 4, 5, 6, 8, opt} {
		scaled := scaleInt16(decoded, scale)
		t.Logf("%8.3f %10.2f %10.2f %10.2f", scale, scaleProbeRMS(scaled),
			scaleProbeGlobalSNR(ref, scaled), scaleProbeSegSNR(ref, scaled))
	}
	t.Logf("least-squares optimal scale = %.4f", opt)
}

func leastSquaresScale(ref, test []int16) float64 {
	var dot, den float64
	for i := range ref {
		x := float64(test[i])
		dot += float64(ref[i]) * x
		den += x * x
	}
	if den == 0 {
		return 0
	}
	return dot / den
}

func scaleInt16(in []int16, scale float64) []int16 {
	out := make([]int16, len(in))
	for i, s := range in {
		v := math.Round(float64(s) * scale)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}

func scaleProbeRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var e float64
	for _, s := range samples {
		x := float64(s)
		e += x * x
	}
	return math.Sqrt(e / float64(len(samples)))
}

func scaleProbeGlobalSNR(ref, test []int16) float64 {
	var sigE, errE float64
	for i := range ref {
		s := float64(ref[i])
		e := s - float64(test[i])
		sigE += s * s
		errE += e * e
	}
	if errE == 0 {
		return math.Inf(+1)
	}
	if sigE == 0 {
		return 0
	}
	return 10 * math.Log10(sigE/errE)
}

func scaleProbeSegSNR(ref, test []int16) float64 {
	var sum float64
	var count int
	for i := 0; i+frameSamples <= len(ref); i += frameSamples {
		var sigE, errE float64
		for j := 0; j < frameSamples; j++ {
			s := float64(ref[i+j])
			e := s - float64(test[i+j])
			sigE += s * s
			errE += e * e
		}
		if sigE < 1 {
			continue
		}
		var db float64
		if errE == 0 {
			db = 35
		} else {
			db = 10 * math.Log10(sigE/errE)
		}
		if db > 35 {
			db = 35
		} else if db < -10 {
			db = -10
		}
		sum += db
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
