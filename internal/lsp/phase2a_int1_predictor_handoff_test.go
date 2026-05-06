package lsp

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
)

func TestOracleHandoff_WriteLSPPredictorResidualHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_LSP_PREDICTOR_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_LSP_PREDICTOR_HANDOFF=1 to refresh LSP predictor residual handoff files")
	}

	records, err := collectLSPPredictorResidualRecords()
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(dir, "lsp_predictor_residual_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writeLSPPredictorResidualGot(filepath.Join(dir, "lsp_predictor_residual_got.csv"), records); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeLSPPredictorResidualExpectedTemplate(expectedPath, records); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_CompareLSPPredictorResidualHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_LSP_PREDICTOR_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 after verifier fills lsp_predictor_residual_expected_template.csv")
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	got, err := readLSPPredictorResidualValues(filepath.Join(dir, "lsp_predictor_residual_got.csv"), "got")
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	expected, blanks, err := readLSPPredictorResidualExpected(filepath.Join(dir, "lsp_predictor_residual_expected_template.csv"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells")
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF") == "1" && blanks > 0 {
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

	t.Logf("LSP predictor residual handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("LSP predictor residual handoff has %d mismatches", mismatch)
	}
}

func TestOracleHandoff_LSPReferenceResidualTrajectoryDiagnostic(t *testing.T) {
	records, err := collectLSPPredictorResidualRecords()
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}
	refResiduals := collectReferenceLSPResiduals(records)
	if len(refResiduals) != len(records) {
		t.Fatalf("reference residual records=%d, want %d", len(refResiduals), len(records))
	}

	type colStats struct {
		exact  int
		sumAbs int64
		maxAbs int
	}
	var cols [10]colStats
	var exact, frameExact int
	var first []string
	frameAll := true
	prevFrame := -1
	for i, r := range records {
		if r.Frame != prevFrame {
			if prevFrame >= 0 && frameAll {
				frameExact++
			}
			prevFrame = r.Frame
			frameAll = true
		}

		ref := refResiduals[i]
		delta := int(r.Value) - int(ref)
		if delta == 0 {
			exact++
			cols[r.Col].exact++
			continue
		}
		frameAll = false
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		cols[r.Col].sumAbs += int64(abs)
		if abs > cols[r.Col].maxAbs {
			cols[r.Col].maxAbs = abs
		}
		if len(first) < 12 {
			first = append(first, fmt.Sprintf(
				"frame=%d col=%d local=(%d,%d,%d,%d) ref=(%d,%d,%d,%d) localResidual=%d refResidual=%d delta=%+d",
				r.Frame, r.Col,
				r.Selector, r.L1, r.L2, r.L3,
				r.ReferenceSelector, r.ReferenceL1, r.ReferenceL2, r.ReferenceL3,
				r.Value, ref, delta,
			))
		}
	}
	if prevFrame >= 0 && frameAll {
		frameExact++
	}

	total := len(records)
	totalFrames := total / 10
	t.Logf("LSP reference-index residual trajectory: exact %d/%d %.2f%% frame-all10 %d/%d %.2f%%",
		exact, total, 100*float64(exact)/float64(total),
		frameExact, totalFrames, 100*float64(frameExact)/float64(totalFrames))
	for col, s := range cols {
		denom := totalFrames - s.exact
		meanAbs := 0.0
		if denom > 0 {
			meanAbs = float64(s.sumAbs) / float64(denom)
		}
		t.Logf("col %d: exact %d/%d %.2f%% mismatchMeanAbs=%.2f maxAbs=%d",
			col, s.exact, totalFrames, 100*float64(s.exact)/float64(totalFrames), meanAbs, s.maxAbs)
	}
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
}

type lspPredictorResidualRecord struct {
	Frame             int
	Selector          uint8
	L1, L2, L3        uint8
	ReferenceSelector uint8
	ReferenceL1       uint8
	ReferenceL2       uint8
	ReferenceL3       uint8
	Col               int
	Value             int16
}

func collectReferenceLSPResiduals(records []lspPredictorResidualRecord) []int16 {
	out := make([]int16, 0, len(records))
	for i := 0; i < len(records); i += 10 {
		r := records[i]
		var residual [10]int16
		combineResidual(r.ReferenceL1, r.ReferenceL2, r.ReferenceL3, &residual)
		for col := range residual {
			out = append(out, residual[col])
		}
	}
	return out
}

func collectLSPPredictorResidualRecords() ([]lspPredictorResidualRecord, error) {
	const (
		inPath           = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath          = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.IN: %w", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.BIT: %w", err)
	}
	if len(inData) != totalFrames*bytesPerInFrame {
		return nil, fmt.Errorf("LSP.IN size = %d, want %d", len(inData), totalFrames*bytesPerInFrame)
	}
	if len(bitData) != totalFrames*bytesPerBitFrame {
		return nil, fmt.Errorf("LSP.BIT size = %d, want %d", len(bitData), totalFrames*bytesPerBitFrame)
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	records := make([]lspPredictorResidualRecord, 0, totalFrames*10)
	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			return nil, fmt.Errorf("frame %d: lpc.Analyze: %w", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				return nil, fmt.Errorf("frame %d: LPToLSP: %w", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])

		idx, residual := quantizeNoCommit(&omega, &freqPrev)
		commitPredictorMemory(&freqPrev, &residual)

		for col, value := range residual {
			records = append(records, lspPredictorResidualRecord{
				Frame:             f,
				Selector:          idx.L0,
				L1:                idx.L1,
				L2:                idx.L2,
				L3:                idx.L3,
				ReferenceSelector: refL0,
				ReferenceL1:       refL1,
				ReferenceL2:       refL2,
				ReferenceL3:       refL3,
				Col:               col,
				Value:             value,
			})
		}
	}
	return records, nil
}

func writeLSPPredictorResidualGot(path string, records []lspPredictorResidualRecord) error {
	return writeLSPPredictorResidualCSV(path, records, "got", true)
}

func writeLSPPredictorResidualExpectedTemplate(path string, records []lspPredictorResidualRecord) error {
	return writeLSPPredictorResidualCSV(path, records, "expected", false)
}

func writeLSPPredictorResidualCSV(path string, records []lspPredictorResidualRecord, valueHeader string, writeValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"frame", "selector", "L1", "L2", "L3", "ref_selector", "ref_L1", "ref_L2", "ref_L3", "col", valueHeader}); err != nil {
		return err
	}
	for _, r := range records {
		value := ""
		if writeValue {
			value = strconv.Itoa(int(r.Value))
		}
		if err := w.Write([]string{
			strconv.Itoa(r.Frame),
			strconv.Itoa(int(r.Selector)),
			strconv.Itoa(int(r.L1)),
			strconv.Itoa(int(r.L2)),
			strconv.Itoa(int(r.L3)),
			strconv.Itoa(int(r.ReferenceSelector)),
			strconv.Itoa(int(r.ReferenceL1)),
			strconv.Itoa(int(r.ReferenceL2)),
			strconv.Itoa(int(r.ReferenceL3)),
			strconv.Itoa(r.Col),
			value,
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func readLSPPredictorResidualValues(path, valueColumn string) (map[string]int, error) {
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
	valueIdx := indexOfHeader(rows[0], valueColumn)
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}

	out := make(map[string]int, len(rows)-1)
	for line, row := range rows[1:] {
		if len(row) <= valueIdx {
			return nil, fmt.Errorf("line %d: short row", line+2)
		}
		key, err := lspPredictorResidualKey(row)
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

func readLSPPredictorResidualExpected(path string) (map[string]int, int, error) {
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
	expectedIdx := indexOfHeader(rows[0], "expected")
	if expectedIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}

	out := make(map[string]int, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		if len(row) <= expectedIdx {
			return nil, 0, fmt.Errorf("line %d: short row", line+2)
		}
		key, err := lspPredictorResidualKey(row)
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

func lspPredictorResidualKey(row []string) (string, error) {
	if len(row) < 10 {
		return "", fmt.Errorf("short key row")
	}
	return row[0] + "/" + row[9], nil
}

func indexOfHeader(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}
