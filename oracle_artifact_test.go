package g729

import (
	"bufio"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/lsp"
	pitchidx "github.com/hunydev/g729/internal/pitch"
	clpitch "github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/tables"
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

type oracleRangeStats struct {
	total int
	exact int
	w1    int
	w2    int
	w5    int
	w10   int
}

func logTopOpenLoopRangeStats(t *testing.T, label string, rows []oracleRow) {
	t.Helper()
	stats := map[string]*oracleRangeStats{}
	for _, row := range rows {
		if row.Field != "top_open_loop" {
			continue
		}
		name := pitchRangeName(row.Expected)
		st := stats[name]
		if st == nil {
			st = &oracleRangeStats{}
			stats[name] = st
		}
		st.total++
		abs := row.Delta
		if abs < 0 {
			abs = -abs
		}
		if abs == 0 {
			st.exact++
		}
		if abs <= 1 {
			st.w1++
		}
		if abs <= 2 {
			st.w2++
		}
		if abs <= 5 {
			st.w5++
		}
		if abs <= 10 {
			st.w10++
		}
	}
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		st := stats[name]
		t.Logf("%s expected range %s: exact %d/%d %.2f%% ±1 %d %.2f%% ±2 %d %.2f%% ±5 %d %.2f%% ±10 %d %.2f%%",
			label, name,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w1, 100*float64(st.w1)/float64(st.total),
			st.w2, 100*float64(st.w2)/float64(st.total),
			st.w5, 100*float64(st.w5)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total))
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
	logTopOpenLoopRangeStats(t, "PITCH top_open_loop", hcenter)
}

func TestOracleHCenter_TopOpenLoopExactGate(t *testing.T) {
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
	s := summarizeOracleRows(hcenter)
	if s.Exact*100 < 70*s.Total {
		t.Fatalf("PITCH top_open_loop exact %d/%d %.2f%%, want >=70.00%%",
			s.Exact, s.Total, 100*float64(s.Exact)/float64(s.Total))
	}
}

func TestOracleHCenter_WriteTopOpenLoopHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_ORACLE_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_ORACLE_HANDOFF=1 to refresh verifier handoff files")
	}

	const (
		inPath       = "testdata/phase2b/hcenter_top_vs_t1.csv"
		outDir       = "testdata/oracle/handoff"
		gotPath      = outDir + "/pitch_top_open_loop_got.csv"
		templatePath = outDir + "/pitch_top_open_loop_expected_template.csv"
		readmePath   = outDir + "/README.md"
	)

	in, err := os.Open(inPath)
	if err != nil {
		t.Fatalf("open %s: %v", inPath, err)
	}
	defer in.Close()

	cr := csv.NewReader(in)
	records, err := cr.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has %d rows, want header plus data", inPath, len(records))
	}
	wantHeader := []string{"frame", "t_op", "int_t1", "delta", "plausible"}
	if len(records[0]) != len(wantHeader) {
		t.Fatalf("%s header width=%d, want %d", inPath, len(records[0]), len(wantHeader))
	}
	for i := range wantHeader {
		if records[0][i] != wantHeader[i] {
			t.Fatalf("%s header[%d]=%q, want %q", inPath, i, records[0][i], wantHeader[i])
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}
	if err := guardVerifierExpectedTemplate(templatePath, "expected_top_open_loop"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}

	var got, tmpl strings.Builder
	got.WriteString("vector,frame,subframe,field,got\n")
	tmpl.WriteString("frame,expected_top_open_loop\n")
	for i, rec := range records[1:] {
		if len(rec) != len(wantHeader) {
			t.Fatalf("%s row %d width=%d, want %d", inPath, i+2, len(rec), len(wantHeader))
		}
		frame, err := strconv.Atoi(rec[0])
		if err != nil {
			t.Fatalf("%s row %d frame: %v", inPath, i+2, err)
		}
		top, err := strconv.Atoi(rec[1])
		if err != nil {
			t.Fatalf("%s row %d t_op: %v", inPath, i+2, err)
		}
		got.WriteString(fmt.Sprintf("PITCH,%d,-1,top_open_loop,%d\n", frame, top))
		tmpl.WriteString(fmt.Sprintf("%d,\n", frame))
	}
	if err := os.WriteFile(gotPath, []byte(got.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", gotPath, err)
	}
	if err := os.WriteFile(templatePath, []byte(tmpl.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", templatePath, err)
	}
	const readme = `# H-CENTER Oracle Handoff

These files are not oracle artifacts and are intentionally ignored by the optional oracle validator because they live in a subdirectory.

- ` + "`pitch_top_open_loop_got.csv`" + `: this implementation's frame-level open-loop ` + "`T_op`" + ` values.
- ` + "`pitch_top_open_loop_expected_template.csv`" + `: verifier-owned template for raw oracle ` + "`T_op`" + ` values.

Verifier workflow:

1. Fill ` + "`expected_top_open_loop`" + ` for every frame in the template.
2. Produce a clean-room oracle artifact at ` + "`testdata/oracle/pitch_top_open_loop.csv`" + ` with:

   ` + "```csv" + `
   vector,frame,subframe,field,expected,got,delta,notes
   PITCH,0,-1,top_open_loop,<expected>,<got>,<got-expected>,mismatch
   ` + "```" + `

3. Use only controlled notes: ` + "`mismatch`" + `, ` + "`out_of_window`" + `, ` + "`range_ok`" + `, ` + "`range_fail`" + `, or ` + "`unknown`" + `.
4. Do not include implementation details, code names, source locations, or explanations for oracle internals.
`
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		t.Fatalf("write %s: %v", readmePath, err)
	}
	t.Logf("wrote %s, %s, %s", gotPath, templatePath, readmePath)
}

func mergeTopOpenLoopArtifact(gotCSV, expectedCSV io.Reader) ([]oracleRow, error) {
	gotReader := csv.NewReader(gotCSV)
	gotRecords, err := gotReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(gotRecords) < 2 {
		return nil, errors.New("got CSV has no data rows")
	}
	gotHeader := []string{"vector", "frame", "subframe", "field", "got"}
	if err := validateCSVHeader(gotRecords[0], gotHeader); err != nil {
		return nil, fmt.Errorf("got CSV: %w", err)
	}

	expectedReader := csv.NewReader(expectedCSV)
	expectedRecords, err := expectedReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(expectedRecords) < 2 {
		return nil, errors.New("expected CSV has no data rows")
	}
	expectedHeader := []string{"frame", "expected_top_open_loop"}
	if err := validateCSVHeader(expectedRecords[0], expectedHeader); err != nil {
		return nil, fmt.Errorf("expected CSV: %w", err)
	}

	expectedByFrame := make(map[int]int, len(expectedRecords)-1)
	for i, rec := range expectedRecords[1:] {
		if len(rec) != len(expectedHeader) {
			return nil, fmt.Errorf("expected CSV row %d has %d fields, want %d", i+2, len(rec), len(expectedHeader))
		}
		frame, err := strconv.Atoi(rec[0])
		if err != nil {
			return nil, fmt.Errorf("expected CSV row %d frame: %w", i+2, err)
		}
		if strings.TrimSpace(rec[1]) == "" {
			return nil, fmt.Errorf("expected CSV row %d frame %d has blank expected_top_open_loop", i+2, frame)
		}
		expected, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("expected CSV row %d expected_top_open_loop: %w", i+2, err)
		}
		if _, dup := expectedByFrame[frame]; dup {
			return nil, fmt.Errorf("expected CSV duplicate frame %d", frame)
		}
		expectedByFrame[frame] = expected
	}

	rows := make([]oracleRow, 0, len(gotRecords)-1)
	seenGot := map[int]bool{}
	for i, rec := range gotRecords[1:] {
		if len(rec) != len(gotHeader) {
			return nil, fmt.Errorf("got CSV row %d has %d fields, want %d", i+2, len(rec), len(gotHeader))
		}
		frame, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("got CSV row %d frame: %w", i+2, err)
		}
		if seenGot[frame] {
			return nil, fmt.Errorf("got CSV duplicate frame %d", frame)
		}
		seenGot[frame] = true
		expected, ok := expectedByFrame[frame]
		if !ok {
			return nil, fmt.Errorf("missing expected row for frame %d", frame)
		}
		subframe, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("got CSV row %d subframe: %w", i+2, err)
		}
		got, err := strconv.Atoi(rec[4])
		if err != nil {
			return nil, fmt.Errorf("got CSV row %d got: %w", i+2, err)
		}
		row := oracleRow{
			Vector:   rec[0],
			Frame:    frame,
			Subframe: subframe,
			Field:    rec[3],
			Expected: expected,
			Got:      got,
			Delta:    got - expected,
			Notes:    "mismatch",
		}
		if expected < 20 || expected > 143 || got < 20 || got > 143 {
			row.Notes = "range_fail"
		} else if row.Delta == 0 {
			row.Notes = "range_ok"
		}
		if err := validateOracleRow(row); err != nil {
			return nil, fmt.Errorf("merged frame %d: %w", frame, err)
		}
		rows = append(rows, row)
	}
	if len(rows) != len(expectedByFrame) {
		return nil, fmt.Errorf("row count mismatch: got rows=%d expected rows=%d", len(rows), len(expectedByFrame))
	}
	return rows, nil
}

func validateCSVHeader(got, want []string) error {
	if len(got) != len(want) {
		return fmt.Errorf("header has %d fields, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("header[%d]=%q, want %q", i, got[i], want[i])
		}
	}
	return nil
}

func writeOracleRowsCSV(rows []oracleRow) string {
	var b strings.Builder
	b.WriteString("vector,frame,subframe,field,expected,got,delta,notes\n")
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("%s,%d,%d,%s,%d,%d,%d,%s\n",
			row.Vector, row.Frame, row.Subframe, row.Field,
			row.Expected, row.Got, row.Delta, row.Notes))
	}
	return b.String()
}

func TestOracleHCenter_MergeTopOpenLoopHandoffFixtures(t *testing.T) {
	const got = `vector,frame,subframe,field,got
PITCH,0,-1,top_open_loop,74
PITCH,1,-1,top_open_loop,85
`
	const expected = `frame,expected_top_open_loop
0,74
1,82
`
	rows, err := mergeTopOpenLoopArtifact(strings.NewReader(got), strings.NewReader(expected))
	if err != nil {
		t.Fatalf("mergeTopOpenLoopArtifact: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d, want 2", len(rows))
	}
	if rows[0].Notes != "range_ok" || rows[0].Delta != 0 {
		t.Fatalf("row0 notes=%s delta=%d, want range_ok/0", rows[0].Notes, rows[0].Delta)
	}
	if rows[1].Notes != "mismatch" || rows[1].Delta != 3 {
		t.Fatalf("row1 notes=%s delta=%d, want mismatch/+3", rows[1].Notes, rows[1].Delta)
	}
	if _, err := parseOracleCSV(strings.NewReader(writeOracleRowsCSV(rows))); err != nil {
		t.Fatalf("merged CSV does not validate as oracle artifact: %v", err)
	}

	const blankExpected = `frame,expected_top_open_loop
0,
`
	if _, err := mergeTopOpenLoopArtifact(strings.NewReader(got), strings.NewReader(blankExpected)); err == nil {
		t.Fatal("mergeTopOpenLoopArtifact accepted blank expected value")
	}
}

func TestOracleHCenter_MergeTopOpenLoopHandoff(t *testing.T) {
	if os.Getenv("G729_MERGE_ORACLE_HANDOFF") != "1" {
		t.Skip("set G729_MERGE_ORACLE_HANDOFF=1 after verifier fills the expected template")
	}

	const (
		gotPath      = "testdata/oracle/handoff/pitch_top_open_loop_got.csv"
		expectedPath = "testdata/oracle/handoff/pitch_top_open_loop_expected_template.csv"
		outPath      = "testdata/oracle/pitch_top_open_loop.csv"
	)

	gotData, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read %s: %v", gotPath, err)
	}
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read %s: %v", expectedPath, err)
	}
	rows, err := mergeTopOpenLoopArtifact(strings.NewReader(string(gotData)), strings.NewReader(string(expectedData)))
	if err != nil {
		t.Fatalf("merge handoff: %v", err)
	}
	out := writeOracleRowsCSV(rows)
	if err := validateOracleRawText(outPath, []byte(out)); err != nil {
		t.Fatalf("validate merged raw text: %v", err)
	}
	if _, err := parseOracleCSV(strings.NewReader(out)); err != nil {
		t.Fatalf("validate merged CSV: %v", err)
	}
	if err := os.WriteFile(outPath, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", outPath, err)
	}
	t.Logf("wrote %s with %d rows", outPath, len(rows))
}

func TestOracleHCenter_RefreshTopOpenLoopArtifactGot(t *testing.T) {
	if os.Getenv("G729_REFRESH_ORACLE_GOT") != "1" {
		t.Skip("set G729_REFRESH_ORACLE_GOT=1 after production T_op changes")
	}

	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	if len(oracleByFrame) != totalFrames {
		t.Fatalf("oracle rows=%d, want %d", len(oracleByFrame), totalFrames)
	}

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	refreshed := make([]oracleRow, 0, totalFrames)
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		got := int(enc.openloopStep())
		row, ok := oracleByFrame[f]
		if !ok {
			t.Fatalf("missing oracle row for frame %d", f)
		}
		row.Got = got
		row.Delta = got - row.Expected
		row.Notes = "mismatch"
		if row.Expected < 20 || row.Expected > 143 || row.Got < 20 || row.Got > 143 {
			row.Notes = "range_fail"
		} else if row.Delta == 0 {
			row.Notes = "range_ok"
		}
		if err := validateOracleRow(row); err != nil {
			t.Fatalf("frame %d refreshed row: %v", f, err)
		}
		refreshed = append(refreshed, row)
	}
	out := writeOracleRowsCSV(refreshed)
	if _, err := parseOracleCSV(strings.NewReader(out)); err != nil {
		t.Fatalf("validate refreshed CSV: %v", err)
	}
	if err := os.WriteFile(oraclePath, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", oraclePath, err)
	}
	t.Logf("refreshed %s with %d rows", oraclePath, len(refreshed))
}

type openLoopRangeDiag struct {
	lag int
	r   fixed.Word32
	e   fixed.Word32
}

type openLoopFrameDiag struct {
	range1 openLoopRangeDiag
	range2 openLoopRangeDiag
	range3 openLoopRangeDiag
}

type openLoopRangeDiag64 struct {
	lag int
	r   int64
	e   int64
}

type openLoopFrameDiag64 struct {
	range1 openLoopRangeDiag64
	range2 openLoopRangeDiag64
	range3 openLoopRangeDiag64
}

type oracleOpenLoopState64 struct {
	lpResidualMem [10]int16
	swMem         [10]int16
	oldWspeech    [143]int16
}

func diagnoseOpenLoopFrame(e *Encoder) openLoopFrameDiag {
	s := (*[FrameSamples]int16)(e.oldSpeech[120:200])

	var aw, aPrime [11]int16
	oracleGammaWeightLP(&e.aQ12Latest, &aw)
	oracleCombineWith07(&aw, &aPrime)

	var residual, freshSw [80]int16
	oracleLPResidual(s, &e.aQ12Latest, &e.lpResidualMem, &residual)
	oracleLowpassWeightedSpeech(&residual, &aPrime, &e.swMem, &freshSw)

	var wsp [223]int16
	copy(wsp[:143], e.oldWspeech[:])
	copy(wsp[143:], freshSw[:])

	return openLoopFrameDiag{
		range1: oraclePickBest(&wsp, 20, 39),
		range2: oraclePickBest(&wsp, 40, 79),
		range3: oraclePickBest(&wsp, 80, 143),
	}
}

func diagnoseOpenLoopFrame64(e *Encoder) openLoopFrameDiag64 {
	return diagnoseOpenLoopFrame64WithA(e, &e.aQ12Latest)
}

func diagnoseOpenLoopFrame64WithA(e *Encoder, a *[11]int16) openLoopFrameDiag64 {
	s := (*[FrameSamples]int16)(e.oldSpeech[120:200])
	var state oracleOpenLoopState64
	state.lpResidualMem = e.lpResidualMem
	state.swMem = e.swMem
	state.oldWspeech = e.oldWspeech
	return diagnoseOpenLoopFrame64WithState(s, a, &state)
}

func diagnoseOpenLoopFrame64WithState(s *[FrameSamples]int16, a *[11]int16, state *oracleOpenLoopState64) openLoopFrameDiag64 {
	var aw, aPrime [11]int16
	oracleGammaWeightLP(a, &aw)
	oracleCombineWith07(&aw, &aPrime)

	var residual, freshSw [80]int16
	oracleLPResidual(s, a, &state.lpResidualMem, &residual)
	oracleLowpassWeightedSpeech(&residual, &aPrime, &state.swMem, &freshSw)

	var wsp [223]int16
	copy(wsp[:143], state.oldWspeech[:])
	copy(wsp[143:], freshSw[:])

	return openLoopFrameDiag64{
		range1: oraclePickBest64(&wsp, 20, 39),
		range2: oraclePickBest64(&wsp, 40, 79),
		range3: oraclePickBest64(&wsp, 80, 143),
	}
}

func oracleAdvanceOpenLoopState64(s *[FrameSamples]int16, a *[11]int16, state *oracleOpenLoopState64) openLoopFrameDiag64 {
	var aw, aPrime [11]int16
	oracleGammaWeightLP(a, &aw)
	oracleCombineWith07(&aw, &aPrime)

	var residual, freshSw [80]int16
	oracleLPResidual(s, a, &state.lpResidualMem, &residual)
	oracleLowpassWeightedSpeech(&residual, &aPrime, &state.swMem, &freshSw)

	var wsp [223]int16
	copy(wsp[:143], state.oldWspeech[:])
	copy(wsp[143:], freshSw[:])

	copy(state.lpResidualMem[:], s[70:80])
	copy(state.swMem[:], freshSw[70:80])
	copy(state.oldWspeech[0:63], state.oldWspeech[80:143])
	copy(state.oldWspeech[63:143], freshSw[:])

	return openLoopFrameDiag64{
		range1: oraclePickBest64(&wsp, 20, 39),
		range2: oraclePickBest64(&wsp, 40, 79),
		range3: oraclePickBest64(&wsp, 80, 143),
	}
}

func oracleGammaWeightLP(a, out *[11]int16) {
	gammaPow := [11]int16{32767, 24576, 18432, 13824, 10368, 7776, 5832, 4374, 3281, 2460, 1845}
	out[0] = a[0]
	for i := 1; i <= 10; i++ {
		out[i] = fixed.Mult(a[i], gammaPow[i])
	}
}

func oracleCombineWith07(aw, out *[11]int16) {
	const gamma07Q15 int16 = 22938
	out[0] = aw[0]
	out[1] = fixed.Saturate(int32(aw[1]) - int32(gamma07Q15))
	for i := 2; i <= 10; i++ {
		out[i] = aw[i] - fixed.MultR(gamma07Q15, aw[i-1])
	}
}

func oracleLPResidual(s *[80]int16, aHat *[11]int16, mem *[10]int16, r *[80]int16) {
	for n := 0; n < 80; n++ {
		acc := fixed.LMult(s[n], aHat[0])
		for i := 1; i <= 10; i++ {
			var sni int16
			if n-i >= 0 {
				sni = s[n-i]
			} else {
				sni = mem[10+n-i]
			}
			acc = fixed.LMac(acc, aHat[i], sni)
		}
		r[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func oracleLowpassWeightedSpeech(r *[80]int16, aPrime *[11]int16, mem *[10]int16, sw *[80]int16) {
	for n := 0; n < 80; n++ {
		acc := fixed.LMult(r[n], aPrime[0])
		for i := 1; i <= 10; i++ {
			var swni int16
			if n-i >= 0 {
				swni = sw[n-i]
			} else {
				swni = mem[10+n-i]
			}
			acc = fixed.LMsu(acc, aPrime[i], swni)
		}
		sw[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func oraclePickBest(wsp *[223]int16, kMin, kMax int) openLoopRangeDiag {
	if kMin == 80 && kMax == 143 {
		return oraclePickBestEvenWithRefinement(wsp)
	}
	best := openLoopRangeDiag{lag: kMax, r: oracleCorrelateAt(wsp, kMax), e: oracleEnergy(wsp, kMax)}
	for k := kMax - 1; k >= kMin; k-- {
		cand := openLoopRangeDiag{lag: k, r: oracleCorrelateAt(wsp, k), e: oracleEnergy(wsp, k)}
		if oracleCompareNormalized(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func oraclePickBestEvenWithRefinement(wsp *[223]int16) openLoopRangeDiag {
	best := openLoopRangeDiag{lag: 142, r: oracleCorrelateAt(wsp, 142), e: oracleEnergy(wsp, 142)}
	for k := 140; k >= 80; k -= 2 {
		cand := openLoopRangeDiag{lag: k, r: oracleCorrelateAt(wsp, k), e: oracleEnergy(wsp, k)}
		if oracleCompareNormalized(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	bestEven := best.lag
	hi := bestEven + 1
	if hi > 143 {
		hi = 143
	}
	lo := bestEven - 1
	if lo < 80 {
		lo = 80
	}
	best = openLoopRangeDiag{lag: hi, r: oracleCorrelateAt(wsp, hi), e: oracleEnergy(wsp, hi)}
	for k := hi - 1; k >= lo; k-- {
		cand := openLoopRangeDiag{lag: k, r: oracleCorrelateAt(wsp, k), e: oracleEnergy(wsp, k)}
		if oracleCompareNormalized(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func oracleCorrelateAt(wsp *[223]int16, k int) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		acc = fixed.LMac(acc, wsp[143+2*n], wsp[143+2*n-k])
	}
	return acc
}

func oracleEnergy(wsp *[223]int16, k int) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		s := fixed.Word32(wsp[143+2*n-k])
		acc = fixed.LAdd(acc, s*s)
	}
	return acc
}

func oracleCompareNormalized(r1In, e1, r2In, e2 fixed.Word32) bool {
	score1Zero := e1 <= 0 || r1In <= 0
	score2Zero := e2 <= 0 || r2In <= 0
	if score1Zero && score2Zero {
		return true
	}
	if score1Zero {
		return false
	}
	if score2Zero {
		return true
	}
	r1 := int64(r1In)
	r2 := int64(r2In)
	maxR := r1
	if r2 > maxR {
		maxR = r2
	}
	var s uint
	if l := bits.Len64(uint64(maxR)); l > 15 {
		s = uint(l - 15)
	}
	r1 >>= s
	r2 >>= s
	return r1*r1*int64(e2) >= r2*r2*int64(e1)
}

func oraclePickBest64(wsp *[223]int16, kMin, kMax int) openLoopRangeDiag64 {
	if kMin == 80 && kMax == 143 {
		return oraclePickBestEvenWithRefinement64(wsp)
	}
	best := openLoopRangeDiag64{lag: kMax, r: oracleCorrelateAt64(wsp, kMax), e: oracleEnergy64(wsp, kMax)}
	for k := kMax - 1; k >= kMin; k-- {
		cand := openLoopRangeDiag64{lag: k, r: oracleCorrelateAt64(wsp, k), e: oracleEnergy64(wsp, k)}
		if oracleCompareNormalized64(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func oraclePickBestEvenWithRefinement64(wsp *[223]int16) openLoopRangeDiag64 {
	best := openLoopRangeDiag64{lag: 142, r: oracleCorrelateAt64(wsp, 142), e: oracleEnergy64(wsp, 142)}
	for k := 140; k >= 80; k -= 2 {
		cand := openLoopRangeDiag64{lag: k, r: oracleCorrelateAt64(wsp, k), e: oracleEnergy64(wsp, k)}
		if oracleCompareNormalized64(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	bestEven := best.lag
	hi := bestEven + 1
	if hi > 143 {
		hi = 143
	}
	lo := bestEven - 1
	if lo < 80 {
		lo = 80
	}
	best = openLoopRangeDiag64{lag: hi, r: oracleCorrelateAt64(wsp, hi), e: oracleEnergy64(wsp, hi)}
	for k := hi - 1; k >= lo; k-- {
		cand := openLoopRangeDiag64{lag: k, r: oracleCorrelateAt64(wsp, k), e: oracleEnergy64(wsp, k)}
		if oracleCompareNormalized64(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func oracleCorrelateAt64(wsp *[223]int16, k int) int64 {
	var acc int64
	for n := 0; n < 40; n++ {
		acc += 2 * int64(wsp[143+2*n]) * int64(wsp[143+2*n-k])
	}
	return acc
}

func oracleEnergy64(wsp *[223]int16, k int) int64 {
	var acc int64
	for n := 0; n < 40; n++ {
		s := int64(wsp[143+2*n-k])
		acc += s * s
	}
	return acc
}

func oracleCompareNormalized64(r1, e1, r2, e2 int64) bool {
	score1Zero := e1 <= 0 || r1 <= 0
	score2Zero := e2 <= 0 || r2 <= 0
	if score1Zero && score2Zero {
		return true
	}
	if score1Zero {
		return false
	}
	if score2Zero {
		return true
	}
	return (float64(r1)*float64(r1))/float64(e1) >= (float64(r2)*float64(r2))/float64(e2)
}

func oracleMergeDiag(d openLoopFrameDiag) int {
	best := d.range1
	if oracleShouldOverride(d.range2, best) {
		best = d.range2
	}
	if oracleShouldOverride(d.range3, best) {
		best = d.range3
	}
	return best.lag
}

func oracleMergeNoHighDiag(d openLoopFrameDiag) int {
	best := d.range1
	if oracleShouldOverride(d.range2, best) {
		best = d.range2
	}
	return best.lag
}

func oracleShouldOverride(h, op openLoopRangeDiag) bool {
	if oracleIsNearSubmultiple(h.lag, op.lag) {
		return oracleLiftedStrictGreater(h.r, h.e, op.r, op.e)
	}
	return !oracleCompareNormalized(op.r, op.e, h.r, h.e)
}

func oracleIsNearSubmultiple(higher, lower int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= 2 {
			return true
		}
		if k*lower > higher+2 {
			return false
		}
	}
	return false
}

func oracleLiftedStrictGreater(rH, eH, rOp, eOp fixed.Word32) bool {
	if eH <= 0 || rH <= 0 {
		return false
	}
	if eOp <= 0 || rOp <= 0 {
		return true
	}
	rh := int64(rH)
	ro := int64(rOp)
	maxR := rh
	if ro > maxR {
		maxR = ro
	}
	var s uint
	if l := bits.Len64(uint64(maxR)); l > 13 {
		s = uint(l - 13)
	}
	rh >>= s
	ro >>= s
	return rh*rh*int64(eOp) > ro*ro*int64(eH)*2
}

func oracleMergeDiag64(d openLoopFrameDiag64) int {
	best := d.range1
	if oracleShouldOverride64(d.range2, best) {
		best = d.range2
	}
	if oracleShouldOverride64(d.range3, best) {
		best = d.range3
	}
	return best.lag
}

func oracleMergeDiag64WithMargin(d openLoopFrameDiag64, margin float64) int {
	best := d.range1
	if oracleShouldOverride64WithMargin(d.range2, best, margin) {
		best = d.range2
	}
	if oracleShouldOverride64WithMargin(d.range3, best, margin) {
		best = d.range3
	}
	return best.lag
}

func oracleMergeDiag64WithTunedLift(d openLoopFrameDiag64, lift, margin float64, tol int) int {
	best := d.range1
	if oracleShouldOverride64WithTunedLift(d.range2, best, lift, margin, tol) {
		best = d.range2
	}
	if oracleShouldOverride64WithTunedLift(d.range3, best, lift, margin, tol) {
		best = d.range3
	}
	return best.lag
}

func oracleMergeDiag64WithRangeMargins(d openLoopFrameDiag64, lift, margin2, margin3 float64, tol int) int {
	best := d.range1
	if oracleShouldOverride64WithTunedLift(d.range2, best, lift, margin2, tol) {
		best = d.range2
	}
	if oracleShouldOverride64WithTunedLift(d.range3, best, lift, margin3, tol) {
		best = d.range3
	}
	return best.lag
}

func oracleShouldOverride64(h, op openLoopRangeDiag64) bool {
	return oracleShouldOverride64WithMargin(h, op, 1.0)
}

func oracleShouldOverride64WithMargin(h, op openLoopRangeDiag64, margin float64) bool {
	if oracleIsNearSubmultiple(h.lag, op.lag) {
		if h.e <= 0 || h.r <= 0 {
			return false
		}
		if op.e <= 0 || op.r <= 0 {
			return true
		}
		return (float64(h.r)*float64(h.r))/float64(h.e) >
			2*(float64(op.r)*float64(op.r))/float64(op.e)
	}
	if h.e <= 0 || h.r <= 0 {
		return false
	}
	if op.e <= 0 || op.r <= 0 {
		return true
	}
	return (float64(h.r)*float64(h.r))/float64(h.e) >
		margin*(float64(op.r)*float64(op.r))/float64(op.e)
}

func oracleShouldOverride64WithTunedLift(h, op openLoopRangeDiag64, lift, margin float64, tol int) bool {
	if oracleIsNearSubmultipleTol(h.lag, op.lag, tol) {
		if h.e <= 0 || h.r <= 0 {
			return false
		}
		if op.e <= 0 || op.r <= 0 {
			return true
		}
		return (float64(h.r)*float64(h.r))/float64(h.e) >
			lift*(float64(op.r)*float64(op.r))/float64(op.e)
	}
	if h.e <= 0 || h.r <= 0 {
		return false
	}
	if op.e <= 0 || op.r <= 0 {
		return true
	}
	return (float64(h.r)*float64(h.r))/float64(h.e) >
		margin*(float64(op.r)*float64(op.r))/float64(op.e)
}

func oracleIsNearSubmultipleTol(higher, lower, tol int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= tol {
			return true
		}
		if k*lower > higher+tol {
			return false
		}
	}
	return false
}

type oracleVariantStats struct {
	total    int
	exact    int
	w10      int
	lowTotal int
	lowExact int
	lowW10   int
}

func (s *oracleVariantStats) add(expected, got int) {
	s.total++
	d := got - expected
	abs := d
	if abs < 0 {
		abs = -abs
	}
	if abs == 0 {
		s.exact++
	}
	if abs <= 10 {
		s.w10++
	}
	if expected >= 20 && expected <= 39 {
		s.lowTotal++
		if abs == 0 {
			s.lowExact++
		}
		if abs <= 10 {
			s.lowW10++
		}
	}
}

type oracleP1VariantStats struct {
	oracleVariantStats
	p1Hits int
}

func (s *oracleP1VariantStats) addP1(top, intT1 int) {
	if intT1 >= top-5 && intT1 <= top+4 {
		s.p1Hits++
	}
}

func TestOracleHCenter_TunedLiftSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	type candidate struct {
		lift   float64
		margin float64
		tol    int
	}
	var candidates []candidate
	for _, lift := range []float64{1.25, 1.50, 1.75, 2.00, 2.25, 2.50, 3.00, 4.00, 6.00, 8.00} {
		for _, margin := range []float64{0.70, 0.75, 0.80, 0.85, 0.90, 0.95, 1.00, 1.02, 1.05, 1.08, 1.10, 1.15, 1.20, 1.30} {
			for _, tol := range []int{0, 1, 2, 3, 4, 5} {
				candidates = append(candidates, candidate{lift: lift, margin: margin, tol: tol})
			}
		}
	}
	stats := make([]oracleVariantStats, len(candidates))

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag64 := diagnoseOpenLoopFrame64(enc)
		_ = enc.openloopStep()
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		for i, cand := range candidates {
			stats[i].add(row.Expected, oracleMergeDiag64WithTunedLift(diag64, cand.lift, cand.margin, cand.tol))
		}
	}

	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a := stats[order[i]]
		b := stats[order[j]]
		if a.exact != b.exact {
			return a.exact > b.exact
		}
		return a.w10 > b.w10
	})
	for rank := 0; rank < 12 && rank < len(order); rank++ {
		i := order[rank]
		cand := candidates[i]
		st := stats[i]
		t.Logf("rank %02d lift %.2f margin %.2f tol %d: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			rank+1, cand.lift, cand.margin, cand.tol,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_RawAndP1ConfigSweep(t *testing.T) {
	const (
		oraclePath        = "testdata/oracle/pitch_top_open_loop.csv"
		inPath            = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame   = 80
		bytesPerInFrame   = 2 * samplesPerFrame
		bytesPerBitFrame  = 164
		totalFrames       = 1835
		minRawExactPct    = 70.0
		targetP1WindowPct = 70.0
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	type candidate struct {
		lift    float64
		margin2 float64
		margin3 float64
		tol     int
	}
	margins := []float64{0.60, 0.70, 0.80, 0.85, 0.90, 0.95, 1.00, 1.05, 1.10, 1.15, 1.20, 1.30, 1.50, 2.00}
	var candidates []candidate
	for _, lift := range []float64{1.00, 1.25, 1.50, 1.75, 2.00, 2.50, 3.00, 4.00, 6.00, 8.00, 10.00, 12.00} {
		for _, margin2 := range margins {
			for _, margin3 := range margins {
				for _, tol := range []int{0, 1, 2, 3, 4, 5, 6, 8, 10, 12, 15, 20, 25, 30} {
					candidates = append(candidates, candidate{lift: lift, margin2: margin2, margin3: margin3, tol: tol})
				}
			}
		}
	}
	stats := make([]oracleP1VariantStats, len(candidates))

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag64 := diagnoseOpenLoopFrame64(enc)
		_ = enc.openloopStep()
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		intT1 := decodeP1ToIntegerLag(extractP1FromG192(bitFrame))
		for i, cand := range candidates {
			top := oracleMergeDiag64WithRangeMargins(diag64, cand.lift, cand.margin2, cand.margin3, cand.tol)
			stats[i].add(row.Expected, top)
			stats[i].addP1(top, intT1)
		}
	}

	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a := stats[order[i]]
		b := stats[order[j]]
		aRaw := 100 * float64(a.exact) / float64(a.total)
		bRaw := 100 * float64(b.exact) / float64(b.total)
		aMeetsRaw := aRaw >= minRawExactPct
		bMeetsRaw := bRaw >= minRawExactPct
		if aMeetsRaw != bMeetsRaw {
			return aMeetsRaw
		}
		if a.p1Hits != b.p1Hits {
			return a.p1Hits > b.p1Hits
		}
		return a.exact > b.exact
	})
	for rank := 0; rank < 16 && rank < len(order); rank++ {
		i := order[rank]
		cand := candidates[i]
		st := stats[i]
		t.Logf("rank %02d lift %.2f margin2 %.2f margin3 %.2f tol %d: raw exact %d/%d %.2f%% p1-window %d/%d %.2f%% ±10 %d %.2f%%",
			rank+1, cand.lift, cand.margin2, cand.margin3, cand.tol,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.p1Hits, st.total, 100*float64(st.p1Hits)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total))
	}
	sort.Slice(order, func(i, j int) bool {
		a := stats[order[i]]
		b := stats[order[j]]
		if a.p1Hits != b.p1Hits {
			return a.p1Hits > b.p1Hits
		}
		return a.exact > b.exact
	})
	for rank := 0; rank < 8 && rank < len(order); rank++ {
		i := order[rank]
		cand := candidates[i]
		st := stats[i]
		t.Logf("p1-rank %02d lift %.2f margin2 %.2f margin3 %.2f tol %d: raw exact %d/%d %.2f%% p1-window %d/%d %.2f%% ±10 %d %.2f%%",
			rank+1, cand.lift, cand.margin2, cand.margin3, cand.tol,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.p1Hits, st.total, 100*float64(st.p1Hits)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total))
	}
	best := stats[order[0]]
	if 100*float64(best.exact)/float64(best.total) >= minRawExactPct &&
		100*float64(best.p1Hits)/float64(best.total) >= targetP1WindowPct {
		t.Logf("found config meeting raw %.2f%% and P1 %.2f%% targets", minRawExactPct, targetP1WindowPct)
	}
}

func TestOracleHCenter_ClosedLoopWithOracleTopDiagnostic(t *testing.T) {
	const (
		oraclePath       = "testdata/oracle/pitch_top_open_loop.csv"
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	type rates struct {
		p1 int
		p0 int
		p2 int
	}
	var production, oracleTop rates
	oracleTopWindowHits := 0
	prodEnc := NewEncoder()
	oracleEnc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, refP0, refP2 := oracleExtractPitchBitsFromG192(bitFrame)

		if _, err := prodEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("production frame %d: lpcStep: %v", f, err)
		}
		_ = prodEnc.openloopStep()
		_, _ = prodEnc.closedloopStep(0)
		_, _ = prodEnc.closedloopStep(1)
		if uint16(prodEnc.p1) == refP1 {
			production.p1++
		}
		if uint16(prodEnc.p0) == refP0 {
			production.p0++
		}
		if uint16(prodEnc.p2) == refP2 {
			production.p2++
		}

		row, ok := oracleByFrame[f]
		if !ok {
			t.Fatalf("missing oracle top row for frame %d", f)
		}
		refIntT1 := decodeP1ToIntegerLag(refP1)
		if refIntT1 >= row.Expected-5 && refIntT1 <= row.Expected+4 {
			oracleTopWindowHits++
		}
		if _, err := oracleEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("oracle-top frame %d: lpcStep: %v", f, err)
		}
		_ = oracleEnc.openloopStep()
		oracleEnc.tOp = int16(row.Expected)
		_, _ = oracleEnc.closedloopStep(0)
		_, _ = oracleEnc.closedloopStep(1)
		if uint16(oracleEnc.p1) == refP1 {
			oracleTop.p1++
		}
		if uint16(oracleEnc.p0) == refP0 {
			oracleTop.p0++
		}
		if uint16(oracleEnc.p2) == refP2 {
			oracleTop.p2++
		}
	}
	t.Logf("production closed-loop byte-EQ: P1 %d/%d %.2f%% P0 %d/%d %.2f%% P2 %d/%d %.2f%%",
		production.p1, totalFrames, 100*float64(production.p1)/float64(totalFrames),
		production.p0, totalFrames, 100*float64(production.p0)/float64(totalFrames),
		production.p2, totalFrames, 100*float64(production.p2)/float64(totalFrames))
	t.Logf("oracle-T_op closed-loop byte-EQ: P1 %d/%d %.2f%% P0 %d/%d %.2f%% P2 %d/%d %.2f%%",
		oracleTop.p1, totalFrames, 100*float64(oracleTop.p1)/float64(totalFrames),
		oracleTop.p0, totalFrames, 100*float64(oracleTop.p0)/float64(totalFrames),
		oracleTop.p2, totalFrames, 100*float64(oracleTop.p2)/float64(totalFrames))
	t.Logf("reference int(T1) inside oracle T_op window: %d/%d %.2f%%",
		oracleTopWindowHits, totalFrames, 100*float64(oracleTopWindowHits)/float64(totalFrames))
}

func oracleExtractPitchBitsFromG192(frame []byte) (p1, p0, p2 uint16) {
	const g192Bit1 uint16 = 0x0081
	getBit := func(idx int) uint16 {
		off := 4 + 2*idx
		if binary.LittleEndian.Uint16(frame[off:off+2]) == g192Bit1 {
			return 1
		}
		return 0
	}
	getField := func(start, n int) uint16 {
		var v uint16
		for i := 0; i < n; i++ {
			v = (v << 1) | getBit(start+i)
		}
		return v
	}
	return getField(18, 8), getBit(26), getField(51, 5)
}

func oracleExtractFCBBitsFromG192(frame []byte) (c1, s1, ga1, gb1, c2, s2, ga2, gb2 uint16) {
	const g192Bit1 uint16 = 0x0081
	getBit := func(idx int) uint16 {
		off := 4 + 2*idx
		if binary.LittleEndian.Uint16(frame[off:off+2]) == g192Bit1 {
			return 1
		}
		return 0
	}
	getField := func(start, n int) uint16 {
		var v uint16
		for i := 0; i < n; i++ {
			v = (v << 1) | getBit(start+i)
		}
		return v
	}
	c1 = getField(27, 13)
	s1 = getField(40, 4)
	ga1 = getField(44, 3)
	gb1 = getField(47, 4)
	c2 = getField(56, 13)
	s2 = getField(69, 4)
	ga2 = getField(73, 3)
	gb2 = getField(76, 4)
	return
}

func TestOracleHandoff_WritePitchClosedLoopSearchInputHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 to refresh pitch closed-loop search handoff files")
	}
	records, err := collectPitchClosedLoopSearchInputRecords(4)
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}
	dir := filepath.Join("testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(dir, "pitch_closedloop_search_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writePitchClosedLoopSearchCSV(filepath.Join(dir, "pitch_closedloop_search_got.csv"), records, "got", true); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writePitchClosedLoopSearchCSV(expectedPath, records, "expected", false); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 after verifier fills pitch_closedloop_search_expected_template.csv")
	}
	dir := filepath.Join("testdata", "oracle", "handoff")
	got, err := readPitchClosedLoopSearchValues(filepath.Join(dir, "pitch_closedloop_search_got.csv"), "got")
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	expected, blanks, err := readPitchClosedLoopSearchExpected(filepath.Join(dir, "pitch_closedloop_search_expected_template.csv"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells")
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF") == "1" && blanks > 0 {
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
		if len(first) < 16 {
			first = append(first, fmt.Sprintf("%s expected=%d got=%d delta=%+d", key, exp, gotVal, gotVal-exp))
		}
	}
	t.Logf("PITCH closed-loop search handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_PITCH_CLOSEDLOOP_SEARCH_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("PITCH closed-loop search handoff has %d mismatches", mismatch)
	}
}

type pitchClosedLoopSearchRecord struct {
	Field string
	Frame int
	Sub   int
	Index int
	Lag   int
	Frac  int
	Value int64
}

func collectPitchClosedLoopSearchInputRecords(frameCount int) ([]pitchClosedLoopSearchRecord, error) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)
	if frameCount > totalFrames {
		frameCount = totalFrames
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("read PITCH.IN: %w", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		return nil, fmt.Errorf("read PITCH.BIT: %w", err)
	}
	if len(inData) < frameCount*bytesPerInFrame {
		return nil, fmt.Errorf("PITCH.IN size = %d, want at least %d", len(inData), frameCount*bytesPerInFrame)
	}
	if len(bitData) < frameCount*bytesPerBitFrame {
		return nil, fmt.Errorf("PITCH.BIT size = %d, want at least %d", len(bitData), frameCount*bytesPerBitFrame)
	}

	var records []pitchClosedLoopSearchRecord
	add := func(field string, frame, sub, index, lag, frac int, value int64) {
		records = append(records, pitchClosedLoopSearchRecord{
			Field: field,
			Frame: frame,
			Sub:   sub,
			Index: index,
			Lag:   lag,
			Frac:  frac,
			Value: value,
		})
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for frame := 0; frame < frameCount; frame++ {
		base := frame * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			return nil, fmt.Errorf("frame %d: lpcStep: %w", frame, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[frame*bytesPerBitFrame : (frame+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		collectPitchClosedLoopSearchSubframeRecords(enc, frame, 0, refInt1, refFrac1, refP1, add)
		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		collectPitchClosedLoopSearchSubframeRecords(enc, frame, 1, refInt2, refFrac2, refP2, add)
		_, _ = enc.closedloopStep(1)
	}
	return records, nil
}

func collectPitchClosedLoopSearchSubframeRecords(e *Encoder, frame, sub, refInt, refFrac int, refCode uint16, add func(string, int, int, int, int, int, int64)) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	kMin, kMax := closedLoopSearchWindow(centre, sub)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]

	prodLag, prodRN := clpitch.SearchInteger(&xb, exc, centre, sub)
	prodFrac := clpitch.RefineFraction(&xb, exc, prodLag, sub == 1 || prodLag < 85)
	prodCode := uint16(clpitch.EncodeP1(prodLag, prodFrac))
	if sub == 1 {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		prodCode = uint16(clpitch.EncodeP2(prodLag, prodFrac, tmin))
	}

	add("centre", frame, sub, -1, -1, -1, int64(centre))
	add("window_min", frame, sub, -1, -1, -1, int64(kMin))
	add("window_max", frame, sub, -1, -1, -1, int64(kMax))
	add("ref_int", frame, sub, -1, -1, -1, int64(refInt))
	add("ref_frac", frame, sub, -1, -1, -1, int64(refFrac))
	add("ref_code", frame, sub, -1, -1, -1, int64(refCode))
	add("prod_int", frame, sub, -1, -1, -1, int64(prodLag))
	add("prod_frac", frame, sub, -1, -1, -1, int64(prodFrac))
	add("prod_code", frame, sub, -1, -1, -1, int64(prodCode))
	add("prod_rn_int", frame, sub, -1, int(prodLag), 0, int64(prodRN))
	add("ref_rn_frac", frame, sub, -1, refInt, refFrac, int64(closedLoopRNFrac(&xb, exc, refInt, refFrac)))

	for i, value := range aHat {
		add("a_hat", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range r {
		add("residual", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range x {
		add("target_x", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range h {
		add("impulse_h", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range xb {
		add("target_xb", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:] {
		add("old_exc", frame, sub, i, -1, -1, int64(value))
	}
	for i, value := range r {
		add("exc_residual_ext", frame, sub, i, -1, -1, int64(value))
	}
	for k := kMin; k <= kMax; k++ {
		add("rn_int", frame, sub, -1, k, 0, int64(closedLoopRNInt(&xb, exc, k)))
		for _, frac := range closedLoopAllowedFracs(sub, k) {
			add("rn_frac", frame, sub, -1, k, int(frac), int64(closedLoopRNFrac(&xb, exc, k, int(frac))))
		}
	}
}

func writePitchClosedLoopSearchCSV(path string, records []pitchClosedLoopSearchRecord, valueColumn string, includeValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"field", "frame", "sub", "index", "lag", "frac", valueColumn}); err != nil {
		return err
	}
	for _, r := range records {
		value := ""
		if includeValue {
			value = strconv.FormatInt(r.Value, 10)
		}
		if err := w.Write([]string{
			r.Field,
			strconv.Itoa(r.Frame),
			strconv.Itoa(r.Sub),
			strconv.Itoa(r.Index),
			strconv.Itoa(r.Lag),
			strconv.Itoa(r.Frac),
			value,
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func readPitchClosedLoopSearchValues(path, valueColumn string) (map[string]int64, error) {
	rows, err := readPitchClosedLoopSearchRows(path)
	if err != nil {
		return nil, err
	}
	valueIdx := pitchClosedLoopSearchHeaderIndex(rows[0], valueColumn)
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}
	out := make(map[string]int64, len(rows)-1)
	for line, row := range rows[1:] {
		key, err := pitchClosedLoopSearchKey(row)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line+2, err)
		}
		value, err := strconv.ParseInt(row[valueIdx], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse %s: %w", line+2, valueColumn, err)
		}
		out[key] = value
	}
	return out, nil
}

func readPitchClosedLoopSearchExpected(path string) (map[string]int64, int, error) {
	rows, err := readPitchClosedLoopSearchRows(path)
	if err != nil {
		return nil, 0, err
	}
	valueIdx := pitchClosedLoopSearchHeaderIndex(rows[0], "expected")
	if valueIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}
	out := make(map[string]int64, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		key, err := pitchClosedLoopSearchKey(row)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: %w", line+2, err)
		}
		if row[valueIdx] == "" {
			blanks++
			continue
		}
		value, err := strconv.ParseInt(row[valueIdx], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: parse expected: %w", line+2, err)
		}
		out[key] = value
	}
	return out, blanks, nil
}

func readPitchClosedLoopSearchRows(path string) ([][]string, error) {
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
	return rows, nil
}

func pitchClosedLoopSearchHeaderIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func pitchClosedLoopSearchKey(row []string) (string, error) {
	if len(row) < 6 {
		return "", fmt.Errorf("short row")
	}
	return strings.Join(row[:6], ","), nil
}

type closedLoopStageSplit struct {
	total       int
	windowHit   int
	intBest     int
	fullBest    int
	prodIntHit  int
	prodFracHit int
	prodCodeHit int
}

type closedLoopSignalVariantStats struct {
	total     int
	windowHit int
	fullBest  int
}

type closedLoopSearchPolicyStats struct {
	total             int
	windowHit         int
	globalBest        int
	positiveBest      int
	selectedCode      int
	nonPositiveGlobal int
}

type closedLoopInputAttributionStats struct {
	total               int
	windowMiss          int
	baselineBest        int
	zeroOldExcBest      int
	xZeroOldExcBest     int
	rZeroOldExcBest     int
	zeroOldExcOnly      int
	xZeroOldExcOnly     int
	rZeroOldExcOnly     int
	baselineSurfaceMiss int
	first               []string
}

func (s *closedLoopInputAttributionStats) add(frame, sub int, refInt, refFrac, kMin, kMax int, baseBest, zeroOldBest, xZeroOldBest, rZeroOldBest bool, prodInt int16, prodFrac int8) {
	s.total++
	inWindow := refInt >= kMin && refInt <= kMax
	if !inWindow {
		s.windowMiss++
	} else if !baseBest {
		s.baselineSurfaceMiss++
	}
	if baseBest {
		s.baselineBest++
	}
	if zeroOldBest {
		s.zeroOldExcBest++
	}
	if xZeroOldBest {
		s.xZeroOldExcBest++
	}
	if rZeroOldBest {
		s.rZeroOldExcBest++
	}
	if !baseBest && zeroOldBest {
		s.zeroOldExcOnly++
	}
	if !baseBest && xZeroOldBest {
		s.xZeroOldExcOnly++
	}
	if !baseBest && rZeroOldBest {
		s.rZeroOldExcOnly++
	}
	if len(s.first) < 10 && (!inWindow || (!baseBest && (zeroOldBest || xZeroOldBest || rZeroOldBest))) {
		s.first = append(s.first, fmt.Sprintf(
			"frame=%d sub=%d ref=(%d,%+d) prod=(%d,%+d) window=[%d,%d] inWindow=%t baselineBest=%t zeroOldExcBest=%t xZeroOldExcBest=%t rZeroOldExcBest=%t",
			frame, sub, refInt, refFrac, prodInt, prodFrac, kMin, kMax, inWindow, baseBest, zeroOldBest, xZeroOldBest, rZeroOldBest))
	}
}

func (s closedLoopInputAttributionStats) log(t *testing.T, label string) {
	t.Helper()
	t.Logf("%s: total=%d window-miss %d/%d %.2f%% baseline-surface-miss %d/%d %.2f%% baseline-best %d/%d %.2f%%",
		label,
		s.total,
		s.windowMiss, s.total, 100*float64(s.windowMiss)/float64(s.total),
		s.baselineSurfaceMiss, s.total, 100*float64(s.baselineSurfaceMiss)/float64(s.total),
		s.baselineBest, s.total, 100*float64(s.baselineBest)/float64(s.total))
	t.Logf("%s variants: zero-oldExc-best %d/%d %.2f%% x+zero-oldExc-best %d/%d %.2f%% r+zero-oldExc-best %d/%d %.2f%%",
		label,
		s.zeroOldExcBest, s.total, 100*float64(s.zeroOldExcBest)/float64(s.total),
		s.xZeroOldExcBest, s.total, 100*float64(s.xZeroOldExcBest)/float64(s.total),
		s.rZeroOldExcBest, s.total, 100*float64(s.rZeroOldExcBest)/float64(s.total))
	t.Logf("%s rescued over baseline: zero-oldExc %d/%d %.2f%% x+zero-oldExc %d/%d %.2f%% r+zero-oldExc %d/%d %.2f%%",
		label,
		s.zeroOldExcOnly, s.total, 100*float64(s.zeroOldExcOnly)/float64(s.total),
		s.xZeroOldExcOnly, s.total, 100*float64(s.xZeroOldExcOnly)/float64(s.total),
		s.rZeroOldExcOnly, s.total, 100*float64(s.rZeroOldExcOnly)/float64(s.total))
	for i, msg := range s.first {
		t.Logf("%s first[%d]: %s", label, i, msg)
	}
}

func (s *closedLoopSignalVariantStats) add(inWindow, fullBest bool) {
	s.total++
	if inWindow {
		s.windowHit++
	}
	if fullBest {
		s.fullBest++
	}
}

func logClosedLoopSignalVariant(t *testing.T, label string, s closedLoopSignalVariantStats) {
	t.Helper()
	t.Logf("%s: ref-window %d/%d %.2f%% full-frac-RN-best %d/%d %.2f%%",
		label,
		s.windowHit, s.total, 100*float64(s.windowHit)/float64(s.total),
		s.fullBest, s.total, 100*float64(s.fullBest)/float64(s.total))
}

func (s *closedLoopSearchPolicyStats) add(r closedLoopSearchPolicyResult) {
	s.total++
	if r.inWindow {
		s.windowHit++
	}
	if r.globalBest {
		s.globalBest++
	}
	if r.positiveBest {
		s.positiveBest++
	}
	if r.selectedCode {
		s.selectedCode++
	}
	if r.nonPositiveGlobal {
		s.nonPositiveGlobal++
	}
}

func logClosedLoopSearchPolicy(t *testing.T, label string, s closedLoopSearchPolicyStats) {
	t.Helper()
	t.Logf("%s: ref-window %d/%d %.2f%% global-best %d/%d %.2f%% positive-global-best %d/%d %.2f%% selected-code %d/%d %.2f%% non-positive-global %d/%d %.2f%%",
		label,
		s.windowHit, s.total, 100*float64(s.windowHit)/float64(s.total),
		s.globalBest, s.total, 100*float64(s.globalBest)/float64(s.total),
		s.positiveBest, s.total, 100*float64(s.positiveBest)/float64(s.total),
		s.selectedCode, s.total, 100*float64(s.selectedCode)/float64(s.total),
		s.nonPositiveGlobal, s.total, 100*float64(s.nonPositiveGlobal)/float64(s.total))
}

func (s *closedLoopStageSplit) add(inWindow, intBest, fullBest bool, prodInt, prodFrac, prodCode bool) {
	s.total++
	if inWindow {
		s.windowHit++
	}
	if intBest {
		s.intBest++
	}
	if fullBest {
		s.fullBest++
	}
	if prodInt {
		s.prodIntHit++
	}
	if prodFrac {
		s.prodFracHit++
	}
	if prodCode {
		s.prodCodeHit++
	}
}

func logClosedLoopStageSplit(t *testing.T, label string, s closedLoopStageSplit) {
	t.Helper()
	t.Logf("%s: ref-window %d/%d %.2f%% int-RN-best %d/%d %.2f%% full-frac-RN-best %d/%d %.2f%% prod-int %d/%d %.2f%% prod-frac %d/%d %.2f%% prod-code %d/%d %.2f%%",
		label,
		s.windowHit, s.total, 100*float64(s.windowHit)/float64(s.total),
		s.intBest, s.total, 100*float64(s.intBest)/float64(s.total),
		s.fullBest, s.total, 100*float64(s.fullBest)/float64(s.total),
		s.prodIntHit, s.total, 100*float64(s.prodIntHit)/float64(s.total),
		s.prodFracHit, s.total, 100*float64(s.prodFracHit)/float64(s.total),
		s.prodCodeHit, s.total, 100*float64(s.prodCodeHit)/float64(s.total))
}

func diagnoseClosedLoopStage(e *Encoder, sub int, refInt int, refFrac int, refCode uint16) (inWindow, intBest, fullBest, prodInt, prodFrac, prodCode bool) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]

	kMin, kMax := closedLoopSearchWindow(centre, sub)
	inWindow = refInt >= kMin && refInt <= kMax

	prodLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	prodF := clpitch.RefineFraction(&xb, exc, prodLag, sub == 1 || prodLag < 85)
	prodInt = int(prodLag) == refInt
	prodFrac = prodInt && int(prodF) == refFrac
	if sub == 0 {
		prodCode = uint16(clpitch.EncodeP1(prodLag, prodF)) == refCode
	} else {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		prodCode = uint16(clpitch.EncodeP2(prodLag, prodF, tmin)) == refCode
	}

	if !inWindow {
		return
	}
	refRNInt := closedLoopRNInt(&xb, exc, refInt)
	bestRNInt := refRNInt
	for k := kMin; k <= kMax; k++ {
		if rn := closedLoopRNInt(&xb, exc, k); rn > bestRNInt {
			bestRNInt = rn
		}
	}
	intBest = refRNInt == bestRNInt

	refRNFull := closedLoopRNFrac(&xb, exc, refInt, refFrac)
	bestRNFull := refRNFull
	for k := kMin; k <= kMax; k++ {
		for _, frac := range closedLoopAllowedFracs(sub, k) {
			if rn := closedLoopRNFrac(&xb, exc, k, int(frac)); rn > bestRNFull {
				bestRNFull = rn
			}
		}
	}
	fullBest = refRNFull == bestRNFull
	return
}

func diagnoseClosedLoopSignalVariants(e *Encoder, sub int, refInt int, refFrac int) map[string][2]bool {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var zeroMem [10]int16
	var xZeroMem, xbZeroMem [clpitch.SubframeLen]int16
	clpitch.TargetSignal(aHat, &r, &zeroMem, &xZeroMem)
	clpitch.BackwardFilter(&xZeroMem, &h, &xbZeroMem)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	kMin, kMax := closedLoopSearchWindow(centre, sub)
	inWindow := refInt >= kMin && refInt <= kMax

	var excBaseline, excZeroOld, excNoResidual [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	var excOldHalf, excOldDouble, excOldNeg, excOldNegHalf [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excBaseline[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excBaseline[clpitch.PitchMaxInt:], r[:])
	copy(excZeroOld[clpitch.PitchMaxInt:], r[:])
	copy(excNoResidual[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	for i, sample := range e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:] {
		excOldHalf[i] = sample / 2
		excOldDouble[i] = fixed.Saturate(int32(sample) * 2)
		excOldNeg[i] = fixed.Saturate(-int32(sample))
		excOldNegHalf[i] = -sample / 2
	}
	copy(excOldHalf[clpitch.PitchMaxInt:], r[:])
	copy(excOldDouble[clpitch.PitchMaxInt:], r[:])
	copy(excOldNeg[clpitch.PitchMaxInt:], r[:])
	copy(excOldNegHalf[clpitch.PitchMaxInt:], r[:])

	out := map[string][2]bool{}
	add := func(name string, target *[clpitch.SubframeLen]int16, exc []int16) {
		out[name] = [2]bool{inWindow, inWindow && closedLoopRefFullBest(target, exc, sub, kMin, kMax, refInt, refFrac)}
	}
	add("baseline-xb+exc", &xb, excBaseline[:])
	add("target-x-direct+exc", &x, excBaseline[:])
	add("target-r-direct+exc", &r, excBaseline[:])
	add("zero-swmem-xb+exc", &xbZeroMem, excBaseline[:])
	add("xb+zero-oldexc", &xb, excZeroOld[:])
	add("xb+no-residual-ext", &xb, excNoResidual[:])
	add("xb+oldexc-half", &xb, excOldHalf[:])
	add("xb+oldexc-double", &xb, excOldDouble[:])
	add("xb+oldexc-neg", &xb, excOldNeg[:])
	add("xb+oldexc-neg-half", &xb, excOldNegHalf[:])
	add("x-direct+zero-oldexc", &x, excZeroOld[:])
	add("r-direct+zero-oldexc", &r, excZeroOld[:])
	return out
}

type closedLoopSearchPolicyResult struct {
	inWindow          bool
	globalBest        bool
	positiveBest      bool
	selectedCode      bool
	nonPositiveGlobal bool
}

func diagnoseClosedLoopSearchPolicy(e *Encoder, sub int, refInt int, refFrac int, refCode uint16, zeroOldExc bool) closedLoopSearchPolicyResult {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	kMin, kMax := closedLoopSearchWindow(centre, sub)
	out := closedLoopSearchPolicyResult{inWindow: refInt >= kMin && refInt <= kMax}

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	if !zeroOldExc {
		copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	}
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]

	if out.inWindow {
		refRN := closedLoopRNFrac(&xb, exc, refInt, refFrac)
		bestRN := refRN
		for k := kMin; k <= kMax; k++ {
			for _, frac := range closedLoopAllowedFracs(sub, k) {
				if rn := closedLoopRNFrac(&xb, exc, k, int(frac)); rn > bestRN {
					bestRN = rn
				}
			}
		}
		out.globalBest = refRN == bestRN
		out.positiveBest = out.globalBest && bestRN > 0
		out.nonPositiveGlobal = bestRN <= 0
	}

	prodLag, _ := clpitch.SearchInteger(&xb, exc, centre, sub)
	prodFrac := clpitch.RefineFraction(&xb, exc, prodLag, sub == 1 || prodLag < 85)
	if sub == 0 {
		out.selectedCode = uint16(clpitch.EncodeP1(prodLag, prodFrac)) == refCode
	} else {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		out.selectedCode = uint16(clpitch.EncodeP2(prodLag, prodFrac, tmin)) == refCode
	}
	return out
}

func closedLoopRefFullBest(xb *[clpitch.SubframeLen]int16, exc []int16, sub, kMin, kMax int, refInt, refFrac int) bool {
	refRN := closedLoopRNFrac(xb, exc, refInt, refFrac)
	bestRN := refRN
	for k := kMin; k <= kMax; k++ {
		for _, frac := range closedLoopAllowedFracs(sub, k) {
			if rn := closedLoopRNFrac(xb, exc, k, int(frac)); rn > bestRN {
				bestRN = rn
			}
		}
	}
	return refRN == bestRN
}

func forceClosedLoopStep(e *Encoder, sub int, intLag int16, frac int8, code uint16) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]

	clpitch.AdaptiveVector(exc, intLag, frac, &v)
	gp := clpitch.GpAndY(&x, &v, &h, &y)
	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(code)
		e.p0 = clpitch.EncodeP0(e.p1)
	} else {
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = uint8(code)
	}
	e.fcbStep(sub, &x, &y, &h, &v, gp)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func closedLoopSearchWindow(centre int16, sub int) (kMin, kMax int) {
	if sub == 0 {
		kMin = int(centre) - 3
		if kMin < clpitch.PitchMinInt {
			kMin = clpitch.PitchMinInt
		}
		kMax = int(centre) + 3
		if kMax > clpitch.PitchMaxInt {
			kMax = clpitch.PitchMaxInt
		}
		return kMin, kMax
	}
	tmin, tmax := clpitch.Subframe2Window(centre)
	return int(tmin), int(tmax)
}

func closedLoopAllowedFracs(sub, k int) []int8 {
	if sub == 1 || k < 85 {
		return []int8{-1, 0, 1}
	}
	return []int8{0}
}

func closedLoopRNInt(xb *[clpitch.SubframeLen]int16, exc []int16, lag int) fixed.Word32 {
	var acc fixed.Word32
	base := len(exc) - clpitch.SubframeLen - lag
	for n := 0; n < clpitch.SubframeLen; n++ {
		acc = fixed.LMac(acc, xb[n], exc[base+n])
	}
	return acc
}

func closedLoopRNFrac(xb *[clpitch.SubframeLen]int16, exc []int16, lag int, frac int) fixed.Word32 {
	var acc fixed.Word32
	if frac == 0 {
		return closedLoopRNInt(xb, exc, lag)
	}
	for n := 0; n < clpitch.SubframeLen; n++ {
		s := clpitch.Interpolate3(exc, int16(lag-n), int8(frac))
		acc = fixed.LMac(acc, xb[n], s)
	}
	return acc
}

func TestOracleHCenter_ClosedLoopStageSplitDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var sf1, sf2 closedLoopStageSplit
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		inWindow, intBest, fullBest, prodInt, prodFrac, prodCode := diagnoseClosedLoopStage(enc, 0, refInt1, refFrac1, refP1)
		sf1.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)

		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		inWindow, intBest, fullBest, prodInt, prodFrac, prodCode = diagnoseClosedLoopStage(enc, 1, refInt2, refFrac2, refP2)
		sf2.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)

		_, _ = enc.closedloopStep(1)
	}
	logClosedLoopStageSplit(t, "subframe1", sf1)
	logClosedLoopStageSplit(t, "subframe2", sf2)
}

func TestOracleHCenter_ClosedLoopForcedPitchStateDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var sf1, sf2 closedLoopStageSplit
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		inWindow, intBest, fullBest, prodInt, prodFrac, prodCode := diagnoseClosedLoopStage(enc, 0, refInt1, refFrac1, refP1)
		sf1.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		inWindow, intBest, fullBest, prodInt, prodFrac, prodCode = diagnoseClosedLoopStage(enc, 1, refInt2, refFrac2, refP2)
		sf2.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}
	logClosedLoopStageSplit(t, "forced-pitch-state subframe1", sf1)
	logClosedLoopStageSplit(t, "forced-pitch-state subframe2", sf2)
}

func TestOracleHCenter_ClosedLoopSignalVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	names := []string{
		"baseline-xb+exc",
		"target-x-direct+exc",
		"target-r-direct+exc",
		"zero-swmem-xb+exc",
		"xb+zero-oldexc",
		"xb+no-residual-ext",
		"xb+oldexc-half",
		"xb+oldexc-double",
		"xb+oldexc-neg",
		"xb+oldexc-neg-half",
		"x-direct+zero-oldexc",
		"r-direct+zero-oldexc",
	}
	sf1 := make(map[string]*closedLoopSignalVariantStats, len(names))
	sf2 := make(map[string]*closedLoopSignalVariantStats, len(names))
	for _, name := range names {
		sf1[name] = &closedLoopSignalVariantStats{}
		sf2[name] = &closedLoopSignalVariantStats{}
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		for name, result := range diagnoseClosedLoopSignalVariants(enc, 0, refInt1, refFrac1) {
			sf1[name].add(result[0], result[1])
		}

		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		for name, result := range diagnoseClosedLoopSignalVariants(enc, 1, refInt2, refFrac2) {
			sf2[name].add(result[0], result[1])
		}

		_, _ = enc.closedloopStep(1)
	}

	for _, name := range names {
		logClosedLoopSignalVariant(t, "subframe1 "+name, *sf1[name])
	}
	for _, name := range names {
		logClosedLoopSignalVariant(t, "subframe2 "+name, *sf2[name])
	}
}

func TestOracleHCenter_ClosedLoopInputAttributionDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var sf1, sf2 closedLoopInputAttributionStats
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		kMin, kMax, prodLag, prodFrac := closedLoopSearchSnapshot(enc, 0)
		_, _, baseBest, _, _, _ := diagnoseClosedLoopStage(enc, 0, refInt1, refFrac1, refP1)
		variants := diagnoseClosedLoopSignalVariants(enc, 0, refInt1, refFrac1)
		sf1.add(f, 0, refInt1, refFrac1, kMin, kMax, baseBest,
			variants["xb+zero-oldexc"][1],
			variants["x-direct+zero-oldexc"][1],
			variants["r-direct+zero-oldexc"][1],
			prodLag, prodFrac)

		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		kMin, kMax, prodLag, prodFrac = closedLoopSearchSnapshot(enc, 1)
		_, _, baseBest, _, _, _ = diagnoseClosedLoopStage(enc, 1, refInt2, refFrac2, refP2)
		variants = diagnoseClosedLoopSignalVariants(enc, 1, refInt2, refFrac2)
		sf2.add(f, 1, refInt2, refFrac2, kMin, kMax, baseBest,
			variants["xb+zero-oldexc"][1],
			variants["x-direct+zero-oldexc"][1],
			variants["r-direct+zero-oldexc"][1],
			prodLag, prodFrac)

		_, _ = enc.closedloopStep(1)
	}

	sf1.log(t, "subframe1")
	sf2.log(t, "subframe2")
}

func TestOracleHCenter_ClosedLoopSearchPolicyFloorDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var baseSF1, baseSF2, zeroSF1, zeroSF2 closedLoopSearchPolicyStats
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		baseSF1.add(diagnoseClosedLoopSearchPolicy(enc, 0, refInt1, refFrac1, refP1, false))
		zeroSF1.add(diagnoseClosedLoopSearchPolicy(enc, 0, refInt1, refFrac1, refP1, true))

		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)
		baseSF2.add(diagnoseClosedLoopSearchPolicy(enc, 1, refInt2, refFrac2, refP2, false))
		zeroSF2.add(diagnoseClosedLoopSearchPolicy(enc, 1, refInt2, refFrac2, refP2, true))

		_, _ = enc.closedloopStep(1)
	}

	logClosedLoopSearchPolicy(t, "baseline subframe1", baseSF1)
	logClosedLoopSearchPolicy(t, "zero-oldExc subframe1", zeroSF1)
	logClosedLoopSearchPolicy(t, "baseline subframe2", baseSF2)
	logClosedLoopSearchPolicy(t, "zero-oldExc subframe2", zeroSF2)
}

func closedLoopSearchSnapshot(e *Encoder, sub int) (kMin, kMax int, prodLag int16, prodFrac int8) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}
	kMin, kMax = closedLoopSearchWindow(centre, sub)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	prodLag, _ = clpitch.SearchInteger(&xb, exc, centre, sub)
	prodFrac = clpitch.RefineFraction(&xb, exc, prodLag, sub == 1 || prodLag < 85)
	return
}

func TestOracleHCenter_ClosedLoopStateCommitVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	type commitVariant struct {
		name        string
		zeroOldExc  bool
		zeroSwMem   bool
		zeroPastQua bool
		enc         *Encoder
		sf1         closedLoopStageSplit
		sf2         closedLoopStageSplit
	}
	variants := []*commitVariant{
		{name: "production", enc: NewEncoder()},
		{name: "zero-oldExc-after-commit", zeroOldExc: true, enc: NewEncoder()},
		{name: "zero-swMemErr-after-commit", zeroSwMem: true, enc: NewEncoder()},
		{name: "zero-oldExc+swMemErr-after-commit", zeroOldExc: true, zeroSwMem: true, enc: NewEncoder()},
		{name: "zero-pastQuaEn-after-commit", zeroPastQua: true, enc: NewEncoder()},
	}

	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for _, v := range variants {
			enc := v.enc
			if _, err := enc.lpcStep(pcm[:]); err != nil {
				t.Fatalf("%s frame %d: lpcStep: %v", v.name, f, err)
			}
			_ = enc.openloopStep()
			inWindow, intBest, fullBest, prodInt, prodFrac, prodCode := diagnoseClosedLoopStage(enc, 0, refInt1, refFrac1, refP1)
			v.sf1.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
			_, _ = enc.closedloopStep(0)
			applyClosedLoopCommitVariant(enc, v.zeroOldExc, v.zeroSwMem, v.zeroPastQua)

			inWindow, intBest, fullBest, prodInt, prodFrac, prodCode = diagnoseClosedLoopStage(enc, 1, refInt2, refFrac2, refP2)
			v.sf2.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
			_, _ = enc.closedloopStep(1)
			applyClosedLoopCommitVariant(enc, v.zeroOldExc, v.zeroSwMem, v.zeroPastQua)
		}
	}

	for _, v := range variants {
		logClosedLoopStageSplit(t, v.name+" subframe1", v.sf1)
		logClosedLoopStageSplit(t, v.name+" subframe2", v.sf2)
	}
}

func applyClosedLoopCommitVariant(e *Encoder, zeroOldExc, zeroSwMem, zeroPastQua bool) {
	if zeroOldExc {
		e.oldExc = [154]int16{}
	}
	if zeroSwMem {
		e.swMemErr = [10]int16{}
	}
	if zeroPastQua {
		for i := range e.pastQuaEn {
			e.pastQuaEn[i] = gain.PastErrorsDefault
		}
	}
}

type oldExcTailMode int

const (
	oldExcTailProduction oldExcTailMode = iota
	oldExcTailDiagnosticSum
	oldExcTailPitchOnly
	oldExcTailCodeOnly
	oldExcTailZero
)

func TestOracleHCenter_ClosedLoopOldExcComponentCommitDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	type componentVariant struct {
		name string
		mode oldExcTailMode
		enc  *Encoder
		sf1  closedLoopStageSplit
		sf2  closedLoopStageSplit
	}
	variants := []*componentVariant{
		{name: "production", mode: oldExcTailProduction, enc: NewEncoder()},
		{name: "diagnostic-sum-tail", mode: oldExcTailDiagnosticSum, enc: NewEncoder()},
		{name: "pitch-only-tail", mode: oldExcTailPitchOnly, enc: NewEncoder()},
		{name: "code-only-tail", mode: oldExcTailCodeOnly, enc: NewEncoder()},
		{name: "zero-tail", mode: oldExcTailZero, enc: NewEncoder()},
	}

	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for _, v := range variants {
			enc := v.enc
			if _, err := enc.lpcStep(pcm[:]); err != nil {
				t.Fatalf("%s frame %d: lpcStep: %v", v.name, f, err)
			}
			_ = enc.openloopStep()

			inWindow, intBest, fullBest, prodInt, prodFrac, prodCode := diagnoseClosedLoopStage(enc, 0, refInt1, refFrac1, refP1)
			v.sf1.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
			sf1Tap := diagnoseFCBCommitTap(enc, 0, false, 0, 0)
			_, _ = enc.closedloopStep(0)
			applyOldExcTailMode(enc, sf1Tap, v.mode)

			inWindow, intBest, fullBest, prodInt, prodFrac, prodCode = diagnoseClosedLoopStage(enc, 1, refInt2, refFrac2, refP2)
			v.sf2.add(inWindow, intBest, fullBest, prodInt, prodFrac, prodCode)
			sf2Tap := diagnoseFCBCommitTap(enc, 1, false, 0, 0)
			_, _ = enc.closedloopStep(1)
			applyOldExcTailMode(enc, sf2Tap, v.mode)
		}
	}

	for _, v := range variants {
		logClosedLoopStageSplit(t, v.name+" subframe1", v.sf1)
		logClosedLoopStageSplit(t, v.name+" subframe2", v.sf2)
	}
}

func applyOldExcTailMode(e *Encoder, tap fcbCommitTap, mode oldExcTailMode) {
	if mode == oldExcTailProduction {
		return
	}
	base := len(e.oldExc) - clpitch.SubframeLen
	switch mode {
	case oldExcTailDiagnosticSum:
		copy(e.oldExc[base:], tap.sumTail[:])
	case oldExcTailPitchOnly:
		copy(e.oldExc[base:], tap.pitchTail[:])
	case oldExcTailCodeOnly:
		copy(e.oldExc[base:], tap.codeTail[:])
	case oldExcTailZero:
		for i := base; i < len(e.oldExc); i++ {
			e.oldExc[i] = 0
		}
	}
}

type fcbCommitTap struct {
	s, c, ga, gb        uint16
	gpQ14               int16
	gcQ12               int32
	taming              bool
	absPitch            int64
	absCode             int64
	pitchTail, codeTail [clpitch.SubframeLen]int16
	sumTail             [clpitch.SubframeLen]int16
	saturations         int
}

type fcbCommitSplitStats struct {
	total         int
	sHit          int
	cHit          int
	gaHit         int
	gbHit         int
	allHit        int
	taming        int
	pitchDominant int
	codeDominant  int
	balanced      int
	saturations   int
	absPitch      int64
	absCode       int64
}

func (s *fcbCommitSplitStats) add(tap fcbCommitTap, refC, refS, refGA, refGB uint16) {
	s.total++
	sHit := tap.s == refS
	cHit := tap.c == refC
	gaHit := tap.ga == refGA
	gbHit := tap.gb == refGB
	if sHit {
		s.sHit++
	}
	if cHit {
		s.cHit++
	}
	if gaHit {
		s.gaHit++
	}
	if gbHit {
		s.gbHit++
	}
	if sHit && cHit && gaHit && gbHit {
		s.allHit++
	}
	if tap.taming {
		s.taming++
	}
	if tap.absPitch > tap.absCode*2 {
		s.pitchDominant++
	} else if tap.absCode > tap.absPitch*2 {
		s.codeDominant++
	} else {
		s.balanced++
	}
	s.saturations += tap.saturations
	s.absPitch += tap.absPitch
	s.absCode += tap.absCode
}

func logFCBCommitSplit(t *testing.T, label string, s fcbCommitSplitStats) {
	t.Helper()
	t.Logf("%s fields: S %d/%d %.2f%% C %d/%d %.2f%% GA %d/%d %.2f%% GB %d/%d %.2f%% all %d/%d %.2f%%",
		label,
		s.sHit, s.total, 100*float64(s.sHit)/float64(s.total),
		s.cHit, s.total, 100*float64(s.cHit)/float64(s.total),
		s.gaHit, s.total, 100*float64(s.gaHit)/float64(s.total),
		s.gbHit, s.total, 100*float64(s.gbHit)/float64(s.total),
		s.allHit, s.total, 100*float64(s.allHit)/float64(s.total))
	t.Logf("%s commit mix: abs(gp*v)=%d abs(gc*c)=%d pitch-dominant %d/%d %.2f%% code-dominant %d/%d %.2f%% balanced %d/%d %.2f%% taming %d/%d %.2f%% saturations=%d",
		label,
		s.absPitch, s.absCode,
		s.pitchDominant, s.total, 100*float64(s.pitchDominant)/float64(s.total),
		s.codeDominant, s.total, 100*float64(s.codeDominant)/float64(s.total),
		s.balanced, s.total, 100*float64(s.balanced)/float64(s.total),
		s.taming, s.total, 100*float64(s.taming)/float64(s.total),
		s.saturations)
}

func diagnoseFCBCommitTap(e *Encoder, sub int, forcePitch bool, forcedLag int16, forcedFrac int8) fcbCommitTap {
	return diagnoseFCBCommitTapWithCode(e, sub, forcePitch, forcedLag, forcedFrac, false, 0, 0)
}

func diagnoseFCBCommitTapWithReferenceCode(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC, refS uint16) fcbCommitTap {
	return diagnoseFCBCommitTapWithCode(e, sub, true, forcedLag, forcedFrac, true, refC, refS)
}

func diagnoseFCBCommitTapWithCode(e *Encoder, sub int, forcePitch bool, forcedLag int16, forcedFrac int8, forceCode bool, refC, refS uint16) fcbCommitTap {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)
	clpitch.BackwardFilter(&x, &h, &xb)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]

	var lag int16
	var frac int8
	if forcePitch {
		lag, frac = forcedLag, forcedFrac
	} else {
		var centre int16
		if sub == 0 {
			centre = e.tOp
		} else {
			centre = e.intT1
		}
		lag, _ = clpitch.SearchInteger(&xb, exc, centre, sub)
		frac = clpitch.RefineFraction(&xb, exc, lag, sub == 1 || lag < 85)
	}
	clpitch.AdaptiveVector(exc, lag, frac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)
	if forceCode {
		return fcbCommitTapFromReferenceCode(e, &x, &y, &h, &v, refC, refS, lag)
	}
	return fcbCommitTapFromSignals(e, sub, &x, &y, &h, &v, gpUnq, lag)
}

func fcbCommitTapFromSignals(e *Encoder, sub int, x, y, h, v *[clpitch.SubframeLen]int16, gpUnq int16, intLag int16) fcbCommitTap {
	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, h, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [clpitch.SubframeLen]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(x, y, &z, gpcPredQ12)
	gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
	taming := gpTamed != gpHatQ14
	gpHatQ14 = gpTamed

	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	gcQ12Wide := mantExpToQ12(gcMantQ14, gcExp)
	_ = gammaCQ13

	var tap fcbCommitTap
	tap.s = uint16(fcbsearch.PackS(&positions, &signs))
	tap.c = fcbsearch.PackC(&positions)
	tap.ga = uint16(gaBits)
	tap.gb = uint16(gbBits)
	tap.gpQ14 = gpHatQ14
	tap.gcQ12 = gcQ12Wide
	tap.taming = taming
	for n := 0; n < clpitch.SubframeLen; n++ {
		gpV := (int32(gpHatQ14) * int32(v[n])) >> 14
		gcC := int32((int64(gcQ12Wide) * int64(c[n])) >> 13)
		sum := gpV + gcC
		if int32(fixed.Saturate(sum)) != sum {
			tap.saturations++
		}
		tap.absPitch += int64(absInt32(gpV))
		tap.absCode += int64(absInt32(gcC))
		tap.pitchTail[n] = fixed.Saturate(gpV)
		tap.codeTail[n] = fixed.Saturate(gcC)
		tap.sumTail[n] = fixed.Saturate(sum)
	}
	return tap
}

func fcbCommitTapFromReferenceCode(e *Encoder, x, y, h, v *[clpitch.SubframeLen]int16, refC, refS uint16, intLag int16) fcbCommitTap {
	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: uint8(refS)}, int(intLag), e.prevGpQ14, &c)

	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	gaPhys, gbPhys, gpHatQ14, _ := gainquant.SearchConjugate(x, y, &z, gpcPredQ12)
	gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
	taming := gpTamed != gpHatQ14
	gpHatQ14 = gpTamed

	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	gcQ12Wide := mantExpToQ12(gcMantQ14, gcExp)

	tap := fcbCommitTap{
		s:      refS,
		c:      refC,
		ga:     uint16(gaBits),
		gb:     uint16(gbBits),
		gpQ14:  gpHatQ14,
		gcQ12:  gcQ12Wide,
		taming: taming,
	}
	for n := 0; n < clpitch.SubframeLen; n++ {
		gpV := (int32(gpHatQ14) * int32(v[n])) >> 14
		gcC := int32((int64(gcQ12Wide) * int64(c[n])) >> 13)
		sum := gpV + gcC
		if int32(fixed.Saturate(sum)) != sum {
			tap.saturations++
		}
		tap.absPitch += int64(absInt32(gpV))
		tap.absCode += int64(absInt32(gcC))
		tap.pitchTail[n] = fixed.Saturate(gpV)
		tap.codeTail[n] = fixed.Saturate(gcC)
		tap.sumTail[n] = fixed.Saturate(sum)
	}
	return tap
}

func absInt32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

type fcbSearchSurfaceTap struct {
	c             uint16
	s             uint16
	refSOnSurface uint16
	refBest       bool
	refValid      bool
	trackHits     [4]bool
}

type fcbSearchSurfaceStats struct {
	total        int
	cHit         int
	sHit         int
	refSurfaceS  int
	refBest      int
	refInvalid   int
	trackHits    [4]int
	refRoundTrip int
}

func (s *fcbSearchSurfaceStats) add(tap fcbSearchSurfaceTap, refC, refS uint16) {
	s.total++
	if tap.c == refC {
		s.cHit++
	}
	if tap.s == refS {
		s.sHit++
	}
	if tap.refSOnSurface == refS {
		s.refSurfaceS++
	}
	if tap.refBest {
		s.refBest++
	}
	if !tap.refValid {
		s.refInvalid++
	}
	for i := range tap.trackHits {
		if tap.trackHits[i] {
			s.trackHits[i]++
		}
	}
	refPos := oracleDecodeFCBPositions(refC)
	if fcbsearch.PackC(&refPos) == refC {
		s.refRoundTrip++
	}
}

func logFCBSearchSurface(t *testing.T, label string, s fcbSearchSurfaceStats) {
	t.Helper()
	t.Logf("%s: C %d/%d %.2f%% S %d/%d %.2f%% ref-position-surface-S %d/%d %.2f%% ref-position-best %d/%d %.2f%% ref-invalid %d/%d ref-C-roundtrip %d/%d",
		label,
		s.cHit, s.total, 100*float64(s.cHit)/float64(s.total),
		s.sHit, s.total, 100*float64(s.sHit)/float64(s.total),
		s.refSurfaceS, s.total, 100*float64(s.refSurfaceS)/float64(s.total),
		s.refBest, s.total, 100*float64(s.refBest)/float64(s.total),
		s.refInvalid, s.total,
		s.refRoundTrip, s.total)
	t.Logf("%s: per-track position hits T0 %d/%d %.2f%% T1 %d/%d %.2f%% T2 %d/%d %.2f%% T3 %d/%d %.2f%%",
		label,
		s.trackHits[0], s.total, 100*float64(s.trackHits[0])/float64(s.total),
		s.trackHits[1], s.total, 100*float64(s.trackHits[1])/float64(s.total),
		s.trackHits[2], s.total, 100*float64(s.trackHits[2])/float64(s.total),
		s.trackHits[3], s.total, 100*float64(s.trackHits[3])/float64(s.total))
}

func diagnoseFCBSearchSurface(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC, refS uint16) fcbSearchSurfaceTap {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	refPos := oracleDecodeFCBPositions(refC)
	refC2, refE, refValid := oracleFCBCriterion(&refPos, &dAbs, &phi)
	tap := fcbSearchSurfaceTap{
		c:             fcbsearch.PackC(&positions),
		s:             uint16(fcbsearch.PackS(&positions, &signs)),
		refSOnSurface: uint16(fcbsearch.PackS(&refPos, &signs)),
		refValid:      refValid,
	}
	if refValid {
		tap.refBest = oracleRatioEqualOrGreater(refC2, refE, sumOut[0], sumOut[1])
	}
	for i := range tap.trackHits {
		tap.trackHits[i] = positions[i] == refPos[i]
	}
	_ = refS
	return tap
}

func oracleDecodeFCBPositions(c uint16) [4]int8 {
	i0 := c & 0x7
	i1 := (c >> 3) & 0x7
	i2 := (c >> 6) & 0x7
	jx := (c >> 9) & 0x1
	i3 := (c >> 10) & 0x7
	return [4]int8{
		int8(5 * i0),
		int8(1 + 5*i1),
		int8(2 + 5*i2),
		int8(3 + jx + 5*i3),
	}
}

func oracleFCBCriterion(pos *[4]int8, dAbs *[clpitch.SubframeLen]int32, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32) (c2, e int64, ok bool) {
	var c int64
	for i := 0; i < 4; i++ {
		pi := pos[i]
		c += int64(dAbs[pi])
		e += int64(phi[pi][pi])
		for j := 0; j < i; j++ {
			e += int64(phi[pos[j]][pi])
		}
	}
	if e <= 0 {
		return 0, e, false
	}
	return c * c, e, true
}

func oracleRatioEqualOrGreater(a, b, c, d int64) bool {
	if b <= 0 || d <= 0 {
		return false
	}
	return !oracleRatioGreater(c, d, a, b)
}

func oracleRatioGreater(a, b, c, d int64) bool {
	hi1, lo1 := bits.Mul64(uint64(a), uint64(d))
	hi2, lo2 := bits.Mul64(uint64(c), uint64(b))
	if hi1 != hi2 {
		return hi1 > hi2
	}
	return lo1 > lo2
}

func TestOracleHCenter_FCBSearchSurfaceDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var sf1, sf2 fcbSearchSurfaceStats
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, _, _, refC2, refS2, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		sf1.add(diagnoseFCBSearchSurface(enc, 0, int16(refInt1), int8(refFrac1), refC1, refS1), refC1, refS1)
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		sf2.add(diagnoseFCBSearchSurface(enc, 1, int16(refInt2), int8(refFrac2), refC2, refS2), refC2, refS2)
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	logFCBSearchSurface(t, "forced-ref-pitch fcb-search subframe1", sf1)
	logFCBSearchSurface(t, "forced-ref-pitch fcb-search subframe2", sf2)
}

type fcbSearchInputVariantStats struct {
	total   int
	refBest int
	sHit    int
}

func (s *fcbSearchInputVariantStats) add(refBest, sHit bool) {
	s.total++
	if refBest {
		s.refBest++
	}
	if sHit {
		s.sHit++
	}
}

func logFCBSearchInputVariant(t *testing.T, label string, s fcbSearchInputVariantStats) {
	t.Helper()
	t.Logf("%s: ref-position-best %d/%d %.2f%% ref-position-surface-S %d/%d %.2f%%",
		label,
		s.refBest, s.total, 100*float64(s.refBest)/float64(s.total),
		s.sHit, s.total, 100*float64(s.sHit)/float64(s.total))
}

func diagnoseFCBSearchInputVariants(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC, refS uint16) map[string][2]bool {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var zeroMem [10]int16
	var xZeroMem [clpitch.SubframeLen]int16
	clpitch.TargetSignal(aHat, &r, &zeroMem, &xZeroMem)

	var excZeroOld [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excZeroOld[clpitch.PitchMaxInt:], r[:])
	var vZeroOld, yZeroOld [clpitch.SubframeLen]int16
	clpitch.AdaptiveVector(excZeroOld[:], forcedLag, forcedFrac, &vZeroOld)
	gpZeroOld := clpitch.GpAndY(&x, &vZeroOld, &h, &yZeroOld)

	out := map[string][2]bool{}
	add := func(name string, xPrime *[clpitch.SubframeLen]int16) {
		refBest, sHit := oracleFCBRefBestForXPrime(xPrime, &h, refC, refS)
		out[name] = [2]bool{refBest, sHit}
	}

	var xPrimeBaseline, xPrimeNoAdaptive, xPrimeZeroMem, xPrimeZeroOldV, xPrimeResidual [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrimeBaseline)
	copy(xPrimeNoAdaptive[:], x[:])
	fcbsearch.AdjustedTarget(&xZeroMem, &y, gpUnq, &xPrimeZeroMem)
	fcbsearch.AdjustedTarget(&x, &yZeroOld, gpZeroOld, &xPrimeZeroOldV)
	copy(xPrimeResidual[:], r[:])

	add("baseline-x-gpY", &xPrimeBaseline)
	add("no-adaptive-x", &xPrimeNoAdaptive)
	add("zero-swmem-x-gpY", &xPrimeZeroMem)
	add("zero-oldexc-v", &xPrimeZeroOldV)
	add("residual-direct-r", &xPrimeResidual)
	return out
}

func oracleFCBRefBestForXPrime(xPrime, h *[clpitch.SubframeLen]int16, refC, refS uint16) (refBest, sHit bool) {
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(xPrime, h, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	refPos := oracleDecodeFCBPositions(refC)
	refC2, refE, refValid := oracleFCBCriterion(&refPos, &dAbs, &phi)
	sHit = uint16(fcbsearch.PackS(&refPos, &signs)) == refS
	if !refValid {
		return false, sHit
	}
	return oracleRatioEqualOrGreater(refC2, refE, sumOut[0], sumOut[1]), sHit
}

type oracleFCBScoreBreakdown struct {
	pos   [4]int8
	c     uint16
	s     uint16
	dSum  int64
	c2    int64
	eDiag int64
	eOff  int64
	e     int64
	ok    bool
}

func oracleFCBScoreForPositions(pos *[4]int8, signs *[clpitch.SubframeLen]int16, dAbs *[clpitch.SubframeLen]int32, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32) oracleFCBScoreBreakdown {
	out := oracleFCBScoreBreakdown{
		pos: *pos,
		c:   fcbsearch.PackC(pos),
		s:   uint16(fcbsearch.PackS(pos, signs)),
	}
	for i := 0; i < 4; i++ {
		pi := pos[i]
		out.dSum += int64(dAbs[pi])
		out.eDiag += int64(phi[pi][pi])
		for j := 0; j < i; j++ {
			out.eOff += int64(phi[pos[j]][pi])
		}
	}
	out.c2 = out.dSum * out.dSum
	out.e = out.eDiag + out.eOff
	out.ok = out.e > 0
	return out
}

func oracleFCBScoreRatio(s oracleFCBScoreBreakdown) float64 {
	if !s.ok {
		return 0
	}
	return float64(s.c2) / float64(s.e)
}

func diagnoseFCBScoreTrace(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC uint16) (prod, ref oracleFCBScoreBreakdown) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)

	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	var prodPos [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &prodPos, &sumOut)
	refPos := oracleDecodeFCBPositions(refC)
	return oracleFCBScoreForPositions(&prodPos, &signs, &dAbs, &phi),
		oracleFCBScoreForPositions(&refPos, &signs, &dAbs, &phi)
}

func TestOracleHCenter_FCBSearchScoreTraceDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
		traceFrames      = 4
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, _, _, _, refC2, _, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		if f < traceFrames {
			prod, ref := diagnoseFCBScoreTrace(enc, 0, int16(refInt1), int8(refFrac1), refC1)
			t.Logf("frame %d sf1 prod pos=%v C=0x%04x S=0x%x dSum=%d C2=%d E=%d diag=%d off=%d ratio=%.6g",
				f, prod.pos, prod.c, prod.s, prod.dSum, prod.c2, prod.e, prod.eDiag, prod.eOff, oracleFCBScoreRatio(prod))
			t.Logf("frame %d sf1 ref  pos=%v C=0x%04x S=0x%x dSum=%d C2=%d E=%d diag=%d off=%d ratio=%.6g",
				f, ref.pos, ref.c, ref.s, ref.dSum, ref.c2, ref.e, ref.eDiag, ref.eOff, oracleFCBScoreRatio(ref))
		}
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		if f < traceFrames {
			prod, ref := diagnoseFCBScoreTrace(enc, 1, int16(refInt2), int8(refFrac2), refC2)
			t.Logf("frame %d sf2 prod pos=%v C=0x%04x S=0x%x dSum=%d C2=%d E=%d diag=%d off=%d ratio=%.6g",
				f, prod.pos, prod.c, prod.s, prod.dSum, prod.c2, prod.e, prod.eDiag, prod.eOff, oracleFCBScoreRatio(prod))
			t.Logf("frame %d sf2 ref  pos=%v C=0x%04x S=0x%x dSum=%d C2=%d E=%d diag=%d off=%d ratio=%.6g",
				f, ref.pos, ref.c, ref.s, ref.dSum, ref.c2, ref.e, ref.eDiag, ref.eOff, oracleFCBScoreRatio(ref))
		}
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}
}

type oracleFCBPhiVariant int

const (
	oraclePhiBaseline oracleFCBPhiVariant = iota
	oraclePhiNoDiagHalf
	oraclePhiNoOffdiagSign
	oraclePhiDoubleOffdiag
)

func oraclePhiPrimeVariant(h, signs *[clpitch.SubframeLen]int16, variant oracleFCBPhiVariant, phi *[clpitch.SubframeLen][clpitch.SubframeLen]int32) {
	for i := 0; i < clpitch.SubframeLen; i++ {
		var diag int32
		for n := i; n < clpitch.SubframeLen; n++ {
			t := int32(h[n-i])
			diag += t * t
		}
		if variant == oraclePhiNoDiagHalf {
			phi[i][i] = diag
		} else {
			phi[i][i] = diag >> 1
		}
		for j := i + 1; j < clpitch.SubframeLen; j++ {
			var sum int32
			for n := j; n < clpitch.SubframeLen; n++ {
				sum += int32(h[n-i]) * int32(h[n-j])
			}
			if variant != oraclePhiNoOffdiagSign {
				sum *= int32(signs[i]) * int32(signs[j])
			}
			if variant == oraclePhiDoubleOffdiag {
				sum *= 2
			}
			phi[i][j] = sum
			phi[j][i] = sum
		}
	}
}

func oracleFCBRefBestForXPrimePhiVariant(xPrime, h *[clpitch.SubframeLen]int16, refC uint16, variant oracleFCBPhiVariant) bool {
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(xPrime, h, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	oraclePhiPrimeVariant(h, &signs, variant, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	refPos := oracleDecodeFCBPositions(refC)
	refC2, refE, refValid := oracleFCBCriterion(&refPos, &dAbs, &phi)
	return refValid && oracleRatioEqualOrGreater(refC2, refE, sumOut[0], sumOut[1])
}

func diagnoseFCBPhiVariants(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC uint16) map[string]bool {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)

	return map[string]bool{
		"baseline":        oracleFCBRefBestForXPrimePhiVariant(&xPrime, &h, refC, oraclePhiBaseline),
		"no-diag-half":    oracleFCBRefBestForXPrimePhiVariant(&xPrime, &h, refC, oraclePhiNoDiagHalf),
		"no-offdiag-sign": oracleFCBRefBestForXPrimePhiVariant(&xPrime, &h, refC, oraclePhiNoOffdiagSign),
		"double-offdiag":  oracleFCBRefBestForXPrimePhiVariant(&xPrime, &h, refC, oraclePhiDoubleOffdiag),
	}
}

func TestOracleHCenter_FCBPhiVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	names := []string{"baseline", "no-diag-half", "no-offdiag-sign", "double-offdiag"}
	sf1 := map[string]int{}
	sf2 := map[string]int{}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, _, _, _, refC2, _, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for name, ok := range diagnoseFCBPhiVariants(enc, 0, int16(refInt1), int8(refFrac1), refC1) {
			if ok {
				sf1[name]++
			}
		}
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		for name, ok := range diagnoseFCBPhiVariants(enc, 1, int16(refInt2), int8(refFrac2), refC2) {
			if ok {
				sf2[name]++
			}
		}
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	for _, name := range names {
		t.Logf("phi variant %s sf1 ref-position-best %d/%d %.2f%% sf2 %d/%d %.2f%%",
			name,
			sf1[name], totalFrames, 100*float64(sf1[name])/float64(totalFrames),
			sf2[name], totalFrames, 100*float64(sf2[name])/float64(totalFrames))
	}
}

type oracleFCBCorrelationVariant int

const (
	oracleCorrBaseline oracleFCBCorrelationVariant = iota
	oracleCorrTargetPrefix
	oracleCorrReversedH
)

func oracleCorrelationDVariant(xPrime, h *[clpitch.SubframeLen]int16, variant oracleFCBCorrelationVariant, d *[clpitch.SubframeLen]int32) {
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc int32
		for i := n; i < clpitch.SubframeLen; i++ {
			switch variant {
			case oracleCorrTargetPrefix:
				acc += int32(xPrime[i-n]) * int32(h[i])
			case oracleCorrReversedH:
				acc += int32(xPrime[i]) * int32(h[clpitch.SubframeLen-1-(i-n)])
			default:
				acc += int32(xPrime[i]) * int32(h[i-n])
			}
		}
		d[n] = acc
	}
}

func oracleFCBRefBestForXPrimeCorrelationVariant(xPrime, h *[clpitch.SubframeLen]int16, refC uint16, variant oracleFCBCorrelationVariant) bool {
	var d [clpitch.SubframeLen]int32
	oracleCorrelationDVariant(xPrime, h, variant, &d)

	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	refPos := oracleDecodeFCBPositions(refC)
	refC2, refE, refValid := oracleFCBCriterion(&refPos, &dAbs, &phi)
	return refValid && oracleRatioEqualOrGreater(refC2, refE, sumOut[0], sumOut[1])
}

func diagnoseFCBCorrelationVariants(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC uint16) map[string]bool {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)

	return map[string]bool{
		"baseline":      oracleFCBRefBestForXPrimeCorrelationVariant(&xPrime, &h, refC, oracleCorrBaseline),
		"target-prefix": oracleFCBRefBestForXPrimeCorrelationVariant(&xPrime, &h, refC, oracleCorrTargetPrefix),
		"reversed-h":    oracleFCBRefBestForXPrimeCorrelationVariant(&xPrime, &h, refC, oracleCorrReversedH),
	}
}

func TestOracleHCenter_FCBCorrelationVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	names := []string{"baseline", "target-prefix", "reversed-h"}
	sf1 := map[string]int{}
	sf2 := map[string]int{}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, _, _, _, refC2, _, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for name, ok := range diagnoseFCBCorrelationVariants(enc, 0, int16(refInt1), int8(refFrac1), refC1) {
			if ok {
				sf1[name]++
			}
		}
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		for name, ok := range diagnoseFCBCorrelationVariants(enc, 1, int16(refInt2), int8(refFrac2), refC2) {
			if ok {
				sf2[name]++
			}
		}
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	for _, name := range names {
		t.Logf("correlation variant %s sf1 ref-position-best %d/%d %.2f%% sf2 %d/%d %.2f%%",
			name,
			sf1[name], totalFrames, 100*float64(sf1[name])/float64(totalFrames),
			sf2[name], totalFrames, 100*float64(sf2[name])/float64(totalFrames))
	}
}

type oracleGammaMode int

const (
	oracleGamma075 oracleGammaMode = iota
	oracleGammaNone
)

func oracleTargetSignalVariant(a *[11]int16, residual *[clpitch.SubframeLen]int16, swMem *[10]int16, gamma oracleGammaMode, x *[clpitch.SubframeLen]int16) {
	var aw [11]int16
	aw[0] = a[0]
	for i := 1; i <= 10; i++ {
		if gamma == oracleGammaNone {
			aw[i] = a[i]
		} else {
			aw[i] = fixed.Mult(a[i], oracleClosedLoopGammaPow(i))
		}
	}
	for n := 0; n < clpitch.SubframeLen; n++ {
		acc := fixed.LMult(residual[n], aw[0])
		for i := 1; i <= 10; i++ {
			var xni int16
			if n-i >= 0 {
				xni = x[n-i]
			} else {
				xni = swMem[10+n-i]
			}
			acc = fixed.LMsu(acc, aw[i], xni)
		}
		x[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func oracleImpulseResponseVariant(a *[11]int16, gamma oracleGammaMode, h *[clpitch.SubframeLen]int16) {
	var aw [11]int16
	aw[0] = a[0]
	for i := 1; i <= 10; i++ {
		if gamma == oracleGammaNone {
			aw[i] = a[i]
		} else {
			aw[i] = fixed.Mult(a[i], oracleClosedLoopGammaPow(i))
		}
	}
	for n := 0; n < clpitch.SubframeLen; n++ {
		var acc fixed.Word32
		if n == 0 {
			acc = fixed.LMult(4096, aw[0])
		}
		limit := n
		if limit > 10 {
			limit = 10
		}
		for i := 1; i <= limit; i++ {
			acc = fixed.LMsu(acc, aw[i], h[n-i])
		}
		acc = fixed.LShl(acc, 3)
		h[n] = fixed.Round(acc)
	}
}

func oracleClosedLoopGammaPow(i int) int16 {
	switch i {
	case 0:
		return 32767
	case 1:
		return 24576
	case 2:
		return 18432
	case 3:
		return 13824
	case 4:
		return 10368
	case 5:
		return 7776
	case 6:
		return 5832
	case 7:
		return 4374
	case 8:
		return 3281
	case 9:
		return 2460
	default:
		return 1845
	}
}

func diagnoseFCBXHVariants(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC uint16) map[string]bool {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	eval := func(aResidual, aXH *[11]int16, gamma oracleGammaMode) bool {
		var r, x, h, v, y, xPrime [clpitch.SubframeLen]int16
		lpResidualSubframe(sFrame, aResidual, &e.lpResidualMemQ, &r)
		oracleTargetSignalVariant(aXH, &r, &e.swMemErr, gamma, &x)
		oracleImpulseResponseVariant(aXH, gamma, &h)

		var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
		copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
		copy(excSearch[clpitch.PitchMaxInt:], r[:])
		exc := excSearch[:]
		clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
		gpUnq := clpitch.GpAndY(&x, &v, &h, &y)
		fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)
		refBest, _ := oracleFCBRefBestForXPrime(&xPrime, &h, refC, 0)
		return refBest
	}

	var hIdentity [clpitch.SubframeLen]int16
	hIdentity[0] = 4096
	evalWithIdentityH := func(aResidual, aX *[11]int16) bool {
		var r, x, v, y, xPrime [clpitch.SubframeLen]int16
		lpResidualSubframe(sFrame, aResidual, &e.lpResidualMemQ, &r)
		oracleTargetSignalVariant(aX, &r, &e.swMemErr, oracleGamma075, &x)
		var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
		copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
		copy(excSearch[clpitch.PitchMaxInt:], r[:])
		exc := excSearch[:]
		clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
		gpUnq := clpitch.GpAndY(&x, &v, &hIdentity, &y)
		fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)
		refBest, _ := oracleFCBRefBestForXPrime(&xPrime, &hIdentity, refC, 0)
		return refBest
	}

	return map[string]bool{
		"baseline-quant-gamma075": eval(aHat, aHat, oracleGamma075),
		"unquant-residual-only":   eval(&e.aQ12Latest, aHat, oracleGamma075),
		"unquant-xh-only":         eval(aHat, &e.aQ12Latest, oracleGamma075),
		"unquant-all":             eval(&e.aQ12Latest, &e.aQ12Latest, oracleGamma075),
		"no-gamma-quant":          eval(aHat, aHat, oracleGammaNone),
		"no-gamma-unquant":        eval(&e.aQ12Latest, &e.aQ12Latest, oracleGammaNone),
		"identity-h-quant-x":      evalWithIdentityH(aHat, aHat),
		"identity-h-unquant-x":    evalWithIdentityH(&e.aQ12Latest, &e.aQ12Latest),
	}
}

func TestOracleHCenter_FCBXHVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	names := []string{
		"baseline-quant-gamma075",
		"unquant-residual-only",
		"unquant-xh-only",
		"unquant-all",
		"no-gamma-quant",
		"no-gamma-unquant",
		"identity-h-quant-x",
		"identity-h-unquant-x",
	}
	sf1 := map[string]int{}
	sf2 := map[string]int{}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, _, _, _, refC2, _, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for name, ok := range diagnoseFCBXHVariants(enc, 0, int16(refInt1), int8(refFrac1), refC1) {
			if ok {
				sf1[name]++
			}
		}
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		for name, ok := range diagnoseFCBXHVariants(enc, 1, int16(refInt2), int8(refFrac2), refC2) {
			if ok {
				sf2[name]++
			}
		}
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	for _, name := range names {
		t.Logf("x/h variant %s sf1 ref-position-best %d/%d %.2f%% sf2 %d/%d %.2f%%",
			name,
			sf1[name], totalFrames, 100*float64(sf1[name])/float64(totalFrames),
			sf2[name], totalFrames, 100*float64(sf2[name])/float64(totalFrames))
	}
}

type oracleFCBFieldError struct {
	prodCost        int64
	refFieldCost    int64
	refBestGainCost int64
}

type oracleFCBFieldErrorStats struct {
	total          int
	refFieldLower  int
	refBestLower   int
	refFieldTie    int
	refBestTie     int
	sumProd        int64
	sumRefField    int64
	sumRefBestGain int64
}

func (s *oracleFCBFieldErrorStats) add(e oracleFCBFieldError) {
	s.total++
	s.sumProd += e.prodCost
	s.sumRefField += e.refFieldCost
	s.sumRefBestGain += e.refBestGainCost
	switch {
	case e.refFieldCost < e.prodCost:
		s.refFieldLower++
	case e.refFieldCost == e.prodCost:
		s.refFieldTie++
	}
	switch {
	case e.refBestGainCost < e.prodCost:
		s.refBestLower++
	case e.refBestGainCost == e.prodCost:
		s.refBestTie++
	}
}

func logFCBFieldError(t *testing.T, label string, s oracleFCBFieldErrorStats) {
	t.Helper()
	t.Logf("%s: ref-fields lower %d/%d %.2f%% tie %d/%d %.2f%% ref-C/S+best-gain lower %d/%d %.2f%% tie %d/%d %.2f%%",
		label,
		s.refFieldLower, s.total, 100*float64(s.refFieldLower)/float64(s.total),
		s.refFieldTie, s.total, 100*float64(s.refFieldTie)/float64(s.total),
		s.refBestLower, s.total, 100*float64(s.refBestLower)/float64(s.total),
		s.refBestTie, s.total, 100*float64(s.refBestTie)/float64(s.total))
	t.Logf("%s: summed cost production=%d ref-fields=%d ref-C/S+best-gain=%d",
		label, s.sumProd, s.sumRefField, s.sumRefBestGain)
}

func diagnoseFCBReferenceFieldError(e *Encoder, sub int, forcedLag int16, forcedFrac int8, refC, refS, refGA, refGB uint16) oracleFCBFieldError {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	exc := excSearch[:]
	clpitch.AdaptiveVector(exc, forcedLag, forcedFrac, &v)
	gpUnq := clpitch.GpAndY(&x, &v, &h, &y)

	var xPrime [clpitch.SubframeLen]int16
	fcbsearch.AdjustedTarget(&x, &y, gpUnq, &xPrime)
	var d [clpitch.SubframeLen]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)
	var signs [clpitch.SubframeLen]int16
	var dAbs [clpitch.SubframeLen]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	var phi [clpitch.SubframeLen][clpitch.SubframeLen]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)
	var prodPos [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &prodPos, &sumOut)

	var prodC [clpitch.SubframeLen]int16
	fcbsearch.BuildCode(&prodPos, &signs, forcedLag, e.prevGpQ14, &prodC)
	var prodZ [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&prodC, &h, &prodZ)
	prodPred := gainquant.PredictedGcQ12(&e.pastQuaEn, &prodC)
	prodGA, prodGB, prodGP, _ := gainquant.SearchConjugate(&x, &y, &prodZ, prodPred)
	prodGP = gainquant.Tame(prodGP, &e.oldExc)
	_, prodGCMant, prodGCExp := gainquant.Reconstruct(&e.pastQuaEn, &prodC, prodGA, prodGB)
	prodGC := mantExpToQ12(prodGCMant, prodGCExp)

	var refCode [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: uint8(refS)}, int(forcedLag), e.prevGpQ14, &refCode)
	var refZ [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&refCode, &h, &refZ)
	refGAPhys := tables.GainImap1[refGA]
	refGBPhys := tables.GainImap2[refGB]
	refGP, refGCMant, refGCExp := gainquant.Reconstruct(&e.pastQuaEn, &refCode, refGAPhys, refGBPhys)
	refGP = gainquant.Tame(refGP, &e.oldExc)
	refGC := mantExpToQ12(refGCMant, refGCExp)

	refPred := gainquant.PredictedGcQ12(&e.pastQuaEn, &refCode)
	bestRefGA, bestRefGB, bestRefGP, _ := gainquant.SearchConjugate(&x, &y, &refZ, refPred)
	bestRefGP = gainquant.Tame(bestRefGP, &e.oldExc)
	_, bestRefGCMant, bestRefGCExp := gainquant.Reconstruct(&e.pastQuaEn, &refCode, bestRefGA, bestRefGB)
	bestRefGC := mantExpToQ12(bestRefGCMant, bestRefGCExp)

	return oracleFCBFieldError{
		prodCost:        oracleFCBWeightedErrorCost(&x, &y, &prodZ, prodGP, prodGC),
		refFieldCost:    oracleFCBWeightedErrorCost(&x, &y, &refZ, refGP, refGC),
		refBestGainCost: oracleFCBWeightedErrorCost(&x, &y, &refZ, bestRefGP, bestRefGC),
	}
}

func oracleFCBWeightedErrorCost(x, y, z *[clpitch.SubframeLen]int16, gpQ14 int16, gcQ12 int32) int64 {
	var cost int64
	for n := 0; n < clpitch.SubframeLen; n++ {
		gpY := (int32(gpQ14) * int32(y[n])) >> 14
		gcZ := int32((int64(gcQ12) * int64(z[n])) >> 12)
		err := int64(int32(x[n]) - gpY - gcZ)
		cost += err * err
	}
	return cost
}

func TestOracleHCenter_FCBReferenceFieldErrorDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var sf1, sf2 oracleFCBFieldErrorStats
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		sf1.add(diagnoseFCBReferenceFieldError(enc, 0, int16(refInt1), int8(refFrac1), refC1, refS1, refGA1, refGB1))
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		sf2.add(diagnoseFCBReferenceFieldError(enc, 1, int16(refInt2), int8(refFrac2), refC2, refS2, refGA2, refGB2))
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	logFCBFieldError(t, "forced-ref-pitch fcb reference-field sf1", sf1)
	logFCBFieldError(t, "forced-ref-pitch fcb reference-field sf2", sf2)
}

type oracleLSPBoundaryStats struct {
	total     int
	l0Hit     int
	l1Hit     int
	l2Hit     int
	l3Hit     int
	allHit    int
	sf1Exact  int
	sf2Exact  int
	sf1Abs    int64
	sf2Abs    int64
	sf1Max    int
	sf2Max    int
	firstMiss int
	firstGot  [4]uint8
	firstWant [4]uint8
	firstSet  bool
}

func (s *oracleLSPBoundaryStats) add(got, want lsp.Indices, gotA1, gotA2, wantA1, wantA2 *[11]int16) {
	s.total++
	if got.L0 == want.L0 {
		s.l0Hit++
	}
	if got.L1 == want.L1 {
		s.l1Hit++
	}
	if got.L2 == want.L2 {
		s.l2Hit++
	}
	if got.L3 == want.L3 {
		s.l3Hit++
	}
	if got == want {
		s.allHit++
	}
	if !s.firstSet && got != want {
		s.firstMiss = s.total - 1
		s.firstGot = [4]uint8{got.L0, got.L1, got.L2, got.L3}
		s.firstWant = [4]uint8{want.L0, want.L1, want.L2, want.L3}
		s.firstSet = true
	}
	if gotA1 == wantA1 {
		s.sf1Exact++
	}
	if gotA2 == wantA2 {
		s.sf2Exact++
	}
	for i := 0; i < 11; i++ {
		d1 := absInt(int(gotA1[i]) - int(wantA1[i]))
		d2 := absInt(int(gotA2[i]) - int(wantA2[i]))
		s.sf1Abs += int64(d1)
		s.sf2Abs += int64(d2)
		if d1 > s.sf1Max {
			s.sf1Max = d1
		}
		if d2 > s.sf2Max {
			s.sf2Max = d2
		}
	}
}

func logLSPBoundary(t *testing.T, label string, s oracleLSPBoundaryStats) {
	t.Helper()
	t.Logf("%s LSP fields: L0 %d/%d %.2f%% L1 %d/%d %.2f%% L2 %d/%d %.2f%% L3 %d/%d %.2f%% all %d/%d %.2f%%",
		label,
		s.l0Hit, s.total, 100*float64(s.l0Hit)/float64(s.total),
		s.l1Hit, s.total, 100*float64(s.l1Hit)/float64(s.total),
		s.l2Hit, s.total, 100*float64(s.l2Hit)/float64(s.total),
		s.l3Hit, s.total, 100*float64(s.l3Hit)/float64(s.total),
		s.allHit, s.total, 100*float64(s.allHit)/float64(s.total))
	t.Logf("%s aHat: sf1 exact %d/%d %.2f%% meanAbs %.2f maxAbs %d sf2 exact %d/%d %.2f%% meanAbs %.2f maxAbs %d",
		label,
		s.sf1Exact, s.total, 100*float64(s.sf1Exact)/float64(s.total),
		float64(s.sf1Abs)/float64(s.total*11), s.sf1Max,
		s.sf2Exact, s.total, 100*float64(s.sf2Exact)/float64(s.total),
		float64(s.sf2Abs)/float64(s.total*11), s.sf2Max)
	if s.firstSet {
		t.Logf("%s first LSP miss frame %d got=(%d,%d,%d,%d) want=(%d,%d,%d,%d)",
			label, s.firstMiss,
			s.firstGot[0], s.firstGot[1], s.firstGot[2], s.firstGot[3],
			s.firstWant[0], s.firstWant[1], s.firstWant[2], s.firstWant[3])
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestOracleHCenter_PITCHLSPUpstreamBoundaryDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var boundary oracleLSPBoundaryStats
	var prodSurfaceSF1, prodSurfaceSF2 oracleFCBFieldErrorStats
	var refLSPSurfaceSF1, refLSPSurfaceSF2 oracleFCBFieldErrorStats
	prodEnc := NewEncoder()
	refLSPEnc := NewEncoder()
	var refLSPDec lsp.Decoder
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192(bitFrame)
		refIdx := lsp.Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}
		refA1, refA2 := refLSPDec.Decode(refIdx)

		gotIdx, err := prodEnc.lpcStep(pcm[:])
		if err != nil {
			t.Fatalf("production frame %d: lpcStep: %v", f, err)
		}
		boundary.add(gotIdx, refIdx, &prodEnc.aHatSF1, &prodEnc.aHatSF2, &refA1, &refA2)
		_ = prodEnc.openloopStep()

		if _, err := refLSPEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("ref-lsp frame %d: lpcStep: %v", f, err)
		}
		refLSPEnc.aHatSF1 = refA1
		refLSPEnc.aHatSF2 = refA2
		_ = refLSPEnc.openloopStep()

		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		prodSurfaceSF1.add(diagnoseFCBReferenceFieldError(prodEnc, 0, int16(refInt1), int8(refFrac1), refC1, refS1, refGA1, refGB1))
		forceClosedLoopStep(prodEnc, 0, int16(refInt1), int8(refFrac1), refP1)
		prodSurfaceSF2.add(diagnoseFCBReferenceFieldError(prodEnc, 1, int16(refInt2), int8(refFrac2), refC2, refS2, refGA2, refGB2))
		forceClosedLoopStep(prodEnc, 1, int16(refInt2), int8(refFrac2), refP2)

		refLSPSurfaceSF1.add(diagnoseFCBReferenceFieldError(refLSPEnc, 0, int16(refInt1), int8(refFrac1), refC1, refS1, refGA1, refGB1))
		forceClosedLoopStep(refLSPEnc, 0, int16(refInt1), int8(refFrac1), refP1)
		refLSPSurfaceSF2.add(diagnoseFCBReferenceFieldError(refLSPEnc, 1, int16(refInt2), int8(refFrac2), refC2, refS2, refGA2, refGB2))
		forceClosedLoopStep(refLSPEnc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	logLSPBoundary(t, "PITCH vector upstream", boundary)
	logFCBFieldError(t, "production-LSP surface sf1", prodSurfaceSF1)
	logFCBFieldError(t, "production-LSP surface sf2", prodSurfaceSF2)
	logFCBFieldError(t, "forced-reference-LSP surface sf1", refLSPSurfaceSF1)
	logFCBFieldError(t, "forced-reference-LSP surface sf2", refLSPSurfaceSF2)
}

func forceReferenceFieldStep(e *Encoder, sub int, intLag int16, frac int8, pitchCode uint16, refC, refS, refGA, refGB uint16) {
	var aHat *[11]int16
	if sub == 0 {
		aHat = &e.aHatSF1
	} else {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, v, y [clpitch.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	clpitch.TargetSignal(aHat, &r, &e.swMemErr, &x)
	clpitch.ImpulseResponse(aHat, &h)

	var excSearch [clpitch.PitchMaxInt + clpitch.SubframeLen]int16
	copy(excSearch[:clpitch.PitchMaxInt], e.oldExc[len(e.oldExc)-clpitch.PitchMaxInt:])
	copy(excSearch[clpitch.PitchMaxInt:], r[:])
	clpitch.AdaptiveVector(excSearch[:], intLag, frac, &v)
	clpitch.GpAndY(&x, &v, &h, &y)

	var c [clpitch.SubframeLen]int16
	fcb.Decode(fcb.Indices{Positions: refC, Signs: uint8(refS)}, int(intLag), e.prevGpQ14, &c)
	var z [clpitch.SubframeLen]int16
	fcbsearch.FilterCode(&c, &h, &z)

	gaPhys := tables.GainImap1[refGA]
	gbPhys := tables.GainImap2[refGB]
	gpQ14, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	gpQ14 = gainquant.Tame(gpQ14, &e.oldExc)
	gcQ12 := mantExpToQ12(gcMantQ14, gcExp)

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = uint8(pitchCode)
		e.p0 = clpitch.EncodeP0(e.p1)
		e.c1 = refC
		e.s1 = uint8(refS)
		e.ga1 = uint8(refGA)
		e.gb1 = uint8(refGB)
	} else {
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = uint8(pitchCode)
		e.c2 = refC
		e.s2 = uint8(refS)
		e.ga2 = uint8(refGA)
		e.gb2 = uint8(refGB)
	}

	for n := 30; n < clpitch.SubframeLen; n++ {
		gpY := (int32(gpQ14) * int32(y[n])) >> 14
		gcZ := int32((int64(gcQ12) * int64(z[n])) >> 12)
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-clpitch.SubframeLen], e.oldExc[clpitch.SubframeLen:])
	base := len(e.oldExc) - clpitch.SubframeLen
	for n := 0; n < clpitch.SubframeLen; n++ {
		gpV := (int32(gpQ14) * int32(v[n])) >> 14
		gcC := int32((int64(gcQ12) * int64(c[n])) >> 13)
		e.oldExc[base+n] = fixed.Saturate(gpV + gcC)
	}

	gammaCQ13 := tables.GainGBK1[gaPhys][1] + tables.GainGBK2[gbPhys][1]
	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)
	e.prevGpQ14 = gpQ14
	e.prevTaming = false
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func TestOracleHCenter_PITCHReferenceTrajectoryDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	type variant struct {
		name      string
		enc       *Encoder
		refLSP    bool
		refFields bool
		lspDec    lsp.Decoder
		sf1       oracleFCBFieldErrorStats
		sf2       oracleFCBFieldErrorStats
	}
	variants := []*variant{
		{name: "production-commit", enc: NewEncoder()},
		{name: "reference-fields-commit", enc: NewEncoder(), refFields: true},
		{name: "reference-lsp+fields-commit", enc: NewEncoder(), refLSP: true, refFields: true},
	}

	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for _, v := range variants {
			enc := v.enc
			if _, err := enc.lpcStep(pcm[:]); err != nil {
				t.Fatalf("%s frame %d: lpcStep: %v", v.name, f, err)
			}
			if v.refLSP {
				a1, a2 := v.lspDec.Decode(lsp.Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3})
				enc.aHatSF1 = a1
				enc.aHatSF2 = a2
			}
			_ = enc.openloopStep()

			v.sf1.add(diagnoseFCBReferenceFieldError(enc, 0, int16(refInt1), int8(refFrac1), refC1, refS1, refGA1, refGB1))
			if v.refFields {
				forceReferenceFieldStep(enc, 0, int16(refInt1), int8(refFrac1), refP1, refC1, refS1, refGA1, refGB1)
			} else {
				forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
			}

			v.sf2.add(diagnoseFCBReferenceFieldError(enc, 1, int16(refInt2), int8(refFrac2), refC2, refS2, refGA2, refGB2))
			if v.refFields {
				forceReferenceFieldStep(enc, 1, int16(refInt2), int8(refFrac2), refP2, refC2, refS2, refGA2, refGB2)
			} else {
				forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
			}
		}
	}

	for _, v := range variants {
		logFCBFieldError(t, v.name+" sf1", v.sf1)
		logFCBFieldError(t, v.name+" sf2", v.sf2)
	}
}

func TestOracleHCenter_FCBSearchInputVariantDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	names := []string{"baseline-x-gpY", "no-adaptive-x", "zero-swmem-x-gpY", "zero-oldexc-v", "residual-direct-r"}
	sf1 := make(map[string]*fcbSearchInputVariantStats, len(names))
	sf2 := make(map[string]*fcbSearchInputVariantStats, len(names))
	for _, name := range names {
		sf1[name] = &fcbSearchInputVariantStats{}
		sf2[name] = &fcbSearchInputVariantStats{}
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, _, _, refC2, refS2, _, _ := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		for name, result := range diagnoseFCBSearchInputVariants(enc, 0, int16(refInt1), int8(refFrac1), refC1, refS1) {
			sf1[name].add(result[0], result[1])
		}
		forceClosedLoopStep(enc, 0, int16(refInt1), int8(refFrac1), refP1)
		for name, result := range diagnoseFCBSearchInputVariants(enc, 1, int16(refInt2), int8(refFrac2), refC2, refS2) {
			sf2[name].add(result[0], result[1])
		}
		forceClosedLoopStep(enc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	for _, name := range names {
		logFCBSearchInputVariant(t, "subframe1 "+name, *sf1[name])
	}
	for _, name := range names {
		logFCBSearchInputVariant(t, "subframe2 "+name, *sf2[name])
	}
}

func TestOracleHCenter_FCBCommitSplitDiagnostic(t *testing.T) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}

	var prodSF1, prodSF2, forcedSF1, forcedSF2, refCodeSF1, refCodeSF2 fcbCommitSplitStats
	prodEnc := NewEncoder()
	forcedEnc := NewEncoder()
	refCodeEnc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		refP1, _, refP2 := oracleExtractPitchBitsFromG192(bitFrame)
		refC1, refS1, refGA1, refGB1, refC2, refS2, refGA2, refGB2 := oracleExtractFCBBitsFromG192(bitFrame)
		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(refP1))
		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(refP2), refInt1)

		if _, err := prodEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("production frame %d: lpcStep: %v", f, err)
		}
		_ = prodEnc.openloopStep()
		prodSF1.add(diagnoseFCBCommitTap(prodEnc, 0, false, 0, 0), refC1, refS1, refGA1, refGB1)
		_, _ = prodEnc.closedloopStep(0)
		prodSF2.add(diagnoseFCBCommitTap(prodEnc, 1, false, 0, 0), refC2, refS2, refGA2, refGB2)
		_, _ = prodEnc.closedloopStep(1)

		if _, err := forcedEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("forced frame %d: lpcStep: %v", f, err)
		}
		_ = forcedEnc.openloopStep()
		forcedSF1.add(diagnoseFCBCommitTap(forcedEnc, 0, true, int16(refInt1), int8(refFrac1)), refC1, refS1, refGA1, refGB1)
		forceClosedLoopStep(forcedEnc, 0, int16(refInt1), int8(refFrac1), refP1)
		forcedSF2.add(diagnoseFCBCommitTap(forcedEnc, 1, true, int16(refInt2), int8(refFrac2)), refC2, refS2, refGA2, refGB2)
		forceClosedLoopStep(forcedEnc, 1, int16(refInt2), int8(refFrac2), refP2)

		if _, err := refCodeEnc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("ref-code frame %d: lpcStep: %v", f, err)
		}
		_ = refCodeEnc.openloopStep()
		refCodeSF1.add(diagnoseFCBCommitTapWithReferenceCode(refCodeEnc, 0, int16(refInt1), int8(refFrac1), refC1, refS1), refC1, refS1, refGA1, refGB1)
		forceClosedLoopStep(refCodeEnc, 0, int16(refInt1), int8(refFrac1), refP1)
		refCodeSF2.add(diagnoseFCBCommitTapWithReferenceCode(refCodeEnc, 1, int16(refInt2), int8(refFrac2), refC2, refS2), refC2, refS2, refGA2, refGB2)
		forceClosedLoopStep(refCodeEnc, 1, int16(refInt2), int8(refFrac2), refP2)
	}

	logFCBCommitSplit(t, "production-pitch subframe1", prodSF1)
	logFCBCommitSplit(t, "production-pitch subframe2", prodSF2)
	logFCBCommitSplit(t, "forced-ref-pitch subframe1", forcedSF1)
	logFCBCommitSplit(t, "forced-ref-pitch subframe2", forcedSF2)
	logFCBCommitSplit(t, "forced-ref-pitch+ref-code subframe1", refCodeSF1)
	logFCBCommitSplit(t, "forced-ref-pitch+ref-code subframe2", refCodeSF2)
}

func TestOracleHCenter_RangeMarginSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	type candidate struct {
		lift    float64
		margin2 float64
		margin3 float64
		tol     int
	}
	margins := []float64{0.70, 0.80, 0.85, 0.90, 0.95, 1.00, 1.05, 1.10, 1.15, 1.20, 1.30, 1.50, 2.00}
	var candidates []candidate
	for _, lift := range []float64{2.00, 3.00, 4.00, 6.00, 8.00} {
		for _, margin2 := range margins {
			for _, margin3 := range margins {
				for _, tol := range []int{2, 3, 4, 5, 6, 8, 10, 12, 15, 20} {
					candidates = append(candidates, candidate{lift: lift, margin2: margin2, margin3: margin3, tol: tol})
				}
			}
		}
	}
	stats := make([]oracleVariantStats, len(candidates))

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag64 := diagnoseOpenLoopFrame64(enc)
		_ = enc.openloopStep()
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		for i, cand := range candidates {
			got := oracleMergeDiag64WithRangeMargins(diag64, cand.lift, cand.margin2, cand.margin3, cand.tol)
			stats[i].add(row.Expected, got)
		}
	}

	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a := stats[order[i]]
		b := stats[order[j]]
		if a.exact != b.exact {
			return a.exact > b.exact
		}
		return a.w10 > b.w10
	})
	for rank := 0; rank < 12 && rank < len(order); rank++ {
		i := order[rank]
		cand := candidates[i]
		st := stats[i]
		t.Logf("rank %02d lift %.2f margin2 %.2f margin3 %.2f tol %d: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			rank+1, cand.lift, cand.margin2, cand.margin3, cand.tol,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_QuantizedOpenLoopVariantSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	stats := map[string]*oracleVariantStats{
		"unquantized-current": {},
		"quantized-sf1":       {},
		"quantized-sf2":       {},
	}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		current := diagnoseOpenLoopFrame64WithA(enc, &enc.aQ12Latest)
		sf1 := diagnoseOpenLoopFrame64WithA(enc, &enc.aHatSF1)
		sf2 := diagnoseOpenLoopFrame64WithA(enc, &enc.aHatSF2)
		_ = enc.openloopStep()
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		stats["unquantized-current"].add(row.Expected, oracleMergeDiag64(current))
		stats["quantized-sf1"].add(row.Expected, oracleMergeDiag64(sf1))
		stats["quantized-sf2"].add(row.Expected, oracleMergeDiag64(sf2))
	}
	for _, name := range []string{"unquantized-current", "quantized-sf1", "quantized-sf2"} {
		st := stats[name]
		t.Logf("%s: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			name,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_StatefulQuantizedOpenLoopVariantSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	stats := map[string]*oracleVariantStats{
		"stateful-unquantized": {},
		"stateful-sf1":         {},
		"stateful-sf2":         {},
	}
	var unqState, sf1State, sf2State oracleOpenLoopState64
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		s := (*[FrameSamples]int16)(enc.oldSpeech[120:200])
		unq := oracleAdvanceOpenLoopState64(s, &enc.aQ12Latest, &unqState)
		sf1 := oracleAdvanceOpenLoopState64(s, &enc.aHatSF1, &sf1State)
		sf2 := oracleAdvanceOpenLoopState64(s, &enc.aHatSF2, &sf2State)
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		stats["stateful-unquantized"].add(row.Expected, oracleMergeDiag64(unq))
		stats["stateful-sf1"].add(row.Expected, oracleMergeDiag64(sf1))
		stats["stateful-sf2"].add(row.Expected, oracleMergeDiag64(sf2))
	}
	for _, name := range []string{"stateful-unquantized", "stateful-sf1", "stateful-sf2"} {
		st := stats[name]
		t.Logf("%s: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			name,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_OpenLoopVariantSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	stats := map[string]*oracleVariantStats{
		"production-wide":  {},
		"legacy-saturated": {},
		"float-wide":       {},
		"no-high-range":    {},
	}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag := diagnoseOpenLoopFrame(enc)
		diag64 := diagnoseOpenLoopFrame64(enc)
		got := int(enc.openloopStep())
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		if got != row.Got {
			t.Skipf("%s is stale for the current open-loop definition: frame %d artifact got=%d encoder got=%d",
				oraclePath, f, row.Got, got)
		}
		stats["production-wide"].add(row.Expected, got)
		stats["legacy-saturated"].add(row.Expected, oracleMergeDiag(diag))
		stats["float-wide"].add(row.Expected, oracleMergeDiag64(diag64))
		stats["no-high-range"].add(row.Expected, oracleMergeNoHighDiag(diag))
	}
	for _, name := range []string{"production-wide", "legacy-saturated", "float-wide", "no-high-range"} {
		st := stats[name]
		t.Logf("%s: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			name,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_HigherRangeMarginSweep(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}

	margins := []float64{1.0, 1.02, 1.05, 1.08, 1.10, 1.15, 1.20, 1.30, 1.50, 2.0}
	stats := make([]oracleVariantStats, len(margins))
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag64 := diagnoseOpenLoopFrame64(enc)
		_ = enc.openloopStep()
		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		for i, margin := range margins {
			stats[i].add(row.Expected, oracleMergeDiag64WithMargin(diag64, margin))
		}
	}
	for i, margin := range margins {
		st := stats[i]
		t.Logf("non-submultiple margin %.2f: exact %d/%d %.2f%% ±10 %d %.2f%% | expected20..39 exact %d/%d %.2f%% ±10 %d %.2f%%",
			margin,
			st.exact, st.total, 100*float64(st.exact)/float64(st.total),
			st.w10, 100*float64(st.w10)/float64(st.total),
			st.lowExact, st.lowTotal, 100*float64(st.lowExact)/float64(st.lowTotal),
			st.lowW10, 100*float64(st.lowW10)/float64(st.lowTotal))
	}
}

func TestOracleHCenter_LowRangeMismatchRangeWinners(t *testing.T) {
	const (
		oraclePath      = "testdata/oracle/pitch_top_open_loop.csv"
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		totalFrames     = 1835
	)

	rows, err := parseOracleFile(oraclePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("no PITCH/top_open_loop oracle artifact present")
		}
		t.Fatalf("parse %s: %v", oraclePath, err)
	}
	oracleByFrame := map[int]oracleRow{}
	for _, row := range rows {
		if row.Vector == "PITCH" && row.Field == "top_open_loop" {
			oracleByFrame[row.Frame] = row
		}
	}
	if len(oracleByFrame) == 0 {
		t.Skip("no PITCH/top_open_loop rows present")
	}

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read %s: %v", inPath, err)
	}
	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	logged := 0
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		diag := diagnoseOpenLoopFrame(enc)
		got := enc.openloopStep()

		row, ok := oracleByFrame[f]
		if !ok {
			continue
		}
		if int(got) != row.Got {
			t.Skipf("%s is stale for the current open-loop definition: frame %d artifact got=%d encoder got=%d",
				oraclePath, f, row.Got, got)
		}
		if row.Expected >= 20 && row.Expected <= 39 && row.Got >= 80 && row.Got <= 143 && logged < 16 {
			t.Logf("low-range long-delay mismatch frame=%d expected=%d got=%d delta=%+d r1=(lag=%d r=%d e=%d) r2=(lag=%d r=%d e=%d) r3=(lag=%d r=%d e=%d)",
				f, row.Expected, row.Got, row.Delta,
				diag.range1.lag, diag.range1.r, diag.range1.e,
				diag.range2.lag, diag.range2.r, diag.range2.e,
				diag.range3.lag, diag.range3.r, diag.range3.e)
			logged++
		}
	}
	if logged == 0 {
		t.Skip("no expected 20..39 / got 80..143 mismatches found")
	}
}
