package lsp

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestOracleHandoff_CompareTAMELSFToLSP(t *testing.T) {
	if os.Getenv("G729_COMPARE_TAME_LSF_TO_LSP") != "1" {
		t.Skip("set G729_COMPARE_TAME_LSF_TO_LSP=1 to compare the TAME LSF-to-LSP oracle")
	}

	path := os.Getenv("G729_TAME_LSF_TO_LSP_EXPECTED")
	if path == "" {
		path = "/home/exedev/g729_untracked/verifier-output/decoder_tame_lsf_to_lsp_expected.csv"
	}

	expected, order, err := readTAMELSFToLSPExpected(path)
	if err != nil {
		t.Fatalf("read TAME LSF-to-LSP oracle: %v", err)
	}
	got, err := collectTAMELSFToLSPGot(expected)
	if err != nil {
		t.Fatalf("collect TAME LSF-to-LSP got: %v", err)
	}

	type stat struct {
		exact    int
		total    int
		missing  int
		mismatch int
	}
	stats := map[string]*stat{}
	fieldOrder := make([]string, 0)
	seenField := map[string]bool{}
	firstMismatches := make([]string, 0, 12)

	for _, k := range order {
		if !seenField[k.field] {
			seenField[k.field] = true
			fieldOrder = append(fieldOrder, k.field)
		}
		s := stats[k.field]
		if s == nil {
			s = &stat{}
			stats[k.field] = s
		}
		s.total++
		want := expected[k]
		have, ok := got[k]
		switch {
		case !ok:
			s.missing++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d field=%s index=%d missing got want=%d",
					k.source, k.frame, k.field, k.index, want,
				))
			}
		case have == want:
			s.exact++
		default:
			s.mismatch++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d field=%s index=%d got=%d want=%d delta=%d",
					k.source, k.frame, k.field, k.index, have, want, have-want,
				))
			}
		}
	}

	var exact, total, missing, mismatches int
	t.Logf("TAME LSF-to-LSP oracle: %s", path)
	for _, field := range fieldOrder {
		s := stats[field]
		exact += s.exact
		total += s.total
		missing += s.missing
		mismatches += s.mismatch
		t.Logf("  %-28s exact %3d/%3d  mismatches=%3d  missing=%3d",
			field, s.exact, s.total, s.mismatch, s.missing)
	}
	t.Logf("  TOTAL exact %d/%d %.2f%%  mismatches=%d  missing=%d",
		exact, total, 100.0*float64(exact)/float64(total), mismatches, missing)
	for _, m := range firstMismatches {
		t.Logf("  first mismatch: %s", m)
	}

	if os.Getenv("G729_REQUIRE_EXACT_TAME_LSF_TO_LSP") == "1" && exact != total {
		t.Fatalf("TAME LSF-to-LSP oracle mismatch: exact=%d total=%d mismatches=%d missing=%d",
			exact, total, mismatches, missing)
	}
}

type tameLSFToLSPKey struct {
	source string
	frame  int
	field  string
	index  int
}

func readTAMELSFToLSPExpected(path string) (map[tameLSFToLSPKey]int64, []tameLSFToLSPKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("empty csv")
	}
	wantHeader := []string{"source", "frame", "field", "index", "expected", "note"}
	if len(rows[0]) != len(wantHeader) {
		return nil, nil, fmt.Errorf("header width = %d, want %d", len(rows[0]), len(wantHeader))
	}
	for i, want := range wantHeader {
		if rows[0][i] != want {
			return nil, nil, fmt.Errorf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}

	expected := make(map[tameLSFToLSPKey]int64, len(rows)-1)
	order := make([]tameLSFToLSPKey, 0, len(rows)-1)
	for rowNum, row := range rows[1:] {
		if len(row) != len(wantHeader) {
			return nil, nil, fmt.Errorf("row %d width = %d, want %d", rowNum+2, len(row), len(wantHeader))
		}
		frame, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, nil, fmt.Errorf("row %d frame: %w", rowNum+2, err)
		}
		index, err := strconv.Atoi(row[3])
		if err != nil {
			return nil, nil, fmt.Errorf("row %d index: %w", rowNum+2, err)
		}
		value, err := strconv.ParseInt(row[4], 10, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("row %d expected: %w", rowNum+2, err)
		}
		k := tameLSFToLSPKey{
			source: row[0],
			frame:  frame,
			field:  row[2],
			index:  index,
		}
		if _, exists := expected[k]; exists {
			return nil, nil, fmt.Errorf("duplicate key at row %d: %+v", rowNum+2, k)
		}
		expected[k] = value
		order = append(order, k)
	}
	return expected, order, nil
}

func collectTAMELSFToLSPGot(expected map[tameLSFToLSPKey]int64) (map[tameLSFToLSPKey]int64, error) {
	type inputKey struct {
		source string
		frame  int
		index  int
	}
	inputs := map[inputKey]int64{}
	for k, v := range expected {
		if k.field == "lsf_to_lsp_input_q13" {
			inputs[inputKey{
				source: k.source,
				frame:  k.frame,
				index:  k.index,
			}] = v
		}
	}

	got := make(map[tameLSFToLSPKey]int64)
	for k := range expected {
		in, ok := inputs[inputKey{
			source: k.source,
			frame:  k.frame,
			index:  k.index,
		}]
		if !ok {
			return nil, fmt.Errorf("missing lsf_to_lsp_input_q13 for source=%s frame=%d index=%d",
				k.source, k.frame, k.index)
		}

		idx, frac, base, slope, out := lsfToLSPParts(int16(in))
		var value int64
		switch k.field {
		case "lsf_after_stability_q13", "lsf_to_lsp_input_q13":
			value = in
		case "lsf_to_lsp_table_index_q0":
			value = int64(idx)
		case "lsf_to_lsp_frac_q0":
			value = int64(frac)
		case "lsf_to_lsp_base_q15":
			value = int64(base)
		case "lsf_to_lsp_slope_q15":
			value = int64(slope)
		case "curr_lsp_q15":
			value = int64(out)
		default:
			continue
		}
		got[k] = value
	}
	return got, nil
}
