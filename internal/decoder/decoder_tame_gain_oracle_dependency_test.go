package decoder

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/tables"
)

// TestDecoderTAMEGainOracleDependencyAudit groups the external reference
// gain-internals oracle by subframe and reports the first dependency layer that
// diverges. This is diagnostic-only: it turns the flat CSV mismatch list into a
// gain pipeline frontier without relying on implementation source outside this
// repository.
func TestDecoderTAMEGainOracleDependencyAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_GAIN_ORACLE_DEPENDENCY") != "1" {
		t.Skip("set G729_DECODER_TAME_GAIN_ORACLE_DEPENDENCY=1 to run TAME gain oracle dependency audit")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_internals_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain internals expected: %v", err)
	}
	got, err := collectDecoderReferenceTAMEGainInternalRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain internals rows: %v", err)
	}

	wantByKey := make(map[decoderStageKey]stageRow, len(expected))
	keysBySubframe := make(map[decoderFrameSubKey][]decoderStageKey)
	for _, row := range expected {
		key := decoderStageRowKey(row)
		wantByKey[key] = row
		fs := decoderFrameSubKey{frame: key.frame, sub: key.sub}
		keysBySubframe[fs] = append(keysBySubframe[fs], key)
	}

	subframes := make([]decoderFrameSubKey, 0, len(keysBySubframe))
	for key := range keysBySubframe {
		subframes = append(subframes, key)
	}
	sort.Slice(subframes, func(i, j int) bool {
		if subframes[i].frame != subframes[j].frame {
			return subframes[i].frame < subframes[j].frame
		}
		return subframes[i].sub < subframes[j].sub
	})

	order := []decoderTAMEGainDependencyStage{
		{name: "bitstream", fields: []string{"bitstream_ga", "bitstream_gb"}},
		{name: "gamma_vq", fields: []string{"gamma_q13"}},
		{name: "fixed_codebook_energy", fields: []string{"fixed_codebook_energy_q26"}},
		{name: "past_errors_before", fields: []string{"past_errors_before_q10"}},
		{name: "predicted_energy", fields: []string{"predicted_energy_q10"}},
		{name: "ec_bar", fields: []string{"ec_bar_q10"}},
		{name: "log_gain", fields: []string{"log_gain_q10"}},
		{name: "log2_gc", fields: []string{"log2_gc_q10"}},
		{name: "gc0", fields: []string{"gc0_q14"}},
		{name: "fixed_gain", fields: []string{"fixed_gain_q14"}},
		{name: "u_current", fields: []string{"u_current_q10"}},
		{name: "past_errors_after", fields: []string{"past_errors_after_q10"}},
	}

	firstStageCounts := make(map[string]int)
	conditional := map[string]*decoderTAMEGainConditionalStats{
		"energy_exact_ec_bar":        {},
		"past_exact_predicted":       {},
		"pred_ec_exact_log_gain":     {},
		"log_gain_exact_log2":        {},
		"log2_exact_gc0":             {},
		"gamma_log2_exact_gain":      {},
		"u_current_exact_fifo_after": {},
	}
	examples := make([]decoderTAMEGainDependencyExample, 0, 16)

	for _, fs := range subframes {
		first := "exact"
		for _, stage := range order {
			if !decoderTAMEGainFieldsExact(fs, stage.fields, wantByKey, got) {
				first = stage.name
				if len(examples) < cap(examples) {
					examples = append(examples, decoderTAMEGainFirstMismatchExample(fs, stage.name, stage.fields, wantByKey, got))
				}
				break
			}
		}
		firstStageCounts[first]++

		decoderTAMEGainAccumulateConditional(fs, "energy_exact_ec_bar",
			[]string{"fixed_codebook_energy_q26"}, []string{"ec_bar_q10"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "past_exact_predicted",
			[]string{"past_errors_before_q10"}, []string{"predicted_energy_q10"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "pred_ec_exact_log_gain",
			[]string{"predicted_energy_q10", "ec_bar_q10"}, []string{"log_gain_q10"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "log_gain_exact_log2",
			[]string{"log_gain_q10"}, []string{"log2_gc_q10"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "log2_exact_gc0",
			[]string{"log2_gc_q10"}, []string{"gc0_q14"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "gamma_log2_exact_gain",
			[]string{"gamma_q13", "log2_gc_q10"}, []string{"fixed_gain_q14"}, conditional, wantByKey, got)
		decoderTAMEGainAccumulateConditional(fs, "u_current_exact_fifo_after",
			[]string{"u_current_q10"}, []string{"past_errors_after_q10"}, conditional, wantByKey, got)
	}

	t.Logf("decoder TAME gain oracle dependency: subframes=%d path=%s", len(subframes), expectedPath)
	t.Logf("first divergent dependency stage")
	names := make([]string, 0, len(firstStageCounts))
	for name := range firstStageCounts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("  %-24s %d", name, firstStageCounts[name])
	}

	t.Logf("conditional dependency checks")
	condNames := make([]string, 0, len(conditional))
	for name := range conditional {
		condNames = append(condNames, name)
	}
	sort.Strings(condNames)
	for _, name := range condNames {
		st := conditional[name]
		t.Logf("  %-28s base_exact=%d target_exact=%d target_mismatch=%d max_abs=%d first=%s",
			name, st.baseExact, st.targetExact, st.targetMismatch, st.maxAbs, st.first.String())
	}

	t.Logf("first mismatch examples")
	for i, ex := range examples {
		t.Logf("  [%d] frame=%d sub=%d stage=%s field=%s index=%d want=%d got=%d delta=%+d",
			i, ex.frame, ex.sub, ex.stage, ex.field, ex.index, ex.want, ex.got, ex.got-ex.want)
	}
}

func TestDecoderTAMEEcBarFormulaSearch(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ECBAR_FORMULA_SEARCH") != "1" {
		t.Skip("set G729_DECODER_TAME_ECBAR_FORMULA_SEARCH=1 to run TAME EcBar formula search")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_internals_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain internals expected: %v", err)
	}
	got, err := collectDecoderReferenceTAMEGainInternalRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain internals rows: %v", err)
	}

	wantByKey := make(map[decoderStageKey]stageRow, len(expected))
	subframesSeen := make(map[decoderFrameSubKey]struct{})
	for _, row := range expected {
		key := decoderStageRowKey(row)
		wantByKey[key] = row
		subframesSeen[decoderFrameSubKey{frame: key.frame, sub: key.sub}] = struct{}{}
	}

	var samples []decoderTAMEEcBarFormulaSample
	for fs := range subframesSeen {
		energyKey := decoderStageKey{source: "TAME", frame: fs.frame, sub: fs.sub, field: "fixed_codebook_energy_q26", index: 0}
		ecBarKey := decoderStageKey{source: "TAME", frame: fs.frame, sub: fs.sub, field: "ec_bar_q10", index: 0}
		wantEnergy, okEnergy := wantByKey[energyKey]
		wantEcBar, okEcBar := wantByKey[ecBarKey]
		gotEnergy, okGotEnergy := got[energyKey]
		if !okEnergy || !okEcBar || !okGotEnergy || !gotEnergy.hasValue || gotEnergy.value != wantEnergy.value {
			continue
		}
		log2Q10 := int32(gain.Log2Fixed(fixed.Word32(wantEnergy.value))) - 26*1024
		samples = append(samples, decoderTAMEEcBarFormulaSample{
			frame:     fs.frame,
			sub:       fs.sub,
			log2Q10:   log2Q10,
			wantQ10:   wantEcBar.value,
			energyQ26: wantEnergy.value,
		})
	}
	if len(samples) == 0 {
		t.Fatalf("no samples with exact fixed_codebook_energy_q26")
	}

	var results []decoderTAMEEcBarFormulaResult
	roundAdds := []int32{0, 2048, 4095, 4096, 4097, 6144, 8191}
	for k := int32(24640); k <= 24680; k++ {
		for tenLog40 := int32(16390); tenLog40 <= 16420; tenLog40++ {
			for logAdj := int32(-8); logAdj <= 8; logAdj++ {
				for _, roundAdd := range roundAdds {
					var r decoderTAMEEcBarFormulaResult
					r.k = k
					r.tenLog40 = tenLog40
					r.logAdj = logAdj
					r.roundAdd = roundAdd
					r.firstFrame = -1
					for _, sample := range samples {
						ecDb := ((sample.log2Q10+logAdj)*k + roundAdd) >> 13
						gotEcBar := int64(30720 + tenLog40 - ecDb)
						if gotEcBar == sample.wantQ10 {
							r.exact++
							continue
						}
						delta := gotEcBar - sample.wantQ10
						if delta < 0 {
							r.sumAbs -= delta
							if -delta > r.maxAbs {
								r.maxAbs = -delta
							}
						} else {
							r.sumAbs += delta
							if delta > r.maxAbs {
								r.maxAbs = delta
							}
						}
						if r.firstFrame < 0 {
							r.firstFrame = sample.frame
							r.firstSub = sample.sub
							r.firstWant = sample.wantQ10
							r.firstGot = gotEcBar
						}
					}
					results = append(results, r)
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].exact != results[j].exact {
			return results[i].exact > results[j].exact
		}
		if results[i].sumAbs != results[j].sumAbs {
			return results[i].sumAbs < results[j].sumAbs
		}
		if results[i].maxAbs != results[j].maxAbs {
			return results[i].maxAbs < results[j].maxAbs
		}
		return results[i].k < results[j].k
	})

	t.Logf("decoder TAME EcBar formula search: samples=%d path=%s", len(samples), expectedPath)
	topN := 12
	if len(results) < topN {
		topN = len(results)
	}
	for i := 0; i < topN; i++ {
		r := results[i]
		t.Logf("[%d] exact=%d/%d sumAbs=%d maxAbs=%d k=%d tenLog40=%d logAdj=%d roundAdd=%d first=%d/%d want=%d got=%d",
			i, r.exact, len(samples), r.sumAbs, r.maxAbs, r.k, r.tenLog40, r.logAdj, r.roundAdd,
			r.firstFrame, r.firstSub, r.firstWant, r.firstGot)
	}
}

func TestDecoderTAMELog2VariantSearch(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_LOG2_VARIANT_SEARCH") != "1" {
		t.Skip("set G729_DECODER_TAME_LOG2_VARIANT_SEARCH=1 to run TAME log2 variant search")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_internals_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain internals expected: %v", err)
	}
	got, err := collectDecoderReferenceTAMEGainInternalRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain internals rows: %v", err)
	}

	wantByKey := make(map[decoderStageKey]stageRow, len(expected))
	subframesSeen := make(map[decoderFrameSubKey]struct{})
	for _, row := range expected {
		key := decoderStageRowKey(row)
		wantByKey[key] = row
		subframesSeen[decoderFrameSubKey{frame: key.frame, sub: key.sub}] = struct{}{}
	}

	var samples []decoderTAMEEcBarFormulaSample
	for fs := range subframesSeen {
		energyKey := decoderStageKey{source: "TAME", frame: fs.frame, sub: fs.sub, field: "fixed_codebook_energy_q26", index: 0}
		ecBarKey := decoderStageKey{source: "TAME", frame: fs.frame, sub: fs.sub, field: "ec_bar_q10", index: 0}
		wantEnergy, okEnergy := wantByKey[energyKey]
		wantEcBar, okEcBar := wantByKey[ecBarKey]
		gotEnergy, okGotEnergy := got[energyKey]
		if !okEnergy || !okEcBar || !okGotEnergy || !gotEnergy.hasValue || gotEnergy.value != wantEnergy.value {
			continue
		}
		samples = append(samples, decoderTAMEEcBarFormulaSample{
			frame:     fs.frame,
			sub:       fs.sub,
			wantQ10:   wantEcBar.value,
			energyQ26: wantEnergy.value,
		})
	}

	var results []decoderTAMELog2VariantResult
	for _, aShift := range []int{9, 10, 11} {
		for _, interpRound := range []bool{false, true} {
			for _, fracRound := range []bool{false, true} {
				for _, outRound := range []bool{false, true} {
					var r decoderTAMELog2VariantResult
					r.aShift = aShift
					r.interpRound = interpRound
					r.fracRound = fracRound
					r.outRound = outRound
					r.firstFrame = -1
					for _, sample := range samples {
						log2Q10 := decoderTAMELog2Variant(fixed.Word32(sample.energyQ26), aShift, interpRound, fracRound) - 26*1024
						prod := log2Q10 * 24660
						if outRound {
							prod += 1 << 12
						}
						ecDb := prod >> 13
						gotEcBar := int64(30720 + 16405 - ecDb)
						if gotEcBar == sample.wantQ10 {
							r.exact++
							continue
						}
						delta := gotEcBar - sample.wantQ10
						if delta < 0 {
							r.sumAbs -= delta
							if -delta > r.maxAbs {
								r.maxAbs = -delta
							}
						} else {
							r.sumAbs += delta
							if delta > r.maxAbs {
								r.maxAbs = delta
							}
						}
						if r.firstFrame < 0 {
							r.firstFrame = sample.frame
							r.firstSub = sample.sub
							r.firstWant = sample.wantQ10
							r.firstGot = gotEcBar
						}
					}
					results = append(results, r)
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].exact != results[j].exact {
			return results[i].exact > results[j].exact
		}
		if results[i].sumAbs != results[j].sumAbs {
			return results[i].sumAbs < results[j].sumAbs
		}
		return results[i].maxAbs < results[j].maxAbs
	})

	t.Logf("decoder TAME log2 variant search: samples=%d path=%s", len(samples), expectedPath)
	topN := 12
	if len(results) < topN {
		topN = len(results)
	}
	for i := 0; i < topN; i++ {
		r := results[i]
		t.Logf("[%d] exact=%d/%d sumAbs=%d maxAbs=%d aShift=%d interpRound=%v fracRound=%v outRound=%v first=%d/%d want=%d got=%d",
			i, r.exact, len(samples), r.sumAbs, r.maxAbs, r.aShift, r.interpRound, r.fracRound, r.outRound,
			r.firstFrame, r.firstSub, r.firstWant, r.firstGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEGainLog2Micro(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO=1 to compare external reference TAME gain-log2 micro oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_log2_micro_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain-log2 micro expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME gain-log2 micro expected is empty")
	}

	got, err := collectDecoderReferenceTAMEGainLog2MicroRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain-log2 micro rows: %v", err)
	}

	fieldStats := make(map[string]*decoderReferenceStageFieldStats)
	first := make([]decoderStageMismatch, 0, 16)
	var exact, missingGot, mismatches int
	for _, want := range expected {
		key := decoderStageRowKey(want)
		st := decoderReferenceStageStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := got[key]
		if !ok {
			missingGot++
			mismatches++
			st.mismatches++
			st.missing++
			appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), "", "missing got")
			continue
		}
		if gotRow.hasValue && gotRow.value == want.value {
			exact++
			st.exact++
			continue
		}

		mismatches++
		st.mismatches++
		delta := absInt64(want.value - gotRow.value)
		if delta > st.maxAbs {
			st.maxAbs = delta
		}
		appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), decoderStageValueString(gotRow), "mismatch")
	}

	t.Logf("decoder_reference_tame_gain_log2_micro: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME gain-log2 micro exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

type decoderTAMEGainDependencyStage struct {
	name   string
	fields []string
}

type decoderTAMEGainConditionalStats struct {
	baseExact      int
	targetExact    int
	targetMismatch int
	maxAbs         int64
	first          decoderTAMEGainConditionalFirst
}

type decoderTAMEGainConditionalFirst struct {
	set   bool
	frame int
	sub   int
	field string
	index int
	want  int64
	got   int64
}

func (f decoderTAMEGainConditionalFirst) String() string {
	if !f.set {
		return "-"
	}
	return strconv.Itoa(f.frame) + "/" + strconv.Itoa(f.sub) + " " + f.field + "[" + strconv.Itoa(f.index) + "] want=" + strconv.FormatInt(f.want, 10) + " got=" + strconv.FormatInt(f.got, 10)
}

type decoderTAMEGainDependencyExample struct {
	frame int
	sub   int
	stage string
	field string
	index int
	want  int64
	got   int64
}

type decoderTAMEEcBarFormulaSample struct {
	frame     int
	sub       int
	log2Q10   int32
	wantQ10   int64
	energyQ26 int64
}

type decoderTAMEEcBarFormulaResult struct {
	exact      int
	sumAbs     int64
	maxAbs     int64
	k          int32
	tenLog40   int32
	logAdj     int32
	roundAdd   int32
	firstFrame int
	firstSub   int
	firstWant  int64
	firstGot   int64
}

type decoderTAMELog2VariantResult struct {
	exact       int
	sumAbs      int64
	maxAbs      int64
	aShift      int
	interpRound bool
	fracRound   bool
	outRound    bool
	firstFrame  int
	firstSub    int
	firstWant   int64
	firstGot    int64
}

func decoderTAMELog2Variant(x fixed.Word32, aShift int, interpRound, fracRound bool) int32 {
	if x <= 0 {
		return 0
	}
	s := fixed.NormL(x)
	normX := fixed.LShl(x, s)
	intPart := fixed.Word32(30) - fixed.Word32(s)
	frac30 := int64(normX) - (1 << 30)
	idx := fixed.Word32(frac30 >> 25)
	a := fixed.Word32((frac30 >> aShift) & 0x7FFF)
	t0 := fixed.Word32(tables.Log2Table[idx])
	t1 := fixed.Word32(tables.Log2Table[idx+1])
	interp := (t1 - t0) * a
	if interpRound {
		interp += 1 << 14
	}
	fracLog2Q15 := t0 + (interp >> 15)
	if fracRound {
		return int32((intPart << 10) + ((fracLog2Q15 + 16) >> 5))
	}
	return int32((intPart << 10) + (fracLog2Q15 >> 5))
}

func collectDecoderReferenceTAMEGainLog2MicroRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targetFrames := make(map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		targetFrames[row.frame] = struct{}{}
	}
	maxFrame := maxIntKey(targetFrames)

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		return nil, fmt.Errorf("unknown ITU decoder vector source TAME")
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTaps(frames[frame])
		if err != nil {
			return nil, err
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEGainLog2MicroRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func appendDecoderReferenceTAMEGainLog2MicroRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		g := st.GainTaps

		ecEnergy := decoderTAMELocalFixedCodebookEnergyWord32(st.C[:])
		appendDecoderReferenceScalar(rows, frame, sub, "ec_energy_q26", int64(ecEnergy))
		ecLog2Raw := appendDecoderTAMELog2Micro(rows, frame, sub, "ec", ecEnergy)
		ecLog2Corrected := ecLog2Raw - 26*1024
		ecDbQ10 := (ecLog2Corrected*24660 + (1 << 12)) >> 13
		ecBarDbQ10 := ecDbQ10 - 16405
		appendDecoderReferenceScalar(rows, frame, sub, "ec_log2_corrected_q10", int64(ecLog2Corrected))
		appendDecoderReferenceScalar(rows, frame, sub, "ec_db_q10", int64(ecDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "ec_bar_db_q10", int64(ecBarDbQ10))

		gamma := fixed.Word32(g.GammaCQ13)
		appendDecoderReferenceScalar(rows, frame, sub, "gamma_q13", int64(gamma))
		gammaLog2Raw := appendDecoderTAMELog2Micro(rows, frame, sub, "gamma", gamma)
		gammaLog2Corrected := gammaLog2Raw - 13*1024
		uCurrent := (gammaLog2Corrected*6165 + (1 << 9)) >> 10
		if uCurrent > 32767 {
			uCurrent = 32767
		} else if uCurrent < -32768 {
			uCurrent = -32768
		}
		appendDecoderReferenceScalar(rows, frame, sub, "gamma_log2_corrected_q10", int64(gammaLog2Corrected))
		appendDecoderReferenceScalar(rows, frame, sub, "u_current_q10", int64(uCurrent))

		appendDecoderReferenceScalar(rows, frame, sub, "predicted_q10", int64(g.Predicted))
		appendDecoderReferenceScalar(rows, frame, sub, "log_gain_q10", int64(g.LogGainDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "log2_gc_q10", int64(g.Log2GcQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "gc0_q14", int64(g.Gc0MantQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_q14", gainQ14FromMantExp(g.GcMantQ14, g.GcExp))
	}
}

func appendDecoderTAMELog2Micro(rows *[]stageRow, frame, sub int, prefix string, x fixed.Word32) int32 {
	if x <= 0 {
		appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_input_q0", int64(x))
		appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_raw_q10", 0)
		return 0
	}

	shift := fixed.NormL(x)
	normX := fixed.LShl(x, shift)
	intPart := fixed.Word32(30) - fixed.Word32(shift)
	frac30 := int64(normX) - (1 << 30)
	index := fixed.Word32(frac30 >> 25)
	fraction := fixed.Word32((frac30 >> 10) & 0x7FFF)
	table0 := fixed.Word32(tables.Log2Table[index])
	table1 := fixed.Word32(tables.Log2Table[index+1])
	interpProduct := (table1 - table0) * fraction
	fracQ15 := table0 + (interpProduct >> 15)
	rawQ10 := (intPart << 10) + (fracQ15 >> 5)

	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_input_q0", int64(x))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_norm_shift_q0", int64(shift))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_norm_x_q0", int64(normX))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_int_part_q0", int64(intPart))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_frac30_q0", frac30)
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_table_index_q0", int64(index))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_fraction_q0", int64(fraction))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_table0_q15", int64(table0))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_table1_q15", int64(table1))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_interp_product_q30", int64(interpProduct))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_frac_q15", int64(fracQ15))
	appendDecoderReferenceScalar(rows, frame, sub, prefix+"_log2_raw_q10", int64(rawQ10))

	return int32(rawQ10)
}

func decoderTAMELocalFixedCodebookEnergyWord32(c []int16) fixed.Word32 {
	var acc fixed.Word32
	for _, sample := range c {
		acc = fixed.LAdd(acc, fixed.LShr(fixed.LMult(fixed.Word16(sample), fixed.Word16(sample)), 1))
	}
	return acc
}

func decoderTAMEGainFieldsExact(fs decoderFrameSubKey, fields []string, want, got map[decoderStageKey]stageRow) bool {
	for _, field := range fields {
		if !decoderTAMEGainFieldExact(fs, field, want, got) {
			return false
		}
	}
	return true
}

func decoderTAMEGainFieldExact(fs decoderFrameSubKey, field string, want, got map[decoderStageKey]stageRow) bool {
	found := false
	for key, wantRow := range want {
		if key.frame != fs.frame || key.sub != fs.sub || key.field != field {
			continue
		}
		found = true
		gotRow, ok := got[key]
		if !ok || !gotRow.hasValue || gotRow.value != wantRow.value {
			return false
		}
	}
	return found
}

func decoderTAMEGainFirstMismatchExample(fs decoderFrameSubKey, stage string, fields []string, want, got map[decoderStageKey]stageRow) decoderTAMEGainDependencyExample {
	var keys []decoderStageKey
	for _, field := range fields {
		for key := range want {
			if key.frame == fs.frame && key.sub == fs.sub && key.field == field {
				keys = append(keys, key)
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].field != keys[j].field {
			return keys[i].field < keys[j].field
		}
		return keys[i].index < keys[j].index
	})
	for _, key := range keys {
		wantRow := want[key]
		gotRow := got[key]
		if !gotRow.hasValue || gotRow.value != wantRow.value {
			return decoderTAMEGainDependencyExample{
				frame: key.frame,
				sub:   key.sub,
				stage: stage,
				field: key.field,
				index: key.index,
				want:  wantRow.value,
				got:   gotRow.value,
			}
		}
	}
	return decoderTAMEGainDependencyExample{frame: fs.frame, sub: fs.sub, stage: stage}
}

func decoderTAMEGainAccumulateConditional(
	fs decoderFrameSubKey,
	name string,
	baseFields, targetFields []string,
	stats map[string]*decoderTAMEGainConditionalStats,
	want, got map[decoderStageKey]stageRow,
) {
	if !decoderTAMEGainFieldsExact(fs, baseFields, want, got) {
		return
	}
	st := stats[name]
	st.baseExact++
	if decoderTAMEGainFieldsExact(fs, targetFields, want, got) {
		st.targetExact++
		return
	}
	for _, field := range targetFields {
		for key, wantRow := range want {
			if key.frame != fs.frame || key.sub != fs.sub || key.field != field {
				continue
			}
			gotRow := got[key]
			if gotRow.hasValue && gotRow.value == wantRow.value {
				continue
			}
			st.targetMismatch++
			delta := absInt64(wantRow.value - gotRow.value)
			if delta > st.maxAbs {
				st.maxAbs = delta
			}
			if !st.first.set {
				st.first = decoderTAMEGainConditionalFirst{
					set:   true,
					frame: key.frame,
					sub:   key.sub,
					field: key.field,
					index: key.index,
					want:  wantRow.value,
					got:   gotRow.value,
				}
			}
		}
	}
}
