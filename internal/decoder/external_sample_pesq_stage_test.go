package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestExternalSampleDecoderStagePESQDiagnostic is an opt-in decoder-only PESQ
// localizer for private user samples. It keeps the clean-room boundary: bcg729
// and FFmpeg are used only as black-box executables, and only numeric output is
// logged.
func TestExternalSampleDecoderStagePESQDiagnostic(t *testing.T) {
	if os.Getenv("G729_DECODER_EXTERNAL_PESQ_STAGE") != "1" {
		t.Skip("set G729_DECODER_EXTERNAL_PESQ_STAGE=1 to run external-sample decoder PESQ stage localization")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	python := externalPESQPython(t)
	samplePath := externalPESQSamplePath(t)

	src := externalPESQReadSample(t, samplePath)
	if len(src) < frameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", samplePath, len(src))
	}
	originalSamples := len(src)
	if rem := len(src) % frameSamples; rem != 0 {
		src = append(src, make([]int16, frameSamples-rem)...)
	}
	frames := len(src) / frameSamples

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	externalPESQWriteBCG729Raw(t, src, bcgRawPath)
	raw, err := os.ReadFile(bcgRawPath)
	if err != nil {
		t.Fatalf("read bcg729 raw payload: %v", err)
	}
	if got, want := len(raw), frames*bitstream.FrameBytes; got != want {
		t.Fatalf("bcg729 raw payload length=%d want %d", got, want)
	}

	ffmpeg := phase3uFFmpegDecodeRaw(t, bcgRawPath, frames, "external-pesq-stage-bcg")
	stages := blackboxDecodeRawStages(t, raw, frames)
	noPostfilter := blackboxDecodeRawNoPostfilter(t, raw, frames)
	enhanced := phase3rDecodeRawEnhanced(t, bcgRawPath, frames)
	preHPBlend25 := externalPESQDecodeRawPostfilterBlend(t, raw, frames, 1, 4)
	preHPBlend50 := externalPESQDecodeRawPostfilterBlend(t, raw, frames, 1, 2)
	preHPBlend75 := externalPESQDecodeRawPostfilterBlend(t, raw, frames, 3, 4)

	variants := []blackboxVariant{
		{name: "bcg729_ffmpeg_anchor", samples: ffmpeg, note: "bcg729 black-box encode -> FFmpeg black-box decode"},
		{name: "production", samples: stages.production, note: "local Decode output: synth -> postfilter -> HP -> final output gain recovery"},
		{name: "enhanced_envelope", samples: enhanced, note: "experimental DecodeFrameEnhanced envelope path"},
		{name: "no_postfilter_hp_x2", samples: noPostfilter, note: "structural postfilter bypass: synth -> HP -> x2"},
		{name: "pre_hp_blend_no_pf_25", samples: preHPBlend25, note: "HP-input blend: 75% postfilter + 25% pre-postfilter synth"},
		{name: "pre_hp_blend_no_pf_50", samples: preHPBlend50, note: "HP-input blend: 50% postfilter + 50% pre-postfilter synth"},
		{name: "pre_hp_blend_no_pf_75", samples: preHPBlend75, note: "HP-input blend: 25% postfilter + 75% pre-postfilter synth"},
		{name: "blend_no_pf_25", samples: externalPESQBlendSamples(stages.production, noPostfilter, 1, 4), note: "final-output blend: 75% production + 25% no-postfilter"},
		{name: "blend_no_pf_50", samples: externalPESQBlendSamples(stages.production, noPostfilter, 1, 2), note: "final-output blend: 50% production + 50% no-postfilter"},
		{name: "blend_no_pf_75", samples: externalPESQBlendSamples(stages.production, noPostfilter, 3, 4), note: "final-output blend: 25% production + 75% no-postfilter"},
		{name: "pf_shortterm_x2_no_tilt", samples: stages.pfShortTermX2, note: "postfilter through short-term stage, tilt+AGC+HP bypass proxy"},
		{name: "pf_tilt_x2_no_agc", samples: stages.pfTiltX2, note: "postfilter through tilt stage, AGC+HP bypass proxy"},
		{name: "postfilter_x2_no_hp", samples: stages.postfilterX2, note: "postfilter output scaled x2, HP bypass proxy"},
		{name: "synth_x2_no_pf_hp", samples: stages.synthX2, note: "synthesis output scaled x2, postfilter+HP bypass proxy"},
		{name: "hp_pre_scale", samples: stages.hpPreScale, note: "HP output before final gain recovery"},
		{name: "production_best_scale_src", samples: scaleInt16(stages.production, leastSquaresScale(src, stages.production)), note: "production with least-squares scale vs source"},
		{name: "production_best_scale_ff", samples: scaleInt16(stages.production, leastSquaresScale(ffmpeg, stages.production)), note: "production with least-squares scale vs FFmpeg decode"},
	}

	type row struct {
		name       string
		pesq       float64
		delta      float64
		srcMetric  blackboxMetrics
		ffMetric   blackboxMetrics
		rmsRatio   float64
		nearClip   int
		exactClip  int
		samplePeak int
		note       string
	}

	rows := make([]row, 0, len(variants))
	anchorPESQ := math.NaN()
	srcRMS := diag4Rms(src)
	for i, variant := range variants {
		score := externalPESQNBScore(t, python, tmp, "stage-"+strconv.Itoa(i), src, variant.samples)
		if i == 0 {
			anchorPESQ = score
		}
		rmsRatio := math.NaN()
		if srcRMS > 0 {
			rmsRatio = diag4Rms(variant.samples) / srcRMS
		}
		rows = append(rows, row{
			name:       variant.name,
			pesq:       score,
			delta:      score - anchorPESQ,
			srcMetric:  blackboxMeasure(src, variant.samples, 80),
			ffMetric:   blackboxMeasure(ffmpeg, variant.samples, 80),
			rmsRatio:   rmsRatio,
			nearClip:   externalPESQNearClipCount(variant.samples),
			exactClip:  externalPESQExactClipCount(variant.samples),
			samplePeak: diag4MaxAbs(variant.samples),
			note:       variant.note,
		})
	}

	t.Logf("external decoder stage PESQ diagnostic: sample=%s originalSamples=%d paddedSamples=%d frames=%d", samplePath, originalSamples, len(src), frames)
	t.Logf("PESQ reference is the converted 8 kHz mono source. Anchor is bcg729 black-box encode -> FFmpeg black-box decode.")
	t.Logf("%-28s %8s %9s %9s %8s %8s %8s %9s %9s %8s %8s %9s",
		"variant", "pesq", "delta", "srcSNR", "srcCorr", "srcLag", "ffSNR", "ffCorr", "ffLag", "rms/ref", "peak", "nearClip")
	t.Logf("%-28s %8s %9s %9s %8s %8s %8s %9s %9s %8s %8s %9s",
		"-------", "----", "-----", "------", "-------", "------", "-----", "------", "-----", "-------", "----", "--------")
	for _, r := range rows {
		t.Logf("%-28s %8.4f %+9.4f %9.2f %8.4f %8d %8.2f %9.4f %9d %8.4f %8d %9d",
			r.name, r.pesq, r.delta,
			r.srcMetric.bestSNR, r.srcMetric.bestCorr, r.srcMetric.bestSNRLag,
			r.ffMetric.bestSNR, r.ffMetric.bestCorr, r.ffMetric.bestSNRLag,
			r.rmsRatio, r.samplePeak, r.nearClip)
	}
	t.Logf("")
	t.Logf("=== variant notes ===")
	for _, r := range rows {
		t.Logf("%-28s %s", r.name, r.note)
	}
	if len(rows) >= 2 {
		t.Logf("")
		t.Logf("decoder PESQ gap on bcg729 payload: production-local minus FFmpeg anchor = %+0.4f", rows[1].delta)
		t.Logf("target decoder gap: >= -0.05 to -0.10 PESQ before moving to encoder-only gap")
	}
}

func externalPESQSamplePath(t *testing.T) string {
	t.Helper()
	var candidates []string
	if env := strings.TrimSpace(os.Getenv("G729_EXTERNAL_SAMPLE_QUALITY")); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates,
		filepath.Join("..", "..", "testdata", "external", "user_quality_audio.m4a"),
		filepath.Join("..", "..", "testdata", "external", "user_quality_input.m4a"),
		filepath.Join("testdata", "external", "user_quality_audio.m4a"),
		filepath.Join("testdata", "external", "user_quality_input.m4a"),
	)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if !filepath.IsAbs(candidate) {
			rel := filepath.Join("..", "..", candidate)
			if _, err := os.Stat(rel); err == nil {
				return rel
			}
		}
	}
	t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input, or add testdata/external/user_quality_audio.m4a")
	return ""
}

func externalPESQReadSample(t *testing.T, path string) []int16 {
	t.Helper()
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pcm", ".raw", ".sln", ".s16le", ".in":
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read raw sample %s: %v", path, err)
		}
		if len(data)%2 != 0 {
			t.Fatalf("raw sample %s has odd byte length %d", path, len(data))
		}
		return externalPESQS16LEToSamples(data)
	default:
		cmd := exec.Command("ffmpeg",
			"-hide_banner", "-loglevel", "error",
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
		return externalPESQS16LEToSamples(out)
	}
}

func externalPESQWriteBCG729Raw(t *testing.T, samples []int16, path string) {
	t.Helper()
	if len(samples)%frameSamples != 0 {
		t.Fatalf("bcg729 black-box encode input has %d samples, want multiple of %d", len(samples), frameSamples)
	}
	bin := strings.TrimSpace(os.Getenv("BCG729_ENCODER"))
	if bin == "" {
		bin = strings.TrimSpace(os.Getenv("G729_BCG729_ENCODER"))
	}
	candidates := []string{bin}
	if bin == "" {
		candidates = candidates[:0]
	}
	candidates = append(candidates,
		filepath.Join("..", "..", "third-party", "bcg729-blackbox", "bcg729_encode"),
		filepath.Join("third-party", "bcg729-blackbox", "bcg729_encode"),
	)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			bin = candidate
			break
		}
	}
	if bin == "" {
		t.Skip("bcg729 black-box executable unavailable; set BCG729_ENCODER or G729_BCG729_ENCODER")
	}

	pcm := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(pcm[2*i:2*i+2], uint16(sample))
	}
	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(pcm)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		t.Fatalf("bcg729 black-box encode: %s", msg)
	}
	want := len(samples) / frameSamples * bitstream.FrameBytes
	if len(out) != want {
		t.Fatalf("bcg729 black-box encoded %d bytes, want %d", len(out), want)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func externalPESQDecodeRawPostfilterBlend(t *testing.T, raw []byte, frames int, synthNum, den int) []int16 {
	t.Helper()
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		if err := dec.DecodePostfilterBlend(raw[start:start+bitstream.FrameBytes], false, out[frame*frameSamples:(frame+1)*frameSamples], synthNum, den); err != nil {
			t.Fatalf("postfilter blend decode frame %d: %v", frame, err)
		}
	}
	return out
}

func externalPESQPython(t *testing.T) string {
	t.Helper()
	python := strings.TrimSpace(os.Getenv("G729_PESQ_PYTHON"))
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			t.Skipf("python3 unavailable for PESQ diagnostic: %v", err)
		}
	}
	if out, err := exec.Command(python, "-c", "import numpy, pesq").CombinedOutput(); err != nil {
		t.Skipf("PESQ Python modules unavailable for %s: %v: %s", python, err, strings.TrimSpace(string(out)))
	}
	return python
}

func externalPESQNBScore(t *testing.T, python, tmp, name string, ref, deg []int16) float64 {
	t.Helper()
	refPath := filepath.Join(tmp, name+".ref.wav")
	degPath := filepath.Join(tmp, name+".deg.wav")
	if err := os.WriteFile(refPath, externalPESQWAVBytes(ref), 0o600); err != nil {
		t.Fatalf("write PESQ ref WAV %s: %v", refPath, err)
	}
	if err := os.WriteFile(degPath, externalPESQWAVBytes(deg), 0o600); err != nil {
		t.Fatalf("write PESQ deg WAV %s: %v", degPath, err)
	}
	out, err := exec.Command(python, "-c", externalPESQNBPythonScript, refPath, degPath).CombinedOutput()
	if err != nil {
		t.Fatalf("PESQ NB failed for %s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
		t.Fatalf("invalid PESQ NB output %q for %s: %v", strings.TrimSpace(string(out)), name, err)
	}
	return score
}

const externalPESQNBPythonScript = `
import sys
import wave

import numpy as np
from pesq import pesq

def read_wav(path):
    with wave.open(path, "rb") as f:
        if f.getnchannels() != 1 or f.getsampwidth() != 2:
            raise ValueError("expected mono 16-bit WAV")
        rate = f.getframerate()
        data = f.readframes(f.getnframes())
    return rate, np.frombuffer(data, dtype="<i2").astype(np.float32)

ref_rate, ref = read_wav(sys.argv[1])
deg_rate, deg = read_wav(sys.argv[2])
n = min(ref.shape[0], deg.shape[0])
if ref_rate != deg_rate or ref_rate != 8000 or n <= 0:
    raise SystemExit("expected non-empty 8 kHz WAV pair")
print("{:.6f}".format(float(pesq(ref_rate, ref[:n], deg[:n], "nb"))))
`

func externalPESQWAVBytes(samples []int16) []byte {
	var b bytes.Buffer
	dataLen := uint32(len(samples) * 2)
	riffLen := uint32(36) + dataLen
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, riffLen)
	b.WriteString("WAVE")
	b.WriteString("fmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(8000))
	_ = binary.Write(&b, binary.LittleEndian, uint32(16000))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, dataLen)
	for _, sample := range samples {
		_ = binary.Write(&b, binary.LittleEndian, sample)
	}
	return b.Bytes()
}

func externalPESQS16LEToSamples(data []byte) []int16 {
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i : 2*i+2]))
	}
	return out
}

func externalPESQNearClipCount(samples []int16) int {
	count := 0
	for _, sample := range samples {
		if sample >= 32760 || sample <= -32760 {
			count++
		}
	}
	return count
}

func externalPESQExactClipCount(samples []int16) int {
	count := 0
	for _, sample := range samples {
		if sample == 32767 || sample == -32768 {
			count++
		}
	}
	return count
}

func externalPESQBlendSamples(a, b []int16, bNum, den int) []int16 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]int16, n)
	aNum := den - bNum
	for i := 0; i < n; i++ {
		out[i] = externalPESQBlendValue(a[i], b[i], aNum, bNum, den)
	}
	return out
}

func externalPESQBlendValue(a, b int16, aNum, bNum, den int) int16 {
	v := (int64(a)*int64(aNum) + int64(b)*int64(bNum) + int64(den/2)) / int64(den)
	if v > 32767 {
		v = 32767
	} else if v < -32768 {
		v = -32768
	}
	return int16(v)
}
