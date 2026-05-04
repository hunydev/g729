// Example: streaming_encode
//
// Same as examples/encode_pcm, but uses the streaming API
// (NewStreamingEncoder + Write + Flush). Demonstrates buffered
// encoding when the input arrives in chunks that are not aligned to
// the 80-sample frame boundary.
//
// Clean-room I1 declaration: this example uses only the public API of
// github.com/exedev/g729 and the Go standard library. No ITU reference
// C, bcg729, FFmpeg, Sipro, or other G.729 implementation source was
// consulted.
//
// Usage:
//
//	go run ./examples/streaming_encode < input.pcm > output.g729
//
// Where input.pcm is raw int16 little-endian 8 kHz mono samples.
// Trailing partial frames (fewer than 80 samples) are zero-padded by
// Flush.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/exedev/g729"
)

const chunkSamples = 240 // arbitrary, intentionally non-multiple of 80 to exercise buffering

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "streaming_encode:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	enc := g729.NewStreamingEncoder(out)

	rawBuf := make([]byte, chunkSamples*2)
	pcm := make([]int16, chunkSamples)

	for {
		n, err := io.ReadFull(in, rawBuf)
		eof := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
		if err != nil && !eof {
			return err
		}

		samples := n / 2
		for i := 0; i < samples; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(rawBuf[i*2:]))
		}
		if samples > 0 {
			if _, werr := enc.Write(pcm[:samples]); werr != nil {
				return werr
			}
		}

		if eof {
			return enc.Flush()
		}
	}
}
