package decoder

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/pitch"
)

// TestPhase3rAsteriskFFmpegQualityGate compares this decoder with FFmpeg
// executable black-box decode on an external Asterisk-origin raw G.729 payload.
//
// This is an opt-in product-quality gate for inbound/local decode. It does not
// inspect FFmpeg or other implementation source. Without the REQUIRE env var it
// is informational; with REQUIRE it fails until the local decoder is close
// enough to the black-box decode for the same payload.
func TestPhase3rAsteriskFFmpegQualityGate(t *testing.T) {
	requireStrict := os.Getenv("G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY") == "1"
	requireEnhanced := os.Getenv("G729_REQUIRE_ENHANCED_DECODER_ASTERISK_FFMPEG_QUALITY") == "1"
	if os.Getenv("G729_DECODER_ASTERISK_FFMPEG_QUALITY") != "1" && !requireStrict && !requireEnhanced {
		t.Skip("set G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 to run Asterisk local-vs-ffmpeg quality gate")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		if requireStrict || requireEnhanced {
			t.Fatalf("ffmpeg required for Asterisk quality gate: %v", err)
		}
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(rawPath); err != nil {
		if requireStrict || requireEnhanced {
			t.Fatalf("required Asterisk payload unavailable at %s: %v", rawPath, err)
		}
		t.Skipf("Asterisk payload unavailable: %v", err)
	}

	local, frames := blackboxDecodeRawG729(t, rawPath)
	enhanced := phase3rDecodeRawEnhanced(t, rawPath, frames)
	enhancedEC25 := phase3rDecodeRawEnhancedCorrections(t, rawPath, frames, 25, 13)
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read Asterisk raw payload for variants: %v", err)
	}
	fixedHalf := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1})
	pitchHalf := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "pitch_gain_half", pitchScaleNum: 1, pitchScaleDen: 2})
	pitchThreeQuarter := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "pitch_gain_three_quarter", pitchScaleNum: 3, pitchScaleDen: 4})
	pitchFiveQuarter := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "pitch_gain_five_quarter", pitchScaleNum: 5, pitchScaleDen: 4})
	noFCBEnhancement := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true})
	gainEC25 := phase3qDecodeRawGainVariant(t, raw, frames, phase3jVariant{name: "gain_ec_q25", mode: phase3jGainECQ25})
	gainGamma14 := phase3qDecodeRawGainVariant(t, raw, frames, phase3jVariant{name: "gain_gamma_q14", mode: phase3jGainGammaQ14})
	tmp := t.TempDir()
	ffPath := filepath.Join(tmp, "asterisk.ffmpeg.s16le")
	ffmpegDecodeRawForEnvelopeAudit(t, rawPath, ffPath)
	ffmpeg := readPCM16LEForEnvelopeAudit(t, ffPath)
	totalSamples := frames * frameSamples
	if len(ffmpeg) > totalSamples {
		ffmpeg = ffmpeg[:totalSamples]
	}
	if len(ffmpeg) < totalSamples {
		t.Fatalf("ffmpeg output too short: got %d samples want >= %d", len(ffmpeg), totalSamples)
	}

	m := blackboxMeasure(ffmpeg, local, 40)
	env := phase3pEnvelopeCompare(ffmpeg, local)
	enhancedM := blackboxMeasure(ffmpeg, enhanced, 40)
	enhancedEnv := phase3pEnvelopeCompare(ffmpeg, enhanced)
	enhancedEC25M := blackboxMeasure(ffmpeg, enhancedEC25, 40)
	enhancedEC25Env := phase3pEnvelopeCompare(ffmpeg, enhancedEC25)
	fixedHalfM := blackboxMeasure(ffmpeg, fixedHalf, 40)
	fixedHalfEnv := phase3pEnvelopeCompare(ffmpeg, fixedHalf)
	pitchHalfM := blackboxMeasure(ffmpeg, pitchHalf, 40)
	pitchThreeQuarterM := blackboxMeasure(ffmpeg, pitchThreeQuarter, 40)
	pitchFiveQuarterM := blackboxMeasure(ffmpeg, pitchFiveQuarter, 40)
	noFCBEnhancementM := blackboxMeasure(ffmpeg, noFCBEnhancement, 40)
	gainEC25M := blackboxMeasure(ffmpeg, gainEC25, 40)
	gainEC25Env := phase3pEnvelopeCompare(ffmpeg, gainEC25)
	gainGamma14M := blackboxMeasure(ffmpeg, gainGamma14, 40)
	gainGamma14Env := phase3pEnvelopeCompare(ffmpeg, gainGamma14)
	localOracle := phase3rMatchFrameRMS(local, ffmpeg)
	localOracleM := blackboxMeasure(ffmpeg, localOracle, 40)
	enhancedOracle := phase3rMatchFrameRMS(enhanced, ffmpeg)
	enhancedOracleM := blackboxMeasure(ffmpeg, enhancedOracle, 40)
	fixedHalfOracle := phase3rMatchFrameRMS(fixedHalf, ffmpeg)
	fixedHalfOracleM := blackboxMeasure(ffmpeg, fixedHalfOracle, 40)
	bestHybrid := phase3rBestOfTwoByFrameSNR(ffmpeg, local, enhanced)
	bestHybridM := blackboxMeasure(ffmpeg, bestHybrid, 40)
	bestGainOracle := phase3rBestOfManyByFrameSNR(ffmpeg, local, enhanced, fixedHalf, gainEC25, gainGamma14)
	bestGainOracleM := blackboxMeasure(ffmpeg, bestGainOracle, 40)
	bestContributionOracle := phase3rBestOfManyByFrameSNR(ffmpeg, local, enhanced, fixedHalf, gainEC25, pitchHalf, pitchThreeQuarter, pitchFiveQuarter, noFCBEnhancement)
	bestContributionOracleM := blackboxMeasure(ffmpeg, bestContributionOracle, 40)
	ffStats := blackboxSignalStats(ffmpeg, frames)
	localStats := blackboxSignalStats(local, frames)
	enhancedStats := blackboxSignalStats(enhanced, frames)
	enhancedEC25Stats := blackboxSignalStats(enhancedEC25, frames)

	rmsRatio := 0.0
	if ffStats.rms > 0 {
		rmsRatio = localStats.rms / ffStats.rms
	}
	enhancedRMSRatio := 0.0
	if ffStats.rms > 0 {
		enhancedRMSRatio = enhancedStats.rms / ffStats.rms
	}
	enhancedEC25RMSRatio := 0.0
	if ffStats.rms > 0 {
		enhancedEC25RMSRatio = enhancedEC25Stats.rms / ffStats.rms
	}
	lowCorrPct := 0.0
	if env.activeFrames > 0 {
		lowCorrPct = float64(env.lowCorrFrames) / float64(env.activeFrames)
	}
	enhancedLowCorrPct := 0.0
	if enhancedEnv.activeFrames > 0 {
		enhancedLowCorrPct = float64(enhancedEnv.lowCorrFrames) / float64(enhancedEnv.activeFrames)
	}

	t.Logf("Phase 3r Asterisk local decoder vs FFmpeg black-box quality gate")
	t.Logf("payload=%s frames=%d samples=%d", rawPath, frames, totalSamples)
	t.Logf("ffmpeg: rms=%.2f peak=%d clipped=%d silenceFrames=%d lowEnergyFrames=%d",
		ffStats.rms, ffStats.peak, ffStats.clipped, ffStats.silenceFrames, ffStats.lowEnergyFrames)
	t.Logf("local:  rms=%.2f peak=%d clipped=%d silenceFrames=%d lowEnergyFrames=%d rmsRatio=%.3f",
		localStats.rms, localStats.peak, localStats.clipped, localStats.silenceFrames, localStats.lowEnergyFrames, rmsRatio)
	t.Logf("local-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		m.globalSNR, m.segSNR, m.corr, m.bestSNRLag, m.bestSNR,
		env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
	t.Logf("enhanced: rms=%.2f peak=%d clipped=%d silenceFrames=%d lowEnergyFrames=%d rmsRatio=%.3f",
		enhancedStats.rms, enhancedStats.peak, enhancedStats.clipped, enhancedStats.silenceFrames, enhancedStats.lowEnergyFrames, enhancedRMSRatio)
	t.Logf("enhanced-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		enhancedM.globalSNR, enhancedM.segSNR, enhancedM.corr, enhancedM.bestSNRLag, enhancedM.bestSNR,
		enhancedEnv.ratioMedian, enhancedEnv.lowRatioFrames, enhancedEnv.lowCorrFrames)
	t.Logf("enhanced_ec25: rms=%.2f peak=%d clipped=%d silenceFrames=%d lowEnergyFrames=%d rmsRatio=%.3f",
		enhancedEC25Stats.rms, enhancedEC25Stats.peak, enhancedEC25Stats.clipped, enhancedEC25Stats.silenceFrames, enhancedEC25Stats.lowEnergyFrames, enhancedEC25RMSRatio)
	t.Logf("enhanced_ec25-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		enhancedEC25M.globalSNR, enhancedEC25M.segSNR, enhancedEC25M.corr, enhancedEC25M.bestSNRLag, enhancedEC25M.bestSNR,
		enhancedEC25Env.ratioMedian, enhancedEC25Env.lowRatioFrames, enhancedEC25Env.lowCorrFrames)
	t.Logf("fixed_gain_half-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		fixedHalfM.globalSNR, fixedHalfM.segSNR, fixedHalfM.corr, fixedHalfM.bestSNRLag, fixedHalfM.bestSNR,
		fixedHalfEnv.ratioMedian, fixedHalfEnv.lowRatioFrames, fixedHalfEnv.lowCorrFrames)
	t.Logf("pitch contribution variants: half gSNR=%.2f seg=%.2f corr=%.3f ; three_quarter gSNR=%.2f seg=%.2f corr=%.3f ; five_quarter gSNR=%.2f seg=%.2f corr=%.3f ; no_fcb_enh gSNR=%.2f seg=%.2f corr=%.3f",
		pitchHalfM.globalSNR, pitchHalfM.segSNR, pitchHalfM.corr,
		pitchThreeQuarterM.globalSNR, pitchThreeQuarterM.segSNR, pitchThreeQuarterM.corr,
		pitchFiveQuarterM.globalSNR, pitchFiveQuarterM.segSNR, pitchFiveQuarterM.corr,
		noFCBEnhancementM.globalSNR, noFCBEnhancementM.segSNR, noFCBEnhancementM.corr)
	t.Logf("gain_ec_q25-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		gainEC25M.globalSNR, gainEC25M.segSNR, gainEC25M.corr, gainEC25M.bestSNRLag, gainEC25M.bestSNR,
		gainEC25Env.ratioMedian, gainEC25Env.lowRatioFrames, gainEC25Env.lowCorrFrames)
	t.Logf("gain_gamma_q14-vs-ffmpeg: gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f ratioMed=%.3f low<0.5=%d corr<0.3=%d",
		gainGamma14M.globalSNR, gainGamma14M.segSNR, gainGamma14M.corr, gainGamma14M.bestSNRLag, gainGamma14M.bestSNR,
		gainGamma14Env.ratioMedian, gainGamma14Env.lowRatioFrames, gainGamma14Env.lowCorrFrames)
	t.Logf("frame-RMS oracle bound: default gSNR=%.2f seg=%.2f ; enhanced gSNR=%.2f seg=%.2f ; fixed_half gSNR=%.2f seg=%.2f",
		localOracleM.globalSNR, localOracleM.segSNR, enhancedOracleM.globalSNR, enhancedOracleM.segSNR,
		fixedHalfOracleM.globalSNR, fixedHalfOracleM.segSNR)
	t.Logf("best-of-default/enhanced oracle bound: gSNR=%.2f seg=%.2f",
		bestHybridM.globalSNR, bestHybridM.segSNR)
	t.Logf("best-of-default/enhanced/fixed_half/ec25/gamma14 oracle bound: gSNR=%.2f seg=%.2f",
		bestGainOracleM.globalSNR, bestGainOracleM.segSNR)
	t.Logf("best-of-contribution-modes oracle bound: gSNR=%.2f seg=%.2f",
		bestContributionOracleM.globalSNR, bestContributionOracleM.segSNR)
	defaultLagOracle := phase3rFrameLagOracle(ffmpeg, local, 20)
	defaultLagOracleM := blackboxMeasure(ffmpeg, defaultLagOracle, 40)
	enhancedLagOracle := phase3rFrameLagOracle(ffmpeg, enhanced, 20)
	enhancedLagOracleM := blackboxMeasure(ffmpeg, enhancedLagOracle, 40)
	fixedHalfLagOracle := phase3rFrameLagOracle(ffmpeg, fixedHalf, 20)
	fixedHalfLagOracleM := blackboxMeasure(ffmpeg, fixedHalfLagOracle, 40)
	t.Logf("per-frame lag oracle bound: default gSNR=%.2f seg=%.2f ; enhanced gSNR=%.2f seg=%.2f ; fixed_half gSNR=%.2f seg=%.2f",
		defaultLagOracleM.globalSNR, defaultLagOracleM.segSNR, enhancedLagOracleM.globalSNR, enhancedLagOracleM.segSNR,
		fixedHalfLagOracleM.globalSNR, fixedHalfLagOracleM.segSNR)
	defaultLagRMSOracle := phase3rFrameLagRMSOracle(ffmpeg, local, 20)
	defaultLagRMSOracleM := blackboxMeasure(ffmpeg, defaultLagRMSOracle, 40)
	enhancedLagRMSOracle := phase3rFrameLagRMSOracle(ffmpeg, enhanced, 20)
	enhancedLagRMSOracleM := blackboxMeasure(ffmpeg, enhancedLagRMSOracle, 40)
	fixedHalfLagRMSOracle := phase3rFrameLagRMSOracle(ffmpeg, fixedHalf, 20)
	fixedHalfLagRMSOracleM := blackboxMeasure(ffmpeg, fixedHalfLagRMSOracle, 40)
	t.Logf("per-frame lag+RMS oracle bound: default gSNR=%.2f seg=%.2f ; enhanced gSNR=%.2f seg=%.2f ; fixed_half gSNR=%.2f seg=%.2f",
		defaultLagRMSOracleM.globalSNR, defaultLagRMSOracleM.segSNR, enhancedLagRMSOracleM.globalSNR, enhancedLagRMSOracleM.segSNR,
		fixedHalfLagRMSOracleM.globalSNR, fixedHalfLagRMSOracleM.segSNR)
	enhancedLags := phase3rBestFrameLags(ffmpeg, enhanced, 20)
	phase3rLogLagHistogram(t, "enhanced oracle lags", enhancedLags)
	phase3rLogLagFeatureSummary(t, rawPath, ffmpeg, enhanced, enhancedLags)
	continuityLagged := phase3rContinuityLagRecover(enhanced, 20)
	continuityLaggedM := blackboxMeasure(ffmpeg, continuityLagged, 40)
	t.Logf("runtime continuity lag candidate: gSNR=%.2f seg=%.2f corr=%.3f",
		continuityLaggedM.globalSNR, continuityLaggedM.segSNR, continuityLaggedM.corr)
	phase3rLogRuntimeLagRuleGrid(t, raw, ffmpeg, enhanced)
	phase3rLogTemporalBlendGrid(t, ffmpeg, enhanced)
	phase3rLogRuntimeLagFeatureGrid(t, raw, ffmpeg, local, enhanced)
	phase3rLogRuntimeLagPredicateSearch(t, raw, ffmpeg, local, enhanced)
	phase3rLogGainModeSelectorGrid(t, ffmpeg, enhanced, gainEC25, fixedHalf)
	phase3rLogGainSaturationSelectorGrid(t, raw, ffmpeg, enhanced, fixedHalf)
	phase3rLogOverhangDampingGrid(t, raw, ffmpeg, enhanced)
	phase3rLogActiveDampingGrid(t, raw, ffmpeg, enhanced)
	phase3rLogContributionSelectorGrid(t, raw, ffmpeg, enhanced, pitchHalf, pitchThreeQuarter, noFCBEnhancement)
	phase3rLogContributionOracleSummary(t, raw, ffmpeg, enhanced, fixedHalf, gainEC25, pitchHalf, pitchThreeQuarter, pitchFiveQuarter, noFCBEnhancement)
	phase3rLogContributionPredicateSearch(t, raw, ffmpeg, enhanced, fixedHalf, gainEC25, pitchHalf, pitchThreeQuarter, noFCBEnhancement)
	phase3rLogGainPairSelectorSearch(t, raw, ffmpeg, enhanced, fixedHalf, gainEC25)
	phase3rLogHybridGrid(t, ffmpeg, local, enhanced)
	for _, w := range blackboxWorstFrames(ffmpeg, enhanced, 6) {
		t.Logf("enhanced worst frame %d: snr=%.2f corr=%.3f refRMS=%.1f gotRMS=%.1f ratio=%.3f errRMS=%.1f",
			w.frame, w.snr, w.corr, w.refRMS, w.gotRMS, w.ratio, w.errRMS)
	}
	phase3rLogWorstFrameDetails(t, raw, ffmpeg, enhanced, enhancedLags, 8)

	if requireStrict {
		requireAsteriskQuality(t, "Asterisk local decoder", m, rmsRatio, lowCorrPct)
	}
	if requireEnhanced {
		requireAsteriskQuality(t, "Asterisk enhanced decoder", enhancedM, enhancedRMSRatio, enhancedLowCorrPct)
	}
}

func requireAsteriskQuality(t *testing.T, label string, m blackboxMetrics, rmsRatio, lowCorrPct float64) {
	t.Helper()
	const (
		minGlobalSNR = 8.0
		minCorr      = 0.85
		minRMSRatio  = 0.50
		maxRMSRatio  = 1.50
		maxLowCorr   = 0.05
	)
	if m.globalSNR < minGlobalSNR || m.corr < minCorr || rmsRatio < minRMSRatio || rmsRatio > maxRMSRatio || lowCorrPct > maxLowCorr {
		t.Fatalf("%s below FFmpeg black-box quality gate: gSNR %.2f<%.2f or corr %.3f<%.3f or rmsRatio %.3f not in [%.2f,%.2f] or lowCorr %.2f%%>%.2f%%",
			label, m.globalSNR, minGlobalSNR, m.corr, minCorr, rmsRatio, minRMSRatio, maxRMSRatio, 100*lowCorrPct, 100*maxLowCorr)
	}
}

func phase3rMatchFrameRMS(test, ref []int16) []int16 {
	n := len(test)
	if len(ref) < n {
		n = len(ref)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		testFrame := test[off:to]
		refFrame := ref[off:to]
		testRMS := envelopeRMS(testFrame)
		refRMS := envelopeRMS(refFrame)
		if testRMS <= 0 || refRMS <= 0 {
			copy(out[off:to], testFrame)
			continue
		}
		scale := refRMS / testRMS
		for i, sample := range testFrame {
			v := math.Round(float64(sample) * scale)
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			out[off+i] = int16(v)
		}
	}
	return out
}

func phase3rBestOfTwoByFrameSNR(ref, a, b []int16) []int16 {
	n := len(ref)
	if len(a) < n {
		n = len(a)
	}
	if len(b) < n {
		n = len(b)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		if envelopeSNRDB(ref[off:to], b[off:to]) > envelopeSNRDB(ref[off:to], a[off:to]) {
			copy(out[off:to], b[off:to])
		} else {
			copy(out[off:to], a[off:to])
		}
	}
	return out
}

func phase3rBestOfManyByFrameSNR(ref []int16, candidates ...[]int16) []int16 {
	n := len(ref)
	for _, c := range candidates {
		if len(c) < n {
			n = len(c)
		}
	}
	n -= n % frameSamples
	out := make([]int16, n)
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		best := candidates[0]
		bestSNR := envelopeSNRDB(ref[off:to], best[off:to])
		for _, c := range candidates[1:] {
			snr := envelopeSNRDB(ref[off:to], c[off:to])
			if snr > bestSNR {
				best = c
				bestSNR = snr
			}
		}
		copy(out[off:to], best[off:to])
	}
	return out
}

func phase3rFrameLagOracle(ref, test []int16, maxLag int) []int16 {
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		refFrame := ref[off:to]
		testFrame := test[off:to]
		bestLag := 0
		bestSNR := math.Inf(-1)
		for lag := -maxLag; lag <= maxLag; lag++ {
			shifted := phase3rShiftFrame(testFrame, lag)
			snr := envelopeSNRDB(refFrame, shifted)
			if snr > bestSNR {
				bestSNR = snr
				bestLag = lag
			}
		}
		copy(out[off:to], phase3rShiftFrame(testFrame, bestLag))
	}
	return out
}

func phase3rBestFrameLags(ref, test []int16, maxLag int) []int {
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	n -= n % frameSamples
	lags := make([]int, 0, n/frameSamples)
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		refFrame := ref[off:to]
		testFrame := test[off:to]
		bestLag := 0
		bestSNR := math.Inf(-1)
		for lag := -maxLag; lag <= maxLag; lag++ {
			shifted := phase3rShiftFrame(testFrame, lag)
			snr := envelopeSNRDB(refFrame, shifted)
			if snr > bestSNR {
				bestSNR = snr
				bestLag = lag
			}
		}
		lags = append(lags, bestLag)
	}
	return lags
}

func phase3rFrameLagRMSOracle(ref, test []int16, maxLag int) []int16 {
	lagged := phase3rFrameLagOracle(ref, test, maxLag)
	return phase3rMatchFrameRMS(lagged, ref)
}

func phase3rShiftFrame(in []int16, lag int) []int16 {
	out := make([]int16, len(in))
	for i := range out {
		j := i + lag
		if j >= 0 && j < len(in) {
			out[i] = in[j]
		}
	}
	return out
}

func phase3rContinuityLagRecover(in []int16, maxLag int) []int16 {
	n := len(in)
	n -= n % frameSamples
	out := make([]int16, n)
	if n == 0 {
		return out
	}
	copy(out[:frameSamples], in[:frameSamples])
	for off := frameSamples; off < n; off += frameSamples {
		to := off + frameSamples
		bestLag := 0
		bestScore := math.Inf(1)
		for lag := -maxLag; lag <= maxLag; lag++ {
			shifted := phase3rShiftFrame(in[off:to], lag)
			score := phase3rBoundaryContinuityScore(out[off-frameSamples:off], shifted)
			score += 0.02 * float64(absInt(lag)) * envelopeRecoveryRMS(in[off:to])
			if score < bestScore {
				bestScore = score
				bestLag = lag
			}
		}
		copy(out[off:to], phase3rShiftFrame(in[off:to], bestLag))
	}
	return out
}

func phase3rBoundaryContinuityScore(prev, cur []int16) float64 {
	const width = 12
	var score float64
	for i := 0; i < width; i++ {
		d := float64(prev[len(prev)-width+i]) - float64(cur[i])
		score += d * d
	}
	return math.Sqrt(score / width)
}

func phase3rLogLagHistogram(t *testing.T, label string, lags []int) {
	t.Helper()
	counts := map[int]int{}
	for _, lag := range lags {
		counts[lag]++
	}
	t.Logf("%s histogram: %s", label, phase3rFormatLagHistogram(counts, len(lags)))
}

func phase3rFormatLagHistogram(counts map[int]int, total int) string {
	var out string
	for lag := -20; lag <= 20; lag++ {
		count := counts[lag]
		if count == 0 {
			continue
		}
		pct := 100 * float64(count) / float64(total)
		if pct < 1.0 {
			continue
		}
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf("%+d:%d(%.1f%%)", lag, count, pct)
	}
	if out == "" {
		return "<no bucket >=1%>"
	}
	return out
}

type phase3rLagGroup struct {
	total   int
	nonzero int
	sum     int
	absSum  int
	counts  map[int]int
}

func phase3rLogLagFeatureSummary(t *testing.T, rawPath string, ref, got []int16, lags []int) {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw g729 payload for lag feature summary: %v", err)
	}
	frames := len(raw) / bitstream.FrameBytes
	if len(lags) < frames {
		frames = len(lags)
	}
	if rf := len(ref) / frameSamples; rf < frames {
		frames = rf
	}
	if gf := len(got) / frameSamples; gf < frames {
		frames = gf
	}
	groups := map[string]*phase3rLagGroup{}
	for frame := 0; frame < frames; frame++ {
		var bf bitstream.Frame
		if err := bitstream.Unpack(raw[frame*bitstream.FrameBytes:(frame+1)*bitstream.FrameBytes], &bf); err != nil {
			t.Fatalf("unpack raw frame %d for lag feature summary: %v", frame, err)
		}
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(bf.P1))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(bf.P2), tInt1)
		off := frame * frameSamples
		refRMS := envelopeRMS(ref[off : off+frameSamples])
		gotRMS := envelopeRMS(got[off : off+frameSamples])
		lag := lags[frame]

		phase3rAddLagFeature(groups, "all", lag)
		phase3rAddLagFeature(groups, fmt.Sprintf("sf1_frac=%+d", tFrac1), lag)
		phase3rAddLagFeature(groups, fmt.Sprintf("sf2_frac=%+d", tFrac2), lag)
		if tFrac1 == 0 && tFrac2 == 0 {
			phase3rAddLagFeature(groups, "both_frac_zero", lag)
		} else {
			phase3rAddLagFeature(groups, "any_frac_nonzero", lag)
		}
		if tInt1 < 40 || tInt2 < 40 {
			phase3rAddLagFeature(groups, "short_pitch", lag)
		} else {
			phase3rAddLagFeature(groups, "long_pitch", lag)
		}
		switch {
		case refRMS < 200:
			phase3rAddLagFeature(groups, "ref_rms<200", lag)
		case refRMS < 500:
			phase3rAddLagFeature(groups, "ref_rms<500", lag)
		default:
			phase3rAddLagFeature(groups, "ref_rms>=500", lag)
		}
		if refRMS > 0 {
			ratio := gotRMS / refRMS
			switch {
			case ratio < 0.5:
				phase3rAddLagFeature(groups, "got/ref<0.5", lag)
			case ratio > 1.5:
				phase3rAddLagFeature(groups, "got/ref>1.5", lag)
			default:
				phase3rAddLagFeature(groups, "got/ref_mid", lag)
			}
		}
	}
	t.Logf("enhanced oracle lag feature summary:")
	for _, label := range []string{
		"all",
		"sf1_frac=-1", "sf1_frac=+0", "sf1_frac=+1",
		"sf2_frac=-1", "sf2_frac=+0", "sf2_frac=+1",
		"both_frac_zero", "any_frac_nonzero",
		"short_pitch", "long_pitch",
		"ref_rms<200", "ref_rms<500", "ref_rms>=500",
		"got/ref<0.5", "got/ref_mid", "got/ref>1.5",
	} {
		g := groups[label]
		if g == nil || g.total == 0 {
			continue
		}
		t.Logf("  %-18s n=%4d nonzero=%5.1f%% mean=%+.2f meanAbs=%.2f hist=%s",
			label, g.total, 100*float64(g.nonzero)/float64(g.total),
			float64(g.sum)/float64(g.total), float64(g.absSum)/float64(g.total),
			phase3rFormatLagHistogram(g.counts, g.total))
	}
}

func phase3rLogRuntimeLagRuleGrid(t *testing.T, raw []byte, ref, got []int16) {
	t.Helper()
	type rule struct {
		name string
		lag  func(frame bitstream.Frame, sub1Frac, sub2Frac, sub1Int, sub2Int int) int
	}
	rules := []rule{
		{name: "no_shift", lag: func(bitstream.Frame, int, int, int, int) int { return 0 }},
		{name: "all_-1", lag: func(bitstream.Frame, int, int, int, int) int { return -1 }},
		{name: "all_+1", lag: func(bitstream.Frame, int, int, int, int) int { return +1 }},
		{name: "short_pitch_-1", lag: func(_ bitstream.Frame, _, _ int, t1, t2 int) int {
			if t1 < 40 || t2 < 40 {
				return -1
			}
			return 0
		}},
		{name: "short_pitch_+1", lag: func(_ bitstream.Frame, _, _ int, t1, t2 int) int {
			if t1 < 40 || t2 < 40 {
				return +1
			}
			return 0
		}},
		{name: "any_frac_-1", lag: func(_ bitstream.Frame, f1, f2, _, _ int) int {
			if f1 != 0 || f2 != 0 {
				return -1
			}
			return 0
		}},
		{name: "any_frac_+1", lag: func(_ bitstream.Frame, f1, f2, _, _ int) int {
			if f1 != 0 || f2 != 0 {
				return +1
			}
			return 0
		}},
		{name: "neg_frac_+1_pos_frac_-1", lag: func(_ bitstream.Frame, f1, f2, _, _ int) int {
			lag := 0
			if f1 < 0 || f2 < 0 {
				lag++
			}
			if f1 > 0 || f2 > 0 {
				lag--
			}
			return phase3rClampLag(lag, -1, 1)
		}},
		{name: "sf1_frac_sign", lag: func(_ bitstream.Frame, f1, _ int, _, _ int) int {
			return phase3rClampLag(-f1, -1, 1)
		}},
		{name: "sf2_frac_sign", lag: func(_ bitstream.Frame, _, f2 int, _, _ int) int {
			return phase3rClampLag(-f2, -1, 1)
		}},
		{name: "ga036_-1", lag: func(fr bitstream.Frame, _, _ int, _, _ int) int {
			if envelopeRecoveryHasGA036(uint8(fr.GA1)) || envelopeRecoveryHasGA036(uint8(fr.GA2)) {
				return -1
			}
			return 0
		}},
		{name: "ga036_+1", lag: func(fr bitstream.Frame, _, _ int, _, _ int) int {
			if envelopeRecoveryHasGA036(uint8(fr.GA1)) || envelopeRecoveryHasGA036(uint8(fr.GA2)) {
				return +1
			}
			return 0
		}},
	}
	t.Logf("runtime bitstream lag rule grid:")
	t.Logf("%-28s %8s %8s %8s", "rule", "gSNR", "seg", "corr")
	for _, r := range rules {
		out := phase3rApplyLagRule(t, raw, got, r.lag)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-28s %8.2f %8.2f %8.3f", r.name, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rLogTemporalBlendGrid(t *testing.T, ref, enhanced []int16) {
	t.Helper()
	variants := []struct {
		name string
		prev int
		cur  int
		next int
	}{
		{name: "identity", cur: 1},
		{name: "smooth_1_6_1", prev: 1, cur: 6, next: 1},
		{name: "smooth_1_4_1", prev: 1, cur: 4, next: 1},
		{name: "smooth_1_2_1", prev: 1, cur: 2, next: 1},
		{name: "blend_prev_1_3", prev: 1, cur: 3},
		{name: "blend_next_3_1", cur: 3, next: 1},
		{name: "blend_prev_1_7", prev: 1, cur: 7},
		{name: "blend_next_7_1", cur: 7, next: 1},
	}
	t.Logf("runtime temporal blend grid:")
	t.Logf("%-20s %8s %8s %8s", "variant", "gSNR", "seg", "corr")
	for _, v := range variants {
		out := phase3rTemporalBlend(enhanced, v.prev, v.cur, v.next)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-20s %8.2f %8.2f %8.3f", v.name, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rLogRuntimeLagFeatureGrid(t *testing.T, raw []byte, ref, local, enhanced []int16) {
	t.Helper()
	type rule struct {
		name string
		lag  func(feature phase3rRuntimeLagFeature) int
	}
	thresholdRules := []rule{
		{name: "enh_rms<200_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS < 200 {
				return -1
			}
			return 0
		}},
		{name: "enh_rms<200_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS < 200 {
				return +1
			}
			return 0
		}},
		{name: "enh_rms<500_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS < 500 {
				return -1
			}
			return 0
		}},
		{name: "enh_rms<500_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS < 500 {
				return +1
			}
			return 0
		}},
		{name: "enh_rms>=500_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS >= 500 {
				return -1
			}
			return 0
		}},
		{name: "enh_rms>=500_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS >= 500 {
				return +1
			}
			return 0
		}},
		{name: "enh_rms>=1000_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS >= 1000 {
				return -1
			}
			return 0
		}},
		{name: "enh_rms>=1000_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS >= 1000 {
				return +1
			}
			return 0
		}},
		{name: "enh/default<0.50_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.defaultRMS > 0 && f.enhancedRMS/f.defaultRMS < 0.50 {
				return -1
			}
			return 0
		}},
		{name: "enh/default<0.50_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.defaultRMS > 0 && f.enhancedRMS/f.defaultRMS < 0.50 {
				return +1
			}
			return 0
		}},
		{name: "enh/default>1.50_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.defaultRMS > 0 && f.enhancedRMS/f.defaultRMS > 1.50 {
				return -1
			}
			return 0
		}},
		{name: "enh/default>1.50_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if f.defaultRMS > 0 && f.enhancedRMS/f.defaultRMS > 1.50 {
				return +1
			}
			return 0
		}},
		{name: "any_frac_active_-1", lag: func(f phase3rRuntimeLagFeature) int {
			if (f.frac1 != 0 || f.frac2 != 0) && f.enhancedRMS >= 500 {
				return -1
			}
			return 0
		}},
		{name: "any_frac_active_+1", lag: func(f phase3rRuntimeLagFeature) int {
			if (f.frac1 != 0 || f.frac2 != 0) && f.enhancedRMS >= 500 {
				return +1
			}
			return 0
		}},
		{name: "frac_sign_active", lag: func(f phase3rRuntimeLagFeature) int {
			if f.enhancedRMS < 500 {
				return 0
			}
			return phase3rClampLag(-(f.frac1 + f.frac2), -1, 1)
		}},
	}
	t.Logf("runtime lag feature grid:")
	t.Logf("%-26s %8s %8s %8s", "rule", "gSNR", "seg", "corr")
	for _, r := range thresholdRules {
		out := phase3rApplyLagFeatureRule(t, raw, local, enhanced, r.lag)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-26s %8.2f %8.2f %8.3f", r.name, m.globalSNR, m.segSNR, m.corr)
	}
}

type phase3rLagPredicate struct {
	name string
	fn   func(phase3rRuntimeLagFeature) bool
}

type phase3rLagSearchResult struct {
	name    string
	changed int
	metric  blackboxMetrics
}

func phase3rLogRuntimeLagPredicateSearch(t *testing.T, raw []byte, ref, local, enhanced []int16) {
	t.Helper()
	features := phase3rBuildRuntimeLagFeatures(t, raw, local, enhanced)
	predicates := phase3rLagPredicates()
	results := []phase3rLagSearchResult{{
		name:   "enhanced_all",
		metric: blackboxMeasure(ref, enhanced, 0),
	}}
	lags := []int{-2, -1, +1, +2}
	for _, p := range predicates {
		for _, lag := range lags {
			out, changed := phase3rSelectLagByPredicate(enhanced, features, p.fn, lag)
			if changed == 0 {
				continue
			}
			results = append(results, phase3rLagSearchResult{
				name:    fmt.Sprintf("%s_lag%+d", p.name, lag),
				changed: changed,
				metric:  blackboxMeasure(ref, out, 0),
			})
		}
	}
	for i := 0; i < len(predicates); i++ {
		for j := i + 1; j < len(predicates); j++ {
			p1, p2 := predicates[i], predicates[j]
			for _, lag := range lags {
				out, changed := phase3rSelectLagByPredicate(enhanced, features, func(f phase3rRuntimeLagFeature) bool {
					return p1.fn(f) && p2.fn(f)
				}, lag)
				if changed == 0 {
					continue
				}
				results = append(results, phase3rLagSearchResult{
					name:    fmt.Sprintf("%s_and_%s_lag%+d", p1.name, p2.name, lag),
					changed: changed,
					metric:  blackboxMeasure(ref, out, 0),
				})
			}
		}
	}

	t.Logf("runtime lag predicate search, top global-SNR candidates:")
	phase3rLogLagSearchResults(t, results, func(a, b phase3rLagSearchResult) bool {
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		return a.name < b.name
	})

	t.Logf("runtime lag predicate search, top segmental-SNR candidates:")
	phase3rLogLagSearchResults(t, results, func(a, b phase3rLagSearchResult) bool {
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		return a.name < b.name
	})
}

func phase3rLagPredicates() []phase3rLagPredicate {
	ratio := func(num, den float64) float64 {
		if den <= 0 {
			return 0
		}
		return num / den
	}
	return []phase3rLagPredicate{
		{name: "enh<500", fn: func(f phase3rRuntimeLagFeature) bool { return f.enhancedRMS < 500 }},
		{name: "enh>=500", fn: func(f phase3rRuntimeLagFeature) bool { return f.enhancedRMS >= 500 }},
		{name: "enh>=1000", fn: func(f phase3rRuntimeLagFeature) bool { return f.enhancedRMS >= 1000 }},
		{name: "enh>=2000", fn: func(f phase3rRuntimeLagFeature) bool { return f.enhancedRMS >= 2000 }},
		{name: "enh>=4000", fn: func(f phase3rRuntimeLagFeature) bool { return f.enhancedRMS >= 4000 }},
		{name: "enh/default<0.5", fn: func(f phase3rRuntimeLagFeature) bool { return ratio(f.enhancedRMS, f.defaultRMS) < 0.5 }},
		{name: "enh/default>=0.5", fn: func(f phase3rRuntimeLagFeature) bool { return ratio(f.enhancedRMS, f.defaultRMS) >= 0.5 }},
		{name: "enh/default>1.2", fn: func(f phase3rRuntimeLagFeature) bool { return ratio(f.enhancedRMS, f.defaultRMS) > 1.2 }},
		{name: "frac", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1 != 0 || f.frac2 != 0 }},
		{name: "nofrac", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1 == 0 && f.frac2 == 0 }},
		{name: "frac_sum_neg", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1+f.frac2 < 0 }},
		{name: "frac_sum_pos", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1+f.frac2 > 0 }},
		{name: "frac_mixed", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1*f.frac2 < 0 }},
		{name: "sf1_frac_neg", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1 < 0 }},
		{name: "sf1_frac_pos", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac1 > 0 }},
		{name: "sf2_frac_neg", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac2 < 0 }},
		{name: "sf2_frac_pos", fn: func(f phase3rRuntimeLagFeature) bool { return f.frac2 > 0 }},
		{name: "short", fn: func(f phase3rRuntimeLagFeature) bool { return f.int1 < 40 || f.int2 < 40 }},
		{name: "long", fn: func(f phase3rRuntimeLagFeature) bool { return f.int1 >= 40 && f.int2 >= 40 }},
	}
}

func phase3rBuildRuntimeLagFeatures(t *testing.T, raw []byte, local, enhanced []int16) []phase3rRuntimeLagFeature {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if lf := len(local) / frameSamples; lf < frames {
		frames = lf
	}
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	features := make([]phase3rRuntimeLagFeature, frames)
	for frame := 0; frame < frames; frame++ {
		var bf bitstream.Frame
		if err := bitstream.Unpack(raw[frame*bitstream.FrameBytes:(frame+1)*bitstream.FrameBytes], &bf); err != nil {
			t.Fatalf("unpack raw frame %d for lag predicate search: %v", frame, err)
		}
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(bf.P1))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(bf.P2), tInt1)
		off := frame * frameSamples
		features[frame] = phase3rRuntimeLagFeature{
			defaultRMS:  envelopeRMS(local[off : off+frameSamples]),
			enhancedRMS: envelopeRMS(enhanced[off : off+frameSamples]),
			frac1:       tFrac1,
			frac2:       tFrac2,
			int1:        tInt1,
			int2:        tInt2,
		}
	}
	return features
}

func phase3rSelectLagByPredicate(enhanced []int16, features []phase3rRuntimeLagFeature, predicate func(phase3rRuntimeLagFeature) bool, lag int) ([]int16, int) {
	frames := len(enhanced) / frameSamples
	if len(features) < frames {
		frames = len(features)
	}
	out := make([]int16, frames*frameSamples)
	var changed int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if predicate(features[frame]) {
			copy(out[off:off+frameSamples], phase3rShiftFrame(enhanced[off:off+frameSamples], lag))
			changed++
		} else {
			copy(out[off:off+frameSamples], enhanced[off:off+frameSamples])
		}
	}
	return out, changed
}

func phase3rLogLagSearchResults(t *testing.T, results []phase3rLagSearchResult, less func(a, b phase3rLagSearchResult) bool) {
	t.Helper()
	rows := append([]phase3rLagSearchResult(nil), results...)
	sort.Slice(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
	t.Logf("%-50s %8s %8s %8s %8s", "selector", "changed", "gSNR", "seg", "corr")
	limit := 12
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		r := rows[i]
		t.Logf("%-50s %8d %8.2f %8.2f %8.3f", r.name, r.changed, r.metric.globalSNR, r.metric.segSNR, r.metric.corr)
	}
}

func phase3rLogGainModeSelectorGrid(t *testing.T, ref, enhanced, ec25, fixedHalf []int16) {
	t.Helper()
	type selector struct {
		name string
		fn   func(enhRMS, ec25RMS, fixedRMS float64) int
	}
	selectors := []selector{
		{name: "enhanced_all", fn: func(float64, float64, float64) int { return 0 }},
		{name: "ec25_all", fn: func(float64, float64, float64) int { return 1 }},
		{name: "fixed_half_all", fn: func(float64, float64, float64) int { return 2 }},
		{name: "ec25_if_enh<200", fn: func(e, _, _ float64) int {
			if e < 200 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_enh<500", fn: func(e, _, _ float64) int {
			if e < 500 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_enh>=500", fn: func(e, _, _ float64) int {
			if e >= 500 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_enh>=1000", fn: func(e, _, _ float64) int {
			if e >= 1000 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_ec/enh>0.9", fn: func(e, q, _ float64) int {
			if e > 0 && q/e > 0.9 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_ec/enh>1.1", fn: func(e, q, _ float64) int {
			if e > 0 && q/e > 1.1 {
				return 1
			}
			return 0
		}},
		{name: "ec25_if_ec/enh<0.9", fn: func(e, q, _ float64) int {
			if e > 0 && q/e < 0.9 {
				return 1
			}
			return 0
		}},
		{name: "fixed_if_enh<500", fn: func(e, _, _ float64) int {
			if e < 500 {
				return 2
			}
			return 0
		}},
		{name: "fixed_if_enh>=500", fn: func(e, _, _ float64) int {
			if e >= 500 {
				return 2
			}
			return 0
		}},
		{name: "fixed_if_fx/enh<0.8", fn: func(e, _, f float64) int {
			if e > 0 && f/e < 0.8 {
				return 2
			}
			return 0
		}},
		{name: "ec25_high_fixed_low", fn: func(e, q, f float64) int {
			if e > 0 && q/e > 0.9 && f/e < 0.8 {
				return 1
			}
			return 0
		}},
	}
	t.Logf("runtime gain-mode selector grid:")
	t.Logf("%-24s %8s %8s %8s %8s %8s", "selector", "ec25", "fixed", "gSNR", "seg", "corr")
	for _, s := range selectors {
		out, ec25Frames, fixedFrames := phase3rSelectGainMode(enhanced, ec25, fixedHalf, s.fn)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-24s %8d %8d %8.2f %8.2f %8.3f", s.name, ec25Frames, fixedFrames, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rSelectGainMode(enhanced, ec25, fixedHalf []int16, selectMode func(enhRMS, ec25RMS, fixedRMS float64) int) ([]int16, int, int) {
	n := len(enhanced)
	if len(ec25) < n {
		n = len(ec25)
	}
	if len(fixedHalf) < n {
		n = len(fixedHalf)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	var ec25Frames, fixedFrames int
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		mode := selectMode(envelopeRMS(enhanced[off:to]), envelopeRMS(ec25[off:to]), envelopeRMS(fixedHalf[off:to]))
		switch mode {
		case 1:
			copy(out[off:to], ec25[off:to])
			ec25Frames++
		case 2:
			copy(out[off:to], fixedHalf[off:to])
			fixedFrames++
		default:
			copy(out[off:to], enhanced[off:to])
		}
	}
	return out, ec25Frames, fixedFrames
}

type phase3rGainSaturationFeature struct {
	enhancedRMS float64
	maxGCQ12    int
	maxGpQ14    int
	frame       bitstream.Frame
	shortPitch  bool
	anyFrac     bool
}

func phase3rLogGainSaturationSelectorGrid(t *testing.T, raw []byte, ref, enhanced, fixedHalf []int16) {
	t.Helper()
	type selector struct {
		name string
		fn   func(phase3rGainSaturationFeature) bool
	}
	selectors := []selector{
		{name: "enhanced_all", fn: func(phase3rGainSaturationFeature) bool { return false }},
		{name: "fixed_if_gc32767", fn: func(f phase3rGainSaturationFeature) bool { return f.maxGCQ12 >= 32767 }},
		{name: "fixed_if_gc>=30000", fn: func(f phase3rGainSaturationFeature) bool { return f.maxGCQ12 >= 30000 }},
		{name: "fixed_if_gc>=24000", fn: func(f phase3rGainSaturationFeature) bool { return f.maxGCQ12 >= 24000 }},
		{name: "fixed_if_gc32767_active", fn: func(f phase3rGainSaturationFeature) bool {
			return f.maxGCQ12 >= 32767 && f.enhancedRMS >= 500
		}},
		{name: "fixed_if_gc32767_loud", fn: func(f phase3rGainSaturationFeature) bool {
			return f.maxGCQ12 >= 32767 && f.enhancedRMS >= 1000
		}},
		{name: "fixed_if_gc32767_short", fn: func(f phase3rGainSaturationFeature) bool {
			return f.maxGCQ12 >= 32767 && f.shortPitch
		}},
		{name: "fixed_if_gc32767_frac", fn: func(f phase3rGainSaturationFeature) bool {
			return f.maxGCQ12 >= 32767 && f.anyFrac
		}},
		{name: "fixed_if_gp>=15000_gc32767", fn: func(f phase3rGainSaturationFeature) bool {
			return f.maxGpQ14 >= 15000 && f.maxGCQ12 >= 32767
		}},
		{name: "fixed_if_ga6_or_gb15", fn: func(f phase3rGainSaturationFeature) bool {
			return f.frame.GA1 == 6 || f.frame.GA2 == 6 || f.frame.GB1 == 15 || f.frame.GB2 == 15
		}},
		{name: "fixed_if_ga5_gb14plus", fn: func(f phase3rGainSaturationFeature) bool {
			return (f.frame.GA1 == 5 && f.frame.GB1 >= 14) || (f.frame.GA2 == 5 && f.frame.GB2 >= 14)
		}},
	}
	features := phase3rBuildGainSaturationFeatures(t, raw, enhanced)
	t.Logf("runtime gain-saturation selector grid:")
	t.Logf("%-28s %8s %8s %8s %8s", "selector", "fixed", "gSNR", "seg", "corr")
	for _, s := range selectors {
		out, fixedFrames := phase3rSelectFixedHalfByFeature(enhanced, fixedHalf, features, s.fn)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-28s %8d %8.2f %8.2f %8.3f", s.name, fixedFrames, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rBuildGainSaturationFeatures(t *testing.T, raw []byte, enhanced []int16) []phase3rGainSaturationFeature {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	features := make([]phase3rGainSaturationFeature, frames)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		taps, err := dec.DecodeWithTaps(raw[start : start+bitstream.FrameBytes])
		if err != nil {
			t.Fatalf("DecodeWithTaps for gain saturation feature frame %d: %v", frame, err)
		}
		maxGC := int(taps.Sub[0].GcQ12)
		if int(taps.Sub[1].GcQ12) > maxGC {
			maxGC = int(taps.Sub[1].GcQ12)
		}
		maxGp := int(taps.Sub[0].GpQ14)
		if int(taps.Sub[1].GpQ14) > maxGp {
			maxGp = int(taps.Sub[1].GpQ14)
		}
		off := frame * frameSamples
		features[frame] = phase3rGainSaturationFeature{
			enhancedRMS: envelopeRMS(enhanced[off : off+frameSamples]),
			maxGCQ12:    maxGC,
			maxGpQ14:    maxGp,
			frame:       taps.Frame,
			shortPitch:  taps.Sub[0].TInt < 40 || taps.Sub[1].TInt < 40,
			anyFrac:     taps.Sub[0].TFrac != 0 || taps.Sub[1].TFrac != 0,
		}
	}
	return features
}

func phase3rSelectFixedHalfByFeature(enhanced, fixedHalf []int16, features []phase3rGainSaturationFeature, useFixed func(phase3rGainSaturationFeature) bool) ([]int16, int) {
	n := len(enhanced)
	if len(fixedHalf) < n {
		n = len(fixedHalf)
	}
	frames := n / frameSamples
	if len(features) < frames {
		frames = len(features)
	}
	out := make([]int16, frames*frameSamples)
	var fixedFrames int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if useFixed(features[frame]) {
			copy(out[off:off+frameSamples], fixedHalf[off:off+frameSamples])
			fixedFrames++
		} else {
			copy(out[off:off+frameSamples], enhanced[off:off+frameSamples])
		}
	}
	return out, fixedFrames
}

type phase3rOverhangFeature struct {
	enhancedRMS float64
	uRMS        float64
	sRMS        float64
	spfRMS      float64
	maxGCQ12    int
	hasGA5GB14  bool
	hasGA036    bool
	shortPitch  bool
	anyFrac     bool
}

func phase3rLogOverhangDampingGrid(t *testing.T, raw []byte, ref, enhanced []int16) {
	t.Helper()
	type condition struct {
		name string
		fn   func(phase3rOverhangFeature) bool
	}
	conditions := []condition{
		{name: "enh<200", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 200 }},
		{name: "enh<500", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 }},
		{name: "enh<500_ga5gb14", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.hasGA5GB14 }},
		{name: "enh<500_u<2", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.uRMS < 2 }},
		{name: "enh<500_u<2_s>20", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.uRMS < 2 && f.sRMS > 20 }},
		{name: "enh<500_u<2_ga5gb14", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.uRMS < 2 && f.hasGA5GB14 }},
		{name: "enh<800_u<2_ga5gb14", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 800 && f.uRMS < 2 && f.hasGA5GB14 }},
		{name: "enh<500_ga036", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.hasGA036 }},
		{name: "enh<500_short", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 && f.shortPitch }},
	}
	scales := []struct {
		name  string
		scale float64
	}{
		{name: "x3/4", scale: 0.75},
		{name: "x1/2", scale: 0.50},
		{name: "x1/4", scale: 0.25},
	}
	features := phase3rBuildOverhangFeatures(t, raw, enhanced)
	t.Logf("runtime overhang damping grid:")
	t.Logf("%-30s %8s %8s %8s %8s %8s", "condition", "frames", "scale", "gSNR", "seg", "corr")
	base := blackboxMeasure(ref, enhanced, 40)
	t.Logf("%-30s %8d %8s %8.2f %8.2f %8.3f", "enhanced_all", 0, "x1", base.globalSNR, base.segSNR, base.corr)
	for _, c := range conditions {
		for _, s := range scales {
			out, changed := phase3rApplyOverhangDamping(enhanced, features, c.fn, s.scale)
			m := blackboxMeasure(ref, out, 40)
			t.Logf("%-30s %8d %8s %8.2f %8.2f %8.3f", c.name, changed, s.name, m.globalSNR, m.segSNR, m.corr)
		}
	}
}

func phase3rBuildOverhangFeatures(t *testing.T, raw []byte, enhanced []int16) []phase3rOverhangFeature {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	features := make([]phase3rOverhangFeature, frames)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		taps, err := dec.DecodeWithTaps(raw[start : start+bitstream.FrameBytes])
		if err != nil {
			t.Fatalf("DecodeWithTaps for overhang feature frame %d: %v", frame, err)
		}
		off := frame * frameSamples
		uRMS := phase3rFrameTapRMS(taps.Sub[0].U[:], taps.Sub[1].U[:])
		sRMS := phase3rFrameTapRMS(taps.Sub[0].S[:], taps.Sub[1].S[:])
		spfRMS := phase3rFrameTapRMS(taps.Sub[0].SPf[:], taps.Sub[1].SPf[:])
		maxGC := int(taps.Sub[0].GcQ12)
		if int(taps.Sub[1].GcQ12) > maxGC {
			maxGC = int(taps.Sub[1].GcQ12)
		}
		features[frame] = phase3rOverhangFeature{
			enhancedRMS: envelopeRMS(enhanced[off : off+frameSamples]),
			uRMS:        uRMS,
			sRMS:        sRMS,
			spfRMS:      spfRMS,
			maxGCQ12:    maxGC,
			hasGA5GB14:  (taps.Frame.GA1 == 5 && taps.Frame.GB1 >= 14) || (taps.Frame.GA2 == 5 && taps.Frame.GB2 >= 14),
			hasGA036:    envelopeRecoveryHasGA036(uint8(taps.Frame.GA1)) || envelopeRecoveryHasGA036(uint8(taps.Frame.GA2)),
			shortPitch:  taps.Sub[0].TInt < 40 || taps.Sub[1].TInt < 40,
			anyFrac:     taps.Sub[0].TFrac != 0 || taps.Sub[1].TFrac != 0,
		}
	}
	return features
}

func phase3rFrameTapRMS(a, b []int16) float64 {
	var e float64
	var n int
	for _, v := range a {
		x := float64(v)
		e += x * x
		n++
	}
	for _, v := range b {
		x := float64(v)
		e += x * x
		n++
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(e / float64(n))
}

func phase3rApplyOverhangDamping(in []int16, features []phase3rOverhangFeature, match func(phase3rOverhangFeature) bool, scale float64) ([]int16, int) {
	frames := len(in) / frameSamples
	if len(features) < frames {
		frames = len(features)
	}
	out := make([]int16, frames*frameSamples)
	var changed int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		copy(out[off:off+frameSamples], in[off:off+frameSamples])
		if !match(features[frame]) {
			continue
		}
		changed++
		for i := off; i < off+frameSamples; i++ {
			out[i] = envelopeRecoveryScale(out[i], scale)
		}
	}
	return out, changed
}

func phase3rLogActiveDampingGrid(t *testing.T, raw []byte, ref, enhanced []int16) {
	t.Helper()
	type condition struct {
		name string
		fn   func(phase3rOverhangFeature) bool
	}
	conditions := []condition{
		{name: "enh>=1000_gc32767", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 1000 && f.maxGCQ12 >= 32767 }},
		{name: "enh>=2000_gc32767", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 2000 && f.maxGCQ12 >= 32767 }},
		{name: "enh>=4000_gc32767", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 4000 && f.maxGCQ12 >= 32767 }},
		{name: "enh>=1000_ga036", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 1000 && f.hasGA036 }},
		{name: "enh>=2000_ga036", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 2000 && f.hasGA036 }},
		{name: "enh>=1000_short_gc32767", fn: func(f phase3rOverhangFeature) bool {
			return f.enhancedRMS >= 1000 && f.maxGCQ12 >= 32767 && f.shortPitch
		}},
		{name: "enh>=2000_short_gc32767", fn: func(f phase3rOverhangFeature) bool {
			return f.enhancedRMS >= 2000 && f.maxGCQ12 >= 32767 && f.shortPitch
		}},
		{name: "enh>=1000_s/u>=8", fn: func(f phase3rOverhangFeature) bool {
			return f.enhancedRMS >= 1000 && f.uRMS > 0 && f.sRMS/f.uRMS >= 8
		}},
		{name: "enh>=2000_s/u>=8", fn: func(f phase3rOverhangFeature) bool {
			return f.enhancedRMS >= 2000 && f.uRMS > 0 && f.sRMS/f.uRMS >= 8
		}},
	}
	scales := []struct {
		name  string
		scale float64
	}{
		{name: "x7/8", scale: 0.875},
		{name: "x3/4", scale: 0.750},
		{name: "x5/8", scale: 0.625},
		{name: "x1/2", scale: 0.500},
	}
	features := phase3rBuildOverhangFeatures(t, raw, enhanced)
	t.Logf("runtime active-frame damping grid:")
	t.Logf("%-30s %8s %8s %8s %8s %8s", "condition", "frames", "scale", "gSNR", "seg", "corr")
	base := blackboxMeasure(ref, enhanced, 40)
	t.Logf("%-30s %8d %8s %8.2f %8.2f %8.3f", "enhanced_all", 0, "x1", base.globalSNR, base.segSNR, base.corr)
	for _, c := range conditions {
		for _, s := range scales {
			out, changed := phase3rApplyOverhangDamping(enhanced, features, c.fn, s.scale)
			m := blackboxMeasure(ref, out, 40)
			t.Logf("%-30s %8d %8s %8.2f %8.2f %8.3f", c.name, changed, s.name, m.globalSNR, m.segSNR, m.corr)
		}
	}
}

func phase3rLogContributionSelectorGrid(t *testing.T, raw []byte, ref, enhanced, pitchHalf, pitchThreeQuarter, noFCBEnhancement []int16) {
	t.Helper()
	type selector struct {
		name string
		fn   func(phase3rOverhangFeature) int
	}
	selectors := []selector{
		{name: "enhanced_all", fn: func(phase3rOverhangFeature) int { return 0 }},
		{name: "pitch_half_if_short", fn: func(f phase3rOverhangFeature) int {
			if f.shortPitch {
				return 1
			}
			return 0
		}},
		{name: "pitch_3q_if_short", fn: func(f phase3rOverhangFeature) int {
			if f.shortPitch {
				return 2
			}
			return 0
		}},
		{name: "pitch_half_if_gc32767", fn: func(f phase3rOverhangFeature) int {
			if f.maxGCQ12 >= 32767 {
				return 1
			}
			return 0
		}},
		{name: "pitch_3q_if_gc32767", fn: func(f phase3rOverhangFeature) int {
			if f.maxGCQ12 >= 32767 {
				return 2
			}
			return 0
		}},
		{name: "pitch_half_if_short_gc32767", fn: func(f phase3rOverhangFeature) int {
			if f.shortPitch && f.maxGCQ12 >= 32767 {
				return 1
			}
			return 0
		}},
		{name: "pitch_3q_if_short_gc32767", fn: func(f phase3rOverhangFeature) int {
			if f.shortPitch && f.maxGCQ12 >= 32767 {
				return 2
			}
			return 0
		}},
		{name: "pitch_half_if_loud_short", fn: func(f phase3rOverhangFeature) int {
			if f.enhancedRMS >= 1000 && f.shortPitch {
				return 1
			}
			return 0
		}},
		{name: "pitch_3q_if_loud_short", fn: func(f phase3rOverhangFeature) int {
			if f.enhancedRMS >= 1000 && f.shortPitch {
				return 2
			}
			return 0
		}},
		{name: "nofcb_if_short_gc32767", fn: func(f phase3rOverhangFeature) int {
			if f.shortPitch && f.maxGCQ12 >= 32767 {
				return 3
			}
			return 0
		}},
		{name: "nofcb_if_ga036_loud", fn: func(f phase3rOverhangFeature) int {
			if f.enhancedRMS >= 1000 && f.hasGA036 {
				return 3
			}
			return 0
		}},
	}
	features := phase3rBuildOverhangFeatures(t, raw, enhanced)
	t.Logf("runtime contribution selector grid:")
	t.Logf("%-30s %8s %8s %8s %8s %8s", "selector", "changed", "mode1", "gSNR", "seg", "corr")
	for _, s := range selectors {
		out, changed, mode1 := phase3rSelectContributionMode(enhanced, pitchHalf, pitchThreeQuarter, noFCBEnhancement, features, s.fn)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-30s %8d %8d %8.2f %8.2f %8.3f", s.name, changed, mode1, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rSelectContributionMode(enhanced, pitchHalf, pitchThreeQuarter, noFCBEnhancement []int16, features []phase3rOverhangFeature, selectMode func(phase3rOverhangFeature) int) ([]int16, int, int) {
	n := len(enhanced)
	for _, candidate := range [][]int16{pitchHalf, pitchThreeQuarter, noFCBEnhancement} {
		if len(candidate) < n {
			n = len(candidate)
		}
	}
	frames := n / frameSamples
	if len(features) < frames {
		frames = len(features)
	}
	out := make([]int16, frames*frameSamples)
	var changed int
	var mode1 int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		switch selectMode(features[frame]) {
		case 1:
			copy(out[off:off+frameSamples], pitchHalf[off:off+frameSamples])
			changed++
			mode1++
		case 2:
			copy(out[off:off+frameSamples], pitchThreeQuarter[off:off+frameSamples])
			changed++
		case 3:
			copy(out[off:off+frameSamples], noFCBEnhancement[off:off+frameSamples])
			changed++
		default:
			copy(out[off:off+frameSamples], enhanced[off:off+frameSamples])
		}
	}
	return out, changed, mode1
}

type phase3rContributionCandidate struct {
	name string
	pcm  []int16
}

type phase3rContributionPredicate struct {
	name string
	fn   func(phase3rOverhangFeature) bool
}

type phase3rContributionSearchResult struct {
	name    string
	changed int
	metric  blackboxMetrics
}

func phase3rLogContributionPredicateSearch(t *testing.T, raw []byte, ref, enhanced, fixedHalf, gainEC25, pitchHalf, pitchThreeQuarter, noFCBEnhancement []int16) {
	t.Helper()
	features := phase3rBuildOverhangFeatures(t, raw, enhanced)
	candidates := []phase3rContributionCandidate{
		{name: "fixed_half", pcm: fixedHalf},
		{name: "ec25", pcm: gainEC25},
		{name: "pitch_half", pcm: pitchHalf},
		{name: "pitch_3q", pcm: pitchThreeQuarter},
		{name: "no_fcb_enh", pcm: noFCBEnhancement},
	}
	predicates := phase3rContributionSearchPredicates()
	results := []phase3rContributionSearchResult{{
		name:   "enhanced_all",
		metric: blackboxMeasure(ref, enhanced, 0),
	}}
	for _, c := range candidates {
		for _, p := range predicates {
			out, changed := phase3rSelectCandidateByFeature(enhanced, c.pcm, features, p.fn)
			if changed == 0 {
				continue
			}
			results = append(results, phase3rContributionSearchResult{
				name:    c.name + "_if_" + p.name,
				changed: changed,
				metric:  blackboxMeasure(ref, out, 0),
			})
		}
		for i := 0; i < len(predicates); i++ {
			for j := i + 1; j < len(predicates); j++ {
				p1, p2 := predicates[i], predicates[j]
				name := p1.name + "_and_" + p2.name
				out, changed := phase3rSelectCandidateByFeature(enhanced, c.pcm, features, func(f phase3rOverhangFeature) bool {
					return p1.fn(f) && p2.fn(f)
				})
				if changed == 0 {
					continue
				}
				results = append(results, phase3rContributionSearchResult{
					name:    c.name + "_if_" + name,
					changed: changed,
					metric:  blackboxMeasure(ref, out, 0),
				})
			}
		}
	}

	t.Logf("runtime contribution predicate search, top global-SNR candidates:")
	phase3rLogContributionSearchResults(t, results, func(a, b phase3rContributionSearchResult) bool {
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		return a.name < b.name
	})

	t.Logf("runtime contribution predicate search, top segmental-SNR candidates:")
	phase3rLogContributionSearchResults(t, results, func(a, b phase3rContributionSearchResult) bool {
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		return a.name < b.name
	})
}

func phase3rContributionSearchPredicates() []phase3rContributionPredicate {
	ratio := func(num, den float64) float64 {
		if den <= 0 {
			return 0
		}
		return num / den
	}
	return []phase3rContributionPredicate{
		{name: "enh<500", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS < 500 }},
		{name: "enh500..2000", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 500 && f.enhancedRMS < 2000 }},
		{name: "enh>=1000", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 1000 }},
		{name: "enh>=2000", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 2000 }},
		{name: "enh>=4000", fn: func(f phase3rOverhangFeature) bool { return f.enhancedRMS >= 4000 }},
		{name: "u<2", fn: func(f phase3rOverhangFeature) bool { return f.uRMS < 2 }},
		{name: "u>=2", fn: func(f phase3rOverhangFeature) bool { return f.uRMS >= 2 }},
		{name: "s/u>=8", fn: func(f phase3rOverhangFeature) bool { return ratio(f.sRMS, f.uRMS) >= 8 }},
		{name: "s/u<8", fn: func(f phase3rOverhangFeature) bool { return ratio(f.sRMS, f.uRMS) < 8 }},
		{name: "spf/s>=1", fn: func(f phase3rOverhangFeature) bool { return ratio(f.spfRMS, f.sRMS) >= 1 }},
		{name: "spf/s<1", fn: func(f phase3rOverhangFeature) bool { return ratio(f.spfRMS, f.sRMS) < 1 }},
		{name: "short", fn: func(f phase3rOverhangFeature) bool { return f.shortPitch }},
		{name: "long", fn: func(f phase3rOverhangFeature) bool { return !f.shortPitch }},
		{name: "frac", fn: func(f phase3rOverhangFeature) bool { return f.anyFrac }},
		{name: "nofrac", fn: func(f phase3rOverhangFeature) bool { return !f.anyFrac }},
		{name: "gc32767", fn: func(f phase3rOverhangFeature) bool { return f.maxGCQ12 >= 32767 }},
		{name: "gc<32767", fn: func(f phase3rOverhangFeature) bool { return f.maxGCQ12 < 32767 }},
		{name: "ga036", fn: func(f phase3rOverhangFeature) bool { return f.hasGA036 }},
		{name: "no_ga036", fn: func(f phase3rOverhangFeature) bool { return !f.hasGA036 }},
		{name: "ga5gb14", fn: func(f phase3rOverhangFeature) bool { return f.hasGA5GB14 }},
		{name: "no_ga5gb14", fn: func(f phase3rOverhangFeature) bool { return !f.hasGA5GB14 }},
	}
}

func phase3rSelectCandidateByFeature(enhanced, candidate []int16, features []phase3rOverhangFeature, useCandidate func(phase3rOverhangFeature) bool) ([]int16, int) {
	n := len(enhanced)
	if len(candidate) < n {
		n = len(candidate)
	}
	frames := n / frameSamples
	if len(features) < frames {
		frames = len(features)
	}
	out := make([]int16, frames*frameSamples)
	var changed int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if useCandidate(features[frame]) {
			copy(out[off:off+frameSamples], candidate[off:off+frameSamples])
			changed++
		} else {
			copy(out[off:off+frameSamples], enhanced[off:off+frameSamples])
		}
	}
	return out, changed
}

func phase3rLogContributionSearchResults(t *testing.T, results []phase3rContributionSearchResult, less func(a, b phase3rContributionSearchResult) bool) {
	t.Helper()
	rows := append([]phase3rContributionSearchResult(nil), results...)
	sort.Slice(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
	t.Logf("%-70s %8s %8s %8s %8s", "selector", "changed", "gSNR", "seg", "corr")
	limit := 12
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		r := rows[i]
		t.Logf("%-70s %8d %8.2f %8.2f %8.3f", r.name, r.changed, r.metric.globalSNR, r.metric.segSNR, r.metric.corr)
	}
}

func phase3rLogGainPairSelectorSearch(t *testing.T, raw []byte, ref, enhanced, fixedHalf, gainEC25 []int16) {
	t.Helper()
	pairs := phase3rCollectFrameGainPairs(t, raw)
	type candidate struct {
		name string
		pcm  []int16
	}
	candidates := []candidate{
		{name: "fixed_half", pcm: fixedHalf},
		{name: "ec25", pcm: gainEC25},
	}
	results := []phase3rContributionSearchResult{{
		name:   "enhanced_all",
		metric: blackboxMeasure(ref, enhanced, 0),
	}}
	seen := map[string]bool{}
	for _, framePairs := range pairs {
		for _, pair := range framePairs {
			seen[pair] = true
		}
	}
	for pair := range seen {
		for _, c := range candidates {
			out, changed := phase3rSelectCandidateByGainPair(enhanced, c.pcm, pairs, pair)
			if changed == 0 {
				continue
			}
			results = append(results, phase3rContributionSearchResult{
				name:    c.name + "_if_" + pair,
				changed: changed,
				metric:  blackboxMeasure(ref, out, 0),
			})
		}
	}

	t.Logf("runtime gain-pair selector search, top global-SNR candidates:")
	phase3rLogContributionSearchResults(t, results, func(a, b phase3rContributionSearchResult) bool {
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		return a.name < b.name
	})
	t.Logf("runtime gain-pair selector search, top segmental-SNR candidates:")
	phase3rLogContributionSearchResults(t, results, func(a, b phase3rContributionSearchResult) bool {
		if a.metric.segSNR != b.metric.segSNR {
			return a.metric.segSNR > b.metric.segSNR
		}
		if a.metric.globalSNR != b.metric.globalSNR {
			return a.metric.globalSNR > b.metric.globalSNR
		}
		return a.name < b.name
	})
}

func phase3rCollectFrameGainPairs(t *testing.T, raw []byte) [][2]string {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	out := make([][2]string, frames)
	for frame := 0; frame < frames; frame++ {
		var fr bitstream.Frame
		start := frame * bitstream.FrameBytes
		if err := bitstream.Unpack(raw[start:start+bitstream.FrameBytes], &fr); err != nil {
			t.Fatalf("Unpack gain-pair frame %d: %v", frame, err)
		}
		out[frame] = [2]string{
			fmt.Sprintf("GA%d_GB%d", fr.GA1, fr.GB1),
			fmt.Sprintf("GA%d_GB%d", fr.GA2, fr.GB2),
		}
	}
	return out
}

func phase3rSelectCandidateByGainPair(enhanced, candidate []int16, pairs [][2]string, pair string) ([]int16, int) {
	n := len(enhanced)
	if len(candidate) < n {
		n = len(candidate)
	}
	frames := n / frameSamples
	if len(pairs) < frames {
		frames = len(pairs)
	}
	out := make([]int16, frames*frameSamples)
	var changed int
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		if pairs[frame][0] == pair || pairs[frame][1] == pair {
			copy(out[off:off+frameSamples], candidate[off:off+frameSamples])
			changed++
		} else {
			copy(out[off:off+frameSamples], enhanced[off:off+frameSamples])
		}
	}
	return out, changed
}

type phase3rContributionOracleGroup struct {
	total      int
	nonBase    int
	positive   int
	deltaSum   float64
	bestMode   map[string]int
	positiveBy map[string]int
	negativeBy map[string]int
}

func phase3rLogContributionOracleSummary(t *testing.T, raw []byte, ref, enhanced, fixedHalf, gainEC25, pitchHalf, pitchThreeQuarter, pitchFiveQuarter, noFCBEnhancement []int16) {
	t.Helper()
	type candidate struct {
		name string
		pcm  []int16
	}
	candidates := []candidate{
		{name: "enhanced", pcm: enhanced},
		{name: "fixed_half", pcm: fixedHalf},
		{name: "ec25", pcm: gainEC25},
		{name: "pitch_half", pcm: pitchHalf},
		{name: "pitch_3q", pcm: pitchThreeQuarter},
		{name: "pitch_5q", pcm: pitchFiveQuarter},
		{name: "no_fcb_enh", pcm: noFCBEnhancement},
	}
	features := phase3rBuildOverhangFeatures(t, raw, enhanced)
	frames := len(ref) / frameSamples
	for _, c := range candidates {
		if cf := len(c.pcm) / frameSamples; cf < frames {
			frames = cf
		}
	}
	if len(features) < frames {
		frames = len(features)
	}

	groups := map[string]*phase3rContributionOracleGroup{}
	modeCounts := map[string]int{}
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		refFrame := ref[off : off+frameSamples]
		baseSNR := phase3rFiniteFrameSNR(refFrame, enhanced[off:off+frameSamples])
		bestName := "enhanced"
		bestSNR := baseSNR
		for _, c := range candidates[1:] {
			snr := phase3rFiniteFrameSNR(refFrame, c.pcm[off:off+frameSamples])
			if snr > bestSNR {
				bestSNR = snr
				bestName = c.name
			}
		}
		delta := bestSNR - baseSNR
		modeCounts[bestName]++
		for _, label := range phase3rContributionFeatureLabels(features[frame]) {
			phase3rAddContributionOracleGroup(groups, label, bestName, delta)
		}
	}

	t.Logf("contribution oracle mode counts: %s", phase3rFormatModeCounts(modeCounts, frames))
	t.Logf("contribution oracle feature summary:")
	for _, label := range []string{
		"all",
		"enh<500", "enh500..2000", "enh>=2000",
		"u<2", "u>=2",
		"short_pitch", "long_pitch",
		"gc32767", "gc<32767",
		"ga036", "no_ga036",
		"ga5gb14", "no_ga5gb14",
		"s/u>=8", "s/u<8",
		"short_gc32767", "loud_short_gc32767",
	} {
		g := groups[label]
		if g == nil || g.total == 0 {
			continue
		}
		t.Logf("  %-20s n=%4d nonbase=%5.1f%% positive=%5.1f%% meanDelta=%+.3f best=%s pos=%s neg=%s",
			label, g.total,
			100*float64(g.nonBase)/float64(g.total),
			100*float64(g.positive)/float64(g.total),
			g.deltaSum/float64(g.total),
			phase3rFormatModeCounts(g.bestMode, g.total),
			phase3rFormatModeCounts(g.positiveBy, g.positive),
			phase3rFormatModeCounts(g.negativeBy, g.total-g.positive))
	}
}

func phase3rFiniteFrameSNR(ref, test []int16) float64 {
	snr := envelopeSNRDB(ref, test)
	if math.IsNaN(snr) || math.IsInf(snr, 0) {
		return -100
	}
	return snr
}

func phase3rContributionFeatureLabels(f phase3rOverhangFeature) []string {
	labels := []string{"all"}
	switch {
	case f.enhancedRMS < 500:
		labels = append(labels, "enh<500")
	case f.enhancedRMS < 2000:
		labels = append(labels, "enh500..2000")
	default:
		labels = append(labels, "enh>=2000")
	}
	if f.uRMS < 2 {
		labels = append(labels, "u<2")
	} else {
		labels = append(labels, "u>=2")
	}
	if f.shortPitch {
		labels = append(labels, "short_pitch")
	} else {
		labels = append(labels, "long_pitch")
	}
	if f.maxGCQ12 >= 32767 {
		labels = append(labels, "gc32767")
	} else {
		labels = append(labels, "gc<32767")
	}
	if f.hasGA036 {
		labels = append(labels, "ga036")
	} else {
		labels = append(labels, "no_ga036")
	}
	if f.hasGA5GB14 {
		labels = append(labels, "ga5gb14")
	} else {
		labels = append(labels, "no_ga5gb14")
	}
	if f.uRMS > 0 && f.sRMS/f.uRMS >= 8 {
		labels = append(labels, "s/u>=8")
	} else {
		labels = append(labels, "s/u<8")
	}
	if f.shortPitch && f.maxGCQ12 >= 32767 {
		labels = append(labels, "short_gc32767")
		if f.enhancedRMS >= 1000 {
			labels = append(labels, "loud_short_gc32767")
		}
	}
	return labels
}

func phase3rAddContributionOracleGroup(groups map[string]*phase3rContributionOracleGroup, label, bestName string, delta float64) {
	g := groups[label]
	if g == nil {
		g = &phase3rContributionOracleGroup{
			bestMode:   map[string]int{},
			positiveBy: map[string]int{},
			negativeBy: map[string]int{},
		}
		groups[label] = g
	}
	g.total++
	g.bestMode[bestName]++
	if bestName != "enhanced" {
		g.nonBase++
	}
	if delta > 0 {
		g.positive++
		g.positiveBy[bestName]++
	} else {
		g.negativeBy[bestName]++
	}
	g.deltaSum += delta
}

func phase3rFormatModeCounts(counts map[string]int, total int) string {
	if total <= 0 || len(counts) == 0 {
		return "<none>"
	}
	type row struct {
		name  string
		count int
	}
	rows := make([]row, 0, len(counts))
	for name, count := range counts {
		if count == 0 {
			continue
		}
		rows = append(rows, row{name: name, count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].name < rows[j].name
	})
	if len(rows) > 4 {
		rows = rows[:4]
	}
	out := ""
	for _, r := range rows {
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf("%s:%d(%.1f%%)", r.name, r.count, 100*float64(r.count)/float64(total))
	}
	if out == "" {
		return "<none>"
	}
	return out
}

type phase3rRuntimeLagFeature struct {
	defaultRMS  float64
	enhancedRMS float64
	frac1       int
	frac2       int
	int1        int
	int2        int
}

func phase3rApplyLagFeatureRule(
	t *testing.T,
	raw []byte,
	local, enhanced []int16,
	lagFor func(feature phase3rRuntimeLagFeature) int,
) []int16 {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if lf := len(local) / frameSamples; lf < frames {
		frames = lf
	}
	if ef := len(enhanced) / frameSamples; ef < frames {
		frames = ef
	}
	out := make([]int16, frames*frameSamples)
	for frame := 0; frame < frames; frame++ {
		var bf bitstream.Frame
		if err := bitstream.Unpack(raw[frame*bitstream.FrameBytes:(frame+1)*bitstream.FrameBytes], &bf); err != nil {
			t.Fatalf("unpack raw frame %d for lag feature rule: %v", frame, err)
		}
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(bf.P1))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(bf.P2), tInt1)
		off := frame * frameSamples
		feature := phase3rRuntimeLagFeature{
			defaultRMS:  envelopeRMS(local[off : off+frameSamples]),
			enhancedRMS: envelopeRMS(enhanced[off : off+frameSamples]),
			frac1:       tFrac1,
			frac2:       tFrac2,
			int1:        tInt1,
			int2:        tInt2,
		}
		copy(out[off:off+frameSamples], phase3rShiftFrame(enhanced[off:off+frameSamples], lagFor(feature)))
	}
	return out
}

func phase3rTemporalBlend(in []int16, prev, cur, next int) []int16 {
	out := make([]int16, len(in))
	den := prev + cur + next
	if den <= 0 {
		copy(out, in)
		return out
	}
	for i := range in {
		acc := cur * int(in[i])
		if prev != 0 {
			j := i - 1
			if j < 0 {
				j = 0
			}
			acc += prev * int(in[j])
		}
		if next != 0 {
			j := i + 1
			if j >= len(in) {
				j = len(in) - 1
			}
			acc += next * int(in[j])
		}
		if acc >= 0 {
			acc += den / 2
		} else {
			acc -= den / 2
		}
		v := acc / den
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}

func phase3rApplyLagRule(
	t *testing.T,
	raw []byte,
	in []int16,
	lagFor func(frame bitstream.Frame, sub1Frac, sub2Frac, sub1Int, sub2Int int) int,
) []int16 {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if inf := len(in) / frameSamples; inf < frames {
		frames = inf
	}
	out := make([]int16, frames*frameSamples)
	for frame := 0; frame < frames; frame++ {
		var bf bitstream.Frame
		if err := bitstream.Unpack(raw[frame*bitstream.FrameBytes:(frame+1)*bitstream.FrameBytes], &bf); err != nil {
			t.Fatalf("unpack raw frame %d for lag rule: %v", frame, err)
		}
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(bf.P1))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(bf.P2), tInt1)
		off := frame * frameSamples
		copy(out[off:off+frameSamples], phase3rShiftFrame(in[off:off+frameSamples], lagFor(bf, tFrac1, tFrac2, tInt1, tInt2)))
	}
	return out
}

func phase3rClampLag(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func phase3rAddLagFeature(groups map[string]*phase3rLagGroup, label string, lag int) {
	g := groups[label]
	if g == nil {
		g = &phase3rLagGroup{counts: map[int]int{}}
		groups[label] = g
	}
	g.total++
	if lag != 0 {
		g.nonzero++
	}
	g.sum += lag
	g.absSum += absInt(lag)
	g.counts[lag]++
}

func phase3rLogHybridGrid(t *testing.T, ref, local, enhanced []int16) {
	t.Helper()
	type selector struct {
		name string
		fn   func(defaultRMS, enhancedRMS float64) bool
	}
	selectors := []selector{
		{name: "enhanced_all", fn: func(_, _ float64) bool { return true }},
		{name: "default_if_enh<200", fn: func(_, e float64) bool { return e >= 200 }},
		{name: "default_if_enh<400", fn: func(_, e float64) bool { return e >= 400 }},
		{name: "default_if_enh<800", fn: func(_, e float64) bool { return e >= 800 }},
		{name: "default_if_enh<1200", fn: func(_, e float64) bool { return e >= 1200 }},
		{name: "default_if_enh/default<0.35", fn: func(d, e float64) bool { return d > 0 && e/d >= 0.35 }},
		{name: "default_if_enh/default<0.50", fn: func(d, e float64) bool { return d > 0 && e/d >= 0.50 }},
		{name: "default_if_enh/default<0.65", fn: func(d, e float64) bool { return d > 0 && e/d >= 0.65 }},
		{name: "default_if_enh/default>1.50", fn: func(d, e float64) bool { return d <= 0 || e/d <= 1.50 }},
		{name: "default_if_enh/default>2.00", fn: func(d, e float64) bool { return d <= 0 || e/d <= 2.00 }},
	}
	t.Logf("runtime hybrid selector grid:")
	t.Logf("%-30s %8s %8s %8s %8s", "selector", "enh", "gSNR", "seg", "corr")
	for _, s := range selectors {
		out, enhancedFrames := phase3rHybridBySelector(local, enhanced, s.fn)
		m := blackboxMeasure(ref, out, 40)
		t.Logf("%-30s %8d %8.2f %8.2f %8.3f", s.name, enhancedFrames, m.globalSNR, m.segSNR, m.corr)
	}
}

func phase3rHybridBySelector(local, enhanced []int16, useEnhanced func(defaultRMS, enhancedRMS float64) bool) ([]int16, int) {
	n := len(local)
	if len(enhanced) < n {
		n = len(enhanced)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	var enhancedFrames int
	for off := 0; off < n; off += frameSamples {
		to := off + frameSamples
		dRMS := envelopeRMS(local[off:to])
		eRMS := envelopeRMS(enhanced[off:to])
		if useEnhanced(dRMS, eRMS) {
			copy(out[off:to], enhanced[off:to])
			enhancedFrames++
		} else {
			copy(out[off:to], local[off:to])
		}
	}
	return out, enhancedFrames
}

type phase3rWorstFrameDetail struct {
	frame  int
	snr    float64
	corr   float64
	refRMS float64
	gotRMS float64
	ratio  float64
	lag    int
	taps   Phase3DiagFrameTaps
}

func phase3rLogWorstFrameDetails(t *testing.T, raw []byte, ref, got []int16, lags []int, limit int) {
	t.Helper()
	frames := len(raw) / bitstream.FrameBytes
	if rf := len(ref) / frameSamples; rf < frames {
		frames = rf
	}
	if gf := len(got) / frameSamples; gf < frames {
		frames = gf
	}
	if lf := len(lags); lf < frames {
		frames = lf
	}
	if frames == 0 {
		return
	}

	var dec Decoder
	details := make([]phase3rWorstFrameDetail, 0, frames)
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		taps, err := dec.DecodeWithTaps(raw[start : start+bitstream.FrameBytes])
		if err != nil {
			t.Fatalf("DecodeWithTaps for worst-frame detail frame %d: %v", frame, err)
		}
		off := frame * frameSamples
		refFrame := ref[off : off+frameSamples]
		gotFrame := got[off : off+frameSamples]
		refRMS := envelopeRMS(refFrame)
		if refRMS < 10 {
			continue
		}
		gotRMS := envelopeRMS(gotFrame)
		ratio := 0.0
		if refRMS > 0 {
			ratio = gotRMS / refRMS
		}
		details = append(details, phase3rWorstFrameDetail{
			frame:  frame,
			snr:    scaleProbeGlobalSNR(refFrame, gotFrame),
			corr:   blackboxCorr(refFrame, gotFrame),
			refRMS: refRMS,
			gotRMS: gotRMS,
			ratio:  ratio,
			lag:    lags[frame],
			taps:   taps,
		})
	}
	sort.Slice(details, func(i, j int) bool {
		if details[i].snr != details[j].snr {
			return details[i].snr < details[j].snr
		}
		return details[i].frame < details[j].frame
	})
	if len(details) > limit {
		details = details[:limit]
	}

	t.Logf("enhanced worst frame detail:")
	for _, d := range details {
		sf0 := d.taps.Sub[0]
		sf1 := d.taps.Sub[1]
		t.Logf("  frame=%4d snr=%6.2f corr=%6.3f oracleLag=%+d refRMS=%7.1f gotRMS=%7.1f ratio=%5.2f P1=%3d t1=%3d/%+d P2=%3d t2=%3d/%+d GA/GB=(%d,%d)(%d,%d) gp=(%5d,%5d) gcQ12=(%5d,%5d) uRMS=(%6.1f,%6.1f) sRMS=(%6.1f,%6.1f) spfRMS=(%6.1f,%6.1f)",
			d.frame, d.snr, d.corr, d.lag, d.refRMS, d.gotRMS, d.ratio,
			d.taps.Frame.P1, sf0.TInt, sf0.TFrac,
			d.taps.Frame.P2, sf1.TInt, sf1.TFrac,
			d.taps.Frame.GA1, d.taps.Frame.GB1, d.taps.Frame.GA2, d.taps.Frame.GB2,
			sf0.GpQ14, sf1.GpQ14, sf0.GcQ12, sf1.GcQ12,
			envelopeRMS(sf0.U[:]), envelopeRMS(sf1.U[:]),
			envelopeRMS(sf0.S[:]), envelopeRMS(sf1.S[:]),
			envelopeRMS(sf0.SPf[:]), envelopeRMS(sf1.SPf[:]))
	}
}

func phase3rDecodeRawEnhanced(t *testing.T, path string, frames int) []int16 {
	t.Helper()
	return phase3rDecodeRawEnhancedCorrections(t, path, frames, 26, 14)
}

func phase3rDecodeRawEnhancedCorrections(t *testing.T, path string, frames int, ecQCorrection, gammaQCorrection int) []int16 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw g729 payload: %v", err)
	}
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for f := 0; f < frames; f++ {
		start := f * 10
		if err := dec.decodeEnvelopeRecoveredWithLogCorrections(raw[start:start+10], false, out[f*frameSamples:(f+1)*frameSamples], ecQCorrection, gammaQCorrection); err != nil {
			t.Fatalf("DecodeEnvelopeRecovered raw frame %d ecQ=%d gammaQ=%d: %v", f, ecQCorrection, gammaQCorrection, err)
		}
	}
	return out
}
