package decoder

import (
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
)

// TestDecoderTAMEPastExcAgeMap maps filled past_exc_pre_acb_q0 oracle
// mismatches back to the prior excitation sample that populated each FIFO slot.
// This does not fix anything; it localizes whether late TAME history drift is
// concentrated in a recent subframe band or spread across the whole 153-sample
// adaptive-codebook history.
func TestDecoderTAMEPastExcAgeMap(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_PAST_EXC_AGE_MAP") != "1" {
		t.Skip("set G729_DECODER_TAME_PAST_EXC_AGE_MAP=1 to run TAME past-excitation age map")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_CHECKPOINT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	expected, err := readDecoderTAMEACBCheckpointRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}
	got, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder TAME ACB checkpoint got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	byOrigin := make(map[int]*decoderPastExcAgeGroup)
	byCurrent := make(map[decoderFrameSubKey]*decoderPastExcAgeGroup)
	byIndexBand := make(map[string]*decoderPastExcAgeGroup)
	byAgeBand := make(map[string]*decoderPastExcAgeGroup)
	var overall decoderPastExcAgeGroup
	first := make([]decoderPastExcAgeSample, 0, 12)
	for _, want := range expected {
		if want.field != "past_exc_pre_acb_q0" || !want.hasValue {
			continue
		}
		gotRow, ok := gotByKey[decoderStageRowKey(want)]
		if !ok || !gotRow.hasValue {
			t.Fatalf("missing local past_exc_pre_acb_q0 got row: frame=%d sub=%d index=%d", want.frame, want.sub, want.index)
		}
		sample := decoderPastExcAgeSampleFromRows(want, gotRow)
		overall.add(sample)
		if len(first) < cap(first) && sample.diff != 0 {
			first = append(first, sample)
		}

		originGroup := byOrigin[sample.originGlobalSubframe]
		if originGroup == nil {
			originGroup = &decoderPastExcAgeGroup{label: decoderPastExcOriginLabel(sample.originGlobalSubframe)}
			byOrigin[sample.originGlobalSubframe] = originGroup
		}
		originGroup.add(sample)

		currentKey := decoderFrameSubKey{frame: sample.currentFrame, sub: sample.currentSub}
		currentGroup := byCurrent[currentKey]
		if currentGroup == nil {
			currentGroup = &decoderPastExcAgeGroup{label: decoderPastExcCurrentLabel(currentKey)}
			byCurrent[currentKey] = currentGroup
		}
		currentGroup.add(sample)

		indexLabel := decoderPastExcBandLabel(sample.index, 20)
		indexGroup := byIndexBand[indexLabel]
		if indexGroup == nil {
			indexGroup = &decoderPastExcAgeGroup{label: indexLabel}
			byIndexBand[indexLabel] = indexGroup
		}
		indexGroup.add(sample)

		ageLabel := decoderPastExcBandLabel(sample.age, 20)
		ageGroup := byAgeBand[ageLabel]
		if ageGroup == nil {
			ageGroup = &decoderPastExcAgeGroup{label: ageLabel}
			byAgeBand[ageLabel] = ageGroup
		}
		ageGroup.add(sample)
	}
	if overall.count == 0 {
		t.Fatalf("no filled past_exc_pre_acb_q0 rows in %s", expectedPath)
	}

	t.Logf("decoder TAME pastExc age map: path=%s filled=%d exact=%d mismatch=%d",
		expectedPath, overall.count, overall.exact, overall.count-overall.exact)
	decoderPastExcLogGroup(t, "overall", &overall)

	t.Logf("first mismatches")
	t.Logf("%5s %3s %5s %5s %3s %5s %5s %5s %8s %8s %8s",
		"frame", "sub", "index", "age", "src", "oFr", "oSub", "oIdx", "want", "got", "diff")
	for _, sample := range first {
		t.Logf("%5d %3d %5d %5d %3d %5d %5d %5d %8d %8d %8d",
			sample.currentFrame, sample.currentSub, sample.index, sample.age,
			sample.originGlobalSubframe, sample.originFrame, sample.originSub, sample.originSample,
			sample.want, sample.got, sample.diff)
	}

	decoderPastExcLogGroups(t, "by current subframe", decoderPastExcCurrentGroups(byCurrent), len(byCurrent))
	decoderPastExcLogGroups(t, "by source subframe", decoderPastExcOriginGroups(byOrigin), decoderITUFrontierTopN())
	decoderPastExcLogGroups(t, "by FIFO index band", decoderPastExcNamedGroups(byIndexBand), len(byIndexBand))
	decoderPastExcLogGroups(t, "by sample age band", decoderPastExcNamedGroups(byAgeBand), len(byAgeBand))
}

type decoderPastExcAgeSample struct {
	currentFrame          int
	currentSub            int
	index                 int
	age                   int
	originGlobalSubframe  int
	originFrame           int
	originSub             int
	originSample          int
	want                  int64
	got                   int64
	diff                  int64
}

type decoderPastExcAgeGroup struct {
	label       string
	count       int
	exact       int
	sumWantSq   float64
	sumGotSq    float64
	sumErrSq    float64
	sumAbsDiff  int64
	maxAbsDiff  int64
	maxAbsIndex int
	maxAbsAge   int
	dot         float64
}

func decoderPastExcAgeSampleFromRows(want, got stageRow) decoderPastExcAgeSample {
	currentGlobalSubframe := want.frame*2 + want.sub
	currentSample := currentGlobalSubframe * subframeLen
	age := pastExcLen - want.index
	originSampleAbs := currentSample - age
	originGlobalSubframe := originSampleAbs / subframeLen
	originSample := originSampleAbs % subframeLen
	if originSample < 0 {
		originGlobalSubframe = -1
		originSample = -1
	}
	return decoderPastExcAgeSample{
		currentFrame:         want.frame,
		currentSub:           want.sub,
		index:                want.index,
		age:                  age,
		originGlobalSubframe: originGlobalSubframe,
		originFrame:          originGlobalSubframe / 2,
		originSub:            originGlobalSubframe % 2,
		originSample:         originSample,
		want:                 want.value,
		got:                  got.value,
		diff:                 got.value - want.value,
	}
}

func (g *decoderPastExcAgeGroup) add(sample decoderPastExcAgeSample) {
	want := float64(sample.want)
	got := float64(sample.got)
	diff := sample.diff
	absDiff := absInt64(diff)
	g.count++
	if diff == 0 {
		g.exact++
	}
	g.sumWantSq += want * want
	g.sumGotSq += got * got
	g.sumErrSq += float64(diff * diff)
	g.sumAbsDiff += absDiff
	g.dot += want * got
	if absDiff > g.maxAbsDiff {
		g.maxAbsDiff = absDiff
		g.maxAbsIndex = sample.index
		g.maxAbsAge = sample.age
	}
}

func (g decoderPastExcAgeGroup) wantRMS() float64 {
	if g.count == 0 {
		return 0
	}
	return math.Sqrt(g.sumWantSq / float64(g.count))
}

func (g decoderPastExcAgeGroup) gotRMS() float64 {
	if g.count == 0 {
		return 0
	}
	return math.Sqrt(g.sumGotSq / float64(g.count))
}

func (g decoderPastExcAgeGroup) errRMS() float64 {
	if g.count == 0 {
		return 0
	}
	return math.Sqrt(g.sumErrSq / float64(g.count))
}

func (g decoderPastExcAgeGroup) meanAbs() float64 {
	if g.count == 0 {
		return 0
	}
	return float64(g.sumAbsDiff) / float64(g.count)
}

func (g decoderPastExcAgeGroup) corr() float64 {
	if g.sumWantSq == 0 || g.sumGotSq == 0 {
		return 0
	}
	return g.dot / math.Sqrt(g.sumWantSq*g.sumGotSq)
}

func (g decoderPastExcAgeGroup) scale() float64 {
	if g.sumGotSq == 0 {
		return 0
	}
	return g.dot / g.sumGotSq
}

func decoderPastExcLogGroups(t *testing.T, title string, groups []decoderPastExcAgeGroup, limit int) {
	t.Helper()
	if limit > len(groups) {
		limit = len(groups)
	}
	t.Logf("%s", title)
	t.Logf("%-12s %6s %6s %8s %8s %8s %8s %8s %8s %8s %8s",
		"group", "count", "exact", "wantRMS", "gotRMS", "errRMS", "meanAbs", "maxAbs", "maxIdx", "maxAge", "corr")
	for i := 0; i < limit; i++ {
		decoderPastExcLogGroup(t, groups[i].label, &groups[i])
	}
}

func decoderPastExcLogGroup(t *testing.T, label string, group *decoderPastExcAgeGroup) {
	t.Helper()
	t.Logf("%-12s %6d %6d %8.2f %8.2f %8.2f %8.2f %8d %8d %8d %8.4f scale=%7.4f",
		label,
		group.count,
		group.exact,
		group.wantRMS(),
		group.gotRMS(),
		group.errRMS(),
		group.meanAbs(),
		group.maxAbsDiff,
		group.maxAbsIndex,
		group.maxAbsAge,
		group.corr(),
		group.scale())
}

func decoderPastExcOriginGroups(src map[int]*decoderPastExcAgeGroup) []decoderPastExcAgeGroup {
	groups := make([]decoderPastExcAgeGroup, 0, len(src))
	for _, group := range src {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].errRMS() != groups[j].errRMS() {
			return groups[i].errRMS() > groups[j].errRMS()
		}
		return groups[i].label < groups[j].label
	})
	return groups
}

func decoderPastExcCurrentGroups(src map[decoderFrameSubKey]*decoderPastExcAgeGroup) []decoderPastExcAgeGroup {
	keys := make([]decoderFrameSubKey, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].frame != keys[j].frame {
			return keys[i].frame < keys[j].frame
		}
		return keys[i].sub < keys[j].sub
	})
	groups := make([]decoderPastExcAgeGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, *src[key])
	}
	return groups
}

func decoderPastExcNamedGroups(src map[string]*decoderPastExcAgeGroup) []decoderPastExcAgeGroup {
	groups := make([]decoderPastExcAgeGroup, 0, len(src))
	for _, group := range src {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].label < groups[j].label
	})
	return groups
}

func decoderPastExcOriginLabel(globalSubframe int) string {
	if globalSubframe < 0 {
		return "pre0"
	}
	return decoderPastExcCurrentLabel(decoderFrameSubKey{frame: globalSubframe / 2, sub: globalSubframe % 2})
}

func decoderPastExcCurrentLabel(key decoderFrameSubKey) string {
	return strconvItoa3(key.frame) + "/" + strconvItoa3(key.sub)
}

func decoderPastExcBandLabel(value, width int) string {
	start := (value / width) * width
	end := start + width - 1
	return strconvItoa3(start) + "-" + strconvItoa3(end)
}

func strconvItoa3(v int) string {
	if v >= 0 && v < 10 {
		return "00" + strconv.Itoa(v)
	}
	if v >= 0 && v < 100 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}
