package g729

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
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
		{
			name:           "encoder closedloop stage",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "encoder_closedloop_stage_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "encoder_closedloop_stage_got.csv"),
			expectedHeader: []string{"field", "frame", "sub", "index", "lag", "frac", "expected"},
			gotHeader:      []string{"field", "frame", "sub", "index", "lag", "frac", "got"},
			keyColumns:     6,
			rows:           100848,
		},
		{
			name:           "fcb tree search",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_got.csv"),
			expectedHeader: []string{"field", "frame", "sub", "index", "expected"},
			gotHeader:      []string{"field", "frame", "sub", "index", "got"},
			keyColumns:     4,
			rows:           10194,
		},
		{
			name:           "fcb tree search user audio",
			expectedPath:   filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_user_audio_expected_template.csv"),
			gotPath:        filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_user_audio_got.csv"),
			expectedHeader: []string{"field", "frame", "sub", "index", "expected"},
			gotHeader:      []string{"field", "frame", "sub", "index", "got"},
			keyColumns:     4,
			rows:           10194,
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
		"`encoder_closedloop_stage_expected_template.csv` | `field,frame,sub,index,lag,frac,expected` | 100848",
		"`encoder_closedloop_stage_got.csv` | `field,frame,sub,index,lag,frac,got` | 100848",
		"`fcb_tree_search_expected_template.csv` | `field,frame,sub,index,expected` | 10194",
		"`fcb_tree_search_got.csv` | `field,frame,sub,index,got` | 10194",
		"`fcb_tree_search_user_audio_expected_template.csv` | `field,frame,sub,index,expected` | 10194",
		"`fcb_tree_search_user_audio_got.csv` | `field,frame,sub,index,got` | 10194",
		"`DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md` | n/a | n/a",
		"`decoder_itu_frame0_hp_input_inverse_expected_template.csv` | `source,frame,sub,field,index,expected` | 480",
		"`DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md` | n/a | n/a",
		"`decoder_tame_pre_acb_history_expected_template.csv` | `source,frame,sub,field,index,expected` | 153",
		"`DECODER_TAME_EXCITATION_HISTORY_PROMPT.md` | n/a | n/a",
		"`decoder_tame_excitation_history_expected_template.csv` | `source,frame,sub,field,index,expected` | 9360",
		"`DECODER_SUPPORT_TABLES_PROMPT.md` | n/a | n/a",
		"`decoder_support_tables_expected_template.csv` | `table,row,col,expected` | 264",
		"`create_verifier_bundle.sh` | n/a | n/a",
		"`validate_verifier_output.sh` | n/a | n/a",
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

func TestOracleHandoff_ManifestHashesMatchCurrentFiles(t *testing.T) {
	manifestPath := filepath.Join("testdata", "oracle", "handoff", "HANDOFF_MANIFEST.md")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(data)

	files := parseManifestHashes(t, text, "## Files", "## Verifier-filled Files")
	filled := parseManifestHashes(t, text, "## Verifier-filled Files", "## Currently Unfilled Files")
	if len(files) == 0 {
		t.Fatal("manifest files table has no hash rows")
	}
	if len(filled) == 0 {
		t.Fatal("manifest verifier-filled table has no hash rows")
	}

	for name, want := range files {
		if strings.HasSuffix(name, "_expected_template.csv") {
			if _, ok := filled[name]; ok {
				// The top table records the pre-fill template hash; the
				// verifier-filled table records the current post-fill hash.
				continue
			}
		}
		assertManifestFileHash(t, name, want)
	}
	for name, want := range filled {
		assertManifestFileHash(t, name, want)
	}
}

func TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles(t *testing.T) {
	manifestPath := filepath.Join("testdata", "oracle", "handoff", "HANDOFF_MANIFEST.md")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(data)

	counts := parseManifestBlankCounts(t, text, "## Currently Unfilled Files", "## Verification Commands")
	wantFiles := map[string]bool{
		"encoder_closedloop_stage_expected_template.csv":        true,
		"decoder_tame_pre_acb_history_expected_template.csv":    true,
		"decoder_tame_excitation_history_expected_template.csv": true,
		"decoder_support_tables_expected_template.csv":          true,
	}
	if len(counts) != len(wantFiles) {
		t.Fatalf("manifest unfilled file count=%d, want %d", len(counts), len(wantFiles))
	}
	for name := range wantFiles {
		want, ok := counts[name]
		if !ok {
			t.Fatalf("manifest missing unfilled count for %s", name)
		}
		rows, blanks := countBlankFinalColumnCells(t, filepath.Join("testdata", "oracle", "handoff", name))
		if blanks != want {
			t.Fatalf("%s blank expected cells=%d, manifest says %d", name, blanks, want)
		}
		if blanks == 0 {
			t.Fatalf("%s is listed as currently unfilled but has no blanks", name)
		}
		if blanks > rows {
			t.Fatalf("%s blanks=%d exceeds rows=%d", name, blanks, rows)
		}
	}
}

func TestOracleHandoff_RemainingConformancePromptMatchesBlankTemplates(t *testing.T) {
	promptPath := filepath.Join("testdata", "oracle", "handoff", "REMAINING_CONFORMANCE_VERIFIER_PROMPT.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read remaining conformance prompt: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"encoder_closedloop_stage_expected_template.csv",
		"encoder_closedloop_stage_got.csv",
		"ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md",
		"Completed Focused FCB Templates",
		"fcb_tree_search_expected_template.csv",
		"fcb_tree_search_user_audio_expected_template.csv",
		"G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1",
		"G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1",
		"G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("remaining conformance prompt missing %q", want)
		}
	}
	for _, stale := range []string{
		"lsp_decision_expected_template.csv",
		"tame_gain_taming_expected_template.csv",
	} {
		if strings.Contains(text, stale) {
			t.Fatalf("remaining conformance prompt still points to filled template %q", stale)
		}
	}
}

func TestOracleHandoff_READMEDocumentsActiveVerifierBundle(t *testing.T) {
	readmePath := filepath.Join("testdata", "oracle", "handoff", "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read handoff README: %v", err)
	}
	text := string(data)
	normalized := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{
		"Active verifier bundle",
		"sh testdata/oracle/handoff/create_verifier_bundle.sh",
		"does not contain source code or external implementation material",
		"archive hash is stable for a fixed set of input files",
		"refuses to build if the remaining",
		"template already has verifier-filled cells",
		"repo-local helper uses deterministic tar/gzip options",
		"--sort=name",
		"--mtime",
		"--numeric-owner",
		"gzip -n",
		"HANDOFF_MANIFEST.md",
		"README.md",
		"EXTERNAL_VERIFIER_REQUEST.md",
		"create_verifier_bundle.sh",
		"validate_verifier_output.sh",
		"REMAINING_CONFORMANCE_VERIFIER_PROMPT.md",
		"FCB_TREE_SEARCH_VERIFIER_PROMPT.md",
		"FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md",
		"ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md",
		"fcb_tree_search_expected_template.csv",
		"fcb_tree_search_got.csv",
		"fcb_tree_search_user_audio_expected_template.csv",
		"fcb_tree_search_user_audio_got.csv",
		"encoder_closedloop_stage_expected_template.csv",
		"encoder_closedloop_stage_got.csv",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("handoff README missing active bundle detail %q", want)
		}
	}
}

func TestOracleHandoff_READMEDocumentsFilledVerifierIntake(t *testing.T) {
	readmePath := filepath.Join("testdata", "oracle", "handoff", "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read handoff README: %v", err)
	}
	normalized := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		"Filled verifier output intake",
		"verifier-returned numeric",
		"validate them before copying",
		"validate_verifier_output.sh",
		"rejects unexpected files",
		"symlinked files",
		"changed headers",
		"changed row counts",
		"changed key columns",
		"non-numeric",
		"validation-only by default",
		"G729_APPLY_VERIFIER_OUTPUT=1",
		"copy validated files into their exact template paths",
		"Do not run any",
		"command after copying filled templates",
		"Do not run",
		"for incoming verifier output",
		"helper is for outgoing verifier bundles",
		"refuses filled remaining-blank templates by default",
		"G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1",
		"G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1",
		"G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1",
		"TestOracleHandoff_CompareFCBTreeSearchHandoff",
		"G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1",
		"G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1",
		"G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1",
		"TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff",
		"G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1",
		"G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1",
		"G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1",
		"TestOracleHandoff_CompareEncoderClosedLoopStageHandoff",
		"G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1",
		"G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1",
		"G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1",
		"TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse",
		"G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1",
		"G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY=1",
		"G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY=1",
		"TestOracleHandoff_CompareDecoderTAMEPreACBHistory",
		"G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1",
		"G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY=1",
		"G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY=1",
		"TestOracleHandoff_CompareDecoderTAMEExcitationHistory",
		"G729_COMPARE_DECODER_SUPPORT_TABLES=1",
		"G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES=1",
		"G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES=1",
		"TestOracleHandoff_CompareDecoderSupportTables",
		"update",
		"and the audit docs",
		"TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles",
		"expected to fail",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("handoff README missing filled verifier intake detail %q", want)
		}
	}
}

func TestOracleHandoff_GoalCompletionAuditMapsObjectiveToArtifacts(t *testing.T) {
	auditPath := filepath.Join("docs", "superpowers", "diagnostics", "2026-05-10-goal-completion-audit.md")
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read goal completion audit: %v", err)
	}
	text := string(data)
	normalized := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{
		"Prompt-to-Artifact Checklist",
		"AGENTS.md",
		"testdata/external/user_quality_audio.m4a",
		"testdata/external/user_quality_input.m4a",
		"third-party/g729-compare-web",
		"curl -fsS http://127.0.0.1:8000/healthz",
		"curl -fsS -F 'file=@testdata/external/user_quality_audio.m4a'",
		"jq 'del(.audio, .downloads)'",
		"EncoderProfileCore",
		"EncoderProfileQuality",
		"Core -> ffmpeg decode | 5.21 | 4.37 | 0.8383 | 0.9007 | 32768 | 2",
		"Core -> local decode | 5.03 | 4.14 | 0.8300 | 0.8820 | 31164 | 0",
		"Core -> ffmpeg decode | 5.09 | 3.78 | 0.8346 | 0.9160 | 26283 | 0",
		"[292 293 294]",
		"2026-05-10-closedloop-pitch-pdf-audit.md",
		"2026-05-10-fcb-search-pdf-audit.md",
		"2026-05-10-gain-preselect-pdf-audit.md",
		"2026-05-10-gain-reconstruction-pdf-audit.md",
		"2026-05-10-state-commit-pdf-audit.md",
		"go test ./... -count=1",
		"G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a",
		"G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_input.m4a",
		"G729_FFMPEG_BLACKBOX_QUALITY=1",
		"validate_verifier_output.sh",
		"external G.729 executables privately only as black-box processes",
		"FCB tree-search strict compare exact-passed",
		"The long-running goal is complete",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("goal completion audit missing prompt-to-artifact evidence %q", want)
		}
	}
	for _, forbidden := range []string{
		"external implementation privately",
		"external implementations privately",
		"may use any external oracle privately",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("goal completion audit contains overly broad clean-room wording %q", forbidden)
		}
	}

	for _, tc := range []struct {
		name  string
		rows  int
		blank bool
	}{
		{name: "fcb_tree_search_expected_template.csv", rows: 10194, blank: false},
		{name: "fcb_tree_search_user_audio_expected_template.csv", rows: 10194, blank: false},
		{name: "encoder_closedloop_stage_expected_template.csv", rows: 100848, blank: true},
	} {
		path := filepath.Join("testdata", "oracle", "handoff", tc.name)
		rows, blanks := countBlankFinalColumnCells(t, path)
		if rows != tc.rows {
			t.Fatalf("%s rows=%d, want %d", path, rows, tc.rows)
		}
		if tc.blank && blanks != rows {
			t.Fatalf("%s blanks=%d, want all %d expected cells blank before verifier output", path, blanks, rows)
		}
		if !tc.blank && blanks != 0 {
			t.Fatalf("%s blanks=%d, want verifier-filled template", path, blanks)
		}
		needle := fmt.Sprintf("`%d/%d`", blanks, rows)
		if !strings.Contains(text, needle) {
			t.Fatalf("goal completion audit missing current blank count %s for %s", needle, tc.name)
		}
	}
}

func TestOracleHandoff_BundleScriptPinsDeterministicInputs(t *testing.T) {
	scriptPath := filepath.Join("testdata", "oracle", "handoff", "create_verifier_bundle.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read bundle helper: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"#!/bin/sh",
		"set -eu",
		"--sort=name",
		"--mtime=\"UTC 2026-05-10 00:00:00\"",
		"--owner=0 --group=0 --numeric-owner",
		"--use-compress-program=\"gzip -n\"",
		"sha256sum \"$archive\"",
		"require_blank_expected",
		"G729_ALLOW_FILLED_VERIFIER_BUNDLE",
		"remaining verifier bundle templates must stay blank",
		"HANDOFF_MANIFEST.md",
		"README.md",
		"EXTERNAL_VERIFIER_REQUEST.md",
		"create_verifier_bundle.sh",
		"validate_verifier_output.sh",
		"DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md",
		"REMAINING_CONFORMANCE_VERIFIER_PROMPT.md",
		"FCB_TREE_SEARCH_VERIFIER_PROMPT.md",
		"FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md",
		"ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md",
		"decoder_pitch_instability_decision_expected_template.csv",
		"decoder_pitch_instability_decision_got.csv",
		"fcb_tree_search_expected_template.csv",
		"fcb_tree_search_got.csv",
		"fcb_tree_search_user_audio_expected_template.csv",
		"fcb_tree_search_user_audio_got.csv",
		"encoder_closedloop_stage_expected_template.csv",
		"encoder_closedloop_stage_got.csv",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bundle helper missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"*.go",
		"third-party",
		"bcg729",
		"Software.zip",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("bundle helper should not copy implementation/source material: %q", forbidden)
		}
	}
}

func TestOracleHandoff_BundleScriptBuildsDocumentedArchive(t *testing.T) {
	const wantSHA256 = "ec13e3ec9eff25bc2522c969e71405f8b550276438e4ab2a18acfb0992201756"
	scriptPath := filepath.Join("testdata", "oracle", "handoff", "create_verifier_bundle.sh")
	tmp := t.TempDir()
	bundleDir := filepath.Join(tmp, "g729-fcb-verifier-handoff")
	archiveA := filepath.Join(tmp, "bundle-a.tar.gz")
	archiveB := filepath.Join(tmp, "bundle-b.tar.gz")

	hashA := runVerifierBundleScript(t, scriptPath, bundleDir, archiveA)
	hashB := runVerifierBundleScript(t, scriptPath, bundleDir, archiveB)
	if hashA != hashB {
		t.Fatalf("bundle helper is not deterministic: first=%s second=%s", hashA, hashB)
	}
	if hashA != wantSHA256 {
		t.Fatalf("bundle sha256=%s, want documented %s", hashA, wantSHA256)
	}

	for _, docPath := range []string{
		filepath.Join("docs", "superpowers", "diagnostics", "2026-05-10-fcb-tree-oracle-handoff.md"),
		filepath.Join("docs", "superpowers", "diagnostics", "2026-05-10-goal-completion-audit.md"),
	} {
		data, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		if !strings.Contains(string(data), wantSHA256) {
			t.Fatalf("%s missing bundle sha256 %s", docPath, wantSHA256)
		}
	}

	out, err := exec.Command("tar", "-tzf", archiveA).Output()
	if err != nil {
		t.Fatalf("list %s: %v", archiveA, err)
	}
	got := strings.TrimSpace(string(out))
	want := strings.Join([]string{
		"g729-fcb-verifier-handoff/",
		"g729-fcb-verifier-handoff/testdata/",
		"g729-fcb-verifier-handoff/testdata/oracle/",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/EXTERNAL_VERIFIER_REQUEST.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/FCB_TREE_SEARCH_VERIFIER_PROMPT.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/HANDOFF_MANIFEST.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/README.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/create_verifier_bundle.sh",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/decoder_pitch_instability_decision_got.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/encoder_closedloop_stage_expected_template.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/encoder_closedloop_stage_got.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/fcb_tree_search_expected_template.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/fcb_tree_search_got.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv",
		"g729-fcb-verifier-handoff/testdata/oracle/handoff/validate_verifier_output.sh",
	}, "\n")
	if got != want {
		t.Fatalf("bundle entries mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestOracleHandoff_BundleScriptRejectsFilledExpectedByDefault(t *testing.T) {
	tmp := t.TempDir()
	copyVerifierBundleInputsTo(t, tmp)

	expectedPath := filepath.Join(tmp, "testdata", "oracle", "handoff", "encoder_closedloop_stage_expected_template.csv")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read temp expected template: %v", err)
	}
	updated := strings.Replace(string(data), "pitch_int,0,0,-1,20,-1,\n", "pitch_int,0,0,-1,20,-1,123\n", 1)
	if updated == string(data) {
		t.Fatalf("temp expected template did not contain the row to fill")
	}
	if err := os.WriteFile(expectedPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write temp filled expected template: %v", err)
	}

	scriptPath := filepath.Join("testdata", "oracle", "handoff", "create_verifier_bundle.sh")
	cmd := exec.Command("sh", scriptPath, filepath.Join(tmp, "bundle"), filepath.Join(tmp, "bundle.tar.gz"))
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bundle helper succeeded with filled expected cell; output:\n%s", out)
	}
	if !strings.Contains(string(out), "remaining verifier bundle templates must stay blank") {
		t.Fatalf("bundle helper failure did not explain filled expected rejection:\n%s", out)
	}

	cmd = exec.Command("sh", scriptPath, filepath.Join(tmp, "bundle"), filepath.Join(tmp, "bundle.tar.gz"))
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "G729_ALLOW_FILLED_VERIFIER_BUNDLE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bundle helper override failed: %v\n%s", err, out)
	}
}

func TestOracleHandoff_IntakeScriptValidatesAndAppliesFilledOutput(t *testing.T) {
	tmp := t.TempDir()
	copyVerifierBundleInputsTo(t, tmp)

	returnedDir := filepath.Join(tmp, "returned")
	if err := os.Mkdir(returnedDir, 0o755); err != nil {
		t.Fatalf("mkdir returned dir: %v", err)
	}
	name := "encoder_closedloop_stage_expected_template.csv"
	templatePath := filepath.Join(tmp, "testdata", "oracle", "handoff", name)
	writeFilledExpectedCSV(t, templatePath, filepath.Join(returnedDir, name), nil)

	scriptPath := filepath.Join("testdata", "oracle", "handoff", "validate_verifier_output.sh")
	cmd := exec.Command("sh", scriptPath, returnedDir)
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate verifier output failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "validation only") {
		t.Fatalf("validate verifier output did not stay validation-only by default:\n%s", out)
	}
	_, blanks := countBlankFinalColumnCells(t, templatePath)
	if blanks == 0 {
		t.Fatal("validation-only intake unexpectedly copied filled verifier output")
	}

	cmd = exec.Command("sh", scriptPath, returnedDir)
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "G729_APPLY_VERIFIER_OUTPUT=1")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply verifier output failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "applied") {
		t.Fatalf("apply verifier output did not report copied file:\n%s", out)
	}
	_, blanks = countBlankFinalColumnCells(t, templatePath)
	if blanks != 0 {
		t.Fatalf("applied verifier output still has %d blank expected cells", blanks)
	}
}

func TestOracleHandoff_IntakeScriptRejectsUnsafeOutput(t *testing.T) {
	tmp := t.TempDir()
	copyVerifierBundleInputsTo(t, tmp)

	scriptPath := filepath.Join("testdata", "oracle", "handoff", "validate_verifier_output.sh")
	name := "fcb_tree_search_expected_template.csv"

	t.Run("non-numeric", func(t *testing.T) {
		returnedDir := filepath.Join(tmp, "returned-nonnumeric")
		if err := os.Mkdir(returnedDir, 0o755); err != nil {
			t.Fatalf("mkdir returned dir: %v", err)
		}
		templatePath := filepath.Join(tmp, "testdata", "oracle", "handoff", name)
		writeFilledExpectedCSV(t, templatePath, filepath.Join(returnedDir, name), func(row int) string {
			if row == 1 {
				return "source-derived-note"
			}
			return strconv.Itoa(row)
		})

		cmd := exec.Command("sh", scriptPath, returnedDir)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("intake validator accepted non-numeric expected cells:\n%s", out)
		}
		if !strings.Contains(string(out), "not a numeric scalar") {
			t.Fatalf("intake validator did not explain non-numeric rejection:\n%s", out)
		}
	})

	t.Run("unexpected-file", func(t *testing.T) {
		returnedDir := filepath.Join(tmp, "returned-unexpected")
		if err := os.Mkdir(returnedDir, 0o755); err != nil {
			t.Fatalf("mkdir returned dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(returnedDir, "notes.txt"), []byte("not a numeric expected CSV\n"), 0o644); err != nil {
			t.Fatalf("write unexpected file: %v", err)
		}

		cmd := exec.Command("sh", scriptPath, returnedDir)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("intake validator accepted unexpected verifier output:\n%s", out)
		}
		if !strings.Contains(string(out), "not an allowed verifier-returned expected CSV") {
			t.Fatalf("intake validator did not explain unexpected-file rejection:\n%s", out)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		returnedDir := filepath.Join(tmp, "returned-symlink")
		if err := os.Mkdir(returnedDir, 0o755); err != nil {
			t.Fatalf("mkdir returned dir: %v", err)
		}
		target := filepath.Join(tmp, "testdata", "oracle", "handoff", name)
		link := filepath.Join(returnedDir, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("create symlink verifier output: %v", err)
		}

		cmd := exec.Command("sh", scriptPath, returnedDir)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("intake validator accepted symlinked verifier output:\n%s", out)
		}
		if !strings.Contains(string(out), "is a symlink") {
			t.Fatalf("intake validator did not explain symlink rejection:\n%s", out)
		}
	})
}

func runVerifierBundleScript(t *testing.T, scriptPath, bundleDir, archivePath string) string {
	t.Helper()
	cmd := exec.Command("sh", scriptPath, bundleDir, archivePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s: %v\n%s", scriptPath, err, out)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 1 {
		t.Fatalf("bundle helper produced no sha256 output")
	}
	return fields[0]
}

func copyVerifierBundleInputsTo(t *testing.T, root string) {
	t.Helper()
	for _, rel := range []string{
		filepath.Join("testdata", "oracle", "handoff", "create_verifier_bundle.sh"),
		filepath.Join("testdata", "oracle", "handoff", "validate_verifier_output.sh"),
		filepath.Join("testdata", "oracle", "handoff", "HANDOFF_MANIFEST.md"),
		filepath.Join("testdata", "oracle", "handoff", "README.md"),
		filepath.Join("testdata", "oracle", "handoff", "EXTERNAL_VERIFIER_REQUEST.md"),
		filepath.Join("testdata", "oracle", "handoff", "REMAINING_CONFORMANCE_VERIFIER_PROMPT.md"),
		filepath.Join("testdata", "oracle", "handoff", "DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md"),
		filepath.Join("testdata", "oracle", "handoff", "FCB_TREE_SEARCH_VERIFIER_PROMPT.md"),
		filepath.Join("testdata", "oracle", "handoff", "FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md"),
		filepath.Join("testdata", "oracle", "handoff", "ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md"),
		filepath.Join("testdata", "oracle", "handoff", "decoder_pitch_instability_decision_expected_template.csv"),
		filepath.Join("testdata", "oracle", "handoff", "decoder_pitch_instability_decision_got.csv"),
		filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_expected_template.csv"),
		filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_got.csv"),
		filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_user_audio_expected_template.csv"),
		filepath.Join("testdata", "oracle", "handoff", "fcb_tree_search_user_audio_got.csv"),
		filepath.Join("testdata", "oracle", "handoff", "encoder_closedloop_stage_expected_template.csv"),
		filepath.Join("testdata", "oracle", "handoff", "encoder_closedloop_stage_got.csv"),
	} {
		data, err := os.ReadFile(rel)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
}

func writeFilledExpectedCSV(t *testing.T, templatePath, dstPath string, valueForRow func(int) string) {
	t.Helper()
	f, err := os.Open(templatePath)
	if err != nil {
		t.Fatalf("open template %s: %v", templatePath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read template %s: %v", templatePath, err)
	}
	if len(rows) < 2 {
		t.Fatalf("template %s has no data rows", templatePath)
	}
	for i := 1; i < len(rows); i++ {
		if len(rows[i]) == len(rows[0])-1 {
			rows[i] = append(rows[i], "")
		}
		if len(rows[i]) != len(rows[0]) {
			t.Fatalf("template %s row %d has %d columns, want %d", templatePath, i+1, len(rows[i]), len(rows[0]))
		}
		value := strconv.Itoa(i)
		if valueForRow != nil {
			value = valueForRow(i)
		}
		rows[i][len(rows[i])-1] = value
	}

	var out strings.Builder
	w := csv.NewWriter(&out)
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("render filled expected CSV: %v", err)
	}
	if err := w.Error(); err != nil {
		t.Fatalf("write filled expected CSV: %v", err)
	}
	if err := os.WriteFile(dstPath, []byte(out.String()), 0o644); err != nil {
		t.Fatalf("write filled expected CSV %s: %v", dstPath, err)
	}
}

func TestOracleHandoff_VerifierPromptsStateCleanRoomBoundary(t *testing.T) {
	prompts := []string{
		"FCB_TREE_SEARCH_VERIFIER_PROMPT.md",
		"FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md",
		"ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md",
		"REMAINING_CONFORMANCE_VERIFIER_PROMPT.md",
		"EXTERNAL_VERIFIER_REQUEST.md",
		"DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md",
		"DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md",
		"DECODER_TAME_EXCITATION_HISTORY_PROMPT.md",
		"DECODER_SUPPORT_TABLES_PROMPT.md",
	}
	for _, name := range prompts {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "oracle", "handoff", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			text := string(data)
			normalized := strings.Join(strings.Fields(text), " ")
			for _, want := range []string{
				"Do not inspect ITU reference C",
				"bcg729",
				"FFmpeg source",
				"black-box executable",
				"numeric scalar oracle artifacts",
			} {
				if !strings.Contains(normalized, want) {
					t.Fatalf("%s missing clean-room boundary phrase %q", name, want)
				}
			}
			for _, forbidden := range []string{
				"use any external oracle privately",
				"may use any external oracle privately",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s contains overly broad oracle wording %q", name, forbidden)
				}
			}
		})
	}
}

func TestOracleHandoff_FCBPromptPinsSpeechInput(t *testing.T) {
	const (
		wantBytes  = 600064
		wantSHA256 = "0fceb7702a05d09a9a39d5442c56b827ae0f43eb88a33746ab6f28e7ab5849d5"
	)
	promptPath := filepath.Join("testdata", "oracle", "handoff", "FCB_TREE_SEARCH_VERIFIER_PROMPT.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read FCB verifier prompt: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		encoderClosedLoopFrameSamplePath,
		"600064",
		"300032",
		wantSHA256,
		"The included frame range is `292..294`",
		"The template has `10194` data rows plus the header",
		"G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1",
		"G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1",
		"-count=1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("FCB verifier prompt missing pinned input detail %q", want)
		}
	}

	input, err := os.ReadFile(encoderClosedLoopFrameSamplePath)
	if err != nil {
		t.Fatalf("read %s: %v", encoderClosedLoopFrameSamplePath, err)
	}
	if len(input) != wantBytes {
		t.Fatalf("%s bytes=%d, want %d", encoderClosedLoopFrameSamplePath, len(input), wantBytes)
	}
	sum := sha256.Sum256(input)
	if got := fmt.Sprintf("%x", sum[:]); got != wantSHA256 {
		t.Fatalf("%s sha256=%s, want %s", encoderClosedLoopFrameSamplePath, got, wantSHA256)
	}
}

func TestOracleHandoff_UserAudioFCBPromptPinsConvertedSample(t *testing.T) {
	promptPath := filepath.Join("testdata", "oracle", "handoff", "FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read user-audio FCB prompt: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"testdata/external/user_quality_audio.m4a",
		"118701",
		"237402",
		"e8d783af34de25d8d7d16a84dfe92238c647e4079a07d8dffd4e715a804ca5fa",
		"frame range is `292..294`",
		"G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("user-audio FCB verifier prompt missing pinned input detail %q", want)
		}
	}

	manifestPath := filepath.Join("testdata", "oracle", "handoff", "HANDOFF_MANIFEST.md")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifestText := string(manifest)
	for _, want := range []string{
		"`FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md` | n/a | n/a | `102cfaccf8bc984507294b2d4ddee7272cd20befdeb513765b0989fb8fa1ada5`",
		"`fcb_tree_search_user_audio_expected_template.csv` | `field,frame,sub,index,expected` | 10194",
		"`fcb_tree_search_user_audio_got.csv` | `field,frame,sub,index,got` | 10194 | `70b70b6f76e224172edd54a3863ebbc3c11e9f8859419af4f9964afe0169f3ae`",
	} {
		if !strings.Contains(manifestText, want) {
			t.Fatalf("manifest missing user-audio FCB pinned detail %q", want)
		}
	}
}

func TestOracleHandoff_UserAudioFCBConvertedPCMHash(t *testing.T) {
	const (
		wantSamples = 118701
		wantBytes   = wantSamples * 2
		wantSHA256  = "e8d783af34de25d8d7d16a84dfe92238c647e4079a07d8dffd4e715a804ca5fa"
	)
	if _, err := os.Stat(fcbTreeUserAudioSamplePath); err != nil {
		t.Skipf("user-audio sample unavailable: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-i", fcbTreeUserAudioSamplePath,
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ar", "8000",
		"-ac", "1",
		"pipe:1",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg convert %s: %v", fcbTreeUserAudioSamplePath, err)
	}
	if len(out) != wantBytes {
		t.Fatalf("converted PCM bytes=%d, want %d", len(out), wantBytes)
	}
	sum := sha256.Sum256(out)
	if got := fmt.Sprintf("%x", sum[:]); got != wantSHA256 {
		t.Fatalf("converted PCM sha256=%s, want %s", got, wantSHA256)
	}
}

func parseManifestHashes(t *testing.T, text, start, end string) map[string]string {
	t.Helper()
	section := markdownSection(t, text, start, end)
	out := make(map[string]string)
	for _, line := range strings.Split(section, "\n") {
		cells := splitMarkdownTableRow(line)
		if len(cells) < 3 {
			continue
		}
		name := markdownCodeCell(cells[0])
		if name == "" {
			continue
		}
		hash := ""
		for _, cell := range cells[1:] {
			candidate := markdownCodeCell(cell)
			if len(candidate) == sha256.Size*2 {
				hash = candidate
				break
			}
		}
		if hash == "" {
			continue
		}
		if len(hash) != sha256.Size*2 {
			t.Fatalf("%s hash for %s has length %d, want %d", start, name, len(hash), sha256.Size*2)
		}
		out[name] = hash
	}
	return out
}

func parseManifestBlankCounts(t *testing.T, text, start, end string) map[string]int {
	t.Helper()
	section := markdownSection(t, text, start, end)
	out := make(map[string]int)
	for _, line := range strings.Split(section, "\n") {
		cells := splitMarkdownTableRow(line)
		if len(cells) < 2 {
			continue
		}
		name := markdownCodeCell(cells[0])
		if name == "" {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(cells[1]))
		if err != nil {
			t.Fatalf("%s blank count for %s: %v", start, name, err)
		}
		out[name] = count
	}
	return out
}

func markdownSection(t *testing.T, text, start, end string) string {
	t.Helper()
	startAt := strings.Index(text, start)
	if startAt < 0 {
		t.Fatalf("manifest missing section %q", start)
	}
	section := text[startAt+len(start):]
	if end == "" {
		return section
	}
	endAt := strings.Index(section, end)
	if endAt < 0 {
		t.Fatalf("manifest section %q missing end marker %q", start, end)
	}
	return section[:endAt]
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	raw := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, len(raw))
	for i, cell := range raw {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells
}

func markdownCodeCell(cell string) string {
	cell = strings.TrimSpace(cell)
	if !strings.HasPrefix(cell, "`") || !strings.HasSuffix(cell, "`") {
		return ""
	}
	return strings.Trim(cell, "`")
}

func assertManifestFileHash(t *testing.T, name, want string) {
	t.Helper()
	path := filepath.Join("testdata", "oracle", "handoff", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	if got := fmt.Sprintf("%x", sum[:]); got != want {
		t.Fatalf("%s sha256=%s, manifest says %s", path, got, want)
	}
}

func countBlankFinalColumnCells(t *testing.T, path string) (int, int) {
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
	if len(rows) == 0 {
		t.Fatalf("%s has no header", path)
	}
	headerColumns := len(rows[0])
	var blanks int
	for line, row := range rows[1:] {
		if len(row) == headerColumns-1 {
			row = append(row, "")
		}
		if len(row) != headerColumns {
			t.Fatalf("%s line %d: columns=%d, want %d", path, line+2, len(row), headerColumns)
		}
		if row[len(row)-1] == "" {
			blanks++
		}
	}
	return len(rows) - 1, blanks
}
