package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3gSynthFloatReferenceAudit_SPEECH compares the production
// fixed-point synthesis filter against a clean-room float64 direct-form
// calculation fed with the same decoded excitation and LP coefficients.
//
// It is an opt-in diagnostic. The float path is not a production candidate; it
// only separates active synthesis arithmetic from upstream excitation/gain/FCB
// reconstruction defects.
func TestPhase3gSynthFloatReferenceAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_SYNTH_FLOAT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_SYNTH_FLOAT_AUDIT=1 to audit synth float reference")
	}

	bitPath := vectorPath("SPEECH.BIT")
	pstPath := vectorPath("SPEECH.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)

	audit := phase3gDecodeSynthFloat(t, bitData, frames)
	productionViaDecode := decodeVariant(t, bitData, frames, nil, nil)
	if !phase3eEqualPCM(audit.production, productionViaDecode) {
		t.Fatalf("phase3g taps production diverges from Decoder.Decode baseline")
	}

	prod := blackboxMeasure(ref, audit.production, 40)
	fixedSynth := blackboxMeasure(ref, audit.fixedSynthX2, 40)
	rounded := blackboxMeasure(ref, audit.floatRoundedX2, 40)
	full := blackboxMeasure(ref, audit.floatFullX2, 40)
	roundedDiff := phase3gDiffStats(audit.fixedSynthRaw, audit.floatRoundedRaw)
	fullDiff := phase3gDiffStats(audit.fixedSynthRaw, audit.floatFullRaw)

	t.Logf("Phase 3g synth float reference audit - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("production baseline: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f",
		prod.rms, prod.peak, prod.globalSNR, prod.segSNR, prod.corr)
	t.Logf("")
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "diffSamp", "maxAbs", "diffRMS")
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "--------", "------", "-------")
	phase3gLogRow(t, "fixed_synth_x2", fixedSynth, fixedSynth, phase3gSignalDiff{})
	phase3gLogRow(t, "float_roundstate_x2", rounded, fixedSynth, roundedDiff)
	phase3gLogRow(t, "float_fullstate_x2", full, fixedSynth, fullDiff)
	t.Logf("")
	t.Logf("verdict: %s", phase3gSynthFloatVerdict(fixedSynth, rounded, full, roundedDiff, fullDiff))

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk synth float audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	astAudit := phase3gDecodeRawSynthFloat(t, raw, astFrames)
	phase3gLogAudit(t, "Asterisk/FFmpeg", astFrames, astRef, astAudit)
}

type phase3gSynthFloatAudit struct {
	production      []int16
	fixedSynthRaw   []int16
	fixedSynthX2    []int16
	floatRoundedRaw []int16
	floatRoundedX2  []int16
	floatFullRaw    []int16
	floatFullX2     []int16
}

func phase3gDecodeSynthFloat(t *testing.T, bitData []byte, frames int) phase3gSynthFloatAudit {
	t.Helper()
	total := frames * frameSamples
	out := phase3gSynthFloatAudit{
		production:      make([]int16, total),
		fixedSynthRaw:   make([]int16, total),
		fixedSynthX2:    make([]int16, total),
		floatRoundedRaw: make([]int16, total),
		floatRoundedX2:  make([]int16, total),
		floatFullRaw:    make([]int16, total),
		floatFullX2:     make([]int16, total),
	}

	var dec Decoder
	var roundedSynth phase3gFloatSynth
	var fullSynth phase3gFloatSynth
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[phase3g] frame %d: %v", f, err)
		}
		taps, err := dec.DecodeWithTaps(packed[:])
		if err != nil {
			t.Fatalf("DecodeWithTaps[phase3g] frame %d: %v", f, err)
		}
		base := f * frameSamples
		copy(out.production[base:base+frameSamples], taps.Output[:])
		for sf := 0; sf < 2; sf++ {
			sub := &taps.Sub[sf]
			off := base + sf*subframeLen
			copy(out.fixedSynthRaw[off:off+subframeLen], sub.S[:])
			blackboxScale2Into(out.fixedSynthX2[off:off+subframeLen], sub.S[:])

			var rounded [subframeLen]int16
			var full [subframeLen]int16
			roundedSynth.filter(&sub.A, &sub.U, &rounded, true)
			fullSynth.filter(&sub.A, &sub.U, &full, false)
			copy(out.floatRoundedRaw[off:off+subframeLen], rounded[:])
			copy(out.floatFullRaw[off:off+subframeLen], full[:])
			blackboxScale2Into(out.floatRoundedX2[off:off+subframeLen], rounded[:])
			blackboxScale2Into(out.floatFullX2[off:off+subframeLen], full[:])
		}
	}
	return out
}

func phase3gDecodeRawSynthFloat(t *testing.T, raw []byte, frames int) phase3gSynthFloatAudit {
	t.Helper()
	total := frames * frameSamples
	out := phase3gSynthFloatAudit{
		production:      make([]int16, total),
		fixedSynthRaw:   make([]int16, total),
		fixedSynthX2:    make([]int16, total),
		floatRoundedRaw: make([]int16, total),
		floatRoundedX2:  make([]int16, total),
		floatFullRaw:    make([]int16, total),
		floatFullX2:     make([]int16, total),
	}

	var dec Decoder
	var roundedSynth phase3gFloatSynth
	var fullSynth phase3gFloatSynth
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("DecodeWithTaps[phase3g raw] frame %d: %v", f, err)
		}
		base := f * frameSamples
		copy(out.production[base:base+frameSamples], taps.Output[:])
		for sf := 0; sf < 2; sf++ {
			sub := &taps.Sub[sf]
			off := base + sf*subframeLen
			copy(out.fixedSynthRaw[off:off+subframeLen], sub.S[:])
			blackboxScale2Into(out.fixedSynthX2[off:off+subframeLen], sub.S[:])

			var rounded [subframeLen]int16
			var full [subframeLen]int16
			roundedSynth.filter(&sub.A, &sub.U, &rounded, true)
			fullSynth.filter(&sub.A, &sub.U, &full, false)
			copy(out.floatRoundedRaw[off:off+subframeLen], rounded[:])
			copy(out.floatFullRaw[off:off+subframeLen], full[:])
			blackboxScale2Into(out.floatRoundedX2[off:off+subframeLen], rounded[:])
			blackboxScale2Into(out.floatFullX2[off:off+subframeLen], full[:])
		}
	}
	return out
}

func phase3gLogAudit(t *testing.T, label string, frames int, ref []int16, audit phase3gSynthFloatAudit) {
	t.Helper()
	prod := blackboxMeasure(ref, audit.production, 40)
	fixedSynth := blackboxMeasure(ref, audit.fixedSynthX2, 40)
	rounded := blackboxMeasure(ref, audit.floatRoundedX2, 40)
	full := blackboxMeasure(ref, audit.floatFullX2, 40)
	roundedDiff := phase3gDiffStats(audit.fixedSynthRaw, audit.floatRoundedRaw)
	fullDiff := phase3gDiffStats(audit.fixedSynthRaw, audit.floatFullRaw)

	t.Logf("Phase 3g synth float reference audit - %s (%d frames)", label, frames)
	t.Logf("production baseline: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f",
		prod.rms, prod.peak, prod.globalSNR, prod.segSNR, prod.corr)
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "diffSamp", "maxAbs", "diffRMS")
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "--------", "------", "-------")
	phase3gLogRow(t, "fixed_synth_x2", fixedSynth, fixedSynth, phase3gSignalDiff{})
	phase3gLogRow(t, "float_roundstate_x2", rounded, fixedSynth, roundedDiff)
	phase3gLogRow(t, "float_fullstate_x2", full, fixedSynth, fullDiff)
	t.Logf("verdict: %s", phase3gSynthFloatVerdict(fixedSynth, rounded, full, roundedDiff, fullDiff))
}

type phase3gFloatSynth struct {
	past [lpcOrder]float64
}

func (s *phase3gFloatSynth) filter(a *[lpcOrder + 1]int16, u *[subframeLen]int16, out *[subframeLen]int16, roundedState bool) {
	var cur [subframeLen]float64
	for n := 0; n < subframeLen; n++ {
		y := float64(a[0]) * float64(u[n])
		for i := 1; i <= lpcOrder; i++ {
			var prev float64
			if n-i >= 0 {
				prev = cur[n-i]
			} else {
				prev = s.past[lpcOrder+n-i]
			}
			y -= float64(a[i]) * prev
		}
		y /= 4096.0
		q := phase3gRoundWord16(y)
		out[n] = q
		if roundedState {
			cur[n] = float64(q)
		} else {
			cur[n] = y
		}
	}
	for i := 0; i < lpcOrder; i++ {
		s.past[i] = cur[subframeLen-lpcOrder+i]
	}
}

func phase3gRoundWord16(x float64) int16 {
	q := math.Floor(x + 0.5)
	if q > 32767 {
		return 32767
	}
	if q < -32768 {
		return -32768
	}
	return int16(q)
}

type phase3gSignalDiff struct {
	diffSamples int
	maxAbs      int
	rms         float64
}

func phase3gDiffStats(a, b []int16) phase3gSignalDiff {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var st phase3gSignalDiff
	var e float64
	for i := 0; i < n; i++ {
		d := int(a[i]) - int(b[i])
		if d != 0 {
			st.diffSamples++
		}
		if d < 0 {
			d = -d
		}
		if d > st.maxAbs {
			st.maxAbs = d
		}
		e += float64(d * d)
	}
	if n > 0 {
		st.rms = math.Sqrt(e / float64(n))
	}
	return st
}

func phase3gLogRow(t *testing.T, name string, m, base blackboxMetrics, diff phase3gSignalDiff) {
	t.Helper()
	t.Logf("%-24s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %9d %9d %9.3f",
		name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
		m.globalSNR-base.globalSNR, m.corr-base.corr,
		diff.diffSamples, diff.maxAbs, diff.rms)
}

func phase3gSynthFloatVerdict(fixedSynth, rounded, full blackboxMetrics, roundedDiff, fullDiff phase3gSignalDiff) string {
	if roundedDiff.diffSamples == 0 && full.globalSNR-fixedSynth.globalSNR < 0.2 && full.corr-fixedSynth.corr < 0.02 {
		return "fixed-point synth matches the independent rounded-state float model; full-precision synth state does not improve black-box metrics"
	}
	if roundedDiff.diffSamples > 0 && rounded.globalSNR-fixedSynth.globalSNR > 1.0 {
		return "rounded-state float synthesis diverges and improves metrics; active fixed-point synth arithmetic needs inspection"
	}
	if full.globalSNR-fixedSynth.globalSNR > 1.0 || full.corr-fixedSynth.corr > 0.05 {
		return "full-precision synth state materially improves output; synthesis quantization/state is a plausible defect"
	}
	if fullDiff.diffSamples > 0 {
		return "full-precision synth state changes samples without material metric recovery; upstream excitation/gain/FCB remains more likely"
	}
	return "synthesis float audit is inconclusive; continue upstream numeric reconstruction checks"
}
