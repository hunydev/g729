package decoder

import (
	"bytes"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestDecoderTAMEHistoryTimeline compares production against a bounded
// diagnostic upstream variant window at subframe resolution. It is meant to
// explain why the [52,239) fixed_gain_half window improves late TAME frames:
// direct fixed contribution vs accumulated past-excitation/adaptive history.
func TestDecoderTAMEHistoryTimeline(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_HISTORY_TIMELINE") != "1" {
		t.Skip("set G729_DECODER_TAME_HISTORY_TIMELINE=1 to run TAME history timeline")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_HISTORY_VECTOR", "TAME")
	candidate := decoderUpstreamWindowVariant(t)
	startSubframe := decoderITUEnvInt("G729_DECODER_HISTORY_START_SUBFRAME", 52)
	endSubframe := decoderITUEnvInt("G729_DECODER_HISTORY_END_SUBFRAME", 239)
	topN := decoderITUFrontierTopN()

	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	frames, bads := readG192Frames(t, bitPath)
	want := readPSTFrames(t, pstPath)
	if len(frames) != len(want) {
		t.Fatalf("%s: frame count mismatch bit=%d pst=%d", tc.name, len(frames), len(want))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch bads=%d frames=%d", tc.name, len(bads), len(frames))
	}
	if startSubframe < 0 || endSubframe > len(frames)*2 || startSubframe >= endSubframe {
		t.Fatalf("invalid subframe window [%d,%d) for %d frames", startSubframe, endSubframe, len(frames))
	}

	prodOut, prodRows := decoderHistoryDecodeWindow(t, bitData, len(frames), 0, 0, phase3eVariant{name: "production"})
	candOut, candRows := decoderHistoryDecodeWindow(t, bitData, len(frames), startSubframe, endSubframe, candidate)
	if len(prodRows) != len(candRows) {
		t.Fatalf("row count mismatch prod=%d cand=%d", len(prodRows), len(candRows))
	}
	baseline := decodeVariant(t, bitData, len(frames), nil, nil)
	if !phase3eEqualPCM(baseline, prodOut) {
		t.Fatalf("history production mirror diverges from Decoder.Decode baseline")
	}

	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)
	candSumSq := decoderGainCandidateOutputSumSq(candOut, want)
	rows := make([]decoderHistoryCompareRow, 0, len(prodRows))
	firstPastDrop := -1
	firstVDrop := -1
	for i := range prodRows {
		row := decoderHistoryCompareRow{
			globalSubframe: prodRows[i].globalSubframe,
			frame:          prodRows[i].frame,
			sub:            prodRows[i].sub,
			inWindow:       candRows[i].inWindow,
			prod:           prodRows[i],
			cand:           candRows[i],
			pastRatio:      safeRatioFloat64(candRows[i].pastRMS, prodRows[i].pastRMS),
			vRatio:         safeRatioFloat64(candRows[i].vRMS, prodRows[i].vRMS),
			fixedRatio:     safeRatioFloat64(candRows[i].fixedRMS, prodRows[i].fixedRMS),
			uRatio:         safeRatioFloat64(candRows[i].uRMS, prodRows[i].uRMS),
			sRatio:         safeRatioFloat64(candRows[i].sRMS, prodRows[i].sRMS),
			pastDelta:      prodRows[i].pastRMS - candRows[i].pastRMS,
			vDelta:         prodRows[i].vRMS - candRows[i].vRMS,
			uDelta:         prodRows[i].uRMS - candRows[i].uRMS,
			sDelta:         prodRows[i].sRMS - candRows[i].sRMS,
		}
		if firstPastDrop < 0 && row.pastRatio < 0.75 {
			firstPastDrop = row.globalSubframe
		}
		if firstVDrop < 0 && row.vRatio < 0.75 {
			firstVDrop = row.globalSubframe
		}
		rows = append(rows, row)
	}

	t.Logf("decoder TAME history timeline: vector=%s candidate=%s subwindow=[%d,%d) productionRMS=%.2f candidateRMS=%.2f firstPast<0.75=%d firstV<0.75=%d",
		tc.name,
		candidate.name,
		startSubframe,
		endSubframe,
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples),
		decoderGainCandidateRMS(candSumSq, len(frames)*frameSamples),
		firstPastDrop,
		firstVDrop)

	t.Logf("window start context")
	decoderHistoryLogRange(t, rows, startSubframe-4, startSubframe+10)
	t.Logf("window end context")
	decoderHistoryLogRange(t, rows, endSubframe-10, endSubframe+8)
	t.Logf("late stress context")
	decoderHistoryLogRange(t, rows, 117*2, len(rows))

	reductions := append([]decoderHistoryCompareRow(nil), rows...)
	sort.Slice(reductions, func(i, j int) bool {
		if reductions[i].vDelta != reductions[j].vDelta {
			return reductions[i].vDelta > reductions[j].vDelta
		}
		return reductions[i].globalSubframe < reductions[j].globalSubframe
	})
	if topN > len(reductions) {
		topN = len(reductions)
	}
	t.Logf("largest ACB-vector RMS reductions")
	decoderHistoryLogRows(t, reductions[:topN])
}

type decoderHistorySubframeMetrics struct {
	globalSubframe int
	frame          int
	sub            int
	inWindow       bool
	tInt           int
	tFrac          int
	gpQ14          int16
	gammaQ13       int16
	gcMantQ14      int16
	gcExp          int8
	predictedQ10   int32
	logGainQ10     int32
	uCurrentQ10    int16
	pastEnergy     int64
	pastRMS        float64
	pastTailRMS    float64
	v              [subframeLen]int16
	vRMS           float64
	pitchRMS       float64
	fixedRMS       float64
	uRMS           float64
	sRMS           float64
}

type decoderHistoryCompareRow struct {
	globalSubframe int
	frame          int
	sub            int
	inWindow       bool
	prod           decoderHistorySubframeMetrics
	cand           decoderHistorySubframeMetrics
	pastRatio      float64
	vRatio         float64
	fixedRatio     float64
	uRatio         float64
	sRatio         float64
	pastDelta      float64
	vDelta         float64
	uDelta         float64
	sDelta         float64
}

func decoderHistoryDecodeWindow(t *testing.T, bitData []byte, frames, startSubframe, endSubframe int, candidate phase3eVariant) ([]int16, []decoderHistorySubframeMetrics) {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	rows := make([]decoderHistorySubframeMetrics, 0, frames*2)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[history=%d:%d] frame %d: %v", startSubframe, endSubframe, frame, err)
		}
		frameRows, err := dec.decodeFrameHistory(frame, packed[:], out[frame*frameSamples:(frame+1)*frameSamples], startSubframe, endSubframe, candidate)
		if err != nil {
			t.Fatalf("decodeFrameHistory frame %d: %v", frame, err)
		}
		rows = append(rows, frameRows[:]...)
	}
	return out, rows
}

func (d *Decoder) decodeFrameHistory(frame int, packed []byte, out []int16, startSubframe, endSubframe int, candidate phase3eVariant) ([2]decoderHistorySubframeMetrics, error) {
	var rows [2]decoderHistorySubframeMetrics
	if len(packed) < bitstream.FrameBytes {
		return rows, ErrShortInput
	}
	if len(out) < frameSamples {
		return rows, ErrShortOutput
	}

	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return rows, err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(fr.L0),
		L1: uint8(fr.L1),
		L2: uint8(fr.L2),
		L3: uint8(fr.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	variants := [2]phase3eVariant{{name: "production"}, {name: "production"}}
	inWindow := [2]bool{}
	for sub := 0; sub < 2; sub++ {
		globalSubframe := frame*2 + sub
		if globalSubframe >= startSubframe && globalSubframe < endSubframe {
			variants[sub] = candidate
			inWindow[sub] = true
		}
	}
	tFrac1 = phase3eAdjustTFrac(tFrac1, variants[0])
	tFrac2 = phase3eAdjustTFrac(tFrac2, variants[1])

	rows[0] = d.decodeSubframeHistory(frame, 0, inWindow[0], &sf1A, tInt1, tFrac1,
		fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variants[0])
	rows[1] = d.decodeSubframeHistory(frame, 1, inWindow[1], &sf2A, tInt2, tFrac2,
		fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variants[1])

	scaleDecoderOutput(out[:frameSamples])
	return rows, nil
}

func (d *Decoder) decodeSubframeHistory(
	frame int,
	sub int,
	inWindow bool,
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	variant phase3eVariant,
) decoderHistorySubframeMetrics {
	metric := decoderHistorySubframeMetrics{
		globalSubframe: frame*2 + sub,
		frame:          frame,
		sub:            sub,
		inWindow:       inWindow,
		tInt:           tInt,
		tFrac:          tFrac,
		pastEnergy:     decoderHistoryEnergy(d.pastExc[:]),
		pastRMS:        decoderHistoryRMS(d.pastExc[:]),
		pastTailRMS:    decoderHistoryRMS(d.pastExc[pastExcLen-subframeLen:]),
	}

	betaQ14 := d.pitchEnhancementBetaQ14()
	if variant.noFCBEnhancement {
		betaQ14 = 0
	}

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)
	metric.v = v
	metric.vRMS = envelopeRMS(v[:])

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, &c)
	gpQ14 := gainTaps.GpQ14Final
	gcMantQ14 := gainTaps.GcMantQ14
	gcExp := gainTaps.GcExp
	if variant.zeroAdaptive {
		gpQ14 = 0
	}
	if variant.pitchCapQ14 > 0 && gpQ14 > variant.pitchCapQ14 {
		gpQ14 = variant.pitchCapQ14
	}
	if variant.pitchScaleNum != 0 && variant.pitchScaleDen != 0 {
		gpQ14 = phase3eScaleWord16(gpQ14, variant.pitchScaleNum, variant.pitchScaleDen)
	}
	if variant.zeroFixed {
		gcMantQ14 = 0
		gcExp = 0
	}
	if variant.fixedExpDelta != 0 && gcMantQ14 != 0 {
		gcExp = phase3eClampExp(int(gcExp) + variant.fixedExpDelta)
	}
	metric.gpQ14 = gpQ14
	metric.gammaQ13 = gainTaps.GammaCQ13
	metric.gcMantQ14 = gcMantQ14
	metric.gcExp = gcExp
	metric.predictedQ10 = gainTaps.Predicted
	metric.logGainQ10 = gainTaps.LogGainDbQ10
	metric.uCurrentQ10 = gainTaps.UCurrent

	var zero [subframeLen]int16
	var pitchOnly [subframeLen]int16
	var fixedOnly [subframeLen]int16
	synth.BuildExcitation(gpQ14, 0, 0, &v, &zero, &pitchOnly)
	synth.BuildExcitation(0, gcMantQ14, gcExp, &zero, &c, &fixedOnly)
	metric.pitchRMS = envelopeRMS(pitchOnly[:])
	metric.fixedRMS = envelopeRMS(fixedOnly[:])

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)
	metric.uRMS = envelopeRMS(u[:])

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	metric.sRMS = envelopeRMS(s[:])

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.rememberPitchGain(gainTaps.GpQ14Final)

	return metric
}

func decoderHistoryLogRange(t *testing.T, rows []decoderHistoryCompareRow, start, end int) {
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
	decoderHistoryLogRows(t, rows[start:end])
}

func decoderHistoryLogRows(t *testing.T, rows []decoderHistoryCompareRow) {
	t.Helper()
	t.Logf("%5s %5s %3s %3s %8s %8s %7s %8s %8s %7s %8s %8s %7s %8s %8s %7s %8s %8s %7s %6s %6s %6s %6s %8s %8s %8s %8s",
		"sf", "frame", "sub", "win",
		"pPast", "cPast", "c/p",
		"pV", "cV", "c/p",
		"pFix", "cFix", "c/p",
		"pU", "cU", "c/p",
		"pS", "cS", "c/p",
		"pGp", "cGp", "pExp", "cExp",
		"pPred", "cPred", "pUCur", "cUCur")
	for _, r := range rows {
		t.Logf("%5d %5d %3d %3t %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %8.1f %8.1f %7.3f %6d %6d %6d %6d %8.1f %8.1f %8.1f %8.1f",
			r.globalSubframe, r.frame, r.sub, r.inWindow,
			r.prod.pastRMS, r.cand.pastRMS, r.pastRatio,
			r.prod.vRMS, r.cand.vRMS, r.vRatio,
			r.prod.fixedRMS, r.cand.fixedRMS, r.fixedRatio,
			r.prod.uRMS, r.cand.uRMS, r.uRatio,
			r.prod.sRMS, r.cand.sRMS, r.sRatio,
			r.prod.gpQ14, r.cand.gpQ14,
			r.prod.gcExp, r.cand.gcExp,
			float64(r.prod.predictedQ10)/1024.0,
			float64(r.cand.predictedQ10)/1024.0,
			float64(r.prod.uCurrentQ10)/1024.0,
			float64(r.cand.uCurrentQ10)/1024.0)
	}
}

func decoderHistoryEnergy(samples []int16) int64 {
	var energy int64
	for _, sample := range samples {
		s := int64(sample)
		energy += s * s
	}
	return energy
}

func decoderHistoryRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	return math.Sqrt(float64(decoderHistoryEnergy(samples)) / float64(len(samples)))
}
