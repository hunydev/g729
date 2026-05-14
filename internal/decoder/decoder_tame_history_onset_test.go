package decoder

import (
	"math"
	"os"
	"sort"
	"testing"
)

// TestDecoderTAMEHistoryOnsetAudit is a PST-only onset diagnostic for the TAME
// past-excitation growth issue. It does not assert conformance; it reports
// where final-output over-amplification and local ACB/history growth first
// become material when clean-room internal oracle rows are unavailable.
func TestDecoderTAMEHistoryOnsetAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_HISTORY_ONSET") != "1" {
		t.Skip("set G729_DECODER_TAME_HISTORY_ONSET=1 to run TAME history onset audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_HISTORY_ONSET_VECTOR", "TAME")
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	want := readPSTFrames(t, pstPath)
	if len(frames) != len(want) {
		t.Fatalf("%s: frame count mismatch bit=%d pst=%d", tc.name, len(frames), len(want))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	var dec Decoder
	rows := make([]decoderTAMEHistoryOnsetRow, 0, len(frames))
	firstMaterial := -1
	firstOutRatio125 := -1
	firstOutRatio150 := -1
	firstPersistent150 := -1
	for frame, packed := range frames {
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, frame, err)
		}
		row := decoderTAMEHistoryOnsetRowFromTaps(frame, taps, &want[frame])
		rows = append(rows, row)
		if firstMaterial < 0 && row.stats.maxAbsDelta >= 4096 {
			firstMaterial = frame
		}
		if row.active() && firstOutRatio125 < 0 && row.outRatio >= 1.25 {
			firstOutRatio125 = frame
		}
		if row.active() && firstOutRatio150 < 0 && row.outRatio >= 1.50 {
			firstOutRatio150 = frame
		}
		if row.active() && firstPersistent150 < 0 && decoderTAMEHistoryPersistentOutRatio(rows, frame, 1.50, 4) {
			firstPersistent150 = frame - 3
		}
	}

	topN := decoderITUFrontierTopN()
	t.Logf("decoder TAME history onset: vector=%s frames=%d firstMaxAbs>=4096=%d firstActiveOut/PST>=1.25=%d firstActiveOut/PST>=1.50=%d firstPersistent4Out/PST>=1.50=%d",
		tc.name, len(rows), firstMaterial, firstOutRatio125, firstOutRatio150, firstPersistent150)

	t.Logf("fixed-gain diagnostic window start context")
	decoderTAMEHistoryLogRange(t, rows, 22, 34)
	if firstOutRatio125 >= 0 {
		t.Logf("first active out/PST>=1.25 context")
		decoderTAMEHistoryLogRange(t, rows, firstOutRatio125-4, firstOutRatio125+8)
	}
	if firstOutRatio150 >= 0 {
		t.Logf("first active out/PST>=1.50 context")
		decoderTAMEHistoryLogRange(t, rows, firstOutRatio150-4, firstOutRatio150+8)
	}
	t.Logf("oracle checkpoint context")
	decoderTAMEHistoryLogRange(t, rows, 112, len(rows))

	byRMS := append([]decoderTAMEHistoryOnsetRow(nil), rows...)
	sort.Slice(byRMS, func(i, j int) bool {
		if byRMS[i].stats.sumSqDelta != byRMS[j].stats.sumSqDelta {
			return byRMS[i].stats.sumSqDelta > byRMS[j].stats.sumSqDelta
		}
		return byRMS[i].frame < byRMS[j].frame
	})
	if topN > len(byRMS) {
		topN = len(byRMS)
	}
	t.Logf("top frames by PST-output RMS error")
	decoderTAMEHistoryLogRows(t, byRMS[:topN])

	byPast := append([]decoderTAMEHistoryOnsetRow(nil), rows...)
	sort.Slice(byPast, func(i, j int) bool {
		if byPast[i].pastRMS != byPast[j].pastRMS {
			return byPast[i].pastRMS > byPast[j].pastRMS
		}
		return byPast[i].frame < byPast[j].frame
	})
	if topN > len(byPast) {
		topN = len(byPast)
	}
	t.Logf("top frames by local pre-ACB history RMS")
	decoderTAMEHistoryLogRows(t, byPast[:topN])
}

// TestDecoderTAMEOnsetCandidateRangeAudit is a compact follow-up to the full
// window scans. It compares the known diagnostic-only candidates over fixed
// frame ranges so the TAME onset behavior can be rechecked quickly without
// rerunning the exhaustive 40-second subframe search.
func TestDecoderTAMEOnsetCandidateRangeAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ONSET_CANDIDATE_RANGE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_TAME_ONSET_CANDIDATE_RANGE_AUDIT=1 to run TAME onset candidate range audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_TAME_ONSET_CANDIDATE_VECTOR", "TAME")
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
	badFrames := 0
	for _, bad := range bads {
		if bad {
			badFrames++
		}
	}

	ref := decoderTAMEFlattenPST(wantFrames)
	fixedHalf := phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1}
	candidates := []decoderTAMEOnsetCandidate{
		{
			name: "production",
			out:  phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"}),
		},
		{
			name: "fixed_half_sf52_239",
			out:  phase3eDecodeVariantSubframeWindow(t, bitData, len(frames), 52, 239, fixedHalf),
		},
		{
			name: "fixed_half_frame26_120",
			out:  phase3eDecodeVariantWindow(t, bitData, len(frames), 26, 120, fixedHalf),
		},
		{
			name: "gain_ec_q25_cutover10",
			out: phase3jDecodeVariantCutover(t, bitData, len(frames), 10, phase3jVariant{
				name: "gain_ec_q25",
				mode: phase3jGainECQ25,
			}),
		},
	}
	ranges := []decoderTAMEOnsetRange{
		{name: "all", start: 0, end: len(frames)},
		{name: "window-start", start: 26, end: 34},
		{name: "first-1.25", start: 49, end: 61},
		{name: "first-1.50", start: 68, end: 80},
		{name: "late-oracle", start: 116, end: 128},
	}

	t.Logf("decoder TAME onset candidate range audit: vector=%s frames=%d bad=%d", tc.name, len(frames), badFrames)
	t.Logf("%-24s %-13s %5s %8s %8s %8s %9s %9s %7s",
		"candidate", "range", "frames", "refRMS", "outRMS", "errRMS", "gSNR", "segSNR", "corr")
	for _, candidate := range candidates {
		for _, frameRange := range ranges {
			stats := decoderTAMEComputeOnsetRangeStats(t, ref, candidate.out, frameRange)
			t.Logf("%-24s %-13s %5d %8.1f %8.1f %8.1f %9.2f %9.2f %7.3f",
				candidate.name,
				frameRange.name,
				frameRange.end-frameRange.start,
				stats.refRMS,
				stats.outRMS,
				stats.errRMS,
				stats.metrics.globalSNR,
				stats.metrics.segSNR,
				stats.metrics.corr)
		}
	}
}

type decoderTAMEOnsetCandidate struct {
	name string
	out  []int16
}

type decoderTAMEOnsetRange struct {
	name       string
	start, end int
}

type decoderTAMEOnsetRangeStats struct {
	refRMS  float64
	outRMS  float64
	errRMS  float64
	metrics blackboxMetrics
}

func decoderTAMEComputeOnsetRangeStats(t *testing.T, ref, out []int16, frameRange decoderTAMEOnsetRange) decoderTAMEOnsetRangeStats {
	t.Helper()
	if frameRange.start < 0 || frameRange.end <= frameRange.start {
		t.Fatalf("invalid range %s [%d,%d)", frameRange.name, frameRange.start, frameRange.end)
	}
	lo := frameRange.start * frameSamples
	hi := frameRange.end * frameSamples
	if hi > len(ref) || hi > len(out) {
		t.Fatalf("range %s [%d,%d) exceeds ref=%d out=%d samples", frameRange.name, lo, hi, len(ref), len(out))
	}
	refSlice := ref[lo:hi]
	outSlice := out[lo:hi]
	return decoderTAMEOnsetRangeStats{
		refRMS:  diag4Rms(refSlice),
		outRMS:  diag4Rms(outSlice),
		errRMS:  decoderTAMEPCMErrorRMS(refSlice, outSlice),
		metrics: blackboxMeasure(refSlice, outSlice, 0),
	}
}

func decoderTAMEPCMErrorRMS(ref, out []int16) float64 {
	if len(ref) == 0 || len(ref) != len(out) {
		return 0
	}
	var sumSq int64
	for i := range ref {
		delta := int64(out[i]) - int64(ref[i])
		sumSq += delta * delta
	}
	return math.Sqrt(float64(sumSq) / float64(len(ref)))
}

type decoderTAMEHistoryOnsetRow struct {
	frame      int
	stats      decoderITUFrameStats
	pstRMS     float64
	outRMS     float64
	outRatio   float64
	pastRMS    float64
	pastTail   float64
	vRMS       float64
	pitchRMS   float64
	fixedRMS   float64
	uRMS       float64
	sRMS       float64
	spfRMS     float64
	hpRMS      float64
	maxGpQ14   int16
	maxGcQ12   int16
	predAvgQ10 float64
}

func decoderTAMEHistoryOnsetRowFromTaps(frame int, taps Phase3DiagFrameTaps, want *[frameSamples]int16) decoderTAMEHistoryOnsetRow {
	row := decoderTAMEHistoryOnsetRow{
		frame:  frame,
		stats:  decoderITUCompareFrame(&taps.Output, want),
		pstRMS: decoderHistoryRMS(want[:]),
		outRMS: decoderHistoryRMS(taps.Output[:]),
	}
	row.outRatio = safeRatioFloat64(row.outRMS, row.pstRMS)
	var predSum float64
	for sub := 0; sub < 2; sub++ {
		st := taps.Sub[sub]
		energy := decoderGainFrontierSubEnergy(st)
		row.pastRMS = math.Max(row.pastRMS, decoderHistoryRMS(st.PastExcPreACB[:]))
		row.pastTail = math.Max(row.pastTail, decoderHistoryRMS(st.PastExcPreACB[pastExcLen-subframeLen:]))
		row.vRMS = math.Max(row.vRMS, envelopeRMS(st.V[:]))
		row.pitchRMS = math.Max(row.pitchRMS, energy.pitchRMS)
		row.fixedRMS = math.Max(row.fixedRMS, energy.fixedRMS)
		row.uRMS = math.Max(row.uRMS, envelopeRMS(st.U[:]))
		row.sRMS = math.Max(row.sRMS, envelopeRMS(st.S[:]))
		row.spfRMS = math.Max(row.spfRMS, envelopeRMS(st.SPf[:]))
		row.hpRMS = math.Max(row.hpRMS, envelopeRMS(st.HpOut[:]))
		row.maxGpQ14 = decoderTAMEMaxAbsInt16(row.maxGpQ14, st.GpQ14)
		row.maxGcQ12 = decoderTAMEMaxAbsInt16(row.maxGcQ12, st.GcQ12)
		predSum += float64(st.GainTaps.Predicted) / 1024.0
	}
	row.predAvgQ10 = predSum / 2.0
	return row
}

func (r decoderTAMEHistoryOnsetRow) active() bool {
	return r.pstRMS >= 500
}

func decoderTAMEHistoryPersistentOutRatio(rows []decoderTAMEHistoryOnsetRow, frame int, threshold float64, count int) bool {
	if frame+1 < count {
		return false
	}
	for i := frame - count + 1; i <= frame; i++ {
		if !rows[i].active() || rows[i].outRatio < threshold {
			return false
		}
	}
	return true
}

func decoderTAMEMaxAbsInt16(current, candidate int16) int16 {
	if candidate < 0 {
		candidate = -candidate
	}
	if current < 0 {
		current = -current
	}
	if candidate > current {
		return candidate
	}
	return current
}

func decoderTAMEHistoryLogRange(t *testing.T, rows []decoderTAMEHistoryOnsetRow, start, end int) {
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
	decoderTAMEHistoryLogRows(t, rows[start:end])
}

func decoderTAMEHistoryLogRows(t *testing.T, rows []decoderTAMEHistoryOnsetRow) {
	t.Helper()
	t.Logf("%5s %8s %8s %7s %8s %7s %8s %8s %8s %8s %8s %8s %8s %8s %8s %6s %6s %8s",
		"frame", "pstRMS", "outRMS", "out/PST", "rmsErr", "maxAbs",
		"past", "tail", "v", "pitch", "fixed", "u", "s", "spf", "hp",
		"gp", "gc", "pred")
	for _, r := range rows {
		t.Logf("%5d %8.1f %8.1f %7.3f %8.1f %7d %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %6d %6d %8.2f",
			r.frame,
			r.pstRMS,
			r.outRMS,
			r.outRatio,
			r.stats.rmsDelta(),
			r.stats.maxAbsDelta,
			r.pastRMS,
			r.pastTail,
			r.vRMS,
			r.pitchRMS,
			r.fixedRMS,
			r.uRMS,
			r.sRMS,
			r.spfRMS,
			r.hpRMS,
			r.maxGpQ14,
			r.maxGcQ12,
			r.predAvgQ10)
	}
}
