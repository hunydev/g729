package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPhase3bhSubframeLagOracleAudit measures how much of the enhanced
// decoder gap is explained by timing at frame vs subframe granularity. FFmpeg
// is used only as an executable black-box decoder for numeric references.
func TestPhase3bhSubframeLagOracleAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_SUBFRAME_LAG_ORACLE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_SUBFRAME_LAG_ORACLE_AUDIT=1 to run subframe lag-oracle audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	datasets := []phase3bfGainGridDataset{
		phase3bfLoadG192GainGridDataset(t, "SPEECH.BIT"),
	}
	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(rawPath); err == nil {
		datasets = append(datasets, phase3bfLoadRawGainGridDataset(t, "Asterisk", rawPath))
	} else {
		t.Logf("Asterisk subframe lag-oracle audit skipped: %v", err)
	}

	for _, ds := range datasets {
		enhanced := phase3rDecodeRawEnhanced(t, ds.rawPath, ds.frames)
		frame4 := phase3rFrameLagOracle(ds.ref, enhanced, 4)
		frame20 := phase3rFrameLagOracle(ds.ref, enhanced, 20)
		sub4, sub4Lags := phase3bhSubframeLagOracle(ds.ref, enhanced, 4)
		sub8, sub8Lags := phase3bhSubframeLagOracle(ds.ref, enhanced, 8)
		sub8RMS := phase3bhMatchSubframeRMS(sub8, ds.ref)

		rows := []phase3bhLagRow{
			{name: "enhanced", pcm: enhanced},
			{name: "frame_lag_oracle_4", pcm: frame4},
			{name: "frame_lag_oracle_20", pcm: frame20},
			{name: "subframe_lag_oracle_4", pcm: sub4},
			{name: "subframe_lag_oracle_8", pcm: sub8},
			{name: "subframe_lag_rms_oracle_8", pcm: sub8RMS},
		}
		phase3bhLogLagRows(t, ds.label, ds.ref, rows)
		t.Logf("%s subframe lag histogram max4: %s", ds.label, phase3rFormatLagHistogram(phase3bhLagCounts(sub4Lags), len(sub4Lags)))
		t.Logf("%s subframe lag histogram max8: %s", ds.label, phase3rFormatLagHistogram(phase3bhLagCounts(sub8Lags), len(sub8Lags)))
	}
}

type phase3bhLagRow struct {
	name string
	pcm  []int16
}

func phase3bhSubframeLagOracle(ref, test []int16, maxLag int) ([]int16, []int) {
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	n -= n % frameSamples
	out := make([]int16, n)
	lags := make([]int, 0, n/subframeLen)
	for off := 0; off < n; off += subframeLen {
		to := off + subframeLen
		refSub := ref[off:to]
		testSub := test[off:to]
		bestLag := 0
		bestSNR := math.Inf(-1)
		for lag := -maxLag; lag <= maxLag; lag++ {
			shifted := phase3rShiftFrame(testSub, lag)
			snr := envelopeSNRDB(refSub, shifted)
			if snr > bestSNR {
				bestSNR = snr
				bestLag = lag
			}
		}
		copy(out[off:to], phase3rShiftFrame(testSub, bestLag))
		lags = append(lags, bestLag)
	}
	return out, lags
}

func phase3bhMatchSubframeRMS(test, ref []int16) []int16 {
	n := len(test)
	if len(ref) < n {
		n = len(ref)
	}
	n -= n % subframeLen
	out := make([]int16, n)
	for off := 0; off < n; off += subframeLen {
		to := off + subframeLen
		testSub := test[off:to]
		refSub := ref[off:to]
		testRMS := envelopeRMS(testSub)
		refRMS := envelopeRMS(refSub)
		if testRMS <= 0 || refRMS <= 0 {
			copy(out[off:to], testSub)
			continue
		}
		scale := refRMS / testRMS
		for i, sample := range testSub {
			out[off+i] = phase3bbScaleSample(sample, scale)
		}
	}
	return out
}

func phase3bhLogLagRows(t *testing.T, label string, ref []int16, rows []phase3bhLagRow) {
	t.Helper()
	base := blackboxMeasure(ref, rows[0].pcm, 40)
	baseEnv := phase3pEnvelopeCompare(ref, rows[0].pcm)
	t.Logf("Phase 3bh subframe lag-oracle audit - %s", label)
	t.Logf("baseline enhanced: gSNR=%.2f seg=%.2f corr=%.3f ratioMed=%.3f low<0.5=%d",
		base.globalSNR, base.segSNR, base.corr, baseEnv.ratioMedian, baseEnv.lowRatioFrames)
	t.Logf("%-30s %9s %9s %8s %9s %10s %9s %9s",
		"variant", "gSNR", "segSNR", "corr", "ratioMed", "low<0.5", "deltaG", "deltaS")
	for _, row := range rows {
		m := blackboxMeasure(ref, row.pcm, 40)
		env := phase3pEnvelopeCompare(ref, row.pcm)
		t.Logf("%-30s %9.2f %9.2f %8.3f %9.3f %10d %+9.2f %+9.2f",
			row.name, m.globalSNR, m.segSNR, m.corr, env.ratioMedian,
			env.lowRatioFrames, m.globalSNR-base.globalSNR, m.segSNR-base.segSNR)
	}
}

func phase3bhLagCounts(lags []int) map[int]int {
	counts := make(map[int]int)
	for _, lag := range lags {
		counts[lag]++
	}
	return counts
}
