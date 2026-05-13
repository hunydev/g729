package decoder

import (
	"fmt"
	"os"
	"testing"
)

const decoderTAMEPreACBHistoryExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_pre_acb_history_expected_template.csv"

func TestDecoderTAMEPreACBHistoryTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_TAME_PRE_ACB_HISTORY_TEMPLATE") != "1" {
		t.Skip("set G729_WRITE_DECODER_TAME_PRE_ACB_HISTORY_TEMPLATE=1 to write decoder TAME pre-ACB history template")
	}
	if err := guardDecoderVerifierExpectedTemplate(decoderTAMEPreACBHistoryExpectedTemplatePath, "expected"); err != nil {
		t.Fatal(err)
	}

	rows := make([]stageRow, 0, pastExcLen)
	for i := 0; i < pastExcLen; i++ {
		rows = append(rows, stageRow{
			source: "TAME",
			frame:  117,
			sub:    0,
			field:  "past_exc_pre_acb_q0",
			index:  i,
		})
	}
	if err := writeStageCSV(decoderTAMEPreACBHistoryExpectedTemplatePath, "expected", rows); err != nil {
		t.Fatalf("write decoder TAME pre-ACB history template: %v", err)
	}
	t.Logf("wrote %d rows to %s", len(rows), decoderTAMEPreACBHistoryExpectedTemplatePath)
}

func TestOracleHandoff_CompareDecoderTAMEPreACBHistory(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1 to compare decoder TAME pre-ACB history artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_PRE_ACB_HISTORY_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEPreACBHistoryExpectedTemplatePath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME pre-ACB history expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME pre-ACB history expected is empty")
	}

	got, err := collectDecoderTAMEPreACBHistoryRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder TAME pre-ACB history got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, total, blankExpected, missingGot, mismatches int
	first := make([]decoderStageMismatch, 0, 16)
	for _, want := range expected {
		key := decoderStageRowKey(want)
		if !want.hasValue {
			blankExpected++
			continue
		}
		total++
		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			mismatches++
			appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), "", "missing got")
			continue
		}
		if gotRow.hasValue && gotRow.value == want.value {
			exact++
			continue
		}
		mismatches++
		appendFrame0ChainMismatch(&first, key, decoderStageValueString(want), decoderStageValueString(gotRow), "mismatch")
	}

	t.Logf("decoder_tame_pre_acb_history: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, total, percent(exact, total), blankExpected, mismatches, missingGot)
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY") == "1" && blankExpected != 0 {
		t.Fatalf("decoder TAME pre-ACB history expected incomplete: blanks=%d", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY") == "1" &&
		(blankExpected != 0 || missingGot != 0 || mismatches != 0) {
		t.Fatalf("decoder TAME pre-ACB history mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, total, blankExpected, missingGot, mismatches)
	}
}

func collectDecoderTAMEPreACBHistoryRows(t testing.TB, expected []stageRow) ([]stageRow, error) {
	t.Helper()

	targets := make(map[int]map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		if row.field != "past_exc_pre_acb_q0" {
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
	for fi := 0; fi <= maxFrame; fi++ {
		taps, err := dec.DecodeWithTaps(frames[fi])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", fi, err)
		}
		subs, wantFrame := targets[fi]
		if !wantFrame {
			continue
		}
		for sub := range subs {
			if sub < 0 || sub >= len(taps.Sub) {
				return nil, fmt.Errorf("TAME frame %d invalid subframe %d", fi, sub)
			}
			for i, v := range taps.Sub[sub].PastExcPreACB {
				rows = append(rows, stageRow{
					source:   "TAME",
					frame:    fi,
					sub:      sub,
					field:    "past_exc_pre_acb_q0",
					index:    i,
					hasValue: true,
					value:    int64(v),
				})
			}
		}
	}
	return rows, nil
}

func targetFrameSet(targets map[int]map[int]struct{}) map[int]struct{} {
	out := make(map[int]struct{}, len(targets))
	for frame := range targets {
		out[frame] = struct{}{}
	}
	return out
}
