package decoder

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

const (
	decoderITUStageExpectedPath        = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_stage_expected.csv"
	decoderITUStageExpectedTemplateOut = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_stage_expected_template.csv"
	decoderITUStageGotOut              = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_stage_got.csv"
)

// TestDecoderITUStageHandoffTemplate writes a clean-room numeric verifier
// handoff for the decoder ITU vector gate. The expected template intentionally
// contains blank values; an external verifier may fill only numeric scalar
// values derived from ITU test vectors, without contributing implementation
// code or implementation-derived branch descriptions.
func TestDecoderITUStageHandoffTemplate(t *testing.T) {
	if os.Getenv("G729_DUMP_DECODER_ITU_STAGE_HANDOFF") != "1" {
		t.Skip("set G729_DUMP_DECODER_ITU_STAGE_HANDOFF=1 to write decoder ITU stage handoff CSVs")
	}

	targets := decoderITUStageDefaultTargets()
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder ITU stage rows: %v", err)
	}
	if err := writeStageCSV(decoderITUStageGotOut, "got", got); err != nil {
		t.Fatalf("write decoder ITU stage got: %v", err)
	}
	if err := writeStageCSV(decoderITUStageExpectedTemplateOut, "expected", blankDecoderStageRows(got)); err != nil {
		t.Fatalf("write decoder ITU stage expected template: %v", err)
	}
	t.Logf("wrote %d rows to %s and %s", len(got), decoderITUStageGotOut, decoderITUStageExpectedTemplateOut)
}

func TestOracleHandoff_CompareDecoderITUStageHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_ITU_STAGE_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_DECODER_ITU_STAGE_HANDOFF=1 to compare decoder ITU stage handoff")
	}

	expectedPath := os.Getenv("G729_DECODER_ITU_STAGE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderITUStageExpectedPath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder ITU stage expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder ITU stage expected is empty")
	}
	if same, err := decoderStageFilesEquivalent(expectedPath, decoderITUStageGotOut); err != nil {
		if !os.IsNotExist(err) {
			t.Logf("decoder_itu_stage self-oracle check unavailable: %v", err)
		}
	} else if same {
		t.Logf("decoder_itu_stage self-oracle warning: expected file is value-identical to local got file %s", decoderITUStageGotOut)
		if os.Getenv("G729_REJECT_DECODER_ITU_STAGE_SELF_ORACLE") == "1" {
			t.Fatalf("decoder ITU stage expected appears to be derived from local got; require an independent numeric verifier artifact")
		}
	}

	got, err := collectDecoderITUStageRowsFromExpected(t, expected)
	if err != nil {
		t.Fatalf("collect decoder ITU stage got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, blankExpected, missingGot, mismatches int
	var filledExact, filledMissing, filledMismatches, filledTotal int
	fieldTotal := make(map[string]int)
	fieldExact := make(map[string]int)
	filledFieldTotal := make(map[string]int)
	filledFieldExact := make(map[string]int)
	first := make([]decoderStageMismatch, 0, 12)
	firstFilled := make([]decoderStageMismatch, 0, 12)

	for _, want := range expected {
		key := decoderStageRowKey(want)
		fieldTotal[key.field]++
		if !want.hasValue {
			blankExpected++
		} else {
			filledTotal++
			filledFieldTotal[key.field]++
		}

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			if want.hasValue {
				filledMissing++
				filledMismatches++
				if len(firstFilled) < cap(firstFilled) {
					firstFilled = append(firstFilled, decoderStageMismatch{
						key:  key,
						want: decoderStageValueString(want),
						got:  "",
						note: "missing got",
					})
				}
			}
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
			if want.hasValue {
				filledExact++
				filledFieldExact[key.field]++
			}
			continue
		}

		mismatches++
		if want.hasValue {
			filledMismatches++
			if len(firstFilled) < cap(firstFilled) {
				firstFilled = append(firstFilled, decoderStageMismatch{
					key:  key,
					want: decoderStageValueString(want),
					got:  decoderStageValueString(gotRow),
					note: "mismatch",
				})
			}
		}
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
	t.Logf("decoder_itu_stage handoff: exact %d/%d %.2f%% mismatches=%d blank_expected=%d missing_got=%d",
		exact, total, percent(exact, total), mismatches, blankExpected, missingGot)
	t.Logf("decoder_itu_stage filled cells: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		filledExact, filledTotal, percent(filledExact, filledTotal), filledMismatches, filledMissing)
	for _, line := range decoderStageFieldSummary(fieldTotal, fieldExact, 16) {
		t.Log(line)
	}
	for _, line := range decoderStageFieldSummary(filledFieldTotal, filledFieldExact, 16) {
		t.Log("filled " + line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}
	for i, m := range firstFilled {
		t.Logf("filled_mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_ITU_STAGE_HANDOFF") == "1" && blankExpected > 0 {
		t.Fatalf("decoder ITU stage expected has %d blank values", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_STAGE_HANDOFF") == "1" && mismatches > 0 {
		t.Fatalf("decoder ITU stage expected/got mismatches: %d", mismatches)
	}
}

func decoderITUStageDefaultTargets() map[string]map[int]struct{} {
	return map[string]map[int]struct{}{
		"ALGTHM":   intSet(0, 14, 15),
		"TAME":     intSet(0, 98, 117, 118, 119, 123),
		"OVERFLOW": intSet(0, 106, 107, 108),
	}
}

func collectDecoderITUStageRowsFromExpected(t testing.TB, expected []stageRow) ([]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}
	return collectDecoderITUStageRows(t, targets)
}

func collectDecoderITUStageRows(t testing.TB, targets map[string]map[int]struct{}) ([]stageRow, error) {
	t.Helper()
	sources := make([]string, 0, len(targets))
	for source := range targets {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var rows []stageRow
	for _, source := range sources {
		frames := targets[source]
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown ITU decoder vector source %q", source)
		}
		sourceRows, err := collectDecoderITUStageRowsForCase(t, tc, frames)
		if err != nil {
			return nil, err
		}
		rows = append(rows, sourceRows...)
	}
	return rows, nil
}

func collectDecoderITUStageRowsForCase(t testing.TB, tc decoderITUValidationCase, targets map[int]struct{}) ([]stageRow, error) {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	frames, _ := readG192Frames(t, bitPath)
	maxFrame := maxIntKey(targets)
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", tc.name, maxFrame, len(frames))
	}

	var dec Decoder
	var rows []stageRow
	for fi := 0; fi <= maxFrame; fi++ {
		taps, err := dec.DecodeWithTaps(frames[fi])
		if err != nil {
			return nil, fmt.Errorf("%s frame %d DecodeWithTaps: %w", tc.name, fi, err)
		}
		if _, want := targets[fi]; !want {
			continue
		}
		art, err := collectFrameFromDecoded(fi, frames[fi], &taps)
		if err != nil {
			return nil, fmt.Errorf("%s frame %d collect: %w", tc.name, fi, err)
		}
		appendArtifactRows(&rows, tc.name, fi, art.cells)
	}
	return rows, nil
}

func blankDecoderStageRows(src []stageRow) []stageRow {
	out := make([]stageRow, len(src))
	for i, row := range src {
		row.hasValue = false
		row.value = 0
		out[i] = row
	}
	return out
}

func intSet(values ...int) map[int]struct{} {
	out := make(map[int]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
