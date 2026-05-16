package main

import (
	"bytes"
	"encoding/binary"
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
	rtp := buildRTPForTest(seq, ts, ssrc, pt, payload)
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
	var b bytes.Buffer
	b.WriteByte(0x80)
	b.WriteByte(byte(pt & 0x7f))
	var hdr [10]byte
	binary.BigEndian.PutUint16(hdr[0:2], seq)
	binary.BigEndian.PutUint32(hdr[2:6], ts)
	binary.BigEndian.PutUint32(hdr[6:10], ssrc)
	b.Write(hdr[:])
	b.Write(payload)
	return b.Bytes()
}
