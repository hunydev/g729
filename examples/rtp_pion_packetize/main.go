// Example: rtp_pion_packetize
//
// Practical RTP packet construction for G.729 payload type 18 using
// github.com/pion/rtp. Reads packed 10-byte G.729 frames from stdin,
// bundles them as ptime=10 or ptime=20 RTP payloads, marshals full RTP
// packets, and writes one hex-dump line per packet.
//
// This example is intentionally outside the codec runtime. Production
// SIP/MRCP/RTP applications may use Pion or their own RTP stack for
// UDP sockets, sequence numbers, timestamps, SSRC selection, jitter
// buffers, RTCP, and network I/O.
//
// Clean-room I1 declaration: this example uses only the public API of
// github.com/hunydev/g729, the Go standard library, and the generic
// github.com/pion/rtp packet library. No ITU reference C, bcg729,
// FFmpeg, Sipro, or other G.729 implementation source was consulted.
//
// Usage:
//
//	go run ./examples/rtp_pion_packetize -ptime=10 < input.g729
//	go run ./examples/rtp_pion_packetize -ptime=20 -seq=1000 -ts=3200 -ssrc=0x11223344 < input.g729
//
// Output is one full RTP packet per line, including the RTP header,
// encoded as hex. The RTP payload remains one 10-byte G.729 frame for
// ptime=10 or two concatenated 10-byte frames for ptime=20.
package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hunydev/g729"
	"github.com/pion/rtp"
)

const rtpClockHz = 8000

func main() {
	ptime := flag.Int("ptime", 20, "RTP packetization time in milliseconds (10 or 20)")
	payloadType := flag.Uint64("pt", 18, "RTP payload type (default static G.729 payload type 18)")
	sequence := flag.Uint64("seq", 0, "initial RTP sequence number")
	timestamp := flag.Uint64("ts", 0, "initial RTP timestamp")
	ssrc := flag.Uint64("ssrc", 0x11223344, "RTP SSRC")
	markerFirst := flag.Bool("marker-first", true, "set the RTP marker bit on the first packet")
	flag.Parse()

	if err := run(os.Stdin, os.Stdout, options{
		ptime:       *ptime,
		payloadType: *payloadType,
		sequence:    *sequence,
		timestamp:   *timestamp,
		ssrc:        *ssrc,
		markerFirst: *markerFirst,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "rtp_pion_packetize:", err)
		os.Exit(1)
	}
}

type options struct {
	ptime       int
	payloadType uint64
	sequence    uint64
	timestamp   uint64
	ssrc        uint64
	markerFirst bool
}

func run(in io.Reader, out io.Writer, opt options) error {
	if opt.ptime != 10 && opt.ptime != 20 {
		return fmt.Errorf("-ptime must be 10 or 20")
	}
	if opt.payloadType > 127 {
		return fmt.Errorf("-pt must fit in 7 bits")
	}
	if opt.sequence > 0xffff {
		return fmt.Errorf("-seq must fit in 16 bits")
	}
	if opt.timestamp > 0xffffffff {
		return fmt.Errorf("-ts must fit in 32 bits")
	}
	if opt.ssrc > 0xffffffff {
		return fmt.Errorf("-ssrc must fit in 32 bits")
	}

	framesPerPacket := opt.ptime / 10
	payloadBytes := framesPerPacket * g729.FrameBytes
	timestampStep := uint32(rtpClockHz * opt.ptime / 1000)

	reader := bufio.NewReader(in)
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	payload := make([]byte, payloadBytes)
	frame := make([]byte, g729.FrameBytes)

	sequence := uint16(opt.sequence)
	timestamp := uint32(opt.timestamp)
	ssrc := uint32(opt.ssrc)
	payloadType := uint8(opt.payloadType)
	var packetIdx uint32

	for {
		for f := 0; f < framesPerPacket; f++ {
			_, err := io.ReadFull(reader, frame)
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

		packet := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         opt.markerFirst && packetIdx == 0,
				PayloadType:    payloadType,
				SequenceNumber: sequence,
				Timestamp:      timestamp,
				SSRC:           ssrc,
			},
			Payload: payload,
		}
		data, err := packet.Marshal()
		if err != nil {
			return err
		}

		fmt.Fprintf(writer, "packet=%d seq=%d ts=%d ssrc=0x%08x pt=%d marker=%t payload_bytes=%d rtp=%s\n",
			packetIdx, sequence, timestamp, ssrc, payloadType, packet.Marker, len(payload), hex.EncodeToString(data))

		packetIdx++
		sequence++
		timestamp += timestampStep
	}
}
