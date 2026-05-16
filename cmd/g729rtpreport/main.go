// Command g729rtpreport writes a compact JSON evidence report for externally
// captured G729/8000 annexb=no SIP/RTP calls.
//
// Clean-room I1 declaration: this command consumes black-box pcap captures and
// this repository's local decoder metrics only. It must not be used with, or
// derive implementation logic from, any third-party G.729 codec source.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/hunydev/g729/internal/rtpcheck"
)

const schemaVersion = "g729.sip_rtp_blackbox_report.v1"

func main() {
	var opt options
	flag.StringVar(&opt.inputPath, "in", "", "input Ethernet/IPv4/UDP/RTP pcap path")
	flag.StringVar(&opt.outputPath, "out", "", "output JSON path (default stdout)")
	flag.IntVar(&opt.payloadType, "pt", rtpcheck.PayloadType18, "RTP payload type to inspect")
	flag.IntVar(&opt.ptime, "ptime", 20, "expected packetization time in ms: 10, 20, or 0 for variable/unknown")
	flag.BoolVar(&opt.strictTS, "strict-ts", true, "require RTP sequence + timestamp continuity per SSRC")
	flag.BoolVar(&opt.decode, "decode", true, "decode matching RTP payloads and include PCM smoke metrics")
	flag.StringVar(&opt.commit, "commit", "", "repository commit hash override")
	flag.StringVar(&opt.peer, "peer", "", "black-box peer product/version summary")
	flag.StringVar(&opt.peerRole, "peer-role", "", "black-box peer role, for example gateway, pbx, sbc, endpoint")
	flag.StringVar(&opt.topology, "topology", "", "integration topology summary")
	flag.StringVar(&opt.sdpOffer, "sdp-offer", "", "SDP offer snippet or private artifact reference")
	flag.StringVar(&opt.sdpAnswer, "sdp-answer", "", "SDP answer snippet or private artifact reference")
	flag.StringVar(&opt.notes, "notes", "", "free-form integration notes")
	flag.Parse()

	if opt.inputPath == "" {
		exitErr(fmt.Errorf("-in is required"))
	}

	in, err := os.Open(opt.inputPath)
	if err != nil {
		exitErr(err)
	}
	defer in.Close()

	rep, err := buildReport(in, opt, time.Now().UTC())
	if err != nil {
		exitErr(err)
	}

	out := io.Writer(os.Stdout)
	if opt.outputPath != "" {
		f, err := os.Create(opt.outputPath)
		if err != nil {
			exitErr(err)
		}
		defer f.Close()
		out = f
	}
	if err := writeJSON(out, rep); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "g729rtpreport:", err)
	os.Exit(1)
}

type options struct {
	inputPath   string
	outputPath  string
	payloadType int
	ptime       int
	strictTS    bool
	decode      bool
	commit      string
	peer        string
	peerRole    string
	topology    string
	sdpOffer    string
	sdpAnswer   string
	notes       string
}

type evidenceReport struct {
	SchemaVersion string          `json:"schemaVersion"`
	Tool          string          `json:"tool"`
	GeneratedAt   string          `json:"generatedAt"`
	Repository    repositoryInfo  `json:"repository"`
	Input         inputInfo       `json:"input"`
	Peer          peerInfo        `json:"peer"`
	Negotiation   negotiationInfo `json:"negotiation"`
	RTP           rtpcheck.Report `json:"rtp"`
	Boundary      []string        `json:"boundary"`
	Notes         string          `json:"notes,omitempty"`
}

type repositoryInfo struct {
	Commit      string `json:"commit,omitempty"`
	VCSModified *bool  `json:"vcsModified,omitempty"`
}

type inputInfo struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type peerInfo struct {
	Summary  string `json:"summary,omitempty"`
	Role     string `json:"role,omitempty"`
	Topology string `json:"topology,omitempty"`
}

type negotiationInfo struct {
	PayloadType int    `json:"payloadType"`
	Ptime       int    `json:"ptime"`
	AnnexB      string `json:"annexb"`
	StrictTS    bool   `json:"strictTs"`
	Decode      bool   `json:"decode"`
	SDPOffer    string `json:"sdpOffer,omitempty"`
	SDPAnswer   string `json:"sdpAnswer,omitempty"`
}

func buildReport(r io.Reader, opt options, now time.Time) (evidenceReport, error) {
	rtp, err := rtpcheck.Run(r, rtpcheck.Options{
		Mode:        "pcap",
		Ptime:       opt.ptime,
		PayloadType: opt.payloadType,
		Decode:      opt.decode,
		StrictTS:    opt.strictTS,
	})
	if err != nil {
		return evidenceReport{}, err
	}

	repo := detectRepositoryInfo()
	if opt.commit != "" {
		repo.Commit = opt.commit
	}
	return evidenceReport{
		SchemaVersion: schemaVersion,
		Tool:          "cmd/g729rtpreport",
		GeneratedAt:   now.Format(time.RFC3339),
		Repository:    repo,
		Input: inputInfo{
			Kind: "ethernet-ipv4-udp-rtp-pcap",
			Path: cleanPath(opt.inputPath),
		},
		Peer: peerInfo{
			Summary:  opt.peer,
			Role:     opt.peerRole,
			Topology: opt.topology,
		},
		Negotiation: negotiationInfo{
			PayloadType: opt.payloadType,
			Ptime:       opt.ptime,
			AnnexB:      "no",
			StrictTS:    opt.strictTS,
			Decode:      opt.decode,
			SDPOffer:    opt.sdpOffer,
			SDPAnswer:   opt.sdpAnswer,
		},
		RTP: rtp,
		Boundary: []string{
			"black-box SIP/RTP evidence only",
			"not ITU certification or endorsement",
			"not encoder byte-exact conformance evidence",
			"not Annex B support evidence",
			"do not commit private pcaps, private audio, third-party codec source, or third-party codec binaries",
		},
		Notes: opt.notes,
	}, nil
}

func detectRepositoryInfo() repositoryInfo {
	info, ok := debug.ReadBuildInfo()
	var out repositoryInfo
	if !ok {
		return detectGitRepositoryInfo()
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			out.Commit = setting.Value
		case "vcs.modified":
			modified := setting.Value == "true"
			out.VCSModified = &modified
		}
	}
	if out.Commit == "" {
		git := detectGitRepositoryInfo()
		out.Commit = git.Commit
		if out.VCSModified == nil {
			out.VCSModified = git.VCSModified
		}
	}
	return out
}

func detectGitRepositoryInfo() repositoryInfo {
	var out repositoryInfo
	if data, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		out.Commit = strings.TrimSpace(string(data))
	}
	if data, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		modified := len(strings.TrimSpace(string(data))) > 0
		out.VCSModified = &modified
	}
	return out
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func writeJSON(w io.Writer, rep evidenceReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
