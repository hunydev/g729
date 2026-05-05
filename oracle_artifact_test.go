package g729

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type oracleRow struct {
	Vector   string `json:"vector"`
	Frame    int    `json:"frame"`
	Subframe int    `json:"subframe"`
	Field    string `json:"field"`
	Expected int    `json:"expected"`
	Got      int    `json:"got"`
	Delta    int    `json:"delta"`
	Notes    string `json:"notes"`
}

var oracleNotes = map[string]bool{
	"mismatch":      true,
	"out_of_window": true,
	"range_ok":      true,
	"range_fail":    true,
	"unknown":       true,
}

var oracleForbiddenTokens = []string{
	"function",
	"variable",
	"source",
	" line ",
	"reference c",
	"bcg729",
	"ffmpeg",
	"sipro",
	"ld8a",
	"g729a",
	"g729ab",
}

func parseOracleCSV(r io.Reader) ([]oracleRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("empty CSV")
	}
	want := []string{"vector", "frame", "subframe", "field", "expected", "got", "delta", "notes"}
	if len(records[0]) != len(want) {
		return nil, fmt.Errorf("CSV header has %d fields, want %d", len(records[0]), len(want))
	}
	for i := range want {
		if records[0][i] != want[i] {
			return nil, fmt.Errorf("CSV header[%d]=%q, want %q", i, records[0][i], want[i])
		}
	}
	rows := make([]oracleRow, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) != len(want) {
			return nil, fmt.Errorf("CSV data row %d has %d fields, want %d", i+2, len(rec), len(want))
		}
		row, err := oracleRowFromStrings(rec)
		if err != nil {
			return nil, fmt.Errorf("CSV row %d: %w", i+2, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseOracleJSONL(r io.Reader) ([]oracleRow, error) {
	var rows []oracleRow
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row oracleRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("JSONL line %d: %w", lineNo, err)
		}
		if err := validateOracleRow(row); err != nil {
			return nil, fmt.Errorf("JSONL line %d: %w", lineNo, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty JSONL")
	}
	return rows, nil
}

func oracleRowFromStrings(fields []string) (oracleRow, error) {
	var row oracleRow
	row.Vector = fields[0]
	row.Field = fields[3]
	row.Notes = fields[7]
	var err error
	if row.Frame, err = strconv.Atoi(fields[1]); err != nil {
		return row, fmt.Errorf("frame: %w", err)
	}
	if row.Subframe, err = strconv.Atoi(fields[2]); err != nil {
		return row, fmt.Errorf("subframe: %w", err)
	}
	if row.Expected, err = strconv.Atoi(fields[4]); err != nil {
		return row, fmt.Errorf("expected: %w", err)
	}
	if row.Got, err = strconv.Atoi(fields[5]); err != nil {
		return row, fmt.Errorf("got: %w", err)
	}
	if row.Delta, err = strconv.Atoi(fields[6]); err != nil {
		return row, fmt.Errorf("delta: %w", err)
	}
	if err := validateOracleRow(row); err != nil {
		return row, err
	}
	return row, nil
}

func validateOracleRow(row oracleRow) error {
	if row.Vector == "" {
		return errors.New("vector is empty")
	}
	if row.Frame < 0 {
		return fmt.Errorf("frame=%d, want >=0", row.Frame)
	}
	if row.Subframe != -1 && row.Subframe != 0 && row.Subframe != 1 {
		return fmt.Errorf("subframe=%d, want -1/0/1", row.Subframe)
	}
	if row.Field == "" {
		return errors.New("field is empty")
	}
	if row.Delta != row.Got-row.Expected {
		return fmt.Errorf("delta=%d, want got-expected=%d", row.Delta, row.Got-row.Expected)
	}
	if !oracleNotes[row.Notes] {
		return fmt.Errorf("notes=%q outside controlled vocabulary", row.Notes)
	}
	return nil
}

func validateOracleRawText(path string, data []byte) error {
	text := " " + strings.ToLower(string(data)) + " "
	for _, token := range oracleForbiddenTokens {
		if strings.Contains(text, token) {
			return fmt.Errorf("%s contains forbidden token %q", path, token)
		}
	}
	return nil
}

func parseOracleFile(path string) ([]oracleRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateOracleRawText(path, data); err != nil {
		return nil, err
	}
	switch filepath.Ext(path) {
	case ".csv":
		return parseOracleCSV(strings.NewReader(string(data)))
	case ".jsonl":
		return parseOracleJSONL(strings.NewReader(string(data)))
	default:
		return nil, fmt.Errorf("unsupported oracle artifact extension %q", filepath.Ext(path))
	}
}

func oracleArtifactPaths(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("testdata/oracle")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("testdata/oracle is absent")
		}
		t.Fatalf("read testdata/oracle: %v", err)
	}
	var paths []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		ext := filepath.Ext(ent.Name())
		if ext == ".csv" || ext == ".jsonl" {
			paths = append(paths, filepath.Join("testdata/oracle", ent.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		t.Skip("no optional oracle artifacts present")
	}
	return paths
}

func summarizeOracleRows(rows []oracleRow) oracleSummary {
	s := oracleSummary{
		Fields:        map[string]*oracleFieldSummary{},
		DeltaHist:     map[int]int{},
		WindowHits:    map[int]int{},
		RangeMismatch: map[string]int{},
	}
	windows := []int{1, 2, 5, 10}
	for _, row := range rows {
		s.Total++
		if row.Delta == 0 {
			s.Exact++
		}
		fs := s.Fields[row.Field]
		if fs == nil {
			fs = &oracleFieldSummary{}
			s.Fields[row.Field] = fs
		}
		fs.Total++
		if row.Delta == 0 {
			fs.Exact++
		}
		s.DeltaHist[row.Delta]++
		abs := row.Delta
		if abs < 0 {
			abs = -abs
		}
		for _, w := range windows {
			if abs <= w {
				s.WindowHits[w]++
			}
		}
		if row.Delta != 0 && len(s.FirstMismatches) < 8 {
			s.FirstMismatches = append(s.FirstMismatches, row)
		}
		if row.Field == "top_open_loop" && row.Delta != 0 {
			s.RangeMismatch[pitchRangeName(row.Expected)]++
		}
	}
	return s
}

type oracleSummary struct {
	Total           int
	Exact           int
	Fields          map[string]*oracleFieldSummary
	DeltaHist       map[int]int
	WindowHits      map[int]int
	FirstMismatches []oracleRow
	RangeMismatch   map[string]int
}

type oracleFieldSummary struct {
	Total int
	Exact int
}

func pitchRangeName(lag int) string {
	switch {
	case lag >= 20 && lag <= 39:
		return "20..39"
	case lag >= 40 && lag <= 79:
		return "40..79"
	case lag >= 80 && lag <= 143:
		return "80..143"
	default:
		return "out_of_range"
	}
}

func logOracleSummary(t *testing.T, label string, rows []oracleRow) {
	t.Helper()
	s := summarizeOracleRows(rows)
	if s.Total == 0 {
		t.Logf("%s: no rows", label)
		return
	}
	t.Logf("%s: exact %d/%d %.2f%%", label, s.Exact, s.Total, 100*float64(s.Exact)/float64(s.Total))
	fieldNames := make([]string, 0, len(s.Fields))
	for name := range s.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	for _, name := range fieldNames {
		fs := s.Fields[name]
		t.Logf("%s field %s: exact %d/%d %.2f%%", label, name, fs.Exact, fs.Total, 100*float64(fs.Exact)/float64(fs.Total))
	}
	for _, w := range []int{1, 2, 5, 10} {
		t.Logf("%s window ±%d: %d/%d %.2f%%", label, w, s.WindowHits[w], s.Total, 100*float64(s.WindowHits[w])/float64(s.Total))
	}
	deltas := make([]int, 0, len(s.DeltaHist))
	for d := range s.DeltaHist {
		deltas = append(deltas, d)
	}
	sort.Ints(deltas)
	var hist strings.Builder
	for _, d := range deltas {
		c := s.DeltaHist[d]
		if c*100 >= s.Total {
			hist.WriteString(fmt.Sprintf(" Δ=%+d:%d(%.1f%%)", d, c, 100*float64(c)/float64(s.Total)))
		}
	}
	t.Logf("%s delta histogram (buckets >=1%%):%s", label, hist.String())
	for i, row := range s.FirstMismatches {
		t.Logf("%s mismatch[%d]: vector=%s frame=%d subframe=%d field=%s expected=%d got=%d delta=%+d notes=%s",
			label, i, row.Vector, row.Frame, row.Subframe, row.Field, row.Expected, row.Got, row.Delta, row.Notes)
	}
	if len(s.RangeMismatch) > 0 {
		ranges := make([]string, 0, len(s.RangeMismatch))
		for r := range s.RangeMismatch {
			ranges = append(ranges, r)
		}
		sort.Strings(ranges)
		for _, r := range ranges {
			t.Logf("%s top_open_loop mismatches in expected range %s: %d", label, r, s.RangeMismatch[r])
		}
	}
}

func TestOracleArtifacts_ParserAndSummaryFixtures(t *testing.T) {
	const fixture = `vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
PITCH,1,-1,top_open_loop,82,85,3,mismatch
PITCH,2,0,P1,120,118,-2,mismatch
`
	rows, err := parseOracleCSV(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("parse fixture CSV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	s := summarizeOracleRows(rows)
	if s.Exact != 1 || s.WindowHits[2] != 2 || s.WindowHits[5] != 3 {
		t.Fatalf("summary exact=%d ±2=%d ±5=%d, want 1/2/3", s.Exact, s.WindowHits[2], s.WindowHits[5])
	}

	const jsonl = `{"vector":"PITCH","frame":0,"subframe":-1,"field":"top_open_loop","expected":74,"got":74,"delta":0,"notes":"range_ok"}
{"vector":"PITCH","frame":1,"subframe":-1,"field":"top_open_loop","expected":82,"got":85,"delta":3,"notes":"mismatch"}`
	rows, err = parseOracleJSONL(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("parse fixture JSONL: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("JSONL rows=%d, want 2", len(rows))
	}
}

func TestOracleArtifacts_RejectUnsafeFixtures(t *testing.T) {
	badSchema := `vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,75,0,mismatch
`
	if _, err := parseOracleCSV(strings.NewReader(badSchema)); err == nil {
		t.Fatal("parseOracleCSV accepted wrong delta")
	}
	badNote := `vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,75,1,branch_hint
`
	if _, err := parseOracleCSV(strings.NewReader(badNote)); err == nil {
		t.Fatal("parseOracleCSV accepted uncontrolled note")
	}
	if err := validateOracleRawText("fixture.csv", []byte("vector,frame,subframe,field,expected,got,delta,notes\nPITCH,0,-1,top_open_loop,1,2,1,function\n")); err == nil {
		t.Fatal("validateOracleRawText accepted forbidden token")
	}
}

func TestOracleArtifacts_ValidateOptionalFiles(t *testing.T) {
	for _, path := range oracleArtifactPaths(t) {
		rows, err := parseOracleFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		logOracleSummary(t, path, rows)
	}
}

func TestOracleHCenter_TopOpenLoopOptionalDiagnostic(t *testing.T) {
	paths := oracleArtifactPaths(t)
	var hcenter []oracleRow
	for _, path := range paths {
		rows, err := parseOracleFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, row := range rows {
			if row.Vector == "PITCH" && row.Field == "top_open_loop" {
				hcenter = append(hcenter, row)
			}
		}
	}
	if len(hcenter) == 0 {
		t.Skip("no PITCH/top_open_loop oracle rows present")
	}
	logOracleSummary(t, "PITCH top_open_loop", hcenter)
}
