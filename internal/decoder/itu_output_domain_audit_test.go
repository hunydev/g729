package decoder

import (
	"os"
	"sort"
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
