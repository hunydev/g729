package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// TestPhase3yRecursiveACBFFmpegAudit checks whether fractional ACB FIR taps
// that cross into already-computed current-subframe samples explain the local
// decoder quality gap. FFmpeg is used only as an executable black-box decoder.
func TestPhase3yRecursiveACBFFmpegAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_RECURSIVE_ACB_FFMPEG_AUDIT") != "1" {
		t.Skip("set G729_DECODER_RECURSIVE_ACB_FFMPEG_AUDIT=1 to run recursive ACB audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	bitPath := vectorPath("SPEECH.BIT")
	ensureTestdataPresent(t, bitPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	speechFrames := len(bitData) / bitstream.G192FrameBytes
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	phase3yReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk recursive ACB audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3yReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3yACBMode int

const (
	phase3yACBProduction phase3yACBMode = iota
	phase3yACBIntegerOnly
	phase3yACBFractionalExisting
	phase3yACBRecursiveCurrent
	phase3yACBRecursiveSignFlip
	phase3yACBRecursivePhaseSwap
)

type phase3yACBVariant struct {
	name string
	mode phase3yACBMode
}

func phase3yVariants() []phase3yACBVariant {
	return []phase3yACBVariant{
		{name: "production_fractional_zero_current", mode: phase3yACBProduction},
		{name: "integer_lag_only", mode: phase3yACBIntegerOnly},
		{name: "explicit_fractional_zero_current", mode: phase3yACBFractionalExisting},
		{name: "recursive_current_taps", mode: phase3yACBRecursiveCurrent},
		{name: "recursive_frac_sign_flip", mode: phase3yACBRecursiveSignFlip},
		{name: "recursive_phase_swap", mode: phase3yACBRecursivePhaseSwap},
	}
}

func phase3yReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	variants := phase3yVariants()
	rows := make([]phase3xRow, 0, len(variants))
	baseOut := decodeVariant(t, bitData, frames, nil, nil)
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)

	t.Logf("Phase 3y recursive ACB FFmpeg audit - %s", label)
	t.Logf("%-34s %9s %7s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	t.Logf("%-34s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------")
	for _, v := range variants {
		out := baseOut
		if v.mode != phase3yACBProduction {
			out = phase3yDecodeG192Variant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-34s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
	}
	phase3yLogBest(t, base, baseEnv, rows)
}

func phase3yReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	variants := phase3yVariants()
	rows := make([]phase3xRow, 0, len(variants))
	baseOut := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)

	t.Logf("Phase 3y recursive ACB FFmpeg audit - %s", label)
	t.Logf("%-34s %9s %7s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	t.Logf("%-34s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------")
	for _, v := range variants {
		out := baseOut
		if v.mode != phase3yACBProduction {
			out = phase3yDecodeRawVariant(t, raw, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-34s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
	}
	phase3yLogBest(t, base, baseEnv, rows)
}

func phase3yLogBest(t *testing.T, base blackboxMetrics, baseEnv phase3pEnvelopeSummary, rows []phase3xRow) {
	t.Helper()
	best := rows[0]
	bestSeg := rows[0]
	bestCorr := rows[0]
	bestEnv := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.segSNR > bestSeg.m.segSNR {
			bestSeg = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
		if r.env.lowRatioFrames < bestEnv.env.lowRatioFrames {
			bestEnv = r
		}
	}
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-base.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-base.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-base.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnv.name, bestEnv.env.lowRatioFrames, bestEnv.env.lowRatioFrames-baseEnv.lowRatioFrames)
	t.Logf("verdict: %s", phase3yVerdict(phase3xRow{name: rows[0].name, m: base, env: baseEnv}, best, bestSeg, bestCorr, bestEnv))
}

func phase3yDecodeG192Variant(t *testing.T, bitData []byte, frames int, variant phase3yACBVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3yVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3yVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func phase3yDecodeRawVariant(t *testing.T, raw []byte, frames int, variant phase3yACBVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3yVariant(packed, out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3yVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3yVariant(packed []byte, out []int16, variant phase3yACBVariant) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(fr.L0),
		L1: uint8(fr.L1),
		L2: uint8(fr.L2),
		L3: uint8(fr.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	d.decodeSubframePhase3yVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3yVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3yVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	variant phase3yACBVariant,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	phase3yAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v, variant.mode)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.rememberPitchGain(gpQ14)
}

func phase3yAdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16, mode phase3yACBMode) {
	switch mode {
	case phase3yACBProduction:
		decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
	case phase3yACBIntegerOnly:
		decodeAdaptiveCodebookIntegerLagOnly(tInt, pastExc, v)
	case phase3yACBFractionalExisting:
		pitch.AdaptiveCodebook(tInt, tFrac, pastExc, v)
	case phase3yACBRecursiveSignFlip:
		phase3yAdaptiveCodebookRecursive(tInt, -tFrac, pastExc, v, false)
	case phase3yACBRecursivePhaseSwap:
		phase3yAdaptiveCodebookRecursive(tInt, tFrac, pastExc, v, true)
	default:
		phase3yAdaptiveCodebookRecursive(tInt, tFrac, pastExc, v, false)
	}
}

func phase3yAdaptiveCodebookRecursive(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16, phaseSwap bool) {
	if tFrac == 0 {
		pitch.AdaptiveCodebook(tInt, 0, pastExc, v)
		return
	}

	k, posPhase, negPhase := phase3yFIRParams(tInt, tFrac)
	if phaseSwap {
		posPhase, negPhase = negPhase, posPhase
	}
	fir := tables.PitchInterpFIR
	for n := 0; n < subframeLen; n++ {
		var acc fixed.Word32
		for i := 0; i < pitch.Linter; i++ {
			back := phase3yACBSource(n-k-i, pastExc, v)
			fwd := phase3yACBSource(n-k+1+i, pastExc, v)
			acc = fixed.LMac(acc, fir[posPhase+3*i], back)
			acc = fixed.LMac(acc, fir[negPhase+3*i], fwd)
		}
		v[n] = fixed.Round(acc)
	}
}

func phase3yFIRParams(tInt, tFrac int) (k, posPhase, negPhase int) {
	if tFrac < 0 {
		return tInt, 1, 2
	}
	return tInt + 1, 2, 1
}

func phase3yACBSource(relative int, pastExc []int16, v *[subframeLen]int16) int16 {
	if relative < 0 {
		idx := len(pastExc) + relative
		if idx >= 0 && idx < len(pastExc) {
			return pastExc[idx]
		}
		return 0
	}
	if relative < subframeLen {
		return v[relative]
	}
	return 0
}

func phase3yVerdict(base, best, bestSeg, bestCorr, bestEnv phase3xRow) string {
	if best.name == "integer_lag_only" || bestSeg.name == "integer_lag_only" || bestCorr.name == "integer_lag_only" || bestEnv.name == "integer_lag_only" {
		return "integer_lag_only is the best quality fallback but ignores transmitted fractional delay; no recursive fractional ACB variant is production-safe"
	}
	if best.name != base.name && best.m.globalSNR-base.m.globalSNR > 1.0 && best.m.segSNR >= base.m.segSNR-0.25 && best.m.corr >= base.m.corr-0.02 {
		return "recursive ACB materially improves global SNR without damaging segmental/correlation; inspect " + best.name
	}
	if bestSeg.name != base.name && bestSeg.m.segSNR-base.m.segSNR > 1.0 && bestSeg.m.globalSNR >= base.m.globalSNR-0.25 && bestSeg.m.corr >= base.m.corr-0.02 {
		return "recursive ACB materially improves segmental SNR without damaging global/correlation; inspect " + bestSeg.name
	}
	if bestCorr.name != base.name && bestCorr.m.corr-base.m.corr > 0.05 && bestCorr.m.globalSNR >= base.m.globalSNR-0.25 {
		return "recursive ACB materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnv.name != base.name && base.env.lowRatioFrames-bestEnv.env.lowRatioFrames > base.env.activeFrames/10 {
		return "recursive ACB reduces low-ratio frames but is not waveform-safe"
	}
	return "no recursive ACB variant is production-safe"
}
