package decoder

import (
	"bytes"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
)

// TestDecoderITUGainCandidateFrontier localizes gain-reconstruction diagnostic
// candidates by final PST-output error. It keeps the candidate opt-in because
// these variants are not production fixes; they are a clean-room way to choose
// the next numeric oracle or spec-audit target.
func TestDecoderITUGainCandidateFrontier(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CANDIDATE_FRONTIER") != "1" {
		t.Skip("set G729_DECODER_GAIN_CANDIDATE_FRONTIER=1 to run gain candidate frontier")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_GAIN_CANDIDATE_VECTOR", "TAME")
	candidate := decoderGainCandidateFrontierVariant(t)

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

	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	mirrorOut := phase3jDecodeVariant(t, bitData, len(frames), phase3jVariant{name: "gain_mirror_default"})
	if !phase3eEqualPCM(prodOut, mirrorOut) {
		t.Fatalf("phase3j gain mirror diverges from Decoder.Decode baseline")
	}
	candidateOut := phase3jDecodeVariant(t, bitData, len(frames), candidate)

	tapsByFrame := decoderGainFrontierTaps(t, frames)
	rows := make([]decoderGainCandidateFrontierRow, 0, len(frames))
	var prodSumSq, candSumSq int64
	for frame := range frames {
		prodStats := decoderGainFrontierCompareFrame(prodOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		candStats := decoderGainFrontierCompareFrame(candidateOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		prodSumSq += prodStats.sumSqDelta
		candSumSq += candStats.sumSqDelta
		rows = append(rows, decoderGainCandidateFrontierRow{
			frame:       frame,
			prod:        prodStats,
			candidate:   candStats,
			improvement: prodStats.sumSqDelta - candStats.sumSqDelta,
			taps:        tapsByFrame[frame],
		})
	}

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}

	t.Logf("decoder ITU gain candidate frontier: vector=%s candidate=%s topN=%d", tc.name, candidate.name, topN)
	t.Logf("aggregate RMS: production=%.2f candidate=%.2f dSumSq=%d",
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples),
		decoderGainCandidateRMS(candSumSq, len(frames)*frameSamples),
		prodSumSq-candSumSq)

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].improvement != rows[j].improvement {
			return rows[i].improvement > rows[j].improvement
		}
		return rows[i].frame < rows[j].frame
	})
	t.Logf("largest improvements")
	decoderGainCandidateLogRows(t, rows[:topN])

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].improvement != rows[j].improvement {
			return rows[i].improvement < rows[j].improvement
		}
		return rows[i].frame < rows[j].frame
	})
	t.Logf("largest regressions")
	decoderGainCandidateLogRows(t, rows[:topN])
}

// TestDecoderITUGainCandidateCutover runs the selected gain candidate from a
// single cutover frame onward. If a late cutover beats both production and a
// frame-0 candidate, the defect is likely state-history dependent rather than
// a globally valid gain formula replacement.
func TestDecoderITUGainCandidateCutover(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CANDIDATE_CUTOVER") != "1" {
		t.Skip("set G729_DECODER_GAIN_CANDIDATE_CUTOVER=1 to run gain candidate cutover")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_GAIN_CANDIDATE_VECTOR", "TAME")
	candidate := decoderGainCandidateFrontierVariant(t)

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
	if len(frames) > 512 {
		t.Fatalf("%s has %d frames; cutover scan is intentionally bounded to <=512 frames", tc.name, len(frames))
	}

	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)

	rows := make([]decoderGainCandidateCutoverRow, 0, len(frames)+1)
	for cutover := 0; cutover <= len(frames); cutover++ {
		out := phase3jDecodeVariantCutover(t, bitData, len(frames), cutover, candidate)
		sumSq := decoderGainCandidateOutputSumSq(out, want)
		rows = append(rows, decoderGainCandidateCutoverRow{
			cutover:     cutover,
			sumSq:       sumSq,
			improvement: prodSumSq - sumSq,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].sumSq != rows[j].sumSq {
			return rows[i].sumSq < rows[j].sumSq
		}
		return rows[i].cutover < rows[j].cutover
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}

	t.Logf("decoder ITU gain candidate cutover: vector=%s candidate=%s topN=%d productionRMS=%.2f",
		tc.name,
		candidate.name,
		topN,
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples))
	t.Logf("%-8s %10s %10s", "cutover", "candRMS", "dSumSq")
	for _, row := range rows[:topN] {
		t.Logf("%-8d %10.2f %10d",
			row.cutover,
			decoderGainCandidateRMS(row.sumSq, len(frames)*frameSamples),
			row.improvement)
	}
}

// TestDecoderITUGainCandidateWindow scans finite frame windows where the
// selected candidate is applied only for [start,end). Decoder state is not
// reset at window boundaries, so any lasting excitation/synthesis effects are
// preserved after the candidate switches back to the production formula.
func TestDecoderITUGainCandidateWindow(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CANDIDATE_WINDOW") != "1" {
		t.Skip("set G729_DECODER_GAIN_CANDIDATE_WINDOW=1 to run gain candidate window scan")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_GAIN_CANDIDATE_VECTOR", "TAME")
	candidate := decoderGainCandidateFrontierVariant(t)

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
	if len(frames) > 256 {
		t.Fatalf("%s has %d frames; window scan is intentionally bounded to <=256 frames", tc.name, len(frames))
	}

	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)

	rows := make([]decoderGainCandidateWindowRow, 0, len(frames)*(len(frames)+1)/2)
	for start := 0; start < len(frames); start++ {
		for end := start + 1; end <= len(frames); end++ {
			out := phase3jDecodeVariantWindow(t, bitData, len(frames), start, end, candidate)
			sumSq := decoderGainCandidateOutputSumSq(out, want)
			rows = append(rows, decoderGainCandidateWindowRow{
				start:       start,
				end:         end,
				sumSq:       sumSq,
				improvement: prodSumSq - sumSq,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].sumSq != rows[j].sumSq {
			return rows[i].sumSq < rows[j].sumSq
		}
		if rows[i].start != rows[j].start {
			return rows[i].start < rows[j].start
		}
		return rows[i].end < rows[j].end
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}

	t.Logf("decoder ITU gain candidate window: vector=%s candidate=%s topN=%d productionRMS=%.2f",
		tc.name,
		candidate.name,
		topN,
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples))
	t.Logf("%-8s %-8s %-8s %10s %10s", "start", "end", "len", "candRMS", "dSumSq")
	for _, row := range rows[:topN] {
		t.Logf("%-8d %-8d %-8d %10.2f %10d",
			row.start,
			row.end,
			row.end-row.start,
			decoderGainCandidateRMS(row.sumSq, len(frames)*frameSamples),
			row.improvement)
	}

	best := rows[0]
	bestOut := phase3jDecodeVariantWindow(t, bitData, len(frames), best.start, best.end, candidate)
	tapsByFrame := decoderGainFrontierTaps(t, frames)
	frameRows := make([]decoderGainCandidateFrontierRow, 0, len(frames))
	for frame := range frames {
		prodStats := decoderGainFrontierCompareFrame(prodOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		candStats := decoderGainFrontierCompareFrame(bestOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		frameRows = append(frameRows, decoderGainCandidateFrontierRow{
			frame:       frame,
			prod:        prodStats,
			candidate:   candStats,
			improvement: prodStats.sumSqDelta - candStats.sumSqDelta,
			taps:        tapsByFrame[frame],
		})
	}

	sort.Slice(frameRows, func(i, j int) bool {
		if frameRows[i].improvement != frameRows[j].improvement {
			return frameRows[i].improvement > frameRows[j].improvement
		}
		return frameRows[i].frame < frameRows[j].frame
	})
	t.Logf("best window largest improvements")
	decoderGainCandidateLogRows(t, frameRows[:topN])

	sort.Slice(frameRows, func(i, j int) bool {
		if frameRows[i].improvement != frameRows[j].improvement {
			return frameRows[i].improvement < frameRows[j].improvement
		}
		return frameRows[i].frame < frameRows[j].frame
	})
	t.Logf("best window largest regressions")
	decoderGainCandidateLogRows(t, frameRows[:topN])
}

// TestDecoderITUUpstreamVariantWindow scans finite windows for the broader
// phase3e upstream variants, especially fixed_gain_half. These variants are
// not production fixes; the scan localizes which frame range has lasting
// past-excitation/adaptive-history impact on final PST agreement.
func TestDecoderITUUpstreamVariantWindow(t *testing.T) {
	if os.Getenv("G729_DECODER_UPSTREAM_VARIANT_WINDOW") != "1" {
		t.Skip("set G729_DECODER_UPSTREAM_VARIANT_WINDOW=1 to run upstream variant window scan")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_UPSTREAM_WINDOW_VECTOR", "TAME")
	candidate := decoderUpstreamWindowVariant(t)

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
	if len(frames) > 256 {
		t.Fatalf("%s has %d frames; upstream window scan is intentionally bounded to <=256 frames", tc.name, len(frames))
	}

	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)

	rows := make([]decoderGainCandidateWindowRow, 0, len(frames)*(len(frames)+1)/2)
	for start := 0; start < len(frames); start++ {
		for end := start + 1; end <= len(frames); end++ {
			out := phase3eDecodeVariantWindow(t, bitData, len(frames), start, end, candidate)
			sumSq := decoderGainCandidateOutputSumSq(out, want)
			rows = append(rows, decoderGainCandidateWindowRow{
				start:       start,
				end:         end,
				sumSq:       sumSq,
				improvement: prodSumSq - sumSq,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].sumSq != rows[j].sumSq {
			return rows[i].sumSq < rows[j].sumSq
		}
		if rows[i].start != rows[j].start {
			return rows[i].start < rows[j].start
		}
		return rows[i].end < rows[j].end
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}

	t.Logf("decoder ITU upstream variant window: vector=%s candidate=%s topN=%d productionRMS=%.2f",
		tc.name,
		candidate.name,
		topN,
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples))
	t.Logf("%-8s %-8s %-8s %10s %10s", "start", "end", "len", "candRMS", "dSumSq")
	for _, row := range rows[:topN] {
		t.Logf("%-8d %-8d %-8d %10.2f %10d",
			row.start,
			row.end,
			row.end-row.start,
			decoderGainCandidateRMS(row.sumSq, len(frames)*frameSamples),
			row.improvement)
	}

	best := rows[0]
	bestOut := phase3eDecodeVariantWindow(t, bitData, len(frames), best.start, best.end, candidate)
	frameRows := make([]decoderUpstreamWindowFrameRow, 0, len(frames))
	for frame := range frames {
		prodStats := decoderGainFrontierCompareFrame(prodOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		candStats := decoderGainFrontierCompareFrame(bestOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		frameRows = append(frameRows, decoderUpstreamWindowFrameRow{
			frame:       frame,
			prod:        prodStats,
			candidate:   candStats,
			improvement: prodStats.sumSqDelta - candStats.sumSqDelta,
		})
	}

	sort.Slice(frameRows, func(i, j int) bool {
		if frameRows[i].improvement != frameRows[j].improvement {
			return frameRows[i].improvement > frameRows[j].improvement
		}
		return frameRows[i].frame < frameRows[j].frame
	})
	t.Logf("best window largest improvements")
	decoderUpstreamWindowLogRows(t, frameRows[:topN])

	sort.Slice(frameRows, func(i, j int) bool {
		if frameRows[i].improvement != frameRows[j].improvement {
			return frameRows[i].improvement < frameRows[j].improvement
		}
		return frameRows[i].frame < frameRows[j].frame
	})
	t.Logf("best window largest regressions")
	decoderUpstreamWindowLogRows(t, frameRows[:topN])
}

// TestDecoderITUUpstreamVariantSubframeWindow is the subframe-resolution
// counterpart to TestDecoderITUUpstreamVariantWindow. It determines whether the
// damaging state accumulation has a frame-internal boundary.
func TestDecoderITUUpstreamVariantSubframeWindow(t *testing.T) {
	if os.Getenv("G729_DECODER_UPSTREAM_VARIANT_SUBFRAME_WINDOW") != "1" {
		t.Skip("set G729_DECODER_UPSTREAM_VARIANT_SUBFRAME_WINDOW=1 to run upstream variant subframe window scan")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_UPSTREAM_WINDOW_VECTOR", "TAME")
	candidate := decoderUpstreamWindowVariant(t)

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
	if len(frames) > 256 {
		t.Fatalf("%s has %d frames; subframe window scan is intentionally bounded to <=256 frames", tc.name, len(frames))
	}

	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)

	subframes := len(frames) * 2
	rows := make([]decoderGainCandidateWindowRow, 0, subframes*(subframes+1)/2)
	for start := 0; start < subframes; start++ {
		for end := start + 1; end <= subframes; end++ {
			out := phase3eDecodeVariantSubframeWindow(t, bitData, len(frames), start, end, candidate)
			sumSq := decoderGainCandidateOutputSumSq(out, want)
			rows = append(rows, decoderGainCandidateWindowRow{
				start:       start,
				end:         end,
				sumSq:       sumSq,
				improvement: prodSumSq - sumSq,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].sumSq != rows[j].sumSq {
			return rows[i].sumSq < rows[j].sumSq
		}
		if rows[i].start != rows[j].start {
			return rows[i].start < rows[j].start
		}
		return rows[i].end < rows[j].end
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}

	t.Logf("decoder ITU upstream variant subframe window: vector=%s candidate=%s topN=%d productionRMS=%.2f",
		tc.name,
		candidate.name,
		topN,
		decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples))
	t.Logf("%-8s %-8s %-8s %-8s %-8s %10s %10s",
		"sfStart", "sfEnd", "sfLen", "frStart", "frEnd", "candRMS", "dSumSq")
	for _, row := range rows[:topN] {
		t.Logf("%-8d %-8d %-8d %-8d %-8d %10.2f %10d",
			row.start,
			row.end,
			row.end-row.start,
			row.start/2,
			(row.end+1)/2,
			decoderGainCandidateRMS(row.sumSq, len(frames)*frameSamples),
			row.improvement)
	}
}

type decoderGainCandidateFrontierRow struct {
	frame       int
	prod        decoderITUFrameStats
	candidate   decoderITUFrameStats
	improvement int64
	taps        Phase3DiagFrameTaps
}

type decoderGainCandidateCutoverRow struct {
	cutover     int
	sumSq       int64
	improvement int64
}

type decoderGainCandidateWindowRow struct {
	start       int
	end         int
	sumSq       int64
	improvement int64
}

type decoderUpstreamWindowFrameRow struct {
	frame       int
	prod        decoderITUFrameStats
	candidate   decoderITUFrameStats
	improvement int64
}

func phase3eDecodeVariantWindow(t *testing.T, bitData []byte, frames, start, end int, candidate phase3eVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[upstream-window=%d:%d] frame %d: %v", start, end, f, err)
		}
		variant := phase3eVariant{name: "production"}
		if f >= start && f < end {
			variant = candidate
		}
		if err := dec.decodeFramePhase3eVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3eVariant[window=%d:%d/%s] frame %d: %v", start, end, variant.name, f, err)
		}
	}
	return out
}

func phase3eDecodeVariantSubframeWindow(t *testing.T, bitData []byte, frames, startSubframe, endSubframe int, candidate phase3eVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[upstream-subwindow=%d:%d] frame %d: %v", startSubframe, endSubframe, f, err)
		}
		if err := dec.decodeFramePhase3eSubframeWindow(
			f,
			packed[:],
			out[f*frameSamples:(f+1)*frameSamples],
			startSubframe,
			endSubframe,
			candidate,
		); err != nil {
			t.Fatalf("decodeFramePhase3eSubframeWindow[window=%d:%d/%s] frame %d: %v", startSubframe, endSubframe, candidate.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3eSubframeWindow(
	frame int,
	packed []byte,
	out []int16,
	startSubframe, endSubframe int,
	candidate phase3eVariant,
) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return err
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

	sf0Variant := phase3eVariant{name: "production"}
	if globalSubframe := frame * 2; globalSubframe >= startSubframe && globalSubframe < endSubframe {
		sf0Variant = candidate
	}
	sf1Variant := phase3eVariant{name: "production"}
	if globalSubframe := frame*2 + 1; globalSubframe >= startSubframe && globalSubframe < endSubframe {
		sf1Variant = candidate
	}

	d.decodeSubframePhase3eVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], sf0Variant)
	d.decodeSubframePhase3eVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], sf1Variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func phase3jDecodeVariantCutover(t *testing.T, bitData []byte, frames, cutover int, candidate phase3jVariant) []int16 {
	t.Helper()
	return phase3jDecodeVariantWindow(t, bitData, frames, cutover, frames, candidate)
}

func phase3jDecodeVariantWindow(t *testing.T, bitData []byte, frames, start, end int, candidate phase3jVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3jGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[window=%d:%d] frame %d: %v", start, end, f, err)
		}
		variant := phase3jVariant{name: "gain_mirror_default"}
		if f >= start && f < end {
			variant = candidate
		}
		if err := dec.decodeFramePhase3jVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3jVariant[window=%d:%d/%s] frame %d: %v", start, end, variant.name, f, err)
		}
	}
	return out
}

func decoderGainCandidateFrontierVariant(t *testing.T) phase3jVariant {
	t.Helper()
	switch strings.TrimSpace(os.Getenv("G729_DECODER_GAIN_CANDIDATE")) {
	case "", "gain_ec_q25":
		return phase3jVariant{name: "gain_ec_q25", mode: phase3jGainECQ25}
	case "gain_ec_q27":
		return phase3jVariant{name: "gain_ec_q27", mode: phase3jGainECQ27}
	case "gain_ec_q13":
		return phase3jVariant{name: "gain_ec_q13", mode: phase3jGainECQ13}
	case "gain_gamma_q14":
		return phase3jVariant{name: "gain_gamma_q14", mode: phase3jGainGammaQ14}
	case "gain_gamma_q12":
		return phase3jVariant{name: "gain_gamma_q12", mode: phase3jGainGammaQ12}
	case "gain_update_10log":
		return phase3jVariant{name: "gain_update_10log", mode: phase3jGainUpdate10Log}
	default:
		t.Fatalf("unknown G729_DECODER_GAIN_CANDIDATE; supported: gain_ec_q25, gain_ec_q27, gain_ec_q13, gain_gamma_q14, gain_gamma_q12, gain_update_10log")
	}
	return phase3jVariant{}
}

func decoderUpstreamWindowVariant(t *testing.T) phase3eVariant {
	t.Helper()
	switch strings.TrimSpace(os.Getenv("G729_DECODER_UPSTREAM_WINDOW_CANDIDATE")) {
	case "", "fixed_gain_half":
		return phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1}
	case "fixed_gain_double":
		return phase3eVariant{name: "fixed_gain_double", fixedExpDelta: +1}
	case "force_pitch_frac_zero":
		return phase3eVariant{name: "force_pitch_frac_zero", forceTFracZero: true}
	case "flip_pitch_frac_sign":
		return phase3eVariant{name: "flip_pitch_frac_sign", flipTFracSign: true}
	case "no_fcb_pitch_enhancement":
		return phase3eVariant{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true}
	case "gain_unenhanced_c":
		return phase3eVariant{name: "gain_unenhanced_c", gainUnenhancedC: true}
	default:
		t.Fatalf("unknown G729_DECODER_UPSTREAM_WINDOW_CANDIDATE; supported: fixed_gain_half, fixed_gain_double, force_pitch_frac_zero, flip_pitch_frac_sign, no_fcb_pitch_enhancement, gain_unenhanced_c")
	}
	return phase3eVariant{}
}

func decoderGainCandidateLogRows(t *testing.T, rows []decoderGainCandidateFrontierRow) {
	t.Helper()
	t.Logf("%-6s %10s %10s %10s %8s %8s %8s %8s",
		"frame", "prodRMS", "candRMS", "dSumSq", "prodMax", "candMax", "prod1st", "cand1st")
	for _, row := range rows {
		t.Logf("%-6d %10.2f %10.2f %10d %8d %8d %8s %8s",
			row.frame,
			row.prod.rmsDelta(),
			row.candidate.rmsDelta(),
			row.improvement,
			row.prod.maxAbsDelta,
			row.candidate.maxAbsDelta,
			row.prod.firstDiffString(),
			row.candidate.firstDiffString())
		for sub := 0; sub < 2; sub++ {
			g := row.taps.Sub[sub].GainTaps
			e := decoderGainFrontierSubEnergy(row.taps.Sub[sub])
			t.Logf("  sf%d T=%d.%+d gp=%d gamma=%d gc=(mant=%d exp=%d) pred=%d ecBar=%d logGain=%d log2Gc=%d gc0=%d prod=%d uCur=%d rms[pitch=%.2f fixed=%.2f u=%.2f]",
				sub,
				row.taps.Sub[sub].TInt,
				row.taps.Sub[sub].TFrac,
				g.GpQ14Final,
				g.GammaCQ13,
				g.GcMantQ14,
				g.GcExp,
				g.Predicted,
				g.EcBarDbQ10,
				g.LogGainDbQ10,
				g.Log2GcQ10,
				g.Gc0Q14Unsat,
				g.ProdUnsat,
				g.UCurrent,
				e.pitchRMS,
				e.fixedRMS,
				e.uRMS)
		}
	}
}

func decoderUpstreamWindowLogRows(t *testing.T, rows []decoderUpstreamWindowFrameRow) {
	t.Helper()
	t.Logf("%-6s %10s %10s %10s %8s %8s %8s %8s",
		"frame", "prodRMS", "candRMS", "dSumSq", "prodMax", "candMax", "prod1st", "cand1st")
	for _, row := range rows {
		t.Logf("%-6d %10.2f %10.2f %10d %8d %8d %8s %8s",
			row.frame,
			row.prod.rmsDelta(),
			row.candidate.rmsDelta(),
			row.improvement,
			row.prod.maxAbsDelta,
			row.candidate.maxAbsDelta,
			row.prod.firstDiffString(),
			row.candidate.firstDiffString())
	}
}

func decoderGainCandidateRMS(sumSq int64, samples int) float64 {
	if samples <= 0 {
		return 0
	}
	return math.Sqrt(float64(sumSq) / float64(samples))
}

func decoderGainCandidateOutputSumSq(out []int16, want [][frameSamples]int16) int64 {
	var sumSq int64
	for frame := range want {
		stats := decoderGainFrontierCompareFrame(out[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		sumSq += stats.sumSqDelta
	}
	return sumSq
}
