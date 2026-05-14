package decoder

import (
	"os"
	"sort"
	"strconv"
	"testing"
)

// TestDecoderITUOutputDomainAudit compares final decoder Output against the
// pre-final-scale HP output for each ITU .PST vector. It is diagnostic-only:
// the public decoder still applies the spec-side final ScaleUpSat path. If
// hp_raw is closer than the final output on stress vectors, read that as a
// pre-scale local-chain over-amplification clue, not as evidence that the PST
// reference changed output domains.
func TestDecoderITUOutputDomainAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_OUTPUT_DOMAIN_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ITU_OUTPUT_DOMAIN_AUDIT=1 to run output domain audit")
	}

	scope := os.Getenv("G729_DECODER_ITU_VECTOR_SCOPE")
	if scope == "" {
		scope = "annexa-good"
	}

	type row struct {
		name    string
		frames  int
		output  decoderITUAggregateStats
		hpRaw   decoderITUAggregateStats
		hpX2    decoderITUAggregateStats
		best    string
		bestRMS float64
	}
	rows := make([]row, 0)
	for _, tc := range decoderITUValidationCases() {
		if !decoderITUValidationCaseSelected(tc, scope) {
			continue
		}
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
		var outStats decoderITUAggregateStats
		var hpRawStats decoderITUAggregateStats
		var hpX2Stats decoderITUAggregateStats
		for fi, packed := range frames {
			taps, err := dec.DecodeWithTaps(packed)
			if err != nil {
				t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, fi, err)
			}
			stages := decoderITUTraceStages(taps)
			outStats.addFrame(decoderITUCompareFrame(&taps.Output, &want[fi]))
			hpRawStats.addFrame(decoderITUCompareFrame(&stages.hpRaw, &want[fi]))
			hpX2Stats.addFrame(decoderITUCompareFrame(&stages.hpX2, &want[fi]))
		}

		best := "output"
		bestRMS := outStats.rmsDelta()
		if hpRawStats.rmsDelta() < bestRMS {
			best = "hp_raw"
			bestRMS = hpRawStats.rmsDelta()
		}
		if hpX2Stats.rmsDelta() < bestRMS {
			best = "hp_x2"
			bestRMS = hpX2Stats.rmsDelta()
		}
		rows = append(rows, row{
			name:    tc.name,
			frames:  len(frames),
			output:  outStats,
			hpRaw:   hpRawStats,
			hpX2:    hpX2Stats,
			best:    best,
			bestRMS: bestRMS,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	t.Logf("decoder ITU pre-final-scale output audit: scope=%s", scope)
	t.Logf("%-10s %7s %10s %10s %10s %10s %10s %10s",
		"vector", "frames", "outRMS", "hpRawRMS", "hpX2RMS", "outExact", "hpRawExact", "closest")
	for _, r := range rows {
		t.Logf("%-10s %7d %10.2f %10.2f %10.2f %10s %10s %10s",
			r.name,
			r.frames,
			r.output.rmsDelta(),
			r.hpRaw.rmsDelta(),
			r.hpX2.rmsDelta(),
			decoderITUPercent(r.output.exactSamples, r.output.samples),
			decoderITUPercent(r.hpRaw.exactSamples, r.hpRaw.samples),
			r.best)
	}
}

// TestDecoderITUPreScaleRatioAudit checks whether the local pre-final-scale
// HP output has the expected half-scale envelope against final-domain .PST PCM.
// Ordinary vectors should mostly put hp_raw near 0.5x PST and final output near
// 1.0x PST. Stress frames where hp_raw approaches 1.0x PST point to upstream
// over-amplification before the final ScaleUpSat step.
func TestDecoderITUPreScaleRatioAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_PRESCALE_RATIO_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ITU_PRESCALE_RATIO_AUDIT=1 to run pre-scale ratio audit")
	}

	scope := os.Getenv("G729_DECODER_ITU_VECTOR_SCOPE")
	if scope == "" {
		scope = "annexa-good"
	}

	const activePSTRMS = 500.0
	type row struct {
		name      string
		frames    int
		active    int
		hpMedian  float64
		hpMean    float64
		hpP10     float64
		hpP90     float64
		outMedian float64
		outMean   float64
		outP10    float64
		outP90    float64
		hpHalf    int
		hpOne     int
		outOne    int
		outHigh   int
	}
	rows := make([]row, 0)
	for _, tc := range decoderITUValidationCases() {
		if !decoderITUValidationCaseSelected(tc, scope) {
			continue
		}
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
		hpRatios := make([]float64, 0, len(frames))
		outRatios := make([]float64, 0, len(frames))
		for fi, packed := range frames {
			taps, err := dec.DecodeWithTaps(packed)
			if err != nil {
				t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, fi, err)
			}
			pstFrame := want[fi]
			pstRMS := envelopeRMS(pstFrame[:])
			if pstRMS < activePSTRMS {
				continue
			}
			stages := decoderITUTraceStages(taps)
			hpRatios = append(hpRatios, envelopeRMS(stages.hpRaw[:])/pstRMS)
			outRatios = append(outRatios, envelopeRMS(taps.Output[:])/pstRMS)
		}

		sort.Float64s(hpRatios)
		sort.Float64s(outRatios)
		rows = append(rows, row{
			name:      tc.name,
			frames:    len(frames),
			active:    len(hpRatios),
			hpMedian:  envelopePercentile(hpRatios, 0.5),
			hpMean:    decoderITUFloatMean(hpRatios),
			hpP10:     envelopePercentile(hpRatios, 0.1),
			hpP90:     envelopePercentile(hpRatios, 0.9),
			outMedian: envelopePercentile(outRatios, 0.5),
			outMean:   decoderITUFloatMean(outRatios),
			outP10:    envelopePercentile(outRatios, 0.1),
			outP90:    envelopePercentile(outRatios, 0.9),
			hpHalf:    decoderITUFloatRangeCount(hpRatios, 0.4, 0.6),
			hpOne:     decoderITUFloatRangeCount(hpRatios, 0.8, 1.2),
			outOne:    decoderITUFloatRangeCount(outRatios, 0.8, 1.2),
			outHigh:   decoderITUFloatGreaterCount(outRatios, 1.5),
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	t.Logf("decoder ITU pre-scale ratio audit: scope=%s activePSTRMS>=%.0f", scope, activePSTRMS)
	t.Logf("%-10s %7s %7s %8s %8s %8s %8s %9s %8s %8s %8s %10s %9s %9s %10s",
		"vector", "frames", "active", "hpMed", "hpMean", "hpP10", "hpP90",
		"outMed", "outMean", "outP10", "outP90", "hp0.5x", "hp1.0x", "out1.0x", "out>1.5x")
	for _, r := range rows {
		t.Logf("%-10s %7d %7d %8.3f %8.3f %8.3f %8.3f %9.3f %8.3f %8.3f %8.3f %10d %9d %9d %10d",
			r.name, r.frames, r.active,
			r.hpMedian, r.hpMean, r.hpP10, r.hpP90,
			r.outMedian, r.outMean, r.outP10, r.outP90,
			r.hpHalf, r.hpOne, r.outOne, r.outHigh)
	}
}

// TestDecoderITUPreScaleStageTimelineAudit reports frame-level stage envelopes
// against final-domain .PST PCM for one vector. It keeps the diagnostic local
// and clean-room: .BIT/.PST numeric vectors plus local taps only.
func TestDecoderITUPreScaleStageTimelineAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_PRESCALE_STAGE_TIMELINE") != "1" {
		t.Skip("set G729_DECODER_ITU_PRESCALE_STAGE_TIMELINE=1 to run pre-scale stage timeline audit")
	}

	vector := os.Getenv("G729_DECODER_ITU_PRESCALE_STAGE_VECTOR")
	if vector == "" {
		vector = "TAME"
	}
	tc, ok := decoderITUValidationCaseByName(vector)
	if !ok {
		t.Fatalf("unknown decoder ITU vector %q", vector)
	}
	limit := decoderITUEnvInt("G729_DECODER_ITU_PRESCALE_STAGE_TOP", 20)
	if limit <= 0 {
		limit = 20
	}
	const activePSTRMS = 500.0

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

	rows := make([]decoderITUStageTimelineRow, 0, len(frames))
	firstHPHot := -1
	firstOutHot := -1
	var dec Decoder
	for fi, packed := range frames {
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, fi, err)
		}
		pstFrame := want[fi]
		pstRMS := envelopeRMS(pstFrame[:])
		if pstRMS < activePSTRMS {
			continue
		}
		stages := envelopeStageSummary(taps)
		hpRatio := stages.hpRMS / pstRMS
		outRatio := stages.outRMS / pstRMS
		if firstHPHot < 0 && hpRatio >= 0.8 {
			firstHPHot = fi
		}
		if firstOutHot < 0 && outRatio >= 1.5 {
			firstOutHot = fi
		}
		rows = append(rows, decoderITUStageTimelineRow{
			frame:    fi,
			pstRMS:   pstRMS,
			uRMS:     stages.uRMS,
			sRMS:     stages.sRMS,
			spfRMS:   stages.spfRMS,
			hpRMS:    stages.hpRMS,
			outRMS:   stages.outRMS,
			sRatio:   stages.sRMS / pstRMS,
			spfRatio: stages.spfRMS / pstRMS,
			hpRatio:  hpRatio,
			outRatio: outRatio,
			spfToS:   safeRatioFloat64(stages.spfRMS, stages.sRMS),
			hpToSPf:  safeRatioFloat64(stages.hpRMS, stages.spfRMS),
			outToHP:  safeRatioFloat64(stages.outRMS, stages.hpRMS),
			gpMax:    stages.gpMax,
			gcMax:    stages.gcMax,
			predAvg:  stages.predictedAvgQ10 / 1024.0,
			logGain:  stages.logGainAvgQ10 / 1024.0,
		})
	}

	hotRows := append([]decoderITUStageTimelineRow(nil), rows...)
	sort.Slice(hotRows, func(i, j int) bool {
		if hotRows[i].outRatio == hotRows[j].outRatio {
			return hotRows[i].hpRatio > hotRows[j].hpRatio
		}
		return hotRows[i].outRatio > hotRows[j].outRatio
	})
	if limit > len(hotRows) {
		limit = len(hotRows)
	}

	t.Logf("decoder ITU pre-scale stage timeline: vector=%s activePSTRMS>=%.0f active=%d firstHP>=0.8=%d firstOut>=1.5=%d",
		tc.name, activePSTRMS, len(rows), firstHPHot, firstOutHot)
	t.Logf("top frames by output/PST ratio:")
	decoderITULogStageTimelineRows(t, hotRows[:limit])
	if firstOutHot >= 0 {
		start := firstOutHot - 3
		if start < 0 {
			start = 0
		}
		end := firstOutHot + 4
		window := make([]decoderITUStageTimelineRow, 0, end-start)
		for _, r := range rows {
			if r.frame >= start && r.frame < end {
				window = append(window, r)
			}
		}
		t.Logf("first output-hot window:")
		decoderITULogStageTimelineRows(t, window)
	}
}

type decoderITUStageTimelineRow struct {
	frame    int
	pstRMS   float64
	uRMS     float64
	sRMS     float64
	spfRMS   float64
	hpRMS    float64
	outRMS   float64
	sRatio   float64
	spfRatio float64
	hpRatio  float64
	outRatio float64
	spfToS   float64
	hpToSPf  float64
	outToHP  float64
	gpMax    float64
	gcMax    float64
	predAvg  float64
	logGain  float64
}

type decoderITUAggregateStats struct {
	samples      int
	exactSamples int
	sumSqDelta   int64
	maxAbsDelta  int
}

func (s *decoderITUAggregateStats) addFrame(frame decoderITUFrameStats) {
	s.samples += frameSamples
	s.exactSamples += frame.exactSamples
	s.sumSqDelta += frame.sumSqDelta
	if frame.maxAbsDelta > s.maxAbsDelta {
		s.maxAbsDelta = frame.maxAbsDelta
	}
}

func (s decoderITUAggregateStats) rmsDelta() float64 {
	if s.samples <= 0 || s.sumSqDelta <= 0 {
		return 0
	}
	return decoderGainCandidateRMS(s.sumSqDelta, s.samples)
}

func decoderITUFloatMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func decoderITUFloatRangeCount(values []float64, lo, hi float64) int {
	var count int
	for _, v := range values {
		if v >= lo && v <= hi {
			count++
		}
	}
	return count
}

func decoderITUFloatGreaterCount(values []float64, threshold float64) int {
	var count int
	for _, v := range values {
		if v > threshold {
			count++
		}
	}
	return count
}

func decoderITUEnvInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func decoderITULogStageTimelineRows(t *testing.T, rows []decoderITUStageTimelineRow) {
	t.Helper()
	t.Logf("%5s %8s %8s %8s %8s %8s %8s %7s %7s %7s %7s %7s %7s %7s %6s %6s %8s %8s",
		"frame", "pstRMS", "uRMS", "sRMS", "spfRMS", "hpRMS", "outRMS",
		"s/PST", "spf/PST", "hp/PST", "out/PST", "spf/s", "hp/spf",
		"out/hp", "gpMax", "gcMax", "predAvg", "logGain")
	for _, r := range rows {
		t.Logf("%5d %8.1f %8.1f %8.1f %8.1f %8.1f %8.1f %7.3f %7.3f %7.3f %7.3f %7.3f %7.3f %7.3f %6.3f %6.3f %8.1f %8.1f",
			r.frame, r.pstRMS, r.uRMS, r.sRMS, r.spfRMS, r.hpRMS, r.outRMS,
			r.sRatio, r.spfRatio, r.hpRatio, r.outRatio, r.spfToS, r.hpToSPf,
			r.outToHP, r.gpMax, r.gcMax, r.predAvg, r.logGain)
	}
}
