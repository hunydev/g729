package decoder

import (
	"os"
	"testing"

	"github.com/hunydev/g729/internal/fcb"
)

// TestDecoderInitialPitchEnhancementBetaAudit checks whether the stream-start
// fixed-codebook pitch-enhancement coefficient explains ITU-vector drift. It is
// diagnostic-only: production intentionally owns the first-subframe beta policy
// in Decoder.pitchEnhancementBetaQ14.
func TestDecoderInitialPitchEnhancementBetaAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_INITIAL_BETA_AUDIT") != "1" {
		t.Skip("set G729_DECODER_INITIAL_BETA_AUDIT=1 to run initial pitch-enhancement beta audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_INITIAL_BETA_VECTOR", "TAME")
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

	ref := decoderInitialBetaFlattenPST(want)
	variants := []decoderInitialBetaVariant{
		{name: "production"},
		{name: "seed_upper", seedPrevGain: true, prevGpQ14: fcb.InitialPitchEnhancementQ14},
		{name: "seed_lower", seedPrevGain: true, prevGpQ14: 0},
	}

	t.Logf("decoder initial beta audit: vector=%s frames=%d initialProductionBeta=%d",
		tc.name, len(frames), fcb.InitialPitchEnhancementQ14)
	t.Logf("%-12s %8s %8s %9s %10s %9s %11s %8s %10s",
		"variant", "gSNR", "segSNR", "corr", "rmsDelta", "maxAbs", "exactSamp", "lag", "bestSNR")

	var baseOut []int16
	var baseMetric blackboxMetrics
	for i, variant := range variants {
		out := decoderInitialBetaDecode(t, frames, bads, variant)
		metric := blackboxMeasure(ref, out, 40)
		stats := decoderInitialBetaStats(out, want)
		if i == 0 {
			baseOut = out
			baseMetric = metric
		}
		t.Logf("%-12s %8.2f %8.2f %9.4f %10.2f %9d %11s %8d %10.2f",
			variant.name, metric.globalSNR, metric.segSNR, metric.corr, stats.rmsDelta(),
			stats.maxAbsDelta, decoderITUPercent(stats.exactSamples, stats.samples),
			metric.bestSNRLag, metric.bestSNR)
		if i > 0 {
			delta := decoderInitialBetaDiff(baseOut, out)
			t.Logf("  delta vs production: changedSamples=%d maxAbs=%d dGsnr=%+.4f dCorr=%+.6f",
				delta.changedSamples, delta.maxAbsDelta,
				metric.globalSNR-baseMetric.globalSNR, metric.corr-baseMetric.corr)
		}
	}
}

type decoderInitialBetaVariant struct {
	name         string
	seedPrevGain bool
	prevGpQ14    int16
}

func decoderInitialBetaDecode(t *testing.T, frames [][]byte, bads []bool, variant decoderInitialBetaVariant) []int16 {
	t.Helper()
	out := make([]int16, len(frames)*frameSamples)
	var dec Decoder
	if variant.seedPrevGain {
		dec.havePrevGpQ14 = true
		dec.prevGpQ14 = variant.prevGpQ14
	}
	for frame, packed := range frames {
		if err := dec.Decode(packed, bads[frame], out[frame*frameSamples:(frame+1)*frameSamples]); err != nil {
			t.Fatalf("%s frame %d Decode: %v", variant.name, frame, err)
		}
	}
	return out
}

func decoderInitialBetaFlattenPST(frames [][frameSamples]int16) []int16 {
	out := make([]int16, len(frames)*frameSamples)
	for frame := range frames {
		copy(out[frame*frameSamples:(frame+1)*frameSamples], frames[frame][:])
	}
	return out
}

func decoderInitialBetaStats(out []int16, want [][frameSamples]int16) decoderITUVectorStats {
	stats := decoderITUVectorStats{firstFrame: -1, firstSample: -1}
	for frame := range want {
		frameStats := decoderGainFrontierCompareFrame(out[frame*frameSamples:(frame+1)*frameSamples], &want[frame])
		stats.frames++
		stats.samples += frameSamples
		stats.exactSamples += frameStats.exactSamples
		if frameStats.exactSamples == frameSamples {
			stats.exactFrames++
		}
		stats.sumAbsDelta += frameStats.sumAbsDelta
		stats.sumSqDelta += frameStats.sumSqDelta
		if frameStats.maxAbsDelta > stats.maxAbsDelta {
			stats.maxAbsDelta = frameStats.maxAbsDelta
		}
		if stats.firstFrame < 0 && frameStats.firstSample >= 0 {
			stats.firstFrame = frame
			stats.firstSample = frameStats.firstSample
			stats.firstGot = frameStats.firstGot
			stats.firstWant = frameStats.firstWant
		}
	}
	return stats
}

type decoderInitialBetaDiffStats struct {
	changedSamples int
	maxAbsDelta    int
}

func decoderInitialBetaDiff(a, b []int16) decoderInitialBetaDiffStats {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var stats decoderInitialBetaDiffStats
	for i := 0; i < n; i++ {
		delta := int(a[i]) - int(b[i])
		if delta < 0 {
			delta = -delta
		}
		if delta == 0 {
			continue
		}
		stats.changedSamples++
		if delta > stats.maxAbsDelta {
			stats.maxAbsDelta = delta
		}
	}
	stats.changedSamples += len(a) - n
	stats.changedSamples += len(b) - n
	return stats
}
