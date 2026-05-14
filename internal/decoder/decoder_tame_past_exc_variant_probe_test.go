package decoder

import (
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
)

func TestDecoderTAMEPastExcVariantProbe(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_PAST_EXC_VARIANT_PROBE") != "1" {
		t.Skip("set G729_DECODER_TAME_PAST_EXC_VARIANT_PROBE=1 to run TAME past-excitation variant probe")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_PAST_EXC_VARIANT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideOnsetExpectedTemplatePath
	}
	expected, err := readDecoderTAMEPastExcAgeRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME past-excitation expected: %v", err)
	}
	expected = decoderTAMEFilledPastExcRows(expected)
	if len(expected) == 0 {
		t.Fatalf("no filled past_exc_pre_acb_q0 rows in %s", expectedPath)
	}

	start := decoderTAMEVariantProbeEnvInt("G729_DECODER_TAME_PAST_EXC_VARIANT_START", 52)
	end := decoderTAMEVariantProbeEnvInt("G729_DECODER_TAME_PAST_EXC_VARIANT_END", 239)
	candidate := decoderTAMEPastExcProbeVariant(t)

	productionGot, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect production TAME past-excitation rows: %v", err)
	}
	candidateGot, err := collectDecoderTAMEPastExcRowsWithSubframeWindow(t, expected, start, end, candidate)
	if err != nil {
		t.Fatalf("collect candidate TAME past-excitation rows: %v", err)
	}

	prodStats := decoderTAMEPastExcCompareStats(expected, productionGot)
	candStats := decoderTAMEPastExcCompareStats(expected, candidateGot)
	t.Logf("decoder TAME pastExc variant probe: path=%s filled=%d candidate=%s sfWindow=[%d,%d)",
		expectedPath, len(expected), candidate.name, start, end)
	decoderTAMELogPastExcProbeStats(t, "production", prodStats)
	decoderTAMELogPastExcProbeStats(t, candidate.name, candStats)
	t.Logf("delta: errRMS=%+.2f meanAbs=%+.2f maxAbs=%+d corr=%+.4f scale=%+.4f",
		candStats.errRMS()-prodStats.errRMS(),
		candStats.meanAbs()-prodStats.meanAbs(),
		candStats.maxAbsDiff-prodStats.maxAbsDiff,
		candStats.corr()-prodStats.corr(),
		candStats.scale()-prodStats.scale())
}

func TestDecoderTAMEPastExcVariantWindowScan(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_PAST_EXC_VARIANT_WINDOW_SCAN") != "1" {
		t.Skip("set G729_DECODER_TAME_PAST_EXC_VARIANT_WINDOW_SCAN=1 to run TAME past-excitation variant window scan")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_PAST_EXC_VARIANT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideOnsetExpectedTemplatePath
	}
	expected, err := readDecoderTAMEPastExcAgeRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME past-excitation expected: %v", err)
	}
	expected = decoderTAMEFilledPastExcRows(expected)
	if len(expected) == 0 {
		t.Fatalf("no filled past_exc_pre_acb_q0 rows in %s", expectedPath)
	}

	scanStart := decoderTAMEVariantProbeEnvInt("G729_DECODER_TAME_PAST_EXC_SCAN_START", 112)
	scanEnd := decoderTAMEVariantProbeEnvInt("G729_DECODER_TAME_PAST_EXC_SCAN_END", 144)
	if scanStart < 0 || scanEnd <= scanStart {
		t.Fatalf("invalid TAME past-excitation scan window [%d,%d)", scanStart, scanEnd)
	}

	candidate := decoderTAMEPastExcProbeVariant(t)
	productionGot, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect production TAME past-excitation rows: %v", err)
	}
	prodStats := decoderTAMEPastExcCompareStats(expected, productionGot)

	rows := make([]decoderTAMEPastExcVariantWindowRow, 0, (scanEnd-scanStart)*(scanEnd-scanStart+1)/2)
	for start := scanStart; start < scanEnd; start++ {
		for end := start + 1; end <= scanEnd; end++ {
			got, err := collectDecoderTAMEPastExcRowsWithSubframeWindow(t, expected, start, end, candidate)
			if err != nil {
				t.Fatalf("collect candidate TAME past-excitation rows [%d,%d): %v", start, end, err)
			}
			stats := decoderTAMEPastExcCompareStats(expected, got)
			rows = append(rows, decoderTAMEPastExcVariantWindowRow{
				start: start,
				end:   end,
				stats: stats,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].stats.sumErrSq != rows[j].stats.sumErrSq {
			return rows[i].stats.sumErrSq < rows[j].stats.sumErrSq
		}
		if rows[i].start != rows[j].start {
			return rows[i].start < rows[j].start
		}
		return rows[i].end < rows[j].end
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}
	t.Logf("decoder TAME pastExc variant window scan: path=%s filled=%d candidate=%s scan=[%d,%d)",
		expectedPath, len(expected), candidate.name, scanStart, scanEnd)
	decoderTAMELogPastExcProbeStats(t, "production", prodStats)
	t.Logf("%-8s %-8s %-8s %8s %8s %8s %8s %8s %8s",
		"start", "end", "len", "exact", "errRMS", "meanAbs", "maxAbs", "corr", "scale")
	for _, row := range rows[:topN] {
		t.Logf("%-8d %-8d %-8d %8d %8.2f %8.2f %8d %8.4f %8.4f",
			row.start,
			row.end,
			row.end-row.start,
			row.stats.exact,
			row.stats.errRMS(),
			row.stats.meanAbs(),
			row.stats.maxAbsDiff,
			row.stats.corr(),
			row.stats.scale())
	}
}

type decoderTAMEPastExcVariantWindowRow struct {
	start int
	end   int
	stats decoderPastExcAgeGroup
}

func decoderTAMEVariantProbeEnvInt(name string, def int) int {
	value := os.Getenv(name)
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}
	return parsed
}

func decoderTAMEPastExcProbeVariant(t testing.TB) phase3eVariant {
	t.Helper()
	switch os.Getenv("G729_DECODER_TAME_PAST_EXC_VARIANT") {
	case "", "fixed_gain_half":
		return phase3eVariant{name: "fixed_gain_half", fixedExpDelta: -1}
	case "fixed_gain_double":
		return phase3eVariant{name: "fixed_gain_double", fixedExpDelta: +1}
	case "gain_unenhanced_c":
		return phase3eVariant{name: "gain_unenhanced_c", gainUnenhancedC: true}
	case "no_fcb_pitch_enhancement":
		return phase3eVariant{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true}
	case "force_pitch_frac_zero":
		return phase3eVariant{name: "force_pitch_frac_zero", forceTFracZero: true}
	case "flip_pitch_frac_sign":
		return phase3eVariant{name: "flip_pitch_frac_sign", flipTFracSign: true}
	case "pitch_gain_cap_0p95":
		return phase3eVariant{name: "pitch_gain_cap_0p95", pitchCapQ14: 15565}
	case "pitch_gain_cap_0p90":
		return phase3eVariant{name: "pitch_gain_cap_0p90", pitchCapQ14: 14746}
	case "pitch_gain_half":
		return phase3eVariant{name: "pitch_gain_half", pitchScaleNum: 1, pitchScaleDen: 2}
	default:
		t.Fatalf("unknown G729_DECODER_TAME_PAST_EXC_VARIANT")
	}
	return phase3eVariant{}
}

func decoderTAMEFilledPastExcRows(rows []stageRow) []stageRow {
	out := make([]stageRow, 0, len(rows))
	for _, row := range rows {
		if row.source == "TAME" && row.field == "past_exc_pre_acb_q0" && row.hasValue {
			out = append(out, row)
		}
	}
	return out
}

func collectDecoderTAMEPastExcRowsWithSubframeWindow(
	t testing.TB,
	expected []stageRow,
	startSubframe, endSubframe int,
	candidate phase3eVariant,
) ([]stageRow, error) {
	t.Helper()

	targets := make(map[int]map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" || row.field != "past_exc_pre_acb_q0" {
			continue
		}
		if _, ok := targets[row.frame]; !ok {
			targets[row.frame] = make(map[int]struct{})
		}
		targets[row.frame][row.sub] = struct{}{}
	}

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		return nil, errUnknownTAMEVector()
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	maxFrame := maxIntKey(targetFrameSet(targets))
	if maxFrame >= len(frames) {
		return nil, errTAMEFrameOutOfRange(maxFrame, len(frames))
	}

	var dec Decoder
	var rows []stageRow
	var out [frameSamples]int16
	for frame := 0; frame <= maxFrame; frame++ {
		if err := dec.decodeFramePhase3eVariantPastExcWindow(
			frame,
			frames[frame],
			out[:],
			startSubframe,
			endSubframe,
			candidate,
			targets[frame],
			&rows,
		); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func errUnknownTAMEVector() error {
	return errString("unknown ITU decoder vector source TAME")
}

func errTAMEFrameOutOfRange(frame, frames int) error {
	return errString("TAME target frame " + strconv.Itoa(frame) + " out of range; vector has " + strconv.Itoa(frames) + " frames")
}

type errString string

func (e errString) Error() string { return string(e) }

func (d *Decoder) decodeFramePhase3eVariantPastExcWindow(
	frame int,
	packed []byte,
	out []int16,
	startSubframe, endSubframe int,
	candidate phase3eVariant,
	targetSubs map[int]struct{},
	rows *[]stageRow,
) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(fr.L0),
		L1: uint8(fr.L1),
		L2: uint8(fr.L2),
		L3: uint8(fr.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	if _, ok := targetSubs[0]; ok {
		appendDecoderTAMEPastExcRows(rows, frame, 0, d.pastExc[:])
	}
	sf0Variant := phase3eVariant{name: "production"}
	if globalSubframe := frame * 2; globalSubframe >= startSubframe && globalSubframe < endSubframe {
		sf0Variant = candidate
	}
	d.decodeSubframePhase3eVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], sf0Variant)

	if _, ok := targetSubs[1]; ok {
		appendDecoderTAMEPastExcRows(rows, frame, 1, d.pastExc[:])
	}
	sf1Variant := phase3eVariant{name: "production"}
	if globalSubframe := frame*2 + 1; globalSubframe >= startSubframe && globalSubframe < endSubframe {
		sf1Variant = candidate
	}
	d.decodeSubframePhase3eVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], sf1Variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func appendDecoderTAMEPastExcRows(rows *[]stageRow, frame, sub int, values []int16) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      sub,
			field:    "past_exc_pre_acb_q0",
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

func decoderTAMEPastExcCompareStats(expected, got []stageRow) decoderPastExcAgeGroup {
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}
	var stats decoderPastExcAgeGroup
	for _, want := range expected {
		gotRow, ok := gotByKey[decoderStageRowKey(want)]
		if !ok || !gotRow.hasValue {
			continue
		}
		stats.add(decoderPastExcAgeSampleFromRows(want, gotRow))
	}
	return stats
}

func decoderTAMELogPastExcProbeStats(t *testing.T, label string, stats decoderPastExcAgeGroup) {
	t.Helper()
	t.Logf("%-16s count=%d exact=%d wantRMS=%.2f gotRMS=%.2f errRMS=%.2f meanAbs=%.2f maxAbs=%d corr=%.4f scale=%.4f",
		label,
		stats.count,
		stats.exact,
		stats.wantRMS(),
		stats.gotRMS(),
		stats.errRMS(),
		stats.meanAbs(),
		stats.maxAbsDiff,
		stats.corr(),
		stats.scale())
}
