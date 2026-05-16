package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	g729 "github.com/hunydev/g729"
)

func TestAnalyzePayloadStreamPtime20(t *testing.T) {
	data := bytes.Repeat([]byte{0}, 2*g729.FrameBytes)
	rep, err := analyzePayloadStream(bytes.NewReader(data), options{
		mode:  "payload",
		ptime: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Packets != 1 || rep.Frames != 2 || rep.PayloadBytes != 20 {
		t.Fatalf("report = %+v, want 1 packet / 2 frames / 20 bytes", rep)
	}
	if rep.Streams != 1 {
		t.Fatalf("streams = %d, want 1", rep.Streams)
	}
}

func TestAnalyzePayloadStreamRejectsSIDLikePayload(t *testing.T) {
	_, err := analyzePayloadStream(bytes.NewReader([]byte{0, 0}), options{
		mode:  "payload",
		ptime: 10,
	})
	if !errors.Is(err, g729.ErrUnsupportedAnnexB) {
		t.Fatalf("err = %v, want ErrUnsupportedAnnexB", err)
	}
}

func TestAnalyzePCAPEthernetRTP(t *testing.T) {
	payload := bytes.Repeat([]byte{0x55}, g729.FrameBytes)
	pcap := buildPCAPForTest(buildEthernetIPv4UDPRTPForTest(7, 160, 0x11223344, rtpPayloadType18, payload))

	rep, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Packets != 1 || rep.Frames != 1 || rep.PayloadBytes != g729.FrameBytes {
		t.Fatalf("report = %+v, want 1 packet / 1 frame / 10 bytes", rep)
	}
	if rep.Streams != 1 {
		t.Fatalf("streams = %d, want 1", rep.Streams)
	}
}

func TestAnalyzePCAPPtime20StrictTimestamp(t *testing.T) {
	payload := bytes.Repeat([]byte{0x44}, 2*g729.FrameBytes)
	frames := [][]byte{
		buildEthernetIPv4UDPRTPForTest(7, 160, 0x11223344, rtpPayloadType18, payload),
		buildEthernetIPv4UDPRTPForTest(8, 320, 0x11223344, rtpPayloadType18, payload),
	}
	pcap := buildPCAPForTest(frames...)

	rep, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       20,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Packets != 2 || rep.Frames != 4 || rep.PayloadBytes != 40 {
		t.Fatalf("report = %+v, want 2 packets / 4 frames / 40 bytes", rep)
	}
}

func TestAnalyzePCAPStrictSequenceRollover(t *testing.T) {
	payload := bytes.Repeat([]byte{0x22}, g729.FrameBytes)
	frames := [][]byte{
		buildEthernetIPv4UDPRTPForTest(0xffff, 160, 0x11223344, rtpPayloadType18, payload),
		buildEthernetIPv4UDPRTPForTest(0, 240, 0x11223344, rtpPayloadType18, payload),
	}
	pcap := buildPCAPForTest(frames...)

	if _, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	}); err != nil {
		t.Fatalf("sequence rollover should pass strict-ts: %v", err)
	}
}

func TestAnalyzePCAPRejectsSIDLikePayload(t *testing.T) {
	pcap := buildPCAPForTest(buildEthernetIPv4UDPRTPForTest(7, 160, 0x11223344, rtpPayloadType18, []byte{0, 0}))

	_, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       0,
		payloadType: rtpPayloadType18,
	})
	if !errors.Is(err, g729.ErrUnsupportedAnnexB) {
		t.Fatalf("err = %v, want ErrUnsupportedAnnexB", err)
	}
}

func TestAnalyzePCAPPaddingExtensionCSRC(t *testing.T) {
	payload := bytes.Repeat([]byte{0x66}, g729.FrameBytes)
	rtp := buildRTPForTestOptions(9, 160, 0x11223344, rtpPayloadType18, payload, rtpBuildOptions{
		marker:        true,
		paddingBytes:  4,
		csrcs:         []uint32{0x01020304, 0x05060708},
		extensionData: []byte{0xaa, 0xbb, 0xcc, 0xdd},
	})
	pcap := buildPCAPForTest(buildEthernetIPv4UDPRTPBytesForTest(rtp))

	rep, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.MarkerPackets != 1 || rep.PaddingPackets != 1 || rep.ExtensionPackets != 1 || rep.CSRCEntries != 2 {
		t.Fatalf("report = %+v, want marker/padding/extension/csrc counters", rep)
	}
}

func TestReportJSONShape(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	printReport(report{Mode: "payload", Packets: 1, Frames: 2, PayloadBytes: 20, Streams: 1}, true)
	var got report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, buf.String())
	}
	if got.Mode != "payload" || got.Packets != 1 || got.Frames != 2 || got.PayloadBytes != 20 || got.Streams != 1 {
		t.Fatalf("json report = %+v, want payload counters", got)
	}
}

func TestAnalyzePCAPStrictTimestamp(t *testing.T) {
	payload := bytes.Repeat([]byte{0x33}, g729.FrameBytes)
	frames := [][]byte{
		buildEthernetIPv4UDPRTPForTest(7, 160, 0x11223344, rtpPayloadType18, payload),
		buildEthernetIPv4UDPRTPForTest(8, 320, 0x11223344, rtpPayloadType18, payload),
	}
	pcap := buildPCAPForTest(frames...)

	_, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("err = %v, want strict timestamp error", err)
	}
}

func buildPCAPForTest(frames ...[]byte) []byte {
	var b bytes.Buffer
	var hdr [24]byte
	binary.LittleEndian.PutUint32(hdr[0:4], 0xa1b2c3d4)
	binary.LittleEndian.PutUint16(hdr[4:6], 2)
	binary.LittleEndian.PutUint16(hdr[6:8], 4)
	binary.LittleEndian.PutUint32(hdr[16:20], 65535)
	binary.LittleEndian.PutUint32(hdr[20:24], 1)
	b.Write(hdr[:])
	for i, frame := range frames {
		var rec [16]byte
		binary.LittleEndian.PutUint32(rec[0:4], uint32(i))
		binary.LittleEndian.PutUint32(rec[8:12], uint32(len(frame)))
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(frame)))
		b.Write(rec[:])
		b.Write(frame)
	}
	return b.Bytes()
}

func buildEthernetIPv4UDPRTPForTest(seq uint16, ts uint32, ssrc uint32, pt int, payload []byte) []byte {
	return buildEthernetIPv4UDPRTPBytesForTest(buildRTPForTest(seq, ts, ssrc, pt, payload))
}

func buildEthernetIPv4UDPRTPBytesForTest(rtp []byte) []byte {
	udpLen := 8 + len(rtp)
	ipLen := 20 + udpLen

	var b bytes.Buffer
	eth := make([]byte, 14)
	eth[12], eth[13] = 0x08, 0x00
	b.Write(eth)

	ip := make([]byte, 20)
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen))
	ip[8] = 64
	ip[9] = 17
	ip[12], ip[13], ip[14], ip[15] = 192, 0, 2, 10
	ip[16], ip[17], ip[18], ip[19] = 192, 0, 2, 20
	b.Write(ip)

	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[0:2], 4000)
	binary.BigEndian.PutUint16(udp[2:4], 5004)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	b.Write(udp)
	b.Write(rtp)
	return b.Bytes()
}

func buildRTPForTest(seq uint16, ts uint32, ssrc uint32, pt int, payload []byte) []byte {
	return buildRTPForTestOptions(seq, ts, ssrc, pt, payload, rtpBuildOptions{})
}

type rtpBuildOptions struct {
	marker        bool
	paddingBytes  int
	csrcs         []uint32
	extensionData []byte
}

func buildRTPForTestOptions(seq uint16, ts uint32, ssrc uint32, pt int, payload []byte, opt rtpBuildOptions) []byte {
	var b bytes.Buffer
	first := byte(0x80 | (len(opt.csrcs) & 0x0f))
	if opt.paddingBytes > 0 {
		first |= 0x20
	}
	if len(opt.extensionData) > 0 {
		first |= 0x10
	}
	b.WriteByte(first)
	second := byte(pt & 0x7f)
	if opt.marker {
		second |= 0x80
	}
	b.WriteByte(second)
	var hdr [10]byte
	binary.BigEndian.PutUint16(hdr[0:2], seq)
	binary.BigEndian.PutUint32(hdr[2:6], ts)
	binary.BigEndian.PutUint32(hdr[6:10], ssrc)
	b.Write(hdr[:])
	for _, csrc := range opt.csrcs {
		var v [4]byte
		binary.BigEndian.PutUint32(v[:], csrc)
		b.Write(v[:])
	}
	if len(opt.extensionData) > 0 {
		var ext [4]byte
		binary.BigEndian.PutUint16(ext[0:2], 0xbede)
		words := (len(opt.extensionData) + 3) / 4
		binary.BigEndian.PutUint16(ext[2:4], uint16(words))
		b.Write(ext[:])
		b.Write(opt.extensionData)
		for pad := words*4 - len(opt.extensionData); pad > 0; pad-- {
			b.WriteByte(0)
		}
	}
	b.Write(payload)
	if opt.paddingBytes > 0 {
		for i := 1; i < opt.paddingBytes; i++ {
			b.WriteByte(0)
		}
		b.WriteByte(byte(opt.paddingBytes))
	}
	return b.Bytes()
}
