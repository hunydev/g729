package decoder

import (
	"math"
	"os"
	"sort"
	"testing"
)

// TestDecoderTAMEACBOracleShape compares local adaptive-codebook vectors with
// the numeric TAME wide-stage oracle. It separates two cases that require
// different fixes: a mostly scalar history-envelope error vs a vector-shape
// error caused by earlier excitation history.
func TestDecoderTAMEACBOracleShape(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ACB_ORACLE_SHAPE") != "1" {
		t.Skip("set G729_DECODER_TAME_ACB_ORACLE_SHAPE=1 to run TAME ACB oracle shape audit")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_ACB_ORACLE_VECTOR", "TAME")
	candidate := decoderUpstreamWindowVariant(t)
	startSubframe := decoderITUEnvInt("G729_DECODER_HISTORY_START_SUBFRAME", 6)
	endSubframe := decoderITUEnvInt("G729_DECODER_HISTORY_END_SUBFRAME", 240)

	bitPath := vectorPath(tc.bitFile)
	ensureTestdataPresent(t, bitPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	frames, bads := readG192Frames(t, bitPath)
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch bads=%d frames=%d", tc.name, len(bads), len(frames))
	}
	if startSubframe < 0 || endSubframe > len(frames)*2 || startSubframe >= endSubframe {
		t.Fatalf("invalid subframe window [%d,%d) for %d frames", startSubframe, endSubframe, len(frames))
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_ORACLE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	oracle := decoderTAMEACBSubframeOverrides(t, expectedPath)
	if len(oracle) == 0 {
		t.Fatalf("no adaptive_v_q0 oracle rows in %s", expectedPath)
	}

	prodOut, prodRows := decoderHistoryDecodeWindow(t, bitData, len(frames), 0, 0, phase3eVariant{name: "production"})
	candOut, candRows := decoderHistoryDecodeWindow(t, bitData, len(frames), startSubframe, endSubframe, candidate)
	baseline := decodeVariant(t, bitData, len(frames), nil, nil)
	if !phase3eEqualPCM(baseline, prodOut) {
		t.Fatalf("ACB oracle production mirror diverges from Decoder.Decode baseline")
	}
	if phase3eEqualPCM(prodOut, candOut) {
		t.Fatalf("candidate %s produced identical output; check window [%d,%d)", candidate.name, startSubframe, endSubframe)
	}

	prodByKey := decoderACBRowsByKey(prodRows)
	candByKey := decoderACBRowsByKey(candRows)
	rows := make([]decoderACBOracleRow, 0, len(oracle))
	var prodAgg, candAgg decoderACBOracleAggregate
	for key, want := range oracle {
		prod, ok := prodByKey[key]
		if !ok {
			t.Fatalf("missing production ACB row for frame=%d sub=%d", key.frame, key.sub)
		}
		cand, ok := candByKey[key]
		if !ok {
			t.Fatalf("missing candidate ACB row for frame=%d sub=%d", key.frame, key.sub)
		}
		prodCmp := decoderCompareACBOracle(want, prod.v)
		candCmp := decoderCompareACBOracle(want, cand.v)
		prodAgg.add(want, prod.v)
		candAgg.add(want, cand.v)
		rows = append(rows, decoderACBOracleRow{
			key:         key,
			tInt:        prod.tInt,
			tFrac:       prod.tFrac,
			inWindow:    cand.inWindow,
			production:  prodCmp,
			candidate:   candCmp,
			improvement: prodCmp.errRMS - candCmp.errRMS,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].key.frame != rows[j].key.frame {
			return rows[i].key.frame < rows[j].key.frame
		}
		return rows[i].key.sub < rows[j].key.sub
	})

	prodAll := prodAgg.finish()
	candAll := candAgg.finish()
	t.Logf("decoder TAME ACB oracle shape: vector=%s candidate=%s subwindow=[%d,%d) oracleSubframes=%d",
		tc.name, candidate.name, startSubframe, endSubframe, len(oracle))
	t.Logf("aggregate %-10s refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		"production", prodAll.refRMS, prodAll.gotRMS, prodAll.errRMS, prodAll.scaledErrRMS, prodAll.corr, prodAll.scale, prodAll.maxAbs)
	t.Logf("aggregate %-10s refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		"candidate", candAll.refRMS, candAll.gotRMS, candAll.errRMS, candAll.scaledErrRMS, candAll.corr, candAll.scale, candAll.maxAbs)

	t.Logf("%5s %3s %8s %5s %7s %8s %8s %8s %8s %7s %7s %8s %8s %8s %8s %7s %7s",
		"frame", "sub", "T", "win", "dErr",
		"pErr", "pScErr", "pRMS", "pRef", "pCorr", "pScale",
		"cErr", "cScErr", "cRMS", "cRef", "cCorr", "cScale")
	for _, row := range rows {
		t.Logf("%5d %3d %5d.%+d %5t %7.2f %8.2f %8.2f %8.2f %8.2f %7.4f %7.4f %8.2f %8.2f %8.2f %8.2f %7.4f %7.4f",
			row.key.frame,
			row.key.sub,
			row.tInt,
			row.tFrac,
			row.inWindow,
			row.improvement,
			row.production.errRMS,
			row.production.scaledErrRMS,
			row.production.gotRMS,
			row.production.refRMS,
			row.production.corr,
			row.production.scale,
			row.candidate.errRMS,
			row.candidate.scaledErrRMS,
			row.candidate.gotRMS,
			row.candidate.refRMS,
			row.candidate.corr,
			row.candidate.scale)
	}
}

type decoderACBOracleRow struct {
	key         decoderFrameSubKey
	tInt        int
	tFrac       int
	inWindow    bool
	production  decoderACBOracleCompare
	candidate   decoderACBOracleCompare
	improvement float64
}

type decoderACBOracleCompare struct {
	refRMS       float64
	gotRMS       float64
	errRMS       float64
	scaledErrRMS float64
	corr         float64
	scale        float64
	maxAbs       int
}

type decoderACBOracleAggregate struct {
	refSq     float64
	gotSq     float64
	errSq     float64
	scaledErr float64
	dot       float64
	maxAbs    int
	count     int
}

func decoderACBRowsByKey(rows []decoderHistorySubframeMetrics) map[decoderFrameSubKey]decoderHistorySubframeMetrics {
	out := make(map[decoderFrameSubKey]decoderHistorySubframeMetrics, len(rows))
	for _, row := range rows {
		out[decoderFrameSubKey{frame: row.frame, sub: row.sub}] = row
	}
	return out
}

func decoderCompareACBOracle(ref, got [subframeLen]int16) decoderACBOracleCompare {
	var agg decoderACBOracleAggregate
	agg.add(ref, got)
	return agg.finish()
}

func (a *decoderACBOracleAggregate) add(ref, got [subframeLen]int16) {
	for i := 0; i < subframeLen; i++ {
		r := float64(ref[i])
		g := float64(got[i])
		d := r - g
		a.refSq += r * r
		a.gotSq += g * g
		a.errSq += d * d
		a.dot += r * g
		if ad := absInt(int(ref[i]) - int(got[i])); ad > a.maxAbs {
			a.maxAbs = ad
		}
		a.count++
	}
}

func (a decoderACBOracleAggregate) finish() decoderACBOracleCompare {
	if a.count == 0 {
		return decoderACBOracleCompare{}
	}
	scale := 0.0
	if a.gotSq != 0 {
		scale = a.dot / a.gotSq
	}
	// Reconstruct the scale-only residual from the sufficient statistics:
	// ||r - scale*g||² = ||r||² - 2*scale*(r·g) + scale²*||g||².
	scaledErrSq := a.refSq - 2*scale*a.dot + scale*scale*a.gotSq
	if scaledErrSq < 0 && scaledErrSq > -1e-6 {
		scaledErrSq = 0
	}
	corr := 0.0
	if a.refSq != 0 && a.gotSq != 0 {
		corr = a.dot / math.Sqrt(a.refSq*a.gotSq)
	}
	return decoderACBOracleCompare{
		refRMS:       math.Sqrt(a.refSq / float64(a.count)),
		gotRMS:       math.Sqrt(a.gotSq / float64(a.count)),
		errRMS:       math.Sqrt(a.errSq / float64(a.count)),
		scaledErrRMS: math.Sqrt(math.Max(0, scaledErrSq) / float64(a.count)),
		corr:         corr,
		scale:        scale,
		maxAbs:       a.maxAbs,
	}
}
