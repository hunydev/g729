// Example: rtp_packetize
//
// Illustrative-only RTP payload packetization for G.729 (payload
// type 18, "G729/8000"). Reads packed 10-byte G.729 frames from
// stdin and writes the resulting RTP payload byte sequences to
// stdout, one hex-dump line per RTP packet.
//
// This example does NOT depend on any external RTP library — it
// emits only the payload bytes (RTP header generation belongs to
// the caller's MRCP / SIP framework).
//
// Clean-room I1 declaration: this example uses only the public API of
// github.com/exedev/g729 and the Go standard library. No ITU reference
// C, bcg729, FFmpeg, Sipro, or other G.729 implementation source was
// consulted.
//
// Usage:
//
//	go run ./examples/rtp_packetize -ptime=10 < input.g729
//	go run ./examples/rtp_packetize -ptime=20 < input.g729
//
// Output is one hex-dump line per RTP packet payload (10 or 20
// bytes), prefixed with the packet index and the RTP timestamp
// increment that the caller's RTP framer would apply (8000 Hz clock
// × frame duration in seconds).
package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/exedev/g729"
)

func main() {
	ptime := flag.Int("ptime", 10, "RTP packetization time in milliseconds (10 or 20)")
	flag.Parse()

	if *ptime != 10 && *ptime != 20 {
		fmt.Fprintln(os.Stderr, "rtp_packetize: -ptime must be 10 or 20")
		os.Exit(1)
	}

	if err := run(os.Stdin, os.Stdout, *ptime); err != nil {
		fmt.Fprintln(os.Stderr, "rtp_packetize:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer, ptime int) error {
	framesPerPacket := ptime / 10
	payloadBytes := framesPerPacket * g729.FrameBytes
	timestampPerPacket := uint32(8000 * ptime / 1000)

	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	defer w.Flush()

	payload := make([]byte, payloadBytes)
	frame := make([]byte, g729.FrameBytes)

	var packetIdx uint32
	var rtpTimestamp uint32 // illustrative; caller would seed and wrap per RFC 3550

	for {
		for f := 0; f < framesPerPacket; f++ {
			_, err := io.ReadFull(r, frame)
			if errors.Is(err, io.EOF) {
				if f == 0 {
					return nil
				}
				return fmt.Errorf("trailing partial RTP packet: %d of %d frames consumed", f, framesPerPacket)
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("trailing partial G.729 frame at packet %d frame %d", packetIdx, f)
			}
			if err != nil {
				return err
			}
			copy(payload[f*g729.FrameBytes:], frame)
		}

		fmt.Fprintf(w, "packet=%d ts=%d payload=%s\n", packetIdx, rtpTimestamp, hex.EncodeToString(payload))

		packetIdx++
		rtpTimestamp += timestampPerPacket
	}
}
