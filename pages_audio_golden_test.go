package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
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
	wantDecoded, decodedFormat := readPCM16WAVFixture(t, "docs/assets/audio/g729-encode-g729-decode.wav")
	if decodedFormat.sampleRate != SampleRate || decodedFormat.channels != 1 || decodedFormat.bitsPerSample != 16 {
		t.Fatalf("decoded WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			decodedFormat.sampleRate, decodedFormat.channels, decodedFormat.bitsPerSample)
	}
	if !bytes.Equal(decoded, wantDecoded) {
		t.Fatalf("Pages decoder golden mismatch: %s", byteMismatch(decoded, wantDecoded))
	}
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
