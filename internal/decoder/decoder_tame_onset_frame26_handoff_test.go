package decoder

import (
	"fmt"
	"os"
	"sort"
	"testing"
)

const decoderTAMEOnsetFrame26ExpectedPath = "/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_onset_frame26_expected.csv"

func TestOracleHandoff_CompareDecoderTAMEOnsetFrame26(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_ONSET_FRAME26") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_ONSET_FRAME26=1 to compare decoder TAME frame-26 onset artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ONSET_FRAME26_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEOnsetFrame26ExpectedPath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME frame-26 onset expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME frame-26 onset expected is empty")
	}

	got, err := collectDecoderTAMEOnsetFrame26RowsFromExpected(t, expected)
	if err != nil {
		t.Fatalf("collect decoder TAME frame-26 onset got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, filled, blanks, missingGot, mismatches int
	fieldStats := make(map[string]*decoderTAMEWideFieldStats)
	first := make([]decoderStageMismatch, 0, 16)
	for _, want := range expected {
		if !want.hasValue {
			blanks++
			continue
		}
		filled++
		key := decoderStageRowKey(want)
		st := decoderTAMEWideStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			st.missing++
			st.mismatches++
			if len(first) < cap(first) {
				first = append(first, decoderStageMismatch{key: key, want: decoderStageValueString(want), note: "missing got"})
			}
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
		if len(first) < cap(first) {
			first = append(first, decoderStageMismatch{
				key:  key,
				want: decoderStageValueString(want),
				got:  decoderStageValueString(gotRow),
				note: "mismatch",
			})
		}
	}

	t.Logf("decoder_tame_onset_frame26: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, filled, percent(exact, filled), blanks, mismatches, missingGot)
	for _, line := range decoderTAMEWideFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_TAME_ONSET_FRAME26") == "1" && blanks > 0 {
		t.Fatalf("decoder TAME frame-26 onset expected has blank cells: %d", blanks)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_ONSET_FRAME26") == "1" &&
		(mismatches != 0 || missingGot != 0) {
		t.Fatalf("decoder TAME frame-26 onset mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, filled, blanks, missingGot, mismatches)
	}
}

func collectDecoderTAMEOnsetFrame26RowsFromExpected(t testing.TB, expected []stageRow) ([]stageRow, error) {
	t.Helper()

	targets := make(map[string]map[int]struct{})
	subTargets := make(map[string]map[int]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
		if row.sub >= 0 {
			if _, ok := subTargets[row.source]; !ok {
				subTargets[row.source] = make(map[int]map[int]struct{})
			}
			if _, ok := subTargets[row.source][row.frame]; !ok {
				subTargets[row.source][row.frame] = make(map[int]struct{})
			}
			subTargets[row.source][row.frame][row.sub] = struct{}{}
		}
	}

	rows, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		return nil, err
	}
	bitstreamRows, err := collectDecoderTAMEOnsetFrame26BitstreamRows(t, subTargets)
	if err != nil {
		return nil, err
	}
	rows = append(rows, bitstreamRows...)
	return rows, nil
}

func collectDecoderTAMEOnsetFrame26BitstreamRows(t testing.TB, targets map[string]map[int]map[int]struct{}) ([]stageRow, error) {
	t.Helper()

	sources := make([]string, 0, len(targets))
	for source := range targets {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var rows []stageRow
	for _, source := range sources {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown ITU decoder vector source %q", source)
		}
		bitPath := vectorPath(tc.bitFile)
		ensureTestdataPresent(t, bitPath)
		frames, _ := readG192Frames(t, bitPath)

		frameKeys := sortedDecoderTAMEOnsetFrame26Frames(targets[source])
		for _, frame := range frameKeys {
			if frame < 0 || frame >= len(frames) {
				return nil, fmt.Errorf("%s target frame %d out of range; vector has %d frames", source, frame, len(frames))
			}
			for _, sub := range sortedKeys(targets[source][frame]) {
				fr, err := decodePitchInstabilityFrameFields(frames[frame], sub)
				if err != nil {
					return nil, fmt.Errorf("%s frame %d sub %d fields: %w", source, frame, sub, err)
				}
				rows = append(rows,
					stageRow{source: source, frame: frame, sub: sub, field: "bitstream_ga", index: -1, hasValue: true, value: int64(fr.ga)},
					stageRow{source: source, frame: frame, sub: sub, field: "bitstream_gb", index: -1, hasValue: true, value: int64(fr.gb)},
					stageRow{source: source, frame: frame, sub: sub, field: "pitch_t_int", index: -1, hasValue: true, value: int64(fr.tInt)},
					stageRow{source: source, frame: frame, sub: sub, field: "pitch_t_frac", index: -1, hasValue: true, value: int64(fr.tFrac)},
				)
			}
		}
	}
	return rows, nil
}

func sortedDecoderTAMEOnsetFrame26Frames(values map[int]map[int]struct{}) []int {
	out := make([]int, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}
