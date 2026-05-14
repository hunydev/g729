package decoder

import (
	"fmt"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/pitch"
)

const (
	decoderPitchInstabilityDecisionExpectedTemplatePath = "/home/exedev/g729/testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv"
	decoderPitchInstabilityDecisionGotPath              = "/home/exedev/g729/testdata/oracle/handoff/decoder_pitch_instability_decision_got.csv"
)

func TestDecoderPitchInstabilityDecisionHandoffTemplate(t *testing.T) {
	if os.Getenv("G729_WRITE_DECODER_PITCH_INSTABILITY_DECISION_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_DECODER_PITCH_INSTABILITY_DECISION_HANDOFF=1 to write decoder pitch-instability decision handoff")
	}
	if err := guardDecoderVerifierExpectedTemplate(decoderPitchInstabilityDecisionExpectedTemplatePath, "expected"); err != nil {
		t.Fatal(err)
	}

	got, err := collectDecoderPitchInstabilityDecisionRows(t, decoderPitchInstabilityDecisionSources())
	if err != nil {
		t.Fatalf("collect decoder pitch-instability decision rows: %v", err)
	}
	if err := writeStageCSV(decoderPitchInstabilityDecisionGotPath, "got", got); err != nil {
		t.Fatalf("write decoder pitch-instability decision got: %v", err)
	}
	if err := writeStageCSV(decoderPitchInstabilityDecisionExpectedTemplatePath, "expected", blankDecoderStageRows(got)); err != nil {
		t.Fatalf("write decoder pitch-instability decision expected template: %v", err)
	}
	t.Logf("wrote decoder pitch-instability decision handoff: rows=%d got=%s expected_template=%s",
		len(got), decoderPitchInstabilityDecisionGotPath, decoderPitchInstabilityDecisionExpectedTemplatePath)
}

func TestOracleHandoff_CompareDecoderPitchInstabilityDecision(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_PITCH_INSTABILITY_DECISION") != "1" {
		t.Skip("set G729_COMPARE_DECODER_PITCH_INSTABILITY_DECISION=1 to compare decoder pitch-instability decision artifact")
	}

	expectedPath := os.Getenv("G729_DECODER_PITCH_INSTABILITY_DECISION_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderPitchInstabilityDecisionExpectedTemplatePath
	}
	expected, err := readDecoderStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder pitch-instability decision expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder pitch-instability decision expected is empty")
	}

	got, err := collectDecoderPitchInstabilityDecisionRowsFromExpected(t, expected)
	if err != nil {
		t.Fatalf("collect decoder pitch-instability decision got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var exact, filled, blankExpected, missingGot, mismatches int
	first := make([]decoderStageMismatch, 0, 16)
	for _, want := range expected {
		key := decoderStageRowKey(want)
		if !want.hasValue {
			blankExpected++
			continue
		}
		filled++
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

	t.Logf("decoder_pitch_instability_decision: exact %d/%d %.2f%% blanks=%d mismatches=%d missing_got=%d",
		exact, filled, percent(exact, filled), blankExpected, mismatches, missingGot)
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_COMPLETE_DECODER_PITCH_INSTABILITY_DECISION") == "1" && blankExpected != 0 {
		t.Fatalf("decoder pitch-instability decision expected incomplete: blanks=%d", blankExpected)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_PITCH_INSTABILITY_DECISION") == "1" &&
		(blankExpected != 0 || missingGot != 0 || mismatches != 0) {
		t.Fatalf("decoder pitch-instability decision mismatch: exact=%d/%d blanks=%d missing=%d mismatches=%d",
			exact, filled, blankExpected, missingGot, mismatches)
	}
}

func decoderPitchInstabilityDecisionSources() []string {
	return []string{"TAME", "SPEECH", "PITCH", "OVERFLOW"}
}

func collectDecoderPitchInstabilityDecisionRowsFromExpected(t *testing.T, expected []stageRow) ([]stageRow, error) {
	t.Helper()
	targets := make(map[string]map[int]map[int]struct{})
	for _, row := range expected {
		if !decoderPitchInstabilityDecisionFieldAllowed(row.field) {
			return nil, fmt.Errorf("unexpected field %q", row.field)
		}
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]map[int]struct{})
		}
		if _, ok := targets[row.source][row.frame]; !ok {
			targets[row.source][row.frame] = make(map[int]struct{})
		}
		targets[row.source][row.frame][row.sub] = struct{}{}
	}
	return collectDecoderPitchInstabilityDecisionRowsForTargets(t, targets)
}

func collectDecoderPitchInstabilityDecisionRows(t *testing.T, sources []string) ([]stageRow, error) {
	t.Helper()
	trigger := decoderTAMEDiagnosticPitchCapTrigger()
	targets := make(map[string]map[int]map[int]struct{})

	for _, source := range sources {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			return nil, fmt.Errorf("unknown ITU decoder vector source %q", source)
		}
		bitPath := vectorPath(tc.bitFile)
		ensureTestdataPresent(t, bitPath)
		frames, _ := readG192Frames(t, bitPath)
		bitData, err := os.ReadFile(bitPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", bitPath, err)
		}
		_, metrics := decoderHistoryDecodeWindow(t, bitData, len(frames), 0, 0, phase3eVariant{name: "production"})
		for _, metric := range metrics {
			feature := decoderTAMEPitchCapTriggerFeature{
				gpQ14:    metric.gpQ14,
				pastRMS:  metric.pastRMS,
				tailRMS:  metric.pastTailRMS,
				vRMS:     metric.vRMS,
				pitchRMS: metric.pitchRMS,
				fixedRMS: metric.fixedRMS,
			}
			if !trigger.match(feature) {
				continue
			}
			if _, ok := targets[source]; !ok {
				targets[source] = make(map[int]map[int]struct{})
			}
			if _, ok := targets[source][metric.frame]; !ok {
				targets[source][metric.frame] = make(map[int]struct{})
			}
			targets[source][metric.frame][metric.sub] = struct{}{}
		}
	}
	return collectDecoderPitchInstabilityDecisionRowsForTargets(t, targets)
}

func collectDecoderPitchInstabilityDecisionRowsForTargets(t *testing.T, targets map[string]map[int]map[int]struct{}) ([]stageRow, error) {
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
		bitData, err := os.ReadFile(bitPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", bitPath, err)
		}
		_, metrics := decoderHistoryDecodeWindow(t, bitData, len(frames), 0, 0, phase3eVariant{name: "production"})
		for _, metric := range metrics {
			sourceTargets, ok := targets[source]
			if !ok {
				continue
			}
			frameTargets, ok := sourceTargets[metric.frame]
			if !ok {
				continue
			}
			if _, ok := frameTargets[metric.sub]; !ok {
				continue
			}
			fr, err := decodePitchInstabilityFrameFields(frames[metric.frame], metric.sub)
			if err != nil {
				return nil, fmt.Errorf("%s frame %d sub %d fields: %w", source, metric.frame, metric.sub, err)
			}
			appendDecoderPitchInstabilityDecisionRows(&rows, source, &metric, fr)
		}
	}
	return rows, nil
}

type decoderPitchInstabilityFrameFields struct {
	ga    uint8
	gb    uint8
	tInt  int
	tFrac int
}

func decodePitchInstabilityFrameFields(packed []byte, sub int) (decoderPitchInstabilityFrameFields, error) {
	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return decoderPitchInstabilityFrameFields{}, err
	}
	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)
	if sub == 0 {
		return decoderPitchInstabilityFrameFields{
			ga:    uint8(fr.GA1),
			gb:    uint8(fr.GB1),
			tInt:  tInt1,
			tFrac: tFrac1,
		}, nil
	}
	return decoderPitchInstabilityFrameFields{
		ga:    uint8(fr.GA2),
		gb:    uint8(fr.GB2),
		tInt:  tInt2,
		tFrac: tFrac2,
	}, nil
}

func appendDecoderPitchInstabilityDecisionRows(rows *[]stageRow, source string, metric *decoderHistorySubframeMetrics, fr decoderPitchInstabilityFrameFields) {
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "bitstream_ga", -1, int64(fr.ga))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "bitstream_gb", -1, int64(fr.gb))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "pitch_t_int", -1, int64(fr.tInt))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "pitch_t_frac", -1, int64(fr.tFrac))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "adaptive_gain_before_q14", -1, int64(metric.gpQ14))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "pitch_instability_flag_q0", -1, 0)
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "adaptive_gain_after_pitch_instability_q14", -1, int64(metric.gpQ14))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "fixed_gain_q14", -1, gainQ14FromMantExp(metric.gcMantQ14, metric.gcExp))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "past_exc_rms_x100", -1, roundFloatToInt64(metric.pastRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "past_tail_rms_x100", -1, roundFloatToInt64(metric.pastTailRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "adaptive_v_rms_x100", -1, roundFloatToInt64(metric.vRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "pitch_contrib_rms_x100", -1, roundFloatToInt64(metric.pitchRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "fixed_contrib_rms_x100", -1, roundFloatToInt64(metric.fixedRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "excitation_u_rms_x100", -1, roundFloatToInt64(metric.uRMS*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "pitch_to_fixed_ratio_x100", -1, roundFloatToInt64(decoderTAMERatio(metric.pitchRMS, metric.fixedRMS)*100))
	appendDecoderPitchInstabilityDecisionRow(rows, source, metric, "past_to_fixed_ratio_x100", -1, roundFloatToInt64(decoderTAMERatio(metric.pastRMS, metric.fixedRMS)*100))
}

func appendDecoderPitchInstabilityDecisionRow(rows *[]stageRow, source string, metric *decoderHistorySubframeMetrics, field string, index int, value int64) {
	*rows = append(*rows, stageRow{
		source:   source,
		frame:    metric.frame,
		sub:      metric.sub,
		field:    field,
		index:    index,
		hasValue: true,
		value:    value,
	})
}

func decoderPitchInstabilityDecisionFieldAllowed(field string) bool {
	switch field {
	case "bitstream_ga",
		"bitstream_gb",
		"pitch_t_int",
		"pitch_t_frac",
		"adaptive_gain_before_q14",
		"pitch_instability_flag_q0",
		"adaptive_gain_after_pitch_instability_q14",
		"fixed_gain_q14",
		"past_exc_rms_x100",
		"past_tail_rms_x100",
		"adaptive_v_rms_x100",
		"pitch_contrib_rms_x100",
		"fixed_contrib_rms_x100",
		"excitation_u_rms_x100",
		"pitch_to_fixed_ratio_x100",
		"past_to_fixed_ratio_x100":
		return true
	default:
		return false
	}
}

func roundFloatToInt64(v float64) int64 {
	return int64(math.Round(v))
}
