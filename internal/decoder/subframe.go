package decoder

import (
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

const (
	erasurePitchGainAttenuationQ15 = int64(29491) // 0.9
	erasureFixedGainAttenuationQ15 = int64(32112) // 0.98
	initialErasurePitchDelay       = 60
	initialErasureRandomSeed       = uint16(21845)
)

// decodeSubframe runs the per-subframe pipeline and writes 40 final PCM
// samples to out[0:40].
//
// sfA     — Q12 LP coefficients for this subframe (from lsp.Decoder)
// tInt    — integer pitch delay (from pitch.DecodeDelaySubframe*)
// tFrac   — fractional pitch delay ∈ {-1, 0, 1}
// C, S    — FCB position (13-bit) and sign (4-bit) indices
// GA, GB  — gain VQ stage-1 (3-bit) and stage-2 (4-bit) indices
//
// Effect: advances d.pastExc, d.prevGpQ14, d.gn (MA predictor FIFO),
// d.syn (pastSynth), d.pst (postfilter state), d.hpX, d.hpY.
func (d *Decoder) decodeSubframe(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)
	d.rememberFixedGain(decoderGainQ14FromMantExp(gcMant, gcExp))

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	commitU := u
	if shift := d.syn.LastExcitationScaleShift(); shift > 0 {
		scalePastExcitationHistory(&d.pastExc, shift)
		scaleExcitationForHistory(&commitU, shift)
	}

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	d.hpFilterFinal(&sPf, out[:subframeLen])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], commitU[:])

	d.rememberPitchGain(gpQ14)
	d.rememberPitchDelay(tInt)
}

func (d *Decoder) decodeErasureSubframe(
	sfA *[lpcOrder + 1]int16,
	tInt int,
	out []int16,
) {
	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, 0, d.pastExc[:], &v)

	var c [subframeLen]int16
	d.decodeErasureFixedCodebook(tInt, &c)
	gpQ14, gcMant, gcExp := d.concealErasureGains()

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	commitU := u
	if shift := d.syn.LastExcitationScaleShift(); shift > 0 {
		scalePastExcitationHistory(&d.pastExc, shift)
		scaleExcitationForHistory(&commitU, shift)
	}

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	d.hpFilterFinal(&sPf, out[:subframeLen])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], commitU[:])

	d.rememberPitchGain(gpQ14)
	d.rememberPitchDelay(tInt)
}

func (d *Decoder) pitchEnhancementBetaQ14() int16 {
	if !d.havePrevGpQ14 {
		return fcb.InitialPitchEnhancementQ14
	}
	return fcb.ClampPitchGainForEnhancement(d.prevGpQ14)
}

func (d *Decoder) rememberPitchGain(gpQ14 int16) {
	d.prevGpQ14 = gpQ14
	d.havePrevGpQ14 = true
}

func (d *Decoder) rememberFixedGain(gainQ14 int64) {
	d.prevFixedGainQ14 = gainQ14
}

func (d *Decoder) rememberPitchDelay(tInt int) {
	d.prevPitchDelay = tInt
	d.havePrevPitchDelay = true
}

func (d *Decoder) concealedPitchDelay() int {
	if !d.havePrevPitchDelay {
		return initialErasurePitchDelay
	}
	return d.prevPitchDelay
}

func nextConcealedPitchDelay(tInt int) int {
	if tInt < pitchMax {
		return tInt + 1
	}
	return pitchMax
}

func (d *Decoder) concealErasureGains() (gpQ14, gcMantQ14 int16, gcExp int8) {
	gpQ14, gcMantQ14, gcExp, _ = d.concealErasureGainsWithTrace()
	return gpQ14, gcMantQ14, gcExp
}

type erasureGainTrace struct {
	pastErrorsBefore [4]int16
	pastErrorsAfter  [4]int16
	avgQ10           int32
	updateQ10        int32
}

func (d *Decoder) concealErasureGainsWithTrace() (gpQ14, gcMantQ14 int16, gcExp int8, trace erasureGainTrace) {
	gp := int64(d.prevGpQ14)
	if !d.havePrevGpQ14 {
		gp = 0
	}
	gp = (gp * erasurePitchGainAttenuationQ15) >> 15
	if gp > 32767 {
		gp = 32767
	}
	if gp < 0 {
		gp = 0
	}

	fixedGainQ14 := (d.prevFixedGainQ14 * erasureFixedGainAttenuationQ15) >> 15
	fixedGainQ14 = quantizeDecoderFixedGainQ1(fixedGainQ14)
	gcMantQ14, gcExp = gain.SplitGainQ14(fixedGainQ14)

	trace.pastErrorsBefore = d.gn.PredictorErrors()
	trace.avgQ10 = (int32(trace.pastErrorsBefore[0]) + int32(trace.pastErrorsBefore[1]) +
		int32(trace.pastErrorsBefore[2]) + int32(trace.pastErrorsBefore[3])) >> 2
	trace.updateQ10 = trace.avgQ10 - 4096
	if trace.updateQ10 < int32(gain.PastErrorsDefault) {
		trace.updateQ10 = int32(gain.PastErrorsDefault)
	}
	d.gn.MarkErasure()
	trace.pastErrorsAfter = d.gn.PredictorErrors()
	d.prevGpQ14 = int16(gp)
	d.havePrevGpQ14 = true
	d.prevFixedGainQ14 = fixedGainQ14
	return int16(gp), gcMantQ14, gcExp, trace
}

func (d *Decoder) decodeErasureFixedCodebook(tInt int, c *[subframeLen]int16) {
	positions, signs := d.nextErasureFixedCodebookIndices()
	fcb.Decode(fcb.Indices{Positions: positions, Signs: signs}, tInt, d.pitchEnhancementBetaQ14(), c)
}

func (d *Decoder) nextErasureFixedCodebookIndices() (uint16, uint8) {
	positions := d.erasureRandom() & 0x1fff
	signs := d.erasureRandom() & 0x000f
	return positions, uint8(signs)
}

func (d *Decoder) currentErasureRandomSeed() uint16 {
	if !d.randomSeedInitialized {
		return initialErasureRandomSeed
	}
	return d.randomSeed
}

func (d *Decoder) erasureRandom() uint16 {
	if !d.randomSeedInitialized {
		d.randomSeed = initialErasureRandomSeed
		d.randomSeedInitialized = true
	}
	d.randomSeed = uint16(uint32(d.randomSeed)*31821 + 13849)
	return d.randomSeed
}

func decoderGainQ14FromMantExp(mant int16, exp int8) int64 {
	if mant == 0 {
		return 0
	}
	v := int64(mant)
	if exp >= 0 {
		shift := uint(exp)
		if shift >= 62 {
			return 1<<63 - 1
		}
		return v << shift
	}
	shift := uint(-int(exp))
	if shift >= 63 {
		return 0
	}
	return v >> shift
}

func quantizeDecoderFixedGainQ1(gainQ14 int64) int64 {
	if gainQ14 <= 0 {
		return 0
	}
	gainQ1 := gainQ14 >> 13
	if gainQ1 > 32767 {
		gainQ1 = 32767
	}
	return gainQ1 << 13
}

func decodeAdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc, v)
}

func decodeAdaptiveCodebookIntegerLagOnly(tInt int, pastExc []int16, v *[subframeLen]int16) {
	pitch.AdaptiveCodebook(tInt, 0, pastExc, v)
}

func scaleExcitationForHistory(u *[subframeLen]int16, shift uint) {
	for i, v := range u {
		u[i] = int16(int32(v) >> shift)
	}
}

func scalePastExcitationHistory(past *[pastExcLen]int16, shift uint) {
	for i, v := range past {
		past[i] = int16(int32(v) >> shift)
	}
}
