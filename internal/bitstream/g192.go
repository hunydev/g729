package bitstream

import (
	"encoding/binary"
	"io"
)

// G.192 serial bitstream word values (ITU-T G.191 STL).
const (
	// G192SyncGood starts a correctly received frame.
	G192SyncGood uint16 = 0x6B21
	// G192SyncBad starts a frame-erasure marker (bad frame).
	G192SyncBad uint16 = 0x6B20
	// G192Bit1 represents a logical 1 source bit.
	G192Bit1 uint16 = 0x0081
	// G192Bit0 represents a logical 0 source bit.
	G192Bit0 uint16 = 0x007F

	// G192FrameWords is the number of 16-bit words per G.192 frame
	// (1 sync + 1 length + FrameBits data words).
	G192FrameWords = 2 + FrameBits
	// G192FrameBytes is the on-disk size of one G.192 frame in bytes
	// (little-endian 16-bit words).
	G192FrameBytes = 2 * G192FrameWords
)

// WriteG192Frame writes one G.192-formatted frame to w. frame must be
// exactly FrameBytes long and hold a packed G.729 frame in the wire
// format produced by Pack. If bad is true, the erasure sync marker is
// emitted instead of the good-frame marker.
//
// Allocates one G192FrameBytes-sized buffer internally.
func WriteG192Frame(w io.Writer, frame []byte, bad bool) error {
	if len(frame) < FrameBytes {
		return ErrShortInput
	}
	words := make([]uint16, G192FrameWords)
	if bad {
		words[0] = G192SyncBad
	} else {
		words[0] = G192SyncGood
	}
	words[1] = FrameBits

	for i := 0; i < FrameBits; i++ {
		byteIdx := i >> 3
		bitIdx := 7 - (i & 7)
		if (frame[byteIdx]>>uint(bitIdx))&1 == 1 {
			words[2+i] = G192Bit1
		} else {
			words[2+i] = G192Bit0
		}
	}

	return binary.Write(w, binary.LittleEndian, words)
}

// ReadG192Frame reads one G.192 frame from r. frame must be at least
// FrameBytes long and is overwritten with the packed bit pattern.
// Returns bad = true if the sync word indicated a frame erasure.
//
// Returns io.EOF if the reader is empty at the start of a frame, or
// io.ErrUnexpectedEOF if the reader ends mid-frame. Returns
// ErrBadG192Sync / ErrBadG192Length / ErrBadG192Bit if the stream
// content does not match the G.192 conventions.
//
// Allocates one G192FrameBytes-sized buffer internally.
func ReadG192Frame(r io.Reader, frame []byte) (bool, error) {
if len(frame) < FrameBytes {
return false, ErrShortOutput
}
words := make([]uint16, G192FrameWords)
if err := binary.Read(r, binary.LittleEndian, words); err != nil {
return false, err
}

var bad bool
switch words[0] {
case G192SyncGood:
bad = false
case G192SyncBad:
bad = true
default:
return false, ErrBadG192Sync
}
if words[1] != FrameBits {
return false, ErrBadG192Length
}

out := frame[:FrameBytes]
for i := range out {
out[i] = 0
}
for i := 0; i < FrameBits; i++ {
switch words[2+i] {
case G192Bit1:
byteIdx := i >> 3
bitIdx := 7 - (i & 7)
out[byteIdx] |= 1 << uint(bitIdx)
case G192Bit0:
// nothing to set
default:
return false, ErrBadG192Bit
}
}
return bad, nil
}
