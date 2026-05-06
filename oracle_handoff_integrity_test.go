package g729

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOracleHandoff_LSPStructuralIntegrity(t *testing.T) {
	cases := []struct {
		name           string
		expectedPath   string
		gotPath        string
		expectedHeader []string
		gotHeader      []string
		keyColumns     int
		rows           int
	}{
		{
			name:           "lsp tables",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "lsp_tables_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "lsp_tables_got.csv"),
			expectedHeader: []string{"table", "selector", "tap", "row", "col", "expected"},
			gotHeader:      []string{"table", "selector", "tap", "row", "col", "got"},
			keyColumns:     5,
			rows:           1680,
		},
		{
			name:           "lsp predictor residual",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "lsp_predictor_residual_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "lsp_predictor_residual_got.csv"),
			expectedHeader: []string{"frame", "selector", "L1", "L2", "L3", "ref_selector", "ref_L1", "ref_L2", "ref_L3", "col", "expected"},
			gotHeader:      []string{"frame", "selector", "L1", "L2", "L3", "ref_selector", "ref_L1", "ref_L2", "ref_L3", "col", "got"},
			keyColumns:     10,
			rows:           22320,
		},
		{
			name:           "lsp frame0 vq",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "lsp_frame0_vq_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "lsp_frame0_vq_got.csv"),
			expectedHeader: []string{"field", "frame", "selector", "tap", "L1", "L2", "L3", "col", "expected"},
			gotHeader:      []string{"field", "frame", "selector", "tap", "L1", "L2", "L3", "col", "got"},
			keyColumns:     8,
			rows:           76,
		},
		{
			name:           "lsp frame0 source",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "lsp_frame0_source_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "lsp_frame0_source_got.csv"),
			expectedHeader: []string{"field", "frame", "col", "expected"},
			gotHeader:      []string{"field", "frame", "col", "got"},
			keyColumns:     3,
			rows:           8,
		},
		{
			name:           "lsp decision",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "lsp_decision_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "lsp_decision_got.csv"),
			expectedHeader: []string{"field", "frame", "tap", "L0", "L1", "L2", "L3", "col", "expected"},
			gotHeader:      []string{"field", "frame", "tap", "L0", "L1", "L2", "L3", "col", "got"},
			keyColumns:     8,
			rows:           1472,
		},
		{
			name:           "pitch closedloop search",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "pitch_closedloop_search_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "pitch_closedloop_search_got.csv"),
			expectedHeader: []string{"field", "frame", "sub", "index", "lag", "frac", "expected"},
			gotHeader:      []string{"field", "frame", "sub", "index", "lag", "frac", "got"},
			keyColumns:     6,
			rows:           3192,
		},
		{
			name:           "tame gain taming",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "tame_gain_taming_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "tame_gain_taming_got.csv"),
			expectedHeader: []string{"field", "frame", "sub", "index", "expected"},
			gotHeader:      []string{"field", "frame", "sub", "index", "got"},
			keyColumns:     4,
			rows:           8962,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectedKeys, filled := readAndCheckHandoffCSV(t, tc.expectedPath, tc.expectedHeader, tc.keyColumns, tc.rows, true)
			gotKeys, _ := readAndCheckHandoffCSV(t, tc.gotPath, tc.gotHeader, tc.keyColumns, tc.rows, false)
			if len(expectedKeys) != len(gotKeys) {
				t.Fatalf("key count: expected template=%d got=%d", len(expectedKeys), len(gotKeys))
			}
			for key := range expectedKeys {
				if !gotKeys[key] {
					t.Fatalf("key exists in expected template but not got: %s", key)
				}
			}
			t.Logf("%s: rows=%d expected-filled=%d", tc.name, tc.rows, filled)
		})
	}
}

func readAndCheckHandoffCSV(t *testing.T, path string, header []string, keyColumns, dataRows int, allowBlankValue bool) (map[string]bool, int) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(rows) != dataRows+1 {
		t.Fatalf("%s rows=%d, want %d data rows plus header", path, len(rows), dataRows)
	}
	if got := strings.Join(rows[0], ","); got != strings.Join(header, ",") {
		t.Fatalf("%s header=%q, want %q", path, got, strings.Join(header, ","))
	}

	keys := make(map[string]bool, dataRows)
	var filled int
	for line, row := range rows[1:] {
		if allowBlankValue && len(row) == len(header)-1 {
			row = append(row, "")
		}
		if len(row) != len(header) {
			t.Fatalf("%s line %d: columns=%d, want %d", path, line+2, len(row), len(header))
		}
		key := strings.Join(row[:keyColumns], ",")
		if keys[key] {
			t.Fatalf("%s line %d: duplicate key %s", path, line+2, key)
		}
		keys[key] = true

		value := row[len(row)-1]
		if value == "" {
			if allowBlankValue {
				continue
			}
			t.Fatalf("%s line %d: blank value in %s", path, line+2, header[len(header)-1])
		}
		if _, err := strconv.Atoi(value); err != nil {
			t.Fatalf("%s line %d: parse %s=%q: %v", path, line+2, header[len(header)-1], value, err)
		}
		filled++
	}
	if len(keys) != dataRows {
		t.Fatalf("%s unique keys=%d, want %d", path, len(keys), dataRows)
	}
	if len(keys) == 0 {
		t.Fatalf("%s has no keys", path)
	}
	if dataRows > 0 && filled > dataRows {
		t.Fatalf("%s filled=%d exceeds data rows=%d", path, filled, dataRows)
	}
	if !allowBlankValue && filled != dataRows {
		t.Fatalf("%s filled got values=%d, want %d", path, filled, dataRows)
	}
	if allowBlankValue && filled > 0 {
		t.Logf("%s has %d verifier-filled expected cells", path, filled)
	}
	return keys, filled
}

func TestOracleHandoff_LSPManifestMatchesCurrentFiles(t *testing.T) {
	manifestPath := filepath.Join("testdata", "oracle", "handoff", "HANDOFF_MANIFEST.md")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"`lsp_tables_expected_template.csv` | `table,selector,tap,row,col,expected` | 1680",
		"`lsp_tables_got.csv` | `table,selector,tap,row,col,got` | 1680",
		"`lsp_predictor_residual_expected_template.csv` | `frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,expected` | 22320",
		"`lsp_predictor_residual_got.csv` | `frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,got` | 22320",
		"`lsp_frame0_vq_expected_template.csv` | `field,frame,selector,tap,L1,L2,L3,col,expected` | 76",
		"`lsp_frame0_vq_got.csv` | `field,frame,selector,tap,L1,L2,L3,col,got` | 76",
		"`lsp_frame0_source_expected_template.csv` | `field,frame,col,expected` | 8",
		"`lsp_frame0_source_got.csv` | `field,frame,col,got` | 8",
		"`lsp_decision_expected_template.csv` | `field,frame,tap,L0,L1,L2,L3,col,expected` | 1472",
		"`lsp_decision_got.csv` | `field,frame,tap,L0,L1,L2,L3,col,got` | 1472",
		"`pitch_closedloop_search_expected_template.csv` | `field,frame,sub,index,lag,frac,expected` | 3192",
		"`pitch_closedloop_search_got.csv` | `field,frame,sub,index,lag,frac,got` | 3192",
		"`tame_gain_taming_expected_template.csv` | `field,frame,sub,index,expected` | 8962",
		"`tame_gain_taming_got.csv` | `field,frame,sub,index,got` | 8962",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest missing row/header entry %q", want)
		}
	}
	if !strings.Contains(text, "pre-fill hashes") {
		t.Fatalf("manifest should identify expected-template hashes as pre-fill hashes")
	}
	if !strings.Contains(text, "strict compare commands documented in `README.md`") {
		t.Fatalf("manifest should point verifier output to README strict compare commands")
	}
}
