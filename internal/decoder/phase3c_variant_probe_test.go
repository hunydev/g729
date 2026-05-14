package decoder

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	pitchidx "github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/tables"
)

// TestPhase3cDecoderVariantProbe_SPEECH compares clean-room bitstream
// interpretation variants over the whole SPEECH corpus. The variants are not
// production candidates by themselves; they are discriminators for locating the
// shape defect exposed by external G.729 payloads.
func TestPhase3cDecoderVariantProbe_SPEECH(t *testing.T) {
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
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}

	ref := readPCM16LEForProbe(t, pstData, frames*frameSamples)

	type variant struct {
		name            string
		transform       func(*bitstream.Frame)
		packedTransform func(*[bitstream.FrameBytes]byte)
	}
	variants := []variant{
		{name: "production"},
		{name: "packed_reverse_bits_per_byte", packedTransform: func(p *[bitstream.FrameBytes]byte) {
			for i := range p {
				p[i] = reverseByte(p[i])
			}
		}},
		{name: "packed_reverse_byte_order", packedTransform: func(p *[bitstream.FrameBytes]byte) {
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}},
		{name: "gain_no_imap_shim", transform: func(f *bitstream.Frame) {
			f.GA1 = uint16(tables.GainMap1[f.GA1])
			f.GB1 = uint16(tables.GainMap2[f.GB1])
			f.GA2 = uint16(tables.GainMap1[f.GA2])
			f.GB2 = uint16(tables.GainMap2[f.GB2])
		}},
		{name: "gain_double_imap_shim", transform: func(f *bitstream.Frame) {
			f.GA1 = uint16(tables.GainImap1[f.GA1])
			f.GB1 = uint16(tables.GainImap2[f.GB1])
			f.GA2 = uint16(tables.GainImap1[f.GA2])
			f.GB2 = uint16(tables.GainImap2[f.GB2])
		}},
		{name: "gain_stage1_no_imap_only", transform: func(f *bitstream.Frame) {
			f.GA1 = uint16(tables.GainMap1[f.GA1])
			f.GA2 = uint16(tables.GainMap1[f.GA2])
		}},
		{name: "gain_stage2_no_imap_only", transform: func(f *bitstream.Frame) {
			f.GB1 = uint16(tables.GainMap2[f.GB1])
			f.GB2 = uint16(tables.GainMap2[f.GB2])
		}},
		{name: "pitch_frac_sign_flip_both", transform: func(f *bitstream.Frame) {
			t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
			if frac1 != 0 {
				p1 := closedloop.EncodeP1(int16(t1), int8(-frac1))
				f.P1 = uint16(p1)
				f.P0 = uint16(closedloop.EncodeP0(p1))
			}
			t1Updated, _ := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
			t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(f.P2), t1Updated)
			if frac2 != 0 {
				tmin, _ := closedloop.Subframe2Window(int16(t1Updated))
				f.P2 = uint16(closedloop.EncodeP2(int16(t2), int8(-frac2), tmin))
			}
		}},
		{name: "pitch_frac_sign_flip_sf1", transform: func(f *bitstream.Frame) {
			t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
			if frac1 == 0 {
				return
			}
			p1 := closedloop.EncodeP1(int16(t1), int8(-frac1))
			f.P1 = uint16(p1)
			f.P0 = uint16(closedloop.EncodeP0(p1))
		}},
		{name: "pitch_frac_sign_flip_sf2", transform: func(f *bitstream.Frame) {
			t1, _ := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
			t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(f.P2), t1)
			if frac2 == 0 {
				return
			}
			tmin, _ := closedloop.Subframe2Window(int16(t1))
			f.P2 = uint16(closedloop.EncodeP2(int16(t2), int8(-frac2), tmin))
		}},
		{name: "fcb_signs_inverted", transform: func(f *bitstream.Frame) {
			f.S1 ^= 0xF
			f.S2 ^= 0xF
		}},
		{name: "fcb_sign_bits_reversed", transform: func(f *bitstream.Frame) {
			f.S1 = uint16(reverseNibble(byte(f.S1)))
			f.S2 = uint16(reverseNibble(byte(f.S2)))
		}},
	}

	t.Logf("Phase 3c decoder variant probe — SPEECH.BIT vs SPEECH.PST (%d frames)", frames)
	t.Logf("%-26s %10s %10s %10s %10s", "variant", "rms", "GlobalSNR", "SegSNR", "optScale")
	for _, v := range variants {
		out := decodeVariant(t, bitData, frames, v.transform, v.packedTransform)
		opt := leastSquaresScale(ref, out)
		t.Logf("%-26s %10.2f %10.2f %10.2f %10.4f",
			v.name, scaleProbeRMS(out), scaleProbeGlobalSNR(ref, out),
			scaleProbeSegSNR(ref, out), opt)
	}
}

func decodeVariant(t *testing.T, bitData []byte, frames int, transform func(*bitstream.Frame), packedTransform func(*[bitstream.FrameBytes]byte)) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	var repacked [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for i := 0; i < frames; i++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", i, err)
		}
		if packedTransform != nil {
			packedTransform(&packed)
		}
		use := packed[:]
		if transform != nil {
			var f bitstream.Frame
			if err := bitstream.Unpack(packed[:], &f); err != nil {
				t.Fatalf("Unpack frame %d: %v", i, err)
			}
			transform(&f)
			for j := range repacked {
				repacked[j] = 0
			}
			if err := bitstream.Pack(&f, repacked[:]); err != nil {
				t.Fatalf("Pack frame %d: %v", i, err)
			}
			use = repacked[:]
		}
		if err := dec.Decode(use, false, out[i*frameSamples:(i+1)*frameSamples]); err != nil {
			t.Fatalf("Decode frame %d: %v", i, err)
		}
	}
	return out
}

func reverseByte(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}

func reverseNibble(b byte) byte {
	b &= 0xF
	return ((b & 0x1) << 3) | ((b & 0x2) << 1) | ((b & 0x4) >> 1) | ((b & 0x8) >> 3)
}
