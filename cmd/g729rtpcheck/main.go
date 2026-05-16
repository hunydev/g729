package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	g729 "github.com/hunydev/g729"
)

const (
	rtpClockHz       = 8000
	rtpPayloadType18 = 18
)

var stdout io.Writer = os.Stdout

type options struct {
	mode        string
	path        string
	ptime       int
	payloadType int
	decode      bool
	strictTS    bool
	jsonOutput  bool
}

type report struct {
	Mode             string `json:"mode"`
	Packets          int    `json:"packets"`
	Frames           int    `json:"frames"`
	PayloadBytes     int    `json:"payloadBytes"`
	DecodedFrames    int    `json:"decodedFrames"`
	Skipped          int    `json:"skipped"`
	MarkerPackets    int    `json:"markerPackets"`
	PaddingPackets   int    `json:"paddingPackets"`
	ExtensionPackets int    `json:"extensionPackets"`
	CSRCEntries      int    `json:"csrcEntries"`
	Streams          int    `json:"streams"`
}

type rtpPacket struct {
	PayloadType int
	Marker      bool
	Padding     bool
	Extension   bool
	CSRCCount   int
	Sequence    uint16
	Timestamp   uint32
	SSRC        uint32
	Payload     []byte
}

type streamState struct {
	seen          bool
	sequence      uint16
	nextTimestamp uint32
}

func main() {
	var opt options
	flag.StringVar(&opt.mode, "mode", "payload", "input mode: payload or pcap")
	flag.StringVar(&opt.path, "in", "", "input file (default stdin)")
	flag.IntVar(&opt.ptime, "ptime", 10, "expected packetization time in ms: 10, 20, or 0 in pcap mode")
	flag.IntVar(&opt.payloadType, "pt", rtpPayloadType18, "RTP payload type to inspect in pcap mode")
	flag.BoolVar(&opt.decode, "decode", false, "decode each 10-byte G.729 frame to exercise the codec API")
	flag.BoolVar(&opt.strictTS, "strict-ts", false, "require RTP sequence + timestamp continuity per SSRC")
	flag.BoolVar(&opt.jsonOutput, "json", false, "print a machine-readable JSON report")
	flag.Parse()

	in := io.Reader(os.Stdin)
	if opt.path != "" {
		f, err := os.Open(opt.path)
		if err != nil {
			exitErr(err)
		}
		defer f.Close()
		in = f
	}

	rep, err := run(in, opt)
	if err != nil {
		exitErr(err)
	}
	printReport(rep, opt.jsonOutput)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "g729rtpcheck:", err)
	os.Exit(1)
}

func run(r io.Reader, opt options) (report, error) {
	switch opt.mode {
	case "payload":
		return analyzePayloadStream(r, opt)
	case "pcap":
		return analyzePCAP(r, opt)
	default:
		return report{}, fmt.Errorf("unknown -mode %q", opt.mode)
	}
}

func analyzePayloadStream(r io.Reader, opt options) (report, error) {
	framesPerPacket, err := packetFrameCount(opt.ptime)
	if err != nil {
		return report{}, err
	}
	packetBytes := framesPerPacket * g729.FrameBytes
	data, err := io.ReadAll(r)
	if err != nil {
		return report{}, err
	}
	if len(data) == 2 {
		return report{}, fmt.Errorf("packet 0: 2-byte Annex B SID/CNG RTP payload: %w", g729.ErrUnsupportedAnnexB)
	}
	if len(data)%packetBytes != 0 {
		return report{}, fmt.Errorf("payload stream length %d is not a multiple of %d bytes for ptime=%d",
			len(data), packetBytes, opt.ptime)
	}

	rep := report{Mode: "payload"}
	var dec g729.Decoder
	var pcm [g729.FrameSamples]int16
	for off := 0; off < len(data); off += packetBytes {
		payload := data[off : off+packetBytes]
		frames, err := validatePayload(payload, framesPerPacket)
		if err != nil {
			return rep, fmt.Errorf("packet %d: %w", rep.Packets, err)
		}
		rep.Packets++
		rep.Frames += frames
		rep.PayloadBytes += len(payload)
		if opt.decode {
			for f := 0; f < frames; f++ {
				frame := payload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
				if err := dec.DecodeFrame(frame, pcm[:]); err != nil {
					return rep, fmt.Errorf("packet %d frame %d decode: %w", rep.Packets-1, f, err)
				}
				rep.DecodedFrames++
			}
		}
	}
	if rep.Packets > 0 {
		rep.Streams = 1
	}
	return rep, nil
}

func analyzePCAP(r io.Reader, opt options) (report, error) {
	framesPerPacket := 0
	if opt.ptime != 0 {
		var err error
		framesPerPacket, err = packetFrameCount(opt.ptime)
		if err != nil {
			return report{}, err
		}
	}

	pcap, err := readPCAP(r)
	if err != nil {
		return report{}, err
	}
	rep := report{Mode: "pcap"}
	var dec g729.Decoder
	var pcm [g729.FrameSamples]int16
	streams := map[uint32]streamState{}
	seenStreams := map[uint32]struct{}{}

	for i, frame := range pcap.frames {
		rtp, ok, err := parseEthernetIPv4UDPRTP(frame)
		if err != nil {
			rep.Skipped++
			continue
		}
		if !ok {
			rep.Skipped++
			continue
		}
		if rtp.PayloadType != opt.payloadType {
			rep.Skipped++
			continue
		}
		seenStreams[rtp.SSRC] = struct{}{}
		if rtp.Marker {
			rep.MarkerPackets++
		}
		if rtp.Padding {
			rep.PaddingPackets++
		}
		if rtp.Extension {
			rep.ExtensionPackets++
		}
		rep.CSRCEntries += rtp.CSRCCount

		frames, err := validatePayload(rtp.Payload, framesPerPacket)
		if err != nil {
			return rep, fmt.Errorf("pcap packet %d seq=%d ts=%d ssrc=%08x: %w",
				i, rtp.Sequence, rtp.Timestamp, rtp.SSRC, err)
		}
		if opt.strictTS {
			state := streams[rtp.SSRC]
			if state.seen {
				wantSeq := state.sequence + 1
				if rtp.Sequence != wantSeq {
					return rep, fmt.Errorf("pcap packet %d ssrc=%08x: seq=%d, want %d",
						i, rtp.SSRC, rtp.Sequence, wantSeq)
				}
				if rtp.Timestamp != state.nextTimestamp {
					return rep, fmt.Errorf("pcap packet %d ssrc=%08x: timestamp=%d, want %d",
						i, rtp.SSRC, rtp.Timestamp, state.nextTimestamp)
				}
			}
			streams[rtp.SSRC] = streamState{
				seen:          true,
				sequence:      rtp.Sequence,
				nextTimestamp: rtp.Timestamp + uint32(frames*g729.FrameSamples),
			}
		}

		rep.Packets++
		rep.Frames += frames
		rep.PayloadBytes += len(rtp.Payload)
		if opt.decode {
			for f := 0; f < frames; f++ {
				frame := rtp.Payload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
				if err := dec.DecodeFrame(frame, pcm[:]); err != nil {
					return rep, fmt.Errorf("pcap packet %d frame %d decode: %w", i, f, err)
				}
				rep.DecodedFrames++
			}
		}
	}
	if rep.Packets == 0 {
		return rep, fmt.Errorf("no RTP packets with payload type %d found", opt.payloadType)
	}
	rep.Streams = len(seenStreams)
	_ = rtpClockHz
	return rep, nil
}

func printReport(rep report, jsonOutput bool) {
	if jsonOutput {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			exitErr(err)
		}
		fmt.Fprintln(stdout, string(data))
		return
	}
	fmt.Fprintf(stdout, "mode=%s packets=%d frames=%d payload_bytes=%d decoded_frames=%d skipped=%d marker_packets=%d padding_packets=%d extension_packets=%d csrc_entries=%d streams=%d\n",
		rep.Mode, rep.Packets, rep.Frames, rep.PayloadBytes, rep.DecodedFrames, rep.Skipped,
		rep.MarkerPackets, rep.PaddingPackets, rep.ExtensionPackets, rep.CSRCEntries, rep.Streams)
}

func packetFrameCount(ptime int) (int, error) {
	switch ptime {
	case 10:
		return 1, nil
	case 20:
		return 2, nil
	default:
		return 0, fmt.Errorf("ptime must be 10 or 20, got %d", ptime)
	}
}

func validatePayload(payload []byte, framesPerPacket int) (int, error) {
	if len(payload) == 0 {
		return 0, errors.New("empty G.729 RTP payload")
	}
	if len(payload) == 2 {
		return 0, fmt.Errorf("2-byte Annex B SID/CNG RTP payload: %w", g729.ErrUnsupportedAnnexB)
	}
	if len(payload)%g729.FrameBytes != 0 {
		return 0, fmt.Errorf("payload length %d is not a multiple of %d-byte G.729 frames (Annex B SID/CNG is not supported)",
			len(payload), g729.FrameBytes)
	}
	frames := len(payload) / g729.FrameBytes
	if framesPerPacket != 0 && frames != framesPerPacket {
		return 0, fmt.Errorf("payload contains %d frame(s), want %d for configured ptime",
			frames, framesPerPacket)
	}
	return frames, nil
}

type pcapData struct {
	linkType uint32
	frames   [][]byte
}

func readPCAP(r io.Reader) (pcapData, error) {
	var hdr [24]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return pcapData{}, err
	}

	var order binary.ByteOrder
	switch binary.BigEndian.Uint32(hdr[0:4]) {
	case 0xa1b2c3d4, 0xa1b23c4d:
		order = binary.BigEndian
	case 0xd4c3b2a1, 0x4d3cb2a1:
		order = binary.LittleEndian
	default:
		return pcapData{}, fmt.Errorf("unsupported pcap magic %08x", binary.BigEndian.Uint32(hdr[0:4]))
	}
	linkType := order.Uint32(hdr[20:24])
	if linkType != 1 {
		return pcapData{}, fmt.Errorf("unsupported pcap link type %d (only Ethernet is supported)", linkType)
	}

	out := pcapData{linkType: linkType}
	for {
		var rec [16]byte
		_, err := io.ReadFull(r, rec[:])
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return out, err
			}
			return out, nil
		}
		if err != nil {
			return out, err
		}
		inclLen := order.Uint32(rec[8:12])
		if inclLen > 1<<20 {
			return out, fmt.Errorf("pcap record too large: %d bytes", inclLen)
		}
		buf := make([]byte, inclLen)
		if _, err := io.ReadFull(r, buf); err != nil {
			return out, err
		}
		out.frames = append(out.frames, buf)
	}
}

func parseEthernetIPv4UDPRTP(frame []byte) (rtpPacket, bool, error) {
	if len(frame) < 14 {
		return rtpPacket{}, false, nil
	}
	etherType := binary.BigEndian.Uint16(frame[12:14])
	ipOff := 14
	if etherType == 0x8100 || etherType == 0x88a8 {
		if len(frame) < 18 {
			return rtpPacket{}, false, nil
		}
		etherType = binary.BigEndian.Uint16(frame[16:18])
		ipOff = 18
	}
	if etherType != 0x0800 {
		return rtpPacket{}, false, nil
	}
	if len(frame) < ipOff+20 {
		return rtpPacket{}, false, nil
	}
	ip := frame[ipOff:]
	if ip[0]>>4 != 4 {
		return rtpPacket{}, false, nil
	}
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || len(ip) < ihl {
		return rtpPacket{}, false, fmt.Errorf("invalid IPv4 header length %d", ihl)
	}
	totalLen := int(binary.BigEndian.Uint16(ip[2:4]))
	if totalLen < ihl || totalLen > len(ip) {
		return rtpPacket{}, false, fmt.Errorf("invalid IPv4 total length %d", totalLen)
	}
	if ip[9] != 17 {
		return rtpPacket{}, false, nil
	}
	udp := ip[ihl:totalLen]
	if len(udp) < 8 {
		return rtpPacket{}, false, nil
	}
	udpLen := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpLen < 8 || udpLen > len(udp) {
		return rtpPacket{}, false, fmt.Errorf("invalid UDP length %d", udpLen)
	}
	payload := udp[8:udpLen]
	pkt, err := parseRTP(payload)
	if err != nil {
		return rtpPacket{}, false, err
	}
	return pkt, true, nil
}

func parseRTP(data []byte) (rtpPacket, error) {
	if len(data) < 12 {
		return rtpPacket{}, errors.New("short RTP header")
	}
	if data[0]>>6 != 2 {
		return rtpPacket{}, errors.New("RTP version is not 2")
	}
	cc := int(data[0] & 0x0f)
	padding := data[0]&0x20 != 0
	x := data[0]&0x10 != 0
	off := 12 + cc*4
	if len(data) < off {
		return rtpPacket{}, errors.New("short RTP CSRC list")
	}
	if x {
		if len(data) < off+4 {
			return rtpPacket{}, errors.New("short RTP extension header")
		}
		extLenWords := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		off += 4 + extLenWords*4
		if len(data) < off {
			return rtpPacket{}, errors.New("short RTP extension payload")
		}
	}
	end := len(data)
	if padding {
		padLen := int(data[len(data)-1])
		if padLen == 0 || padLen > len(data)-off {
			return rtpPacket{}, errors.New("invalid RTP padding length")
		}
		end -= padLen
	}
	return rtpPacket{
		PayloadType: int(data[1] & 0x7f),
		Marker:      data[1]&0x80 != 0,
		Padding:     padding,
		Extension:   x,
		CSRCCount:   cc,
		Sequence:    binary.BigEndian.Uint16(data[2:4]),
		Timestamp:   binary.BigEndian.Uint32(data[4:8]),
		SSRC:        binary.BigEndian.Uint32(data[8:12]),
		Payload:     data[off:end],
	}, nil
}
