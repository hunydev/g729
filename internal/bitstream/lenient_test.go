package bitstream

// Phase 1o D-2 (variant F2) — lenient G.192 frame reader tests.
//
// Plan: docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md
// Measurement basis: internal/bitstream/phase1o_d2_overflow_diagnostic_test.go
// (commit 1e83d6b) characterizing OVERFLOW.BIT frame 19 as 80 0x0000
// data-words behind a canonical 0x6B21 sync + length=80 header.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// makeFrame writes one G.192 frame to a buffer with the given sync,
// length word, and explicit per-bit words (must be FrameBits long).
func makeFrame(t *testing.T, sync, length uint16, words []uint16) []byte {
	t.Helper()
	if len(words) != FrameBits {
		t.Fatalf("makeFrame: words len = %d, want %d", len(words), FrameBits)
	}
	var buf bytes.Buffer
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, sync)
	buf.Write(tmp)
	binary.LittleEndian.PutUint16(tmp, length)
	buf.Write(tmp)
	for _, w := range words {
		binary.LittleEndian.PutUint16(tmp, w)
		buf.Write(tmp)
	}
	return buf.Bytes()
}

func TestReadG192FrameLenient_AcceptsZeroSoftbit(t *testing.T) {
	words := make([]uint16, FrameBits)
	for i := range words {
		switch i % 3 {
		case 0:
			words[i] = 0x0000 // lenient-only: indeterminate softbit → logical 0
		case 1:
			words[i] = G192Bit0 // canonical 0
		case 2:
			words[i] = G192Bit1 // canonical 1
		}
	}
	raw := makeFrame(t, G192SyncGood, FrameBits, words)
	frame := make([]byte, FrameBytes)
	bad, err := ReadG192FrameLenient(bytes.NewReader(raw), frame)
	if err != nil {
		t.Fatalf("ReadG192FrameLenient: unexpected error %v", err)
	}
	if bad {
		t.Errorf("bad = true, want false for mixed zero softbits on good sync")
	}
	for i := 0; i < FrameBits; i++ {
		byteIdx := i >> 3
		bitIdx := 7 - (i & 7)
		got := (frame[byteIdx] >> uint(bitIdx)) & 1
		var want byte
		switch i % 3 {
		case 0, 1:
			want = 0
		case 2:
			want = 1
		}
		if got != want {
			t.Errorf("bit %d: got %d, want %d", i, got, want)
		}
	}
}

func TestReadG192FrameLenient_StillRejectsGarbage(t *testing.T) {
	words := make([]uint16, FrameBits)
	for i := range words {
		words[i] = G192Bit0
	}
	words[42] = 0x1234 // not in {0x0000, 0x007F, 0x0081}
	raw := makeFrame(t, G192SyncGood, FrameBits, words)
	frame := make([]byte, FrameBytes)
	_, err := ReadG192FrameLenient(bytes.NewReader(raw), frame)
	if !errors.Is(err, ErrBadG192Bit) {
		t.Fatalf("ReadG192FrameLenient: err = %v, want ErrBadG192Bit", err)
	}
}

func TestReadG192Frame_StrictStillRejectsZeroSoftbit(t *testing.T) {
	// Regression guard: strict reader must continue to reject 0x0000.
	words := make([]uint16, FrameBits)
	for i := range words {
		words[i] = G192Bit0
	}
	words[0] = 0x0000
	raw := makeFrame(t, G192SyncGood, FrameBits, words)
	frame := make([]byte, FrameBytes)
	_, err := ReadG192Frame(bytes.NewReader(raw), frame)
	if !errors.Is(err, ErrBadG192Bit) {
		t.Fatalf("ReadG192Frame strict: err = %v, want ErrBadG192Bit", err)
	}
}

func TestReadG192File_HandlesOverflowVector(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", "OVERFLOW.BIT")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open OVERFLOW.BIT: %v", err)
	}
	defer f.Close()

	frames, bads, err := ReadG192File(f)
	if err != nil {
		t.Fatalf("ReadG192File OVERFLOW.BIT: unexpected error %v", err)
	}
	if got, want := len(frames), 384; got != want {
		t.Fatalf("len(frames) = %d, want %d", got, want)
	}
	if len(bads) != len(frames) {
		t.Fatalf("len(bads) = %d, want %d", len(bads), len(frames))
	}
	// Frame 19 — characterized as all-zero softbit payload by the
	// D-2 diagnostic; under the lenient policy this maps to an
	// all-zero 10-byte packed payload.
	for i, b := range frames[19] {
		if b != 0 {
			t.Errorf("frame 19 byte %d = %#x, want 0", i, b)
		}
	}
	if !bads[19] {
		t.Errorf("bads[19] = false, want true for all-zero softbit frame")
	}
}

// Compile-time assertion that ReadG192FrameLenient has the same
// signature as ReadG192Frame so the two readers remain interchangeable.
var _ = func(r io.Reader, frame []byte) (bool, error) {
	return ReadG192FrameLenient(r, frame)
}
