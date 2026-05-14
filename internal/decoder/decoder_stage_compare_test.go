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

const decoderExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_stage_expected.csv"

type decoderStageKey struct {
	source string
	frame  int
	sub    int
	field  string
	index  int
}

type decoderStageMismatch struct {
	key  decoderStageKey
	want string
	got  string
	note string
}

func TestOracleHandoff_CompareDecoderStageHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_STAGE_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_DECODER_STAGE_HANDOFF=1 to compare decoder stage handoff")
	}

	expected, err := readDecoderStageRows(decoderExpectedPath)
	if err != nil {
		t.Fatalf("read decoder stage expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder stage expected is empty")
	}
	if same, err := decoderStageFilesEquivalent(decoderExpectedPath, decoderGotOut); err != nil {
		if !os.IsNotExist(err) {
			t.Logf("decoder_stage self-oracle check unavailable: %v", err)
		}
	} else if same {
		t.Logf("decoder_stage self-oracle warning: expected file is value-identical to local got file %s", decoderGotOut)
		if os.Getenv("G729_REJECT_DECODER_STAGE_SELF_ORACLE") == "1" {
			t.Fatalf("decoder stage expected appears to be derived from local got; require an independent numeric verifier artifact")
		}
	}

	got, err := collectDecoderStageGotRows(expected)
	if err != nil {
		t.Fatalf("collect decoder stage got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, blankExpected, missingGot, mismatches int
	fieldTotal := make(map[string]int)
	fieldExact := make(map[string]int)
	first := make([]decoderStageMismatch, 0, 12)

	for _, want := range expected {
		key := decoderStageRowKey(want)
		fieldTotal[key.field]++
		if !want.hasValue {
			blankExpected++
		}

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			if len(first) < cap(first) {
				first = append(first, decoderStageMismatch{
					key:  key,
					want: decoderStageValueString(want),
					got:  "",
					note: "missing got",
				})
			}
			continue
		}
		if want.hasValue == gotRow.hasValue && want.value == gotRow.value {
			exact++
			fieldExact[key.field]++
			continue
		}

		mismatches++
		if len(first) < cap(first) {
			first = append(first, decoderStageMismatch{
				key:  key,
				want: decoderStageValueString(want),
				got:  decoderStageValueString(gotRow),
				note: "mismatch",
			})
		}
	}

	total := len(expected)
	t.Logf("decoder_stage handoff: exact %d/%d %.2f%% mismatches=%d blank_expected=%d missing_got=%d",
		exact, total, percent(exact, total), mismatches, blankExpected, missingGot)
	for _, line := range decoderStageFieldSummary(fieldTotal, fieldExact, 12) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_STAGE_HANDOFF") == "1" && blankExpected > 0 {
		t.Fatalf("decoder stage expected has %d blank values", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_STAGE_HANDOFF") == "1" && mismatches > 0 {
		t.Fatalf("decoder stage expected/got mismatches: %d", mismatches)
	}
}

func readDecoderStageRows(path string) ([]stageRow, error) {
	return readDecoderStageRowsWithValueColumn(path, "expected")
}

func readDecoderStageRowsWithValueColumn(path, valueColumn string) ([]stageRow, error) {
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
	if (len(header) != 6 && len(header) != 7) ||
		header[0] != "source" ||
		header[1] != "frame" ||
		header[2] != "sub" ||
		header[3] != "field" ||
		header[4] != "index" ||
		header[5] != valueColumn {
		return nil, fmt.Errorf("unexpected header %v", header)
	}
	if len(header) == 7 && header[6] != "note" {
		return nil, fmt.Errorf("unexpected header %v", header)
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
		if len(rec) != len(header) {
			return nil, fmt.Errorf("line %d: got %d columns, want %d", line, len(rec), len(header))
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
				return nil, fmt.Errorf("line %d %s: %w", line, valueColumn, err)
			}
			row.hasValue = true
			row.value = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func decoderStageFilesEquivalent(expectedPath, gotPath string) (bool, error) {
	expected, err := readDecoderStageRowsWithValueColumn(expectedPath, "expected")
	if err != nil {
		return false, err
	}
	got, err := readDecoderStageRowsWithValueColumn(gotPath, "got")
	if err != nil {
		return false, err
	}
	if len(expected) != len(got) {
		return false, nil
	}

	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}
	for _, want := range expected {
		have, ok := gotByKey[decoderStageRowKey(want)]
		if !ok {
			return false, nil
		}
		if want.hasValue != have.hasValue || want.value != have.value {
			return false, nil
		}
	}
	return true, nil
}

func collectDecoderStageGotRows(expected []stageRow) ([]stageRow, error) {
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}

	var rows []stageRow
	if frames, ok := targets["SPEECH"]; ok {
		speechFrames, err := readSpeechFrames(speechPath)
		if err != nil {
			return nil, fmt.Errorf("read speech frames: %w", err)
		}
		arts := collectSpeechArtifacts(speechFrames, frames, maxIntKey(frames), &summaryState{})
		for _, frame := range sortedKeys(frames) {
			if art, ok := arts[frame]; ok {
				appendArtifactRows(&rows, "SPEECH", frame, art.cells)
			}
		}
	}

	var asteriskFrames [][]byte
	for _, source := range []string{"ASTERISK", "ASTERISK_VOICED"} {
		frames, ok := targets[source]
		if !ok {
			continue
		}
		if asteriskFrames == nil {
			var err error
			asteriskFrames, err = readAsteriskFrames(asteriskPath)
			if err != nil {
				return nil, fmt.Errorf("read asterisk frames: %w", err)
			}
		}
		arts := collectAsteriskArtifacts(asteriskFrames, frames, maxIntKey(frames), &summaryState{})
		for _, frame := range sortedKeys(frames) {
			if art, ok := arts[frame]; ok {
				appendArtifactRows(&rows, source, frame, art.cells)
			}
		}
	}

	return rows, nil
}

func decoderStageFieldSummary(total, exact map[string]int, limit int) []string {
	type item struct {
		field     string
		total     int
		exact     int
		mismatch  int
		exactRate float64
	}
	items := make([]item, 0, len(total))
	for field, n := range total {
		eq := exact[field]
		items = append(items, item{
			field:     field,
			total:     n,
			exact:     eq,
			mismatch:  n - eq,
			exactRate: percent(eq, n),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].mismatch != items[j].mismatch {
			return items[i].mismatch > items[j].mismatch
		}
		return items[i].field < items[j].field
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, fmt.Sprintf("field %s: exact %d/%d %.2f%% mismatches=%d",
			it.field, it.exact, it.total, it.exactRate, it.mismatch))
	}
	return out
}

func decoderStageRowKey(row stageRow) decoderStageKey {
	return decoderStageKey{
		source: row.source,
		frame:  row.frame,
		sub:    row.sub,
		field:  row.field,
		index:  row.index,
	}
}

func decoderStageValueString(row stageRow) string {
	if !row.hasValue {
		return ""
	}
	return strconv.FormatInt(row.value, 10)
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}
