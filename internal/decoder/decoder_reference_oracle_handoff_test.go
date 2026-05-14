package decoder

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

const decoderReferenceOracleDefaultDir = "/home/exedev/g729_untracked/verifier-output"

func TestOracleHandoff_CompareDecoderReferenceFinalPCM(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_FINAL_PCM") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 to compare external reference final PCM oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_final_pcm_expected.csv")
	expected, err := readDecoderReferenceFinalPCMExpected(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference final PCM expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference final PCM expected is empty")
	}

	requireExact := os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM") == "1"
	sources := make([]string, 0, len(expected))
	for source := range expected {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	var total decoderReferencePCMStats
	for _, source := range sources {
		tc, ok := decoderITUValidationCaseByName(source)
		if !ok {
			t.Fatalf("unknown decoder reference vector source %q", source)
		}
		got := decodeDecoderReferenceVectorPCM(t, tc)
		stats := compareDecoderReferencePCM(source, got, expected[source])
		total.add(stats)
		t.Logf("%-10s exact=%d/%d %.2f%% mismatches=%d max_abs=%d mean_abs=%.2f first=%s extra_got=%d",
			source, stats.exact, stats.total, percent(stats.exact, stats.total),
			stats.mismatches, stats.maxAbs, stats.meanAbsDelta(), stats.firstDiffString(), stats.extraGot)
		if requireExact && stats.mismatches != 0 {
			t.Errorf("%s final PCM mismatch: exact=%d/%d first=%s got=%d want=%d max_abs=%d",
				source, stats.exact, stats.total, stats.firstDiffString(),
				stats.firstGot, stats.firstWant, stats.maxAbs)
		}
	}

	t.Logf("%-10s exact=%d/%d %.2f%% mismatches=%d max_abs=%d mean_abs=%.2f first=%s extra_got=%d",
		"TOTAL", total.exact, total.total, percent(total.exact, total.total),
		total.mismatches, total.maxAbs, total.meanAbsDelta(), total.firstDiffString(), total.extraGot)
	if requireExact && total.mismatches != 0 {
		t.Fatalf("decoder reference final PCM exact gate failed: mismatches=%d/%d",
			total.mismatches, total.total)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEFullStage(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_STAGE") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_STAGE=1 to compare external reference TAME full-stage oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_full_stage_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME stage expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME stage expected is empty")
	}

	got, err := collectDecoderReferenceTAMEStageRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME stage rows: %v", err)
	}

	fieldStats := make(map[string]*decoderReferenceStageFieldStats)
	first := make([]decoderStageMismatch, 0, 16)
	var exact, missingGot, mismatches int
	for _, want := range expected {
		key := decoderStageRowKey(want)
		st := decoderReferenceStageStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := got[key]
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

	t.Logf("decoder_reference_tame_stage: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_STAGE") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME stage exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func TestOracleHandoff_CompareDecoderReferenceTAMEGainInternals(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_INTERNALS") != "1" {
		t.Skip("set G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_INTERNALS=1 to compare external reference TAME gain-internals oracle")
	}

	expectedPath := decoderReferenceOraclePath("decoder_tame_gain_internals_expected.csv")
	expected, err := readDecoderReferenceStageRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder reference TAME gain internals expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("decoder reference TAME gain internals expected is empty")
	}

	got, err := collectDecoderReferenceTAMEGainInternalRows(t, expected)
	if err != nil {
		t.Fatalf("collect decoder reference TAME gain internals rows: %v", err)
	}

	fieldStats := make(map[string]*decoderReferenceStageFieldStats)
	first := make([]decoderStageMismatch, 0, 16)
	var exact, missingGot, mismatches int
	for _, want := range expected {
		key := decoderStageRowKey(want)
		st := decoderReferenceStageStatsFor(fieldStats, key.field)
		st.total++

		gotRow, ok := got[key]
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

	t.Logf("decoder_reference_tame_gain_internals: exact %d/%d %.2f%% mismatches=%d missing_got=%d",
		exact, len(expected), percent(exact, len(expected)), mismatches, missingGot)
	for _, line := range decoderReferenceStageFieldSummary(fieldStats) {
		t.Log(line)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_GAIN_INTERNALS") == "1" && mismatches != 0 {
		t.Fatalf("decoder reference TAME gain internals exact gate failed: mismatches=%d/%d missing_got=%d",
			mismatches, len(expected), missingGot)
	}
}

func decoderReferenceOraclePath(name string) string {
	dir := os.Getenv("G729_DECODER_REFERENCE_ORACLE_DIR")
	if dir == "" {
		dir = decoderReferenceOracleDefaultDir
	}
	return filepath.Join(dir, name)
}

type decoderReferencePCMBuild struct {
	samples []int16
	set     []bool
}

func readDecoderReferenceFinalPCMExpected(path string) (map[string][]int16, error) {
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
	if len(header) != 5 ||
		header[0] != "source" ||
		header[1] != "frame" ||
		header[2] != "index" ||
		header[3] != "expected" ||
		header[4] != "note" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	builds := make(map[string]*decoderReferencePCMBuild)
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
		if len(rec) != 5 {
			return nil, fmt.Errorf("line %d: got %d columns, want 5", line, len(rec))
		}
		frame, err := strconv.Atoi(rec[1])
		if err != nil {
			return nil, fmt.Errorf("line %d frame: %w", line, err)
		}
		index, err := strconv.Atoi(rec[2])
		if err != nil {
			return nil, fmt.Errorf("line %d index: %w", line, err)
		}
		if frame < 0 || index < 0 || index >= frameSamples {
			return nil, fmt.Errorf("line %d invalid frame/index %d/%d", line, frame, index)
		}
		value64, err := strconv.ParseInt(rec[3], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("line %d expected: %w", line, err)
		}
		if rec[4] == "" {
			return nil, fmt.Errorf("line %d missing note", line)
		}

		pos := frame*frameSamples + index
		build := builds[rec[0]]
		if build == nil {
			build = &decoderReferencePCMBuild{}
			builds[rec[0]] = build
		}
		if pos >= len(build.samples) {
			newLen := pos + 1
			build.samples = append(build.samples, make([]int16, newLen-len(build.samples))...)
			build.set = append(build.set, make([]bool, newLen-len(build.set))...)
		}
		if build.set[pos] {
			return nil, fmt.Errorf("line %d duplicate sample source=%s frame=%d index=%d", line, rec[0], frame, index)
		}
		build.samples[pos] = int16(value64)
		build.set[pos] = true
	}

	out := make(map[string][]int16, len(builds))
	for source, build := range builds {
		for i, ok := range build.set {
			if !ok {
				return nil, fmt.Errorf("%s missing sample at frame=%d index=%d", source, i/frameSamples, i%frameSamples)
			}
		}
		out[source] = build.samples
	}
	return out, nil
}

func readDecoderReferenceStageRows(path string) ([]stageRow, error) {
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
		value, err := strconv.ParseInt(rec[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d expected: %w", line, err)
		}
		if rec[6] == "" {
			return nil, fmt.Errorf("line %d missing note", line)
		}
		rows = append(rows, stageRow{
			source:   rec[0],
			frame:    frame,
			sub:      sub,
			field:    rec[3],
			index:    index,
			hasValue: true,
			value:    value,
		})
	}
	return rows, nil
}

func decodeDecoderReferenceVectorPCM(t testing.TB, tc decoderITUValidationCase) []int16 {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	ensureTestdataPresent(t, bitPath)

	frames, bads := readG192Frames(t, bitPath)
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch: bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	out := make([]int16, len(frames)*frameSamples)
	var dec Decoder
	for frame, packed := range frames {
		if err := dec.Decode(packed, bads[frame], out[frame*frameSamples:(frame+1)*frameSamples]); err != nil {
			t.Fatalf("%s frame %d Decode: %v", tc.name, frame, err)
		}
	}
	return out
}

type decoderReferencePCMStats struct {
	total       int
	exact       int
	mismatches  int
	extraGot    int
	firstGlobal int
	firstGot    int16
	firstWant   int16
	maxAbs      int
	sumAbs      int64
}

func compareDecoderReferencePCM(source string, got, want []int16) decoderReferencePCMStats {
	stats := decoderReferencePCMStats{firstGlobal: -1}
	n := len(want)
	stats.total = n
	limit := n
	if len(got) < limit {
		limit = len(got)
	}
	for i := 0; i < limit; i++ {
		if got[i] == want[i] {
			stats.exact++
			continue
		}
		stats.recordMismatch(i, got[i], want[i])
	}
	if len(got) < n {
		for i := len(got); i < n; i++ {
			stats.recordMismatch(i, 0, want[i])
		}
	}
	if len(got) > n {
		stats.extraGot = len(got) - n
		stats.mismatches += stats.extraGot
		if stats.firstGlobal < 0 {
			stats.firstGlobal = n
		}
	}
	_ = source
	return stats
}

func (s *decoderReferencePCMStats) recordMismatch(global int, got, want int16) {
	s.mismatches++
	if s.firstGlobal < 0 {
		s.firstGlobal = global
		s.firstGot = got
		s.firstWant = want
	}
	delta := int(got) - int(want)
	if delta < 0 {
		delta = -delta
	}
	if delta > s.maxAbs {
		s.maxAbs = delta
	}
	s.sumAbs += int64(delta)
}

func (s *decoderReferencePCMStats) add(other decoderReferencePCMStats) {
	offset := s.total
	s.total += other.total
	s.exact += other.exact
	s.mismatches += other.mismatches
	s.extraGot += other.extraGot
	s.sumAbs += other.sumAbs
	if other.maxAbs > s.maxAbs {
		s.maxAbs = other.maxAbs
	}
	if s.firstGlobal < 0 && other.firstGlobal >= 0 {
		s.firstGlobal = offset + other.firstGlobal
		s.firstGot = other.firstGot
		s.firstWant = other.firstWant
	}
}

func (s decoderReferencePCMStats) meanAbsDelta() float64 {
	if s.mismatches == 0 {
		return 0
	}
	return float64(s.sumAbs) / float64(s.mismatches)
}

func (s decoderReferencePCMStats) firstDiffString() string {
	if s.firstGlobal < 0 {
		return "-"
	}
	return strconv.Itoa(s.firstGlobal/frameSamples) + ":" + strconv.Itoa(s.firstGlobal%frameSamples)
}

func collectDecoderReferenceTAMEStageRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targetFrames := make(map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		targetFrames[row.frame] = struct{}{}
	}
	maxFrame := maxIntKey(targetFrames)

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		return nil, fmt.Errorf("unknown ITU decoder vector source TAME")
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTaps(frames[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEStageRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func collectDecoderReferenceTAMEGainInternalRows(t testing.TB, expected []stageRow) (map[decoderStageKey]stageRow, error) {
	t.Helper()
	targetFrames := make(map[int]struct{})
	for _, row := range expected {
		if row.source != "TAME" {
			return nil, fmt.Errorf("unexpected source %q", row.source)
		}
		targetFrames[row.frame] = struct{}{}
	}
	maxFrame := maxIntKey(targetFrames)

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		return nil, fmt.Errorf("unknown ITU decoder vector source TAME")
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	if maxFrame >= len(frames) {
		return nil, fmt.Errorf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	rows := make([]stageRow, 0, len(expected))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTaps(frames[frame])
		if err != nil {
			return nil, fmt.Errorf("TAME frame %d DecodeWithTaps: %w", frame, err)
		}
		if _, ok := targetFrames[frame]; !ok {
			continue
		}
		appendDecoderReferenceTAMEGainInternalRows(&rows, frame, &taps)
	}

	out := make(map[decoderStageKey]stageRow, len(rows))
	for _, row := range rows {
		out[decoderStageRowKey(row)] = row
	}
	return out, nil
}

func appendDecoderReferenceTAMEStageRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	appendDecoderReferenceFrameArray(rows, frame, "pcm_q0", taps.Output[:])
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_int", int64(st.TInt))
		appendDecoderReferenceScalar(rows, frame, sub, "pitch_t_frac", int64(st.TFrac))
		appendDecoderReferenceScalar(rows, frame, sub, "adaptive_gain_q14", int64(st.GpQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_q14", gainQ14FromMantExp(st.GainTaps.GcMantQ14, st.GainTaps.GcExp))

		var zero [subframeLen]int16
		var pitchContrib [subframeLen]int16
		var fixedContrib [subframeLen]int16
		synth.BuildExcitation(st.GpQ14, 0, 0, &st.V, &zero, &pitchContrib)
		synth.BuildExcitation(0, st.GainTaps.GcMantQ14, st.GainTaps.GcExp, &zero, &st.C, &fixedContrib)

		appendDecoderReferenceArray(rows, frame, sub, "past_exc_pre_acb_q0", st.PastExcPreACB[:])
		appendDecoderReferenceArray(rows, frame, sub, "adaptive_v_q0", st.V[:])
		appendDecoderReferenceArray(rows, frame, sub, "fixed_c_q13", st.C[:])
		appendDecoderReferenceArray(rows, frame, sub, "pitch_contrib_q0", pitchContrib[:])
		appendDecoderReferenceArray(rows, frame, sub, "fixed_contrib_q0", fixedContrib[:])
		appendDecoderReferenceArray(rows, frame, sub, "excitation_u_q0", st.U[:])
		appendDecoderReferenceArray(rows, frame, sub, "synth_s_q0", st.S[:])
		appendDecoderReferenceArray(rows, frame, sub, "postfilter_s_q0", st.SPf[:])
	}
}

func appendDecoderReferenceTAMEGainInternalRows(rows *[]stageRow, frame int, taps *Phase3DiagFrameTaps) {
	for sub := 0; sub < 2; sub++ {
		st := &taps.Sub[sub]
		g := st.GainTaps
		ga, gb := decoderReferenceGainIndices(taps, sub)

		appendDecoderReferenceScalar(rows, frame, sub, "bitstream_ga", ga)
		appendDecoderReferenceScalar(rows, frame, sub, "bitstream_gb", gb)
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_codebook_energy_q26", decoderTAMEFixedCodebookEnergy64(st.C[:]))
		appendDecoderReferenceScalar(rows, frame, sub, "predicted_energy_q10", int64(g.Predicted)-int64(tables.GainMeanEnergyQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "ec_bar_q10", int64(tables.GainMeanEnergyQ10)-int64(g.EcBarDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "log_gain_q10", int64(g.LogGainDbQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "log2_gc_q10", int64(g.Log2GcQ10))
		appendDecoderReferenceScalar(rows, frame, sub, "gamma_q13", int64(g.GammaCQ13))
		appendDecoderReferenceScalar(rows, frame, sub, "gc0_q14", int64(g.Gc0MantQ14))
		appendDecoderReferenceScalar(rows, frame, sub, "fixed_gain_q14", gainQ14FromMantExp(g.GcMantQ14, g.GcExp))
		appendDecoderReferenceScalar(rows, frame, sub, "u_current_q10", int64(g.UCurrent))
		for i, value := range g.PastErrorsBefore {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "past_errors_before_q10", i, int64(value))
		}
		for i, value := range g.PastErrorsAfter {
			appendDecoderReferenceIndexedScalar(rows, frame, sub, "past_errors_after_q10", i, int64(value))
		}
	}
}

func decoderReferenceGainIndices(taps *Phase3DiagFrameTaps, sub int) (ga, gb int64) {
	if sub == 0 {
		return int64(taps.Frame.GA1), int64(taps.Frame.GB1)
	}
	return int64(taps.Frame.GA2), int64(taps.Frame.GB2)
}

func appendDecoderReferenceScalar(rows *[]stageRow, frame, sub int, field string, value int64) {
	appendDecoderReferenceIndexedScalar(rows, frame, sub, field, 0, value)
}

func appendDecoderReferenceIndexedScalar(rows *[]stageRow, frame, sub int, field string, index int, value int64) {
	*rows = append(*rows, stageRow{
		source:   "TAME",
		frame:    frame,
		sub:      sub,
		field:    field,
		index:    index,
		hasValue: true,
		value:    value,
	})
}

func appendDecoderReferenceArray(rows *[]stageRow, frame, sub int, field string, values []int16) {
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

func appendDecoderReferenceFrameArray(rows *[]stageRow, frame int, field string, values []int16) {
	for i, value := range values {
		*rows = append(*rows, stageRow{
			source:   "TAME",
			frame:    frame,
			sub:      -1,
			field:    field,
			index:    i,
			hasValue: true,
			value:    int64(value),
		})
	}
}

type decoderReferenceStageFieldStats struct {
	total      int
	exact      int
	mismatches int
	missing    int
	maxAbs     int64
}

func decoderReferenceStageStatsFor(stats map[string]*decoderReferenceStageFieldStats, field string) *decoderReferenceStageFieldStats {
	st := stats[field]
	if st == nil {
		st = &decoderReferenceStageFieldStats{}
		stats[field] = st
	}
	return st
}

func decoderReferenceStageFieldSummary(stats map[string]*decoderReferenceStageFieldStats) []string {
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
		return fields[i] < fields[j]
	})

	out := make([]string, 0, len(fields))
	for _, field := range fields {
		st := stats[field]
		out = append(out, fmt.Sprintf("field %-22s exact=%d/%d %.2f%% mismatches=%d missing=%d max_abs=%d",
			field, st.exact, st.total, percent(st.exact, st.total), st.mismatches, st.missing, st.maxAbs))
	}
	return out
}
