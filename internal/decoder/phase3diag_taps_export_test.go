package decoder

import (
	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
)

// Phase3DiagSubframeTaps records the per-subframe intermediate signals
// captured by DecodeWithTaps. Test-only — exists in a _test.go file so
// no production API surface is added.
type Phase3DiagSubframeTaps struct {
	TInt           int
	TFrac          int
	GpQ14          int16
	GcQ12          int16
	A              [lpcOrder + 1]int16 // synthesis LP coefficients (Q12)
	PastExcPreACB  [pastExcLen]int16   // past excitation before adaptive codebook (Q0)
	V              [40]int16           // adaptive codebook vector  (Q0)
	C              [40]int16           // fixed codebook vector     (Q13)
	U              [40]int16           // total excitation          (Q0, after BuildExcitation)
	S              [40]int16           // post 1/Â(z)               (Q0, pre-postfilter)
	PFR            [40]int16           // postfilter residual       (Q0)
	PFLT           [40]int16           // postfilter long-term      (Q0, residual domain)
	PFST           [40]int16           // postfilter short-term     (Q0)
	PFT            [40]int16           // postfilter tilt           (Q0)
	SPf            [40]int16           // post postfilter           (Q0)
	HpOut          [40]int16           // post HP filter            (Q0, pre final output scaling)
	PostfilterTaps postfilter.FilterTaps

	// GainTaps captures the unsaturated 32-bit gain-decoder
	// intermediates (Phase 3a DIAG-1). Test-only; populated by
	// gain.Decoder.DecodeWithFullTaps which is called in place of
	// Decode in this taps pathway so the predictor is advanced
	// exactly once per subframe.
	GainTaps gain.GainDecodeFullTaps
}

// Phase3DiagFrameTaps groups the two subframes and the post-output-scale
// final 80-sample frame, plus the unpacked transmitted indices.
type Phase3DiagFrameTaps struct {
	Frame  bitstream.Frame
	Sub    [2]Phase3DiagSubframeTaps
	Output [80]int16
}

// DecodeWithTaps mirrors Decoder.Decode but captures the per-stage
// signals needed by phase3diag_02 / 03 diagnostic tests. The mirroring
// is line-for-line equivalent to decode.go / subframe.go; any drift
// would produce divergent Output relative to Decode and is not
// expected. Test-only helper.
func (d *Decoder) DecodeWithTaps(packed []byte) (Phase3DiagFrameTaps, error) {
	var taps Phase3DiagFrameTaps
	if len(packed) < bitstream.FrameBytes {
		return taps, ErrShortInput
	}
	if err := bitstream.Unpack(packed, &taps.Frame); err != nil {
		return taps, err
	}
	f := taps.Frame

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	d.decodeSubframeWithTaps(&sf1A, tInt1, tFrac1,
		f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1),
		taps.Output[:subframeLen], &taps.Sub[0])
	d.decodeSubframeWithTaps(&sf2A, tInt2, tFrac2,
		f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2),
		taps.Output[subframeLen:frameSamples], &taps.Sub[1])

	scaleDecoderOutput(taps.Output[:frameSamples])
	return taps, nil
}

func (d *Decoder) decodeSubframeWithTaps(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8, GA, GB uint8,
	out []int16,
	taps *Phase3DiagSubframeTaps,
) {
	taps.TInt = tInt
	taps.TFrac = tFrac
	taps.A = *sfA
	taps.PastExcPreACB = d.pastExc

	betaQ14 := d.pitchEnhancementBetaQ14()

	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &taps.V)

	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &taps.C)

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, &taps.C)
	taps.GainTaps = gainTaps
	gpQ14 := gainTaps.GpQ14Final
	gcQ12 := gainTaps.GcQ12Final
	taps.GpQ14 = gpQ14
	taps.GcQ12 = gcQ12

	synth.BuildExcitation(gpQ14, gainTaps.GcMantQ14, gainTaps.GcExp, &taps.V, &taps.C, &taps.U)

	d.syn.Filter(sfA, &taps.U, &taps.S)

	pfTaps := d.pst.FilterWithTaps(sfA, tInt, &taps.S)
	taps.PostfilterTaps = pfTaps
	taps.PFR = pfTaps.Residual
	taps.PFLT = pfTaps.LongTerm
	taps.PFST = pfTaps.ShortTerm
	taps.PFT = pfTaps.Tilt
	taps.SPf = pfTaps.Output

	d.hpFilter(&taps.SPf, taps.HpOut[:])
	copy(out[:subframeLen], taps.HpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], taps.U[:])

	d.rememberPitchGain(gpQ14)
}
