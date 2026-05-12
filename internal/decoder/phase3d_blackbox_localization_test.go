package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3dBlackBoxLocalization_SPEECHAndAsterisk is an opt-in
// black-box decoder-quality localizer for the no-stage-oracle path.
//
// It uses only shipped bitstream/PCM vectors and local implementation taps:
// SPEECH.BIT -> Decoder -> PCM is compared against SPEECH.PST. When an
// external Asterisk payload and ffmpeg executable are available, the same
// stage proxies are compared against ffmpeg black-box decode. The test is
// informational and must not be treated as a conformance oracle.
type blackboxVariant struct {
	name    string
	samples []int16
	note    string
}

func TestPhase3dBlackBoxLocalization_SPEECHAndAsterisk(t *testing.T) {
	if os.Getenv("G729_DECODER_BLACKBOX_LOCALIZE") != "1" {
		t.Skip("set G729_DECODER_BLACKBOX_LOCALIZE=1 to run black-box decoder localization")
	}

	bitPath := vectorPath("SPEECH.BIT")
	pstPath := vectorPath("SPEECH.PST")
	inPath := vectorPath("SPEECH.IN")
	ensureTestdataPresent(t, bitPath, pstPath, inPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}

	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if inf := len(inData) / (2 * frameSamples); inf < frames {
		frames = inf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	totalSamples := frames * frameSamples
	refPST := blackboxReadPCM16LE(t, pstData, totalSamples)
	refIN := blackboxReadPCM16LE(t, inData, totalSamples)

	stages := blackboxDecodeSpeechStages(t, bitData, frames)
	noPostfilter := blackboxDecodeNoPostfilter(t, bitData, frames)

	variants := []blackboxVariant{
		{name: "production", samples: stages.production, note: "Decode output: synth -> postfilter -> HP -> final output gain recovery"},
		{name: "hp_pre_scale", samples: stages.hpPreScale, note: "HP output before final gain recovery"},
		{name: "postfilter_x2_no_hp", samples: stages.postfilterX2, note: "postfilter output scaled x2, HP bypass proxy"},
		{name: "synth_x2_no_pf_hp", samples: stages.synthX2, note: "synthesis output scaled x2, postfilter+HP bypass proxy"},
		{name: "excitation_x2_proxy", samples: stages.excitationX2, note: "excitation scaled x2, not PCM but upstream shape proxy"},
		{name: "pitch_contrib_x2_proxy", samples: stages.pitchContribX2, note: "adaptive contribution proxy"},
		{name: "fixed_contrib_x2_proxy", samples: stages.fixedContribX2, note: "fixed contribution proxy"},
		{name: "no_postfilter_hp_x2", samples: noPostfilter, note: "structural DecodeFrameNoPostfilter: synth -> HP -> x2"},
		{name: "production_best_scale", samples: scaleInt16(stages.production, leastSquaresScale(refPST, stages.production)), note: "least-squares scale-only variant"},
	}

	t.Logf("Phase 3d black-box decoder localization — SPEECH.BIT/SPEECH.PST (%d frames, %d samples)", frames, totalSamples)
	t.Logf("Reference orientation: SPEECH.IN rms=%.2f peak=%d dc=%.2f; SPEECH.PST rms=%.2f peak=%d dc=%.2f",
		diag4Rms(refIN), diag4MaxAbs(refIN), blackboxDC(refIN),
		diag4Rms(refPST), diag4MaxAbs(refPST), blackboxDC(refPST))
	metrics := blackboxLogVariantTable(t, "SPEECH.PST", refPST, variants)

	prod := metrics["production"]
	scaled := metrics["production_best_scale"]
	noPF := metrics["no_postfilter_hp_x2"]
	synthOnly := metrics["synth_x2_no_pf_hp"]
	postNoHP := metrics["postfilter_x2_no_hp"]
	t.Logf("")
	t.Logf("=== Worst production frames vs SPEECH.PST ===")
	for _, w := range blackboxWorstFrames(refPST, stages.production, 8) {
		t.Logf("frame=%4d snr=%7.2f corr=%7.3f refRMS=%8.2f gotRMS=%8.2f ratio=%7.3f errRMS=%8.2f",
			w.frame, w.snr, w.corr, w.refRMS, w.gotRMS, w.ratio, w.errRMS)
	}
	t.Logf("")
	t.Logf("=== Decision signals ===")
	t.Logf("scale-only delta: production gSNR@0 %.2f -> best-scale %.2f (Δ=%+.2f), corr %.3f -> %.3f",
		prod.globalSNR, scaled.globalSNR, scaled.globalSNR-prod.globalSNR, prod.corr, scaled.corr)
	t.Logf("alignment-only delta: gSNR@0 %.2f -> lag %+d gSNR %.2f (Δ=%+.2f); best corr lag %+d corr %.3f",
		prod.globalSNR, prod.bestSNRLag, prod.bestSNR, prod.bestSNR-prod.globalSNR, prod.bestCorrLag, prod.bestCorr)
	t.Logf("postfilter bypass delta: production gSNR %.2f corr %.3f -> no_postfilter gSNR %.2f corr %.3f",
		prod.globalSNR, prod.corr, noPF.globalSNR, noPF.corr)
	t.Logf("HP bypass proxy delta: production gSNR %.2f corr %.3f -> postfilter_x2_no_hp gSNR %.2f corr %.3f",
		prod.globalSNR, prod.corr, postNoHP.globalSNR, postNoHP.corr)
	t.Logf("synth proxy delta: production gSNR %.2f corr %.3f -> synth_x2_no_pf_hp gSNR %.2f corr %.3f",
		prod.globalSNR, prod.corr, synthOnly.globalSNR, synthOnly.corr)
	t.Logf("verdict: %s", blackboxVerdict(prod, scaled, noPF, postNoHP, synthOnly))

	asteriskPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(asteriskPath); err != nil {
		t.Logf("Asterisk payload metrics skipped: %v", err)
		return
	}
	ast, astFrames := blackboxDecodeRawG729(t, asteriskPath)
	astStats := blackboxSignalStats(ast, astFrames)
	t.Logf("")
	t.Logf("=== Asterisk raw G.729 payload decode metrics ===")
	t.Logf("path=%s", asteriskPath)
	t.Logf("frames=%d samples=%d rms=%.2f peak=%d dc=%.2f clipped=%d silenceFrames=%d lowEnergyFrames=%d zeroSamples=%d",
		astFrames, len(ast), astStats.rms, astStats.peak, astStats.dc, astStats.clipped,
		astStats.silenceFrames, astStats.lowEnergyFrames, astStats.zeroSamples)
	t.Logf("frameRMS p10=%.2f p50=%.2f p90=%.2f boundaryJump mean=%.2f max=%d",
		astStats.p10RMS, astStats.p50RMS, astStats.p90RMS, astStats.meanBoundaryJump, astStats.maxBoundaryJump)

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Logf("Asterisk FFmpeg-referenced stage metrics skipped: %v", err)
		return
	}
	raw, err := os.ReadFile(asteriskPath)
	if err != nil {
		t.Fatalf("read Asterisk raw payload for stage metrics: %v", err)
	}
	astRef := phase3uFFmpegDecodeRaw(t, asteriskPath, astFrames, "asterisk-phase3d")
	astStages := blackboxDecodeRawStages(t, raw, astFrames)
	astNoPostfilter := blackboxDecodeRawNoPostfilter(t, raw, astFrames)
	astEnhanced := phase3rDecodeRawEnhanced(t, asteriskPath, astFrames)
	astVariants := []blackboxVariant{
		{name: "production", samples: astStages.production, note: "Decode output: synth -> postfilter -> HP -> final output gain recovery"},
		{name: "enhanced_envelope", samples: astEnhanced, note: "DecodeEnvelopeRecovered experimental listening path"},
		{name: "hp_pre_scale", samples: astStages.hpPreScale, note: "HP output before final gain recovery"},
		{name: "postfilter_x2_no_hp", samples: astStages.postfilterX2, note: "postfilter output scaled x2, HP bypass proxy"},
		{name: "synth_x2_no_pf_hp", samples: astStages.synthX2, note: "synthesis output scaled x2, postfilter+HP bypass proxy"},
		{name: "excitation_x2_proxy", samples: astStages.excitationX2, note: "excitation scaled x2, not PCM but upstream shape proxy"},
		{name: "pitch_contrib_x2_proxy", samples: astStages.pitchContribX2, note: "adaptive contribution proxy"},
		{name: "fixed_contrib_x2_proxy", samples: astStages.fixedContribX2, note: "fixed contribution proxy"},
		{name: "no_postfilter_hp_x2", samples: astNoPostfilter, note: "structural DecodeFrameNoPostfilter: synth -> HP -> x2"},
		{name: "production_best_scale", samples: scaleInt16(astStages.production, leastSquaresScale(astRef, astStages.production)), note: "least-squares scale-only variant"},
	}
	astMetrics := blackboxLogVariantTable(t, "Asterisk raw payload vs FFmpeg black-box", astRef, astVariants)
	astProd := astMetrics["production"]
	astEnhancedM := astMetrics["enhanced_envelope"]
	astNoPF := astMetrics["no_postfilter_hp_x2"]
	astSynth := astMetrics["synth_x2_no_pf_hp"]
	t.Logf("")
	t.Logf("=== Asterisk stage decision signals ===")
	t.Logf("enhanced delta: production gSNR %.2f seg %.2f corr %.3f -> enhanced gSNR %.2f seg %.2f corr %.3f",
		astProd.globalSNR, astProd.segSNR, astProd.corr,
		astEnhancedM.globalSNR, astEnhancedM.segSNR, astEnhancedM.corr)
	t.Logf("postfilter bypass delta: production gSNR %.2f corr %.3f -> no_postfilter gSNR %.2f corr %.3f",
		astProd.globalSNR, astProd.corr, astNoPF.globalSNR, astNoPF.corr)
	t.Logf("synth proxy delta: production gSNR %.2f corr %.3f -> synth_x2_no_pf_hp gSNR %.2f corr %.3f",
		astProd.globalSNR, astProd.corr, astSynth.globalSNR, astSynth.corr)
}

type blackboxStages struct {
	production     []int16
	hpPreScale     []int16
	pfShortTermX2  []int16
	pfTiltX2       []int16
	postfilterX2   []int16
	synthX2        []int16
	excitationX2   []int16
	pitchContribX2 []int16
	fixedContribX2 []int16
}

func blackboxDecodeSpeechStages(t *testing.T, bitData []byte, frames int) blackboxStages {
	t.Helper()
	total := frames * frameSamples
	out := blackboxStages{
		production:     make([]int16, total),
		hpPreScale:     make([]int16, total),
		pfShortTermX2:  make([]int16, total),
		pfTiltX2:       make([]int16, total),
		postfilterX2:   make([]int16, total),
		synthX2:        make([]int16, total),
		excitationX2:   make([]int16, total),
		pitchContribX2: make([]int16, total),
		fixedContribX2: make([]int16, total),
	}

	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[taps] frame %d: %v", f, err)
		}
		taps, err := dec.DecodeWithTaps(packed[:])
		if err != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", f, err)
		}
		base := f * frameSamples
		copy(out.production[base:base+frameSamples], taps.Output[:])
		for sf := 0; sf < 2; sf++ {
			sub := &taps.Sub[sf]
			off := base + sf*subframeLen
			copy(out.hpPreScale[off:off+subframeLen], sub.HpOut[:])
			blackboxScale2Into(out.pfShortTermX2[off:off+subframeLen], sub.PFST[:])
			blackboxScale2Into(out.pfTiltX2[off:off+subframeLen], sub.PFT[:])
			blackboxScale2Into(out.postfilterX2[off:off+subframeLen], sub.SPf[:])
			blackboxScale2Into(out.synthX2[off:off+subframeLen], sub.S[:])
			blackboxScale2Into(out.excitationX2[off:off+subframeLen], sub.U[:])

			var zero [subframeLen]int16
			var pitchOnly [subframeLen]int16
			var fixedOnly [subframeLen]int16
			synth.BuildExcitation(sub.GpQ14, 0, 0, &sub.V, &zero, &pitchOnly)
			synth.BuildExcitation(0, sub.GainTaps.GcMantQ14, sub.GainTaps.GcExp, &zero, &sub.C, &fixedOnly)
			blackboxScale2Into(out.pitchContribX2[off:off+subframeLen], pitchOnly[:])
			blackboxScale2Into(out.fixedContribX2[off:off+subframeLen], fixedOnly[:])
		}
	}
	return out
}

func blackboxDecodeNoPostfilter(t *testing.T, bitData []byte, frames int) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[noPF] frame %d: %v", f, err)
		}
		if err := dec.DecodeFrameNoPostfilter(packed[:], out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("DecodeFrameNoPostfilter frame %d: %v", f, err)
		}
	}
	return out
}

func blackboxDecodeRawStages(t *testing.T, raw []byte, frames int) blackboxStages {
	t.Helper()
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}
	total := frames * frameSamples
	out := blackboxStages{
		production:     make([]int16, total),
		hpPreScale:     make([]int16, total),
		pfShortTermX2:  make([]int16, total),
		pfTiltX2:       make([]int16, total),
		postfilterX2:   make([]int16, total),
		synthX2:        make([]int16, total),
		excitationX2:   make([]int16, total),
		pitchContribX2: make([]int16, total),
		fixedContribX2: make([]int16, total),
	}

	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		taps, err := dec.DecodeWithTaps(raw[start : start+bitstream.FrameBytes])
		if err != nil {
			t.Fatalf("DecodeWithTaps raw frame %d: %v", f, err)
		}
		base := f * frameSamples
		copy(out.production[base:base+frameSamples], taps.Output[:])
		for sf := 0; sf < 2; sf++ {
			sub := &taps.Sub[sf]
			off := base + sf*subframeLen
			copy(out.hpPreScale[off:off+subframeLen], sub.HpOut[:])
			blackboxScale2Into(out.pfShortTermX2[off:off+subframeLen], sub.PFST[:])
			blackboxScale2Into(out.pfTiltX2[off:off+subframeLen], sub.PFT[:])
			blackboxScale2Into(out.postfilterX2[off:off+subframeLen], sub.SPf[:])
			blackboxScale2Into(out.synthX2[off:off+subframeLen], sub.S[:])
			blackboxScale2Into(out.excitationX2[off:off+subframeLen], sub.U[:])

			var zero [subframeLen]int16
			var pitchOnly [subframeLen]int16
			var fixedOnly [subframeLen]int16
			synth.BuildExcitation(sub.GpQ14, 0, 0, &sub.V, &zero, &pitchOnly)
			synth.BuildExcitation(0, sub.GainTaps.GcMantQ14, sub.GainTaps.GcExp, &zero, &sub.C, &fixedOnly)
			blackboxScale2Into(out.pitchContribX2[off:off+subframeLen], pitchOnly[:])
			blackboxScale2Into(out.fixedContribX2[off:off+subframeLen], fixedOnly[:])
		}
	}
	return out
}

func blackboxDecodeRawNoPostfilter(t *testing.T, raw []byte, frames int) []int16 {
	t.Helper()
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.DecodeFrameNoPostfilter(raw[start:start+bitstream.FrameBytes], out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("DecodeFrameNoPostfilter raw frame %d: %v", f, err)
		}
	}
	return out
}

func blackboxDecodeRawG729(t *testing.T, path string) ([]int16, int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw g729 payload: %v", err)
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * bitstream.FrameBytes
		if err := dec.Decode(raw[start:start+bitstream.FrameBytes], false, out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("Decode raw frame %d: %v", f, err)
		}
	}
	return out, frames
}

func blackboxLogVariantTable(t *testing.T, label string, ref []int16, variants []blackboxVariant) map[string]blackboxMetrics {
	t.Helper()
	t.Logf("")
	t.Logf("=== Stage / variant metrics vs %s (lag sweep -40..+40) ===", label)
	t.Logf("%-24s %9s %7s %9s %10s %10s %8s %8s %9s %10s %10s",
		"variant", "rms", "peak", "dc", "gSNR@0", "seg@0", "corr@0", "lagCorr", "corrBest", "lagSNR", "gSNRBest")
	t.Logf("%-24s %9s %7s %9s %10s %10s %8s %8s %9s %10s %10s",
		"-------", "---", "----", "--", "------", "-----", "------", "-------", "--------", "------", "---------")
	metrics := make(map[string]blackboxMetrics, len(variants))
	for _, v := range variants {
		m := blackboxMeasure(ref, v.samples, 40)
		metrics[v.name] = m
		t.Logf("%-24s %9.2f %7d %9.2f %10.2f %10.2f %8.3f %8d %9.3f %10d %10.2f",
			v.name, m.rms, m.peak, m.dc, m.globalSNR, m.segSNR, m.corr,
			m.bestCorrLag, m.bestCorr, m.bestSNRLag, m.bestSNR)
	}

	t.Logf("")
	t.Logf("=== %s variant notes ===", label)
	for _, v := range variants {
		t.Logf("%-24s %s", v.name, v.note)
	}
	return metrics
}

func blackboxReadPCM16LE(t *testing.T, data []byte, samples int) []int16 {
	t.Helper()
	if len(data) < samples*2 {
		t.Fatalf("PCM data too short: got %d bytes, want %d", len(data), samples*2)
	}
	out := make([]int16, samples)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i : 2*i+2]))
	}
	return out
}

func blackboxScale2Into(dst []int16, src []int16) {
	for i, s := range src {
		v := int(s) * 2
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		dst[i] = int16(v)
	}
}

type blackboxMetrics struct {
	rms         float64
	peak        int
	dc          float64
	globalSNR   float64
	segSNR      float64
	corr        float64
	bestCorrLag int
	bestCorr    float64
	bestSNRLag  int
	bestSNR     float64
	bestSegSNR  float64
}

func blackboxMeasure(ref, test []int16, maxLag int) blackboxMetrics {
	bestSNRLag, bestSNR, bestSeg := diag4BestAligned(ref, test, maxLag)
	bestCorrLag, bestCorr := blackboxBestCorr(ref, test, maxLag)
	return blackboxMetrics{
		rms:         diag4Rms(test),
		peak:        diag4MaxAbs(test),
		dc:          blackboxDC(test),
		globalSNR:   scaleProbeGlobalSNR(ref, test),
		segSNR:      scaleProbeSegSNR(ref, test),
		corr:        blackboxCorr(ref, test),
		bestCorrLag: bestCorrLag,
		bestCorr:    bestCorr,
		bestSNRLag:  bestSNRLag,
		bestSNR:     bestSNR,
		bestSegSNR:  bestSeg,
	}
}

func blackboxBestCorr(ref, test []int16, maxLag int) (int, float64) {
	bestLag := 0
	bestCorr := math.Inf(-1)
	for lag := -maxLag; lag <= maxLag; lag++ {
		c := blackboxCorrLag(ref, test, lag)
		if c > bestCorr {
			bestCorr = c
			bestLag = lag
		}
	}
	return bestLag, bestCorr
}

func blackboxCorr(ref, test []int16) float64 {
	return blackboxCorrLag(ref, test, 0)
}

func blackboxCorrLag(ref, test []int16, lag int) float64 {
	var sx, sy float64
	var n int
	for i := range ref {
		j := i + lag
		if j < 0 || j >= len(test) {
			continue
		}
		sx += float64(ref[i])
		sy += float64(test[j])
		n++
	}
	if n == 0 {
		return math.NaN()
	}
	mx := sx / float64(n)
	my := sy / float64(n)

	var dot, ex, ey float64
	for i := range ref {
		j := i + lag
		if j < 0 || j >= len(test) {
			continue
		}
		x := float64(ref[i]) - mx
		y := float64(test[j]) - my
		dot += x * y
		ex += x * x
		ey += y * y
	}
	if ex == 0 || ey == 0 {
		return 0
	}
	return dot / math.Sqrt(ex*ey)
}

func blackboxDC(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s)
	}
	return sum / float64(len(samples))
}

type blackboxWorstFrame struct {
	frame  int
	snr    float64
	corr   float64
	refRMS float64
	gotRMS float64
	ratio  float64
	errRMS float64
}

func blackboxWorstFrames(ref, test []int16, limit int) []blackboxWorstFrame {
	var frames []blackboxWorstFrame
	for base, frame := 0, 0; base+frameSamples <= len(ref) && base+frameSamples <= len(test); base, frame = base+frameSamples, frame+1 {
		r := ref[base : base+frameSamples]
		g := test[base : base+frameSamples]
		refRMS := diag4Rms(r)
		gotRMS := diag4Rms(g)
		if refRMS < 10 {
			continue
		}
		snr := scaleProbeGlobalSNR(r, g)
		corr := blackboxCorr(r, g)
		ratio := 0.0
		if refRMS > 0 {
			ratio = gotRMS / refRMS
		}
		frames = append(frames, blackboxWorstFrame{
			frame:  frame,
			snr:    snr,
			corr:   corr,
			refRMS: refRMS,
			gotRMS: gotRMS,
			ratio:  ratio,
			errRMS: blackboxErrRMS(r, g),
		})
	}
	sort.Slice(frames, func(i, j int) bool {
		if frames[i].snr != frames[j].snr {
			return frames[i].snr < frames[j].snr
		}
		return frames[i].frame < frames[j].frame
	})
	if len(frames) > limit {
		frames = frames[:limit]
	}
	return frames
}

func blackboxErrRMS(ref, test []int16) float64 {
	var e float64
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	if n == 0 {
		return 0
	}
	for i := 0; i < n; i++ {
		d := float64(ref[i]) - float64(test[i])
		e += d * d
	}
	return math.Sqrt(e / float64(n))
}

func blackboxVerdict(prod, scaled, noPF, postNoHP, synthOnly blackboxMetrics) string {
	if scaled.globalSNR-prod.globalSNR > 3 && scaled.corr >= prod.corr-0.02 {
		return "amplitude/Q-format remains plausible: scale-only gives material SNR recovery without correlation loss"
	}
	if absInt(prod.bestSNRLag) > 2 && prod.bestSNR-prod.globalSNR > 1.5 {
		return "sample/frame alignment deserves first investigation: lag sweep materially improves SNR"
	}
	if noPF.globalSNR-prod.globalSNR > 1 || noPF.corr-prod.corr > 0.05 {
		return "postfilter path is suspect: structural postfilter bypass improves black-box metrics"
	}
	if postNoHP.globalSNR-prod.globalSNR > 1 || postNoHP.corr-prod.corr > 0.05 {
		return "HP output filter is suspect: HP bypass proxy improves black-box metrics"
	}
	if synthOnly.globalSNR >= prod.globalSNR-0.5 && synthOnly.corr >= prod.corr-0.02 {
		return "postfilter/HP do not rescue the signal; upstream synthesis/excitation shape remains suspect"
	}
	return "no single bypass/scale/alignment variant fixes quality; continue upstream pitch/gain/FCB/adaptive localization"
}

type blackboxAsteriskStats struct {
	rms              float64
	peak             int
	dc               float64
	clipped          int
	zeroSamples      int
	silenceFrames    int
	lowEnergyFrames  int
	p10RMS           float64
	p50RMS           float64
	p90RMS           float64
	meanBoundaryJump float64
	maxBoundaryJump  int
}

func blackboxSignalStats(samples []int16, frames int) blackboxAsteriskStats {
	st := blackboxAsteriskStats{
		rms:  diag4Rms(samples),
		peak: diag4MaxAbs(samples),
		dc:   blackboxDC(samples),
	}
	frameRMS := make([]float64, 0, frames)
	for f := 0; f < frames; f++ {
		start := f * frameSamples
		end := start + frameSamples
		if end > len(samples) {
			break
		}
		r := diag4Rms(samples[start:end])
		frameRMS = append(frameRMS, r)
		if r < 20 {
			st.silenceFrames++
		}
		if r < 100 {
			st.lowEnergyFrames++
		}
	}
	for _, s := range samples {
		if s == 0 {
			st.zeroSamples++
		}
		if s == 32767 || s == -32768 {
			st.clipped++
		}
	}
	var jumpSum float64
	var jumpN int
	for f := 1; f < frames; f++ {
		prev := int(samples[f*frameSamples-1])
		next := int(samples[f*frameSamples])
		jump := prev - next
		if jump < 0 {
			jump = -jump
		}
		jumpSum += float64(jump)
		jumpN++
		if jump > st.maxBoundaryJump {
			st.maxBoundaryJump = jump
		}
	}
	if jumpN > 0 {
		st.meanBoundaryJump = jumpSum / float64(jumpN)
	}
	sort.Float64s(frameRMS)
	st.p10RMS = blackboxQuantile(frameRMS, 0.10)
	st.p50RMS = blackboxQuantile(frameRMS, 0.50)
	st.p90RMS = blackboxQuantile(frameRMS, 0.90)
	return st
}

func blackboxQuantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
