package g729

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/lpc"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	pitchidx "github.com/hunydev/g729/internal/pitch"
	clpitch "github.com/hunydev/g729/internal/pitch/closedloop"
	olpitch "github.com/hunydev/g729/internal/pitch/openloop"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// encoderDiagnosticBoundedGainPredictorTuning is a test-only no-op tuning bit.
// It makes qualityHeuristicsEnabled true while none of the named heuristic
// predicates match, isolating the bounded PredictedGcQ12/Reconstruct path from
// the core profile's default wide predictor path.
const encoderDiagnosticBoundedGainPredictorTuning encoderQualityTuning = 1 << 31

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
// little-endian int16 PCM. Other extensions, including .m4a, are
// decoded/resampled to that format by ffmpeg as an executable process; no
// FFmpeg source is inspected.
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
	ffMetrics := externalQualityMetricsFor(ref, ff, maxShift)
	localMetrics := externalQualityMetricsFor(ref, local, maxShift)
	localVsFFMetrics := externalQualityMetricsFor(ff, local, maxShift)

	t.Logf("external sample quality diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d encodedBytes=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples, len(raw))
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ffMetrics)
	logExternalQualityMetrics(t, "input -> our encoder -> local", localMetrics)
	logExternalQualityMetrics(t, "local decoder vs ffmpeg", localVsFFMetrics)
}

func TestExternalSamplePESQMatrixDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PESQ_MATRIX") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PESQ_MATRIX=1 to compute the four-path PESQ NB baseline")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourFFmpegPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgFFmpegPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")

	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourFFmpegPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgFFmpegPath)

	ourRaw := readFile(t, ourRawPath)
	bcgRaw := readFile(t, bcgRawPath)
	ourLocal := decodeRawG729WithLocal(t, ourRaw)
	ourBlend50 := decodeRawG729WithLocalPostfilterBlend(t, ourRaw, 1, 2)
	ourEnhanced := decodeRawG729WithLocalEnhanced(t, ourRaw)
	ourFFmpeg := s16leToSamples(readFile(t, ourFFmpegPath))
	bcgLocal := decodeRawG729WithLocal(t, bcgRaw)
	bcgBlend50 := decodeRawG729WithLocalPostfilterBlend(t, bcgRaw, 1, 2)
	bcgEnhanced := decodeRawG729WithLocalEnhanced(t, bcgRaw)
	bcgFFmpeg := s16leToSamples(readFile(t, bcgFFmpegPath))

	rows := []struct {
		path    string
		samples []int16
		metrics externalQualityMetrics
		pesq    float64
	}{
		{path: "our encode -> local decode", samples: ourLocal, metrics: externalQualityMetricsFor(src, ourLocal, 240)},
		{path: "our encode -> blend50 local decode", samples: ourBlend50, metrics: externalQualityMetricsFor(src, ourBlend50, 240)},
		{path: "our encode -> enhanced local decode", samples: ourEnhanced, metrics: externalQualityMetricsFor(src, ourEnhanced, 240)},
		{path: "our encode -> ffmpeg decode", samples: ourFFmpeg, metrics: externalQualityMetricsFor(src, ourFFmpeg, 240)},
		{path: "bcg729 encode -> local decode", samples: bcgLocal, metrics: externalQualityMetricsFor(src, bcgLocal, 240)},
		{path: "bcg729 encode -> blend50 local decode", samples: bcgBlend50, metrics: externalQualityMetricsFor(src, bcgBlend50, 240)},
		{path: "bcg729 encode -> enhanced local decode", samples: bcgEnhanced, metrics: externalQualityMetricsFor(src, bcgEnhanced, 240)},
		{path: "bcg729 encode -> ffmpeg decode", samples: bcgFFmpeg, metrics: externalQualityMetricsFor(src, bcgFFmpeg, 240)},
	}
	for i := range rows {
		rows[i].pesq = pesqNBScore(t, tmp, fmt.Sprintf("pesq-%02d", i), src, rows[i].samples)
	}

	t.Logf("external sample PESQ NB matrix: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-34s,%8s,%10s,%10s,%8s,%8s,%7s,%8s", "Path", "PESQ_NB", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, row := range rows {
		t.Logf("%-34s,%8.4f,%10.2f,%10.2f,%8.4f,%8.4f,%7d,%8d",
			row.path,
			row.pesq,
			row.metrics.globalSNR,
			row.metrics.segSNR,
			row.metrics.corr,
			row.metrics.rmsRatio,
			row.metrics.peak,
			row.metrics.nearClip)
	}
	scoreByPath := make(map[string]float64, len(rows))
	for _, row := range rows {
		scoreByPath[row.path] = row.pesq
	}
	bcgFFmpegScore := scoreByPath["bcg729 encode -> ffmpeg decode"]
	t.Logf("decoder gap on bcg729 payload: local-vs-ffmpeg PESQ delta %+0.4f", scoreByPath["bcg729 encode -> local decode"]-bcgFFmpegScore)
	t.Logf("blend50 decoder gap on bcg729 payload: blend50-vs-ffmpeg PESQ delta %+0.4f", scoreByPath["bcg729 encode -> blend50 local decode"]-bcgFFmpegScore)
	t.Logf("enhanced decoder gap on bcg729 payload: enhanced-vs-ffmpeg PESQ delta %+0.4f", scoreByPath["bcg729 encode -> enhanced local decode"]-bcgFFmpegScore)
	t.Logf("encoder gap under ffmpeg decode: our-vs-bcg729 PESQ delta %+0.4f", scoreByPath["our encode -> ffmpeg decode"]-bcgFFmpegScore)
	t.Logf("end-to-end gap: our local-vs-bcg729 ffmpeg PESQ delta %+0.4f", scoreByPath["our encode -> local decode"]-bcgFFmpegScore)
	t.Logf("blend50 end-to-end gap: our blend50-vs-bcg729 ffmpeg PESQ delta %+0.4f", scoreByPath["our encode -> blend50 local decode"]-bcgFFmpegScore)
	t.Logf("enhanced end-to-end gap: our enhanced-vs-bcg729 ffmpeg PESQ delta %+0.4f", scoreByPath["our encode -> enhanced local decode"]-bcgFFmpegScore)
}

func TestExternalSamplePostfilterBlendSweepDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BLEND_SWEEP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BLEND_SWEEP=1 to sweep postfilter-blend decoder ratios")
	}
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
	type payloadCase struct {
		name string
		raw  []byte
		ff   []int16
	}
	writePayload := func(name string, write func(string)) payloadCase {
		rawPath := filepath.Join(tmp, name+".g729")
		pcmPath := filepath.Join(tmp, name+".ffmpeg.s16le")
		write(rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		return payloadCase{
			name: name,
			raw:  readFile(t, rawPath),
			ff:   s16leToSamples(readFile(t, pcmPath)),
		}
	}
	cases := []payloadCase{
		writePayload("our", func(path string) { writeOurEncodedRawG729(t, src, path) }),
		writePayload("bcg729", func(path string) { writeBCGEncodedRawG729(t, src, path) }),
	}

	t.Logf("external postfilter-blend sweep: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-8s,%10s,%8s,%10s,%10s,%8s,%8s,%7s,%8s", "Payload", "SynthShare", "PESQ_NB", "DeltaFF", "GlobalSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, tc := range cases {
		ffPESQ := pesqNBScore(t, tmp, tc.name+"-ffmpeg", src, tc.ff)
		ffMetrics := externalQualityMetricsFor(src, tc.ff, 240)
		t.Logf("%-8s,%10s,%8.4f,%10s,%10.2f,%8.4f,%8.4f,%7d,%8d",
			tc.name, "ffmpeg", ffPESQ, "anchor",
			ffMetrics.globalSNR, ffMetrics.corr, ffMetrics.rmsRatio, ffMetrics.peak, ffMetrics.nearClip)
		for synthNum := 0; synthNum <= 8; synthNum++ {
			decoded := decodeRawG729WithLocalPostfilterBlend(t, tc.raw, synthNum, 8)
			score := pesqNBScore(t, tmp, fmt.Sprintf("%s-blend-%02d", tc.name, synthNum), src, decoded)
			metrics := externalQualityMetricsFor(src, decoded, 240)
			t.Logf("%-8s,%10.3f,%8.4f,%+10.4f,%10.2f,%8.4f,%8.4f,%7d,%8d",
				tc.name, float64(synthNum)/8.0, score, score-ffPESQ,
				metrics.globalSNR, metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)
		}
	}
}

func TestExternalSampleProfileCompareDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_COMPARE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_COMPARE=1 to compare EncoderProfileCore and EncoderProfileQuality")
	}
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
	ref := src[:originalSamples]

	type result struct {
		name      string
		raw       []byte
		ff        []int16
		local     []int16
		ffm       externalQualityMetrics
		localm    externalQualityMetrics
		ffPESQ    float64
		localPESQ float64
		payloadEq float64
	}
	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
		{name: "clean-snr", profile: EncoderProfileQualityCleanSNR},
		{name: "clean-smooth", profile: EncoderProfileQualityCleanSmooth},
		{name: "clean-voiced", profile: EncoderProfileQualityCleanVoiced},
		{name: "clean-degrit", profile: EncoderProfileQualityCleanDegrit},
		{name: "clean-harmonic", profile: EncoderProfileQualityCleanHarmonic},
		{name: "clean-harmonic-strong", profile: EncoderProfileQualityCleanHarmonicStrong},
		{name: "clean-harmonic-deep", profile: EncoderProfileQualityCleanHarmonicDeep},
		{name: "clean-fcb", profile: EncoderProfileQualityCleanFCBRerank},
	}

	tmp := t.TempDir()
	withPESQ := os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_COMPARE_PESQ") == "1" ||
		strings.TrimSpace(os.Getenv("G729_PESQ_PYTHON")) != ""
	results := make([]result, 0, len(profiles))
	var bcgRaw []byte
	bcgFFPESQ := math.NaN()
	if withPESQ {
		rawPath := filepath.Join(tmp, "bcg729.g729")
		pcmPath := filepath.Join(tmp, "bcg729.ffmpeg.s16le")
		writeBCGEncodedRawG729(t, src, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		ff := s16leToSamples(readFile(t, pcmPath))
		raw := readFile(t, rawPath)
		local := decodeRawG729WithLocal(t, raw)
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(local) > originalSamples {
			local = local[:originalSamples]
		}
		if len(ff) < originalSamples || len(local) < originalSamples {
			t.Fatalf("bcg729 decoded output too short: ffmpeg=%d local=%d want >= %d", len(ff), len(local), originalSamples)
		}
		ffPESQ := pesqNBScore(t, tmp, "profile-bcg729-ffmpeg", ref, ff)
		localPESQ := pesqNBScore(t, tmp, "profile-bcg729-local", ref, local)
		bcgRaw = raw
		bcgFFPESQ = ffPESQ
		results = append(results, result{
			name:      "bcg729",
			raw:       raw,
			ff:        ff,
			local:     local,
			ffm:       externalQualityMetricsFor(ref, ff, 240),
			localm:    externalQualityMetricsFor(ref, local, 240),
			ffPESQ:    ffPESQ,
			localPESQ: localPESQ,
			payloadEq: 100,
		})
	}
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		raw := readFile(t, rawPath)
		ff := s16leToSamples(readFile(t, pcmPath))
		local := decodeRawG729WithLocal(t, raw)
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(local) > originalSamples {
			local = local[:originalSamples]
		}
		if len(ff) < originalSamples || len(local) < originalSamples {
			t.Fatalf("%s decoded output too short: ffmpeg=%d local=%d want >= %d", p.name, len(ff), len(local), originalSamples)
		}
		ffPESQ := math.NaN()
		localPESQ := math.NaN()
		payloadEq := math.NaN()
		if withPESQ {
			ffPESQ = pesqNBScore(t, tmp, "profile-"+p.name+"-ffmpeg", ref, ff)
			localPESQ = pesqNBScore(t, tmp, "profile-"+p.name+"-local", ref, local)
			if len(bcgRaw) > 0 {
				payloadEq = payloadEqualPercent(raw, bcgRaw)
			}
		}
		results = append(results, result{
			name:      p.name,
			raw:       raw,
			ff:        ff,
			local:     local,
			ffm:       externalQualityMetricsFor(ref, ff, 240),
			localm:    externalQualityMetricsFor(ref, local, 240),
			ffPESQ:    ffPESQ,
			localPESQ: localPESQ,
			payloadEq: payloadEq,
		})
	}

	t.Logf("external sample profile compare diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-22s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, r := range results {
		logExternalQualityMetrics(t, r.name+" -> ffmpeg", r.ffm)
		logExternalQualityMetrics(t, r.name+" -> local", r.localm)
		ffNearFrames := externalNearClipFrames(r.ff, r.ffm.shift, len(src)/FrameSamples, 32700)
		localNearFrames := externalNearClipFrames(r.local, r.localm.shift, len(src)/FrameSamples, 32700)
		if len(ffNearFrames) > 0 || len(localNearFrames) > 0 {
			t.Logf("%s near-clip frames: ffmpeg=%v local=%v", r.name, ffNearFrames, localNearFrames)
		}
	}
	if withPESQ {
		t.Logf("")
		t.Logf("PESQ reference is the converted 8 kHz mono source. bcg729 is black-box encode; FFmpeg is black-box decode.")
		t.Logf("%-24s %10s %10s %12s %12s %11s", "Profile", "LocalPESQ", "FFPESQ", "LocalΔBCG", "FFΔBCG", "PayloadEq")
		t.Logf("%-24s %10s %10s %12s %12s %11s", "-------", "---------", "------", "---------", "------", "---------")
		for _, r := range results {
			localDelta := math.NaN()
			ffDelta := math.NaN()
			if !math.IsNaN(bcgFFPESQ) {
				localDelta = r.localPESQ - bcgFFPESQ
				ffDelta = r.ffPESQ - bcgFFPESQ
			}
			t.Logf("%-24s %10.4f %10.4f %+12.4f %+12.4f %10.2f%%",
				r.name, r.localPESQ, r.ffPESQ, localDelta, ffDelta, r.payloadEq)
		}
	}
	if len(results) == 2 {
		equal := payloadEqualPercent(results[0].raw, results[1].raw)
		coreVsQualityFF := externalQualityMetricsFor(results[0].ff, results[1].ff, 240)
		coreVsQualityLocal := externalQualityMetricsFor(results[0].local, results[1].local, 240)
		t.Logf("core-vs-quality payload byte equality %.2f%%", equal)
		logExternalQualityMetrics(t, "quality ffmpeg vs core ffmpeg", coreVsQualityFF)
		logExternalQualityMetrics(t, "quality local vs core local", coreVsQualityLocal)
	}
}

func TestExternalSampleQualityTuningAblationDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_TUNING_ABLATION") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_TUNING_ABLATION=1 to split quality tuning effects")
	}
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
	ref := src[:originalSamples]

	type result struct {
		name  string
		raw   []byte
		ff    []int16
		local []int16
		ffm   externalQualityMetrics
	}
	variants := []struct {
		name   string
		tuning encoderQualityTuning
	}{
		{name: "core", tuning: 0},
		{name: "core-wide-flag", tuning: encoderTuningWideGainPredictor},
		{name: "core-bounded-pred", tuning: encoderDiagnosticBoundedGainPredictorTuning},
		{name: "pitch", tuning: encoderTuningPitchCenterCandidate},
		{name: "fcb", tuning: encoderTuningFCBThresholdScan},
		{name: "fcb+wide", tuning: encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "gain", tuning: encoderTuningGainSearchBias},
		{name: "gain+wide", tuning: encoderTuningGainSearchBias | encoderTuningWideGainPredictor},
		{name: "nativegain", tuning: encoderTuningNativeGainSearch},
		{name: "nativegain+gainclip", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair},
		{name: "gainclip+fcbrerank", tuning: encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "nativegain+gainclip+mse", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair},
		{name: "nativegain+gainclip+fcbrerank", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "nativegain+gainclip+mse+noise+fcbrerank", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair | encoderTuningGainNoiseRepair | encoderTuningFCBNoiseRerank},
		{name: "early", tuning: encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "early+wide", tuning: encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningWideGainPredictor},
		{name: "norm", tuning: encoderTuningNormalizedAdaptivePitchSearch},
		{name: "norm+wide", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningWideGainPredictor},
		{name: "norm+nativegain", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch},
		{name: "norm+nativegain+gainclip", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair},
		{name: "norm+nativegain+gainclip+fcbrerank", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "norm+nativegain+pitchclip", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningPitchClipRepair},
		{name: "norm+nativegain+gainclip+mse", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair},
		{name: "norm+nativegain+gainclip+mse+noise", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair | encoderTuningGainNoiseRepair},
		{name: "norm+early+wide", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningWideGainPredictor},
		{name: "residacb", tuning: encoderTuningResidualExtensionAdaptiveVector},
		{name: "pitch+fcb", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan},
		{name: "pitch+fcb+wide", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "pitch+gain", tuning: encoderTuningPitchCenterCandidate | encoderTuningGainSearchBias},
		{name: "fcb+gain", tuning: encoderTuningFCBThresholdScan | encoderTuningGainSearchBias},
		{name: "gain+early", tuning: encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "norm+gain", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias},
		{name: "norm+gain+wide", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningWideGainPredictor},
		{name: "norm+gain+early", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "norm+gain+early+wide", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningWideGainPredictor},
		{name: "norm+gain+early+residacb", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningResidualExtensionAdaptiveVector},
		{name: "pitch+fcb+gain", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningGainSearchBias},
		{name: "pitch+fcb+gain+norm", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningGainSearchBias | encoderTuningNormalizedAdaptivePitchSearch},
		{name: "quality+pitch", tuning: encoderQualityTuningAll | encoderTuningPitchCenterCandidate},
		{name: "quality-no-fcb", tuning: encoderQualityTuningAll &^ encoderTuningFCBThresholdScan},
		{name: "quality-no-gain", tuning: encoderQualityTuningAll &^ encoderTuningGainSearchBias},
		{name: "quality-no-early", tuning: encoderQualityTuningAll &^ encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "quality-no-norm", tuning: encoderQualityTuningAll &^ encoderTuningNormalizedAdaptivePitchSearch},
		{name: "quality-no-clip", tuning: encoderQualityTuningAll &^ encoderTuningGainClipRepair},
		{name: "quality-no-mse", tuning: encoderQualityTuningAll &^ encoderTuningGainMSERepair},
		{name: "quality-no-mse-no-noise", tuning: (encoderQualityTuningAll &^ encoderTuningGainMSERepair) &^ encoderTuningGainNoiseRepair},
		{name: "quality-no-noise", tuning: encoderQualityTuningAll &^ encoderTuningGainNoiseRepair},
		{name: "quality+lspx", tuning: encoderQualityTuningAll | encoderTuningExpandedLSPSearch},
		{name: "quality-wide-no-gain", tuning: (encoderQualityTuningAll &^ encoderTuningGainSearchBias) | encoderTuningWideGainPredictor},
		{name: "quality-wide+gain", tuning: encoderQualityTuningAll | encoderTuningWideGainPredictor},
		{name: "quality+nativegain", tuning: encoderQualityTuningAll | encoderTuningNativeGainSearch},
		{name: "quality", tuning: encoderQualityTuningAll},
		{name: "quality+residacb", tuning: encoderQualityTuningAll | encoderTuningResidualExtensionAdaptiveVector},
	}

	tmp := t.TempDir()
	results := make([]result, 0, len(variants))
	for _, v := range variants {
		rawPath := filepath.Join(tmp, strings.ReplaceAll(v.name, "+", "_")+".g729")
		pcmPath := filepath.Join(tmp, strings.ReplaceAll(v.name, "+", "_")+".ffmpeg.s16le")
		writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, v.tuning)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		raw := readFile(t, rawPath)
		ff := s16leToSamples(readFile(t, pcmPath))
		local := decodeRawG729WithLocal(t, raw)
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(local) > originalSamples {
			local = local[:originalSamples]
		}
		if len(ff) < originalSamples || len(local) < originalSamples {
			t.Fatalf("%s decoded output too short: ffmpeg=%d local=%d want >= %d", v.name, len(ff), len(local), originalSamples)
		}
		results = append(results, result{
			name:  v.name,
			raw:   raw,
			ff:    ff,
			local: local,
			ffm:   externalQualityMetricsFor(ref, ff, 240),
		})
	}

	t.Logf("external sample quality tuning ablation diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-22s %6s %10s %10s %10s %8s %8s %7s %8s %8s %8s %7s %10s %10s",
		"Variant", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "HighDB", "WorstDB", "WorstF", "eqCore", "eqQuality")
	coreRaw := results[0].raw
	var qualityRaw []byte
	for _, r := range results {
		if r.name == "quality" {
			qualityRaw = r.raw
			break
		}
	}
	if qualityRaw == nil {
		t.Fatal("quality variant missing from ablation results")
	}
	for _, r := range results {
		noise := externalResidualNoiseMetricsFor(ref, r.ff, r.ffm.shift)
		t.Logf("%-22s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %8.2f %8.2f %7d %9.2f%% %9.2f%%",
			r.name,
			r.ffm.shift,
			r.ffm.rms,
			r.ffm.globalSNR,
			r.ffm.segSNR,
			r.ffm.corr,
			r.ffm.rmsRatio,
			r.ffm.peak,
			r.ffm.nearClip,
			noise.highDB,
			noise.worstHighDB,
			noise.worstFrame,
			payloadEqualPercent(coreRaw, r.raw),
			payloadEqualPercent(qualityRaw, r.raw),
		)
	}
}

func TestExternalSampleEncoderCandidatePESQDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 to compare focused encoder candidates with PESQ")
	}
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
	ref := src[:originalSamples]

	type candidate struct {
		name    string
		profile EncoderProfile
		tuning  encoderQualityTuning
		bcg     bool
	}
	candidates := []candidate{
		{name: "bcg729", bcg: true},
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean-fcb", profile: EncoderProfileQualityCleanFCBRerank},
		{name: "pesq", profile: EncoderProfileQualityPESQ},
		{name: "core+gainclip+fcbrerank", profile: EncoderProfileCore, tuning: encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "core+nativegain+gainclip+fcbrerank", profile: EncoderProfileCore, tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "core+nativegain+gainclip+mse+noise+fcbrerank", profile: EncoderProfileCore, tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair | encoderTuningGainNoiseRepair | encoderTuningFCBNoiseRerank},
		{name: "core+norm+nativegain+gainclip+fcbrerank", profile: EncoderProfileCore, tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank},
		{name: "quality+fcbrerank", profile: EncoderProfileCore, tuning: encoderQualityTuningAll | encoderTuningFCBNoiseRerank},
		{name: "quality-no-norm+fcbrerank", profile: EncoderProfileCore, tuning: (encoderQualityTuningAll &^ encoderTuningNormalizedAdaptivePitchSearch) | encoderTuningFCBNoiseRerank},
	}

	tmp := t.TempDir()
	type row struct {
		name      string
		raw       []byte
		localPESQ float64
		ffPESQ    float64
		localm    externalQualityMetrics
		ffm       externalQualityMetrics
	}
	rows := make([]row, 0, len(candidates))
	var bcgRaw []byte
	bcgFFPESQ := math.NaN()
	for _, c := range candidates {
		base := sanitizeExternalSampleName(c.name)
		rawPath := filepath.Join(tmp, base+".g729")
		pcmPath := filepath.Join(tmp, base+".ffmpeg.s16le")
		if c.bcg {
			writeBCGEncodedRawG729(t, src, rawPath)
		} else if c.tuning != 0 {
			writeOurEncodedRawG729WithTuning(t, src, rawPath, c.profile, c.tuning)
		} else {
			writeOurEncodedRawG729WithProfile(t, src, rawPath, c.profile)
		}
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		raw := readFile(t, rawPath)
		ff := s16leToSamples(readFile(t, pcmPath))
		local := decodeRawG729WithLocal(t, raw)
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(local) > originalSamples {
			local = local[:originalSamples]
		}
		if len(ff) < originalSamples || len(local) < originalSamples {
			t.Fatalf("%s decoded output too short: ffmpeg=%d local=%d want >= %d", c.name, len(ff), len(local), originalSamples)
		}
		ffPESQ := pesqNBScore(t, tmp, base+"-ffmpeg", ref, ff)
		localPESQ := pesqNBScore(t, tmp, base+"-local", ref, local)
		if c.bcg {
			bcgRaw = raw
			bcgFFPESQ = ffPESQ
		}
		rows = append(rows, row{
			name:      c.name,
			raw:       raw,
			localPESQ: localPESQ,
			ffPESQ:    ffPESQ,
			localm:    externalQualityMetricsFor(ref, local, 240),
			ffm:       externalQualityMetricsFor(ref, ff, 240),
		})
	}

	t.Logf("external sample focused encoder PESQ diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-48s %10s %10s %12s %12s %10s %8s %8s", "Candidate", "LocalPESQ", "FFPESQ", "LocalΔBCG", "FFΔBCG", "PayloadEq", "LocClip", "FFClip")
	t.Logf("%-48s %10s %10s %12s %12s %10s %8s %8s", "---------", "---------", "------", "---------", "------", "---------", "-------", "------")
	for _, r := range rows {
		localDelta := math.NaN()
		ffDelta := math.NaN()
		if !math.IsNaN(bcgFFPESQ) {
			localDelta = r.localPESQ - bcgFFPESQ
			ffDelta = r.ffPESQ - bcgFFPESQ
		}
		payloadEq := math.NaN()
		if len(bcgRaw) > 0 {
			payloadEq = payloadEqualPercent(r.raw, bcgRaw)
		}
		t.Logf("%-48s %10.4f %10.4f %+12.4f %+12.4f %9.2f%% %8d %8d",
			r.name, r.localPESQ, r.ffPESQ, localDelta, ffDelta, payloadEq, r.localm.nearClip, r.ffm.nearClip)
	}
}

func TestExternalSampleQualityWindowDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_QUALITY_WINDOW") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY_WINDOW=1 to compare quality variants in a focused frame window")
	}
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
	ref := src[:originalSamples]
	startFrame, endFrame := externalQualityWindowFrameRange()

	variants := []struct {
		name   string
		tuning encoderQualityTuning
		bcg    bool
	}{
		{name: "bcg729-blackbox", bcg: true},
		{name: "core", tuning: 0},
		{name: "nativegain+gainclip+mse", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair},
		{name: "norm+nativegain+gainclip+mse", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair},
		{name: "quality-no-fcb", tuning: encoderQualityTuningAll &^ encoderTuningFCBThresholdScan},
		{name: "quality+lspx", tuning: encoderQualityTuningAll | encoderTuningExpandedLSPSearch},
		{name: "quality", tuning: encoderQualityTuningAll},
		{name: "quality+residacb", tuning: encoderQualityTuningAll | encoderTuningResidualExtensionAdaptiveVector},
	}

	tmp := t.TempDir()
	t.Logf("external sample quality focused-window diagnostic: %s", path)
	t.Logf("window frames=%d..%d (%.3fs..%.3fs at 8 kHz)", startFrame, endFrame,
		float64(startFrame*FrameSamples)/8000.0, float64((endFrame+1)*FrameSamples)/8000.0)
	t.Logf("%-30s %6s %8s %8s %8s %8s %8s %7s %8s %9s %8s %8s %7s %8s",
		"Variant", "shift", "gSNR", "segSNR", "corr", "RMS/ref", "wSNR", "wCorr", "wRMS/ref", "wMSE", "wPeak", "wNear", "Peak", "NearClip")
	for _, v := range variants {
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(v.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(v.name)+".ffmpeg.s16le")
		if v.bcg {
			writeBCGEncodedRawG729(t, src, rawPath)
		} else {
			writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, v.tuning)
		}
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s decoded output too short: got %d want >= %d", v.name, len(decoded), originalSamples)
		}
		full := externalQualityMetricsFor(ref, decoded, 240)
		window := externalAlignedWindowQualityMetrics(ref, decoded, full.shift, startFrame, endFrame, 32700)
		t.Logf("%-30s %6d %8.2f %8.2f %8.4f %8.4f %8.2f %7.4f %8.4f %9.0f %8d %8d %7d %8d",
			v.name, full.shift, full.globalSNR, full.segSNR, full.corr, full.rmsRatio,
			window.snr, window.corr, window.rmsRatio, window.mse, window.peak, window.nearClip,
			full.peak, full.nearClip)
	}
}

func externalQualityWindowFrameRange() (startFrame, endFrame int) {
	startFrame, endFrame = 286, 312
	if spec := os.Getenv("G729_EXTERNAL_SAMPLE_QUALITY_WINDOW_FRAMES"); spec != "" {
		var start, end int
		if n, err := fmt.Sscanf(spec, "%d:%d", &start, &end); n == 2 && err == nil {
			startFrame, endFrame = start, end
		}
	}
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame < startFrame {
		endFrame = startFrame
	}
	return startFrame, endFrame
}

func TestExternalSampleOpenLoopTopVariantDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_TOP_VARIANT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_OPENLOOP_TOP_VARIANT=1 to compare diagnostic open-loop T_op choices")
	}
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
	ref := src[:originalSamples]

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}
	variants := []externalOpenLoopTopVariant{
		{name: "production"},
		{name: "range1", mode: "range1"},
		{name: "range2", mode: "range2"},
		{name: "range3", mode: "range3"},
		{name: "best-range", mode: "best-range"},
		{name: "no-high", mode: "no-high"},
		{name: "best>=1.03", mode: "best-margin:1.03"},
		{name: "best>=1.08", mode: "best-margin:1.08"},
		{name: "low>=0.90", mode: "low-close:0.90"},
		{name: "low>=0.95", mode: "low-close:0.95"},
		{name: "r2>=0.90", mode: "range2-close:0.90"},
		{name: "r2>=0.95", mode: "range2-close:0.95"},
		{name: "r2-if-best", mode: "range2-if-best"},
		{name: "cont-high0.95", mode: "continuity-high:0.95"},
		{name: "cont-high1.00", mode: "continuity-high:1.00"},
	}

	tmp := t.TempDir()
	t.Logf("external sample open-loop T_op variant diagnostic: %s", path)
	t.Logf("%-8s %-12s %6s %10s %10s %10s %8s %8s %7s %8s %8s %8s %7s %10s %10s %8s %8s %8s %7s",
		"Profile", "Variant", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip",
		"HighDB", "WorstDB", "WorstF", "LocalSNR", "LocalSeg", "LocalNC", "LHighDB", "LWorstDB", "LWorstF")
	for _, p := range profiles {
		for _, v := range variants {
			frames := encodeBitstreamFramesOpenLoopTopVariant(t, src, p.profile, v)
			name := p.name + "-" + v.name
			ff, local, ffDecoded, localDecoded := measureExternalSampleFramesQualityPairWithAudio(t, tmp, name, frames, ref, originalSamples)
			ffNoise := externalResidualNoiseMetricsFor(ref, ffDecoded, ff.shift)
			localNoise := externalResidualNoiseMetricsFor(ref, localDecoded, local.shift)
			t.Logf("%-8s %-12s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %8.2f %8.2f %7d %10.2f %10.2f %8d %8.2f %8.2f %7d",
				p.name, v.name,
				ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
				ff.peak, ff.nearClip, ffNoise.highDB, ffNoise.worstHighDB, ffNoise.worstFrame,
				local.globalSNR, local.segSNR, local.nearClip, localNoise.highDB, localNoise.worstHighDB, localNoise.worstFrame)
		}
	}
}

func TestExternalSampleQualityVariantDeltaDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_VARIANT_DELTA") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_VARIANT_DELTA=1 to compare quality variants against bcg729 black-box decoded output")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	prevGuard := qualityNormalizedPitchHarmonicGuardRatio
	prevMSEThreshold := qualityGainMSERepairThreshold
	prevHighMSEBetterNum := qualityGainNoiseRepairHighMSEBetterMSEToleranceNum
	prevHighMSEBetterDen := qualityGainNoiseRepairHighMSEBetterMSEToleranceDen
	prevMSEBetterHighNum := qualityGainNoiseRepairMSEBetterHighMSEToleranceNum
	prevMSEBetterHighDen := qualityGainNoiseRepairMSEBetterHighMSEToleranceDen
	defer func() {
		qualityNormalizedPitchHarmonicGuardRatio = prevGuard
		qualityGainMSERepairThreshold = prevMSEThreshold
		qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
		qualityGainNoiseRepairHighMSEBetterMSEToleranceDen = prevHighMSEBetterDen
		qualityGainNoiseRepairMSEBetterHighMSEToleranceNum = prevMSEBetterHighNum
		qualityGainNoiseRepairMSEBetterHighMSEToleranceDen = prevMSEBetterHighDen
	}()

	type variant struct {
		name   string
		frames func() []bitstream.Frame
	}
	variants := []variant{
		{
			name: "quality",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "quality+residacb",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll|encoderTuningResidualExtensionAdaptiveVector)
			},
		},
		{
			name: "quality+normol",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll|encoderTuningNormalizedOpenLoopSearch)
			},
		},
		{
			name: "quality-no-norm",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "quality-no-mse",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningGainMSERepair)
			},
		},
		{
			name: "quality-no-mse-no-noise",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, (encoderQualityTuningAll&^encoderTuningGainMSERepair)&^encoderTuningGainNoiseRepair)
			},
		},
		{
			name: "quality-clean",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithProfile(t, src, EncoderProfileQualityClean)
			},
		},
		{
			name: "clean-no-native",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, (encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)&^encoderTuningNativeGainSearch)
			},
		},
		{
			name: "clean-no-noise",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, (encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)&^encoderTuningGainNoiseRepair)
			},
		},
		{
			name: "clean-annex-lsp",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, (encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)&^encoderTuningExpandedLSPSearch)
			},
		},
		{
			name: "clean-core-gain",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderTuningGainClipRepair|encoderTuningGainMSERepair|encoderTuningGainNoiseRepair)
			},
		},
		{
			name: "clean-core-gain-lspx",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderTuningGainClipRepair|encoderTuningGainMSERepair|encoderTuningGainNoiseRepair|encoderTuningExpandedLSPSearch)
			},
		},
		{
			name: "clean-high10",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 110
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high12",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 112
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high15",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 115
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high18",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 118
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high20",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 120
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high15-mse24",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 24000
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 115
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high20-mse24",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 24000
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 120
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "clean-high50",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = qualityCleanGainMSERepairThreshold
				qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = 150
				defer func() {
					qualityGainMSERepairThreshold = prevMSEThreshold
					qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
				}()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "no-norm-mse26800",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 26800
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "no-norm-mse26000",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 26000
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "no-norm-mse23800",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23800
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "no-norm-mse23600",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23600
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "no-norm-mse23200",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23200
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll&^encoderTuningNormalizedAdaptivePitchSearch)
			},
		},
		{
			name: "gain-mse26800",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 26800
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "gain-mse26000",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 26000
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "gain-mse23800",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23800
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "gain-mse23600",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23600
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "gain-mse23200",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 23200
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "gain-mse28000",
			frames: func() []bitstream.Frame {
				qualityGainMSERepairThreshold = 28000
				defer func() { qualityGainMSERepairThreshold = prevMSEThreshold }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "open-range2",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "range2"})
			},
		},
		{
			name: "open-no-high",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "no-high"})
			},
		},
		{
			name: "open-best1.03",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "best-margin:1.03"})
			},
		},
		{
			name: "open-r2close0.90",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "range2-close:0.90"})
			},
		},
		{
			name: "open-cont0.95",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "continuity-high:0.95"})
			},
		},
		{
			name: "open-cont1.00",
			frames: func() []bitstream.Frame {
				return encodeBitstreamFramesOpenLoopTopVariant(t, src, EncoderProfileQuality, externalOpenLoopTopVariant{mode: "continuity-high:1.00"})
			},
		},
		{
			name: "pitch-guard1.05",
			frames: func() []bitstream.Frame {
				qualityNormalizedPitchHarmonicGuardRatio = 1.05
				defer func() { qualityNormalizedPitchHarmonicGuardRatio = prevGuard }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
		{
			name: "pitch-guard1.25",
			frames: func() []bitstream.Frame {
				qualityNormalizedPitchHarmonicGuardRatio = 1.25
				defer func() { qualityNormalizedPitchHarmonicGuardRatio = prevGuard }()
				return encodeBitstreamFramesWithQualityTuning(t, src, encoderQualityTuningAll)
			},
		},
	}

	t.Logf("external sample quality variant delta diagnostic: %s", path)
	t.Logf("%-18s %8s %8s %8s %7s %7s %5s %8s %8s %8s %7s %7s %8s %8s %8s",
		"Variant", "SrcSNR", "SrcHigh", "SrcWorst", "SrcWF", "Peak", "NC", "DeltaSNR", "DeltaHigh", "DeltaWorst", "DeltaWF", "DeltaLag", "DLocalSNR", "DLocalHi", "DLocalWorst")
	for _, v := range variants {
		qualityNormalizedPitchHarmonicGuardRatio = prevGuard
		qualityGainMSERepairThreshold = prevMSEThreshold
		qualityGainNoiseRepairHighMSEBetterMSEToleranceNum = prevHighMSEBetterNum
		qualityGainNoiseRepairHighMSEBetterMSEToleranceDen = prevHighMSEBetterDen
		qualityGainNoiseRepairMSEBetterHighMSEToleranceNum = prevMSEBetterHighNum
		qualityGainNoiseRepairMSEBetterHighMSEToleranceDen = prevMSEBetterHighDen
		ff, _, ffDecoded, localDecoded := measureExternalSampleFramesQualityPairWithAudio(t, tmp, v.name, v.frames(), ref, originalSamples)
		srcNoise := externalResidualNoiseMetricsFor(ref, ffDecoded, ff.shift)
		delta := externalQualityMetricsFor(bcgDecoded, ffDecoded, 240)
		deltaNoise := externalResidualNoiseMetricsFor(bcgDecoded, ffDecoded, delta.shift)
		localDelta := externalQualityMetricsFor(bcgDecoded, localDecoded, 240)
		localDeltaNoise := externalResidualNoiseMetricsFor(bcgDecoded, localDecoded, localDelta.shift)
		t.Logf("%-18s %8.2f %8.2f %8.2f %7d %7d %5d %8.2f %8.2f %8.2f %7d %7d %8.2f %8.2f %8.2f",
			v.name,
			ff.globalSNR, srcNoise.highDB, srcNoise.worstHighDB, srcNoise.worstFrame, ff.peak, ff.nearClip,
			delta.globalSNR, deltaNoise.highDB, deltaNoise.worstHighDB, deltaNoise.worstFrame, delta.shift,
			localDelta.globalSNR, localDeltaNoise.highDB, localDeltaNoise.worstHighDB)
	}
}

func TestExternalSampleProfileBitstreamSummaryDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_BITSTREAM_SUMMARY") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_BITSTREAM_SUMMARY=1 to compare actual profile bitstream fields against bcg729 black-box")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
		{name: "clean-snr", profile: EncoderProfileQualityCleanSNR},
		{name: "clean-smooth", profile: EncoderProfileQualityCleanSmooth},
		{name: "clean-voiced", profile: EncoderProfileQualityCleanVoiced},
		{name: "clean-degrit", profile: EncoderProfileQualityCleanDegrit},
		{name: "clean-harmonic", profile: EncoderProfileQualityCleanHarmonic},
		{name: "clean-harmonic-strong", profile: EncoderProfileQualityCleanHarmonicStrong},
		{name: "clean-harmonic-deep", profile: EncoderProfileQualityCleanHarmonicDeep},
		{name: "clean-fcb", profile: EncoderProfileQualityCleanFCBRerank},
		{name: "core", profile: EncoderProfileCore},
	}

	t.Logf("external sample profile bitstream summary: %s", path)
	t.Logf("%-12s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %8s %8s %8s",
		"Profile", "FrameEq", "LSPEq", "PitchEq", "PIntEq", "CodeEq", "SignEq", "GainEq", "GpEq", "HighP", "LowP", "MultP", "MeanGpD", "MeanGdD", "MeanTD")
	for _, p := range profiles {
		frames := encodeBitstreamFramesWithProfile(t, src, p.profile)
		stats := externalBitstreamSummaryAgainstBCG(frames, bcgFrames)
		t.Logf("%-12s %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %7.2f %8.1f %8.1f %8.1f",
			p.name,
			percent(stats.sameFrame, stats.frames),
			percent(stats.sameLSP, stats.frames),
			percent(stats.samePitch, stats.subframes),
			percent(stats.samePitchInt, stats.subframes),
			percent(stats.sameCode, stats.subframes),
			percent(stats.sameSign, stats.subframes),
			percent(stats.sameGain, stats.subframes),
			percent(stats.sameGp, stats.subframes),
			percent(stats.higherPitch, stats.subframes),
			percent(stats.lowerPitch, stats.subframes),
			percent(stats.multiplePitch, stats.subframes),
			stats.meanGpDelta(),
			stats.meanGammaDelta(),
			stats.meanPitchDelta())
	}
}

type externalBitstreamSummaryStats struct {
	frames    int
	subframes int

	sameFrame    int
	sameLSP      int
	samePitch    int
	samePitchInt int
	sameCode     int
	sameSign     int
	sameGain     int
	sameGp       int

	higherPitch   int
	lowerPitch    int
	multiplePitch int

	gpDeltaSum    int64
	gammaDeltaSum int64
	pitchDeltaSum int64
}

func (s externalBitstreamSummaryStats) meanGpDelta() float64 {
	if s.subframes == 0 {
		return 0
	}
	return float64(s.gpDeltaSum) / float64(s.subframes)
}

func (s externalBitstreamSummaryStats) meanGammaDelta() float64 {
	if s.subframes == 0 {
		return 0
	}
	return float64(s.gammaDeltaSum) / float64(s.subframes)
}

func (s externalBitstreamSummaryStats) meanPitchDelta() float64 {
	if s.subframes == 0 {
		return 0
	}
	return float64(s.pitchDeltaSum) / float64(s.subframes)
}

func externalBitstreamSummaryAgainstBCG(frames, bcgFrames []bitstream.Frame) externalBitstreamSummaryStats {
	n := len(frames)
	if len(bcgFrames) < n {
		n = len(bcgFrames)
	}
	var stats externalBitstreamSummaryStats
	stats.frames = n
	stats.subframes = n * 2
	for i := 0; i < n; i++ {
		f := frames[i]
		b := bcgFrames[i]
		if f == b {
			stats.sameFrame++
		}
		if f.L0 == b.L0 && f.L1 == b.L1 && f.L2 == b.L2 && f.L3 == b.L3 {
			stats.sameLSP++
		}
		for sub := 0; sub < 2; sub++ {
			t, frac, code, signs, ga, gb := externalSubframeFields(frames, i, sub)
			bt, bfrac, bcode, bsigns, bga, bgb := externalSubframeFields(bcgFrames, i, sub)
			if t == bt && frac == bfrac {
				stats.samePitch++
			}
			if t == bt {
				stats.samePitchInt++
			}
			if code == bcode {
				stats.sameCode++
			}
			if signs == bsigns {
				stats.sameSign++
			}
			if ga == bga && gb == bgb {
				stats.sameGain++
			}
			gp := externalGainGpQ14(uint8(ga), uint8(gb))
			bgp := externalGainGpQ14(uint8(bga), uint8(bgb))
			if gp == bgp {
				stats.sameGp++
			}
			gamma := externalGainGammaQ13(uint8(ga), uint8(gb))
			bgamma := externalGainGammaQ13(uint8(bga), uint8(bgb))
			stats.gpDeltaSum += int64(gp) - int64(bgp)
			stats.gammaDeltaSum += int64(gamma) - int64(bgamma)
			stats.pitchDeltaSum += int64(t - bt)
			if t > bt {
				stats.higherPitch++
				if isHigherPitchMultipleCandidate(bt, t) {
					stats.multiplePitch++
				}
			} else if t < bt {
				stats.lowerPitch++
			}
		}
	}
	return stats
}

func TestExternalSampleOpenLoopASourceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_A_SOURCE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_OPENLOOP_A_SOURCE=1 to compare open-loop LP coefficient sources")
	}
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
	ref := src[:originalSamples]

	type mode struct {
		name       string
		useUnquant bool
		normalized bool
	}
	modes := []mode{
		{name: "core-production"},
		{name: "unquant-openloop", useUnquant: true},
		{name: "unquant-openloop-norm", useUnquant: true, normalized: true},
	}

	tmp := t.TempDir()
	t.Logf("external sample open-loop A-source diagnostic: %s", path)
	t.Logf("%-22s %6s %10s %10s %10s %8s %8s %7s %8s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "chgTop")
	for _, m := range modes {
		frames, changedTop := encodeBitstreamFramesCoreOpenLoopASource(t, src, m.useUnquant, m.normalized)
		ff, _ := measureExternalSampleFramesQualityPair(t, tmp, m.name, frames, ref, originalSamples)
		t.Logf("%-22s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %8d",
			m.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, changedTop)
	}
}

func TestExternalSampleSpeechWindowDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_SPEECH_WINDOW") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_SPEECH_WINDOW=1 to compare open-loop/closed-loop speech windows")
	}
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
	ref := src[:originalSamples]

	type mode struct {
		name           string
		openBase       int
		closeBase      int
		normalizedOpen bool
	}
	modes := []mode{
		{name: "prod", openBase: -1, closeBase: -1},
		{name: "ol120-cl80", openBase: -1, closeBase: 80},
		{name: "ol80-cl120", openBase: 80, closeBase: -1},
		{name: "ol80-cl80", openBase: 80, closeBase: 80},
		{name: "ol160-cl120", openBase: 160, closeBase: -1},
		{name: "ol160-cl160", openBase: 160, closeBase: 160},
		{name: "ol80n-cl80", openBase: 80, closeBase: 80, normalizedOpen: true},
		{name: "ol120n-cl80", openBase: -1, closeBase: 80, normalizedOpen: true},
	}

	tmp := t.TempDir()
	t.Logf("external sample speech-window diagnostic: %s", path)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "chgTop")
	for _, m := range modes {
		frames, changedTop := encodeBitstreamFramesCoreSpeechWindows(t, src, m.openBase, m.closeBase, m.normalizedOpen)
		ff, _ := measureExternalSampleFramesQualityPair(t, tmp, m.name, frames, ref, originalSamples)
		t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %8d",
			m.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, changedTop)
	}
}

func TestExternalSampleClosedLoopXBPrecisionDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_XB_PRECISION") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_XB_PRECISION=1 to compare closed-loop xb precision variants")
	}
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
	ref := src[:originalSamples]

	modes := []struct {
		name    string
		variant string
	}{
		{name: "core"},
		{name: "core-xbrnd", variant: "round"},
		{name: "core-xb32", variant: "q12"},
	}

	tmp := t.TempDir()
	t.Logf("external sample closed-loop xb precision diagnostic: %s", path)
	t.Logf("%-10s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		var frames []bitstream.Frame
		switch mode.variant {
		case "q12":
			frames = encodeBitstreamFramesCoreClosedLoopXB32(t, src)
		case "round":
			frames = encodeBitstreamFramesCoreClosedLoopXBRound(t, src)
		default:
			frames = encodeBitstreamFramesWithProfile(t, src, EncoderProfileCore)
		}
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-10s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			mode.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleOpenLoopProductionTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_PRODUCTION_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_OPENLOOP_PRODUCTION_TRACE=1 to trace production open-loop candidates")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	startFrame, endFrame := 286, 292
	if spec := os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_TRACE_FRAMES"); spec != "" {
		var start, end int
		if n, err := fmt.Sscanf(spec, "%d:%d", &start, &end); n == 2 && err == nil {
			startFrame, endFrame = start, end
		}
	}
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame < startFrame {
		endFrame = startFrame
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}

	t.Logf("external sample production open-loop trace: %s frames=%d:%d", path, startFrame, endFrame)
	t.Logf("%-8s %6s %7s %7s %8s %8s %8s %8s %8s %8s %7s %7s %7s %7s",
		"Profile", "frame", "prod", "norm", "pR1", "pR2", "pR3", "nR1", "nR2", "nR3", "locT1", "bcgT1", "locT2", "bcgT2")
	for _, p := range profiles {
		enc := NewEncoderWithProfile(p.profile)
		for off := 0; off+FrameSamples <= len(src); off += FrameSamples {
			frameIndex := off / FrameSamples
			if _, err := enc.lpcStep(src[off : off+FrameSamples]); err != nil {
				t.Fatalf("%s lpcStep frame %d: %v", p.name, frameIndex, err)
			}
			prod, norm := externalOpenLoopProductionResults(enc)
			_ = enc.openloopStep()
			_, _ = enc.closedloopStep(0)
			_, _ = enc.closedloopStep(1)

			if frameIndex < startFrame || frameIndex > endFrame {
				continue
			}
			bcgT1, bcgF1, _, _, _, _ := externalSubframeFields(bcgFrames, frameIndex, 0)
			bcgT2, bcgF2, _, _, _, _ := externalSubframeFields(bcgFrames, frameIndex, 1)
			pr1, pr2, pr3 := externalOpenLoopResultRel(prod)
			nr1, nr2, nr3 := externalOpenLoopResultRel(norm)
			t.Logf("%-8s %6d %7d %7d %8.3f %8.3f %8.3f %8.3f %8.3f %8.3f %7s %7s %7s %7s",
				p.name, frameIndex, prod.Top, norm.Top,
				pr1, pr2, pr3, nr1, nr2, nr3,
				fmt.Sprintf("%d/%d", enc.intT1, enc.frac1),
				fmt.Sprintf("%d/%d", bcgT1, bcgF1),
				fmt.Sprintf("%d/%d", enc.intT2, enc.frac2),
				fmt.Sprintf("%d/%d", bcgT2, bcgF2))
		}
	}
}

func TestExternalSampleOpenLoopMergeDeltaDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_MERGE_DELTA") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_OPENLOOP_MERGE_DELTA=1 to compare core open-loop merge variants")
	}
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
	ref := src[:originalSamples]

	currentFrames := encodeBitstreamFramesWithProfile(t, src, EncoderProfileCore)
	pairwiseFrames, changedTop := encodeBitstreamFramesCoreOpenLoopPairwise(t, src)

	tmp := t.TempDir()
	currentFF, currentLocal := measureExternalSampleFramesQualityPair(t, tmp, "core-current", currentFrames, ref, originalSamples)
	pairwiseFF, pairwiseLocal := measureExternalSampleFramesQualityPair(t, tmp, "core-pairwise", pairwiseFrames, ref, originalSamples)
	currentDecoded := decodeExternalFramesWithFFmpeg(t, tmp, "core-current-frames", currentFrames, originalSamples)
	pairwiseDecoded := decodeExternalFramesWithFFmpeg(t, tmp, "core-pairwise-frames", pairwiseFrames, originalSamples)

	t.Logf("external sample open-loop merge delta diagnostic: %s", path)
	t.Logf("changed open-loop top: %d/%d frames", changedTop, len(src)/FrameSamples)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"current", currentFF.shift, currentFF.rms, currentFF.globalSNR, currentFF.segSNR, currentFF.corr, currentFF.rmsRatio,
		currentFF.peak, currentFF.nearClip, currentLocal.globalSNR, currentLocal.segSNR, currentLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"pairwise", pairwiseFF.shift, pairwiseFF.rms, pairwiseFF.globalSNR, pairwiseFF.segSNR, pairwiseFF.corr, pairwiseFF.rmsRatio,
		pairwiseFF.peak, pairwiseFF.nearClip, pairwiseLocal.globalSNR, pairwiseLocal.segSNR, pairwiseLocal.nearClip)
	t.Logf("current near-clip frames: %v", externalNearClipFrames(currentDecoded, currentFF.shift, len(currentFrames), 32700))
	t.Logf("pairwise near-clip frames: %v", externalNearClipFrames(pairwiseDecoded, pairwiseFF.shift, len(pairwiseFrames), 32700))
}

func TestExternalSampleOpenLoopSubmultipleLiftSweepDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_OPENLOOP_LIFT_SWEEP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_OPENLOOP_LIFT_SWEEP=1 to sweep core open-loop submultiple lift")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	lifts := []float64{1.05, 1.10, 1.12, 1.15, 20.0 / 17.0, 1.20, 1.30, 1.50, 1.75, 2.00}
	t.Logf("external sample open-loop submultiple lift sweep: %s", path)
	t.Logf("%-8s %8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Lift", "chgTop", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, lift := range lifts {
		frames, changedTop := encodeBitstreamFramesCoreOpenLoopLift(t, src, lift)
		name := fmt.Sprintf("lift%.2f", lift)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		t.Logf("%-8.2f %8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			lift, changedTop, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

type externalOpenLoopTopVariant struct {
	name string
	mode string
}

func encodeBitstreamFramesOpenLoopTopVariant(t *testing.T, samples []int16, profile EncoderProfile, variant externalOpenLoopTopVariant) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		prevPitch := enc.intT2
		diag := externalDiagnoseOpenLoopSplitFrame(enc)
		_ = enc.openloopStep()
		if variant.mode != "" {
			enc.tOp = externalOpenLoopTopFromVariant(diag, variant.mode, prevPitch)
		}
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func externalOpenLoopTopFromVariant(diag openLoopFrameDiag, mode string, prevPitch int16) int16 {
	current := int16(oracleMergeDiag(diag))
	switch mode {
	case "range1":
		return int16(diag.range1.lag)
	case "range2":
		return int16(diag.range2.lag)
	case "range3":
		return int16(diag.range3.lag)
	case "best-range":
		best := diag.range1
		if externalOpenLoopScore(diag.range2) > externalOpenLoopScore(best) {
			best = diag.range2
		}
		if externalOpenLoopScore(diag.range3) > externalOpenLoopScore(best) {
			best = diag.range3
		}
		return int16(best.lag)
	case "no-high":
		return int16(oracleMergeNoHighDiag(diag))
	case "range2-if-best":
		best := diag.range1
		if externalOpenLoopScore(diag.range2) > externalOpenLoopScore(best) {
			best = diag.range2
		}
		if externalOpenLoopScore(diag.range3) > externalOpenLoopScore(best) {
			best = diag.range3
		}
		if best.lag == diag.range2.lag {
			return int16(diag.range2.lag)
		}
		return current
	default:
		if margin, ok := parseExternalOpenLoopVariantFloat(mode, "best-margin:"); ok {
			return externalOpenLoopBestIfMargin(diag, current, margin)
		}
		if ratio, ok := parseExternalOpenLoopVariantFloat(mode, "low-close:"); ok {
			return externalOpenLoopLowIfClose(diag, current, ratio)
		}
		if ratio, ok := parseExternalOpenLoopVariantFloat(mode, "range2-close:"); ok {
			return externalOpenLoopRange2IfClose(diag, current, ratio)
		}
		if ratio, ok := parseExternalOpenLoopVariantFloat(mode, "continuity-high:"); ok {
			return externalOpenLoopContinuityHighIfClose(diag, current, prevPitch, ratio)
		}
		return current
	}
}

func parseExternalOpenLoopVariantFloat(mode, prefix string) (float64, bool) {
	if !strings.HasPrefix(mode, prefix) {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimPrefix(mode, prefix), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func externalOpenLoopBestIfMargin(diag openLoopFrameDiag, current int16, margin float64) int16 {
	best := diag.range1
	if externalOpenLoopScore(diag.range2) > externalOpenLoopScore(best) {
		best = diag.range2
	}
	if externalOpenLoopScore(diag.range3) > externalOpenLoopScore(best) {
		best = diag.range3
	}
	currentScore := externalOpenLoopScoreForLag(diag, current)
	if currentScore <= 0 || externalOpenLoopScore(best) >= currentScore*margin {
		return int16(best.lag)
	}
	return current
}

func externalOpenLoopLowIfClose(diag openLoopFrameDiag, current int16, ratio float64) int16 {
	low := diag.range1
	if externalOpenLoopScore(diag.range2) > externalOpenLoopScore(low) {
		low = diag.range2
	}
	currentScore := externalOpenLoopScoreForLag(diag, current)
	if currentScore <= 0 || externalOpenLoopScore(low) >= currentScore*ratio {
		return int16(low.lag)
	}
	return current
}

func externalOpenLoopRange2IfClose(diag openLoopFrameDiag, current int16, ratio float64) int16 {
	currentScore := externalOpenLoopScoreForLag(diag, current)
	if currentScore <= 0 || externalOpenLoopScore(diag.range2) >= currentScore*ratio {
		return int16(diag.range2.lag)
	}
	return current
}

func externalOpenLoopContinuityHighIfClose(diag openLoopFrameDiag, current int16, prevPitch int16, ratio float64) int16 {
	if prevPitch <= 0 {
		return current
	}
	currentScore := externalOpenLoopScoreForLag(diag, current)
	if currentScore <= 0 {
		return current
	}
	best := current
	bestScore := currentScore
	for _, cand := range []openLoopRangeDiag{diag.range2, diag.range3} {
		if cand.lag <= int(current) {
			continue
		}
		if !externalOpenLoopNearMultipleWithTolerance(cand.lag, int(current), 5) {
			continue
		}
		if d := cand.lag - int(prevPitch); d < -20 || d > 20 {
			continue
		}
		score := externalOpenLoopScore(cand)
		if score >= currentScore*ratio && score > bestScore {
			best = int16(cand.lag)
			bestScore = score
		}
	}
	return best
}

func externalOpenLoopNearMultipleWithTolerance(higher, lower, tolerance int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= tolerance {
			return true
		}
		if k*lower > higher+tolerance {
			return false
		}
	}
	return false
}

func externalOpenLoopScoreForLag(diag openLoopFrameDiag, lag int16) float64 {
	switch int(lag) {
	case diag.range1.lag:
		return externalOpenLoopScore(diag.range1)
	case diag.range2.lag:
		return externalOpenLoopScore(diag.range2)
	case diag.range3.lag:
		return externalOpenLoopScore(diag.range3)
	default:
		return 0
	}
}

func TestExternalSampleFCBThresholdLimitDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FCB_THRESHOLD_LIMIT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FCB_THRESHOLD_LIMIT=1 to sweep focused FCB search limits")
	}
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
	ref := src[:originalSamples]

	prevLimit := qualityFCBThresholdScanLimit
	defer func() { qualityFCBThresholdScanLimit = prevLimit }()

	modes := []struct {
		name   string
		tuning encoderQualityTuning
	}{
		{name: "fcb+wide", tuning: encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "pitch+fcb+wide", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "quality", tuning: encoderQualityTuningAll},
	}
	limits := []int{30, 45, 60, 90, 120, 180}

	tmp := t.TempDir()
	t.Logf("external sample focused FCB threshold-limit diagnostic: %s", path)
	t.Logf("%-16s %5s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "Limit", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, limit := range limits {
		qualityFCBThresholdScanLimit = limit
		for _, mode := range modes {
			name := fmt.Sprintf("%s-l%d", strings.ReplaceAll(mode.name, "+", "_"), limit)
			rawPath := filepath.Join(tmp, name+".g729")
			pcmPath := filepath.Join(tmp, name+".ffmpeg.s16le")
			writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, mode.tuning)
			ffmpegDecodeRawG729(t, rawPath, pcmPath)
			ff := s16leToSamples(readFile(t, pcmPath))
			if len(ff) > originalSamples {
				ff = ff[:originalSamples]
			}
			if len(ff) < originalSamples {
				t.Fatalf("%s decoded output too short: ffmpeg=%d want >= %d", name, len(ff), originalSamples)
			}
			local := decodeRawG729WithLocal(t, readFile(t, rawPath))
			if len(local) > originalSamples {
				local = local[:originalSamples]
			}
			if len(local) < originalSamples {
				t.Fatalf("%s local output too short: local=%d want >= %d", name, len(local), originalSamples)
			}
			m := externalQualityMetricsFor(ref, ff, 240)
			localMetrics := externalQualityMetricsFor(ref, local, 240)
			t.Logf("%-16s %5d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
				mode.name, limit, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip,
				localMetrics.globalSNR, localMetrics.segSNR, localMetrics.nearClip)
		}
	}
}

func TestExternalSampleNativeGainFCBThresholdDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_NATIVEGAIN_FCB_LIMIT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_NATIVEGAIN_FCB_LIMIT=1 to sweep native-gain + focused FCB search")
	}
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
	ref := src[:originalSamples]
	tmp := t.TempDir()

	prevLimit := qualityFCBThresholdScanLimit
	defer func() { qualityFCBThresholdScanLimit = prevLimit }()

	type mode struct {
		name   string
		tuning encoderQualityTuning
		limit  int
	}
	modes := []mode{
		{name: "nativegain-exhaustive", tuning: encoderTuningNativeGainSearch},
		{name: "nativegain-fcb60", tuning: encoderTuningNativeGainSearch | encoderTuningFCBThresholdScan, limit: 60},
		{name: "nativegain-fcb90", tuning: encoderTuningNativeGainSearch | encoderTuningFCBThresholdScan, limit: 90},
		{name: "nativegain-fcb180", tuning: encoderTuningNativeGainSearch | encoderTuningFCBThresholdScan, limit: 180},
		{name: "nativegain-repair-exhaustive", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair},
		{name: "nativegain-repair-fcb90", tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair | encoderTuningFCBThresholdScan, limit: 90},
	}

	t.Logf("external sample native-gain FCB threshold diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-30s %6s %10s %10s %10s %8s %8s %7s %8s", "Variant", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, m := range modes {
		if m.limit > 0 {
			qualityFCBThresholdScanLimit = m.limit
		}
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".s16le")
		writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, m.tuning)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s: decoded output too short: got %d want >= %d", m.name, len(decoded), originalSamples)
		}
		q := externalQualityMetricsFor(ref, decoded, 240)
		t.Logf("%-30s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			m.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
	}
}

func TestExternalSampleGainPreselectGpClipDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_GPCLIP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_GPCLIP=1 to test gp_opt clipping in gain preselect")
	}
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
	ref := src[:originalSamples]

	modes := []externalProductionGainMode{
		{name: "core", production: true},
		{name: "preselect", search: "preselect", wide: true, thresholdScan: true},
		{name: "preselect-gpclip", search: "preselect-gpclip", wide: true, thresholdScan: true},
		{name: "preselect-native-gpclip", search: "preselect-native-gpclip", wide: true, thresholdScan: true},
		{name: "native", search: "native", wide: true, thresholdScan: true},
	}

	tmp := t.TempDir()
	t.Logf("external sample gain preselect gp_opt clip diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d gpOptClipQ14=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples, gainPreselectGpOptUpperQ14)
	t.Logf("%-26s %6s %10s %10s %10s %8s %8s %7s %8s", "Variant", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, mode := range modes {
		frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
		q := measureExternalSampleFramesQuality(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-26s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			mode.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
	}
}

func TestExternalSampleFCBThresholdEntryCountDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FCB_THRESHOLD_COUNTS") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FCB_THRESHOLD_COUNTS=1 to count focused FCB fourth-loop entries")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	prevLimit := qualityFCBThresholdScanLimit
	defer func() { qualityFCBThresholdScanLimit = prevLimit }()

	modes := []struct {
		name   string
		tuning encoderQualityTuning
	}{
		{name: "core", tuning: 0},
		{name: "fcb+wide", tuning: encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "pitch+fcb+wide", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "quality", tuning: encoderQualityTuningAll},
	}
	limits := []int{30, 45, 60, 90, 120, 180}

	t.Logf("external sample focused FCB threshold entry-count diagnostic: %s", path)
	t.Logf("%-16s %5s %6s %8s %8s %8s %8s %8s %8s %8s",
		"Mode", "Limit", "Frames", "sf0Mean", "sf1Mean", "frmMean", "sf0Hit", "sf1Hit", "frm>180", "frmMax")
	for _, limit := range limits {
		qualityFCBThresholdScanLimit = limit
		for _, mode := range modes {
			stats := collectExternalFCBThresholdEntryCounts(t, src, mode.tuning, limit)
			t.Logf("%-16s %5d %6d %8.1f %8.1f %8.1f %7.2f%% %7.2f%% %7.2f%% %8d",
				mode.name, limit, stats.frames,
				stats.meanSub0(), stats.meanSub1(), stats.meanFrame(),
				percent(stats.sub0LimitHits, stats.frames),
				percent(stats.sub1LimitHits, stats.frames),
				percent(stats.frameOver180, stats.frames),
				stats.frameMax)
		}
	}
}

func TestExternalSampleCoreFCBSubframe0LimitDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CORE_FCB_SUBFRAME0_LIMIT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CORE_FCB_SUBFRAME0_LIMIT=1 to sweep core subframe-0 FCB threshold cap")
	}
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
	ref := src[:originalSamples]

	prevCap := encoderCoreFCBThresholdScanSubframe0Limit
	defer func() { encoderCoreFCBThresholdScanSubframe0Limit = prevCap }()

	caps := []int{45, 60, 75, 90, 120, 150, 170, fcbsearch.SearchThresholdScanDefaultLimit}
	tmp := t.TempDir()
	t.Logf("external sample core FCB subframe-0 cap diagnostic: %s", path)
	t.Logf("%5s %6s %8s %8s %8s %8s %8s %6s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Cap", "Frames", "sf0Mean", "sf1Mean", "frmMean", "sf0Hit", "frm>180", "shift",
		"RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "LocalSNR", "LocalSeg", "LocalNC")
	for _, cap := range caps {
		encoderCoreFCBThresholdScanSubframe0Limit = cap
		stats := collectExternalFCBThresholdEntryCounts(t, src, 0, cap)

		name := fmt.Sprintf("core-sf0cap-%d", cap)
		rawPath := filepath.Join(tmp, name+".g729")
		pcmPath := filepath.Join(tmp, name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, EncoderProfileCore)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		ff := s16leToSamples(readFile(t, pcmPath))
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(ff) < originalSamples {
			t.Fatalf("%s decoded output too short: ffmpeg=%d want >= %d", name, len(ff), originalSamples)
		}
		local := decodeRawG729WithLocal(t, readFile(t, rawPath))
		if len(local) > originalSamples {
			local = local[:originalSamples]
		}
		if len(local) < originalSamples {
			t.Fatalf("%s local output too short: local=%d want >= %d", name, len(local), originalSamples)
		}

		m := externalQualityMetricsFor(ref, ff, 240)
		localMetrics := externalQualityMetricsFor(ref, local, 240)
		t.Logf("%5d %6d %8.1f %8.1f %8.1f %7.2f%% %7.2f%% %6d %10.0f %10.2f %8.2f %8.4f %8.4f %7d %10.2f %10.2f %8d",
			cap, stats.frames, stats.meanSub0(), stats.meanSub1(), stats.meanFrame(),
			percent(stats.sub0LimitHits, stats.frames),
			percent(stats.frameOver180, stats.frames),
			m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak,
			localMetrics.globalSNR, localMetrics.segSNR, localMetrics.nearClip)
	}
}

type externalFCBThresholdEntryStats struct {
	frames        int
	sub0Sum       int64
	sub1Sum       int64
	frameSum      int64
	frameMax      int
	sub0LimitHits int
	sub1LimitHits int
	frameOver180  int
}

func (s externalFCBThresholdEntryStats) meanSub0() float64 {
	if s.frames == 0 {
		return 0
	}
	return float64(s.sub0Sum) / float64(s.frames)
}

func (s externalFCBThresholdEntryStats) meanSub1() float64 {
	if s.frames == 0 {
		return 0
	}
	return float64(s.sub1Sum) / float64(s.frames)
}

func (s externalFCBThresholdEntryStats) meanFrame() float64 {
	if s.frames == 0 {
		return 0
	}
	return float64(s.frameSum) / float64(s.frames)
}

func collectExternalFCBThresholdEntryCounts(t *testing.T, samples []int16, tuning encoderQualityTuning, limit int) externalFCBThresholdEntryStats {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	enc.qualityTuning = tuning
	var stats externalFCBThresholdEntryStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		sub0 := enc.qualityFCBThresholdEntriesLast
		_, _ = enc.closedloopStep(1)
		sub1 := enc.qualityFCBThresholdEntriesLast
		frame := sub0 + sub1

		stats.frames++
		stats.sub0Sum += int64(sub0)
		stats.sub1Sum += int64(sub1)
		stats.frameSum += int64(frame)
		if frame > stats.frameMax {
			stats.frameMax = frame
		}
		if sub0 >= limit {
			stats.sub0LimitHits++
		}
		if sub1 >= limit {
			stats.sub1LimitHits++
		}
		if frame > 180 {
			stats.frameOver180++
		}
	}
	return stats
}

func TestExternalSampleCoreGainModeDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CORE_GAIN_MODE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CORE_GAIN_MODE=1 to compare core-path gain quantizer modes")
	}
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
	ref := src[:originalSamples]

	type mode struct {
		name                        string
		xNum, xDen, yNum, yDen      int32
		zNum, zDen, gpcNum, gpcDen  int32
		native, rawLinear, fullCost bool
	}
	modes := []mode{
		{name: "core-current", gpcNum: 1, gpcDen: 1},
		{name: "fullcost", gpcNum: 1, gpcDen: 1, fullCost: true},
		{name: "native", gpcNum: 1, gpcDen: 1, native: true},
		{name: "raw-linear", gpcNum: 1, gpcDen: 1, rawLinear: true},
		{name: "gpc-x4", gpcNum: 4, gpcDen: 1},
		{name: "xhalf-gpc4", xNum: 1, xDen: 2, gpcNum: 4, gpcDen: 1},
		{name: "quality-gain-bias", xNum: qualityGainSearchTargetScaleNum, xDen: qualityGainSearchTargetScaleDen, yNum: qualityGainSearchAdaptiveContributionScaleNum, yDen: qualityGainSearchAdaptiveContributionScaleDen, gpcNum: qualityGainSearchFixedContributionScaleNum, gpcDen: qualityGainSearchFixedContributionScaleDen},
	}

	tmp := t.TempDir()
	t.Logf("external sample core gain-mode diagnostic: %s", path)
	t.Logf("%-18s %6s %10s %10s %10s %8s %8s %7s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, mode := range modes {
		xNum, xDen := mode.xNum, mode.xDen
		yNum, yDen := mode.yNum, mode.yDen
		zNum, zDen := mode.zNum, mode.zDen
		gpcNum, gpcDen := mode.gpcNum, mode.gpcDen
		normalizeGainSweepMode(&xNum, &xDen)
		normalizeGainSweepMode(&yNum, &yDen)
		normalizeGainSweepMode(&zNum, &zDen)
		normalizeGainSweepMode(&gpcNum, &gpcDen)
		frames := encodeBitstreamFramesGainSearchScale(t, src,
			xNum, xDen, yNum, yDen,
			zNum, zDen, gpcNum, gpcDen,
			mode.native, mode.rawLinear, mode.fullCost)
		rawPath := filepath.Join(tmp, mode.name+".g729")
		pcmPath := filepath.Join(tmp, mode.name+".ffmpeg.s16le")
		writePackedFrames(t, frames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		ff := s16leToSamples(readFile(t, pcmPath))
		if len(ff) > originalSamples {
			ff = ff[:originalSamples]
		}
		if len(ff) < originalSamples {
			t.Fatalf("%s decoded output too short: ffmpeg=%d want >= %d", mode.name, len(ff), originalSamples)
		}
		m := externalQualityMetricsFor(ref, ff, 240)
		t.Logf("%-18s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
	}
}

func TestExternalSampleProductionGainModeDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_MODE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_MODE=1 to compare production-state gain quantizer modes")
	}
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
	ref := src[:originalSamples]

	modes := []externalProductionGainMode{
		{name: "core-production", production: true},
		{name: "preselect-wide", search: "preselect", wide: true},
		{name: "preselect-floatopt-core", search: "preselect-floatopt", wide: true, thresholdScan: true},
		{name: "preselect-bigopt-core", search: "preselect-bigopt", wide: true, thresholdScan: true},
		{name: "preselect-bigopt-cliprepair-core", search: "preselect-bigopt", wide: true, thresholdScan: true, gainClipRepair: true},
		{name: "preselect-norm23-core", search: "preselect-norm", wide: true, thresholdScan: true, preselectTargetBits: 23},
		{name: "preselect-floatgp-core", search: "preselect-floatgp", wide: true, thresholdScan: true},
		{name: "preselect-floatgc-core", search: "preselect-floatgc", wide: true, thresholdScan: true},
		{name: "preselect-floatopt-sparse-search", search: "preselect-floatopt", wide: true, thresholdScan: true, gainSparseSearch: true},
		{name: "preselect-bigopt-sparse-search", search: "preselect-bigopt", wide: true, thresholdScan: true, gainSparseSearch: true},
		{name: "preselect-bigopt-cliprepair-sparse-search", search: "preselect-bigopt", wide: true, thresholdScan: true, gainSparseSearch: true, gainClipRepair: true},
		{name: "preselect-norm23-sparse-search", search: "preselect-norm", wide: true, thresholdScan: true, gainSparseSearch: true, preselectTargetBits: 23},
		{name: "preselect-floatopt-sparse-local", search: "preselect-floatopt", wide: true, thresholdScan: true, gainSparseSearch: true, gainSparseCommit: true},
		{name: "preselect-bigopt-sparse-local", search: "preselect-bigopt", wide: true, thresholdScan: true, gainSparseSearch: true, gainSparseCommit: true},
		{name: "preselect-norm23-sparse-local", search: "preselect-norm", wide: true, thresholdScan: true, gainSparseSearch: true, gainSparseCommit: true, preselectTargetBits: 23},
		{name: "preselect-floatopt-wide", search: "preselect-floatopt", wide: true},
		{name: "preselect-bigopt-wide", search: "preselect-bigopt", wide: true},
		{name: "preselect-bigopt-cliprepair-wide", search: "preselect-bigopt", wide: true, gainClipRepair: true},
		{name: "preselect-norm23-wide", search: "preselect-norm", wide: true, preselectTargetBits: 23},
		{name: "preselect-wide-sparse-search", search: "preselect", wide: true, gainSparseSearch: true},
		{name: "preselect-wide-sparse-local", search: "preselect", wide: true, gainSparseSearch: true, gainSparseCommit: true},
		{name: "preselect-linear-wide", search: "preselect-linear", wide: true},
		{name: "preselect-native-wide", search: "preselect-native", wide: true},
		{name: "fullcost-wide", search: "fullcost", wide: true},
		{name: "native-wide", search: "native", wide: true},
		{name: "native-wide-sparse-search", search: "native", wide: true, gainSparseSearch: true},
		{name: "native-wide-sparse-local", search: "native", wide: true, gainSparseSearch: true, gainSparseCommit: true},
		{name: "preselect-bounded", search: "preselect"},
		{name: "preselect-bounded-sparse-search", search: "preselect", gainSparseSearch: true},
		{name: "preselect-bounded-sparse-local", search: "preselect", gainSparseSearch: true, gainSparseCommit: true},
		{name: "preselect-linear-bounded", search: "preselect-linear"},
		{name: "preselect-native-bounded", search: "preselect-native"},
		{name: "fullcost-bounded", search: "fullcost"},
		{name: "native-bounded", search: "native"},
	}

	tmp := t.TempDir()
	t.Logf("external sample production gain-mode diagnostic: %s", path)
	t.Logf("%-18s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-18s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			mode.name,
			ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleProductionGainClipMarkerDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_CLIP_MARKERS") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_CLIP_MARKERS=1 to locate production gain-mode near-clip markers")
	}
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
	ref := src[:originalSamples]

	modes := []externalProductionGainMode{
		{name: "core-production", production: true},
		{name: "preselect-bigopt-core", search: "preselect-bigopt", wide: true, thresholdScan: true},
		{name: "preselect-bigopt-cliprepair-core", search: "preselect-bigopt", wide: true, thresholdScan: true, gainClipRepair: true},
		{name: "preselect-bigopt-cliprepair-wide", search: "preselect-bigopt", wide: true, gainClipRepair: true},
		{name: "norm22-sparse-search", search: "preselect-norm", wide: true, thresholdScan: true, gainSparseSearch: true, preselectTargetBits: 22},
	}

	tmp := t.TempDir()
	t.Logf("external sample production gain near-clip marker diagnostic: %s", path)
	t.Logf("%-38s %6s %10s %10s %10s %8s %8s %7s %8s", "Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, mode := range modes {
		frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, mode.name+"-markers", frames, originalSamples)
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		markers := externalNearClipMarkers(decoded, metrics.shift, len(frames), 32700, 40)
		t.Logf("%-38s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			mode.name, metrics.shift, metrics.rms, metrics.globalSNR, metrics.segSNR,
			metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)
		for _, marker := range markers {
			t.Logf("  %s decoded %.3fs..%.3fs samples %d..%d ref %.3fs..%.3fs frames %d..%d count %d peak %d value %d",
				mode.name,
				float64(marker.startSample)/8000.0,
				float64(marker.endSample)/8000.0,
				marker.startSample,
				marker.endSample,
				float64(marker.refStartSample)/8000.0,
				float64(marker.refEndSample)/8000.0,
				marker.startFrame,
				marker.endFrame,
				marker.count,
				marker.peak,
				marker.value)
		}
	}
}

func TestExternalSampleProductionGainBigOptRepairThresholdDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_BIGOPT_REPAIR_THRESHOLD") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_BIGOPT_REPAIR_THRESHOLD=1 to sweep bigopt gain-repair thresholds")
	}
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
	ref := src[:originalSamples]

	thresholds := []int{32300, 32000, 31800, 31600, 31200, 30800, 30600, 30500, 30400, 30300, 30200, 30000, 29200, 28400}
	tmp := t.TempDir()
	t.Logf("external sample production bigopt/norm gain-repair threshold diagnostic: %s", path)
	t.Logf("%-10s %-8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s", "Mode", "Thresh", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	searchModes := []externalProductionGainMode{
		{name: "bigopt-wide", search: "preselect-bigopt", wide: true, gainClipRepair: true},
		{name: "bigopt-wide-tscan", search: "preselect-bigopt", wide: true, thresholdScan: true, gainClipRepair: true},
		{name: "norm24-wide", search: "preselect-norm", wide: true, gainClipRepair: true, preselectTargetBits: 24},
	}
	for _, baseMode := range searchModes {
		for _, threshold := range thresholds {
			mode := baseMode
			mode.name = fmt.Sprintf("%s-%d", baseMode.name, threshold)
			mode.gainClipRepairThreshold = threshold
			frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
			ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
			decoded := decodeExternalFramesWithFFmpeg(t, tmp, mode.name+"-markers", frames, originalSamples)
			markers := externalNearClipMarkers(decoded, ff.shift, len(frames), 32700, 40)
			t.Logf("%-10s %-8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
				baseMode.name, threshold, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
				ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
			if len(markers) > 0 {
				t.Logf("%s threshold %d markers: %v", baseMode.name, threshold, externalNearClipMarkerSummary(markers))
			}
		}
	}
}

func TestExternalSampleProductionGainNormTargetDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_NORM_TARGET") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_NORM_TARGET=1 to sweep fixed-point gain preselect target bits")
	}
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
	ref := src[:originalSamples]

	type variant struct {
		suffix           string
		thresholdScan    bool
		gainSparseSearch bool
		gainSparseCommit bool
	}
	variants := []variant{
		{suffix: "core", thresholdScan: true},
		{suffix: "sparse-search", thresholdScan: true, gainSparseSearch: true},
		{suffix: "sparse-local", thresholdScan: true, gainSparseSearch: true, gainSparseCommit: true},
		{suffix: "wide"},
	}
	targets := []uint{14, 16, 18, 20, 21, 22, 23, 24}

	tmp := t.TempDir()
	t.Logf("external sample production gain normalized-optimum target diagnostic: %s", path)
	t.Logf("%-22s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, target := range targets {
		for _, v := range variants {
			mode := externalProductionGainMode{
				name:                fmt.Sprintf("norm%d-%s", target, v.suffix),
				search:              "preselect-norm",
				wide:                true,
				thresholdScan:       v.thresholdScan,
				gainSparseSearch:    v.gainSparseSearch,
				gainSparseCommit:    v.gainSparseCommit,
				preselectTargetBits: target,
			}
			frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
			ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
			t.Logf("%-22s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
				mode.name,
				ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
				ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		}
	}
}

func TestExternalSampleTamingDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_TAMING") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_TAMING=1 to audit encoder-side gain taming")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	t.Logf("external sample taming diagnostic: %s", path)
	t.Logf("%-24s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s %8s %8s %8s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip",
		"LocalSNR", "LocalSeg", "LocalNC", "subfr", "tame", "maxRaw", "maxDelta")

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core-production", profile: EncoderProfileCore},
		{name: "quality-production", profile: EncoderProfileQuality},
	}
	for _, p := range profiles {
		frames, stats := encodeBitstreamFramesWithProfileTamingStats(t, src, p.profile)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, p.name, frames, ref, originalSamples)
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d %8d %8d %8d %8d",
			p.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip,
			stats.subframes, stats.tameCount, stats.maxRawGpQ14, stats.maxDeltaQ14)
		for _, ex := range stats.examples {
			t.Logf("  tame %-18s frame=%d sub=%d rawGpQ14=%d commitGpQ14=%d deltaQ14=%d ga=%d gb=%d",
				p.name, ex.frame, ex.sub, ex.rawGpQ14, ex.commitGpQ14, ex.deltaQ14, ex.gaBits, ex.gbBits)
		}
	}

	modes := []externalProductionGainMode{
		{name: "core-helper-tame", search: "preselect", wide: true, thresholdScan: true},
		{name: "core-helper-no-tame", search: "preselect", wide: true, thresholdScan: true, taming: "none"},
	}
	for _, mode := range modes {
		frames := encodeBitstreamFramesProductionGainMode(t, src, mode)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d %8s %8s %8s %8s",
			mode.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip,
			"-", "-", "-", "-")
	}
}

type externalTamingAuditStats struct {
	subframes   int
	tameCount   int
	maxRawGpQ14 int
	maxDeltaQ14 int
	examples    []externalTamingAuditExample
}

type externalTamingAuditExample struct {
	frame       int
	sub         int
	gaBits      uint8
	gbBits      uint8
	rawGpQ14    int
	commitGpQ14 int
	deltaQ14    int
}

func encodeBitstreamFramesWithProfileTamingStats(t *testing.T, samples []int16, profile EncoderProfile) ([]bitstream.Frame, externalTamingAuditStats) {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	var stats externalTamingAuditStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frame := off / FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frame, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			_, _ = enc.closedloopStep(sub)
			recordExternalTamingAudit(&stats, enc, frame, sub)
		}
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, stats
}

func recordExternalTamingAudit(stats *externalTamingAuditStats, e *Encoder, frame, sub int) {
	stats.subframes++
	var gaBits, gbBits uint8
	if sub == 0 {
		gaBits, gbBits = e.ga1, e.gb1
	} else {
		gaBits, gbBits = e.ga2, e.gb2
	}
	rawGpQ14 := int(externalGainGpQ14(gaBits, gbBits))
	if rawGpQ14 > stats.maxRawGpQ14 {
		stats.maxRawGpQ14 = rawGpQ14
	}
	if !e.prevTaming {
		return
	}
	commitGpQ14 := int(e.prevGpQ14)
	deltaQ14 := rawGpQ14 - commitGpQ14
	if deltaQ14 < 0 {
		deltaQ14 = -deltaQ14
	}
	stats.tameCount++
	if deltaQ14 > stats.maxDeltaQ14 {
		stats.maxDeltaQ14 = deltaQ14
	}
	if len(stats.examples) < 12 {
		stats.examples = append(stats.examples, externalTamingAuditExample{
			frame:       frame,
			sub:         sub,
			gaBits:      gaBits,
			gbBits:      gbBits,
			rawGpQ14:    rawGpQ14,
			commitGpQ14: commitGpQ14,
			deltaQ14:    deltaQ14,
		})
	}
}

type externalProductionGainMode struct {
	name                    string
	search                  string
	wide                    bool
	gainSparseSearch        bool
	gainSparseCommit        bool
	gainClipRepair          bool
	gainClipRepairThreshold int
	production              bool
	thresholdScan           bool
	taming                  string
	preselectTargetBits     uint
}

func encodeBitstreamFramesProductionGainMode(t *testing.T, samples []int16, mode externalProductionGainMode) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		if mode.production {
			_, _ = enc.closedloopStep(0)
			_, _ = enc.closedloopStep(1)
		} else {
			closedloopStepProductionGainMode(enc, 0, mode)
			closedloopStepProductionGainMode(enc, 1, mode)
		}
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func closedloopStepProductionGainMode(e *Encoder, sub int, mode externalProductionGainMode) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := clpitch.SearchInteger(&xb, excSlice, centre, sub)
	intLag, frac := refineProductionPitchFraction(&xb, excSlice, sub, intLag, e.intT1)

	var v, y [clpitch.SubframeLen]int16
	e.adaptiveVectorForSynthesis(excSlice, intLag, frac, &v)
	gp := clpitch.GpAndY(&x, &v, &h, &y)

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = clpitch.EncodeP1(intLag, frac)
		e.p0 = clpitch.EncodeP0(e.p1)
	} else {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = clpitch.EncodeP2(intLag, frac, tmin)
	}

	e.fcbStepProductionGainMode(sub, aHat, sFrame, &x, &y, &h, &v, gp, mode)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func refineProductionPitchFraction(xb *[clpitch.SubframeLen]int16, exc []int16, sub int, intLag, intT1 int16) (int16, int8) {
	if sub == 0 {
		return clpitch.RefineFractionSubframe1(xb, exc, intLag)
	}
	return clpitch.RefineFractionSubframe2(xb, exc, intLag, intT1)
}

func (e *Encoder) fcbStepProductionGainMode(
	sub int,
	aHat *[lpc.LPCOrder + 1]int16,
	refSpeech *[clpitch.SubframeLen]int16,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	mode externalProductionGainMode,
) {
	const N = clpitch.SubframeLen

	var xPrime [N]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}

	hSearch := *h
	fcb.ApplyPitchEnhancement(&hSearch, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14))

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	if mode.thresholdScan {
		limit := e.coreFCBThresholdScanLimit(sub)
		entered := fcbsearch.SearchDepthFirstThresholdScanEntered(
			&dAbs, &phi, &positions, &sumOut,
			limit,
		)
		e.recordCoreFCBThresholdEntries(entered)
	} else {
		fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)
	}

	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)
	cSearch := &c
	cCommit := &c
	if mode.gainSparseSearch || mode.gainSparseCommit {
		var sparse [N]int16
		fcbsearch.BuildSparseCode(&positions, &signs, &sparse)
		if mode.gainSparseSearch {
			cSearch = &sparse
		}
		if mode.gainSparseCommit {
			cCommit = &sparse
		}
	}

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, cSearch)
	if mode.wide {
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&e.pastQuaEn, cSearch)
	}

	var gaPhys, gbPhys uint8
	var gpHatQ14, gammaCQ13 int16
	switch mode.search {
	case "preselect":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = gainquant.SearchConjugate(x, y, &z, gpcPredQ12)
	case "preselect-floatopt":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectMixedOptExhaustive(x, y, &z, gpcPredQ12, true, true)
	case "preselect-bigopt":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectBigOptExhaustive(x, y, &z, gpcPredQ12)
	case "preselect-norm":
		if mode.preselectTargetBits == 0 {
			panic("preselect-norm requires preselectTargetBits")
		}
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = gainquant.SearchConjugatePreselectTargetBits(x, y, &z, gpcPredQ12, mode.preselectTargetBits)
	case "preselect-floatgp":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectMixedOptExhaustive(x, y, &z, gpcPredQ12, true, false)
	case "preselect-floatgc":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectMixedOptExhaustive(x, y, &z, gpcPredQ12, false, true)
	case "preselect-gpclip":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectGpClipExhaustive(x, y, &z, gpcPredQ12)
	case "preselect-linear":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectLinearExhaustive(x, y, &z, gpcPredQ12)
	case "preselect-native":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectNativeExhaustiveWide(&e.pastQuaEn, cSearch, x, y, &z, gpcPredQ12, mode.wide)
	case "preselect-native-gpclip":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugatePreselectNativeGpClipExhaustiveWide(&e.pastQuaEn, cSearch, x, y, &z, gpcPredQ12, mode.wide)
	case "fullcost":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateFullCostExhaustive(x, y, &z, gpcPredQ12)
	case "native":
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateNativeExhaustiveWide(&e.pastQuaEn, cSearch, x, y, &z, mode.wide)
	default:
		panic("unknown production gain mode: " + mode.search)
	}

	taming := false
	if mode.taming != "none" {
		gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
		taming = gpTamed != gpHatQ14
		gpHatQ14 = gpTamed
	}

	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
	_, gcMantQ14, gcExp := reconstructExternalGainCandidate(&e.pastQuaEn, cCommit, gaPhys, gbPhys, mode.wide)
	if mode.gainClipRepair {
		tFrac := e.frac1
		if sub == 1 {
			tFrac = e.frac2
		}
		prevTuning := e.qualityTuning
		prevClipThreshold := qualityGainClipRepairThreshold
		e.qualityTuning = encoderTuningGainClipRepair
		if mode.wide {
			e.qualityTuning |= encoderTuningWideGainPredictor
		}
		if mode.gainClipRepairThreshold > 0 {
			qualityGainClipRepairThreshold = mode.gainClipRepairThreshold
		}
		gaPhys, gbPhys, gpHatQ14, gammaCQ13, taming, gcMantQ14, gcExp = e.qualityRepairGainClip(
			aHat, refSpeech, intLag, tFrac, cPacked, s, cCommit, gaPhys, gbPhys, gpHatQ14, gammaCQ13, taming, gcMantQ14, gcExp,
		)
		qualityGainClipRepairThreshold = prevClipThreshold
		e.qualityTuning = prevTuning
	}

	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	if sub == 0 {
		e.s1 = s
		e.c1 = cPacked
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.s2 = s
		e.c2 = cPacked
		e.ga2 = gaBits
		e.gb2 = gbBits
	}

	for n := 30; n < N; n++ {
		gpY := applyGainQ14ToQ0(gpHatQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-N], e.oldExc[N:])
	base := len(e.oldExc) - N
	var uHat [N]int16
	synth.BuildExcitation(gpHatQ14, gcMantQ14, gcExp, v, &c, &uHat)
	copy(e.oldExc[base:], uHat[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)
	e.prevGpQ14 = gpHatQ14
	e.prevTaming = taming
}

func searchConjugatePreselectNativeExhaustiveWide(past *[4]int16, c, x, y, z *[40]int16, gpcSearchQ12 int32, wide bool) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !gainSearchPreselectContains(&ctx, gpcSearchQ12, gai, gbi) {
				continue
			}
			gp, gcMant, gcExp := reconstructExternalGainCandidate(past, c, gai, gbi, wide)
			cost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = gp
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

const gainPreselectGpOptUpperQ14 = int64(19661) // round(1.2 * 2^14), §3.7.3 eq. (43)

func searchConjugatePreselectNativeGpClipExhaustiveWide(past *[4]int16, c, x, y, z *[40]int16, gpcSearchQ12 int32, wide bool) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !gainSearchPreselectContainsGpClip(&ctx, gpcSearchQ12, gai, gbi) {
				continue
			}
			gp, gcMant, gcExp := reconstructExternalGainCandidate(past, c, gai, gbi, wide)
			cost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = gp
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func searchConjugateNativeExhaustiveWide(past *[4]int16, c, x, y, z *[40]int16, wide bool) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp, gcMant, gcExp := reconstructExternalGainCandidate(past, c, gai, gbi, wide)
			cost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = gp
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func searchConjugatePreselectGpClipExhaustive(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	shift := gainSearchCostShiftDiagnostic(&ctx, gpcPredQ12, func(ga, gb uint8) bool {
		return gainSearchPreselectContainsGpClip(&ctx, gpcPredQ12, ga, gb)
	})
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !gainSearchPreselectContainsGpClip(&ctx, gpcPredQ12, gai, gbi) {
				continue
			}
			cost := gainSearchCostWithShift(&ctx, gai, gbi, gpcPredQ12, shift)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0])))
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func searchConjugatePreselectLinearExhaustive(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	bestCost := math.Inf(1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !gainSearchPreselectContains(&ctx, gpcPredQ12, gai, gbi) {
				continue
			}
			cost := gainLinearResidualCostFloat(x, y, z, gai, gbi, gpcPredQ12)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0])))
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func searchConjugatePreselectMixedOptExhaustive(x, y, z *[40]int16, gpcPredQ12 int32, floatGP, floatGC bool) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	gpOptQ14, gcOptQ12 := gainSearchFloatOptQ(&ctx)
	if !floatGP {
		gpOptQ14 = float64(ctx.gpOptQ14)
	}
	if !floatGC {
		gcOptQ12 = float64(ctx.gcOptQ12)
	}
	allow := func(ga, gb uint8) bool {
		return gainSearchPreselectContainsFloatOpt(gpcPredQ12, ga, gb, gpOptQ14, gcOptQ12)
	}
	shift := gainSearchCostShiftDiagnostic(&ctx, gpcPredQ12, allow)
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !allow(gai, gbi) {
				continue
			}
			cost := gainSearchCostWithShift(&ctx, gai, gbi, gpcPredQ12, shift)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0])))
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func searchConjugatePreselectBigOptExhaustive(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	ctx := gainSearchCostContext(x, y, z)
	gpOptQ14, gcOptQ12 := gainSearchBigOptQ(&ctx)
	allow := func(ga, gb uint8) bool {
		return gainSearchPreselectContainsIntOpt(gpcPredQ12, ga, gb, gpOptQ14, gcOptQ12)
	}
	shift := gainSearchCostShiftDiagnostic(&ctx, gpcPredQ12, allow)
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if !allow(gai, gbi) {
				continue
			}
			cost := gainSearchCostWithShift(&ctx, gai, gbi, gpcPredQ12, shift)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0])))
				gammaCQ13 = gainGammaQ13(gai, gbi)
			}
		}
	}
	return
}

func gainSearchFloatOptQ(ctx *gainSearchCostCtx) (gpOptQ14, gcOptQ12 float64) {
	A := float64(ctx.rawA)
	B := float64(ctx.rawB)
	C := float64(ctx.rawC)
	D := float64(ctx.rawD)
	F := float64(ctx.rawF)
	det := A*B - C*C
	switch {
	case det > 0:
		gpOptQ14 = ((D*B - F*C) * 16384.0) / det
		gcOptQ12 = ((F*A - D*C) * 4096.0) / det
	case A > 0:
		gpOptQ14 = D * 16384.0 / A
	case B > 0:
		gcOptQ12 = F * 4096.0 / B
	}
	if math.IsNaN(gpOptQ14) || gpOptQ14 < 0 {
		gpOptQ14 = 0
	}
	if math.IsNaN(gcOptQ12) || gcOptQ12 < 0 {
		gcOptQ12 = 0
	}
	return gpOptQ14, gcOptQ12
}

func gainSearchBigOptQ(ctx *gainSearchCostCtx) (gpOptQ14, gcOptQ12 int64) {
	A := big.NewInt(ctx.rawA)
	B := big.NewInt(ctx.rawB)
	C := big.NewInt(ctx.rawC)
	D := big.NewInt(ctx.rawD)
	F := big.NewInt(ctx.rawF)

	det := new(big.Int).Sub(new(big.Int).Mul(A, B), new(big.Int).Mul(C, C))
	switch {
	case det.Sign() > 0:
		numGp := new(big.Int).Sub(new(big.Int).Mul(D, B), new(big.Int).Mul(F, C))
		numGc := new(big.Int).Sub(new(big.Int).Mul(F, A), new(big.Int).Mul(D, C))
		gpOptQ14 = bigScaledQuoInt64Diagnostic(numGp, 14, det)
		gcOptQ12 = bigScaledQuoInt64Diagnostic(numGc, 12, det)
	case ctx.rawA > 0:
		gpOptQ14 = bigScaledQuoInt64Diagnostic(D, 14, A)
	case ctx.rawB > 0:
		gcOptQ12 = bigScaledQuoInt64Diagnostic(F, 12, B)
	}
	if gpOptQ14 < 0 {
		gpOptQ14 = 0
	}
	if gcOptQ12 < 0 {
		gcOptQ12 = 0
	}
	return gpOptQ14, gcOptQ12
}

func bigScaledQuoInt64Diagnostic(num *big.Int, shift uint, den *big.Int) int64 {
	if den.Sign() <= 0 {
		return 0
	}
	var scaled big.Int
	scaled.Lsh(num, shift)
	scaled.Quo(&scaled, den)
	if !scaled.IsInt64() {
		if scaled.Sign() < 0 {
			return 0
		}
		return int64(^uint64(0) >> 1)
	}
	return scaled.Int64()
}

func gainSearchPreselectContainsFloatOpt(gpcSearchQ12 int32, ga, gb uint8, gpOptQ14, gcOptQ12 float64) bool {
	gaCand := float64((int64(tables.GainGBK1[ga][1]) * int64(gpcSearchQ12)) >> 13)
	var betterGA int
	for i := uint8(0); i < 8; i++ {
		cand := float64((int64(tables.GainGBK1[i][1]) * int64(gpcSearchQ12)) >> 13)
		if math.Abs(cand-gcOptQ12) < math.Abs(gaCand-gcOptQ12) {
			betterGA++
		}
	}
	if betterGA >= 4 {
		return false
	}
	gbCand := float64(tables.GainGBK2[gb][0])
	var betterGB int
	for j := uint8(0); j < 16; j++ {
		if math.Abs(float64(tables.GainGBK2[j][0])-gpOptQ14) < math.Abs(gbCand-gpOptQ14) {
			betterGB++
		}
	}
	return betterGB < 8
}

func gainSearchPreselectContainsIntOpt(gpcSearchQ12 int32, ga, gb uint8, gpOptQ14, gcOptQ12 int64) bool {
	gaCand := (int64(tables.GainGBK1[ga][1]) * int64(gpcSearchQ12)) >> 13
	gaDist := absInt64Diagnostic(gaCand - gcOptQ12)
	var betterGA int
	for i := uint8(0); i < 8; i++ {
		cand := (int64(tables.GainGBK1[i][1]) * int64(gpcSearchQ12)) >> 13
		if absInt64Diagnostic(cand-gcOptQ12) < gaDist {
			betterGA++
		}
	}
	if betterGA >= 4 {
		return false
	}
	gbCand := int64(tables.GainGBK2[gb][0])
	gbDist := absInt64Diagnostic(gbCand - gpOptQ14)
	var betterGB int
	for j := uint8(0); j < 16; j++ {
		if absInt64Diagnostic(int64(tables.GainGBK2[j][0])-gpOptQ14) < gbDist {
			betterGB++
		}
	}
	return betterGB < 8
}

func TestExternalSampleGainPreselectNativeAuditDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_NATIVE_AUDIT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_NATIVE_AUDIT=1 to audit native gain optimum vs Annex A preselect")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	stats := collectGainPreselectNativeAudit(t, src)
	logGainPreselectNativeAudit(t, "external sample gain preselect/native audit: "+path, stats)
}

type gainPreselectNativeAuditStats struct {
	count int

	fullInPreselect int
	preSameFull     int
	coreSameFull    int
	coreSamePre     int
	preLowerCore    int
	fullLowerCore   int

	coreLossPctSum float64
	preLossPctSum  float64
	coreLossPctMax float64
	preLossPctMax  float64

	examples []gainPreselectNativeAuditExample
}

type gainPreselectNativeAuditExample struct {
	frame int
	sub   int

	coreGA uint8
	coreGB uint8
	preGA  uint8
	preGB  uint8
	fullGA uint8
	fullGB uint8

	fullInPreselect bool
	coreLossPct     float64
	preLossPct      float64
	gpOptQ14        int64
	gcOptQ12        int64
	gpcPredQ12      int32
}

func collectGainPreselectNativeAudit(t *testing.T, samples []int16) gainPreselectNativeAuditStats {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	var stats gainPreselectNativeAuditStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frame := off / FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frame, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			row := gainPreselectNativeAuditSubframe(enc, sub)
			recordGainPreselectNativeAudit(&stats, frame, sub, row)
			closedloopStepProductionGainMode(enc, sub, externalProductionGainMode{search: "preselect", wide: true})
		}
	}
	return stats
}

type gainPreselectNativeAuditRow struct {
	coreGA uint8
	coreGB uint8
	preGA  uint8
	preGB  uint8
	fullGA uint8
	fullGB uint8

	fullInPreselect bool

	coreNativeCost int64
	preNativeCost  int64
	fullNativeCost int64

	gpOptQ14   int64
	gcOptQ12   int64
	gpcPredQ12 int32
}

func gainPreselectNativeAuditSubframe(e *Encoder, sub int) gainPreselectNativeAuditRow {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := clpitch.SearchInteger(&xb, excSlice, centre, sub)
	intLag, frac := refineProductionPitchFraction(&xb, excSlice, sub, intLag, e.intT1)

	var v, y [clpitch.SubframeLen]int16
	e.adaptiveVectorForSynthesis(excSlice, intLag, frac, &v)
	gp := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)

	hSearch := h
	fcb.ApplyPitchEnhancement(&hSearch, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14))

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [clpitch.SubframeLen]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12Wide(&e.pastQuaEn, &c)
	ctx := gainSearchCostContext(&x, &y, &z)

	coreGA, coreGB, _, _ := gainquant.SearchConjugate(&x, &y, &z, gpcPredQ12)
	preGA, preGB, _, _ := searchConjugatePreselectNativeExhaustiveWide(&e.pastQuaEn, &c, &x, &y, &z, gpcPredQ12, true)
	fullGA, fullGB, _, _ := searchConjugateNativeExhaustiveWide(&e.pastQuaEn, &c, &x, &y, &z, true)

	coreGp, coreMant, coreExp := gainquant.ReconstructWide(&e.pastQuaEn, &c, coreGA, coreGB)
	preGp, preMant, preExp := gainquant.ReconstructWide(&e.pastQuaEn, &c, preGA, preGB)
	fullGp, fullMant, fullExp := gainquant.ReconstructWide(&e.pastQuaEn, &c, fullGA, fullGB)

	return gainPreselectNativeAuditRow{
		coreGA:          coreGA,
		coreGB:          coreGB,
		preGA:           preGA,
		preGB:           preGB,
		fullGA:          fullGA,
		fullGB:          fullGB,
		fullInPreselect: gainSearchPreselectContains(&ctx, gpcPredQ12, fullGA, fullGB),
		coreNativeCost:  gainResidualEnergyQ0(&x, &y, &z, coreGp, coreMant, coreExp),
		preNativeCost:   gainResidualEnergyQ0(&x, &y, &z, preGp, preMant, preExp),
		fullNativeCost:  gainResidualEnergyQ0(&x, &y, &z, fullGp, fullMant, fullExp),
		gpOptQ14:        ctx.gpOptQ14,
		gcOptQ12:        ctx.gcOptQ12,
		gpcPredQ12:      gpcPredQ12,
	}
}

func recordGainPreselectNativeAudit(stats *gainPreselectNativeAuditStats, frame, sub int, row gainPreselectNativeAuditRow) {
	stats.count++
	if row.fullInPreselect {
		stats.fullInPreselect++
	}
	if row.preGA == row.fullGA && row.preGB == row.fullGB {
		stats.preSameFull++
	}
	if row.coreGA == row.fullGA && row.coreGB == row.fullGB {
		stats.coreSameFull++
	}
	if row.coreGA == row.preGA && row.coreGB == row.preGB {
		stats.coreSamePre++
	}
	if row.preNativeCost < row.coreNativeCost {
		stats.preLowerCore++
	}
	if row.fullNativeCost < row.coreNativeCost {
		stats.fullLowerCore++
	}

	coreLossPct := gainCostLossPct(row.coreNativeCost, row.fullNativeCost)
	preLossPct := gainCostLossPct(row.preNativeCost, row.fullNativeCost)
	stats.coreLossPctSum += coreLossPct
	stats.preLossPctSum += preLossPct
	if coreLossPct > stats.coreLossPctMax {
		stats.coreLossPctMax = coreLossPct
	}
	if preLossPct > stats.preLossPctMax {
		stats.preLossPctMax = preLossPct
	}
	if len(stats.examples) < 8 && (!row.fullInPreselect || preLossPct > 10 || coreLossPct > 25) {
		stats.examples = append(stats.examples, gainPreselectNativeAuditExample{
			frame:           frame,
			sub:             sub,
			coreGA:          row.coreGA,
			coreGB:          row.coreGB,
			preGA:           row.preGA,
			preGB:           row.preGB,
			fullGA:          row.fullGA,
			fullGB:          row.fullGB,
			fullInPreselect: row.fullInPreselect,
			coreLossPct:     coreLossPct,
			preLossPct:      preLossPct,
			gpOptQ14:        row.gpOptQ14,
			gcOptQ12:        row.gcOptQ12,
			gpcPredQ12:      row.gpcPredQ12,
		})
	}
}

func gainCostLossPct(cost, best int64) float64 {
	if best <= 0 {
		if cost == best {
			return 0
		}
		return math.Inf(1)
	}
	return 100 * float64(cost-best) / float64(best)
}

func logGainPreselectNativeAudit(t *testing.T, title string, stats gainPreselectNativeAuditStats) {
	t.Helper()
	t.Logf("%s", title)
	t.Logf("subframes=%d fullInPreselect=%.2f%% preSameFull=%.2f%% coreSameFull=%.2f%% coreSamePre=%.2f%% preLowerCore=%.2f%% fullLowerCore=%.2f%%",
		stats.count,
		percent(stats.fullInPreselect, stats.count),
		percent(stats.preSameFull, stats.count),
		percent(stats.coreSameFull, stats.count),
		percent(stats.coreSamePre, stats.count),
		percent(stats.preLowerCore, stats.count),
		percent(stats.fullLowerCore, stats.count))
	meanCoreLoss, meanPreLoss := 0.0, 0.0
	if stats.count > 0 {
		meanCoreLoss = stats.coreLossPctSum / float64(stats.count)
		meanPreLoss = stats.preLossPctSum / float64(stats.count)
	}
	t.Logf("native-cost loss vs full-native: core mean %.2f%% max %.2f%% ; preselect-native mean %.2f%% max %.2f%%",
		meanCoreLoss, stats.coreLossPctMax, meanPreLoss, stats.preLossPctMax)
	for _, ex := range stats.examples {
		t.Logf("example frame=%d sub=%d core=(%d,%d) pre=(%d,%d) full=(%d,%d) fullInPre=%t coreLoss=%.2f%% preLoss=%.2f%% gpOptQ14=%d gcOptQ12=%d gpcPredQ12=%d",
			ex.frame, ex.sub,
			ex.coreGA, ex.coreGB,
			ex.preGA, ex.preGB,
			ex.fullGA, ex.fullGB,
			ex.fullInPreselect,
			ex.coreLossPct, ex.preLossPct,
			ex.gpOptQ14, ex.gcOptQ12, ex.gpcPredQ12)
	}
}

func TestExternalSampleGainCostModelAuditDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_COST_MODEL_AUDIT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_COST_MODEL_AUDIT=1 to compare eq.63 cost ordering against direct linear residual")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	bounded := collectGainCostModelAudit(t, src, false)
	logGainCostModelAudit(t, "external sample gain cost-model audit bounded: "+path, bounded)
	wide := collectGainCostModelAudit(t, src, true)
	logGainCostModelAudit(t, "external sample gain cost-model audit wide: "+path, wide)
}

type gainCostModelAuditStats struct {
	count int

	fullCostLinearSame int
	preCostLinearSame  int
	corePreCostSame    int

	fullLinearInPreselect int
	preLinearSameFull     int
	preCostSameFull       int

	examples []gainCostModelAuditExample
}

type gainCostModelAuditExample struct {
	frame int
	sub   int

	fullCostGA   uint8
	fullCostGB   uint8
	fullLinearGA uint8
	fullLinearGB uint8
	preCostGA    uint8
	preCostGB    uint8
	preLinearGA  uint8
	preLinearGB  uint8
	coreGA       uint8
	coreGB       uint8

	fullLinearInPreselect bool
	costDelta             int64
	linearDelta           float64
	gpOptQ14              int64
	gcOptQ12              int64
	gpcPredQ12            int32
}

func collectGainCostModelAudit(t *testing.T, samples []int16, wide bool) gainCostModelAuditStats {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	var stats gainCostModelAuditStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frame := off / FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frame, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			row := gainCostModelAuditSubframe(enc, sub, wide)
			recordGainCostModelAudit(&stats, frame, sub, row)
			closedloopStepProductionGainMode(enc, sub, externalProductionGainMode{search: "preselect", wide: true})
		}
	}
	return stats
}

type gainCostModelAuditRow struct {
	fullCostGA   uint8
	fullCostGB   uint8
	fullLinearGA uint8
	fullLinearGB uint8
	preCostGA    uint8
	preCostGB    uint8
	preLinearGA  uint8
	preLinearGB  uint8
	coreGA       uint8
	coreGB       uint8

	fullLinearInPreselect bool
	preLinearSameFull     bool
	preCostSameFull       bool

	costDelta   int64
	linearDelta float64
	gpOptQ14    int64
	gcOptQ12    int64
	gpcPredQ12  int32
}

func gainCostModelAuditSubframe(e *Encoder, sub int, wide bool) gainCostModelAuditRow {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := clpitch.SearchInteger(&xb, excSlice, centre, sub)
	intLag, frac := refineProductionPitchFraction(&xb, excSlice, sub, intLag, e.intT1)

	var v, y [clpitch.SubframeLen]int16
	e.adaptiveVectorForSynthesis(excSlice, intLag, frac, &v)
	gp := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)

	hSearch := h
	fcb.ApplyPitchEnhancement(&hSearch, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14))

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [clpitch.SubframeLen]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	if wide {
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&e.pastQuaEn, &c)
	}
	ctx := gainSearchCostContext(&x, &y, &z)

	var out gainCostModelAuditRow
	out.gpOptQ14 = ctx.gpOptQ14
	out.gcOptQ12 = ctx.gcOptQ12
	out.gpcPredQ12 = gpcPredQ12
	out.fullCostGA, out.fullCostGB, out.costDelta = bestGainSearchCostCandidate(&ctx, gpcPredQ12, nil)
	out.fullLinearGA, out.fullLinearGB, out.linearDelta = bestGainLinearResidualCandidate(&x, &y, &z, gpcPredQ12, nil)
	out.preCostGA, out.preCostGB, _ = bestGainSearchCostCandidate(&ctx, gpcPredQ12, func(ga, gb uint8) bool {
		return gainSearchPreselectContains(&ctx, gpcPredQ12, ga, gb)
	})
	out.preLinearGA, out.preLinearGB, _ = bestGainLinearResidualCandidate(&x, &y, &z, gpcPredQ12, func(ga, gb uint8) bool {
		return gainSearchPreselectContains(&ctx, gpcPredQ12, ga, gb)
	})
	out.coreGA, out.coreGB, _, _ = gainquant.SearchConjugate(&x, &y, &z, gpcPredQ12)
	out.fullLinearInPreselect = gainSearchPreselectContains(&ctx, gpcPredQ12, out.fullLinearGA, out.fullLinearGB)
	out.preLinearSameFull = out.preLinearGA == out.fullLinearGA && out.preLinearGB == out.fullLinearGB
	out.preCostSameFull = out.preCostGA == out.fullCostGA && out.preCostGB == out.fullCostGB
	return out
}

func recordGainCostModelAudit(stats *gainCostModelAuditStats, frame, sub int, row gainCostModelAuditRow) {
	stats.count++
	if row.fullCostGA == row.fullLinearGA && row.fullCostGB == row.fullLinearGB {
		stats.fullCostLinearSame++
	}
	if row.preCostGA == row.preLinearGA && row.preCostGB == row.preLinearGB {
		stats.preCostLinearSame++
	}
	if row.coreGA == row.preCostGA && row.coreGB == row.preCostGB {
		stats.corePreCostSame++
	}
	if row.fullLinearInPreselect {
		stats.fullLinearInPreselect++
	}
	if row.preLinearSameFull {
		stats.preLinearSameFull++
	}
	if row.preCostSameFull {
		stats.preCostSameFull++
	}
	if len(stats.examples) < 8 &&
		(row.fullCostGA != row.fullLinearGA ||
			row.fullCostGB != row.fullLinearGB ||
			row.preCostGA != row.preLinearGA ||
			row.preCostGB != row.preLinearGB ||
			!row.fullLinearInPreselect) {
		stats.examples = append(stats.examples, gainCostModelAuditExample{
			frame:                 frame,
			sub:                   sub,
			fullCostGA:            row.fullCostGA,
			fullCostGB:            row.fullCostGB,
			fullLinearGA:          row.fullLinearGA,
			fullLinearGB:          row.fullLinearGB,
			preCostGA:             row.preCostGA,
			preCostGB:             row.preCostGB,
			preLinearGA:           row.preLinearGA,
			preLinearGB:           row.preLinearGB,
			coreGA:                row.coreGA,
			coreGB:                row.coreGB,
			fullLinearInPreselect: row.fullLinearInPreselect,
			costDelta:             row.costDelta,
			linearDelta:           row.linearDelta,
			gpOptQ14:              row.gpOptQ14,
			gcOptQ12:              row.gcOptQ12,
			gpcPredQ12:            row.gpcPredQ12,
		})
	}
}

func bestGainSearchCostCandidate(ctx *gainSearchCostCtx, gpcPredQ12 int32, allow func(uint8, uint8) bool) (ga, gb uint8, margin int64) {
	bestCost := int64(1<<63 - 1)
	secondCost := int64(1<<63 - 1)
	shift := gainSearchCostShiftDiagnostic(ctx, gpcPredQ12, allow)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if allow != nil && !allow(gai, gbi) {
				continue
			}
			cost := gainSearchCostWithShift(ctx, gai, gbi, gpcPredQ12, shift)
			if cost < bestCost {
				secondCost = bestCost
				bestCost = cost
				ga, gb = gai, gbi
			} else if cost < secondCost {
				secondCost = cost
			}
		}
	}
	if secondCost == int64(1<<63-1) {
		return ga, gb, 0
	}
	return ga, gb, secondCost - bestCost
}

func bestGainLinearResidualCandidate(x, y, z *[40]int16, gpcPredQ12 int32, allow func(uint8, uint8) bool) (ga, gb uint8, margin float64) {
	bestCost := math.Inf(1)
	secondCost := math.Inf(1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if allow != nil && !allow(gai, gbi) {
				continue
			}
			cost := gainLinearResidualCostFloat(x, y, z, gai, gbi, gpcPredQ12)
			if cost < bestCost {
				secondCost = bestCost
				bestCost = cost
				ga, gb = gai, gbi
			} else if cost < secondCost {
				secondCost = cost
			}
		}
	}
	if math.IsInf(secondCost, 1) {
		return ga, gb, 0
	}
	return ga, gb, secondCost - bestCost
}

func gainLinearResidualCostFloat(x, y, z *[40]int16, ga, gb uint8, gpcPredQ12 int32) float64 {
	gpQ14 := float64(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0]))
	gammaQ13 := float64(gainGammaQ13(ga, gb))
	gcQ12 := math.Trunc((gammaQ13 * float64(gpcPredQ12)) / 8192.0)
	const (
		gpScale = 16384.0
		gcScale = 16777216.0
	)
	var sum float64
	for n := 0; n < 40; n++ {
		err := float64(x[n]) -
			(gpQ14*float64(y[n]))/gpScale -
			(gcQ12*float64(z[n]))/gcScale
		sum += err * err
	}
	return sum
}

func logGainCostModelAudit(t *testing.T, title string, stats gainCostModelAuditStats) {
	t.Helper()
	t.Logf("%s", title)
	t.Logf("subframes=%d fullCost==linear %.2f%% preCost==linear %.2f%% core==preCost %.2f%% fullLinearInPreselect %.2f%% preLinearSameFull %.2f%% preCostSameFull %.2f%%",
		stats.count,
		percent(stats.fullCostLinearSame, stats.count),
		percent(stats.preCostLinearSame, stats.count),
		percent(stats.corePreCostSame, stats.count),
		percent(stats.fullLinearInPreselect, stats.count),
		percent(stats.preLinearSameFull, stats.count),
		percent(stats.preCostSameFull, stats.count))
	for _, ex := range stats.examples {
		t.Logf("example frame=%d sub=%d fullCost=(%d,%d) fullLinear=(%d,%d) preCost=(%d,%d) preLinear=(%d,%d) core=(%d,%d) fullLinearInPre=%t costMargin=%d linearMargin=%.6g gpOptQ14=%d gcOptQ12=%d gpcPredQ12=%d",
			ex.frame, ex.sub,
			ex.fullCostGA, ex.fullCostGB,
			ex.fullLinearGA, ex.fullLinearGB,
			ex.preCostGA, ex.preCostGB,
			ex.preLinearGA, ex.preLinearGB,
			ex.coreGA, ex.coreGB,
			ex.fullLinearInPreselect,
			ex.costDelta,
			ex.linearDelta,
			ex.gpOptQ14,
			ex.gcOptQ12,
			ex.gpcPredQ12)
	}
}

func TestExternalSampleGainTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_TRACE=1 to trace gain selection near the user-sample clip")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	startFrame, endFrame := externalGainTraceFrameWindow()
	variants := []struct {
		name   string
		tuning encoderQualityTuning
	}{
		{name: "core", tuning: 0},
		{name: "gain", tuning: encoderTuningGainSearchBias},
		{name: "early", tuning: encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "gain+early", tuning: encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "norm+gain", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias},
		{name: "norm+gain+early", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "quality", tuning: encoderQualityTuningAll},
		{name: "quality+residacb", tuning: encoderQualityTuningAll | encoderTuningResidualExtensionAdaptiveVector},
	}

	t.Logf("external sample gain trace diagnostic: %s", path)
	t.Logf("frame window=%d..%d (%.3fs..%.3fs at 8 kHz)", startFrame, endFrame,
		float64(startFrame*FrameSamples)/8000.0, float64((endFrame+1)*FrameSamples)/8000.0)
	t.Logf("%-8s %5s %3s %4s %4s %7s %7s %11s %11s %7s %7s %7s %7s %8s %8s %8s %8s %8s %8s %7s %9s %9s %9s %9s %8s %8s %7s %8s %8s %8s",
		"variant", "frame", "sub", "T", "fr",
		"GA", "GB", "sBest", "nBest", "gpUnq", "gpSel", "gpCom", "gcQ12",
		"gpcPred", "gpcSrch", "predSat", "predWide", "past0", "gpOpt", "gcOpt",
		"sRank", "nRank", "sCost", "nCost",
		"xRMS", "yRMS", "zPk", "uRMS", "uPk", "ewPk")
	for _, variant := range variants {
		rows := collectExternalGainTrace(t, src, variant.name, variant.tuning, startFrame, endFrame)
		for _, row := range rows {
			t.Logf("%-8s %5d %3d %4d %4d %7s %7s %11s %11s %7d %7d %7d %7d %8d %8d %8d %8d %8d %8d %7d %9d %9d %9d %9d %8.0f %8.0f %7d %8.0f %8d %8d",
				row.variant,
				row.frame,
				row.sub,
				row.intLag,
				row.frac,
				fmt.Sprintf("%d/%d", row.gaBits, row.gaPhys),
				fmt.Sprintf("%d/%d", row.gbBits, row.gbPhys),
				fmt.Sprintf("%d/%d", row.bestSearchGABits, row.bestSearchGBBits),
				fmt.Sprintf("%d/%d", row.bestNativeGABits, row.bestNativeGBBits),
				row.gpUnqQ14,
				row.gpSelectedQ14,
				row.gpCommitQ14,
				row.gcQ12,
				row.gpcPredQ12,
				row.gpcSearchQ12,
				row.predLogSatQ10,
				row.predLogWideQ10,
				row.pastQuaEn0,
				row.gpOptQ14,
				row.gcOptQ12,
				row.searchRank,
				row.nativeRank,
				row.searchCost/1000000,
				row.nativeCost/1000,
				row.xRMS,
				row.yRMS,
				row.zPeak,
				row.uRMS,
				row.uPeak,
				row.ewPeak,
			)
		}
	}
}

func TestExternalSampleGainPreselectMissDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_MISS") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_PRESELECT_MISS=1 to audit Core gain preselect misses")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	startFrame, endFrame := externalGainPreselectMissFrameWindow()
	stats := collectExternalGainPreselectMiss(t, src, startFrame, endFrame)
	t.Logf("external sample Core gain-preselect miss diagnostic: %s", path)
	t.Logf("focused frame window=%d..%d (%.3fs..%.3fs at 8 kHz)", startFrame, endFrame,
		float64(startFrame*FrameSamples)/8000.0, float64((endFrame+1)*FrameSamples)/8000.0)
	t.Logf("subframes=%d selected==full %.2f%% fullInPreselect %.2f%% fullGAInTop4 %.2f%% fullGBInTop8 %.2f%% gaMissOnly %.2f%% gbMissOnly %.2f%% bothMiss %.2f%% meanSelectedRank %.1f",
		stats.count,
		percent(stats.selectedSameFull, stats.count),
		percent(stats.fullInPreselect, stats.count),
		percent(stats.fullGAInTop, stats.count),
		percent(stats.fullGBInTop, stats.count),
		percent(stats.gaMissOnly, stats.count),
		percent(stats.gbMissOnly, stats.count),
		percent(stats.bothMiss, stats.count),
		meanInt64(stats.selectedRankSum, stats.count))
	t.Logf("%-6s %-3s %-9s %-9s %-9s %-8s %-8s %-8s %-8s %-8s %-8s %-10s %-10s",
		"frame", "sub", "selected", "preBest", "fullBest", "fullIn", "gaTop4", "gbTop8", "rank", "gpOpt", "gcOpt", "selCost", "fullCost")
	for _, row := range stats.examples {
		t.Logf("%-6d %-3d %-9s %-9s %-9s %-8t %-8t %-8t %-8d %-8d %-8d %-10d %-10d",
			row.frame, row.sub,
			fmt.Sprintf("%d/%d", row.selectedGABits, row.selectedGBBits),
			fmt.Sprintf("%d/%d", row.preBestGABits, row.preBestGBBits),
			fmt.Sprintf("%d/%d", row.fullBestGABits, row.fullBestGBBits),
			row.fullInPreselect,
			row.fullGAInTop,
			row.fullGBInTop,
			row.selectedRank,
			row.gpOptQ14,
			row.gcOptQ12,
			row.selectedCost/1000000,
			row.fullCost/1000000)
	}
}

func externalGainPreselectMissFrameWindow() (startFrame, endFrame int) {
	startFrame, endFrame = 292, 294
	if spec := os.Getenv("G729_EXTERNAL_GAIN_PRESELECT_MISS_FRAMES"); spec != "" {
		var start, end int
		if n, err := fmt.Sscanf(spec, "%d:%d", &start, &end); n == 2 && err == nil {
			startFrame, endFrame = start, end
		}
	}
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame < startFrame {
		endFrame = startFrame
	}
	return startFrame, endFrame
}

func TestExternalSampleDecodeClipStageDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_DECODE_CLIP_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_DECODE_CLIP_TRACE=1 to trace local decoder stages near clipped output")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	originalSamples := len(src)
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	profileName := strings.ToLower(strings.TrimSpace(os.Getenv("G729_EXTERNAL_SAMPLE_DECODE_CLIP_TRACE_PROFILE")))
	profile := EncoderProfileQuality
	if profileName == "core" {
		profile = EncoderProfileCore
	} else {
		profileName = "quality"
	}
	frames := encodeBitstreamFramesWithProfile(t, src, profile)
	var bcgFrames []bitstream.Frame
	if os.Getenv("G729_EXTERNAL_SAMPLE_DECODE_CLIP_TRACE_BCG") == "1" {
		tmp := t.TempDir()
		bcgRawPath := filepath.Join(tmp, "bcg.g729")
		writeBCGEncodedRawG729(t, src, bcgRawPath)
		bcgFrames = readRawG729Frames(t, readFile(t, bcgRawPath))
	}
	rows := collectExternalDecodeClipStageRows(t, frames, bcgFrames, originalSamples, 32700, 40)
	t.Logf("external sample local decode clip-stage diagnostic: %s profile=%s", path, profileName)
	t.Logf("%-6s %-3s %-4s %8s %8s %8s %8s %8s %7s %7s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s",
		"frame", "sub", "n", "out", "hp", "spf", "synth", "u", "gp", "gcQ12", "P", "fr", "C", "S", "GA", "GB", "bP", "bfr", "bC", "bS", "bGA", "bGB", "uPk", "sPk")
	for _, row := range rows {
		t.Logf("%-6d %-3d %-4d %8d %8d %8d %8d %8d %7d %7d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d",
			row.frame, row.sub, row.n, row.out, row.hp, row.spf, row.synth, row.u,
			row.gpQ14, row.gcQ12, row.pitch, row.frac, row.c, row.s, row.ga, row.gb,
			row.bcgPitch, row.bcgFrac, row.bcgC, row.bcgS, row.bcgGA, row.bcgGB,
			row.uPeak, row.sPeak)
	}
	if len(rows) == 0 {
		t.Logf("no output samples at or above threshold")
	}
}

func encodeBitstreamFramesWithProfile(t *testing.T, samples []int16, profile EncoderProfile) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	return encodeBitstreamFramesWithEncoder(t, samples, enc)
}

func encodeBitstreamFramesWithQualityTuning(t *testing.T, samples []int16, tuning encoderQualityTuning) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	enc.qualityTuning = tuning
	return encodeBitstreamFramesWithEncoder(t, samples, enc)
}

func encodeBitstreamFramesWithEncoder(t *testing.T, samples []int16, enc *Encoder) []bitstream.Frame {
	t.Helper()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	var packed [FrameBytes]byte
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if err := enc.EncodeFrame(samples[off:off+FrameSamples], packed[:]); err != nil {
			t.Fatalf("EncodeFrame frame %d: %v", off/FrameSamples, err)
		}
		var f bitstream.Frame
		if err := bitstream.Unpack(packed[:], &f); err != nil {
			t.Fatalf("Unpack encoded frame %d: %v", off/FrameSamples, err)
		}
		frames = append(frames, f)
	}
	return frames
}

func TestExternalSampleQualityPitchWindowDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_WINDOW") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_WINDOW=1 to sweep quality normalized pitch windows")
	}
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
	ref := src[:originalSamples]

	prevWindow := qualityNormalizedPitchSearchHalfWindow
	defer func() { qualityNormalizedPitchSearchHalfWindow = prevWindow }()

	tmp := t.TempDir()
	windows := []int{3, 5, 10, 20, 40, 60, 80, 123}
	t.Logf("external sample quality pitch-window diagnostic: %s", path)
	t.Logf("%-8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Window", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, window := range windows {
		qualityNormalizedPitchSearchHalfWindow = window
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, fmt.Sprintf("win%d", window), encodeBitstreamFrames(t, src), ref, originalSamples)
		t.Logf("%-8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			window, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleQualityPitchHarmonicGuardDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_HARMONIC_GUARD") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_HARMONIC_GUARD=1 to sweep normalized pitch doubled-period guard ratios")
	}
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
	ref := src[:originalSamples]

	prevGuard := qualityNormalizedPitchHarmonicGuardRatio
	defer func() { qualityNormalizedPitchHarmonicGuardRatio = prevGuard }()

	tmp := t.TempDir()
	type mode struct {
		name   string
		ratio  float64
		tuning encoderQualityTuning
	}
	modes := []mode{
		{name: "base", ratio: 0, tuning: encoderQualityTuningAll},
		{name: "g1.01", ratio: 1.01, tuning: encoderQualityTuningAll},
		{name: "g1.03", ratio: 1.03, tuning: encoderQualityTuningAll},
		{name: "g1.05", ratio: 1.05, tuning: encoderQualityTuningAll},
		{name: "g1.10", ratio: 1.10, tuning: encoderQualityTuningAll},
		{name: "g1.25", ratio: 1.25, tuning: encoderQualityTuningAll},
		{name: "g1.50", ratio: 1.50, tuning: encoderQualityTuningAll},
		{name: "g2.00", ratio: 2.00, tuning: encoderQualityTuningAll},
		{name: "g1.25+pc", ratio: 1.25, tuning: encoderQualityTuningAll | encoderTuningPitchClipRepair},
		{name: "g1.50+pc", ratio: 1.50, tuning: encoderQualityTuningAll | encoderTuningPitchClipRepair},
		{name: "g2.00+pc", ratio: 2.00, tuning: encoderQualityTuningAll | encoderTuningPitchClipRepair},
	}
	t.Logf("external sample quality pitch harmonic-guard diagnostic: %s", path)
	t.Logf("%-10s %6s %10s %10s %10s %8s %8s %7s %8s %9s %9s %7s %10s %10s %8s %9s %9s %7s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip",
		"HighDB", "WorstDB", "WorstF", "LocalSNR", "LocalSeg", "LocalNC", "LHighDB", "LWorstDB", "LWorstF")
	for _, mode := range modes {
		qualityNormalizedPitchHarmonicGuardRatio = mode.ratio
		ff, local, ffDecoded, localDecoded := measureExternalSampleFramesQualityPairWithAudio(t, tmp, mode.name, encodeBitstreamFramesWithQualityTuning(t, src, mode.tuning), ref, originalSamples)
		ffNoise := externalResidualNoiseMetricsFor(ref, ffDecoded, ff.shift)
		localNoise := externalResidualNoiseMetricsFor(ref, localDecoded, local.shift)
		t.Logf("%-10s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %9.2f %9.2f %7d %10.2f %10.2f %8d %9.2f %9.2f %7d",
			mode.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, ffNoise.highDB, ffNoise.worstHighDB, ffNoise.worstFrame,
			local.globalSNR, local.segSNR, local.nearClip, localNoise.highDB, localNoise.worstHighDB, localNoise.worstFrame)
	}
}

func TestExternalSampleQualityPitchCenterRescueDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_CENTER_RESCUE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_CENTER_RESCUE=1 to sweep quality pitch centre rescue thresholds")
	}
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
	ref := src[:originalSamples]

	prevRatio := qualityPitchCenterFullSearchMinScoreRatio
	defer func() { qualityPitchCenterFullSearchMinScoreRatio = prevRatio }()

	tmp := t.TempDir()
	ratios := []float64{1.00, 1.03, 1.05, 1.07, 1.08, 1.09, 1.10, 1.25, 1.50, 1.75, 2.00}
	t.Logf("external sample quality pitch-centre rescue diagnostic: %s", path)
	t.Logf("%-8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Ratio", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, ratio := range ratios {
		qualityPitchCenterFullSearchMinScoreRatio = ratio
		name := fmt.Sprintf("center%.2f", ratio)
		frames := encodeBitstreamFrames(t, src)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, name+"-frames", frames, originalSamples)
		nearFrames := externalNearClipFrames(decoded, ff.shift, len(frames), 32700)
		t.Logf("%-8.2f %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			ratio, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		if len(nearFrames) > 0 {
			t.Logf("ratio %.2f near-clip frames: %v", ratio, nearFrames)
		}
	}
}

func TestExternalSampleQualityPitchCenterRescueTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_CENTER_RESCUE_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_CENTER_RESCUE_TRACE=1 to trace quality pitch centre rescue decisions")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	prevRatio := qualityPitchCenterFullSearchMinScoreRatio
	defer func() { qualityPitchCenterFullSearchMinScoreRatio = prevRatio }()

	ratios := []float64{1.00, 1.03, 1.07, 1.08, 1.09, 1.10, 1.75}
	focus := map[int]bool{
		310: true, 311: true,
		330: true, 331: true,
		345: true, 346: true,
		361: true, 362: true,
	}
	t.Logf("external sample quality pitch-centre rescue trace: %s", path)
	t.Logf("%-6s %-6s %7s %7s %7s %9s %7s %7s %6s %6s %8s %8s",
		"ratio", "frame", "centre", "fullT", "winT", "full/win", "pickT", "pickF", "lower", "submul", "rescued", "focus")
	for _, ratio := range ratios {
		qualityPitchCenterFullSearchMinScoreRatio = ratio
		enc := NewEncoder()
		for off := 0; off+FrameSamples <= len(src); off += FrameSamples {
			frame := off / FrameSamples
			if _, err := enc.lpcStep(src[off : off+FrameSamples]); err != nil {
				t.Fatalf("lpcStep frame %d: %v", frame, err)
			}
			_ = enc.openloopStep()

			x, h, _, exc, centre := externalPitchRankSurface(enc, 0)
			fullT, fullScore := enc.bestAdaptivePitchScoreInRange(&x, &h, exc[:], clpitch.PitchMinInt, clpitch.PitchMaxInt, 0)
			kMin, kMax := closedLoopPitchSearchRangeWithHalfWindow(centre, 0, qualityNormalizedPitchSearchHalfWindow)
			winT, windowScore := enc.bestAdaptivePitchScoreInRange(&x, &h, exc[:], kMin, kMax, 0)
			lower := isLowerPitchCenterRescueCandidate(int(centre), int(fullT))
			submul := isPitchSubmultipleForCenter(int(centre), int(fullT))
			rescued := false
			selectedCentre := centre
			if (lower || submul) &&
				!math.IsInf(fullScore, -1) &&
				!math.IsInf(windowScore, -1) &&
				fullScore >= windowScore*qualityPitchCenterFullSearchMinScoreRatio {
				selectedCentre = fullT
				rescued = true
			}
			pickT := enc.searchPitchNormalizedAdaptive(&x, &h, exc[:], selectedCentre, 0)
			pickF := enc.refinePitchNormalizedAdaptive(&x, &h, exc[:], pickT, pickT < 85, 0)

			if focus[frame] || (rescued && frame >= 280 && frame <= 380) {
				scoreRatio := 0.0
				if windowScore > 0 && !math.IsInf(fullScore, -1) && !math.IsInf(windowScore, -1) {
					scoreRatio = fullScore / windowScore
				}
				t.Logf("%-6.2f %-6d %7d %7d %7d %9.3f %7d %7d %6t %6t %8t %8t",
					ratio, frame, centre, fullT, winT, scoreRatio, pickT, pickF, lower, submul, rescued, focus[frame])
			}

			_, _ = enc.closedloopStep(0)
			_, _ = enc.closedloopStep(1)
		}
	}
}

func TestExternalSampleQualityShortPositiveFracDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_SHORT_POSITIVE_FRAC") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_SHORT_POSITIVE_FRAC=1 to sweep quality short-pitch +1/3 fractional guard thresholds")
	}
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
	ref := src[:originalSamples]

	prevRatio := qualityNormalizedPitchShortPositiveFracRatio
	defer func() { qualityNormalizedPitchShortPositiveFracRatio = prevRatio }()

	tmp := t.TempDir()
	ratios := []float64{0, 1.00, 0.995, 0.990, 0.980, 0.970, 0.950, 0.900}
	t.Logf("external sample quality short-pitch positive-frac diagnostic: %s", path)
	t.Logf("%-8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Ratio", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, ratio := range ratios {
		qualityNormalizedPitchShortPositiveFracRatio = ratio
		name := fmt.Sprintf("posfrac%.3f", ratio)
		frames := encodeBitstreamFrames(t, src)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, name+"-frames", frames, originalSamples)
		nearFrames := externalNearClipFrames(decoded, ff.shift, len(frames), 32700)
		t.Logf("%-8.3f %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			ratio, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		if len(nearFrames) > 0 {
			t.Logf("ratio %.3f near-clip frames: %v", ratio, nearFrames)
		}
	}
}

func TestExternalSampleQualityPitchComboDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_COMBO") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_COMBO=1 to sweep quality pitch-centre/short-frac guard combinations")
	}
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
	ref := src[:originalSamples]

	prevCenter := qualityPitchCenterFullSearchMinScoreRatio
	prevFrac := qualityNormalizedPitchShortPositiveFracRatio
	defer func() {
		qualityPitchCenterFullSearchMinScoreRatio = prevCenter
		qualityNormalizedPitchShortPositiveFracRatio = prevFrac
	}()

	type mode struct {
		centerRatio float64
		fracRatio   float64
	}
	modes := []mode{
		{centerRatio: 1.75, fracRatio: 0},
		{centerRatio: 1.75, fracRatio: 0.990},
		{centerRatio: 1.75, fracRatio: 0.970},
		{centerRatio: 1.10, fracRatio: 0},
		{centerRatio: 1.10, fracRatio: 0.990},
		{centerRatio: 1.08, fracRatio: 0},
		{centerRatio: 1.08, fracRatio: 0.990},
		{centerRatio: 1.08, fracRatio: 0.970},
		{centerRatio: 1.03, fracRatio: 0},
		{centerRatio: 1.03, fracRatio: 0.990},
	}

	tmp := t.TempDir()
	t.Logf("external sample quality pitch combo diagnostic: %s", path)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		qualityPitchCenterFullSearchMinScoreRatio = mode.centerRatio
		qualityNormalizedPitchShortPositiveFracRatio = mode.fracRatio
		name := fmt.Sprintf("c%.2f-f%.3f", mode.centerRatio, mode.fracRatio)
		frames := encodeBitstreamFrames(t, src)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, name+"-frames", frames, originalSamples)
		nearFrames := externalNearClipFrames(decoded, ff.shift, len(frames), 32700)
		t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		if len(nearFrames) > 0 {
			t.Logf("%s near-clip frames: %v", name, nearFrames)
		}
	}
}

func TestExternalSampleQualityGainClipThresholdDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_CLIP_THRESHOLD") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_CLIP_THRESHOLD=1 to sweep quality gain clip repair thresholds")
	}
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
	ref := src[:originalSamples]

	prevThreshold := qualityGainClipRepairThreshold
	defer func() { qualityGainClipRepairThreshold = prevThreshold }()

	tmp := t.TempDir()
	thresholds := []int{32400, 32300, 32200, 32100, 32000, 31800, 31600}
	t.Logf("external sample quality gain clip-threshold diagnostic: %s", path)
	t.Logf("%-8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Thresh", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, threshold := range thresholds {
		qualityGainClipRepairThreshold = threshold
		name := fmt.Sprintf("clip%d", threshold)
		frames := encodeBitstreamFrames(t, src)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, name+"-frames", frames, originalSamples)
		nearFrames := externalNearClipFrames(decoded, ff.shift, len(frames), 32700)
		t.Logf("%-8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			threshold, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		if len(nearFrames) > 0 {
			t.Logf("threshold %d near-clip frames: %v", threshold, nearFrames)
		}
	}
}

func TestExternalSampleQualityGainMSEThresholdDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_MSE_THRESHOLD") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_MSE_THRESHOLD=1 to sweep quality gain MSE repair thresholds")
	}
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
	ref := src[:originalSamples]

	prevThreshold := qualityGainMSERepairThreshold
	defer func() { qualityGainMSERepairThreshold = prevThreshold }()

	tmp := t.TempDir()
	startFrame, endFrame := externalQualityWindowFrameRange()
	thresholds := []int{22000, 23000, 23200, 23400, 23600, 23800, 24000, 25000, 26000, 26200, 26400, 26600, 26800, 27000, 28000}
	t.Logf("external sample quality gain-MSE-threshold diagnostic: %s", path)
	t.Logf("window frames=%d..%d (%.3fs..%.3fs at 8 kHz)", startFrame, endFrame,
		float64(startFrame*FrameSamples)/8000.0, float64((endFrame+1)*FrameSamples)/8000.0)
	t.Logf("%-9s %6s %10s %10s %10s %8s %8s %7s %8s %8s %7s %10s %10s %8s",
		"Threshold", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "wSNR", "wNear", "LocalSNR", "LocalSeg", "LocalNC")
	for _, threshold := range thresholds {
		qualityGainMSERepairThreshold = threshold
		name := fmt.Sprintf("mse%d", threshold)
		frames := encodeBitstreamFrames(t, src)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, name+"-frames", frames, originalSamples)
		window := externalAlignedWindowQualityMetrics(ref, decoded, ff.shift, startFrame, endFrame, 32700)
		t.Logf("%-9d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %8.2f %7d %10.2f %10.2f %8d",
			threshold, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, window.snr, window.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleQualityClippedInputModeDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CLIPPED_INPUT_MODE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CLIPPED_INPUT_MODE=1 to sweep clipped-input quality tuning modes")
	}
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
	ref := src[:originalSamples]

	type mode struct {
		name      string
		threshold int
		cooldown  int
		tuning    encoderQualityTuning
	}
	wideNoGain := (encoderQualityTuningAll &^ encoderTuningGainSearchBias) | encoderTuningWideGainPredictor
	noFCB := encoderQualityTuningAll &^ encoderTuningFCBThresholdScan
	modes := []mode{
		{name: "current", threshold: 0, cooldown: 0, tuning: encoderQualityTuningAll},
		{name: "core32700-c20", threshold: 32700, cooldown: 20, tuning: 0},
		{name: "core32700-c40", threshold: 32700, cooldown: 40, tuning: 0},
		{name: "core32700-c60", threshold: 32700, cooldown: 60, tuning: 0},
		{name: "nofcb32700-c10", threshold: 32700, cooldown: 10, tuning: noFCB},
		{name: "nofcb32700-c20", threshold: 32700, cooldown: 20, tuning: noFCB},
		{name: "nofcb32400-c20", threshold: 32400, cooldown: 20, tuning: noFCB},
		{name: "nofcb32000-c20", threshold: 32000, cooldown: 20, tuning: noFCB},
		{name: "clip32700-c20", threshold: 32700, cooldown: 20, tuning: wideNoGain},
		{name: "clip32700-c40", threshold: 32700, cooldown: 40, tuning: wideNoGain},
		{name: "clip32700-c60", threshold: 32700, cooldown: 60, tuning: wideNoGain},
		{name: "clip32400-c40", threshold: 32400, cooldown: 40, tuning: wideNoGain},
		{name: "clip32000-c40", threshold: 32000, cooldown: 40, tuning: wideNoGain},
	}

	tmp := t.TempDir()
	t.Logf("external sample clipped-input quality tuning diagnostic: %s", path)
	t.Logf("%-14s %8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "swFrames", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		frames, switched := encodeBitstreamFramesClippedInputMode(t, src, mode.threshold, mode.cooldown, mode.tuning)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, mode.name+"-frames", frames, originalSamples)
		nearFrames := externalNearClipFrames(decoded, ff.shift, len(frames), 32700)
		t.Logf("%-14s %8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			mode.name, switched, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
		if len(nearFrames) > 0 {
			t.Logf("%s near-clip frames: %v", mode.name, nearFrames)
		}
	}
}

func TestExternalSampleClippedOpenLoopTopVariantDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CLIPPED_OPENLOOP_TOP_VARIANT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CLIPPED_OPENLOOP_TOP_VARIANT=1 to sweep clipped-input open-loop T_op choices")
	}
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
	ref := src[:originalSamples]

	type mode struct {
		name      string
		threshold int
		cooldown  int
		variant   externalOpenLoopTopVariant
	}
	modes := []mode{
		{name: "current"},
		{name: "r2c95-32700-c1", threshold: 32700, cooldown: 1, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c2", threshold: 32700, cooldown: 2, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c5", threshold: 32700, cooldown: 5, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c10", threshold: 32700, cooldown: 10, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c20", threshold: 32700, cooldown: 20, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "low95-32700-c5", threshold: 32700, cooldown: 5, variant: externalOpenLoopTopVariant{mode: "low-close:0.95"}},
		{name: "best108-32700-c5", threshold: 32700, cooldown: 5, variant: externalOpenLoopTopVariant{mode: "best-margin:1.08"}},
	}

	tmp := t.TempDir()
	t.Logf("external sample clipped-input open-loop T_op variant diagnostic: %s", path)
	t.Logf("%-17s %8s %8s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "swFrames", "chgFrames", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		frames, switched, changed := encodeBitstreamFramesClippedOpenLoopTopVariant(t, src, mode.threshold, mode.cooldown, mode.variant)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-17s %8d %8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			mode.name, switched, changed, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleFFmpegPatchMatrixDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FFMPEG_PATCH_MATRIX") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FFMPEG_PATCH_MATRIX=1 to test limited bitstream patches against ffmpeg black-box decode")
	}
	runExternalSampleFFmpegPatchMatrixDiagnostic(t, "quality", EncoderProfileQuality)
}

func TestExternalSampleCoreFFmpegPatchMatrixDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CORE_FFMPEG_PATCH_MATRIX") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CORE_FFMPEG_PATCH_MATRIX=1 to test limited Core bitstream patches against ffmpeg black-box decode")
	}
	runExternalSampleFFmpegPatchMatrixDiagnostic(t, "core", EncoderProfileCore)
}

func runExternalSampleFFmpegPatchMatrixDiagnostic(t *testing.T, label string, profile EncoderProfile) {
	t.Helper()
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	frames := encodeBitstreamFramesWithProfile(t, src, profile)
	baseDecoded := decodeExternalFramesWithFFmpeg(t, tmp, "base", frames, originalSamples)
	baseMetrics := externalQualityMetricsFor(ref, baseDecoded, 240)
	targets := externalNearClipFrames(baseDecoded, baseMetrics.shift, len(frames), 32700)
	if len(targets) == 0 {
		t.Fatalf("%s base ffmpeg decode has no near-clip frames; metrics nearClip=%d", label, baseMetrics.nearClip)
	}

	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))

	var patches []externalFFmpegBitstreamPatch
	for _, frameIndex := range targets {
		for sub := 0; sub < 2; sub++ {
			frameIndex := frameIndex
			sub := sub
			for _, cand := range externalGainPatchCandidates(frames[frameIndex], sub, 12) {
				cand := cand
				patches = append(patches, externalFFmpegBitstreamPatch{
					name:  cand.name,
					frame: frameIndex,
					sub:   sub,
					apply: func(out []bitstream.Frame) {
						setExternalSubframeGain(&out[frameIndex], sub, cand.ga, cand.gb)
					},
				})
			}
			for _, cand := range externalPitchPatchCandidates(frames[frameIndex], sub) {
				cand := cand
				patches = append(patches, externalFFmpegBitstreamPatch{
					name:  cand.name,
					frame: frameIndex,
					sub:   sub,
					apply: func(out []bitstream.Frame) {
						setExternalSubframePitch(&out[frameIndex], sub, cand.intLag, cand.frac)
					},
				})
			}
			if frameIndex < len(bcgFrames) {
				patches = append(patches, externalBCGDonorPatches(frames, bcgFrames, frameIndex, sub)...)
			}
		}
	}

	baseWindow := externalDecodeWindowScoreForTargets(ref, baseDecoded, baseMetrics.shift, targets, 32700)
	results := make([]externalFFmpegPatchResult, 0, len(patches))
	for i, p := range patches {
		candFrames := cloneFrames(frames)
		p.apply(candFrames)
		decoded := decodeExternalFramesWithFFmpeg(t, tmp, fmt.Sprintf("patch-%03d", i), candFrames, originalSamples)
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		window := externalDecodeWindowScoreForTargets(ref, decoded, metrics.shift, targets, 32700)
		if metrics.nearClip < baseMetrics.nearClip || window.nearClip < baseWindow.nearClip || metrics.globalSNR >= baseMetrics.globalSNR {
			results = append(results, externalFFmpegPatchResult{
				name:       p.name,
				frame:      p.frame,
				sub:        p.sub,
				patchIndex: i,
				metrics:    metrics,
				window:     window,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].metrics.nearClip != results[j].metrics.nearClip {
			return results[i].metrics.nearClip < results[j].metrics.nearClip
		}
		if results[i].window.nearClip != results[j].window.nearClip {
			return results[i].window.nearClip < results[j].window.nearClip
		}
		return results[i].metrics.globalSNR > results[j].metrics.globalSNR
	})

	pairLimit := 24
	if len(results) < pairLimit {
		pairLimit = len(results)
	}
	var pairResults []externalFFmpegPatchResult
	for i := 0; i < pairLimit; i++ {
		for j := i + 1; j < pairLimit; j++ {
			a := patches[results[i].patchIndex]
			b := patches[results[j].patchIndex]
			if a.frame == b.frame && a.sub == b.sub {
				continue
			}
			candFrames := cloneFrames(frames)
			a.apply(candFrames)
			b.apply(candFrames)
			decoded := decodeExternalFramesWithFFmpeg(t, tmp, fmt.Sprintf("pair-%02d-%02d", i, j), candFrames, originalSamples)
			metrics := externalQualityMetricsFor(ref, decoded, 240)
			window := externalDecodeWindowScoreForTargets(ref, decoded, metrics.shift, targets, 32700)
			if metrics.nearClip < baseMetrics.nearClip || window.nearClip < baseWindow.nearClip || metrics.globalSNR >= baseMetrics.globalSNR {
				pairResults = append(pairResults, externalFFmpegPatchResult{
					name:    externalPatchLabel(a) + "+" + externalPatchLabel(b),
					frame:   -1,
					sub:     -1,
					metrics: metrics,
					window:  window,
				})
			}
		}
	}
	sort.SliceStable(pairResults, func(i, j int) bool {
		if pairResults[i].metrics.nearClip != pairResults[j].metrics.nearClip {
			return pairResults[i].metrics.nearClip < pairResults[j].metrics.nearClip
		}
		if pairResults[i].window.nearClip != pairResults[j].window.nearClip {
			return pairResults[i].window.nearClip < pairResults[j].window.nearClip
		}
		return pairResults[i].metrics.globalSNR > pairResults[j].metrics.globalSNR
	})

	t.Logf("external sample %s ffmpeg patch-matrix diagnostic: %s", label, path)
	t.Logf("targets=%v patches=%d kept=%d pairKept=%d", targets, len(patches), len(results), len(pairResults))
	t.Logf("%-24s %6s %10s %10s %10s %8s %8s %7s %8s %7s %8s %10s",
		"Patch", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "wPeak", "wNear", "wMSE")
	t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %7d %8d %10.0f",
		"base", baseMetrics.shift, baseMetrics.rms, baseMetrics.globalSNR, baseMetrics.segSNR, baseMetrics.corr, baseMetrics.rmsRatio,
		baseMetrics.peak, baseMetrics.nearClip, baseWindow.peak, baseWindow.nearClip, baseWindow.mse)
	limit := 40
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		r := results[i]
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %7d %8d %10.0f frame=%d sub=%d",
			r.name, r.metrics.shift, r.metrics.rms, r.metrics.globalSNR, r.metrics.segSNR, r.metrics.corr, r.metrics.rmsRatio,
			r.metrics.peak, r.metrics.nearClip, r.window.peak, r.window.nearClip, r.window.mse, r.frame, r.sub)
	}
	pairLogLimit := 20
	if len(pairResults) < pairLogLimit {
		pairLogLimit = len(pairResults)
	}
	for i := 0; i < pairLogLimit; i++ {
		r := pairResults[i]
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %7d %8d %10.0f",
			"pair:"+r.name, r.metrics.shift, r.metrics.rms, r.metrics.globalSNR, r.metrics.segSNR, r.metrics.corr, r.metrics.rmsRatio,
			r.metrics.peak, r.metrics.nearClip, r.window.peak, r.window.nearClip, r.window.mse)
	}
}

func TestExternalSampleFFmpegPitchPatchTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FFMPEG_PITCH_PATCH_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FFMPEG_PITCH_PATCH_TRACE=1 to trace pitch-only patches against ffmpeg black-box decode")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	frames := encodeBitstreamFrames(t, src)
	baseDecoded := decodeExternalFramesWithFFmpeg(t, tmp, "pitch-trace-base", frames, originalSamples)
	baseMetrics := externalQualityMetricsFor(ref, baseDecoded, 240)
	targets := externalNearClipFrames(baseDecoded, baseMetrics.shift, len(frames), 32700)
	if len(targets) == 0 {
		t.Fatalf("base ffmpeg decode has no near-clip frames; metrics nearClip=%d", baseMetrics.nearClip)
	}
	traceFrames := externalFrameNeighborhood(targets, len(frames), 1)
	baseWindow := externalDecodeWindowScoreForTargets(ref, baseDecoded, baseMetrics.shift, targets, 32700)

	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))

	t.Logf("external sample ffmpeg pitch-patch trace: %s", path)
	t.Logf("targets=%v traceFrames=%v", targets, traceFrames)
	t.Logf("%-24s %6s %10s %10s %10s %8s %8s %7s %8s %7s %8s %10s",
		"Patch", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "wPeak", "wNear", "wMSE")
	t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %7d %8d %10.0f",
		"base", baseMetrics.shift, baseMetrics.rms, baseMetrics.globalSNR, baseMetrics.segSNR, baseMetrics.corr, baseMetrics.rmsRatio,
		baseMetrics.peak, baseMetrics.nearClip, baseWindow.peak, baseWindow.nearClip, baseWindow.mse)

	t.Logf("%-6s %-3s %8s %8s %8s %8s %8s %8s %8s %8s",
		"frame", "sub", "ourT", "ourF", "bcgT", "bcgF", "ourGA", "ourGB", "bcgGA", "bcgGB")
	for _, frameIndex := range traceFrames {
		for sub := 0; sub < 2; sub++ {
			ourT, ourF, _, _, ourGA, ourGB := externalSubframeFields(frames, frameIndex, sub)
			bcgT, bcgF, _, _, bcgGA, bcgGB := externalSubframeFields(bcgFrames, frameIndex, sub)
			t.Logf("%-6d %-3d %8d %8d %8d %8d %8d %8d %8d %8d",
				frameIndex, sub, ourT, ourF, bcgT, bcgF, ourGA, ourGB, bcgGA, bcgGB)
		}
	}

	var results []externalPitchPatchTraceResult
	for _, frameIndex := range traceFrames {
		for sub := 0; sub < 2; sub++ {
			ourT, ourF, _, _, _, _ := externalSubframeFields(frames, frameIndex, sub)
			bcgT, bcgF, _, _, _, _ := externalSubframeFields(bcgFrames, frameIndex, sub)
			patches := make([]externalFFmpegBitstreamPatch, 0, 18)
			for _, cand := range externalPitchPatchCandidates(frames[frameIndex], sub) {
				cand := cand
				fidx := frameIndex
				sidx := sub
				patches = append(patches, externalFFmpegBitstreamPatch{
					name:  cand.name,
					frame: fidx,
					sub:   sidx,
					apply: func(out []bitstream.Frame) {
						setExternalSubframePitch(&out[fidx], sidx, cand.intLag, cand.frac)
					},
				})
			}
			if bcgT >= 0 && (bcgT != ourT || bcgF != ourF) {
				fidx := frameIndex
				sidx := sub
				patches = append(patches, externalFFmpegBitstreamPatch{
					name:  fmt.Sprintf("bcg-pitch-%d/%d", bcgT, bcgF),
					frame: fidx,
					sub:   sidx,
					apply: func(out []bitstream.Frame) {
						copyExternalSubframePitch(&out[fidx], bcgFrames[fidx], sidx)
					},
				})
			}

			for i, patch := range patches {
				candFrames := cloneFrames(frames)
				patch.apply(candFrames)
				newT, newF, _, _, _, _ := externalSubframeFields(candFrames, frameIndex, sub)
				decoded := decodeExternalFramesWithFFmpeg(t, tmp, fmt.Sprintf("pitch-trace-%03d-%02d", len(results), i), candFrames, originalSamples)
				metrics := externalQualityMetricsFor(ref, decoded, 240)
				window := externalDecodeWindowScoreForTargets(ref, decoded, metrics.shift, targets, 32700)
				if metrics.nearClip < baseMetrics.nearClip ||
					window.nearClip < baseWindow.nearClip ||
					window.mse < baseWindow.mse*0.98 ||
					metrics.globalSNR >= baseMetrics.globalSNR {
					results = append(results, externalPitchPatchTraceResult{
						name:    patch.name,
						frame:   frameIndex,
						sub:     sub,
						oldT:    ourT,
						oldF:    ourF,
						newT:    newT,
						newF:    newF,
						bcgT:    bcgT,
						bcgF:    bcgF,
						metrics: metrics,
						window:  window,
					})
				}
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].metrics.nearClip != results[j].metrics.nearClip {
			return results[i].metrics.nearClip < results[j].metrics.nearClip
		}
		if results[i].window.nearClip != results[j].window.nearClip {
			return results[i].window.nearClip < results[j].window.nearClip
		}
		if results[i].window.mse != results[j].window.mse {
			return results[i].window.mse < results[j].window.mse
		}
		return results[i].metrics.globalSNR > results[j].metrics.globalSNR
	})

	limit := 60
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		r := results[i]
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %7d %8d %10.0f frame=%d sub=%d old=%d/%d new=%d/%d bcg=%d/%d",
			r.name, r.metrics.shift, r.metrics.rms, r.metrics.globalSNR, r.metrics.segSNR, r.metrics.corr, r.metrics.rmsRatio,
			r.metrics.peak, r.metrics.nearClip, r.window.peak, r.window.nearClip, r.window.mse,
			r.frame, r.sub, r.oldT, r.oldF, r.newT, r.newF, r.bcgT, r.bcgF)
	}
}

func TestExternalSampleQualityPitchClipScoreTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_CLIP_SCORE_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_CLIP_SCORE_TRACE=1 to trace quality pitch clip-repair scores")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	targets := map[int]bool{
		297: true,
		298: true,
		299: true,
		300: true,
	}
	enc := NewEncoder()
	enc.qualityTuning &^= encoderTuningPitchClipRepair
	t.Logf("external sample quality pitch clip-score trace: %s", path)
	t.Logf("%-6s %-3s %-16s %7s %7s %6s %6s %9s %9s %12s",
		"frame", "sub", "candidate", "T", "frac", "hard", "near", "scorePk", "refPk", "mse")
	for off := 0; off+FrameSamples <= len(src); off += FrameSamples {
		frameIndex := off / FrameSamples
		if _, err := enc.lpcStep(src[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			aHat, sFrame, x, h, excSearch, intLag, frac := externalQualityPitchClipSurface(t, enc, sub)
			if targets[frameIndex] {
				logExternalQualityPitchClipScore(t, enc, frameIndex, sub, "base", intLag, frac, aHat, sFrame, x, h, excSearch[:])
				var candidates [18]encoderPitchClipCandidate
				count := qualityPitchClipCandidates(sub, enc.intT1, intLag, frac, &candidates)
				for i := 0; i < count; i++ {
					cand := candidates[i]
					name := fmt.Sprintf("cand-%02d", i)
					logExternalQualityPitchClipScore(t, enc, frameIndex, sub, name, cand.intLag, cand.frac, aHat, sFrame, x, h, excSearch[:])
				}
			}
			enc.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, excSearch[:], intLag, frac)
		}
	}
}

func externalQualityPitchClipSurface(
	t *testing.T,
	e *Encoder,
	sub int,
) (
	aHat *[lpc.LPCOrder + 1]int16,
	sFrame *[clpitch.SubframeLen]int16,
	x [clpitch.SubframeLen]int16,
	h [clpitch.SubframeLen]int16,
	excSearch [closedLoopPitchSearchLen]int16,
	intLag int16,
	frac int8,
) {
	t.Helper()
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	if e.qualityEarlyClosedLoopSpeechWindowEnabled() {
		sStart = 80 + 40*sub
	}
	sFrame = (*[clpitch.SubframeLen]int16)(e.oldSpeech[sStart : sStart+clpitch.SubframeLen])

	var r, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	if e.qualityPitchCenterCandidateEnabled() {
		centre = e.qualityPitchCenterCandidate(&x, &h, exc, centre, sub)
	}
	if e.qualityNormalizedAdaptivePitchSearchEnabled() {
		intLag = e.searchPitchNormalizedAdaptive(&x, &h, exc, centre, sub)
		frac = e.refinePitchNormalizedAdaptive(&x, &h, exc, intLag, sub == 1 || intLag < 85, sub)
	} else {
		intLag, _ = clpitch.SearchInteger(&xb, exc, centre, sub)
		intLag, frac = refineProductionPitchFraction(&xb, exc, sub, intLag, e.intT1)
	}
	return aHat, sFrame, x, h, excSearch, intLag, frac
}

func logExternalQualityPitchClipScore(
	t *testing.T,
	e *Encoder,
	frameIndex int,
	sub int,
	name string,
	intLag int16,
	frac int8,
	aHat *[lpc.LPCOrder + 1]int16,
	sFrame *[clpitch.SubframeLen]int16,
	x, h [clpitch.SubframeLen]int16,
	exc []int16,
) {
	t.Helper()
	cand := *e
	cand.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, exc, intLag, frac)
	ref := *sFrame
	pcm.ScaleUpSat(ref[:], ref[:])
	refPeak, _ := externalPeakAndNearClip(ref[:])
	score := cand.qualityLastOutput
	t.Logf("%-6d %-3d %-16s %7d %7d %6d %6d %9d %9d %12d",
		frameIndex, sub, name, intLag, frac, score.hardClip, score.nearClip, score.peak, refPeak, score.mse)
}

func externalPatchLabel(p externalFFmpegBitstreamPatch) string {
	return fmt.Sprintf("f%d/s%d:%s", p.frame, p.sub, p.name)
}

// TestExternalSampleBCGHybridDiagnostic compares the local encoder against a
// locally installed bcg729 executable used strictly as a black-box encoder. It
// does not inspect or import external implementation code. The diagnostic
// swaps transmitted field families between the two bitstreams and decodes the
// result with FFmpeg as a black-box decoder to localize where user-sample
// quality diverges.
func TestExternalSampleBCGHybridDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_HYBRID") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_HYBRID=1 to run user-sample bcg729 black-box hybrid diagnostic")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	profileName := strings.ToLower(strings.TrimSpace(os.Getenv("G729_EXTERNAL_SAMPLE_BCG_HYBRID_PROFILE")))
	profile := EncoderProfileQuality
	profileLabel := "quality"
	if profileName == "core" {
		profile = EncoderProfileCore
		profileLabel = "core"
	}
	writeOurEncodedRawG729WithProfile(t, src, ourRawPath, profile)
	writeBCGEncodedRawG729(t, src, bcgRawPath)

	ourRaw := readFile(t, ourRawPath)
	bcgRaw := readFile(t, bcgRawPath)
	ourFrames := readRawG729Frames(t, ourRaw)
	bcgFrames := readRawG729Frames(t, bcgRaw)
	if len(ourFrames) != len(bcgFrames) {
		t.Fatalf("frame count mismatch: our=%d bcg=%d", len(ourFrames), len(bcgFrames))
	}

	ref := src[:originalSamples]
	type mode struct {
		name   string
		frames string
		base   string
		fam    string
	}
	modes := []mode{
		{name: "bcg all", frames: "bcg"},
		{name: "our all", frames: "our"},
		{name: "bcg + our LSP", base: "bcg", fam: "lsp"},
		{name: "bcg + our pitch", base: "bcg", fam: "pitch"},
		{name: "bcg + our FCB", base: "bcg", fam: "fcb"},
		{name: "bcg + our gain", base: "bcg", fam: "gain"},
		{name: "bcg + our FCB+gain", base: "bcg", fam: "fcb+gain"},
		{name: "bcg + our nonLSP", base: "bcg", fam: "pitch+fcb+gain"},
		{name: "our + bcg LSP", base: "our", fam: "lsp"},
		{name: "our + bcg pitch", base: "our", fam: "pitch"},
		{name: "our + bcg FCB", base: "our", fam: "fcb"},
		{name: "our + bcg gain", base: "our", fam: "gain"},
		{name: "our + bcg FCB+gain", base: "our", fam: "fcb+gain"},
		{name: "our + bcg nonLSP", base: "our", fam: "pitch+fcb+gain"},
		{name: "our + bcg LSP+nonLSP", base: "our", fam: "lsp+pitch+fcb+gain"},
	}

	t.Logf("external sample bcg729 black-box hybrid diagnostic: %s", path)
	t.Logf("our profile=%s", profileLabel)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(ourFrames))
	t.Logf("%-24s %6s %10s %10s %10s %8s %8s %7s %8s %10s %8s",
		"Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "vsBCG SNR", "vsBCG r")

	var bcgDecoded []int16
	decodedByName := make(map[string][]int16, len(modes))
	for _, m := range modes {
		var framesOut []bitstream.Frame
		switch m.frames {
		case "bcg":
			framesOut = bcgFrames
		case "our":
			framesOut = ourFrames
		default:
			if m.base == "our" {
				framesOut = makeHybridFrames(bcgFrames, ourFrames, "our", m.fam)
			} else {
				framesOut = makeHybridFrames(bcgFrames, ourFrames, "ref", m.fam)
			}
		}
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s: decoded output too short: got %d want >= %d", m.name, len(decoded), originalSamples)
		}
		if m.name == "bcg all" {
			bcgDecoded = decoded
		}
		decodedByName[m.name] = decoded
	}
	if bcgDecoded == nil {
		t.Fatal("missing bcg decoded baseline")
	}
	for _, m := range modes {
		decoded := decodedByName[m.name]
		q := externalQualityMetricsFor(ref, decoded, 240)
		vsBCGMetrics := externalQualityMetricsFor(bcgDecoded, decoded, 240)
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %8.4f",
			m.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip, vsBCGMetrics.globalSNR, vsBCGMetrics.corr)
	}
	logExternalBCGFieldAgreement(t, ourFrames, bcgFrames)
}

func TestExternalSampleSpectralTiltDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_SPECTRAL_TILT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_SPECTRAL_TILT=1 to compare source/core/quality/bcg spectral shape")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	type stream struct {
		name           string
		profile        EncoderProfile
		tuning         encoderQualityTuning
		explicitTuning bool
		bcg            bool
	}
	streams := []stream{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "bcg", bcg: true},
	}
	if os.Getenv("G729_EXTERNAL_SAMPLE_SPECTRAL_ABLATION") == "1" {
		streams = append(streams,
			stream{name: "fcb+wide", profile: EncoderProfileCore, tuning: encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor, explicitTuning: true},
			stream{name: "nativegain+mse", profile: EncoderProfileCore, tuning: encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair, explicitTuning: true},
			stream{name: "norm+native+mse", profile: EncoderProfileCore, tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair, explicitTuning: true},
			stream{name: "quality-no-fcb", profile: EncoderProfileCore, tuning: encoderQualityTuningAll &^ encoderTuningFCBThresholdScan, explicitTuning: true},
			stream{name: "quality-no-clip", profile: EncoderProfileCore, tuning: encoderQualityTuningAll &^ encoderTuningGainClipRepair, explicitTuning: true},
			stream{name: "quality+lspx", profile: EncoderProfileCore, tuning: encoderQualityTuningAll | encoderTuningExpandedLSPSearch, explicitTuning: true},
		)
	}

	type result struct {
		name    string
		metrics externalQualityMetrics
		profile externalSpectralProfile
	}
	results := []result{{
		name:    "source",
		profile: externalSpectralProfileFor(ref),
	}}
	for _, s := range streams {
		rawPath := filepath.Join(tmp, s.name+".g729")
		pcmPath := filepath.Join(tmp, s.name+".s16le")
		if s.bcg {
			writeBCGEncodedRawG729(t, src, rawPath)
		} else if s.explicitTuning {
			writeOurEncodedRawG729WithTuning(t, src, rawPath, s.profile, s.tuning)
		} else {
			writeOurEncodedRawG729WithProfile(t, src, rawPath, s.profile)
		}
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s: ffmpeg decoded output too short: got %d want >= %d", s.name, len(decoded), originalSamples)
		}
		m := externalQualityMetricsFor(ref, decoded, 240)
		results = append(results, result{
			name:    s.name,
			metrics: m,
			profile: externalSpectralProfileFor(alignByShift(ref, decoded, m.shift)),
		})
	}

	t.Logf("external sample spectral tilt diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-10s %6s %10s %10s %10s %8s %8s %7s %8s",
		"Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, r := range results[1:] {
		t.Logf("%-10s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			r.name, r.metrics.shift, r.metrics.rms, r.metrics.globalSNR, r.metrics.segSNR,
			r.metrics.corr, r.metrics.rmsRatio, r.metrics.peak, r.metrics.nearClip)
	}

	srcProf := results[0].profile
	coreProf := results[1].profile
	qualityProf := results[2].profile
	bcgProf := results[3].profile
	t.Logf("%-12s %9s %9s %9s %9s %9s %9s",
		"BandHz", "srcShare", "core-src", "qual-src", "bcg-src", "qual-bcg", "core-bcg")
	for i, band := range externalSpectralBands {
		t.Logf("%-12s %9.2f %9.2f %9.2f %9.2f %9.2f %9.2f",
			band.name,
			srcProf.shareDB(i),
			coreProf.shareDB(i)-srcProf.shareDB(i),
			qualityProf.shareDB(i)-srcProf.shareDB(i),
			bcgProf.shareDB(i)-srcProf.shareDB(i),
			qualityProf.shareDB(i)-bcgProf.shareDB(i),
			coreProf.shareDB(i)-bcgProf.shareDB(i),
		)
	}

	for _, tilt := range []struct {
		name string
		hi   []int
		mid  []int
	}{
		{name: "presence/mid", hi: []int{3, 4, 5}, mid: []int{1, 2}},
		{name: "air/mid", hi: []int{4, 5}, mid: []int{1, 2}},
	} {
		srcTilt := srcProf.ratioDB(tilt.hi, tilt.mid)
		coreTilt := coreProf.ratioDB(tilt.hi, tilt.mid)
		qualityTilt := qualityProf.ratioDB(tilt.hi, tilt.mid)
		bcgTilt := bcgProf.ratioDB(tilt.hi, tilt.mid)
		t.Logf("tilt %-12s src=%7.2f core=%7.2f quality=%7.2f bcg=%7.2f quality-src=%+7.2f quality-bcg=%+7.2f",
			tilt.name, srcTilt, coreTilt, qualityTilt, bcgTilt, qualityTilt-srcTilt, qualityTilt-bcgTilt)
	}
	if os.Getenv("G729_EXTERNAL_SAMPLE_SPECTRAL_ABLATION") == "1" {
		t.Logf("%-16s %12s %12s %12s %12s", "Pipeline", "presence-src", "presence-bcg", "air-src", "air-bcg")
		bcgPresence := bcgProf.ratioDB([]int{3, 4, 5}, []int{1, 2})
		bcgAir := bcgProf.ratioDB([]int{4, 5}, []int{1, 2})
		srcPresence := srcProf.ratioDB([]int{3, 4, 5}, []int{1, 2})
		srcAir := srcProf.ratioDB([]int{4, 5}, []int{1, 2})
		for _, r := range results[1:] {
			presence := r.profile.ratioDB([]int{3, 4, 5}, []int{1, 2})
			air := r.profile.ratioDB([]int{4, 5}, []int{1, 2})
			t.Logf("%-16s %+12.2f %+12.2f %+12.2f %+12.2f",
				r.name, presence-srcPresence, presence-bcgPresence, air-srcAir, air-bcgAir)
		}
	}
}

func TestExternalSampleLSPTopKDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_LSP_TOPK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_LSP_TOPK=1 to run user-sample expanded LSP VQ diagnostic")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	type mode struct {
		name        string
		topK        int
		impliedInit bool
	}
	modes := []mode{
		{name: "current", topK: 0},
		{name: "lsp implied init", impliedInit: true},
		{name: "lsp top1 pair", topK: 1},
		{name: "lsp top2 pair", topK: 2},
		{name: "lsp top4 pair", topK: 4},
		{name: "lsp top8 pair", topK: 8},
		{name: "lsp top16 pair", topK: 16},
	}

	t.Logf("external sample expanded LSP VQ diagnostic: %s", path)
	t.Logf("samples=%d padded=%d frames=%d", originalSamples, len(src)-originalSamples, len(src)/FrameSamples)
	t.Logf("%-16s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, m := range modes {
		var frames []bitstream.Frame
		if m.impliedInit {
			frames = encodeBitstreamFramesLSPInitialMemory(t, src, externalLSPImpliedUniformInitMemory())
		} else if m.topK == 0 {
			frames = encodeBitstreamFrames(t, src)
		} else {
			frames = encodeBitstreamFramesLSPTopK(t, src, m.topK)
		}
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".s16le")
		writePackedFrames(t, frames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s: decoded output too short: got %d want >= %d", m.name, len(decoded), originalSamples)
		}
		q := externalQualityMetricsFor(ref, decoded, 240)
		t.Logf("%-16s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			m.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
	}
}

func TestExternalSampleFrameFamilyShiftDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FRAME_SHIFT") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FRAME_SHIFT=1 to run user-sample transmitted-field frame-shift diagnostic")
	}
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
	ref := src[:originalSamples]
	ourFrames := encodeBitstreamFrames(t, src)

	type mode struct {
		name   string
		frames []bitstream.Frame
	}
	modes := []mode{
		{name: "current", frames: ourFrames},
		{name: "all prev", frames: shiftFrameSequence(ourFrames, -1)},
		{name: "all next", frames: shiftFrameSequence(ourFrames, +1)},
		{name: "LSP prev", frames: shiftFrameFamily(ourFrames, -1, "lsp")},
		{name: "LSP next", frames: shiftFrameFamily(ourFrames, +1, "lsp")},
		{name: "nonLSP prev", frames: shiftFrameFamily(ourFrames, -1, "pitch+fcb+gain")},
		{name: "nonLSP next", frames: shiftFrameFamily(ourFrames, +1, "pitch+fcb+gain")},
		{name: "pitch prev", frames: shiftFrameFamily(ourFrames, -1, "pitch")},
		{name: "pitch next", frames: shiftFrameFamily(ourFrames, +1, "pitch")},
		{name: "gain prev", frames: shiftFrameFamily(ourFrames, -1, "gain")},
		{name: "gain next", frames: shiftFrameFamily(ourFrames, +1, "gain")},
	}

	tmp := t.TempDir()
	t.Logf("external sample transmitted-field frame-shift diagnostic: %s", path)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, m := range modes {
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".s16le")
		writePackedFrames(t, m.frames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s: decoded output too short: got %d want >= %d", m.name, len(decoded), originalSamples)
		}
		q := externalQualityMetricsFor(ref, decoded, 240)
		t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			m.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
	}
}

func TestExternalSampleBCGLSPCostDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_LSP_COST") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_LSP_COST=1 to run user-sample bcg729 LSP cost diagnostic")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))

	var analysis Encoder
	lsp.InitFreqPrev(&analysis.freqPrev)
	lsp.InitLSPOld(&analysis.lspOld)
	var localMem, bcgMem [4][10]int16
	lsp.InitFreqPrev(&localMem)
	lsp.InitFreqPrev(&bcgMem)

	type example struct {
		frame                        int
		local, bcg                   lsp.Indices
		localCostLocal, bcgCostLocal int64
		localCostBcg, bcgCostBcg     int64
	}
	var (
		total, same, bcgLowerLocalMem, bcgLowerBcgMem int
		localCostLocalSum, bcgCostLocalSum            int64
		localCostBcgSum, bcgCostBcgSum                int64
		firstExamples                                 []example
		examples                                      []example
	)
	for off := 0; off+FrameSamples <= len(src); off += FrameSamples {
		frame := off / FrameSamples
		var omega [10]int16
		nextEncoderOmegaForDiagnostic(t, &analysis, src[off:off+FrameSamples], &omega)

		localBefore := localMem
		localIdx := lsp.Quantize(&omega, &localMem)
		bcgIdx := lspIndicesFromFrame(bcgFrames[frame])

		localCostLocal := lsp.TupleCostForDiagnostic(&omega, &localBefore, localIdx)
		bcgCostLocal := lsp.TupleCostForDiagnostic(&omega, &localBefore, bcgIdx)
		bcgBefore := bcgMem
		localCostBcg := lsp.TupleCostForDiagnostic(&omega, &bcgBefore, localIdx)
		bcgCostBcg := lsp.TupleCostForDiagnostic(&omega, &bcgBefore, bcgIdx)
		lsp.CommitIndicesForDiagnostic(&bcgMem, bcgIdx)

		total++
		if localIdx == bcgIdx {
			same++
		}
		if len(firstExamples) < 8 {
			firstExamples = append(firstExamples, example{
				frame: frame, local: localIdx, bcg: bcgIdx,
				localCostLocal: localCostLocal, bcgCostLocal: bcgCostLocal,
				localCostBcg: localCostBcg, bcgCostBcg: bcgCostBcg,
			})
		}
		if bcgCostLocal < localCostLocal {
			bcgLowerLocalMem++
		}
		if bcgCostBcg < localCostBcg {
			bcgLowerBcgMem++
			if len(examples) < 8 {
				examples = append(examples, example{
					frame: frame, local: localIdx, bcg: bcgIdx,
					localCostLocal: localCostLocal, bcgCostLocal: bcgCostLocal,
					localCostBcg: localCostBcg, bcgCostBcg: bcgCostBcg,
				})
			}
		}
		localCostLocalSum += localCostLocal
		bcgCostLocalSum += bcgCostLocal
		localCostBcgSum += localCostBcg
		bcgCostBcgSum += bcgCostBcg
	}

	t.Logf("external sample bcg729 LSP cost diagnostic: %s", path)
	t.Logf("frames=%d sameTuple=%.2f%% bcgLowerUnderLocalMem=%.2f%% bcgLowerUnderBCGMem=%.2f%%",
		total, percent(same, total), percent(bcgLowerLocalMem, total), percent(bcgLowerBcgMem, total))
	t.Logf("mean cost under localMem: local=%.0f bcg=%.0f ratio=%.3f",
		meanInt64(localCostLocalSum, total), meanInt64(bcgCostLocalSum, total), meanRatio(bcgCostLocalSum, localCostLocalSum))
	t.Logf("mean cost under bcgMem:   local=%.0f bcg=%.0f ratio=%.3f",
		meanInt64(localCostBcgSum, total), meanInt64(bcgCostBcgSum, total), meanRatio(bcgCostBcgSum, localCostBcgSum))
	for i, ex := range firstExamples {
		t.Logf("first[%d]: frame=%d local=(%d,%d,%d,%d) bcg=(%d,%d,%d,%d) localMemCost local=%d bcg=%d bcgMemCost local=%d bcg=%d",
			i, ex.frame,
			ex.local.L0, ex.local.L1, ex.local.L2, ex.local.L3,
			ex.bcg.L0, ex.bcg.L1, ex.bcg.L2, ex.bcg.L3,
			ex.localCostLocal, ex.bcgCostLocal, ex.localCostBcg, ex.bcgCostBcg)
	}
	for i, ex := range examples {
		t.Logf("bcg-lower[%d]: frame=%d local=(%d,%d,%d,%d) bcg=(%d,%d,%d,%d) localMemCost local=%d bcg=%d bcgMemCost local=%d bcg=%d",
			i, ex.frame,
			ex.local.L0, ex.local.L1, ex.local.L2, ex.local.L3,
			ex.bcg.L0, ex.bcg.L1, ex.bcg.L2, ex.bcg.L3,
			ex.localCostLocal, ex.bcgCostLocal, ex.localCostBcg, ex.bcgCostBcg)
	}
}

func TestExternalSampleBCGForcedLSPDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FORCED_LSP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FORCED_LSP=1 to run user-sample coherent forced-LSP diagnostic")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	type mode struct {
		name   string
		frames []bitstream.Frame
	}
	modes := []mode{
		{name: "bcg all", frames: bcgFrames},
		{name: "our current", frames: encodeBitstreamFrames(t, src)},
		{name: "forced bcg LSP", frames: encodeBitstreamFramesForcedLSP(t, src, bcgFrames)},
	}

	t.Logf("external sample coherent forced-LSP diagnostic: %s", path)
	t.Logf("%-16s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, m := range modes {
		logExternalSampleFramesQuality(t, tmp, m.name, m.frames, ref, originalSamples)
	}
}

func TestExternalSampleBCGForcedStageDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FORCED_STAGE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FORCED_STAGE=1 to run user-sample bcg729 black-box forced-stage diagnostic")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	type mode struct {
		name string
		kind string
	}
	modes := []mode{
		{name: "bcg all", kind: "bcgAll"},
		{name: "our current", kind: "ourAll"},
		{name: "force bcg LSP normal", kind: "bcgLSPNormal"},
		{name: "force bcg pitch", kind: "bcgPitch"},
		{name: "force bcg LSP+pitch", kind: "bcgLSPPitch"},
		{name: "force bcg pitch+code own gain", kind: "bcgPitchCodeOwnGain"},
		{name: "force bcg LSP+pitch+code own gain", kind: "bcgLSPPitchCodeOwnGain"},
	}

	t.Logf("external sample bcg729 black-box forced-stage diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, m := range modes {
		var frames []bitstream.Frame
		switch m.kind {
		case "bcgAll":
			frames = bcgFrames
		case "ourAll":
			frames = encodeBitstreamFrames(t, src)
		default:
			frames = encodeBitstreamFramesForcedBCGStages(t, src, bcgFrames, m.kind)
		}
		logExternalSampleFramesQuality(t, tmp, m.name, frames, ref, originalSamples)
	}
}

func TestExternalSampleBCGForcedGainSelectionDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_GAIN_SELECTION") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_GAIN_SELECTION=1 to run user-sample bcg729 black-box forced-gain diagnostic")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	type mode struct {
		name     string
		forceLSP bool
		commit   string
	}
	modes := []mode{
		{name: "own LSP / own-gain state", commit: "own"},
		{name: "bcg LSP / own-gain state", forceLSP: true, commit: "own"},
		{name: "own LSP / bcg-gain state", commit: "bcg"},
		{name: "bcg LSP / bcg-gain state", forceLSP: true, commit: "bcg"},
	}

	t.Logf("external sample bcg729 black-box forced-gain selection diagnostic: %s", path)
	t.Logf("%-28s %6s %9s %9s %9s %9s %9s %10s %10s %9s %9s",
		"Mode", "N", "GA eq", "GB eq", "both eq", "own<gp", "own<gc", "own gp", "bcg gp", "own gc", "bcg gc")
	for _, m := range modes {
		stats := collectExternalBCGForcedGainSelection(t, src, bcgFrames, m.forceLSP, m.commit)
		t.Logf("%-28s %6d %8.2f%% %8.2f%% %8.2f%% %8.2f%% %8.2f%% %10.0f %10.0f %9.0f %9.0f",
			m.name,
			stats.count,
			percent(stats.sameGA, stats.count),
			percent(stats.sameGB, stats.count),
			percent(stats.sameBoth, stats.count),
			percent(stats.ownLowerGp, stats.count),
			percent(stats.ownLowerGc, stats.count),
			meanInt64(stats.ownGpSum, stats.count),
			meanInt64(stats.refGpSum, stats.count),
			meanInt64(stats.ownGcSum, stats.count),
			meanInt64(stats.refGcSum, stats.count),
		)
		for i, ex := range stats.examples {
			t.Logf("%s mismatch[%d]: frame=%d sub=%d own=(GA=%d GB=%d gp=%d gc=%d) bcg=(GA=%d GB=%d gp=%d gc=%d) gpcPred=%d",
				m.name, i, ex.frame, ex.sub,
				ex.ownGA, ex.ownGB, ex.ownGpQ14, ex.ownGcQ12,
				ex.refGA, ex.refGB, ex.refGpQ14, ex.refGcQ12,
				ex.gpcPredQ12)
		}
	}
}

func TestExternalSampleBCGGainRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_GAIN_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_GAIN_RANK=1 to run user-sample bcg729 black-box gain-rank diagnostic")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	for _, m := range []struct {
		name           string
		commit         string
		xNum, xDen     int32
		yNum, yDen     int32
		gpcNum, gpcDen int32
	}{
		{name: "production/own-gain state", commit: "own", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "production/bcg-gain state", commit: "bcg", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "previous/own-gain state", commit: "own", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 5, gpcDen: 3},
		{name: "previous/bcg-gain state", commit: "bcg", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 5, gpcDen: 3},
	} {
		stats := collectExternalBCGGainRank(t, src, bcgFrames, m.commit, m.xNum, m.xDen, m.yNum, m.yDen, m.gpcNum, m.gpcDen)
		t.Logf("external sample bcg729 black-box gain-rank diagnostic: %s / %s", path, m.name)
		t.Logf("N=%d same=%.2f%% bcgInPreselect=%.2f%% searchRank mean=%.1f <=1 %.2f%% <=4 %.2f%% <=8 %.2f%% <=32 %.2f%% bcgSearch<own %.2f%%",
			stats.count,
			percent(stats.sameBoth, stats.count),
			percent(stats.bcgInPreselect, stats.count),
			meanInt64(stats.bcgSearchRankSum, stats.count),
			percent(stats.bcgSearchRankLE1, stats.count),
			percent(stats.bcgSearchRankLE4, stats.count),
			percent(stats.bcgSearchRankLE8, stats.count),
			percent(stats.bcgSearchRankLE32, stats.count),
			percent(stats.bcgSearchLowerOwn, stats.count),
		)
		t.Logf("nativeRank mean=%.1f <=1 %.2f%% <=4 %.2f%% <=8 %.2f%% <=32 %.2f%% bcgNative<own %.2f%%",
			meanInt64(stats.bcgNativeRankSum, stats.count),
			percent(stats.bcgNativeRankLE1, stats.count),
			percent(stats.bcgNativeRankLE4, stats.count),
			percent(stats.bcgNativeRankLE8, stats.count),
			percent(stats.bcgNativeRankLE32, stats.count),
			percent(stats.bcgNativeLowerOwn, stats.count),
		)
		for i, ex := range stats.examples {
			t.Logf("%s rank[%d]: frame=%d sub=%d own=(GA=%d GB=%d) bcg=(GA=%d GB=%d) searchRank=%d nativeRank=%d bcgInPreselect=%t ownSearchCost=%d bcgSearchCost=%d ownNativeCost=%d bcgNativeCost=%d",
				m.name, i, ex.frame, ex.sub,
				ex.ownGA, ex.ownGB, ex.bcgGA, ex.bcgGB,
				ex.searchRank, ex.nativeRank, ex.bcgInPreselect,
				ex.ownSearchCost, ex.bcgSearchCost, ex.ownNativeCost, ex.bcgNativeCost,
			)
		}
	}
}

func TestExternalSampleProfileGainRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_GAIN_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_GAIN_RANK=1 to compare core/quality gain rank against bcg729 black-box")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
	}
	t.Logf("external sample profile gain-rank diagnostic: %s", path)
	for _, p := range profiles {
		for _, commit := range []string{"own", "bcg"} {
			stats := collectExternalBCGGainRankWithProfile(t, src, bcgFrames, p.profile, commit)
			t.Logf("%s / %s-gain state: N=%d same=%.2f%% bcgInPreselect=%.2f%% searchRank mean=%.1f <=1 %.2f%% <=4 %.2f%% <=8 %.2f%% <=32 %.2f%% bcgSearch<own %.2f%%",
				p.name,
				commit,
				stats.count,
				percent(stats.sameBoth, stats.count),
				percent(stats.bcgInPreselect, stats.count),
				meanInt64(stats.bcgSearchRankSum, stats.count),
				percent(stats.bcgSearchRankLE1, stats.count),
				percent(stats.bcgSearchRankLE4, stats.count),
				percent(stats.bcgSearchRankLE8, stats.count),
				percent(stats.bcgSearchRankLE32, stats.count),
				percent(stats.bcgSearchLowerOwn, stats.count),
			)
			t.Logf("%s / %s-gain state nativeRank mean=%.1f <=1 %.2f%% <=4 %.2f%% <=8 %.2f%% <=32 %.2f%% bcgNative<own %.2f%%",
				p.name,
				commit,
				meanInt64(stats.bcgNativeRankSum, stats.count),
				percent(stats.bcgNativeRankLE1, stats.count),
				percent(stats.bcgNativeRankLE4, stats.count),
				percent(stats.bcgNativeRankLE8, stats.count),
				percent(stats.bcgNativeRankLE32, stats.count),
				percent(stats.bcgNativeLowerOwn, stats.count),
			)
			for i, ex := range stats.examples {
				t.Logf("%s / %s-gain state rank[%d]: frame=%d sub=%d own=(GA=%d GB=%d) bcg=(GA=%d GB=%d) searchRank=%d nativeRank=%d bcgInPreselect=%t ownSearchCost=%d bcgSearchCost=%d ownNativeCost=%d bcgNativeCost=%d",
					p.name, commit, i, ex.frame, ex.sub,
					ex.ownGA, ex.ownGB, ex.bcgGA, ex.bcgGB,
					ex.searchRank, ex.nativeRank, ex.bcgInPreselect,
					ex.ownSearchCost, ex.bcgSearchCost, ex.ownNativeCost, ex.bcgNativeCost,
				)
			}
		}
	}
}

func TestExternalSampleBCGFCBRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FCB_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FCB_RANK=1 to run user-sample bcg729 black-box FCB-rank diagnostic")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	for _, m := range []struct {
		name     string
		forceLSP bool
		commit   string
	}{
		{name: "own LSP + bcg pitch / local state", commit: "local"},
		{name: "bcg LSP + bcg pitch / local state", forceLSP: true, commit: "local"},
		{name: "own LSP + bcg nonLSP state", commit: "bcg"},
		{name: "bcg LSP + bcg nonLSP state", forceLSP: true, commit: "bcg"},
	} {
		stats := collectExternalBCGFCBRank(t, src, bcgFrames, m.forceLSP, m.commit, 0)
		t.Logf("external sample bcg729 black-box FCB-rank diagnostic: %s / %s", path, m.name)
		t.Logf("N=%d C-eq %.2f%% S-eq %.2f%% C+S-eq %.2f%% bcgC top1 %.2f%% top4 %.2f%% top8 %.2f%% top32 %.2f%% bcgS-local %.2f%%",
			stats.count,
			percent(stats.sameC, stats.count),
			percent(stats.sameS, stats.count),
			percent(stats.sameBoth, stats.count),
			percent(stats.bcgTop1, stats.count),
			percent(stats.bcgTop4, stats.count),
			percent(stats.bcgTop8, stats.count),
			percent(stats.bcgTop32, stats.count),
			percent(stats.bcgSignMatchesLocal, stats.count),
		)
		for i, ex := range stats.examples {
			t.Logf("%s fcb[%d]: frame=%d sub=%d localC=%d localS=%d bcgC=%d bcgS=%d topK=%d bcgLocalS=%d",
				m.name, i, ex.frame, ex.sub, ex.localC, ex.localS, ex.bcgC, ex.bcgS, ex.bcgTopK, ex.bcgLocalS)
		}
		activeStats := collectExternalBCGFCBRank(t, src, bcgFrames, m.forceLSP, m.commit, 1000)
		t.Logf("%s active-rms>=1000: N=%d C-eq %.2f%% S-eq %.2f%% C+S-eq %.2f%% bcgC top1 %.2f%% top4 %.2f%% top8 %.2f%% top32 %.2f%% bcgS-local %.2f%%",
			m.name,
			activeStats.count,
			percent(activeStats.sameC, activeStats.count),
			percent(activeStats.sameS, activeStats.count),
			percent(activeStats.sameBoth, activeStats.count),
			percent(activeStats.bcgTop1, activeStats.count),
			percent(activeStats.bcgTop4, activeStats.count),
			percent(activeStats.bcgTop8, activeStats.count),
			percent(activeStats.bcgTop32, activeStats.count),
			percent(activeStats.bcgSignMatchesLocal, activeStats.count),
		)
	}
}

func TestExternalSampleProfileFCBRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_FCB_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_FCB_RANK=1 to compare core/quality FCB rank against bcg729 black-box")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
	}
	modes := []struct {
		name     string
		forceLSP bool
		commit   string
	}{
		{name: "own LSP + bcg pitch / local state", commit: "local"},
		{name: "bcg LSP + bcg nonLSP state", forceLSP: true, commit: "bcg"},
	}

	t.Logf("external sample profile FCB-rank diagnostic: %s", path)
	for _, p := range profiles {
		for _, m := range modes {
			stats := collectExternalBCGFCBRankWithProfile(t, src, bcgFrames, p.profile, m.forceLSP, m.commit, 1000)
			t.Logf("%s / %s active-rms>=1000: N=%d C-eq %.2f%% S-eq %.2f%% C+S-eq %.2f%% bcgC top1 %.2f%% top4 %.2f%% top8 %.2f%% top32 %.2f%% bcgS-local %.2f%%",
				p.name,
				m.name,
				stats.count,
				percent(stats.sameC, stats.count),
				percent(stats.sameS, stats.count),
				percent(stats.sameBoth, stats.count),
				percent(stats.bcgTop1, stats.count),
				percent(stats.bcgTop4, stats.count),
				percent(stats.bcgTop8, stats.count),
				percent(stats.bcgTop32, stats.count),
				percent(stats.bcgSignMatchesLocal, stats.count),
			)
			for i, ex := range stats.examples {
				t.Logf("%s / %s fcb[%d]: frame=%d sub=%d localC=%d localS=%d bcgC=%d bcgS=%d topK=%d bcgLocalS=%d",
					p.name, m.name, i, ex.frame, ex.sub, ex.localC, ex.localS, ex.bcgC, ex.bcgS, ex.bcgTopK, ex.bcgLocalS)
			}
		}
	}
}

func TestExternalSampleBCGFCBSurfaceVariantDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FCB_SURFACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FCB_SURFACE=1 to run user-sample bcg729 black-box FCB-surface diagnostic")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	modes := externalFCBSurfaceVariantModes()

	t.Logf("external sample bcg729 black-box FCB-surface variant diagnostic: %s", path)
	t.Logf("%-20s %6s %8s %8s %8s %8s %8s %8s", "Mode", "N", "C-eq", "S-eq", "C+S", "top1", "top8", "top32")
	for _, mode := range modes {
		stats := collectExternalBCGFCBSurfaceVariant(t, src, bcgFrames, mode, 1000)
		t.Logf("%-20s %6d %7.2f%% %7.2f%% %7.2f%% %7.2f%% %7.2f%% %7.2f%%",
			mode.name,
			stats.count,
			percent(stats.sameC, stats.count),
			percent(stats.sameS, stats.count),
			percent(stats.sameBoth, stats.count),
			percent(stats.bcgTop1, stats.count),
			percent(stats.bcgTop8, stats.count),
			percent(stats.bcgTop32, stats.count),
		)
	}
}

func externalFCBSurfaceVariantModes() []fcbSurfaceVariant {
	return []fcbSurfaceVariant{
		{name: "current"},
		{name: "target trunc", target: "trunc"},
		{name: "no pitch target", target: "none"},
		{name: "half pitch target", target: "half"},
		{name: "double pitch target", target: "double"},
		{name: "ref gp target", target: "refgp"},
		{name: "prev gp target", target: "prevgp"},
		{name: "plain h", hMode: "plain"},
		{name: "phi diag full", phiMode: "diagFull"},
		{name: "phi cross half", phiMode: "crossHalf"},
		{name: "phi cross double", phiMode: "crossDouble"},
		{name: "abs d no phi sign", phiMode: "unsigned"},
		{name: "signed d no abs", dMode: "signed"},
	}
}

func TestExternalSampleProfileFCBSurfaceVariantDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_FCB_SURFACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_FCB_SURFACE=1 to compare core/quality FCB surface variants")
	}
	path := externalSampleQualityPath()
	if path == "" {
		t.Skip("set G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav, or add testdata/external/user_quality_input.{wav,mp3,pcm,raw,sln,s16le,in}")
	}

	src := readExternalQualitySamples(t, path)
	if len(src) < FrameSamples {
		t.Fatalf("%s produced %d samples; need at least one 80-sample frame", path, len(src))
	}
	if rem := len(src) % FrameSamples; rem != 0 {
		src = append(src, make([]int16, FrameSamples-rem)...)
	}

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}
	modes := externalFCBSurfaceVariantModes()

	t.Logf("external sample profile FCB-surface variant diagnostic: %s", path)
	t.Logf("%-8s %-20s %6s %8s %8s %8s %8s %8s %8s", "Profile", "Mode", "N", "C-eq", "S-eq", "C+S", "top1", "top8", "top32")
	for _, p := range profiles {
		for _, mode := range modes {
			stats := collectExternalBCGFCBSurfaceVariantWithProfile(t, src, bcgFrames, p.profile, mode, 1000)
			t.Logf("%-8s %-20s %6d %7.2f%% %7.2f%% %7.2f%% %7.2f%% %7.2f%% %7.2f%%",
				p.name,
				mode.name,
				stats.count,
				percent(stats.sameC, stats.count),
				percent(stats.sameS, stats.count),
				percent(stats.sameBoth, stats.count),
				percent(stats.bcgTop1, stats.count),
				percent(stats.bcgTop8, stats.count),
				percent(stats.bcgTop32, stats.count),
			)
		}
	}
}

func TestExternalSampleBCGFCBAnomalyDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FCB_ANOMALY") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FCB_ANOMALY=1 to run user-sample bcg729 black-box FCB-anomaly diagnostic")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}

	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	t.Logf("external sample bcg729 black-box FCB-anomaly diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)

	limit := 8
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	selected := make(map[int]bool, limit)
	t.Logf("%-6s %9s %12s %12s %12s %8s %9s %8s %9s", "frame", "refRMS", "ourMSE", "bcgMSE", "deltaMSE", "ourPk", "ourClip", "bcgPk", "bcgClip")
	for i := 0; i < limit; i++ {
		e := frameErrors[i]
		selected[e.frame] = true
		t.Logf("%-6d %9.1f %12.1f %12.1f %12.1f %8d %9d %8d %9d",
			e.frame, e.refRMS, e.ourMSE, e.bcgMSE, e.deltaMSE, e.ourPeak, e.ourNearClip, e.bcgPeak, e.bcgNearClip)
	}

	for _, m := range []struct {
		name     string
		forceLSP bool
		commit   string
	}{
		{name: "ownLSP+bcgPitch/localCommit", commit: "local"},
		{name: "bcgLSP+bcgPitch/bcgCommit", forceLSP: true, commit: "bcg"},
	} {
		traces := collectExternalBCGFCBAnomalyTrace(t, src, bcgFrames, selected, m.forceLSP, m.commit)
		t.Logf("%s: selected-frame FCB search surface", m.name)
		t.Logf("%-6s %-3s %4s %4s %7s %8s %8s %8s %8s %11s %7s %7s %8s %8s %8s %8s %9s %9s %9s",
			"frame", "sub", "T", "fr", "gp", "xRMS", "yRMS", "hRMS", "xpRMS", "dMax", "localC", "bcgC", "topK", "rank", "S", "bcgS", "bcg/local", "bcg/best", "dBcg/dLoc")
		for _, tr := range traces {
			t.Logf("%-6d %-3d %4d %4d %7d %8.1f %8.1f %8.1f %8.1f %11d %7d %7d %8d %8d %08b %08b %9.3f %9.3f %9.3f",
				tr.frame, tr.sub, tr.intLag, tr.frac, tr.gpQ14,
				tr.xRMS, tr.yRMS, tr.hRMS, tr.xPrimeRMS, tr.dMaxAbs,
				tr.localC, tr.bcgC, tr.bcgTopK, tr.bcgRank, tr.localS, tr.bcgS,
				tr.bcgToLocalScoreRatio, tr.bcgToBestScoreRatio, tr.bcgToLocalDSumRatio)
		}
	}
}

func TestExternalSampleFCBGainAwareRerankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_FCB_GAIN_RERANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_FCB_GAIN_RERANK=1 to run user-sample FCB gain-aware rerank diagnostic")
	}
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
	ref := src[:originalSamples]

	modes := []struct {
		name                   string
		topK                   int
		gpcNum, gpcDen         int32
		searchXNum, searchXDen int32
		searchYNum, searchYDen int32
	}{
		{name: "current", topK: 0},
		{name: "top1 current", topK: 1, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top4 current", topK: 4, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top8 current", topK: 8, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top16 current", topK: 16, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top32 current", topK: 32, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top64 current", topK: 64, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top128 current", topK: 128, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top256 current", topK: 256, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top8 thin", topK: 8, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
		{name: "top16 thin", topK: 16, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
		{name: "top32 thin", topK: 32, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
		{name: "top64 thin", topK: 64, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
		{name: "top128 thin", topK: 128, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
	}

	type result struct {
		name string
		m    externalQualityMetrics
	}
	tmp := t.TempDir()
	results := make([]result, 0, len(modes))
	for _, mode := range modes {
		var frames []bitstream.Frame
		if mode.topK == 0 {
			frames = encodeBitstreamFrames(t, src)
		} else {
			normalizeGainSweepMode(&mode.gpcNum, &mode.gpcDen)
			normalizeGainSweepMode(&mode.searchXNum, &mode.searchXDen)
			normalizeGainSweepMode(&mode.searchYNum, &mode.searchYDen)
			frames = encodeBitstreamFramesFCBGainAwareRerank(t, src, mode.topK,
				mode.gpcNum, mode.gpcDen,
				mode.searchXNum, mode.searchXDen,
				mode.searchYNum, mode.searchYDen)
		}
		results = append(results, result{
			name: mode.name,
			m:    measureExternalSampleFramesQuality(t, tmp, mode.name, frames, ref, originalSamples),
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].m.globalSNR == results[j].m.globalSNR {
			return results[i].m.segSNR > results[j].m.segSNR
		}
		return results[i].m.globalSNR > results[j].m.globalSNR
	})

	t.Logf("external sample FCB gain-aware rerank diagnostic: %s", path)
	t.Logf("%-16s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, r := range results {
		t.Logf("%-16s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			r.name, r.m.shift, r.m.rms, r.m.globalSNR, r.m.segSNR, r.m.corr, r.m.rmsRatio, r.m.peak, r.m.nearClip)
	}
}

func TestExternalSampleBCGStateDivergenceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_STATE_DIVERGENCE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_STATE_DIVERGENCE=1 to run user-sample bcg729 black-box state-divergence diagnostic")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 8
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	selected := make(map[int]bool, limit)
	for i := 0; i < limit; i++ {
		selected[frameErrors[i].frame] = true
	}

	rows := collectExternalBCGStateDivergence(t, src, bcgFrames, selected)
	t.Logf("external sample bcg729 black-box state-divergence diagnostic: %s", path)
	t.Logf("%-6s %-3s %8s %8s %8s %8s %8s %8s %8s %8s %8s %8s %8s",
		"frame", "sub", "swRMS-L", "swRMS-B", "excCorr", "xCorr", "xpCorr", "hCorr", "yCorr", "dCorr", "bcgTopL", "bcgTopB", "b/lScore")
	for _, row := range rows {
		t.Logf("%-6d %-3d %8.1f %8.1f %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f %8d %8d %8.3f",
			row.frame, row.sub,
			row.localSwMemRMS, row.bcgSwMemRMS,
			row.oldExcCorr, row.xCorr, row.xPrimeCorr, row.hCorr, row.yCorr, row.dCorr,
			row.bcgTopKLocal, row.bcgTopKBCGState, row.bcgStateToLocalScoreRatio)
	}
}

func TestExternalSampleProfileStateDivergenceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_STATE_DIVERGENCE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_STATE_DIVERGENCE=1 to compare core/quality state divergence against bcg729 black-box")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
	}

	t.Logf("external sample profile state-divergence diagnostic: %s", path)
	t.Logf("%-8s %-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Profile", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		raw := readFile(t, rawPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		localDecoded := decodeRawG729WithLocal(t, raw)
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(localDecoded) > originalSamples {
			localDecoded = localDecoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s ffmpeg output too short: got %d want >= %d", p.name, len(decoded), originalSamples)
		}
		if len(localDecoded) < originalSamples {
			t.Fatalf("%s local output too short: got %d want >= %d", p.name, len(localDecoded), originalSamples)
		}
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		localMetrics := externalQualityMetricsFor(ref, localDecoded, 240)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> our encoder -> ffmpeg",
			metrics.shift, metrics.rms, metrics.globalSNR, metrics.segSNR,
			metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> our encoder -> local",
			localMetrics.shift, localMetrics.rms, localMetrics.globalSNR, localMetrics.segSNR,
			localMetrics.corr, localMetrics.rmsRatio, localMetrics.peak, localMetrics.nearClip)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> bcg encoder -> ffmpeg",
			bcgMetrics.shift, bcgMetrics.rms, bcgMetrics.globalSNR, bcgMetrics.segSNR,
			bcgMetrics.corr, bcgMetrics.rmsRatio, bcgMetrics.peak, bcgMetrics.nearClip)

		aligned := alignByShift(ref, decoded, metrics.shift)
		frameErrors := rankExternalFrameErrors(ref, aligned, bcgAligned, 1000)
		limit := 8
		if len(frameErrors) < limit {
			limit = len(frameErrors)
		}
		selected := make(map[int]bool, limit)
		for i := 0; i < limit; i++ {
			selected[frameErrors[i].frame] = true
		}
		nearClipFrames := externalNearClipFrames(decoded, metrics.shift, len(src)/FrameSamples, 32700)
		for _, frame := range nearClipFrames {
			selected[frame] = true
		}
		localNearClipFrames := externalNearClipFrames(localDecoded, localMetrics.shift, len(src)/FrameSamples, 32700)
		for _, frame := range localNearClipFrames {
			selected[frame] = true
		}

		t.Logf("%s selected worst frames:", p.name)
		t.Logf("%-8s %-6s %9s %9s %9s %7s %8s %8s", "Profile", "frame", "refRMS", "ourMSE", "bcgMSE", "delta", "ourPeak", "bcgPeak")
		for i := 0; i < limit; i++ {
			e := frameErrors[i]
			t.Logf("%-8s %-6d %9.1f %9.0f %9.0f %7.0f %8d %8d",
				p.name, e.frame, e.refRMS, e.ourMSE, e.bcgMSE, e.deltaMSE, e.ourPeak, e.bcgPeak)
		}
		if len(nearClipFrames) > 0 {
			t.Logf("%s selected near-clip frames: %v", p.name, nearClipFrames)
		}
		if len(localNearClipFrames) > 0 {
			t.Logf("%s selected local near-clip frames: %v", p.name, localNearClipFrames)
		}

		rows := collectExternalBCGStateDivergenceWithProfile(t, src, bcgFrames, selected, p.profile)
		t.Logf("%-8s %-6s %-3s %8s %8s %8s %8s %8s %8s %8s %8s %8s %8s %8s",
			"Profile", "frame", "sub", "swRMS-L", "swRMS-B", "excCorr", "xCorr", "xpCorr", "hCorr", "yCorr", "dCorr", "bcgTopL", "bcgTopB", "b/lScore")
		for _, row := range rows {
			t.Logf("%-8s %-6d %-3d %8.1f %8.1f %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f %8d %8d %8.3f",
				p.name, row.frame, row.sub,
				row.localSwMemRMS, row.bcgSwMemRMS,
				row.oldExcCorr, row.xCorr, row.xPrimeCorr, row.hCorr, row.yCorr, row.dCorr,
				row.bcgTopKLocal, row.bcgTopKBCGState, row.bcgStateToLocalScoreRatio)
		}
	}
}

func TestExternalSampleBCGExcitationCommitTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_EXCITATION_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_EXCITATION_TRACE=1 to run user-sample bcg729 black-box excitation commit trace")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 4
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	targets := make([]int, limit)
	for i := 0; i < limit; i++ {
		targets[i] = frameErrors[i].frame
	}

	const lookbackFrames = 4
	rows := collectExternalBCGExcitationCommitTrace(t, src, bcgFrames, targets, lookbackFrames)
	t.Logf("external sample bcg729 black-box excitation commit trace: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	t.Logf("%-6s %-6s %-4s %-3s %7s %7s %7s %7s %8s %8s %8s %8s %8s %8s %6s %6s %7s %7s %7s %7s",
		"target", "frame", "off", "sub", "locT", "bcgT", "locG", "bcgG", "oldCorr", "vCorr", "cCorr", "pCorr", "kCorr", "uCorr", "lGp", "bGp", "lGc", "bGc", "lURMS", "bURMS")
	for _, row := range rows {
		t.Logf("%-6d %-6d %-4d %-3d %7d %7d %7s %7s %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f %6d %6d %7d %7d %7.1f %7.1f",
			row.targetFrame, row.frame, row.frame-row.targetFrame, row.sub,
			row.localT, row.bcgT,
			fmt.Sprintf("%d/%d", row.localGA, row.localGB),
			fmt.Sprintf("%d/%d", row.bcgGA, row.bcgGB),
			row.oldExcCorr, row.vCorr, row.cCorr, row.pitchTermCorr, row.codeTermCorr, row.uCorr,
			row.localGpQ14, row.bcgGpQ14, row.localGcQ12, row.bcgGcQ12,
			row.localURMS, row.bcgURMS)
	}
}

func TestExternalSampleProfileExcitationCommitTraceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_EXCITATION_TRACE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_EXCITATION_TRACE=1 to compare core/quality excitation commit traces against bcg729 black-box")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}

	const lookbackFrames = 4
	t.Logf("external sample profile excitation commit trace: %s", path)
	t.Logf("%-8s %-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Profile", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s ffmpeg output too short: got %d want >= %d", p.name, len(decoded), originalSamples)
		}
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> our encoder -> ffmpeg",
			metrics.shift, metrics.rms, metrics.globalSNR, metrics.segSNR,
			metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> bcg encoder -> ffmpeg",
			bcgMetrics.shift, bcgMetrics.rms, bcgMetrics.globalSNR, bcgMetrics.segSNR,
			bcgMetrics.corr, bcgMetrics.rmsRatio, bcgMetrics.peak, bcgMetrics.nearClip)

		aligned := alignByShift(ref, decoded, metrics.shift)
		frameErrors := rankExternalFrameErrors(ref, aligned, bcgAligned, 1000)
		targetSet := make(map[int]bool)
		limit := 4
		if len(frameErrors) < limit {
			limit = len(frameErrors)
		}
		for i := 0; i < limit; i++ {
			targetSet[frameErrors[i].frame] = true
		}
		for _, frame := range externalNearClipFrames(decoded, metrics.shift, len(src)/FrameSamples, 32700) {
			targetSet[frame] = true
		}
		targets := make([]int, 0, len(targetSet))
		for frame := range targetSet {
			targets = append(targets, frame)
		}
		sort.Ints(targets)
		t.Logf("%s excitation trace targets=%v lookback=%d", p.name, targets, lookbackFrames)

		rows := collectExternalBCGExcitationCommitTraceWithProfile(t, src, bcgFrames, targets, lookbackFrames, p.profile)
		t.Logf("%-8s %-6s %-6s %-4s %-3s %7s %7s %7s %7s %8s %8s %8s %8s %8s %8s %6s %6s %7s %7s %7s %7s",
			"Profile", "target", "frame", "off", "sub", "locT", "bcgT", "locG", "bcgG", "oldCorr", "vCorr", "cCorr", "pCorr", "kCorr", "uCorr", "lGp", "bGp", "lGc", "bGc", "lURMS", "bURMS")
		for _, row := range rows {
			t.Logf("%-8s %-6d %-6d %-4d %-3d %7d %7d %7s %7s %8.4f %8.4f %8.4f %8.4f %8.4f %8.4f %6d %6d %7d %7d %7.1f %7.1f",
				p.name, row.targetFrame, row.frame, row.frame-row.targetFrame, row.sub,
				row.localT, row.bcgT,
				fmt.Sprintf("%d/%d", row.localGA, row.localGB),
				fmt.Sprintf("%d/%d", row.bcgGA, row.bcgGB),
				row.oldExcCorr, row.vCorr, row.cCorr, row.pitchTermCorr, row.codeTermCorr, row.uCorr,
				row.localGpQ14, row.bcgGpQ14, row.localGcQ12, row.bcgGcQ12,
				row.localURMS, row.bcgURMS)
		}
	}
}

func TestExternalSampleBCGPitchTrajectoryTimelineDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_PITCH_TIMELINE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_PITCH_TIMELINE=1 to run user-sample bcg729 black-box pitch trajectory timeline")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 4
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	targets := make([]int, limit)
	for i := 0; i < limit; i++ {
		targets[i] = frameErrors[i].frame
	}
	for _, extra := range strings.FieldsFunc(os.Getenv("G729_EXTERNAL_SAMPLE_BCG_PITCH_TARGETS"), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		frame, err := strconv.Atoi(extra)
		if err != nil {
			t.Fatalf("invalid G729_EXTERNAL_SAMPLE_BCG_PITCH_TARGETS frame %q: %v", extra, err)
		}
		targets = append(targets, frame)
	}
	sort.Ints(targets)

	const lookbackFrames = 6
	rows := collectExternalBCGPitchTimeline(t, src, bcgFrames, targets, lookbackFrames)
	t.Logf("external sample bcg729 black-box pitch trajectory timeline: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	t.Logf("%-6s %-6s %-4s %5s %5s %6s %6s %7s %7s %7s %7s %7s %7s %6s %6s %6s %6s %6s %6s %8s",
		"target", "frame", "off", "top", "bTop", "r1", "r2", "r3", "r1Rel", "r2Rel", "r3Rel", "locT1", "bcgT1", "locT2", "bcgT2", "locG1", "bcgG1", "locG2", "bcgG2", "oldCorr")
	for _, row := range rows {
		t.Logf("%-6d %-6d %-4d %5d %5d %6d %6d %7d %7.3f %7.3f %7.3f %7s %7s %6s %6s %6s %6s %6s %6s %8.4f",
			row.targetFrame, row.frame, row.frame-row.targetFrame,
			row.localTop, row.bcgStateTop,
			row.range1Lag, row.range2Lag, row.range3Lag,
			row.range1Rel, row.range2Rel, row.range3Rel,
			fmt.Sprintf("%d/%d", row.localT1, row.localFrac1),
			fmt.Sprintf("%d/%d", row.bcgT1, row.bcgFrac1),
			fmt.Sprintf("%d/%d", row.localT2, row.localFrac2),
			fmt.Sprintf("%d/%d", row.bcgT2, row.bcgFrac2),
			fmt.Sprintf("%d/%d", row.localGA1, row.localGB1),
			fmt.Sprintf("%d/%d", row.bcgGA1, row.bcgGB1),
			fmt.Sprintf("%d/%d", row.localGA2, row.localGB2),
			fmt.Sprintf("%d/%d", row.bcgGA2, row.bcgGB2),
			row.oldExcCorr)
	}
}

func TestExternalSampleProfilePitchTrajectoryTimelineDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_PITCH_TIMELINE") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_PITCH_TIMELINE=1 to compare core/quality pitch timelines against bcg729 black-box")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)
	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	if len(bcgFrames) != len(src)/FrameSamples {
		t.Fatalf("bcg frame count=%d want %d", len(bcgFrames), len(src)/FrameSamples)
	}
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
	}

	const lookbackFrames = 6
	t.Logf("external sample profile pitch trajectory timeline: %s", path)
	t.Logf("%-8s %-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Profile", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)

		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s ffmpeg output too short: got %d want >= %d", p.name, len(decoded), originalSamples)
		}
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> our encoder -> ffmpeg",
			metrics.shift, metrics.rms, metrics.globalSNR, metrics.segSNR,
			metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)
		t.Logf("%-8s %-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			p.name, "input -> bcg encoder -> ffmpeg",
			bcgMetrics.shift, bcgMetrics.rms, bcgMetrics.globalSNR, bcgMetrics.segSNR,
			bcgMetrics.corr, bcgMetrics.rmsRatio, bcgMetrics.peak, bcgMetrics.nearClip)

		aligned := alignByShift(ref, decoded, metrics.shift)
		frameErrors := rankExternalFrameErrors(ref, aligned, bcgAligned, 1000)
		targetSet := make(map[int]bool)
		limit := 4
		if len(frameErrors) < limit {
			limit = len(frameErrors)
		}
		for i := 0; i < limit; i++ {
			targetSet[frameErrors[i].frame] = true
		}
		for _, frame := range externalNearClipFrames(decoded, metrics.shift, len(src)/FrameSamples, 32700) {
			targetSet[frame] = true
		}
		targets := make([]int, 0, len(targetSet))
		for frame := range targetSet {
			targets = append(targets, frame)
		}
		sort.Ints(targets)
		t.Logf("%s pitch timeline targets=%v lookback=%d", p.name, targets, lookbackFrames)

		rows := collectExternalBCGPitchTimelineWithProfile(t, src, bcgFrames, targets, lookbackFrames, p.profile)
		t.Logf("%-8s %-6s %-6s %-4s %5s %5s %6s %6s %7s %7s %7s %7s %7s %6s %6s %6s %6s %6s %6s %6s %8s",
			"Profile", "target", "frame", "off", "top", "bTop", "r1", "r2", "r3", "r1Rel", "r2Rel", "r3Rel", "locT1", "bcgT1", "locT2", "bcgT2", "locG1", "bcgG1", "locG2", "bcgG2", "oldCorr")
		for _, row := range rows {
			t.Logf("%-8s %-6d %-6d %-4d %5d %5d %6d %6d %7d %7.3f %7.3f %7.3f %7s %6s %6s %6s %6s %6s %6s %6s %8.4f",
				p.name, row.targetFrame, row.frame, row.frame-row.targetFrame,
				row.localTop, row.bcgStateTop,
				row.range1Lag, row.range2Lag, row.range3Lag,
				row.range1Rel, row.range2Rel, row.range3Rel,
				fmt.Sprintf("%d/%d", row.localT1, row.localFrac1),
				fmt.Sprintf("%d/%d", row.bcgT1, row.bcgFrac1),
				fmt.Sprintf("%d/%d", row.localT2, row.localFrac2),
				fmt.Sprintf("%d/%d", row.bcgT2, row.bcgFrac2),
				fmt.Sprintf("%d/%d", row.localGA1, row.localGB1),
				fmt.Sprintf("%d/%d", row.bcgGA1, row.bcgGB1),
				fmt.Sprintf("%d/%d", row.localGA2, row.localGB2),
				fmt.Sprintf("%d/%d", row.bcgGA2, row.bcgGB2),
				row.oldExcCorr)
		}
	}
}

func TestExternalSampleBCGTrajectoryAuditDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_TRAJECTORY") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_TRAJECTORY=1 to run user-sample bcg729 black-box trajectory audit")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 8
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	selected := make(map[int]bool, limit)
	for i := 0; i < limit; i++ {
		selected[frameErrors[i].frame] = true
	}

	lspRows := collectExternalBCGLSPTrajectory(t, src, bcgFrames, selected)
	_, pitchRows := collectExternalBCGPitchRank(t, src, bcgFrames, selected, 1000)
	stateRows := collectExternalBCGStateDivergence(t, src, bcgFrames, selected)
	lspByFrame := make(map[int]externalBCGLSPTrajectoryRow, len(lspRows))
	for _, row := range lspRows {
		lspByFrame[row.frame] = row
	}
	pitchBySub := make(map[int]externalPitchRankRow, len(pitchRows))
	for _, row := range pitchRows {
		pitchBySub[row.frame*2+row.sub] = row
	}
	stateBySub := make(map[int]externalBCGStateDivergenceRow, len(stateRows))
	for _, row := range stateRows {
		stateBySub[row.frame*2+row.sub] = row
	}

	t.Logf("external sample bcg729 black-box trajectory audit: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	t.Logf("%-6s %9s %9s %9s %7s %8s %8s", "frame", "refRMS", "ourMSE", "bcgMSE", "delta", "ourPeak", "bcgPeak")
	for i := 0; i < limit; i++ {
		e := frameErrors[i]
		t.Logf("%-6d %9.1f %9.0f %9.0f %7.0f %8d %8d",
			e.frame, e.refRMS, e.ourMSE, e.bcgMSE, e.deltaMSE, e.ourPeak, e.bcgPeak)
	}
	t.Logf("%-6s %-17s %-17s %9s %9s %9s %9s %7s %7s",
		"frame", "localLSP", "bcgLSP", "lCostL", "bCostL", "lCostB", "bCostB", "a1Corr", "a2Corr")
	for i := 0; i < limit; i++ {
		row := lspByFrame[frameErrors[i].frame]
		t.Logf("%-6d (%d,%d,%d,%d)     (%d,%d,%d,%d)     %9d %9d %9d %9d %7.4f %7.4f",
			row.frame,
			row.local.L0, row.local.L1, row.local.L2, row.local.L3,
			row.bcg.L0, row.bcg.L1, row.bcg.L2, row.bcg.L3,
			row.localCostLocal, row.bcgCostLocal, row.localCostBcg, row.bcgCostBcg,
			row.aHatSF1Corr, row.aHatSF2Corr)
	}
	t.Logf("%-6s %-3s %6s %7s %7s %7s %7s %7s %7s %5s %7s %8s %8s %8s %8s %8s %8s %8s %7s %7s",
		"frame", "sub", "center", "localT", "localF", "bcgT", "bcgF", "bestT", "bestF", "rank", "allRank", "scoreR", "allScore", "xCorr", "xpCorr", "hCorr", "yCorr", "dCorr", "topL", "topB")
	for i := 0; i < limit; i++ {
		frame := frameErrors[i].frame
		for sub := 0; sub < 2; sub++ {
			key := frame*2 + sub
			p := pitchBySub[key]
			s := stateBySub[key]
			t.Logf("%-6d %-3d %6d %7d %7d %7d %7d %7d %7d %5d %7d %8.3f %8.3f %8.4f %8.4f %8.4f %8.4f %8.4f %7d %7d",
				frame, sub, p.centre, p.localT, p.localFrac, p.refT, p.refFrac, p.fullBestT, p.fullBestF, p.fullRank, p.allRank, p.refToLocalScoreRatio, p.refToFullScoreRatio,
				s.xCorr, s.xPrimeCorr, s.hCorr, s.yCorr, s.dCorr, s.bcgTopKLocal, s.bcgTopKBCGState)
		}
	}
}

func TestExternalSampleBCGFCBMixedSurfaceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_FCB_MIX") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_FCB_MIX=1 to run user-sample bcg729 black-box FCB mixed-surface diagnostic")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 8
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	selected := make(map[int]bool, limit)
	for i := 0; i < limit; i++ {
		selected[frameErrors[i].frame] = true
	}

	rows := collectExternalBCGFCBMixedSurface(t, src, bcgFrames, selected)
	statsByMode := make(map[string]externalFCBMixedSurfaceStats)
	for _, row := range rows {
		s := statsByMode[row.mode]
		s.count++
		s.rankSum += row.rank
		s.scoreRatioSum += row.scoreRatio
		if row.rank <= 32 {
			s.top32++
		}
		if row.signMatch {
			s.signMatch++
		}
		statsByMode[row.mode] = s
	}

	t.Logf("external sample bcg729 black-box FCB mixed-surface diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	t.Logf("%-10s %6s %10s %8s %10s %8s", "mode", "N", "meanRank", "<=32", "meanScore", "signEq")
	for _, mode := range externalFCBMixedSurfaceModes() {
		s := statsByMode[mode.name]
		t.Logf("%-10s %6d %10.1f %7.2f%% %10.3f %7.2f%%",
			mode.name, s.count, s.meanRank(), percent(s.top32, s.count), s.meanScoreRatio(), percent(s.signMatch, s.count))
	}
	t.Logf("%-6s %-3s %-10s %8s %9s %7s %5s %8s %8s %11s %8s %8s %8s",
		"frame", "sub", "mode", "rank", "scoreR", "sign", "sN", "sD/absD", "xpRMS", "dMax", "xRMS", "yRMS", "hRMS")
	for _, row := range rows {
		t.Logf("%-6d %-3d %-10s %8d %9.3f %7t %5d %8.3f %8.1f %11d %8.1f %8.1f %8.1f",
			row.frame, row.sub, row.mode, row.rank, row.scoreRatio, row.signMatch,
			row.pulseSignMatches, row.signedDSumRatio,
			row.xPrimeRMS, row.dMaxAbs, row.xRMS, row.yRMS, row.hRMS)
	}
}

func TestExternalSampleProfileFCBMixedSurfaceDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_FCB_MIX") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_FCB_MIX=1 to compare core/quality FCB mixed surfaces")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)

	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}
	type profileMixedResult struct {
		name    string
		profile EncoderProfile
		metrics externalQualityMetrics
	}
	selected := make(map[int]bool)
	results := make([]profileMixedResult, 0, len(profiles))
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s ffmpeg output too short: got %d want >= %d", p.name, len(decoded), originalSamples)
		}
		metrics := externalQualityMetricsFor(ref, decoded, 240)
		aligned := alignByShift(ref, decoded, metrics.shift)
		frameErrors := rankExternalFrameErrors(ref, aligned, bcgAligned, 1000)
		limit := 4
		if len(frameErrors) < limit {
			limit = len(frameErrors)
		}
		for i := 0; i < limit; i++ {
			selected[frameErrors[i].frame] = true
		}
		results = append(results, profileMixedResult{name: p.name, profile: p.profile, metrics: metrics})
	}

	t.Logf("external sample profile FCB mixed-surface diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	for _, result := range results {
		logExternalQualityMetrics(t, "input -> "+result.name+" encoder -> ffmpeg", result.metrics)
	}
	t.Logf("%-8s %-10s %6s %10s %8s %10s %8s", "profile", "mode", "N", "meanRank", "<=32", "meanScore", "signEq")
	for _, result := range results {
		rows := collectExternalBCGFCBMixedSurfaceWithProfile(t, src, bcgFrames, selected, result.profile)
		statsByMode := make(map[string]externalFCBMixedSurfaceStats)
		for _, row := range rows {
			s := statsByMode[row.mode]
			s.count++
			s.rankSum += row.rank
			s.scoreRatioSum += row.scoreRatio
			if row.rank <= 32 {
				s.top32++
			}
			if row.signMatch {
				s.signMatch++
			}
			statsByMode[row.mode] = s
		}
		for _, mode := range externalFCBMixedSurfaceModes() {
			s := statsByMode[mode.name]
			t.Logf("%-8s %-10s %6d %10.1f %7.2f%% %10.3f %7.2f%%",
				result.name, mode.name, s.count, s.meanRank(), percent(s.top32, s.count), s.meanScoreRatio(), percent(s.signMatch, s.count))
		}
	}
}

func TestExternalSampleBCGPitchRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_BCG_PITCH_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_BCG_PITCH_RANK=1 to run user-sample bcg729 black-box pitch-rank diagnostic")
	}
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
	ourRawPath := filepath.Join(tmp, "our.g729")
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	ourPCMPath := filepath.Join(tmp, "our.ffmpeg.s16le")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, ourRawPath)
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, ourRawPath, ourPCMPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	ref := src[:originalSamples]
	ourDecoded := s16leToSamples(readFile(t, ourPCMPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(ourDecoded) > originalSamples {
		ourDecoded = ourDecoded[:originalSamples]
	}
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(ourDecoded) < originalSamples {
		t.Fatalf("our ffmpeg output too short: got %d want >= %d", len(ourDecoded), originalSamples)
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}

	ourMetrics := externalQualityMetricsFor(ref, ourDecoded, 240)
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	ourAligned := alignByShift(ref, ourDecoded, ourMetrics.shift)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)
	frameErrors := rankExternalFrameErrors(ref, ourAligned, bcgAligned, 1000)

	limit := 8
	if len(frameErrors) < limit {
		limit = len(frameErrors)
	}
	selected := make(map[int]bool, limit)
	for i := 0; i < limit; i++ {
		selected[frameErrors[i].frame] = true
	}

	stats, rows := collectExternalBCGPitchRank(t, src, bcgFrames, selected, 1000)
	t.Logf("external sample bcg729 black-box pitch-rank diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> our encoder -> ffmpeg", ourMetrics)
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	t.Logf("active frame pitch rank summary (frame RMS >= 1000): N=%d in-window=%d %.2f%% same-int=%d %.2f%% same-frac=%d %.2f%% same-both=%d %.2f%%",
		stats.count,
		stats.inWindow, percent(stats.inWindow, stats.count),
		stats.sameInt, percent(stats.sameInt, stats.count),
		stats.sameFrac, percent(stats.sameFrac, stats.count),
		stats.sameBoth, percent(stats.sameBoth, stats.count))
	t.Logf("active pitch rank: int top1=%d %.2f%% top3=%d %.2f%% top8=%d %.2f%% mean-rank=%.2f; full top1=%d %.2f%% top3=%d %.2f%% top8=%d %.2f%% mean-rank=%.2f mean-score-ratio=%.3f",
		stats.intTop1, percent(stats.intTop1, stats.count),
		stats.intTop3, percent(stats.intTop3, stats.count),
		stats.intTop8, percent(stats.intTop8, stats.count),
		stats.meanIntRank(),
		stats.fullTop1, percent(stats.fullTop1, stats.count),
		stats.fullTop3, percent(stats.fullTop3, stats.count),
		stats.fullTop8, percent(stats.fullTop8, stats.count),
		stats.meanFullRank(),
		stats.meanScoreRatio())
	t.Logf("active pitch full-range: mean-rank=%.2f mean-score-ratio=%.3f",
		stats.meanAllRank(), stats.meanAllScoreRatio())
	t.Logf("%-6s %-3s %6s %7s %7s %7s %7s %7s %7s %7s %7s %8s %8s %8s %8s %8s %8s",
		"frame", "sub", "center", "localT", "localF", "bcgT", "bcgF", "bestT", "bestF", "intRank", "winRank", "allRank", "winScore", "allScore", "xRMS", "swRMS", "excRMS")
	for _, row := range rows {
		t.Logf("%-6d %-3d %6d %7d %7d %7d %7d %7d %7d %7d %7d %8d %8.3f %8.3f %8.1f %8.1f %8.1f",
			row.frame, row.sub, row.centre,
			row.localT, row.localFrac, row.refT, row.refFrac,
			row.fullBestT, row.fullBestF,
			row.intRank, row.fullRank, row.allRank,
			row.refToLocalScoreRatio, row.refToFullScoreRatio,
			row.xRMS, row.swMemRMS, row.oldExcRMS)
	}
}

func TestExternalSampleProfilePitchRankDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PROFILE_PITCH_RANK") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PROFILE_PITCH_RANK=1 to compare core/quality pitch rank against bcg729 black-box")
	}
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
	ref := src[:originalSamples]

	tmp := t.TempDir()
	bcgRawPath := filepath.Join(tmp, "bcg.g729")
	bcgPCMPath := filepath.Join(tmp, "bcg.ffmpeg.s16le")
	writeBCGEncodedRawG729(t, src, bcgRawPath)
	ffmpegDecodeRawG729(t, bcgRawPath, bcgPCMPath)

	bcgFrames := readRawG729Frames(t, readFile(t, bcgRawPath))
	bcgDecoded := s16leToSamples(readFile(t, bcgPCMPath))
	if len(bcgDecoded) > originalSamples {
		bcgDecoded = bcgDecoded[:originalSamples]
	}
	if len(bcgDecoded) < originalSamples {
		t.Fatalf("bcg ffmpeg output too short: got %d want >= %d", len(bcgDecoded), originalSamples)
	}
	bcgMetrics := externalQualityMetricsFor(ref, bcgDecoded, 240)
	bcgAligned := alignByShift(ref, bcgDecoded, bcgMetrics.shift)

	type profilePitchResult struct {
		name    string
		profile EncoderProfile
		metrics externalQualityMetrics
		stats   externalPitchRankStats
		rows    []externalPitchRankRow
	}
	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
		{name: "clean", profile: EncoderProfileQualityClean},
	}

	selected := make(map[int]bool)
	results := make([]profilePitchResult, 0, len(profiles))
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, p.name+".g729")
		pcmPath := filepath.Join(tmp, p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > originalSamples {
			decoded = decoded[:originalSamples]
		}
		if len(decoded) < originalSamples {
			t.Fatalf("%s ffmpeg output too short: got %d want >= %d", p.name, len(decoded), originalSamples)
		}

		metrics := externalQualityMetricsFor(ref, decoded, 240)
		aligned := alignByShift(ref, decoded, metrics.shift)
		frameErrors := rankExternalFrameErrors(ref, aligned, bcgAligned, 1000)
		limit := 8
		if len(frameErrors) < limit {
			limit = len(frameErrors)
		}
		for i := 0; i < limit; i++ {
			selected[frameErrors[i].frame] = true
		}
		for _, frame := range externalNearClipFrames(decoded, metrics.shift, len(src)/FrameSamples, 32700) {
			selected[frame] = true
		}
		results = append(results, profilePitchResult{
			name:    p.name,
			profile: p.profile,
			metrics: metrics,
		})
	}

	for i := range results {
		stats, rows := collectExternalBCGPitchRankWithProfile(t, src, bcgFrames, selected, 1000, results[i].profile)
		results[i].stats = stats
		results[i].rows = rows
	}

	t.Logf("external sample profile pitch-rank diagnostic: %s", path)
	t.Logf("%-34s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	logExternalQualityMetrics(t, "input -> bcg encoder -> ffmpeg", bcgMetrics)
	for _, result := range results {
		logExternalQualityMetrics(t, "input -> "+result.name+" encoder -> ffmpeg", result.metrics)
	}
	for _, result := range results {
		stats := result.stats
		t.Logf("%s active frame pitch rank summary (frame RMS >= 1000): N=%d in-window=%d %.2f%% same-int=%d %.2f%% same-frac=%d %.2f%% same-both=%d %.2f%%",
			result.name,
			stats.count,
			stats.inWindow, percent(stats.inWindow, stats.count),
			stats.sameInt, percent(stats.sameInt, stats.count),
			stats.sameFrac, percent(stats.sameFrac, stats.count),
			stats.sameBoth, percent(stats.sameBoth, stats.count))
		t.Logf("%s active pitch rank: int top1=%d %.2f%% top3=%d %.2f%% top8=%d %.2f%% mean-rank=%.2f; full top1=%d %.2f%% top3=%d %.2f%% top8=%d %.2f%% mean-rank=%.2f mean-score-ratio=%.3f",
			result.name,
			stats.intTop1, percent(stats.intTop1, stats.count),
			stats.intTop3, percent(stats.intTop3, stats.count),
			stats.intTop8, percent(stats.intTop8, stats.count),
			stats.meanIntRank(),
			stats.fullTop1, percent(stats.fullTop1, stats.count),
			stats.fullTop3, percent(stats.fullTop3, stats.count),
			stats.fullTop8, percent(stats.fullTop8, stats.count),
			stats.meanFullRank(),
			stats.meanScoreRatio())
		t.Logf("%s active pitch full-range: mean-rank=%.2f mean-score-ratio=%.3f",
			result.name, stats.meanAllRank(), stats.meanAllScoreRatio())
		t.Logf("%s %-6s %-3s %6s %7s %7s %7s %7s %7s %7s %7s %7s %8s %8s %8s %8s %8s %8s",
			result.name, "frame", "sub", "center", "localT", "localF", "bcgT", "bcgF", "bestT", "bestF", "intRank", "winRank", "allRank", "winScore", "allScore", "xRMS", "swRMS", "excRMS")
		for _, row := range result.rows {
			t.Logf("%s %-6d %-3d %6d %7d %7d %7d %7d %7d %7d %7d %7d %8d %8.3f %8.3f %8.1f %8.1f %8.1f",
				result.name, row.frame, row.sub, row.centre,
				row.localT, row.localFrac, row.refT, row.refFrac,
				row.fullBestT, row.fullBestF,
				row.intRank, row.fullRank, row.allRank,
				row.refToLocalScoreRatio, row.refToFullScoreRatio,
				row.xRMS, row.swMemRMS, row.oldExcRMS)
		}
	}
}

func TestExternalSampleClosedLoopFocusedSweep(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_CLOSEDLOOP_SWEEP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_CLOSEDLOOP_SWEEP=1 to run user-sample focused closed-loop sweep")
	}
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
	ref := src[:originalSamples]

	modes := []closedLoopSurfaceMode{
		{name: "surface prodlike", speechStart: 80, pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "start40", speechStart: 40, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "start120", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "start160", speechStart: 160, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "raw pitch corr", speechStart: 80, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "allow negative", speechStart: 80, pitchMode: "allowNegative", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "decoder short norm", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center fullbest", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullBest", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center submul", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleFullBest", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center half", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "topHalf", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center preferhalf", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "topPreferHalf", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center double", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "topDouble", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "center preferdbl", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "topPreferDouble", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "frac zero", speechStart: 80, pitchMode: "normalized", transmitFracZero: true, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "win1", speechStart: 80, pitchMode: "normalized", halfWindow: 1, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "win5", speechStart: 80, pitchMode: "normalized", halfWindow: 5, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "win10", speechStart: 80, pitchMode: "normalized", halfWindow: 10, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "win20", speechStart: 80, pitchMode: "normalized", halfWindow: 20, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "win60", speechStart: 80, pitchMode: "normalized", halfWindow: 60, xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "unquant residual", speechStart: 80, residualA: "unquant", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "unquant filter", speechStart: 80, filterA: "unquant", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "unquant all", speechStart: 80, residualA: "unquant", filterA: "unquant", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "zero target mem", speechStart: 80, targetMem: "zero", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "zero residual mem", speechStart: 80, residualMem: "zero", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "zero residual ext", speechStart: 80, residualExt: "zero", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "weighted direct", speechStart: 80, targetMode: "weightedDirect", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "aprime filter", speechStart: 80, filterA: "aprime", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb sharpen h", speechStart: 80, fcbMode: "sharpenH", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb sharpen h prod", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "sharpenH", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb annexA90", speechStart: 80, fcbMode: "annexA90", pitchMode: "normalized", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb score p0", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "score:p0", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb score p05", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "score:p05", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb score p15", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "score:p15", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb score p2", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "score:p2", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb tracktop1", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "tracktop:1", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb tracktop2", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "tracktop:2", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb tracktop3", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "tracktop:3", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb tracktop4", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "tracktop:4", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb tracktop6", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "tracktop:6", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb first3top10", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "first3top:10", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb first3top20", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "first3top:20", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb first3top40", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "first3top:40", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb first3top90", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "first3top:90", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb threshold60", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "thresholdscan:60", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb threshold90", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "thresholdscan:90", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb threshold180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "fcb target trunc", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", fcbMode: "targetTrunc", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "decoder short native", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", gainMode: "native", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 1, gpcDen: 1},
		{name: "decoder short fullcost", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", gainMode: "fullCost", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "decoder short rawlinear", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", gainMode: "rawLinear", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
	}

	tmp := t.TempDir()
	t.Logf("external sample focused closed-loop sweep: %s", path)
	t.Logf("%-18s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")

	currentFrames := encodeBitstreamFrames(t, src)
	logExternalSampleFramesQuality(t, tmp, "current", currentFrames, ref, originalSamples)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
		logExternalSampleFramesQuality(t, tmp, mode.name, framesOut, ref, originalSamples)
	}
}

func TestExternalSamplePitchCenterThresholdSweep(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_PITCH_CENTER_SWEEP") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_PITCH_CENTER_SWEEP=1 to run user-sample pitch-center threshold sweep")
	}
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
	ref := src[:originalSamples]

	modes := []closedLoopSurfaceMode{
		{name: "current", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "submul110", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleRatio110", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "submul125", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleRatio125", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "submul150", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleRatio150", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "submul175", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleRatio175", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "submul200", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "submultipleRatio200", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "harmonic110", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "harmonicRatio110", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "harmonic125", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "harmonicRatio125", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "harmonic150", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "harmonicRatio150", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full125", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio125", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full150", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio150", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full175", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio175", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full125+t180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio125", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full150+t180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio150", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "full175+t180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "fullRatio175", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop150", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio150", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop175", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio175", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop200", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio200", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop150+t180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio150", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop175+t180", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio175", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
		{name: "drop175+t180 g1", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio175", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "drop175+t180 g2", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio175", fcbMode: "thresholdscan:180", xNum: 1, xDen: 2, yNum: 9, yDen: 2, gpcNum: 5, gpcDen: 2},
		{name: "drop175+t180 g3", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", centerMode: "dropRatio175", fcbMode: "thresholdscan:180", xNum: 2, xDen: 5, yNum: 7, yDen: 2, gpcNum: 5, gpcDen: 3},
	}

	tmp := t.TempDir()
	t.Logf("external sample pitch-center threshold sweep: %s", path)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	for _, mode := range modes {
		frames := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
		ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
		t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			mode.name, ff.shift, ff.rms, ff.globalSNR, ff.segSNR, ff.corr, ff.rmsRatio,
			ff.peak, ff.nearClip, local.globalSNR, local.segSNR, local.nearClip)
	}
}

func TestExternalSampleGainScaleGridDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_SCALE_GRID") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_SCALE_GRID=1 to run user-sample gain-search scale grid")
	}
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
	ref := src[:originalSamples]

	type ratio struct {
		num int32
		den int32
	}
	xScales := []ratio{{2, 5}, {1, 2}, {3, 5}, {2, 3}, {3, 4}}
	yScales := []ratio{{5, 2}, {3, 1}, {7, 2}, {4, 1}, {9, 2}}
	gpcScales := []ratio{{4, 3}, {3, 2}, {5, 3}, {2, 1}, {5, 2}}

	type result struct {
		name  string
		ff    externalQualityMetrics
		local externalQualityMetrics
	}
	tmp := t.TempDir()
	results := make([]result, 0, 1+len(xScales)*len(yScales)*len(gpcScales))
	currentFF, currentLocal := measureExternalSampleFramesQualityPair(t, tmp, "current", encodeBitstreamFrames(t, src), ref, originalSamples)
	results = append(results, result{name: "current", ff: currentFF, local: currentLocal})

	for _, x := range xScales {
		for _, y := range yScales {
			for _, gpc := range gpcScales {
				mode := closedLoopSurfaceMode{
					name:        fmt.Sprintf("x%d_%d_y%d_%d_gpc%d_%d", x.num, x.den, y.num, y.den, gpc.num, gpc.den),
					speechStart: 80,
					acbMode:     "decoderShort",
					pitchMode:   "decoderShortNorm",
					xNum:        x.num,
					xDen:        x.den,
					yNum:        y.num,
					yDen:        y.den,
					gpcNum:      gpc.num,
					gpcDen:      gpc.den,
				}
				frames := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
				ff, local := measureExternalSampleFramesQualityPair(t, tmp, mode.name, frames, ref, originalSamples)
				results = append(results, result{
					name: fmt.Sprintf("x=%d/%d y=%d/%d gpc=%d/%d", x.num, x.den, y.num, y.den, gpc.num, gpc.den),
					ff:   ff, local: local,
				})
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].ff.globalSNR == results[j].ff.globalSNR {
			return results[i].ff.segSNR > results[j].ff.segSNR
		}
		return results[i].ff.globalSNR > results[j].ff.globalSNR
	})
	t.Logf("external sample gain-search scale grid: %s", path)
	t.Logf("%-22s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	limit := 15
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		r := results[i]
		t.Logf("%-22s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
			r.name, r.ff.shift, r.ff.rms, r.ff.globalSNR, r.ff.segSNR, r.ff.corr, r.ff.rmsRatio,
			r.ff.peak, r.ff.nearClip, r.local.globalSNR, r.local.segSNR, r.local.nearClip)
	}
}

func TestExternalSampleGreedyGainRepairDiagnostic(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_SAMPLE_GAIN_REPAIR") != "1" {
		t.Skip("set G729_EXTERNAL_SAMPLE_GAIN_REPAIR=1 to run user-sample decoder-in-loop gain repair diagnostic")
	}
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
	ref := src[:originalSamples]

	currentFrames := encodeBitstreamFrames(t, src)
	repairedFrames, changes := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 32700, "ref")
	preserveFrames, preserveChanges := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 32700, "original")
	alwaysFrames, alwaysChanges := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 32700, "ref-always")
	always32300Frames, always32300Changes := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 32300, "ref-always")
	always32000Frames, always32000Changes := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 32000, "ref-always")
	always30000Frames, always30000Changes := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 30000, "ref-always")
	always28000Frames, always28000Changes := greedyExternalGainRepairFrames(t, currentFrames, ref, 80, 28000, "ref-always")

	tmp := t.TempDir()
	currentFF, currentLocal := measureExternalSampleFramesQualityPair(t, tmp, "current", currentFrames, ref, originalSamples)
	repairedFF, repairedLocal := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair", repairedFrames, ref, originalSamples)
	preserveFF, preserveLocal := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-preserve", preserveFrames, ref, originalSamples)
	alwaysFF, alwaysLocal := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-always", alwaysFrames, ref, originalSamples)
	always32300FF, always32300Local := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-always32300", always32300Frames, ref, originalSamples)
	always32000FF, always32000Local := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-always32000", always32000Frames, ref, originalSamples)
	always30000FF, always30000Local := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-always30000", always30000Frames, ref, originalSamples)
	always28000FF, always28000Local := measureExternalSampleFramesQualityPair(t, tmp, "gain-repair-always28000", always28000Frames, ref, originalSamples)

	t.Logf("external sample decoder-in-loop gain repair diagnostic: %s", path)
	t.Logf("changed gain fields: ref=%d preserve=%d always32700=%d always32300=%d always32000=%d always30000=%d always28000=%d / %d subframes", len(changes), len(preserveChanges), len(alwaysChanges), len(always32300Changes), len(always32000Changes), len(always30000Changes), len(always28000Changes), len(currentFrames)*2)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"current", currentFF.shift, currentFF.rms, currentFF.globalSNR, currentFF.segSNR, currentFF.corr, currentFF.rmsRatio,
		currentFF.peak, currentFF.nearClip, currentLocal.globalSNR, currentLocal.segSNR, currentLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"gain-repair", repairedFF.shift, repairedFF.rms, repairedFF.globalSNR, repairedFF.segSNR, repairedFF.corr, repairedFF.rmsRatio,
		repairedFF.peak, repairedFF.nearClip, repairedLocal.globalSNR, repairedLocal.segSNR, repairedLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"preserve", preserveFF.shift, preserveFF.rms, preserveFF.globalSNR, preserveFF.segSNR, preserveFF.corr, preserveFF.rmsRatio,
		preserveFF.peak, preserveFF.nearClip, preserveLocal.globalSNR, preserveLocal.segSNR, preserveLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always", alwaysFF.shift, alwaysFF.rms, alwaysFF.globalSNR, alwaysFF.segSNR, alwaysFF.corr, alwaysFF.rmsRatio,
		alwaysFF.peak, alwaysFF.nearClip, alwaysLocal.globalSNR, alwaysLocal.segSNR, alwaysLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always32300", always32300FF.shift, always32300FF.rms, always32300FF.globalSNR, always32300FF.segSNR, always32300FF.corr, always32300FF.rmsRatio,
		always32300FF.peak, always32300FF.nearClip, always32300Local.globalSNR, always32300Local.segSNR, always32300Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always32000", always32000FF.shift, always32000FF.rms, always32000FF.globalSNR, always32000FF.segSNR, always32000FF.corr, always32000FF.rmsRatio,
		always32000FF.peak, always32000FF.nearClip, always32000Local.globalSNR, always32000Local.segSNR, always32000Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always30000", always30000FF.shift, always30000FF.rms, always30000FF.globalSNR, always30000FF.segSNR, always30000FF.corr, always30000FF.rmsRatio,
		always30000FF.peak, always30000FF.nearClip, always30000Local.globalSNR, always30000Local.segSNR, always30000Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always28000", always28000FF.shift, always28000FF.rms, always28000FF.globalSNR, always28000FF.segSNR, always28000FF.corr, always28000FF.rmsRatio,
		always28000FF.peak, always28000FF.nearClip, always28000Local.globalSNR, always28000Local.segSNR, always28000Local.nearClip)
	t.Logf("%-6s %-3s %7s %7s %7s %7s %8s %8s %10s %10s",
		"frame", "sub", "oldGA", "oldGB", "newGA", "newGB", "oldNC", "newNC", "oldMSE", "newMSE")
	for _, change := range changes {
		t.Logf("%-6d %-3d %7d %7d %7d %7d %8d %8d %10d %10d",
			change.frame, change.sub, change.oldGA, change.oldGB, change.newGA, change.newGB,
			change.oldScore.nearClip, change.newScore.nearClip, change.oldScore.mse, change.newScore.mse)
	}
}

func TestExternalFFmpegBlackboxGreedyGainRepair_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_GAIN_REPAIR") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_GAIN_REPAIR=1 to run SPEECH decoder-in-loop gain repair diagnostic")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	const (
		bytesPerInFrame  = 2 * FrameSamples
		bytesPerBitFrame = 164
	)
	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.IN"))
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}
	bitData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.BIT"))
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])

	currentFrames := encodeBitstreamFrames(t, src)
	repairedFrames, changes := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 32700, "ref")
	alwaysFrames, alwaysChanges := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 32700, "ref-always")
	always32300Frames, always32300Changes := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 32300, "ref-always")
	always32000Frames, always32000Changes := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 32000, "ref-always")
	always30000Frames, always30000Changes := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 30000, "ref-always")
	always28000Frames, always28000Changes := greedyExternalGainRepairFrames(t, currentFrames, src, 80, 28000, "ref-always")

	tmp := t.TempDir()
	refRaw := filepath.Join(tmp, "speech-ref.g729")
	refPCM := filepath.Join(tmp, "speech-ref.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], refRaw)
	ffmpegDecodeRawG729(t, refRaw, refPCM)
	refDecoded := s16leToSamples(readFile(t, refPCM))
	if len(refDecoded) > totalSamples {
		refDecoded = refDecoded[:totalSamples]
	}
	refMetrics := externalQualityMetricsFor(src, refDecoded, 240)
	currentFF, currentLocal := measureExternalSampleFramesQualityPair(t, tmp, "speech-current", currentFrames, src, totalSamples)
	repairedFF, repairedLocal := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair", repairedFrames, src, totalSamples)
	alwaysFF, alwaysLocal := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair-always", alwaysFrames, src, totalSamples)
	always32300FF, always32300Local := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair-always32300", always32300Frames, src, totalSamples)
	always32000FF, always32000Local := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair-always32000", always32000Frames, src, totalSamples)
	always30000FF, always30000Local := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair-always30000", always30000Frames, src, totalSamples)
	always28000FF, always28000Local := measureExternalSampleFramesQualityPair(t, tmp, "speech-gain-repair-always28000", always28000Frames, src, totalSamples)

	t.Logf("SPEECH decoder-in-loop gain repair diagnostic")
	t.Logf("changed gain fields: ref=%d always32700=%d always32300=%d always32000=%d always30000=%d always28000=%d / %d subframes", len(changes), len(alwaysChanges), len(always32300Changes), len(always32000Changes), len(always30000Changes), len(always28000Changes), len(currentFrames)*2)
	t.Logf("%-14s %6s %10s %10s %10s %8s %8s %7s %8s %10s %10s %8s",
		"Mode", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "LocalSNR", "LocalSeg", "LocalNC")
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10s %10s %8s",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR, refMetrics.corr, refMetrics.rmsRatio,
		refMetrics.peak, refMetrics.nearClip, "-", "-", "-")
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"current", currentFF.shift, currentFF.rms, currentFF.globalSNR, currentFF.segSNR, currentFF.corr, currentFF.rmsRatio,
		currentFF.peak, currentFF.nearClip, currentLocal.globalSNR, currentLocal.segSNR, currentLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"gain-repair", repairedFF.shift, repairedFF.rms, repairedFF.globalSNR, repairedFF.segSNR, repairedFF.corr, repairedFF.rmsRatio,
		repairedFF.peak, repairedFF.nearClip, repairedLocal.globalSNR, repairedLocal.segSNR, repairedLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always", alwaysFF.shift, alwaysFF.rms, alwaysFF.globalSNR, alwaysFF.segSNR, alwaysFF.corr, alwaysFF.rmsRatio,
		alwaysFF.peak, alwaysFF.nearClip, alwaysLocal.globalSNR, alwaysLocal.segSNR, alwaysLocal.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always32300", always32300FF.shift, always32300FF.rms, always32300FF.globalSNR, always32300FF.segSNR, always32300FF.corr, always32300FF.rmsRatio,
		always32300FF.peak, always32300FF.nearClip, always32300Local.globalSNR, always32300Local.segSNR, always32300Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always32000", always32000FF.shift, always32000FF.rms, always32000FF.globalSNR, always32000FF.segSNR, always32000FF.corr, always32000FF.rmsRatio,
		always32000FF.peak, always32000FF.nearClip, always32000Local.globalSNR, always32000Local.segSNR, always32000Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always30000", always30000FF.shift, always30000FF.rms, always30000FF.globalSNR, always30000FF.segSNR, always30000FF.corr, always30000FF.rmsRatio,
		always30000FF.peak, always30000FF.nearClip, always30000Local.globalSNR, always30000Local.segSNR, always30000Local.nearClip)
	t.Logf("%-14s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10.2f %10.2f %8d",
		"always28000", always28000FF.shift, always28000FF.rms, always28000FF.globalSNR, always28000FF.segSNR, always28000FF.corr, always28000FF.rmsRatio,
		always28000FF.peak, always28000FF.nearClip, always28000Local.globalSNR, always28000Local.segSNR, always28000Local.nearClip)
}

func TestExternalFFmpegBlackboxLSPTopKSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run SPEECH expanded LSP VQ diagnostic")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	const (
		bytesPerInFrame  = 2 * FrameSamples
		bytesPerBitFrame = 164
	)
	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.IN"))
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}
	bitData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.BIT"))
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])

	tmp := t.TempDir()
	refRaw := filepath.Join(tmp, "speech-ref.g729")
	refPCM := filepath.Join(tmp, "speech-ref.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], refRaw)
	ffmpegDecodeRawG729(t, refRaw, refPCM)
	refFF := s16leToSamples(readFile(t, refPCM))
	if len(refFF) > totalSamples {
		refFF = refFF[:totalSamples]
	}
	refMetrics := externalQualityMetricsFor(src, refFF, 240)

	type mode struct {
		name string
		topK int
	}
	modes := []mode{
		{name: "current", topK: 0},
		{name: "lsp top1 pair", topK: 1},
		{name: "lsp top2 pair", topK: 2},
		{name: "lsp top4 pair", topK: 4},
		{name: "lsp top8 pair", topK: 8},
		{name: "lsp top16 pair", topK: 16},
	}

	t.Logf("SPEECH expanded LSP VQ diagnostic (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-16s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	t.Logf("%-16s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.rmsRatio, refMetrics.peak, refMetrics.nearClip)
	for _, m := range modes {
		var framesOut []bitstream.Frame
		if m.topK == 0 {
			framesOut = encodeBitstreamFrames(t, src)
		} else {
			framesOut = encodeBitstreamFramesLSPTopK(t, src, m.topK)
		}
		rawPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".g729")
		pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(m.name)+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", m.name, len(decoded), totalSamples)
		}
		q := externalQualityMetricsFor(src, decoded, 240)
		t.Logf("%-16s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			m.name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
	}
}

func nextEncoderOmegaForDiagnostic(t *testing.T, e *Encoder, pcm []int16, omega *[10]int16) {
	t.Helper()
	var processed [FrameSamples]int16
	e.pre.Process(pcm, processed[:])

	copy(e.oldSpeech[0:160], e.oldSpeech[80:240])
	copy(e.oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := e.lpc.Analyze(&e.oldSpeech, &aQ12); err != nil {
		t.Fatalf("lpc Analyze: %v", err)
	}
	var qQ15 [10]int16
	if err := lsp.LPToLSP(&aQ12, &qQ15); err != nil {
		if errors.Is(err, lsp.ErrLPCNonStable) {
			qQ15 = e.lspOld
			e.lspReuseCount++
		} else {
			t.Fatalf("LPToLSP: %v", err)
		}
	} else {
		e.lspOld = qQ15
	}
	lsp.LSPToLSF(&qQ15, omega)
}

func lspIndicesFromFrame(f bitstream.Frame) lsp.Indices {
	return lsp.Indices{
		L0: uint8(f.L0),
		L1: uint8(f.L1),
		L2: uint8(f.L2),
		L3: uint8(f.L3),
	}
}

type externalQualityMetrics struct {
	shift     int
	rms       float64
	globalSNR float64
	segSNR    float64
	corr      float64
	rmsRatio  float64
	peak      int
	nearClip  int
}

type externalWindowQualityMetrics struct {
	snr      float64
	corr     float64
	rmsRatio float64
	mse      float64
	peak     int
	nearClip int
}

type externalResidualNoiseMetrics struct {
	errorDB     float64
	highDB      float64
	highShareDB float64
	worstHighDB float64
	worstFrame  int
}

type externalSpectralBand struct {
	name     string
	lowHz    float64
	highHz   float64
	binStart int
	binEnd   int
}

var externalSpectralBands = []externalSpectralBand{
	{name: "0-300", lowHz: 0, highHz: 300},
	{name: "300-800", lowHz: 300, highHz: 800},
	{name: "800-1600", lowHz: 800, highHz: 1600},
	{name: "1600-2400", lowHz: 1600, highHz: 2400},
	{name: "2400-3400", lowHz: 2400, highHz: 3400},
	{name: "3400-3900", lowHz: 3400, highHz: 3900},
}

type externalSpectralProfile struct {
	bandEnergy []float64
	total      float64
}

type externalFFmpegBitstreamPatch struct {
	name  string
	frame int
	sub   int
	apply func([]bitstream.Frame)
}

type externalFFmpegPatchResult struct {
	name       string
	frame      int
	sub        int
	patchIndex int
	metrics    externalQualityMetrics
	window     externalDecodeWindowScore
}

type externalPitchPatchTraceResult struct {
	name string

	frame int
	sub   int

	oldT int
	oldF int
	newT int
	newF int
	bcgT int
	bcgF int

	metrics externalQualityMetrics
	window  externalDecodeWindowScore
}

type externalDecodeWindowScore struct {
	nearClip int
	peak     int
	mse      float64
}

type externalNearClipMarker struct {
	startSample    int
	endSample      int
	refStartSample int
	refEndSample   int
	startFrame     int
	endFrame       int
	count          int
	peak           int
	value          int
}

type externalGainPatchCandidate struct {
	name string
	ga   uint8
	gb   uint8
}

type externalPitchPatchCandidate struct {
	name   string
	intLag int16
	frac   int8
}

func externalQualityMetricsFor(ref, test []int16, maxShift int) externalQualityMetrics {
	shift, global, seg := bestAlignedSNR(ref, test, maxShift)
	aligned := alignByShift(ref, test, shift)
	refRMS := rmsAmp(ref)
	ratio := 0.0
	if refRMS > 0 {
		ratio = rmsAmp(test) / refRMS
	}
	peak, nearClip := externalPeakAndNearClip(test)
	return externalQualityMetrics{
		shift:     shift,
		rms:       rmsAmp(test),
		globalSNR: global,
		segSNR:    seg,
		corr:      corrCoeff(ref, aligned),
		rmsRatio:  ratio,
		peak:      peak,
		nearClip:  nearClip,
	}
}

func externalAlignedWindowQualityMetrics(ref, test []int16, shift, startFrame, endFrame, threshold int) externalWindowQualityMetrics {
	start := startFrame * FrameSamples
	end := (endFrame + 1) * FrameSamples
	if start < 0 {
		start = 0
	}
	if end > len(ref) {
		end = len(ref)
	}
	var sigE, testE, errE, cross float64
	var count int
	var out externalWindowQualityMetrics
	for i := start; i < end; i++ {
		j := i + shift
		if j < 0 || j >= len(test) {
			continue
		}
		r := float64(ref[i])
		y := float64(test[j])
		diff := r - y
		sigE += r * r
		testE += y * y
		errE += diff * diff
		cross += r * y
		v := int(test[j])
		if v < 0 {
			v = -v
		}
		if v > out.peak {
			out.peak = v
		}
		if v >= threshold {
			out.nearClip++
		}
		count++
	}
	if count == 0 {
		out.snr = math.Inf(-1)
		return out
	}
	out.mse = errE / float64(count)
	if errE <= 0 {
		out.snr = math.Inf(+1)
	} else if sigE > 0 {
		out.snr = 10 * math.Log10(sigE/errE)
	} else {
		out.snr = math.Inf(-1)
	}
	if sigE > 0 && testE > 0 {
		out.corr = cross / math.Sqrt(sigE*testE)
	}
	if sigE > 0 {
		out.rmsRatio = math.Sqrt(testE / sigE)
	}
	return out
}

func externalResidualNoiseMetricsFor(ref, test []int16, shift int) externalResidualNoiseMetrics {
	startRef, startTest, n := externalAlignedSampleWindow(len(ref), len(test), shift)
	if n <= 1 {
		return externalResidualNoiseMetrics{
			errorDB:     math.Inf(-1),
			highDB:      math.Inf(-1),
			highShareDB: math.Inf(-1),
			worstHighDB: math.Inf(-1),
			worstFrame:  -1,
		}
	}

	var signal, errEnergy, highEnergy float64
	var prevErr float64
	for i := 0; i < n; i++ {
		refSample := float64(ref[startRef+i])
		outSample := float64(test[startTest+i])
		err := outSample - refSample
		signal += refSample * refSample
		errEnergy += err * err
		if i > 0 {
			high := err - prevErr
			highEnergy += high * high
		}
		prevErr = err
	}

	refRMS := math.Sqrt(signal / float64(n))
	errRMS := math.Sqrt(errEnergy / float64(n))
	highRMS := math.Sqrt(highEnergy / float64(n-1))

	minFrameRefRMS := math.Max(200, refRMS*0.15)
	worstHighDB := math.Inf(-1)
	worstFrame := -1
	for frameStart := 0; frameStart+1 < n; frameStart += FrameSamples {
		frameEnd := frameStart + FrameSamples
		if frameEnd > n {
			frameEnd = n
		}
		var frameSignal, frameHigh float64
		prev := float64(test[startTest+frameStart]) - float64(ref[startRef+frameStart])
		for i := frameStart; i < frameEnd; i++ {
			refSample := float64(ref[startRef+i])
			outSample := float64(test[startTest+i])
			err := outSample - refSample
			frameSignal += refSample * refSample
			if i > frameStart {
				high := err - prev
				frameHigh += high * high
			}
			prev = err
		}
		if frameEnd-frameStart <= 1 || frameSignal <= 0 {
			continue
		}
		frameRefRMS := math.Sqrt(frameSignal / float64(frameEnd-frameStart))
		if frameRefRMS < minFrameRefRMS {
			continue
		}
		frameHighRMS := math.Sqrt(frameHigh / float64(frameEnd-frameStart-1))
		frameHighDB := 20 * math.Log10((frameHighRMS+1)/(frameRefRMS+1))
		if frameHighDB > worstHighDB {
			worstHighDB = frameHighDB
			worstFrame = (startRef + frameStart) / FrameSamples
		}
	}
	if math.IsInf(worstHighDB, -1) {
		worstHighDB = 20 * math.Log10((highRMS+1)/(refRMS+1))
	}

	return externalResidualNoiseMetrics{
		errorDB:     20 * math.Log10((errRMS+1)/(refRMS+1)),
		highDB:      20 * math.Log10((highRMS+1)/(refRMS+1)),
		highShareDB: 10 * math.Log10((highEnergy+1)/(2*errEnergy+1)),
		worstHighDB: worstHighDB,
		worstFrame:  worstFrame,
	}
}

func externalAlignedSampleWindow(refSamples, testSamples, shift int) (startRef, startTest, n int) {
	if shift >= 0 {
		startRef = 0
		startTest = shift
		n = min(refSamples, testSamples-shift)
		return
	}
	startRef = -shift
	startTest = 0
	n = min(refSamples+shift, testSamples)
	return
}

func logExternalQualityMetrics(t *testing.T, name string, m externalQualityMetrics) {
	t.Helper()
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
}

func externalSpectralProfileFor(samples []int16) externalSpectralProfile {
	const (
		sampleRate = 8000.0
		blockSize  = 256
		hopSize    = 128
	)
	profile := externalSpectralProfile{
		bandEnergy: make([]float64, len(externalSpectralBands)),
	}
	if len(samples) < blockSize {
		return profile
	}

	var window [blockSize]float64
	for n := 0; n < blockSize; n++ {
		window[n] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(n)/float64(blockSize-1))
	}
	bands := externalSpectralBandsWithBins(sampleRate, blockSize)

	frames := 0
	for start := 0; start+blockSize <= len(samples); start += hopSize {
		frames++
		for k := 1; k <= blockSize/2; k++ {
			bandIndex := externalSpectralBandIndexForBin(bands, k)
			if bandIndex < 0 {
				continue
			}
			power := externalGoertzelPower(samples[start:start+blockSize], window[:], k, blockSize)
			profile.bandEnergy[bandIndex] += power
			profile.total += power
		}
	}
	if frames > 0 {
		scale := float64(frames)
		for i := range profile.bandEnergy {
			profile.bandEnergy[i] /= scale
		}
		profile.total /= scale
	}
	return profile
}

func externalSpectralBandsWithBins(sampleRate float64, blockSize int) []externalSpectralBand {
	bands := make([]externalSpectralBand, len(externalSpectralBands))
	copy(bands, externalSpectralBands)
	for i := range bands {
		bands[i].binStart = int(math.Ceil(bands[i].lowHz * float64(blockSize) / sampleRate))
		if bands[i].binStart < 1 {
			bands[i].binStart = 1
		}
		bands[i].binEnd = int(math.Floor(bands[i].highHz * float64(blockSize) / sampleRate))
		if bands[i].binEnd > blockSize/2 {
			bands[i].binEnd = blockSize / 2
		}
	}
	return bands
}

func externalSpectralBandIndexForBin(bands []externalSpectralBand, bin int) int {
	for i, band := range bands {
		if bin >= band.binStart && bin <= band.binEnd {
			return i
		}
	}
	return -1
}

func externalGoertzelPower(samples []int16, window []float64, bin, blockSize int) float64 {
	coeff := 2 * math.Cos(2*math.Pi*float64(bin)/float64(blockSize))
	var s0, s1, s2 float64
	for n, sample := range samples {
		s0 = float64(sample)*window[n] + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	return s1*s1 + s2*s2 - coeff*s1*s2
}

func (p externalSpectralProfile) shareDB(band int) float64 {
	if p.total <= 0 || band < 0 || band >= len(p.bandEnergy) || p.bandEnergy[band] <= 0 {
		return math.Inf(-1)
	}
	return 10 * math.Log10(p.bandEnergy[band]/p.total)
}

func (p externalSpectralProfile) ratioDB(numBands, denBands []int) float64 {
	num := p.energyForBands(numBands)
	den := p.energyForBands(denBands)
	if num <= 0 || den <= 0 {
		return math.Inf(-1)
	}
	return 10 * math.Log10(num/den)
}

func (p externalSpectralProfile) energyForBands(bands []int) float64 {
	var energy float64
	for _, band := range bands {
		if band >= 0 && band < len(p.bandEnergy) {
			energy += p.bandEnergy[band]
		}
	}
	return energy
}

func payloadEqualPercent(a, b []byte) float64 {
	den := len(a)
	if len(b) > den {
		den = len(b)
	}
	if den == 0 {
		return 100
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	equal := 0
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			equal++
		}
	}
	return float64(equal) * 100 / float64(den)
}

func externalPeakAndNearClip(samples []int16) (peak, nearClip int) {
	for _, s := range samples {
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
		if v >= 32700 {
			nearClip++
		}
	}
	return peak, nearClip
}

func decodeExternalFramesWithFFmpeg(t *testing.T, tmp, name string, frames []bitstream.Frame, originalSamples int) []int16 {
	t.Helper()
	rawPath := filepath.Join(tmp, sanitizeExternalSampleName(name)+".g729")
	pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(name)+".s16le")
	writePackedFrames(t, frames, rawPath)
	ffmpegDecodeRawG729(t, rawPath, pcmPath)
	decoded := s16leToSamples(readFile(t, pcmPath))
	if len(decoded) > originalSamples {
		decoded = decoded[:originalSamples]
	}
	if len(decoded) < originalSamples {
		t.Fatalf("%s: ffmpeg decoded output too short: got %d want >= %d", name, len(decoded), originalSamples)
	}
	return decoded
}

func externalDecodeWindowScoreForFrames(ref, decoded []int16, shift, startFrame, endFrame, threshold int) externalDecodeWindowScore {
	start := startFrame * FrameSamples
	end := (endFrame + 1) * FrameSamples
	if start < 0 {
		start = 0
	}
	if end > len(ref) {
		end = len(ref)
	}
	var score externalDecodeWindowScore
	var sum float64
	var count int
	for i := start; i < end; i++ {
		j := i + shift
		if j < 0 || j >= len(decoded) {
			continue
		}
		v := int(decoded[j])
		if v < 0 {
			v = -v
		}
		if v > score.peak {
			score.peak = v
		}
		if v >= threshold {
			score.nearClip++
		}
		d := float64(ref[i]) - float64(decoded[j])
		sum += d * d
		count++
	}
	if count > 0 {
		score.mse = sum / float64(count)
	}
	return score
}

func externalDecodeWindowScoreForTargets(ref, decoded []int16, shift int, frames []int, threshold int) externalDecodeWindowScore {
	var total externalDecodeWindowScore
	var sum float64
	var count int
	for _, frame := range frames {
		score := externalDecodeWindowScoreForFrames(ref, decoded, shift, frame-1, frame+1, threshold)
		total.nearClip += score.nearClip
		if score.peak > total.peak {
			total.peak = score.peak
		}
		sum += score.mse
		count++
	}
	if count > 0 {
		total.mse = sum / float64(count)
	}
	return total
}

func externalNearClipFrames(samples []int16, shift, frameCount, threshold int) []int {
	seen := make(map[int]bool)
	for i, s := range samples {
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v < threshold {
			continue
		}
		if frame := i / FrameSamples; frame >= 0 && frame < frameCount {
			seen[frame] = true
		}
		if frame := (i - shift) / FrameSamples; i >= shift && frame >= 0 && frame < frameCount {
			seen[frame] = true
		}
	}
	frames := make([]int, 0, len(seen))
	for frame := range seen {
		frames = append(frames, frame)
	}
	sort.Ints(frames)
	return frames
}

func externalNearClipMarkers(samples []int16, shift, frameCount, threshold, maxGap int) []externalNearClipMarker {
	var markers []externalNearClipMarker
	var cur externalNearClipMarker
	lastHit := -1
	flush := func() {
		if cur.count == 0 {
			return
		}
		cur.refStartSample = cur.startSample - shift
		cur.refEndSample = cur.endSample - shift
		cur.startFrame = externalFrameIndexForSample(cur.refStartSample, frameCount)
		cur.endFrame = externalFrameIndexForSample(cur.refEndSample, frameCount)
		markers = append(markers, cur)
		cur = externalNearClipMarker{}
		lastHit = -1
	}

	for i, s := range samples {
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v < threshold {
			continue
		}
		if cur.count > 0 && i-lastHit > maxGap {
			flush()
		}
		if cur.count == 0 {
			cur.startSample = i
			cur.endSample = i
			cur.peak = v
			cur.value = int(s)
		}
		cur.endSample = i
		cur.count++
		if v > cur.peak {
			cur.peak = v
			cur.value = int(s)
		}
		lastHit = i
	}
	flush()
	return markers
}

func externalFrameIndexForSample(sample, frameCount int) int {
	if sample < 0 {
		return -1
	}
	frame := sample / FrameSamples
	if frame >= frameCount {
		return -1
	}
	return frame
}

func externalNearClipMarkerSummary(markers []externalNearClipMarker) string {
	parts := make([]string, 0, len(markers))
	for _, marker := range markers {
		parts = append(parts, fmt.Sprintf("%.3fs f%d..%d c%d p%d",
			float64(marker.startSample)/8000.0,
			marker.startFrame,
			marker.endFrame,
			marker.count,
			marker.peak))
	}
	return strings.Join(parts, ", ")
}

func externalFrameNeighborhood(frames []int, frameCount, radius int) []int {
	seen := make(map[int]bool)
	for _, frame := range frames {
		for f := frame - radius; f <= frame+radius; f++ {
			if f >= 0 && f < frameCount {
				seen[f] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for frame := range seen {
		out = append(out, frame)
	}
	sort.Ints(out)
	return out
}

func externalGainPatchCandidates(frame bitstream.Frame, sub int, limit int) []externalGainPatchCandidate {
	origGA := uint8(frame.GA1 & 7)
	origGB := uint8(frame.GB1 & 15)
	if sub == 1 {
		origGA = uint8(frame.GA2 & 7)
		origGB = uint8(frame.GB2 & 15)
	}
	origPhysGA := tables.GainImap1[origGA]
	origPhysGB := tables.GainImap2[origGB]
	origGP := int(tables.GainGBK1[origPhysGA][0]) + int(tables.GainGBK2[origPhysGB][0])
	origGamma := int(tables.GainGBK1[origPhysGA][1]) + int(tables.GainGBK2[origPhysGB][1])

	type candidate struct {
		ga    uint8
		gb    uint8
		score int
	}
	var all []candidate
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			if ga == origGA && gb == origGB {
				continue
			}
			physGA := tables.GainImap1[ga]
			physGB := tables.GainImap2[gb]
			gp := int(tables.GainGBK1[physGA][0]) + int(tables.GainGBK2[physGB][0])
			gamma := int(tables.GainGBK1[physGA][1]) + int(tables.GainGBK2[physGB][1])
			if gp > origGP || gamma > origGamma {
				continue
			}
			all = append(all, candidate{
				ga:    ga,
				gb:    gb,
				score: (origGP - gp) + (origGamma - gamma),
			})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].score == all[j].score {
			if all[i].ga == all[j].ga {
				return all[i].gb < all[j].gb
			}
			return all[i].ga < all[j].ga
		}
		return all[i].score < all[j].score
	})
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]externalGainPatchCandidate, 0, len(all))
	for _, c := range all {
		out = append(out, externalGainPatchCandidate{
			name: fmt.Sprintf("gain-%d/%d", c.ga, c.gb),
			ga:   c.ga,
			gb:   c.gb,
		})
	}
	return out
}

func externalPitchPatchCandidates(frame bitstream.Frame, sub int) []externalPitchPatchCandidate {
	seen := make(map[string]bool)
	var out []externalPitchPatchCandidate
	add := func(name string, intLag int, frac int) {
		key := fmt.Sprintf("%d/%d", intLag, frac)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, externalPitchPatchCandidate{
			name:   name + "-" + key,
			intLag: int16(intLag),
			frac:   int8(frac),
		})
	}
	t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(frame.P1))
	if sub == 0 {
		origKey := fmt.Sprintf("%d/%d", t1, frac1)
		for dt := -2; dt <= 2; dt++ {
			for _, frac := range []int{-1, 0, 1} {
				lag := t1 + dt
				if lag < 19 || lag > 143 {
					continue
				}
				p1 := clpitch.EncodeP1(int16(lag), int8(frac))
				rtLag, rtFrac := pitchidx.DecodeDelaySubframe1(p1)
				key := fmt.Sprintf("%d/%d", rtLag, rtFrac)
				if key == origKey {
					continue
				}
				add("pitch", rtLag, rtFrac)
			}
		}
		return out
	}
	t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(frame.P2), t1)
	origP2 := int(frame.P2 & 31)
	origKey := fmt.Sprintf("%d/%d", t2, frac2)
	for dp := -4; dp <= 4; dp++ {
		p2 := origP2 + dp
		if p2 < 0 || p2 > 31 {
			continue
		}
		rtLag, rtFrac := pitchidx.DecodeDelaySubframe2(uint8(p2), t1)
		key := fmt.Sprintf("%d/%d", rtLag, rtFrac)
		if key == origKey {
			continue
		}
		add("pitch", rtLag, rtFrac)
	}
	return out
}

func setExternalSubframeGain(frame *bitstream.Frame, sub int, ga, gb uint8) {
	if sub == 0 {
		frame.GA1 = uint16(ga & 7)
		frame.GB1 = uint16(gb & 15)
		return
	}
	frame.GA2 = uint16(ga & 7)
	frame.GB2 = uint16(gb & 15)
}

func setExternalSubframePitch(frame *bitstream.Frame, sub int, intLag int16, frac int8) {
	if sub == 0 {
		p1 := clpitch.EncodeP1(intLag, frac)
		frame.P1 = uint16(p1)
		frame.P0 = uint16(clpitch.EncodeP0(p1))
		return
	}
	t1, _ := pitchidx.DecodeDelaySubframe1(uint8(frame.P1))
	tmin, _ := clpitch.Subframe2Window(int16(t1))
	p2 := clpitch.EncodeP2(intLag, frac, tmin)
	if p2 <= 31 {
		frame.P2 = uint16(p2)
	}
}

func externalBCGDonorPatches(frames, bcgFrames []bitstream.Frame, frameIndex int, sub int) []externalFFmpegBitstreamPatch {
	if frameIndex < 0 || frameIndex >= len(frames) || frameIndex >= len(bcgFrames) {
		return nil
	}
	fidx := frameIndex
	sidx := sub
	return []externalFFmpegBitstreamPatch{
		{
			name:  "bcg-gain",
			frame: fidx,
			sub:   sidx,
			apply: func(out []bitstream.Frame) {
				copyExternalSubframeGain(&out[fidx], bcgFrames[fidx], sidx)
			},
		},
		{
			name:  "bcg-pitch",
			frame: fidx,
			sub:   sidx,
			apply: func(out []bitstream.Frame) {
				copyExternalSubframePitch(&out[fidx], bcgFrames[fidx], sidx)
			},
		},
		{
			name:  "bcg-pitch+gain",
			frame: fidx,
			sub:   sidx,
			apply: func(out []bitstream.Frame) {
				copyExternalSubframePitch(&out[fidx], bcgFrames[fidx], sidx)
				copyExternalSubframeGain(&out[fidx], bcgFrames[fidx], sidx)
			},
		},
		{
			name:  "bcg-sub",
			frame: fidx,
			sub:   sidx,
			apply: func(out []bitstream.Frame) {
				copyExternalSubframePitch(&out[fidx], bcgFrames[fidx], sidx)
				copyExternalSubframeFCB(&out[fidx], bcgFrames[fidx], sidx)
				copyExternalSubframeGain(&out[fidx], bcgFrames[fidx], sidx)
			},
		},
	}
}

func copyExternalSubframeGain(dst *bitstream.Frame, src bitstream.Frame, sub int) {
	if sub == 0 {
		dst.GA1 = src.GA1 & 7
		dst.GB1 = src.GB1 & 15
		return
	}
	dst.GA2 = src.GA2 & 7
	dst.GB2 = src.GB2 & 15
}

func copyExternalSubframePitch(dst *bitstream.Frame, src bitstream.Frame, sub int) {
	if sub == 0 {
		dst.P1 = src.P1 & 0xff
		dst.P0 = src.P0 & 1
		return
	}
	dst.P2 = src.P2 & 31
}

func copyExternalSubframeFCB(dst *bitstream.Frame, src bitstream.Frame, sub int) {
	if sub == 0 {
		dst.C1 = src.C1 & 0x1fff
		dst.S1 = src.S1 & 0xf
		return
	}
	dst.C2 = src.C2 & 0x1fff
	dst.S2 = src.S2 & 0xf
}

type externalDecodeClipStageRow struct {
	frame int
	sub   int
	n     int

	out   int16
	hp    int16
	spf   int16
	synth int16
	u     int16

	gpQ14 int16
	gcQ12 int16

	pitch int
	frac  int
	c     uint16
	s     uint8
	ga    uint8
	gb    uint8

	bcgPitch int
	bcgFrac  int
	bcgC     int
	bcgS     int
	bcgGA    int
	bcgGB    int

	uPeak int
	sPeak int
}

func collectExternalDecodeClipStageRows(
	t *testing.T,
	frames []bitstream.Frame,
	bcgFrames []bitstream.Frame,
	originalSamples int,
	threshold int,
	limit int,
) []externalDecodeClipStageRow {
	t.Helper()
	const diagPastExcLen = 153

	var lspDec lsp.Decoder
	var gainDec gain.Decoder
	var sy synth.Synthesizer
	var pf postfilter.Postfilter
	var pastExc [diagPastExcLen]int16
	var hpX [2]int16
	var hpY [2]int32
	var prevGpQ14 int16
	rows := make([]externalDecodeClipStageRow, 0, limit)

	for frameIndex, frame := range frames {
		sf1A, sf2A := lspDec.Decode(lsp.Indices{
			L0: uint8(frame.L0),
			L1: uint8(frame.L1),
			L2: uint8(frame.L2),
			L3: uint8(frame.L3),
		})
		tInt1, tFrac1 := pitchidx.DecodeDelaySubframe1(uint8(frame.P1))
		tInt2, tFrac2 := pitchidx.DecodeDelaySubframe2(uint8(frame.P2), tInt1)

		for sub := 0; sub < 2; sub++ {
			sfA := &sf1A
			tInt, tFrac := tInt1, tFrac1
			cPacked, sPacked := frame.C1, uint8(frame.S1)
			ga, gb := uint8(frame.GA1), uint8(frame.GB1)
			if sub == 1 {
				sfA = &sf2A
				tInt, tFrac = tInt2, tFrac2
				cPacked, sPacked = frame.C2, uint8(frame.S2)
				ga, gb = uint8(frame.GA2), uint8(frame.GB2)
			}
			bcgPitch, bcgFrac, bcgC, bcgS, bcgGA, bcgGB := externalSubframeFields(bcgFrames, frameIndex, sub)

			var v [FrameSamples / 2]int16
			pitchidx.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)

			var c [FrameSamples / 2]int16
			betaQ14 := fcb.ClampPitchGainForEnhancement(prevGpQ14)
			fcb.Decode(fcb.Indices{Positions: cPacked, Signs: sPacked}, tInt, betaQ14, &c)

			gainTaps := gainDec.DecodeWithFullTaps(gain.Indices{GA: ga, GB: gb}, &c)
			var u [FrameSamples / 2]int16
			synth.BuildExcitation(gainTaps.GpQ14Final, gainTaps.GcMantQ14, gainTaps.GcExp, &v, &c, &u)

			var s [FrameSamples / 2]int16
			sy.Filter(sfA, &u, &s)

			var sPf [FrameSamples / 2]int16
			pf.Filter(sfA, tInt, &s, &sPf)

			var hp [FrameSamples / 2]int16
			externalHPFilterDiag(&hpX, &hpY, &sPf, hp[:])

			out := hp
			pcm.ScaleUpSat(out[:], out[:])

			uPeak, _ := externalPeakAndNearClip(u[:])
			sPeak, _ := externalPeakAndNearClip(s[:])
			for n := 0; n < FrameSamples/2; n++ {
				sampleIndex := frameIndex*FrameSamples + sub*(FrameSamples/2) + n
				if sampleIndex >= originalSamples {
					continue
				}
				if externalAbsInt16(out[n]) < threshold {
					continue
				}
				rows = append(rows, externalDecodeClipStageRow{
					frame:    frameIndex,
					sub:      sub,
					n:        n,
					out:      out[n],
					hp:       hp[n],
					spf:      sPf[n],
					synth:    s[n],
					u:        u[n],
					gpQ14:    gainTaps.GpQ14Final,
					gcQ12:    gainTaps.GcQ12Final,
					pitch:    tInt,
					frac:     tFrac,
					c:        cPacked,
					s:        sPacked,
					ga:       ga,
					gb:       gb,
					bcgPitch: bcgPitch,
					bcgFrac:  bcgFrac,
					bcgC:     bcgC,
					bcgS:     bcgS,
					bcgGA:    bcgGA,
					bcgGB:    bcgGB,
					uPeak:    uPeak,
					sPeak:    sPeak,
				})
				if len(rows) >= limit {
					return rows
				}
			}

			copy(pastExc[:diagPastExcLen-FrameSamples/2], pastExc[FrameSamples/2:])
			copy(pastExc[diagPastExcLen-FrameSamples/2:], u[:])
			prevGpQ14 = gainTaps.GpQ14Final
		}
	}
	return rows
}

func externalSubframeFields(frames []bitstream.Frame, frameIndex int, sub int) (pitchDelay, pitchFrac, code, signs, ga, gb int) {
	if frameIndex < 0 || frameIndex >= len(frames) {
		return -1, -1, -1, -1, -1, -1
	}
	frame := frames[frameIndex]
	tInt1, tFrac1 := pitchidx.DecodeDelaySubframe1(uint8(frame.P1))
	if sub == 0 {
		return tInt1, tFrac1, int(frame.C1), int(frame.S1), int(frame.GA1), int(frame.GB1)
	}
	tInt2, tFrac2 := pitchidx.DecodeDelaySubframe2(uint8(frame.P2), tInt1)
	return tInt2, tFrac2, int(frame.C2), int(frame.S2), int(frame.GA2), int(frame.GB2)
}

type externalGainRepairDecodeState struct {
	gainDec gain.Decoder
	synth   synth.Synthesizer
	pf      postfilter.Postfilter
	pastExc [153]int16
	hpX     [2]int16
	hpY     [2]int32
	prevGp  int16
}

type externalGainRepairScore struct {
	nearClip int
	hardClip int
	mse      int64
}

type externalGainRepairChange struct {
	frame    int
	sub      int
	oldGA    uint8
	oldGB    uint8
	newGA    uint8
	newGB    uint8
	oldScore externalGainRepairScore
	newScore externalGainRepairScore
}

func greedyExternalGainRepairFrames(t *testing.T, frames []bitstream.Frame, ref []int16, refShift int, threshold int, objective string) ([]bitstream.Frame, []externalGainRepairChange) {
	t.Helper()
	out := append([]bitstream.Frame(nil), frames...)
	alwaysSearch := strings.HasSuffix(objective, "-always")
	objective = strings.TrimSuffix(objective, "-always")

	var lspDec lsp.Decoder
	var state externalGainRepairDecodeState
	changes := make([]externalGainRepairChange, 0)

	for frameIndex := range out {
		frame := out[frameIndex]
		sf1A, sf2A := lspDec.Decode(lsp.Indices{
			L0: uint8(frame.L0),
			L1: uint8(frame.L1),
			L2: uint8(frame.L2),
			L3: uint8(frame.L3),
		})
		tInt1, tFrac1 := pitchidx.DecodeDelaySubframe1(uint8(frame.P1))
		tInt2, tFrac2 := pitchidx.DecodeDelaySubframe2(uint8(frame.P2), tInt1)

		for sub := 0; sub < 2; sub++ {
			sfA := &sf1A
			tInt, tFrac := tInt1, tFrac1
			cPacked, sPacked := frame.C1, uint8(frame.S1)
			origGA, origGB := uint8(frame.GA1), uint8(frame.GB1)
			if sub == 1 {
				sfA = &sf2A
				tInt, tFrac = tInt2, tFrac2
				cPacked, sPacked = frame.C2, uint8(frame.S2)
				origGA, origGB = uint8(frame.GA2), uint8(frame.GB2)
			}

			baseSample := frameIndex*FrameSamples + sub*(FrameSamples/2)
			bestState, bestOut := externalDecodeGainRepairSubframe(state, sfA, tInt, tFrac, cPacked, sPacked, origGA, origGB)
			origOut := bestOut
			bestScore := externalScoreGainRepairCandidate(bestOut[:], origOut[:], ref, baseSample, refShift, threshold, objective)
			origScore := bestScore
			bestGA, bestGB := origGA, origGB

			if alwaysSearch || bestScore.nearClip > 0 {
				for ga := uint8(0); ga < 8; ga++ {
					for gb := uint8(0); gb < 16; gb++ {
						candState, candOut := externalDecodeGainRepairSubframe(state, sfA, tInt, tFrac, cPacked, sPacked, ga, gb)
						candScore := externalScoreGainRepairCandidate(candOut[:], origOut[:], ref, baseSample, refShift, threshold, objective)
						if externalGainRepairScoreLess(candScore, bestScore) {
							bestScore = candScore
							bestState = candState
							bestGA, bestGB = ga, gb
						}
					}
				}
			}

			if bestGA != origGA || bestGB != origGB {
				if sub == 0 {
					out[frameIndex].GA1 = uint16(bestGA)
					out[frameIndex].GB1 = uint16(bestGB)
				} else {
					out[frameIndex].GA2 = uint16(bestGA)
					out[frameIndex].GB2 = uint16(bestGB)
				}
				changes = append(changes, externalGainRepairChange{
					frame:    frameIndex,
					sub:      sub,
					oldGA:    origGA,
					oldGB:    origGB,
					newGA:    bestGA,
					newGB:    bestGB,
					oldScore: origScore,
					newScore: bestScore,
				})
			}
			_ = bestOut
			state = bestState
		}
	}
	return out, changes
}

func externalDecodeGainRepairSubframe(
	state externalGainRepairDecodeState,
	a *[11]int16,
	tInt int,
	tFrac int,
	cPacked uint16,
	sPacked uint8,
	ga uint8,
	gb uint8,
) (externalGainRepairDecodeState, [FrameSamples / 2]int16) {
	var v [FrameSamples / 2]int16
	pitchidx.AdaptiveCodebook(tInt, tFrac, state.pastExc[:], &v)

	var c [FrameSamples / 2]int16
	betaQ14 := fcb.ClampPitchGainForEnhancement(state.prevGp)
	fcb.Decode(fcb.Indices{Positions: cPacked, Signs: sPacked}, tInt, betaQ14, &c)

	gainTaps := state.gainDec.DecodeWithFullTaps(gain.Indices{GA: ga, GB: gb}, &c)
	var u [FrameSamples / 2]int16
	synth.BuildExcitation(gainTaps.GpQ14Final, gainTaps.GcMantQ14, gainTaps.GcExp, &v, &c, &u)

	var s [FrameSamples / 2]int16
	state.synth.Filter(a, &u, &s)

	var sPf [FrameSamples / 2]int16
	state.pf.Filter(a, tInt, &s, &sPf)

	var hp [FrameSamples / 2]int16
	externalHPFilterDiag(&state.hpX, &state.hpY, &sPf, hp[:])

	out := hp
	pcm.ScaleUpSat(out[:], out[:])

	copy(state.pastExc[:len(state.pastExc)-FrameSamples/2], state.pastExc[FrameSamples/2:])
	copy(state.pastExc[len(state.pastExc)-FrameSamples/2:], u[:])
	state.prevGp = gainTaps.GpQ14Final
	return state, out
}

func externalScoreGainRepairCandidate(out []int16, original []int16, ref []int16, baseSample int, refShift int, threshold int, objective string) externalGainRepairScore {
	if objective == "original" {
		return externalScoreGainRepairOutput(out, original, 0, 0, threshold)
	}
	return externalScoreGainRepairOutput(out, ref, baseSample, refShift, threshold)
}

func externalScoreGainRepairOutput(out []int16, ref []int16, baseSample int, refShift int, threshold int) externalGainRepairScore {
	var score externalGainRepairScore
	var sum int64
	var count int64
	for i, sample := range out {
		absSample := externalAbsInt16(sample)
		if absSample >= threshold {
			score.nearClip++
		}
		if absSample >= 32767 {
			score.hardClip++
		}
		refIndex := baseSample + i - refShift
		if refIndex < 0 || refIndex >= len(ref) {
			continue
		}
		diff := int64(int(sample) - int(ref[refIndex]))
		sum += diff * diff
		count++
	}
	if count == 0 {
		score.mse = 1<<63 - 1
	} else {
		score.mse = sum / count
	}
	return score
}

func externalGainRepairScoreLess(a externalGainRepairScore, b externalGainRepairScore) bool {
	if a.hardClip != b.hardClip {
		return a.hardClip < b.hardClip
	}
	if a.nearClip != b.nearClip {
		return a.nearClip < b.nearClip
	}
	return a.mse < b.mse
}

func externalHPFilterDiag(hpX *[2]int16, hpY *[2]int32, in *[FrameSamples / 2]int16, out []int16) {
	const (
		hpB0Q13    = 7699
		hpB1Q13    = -15399
		hpB2Q13    = 7699
		hpNegA1Q12 = 7918
		hpA2Q13    = 7667
	)
	x1 := hpX[0]
	x2 := hpX[1]
	y1 := hpY[0]
	y2 := hpY[1]
	for n := 0; n < FrameSamples/2; n++ {
		xn := in[n]
		ff := int32(hpB0Q13)*int32(xn) +
			int32(hpB1Q13)*int32(x1) +
			int32(hpB2Q13)*int32(x2)
		ff >>= 1
		fb := int64(hpNegA1Q12) * int64(y1)
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(y2)) >> 13
		acc := int64(ff) + fb
		yn := (acc + (1 << 11)) >> 12
		if yn > 32767 {
			yn = 32767
		} else if yn < -32768 {
			yn = -32768
		}
		out[n] = int16(yn)
		x2 = x1
		x1 = xn
		y2 = y1
		y1 = int32(acc)
	}
	hpX[0] = x1
	hpX[1] = x2
	hpY[0] = y1
	hpY[1] = y2
}

func externalAbsInt16(v int16) int {
	if v == -32768 {
		return 32768
	}
	if v < 0 {
		return int(-v)
	}
	return int(v)
}

type externalGainTraceRow struct {
	variant string
	frame   int
	sub     int

	intLag int16
	frac   int8

	gaBits uint8
	gbBits uint8
	gaPhys uint8
	gbPhys uint8

	bestSearchGABits uint8
	bestSearchGBBits uint8
	bestNativeGABits uint8
	bestNativeGBBits uint8

	gpUnqQ14       int16
	gpSelectedQ14  int16
	gpCommitQ14    int16
	gcQ12          int32
	gpcPredQ12     int32
	gpcSearchQ12   int32
	predLogSatQ10  int16
	predLogWideQ10 int32
	pastQuaEn0     int16

	gpOptQ14 int64
	gcOptQ12 int64

	searchRank int
	nativeRank int
	searchCost int64
	nativeCost int64

	xRMS   float64
	yRMS   float64
	uRMS   float64
	zPeak  int
	uPeak  int
	ewPeak int
}

func externalGainTraceFrameWindow() (startFrame, endFrame int) {
	startFrame, endFrame = 286, 312
	if spec := os.Getenv("G729_EXTERNAL_GAIN_TRACE_FRAMES"); spec != "" {
		var start, end int
		if n, err := fmt.Sscanf(spec, "%d:%d", &start, &end); n == 2 && err == nil {
			startFrame, endFrame = start, end
		}
	}
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame < startFrame {
		endFrame = startFrame
	}
	return startFrame, endFrame
}

func collectExternalGainTrace(
	t *testing.T,
	samples []int16,
	variant string,
	tuning encoderQualityTuning,
	startFrame, endFrame int,
) []externalGainTraceRow {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	enc.qualityTuning = tuning
	rows := make([]externalGainTraceRow, 0, (endFrame-startFrame+1)*2)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("%s lpcStep frame %d: %v", variant, frameIndex, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			before := *enc
			_, _ = enc.closedloopStep(sub)
			if frameIndex >= startFrame && frameIndex <= endFrame {
				rows = append(rows, externalGainTraceFromTransition(t, variant, frameIndex, sub, tuning, &before, enc))
			}
		}
	}
	return rows
}

func externalGainTraceFromTransition(
	t *testing.T,
	variant string,
	frameIndex int,
	sub int,
	tuning encoderQualityTuning,
	before, after *Encoder,
) externalGainTraceRow {
	t.Helper()
	var intLag int16
	var frac int8
	var cPacked uint16
	var sPacked uint8
	var gaBits uint8
	var gbBits uint8
	if sub == 0 {
		intLag, frac = after.intT1, after.frac1
		cPacked, sPacked = after.c1, after.s1
		gaBits, gbBits = after.ga1, after.gb1
	} else {
		intLag, frac = after.intT2, after.frac2
		cPacked, sPacked = after.c2, after.s2
		gaBits, gbBits = after.ga2, after.gb2
	}

	x, y, h, v, gpUnqQ14, _ := forcedPitchSurface(before, sub, intLag, frac)
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: cPacked, Signs: sPacked}, int(intLag), fcb.ClampPitchGainForEnhancement(before.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gaPhys := tables.GainImap1[gaBits&7]
	gbPhys := tables.GainImap2[gbBits&15]
	useNativeGainSearch := tuning&encoderTuningNativeGainSearch != 0
	useWideGainPredictor := tuning == 0 || tuning&encoderTuningWideGainPredictor != 0 || useNativeGainSearch
	gpSelectedQ14, gcMantQ14, gcExp := gainquant.Reconstruct(&before.pastQuaEn, &c, gaPhys, gbPhys)
	gpcPredQ12 := gainquant.PredictedGcQ12(&before.pastQuaEn, &c)
	if useWideGainPredictor {
		gpSelectedQ14, gcMantQ14, gcExp = gainquant.ReconstructWide(&before.pastQuaEn, &c, gaPhys, gbPhys)
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&before.pastQuaEn, &c)
	}
	xSearch := x
	ySearch := y
	gpcSearchQ12 := gpcPredQ12
	if tuning&encoderTuningGainSearchBias != 0 {
		scaleGainSearchVector(&xSearch, qualityGainSearchTargetScaleNum, qualityGainSearchTargetScaleDen)
		scaleGainSearchVector(&ySearch, qualityGainSearchAdaptiveContributionScaleNum, qualityGainSearchAdaptiveContributionScaleDen)
		gpcSearchQ12 = scaleInt32RatioForGainSearch(
			gpcPredQ12,
			qualityGainSearchFixedContributionScaleNum,
			qualityGainSearchFixedContributionScaleDen,
		)
	}
	ctx := gainSearchCostContext(&xSearch, &ySearch, &z)
	rank := externalGainTraceRank(&ctx, &before.pastQuaEn, &c, &x, &y, &z, gpcSearchQ12, gaPhys, gbPhys, useWideGainPredictor)

	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(after.prevGpQ14, gcMantQ14, gcExp, &v, &c, &u)
	uPeak, _ := externalPeakAndNearClip(u[:])
	zPeak, _ := externalPeakAndNearClip(z[:])
	ewPeak, _ := externalPeakAndNearClip(after.swMemErr[:])

	return externalGainTraceRow{
		variant:          variant,
		frame:            frameIndex,
		sub:              sub,
		intLag:           intLag,
		frac:             frac,
		gaBits:           gaBits & 7,
		gbBits:           gbBits & 15,
		gaPhys:           gaPhys,
		gbPhys:           gbPhys,
		bestSearchGABits: tables.GainMap1[rank.bestSearchGA],
		bestSearchGBBits: tables.GainMap2[rank.bestSearchGB],
		bestNativeGABits: tables.GainMap1[rank.bestNativeGA],
		bestNativeGBBits: tables.GainMap2[rank.bestNativeGB],
		gpUnqQ14:         gpUnqQ14,
		gpSelectedQ14:    gpSelectedQ14,
		gpCommitQ14:      after.prevGpQ14,
		gcQ12:            mantExpToQ12(gcMantQ14, gcExp),
		gpcPredQ12:       gpcPredQ12,
		gpcSearchQ12:     gpcSearchQ12,
		predLogSatQ10:    gain.PredictedLogGainSat16(&before.pastQuaEn),
		predLogWideQ10:   gain.PredictedLogGain(&before.pastQuaEn),
		pastQuaEn0:       before.pastQuaEn[0],
		gpOptQ14:         ctx.gpOptQ14,
		gcOptQ12:         ctx.gcOptQ12,
		searchRank:       rank.searchRank,
		nativeRank:       rank.nativeRank,
		searchCost:       rank.searchCost,
		nativeCost:       rank.nativeCost,
		xRMS:             rmsAmp(x[:]),
		yRMS:             rmsAmp(y[:]),
		uRMS:             rmsAmp(u[:]),
		zPeak:            zPeak,
		uPeak:            uPeak,
		ewPeak:           ewPeak,
	}
}

type externalGainTraceRankResult struct {
	searchRank int
	nativeRank int
	searchCost int64
	nativeCost int64

	bestSearchGA uint8
	bestSearchGB uint8
	bestNativeGA uint8
	bestNativeGB uint8
}

type externalGainPreselectMissStats struct {
	count int

	selectedSameFull int
	fullInPreselect  int
	fullGAInTop      int
	fullGBInTop      int
	gaMissOnly       int
	gbMissOnly       int
	bothMiss         int
	selectedRankSum  int64

	examples []externalGainPreselectMissRow
}

type externalGainPreselectMissRow struct {
	frame int
	sub   int

	selectedGABits uint8
	selectedGBBits uint8
	preBestGABits  uint8
	preBestGBBits  uint8
	fullBestGABits uint8
	fullBestGBBits uint8

	fullInPreselect bool
	fullGAInTop     bool
	fullGBInTop     bool
	selectedRank    int

	gpOptQ14 int64
	gcOptQ12 int64

	selectedCost int64
	fullCost     int64
}

func collectExternalGainPreselectMiss(t *testing.T, samples []int16, startFrame, endFrame int) externalGainPreselectMissStats {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	var stats externalGainPreselectMissStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("preselect miss lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			before := *enc
			_, _ = enc.closedloopStep(sub)
			row := externalGainPreselectMissFromTransition(t, frameIndex, sub, &before, enc)
			recordExternalGainPreselectMiss(&stats, row, frameIndex >= startFrame && frameIndex <= endFrame)
		}
	}
	return stats
}

func externalGainPreselectMissFromTransition(
	t *testing.T,
	frameIndex int,
	sub int,
	before, after *Encoder,
) externalGainPreselectMissRow {
	t.Helper()
	var intLag int16
	var frac int8
	var cPacked uint16
	var sPacked uint8
	var gaBits uint8
	var gbBits uint8
	if sub == 0 {
		intLag, frac = after.intT1, after.frac1
		cPacked, sPacked = after.c1, after.s1
		gaBits, gbBits = after.ga1&7, after.gb1&15
	} else {
		intLag, frac = after.intT2, after.frac2
		cPacked, sPacked = after.c2, after.s2
		gaBits, gbBits = after.ga2&7, after.gb2&15
	}

	x, y, h, _, _, _ := forcedPitchSurface(before, sub, intLag, frac)
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: cPacked, Signs: sPacked}, int(intLag), fcb.ClampPitchGainForEnhancement(before.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12Wide(&before.pastQuaEn, &c)
	ctx := gainSearchCostContextTargetBits(&x, &y, &z, encoderCoreGainPreselectTargetBits)
	selectedGA := tables.GainImap1[gaBits]
	selectedGB := tables.GainImap2[gbBits]
	preGA, preGB, _ := bestGainSearchCostCandidate(&ctx, gpcPredQ12, func(ga, gb uint8) bool {
		return gainSearchPreselectContains(&ctx, gpcPredQ12, ga, gb)
	})
	fullGA, fullGB, _ := bestGainSearchCostCandidate(&ctx, gpcPredQ12, nil)
	fullGAInTop, fullGBInTop := gainSearchPreselectAxisContains(&ctx, gpcPredQ12, fullGA, fullGB)

	selectedCost := gainSearchCost(&ctx, selectedGA, selectedGB, gpcPredQ12)
	fullCost := gainSearchCost(&ctx, fullGA, fullGB, gpcPredQ12)
	selectedRank := 1
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			if gainSearchCost(&ctx, ga, gb, gpcPredQ12) < selectedCost {
				selectedRank++
			}
		}
	}

	return externalGainPreselectMissRow{
		frame:           frameIndex,
		sub:             sub,
		selectedGABits:  gaBits,
		selectedGBBits:  gbBits,
		preBestGABits:   tables.GainMap1[preGA],
		preBestGBBits:   tables.GainMap2[preGB],
		fullBestGABits:  tables.GainMap1[fullGA],
		fullBestGBBits:  tables.GainMap2[fullGB],
		fullInPreselect: gainSearchPreselectContains(&ctx, gpcPredQ12, fullGA, fullGB),
		fullGAInTop:     fullGAInTop,
		fullGBInTop:     fullGBInTop,
		selectedRank:    selectedRank,
		gpOptQ14:        ctx.gpOptQ14,
		gcOptQ12:        ctx.gcOptQ12,
		selectedCost:    selectedCost,
		fullCost:        fullCost,
	}
}

func recordExternalGainPreselectMiss(stats *externalGainPreselectMissStats, row externalGainPreselectMissRow, focused bool) {
	stats.count++
	if row.selectedGABits == row.fullBestGABits && row.selectedGBBits == row.fullBestGBBits {
		stats.selectedSameFull++
	}
	if row.fullInPreselect {
		stats.fullInPreselect++
	}
	if row.fullGAInTop {
		stats.fullGAInTop++
	}
	if row.fullGBInTop {
		stats.fullGBInTop++
	}
	switch {
	case !row.fullGAInTop && !row.fullGBInTop:
		stats.bothMiss++
	case !row.fullGAInTop:
		stats.gaMissOnly++
	case !row.fullGBInTop:
		stats.gbMissOnly++
	}
	stats.selectedRankSum += int64(row.selectedRank)
	if focused || (!row.fullInPreselect && len(stats.examples) < 12) {
		stats.examples = append(stats.examples, row)
	}
}

func externalGainTraceRank(
	ctx *gainSearchCostCtx,
	past *[4]int16,
	c *[40]int16,
	x, y, z *[40]int16,
	gpcSearchQ12 int32,
	gaPhys, gbPhys uint8,
	wide bool,
) externalGainTraceRankResult {
	out := externalGainTraceRankResult{
		searchRank:   1,
		nativeRank:   1,
		searchCost:   gainSearchCost(ctx, gaPhys, gbPhys, gpcSearchQ12),
		bestSearchGA: gaPhys,
		bestSearchGB: gbPhys,
		bestNativeGA: gaPhys,
		bestNativeGB: gbPhys,
	}
	gpQ14, gcMantQ14, gcExp := reconstructExternalGainCandidate(past, c, gaPhys, gbPhys, wide)
	out.nativeCost = gainResidualEnergyQ0(x, y, z, gpQ14, gcMantQ14, gcExp)
	bestSearchCost := out.searchCost
	bestNativeCost := out.nativeCost
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			searchCost := gainSearchCost(ctx, gai, gbi, gpcSearchQ12)
			if searchCost < out.searchCost {
				out.searchRank++
			}
			if searchCost < bestSearchCost {
				bestSearchCost = searchCost
				out.bestSearchGA = gai
				out.bestSearchGB = gbi
			}
			gp, gcMant, gcExp := reconstructExternalGainCandidate(past, c, gai, gbi, wide)
			nativeCost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if nativeCost < out.nativeCost {
				out.nativeRank++
			}
			if nativeCost < bestNativeCost {
				bestNativeCost = nativeCost
				out.bestNativeGA = gai
				out.bestNativeGB = gbi
			}
		}
	}
	return out
}

func sanitizeExternalSampleName(name string) string {
	repl := strings.NewReplacer(" ", "_", "+", "_", "/", "_", ">", "_", "<", "_")
	return repl.Replace(name)
}

func logExternalBCGFieldAgreement(t *testing.T, ourFrames, bcgFrames []bitstream.Frame) {
	t.Helper()
	n := len(ourFrames)
	if len(bcgFrames) < n {
		n = len(bcgFrames)
	}
	type fieldStats struct {
		lsp, pitch, pitch1, pitch2, fcb, fcb1, fcb2, gain, gain1, gain2, all int
	}
	var s fieldStats
	for i := 0; i < n; i++ {
		our, bcg := ourFrames[i], bcgFrames[i]
		if our == bcg {
			s.all++
		}
		if our.L0 == bcg.L0 && our.L1 == bcg.L1 && our.L2 == bcg.L2 && our.L3 == bcg.L3 {
			s.lsp++
		}
		pitch1 := our.P1 == bcg.P1 && our.P0 == bcg.P0
		pitch2 := our.P2 == bcg.P2
		if pitch1 {
			s.pitch1++
		}
		if pitch2 {
			s.pitch2++
		}
		if pitch1 && pitch2 {
			s.pitch++
		}
		fcb1 := our.C1 == bcg.C1 && our.S1 == bcg.S1
		fcb2 := our.C2 == bcg.C2 && our.S2 == bcg.S2
		if fcb1 {
			s.fcb1++
		}
		if fcb2 {
			s.fcb2++
		}
		if fcb1 && fcb2 {
			s.fcb++
		}
		gain1 := our.GA1 == bcg.GA1 && our.GB1 == bcg.GB1
		gain2 := our.GA2 == bcg.GA2 && our.GB2 == bcg.GB2
		if gain1 {
			s.gain1++
		}
		if gain2 {
			s.gain2++
		}
		if gain1 && gain2 {
			s.gain++
		}
	}
	t.Logf("our-vs-bcg field agreement: all=%.2f%% lsp=%.2f%% pitch=%.2f%% pitch1=%.2f%% pitch2=%.2f%% fcb=%.2f%% fcb1=%.2f%% fcb2=%.2f%% gain=%.2f%% gain1=%.2f%% gain2=%.2f%%",
		percent(s.all, n), percent(s.lsp, n), percent(s.pitch, n), percent(s.pitch1, n), percent(s.pitch2, n),
		percent(s.fcb, n), percent(s.fcb1, n), percent(s.fcb2, n), percent(s.gain, n), percent(s.gain1, n), percent(s.gain2, n))
}

func logExternalSampleFramesQuality(t *testing.T, tmp, name string, frames []bitstream.Frame, ref []int16, originalSamples int) {
	t.Helper()
	q := measureExternalSampleFramesQuality(t, tmp, name, frames, ref, originalSamples)
	t.Logf("%-18s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		name, q.shift, q.rms, q.globalSNR, q.segSNR, q.corr, q.rmsRatio, q.peak, q.nearClip)
}

func measureExternalSampleFramesQuality(t *testing.T, tmp, name string, frames []bitstream.Frame, ref []int16, originalSamples int) externalQualityMetrics {
	t.Helper()
	ff, _ := measureExternalSampleFramesQualityPair(t, tmp, name, frames, ref, originalSamples)
	return ff
}

func measureExternalSampleFramesQualityPair(t *testing.T, tmp, name string, frames []bitstream.Frame, ref []int16, originalSamples int) (ffmpegMetrics, localMetrics externalQualityMetrics) {
	ffmpegMetrics, localMetrics, _, _ = measureExternalSampleFramesQualityPairWithAudio(t, tmp, name, frames, ref, originalSamples)
	return ffmpegMetrics, localMetrics
}

func measureExternalSampleFramesQualityPairWithAudio(t *testing.T, tmp, name string, frames []bitstream.Frame, ref []int16, originalSamples int) (ffmpegMetrics, localMetrics externalQualityMetrics, ffmpegDecoded, localDecoded []int16) {
	t.Helper()
	rawPath := filepath.Join(tmp, sanitizeExternalSampleName(name)+".g729")
	pcmPath := filepath.Join(tmp, sanitizeExternalSampleName(name)+".s16le")
	writePackedFrames(t, frames, rawPath)
	raw := readFile(t, rawPath)
	ffmpegDecodeRawG729(t, rawPath, pcmPath)
	ffmpegDecoded = s16leToSamples(readFile(t, pcmPath))
	if len(ffmpegDecoded) > originalSamples {
		ffmpegDecoded = ffmpegDecoded[:originalSamples]
	}
	if len(ffmpegDecoded) < originalSamples {
		t.Fatalf("%s: decoded output too short: got %d want >= %d", name, len(ffmpegDecoded), originalSamples)
	}
	localDecoded = decodeRawG729WithLocal(t, raw)
	if len(localDecoded) > originalSamples {
		localDecoded = localDecoded[:originalSamples]
	}
	if len(localDecoded) < originalSamples {
		t.Fatalf("%s: local decoded output too short: got %d want >= %d", name, len(localDecoded), originalSamples)
	}
	return externalQualityMetricsFor(ref, ffmpegDecoded, 240), externalQualityMetricsFor(ref, localDecoded, 240), ffmpegDecoded, localDecoded
}

func encodeBitstreamFramesClippedInputMode(t *testing.T, samples []int16, threshold int, cooldownFrames int, tuning encoderQualityTuning) ([]bitstream.Frame, int) {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	switched := 0
	cooldown := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if threshold > 0 && externalFrameNearClip(samples[off:off+FrameSamples], threshold) {
			cooldown = cooldownFrames
		}
		if cooldown > 0 {
			enc.qualityTuning = tuning
			switched++
			cooldown--
		} else {
			enc.qualityTuning = encoderQualityTuningAll
		}
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, switched
}

func encodeBitstreamFramesClippedOpenLoopTopVariant(t *testing.T, samples []int16, threshold int, cooldownFrames int, variant externalOpenLoopTopVariant) ([]bitstream.Frame, int, int) {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	switched := 0
	changed := 0
	cooldown := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if threshold > 0 && externalFrameNearClip(samples[off:off+FrameSamples], threshold) {
			cooldown = cooldownFrames
		}
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		diag := externalDiagnoseOpenLoopSplitFrame(enc)
		_ = enc.openloopStep()
		if cooldown > 0 && variant.mode != "" {
			switched++
			top := externalOpenLoopTopFromVariant(diag, variant.mode, enc.intT2)
			if top != enc.tOp {
				changed++
				enc.tOp = top
			}
			cooldown--
		}
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, switched, changed
}

func externalFrameNearClip(samples []int16, threshold int) bool {
	for _, sample := range samples {
		v := int(sample)
		if v < 0 {
			v = -v
		}
		if v >= threshold {
			return true
		}
	}
	return false
}

func encodeBitstreamFramesLSPTopK(t *testing.T, samples []int16, topK int) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		lpcStepLSPTopK(t, enc, samples[off:off+FrameSamples], topK)
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func externalLSPImpliedUniformInitMemory() [10]int16 {
	return [10]int16{2232, 4556, 7204, 9148, 11674, 13961, 16326, 19031, 21339, 23269}
}

func encodeBitstreamFramesLSPInitialMemory(t *testing.T, samples []int16, init [10]int16) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	for tap := range enc.freqPrev {
		enc.freqPrev[tap] = init
	}
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesForcedLSP(t *testing.T, samples []int16, lspFrames []bitstream.Frame) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	var forcedDec lsp.Decoder
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(lspFrames) {
			t.Fatalf("missing forced LSP frame %d", frameIndex)
		}
		lpcAnalysisPreludeForDiagnostic(t, enc, samples[off:off+FrameSamples])
		idx := lspIndicesFromFrame(lspFrames[frameIndex])
		enc.l0 = uint16(idx.L0)
		enc.l1 = uint16(idx.L1)
		enc.l2 = uint16(idx.L2)
		enc.l3 = uint16(idx.L3)
		enc.aHatSF1, enc.aHatSF2 = forcedDec.Decode(idx)
		lsp.CommitIndicesForDiagnostic(&enc.freqPrev, idx)

		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesForcedBCGStages(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, mode string) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	var forcedDec lsp.Decoder
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		forceLSP := mode == "bcgLSPNormal" || mode == "bcgLSPPitch" || mode == "bcgLSPPitchCodeOwnGain"
		if forceLSP {
			lpcAnalysisPreludeForDiagnostic(t, enc, samples[off:off+FrameSamples])
			idx := lspIndicesFromFrame(ref)
			enc.l0 = uint16(idx.L0)
			enc.l1 = uint16(idx.L1)
			enc.l2 = uint16(idx.L2)
			enc.l3 = uint16(idx.L3)
			enc.aHatSF1, enc.aHatSF2 = forcedDec.Decode(idx)
			lsp.CommitIndicesForDiagnostic(&enc.freqPrev, idx)
		} else if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		switch mode {
		case "bcgLSPNormal":
			_, _ = enc.closedloopStep(0)
			_, _ = enc.closedloopStep(1)
		case "bcgPitch":
			forceBCGPitchOwnCodeGainStep(enc, 0, int16(t1), int8(frac1), uint16(ref.P1))
			forceBCGPitchOwnCodeGainStep(enc, 1, int16(t2), int8(frac2), uint16(ref.P2))
		case "bcgLSPPitch":
			forceBCGPitchOwnCodeGainStep(enc, 0, int16(t1), int8(frac1), uint16(ref.P1))
			forceBCGPitchOwnCodeGainStep(enc, 1, int16(t2), int8(frac2), uint16(ref.P2))
		case "bcgPitchCodeOwnGain", "bcgLSPPitchCodeOwnGain":
			forceBCGCodeOwnGainStep(enc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1))
			forceBCGCodeOwnGainStep(enc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2))
		default:
			t.Fatalf("unknown forced BCG stage mode %q", mode)
		}

		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func forceBCGPitchOwnCodeGainStep(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16) {
	x, y, h, v, gp, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)
	e.fcbStep(sub, nil, nil, &x, &y, &h, &v, gp)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func forceBCGCodeOwnGainStep(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16, refC uint16, refS uint8) {
	x, y, h, v, _, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: refS}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	xSearch := x
	ySearch := y
	const (
		gainSearchTargetScaleNum               = 1
		gainSearchTargetScaleDen               = 2
		gainSearchAdaptiveContributionScaleNum = 7
		gainSearchAdaptiveContributionScaleDen = 2
		gainSearchFixedContributionScaleNum    = 5
		gainSearchFixedContributionScaleDen    = 3
	)
	scaleGainSearchVector(&xSearch, gainSearchTargetScaleNum, gainSearchTargetScaleDen)
	scaleGainSearchVector(&ySearch, gainSearchAdaptiveContributionScaleNum, gainSearchAdaptiveContributionScaleDen)
	gpcSearchQ12 := scaleInt32RatioForGainSearch(
		gpcPredQ12,
		gainSearchFixedContributionScaleNum,
		gainSearchFixedContributionScaleDen,
	)
	gaPhys, gbPhys, gpQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	gpQ14 = gainquant.Tame(gpQ14, &e.oldExc)
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)

	if sub == 0 {
		e.c1 = refC
		e.s1 = refS
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.c2 = refC
		e.s2 = refS
		e.ga2 = gaBits
		e.gb2 = gbBits
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(gpQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)
	e.prevGpQ14 = gpQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

type externalGainRankStats struct {
	count int

	sameBoth       int
	bcgInPreselect int

	bcgSearchRankSum  int64
	bcgSearchRankLE1  int
	bcgSearchRankLE4  int
	bcgSearchRankLE8  int
	bcgSearchRankLE32 int
	bcgSearchLowerOwn int

	bcgNativeRankSum  int64
	bcgNativeRankLE1  int
	bcgNativeRankLE4  int
	bcgNativeRankLE8  int
	bcgNativeRankLE32 int
	bcgNativeLowerOwn int

	examples []externalGainRankExample
}

type externalFCBRankStats struct {
	count int

	sameC    int
	sameS    int
	sameBoth int

	bcgTop1  int
	bcgTop4  int
	bcgTop8  int
	bcgTop32 int

	bcgSignMatchesLocal int

	examples []externalFCBRankExample
}

type externalFCBRankExample struct {
	frame int
	sub   int

	localC uint16
	localS uint8
	bcgC   uint16
	bcgS   uint8

	bcgTopK   int
	bcgLocalS uint8
}

type fcbSurfaceVariant struct {
	name    string
	target  string
	hMode   string
	phiMode string
	dMode   string
}

type externalFrameError struct {
	frame int

	refRMS   float64
	ourMSE   float64
	bcgMSE   float64
	deltaMSE float64

	ourPeak     int
	ourNearClip int
	bcgPeak     int
	bcgNearClip int
}

type externalFCBAnomalyTrace struct {
	frame int
	sub   int

	intLag int16
	frac   int8
	gpQ14  int16

	xRMS      float64
	yRMS      float64
	hRMS      float64
	xPrimeRMS float64
	dMaxAbs   int64

	localC uint16
	localS uint8
	bcgC   uint16
	bcgS   uint8

	bcgTopK int
	bcgRank int

	bcgToLocalScoreRatio float64
	bcgToBestScoreRatio  float64
	bcgToLocalDSumRatio  float64
}

type externalBCGStateDivergenceRow struct {
	frame int
	sub   int

	localSwMemRMS float64
	bcgSwMemRMS   float64
	oldExcCorr    float64

	xCorr      float64
	xPrimeCorr float64
	hCorr      float64
	yCorr      float64
	dCorr      float64

	bcgTopKLocal    int
	bcgTopKBCGState int

	bcgStateToLocalScoreRatio float64
}

type externalExcitationCommitTraceRow struct {
	targetFrame int
	frame       int
	sub         int

	localT    int16
	localFrac int8
	bcgT      int16
	bcgFrac   int8

	localGA uint8
	localGB uint8
	bcgGA   uint8
	bcgGB   uint8

	localGpQ14 int16
	bcgGpQ14   int16
	localGcQ12 int32
	bcgGcQ12   int32

	oldExcCorr    float64
	vCorr         float64
	cCorr         float64
	pitchTermCorr float64
	codeTermCorr  float64
	uCorr         float64

	localURMS float64
	bcgURMS   float64
}

type externalPitchTimelineRow struct {
	targetFrame int
	frame       int

	localTop    int16
	bcgStateTop int16

	range1Lag int
	range2Lag int
	range3Lag int
	range1Rel float64
	range2Rel float64
	range3Rel float64

	localT1    int16
	localFrac1 int8
	localT2    int16
	localFrac2 int8
	bcgT1      int16
	bcgFrac1   int8
	bcgT2      int16
	bcgFrac2   int8

	localGA1 uint8
	localGB1 uint8
	localGA2 uint8
	localGB2 uint8
	bcgGA1   uint8
	bcgGB1   uint8
	bcgGA2   uint8
	bcgGB2   uint8

	oldExcCorr float64
}

type externalExcitationCommitSnapshot struct {
	intLag int16
	frac   int8
	ga     uint8
	gb     uint8

	gpQ14 int16
	gcQ12 int32

	v         [clpitch.SubframeLen]int16
	c         [clpitch.SubframeLen]int16
	pitchTerm [clpitch.SubframeLen]int16
	codeTerm  [clpitch.SubframeLen]int16
	u         [clpitch.SubframeLen]int16
}

type externalBCGLSPTrajectoryRow struct {
	frame int

	local lsp.Indices
	bcg   lsp.Indices

	localCostLocal int64
	bcgCostLocal   int64
	localCostBcg   int64
	bcgCostBcg     int64

	aHatSF1Corr float64
	aHatSF2Corr float64
}

type externalFCBMixedSurfaceMode struct {
	name string
	x    string
	y    string
	h    string
}

type externalFCBMixedSurfaceRow struct {
	frame int
	sub   int
	mode  string

	rank       int
	scoreRatio float64
	signMatch  bool

	pulseSignMatches int
	signedDSumRatio  float64

	xRMS      float64
	yRMS      float64
	hRMS      float64
	xPrimeRMS float64
	dMaxAbs   int64
}

type externalFCBMixedSurfaceStats struct {
	count         int
	rankSum       int
	top32         int
	scoreRatioSum float64
	signMatch     int
}

func (s externalFCBMixedSurfaceStats) meanRank() float64 {
	if s.count == 0 {
		return 0
	}
	return float64(s.rankSum) / float64(s.count)
}

func (s externalFCBMixedSurfaceStats) meanScoreRatio() float64 {
	if s.count == 0 {
		return 0
	}
	return s.scoreRatioSum / float64(s.count)
}

type externalPitchRankRow struct {
	frame int
	sub   int

	centre    int16
	localT    int16
	localFrac int8
	refT      int16
	refFrac   int8
	fullBestT int16
	fullBestF int8

	inWindow bool
	intRank  int
	fullRank int
	allRank  int

	refToLocalScoreRatio float64
	refToFullScoreRatio  float64

	xRMS      float64
	hRMS      float64
	swMemRMS  float64
	oldExcRMS float64
}

type externalPitchRankStats struct {
	count    int
	inWindow int

	sameInt  int
	sameFrac int
	sameBoth int

	intTop1 int
	intTop3 int
	intTop8 int

	fullTop1 int
	fullTop3 int
	fullTop8 int

	intRankSum  int
	fullRankSum int
	allRankSum  int
	scoreRatSum float64
	allScoreSum float64
}

func (s externalPitchRankStats) meanIntRank() float64 {
	if s.inWindow == 0 {
		return 0
	}
	return float64(s.intRankSum) / float64(s.inWindow)
}

func (s externalPitchRankStats) meanFullRank() float64 {
	if s.inWindow == 0 {
		return 0
	}
	return float64(s.fullRankSum) / float64(s.inWindow)
}

func (s externalPitchRankStats) meanScoreRatio() float64 {
	if s.inWindow == 0 {
		return 0
	}
	return s.scoreRatSum / float64(s.inWindow)
}

func (s externalPitchRankStats) meanAllRank() float64 {
	if s.count == 0 {
		return 0
	}
	return float64(s.allRankSum) / float64(s.count)
}

func (s externalPitchRankStats) meanAllScoreRatio() float64 {
	if s.count == 0 {
		return 0
	}
	return s.allScoreSum / float64(s.count)
}

type externalGainRankExample struct {
	frame int
	sub   int

	ownGA uint8
	ownGB uint8
	bcgGA uint8
	bcgGB uint8

	searchRank     int
	nativeRank     int
	bcgInPreselect bool

	ownSearchCost int64
	bcgSearchCost int64
	ownNativeCost int64
	bcgNativeCost int64
}

func rankExternalFrameErrors(ref, our, bcg []int16, minRefRMS float64) []externalFrameError {
	frames := len(ref) / FrameSamples
	out := make([]externalFrameError, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * FrameSamples
		refFrame := ref[off : off+FrameSamples]
		refRMS := rmsAmp(refFrame)
		if refRMS < minRefRMS {
			continue
		}
		ourFrame := our[off : off+FrameSamples]
		bcgFrame := bcg[off : off+FrameSamples]
		ourPeak, ourNearClip := externalPeakAndNearClip(ourFrame)
		bcgPeak, bcgNearClip := externalPeakAndNearClip(bcgFrame)
		ourMSE := externalFrameMSE(refFrame, ourFrame)
		bcgMSE := externalFrameMSE(refFrame, bcgFrame)
		out = append(out, externalFrameError{
			frame:       frame,
			refRMS:      refRMS,
			ourMSE:      ourMSE,
			bcgMSE:      bcgMSE,
			deltaMSE:    ourMSE - bcgMSE,
			ourPeak:     ourPeak,
			ourNearClip: ourNearClip,
			bcgPeak:     bcgPeak,
			bcgNearClip: bcgNearClip,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].deltaMSE == out[j].deltaMSE {
			return out[i].ourMSE > out[j].ourMSE
		}
		return out[i].deltaMSE > out[j].deltaMSE
	})
	return out
}

func externalFrameMSE(a, b []int16) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return sum / float64(n)
}

func collectExternalBCGFCBAnomalyTrace(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	selected map[int]bool,
	forceLSP bool,
	commit string,
) []externalFCBAnomalyTrace {
	t.Helper()
	enc := NewEncoder()
	var forcedDec lsp.Decoder
	var traces []externalFCBAnomalyTrace
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if forceLSP {
			lpcAnalysisPreludeForDiagnostic(t, enc, samples[off:off+FrameSamples])
			idx := lspIndicesFromFrame(ref)
			enc.l0 = uint16(idx.L0)
			enc.l1 = uint16(idx.L1)
			enc.l2 = uint16(idx.L2)
			enc.l3 = uint16(idx.L3)
			enc.aHatSF1, enc.aHatSF2 = forcedDec.Decode(idx)
			lsp.CommitIndicesForDiagnostic(&enc.freqPrev, idx)
		} else if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		record := selected[frameIndex]
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		if tr, ok := observeExternalBCGFCBAnomalyTrace(t, enc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), frameIndex, record, commit); ok {
			traces = append(traces, tr)
		}
		if tr, ok := observeExternalBCGFCBAnomalyTrace(t, enc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), frameIndex, record, commit); ok {
			traces = append(traces, tr)
		}
	}
	return traces
}

func observeExternalBCGFCBAnomalyTrace(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	frameIndex int,
	record bool,
	commit string,
) (externalFCBAnomalyTrace, bool) {
	t.Helper()
	x, y, h, v, gp, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)
	hSearch := productionFCBSearchImpulse(&h, intLag, e.prevGpQ14)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var localPos [4]int8
	var sumOut [2]int64
	searchExternalFCBForEncoder(e, &dAbs, &phi, &localPos, &sumOut)
	localC := fcbsearch.PackC(&localPos)
	localS := fcbsearch.PackS(&localPos, &signs)
	bcgPos := unpackExternalFCBPositions(refC)

	var top [fcbsearch.SearchTopKMax][4]int8
	topN := fcbsearch.SearchTopK(&dAbs, &phi, &top, fcbsearch.SearchTopKMax)
	bcgTopK := 0
	for i := 0; i < topN; i++ {
		if fcbsearch.PackC(&top[i]) == refC {
			bcgTopK = i + 1
			break
		}
	}

	tr := externalFCBAnomalyTrace{}
	if record {
		localCsum, _, localScore := externalFCBCandidateScore(&dAbs, &phi, localPos)
		bcgCsum, _, bcgScore := externalFCBCandidateScore(&dAbs, &phi, bcgPos)
		bcgRank, bestScore := externalFCBExactRank(&dAbs, &phi, bcgPos)
		tr = externalFCBAnomalyTrace{
			frame:                frameIndex,
			sub:                  sub,
			intLag:               intLag,
			frac:                 frac,
			gpQ14:                gp,
			xRMS:                 rmsAmp(x[:]),
			yRMS:                 rmsAmp(y[:]),
			hRMS:                 rmsAmp(h[:]),
			xPrimeRMS:            rmsAmp(xPrime[:]),
			dMaxAbs:              maxAbsInt32Array(d),
			localC:               localC,
			localS:               localS,
			bcgC:                 refC,
			bcgS:                 refS,
			bcgTopK:              bcgTopK,
			bcgRank:              bcgRank,
			bcgToLocalScoreRatio: externalSafeRatio(bcgScore, localScore),
			bcgToBestScoreRatio:  externalSafeRatio(bcgScore, bestScore),
			bcgToLocalDSumRatio:  externalSafeRatio(float64(bcgCsum), float64(localCsum)),
		}
	}

	switch commit {
	case "local":
		e.fcbStep(sub, nil, nil, &x, &y, &h, &v, gp)
	case "bcg":
		commitExternalBCGCodeGainState(e, sub, refC, refS, refGA, refGB, intLag, &x, &y, &h, &v, sFrame)
	default:
		t.Fatalf("unknown FCB anomaly commit mode %q", commit)
	}
	copy(e.lpResidualMemQ[:], sFrame[30:40])

	return tr, record
}

func externalFCBCandidateScore(dAbs *[clpitch.SubframeLen]int32, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32, pos [4]int8) (cSum, eSum int64, score float64) {
	for i := 0; i < 4; i++ {
		pi := pos[i]
		cSum += int64(dAbs[pi])
		eSum += int64(phi[pi][pi])
		for j := 0; j < i; j++ {
			eSum += int64(phi[pos[j]][pi])
		}
	}
	if eSum <= 0 {
		return cSum, eSum, 0
	}
	return cSum, eSum, (float64(cSum) * float64(cSum)) / float64(eSum)
}

func productionFCBSearchImpulse(h *[clpitch.SubframeLen]int16, intLag int16, prevGpQ14 int16) [clpitch.SubframeLen]int16 {
	out := *h
	fcb.ApplyPitchEnhancement(&out, int(intLag), fcb.ClampPitchGainForEnhancement(prevGpQ14))
	return out
}

func searchExternalFCBForEncoder(
	e *Encoder,
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	if e.qualityFCBThresholdScanEnabled() {
		fcbsearch.SearchDepthFirstThresholdScan(
			dAbs, phi, positions, sumOut,
			fcbsearch.SearchThresholdScanDefaultLimit,
		)
		return
	}
	fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
}

func externalFCBExactRank(dAbs *[clpitch.SubframeLen]int32, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32, target [4]int8) (rank int, bestScore float64) {
	_, _, targetScore := externalFCBCandidateScore(dAbs, phi, target)
	rank = 1
	bestScore = targetScore
	for _, m0 := range track0Diag {
		for _, m1 := range track1Diag {
			for _, m2 := range track2Diag {
				for _, m3 := range track3Diag {
					_, _, score := externalFCBCandidateScore(dAbs, phi, [4]int8{m0, m1, m2, m3})
					if score > bestScore {
						bestScore = score
					}
					if score > targetScore {
						rank++
					}
				}
			}
		}
	}
	return rank, bestScore
}

func maxAbsInt32Array(v [clpitch.SubframeLen]int32) int64 {
	var max int64
	for _, x := range v {
		a := int64(x)
		if a < 0 {
			a = -a
		}
		if a > max {
			max = a
		}
	}
	return max
}

func externalSafeRatio(num, den float64) float64 {
	if den == 0 {
		if num == 0 {
			return 1
		}
		return 0
	}
	return num / den
}

func collectExternalBCGPitchRank(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	selected map[int]bool,
	activeRMS float64,
) (externalPitchRankStats, []externalPitchRankRow) {
	t.Helper()
	return collectExternalBCGPitchRankWithProfile(t, samples, bcgFrames, selected, activeRMS, EncoderProfileQuality)
}

func collectExternalBCGPitchRankWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	selected map[int]bool,
	activeRMS float64,
	profile EncoderProfile,
) (externalPitchRankStats, []externalPitchRankRow) {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	var stats externalPitchRankStats
	var rows []externalPitchRankRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		frameActive := rmsAmp(samples[off:off+FrameSamples]) >= activeRMS
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		row1 := externalBCGPitchRankForSubframe(t, enc, frameIndex, 0, int16(t1), int8(frac1))
		if frameActive {
			stats.add(row1)
		}
		if selected[frameIndex] {
			rows = append(rows, row1)
		}
		_, _ = enc.closedloopStep(0)

		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		row2 := externalBCGPitchRankForSubframe(t, enc, frameIndex, 1, int16(t2), int8(frac2))
		if frameActive {
			stats.add(row2)
		}
		if selected[frameIndex] {
			rows = append(rows, row2)
		}
		_, _ = enc.closedloopStep(1)
	}
	return stats, rows
}

func (s *externalPitchRankStats) add(row externalPitchRankRow) {
	s.count++
	s.allRankSum += row.allRank
	s.allScoreSum += row.refToFullScoreRatio
	if row.localT == row.refT {
		s.sameInt++
	}
	if row.localFrac == row.refFrac {
		s.sameFrac++
	}
	if row.localT == row.refT && row.localFrac == row.refFrac {
		s.sameBoth++
	}
	if !row.inWindow {
		return
	}
	s.inWindow++
	s.intRankSum += row.intRank
	s.fullRankSum += row.fullRank
	s.scoreRatSum += row.refToLocalScoreRatio
	if row.intRank == 1 {
		s.intTop1++
	}
	if row.intRank > 0 && row.intRank <= 3 {
		s.intTop3++
	}
	if row.intRank > 0 && row.intRank <= 8 {
		s.intTop8++
	}
	if row.fullRank == 1 {
		s.fullTop1++
	}
	if row.fullRank > 0 && row.fullRank <= 3 {
		s.fullTop3++
	}
	if row.fullRank > 0 && row.fullRank <= 8 {
		s.fullTop8++
	}
}

func externalBCGPitchRankForSubframe(
	t *testing.T,
	e *Encoder,
	frame int,
	sub int,
	refT int16,
	refFrac int8,
) externalPitchRankRow {
	t.Helper()
	x, h, xb, exc, centre := externalPitchRankSurface(e, sub)
	var localT int16
	var localFrac int8
	if e.qualityNormalizedAdaptivePitchSearchEnabled() {
		localT = e.searchPitchNormalizedAdaptive(&x, &h, exc[:], centre, sub)
		localFrac = e.refinePitchNormalizedAdaptive(&x, &h, exc[:], localT, sub == 1 || localT < 85, sub)
	} else {
		localT, _ = clpitch.SearchInteger(&xb, exc[:], centre, sub)
		localT, localFrac = refineProductionPitchFraction(&xb, exc[:], sub, localT, e.intT1)
	}
	localScore := externalPitchScoreForProfile(e, &x, &h, &xb, exc[:], localT, localFrac)
	bestFullT, bestFullF, bestFullScore := externalBestFullRangePitch(e, &x, &h, &xb, exc[:], sub)
	refScore := externalPitchScoreForProfile(e, &x, &h, &xb, exc[:], refT, refFrac)
	row := externalPitchRankRow{
		frame:     frame,
		sub:       sub,
		centre:    centre,
		localT:    localT,
		localFrac: localFrac,
		refT:      refT,
		refFrac:   refFrac,
		fullBestT: bestFullT,
		fullBestF: bestFullF,
		xRMS:      rmsAmp(x[:]),
		hRMS:      rmsAmp(h[:]),
		swMemRMS:  rmsAmp(e.swMemErr[:]),
		oldExcRMS: rmsAmp(e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:]),
	}
	row.allRank = externalFullRangePitchRank(e, &x, &h, &xb, exc[:], sub, refT, refFrac, refScore)
	row.refToFullScoreRatio = externalPitchScoreRatio(refScore, bestFullScore)

	kMin, kMax := closedLoopPitchSearchRange(centre, sub)
	if int(refT) < kMin || int(refT) > kMax {
		return row
	}
	if !externalPitchFracAllowed(sub, refT, refFrac) {
		return row
	}
	row.inWindow = true

	refIntScore := externalPitchScoreForProfile(e, &x, &h, &xb, exc[:], refT, 0)
	row.intRank = 1
	for k := kMin; k <= kMax; k++ {
		score := externalPitchScoreForProfile(e, &x, &h, &xb, exc[:], int16(k), 0)
		if externalPitchScoreGreater(score, refIntScore) {
			row.intRank++
		}
	}

	row.fullRank = 1
	for k := kMin; k <= kMax; k++ {
		fracs, n := externalPitchCandidateFracs(sub, int16(k))
		for i := 0; i < n; i++ {
			score := externalPitchScoreForProfile(e, &x, &h, &xb, exc[:], int16(k), fracs[i])
			if externalPitchScoreGreater(score, refScore) {
				row.fullRank++
			}
		}
	}
	row.refToLocalScoreRatio = externalPitchScoreRatio(refScore, localScore)
	return row
}

func externalPitchRankSurface(e *Encoder, sub int) (x, h, xb [clpitch.SubframeLen]int16, exc [closedLoopPitchSearchLen]int16, centre int16) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
		centre = e.tOp
	} else {
		aHat = &e.aHatSF2
		centre = e.intT1
	}
	sStart := 120 + 40*sub
	if e.qualityEarlyClosedLoopSpeechWindowEnabled() {
		sStart = 80 + 40*sub
	}
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])
	var r [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)
	e.closedLoopExcitationSearch(&r, &exc)
	return x, h, xb, exc, centre
}

func externalPitchScoreForProfile(
	e *Encoder,
	x, h, xb *[clpitch.SubframeLen]int16,
	exc []int16,
	intLag int16,
	frac int8,
) float64 {
	if e.qualityNormalizedAdaptivePitchSearchEnabled() {
		return e.normalizedAdaptivePitchScore(x, h, exc, intLag, frac)
	}
	return externalPitchCoreRN(xb, exc, intLag, frac)
}

func externalPitchCoreRN(
	xb *[clpitch.SubframeLen]int16,
	exc []int16,
	intLag int16,
	frac int8,
) float64 {
	var acc fixed.Word32
	for n := 0; n < clpitch.SubframeLen; n++ {
		s := clpitch.Interpolate3(exc, intLag-int16(n), frac)
		acc = fixed.LMac(acc, xb[n], s)
	}
	return float64(acc)
}

func externalBestFullRangePitch(
	e *Encoder,
	x, h, xb *[clpitch.SubframeLen]int16,
	exc []int16,
	sub int,
) (bestT int16, bestFrac int8, bestScore float64) {
	bestT = clpitch.PitchMinInt
	bestScore = math.Inf(-1)
	for k := clpitch.PitchMinInt; k <= clpitch.PitchMaxInt; k++ {
		fracs, n := externalPitchCandidateFracs(sub, int16(k))
		for i := 0; i < n; i++ {
			score := externalPitchScoreForProfile(e, x, h, xb, exc, int16(k), fracs[i])
			if externalPitchScoreGreater(score, bestScore) {
				bestScore = score
				bestT = int16(k)
				bestFrac = fracs[i]
			}
		}
	}
	return bestT, bestFrac, bestScore
}

func externalFullRangePitchRank(
	e *Encoder,
	x, h, xb *[clpitch.SubframeLen]int16,
	exc []int16,
	sub int,
	refT int16,
	refFrac int8,
	refScore float64,
) int {
	rank := 1
	for k := clpitch.PitchMinInt; k <= clpitch.PitchMaxInt; k++ {
		fracs, n := externalPitchCandidateFracs(sub, int16(k))
		for i := 0; i < n; i++ {
			score := externalPitchScoreForProfile(e, x, h, xb, exc, int16(k), fracs[i])
			if externalPitchScoreGreater(score, refScore) {
				rank++
			}
		}
	}
	return rank
}

func externalPitchCandidateFracs(sub int, intLag int16) ([3]int8, int) {
	if sub == 1 || intLag < 85 {
		return [3]int8{-1, 0, 1}, 3
	}
	return [3]int8{0, 0, 0}, 1
}

func externalPitchFracAllowed(sub int, intLag int16, frac int8) bool {
	fracs, n := externalPitchCandidateFracs(sub, intLag)
	for i := 0; i < n; i++ {
		if fracs[i] == frac {
			return true
		}
	}
	return false
}

func externalPitchScoreGreater(a, b float64) bool {
	if math.IsNaN(a) {
		return false
	}
	if math.IsNaN(b) {
		return true
	}
	return a > b
}

func externalPitchScoreRatio(num, den float64) float64 {
	if math.IsNaN(num) || math.IsInf(num, -1) || math.IsNaN(den) || math.IsInf(den, -1) || den == 0 {
		return 0
	}
	return num / den
}

func collectExternalBCGLSPTrajectory(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, selected map[int]bool) []externalBCGLSPTrajectoryRow {
	t.Helper()
	var analysis Encoder
	lsp.InitFreqPrev(&analysis.freqPrev)
	lsp.InitLSPOld(&analysis.lspOld)
	var localMem, bcgMem [4][10]int16
	lsp.InitFreqPrev(&localMem)
	lsp.InitFreqPrev(&bcgMem)
	var localDec, bcgDec lsp.Decoder
	var rows []externalBCGLSPTrajectoryRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		var omega [10]int16
		nextEncoderOmegaForDiagnostic(t, &analysis, samples[off:off+FrameSamples], &omega)

		localBefore := localMem
		localIdx := lsp.Quantize(&omega, &localMem)
		bcgIdx := lspIndicesFromFrame(bcgFrames[frameIndex])
		localCostLocal := lsp.TupleCostForDiagnostic(&omega, &localBefore, localIdx)
		bcgCostLocal := lsp.TupleCostForDiagnostic(&omega, &localBefore, bcgIdx)
		bcgBefore := bcgMem
		localCostBcg := lsp.TupleCostForDiagnostic(&omega, &bcgBefore, localIdx)
		bcgCostBcg := lsp.TupleCostForDiagnostic(&omega, &bcgBefore, bcgIdx)
		lsp.CommitIndicesForDiagnostic(&bcgMem, bcgIdx)

		localA1, localA2 := localDec.Decode(localIdx)
		bcgA1, bcgA2 := bcgDec.Decode(bcgIdx)
		if selected[frameIndex] {
			rows = append(rows, externalBCGLSPTrajectoryRow{
				frame:          frameIndex,
				local:          localIdx,
				bcg:            bcgIdx,
				localCostLocal: localCostLocal,
				bcgCostLocal:   bcgCostLocal,
				localCostBcg:   localCostBcg,
				bcgCostBcg:     bcgCostBcg,
				aHatSF1Corr:    corrCoeff(localA1[1:], bcgA1[1:]),
				aHatSF2Corr:    corrCoeff(localA2[1:], bcgA2[1:]),
			})
		}
	}
	return rows
}

func collectExternalBCGStateDivergence(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, selected map[int]bool) []externalBCGStateDivergenceRow {
	t.Helper()
	return collectExternalBCGStateDivergenceWithProfile(t, samples, bcgFrames, selected, EncoderProfileQuality)
}

func collectExternalBCGStateDivergenceWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	selected map[int]bool,
	profile EncoderProfile,
) []externalBCGStateDivergenceRow {
	t.Helper()
	localEnc := NewEncoderWithProfile(profile)
	bcgStateEnc := NewEncoderWithProfile(profile)
	var bcgLSPDec lsp.Decoder
	var rows []externalBCGStateDivergenceRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := localEnc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("local lpcStep frame %d: %v", frameIndex, err)
		}
		_ = localEnc.openloopStep()

		lpcAnalysisPreludeForDiagnostic(t, bcgStateEnc, samples[off:off+FrameSamples])
		idx := lspIndicesFromFrame(ref)
		bcgStateEnc.l0 = uint16(idx.L0)
		bcgStateEnc.l1 = uint16(idx.L1)
		bcgStateEnc.l2 = uint16(idx.L2)
		bcgStateEnc.l3 = uint16(idx.L3)
		bcgStateEnc.aHatSF1, bcgStateEnc.aHatSF2 = bcgLSPDec.Decode(idx)
		lsp.CommitIndicesForDiagnostic(&bcgStateEnc.freqPrev, idx)
		_ = bcgStateEnc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		if selected[frameIndex] {
			rows = append(rows,
				externalBCGStateDivergenceForSubframe(t, localEnc, bcgStateEnc, frameIndex, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1)),
				externalBCGStateDivergenceForSubframe(t, localEnc, bcgStateEnc, frameIndex, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2)),
			)
		}

		_, _ = localEnc.closedloopStep(0)
		commitExternalBCGStateSubframe(t, bcgStateEnc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1))
		_, _ = localEnc.closedloopStep(1)
		commitExternalBCGStateSubframe(t, bcgStateEnc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2))
	}
	return rows
}

func collectExternalBCGExcitationCommitTrace(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, targets []int, lookbackFrames int) []externalExcitationCommitTraceRow {
	t.Helper()
	return collectExternalBCGExcitationCommitTraceWithProfile(t, samples, bcgFrames, targets, lookbackFrames, EncoderProfileQuality)
}

func collectExternalBCGExcitationCommitTraceWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	targets []int,
	lookbackFrames int,
	profile EncoderProfile,
) []externalExcitationCommitTraceRow {
	t.Helper()
	localEnc := NewEncoderWithProfile(profile)
	bcgStateEnc := NewEncoderWithProfile(profile)
	var bcgLSPDec lsp.Decoder
	var rows []externalExcitationCommitTraceRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := localEnc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("local lpcStep frame %d: %v", frameIndex, err)
		}
		_ = localEnc.openloopStep()

		lpcAnalysisPreludeForDiagnostic(t, bcgStateEnc, samples[off:off+FrameSamples])
		idx := lspIndicesFromFrame(ref)
		bcgStateEnc.l0 = uint16(idx.L0)
		bcgStateEnc.l1 = uint16(idx.L1)
		bcgStateEnc.l2 = uint16(idx.L2)
		bcgStateEnc.l3 = uint16(idx.L3)
		bcgStateEnc.aHatSF1, bcgStateEnc.aHatSF2 = bcgLSPDec.Decode(idx)
		lsp.CommitIndicesForDiagnostic(&bcgStateEnc.freqPrev, idx)
		_ = bcgStateEnc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		rows = append(rows, collectExternalBCGExcitationCommitTraceSubframe(t, localEnc, bcgStateEnc, targets, lookbackFrames, frameIndex, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1))...)
		rows = append(rows, collectExternalBCGExcitationCommitTraceSubframe(t, localEnc, bcgStateEnc, targets, lookbackFrames, frameIndex, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2))...)
	}
	return rows
}

func collectExternalBCGPitchTimeline(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, targets []int, lookbackFrames int) []externalPitchTimelineRow {
	t.Helper()
	return collectExternalBCGPitchTimelineWithProfile(t, samples, bcgFrames, targets, lookbackFrames, EncoderProfileQuality)
}

func collectExternalBCGPitchTimelineWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	targets []int,
	lookbackFrames int,
	profile EncoderProfile,
) []externalPitchTimelineRow {
	t.Helper()
	localEnc := NewEncoderWithProfile(profile)
	bcgStateEnc := NewEncoderWithProfile(profile)
	var bcgLSPDec lsp.Decoder
	var rows []externalPitchTimelineRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := localEnc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("local lpcStep frame %d: %v", frameIndex, err)
		}
		openDiag := externalDiagnoseOpenLoopSplitFrame(localEnc)
		localTop := localEnc.openloopStep()
		r1Rel, r2Rel, r3Rel := externalOpenLoopRangeRel(openDiag)

		lpcAnalysisPreludeForDiagnostic(t, bcgStateEnc, samples[off:off+FrameSamples])
		idx := lspIndicesFromFrame(ref)
		bcgStateEnc.l0 = uint16(idx.L0)
		bcgStateEnc.l1 = uint16(idx.L1)
		bcgStateEnc.l2 = uint16(idx.L2)
		bcgStateEnc.l3 = uint16(idx.L3)
		bcgStateEnc.aHatSF1, bcgStateEnc.aHatSF2 = bcgLSPDec.Decode(idx)
		lsp.CommitIndicesForDiagnostic(&bcgStateEnc.freqPrev, idx)
		bcgStateTop := bcgStateEnc.openloopStep()

		oldExcCorr := corrCoeff(localEnc.oldExc[len(localEnc.oldExc)-clpitch.PitchMaxInt:], bcgStateEnc.oldExc[len(bcgStateEnc.oldExc)-clpitch.PitchMaxInt:])
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)

		_, _ = localEnc.closedloopStep(0)
		_, _ = localEnc.closedloopStep(1)
		commitExternalBCGStateSubframe(t, bcgStateEnc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1))
		commitExternalBCGStateSubframe(t, bcgStateEnc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2))

		for _, target := range targets {
			if frameIndex < target-lookbackFrames || frameIndex > target {
				continue
			}
			rows = append(rows, externalPitchTimelineRow{
				targetFrame: target,
				frame:       frameIndex,
				localTop:    localTop,
				bcgStateTop: bcgStateTop,
				range1Lag:   openDiag.range1.lag,
				range2Lag:   openDiag.range2.lag,
				range3Lag:   openDiag.range3.lag,
				range1Rel:   r1Rel,
				range2Rel:   r2Rel,
				range3Rel:   r3Rel,
				localT1:     localEnc.intT1,
				localFrac1:  localEnc.frac1,
				localT2:     localEnc.intT2,
				localFrac2:  localEnc.frac2,
				bcgT1:       int16(t1),
				bcgFrac1:    int8(frac1),
				bcgT2:       int16(t2),
				bcgFrac2:    int8(frac2),
				localGA1:    localEnc.ga1,
				localGB1:    localEnc.gb1,
				localGA2:    localEnc.ga2,
				localGB2:    localEnc.gb2,
				bcgGA1:      uint8(ref.GA1),
				bcgGB1:      uint8(ref.GB1),
				bcgGA2:      uint8(ref.GA2),
				bcgGB2:      uint8(ref.GB2),
				oldExcCorr:  oldExcCorr,
			})
		}
	}
	return rows
}

func externalOpenLoopSplitWSP(e *Encoder) [223]int16 {
	s := (*[FrameSamples]int16)(e.oldSpeech[120:200])

	var aw1, aw2, aPrime1, aPrime2 [11]int16
	externalOpenLoopGammaWeightLP(&e.aHatSF1, &aw1)
	externalOpenLoopCombineWith07(&aw1, &aPrime1)
	externalOpenLoopGammaWeightLP(&e.aHatSF2, &aw2)
	externalOpenLoopCombineWith07(&aw2, &aPrime2)

	var s1, s2, r1, r2, sw1, sw2 [clpitch.SubframeLen]int16
	copy(s1[:], s[:clpitch.SubframeLen])
	copy(s2[:], s[clpitch.SubframeLen:])
	lpResidualSubframe(&s1, &e.aHatSF1, &e.lpResidualMem, &r1)
	var mem2 [10]int16
	copy(mem2[:], s1[clpitch.SubframeLen-10:])
	lpResidualSubframe(&s2, &e.aHatSF2, &mem2, &r2)

	targetSignalFromWeightedLPDiagnostic(&aPrime1, &r1, &e.swMem, &sw1)
	copy(mem2[:], sw1[clpitch.SubframeLen-10:])
	targetSignalFromWeightedLPDiagnostic(&aPrime2, &r2, &mem2, &sw2)

	var wsp [223]int16
	copy(wsp[:143], e.oldWspeech[:])
	copy(wsp[143:183], sw1[:])
	copy(wsp[183:], sw2[:])
	return wsp
}

func externalOpenLoopProductionResults(e *Encoder) (production, normalized olpitch.SearchResult) {
	wsp := externalOpenLoopSplitWSP(e)
	return olpitch.SearchWithRanges(&wsp), olpitch.SearchWithRangesNormalized(&wsp)
}

func encodeBitstreamFramesCoreOpenLoopPairwise(t *testing.T, samples []int16) ([]bitstream.Frame, int) {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	changedTop := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		prod, _ := externalOpenLoopProductionResults(enc)
		pairwiseTop := externalOpenLoopPairwiseTop(prod)
		_ = enc.openloopStep()
		if enc.tOp != pairwiseTop {
			changedTop++
			enc.tOp = pairwiseTop
		}
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, changedTop
}

func encodeBitstreamFramesCoreOpenLoopLift(t *testing.T, samples []int16, lift float64) ([]bitstream.Frame, int) {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	changedTop := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		prod, _ := externalOpenLoopProductionResults(enc)
		customTop := externalOpenLoopGlobalTopWithLift(prod, lift)
		_ = enc.openloopStep()
		if enc.tOp != customTop {
			changedTop++
			enc.tOp = customTop
		}
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, changedTop
}

func encodeBitstreamFramesCoreOpenLoopASource(t *testing.T, samples []int16, useUnquant bool, normalized bool) ([]bitstream.Frame, int) {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	changedTop := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		prod, _ := externalOpenLoopProductionResults(enc)
		if useUnquant {
			s := (*[FrameSamples]int16)(enc.oldSpeech[120:200])
			var result olpitch.SearchResult
			if normalized {
				result = olpitch.StepSplitSearchNormalizedRanges(&enc.aQ12Latest, &enc.aQ12Latest, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
			} else {
				result = olpitch.StepSplitSearch(&enc.aQ12Latest, &enc.aQ12Latest, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
			}
			enc.tOp = result.Top
			if enc.tOp != prod.Top {
				changedTop++
			}
		} else {
			_ = enc.openloopStep()
		}
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, changedTop
}

func encodeBitstreamFramesCoreSpeechWindows(t *testing.T, samples []int16, openBase, closeBase int, normalizedOpen bool) ([]bitstream.Frame, int) {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	changedTop := 0
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		prod, _ := externalOpenLoopProductionResults(enc)
		if openBase < 0 {
			if normalizedOpen {
				s := (*[FrameSamples]int16)(enc.oldSpeech[120:200])
				result := olpitch.StepSplitSearchNormalizedRanges(&enc.aHatSF1, &enc.aHatSF2, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
				enc.tOp = result.Top
			} else {
				_ = enc.openloopStep()
			}
		} else {
			if openBase < 0 || openBase+FrameSamples > len(enc.oldSpeech) {
				t.Fatalf("invalid open-loop speech window base %d", openBase)
			}
			s := (*[FrameSamples]int16)(enc.oldSpeech[openBase : openBase+FrameSamples])
			var result olpitch.SearchResult
			if normalizedOpen {
				result = olpitch.StepSplitSearchNormalizedRanges(&enc.aHatSF1, &enc.aHatSF2, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
			} else {
				result = olpitch.StepSplitSearch(&enc.aHatSF1, &enc.aHatSF2, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
			}
			enc.tOp = result.Top
		}
		if enc.tOp != prod.Top {
			changedTop++
		}

		for sub := 0; sub < 2; sub++ {
			if closeBase < 0 {
				_, _ = enc.closedloopStep(sub)
			} else {
				closedloopStepCoreSpeechWindow(enc, sub, closeBase)
			}
		}
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames, changedTop
}

func encodeBitstreamFramesCoreClosedLoopXB32(t *testing.T, samples []int16) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepCoreXB32(enc, 0)
		closedloopStepCoreXB32(enc, 1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesCoreClosedLoopXBRound(t *testing.T, samples []int16) []bitstream.Frame {
	t.Helper()
	enc := NewEncoderWithProfile(EncoderProfileCore)
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepCoreXBRound(enc, 0)
		closedloopStepCoreXBRound(enc, 1)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func closedloopStepCoreXBRound(e *Encoder, sub int) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[clpitch.SubframeLen]int16)(e.oldSpeech[sStart : sStart+clpitch.SubframeLen])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	externalBackwardFilterRoundedQ0(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)

	intLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	intLag, frac := refineProductionPitchFraction(&xb, exc, sub, intLag, e.intT1)
	e.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, exc, intLag, frac)
}

func closedloopStepCoreXB32(e *Encoder, sub int) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[clpitch.SubframeLen]int16)(e.oldSpeech[sStart : sStart+clpitch.SubframeLen])

	var r, x, h [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var xb32 [clpitch.SubframeLen]int32
	externalBackwardFilterQ12(&x, &h, &xb32)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)

	intLag := externalSearchIntegerXB32(&xb32, exc, centre, sub)
	var frac int8
	if sub == 0 {
		intLag, frac = externalRefineFractionSubframe1XB32(&xb32, exc, intLag)
	} else {
		intLag, frac = externalRefineFractionSubframe2XB32(&xb32, exc, intLag, e.intT1)
	}
	e.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, exc, intLag, frac)
}

func externalBackwardFilterRoundedQ0(x, h *[clpitch.SubframeLen]int16, xb *[clpitch.SubframeLen]int16) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc int64
		for m := n; m < clpitch.SubframeLen; m++ {
			acc += int64(x[m]) * int64(h[m-n])
		}
		xb[n] = fixed.Saturate(externalRoundShift64(acc, 12))
	}
}

func externalBackwardFilterQ12(x, h *[clpitch.SubframeLen]int16, xb *[clpitch.SubframeLen]int32) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc int64
		for m := n; m < clpitch.SubframeLen; m++ {
			acc += int64(x[m]) * int64(h[m-n])
		}
		xb[n] = externalSaturateInt64ToInt32(acc)
	}
}

func externalRoundShift64(v int64, shift uint) int32 {
	if shift == 0 {
		return externalSaturateInt64ToInt32(v)
	}
	add := int64(1 << (shift - 1))
	if v >= 0 {
		return externalSaturateInt64ToInt32((v + add) >> shift)
	}
	return externalSaturateInt64ToInt32(-(((-v) + add) >> shift))
}

func externalSearchIntegerXB32(xb *[clpitch.SubframeLen]int32, exc []int16, centre int16, sub int) int16 {
	var kMin, kMax int
	if sub == 0 {
		tMin, tMax := clpitch.Subframe1Window(centre)
		kMin, kMax = int(tMin), int(tMax)
	} else {
		tMin, tMax := clpitch.Subframe2Window(centre)
		kMin, kMax = int(tMin), int(tMax)
	}
	intLag := int16(kMin)
	best := int64(-1 << 63)
	base := len(exc) - clpitch.SubframeLen
	for k := kMin; k <= kMax; k++ {
		excBase := base - k
		var acc int64
		for n := 0; n < clpitch.SubframeLen; n++ {
			acc += 2 * int64(xb[n]) * int64(exc[excBase+n])
		}
		if acc > best {
			best = acc
			intLag = int16(k)
		}
	}
	return intLag
}

func externalRefineFractionSubframe1XB32(xb *[clpitch.SubframeLen]int32, exc []int16, intLag int16) (int16, int8) {
	if intLag >= 85 {
		return intLag, 0
	}

	bestLag := intLag
	bestFrac := int8(-1)
	bestSet := false
	var best int64
	consider := func(lag int16, frac int8) {
		rn := externalCorrelateAtFracXB32(xb, exc, lag, frac)
		if !bestSet || rn > best {
			bestSet = true
			best = rn
			bestLag = lag
			bestFrac = frac
		}
	}

	if intLag == clpitch.PitchMinInt {
		consider(clpitch.PitchMinInt-1, +1)
	}
	consider(intLag, -1)
	consider(intLag, 0)
	consider(intLag, +1)
	if intLag == 84 {
		consider(85, -1)
	}
	return bestLag, bestFrac
}

func externalRefineFractionSubframe2XB32(xb *[clpitch.SubframeLen]int32, exc []int16, intLag int16, intT1 int16) (int16, int8) {
	tMin, tMax := clpitch.Subframe2Window(intT1)
	bestLag := intLag
	bestFrac := int8(-1)
	bestSet := false
	var best int64
	consider := func(lag int16, frac int8) {
		rn := externalCorrelateAtFracXB32(xb, exc, lag, frac)
		if !bestSet || rn > best {
			bestSet = true
			best = rn
			bestLag = lag
			bestFrac = frac
		}
	}

	if intLag == tMin {
		consider(tMin-1, +1)
	}
	consider(intLag, -1)
	consider(intLag, 0)
	consider(intLag, +1)
	if intLag == tMax {
		consider(tMax+1, -1)
	}
	return bestLag, bestFrac
}

func externalCorrelateAtFracXB32(xb *[clpitch.SubframeLen]int32, exc []int16, intLag int16, frac int8) int64 {
	var acc int64
	for n := 0; n < clpitch.SubframeLen; n++ {
		s := clpitch.Interpolate3(exc, intLag-int16(n), frac)
		acc += 2 * int64(xb[n]) * int64(s)
	}
	return acc
}

func externalSaturateInt64ToInt32(v int64) int32 {
	if v > 0x7fffffff {
		return 0x7fffffff
	}
	if v < -0x80000000 {
		return -0x80000000
	}
	return int32(v)
}

func closedloopStepCoreSpeechWindow(e *Encoder, sub int, base int) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := base + 40*sub
	sFrame := (*[clpitch.SubframeLen]int16)(e.oldSpeech[sStart : sStart+clpitch.SubframeLen])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	intLag, frac := refineProductionPitchFraction(&xb, exc, sub, intLag, e.intT1)
	e.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, exc, intLag, frac)
}

func externalOpenLoopPairwiseTop(result olpitch.SearchResult) int16 {
	best := result.Range1
	if externalOpenLoopShouldOverridePairwise(result.Range2, best) {
		best = result.Range2
	}
	if externalOpenLoopShouldOverridePairwise(result.Range3, best) {
		best = result.Range3
	}
	return best.Lag
}

func externalOpenLoopGlobalTopWithLift(result olpitch.SearchResult, lift float64) int16 {
	best := result.Range1
	if externalOpenLoopShouldOverrideWithLift(result.Range2, best, lift) {
		best = result.Range2
	}
	if externalOpenLoopShouldOverrideRange3WithLift(result.Range3, best, result.Range1, result.Range2, lift) {
		best = result.Range3
	}
	return best.Lag
}

func externalOpenLoopShouldOverridePairwise(h, op olpitch.RangeScore) bool {
	if externalOpenLoopIsNearSubmultiple(int(h.Lag), int(op.Lag)) {
		return externalOpenLoopRangeScore(h) > 2*externalOpenLoopRangeScore(op)
	}
	return externalOpenLoopRangeScore(h) > externalOpenLoopRangeScore(op)
}

func externalOpenLoopShouldOverrideWithLift(h, op olpitch.RangeScore, lift float64) bool {
	if externalOpenLoopIsNearSubmultiple(int(h.Lag), int(op.Lag)) {
		return externalOpenLoopRangeScore(h) > lift*externalOpenLoopRangeScore(op)
	}
	return externalOpenLoopRangeScore(h) > externalOpenLoopRangeScore(op)
}

func externalOpenLoopShouldOverrideRange3WithLift(r3, op, r1, r2 olpitch.RangeScore, lift float64) bool {
	if externalOpenLoopIsNearSubmultiple(int(r3.Lag), int(r1.Lag)) &&
		externalOpenLoopRangeScore(r3) <= lift*externalOpenLoopRangeScore(r1) {
		return false
	}
	if externalOpenLoopIsNearSubmultiple(int(r3.Lag), int(r2.Lag)) &&
		externalOpenLoopRangeScore(r3) <= lift*externalOpenLoopRangeScore(r2) {
		return false
	}
	return externalOpenLoopShouldOverrideWithLift(r3, op, lift)
}

func externalOpenLoopIsNearSubmultiple(higher, lower int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= 2 {
			return true
		}
		if k*lower > higher+2 {
			return false
		}
	}
	return false
}

func externalOpenLoopResultRel(result olpitch.SearchResult) (float64, float64, float64) {
	s1 := externalOpenLoopRangeScore(result.Range1)
	s2 := externalOpenLoopRangeScore(result.Range2)
	s3 := externalOpenLoopRangeScore(result.Range3)
	maxScore := math.Max(s1, math.Max(s2, s3))
	if maxScore <= 0 {
		return 0, 0, 0
	}
	return s1 / maxScore, s2 / maxScore, s3 / maxScore
}

func externalOpenLoopRangeScore(r olpitch.RangeScore) float64 {
	if r.E <= 0 || r.R <= 0 {
		return 0
	}
	return float64(r.R) * float64(r.R) / float64(r.E)
}

func externalDiagnoseOpenLoopSplitFrame(e *Encoder) openLoopFrameDiag {
	wsp := externalOpenLoopSplitWSP(e)
	return openLoopFrameDiag{
		range1: oraclePickBest(&wsp, 20, 39),
		range2: oraclePickBest(&wsp, 40, 79),
		range3: oraclePickBest(&wsp, 80, 143),
	}
}

func externalOpenLoopGammaWeightLP(a, out *[11]int16) {
	gammaPow := [11]int16{32767, 24576, 18432, 13824, 10368, 7776, 5832, 4374, 3281, 2460, 1845}
	out[0] = a[0]
	for i := 1; i <= 10; i++ {
		out[i] = fixed.Mult(a[i], gammaPow[i])
	}
}

func externalOpenLoopCombineWith07(aw, out *[11]int16) {
	const gamma07Q15 int16 = 22938
	out[0] = aw[0]
	for i := 1; i <= 10; i++ {
		out[i] = aw[i] - fixed.MultR(gamma07Q15, aw[i-1])
	}
}

func externalOpenLoopScore(r openLoopRangeDiag) float64 {
	if r.e <= 0 || r.r <= 0 {
		return 0
	}
	return float64(r.r) * float64(r.r) / float64(r.e)
}

func externalOpenLoopRangeRel(d openLoopFrameDiag) (float64, float64, float64) {
	s1 := externalOpenLoopScore(d.range1)
	s2 := externalOpenLoopScore(d.range2)
	s3 := externalOpenLoopScore(d.range3)
	maxScore := math.Max(s1, math.Max(s2, s3))
	if maxScore <= 0 {
		return 0, 0, 0
	}
	return s1 / maxScore, s2 / maxScore, s3 / maxScore
}

func collectExternalBCGExcitationCommitTraceSubframe(
	t *testing.T,
	localEnc, bcgStateEnc *Encoder,
	targets []int,
	lookbackFrames int,
	frameIndex int,
	sub int,
	bcgIntLag int16,
	bcgFrac int8,
	bcgPitchCode uint16,
	bcgC uint16,
	bcgS uint8,
	bcgGA uint8,
	bcgGB uint8,
) []externalExcitationCommitTraceRow {
	t.Helper()
	oldExcCorr := corrCoeff(localEnc.oldExc[len(localEnc.oldExc)-clpitch.PitchMaxInt:], bcgStateEnc.oldExc[len(bcgStateEnc.oldExc)-clpitch.PitchMaxInt:])

	localBefore := *localEnc
	_, _ = localEnc.closedloopStep(sub)
	localSnap := externalCommitSnapshotFromEncoderFields(t, &localBefore, localEnc, sub)

	bcgBefore := *bcgStateEnc
	bcgSnap := externalCommitSnapshotFromFields(t, &bcgBefore, sub, bcgIntLag, bcgFrac, bcgC, bcgS, bcgGA, bcgGB, nil, false)
	commitExternalBCGStateSubframe(t, bcgStateEnc, sub, bcgIntLag, bcgFrac, bcgPitchCode, bcgC, bcgS, bcgGA, bcgGB)

	var rows []externalExcitationCommitTraceRow
	for _, target := range targets {
		if frameIndex < target-lookbackFrames || frameIndex > target {
			continue
		}
		rows = append(rows, externalExcitationCommitTraceRow{
			targetFrame:   target,
			frame:         frameIndex,
			sub:           sub,
			localT:        localSnap.intLag,
			localFrac:     localSnap.frac,
			bcgT:          bcgSnap.intLag,
			bcgFrac:       bcgSnap.frac,
			localGA:       localSnap.ga,
			localGB:       localSnap.gb,
			bcgGA:         bcgSnap.ga,
			bcgGB:         bcgSnap.gb,
			localGpQ14:    localSnap.gpQ14,
			bcgGpQ14:      bcgSnap.gpQ14,
			localGcQ12:    localSnap.gcQ12,
			bcgGcQ12:      bcgSnap.gcQ12,
			oldExcCorr:    oldExcCorr,
			vCorr:         corrCoeff(localSnap.v[:], bcgSnap.v[:]),
			cCorr:         corrCoeff(localSnap.c[:], bcgSnap.c[:]),
			pitchTermCorr: corrCoeff(localSnap.pitchTerm[:], bcgSnap.pitchTerm[:]),
			codeTermCorr:  corrCoeff(localSnap.codeTerm[:], bcgSnap.codeTerm[:]),
			uCorr:         corrCoeff(localSnap.u[:], bcgSnap.u[:]),
			localURMS:     rmsAmp(localSnap.u[:]),
			bcgURMS:       rmsAmp(bcgSnap.u[:]),
		})
	}
	return rows
}

func externalCommitSnapshotFromEncoderFields(t *testing.T, before, after *Encoder, sub int) externalExcitationCommitSnapshot {
	t.Helper()
	wide := !before.qualityHeuristicsEnabled() || before.qualityWideGainPredictorEnabled()
	if sub == 0 {
		return externalCommitSnapshotFromFields(t, before, sub, after.intT1, after.frac1, after.c1, after.s1, after.ga1, after.gb1, &after.prevGpQ14, wide)
	}
	return externalCommitSnapshotFromFields(t, before, sub, after.intT2, after.frac2, after.c2, after.s2, after.ga2, after.gb2, &after.prevGpQ14, wide)
}

func externalCommitSnapshotFromFields(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	code uint16,
	signs uint8,
	gaBits uint8,
	gbBits uint8,
	gpOverride *int16,
	wide bool,
) externalExcitationCommitSnapshot {
	t.Helper()
	_, _, _, v, _, _ := forcedPitchSurface(e, sub, intLag, frac)
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: code, Signs: signs}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	gaPhys := tables.GainImap1[gaBits&7]
	gbPhys := tables.GainImap2[gbBits&15]
	gpQ14, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	if wide {
		gpQ14, gcMantQ14, gcExp = gainquant.ReconstructWide(&e.pastQuaEn, &c, gaPhys, gbPhys)
	}
	if gpOverride != nil {
		gpQ14 = *gpOverride
	}
	var zeroV, zeroC [clpitch.SubframeLen]int16
	var pitchTerm, codeTerm, u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpQ14, 0, 0, &v, &zeroC, &pitchTerm)
	synth.BuildExcitation(0, gcMantQ14, gcExp, &zeroV, &c, &codeTerm)
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)
	return externalExcitationCommitSnapshot{
		intLag:    intLag,
		frac:      frac,
		ga:        gaBits & 7,
		gb:        gbBits & 15,
		gpQ14:     gpQ14,
		gcQ12:     mantExpToQ12(gcMantQ14, gcExp),
		v:         v,
		c:         c,
		pitchTerm: pitchTerm,
		codeTerm:  codeTerm,
		u:         u,
	}
}

func externalBCGStateDivergenceForSubframe(
	t *testing.T,
	localEnc, bcgStateEnc *Encoder,
	frame, sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
) externalBCGStateDivergenceRow {
	t.Helper()
	localProbe := *localEnc
	bcgProbe := *bcgStateEnc
	localSurf := externalFCBSurfaceForState(t, &localProbe, sub, intLag, frac, pitchCode, refC, refS)
	bcgSurf := externalFCBSurfaceForState(t, &bcgProbe, sub, intLag, frac, pitchCode, refC, refS)
	return externalBCGStateDivergenceRow{
		frame:                     frame,
		sub:                       sub,
		localSwMemRMS:             rmsAmp(localEnc.swMemErr[:]),
		bcgSwMemRMS:               rmsAmp(bcgStateEnc.swMemErr[:]),
		oldExcCorr:                corrCoeff(localEnc.oldExc[len(localEnc.oldExc)-clpitch.PitchMaxInt:], bcgStateEnc.oldExc[len(bcgStateEnc.oldExc)-clpitch.PitchMaxInt:]),
		xCorr:                     corrCoeff(localSurf.x[:], bcgSurf.x[:]),
		xPrimeCorr:                corrCoeff(localSurf.xPrime[:], bcgSurf.xPrime[:]),
		hCorr:                     corrCoeff(localSurf.h[:], bcgSurf.h[:]),
		yCorr:                     corrCoeff(localSurf.y[:], bcgSurf.y[:]),
		dCorr:                     corrCoeffInt32(localSurf.d[:], bcgSurf.d[:]),
		bcgTopKLocal:              localSurf.bcgTopK,
		bcgTopKBCGState:           bcgSurf.bcgTopK,
		bcgStateToLocalScoreRatio: externalSafeRatio(bcgSurf.bcgScore, localSurf.bcgScore),
	}
}

type externalFCBSurfaceSnapshot struct {
	x      [clpitch.SubframeLen]int16
	y      [clpitch.SubframeLen]int16
	h      [clpitch.SubframeLen]int16
	xPrime [clpitch.SubframeLen]int16
	d      [clpitch.SubframeLen]int32

	bcgTopK  int
	bcgScore float64
}

func externalFCBSurfaceForState(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
) externalFCBSurfaceSnapshot {
	t.Helper()
	x, y, h, _, gp, _ := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)
	hSearch := productionFCBSearchImpulse(&h, intLag, e.prevGpQ14)
	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)
	bcgPos := unpackExternalFCBPositions(refC)
	bcgLocalS := fcbsearch.PackS(&bcgPos, &signs)
	if bcgLocalS != refS {
		// Keep this as a scalar surface diagnostic: sign mismatch is expected
		// on many divergent frames, but the position score is still useful.
	}
	var top [fcbsearch.SearchTopKMax][4]int8
	topN := fcbsearch.SearchTopK(&dAbs, &phi, &top, fcbsearch.SearchTopKMax)
	bcgTopK := 0
	for i := 0; i < topN; i++ {
		if fcbsearch.PackC(&top[i]) == refC {
			bcgTopK = i + 1
			break
		}
	}
	_, _, bcgScore := externalFCBCandidateScore(&dAbs, &phi, bcgPos)
	return externalFCBSurfaceSnapshot{
		x:        x,
		y:        y,
		h:        h,
		xPrime:   xPrime,
		d:        d,
		bcgTopK:  bcgTopK,
		bcgScore: bcgScore,
	}
}

func externalFCBMixedSurfaceModes() []externalFCBMixedSurfaceMode {
	return []externalFCBMixedSurfaceMode{
		{name: "local", x: "local", y: "local", h: "local"},
		{name: "bcg", x: "bcg", y: "bcg", h: "bcg"},
		{name: "xB", x: "bcg", y: "local", h: "local"},
		{name: "yB", x: "local", y: "bcg", h: "local"},
		{name: "hB", x: "local", y: "local", h: "bcg"},
		{name: "xyB", x: "bcg", y: "bcg", h: "local"},
		{name: "xhB", x: "bcg", y: "local", h: "bcg"},
		{name: "yhB", x: "local", y: "bcg", h: "bcg"},
	}
}

type externalFCBSurfaceComponents struct {
	x      [clpitch.SubframeLen]int16
	y      [clpitch.SubframeLen]int16
	h      [clpitch.SubframeLen]int16
	gp     int16
	prevGp int16
}

func collectExternalBCGFCBMixedSurface(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, selected map[int]bool) []externalFCBMixedSurfaceRow {
	t.Helper()
	return collectExternalBCGFCBMixedSurfaceWithProfile(t, samples, bcgFrames, selected, EncoderProfileQuality)
}

func collectExternalBCGFCBMixedSurfaceWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	selected map[int]bool,
	profile EncoderProfile,
) []externalFCBMixedSurfaceRow {
	t.Helper()
	localEnc := NewEncoderWithProfile(profile)
	bcgStateEnc := NewEncoderWithProfile(profile)
	var bcgLSPDec lsp.Decoder
	var rows []externalFCBMixedSurfaceRow
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := localEnc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("local lpcStep frame %d: %v", frameIndex, err)
		}
		_ = localEnc.openloopStep()

		lpcAnalysisPreludeForDiagnostic(t, bcgStateEnc, samples[off:off+FrameSamples])
		idx := lspIndicesFromFrame(ref)
		bcgStateEnc.l0 = uint16(idx.L0)
		bcgStateEnc.l1 = uint16(idx.L1)
		bcgStateEnc.l2 = uint16(idx.L2)
		bcgStateEnc.l3 = uint16(idx.L3)
		bcgStateEnc.aHatSF1, bcgStateEnc.aHatSF2 = bcgLSPDec.Decode(idx)
		lsp.CommitIndicesForDiagnostic(&bcgStateEnc.freqPrev, idx)
		_ = bcgStateEnc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		if selected[frameIndex] {
			rows = append(rows,
				externalBCGFCBMixedSurfaceForSubframe(t, localEnc, bcgStateEnc, frameIndex, 0, int16(t1), int8(frac1), ref.C1, uint8(ref.S1))...,
			)
			rows = append(rows,
				externalBCGFCBMixedSurfaceForSubframe(t, localEnc, bcgStateEnc, frameIndex, 1, int16(t2), int8(frac2), ref.C2, uint8(ref.S2))...,
			)
		}

		_, _ = localEnc.closedloopStep(0)
		commitExternalBCGStateSubframe(t, bcgStateEnc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1))
		_, _ = localEnc.closedloopStep(1)
		commitExternalBCGStateSubframe(t, bcgStateEnc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2))
	}
	return rows
}

func externalBCGFCBMixedSurfaceForSubframe(
	t *testing.T,
	localEnc, bcgStateEnc *Encoder,
	frame, sub int,
	intLag int16,
	frac int8,
	refC uint16,
	refS uint8,
) []externalFCBMixedSurfaceRow {
	t.Helper()
	localProbe := *localEnc
	bcgProbe := *bcgStateEnc
	local := externalFCBSurfaceComponentsForState(&localProbe, sub, intLag, frac)
	bcg := externalFCBSurfaceComponentsForState(&bcgProbe, sub, intLag, frac)

	modes := externalFCBMixedSurfaceModes()
	rows := make([]externalFCBMixedSurfaceRow, 0, len(modes))
	for _, mode := range modes {
		components := externalMixFCBSurfaceComponents(local, bcg, mode)
		row := externalScoreMixedFCBSurface(frame, sub, mode.name, components, intLag, refC, refS)
		rows = append(rows, row)
	}
	return rows
}

func externalFCBSurfaceComponentsForState(e *Encoder, sub int, intLag int16, frac int8) externalFCBSurfaceComponents {
	x, y, h, _, gp, _ := forcedPitchSurface(e, sub, intLag, frac)
	return externalFCBSurfaceComponents{
		x:      x,
		y:      y,
		h:      h,
		gp:     gp,
		prevGp: e.prevGpQ14,
	}
}

func externalMixFCBSurfaceComponents(local, bcg externalFCBSurfaceComponents, mode externalFCBMixedSurfaceMode) externalFCBSurfaceComponents {
	out := local
	if mode.x == "bcg" {
		out.x = bcg.x
	}
	if mode.y == "bcg" {
		out.y = bcg.y
		out.gp = bcg.gp
	}
	if mode.h == "bcg" {
		out.h = bcg.h
		out.prevGp = bcg.prevGp
	}
	return out
}

func externalScoreMixedFCBSurface(
	frame, sub int,
	mode string,
	components externalFCBSurfaceComponents,
	intLag int16,
	refC uint16,
	refS uint8,
) externalFCBMixedSurfaceRow {
	hSearch := productionFCBSearchImpulse(&components.h, intLag, components.prevGp)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&components.x, &components.y, components.gp, &xPrime)
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	bcgPos := unpackExternalFCBPositions(refC)
	bcgRank, bestScore := externalFCBExactRank(&dAbs, &phi, bcgPos)
	_, _, bcgScore := externalFCBCandidateScore(&dAbs, &phi, bcgPos)
	bcgLocalS := fcbsearch.PackS(&bcgPos, &signs)
	pulseSignMatches, signedDSumRatio := externalBCGPulseSignAgreement(&d, &dAbs, &bcgPos, refS)

	return externalFCBMixedSurfaceRow{
		frame:            frame,
		sub:              sub,
		mode:             mode,
		rank:             bcgRank,
		scoreRatio:       externalSafeRatio(bcgScore, bestScore),
		signMatch:        bcgLocalS == refS,
		pulseSignMatches: pulseSignMatches,
		signedDSumRatio:  signedDSumRatio,
		xRMS:             rmsAmp(components.x[:]),
		yRMS:             rmsAmp(components.y[:]),
		hRMS:             rmsAmp(components.h[:]),
		xPrimeRMS:        rmsAmp(xPrime[:]),
		dMaxAbs:          maxAbsInt32Array(d),
	}
}

func externalBCGPulseSignAgreement(d *[clpitch.SubframeLen]int32, dAbs *[clpitch.SubframeLen]int32, pos *[4]int8, refS uint8) (matches int, signedDSumRatio float64) {
	var signedSum int64
	var absSum int64
	for i := 0; i < 4; i++ {
		p := pos[i]
		refSign := int32(-1)
		if (refS>>uint(i))&1 == 1 {
			refSign = 1
		}
		localSign := int32(-1)
		if d[p] >= 0 {
			localSign = 1
		}
		if localSign == refSign {
			matches++
		}
		signedSum += int64(refSign) * int64(d[p])
		absSum += int64(dAbs[p])
	}
	if absSum == 0 {
		return matches, 0
	}
	return matches, float64(signedSum) / float64(absSum)
}

func commitExternalBCGStateSubframe(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
) {
	t.Helper()
	x, y, h, v, _, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)
	commitExternalBCGCodeGainState(e, sub, refC, refS, refGA, refGB, intLag, &x, &y, &h, &v, sFrame)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func corrCoeffInt32(a, b []int32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var aa, bb, ab float64
	for i := 0; i < n; i++ {
		x := float64(a[i])
		y := float64(b[i])
		aa += x * x
		bb += y * y
		ab += x * y
	}
	if aa <= 0 || bb <= 0 {
		return 0
	}
	return ab / math.Sqrt(aa*bb)
}

func collectExternalBCGFCBSurfaceVariant(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, mode fcbSurfaceVariant, minFrameRMS float64) externalFCBRankStats {
	t.Helper()
	return collectExternalBCGFCBSurfaceVariantWithProfile(t, samples, bcgFrames, EncoderProfileQuality, mode, minFrameRMS)
}

func collectExternalBCGFCBSurfaceVariantWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	profile EncoderProfile,
	mode fcbSurfaceVariant,
	minFrameRMS float64,
) externalFCBRankStats {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	var stats externalFCBRankStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		record := minFrameRMS <= 0 || rmsAmp(samples[off:off+FrameSamples]) >= minFrameRMS
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		observeExternalBCGFCBSurfaceVariant(t, enc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), frameIndex, record, mode, &stats)
		observeExternalBCGFCBSurfaceVariant(t, enc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), frameIndex, record, mode, &stats)
	}
	return stats
}

func observeExternalBCGFCBSurfaceVariant(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	frameIndex int,
	record bool,
	mode fcbSurfaceVariant,
	stats *externalFCBRankStats,
) {
	x, y, h, v, gp, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)

	hSearch := productionFCBSearchImpulse(&h, intLag, e.prevGpQ14)
	if mode.hMode == "plain" {
		hSearch = h
	}

	gpForTarget := gp
	switch mode.target {
	case "", "trunc":
	case "none":
		gpForTarget = 0
	case "half":
		gpForTarget = scaleInt16RatioForDiagnostic(gp, 1, 2)
	case "double":
		gpForTarget = scaleInt16RatioForDiagnostic(gp, 2, 1)
	case "refgp":
		gpForTarget = externalGainGpQ14(refGA, refGB)
	case "prevgp":
		gpForTarget = e.prevGpQ14
	default:
		t.Fatalf("unknown FCB surface target mode %q", mode.target)
	}
	var xPrime [clpitch.SubframeLen]int16
	if mode.target == "trunc" {
		adjustedTargetTruncForDiag(&x, &y, gpForTarget, &xPrime)
	} else {
		fcbsearch.AdjustedTarget(&x, &y, gpForTarget, &xPrime)
	}

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	if mode.dMode == "signed" {
		for i := 0; i < len(d); i++ {
			signs[i] = 1
			dAbs[i] = d[i]
			if dAbs[i] < 0 {
				dAbs[i] = 0
			}
		}
	} else {
		fcbsearch.SignsFromD(&d, &signs, &dAbs)
	}

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	if mode.phiMode == "unsigned" {
		var unsignedSigns [clpitch.SubframeLen]int16
		for i := 0; i < len(unsignedSigns); i++ {
			unsignedSigns[i] = 1
		}
		fcbsearch.PhiPrime(&hSearch, &unsignedSigns, &phi)
	} else {
		fcbsearch.PhiPrime(&hSearch, &signs, &phi)
		adjustPhiVariant(&phi, mode.phiMode)
	}

	var localPos [4]int8
	var sumOut [2]int64
	searchExternalFCBForEncoder(e, &dAbs, &phi, &localPos, &sumOut)
	localC := fcbsearch.PackC(&localPos)
	localS := fcbsearch.PackS(&localPos, &signs)

	var top [fcbsearch.SearchTopKMax][4]int8
	topN := fcbsearch.SearchTopK(&dAbs, &phi, &top, fcbsearch.SearchTopKMax)
	bcgTopK := 0
	for i := 0; i < topN; i++ {
		if fcbsearch.PackC(&top[i]) == refC {
			bcgTopK = i + 1
			break
		}
	}

	if record {
		stats.count++
		if localC == refC {
			stats.sameC++
		}
		if localS == refS {
			stats.sameS++
		}
		if localC == refC && localS == refS {
			stats.sameBoth++
		}
		if bcgTopK == 1 {
			stats.bcgTop1++
		}
		if bcgTopK > 0 && bcgTopK <= 4 {
			stats.bcgTop4++
		}
		if bcgTopK > 0 && bcgTopK <= 8 {
			stats.bcgTop8++
		}
		if bcgTopK > 0 && bcgTopK <= 32 {
			stats.bcgTop32++
		}
	}

	e.fcbStep(sub, nil, nil, &x, &y, &h, &v, gp)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func externalGainGpQ14(gaBits, gbBits uint8) int16 {
	ga := tables.GainImap1[gaBits&7]
	gb := tables.GainImap2[gbBits&15]
	return saturateInt32ToInt16(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0]))
}

func externalGainGammaQ13(gaBits, gbBits uint8) int16 {
	ga := tables.GainImap1[gaBits&7]
	gb := tables.GainImap2[gbBits&15]
	return saturateInt32ToInt16(int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1]))
}

func scaleInt16RatioForDiagnostic(v int16, num, den int32) int16 {
	return saturateInt32ToInt16(scaleInt32RatioForGainSearch(int32(v), num, den))
}

func adjustPhiVariant(phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32, mode string) {
	switch mode {
	case "":
		return
	case "diagFull":
		for i := 0; i < clpitch.SubframeLen; i++ {
			phi[i][i] = scaleInt32RatioForGainSearch(phi[i][i], 2, 1)
		}
	case "crossHalf":
		for i := 0; i < clpitch.SubframeLen; i++ {
			for j := 0; j < clpitch.SubframeLen; j++ {
				if i != j {
					phi[i][j] = scaleInt32RatioForGainSearch(phi[i][j], 1, 2)
				}
			}
		}
	case "crossDouble":
		for i := 0; i < clpitch.SubframeLen; i++ {
			for j := 0; j < clpitch.SubframeLen; j++ {
				if i != j {
					phi[i][j] = scaleInt32RatioForGainSearch(phi[i][j], 2, 1)
				}
			}
		}
	default:
		panic("unknown phi variant")
	}
}

func collectExternalBCGFCBRank(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, forceLSP bool, commit string, minFrameRMS float64) externalFCBRankStats {
	t.Helper()
	return collectExternalBCGFCBRankWithProfile(t, samples, bcgFrames, EncoderProfileQuality, forceLSP, commit, minFrameRMS)
}

func collectExternalBCGFCBRankWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	profile EncoderProfile,
	forceLSP bool,
	commit string,
	minFrameRMS float64,
) externalFCBRankStats {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	var forcedDec lsp.Decoder
	var stats externalFCBRankStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if forceLSP {
			lpcAnalysisPreludeForDiagnostic(t, enc, samples[off:off+FrameSamples])
			idx := lspIndicesFromFrame(ref)
			enc.l0 = uint16(idx.L0)
			enc.l1 = uint16(idx.L1)
			enc.l2 = uint16(idx.L2)
			enc.l3 = uint16(idx.L3)
			enc.aHatSF1, enc.aHatSF2 = forcedDec.Decode(idx)
			lsp.CommitIndicesForDiagnostic(&enc.freqPrev, idx)
		} else if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		record := minFrameRMS <= 0 || rmsAmp(samples[off:off+FrameSamples]) >= minFrameRMS
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		observeExternalBCGFCBRank(t, enc, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), frameIndex, record, commit, &stats)
		observeExternalBCGFCBRank(t, enc, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), frameIndex, record, commit, &stats)
	}
	return stats
}

func observeExternalBCGFCBRank(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	frameIndex int,
	record bool,
	commit string,
	stats *externalFCBRankStats,
) {
	x, y, h, v, gp, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)
	hSearch := productionFCBSearchImpulse(&h, intLag, e.prevGpQ14)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var localPos [4]int8
	var sumOut [2]int64
	searchExternalFCBForEncoder(e, &dAbs, &phi, &localPos, &sumOut)
	localC := fcbsearch.PackC(&localPos)
	localS := fcbsearch.PackS(&localPos, &signs)
	bcgPos := unpackExternalFCBPositions(refC)
	bcgLocalS := fcbsearch.PackS(&bcgPos, &signs)

	var top [fcbsearch.SearchTopKMax][4]int8
	topN := fcbsearch.SearchTopK(&dAbs, &phi, &top, fcbsearch.SearchTopKMax)
	bcgTopK := 0
	for i := 0; i < topN; i++ {
		if fcbsearch.PackC(&top[i]) == refC {
			bcgTopK = i + 1
			break
		}
	}

	if record {
		stats.count++
		if localC == refC {
			stats.sameC++
		}
		if localS == refS {
			stats.sameS++
		}
		if localC == refC && localS == refS {
			stats.sameBoth++
		}
		if bcgLocalS == refS {
			stats.bcgSignMatchesLocal++
		}
		if bcgTopK == 1 {
			stats.bcgTop1++
		}
		if bcgTopK > 0 && bcgTopK <= 4 {
			stats.bcgTop4++
		}
		if bcgTopK > 0 && bcgTopK <= 8 {
			stats.bcgTop8++
		}
		if bcgTopK > 0 && bcgTopK <= 32 {
			stats.bcgTop32++
		}
		if len(stats.examples) < 6 && (localC != refC || localS != refS) {
			stats.examples = append(stats.examples, externalFCBRankExample{
				frame:     frameIndex,
				sub:       sub,
				localC:    localC,
				localS:    localS,
				bcgC:      refC,
				bcgS:      refS,
				bcgTopK:   bcgTopK,
				bcgLocalS: bcgLocalS,
			})
		}
	}

	switch commit {
	case "local":
		e.fcbStep(sub, nil, nil, &x, &y, &h, &v, gp)
	case "bcg":
		commitExternalBCGCodeGainState(e, sub, refC, refS, refGA, refGB, intLag, &x, &y, &h, &v, sFrame)
	default:
		t.Fatalf("unknown FCB rank commit mode %q", commit)
	}
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func commitExternalBCGCodeGainState(
	e *Encoder,
	sub int,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	intLag int16,
	x, y, h, v *[clpitch.SubframeLen]int16,
	sFrame *[40]int16,
) {
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: refS}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, h, &z)

	refGAPhys := tables.GainImap1[refGA&7]
	refGBPhys := tables.GainImap2[refGB&15]
	gpQ14, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, refGAPhys, refGBPhys)
	gammaQ13 := gainGammaQ13(refGAPhys, refGBPhys)

	if sub == 0 {
		e.c1 = refC
		e.s1 = refS
		e.ga1 = refGA & 7
		e.gb1 = refGB & 15
	} else {
		e.c2 = refC
		e.s2 = refS
		e.ga2 = refGA & 7
		e.gb2 = refGB & 15
	}
	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(gpQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}
	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaQ13)
	e.prevGpQ14 = gpQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func unpackExternalFCBPositions(code uint16) [4]int8 {
	i0 := int8(code & 0x7)
	i1 := int8((code >> 3) & 0x7)
	i2 := int8((code >> 6) & 0x7)
	jx := int8((code >> 9) & 0x1)
	i3 := int8((code >> 10) & 0x7)
	return [4]int8{
		i0 * 5,
		i1*5 + 1,
		i2*5 + 2,
		i3*5 + 3 + jx,
	}
}

func collectExternalBCGGainRank(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	commit string,
	xNum, xDen, yNum, yDen, gpcNum, gpcDen int32,
) externalGainRankStats {
	t.Helper()
	enc := NewEncoder()
	var stats externalGainRankStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		observeExternalBCGGainRank(t, enc, frameIndex, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), commit, xNum, xDen, yNum, yDen, gpcNum, gpcDen, &stats)
		observeExternalBCGGainRank(t, enc, frameIndex, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), commit, xNum, xDen, yNum, yDen, gpcNum, gpcDen, &stats)
	}
	return stats
}

func collectExternalBCGGainRankWithProfile(
	t *testing.T,
	samples []int16,
	bcgFrames []bitstream.Frame,
	profile EncoderProfile,
	commit string,
) externalGainRankStats {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	var stats externalGainRankStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		observeExternalBCGGainRankForEncoder(t, enc, frameIndex, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), commit, &stats)
		observeExternalBCGGainRankForEncoder(t, enc, frameIndex, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), commit, &stats)
	}
	return stats
}

func observeExternalBCGGainRankForEncoder(
	t *testing.T,
	e *Encoder,
	frameIndex, sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	commit string,
	stats *externalGainRankStats,
) {
	t.Helper()
	x, y, h, v, _, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: refS}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	useWideGainPredictor := !e.qualityHeuristicsEnabled() || e.qualityWideGainPredictorEnabled()
	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	if useWideGainPredictor {
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&e.pastQuaEn, &c)
	}
	xSearch := x
	ySearch := y
	gpcSearchQ12 := gpcPredQ12
	if e.qualityGainSearchBiasEnabled() {
		scaleGainSearchVector(&xSearch, qualityGainSearchTargetScaleNum, qualityGainSearchTargetScaleDen)
		scaleGainSearchVector(&ySearch, qualityGainSearchAdaptiveContributionScaleNum, qualityGainSearchAdaptiveContributionScaleDen)
		gpcSearchQ12 = scaleInt32RatioForGainSearch(
			gpcPredQ12,
			qualityGainSearchFixedContributionScaleNum,
			qualityGainSearchFixedContributionScaleDen,
		)
	}

	ownGAPhys, ownGBPhys, ownGpTxQ14, ownGammaQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	ownBitsGA, ownBitsGB := gainquant.PackGains(ownGAPhys, ownGBPhys)
	bcgGAPhys := tables.GainImap1[refGA&7]
	bcgGBPhys := tables.GainImap2[refGB&15]
	ownGpCommitQ14 := gainquant.Tame(ownGpTxQ14, &e.oldExc)
	var ownGcMantQ14 int16
	var ownGcExp int8
	if useWideGainPredictor {
		_, ownGcMantQ14, ownGcExp = gainquant.ReconstructWide(&e.pastQuaEn, &c, ownGAPhys, ownGBPhys)
	} else {
		_, ownGcMantQ14, ownGcExp = gainquant.Reconstruct(&e.pastQuaEn, &c, ownGAPhys, ownGBPhys)
	}
	bcgGpQ14, bcgGcMantQ14, bcgGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, bcgGAPhys, bcgGBPhys)
	bcgGammaQ13 := gainGammaQ13(bcgGAPhys, bcgGBPhys)

	rank := rankExternalGainCandidateWithPredictor(&e.pastQuaEn, &c, &x, &y, &z, &xSearch, &ySearch, gpcSearchQ12, ownGAPhys, ownGBPhys, bcgGAPhys, bcgGBPhys, useWideGainPredictor)
	recordExternalGainRankStats(stats, frameIndex, sub, ownBitsGA, ownBitsGB, refGA&7, refGB&15, rank)

	var gpCommitQ14, gcMantQ14 int16
	var gcExp int8
	var gammaQ13 int16
	switch commit {
	case "own":
		gpCommitQ14, gcMantQ14, gcExp = ownGpCommitQ14, ownGcMantQ14, ownGcExp
		gammaQ13 = ownGammaQ13
	case "bcg":
		gpCommitQ14, gcMantQ14, gcExp = bcgGpQ14, bcgGcMantQ14, bcgGcExp
		gammaQ13 = bcgGammaQ13
	default:
		t.Fatalf("unknown gain-rank commit mode %q", commit)
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(gpCommitQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}
	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpCommitQ14, gcMantQ14, gcExp, &v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaQ13)
	e.prevGpQ14 = gpCommitQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func observeExternalBCGGainRank(
	t *testing.T,
	e *Encoder,
	frameIndex, sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	commit string,
	xNum, xDen, yNum, yDen, gpcNum, gpcDen int32,
	stats *externalGainRankStats,
) {
	t.Helper()
	x, y, h, v, _, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: refS}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	xSearch := x
	ySearch := y
	scaleGainSearchVector(&xSearch, xNum, xDen)
	scaleGainSearchVector(&ySearch, yNum, yDen)
	gpcSearchQ12 := scaleInt32RatioForGainSearch(gpcPredQ12, gpcNum, gpcDen)

	ownGAPhys, ownGBPhys, ownGpTxQ14, ownGammaQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	ownBitsGA, ownBitsGB := gainquant.PackGains(ownGAPhys, ownGBPhys)
	bcgGAPhys := tables.GainImap1[refGA&7]
	bcgGBPhys := tables.GainImap2[refGB&15]
	ownGpCommitQ14 := gainquant.Tame(ownGpTxQ14, &e.oldExc)
	_, ownGcMantQ14, ownGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, ownGAPhys, ownGBPhys)
	bcgGpQ14, bcgGcMantQ14, bcgGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, bcgGAPhys, bcgGBPhys)
	bcgGammaQ13 := gainGammaQ13(bcgGAPhys, bcgGBPhys)

	rank := rankExternalGainCandidate(&e.pastQuaEn, &c, &x, &y, &z, &xSearch, &ySearch, gpcSearchQ12, ownGAPhys, ownGBPhys, bcgGAPhys, bcgGBPhys)
	recordExternalGainRankStats(stats, frameIndex, sub, ownBitsGA, ownBitsGB, refGA&7, refGB&15, rank)

	var gpCommitQ14, gcMantQ14 int16
	var gcExp int8
	var gammaQ13 int16
	switch commit {
	case "own":
		gpCommitQ14, gcMantQ14, gcExp = ownGpCommitQ14, ownGcMantQ14, ownGcExp
		gammaQ13 = ownGammaQ13
	case "bcg":
		gpCommitQ14, gcMantQ14, gcExp = bcgGpQ14, bcgGcMantQ14, bcgGcExp
		gammaQ13 = bcgGammaQ13
	default:
		t.Fatalf("unknown gain-rank commit mode %q", commit)
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(gpCommitQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}
	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpCommitQ14, gcMantQ14, gcExp, &v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaQ13)
	e.prevGpQ14 = gpCommitQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func recordExternalGainRankStats(
	stats *externalGainRankStats,
	frameIndex, sub int,
	ownBitsGA, ownBitsGB, refGA, refGB uint8,
	rank externalGainRankResult,
) {
	stats.count++
	if ownBitsGA == refGA && ownBitsGB == refGB {
		stats.sameBoth++
	}
	if rank.bcgInPreselect {
		stats.bcgInPreselect++
	}
	stats.bcgSearchRankSum += int64(rank.bcgSearchRank)
	stats.bcgNativeRankSum += int64(rank.bcgNativeRank)
	if rank.bcgSearchRank <= 1 {
		stats.bcgSearchRankLE1++
	}
	if rank.bcgSearchRank <= 4 {
		stats.bcgSearchRankLE4++
	}
	if rank.bcgSearchRank <= 8 {
		stats.bcgSearchRankLE8++
	}
	if rank.bcgSearchRank <= 32 {
		stats.bcgSearchRankLE32++
	}
	if rank.bcgNativeRank <= 1 {
		stats.bcgNativeRankLE1++
	}
	if rank.bcgNativeRank <= 4 {
		stats.bcgNativeRankLE4++
	}
	if rank.bcgNativeRank <= 8 {
		stats.bcgNativeRankLE8++
	}
	if rank.bcgNativeRank <= 32 {
		stats.bcgNativeRankLE32++
	}
	if rank.bcgSearchCost < rank.ownSearchCost {
		stats.bcgSearchLowerOwn++
	}
	if rank.bcgNativeCost < rank.ownNativeCost {
		stats.bcgNativeLowerOwn++
	}
	if len(stats.examples) < 6 &&
		(ownBitsGA != refGA || ownBitsGB != refGB) &&
		(rank.bcgSearchRank <= 8 || !rank.bcgInPreselect || rank.bcgNativeCost < rank.ownNativeCost) {
		stats.examples = append(stats.examples, externalGainRankExample{
			frame:          frameIndex,
			sub:            sub,
			ownGA:          ownBitsGA,
			ownGB:          ownBitsGB,
			bcgGA:          refGA,
			bcgGB:          refGB,
			searchRank:     rank.bcgSearchRank,
			nativeRank:     rank.bcgNativeRank,
			bcgInPreselect: rank.bcgInPreselect,
			ownSearchCost:  rank.ownSearchCost,
			bcgSearchCost:  rank.bcgSearchCost,
			ownNativeCost:  rank.ownNativeCost,
			bcgNativeCost:  rank.bcgNativeCost,
		})
	}
}

type externalGainRankResult struct {
	bcgSearchRank  int
	bcgNativeRank  int
	bcgInPreselect bool

	ownSearchCost int64
	bcgSearchCost int64
	ownNativeCost int64
	bcgNativeCost int64
}

func rankExternalGainCandidate(
	past *[4]int16,
	c *[40]int16,
	x, y, z, xSearch, ySearch *[40]int16,
	gpcSearchQ12 int32,
	ownGA, ownGB, bcgGA, bcgGB uint8,
) externalGainRankResult {
	return rankExternalGainCandidateWithPredictor(past, c, x, y, z, xSearch, ySearch, gpcSearchQ12, ownGA, ownGB, bcgGA, bcgGB, false)
}

func rankExternalGainCandidateWithPredictor(
	past *[4]int16,
	c *[40]int16,
	x, y, z, xSearch, ySearch *[40]int16,
	gpcSearchQ12 int32,
	ownGA, ownGB, bcgGA, bcgGB uint8,
	wide bool,
) externalGainRankResult {
	ctx := gainSearchCostContext(xSearch, ySearch, z)
	out := externalGainRankResult{
		bcgSearchRank:  1,
		bcgNativeRank:  1,
		bcgInPreselect: gainSearchPreselectContains(&ctx, gpcSearchQ12, bcgGA, bcgGB),
		ownSearchCost:  gainSearchCost(&ctx, ownGA, ownGB, gpcSearchQ12),
		bcgSearchCost:  gainSearchCost(&ctx, bcgGA, bcgGB, gpcSearchQ12),
	}
	ownGp, ownGcMant, ownGcExp := reconstructExternalGainCandidate(past, c, ownGA, ownGB, wide)
	bcgGp, bcgGcMant, bcgGcExp := reconstructExternalGainCandidate(past, c, bcgGA, bcgGB, wide)
	out.ownNativeCost = gainResidualEnergyQ0(x, y, z, ownGp, ownGcMant, ownGcExp)
	out.bcgNativeCost = gainResidualEnergyQ0(x, y, z, bcgGp, bcgGcMant, bcgGcExp)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			searchCost := gainSearchCost(&ctx, gai, gbi, gpcSearchQ12)
			if searchCost < out.bcgSearchCost {
				out.bcgSearchRank++
			}
			gp, gcMant, gcExp := reconstructExternalGainCandidate(past, c, gai, gbi, wide)
			nativeCost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if nativeCost < out.bcgNativeCost {
				out.bcgNativeRank++
			}
		}
	}
	return out
}

func reconstructExternalGainCandidate(past *[4]int16, c *[40]int16, ga, gb uint8, wide bool) (int16, int16, int8) {
	if wide {
		return gainquant.ReconstructWide(past, c, ga, gb)
	}
	return gainquant.Reconstruct(past, c, ga, gb)
}

type gainSearchCostCtx struct {
	A int64
	B int64
	C int64
	D int64
	F int64

	rawA int64
	rawB int64
	rawC int64
	rawD int64
	rawF int64

	gpOptQ14 int64
	gcOptQ12 int64
}

func gainSearchCostContext(x, y, z *[40]int16) gainSearchCostCtx {
	return gainSearchCostContextTargetBits(x, y, z, 14)
}

func gainSearchCostContextTargetBits(x, y, z *[40]int16, targetBits uint) gainSearchCostCtx {
	if targetBits == 0 {
		targetBits = 1
	}
	var ctx gainSearchCostCtx
	for i := 0; i < 40; i++ {
		xi := int64(x[i])
		yi := int64(y[i])
		zi := int64(z[i])
		ctx.A += (yi * yi) << 24
		ctx.B += zi * zi
		ctx.C += (yi * zi) << 12
		ctx.D += (xi * yi) << 24
		ctx.F += (xi * zi) << 12
	}
	ctx.rawA, ctx.rawB, ctx.rawC, ctx.rawD, ctx.rawF = ctx.A, ctx.B, ctx.C, ctx.D, ctx.F
	maxAbs := absInt64Diagnostic(ctx.A)
	for _, v := range [...]int64{ctx.B, ctx.C, ctx.D, ctx.F} {
		if a := absInt64Diagnostic(v); a > maxAbs {
			maxAbs = a
		}
	}
	var nshift uint
	if maxAbs > 0 {
		blen := uint(bits.Len64(uint64(maxAbs)))
		if blen > targetBits {
			nshift = blen - targetBits
		}
	}
	if nshift > 0 {
		ctx.A >>= nshift
		ctx.B >>= nshift
		ctx.C >>= nshift
		ctx.D >>= nshift
		ctx.F >>= nshift
	}

	det := ctx.A*ctx.B - ctx.C*ctx.C
	switch {
	case det > 0:
		ctx.gpOptQ14 = ((ctx.D*ctx.B - ctx.F*ctx.C) << 14) / det
		ctx.gcOptQ12 = ((ctx.F*ctx.A - ctx.D*ctx.C) << 12) / det
	case ctx.A > 0:
		ctx.gpOptQ14 = (ctx.D << 14) / ctx.A
	case ctx.B > 0:
		ctx.gcOptQ12 = (ctx.F << 12) / ctx.B
	}
	if ctx.gpOptQ14 < 0 {
		ctx.gpOptQ14 = 0
	}
	if ctx.gcOptQ12 < 0 {
		ctx.gcOptQ12 = 0
	}
	return ctx
}

func gainSearchCost(ctx *gainSearchCostCtx, ga, gb uint8, gpcSearchQ12 int32) int64 {
	shift := gainSearchCostShiftDiagnostic(ctx, gpcSearchQ12, func(uint8, uint8) bool { return true })
	return gainSearchCostWithShift(ctx, ga, gb, gpcSearchQ12, shift)
}

func gainSearchCostWithShift(ctx *gainSearchCostCtx, ga, gb uint8, gpcSearchQ12 int32, shift uint) int64 {
	gp := int64(tables.GainGBK1[ga][0]) + int64(tables.GainGBK2[gb][0])
	gam := int64(gainGammaQ13(ga, gb))
	gc := (gam * int64(gpcSearchQ12)) >> 13
	A := ctx.rawA >> shift
	B := ctx.rawB >> shift
	C := ctx.rawC >> shift
	D := ctx.rawD >> shift
	F := ctx.rawF >> shift
	cost := gp * gp * A
	cost += (gc * gc * B) << 4
	cost += (2 * gp * gc * C) << 2
	cost -= (2 * gp * D) << 14
	cost -= (2 * gc * F) << 16
	return cost
}

func gainSearchCostShiftDiagnostic(ctx *gainSearchCostCtx, gpcSearchQ12 int32, allow func(uint8, uint8) bool) uint {
	const targetBits = 58
	var maxGp, maxGc int64
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			if allow != nil && !allow(gai, gbi) {
				continue
			}
			gp := int64(tables.GainGBK1[gai][0]) + int64(tables.GainGBK2[gbi][0])
			if gp < 0 {
				gp = -gp
			}
			if gp > maxGp {
				maxGp = gp
			}
			gam := int64(fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][1]) + int32(tables.GainGBK2[gbi][1]))))
			gc := (gam * int64(gpcSearchQ12)) >> 13
			if gc < 0 {
				gc = -gc
			}
			if gc > maxGc {
				maxGc = gc
			}
		}
	}
	if maxGp == 0 {
		maxGp = 1
	}
	if maxGc == 0 {
		maxGc = 1
	}

	gpBits := bitLenAbsInt64Diagnostic(maxGp)
	gcBits := bitLenAbsInt64Diagnostic(maxGc)
	var shift uint
	shift = maxUintDiagnostic(shift, gainSearchTermShiftDiagnostic(ctx.rawA, gpBits+gpBits, 0, targetBits))
	shift = maxUintDiagnostic(shift, gainSearchTermShiftDiagnostic(ctx.rawB, gcBits+gcBits, 4, targetBits))
	shift = maxUintDiagnostic(shift, gainSearchTermShiftDiagnostic(ctx.rawC, gpBits+gcBits, 3, targetBits))
	shift = maxUintDiagnostic(shift, gainSearchTermShiftDiagnostic(ctx.rawD, gpBits, 15, targetBits))
	shift = maxUintDiagnostic(shift, gainSearchTermShiftDiagnostic(ctx.rawF, gcBits, 17, targetBits))
	return shift
}

func gainSearchTermShiftDiagnostic(corr int64, factorBits, extraShift, targetBits uint) uint {
	corrBits := bitLenAbsInt64Diagnostic(corr)
	if corrBits == 0 {
		return 0
	}
	totalBits := corrBits + factorBits + extraShift
	if totalBits <= targetBits {
		return 0
	}
	return totalBits - targetBits
}

func bitLenAbsInt64Diagnostic(v int64) uint {
	if v < 0 {
		v = -v
	}
	return uint(bits.Len64(uint64(v)))
}

func maxUintDiagnostic(a, b uint) uint {
	if b > a {
		return b
	}
	return a
}

func gainSearchPreselectContains(ctx *gainSearchCostCtx, gpcSearchQ12 int32, ga, gb uint8) bool {
	gaOK, gbOK := gainSearchPreselectAxisContainsWithGpOpt(ctx, gpcSearchQ12, ga, gb, ctx.gpOptQ14)
	return gaOK && gbOK
}

func gainSearchPreselectContainsGpClip(ctx *gainSearchCostCtx, gpcSearchQ12 int32, ga, gb uint8) bool {
	gpOpt := ctx.gpOptQ14
	if gpOpt > gainPreselectGpOptUpperQ14 {
		gpOpt = gainPreselectGpOptUpperQ14
	}
	return gainSearchPreselectContainsWithGpOpt(ctx, gpcSearchQ12, ga, gb, gpOpt)
}

func gainSearchPreselectContainsWithGpOpt(ctx *gainSearchCostCtx, gpcSearchQ12 int32, ga, gb uint8, gpOptQ14 int64) bool {
	gaOK, gbOK := gainSearchPreselectAxisContainsWithGpOpt(ctx, gpcSearchQ12, ga, gb, gpOptQ14)
	return gaOK && gbOK
}

func gainSearchPreselectAxisContains(ctx *gainSearchCostCtx, gpcSearchQ12 int32, ga, gb uint8) (gaOK, gbOK bool) {
	return gainSearchPreselectAxisContainsWithGpOpt(ctx, gpcSearchQ12, ga, gb, ctx.gpOptQ14)
}

func gainSearchPreselectAxisContainsWithGpOpt(ctx *gainSearchCostCtx, gpcSearchQ12 int32, ga, gb uint8, gpOptQ14 int64) (gaOK, gbOK bool) {
	var betterGA int
	for i := uint8(0); i < 8; i++ {
		cand := (int64(tables.GainGBK1[i][1]) * int64(gpcSearchQ12)) >> 13
		if absInt64Diagnostic(cand-ctx.gcOptQ12) < absInt64Diagnostic(((int64(tables.GainGBK1[ga][1])*int64(gpcSearchQ12))>>13)-ctx.gcOptQ12) {
			betterGA++
		}
	}
	if betterGA >= 4 {
		gaOK = false
	} else {
		gaOK = true
	}
	var betterGB int
	for j := uint8(0); j < 16; j++ {
		if absInt64Diagnostic(int64(tables.GainGBK2[j][0])-gpOptQ14) < absInt64Diagnostic(int64(tables.GainGBK2[gb][0])-gpOptQ14) {
			betterGB++
		}
	}
	gbOK = betterGB < 8
	return gaOK, gbOK
}

func collectExternalBCGForcedGainSelection(t *testing.T, samples []int16, bcgFrames []bitstream.Frame, forceLSP bool, commit string) forcedGainSelectionStats {
	t.Helper()
	enc := NewEncoder()
	var forcedDec lsp.Decoder
	var stats forcedGainSelectionStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		if frameIndex >= len(bcgFrames) {
			t.Fatalf("missing bcg frame %d", frameIndex)
		}
		ref := bcgFrames[frameIndex]
		if forceLSP {
			lpcAnalysisPreludeForDiagnostic(t, enc, samples[off:off+FrameSamples])
			idx := lspIndicesFromFrame(ref)
			enc.l0 = uint16(idx.L0)
			enc.l1 = uint16(idx.L1)
			enc.l2 = uint16(idx.L2)
			enc.l3 = uint16(idx.L3)
			enc.aHatSF1, enc.aHatSF2 = forcedDec.Decode(idx)
			lsp.CommitIndicesForDiagnostic(&enc.freqPrev, idx)
		} else if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), t1)
		observeExternalBCGCodeGainSelection(t, enc, frameIndex, 0, int16(t1), int8(frac1), uint16(ref.P1), ref.C1, uint8(ref.S1), uint8(ref.GA1), uint8(ref.GB1), commit, &stats)
		observeExternalBCGCodeGainSelection(t, enc, frameIndex, 1, int16(t2), int8(frac2), uint16(ref.P2), ref.C2, uint8(ref.S2), uint8(ref.GA2), uint8(ref.GB2), commit, &stats)
	}
	return stats
}

func observeExternalBCGCodeGainSelection(
	t *testing.T,
	e *Encoder,
	frameIndex, sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC uint16,
	refS uint8,
	refGA uint8,
	refGB uint8,
	commit string,
	stats *forcedGainSelectionStats,
) {
	t.Helper()
	x, y, h, v, _, sFrame := forcedPitchSurface(e, sub, intLag, frac)
	setForcedPitchFields(e, sub, intLag, frac, pitchCode)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: refS}, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14), &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	xSearch := x
	ySearch := y
	scaleGainSearchVector(&xSearch, 2, 5)
	scaleGainSearchVector(&ySearch, 3, 1)
	gpcSearchQ12 := scaleInt32RatioForGainSearch(gpcPredQ12, 4, 3)
	ownGAPhys, ownGBPhys, ownGpTxQ14, ownGammaQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	ownBitsGA, ownBitsGB := gainquant.PackGains(ownGAPhys, ownGBPhys)
	ownGpCommitQ14 := gainquant.Tame(ownGpTxQ14, &e.oldExc)
	_, ownGcMantQ14, ownGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, ownGAPhys, ownGBPhys)
	ownGcQ12 := mantExpToQ12(ownGcMantQ14, ownGcExp)

	refGAPhys := tables.GainImap1[refGA&7]
	refGBPhys := tables.GainImap2[refGB&15]
	refGpQ14, refGcMantQ14, refGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, refGAPhys, refGBPhys)
	refGammaQ13 := gainGammaQ13(refGAPhys, refGBPhys)
	refGcQ12 := mantExpToQ12(refGcMantQ14, refGcExp)

	stats.count++
	stats.ownGpSum += int64(ownGpTxQ14)
	stats.refGpSum += int64(refGpQ14)
	stats.ownGcSum += int64(ownGcQ12)
	stats.refGcSum += int64(refGcQ12)
	if ownBitsGA == refGA&7 {
		stats.sameGA++
	}
	if ownBitsGB == refGB&15 {
		stats.sameGB++
	}
	if ownBitsGA == refGA&7 && ownBitsGB == refGB&15 {
		stats.sameBoth++
	}
	if ownGpTxQ14 < refGpQ14 {
		stats.ownLowerGp++
	}
	if ownGammaQ13 < refGammaQ13 {
		stats.ownLowerGamma++
	}
	if ownGcQ12 < refGcQ12 {
		stats.ownLowerGc++
	}
	if (ownBitsGA != refGA&7 || ownBitsGB != refGB&15) && len(stats.examples) < 4 {
		stats.examples = append(stats.examples, forcedGainSelectionExample{
			frame:      frameIndex,
			sub:        sub,
			ownGA:      ownBitsGA,
			ownGB:      ownBitsGB,
			ownGpQ14:   ownGpTxQ14,
			ownGcQ12:   ownGcQ12,
			refGA:      refGA & 7,
			refGB:      refGB & 15,
			refGpQ14:   refGpQ14,
			refGcQ12:   refGcQ12,
			gpcPredQ12: gpcPredQ12,
		})
	}

	var gpCommitQ14, gcMantQ14 int16
	var gcExp int8
	var gammaQ13 int16
	switch commit {
	case "own":
		gpCommitQ14, gcMantQ14, gcExp = ownGpCommitQ14, ownGcMantQ14, ownGcExp
		gammaQ13 = ownGammaQ13
	case "bcg":
		gpCommitQ14, gcMantQ14, gcExp = refGpQ14, refGcMantQ14, refGcExp
		gammaQ13 = refGammaQ13
	default:
		t.Fatalf("unknown forced BCG gain-selection commit mode %q", commit)
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(gpCommitQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}
	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(gpCommitQ14, gcMantQ14, gcExp, &v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaQ13)
	e.prevGpQ14 = gpCommitQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func forcedPitchSurface(e *Encoder, sub int, intLag int16, frac int8) (x, y, h, v [clpitch.SubframeLen]int16, gp int16, sFrame *[40]int16) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	if e.qualityEarlyClosedLoopSpeechWindowEnabled() {
		sStart = 80 + 40*sub
	}
	sFrame = (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	e.adaptiveVectorForSynthesis(exc, intLag, frac, &v)
	gp = clpitch.GpAndY(&x, &v, &h, &y)
	return x, y, h, v, gp, sFrame
}

func setForcedPitchFields(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16) {
	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(pitchCode)
		e.p0 = clpitch.EncodeP0(e.p1)
		return
	}
	e.intT2 = intLag
	e.frac2 = frac
	e.p2 = uint8(pitchCode)
}

func lpcAnalysisPreludeForDiagnostic(t *testing.T, e *Encoder, pcm []int16) {
	t.Helper()
	if len(pcm) != FrameSamples {
		t.Fatalf("lpcAnalysisPreludeForDiagnostic got %d samples, want %d", len(pcm), FrameSamples)
	}

	var processed [FrameSamples]int16
	e.pre.Process(pcm, processed[:])

	copy(e.oldSpeech[0:160], e.oldSpeech[80:240])
	copy(e.oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := e.lpc.Analyze(&e.oldSpeech, &aQ12); err != nil {
		t.Fatalf("lpc Analyze: %v", err)
	}
	e.aQ12Latest = aQ12

	var qQ15 [10]int16
	if err := lsp.LPToLSP(&aQ12, &qQ15); err != nil {
		if errors.Is(err, lsp.ErrLPCNonStable) {
			qQ15 = e.lspOld
			e.lspReuseCount++
		} else {
			t.Fatalf("LPToLSP: %v", err)
		}
	} else {
		e.lspOld = qQ15
	}
}

func lpcStepLSPTopK(t *testing.T, e *Encoder, pcm []int16, topK int) {
	t.Helper()
	if len(pcm) != FrameSamples {
		t.Fatalf("lpcStepLSPTopK got %d samples, want %d", len(pcm), FrameSamples)
	}

	var processed [FrameSamples]int16
	e.pre.Process(pcm, processed[:])

	copy(e.oldSpeech[0:160], e.oldSpeech[80:240])
	copy(e.oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := e.lpc.Analyze(&e.oldSpeech, &aQ12); err != nil {
		t.Fatalf("lpc Analyze: %v", err)
	}
	e.aQ12Latest = aQ12

	var qQ15 [10]int16
	if err := lsp.LPToLSP(&aQ12, &qQ15); err != nil {
		if errors.Is(err, lsp.ErrLPCNonStable) {
			qQ15 = e.lspOld
			e.lspReuseCount++
		} else {
			t.Fatalf("LPToLSP: %v", err)
		}
	} else {
		e.lspOld = qQ15
	}

	var omega [10]int16
	lsp.LSPToLSF(&qQ15, &omega)
	indices := lsp.QuantizeTopK(&omega, &e.freqPrev, topK)

	e.l0 = uint16(indices.L0)
	e.l1 = uint16(indices.L1)
	e.l2 = uint16(indices.L2)
	e.l3 = uint16(indices.L3)
	e.aHatSF1, e.aHatSF2 = e.lspDec.Decode(indices)
}

func externalSampleQualityPath() string {
	if path := strings.TrimSpace(os.Getenv("G729_EXTERNAL_SAMPLE_QUALITY")); path != "" {
		return path
	}
	for _, path := range externalSampleQualityFallbackPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func externalSampleQualityFallbackPaths() []string {
	return []string{
		"testdata/external/user_quality_audio.m4a",
		"testdata/external/user_quality_input.m4a",
		"testdata/external/user_quality_input.wav",
		"testdata/external/user_quality_input.mp3",
		"testdata/external/user_quality_input.pcm",
		"testdata/external/user_quality_input.raw",
		"testdata/external/user_quality_input.sln",
		"testdata/external/user_quality_input.s16le",
		"testdata/external/user_quality_input.in",
	}
}

func TestExternalSampleQualityFallbackPaths(t *testing.T) {
	paths := externalSampleQualityFallbackPaths()
	if len(paths) < 2 {
		t.Fatalf("external sample fallback paths = %v, want at least new and legacy samples", paths)
	}
	if paths[0] != "testdata/external/user_quality_audio.m4a" {
		t.Fatalf("first external sample fallback = %q, want new problem sample", paths[0])
	}
	if paths[1] != "testdata/external/user_quality_input.m4a" {
		t.Fatalf("second external sample fallback = %q, want legacy m4a sample", paths[1])
	}
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

func pesqNBScore(t *testing.T, tmp, name string, ref, deg []int16) float64 {
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
	refPath := filepath.Join(tmp, name+".ref.wav")
	degPath := filepath.Join(tmp, name+".deg.wav")
	if err := os.WriteFile(refPath, wavBytesFromSamples(ref), 0o600); err != nil {
		t.Fatalf("write PESQ ref WAV %s: %v", refPath, err)
	}
	if err := os.WriteFile(degPath, wavBytesFromSamples(deg), 0o600); err != nil {
		t.Fatalf("write PESQ deg WAV %s: %v", degPath, err)
	}
	out, err := exec.Command(python, "-c", pesqNBPythonScript, refPath, degPath).CombinedOutput()
	if err != nil {
		t.Fatalf("PESQ NB failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
		t.Fatalf("invalid PESQ NB output %q: %v", strings.TrimSpace(string(out)), err)
	}
	return score
}

const pesqNBPythonScript = `
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

func wavBytesFromSamples(samples []int16) []byte {
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
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, dataLen)
	for _, sample := range samples {
		_ = binary.Write(&b, binary.LittleEndian, sample)
	}
	return b.Bytes()
}
