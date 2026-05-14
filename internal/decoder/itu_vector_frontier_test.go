package decoder

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestDecoderITUVectorFailureFrontier is an opt-in PST-output frontier report.
// It intentionally avoids stage oracle artifacts: input is only ITU .BIT,
// expected output is only companion .PST, and all "got" values come from the
// local decoder final output. The goal is to choose the next local debugging
// target by severity rather than by the first tiny frame-0 difference.
func TestDecoderITUVectorFailureFrontier(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_VECTOR_FRONTIER") != "1" {
		t.Skip("set G729_DECODER_ITU_VECTOR_FRONTIER=1 to run ITU vector failure frontier")
	}

	scope := strings.TrimSpace(os.Getenv("G729_DECODER_ITU_VECTOR_SCOPE"))
	if scope == "" {
		scope = "annexa-good"
	}
	topN := decoderITUFrontierTopN()
	thresholds := []int{1, 16, 256, 1024, 4096}

	t.Logf("decoder ITU vector failure frontier: scope=%s topN=%d", scope, topN)
	for _, tc := range decoderITUValidationCases() {
		if !decoderITUValidationCaseSelected(tc, scope) {
			continue
		}
		report := runDecoderITUVectorFrontierCase(t, tc, topN, thresholds)
		t.Logf("%-10s exact=%s first=%s thresholds=%s worstRMS=%s worstMax=%s",
			tc.name,
			decoderITUPercent(report.exactSamples, report.samples),
			report.first.String(),
			report.thresholdString(),
			report.worstRMSString(),
			report.worstMaxString())
		for i, frame := range report.topFrames {
			t.Logf("  top[%d] frame=%d first=%s exact=%s maxAbs=%d meanAbs=%.2f rms=%.2f signMismatch=%d near1=%s",
				i,
				frame.frame,
				frame.stats.firstDiffString(),
				decoderITUPercent(frame.stats.exactSamples, frameSamples),
				frame.stats.maxAbsDelta,
				frame.stats.meanAbsDelta(),
				frame.stats.rmsDelta(),
				frame.signMismatch,
				decoderITUPercent(frame.near1, frameSamples))
		}
	}
}

type decoderITUFrontierReport struct {
	frames       int
	samples      int
	exactSamples int
	first        decoderITUFrontierPoint
	thresholds   []decoderITUFrontierPoint
	topFrames    []decoderITUFrontierFrame
}

type decoderITUFrontierPoint struct {
	threshold int
	frame     int
	sample    int
	got       int16
	want      int16
	delta     int
}

type decoderITUFrontierFrame struct {
	frame        int
	stats        decoderITUFrameStats
	signMismatch int
	near1        int
}

func runDecoderITUVectorFrontierCase(t *testing.T, tc decoderITUValidationCase, topN int, thresholds []int) decoderITUFrontierReport {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) != len(wantFrames) {
		t.Fatalf("%s: frame count mismatch: bit=%d pst=%d", tc.name, len(frames), len(wantFrames))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch: bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	report := decoderITUFrontierReport{
		first:      decoderITUFrontierPoint{frame: -1, sample: -1},
		thresholds: make([]decoderITUFrontierPoint, len(thresholds)),
	}
	for i, threshold := range thresholds {
		report.thresholds[i] = decoderITUFrontierPoint{
			threshold: threshold,
			frame:     -1,
			sample:    -1,
		}
	}

	var d Decoder
	var out [frameSamples]int16
	var frameReports []decoderITUFrontierFrame
	for fi, packed := range frames {
		if err := d.Decode(packed, bads[fi], out[:]); err != nil {
			t.Fatalf("%s frame %d Decode: %v", tc.name, fi, err)
		}
		stats := decoderITUCompareFrame(&out, &wantFrames[fi])
		report.frames++
		report.samples += frameSamples
		report.exactSamples += stats.exactSamples
		if report.first.frame < 0 && stats.diffSamples() > 0 {
			report.first = decoderITUFrontierPoint{
				threshold: 1,
				frame:     fi,
				sample:    stats.firstSample,
				got:       stats.firstGot,
				want:      stats.firstWant,
				delta:     decoderITUAbsDelta(stats.firstGot, stats.firstWant),
			}
		}
		updateDecoderITUFrontierThresholds(&report, thresholds, fi, &out, &wantFrames[fi])
		if stats.diffSamples() > 0 {
			frameReports = append(frameReports, decoderITUFrontierFrame{
				frame:        fi,
				stats:        stats,
				signMismatch: decoderITUSignMismatchCount(&out, &wantFrames[fi]),
				near1:        decoderITUTraceNearCount(out[:], wantFrames[fi][:], 1),
			})
		}
	}

	sort.Slice(frameReports, func(i, j int) bool {
		left := frameReports[i]
		right := frameReports[j]
		if left.stats.sumSqDelta != right.stats.sumSqDelta {
			return left.stats.sumSqDelta > right.stats.sumSqDelta
		}
		if left.stats.maxAbsDelta != right.stats.maxAbsDelta {
			return left.stats.maxAbsDelta > right.stats.maxAbsDelta
		}
		return left.frame < right.frame
	})
	if topN > len(frameReports) {
		topN = len(frameReports)
	}
	report.topFrames = append(report.topFrames, frameReports[:topN]...)
	return report
}

func updateDecoderITUFrontierThresholds(report *decoderITUFrontierReport, thresholds []int, frame int, got, want *[frameSamples]int16) {
	for sample := 0; sample < frameSamples; sample++ {
		delta := decoderITUAbsDelta(got[sample], want[sample])
		for i, threshold := range thresholds {
			if report.thresholds[i].frame >= 0 || delta < threshold {
				continue
			}
			report.thresholds[i] = decoderITUFrontierPoint{
				threshold: threshold,
				frame:     frame,
				sample:    sample,
				got:       got[sample],
				want:      want[sample],
				delta:     delta,
			}
		}
	}
}

func decoderITUFrontierTopN() int {
	raw := strings.TrimSpace(os.Getenv("G729_DECODER_ITU_VECTOR_FRONTIER_TOP"))
	if raw == "" {
		return 3
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 3
	}
	if n > 20 {
		return 20
	}
	return n
}

func (r decoderITUFrontierReport) thresholdString() string {
	parts := make([]string, 0, len(r.thresholds))
	for _, point := range r.thresholds {
		parts = append(parts, ">="+strconv.Itoa(point.threshold)+":"+point.String())
	}
	return strings.Join(parts, " ")
}

func (r decoderITUFrontierReport) worstRMSString() string {
	if len(r.topFrames) == 0 {
		return "-"
	}
	top := r.topFrames[0]
	return strconv.Itoa(top.frame) + ":" + strconv.FormatFloat(top.stats.rmsDelta(), 'f', 2, 64)
}

func (r decoderITUFrontierReport) worstMaxString() string {
	if len(r.topFrames) == 0 {
		return "-"
	}
	best := r.topFrames[0]
	for _, frame := range r.topFrames[1:] {
		if frame.stats.maxAbsDelta > best.stats.maxAbsDelta {
			best = frame
		}
	}
	return strconv.Itoa(best.frame) + ":" + strconv.Itoa(best.stats.maxAbsDelta)
}

func (p decoderITUFrontierPoint) String() string {
	if p.frame < 0 {
		return "-"
	}
	return strconv.Itoa(p.frame) + ":" + strconv.Itoa(p.sample) +
		"(got=" + strconv.Itoa(int(p.got)) +
		",want=" + strconv.Itoa(int(p.want)) +
		",d=" + strconv.Itoa(p.delta) + ")"
}

func decoderITUSignMismatchCount(got, want *[frameSamples]int16) int {
	var count int
	for i := 0; i < frameSamples; i++ {
		if (got[i] < 0 && want[i] > 0) || (got[i] > 0 && want[i] < 0) {
			count++
		}
	}
	return count
}

func decoderITUAbsDelta(got, want int16) int {
	delta := int(got) - int(want)
	if delta < 0 {
		return -delta
	}
	return delta
}
