package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	g729 "github.com/hunydev/g729"
	"github.com/pion/rtp"
)

func TestAnalyzePCAPPionRTPPtime20(t *testing.T) {
	payload := bytes.Repeat([]byte{0x5a}, 2*g729.FrameBytes)
	packets := [][]byte{
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 40000, 160, 0x01020304, rtpPayloadType18, payload)),
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 40001, 320, 0x01020304, rtpPayloadType18, payload)),
	}

	rep, err := analyzePCAP(bytes.NewReader(buildPCAPForTest(packets...)), options{
		mode:        "pcap",
		ptime:       20,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Packets != 2 || rep.Frames != 4 || rep.PayloadBytes != 40 || rep.Streams != 1 {
		t.Fatalf("report = %+v, want 2 packets / 4 frames / 40 bytes / 1 stream", rep)
	}
}

func TestAnalyzePCAPPionRTPRejectsAnnexBSIDPayload(t *testing.T) {
	packet := marshalPionRTPForTest(t, 7, 160, 0x01020304, rtpPayloadType18, []byte{0x00, 0x00})
	pcap := buildPCAPForTest(buildEthernetIPv4UDPRTPBytesForTest(packet))

	_, err := analyzePCAP(bytes.NewReader(pcap), options{
		mode:        "pcap",
		ptime:       0,
		payloadType: rtpPayloadType18,
	})
	if !errors.Is(err, g729.ErrUnsupportedAnnexB) {
		t.Fatalf("err = %v, want ErrUnsupportedAnnexB", err)
	}
}

func TestAnalyzePCAPPionRTPMultiSSRCAndPayloadTypeSkip(t *testing.T) {
	payload := bytes.Repeat([]byte{0x2a}, g729.FrameBytes)
	packets := [][]byte{
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 10, 800, 0x01020304, rtpPayloadType18, payload)),
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 11, 880, 0x01020304, 96, payload)),
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 20, 1600, 0x11121314, rtpPayloadType18, payload)),
	}

	rep, err := analyzePCAP(bytes.NewReader(buildPCAPForTest(packets...)), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Packets != 2 || rep.Frames != 2 || rep.Skipped != 1 || rep.Streams != 2 {
		t.Fatalf("report = %+v, want 2 packets / 2 frames / 1 skipped / 2 streams", rep)
	}
}

func TestAnalyzePCAPPionRTPHeaderCounters(t *testing.T) {
	payload := bytes.Repeat([]byte{0x71}, g729.FrameBytes)
	packet := marshalPionRTPForTestOptions(t, 12, 960, 0x01020304, rtpPayloadType18, payload, pionRTPOptions{
		marker:           true,
		paddingSize:      4,
		csrc:             []uint32{0x20212223, 0x30313233},
		extensionID:      1,
		extensionPayload: []byte{0xaa, 0xbb},
	})
	pcap := buildPCAPForTest(buildEthernetIPv4UDPRTPBytesForTest(packet))

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
	if rep.PayloadBytes != g729.FrameBytes {
		t.Fatalf("payload bytes = %d, want %d after RTP padding trim", rep.PayloadBytes, g729.FrameBytes)
	}
}

func TestAnalyzePCAPPionRTPStrictTimestampDiscontinuity(t *testing.T) {
	payload := bytes.Repeat([]byte{0x5c}, g729.FrameBytes)
	packets := [][]byte{
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 40, 3200, 0x01020304, rtpPayloadType18, payload)),
		buildEthernetIPv4UDPRTPBytesForTest(marshalPionRTPForTest(t, 41, 3400, 0x01020304, rtpPayloadType18, payload)),
	}

	_, err := analyzePCAP(bytes.NewReader(buildPCAPForTest(packets...)), options{
		mode:        "pcap",
		ptime:       10,
		payloadType: rtpPayloadType18,
		strictTS:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("err = %v, want strict timestamp error", err)
	}
}

func TestParseRTPPionMalformedPadding(t *testing.T) {
	payload := bytes.Repeat([]byte{0x45}, g729.FrameBytes)
	packet := marshalPionRTPForTestOptions(t, 50, 4000, 0x01020304, rtpPayloadType18, payload, pionRTPOptions{
		paddingSize: 4,
	})
	packet[len(packet)-1] = 0

	_, err := parseRTP(packet)
	if err == nil || !strings.Contains(err.Error(), "invalid RTP padding length") {
		t.Fatalf("err = %v, want invalid RTP padding length", err)
	}
}

func TestParseRTPPionMalformedExtension(t *testing.T) {
	payload := bytes.Repeat([]byte{0x46}, g729.FrameBytes)
	packet := marshalPionRTPForTestOptions(t, 51, 4080, 0x01020304, rtpPayloadType18, payload, pionRTPOptions{
		extensionID:      1,
		extensionPayload: []byte{0xaa, 0xbb},
	})
	packet = packet[:15]

	_, err := parseRTP(packet)
	if err == nil || !strings.Contains(err.Error(), "short RTP extension header") {
		t.Fatalf("err = %v, want short RTP extension header", err)
	}
}

func marshalPionRTPForTest(t *testing.T, seq uint16, ts uint32, ssrc uint32, pt int, payload []byte) []byte {
	return marshalPionRTPForTestOptions(t, seq, ts, ssrc, pt, payload, pionRTPOptions{})
}

type pionRTPOptions struct {
	marker           bool
	paddingSize      byte
	csrc             []uint32
	extensionID      uint8
	extensionPayload []byte
}

func marshalPionRTPForTestOptions(t *testing.T, seq uint16, ts uint32, ssrc uint32, pt int, payload []byte, opt pionRTPOptions) []byte {
	t.Helper()

	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         opt.marker,
			PayloadType:    uint8(pt),
			SequenceNumber: seq,
			Timestamp:      ts,
			SSRC:           ssrc,
			CSRC:           opt.csrc,
			Padding:        opt.paddingSize > 0,
			PaddingSize:    opt.paddingSize,
		},
		Payload: payload,
	}
	if len(opt.extensionPayload) > 0 {
		if err := packet.SetExtension(opt.extensionID, opt.extensionPayload); err != nil {
			t.Fatal(err)
		}
	}
	data, err := packet.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
