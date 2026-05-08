package main

import (
	"bytes"
	"encoding/base64"
	"math"
	"mime/multipart"
	"testing"

	g729 "github.com/hunydev/g729"
)

func TestNormalizeHostAllowsDomainWithPort(t *testing.T) {
	if got := normalizeHost("g729ab.exe.xyz:443"); got != "g729ab.exe.xyz" {
		t.Fatalf("normalizeHost = %q, want g729ab.exe.xyz", got)
	}
}

func TestCheckSDPPassesRequiredG729Offer(t *testing.T) {
	resp := checkSDP("m=audio 49170 RTP/AVP 18\r\na=rtpmap:18 G729/8000\r\na=fmtp:18 annexb=no\r\na=ptime:20\r\na=maxptime:20\r\n")
	if !resp.OK {
		t.Fatalf("checkSDP failed: %+v", resp.Checks)
	}
}

func TestCheckSDPRejectsAnnexBYes(t *testing.T) {
	resp := checkSDP("m=audio 49170 RTP/AVP 18\na=rtpmap:18 G729/8000\na=fmtp:18 annexb=yes\na=ptime:20\na=maxptime:20")
	if resp.OK {
		t.Fatalf("checkSDP passed annexb=yes: %+v", resp.Checks)
	}
}

func TestCheckPayloadPtime20(t *testing.T) {
	payload := bytes.Repeat([]byte{0}, 2*g729.FrameBytes)
	resp, err := checkPayload(bytes.NewReader(payload), &multipart.FileHeader{Filename: "sample.g729"}, 20, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Packets != 1 || resp.Frames != 2 || resp.DecodedFrames != 2 {
		t.Fatalf("payload response = %+v", resp)
	}
	if resp.DecodedSamples != 2*g729.FrameSamples || resp.DecodedPreviewSamples != resp.DecodedSamples {
		t.Fatalf("decoded sample counts = %d/%d", resp.DecodedSamples, resp.DecodedPreviewSamples)
	}
	if resp.DecodedWAVBase64 == "" || resp.NormalizedDecodedWAVBase64 == "" {
		t.Fatalf("missing decoded WAV payload artifacts: %+v", resp)
	}
	if resp.RecoveredDecodedWAVBase64 == "" || resp.RecoveredNormalizedWAVBase64 == "" {
		t.Fatalf("missing recovered WAV payload artifacts: %+v", resp)
	}
	if !hasCheck(resp.Checks, "audibility") {
		t.Fatalf("missing payload audibility diagnostic: %+v", resp.Checks)
	}
	if !hasCheck(resp.Checks, "enhanced listening aid") {
		t.Fatalf("missing payload enhanced listening diagnostic: %+v", resp.Checks)
	}
}

func TestCheckPayloadNoDecodeSkipsAudioArtifacts(t *testing.T) {
	payload := bytes.Repeat([]byte{0}, 2*g729.FrameBytes)
	resp, err := checkPayload(bytes.NewReader(payload), &multipart.FileHeader{Filename: "sample.g729"}, 20, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.DecodedFrames != 0 || resp.DecodedWAVBase64 != "" {
		t.Fatalf("payload response = %+v", resp)
	}
}

func TestCheckPayloadPtime20AcceptsFinalPartialGroup(t *testing.T) {
	payload := bytes.Repeat([]byte{0}, 3*g729.FrameBytes)
	resp, err := checkPayload(bytes.NewReader(payload), &multipart.FileHeader{Filename: "sample.g729"}, 20, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Frames != 3 || resp.Packets != 2 {
		t.Fatalf("payload response = %+v", resp)
	}
	if !hasCheck(resp.Checks, "ptime grouping") {
		t.Fatalf("missing ptime grouping diagnostic: %+v", resp.Checks)
	}
}

func TestRunSelfTest(t *testing.T) {
	resp := runSelfTest()
	if !resp.OK {
		t.Fatalf("self test failed: %+v", resp.Checks)
	}
}

func TestRunRoundtrip(t *testing.T) {
	pcm := make([]byte, g729.FrameSamples*2)
	for i := 0; i < g729.FrameSamples; i++ {
		v := int16(i * 97)
		pcm[i*2] = byte(uint16(v))
		pcm[i*2+1] = byte(uint16(v) >> 8)
	}
	resp, err := runRoundtrip(bytes.NewReader(pcm), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Frames != 1 || resp.EncodedBytes != g729.FrameBytes || resp.OutputSamples != g729.FrameSamples {
		t.Fatalf("roundtrip response = %+v", resp)
	}
	if resp.DecodedWAVBase64 == "" || resp.NormalizedDecodedWAVBase64 == "" || resp.EncodedG729Base64 == "" {
		t.Fatalf("missing encoded output artifacts: %+v", resp)
	}
	if resp.RecoveredDecodedWAVBase64 == "" || resp.RecoveredNormalizedWAVBase64 == "" {
		t.Fatalf("missing recovered roundtrip artifacts: %+v", resp)
	}
	encoded, err := base64.StdEncoding.DecodeString(resp.EncodedG729Base64)
	if err != nil {
		t.Fatalf("encoded G.729 artifact is not valid base64: %v", err)
	}
	if len(encoded) != resp.EncodedBytes {
		t.Fatalf("encoded artifact length = %d, want %d", len(encoded), resp.EncodedBytes)
	}
	if !hasCheck(resp.Checks, "audibility") {
		t.Fatalf("missing audibility diagnostic: %+v", resp.Checks)
	}
	if !hasCheck(resp.Checks, "enhanced listening aid") {
		t.Fatalf("missing enhanced listening diagnostic: %+v", resp.Checks)
	}
}

func TestBestAlignedQualityFindsShift(t *testing.T) {
	ref := []int16{1000, -1000, 500, -500, 250, -250, 0}
	test := []int16{0, 0, 1000, -1000, 500, -500, 250, -250}
	q := bestAlignedQuality(ref, test, 4)
	if q.shift != 2 {
		t.Fatalf("shift = %d, want 2", q.shift)
	}
	if q.globalSNR < 40 {
		t.Fatalf("global SNR = %.2f, want high aligned match", q.globalSNR)
	}
}

func TestMatchFrameRMSScalesEachFrame(t *testing.T) {
	test := []int16{1000, -1000, 500, -500, 100, -100, 50, -50}
	ref := []int16{2000, -2000, 1000, -1000, 25, -25, 25, -25}
	out := matchFrameRMS(test, ref, 4)
	firstRatio := rms(out[:4]) / rms(ref[:4])
	secondRatio := rms(out[4:]) / rms(ref[4:])
	if math.Abs(firstRatio-1) > 0.02 {
		t.Fatalf("first frame RMS ratio %.3f, want ~1; out=%v", firstRatio, out)
	}
	if math.Abs(secondRatio-1) > 0.02 {
		t.Fatalf("second frame RMS ratio %.3f, want ~1; out=%v", secondRatio, out)
	}
}

func TestNormalizeForListeningRaisesLowLevelSignal(t *testing.T) {
	in := []int16{0, 1, -2, 3, -4}
	out := normalizeForListening(in, 24000)
	peak, _ := peakAndClipped(out)
	if peak != 24000 {
		t.Fatalf("normalized peak = %d, want 24000; out=%v", peak, out)
	}
}

func hasCheck(checks []check, name string) bool {
	for _, c := range checks {
		if c.Name == name {
			return true
		}
	}
	return false
}
