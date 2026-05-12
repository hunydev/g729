package g729

import (
	"errors"
	"io"
	"math"

	"github.com/hunydev/g729/internal/acelp"
	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/filter"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/lpc"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	pitchcore "github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/pitch/openloop"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// Encoder holds G.729 Annex A encoder state for one logical stream.
//
// All buffers are preallocated; EncodeFrame allocates 0 in steady state.
// Concurrent calls on the same Encoder are a data race; callers needing
// parallel encoding must own one Encoder per channel.
type Encoder struct {
	profile       EncoderProfile
	qualityTuning encoderQualityTuning

	pre pcm.PreProcessor

	// §5.3 preallocated histories.
	oldSpeech  [240]int16
	oldWspeech [143]int16
	oldExc     [154]int16
	synMem     [10]int16
	wMem       [10]int16
	errMem     [10]int16
	lspOld     [10]int16
	lspOldQ    [10]int16
	pastQuaEn  [4]int16
	freqPrev   [4][10]int16

	// FIX-3-B (Phase 2a INT-1 d10): count of frames that triggered the
	// anti-palindromic LP guard in lpcStep (LPToLSP returned
	// ErrLPCNonStable → previous-frame LSP reuse). Diagnostic-only;
	// not gating. Per spec §3.2.6 stability-and-reuse precedent.
	lspReuseCount uint64

	// Phase 2b INT-0: open-loop pitch state.
	//
	// aQ12Latest caches the order-10 analysis LP polynomial (Q12) produced
	// by the most recent lpcStep call for diagnostics. The production
	// openloopStep path uses aHatSF1/aHatSF2, i.e. the reconstructed
	// quantized LP polynomials, per Annex A §A.3.2.5 and §A.3.3.
	//
	// lpResidualMem and swMem are the §A.3.3 filter memories owned
	// by the encoder per the I10 / Phase-2a state-isolation
	// doctrine: the openloop package's helpers are pure on these
	// pointers and the root encoder coordinates lifetime.
	//
	// tOp records the most recently computed open-loop pitch
	// T_op ∈ [20,143] so Phase 2c (closed-loop refinement) can
	// recover the search centre without re-running §A.3.4.
	aQ12Latest    [lpc.LPCOrder + 1]int16
	lpResidualMem [10]int16
	swMem         [10]int16
	tOp           int16

	// Phase 2c INT-0: closed-loop pitch state.
	//
	// lspDec mirrors the decoder-side LSP reconstruction so the
	// encoder can derive the per-subframe quantized LP polynomials
	// Â (Q12) used by the closed-loop filters per I-2c-2 (plan
	// §0). It is advanced exactly once per frame inside lpcStep
	// after Quantize selects the LSP indices.
	//
	// aHatSF1 / aHatSF2 cache those per-subframe Â vectors so that
	// closedloopStep(0) and closedloopStep(1) read the right
	// polynomial without re-running the LSP→LP chain.
	//
	// swMemErr is the §A.3.10 weighted-error memory ew(n) for
	// the target filter 1/Â(z/γ); committed at the end of every
	// subframe per eq. A.10 after both adaptive and fixed gains are
	// quantized.
	//
	// lpResidualMemQ is the analysis-filter memory for residual
	// r(n) computed via the QUANTIZED Â. It is separate from the
	// frame-level open-loop lpResidualMem because closed-loop commits at
	// subframe boundaries while open-loop advances once per 80-sample frame.
	//
	// oldExc holds the trailing past excitation u(-1..-LookbackExc)
	// in chronological order, with oldExc[len-1] = u(-1). Updated
	// per subframe after gain quantization by shifting left by
	// SubframeLen and appending u(n) = ĝp·v(n) + ĝc·c(n).
	//
	// intT1 caches the subframe-1 integer winner so closedloopStep(1)
	// can centre its 10-lag P2 search window per §4.1.3 (lines
	// 1512–1523) via closedloop.Subframe2Window.
	//
	// frac1 / frac2 / intT2 + p1 / p0 / p2 store the per-frame
	// pitch-bit outputs; future bit-packing tasks (Phase 2g) will
	// drain them into the 80-bit frame layout.
	lspDec         lsp.Decoder
	aHatSF1        [lpc.LPCOrder + 1]int16
	aHatSF2        [lpc.LPCOrder + 1]int16
	swMemErr       [10]int16
	lpResidualMemQ [10]int16
	intT1          int16
	intT2          int16
	frac1          int8
	frac2          int8
	p1             uint8
	p0             uint8
	p2             uint8

	// Phase 2d INT-0: fixed-codebook + gain quantization state.
	//
	// prevGpQ14 caches the quantized adaptive-codebook gain ĝp from
	// the previous subframe so CB-4 (BuildCode) can derive the
	// harmonic-enhancement coefficient β per §3.8 eq. 47. At cold
	// start (first subframe of stream) this is 0.
	//
	// prevTaming is the §3.9.2 sticky taming flag (currently a
	// per-subframe diagnostic; the eq. 73/74 quantizer codebooks
	// are non-negative so taming is one-sided).
	//
	// s{1,2} / c{1,2} / ga{1,2} / gb{1,2} hold the per-frame FCB +
	// gain bits for Phase 2f bitstream packing (S = 4 bits; C =
	// 13 bits; GA = 3 bits; GB = 4 bits).
	prevGpQ14  int16
	prevTaming bool
	s1, s2     uint8
	c1, c2     uint16
	ga1, gb1   uint8
	ga2, gb2   uint8

	// Quality-profile decoder mirror used only to avoid pathological
	// full-scale output on very loud inputs. It does not affect the core
	// Annex A search path; it only reranks gain candidates when the
	// initially selected standard payload would clip after local decode.
	qualityGainDec    gain.Decoder
	qualitySynth      synth.Synthesizer
	qualityPostfilter postfilter.Postfilter
	qualityPastExc    [153]int16
	qualityPrevGpQ14  int16
	qualityHPX        [2]int16
	qualityHPY        [2]int32
	qualityLastOutput encoderQualityOutputScore

	// Diagnostic counters for focused FCB search complexity. They track how
	// many first-three-pulse prefixes entered the fourth loop; encoder output
	// is unaffected.
	qualityFCBThresholdEntriesFrame  int
	qualityFCBThresholdEntriesLast   int
	coreFCBThresholdEntriesRemaining int
	qualityFCBClipCooldown           int
	qualityOpenLoopClipCooldown      int

	// Phase 2f PACK-1: per-frame LSP VQ indices retained from
	// lpcStep so buildBitstreamFrame can compose the §4.2.1 + Table 8
	// 80-bit packed frame. Mirrors the p1/p0/p2 + s/c/ga/gb pattern
	// above. Widths per Table 8: L0=1, L1=7, L2=5, L3=5.
	l0, l1, l2, l3 uint16

	// Phase 2f API-2: streaming Write/Flush state.
	//
	// streamBuf is the PCM tail buffer holding 0..FrameSamples-1
	// samples not yet emitted as a frame. streamBufLen counts the
	// valid samples. streamSink is the destination io.Writer (nil
	// for non-streaming Encoder instances; (*Encoder).Write returns
	// ErrNoStreamSink in that case).
	//
	// OQ-FLUSH-PAD pin: Flush zero-pads any partial trailing frame
	// to FrameSamples with 0x0000 (linear-PCM silence) and emits
	// one final 10-byte frame. Plan §1 I-2f-3 + §10 OQ-FLUSH-PAD.
	//
	// streamPacked is the per-frame packed-byte scratch reused
	// across Write/Flush; promoting it to a receiver field keeps
	// the streamSink.Write(streamPacked[:]) interface call from
	// escaping the slice header onto the heap. INT-2 zero-alloc.
	streamBuf    [FrameSamples]int16
	streamPacked [FrameBytes]byte
	streamBufLen int
	streamSink   io.Writer

	// Per-block state owners.
	lpc    lpc.Analyzer
	acelp  acelp.Searcher
	weight filter.Weighting
}

// EncoderProfile selects the encoder-side search policy. All profiles emit
// standard 10-byte G.729 frames; none is an ITU byte-exact conformance claim.
type EncoderProfile uint8

const (
	// EncoderProfileCore disables repository-local quality heuristics that
	// widen or bias the encoder search. It is intended for diagnostics and
	// clean-room algorithm work, not as a quality recommendation.
	EncoderProfileCore EncoderProfile = iota

	// EncoderProfileQuality enables standard-compatible encoder search
	// heuristics tuned by black-box executable decode metrics. This is the
	// product default used by NewEncoder and NewStreamingEncoder.
	EncoderProfileQuality

	// EncoderProfileQualityAnnexALSP explicitly selects the quality profile
	// with the sequential Annex A LSP quantizer. EncoderProfileQuality uses a
	// wider encoder-side LSP search while still emitting standard LSP indices.
	EncoderProfileQualityAnnexALSP

	// EncoderProfileQualityClean is a listening-diagnostic quality profile that
	// disables normalized closed-loop pitch reranking and uses stricter
	// decoder-in-loop gain repair with a stronger high-residual preference. It
	// is intended to compare a smoother, bcg729-like candidate against the
	// default quality profile.
	EncoderProfileQualityClean

	// EncoderProfileQualityCleanSNR is a listening-diagnostic variant of
	// EncoderProfileQualityClean that keeps the older high-SNR gain-repair
	// preference. It is useful for A/B testing clarity against the smoother
	// high-residual-aware clean profile.
	EncoderProfileQualityCleanSNR

	// EncoderProfileQualityCleanSmooth is a listening-diagnostic variant that
	// applies the clean pitch policy with a lower decoder-in-loop repair
	// threshold and stronger high-residual preference. It deliberately trades
	// some waveform SNR for more headroom and a smoother bitstream candidate.
	EncoderProfileQualityCleanSmooth

	// EncoderProfileQualityCleanVoiced is a listening-diagnostic variant that
	// keeps the clean pitch policy and allows gain repair to prefer a slightly
	// stronger adaptive-codebook gain when MSE and high-residual scores remain
	// close. This tests whether reducing fixed-codebook roughness is more
	// audible than the small objective-score tradeoff.
	EncoderProfileQualityCleanVoiced

	// EncoderProfileQualityCleanDegrit is a listening-diagnostic variant that
	// keeps the clean pitch policy and allows gain repair to prefer lower
	// fixed-codebook gain correction when adaptive gain is not reduced and the
	// objective-score tradeoff remains bounded.
	EncoderProfileQualityCleanDegrit

	// EncoderProfileQualityCleanHarmonic is a listening-diagnostic variant
	// that keeps the clean pitch policy and, on voiced subframes, allows gain
	// repair to trade a bounded score loss for higher adaptive gain and lower
	// fixed-codebook correction.
	EncoderProfileQualityCleanHarmonic

	// EncoderProfileQualityCleanHarmonicStrong is a stronger listening
	// diagnostic variant of EncoderProfileQualityCleanHarmonic. It allows a
	// wider bounded tradeoff toward harmonic/adaptive energy to test whether
	// residual grit decreases before speech becomes too muffled.
	EncoderProfileQualityCleanHarmonicStrong

	// EncoderProfileQualityCleanHarmonicDeep pushes the harmonic/adaptive gain
	// tradeoff beyond EncoderProfileQualityCleanHarmonicStrong. It is intended
	// to locate the grit-vs-muffling boundary in listening tests.
	EncoderProfileQualityCleanHarmonicDeep

	// EncoderProfileQualityCleanFCBRerank is a listening-diagnostic variant
	// that keeps the clean pitch policy and reranks a small fixed-codebook
	// candidate set with the decoder-in-loop residual score before gain repair.
	EncoderProfileQualityCleanFCBRerank

	// EncoderProfileQualityPESQ is a listening-diagnostic profile that keeps a
	// smaller encoder-side search set but enables native reconstructed-gain
	// search, gain clip repair, and fixed-codebook residual reranking. It is
	// intended to A/B test the current PESQ-leading repository-local candidate.
	EncoderProfileQualityPESQ

	// EncoderProfileQualityPESQDegrit is a listening-diagnostic variant of the
	// PESQ candidate that also enables bounded gain MSE/noise repair. It usually
	// gives up PESQ score for lower high-residual energy, so it exists only for
	// blind tests around the user-reported gritty/smoky artifact.
	EncoderProfileQualityPESQDegrit
)

type encoderQualityTuning uint32

const (
	encoderTuningPitchCenterCandidate encoderQualityTuning = 1 << iota
	encoderTuningFCBThresholdScan
	encoderTuningGainSearchBias
	encoderTuningEarlyClosedLoopSpeechWindow
	encoderTuningNormalizedAdaptivePitchSearch
	encoderTuningResidualExtensionAdaptiveVector
	encoderTuningGainClipRepair
	encoderTuningGainMSERepair
	encoderTuningPitchClipRepair
	encoderTuningWideGainPredictor
	encoderTuningExpandedLSPSearch
	encoderTuningNativeGainSearch
	encoderTuningClippedInputOpenLoopRange2
	encoderTuningNormalizedOpenLoopSearch
	encoderTuningGainNoiseRepair
	encoderTuningFCBNoiseRerank

	encoderQualityTuningAll = encoderTuningNormalizedAdaptivePitchSearch |
		encoderTuningGainClipRepair |
		encoderTuningGainMSERepair |
		encoderTuningNativeGainSearch |
		encoderTuningExpandedLSPSearch |
		encoderTuningGainNoiseRepair
)

// NewEncoder returns an Encoder in initial state using EncoderProfileQuality.
func NewEncoder() *Encoder {
	return NewEncoderWithProfile(EncoderProfileQuality)
}

// NewEncoderWithProfile returns an Encoder in initial state using the selected
// encoder search profile.
func NewEncoderWithProfile(profile EncoderProfile) *Encoder {
	profile = normalizeEncoderProfile(profile)
	e := &Encoder{
		profile:       profile,
		qualityTuning: encoderQualityTuningForProfile(profile),
	}
	e.initState()
	return e
}

func normalizeEncoderProfile(profile EncoderProfile) EncoderProfile {
	switch profile {
	case EncoderProfileCore, EncoderProfileQuality, EncoderProfileQualityAnnexALSP, EncoderProfileQualityClean, EncoderProfileQualityCleanSNR, EncoderProfileQualityCleanSmooth, EncoderProfileQualityCleanVoiced, EncoderProfileQualityCleanDegrit, EncoderProfileQualityCleanHarmonic, EncoderProfileQualityCleanHarmonicStrong, EncoderProfileQualityCleanHarmonicDeep, EncoderProfileQualityCleanFCBRerank, EncoderProfileQualityPESQ, EncoderProfileQualityPESQDegrit:
		return profile
	default:
		return EncoderProfileQuality
	}
}

func encoderQualityTuningForProfile(profile EncoderProfile) encoderQualityTuning {
	switch profile {
	case EncoderProfileQuality:
		return encoderQualityTuningAll
	case EncoderProfileQualityAnnexALSP:
		return encoderQualityTuningAll &^ encoderTuningExpandedLSPSearch
	case EncoderProfileQualityCleanFCBRerank:
		return (encoderQualityTuningAll &^ encoderTuningNormalizedAdaptivePitchSearch) | encoderTuningFCBNoiseRerank
	case EncoderProfileQualityPESQ:
		return encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningFCBNoiseRerank
	case EncoderProfileQualityPESQDegrit:
		return encoderTuningNativeGainSearch | encoderTuningGainClipRepair | encoderTuningGainMSERepair | encoderTuningGainNoiseRepair | encoderTuningFCBNoiseRerank
	case EncoderProfileQualityClean, EncoderProfileQualityCleanSNR, EncoderProfileQualityCleanSmooth, EncoderProfileQualityCleanVoiced, EncoderProfileQualityCleanDegrit, EncoderProfileQualityCleanHarmonic, EncoderProfileQualityCleanHarmonicStrong, EncoderProfileQualityCleanHarmonicDeep:
		return encoderQualityTuningAll &^ encoderTuningNormalizedAdaptivePitchSearch
	default:
		return 0
	}
}

func (e *Encoder) initState() {
	lsp.InitFreqPrev(&e.freqPrev)
	lsp.InitLSPOld(&e.lspOld)
	for i := range e.pastQuaEn {
		e.pastQuaEn[i] = gain.PastErrorsDefault
	}
}

func (e *Encoder) qualityHeuristicsEnabled() bool {
	return e.qualityTuning != 0
}

func (e *Encoder) qualityPitchCenterCandidateEnabled() bool {
	return e.qualityTuning&encoderTuningPitchCenterCandidate != 0
}

func (e *Encoder) qualityFCBThresholdScanEnabled() bool {
	return e.qualityTuning&encoderTuningFCBThresholdScan != 0
}

func (e *Encoder) qualityFCBThresholdScanActive() bool {
	return e.qualityFCBThresholdScanEnabled() && e.qualityFCBClipCooldown == 0
}

func (e *Encoder) coreFCBThresholdScanEnabled() bool {
	return e.profile == EncoderProfileCore && e.qualityTuning == 0
}

func (e *Encoder) coreGainPreselectPrecisionEnabled() bool {
	return e.profile == EncoderProfileCore && e.qualityTuning == 0
}

func (e *Encoder) coreFCBThresholdScanLimit(sub int) int {
	limit := e.coreFCBThresholdEntriesRemaining
	if sub == 0 && limit > encoderCoreFCBThresholdScanSubframe0Limit {
		return encoderCoreFCBThresholdScanSubframe0Limit
	}
	return limit
}

func (e *Encoder) recordCoreFCBThresholdEntries(entered int) {
	e.coreFCBThresholdEntriesRemaining -= entered
	if e.coreFCBThresholdEntriesRemaining < 0 {
		e.coreFCBThresholdEntriesRemaining = 0
	}
}

func (e *Encoder) qualityGainSearchBiasEnabled() bool {
	return e.qualityTuning&encoderTuningGainSearchBias != 0
}

func (e *Encoder) qualityEarlyClosedLoopSpeechWindowEnabled() bool {
	return e.qualityTuning&encoderTuningEarlyClosedLoopSpeechWindow != 0
}

func (e *Encoder) qualityNormalizedAdaptivePitchSearchEnabled() bool {
	return e.qualityTuning&encoderTuningNormalizedAdaptivePitchSearch != 0
}

func (e *Encoder) qualityResidualExtensionAdaptiveVectorEnabled() bool {
	return e.qualityTuning&encoderTuningResidualExtensionAdaptiveVector != 0
}

func (e *Encoder) qualityGainClipRepairEnabled() bool {
	return e.qualityTuning&encoderTuningGainClipRepair != 0
}

func (e *Encoder) qualityGainMSERepairEnabled() bool {
	return e.qualityTuning&encoderTuningGainMSERepair != 0
}

func (e *Encoder) qualityGainMSERepairThreshold() int {
	switch e.profile {
	case EncoderProfileQualityClean, EncoderProfileQualityCleanSNR, EncoderProfileQualityCleanVoiced, EncoderProfileQualityCleanDegrit, EncoderProfileQualityCleanHarmonic, EncoderProfileQualityCleanHarmonicStrong, EncoderProfileQualityCleanHarmonicDeep, EncoderProfileQualityCleanFCBRerank:
		return qualityCleanGainMSERepairThreshold
	case EncoderProfileQualityCleanSmooth:
		return qualityCleanSmoothGainMSERepairThreshold
	}
	return qualityGainMSERepairThreshold
}

func (e *Encoder) qualityGainNoiseRepairHighMSEBetterTolerance() (num, den int64) {
	switch e.profile {
	case EncoderProfileQualityClean:
		return qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceNum, qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceDen
	case EncoderProfileQualityCleanSmooth:
		return qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceNum, qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceDen
	}
	return qualityGainNoiseRepairHighMSEBetterMSEToleranceNum, qualityGainNoiseRepairHighMSEBetterMSEToleranceDen
}

func (e *Encoder) qualityGainNoiseRepairMSEBetterTolerance() (num, den int64) {
	return qualityGainNoiseRepairMSEBetterHighMSEToleranceNum, qualityGainNoiseRepairMSEBetterHighMSEToleranceDen
}

func (e *Encoder) qualityGainPitchPreferenceEnabled() bool {
	return e.profile == EncoderProfileQualityCleanVoiced
}

func (e *Encoder) qualityGainDegritPreferenceEnabled() bool {
	return e.profile == EncoderProfileQualityCleanDegrit
}

func (e *Encoder) qualityGainHarmonicPreferenceEnabled() bool {
	return e.profile == EncoderProfileQualityCleanHarmonic ||
		e.profile == EncoderProfileQualityCleanHarmonicStrong ||
		e.profile == EncoderProfileQualityCleanHarmonicDeep
}

func (e *Encoder) qualityGainHarmonicPreferenceParams() (minGp, minStep, gammaDrop int32, mseNum, mseDen, highNum, highDen int64) {
	if e.profile == EncoderProfileQualityCleanHarmonicDeep {
		return qualityCleanHarmonicDeepGainPitchMinQ14,
			qualityCleanHarmonicDeepGainPitchMinStepQ14,
			qualityCleanHarmonicDeepGammaDropMinQ13,
			qualityCleanHarmonicDeepMSEToleranceNum,
			qualityCleanHarmonicDeepMSEToleranceDen,
			qualityCleanHarmonicDeepHighMSEToleranceNum,
			qualityCleanHarmonicDeepHighMSEToleranceDen
	}
	if e.profile == EncoderProfileQualityCleanHarmonicStrong {
		return qualityCleanHarmonicStrongGainPitchMinQ14,
			qualityCleanHarmonicStrongGainPitchMinStepQ14,
			qualityCleanHarmonicStrongGammaDropMinQ13,
			qualityCleanHarmonicStrongMSEToleranceNum,
			qualityCleanHarmonicStrongMSEToleranceDen,
			qualityCleanHarmonicStrongHighMSEToleranceNum,
			qualityCleanHarmonicStrongHighMSEToleranceDen
	}
	return qualityCleanHarmonicGainPitchMinQ14,
		qualityCleanHarmonicGainPitchMinStepQ14,
		qualityCleanHarmonicGammaDropMinQ13,
		qualityCleanHarmonicMSEToleranceNum,
		qualityCleanHarmonicMSEToleranceDen,
		qualityCleanHarmonicHighMSEToleranceNum,
		qualityCleanHarmonicHighMSEToleranceDen
}

func (e *Encoder) qualityFCBNoiseRerankEnabled() bool {
	return e.qualityTuning&encoderTuningFCBNoiseRerank != 0
}

func (e *Encoder) qualityGainNoiseRepairEnabled() bool {
	return e.qualityTuning&encoderTuningGainNoiseRepair != 0
}

func (e *Encoder) qualityPitchClipRepairEnabled() bool {
	return e.qualityTuning&encoderTuningPitchClipRepair != 0
}

func (e *Encoder) qualityWideGainPredictorEnabled() bool {
	return e.qualityTuning&encoderTuningWideGainPredictor != 0
}

func (e *Encoder) qualityExpandedLSPSearchEnabled() bool {
	return e.qualityTuning&encoderTuningExpandedLSPSearch != 0
}

func (e *Encoder) qualityNativeGainSearchEnabled() bool {
	return e.qualityTuning&encoderTuningNativeGainSearch != 0
}

func (e *Encoder) qualityClippedInputOpenLoopRange2Enabled() bool {
	return e.qualityTuning&encoderTuningClippedInputOpenLoopRange2 != 0
}

func (e *Encoder) qualityNormalizedOpenLoopSearchEnabled() bool {
	return e.qualityTuning&encoderTuningNormalizedOpenLoopSearch != 0
}

const (
	qualityGainSearchTargetScaleNum               int32 = 2
	qualityGainSearchTargetScaleDen               int32 = 5
	qualityGainSearchAdaptiveContributionScaleNum int32 = 3
	qualityGainSearchAdaptiveContributionScaleDen int32 = 1
	qualityGainSearchFixedContributionScaleNum    int32 = 4
	qualityGainSearchFixedContributionScaleDen    int32 = 3

	// Core keeps the Annex A preselect structure but preserves the maximum
	// zero-allocation-safe correlation precision for the unquantized
	// gp_opt/gc_opt center solve. This is not used by the Quality profile's
	// native gain-search heuristic.
	encoderCoreGainPreselectTargetBits uint = 24
)

// qualityNormalizedPitchSearchHalfWindow is a quality-profile search
// heuristic. Core/profile-spec pitch search keeps the Annex A ±3 window.
var qualityNormalizedPitchSearchHalfWindow = 60

// qualityFCBThresholdScanLimit is the quality-profile focused fixed-codebook
// search budget used when encoderTuningFCBThresholdScan is enabled. Annex A
// describes K3=0.4 thresholding with a maximum of 180 fourth-loop entries over
// two subframes; this product heuristic uses a more conservative per-subframe
// budget that reduced clipped output on black-box decode metrics.
var qualityFCBThresholdScanLimit = 60

// Near-clipped input can make the focused fixed-codebook threshold search pick
// pulse sets that decode hotter than the exhaustive C²/E winner. Temporarily
// falling back to exhaustive search keeps the payload standard-compatible while
// avoiding a sample-specific full-scale edge.
var qualityFCBClipThreshold = 32000
var qualityFCBClipCooldownFrames = 20

// encoderCoreFCBThresholdScanSubframe0Limit keeps the first subframe from
// consuming the whole §3.8.1 focused-search budget. The PDF fixes the maximum
// over two subframes at 180 and notes an average worst case of 90 per subframe;
// this conservative first-subframe guard preserves that frame cap while leaving
// the second subframe the remaining budget.
var encoderCoreFCBThresholdScanSubframe0Limit = fcbsearch.SearchThresholdScanDefaultLimit / 2

// qualityPitchCenterFullSearchMinScoreRatio is a diagnostic rescue
// threshold for replacing a high open-loop centre with a lower full-range
// pitch candidate. The default quality profile no longer enables this
// heuristic; diagnostics may opt in and sweep values without changing
// core/profile-spec search.
var qualityPitchCenterFullSearchMinScoreRatio = 1.75

// qualityNormalizedPitchHarmonicGuardRatio is a diagnostic-only threshold
// for preferring the centre-window pitch candidate over a doubled-period
// normalized-search winner. A value <= 0 disables the guard.
var qualityNormalizedPitchHarmonicGuardRatio = 0.0

// qualityNormalizedPitchShortPositiveFracRatio is a diagnostic-only threshold
// for preferring a +1/3 short-pitch fractional candidate in the quality
// normalized search when its score is close to the selected fraction. A value
// <= 0 disables the guard.
var qualityNormalizedPitchShortPositiveFracRatio = 0.0

// qualityOpenLoopClipThreshold and qualityOpenLoopClipCooldownFrames gate a
// quality-profile pitch-centre heuristic for hard-clipped input. Around clipped
// frames, the encoder may prefer the [40,79] open-loop range when its
// normalized score is close to the merged §A.3.4 winner. Core remains unchanged.
var qualityOpenLoopClipThreshold = 32700
var qualityOpenLoopClipCooldownFrames = 10

const (
	qualityOpenLoopRange2CloseNum int64 = 95
	qualityOpenLoopRange2CloseDen int64 = 100
)

// qualityGainClipRepairThreshold controls the quality-profile decoder-in-loop
// gain repair trigger. Keep this below hard clip so the repair has enough
// decoder-side headroom before FFmpeg/local output reaches saturation.
var qualityGainClipRepairThreshold = 32300

// qualityGainMSERepairThreshold is stricter than the hard clip-repair
// trigger because this path can change otherwise non-clipping gain fields.
// Keep it below the near-clip threshold while leaving enough level headroom
// that the high-residual-aware candidate can improve voiced-frame roughness
// without trading it for full-scale FFmpeg/local decoder peaks.
var qualityGainMSERepairThreshold = 28000

const qualityCleanGainMSERepairThreshold = 26000
const qualityCleanSmoothGainMSERepairThreshold = 24000

const (
	qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceNum       int64 = 110
	qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceDen       int64 = 100
	qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceNum int64 = 120
	qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceDen int64 = 100
	qualityCleanVoicedGainPitchMSEToleranceNum                    int64 = 108
	qualityCleanVoicedGainPitchMSEToleranceDen                    int64 = 100
	qualityCleanVoicedGainPitchHighMSEToleranceNum                int64 = 108
	qualityCleanVoicedGainPitchHighMSEToleranceDen                int64 = 100
	qualityCleanVoicedGainPitchMinStepQ14                         int16 = 256
	qualityCleanDegritGammaDropMinQ13                             int32 = 256
	qualityCleanDegritMSEToleranceNum                             int64 = 110
	qualityCleanDegritMSEToleranceDen                             int64 = 100
	qualityCleanDegritHighMSEToleranceNum                         int64 = 108
	qualityCleanDegritHighMSEToleranceDen                         int64 = 100
	qualityCleanHarmonicGainPitchMinQ14                           int32 = 8192
	qualityCleanHarmonicGainPitchMinStepQ14                       int32 = 256
	qualityCleanHarmonicGammaDropMinQ13                           int32 = 256
	qualityCleanHarmonicMSEToleranceNum                           int64 = 112
	qualityCleanHarmonicMSEToleranceDen                           int64 = 100
	qualityCleanHarmonicHighMSEToleranceNum                       int64 = 110
	qualityCleanHarmonicHighMSEToleranceDen                       int64 = 100
	qualityCleanHarmonicStrongGainPitchMinQ14                     int32 = 8192
	qualityCleanHarmonicStrongGainPitchMinStepQ14                 int32 = 128
	qualityCleanHarmonicStrongGammaDropMinQ13                     int32 = 512
	qualityCleanHarmonicStrongMSEToleranceNum                     int64 = 118
	qualityCleanHarmonicStrongMSEToleranceDen                     int64 = 100
	qualityCleanHarmonicStrongHighMSEToleranceNum                 int64 = 115
	qualityCleanHarmonicStrongHighMSEToleranceDen                 int64 = 100
	qualityCleanHarmonicDeepGainPitchMinQ14                       int32 = 8192
	qualityCleanHarmonicDeepGainPitchMinStepQ14                   int32 = 0
	qualityCleanHarmonicDeepGammaDropMinQ13                       int32 = 768
	qualityCleanHarmonicDeepMSEToleranceNum                       int64 = 125
	qualityCleanHarmonicDeepMSEToleranceDen                       int64 = 100
	qualityCleanHarmonicDeepHighMSEToleranceNum                   int64 = 122
	qualityCleanHarmonicDeepHighMSEToleranceDen                   int64 = 100
	qualityCleanFCBRerankTopK                                     int   = 8
	qualityCleanFCBRerankHighMSEBetterMSEToleranceNum             int64 = 112
	qualityCleanFCBRerankHighMSEBetterMSEToleranceDen             int64 = 100
	qualityCleanFCBRerankMSEBetterHighMSEToleranceNum             int64 = 105
	qualityCleanFCBRerankMSEBetterHighMSEToleranceDen             int64 = 100
)

var (
	qualityGainNoiseRepairHighMSEBetterMSEToleranceNum int64 = 105
	qualityGainNoiseRepairHighMSEBetterMSEToleranceDen int64 = 100
	qualityGainNoiseRepairMSEBetterHighMSEToleranceNum int64 = 130
	qualityGainNoiseRepairMSEBetterHighMSEToleranceDen int64 = 100
)

// qualityPitchClipRepairPeakThreshold is a quality-profile decoder-in-loop
// pitch guard. Some black-box decoders clip before the local mirror reaches the
// gain-repair threshold, so high local peaks may rerank nearby pitch candidates
// if subframe MSE remains close to the original candidate.
var qualityPitchClipRepairPeakThreshold = 29500

const (
	closedLoopPitchSearchHistory = closedloop.PitchMaxInt + pitchcore.Linter
	closedLoopPitchSearchLen     = closedLoopPitchSearchHistory + closedloop.SubframeLen
)

// Reset returns the Encoder codec state to initial state and clears any
// buffered streaming tail. It reuses the existing memory and preserves the
// streaming sink configured by NewStreamingEncoder, if any.
func (e *Encoder) Reset() {
	sink := e.streamSink
	profile := e.profile
	qualityTuning := e.qualityTuning
	*e = Encoder{
		profile:       normalizeEncoderProfile(profile),
		qualityTuning: qualityTuning,
	}
	e.streamSink = sink
	e.initState()
}

// LSPReuseCount returns the running tally of frames where the
// FIX-3-B anti-palindromic LP guard fired (LPToLSP returned
// ErrLPCNonStable → previous-frame LSP reuse). Diagnostic-only;
// not part of the encoded bitstream. See §3.2.6 / §3.2.3 cite in
// lpcStep.
func (e *Encoder) LSPReuseCount() uint64 { return e.lspReuseCount }

// EncodeFrame consumes exactly FrameSamples samples and writes exactly
// FrameBytes bytes to out. Internal state is retained across calls.
//
// Phase 2f API-1 wiring (plan §6 Task API-1):
//
//  1. lpcStep(pcm)            — §3.2 LP analysis + LSP VQ → l0..l3,
//     aQ12Latest, aHatSF1/2, oldSpeech slide.
//  2. openloopStep()          — §A.3.3/A.3.4 weighted speech + open-loop
//     pitch → tOp.
//  3. closedloopStep(0)       — §A.3.5–A.3.10 subframe 1 → p1, p0; calls
//     fcbStep(0) internally → s1, c1, ga1, gb1.
//  4. closedloopStep(1)       — subframe 2 → p2; calls fcbStep(1)
//     internally → s2, c2, ga2, gb2.
//  5. buildBitstreamFrame     — composes the 15 per-frame indices into
//     a stack-allocated bitstream.Frame.
//  6. bitstream.Pack          — emits the canonical 10-byte G.729
//     frame per §4.2.1 + Table 8.
//
// I3 / I4: per-frame state is mutated exactly once per call by the
// step methods; no allocation on the hot path (the bitstream.Frame
// value is stack-resident).
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
	if len(pcm) != FrameSamples {
		return ErrShortPCM
	}
	if len(out) < FrameBytes {
		return ErrShortOutput
	}

	if _, err := e.lpcStep(pcm); err != nil {
		return err
	}
	_ = e.openloopStep()
	_, _ = e.closedloopStep(0)
	_, _ = e.closedloopStep(1)

	var bsFrame bitstream.Frame
	e.buildBitstreamFrame(&bsFrame)
	return bitstream.Pack(&bsFrame, out)
}

// lpcStep runs the §3.2.1–§3.2.4 chain on one 80-sample PCM frame
// and returns the LSP quantizer indices (L0, L1, L2, L3) for that
// frame. Internal state advanced per call:
//
//	e.oldSpeech : 240-sample sliding analysis buffer (shifted left by
//	              80 each call; new pre-processed samples appended at
//	              [160..239]).
//	e.freqPrev  : MA-predictor FIFO; commitPredictorMemory advances
//	              it once Quantize selects the L0 winner.
//
// Buffer-shift convention (per plan §3.2.1 cite line 671). The
// 30 ms LP analysis window covers 240 = 200 past + 40 future
// samples. With the slide-by-80 layout used here the window applied
// at iteration n centres on oldSpeech[200], so the analysis output
// at iteration n corresponds to the speech segment ending at the
// first 40 samples of frame n — i.e. there is a 1-frame
// analysis-vs-encode delay between PCM-in and indices-out. Phase 2a
// gate vector LSP.BIT was generated by ITU's encoder under this
// convention; Phase 2b will own the same shift ordering for the
// pitch / FCB stages.
//
// I3 / I4: pure (apart from advancing e.oldSpeech / e.freqPrev),
// zero allocation; all per-frame scratch lives in stack arrays.
func (e *Encoder) lpcStep(pcm []int16) (lsp.Indices, error) {
	if len(pcm) != FrameSamples {
		return lsp.Indices{}, ErrShortPCM
	}
	e.qualityFCBThresholdEntriesFrame = 0
	e.qualityFCBThresholdEntriesLast = 0
	if e.coreFCBThresholdScanEnabled() {
		e.coreFCBThresholdEntriesRemaining = fcbsearch.SearchThresholdScanDefaultLimit
	} else {
		e.coreFCBThresholdEntriesRemaining = 0
	}
	if e.qualityFCBThresholdScanEnabled() && samplesNearClip(pcm, qualityFCBClipThreshold) {
		e.qualityFCBClipCooldown = qualityFCBClipCooldownFrames
	}
	if e.qualityClippedInputOpenLoopRange2Enabled() && samplesNearClip(pcm, qualityOpenLoopClipThreshold) {
		e.qualityOpenLoopClipCooldown = qualityOpenLoopClipCooldownFrames
	}

	var processed [FrameSamples]int16
	e.pre.Process(pcm, processed[:])

	copy(e.oldSpeech[0:160], e.oldSpeech[80:240])
	copy(e.oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := e.lpc.Analyze(&e.oldSpeech, &aQ12); err != nil {
		return lsp.Indices{}, err
	}
	e.aQ12Latest = aQ12

	var qQ15 [10]int16
	if err := lsp.LPToLSP(&aQ12, &qQ15); err != nil {
		// FIX-3-B (Phase 2a INT-1 d10): anti-palindromic LP guard.
		//
		// Spec citation: G.729 (06/2012) §3.2.3 establishes the F1/F2
		// sum/difference polynomial construction and the 60-point
		// sign-change scan, but is silent on the degenerate case
		// where a[k] = ±a[10−k] makes one polynomial identically
		// zero (no sign changes ⇒ root extraction fails). §3.2.6
		// (LSP→LP for the synthesis filter) provides the
		// stability-and-reuse precedent: when a freshly reconstructed
		// LSP vector violates the ordering / spacing constraints,
		// the previous frame's quantized LSPs are reused. Applying
		// the same precedent here keeps the encoder graceful on
		// rare anti-palindromic transients (e.g. LSP.IN frame 596)
		// instead of fail-fast at the first non-stable frame.
		//
		// Cold-start safety: NewEncoder / Reset seed e.lspOld via
		// InitLSPOld (cos(i·π/11) Q15 per §3.2.6 / §4.1.5), so the
		// fallback is well-defined even if the very first frame
		// triggers the guard.
		if errors.Is(err, lsp.ErrLPCNonStable) {
			qQ15 = e.lspOld
			e.lspReuseCount++
		} else {
			return lsp.Indices{}, err
		}
	} else {
		// Successful extraction: cache for future-frame reuse.
		e.lspOld = qQ15
	}

	var omega [10]int16
	lsp.LSPToLSF(&qQ15, &omega)

	var indices lsp.Indices
	if e.qualityExpandedLSPSearchEnabled() {
		// Quality heuristic: keep the first-stage L1 search identical to
		// the Annex A sequential search, then jointly evaluate the L2/L3
		// pair for that L1. This remains a standard 18-bit LSP tuple on
		// the wire, but the broader second-stage search is intentionally
		// not part of EncoderProfileCore.
		indices = lsp.QuantizeTopK(&omega, &e.freqPrev, 1)
	} else {
		indices = lsp.Quantize(&omega, &e.freqPrev)
	}

	// Phase 2f PACK-1: retain the LSP VQ indices for §4.2.1 + Table 8
	// frame packing in buildBitstreamFrame.
	e.l0 = uint16(indices.L0)
	e.l1 = uint16(indices.L1)
	e.l2 = uint16(indices.L2)
	e.l3 = uint16(indices.L3)

	// Phase 2c INT-0: derive per-subframe quantized LP polynomials Â
	// from the just-emitted indices. The decoder-side state in
	// e.lspDec runs in lock-step with the encoder VQ + MA-predictor
	// so the reconstructed Â matches what the receiver will see.
	// Cached on the encoder for the per-subframe closed-loop driver.
	e.aHatSF1, e.aHatSF2 = e.lspDec.Decode(indices)

	return indices, nil
}

// openloopStep runs the §A.3.3 weighted-speech construction and the
// §A.3.4 open-loop pitch search on the current 80-sample analysis
// frame. It must be called after lpcStep on the same frame: lpcStep
// reconstructs e.aHatSF{1,2} and writes the newest 80 pre-processed PCM
// samples into e.oldSpeech[160:240]. openloopStep reads the encoded
// speech frame e.oldSpeech[120:200]; e.oldSpeech[200:240] is the 5 ms
// lookahead.
//
// State advanced per call:
//
//	e.lpResidualMem : 10-sample s-history for eq. A.3
//	e.swMem         : 10-sample sw-history for eq. A.2
//	e.oldWspeech    : 143-sample sw history (slid in-place per
//	                  slideOldWspeech / I-2b-2)
//	e.tOp           : the per-frame open-loop pitch ∈ [20,143]
//
// I3 / I4: pure (apart from advancing the four state buffers above);
// zero allocation. The 80-sample current-frame view is taken via a
// pointer-to-array reslice over e.oldSpeech, avoiding a stack copy.
func (e *Encoder) openloopStep() int16 {
	s := (*[FrameSamples]int16)(e.oldSpeech[120:200])
	var result openloop.SearchResult
	if e.qualityNormalizedOpenLoopSearchEnabled() {
		result = openloop.StepSplitSearchNormalizedRanges(&e.aHatSF1, &e.aHatSF2, s, &e.lpResidualMem, &e.swMem, &e.oldWspeech)
	} else {
		result = openloop.StepSplitSearch(&e.aHatSF1, &e.aHatSF2, s, &e.lpResidualMem, &e.swMem, &e.oldWspeech)
	}
	e.tOp = result.Top
	if e.qualityClippedInputOpenLoopRange2Enabled() && e.qualityOpenLoopClipCooldown > 0 {
		if result.Range2AtLeastRatioOf(e.tOp, qualityOpenLoopRange2CloseNum, qualityOpenLoopRange2CloseDen) {
			e.tOp = result.Range2.Lag
		}
		e.qualityOpenLoopClipCooldown--
	}
	return e.tOp
}

func samplesNearClip(samples []int16, threshold int) bool {
	if threshold <= 0 {
		return false
	}
	for _, sample := range samples {
		v := int(sample)
		if v < 0 {
			v = -v
		}
		if v >= threshold {
			return true
		}
	}
	return false
}

// lpResidualSubframe computes the 40-sample LP residual r(n) for one
// subframe per ITU-T G.729 §A.3.3 eq. A.3 (G729E.txt line 2080):
//
// r(n) = s(n) + Σ_{i=1..10} â_i · s(n − i),  n = 0,...,39
//
// using the supplied 10-sample input history mem (s(-10..-1)). aHat
// is the QUANTIZED Â (Q12, leading 4096), per Phase 2c invariant
// I-2c-2. Mirrors openloop/lowpass.go lpResidual but specialised to
// SubframeLen = 40 and quantized-Â discipline.
//
// I3 / I4: pure (writes only through r), zero allocation. The caller
// is responsible for advancing mem at subframe boundaries.
func lpResidualSubframe(s *[40]int16, aHat *[lpc.LPCOrder + 1]int16, mem *[10]int16, r *[40]int16) {
	for n := 0; n < 40; n++ {
		acc := fixed.LMult(s[n], aHat[0])
		for i := 1; i <= 10; i++ {
			var sni int16
			if n-i >= 0 {
				sni = s[n-i]
			} else {
				sni = mem[10+n-i]
			}
			acc = fixed.LMac(acc, aHat[i], sni)
		}
		r[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

// closedloopStep runs ITU-T G.729 Annex A §A.3.5–§A.3.10 for one
// subframe and returns the selected (intLag, frac). Subframe 1 may return the
// P1 boundary codepoints (19,+1) or (85,-1); subframe 2 may return the P2
// boundary codepoints (tmin-1,+1) or (tmax+1,-1).
//
// Pre-condition: lpcStep + openloopStep have been called for the
// current frame, so e.aHatSF{1,2} carry the quantized Â and e.tOp
// carries the open-loop centre. Must be called twice per
// frame in order: closedloopStep(0) then closedloopStep(1). The
// subframe-1 winner is committed to e.intT1 so subframe-2's 10-lag
// P2 search window (closedloop.Subframe2Window) can centre on it
// per §4.1.3 (G729E.txt lines 1512–1523).
//
// Per-subframe pipeline:
//
//  1. r(n) = analysis filter Â(z) on s (§A.3.3 eq. A.3).
//  2. x(n) = target signal via 1/Â(z/γ) on r (§A.3.6 eq. cited;
//     closedloop.TargetSignal).
//  3. h(n) = impulse response of 1/Â(z/γ) (§A.3.5;
//     closedloop.ImpulseResponse).
//  4. xb(n) = backward filter of x against h (§A.3.7 eq. A.7;
//     closedloop.BackwardFilter).
//  5. intLag = closedloop.SearchInteger(xb, exc, centre, sub)
//     where centre = tOp for sub=0 and intT1 for sub=1.
//  6. refinedFrac = closedloop.RefineFractionSubframe{1,2}(...) per
//     §A.3.7 lines 2169–2170. T1 uses fractions in the lower region plus
//     P1 boundary codepoints; T2 is always fraction-capable by the P2 codebook.
//     The selected fraction is both transmitted and used by the
//     analysis-by-synthesis path so encoder state stays aligned with the
//     receiver's reconstructed adaptive-codebook vector.
//  7. v(n) = closedloop.AdaptiveVector(exc, intLag, transmittedFrac)
//     (§3.7.1 eq. 40).
//  8. (gp, y) = closedloop.GpAndY(x, v, h) (§3.7.3 eq. 43, 44).
//  9. P1/P0 (sub=0) or P2 (sub=1) packed via closedloop.EncodeP*
//     per §3.7.2.
//  10. Per-subframe state commit:
//     - fcbStep searches the fixed codebook and quantizes gains.
//     - swMemErr trail: ew(n) = x(n) − ĝp·y(n) − ĝc·z(n) for
//     n = 30..39 per §A.3.10 eq. A.10.
//     - oldExc shift-by-40 + append the full excitation
//     u(n) = ĝp·v(n) + ĝc·c(n) for n = 0..39 per eq. A.9.
//     - pastQuaEn, prevGpQ14, and prevTaming advance from the selected
//     gain candidate.
//     - lpResidualMemQ ← s(30..39) for the next subframe's r(n).
//
// Buffer convention (closedloop.SearchInteger / RefineFractionSubframe{1,2} /
// AdaptiveVector): exc is sized PitchMaxInt + pitch.Linter +
// SubframeLen = 193 samples; u(0) is anchored at
// exc[len − SubframeLen]. The past excitation u(−153..−1) occupies
// the leading 153 samples so the b30 FIR has real history for high-delay
// fractional candidates, including the P2 upper edge T_int=144, frac=-1;
// the trailing SubframeLen samples
// carry the LP-residual extension r(0..39) per §A.3.7 line 2161,
// supporting the short-pitch case k < SubframeLen.
//
// I3: swMemErr / lpResidualMemQ / oldExc / pastQuaEn are updated PER
// SUBFRAME (not per frame) because subframe-2's adaptive-codebook search and
// gain prediction must observe subframe-1's freshly committed state. The
// frame-level state (oldSpeech, freqPrev, lspDec internals) remains committed
// only at frame boundaries via lpcStep.
//
// I4: zero allocation. All scratch (r, x, h, xb, v, y, ew) lives on
// the stack as fixed-size arrays.
func (e *Encoder) closedloopStep(sub int) (intLag int16, frac int8) {
	var aHat *[lpc.LPCOrder + 1]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}

	// Core follows the encoded speech frame documented by openloopStep
	// (oldSpeech[120:200]). The quality profile keeps the earlier
	// black-box-localized window that improves waveform alignment while
	// retaining the existing LP/open-loop analysis path.
	sStart := 120 + 40*sub
	if e.qualityEarlyClosedLoopSpeechWindowEnabled() {
		sStart = 80 + 40*sub
	}
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [closedloop.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	closedloop.TargetSignal(aHat, &r, &e.swMemErr, &x)
	closedloop.ImpulseResponse(aHat, &h)
	closedloop.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}

	var excSearch [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&r, &excSearch)
	if e.qualityPitchCenterCandidateEnabled() {
		centre = e.qualityPitchCenterCandidate(&x, &h, excSlice, centre, sub)
	}
	if e.qualityNormalizedAdaptivePitchSearchEnabled() {
		intLag = e.searchPitchNormalizedAdaptive(&x, &h, excSlice, centre, sub)
		frac = e.refinePitchNormalizedAdaptive(&x, &h, excSlice, intLag, sub == 1 || intLag < 85, sub)
	} else {
		intLag, _ = closedloop.SearchInteger(&xb, excSlice, centre, sub)
		if sub == 1 {
			intLag, frac = closedloop.RefineFractionSubframe2(&xb, excSlice, intLag, e.intT1)
		} else {
			intLag, frac = closedloop.RefineFractionSubframe1(&xb, excSlice, intLag)
		}
	}

	if e.qualityPitchClipRepairEnabled() && e.qualityGainClipRepairEnabled() {
		return e.qualityRepairPitchClip(sub, aHat, sFrame, &x, &h, excSlice, intLag, frac)
	}

	e.commitClosedLoopPitch(sub, aHat, sFrame, &x, &h, excSlice, intLag, frac)
	return intLag, frac
}

func (e *Encoder) commitClosedLoopPitch(
	sub int,
	aHat *[lpc.LPCOrder + 1]int16,
	sFrame *[closedloop.SubframeLen]int16,
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	intLag int16,
	frac int8,
) {
	var v, y [closedloop.SubframeLen]int16

	// Keep bitstream packing, local synthesis, gain search, FCB search,
	// and excitation-state commit on the same selected fractional delay.
	// Diverging here lets the encoder search against a state the receiver
	// will never reconstruct.
	e.adaptiveVectorForSynthesis(exc, intLag, frac, &v)
	gp := closedloop.GpAndY(x, &v, h, &y)

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = closedloop.EncodeP1(intLag, frac)
		e.p0 = closedloop.EncodeP0(e.p1)
	} else {
		tmin, _ := closedloop.Subframe2Window(e.intT1)
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = closedloop.EncodeP2(intLag, frac, tmin)
	}

	// §A.3.10 eq. A.9 / A.10 commit and FCB + gain quantization
	// driver per Phase 2d INT-0. fcbStep owns:
	//   - oldExc shift + append u(n) = ĝp·v(n) + ĝc·c(n) (eq. A.9)
	//   - swMemErr ← x(n) − ĝp·y(n) − ĝc·z(n) for n=30..39 (eq. A.10)
	//   - per-frame s/c/ga/gb output bits
	//   - pastQuaEn FIFO advance and prevGpQ14 / prevTaming carry
	e.fcbStep(sub, aHat, sFrame, x, &y, h, &v, gp)

	// Quantized-Â analysis-filter memory advance for next subframe.
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func (e *Encoder) closedLoopExcitationSearch(
	residual *[closedloop.SubframeLen]int16,
	exc *[closedLoopPitchSearchLen]int16,
) []int16 {
	copy(exc[:closedLoopPitchSearchHistory],
		e.oldExc[len(e.oldExc)-closedLoopPitchSearchHistory:])
	copy(exc[closedLoopPitchSearchHistory:], residual[:])
	return exc[:]
}

type encoderPitchClipCandidate struct {
	intLag int16
	frac   int8
}

func (e *Encoder) qualityRepairPitchClip(
	sub int,
	aHat *[lpc.LPCOrder + 1]int16,
	sFrame *[closedloop.SubframeLen]int16,
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	intLag int16,
	frac int8,
) (int16, int8) {
	bestEncoder := *e
	bestEncoder.commitClosedLoopPitch(sub, aHat, sFrame, x, h, exc, intLag, frac)
	bestScore := bestEncoder.qualityLastOutput
	baseScore := bestScore
	bestLag, bestFrac := intLag, frac

	if bestScore.nearClip > 0 || bestScore.peak >= qualityPitchClipRepairPeakThreshold {
		var candidates [18]encoderPitchClipCandidate
		count := qualityPitchClipCandidates(sub, e.intT1, intLag, frac, &candidates)
		for i := 0; i < count; i++ {
			cand := candidates[i]
			candEncoder := *e
			candEncoder.commitClosedLoopPitch(sub, aHat, sFrame, x, h, exc, cand.intLag, cand.frac)
			candScore := candEncoder.qualityLastOutput
			if qualityPitchClipOutputScoreLess(candScore, bestScore, baseScore) {
				bestEncoder = candEncoder
				bestScore = candScore
				bestLag, bestFrac = cand.intLag, cand.frac
			}
		}
	}

	*e = bestEncoder
	return bestLag, bestFrac
}

func qualityPitchClipOutputScoreLess(candidate, best, base encoderQualityOutputScore) bool {
	if base.hardClip > 0 || base.nearClip > 0 {
		return encoderQualityOutputScoreLess(candidate, best)
	}
	if candidate.peak >= base.peak {
		return false
	}
	if candidate.mse*100 > base.mse*117 {
		return false
	}
	if best.peak >= base.peak {
		return true
	}
	if candidate.mse != best.mse {
		return candidate.mse < best.mse
	}
	return candidate.peak < best.peak
}

func qualityPitchClipCandidates(
	sub int,
	intT1 int16,
	intLag int16,
	frac int8,
	out *[18]encoderPitchClipCandidate,
) int {
	count := 0
	add := func(lag int16, candFrac int8) {
		if lag == intLag && candFrac == frac {
			return
		}
		for i := 0; i < count; i++ {
			if out[i].intLag == lag && out[i].frac == candFrac {
				return
			}
		}
		if count < len(out) {
			out[count] = encoderPitchClipCandidate{intLag: lag, frac: candFrac}
			count++
		}
	}

	if sub == 0 {
		for dt := int16(-2); dt <= 2; dt++ {
			lag := intLag + dt
			for _, candFrac := range [...]int8{-1, 0, 1} {
				if !qualityPitchClipSubframe1CandidateRepresentable(lag, candFrac) {
					continue
				}
				p1 := closedloop.EncodeP1(lag, candFrac)
				rtLag, rtFrac := pitchcore.DecodeDelaySubframe1(p1)
				add(int16(rtLag), int8(rtFrac))
			}
		}
		return count
	}

	tmin, _ := closedloop.Subframe2Window(intT1)
	for dt := int16(-2); dt <= 2; dt++ {
		lag := intLag + dt
		for _, candFrac := range [...]int8{-1, 0, 1} {
			p2 := 3*(int(lag)-int(tmin)) + int(candFrac) + 2
			if p2 < 0 || p2 > 31 {
				continue
			}
			rtLag, rtFrac := pitchcore.DecodeDelaySubframe2(uint8(p2), int(intT1))
			add(int16(rtLag), int8(rtFrac))
		}
	}
	return count
}

func qualityPitchClipSubframe1CandidateRepresentable(intLag int16, frac int8) bool {
	if frac < -1 || frac > 1 {
		return false
	}
	if intLag == 19 {
		return frac == 1
	}
	if intLag >= 20 && intLag <= 84 {
		return true
	}
	if intLag == 85 {
		return frac <= 0
	}
	if intLag >= 86 && intLag <= closedloop.PitchMaxInt {
		return frac == 0
	}
	return false
}

func (e *Encoder) searchPitchNormalizedAdaptive(
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	centre int16,
	sub int,
) int16 {
	kMin, kMax := closedLoopPitchSearchRangeWithHalfWindow(centre, sub, qualityNormalizedPitchSearchHalfWindow)
	bestLag := int16(kMin)
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		score := e.normalizedAdaptivePitchScore(x, h, exc, int16(k), 0)
		if score > bestScore {
			bestScore = score
			bestLag = int16(k)
		}
	}
	if qualityNormalizedPitchHarmonicGuardRatio > 0 && sub == 0 && isHigherPitchMultipleCandidate(int(centre), int(bestLag)) {
		localMin, localMax := closedloop.Subframe1Window(centre)
		localLag := localMin
		localScore := math.Inf(-1)
		for k := int(localMin); k <= int(localMax); k++ {
			score := e.normalizedAdaptivePitchScore(x, h, exc, int16(k), 0)
			if score > localScore {
				localScore = score
				localLag = int16(k)
			}
		}
		if localScore > 0 && bestScore <= localScore*qualityNormalizedPitchHarmonicGuardRatio {
			bestLag = localLag
		}
	}
	return bestLag
}

func (e *Encoder) refinePitchNormalizedAdaptive(
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	intLag int16,
	allowFrac bool,
	sub int,
) int8 {
	if !allowFrac {
		return 0
	}
	bestFrac := int8(-1)
	bestScore := e.normalizedAdaptivePitchScore(x, h, exc, intLag, -1)
	positiveScore := math.Inf(-1)
	for _, frac := range [2]int8{0, 1} {
		score := e.normalizedAdaptivePitchScore(x, h, exc, intLag, frac)
		if frac == 1 {
			positiveScore = score
		}
		if score > bestScore {
			bestScore = score
			bestFrac = frac
		}
	}
	if qualityNormalizedPitchShortPositiveFracRatio > 0 &&
		sub == 1 &&
		intLag < closedloop.SubframeLen &&
		bestFrac != 1 &&
		positiveScore > 0 &&
		positiveScore >= bestScore*qualityNormalizedPitchShortPositiveFracRatio {
		return 1
	}
	return bestFrac
}

func (e *Encoder) normalizedAdaptivePitchScore(
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	intLag int16,
	frac int8,
) float64 {
	var v, y [closedloop.SubframeLen]int16
	e.adaptiveVectorForSynthesis(exc, intLag, frac, &v)
	closedloop.GpAndY(x, &v, h, &y)
	var corr, energy float64
	for n := 0; n < closedloop.SubframeLen; n++ {
		yn := float64(y[n])
		corr += float64(x[n]) * yn
		energy += yn * yn
	}
	if corr <= 0 || energy <= 0 {
		return math.Inf(-1)
	}
	return (corr * corr) / energy
}

func (e *Encoder) adaptiveVectorForSynthesis(
	exc []int16,
	intLag int16,
	frac int8,
	v *[closedloop.SubframeLen]int16,
) {
	// Annex A copies the LP residual into u(0..39) to simplify the
	// closed-loop pitch search. Once the transmitted delay is chosen, the
	// synthesis/gain/commit path must use the same adaptive-codebook vector
	// the decoder reconstructs; otherwise the encoder optimizes against an
	// unreachable receiver state. This matters not only for integer delays
	// shorter than 40, but also for fractional delays near the boundary where
	// b30 forward taps would otherwise read the residual-extension tail.
	if e.qualityResidualExtensionAdaptiveVectorEnabled() {
		closedloop.AdaptiveVector(exc, intLag, frac, v)
		return
	}
	pitchcore.AdaptiveCodebook(int(intLag), int(frac), e.oldExc[:], v)
}

func (e *Encoder) qualityPitchCenterCandidate(
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	centre int16,
	sub int,
) int16 {
	if sub != 0 {
		return centre
	}
	fullT, fullScore := e.bestAdaptivePitchScoreInRange(x, h, exc, closedloop.PitchMinInt, closedloop.PitchMaxInt, sub)
	if !isPitchSubmultipleForCenter(int(centre), int(fullT)) &&
		!isLowerPitchCenterRescueCandidate(int(centre), int(fullT)) {
		return centre
	}
	kMin, kMax := closedLoopPitchSearchRangeWithHalfWindow(centre, sub, qualityNormalizedPitchSearchHalfWindow)
	_, windowScore := e.bestAdaptivePitchScoreInRange(x, h, exc, kMin, kMax, sub)
	if math.IsInf(fullScore, -1) || math.IsInf(windowScore, -1) || fullScore < windowScore*qualityPitchCenterFullSearchMinScoreRatio {
		return centre
	}
	return fullT
}

func isLowerPitchCenterRescueCandidate(centre, candidate int) bool {
	return candidate+20 <= centre
}

func isHigherPitchMultipleCandidate(centre, candidate int) bool {
	if centre <= 0 || candidate <= centre {
		return false
	}
	for multiple := 2; multiple <= 4; multiple++ {
		d := candidate - multiple*centre
		if d < 0 {
			d = -d
		}
		if d <= 4 {
			return true
		}
		if multiple*centre > candidate+4 {
			return false
		}
	}
	return false
}

func (e *Encoder) bestAdaptivePitchScoreInRange(
	x, h *[closedloop.SubframeLen]int16,
	exc []int16,
	kMin, kMax int,
	sub int,
) (int16, float64) {
	bestT := int16(kMin)
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		fracs, n := pitchCandidateFractions(sub, int16(k))
		for i := 0; i < n; i++ {
			score := e.normalizedAdaptivePitchScore(x, h, exc, int16(k), fracs[i])
			if score > bestScore {
				bestScore = score
				bestT = int16(k)
			}
		}
	}
	return bestT, bestScore
}

func pitchCandidateFractions(sub int, intLag int16) ([3]int8, int) {
	if sub == 1 || intLag < 85 {
		return [3]int8{-1, 0, 1}, 3
	}
	return [3]int8{0, 0, 0}, 1
}

func isPitchSubmultipleForCenter(higher, lower int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= 2 {
			return true
		}
		if k*lower > higher+2 {
			return false
		}
	}
	return false
}

func closedLoopPitchSearchRange(centre int16, sub int) (kMin, kMax int) {
	return closedLoopPitchSearchRangeWithHalfWindow(centre, sub, 3)
}

func closedLoopPitchSearchRangeWithHalfWindow(centre int16, sub int, halfWindow int) (kMin, kMax int) {
	if sub == 0 {
		if halfWindow <= 0 {
			halfWindow = 3
		}
		if halfWindow == 3 {
			tMin, tMax := closedloop.Subframe1Window(centre)
			return int(tMin), int(tMax)
		}
		kMin = int(centre) - halfWindow
		if kMin < closedloop.PitchMinInt {
			kMin = closedloop.PitchMinInt
		}
		kMax = int(centre) + halfWindow
		if kMax > closedloop.PitchMaxInt {
			kMax = closedloop.PitchMaxInt
		}
		return kMin, kMax
	}
	tMin, tMax := closedloop.Subframe2Window(centre)
	return int(tMin), int(tMax)
}

// fcbStep runs the §3.8 + §3.9 fixed-codebook search + gain
// quantization chain on one subframe, and commits the §A.3.10 eq. A.9
// excitation update (oldExc) and eq. A.10 weighted-error update
// (swMemErr). Per Phase 2d sub-plan §6.1 INT-0:
//
//  1. CB-1 AdjustedTarget  : x'(n) = x(n) − gp·y(n)        (§3.8.1 eq. 50)
//  2. CB-1 hSearch         : h′(n) with fixed-codebook pitch enh.
//     folded into search                                      (§3.8 eq. 49)
//  3. CB-1 CorrelationD    : d(n) = Σ x'(i)·h′(i−n)        (§3.8.1 eq. 52)
//  4. CB-3 SignsFromD      : signs[n], |d(n)|              (§3.8.1)
//  5. CB-2 PhiPrime        : φ′(i,j) from h′               (§3.8.1 eq. 56–57)
//  6. CB-2 focused/depth-first search: 4-pulse positions  (§3.8.1/A.3.8.1)
//  7. CB-4 BuildCode       : c[40] (Q13) + harmonic enh.   (§3.8 eq. 45–47)
//  8. CB-5 FilterCode      : z[40] (Q12) = c ⊛ h           (§3.9 eq. 64)
//  9. GQ-1 PredictedGcQ12  : g'c (Q12)                     (§3.9.1 eq. 71)
//  10. GQ-2 SearchConjugate: (ga, gb, ĝp Q14, γ̂c Q13)     (§3.9.2 eq. 73, 74)
//  11. GQ-3 Tame           : optional ĝp clamp             (§3.9.2 taming)
//  12. ENC-1 PackS / PackC : (S, C) bit fields             (§3.8.2 eq. 61, 62)
//  13. ENC-1 PackGains     : (GA, GB) bit fields           (§3.9.3)
//  14. eq. A.10 commit     : swMemErr[n−30] = sat(x(n) −
//     ĝp·y(n) − ĝc·z(n)) for n=30..39 (G729E.txt line 2211)
//  15. eq. A.9  commit     : shift oldExc left by SubframeLen and
//     append u(n) = sat(ĝp·v(n) + ĝc·c(n)) for n=0..39
//     (line 2202)
//  16. GQ-3 UpdatePastQuaEn: FIFO shift; new entry = 20·log10(γ̂_c) Q10
//     (§3.9.1 eq. 72)
//  17. prevGpQ14 ← ĝp ; prevTaming ← taming
//
// Q-format reconciliation: ĝp is Q14 and y/v are Q0. The fixed gain ĝc is
// reconstructed from γ̂c and g'c into the decoder-shared mantissa/exponent
// representation. applyGcToQ12 maps ĝc·z from z Q12 to a Q0 weighted-error
// contribution; synth.BuildExcitation maps ĝc·c from c Q13 to a Q0 excitation
// contribution using the same decoder-side arithmetic as the receive path.
// SearchConjugate's Eq. 63 cost uses the equivalent γ̂c·g'c Q12 value against
// z Q12, preserving the standard gain-search objective without storing ĝc in a
// saturated Word16.
//
// I3 (relaxed for per-subframe state): commits oldExc, swMemErr,
// pastQuaEn, prevGpQ14, prevTaming, and the per-subframe s*/c*/ga*/gb*
// output fields. I4: zero allocation — all scratch (xPrime, hSearch, d,
// signs, dAbs, phi, positions, c, z) lives on the stack.
func (e *Encoder) fcbStep(
	sub int,
	aHat *[lpc.LPCOrder + 1]int16,
	refSpeech *[closedloop.SubframeLen]int16,
	x, y, h, v *[closedloop.SubframeLen]int16,
	gpUnq int16,
) {
	const N = closedloop.SubframeLen

	// 1. CB-1: x'(n) = x(n) − gp·y(n).
	var xPrime [N]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	// intLag for harmonic enhancement: per §3.8 eq. 46/48, T is the
	// integer pitch lag of the current subframe; reuse the closed-loop
	// winner cached on the encoder.
	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}

	// §3.8 eq. 49 incorporates fixed-codebook pitch enhancement into
	// the search by applying the same β/T recursion to the impulse
	// response used for d(n) and phi. The final code vector is still
	// pitch-enhanced in BuildCode and filtered through the original h,
	// which is the equivalent synthesis path within the fixed-point
	// rounding bound pinned by the FCB FilterCode tests.
	hSearch := *h
	fcb.ApplyPitchEnhancement(&hSearch, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14))

	// 2. CB-1: d(n) = Σ x'(i)·h(i−n).
	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	// 3. CB-3: signs / |d|.
	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	// 4. CB-2: φ′(i,j).  ~6.4 KB on stack.
	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	// 5. CB-2: depth-first 4-pulse search.
	var positions [4]int8
	var sumOut [2]int64
	if e.qualityFCBThresholdScanActive() || e.coreFCBThresholdScanEnabled() {
		limit := qualityFCBThresholdScanLimit
		if e.coreFCBThresholdScanEnabled() {
			limit = e.coreFCBThresholdScanLimit(sub)
		}
		entered := fcbsearch.SearchDepthFirstThresholdScanEntered(
			&dAbs, &phi, &positions, &sumOut,
			limit,
		)
		e.qualityFCBThresholdEntriesLast = entered
		e.qualityFCBThresholdEntriesFrame += entered
		if e.coreFCBThresholdScanEnabled() {
			e.recordCoreFCBThresholdEntries(entered)
		}
	} else {
		fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)
		e.qualityFCBThresholdEntriesLast = 0
	}
	if e.qualityFCBNoiseRerankEnabled() && aHat != nil && refSpeech != nil {
		tFrac := e.frac1
		if sub == 1 {
			tFrac = e.frac2
		}
		positions = e.qualityRerankFCBPositions(
			aHat, refSpeech, x, y, h,
			&signs, &dAbs, &phi,
			intLag, tFrac, positions,
		)
	}

	// 6. CB-4: c[40] with harmonic enhancement.
	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	// 7. CB-5: z[40] = c ⊛ h (Q12).
	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	// 8. GQ-1: g'c (Q12, NOT saturated; see PredictedGcQ12 docstring)
	// and the predictor's log2 form for §3.9.2 eq. (74) reconstruction
	// in the native (mant Q14, exp) decoder representation per REF-1.
	// Use the pitch-enhanced fixed-codebook vector `c`: §4.1.4 constructs
	// c from the pulses and then applies eq. 48 before §4.1.5 gain decode,
	// so encoder local state must mirror the receiver rather than reusing
	// the sparse eq. 45 vector for gain prediction.
	useNativeGainSearch := e.qualityNativeGainSearchEnabled()
	useWideGainPredictor := !e.qualityHeuristicsEnabled() || e.qualityWideGainPredictorEnabled() || useNativeGainSearch
	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	if useWideGainPredictor {
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&e.pastQuaEn, &c)
	}
	// Black-box gain-selection localization shows the unscaled local
	// search over-selects the fixed-codebook contribution. Apply the
	// bias only inside the search surface; reconstruction and predictor
	// update below continue to use the transmitted codebook pair exactly.
	xSearch := *x
	ySearch := *y
	gpcSearchQ12 := gpcPredQ12

	if e.qualityGainSearchBiasEnabled() {
		scaleGainSearchVector(&xSearch, qualityGainSearchTargetScaleNum, qualityGainSearchTargetScaleDen)
		scaleGainSearchVector(&ySearch, qualityGainSearchAdaptiveContributionScaleNum, qualityGainSearchAdaptiveContributionScaleDen)
		gpcSearchQ12 = scaleInt32RatioForGainSearch(
			gpcPredQ12,
			qualityGainSearchFixedContributionScaleNum,
			qualityGainSearchFixedContributionScaleDen,
		)
	}

	// 9. GQ-2: conjugate-codebook 2D VQ → (ga, gb, ĝp Q14, γ̂_c Q13).
	var gaPhys, gbPhys uint8
	var gpHatQ14, gammaCQ13 int16
	if useNativeGainSearch {
		// Quality heuristic: evaluate all standard gain-index pairs using
		// the exact reconstructed-gain residual that will be committed below.
		// This keeps the payload standard-compatible but is not Annex A's
		// GA/GB preselect procedure, so Core deliberately leaves it disabled.
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = searchConjugateNativeGainWide(&e.pastQuaEn, &c, x, y, &z)
	} else if e.coreGainPreselectPrecisionEnabled() {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = gainquant.SearchConjugatePreselectTargetBits(&xSearch, &ySearch, &z, gpcSearchQ12, encoderCoreGainPreselectTargetBits)
	} else {
		gaPhys, gbPhys, gpHatQ14, gammaCQ13 = gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	}

	// 10. GQ-3: taming (one-sided clamp on ĝp under predicted-overflow).
	gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
	taming := gpTamed != gpHatQ14
	gpHatQ14 = gpTamed

	// 11. ENC-1: pack S, C.
	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)

	// 12b. Reconstruct the chosen quantized g_c in the native
	// (mant Q14, exp) representation that mirrors gain.Decoder.Decode
	// bit-for-bit (REF-1 §2 / IMPL-3 step C). The §A.3.10 commits below
	// then use that same representation for swMemErr and oldExc commits,
	// eliminating the pre-IMPL-3 int16 collapse in `gcHatQ12` while
	// keeping encoder-side state aligned with decoder reconstruction.
	var gcMantQ14 int16
	var gcExp int8
	if useWideGainPredictor {
		_, gcMantQ14, gcExp = gainquant.ReconstructWide(&e.pastQuaEn, &c, gaPhys, gbPhys)
	} else {
		_, gcMantQ14, gcExp = gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	}

	if e.qualityGainClipRepairEnabled() && aHat != nil && refSpeech != nil {
		tFrac := e.frac1
		if sub == 1 {
			tFrac = e.frac2
		}
		gaPhys, gbPhys, gpHatQ14, gammaCQ13, taming, gcMantQ14, gcExp = e.qualityRepairGainClip(
			aHat, refSpeech, intLag, tFrac, cPacked, s, &c, gaPhys, gbPhys, gpHatQ14, gammaCQ13, taming, gcMantQ14, gcExp,
		)
	}

	// 12. ENC-1: pack GA, GB (forward §3.9.3 imap).
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)

	if sub == 0 {
		e.s1 = s
		e.c1 = cPacked
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.s2 = s
		e.c2 = cPacked
		e.ga2 = gaBits
		e.gb2 = gbBits
	}

	// 13. §A.3.10 eq. A.10 commit: swMemErr ← x − ĝp·y − ĝc·z.
	// Keep this commit in the same physical Q0 domain as x. y is Q0;
	// z is Q12 from FilterCode, so the fixed contribution uses the
	// Q12-sample variant of the same mantissa/exponent arithmetic used
	// by synth.BuildExcitation.
	for n := 30; n < N; n++ {
		gpY := applyGainQ14ToQ0(gpHatQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	// 14. §A.3.10 eq. A.9 commit: shift oldExc left by N, append
	// u(n) = ĝp·v(n) + ĝc·c(n). Use the decoder-side excitation builder
	// directly so the encoder's adaptive-codebook memory follows the
	// same reconstructed excitation the receiver will synthesize.
	copy(e.oldExc[:len(e.oldExc)-N], e.oldExc[N:])
	base := len(e.oldExc) - N
	var uHat [N]int16
	synth.BuildExcitation(gpHatQ14, gcMantQ14, gcExp, v, &c, &uHat)
	copy(e.oldExc[base:], uHat[:])

	// 15. GQ-3 part B: pastQuaEn FIFO advance with γ̂_c (Q13). The
	// chosen γ̂_c is now returned directly by SearchConjugate (sum of
	// GBK1[ga][1] + GBK2[gb][1]); no inversion of the saturated ĝc is
	// required (replaces the pre-IMPL-3 recoverGammaCQ13 helper).
	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)

	// 16. Carry forward.
	e.prevGpQ14 = gpHatQ14
	e.prevTaming = taming
	if sub == 1 && e.qualityFCBClipCooldown > 0 {
		e.qualityFCBClipCooldown--
	}
}

func (e *Encoder) qualityRerankFCBPositions(
	aHat *[lpc.LPCOrder + 1]int16,
	refSpeech *[closedloop.SubframeLen]int16,
	x, y, h *[closedloop.SubframeLen]int16,
	signs *[closedloop.SubframeLen]int16,
	dAbs *[closedloop.SubframeLen]int32,
	phi *[closedloop.SubframeLen][closedloop.SubframeLen]int32,
	intLag int16,
	tFrac int8,
	current [4]int8,
) [4]int8 {
	ref := *refSpeech
	pcm.ScaleUpSat(ref[:], ref[:])

	initialState := encoderQualityDecodeState{
		gainDec: e.qualityGainDec,
		synth:   e.qualitySynth,
		pf:      e.qualityPostfilter,
		pastExc: e.qualityPastExc,
		prevGp:  e.qualityPrevGpQ14,
		hpX:     e.qualityHPX,
		hpY:     e.qualityHPY,
	}

	best := current
	bestScore := e.scoreFCBRerankPosition(initialState, aHat, &ref, x, y, h, signs, &best, intLag, tFrac)

	var top [fcbsearch.SearchTopKMax][4]int8
	topN := fcbsearch.SearchTopK(dAbs, phi, &top, qualityCleanFCBRerankTopK)
	for i := 0; i < topN; i++ {
		cand := top[i]
		score := e.scoreFCBRerankPosition(initialState, aHat, &ref, x, y, h, signs, &cand, intLag, tFrac)
		if encoderQualityFCBRerankScoreLess(score, bestScore) {
			best = cand
			bestScore = score
		}
	}
	return best
}

func (e *Encoder) scoreFCBRerankPosition(
	initialState encoderQualityDecodeState,
	aHat *[lpc.LPCOrder + 1]int16,
	ref *[closedloop.SubframeLen]int16,
	x, y, h *[closedloop.SubframeLen]int16,
	signs *[closedloop.SubframeLen]int16,
	positions *[4]int8,
	intLag int16,
	tFrac int8,
) encoderQualityOutputScore {
	var c [closedloop.SubframeLen]int16
	fcbsearch.BuildCode(positions, signs, intLag, e.prevGpQ14, &c)

	var z [closedloop.SubframeLen]int16
	fcbsearch.FilterCode(&c, h, &z)

	useNativeGainSearch := e.qualityNativeGainSearchEnabled()
	useWideGainPredictor := !e.qualityHeuristicsEnabled() || e.qualityWideGainPredictorEnabled() || useNativeGainSearch
	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	if useWideGainPredictor {
		gpcPredQ12 = gainquant.PredictedGcQ12Wide(&e.pastQuaEn, &c)
	}

	xSearch := *x
	ySearch := *y
	gpcSearchQ12 := gpcPredQ12
	if e.qualityGainSearchBiasEnabled() {
		scaleGainSearchVector(&xSearch, qualityGainSearchTargetScaleNum, qualityGainSearchTargetScaleDen)
		scaleGainSearchVector(&ySearch, qualityGainSearchAdaptiveContributionScaleNum, qualityGainSearchAdaptiveContributionScaleDen)
		gpcSearchQ12 = scaleInt32RatioForGainSearch(
			gpcPredQ12,
			qualityGainSearchFixedContributionScaleNum,
			qualityGainSearchFixedContributionScaleDen,
		)
	}

	var gaPhys, gbPhys uint8
	if useNativeGainSearch {
		gaPhys, gbPhys, _, _ = searchConjugateNativeGainWide(&e.pastQuaEn, &c, x, y, &z)
	} else if e.coreGainPreselectPrecisionEnabled() {
		gaPhys, gbPhys, _, _ = gainquant.SearchConjugatePreselectTargetBits(&xSearch, &ySearch, &z, gpcSearchQ12, encoderCoreGainPreselectTargetBits)
	} else {
		gaPhys, gbPhys, _, _ = gainquant.SearchConjugate(&xSearch, &ySearch, &z, gpcSearchQ12)
	}

	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	cPacked := fcbsearch.PackC(positions)
	sPacked := fcbsearch.PackS(positions, signs)
	_, out := simulateEncoderQualityDecodeOutput(initialState, aHat, int(intLag), int(tFrac), cPacked, sPacked, gaBits, gbBits)
	return scoreEncoderQualityOutput(out[:], ref[:], qualityGainClipRepairThreshold)
}

func encoderQualityFCBRerankScoreLess(candidate, best encoderQualityOutputScore) bool {
	if candidate.hardClip != best.hardClip {
		return candidate.hardClip < best.hardClip
	}
	if candidate.nearClip != best.nearClip {
		return candidate.nearClip < best.nearClip
	}
	if candidate.highMSE < best.highMSE &&
		candidate.mse*qualityCleanFCBRerankHighMSEBetterMSEToleranceDen <= best.mse*qualityCleanFCBRerankHighMSEBetterMSEToleranceNum {
		return true
	}
	if candidate.mse < best.mse &&
		candidate.highMSE*qualityCleanFCBRerankMSEBetterHighMSEToleranceDen <= best.highMSE*qualityCleanFCBRerankMSEBetterHighMSEToleranceNum {
		return true
	}
	return false
}

func searchConjugateNativeGainWide(past *[4]int16, c, x, y, z *[40]int16) (ga, gb uint8, gpQ14, gammaCQ13 int16) {
	bestCost := int64(1<<63 - 1)
	for gai := uint8(0); gai < 8; gai++ {
		for gbi := uint8(0); gbi < 16; gbi++ {
			gp, gcMant, gcExp := gainquant.ReconstructWide(past, c, gai, gbi)
			cost := gainResidualEnergyCostQ0(x, y, z, gp, gcMant, gcExp)
			if cost < bestCost {
				bestCost = cost
				ga = gai
				gb = gbi
				gpQ14 = gp
				gammaCQ13 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[gai][1]) + int32(tables.GainGBK2[gbi][1])))
			}
		}
	}
	return
}

func gainResidualEnergyCostQ0(x, y, z *[40]int16, gpQ14, gcMantQ14 int16, gcExp int8) int64 {
	const (
		maxInt64     = int64(1<<63 - 1)
		maxSqrtInt64 = int64(3037000499)
	)
	var sum int64
	for n := 0; n < 40; n++ {
		gpY := applyGainQ14ToQ0(gpQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		err := int64(int32(x[n]) - gpY - gcZ)
		if err < 0 {
			err = -err
		}
		if err > maxSqrtInt64 {
			return maxInt64
		}
		sq := err * err
		if sum > maxInt64-sq {
			return maxInt64
		}
		sum += sq
	}
	return sum
}

type encoderQualityDecodeState struct {
	gainDec gain.Decoder
	synth   synth.Synthesizer
	pf      postfilter.Postfilter
	pastExc [153]int16
	prevGp  int16
	hpX     [2]int16
	hpY     [2]int32
}

type encoderQualityOutputScore struct {
	hardClip int
	nearClip int
	peak     int
	mse      int64
	highMSE  int64
}

func (e *Encoder) qualityRepairGainClip(
	aHat *[lpc.LPCOrder + 1]int16,
	refSpeech *[closedloop.SubframeLen]int16,
	intLag int16,
	tFrac int8,
	cPacked uint16,
	sPacked uint8,
	encoderC *[closedloop.SubframeLen]int16,
	gaPhys uint8,
	gbPhys uint8,
	gpQ14 int16,
	gammaCQ13 int16,
	taming bool,
	gcMantQ14 int16,
	gcExp int8,
) (uint8, uint8, int16, int16, bool, int16, int8) {
	ref := *refSpeech
	pcm.ScaleUpSat(ref[:], ref[:])

	initialState := encoderQualityDecodeState{
		gainDec: e.qualityGainDec,
		synth:   e.qualitySynth,
		pf:      e.qualityPostfilter,
		pastExc: e.qualityPastExc,
		prevGp:  e.qualityPrevGpQ14,
		hpX:     e.qualityHPX,
		hpY:     e.qualityHPY,
	}
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	bestState, bestOut := simulateEncoderQualityDecodeOutput(initialState, aHat, int(intLag), int(tFrac), cPacked, sPacked, gaBits, gbBits)
	bestScore := scoreEncoderQualityOutput(bestOut[:], ref[:], qualityGainClipRepairThreshold)

	bestGA, bestGB := gaPhys, gbPhys
	bestGp := gpQ14
	bestGamma := gammaCQ13
	bestTaming := taming
	bestGcMant := gcMantQ14
	bestGcExp := gcExp

	searchThreshold := qualityGainClipRepairThreshold
	searchAll := bestScore.nearClip > 0
	mseRepair := false
	if !searchAll && e.qualityGainMSERepairEnabled() {
		searchThreshold = e.qualityGainMSERepairThreshold()
		bestScore = scoreEncoderQualityOutput(bestOut[:], ref[:], searchThreshold)
		searchAll = true
		mseRepair = true
	}

	if searchAll {
		useWideCandidate := e.qualityWideGainPredictorEnabled() || e.qualityNativeGainSearchEnabled()
		for ga := uint8(0); ga < 8; ga++ {
			for gb := uint8(0); gb < 16; gb++ {
				candGpRaw, candGamma := encoderGainCandidate(ga, gb)
				candGp := gainquant.Tame(candGpRaw, &e.oldExc)
				_, candGcMant, candGcExp := gainquant.Reconstruct(&e.pastQuaEn, encoderC, ga, gb)
				if useWideCandidate {
					_, candGcMant, candGcExp = gainquant.ReconstructWide(&e.pastQuaEn, encoderC, ga, gb)
				}
				candGABits, candGBBits := gainquant.PackGains(ga, gb)
				candState, candOut := simulateEncoderQualityDecodeOutput(initialState, aHat, int(intLag), int(tFrac), cPacked, sPacked, candGABits, candGBBits)
				candScore := scoreEncoderQualityOutput(candOut[:], ref[:], searchThreshold)
				better := encoderQualityOutputScoreLess(candScore, bestScore)
				if mseRepair {
					highBetterNum, highBetterDen := e.qualityGainNoiseRepairHighMSEBetterTolerance()
					mseBetterNum, mseBetterDen := e.qualityGainNoiseRepairMSEBetterTolerance()
					better = candScore.hardClip == 0 &&
						candScore.nearClip == 0 &&
						(bestScore.hardClip != 0 || bestScore.nearClip != 0 ||
							encoderQualityMSERepairScoreLess(candScore, bestScore, e.qualityGainNoiseRepairEnabled(),
								highBetterNum, highBetterDen, mseBetterNum, mseBetterDen))
					if !better && e.qualityGainPitchPreferenceEnabled() &&
						candScore.hardClip == 0 && candScore.nearClip == 0 &&
						bestScore.hardClip == 0 && bestScore.nearClip == 0 &&
						int32(candGp) >= int32(bestGp)+int32(qualityCleanVoicedGainPitchMinStepQ14) &&
						candScore.mse*qualityCleanVoicedGainPitchMSEToleranceDen <= bestScore.mse*qualityCleanVoicedGainPitchMSEToleranceNum &&
						candScore.highMSE*qualityCleanVoicedGainPitchHighMSEToleranceDen <= bestScore.highMSE*qualityCleanVoicedGainPitchHighMSEToleranceNum {
						better = true
					}
					if !better && e.qualityGainDegritPreferenceEnabled() &&
						candScore.hardClip == 0 && candScore.nearClip == 0 &&
						bestScore.hardClip == 0 && bestScore.nearClip == 0 &&
						int32(candGp) >= int32(bestGp) &&
						int32(candGamma)+qualityCleanDegritGammaDropMinQ13 <= int32(bestGamma) &&
						candScore.mse*qualityCleanDegritMSEToleranceDen <= bestScore.mse*qualityCleanDegritMSEToleranceNum &&
						candScore.highMSE*qualityCleanDegritHighMSEToleranceDen <= bestScore.highMSE*qualityCleanDegritHighMSEToleranceNum {
						better = true
					}
					if !better && e.qualityGainHarmonicPreferenceEnabled() {
						minGp, minStep, gammaDrop, mseNum, mseDen, highNum, highDen := e.qualityGainHarmonicPreferenceParams()
						if candScore.hardClip == 0 && candScore.nearClip == 0 &&
							bestScore.hardClip == 0 && bestScore.nearClip == 0 &&
							int32(candGp) >= minGp &&
							int32(candGp) >= int32(bestGp)+minStep &&
							int32(candGamma)+gammaDrop <= int32(bestGamma) &&
							candScore.mse*mseDen <= bestScore.mse*mseNum &&
							candScore.highMSE*highDen <= bestScore.highMSE*highNum {
							better = true
						}
					}
				}
				if better {
					bestScore = candScore
					bestState = candState
					bestOut = candOut
					bestGA, bestGB = ga, gb
					bestGp = candGp
					bestGamma = candGamma
					bestTaming = candGp != candGpRaw
					bestGcMant = candGcMant
					bestGcExp = candGcExp
				}
			}
		}
	}

	e.qualityLastOutput = scoreEncoderQualityOutput(bestOut[:], ref[:], qualityGainClipRepairThreshold)
	e.qualityGainDec = bestState.gainDec
	e.qualitySynth = bestState.synth
	e.qualityPostfilter = bestState.pf
	e.qualityPastExc = bestState.pastExc
	e.qualityPrevGpQ14 = bestState.prevGp
	e.qualityHPX = bestState.hpX
	e.qualityHPY = bestState.hpY
	return bestGA, bestGB, bestGp, bestGamma, bestTaming, bestGcMant, bestGcExp
}

func encoderGainCandidate(ga, gb uint8) (gpQ14 int16, gammaCQ13 int16) {
	gp := int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])
	gamma := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
	return int16(gp), fixed.Saturate(fixed.Word32(gamma))
}

func simulateEncoderQualityDecodeOutput(
	state encoderQualityDecodeState,
	aHat *[lpc.LPCOrder + 1]int16,
	intLag int,
	tFrac int,
	cPacked uint16,
	sPacked uint8,
	gaBits uint8,
	gbBits uint8,
) (encoderQualityDecodeState, [closedloop.SubframeLen]int16) {
	var v [closedloop.SubframeLen]int16
	pitchcore.AdaptiveCodebook(intLag, tFrac, state.pastExc[:], &v)

	var c [closedloop.SubframeLen]int16
	betaQ14 := fcb.ClampPitchGainForEnhancement(state.prevGp)
	fcb.Decode(fcb.Indices{Positions: cPacked, Signs: sPacked}, intLag, betaQ14, &c)

	gainTaps := state.gainDec.DecodeWithFullTaps(gain.Indices{GA: gaBits, GB: gbBits}, &c)

	var u [closedloop.SubframeLen]int16
	synth.BuildExcitation(gainTaps.GpQ14Final, gainTaps.GcMantQ14, gainTaps.GcExp, &v, &c, &u)

	var s [closedloop.SubframeLen]int16
	state.synth.Filter(aHat, &u, &s)

	var sPf [closedloop.SubframeLen]int16
	state.pf.Filter(aHat, intLag, &s, &sPf)

	var hp [closedloop.SubframeLen]int16
	encoderQualityHPFilter(&state.hpX, &state.hpY, &sPf, hp[:])

	out := hp
	pcm.ScaleUpSat(out[:], out[:])

	copy(state.pastExc[:len(state.pastExc)-closedloop.SubframeLen], state.pastExc[closedloop.SubframeLen:])
	copy(state.pastExc[len(state.pastExc)-closedloop.SubframeLen:], u[:])
	state.prevGp = gainTaps.GpQ14Final
	return state, out
}

func scoreEncoderQualityOutput(out []int16, ref []int16, threshold int) encoderQualityOutputScore {
	var score encoderQualityOutputScore
	var sum int64
	var highSum int64
	limit := len(out)
	if len(ref) < limit {
		limit = len(ref)
	}
	var prevErr int64
	for i := 0; i < limit; i++ {
		v := int(out[i])
		if v < 0 {
			v = -v
		}
		if v > score.peak {
			score.peak = v
		}
		if v >= threshold {
			score.nearClip++
		}
		if v >= 32767 {
			score.hardClip++
		}
		diff := int64(int(out[i]) - int(ref[i]))
		sum += diff * diff
		if i > 0 {
			high := diff - prevErr
			highSum += high * high
		}
		prevErr = diff
	}
	if limit == 0 {
		score.mse = 1<<63 - 1
		score.highMSE = 1<<63 - 1
	} else {
		score.mse = sum / int64(limit)
		if limit > 1 {
			score.highMSE = highSum / int64(limit-1)
		}
	}
	return score
}

func encoderQualityMSERepairScoreLess(
	candidate, best encoderQualityOutputScore,
	highAware bool,
	highMSEBetterMSEToleranceNum, highMSEBetterMSEToleranceDen int64,
	mseBetterHighMSEToleranceNum, mseBetterHighMSEToleranceDen int64,
) bool {
	if !highAware {
		return candidate.mse < best.mse
	}
	if candidate.highMSE < best.highMSE &&
		candidate.mse*highMSEBetterMSEToleranceDen <= best.mse*highMSEBetterMSEToleranceNum {
		return true
	}
	if candidate.mse < best.mse &&
		candidate.highMSE*mseBetterHighMSEToleranceDen <= best.highMSE*mseBetterHighMSEToleranceNum {
		return true
	}
	return false
}

func encoderQualityOutputScoreLess(a encoderQualityOutputScore, b encoderQualityOutputScore) bool {
	if a.hardClip != b.hardClip {
		return a.hardClip < b.hardClip
	}
	if a.nearClip != b.nearClip {
		return a.nearClip < b.nearClip
	}
	if a.mse == b.mse && a.highMSE != b.highMSE {
		return a.highMSE < b.highMSE
	}
	return a.mse < b.mse
}

func encoderQualityHPFilter(hpX *[2]int16, hpY *[2]int32, in *[closedloop.SubframeLen]int16, out []int16) {
	const (
		hpB0Q13    = 7699
		hpB1Q13    = -15399
		hpB2Q13    = 7699
		hpNegA1Q12 = 7918
		hpA2Q13    = 7667
	)
	x1 := hpX[0]
	x2 := hpX[1]
	y1 := hpY[0]
	y2 := hpY[1]
	for n := 0; n < closedloop.SubframeLen; n++ {
		xn := in[n]
		ff := int32(hpB0Q13)*int32(xn) +
			int32(hpB1Q13)*int32(x1) +
			int32(hpB2Q13)*int32(x2)
		fb := int64(hpNegA1Q12) * int64(y1)
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(y2)) >> 13
		acc := int64(ff) + fb
		yn := qualityRoundShift64(acc, 13)
		out[n] = fixed.Saturate(fixed.Word32(yn))
		x2 = x1
		x1 = xn
		y2 = y1
		y1 = int32(acc)
	}
	hpX[0] = x1
	hpX[1] = x2
	hpY[0] = y1
	hpY[1] = y2
}

func qualityRoundShift64(v int64, shift uint) int64 {
	if shift == 0 {
		return v
	}
	add := int64(1 << (shift - 1))
	if v >= 0 {
		return (v + add) >> shift
	}
	return -(((-v) + add) >> shift)
}

// mantExpToQ12 converts the native (mantissa Q14, exponent int8) g_c
// representation into a NON-saturated int32 Q12 value suitable for the
// §A.3.10 commit accumulators. Mirrors the decoder-side path used by
// synth.BuildExcitation: gc·c(n) at Q(14+exp+13)>>(15) = Q(12+exp);
// here we reduce to the sub-plan §3 chosen Q12 sample envelope.
//
// Math:
//
//	gcQ12 = (mant << 14) >> (14 - exp + 2)   for the canonical case
//	       = mant << exp / 4                (because mant is Q14, target Q12)
//	→ if exp ≥ 2:   mant << (exp - 2)
//	  if exp <  2:  mant >> (2 - exp)
//
// All shifts kept in int64 to absorb the rare exp ≥ 8 codebook entries
// (γ̂_c·g'c can run up to ≈160 ⇒ Q12 ≈ 655 360, fits in 20 bits) without
// the pre-IMPL-3 int16 saturation that biased the §A.3.10 commit.
func mantExpToQ12(mantQ14 int16, exp int8) int32 {
	if mantQ14 == 0 {
		return 0
	}
	shift := int(exp) - 2
	if shift >= 0 {
		v := int64(mantQ14) << uint(shift)
		if v > 0x7FFFFFFF {
			return 0x7FFFFFFF
		}
		if v < -0x80000000 {
			return -0x80000000
		}
		return int32(v)
	}
	return int32(int64(mantQ14) >> uint(-shift))
}

func applyGainQ14ToQ0(gainQ14 int16, sampleQ0 int16) int32 {
	prod := fixed.LMult(fixed.Word16(gainQ14), fixed.Word16(sampleQ0))
	return int32(fixed.Round(fixed.LShl(prod, 1)))
}

func applyGcToQ12(gcMantQ14 int16, gcExp int8, sampleQ12 int16) int32 {
	if gcMantQ14 == 0 || sampleQ12 == 0 {
		return 0
	}
	prod := fixed.LMult(fixed.Word16(gcMantQ14), fixed.Word16(sampleQ12))
	shiftR := 12 - int(gcExp)
	var scaled fixed.Word32
	if shiftR >= 0 {
		scaled = fixed.LShr(prod, fixed.Word16(shiftR))
	} else {
		scaled = fixed.LShl(prod, fixed.Word16(-shiftR))
	}
	return int32(fixed.Round(fixed.LShl(scaled, 1)))
}

func scaleInt32RatioForGainSearch(v int32, num, den int32) int32 {
	if den == 0 {
		return v
	}
	x := int64(v) * int64(num)
	if x >= 0 {
		x += int64(den) / 2
	} else {
		x -= int64(den) / 2
	}
	x /= int64(den)
	if x > 0x7fffffff {
		return 0x7fffffff
	}
	if x < -0x80000000 {
		return -0x80000000
	}
	return int32(x)
}

func scaleInt32ForGainSearch(v int32, factor int32) int32 {
	return scaleInt32RatioForGainSearch(v, factor, 1)
}

func scaleGainSearchVector(v *[closedloop.SubframeLen]int16, num, den int32) {
	if den == 0 {
		return
	}
	for i := range v {
		x := int64(v[i]) * int64(num)
		if x >= 0 {
			x += int64(den) / 2
		} else {
			x -= int64(den) / 2
		}
		v[i] = fixed.Saturate(int32(x / int64(den)))
	}
}
