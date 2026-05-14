package decoder

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const decoderTAMEStageWideExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_stage_wide_expected.csv"
const decoderTAMEStageWideOnsetExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_stage_wide_onset_expected_template.csv"
const decoderTAMEStageWideOnsetGotPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_stage_wide_onset_got.csv"

func TestDecoderTAMEStageWideOnsetHandoffTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_TAME_STAGE_WIDE_ONSET_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_DECODER_TAME_STAGE_WIDE_ONSET_HANDOFF=1 to write TAME onset wide handoff CSVs")
	}

	targets := map[string]map[int]struct{}{"TAME": decoderTAMEStageWideOnsetTargets()}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder TAME onset stage rows: %v", err)
	}
	columns := decoderTAMEStageWideOnsetColumns()
	if err := writeDecoderTAMEStageWideCSV(decoderTAMEStageWideOnsetGotPath, columns, got, false); err != nil {
		t.Fatalf("write decoder TAME onset got: %v", err)
	}
	if err := writeDecoderTAMEStageWideCSV(decoderTAMEStageWideOnsetExpectedTemplatePath, columns, got, true); err != nil {
		t.Fatalf("write decoder TAME onset expected template: %v", err)
	}
	t.Logf("wrote TAME onset wide handoff: got=%s expected_template=%s rows=%d cols=%d",
		decoderTAMEStageWideOnsetGotPath, decoderTAMEStageWideOnsetExpectedTemplatePath,
		len(decoderTAMEStageWideFrameSubs(got)), len(columns))
}

func TestOracleHandoff_CompareDecoderTAMEStageWideOnset(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_STAGE_WIDE_ONSET") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_STAGE_WIDE_ONSET=1 to compare decoder TAME onset wide stage artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_STAGE_WIDE_ONSET_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideOnsetExpectedTemplatePath
	}
	expected, err := readDecoderTAMEStageWideRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME onset stage wide expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME onset stage wide expected is empty")
	}

	targets := map[string]map[int]struct{}{"TAME": make(map[int]struct{})}
	for _, row := range expected {
		targets["TAME"][row.frame] = struct{}{}
	}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder TAME onset stage got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, blanks, filled, missingGot, mismatches int
	fieldStats := make(map[string]*decoderTAMEWideFieldStats)
	first := make([]decoderStageMismatch, 0, 16)

	for _, want := range expected {
		if !want.hasValue {
			blanks++
			continue
		}
		filled++
		key := decoderStageRowKey(want)
		st := decoderTAMEWideStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			st.mismatches++
			st.missing++
			if len(first) < cap(first) {
				first = append(first, decoderStageMismatch{
					key:  key,
					want: decoderStageValueString(want),
					got:  "",
					note: "missing got",
				})
			}
			continue
		}
		if gotRow.hasValue && want.value == gotRow.value {
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
		if len(first) < cap(first) {
			first = append(first, decoderStageMismatch{
				key:  key,
				want: decoderStageValueString(want),
				got:  decoderStageValueString(gotRow),
				note: "mismatch",
			})
		}
	}

	t.Logf("decoder_tame_stage_wide_onset handoff: exact %d/%d %.2f%% mismatches=%d blanks=%d missing_got=%d",
		exact, filled, percent(exact, filled), mismatches, blanks, missingGot)
	for _, line := range decoderTAMEWideFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_TAME_STAGE_WIDE_ONSET") == "1" && blanks > 0 {
		t.Fatalf("decoder TAME onset stage wide expected has blank cells: %d", blanks)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_STAGE_WIDE_ONSET") == "1" && mismatches > 0 {
		t.Fatalf("decoder TAME onset stage wide expected/got mismatches: %d", mismatches)
	}
}

func TestOracleHandoff_CompareDecoderTAMEStageWide(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_STAGE_WIDE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_STAGE_WIDE=1 to compare decoder TAME wide stage artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_STAGE_WIDE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideExpectedPath
	}
	expected, err := readDecoderTAMEStageWideRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME stage wide expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME stage wide expected is empty")
	}

	targets := map[string]map[int]struct{}{"TAME": make(map[int]struct{})}
	for _, row := range expected {
		targets["TAME"][row.frame] = struct{}{}
	}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder TAME stage got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, missingGot, mismatches int
	fieldStats := make(map[string]*decoderTAMEWideFieldStats)
	first := make([]decoderStageMismatch, 0, 16)

	for _, want := range expected {
		key := decoderStageRowKey(want)
		st := decoderTAMEWideStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			st.mismatches++
			st.missing++
			if len(first) < cap(first) {
				first = append(first, decoderStageMismatch{
					key:  key,
					want: decoderStageValueString(want),
					got:  "",
					note: "missing got",
				})
			}
			continue
		}
		if want.hasValue == gotRow.hasValue && want.value == gotRow.value {
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
		if len(first) < cap(first) {
			first = append(first, decoderStageMismatch{
				key:  key,
				want: decoderStageValueString(want),
				got:  decoderStageValueString(gotRow),
				note: "mismatch",
			})
		}
	}

	total := len(expected)
	t.Logf("decoder_tame_stage_wide handoff: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, total, percent(exact, total), mismatches, missingGot)
	for _, line := range decoderTAMEWideFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_STAGE_WIDE") == "1" && mismatches > 0 {
		t.Fatalf("decoder TAME stage wide expected/got mismatches: %d", mismatches)
	}
}

func readDecoderTAMEStageWideRows(path string) ([]stageRow, error) {
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
	if len(header) < 3 || header[0] != "frame" || header[1] != "sub" {
		return nil, fmt.Errorf("unexpected header prefix %v", header)
	}

	fields := make([]decoderTAMEWideColumn, 0, len(header)-2)
	for _, name := range header[2:] {
		col, err := parseDecoderTAMEWideColumn(name)
		if err != nil {
			return nil, err
		}
		fields = append(fields, col)
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
		if len(rec) != len(header) {
			return nil, fmt.Errorf("line %d: got %d columns, want %d", line, len(rec), len(header))
		}
		frame, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, fmt.Errorf("line %d frame: %w", line, err)
		}
		sub, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d sub: %w", line, err)
		}
		for i, col := range fields {
			row := stageRow{
				source: "TAME",
				frame:  frame,
				sub:    sub,
				field:  col.field,
				index:  col.index,
			}
			if rec[i+2] != "" {
				value, err := strconv.ParseInt(rec[i+2], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("line %d %s: %w", line, header[i+2], err)
				}
				row.hasValue = true
				row.value = value
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func decoderTAMEStageWideOnsetTargets() map[int]struct{} {
	targets := make(map[int]struct{})
	for _, r := range [][2]int{
		{0, 5},
		{22, 33},
		{49, 60},
		{68, 79},
		{112, 127},
	} {
		for frame := r[0]; frame <= r[1]; frame++ {
			targets[frame] = struct{}{}
		}
	}
	return targets
}

func decoderTAMEStageWideOnsetColumns() []decoderTAMEWideColumn {
	var columns []decoderTAMEWideColumn
	for i := 0; i < pastExcLen; i++ {
		columns = append(columns, decoderTAMEWideColumn{field: "past_exc_pre_acb_q0", index: i})
	}
	for i := 0; i <= lpcOrder; i++ {
		columns = append(columns, decoderTAMEWideColumn{field: "lp_a_q12", index: i})
	}
	columns = append(columns,
		decoderTAMEWideColumn{field: "adaptive_gain_q14", index: -1},
		decoderTAMEWideColumn{field: "fixed_gain_q14", index: -1},
	)
	for _, field := range []string{
		"adaptive_v_q0",
		"fixed_c_q13",
		"pitch_contrib_q0",
		"fixed_contrib_q0",
		"excitation_u_q0",
		"synth_s_q0",
	} {
		for i := 0; i < subframeLen; i++ {
			columns = append(columns, decoderTAMEWideColumn{field: field, index: i})
		}
	}
	return columns
}

func writeDecoderTAMEStageWideCSV(path string, columns []decoderTAMEWideColumn, rows []stageRow, blankValues bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	header := make([]string, 0, len(columns)+2)
	header = append(header, "frame", "sub")
	for _, col := range columns {
		header = append(header, col.name())
	}
	if err := w.Write(header); err != nil {
		return err
	}

	values := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		values[decoderStageRowKey(row)] = row
	}

	for _, frameSub := range decoderTAMEStageWideFrameSubs(rows) {
		rec := make([]string, 0, len(columns)+2)
		rec = append(rec, strconv.Itoa(frameSub.frame), strconv.Itoa(frameSub.sub))
		for _, col := range columns {
			key := decoderStageKey{
				source: "TAME",
				frame:  frameSub.frame,
				sub:    frameSub.sub,
				field:  col.field,
				index:  col.index,
			}
			row, ok := values[key]
			if blankValues || !ok || !row.hasValue {
				rec = append(rec, "")
				continue
			}
			rec = append(rec, strconv.FormatInt(row.value, 10))
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

type decoderTAMEWideFrameSub struct {
	frame int
	sub   int
}

func decoderTAMEStageWideFrameSubs(rows []stageRow) []decoderTAMEWideFrameSub {
	seen := make(map[decoderTAMEWideFrameSub]struct{})
	out := make([]decoderTAMEWideFrameSub, 0)
	for _, row := range rows {
		if row.source != "TAME" || row.sub < 0 {
			continue
		}
		key := decoderTAMEWideFrameSub{frame: row.frame, sub: row.sub}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].frame != out[j].frame {
			return out[i].frame < out[j].frame
		}
		return out[i].sub < out[j].sub
	})
	return out
}

type decoderTAMEWideColumn struct {
	field string
	index int
}

func (c decoderTAMEWideColumn) name() string {
	if c.index < 0 {
		return c.field
	}
	return fmt.Sprintf("%s_%d", c.field, c.index)
}

func parseDecoderTAMEWideColumn(name string) (decoderTAMEWideColumn, error) {
	switch name {
	case "adaptive_gain_q14", "fixed_gain_q14":
		return decoderTAMEWideColumn{field: name, index: -1}, nil
	}
	pos := strings.LastIndexByte(name, '_')
	if pos < 0 || pos == len(name)-1 {
		return decoderTAMEWideColumn{}, fmt.Errorf("wide column %q has no numeric suffix", name)
	}
	index, err := strconv.Atoi(name[pos+1:])
	if err != nil {
		return decoderTAMEWideColumn{}, fmt.Errorf("wide column %q suffix: %w", name, err)
	}
	return decoderTAMEWideColumn{field: name[:pos], index: index}, nil
}

type decoderTAMEWideFieldStats struct {
	total      int
	exact      int
	mismatches int
	missing    int
	maxAbs     int64
}

func decoderTAMEWideStatsFor(stats map[string]*decoderTAMEWideFieldStats, field string) *decoderTAMEWideFieldStats {
	st := stats[field]
	if st == nil {
		st = &decoderTAMEWideFieldStats{}
		stats[field] = st
	}
	return st
}

func decoderTAMEWideFieldSummary(stats map[string]*decoderTAMEWideFieldStats) []string {
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
		out = append(out, fmt.Sprintf("field %s: exact %d/%d %.2f%% mismatches=%d missing=%d maxAbs=%d",
			field, st.exact, st.total, percent(st.exact, st.total), st.mismatches, st.missing, st.maxAbs))
	}
	return out
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
