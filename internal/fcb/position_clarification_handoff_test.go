package fcb

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
)

const fcbPositionClarificationExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_fcb_position_clarification_expected.csv"

func TestOracleHandoff_CompareDecoderITUFCBPositionClarification(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_ITU_FCB_POSITION_CLARIFICATION") != "1" {
		t.Skip("set G729_COMPARE_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 to compare decoder ITU FCB position clarification")
	}

	expectedPath := os.Getenv("G729_DECODER_ITU_FCB_POSITION_CLARIFICATION_EXPECTED")
	if expectedPath == "" {
		expectedPath = fcbPositionClarificationExpectedPath
	}
	rows, err := readFCBPositionClarificationRows(expectedPath)
	if err != nil {
		t.Fatalf("read FCB position clarification: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("FCB position clarification expected is empty")
	}

	var total, exact, blank, mismatches int
	for _, row := range rows {
		got := decomposePositionCode(row.code)
		for _, field := range []string{"i0", "i1", "i2", "i3", "jx", "m0", "m1", "m2", "m3"} {
			total++
			want, ok := row.values[field]
			if !ok {
				blank++
				continue
			}
			have := got[field]
			if have == want {
				exact++
				continue
			}
			mismatches++
			t.Logf("mismatch C=%d field=%s expected=%d got=%d", row.code, field, want, have)
		}

		total++
		if row.note == "" {
			blank++
		} else if row.note == "formula_ok" {
			exact++
		} else {
			mismatches++
			t.Logf("mismatch C=%d field=note expected=formula_ok got=%s", row.code, row.note)
		}
	}

	t.Logf("decoder ITU FCB position clarification: exact %d/%d %.2f%% mismatches=%d blank=%d",
		exact, total, 100*float64(exact)/float64(total), mismatches, blank)
	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_ITU_FCB_POSITION_CLARIFICATION") == "1" && blank > 0 {
		t.Fatalf("FCB position clarification has %d blank values", blank)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FCB_POSITION_CLARIFICATION") == "1" && mismatches > 0 {
		t.Fatalf("FCB position clarification mismatches: %d", mismatches)
	}
}

type fcbPositionClarificationRow struct {
	code   uint16
	values map[string]int
	note   string
}

func readFCBPositionClarificationRows(path string) ([]fcbPositionClarificationRow, error) {
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
	wantHeader := []string{"C", "i0", "i1", "i2", "i3", "jx", "m0", "m1", "m2", "m3", "note"}
	if len(header) != len(wantHeader) {
		return nil, fmt.Errorf("header length=%d want %d", len(header), len(wantHeader))
	}
	for i := range wantHeader {
		if header[i] != wantHeader[i] {
			return nil, fmt.Errorf("header[%d]=%q want %q", i, header[i], wantHeader[i])
		}
	}

	var out []fcbPositionClarificationRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) != len(wantHeader) {
			return nil, fmt.Errorf("record length=%d want %d", len(rec), len(wantHeader))
		}
		code64, err := strconv.ParseUint(rec[0], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("parse C %q: %w", rec[0], err)
		}
		row := fcbPositionClarificationRow{
			code:   uint16(code64),
			values: make(map[string]int),
			note:   rec[10],
		}
		for i, field := range wantHeader[1:10] {
			if rec[i+1] == "" {
				continue
			}
			value, err := strconv.Atoi(rec[i+1])
			if err != nil {
				return nil, fmt.Errorf("parse C=%d %s %q: %w", row.code, field, rec[i+1], err)
			}
			row.values[field] = value
		}
		out = append(out, row)
	}
	return out, nil
}

func decomposePositionCode(code uint16) map[string]int {
	i0 := int(code & 0x07)
	i1 := int((code >> 3) & 0x07)
	i2 := int((code >> 6) & 0x07)
	jx := int((code >> 9) & 0x01)
	i3 := int((code >> 10) & 0x07)
	return map[string]int{
		"i0": i0,
		"i1": i1,
		"i2": i2,
		"i3": i3,
		"jx": jx,
		"m0": 5 * i0,
		"m1": 5*i1 + 1,
		"m2": 5*i2 + 2,
		"m3": 5*i3 + 3 + jx,
	}
}
