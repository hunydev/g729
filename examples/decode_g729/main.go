// Example: decode_g729
//
// Read packed 10-byte G.729 frames from stdin, emit raw 8 kHz mono
// int16 little-endian PCM to stdout. One 80-sample (10 ms) PCM block
// per input frame.
//
// Clean-room I1 declaration: this example uses only the public API of
// github.com/exedev/g729 and the Go standard library. No ITU reference
// C, bcg729, FFmpeg, Sipro, or other G.729 implementation source was
// consulted.
//
// Usage:
//
//	go run ./examples/decode_g729 < input.g729 > output.pcm
//
// Where output.pcm is raw int16 little-endian 8 kHz mono samples.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/exedev/g729"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "decode_g729:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	dec := g729.NewDecoder()

	bits := make([]byte, g729.FrameBytes)
	pcm := make([]int16, g729.FrameSamples)
	rawBuf := make([]byte, g729.FrameSamples*2)

	for {
		_, err := io.ReadFull(in, bits)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("trailing partial frame: input not a multiple of %d bytes", g729.FrameBytes)
		}
		if err != nil {
			return err
		}

		if err := dec.DecodeFrame(bits, pcm); err != nil {
			return err
		}

		for i := 0; i < g729.FrameSamples; i++ {
			binary.LittleEndian.PutUint16(rawBuf[i*2:], uint16(pcm[i]))
		}

		if _, err := out.Write(rawBuf); err != nil {
			return err
		}
	}
}
