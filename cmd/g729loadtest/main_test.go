package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestMergeResultsComputesCapacityMetrics(t *testing.T) {
	results := []streamResult{
		{
			Frames:              2,
			ProcessingTotal:     2 * time.Millisecond,
			ProcessingDurations: []int64{500_000, 1_500_000},
		},
		{
			Frames:              2,
			ProcessingTotal:     2 * time.Millisecond,
			ProcessingDurations: []int64{700_000, 1_300_000},
		},
	}

	report := mergeResults(results, 20*time.Millisecond, 2, 1, opEncode, "core", true, time.Millisecond)
	if report.Frames != 4 {
		t.Fatalf("frames = %d, want 4", report.Frames)
	}
	if report.MediaDuration != 40*time.Millisecond {
		t.Fatalf("media duration = %s, want 40ms", report.MediaDuration)
	}
	if report.CodecRTF != 0.1 {
		t.Fatalf("codec rtf = %.6f, want 0.1", report.CodecRTF)
	}
	if report.CodecStreamsPerCore != 10 {
		t.Fatalf("streams/core = %.2f, want 10", report.CodecStreamsPerCore)
	}
	if report.P99DeadlineRatio != 0.15 {
		t.Fatalf("p99 deadline = %.6f, want 0.15", report.P99DeadlineRatio)
	}
}

func TestPrintReportJSONShape(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	printReport(loadReport{
		Streams:             1,
		GOMAXPROCS:          1,
		Mode:                "encode",
		Profile:             "core",
		Realtime:            true,
		Elapsed:             20 * time.Millisecond,
		Frames:              2,
		MediaDuration:       20 * time.Millisecond,
		ProcessingTotal:     2 * time.Millisecond,
		CodecRTF:            0.1,
		CodecStreamsPerCore: 10,
		ProcessingStats: durationStats{
			P99: 500 * time.Microsecond,
		},
		P99DeadlineRatio: 0.05,
	}, true)

	var got loadReportJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, buf.String())
	}
	if got.Mode != "encode" || got.Profile != "core" || got.Frames != 2 {
		t.Fatalf("json report = %+v, want encode/core counters", got)
	}
	if got.ProcessingStats.P99Nanos != int64(500*time.Microsecond) || got.ProcessingStats.P99US != 500 {
		t.Fatalf("processing stats = %+v, want p99 500us", got.ProcessingStats)
	}
}
