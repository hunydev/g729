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
