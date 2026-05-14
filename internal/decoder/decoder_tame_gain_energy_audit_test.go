package decoder

import (
	"math"
	"os"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/gain"
)

// TestDecoderTAMEGainEnergyAudit checks whether TAME history growth coincides
// with fixed-codebook energy saturation or suspicious gain-decoder state. It is
// opt-in and diagnostic-only.
func TestDecoderTAMEGainEnergyAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_GAIN_ENERGY_AUDIT") != "1" {
		t.Skip("set G729_DECODER_TAME_GAIN_ENERGY_AUDIT=1 to run TAME gain energy audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_TAME_GAIN_ENERGY_VECTOR", "TAME")
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
	rows := make([]decoderTAMEGainEnergyRow, 0, len(frames)*2)
	var saturated int
	for frame, packed := range frames {
		taps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, frame, err)
		}
		stats := decoderITUCompareFrame(&taps.Output, &want[frame])
		pstRMS := decoderHistoryRMS(want[frame][:])
		outRMS := decoderHistoryRMS(taps.Output[:])
		for sub := 0; sub < 2; sub++ {
			row := decoderTAMEGainEnergyRowFromTaps(frame, sub, taps.Sub[sub], pstRMS, outRMS, stats)
			if row.energySaturated {
				saturated++
			}
			rows = append(rows, row)
		}
	}

	topN := decoderITUFrontierTopN()
	t.Logf("decoder TAME gain energy audit: vector=%s frames=%d subframes=%d energySaturated=%d",
		tc.name, len(frames), len(rows), saturated)

	t.Logf("onset context")
	decoderTAMEGainEnergyLogRange(t, rows, 26*2, 54*2)

	byEnergy := append([]decoderTAMEGainEnergyRow(nil), rows...)
	sort.Slice(byEnergy, func(i, j int) bool {
		if byEnergy[i].energyQ26 != byEnergy[j].energyQ26 {
			return byEnergy[i].energyQ26 > byEnergy[j].energyQ26
		}
		return byEnergy[i].globalSubframe < byEnergy[j].globalSubframe
	})
	if topN > len(byEnergy) {
		topN = len(byEnergy)
	}
	t.Logf("top fixed-codebook energy rows")
	decoderTAMEGainEnergyLogRows(t, byEnergy[:topN])

	byProd := append([]decoderTAMEGainEnergyRow(nil), rows...)
	sort.Slice(byProd, func(i, j int) bool {
		li := decoderTAMEAbsInt32(byProd[i].prodUnsatQ12)
		lj := decoderTAMEAbsInt32(byProd[j].prodUnsatQ12)
		if li != lj {
			return li > lj
		}
		return byProd[i].globalSubframe < byProd[j].globalSubframe
	})
	if topN > len(byProd) {
		topN = len(byProd)
	}
	t.Logf("top unsaturated fixed-gain product rows")
	decoderTAMEGainEnergyLogRows(t, byProd[:topN])
}

type decoderTAMEGainEnergyRow struct {
	globalSubframe  int
	frame           int
	sub             int
	outRatio        float64
	rmsErr          float64
	maxAbs          int
	energyQ26       int64
	energySatQ26    int32
	energySaturated bool
	ecBarDb         float64
	predDb          float64
	logGainDb       float64
	log2Gc          float64
	gammaCQ13       int16
	gcMantQ14       int16
	gcExp           int8
	prodUnsatQ12    int32
	gcQ12           int16
	uCurrentDb      float64
	pastRMS         float64
	vRMS            float64
	pitchRMS        float64
	fixedRMS        float64
	uRMS            float64
}

func decoderTAMEGainEnergyRowFromTaps(frame, sub int, taps Phase3DiagSubframeTaps, pstRMS, outRMS float64, stats decoderITUFrameStats) decoderTAMEGainEnergyRow {
	g := taps.GainTaps
	energyQ26 := decoderTAMEFixedCodebookEnergy64(taps.C[:])
	energySat := int32(gain.FixedCodebookEnergy(&taps.C))
	energy := decoderGainFrontierSubEnergy(taps)
	return decoderTAMEGainEnergyRow{
		globalSubframe:  frame*2 + sub,
		frame:           frame,
		sub:             sub,
		outRatio:        safeRatioFloat64(outRMS, pstRMS),
		rmsErr:          stats.rmsDelta(),
		maxAbs:          stats.maxAbsDelta,
		energyQ26:       energyQ26,
		energySatQ26:    energySat,
		energySaturated: energyQ26 > math.MaxInt32,
		ecBarDb:         float64(g.EcBarDbQ10) / 1024.0,
		predDb:          float64(g.Predicted) / 1024.0,
		logGainDb:       float64(g.LogGainDbQ10) / 1024.0,
		log2Gc:          float64(g.Log2GcQ10) / 1024.0,
		gammaCQ13:       g.GammaCQ13,
		gcMantQ14:       g.GcMantQ14,
		gcExp:           g.GcExp,
		prodUnsatQ12:    g.ProdUnsat,
		gcQ12:           g.GcQ12Final,
		uCurrentDb:      float64(g.UCurrent) / 1024.0,
		pastRMS:         decoderHistoryRMS(taps.PastExcPreACB[:]),
		vRMS:            envelopeRMS(taps.V[:]),
		pitchRMS:        energy.pitchRMS,
		fixedRMS:        energy.fixedRMS,
		uRMS:            envelopeRMS(taps.U[:]),
	}
}

func decoderTAMEFixedCodebookEnergy64(c []int16) int64 {
	var energy int64
	for _, sample := range c {
		s := int64(sample)
		energy += s * s
	}
	return energy
}

func decoderTAMEAbsInt32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func decoderTAMEGainEnergyLogRange(t *testing.T, rows []decoderTAMEGainEnergyRow, start, end int) {
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
	decoderTAMEGainEnergyLogRows(t, rows[start:end])
}

func decoderTAMEGainEnergyLogRows(t *testing.T, rows []decoderTAMEGainEnergyRow) {
	t.Helper()
	t.Logf("%5s %5s %3s %7s %8s %7s %12s %12s %3s %8s %8s %8s %8s %8s %8s %7s %6s %6s %8s %8s %8s %8s %8s %8s",
		"sf", "frame", "sub", "out/PST", "rmsErr", "maxAbs",
		"energy", "satEnergy", "sat", "ecBar", "pred", "logGain",
		"log2Gc", "gamma", "prodQ12", "gcQ12", "mant", "exp",
		"uCur", "past", "v", "pitch", "fixed", "u")
	for _, r := range rows {
		t.Logf("%5d %5d %3d %7.3f %8.1f %7d %12d %12d %3t %8.2f %8.2f %8.2f %8.2f %8d %8d %7d %6d %6d %8.2f %8.1f %8.1f %8.1f %8.1f %8.1f",
			r.globalSubframe,
			r.frame,
			r.sub,
			r.outRatio,
			r.rmsErr,
			r.maxAbs,
			r.energyQ26,
			r.energySatQ26,
			r.energySaturated,
			r.ecBarDb,
			r.predDb,
			r.logGainDb,
			r.log2Gc,
			r.gammaCQ13,
			r.prodUnsatQ12,
			r.gcQ12,
			r.gcMantQ14,
			r.gcExp,
			r.uCurrentDb,
			r.pastRMS,
			r.vRMS,
			r.pitchRMS,
			r.fixedRMS,
			r.uRMS)
	}
}
