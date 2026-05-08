// Example: encode_pcm
//
// Read raw 8 kHz mono int16 little-endian PCM from stdin, emit packed
// 10-byte G.729 frames to stdout. One frame per 80 input samples
// (10 ms at 8 kHz).
//
// Clean-room I1 declaration: this example uses only the public API of
// github.com/hunydev/g729 and the Go standard library. No ITU reference
// C, bcg729, FFmpeg, Sipro, or other G.729 implementation source was
// consulted.
//
// Usage:
//
//	go run ./examples/encode_pcm < input.pcm > output.g729
//
// Where input.pcm is raw int16 little-endian 8 kHz mono samples.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/hunydev/g729"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "encode_pcm:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	enc := g729.NewEncoder()

	pcm := make([]int16, g729.FrameSamples)
	bits := make([]byte, g729.FrameBytes)
	rawBuf := make([]byte, g729.FrameSamples*2)

	for {
		_, err := io.ReadFull(in, rawBuf)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("trailing partial frame: input not a multiple of %d bytes", g729.FrameSamples*2)
		}
		if err != nil {
			return err
		}

		for i := 0; i < g729.FrameSamples; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(rawBuf[i*2:]))
		}

		if err := enc.EncodeFrame(pcm, bits); err != nil {
			return err
		}

		if _, err := out.Write(bits); err != nil {
			return err
		}
	}
}
