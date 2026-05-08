package decoder

import (
	"bytes"
	"os"
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

// TestPhase3hPitchACBVariantAudit_SPEECH probes the adaptive-codebook
// reconstruction surface with final-output black-box metrics. Each variant
// changes only the ACB vector v; the decoded pitch delay still feeds FCB
// enhancement and postfilter as in production.
func TestPhase3hPitchACBVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_PITCH_ACB_AUDIT") != "1" {
		t.Skip("set G729_DECODER_PITCH_ACB_AUDIT=1 to audit pitch ACB variants")
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

	production := phase3hDecodeVariant(t, bitData, frames, phase3hVariant{name: "production"})
	productionViaDecode := decodeVariant(t, bitData, frames, nil, nil)
	if !phase3eEqualPCM(production, productionViaDecode) {
		t.Fatalf("phase3h production variant diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, production, 40)
	pitchStats := phase3hCollectPitchStats(t, bitData, frames)

	variants := []phase3hVariant{
		{name: "production"},
		{name: "acb_fractional_interpolation", mode: phase3hACBFractionalInterpolation},
		{name: "acb_delay_minus_1", mode: phase3hACBDelayMinus1},
		{name: "acb_delay_plus_1", mode: phase3hACBDelayPlus1},
		{name: "acb_frac_sign_flip", mode: phase3hACBFracSignFlip},
		{name: "acb_frac_ignore_integer", mode: phase3hACBFracIgnoreInteger},
		{name: "acb_frac_phase_swap", mode: phase3hACBFracPhaseSwap},
		{name: "acb_frac_phase_minus_1", mode: phase3hACBFracPhaseMinus1},
		{name: "acb_frac_phase_plus_1", mode: phase3hACBFracPhasePlus1},
		{name: "acb_frac_neg_no_k_adj", mode: phase3hACBFracNegNoKAdjust},
		{name: "acb_frac_pos_k_minus_1", mode: phase3hACBFracPosKMinus1},
		{name: "acb_frac_forward_arm", mode: phase3hACBFracForwardArm},
		{name: "acb_short_no_periodic", mode: phase3hACBShortNoPeriodic},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3h pitch ACB variant audit - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("pitch subframes: total=%d frac[-1]=%d frac[0]=%d frac[+1]=%d short(T<40)=%d",
		pitchStats.total, pitchStats.fracNeg, pitchStats.fracZero, pitchStats.fracPos, pitchStats.shortPitch)
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
		if v.mode != phase3hACBProduction {
			out = phase3hDecodeVariant(t, bitData, frames, v)
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
	t.Logf("verdict: %s", phase3hVerdict(prodMetrics, best, bestCorr))
}

type phase3hACBMode int

const (
	phase3hACBProduction phase3hACBMode = iota
	phase3hACBFractionalInterpolation
	phase3hACBDelayMinus1
	phase3hACBDelayPlus1
	phase3hACBFracSignFlip
	phase3hACBFracIgnoreInteger
	phase3hACBFracPhaseSwap
	phase3hACBFracPhaseMinus1
	phase3hACBFracPhasePlus1
	phase3hACBFracNegNoKAdjust
	phase3hACBFracPosKMinus1
	phase3hACBFracForwardArm
	phase3hACBShortNoPeriodic
)

type phase3hVariant struct {
	name string
	mode phase3hACBMode
}

type phase3hPitchStats struct {
	total      int
	fracNeg    int
	fracZero   int
	fracPos    int
	shortPitch int
}

func phase3hDecodeVariant(t *testing.T, bitData []byte, frames int, variant phase3hVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3hVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3hVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3hVariant(packed []byte, out []int16, variant phase3hVariant) error {
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

	d.decodeSubframePhase3hVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3hVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3hVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	variant phase3hVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	phase3hAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v, variant.mode)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

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

func phase3hAdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16, mode phase3hACBMode) {
	switch mode {
	case phase3hACBProduction:
		decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
	case phase3hACBFractionalInterpolation:
		pitch.AdaptiveCodebook(tInt, tFrac, pastExc, v)
	case phase3hACBDelayMinus1:
		pitch.AdaptiveCodebook(phase3hClampDelay(tInt-1), tFrac, pastExc, v)
	case phase3hACBDelayPlus1:
		pitch.AdaptiveCodebook(phase3hClampDelay(tInt+1), tFrac, pastExc, v)
	case phase3hACBFracSignFlip:
		pitch.AdaptiveCodebook(tInt, -tFrac, pastExc, v)
	case phase3hACBFracIgnoreInteger:
		pitch.AdaptiveCodebook(tInt, 0, pastExc, v)
	case phase3hACBFracPhaseSwap, phase3hACBFracPhaseMinus1, phase3hACBFracPhasePlus1, phase3hACBFracNegNoKAdjust, phase3hACBFracPosKMinus1, phase3hACBFracForwardArm, phase3hACBShortNoPeriodic:
		phase3hAdaptiveCodebookCustom(tInt, tFrac, pastExc, v, mode)
	default:
		decodeAdaptiveCodebook(tInt, tFrac, pastExc, v)
	}
}

func phase3hClampDelay(tInt int) int {
	if tInt < 1 {
		return 1
	}
	if tInt >= len([pastExcLen]int16{}) {
		return pastExcLen - 1
	}
	return tInt
}

func phase3hAdaptiveCodebookCustom(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16, mode phase3hACBMode) {
	if mode == phase3hACBShortNoPeriodic && tInt < subframeLen {
		if tFrac == 0 {
			base := len(pastExc) - tInt
			for n := 0; n < tInt; n++ {
				if idx := base + n; idx >= 0 && idx < len(pastExc) {
					v[n] = pastExc[idx]
				}
			}
		} else {
			phase3hFIRInterpolate(tInt, tFrac, pastExc, v, 0, tInt, mode)
		}
		return
	}
	if tFrac == 0 {
		pitch.AdaptiveCodebook(tInt, tFrac, pastExc, v)
		return
	}
	if tInt >= subframeLen {
		phase3hFIRInterpolate(tInt, tFrac, pastExc, v, 0, subframeLen, mode)
		return
	}
	phase3hFIRInterpolate(tInt, tFrac, pastExc, v, 0, tInt, mode)
	for n := tInt; n < subframeLen; n++ {
		v[n] = v[n-tInt]
	}
}

func phase3hFIRInterpolate(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16, start, end int, mode phase3hACBMode) {
	k, posPhase, negPhase := phase3hFIRParams(tInt, tFrac, mode)
	base := len(pastExc) - k
	fir := tables.PitchInterpFIR
	N := len(pastExc)
	for n := start; n < end; n++ {
		var acc fixed.Word32
		for i := 0; i < pitch.Linter; i++ {
			backIdx := base + n - i
			if mode == phase3hACBFracForwardArm {
				backIdx = base + n + i
			}
			fwdIdx := base + n + 1 + i
			var back, fwd int16
			if backIdx >= 0 && backIdx < N {
				back = pastExc[backIdx]
			}
			if fwdIdx >= 0 && fwdIdx < N {
				fwd = pastExc[fwdIdx]
			}
			acc = fixed.LMac(acc, fir[posPhase+3*i], back)
			acc = fixed.LMac(acc, fir[negPhase+3*i], fwd)
		}
		v[n] = fixed.Round(acc)
	}
}

func phase3hFIRParams(tInt, tFrac int, mode phase3hACBMode) (k, posPhase, negPhase int) {
	switch tFrac {
	case -1:
		k, posPhase, negPhase = tInt, 1, 2
	case 1:
		k, posPhase, negPhase = tInt+1, 2, 1
	default:
		return tInt, 0, 0
	}
	switch mode {
	case phase3hACBFracPhaseSwap:
		posPhase, negPhase = negPhase, posPhase
	case phase3hACBFracPhaseMinus1:
		posPhase--
		negPhase--
	case phase3hACBFracPhasePlus1:
		posPhase++
		negPhase++
	case phase3hACBFracNegNoKAdjust:
		if tFrac == -1 {
			k = tInt - 1
		}
	case phase3hACBFracPosKMinus1:
		if tFrac == 1 {
			k = tInt
		}
	}
	return k, posPhase, negPhase
}

func phase3hCollectPitchStats(t *testing.T, bitData []byte, frames int) phase3hPitchStats {
	t.Helper()
	var stats phase3hPitchStats
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[pitch stats] frame %d: %v", f, err)
		}
		var fr bitstream.Frame
		if err := bitstream.Unpack(packed[:], &fr); err != nil {
			t.Fatalf("Unpack[pitch stats] frame %d: %v", f, err)
		}
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)
		phase3hCountPitch(&stats, tInt1, tFrac1)
		phase3hCountPitch(&stats, tInt2, tFrac2)
	}
	return stats
}

func phase3hCountPitch(stats *phase3hPitchStats, tInt, tFrac int) {
	stats.total++
	if tInt < subframeLen {
		stats.shortPitch++
	}
	switch tFrac {
	case -1:
		stats.fracNeg++
	case 0:
		stats.fracZero++
	case 1:
		stats.fracPos++
	}
}

func phase3hVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "ACB variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.m.corr-prod.corr > 0.05 {
		return "ACB variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no simple ACB delay/frac/periodicity variant improves output; continue gain and fixed-codebook numeric audits"
}
