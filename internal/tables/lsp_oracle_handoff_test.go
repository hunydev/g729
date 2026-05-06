package tables

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestOracleHandoff_WriteLSPTableHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_LSP_TABLE_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_LSP_TABLE_HANDOFF=1 to refresh LSP table handoff files")
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}

	expectedPath := filepath.Join(dir, "lsp_tables_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writeLSPTableGot(filepath.Join(dir, "lsp_tables_got.csv")); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeLSPTableExpectedTemplate(expectedPath); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_CompareLSPTableHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_LSP_TABLE_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_LSP_TABLE_HANDOFF=1 after verifier fills lsp_tables_expected_template.csv")
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	gotPath := filepath.Join(dir, "lsp_tables_got.csv")
	expectedPath := filepath.Join(dir, "lsp_tables_expected_template.csv")

	got, err := readLSPTableHandoffValues(gotPath, "got")
	if err != nil {
		t.Fatalf("read got handoff: %v", err)
	}
	expected, blanks, err := readLSPTableExpectedValues(expectedPath)
	if err != nil {
		t.Fatalf("read expected handoff: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells; fill %s first", expectedPath)
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF") == "1" && blanks > 0 {
		t.Fatalf("expected handoff still has %d blank cells", blanks)
	}

	var exact, mismatch int
	var first []string
	for key, exp := range expected {
		gotVal, ok := got[key]
		if !ok {
			t.Fatalf("expected key missing from got handoff: %s", key)
		}
		if gotVal == exp {
			exact++
			continue
		}
		mismatch++
		if len(first) < 12 {
			first = append(first, fmt.Sprintf("%s expected=%d got=%d delta=%+d", key, exp, gotVal, gotVal-exp))
		}
	}

	t.Logf("LSP table handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("LSP table handoff has %d mismatches", mismatch)
	}
}

func writeLSPTableGot(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"table", "selector", "tap", "row", "col", "got"}); err != nil {
		return err
	}
	if err := forEachLSPTableScalar(func(table string, selector, tap, row, col int, value int16) error {
		return w.Write([]string{
			table,
			strconv.Itoa(selector),
			strconv.Itoa(tap),
			strconv.Itoa(row),
			strconv.Itoa(col),
			strconv.Itoa(int(value)),
		})
	}); err != nil {
		return err
	}
	return w.Error()
}

func writeLSPTableExpectedTemplate(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"table", "selector", "tap", "row", "col", "expected"}); err != nil {
		return err
	}
	if err := forEachLSPTableScalar(func(table string, selector, tap, row, col int, value int16) error {
		_ = value
		return w.Write([]string{
			table,
			strconv.Itoa(selector),
			strconv.Itoa(tap),
			strconv.Itoa(row),
			strconv.Itoa(col),
			"",
		})
	}); err != nil {
		return err
	}
	return w.Error()
}

func readLSPTableHandoffValues(path, valueColumn string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty csv")
	}
	valueIdx := -1
	for i, h := range rows[0] {
		if h == valueColumn {
			valueIdx = i
			break
		}
	}
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}

	out := make(map[string]int, len(rows)-1)
	for line, row := range rows[1:] {
		if len(row) <= valueIdx {
			return nil, fmt.Errorf("line %d: short row", line+2)
		}
		key, err := lspTableHandoffKey(row)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line+2, err)
		}
		value, err := strconv.Atoi(row[valueIdx])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse %s: %w", line+2, valueColumn, err)
		}
		out[key] = value
	}
	return out, nil
}

func readLSPTableExpectedValues(path string) (map[string]int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, fmt.Errorf("empty csv")
	}
	expectedIdx := -1
	for i, h := range rows[0] {
		if h == "expected" {
			expectedIdx = i
			break
		}
	}
	if expectedIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}

	out := make(map[string]int, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		if len(row) <= expectedIdx {
			return nil, 0, fmt.Errorf("line %d: short row", line+2)
		}
		key, err := lspTableHandoffKey(row)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: %w", line+2, err)
		}
		if row[expectedIdx] == "" {
			blanks++
			continue
		}
		value, err := strconv.Atoi(row[expectedIdx])
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: parse expected: %w", line+2, err)
		}
		out[key] = value
	}
	return out, blanks, nil
}

func lspTableHandoffKey(row []string) (string, error) {
	if len(row) < 5 {
		return "", fmt.Errorf("short key row")
	}
	return row[0] + "/" + row[1] + "/" + row[2] + "/" + row[3] + "/" + row[4], nil
}

func forEachLSPTableScalar(fn func(table string, selector, tap, row, col int, value int16) error) error {
	for row := range LSPCodebookL1 {
		for col, value := range LSPCodebookL1[row] {
			if err := fn("LSPCodebookL1", -1, -1, row, col, value); err != nil {
				return err
			}
		}
	}
	for row := range LSPCodebookL2 {
		for col, value := range LSPCodebookL2[row] {
			if err := fn("LSPCodebookL2", -1, -1, row, col, value); err != nil {
				return err
			}
		}
	}
	for row := range LSPCodebookL3 {
		for col, value := range LSPCodebookL3[row] {
			if err := fn("LSPCodebookL3", -1, -1, row, col, value); err != nil {
				return err
			}
		}
	}
	for selector := range MAPredictorsLSP {
		for tap := range MAPredictorsLSP[selector] {
			for col, value := range MAPredictorsLSP[selector][tap] {
				if err := fn("MAPredictorsLSP", selector, tap, -1, col, value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
