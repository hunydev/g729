// Command g729rtpcheck validates raw G.729 RTP payload streams and
// Ethernet/IPv4/UDP/RTP pcap captures.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hunydev/g729/internal/rtpcheck"
)

var stdout io.Writer = os.Stdout

func main() {
	var opt options
	flag.StringVar(&opt.mode, "mode", "payload", "input mode: payload or pcap")
	flag.StringVar(&opt.path, "in", "", "input file (default stdin)")
	flag.IntVar(&opt.ptime, "ptime", 10, "expected packetization time in ms: 10, 20, or 0 in pcap mode")
	flag.IntVar(&opt.payloadType, "pt", rtpcheck.PayloadType18, "RTP payload type to inspect in pcap mode")
	flag.BoolVar(&opt.decode, "decode", false, "decode each 10-byte G.729 frame to exercise the codec API")
	flag.BoolVar(&opt.strictTS, "strict-ts", false, "require RTP sequence + timestamp continuity per SSRC")
	flag.BoolVar(&opt.jsonOutput, "json", false, "print a machine-readable JSON report")
	flag.Parse()

	in := io.Reader(os.Stdin)
	if opt.path != "" {
		f, err := os.Open(opt.path)
		if err != nil {
			exitErr(err)
		}
		defer f.Close()
		in = f
	}

	rep, err := rtpcheck.Run(in, rtpcheck.Options{
		Mode:        opt.mode,
		Ptime:       opt.ptime,
		PayloadType: opt.payloadType,
		Decode:      opt.decode,
		StrictTS:    opt.strictTS,
	})
	if err != nil {
		exitErr(err)
	}
	printReport(rep, opt.jsonOutput)
}

type options struct {
	mode        string
	path        string
	ptime       int
	payloadType int
	decode      bool
	strictTS    bool
	jsonOutput  bool
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "g729rtpcheck:", err)
	os.Exit(1)
}

func printReport(rep rtpcheck.Report, jsonOutput bool) {
	if jsonOutput {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			exitErr(err)
		}
		fmt.Fprintln(stdout, string(data))
		return
	}
	fmt.Fprintf(stdout, "mode=%s packets=%d frames=%d payload_bytes=%d decoded_frames=%d skipped=%d marker_packets=%d padding_packets=%d extension_packets=%d csrc_entries=%d streams=%d\n",
		rep.Mode, rep.Packets, rep.Frames, rep.PayloadBytes, rep.DecodedFrames, rep.Skipped,
		rep.MarkerPackets, rep.PaddingPackets, rep.ExtensionPackets, rep.CSRCEntries, rep.Streams)
}
