package g729

import (
	"errors"
	"os"
	"testing"

	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/lpc"
	pitchcore "github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/pitch/openloop"
	"github.com/hunydev/g729/internal/synth"
)

func TestEncoder_NewEncoder_NotNil(t *testing.T) {
	e := NewEncoder()
	if e == nil {
		t.Fatal("NewEncoder returned nil")
	}
}

func TestEncoderProfiles(t *testing.T) {
	if defaultEncoderProfile != EncoderProfileCore {
		t.Fatalf("defaultEncoderProfile = %v, want EncoderProfileCore", defaultEncoderProfile)
	}

	defaultEnc := NewEncoder()
	if defaultEnc.profile != defaultEncoderProfile ||
		defaultEnc.qualityHeuristicsEnabled() ||
		defaultEnc.qualityExpandedLSPSearchEnabled() ||
		defaultEnc.qualityFCBThresholdScanActive() ||
		defaultEnc.qualityNormalizedOpenLoopSearchEnabled() ||
		defaultEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		defaultEnc.qualityNativeGainSearchEnabled() ||
		defaultEnc.qualityGainClipRepairEnabled() ||
		defaultEnc.qualityGainMSERepairEnabled() ||
		defaultEnc.qualityGainNoiseRepairEnabled() ||
		defaultEnc.qualityFCBNoiseRerankEnabled() ||
		!defaultEnc.coreFCBThresholdScanEnabled() {
		t.Fatalf("NewEncoder profile = %v quality=%t lspx=%t qualityFCB=%t normOpenLoop=%t normPitch=%t nativeGain=%t gainClip=%t gainMSE=%t gainNoise=%t fcbRerank=%t coreFCB=%t, want product default Core tuning",
			defaultEnc.profile,
			defaultEnc.qualityHeuristicsEnabled(),
			defaultEnc.qualityExpandedLSPSearchEnabled(),
			defaultEnc.qualityFCBThresholdScanActive(),
			defaultEnc.qualityNormalizedOpenLoopSearchEnabled(),
			defaultEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			defaultEnc.qualityNativeGainSearchEnabled(),
			defaultEnc.qualityGainClipRepairEnabled(),
			defaultEnc.qualityGainMSERepairEnabled(),
			defaultEnc.qualityGainNoiseRepairEnabled(),
			defaultEnc.qualityFCBNoiseRerankEnabled(),
			defaultEnc.coreFCBThresholdScanEnabled())
	}

	coreEnc := NewEncoderWithProfile(EncoderProfileCore)
	if coreEnc.profile != EncoderProfileCore ||
		coreEnc.qualityHeuristicsEnabled() ||
		coreEnc.qualityNormalizedOpenLoopSearchEnabled() ||
		!coreEnc.coreFCBThresholdScanEnabled() ||
		!coreEnc.coreGainPreselectPrecisionEnabled() ||
		!coreEnc.coreGainPredictorPrecisionEnabled() {
		t.Fatalf("NewEncoderWithProfile(Core) profile = %v quality=%t normOpenLoop=%t coreFCB=%t coreGainPreselect=%t coreGainPredictor=%t, want core heuristics disabled with spec FCB threshold scan",
			coreEnc.profile,
			coreEnc.qualityHeuristicsEnabled(),
			coreEnc.qualityNormalizedOpenLoopSearchEnabled(),
			coreEnc.coreFCBThresholdScanEnabled(),
			coreEnc.coreGainPreselectPrecisionEnabled(),
			coreEnc.coreGainPredictorPrecisionEnabled())
	}

	coreFastEnc := NewEncoderWithProfile(EncoderProfileCoreFast)
	if coreFastEnc.profile != EncoderProfileCoreFast ||
		coreFastEnc.qualityHeuristicsEnabled() ||
		coreFastEnc.qualityNormalizedOpenLoopSearchEnabled() ||
		!coreFastEnc.coreFCBThresholdScanEnabled() ||
		!coreFastEnc.coreFastFCBThresholdScanEnabled() ||
		coreFastEnc.coreFCBThresholdScanFrameLimit() != encoderCoreFastFCBThresholdScanFrameLimit ||
		coreFastEnc.coreFCBThresholdScanSubframe0Limit() != encoderCoreFastFCBThresholdScanSubframe0Limit ||
		coreFastEnc.coreGainPreselectPrecisionEnabled() ||
		!coreFastEnc.coreGainPredictorPrecisionEnabled() {
		t.Fatalf("NewEncoderWithProfile(CoreFast) profile=%v quality=%t normOpenLoop=%t coreFCB=%t fastFCB=%t frameLimit=%d sub0Limit=%d coreGainPreselect=%t coreGainPredictor=%t, want opt-in reduced-budget Core fast profile",
			coreFastEnc.profile,
			coreFastEnc.qualityHeuristicsEnabled(),
			coreFastEnc.qualityNormalizedOpenLoopSearchEnabled(),
			coreFastEnc.coreFCBThresholdScanEnabled(),
			coreFastEnc.coreFastFCBThresholdScanEnabled(),
			coreFastEnc.coreFCBThresholdScanFrameLimit(),
			coreFastEnc.coreFCBThresholdScanSubframe0Limit(),
			coreFastEnc.coreGainPreselectPrecisionEnabled(),
			coreFastEnc.coreGainPredictorPrecisionEnabled())
	}

	coreClipEnc := NewEncoderWithProfile(EncoderProfileCoreClipRepair)
	if coreClipEnc.profile != EncoderProfileCoreClipRepair ||
		!coreClipEnc.qualityHeuristicsEnabled() ||
		!coreClipEnc.qualityGainClipRepairEnabled() ||
		coreClipEnc.qualityNativeGainSearchEnabled() ||
		coreClipEnc.qualityGainMSERepairEnabled() ||
		coreClipEnc.qualityGainNoiseRepairEnabled() ||
		coreClipEnc.qualityGainClipRepairThreshold() != qualityCoreClipGainRepairThreshold ||
		!coreClipEnc.coreFCBThresholdScanEnabled() ||
		!coreClipEnc.coreGainPreselectPrecisionEnabled() {
		t.Fatalf("NewEncoderWithProfile(CoreClipRepair) profile = %v quality=%t gainClip=%t nativeGain=%t gainMSE=%t gainNoise=%t clipThreshold=%d coreFCB=%t coreGainPreselect=%t, want Core search with gain clip repair only",
			coreClipEnc.profile,
			coreClipEnc.qualityHeuristicsEnabled(),
			coreClipEnc.qualityGainClipRepairEnabled(),
			coreClipEnc.qualityNativeGainSearchEnabled(),
			coreClipEnc.qualityGainMSERepairEnabled(),
			coreClipEnc.qualityGainNoiseRepairEnabled(),
			coreClipEnc.qualityGainClipRepairThreshold(),
			coreClipEnc.coreFCBThresholdScanEnabled(),
			coreClipEnc.coreGainPreselectPrecisionEnabled())
	}

	annexALSPEnc := NewEncoderWithProfile(EncoderProfileQualityAnnexALSP)
	if annexALSPEnc.profile != EncoderProfileQualityAnnexALSP ||
		!annexALSPEnc.qualityHeuristicsEnabled() ||
		annexALSPEnc.qualityExpandedLSPSearchEnabled() ||
		annexALSPEnc.qualityNormalizedOpenLoopSearchEnabled() ||
		!annexALSPEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!annexALSPEnc.qualityNativeGainSearchEnabled() ||
		!annexALSPEnc.qualityGainClipRepairEnabled() ||
		!annexALSPEnc.qualityGainNoiseRepairEnabled() {
		t.Fatalf("NewEncoderWithProfile(QualityAnnexALSP) profile=%v quality=%t lspx=%t normOpenLoop=%t normPitch=%t nativeGain=%t gainClip=%t gainNoise=%t, want focused quality without expanded LSP",
			annexALSPEnc.profile,
			annexALSPEnc.qualityHeuristicsEnabled(),
			annexALSPEnc.qualityExpandedLSPSearchEnabled(),
			annexALSPEnc.qualityNormalizedOpenLoopSearchEnabled(),
			annexALSPEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			annexALSPEnc.qualityNativeGainSearchEnabled(),
			annexALSPEnc.qualityGainClipRepairEnabled(),
			annexALSPEnc.qualityGainNoiseRepairEnabled())
	}

	cleanEnc := NewEncoderWithProfile(EncoderProfileQualityClean)
	if cleanEnc.profile != EncoderProfileQualityClean ||
		!cleanEnc.qualityHeuristicsEnabled() ||
		!cleanEnc.qualityExpandedLSPSearchEnabled() ||
		cleanEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanEnc.qualityNativeGainSearchEnabled() ||
		!cleanEnc.qualityGainClipRepairEnabled() ||
		!cleanEnc.qualityGainMSERepairEnabled() ||
		!cleanEnc.qualityGainNoiseRepairEnabled() ||
		cleanEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityClean) profile=%v quality=%t lspx=%t normPitch=%t nativeGain=%t gainClip=%t gainMSE=%t gainNoise=%t mseThreshold=%d, want clean quality tuning",
			cleanEnc.profile,
			cleanEnc.qualityHeuristicsEnabled(),
			cleanEnc.qualityExpandedLSPSearchEnabled(),
			cleanEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanEnc.qualityNativeGainSearchEnabled(),
			cleanEnc.qualityGainClipRepairEnabled(),
			cleanEnc.qualityGainMSERepairEnabled(),
			cleanEnc.qualityGainNoiseRepairEnabled(),
			cleanEnc.qualityGainMSERepairThreshold())
	}
	highNum, highDen := cleanEnc.qualityGainNoiseRepairHighMSEBetterTolerance()
	if highNum != qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceNum ||
		highDen != qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceDen {
		t.Fatalf("QualityClean high-MSE tolerance = %d/%d, want %d/%d",
			highNum, highDen,
			qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceNum,
			qualityCleanGainNoiseRepairHighMSEBetterMSEToleranceDen)
	}

	cleanSNREnc := NewEncoderWithProfile(EncoderProfileQualityCleanSNR)
	if cleanSNREnc.profile != EncoderProfileQualityCleanSNR ||
		cleanSNREnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		cleanSNREnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanSNR) profile=%v normPitch=%t mseThreshold=%d, want SNR clean tuning",
			cleanSNREnc.profile,
			cleanSNREnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanSNREnc.qualityGainMSERepairThreshold())
	}
	snrHighNum, snrHighDen := cleanSNREnc.qualityGainNoiseRepairHighMSEBetterTolerance()
	if snrHighNum != qualityGainNoiseRepairHighMSEBetterMSEToleranceNum ||
		snrHighDen != qualityGainNoiseRepairHighMSEBetterMSEToleranceDen {
		t.Fatalf("QualityCleanSNR high-MSE tolerance = %d/%d, want default %d/%d",
			snrHighNum, snrHighDen,
			qualityGainNoiseRepairHighMSEBetterMSEToleranceNum,
			qualityGainNoiseRepairHighMSEBetterMSEToleranceDen)
	}

	cleanSmoothEnc := NewEncoderWithProfile(EncoderProfileQualityCleanSmooth)
	if cleanSmoothEnc.profile != EncoderProfileQualityCleanSmooth ||
		cleanSmoothEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		cleanSmoothEnc.qualityGainMSERepairThreshold() != qualityCleanSmoothGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanSmooth) profile=%v normPitch=%t mseThreshold=%d, want smooth clean tuning",
			cleanSmoothEnc.profile,
			cleanSmoothEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanSmoothEnc.qualityGainMSERepairThreshold())
	}
	smoothHighNum, smoothHighDen := cleanSmoothEnc.qualityGainNoiseRepairHighMSEBetterTolerance()
	if smoothHighNum != qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceNum ||
		smoothHighDen != qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceDen {
		t.Fatalf("QualityCleanSmooth high-MSE tolerance = %d/%d, want %d/%d",
			smoothHighNum, smoothHighDen,
			qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceNum,
			qualityCleanSmoothGainNoiseRepairHighMSEBetterMSEToleranceDen)
	}

	cleanVoicedEnc := NewEncoderWithProfile(EncoderProfileQualityCleanVoiced)
	if cleanVoicedEnc.profile != EncoderProfileQualityCleanVoiced ||
		cleanVoicedEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanVoicedEnc.qualityGainPitchPreferenceEnabled() ||
		cleanVoicedEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanVoiced) profile=%v normPitch=%t pitchPref=%t mseThreshold=%d, want voiced clean tuning",
			cleanVoicedEnc.profile,
			cleanVoicedEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanVoicedEnc.qualityGainPitchPreferenceEnabled(),
			cleanVoicedEnc.qualityGainMSERepairThreshold())
	}

	cleanDegritEnc := NewEncoderWithProfile(EncoderProfileQualityCleanDegrit)
	if cleanDegritEnc.profile != EncoderProfileQualityCleanDegrit ||
		cleanDegritEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanDegritEnc.qualityGainDegritPreferenceEnabled() ||
		cleanDegritEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanDegrit) profile=%v normPitch=%t degritPref=%t mseThreshold=%d, want degrit clean tuning",
			cleanDegritEnc.profile,
			cleanDegritEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanDegritEnc.qualityGainDegritPreferenceEnabled(),
			cleanDegritEnc.qualityGainMSERepairThreshold())
	}

	cleanHarmonicEnc := NewEncoderWithProfile(EncoderProfileQualityCleanHarmonic)
	if cleanHarmonicEnc.profile != EncoderProfileQualityCleanHarmonic ||
		cleanHarmonicEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanHarmonicEnc.qualityGainHarmonicPreferenceEnabled() ||
		cleanHarmonicEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanHarmonic) profile=%v normPitch=%t harmonicPref=%t mseThreshold=%d, want harmonic clean tuning",
			cleanHarmonicEnc.profile,
			cleanHarmonicEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanHarmonicEnc.qualityGainHarmonicPreferenceEnabled(),
			cleanHarmonicEnc.qualityGainMSERepairThreshold())
	}

	cleanHarmonicStrongEnc := NewEncoderWithProfile(EncoderProfileQualityCleanHarmonicStrong)
	if cleanHarmonicStrongEnc.profile != EncoderProfileQualityCleanHarmonicStrong ||
		cleanHarmonicStrongEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanHarmonicStrongEnc.qualityGainHarmonicPreferenceEnabled() ||
		cleanHarmonicStrongEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanHarmonicStrong) profile=%v normPitch=%t harmonicPref=%t mseThreshold=%d, want strong harmonic clean tuning",
			cleanHarmonicStrongEnc.profile,
			cleanHarmonicStrongEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanHarmonicStrongEnc.qualityGainHarmonicPreferenceEnabled(),
			cleanHarmonicStrongEnc.qualityGainMSERepairThreshold())
	}
	_, minStep, gammaDrop, mseNum, mseDen, highNum, highDen := cleanHarmonicStrongEnc.qualityGainHarmonicPreferenceParams()
	if minStep != qualityCleanHarmonicStrongGainPitchMinStepQ14 ||
		gammaDrop != qualityCleanHarmonicStrongGammaDropMinQ13 ||
		mseNum != qualityCleanHarmonicStrongMSEToleranceNum ||
		mseDen != qualityCleanHarmonicStrongMSEToleranceDen ||
		highNum != qualityCleanHarmonicStrongHighMSEToleranceNum ||
		highDen != qualityCleanHarmonicStrongHighMSEToleranceDen {
		t.Fatalf("QualityCleanHarmonicStrong params minStep=%d gammaDrop=%d mse=%d/%d high=%d/%d, want strong harmonic params",
			minStep, gammaDrop, mseNum, mseDen, highNum, highDen)
	}

	cleanHarmonicDeepEnc := NewEncoderWithProfile(EncoderProfileQualityCleanHarmonicDeep)
	if cleanHarmonicDeepEnc.profile != EncoderProfileQualityCleanHarmonicDeep ||
		cleanHarmonicDeepEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanHarmonicDeepEnc.qualityGainHarmonicPreferenceEnabled() ||
		cleanHarmonicDeepEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanHarmonicDeep) profile=%v normPitch=%t harmonicPref=%t mseThreshold=%d, want deep harmonic clean tuning",
			cleanHarmonicDeepEnc.profile,
			cleanHarmonicDeepEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanHarmonicDeepEnc.qualityGainHarmonicPreferenceEnabled(),
			cleanHarmonicDeepEnc.qualityGainMSERepairThreshold())
	}
	_, minStep, gammaDrop, mseNum, mseDen, highNum, highDen = cleanHarmonicDeepEnc.qualityGainHarmonicPreferenceParams()
	if minStep != qualityCleanHarmonicDeepGainPitchMinStepQ14 ||
		gammaDrop != qualityCleanHarmonicDeepGammaDropMinQ13 ||
		mseNum != qualityCleanHarmonicDeepMSEToleranceNum ||
		mseDen != qualityCleanHarmonicDeepMSEToleranceDen ||
		highNum != qualityCleanHarmonicDeepHighMSEToleranceNum ||
		highDen != qualityCleanHarmonicDeepHighMSEToleranceDen {
		t.Fatalf("QualityCleanHarmonicDeep params minStep=%d gammaDrop=%d mse=%d/%d high=%d/%d, want deep harmonic params",
			minStep, gammaDrop, mseNum, mseDen, highNum, highDen)
	}

	cleanFCBEnc := NewEncoderWithProfile(EncoderProfileQualityCleanFCBRerank)
	if cleanFCBEnc.profile != EncoderProfileQualityCleanFCBRerank ||
		cleanFCBEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!cleanFCBEnc.qualityFCBNoiseRerankEnabled() ||
		cleanFCBEnc.qualityGainMSERepairThreshold() != qualityCleanGainMSERepairThreshold {
		t.Fatalf("NewEncoderWithProfile(QualityCleanFCBRerank) profile=%v normPitch=%t fcbRerank=%t mseThreshold=%d, want FCB-rerank clean tuning",
			cleanFCBEnc.profile,
			cleanFCBEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			cleanFCBEnc.qualityFCBNoiseRerankEnabled(),
			cleanFCBEnc.qualityGainMSERepairThreshold())
	}

	pesqEnc := NewEncoderWithProfile(EncoderProfileQualityPESQ)
	if pesqEnc.profile != EncoderProfileQualityPESQ ||
		!pesqEnc.qualityHeuristicsEnabled() ||
		pesqEnc.qualityExpandedLSPSearchEnabled() ||
		pesqEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!pesqEnc.qualityNativeGainSearchEnabled() ||
		!pesqEnc.qualityGainClipRepairEnabled() ||
		pesqEnc.qualityGainMSERepairEnabled() ||
		pesqEnc.qualityGainNoiseRepairEnabled() ||
		!pesqEnc.qualityFCBNoiseRerankEnabled() ||
		pesqEnc.coreFCBThresholdScanEnabled() {
		t.Fatalf("NewEncoderWithProfile(QualityPESQ) profile=%v quality=%t lspx=%t normPitch=%t nativeGain=%t gainClip=%t gainMSE=%t gainNoise=%t fcbRerank=%t coreFCB=%t, want focused PESQ candidate tuning",
			pesqEnc.profile,
			pesqEnc.qualityHeuristicsEnabled(),
			pesqEnc.qualityExpandedLSPSearchEnabled(),
			pesqEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			pesqEnc.qualityNativeGainSearchEnabled(),
			pesqEnc.qualityGainClipRepairEnabled(),
			pesqEnc.qualityGainMSERepairEnabled(),
			pesqEnc.qualityGainNoiseRepairEnabled(),
			pesqEnc.qualityFCBNoiseRerankEnabled(),
			pesqEnc.coreFCBThresholdScanEnabled())
	}

	pesqDegritEnc := NewEncoderWithProfile(EncoderProfileQualityPESQDegrit)
	if pesqDegritEnc.profile != EncoderProfileQualityPESQDegrit ||
		!pesqDegritEnc.qualityHeuristicsEnabled() ||
		pesqDegritEnc.qualityExpandedLSPSearchEnabled() ||
		pesqDegritEnc.qualityNormalizedAdaptivePitchSearchEnabled() ||
		!pesqDegritEnc.qualityNativeGainSearchEnabled() ||
		!pesqDegritEnc.qualityGainClipRepairEnabled() ||
		!pesqDegritEnc.qualityGainMSERepairEnabled() ||
		!pesqDegritEnc.qualityGainNoiseRepairEnabled() ||
		!pesqDegritEnc.qualityFCBNoiseRerankEnabled() ||
		pesqDegritEnc.coreFCBThresholdScanEnabled() {
		t.Fatalf("NewEncoderWithProfile(QualityPESQDegrit) profile=%v quality=%t lspx=%t normPitch=%t nativeGain=%t gainClip=%t gainMSE=%t gainNoise=%t fcbRerank=%t coreFCB=%t, want PESQ degrit tuning",
			pesqDegritEnc.profile,
			pesqDegritEnc.qualityHeuristicsEnabled(),
			pesqDegritEnc.qualityExpandedLSPSearchEnabled(),
			pesqDegritEnc.qualityNormalizedAdaptivePitchSearchEnabled(),
			pesqDegritEnc.qualityNativeGainSearchEnabled(),
			pesqDegritEnc.qualityGainClipRepairEnabled(),
			pesqDegritEnc.qualityGainMSERepairEnabled(),
			pesqDegritEnc.qualityGainNoiseRepairEnabled(),
			pesqDegritEnc.qualityFCBNoiseRerankEnabled(),
			pesqDegritEnc.coreFCBThresholdScanEnabled())
	}

	invalidEnc := NewEncoderWithProfile(EncoderProfile(99))
	if invalidEnc.profile != defaultEncoderProfile || invalidEnc.qualityHeuristicsEnabled() || !invalidEnc.coreFCBThresholdScanEnabled() {
		t.Fatalf("invalid profile normalized to %v quality=%t coreFCB=%t, want Core default profile",
			invalidEnc.profile,
			invalidEnc.qualityHeuristicsEnabled(),
			invalidEnc.coreFCBThresholdScanEnabled())
	}
}

func TestEncoderQualityFCBClipCooldown(t *testing.T) {
	e := NewEncoder()
	e.qualityTuning |= encoderTuningFCBThresholdScan
	pcm := make([]int16, FrameSamples)
	pcm[0] = int16(qualityFCBClipThreshold)
	var out [FrameBytes]byte

	if err := e.EncodeFrame(pcm, out[:]); err != nil {
		t.Fatalf("EncodeFrame near-clipped input: %v", err)
	}
	if e.qualityFCBClipCooldown != qualityFCBClipCooldownFrames-1 {
		t.Fatalf("qualityFCBClipCooldown = %d, want %d",
			e.qualityFCBClipCooldown, qualityFCBClipCooldownFrames-1)
	}
	if e.qualityFCBThresholdScanActive() {
		t.Fatal("quality FCB threshold scan active during near-clipped cooldown")
	}

	for i := 0; i < qualityFCBClipCooldownFrames-1; i++ {
		if err := e.EncodeFrame(make([]int16, FrameSamples), out[:]); err != nil {
			t.Fatalf("EncodeFrame cooldown frame %d: %v", i, err)
		}
	}
	if e.qualityFCBClipCooldown != 0 || !e.qualityFCBThresholdScanActive() {
		t.Fatalf("cooldown ended with cooldown=%d active=%t, want cooldown=0 active=true",
			e.qualityFCBClipCooldown, e.qualityFCBThresholdScanActive())
	}
}

func TestEncoderCoreFCBThresholdFrameBudget(t *testing.T) {
	e := NewEncoderWithProfile(EncoderProfileCore)
	e.coreFCBThresholdEntriesRemaining = fcbsearch.SearchThresholdScanDefaultLimit

	if got := e.coreFCBThresholdScanLimit(0); got != encoderCoreFCBThresholdScanSubframe0Limit {
		t.Fatalf("core subframe-0 FCB limit = %d, want %d", got, encoderCoreFCBThresholdScanSubframe0Limit)
	}

	e.recordCoreFCBThresholdEntries(37)
	if got := e.coreFCBThresholdScanLimit(1); got != fcbsearch.SearchThresholdScanDefaultLimit-37 {
		t.Fatalf("core subframe-1 FCB carryover limit = %d, want %d", got, fcbsearch.SearchThresholdScanDefaultLimit-37)
	}

	e.recordCoreFCBThresholdEntries(fcbsearch.SearchThresholdScanDefaultLimit)
	if got := e.coreFCBThresholdEntriesRemaining; got != 0 {
		t.Fatalf("core FCB remaining after overspend = %d, want 0", got)
	}
}

func TestEncoderAdaptiveVectorForSynthesisMatchesDecoder(t *testing.T) {
	e := NewEncoderWithProfile(EncoderProfileCore)
	for i := range e.oldExc {
		e.oldExc[i] = int16((i*97)%20000 - 10000)
	}
	var residual [closedloop.SubframeLen]int16
	for i := range residual {
		residual[i] = int16(25000 - i*300)
	}
	var exc [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&residual, &exc)

	for _, tc := range []struct {
		intLag int16
		frac   int8
	}{
		{intLag: 20, frac: 0},
		{intLag: 20, frac: -1},
		{intLag: 39, frac: +1},
		{intLag: 40, frac: -1},
		{intLag: 48, frac: +1},
		{intLag: 49, frac: -1},
		{intLag: 50, frac: +1},
		{intLag: 80, frac: -1},
	} {
		var got, want [closedloop.SubframeLen]int16
		e.adaptiveVectorForSynthesis(excSlice, tc.intLag, tc.frac, &got)
		pitchcore.AdaptiveCodebook(int(tc.intLag), int(tc.frac), e.oldExc[:], &want)
		if got != want {
			t.Fatalf("adaptiveVectorForSynthesis(%d,%d) diverged from decoder vector", tc.intLag, tc.frac)
		}
	}

	e.qualityTuning = encoderTuningResidualExtensionAdaptiveVector
	var got, residualVector [closedloop.SubframeLen]int16
	e.adaptiveVectorForSynthesis(excSlice, 40, -1, &got)
	closedloop.AdaptiveVector(excSlice, 40, -1, &residualVector)
	if got != residualVector {
		t.Fatal("residual-extension diagnostic vector did not use closed-loop residual extension")
	}
}

func TestEncoder_ResetPreservesProfile(t *testing.T) {
	e := NewEncoderWithProfile(EncoderProfileCore)
	e.Reset()
	if e.profile != EncoderProfileCore ||
		e.qualityHeuristicsEnabled() ||
		e.qualityNormalizedOpenLoopSearchEnabled() ||
		!e.coreFCBThresholdScanEnabled() {
		t.Fatalf("Reset profile = %v quality=%t normOpenLoop=%t coreFCB=%t, want core disabled with spec FCB threshold scan",
			e.profile,
			e.qualityHeuristicsEnabled(),
			e.qualityNormalizedOpenLoopSearchEnabled(),
			e.coreFCBThresholdScanEnabled())
	}

	annexALSP := NewEncoderWithProfile(EncoderProfileQualityAnnexALSP)
	annexALSP.Reset()
	if annexALSP.profile != EncoderProfileQualityAnnexALSP ||
		!annexALSP.qualityHeuristicsEnabled() ||
		annexALSP.qualityExpandedLSPSearchEnabled() ||
		annexALSP.qualityNormalizedOpenLoopSearchEnabled() ||
		!annexALSP.qualityNormalizedAdaptivePitchSearchEnabled() {
		t.Fatalf("Reset profile = %v quality=%t lspx=%t normOpenLoop=%t normPitch=%t, want focused QualityAnnexALSP",
			annexALSP.profile,
			annexALSP.qualityHeuristicsEnabled(),
			annexALSP.qualityExpandedLSPSearchEnabled(),
			annexALSP.qualityNormalizedOpenLoopSearchEnabled(),
			annexALSP.qualityNormalizedAdaptivePitchSearchEnabled())
	}
}

func TestEncoder_EncodeFrame_RejectsShortPCM(t *testing.T) {
	e := NewEncoder()
	var out [FrameBytes]byte
	if err := e.EncodeFrame(make([]int16, FrameSamples-1), out[:]); !errors.Is(err, ErrShortPCM) {
		t.Fatalf("got %v want ErrShortPCM", err)
	}
}

func TestEncoder_EncodeFrame_RejectsShortOutput(t *testing.T) {
	e := NewEncoder()
	pcm := make([]int16, FrameSamples)
	if err := e.EncodeFrame(pcm, make([]byte, FrameBytes-1)); !errors.Is(err, ErrShortOutput) {
		t.Fatalf("got %v want ErrShortOutput", err)
	}
}

func TestEncoder_Reset_ZeroValueIsSafe(t *testing.T) {
	var e Encoder
	e.Reset()
}

func TestEncoder_ResetRestoresFreshFrameOutput(t *testing.T) {
	pcm := makeRamp(FrameSamples)
	var got, want [FrameBytes]byte

	e := NewEncoder()
	if err := e.EncodeFrame(makeRamp(FrameSamples * 2)[:FrameSamples], got[:]); err != nil {
		t.Fatalf("warmup EncodeFrame: %v", err)
	}
	e.Reset()
	if err := e.EncodeFrame(pcm, got[:]); err != nil {
		t.Fatalf("EncodeFrame after Reset: %v", err)
	}

	fresh := NewEncoder()
	if err := fresh.EncodeFrame(pcm, want[:]); err != nil {
		t.Fatalf("fresh EncodeFrame: %v", err)
	}
	if got != want {
		t.Fatalf("Reset output differs from fresh encoder\n got=% x\nwant=% x", got, want)
	}
}

func TestEncoderGainCommitScaleHelpers(t *testing.T) {
	if got := applyGainQ14ToQ0(16384, 1000); got != 1000 {
		t.Fatalf("applyGainQ14ToQ0 unity = %d, want 1000", got)
	}
	if got := applyGcToQ12(16384, 0, 4096); got != 1 {
		t.Fatalf("applyGcToQ12 gc=1 z=1 = %d, want 1", got)
	}
	if got := applyGcToQ12(16384, 12, 4096); got != 4096 {
		t.Fatalf("applyGcToQ12 gc=4096 z=1 = %d, want 4096", got)
	}
}

func TestEncoderGainCommitCodeScaleMatchesDecoderExcitation(t *testing.T) {
	var h [closedloop.SubframeLen]int16
	h[0] = 4096 // identity weighted-synthesis impulse response, Q12.

	var c [closedloop.SubframeLen]int16
	for _, pulse := range []struct {
		pos int
		amp int16
	}{
		{0, 8192},
		{6, -8192},
		{12, 4096},
		{18, -4096},
		{24, 6144},
		{30, -3072},
	} {
		c[pulse.pos] = pulse.amp
	}

	var z [closedloop.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	tests := []struct {
		name string
		mant int16
		exp  int8
	}{
		{name: "unity", mant: 16384, exp: 0},
		{name: "fractional", mant: 8192, exp: 0},
		{name: "large positive exponent", mant: 20000, exp: 5},
		{name: "small negative exponent", mant: 20000, exp: -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v, u [closedloop.SubframeLen]int16
			synth.BuildExcitation(0, tt.mant, tt.exp, &v, &c, &u)

			for n := 0; n < closedloop.SubframeLen; n++ {
				got := applyGcToQ12(tt.mant, tt.exp, z[n])
				want := int32(u[n])
				if got != want {
					t.Fatalf("n=%d applyGcToQ12(mant=%d,exp=%d,z=%d) = %d, want BuildExcitation code contribution %d (c=%d)",
						n, tt.mant, tt.exp, z[n], got, want, c[n])
				}
			}
		})
	}
}

func TestLowerPitchCenterRescueCandidate(t *testing.T) {
	tests := []struct {
		name      string
		centre    int
		candidate int
		want      bool
	}{
		{name: "large lower-lag drop", centre: 75, candidate: 41, want: true},
		{name: "boundary drop", centre: 60, candidate: 40, want: true},
		{name: "small local adjustment", centre: 60, candidate: 41, want: false},
		{name: "higher candidate", centre: 41, candidate: 75, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLowerPitchCenterRescueCandidate(tt.centre, tt.candidate); got != tt.want {
				t.Fatalf("isLowerPitchCenterRescueCandidate(%d, %d) = %t, want %t",
					tt.centre, tt.candidate, got, tt.want)
			}
		})
	}
}

func TestQualityPitchClipSubframe1CandidateRepresentable(t *testing.T) {
	tests := []struct {
		name string
		lag  int16
		frac int8
		want bool
	}{
		{name: "below lower edge", lag: 18, frac: 1, want: false},
		{name: "lower edge minus frac", lag: 19, frac: -1, want: false},
		{name: "lower edge zero frac", lag: 19, frac: 0, want: false},
		{name: "lower edge plus frac", lag: 19, frac: 1, want: true},
		{name: "fractional interior negative", lag: 20, frac: -1, want: true},
		{name: "fractional interior zero", lag: 50, frac: 0, want: true},
		{name: "fractional interior positive", lag: 84, frac: 1, want: true},
		{name: "upper fractional edge negative", lag: 85, frac: -1, want: true},
		{name: "upper fractional edge zero", lag: 85, frac: 0, want: true},
		{name: "upper fractional edge positive", lag: 85, frac: 1, want: false},
		{name: "integer region negative frac", lag: 86, frac: -1, want: false},
		{name: "integer region zero frac", lag: 143, frac: 0, want: true},
		{name: "above upper edge", lag: 144, frac: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qualityPitchClipSubframe1CandidateRepresentable(tt.lag, tt.frac)
			if got != tt.want {
				t.Fatalf("qualityPitchClipSubframe1CandidateRepresentable(%d, %d) = %t, want %t",
					tt.lag, tt.frac, got, tt.want)
			}
		})
	}
}

func TestQualityPitchClipCandidatesSubframe1SkipsP1Wraparound(t *testing.T) {
	var out [18]encoderPitchClipCandidate
	count := qualityPitchClipCandidates(0, 0, 20, 0, &out)
	if count == 0 {
		t.Fatal("qualityPitchClipCandidates returned no candidates")
	}

	foundLowerEdge := false
	for i := 0; i < count; i++ {
		cand := out[i]
		if cand.intLag == 19 && cand.frac == 1 {
			foundLowerEdge = true
		}
		if cand.intLag >= 142 {
			t.Fatalf("candidate[%d] = (%d,%d), unrepresentable lower-edge P1 wrapped to a far lag",
				i, cand.intLag, cand.frac)
		}
		if cand.intLag < 19 || cand.intLag > 22 {
			t.Fatalf("candidate[%d] = (%d,%d), want local candidate around lag 20",
				i, cand.intLag, cand.frac)
		}
	}
	if !foundLowerEdge {
		t.Fatal("lower-edge representable candidate (19,+1) was not included")
	}
}

func TestClosedLoopExcitationSearchIncludesB30Prehistory(t *testing.T) {
	var e Encoder
	for i := range e.oldExc {
		e.oldExc[i] = int16(i + 1)
	}
	var residual [closedloop.SubframeLen]int16
	for i := range residual {
		residual[i] = int16(1000 + i)
	}

	var exc [closedLoopPitchSearchLen]int16
	out := e.closedLoopExcitationSearch(&residual, &exc)
	if len(out) != closedLoopPitchSearchLen {
		t.Fatalf("closed-loop search len = %d, want %d", len(out), closedLoopPitchSearchLen)
	}

	anchor := len(out) - closedloop.SubframeLen
	if anchor != closedLoopPitchSearchHistory {
		t.Fatalf("anchor = %d, want %d", anchor, closedLoopPitchSearchHistory)
	}
	if got, want := out[0], e.oldExc[len(e.oldExc)-closedLoopPitchSearchHistory]; got != want {
		t.Fatalf("oldest b30 prehistory sample = %d, want %d", got, want)
	}
	if got, want := out[anchor-closedloop.PitchMaxInt], e.oldExc[len(e.oldExc)-closedloop.PitchMaxInt]; got != want {
		t.Fatalf("u(-143) sample = %d, want %d", got, want)
	}
	for i := range residual {
		if out[anchor+i] != residual[i] {
			t.Fatalf("residual extension[%d] = %d, want %d", i, out[anchor+i], residual[i])
		}
	}
}

func TestEncoderOpenLoopStepUsesQuantizedSubframeLP(t *testing.T) {
	enc := NewEncoderWithProfile(EncoderProfileCore)
	enc.aQ12Latest = [lpc.LPCOrder + 1]int16{
		4096,
		2900, -2100, 1500, -1050, 730,
		-510, 350, -240, 160, -90,
	}
	enc.aHatSF1 = [lpc.LPCOrder + 1]int16{
		4096,
		-720, 460, -310, 210, -140,
		95, -60, 35, -18, 8,
	}
	enc.aHatSF2 = [lpc.LPCOrder + 1]int16{
		4096,
		-980, 620, -390, 240, -150,
		85, -48, 24, -11, 4,
	}
	for i := 0; i < FrameSamples; i++ {
		enc.oldSpeech[120+i] = int16(((i*197 + 53) % 9000) - 4500)
	}
	for i := range enc.lpResidualMem {
		enc.lpResidualMem[i] = int16(i*17 - 80)
		enc.swMem[i] = int16(60 - i*13)
	}
	for i := range enc.oldWspeech {
		enc.oldWspeech[i] = int16(((i*71 + 19) % 3000) - 1500)
	}

	s := (*[FrameSamples]int16)(enc.oldSpeech[120:200])
	residualMem := enc.lpResidualMem
	swMem := enc.swMem
	oldWspeech := enc.oldWspeech
	want := openloop.StepSplitSearch(&enc.aHatSF1, &enc.aHatSF2, s, &residualMem, &swMem, &oldWspeech)

	got := enc.openloopStep()
	if got != want.Top || enc.tOp != want.Top {
		t.Fatalf("openloopStep Top=%d tOp=%d, want quantized StepSplit Top=%d",
			got, enc.tOp, want.Top)
	}
	if enc.lpResidualMem != residualMem {
		t.Fatal("openloopStep lpResidualMem diverged from quantized StepSplit state")
	}
	if enc.swMem != swMem {
		t.Fatal("openloopStep swMem diverged from quantized StepSplit state")
	}
	if enc.oldWspeech != oldWspeech {
		t.Fatal("openloopStep oldWspeech diverged from quantized StepSplit state")
	}
}

func TestClosedLoopStepCommitsStatePerSubframe(t *testing.T) {
	enc := NewEncoderWithProfile(EncoderProfileCore)

	var pcm [FrameSamples]int16
	drivePeriodicFrame(&pcm)

	for f := 0; f < 4; f++ {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("warmup lpcStep frame %d: %v", f, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}

	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep: %v", err)
	}
	_ = enc.openloopStep()

	oldExcBefore0 := enc.oldExc
	pastBefore0 := enc.pastQuaEn
	swBefore0 := enc.swMemErr
	s0 := (*[closedloop.SubframeLen]int16)(enc.oldSpeech[120:160])
	_, _ = enc.closedloopStep(0)

	for i := 0; i < lpc.LPCOrder; i++ {
		if enc.lpResidualMemQ[i] != s0[30+i] {
			t.Fatalf("subframe-0 lpResidualMemQ[%d] = %d, want speech tail %d",
				i, enc.lpResidualMemQ[i], s0[30+i])
		}
	}
	assertOldExcShifted(t, "subframe-0", oldExcBefore0, enc.oldExc)
	assertPastQuaEnShifted(t, "subframe-0", pastBefore0, enc.pastQuaEn)
	if enc.swMemErr == swBefore0 {
		t.Fatal("subframe-0 swMemErr did not change; A.10 weighted-error commit did not visibly run")
	}

	oldExcBefore1 := enc.oldExc
	pastBefore1 := enc.pastQuaEn
	swBefore1 := enc.swMemErr
	s1 := (*[closedloop.SubframeLen]int16)(enc.oldSpeech[160:200])
	_, _ = enc.closedloopStep(1)

	for i := 0; i < lpc.LPCOrder; i++ {
		if enc.lpResidualMemQ[i] != s1[30+i] {
			t.Fatalf("subframe-1 lpResidualMemQ[%d] = %d, want speech tail %d",
				i, enc.lpResidualMemQ[i], s1[30+i])
		}
	}
	assertOldExcShifted(t, "subframe-1", oldExcBefore1, enc.oldExc)
	assertPastQuaEnShifted(t, "subframe-1", pastBefore1, enc.pastQuaEn)
	if enc.swMemErr == swBefore1 {
		t.Fatal("subframe-1 swMemErr did not change; A.10 weighted-error commit did not visibly run")
	}
}

func assertOldExcShifted(t *testing.T, label string, before, after [154]int16) {
	t.Helper()
	const n = closedloop.SubframeLen
	for i := 0; i < len(before)-n; i++ {
		if after[i] != before[i+n] {
			t.Fatalf("%s oldExc shift mismatch at %d: got %d want prior[%d]=%d",
				label, i, after[i], i+n, before[i+n])
		}
	}
	tailChanged := false
	for i := len(after) - n; i < len(after); i++ {
		if after[i] != before[i] {
			tailChanged = true
			break
		}
	}
	if !tailChanged {
		t.Fatalf("%s oldExc tail did not change after A.9 excitation commit", label)
	}
}

func assertPastQuaEnShifted(t *testing.T, label string, before, after [4]int16) {
	t.Helper()
	for i := 1; i < len(after); i++ {
		if after[i] != before[i-1] {
			t.Fatalf("%s pastQuaEn[%d] = %d, want prior[%d]=%d",
				label, i, after[i], i-1, before[i-1])
		}
	}
}

func TestWeightedErrorCommitIdentityFilterMatchesExcitation(t *testing.T) {
	aHat := [11]int16{4096}
	var h [closedloop.SubframeLen]int16
	closedloop.ImpulseResponse(&aHat, &h)

	if h[0] != 4096 {
		t.Fatalf("identity h[0] = %d, want 4096", h[0])
	}
	for n := 1; n < closedloop.SubframeLen; n++ {
		if h[n] != 0 {
			t.Fatalf("identity h[%d] = %d, want 0", n, h[n])
		}
	}

	var r, v, c [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		r[n] = int16((n*113)%5000 - 2500)
		v[n] = int16((n*37)%1800 - 900)
	}
	c[3] = 8192
	c[17] = -8192
	c[29] = 8192
	c[37] = -8192

	var x, y, z, u [closedloop.SubframeLen]int16
	var zeroMem [10]int16
	closedloop.TargetSignal(&aHat, &r, &zeroMem, &x)
	closedloop.GpAndY(&x, &v, &h, &y)
	fcbsearch.FilterCode(&c, &h, &z)

	const gpQ14 int16 = 8192 // 0.5
	const gcMantQ14 int16 = 16384
	const gcExp int8 = 0 // 1.0
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var rMinusU, filteredError [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		rMinusU[n] = fixed.Saturate(int32(r[n]) - int32(u[n]))
	}
	closedloop.TargetSignal(&aHat, &rMinusU, &zeroMem, &filteredError)

	for n := 30; n < closedloop.SubframeLen; n++ {
		direct := fixed.Saturate(int32(x[n]) - applyGainQ14ToQ0(gpQ14, y[n]) - applyGcToQ12(gcMantQ14, gcExp, z[n]))
		if direct != filteredError[n] {
			t.Fatalf("n=%d direct weighted error=%d, filtered r-u=%d (x=%d y=%d z=%d u=%d)",
				n, direct, filteredError[n], x[n], y[n], z[n], u[n])
		}
	}
}

func TestWeightedErrorCommitAllPoleTailEquivalence(t *testing.T) {
	aHat := [11]int16{
		4096,
		-1510, 986, -671, 394, -205,
		103, -69, 41, -25, 12,
	}
	var h [closedloop.SubframeLen]int16
	closedloop.ImpulseResponse(&aHat, &h)

	var r, v, c [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		r[n] = int16((n*197)%7000 - 3500)
		v[n] = int16((n*89)%2600 - 1300)
	}
	for _, pulse := range []struct {
		pos int
		amp int16
	}{
		{0, 8192},
		{9, -8192},
		{21, 8192},
		{34, -8192},
	} {
		c[pulse.pos] = pulse.amp
	}

	var x, y, z, u [closedloop.SubframeLen]int16
	var zeroMem [10]int16
	closedloop.TargetSignal(&aHat, &r, &zeroMem, &x)
	closedloop.GpAndY(&x, &v, &h, &y)
	fcbsearch.FilterCode(&c, &h, &z)

	const gpQ14 int16 = 9830
	const gcMantQ14 int16 = 14746
	const gcExp int8 = 1
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var rMinusU, filteredError [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		rMinusU[n] = fixed.Saturate(int32(r[n]) - int32(u[n]))
	}
	closedloop.TargetSignal(&aHat, &rMinusU, &zeroMem, &filteredError)

	for n := 30; n < closedloop.SubframeLen; n++ {
		direct := fixed.Saturate(int32(x[n]) - applyGainQ14ToQ0(gpQ14, y[n]) - applyGcToQ12(gcMantQ14, gcExp, z[n]))
		delta := int32(direct) - int32(filteredError[n])
		if delta < 0 {
			delta = -delta
		}
		if delta > 1 {
			t.Fatalf("n=%d direct weighted error=%d, filtered r-u=%d, delta=%d; want <= 1 LSB",
				n, direct, filteredError[n], delta)
		}
	}
}

func TestWeightedErrorCommitAllPoleEquivalenceDiagnostic(t *testing.T) {
	if os.Getenv("G729_WEIGHTED_ERROR_EQUIV_DIAG") != "1" {
		t.Skip("set G729_WEIGHTED_ERROR_EQUIV_DIAG=1 to run weighted-error equivalence diagnostic")
	}

	aHat := [11]int16{
		4096,
		-1510, 986, -671, 394, -205,
		103, -69, 41, -25, 12,
	}
	var h [closedloop.SubframeLen]int16
	closedloop.ImpulseResponse(&aHat, &h)

	var r, v, c [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		r[n] = int16((n*197)%7000 - 3500)
		v[n] = int16((n*89)%2600 - 1300)
	}
	for _, pulse := range []struct {
		pos int
		amp int16
	}{
		{0, 8192},
		{9, -8192},
		{21, 8192},
		{34, -8192},
	} {
		c[pulse.pos] = pulse.amp
	}

	var x, y, z, u [closedloop.SubframeLen]int16
	var zeroMem [10]int16
	closedloop.TargetSignal(&aHat, &r, &zeroMem, &x)
	closedloop.GpAndY(&x, &v, &h, &y)
	fcbsearch.FilterCode(&c, &h, &z)

	const gpQ14 int16 = 9830 // about 0.6
	const gcMantQ14 int16 = 14746
	const gcExp int8 = 1 // about 1.8
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var rMinusU, filteredError [closedloop.SubframeLen]int16
	for n := 0; n < closedloop.SubframeLen; n++ {
		rMinusU[n] = fixed.Saturate(int32(r[n]) - int32(u[n]))
	}
	closedloop.TargetSignal(&aHat, &rMinusU, &zeroMem, &filteredError)

	var maxAbs int32
	var sumAbs int64
	var maxN int
	for n := 0; n < closedloop.SubframeLen; n++ {
		direct := fixed.Saturate(int32(x[n]) - applyGainQ14ToQ0(gpQ14, y[n]) - applyGcToQ12(gcMantQ14, gcExp, z[n]))
		delta := int32(direct) - int32(filteredError[n])
		if delta < 0 {
			delta = -delta
		}
		sumAbs += int64(delta)
		if delta > maxAbs {
			maxAbs = delta
			maxN = n
		}
		if n >= 30 {
			t.Logf("tail n=%02d direct=%d filtered=%d delta=%d x=%d y=%d z=%d u=%d",
				n, direct, filteredError[n], delta, x[n], y[n], z[n], u[n])
		}
	}
	t.Logf("weighted-error all-pole diagnostic: maxAbs=%d at n=%d meanAbs=%.2f",
		maxAbs, maxN, float64(sumAbs)/closedloop.SubframeLen)
}

func TestGainSearchSurfaceScaleHelpers(t *testing.T) {
	target := [FrameSamples / 2]int16{-3, -2, -1, 0, 1, 2, 3, 32767, -32768}
	scaleGainSearchVector(&target, 1, 2)
	wantTarget := [...]int16{-2, -1, -1, 0, 1, 1, 2, 16384, -16384}
	for i, want := range wantTarget {
		if target[i] != want {
			t.Fatalf("scaleGainSearchVector(target half)[%d] = %d, want %d", i, target[i], want)
		}
	}

	adaptive := [FrameSamples / 2]int16{-20000, -2, -1, 0, 1, 2, 20000}
	scaleGainSearchVector(&adaptive, 7, 2)
	wantAdaptive := [...]int16{-32768, -7, -4, 0, 4, 7, 32767}
	for i, want := range wantAdaptive {
		if adaptive[i] != want {
			t.Fatalf("scaleGainSearchVector(adaptive 7/2)[%d] = %d, want %d", i, adaptive[i], want)
		}
	}
}
