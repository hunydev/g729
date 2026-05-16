package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEvaluatePassesValidReport(t *testing.T) {
	rep := validReport()
	clean := false
	rep.Repository.VCSModified = &clean

	got := evaluate(rep, options{
		schemaVersion:   defaultSchemaVersion,
		payloadType:     18,
		ptime:           20,
		minPackets:      1,
		minFrames:       2,
		minStreams:      1,
		requireAnnexBNo: true,
		requireStrictTS: true,
		requireDecode:   true,
		requireCleanVCS: true,
		minDuration:     0.01,
		minRMS:          1,
		maxNearClip:     0,
		maxHardClip:     0,
	})
	if !got.OK || len(got.Failures) != 0 {
		t.Fatalf("verdict = %+v, want pass", got)
	}
}

func TestEvaluateReportsFailures(t *testing.T) {
	rep := validReport()
	rep.Negotiation.AnnexB = "yes"
	rep.RTP.Decode.NearClip = 2

	got := evaluate(rep, options{
		schemaVersion:   defaultSchemaVersion,
		payloadType:     18,
		ptime:           20,
		minPackets:      1,
		minFrames:       2,
		minStreams:      1,
		requireAnnexBNo: true,
		requireStrictTS: true,
		requireDecode:   true,
		minRMS:          1,
		maxNearClip:     0,
		maxHardClip:     0,
	})
	if got.OK {
		t.Fatalf("verdict passed, want failure")
	}
	joined := strings.Join(got.Failures, "\n")
	if !strings.Contains(joined, "annexb") || !strings.Contains(joined, "nearClip") {
		t.Fatalf("failures = %q, want annexb and nearClip failures", joined)
	}
}

func TestReadReportAndPrintJSONVerdict(t *testing.T) {
	rep := validReport()
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readReport(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != defaultSchemaVersion || got.RTP.Frames != 2 {
		t.Fatalf("report = %+v, want decoded JSON fields", got)
	}

	var out bytes.Buffer
	if err := printVerdict(&out, verdict{OK: true}, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("json verdict = %s, want ok true", out.String())
	}
}

func validReport() evidenceReport {
	var rep evidenceReport
	rep.SchemaVersion = defaultSchemaVersion
	rep.Negotiation.PayloadType = 18
	rep.Negotiation.Ptime = 20
	rep.Negotiation.AnnexB = "no"
	rep.Negotiation.StrictTS = true
	rep.Negotiation.Decode = true
	rep.RTP.Packets = 1
	rep.RTP.Frames = 2
	rep.RTP.PayloadBytes = 20
	rep.RTP.DecodedFrames = 2
	rep.RTP.Streams = 1
	rep.RTP.Decode = &struct {
		Samples         int     `json:"samples"`
		DurationSeconds float64 `json:"durationSeconds"`
		RMS             float64 `json:"rms"`
		Peak            int     `json:"peak"`
		NearClip        int     `json:"nearClip"`
		HardClip        int     `json:"hardClip"`
	}{
		Samples:         160,
		DurationSeconds: 0.02,
		RMS:             1200,
		Peak:            4096,
		NearClip:        0,
		HardClip:        0,
	}
	return rep
}
