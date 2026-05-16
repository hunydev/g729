package main

import (
	"bytes"
	"errors"
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

func marshalPionRTPForTest(t *testing.T, seq uint16, ts uint32, ssrc uint32, pt int, payload []byte) []byte {
	t.Helper()

	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    uint8(pt),
			SequenceNumber: seq,
			Timestamp:      ts,
			SSRC:           ssrc,
		},
		Payload: payload,
	}
	data, err := packet.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
