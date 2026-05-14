package g729

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/tables"
)

const decoderSupportTablesExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_support_tables_expected_template.csv"

type decoderSupportTableRow struct {
	table    string
	row      int
	col      int
	hasValue bool
	value    int64
}

func TestOracleHandoff_WriteDecoderSupportTablesTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_SUPPORT_TABLES_TEMPLATE") != "1" {
		t.Skip("set G729_WRITE_DECODER_SUPPORT_TABLES_TEMPLATE=1 to write decoder support tables template")
	}
	if err := guardRootVerifierExpectedTemplate(decoderSupportTablesExpectedTemplatePath, "expected"); err != nil {
		t.Fatal(err)
	}
	rows := decoderSupportTableRows(false)
	if err := writeDecoderSupportTableRows(decoderSupportTablesExpectedTemplatePath, "expected", rows); err != nil {
		t.Fatalf("write decoder support tables template: %v", err)
	}
	t.Logf("wrote %d rows to %s", len(rows), decoderSupportTablesExpectedTemplatePath)
}

func TestOracleHandoff_CompareDecoderSupportTables(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_SUPPORT_TABLES") != "1" {
		t.Skip("set G729_COMPARE_DECODER_SUPPORT_TABLES=1 to compare decoder support table artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_SUPPORT_TABLES_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderSupportTablesExpectedTemplatePath
	}
	expected, err := readDecoderSupportTableRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder support tables expected: %v", err)
	}
	gotByKey := make(map[string]decoderSupportTableRow)
	for _, row := range decoderSupportTableRows(true) {
		gotByKey[decoderSupportTableKey(row)] = row
	}

	var exact, total, blankExpected, missingGot, mismatches int
	first := make([]string, 0, 16)
	for _, want := range expected {
		key := decoderSupportTableKey(want)
		if !want.hasValue {
			blankExpected++
			continue
		}
		total++
		got, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			if len(first) < cap(first) {
				first = append(first, fmt.Sprintf("%s expected=%d got=<missing>", key, want.value))
			}
			continue
		}
		if got.value == want.value {
			exact++
			continue
		}
		mismatches++
		if len(first) < cap(first) {
			first = append(first, fmt.Sprintf("%s expected=%d got=%d", key, want.value, got.value))
		}
	}

	t.Logf("decoder_support_tables: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, total, rootPercent(exact, total), blankExpected, mismatches, missingGot)
	for i, line := range first {
		t.Logf("mismatch[%d]: %s", i, line)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES") == "1" && blankExpected != 0 {
		t.Fatalf("decoder support tables expected incomplete: blanks=%d", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES") == "1" &&
		(blankExpected != 0 || missingGot != 0 || mismatches != 0) {
		t.Fatalf("decoder support tables mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, total, blankExpected, missingGot, mismatches)
	}
}

func decoderSupportTableRows(withValues bool) []decoderSupportTableRow {
	var rows []decoderSupportTableRow
	append1D := func(table string, values []int64) {
		for i, value := range values {
			rows = append(rows, decoderSupportTableRow{
				table:    table,
				row:      i,
				col:      -1,
				hasValue: withValues,
				value:    value,
			})
		}
	}
	append2D := func(table string, values [][2]int64) {
		for i, pair := range values {
			for col, value := range pair {
				rows = append(rows, decoderSupportTableRow{
					table:    table,
					row:      i,
					col:      col,
					hasValue: withValues,
					value:    value,
				})
			}
		}
	}
	appendScalar := func(table string, value int64) {
		rows = append(rows, decoderSupportTableRow{
			table:    table,
			row:      -1,
			col:      -1,
			hasValue: withValues,
			value:    value,
		})
	}

	append1D("CosLSP", int16Array65ToInt64(tables.CosLSP))
	append1D("PitchInterpFIR", int16Array31ToInt64(tables.PitchInterpFIR))
	append1D("Pow2Table", int16Array33ToInt64(tables.Pow2Table))
	append1D("Log2Table", int16Array33ToInt64(tables.Log2Table))
	append2D("GainGBK1", int16Array8x2ToInt64(tables.GainGBK1))
	append2D("GainGBK2", int16Array16x2ToInt64(tables.GainGBK2))
	append1D("GainMap1", uint8Array8ToInt64(tables.GainMap1))
	append1D("GainImap1", uint8Array8ToInt64(tables.GainImap1))
	append1D("GainMap2", uint8Array16ToInt64(tables.GainMap2))
	append1D("GainImap2", uint8Array16ToInt64(tables.GainImap2))
	append1D("GainMAPredictor", int16Array4ToInt64(tables.GainMAPredictor))
	appendScalar("GainMeanEnergyQ10", int64(tables.GainMeanEnergyQ10))
	appendScalar("GainPastErrorsDefaultQ10", int64(gain.PastErrorsDefault))

	return rows
}

func writeDecoderSupportTableRows(path, valueColumn string, rows []decoderSupportTableRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"table", "row", "col", valueColumn}); err != nil {
		return err
	}
	for _, row := range rows {
		value := ""
		if row.hasValue {
			value = strconv.FormatInt(row.value, 10)
		}
		if err := w.Write([]string{row.table, strconv.Itoa(row.row), strconv.Itoa(row.col), value}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func readDecoderSupportTableRows(path string) ([]decoderSupportTableRow, error) {
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
	if len(header) != 4 || header[0] != "table" || header[1] != "row" || header[2] != "col" || header[3] != "expected" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	var rows []decoderSupportTableRow
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	for i, rec := range records {
		if len(rec) != len(header) {
			return nil, fmt.Errorf("line %d has %d columns, want %d", i+2, len(rec), len(header))
		}
		rowIndex, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d row: %w", i+2, err)
		}
		col, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("line %d col: %w", i+2, err)
		}
		row := decoderSupportTableRow{
			table: rec[0],
			row:   rowIndex,
			col:   col,
		}
		if strings.TrimSpace(rec[3]) != "" {
			value, err := strconv.ParseInt(rec[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("line %d expected: %w", i+2, err)
			}
			row.hasValue = true
			row.value = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func decoderSupportTableKey(row decoderSupportTableRow) string {
	return row.table + "/" + strconv.Itoa(row.row) + "/" + strconv.Itoa(row.col)
}

func guardRootVerifierExpectedTemplate(path, valueColumn string) error {
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
	valueIndex := -1
	for i, h := range rows[0] {
		if h == valueColumn {
			valueIndex = i
			break
		}
	}
	if valueIndex < 0 {
		return fmt.Errorf("%s is missing %q column", path, valueColumn)
	}
	var filled int
	for _, row := range rows[1:] {
		if len(row) > valueIndex && strings.TrimSpace(row[valueIndex]) != "" {
			filled++
		}
	}
	if filled > 0 {
		return fmt.Errorf("refusing to overwrite verifier-filled expected template %s (%d filled cells); set G729_OVERWRITE_VERIFIER_EXPECTED=1 to regenerate from scratch", path, filled)
	}
	return nil
}

func int16Array4ToInt64(in [4]int16) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func int16Array31ToInt64(in [31]int16) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func int16Array33ToInt64(in [33]int16) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func int16Array65ToInt64(in [65]int16) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func int16Array8x2ToInt64(in [8][2]int16) [][2]int64 {
	out := make([][2]int64, len(in))
	for i, pair := range in {
		out[i] = [2]int64{int64(pair[0]), int64(pair[1])}
	}
	return out
}

func int16Array16x2ToInt64(in [16][2]int16) [][2]int64 {
	out := make([][2]int64, len(in))
	for i, pair := range in {
		out[i] = [2]int64{int64(pair[0]), int64(pair[1])}
	}
	return out
}

func uint8Array8ToInt64(in [8]uint8) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func uint8Array16ToInt64(in [16]uint8) []int64 {
	out := make([]int64, len(in))
	for i, value := range in {
		out[i] = int64(value)
	}
	return out
}

func rootPercent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}
