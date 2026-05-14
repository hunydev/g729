package decoder

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"testing"
)

const decoderTAMEACBCheckpointExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_acb_checkpoint_expected.csv"

func TestOracleHandoff_CompareDecoderTAMEACBCheckpoint(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_ACB_CHECKPOINT") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_ACB_CHECKPOINT=1 to compare decoder TAME ACB checkpoint artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_CHECKPOINT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	expected, err := readDecoderTAMEACBCheckpointRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME ACB checkpoint expected is empty")
	}

	got, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder TAME ACB checkpoint got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, filled, blankExpected, missingGot, mismatches int
	fieldStats := make(map[string]*decoderTAMEACBCheckpointFieldStats)
	first := make([]decoderStageMismatch, 0, 16)
	for _, want := range expected {
		key := decoderStageRowKey(want)
		st := decoderTAMEACBCheckpointStatsFor(fieldStats, key.field)
		st.total++
		if !want.hasValue {
			blankExpected++
			st.blank++
			continue
		}
		filled++
		st.filled++

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			st.mismatches++
			st.missing++
			appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), "", "missing got")
			continue
		}
		if gotRow.hasValue && gotRow.value == want.value {
			exact++
			st.exact++
			continue
		}
		mismatches++
		st.mismatches++
		delta := absInt64(want.value - gotRow.value)
		if delta > st.maxAbs {
			st.maxAbs = delta
		}
		appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), decoderStageValueString(gotRow), "mismatch")
	}

	t.Logf("decoder_tame_acb_checkpoint: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, filled, percent(exact, filled), blankExpected, mismatches, missingGot)
	for _, line := range decoderTAMEACBCheckpointFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_TAME_ACB_CHECKPOINT") == "1" && blankExpected != 0 {
		t.Fatalf("decoder TAME ACB checkpoint expected incomplete: blanks=%d", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_ACB_CHECKPOINT") == "1" &&
		(blankExpected != 0 || missingGot != 0 || mismatches != 0) {
		t.Fatalf("decoder TAME ACB checkpoint mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, filled, blankExpected, missingGot, mismatches)
	}
}

func readDecoderTAMEACBCheckpointRows(path string) ([]stageRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) != 7 ||
		header[0] != "source" ||
		header[1] != "frame" ||
		header[2] != "sub" ||
		header[3] != "field" ||
		header[4] != "index" ||
		header[5] != "expected" ||
		header[6] != "note" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	allowedNotes := map[string]bool{
		"blank_unavailable": true,
		"formula_ok":        true,
		"independent_trace": true,
		"pst_file":          true,
	}

	var rows []stageRow
	line := 1
	for {
		rec, err := r.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != 7 {
			return nil, fmt.Errorf("line %d: got %d columns, want 7", line, len(rec))
		}
		if !allowedNotes[rec[6]] {
			return nil, fmt.Errorf("line %d: unsupported note %q", line, rec[6])
		}
		frame, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d frame: %w", line, err)
		}
		sub, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("line %d sub: %w", line, err)
		}
		index, err := strconv.Atoi(rec[4])
		if err != nil {
			return nil, fmt.Errorf("line %d index: %w", line, err)
		}
		row := stageRow{
			source: rec[0],
			frame:  frame,
			sub:    sub,
			field:  rec[3],
			index:  index,
		}
		if rec[5] != "" {
			value, err := strconv.ParseInt(rec[5], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("line %d expected: %w", line, err)
			}
			row.hasValue = true
			row.value = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func collectDecoderTAMEACBCheckpointRows(t testing.TB, expected []stageRow) ([]stageRow, error) {
	t.Helper()

	targets := make(map[int]map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		switch row.field {
		case "adaptive_v_q0", "past_exc_pre_acb_q0", "excitation_u_q0", "adaptive_gain_q14", "fixed_gain_q14":
		default:
			return nil, fmt.Errorf("unexpected field %q", row.field)
		}
		if _, ok := targets[row.frame]; !ok {
			targets[row.frame] = make(map[int]struct{})
		}
		targets[row.frame][row.sub] = struct{}{}
	}

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		return nil, fmt.Errorf("unknown ITU decoder vector source TAME")
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	maxFrame := maxIntKey(targetFrameSet(targets))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	var dec Decoder
	var rows []stageRow
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTaps(frames[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		subs, wantFrame := targets[frame]
		if !wantFrame {
			continue
		}
		for sub := range subs {
			if sub < 0 || sub >= len(taps.Sub) {
				return nil, fmt.Errorf("TAME frame %d invalid subframe %d", frame, sub)
			}
			appendDecoderTAMEACBCheckpointGotRows(&rows, frame, sub, &taps.Sub[sub])
		}
	}
	return rows, nil
}

func appendDecoderTAMEACBCheckpointGotRows(rows *[]stageRow, frame, sub int, taps *Phase3DiagSubframeTaps) {
	appendDecoderTAMEACBArrayRows(rows, frame, sub, "adaptive_v_q0", taps.V[:])
	appendDecoderTAMEACBArrayRows(rows, frame, sub, "past_exc_pre_acb_q0", taps.PastExcPreACB[:])
	appendDecoderTAMEACBArrayRows(rows, frame, sub, "excitation_u_q0", taps.U[:])
	*rows = append(*rows, stageRow{
		source:   "TAME",
		frame:    frame,
		sub:      sub,
		field:    "adaptive_gain_q14",
		index:    -1,
		hasValue: true,
		value:    int64(taps.GpQ14),
	})
	*rows = append(*rows, stageRow{
		source:   "TAME",
		frame:    frame,
		sub:      sub,
		field:    "fixed_gain_q14",
		index:    -1,
		hasValue: true,
		value:    gainQ14FromMantExp(taps.GainTaps.GcMantQ14, taps.GainTaps.GcExp),
	})
}

func appendDecoderTAMEACBArrayRows(rows *[]stageRow, frame, sub int, field string, values []int16) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      sub,
			field:    field,
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

func decoderTAMEACBSubframeOverrides(t *testing.T, path string) map[decoderFrameSubKey][subframeLen]int16 {
	t.Helper()
	if decoderCSVHasHeaderPrefix(t, path, "frame", "sub") {
		return decoderTAMEWideSubframeOverrides(t, path, "adaptive_v_q0")
	}
	rows, err := readDecoderTAMEACBCheckpointRows(path)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}
	return decoderTAMESubframeOverridesFromStageRows(t, rows, "adaptive_v_q0")
}

func decoderTAMESubframeOverridesFromStageRows(t *testing.T, rows []stageRow, field string) map[decoderFrameSubKey][subframeLen]int16 {
	t.Helper()
	type build struct {
		values [subframeLen]int16
		set    [subframeLen]bool
	}
	builds := make(map[decoderFrameSubKey]*build)
	for _, row := range rows {
		if row.field != field || !row.hasValue {
			continue
		}
		if row.index < 0 || row.index >= subframeLen {
			t.Fatalf("%s index out of range: frame=%d sub=%d index=%d", field, row.frame, row.sub, row.index)
		}
		if row.value < -32768 || row.value > 32767 {
			t.Fatalf("%s value out of int16 range: frame=%d sub=%d index=%d value=%d",
				field, row.frame, row.sub, row.index, row.value)
		}
		key := decoderFrameSubKey{frame: row.frame, sub: row.sub}
		b := builds[key]
		if b == nil {
			b = &build{}
			builds[key] = b
		}
		b.values[row.index] = int16(row.value)
		b.set[row.index] = true
	}

	out := make(map[decoderFrameSubKey][subframeLen]int16)
	for key, b := range builds {
		for i, ok := range b.set {
			if !ok {
				t.Fatalf("incomplete %s override: frame=%d sub=%d missing index=%d", field, key.frame, key.sub, i)
			}
		}
		out[key] = b.values
	}
	return out
}

func decoderCSVHasHeaderPrefix(t *testing.T, path string, prefix ...string) bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	header, err := csv.NewReader(f).Read()
	if err != nil {
		t.Fatalf("read %s header: %v", path, err)
	}
	if len(header) < len(prefix) {
		return false
	}
	for i, want := range prefix {
		if header[i] != want {
			return false
		}
	}
	return true
}

type decoderTAMEACBCheckpointFieldStats struct {
	total      int
	filled     int
	blank      int
	exact      int
	mismatches int
	missing    int
	maxAbs     int64
}

func decoderTAMEACBCheckpointStatsFor(stats map[string]*decoderTAMEACBCheckpointFieldStats, field string) *decoderTAMEACBCheckpointFieldStats {
	st := stats[field]
	if st == nil {
		st = &decoderTAMEACBCheckpointFieldStats{}
		stats[field] = st
	}
	return st
}

func decoderTAMEACBCheckpointFieldSummary(stats map[string]*decoderTAMEACBCheckpointFieldStats) []string {
	fields := make([]string, 0, len(stats))
	for field := range stats {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool {
		left := stats[fields[i]]
		right := stats[fields[j]]
		if left.mismatches != right.mismatches {
			return left.mismatches > right.mismatches
		}
		if left.filled != right.filled {
			return left.filled > right.filled
		}
		return fields[i] < fields[j]
	})

	out := make([]string, 0, len(fields))
	for _, field := range fields {
		st := stats[field]
		out = append(out, fmt.Sprintf("field %s: exact %d/%d %.2f%% blanks=%d mismatches=%d missing=%d maxAbs=%d",
			field, st.exact, st.filled, percent(st.exact, st.filled), st.blank, st.mismatches, st.missing, st.maxAbs))
	}
	return out
}
