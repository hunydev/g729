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
	HPTaps         hpFilterTaps
	PostfilterTaps postfilter.FilterTaps

	ErasureRandomSeedBefore uint16
	ErasureRandomSeedAfter  uint16
	FixedCodebookIndex      uint16
	FixedCodebookSigns      uint8

	AdaptiveGainBeforeQ14 int16
	AdaptiveGainAfterQ14  int16
	FixedGainBeforeQ14    int64
	FixedGainAfterQ14     int64

	GainPredictorErrorBefore [4]int16
	GainPredictorErrorAfter  [4]int16
	GainErasureAvgQ10        int32
	GainErasureUpdateQ10     int32

	SynthMemBefore [lpcOrder]int16
	SynthMemAfter  [lpcOrder]int16

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
	Frame                 bitstream.Frame
	LSPPastResidualBefore [4][lpcOrder]int16
	LSPPastResidualAfter  [4][lpcOrder]int16
	PrevLSPBefore         [lpcOrder]int16
	PrevLSPAfter          [lpcOrder]int16
	PrevLSFBefore         [lpcOrder]int16
	PrevLSFAfter          [lpcOrder]int16
	LSFAfterPredictor     [lpcOrder]int16
	LSFAfterStability     [lpcOrder]int16
	CurrLSP               [lpcOrder]int16
	Sub                   [2]Phase3DiagSubframeTaps
	Output                [80]int16
}

// DecodeWithTaps mirrors Decoder.Decode but captures the per-stage
// signals needed by phase3diag_02 / 03 diagnostic tests. The mirroring
// is line-for-line equivalent to decode.go / subframe.go; any drift
// would produce divergent Output relative to Decode and is not
// expected. Test-only helper.
func (d *Decoder) DecodeWithTaps(packed []byte) (Phase3DiagFrameTaps, error) {
	return d.DecodeWithTapsBad(packed, false)
}

func (d *Decoder) DecodeWithTapsBad(packed []byte, bad bool) (Phase3DiagFrameTaps, error) {
	var taps Phase3DiagFrameTaps
	if len(packed) < bitstream.FrameBytes {
		return taps, ErrShortInput
	}
	if err := bitstream.Unpack(packed, &taps.Frame); err != nil {
		return taps, err
	}
	f := taps.Frame

	taps.PrevLSPBefore = d.lsp.PrevLSP()
	taps.PrevLSFBefore = d.lsp.PrevLSF()
	taps.LSPPastResidualBefore = d.lsp.PastResiduals()

	if bad {
		sf1A, sf2A := d.lsp.DecodeErasure()
		taps.PrevLSPAfter = d.lsp.PrevLSP()
		taps.PrevLSFAfter = d.lsp.PrevLSF()
		taps.LSPPastResidualAfter = d.lsp.PastResiduals()
		taps.LSFAfterPredictor = d.lsp.LastLSFAfterPredictor()
		taps.LSFAfterStability = d.lsp.LastLSFAfterStability()
		taps.CurrLSP = d.lsp.LastCurrLSP()
		tInt1 := d.concealedPitchDelay()
		tInt2 := nextConcealedPitchDelay(tInt1)
		d.decodeErasureSubframeWithTaps(&sf1A, tInt1, taps.Output[:subframeLen], &taps.Sub[0])
		d.decodeErasureSubframeWithTaps(&sf2A, tInt2, taps.Output[subframeLen:frameSamples], &taps.Sub[1])
		d.rememberPitchDelay(nextConcealedPitchDelay(tInt2))
		return taps, nil
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})
	taps.PrevLSPAfter = d.lsp.PrevLSP()
	taps.PrevLSFAfter = d.lsp.PrevLSF()
	taps.LSPPastResidualAfter = d.lsp.PastResiduals()
	taps.LSFAfterPredictor = d.lsp.LastLSFAfterPredictor()
	taps.LSFAfterStability = d.lsp.LastLSFAfterStability()
	taps.CurrLSP = d.lsp.LastCurrLSP()

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	if !pitch.CheckParity(uint8(f.P1), uint8(f.P0)) {
		tInt1 = d.concealedPitchDelay()
		tFrac1 = 0
	}
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	d.decodeSubframeWithTaps(&sf1A, tInt1, tFrac1,
		f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1),
		taps.Output[:subframeLen], &taps.Sub[0])
	d.decodeSubframeWithTaps(&sf2A, tInt2, tFrac2,
		f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2),
		taps.Output[subframeLen:frameSamples], &taps.Sub[1])

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
	taps.ErasureRandomSeedBefore = d.currentErasureRandomSeed()
	taps.ErasureRandomSeedAfter = taps.ErasureRandomSeedBefore
	taps.FixedCodebookIndex = C
	taps.FixedCodebookSigns = S
	if d.havePrevGpQ14 {
		taps.AdaptiveGainBeforeQ14 = d.prevGpQ14
	}
	taps.FixedGainBeforeQ14 = d.prevFixedGainQ14

	betaQ14 := d.pitchEnhancementBetaQ14()

	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &taps.V)

	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &taps.C)

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, &taps.C)
	taps.GainTaps = gainTaps
	gpQ14 := gainTaps.GpQ14Final
	gcQ12 := gainTaps.GcQ12Final
	taps.GpQ14 = gpQ14
	taps.GcQ12 = gcQ12
	taps.AdaptiveGainAfterQ14 = gpQ14
	taps.FixedGainAfterQ14 = decoderGainQ14FromMantExp(gainTaps.GcMantQ14, gainTaps.GcExp)
	taps.GainPredictorErrorBefore = gainTaps.PastErrorsBefore
	taps.GainPredictorErrorAfter = gainTaps.PastErrorsAfter
	d.rememberFixedGain(taps.FixedGainAfterQ14)

	synth.BuildExcitation(gpQ14, gainTaps.GcMantQ14, gainTaps.GcExp, &taps.V, &taps.C, &taps.U)

	taps.SynthMemBefore = d.syn.PastSynth()
	d.syn.Filter(sfA, &taps.U, &taps.S)
	taps.SynthMemAfter = d.syn.PastSynth()
	commitU := taps.U
	if shift := d.syn.LastExcitationScaleShift(); shift > 0 {
		scalePastExcitationHistory(&d.pastExc, shift)
		scaleExcitationForHistory(&commitU, shift)
	}

	pfTaps := d.pst.FilterWithTaps(sfA, tInt, &taps.S)
	taps.PostfilterTaps = pfTaps
	taps.PFR = pfTaps.Residual
	taps.PFLT = pfTaps.LongTerm
	taps.PFST = pfTaps.ShortTerm
	taps.PFT = pfTaps.Tilt
	taps.SPf = pfTaps.Output

	d.hpFilterCore(&taps.SPf, taps.HpOut[:], out[:subframeLen], &taps.HPTaps)

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], commitU[:])

	d.rememberPitchGain(gpQ14)
	d.rememberPitchDelay(tInt)
}

func (d *Decoder) decodeErasureSubframeWithTaps(
	sfA *[lpcOrder + 1]int16,
	tInt int,
	out []int16,
	taps *Phase3DiagSubframeTaps,
) {
	taps.TInt = tInt
	taps.TFrac = 0
	taps.A = *sfA
	taps.PastExcPreACB = d.pastExc
	taps.ErasureRandomSeedBefore = d.currentErasureRandomSeed()
	if d.havePrevGpQ14 {
		taps.AdaptiveGainBeforeQ14 = d.prevGpQ14
	}
	taps.FixedGainBeforeQ14 = d.prevFixedGainQ14

	decodeAdaptiveCodebook(tInt, 0, d.pastExc[:], &taps.V)
	positions, signs := d.nextErasureFixedCodebookIndices()
	taps.ErasureRandomSeedAfter = d.currentErasureRandomSeed()
	taps.FixedCodebookIndex = positions
	taps.FixedCodebookSigns = signs
	fcb.Decode(fcb.Indices{Positions: positions, Signs: signs}, tInt, d.pitchEnhancementBetaQ14(), &taps.C)

	gpQ14, gcMantQ14, gcExp, erasureGainTaps := d.concealErasureGainsWithTrace()
	taps.GpQ14 = gpQ14
	taps.GainTaps.GpQ14Final = gpQ14
	taps.GainTaps.GcMantQ14 = gcMantQ14
	taps.GainTaps.GcExp = gcExp
	taps.AdaptiveGainAfterQ14 = gpQ14
	taps.FixedGainAfterQ14 = d.prevFixedGainQ14
	taps.GainPredictorErrorBefore = erasureGainTaps.pastErrorsBefore
	taps.GainPredictorErrorAfter = erasureGainTaps.pastErrorsAfter
	taps.GainErasureAvgQ10 = erasureGainTaps.avgQ10
	taps.GainErasureUpdateQ10 = erasureGainTaps.updateQ10

	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &taps.V, &taps.C, &taps.U)

	taps.SynthMemBefore = d.syn.PastSynth()
	d.syn.Filter(sfA, &taps.U, &taps.S)
	taps.SynthMemAfter = d.syn.PastSynth()
	commitU := taps.U
	if shift := d.syn.LastExcitationScaleShift(); shift > 0 {
		scalePastExcitationHistory(&d.pastExc, shift)
		scaleExcitationForHistory(&commitU, shift)
	}

	pfTaps := d.pst.FilterWithTaps(sfA, tInt, &taps.S)
	taps.PostfilterTaps = pfTaps
	taps.PFR = pfTaps.Residual
	taps.PFLT = pfTaps.LongTerm
	taps.PFST = pfTaps.ShortTerm
	taps.PFT = pfTaps.Tilt
	taps.SPf = pfTaps.Output

	d.hpFilterCore(&taps.SPf, taps.HpOut[:], out[:subframeLen], &taps.HPTaps)

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], commitU[:])

	d.rememberPitchGain(gpQ14)
	d.rememberPitchDelay(tInt)
}
