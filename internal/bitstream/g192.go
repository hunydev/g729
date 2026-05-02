package bitstream

import (
	"encoding/binary"
	"io"
	"sync"
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
// Zero-allocation: the implementation serializes through a pooled
// G192FrameBytes-sized buffer and performs a single w.Write call.
func WriteG192Frame(w io.Writer, frame []byte, bad bool) error {
	if len(frame) < FrameBytes {
		return ErrShortInput
	}

	bufp := g192BufPool.Get().(*[G192FrameBytes]byte)
	defer g192BufPool.Put(bufp)
	buf := bufp[:]

	if bad {
		binary.LittleEndian.PutUint16(buf[0:2], G192SyncBad)
	} else {
		binary.LittleEndian.PutUint16(buf[0:2], G192SyncGood)
	}
	binary.LittleEndian.PutUint16(buf[2:4], FrameBits)

	for i := 0; i < FrameBits; i++ {
		byteIdx := i >> 3
		bitIdx := 7 - (i & 7)
		word := G192Bit0
		if (frame[byteIdx]>>uint(bitIdx))&1 == 1 {
			word = G192Bit1
		}
		// Data words begin at byte offset 4 (after sync + length).
		off := 4 + 2*i
		binary.LittleEndian.PutUint16(buf[off:off+2], word)
	}

	_, err := w.Write(buf)
	return err
}

// g192BufPool holds reusable G192FrameBytes-sized scratch buffers used
// by WriteG192Frame and ReadG192Frame to keep them zero-allocation. A
// fixed-size byte array would otherwise escape to the heap because
// io.Writer.Write / io.ReadFull are interface calls and Go's escape
// analysis conservatively heap-allocates any address handed to an
// interface method.
var g192BufPool = sync.Pool{
	New: func() any { return new([G192FrameBytes]byte) },
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
// Strict policy: only the canonical softbit markers 0x007F (logical 0)
// and 0x0081 (logical 1) are accepted. Any other 16-bit data word —
// including 0x0000 — yields ErrBadG192Bit. See ReadG192FrameLenient
// for the relaxed variant used by the ITU test-vector loader.
//
// Zero-allocation: the implementation reads into a pooled
// G192FrameBytes-sized buffer via a single io.ReadFull call.
func ReadG192Frame(r io.Reader, frame []byte) (bool, error) {
	return readG192FrameWithSoftbitPolicy(r, frame, false)
}

// ReadG192FrameLenient reads one G.192 frame from r with a relaxed
// softbit policy: the data words 0x0000 are additionally accepted and
// mapped to logical 0, alongside the canonical 0x007F (logical 0) and
// 0x0081 (logical 1). All other validation (sync word, length word,
// frame-bits count, error semantics) is identical to ReadG192Frame.
//
// Rationale (informed inference, not chapter-and-verse):
// the ITU G.729A test vector OVERFLOW.BIT contains exactly one frame
// (index 19) whose 80 data-words are all 0x0000 behind a canonical
// 0x6B21 sync + length=80 header. Strict parsing rejects this frame
// with ErrBadG192Bit; the rest of the file is bit-perfect G.192. See
// internal/bitstream/phase1o_d2_overflow_diagnostic_test.go (commit
// 1e83d6b) for the full structural survey:
//
//   "Across all 30 720 bit-words (384 × 80), the only values observed
//    are 0x007F (~17 505), 0x0081 (~13 135), and 0x0000 (exactly 80,
//    ALL concentrated in a single frame)."
//
// G.191 STL (per the broader ITU softbit convention referenced in
// textbook treatments and STL2009 source documentation) defines
// 0x0000 as the "indeterminate" / "unknown" softbit. We cannot quote
// chapter-and-verse without the G.191 PDF in-repo, so this lenient
// mapping is recorded as informed inference, not citation.
func ReadG192FrameLenient(r io.Reader, frame []byte) (bool, error) {
	return readG192FrameWithSoftbitPolicy(r, frame, true)
}

// readG192FrameWithSoftbitPolicy is the shared implementation of the
// strict and lenient G.192 frame readers. When allowZeroSoftbit is
// true, a data word of 0x0000 is accepted and treated as logical 0;
// when false, only the canonical 0x007F / 0x0081 markers are valid.
func readG192FrameWithSoftbitPolicy(r io.Reader, frame []byte, allowZeroSoftbit bool) (bool, error) {
	if len(frame) < FrameBytes {
		return false, ErrShortOutput
	}

	bufp := g192BufPool.Get().(*[G192FrameBytes]byte)
	defer g192BufPool.Put(bufp)
	buf := bufp[:]

	if _, err := io.ReadFull(r, buf); err != nil {
		return false, err
	}

	sync := binary.LittleEndian.Uint16(buf[0:2])
	var bad bool
	switch sync {
	case G192SyncGood:
		bad = false
	case G192SyncBad:
		bad = true
	default:
		return false, ErrBadG192Sync
	}
	if binary.LittleEndian.Uint16(buf[2:4]) != FrameBits {
		return false, ErrBadG192Length
	}

	out := frame[:FrameBytes]
	for i := range out {
		out[i] = 0
	}
	for i := 0; i < FrameBits; i++ {
		off := 4 + 2*i
		word := binary.LittleEndian.Uint16(buf[off : off+2])
		switch word {
		case G192Bit1:
			byteIdx := i >> 3
			bitIdx := 7 - (i & 7)
			out[byteIdx] |= 1 << uint(bitIdx)
		case G192Bit0:
			// nothing to set
		case 0x0000:
			if !allowZeroSoftbit {
				return false, ErrBadG192Bit
			}
			// lenient: indeterminate softbit → logical 0
		default:
			return false, ErrBadG192Bit
		}
	}
	return bad, nil
}

// ReadG192File reads G.192 frames from r until EOF. It returns a slice
// of packed frame bytes (each FrameBytes long) and a parallel slice of
// bad-frame flags. A clean EOF at a frame boundary terminates reading
// normally. A truncated last frame returns io.ErrUnexpectedEOF.
//
// Intended for loading ITU test-vector .bit files, not for the hot
// path: it allocates one backing buffer for the full output.
//
// Softbit policy: ReadG192File uses ReadG192FrameLenient internally so
// that the ITU G.729A OVERFLOW.BIT vector — which carries one frame
// (index 19) of 80 indeterminate 0x0000 softbit words behind otherwise
// canonical headers — loads cleanly. See ReadG192FrameLenient for the
// lenience rationale and provenance (informed inference from G.191 STL
// "indeterminate softbit" convention; not chapter-and-verse). Callers
// that require strict 0x007F/0x0081-only validation should drive
// ReadG192Frame directly.
func ReadG192File(r io.Reader) ([][]byte, []bool, error) {
	var frames [][]byte
	var bads []bool
	for {
		frame := make([]byte, FrameBytes)
		bad, err := ReadG192FrameLenient(r, frame)
		if err == io.EOF {
			return frames, bads, nil
		}
		if err != nil {
			return frames, bads, err
		}
		frames = append(frames, frame)
		bads = append(bads, bad)
	}
}
