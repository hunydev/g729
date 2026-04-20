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

func TestReadG192Frame_GoodZeroFrame(t *testing.T) {
// Build a valid G.192 representation of an all-zero frame.
words := make([]uint16, G192FrameWords)
words[0] = G192SyncGood
words[1] = FrameBits
for i := 0; i < FrameBits; i++ {
words[2+i] = G192Bit0
}
var buf bytes.Buffer
if err := binary.Write(&buf, binary.LittleEndian, words); err != nil {
t.Fatalf("binary.Write: %v", err)
}

var frame [FrameBytes]byte
bad, err := ReadG192Frame(&buf, frame[:])
if err != nil {
t.Fatalf("ReadG192Frame: %v", err)
}
if bad {
t.Errorf("bad = true, want false")
}
for i, b := range frame {
if b != 0 {
t.Errorf("frame[%d] = %#x, want 0", i, b)
}
}
}

func TestReadG192Frame_BadFlagPropagates(t *testing.T) {
words := make([]uint16, G192FrameWords)
words[0] = G192SyncBad
words[1] = FrameBits
for i := 0; i < FrameBits; i++ {
words[2+i] = G192Bit0
}
var buf bytes.Buffer
_ = binary.Write(&buf, binary.LittleEndian, words)

var frame [FrameBytes]byte
bad, err := ReadG192Frame(&buf, frame[:])
if err != nil {
t.Fatalf("ReadG192Frame: %v", err)
}
if !bad {
t.Errorf("bad = false, want true")
}
}

func TestReadG192Frame_FirstBitSet(t *testing.T) {
words := make([]uint16, G192FrameWords)
words[0] = G192SyncGood
words[1] = FrameBits
words[2] = G192Bit1
for i := 1; i < FrameBits; i++ {
words[2+i] = G192Bit0
}
var buf bytes.Buffer
_ = binary.Write(&buf, binary.LittleEndian, words)

var frame [FrameBytes]byte
if _, err := ReadG192Frame(&buf, frame[:]); err != nil {
t.Fatalf("ReadG192Frame: %v", err)
}
want := [FrameBytes]byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
if frame != want {
t.Errorf("frame = % x, want % x", frame, want)
}
}

func TestReadG192Frame_BadSync(t *testing.T) {
words := make([]uint16, G192FrameWords)
words[0] = 0xFFFF
words[1] = FrameBits
var buf bytes.Buffer
_ = binary.Write(&buf, binary.LittleEndian, words)

var frame [FrameBytes]byte
if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Sync) {
t.Errorf("err = %v, want ErrBadG192Sync", err)
}
}

func TestReadG192Frame_BadLength(t *testing.T) {
words := make([]uint16, G192FrameWords)
words[0] = G192SyncGood
words[1] = 40 // not FrameBits
var buf bytes.Buffer
_ = binary.Write(&buf, binary.LittleEndian, words)

var frame [FrameBytes]byte
if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Length) {
t.Errorf("err = %v, want ErrBadG192Length", err)
}
}

func TestReadG192Frame_BadDataWord(t *testing.T) {
words := make([]uint16, G192FrameWords)
words[0] = G192SyncGood
words[1] = FrameBits
words[2] = 0xDEAD
var buf bytes.Buffer
_ = binary.Write(&buf, binary.LittleEndian, words)

var frame [FrameBytes]byte
if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Bit) {
t.Errorf("err = %v, want ErrBadG192Bit", err)
}
}

func TestReadG192Frame_EOF(t *testing.T) {
var empty bytes.Buffer
var frame [FrameBytes]byte
if _, err := ReadG192Frame(&empty, frame[:]); !errors.Is(err, io.EOF) {
t.Errorf("err = %v, want io.EOF", err)
}
}

func TestG192RoundTrip(t *testing.T) {
original := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
var buf bytes.Buffer
if err := WriteG192Frame(&buf, original, false); err != nil {
t.Fatalf("WriteG192Frame: %v", err)
}
var got [FrameBytes]byte
bad, err := ReadG192Frame(&buf, got[:])
if err != nil {
t.Fatalf("ReadG192Frame: %v", err)
}
if bad {
t.Errorf("bad = true, want false")
}
if !bytes.Equal(got[:], original) {
t.Errorf("round-trip: got % x, want % x", got, original)
}
}

func TestReadG192File_MultipleFrames(t *testing.T) {
frames := [][]byte{
{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A},
{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90, 0xA0},
}
bads := []bool{false, true}

var buf bytes.Buffer
for i, f := range frames {
if err := WriteG192Frame(&buf, f, bads[i]); err != nil {
t.Fatalf("WriteG192Frame[%d]: %v", i, err)
}
}

gotFrames, gotBads, err := ReadG192File(&buf)
if err != nil {
t.Fatalf("ReadG192File: %v", err)
}
if len(gotFrames) != len(frames) {
t.Fatalf("frame count = %d, want %d", len(gotFrames), len(frames))
}
for i := range frames {
if !bytes.Equal(gotFrames[i], frames[i]) {
t.Errorf("frame[%d] = % x, want % x", i, gotFrames[i], frames[i])
}
if gotBads[i] != bads[i] {
t.Errorf("bad[%d] = %v, want %v", i, gotBads[i], bads[i])
}
}
}

func TestReadG192File_Empty(t *testing.T) {
var buf bytes.Buffer
frames, bads, err := ReadG192File(&buf)
if err != nil {
t.Fatalf("ReadG192File empty: %v", err)
}
if len(frames) != 0 || len(bads) != 0 {
t.Errorf("empty file -> %d frames, want 0", len(frames))
}
}
