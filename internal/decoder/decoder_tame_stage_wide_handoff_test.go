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

type decoderTAMEWideColumn struct {
	field string
	index int
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
