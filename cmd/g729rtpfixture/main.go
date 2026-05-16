// Command g729rtpfixture generates small Ethernet/IPv4/UDP/RTP pcap fixtures
// for cmd/g729rtpcheck and integration smoke tests.
//
// Clean-room I1 declaration: this command uses only the public API of
// github.com/hunydev/g729, the Go standard library, and the generic
// github.com/pion/rtp packet library. No ITU reference C, bcg729, FFmpeg,
// Sipro, or other G.729 implementation source was consulted.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"

	"github.com/hunydev/g729"
	"github.com/pion/rtp"
)

const (
	rtpClockHz       = 8000
	rtpPayloadType18 = 18
	defaultSSRC      = 0x11223344
)

func main() {
	var opt options
	flag.IntVar(&opt.ptime, "ptime", 20, "RTP packetization time in milliseconds (10 or 20)")
	flag.IntVar(&opt.packets, "packets", 3, "number of matching RTP packets to generate")
	flag.Uint64Var(&opt.payloadType, "pt", rtpPayloadType18, "RTP payload type for matching packets")
	flag.Uint64Var(&opt.wrongPayloadType, "wrong-pt-value", 96, "payload type used by -wrong-pt")
	flag.BoolVar(&opt.includeWrongPT, "wrong-pt", false, "append one RTP packet with a non-matching payload type")
	flag.BoolVar(&opt.multiSSRC, "multi-ssrc", false, "alternate matching packets across two SSRCs")
	flag.BoolVar(&opt.sid, "sid", false, "emit one 2-byte Annex B SID/CNG-like payload instead of speech packets")
	flag.Uint64Var(&opt.sequence, "seq", 0, "initial RTP sequence number")
	flag.Uint64Var(&opt.timestamp, "ts", 0, "initial RTP timestamp")
	flag.Uint64Var(&opt.ssrc, "ssrc", defaultSSRC, "primary RTP SSRC")
	flag.StringVar(&opt.outPath, "out", "", "output pcap path (default stdout)")
	flag.Parse()

	out := io.Writer(os.Stdout)
	if opt.outPath != "" {
		f, err := os.Create(opt.outPath)
		if err != nil {
			exitErr(err)
		}
		defer f.Close()
		out = f
	}

	if err := run(out, opt); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "g729rtpfixture:", err)
	os.Exit(1)
}

type options struct {
	ptime            int
	packets          int
	payloadType      uint64
	wrongPayloadType uint64
	includeWrongPT   bool
	multiSSRC        bool
	sid              bool
	sequence         uint64
	timestamp        uint64
	ssrc             uint64
	outPath          string
}

func run(out io.Writer, opt options) error {
	if err := validateOptions(opt); err != nil {
		return err
	}

	if err := writePCAPHeader(out); err != nil {
		return err
	}

	builder := fixtureBuilder{
		ptime:            opt.ptime,
		payloadType:      uint8(opt.payloadType),
		wrongPayloadType: uint8(opt.wrongPayloadType),
		initialSequence:  uint16(opt.sequence),
		initialTimestamp: uint32(opt.timestamp),
		ssrc:             uint32(opt.ssrc),
		states:           map[uint32]*rtpState{},
	}

	if opt.sid {
		frame, err := builder.buildRTPPacket(builder.payloadType, builder.ssrc, []byte{0x00, 0x00}, true)
		if err != nil {
			return err
		}
		return writePCAPRecord(out, 0, wrapEthernetIPv4UDPRTP(frame))
	}

	for i := 0; i < opt.packets; i++ {
		ssrc := builder.ssrc
		if opt.multiSSRC && i%2 == 1 {
			ssrc++
		}
		payload := speechPayload(opt.ptime, i)
		frame, err := builder.buildRTPPacket(builder.payloadType, ssrc, payload, i == 0)
		if err != nil {
			return err
		}
		if err := writePCAPRecord(out, uint32(i), wrapEthernetIPv4UDPRTP(frame)); err != nil {
			return err
		}
	}

	if opt.includeWrongPT {
		payload := speechPayload(opt.ptime, opt.packets)
		frame, err := builder.buildRTPPacket(builder.wrongPayloadType, builder.ssrc, payload, false)
		if err != nil {
			return err
		}
		if err := writePCAPRecord(out, uint32(opt.packets), wrapEthernetIPv4UDPRTP(frame)); err != nil {
			return err
		}
	}

	return nil
}

func validateOptions(opt options) error {
	if opt.ptime != 10 && opt.ptime != 20 {
		return fmt.Errorf("-ptime must be 10 or 20")
	}
	if opt.packets < 1 {
		return fmt.Errorf("-packets must be >= 1")
	}
	if opt.payloadType > 127 || opt.wrongPayloadType > 127 {
		return fmt.Errorf("RTP payload types must fit in 7 bits")
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
	return nil
}

type fixtureBuilder struct {
	ptime            int
	payloadType      uint8
	wrongPayloadType uint8
	initialSequence  uint16
	initialTimestamp uint32
	ssrc             uint32
	states           map[uint32]*rtpState
}

type rtpState struct {
	sequence  uint16
	timestamp uint32
}

func (b *fixtureBuilder) buildRTPPacket(payloadType uint8, ssrc uint32, payload []byte, marker bool) ([]byte, error) {
	state := b.stateForSSRC(ssrc)
	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         marker,
			PayloadType:    payloadType,
			SequenceNumber: state.sequence,
			Timestamp:      state.timestamp,
			SSRC:           ssrc,
		},
		Payload: payload,
	}
	data, err := packet.Marshal()
	if err != nil {
		return nil, err
	}
	state.sequence++
	state.timestamp += uint32(rtpClockHz * b.ptime / 1000)
	return data, nil
}

func (b *fixtureBuilder) stateForSSRC(ssrc uint32) *rtpState {
	if state := b.states[ssrc]; state != nil {
		return state
	}
	state := &rtpState{sequence: b.initialSequence, timestamp: b.initialTimestamp}
	b.states[ssrc] = state
	return state
}

func speechPayload(ptime int, packetIndex int) []byte {
	framesPerPacket := ptime / 10
	payload := make([]byte, framesPerPacket*g729.FrameBytes)
	for i := range payload {
		payload[i] = byte((packetIndex + i) & 0xff)
	}
	return payload
}

func writePCAPHeader(w io.Writer) error {
	var hdr [24]byte
	binary.LittleEndian.PutUint32(hdr[0:4], 0xa1b2c3d4)
	binary.LittleEndian.PutUint16(hdr[4:6], 2)
	binary.LittleEndian.PutUint16(hdr[6:8], 4)
	binary.LittleEndian.PutUint32(hdr[16:20], 65535)
	binary.LittleEndian.PutUint32(hdr[20:24], 1) // Ethernet
	_, err := w.Write(hdr[:])
	return err
}

func writePCAPRecord(w io.Writer, index uint32, frame []byte) error {
	var rec [16]byte
	binary.LittleEndian.PutUint32(rec[0:4], index)
	binary.LittleEndian.PutUint32(rec[8:12], uint32(len(frame)))
	binary.LittleEndian.PutUint32(rec[12:16], uint32(len(frame)))
	if _, err := w.Write(rec[:]); err != nil {
		return err
	}
	_, err := w.Write(frame)
	return err
}

func wrapEthernetIPv4UDPRTP(rtpBytes []byte) []byte {
	udpLen := 8 + len(rtpBytes)
	ipLen := 20 + udpLen

	frame := make([]byte, 0, 14+ipLen)
	eth := make([]byte, 14)
	eth[0], eth[1], eth[2], eth[3], eth[4], eth[5] = 0x02, 0x00, 0x00, 0x00, 0x00, 0x02
	eth[6], eth[7], eth[8], eth[9], eth[10], eth[11] = 0x02, 0x00, 0x00, 0x00, 0x00, 0x01
	eth[12], eth[13] = 0x08, 0x00
	frame = append(frame, eth...)

	ip := make([]byte, 20)
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen))
	ip[8] = 64
	ip[9] = 17
	putIPv4(ip[12:16], mustIPv4("192.0.2.10"))
	putIPv4(ip[16:20], mustIPv4("192.0.2.20"))
	frame = append(frame, ip...)

	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[0:2], 4000)
	binary.BigEndian.PutUint16(udp[2:4], 5004)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	frame = append(frame, udp...)
	frame = append(frame, rtpBytes...)
	return frame
}

func mustIPv4(value string) netip.Addr {
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.Is4() {
		panic("invalid fixture IPv4 address: " + value)
	}
	return addr
}

func putIPv4(dst []byte, addr netip.Addr) {
	copy(dst, addr.AsSlice())
}
