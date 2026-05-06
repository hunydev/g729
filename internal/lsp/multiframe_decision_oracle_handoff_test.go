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

const lspDecisionHandoffFrames = 16

func TestOracleHandoff_WriteLSPDecisionHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_LSP_DECISION_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_LSP_DECISION_HANDOFF=1 to refresh LSP multi-frame decision handoff files")
	}

	records, err := collectLSPDecisionRecords(lspDecisionHandoffFrames)
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(dir, "lsp_decision_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writeLSPDecisionCSV(filepath.Join(dir, "lsp_decision_got.csv"), records, "got", true); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeLSPDecisionCSV(expectedPath, records, "expected", false); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_CompareLSPDecisionHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_LSP_DECISION_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_LSP_DECISION_HANDOFF=1 after verifier fills lsp_decision_expected_template.csv")
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	got, err := readLSPDecisionValues(filepath.Join(dir, "lsp_decision_got.csv"), "got")
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	expected, blanks, err := readLSPDecisionExpected(filepath.Join(dir, "lsp_decision_expected_template.csv"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells")
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF") == "1" && blanks > 0 {
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

	t.Logf("LSP decision handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("LSP decision handoff has %d mismatches", mismatch)
	}
}

type lspDecisionRecord struct {
	Field string
	Frame int
	Tap   int
	L0    int
	L1    int
	L2    int
	L3    int
	Col   int
	Value int64
}

func collectLSPDecisionRecords(frameCount int) ([]lspDecisionRecord, error) {
	const (
		inPath           = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath          = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)
	if frameCount > totalFrames {
		frameCount = totalFrames
	}

	inData, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.IN: %w", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.BIT: %w", err)
	}
	if len(inData) < frameCount*bytesPerInFrame {
		return nil, fmt.Errorf("LSP.IN size = %d, want at least %d", len(inData), frameCount*bytesPerInFrame)
	}
	if len(bitData) < frameCount*bytesPerBitFrame {
		return nil, fmt.Errorf("LSP.BIT size = %d, want at least %d", len(bitData), frameCount*bytesPerBitFrame)
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	var records []lspDecisionRecord
	add := func(field string, frame, tap, l0, l1, l2, l3, col int, value int64) {
		records = append(records, lspDecisionRecord{
			Field: field,
			Frame: frame,
			Tap:   tap,
			L0:    l0,
			L1:    l1,
			L2:    l2,
			L3:    l3,
			Col:   col,
			Value: value,
		})
	}

	for frame := 0; frame < frameCount; frame++ {
		var pcmFrame [samplesPerFrame]int16
		off := frame * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			return nil, fmt.Errorf("frame %d: lpc.Analyze: %w", frame, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				return nil, fmt.Errorf("frame %d: LPToLSP: %w", frame, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		var weights [10]int16
		weightsLSF(&omega, &weights)

		bitOff := frame * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}
		got, residual := quantizeNoCommit(&omega, &freqPrev)

		for tap := 0; tap < 4; tap++ {
			for col, value := range freqPrev[tap] {
				add("predictor_memory", frame, tap, -1, -1, -1, -1, col, int64(value))
			}
		}
		for col, value := range omega {
			add("omega_lsf", frame, -1, -1, -1, -1, -1, col, int64(value))
		}
		for col, value := range weights {
			add("weight_lsf", frame, -1, -1, -1, -1, -1, col, int64(value))
		}
		for selector := uint8(0); selector < 2; selector++ {
			var target [10]int16
			computeTargetLSF(selector, &freqPrev, &omega, &target)
			for col, value := range target {
				add("target_lsf", frame, -1, int(selector), -1, -1, -1, col, int64(value))
			}
		}
		for col, value := range []uint8{got.L0, got.L1, got.L2, got.L3} {
			add("encoder_index", frame, -1, -1, -1, -1, -1, col, int64(value))
		}
		for col, value := range []uint8{ref.L0, ref.L1, ref.L2, ref.L3} {
			add("bitstream_index", frame, -1, -1, -1, -1, -1, col, int64(value))
		}
		add("encoder_tuple_cost", frame, -1, int(got.L0), int(got.L1), int(got.L2), int(got.L3), -1,
			finalLSPTupleCost(got, &freqPrev, &omega, &weights))
		add("bitstream_tuple_cost", frame, -1, int(ref.L0), int(ref.L1), int(ref.L2), int(ref.L3), -1,
			finalLSPTupleCost(ref, &freqPrev, &omega, &weights))
		add("encoder_tuple_rank", frame, -1, int(got.L0), int(got.L1), int(got.L2), int(got.L3), -1,
			int64(rankFullLSPTuple(got, &freqPrev, &omega, &weights)))
		add("bitstream_tuple_rank", frame, -1, int(ref.L0), int(ref.L1), int(ref.L2), int(ref.L3), -1,
			int64(rankFullLSPTuple(ref, &freqPrev, &omega, &weights)))

		commitPredictorMemory(&freqPrev, &residual)
	}
	return records, nil
}

func writeLSPDecisionCSV(path string, records []lspDecisionRecord, valueColumn string, includeValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"field", "frame", "tap", "L0", "L1", "L2", "L3", "col", valueColumn}); err != nil {
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
			strconv.Itoa(r.Tap),
			strconv.Itoa(r.L0),
			strconv.Itoa(r.L1),
			strconv.Itoa(r.L2),
			strconv.Itoa(r.L3),
			strconv.Itoa(r.Col),
			value,
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func readLSPDecisionValues(path, valueColumn string) (map[string]int64, error) {
	rows, err := readLSPDecisionRows(path)
	if err != nil {
		return nil, err
	}
	valueIdx := lspDecisionHeaderIndex(rows[0], valueColumn)
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}
	out := make(map[string]int64, len(rows)-1)
	for line, row := range rows[1:] {
		key, err := lspDecisionKey(row)
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

func readLSPDecisionExpected(path string) (map[string]int64, int, error) {
	rows, err := readLSPDecisionRows(path)
	if err != nil {
		return nil, 0, err
	}
	valueIdx := lspDecisionHeaderIndex(rows[0], "expected")
	if valueIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}
	out := make(map[string]int64, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		if len(row) == valueIdx {
			row = append(row, "")
		}
		key, err := lspDecisionKey(row)
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

func readLSPDecisionRows(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty csv")
	}
	return rows, nil
}

func lspDecisionHeaderIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func lspDecisionKey(row []string) (string, error) {
	if len(row) < 8 {
		return "", fmt.Errorf("short row")
	}
	return row[0] + "|" + row[1] + "|" + row[2] + "|" + row[3] + "|" + row[4] + "|" + row[5] + "|" + row[6] + "|" + row[7], nil
}
