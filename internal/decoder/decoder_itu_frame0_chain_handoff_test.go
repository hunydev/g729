package decoder

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const decoderITUFrame0ChainExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_stage_frame0_chain_expected.csv"
const decoderITUFrame0HPInputInverseExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv"

func TestOracleHandoff_CompareDecoderITUFrame0Chain(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_ITU_FRAME0_CHAIN") != "1" {
		t.Skip("set G729_COMPARE_DECODER_ITU_FRAME0_CHAIN=1 to compare decoder ITU frame-0 chain artifact")
	}

	expected, err := readDecoderITUFrame0ChainRows(decoderITUFrame0ChainExpectedPath)
	if err != nil {
		t.Fatalf("read decoder ITU frame-0 chain expected: %v", err)
	}
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder ITU frame-0 chain got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var fixedExact, fixedTotal, pstExact, pstTotal int
	var hpRangeTotal, hpRangeOK, missingGot int
	first := make([]decoderStageMismatch, 0, 16)
	bySource := make(map[string]*decoderITUFrame0ChainStats)

	hpRanges := make(map[decoderStageKey]*decoderITUFrame0HPRange)
	for _, want := range expected {
		key := decoderStageRowKey(want.stageRow)
		switch want.field {
		case "fixed_c_q13":
			fixedTotal++
			sourceStats := decoderITUFrame0ChainStatsFor(bySource, want.source)
			sourceStats.fixedTotal++
			gotRow, ok := gotByKey[key]
			if !ok {
				missingGot++
				appendFrame0ChainMismatch(&first, key, decoderStageValueString(want.stageRow), "", "missing got")
				continue
			}
			if gotRow.hasValue && gotRow.value == want.value {
				fixedExact++
				sourceStats.fixedExact++
				continue
			}
			appendFrame0ChainMismatch(&first, key, decoderStageValueString(want.stageRow), decoderStageValueString(gotRow), "fixed_c mismatch")
		case "pst_pcm_q0":
			pstTotal++
			sourceStats := decoderITUFrame0ChainStatsFor(bySource, want.source)
			sourceStats.pstTotal++
			gotKey := key
			gotKey.field = "pcm_q0"
			gotRow, ok := gotByKey[gotKey]
			if !ok {
				missingGot++
				appendFrame0ChainMismatch(&first, gotKey, decoderStageValueString(want.stageRow), "", "missing pcm got")
				continue
			}
			if gotRow.hasValue && gotRow.value == want.value {
				pstExact++
				sourceStats.pstExact++
				continue
			}
			appendFrame0ChainMismatch(&first, gotKey, decoderStageValueString(want.stageRow), decoderStageValueString(gotRow), "pst_pcm mismatch")
		case "hp_inverse_low_q0", "hp_inverse_high_q0":
			gotKey := key
			gotKey.field = "hp_q0"
			rng := hpRanges[gotKey]
			if rng == nil {
				rng = &decoderITUFrame0HPRange{}
				hpRanges[gotKey] = rng
			}
			if want.field == "hp_inverse_low_q0" {
				rng.haveLow = true
				rng.low = want.value
			} else {
				rng.haveHigh = true
				rng.high = want.value
			}
		default:
			t.Fatalf("unexpected frame-0 chain field %q", want.field)
		}
	}

	for key, rng := range hpRanges {
		if !rng.haveLow || !rng.haveHigh {
			t.Fatalf("incomplete hp inverse range for %+v", key)
		}
		hpRangeTotal++
		sourceStats := decoderITUFrame0ChainStatsFor(bySource, key.source)
		sourceStats.hpTotal++
		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), "", "missing hp got")
			continue
		}
		if gotRow.hasValue && gotRow.value >= rng.low && gotRow.value <= rng.high {
			hpRangeOK++
			sourceStats.hpOK++
			continue
		}
		appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), decoderStageValueString(gotRow), "hp outside inverse range")
	}

	t.Logf("decoder_itu_frame0_chain: fixed_c_q13 exact %d/%d %.2f%%",
		fixedExact, fixedTotal, percent(fixedExact, fixedTotal))
	t.Logf("decoder_itu_frame0_chain: pst_pcm_q0 exact %d/%d %.2f%%",
		pstExact, pstTotal, percent(pstExact, pstTotal))
	t.Logf("decoder_itu_frame0_chain: hp_q0 within PST-derived inverse range %d/%d %.2f%% missing_got=%d",
		hpRangeOK, hpRangeTotal, percent(hpRangeOK, hpRangeTotal), missingGot)
	for _, source := range sortedStringKeys(bySource) {
		st := bySource[source]
		t.Logf("decoder_itu_frame0_chain source %s: fixed_c=%d/%d pst=%d/%d hp_range=%d/%d",
			source, st.fixedExact, st.fixedTotal, st.pstExact, st.pstTotal, st.hpOK, st.hpTotal)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_CHAIN_FIXED_C") == "1" && fixedExact != fixedTotal {
		t.Fatalf("decoder ITU frame-0 fixed_c rows mismatch: exact %d/%d", fixedExact, fixedTotal)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_CHAIN") == "1" &&
		(fixedExact != fixedTotal || pstExact != pstTotal || hpRangeOK != hpRangeTotal || missingGot != 0) {
		t.Fatalf("decoder ITU frame-0 chain is not exact: fixed_c=%d/%d pst=%d/%d hp=%d/%d missing=%d",
			fixedExact, fixedTotal, pstExact, pstTotal, hpRangeOK, hpRangeTotal, missingGot)
	}
}

func TestDecoderITUFrame0HPInputInverseTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE_TEMPLATE") != "1" {
		t.Skip("set G729_WRITE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE_TEMPLATE=1 to write decoder frame-0 HP-input inverse template")
	}
	if err := guardDecoderVerifierExpectedTemplate(decoderITUFrame0HPInputInverseExpectedTemplatePath, "expected"); err != nil {
		t.Fatal(err)
	}

	chainRows, err := readDecoderITUFrame0ChainRows(decoderITUFrame0ChainExpectedPath)
	if err != nil {
		t.Fatalf("read decoder ITU frame-0 chain expected: %v", err)
	}
	template := make([]stageRow, 0, 480)
	for _, row := range chainRows {
		if row.field != "pst_pcm_q0" {
			continue
		}
		for _, field := range []string{"postfilter_inverse_low_q0", "postfilter_inverse_high_q0"} {
			template = append(template, stageRow{
				source: row.source,
				frame:  row.frame,
				sub:    row.sub,
				field:  field,
				index:  row.index,
			})
		}
	}
	if len(template) != 480 {
		t.Fatalf("template rows=%d, want 480", len(template))
	}
	if err := writeStageCSV(decoderITUFrame0HPInputInverseExpectedTemplatePath, "expected", template); err != nil {
		t.Fatalf("write decoder frame-0 HP-input inverse template: %v", err)
	}
	t.Logf("wrote %d rows to %s", len(template), decoderITUFrame0HPInputInverseExpectedTemplatePath)
}

func TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 to compare decoder frame-0 HP-input inverse artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_ITU_FRAME0_HP_INPUT_INVERSE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderITUFrame0HPInputInverseExpectedTemplatePath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder frame-0 HP-input inverse expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder frame-0 HP-input inverse expected is empty")
	}

	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder ITU frame-0 HP-input got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var blankExpected, malformedField int
	ranges := make(map[decoderStageKey]*decoderITUFrame0HPRange)
	for _, want := range expected {
		if !want.hasValue {
			blankExpected++
			continue
		}
		key := decoderStageRowKey(want)
		key.field = "postfilter_s_q0"
		rng := ranges[key]
		if rng == nil {
			rng = &decoderITUFrame0HPRange{}
			ranges[key] = rng
		}
		switch want.field {
		case "postfilter_inverse_low_q0":
			rng.haveLow = true
			rng.low = want.value
		case "postfilter_inverse_high_q0":
			rng.haveHigh = true
			rng.high = want.value
		default:
			malformedField++
		}
	}

	var rangeTotal, rangeOK, rangeIncomplete, missingGot int
	first := make([]decoderStageMismatch, 0, 16)
	bySource := make(map[string]*decoderITUFrame0ChainStats)
	for key, rng := range ranges {
		sourceStats := decoderITUFrame0ChainStatsFor(bySource, key.source)
		sourceStats.hpTotal++
		if !rng.haveLow || !rng.haveHigh {
			rangeIncomplete++
			appendFrame0ChainMismatch(&first, key, "complete low/high range", "", "incomplete range")
			continue
		}
		rangeTotal++
		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), "", "missing postfilter got")
			continue
		}
		if gotRow.hasValue && gotRow.value >= rng.low && gotRow.value <= rng.high {
			rangeOK++
			sourceStats.hpOK++
			continue
		}
		appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), decoderStageValueString(gotRow), "postfilter_s outside inverse range")
	}

	t.Logf("decoder_itu_frame0_hp_input_inverse: postfilter_s_q0 within verifier inverse range %d/%d %.2f%% blanks=%d incomplete=%d missing_got=%d malformed_field=%d",
		rangeOK, rangeTotal, percent(rangeOK, rangeTotal), blankExpected, rangeIncomplete, missingGot, malformedField)
	for _, source := range sortedStringKeys(bySource) {
		st := bySource[source]
		t.Logf("decoder_itu_frame0_hp_input_inverse source %s: postfilter_range=%d/%d",
			source, st.hpOK, st.hpTotal)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE") == "1" &&
		(blankExpected != 0 || rangeIncomplete != 0 || malformedField != 0) {
		t.Fatalf("decoder frame-0 HP-input inverse expected incomplete: blanks=%d incomplete=%d malformed=%d",
			blankExpected, rangeIncomplete, malformedField)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE") == "1" &&
		(blankExpected != 0 || rangeIncomplete != 0 || malformedField != 0 || missingGot != 0 || rangeOK != rangeTotal) {
		t.Fatalf("decoder frame-0 HP-input inverse mismatch: range=%d/%d blanks=%d incomplete=%d malformed=%d missing=%d",
			rangeOK, rangeTotal, blankExpected, rangeIncomplete, malformedField, missingGot)
	}
}

type decoderITUFrame0ChainRow struct {
	stageRow
	note string
}

type decoderITUFrame0HPRange struct {
	low      int64
	high     int64
	haveLow  bool
	haveHigh bool
}

type decoderITUFrame0ChainStats struct {
	fixedExact int
	fixedTotal int
	pstExact   int
	pstTotal   int
	hpOK       int
	hpTotal    int
}

func decoderITUFrame0ChainStatsFor(stats map[string]*decoderITUFrame0ChainStats, source string) *decoderITUFrame0ChainStats {
	st := stats[source]
	if st == nil {
		st = &decoderITUFrame0ChainStats{}
		stats[source] = st
	}
	return st
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readDecoderITUFrame0ChainRows(path string) ([]decoderITUFrame0ChainRow, error) {
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

	var rows []decoderITUFrame0ChainRow
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
		rows = append(rows, decoderITUFrame0ChainRow{
			stageRow: stageRow{
				source:   rec[0],
				frame:    frame,
				sub:      sub,
				field:    rec[3],
				index:    index,
				hasValue: true,
				value:    value,
			},
			note: rec[6],
		})
	}
	return rows, nil
}

func appendFrame0ChainMismatch(dst *[]decoderStageMismatch, key decoderStageKey, want, got, note string) {
	if len(*dst) == cap(*dst) {
		return
	}
	*dst = append(*dst, decoderStageMismatch{
		key:  key,
		want: want,
		got:  got,
		note: note,
	})
}

func guardDecoderVerifierExpectedTemplate(path, valueColumn string) error {
	if os.Getenv("G729_OVERWRITE_VERIFIER_EXPECTED") == "1" {
		return nil
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	valueIdx := -1
	for i, h := range rows[0] {
		if h == valueColumn {
			valueIdx = i
			break
		}
	}
	if valueIdx < 0 {
		return fmt.Errorf("%s is missing %q column", path, valueColumn)
	}

	var filled int
	for _, row := range rows[1:] {
		if len(row) <= valueIdx {
			continue
		}
		if strings.TrimSpace(row[valueIdx]) != "" {
			filled++
		}
	}
	if filled > 0 {
		return fmt.Errorf("refusing to overwrite verifier-filled expected template %s (%d filled cells); set G729_OVERWRITE_VERIFIER_EXPECTED=1 to regenerate from scratch", path, filled)
	}
	return nil
}
