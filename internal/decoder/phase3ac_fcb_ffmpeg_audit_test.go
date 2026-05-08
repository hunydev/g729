package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3acFCBFFmpegAudit isolates fixed-codebook pulse reconstruction
// variants against FFmpeg executable black-box decode. It keeps pitch, gain,
// synthesis, postfilter, and high-pass on the production path.
func TestPhase3acFCBFFmpegAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_FCB_FFMPEG_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FCB_FFMPEG_AUDIT=1 to run FCB FFmpeg audit")
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
	phase3acReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk FCB FFmpeg audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3acReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3acFCBMode int

const (
	phase3acFCBProduction phase3acFCBMode = iota
	phase3acFCBSignInvert
	phase3acFCBSignMSBToPulse0
	phase3acFCBSignReverseNibble
	phase3acFCBPositionMSBGroups
	phase3acFCBPositionJXFlip
	phase3acFCBPositionJXZero
	phase3acFCBPositionJXOne
	phase3acFCBNoPitchEnhance
	phase3acFCBGainRawExcEnhanced
	phase3acFCBGainEnhancedExcRaw
	phase3acFCBGainRawExcEnhancedCurrentBeta
)

type phase3acFCBVariant struct {
	name string
	mode phase3acFCBMode
}

func phase3acVariants() []phase3acFCBVariant {
	return []phase3acFCBVariant{
		{name: "production", mode: phase3acFCBProduction},
		{name: "sign_invert", mode: phase3acFCBSignInvert},
		{name: "sign_msb_to_pulse0", mode: phase3acFCBSignMSBToPulse0},
		{name: "sign_reverse_nibble", mode: phase3acFCBSignReverseNibble},
		{name: "pos_msb_groups", mode: phase3acFCBPositionMSBGroups},
		{name: "pos_jx_flip", mode: phase3acFCBPositionJXFlip},
		{name: "pos_jx_zero", mode: phase3acFCBPositionJXZero},
		{name: "pos_jx_one", mode: phase3acFCBPositionJXOne},
		{name: "no_pitch_enhance", mode: phase3acFCBNoPitchEnhance},
		{name: "gain_raw_exc_enh", mode: phase3acFCBGainRawExcEnhanced},
		{name: "gain_enh_exc_raw", mode: phase3acFCBGainEnhancedExcRaw},
		{name: "gain_raw_exc_enh_curbeta", mode: phase3acFCBGainRawExcEnhancedCurrentBeta},
	}
}

func phase3acReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	baseOut := decodeVariant(t, bitData, frames, nil, nil)
	phase3acReport(t, label, ref, baseOut, func(v phase3acFCBVariant) []int16 {
		return phase3acDecodeG192Variant(t, bitData, frames, v)
	})
}

func phase3acReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	baseOut := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	phase3acReport(t, label, ref, baseOut, func(v phase3acFCBVariant) []int16 {
		return phase3acDecodeRawVariant(t, raw, frames, v)
	})
}

func phase3acReport(t *testing.T, label string, ref, baseOut []int16, decode func(phase3acFCBVariant) []int16) {
	t.Helper()
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)
	rows := make([]phase3xRow, 0, len(phase3acVariants()))

	t.Logf("Phase 3ac FCB FFmpeg audit - %s", label)
	t.Logf("%-20s %9s %7s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	t.Logf("%-20s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------")
	for _, v := range phase3acVariants() {
		out := baseOut
		if v.mode != phase3acFCBProduction {
			out = decode(v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-20s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
	}
	phase3acLogBest(t, phase3xRow{name: "production", m: base, env: baseEnv}, rows)
}

func phase3acLogBest(t *testing.T, base phase3xRow, rows []phase3xRow) {
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
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-base.m.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-base.m.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-base.m.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnv.name, bestEnv.env.lowRatioFrames, bestEnv.env.lowRatioFrames-base.env.lowRatioFrames)
	t.Logf("verdict: %s", phase3acVerdict(base, best, bestSeg, bestCorr, bestEnv))
}

func phase3acDecodeG192Variant(t *testing.T, bitData []byte, frames int, variant phase3acFCBVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3acVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3acVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func phase3acDecodeRawVariant(t *testing.T, raw []byte, frames int, variant phase3acFCBVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3acVariant(packed, out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3acVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3acVariant(packed []byte, out []int16, variant phase3acFCBVariant) error {
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

	d.decodeSubframePhase3acVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3acVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3acVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	variant phase3acFCBVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var cGain [subframeLen]int16
	var cExc [subframeLen]int16
	var gpQ14, gcMant int16
	var gcExp int8
	gainDecoded := false

	switch variant.mode {
	case phase3acFCBGainRawExcEnhanced:
		phase3acDecodeFCB(C, S, tInt, betaQ14, &cGain, phase3acFCBNoPitchEnhance)
		cExc = cGain
		fcb.ApplyPitchEnhancement(&cExc, tInt, betaQ14)
	case phase3acFCBGainEnhancedExcRaw:
		phase3acDecodeFCB(C, S, tInt, betaQ14, &cGain, phase3acFCBProduction)
		phase3acDecodeFCB(C, S, tInt, betaQ14, &cExc, phase3acFCBNoPitchEnhance)
	case phase3acFCBGainRawExcEnhancedCurrentBeta:
		phase3acDecodeFCB(C, S, tInt, betaQ14, &cGain, phase3acFCBNoPitchEnhance)
		gpQ14, gcMant, gcExp = d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &cGain)
		gainDecoded = true
		cExc = cGain
		fcb.ApplyPitchEnhancement(&cExc, tInt, fcb.ClampPitchGainForEnhancement(gpQ14))
	default:
		phase3acDecodeFCB(C, S, tInt, betaQ14, &cGain, variant.mode)
		cExc = cGain
	}
	if !gainDecoded {
		gpQ14, gcMant, gcExp = d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &cGain)
	}

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &cExc, &u)

	var synthOut [subframeLen]int16
	d.syn.Filter(sfA, &u, &synthOut)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &synthOut, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func phase3acDecodeFCB(C uint16, S uint8, tInt int, betaQ14 int16, c *[subframeLen]int16, mode phase3acFCBMode) {
	if mode == phase3acFCBProduction {
		fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, c)
		return
	}

	positions := phase3acPositions(C, mode)
	phase3acPlacePulses(positions, S, c, mode)
	if mode == phase3acFCBNoPitchEnhance {
		return
	}
	fcb.ApplyPitchEnhancement(c, tInt, betaQ14)
}

func phase3acPositions(code uint16, mode phase3acFCBMode) [4]int {
	if mode == phase3acFCBPositionMSBGroups {
		i0 := int((code >> 10) & 0x07)
		i1 := int((code >> 7) & 0x07)
		i2 := int((code >> 4) & 0x07)
		jx := int((code >> 3) & 0x01)
		i3 := int(code & 0x07)
		return [4]int{5 * i0, 5*i1 + 1, 5*i2 + 2, 5*i3 + 3 + jx}
	}

	i0 := int(code & 0x07)
	i1 := int((code >> 3) & 0x07)
	i2 := int((code >> 6) & 0x07)
	jx := int((code >> 9) & 0x01)
	i3 := int((code >> 10) & 0x07)
	switch mode {
	case phase3acFCBPositionJXFlip:
		jx ^= 1
	case phase3acFCBPositionJXZero:
		jx = 0
	case phase3acFCBPositionJXOne:
		jx = 1
	}
	return [4]int{5 * i0, 5*i1 + 1, 5*i2 + 2, 5*i3 + 3 + jx}
}

func phase3acPlacePulses(positions [4]int, signs uint8, c *[subframeLen]int16, mode phase3acFCBMode) {
	for i := range c {
		c[i] = 0
	}
	switch mode {
	case phase3acFCBSignInvert:
		signs ^= 0x0f
	case phase3acFCBSignReverseNibble:
		signs = reverseNibble(signs)
	}
	for i := 0; i < 4; i++ {
		bit := uint(i)
		if mode == phase3acFCBSignMSBToPulse0 {
			bit = uint(3 - i)
		}
		if (signs>>bit)&1 == 1 {
			c[positions[i]] = fcb.PulseAmplitude
		} else {
			c[positions[i]] = -fcb.PulseAmplitude
		}
	}
}

func phase3acVerdict(base, best, bestSeg, bestCorr, bestEnv phase3xRow) string {
	if best.name != base.name && best.m.globalSNR-base.m.globalSNR > 1.0 && best.m.segSNR >= base.m.segSNR-0.25 && best.m.corr >= base.m.corr-0.02 {
		return "FCB variant materially improves global SNR without damaging segmental/correlation; inspect " + best.name
	}
	if bestSeg.name != base.name && bestSeg.m.segSNR-base.m.segSNR > 1.0 && bestSeg.m.globalSNR >= base.m.globalSNR-0.25 && bestSeg.m.corr >= base.m.corr-0.02 {
		return "FCB variant materially improves segmental SNR without damaging global/correlation; inspect " + bestSeg.name
	}
	if bestCorr.name != base.name && bestCorr.m.corr-base.m.corr > 0.05 && bestCorr.m.globalSNR >= base.m.globalSNR-0.25 {
		return "FCB variant materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnv.name != base.name && base.env.lowRatioFrames-bestEnv.env.lowRatioFrames > base.env.activeFrames/10 {
		return "FCB variant reduces low-ratio frames but is not waveform-safe"
	}
	return "no FCB variant is production-safe"
}
