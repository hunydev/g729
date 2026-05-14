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

// TestPhase3jGainVariantAudit_SPEECH probes gain reconstruction variants with
// final-output black-box metrics. A local mirror of the gain decoder is first
// checked against production, then small Q-format/predictor variants are run.
func TestPhase3jGainVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_AUDIT=1 to audit gain variants")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_GAIN_VECTOR", "SPEECH")
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.pstFile, err)
	}

	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)

	production := decodeVariant(t, bitData, frames, nil, nil)
	mirror := phase3jDecodeVariant(t, bitData, frames, phase3jVariant{name: "gain_mirror_default"})
	if !phase3eEqualPCM(production, mirror) {
		t.Fatalf("phase3j gain mirror diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, production, 40)

	variants := []phase3jVariant{
		{name: "production"},
		{name: "gain_mirror_default", mode: phase3jGainMirrorDefault},
		{name: "gain_loggain_sat16", mode: phase3jGainLogGainSat16},
		{name: "gain_legacy_q12_build", mode: phase3jGainLegacyQ12Build},
		{name: "gain_ec_q26", mode: phase3jGainECQ26},
		{name: "gain_ec_q25", mode: phase3jGainECQ25},
		{name: "gain_ec_q27", mode: phase3jGainECQ27},
		{name: "gain_ec_q13", mode: phase3jGainECQ13},
		{name: "gain_gamma_q12", mode: phase3jGainGammaQ12},
		{name: "gain_gamma_q14", mode: phase3jGainGammaQ14},
		{name: "gain_ec25_gamma14", mode: phase3jGainEC25GammaQ14},
		{name: "gain_ec25_gamma12", mode: phase3jGainEC25GammaQ12},
		{name: "gain_ignore_gamma_log", mode: phase3jGainIgnoreGammaLog},
		{name: "gain_loggain_i32", mode: phase3jGainLogGainI32},
		{name: "gain_pred_i32", mode: phase3jGainPredictedI32},
		{name: "gain_pred_i32_ec27", mode: phase3jGainPredictedI32EC27},
		{name: "gain_update_10log", mode: phase3jGainUpdate10Log},
		{name: "gain_update_default", mode: phase3jGainUpdateDefault},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3j gain variant audit - %s/%s (%d frames)", tc.bitFile, tc.pstFile, frames)
	t.Logf("baseline production: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR)
	t.Logf("")
	t.Logf("%-26s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "lagSNR", "bestSNR")
	t.Logf("%-26s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "------", "-------")
	for _, v := range variants {
		out := production
		if v.mode != phase3jGainProduction {
			out = phase3jDecodeVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		rows = append(rows, row{name: v.name, m: m})
		t.Logf("%-26s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %10d %10.2f",
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
	t.Logf("verdict: %s", phase3jVerdict(prodMetrics, best, bestCorr))
}

type phase3jGainMode int

const (
	phase3jGainProduction phase3jGainMode = iota
	phase3jGainMirrorDefault
	phase3jGainLogGainSat16
	phase3jGainLegacyQ12Build
	phase3jGainECQ26
	phase3jGainECQ25
	phase3jGainECQ27
	phase3jGainECQ13
	phase3jGainGammaQ12
	phase3jGainGammaQ14
	phase3jGainEC25GammaQ14
	phase3jGainEC25GammaQ12
	phase3jGainIgnoreGammaLog
	phase3jGainLogGainI32
	phase3jGainPredictedI32
	phase3jGainPredictedI32EC27
	phase3jGainUpdate10Log
	phase3jGainUpdateDefault
)

const (
	phase3jPastErrorsDefault = -14336
	phase3jDBPerLog2Q13      = 24660
	phase3jTenLog10_40Q10    = 16405
	phase3jInvDBScaleQ15     = 5443
	phase3jDBPerLog2Q10      = 6165
)

type phase3jVariant struct {
	name string
	mode phase3jGainMode
}

type phase3jGainParams struct {
	ecQCorrection  int
	gammaQCorr     int
	ignoreGammaLog bool
	logGainI32     bool
	predictedI32   bool
	updateFactor   int32
	updateDefault  bool
	legacyQ12Build bool
}

type phase3jGainOut struct {
	gpQ14     int16
	gcMantQ14 int16
	gcExp     int8
	gcQ12     int16
}

type phase3jGainDecoder struct {
	initialized bool
	pastErrors  [4]int16
}

func phase3jDecodeVariant(t *testing.T, bitData []byte, frames int, variant phase3jVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3jGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3jVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3jVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3jVariant(packed []byte, out []int16, gd *phase3jGainDecoder, variant phase3jVariant) error {
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

	d.decodeSubframePhase3jVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], gd, variant)
	d.decodeSubframePhase3jVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], gd, variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3jVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	gd *phase3jGainDecoder,
	variant phase3jVariant,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gout := gd.decode(gain.Indices{GA: GA, GB: GB}, &c, phase3jParams(variant.mode))

	var u [subframeLen]int16
	if phase3jParams(variant.mode).legacyQ12Build {
		phase3jBuildExcitationQ12(gout.gpQ14, gout.gcQ12, &v, &c, &u)
	} else {
		synth.BuildExcitation(gout.gpQ14, gout.gcMantQ14, gout.gcExp, &v, &c, &u)
	}

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.rememberPitchGain(gout.gpQ14)
}

func phase3jParams(mode phase3jGainMode) phase3jGainParams {
	p := phase3jGainParams{
		ecQCorrection: 26,
		gammaQCorr:    13,
		logGainI32:    true,
		predictedI32:  true,
		updateFactor:  phase3jDBPerLog2Q10,
	}
	switch mode {
	case phase3jGainLogGainSat16:
		p.logGainI32 = false
		p.predictedI32 = false
	case phase3jGainLegacyQ12Build:
		p.legacyQ12Build = true
	case phase3jGainECQ26:
		p.ecQCorrection = 26
	case phase3jGainECQ25:
		p.ecQCorrection = 25
	case phase3jGainECQ27:
		p.ecQCorrection = 27
	case phase3jGainECQ13:
		p.ecQCorrection = 13
	case phase3jGainGammaQ12:
		p.gammaQCorr = 12
	case phase3jGainGammaQ14:
		p.gammaQCorr = 14
	case phase3jGainEC25GammaQ14:
		p.ecQCorrection = 25
		p.gammaQCorr = 14
	case phase3jGainEC25GammaQ12:
		p.ecQCorrection = 25
		p.gammaQCorr = 12
	case phase3jGainIgnoreGammaLog:
		p.ignoreGammaLog = true
	case phase3jGainLogGainI32:
		p.logGainI32 = true
	case phase3jGainPredictedI32:
		p.logGainI32 = true
		p.predictedI32 = true
	case phase3jGainPredictedI32EC27:
		p.ecQCorrection = 27
		p.logGainI32 = true
		p.predictedI32 = true
	case phase3jGainUpdate10Log:
		p.updateFactor = phase3jDBPerLog2Q10 / 2
	case phase3jGainUpdateDefault:
		p.updateDefault = true
	}
	return p
}

func (d *phase3jGainDecoder) decode(idx gain.Indices, c *[subframeLen]int16, params phase3jGainParams) phase3jGainOut {
	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = phase3jPastErrorsDefault
		}
		d.initialized = true
	}

	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		gp, _ := phase3jDecodeVQ(idx)
		d.shiftPastErrors(phase3jPastErrorsDefault)
		return phase3jGainOut{gpQ14: gp}
	}

	predictedI32 := gain.PredictedLogGain(&d.pastErrors)
	predictedSat16 := fixed.Saturate(fixed.Word32(predictedI32))
	predictedForLog := predictedI32
	if !params.predictedI32 {
		predictedForLog = int32(predictedSat16)
	}
	ecLog2Q10 := int32(gain.Log2Fixed(ecEnergy)) - int32(params.ecQCorrection)*1024
	ecDBQ10 := (ecLog2Q10*phase3jDBPerLog2Q13 + (1 << 12)) >> 13
	ecBarDBQ10 := fixed.Saturate(fixed.Word32(ecDBQ10 - phase3jTenLog10_40Q10))
	logGainDBQ10 := int32(fixed.Sub(predictedSat16, ecBarDBQ10))
	if params.logGainI32 {
		logGainDBQ10 = predictedForLog - int32(ecBarDBQ10)
	}
	log2GcQ10 := (logGainDBQ10*phase3jInvDBScaleQ15 + (1 << 14)) >> 15

	gp, gammaC := phase3jDecodeVQ(idx)
	out := phase3jGainOut{gpQ14: gp}

	gc0Q14 := gain.Pow2Fixed(fixed.Word32(log2GcQ10) + 14*1024)
	prod64 := (int64(gammaC) * int64(gc0Q14)) >> 15
	if prod64 > 32767 {
		out.gcQ12 = 32767
	} else if prod64 < -32768 {
		out.gcQ12 = -32768
	} else {
		out.gcQ12 = int16(prod64)
	}

	if gammaC > 0 {
		gammaLog2Q10 := int32(0)
		if !params.ignoreGammaLog {
			gammaLog2Q10 = int32(gain.Log2Fixed(fixed.Word32(gammaC))) - int32(params.gammaQCorr)*1024
		}
		log2GcWithGammaQ10 := log2GcQ10 + gammaLog2Q10
		intPart := log2GcWithGammaQ10 >> 10
		frac := log2GcWithGammaQ10 - (intPart << 10)
		out.gcMantQ14 = gain.Pow2FracQ14(frac)
		switch {
		case intPart > 127:
			out.gcExp = 127
		case intPart < -128:
			out.gcExp = -128
		default:
			out.gcExp = int8(intPart)
		}
	}

	uCurrent := int16(phase3jPastErrorsDefault)
	if gammaC > 0 && !params.updateDefault {
		gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaC))) - 13*1024
		val := (gammaLog2Q10*params.updateFactor + (1 << 9)) >> 10
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		uCurrent = int16(val)
	}
	d.shiftPastErrors(uCurrent)
	return out
}

func (d *phase3jGainDecoder) shiftPastErrors(v int16) {
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = v
}

func phase3jDecodeVQ(idx gain.Indices) (gpQ14, gammaCQ13 int16) {
	gaEntry := tables.GainImap1[idx.GA]
	gbEntry := tables.GainImap2[idx.GB]
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][0]), fixed.Word16(tables.GainGBK2[gbEntry][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][1]), fixed.Word16(tables.GainGBK2[gbEntry][1])))
	return gpQ14, gammaCQ13
}

func phase3jBuildExcitationQ12(gpQ14, gcQ12 int16, v, c *[subframeLen]int16, u *[subframeLen]int16) {
	for n := 0; n < subframeLen; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
		lSum := fixed.LAdd(lPitch, lCode)
		u[n] = int16(fixed.Round(fixed.LShl(lSum, 1)))
	}
}

func phase3jVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.name != "gain_mirror_default" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "gain variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.name != "gain_mirror_default" && bestCorr.m.corr-prod.corr > 0.05 {
		return "gain variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no tested gain Q-format/predictor variant recovers output; remaining defect is likely a deeper shared reconstruction issue"
}
