package decoder

import (
	"math"
	"os"
	"testing"
)

// TestDecoderTAMEFixedFeedbackPropagationAudit explains why the diagnostic
// fixed_gain_half window stabilizes TAME: the direct fixed contribution is
// damped first, then the changed U samples are fed back through pastExc and the
// later adaptive-codebook vector. It is diagnostic-only and does not assert
// conformance.
func TestDecoderTAMEFixedFeedbackPropagationAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_FIXED_FEEDBACK_AUDIT") != "1" {
		t.Skip("set G729_DECODER_TAME_FIXED_FEEDBACK_AUDIT=1 to run TAME fixed-feedback propagation audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_TAME_FIXED_FEEDBACK_VECTOR", "TAME")
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) != len(wantFrames) {
		t.Fatalf("%s: frame count mismatch bit=%d pst=%d", tc.name, len(frames), len(wantFrames))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	startSubframe := decoderITUEnvInt("G729_DECODER_TAME_FIXED_FEEDBACK_START_SUBFRAME", 26*2)
	endSubframe := decoderITUEnvInt("G729_DECODER_TAME_FIXED_FEEDBACK_END_SUBFRAME", 120*2)
	if startSubframe < 0 || endSubframe > len(frames)*2 || startSubframe >= endSubframe {
		t.Fatalf("invalid subframe window [%d,%d) for %d frames", startSubframe, endSubframe, len(frames))
	}

	fixedHalf := phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1}
	prodOut, prodRows := decoderHistoryDecodeWindow(t, bitData, len(frames), 0, 0, phase3eVariant{name: "production"})
	candOut, candRows := decoderHistoryDecodeWindow(t, bitData, len(frames), startSubframe, endSubframe, fixedHalf)
	if len(prodRows) != len(candRows) {
		t.Fatalf("row count mismatch prod=%d cand=%d", len(prodRows), len(candRows))
	}
	baseline := decodeVariant(t, bitData, len(frames), nil, nil)
	if !phase3eEqualPCM(baseline, prodOut) {
		t.Fatalf("fixed-feedback production mirror diverges from Decoder.Decode baseline")
	}

	ref := decoderTAMEFlattenPST(wantFrames)
	ranges := []decoderTAMEOnsetRange{
		{name: "all", start: 0, end: len(frames)},
		{name: "window-start", start: 26, end: 34},
		{name: "root-49-72", start: 49, end: 72},
		{name: "first-1.25", start: 49, end: 61},
		{name: "first-1.50", start: 68, end: 80},
		{name: "pre-late", start: 112, end: 116},
		{name: "late-oracle", start: 116, end: 128},
	}
	prodByRange := make(map[string]decoderTAMEOnsetRangeStats, len(ranges))
	for _, frameRange := range ranges {
		prodByRange[frameRange.name] = decoderTAMEComputeOnsetRangeStats(t, ref, prodOut, frameRange)
	}

	t.Logf("decoder TAME fixed-feedback propagation audit: vector=%s frames=%d fixedWindow=[%d,%d)",
		tc.name, len(frames), startSubframe, endSubframe)
	t.Logf("%-15s %-16s %9s %9s %9s %9s %9s",
		"candidate", "range", "gSNR", "deltaG", "outRMS", "errRMS", "corr")
	for _, candidate := range []struct {
		name string
		out  []int16
	}{
		{name: "production", out: prodOut},
		{name: "fixed_gain_half", out: candOut},
	} {
		for _, frameRange := range ranges {
			stats := decoderTAMEComputeOnsetRangeStats(t, ref, candidate.out, frameRange)
			prod := prodByRange[frameRange.name]
			t.Logf("%-15s %-16s %9.2f %+9.2f %9.1f %9.1f %9.3f",
				candidate.name,
				frameRange.name,
				stats.metrics.globalSNR,
				stats.metrics.globalSNR-prod.metrics.globalSNR,
				stats.outRMS,
				stats.errRMS,
				stats.metrics.corr)
		}
	}

	rows := decoderTAMEFixedFeedbackRows(prodRows, candRows)
	decoderTAMEFixedFeedbackLogSummary(t, rows, startSubframe, endSubframe)
	decoderTAMEFixedFeedbackLogLag(t, rows, startSubframe, endSubframe)

	t.Logf("window-start context")
	decoderTAMEFixedFeedbackLogRange(t, rows, startSubframe-4, startSubframe+16)
	t.Logf("root 49..72 context")
	decoderTAMEFixedFeedbackLogRange(t, rows, 49*2, 72*2)
	t.Logf("late oracle context")
	decoderTAMEFixedFeedbackLogRange(t, rows, 116*2, len(rows))
}

type decoderTAMEFixedFeedbackRow struct {
	globalSubframe int
	frame          int
	sub            int
	inWindow       bool
	prod           decoderHistorySubframeMetrics
	cand           decoderHistorySubframeMetrics
	pastRatio      float64
	vRatio         float64
	pitchRatio     float64
	fixedRatio     float64
	uRatio         float64
	sRatio         float64
	pastDelta      float64
	vDelta         float64
	pitchDelta     float64
	fixedDelta     float64
	uDelta         float64
	sDelta         float64
}

func decoderTAMEFixedFeedbackRows(prodRows, candRows []decoderHistorySubframeMetrics) []decoderTAMEFixedFeedbackRow {
	rows := make([]decoderTAMEFixedFeedbackRow, 0, len(prodRows))
	for i := range prodRows {
		rows = append(rows, decoderTAMEFixedFeedbackRow{
			globalSubframe: prodRows[i].globalSubframe,
			frame:          prodRows[i].frame,
			sub:            prodRows[i].sub,
			inWindow:       candRows[i].inWindow,
			prod:           prodRows[i],
			cand:           candRows[i],
			pastRatio:      safeRatioFloat64(candRows[i].pastRMS, prodRows[i].pastRMS),
			vRatio:         safeRatioFloat64(candRows[i].vRMS, prodRows[i].vRMS),
			pitchRatio:     safeRatioFloat64(candRows[i].pitchRMS, prodRows[i].pitchRMS),
			fixedRatio:     safeRatioFloat64(candRows[i].fixedRMS, prodRows[i].fixedRMS),
			uRatio:         safeRatioFloat64(candRows[i].uRMS, prodRows[i].uRMS),
			sRatio:         safeRatioFloat64(candRows[i].sRMS, prodRows[i].sRMS),
			pastDelta:      prodRows[i].pastRMS - candRows[i].pastRMS,
			vDelta:         prodRows[i].vRMS - candRows[i].vRMS,
			pitchDelta:     prodRows[i].pitchRMS - candRows[i].pitchRMS,
			fixedDelta:     prodRows[i].fixedRMS - candRows[i].fixedRMS,
			uDelta:         prodRows[i].uRMS - candRows[i].uRMS,
			sDelta:         prodRows[i].sRMS - candRows[i].sRMS,
		})
	}
	return rows
}

func decoderTAMEFixedFeedbackLogSummary(t *testing.T, rows []decoderTAMEFixedFeedbackRow, startSubframe, endSubframe int) {
	t.Helper()
	windows := []struct {
		name  string
		start int
		end   int
	}{
		{name: "pre-window", start: startSubframe - 8, end: startSubframe},
		{name: "window-start", start: startSubframe, end: startSubframe + 16},
		{name: "root-49-72", start: 49 * 2, end: 72 * 2},
		{name: "first-1.25", start: 49 * 2, end: 61 * 2},
		{name: "first-1.50", start: 68 * 2, end: 80 * 2},
		{name: "pre-late", start: 112 * 2, end: 116 * 2},
		{name: "late-oracle", start: 116 * 2, end: len(rows)},
		{name: "full-window", start: startSubframe, end: endSubframe},
	}
	t.Logf("feedback RMS summary")
	t.Logf("%-13s %8s %8s %8s %8s %8s %8s %8s %8s %8s",
		"window", "subfrm", "fixR", "uR", "pastR", "vR", "pitchR", "dFix", "dU", "dV")
	for _, window := range windows {
		start := window.start
		end := window.end
		if start < 0 {
			start = 0
		}
		if end > len(rows) {
			end = len(rows)
		}
		if start >= end {
			continue
		}
		summary := decoderTAMEFixedFeedbackSummarize(rows[start:end])
		t.Logf("%-13s %8d %8.3f %8.3f %8.3f %8.3f %8.3f %8.1f %8.1f %8.1f",
			window.name,
			summary.count,
			summary.fixedRatio,
			summary.uRatio,
			summary.pastRatio,
			summary.vRatio,
			summary.pitchRatio,
			summary.fixedDelta,
			summary.uDelta,
			summary.vDelta)
	}
}

type decoderTAMEFixedFeedbackSummary struct {
	count      int
	pastRatio  float64
	vRatio     float64
	pitchRatio float64
	fixedRatio float64
	uRatio     float64
	fixedDelta float64
	uDelta     float64
	vDelta     float64
}

func decoderTAMEFixedFeedbackSummarize(rows []decoderTAMEFixedFeedbackRow) decoderTAMEFixedFeedbackSummary {
	var summary decoderTAMEFixedFeedbackSummary
	summary.count = len(rows)
	if len(rows) == 0 {
		return summary
	}
	for _, row := range rows {
		summary.pastRatio += row.pastRatio
		summary.vRatio += row.vRatio
		summary.pitchRatio += row.pitchRatio
		summary.fixedRatio += row.fixedRatio
		summary.uRatio += row.uRatio
		summary.fixedDelta += row.fixedDelta
		summary.uDelta += row.uDelta
		summary.vDelta += row.vDelta
	}
	n := float64(len(rows))
	summary.pastRatio /= n
	summary.vRatio /= n
	summary.pitchRatio /= n
	summary.fixedRatio /= n
	summary.uRatio /= n
	summary.fixedDelta /= n
	summary.uDelta /= n
	summary.vDelta /= n
	return summary
}

func decoderTAMEFixedFeedbackLogLag(t *testing.T, rows []decoderTAMEFixedFeedbackRow, startSubframe, endSubframe int) {
	t.Helper()
	t.Logf("fixed-delta propagation lag")
	t.Logf("%5s %8s %9s %9s %9s %9s %9s",
		"lag", "count", "corrFixU", "corrFixV", "corrFixPast", "avgU", "avgV")
	for _, lag := range []int{0, 1, 2, 4, 8, 16, 32, 64, 128} {
		var fixedDeltas []float64
		var uDeltas []float64
		var vDeltas []float64
		var pastDeltas []float64
		var sumU float64
		var sumV float64
		for i := startSubframe; i < endSubframe && i+lag < len(rows); i++ {
			fixedDeltas = append(fixedDeltas, rows[i].fixedDelta)
			uDeltas = append(uDeltas, rows[i+lag].uDelta)
			vDeltas = append(vDeltas, rows[i+lag].vDelta)
			pastDeltas = append(pastDeltas, rows[i+lag].pastDelta)
			sumU += rows[i+lag].uDelta
			sumV += rows[i+lag].vDelta
		}
		if len(fixedDeltas) == 0 {
			continue
		}
		t.Logf("%5d %8d %9.3f %9.3f %9.3f %9.1f %9.1f",
			lag,
			len(fixedDeltas),
			decoderTAMEFixedFeedbackPearson(fixedDeltas, uDeltas),
			decoderTAMEFixedFeedbackPearson(fixedDeltas, vDeltas),
			decoderTAMEFixedFeedbackPearson(fixedDeltas, pastDeltas),
			sumU/float64(len(fixedDeltas)),
			sumV/float64(len(fixedDeltas)))
	}
}

func decoderTAMEFixedFeedbackPearson(x, y []float64) float64 {
	if len(x) == 0 || len(x) != len(y) {
		return 0
	}
	var sumX, sumY float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(len(x))
	meanY := sumY / float64(len(y))
	var num, denX, denY float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	if denX == 0 || denY == 0 {
		return 0
	}
	return num / math.Sqrt(denX*denY)
}

func decoderTAMEFixedFeedbackLogRange(t *testing.T, rows []decoderTAMEFixedFeedbackRow, start, end int) {
	t.Helper()
	if start < 0 {
		start = 0
	}
	if end > len(rows) {
		end = len(rows)
	}
	if start >= end {
		return
	}
	decoderTAMEFixedFeedbackLogRows(t, rows[start:end])
}

func decoderTAMEFixedFeedbackLogRows(t *testing.T, rows []decoderTAMEFixedFeedbackRow) {
	t.Helper()
	t.Logf("%5s %5s %3s %3s %8s %8s %7s %8s %8s %7s %8s %8s %7s %8s %8s %7s %8s %8s %7s %6s %6s",
		"sf", "frame", "sub", "win",
		"pPast", "cPast", "c/p",
		"pV", "cV", "c/p",
		"pPitch", "cPitch", "c/p",
		"pFix", "cFix", "c/p",
		"pU", "cU", "c/p",
		"pGp", "cGp")
	for _, r := range rows {
		t.Logf("%5d %5d %3d %3t %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %6d %6d",
			r.globalSubframe,
			r.frame,
			r.sub,
			r.inWindow,
			r.prod.pastRMS,
			r.cand.pastRMS,
			r.pastRatio,
			r.prod.vRMS,
			r.cand.vRMS,
			r.vRatio,
			r.prod.pitchRMS,
			r.cand.pitchRMS,
			r.pitchRatio,
			r.prod.fixedRMS,
			r.cand.fixedRMS,
			r.fixedRatio,
			r.prod.uRMS,
			r.cand.uRMS,
			r.uRatio,
			r.prod.gpQ14,
			r.cand.gpQ14)
	}
}
