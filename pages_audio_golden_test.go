package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestPagesAudioGoldenOutputs(t *testing.T) {
	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/source-8k-16bit.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	padded := padPCM16ToFrames(t, sourcePCM)
	encoded := encodePCM16LEForGolden(t, padded)
	wantEncoded := readGoldenFile(t, "docs/assets/audio/g729-encode.g729")
	if !bytes.Equal(encoded, wantEncoded) {
		t.Fatalf("Pages encoder golden mismatch: %s", byteMismatch(encoded, wantEncoded))
	}

	decoded := decodeG729ForGolden(t, wantEncoded)
	wantDecoded := readPagesAudioWAVFixture(t, "docs/assets/audio/g729-encode-g729-decode.wav", len(padded))
	if !bytes.Equal(decoded, wantDecoded) {
		t.Fatalf("Pages decoder golden mismatch: %s", byteMismatch(decoded, wantDecoded))
	}

	bcgPayload := readGoldenFile(t, "docs/assets/audio/bcg729-encode.g729")
	if len(bcgPayload) != len(wantEncoded) {
		t.Fatalf("bcg729 payload bytes = %d, want %d", len(bcgPayload), len(wantEncoded))
	}
	bcgDecoded := decodeG729ForGolden(t, bcgPayload)
	wantBCGDecoded := readPagesAudioWAVFixture(t, "docs/assets/audio/bcg729-encode-g729-decode.wav", len(padded))
	if !bytes.Equal(bcgDecoded, wantBCGDecoded) {
		t.Fatalf("Pages bcg729 payload local decode golden mismatch: %s", byteMismatch(bcgDecoded, wantBCGDecoded))
	}

	readPagesAudioWAVFixture(t, "docs/assets/audio/g729-encode-ffmpeg-decode.wav", len(padded))
	readPagesAudioWAVFixture(t, "docs/assets/audio/bcg729-encode-ffmpeg-decode.wav", len(padded))
}

func TestPagesAudioArenaOutputs(t *testing.T) {
	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/arena/source-osr-us-0010-8k.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("arena source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	for _, clip := range pagesArenaClips {
		clipPCM := normalizeArenaClipPCM16(t, arenaClipPCMBytes(t, sourcePCM, clip.offsetSamples, clip.sampleCount))
		padded := padPCM16ToFrames(t, clipPCM)
		encoded := encodePCM16LEForGolden(t, padded)
		decoded := decodeG729ForGolden(t, encoded)
		wantDecoded := readPagesAudioWAVFixture(t, arenaAudioPath(clip.name, "our-loopback.wav"), len(padded))
		if !bytes.Equal(decoded, wantDecoded) {
			t.Fatalf("arena %s local loopback mismatch: %s", clip.name, byteMismatch(decoded, wantDecoded))
		}
		readPagesAudioWAVFixture(t, arenaAudioPath(clip.name, "bcg729-ffmpeg.wav"), len(padded))
	}
}

func TestPagesAudioWriteGoldenOutputs(t *testing.T) {
	if os.Getenv("G729_WRITE_PAGES_AUDIO_GOLDEN") != "1" {
		t.Skip("set G729_WRITE_PAGES_AUDIO_GOLDEN=1 to refresh docs/assets/audio golden outputs")
	}

	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/source-8k-16bit.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	padded := padPCM16ToFrames(t, sourcePCM)
	encoded := encodePCM16LEForGolden(t, padded)
	if err := os.WriteFile("docs/assets/audio/g729-encode.g729", encoded, 0o644); err != nil {
		t.Fatalf("write g729-encode.g729: %v", err)
	}

	decoded := decodeG729ForGolden(t, encoded)
	writePCM16WAVFixture(t, "docs/assets/audio/g729-encode-g729-decode.wav", decoded)
	writeFFmpegDecodedG729WAVFixture(t, "docs/assets/audio/g729-encode-ffmpeg-decode.wav", encoded)

	bcgEncoded := encodeBCG729ForPagesGolden(t, padded)
	if err := os.WriteFile("docs/assets/audio/bcg729-encode.g729", bcgEncoded, 0o644); err != nil {
		t.Fatalf("write bcg729-encode.g729: %v", err)
	}
	bcgDecoded := decodeG729ForGolden(t, bcgEncoded)
	writePCM16WAVFixture(t, "docs/assets/audio/bcg729-encode-g729-decode.wav", bcgDecoded)
	writeFFmpegDecodedG729WAVFixture(t, "docs/assets/audio/bcg729-encode-ffmpeg-decode.wav", bcgEncoded)

	t.Logf("wrote local payload %d bytes, bcg729 payload %d bytes, decoded PCM %d bytes", len(encoded), len(bcgEncoded), len(decoded))
}

func TestPagesAudioArenaWriteGoldenOutputs(t *testing.T) {
	if os.Getenv("G729_WRITE_PAGES_AUDIO_ARENA") != "1" {
		t.Skip("set G729_WRITE_PAGES_AUDIO_ARENA=1 to refresh docs/assets/audio/arena outputs")
	}

	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/arena/source-osr-us-0010-8k.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("arena source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	for _, clip := range pagesArenaClips {
		clipPCM := normalizeArenaClipPCM16(t, arenaClipPCMBytes(t, sourcePCM, clip.offsetSamples, clip.sampleCount))
		padded := padPCM16ToFrames(t, clipPCM)

		encoded := encodePCM16LEForGolden(t, padded)
		decoded := decodeG729ForGolden(t, encoded)
		writePCM16WAVFixture(t, arenaAudioPath(clip.name, "our-loopback.wav"), decoded)

		bcgEncoded := encodeBCG729ForPagesGolden(t, padded)
		writeFFmpegDecodedG729WAVFixture(t, arenaAudioPath(clip.name, "bcg729-ffmpeg.wav"), bcgEncoded)
		t.Logf("wrote arena %s: %d source samples, %d padded samples", clip.name, clip.sampleCount, len(padded)/2)
	}
}

var pagesArenaClips = []struct {
	name          string
	offsetSamples int
	sampleCount   int
}{
	{name: "trial-01", offsetSamples: 4160, sampleCount: 12800},
	{name: "trial-02", offsetSamples: 34400, sampleCount: 12800},
	{name: "trial-03", offsetSamples: 45600, sampleCount: 12800},
	{name: "trial-04", offsetSamples: 61760, sampleCount: 12800},
	{name: "trial-05", offsetSamples: 88160, sampleCount: 12800},
	{name: "trial-06", offsetSamples: 114800, sampleCount: 12800},
	{name: "trial-07", offsetSamples: 139040, sampleCount: 12800},
	{name: "trial-08", offsetSamples: 155600, sampleCount: 12800},
	{name: "trial-09", offsetSamples: 188960, sampleCount: 12800},
	{name: "trial-10", offsetSamples: 238560, sampleCount: 12800},
}

func arenaAudioPath(name, suffix string) string {
	return fmt.Sprintf("docs/assets/audio/arena/%s-%s", name, suffix)
}

func arenaClipPCMBytes(t *testing.T, pcm []byte, offsetSamples, sampleCount int) []byte {
	t.Helper()
	start := offsetSamples * 2
	end := start + sampleCount*2
	if start < 0 || end > len(pcm) || start > end {
		t.Fatalf("arena clip range [%d,%d) outside source PCM length %d", start, end, len(pcm))
	}
	return append([]byte(nil), pcm[start:end]...)
}

func normalizeArenaClipPCM16(t *testing.T, pcm []byte) []byte {
	t.Helper()
	if len(pcm)%2 != 0 {
		t.Fatalf("PCM byte length is odd: %d", len(pcm))
	}
	const targetPeak = 18000
	maxAbs := 0
	for off := 0; off < len(pcm); off += 2 {
		sample := int(int16(binary.LittleEndian.Uint16(pcm[off:])))
		if sample < 0 {
			sample = -sample
		}
		if sample > maxAbs {
			maxAbs = sample
		}
	}
	if maxAbs == 0 {
		return append([]byte(nil), pcm...)
	}

	out := make([]byte, len(pcm))
	for off := 0; off < len(pcm); off += 2 {
		sample := int64(int16(binary.LittleEndian.Uint16(pcm[off:])))
		scaled := sample * targetPeak / int64(maxAbs)
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(out[off:], uint16(int16(scaled)))
	}
	return out
}

type wavFixtureFormat struct {
	sampleRate    int
	channels      int
	bitsPerSample int
}

func readPCM16WAVFixture(t *testing.T, path string) ([]byte, wavFixtureFormat) {
	t.Helper()
	data := readGoldenFile(t, path)
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("%s is not a RIFF/WAVE file", path)
	}

	var format wavFixtureFormat
	haveFmt := false
	var pcm []byte
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		chunkStart := off + 8
		chunkEnd := chunkStart + size
		if size < 0 || chunkEnd > len(data) {
			t.Fatalf("%s has truncated %q chunk", path, id)
		}

		switch id {
		case "fmt ":
			if size < 16 {
				t.Fatalf("%s fmt chunk too short: %d", path, size)
			}
			audioFormat := binary.LittleEndian.Uint16(data[chunkStart : chunkStart+2])
			if audioFormat != 1 {
				t.Fatalf("%s audio format = %d; want PCM", path, audioFormat)
			}
			format.channels = int(binary.LittleEndian.Uint16(data[chunkStart+2 : chunkStart+4]))
			format.sampleRate = int(binary.LittleEndian.Uint32(data[chunkStart+4 : chunkStart+8]))
			format.bitsPerSample = int(binary.LittleEndian.Uint16(data[chunkStart+14 : chunkStart+16]))
			haveFmt = true
		case "data":
			pcm = append([]byte(nil), data[chunkStart:chunkEnd]...)
		}

		off = chunkEnd
		if off%2 == 1 {
			off++
		}
	}
	if !haveFmt {
		t.Fatalf("%s missing fmt chunk", path)
	}
	if pcm == nil {
		t.Fatalf("%s missing data chunk", path)
	}
	if len(pcm)%2 != 0 {
		t.Fatalf("%s PCM byte length is odd: %d", path, len(pcm))
	}
	return pcm, format
}

func readPagesAudioWAVFixture(t *testing.T, path string, wantPCMBytes int) []byte {
	t.Helper()
	pcm, format := readPCM16WAVFixture(t, path)
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("%s format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			path, format.sampleRate, format.channels, format.bitsPerSample)
	}
	if len(pcm) != wantPCMBytes {
		t.Fatalf("%s PCM bytes = %d, want %d", path, len(pcm), wantPCMBytes)
	}
	return pcm
}

func padPCM16ToFrames(t *testing.T, pcm []byte) []byte {
	t.Helper()
	frameBytes := FrameSamples * 2
	if len(pcm)%2 != 0 {
		t.Fatalf("PCM byte length is odd: %d", len(pcm))
	}
	pad := (frameBytes - len(pcm)%frameBytes) % frameBytes
	out := append([]byte(nil), pcm...)
	if pad != 0 {
		out = append(out, make([]byte, pad)...)
	}
	return out
}

func encodeBCG729ForPagesGolden(t *testing.T, pcm []byte) []byte {
	t.Helper()
	bin := "third-party/bcg729-blackbox/bcg729_encode"
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("bcg729 black-box executable unavailable at %s: %v", bin, err)
	}
	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(pcm)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bcg729 black-box encode failed: %v: %s", err, string(stderr.Bytes()))
	}
	want := len(pcm) / (FrameSamples * 2) * FrameBytes
	if len(out) != want {
		t.Fatalf("bcg729 black-box encoded bytes = %d, want %d", len(out), want)
	}
	return out
}

func encodePCM16LEForGolden(t *testing.T, pcm []byte) []byte {
	t.Helper()
	if len(pcm)%(FrameSamples*2) != 0 {
		t.Fatalf("PCM byte length %d is not frame-aligned", len(pcm))
	}
	enc := NewEncoder()
	out := make([]byte, 0, len(pcm)/(FrameSamples*2)*FrameBytes)
	framePCM := make([]int16, FrameSamples)
	var frameBits [FrameBytes]byte
	for off := 0; off < len(pcm); off += FrameSamples * 2 {
		for i := range framePCM {
			sampleOff := off + i*2
			framePCM[i] = int16(binary.LittleEndian.Uint16(pcm[sampleOff:]))
		}
		if err := enc.EncodeFrame(framePCM, frameBits[:]); err != nil {
			t.Fatalf("EncodeFrame frame %d: %v", off/(FrameSamples*2), err)
		}
		out = append(out, frameBits[:]...)
	}
	return out
}

func writeFFmpegDecodedG729WAVFixture(t *testing.T, path string, payload []byte) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatalf("ffmpeg unavailable for Pages audio golden refresh: %v", err)
	}
	tmp := t.TempDir()
	payloadPath := tmp + "/input.g729"
	if err := os.WriteFile(payloadPath, payload, 0o600); err != nil {
		t.Fatalf("write temporary G.729 payload: %v", err)
	}
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "g729",
		"-i", payloadPath,
		"-ar", "8000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg decode %s: %v: %s", path, err, string(out))
	}
}

func decodeG729ForGolden(t *testing.T, payload []byte) []byte {
	t.Helper()
	if len(payload)%FrameBytes != 0 {
		t.Fatalf("G.729 payload byte length %d is not divisible by %d", len(payload), FrameBytes)
	}
	dec := NewDecoder()
	out := make([]byte, 0, len(payload)/FrameBytes*FrameSamples*2)
	framePCM := make([]int16, FrameSamples)
	var pair [2]byte
	for off := 0; off < len(payload); off += FrameBytes {
		if err := dec.DecodeFrame(payload[off:off+FrameBytes], framePCM); err != nil {
			t.Fatalf("DecodeFrame frame %d: %v", off/FrameBytes, err)
		}
		for _, sample := range framePCM {
			binary.LittleEndian.PutUint16(pair[:], uint16(sample))
			out = append(out, pair[:]...)
		}
	}
	return out
}

func readGoldenFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func writePCM16WAVFixture(t *testing.T, path string, pcm []byte) {
	t.Helper()
	if len(pcm)%2 != 0 {
		t.Fatalf("PCM byte length is odd: %d", len(pcm))
	}
	dataBytes := uint32(len(pcm))
	riffBytes := uint32(36 + len(pcm))
	byteRate := uint32(SampleRate * 2)
	blockAlign := uint16(2)

	out := make([]byte, 44+len(pcm))
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], riffBytes)
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], 1)
	binary.LittleEndian.PutUint32(out[24:28], SampleRate)
	binary.LittleEndian.PutUint32(out[28:32], byteRate)
	binary.LittleEndian.PutUint16(out[32:34], blockAlign)
	binary.LittleEndian.PutUint16(out[34:36], 16)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], dataBytes)
	copy(out[44:], pcm)

	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func byteMismatch(got, want []byte) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return fmt.Sprintf("first diff at byte %d: got 0x%02x want 0x%02x (len got=%d want=%d)",
				i, got[i], want[i], len(got), len(want))
		}
	}
	return fmt.Sprintf("length mismatch: got=%d want=%d", len(got), len(want))
}
