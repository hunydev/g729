package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/hunydev/g729"
	"github.com/pion/rtp"
)

func TestRunSpeechFixtureWithWrongPTAndMultiSSRC(t *testing.T) {
	var out bytes.Buffer
	err := run(&out, options{
		ptime:            20,
		packets:          4,
		payloadType:      18,
		wrongPayloadType: 96,
		includeWrongPT:   true,
		multiSSRC:        true,
		sequence:         1000,
		timestamp:        3200,
		ssrc:             0x11223344,
	})
	if err != nil {
		t.Fatal(err)
	}

	records := readPCAPRecordsForTest(t, out.Bytes())
	if len(records) != 5 {
		t.Fatalf("records = %d, want 5", len(records))
	}

	p0 := parseFixtureRTPForTest(t, records[0])
	if p0.PayloadType != 18 || p0.SequenceNumber != 1000 || p0.Timestamp != 3200 || p0.SSRC != 0x11223344 || !p0.Marker {
		t.Fatalf("packet 0 header = %+v, want PT 18 seq 1000 ts 3200 primary SSRC marker", p0.Header)
	}
	if len(p0.Payload) != 2*g729.FrameBytes {
		t.Fatalf("packet 0 payload bytes = %d, want %d", len(p0.Payload), 2*g729.FrameBytes)
	}

	p1 := parseFixtureRTPForTest(t, records[1])
	if p1.PayloadType != 18 || p1.SequenceNumber != 1000 || p1.Timestamp != 3200 || p1.SSRC != 0x11223345 || p1.Marker {
		t.Fatalf("packet 1 header = %+v, want PT 18 seq 1000 ts 3200 alternate SSRC no marker", p1.Header)
	}

	p2 := parseFixtureRTPForTest(t, records[2])
	if p2.PayloadType != 18 || p2.SequenceNumber != 1001 || p2.Timestamp != 3360 || p2.SSRC != 0x11223344 {
		t.Fatalf("packet 2 header = %+v, want PT 18 seq 1001 ts 3360 primary SSRC", p2.Header)
	}

	p3 := parseFixtureRTPForTest(t, records[3])
	if p3.PayloadType != 18 || p3.SequenceNumber != 1001 || p3.Timestamp != 3360 || p3.SSRC != 0x11223345 {
		t.Fatalf("packet 3 header = %+v, want PT 18 seq 1001 ts 3360 alternate SSRC", p3.Header)
	}

	p4 := parseFixtureRTPForTest(t, records[4])
	if p4.PayloadType != 96 || p4.SequenceNumber != 1002 || p4.Timestamp != 3520 || p4.SSRC != 0x11223344 {
		t.Fatalf("packet 4 header = %+v, want wrong PT 96 seq 1002 ts 3520 primary SSRC", p4.Header)
	}
}

func TestRunSIDFixture(t *testing.T) {
	var out bytes.Buffer
	err := run(&out, options{
		ptime:            10,
		packets:          3,
		payloadType:      18,
		wrongPayloadType: 96,
		sid:              true,
		ssrc:             0x01020304,
	})
	if err != nil {
		t.Fatal(err)
	}

	records := readPCAPRecordsForTest(t, out.Bytes())
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	packet := parseFixtureRTPForTest(t, records[0])
	if packet.PayloadType != 18 || packet.SSRC != 0x01020304 || len(packet.Payload) != 2 {
		t.Fatalf("sid packet = header %+v payload bytes %d, want PT 18 SSRC 01020304 2-byte payload", packet.Header, len(packet.Payload))
	}
}

func TestValidateOptionsRejectsInvalidValues(t *testing.T) {
	tests := []options{
		{ptime: 30, packets: 1, payloadType: 18, wrongPayloadType: 96},
		{ptime: 20, packets: 0, payloadType: 18, wrongPayloadType: 96},
		{ptime: 20, packets: 1, payloadType: 128, wrongPayloadType: 96},
		{ptime: 20, packets: 1, payloadType: 18, wrongPayloadType: 128},
		{ptime: 20, packets: 1, payloadType: 18, wrongPayloadType: 96, sequence: 0x10000},
		{ptime: 20, packets: 1, payloadType: 18, wrongPayloadType: 96, timestamp: 0x100000000},
		{ptime: 20, packets: 1, payloadType: 18, wrongPayloadType: 96, ssrc: 0x100000000},
	}
	for _, opt := range tests {
		if err := validateOptions(opt); err == nil {
			t.Fatalf("validateOptions(%+v) succeeded, want error", opt)
		}
	}
}

func readPCAPRecordsForTest(t *testing.T, data []byte) [][]byte {
	t.Helper()
	if len(data) < 24 {
		t.Fatalf("pcap too short: %d", len(data))
	}
	if got := binary.LittleEndian.Uint32(data[0:4]); got != 0xa1b2c3d4 {
		t.Fatalf("pcap magic = %08x, want a1b2c3d4", got)
	}
	if got := binary.LittleEndian.Uint32(data[20:24]); got != 1 {
		t.Fatalf("pcap linktype = %d, want Ethernet", got)
	}

	var records [][]byte
	off := 24
	for off < len(data) {
		if len(data)-off < 16 {
			t.Fatalf("truncated record header at offset %d", off)
		}
		inclLen := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
		off += 16
		if inclLen < 0 || len(data)-off < inclLen {
			t.Fatalf("truncated record body at offset %d len %d", off, inclLen)
		}
		records = append(records, data[off:off+inclLen])
		off += inclLen
	}
	return records
}

func parseFixtureRTPForTest(t *testing.T, ethernet []byte) rtp.Packet {
	t.Helper()
	if len(ethernet) < 14+20+8+12 {
		t.Fatalf("fixture frame too short: %d", len(ethernet))
	}
	if got := binary.BigEndian.Uint16(ethernet[12:14]); got != 0x0800 {
		t.Fatalf("ether type = %04x, want IPv4", got)
	}
	ip := ethernet[14:]
	ihl := int(ip[0]&0x0f) * 4
	totalLen := int(binary.BigEndian.Uint16(ip[2:4]))
	udp := ip[ihl:totalLen]
	udpLen := int(binary.BigEndian.Uint16(udp[4:6]))
	var packet rtp.Packet
	if err := packet.Unmarshal(udp[8:udpLen]); err != nil {
		t.Fatal(err)
	}
	return packet
}
