package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	g729 "github.com/hunydev/g729"
)

func TestBuildReportIncludesRTPAndDecodeMetrics(t *testing.T) {
	payload := bytes.Repeat([]byte{0x00}, 2*g729.FrameBytes)
	pcap := buildPCAPForReportTest(buildEthernetIPv4UDPRTPForReportTest(10, 160, 0x01020304, 18, payload))

	rep, err := buildReport(bytes.NewReader(pcap), options{
		inputPath:   "/tmp/g729-call.pcap",
		payloadType: 18,
		ptime:       20,
		strictTS:    true,
		decode:      true,
		commit:      "test-commit",
		peer:        "black-box-pbx 1.0",
		peerRole:    "pbx",
		topology:    "local encoder -> peer",
	}, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if rep.SchemaVersion != schemaVersion || rep.Tool != "cmd/g729rtpreport" {
		t.Fatalf("report header = %+v", rep)
	}
	if rep.Repository.Commit != "test-commit" {
		t.Fatalf("commit = %q, want override", rep.Repository.Commit)
	}
	if rep.RTP.Packets != 1 || rep.RTP.Frames != 2 || rep.RTP.PayloadBytes != 20 || rep.RTP.DecodedFrames != 2 {
		t.Fatalf("rtp report = %+v, want 1 packet / 2 frames / 20 bytes / 2 decoded", rep.RTP)
	}
	if rep.RTP.Decode == nil || rep.RTP.Decode.Samples != 2*g729.FrameSamples {
		t.Fatalf("decode metrics = %+v, want %d samples", rep.RTP.Decode, 2*g729.FrameSamples)
	}
	if rep.Negotiation.AnnexB != "no" || !rep.Negotiation.StrictTS {
		t.Fatalf("negotiation = %+v, want annexb=no strict-ts", rep.Negotiation)
	}
	if len(rep.Boundary) == 0 {
		t.Fatalf("boundary is empty")
	}
}

func TestWriteJSONShape(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSON(&out, evidenceReport{
		SchemaVersion: schemaVersion,
		Tool:          "cmd/g729rtpreport",
		GeneratedAt:   "2026-05-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != schemaVersion || got["tool"] != "cmd/g729rtpreport" {
		t.Fatalf("json = %v, want schema/tool", got)
	}
}

func buildPCAPForReportTest(frames ...[]byte) []byte {
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

func buildEthernetIPv4UDPRTPForReportTest(seq uint16, ts uint32, ssrc uint32, pt int, payload []byte) []byte {
	udpLen := 8 + 12 + len(payload)
	ipLen := 20 + udpLen
	frame := make([]byte, 0, 14+ipLen)

	eth := make([]byte, 14)
	eth[12], eth[13] = 0x08, 0x00
	frame = append(frame, eth...)

	ip := make([]byte, 20)
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen))
	ip[8] = 64
	ip[9] = 17
	ip[12], ip[13], ip[14], ip[15] = 192, 0, 2, 10
	ip[16], ip[17], ip[18], ip[19] = 192, 0, 2, 20
	frame = append(frame, ip...)

	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[0:2], 4000)
	binary.BigEndian.PutUint16(udp[2:4], 5004)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	frame = append(frame, udp...)

	rtp := make([]byte, 12)
	rtp[0] = 0x80
	rtp[1] = byte(pt & 0x7f)
	binary.BigEndian.PutUint16(rtp[2:4], seq)
	binary.BigEndian.PutUint32(rtp[4:8], ts)
	binary.BigEndian.PutUint32(rtp[8:12], ssrc)
	frame = append(frame, rtp...)
	frame = append(frame, payload...)
	return frame
}
