package bitstream

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestG192Constants(t *testing.T) {
	if G192SyncGood != 0x6B21 {
		t.Errorf("G192SyncGood = %#x, want 0x6B21", G192SyncGood)
	}
	if G192SyncBad != 0x6B20 {
		t.Errorf("G192SyncBad = %#x, want 0x6B20", G192SyncBad)
	}
	if G192Bit1 != 0x0081 {
		t.Errorf("G192Bit1 = %#x, want 0x0081", G192Bit1)
	}
	if G192Bit0 != 0x007F {
		t.Errorf("G192Bit0 = %#x, want 0x007F", G192Bit0)
	}
	if G192FrameWords != 2+FrameBits {
		t.Errorf("G192FrameWords = %d, want %d", G192FrameWords, 2+FrameBits)
	}
	if G192FrameBytes != 2*G192FrameWords {
		t.Errorf("G192FrameBytes = %d, want %d", G192FrameBytes, 2*G192FrameWords)
	}
}

func TestWriteG192Frame_AllZero(t *testing.T) {
	var frame [FrameBytes]byte
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame[:], false); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	// Decode the stream: 82 LE uint16 words.
	if buf.Len() != G192FrameBytes {
		t.Fatalf("buf.Len = %d, want %d", buf.Len(), G192FrameBytes)
	}
	words := make([]uint16, G192FrameWords)
	if err := binary.Read(&buf, binary.LittleEndian, words); err != nil {
		t.Fatalf("binary.Read: %v", err)
	}
	if words[0] != G192SyncGood {
		t.Errorf("sync = %#x, want %#x", words[0], G192SyncGood)
	}
	if words[1] != FrameBits {
		t.Errorf("length = %d, want %d", words[1], FrameBits)
	}
	for i := 2; i < G192FrameWords; i++ {
		if words[i] != G192Bit0 {
			t.Errorf("words[%d] = %#x, want %#x (bit 0)", i, words[i], G192Bit0)
		}
	}
}

func TestWriteG192Frame_BadFlagsSync(t *testing.T) {
	var frame [FrameBytes]byte
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame[:], true); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	var sync uint16
	if err := binary.Read(&buf, binary.LittleEndian, &sync); err != nil {
		t.Fatalf("binary.Read sync: %v", err)
	}
	if sync != G192SyncBad {
		t.Errorf("sync = %#x, want %#x", sync, G192SyncBad)
	}
}

func TestWriteG192Frame_BitsMatchInput(t *testing.T) {
	// First bit (MSB of byte 0) = 1, rest = 0.
	frame := []byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame, false); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	words := make([]uint16, G192FrameWords)
	if err := binary.Read(&buf, binary.LittleEndian, words); err != nil {
		t.Fatalf("binary.Read: %v", err)
	}
	if words[2] != G192Bit1 {
		t.Errorf("words[2] = %#x, want %#x (first data bit)", words[2], G192Bit1)
	}
	for i := 3; i < G192FrameWords; i++ {
		if words[i] != G192Bit0 {
			t.Errorf("words[%d] = %#x, want %#x", i, words[i], G192Bit0)
		}
	}
}

func TestWriteG192Frame_ShortFrame(t *testing.T) {
	short := make([]byte, FrameBytes-1)
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, short, false); !errors.Is(err, ErrShortInput) {
		t.Errorf("WriteG192Frame short = %v, want ErrShortInput", err)
	}
}

// Placeholder for stdlib; keeps the test file self-contained even if
// editors auto-import differently.
var _ = io.Discard
