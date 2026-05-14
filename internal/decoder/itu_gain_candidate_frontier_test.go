package decoder

import (
	"bytes"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
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

func phase3jDecodeVariantCutover(t *testing.T, bitData []byte, frames, cutover int, candidate phase3jVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3jGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[cutover=%d] frame %d: %v", cutover, f, err)
		}
		variant := phase3jVariant{name: "gain_mirror_default"}
		if f >= cutover {
			variant = candidate
		}
		if err := dec.decodeFramePhase3jVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3jVariant[cutover=%d/%s] frame %d: %v", cutover, variant.name, f, err)
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
