package decoder

import (
	"os"
	"sort"
	"testing"
)

// TestDecoderITUOutputDomainAudit compares final decoder Output against the
// pre-final-scale HP output for each ITU .PST vector. It is diagnostic-only:
// the public decoder still applies the spec-side final ScaleUpSat path, but
// the ITU .PST files may be in the pre-scale post-HP domain for some vectors.
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

	t.Logf("decoder ITU output-domain audit: scope=%s", scope)
	t.Logf("%-10s %7s %10s %10s %10s %10s %10s %10s",
		"vector", "frames", "outRMS", "hpRawRMS", "hpX2RMS", "outExact", "hpRawExact", "best")
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
