package bitstream

import (
	"encoding/binary"
	"os"
	"testing"
)

// extractLSPFields reads the four LSP-side bit-fields (L0, L1, L2, L3)
// from one G.192-framed bitstream record per §A.4 Table A.4 transmission
// order. g192Frame must be at least G192FrameBytes long (sync + length
// + 80 softbit data words).
//
// Layout: data words start at byte offset 4 (after sync+length); each
// 16-bit little-endian word is a softbit (0x0081 → 1, anything else → 0).
// Bits 0..17 of the payload are MSB-first packed as L0(1) | L1(7) |
// L2(5) | L3(5) per Table A.4.
//
// Test-only helper (lives in _test.go); no production decoder.
func extractLSPFields(g192Frame []byte) (l0, l1, l2, l3 uint8) {
	bit := func(i int) uint8 {
		off := 4 + 2*i
		if binary.LittleEndian.Uint16(g192Frame[off:off+2]) == G192Bit1 {
			return 1
		}
		return 0
	}
	pack := func(start, n int) uint8 {
		var v uint8
		for i := 0; i < n; i++ {
			v = (v << 1) | bit(start+i)
		}
		return v
	}
	l0 = pack(0, 1)
	l1 = pack(1, 7)
	l2 = pack(8, 5)
	l3 = pack(13, 5)
	return
}

// TestExtractLSPFields_FirstFrameOracle validates the test-only
// extractLSPFields helper against a hand-decoded oracle for frame 0
// of the ITU LSP.BIT test vector.
//
// Oracle derivation (manual, in-spec):
//   - LSP.BIT is G.192 framed: 1 sync word (0x6B21) + 1 length word (0x0050=80)
//   - 80 softbit data words per frame, little-endian on disk.
//   - Softbit map: 0x0081 → 1, 0x007F → 0 (G.191 STL convention).
//   - §A.4 Table A.4 transmission order places L0 (1b), L1 (7b), L2 (5b),
//     L3 (5b) at bit positions 0..17 of the 80-bit payload, MSB-first.
//   - Reading frame 0 of LSP.BIT yields the bit sequence
//     0,1,1,1,1,0,0,0,0,1,0,1,0,0,1,0,1,0 → L0=0, L1=120, L2=10, L3=10.
//
// The helper consumes the full 164-byte G.192 frame (sync+len+data) so
// callers do not need to know the on-disk layout.
func TestExtractLSPFields_FirstFrameOracle(t *testing.T) {
	const path = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) < G192FrameBytes {
		t.Fatalf("LSP.BIT too short: %d < %d", len(data), G192FrameBytes)
	}

	gotL0, gotL1, gotL2, gotL3 := extractLSPFields(data[:G192FrameBytes])

	const wantL0, wantL1, wantL2, wantL3 uint8 = 0, 120, 10, 10
	if gotL0 != wantL0 || gotL1 != wantL1 || gotL2 != wantL2 || gotL3 != wantL3 {
		t.Errorf("frame0 LSP fields: got (L0=%d,L1=%d,L2=%d,L3=%d), want (L0=%d,L1=%d,L2=%d,L3=%d)",
			gotL0, gotL1, gotL2, gotL3, wantL0, wantL1, wantL2, wantL3)
	}
}
