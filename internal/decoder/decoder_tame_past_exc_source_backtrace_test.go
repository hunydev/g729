package decoder

import (
	"math"
	"os"
	"sort"
	"testing"
)

// TestDecoderTAMEPastExcSourceBacktraceAudit maps filled late
// past_exc_pre_acb_q0 oracle rows back to the earlier U subframes that populated
// each pastExc sample. It is a diagnostic-only bridge between the late oracle
// and the upstream feedback trajectory.
func TestDecoderTAMEPastExcSourceBacktraceAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_PAST_EXC_SOURCE_BACKTRACE") != "1" {
		t.Skip("set G729_DECODER_TAME_PAST_EXC_SOURCE_BACKTRACE=1 to run TAME pastExc source backtrace audit")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_CHECKPOINT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	expected, err := readDecoderTAMEACBCheckpointRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}
	filled := decoderTAMEFilledStageRows(expected, "past_exc_pre_acb_q0")
	if len(filled) == 0 {
		t.Fatalf("no filled past_exc_pre_acb_q0 rows in %s", expectedPath)
	}

	candidates := decoderTAMEPastExcSourceBacktraceCandidates(t, expected)
	t.Logf("decoder TAME pastExc source backtrace: path=%s filledPastExc=%d candidates=%d",
		expectedPath, len(filled), len(candidates))
	t.Logf("%-24s %8s %8s %8s %8s %8s %7s %7s %7s",
		"candidate", "count", "refRMS", "gotRMS", "errRMS", "scErr", "corr", "scale", "maxAbs")
	for _, candidate := range candidates {
		total, bySource, missing := decoderTAMEPastExcSourceBacktraceStats(t, filled, candidate.rows)
		t.Logf("%-24s %8d %8.2f %8.2f %8.2f %8.2f %7.4f %7.4f %7d",
			candidate.name,
			total.count,
			total.refRMS,
			total.gotRMS,
			total.errRMS,
			total.scaledErrRMS,
			total.corr,
			total.scale,
			total.maxAbs)
		if missing != 0 {
			t.Fatalf("%s missing %d got rows", candidate.name, missing)
		}

		t.Logf("%s by source U subframe", candidate.name)
		t.Logf("%-8s %-7s %-7s %-8s %-8s %-8s %-8s %-8s %-7s %-7s %-7s",
			"srcSF", "frame", "sub", "samples", "refRMS", "gotRMS", "errRMS", "scErr", "corr", "scale", "maxAbs")
		sourceKeys := make([]int, 0, len(bySource))
		for source := range bySource {
			sourceKeys = append(sourceKeys, source)
		}
		sort.Ints(sourceKeys)
		for _, source := range sourceKeys {
			stats := bySource[source].finish()
			t.Logf("%-8d %-7d %-7d %-8d %-8.2f %-8.2f %-8.2f %-8.2f %-7.4f %-7.4f %-7d",
				source,
				source/2,
				source%2,
				stats.count,
				stats.refRMS,
				stats.gotRMS,
				stats.errRMS,
				stats.scaledErrRMS,
				stats.corr,
				stats.scale,
				stats.maxAbs)
		}
	}
}

type decoderTAMEPastExcSourceCandidate struct {
	name string
	rows []stageRow
}

func decoderTAMEPastExcSourceBacktraceCandidates(t *testing.T, expected []stageRow) []decoderTAMEPastExcSourceCandidate {
	t.Helper()
	production, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect production TAME ACB checkpoint rows: %v", err)
	}

	fixedHalf := phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1}
	pitchCap := phase3eVariant{name: "pitch_gain_cap_0p95", pitchCapQ14: 15565}
	zeroAdaptive := phase3eVariant{name: "zero_adaptive", zeroAdaptive: true}

	return []decoderTAMEPastExcSourceCandidate{
		{name: "production", rows: production},
		{name: "fixed_half_f26_120", rows: decoderTAMECollectPastExcWindowRows(t, expected, 52, 120*2, fixedHalf)},
		{name: "fixed_half_sf52_239", rows: decoderTAMECollectPastExcWindowRows(t, expected, 52, 239, fixedHalf)},
		{name: "pitch_cap_f49_72", rows: decoderTAMECollectPastExcWindowRows(t, expected, 49*2, 72*2, pitchCap)},
		{name: "zero_adaptive_f49_72", rows: decoderTAMECollectPastExcWindowRows(t, expected, 49*2, 72*2, zeroAdaptive)},
	}
}

func decoderTAMECollectPastExcWindowRows(t *testing.T, expected []stageRow, startSubframe, endSubframe int, candidate phase3eVariant) []stageRow {
	t.Helper()
	rows, err := collectDecoderTAMEPastExcRowsWithSubframeWindow(t, expected, startSubframe, endSubframe, candidate)
	if err != nil {
		t.Fatalf("collect %s TAME pastExc rows [%d,%d): %v", candidate.name, startSubframe, endSubframe, err)
	}
	return rows
}

func decoderTAMEPastExcSourceBacktraceStats(t *testing.T, expected, got []stageRow) (decoderTAMEScalarCompare, map[int]*decoderTAMEScalarCompareAgg, int) {
	t.Helper()
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var totalAgg decoderTAMEScalarCompareAgg
	bySource := make(map[int]*decoderTAMEScalarCompareAgg)
	var missing int
	for _, want := range expected {
		key := decoderStageRowKey(want)
		gotRow, ok := gotByKey[key]
		if !ok || !gotRow.hasValue {
			missing++
			continue
		}
		source := decoderTAMEPastExcSourceSubframe(want.frame, want.sub, want.index)
		agg := bySource[source]
		if agg == nil {
			agg = &decoderTAMEScalarCompareAgg{}
			bySource[source] = agg
		}
		totalAgg.add(want.value, gotRow.value)
		agg.add(want.value, gotRow.value)
	}
	return totalAgg.finish(), bySource, missing
}

func decoderTAMEPastExcSourceSubframe(frame, sub, index int) int {
	globalSubframe := frame*2 + sub
	absSample := globalSubframe*subframeLen - pastExcLen + index
	if absSample < 0 {
		return -1
	}
	return absSample / subframeLen
}

func decoderTAMEFilledStageRows(rows []stageRow, field string) []stageRow {
	out := make([]stageRow, 0, len(rows))
	for _, row := range rows {
		if row.source == "TAME" && row.field == field && row.hasValue {
			out = append(out, row)
		}
	}
	return out
}

type decoderTAMEScalarCompare struct {
	count        int
	refRMS       float64
	gotRMS       float64
	errRMS       float64
	scaledErrRMS float64
	corr         float64
	scale        float64
	maxAbs       int64
}

type decoderTAMEScalarCompareAgg struct {
	count  int
	refSq  float64
	gotSq  float64
	errSq  float64
	dot    float64
	maxAbs int64
}

func (a *decoderTAMEScalarCompareAgg) add(ref, got int64) {
	r := float64(ref)
	g := float64(got)
	d := r - g
	a.count++
	a.refSq += r * r
	a.gotSq += g * g
	a.errSq += d * d
	a.dot += r * g
	if ad := absInt64(ref - got); ad > a.maxAbs {
		a.maxAbs = ad
	}
}

func (a decoderTAMEScalarCompareAgg) finish() decoderTAMEScalarCompare {
	if a.count == 0 {
		return decoderTAMEScalarCompare{}
	}
	scale := 0.0
	if a.gotSq != 0 {
		scale = a.dot / a.gotSq
	}
	scaledErrSq := a.refSq - 2*scale*a.dot + scale*scale*a.gotSq
	if scaledErrSq < 0 && scaledErrSq > -1e-6 {
		scaledErrSq = 0
	}
	corr := 0.0
	if a.refSq != 0 && a.gotSq != 0 {
		corr = a.dot / math.Sqrt(a.refSq*a.gotSq)
	}
	return decoderTAMEScalarCompare{
		count:        a.count,
		refRMS:       math.Sqrt(a.refSq / float64(a.count)),
		gotRMS:       math.Sqrt(a.gotSq / float64(a.count)),
		errRMS:       math.Sqrt(a.errSq / float64(a.count)),
		scaledErrRMS: math.Sqrt(math.Max(0, scaledErrSq) / float64(a.count)),
		corr:         corr,
		scale:        scale,
		maxAbs:       a.maxAbs,
	}
}
