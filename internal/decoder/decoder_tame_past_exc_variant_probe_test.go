package decoder

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
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

func TestDecoderTAMEPitchGainCapTriggerSearch(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_PITCH_CAP_TRIGGER_SEARCH") != "1" {
		t.Skip("set G729_DECODER_TAME_PITCH_CAP_TRIGGER_SEARCH=1 to run TAME pitch-gain cap trigger search")
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

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		t.Fatal(errUnknownTAMEVector())
	}
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)
	frames, _ := readG192Frames(t, bitPath)
	want := readPSTFrames(t, pstPath)

	productionGot, err := collectDecoderTAMEACBCheckpointRows(t, expected)
	if err != nil {
		t.Fatalf("collect production TAME past-excitation rows: %v", err)
	}
	prodStats := decoderTAMEPastExcCompareStats(expected, productionGot)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}
	prodOut := phase3eDecodeVariant(t, bitData, len(frames), phase3eVariant{name: "production"})
	prodSumSq := decoderGainCandidateOutputSumSq(prodOut, want)

	rows := make([]decoderTAMEPitchCapTriggerSearchRow, 0)
	for _, trigger := range decoderTAMEPitchCapTriggerGrid() {
		got, out, applied, err := collectDecoderTAMEPastExcRowsWithPitchCapTrigger(t, expected, frames, trigger)
		if err != nil {
			t.Fatalf("collect trigger %s: %v", trigger.name(), err)
		}
		stats := decoderTAMEPastExcCompareStats(expected, got)
		sumSq := decoderGainCandidateOutputSumSq(out, want)
		rows = append(rows, decoderTAMEPitchCapTriggerSearchRow{
			trigger: trigger,
			applied: applied,
			stats:   stats,
			sumSq:   sumSq,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].stats.sumErrSq != rows[j].stats.sumErrSq {
			return rows[i].stats.sumErrSq < rows[j].stats.sumErrSq
		}
		if rows[i].sumSq != rows[j].sumSq {
			return rows[i].sumSq < rows[j].sumSq
		}
		return rows[i].trigger.name() < rows[j].trigger.name()
	})

	topN := decoderITUFrontierTopN()
	if topN > len(rows) {
		topN = len(rows)
	}
	t.Logf("decoder TAME pitch-gain cap trigger search: path=%s filled=%d candidates=%d",
		expectedPath, len(expected), len(rows))
	decoderTAMELogPastExcProbeStats(t, "production", prodStats)
	t.Logf("production pstRMS=%.2f", decoderGainCandidateRMS(prodSumSq, len(frames)*frameSamples))
	t.Logf("%-42s %8s %8s %8s %8s %8s %8s %10s",
		"trigger", "applied", "errRMS", "meanAbs", "maxAbs", "corr", "scale", "pstRMS")
	for _, row := range rows[:topN] {
		t.Logf("%-42s %8d %8.2f %8.2f %8d %8.4f %8.4f %10.2f",
			row.trigger.name(),
			row.applied,
			row.stats.errRMS(),
			row.stats.meanAbs(),
			row.stats.maxAbsDiff,
			row.stats.corr(),
			row.stats.scale(),
			decoderGainCandidateRMS(row.sumSq, len(frames)*frameSamples))
	}
}

func TestDecoderPitchGainCapTriggerCrossVectorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_PITCH_CAP_TRIGGER_CROSS_VECTOR") != "1" {
		t.Skip("set G729_DECODER_PITCH_CAP_TRIGGER_CROSS_VECTOR=1 to run pitch cap trigger cross-vector audit")
	}

	trigger := decoderTAMEDiagnosticPitchCapTrigger()
	t.Logf("decoder pitch-gain cap trigger cross-vector audit: trigger=%s", trigger.name())
	t.Logf("%-8s %8s %10s %10s %9s %9s %8s %8s",
		"vector", "applied", "prodRMS", "candRMS", "prodSNR", "candSNR", "prodCor", "candCor")
	for _, tc := range decoderITUValidationCases() {
		if !decoderITUValidationCaseSelected(tc, "annexa-good") {
			continue
		}
		bitPath := vectorPath(tc.bitFile)
		pstPath := vectorPath(tc.pstFile)
		ensureTestdataPresent(t, bitPath, pstPath)
		frames, _ := readG192Frames(t, bitPath)
		want := readPSTFrames(t, pstPath)
		n := len(frames)
		if len(want) < n {
			n = len(want)
		}
		if n == 0 {
			t.Fatalf("%s reconciled to zero frames", tc.name)
		}
		frames = frames[:n]
		want = want[:n]

		bitData, err := os.ReadFile(bitPath)
		if err != nil {
			t.Fatalf("read %s: %v", bitPath, err)
		}
		prodOut := phase3eDecodeVariant(t, bitData, n, phase3eVariant{name: "production"})
		_, candOut, applied, err := collectDecoderTAMEPastExcRowsWithPitchCapTrigger(t, nil, frames, trigger)
		if err != nil {
			t.Fatalf("%s trigger decode: %v", tc.name, err)
		}

		ref := decoderTAMEFlattenPST(want)
		prodM := blackboxMeasure(ref, prodOut, 40)
		candM := blackboxMeasure(ref, candOut, 40)
		t.Logf("%-8s %8d %10.2f %10.2f %9.2f %9.2f %8.4f %8.4f",
			tc.name,
			applied,
			decoderGainCandidateRMS(decoderGainCandidateOutputSumSq(prodOut, want), n*frameSamples),
			decoderGainCandidateRMS(decoderGainCandidateOutputSumSq(candOut, want), n*frameSamples),
			prodM.globalSNR,
			candM.globalSNR,
			prodM.corr,
			candM.corr)
	}
}

type decoderTAMEPitchCapTriggerSearchRow struct {
	trigger decoderTAMEPitchCapTrigger
	applied int
	stats   decoderPastExcAgeGroup
	sumSq   int64
}

type decoderTAMEPitchCapTrigger struct {
	minPastRMS  float64
	minTailRMS  float64
	minVRMS     float64
	minPitchRMS float64
	maxFixedRMS float64
	minGpQ14    int16
	capGpQ14    int16
}

type decoderTAMEPitchCapTriggerFeature struct {
	gpQ14    int16
	pastRMS  float64
	tailRMS  float64
	vRMS     float64
	pitchRMS float64
	fixedRMS float64
}

func (tr decoderTAMEPitchCapTrigger) match(f decoderTAMEPitchCapTriggerFeature) bool {
	minGp := tr.minGpQ14
	if minGp == 0 {
		minGp = tr.cap()
	}
	if f.gpQ14 <= minGp {
		return false
	}
	if tr.minPastRMS > 0 && f.pastRMS < tr.minPastRMS {
		return false
	}
	if tr.minTailRMS > 0 && f.tailRMS < tr.minTailRMS {
		return false
	}
	if tr.minVRMS > 0 && f.vRMS < tr.minVRMS {
		return false
	}
	if tr.minPitchRMS > 0 && f.pitchRMS < tr.minPitchRMS {
		return false
	}
	if tr.maxFixedRMS > 0 && f.fixedRMS > tr.maxFixedRMS {
		return false
	}
	return true
}

func (tr decoderTAMEPitchCapTrigger) cap() int16 {
	if tr.capGpQ14 != 0 {
		return tr.capGpQ14
	}
	return 15565
}

func (tr decoderTAMEPitchCapTrigger) name() string {
	name := "gp>cap"
	if tr.minPastRMS > 0 {
		name += fmt.Sprintf("+past>=%.0f", tr.minPastRMS)
	}
	if tr.minTailRMS > 0 {
		name += fmt.Sprintf("+tail>=%.0f", tr.minTailRMS)
	}
	if tr.minVRMS > 0 {
		name += fmt.Sprintf("+v>=%.0f", tr.minVRMS)
	}
	if tr.minPitchRMS > 0 {
		name += fmt.Sprintf("+pitch>=%.0f", tr.minPitchRMS)
	}
	if tr.maxFixedRMS > 0 {
		name += fmt.Sprintf("+fixed<=%.0f", tr.maxFixedRMS)
	}
	return name
}

func decoderTAMEPitchCapTriggerGrid() []decoderTAMEPitchCapTrigger {
	base := decoderTAMEPitchCapTrigger{capGpQ14: 15565}
	triggers := []decoderTAMEPitchCapTrigger{base}
	minThresholds := []float64{220, 240, 260, 280, 300, 320, 340}
	fixedCeilings := []float64{40, 60, 80}

	for _, threshold := range minThresholds {
		tr := base
		tr.minPastRMS = threshold
		triggers = append(triggers, tr)

		tr = base
		tr.minTailRMS = threshold
		triggers = append(triggers, tr)

		tr = base
		tr.minVRMS = threshold
		triggers = append(triggers, tr)

		tr = base
		tr.minPitchRMS = threshold
		triggers = append(triggers, tr)
	}
	for _, ceiling := range fixedCeilings {
		tr := base
		tr.maxFixedRMS = ceiling
		triggers = append(triggers, tr)
	}
	for _, threshold := range minThresholds {
		for _, ceiling := range fixedCeilings {
			tr := base
			tr.minPastRMS = threshold
			tr.maxFixedRMS = ceiling
			triggers = append(triggers, tr)

			tr = base
			tr.minVRMS = threshold
			tr.maxFixedRMS = ceiling
			triggers = append(triggers, tr)

			tr = base
			tr.minPitchRMS = threshold
			tr.maxFixedRMS = ceiling
			triggers = append(triggers, tr)
		}
	}
	return triggers
}

func decoderTAMEDiagnosticPitchCapTrigger() decoderTAMEPitchCapTrigger {
	// Diagnostic only: the cross-vector audit documents why this trigger is
	// not production-safe even though it improves the late TAME checkpoint.
	return decoderTAMEPitchCapTrigger{
		capGpQ14:   15565,
		minPastRMS: 240,
	}
}

func decoderTAMEFlattenPST(frames [][frameSamples]int16) []int16 {
	out := make([]int16, 0, len(frames)*frameSamples)
	for frame := range frames {
		out = append(out, frames[frame][:]...)
	}
	return out
}

func collectDecoderTAMEPastExcRowsWithPitchCapTrigger(
	t testing.TB,
	expected []stageRow,
	frames [][]byte,
	trigger decoderTAMEPitchCapTrigger,
) ([]stageRow, []int16, int, error) {
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

	out := make([]int16, len(frames)*frameSamples)
	var dec Decoder
	var rows []stageRow
	var applied int
	for frame := range frames {
		if err := dec.decodeFrameTAMEPitchCapTrigger(
			frame,
			frames[frame],
			out[frame*frameSamples:(frame+1)*frameSamples],
			trigger,
			targets[frame],
			&rows,
			&applied,
		); err != nil {
			return nil, nil, 0, err
		}
	}
	return rows, out, applied, nil
}

func (d *Decoder) decodeFrameTAMEPitchCapTrigger(
	frame int,
	packed []byte,
	out []int16,
	trigger decoderTAMEPitchCapTrigger,
	targetSubs map[int]struct{},
	rows *[]stageRow,
	applied *int,
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
	d.decodeSubframeTAMEPitchCapTrigger(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], trigger, applied)

	if _, ok := targetSubs[1]; ok {
		appendDecoderTAMEPastExcRows(rows, frame, 1, d.pastExc[:])
	}
	d.decodeSubframeTAMEPitchCapTrigger(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], trigger, applied)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframeTAMEPitchCapTrigger(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	trigger decoderTAMEPitchCapTrigger,
	applied *int,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, &c)
	gpQ14 := gainTaps.GpQ14Final
	gcMantQ14 := gainTaps.GcMantQ14
	gcExp := gainTaps.GcExp

	var zero [subframeLen]int16
	var pitchOnly [subframeLen]int16
	var fixedOnly [subframeLen]int16
	synth.BuildExcitation(gpQ14, 0, 0, &v, &zero, &pitchOnly)
	synth.BuildExcitation(0, gcMantQ14, gcExp, &zero, &c, &fixedOnly)
	feature := decoderTAMEPitchCapTriggerFeature{
		gpQ14:    gpQ14,
		pastRMS:  decoderHistoryRMS(d.pastExc[:]),
		tailRMS:  decoderHistoryRMS(d.pastExc[pastExcLen-subframeLen:]),
		vRMS:     envelopeRMS(v[:]),
		pitchRMS: envelopeRMS(pitchOnly[:]),
		fixedRMS: envelopeRMS(fixedOnly[:]),
	}
	if trigger.match(feature) {
		gpQ14 = trigger.cap()
		(*applied)++
	}

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.rememberPitchGain(gainTaps.GpQ14Final)
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
