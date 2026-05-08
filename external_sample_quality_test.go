package g729

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExternalSampleQualityDiagnostic is an opt-in harness for user-provided
// problem audio. It converts WAV/MP3/etc. with the local ffmpeg executable
// only as a black-box media converter, then compares:
//   - input -> our encoder -> FFmpeg decode
//   - input -> our encoder -> local decoder
//   - local decoder vs FFmpeg decode of the exact same G.729 payload
//
// Usage:
//
//	G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav go test -run TestExternalSampleQualityDiagnostic -count=1 -v
//
// Raw .pcm/.raw/.sln/.s16le/.in files are treated as 8 kHz mono signed
// little-endian int16 PCM. Other extensions are decoded/resampled to that
// format by ffmpeg as an executable process; no FFmpeg source is inspected.
func TestExternalSampleQualityDiagnostic(t *testing.T) {
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	originalSamples := len(src)
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, "sample.g729")
	ffPath := filepath.Join(tmp, "sample.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, rawPath)
	ffmpegDecodeRawG729(t, rawPath, ffPath)

	raw := readFile(t, rawPath)
	ff := s16leToSamples(readFile(t, ffPath))
	local := decodeRawG729WithLocal(t, raw)

	ref := src[:originalSamples]
	if len(ff) > originalSamples {
		ff = ff[:originalSamples]
	}
	if len(local) > originalSamples {
		local = local[:originalSamples]
	}
	const maxShift = 240
	shFF, gFF, sFF := bestAlignedSNR(ref, ff, maxShift)
	shLocal, gLocal, sLocal := bestAlignedSNR(ref, local, maxShift)
	shLocalVsFF, gLocalVsFF, sLocalVsFF := bestAlignedSNR(ff, local, maxShift)

	t.Logf("external sample quality diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d encodedBytes=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples, len(raw))
	t.Logf("%-34s %6s %10s %10s %10s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR")
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "input -> our encoder -> ffmpeg", shFF, rmsAmp(ff), gFF, sFF)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "input -> our encoder -> local", shLocal, rmsAmp(local), gLocal, sLocal)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "local decoder vs ffmpeg", shLocalVsFF, rmsAmp(local), gLocalVsFF, sLocalVsFF)
}

func externalSampleQualityPath() string {
	if path := strings.TrimSpace(os.Getenv("G729_EXTERNAL_SAMPLE_QUALITY")); path != "" {
		return path
	}
	for _, path := range []string{
		"testdata/external/user_quality_input.wav",
		"testdata/external/user_quality_input.mp3",
		"testdata/external/user_quality_input.pcm",
		"testdata/external/user_quality_input.raw",
		"testdata/external/user_quality_input.sln",
		"testdata/external/user_quality_input.s16le",
		"testdata/external/user_quality_input.in",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func readExternalQualitySamples(t *testing.T, path string) []int16 {
	t.Helper()
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pcm", ".raw", ".sln", ".s16le", ".in":
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read raw sample %s: %v", path, err)
		}
		if len(data)%2 != 0 {
			t.Fatalf("raw sample %s has odd byte length %d; expected 8 kHz mono signed little-endian int16", path, len(data))
		}
		return s16leToSamples(data)
	default:
		cmd := exec.Command(
			"ffmpeg",
			"-hide_banner",
			"-loglevel", "error",
			"-i", path,
			"-f", "s16le",
			"-acodec", "pcm_s16le",
			"-ar", "8000",
			"-ac", "1",
			"pipe:1",
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			t.Fatalf("ffmpeg convert %s: %s", path, msg)
		}
		if len(out)%2 != 0 {
			t.Fatalf("ffmpeg produced odd byte count %d for %s", len(out), path)
		}
		return s16leToSamples(out)
	}
}
