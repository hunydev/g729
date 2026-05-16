package main

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/hunydev/g729"
	"github.com/pion/rtp"
)

func TestRunPionPacketizePtime20(t *testing.T) {
	input := bytes.Repeat([]byte{0x33}, 2*g729.FrameBytes)
	var out bytes.Buffer

	err := run(bytes.NewReader(input), &out, options{
		ptime:       20,
		payloadType: 18,
		sequence:    1000,
		timestamp:   3200,
		ssrc:        0x11223344,
		markerFirst: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(out.String())
	if !strings.Contains(line, "packet=0 seq=1000 ts=3200 ssrc=0x11223344 pt=18 marker=true payload_bytes=20 rtp=") {
		t.Fatalf("line = %q, want RTP metadata", line)
	}

	rawHex := strings.TrimPrefix(line[strings.LastIndex(line, "rtp="):], "rtp=")
	raw, err := hex.DecodeString(rawHex)
	if err != nil {
		t.Fatal(err)
	}
	var packet rtp.Packet
	if err := packet.Unmarshal(raw); err != nil {
		t.Fatal(err)
	}
	if packet.PayloadType != 18 || packet.SequenceNumber != 1000 || packet.Timestamp != 3200 || packet.SSRC != 0x11223344 || !packet.Marker {
		t.Fatalf("packet header = %+v, want PT 18 seq 1000 ts 3200 SSRC 11223344 marker", packet.Header)
	}
	if len(packet.Payload) != 2*g729.FrameBytes {
		t.Fatalf("payload bytes = %d, want %d", len(packet.Payload), 2*g729.FrameBytes)
	}
}

func TestRunPionPacketizeRejectsTrailingPartialPacket(t *testing.T) {
	input := bytes.Repeat([]byte{0x33}, g729.FrameBytes)
	var out bytes.Buffer

	err := run(bytes.NewReader(input), &out, options{
		ptime:       20,
		payloadType: 18,
	})
	if err == nil || !strings.Contains(err.Error(), "trailing partial RTP packet") {
		t.Fatalf("err = %v, want trailing partial RTP packet", err)
	}
}
