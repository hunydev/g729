package g729

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// Phase 2f API-2 RED: streaming Write/Flush wrapper.
//
// Plan §6 Task API-2 + §1 I-2f-3 + OQ-FLUSH-PAD (zero-pad with 0x0000).

func encodeFramesReference(t *testing.T, samples []int16) []byte {
	t.Helper()
	if len(samples)%FrameSamples != 0 {
		t.Fatalf("encodeFramesReference: len=%d not multiple of %d", len(samples), FrameSamples)
	}
	enc := NewEncoder()
	out := make([]byte, 0, (len(samples)/FrameSamples)*FrameBytes)
	var frame [FrameBytes]byte
	for i := 0; i < len(samples); i += FrameSamples {
		if err := enc.EncodeFrame(samples[i:i+FrameSamples], frame[:]); err != nil {
			t.Fatalf("EncodeFrame: %v", err)
		}
		out = append(out, frame[:]...)
	}
	return out
}

func makeRamp(n int) []int16 {
	s := make([]int16, n)
	for i := range s {
		s[i] = int16((i*37 + 11) & 0x7fff)
	}
	return s
}

func TestStreamingEncoder_FullFrames_ByteEqual(t *testing.T) {
	samples := makeRamp(240)
	want := encodeFramesReference(t, samples)

	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)
	n, err := se.Write(samples)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(samples) {
		t.Fatalf("Write n: got %d want %d", n, len(samples))
	}
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("streamed bytes differ from EncodeFrame reference\n got=% x\nwant=% x", buf.Bytes(), want)
	}
	if buf.Len() != 30 {
		t.Fatalf("buf.Len: got %d want 30", buf.Len())
	}
}

func TestStreamingEncoder_ChunkedWrites_ByteEqual(t *testing.T) {
	samples := makeRamp(80 * 5) // 5 frames
	want := encodeFramesReference(t, samples)

	chunkSizes := []int{1, 80, 79, 81, 7, 13, 50, 50, 39, 1, 9, 1}
	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)

	off := 0
	idx := 0
	for off < len(samples) {
		size := chunkSizes[idx%len(chunkSizes)]
		idx++
		if off+size > len(samples) {
			size = len(samples) - off
		}
		n, err := se.Write(samples[off : off+size])
		if err != nil {
			t.Fatalf("Write off=%d size=%d: %v", off, size, err)
		}
		if n != size {
			t.Fatalf("Write n: got %d want %d", n, size)
		}
		off += size
	}
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("chunked stream differs from single-Write reference\n got=% x\nwant=% x", buf.Bytes(), want)
	}
}

func TestStreamingEncoder_WriteZero_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)
	n, err := se.Write(nil)
	if err != nil || n != 0 {
		t.Fatalf("Write nil: n=%d err=%v", n, err)
	}
	n, err = se.Write([]int16{})
	if err != nil || n != 0 {
		t.Fatalf("Write []int16{}: n=%d err=%v", n, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buf.Len: got %d want 0", buf.Len())
	}
	// Flush on empty buffer is a no-op.
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush empty: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("after empty Flush buf.Len: got %d want 0", buf.Len())
	}
}

func TestStreamingEncoder_FlushZeroPads_PartialTail(t *testing.T) {
	// OQ-FLUSH-PAD: zero-pad partial tail to 80 samples and emit one
	// final frame (plan §1 I-2f-3 + §10 OQ-FLUSH-PAD pin = 0x0000).
	partialLen := 50
	tail := makeRamp(partialLen)

	// Reference: same encoder fed (tail || 30 zeros).
	padded := make([]int16, FrameSamples)
	copy(padded, tail)
	want := encodeFramesReference(t, padded)

	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)
	if _, err := se.Write(tail); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("pre-flush buf.Len: got %d want 0", buf.Len())
	}
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if buf.Len() != FrameBytes {
		t.Fatalf("post-flush buf.Len: got %d want %d", buf.Len(), FrameBytes)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("zero-pad flush bytes differ\n got=% x\nwant=% x", buf.Bytes(), want)
	}
}

func TestStreamingEncoder_MultipleWritesAccumulate(t *testing.T) {
	samples := makeRamp(80 * 3)
	want := encodeFramesReference(t, samples)

	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)
	// 30 + 30 + 20 = 80 (one frame), then 80, then 80. Chunks must
	// sum to exactly len(samples) and be written in input order so
	// the streamed bytes equal the contiguous reference.
	chunkSizes := []int{30, 30, 20, 80, 80}
	off := 0
	for _, size := range chunkSizes {
		if _, err := se.Write(samples[off : off+size]); err != nil {
			t.Fatalf("Write off=%d size=%d: %v", off, size, err)
		}
		off += size
	}
	if off != len(samples) {
		t.Fatalf("chunkSizes sum: got %d want %d", off, len(samples))
	}
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("multi-write stream differs from reference\n got=% x\nwant=% x", buf.Bytes(), want)
	}
}

func TestStreamingEncoder_ResetPreservesSinkAndClearsTail(t *testing.T) {
	var buf bytes.Buffer
	se := NewStreamingEncoder(&buf)

	if n, err := se.Write(makeRamp(17)); err != nil || n != 17 {
		t.Fatalf("partial Write before Reset: n=%d err=%v", n, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("partial Write emitted %d bytes, want 0", buf.Len())
	}

	se.Reset()
	frame := makeRamp(FrameSamples)
	want := encodeFramesReference(t, frame)
	if n, err := se.Write(frame); err != nil || n != len(frame) {
		t.Fatalf("Write after Reset: n=%d err=%v", n, err)
	}
	if err := se.Flush(); err != nil {
		t.Fatalf("Flush after Reset: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("Reset did not clear streaming tail or preserve sink\n got=% x\nwant=% x", buf.Bytes(), want)
	}
}

func TestStreamingEncoder_NoSinkErrors(t *testing.T) {
	e := NewEncoder()
	if n, err := e.Write(makeRamp(FrameSamples)); n != 0 || !errors.Is(err, ErrNoStreamSink) {
		t.Fatalf("Write without sink: n=%d err=%v", n, err)
	}
	if err := e.Flush(); !errors.Is(err, ErrNoStreamSink) {
		t.Fatalf("Flush without sink: err=%v", err)
	}
}

func TestStreamingEncoder_SteadyStateZeroAlloc(t *testing.T) {
	se := NewStreamingEncoder(io.Discard)
	pcm := makeRamp(FrameSamples)

	// Warm up to get past constructor / first-frame state init.
	for i := 0; i < 4; i++ {
		if _, err := se.Write(pcm); err != nil {
			t.Fatalf("warmup Write: %v", err)
		}
	}

	avg := testing.AllocsPerRun(128, func() {
		_, _ = se.Write(pcm)
	})
	if avg != 0 {
		t.Fatalf("steady-state allocs/op: got %v want 0", avg)
	}
}
