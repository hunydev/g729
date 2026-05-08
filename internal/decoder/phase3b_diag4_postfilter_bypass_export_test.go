package decoder

import (
	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// DecodeFrameNoPostfilter mirrors Decoder.Decode (decode.go) line-for-line
// EXCEPT the §A.4.2 postfilter chain (long-term + short-term + tilt + AGC)
// is bypassed: the synthesis-filter output s[n] is fed directly to the
// §4.2.2 output HP filter, then through pcm.ScaleUpSat.
//
// This is the Phase 3b DIAG-4 postfilter-bypass discriminator. It exists
// in a _test.go file so no production API surface is added; correctness
// is anchored by the structural identity to decode.go / subframe.go.
//
// Intentional differences from Decode:
//   - d.pst.Filter(...) call replaced by copy(sPf, s).
//
// State advanced as per Decode: lsp / gn / syn / pastExc / prevGpQ14 /
// hpX / hpY. NOTE: d.pst is NOT advanced because we never invoke it; a
// Decoder used via DecodeFrameNoPostfilter must therefore not be reused
// for normal Decode calls (the postfilter memory would diverge).
func (d *Decoder) DecodeFrameNoPostfilter(packed []byte, out []int16) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	d.decodeSubframeNoPF(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen])
	d.decodeSubframeNoPF(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples])

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframeNoPF(
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

	// Postfilter BYPASSED: feed s directly to HP filter.
	var hpOut [subframeLen]int16
	d.hpFilter(&s, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.prevGpQ14 = gpQ14
}
