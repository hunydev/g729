package decoder

import (
	"math"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

const (
	envelopeRecoveryMinRMS = 500
	envelopeRecoveryMin    = 0.25
	envelopeRecoveryMax    = 3.0

	envelopeRecoveryOverhangRMS = 500
	envelopeRecoveryOverhangU   = 2
	envelopeRecoveryOverhangMul = 0.5
)

var envelopeRecoveryCoeff = [...]float64{
	// Coefficients are fit from local stage taps against FFmpeg executable
	// black-box envelope labels; no external implementation source is used.
	-0.329, // intercept
	+0.057, // log1p(outRMS)
	+0.200, // log1p(uRMS)
	-0.341, // log1p(sRMS)
	+0.190, // log1p(gcMax)
	+0.149, // fixedRMS/uRMS
	+0.600, // pitchRMS/uRMS
	+0.009, // sRMS/uRMS
	+0.053, // frame contains GA036-like gain index
}

// DecodeEnvelopeRecovered decodes one frame through an opt-in, non-strict
// listening path and applies bounded frame-level envelope recovery. The base
// decoder path remains Decode; this method is for black-box quality
// diagnostics and gateways that prefer speech audibility over strict decoder
// behavior.
func (d *Decoder) DecodeEnvelopeRecovered(packed []byte, bad bool, out []int16) error {
	return d.decodeEnvelopeRecoveredWithLogCorrections(packed, bad, out, 26, 14)
}

func (d *Decoder) decodeEnvelopeRecoveredWithLogCorrections(packed []byte, bad bool, out []int16, ecQCorrection, gammaQCorrection int) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}
	_ = bad

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0),
		L1: uint8(f.L1),
		L2: uint8(f.L2),
		L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	var stats envelopeRecoveryStats
	stats.hasGA036 = envelopeRecoveryHasGA036(uint8(f.GA1)) || envelopeRecoveryHasGA036(uint8(f.GA2))
	d.decodeSubframeEnvelopeRecovered(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen], &stats, ecQCorrection, gammaQCorrection)
	d.decodeSubframeEnvelopeRecovered(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples], &stats, ecQCorrection, gammaQCorrection)

	scaleDecoderOutputForEnvelopeRecovery(out[:frameSamples])
	applyEnvelopeRecovery(out[:frameSamples], &stats)
	return nil
}

type envelopeRecoveryStats struct {
	pitchE   float64
	fixedE   float64
	uE       float64
	sE       float64
	gcMax    float64
	hasGA036 bool
}

func (d *Decoder) decodeSubframeEnvelopeRecovered(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	stats *envelopeRecoveryStats,
	ecQCorrection, gammaQCorrection int,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.DecodeWithLogCorrections(gain.Indices{GA: GA, GB: GB}, &c, ecQCorrection, gammaQCorrection)
	gcSigned := float64(gcMant) * math.Exp2(float64(gcExp)-14)
	gcAbs := math.Abs(gcSigned)
	if gcAbs > stats.gcMax {
		stats.gcMax = gcAbs
	}

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	gp := float64(gpQ14) / 16384.0
	for n := 0; n < subframeLen; n++ {
		pitchPart := gp * float64(v[n])
		fixedPart := gcSigned * float64(c[n]) / 8192.0
		stats.pitchE += pitchPart * pitchPart
		stats.fixedE += fixedPart * fixedPart
		stats.uE += float64(u[n]) * float64(u[n])
		stats.sE += float64(s[n]) * float64(s[n])
	}

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.prevGpQ14 = gpQ14
}

func applyEnvelopeRecovery(out []int16, stats *envelopeRecoveryStats) {
	outRMS := envelopeRecoveryRMS(out)
	uRMS := math.Sqrt(stats.uE / frameSamples)
	if uRMS <= 0 {
		return
	}
	if outRMS < envelopeRecoveryMinRMS {
		applyEnvelopeOverhangDamping(out, outRMS, uRMS)
		return
	}
	sRMS := math.Sqrt(stats.sE / frameSamples)
	features := [...]float64{
		1,
		math.Log1p(outRMS),
		math.Log1p(uRMS),
		math.Log1p(sRMS),
		math.Log1p(stats.gcMax),
		math.Sqrt(stats.fixedE/frameSamples) / uRMS,
		math.Sqrt(stats.pitchE/frameSamples) / uRMS,
		sRMS / uRMS,
		envelopeRecoveryBool(stats.hasGA036),
	}
	var logScale float64
	for i, c := range envelopeRecoveryCoeff {
		logScale += c * features[i]
	}
	scale := math.Exp(logScale)
	if scale < envelopeRecoveryMin {
		scale = envelopeRecoveryMin
	} else if scale > envelopeRecoveryMax {
		scale = envelopeRecoveryMax
	}
	for i, v := range out {
		out[i] = envelopeRecoveryScale(v, scale)
	}
	applyEnvelopeOverhangDamping(out, envelopeRecoveryRMS(out), uRMS)
}

func applyEnvelopeOverhangDamping(out []int16, outRMS, uRMS float64) {
	if outRMS >= envelopeRecoveryOverhangRMS || uRMS >= envelopeRecoveryOverhangU {
		return
	}
	for i, v := range out {
		out[i] = envelopeRecoveryScale(v, envelopeRecoveryOverhangMul)
	}
}

func envelopeRecoveryHasGA036(ga uint8) bool {
	return ga == 0 || ga == 3 || ga == 6
}

func envelopeRecoveryBool(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func envelopeRecoveryRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var e float64
	for _, v := range samples {
		x := float64(v)
		e += x * x
	}
	return math.Sqrt(e / float64(len(samples)))
}

func envelopeRecoveryScale(v int16, scale float64) int16 {
	x := math.Round(float64(v) * scale)
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return int16(x)
}
