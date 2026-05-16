package main

import (
	"io"

	"github.com/hunydev/g729/internal/rtpcheck"
)

const rtpPayloadType18 = rtpcheck.PayloadType18

func analyzePayloadStream(r io.Reader, opt options) (rtpcheck.Report, error) {
	return rtpcheck.AnalyzePayloadStream(r, rtpcheck.Options{
		Mode:        opt.mode,
		Ptime:       opt.ptime,
		PayloadType: opt.payloadType,
		Decode:      opt.decode,
		StrictTS:    opt.strictTS,
	})
}

func analyzePCAP(r io.Reader, opt options) (rtpcheck.Report, error) {
	return rtpcheck.AnalyzePCAP(r, rtpcheck.Options{
		Mode:        opt.mode,
		Ptime:       opt.ptime,
		PayloadType: opt.payloadType,
		Decode:      opt.decode,
		StrictTS:    opt.strictTS,
	})
}

func parseRTP(data []byte) (rtpcheck.RTPPacket, error) {
	return rtpcheck.ParseRTP(data)
}
