package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3iFCBVariantAudit_SPEECH probes fixed-codebook pulse
// reconstruction variants with final-output black-box metrics. Variants alter
// only c[] construction; pitch, gain, synthesis, postfilter, and HP stay on the
// production path.
func TestPhase3iFCBVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FCB_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FCB_AUDIT=1 to audit FCB variants")
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

	production := phase3iDecodeVariant(t, bitData, frames, phase3iVariant{name: "production"})
	productionViaDecode := decodeVariant(t, bitData, frames, nil, nil)
	if !phase3eEqualPCM(production, productionViaDecode) {
		t.Fatalf("phase3i production variant diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, production, 40)

	variants := []phase3iVariant{
		{name: "production"},
		{name: "fcb_signs_inverted", mode: phase3iFCBSignsInverted},
		{name: "fcb_sign_bits_reversed", mode: phase3iFCBSignBitsReversed},
		{name: "fcb_sign_lsb_order", mode: phase3iFCBSignLSBOrder},
		{name: "fcb_pos_jx_flipped", mode: phase3iFCBPosJXFlipped},
		{name: "fcb_pos_no_jx", mode: phase3iFCBPosNoJX},
		{name: "fcb_pos_force_jx", mode: phase3iFCBPosForceJX},
		{name: "fcb_pos_tracks_reversed", mode: phase3iFCBPosTracksReversed},
		{name: "fcb_pos_bits_reversed13", mode: phase3iFCBPosBitsReversed13},
		{name: "fcb_no_pitch_enhance", mode: phase3iFCBNoPitchEnhance},
		{name: "fcb_beta_lower", mode: phase3iFCBBetaLower},
		{name: "fcb_beta_upper", mode: phase3iFCBBetaUpper},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3i FCB variant audit - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("baseline production: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR)
	t.Logf("")
	t.Logf("%-28s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "lagSNR", "bestSNR")
	t.Logf("%-28s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3iFCBProduction {
			out = phase3iDecodeVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		rows = append(rows, row{name: v.name, m: m})
		t.Logf("%-28s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %10d %10.2f",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			m.globalSNR-prodMetrics.globalSNR, m.corr-prodMetrics.corr,
			m.bestSNRLag, m.bestSNR)
	}

	best := rows[0]
	bestCorr := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
	}
	t.Logf("")
	t.Logf("best by gSNR@0: %s %.2f dB (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("verdict: %s", phase3iVerdict(prodMetrics, best, bestCorr))
}

type phase3iFCBMode int

const (
	phase3iFCBProduction phase3iFCBMode = iota
	phase3iFCBSignsInverted
	phase3iFCBSignBitsReversed
	phase3iFCBSignLSBOrder
	phase3iFCBPosJXFlipped
	phase3iFCBPosNoJX
	phase3iFCBPosForceJX
	phase3iFCBPosTracksReversed
	phase3iFCBPosBitsReversed13
	phase3iFCBNoPitchEnhance
	phase3iFCBBetaLower
	phase3iFCBBetaUpper
)

type phase3iVariant struct {
	name string
	mode phase3iFCBMode
}

func phase3iDecodeVariant(t *testing.T, bitData []byte, frames int, variant phase3iVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3iVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3iVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3iVariant(packed []byte, out []int16, variant phase3iVariant) error {
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

	d.decodeSubframePhase3iVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3iVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3iVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	variant phase3iVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	phase3iDecodeFCB(C, S, tInt, betaQ14, &c, variant.mode)

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, &c)
	gpQ14 := gainTaps.GpQ14Final

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gainTaps.GcMantQ14, gainTaps.GcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func phase3iDecodeFCB(C uint16, S uint8, tInt int, betaQ14 int16, c *[subframeLen]int16, mode phase3iFCBMode) {
	if mode == phase3iFCBProduction {
		fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, c)
		return
	}
	positions := phase3iPositions(C, mode)
	phase3iPlacePulses(positions, S, c, mode)

	switch mode {
	case phase3iFCBNoPitchEnhance:
		return
	case phase3iFCBBetaLower:
		fcb.ApplyPitchEnhancement(c, tInt, 3277)
	case phase3iFCBBetaUpper:
		fcb.ApplyPitchEnhancement(c, tInt, 13107)
	default:
		fcb.ApplyPitchEnhancement(c, tInt, betaQ14)
	}
}

func phase3iPositions(code uint16, mode phase3iFCBMode) [4]int {
	if mode == phase3iFCBPosBitsReversed13 {
		code = phase3iReverse13(code)
	}
	i0 := int((code >> 10) & 0x07)
	i1 := int((code >> 7) & 0x07)
	i2 := int((code >> 4) & 0x07)
	jx := int((code >> 3) & 0x01)
	i3 := int(code & 0x07)
	switch mode {
	case phase3iFCBPosJXFlipped:
		jx ^= 1
	case phase3iFCBPosNoJX:
		jx = 0
	case phase3iFCBPosForceJX:
		jx = 1
	case phase3iFCBPosTracksReversed:
		i0, i1, i2, i3 = i3, i2, i1, i0
	}
	return [4]int{
		5 * i0,
		5*i1 + 1,
		5*i2 + 2,
		5*i3 + 3 + jx,
	}
}

func phase3iPlacePulses(positions [4]int, signs uint8, c *[subframeLen]int16, mode phase3iFCBMode) {
	for i := range c {
		c[i] = 0
	}
	if mode == phase3iFCBSignsInverted {
		signs ^= 0x0F
	}
	if mode == phase3iFCBSignBitsReversed {
		signs = reverseNibble(signs)
	}
	for i := 0; i < 4; i++ {
		bit := uint(3 - i)
		if mode == phase3iFCBSignLSBOrder {
			bit = uint(i)
		}
		if (signs>>bit)&1 == 1 {
			c[positions[i]] = fcb.PulseAmplitude
		} else {
			c[positions[i]] = -fcb.PulseAmplitude
		}
	}
}

func phase3iReverse13(v uint16) uint16 {
	var out uint16
	for i := 0; i < 13; i++ {
		out = (out << 1) | ((v >> uint(i)) & 1)
	}
	return out
}

func phase3iVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "FCB variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.m.corr-prod.corr > 0.05 {
		return "FCB variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no simple FCB sign/position/enhancement variant improves output; gain reconstruction is the next audit target"
}
