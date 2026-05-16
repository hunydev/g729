package g729

import (
	"fmt"
	"math"
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
	"github.com/hunydev/g729/internal/gainquant"
	pitchcore "github.com/hunydev/g729/internal/pitch"
	clpitch "github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// TestExternalFFmpegBlackboxGainSearchScaleSweep_SPEECH keeps FFmpeg as a
// black-box decoder and varies only the encoder-side g'c value passed into
// the §3.9.2 gain-search cost. The emitted bitstream is then decoded by
// FFmpeg. The diagnostic logs both raw and scale-normalized metrics so a
// quiet-output artifact is not mistaken for recovered speech shape.
func TestExternalFFmpegBlackboxGainSearchScaleSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box gain-search sweep")
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
	refRaw := filepath.Join(tmp, "speech-ref.g729")
	refPCM := filepath.Join(tmp, "speech-ref.ffmpeg.s16le")
	writeG192AsRawG729(t, bitData[:frames*bytesPerBitFrame], refRaw)
	ffmpegDecodeRawG729(t, refRaw, refPCM)
	refFF := s16leToSamples(readFile(t, refPCM))
	if len(refFF) > totalSamples {
		refFF = refFF[:totalSamples]
	}

	refMetrics := measureFFmpegSweep(src, refFF, 240)
	pstMetrics := measureFFmpegSweep(src, pst, 240)
	t.Logf("gain-search g'c scale sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-18s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-18s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.PST", pstMetrics.shift, pstMetrics.rms, pstMetrics.globalSNR, pstMetrics.segSNR,
		pstMetrics.corr, pstMetrics.optGlobalSNR, pstMetrics.optSegSNR, pstMetrics.rms/refMetrics.rms)
	t.Logf("%-18s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT->ff", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, refMetrics.rms/refMetrics.rms)

	modes := []struct {
		name           string
		xNum, xDen     int32
		yNum, yDen     int32
		zNum, zDen     int32
		gpcNum, gpcDen int32
		native         bool
		rawLinear      bool
		fullCost       bool
	}{
		{name: "gpc x1/4", gpcNum: 1, gpcDen: 4},
		{name: "gpc x1/2", gpcNum: 1, gpcDen: 2},
		{name: "gpc x1", gpcNum: 1, gpcDen: 1},
		{name: "gpc x3/2", gpcNum: 3, gpcDen: 2},
		{name: "gpc x2", gpcNum: 2, gpcDen: 1},
		{name: "gpc x5/2", gpcNum: 5, gpcDen: 2},
		{name: "gpc x3", gpcNum: 3, gpcDen: 1},
		{name: "gpc x4", gpcNum: 4, gpcDen: 1},
		{name: "gpc x8", gpcNum: 8, gpcDen: 1},
		{name: "x half", xNum: 1, xDen: 2, gpcNum: 1, gpcDen: 1},
		{name: "y x2", yNum: 2, yDen: 1, gpcNum: 1, gpcDen: 1},
		{name: "z x2", zNum: 2, zDen: 1, gpcNum: 1, gpcDen: 1},
		{name: "x half+gpc x4", xNum: 1, xDen: 2, gpcNum: 4, gpcDen: 1},
		{name: "y x2+gpc x4", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "x half+y x2+gpc x4", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "native exhaustive", gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+x eighth", xNum: 1, xDen: 8, gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+x quarter", xNum: 1, xDen: 4, gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+x half", xNum: 1, xDen: 2, gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+x quarter+y x2", xNum: 1, xDen: 4, yNum: 2, yDen: 1, gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+y x2", yNum: 2, yDen: 1, gpcNum: 1, gpcDen: 1, native: true},
		{name: "native+x half+y x2", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 1, gpcDen: 1, native: true},
		{name: "raw linear exhaustive", gpcNum: 1, gpcDen: 1, rawLinear: true},
		{name: "raw linear gpc x4", gpcNum: 4, gpcDen: 1, rawLinear: true},
		{name: "full cost gpc x1", gpcNum: 1, gpcDen: 1, fullCost: true},
		{name: "full cost gpc x4", gpcNum: 4, gpcDen: 1, fullCost: true},
		{name: "full cost xhalf gpc4", xNum: 1, xDen: 2, gpcNum: 4, gpcDen: 1, fullCost: true},
	}

	var bestCorr struct {
		name string
		m    ffmpegSweepMetrics
	}
	bestCorr.m.corr = math.Inf(-1)
	var bestOpt struct {
		name string
		m    ffmpegSweepMetrics
	}
	bestOpt.m.optSegSNR = math.Inf(-1)

	for _, mode := range modes {
		normalizeGainSweepMode(&mode.xNum, &mode.xDen)
		normalizeGainSweepMode(&mode.yNum, &mode.yDen)
		normalizeGainSweepMode(&mode.zNum, &mode.zDen)
		normalizeGainSweepMode(&mode.gpcNum, &mode.gpcDen)
		framesOut := encodeBitstreamFramesGainSearchScale(t, src, mode.xNum, mode.xDen, mode.yNum, mode.yDen, mode.zNum, mode.zDen, mode.gpcNum, mode.gpcDen, mode.native, mode.rawLinear, mode.fullCost)
		fileBase := strings.ReplaceAll(mode.name, "/", "_")
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-18s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
		if m.corr > bestCorr.m.corr {
			bestCorr.name = mode.name
			bestCorr.m = m
		}
		if m.optSegSNR > bestOpt.m.optSegSNR {
			bestOpt.name = mode.name
			bestOpt.m = m
		}
	}
	t.Logf("best corr: %s corr %.3f optSeg %.2f rms/ref %.3f",
		bestCorr.name, bestCorr.m.corr, bestCorr.m.optSegSNR, bestCorr.m.rms/refMetrics.rms)
	t.Logf("best optSeg: %s corr %.3f optSeg %.2f rms/ref %.3f",
		bestOpt.name, bestOpt.m.corr, bestOpt.m.optSegSNR, bestOpt.m.rms/refMetrics.rms)
}

func TestExternalFFmpegBlackboxGainSearchWideGrid_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box wide gain-search grid")
	}
	if os.Getenv("G729_FFMPEG_BLACKBOX_WIDE_GRID") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_WIDE_GRID=1 to run the long wide gain-search grid")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	type scale struct {
		label string
		num   int32
		den   int32
	}
	scales := []scale{
		{"1/4", 1, 4},
		{"1/2", 1, 2},
		{"1", 1, 1},
		{"2", 2, 1},
		{"4", 4, 1},
		{"8", 8, 1},
	}
	type result struct {
		name string
		m    ffmpegSweepMetrics
	}
	top := make([]result, 0, 10)
	addResult := func(name string, m ffmpegSweepMetrics) {
		top = append(top, result{name: name, m: m})
		for i := len(top) - 1; i > 0 && top[i].m.segSNR > top[i-1].m.segSNR; i-- {
			top[i], top[i-1] = top[i-1], top[i]
		}
		if len(top) > 10 {
			top = top[:10]
		}
	}

	t.Logf("wide gain-search scale grid — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("reference: shift=%d rms=%.0f Global=%.2f Seg=%.2f corr=%.3f",
		refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR, refMetrics.corr)

	for _, x := range scales {
		for _, y := range scales {
			for _, z := range scales {
				for _, gpc := range scales {
					name := fmt.Sprintf("x%s_y%s_z%s_gpc%s", x.label, y.label, z.label, gpc.label)
					framesOut := encodeBitstreamFramesGainSearchScale(t, src, x.num, x.den, y.num, y.den, z.num, z.den, gpc.num, gpc.den, false, false, false)
					rawPath := filepath.Join(tmp, strings.ReplaceAll(name, "/", "_")+".g729")
					pcmPath := filepath.Join(tmp, strings.ReplaceAll(name, "/", "_")+".s16le")
					writePackedFrames(t, framesOut, rawPath)
					ffmpegDecodeRawG729(t, rawPath, pcmPath)
					decoded := s16leToSamples(readFile(t, pcmPath))
					if len(decoded) > totalSamples {
						decoded = decoded[:totalSamples]
					}
					if len(decoded) < totalSamples {
						t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", name, len(decoded), totalSamples)
					}
					m := measureFFmpegSweep(src, decoded, 240)
					addResult(name, m)
				}
			}
		}
	}

	t.Logf("%-24s %7s %8s %8s %8s %8s %8s %8s",
		"mode", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg")
	for _, r := range top {
		t.Logf("%-24s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f",
			r.name, r.m.shift, r.m.rms, r.m.globalSNR, r.m.segSNR, r.m.corr, r.m.optGlobalSNR, r.m.optSegSNR)
	}
}

// TestExternalFFmpegBlackboxFCBTargetGainSweep_SPEECH varies only the
// adaptive-gain contribution used to form the fixed-codebook target
// x' = x - gp*y, then decodes the produced bitstream with FFmpeg as a
// black box. This separates FCB-search surface damage from the already
// retained gain-search g'c bias.
func TestExternalFFmpegBlackboxFCBTargetGainSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box FCB target sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []struct {
		name                   string
		fcbGpNum, fcbGpDen     int32
		gpcNum, gpcDen         int32
		searchXNum, searchXDen int32
		searchYNum, searchYDen int32
	}{
		{name: "fcb gp x0", fcbGpNum: 0, fcbGpDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x1/4", fcbGpNum: 1, fcbGpDen: 4, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x1/2", fcbGpNum: 1, fcbGpDen: 2, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x3/4", fcbGpNum: 3, fcbGpDen: 4, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x1", fcbGpNum: 1, fcbGpDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x5/4", fcbGpNum: 5, fcbGpDen: 4, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x3/2", fcbGpNum: 3, fcbGpDen: 2, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x2", fcbGpNum: 2, fcbGpDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb gp x1 gpc x2", fcbGpNum: 1, fcbGpDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "fcb gp x1 gpc x8", fcbGpNum: 1, fcbGpDen: 1, gpcNum: 8, gpcDen: 1},
		{name: "fcb gp x1 xhalf", fcbGpNum: 1, fcbGpDen: 1, gpcNum: 4, gpcDen: 1, searchXNum: 1, searchXDen: 2},
		{name: "fcb gp x1 y2", fcbGpNum: 1, fcbGpDen: 1, gpcNum: 4, gpcDen: 1, searchYNum: 2, searchYDen: 1},
	}

	t.Logf("FCB target gp sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-20s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-20s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT->ff", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		normalizeGainSweepMode(&mode.fcbGpNum, &mode.fcbGpDen)
		normalizeGainSweepMode(&mode.gpcNum, &mode.gpcDen)
		normalizeGainSweepMode(&mode.searchXNum, &mode.searchXDen)
		normalizeGainSweepMode(&mode.searchYNum, &mode.searchYDen)
		framesOut := encodeBitstreamFramesFCBTargetGainScale(t, src,
			mode.fcbGpNum, mode.fcbGpDen,
			mode.gpcNum, mode.gpcDen,
			mode.searchXNum, mode.searchXDen,
			mode.searchYNum, mode.searchYDen)
		fileBase := strings.ReplaceAll(mode.name, "/", "_")
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-20s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

func TestExternalFFmpegBlackboxFCBGainAwareRerank_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box FCB gain-aware rerank diagnostic")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []struct {
		name                   string
		topK                   int
		gpcNum, gpcDen         int32
		searchXNum, searchXDen int32
		searchYNum, searchYDen int32
	}{
		{name: "top1 current", topK: 1, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top4 current", topK: 4, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top8 current", topK: 8, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top16 current", topK: 16, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top32 current", topK: 32, gpcNum: 5, gpcDen: 3, searchXNum: 1, searchXDen: 2, searchYNum: 7, searchYDen: 2},
		{name: "top8 thin", topK: 8, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
		{name: "top16 thin", topK: 16, gpcNum: 7, gpcDen: 4, searchXNum: 1, searchXDen: 2, searchYNum: 15, searchYDen: 4},
	}

	t.Logf("FCB gain-aware rerank sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-14s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-14s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		normalizeGainSweepMode(&mode.searchXNum, &mode.searchXDen)
		normalizeGainSweepMode(&mode.searchYNum, &mode.searchYDen)
		normalizeGainSweepMode(&mode.gpcNum, &mode.gpcDen)
		framesOut := encodeBitstreamFramesFCBGainAwareRerank(t, src, mode.topK,
			mode.gpcNum, mode.gpcDen,
			mode.searchXNum, mode.searchXDen, mode.searchYNum, mode.searchYDen)
		fileBase := strings.ReplaceAll(mode.name, " ", "_")
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-14s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

// TestExternalFFmpegBlackboxPitchCenterSweep_SPEECH probes whether the
// open-loop pitch centre is sending the closed-loop search into a harmonic
// basin. Only the subframe-1 centre is transformed; the P2 window still
// follows the selected T1.
func TestExternalFFmpegBlackboxPitchCenterSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box pitch-centre sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []string{
		"identity",
		"minus40",
		"minus20",
		"plus20",
		"plus40",
		"half",
		"double",
		"preferHalf",
		"preferDouble",
	}

	t.Logf("pitch centre sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-16s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-16s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT->ff", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesPitchCenterMode(t, src, mode)
		rawPath := filepath.Join(tmp, mode+".g729")
		pcmPath := filepath.Join(tmp, mode+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-16s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

func TestExternalFFmpegBlackboxPitchWindowSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box pitch-window sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	windows := []int{1, 2, 3, 5, 7, 10, 15, 20, 40, 80}
	t.Logf("pitch window sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-12s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-12s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, half := range windows {
		name := "win" + strconv.Itoa(half)
		framesOut := encodeBitstreamFramesPitchWindow(t, src, half)
		rawPath := filepath.Join(tmp, name+".g729")
		pcmPath := filepath.Join(tmp, name+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-12s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

func TestExternalFFmpegBlackboxPitchWindowGainComboSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box pitch/gain combo sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []struct {
		name           string
		halfWindow     int
		xNum, xDen     int32
		yNum, yDen     int32
		gpcNum, gpcDen int32
	}{
		{name: "win1 gpc4", halfWindow: 1, xNum: 1, xDen: 1, yNum: 1, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win1 xhalf gpc4", halfWindow: 1, xNum: 1, xDen: 2, yNum: 1, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win1 y2 gpc4", halfWindow: 1, xNum: 1, xDen: 1, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win1 xhalf y2 gpc4", halfWindow: 1, xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win1 gpc2", halfWindow: 1, xNum: 1, xDen: 1, yNum: 1, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "win1 gpc8", halfWindow: 1, xNum: 1, xDen: 1, yNum: 1, yDen: 1, gpcNum: 8, gpcDen: 1},
		{name: "win2 xhalf gpc4", halfWindow: 2, xNum: 1, xDen: 2, yNum: 1, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win3 xhalf gpc4", halfWindow: 3, xNum: 1, xDen: 2, yNum: 1, yDen: 1, gpcNum: 4, gpcDen: 1},
	}

	t.Logf("pitch-window/gain combo sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-22s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-22s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesPitchWindowGain(t, src, mode.halfWindow, mode.xNum, mode.xDen, mode.yNum, mode.yDen, mode.gpcNum, mode.gpcDen)
		fileBase := strings.ReplaceAll(mode.name, " ", "_")
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-22s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

// TestExternalFFmpegBlackboxClosedLoopSurfaceSweep_SPEECH varies the
// encoder-side closed-loop input surface before pitch/FCB/gain selection.
// It keeps FFmpeg as a black-box decoder and does not consume external
// implementation code. The purpose is to distinguish a bad local search
// algorithm from a bad x/h/r/excitation surface feeding otherwise coherent
// local searches.
func TestExternalFFmpegBlackboxClosedLoopSurfaceSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box closed-loop surface sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []closedLoopSurfaceMode{
		{name: "production", speechStart: 120, gpcNum: 4, gpcDen: 1},
		{name: "unquant residual", speechStart: 120, residualA: "unquant", gpcNum: 4, gpcDen: 1},
		{name: "unquant filter", speechStart: 120, filterA: "unquant", gpcNum: 4, gpcDen: 1},
		{name: "unquant all", speechStart: 120, residualA: "unquant", filterA: "unquant", gpcNum: 4, gpcDen: 1},
		{name: "aprime filter", speechStart: 120, filterA: "aprime", gpcNum: 4, gpcDen: 1},
		{name: "aprime filter win1", speechStart: 120, filterA: "aprime", halfWindow: 1, gpcNum: 4, gpcDen: 1},
		{name: "aprime filter y2", speechStart: 120, filterA: "aprime", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "zero target mem", speechStart: 120, targetMem: "zero", gpcNum: 4, gpcDen: 1},
		{name: "zero residual mem", speechStart: 120, residualMem: "zero", gpcNum: 4, gpcDen: 1},
		{name: "zero residual ext", speechStart: 120, residualExt: "zero", gpcNum: 4, gpcDen: 1},
		{name: "speech start 80", speechStart: 80, gpcNum: 4, gpcDen: 1},
		{name: "speech start 160", speechStart: 160, gpcNum: 4, gpcDen: 1},
		{name: "unquant all zero mem", speechStart: 120, residualA: "unquant", filterA: "unquant", targetMem: "zero", gpcNum: 4, gpcDen: 1},
		{name: "production gpc x1", speechStart: 120, gpcNum: 1, gpcDen: 1},
		{name: "win1 y2 gpc4", speechStart: 120, halfWindow: 1, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "win1 xhalf y2 gpc4", speechStart: 120, halfWindow: 1, xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "zero resmem win1", speechStart: 120, residualMem: "zero", halfWindow: 1, gpcNum: 4, gpcDen: 1},
		{name: "zero resmem win1 y2", speechStart: 120, residualMem: "zero", halfWindow: 1, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "zero resmem win1 xhalf", speechStart: 120, residualMem: "zero", halfWindow: 1, xNum: 1, xDen: 2, gpcNum: 4, gpcDen: 1},
		{name: "zero resmem win1 xhalf y2", speechStart: 120, residualMem: "zero", halfWindow: 1, xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "unquant all win1 y2", speechStart: 120, residualA: "unquant", filterA: "unquant", halfWindow: 1, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short", speechStart: 120, acbMode: "decoderShort", gpcNum: 4, gpcDen: 1},
		{name: "decoder acb all", speechStart: 120, acbMode: "decoderAll", gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short win1", speechStart: 120, acbMode: "decoderShort", halfWindow: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short y2", speechStart: 120, acbMode: "decoderShort", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short prod", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short p05", speechStart: 120, acbMode: "decoderShort", fcbMode: "score:p05", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short x4", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 4, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short x8", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 8, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short y4", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "decoder acb short gpc8", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 8, gpcDen: 1},
		{name: "decoder acb short gpc16", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 16, gpcDen: 1},
		{name: "pitch allow negative", speechStart: 120, pitchMode: "allowNegative", gpcNum: 4, gpcDen: 1},
		{name: "pitch allow neg win1", speechStart: 120, pitchMode: "allowNegative", halfWindow: 1, gpcNum: 4, gpcDen: 1},
		{name: "pitch normalized", speechStart: 120, pitchMode: "normalized", gpcNum: 4, gpcDen: 1},
		{name: "pitch norm win1", speechStart: 120, pitchMode: "normalized", halfWindow: 1, gpcNum: 4, gpcDen: 1},
		{name: "pitch norm y2", speechStart: 120, pitchMode: "normalized", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb sharpen h", speechStart: 120, fcbMode: "sharpenH", gpcNum: 4, gpcDen: 1},
		{name: "fcb sharpen h y2", speechStart: 120, fcbMode: "sharpenH", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb sharp win1 y2", speechStart: 120, fcbMode: "sharpenH", halfWindow: 1, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb annexA90", speechStart: 120, fcbMode: "annexA90", gpcNum: 4, gpcDen: 1},
		{name: "fcb annexA90 y2", speechStart: 120, fcbMode: "annexA90", yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb score p0 prod", speechStart: 120, fcbMode: "score:p0", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb score p05 prod", speechStart: 120, fcbMode: "score:p05", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb score p15 prod", speechStart: 120, fcbMode: "score:p15", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb score p2 prod", speechStart: 120, fcbMode: "score:p2", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "fcb p05 native x8", speechStart: 120, fcbMode: "score:p05", gainMode: "native", xNum: 1, xDen: 8, gpcNum: 1, gpcDen: 1},
		{name: "fcb p05 native x4y2", speechStart: 120, fcbMode: "score:p05", gainMode: "native", xNum: 1, xDen: 4, yNum: 2, yDen: 1, gpcNum: 1, gpcDen: 1},
		{name: "weighted direct prod", speechStart: 120, targetMode: "weightedDirect", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "weighted direct p05", speechStart: 120, targetMode: "weightedDirect", fcbMode: "score:p05", xNum: 1, xDen: 2, yNum: 2, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "weighted native x8", speechStart: 120, targetMode: "weightedDirect", gainMode: "native", xNum: 1, xDen: 8, gpcNum: 1, gpcDen: 1},
	}

	t.Logf("closed-loop surface sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-22s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-22s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT->ff", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
		fileBase := strings.NewReplacer(" ", "_", "/", "_").Replace(mode.name)
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-22s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

func TestExternalFFmpegBlackboxPitchCenterThresholdSweep_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box pitch-center threshold sweep")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []closedLoopSurfaceMode{
		{name: "current", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 5, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 3},
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

	t.Logf("pitch-center threshold sweep — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-14s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-14s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
		fileBase := strings.NewReplacer(" ", "_", "/", "_").Replace(mode.name)
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-14s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

func TestExternalFFmpegBlackboxClosedLoopShortACBGainGrid_SPEECH(t *testing.T) {
	if os.Getenv("G729_FFMPEG_BLACKBOX_QUALITY") != "1" {
		t.Skip("set G729_FFMPEG_BLACKBOX_QUALITY=1 to run ffmpeg black-box short-ACB gain grid")
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
	refMetrics := measureFFmpegSweep(src, refFF, 240)

	modes := []closedLoopSurfaceMode{
		{name: "production", speechStart: 120, gpcNum: 4, gpcDen: 1},
		{name: "short y3", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 3, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short y4", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short y5", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 5, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short y6", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 6, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short x1 y4", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 1, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short x2/3 y4", speechStart: 120, acbMode: "decoderShort", xNum: 2, xDen: 3, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short x3/4 y4", speechStart: 120, acbMode: "decoderShort", xNum: 3, xDen: 4, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short x1/3 y4", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 3, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short y4 gpc2", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short y4 gpc6", speechStart: 120, acbMode: "decoderShort", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 6, gpcDen: 1},
		{name: "short y4 p05", speechStart: 120, acbMode: "decoderShort", fcbMode: "score:p05", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short y5 p05", speechStart: 120, acbMode: "decoderShort", fcbMode: "score:p05", xNum: 1, xDen: 2, yNum: 5, yDen: 1, gpcNum: 4, gpcDen: 1},
		{name: "short search y3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 3, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short search y10/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 10, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "short search y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "short search y11/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 11, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "short search y4 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short search y7/2 gpc3/2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 3, gpcDen: 2},
		{name: "short search y7/2 gpc5/2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 5, gpcDen: 2},
		{name: "short search win1 y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", halfWindow: 1, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "short search y5 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 5, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short search x3/4 y3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 3, xDen: 4, yNum: 3, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short search x2/3 y4 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 2, xDen: 3, yNum: 4, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short search y4 gpc3", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortACB", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 3, gpcDen: 1},
		{name: "short norm y3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 1, xDen: 2, yNum: 3, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "short norm y10/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 1, xDen: 2, yNum: 10, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "short norm y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "short norm y11/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 1, xDen: 2, yNum: 11, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "short norm y4 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 3, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y10/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 10, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y17/5 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 17, yDen: 5, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm start80 y7/2 gpc2", speechStart: 80, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm start160 y7/2 gpc2", speechStart: 160, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y18/5 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 18, yDen: 5, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y11/3 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 11, yDen: 3, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm y7/2 gpc7/4", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 7, gpcDen: 4},
		{name: "zero short norm y7/2 gpc9/4", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 9, gpcDen: 4},
		{name: "zero short norm y7/2 gpc5/2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 5, gpcDen: 2},
		{name: "zero short norm x4/9 y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 4, xDen: 9, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "zero short norm x5/9 y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", transmitFracZero: true, xNum: 5, xDen: 9, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "short norm win1 y7/2 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", halfWindow: 1, xNum: 1, xDen: 2, yNum: 7, yDen: 2, gpcNum: 2, gpcDen: 1},
		{name: "short norm x2/3 y4 gpc2", speechStart: 120, acbMode: "decoderShort", pitchMode: "decoderShortNorm", xNum: 2, xDen: 3, yNum: 4, yDen: 1, gpcNum: 2, gpcDen: 1},
		{name: "all y4", speechStart: 120, acbMode: "decoderAll", xNum: 1, xDen: 2, yNum: 4, yDen: 1, gpcNum: 4, gpcDen: 1},
	}

	t.Logf("short-ACB gain grid — SPEECH corpus (%d frames, %d samples)", frames, totalSamples)
	t.Logf("%-18s %7s %8s %8s %8s %8s %8s %8s %8s",
		"stream", "shift", "rms", "gSNR", "seg", "corr", "optG", "optSeg", "rms/ref")
	t.Logf("%-18s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
		"SPEECH.BIT->ff", refMetrics.shift, refMetrics.rms, refMetrics.globalSNR, refMetrics.segSNR,
		refMetrics.corr, refMetrics.optGlobalSNR, refMetrics.optSegSNR, 1.0)
	for _, mode := range modes {
		framesOut := encodeBitstreamFramesClosedLoopSurface(t, src, mode)
		fileBase := strings.NewReplacer(" ", "_", "/", "_").Replace(mode.name)
		rawPath := filepath.Join(tmp, fileBase+".g729")
		pcmPath := filepath.Join(tmp, fileBase+".s16le")
		writePackedFrames(t, framesOut, rawPath)
		ffmpegDecodeRawG729(t, rawPath, pcmPath)
		decoded := s16leToSamples(readFile(t, pcmPath))
		if len(decoded) > totalSamples {
			decoded = decoded[:totalSamples]
		}
		if len(decoded) < totalSamples {
			t.Fatalf("%s: ffmpeg output too short: got %d want >= %d", mode.name, len(decoded), totalSamples)
		}
		m := measureFFmpegSweep(src, decoded, 240)
		t.Logf("%-18s %7d %8.0f %8.2f %8.2f %8.3f %8.2f %8.2f %8.3f",
			mode.name, m.shift, m.rms, m.globalSNR, m.segSNR, m.corr, m.optGlobalSNR, m.optSegSNR, m.rms/refMetrics.rms)
	}
}

type ffmpegSweepMetrics struct {
	shift        int
	rms          float64
	globalSNR    float64
	segSNR       float64
	corr         float64
	optGlobalSNR float64
	optSegSNR    float64
}

func measureFFmpegSweep(ref, test []int16, maxShift int) ffmpegSweepMetrics {
	shift, global, seg := bestAlignedSNR(ref, test, maxShift)
	aligned := alignByShift(ref, test, shift)
	corr := corrCoeff(ref, aligned)
	scale := leastSquaresScale(ref, aligned)
	scaled := scaleSamples(aligned, scale)
	optGlobal, optSeg := snrPair(ref, scaled)
	return ffmpegSweepMetrics{
		shift:        shift,
		rms:          rmsAmp(test),
		globalSNR:    global,
		segSNR:       seg,
		corr:         corr,
		optGlobalSNR: optGlobal,
		optSegSNR:    optSeg,
	}
}

func alignByShift(ref, test []int16, shift int) []int16 {
	out := make([]int16, len(ref))
	for i := range ref {
		j := i + shift
		if j >= 0 && j < len(test) {
			out[i] = test[j]
		}
	}
	return out
}

func corrCoeff(a, b []int16) float64 {
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

func leastSquaresScale(ref, test []int16) float64 {
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	var num, den float64
	for i := 0; i < n; i++ {
		r := float64(ref[i])
		t := float64(test[i])
		num += r * t
		den += t * t
	}
	if den <= 0 {
		return 0
	}
	return num / den
}

func scaleSamples(in []int16, scale float64) []int16 {
	out := make([]int16, len(in))
	for i, v := range in {
		x := int32(math.Round(float64(v) * scale))
		if x > 32767 {
			x = 32767
		} else if x < -32768 {
			x = -32768
		}
		out[i] = int16(x)
	}
	return out
}

func normalizeGainSweepMode(num, den *int32) {
	if *num == 0 {
		*num = 1
	}
	if *den == 0 {
		*den = 1
	}
}

func encodeBitstreamFramesGainSearchScale(t *testing.T, samples []int16, xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen int32, native, rawLinear, fullCost bool) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepGainSearchScale(enc, 0, xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen, native, rawLinear, fullCost)
		closedloopStepGainSearchScale(enc, 1, xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen, native, rawLinear, fullCost)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesFCBTargetGainScale(t *testing.T, samples []int16, fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen int32) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepFCBTargetGainScale(enc, 0, fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
		closedloopStepFCBTargetGainScale(enc, 1, fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesFCBGainAwareRerank(t *testing.T, samples []int16, topK int, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen int32) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepFCBGainAwareRerank(enc, 0, topK, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
		closedloopStepFCBGainAwareRerank(enc, 1, topK, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesPitchCenterMode(t *testing.T, samples []int16, mode string) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepPitchCenterMode(enc, 0, mode)
		closedloopStepPitchCenterMode(enc, 1, mode)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func encodeBitstreamFramesPitchWindow(t *testing.T, samples []int16, halfWindow int) []bitstream.Frame {
	t.Helper()
	return encodeBitstreamFramesPitchWindowGain(t, samples, halfWindow, 1, 1, 1, 1, 4, 1)
}

func encodeBitstreamFramesPitchWindowGain(t *testing.T, samples []int16, halfWindow int, xNum, xDen, yNum, yDen, gpcNum, gpcDen int32) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepPitchWindow(enc, 0, halfWindow, xNum, xDen, yNum, yDen, gpcNum, gpcDen)
		closedloopStepPitchWindow(enc, 1, halfWindow, xNum, xDen, yNum, yDen, gpcNum, gpcDen)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

type closedLoopSurfaceMode struct {
	name             string
	speechStart      int
	residualA        string
	filterA          string
	targetMem        string
	targetMode       string
	residualMem      string
	residualExt      string
	centerMode       string
	halfWindow       int
	xNum             int32
	xDen             int32
	yNum             int32
	yDen             int32
	acbMode          string
	pitchMode        string
	transmitFracZero bool
	fcbMode          string
	gainMode         string
	gpcNum           int32
	gpcDen           int32
}

func encodeBitstreamFramesClosedLoopSurface(t *testing.T, samples []int16, mode closedLoopSurfaceMode) []bitstream.Frame {
	t.Helper()
	enc := NewEncoder()
	frames := make([]bitstream.Frame, 0, len(samples)/FrameSamples)
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", off/FrameSamples, err)
		}
		_ = enc.openloopStep()
		closedloopStepSurfaceVariant(enc, 0, mode)
		closedloopStepSurfaceVariant(enc, 1, mode)
		var f bitstream.Frame
		enc.buildBitstreamFrame(&f)
		frames = append(frames, f)
	}
	return frames
}

func closedloopStepSurfaceVariant(e *Encoder, sub int, mode closedLoopSurfaceMode) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	aResidual := aHat
	if mode.residualA == "unquant" {
		aResidual = &e.aQ12Latest
	}
	aFilter := aHat
	if mode.filterA == "unquant" {
		aFilter = &e.aQ12Latest
	}
	var aPrime [11]int16
	useAPrime := mode.filterA == "aprime"
	if useAPrime {
		buildAPrimeDiagnostic(aFilter, &aPrime)
	}

	sStart := mode.speechStart + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var residualMemZero [10]int16
	residualMem := &e.lpResidualMemQ
	if mode.residualMem == "zero" {
		residualMem = &residualMemZero
	}
	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aResidual, residualMem, &r)

	targetMem := &e.swMemErr
	var targetMemZero [10]int16
	if mode.targetMem == "zero" {
		targetMem = &targetMemZero
	}
	if useAPrime {
		targetSignalFromWeightedLPDiagnostic(&aPrime, &r, targetMem, &x)
		impulseResponseFromWeightedLPDiagnostic(&aPrime, &h)
	} else {
		clpitch.TargetSignal(aFilter, &r, targetMem, &x)
		clpitch.ImpulseResponse(aFilter, &h)
	}
	if mode.targetMode == "weightedDirect" {
		copy(x[:], e.oldWspeech[63+40*sub:63+40*(sub+1)])
	}
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}

	var excSearch [closedLoopPitchSearchLen]int16
	copy(excSearch[:closedLoopPitchSearchHistory], e.oldExc[len(e.oldExc)-closedLoopPitchSearchHistory:])
	if mode.residualExt != "zero" {
		copy(excSearch[closedLoopPitchSearchHistory:], r[:])
	}
	exc := excSearch[:]
	if sub == 0 && mode.centerMode != "" {
		fullBestT, fullBestScore := bestFullRangePitchCenterForMode(&x, &h, exc, e.oldExc[:], mode.pitchMode)
		switch mode.centerMode {
		case "fullBest":
			centre = fullBestT
		case "submultipleFullBest":
			if isNearSubmultipleForDiag(int(centre), int(fullBestT)) {
				centre = fullBestT
			}
		case "submultipleRatio110", "submultipleRatio125", "submultipleRatio150", "submultipleRatio175", "submultipleRatio200",
			"harmonicRatio110", "harmonicRatio125", "harmonicRatio150",
			"fullRatio110", "fullRatio125", "fullRatio150", "fullRatio175", "fullRatio200",
			"dropRatio110", "dropRatio125", "dropRatio150", "dropRatio175", "dropRatio200":
			windowScore := bestWindowPitchScoreForMode(&x, &h, exc, e.oldExc[:], centre, sub, mode.halfWindow, mode.pitchMode)
			if shouldSwitchPitchCenterForDiag(mode.centerMode, centre, fullBestT, windowScore, fullBestScore) {
				centre = fullBestT
			}
		case "topHalf", "topDouble", "topPreferHalf", "topPreferDouble":
			centre = transformClosedLoopSurfaceCentre(centre, mode.centerMode)
		default:
			panic("unknown closed-loop diagnostic center mode: " + mode.centerMode)
		}
	}
	var intLag int16
	if mode.pitchMode == "decoderShortACB" {
		intLag = searchIntegerDecoderShortACB(&xb, exc, e.oldExc[:], centre, sub, mode.halfWindow)
	} else if mode.pitchMode == "decoderShortNorm" {
		intLag = searchIntegerDecoderShortNorm(&x, &h, exc, e.oldExc[:], centre, sub, mode.halfWindow)
	} else if mode.pitchMode == "normalized" {
		intLag = searchIntegerNormalized(&x, &h, exc, centre, sub, mode.halfWindow)
	} else if mode.pitchMode == "allowNegative" {
		intLag = searchIntegerAllowNegative(&xb, exc, centre, sub, mode.halfWindow)
	} else if mode.halfWindow > 0 {
		intLag = searchIntegerWindow(&xb, exc, centre, sub, mode.halfWindow)
	} else {
		intLag, _ = clpitch.SearchInteger(&xb, exc, centre, sub)
	}
	allowFrac := sub == 1 || intLag < 85
	frac := clpitch.RefineFraction(&xb, exc, intLag, allowFrac)
	if mode.pitchMode == "decoderShortACB" {
		frac = refineFractionDecoderShortACB(&xb, exc, e.oldExc[:], intLag, allowFrac)
	} else if mode.pitchMode == "decoderShortNorm" {
		frac = refineFractionDecoderShortNorm(&x, &h, exc, e.oldExc[:], intLag, allowFrac)
	} else if mode.pitchMode == "normalized" {
		frac = refineFractionNormalized(&x, &h, exc, intLag, allowFrac)
	}

	if mode.acbMode == "decoderAll" || (mode.acbMode == "decoderShort" && intLag < clpitch.SubframeLen) {
		pitchcore.AdaptiveCodebook(int(intLag), int(frac), e.oldExc[:], &v)
	} else {
		clpitch.AdaptiveVector(exc, intLag, frac, &v)
	}
	gp := clpitch.GpAndY(&x, &v, &h, &y)
	packFrac := frac
	if mode.transmitFracZero {
		packFrac = 0
	}

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = packFrac
		e.p1 = clpitch.EncodeP1(intLag, packFrac)
		e.p0 = clpitch.EncodeP0(e.p1)
	} else {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		e.intT2 = intLag
		e.frac2 = packFrac
		e.p2 = clpitch.EncodeP2(intLag, packFrac, tmin)
	}

	xNum, xDen := mode.xNum, mode.xDen
	yNum, yDen := mode.yNum, mode.yDen
	normalizeGainSweepMode(&xNum, &xDen)
	normalizeGainSweepMode(&yNum, &yDen)
	if mode.fcbMode == "annexA90" {
		fcbStepAnnexA90GainSearchScale(e, sub, &x, &y, &h, &v, gp, xNum, xDen, yNum, yDen, mode.gpcNum, mode.gpcDen)
	} else if mode.fcbMode == "sharpenH" {
		fcbStepSharpenedHGainSearchScale(e, sub, &x, &y, &h, &v, gp, xNum, xDen, yNum, yDen, mode.gpcNum, mode.gpcDen)
	} else {
		native := mode.gainMode == "native"
		rawLinear := mode.gainMode == "rawLinear"
		fullCost := mode.gainMode == "fullCost"
		fcbStepGainSearchScale(e, sub, &x, &y, &h, &v, gp, xNum, xDen, yNum, yDen, 1, 1, mode.gpcNum, mode.gpcDen, native, rawLinear, fullCost, mode.fcbMode)
	}
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func searchIntegerAllowNegative(xb *[clpitch.SubframeLen]int16, exc []int16, centre int16, sub int, halfWindow int) int16 {
	var kMin, kMax int
	if sub == 0 {
		if halfWindow <= 0 {
			halfWindow = 3
		}
		kMin = int(centre) - halfWindow
		if kMin < clpitch.PitchMinInt {
			kMin = clpitch.PitchMinInt
		}
		kMax = int(centre) + halfWindow
		if kMax > clpitch.PitchMaxInt {
			kMax = clpitch.PitchMaxInt
		}
	} else {
		tmin, tmax := clpitch.Subframe2Window(centre)
		kMin, kMax = int(tmin), int(tmax)
	}
	intLag := int16(kMin)
	base := len(exc) - clpitch.SubframeLen
	excBase := base - kMin
	var best int64
	for n := 0; n < clpitch.SubframeLen; n++ {
		best += int64(xb[n]) * int64(exc[excBase+n])
	}
	for k := kMin + 1; k <= kMax; k++ {
		excBase = base - k
		var acc int64
		for n := 0; n < clpitch.SubframeLen; n++ {
			acc += int64(xb[n]) * int64(exc[excBase+n])
		}
		if acc > best {
			best = acc
			intLag = int16(k)
		}
	}
	return intLag
}

func searchIntegerDecoderShortACB(xb *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, centre int16, sub int, halfWindow int) int16 {
	kMin, kMax := pitchSearchRange(centre, sub, halfWindow)
	intLag := int16(kMin)
	var best fixed.Word32
	for k := kMin; k <= kMax; k++ {
		var acc fixed.Word32
		if k < clpitch.SubframeLen {
			var v [clpitch.SubframeLen]int16
			pitchcore.AdaptiveCodebook(k, 0, oldExc, &v)
			for n := 0; n < clpitch.SubframeLen; n++ {
				acc = fixed.LMac(acc, xb[n], v[n])
			}
		} else {
			base := len(exc) - clpitch.SubframeLen - k
			for n := 0; n < clpitch.SubframeLen; n++ {
				acc = fixed.LMac(acc, xb[n], exc[base+n])
			}
		}
		if k == kMin || acc > best {
			best = acc
			intLag = int16(k)
		}
	}
	return intLag
}

func refineFractionDecoderShortACB(xb *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, intLag int16, allowFrac bool) int8 {
	if !allowFrac {
		return 0
	}
	if intLag >= clpitch.SubframeLen {
		return clpitch.RefineFraction(xb, exc, intLag, allowFrac)
	}
	bestFrac := int8(-1)
	bestRN := correlateDecoderACBAtFrac(xb, oldExc, int(intLag), -1)
	for _, frac := range [2]int8{0, +1} {
		rn := correlateDecoderACBAtFrac(xb, oldExc, int(intLag), frac)
		if rn > bestRN {
			bestRN = rn
			bestFrac = frac
		}
	}
	return bestFrac
}

func correlateDecoderACBAtFrac(xb *[clpitch.SubframeLen]int16, oldExc []int16, intLag int, frac int8) fixed.Word32 {
	var v [clpitch.SubframeLen]int16
	pitchcore.AdaptiveCodebook(intLag, int(frac), oldExc, &v)
	var acc fixed.Word32
	for n := 0; n < clpitch.SubframeLen; n++ {
		acc = fixed.LMac(acc, xb[n], v[n])
	}
	return acc
}

func searchIntegerDecoderShortNorm(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, centre int16, sub int, halfWindow int) int16 {
	kMin, kMax := pitchSearchRange(centre, sub, halfWindow)
	bestLag := int16(kMin)
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		score := normalizedPitchScoreDecoderShort(x, h, exc, oldExc, int16(k), 0)
		if score > bestScore {
			bestScore = score
			bestLag = int16(k)
		}
	}
	return bestLag
}

func refineFractionDecoderShortNorm(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, intLag int16, allowFrac bool) int8 {
	if !allowFrac {
		return 0
	}
	bestFrac := int8(-1)
	bestScore := normalizedPitchScoreDecoderShort(x, h, exc, oldExc, intLag, -1)
	for _, frac := range [2]int8{0, 1} {
		score := normalizedPitchScoreDecoderShort(x, h, exc, oldExc, intLag, frac)
		if score > bestScore {
			bestScore = score
			bestFrac = frac
		}
	}
	return bestFrac
}

func normalizedPitchScoreDecoderShort(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, intLag int16, frac int8) float64 {
	var v [clpitch.SubframeLen]int16
	if intLag < clpitch.SubframeLen {
		pitchcore.AdaptiveCodebook(int(intLag), int(frac), oldExc, &v)
	} else {
		clpitch.AdaptiveVector(exc, intLag, frac, &v)
	}
	return normalizedPitchScoreForVector(x, h, &v)
}

func bestFullRangePitchCenterForMode(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, pitchMode string) (int16, float64) {
	bestLag := int16(clpitch.PitchMinInt)
	bestScore := math.Inf(-1)
	for k := clpitch.PitchMinInt; k <= clpitch.PitchMaxInt; k++ {
		score := normalizedPitchScoreForMode(x, h, exc, oldExc, int16(k), 0, pitchMode)
		if score > bestScore {
			bestScore = score
			bestLag = int16(k)
		}
	}
	return bestLag, bestScore
}

func bestWindowPitchScoreForMode(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, centre int16, sub int, halfWindow int, pitchMode string) float64 {
	kMin, kMax := pitchSearchRange(centre, sub, halfWindow)
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		fracs, n := externalPitchCandidateFracs(sub, int16(k))
		for i := 0; i < n; i++ {
			score := normalizedPitchScoreForMode(x, h, exc, oldExc, int16(k), fracs[i], pitchMode)
			if score > bestScore {
				bestScore = score
			}
		}
	}
	return bestScore
}

func normalizedPitchScoreForMode(x, h *[clpitch.SubframeLen]int16, exc []int16, oldExc []int16, intLag int16, frac int8, pitchMode string) float64 {
	if pitchMode == "decoderShortNorm" {
		return normalizedPitchScoreDecoderShort(x, h, exc, oldExc, intLag, frac)
	}
	return normalizedPitchScore(x, h, exc, intLag, frac)
}

func shouldSwitchPitchCenterForDiag(mode string, centre, fullBestT int16, windowScore, fullBestScore float64) bool {
	ratio := 1.0
	switch {
	case strings.HasSuffix(mode, "110"):
		ratio = 1.10
	case strings.HasSuffix(mode, "125"):
		ratio = 1.25
	case strings.HasSuffix(mode, "150"):
		ratio = 1.50
	case strings.HasSuffix(mode, "175"):
		ratio = 1.75
	case strings.HasSuffix(mode, "200"):
		ratio = 2.00
	default:
		return false
	}
	if math.IsInf(fullBestScore, -1) || math.IsInf(windowScore, -1) || fullBestScore < windowScore*ratio {
		return false
	}
	if strings.HasPrefix(mode, "fullRatio") {
		return true
	}
	if strings.HasPrefix(mode, "dropRatio") {
		return int(fullBestT)+20 <= int(centre)
	}
	submultiple := isNearSubmultipleForDiag(int(centre), int(fullBestT))
	if strings.HasPrefix(mode, "submultipleRatio") {
		return submultiple
	}
	harmonic := submultiple || isNearSubmultipleForDiag(int(fullBestT), int(centre))
	return harmonic
}

func isNearSubmultipleForDiag(higher, lower int) bool {
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

func transformClosedLoopSurfaceCentre(centre int16, mode string) int16 {
	t := int(centre)
	switch mode {
	case "topHalf":
		t = (t + 1) / 2
	case "topDouble":
		t *= 2
	case "topPreferHalf":
		if t >= 40 {
			t = (t + 1) / 2
		}
	case "topPreferDouble":
		if t <= 71 {
			t *= 2
		}
	default:
		panic("unknown closed-loop surface centre transform")
	}
	if t < clpitch.PitchMinInt {
		t = clpitch.PitchMinInt
	}
	if t > clpitch.PitchMaxInt {
		t = clpitch.PitchMaxInt
	}
	return int16(t)
}

func searchIntegerNormalized(x, h *[clpitch.SubframeLen]int16, exc []int16, centre int16, sub int, halfWindow int) int16 {
	kMin, kMax := pitchSearchRange(centre, sub, halfWindow)
	bestLag := int16(kMin)
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		score := normalizedPitchScore(x, h, exc, int16(k), 0)
		if score > bestScore {
			bestScore = score
			bestLag = int16(k)
		}
	}
	return bestLag
}

func refineFractionNormalized(x, h *[clpitch.SubframeLen]int16, exc []int16, intLag int16, allowFrac bool) int8 {
	if !allowFrac {
		return 0
	}
	bestFrac := int8(-1)
	bestScore := normalizedPitchScore(x, h, exc, intLag, -1)
	for _, frac := range [2]int8{0, 1} {
		score := normalizedPitchScore(x, h, exc, intLag, frac)
		if score > bestScore {
			bestScore = score
			bestFrac = frac
		}
	}
	return bestFrac
}

func normalizedPitchScore(x, h *[clpitch.SubframeLen]int16, exc []int16, intLag int16, frac int8) float64 {
	var v [clpitch.SubframeLen]int16
	clpitch.AdaptiveVector(exc, intLag, frac, &v)
	return normalizedPitchScoreForVector(x, h, &v)
}

func normalizedPitchScoreForVector(x, h, v *[clpitch.SubframeLen]int16) float64 {
	var y [clpitch.SubframeLen]int16
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc int64
		for i := 0; i <= n; i++ {
			acc += int64(v[i]) * int64(h[n-i])
		}
		y[n] = saturateInt64ToInt16ForDiag(roundShift64ForDiag(acc, 12))
	}
	var corr, energy float64
	for n := 0; n < clpitch.SubframeLen; n++ {
		yn := float64(y[n])
		corr += float64(x[n]) * yn
		energy += yn * yn
	}
	if corr <= 0 || energy <= 0 {
		return math.Inf(-1)
	}
	return (corr * corr) / energy
}

func pitchSearchRange(centre int16, sub int, halfWindow int) (kMin, kMax int) {
	if sub == 0 {
		if halfWindow <= 0 {
			halfWindow = 3
		}
		kMin = int(centre) - halfWindow
		if kMin < clpitch.PitchMinInt {
			kMin = clpitch.PitchMinInt
		}
		kMax = int(centre) + halfWindow
		if kMax > clpitch.PitchMaxInt {
			kMax = clpitch.PitchMaxInt
		}
		return kMin, kMax
	}
	tmin, tmax := clpitch.Subframe2Window(centre)
	return int(tmin), int(tmax)
}

func roundShift64ForDiag(v int64, shift uint) int64 {
	if shift == 0 {
		return v
	}
	add := int64(1 << (shift - 1))
	if v >= 0 {
		return (v + add) >> shift
	}
	return -(((-v) + add) >> shift)
}

func saturateInt64ToInt16ForDiag(v int64) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

func buildAPrimeDiagnostic(aHat, out *[11]int16) {
	gamma := [11]int16{32767, 24576, 18432, 13824, 10368, 7776, 5832, 4374, 3281, 2460, 1845}
	var aw [11]int16
	aw[0] = aHat[0]
	for i := 1; i <= 10; i++ {
		aw[i] = fixed.Mult(aHat[i], gamma[i])
	}
	out[0] = aw[0]
	out[1] = fixed.Saturate(int32(aw[1]) - int32(22938))
	for i := 2; i <= 10; i++ {
		out[i] = aw[i] - fixed.MultR(22938, aw[i-1])
	}
}

func targetSignalFromWeightedLPDiagnostic(aWeightedQ12 *[11]int16, residual *[clpitch.SubframeLen]int16, mem *[10]int16, x *[clpitch.SubframeLen]int16) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		acc := fixed.LMult(residual[n], aWeightedQ12[0])
		for i := 1; i <= 10; i++ {
			var xni int16
			if n-i >= 0 {
				xni = x[n-i]
			} else {
				xni = mem[10+n-i]
			}
			acc = fixed.LMsu(acc, aWeightedQ12[i], xni)
		}
		x[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func impulseResponseFromWeightedLPDiagnostic(aWeightedQ12 *[11]int16, h *[clpitch.SubframeLen]int16) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc fixed.Word32
		if n == 0 {
			acc = fixed.LMult(4096, aWeightedQ12[0])
		}
		limit := n
		if limit > 10 {
			limit = 10
		}
		for i := 1; i <= limit; i++ {
			acc = fixed.LMsu(acc, aWeightedQ12[i], h[n-i])
		}
		h[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func closedloopStepPitchWindow(e *Encoder, sub int, halfWindow int, xNum, xDen, yNum, yDen, gpcNum, gpcDen int32) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
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
	intLag := searchIntegerWindow(&xb, exc, centre, sub, halfWindow)
	frac := clpitch.RefineFraction(&xb, exc, intLag, sub == 1 || intLag < 85)

	clpitch.AdaptiveVector(exc, intLag, frac, &v)
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

	fcbStepGainSearchScale(e, sub, &x, &y, &h, &v, gp, xNum, xDen, yNum, yDen, 1, 1, gpcNum, gpcDen, false, false, false, "")
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func searchIntegerWindow(xb *[clpitch.SubframeLen]int16, exc []int16, centre int16, sub int, halfWindow int) int16 {
	if sub != 0 {
		t, _ := clpitch.SearchInteger(xb, exc, centre, sub)
		return t
	}
	kMin := int(centre) - halfWindow
	if kMin < clpitch.PitchMinInt {
		kMin = clpitch.PitchMinInt
	}
	kMax := int(centre) + halfWindow
	if kMax > clpitch.PitchMaxInt {
		kMax = clpitch.PitchMaxInt
	}
	intLag := int16(kMin)
	best := int64(0)
	base := len(exc) - clpitch.SubframeLen
	for k := kMin; k <= kMax; k++ {
		excBase := base - k
		var acc int64
		for n := 0; n < clpitch.SubframeLen; n++ {
			acc += int64(xb[n]) * int64(exc[excBase+n])
		}
		if acc > best {
			best = acc
			intLag = int16(k)
		}
	}
	return intLag
}

func closedloopStepPitchCenterMode(e *Encoder, sub int, mode string) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = transformPitchCentre(e.tOp, mode)
	} else {
		centre = e.intT1
	}

	var excSearch [closedLoopPitchSearchLen]int16
	exc := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	frac := clpitch.RefineFraction(&xb, exc, intLag, sub == 1 || intLag < 85)

	clpitch.AdaptiveVector(exc, intLag, frac, &v)
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

	fcbStepGainSearchScale(e, sub, &x, &y, &h, &v, gp, 1, 1, 1, 1, 1, 1, 4, 1, false, false, false, "")
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func transformPitchCentre(top int16, mode string) int16 {
	t := int(top)
	switch mode {
	case "identity":
	case "minus40":
		t -= 40
	case "minus20":
		t -= 20
	case "plus20":
		t += 20
	case "plus40":
		t += 40
	case "half":
		t = (t + 1) / 2
	case "double":
		t *= 2
	case "preferHalf":
		if t >= 40 {
			t = (t + 1) / 2
		}
	case "preferDouble":
		if t <= 71 {
			t *= 2
		}
	default:
		panic("unknown pitch centre mode")
	}
	if t < clpitch.PitchMinInt {
		t = clpitch.PitchMinInt
	}
	if t > clpitch.PitchMaxInt {
		t = clpitch.PitchMaxInt
	}
	return int16(t)
}

func closedloopStepFCBTargetGainScale(e *Encoder, sub int, fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen int32) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
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
	intLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	frac := clpitch.RefineFraction(&xb, exc, intLag, sub == 1 || intLag < 85)

	clpitch.AdaptiveVector(exc, intLag, frac, &v)
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

	fcbStepFCBTargetGainScale(e, sub, &x, &y, &h, &v, gp, fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func closedloopStepFCBGainAwareRerank(e *Encoder, sub int, topK int, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen int32) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 80 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
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
	intLag := e.searchPitchNormalizedAdaptive(&x, &h, exc, centre, sub)
	frac := e.refinePitchNormalizedAdaptive(&x, &h, exc, intLag, sub == 1 || intLag < 85, sub)

	e.adaptiveVectorForSynthesis(exc, intLag, frac, &v)
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

	fcbStepGainAwareRerank(e, sub, &x, &y, &h, &v, gp, topK, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func fcbStepFCBTargetGainScale(
	e *Encoder,
	sub int,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	fcbGpNum, fcbGpDen, gpcNum, gpcDen, searchXNum, searchXDen, searchYNum, searchYDen int32,
) {
	const N = clpitch.SubframeLen
	var xPrime [N]int16
	gpForFCB := saturateInt32ToInt16(scaleInt32Ratio(int32(gpUnq), fcbGpNum, fcbGpDen))
	fcbsearch.AdjustedTarget(x, y, gpForFCB, &xPrime)

	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}
	hSearch := productionFCBSearchImpulse(h, intLag, e.prevGpQ14)

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	xSearch, ySearch := *x, *y
	scaleVectorInt16(&xSearch, searchXNum, searchXDen)
	scaleVectorInt16(&ySearch, searchYNum, searchYDen)
	gpcPredQ12 := scaleInt32Ratio(gainquant.PredictedGcQ12(&e.pastQuaEn, &c), gpcNum, gpcDen)
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcPredQ12)
	gpHatQ14 = gainquant.Tame(gpHatQ14, &e.oldExc)

	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
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

	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
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
	e.prevTaming = false
}

func fcbStepGainAwareRerank(
	e *Encoder,
	sub int,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	topK int,
	gpcNum, gpcDen int32,
	searchXNum, searchXDen, searchYNum, searchYDen int32,
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
	hSearch := productionFCBSearchImpulse(h, intLag, e.prevGpQ14)

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	cands := fcbTopKCandidatesDiag(&dAbs, &phi, topK)
	if len(cands) == 0 {
		cands = []fcbCandidateDiag{{pos: [4]int8{0, 1, 2, 3}}}
	}

	bestCost := int64(1<<63 - 1)
	var bestPos [4]int8
	var bestC, bestZ [N]int16
	var bestGA, bestGB uint8
	var bestGp int16
	var bestGamma int32
	var bestMant int16
	var bestExp int8
	bestTaming := false
	for _, cand := range cands {
		var c [N]int16
		fcbsearch.BuildCode(&cand.pos, &signs, intLag, e.prevGpQ14, &c)
		var z [N]int16
		fcbsearch.FilterCode(&c, h, &z)
		gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
		gpcPredQ12 = scaleInt32Ratio(gpcPredQ12, gpcNum, gpcDen)
		xSearch, ySearch := *x, *y
		scaleVectorInt16(&xSearch, searchXNum, searchXDen)
		scaleVectorInt16(&ySearch, searchYNum, searchYDen)
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcPredQ12)
		gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
		taming := gpTamed != gpHatQ14
		gpHatQ14 = gpTamed
		_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
		cost := gainResidualEnergyQ0(x, y, &z, gpHatQ14, gcMantQ14, gcExp)
		if cost < bestCost {
			bestCost = cost
			bestPos = cand.pos
			bestC = c
			bestZ = z
			bestGA, bestGB = gaPhys, gbPhys
			bestGp, bestGamma = gpHatQ14, gammaCQ13
			bestMant, bestExp = gcMantQ14, gcExp
			bestTaming = taming
		}
	}

	s := fcbsearch.PackS(&bestPos, &signs)
	cPacked := fcbsearch.PackC(&bestPos)
	gaBits, gbBits := gainquant.PackGains(bestGA, bestGB)

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
		gpY := applyGainQ14ToQ0(bestGp, y[n])
		gcZ := applyGcToQ12(bestMant, bestExp, bestZ[n])
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-N], e.oldExc[N:])
	base := len(e.oldExc) - N
	var uHat [N]int16
	synth.BuildExcitation(bestGp, bestMant, bestExp, v, &bestC, &uHat)
	copy(e.oldExc[base:], uHat[:])

	gainquant.UpdatePastQuaEn(&e.pastQuaEn, bestGamma)
	e.prevGpQ14 = bestGp
	e.prevTaming = bestTaming
}

type fcbCandidateDiag struct {
	pos   [4]int8
	c2, e int64
}

func fcbTopKCandidatesDiag(dAbs *[clpitch.SubframeLen]int32, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32, topK int) []fcbCandidateDiag {
	if topK <= 0 {
		topK = 1
	}
	top := make([]fcbCandidateDiag, 0, topK)
	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2Diag {
				d012 := d01 + int64(dAbs[m2])
				e012 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3Diag {
					c := d012 + int64(dAbs[m3])
					e := e012 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if e <= 0 {
						continue
					}
					cand := fcbCandidateDiag{pos: [4]int8{m0, m1, m2, m3}, c2: squareSaturatingInt64ForDiag(c), e: e}
					insertTopFCBCandidateDiag(&top, topK, cand)
				}
			}
		}
	}
	return top
}

func insertTopFCBCandidateDiag(top *[]fcbCandidateDiag, limit int, cand fcbCandidateDiag) {
	n := len(*top)
	if n == limit && !ratioGreaterDiag(cand.c2, cand.e, (*top)[n-1].c2, (*top)[n-1].e) {
		return
	}
	pos := n
	for pos > 0 && ratioGreaterDiag(cand.c2, cand.e, (*top)[pos-1].c2, (*top)[pos-1].e) {
		pos--
	}
	if n < limit {
		*top = append(*top, fcbCandidateDiag{})
		n++
	}
	copy((*top)[pos+1:n], (*top)[pos:n-1])
	(*top)[pos] = cand
}

func closedloopStepGainSearchScale(e *Encoder, sub int, xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen int32, native, rawLinear, fullCost bool) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
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
	intLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	frac := clpitch.RefineFraction(&xb, exc, intLag, sub == 1 || intLag < 85)

	clpitch.AdaptiveVector(exc, intLag, frac, &v)
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

	fcbStepGainSearchScale(e, sub, &x, &y, &h, &v, gp, xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen, native, rawLinear, fullCost, "")
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func fcbStepGainSearchScale(
	e *Encoder,
	sub int,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	xNum, xDen, yNum, yDen, zNum, zDen, gpcNum, gpcDen int32,
	native bool,
	rawLinear bool,
	fullCost bool,
	fcbScoreMode string,
) {
	const N = clpitch.SubframeLen
	searchMode := fcbScoreMode
	usePlainH := false
	if strings.HasPrefix(searchMode, "plainH:") {
		usePlainH = true
		searchMode = strings.TrimPrefix(searchMode, "plainH:")
	} else if searchMode == "plainH" {
		usePlainH = true
		searchMode = ""
	}

	var xPrime [N]int16
	if searchMode == "targetTrunc" {
		adjustedTargetTruncForDiag(x, y, gpUnq, &xPrime)
	} else {
		fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)
	}

	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}
	hSearch := productionFCBSearchImpulse(h, intLag, e.prevGpQ14)
	if usePlainH {
		hSearch = *h
	}

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	if strings.HasPrefix(searchMode, "score:") {
		searchDepthFirstScoreModeForDiag(&dAbs, &phi, strings.TrimPrefix(searchMode, "score:"), &positions, &sumOut)
	} else if strings.HasPrefix(searchMode, "tracktop:") {
		searchDepthFirstTrackTopForDiag(&dAbs, &phi, strings.TrimPrefix(searchMode, "tracktop:"), &positions, &sumOut)
	} else if strings.HasPrefix(searchMode, "first3top:") {
		searchDepthFirstFirst3TopForDiag(&dAbs, &phi, strings.TrimPrefix(searchMode, "first3top:"), &positions, &sumOut)
	} else if strings.HasPrefix(searchMode, "thresholdscan:") {
		searchDepthFirstThresholdScanForDiag(&dAbs, &phi, strings.TrimPrefix(searchMode, "thresholdscan:"), &positions, &sumOut)
	} else {
		fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)
	}

	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	xSearch, ySearch, zSearch := *x, *y, z
	scaleVectorInt16(&xSearch, xNum, xDen)
	scaleVectorInt16(&ySearch, yNum, yDen)
	scaleVectorInt16(&zSearch, zNum, zDen)
	gpcPredQ12 := scaleInt32Ratio(gainquant.PredictedGcQ12(&e.pastQuaEn, &c), gpcNum, gpcDen)
	var gaPhys, gbPhys uint8
	var gpHatQ14 int16
	var gammaCQ13 int32
	if rawLinear {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateRawLinearExhaustive(&xSearch, &ySearch, &zSearch, gpcPredQ12)
	} else if native {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateNativeExhaustive(&e.pastQuaEn, &c, &xSearch, &ySearch, &zSearch)
	} else if fullCost {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateFullCostExhaustive(&xSearch, &ySearch, &zSearch, gpcPredQ12)
	} else {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = gainquant.SearchConjugate(&xSearch, &ySearch, &zSearch, gpcPredQ12)
	}
	gpHatQ14 = gainquant.Tame(gpHatQ14, &e.oldExc)

	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
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

	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
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
	e.prevTaming = false
}

func fcbStepSharpenedHGainSearchScale(
	e *Encoder,
	sub int,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	xNum, xDen, yNum, yDen, gpcNum, gpcDen int32,
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
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	xSearch, ySearch, zSearch := *x, *y, z
	scaleVectorInt16(&xSearch, xNum, xDen)
	scaleVectorInt16(&ySearch, yNum, yDen)
	gpcPredQ12 := scaleInt32Ratio(gainquant.PredictedGcQ12(&e.pastQuaEn, &c), gpcNum, gpcDen)
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &zSearch, gpcPredQ12)
	gpHatQ14 = gainquant.Tame(gpHatQ14, &e.oldExc)

	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
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

	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
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
	e.prevTaming = false
}

func fcbStepAnnexA90GainSearchScale(
	e *Encoder,
	sub int,
	x, y, h, v *[clpitch.SubframeLen]int16,
	gpUnq int16,
	xNum, xDen, yNum, yDen, gpcNum, gpcDen int32,
) {
	const N = clpitch.SubframeLen
	var xPrime [N]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, h, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	searchDepthFirstAnnexA90ForDiag(&dAbs, &phi, &positions, &sumOut)

	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}
	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	xSearch, ySearch, zSearch := *x, *y, z
	scaleVectorInt16(&xSearch, xNum, xDen)
	scaleVectorInt16(&ySearch, yNum, yDen)
	gpcPredQ12 := scaleInt32Ratio(gainquant.PredictedGcQ12(&e.pastQuaEn, &c), gpcNum, gpcDen)
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(&xSearch, &ySearch, &zSearch, gpcPredQ12)
	gpHatQ14 = gainquant.Tame(gpHatQ14, &e.oldExc)

	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
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

	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
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
	e.prevTaming = false
}

type fcbFirst3Diag struct {
	m0, m1, m2 int8
	c          int64
	e          int64
}

func searchDepthFirstAnnexA90ForDiag(
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	first3 := make([]fcbFirst3Diag, 0, 8*8*8)
	var sumC, maxC int64
	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2Diag {
				c := d01 + int64(dAbs[m2])
				e := e01 + int64(phi[m2][m2]) + int64(phi[m0][m2]) + int64(phi[m1][m2])
				first3 = append(first3, fcbFirst3Diag{m0: m0, m1: m1, m2: m2, c: c, e: e})
				sumC += c
				if c > maxC {
					maxC = c
				}
			}
		}
	}
	avgC := sumC / int64(len(first3))
	threshold := avgC + (4*(maxC-avgC))/10
	sort.Slice(first3, func(i, j int) bool {
		if first3[i].c != first3[j].c {
			return first3[i].c > first3[j].c
		}
		if first3[i].m0 != first3[j].m0 {
			return first3[i].m0 < first3[j].m0
		}
		if first3[i].m1 != first3[j].m1 {
			return first3[i].m1 < first3[j].m1
		}
		return first3[i].m2 < first3[j].m2
	})

	bestPos := [4]int8{0, 1, 2, 3}
	var bestC2, bestE int64
	found := false
	entered := 0
	for _, p := range first3 {
		if p.c < threshold {
			continue
		}
		if entered >= 90 {
			break
		}
		entered++
		for _, m3 := range track3Diag {
			c := p.c + int64(dAbs[m3])
			e := p.e + int64(phi[m3][m3]) +
				int64(phi[p.m0][m3]) + int64(phi[p.m1][m3]) + int64(phi[p.m2][m3])
			if e <= 0 {
				continue
			}
			c2 := c * c
			if !found || ratioGreaterDiag(c2, e, bestC2, bestE) {
				found = true
				bestC2, bestE = c2, e
				bestPos = [4]int8{p.m0, p.m1, p.m2, m3}
			}
		}
	}
	if !found {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	*positions = bestPos
	sumOut[0], sumOut[1] = bestC2, bestE
}

func searchDepthFirstScoreModeForDiag(
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	mode string,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	bestPos := [4]int8{0, 1, 2, 3}
	bestScore := math.Inf(-1)
	var bestC, bestE int64
	found := false

	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2Diag {
				d012 := d01 + int64(dAbs[m2])
				e012 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3Diag {
					c := d012 + int64(dAbs[m3])
					e := e012 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if c <= 0 || e <= 0 {
						continue
					}
					score := fcbScoreForDiag(float64(c), float64(e), mode)
					if !found || score > bestScore {
						found = true
						bestScore = score
						bestC, bestE = c, e
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}

	if !found {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	*positions = bestPos
	sumOut[0] = squareSaturatingInt64ForDiag(bestC)
	sumOut[1] = bestE
}

func searchDepthFirstTrackTopForDiag(
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	mode string,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	limit := 4
	switch mode {
	case "1":
		limit = 1
	case "2":
		limit = 2
	case "3":
		limit = 3
	case "4":
		limit = 4
	case "6":
		limit = 6
	case "8":
		limit = 8
	default:
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}

	t0 := topTrackForDiag(track0Diag[:], dAbs, limit)
	t1 := topTrackForDiag(track1Diag[:], dAbs, limit)
	t2 := topTrackForDiag(track2Diag[:], dAbs, limit)
	t3 := topTrackForDiag(track3Diag[:], dAbs, limit*2)

	bestPos := [4]int8{0, 1, 2, 3}
	var bestC, bestE int64
	found := false
	for _, m0 := range t0 {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range t1 {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range t2 {
				d012 := d01 + int64(dAbs[m2])
				e012 := e01 + int64(phi[m2][m2]) +
					int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range t3 {
					c := d012 + int64(dAbs[m3])
					e := e012 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) +
						int64(phi[m2][m3])
					if e <= 0 {
						continue
					}
					if !found || ratioGreaterDiag(c, e, bestC, bestE) {
						found = true
						bestC, bestE = c, e
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}

	if !found {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	*positions = bestPos
	sumOut[0] = squareSaturatingInt64ForDiag(bestC)
	sumOut[1] = bestE
}

func searchDepthFirstFirst3TopForDiag(
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	mode string,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	limit, err := strconv.Atoi(mode)
	if err != nil || limit <= 0 {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}

	first3 := make([]fcbFirst3Diag, 0, len(track0Diag)*len(track1Diag)*len(track2Diag))
	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2Diag {
				c := d01 + int64(dAbs[m2])
				e := e01 + int64(phi[m2][m2]) + int64(phi[m0][m2]) + int64(phi[m1][m2])
				if e <= 0 {
					continue
				}
				first3 = append(first3, fcbFirst3Diag{m0: m0, m1: m1, m2: m2, c: c, e: e})
			}
		}
	}
	sort.SliceStable(first3, func(i, j int) bool {
		ic2 := squareSaturatingInt64ForDiag(first3[i].c)
		jc2 := squareSaturatingInt64ForDiag(first3[j].c)
		if ratioGreaterDiag(ic2, first3[i].e, jc2, first3[j].e) {
			return true
		}
		if ratioGreaterDiag(jc2, first3[j].e, ic2, first3[i].e) {
			return false
		}
		if first3[i].m0 != first3[j].m0 {
			return first3[i].m0 < first3[j].m0
		}
		if first3[i].m1 != first3[j].m1 {
			return first3[i].m1 < first3[j].m1
		}
		return first3[i].m2 < first3[j].m2
	})
	if limit > len(first3) {
		limit = len(first3)
	}

	bestPos := [4]int8{0, 1, 2, 3}
	var bestC2, bestE int64
	found := false
	for _, p := range first3[:limit] {
		for _, m3 := range track3Diag {
			c := p.c + int64(dAbs[m3])
			e := p.e + int64(phi[m3][m3]) +
				int64(phi[p.m0][m3]) + int64(phi[p.m1][m3]) + int64(phi[p.m2][m3])
			if e <= 0 {
				continue
			}
			c2 := squareSaturatingInt64ForDiag(c)
			if !found || ratioGreaterDiag(c2, e, bestC2, bestE) {
				found = true
				bestC2, bestE = c2, e
				bestPos = [4]int8{p.m0, p.m1, p.m2, m3}
			}
		}
	}
	if !found {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	*positions = bestPos
	sumOut[0] = bestC2
	sumOut[1] = bestE
}

func searchDepthFirstThresholdScanForDiag(
	dAbs *[clpitch.SubframeLen]int32,
	phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32,
	mode string,
	positions *[4]int8,
	sumOut *[2]int64,
) {
	limit, err := strconv.Atoi(mode)
	if err != nil || limit <= 0 {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}

	var sumC, maxC int64
	first3Count := int64(0)
	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			for _, m2 := range track2Diag {
				c := d01 + int64(dAbs[m2])
				sumC += c
				first3Count++
				if c > maxC {
					maxC = c
				}
			}
		}
	}
	if first3Count == 0 {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	avgC := sumC / first3Count
	threshold := avgC + (4*(maxC-avgC))/10

	bestPos := [4]int8{0, 1, 2, 3}
	var bestC2, bestE int64
	found := false
	entered := 0
scan:
	for _, m0 := range track0Diag {
		d0 := int64(dAbs[m0])
		e0 := int64(phi[m0][m0])
		for _, m1 := range track1Diag {
			d01 := d0 + int64(dAbs[m1])
			e01 := e0 + int64(phi[m1][m1]) + int64(phi[m0][m1])
			for _, m2 := range track2Diag {
				c3 := d01 + int64(dAbs[m2])
				if c3 < threshold {
					continue
				}
				if entered >= limit {
					break scan
				}
				entered++
				e3 := e01 + int64(phi[m2][m2]) + int64(phi[m0][m2]) + int64(phi[m1][m2])
				for _, m3 := range track3Diag {
					c := c3 + int64(dAbs[m3])
					e := e3 + int64(phi[m3][m3]) +
						int64(phi[m0][m3]) + int64(phi[m1][m3]) + int64(phi[m2][m3])
					if e <= 0 {
						continue
					}
					c2 := squareSaturatingInt64ForDiag(c)
					if !found || ratioGreaterDiag(c2, e, bestC2, bestE) {
						found = true
						bestC2, bestE = c2, e
						bestPos = [4]int8{m0, m1, m2, m3}
					}
				}
			}
		}
	}
	if !found {
		fcbsearch.SearchDepthFirst(dAbs, phi, positions, sumOut)
		return
	}
	*positions = bestPos
	sumOut[0] = bestC2
	sumOut[1] = bestE
}

func topTrackForDiag(track []int8, dAbs *[clpitch.SubframeLen]int32, limit int) []int8 {
	if limit > len(track) {
		limit = len(track)
	}
	out := make([]int8, len(track))
	copy(out, track)
	sort.SliceStable(out, func(i, j int) bool {
		di := dAbs[out[i]]
		dj := dAbs[out[j]]
		if di == dj {
			return out[i] < out[j]
		}
		return di > dj
	})
	return out[:limit]
}

func adjustedTargetTruncForDiag(x, y *[clpitch.SubframeLen]int16, gp int16, xPrime *[clpitch.SubframeLen]int16) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		prod := int32(gp) * int32(y[n])
		xPrime[n] = saturateInt32ToInt16(int32(x[n]) - (prod >> 14))
	}
}

func fcbScoreForDiag(c, e float64, mode string) float64 {
	switch mode {
	case "p0":
		return 2 * math.Log(c)
	case "p05":
		return 2*math.Log(c) - 0.5*math.Log(e)
	case "p15":
		return 2*math.Log(c) - 1.5*math.Log(e)
	case "p2":
		return 2*math.Log(c) - 2*math.Log(e)
	default:
		return 2*math.Log(c) - math.Log(e)
	}
}

func ratioGreaterDiag(a, b, c, d int64) bool {
	hi1, lo1 := bits.Mul64(uint64(a), uint64(d))
	hi2, lo2 := bits.Mul64(uint64(c), uint64(b))
	if hi1 != hi2 {
		return hi1 > hi2
	}
	return lo1 > lo2
}

func squareSaturatingInt64ForDiag(v int64) int64 {
	if v < 0 {
		v = -v
	}
	const maxSqrtInt64 int64 = 3037000499
	if v > maxSqrtInt64 {
		return 1<<63 - 1
	}
	return v * v
}

var (
	track0Diag = [8]int8{0, 5, 10, 15, 20, 25, 30, 35}
	track1Diag = [8]int8{1, 6, 11, 16, 21, 26, 31, 36}
	track2Diag = [8]int8{2, 7, 12, 17, 22, 27, 32, 37}
	track3Diag = [16]int8{3, 4, 8, 9, 13, 14, 18, 19, 23, 24, 28, 29, 33, 34, 38, 39}
)

func scaleInt32Ratio(v, num, den int32) int32 {
	if den == 0 {
		panic("zero denominator")
	}
	x := int64(v) * int64(num)
	if den != 1 {
		if x >= 0 {
			x += int64(den / 2)
		} else {
			x -= int64(den / 2)
		}
		x /= int64(den)
	}
	const (
		maxInt32 = int64(1<<31 - 1)
		minInt32 = -1 << 31
	)
	if x > maxInt32 {
		return int32(maxInt32)
	}
	if x < minInt32 {
		return int32(minInt32)
	}
	return int32(x)
}

func searchConjugateNativeExhaustive(past *[4]int16, c, x, y, z *[40]int16) (ga, gb uint8, gpQ14 int16, gammaCQ13 int32) {
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp, gcMant, gcExp := gainquant.Reconstruct(past, c, gai, gbi)
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

func searchConjugateFullCostExhaustive(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14 int16, gammaCQ13 int32) {
	var A, B, C, D, F int64
	for i := 0; i < 40; i++ {
		xi := int64(x[i])
		yi := int64(y[i])
		zi := int64(z[i])
		A += (yi * yi) << 24
		B += zi * zi
		C += (yi * zi) << 12
		D += (xi * yi) << 24
		F += (xi * zi) << 12
	}
	maxAbs := absInt64Diagnostic(A)
	for _, v := range [...]int64{B, C, D, F} {
		if a := absInt64Diagnostic(v); a > maxAbs {
			maxAbs = a
		}
	}
	var nshift uint
	if maxAbs > 0 {
		blen := uint(bits.Len64(uint64(maxAbs)))
		if blen > 14 {
			nshift = blen - 14
		}
	}
	if nshift > 0 {
		A >>= nshift
		B >>= nshift
		C >>= nshift
		D >>= nshift
		F >>= nshift
	}

	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp := int64(fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0]))))
			gamma := int64(gainGammaQ13(gai, gbi))
			gc := (gamma * int64(gpcPredQ12)) >> 13
			cost := gp * gp * A
			cost += (gc * gc * B) << 4
			cost += (2 * gp * gc * C) << 2
			cost -= (2 * gp * D) << 14
			cost -= (2 * gc * F) << 16
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = int16(gp)
				gammaCQ13 = int32(gamma)
			}
		}
	}
	return
}

func searchConjugateRawLinearExhaustive(x, y, z *[40]int16, gpcPredQ12 int32) (ga, gb uint8, gpQ14 int16, gammaCQ13 int32) {
	var A, B, C, D, F int64
	for i := 0; i < 40; i++ {
		xi := int64(x[i])
		yi := int64(y[i])
		zi := int64(z[i])
		A += yi * yi
		B += zi * zi
		C += yi * zi
		D += xi * yi
		F += xi * zi
	}
	maxAbs := absInt64Diagnostic(A)
	for _, v := range [...]int64{B, C, D, F} {
		if a := absInt64Diagnostic(v); a > maxAbs {
			maxAbs = a
		}
	}
	for maxAbs > 1<<14 {
		A >>= 1
		B >>= 1
		C >>= 1
		D >>= 1
		F >>= 1
		maxAbs >>= 1
	}

	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp := int64(fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][0]) + int32(tables.GainGBK2[gbi][0]))))
			gamma := int64(gainGammaQ13(gai, gbi))
			gc := (gamma * int64(gpcPredQ12)) >> 13
			cost := gp * gp * A
			cost += (gc * gc * B) << 4
			cost += (2 * gp * gc * C) << 2
			cost -= (2 * gp * D) << 14
			cost -= (2 * gc * F) << 16
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = int16(gp)
				gammaCQ13 = int32(gamma)
			}
		}
	}
	return
}

func absInt64Diagnostic(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
