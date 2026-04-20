package bitstream

import (
	"bytes"
	"io"
	"testing"
)

func TestNoAllocation_PackUnpackParity(t *testing.T) {
	var f Frame
	f.L0 = 1
	f.P1 = 0x55
	var out [FrameBytes]byte

	cases := []struct {
		name string
		fn   func()
	}{
		{"Pack", func() { _ = Pack(&f, out[:]) }},
		{"Unpack", func() {
			var got Frame
			_ = Unpack(out[:], &got)
		}},
		{"Parity", func() { _ = Parity(f.P1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(1000, tc.fn)
			if allocs != 0 {
				t.Errorf("%s allocated %.2f times per call, want 0", tc.name, allocs)
			}
		})
	}
}

func TestNoAllocation_G192IO(t *testing.T) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}

	// Pre-build a valid G.192 frame we can read back repeatedly.
	var encoded bytes.Buffer
	encoded.Grow(G192FrameBytes)
	if err := WriteG192Frame(&encoded, frame, false); err != nil {
		t.Fatalf("WriteG192Frame setup: %v", err)
	}
	encodedBytes := encoded.Bytes()

	// discardWriter is an io.Writer that never allocates; bytes.Buffer may
	// grow on the first call, so we re-use a pre-sized buffer and Reset it
	// between iterations.
	var writeBuf bytes.Buffer
	writeBuf.Grow(G192FrameBytes)

	var readBuf [FrameBytes]byte
	var br bytes.Reader

	writeFn := func() {
		writeBuf.Reset()
		_ = WriteG192Frame(&writeBuf, frame, false)
	}
	readFn := func() {
		br.Reset(encodedBytes)
		_, _ = ReadG192Frame(&br, readBuf[:])
	}

	cases := []struct {
		name string
		fn   func()
	}{
		{"WriteG192Frame", writeFn},
		{"ReadG192Frame", readFn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(1000, tc.fn)
			if allocs != 0 {
				t.Errorf("%s allocated %.2f times per call, want 0", tc.name, allocs)
			}
		})
	}
}

// Compile-time assertion that bytes.NewReader still satisfies io.Reader.
var _ io.Reader = (*bytes.Reader)(nil)
