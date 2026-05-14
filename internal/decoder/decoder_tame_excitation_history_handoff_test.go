package decoder

import (
	"fmt"
	"os"
	"testing"
)

const decoderTAMEExcitationHistoryExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_tame_excitation_history_expected_template.csv"

func TestDecoderTAMEExcitationHistoryTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_TAME_EXCITATION_HISTORY_TEMPLATE") != "1" {
		t.Skip("set G729_WRITE_DECODER_TAME_EXCITATION_HISTORY_TEMPLATE=1 to write decoder TAME excitation history template")
	}
	if err := guardDecoderVerifierExpectedTemplate(decoderTAMEExcitationHistoryExpectedTemplatePath, "expected"); err != nil {
		t.Fatal(err)
	}

	rows := make([]stageRow, 0, 117*2*subframeLen)
	for frame := 0; frame <= 116; frame++ {
		for sub := 0; sub < 2; sub++ {
			for i := 0; i < subframeLen; i++ {
				rows = append(rows, stageRow{
					source: "TAME",
					frame:  frame,
					sub:    sub,
					field:  "excitation_u_q0",
					index:  i,
				})
			}
		}
	}
	if err := writeStageCSV(decoderTAMEExcitationHistoryExpectedTemplatePath, "expected", rows); err != nil {
		t.Fatalf("write decoder TAME excitation history template: %v", err)
	}
	t.Logf("wrote %d rows to %s", len(rows), decoderTAMEExcitationHistoryExpectedTemplatePath)
}

func TestOracleHandoff_CompareDecoderTAMEExcitationHistory(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY") != "1" {
		t.Skip("set G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1 to compare decoder TAME excitation history artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_EXCITATION_HISTORY_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEExcitationHistoryExpectedTemplatePath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME excitation history expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder TAME excitation history expected is empty")
	}

	got, err := collectDecoderTAMEExcitationHistoryRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder TAME excitation history got rows: %v", err)
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

	t.Logf("decoder_tame_excitation_history: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, total, percent(exact, total), blankExpected, mismatches, missingGot)
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY") == "1" && blankExpected != 0 {
		t.Fatalf("decoder TAME excitation history expected incomplete: blanks=%d", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY") == "1" &&
		(blankExpected != 0 || missingGot != 0 || mismatches != 0) {
		t.Fatalf("decoder TAME excitation history mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, total, blankExpected, missingGot, mismatches)
	}
}

func collectDecoderTAMEExcitationHistoryRows(t testing.TB, expected []stageRow) ([]stageRow, error) {
	t.Helper()

	targets := make(map[int]map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		if row.field != "excitation_u_q0" {
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
			for i, v := range taps.Sub[sub].U {
				rows = append(rows, stageRow{
					source:   "TAME",
					frame:    fi,
					sub:      sub,
					field:    "excitation_u_q0",
					index:    i,
					hasValue: true,
					value:    int64(v),
				})
			}
		}
	}
	return rows, nil
}
