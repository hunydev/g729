package decoder

import (
	"math"
	"os"
	"sort"
	"testing"
)

// TestDecoderITUFixedGainFrontier is an opt-in PST-output diagnostic for the
// current TAME fixed-gain suspicion. It compares production decode with a
// fixed-gain-half perturbation, then logs the frames where that perturbation
// most reduces final-output squared error. It does not assert; the output is a
// triage aid for choosing the next clean-room numeric target.
func TestDecoderITUFixedGainFrontier(t *testing.T) {
	if os.Getenv("G729_DECODER_FIXED_GAIN_FRONTIER") != "1" {
		t.Skip("set G729_DECODER_FIXED_GAIN_FRONTIER=1 to run fixed-gain frontier")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_FIXED_GAIN_VECTOR", "TAME")
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
	halfOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1})

	tapsByFrame := decoderGainFrontierTaps(t, frames)
	rows := make([]decoderGainFrontierRow, 0, len(frames))
	for frame := range frames {
		prodStats := decoderGainFrontierCompareFrame(prodOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		halfStats := decoderGainFrontierCompareFrame(halfOut[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		rows = append(rows, decoderGainFrontierRow{
			frame:       frame,
			prod:        prodStats,
			half:        halfStats,
			improvement: prodStats.sumSqDelta - halfStats.sumSqDelta,
			taps:        tapsByFrame[frame],
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].improvement != rows[j].improvement {
			return rows[i].improvement > rows[j].improvement
		}
		if rows[i].prod.sumSqDelta != rows[j].prod.sumSqDelta {
			return rows[i].prod.sumSqDelta > rows[j].prod.sumSqDelta
		}
		return rows[i].frame < rows[j].frame
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}
	t.Logf("decoder ITU fixed-gain frontier: vector=%s topN=%d", tc.name, topN)
	t.Logf("%-6s %10s %10s %10s %8s %8s %8s %8s",
		"frame", "prodRMS", "halfRMS", "dSumSq", "prodMax", "halfMax", "prod1st", "half1st")
	for i := 0; i < topN; i++ {
		row := rows[i]
		t.Logf("%-6d %10.2f %10.2f %10d %8d %8d %8s %8s",
			row.frame,
			row.prod.rmsDelta(),
			row.half.rmsDelta(),
			row.improvement,
			row.prod.maxAbsDelta,
			row.half.maxAbsDelta,
			row.prod.firstDiffString(),
			row.half.firstDiffString())
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

type decoderGainFrontierRow struct {
	frame       int
	prod        decoderITUFrameStats
	half        decoderITUFrameStats
	improvement int64
	taps        Phase3DiagFrameTaps
}

type decoderGainFrontierEnergy struct {
	pitchRMS float64
	fixedRMS float64
	uRMS     float64
}

func decoderGainFrontierTaps(t *testing.T, frames [][]byte) []Phase3DiagFrameTaps {
	t.Helper()
	out := make([]Phase3DiagFrameTaps, len(frames))
	var dec Decoder
	for i, packed := range frames {
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", i, err)
		}
		out[i] = taps
	}
	return out
}

func decoderGainFrontierCompareFrame(got []int16, want *[frameSamples]int16) decoderITUFrameStats {
	var gotFrame [frameSamples]int16
	copy(gotFrame[:], got)
	return decoderITUCompareFrame(&gotFrame, want)
}

func decoderGainFrontierSubEnergy(sub Phase3DiagSubframeTaps) decoderGainFrontierEnergy {
	gc := float64(sub.GainTaps.GcMantQ14) / 16384.0
	gc = math.Ldexp(gc, int(sub.GainTaps.GcExp))

	var pitchSum, fixedSum, uSum float64
	for i := 0; i < subframeLen; i++ {
		pitchPart := float64(sub.GpQ14) * float64(sub.V[i]) / 16384.0
		fixedPart := gc * float64(sub.C[i]) / 8192.0
		u := float64(sub.U[i])
		pitchSum += pitchPart * pitchPart
		fixedSum += fixedPart * fixedPart
		uSum += u * u
	}
	return decoderGainFrontierEnergy{
		pitchRMS: math.Sqrt(pitchSum / subframeLen),
		fixedRMS: math.Sqrt(fixedSum / subframeLen),
		uRMS:     math.Sqrt(uSum / subframeLen),
	}
}
