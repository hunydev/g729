// Package rtpcheck analyzes G.729 RTP payload streams and Ethernet/IPv4/UDP/RTP
// pcap captures.
//
// Clean-room I1 declaration: this package uses only this repository's public
// codec API and RTP packet syntax. It does not inspect or depend on any
// third-party G.729 implementation source.
package rtpcheck

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	g729 "github.com/hunydev/g729"
)

const (
	// RTPClockHz is the G.729 RTP timestamp clock.
	RTPClockHz = 8000
	// PayloadType18 is the static RTP payload type assigned to G729/8000.
	PayloadType18 = 18
	// NearClipThreshold is the decoded PCM absolute-value threshold counted as
	// near clipping in integration reports.
	NearClipThreshold = 32760
)

// Options controls payload or pcap analysis.
type Options struct {
	Mode        string
	Ptime       int
	PayloadType int
	Decode      bool
	StrictTS    bool
}

// Report is the machine-readable analysis result.
type Report struct {
	Mode             string         `json:"mode"`
	Packets          int            `json:"packets"`
	Frames           int            `json:"frames"`
	PayloadBytes     int            `json:"payloadBytes"`
	DecodedFrames    int            `json:"decodedFrames"`
	Skipped          int            `json:"skipped"`
	MarkerPackets    int            `json:"markerPackets"`
	PaddingPackets   int            `json:"paddingPackets"`
	ExtensionPackets int            `json:"extensionPackets"`
	CSRCEntries      int            `json:"csrcEntries"`
	Streams          int            `json:"streams"`
	Decode           *DecodeMetrics `json:"decode,omitempty"`
}

// DecodeMetrics summarizes decoded PCM when Options.Decode is enabled.
type DecodeMetrics struct {
	Samples           int     `json:"samples"`
	DurationSeconds   float64 `json:"durationSeconds"`
	RMS               float64 `json:"rms"`
	DC                float64 `json:"dc"`
	Peak              int     `json:"peak"`
	NearClip          int     `json:"nearClip"`
	HardClip          int     `json:"hardClip"`
	ZeroCrossings     int     `json:"zeroCrossings"`
	NearClipThreshold int     `json:"nearClipThreshold"`
}

// RTPPacket is the subset of RTP fields needed by the checker.
type RTPPacket struct {
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

// Run analyzes r as either a raw payload stream or an Ethernet pcap.
func Run(r io.Reader, opt Options) (Report, error) {
	switch opt.Mode {
	case "payload":
		return AnalyzePayloadStream(r, opt)
	case "pcap":
		return AnalyzePCAP(r, opt)
	default:
		return Report{}, fmt.Errorf("unknown mode %q", opt.Mode)
	}
}

// AnalyzePayloadStream validates a raw G.729 payload byte stream.
func AnalyzePayloadStream(r io.Reader, opt Options) (Report, error) {
	framesPerPacket, err := PacketFrameCount(opt.Ptime)
	if err != nil {
		return Report{}, err
	}
	packetBytes := framesPerPacket * g729.FrameBytes
	data, err := io.ReadAll(r)
	if err != nil {
		return Report{}, err
	}
	if len(data) == 2 {
		return Report{}, fmt.Errorf("packet 0: 2-byte Annex B SID/CNG RTP payload: %w", g729.ErrUnsupportedAnnexB)
	}
	if len(data)%packetBytes != 0 {
		return Report{}, fmt.Errorf("payload stream length %d is not a multiple of %d bytes for ptime=%d",
			len(data), packetBytes, opt.Ptime)
	}

	rep := Report{Mode: "payload"}
	dec := g729.NewDecoder()
	var pcm [g729.FrameSamples]int16
	var stats decodeStats
	for off := 0; off < len(data); off += packetBytes {
		payload := data[off : off+packetBytes]
		frames, err := ValidatePayload(payload, framesPerPacket)
		if err != nil {
			return rep, fmt.Errorf("packet %d: %w", rep.Packets, err)
		}
		rep.Packets++
		rep.Frames += frames
		rep.PayloadBytes += len(payload)
		if opt.Decode {
			for f := 0; f < frames; f++ {
				frame := payload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
				if err := dec.DecodeFrame(frame, pcm[:]); err != nil {
					return rep, fmt.Errorf("packet %d frame %d decode: %w", rep.Packets-1, f, err)
				}
				stats.Add(pcm[:])
				rep.DecodedFrames++
			}
		}
	}
	if rep.Packets > 0 {
		rep.Streams = 1
	}
	if opt.Decode {
		metrics := stats.Metrics()
		rep.Decode = &metrics
	}
	return rep, nil
}

// AnalyzePCAP validates RTP packets in an Ethernet/IPv4/UDP/RTP pcap.
func AnalyzePCAP(r io.Reader, opt Options) (Report, error) {
	framesPerPacket := 0
	if opt.Ptime != 0 {
		var err error
		framesPerPacket, err = PacketFrameCount(opt.Ptime)
		if err != nil {
			return Report{}, err
		}
	}

	pcap, err := readPCAP(r)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Mode: "pcap"}
	streams := map[uint32]streamState{}
	seenStreams := map[uint32]struct{}{}
	decoders := map[uint32]*g729.Decoder{}
	var pcm [g729.FrameSamples]int16
	var stats decodeStats

	for i, frame := range pcap.frames {
		rtp, ok, err := ParseEthernetIPv4UDPRTP(frame)
		if err != nil {
			rep.Skipped++
			continue
		}
		if !ok {
			rep.Skipped++
			continue
		}
		if rtp.PayloadType != opt.PayloadType {
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

		frames, err := ValidatePayload(rtp.Payload, framesPerPacket)
		if err != nil {
			return rep, fmt.Errorf("pcap packet %d seq=%d ts=%d ssrc=%08x: %w",
				i, rtp.Sequence, rtp.Timestamp, rtp.SSRC, err)
		}
		if opt.StrictTS {
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
		if opt.Decode {
			dec := decoders[rtp.SSRC]
			if dec == nil {
				dec = g729.NewDecoder()
				decoders[rtp.SSRC] = dec
			}
			for f := 0; f < frames; f++ {
				frame := rtp.Payload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
				if err := dec.DecodeFrame(frame, pcm[:]); err != nil {
					return rep, fmt.Errorf("pcap packet %d frame %d decode: %w", i, f, err)
				}
				stats.Add(pcm[:])
				rep.DecodedFrames++
			}
		}
	}
	if rep.Packets == 0 {
		return rep, fmt.Errorf("no RTP packets with payload type %d found", opt.PayloadType)
	}
	rep.Streams = len(seenStreams)
	if opt.Decode {
		metrics := stats.Metrics()
		rep.Decode = &metrics
	}
	return rep, nil
}

// PacketFrameCount maps supported ptime values to frames per RTP payload.
func PacketFrameCount(ptime int) (int, error) {
	switch ptime {
	case 10:
		return 1, nil
	case 20:
		return 2, nil
	default:
		return 0, fmt.Errorf("ptime must be 10 or 20, got %d", ptime)
	}
}

// ValidatePayload validates one RTP payload and returns its G.729 speech frame count.
func ValidatePayload(payload []byte, framesPerPacket int) (int, error) {
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

// ParseEthernetIPv4UDPRTP extracts one RTP packet from an Ethernet frame.
func ParseEthernetIPv4UDPRTP(frame []byte) (RTPPacket, bool, error) {
	if len(frame) < 14 {
		return RTPPacket{}, false, nil
	}
	etherType := binary.BigEndian.Uint16(frame[12:14])
	ipOff := 14
	if etherType == 0x8100 || etherType == 0x88a8 {
		if len(frame) < 18 {
			return RTPPacket{}, false, nil
		}
		etherType = binary.BigEndian.Uint16(frame[16:18])
		ipOff = 18
	}
	if etherType != 0x0800 {
		return RTPPacket{}, false, nil
	}
	if len(frame) < ipOff+20 {
		return RTPPacket{}, false, nil
	}
	ip := frame[ipOff:]
	if ip[0]>>4 != 4 {
		return RTPPacket{}, false, nil
	}
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || len(ip) < ihl {
		return RTPPacket{}, false, fmt.Errorf("invalid IPv4 header length %d", ihl)
	}
	totalLen := int(binary.BigEndian.Uint16(ip[2:4]))
	if totalLen < ihl || totalLen > len(ip) {
		return RTPPacket{}, false, fmt.Errorf("invalid IPv4 total length %d", totalLen)
	}
	if ip[9] != 17 {
		return RTPPacket{}, false, nil
	}
	udp := ip[ihl:totalLen]
	if len(udp) < 8 {
		return RTPPacket{}, false, nil
	}
	udpLen := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpLen < 8 || udpLen > len(udp) {
		return RTPPacket{}, false, fmt.Errorf("invalid UDP length %d", udpLen)
	}
	payload := udp[8:udpLen]
	pkt, err := ParseRTP(payload)
	if err != nil {
		return RTPPacket{}, false, err
	}
	return pkt, true, nil
}

// ParseRTP parses one RTP packet.
func ParseRTP(data []byte) (RTPPacket, error) {
	if len(data) < 12 {
		return RTPPacket{}, errors.New("short RTP header")
	}
	if data[0]>>6 != 2 {
		return RTPPacket{}, errors.New("RTP version is not 2")
	}
	cc := int(data[0] & 0x0f)
	padding := data[0]&0x20 != 0
	x := data[0]&0x10 != 0
	off := 12 + cc*4
	if len(data) < off {
		return RTPPacket{}, errors.New("short RTP CSRC list")
	}
	if x {
		if len(data) < off+4 {
			return RTPPacket{}, errors.New("short RTP extension header")
		}
		extLenWords := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		off += 4 + extLenWords*4
		if len(data) < off {
			return RTPPacket{}, errors.New("short RTP extension payload")
		}
	}
	end := len(data)
	if padding {
		padLen := int(data[len(data)-1])
		if padLen == 0 || padLen > len(data)-off {
			return RTPPacket{}, errors.New("invalid RTP padding length")
		}
		end -= padLen
	}
	return RTPPacket{
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

type decodeStats struct {
	samples      int
	sum          int64
	sumSquares   float64
	peak         int
	nearClip     int
	hardClip     int
	zeroCrossing int
	prevSign     int
}

func (s *decodeStats) Add(pcm []int16) {
	for _, sample := range pcm {
		v := int(sample)
		abs := absInt16(v)
		s.samples++
		s.sum += int64(v)
		s.sumSquares += float64(v) * float64(v)
		if abs > s.peak {
			s.peak = abs
		}
		if abs >= NearClipThreshold {
			s.nearClip++
		}
		if v == 32767 || v == -32768 {
			s.hardClip++
		}
		sign := signInt(v)
		if sign != 0 {
			if s.prevSign != 0 && sign != s.prevSign {
				s.zeroCrossing++
			}
			s.prevSign = sign
		}
	}
}

func (s decodeStats) Metrics() DecodeMetrics {
	out := DecodeMetrics{NearClipThreshold: NearClipThreshold}
	if s.samples == 0 {
		return out
	}
	out.Samples = s.samples
	out.DurationSeconds = float64(s.samples) / RTPClockHz
	out.RMS = math.Sqrt(s.sumSquares / float64(s.samples))
	out.DC = float64(s.sum) / float64(s.samples)
	out.Peak = s.peak
	out.NearClip = s.nearClip
	out.HardClip = s.hardClip
	out.ZeroCrossings = s.zeroCrossing
	return out
}

func absInt16(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func signInt(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}
