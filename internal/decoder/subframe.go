package decoder

import (
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// decodeSubframe runs the per-subframe pipeline and writes 40 samples of
// post-HP, pre-final-output-scale Q0 PCM to out[0:40].
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
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

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

	d.prevGpQ14 = gpQ14
}

func decodeAdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[subframeLen]int16) {
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc, v)
}

func decodeAdaptiveCodebookIntegerLagOnly(tInt int, pastExc []int16, v *[subframeLen]int16) {
	pitch.AdaptiveCodebook(tInt, 0, pastExc, v)
}
