package decoder

import (
	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// DecodePostfilterBlend is a non-strict quality diagnostic path. It blends the
// postfilter output with the pre-postfilter synthesis before the output
// high-pass filter. A synthNum/den of 0 uses the strict Decode path; 1/2 means
// 50% postfilter + 50% synthesis.
func (d *Decoder) DecodePostfilterBlend(packed []byte, bad bool, out []int16, synthNum, den int) error {
	if den <= 0 || synthNum <= 0 {
		return d.Decode(packed, bad, out)
	}
	if synthNum > den {
		synthNum = den
	}
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

	d.decodeSubframePostfilterBlend(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen], synthNum, den)
	d.decodeSubframePostfilterBlend(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples], synthNum, den)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePostfilterBlend(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	synthNum, den int,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

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

	var hpIn [subframeLen]int16
	postNum := den - synthNum
	for i := 0; i < subframeLen; i++ {
		hpIn[i] = blendPostfilterSample(sPf[i], s[i], postNum, synthNum, den)
	}

	var hpOut [subframeLen]int16
	d.hpFilter(&hpIn, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.rememberPitchGain(gpQ14)
}

func blendPostfilterSample(postfiltered, synth int16, postNum, synthNum, den int) int16 {
	v := (int64(postfiltered)*int64(postNum) + int64(synth)*int64(synthNum) + int64(den/2)) / int64(den)
	if v > 32767 {
		v = 32767
	} else if v < -32768 {
		v = -32768
	}
	return int16(v)
}
