package decoder

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3cLPVariantProbe_SPEECH probes whether the decoder-only quality
// defect is dominated by the LP envelope feeding the synthesis filter.
func TestPhase3cLPVariantProbe_SPEECH(t *testing.T) {
	const bytesPerOutFrame = 2 * frameSamples

	vecDir := filepath.Join("..", "..", "testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
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

	frames := len(pstData) / bytesPerOutFrame
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	ref := readPCM16LEForProbe(t, pstData, frames*frameSamples)

	type lpVariant struct {
		name string
		fn   func(*[11]int16)
	}
	variants := []lpVariant{
		{name: "production"},
		{name: "lp_zero_identity_synth", fn: func(a *[11]int16) {
			for i := 1; i < len(a); i++ {
				a[i] = 0
			}
		}},
		{name: "lp_half", fn: func(a *[11]int16) {
			for i := 1; i < len(a); i++ {
				a[i] = int16(int32(a[i]) >> 1)
			}
		}},
		{name: "lp_double_sat", fn: func(a *[11]int16) {
			for i := 1; i < len(a); i++ {
				a[i] = sat16(int32(a[i]) << 1)
			}
		}},
		{name: "lp_negate", fn: func(a *[11]int16) {
			for i := 1; i < len(a); i++ {
				a[i] = sat16(-int32(a[i]))
			}
		}},
	}

	t.Logf("Phase 3c LP variant probe — SPEECH.BIT vs SPEECH.PST (%d frames)", frames)
	t.Logf("%-24s %10s %10s %10s %10s", "variant", "rms", "GlobalSNR", "SegSNR", "optScale")
	for _, v := range variants {
		out := decodeLPVariant(t, bitData, frames, v.fn)
		opt := leastSquaresScale(ref, out)
		t.Logf("%-24s %10.2f %10.2f %10.2f %10.4f",
			v.name, scaleProbeRMS(out), scaleProbeGlobalSNR(ref, out),
			scaleProbeSegSNR(ref, out), opt)
	}
}

func decodeLPVariant(t *testing.T, bitData []byte, frames int, lpTransform func(*[11]int16)) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var d Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for i := 0; i < frames; i++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", i, err)
		}
		if err := decodeFrameLPVariant(&d, packed[:], out[i*frameSamples:(i+1)*frameSamples], lpTransform); err != nil {
			t.Fatalf("decodeFrameLPVariant frame %d: %v", i, err)
		}
	}
	return out
}

func decodeFrameLPVariant(d *Decoder, packed []byte, out []int16, lpTransform func(*[11]int16)) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		return err
	}
	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})
	if lpTransform != nil {
		lpTransform(&sf1A)
		lpTransform(&sf2A)
	}

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	decodeSubframeLPVariant(d, &sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen])
	decodeSubframeLPVariant(d, &sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples])
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func decodeSubframeLPVariant(
	d *Decoder,
	sfA *[11]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.prevGpQ14 = gpQ14
}

func sat16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
