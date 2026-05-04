package g729

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase3Diag_UnpackedIndices_SPEECH dumps the 15 transmitted
// indices for the first 10 frames of SPEECH.BIT and computes range
// statistics across the full corpus. Purpose: verify the G.192 →
// packed → bitstream.Unpack path produces in-range field values that
// match the ITU-T G.729 §4 / Annex A Table 8 transmission order.
//
// Out-of-range values for any field would indicate a wire-format
// (G.192 → packed-bit) defect upstream of every numerical decoder
// stage — a strong, low-cost falsification target before suspecting
// the LSP / pitch / FCB / gain numerical paths.
//
// Informational: t.Logf only.
func TestPhase3Diag_UnpackedIndices_SPEECH(t *testing.T) {
	bitPath := filepath.Join("testdata/itu/G729_Release3/g729AnnexA/test_vectors", "SPEECH.BIT")
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}

	r := bytes.NewReader(bitData)
	var packed [FrameBytes]byte

	const showFrames = 10
	t.Logf("First %d frames — transmitted indices (G.729 §4 / Annex A Table 8 order):", showFrames)
	t.Logf("%4s %2s %3s %3s %3s %3s %2s %5s %3s %3s %3s %3s %5s %3s %3s %3s",
		"f", "L0", "L1", "L2", "L3", "P1", "P0", "C1", "S1", "GA1", "GB1", "P2", "C2", "S2", "GA2", "GB2")

	type stats struct {
		min, max int
		bits     int
	}
	fields := []struct {
		name string
		bits int
	}{
		{"L0", 1}, {"L1", 7}, {"L2", 5}, {"L3", 5},
		{"P1", 8}, {"P0", 1},
		{"C1", 13}, {"S1", 4}, {"GA1", 3}, {"GB1", 4},
		{"P2", 5}, {"C2", 13}, {"S2", 4}, {"GA2", 3}, {"GB2", 4},
	}
	st := make([]stats, len(fields))
	for i := range st {
		st[i].min = 1 << 30
		st[i].max = -(1 << 30)
		st[i].bits = fields[i].bits
	}

	frames := 0
	for {
		if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
			break // EOF
		}
		var f bitstream.Frame
		if err := bitstream.Unpack(packed[:], &f); err != nil {
			t.Fatalf("Unpack frame %d: %v", frames, err)
		}
		vals := []int{
			int(f.L0), int(f.L1), int(f.L2), int(f.L3),
			int(f.P1), int(f.P0),
			int(f.C1), int(f.S1), int(f.GA1), int(f.GB1),
			int(f.P2), int(f.C2), int(f.S2), int(f.GA2), int(f.GB2),
		}
		for i, v := range vals {
			if v < st[i].min {
				st[i].min = v
			}
			if v > st[i].max {
				st[i].max = v
			}
		}

		if frames < showFrames {
			t.Logf("%4d %2d %3d %3d %3d %3d %2d %5d %3d %3d %3d %3d %5d %3d %3d %3d",
				frames,
				f.L0, f.L1, f.L2, f.L3, f.P1, f.P0,
				f.C1, f.S1, f.GA1, f.GB1,
				f.P2, f.C2, f.S2, f.GA2, f.GB2)
		}
		frames++
	}

	t.Logf("")
	t.Logf("Range statistics across %d frames (max permitted by bit-width in []):", frames)
	for i, fld := range fields {
		t.Logf("  %-3s : observed [%d, %d]   permitted [0, %d]",
			fld.name, st[i].min, st[i].max, (1<<uint(fld.bits))-1)
	}
}
