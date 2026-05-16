package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/lsp"
	pitchidx "github.com/hunydev/g729/internal/pitch"
	clpitch "github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// TestExternalFFmpegBlackboxQuality_SPEECH is an opt-in diagnostic that
// uses the local ffmpeg binary only as an external decoder executable.
// It does not inspect or import external implementation code. The goal is
// to separate encoder defects from decoder defects:
//
//   - SPEECH.BIT -> ffmpeg decode establishes that the local G.192->raw
//     payload conversion and ffmpeg path are sane.
//   - SPEECH.IN -> our encoder -> ffmpeg decode measures our encoder
//     output without using our decoder.
//
// Enable with G729_FFMPEG_BLACKBOX_QUALITY=1.
func TestExternalFFmpegBlackboxQuality_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box quality diagnostic")
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
	pstData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.PST"))
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	if pf := len(pstData) / bytesPerInFrame; pf < frames {
		frames = pf
	}
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])
	pst := s16leToSamples(pstData[:totalSamples*2])

	tmp := t.TempDir()
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ourRaw := filepath.Join(tmp, "speech-our-encoder.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	ourPCM := filepath.Join(tmp, "speech-our-encoder.ffmpeg.s16le")

	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	writeOurEncodedRawG729(t, src, ourRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ffmpegDecodeRawG729(t, ourRaw, ourPCM)

	ituFF := s16leToSamples(readFile(t, ituPCM))
	ourFF := s16leToSamples(readFile(t, ourPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ourFF) > totalSamples {
		ourFF = ourFF[:totalSamples]
	}
	if len(ituFF) < totalSamples || len(ourFF) < totalSamples {
		t.Fatalf("ffmpeg output too short: itu=%d our=%d want>=%d", len(ituFF), len(ourFF), totalSamples)
	}

	const maxShift = 240
	shPST, gPST, sPST := bestAlignedSNR(src, pst, maxShift)
	shITU, gITU, sITU := bestAlignedSNR(src, ituFF, maxShift)
	shOur, gOur, sOur := bestAlignedSNR(src, ourFF, maxShift)

	t.Logf("ffmpeg black-box quality report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-42s %6s %10s %10s %10s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR")
	t.Logf("%-42s %6d %10.0f %10.2f %10.2f", "SPEECH.PST", shPST, rmsAmp(pst), gPST, sPST)
	t.Logf("%-42s %6d %10.0f %10.2f %10.2f", "SPEECH.BIT -> ffmpeg", shITU, rmsAmp(ituFF), gITU, sITU)
	t.Logf("%-42s %6d %10.0f %10.2f %10.2f", "our encoder -> ffmpeg", shOur, rmsAmp(ourFF), gOur, sOur)
	for _, shift := range []int{-80, -40, 40, 80} {
		shiftedRaw := filepath.Join(tmp, "speech-our-encoder-shift.g729")
		shiftedPCM := filepath.Join(tmp, "speech-our-encoder-shift.s16le")
		writeOurEncodedRawG729(t, shiftedSamples(src, shift), shiftedRaw)
		ffmpegDecodeRawG729(t, shiftedRaw, shiftedPCM)
		shiftedFF := s16leToSamples(readFile(t, shiftedPCM))
		if len(shiftedFF) > totalSamples {
			shiftedFF = shiftedFF[:totalSamples]
		}
		if len(shiftedFF) < totalSamples {
			t.Fatalf("ffmpeg shifted output too short: shift=%d got=%d want>=%d", shift, len(shiftedFF), totalSamples)
		}
		sh, g, s := bestAlignedSNR(src, shiftedFF, maxShift)
		t.Logf("%-42s %6d %10.0f %10.2f %10.2f", "our encoder shift "+itoaSigned(shift)+" -> ffmpeg", sh, rmsAmp(shiftedFF), g, s)
	}
	t.Logf("encoder-only delta vs ffmpeg reference decode: GlobalSNR %+0.2f dB ; SegSNR %+0.2f dB", gOur-gITU, sOur-sITU)
	if os.Getenv("G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY") == "1" {
		const minDeltaDB = -2.0
		if gOur-gITU < minDeltaDB || sOur-sITU < minDeltaDB {
			t.Fatalf("our encoder -> ffmpeg quality below release gate: Global delta %.2f dB, Seg delta %.2f dB; require both >= %.2f dB",
				gOur-gITU, sOur-sITU, minDeltaDB)
		}
	}
}

func TestWriteOurEncodedRawG729UsesProductDefault(t *testing.T) {
	samples := make([]int16, FrameSamples*3)
	for i := range samples {
		// Deterministic non-silence across several frames so encoder state is
		// exercised without depending on external media fixtures.
		samples[i] = int16(((i*251 + 97) % 4096) - 2048)
	}

	tmp := t.TempDir()
	helperPath := filepath.Join(tmp, "helper.g729")
	defaultPath := filepath.Join(tmp, "default.g729")
	writeOurEncodedRawG729(t, samples, helperPath)
	writeRawG729WithEncoder(t, samples, defaultPath, NewEncoder())

	helper := readFile(t, helperPath)
	def := readFile(t, defaultPath)
	if !bytes.Equal(helper, def) {
		t.Fatalf("writeOurEncodedRawG729 helper no longer matches NewEncoder product default")
	}
}

func TestExternalFFmpegBlackboxProfileCompare_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_PROFILE_COMPARE") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_PROFILE_COMPARE=1 to compare encoder profiles on SPEECH")
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
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ituFF := s16leToSamples(readFile(t, ituPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ituFF) < totalSamples {
		t.Fatalf("ffmpeg reference output too short: got=%d want>=%d", len(ituFF), totalSamples)
	}

	type result struct {
		name  string
		raw   []byte
		pcm   []int16
		shift int
		gSNR  float64
		sSNR  float64
	}
	profiles := []struct {
		name    string
		profile EncoderProfile
	}{
		{name: "core", profile: EncoderProfileCore},
		{name: "quality", profile: EncoderProfileQuality},
	}
	results := make([]result, 0, len(profiles))
	for _, p := range profiles {
		rawPath := filepath.Join(tmp, "speech-"+p.name+".g729")
		pcmPath := filepath.Join(tmp, "speech-"+p.name+".ffmpeg.s16le")
		writeOurEncodedRawG729WithProfile(t, src, rawPath, p.profile)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		raw := readFile(t, rawPath)
		pcm := s16leToSamples(readFile(t, pcmPath))
		if len(pcm) > totalSamples {
			pcm = pcm[:totalSamples]
		}
		if len(pcm) < totalSamples {
			t.Fatalf("%s ffmpeg output too short: got=%d want>=%d", p.name, len(pcm), totalSamples)
		}
		shift, g, s := bestAlignedSNR(src, pcm, 240)
		results = append(results, result{name: p.name, raw: raw, pcm: pcm, shift: shift, gSNR: g, sSNR: s})
	}

	shITU, gITU, sITU := bestAlignedSNR(src, ituFF, 240)
	t.Logf("ffmpeg black-box profile compare — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-24s %6s %10s %10s %10s %10s %10s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "dGlobal", "dSeg")
	t.Logf("%-24s %6d %10.0f %10.2f %10.2f %10s %10s", "SPEECH.BIT -> ffmpeg", shITU, rmsAmp(ituFF), gITU, sITU, "-", "-")
	for _, r := range results {
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f %10.2f %10.2f",
			r.name+" -> ffmpeg", r.shift, rmsAmp(r.pcm), r.gSNR, r.sSNR, r.gSNR-gITU, r.sSNR-sITU)
	}
	if len(results) == 2 {
		t.Logf("core-vs-quality payload byte equality %.2f%%", payloadEqualPercent(results[0].raw, results[1].raw))
		sh, g, s := bestAlignedSNR(results[0].pcm, results[1].pcm, 240)
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f", "quality vs core", sh, rmsAmp(results[1].pcm), g, s)
	}
}

func TestExternalFFmpegBlackboxTuningAblation_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_TUNING_ABLATION") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_TUNING_ABLATION=1 to split encoder tuning effects on SPEECH")
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

	variants := []struct {
		name   string
		tuning encoderQualityTuning
	}{
		{name: "core", tuning: 0},
		{name: "core-wide-flag", tuning: encoderTuningWideGainPredictor},
		{name: "core-bounded-pred", tuning: encoderDiagnosticBoundedGainPredictorTuning},
		{name: "fcb+wide", tuning: encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "pitch+fcb+wide", tuning: encoderTuningPitchCenterCandidate | encoderTuningFCBThresholdScan | encoderTuningWideGainPredictor},
		{name: "gain", tuning: encoderTuningGainSearchBias},
		{name: "gain+wide", tuning: encoderTuningGainSearchBias | encoderTuningWideGainPredictor},
		{name: "norm+gain+early", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow},
		{name: "norm+gain+early+wide", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningWideGainPredictor},
		{name: "norm+gain+early+residacb", tuning: encoderTuningNormalizedAdaptivePitchSearch | encoderTuningGainSearchBias | encoderTuningEarlyClosedLoopSpeechWindow | encoderTuningResidualExtensionAdaptiveVector},
		{name: "quality+pitch", tuning: encoderQualityTuningAll | encoderTuningPitchCenterCandidate},
		{name: "quality-no-fcb", tuning: encoderQualityTuningAll &^ encoderTuningFCBThresholdScan},
		{name: "quality+lspx", tuning: encoderQualityTuningAll | encoderTuningExpandedLSPSearch},
		{name: "quality-wide-no-gain", tuning: (encoderQualityTuningAll &^ encoderTuningGainSearchBias) | encoderTuningWideGainPredictor},
		{name: "quality-wide+gain", tuning: encoderQualityTuningAll | encoderTuningWideGainPredictor},
		{name: "quality", tuning: encoderQualityTuningAll},
		{name: "quality+residacb", tuning: encoderQualityTuningAll | encoderTuningResidualExtensionAdaptiveVector},
	}

	tmp := t.TempDir()
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ituFF := s16leToSamples(readFile(t, ituPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ituFF) < totalSamples {
		t.Fatalf("SPEECH.BIT ffmpeg output too short: got=%d want>=%d", len(ituFF), totalSamples)
	}

	type result struct {
		name string
		raw  []byte
		m    externalQualityMetrics
	}
	results := make([]result, 0, len(variants))
	for _, v := range variants {
		rawPath := filepath.Join(tmp, "speech-"+strings.ReplaceAll(v.name, "+", "_")+".g729")
		pcmPath := filepath.Join(tmp, "speech-"+strings.ReplaceAll(v.name, "+", "_")+".ffmpeg.s16le")
		writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, v.tuning)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		raw := readFile(t, rawPath)
		pcm := s16leToSamples(readFile(t, pcmPath))
		if len(pcm) > totalSamples {
			pcm = pcm[:totalSamples]
		}
		if len(pcm) < totalSamples {
			t.Fatalf("%s ffmpeg output too short: got=%d want>=%d", v.name, len(pcm), totalSamples)
		}
		results = append(results, result{
			name: v.name,
			raw:  raw,
			m:    externalQualityMetricsFor(src, pcm, 240),
		})
	}

	ituMetrics := externalQualityMetricsFor(src, ituFF, 240)
	var qualityRaw []byte
	for _, r := range results {
		if r.name == "quality" {
			qualityRaw = r.raw
			break
		}
	}
	if qualityRaw == nil {
		t.Fatal("quality variant missing from SPEECH tuning ablation results")
	}
	t.Logf("ffmpeg black-box tuning ablation — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-28s %6s %10s %10s %10s %8s %8s %7s %8s %10s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip", "eqQuality")
	t.Logf("%-28s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %10s",
		"SPEECH.BIT -> ffmpeg", ituMetrics.shift, ituMetrics.rms, ituMetrics.globalSNR, ituMetrics.segSNR,
		ituMetrics.corr, ituMetrics.rmsRatio, ituMetrics.peak, ituMetrics.nearClip, "-")
	for _, r := range results {
		t.Logf("%-28s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d %9.2f%%",
			r.name+" -> ffmpeg", r.m.shift, r.m.rms, r.m.globalSNR, r.m.segSNR,
			r.m.corr, r.m.rmsRatio, r.m.peak, r.m.nearClip, payloadEqualPercent(r.raw, qualityRaw))
	}
}

func TestExternalFFmpegBlackboxProductionGainMode_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_PRODUCTION_GAIN_MODE") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_PRODUCTION_GAIN_MODE=1 to compare production-state gain modes on SPEECH")
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
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ituFF := s16leToSamples(readFile(t, ituPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ituFF) < totalSamples {
		t.Fatalf("SPEECH.BIT ffmpeg output too short: got=%d want>=%d", len(ituFF), totalSamples)
	}

	modes := []externalProductionGainMode{
		{name: "core-production", production: true},
		{name: "preselect-wide", search: "preselect", wide: true},
		{name: "norm24-wide", search: "preselect-norm", wide: true, preselectTargetBits: 24},
		{name: "norm24-wide-repair30000", search: "preselect-norm", wide: true, gainClipRepair: true, gainClipRepairThreshold: 30000, preselectTargetBits: 24},
		{name: "norm24-wide-repair28400", search: "preselect-norm", wide: true, gainClipRepair: true, gainClipRepairThreshold: 28400, preselectTargetBits: 24},
		{name: "bigopt-wide-repair30400", search: "preselect-bigopt", wide: true, gainClipRepair: true, gainClipRepairThreshold: 30400},
		{name: "preselect-linear-wide", search: "preselect-linear", wide: true},
		{name: "preselect-native-wide", search: "preselect-native", wide: true},
		{name: "native-wide", search: "native", wide: true},
	}

	ituMetrics := externalQualityMetricsFor(src, ituFF, 240)
	t.Logf("ffmpeg black-box production gain-mode report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-26s %6s %10s %10s %10s %8s %8s %7s %8s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	t.Logf("%-26s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		"SPEECH.BIT -> ffmpeg", ituMetrics.shift, ituMetrics.rms, ituMetrics.globalSNR, ituMetrics.segSNR,
		ituMetrics.corr, ituMetrics.rmsRatio, ituMetrics.peak, ituMetrics.nearClip)
	for _, mode := range modes {
		rawPath := filepath.Join(tmp, "speech-"+strings.ReplaceAll(mode.name, "+", "_")+".g729")
		pcmPath := filepath.Join(tmp, "speech-"+strings.ReplaceAll(mode.name, "+", "_")+".ffmpeg.s16le")
		writePackedFrames(t, encodeBitstreamFramesProductionGainMode(t, src, mode), rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		pcm := s16leToSamples(readFile(t, pcmPath))
		if len(pcm) > totalSamples {
			pcm = pcm[:totalSamples]
		}
		if len(pcm) < totalSamples {
			t.Fatalf("%s ffmpeg output too short: got=%d want>=%d", mode.name, len(pcm), totalSamples)
		}
		m := externalQualityMetricsFor(src, pcm, 240)
		t.Logf("%-26s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			mode.name+" -> ffmpeg", m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
	}
}

func TestExternalFFmpegBlackboxOpenLoopTopVariant_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_OPENLOOP_TOP_VARIANT") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_OPENLOOP_TOP_VARIANT=1 to compare diagnostic open-loop T_op choices on SPEECH")
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
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ituFF := s16leToSamples(readFile(t, ituPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ituFF) < totalSamples {
		t.Fatalf("SPEECH.BIT ffmpeg output too short: got=%d want>=%d", len(ituFF), totalSamples)
	}

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
	}

	ituMetrics := externalQualityMetricsFor(src, ituFF, 240)
	t.Logf("ffmpeg black-box open-loop T_op variant report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-8s %-12s %6s %10s %10s %10s %8s %8s %7s %8s", "Profile", "Variant", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	t.Logf("%-8s %-12s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		"ref", "ffmpeg", ituMetrics.shift, ituMetrics.rms, ituMetrics.globalSNR, ituMetrics.segSNR,
		ituMetrics.corr, ituMetrics.rmsRatio, ituMetrics.peak, ituMetrics.nearClip)
	for _, p := range profiles {
		for _, v := range variants {
			rawPath := filepath.Join(tmp, fmt.Sprintf("speech-%s-%s.g729", p.name, strings.ReplaceAll(v.name, "+", "_")))
			pcmPath := filepath.Join(tmp, fmt.Sprintf("speech-%s-%s.ffmpeg.s16le", p.name, strings.ReplaceAll(v.name, "+", "_")))
			writePackedFrames(t, encodeBitstreamFramesOpenLoopTopVariant(t, src, p.profile, v), rawPath)
			ffmpegDecodeRawG729(t, rawPath, pcmPath)
			pcm := s16leToSamples(readFile(t, pcmPath))
			if len(pcm) > totalSamples {
				pcm = pcm[:totalSamples]
			}
			if len(pcm) < totalSamples {
				t.Fatalf("%s/%s ffmpeg output too short: got=%d want>=%d", p.name, v.name, len(pcm), totalSamples)
			}
			m := externalQualityMetricsFor(src, pcm, 240)
			t.Logf("%-8s %-12s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
				p.name, v.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
		}
	}
}

func TestExternalFFmpegBlackboxOpenLoopLiftSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_OPENLOOP_LIFT_SWEEP") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_OPENLOOP_LIFT_SWEEP=1 to sweep diagnostic Core open-loop submultiple lift on SPEECH")
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
	ituRaw := filepath.Join(tmp, "speech-itu.g729")
	ituPCM := filepath.Join(tmp, "speech-itu.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], ituRaw)
	ffmpegDecodeRawG729(t, ituRaw, ituPCM)
	ituFF := s16leToSamples(readFile(t, ituPCM))
	if len(ituFF) > totalSamples {
		ituFF = ituFF[:totalSamples]
	}
	if len(ituFF) < totalSamples {
		t.Fatalf("SPEECH.BIT ffmpeg output too short: got=%d want>=%d", len(ituFF), totalSamples)
	}

	ituMetrics := externalQualityMetricsFor(src, ituFF, 240)
	lifts := []float64{1.05, 1.10, 1.12, 1.15, 20.0 / 17.0, 1.20, 1.30, 1.50, 1.75, 2.00}
	t.Logf("ffmpeg black-box Core open-loop submultiple-lift sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-8s %8s %6s %10s %10s %10s %8s %8s %7s %8s",
		"Lift", "chgTop", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	t.Logf("%-8s %8s %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
		"ref", "-", ituMetrics.shift, ituMetrics.rms, ituMetrics.globalSNR, ituMetrics.segSNR,
		ituMetrics.corr, ituMetrics.rmsRatio, ituMetrics.peak, ituMetrics.nearClip)
	for _, lift := range lifts {
		rawPath := filepath.Join(tmp, fmt.Sprintf("speech-core-lift-%.2f.g729", lift))
		pcmPath := filepath.Join(tmp, fmt.Sprintf("speech-core-lift-%.2f.ffmpeg.s16le", lift))
		encoded, changedTop := encodeBitstreamFramesCoreOpenLoopLift(t, src, lift)
		writePackedFrames(t, encoded, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		pcm := s16leToSamples(readFile(t, pcmPath))
		if len(pcm) > totalSamples {
			pcm = pcm[:totalSamples]
		}
		if len(pcm) < totalSamples {
			t.Fatalf("lift %.2f ffmpeg output too short: got=%d want>=%d", lift, len(pcm), totalSamples)
		}
		m := externalQualityMetricsFor(src, pcm, 240)
		t.Logf("%-8.2f %8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			lift, changedTop, m.shift, m.rms, m.globalSNR, m.segSNR,
			m.corr, m.rmsRatio, m.peak, m.nearClip)
	}
}

func TestExternalFFmpegBlackboxClippedOpenLoopTopVariant_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_CLIPPED_OPENLOOP_TOP_VARIANT") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_CLIPPED_OPENLOOP_TOP_VARIANT=1 to compare clipped-input open-loop T_op choices on SPEECH")
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

	type mode struct {
		name      string
		threshold int
		cooldown  int
		variant   externalOpenLoopTopVariant
	}
	modes := []mode{
		{name: "current"},
		{name: "r2c95-32700-c5", threshold: 32700, cooldown: 5, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c10", threshold: 32700, cooldown: 10, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
		{name: "r2c95-32700-c20", threshold: 32700, cooldown: 20, variant: externalOpenLoopTopVariant{mode: "range2-close:0.95"}},
	}

	tmp := t.TempDir()
	t.Logf("ffmpeg black-box clipped-input open-loop T_op variant report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-17s %8s %8s %6s %10s %10s %10s %8s %8s %7s %8s",
		"Mode", "swFrames", "chgFrames", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, mode := range modes {
		rawPath := filepath.Join(tmp, fmt.Sprintf("speech-%s.g729", strings.ReplaceAll(mode.name, "+", "_")))
		pcmPath := filepath.Join(tmp, fmt.Sprintf("speech-%s.ffmpeg.s16le", strings.ReplaceAll(mode.name, "+", "_")))
		outFrames, switched, changed := encodeBitstreamFramesClippedOpenLoopTopVariant(t, src, mode.threshold, mode.cooldown, mode.variant)
		writePackedFrames(t, outFrames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		pcm := s16leToSamples(readFile(t, pcmPath))
		if len(pcm) > totalSamples {
			pcm = pcm[:totalSamples]
		}
		if len(pcm) < totalSamples {
			t.Fatalf("%s ffmpeg output too short: got=%d want>=%d", mode.name, len(pcm), totalSamples)
		}
		m := externalQualityMetricsFor(src, pcm, 240)
		t.Logf("%-17s %8d %8d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
			mode.name, switched, changed, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
	}
}

func TestExternalFFmpegBlackboxGainPreselectNativeAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_GAIN_PRESELECT_NATIVE_AUDIT") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_GAIN_PRESELECT_NATIVE_AUDIT=1 to audit native gain optimum vs Annex A preselect on SPEECH")
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

	stats := collectGainPreselectNativeAudit(t, src)
	logGainPreselectNativeAudit(t, fmt.Sprintf("SPEECH gain preselect/native audit: frames=%d samples=%d", frames, totalSamples), stats)
}

func TestExternalFFmpegBlackboxGainCostModelAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_GAIN_COST_MODEL_AUDIT") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_GAIN_COST_MODEL_AUDIT=1 to compare eq.63 cost ordering against direct linear residual on SPEECH")
	}

	const bytesPerInFrame = 2 * FrameSamples
	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.IN"))
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}

	frames := len(inData) / bytesPerInFrame
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])

	bounded := collectGainCostModelAudit(t, src, false)
	logGainCostModelAudit(t, fmt.Sprintf("SPEECH gain cost-model audit bounded: frames=%d samples=%d", frames, totalSamples), bounded)
	wide := collectGainCostModelAudit(t, src, true)
	logGainCostModelAudit(t, fmt.Sprintf("SPEECH gain cost-model audit wide: frames=%d samples=%d", frames, totalSamples), wide)
}

func TestExternalFFmpegBlackboxFCBThresholdLimit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_FCB_THRESHOLD_LIMIT") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_FCB_THRESHOLD_LIMIT=1 to sweep focused FCB search limits on SPEECH")
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
	t.Logf("ffmpeg black-box focused FCB threshold-limit diagnostic — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-16s %5s %6s %10s %10s %10s %8s %8s %7s %8s", "Mode", "Limit", "shift", "RMS", "GlobalSNR", "SegSNR", "Corr", "RMS/ref", "Peak", "NearClip")
	for _, limit := range limits {
		qualityFCBThresholdScanLimit = limit
		for _, mode := range modes {
			name := fmt.Sprintf("%s-l%d", strings.ReplaceAll(mode.name, "+", "_"), limit)
			rawPath := filepath.Join(tmp, name+".g729")
			pcmPath := filepath.Join(tmp, name+".ffmpeg.s16le")
			writeOurEncodedRawG729WithTuning(t, src, rawPath, EncoderProfileCore, mode.tuning)
			ffmpegDecodeRawG729(t, rawPath, pcmPath)
			ff := s16leToSamples(readFile(t, pcmPath))
			if len(ff) > totalSamples {
				ff = ff[:totalSamples]
			}
			if len(ff) < totalSamples {
				t.Fatalf("%s decoded output too short: ffmpeg=%d want >= %d", name, len(ff), totalSamples)
			}
			m := externalQualityMetricsFor(src, ff, 240)
			t.Logf("%-16s %5d %6d %10.0f %10.2f %10.2f %8.4f %8.4f %7d %8d",
				mode.name, limit, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.rmsRatio, m.peak, m.nearClip)
		}
	}
}

// TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH compares local decode
// against FFmpeg executable black-box decode for the exact G.729 payload
// emitted by the local encoder. FFmpeg is used only as an external process.
//
// Enable with G729_FFMPEG_BLACKBOX_QUALITY=1.
func TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run local decoder vs ffmpeg diagnostic")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	const bytesPerInFrame = 2 * FrameSamples
	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.IN"))
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}
	frames := len(inData) / bytesPerInFrame
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])

	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, "speech-our-encoder.g729")
	ffPCMPath := filepath.Join(tmp, "speech-our-encoder.ffmpeg.s16le")
	writeOurEncodedRawG729(t, src, rawPath)
	ffmpegDecodeRawG729(t, rawPath, ffPCMPath)

	raw := readFile(t, rawPath)
	ff := s16leToSamples(readFile(t, ffPCMPath))
	local := decodeRawG729WithLocal(t, raw)
	enhanced := decodeRawG729WithLocalEnhanced(t, raw)
	if len(ff) > totalSamples {
		ff = ff[:totalSamples]
	}
	if len(local) > totalSamples {
		local = local[:totalSamples]
	}
	if len(enhanced) > totalSamples {
		enhanced = enhanced[:totalSamples]
	}
	if len(ff) < totalSamples || len(local) < totalSamples || len(enhanced) < totalSamples {
		t.Fatalf("decoded output too short: ffmpeg=%d local=%d enhanced=%d want >= %d", len(ff), len(local), len(enhanced), totalSamples)
	}

	const maxShift = 240
	shFF, gFF, sFF := bestAlignedSNR(src, ff, maxShift)
	shLocal, gLocal, sLocal := bestAlignedSNR(src, local, maxShift)
	shLocalVsFF, gLocalVsFF, sLocalVsFF := bestAlignedSNR(ff, local, maxShift)
	shEnhanced, gEnhanced, sEnhanced := bestAlignedSNR(src, enhanced, maxShift)
	shEnhancedVsFF, gEnhancedVsFF, sEnhancedVsFF := bestAlignedSNR(ff, enhanced, maxShift)
	rmsRatio := 0.0
	if ffRMS := rmsAmp(ff); ffRMS > 0 {
		rmsRatio = rmsAmp(local) / ffRMS
	}
	enhancedRMSRatio := 0.0
	if ffRMS := rmsAmp(ff); ffRMS > 0 {
		enhancedRMSRatio = rmsAmp(enhanced) / ffRMS
	}

	t.Logf("local decoder vs ffmpeg black-box — our encoder SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-34s %6s %10s %10s %10s", "Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR")
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "our encoder -> ffmpeg", shFF, rmsAmp(ff), gFF, sFF)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "our encoder -> local decoder", shLocal, rmsAmp(local), gLocal, sLocal)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "our encoder -> enhanced decoder", shEnhanced, rmsAmp(enhanced), gEnhanced, sEnhanced)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "local decoder vs ffmpeg", shLocalVsFF, rmsAmp(local), gLocalVsFF, sLocalVsFF)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f", "enhanced decoder vs ffmpeg", shEnhancedVsFF, rmsAmp(enhanced), gEnhancedVsFF, sEnhancedVsFF)
	t.Logf("local-vs-ffmpeg RMS ratio: %.3f", rmsRatio)
	t.Logf("enhanced-vs-ffmpeg RMS ratio: %.3f", enhancedRMSRatio)
	for _, scale := range []int{2, 3, 4, 5} {
		scaled := scaleSamplesForDiagnostic(local, scale, 1)
		sh, g, s := bestAlignedSNR(src, scaled, maxShift)
		shVsFF, gVsFF, sVsFF := bestAlignedSNR(ff, scaled, maxShift)
		t.Logf("%-34s %6d %10.0f %10.2f %10.2f    vs ffmpeg shift=%+d g=%.2f seg=%.2f",
			"local decoder x"+itoa(scale), sh, rmsAmp(scaled), g, s, shVsFF, gVsFF, sVsFF)
	}
	frameGainMatched := matchFrameRMSForDiagnostic(local, ff)
	shFG, gFG, sFG := bestAlignedSNR(src, frameGainMatched, maxShift)
	shFGVsFF, gFGVsFF, sFGVsFF := bestAlignedSNR(ff, frameGainMatched, maxShift)
	t.Logf("%-34s %6d %10.0f %10.2f %10.2f    vs ffmpeg shift=%+d g=%.2f seg=%.2f",
		"local frame-rms matched", shFG, rmsAmp(frameGainMatched), gFG, sFG, shFGVsFF, gFGVsFF, sFGVsFF)
	t.Logf("decoder delta vs ffmpeg path: GlobalSNR %+0.2f dB ; SegSNR %+0.2f dB",
		gLocal-gFF, sLocal-sFF)
	logWorstLocalDecoderFrames(t, src, ff, local, 8)

	if os.Getenv("G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY") == "1" {
		const (
			minGlobalSNR = 8.0
			minSegSNR    = 8.0
			minRMSRatio  = 0.75
			maxRMSRatio  = 1.50
		)
		if gLocalVsFF < minGlobalSNR || sLocalVsFF < minSegSNR || rmsRatio < minRMSRatio || rmsRatio > maxRMSRatio {
			t.Fatalf("local decoder below FFmpeg black-box quality gate: local-vs-ffmpeg GlobalSNR %.2f<%.2f or SegSNR %.2f<%.2f or rmsRatio %.3f not in [%.2f,%.2f]",
				gLocalVsFF, minGlobalSNR, sLocalVsFF, minSegSNR, rmsRatio, minRMSRatio, maxRMSRatio)
		}
	}
	if os.Getenv("G729_REQUIRE_ENHANCED_LOCAL_DECODER_FFMPEG_QUALITY") == "1" {
		const (
			minGlobalSNR = 8.0
			minSegSNR    = 8.0
			minRMSRatio  = 0.75
			maxRMSRatio  = 1.50
		)
		if gEnhancedVsFF < minGlobalSNR || sEnhancedVsFF < minSegSNR || enhancedRMSRatio < minRMSRatio || enhancedRMSRatio > maxRMSRatio {
			t.Fatalf("enhanced local decoder below FFmpeg black-box quality gate: enhanced-vs-ffmpeg GlobalSNR %.2f<%.2f or SegSNR %.2f<%.2f or rmsRatio %.3f not in [%.2f,%.2f]",
				gEnhancedVsFF, minGlobalSNR, sEnhancedVsFF, minSegSNR, enhancedRMSRatio, minRMSRatio, maxRMSRatio)
		}
	}
}

// TestExternalFFmpegBlackboxHybridFields_SPEECH localizes encoder damage by
// mixing transmitted field families between SPEECH.BIT and our encoder, then
// decoding each hybrid stream with ffmpeg as a black-box decoder.
//
// Enable with G729_FFMPEG_BLACKBOX_QUALITY=1.
func TestExternalFFmpegBlackboxHybridFields_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box hybrid diagnostic")
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
	pstData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.PST"))
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	if pf := len(pstData) / bytesPerInFrame; pf < frames {
		frames = pf
	}
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])
	refFrames := readG192Frames(t, bitData[:frames*bytesPerBitFrame])
	ourFrames := encodeBitstreamFrames(t, src)

	type mode struct {
		name string
		base string
		fam  string
	}
	modes := []mode{
		{name: "ref all", base: "ref"},
		{name: "ref gain inverse", base: "ref", fam: "gainInverse"},
		{name: "ref GA inverse", base: "ref", fam: "gaInverse"},
		{name: "ref GB inverse", base: "ref", fam: "gbInverse"},
		{name: "our all", base: "our"},
		{name: "ref + our LSP", base: "ref", fam: "lsp"},
		{name: "ref + our pitch", base: "ref", fam: "pitch"},
		{name: "ref + our pitch1", base: "ref", fam: "pitch1"},
		{name: "ref + our pitch2", base: "ref", fam: "pitch2"},
		{name: "ref + our FCB", base: "ref", fam: "fcb"},
		{name: "ref + our FCB1", base: "ref", fam: "fcb1"},
		{name: "ref + our FCB2", base: "ref", fam: "fcb2"},
		{name: "ref + our C", base: "ref", fam: "c"},
		{name: "ref + our S", base: "ref", fam: "s"},
		{name: "ref + our gain", base: "ref", fam: "gain"},
		{name: "ref + our gain1", base: "ref", fam: "gain1"},
		{name: "ref + our gain2", base: "ref", fam: "gain2"},
		{name: "ref + our GA", base: "ref", fam: "ga"},
		{name: "ref + our GB", base: "ref", fam: "gb"},
		{name: "our + ref LSP", base: "our", fam: "lsp"},
		{name: "our + ref pitch", base: "our", fam: "pitch"},
		{name: "our + ref FCB", base: "our", fam: "fcb"},
		{name: "our + ref gain", base: "our", fam: "gain"},
		{name: "our + ref FCB+gain", base: "our", fam: "fcb+gain"},
		{name: "our + ref pitch+FCB", base: "our", fam: "pitch+fcb"},
		{name: "our + ref pitch+gain", base: "our", fam: "pitch+gain"},
		{name: "our + ref pitch+FCB+gain", base: "our", fam: "pitch+fcb+gain"},
		{name: "our + ref LSP+pitch+FCB+gain", base: "our", fam: "lsp+pitch+fcb+gain"},
		{name: "ref + our FCB+gain", base: "ref", fam: "fcb+gain"},
		{name: "ref + our pitch+FCB+gain", base: "ref", fam: "pitch+fcb+gain"},
		{name: "our gain low-gamma", base: "our", fam: "gainLowGamma"},
		{name: "our gain low-gp", base: "our", fam: "gainLowGp"},
		{name: "our gain mid", base: "our", fam: "gainMid"},
		{name: "our gain high", base: "our", fam: "gainHigh"},
		{name: "our gain zero-bits", base: "our", fam: "gainZeroBits"},
		{name: "our gain identity", base: "our", fam: "gainIdentity"},
		{name: "our gain inverse", base: "our", fam: "gainInverse"},
		{name: "our GB identity", base: "our", fam: "gbIdentity"},
		{name: "our GB inverse", base: "our", fam: "gbInverse"},
		{name: "our S inverted", base: "our", fam: "signInvert"},
		{name: "our S reversed", base: "our", fam: "signReverse"},
		{name: "our S rev+inv", base: "our", fam: "signReverseInvert"},
		{name: "our C flip jx", base: "our", fam: "cFlipJx"},
		{name: "our pitch frac flip", base: "our", fam: "pitchFracFlip"},
		{name: "our pitch frac flip1", base: "our", fam: "pitchFracFlip1"},
		{name: "our pitch frac flip2", base: "our", fam: "pitchFracFlip2"},
		{name: "our pitch frac zero", base: "our", fam: "pitchFracZero"},
	}

	tmp := t.TempDir()
	t.Logf("ffmpeg hybrid field-family report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-22s %6s %10s %10s %10s", "Stream", "shift", "RMS", "GlobalSNR", "SegSNR")
	for _, m := range modes {
		hybrid := makeHybridFrames(refFrames, ourFrames, m.base, m.fam)
		rawPath := filepath.Join(tmp, m.name+".g729")
		pcmPath := filepath.Join(tmp, m.name+".s16le")
		writePackedFrames(t, hybrid, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", m.name, len(decoded), totalSamples)
		}
		sh, g, s := bestAlignedSNR(src, decoded, 240)
		t.Logf("%-22s %6d %10.0f %10.2f %10.2f", m.name, sh, rmsAmp(decoded), g, s)
	}
}

// TestExternalFFmpegBlackboxHybridFrameBlocks_SPEECH localizes whether the
// encoder damage is concentrated in a small frame range or spread across the
// corpus. It keeps FFmpeg as a black-box decoder and swaps only non-LSP
// closed-loop fields (pitch + fixed codebook + gains) between our encoder and
// SPEECH.BIT in coarse blocks.
func TestExternalFFmpegBlackboxHybridFrameBlocks_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box block hybrid diagnostic")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	const (
		bytesPerInFrame  = 2 * FrameSamples
		bytesPerBitFrame = 164
		blockFrames      = 500
		family           = "pitch+fcb+gain"
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
	pstData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.PST"))
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	if pf := len(pstData) / bytesPerInFrame; pf < frames {
		frames = pf
	}
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])
	refFrames := readG192Frames(t, bitData[:frames*bytesPerBitFrame])
	ourFrames := encodeBitstreamFrames(t, src)

	tmp := t.TempDir()
	measure := func(name string, frames []bitstream.Frame) (int, float64, float64, float64) {
		t.Helper()
		rawPath := filepath.Join(tmp, strings.NewReplacer(" ", "_", "/", "_", "+", "_").Replace(name)+".g729")
		pcmPath := filepath.Join(tmp, strings.NewReplacer(" ", "_", "/", "_", "+", "_").Replace(name)+".s16le")
		writePackedFrames(t, frames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", name, len(decoded), totalSamples)
		}
		shift, global, seg := bestAlignedSNR(src, decoded, 240)
		return shift, rmsAmp(decoded), global, seg
	}

	t.Logf("ffmpeg hybrid block report — SPEECH corpus (%d frames, %d samples), family=%s, block=%d",
		frames, totalSamples, family, blockFrames)
	t.Logf("%-34s %10s %6s %10s %10s %10s", "Stream", "frames", "shift", "RMS", "GlobalSNR", "SegSNR")

	for _, m := range []struct {
		name   string
		frames []bitstream.Frame
	}{
		{name: "ref all", frames: refFrames},
		{name: "our all", frames: ourFrames},
		{name: "our + ref nonLSP all", frames: makeHybridFrames(refFrames, ourFrames, "our", family)},
	} {
		sh, rms, global, seg := measure(m.name, m.frames)
		t.Logf("%-34s %10s %6d %10.0f %10.2f %10.2f", m.name, "all", sh, rms, global, seg)
	}

	for start := 0; start < frames; start += blockFrames {
		end := start + blockFrames
		if end > frames {
			end = frames
		}
		label := itoa(start) + ".." + itoa(end-1)

		ourWithRefBlock := makeHybridFrameRange(ourFrames, refFrames, family, start, end)
		sh, rms, global, seg := measure("our + ref nonLSP "+label, ourWithRefBlock)
		t.Logf("%-34s %10s %6d %10.0f %10.2f %10.2f", "our + ref nonLSP block", label, sh, rms, global, seg)

		refWithOurBlock := makeHybridFrameRange(refFrames, ourFrames, family, start, end)
		sh, rms, global, seg = measure("ref + our nonLSP "+label, refWithOurBlock)
		t.Logf("%-34s %10s %6d %10.0f %10.2f %10.2f", "ref + our nonLSP block", label, sh, rms, global, seg)
	}

	for end := blockFrames; end <= frames; end += blockFrames {
		label := "0.." + itoa(end-1)
		ourWithRefPrefix := makeHybridFrameRange(ourFrames, refFrames, family, 0, end)
		sh, rms, global, seg := measure("our + ref nonLSP prefix "+label, ourWithRefPrefix)
		t.Logf("%-34s %10s %6d %10.0f %10.2f %10.2f", "our + ref nonLSP prefix", label, sh, rms, global, seg)

		refWithOurPrefix := makeHybridFrameRange(refFrames, ourFrames, family, 0, end)
		sh, rms, global, seg = measure("ref + our nonLSP prefix "+label, refWithOurPrefix)
		t.Logf("%-34s %10s %6d %10.0f %10.2f %10.2f", "ref + our nonLSP prefix", label, sh, rms, global, seg)
	}
}

// TestExternalFFmpegBlackboxOurTransformGrid_SPEECH enumerates a small grid
// of reversible bit-field interpretation variants over our encoder output.
// It is a clean-room diagnostic for ruling out remaining packing/sign/map
// mistakes; ffmpeg is still used only as an external decoder executable.
func TestExternalFFmpegBlackboxOurTransformGrid_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box transform-grid diagnostic")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	const bytesPerInFrame = 2 * FrameSamples
	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData, err := os.ReadFile(filepath.Join(vecDir, "SPEECH.IN"))
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}

	frames := len(inData) / bytesPerInFrame
	totalSamples := frames * FrameSamples
	src := s16leToSamples(inData[:totalSamples*2])
	ourFrames := encodeBitstreamFrames(t, src)

	signModes := []string{"", "signInvert", "signReverse", "signReverseInvert"}
	gainModes := []string{"", "gainInverse", "gaIdentity", "gaInverse", "gbIdentity", "gbInverse", "gainLowGp", "gainMid"}
	cModes := []string{"", "cFlipJx"}
	pitchModes := []string{"", "pitchFracFlip", "pitchFracZero"}

	tmp := t.TempDir()
	t.Logf("ffmpeg transform-grid report — our encoder SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-34s %6s %10s %10s %10s", "Transform", "shift", "RMS", "GlobalSNR", "SegSNR")
	for _, sm := range signModes {
		for _, gm := range gainModes {
			for _, cm := range cModes {
				for _, pm := range pitchModes {
					label := joinTransformLabel(sm, gm, cm, pm)
					candidate := cloneFrames(ourFrames)
					for i := range candidate {
						applyFrameTransform(&candidate[i], sm)
						applyFrameTransform(&candidate[i], gm)
						applyFrameTransform(&candidate[i], cm)
						applyFrameTransform(&candidate[i], pm)
					}
					rawPath := filepath.Join(tmp, label+".g729")
					pcmPath := filepath.Join(tmp, label+".s16le")
					writePackedFrames(t, candidate, rawPath)
					ffmpegDecodeRawG729(t, rawPath, pcmPath)
					decoded := s16leToSamples(readFile(t, pcmPath))
					if len(decoded) > totalSamples {
						decoded = decoded[:totalSamples]
					}
					if len(decoded) < totalSamples {
						t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", label, len(decoded), totalSamples)
					}
					sh, g, s := bestAlignedSNR(src, decoded, 240)
					t.Logf("%-34s %6d %10.0f %10.2f %10.2f", label, sh, rmsAmp(decoded), g, s)
				}
			}
		}
	}
}

func TestExternalFFmpegBlackboxOurFrameShift_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box frame-shift diagnostic")
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
	ourFrames := encodeBitstreamFrames(t, src)
	refFrames := readG192Frames(t, bitData[:frames*bytesPerBitFrame])

	type mode struct {
		name   string
		frames []bitstream.Frame
	}
	modes := []mode{
		{name: "our identity", frames: ourFrames},
		{name: "our fields previous", frames: shiftFrameSequence(ourFrames, -1)},
		{name: "our fields next", frames: shiftFrameSequence(ourFrames, +1)},
		{name: "our nonLSP previous", frames: shiftFrameFamily(ourFrames, -1, "pitch+fcb+gain")},
		{name: "our nonLSP next", frames: shiftFrameFamily(ourFrames, +1, "pitch+fcb+gain")},
		{name: "ref nonLSP previous", frames: shiftFrameFamily(refFrames, -1, "pitch+fcb+gain")},
		{name: "ref nonLSP next", frames: shiftFrameFamily(refFrames, +1, "pitch+fcb+gain")},
	}

	tmp := t.TempDir()
	t.Logf("ffmpeg frame-shift report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-24s %6s %10s %10s %10s", "Stream", "shift", "RMS", "GlobalSNR", "SegSNR")
	for _, m := range modes {
		base := strings.ReplaceAll(m.name, " ", "_")
		rawPath := filepath.Join(tmp, base+".g729")
		pcmPath := filepath.Join(tmp, base+".s16le")
		writePackedFrames(t, m.frames, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", m.name, len(decoded), totalSamples)
		}
		sh, g, s := bestAlignedSNR(src, decoded, 240)
		t.Logf("%-24s %6d %10.0f %10.2f %10.2f", m.name, sh, rmsAmp(decoded), g, s)
	}
}

// TestExternalFFmpegBlackboxForcedReferenceStages_SPEECH distinguishes
// output-field replacement from encoder-state replacement. In particular,
// forcing reference pitch before running the local FCB/gain search shows
// whether poor P1/P2 selection is the dominant cause of the bad external
// decode, rather than merely producing an inconsistent hybrid bitstream.
//
// Enable with G729_FFMPEG_BLACKBOX_QUALITY=1.
func TestExternalFFmpegBlackboxForcedReferenceStages_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box forced-stage diagnostic")
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
	refFrames := readG192Frames(t, bitData[:frames*bytesPerBitFrame])

	type mode struct {
		name string
		kind string
	}
	modes := []mode{
		{name: "ref all", kind: "refAll"},
		{name: "our all", kind: "ourAll"},
		{name: "force ref LSP normal", kind: "refLSPNormal"},
		{name: "force ref pitch commit", kind: "refPitch"},
		{name: "force ref pitch+code own gain", kind: "refPitchCodeOwnGain"},
		{name: "force ref pitch+code sign inv", kind: "refPitchCodeOwnGainSignInvert"},
		{name: "force ref pitch+code sparse", kind: "refPitchCodeOwnGainSparse"},
		{name: "force ref pitch+code beta 0.2", kind: "refPitchCodeOwnGainBeta02"},
		{name: "force ref pitch+code beta 0.8", kind: "refPitchCodeOwnGainBeta08"},
		{name: "force ref pitch+code x half", kind: "refPitchCodeOwnGainXHalf"},
		{name: "force ref pitch+code gain x1.5", kind: "refPitchCodeOwnGainX3_2"},
		{name: "force ref pitch+code gain x2", kind: "refPitchCodeOwnGainX2"},
		{name: "force ref pitch+code y half", kind: "refPitchCodeOwnGainYHalf"},
		{name: "force ref pitch+code y x2", kind: "refPitchCodeOwnGainY2"},
		{name: "force ref pitch+code z x2", kind: "refPitchCodeOwnGainZ2"},
		{name: "force ref pitch+code gpc half", kind: "refPitchCodeOwnGainGpcHalf"},
		{name: "force ref pitch+code gpc quarter", kind: "refPitchCodeOwnGainGpcQuarter"},
		{name: "force ref pitch+code gpc x2", kind: "refPitchCodeOwnGainGpc2"},
		{name: "force ref pitch+code gpc x4", kind: "refPitchCodeOwnGainGpc4"},
		{name: "force ref pitch+code z half", kind: "refPitchCodeOwnGainZHalf"},
		{name: "force ref pitch+code target q12", kind: "refPitchCodeOwnGainTargetQ12"},
		{name: "force ref pitch+code gain identity", kind: "refPitchCodeGainIdentity"},
		{name: "force ref pitch+code gain inverse", kind: "refPitchCodeGainInverse"},
		{name: "force ref LSP+pitch+code own gain", kind: "refLSPPitchCodeOwnGain"},
		{name: "force ref fields commit", kind: "refFields"},
		{name: "force ref LSP+fields", kind: "refLSPFields"},
	}

	tmp := t.TempDir()
	t.Logf("ffmpeg forced-reference-stage report — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-28s %6s %10s %10s %10s", "Stream", "shift", "RMS", "GlobalSNR", "SegSNR")
	for _, m := range modes {
		var framesOut []bitstream.Frame
		switch m.kind {
		case "refAll":
			framesOut = refFrames
		case "ourAll":
			framesOut = encodeBitstreamFrames(t, src)
		default:
			framesOut = encodeBitstreamFramesForcedReferenceStages(t, src, bitData[:frames*bytesPerBitFrame], m.kind)
		}
		rawPath := filepath.Join(tmp, m.name+".g729")
		pcmPath := filepath.Join(tmp, m.name+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", m.name, len(decoded), totalSamples)
		}
		sh, g, s := bestAlignedSNR(src, decoded, 240)
		t.Logf("%-28s %6d %10.0f %10.2f %10.2f", m.name, sh, rmsAmp(decoded), g, s)
	}
}

// TestExternalFFmpegBlackboxForcedReferenceGainSelection_SPEECH compares the
// local gain-search choice against the reference bitstream's gain fields while
// pitch and fixed-codebook fields are forced to the reference trajectory.
//
// Enable with G729_FFMPEG_BLACKBOX_QUALITY=1.
func TestExternalFFmpegBlackboxForcedReferenceGainSelection_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box gain-selection diagnostic")
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

	type mode struct {
		name     string
		forceLSP bool
		commit   string
	}
	modes := []mode{
		{name: "our LSP / own-gain commit", commit: "own"},
		{name: "ref LSP / own-gain commit", forceLSP: true, commit: "own"},
		{name: "our LSP / ref-gain commit", commit: "ref"},
		{name: "ref LSP / ref-gain commit", forceLSP: true, commit: "ref"},
	}

	t.Logf("forced reference pitch+code gain-selection report — SPEECH corpus (%d frames, %d subframes)", frames, frames*2)
	t.Logf("%-28s %6s %9s %9s %9s %10s %10s %10s %10s %9s %9s",
		"Mode", "N", "GA eq", "GB eq", "both eq", "own<ref E", "ref<own E", "own gp", "ref gp", "own gc", "ref gc")
	for _, m := range modes {
		stats := collectForcedReferenceGainSelection(t, src, bitData[:frames*bytesPerBitFrame], m.forceLSP, m.commit)
		t.Logf("%-28s %6d %8.2f%% %8.2f%% %8.2f%% %9.2f%% %9.2f%% %10.0f %10.0f %9.0f %9.0f",
			m.name,
			stats.count,
			percent(stats.sameGA, stats.count),
			percent(stats.sameGB, stats.count),
			percent(stats.sameBoth, stats.count),
			percent(stats.ownLowerCost, stats.count),
			percent(stats.refLowerCost, stats.count),
			meanInt64(stats.ownGpSum, stats.count),
			meanInt64(stats.refGpSum, stats.count),
			meanInt64(stats.ownGcSum, stats.count),
			meanInt64(stats.refGcSum, stats.count),
		)
		t.Logf("%-28s own<ref: gp %.2f%% gamma %.2f%% gc %.2f%% ; tame %.2f%%",
			m.name,
			percent(stats.ownLowerGp, stats.count),
			percent(stats.ownLowerGamma, stats.count),
			percent(stats.ownLowerGc, stats.count),
			percent(stats.tamed, stats.count),
		)
		for i, ex := range stats.examples {
			t.Logf("%s mismatch[%d]: frame=%d sub=%d own=(GA=%d GB=%d gp=%d gamma=%d gc=%d E=%d) ref=(GA=%d GB=%d gp=%d gamma=%d gc=%d E=%d) gpcPred=%d",
				m.name, i, ex.frame, ex.sub,
				ex.ownGA, ex.ownGB, ex.ownGpQ14, ex.ownGammaQ13, ex.ownGcQ12, ex.ownCost,
				ex.refGA, ex.refGB, ex.refGpQ14, ex.refGammaQ13, ex.refGcQ12, ex.refCost,
				ex.gpcPredQ12,
			)
		}
	}
}

func TestExternalFFmpegBlackboxReferenceGainCostSurfaceAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run reference gain-cost surface audit")
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
	src := s16leToSamples(inData[:frames*bytesPerInFrame])

	modes := []gainCostSurfaceMode{
		{name: "residual x1/y1/z1", xNum: 1, xDen: 1, yNum: 1, yDen: 1, zNum: 1, zDen: 1},
		{name: "residual x1/2", xNum: 1, xDen: 2, yNum: 1, yDen: 1, zNum: 1, zDen: 1},
		{name: "residual x2", xNum: 2, xDen: 1, yNum: 1, yDen: 1, zNum: 1, zDen: 1},
		{name: "residual y1/2", xNum: 1, xDen: 1, yNum: 1, yDen: 2, zNum: 1, zDen: 1},
		{name: "residual y2", xNum: 1, xDen: 1, yNum: 2, yDen: 1, zNum: 1, zDen: 1},
		{name: "residual z1/2", xNum: 1, xDen: 1, yNum: 1, yDen: 1, zNum: 1, zDen: 2},
		{name: "residual z2", xNum: 1, xDen: 1, yNum: 1, yDen: 1, zNum: 2, zDen: 1},
		{name: "residual x1/2 y2", xNum: 1, xDen: 2, yNum: 2, yDen: 1, zNum: 1, zDen: 1},
		{name: "residual x1/2 z2", xNum: 1, xDen: 2, yNum: 1, yDen: 1, zNum: 2, zDen: 1},
		{name: "target q12", target: "q12", xNum: 1, xDen: 1, yNum: 1, yDen: 1, zNum: 1, zDen: 1},
		{name: "target zero mem", target: "zeroMem", xNum: 1, xDen: 1, yNum: 1, yDen: 1, zNum: 1, zDen: 1},
	}
	for i := range modes {
		normalizeGainSurfaceMode(&modes[i])
	}
	stats := make([]gainCostSurfaceStats, len(modes))

	enc := NewEncoder()
	var refLSPDec lsp.Decoder
	for off := 0; off+FrameSamples <= len(src); off += FrameSamples {
		frameIndex := off / FrameSamples
		bitFrame := bitData[frameIndex*bytesPerBitFrame : (frameIndex+1)*bytesPerBitFrame]
		if _, err := enc.lpcStep(src[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		l0, l1, l2, l3 := extractLSPFieldsFromG192(bitFrame)
		enc.l0, enc.l1, enc.l2, enc.l3 = uint16(l0), uint16(l1), uint16(l2), uint16(l3)
		enc.aHatSF1, enc.aHatSF2 = refLSPDec.Decode(lsp.Indices{L0: l0, L1: l1, L2: l2, L3: l3})
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		observeReferenceGainCostSurface(t, enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, refGA1, refGB1, modes, stats)
		observeReferenceGainCostSurface(t, enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, refGA2, refGB2, modes, stats)
	}

	t.Logf("reference gain-cost surface audit — SPEECH corpus (%d frames, %d subframes), ref LSP/pitch/code/gain committed", frames, frames*2)
	t.Logf("%-24s %7s %7s %7s %8s %8s %8s %8s", "mode", "GA eq", "GB eq", "both", "refBest", "ref<=4", "rank", "best/ref")
	for i, mode := range modes {
		s := stats[i]
		t.Logf("%-24s %6.2f%% %6.2f%% %6.2f%% %7.2f%% %7.2f%% %8.2f %8.3f",
			mode.name,
			percent(s.sameGA, s.total),
			percent(s.sameGB, s.total),
			percent(s.sameBoth, s.total),
			percent(s.refBestOrTie, s.total),
			percent(s.refRankLE4, s.total),
			meanInt64(s.refRankSum, s.total),
			meanRatio(s.bestCostSum, s.refCostSum))
	}
}

func TestExternalFFmpegBlackboxGainEnergyAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run gain-energy diagnostic")
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
	refFrames := readG192Frames(t, bitData[:frames*bytesPerBitFrame])
	ourFrames := encodeBitstreamFrames(t, src)

	refStats := collectGainEnergyStats(refFrames)
	ourStats := collectGainEnergyStats(ourFrames)

	t.Logf("gain fixed-codebook energy audit — SPEECH corpus (%d frames, %d subframes)", frames, frames*2)
	t.Logf("%-10s %7s %10s %10s %10s %10s %10s",
		"stream", "N", "sat>i32", "maxE/i32", "meanE/i32", "meanNZ", "maxNZ")
	t.Logf("%-10s %7d %9.2f%% %10.2f %10.2f %10.2f %10d",
		"ref", refStats.count, percent(refStats.saturated, refStats.count),
		float64(refStats.maxEnergy)/float64(maxInt32ForEnergyAudit),
		float64(refStats.energySum)/float64(refStats.count)/float64(maxInt32ForEnergyAudit),
		float64(refStats.nonZeroSum)/float64(refStats.count), refStats.maxNonZero)
	t.Logf("%-10s %7d %9.2f%% %10.2f %10.2f %10.2f %10d",
		"our", ourStats.count, percent(ourStats.saturated, ourStats.count),
		float64(ourStats.maxEnergy)/float64(maxInt32ForEnergyAudit),
		float64(ourStats.energySum)/float64(ourStats.count)/float64(maxInt32ForEnergyAudit),
		float64(ourStats.nonZeroSum)/float64(ourStats.count), ourStats.maxNonZero)
}

func TestExternalFFmpegBlackboxPitchSelectionAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run pitch-selection diagnostic")
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

	var enc Encoder
	lsp.InitFreqPrev(&enc.freqPrev)
	lsp.InitLSPOld(&enc.lspOld)
	for i := range enc.pastQuaEn {
		enc.pastQuaEn[i] = gain.PastErrorsDefault
	}

	var stats pitchSelectionAuditStats
	for off := 0; off+FrameSamples <= totalSamples; off += FrameSamples {
		frameIndex := off / FrameSamples
		bitFrame := bitData[frameIndex*bytesPerBitFrame : (frameIndex+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		if _, err := enc.lpcStep(src[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		_ = enc.openloopStep()

		observePitchSelection(&enc, frameIndex, 0, int16(refInt1), int8(refFrac1), &stats)
		enc.closedloopStep(0)
		observePitchSelection(&enc, frameIndex, 1, int16(refInt2), int8(refFrac2), &stats)
		enc.closedloopStep(1)
	}

	t.Logf("pitch selection audit — SPEECH corpus (%d frames, %d subframes)", frames, frames*2)
	t.Logf("total=%d intEq=%.2f%% fracEqWhenIntEq=%.2f%% refIntInWindow=%.2f%% refRNBetterWhenInWindow=%.2f%% refFracBestWhenIntEq=%.2f%%",
		stats.total,
		percent(stats.intEqual, stats.total),
		percent(stats.fracEqualWhenIntEqual, stats.intEqual),
		percent(stats.refIntInWindow, stats.total),
		percent(stats.refRNBetter, stats.refIntInWindow),
		percent(stats.refFracBest, stats.intEqual))
	t.Logf("oracle-centred local RN: intEq=%.2f%% fracEqWhenIntEq=%.2f%%",
		percent(stats.refCenteredIntEqual, stats.total),
		percent(stats.refCenteredFracEqual, stats.refCenteredIntEqual))
	t.Logf("by sub0: total=%d intEq=%.2f%% refIntInWindow=%.2f%% refRNBetter=%.2f%%",
		stats.bySub[0].total,
		percent(stats.bySub[0].intEqual, stats.bySub[0].total),
		percent(stats.bySub[0].refIntInWindow, stats.bySub[0].total),
		percent(stats.bySub[0].refRNBetter, stats.bySub[0].refIntInWindow))
	t.Logf("by sub1: total=%d intEq=%.2f%% refIntInWindow=%.2f%% refRNBetter=%.2f%%",
		stats.bySub[1].total,
		percent(stats.bySub[1].intEqual, stats.bySub[1].total),
		percent(stats.bySub[1].refIntInWindow, stats.bySub[1].total),
		percent(stats.bySub[1].refRNBetter, stats.bySub[1].refIntInWindow))
	for i, ex := range stats.examples {
		t.Logf("pitch mismatch[%d]: frame=%d sub=%d centre=%d local=(%d,%d rn=%d) ref=(%d,%d rn=%d) inWindow=%t",
			i, ex.frame, ex.sub, ex.centre,
			ex.localInt, ex.localFrac, ex.localRN,
			ex.refInt, ex.refFrac, ex.refRN, ex.refInWindow)
	}
}

func writeG192AsRawG729(t *testing.T, g192 []byte, path string) {
	t.Helper()
	var raw bytes.Buffer
	r := bytes.NewReader(g192)
	var packed [FrameBytes]byte
	for {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			if raw.Len() > 0 {
				break
			}
			t.Fatalf("ReadG192Frame: %v", err)
		}
		raw.Write(packed[:])
	}
	if err := os.WriteFile(path, raw.Bytes(), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeOurEncodedRawG729(t *testing.T, samples []int16, path string) {
	t.Helper()
	enc := NewEncoder()
	writeRawG729WithEncoder(t, samples, path, enc)
}

func writeOurEncodedRawG729WithProfile(t *testing.T, samples []int16, path string, profile EncoderProfile) {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	writeRawG729WithEncoder(t, samples, path, enc)
}

func writeOurEncodedRawG729WithTuning(t *testing.T, samples []int16, path string, profile EncoderProfile, tuning encoderQualityTuning) {
	t.Helper()
	enc := NewEncoderWithProfile(profile)
	enc.qualityTuning = tuning
	writeRawG729WithEncoder(t, samples, path, enc)
}

func writeRawG729WithEncoder(t *testing.T, samples []int16, path string, enc *Encoder) {
	t.Helper()
	raw := make([]byte, 0, len(samples)/FrameSamples*FrameBytes)
	var packed [FrameBytes]byte
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if err := enc.EncodeFrame(samples[off:off+FrameSamples], packed[:]); err != nil {
			t.Fatalf("EncodeFrame frame %d: %v", off/FrameSamples, err)
		}
		raw = append(raw, packed[:]...)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeBCGEncodedRawG729(t *testing.T, samples []int16, path string) {
	t.Helper()
	if len(samples)%FrameSamples != 0 {
		t.Fatalf("bcg729 black-box encode input has %d samples, want multiple of %d", len(samples), FrameSamples)
	}
	bin := filepath.Join("third-party", "bcg729-blackbox", "bcg729_encode")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("bcg729 black-box executable unavailable: %v", err)
	}

	pcm := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(pcm[2*i:2*i+2], uint16(s))
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
	want := len(samples) / FrameSamples * FrameBytes
	if len(out) != want {
		t.Fatalf("bcg729 black-box encoded %d bytes, want %d", len(out), want)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readRawG729Frames(t *testing.T, raw []byte) []bitstream.Frame {
	t.Helper()
	if len(raw)%FrameBytes != 0 {
		t.Fatalf("raw G.729 length %d not divisible by %d", len(raw), FrameBytes)
	}
	frames := make([]bitstream.Frame, 0, len(raw)/FrameBytes)
	for off := 0; off < len(raw); off += FrameBytes {
		var f bitstream.Frame
		if err := bitstream.Unpack(raw[off:off+FrameBytes], &f); err != nil {
			t.Fatalf("Unpack raw G.729 frame %d: %v", off/FrameBytes, err)
		}
		frames = append(frames, f)
	}
	return frames
}

func decodeRawG729WithLocal(t *testing.T, raw []byte) []int16 {
	t.Helper()
	if len(raw)%FrameBytes != 0 {
		t.Fatalf("raw G.729 length %d not divisible by %d", len(raw), FrameBytes)
	}
	dec := NewDecoder()
	out := make([]int16, (len(raw)/FrameBytes)*FrameSamples)
	for off := 0; off < len(raw); off += FrameBytes {
		frame := off / FrameBytes
		if err := dec.DecodeFrame(raw[off:off+FrameBytes], out[frame*FrameSamples:(frame+1)*FrameSamples]); err != nil {
			t.Fatalf("local DecodeFrame frame %d: %v", frame, err)
		}
	}
	return out
}

func decodeRawG729WithLocalEnhanced(t *testing.T, raw []byte) []int16 {
	t.Helper()
	if len(raw)%FrameBytes != 0 {
		t.Fatalf("raw G.729 length %d not divisible by %d", len(raw), FrameBytes)
	}
	dec := NewDecoder()
	out := make([]int16, (len(raw)/FrameBytes)*FrameSamples)
	for off := 0; off < len(raw); off += FrameBytes {
		frame := off / FrameBytes
		if err := dec.DecodeFrameEnhanced(raw[off:off+FrameBytes], out[frame*FrameSamples:(frame+1)*FrameSamples]); err != nil {
			t.Fatalf("local DecodeFrameEnhanced frame %d: %v", frame, err)
		}
	}
	return out
}

func decodeRawG729WithLocalPostfilterBlend(t *testing.T, raw []byte, synthNum, den int) []int16 {
	t.Helper()
	if len(raw)%FrameBytes != 0 {
		t.Fatalf("raw G.729 length %d not divisible by %d", len(raw), FrameBytes)
	}
	dec := NewDecoder()
	out := make([]int16, (len(raw)/FrameBytes)*FrameSamples)
	for off := 0; off < len(raw); off += FrameBytes {
		frame := off / FrameBytes
		if err := dec.DecodeFramePostfilterBlend(raw[off:off+FrameBytes], out[frame*FrameSamples:(frame+1)*FrameSamples], synthNum, den); err != nil {
			t.Fatalf("local DecodeFramePostfilterBlend frame %d: %v", frame, err)
		}
	}
	return out
}

func scaleSamplesForDiagnostic(in []int16, num, den int) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		x := int(v) * num
		if den != 0 {
			x /= den
		}
		if x > 32767 {
			x = 32767
		} else if x < -32768 {
			x = -32768
		}
		out[i] = int16(x)
	}
	return out
}

func matchFrameRMSForDiagnostic(in, target []int16) []int16 {
	n := len(in)
	if len(target) < n {
		n = len(target)
	}
	out := make([]int16, n)
	for off := 0; off+FrameSamples <= n; off += FrameSamples {
		srcFrame := in[off : off+FrameSamples]
		targetFrame := target[off : off+FrameSamples]
		srcRMS := rmsAmp(srcFrame)
		targetRMS := rmsAmp(targetFrame)
		if srcRMS < 1 || targetRMS < 1 {
			copy(out[off:off+FrameSamples], srcFrame)
			continue
		}
		gain := targetRMS / srcRMS
		for i, v := range srcFrame {
			x := math.Round(float64(v) * gain)
			if x > 32767 {
				x = 32767
			} else if x < -32768 {
				x = -32768
			}
			out[off+i] = int16(x)
		}
	}
	return out
}

type localDecoderFrameDelta struct {
	frame       int
	ffRMS       float64
	localRMS    float64
	ratio       float64
	snrVsFF     float64
	corrVsFF    float64
	ffSNRSrc    float64
	localSNRSrc float64
}

func logWorstLocalDecoderFrames(t *testing.T, src, ff, local []int16, limit int) {
	t.Helper()
	frames := len(src) / FrameSamples
	if ffFrames := len(ff) / FrameSamples; ffFrames < frames {
		frames = ffFrames
	}
	if localFrames := len(local) / FrameSamples; localFrames < frames {
		frames = localFrames
	}
	rows := make([]localDecoderFrameDelta, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * FrameSamples
		srcFrame := src[off : off+FrameSamples]
		ffFrame := ff[off : off+FrameSamples]
		localFrame := local[off : off+FrameSamples]
		ffRMS := rmsAmp(ffFrame)
		localRMS := rmsAmp(localFrame)
		ratio := 0.0
		if ffRMS > 0 {
			ratio = localRMS / ffRMS
		}
		rows = append(rows, localDecoderFrameDelta{
			frame:       frame,
			ffRMS:       ffRMS,
			localRMS:    localRMS,
			ratio:       ratio,
			snrVsFF:     frameSNRDB(ffFrame, localFrame),
			corrVsFF:    frameCorr(ffFrame, localFrame),
			ffSNRSrc:    frameSNRDB(srcFrame, ffFrame),
			localSNRSrc: frameSNRDB(srcFrame, localFrame),
		})
	}
	logWorstLocalDecoderFrameRows(t, rows, limit, "all frames")

	active := make([]localDecoderFrameDelta, 0, len(rows))
	for _, r := range rows {
		if r.ffRMS >= 500 {
			active = append(active, r)
		}
	}
	logWorstLocalDecoderFrameRows(t, active, limit, "active frames with ffRMS >= 500")
}

func logWorstLocalDecoderFrameRows(t *testing.T, rows []localDecoderFrameDelta, limit int, label string) {
	t.Helper()
	sort.Slice(rows, func(i, j int) bool {
		li, lj := rows[i].snrVsFF, rows[j].snrVsFF
		if math.IsNaN(li) {
			return false
		}
		if math.IsNaN(lj) {
			return true
		}
		if li == lj {
			return rows[i].ffRMS > rows[j].ffRMS
		}
		return li < lj
	})
	if limit > len(rows) {
		limit = len(rows)
	}
	t.Logf("")
	t.Logf("worst local decoder frames vs FFmpeg black-box for the same local-encoder raw G.729 payload (%s)", label)
	t.Logf("%5s %9s %9s %8s %9s %8s %10s %10s",
		"frame", "ffRMS", "localRMS", "ratio", "snrVsFF", "corr", "ffSNRSrc", "localSNR")
	for i := 0; i < limit; i++ {
		r := rows[i]
		t.Logf("%5d %9.1f %9.1f %8.3f %9.2f %8.3f %10.2f %10.2f",
			r.frame, r.ffRMS, r.localRMS, r.ratio, r.snrVsFF,
			r.corrVsFF, r.ffSNRSrc, r.localSNRSrc)
	}
}

func frameSNRDB(ref, test []int16) float64 {
	if len(ref) != len(test) || len(ref) == 0 {
		return math.NaN()
	}
	var sigE, errE float64
	for i := range ref {
		s := float64(ref[i])
		e := s - float64(test[i])
		sigE += s * s
		errE += e * e
	}
	if sigE < 1 {
		return math.NaN()
	}
	if errE < 1 {
		return math.Inf(+1)
	}
	return 10 * math.Log10(sigE/errE)
}

func frameCorr(a, b []int16) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.NaN()
	}
	var meanA, meanB float64
	for i := range a {
		meanA += float64(a[i])
		meanB += float64(b[i])
	}
	meanA /= float64(len(a))
	meanB /= float64(len(b))
	var num, denA, denB float64
	for i := range a {
		da := float64(a[i]) - meanA
		db := float64(b[i]) - meanB
		num += da * db
		denA += da * da
		denB += db * db
	}
	den := math.Sqrt(denA * denB)
	if den <= 0 {
		return math.NaN()
	}
	return num / den
}

func readG192Frames(t *testing.T, g192 []byte) []bitstream.Frame {
	t.Helper()
	r := bytes.NewReader(g192)
	var packed [FrameBytes]byte
	var frames []bitstream.Frame
	for {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			if len(frames) > 0 {
				break
			}
			t.Fatalf("ReadG192Frame: %v", err)
		}
		var f bitstream.Frame
		if err := bitstream.Unpack(packed[:], &f); err != nil {
			t.Fatalf("Unpack G.192 frame %d: %v", len(frames), err)
		}
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFrames(t *testing.T, samples []int16) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
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

func encodeBitstreamFramesForcedReferenceStages(t *testing.T, samples []int16, g192 []byte, mode string) []bitstream.Frame {
	t.Helper()
	const bytesPerBitFrame = 164
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	var refLSPDec lsp.Decoder
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		bitFrame := g192[frameIndex*bytesPerBitFrame : (frameIndex+1)*bytesPerBitFrame]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		if mode == "refLSPFields" || mode == "refLSPPitchCodeOwnGain" || mode == "refLSPNormal" {
			l0, l1, l2, l3 := extractLSPFieldsFromG192(bitFrame)
			enc.l0, enc.l1, enc.l2, enc.l3 = uint16(l0), uint16(l1), uint16(l2), uint16(l3)
			a1, a2 := refLSPDec.Decode(lsp.Indices{L0: l0, L1: l1, L2: l2, L3: l3})
			enc.aHatSF1 = a1
			enc.aHatSF2 = a2
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		switch mode {
		case "refPitch":
			forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
			forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
		case "refLSPNormal":
			enc.closedloopStep(0)
			enc.closedloopStep(1)
		case "refPitchCodeOwnGain", "refLSPPitchCodeOwnGain":
			forceReferenceCodeOwnGainStep(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack")
			forceReferenceCodeOwnGainStep(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack")
		case "refPitchCodeOwnGainSignInvert":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "codeSignInvert")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "codeSignInvert")
		case "refPitchCodeOwnGainSparse":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "codeSparse")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "codeSparse")
		case "refPitchCodeOwnGainBeta02":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "codeBeta02")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "codeBeta02")
		case "refPitchCodeOwnGainBeta08":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "codeBeta08")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "codeBeta08")
		case "refPitchCodeOwnGainXHalf":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "xHalf")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "xHalf")
		case "refPitchCodeOwnGainX3_2":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "x3_2")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "x3_2")
		case "refPitchCodeOwnGainX2":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "x2")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "x2")
		case "refPitchCodeOwnGainYHalf":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "yHalf")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "yHalf")
		case "refPitchCodeOwnGainY2":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "y2")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "y2")
		case "refPitchCodeOwnGainZ2":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "z2")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "z2")
		case "refPitchCodeOwnGainGpcHalf":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "gpcHalf")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "gpcHalf")
		case "refPitchCodeOwnGainGpcQuarter":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "gpcQuarter")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "gpcQuarter")
		case "refPitchCodeOwnGainGpc2":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "gpc2")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "gpc2")
		case "refPitchCodeOwnGainGpc4":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "gpc4")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "gpc4")
		case "refPitchCodeOwnGainZHalf":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "zHalf")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "zHalf")
		case "refPitchCodeOwnGainTargetQ12":
			forceReferenceCodeOwnGainStepVariant(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "pack", "targetQ12")
			forceReferenceCodeOwnGainStepVariant(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "pack", "targetQ12")
		case "refPitchCodeGainIdentity":
			forceReferenceCodeOwnGainStep(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "identity")
			forceReferenceCodeOwnGainStep(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "identity")
		case "refPitchCodeGainInverse":
			forceReferenceCodeOwnGainStep(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, "inverse")
			forceReferenceCodeOwnGainStep(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, "inverse")
		case "refFields", "refLSPFields":
			forceReferenceFieldStep(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, refGA1, refGB1)
			forceReferenceFieldStep(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, refGA2, refGB2)
		default:
			t.Fatalf("unknown forced reference mode %q", mode)
		}

		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func forceReferenceCodeOwnGainStep(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16, refC, refS uint16, gainMap string) {
	forceReferenceCodeOwnGainStepVariant(e, sub, intLag, frac, pitchCode, refC, refS, gainMap, "")
}

func forceReferenceCodeOwnGainStepVariant(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16, refC, refS uint16, gainMap, searchVariant string) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	if searchVariant == "targetQ12" {
		targetSignalQ12ForDiagnostic(aHat, &r, &e.swMemErr, &x)
	}
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	clpitch.AdaptiveVector(exc, intLag, frac, &v)
	clpitch.GpAndY(&x, &v, &h, &y)

	codeSigns := uint8(refS)
	if searchVariant == "codeSignInvert" {
		codeSigns ^= 0x0f
	}
	codeBetaQ14 := e.prevGpQ14
	switch searchVariant {
	case "codeSparse":
		codeBetaQ14 = 0
	case "codeBeta02":
		codeBetaQ14 = 3277
	case "codeBeta08":
		codeBetaQ14 = 13107
	}
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: codeSigns}, int(intLag), codeBetaQ14, &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	xSearch, ySearch, zSearch := x, y, z
	gpcSearchQ12 := gpcPredQ12
	switch searchVariant {
	case "":
	case "targetQ12", "codeSignInvert", "codeSparse", "codeBeta02", "codeBeta08":
	case "xHalf":
		scaleVectorInt16(&xSearch, 1, 2)
	case "x3_2":
		scaleVectorInt16(&xSearch, 3, 2)
	case "x2":
		scaleVectorInt16(&xSearch, 2, 1)
	case "yHalf":
		scaleVectorInt16(&ySearch, 1, 2)
	case "y2":
		scaleVectorInt16(&ySearch, 2, 1)
	case "gpcHalf":
		gpcSearchQ12 >>= 1
	case "gpcQuarter":
		gpcSearchQ12 >>= 2
	case "gpc2":
		gpcSearchQ12 = scaleInt32ForDiagnostic(gpcSearchQ12, 2, 1)
	case "gpc4":
		gpcSearchQ12 = scaleInt32ForDiagnostic(gpcSearchQ12, 4, 1)
	case "zHalf":
		scaleVectorInt16(&zSearch, 1, 2)
	case "z2":
		scaleVectorInt16(&zSearch, 2, 1)
	default:
		panic("unknown gain search variant")
	}
	gaPhys, gbPhys, gpQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &zSearch, gpcSearchQ12)
	gpQ14 = gainquant.Tame(gpQ14, &e.oldExc)
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	switch gainMap {
	case "pack":
	case "identity":
		gaBits, gbBits = gaPhys, gbPhys
	case "inverse":
		gaBits, gbBits = tables.GainImap1[gaPhys], tables.GainImap2[gbPhys]
	default:
		panic("unknown gain map mode")
	}
	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(pitchCode)
		e.p0 = clpitch.EncodeP0(e.p1)
		e.c1 = refC
		e.s1 = uint8(refS)
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = uint8(pitchCode)
		e.c2 = refC
		e.s2 = uint8(refS)
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

type forcedGainSelectionStats struct {
	count int

	sameGA   int
	sameGB   int
	sameBoth int

	ownLowerCost int
	refLowerCost int

	ownLowerGp    int
	ownLowerGamma int
	ownLowerGc    int
	tamed         int

	ownGpSum int64
	refGpSum int64
	ownGcSum int64
	refGcSum int64

	examples []forcedGainSelectionExample
}

type forcedGainSelectionExample struct {
	frame int
	sub   int

	ownGA       uint8
	ownGB       uint8
	ownGpQ14    int16
	ownGammaQ13 int32
	ownGcQ12    int32
	ownCost     int64

	refGA       uint8
	refGB       uint8
	refGpQ14    int16
	refGammaQ13 int32
	refGcQ12    int32
	refCost     int64

	gpcPredQ12 int32
}

type gainCostSurfaceMode struct {
	name   string
	target string

	xNum, xDen int32
	yNum, yDen int32
	zNum, zDen int32
}

type gainCostSurfaceStats struct {
	total int

	sameGA   int
	sameGB   int
	sameBoth int

	refBestOrTie int
	refRankLE4   int
	refRankSum   int64

	bestCostSum int64
	refCostSum  int64
}

func collectForcedReferenceGainSelection(t *testing.T, samples []int16, g192 []byte, forceLSP bool, commit string) forcedGainSelectionStats {
	t.Helper()
	const bytesPerBitFrame = 164
	enc := NewEncoder()
	var refLSPDec lsp.Decoder
	var stats forcedGainSelectionStats
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		frameIndex := off / FrameSamples
		bitFrame := g192[frameIndex*bytesPerBitFrame : (frameIndex+1)*bytesPerBitFrame]
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frameIndex, err)
		}
		if forceLSP {
			l0, l1, l2, l3 := extractLSPFieldsFromG192(bitFrame)
			enc.l0, enc.l1, enc.l2, enc.l3 = uint16(l0), uint16(l1), uint16(l2), uint16(l3)
			a1, a2 := refLSPDec.Decode(lsp.Indices{L0: l0, L1: l1, L2: l2, L3: l3})
			enc.aHatSF1 = a1
			enc.aHatSF2 = a2
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		observeReferenceCodeGainSelection(t, enc, frameIndex, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, refGA1, refGB1, commit, &stats)
		observeReferenceCodeGainSelection(t, enc, frameIndex, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, refGA2, refGB2, commit, &stats)
	}
	return stats
}

func observeReferenceCodeGainSelection(
	t *testing.T,
	e *Encoder,
	frameIndex, sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC, refS, refGA, refGB uint16,
	commit string,
	stats *forcedGainSelectionStats,
) {
	t.Helper()
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	clpitch.AdaptiveVector(exc, intLag, frac, &v)
	clpitch.GpAndY(&x, &v, &h, &y)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: uint8(refS)}, int(intLag), e.prevGpQ14, &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	ownGAPhys, ownGBPhys, _, ownGammaQ13 := gainquant.SearchConjugate(&x, &y, &z, gpcPredQ12)
	ownBitsGA, ownBitsGB := gainquant.PackGains(ownGAPhys, ownGBPhys)
	ownGpTxQ14, ownGcMantQ14, ownGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, ownGAPhys, ownGBPhys)
	ownGpCommitQ14 := gainquant.Tame(ownGpTxQ14, &e.oldExc)

	refBitsGA := uint8(refGA & 7)
	refBitsGB := uint8(refGB & 15)
	refGAPhys := tables.GainImap1[refBitsGA]
	refGBPhys := tables.GainImap2[refBitsGB]
	refGpQ14, refGcMantQ14, refGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, refGAPhys, refGBPhys)
	refGammaQ13 := gainGammaQ13(refGAPhys, refGBPhys)
	refGpCommitQ14 := gainquant.Tame(refGpQ14, &e.oldExc)

	ownGcQ12 := mantExpToQ12(ownGcMantQ14, ownGcExp)
	refGcQ12 := mantExpToQ12(refGcMantQ14, refGcExp)
	ownCost := gainResidualEnergyQ0(&x, &y, &z, ownGpTxQ14, ownGcMantQ14, ownGcExp)
	refCost := gainResidualEnergyQ0(&x, &y, &z, refGpQ14, refGcMantQ14, refGcExp)

	stats.count++
	stats.ownGpSum += int64(ownGpTxQ14)
	stats.refGpSum += int64(refGpQ14)
	stats.ownGcSum += int64(ownGcQ12)
	stats.refGcSum += int64(refGcQ12)
	if ownBitsGA == refBitsGA {
		stats.sameGA++
	}
	if ownBitsGB == refBitsGB {
		stats.sameGB++
	}
	if ownBitsGA == refBitsGA && ownBitsGB == refBitsGB {
		stats.sameBoth++
	}
	if ownCost < refCost {
		stats.ownLowerCost++
	} else if refCost < ownCost {
		stats.refLowerCost++
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
	if ownGpCommitQ14 != ownGpTxQ14 {
		stats.tamed++
	}
	if (ownBitsGA != refBitsGA || ownBitsGB != refBitsGB) && len(stats.examples) < 4 {
		stats.examples = append(stats.examples, forcedGainSelectionExample{
			frame: frameIndex,
			sub:   sub,

			ownGA:       ownBitsGA,
			ownGB:       ownBitsGB,
			ownGpQ14:    ownGpTxQ14,
			ownGammaQ13: ownGammaQ13,
			ownGcQ12:    ownGcQ12,
			ownCost:     ownCost,

			refGA:       refBitsGA,
			refGB:       refBitsGB,
			refGpQ14:    refGpQ14,
			refGammaQ13: refGammaQ13,
			refGcQ12:    refGcQ12,
			refCost:     refCost,

			gpcPredQ12: gpcPredQ12,
		})
	}

	var gaBits, gbBits uint8
	var gpCommitQ14, gcMantQ14 int16
	var gcExp int8
	var gammaQ13 int32
	var tamed bool
	switch commit {
	case "own":
		gaBits, gbBits = ownBitsGA, ownBitsGB
		gpCommitQ14, gcMantQ14, gcExp = ownGpCommitQ14, ownGcMantQ14, ownGcExp
		gammaQ13 = ownGammaQ13
		tamed = ownGpCommitQ14 != ownGpTxQ14
	case "ref":
		gaBits, gbBits = refBitsGA, refBitsGB
		gpCommitQ14, gcMantQ14, gcExp = refGpCommitQ14, refGcMantQ14, refGcExp
		gammaQ13 = refGammaQ13
		tamed = refGpCommitQ14 != refGpQ14
	default:
		t.Fatalf("unknown gain-selection commit mode %q", commit)
	}

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(pitchCode)
		e.p0 = clpitch.EncodeP0(e.p1)
		e.c1 = refC
		e.s1 = uint8(refS)
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = uint8(pitchCode)
		e.c2 = refC
		e.s2 = uint8(refS)
		e.ga2 = gaBits
		e.gb2 = gbBits
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
	e.prevTaming = tamed
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func observeReferenceGainCostSurface(
	t *testing.T,
	e *Encoder,
	sub int,
	intLag int16,
	frac int8,
	pitchCode uint16,
	refC, refS, refGA, refGB uint16,
	modes []gainCostSurfaceMode,
	stats []gainCostSurfaceStats,
) {
	t.Helper()
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	clpitch.AdaptiveVector(exc, intLag, frac, &v)
	clpitch.GpAndY(&x, &v, &h, &y)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: uint8(refS)}, int(intLag), e.prevGpQ14, &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	refBitsGA := uint8(refGA & 7)
	refBitsGB := uint8(refGB & 15)
	refGAPhys := tables.GainImap1[refBitsGA]
	refGBPhys := tables.GainImap2[refBitsGB]
	refGpQ14, refGcMantQ14, refGcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, refGAPhys, refGBPhys)
	refGammaQ13 := gainGammaQ13(refGAPhys, refGBPhys)

	for i, mode := range modes {
		xCost, yCost, zCost := x, y, z
		switch mode.target {
		case "":
		case "q12":
			targetSignalQ12ForDiagnostic(aHat, &r, &e.swMemErr, &xCost)
		case "zeroMem":
			var zeroMem [10]int16
			clpitch.TargetSignal(aHat, &r, &zeroMem, &xCost)
		default:
			t.Fatalf("unknown gain cost target mode %q", mode.target)
		}
		scaleVectorInt16(&xCost, mode.xNum, mode.xDen)
		scaleVectorInt16(&yCost, mode.yNum, mode.yDen)
		scaleVectorInt16(&zCost, mode.zNum, mode.zDen)
		observeGainCostSurfaceMode(&stats[i], &e.pastQuaEn, &c, &xCost, &yCost, &zCost, refBitsGA, refBitsGB, refGpQ14, refGcMantQ14, refGcExp)
	}

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(pitchCode)
		e.p0 = clpitch.EncodeP0(e.p1)
		e.c1 = refC
		e.s1 = uint8(refS)
		e.ga1 = refBitsGA
		e.gb1 = refBitsGB
	} else {
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = uint8(pitchCode)
		e.c2 = refC
		e.s2 = uint8(refS)
		e.ga2 = refBitsGA
		e.gb2 = refBitsGB
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := applyGainQ14ToQ0(refGpQ14, y[n])
		gcZ := applyGcToQ12(refGcMantQ14, refGcExp, z[n])
		e.swMemErr[n-30] = saturateInt32ToInt16(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	var u [clpitch.SubframeLen]int16
	synth.BuildExcitation(refGpQ14, refGcMantQ14, refGcExp, &v, &c, &u)
	copy(e.oldExc[base:], u[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, refGammaQ13)
	e.prevGpQ14 = refGpQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func observeGainCostSurfaceMode(
	stats *gainCostSurfaceStats,
	past *[4]int16,
	c, x, y, z *[40]int16,
	refBitsGA, refBitsGB uint8,
	refGpQ14, refGcMantQ14 int16,
	refGcExp int8,
) {
	refCost := gainResidualEnergyQ0(x, y, z, refGpQ14, refGcMantQ14, refGcExp)
	bestCost := int64(1<<63 - 1)
	var bestBitsGA, bestBitsGB uint8
	rank := 1
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp, gcMant, gcExp := gainquant.Reconstruct(past, c, gai, gbi)
			cost := gainResidualEnergyQ0(x, y, z, gp, gcMant, gcExp)
			if cost < refCost {
				rank++
			}
			if cost < bestCost {
				bestCost = cost
				bestBitsGA, bestBitsGB = gainquant.PackGains(gai, gbi)
			}
		}
	}

	stats.total++
	stats.refRankSum += int64(rank)
	stats.bestCostSum += bestCost
	stats.refCostSum += refCost
	if bestBitsGA == refBitsGA {
		stats.sameGA++
	}
	if bestBitsGB == refBitsGB {
		stats.sameGB++
	}
	if bestBitsGA == refBitsGA && bestBitsGB == refBitsGB {
		stats.sameBoth++
	}
	if bestCost >= refCost {
		stats.refBestOrTie++
	}
	if rank <= 4 {
		stats.refRankLE4++
	}
}

func gainGammaQ13(gaPhys, gbPhys uint8) int32 {
	return int32(tables.GainGBK1[gaPhys][1]) + int32(tables.GainGBK2[gbPhys][1])
}

const maxInt32ForEnergyAudit = int64(1<<31 - 1)

type gainEnergyStats struct {
	count     int
	saturated int

	energySum int64
	maxEnergy int64

	nonZeroSum int64
	maxNonZero int
}

func collectGainEnergyStats(frames []bitstream.Frame) gainEnergyStats {
	var stats gainEnergyStats
	var prevGpQ14 int16
	for _, fr := range frames {
		tInt1, _ := pitchidx.DecodeDelaySubframe1(uint8(fr.P1))
		tInt2, _ := pitchidx.DecodeDelaySubframe2(uint8(fr.P2), tInt1)
		stats.observeFCB(fr.C1, uint8(fr.S1), tInt1, prevGpQ14)
		prevGpQ14 = frameGainGpQ14(uint8(fr.GA1), uint8(fr.GB1))
		stats.observeFCB(fr.C2, uint8(fr.S2), tInt2, prevGpQ14)
		prevGpQ14 = frameGainGpQ14(uint8(fr.GA2), uint8(fr.GB2))
	}
	return stats
}

func (s *gainEnergyStats) observeFCB(C uint16, S uint8, tInt int, prevGpQ14 int16) {
	var c [40]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, fcb.ClampPitchGainForEnhancement(prevGpQ14), &c)
	var energy int64
	var nonZero int
	for _, v := range c {
		if v != 0 {
			nonZero++
		}
		x := int64(v)
		energy += x * x
	}
	s.count++
	s.energySum += energy
	if energy > s.maxEnergy {
		s.maxEnergy = energy
	}
	if energy > maxInt32ForEnergyAudit {
		s.saturated++
	}
	s.nonZeroSum += int64(nonZero)
	if nonZero > s.maxNonZero {
		s.maxNonZero = nonZero
	}
}

func frameGainGpQ14(gaBits, gbBits uint8) int16 {
	ga := tables.GainImap1[gaBits&7]
	gb := tables.GainImap2[gbBits&15]
	return fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])))
}

type pitchSelectionAuditStats struct {
	total int

	intEqual              int
	fracEqualWhenIntEqual int
	refIntInWindow        int
	refRNBetter           int
	refFracBest           int
	refCenteredIntEqual   int
	refCenteredFracEqual  int

	bySub [2]pitchSelectionAuditSubStats

	examples []pitchSelectionExample
}

type pitchSelectionAuditSubStats struct {
	total          int
	intEqual       int
	refIntInWindow int
	refRNBetter    int
}

type pitchSelectionExample struct {
	frame int
	sub   int

	centre int16

	localInt  int16
	localFrac int8
	localRN   int64

	refInt      int16
	refFrac     int8
	refRN       int64
	refInWindow bool
}

func observePitchSelection(e *Encoder, frameIndex, sub int, refInt int16, refFrac int8, stats *pitchSelectionAuditStats) {
	var aHat *[11]int16
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

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)

	localInt, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	localFrac := clpitch.RefineFraction(&xb, exc, localInt, sub == 1 || localInt < 85)
	localRN := pitchRNAtFrac(&xb, exc, localInt, localFrac)
	refCenteredInt := searchIntegerWindow(&xb, exc, refInt, sub, 3)
	refCenteredFrac := clpitch.RefineFraction(&xb, exc, refCenteredInt, sub == 1 || refCenteredInt < 85)

	refInWindow := pitchIntInSearchWindow(refInt, centre, sub)
	var refRN int64
	if refInWindow {
		refRN = pitchRNAtInt(&xb, exc, refInt)
	}

	stats.total++
	stats.bySub[sub].total++
	if localInt == refInt {
		stats.intEqual++
		stats.bySub[sub].intEqual++
		if localFrac == refFrac {
			stats.fracEqualWhenIntEqual++
		}
		if pitchRNAtFrac(&xb, exc, refInt, refFrac) >= pitchRNAtFrac(&xb, exc, localInt, localFrac) {
			stats.refFracBest++
		}
	}
	if refCenteredInt == refInt {
		stats.refCenteredIntEqual++
		if refCenteredFrac == refFrac {
			stats.refCenteredFracEqual++
		}
	}
	if refInWindow {
		stats.refIntInWindow++
		stats.bySub[sub].refIntInWindow++
		if refRN > pitchRNAtInt(&xb, exc, localInt) {
			stats.refRNBetter++
			stats.bySub[sub].refRNBetter++
		}
	}
	if (localInt != refInt || localFrac != refFrac) && len(stats.examples) < 8 {
		stats.examples = append(stats.examples, pitchSelectionExample{
			frame: frameIndex,
			sub:   sub,

			centre: centre,

			localInt:  localInt,
			localFrac: localFrac,
			localRN:   localRN,

			refInt:      refInt,
			refFrac:     refFrac,
			refRN:       refRN,
			refInWindow: refInWindow,
		})
	}
}

func pitchIntInSearchWindow(intLag, centre int16, sub int) bool {
	if sub == 0 {
		kMin := centre - 3
		if kMin < clpitch.PitchMinInt {
			kMin = clpitch.PitchMinInt
		}
		kMax := centre + 3
		if kMax > clpitch.PitchMaxInt {
			kMax = clpitch.PitchMaxInt
		}
		return intLag >= kMin && intLag <= kMax
	}
	tMin, tMax := clpitch.Subframe2Window(centre)
	return intLag >= tMin && intLag <= tMax
}

func pitchRNAtInt(xb *[clpitch.SubframeLen]int16, exc []int16, intLag int16) int64 {
	base := len(exc) - clpitch.SubframeLen - int(intLag)
	var acc int64
	for n := 0; n < clpitch.SubframeLen; n++ {
		acc += int64(xb[n]) * int64(exc[base+n])
	}
	return acc
}

func pitchRNAtFrac(xb *[clpitch.SubframeLen]int16, exc []int16, intLag int16, frac int8) int64 {
	if frac == 0 {
		return pitchRNAtInt(xb, exc, intLag)
	}
	var acc int64
	for n := 0; n < clpitch.SubframeLen; n++ {
		s := clpitch.Interpolate3(exc, intLag-int16(n), frac)
		acc += int64(xb[n]) * int64(s)
	}
	return acc
}

func gainResidualEnergyQ0(x, y, z *[40]int16, gpQ14, gcMantQ14 int16, gcExp int8) int64 {
	var sum int64
	for n := 0; n < 40; n++ {
		gpY := applyGainQ14ToQ0(gpQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		err := int64(int32(x[n]) - gpY - gcZ)
		sum += err * err
	}
	return sum
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

func meanInt64(sum int64, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(sum) / float64(total)
}

func meanRatio(num, den int64) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func normalizeGainSurfaceMode(m *gainCostSurfaceMode) {
	if m.xNum == 0 {
		m.xNum = 1
	}
	if m.xDen == 0 {
		m.xDen = 1
	}
	if m.yNum == 0 {
		m.yNum = 1
	}
	if m.yDen == 0 {
		m.yDen = 1
	}
	if m.zNum == 0 {
		m.zNum = 1
	}
	if m.zDen == 0 {
		m.zDen = 1
	}
}

func scaleVectorInt16(v *[40]int16, num, den int32) {
	for i := range v {
		scaled := int32(v[i]) * num
		if den != 1 {
			if scaled >= 0 {
				scaled += den / 2
			} else {
				scaled -= den / 2
			}
			scaled /= den
		}
		v[i] = saturateInt32ToInt16(scaled)
	}
}

func scaleInt32ForDiagnostic(v, num, den int32) int32 {
	scaled := int64(v) * int64(num)
	if den != 1 {
		if scaled >= 0 {
			scaled += int64(den / 2)
		} else {
			scaled -= int64(den / 2)
		}
		scaled /= int64(den)
	}
	if scaled > 0x7fffffff {
		return 0x7fffffff
	}
	if scaled < -0x80000000 {
		return -0x80000000
	}
	return int32(scaled)
}

func targetSignalQ12ForDiagnostic(aHatQ12 *[11]int16, residual *[40]int16, swMem *[10]int16, x *[40]int16) {
	gamma := [11]int16{32767, 24576, 18432, 13824, 10368, 7776, 5832, 4374, 3281, 2460, 1845}
	var aw [11]int16
	aw[0] = aHatQ12[0]
	for i := 1; i <= 10; i++ {
		aw[i] = fixed.Mult(aHatQ12[i], gamma[i])
	}
	for n := 0; n < 40; n++ {
		var sumProd int32
		for i := 1; i <= 10; i++ {
			var xni int16
			if n-i >= 0 {
				xni = x[n-i]
			} else {
				xni = swMem[10+n-i]
			}
			sumProd += (int32(aw[i]) * int32(xni)) >> 12
		}
		x[n] = saturateInt32ToInt16(int32(residual[n]) - sumProd)
	}
}

func saturateInt32ToInt16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

func makeHybridFrames(refFrames, ourFrames []bitstream.Frame, base, family string) []bitstream.Frame {
	n := len(refFrames)
	if len(ourFrames) < n {
		n = len(ourFrames)
	}
	out := make([]bitstream.Frame, n)
	for i := 0; i < n; i++ {
		if base == "our" {
			out[i] = ourFrames[i]
			copyFieldFamily(&out[i], refFrames[i], family)
		} else {
			out[i] = refFrames[i]
			copyFieldFamily(&out[i], ourFrames[i], family)
		}
	}
	return out
}

func makeHybridFrameRange(baseFrames, donorFrames []bitstream.Frame, family string, start, end int) []bitstream.Frame {
	n := len(baseFrames)
	if len(donorFrames) < n {
		n = len(donorFrames)
	}
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	out := make([]bitstream.Frame, n)
	copy(out, baseFrames[:n])
	for i := start; i < end; i++ {
		copyFieldFamily(&out[i], donorFrames[i], family)
	}
	return out
}

func copyFieldFamily(dst *bitstream.Frame, src bitstream.Frame, family string) {
	if strings.Contains(family, "+") {
		for _, part := range strings.Split(family, "+") {
			copyFieldFamily(dst, src, part)
		}
		return
	}
	switch family {
	case "":
		return
	case "lsp":
		dst.L0, dst.L1, dst.L2, dst.L3 = src.L0, src.L1, src.L2, src.L3
	case "pitch":
		dst.P1, dst.P0, dst.P2 = src.P1, src.P0, src.P2
	case "pitch1":
		dst.P1, dst.P0 = src.P1, src.P0
	case "pitch2":
		dst.P2 = src.P2
	case "fcb":
		dst.C1, dst.S1, dst.C2, dst.S2 = src.C1, src.S1, src.C2, src.S2
	case "fcb1":
		dst.C1, dst.S1 = src.C1, src.S1
	case "fcb2":
		dst.C2, dst.S2 = src.C2, src.S2
	case "c":
		dst.C1, dst.C2 = src.C1, src.C2
	case "s":
		dst.S1, dst.S2 = src.S1, src.S2
	case "gain":
		dst.GA1, dst.GB1, dst.GA2, dst.GB2 = src.GA1, src.GB1, src.GA2, src.GB2
	case "gain1":
		dst.GA1, dst.GB1 = src.GA1, src.GB1
	case "gain2":
		dst.GA2, dst.GB2 = src.GA2, src.GB2
	case "ga":
		dst.GA1, dst.GA2 = src.GA1, src.GA2
	case "gb":
		dst.GB1, dst.GB2 = src.GB1, src.GB2
	case "gainLowGamma":
		// Physical GBK1[0] + GBK2[1] has the smallest gamma correction.
		setGainFields(dst, tables.GainMap1[0], tables.GainMap2[1])
	case "gainLowGp":
		// Physical GBK1[0] + GBK2[0] has the smallest pitch gain.
		setGainFields(dst, tables.GainMap1[0], tables.GainMap2[0])
	case "gainMid":
		setGainFields(dst, tables.GainMap1[3], tables.GainMap2[4])
	case "gainHigh":
		setGainFields(dst, tables.GainMap1[5], tables.GainMap2[15])
	case "gainZeroBits":
		setGainFields(dst, 0, 0)
	case "gainIdentity":
		remapGainFields(dst, "identity", true, true)
	case "gainInverse":
		remapGainFields(dst, "inverse", true, true)
	case "gbIdentity":
		remapGainFields(dst, "identity", false, true)
	case "gbInverse":
		remapGainFields(dst, "inverse", false, true)
	case "gaIdentity":
		remapGainFields(dst, "identity", true, false)
	case "gaInverse":
		remapGainFields(dst, "inverse", true, false)
	case "signInvert":
		dst.S1 ^= 0x0f
		dst.S2 ^= 0x0f
	case "signReverse":
		dst.S1 = reverse4(dst.S1)
		dst.S2 = reverse4(dst.S2)
	case "signReverseInvert":
		dst.S1 = reverse4(dst.S1) ^ 0x0f
		dst.S2 = reverse4(dst.S2) ^ 0x0f
	case "cFlipJx":
		dst.C1 ^= 1 << 9
		dst.C2 ^= 1 << 9
	case "pitchFracFlip":
		flipPitchFracFields(dst, true, true)
	case "pitchFracFlip1":
		flipPitchFracFields(dst, true, false)
	case "pitchFracFlip2":
		flipPitchFracFields(dst, false, true)
	case "pitchFracZero":
		zeroPitchFracFields(dst)
	}
}

func cloneFrames(in []bitstream.Frame) []bitstream.Frame {
	out := make([]bitstream.Frame, len(in))
	copy(out, in)
	return out
}

func shiftFrameSequence(in []bitstream.Frame, offset int) []bitstream.Frame {
	out := make([]bitstream.Frame, len(in))
	for i := range out {
		src := i + offset
		if src < 0 {
			src = 0
		}
		if src >= len(in) {
			src = len(in) - 1
		}
		out[i] = in[src]
	}
	return out
}

func shiftFrameFamily(in []bitstream.Frame, offset int, family string) []bitstream.Frame {
	out := cloneFrames(in)
	for i := range out {
		src := i + offset
		if src < 0 {
			src = 0
		}
		if src >= len(in) {
			src = len(in) - 1
		}
		copyFieldFamily(&out[i], in[src], family)
	}
	return out
}

func applyFrameTransform(f *bitstream.Frame, mode string) {
	copyFieldFamily(f, bitstream.Frame{}, mode)
}

func joinTransformLabel(parts ...string) string {
	out := "identity"
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out == "identity" {
			out = p
		} else {
			out += "+" + p
		}
	}
	return out
}

func setGainFields(f *bitstream.Frame, ga, gb uint8) {
	f.GA1, f.GA2 = uint16(ga), uint16(ga)
	f.GB1, f.GB2 = uint16(gb), uint16(gb)
}

func reverse4(v uint16) uint16 {
	return ((v & 0x1) << 3) | ((v & 0x2) << 1) | ((v & 0x4) >> 1) | ((v & 0x8) >> 3)
}

func remapGainFields(f *bitstream.Frame, mode string, includeGA, includeGB bool) {
	remapGA := func(v uint16) uint16 {
		phys := tables.GainImap1[v&7]
		if mode == "inverse" {
			return uint16(tables.GainImap1[phys])
		}
		return uint16(phys)
	}
	remapGB := func(v uint16) uint16 {
		phys := tables.GainImap2[v&15]
		if mode == "inverse" {
			return uint16(tables.GainImap2[phys])
		}
		return uint16(phys)
	}
	if includeGA {
		f.GA1 = remapGA(f.GA1)
		f.GA2 = remapGA(f.GA2)
	}
	if includeGB {
		f.GB1 = remapGB(f.GB1)
		f.GB2 = remapGB(f.GB2)
	}
}

func flipPitchFracFields(f *bitstream.Frame, sf1, sf2 bool) {
	if sf1 {
		t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
		if frac1 != 0 {
			p1 := clpitch.EncodeP1(int16(t1), int8(-frac1))
			f.P1 = uint16(p1)
			f.P0 = uint16(clpitch.EncodeP0(p1))
		}
	}
	if sf2 {
		t1, _ := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
		t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(f.P2), t1)
		if frac2 != 0 {
			tmin, _ := clpitch.Subframe2Window(int16(t1))
			f.P2 = uint16(clpitch.EncodeP2(int16(t2), int8(-frac2), tmin))
		}
	}
}

func zeroPitchFracFields(f *bitstream.Frame) {
	t1, frac1 := pitchidx.DecodeDelaySubframe1(uint8(f.P1))
	if frac1 != 0 {
		p1 := clpitch.EncodeP1(int16(t1), 0)
		f.P1 = uint16(p1)
		f.P0 = uint16(clpitch.EncodeP0(p1))
	}
	t1, _ = pitchidx.DecodeDelaySubframe1(uint8(f.P1))
	t2, frac2 := pitchidx.DecodeDelaySubframe2(uint8(f.P2), t1)
	if frac2 != 0 {
		tmin, _ := clpitch.Subframe2Window(int16(t1))
		f.P2 = uint16(clpitch.EncodeP2(int16(t2), 0, tmin))
	}
}

func writePackedFrames(t *testing.T, frames []bitstream.Frame, path string) {
	t.Helper()
	raw := make([]byte, 0, len(frames)*FrameBytes)
	var packed [FrameBytes]byte
	for i := range frames {
		if err := bitstream.Pack(&frames[i], packed[:]); err != nil {
			t.Fatalf("Pack hybrid frame %d: %v", i, err)
		}
		raw = append(raw, packed[:]...)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func ffmpegDecodeRawG729(t *testing.T, inPath, outPath string) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "g729", "-i", inPath,
		"-f", "s16le", "-ar", "8000", "-ac", "1", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg decode %s: %v\n%s", inPath, err, out)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func s16leToSamples(data []byte) []int16 {
	n := len(data) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i : 2*i+2]))
	}
	return out
}

func shiftedSamples(in []int16, shift int) []int16 {
	out := make([]int16, len(in))
	for i := range out {
		j := i + shift
		if j >= 0 && j < len(in) {
			out[i] = in[j]
		}
	}
	return out
}

func itoaSigned(v int) string {
	if v >= 0 {
		return "+" + itoa(v)
	}
	return "-" + itoa(-v)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
