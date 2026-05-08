package lsp

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/lpc"
	"github.com/hunydev/g729/internal/pcm"
)

func TestOracleHandoff_WriteLSPFrame0VQHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_LSP_FRAME0_VQ_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_LSP_FRAME0_VQ_HANDOFF=1 to refresh LSP frame-0 VQ handoff files")
	}

	records, err := collectLSPFrame0VQRecords()
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(dir, "lsp_frame0_vq_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writeLSPFrame0VQCSV(filepath.Join(dir, "lsp_frame0_vq_got.csv"), records, "got", true); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeLSPFrame0VQCSV(expectedPath, records, "expected", false); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_CompareLSPFrame0VQHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_LSP_FRAME0_VQ_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 after verifier fills lsp_frame0_vq_expected_template.csv")
	}

	dir := filepath.Join("..", "..", "testdata", "oracle", "handoff")
	got, err := readLSPFrame0VQValues(filepath.Join(dir, "lsp_frame0_vq_got.csv"), "got")
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	expected, blanks, err := readLSPFrame0VQExpected(filepath.Join(dir, "lsp_frame0_vq_expected_template.csv"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells")
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_LSP_FRAME0_VQ_HANDOFF") == "1" && blanks > 0 {
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

	t.Logf("LSP frame-0 VQ handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_LSP_FRAME0_VQ_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("LSP frame-0 VQ handoff has %d mismatches", mismatch)
	}
}

type lspFrame0VQRecord struct {
	Field    string
	Frame    int
	Selector int
	Tap      int
	L1       int
	L2       int
	L3       int
	Col      int
	Value    int64
}

func collectLSPFrame0VQRecords() ([]lspFrame0VQRecord, error) {
	const (
		inPath           = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath          = "../../testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.IN: %w", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		return nil, fmt.Errorf("read LSP.BIT: %w", err)
	}
	if len(inData) < bytesPerInFrame {
		return nil, fmt.Errorf("LSP.IN size = %d, want at least %d", len(inData), bytesPerInFrame)
	}
	if len(bitData) < bytesPerBitFrame {
		return nil, fmt.Errorf("LSP.BIT size = %d, want at least %d", len(bitData), bytesPerBitFrame)
	}

	var pcmFrame [samplesPerFrame]int16
	for i := 0; i < samplesPerFrame; i++ {
		pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[2*i : 2*i+2]))
	}
	var pp pcm.PreProcessor
	var processed [samplesPerFrame]int16
	pp.Process(pcmFrame[:], processed[:])
	var oldSpeech [240]int16
	copy(oldSpeech[160:240], processed[:])

	var an lpc.Analyzer
	var aQ12 [lpc.LPCOrder + 1]int16
	if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
		return nil, fmt.Errorf("frame 0: lpc.Analyze: %w", err)
	}
	var qQ15 [10]int16
	if err := LPToLSP(&aQ12, &qQ15); err != nil {
		return nil, fmt.Errorf("frame 0: LPToLSP: %w", err)
	}
	var omega [10]int16
	LSPToLSF(&qQ15, &omega)
	var weights [10]int16
	weightsLSF(&omega, &weights)

	refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[:bytesPerBitFrame])
	ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)
	got, _ := quantizeNoCommit(&omega, &freqPrev)

	var records []lspFrame0VQRecord
	add := func(field string, selector, tap, l1, l2, l3, col int, value int64) {
		records = append(records, lspFrame0VQRecord{
			Field: field, Frame: 0, Selector: selector, Tap: tap,
			L1: l1, L2: l2, L3: l3, Col: col, Value: value,
		})
	}

	for tap := 0; tap < 4; tap++ {
		for col := 0; col < 10; col++ {
			add("initial_memory", -1, tap, -1, -1, -1, col, int64(freqPrev[tap][col]))
		}
	}
	for selector := uint8(0); selector < 2; selector++ {
		var target [10]int16
		computeTargetLSF(selector, &freqPrev, &omega, &target)
		for col, value := range target {
			add("target_lsf", int(selector), -1, -1, -1, -1, col, int64(value))
		}
	}
	for col, value := range []uint8{got.L0, got.L1, got.L2, got.L3} {
		add("selected_index", -1, -1, -1, -1, -1, col, int64(value))
	}
	for col, value := range []uint8{ref.L0, ref.L1, ref.L2, ref.L3} {
		add("reference_index", -1, -1, -1, -1, -1, col, int64(value))
	}

	var refTarget [10]int16
	computeTargetLSF(ref.L0, &freqPrev, &omega, &refTarget)
	l1Rank, l1Best, l1RefCost, l1BestCost := rankL1Target(&refTarget, int(ref.L1))
	add("l1_rank", int(ref.L0), -1, int(ref.L1), -1, -1, -1, int64(l1Rank))
	add("l1_cost", int(ref.L0), -1, int(ref.L1), -1, -1, -1, l1RefCost)
	if l1Best != int(ref.L1) {
		add("l1_cost", int(ref.L0), -1, l1Best, -1, -1, -1, l1BestCost)
	}

	pairCosts := computeL2L3PairCost(ref.L1, ref.L0, &freqPrev, &omega, &weights)
	bestL2, bestL3 := bestPairCost(pairCosts)
	add("l23_pair_rank", int(ref.L0), -1, int(ref.L1), int(ref.L2), int(ref.L3), -1,
		int64(rankPairCost(pairCosts, int(ref.L2), int(ref.L3))))
	add("l23_pair_cost", int(ref.L0), -1, int(ref.L1), int(ref.L2), int(ref.L3), -1,
		pairCosts[ref.L2][ref.L3])
	add("l23_pair_cost", int(ref.L0), -1, int(ref.L1), int(bestL2), int(bestL3), -1,
		pairCosts[bestL2][bestL3])

	gotCost := finalLSPTupleCost(got, &freqPrev, &omega, &weights)
	refCost := finalLSPTupleCost(ref, &freqPrev, &omega, &weights)
	add("full_tuple_cost", int(got.L0), -1, int(got.L1), int(got.L2), int(got.L3), -1, gotCost)
	add("full_tuple_cost", int(ref.L0), -1, int(ref.L1), int(ref.L2), int(ref.L3), -1, refCost)
	add("full_tuple_rank", int(ref.L0), -1, int(ref.L1), int(ref.L2), int(ref.L3), -1,
		int64(rankFullLSPTuple(ref, &freqPrev, &omega, &weights)))

	return records, nil
}

func writeLSPFrame0VQCSV(path string, records []lspFrame0VQRecord, valueColumn string, includeValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"field", "frame", "selector", "tap", "L1", "L2", "L3", "col", valueColumn}); err != nil {
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
			strconv.Itoa(r.Selector),
			strconv.Itoa(r.Tap),
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

func readLSPFrame0VQValues(path, valueColumn string) (map[string]int64, error) {
	rows, err := readLSPFrame0VQRows(path)
	if err != nil {
		return nil, err
	}
	valueIdx := lspFrame0VQHeaderIndex(rows[0], valueColumn)
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}
	out := make(map[string]int64, len(rows)-1)
	for line, row := range rows[1:] {
		key, err := lspFrame0VQKey(row)
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

func readLSPFrame0VQExpected(path string) (map[string]int64, int, error) {
	rows, err := readLSPFrame0VQRows(path)
	if err != nil {
		return nil, 0, err
	}
	valueIdx := lspFrame0VQHeaderIndex(rows[0], "expected")
	if valueIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}
	out := make(map[string]int64, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		key, err := lspFrame0VQKey(row)
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

func readLSPFrame0VQRows(path string) ([][]string, error) {
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

func lspFrame0VQHeaderIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func lspFrame0VQKey(row []string) (string, error) {
	if len(row) < 8 {
		return "", fmt.Errorf("short row")
	}
	return row[0] + "|" + row[1] + "|" + row[2] + "|" + row[3] + "|" + row[4] + "|" + row[5] + "|" + row[6] + "|" + row[7], nil
}
