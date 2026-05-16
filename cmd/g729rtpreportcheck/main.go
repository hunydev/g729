// Command g729rtpreportcheck validates cmd/g729rtpreport JSON evidence against
// conservative release-gate thresholds.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

const defaultSchemaVersion = "g729.sip_rtp_blackbox_report.v1"

func main() {
	var opt options
	flag.StringVar(&opt.inputPath, "in", "", "g729rtpreport JSON path")
	flag.StringVar(&opt.schemaVersion, "schema", defaultSchemaVersion, "required schemaVersion")
	flag.IntVar(&opt.payloadType, "pt", 18, "required RTP payload type; use -1 to skip")
	flag.IntVar(&opt.ptime, "ptime", 20, "required ptime; use -1 to skip")
	flag.IntVar(&opt.minPackets, "min-packets", 1, "minimum matching RTP packets")
	flag.IntVar(&opt.minFrames, "min-frames", 1, "minimum matching G.729 frames")
	flag.IntVar(&opt.minStreams, "min-streams", 1, "minimum matching SSRC streams")
	flag.IntVar(&opt.maxStreams, "max-streams", 0, "maximum matching SSRC streams; 0 means unlimited")
	flag.IntVar(&opt.maxSkipped, "max-skipped", -1, "maximum skipped packets; -1 means unlimited")
	flag.BoolVar(&opt.requireAnnexBNo, "require-annexb-no", true, "require negotiation.annexb to be no")
	flag.BoolVar(&opt.requireStrictTS, "require-strict-ts", true, "require report to have strict RTP timestamp checking enabled")
	flag.BoolVar(&opt.requireDecode, "require-decode", true, "require local decode smoke metrics")
	flag.BoolVar(&opt.requireCleanVCS, "require-clean-vcs", false, "require repository.vcsModified=false")
	flag.Float64Var(&opt.minDuration, "min-duration", 0, "minimum decoded duration in seconds; 0 disables")
	flag.Float64Var(&opt.minRMS, "min-rms", 1, "minimum decoded RMS when decode is required")
	flag.IntVar(&opt.maxNearClip, "max-near-clip", 0, "maximum decoded near-clip sample count")
	flag.IntVar(&opt.maxHardClip, "max-hard-clip", 0, "maximum decoded hard-clip sample count")
	flag.BoolVar(&opt.jsonOutput, "json", false, "print machine-readable verdict")
	flag.Parse()

	if opt.inputPath == "" {
		exitErr(fmt.Errorf("-in is required"))
	}
	f, err := os.Open(opt.inputPath)
	if err != nil {
		exitErr(err)
	}
	defer f.Close()

	rep, err := readReport(f)
	if err != nil {
		exitErr(err)
	}
	verdict := evaluate(rep, opt)
	if err := printVerdict(os.Stdout, verdict, opt.jsonOutput); err != nil {
		exitErr(err)
	}
	if !verdict.OK {
		os.Exit(1)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "g729rtpreportcheck:", err)
	os.Exit(1)
}

type options struct {
	inputPath       string
	schemaVersion   string
	payloadType     int
	ptime           int
	minPackets      int
	minFrames       int
	minStreams      int
	maxStreams      int
	maxSkipped      int
	requireAnnexBNo bool
	requireStrictTS bool
	requireDecode   bool
	requireCleanVCS bool
	minDuration     float64
	minRMS          float64
	maxNearClip     int
	maxHardClip     int
	jsonOutput      bool
}

type evidenceReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Repository    struct {
		Commit      string `json:"commit"`
		VCSModified *bool  `json:"vcsModified"`
	} `json:"repository"`
	Negotiation struct {
		PayloadType int    `json:"payloadType"`
		Ptime       int    `json:"ptime"`
		AnnexB      string `json:"annexb"`
		StrictTS    bool   `json:"strictTs"`
		Decode      bool   `json:"decode"`
	} `json:"negotiation"`
	RTP struct {
		Packets       int `json:"packets"`
		Frames        int `json:"frames"`
		PayloadBytes  int `json:"payloadBytes"`
		DecodedFrames int `json:"decodedFrames"`
		Skipped       int `json:"skipped"`
		Streams       int `json:"streams"`
		Decode        *struct {
			Samples         int     `json:"samples"`
			DurationSeconds float64 `json:"durationSeconds"`
			RMS             float64 `json:"rms"`
			Peak            int     `json:"peak"`
			NearClip        int     `json:"nearClip"`
			HardClip        int     `json:"hardClip"`
		} `json:"decode"`
	} `json:"rtp"`
}

type verdict struct {
	OK       bool     `json:"ok"`
	Failures []string `json:"failures,omitempty"`
}

func readReport(r io.Reader) (evidenceReport, error) {
	var rep evidenceReport
	dec := json.NewDecoder(r)
	if err := dec.Decode(&rep); err != nil {
		return evidenceReport{}, err
	}
	return rep, nil
}

func evaluate(rep evidenceReport, opt options) verdict {
	var failures []string
	if rep.SchemaVersion != opt.schemaVersion {
		failures = append(failures, fmt.Sprintf("schemaVersion=%q, want %q", rep.SchemaVersion, opt.schemaVersion))
	}
	if opt.payloadType >= 0 && rep.Negotiation.PayloadType != opt.payloadType {
		failures = append(failures, fmt.Sprintf("payloadType=%d, want %d", rep.Negotiation.PayloadType, opt.payloadType))
	}
	if opt.ptime >= 0 && rep.Negotiation.Ptime != opt.ptime {
		failures = append(failures, fmt.Sprintf("ptime=%d, want %d", rep.Negotiation.Ptime, opt.ptime))
	}
	if opt.requireAnnexBNo && rep.Negotiation.AnnexB != "no" {
		failures = append(failures, fmt.Sprintf("annexb=%q, want no", rep.Negotiation.AnnexB))
	}
	if opt.requireStrictTS && !rep.Negotiation.StrictTS {
		failures = append(failures, "strictTs=false, want true")
	}
	if opt.requireCleanVCS {
		if rep.Repository.VCSModified == nil {
			failures = append(failures, "repository.vcsModified missing")
		} else if *rep.Repository.VCSModified {
			failures = append(failures, "repository.vcsModified=true, want false")
		}
	}
	if rep.RTP.Packets < opt.minPackets {
		failures = append(failures, fmt.Sprintf("packets=%d, want >= %d", rep.RTP.Packets, opt.minPackets))
	}
	if rep.RTP.Frames < opt.minFrames {
		failures = append(failures, fmt.Sprintf("frames=%d, want >= %d", rep.RTP.Frames, opt.minFrames))
	}
	if rep.RTP.Streams < opt.minStreams {
		failures = append(failures, fmt.Sprintf("streams=%d, want >= %d", rep.RTP.Streams, opt.minStreams))
	}
	if opt.maxStreams > 0 && rep.RTP.Streams > opt.maxStreams {
		failures = append(failures, fmt.Sprintf("streams=%d, want <= %d", rep.RTP.Streams, opt.maxStreams))
	}
	if opt.maxSkipped >= 0 && rep.RTP.Skipped > opt.maxSkipped {
		failures = append(failures, fmt.Sprintf("skipped=%d, want <= %d", rep.RTP.Skipped, opt.maxSkipped))
	}
	if opt.requireDecode {
		if !rep.Negotiation.Decode {
			failures = append(failures, "negotiation.decode=false, want true")
		}
		if rep.RTP.Decode == nil {
			failures = append(failures, "rtp.decode missing")
		} else {
			if rep.RTP.DecodedFrames != rep.RTP.Frames {
				failures = append(failures, fmt.Sprintf("decodedFrames=%d, want frames=%d", rep.RTP.DecodedFrames, rep.RTP.Frames))
			}
			if opt.minDuration > 0 && rep.RTP.Decode.DurationSeconds < opt.minDuration {
				failures = append(failures, fmt.Sprintf("durationSeconds=%.6f, want >= %.6f", rep.RTP.Decode.DurationSeconds, opt.minDuration))
			}
			if rep.RTP.Decode.RMS < opt.minRMS {
				failures = append(failures, fmt.Sprintf("rms=%.6f, want >= %.6f", rep.RTP.Decode.RMS, opt.minRMS))
			}
			if rep.RTP.Decode.NearClip > opt.maxNearClip {
				failures = append(failures, fmt.Sprintf("nearClip=%d, want <= %d", rep.RTP.Decode.NearClip, opt.maxNearClip))
			}
			if rep.RTP.Decode.HardClip > opt.maxHardClip {
				failures = append(failures, fmt.Sprintf("hardClip=%d, want <= %d", rep.RTP.Decode.HardClip, opt.maxHardClip))
			}
		}
	}
	return verdict{OK: len(failures) == 0, Failures: failures}
}

func printVerdict(w io.Writer, v verdict, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	if v.OK {
		_, err := fmt.Fprintln(w, "PASS")
		return err
	}
	if _, err := fmt.Fprintln(w, "FAIL"); err != nil {
		return err
	}
	for _, failure := range v.Failures {
		if _, err := fmt.Fprintf(w, "- %s\n", failure); err != nil {
			return err
		}
	}
	return nil
}
