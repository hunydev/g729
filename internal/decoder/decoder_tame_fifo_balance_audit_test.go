package decoder

import (
	"math"
	"os"
	"sort"
	"testing"
)

// TestDecoderTAMEPastExcFIFOBalanceAudit reports the per-subframe energy
// balance of the past-excitation FIFO. It is diagnostic-only: if incoming U is
// consistently larger than the 40 samples shifted out, pastExc growth is a
// direct consequence of the local feedback trajectory rather than a FIFO copy
// bug.
func TestDecoderTAMEPastExcFIFOBalanceAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_FIFO_BALANCE") != "1" {
		t.Skip("set G729_DECODER_TAME_FIFO_BALANCE=1 to run TAME pastExc FIFO balance audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_TAME_FIFO_BALANCE_VECTOR", "TAME")
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
	rows := make([]decoderTAMEFIFOBalanceRow, 0, len(frames)*2)
	incomingGTOutgoing := 0
	postGrows := 0
	for frame, packed := range frames {
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, frame, err)
		}
		stats := decoderITUCompareFrame(&taps.Output, &want[frame])
		pstRMS := decoderHistoryRMS(want[frame][:])
		outRMS := decoderHistoryRMS(taps.Output[:])
		for sub := 0; sub < 2; sub++ {
			row := decoderTAMEFIFOBalanceRowFromTaps(frame, sub, taps.Sub[sub], pstRMS, outRMS, stats)
			if row.inOutRatio > 1 {
				incomingGTOutgoing++
			}
			if row.postPreRatio > 1 {
				postGrows++
			}
			rows = append(rows, row)
		}
	}

	topN := decoderITUFrontierTopN()
	t.Logf("decoder TAME FIFO balance: vector=%s frames=%d subframes=%d incoming>outgoing=%d postRMS>preRMS=%d",
		tc.name, len(frames), len(rows), incomingGTOutgoing, postGrows)
	decoderTAMEFIFOBalanceLogSummary(t, rows)

	t.Logf("onset context")
	decoderTAMEFIFOBalanceLogRange(t, rows, 3*2, 32*2)
	t.Logf("severe checkpoint context")
	decoderTAMEFIFOBalanceLogRange(t, rows, 112*2, len(rows))

	byGrowth := append([]decoderTAMEFIFOBalanceRow(nil), rows...)
	sort.Slice(byGrowth, func(i, j int) bool {
		if byGrowth[i].postDelta != byGrowth[j].postDelta {
			return byGrowth[i].postDelta > byGrowth[j].postDelta
		}
		return byGrowth[i].globalSubframe < byGrowth[j].globalSubframe
	})
	if topN > len(byGrowth) {
		topN = len(byGrowth)
	}
	t.Logf("top FIFO growth rows")
	decoderTAMEFIFOBalanceLogRows(t, byGrowth[:topN])
}

type decoderTAMEFIFOBalanceRow struct {
	globalSubframe int
	frame          int
	sub            int
	tInt           int
	tFrac          int
	shortPitch     bool
	outRatio       float64
	rmsErr         float64
	maxAbs         int
	preRMS         float64
	outgoingRMS    float64
	keptRMS        float64
	incomingRMS    float64
	postRMS        float64
	inOutRatio     float64
	postPreRatio   float64
	postDelta      float64
	vRMS           float64
	pitchRMS       float64
	fixedRMS       float64
	gpQ14          int16
}

func decoderTAMEFIFOBalanceRowFromTaps(frame, sub int, taps Phase3DiagSubframeTaps, pstRMS, outRMS float64, stats decoderITUFrameStats) decoderTAMEFIFOBalanceRow {
	outgoingEnergy := decoderHistoryEnergy(taps.PastExcPreACB[:subframeLen])
	keptEnergy := decoderHistoryEnergy(taps.PastExcPreACB[subframeLen:])
	incomingEnergy := decoderHistoryEnergy(taps.U[:])
	postEnergy := keptEnergy + incomingEnergy
	energy := decoderGainFrontierSubEnergy(taps)
	preRMS := decoderHistoryRMS(taps.PastExcPreACB[:])
	postRMS := decoderTAMERMSFromEnergy(postEnergy, pastExcLen)
	outgoingRMS := decoderTAMERMSFromEnergy(outgoingEnergy, subframeLen)
	incomingRMS := decoderTAMERMSFromEnergy(incomingEnergy, subframeLen)
	return decoderTAMEFIFOBalanceRow{
		globalSubframe: frame*2 + sub,
		frame:          frame,
		sub:            sub,
		tInt:           taps.TInt,
		tFrac:          taps.TFrac,
		shortPitch:     taps.TInt < subframeLen,
		outRatio:       safeRatioFloat64(outRMS, pstRMS),
		rmsErr:         stats.rmsDelta(),
		maxAbs:         stats.maxAbsDelta,
		preRMS:         preRMS,
		outgoingRMS:    outgoingRMS,
		keptRMS:        decoderTAMERMSFromEnergy(keptEnergy, pastExcLen-subframeLen),
		incomingRMS:    incomingRMS,
		postRMS:        postRMS,
		inOutRatio:     safeRatioFloat64(incomingRMS, outgoingRMS),
		postPreRatio:   safeRatioFloat64(postRMS, preRMS),
		postDelta:      postRMS - preRMS,
		vRMS:           envelopeRMS(taps.V[:]),
		pitchRMS:       energy.pitchRMS,
		fixedRMS:       energy.fixedRMS,
		gpQ14:          taps.GpQ14,
	}
}

func decoderTAMERMSFromEnergy(energy int64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return math.Sqrt(float64(energy) / float64(count))
}

func decoderTAMEFIFOBalanceLogSummary(t *testing.T, rows []decoderTAMEFIFOBalanceRow) {
	t.Helper()
	windows := []struct {
		name  string
		start int
		end   int
	}{
		{name: "cold-start", start: 0, end: 4 * 2},
		{name: "pre-1.25x", start: 3 * 2, end: 31 * 2},
		{name: "first-1.25x", start: 31 * 2, end: 61 * 2},
		{name: "severe-rise", start: 61 * 2, end: 117 * 2},
		{name: "checkpoint", start: 117 * 2, end: len(rows)},
	}
	t.Logf("%-12s %8s %8s %8s %8s %8s %8s",
		"window", "subfrm", "grow", "sumD", "avgD", "firstPre", "lastPost")
	for _, window := range windows {
		start := window.start
		end := window.end
		if start < 0 {
			start = 0
		}
		if end > len(rows) {
			end = len(rows)
		}
		if start >= end {
			continue
		}
		var sumDelta float64
		var grow int
		for _, row := range rows[start:end] {
			sumDelta += row.postDelta
			if row.postDelta > 0 {
				grow++
			}
		}
		t.Logf("%-12s %8d %8d %8.2f %8.2f %8.1f %8.1f",
			window.name,
			end-start,
			grow,
			sumDelta,
			sumDelta/float64(end-start),
			rows[start].preRMS,
			rows[end-1].postRMS)
	}
}

func decoderTAMEFIFOBalanceLogRange(t *testing.T, rows []decoderTAMEFIFOBalanceRow, start, end int) {
	t.Helper()
	if start < 0 {
		start = 0
	}
	if end > len(rows) {
		end = len(rows)
	}
	if start >= end {
		return
	}
	decoderTAMEFIFOBalanceLogRows(t, rows[start:end])
}

func decoderTAMEFIFOBalanceLogRows(t *testing.T, rows []decoderTAMEFIFOBalanceRow) {
	t.Helper()
	t.Logf("%5s %5s %3s %5s %5s %5s %7s %8s %7s %8s %8s %8s %8s %8s %7s %7s %8s %8s %8s %8s %6s",
		"sf", "frame", "sub", "T", "frac", "short", "out/PST", "rmsErr", "maxAbs",
		"pre", "outOld", "kept", "inU", "post", "in/out", "post/pre",
		"v", "pitch", "fixed", "dPost", "gp")
	for _, r := range rows {
		t.Logf("%5d %5d %3d %5d %5d %5t %7.3f %8.1f %7d %8.1f %8.1f %8.1f %8.1f %8.1f %7.3f %7.3f %8.1f %8.1f %8.1f %8.2f %6d",
			r.globalSubframe,
			r.frame,
			r.sub,
			r.tInt,
			r.tFrac,
			r.shortPitch,
			r.outRatio,
			r.rmsErr,
			r.maxAbs,
			r.preRMS,
			r.outgoingRMS,
			r.keptRMS,
			r.incomingRMS,
			r.postRMS,
			r.inOutRatio,
			r.postPreRatio,
			r.vRMS,
			r.pitchRMS,
			r.fixedRMS,
			r.postDelta,
			r.gpQ14)
	}
}
