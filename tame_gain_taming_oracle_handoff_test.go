package g729

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/gainquant"
	pitchidx "github.com/hunydev/g729/internal/pitch"
	clpitch "github.com/hunydev/g729/internal/pitch/closedloop"
)

func TestOracleHandoff_WriteTAMEGainTamingHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_TAME_GAIN_TAMING_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_TAME_GAIN_TAMING_HANDOFF=1 to refresh TAME gain/taming handoff files")
	}

	records, err := collectTAMEGainTamingRecords()
	if err != nil {
		t.Fatalf("collect records: %v", err)
	}

	dir := filepath.Join("testdata", "oracle", "handoff")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(dir, "tame_gain_taming_expected_template.csv")
	if err := guardVerifierExpectedTemplate(expectedPath, "expected"); err != nil {
		t.Fatalf("expected template guard: %v", err)
	}
	if err := writeTAMEGainTamingCSV(filepath.Join(dir, "tame_gain_taming_got.csv"), records, "got", true); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeTAMEGainTamingCSV(expectedPath, records, "expected", false); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
}

func TestOracleHandoff_CompareTAMEGainTamingHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_TAME_GAIN_TAMING_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 after verifier fills tame_gain_taming_expected_template.csv")
	}

	dir := filepath.Join("testdata", "oracle", "handoff")
	got, err := readTAMEGainTamingValues(filepath.Join(dir, "tame_gain_taming_got.csv"), "got")
	if err != nil {
		t.Fatalf("read got: %v", err)
	}
	expected, blanks, err := readTAMEGainTamingExpected(filepath.Join(dir, "tame_gain_taming_expected_template.csv"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no filled numeric cells")
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF") == "1" && blanks > 0 {
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

	t.Logf("TAME gain/taming handoff compare: exact %d/%d %.2f%% mismatches=%d blanks=%d",
		exact, len(expected), 100*float64(exact)/float64(len(expected)), mismatch, blanks)
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
	if os.Getenv("G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("TAME gain/taming handoff has %d mismatches", mismatch)
	}
}

type tameGainTamingRecord struct {
	Field string
	Frame int
	Sub   int
	Index int
	Value int64
}

func collectTAMEGainTamingRecords() ([]tameGainTamingRecord, error) {
	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.BIT"
		samplesPerFrame  = FrameSamples
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 128
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("read TAME.IN: %w", err)
	}
	if len(inData) != totalFrames*bytesPerInFrame {
		return nil, fmt.Errorf("TAME.IN size = %d, want %d", len(inData), totalFrames*bytesPerInFrame)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		return nil, fmt.Errorf("read TAME.BIT: %w", err)
	}
	if len(bitData) != totalFrames*bytesPerBitFrame {
		return nil, fmt.Errorf("TAME.BIT size = %d, want %d", len(bitData), totalFrames*bytesPerBitFrame)
	}

	want := make([][bitstream.FrameBytes]byte, totalFrames)
	bitReader := bytes.NewReader(bitData)
	for f := 0; f < totalFrames; f++ {
		if _, err := bitstream.ReadG192Frame(bitReader, want[f][:]); err != nil {
			return nil, fmt.Errorf("ReadG192Frame frame %d: %w", f, err)
		}
	}

	var records []tameGainTamingRecord
	add := func(field string, frame, sub, index int, value int64) {
		records = append(records, tameGainTamingRecord{
			Field: field,
			Frame: frame,
			Sub:   sub,
			Index: index,
			Value: value,
		})
	}
	add("taming_clip_q14", -1, -1, -1, int64(gainquant.GpClipQ14))
	add("taming_energy_threshold_q0", -1, -1, -1, gainquant.TameEnergyThresholdQ0)

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	for frame := 0; frame < totalFrames; frame++ {
		base := frame * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}

		idx, err := enc.lpcStep(pcm[:])
		if err != nil {
			return nil, fmt.Errorf("frame %d: lpcStep: %w", frame, err)
		}
		_ = enc.openloopStep()

		var ref bitstream.Frame
		if err := bitstream.Unpack(want[frame][:], &ref); err != nil {
			return nil, fmt.Errorf("frame %d: unpack TAME.BIT: %w", frame, err)
		}
		for col, value := range []uint8{idx.L0, idx.L1, idx.L2, idx.L3} {
			add("local_lsp_index", frame, -1, col, int64(value))
		}
		for col, value := range []uint16{ref.L0, ref.L1, ref.L2, ref.L3} {
			add("bitstream_lsp_index", frame, -1, col, int64(value))
		}

		refInt1, refFrac1 := pitchidx.DecodeDelaySubframe1(uint8(ref.P1))
		collectTAMEGainTamingSubframeRecords(add, enc, frame, 0, int(ref.P1), int(ref.P0), int(ref.C1), int(ref.S1), int(ref.GA1), int(ref.GB1), refInt1, refFrac1)
		_, _ = enc.closedloopStep(0)

		refInt2, refFrac2 := pitchidx.DecodeDelaySubframe2(uint8(ref.P2), refInt1)
		collectTAMEGainTamingSubframeRecords(add, enc, frame, 1, int(ref.P2), -1, int(ref.C2), int(ref.S2), int(ref.GA2), int(ref.GB2), refInt2, refFrac2)
		_, _ = enc.closedloopStep(1)
	}
	return records, nil
}

func collectTAMEGainTamingSubframeRecords(
	add func(string, int, int, int, int64),
	e *Encoder,
	frame, sub int,
	refP, refP0, refC, refS, refGA, refGB int,
	refInt int,
	refFrac int,
) {
	add("state_prev_gp_q14", frame, sub, -1, int64(e.prevGpQ14))
	add("state_prev_taming", frame, sub, -1, boolInt64(e.prevTaming))
	add("state_old_exc_energy", frame, sub, -1, oldExcEnergy(&e.oldExc))
	add("state_old_exc_max_abs", frame, sub, -1, int64(oldExcMaxAbs(&e.oldExc)))
	for i, value := range e.pastQuaEn {
		add("state_past_qua_en", frame, sub, i, int64(value))
	}

	kMin, kMax, prodLag, prodFrac := closedLoopSearchSnapshot(e, sub)
	add("local_window_min", frame, sub, -1, int64(kMin))
	add("local_window_max", frame, sub, -1, int64(kMax))
	add("local_int_lag", frame, sub, -1, int64(prodLag))
	add("local_frac", frame, sub, -1, int64(prodFrac))
	if sub == 0 {
		localP1 := clpitch.EncodeP1(prodLag, prodFrac)
		add("local_pitch_code", frame, sub, -1, int64(localP1))
		add("local_p0", frame, sub, -1, int64(clpitch.EncodeP0(localP1)))
	} else {
		tmin, _ := clpitch.Subframe2Window(e.intT1)
		add("local_pitch_code", frame, sub, -1, int64(clpitch.EncodeP2(prodLag, prodFrac, tmin)))
	}

	tap := diagnoseFCBCommitTap(e, sub, false, 0, 0)
	add("local_c", frame, sub, -1, int64(tap.c))
	add("local_s", frame, sub, -1, int64(tap.s))
	add("local_ga", frame, sub, -1, int64(tap.ga))
	add("local_gb", frame, sub, -1, int64(tap.gb))
	add("tap_gp_q14", frame, sub, -1, int64(tap.gpQ14))
	add("tap_gc_q12", frame, sub, -1, int64(tap.gcQ12))
	add("tap_taming", frame, sub, -1, boolInt64(tap.taming))
	add("tap_abs_pitch", frame, sub, -1, tap.absPitch)
	add("tap_abs_code", frame, sub, -1, tap.absCode)
	add("tap_saturation_count", frame, sub, -1, int64(tap.saturations))

	add("bitstream_pitch_code", frame, sub, -1, int64(refP))
	if refP0 >= 0 {
		add("bitstream_p0", frame, sub, -1, int64(refP0))
	}
	add("bitstream_int_lag", frame, sub, -1, int64(refInt))
	add("bitstream_frac", frame, sub, -1, int64(refFrac))
	add("bitstream_c", frame, sub, -1, int64(refC))
	add("bitstream_s", frame, sub, -1, int64(refS))
	add("bitstream_ga", frame, sub, -1, int64(refGA))
	add("bitstream_gb", frame, sub, -1, int64(refGB))
}

func boolInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func oldExcEnergy(oldExc *[154]int16) int64 {
	var energy int64
	for _, sample := range oldExc {
		s := int64(sample)
		energy += s * s
	}
	return energy
}

func oldExcMaxAbs(oldExc *[154]int16) int {
	var max int
	for _, sample := range oldExc {
		v := int(sample)
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	return max
}

func writeTAMEGainTamingCSV(path string, records []tameGainTamingRecord, valueColumn string, includeValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"field", "frame", "sub", "index", valueColumn}); err != nil {
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
			value,
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func readTAMEGainTamingValues(path, valueColumn string) (map[string]int64, error) {
	rows, err := readTAMEGainTamingRows(path)
	if err != nil {
		return nil, err
	}
	valueIdx := tameGainTamingHeaderIndex(rows[0], valueColumn)
	if valueIdx < 0 {
		return nil, fmt.Errorf("missing %q column", valueColumn)
	}
	out := make(map[string]int64, len(rows)-1)
	for line, row := range rows[1:] {
		key, err := tameGainTamingKey(row)
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

func readTAMEGainTamingExpected(path string) (map[string]int64, int, error) {
	rows, err := readTAMEGainTamingRows(path)
	if err != nil {
		return nil, 0, err
	}
	valueIdx := tameGainTamingHeaderIndex(rows[0], "expected")
	if valueIdx < 0 {
		return nil, 0, fmt.Errorf("missing expected column")
	}
	out := make(map[string]int64, len(rows)-1)
	var blanks int
	for line, row := range rows[1:] {
		if len(row) == valueIdx {
			row = append(row, "")
		}
		key, err := tameGainTamingKey(row)
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

func readTAMEGainTamingRows(path string) ([][]string, error) {
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

func tameGainTamingHeaderIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func tameGainTamingKey(row []string) (string, error) {
	if len(row) < 4 {
		return "", fmt.Errorf("short row")
	}
	return strings.Join(row[:4], "|"), nil
}
