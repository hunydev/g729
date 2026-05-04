package g729

import (
	"bytes"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase2fPACK1_BuildBitstreamFrame_FieldCopy asserts that
// (*Encoder).buildBitstreamFrame copies all 15 per-frame field values
// (l0..l3, p1, p0, c1, s1, ga1, gb1, p2, c2, s2, ga2, gb2) into the
// destination bitstream.Frame in the §4.2.1 Table 8 mapping.
func TestPhase2fPACK1_BuildBitstreamFrame_FieldCopy(t *testing.T) {
	e := NewEncoder()
	e.l0 = 1
	e.l1 = 42
	e.l2 = 17
	e.l3 = 9
	e.p1 = 200
	e.p0 = 1
	e.c1 = 0x1ABC
	e.s1 = 0xA
	e.ga1 = 5
	e.gb1 = 11
	e.p2 = 27
	e.c2 = 0x0123
	e.s2 = 0x3
	e.ga2 = 2
	e.gb2 = 8

	var f bitstream.Frame
	e.buildBitstreamFrame(&f)

	want := bitstream.Frame{
		L0: 1, L1: 42, L2: 17, L3: 9,
		P1: 200, P0: 1,
		C1: 0x1ABC, S1: 0xA, GA1: 5, GB1: 11,
		P2: 27,
		C2: 0x0123, S2: 0x3, GA2: 2, GB2: 8,
	}
	if f != want {
		t.Errorf("buildBitstreamFrame: got %+v, want %+v", f, want)
	}
}

// TestPhase2fPACK1_PackBytes_HandDerived asserts that piping the
// per-frame fields through buildBitstreamFrame + bitstream.Pack
// yields the exact 10-byte MSB-first sequence derived by hand from
// the §4.2.1 Table 8 layout.
//
// Bit layout (80 bits, MSB-first within byte, transmission order
// across bytes — Table 8):
//
//	L0(1) | L1(7) | L2(5) | L3(5) | P1(8) | P0(1) | C1(13) | S1(4)
//	  | GA1(3) | GB1(4) | P2(5) | C2(13) | S2(4) | GA2(3) | GB2(4)
//
// Concatenating the chosen indices gives:
//
//	1 0101010 10001 01001 11001000 1 1101010111100 1010 101 1011
//	  11011 0000100100011 0011 010 1000
//
// = 10101010 10001010 01110010 00111010 10111100 10101011
//
//	01111011 00001001 00011001 10101000
//
// = AA 8A 72 3A BC AB 7B 09 19 A8
func TestPhase2fPACK1_PackBytes_HandDerived(t *testing.T) {
	e := NewEncoder()
	e.l0 = 1
	e.l1 = 42
	e.l2 = 17
	e.l3 = 9
	e.p1 = 200
	e.p0 = 1
	e.c1 = 0x1ABC
	e.s1 = 0xA
	e.ga1 = 5
	e.gb1 = 11
	e.p2 = 27
	e.c2 = 0x0123
	e.s2 = 0x3
	e.ga2 = 2
	e.gb2 = 8

	var f bitstream.Frame
	e.buildBitstreamFrame(&f)

	var out [bitstream.FrameBytes]byte
	if err := bitstream.Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	want := [bitstream.FrameBytes]byte{
		0xAA, 0x8A, 0x72, 0x3A, 0xBC,
		0xAB, 0x7B, 0x09, 0x19, 0xA8,
	}
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(buildBitstreamFrame): got % X, want % X", out, want)
	}
}

// TestPhase2fPACK1_PackUnpackRoundTrip confirms that the encoder-side
// frame, packed to 10 bytes and then unpacked via the decoder-side
// bitstream.Unpack, recovers every field — i.e., the encoder packer
// is the byte-exact inverse of the decoder unpacker.
func TestPhase2fPACK1_PackUnpackRoundTrip(t *testing.T) {
	e := NewEncoder()
	e.l0 = 1
	e.l1 = 0x7F
	e.l2 = 0x1F
	e.l3 = 0x1F
	e.p1 = 0xFF
	e.p0 = 1
	e.c1 = 0x1FFF
	e.s1 = 0xF
	e.ga1 = 7
	e.gb1 = 0xF
	e.p2 = 0x1F
	e.c2 = 0x1FFF
	e.s2 = 0xF
	e.ga2 = 7
	e.gb2 = 0xF

	var f bitstream.Frame
	e.buildBitstreamFrame(&f)

	var out [bitstream.FrameBytes]byte
	if err := bitstream.Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	for i, b := range out {
		if b != 0xFF {
			t.Errorf("out[%d] = %#x, want 0xFF (every bit field at max)", i, b)
		}
	}

	var got bitstream.Frame
	if err := bitstream.Unpack(out[:], &got); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if got != f {
		t.Errorf("round-trip: got %+v, want %+v", got, f)
	}
}

// TestPhase2fPACK1_BuildBitstreamFrame_ZeroAlloc enforces the
// zero-allocation contract for the per-frame field copy step.
func TestPhase2fPACK1_BuildBitstreamFrame_ZeroAlloc(t *testing.T) {
	e := NewEncoder()
	e.l0 = 1
	e.l1 = 42
	e.l2 = 17
	e.l3 = 9
	e.p1 = 200
	e.p0 = 1
	e.c1 = 0x1ABC
	e.s1 = 0xA
	e.ga1 = 5
	e.gb1 = 11
	e.p2 = 27
	e.c2 = 0x0123
	e.s2 = 0x3
	e.ga2 = 2
	e.gb2 = 8

	var f bitstream.Frame
	allocs := testing.AllocsPerRun(128, func() {
		e.buildBitstreamFrame(&f)
	})
	if allocs != 0 {
		t.Errorf("buildBitstreamFrame allocs/op = %v, want 0", allocs)
	}
}
