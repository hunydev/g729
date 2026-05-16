package decoder

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

func TestOracleHandoff_CompareDecoderReferenceFinalPCM(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_FINAL_PCM") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 to compare external reference final PCM oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_final_pcm_expected.csv")
	expected, err := readDecoderReferenceFinalPCMExpected(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference final PCM expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference final PCM expected is empty")
	}

	requireExact := os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM") == "1"
	sources := make([]string, 0, len(expected))
	for source := range expected {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var total decoderReferencePCMStats
	for _, source := range sources {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			t.Fatalf("unknown decoder reference vector source %q", source)
		}
		got := decodeDecoderReferenceVectorPCM(t, tc)
		stats := compareDecoderReferencePCM(source, got, expected[source])
		total.add(stats)
		t.Logf("%-10s exact=%d/%d %.2f%% mismatches=%d max_abs=%d mean_abs=%.2f first=%s extra_got=%d",
			source, stats.exact, stats.total, percent(stats.exact, stats.total),
			stats.mismatches, stats.maxAbs, stats.meanAbsDelta(), stats.firstDiffString(), stats.extraGot)
		if requireExact && stats.mismatches != 0 {
			t.Errorf("%s final PCM mismatch: exact=%d/%d first=%s got=%d want=%d max_abs=%d",
				source, stats.exact, stats.total, stats.firstDiffString(),
				stats.firstGot, stats.firstWant, stats.maxAbs)
		}
	}

	t.Logf("%-10s exact=%d/%d %.2f%% mismatches=%d max_abs=%d mean_abs=%.2f first=%s extra_got=%d",
		"TOTAL", total.exact, total.total, percent(total.exact, total.total),
		total.mismatches, total.maxAbs, total.meanAbsDelta(), total.firstDiffString(), total.extraGot)
	if requireExact && total.mismatches != 0 {
		t.Fatalf("decoder reference final PCM exact gate failed: mismatches=%d/%d",
			total.mismatches, total.total)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEFullStage(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_STAGE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_STAGE=1 to compare external reference TAME full-stage oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_full_stage_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME stage expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME stage expected is empty")
	}

	got, err := collectDecoderReferenceTAMEStageRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME stage rows: %v", err)
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

	t.Logf("decoder_reference_tame_stage: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_STAGE") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME stage exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceFullStage(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_FULL_STAGE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_FULL_STAGE=1 to compare external reference full-stage oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_FULL_STAGE_FILE"))
	if file == "" {
		file = "decoder_full_stage_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference full-stage expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference full-stage expected is empty")
	}

	got, err := collectDecoderReferenceFullStageRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference full-stage rows: %v", err)
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

	t.Logf("decoder_reference_full_stage file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_FULL_STAGE") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference full-stage exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_ProbeDecoderReferenceFullStageFirstMismatchByField(t *testing.T) {
	if os.Getenv("G729_PROBE_DECODER_REFERENCE_FULL_STAGE_FIELDS") != "1" {
		t.Skip("set G729_PROBE_DECODER_REFERENCE_FULL_STAGE_FIELDS=1 to print first full-stage mismatch per field")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_FULL_STAGE_FILE"))
	if file == "" {
		file = "decoder_full_stage_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference full-stage expected: %v", err)
	}
	got, err := collectDecoderReferenceFullStageRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference full-stage rows: %v", err)
	}

	t.Logf("decoder_reference_full_stage first mismatch by field for %s", file)
	logFirstDecoderReferenceMismatchByField(t, expected, got)
}

func TestOracleHandoff_ProbeDecoderReferenceTAMEStageFirstMismatchByField(t *testing.T) {
	if os.Getenv("G729_PROBE_DECODER_REFERENCE_TAME_STAGE_FIELDS") != "1" {
		t.Skip("set G729_PROBE_DECODER_REFERENCE_TAME_STAGE_FIELDS=1 to print first TAME stage mismatch per field")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_full_stage_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME stage expected: %v", err)
	}
	got, err := collectDecoderReferenceTAMEStageRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME stage rows: %v", err)
	}

	logFirstDecoderReferenceMismatchByField(t, expected, got)
}

func TestOracleHandoff_CompareDecoderReferenceTAMEGainInternals(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_INTERNALS") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_INTERNALS=1 to compare external reference TAME gain-internals oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_internals_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain internals expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME gain internals expected is empty")
	}

	got, err := collectDecoderReferenceTAMEGainInternalRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain internals rows: %v", err)
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

	t.Logf("decoder_reference_tame_gain_internals: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_GAIN_INTERNALS") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME gain internals exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEPostfilterMicro(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_POSTFILTER_MICRO") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_POSTFILTER_MICRO=1 to compare external reference TAME postfilter micro oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_postfilter_micro_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME postfilter micro expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME postfilter micro expected is empty")
	}

	got, err := collectDecoderReferenceTAMEPostfilterMicroRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME postfilter micro rows: %v", err)
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

	t.Logf("decoder_reference_tame_postfilter_micro: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_POSTFILTER_MICRO") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME postfilter micro exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferencePostfilterMicro(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_POSTFILTER_MICRO") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_POSTFILTER_MICRO=1 to compare external reference postfilter micro oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_POSTFILTER_MICRO_FILE"))
	if file == "" {
		file = "decoder_postfilter_micro_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference postfilter micro expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference postfilter micro expected is empty")
	}

	got, err := collectDecoderReferencePostfilterMicroRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference postfilter micro rows: %v", err)
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

	t.Logf("decoder_reference_postfilter_micro file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_POSTFILTER_MICRO") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference postfilter micro exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_ProbeDecoderReferencePostfilterMicroFirstMismatchByField(t *testing.T) {
	if os.Getenv("G729_PROBE_DECODER_REFERENCE_POSTFILTER_MICRO_FIELDS") != "1" {
		t.Skip("set G729_PROBE_DECODER_REFERENCE_POSTFILTER_MICRO_FIELDS=1 to print first postfilter micro mismatch per field")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_POSTFILTER_MICRO_FILE"))
	if file == "" {
		file = "decoder_postfilter_micro_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference postfilter micro expected: %v", err)
	}
	got, err := collectDecoderReferencePostfilterMicroRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference postfilter micro rows: %v", err)
	}

	t.Logf("decoder_reference_postfilter_micro first mismatch by field for %s", file)
	logFirstDecoderReferenceMismatchByField(t, expected, got)
}

func TestOracleHandoff_ProbeDecoderReferenceTAMEPostfilterMicroFirstMismatchByField(t *testing.T) {
	if os.Getenv("G729_PROBE_DECODER_REFERENCE_TAME_POSTFILTER_MICRO_FIELDS") != "1" {
		t.Skip("set G729_PROBE_DECODER_REFERENCE_TAME_POSTFILTER_MICRO_FIELDS=1 to print first TAME postfilter micro mismatch per field")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_postfilter_micro_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME postfilter micro expected: %v", err)
	}
	got, err := collectDecoderReferenceTAMEPostfilterMicroRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME postfilter micro rows: %v", err)
	}
	logFirstDecoderReferenceMismatchByField(t, expected, got)
}

func TestOracleHandoff_CompareDecoderReferenceTAMEPostfilterAGCArith(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_POSTFILTER_AGC_ARITH") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_POSTFILTER_AGC_ARITH=1 to compare external reference TAME postfilter AGC arithmetic oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_postfilter_agc_arith_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME postfilter AGC arithmetic expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME postfilter AGC arithmetic expected is empty")
	}

	got, err := collectDecoderReferenceTAMEPostfilterAGCArithRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME postfilter AGC arithmetic rows: %v", err)
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

	t.Logf("decoder_reference_tame_postfilter_agc_arith: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_POSTFILTER_AGC_ARITH") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME postfilter AGC arithmetic exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferencePostfilterAGCArith(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_POSTFILTER_AGC_ARITH") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_POSTFILTER_AGC_ARITH=1 to compare external reference postfilter AGC arithmetic oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_POSTFILTER_AGC_ARITH_FILE"))
	if file == "" {
		file = "decoder_postfilter_agc_arith_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference postfilter AGC arithmetic expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference postfilter AGC arithmetic expected is empty")
	}

	got, err := collectDecoderReferencePostfilterAGCArithRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference postfilter AGC arithmetic rows: %v", err)
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

	t.Logf("decoder_reference_postfilter_agc_arith file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_POSTFILTER_AGC_ARITH") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference postfilter AGC arithmetic exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceAGCTargetDecision(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_AGC_TARGET_DECISION") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_AGC_TARGET_DECISION=1 to compare external reference AGC target-decision oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_AGC_TARGET_DECISION_FILE"))
	if file == "" {
		file = "decoder_agc_target_decision_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference AGC target-decision expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference AGC target-decision expected is empty")
	}

	got, err := collectDecoderReferenceAGCTargetDecisionRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference AGC target-decision rows: %v", err)
	}

	fieldStats := make(map[string]*decoderReferenceStageFieldStats)
	first := make([]decoderStageMismatch, 0, 64)
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

	t.Logf("decoder_reference_agc_target_decision file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_AGC_TARGET_DECISION") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference AGC target-decision exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEOutputHPArith(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_OUTPUT_HP_ARITH") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_OUTPUT_HP_ARITH=1 to compare external reference TAME output HP arithmetic oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_output_hp_arith_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME output HP arithmetic expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME output HP arithmetic expected is empty")
	}

	got, err := collectDecoderReferenceTAMEOutputHPArithRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME output HP arithmetic rows: %v", err)
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

	t.Logf("decoder_reference_tame_output_hp_arith: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_OUTPUT_HP_ARITH") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME output HP arithmetic exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceOutputHPArith(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_OUTPUT_HP_ARITH") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_OUTPUT_HP_ARITH=1 to compare external reference output HP arithmetic oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_OUTPUT_HP_ARITH_FILE"))
	if file == "" {
		file = "decoder_output_hp_arith_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference output HP arithmetic expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference output HP arithmetic expected is empty")
	}

	got, err := collectDecoderReferenceOutputHPArithRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference output HP arithmetic rows: %v", err)
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

	t.Logf("decoder_reference_output_hp_arith file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_OUTPUT_HP_ARITH") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference output HP arithmetic exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceRecoveryState(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_RECOVERY_STATE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_RECOVERY_STATE=1 to compare external reference recovery-state oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_RECOVERY_STATE_FILE"))
	if file == "" {
		file = "decoder_erasure_recovery_state_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference recovery-state expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference recovery-state expected is empty")
	}

	got, err := collectDecoderReferenceRecoveryStateRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference recovery-state rows: %v", err)
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

	t.Logf("decoder_reference_recovery_state file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}
	if os.Getenv("G729_PROBE_DECODER_REFERENCE_RECOVERY_STATE_FIELDS") == "1" {
		logFirstDecoderReferenceMismatchByField(t, expected, got)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_RECOVERY_STATE") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference recovery-state exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceLSPPredictorState(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_LSP_PREDICTOR_STATE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_LSP_PREDICTOR_STATE=1 to compare external reference LSP predictor-state oracle")
	}

	file := strings.TrimSpace(os.Getenv("G729_DECODER_REFERENCE_LSP_PREDICTOR_STATE_FILE"))
	if file == "" {
		file = "decoder_erasure_overflow_lsp_predictor_state_expected.csv"
	}
	expectedPath := decoderReferenceOraclePath(file)
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference LSP predictor-state expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference LSP predictor-state expected is empty")
	}

	got, err := collectDecoderReferenceLSPPredictorStateRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference LSP predictor-state rows: %v", err)
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

	t.Logf("decoder_reference_lsp_predictor_state file=%s: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		file, exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_LSP_PREDICTOR_STATE") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference LSP predictor-state exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func decoderReferenceOraclePath(name string) string {
	dir := os.Getenv("G729_DECODER_REFERENCE_ORACLE_DIR")
	return filepath.Join(dir, name)
}

type decoderReferencePCMBuild struct {
	samples []int16
	set     []bool
}

func readDecoderReferenceFinalPCMExpected(path string) (map[string][]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) != 5 ||
		header[0] != "source" ||
		header[1] != "frame" ||
		header[2] != "index" ||
		header[3] != "expected" ||
		header[4] != "note" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	builds := make(map[string]*decoderReferencePCMBuild)
	line := 1
	for {
		rec, err := r.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != 5 {
			return nil, fmt.Errorf("line %d: got %d columns, want 5", line, len(rec))
		}
		frame, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d frame: %w", line, err)
		}
		index, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("line %d index: %w", line, err)
		}
		if frame < 0 || index < 0 || index >= frameSamples {
			return nil, fmt.Errorf("line %d invalid frame/index %d/%d", line, frame, index)
		}
		value64, err := strconv.ParseInt(rec[3], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("line %d expected: %w", line, err)
		}
		if rec[4] == "" {
			return nil, fmt.Errorf("line %d missing note", line)
		}

		pos := frame*frameSamples + index
		build := builds[rec[0]]
		if build == nil {
			build = &decoderReferencePCMBuild{}
			builds[rec[0]] = build
		}
		if pos >= len(build.samples) {
			newLen := pos + 1
			build.samples = append(build.samples, make([]int16, newLen-len(build.samples))...)
			build.set = append(build.set, make([]bool, newLen-len(build.set))...)
		}
		if build.set[pos] {
			return nil, fmt.Errorf("line %d duplicate sample source=%s frame=%d index=%d", line, rec[0], frame, index)
		}
		build.samples[pos] = int16(value64)
		build.set[pos] = true
	}

	out := make(map[string][]int16, len(builds))
	for source, build := range builds {
		for i, ok := range build.set {
			if !ok {
				return nil, fmt.Errorf("%s missing sample at frame=%d index=%d", source, i/frameSamples, i%frameSamples)
			}
		}
		out[source] = build.samples
	}
	return out, nil
}

func readDecoderReferenceStageRows(path string) ([]stageRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) != 7 ||
		header[0] != "source" ||
		header[1] != "frame" ||
		header[2] != "sub" ||
		header[3] != "field" ||
		header[4] != "index" ||
		header[5] != "expected" ||
		header[6] != "note" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	var rows []stageRow
	line := 1
	for {
		rec, err := r.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != 7 {
			return nil, fmt.Errorf("line %d: got %d columns, want 7", line, len(rec))
		}
		frame, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d frame: %w", line, err)
		}
		sub, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("line %d sub: %w", line, err)
		}
		index, err := strconv.Atoi(rec[4])
		if err != nil {
			return nil, fmt.Errorf("line %d index: %w", line, err)
		}
		value, err := strconv.ParseInt(rec[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d expected: %w", line, err)
		}
		if rec[6] == "" {
			return nil, fmt.Errorf("line %d missing note", line)
		}
		rows = append(rows, stageRow{
			source:   rec[0],
			frame:    frame,
			sub:      sub,
			field:    rec[3],
			index:    index,
			hasValue: true,
			value:    value,
		})
	}
	return rows, nil
}

func decodeDecoderReferenceVectorPCM(t testing.TB, tc decoderITUValidationCase) []int16 {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	ensureTestdataPresent(t, bitPath)

	frames, bads := readG192Frames(t, bitPath)
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch: bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	out := make([]int16, len(frames)*frameSamples)
	var dec Decoder
	for frame, packed := range frames {
		if err := dec.Decode(packed, bads[frame], out[frame*frameSamples:(frame+1)*frameSamples]); err != nil {
			t.Fatalf("%s frame %d Decode: %v", tc.name, frame, err)
		}
	}
	return out
}

type decoderReferencePCMStats struct {
	total       int
	exact       int
	mismatches  int
	extraGot    int
	firstGlobal int
	firstGot    int16
	firstWant   int16
	maxAbs      int
	sumAbs      int64
}

func compareDecoderReferencePCM(source string, got, want []int16) decoderReferencePCMStats {
	stats := decoderReferencePCMStats{firstGlobal: -1}
	n := len(want)
	stats.total = n
	limit := n
	if len(got) < limit {
		limit = len(got)
	}
	for i := 0; i < limit; i++ {
		if got[i] == want[i] {
			stats.exact++
			continue
		}
		stats.recordMismatch(i, got[i], want[i])
	}
	if len(got) < n {
		for i := len(got); i < n; i++ {
			stats.recordMismatch(i, 0, want[i])
		}
	}
	if len(got) > n {
		stats.extraGot = len(got) - n
		stats.mismatches += stats.extraGot
		if stats.firstGlobal < 0 {
			stats.firstGlobal = n
		}
	}
	_ = source
	return stats
}

func (s *decoderReferencePCMStats) recordMismatch(global int, got, want int16) {
	s.mismatches++
	if s.firstGlobal < 0 {
		s.firstGlobal = global
		s.firstGot = got
		s.firstWant = want
	}
	delta := int(got) - int(want)
	if delta < 0 {
		delta = -delta
	}
	if delta > s.maxAbs {
		s.maxAbs = delta
	}
	s.sumAbs += int64(delta)
}

func (s *decoderReferencePCMStats) add(other decoderReferencePCMStats) {
	offset := s.total
	s.total += other.total
	s.exact += other.exact
	s.mismatches += other.mismatches
	s.extraGot += other.extraGot
	s.sumAbs += other.sumAbs
	if other.maxAbs > s.maxAbs {
		s.maxAbs = other.maxAbs
	}
	if s.firstGlobal < 0 && other.firstGlobal >= 0 {
		s.firstGlobal = offset + other.firstGlobal
		s.firstGot = other.firstGot
		s.firstWant = other.firstWant
	}
}

func (s decoderReferencePCMStats) meanAbsDelta() float64 {
	if s.mismatches == 0 {
		return 0
	}
	return float64(s.sumAbs) / float64(s.mismatches)
}

func (s decoderReferencePCMStats) firstDiffString() string {
	if s.firstGlobal < 0 {
		return "-"
	}
	return strconv.Itoa(s.firstGlobal/frameSamples) + ":" + strconv.Itoa(s.firstGlobal%frameSamples)
}

func collectDecoderReferenceTAMEStageRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
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
	frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEStageRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceFullStageRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceTAMEStageRows(&rows, frame, &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceTAMEGainInternalRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
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
	frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEGainInternalRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceTAMEPostfilterMicroRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
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
	frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEPostfilterMicroRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferencePostfilterMicroRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		bitPath := vectorPath(tc.bitFile)
		frames, bads := readG192Frames(t, bitPath)
		zeroSoftbits := readG192ZeroSoftbitCounts(t, bitPath)
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceTAMEPostfilterMicroRows(&rows, frame, &taps)
			appendDecoderReferenceG192FrameModeRows(&rows, frame, bads[frame], zeroSoftbits[frame])
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceTAMEPostfilterAGCArithRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
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
	frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEPostfilterAGCArithRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferencePostfilterAGCArithRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceTAMEPostfilterAGCArithRows(&rows, frame, &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceAGCTargetDecisionRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceAGCTargetDecisionRows(&rows, frame, &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceTAMEOutputHPArithRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
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
	frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEOutputHPArithRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceOutputHPArithRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		frames, bads := readG192Frames(t, vectorPath(tc.bitFile))
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceTAMEOutputHPArithRows(&rows, frame, &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceRecoveryStateRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		bitPath := vectorPath(tc.bitFile)
		frames, bads := readG192Frames(t, bitPath)
		zeroSoftbits := readG192ZeroSoftbitCounts(t, bitPath)
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}
		if len(zeroSoftbits) != len(frames) {
			return nil, fmt.Errorf("%s zero-softbit count mismatch: zero=%d frames=%d", source, len(zeroSoftbits), len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceRecoveryStateRows(&rows, frame, bads[frame], zeroSoftbits[frame], &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceLSPPredictorStateRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if row.source == "" {
			return nil, fmt.Errorf("empty source in expected row")
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	rows := make([]stageRow, 0, len(expected))
	for source, targetFrames := range targets {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown decoder reference vector source %q", source)
		}
		maxFrame := maxIntKey(targetFrames)
		bitPath := vectorPath(tc.bitFile)
		frames, bads := readG192Frames(t, bitPath)
		zeroSoftbits := readG192ZeroSoftbitCounts(t, bitPath)
		if maxFrame >= len(frames) {
			return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, maxFrame, len(frames))
		}

		var dec Decoder
		for frame := 0; frame <= maxFrame; frame++ {
			taps, err := dec.DecodeWithTapsBad(frames[frame], bads[frame])
			if err != nil {
				return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", source, frame, err)
			}
			if _, ok := targetFrames[frame]; !ok {
				continue
			}
			start := len(rows)
			appendDecoderReferenceLSPPredictorStateRows(&rows, frame, bads[frame], zeroSoftbits[frame], &taps)
			for i := start; i < len(rows); i++ {
				rows[i].source = source
			}
		}
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func appendDecoderReferenceTAMEStageRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	appendDecoderReferenceFrameArray(rows, frame, "pcm_q0", taps.Output[:])
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_int", int64(st.TInt))
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_frac", int64(st.TFrac))
		appendDecoderReferenceScalar(rows, frame, sub, "adaptive_gain_q14", int64(st.GpQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_q14", gainQ14FromMantExp(st.GainTaps.GcMantQ14, st.GainTaps.GcExp))

		var zero [subframeLen]int16
		var pitchContrib [subframeLen]int16
		var fixedContrib [subframeLen]int16
		synth.BuildExcitation(st.GpQ14, 0, 0, &st.V, &zero, &pitchContrib)
		synth.BuildExcitation(0, st.GainTaps.GcMantQ14, st.GainTaps.GcExp, &zero, &st.C, &fixedContrib)

		appendDecoderReferenceArray(rows, frame, sub, "past_exc_pre_acb_q0", st.PastExcPreACB[:])
		appendDecoderReferenceArray(rows, frame, sub, "adaptive_v_q0", st.V[:])
		appendDecoderReferenceArray(rows, frame, sub, "fixed_c_q13", st.C[:])
		appendDecoderReferenceArray(rows, frame, sub, "pitch_contrib_q0", pitchContrib[:])
		appendDecoderReferenceArray(rows, frame, sub, "fixed_contrib_q0", fixedContrib[:])
		appendDecoderReferenceArray(rows, frame, sub, "excitation_u_q0", st.U[:])
		appendDecoderReferenceArray(rows, frame, sub, "synth_s_q0", st.S[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_s_q0", st.SPf[:])
	}
}

func appendDecoderReferenceTAMEGainInternalRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		g := st.GainTaps
		ga, gb := decoderReferenceGainIndices(taps, sub)

		appendDecoderReferenceScalar(rows, frame, sub, "bitstream_ga", ga)
		appendDecoderReferenceScalar(rows, frame, sub, "bitstream_gb", gb)
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_codebook_energy_q26", decoderTAMEFixedCodebookEnergy64(st.C[:]))
		appendDecoderReferenceScalar(rows, frame, sub, "predicted_energy_q10", int64(g.Predicted)-int64(tables.GainMeanEnergyQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "ec_bar_q10", int64(tables.GainMeanEnergyQ10)-int64(g.EcBarDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "log_gain_q10", int64(g.LogGainDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "log2_gc_q10", int64(g.Log2GcQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "gamma_q13", int64(g.GammaCQ13))
		appendDecoderReferenceScalar(rows, frame, sub, "gc0_q14", int64(g.Gc0MantQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_q14", gainQ14FromMantExp(g.GcMantQ14, g.GcExp))
		appendDecoderReferenceScalar(rows, frame, sub, "u_current_q10", int64(g.UCurrent))
		for i, value := range g.PastErrorsBefore {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "past_errors_before_q10", i, int64(value))
		}
		for i, value := range g.PastErrorsAfter {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "past_errors_after_q10", i, int64(value))
		}
	}
}

func appendDecoderReferenceTAMEPostfilterMicroRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		pf := &st.PostfilterTaps

		appendDecoderReferenceArray(rows, frame, sub, "lp_a_q12", st.A[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_a_num_q12", pf.ANum[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_a_den_q12", pf.ADen[:])

		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_s_before_q0", pf.PastSBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_s_after_q0", pf.PastSAfter[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_residual_before_q0", pf.PastResidualBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_residual_after_q0", pf.PastResidualAfter[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_synth_post_before_q0", pf.PastSynthPostBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_past_synth_post_after_q0", pf.PastSynthPostAfter[:])
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_past_tilt_input_before_q0", int64(pf.PastTiltInputBefore))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_past_tilt_input_after_q0", int64(pf.PastTiltInputAfter))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_agc_gain_before_q24", int64(pf.AGCGainBeforeQ24))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_agc_gain_after_q24", int64(pf.AGCGainAfterQ24))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_initialized_before_q0", boolToInt64(pf.InitializedBefore))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_initialized_after_q0", boolToInt64(pf.InitializedAfter))

		appendDecoderReferenceArray(rows, frame, sub, "postfilter_residual_q0", pf.Residual[:])
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_refined_t_q0", int64(pf.LongTermT))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_longterm_g0_q14", int64(pf.LongTermG0))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_longterm_g1_q14", int64(pf.LongTermG1))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_longterm_gamma_scaled_q15", int64(pf.LongTermGammaScaledQ15))
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_longterm_enabled_q0", boolToInt64(pf.LongTermEnabled))
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_longterm_q0", pf.LongTerm[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_shortterm_q0", pf.ShortTerm[:])
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_tilt_mu_q15", int64(pf.TiltMuQ15))
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_tilt_q0", pf.Tilt[:])
		appendDecoderReferenceScalar(rows, frame, sub, "postfilter_agc_target_q14", int64(pf.AGCTargetQ14))
		appendDecoderReferenceInt32Array(rows, frame, sub, "postfilter_agc_gain_q24", pf.AGCGainQ24[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_s_q0", pf.Output[:])
	}
}

func appendDecoderReferenceTAMEPostfilterAGCArithRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		pf := &st.PostfilterTaps

		energyIn, absIn := decoderReferenceEnergyAndAbs(st.S[:])
		energyPost, absPost := decoderReferenceEnergyAndAbs(pf.ShortTerm[:])
		appendDecoderReferenceScalar(rows, frame, sub, "agc_energy_input_raw_q0", energyIn)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_energy_postfilter_raw_q0", energyPost)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_abs_input_raw_q0", absIn)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_abs_postfilter_raw_q0", absPost)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_target_internal_q14", int64(pf.AGCTargetQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "agc_target_internal_q24", int64(pf.AGCTargetQ14)<<10)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_gain_before_q24", int64(pf.AGCGainBeforeQ24))
		appendDecoderReferenceScalar(rows, frame, sub, "agc_gain_after_q24", int64(pf.AGCGainAfterQ24))

		appendDecoderReferenceArray(rows, frame, sub, "agc_input_s_q0", st.S[:])
		appendDecoderReferenceArray(rows, frame, sub, "agc_postfilter_pre_agc_q0", pf.ShortTerm[:])
		appendDecoderReferenceInt32Array(rows, frame, sub, "agc_gain_before_update_q24", pf.AGCGainBeforeUpdateQ24[:])
		appendDecoderReferenceInt32Array(rows, frame, sub, "agc_update_mul_prev_q0", pf.AGCUpdateMulPrevQ0[:])
		appendDecoderReferenceInt32Array(rows, frame, sub, "agc_update_mul_target_q0", pf.AGCUpdateMulTargetQ0[:])
		appendDecoderReferenceInt32Array(rows, frame, sub, "agc_update_acc_q0", pf.AGCUpdateAccQ0[:])
		appendDecoderReferenceInt32Array(rows, frame, sub, "agc_gain_after_update_q24", pf.AGCGainQ24[:])
		appendDecoderReferenceInt64Array(rows, frame, sub, "agc_output_product_q24", pf.AGCOutputProductQ24[:])
		appendDecoderReferenceArray(rows, frame, sub, "agc_output_q0", pf.Output[:])
	}
}

func appendDecoderReferenceAGCTargetDecisionRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		pf := &st.PostfilterTaps
		decision := decoderReferenceAGCTargetDecision(st.S[:], pf.ShortTerm[:])

		appendDecoderReferenceScalar(rows, frame, sub, "agc_energy_input_raw_q0", decision.energyInput)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_energy_postfilter_raw_q0", decision.energyPost)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_target_path_q0", decision.path)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_norm_shift_input_q0", decision.normShiftInput)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_norm_shift_post_q0", decision.normShiftPost)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_norm_input_q0", decision.normInput)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_norm_post_q0", decision.normPost)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_ratio_div_q15", decision.ratioDivQ15)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_ratio_q28", decision.ratioQ28)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_inverse_sqrt_norm_shift_q0", decision.inverseSqrtNormShift)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_inverse_sqrt_index_q0", decision.inverseSqrtIndex)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_inverse_sqrt_frac_q0", decision.inverseSqrtFrac)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_inverse_sqrt_out_pre_round_q0", decision.inverseSqrtOutPreRound)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_sqrt_q12", decision.sqrtQ12)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_target_q12", decision.targetQ12)
		appendDecoderReferenceScalar(rows, frame, sub, "agc_target_internal_q14", int64(pf.AGCTargetQ14))
	}
}

func appendDecoderReferenceTAMEOutputHPArithRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		hp := &taps.Sub[sub].HPTaps
		for i, sample := range hp.Sample {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_input_postfilter_q0", i, int64(sample.InputPostfilter))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_x1_before_q0", i, int64(sample.X1Before))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_x2_before_q0", i, int64(sample.X2Before))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_before_native_q0", i, int64(sample.Y1Before))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_before_native_q0", i, int64(sample.Y2Before))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_feedforward_acc_native_q0", i, int64(sample.Feedforward))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_feedback_acc_native_q0", i, int64(sample.Feedback))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_total_acc_native_q0", i, int64(sample.Total))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_output_pre_scale_q0", i, int64(sample.OutputPreScale))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_output_pcm_q0", i, int64(sample.OutputPCM))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_x1_after_q0", i, int64(sample.X1After))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_x2_after_q0", i, int64(sample.X2After))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_after_native_q0", i, int64(sample.Y1After))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_after_native_q0", i, int64(sample.Y2After))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_hi_before_q0", i, int64(sample.Y1HiBefore))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_lo_before_q0", i, int64(sample.Y1LoBefore))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_hi_before_q0", i, int64(sample.Y2HiBefore))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_lo_before_q0", i, int64(sample.Y2LoBefore))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_hi_after_q0", i, int64(sample.Y1HiAfter))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y1_lo_after_q0", i, int64(sample.Y1LoAfter))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_hi_after_q0", i, int64(sample.Y2HiAfter))
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_y2_lo_after_q0", i, int64(sample.Y2LoAfter))
		}
	}
}

func appendDecoderReferenceRecoveryStateRows(rows *[]stageRow, frame int, bad bool, zeroSoftbits int, taps *Phase3DiagFrameTaps) {
	frameMode := int64(0)
	lspUpdate := int64(1)
	if bad || zeroSoftbits > 0 {
		frameMode = 1
		lspUpdate = 0
	}

	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		appendDecoderReferenceScalar(rows, frame, sub, "g192_zero_softbit_count_q0", int64(zeroSoftbits))
		appendDecoderReferenceScalar(rows, frame, sub, "reference_frame_mode_q0", frameMode)
		appendDecoderReferenceScalar(rows, frame, sub, "reference_lsp_update_q0", lspUpdate)
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_int", int64(st.TInt))
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_frac", int64(st.TFrac))
		appendDecoderReferenceScalar(rows, frame, sub, "erasure_random_seed_before_q0", int64(int16(st.ErasureRandomSeedBefore)))
		appendDecoderReferenceScalar(rows, frame, sub, "erasure_random_seed_after_q0", int64(int16(st.ErasureRandomSeedAfter)))
		appendDecoderReferenceScalar(rows, frame, sub, "erasure_fixed_c_index_q0", int64(st.FixedCodebookIndex))
		appendDecoderReferenceScalar(rows, frame, sub, "erasure_fixed_s_index_q0", int64(st.FixedCodebookSigns))

		appendDecoderReferenceArray(rows, frame, sub, "fixed_c_q13", st.C[:])
		appendDecoderReferenceScalar(rows, frame, sub, "adaptive_gain_before_erasure_q14", int64(st.AdaptiveGainBeforeQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "adaptive_gain_after_erasure_q14", int64(st.AdaptiveGainAfterQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_before_erasure_q14", st.FixedGainBeforeQ14)
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_after_erasure_q14", st.FixedGainAfterQ14)
		appendDecoderReferenceArray(rows, frame, sub, "gain_predictor_error_before_q10", st.GainPredictorErrorBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "gain_predictor_error_after_q10", st.GainPredictorErrorAfter[:])
		appendDecoderReferenceScalar(rows, frame, sub, "gain_erasure_avg_q10", int64(st.GainErasureAvgQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "gain_erasure_update_q10", int64(st.GainErasureUpdateQ10))

		appendDecoderReferenceArray(rows, frame, sub, "prev_lsp_before_erasure_q15", taps.PrevLSPBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "prev_lsp_after_erasure_q15", taps.PrevLSPAfter[:])
		appendDecoderReferenceArray(rows, frame, sub, "prev_lsf_before_erasure_q13", taps.PrevLSFBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "prev_lsf_after_erasure_q13", taps.PrevLSFAfter[:])
		appendDecoderReferenceArray(rows, frame, sub, "lp_a_q12", st.A[:])
		appendDecoderReferenceArray(rows, frame, sub, "synth_mem_before_q0", st.SynthMemBefore[:])
		appendDecoderReferenceArray(rows, frame, sub, "synth_mem_after_q0", st.SynthMemAfter[:])
		appendDecoderReferenceArray(rows, frame, sub, "excitation_u_q0", st.U[:])
		appendDecoderReferenceArray(rows, frame, sub, "synth_s_q0", st.S[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_s_q0", st.SPf[:])
		for i, sample := range st.HPTaps.Sample {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "hp_output_pcm_q0", i, int64(sample.OutputPCM))
		}
	}
}

func appendDecoderReferenceLSPPredictorStateRows(rows *[]stageRow, frame int, bad bool, zeroSoftbits int, taps *Phase3DiagFrameTaps) {
	frameMode := int64(0)
	lspUpdate := int64(1)
	if bad || zeroSoftbits > 0 {
		frameMode = 1
		lspUpdate = 0
	}

	appendDecoderReferenceScalar(rows, frame, -1, "reference_frame_mode_q0", frameMode)
	appendDecoderReferenceScalar(rows, frame, -1, "reference_lsp_update_q0", lspUpdate)
	appendDecoderReferenceLSPResidualMatrix(rows, frame, "lsp_past_residual_before_q13", taps.LSPPastResidualBefore)
	appendDecoderReferenceLSPResidualMatrix(rows, frame, "lsp_past_residual_after_q13", taps.LSPPastResidualAfter)
	appendDecoderReferenceArray(rows, frame, -1, "prev_lsp_before_q15", taps.PrevLSPBefore[:])
	appendDecoderReferenceArray(rows, frame, -1, "prev_lsp_after_q15", taps.PrevLSPAfter[:])
	appendDecoderReferenceArray(rows, frame, -1, "lsf_after_predictor_q13", taps.LSFAfterPredictor[:])
	appendDecoderReferenceArray(rows, frame, -1, "lsf_after_stability_q13", taps.LSFAfterStability[:])
	appendDecoderReferenceArray(rows, frame, -1, "curr_lsp_q15", taps.CurrLSP[:])
}

func appendDecoderReferenceLSPResidualMatrix(rows *[]stageRow, frame int, field string, values [4][lpcOrder]int16) {
	for tap := 0; tap < 4; tap++ {
		for i, value := range values[tap] {
			appendDecoderReferenceIndexedScalar(rows, frame, -1, field, tap*lpcOrder+i, int64(value))
		}
	}
}

func decoderReferenceGainIndices(taps *Phase3DiagFrameTaps, sub int) (ga, gb int64) {
	if sub == 0 {
		return int64(taps.Frame.GA1), int64(taps.Frame.GB1)
	}
	return int64(taps.Frame.GA2), int64(taps.Frame.GB2)
}

func appendDecoderReferenceScalar(rows *[]stageRow, frame, sub int, field string, value int64) {
	appendDecoderReferenceIndexedScalar(rows, frame, sub, field, 0, value)
}

func appendDecoderReferenceIndexedScalar(rows *[]stageRow, frame, sub int, field string, index int, value int64) {
	*rows = append(*rows, stageRow{
		source:   "TAME",
		frame:    frame,
		sub:      sub,
		field:    field,
		index:    index,
		hasValue: true,
		value:    value,
	})
}

func appendDecoderReferenceArray(rows *[]stageRow, frame, sub int, field string, values []int16) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      sub,
			field:    field,
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

func appendDecoderReferenceInt32Array(rows *[]stageRow, frame, sub int, field string, values []int32) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      sub,
			field:    field,
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

func appendDecoderReferenceInt64Array(rows *[]stageRow, frame, sub int, field string, values []int64) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      sub,
			field:    field,
			index:    i,
			hasValue: true,
			value:    value,
		})
	}
}

func appendDecoderReferenceFrameArray(rows *[]stageRow, frame int, field string, values []int16) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      -1,
			field:    field,
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

func logFirstDecoderReferenceMismatchByField(t testing.TB, expected []stageRow, got map[decoderStageKey]stageRow) {
	t.Helper()

	firstByField := make(map[string]decoderStageMismatch)
	for _, want := range expected {
		key := decoderStageRowKey(want)
		gotRow, ok := got[key]
		if ok && gotRow.hasValue && gotRow.value == want.value {
			continue
		}
		if _, exists := firstByField[key.field]; exists {
			continue
		}
		gotText := ""
		note := "missing got"
		if ok {
			gotText = decoderStageValueString(gotRow)
			note = "mismatch"
		}
		firstByField[key.field] = decoderStageMismatch{
			key:  key,
			want: decoderStageValueString(want),
			got:  gotText,
			note: note,
		}
	}

	fields := make([]string, 0, len(firstByField))
	for field := range firstByField {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		m := firstByField[field]
		t.Logf("first[%s]: source=%s frame=%d sub=%d index=%d expected=%s got=%s notes=%s",
			field, m.key.source, m.key.frame, m.key.sub, m.key.index, m.want, m.got, m.note)
	}
}

func decoderReferenceEnergyAndAbs(values []int16) (energy, absSum int64) {
	for _, value := range values {
		raw := int64(value)
		if raw < 0 {
			absSum -= raw
		} else {
			absSum += raw
		}
		v := int64(value >> 2)
		energy += 2 * v * v
		if energy > int64(fixed.Max32) {
			energy = int64(fixed.Max32)
		}
	}
	return energy, absSum
}

type decoderReferenceAGCTargetDecisionValues struct {
	energyInput            int64
	energyPost             int64
	path                   int64
	normShiftInput         int64
	normShiftPost          int64
	normInput              int64
	normPost               int64
	ratioDivQ15            int64
	ratioQ28               int64
	inverseSqrtNormShift   int64
	inverseSqrtIndex       int64
	inverseSqrtFrac        int64
	inverseSqrtOutPreRound int64
	sqrtQ12                int64
	targetQ12              int64
	targetInternalQ14      int64
}

func decoderReferenceAGCTargetDecision(input, post []int16) decoderReferenceAGCTargetDecisionValues {
	const agcAlphaComplementQ15 = int64(3276)

	inputEnergy, _ := decoderReferenceEnergyAndAbs(input)
	postEnergy, _ := decoderReferenceEnergyAndAbs(post)
	out := decoderReferenceAGCTargetDecisionValues{
		energyInput: inputEnergy,
		energyPost:  postEnergy,
	}
	if inputEnergy > 0 {
		out.normShiftInput = int64(decoderReferenceAGCTargetNormShift(inputEnergy))
		out.normInput = decoderReferenceAGCTargetRoundedNorm(inputEnergy, int(out.normShiftInput))
	}
	if postEnergy > 0 {
		out.normShiftPost = int64(decoderReferenceAGCTargetNormShift(postEnergy))
		out.normShiftPost--
		out.normPost = decoderReferenceAGCTargetRoundedNorm(postEnergy, int(out.normShiftPost))
	}

	if inputEnergy == 0 && postEnergy > 0 && out.normShiftPost > 0 {
		out.normShiftPost--
		out.normPost = decoderReferenceAGCTargetRoundedNorm(postEnergy, int(out.normShiftPost))
	}

	if inputEnergy == 0 || postEnergy == 0 {
		return out
	}

	if out.normPost <= 0 || out.normInput <= 0 {
		return out
	}
	out.ratioDivQ15 = int64(fixed.DivS(int16(out.normPost), int16(out.normInput)))
	expDelta := out.normShiftPost - out.normShiftInput
	shift := 7 - expDelta
	out.ratioQ28 = out.ratioDivQ15
	if shift >= 0 {
		out.ratioQ28 <<= shift
	} else {
		out.ratioQ28 >>= -shift
	}

	inv := decoderReferenceAGCInverseSqrtDetails(out.ratioQ28)
	out.inverseSqrtNormShift = int64(inv.normShift)
	out.inverseSqrtIndex = int64(inv.index)
	out.inverseSqrtFrac = inv.frac
	out.inverseSqrtOutPreRound = inv.outPreRound
	out.sqrtQ12 = inv.sqrtQ12

	out.path = 0
	out.targetQ12 = (out.sqrtQ12 * agcAlphaComplementQ15) >> 15
	out.targetInternalQ14 = out.targetQ12 << 2
	return out
}

func decoderReferenceAGCTargetNormShift(v int64) int {
	if v <= 0 {
		return 0
	}
	var shift int
	for v < 0x40000000 {
		v <<= 1
		shift++
	}
	return shift
}

func decoderReferenceAGCTargetRoundedNorm(v int64, shift int) int64 {
	if shift < 0 {
		norm := ((v >> uint(-shift)) + 0x8000) >> 16
		if norm > 32767 {
			return 32767
		}
		return norm
	}
	norm := ((v << shift) + 0x8000) >> 16
	if norm > 32767 {
		return 32767
	}
	return norm
}

type decoderReferenceAGCInverseSqrtDetailsValues struct {
	normShift   int
	index       int
	frac        int64
	outPreRound int64
	sqrtQ12     int64
}

func decoderReferenceAGCInverseSqrtDetails(x int64) decoderReferenceAGCInverseSqrtDetailsValues {
	if x <= 0 {
		return decoderReferenceAGCInverseSqrtDetailsValues{}
	}
	normShift := decoderReferenceAGCTargetNormShift(x)
	normX := x << normShift
	adjustedX := normX
	if normShift%2 == 0 {
		adjustedX >>= 1
	}

	index := int((adjustedX >> 25) - 16)
	if index < 0 {
		index = 0
	}
	frac := (adjustedX >> 10) & 0x7fff

	base := decoderReferenceAGCInverseSqrtTableValue(index)
	next := decoderReferenceAGCInverseSqrtTableValue(index + 1)
	acc := (base << 16) - 2*(base-next)*frac
	denormShift := 16 - ((normShift + 1) >> 1)
	outPreRound := acc >> denormShift
	return decoderReferenceAGCInverseSqrtDetailsValues{
		normShift:   normShift,
		index:       index,
		frac:        frac,
		outPreRound: outPreRound,
		sqrtQ12:     (outPreRound + 64) >> 7,
	}
}

func decoderReferenceAGCInverseSqrtTableValue(index int) int64 {
	const tableScale = int64(32768)
	numerator := int64(16) * tableScale * tableScale
	denominator := int64(16 + index)
	root := decoderReferenceISqrt64(numerator / denominator)
	for (root+1)*(root+1)*denominator <= numerator {
		root++
	}
	loDiff := numerator - root*root*denominator
	hiDiff := (root+1)*(root+1)*denominator - numerator
	if hiDiff < loDiff {
		root++
	}
	if root > int64(fixed.Max16) {
		return int64(fixed.Max16)
	}
	return root
}

func decoderReferenceISqrtRoundedQ12(xQ24 int64) int64 {
	if xQ24 <= 0 {
		return 0
	}
	root := decoderReferenceISqrt64(xQ24)
	loDiff := xQ24 - root*root
	hi := root + 1
	hiDiff := hi*hi - xQ24
	if hiDiff < loDiff {
		return hi
	}
	return root
}

func decoderReferenceISqrt64(x int64) int64 {
	if x <= 0 {
		return 0
	}
	guess := x
	for {
		next := (guess + x/guess) >> 1
		if next >= guess {
			return guess
		}
		guess = next
	}
}

func appendDecoderReferenceG192FrameModeRows(rows *[]stageRow, frame int, bad bool, zeroSoftbits int) {
	frameMode := int64(0)
	lspUpdate := int64(1)
	if bad || zeroSoftbits > 0 {
		frameMode = 1
		lspUpdate = 0
	}
	appendDecoderReferenceScalar(rows, frame, -1, "g192_zero_softbit_count_q0", int64(zeroSoftbits))
	appendDecoderReferenceScalar(rows, frame, -1, "reference_frame_mode_q0", frameMode)
	appendDecoderReferenceScalar(rows, frame, -1, "reference_lsp_update_q0", lspUpdate)
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

type decoderReferenceStageFieldStats struct {
	total      int
	exact      int
	mismatches int
	missing    int
	maxAbs     int64
}

func decoderReferenceStageStatsFor(stats map[string]*decoderReferenceStageFieldStats, field string) *decoderReferenceStageFieldStats {
	st := stats[field]
	if st == nil {
		st = &decoderReferenceStageFieldStats{}
		stats[field] = st
	}
	return st
}

func decoderReferenceStageFieldSummary(stats map[string]*decoderReferenceStageFieldStats) []string {
	fields := make([]string, 0, len(stats))
	for field := range stats {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool {
		left := stats[fields[i]]
		right := stats[fields[j]]
		if left.mismatches != right.mismatches {
			return left.mismatches > right.mismatches
		}
		return fields[i] < fields[j]
	})

	out := make([]string, 0, len(fields))
	for _, field := range fields {
		st := stats[field]
		out = append(out, fmt.Sprintf("field %-22s exact=%d/%d %.2f%% mismatches=%d missing=%d max_abs=%d",
			field, st.exact, st.total, percent(st.exact, st.total), st.mismatches, st.missing, st.maxAbs))
	}
	return out
}
