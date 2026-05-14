package decoder

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/pcm"
)

// TestDecoderITUVectorValidation is the decoder conformance gate we want to
// make authoritative before claiming stronger decoder credibility.
//
// It compares fixed ITU Annex A test bitstreams against their companion .PST
// PCM reference outputs at sample level:
//
//	ITU .BIT -> local decoder -> PCM
//	ITU .PST reference PCM
//
// PESQ/POLQA/MOS are deliberately not involved here. For decoder validation,
// the bitstream is fixed, so sample-level equality against the reference vector
// is a stronger and more direct signal than an objective listening score.
//
// Clean-room boundary: this test consumes only test-vector data files already
// present in testdata/itu. It does not inspect or execute any external G.729
// implementation source.
//
// Usage:
//
//	G729_DECODER_ITU_VECTOR_VALIDATION=1 \
//	go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
//
// Once the matrix reaches exact equality for the selected scope, promote it to
// a hard release gate with:
//
//	G729_DECODER_ITU_VECTOR_VALIDATION=1 \
//	G729_REQUIRE_DECODER_ITU_VECTOR_EXACT=1 \
//	go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
func TestDecoderITUVectorValidation(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_VECTOR_VALIDATION") != "1" {
		t.Skip("set G729_DECODER_ITU_VECTOR_VALIDATION=1 to run ITU vector sample-level decoder validation")
	}

	scope := strings.TrimSpace(os.Getenv("G729_DECODER_ITU_VECTOR_SCOPE"))
	if scope == "" {
		scope = "annexa-good"
	}
	requireExact := os.Getenv("G729_REQUIRE_DECODER_ITU_VECTOR_EXACT") == "1"

	t.Logf("decoder ITU vector validation: scope=%s requireExact=%t", scope, requireExact)
	t.Logf("%-10s %-16s %7s %7s %10s %10s %9s %9s %8s %11s",
		"vector", "scope", "frames", "bad", "exactFr", "exactSamp", "first", "maxAbs", "meanAbs", "rmsDelta")

	var selected int
	var total decoderITUVectorStats
	for _, tc := range decoderITUValidationCases() {
		if !decoderITUValidationCaseSelected(tc, scope) {
			continue
		}
		selected++
		stats := runDecoderITUVectorValidationCase(t, tc)
		total.add(stats)
		t.Logf("%-10s %-16s %7d %7d %10s %10s %9s %9d %8.2f %11.2f",
			tc.name, tc.scope, stats.frames, stats.badFrames,
			decoderITUPercent(stats.exactFrames, stats.frames),
			decoderITUPercent(stats.exactSamples, stats.samples),
			stats.firstDiffString(), stats.maxAbsDelta, stats.meanAbsDelta(), stats.rmsDelta())
		if requireExact && stats.diffSamples() != 0 {
			t.Errorf("%s: %d/%d samples differ from %s; first diff %s got=%d want=%d maxAbs=%d",
				tc.name, stats.diffSamples(), stats.samples, tc.pstFile,
				stats.firstDiffString(), stats.firstGot, stats.firstWant, stats.maxAbsDelta)
		}
	}
	if selected == 0 {
		t.Fatalf("no decoder validation vectors selected by scope %q", scope)
	}
	t.Logf("%-10s %-16s %7d %7d %10s %10s %9s %9d %8.2f %11.2f",
		"TOTAL", scope, total.frames, total.badFrames,
		decoderITUPercent(total.exactFrames, total.frames),
		decoderITUPercent(total.exactSamples, total.samples),
		total.firstDiffString(), total.maxAbsDelta, total.meanAbsDelta(), total.rmsDelta())
	if requireExact && total.diffSamples() != 0 {
		t.Fatalf("decoder ITU vector exact gate failed: %d/%d samples differ", total.diffSamples(), total.samples)
	}
}

// TestDecoderITUVectorFirstDiffTrace is an opt-in stage-localization aid for
// the vector gate above. It consumes only the same numeric .BIT/.PST vector
// artifacts and local decoder taps; no external implementation source or
// executable is involved.
//
// Usage:
//
//	G729_DECODER_ITU_VECTOR_TRACE=1 \
//	go test ./internal/decoder -run TestDecoderITUVectorFirstDiffTrace -count=1 -v
//
// Optional:
//
//	G729_DECODER_ITU_VECTOR_TRACE_VECTOR=ALGTHM
//	G729_DECODER_ITU_VECTOR_TRACE_MODE=first-diff   # or worst-frame / max-sample
func TestDecoderITUVectorFirstDiffTrace(t *testing.T) {
	if os.Getenv("G729_DECODER_ITU_VECTOR_TRACE") != "1" {
		t.Skip("set G729_DECODER_ITU_VECTOR_TRACE=1 to run ITU vector first-diff stage trace")
	}

	vector := strings.TrimSpace(os.Getenv("G729_DECODER_ITU_VECTOR_TRACE_VECTOR"))
	if vector == "" {
		vector = "ALGTHM"
	}
	mode := strings.TrimSpace(os.Getenv("G729_DECODER_ITU_VECTOR_TRACE_MODE"))
	if mode == "" {
		mode = "first-diff"
	}

	tc, ok := decoderITUValidationCaseByName(vector)
	if !ok {
		t.Fatalf("unknown decoder ITU vector %q", vector)
	}
	trace := runDecoderITUVectorTraceCase(t, tc, mode)
	if !trace.hasDiff {
		t.Logf("%s: no differences found", tc.name)
		return
	}

	logDecoderITUVectorTrace(t, tc, trace)
}

type decoderITUValidationCase struct {
	name    string
	bitFile string
	pstFile string
	scope   string
	note    string
}

func decoderITUValidationCases() []decoderITUValidationCase {
	return []decoderITUValidationCase{
		{name: "ALGTHM", bitFile: "ALGTHM.BIT", pstFile: "ALGTHM.PST", scope: "annexa-good", note: "algorithm coverage"},
		{name: "SPEECH", bitFile: "SPEECH.BIT", pstFile: "SPEECH.PST", scope: "annexa-good", note: "long natural-speech coverage"},
		{name: "FIXED", bitFile: "FIXED.BIT", pstFile: "FIXED.PST", scope: "annexa-good", note: "fixed-codebook coverage"},
		{name: "LSP", bitFile: "LSP.BIT", pstFile: "LSP.PST", scope: "annexa-good", note: "LSP coverage"},
		{name: "PITCH", bitFile: "PITCH.BIT", pstFile: "PITCH.PST", scope: "annexa-good", note: "pitch coverage"},
		{name: "TAME", bitFile: "TAME.BIT", pstFile: "TAME.PST", scope: "annexa-good", note: "taming coverage"},
		{name: "TEST", bitFile: "TEST.BIT", pstFile: "TEST.pst", scope: "annexa-good", note: "generic coverage"},
		{name: "OVERFLOW", bitFile: "OVERFLOW.BIT", pstFile: "OVERFLOW.PST", scope: "annexa-good", note: "overflow stress coverage"},
		{name: "ERASURE", bitFile: "ERASURE.BIT", pstFile: "ERASURE.PST", scope: "annexa-robustness", note: "bad-frame concealment coverage"},
		{name: "PARITY", bitFile: "PARITY.BIT", pstFile: "PARITY.PST", scope: "annexa-robustness", note: "pitch parity coverage"},
	}
}

func decoderITUValidationCaseByName(name string) (decoderITUValidationCase, bool) {
	for _, tc := range decoderITUValidationCases() {
		if strings.EqualFold(name, tc.name) || strings.EqualFold(name, tc.bitFile) {
			return tc, true
		}
	}
	return decoderITUValidationCase{}, false
}

func decoderITUValidationCaseSelected(tc decoderITUValidationCase, scope string) bool {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "", "annexa-good":
		return tc.scope == "annexa-good"
	case "annexa-robustness":
		return tc.scope == "annexa-robustness"
	case "all":
		return true
	default:
		return strings.EqualFold(scope, tc.name)
	}
}

type decoderITUVectorStats struct {
	frames       int
	samples      int
	badFrames    int
	exactFrames  int
	exactSamples int
	firstFrame   int
	firstSample  int
	firstGot     int16
	firstWant    int16
	maxAbsDelta  int
	sumAbsDelta  int64
	sumSqDelta   int64
}

func runDecoderITUVectorValidationCase(t *testing.T, tc decoderITUValidationCase) decoderITUVectorStats {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) != len(wantFrames) {
		t.Fatalf("%s: frame count mismatch: bit=%d pst=%d", tc.name, len(frames), len(wantFrames))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch: bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	stats := decoderITUVectorStats{firstFrame: -1, firstSample: -1}
	var d Decoder
	var out [frameSamples]int16
	for fi, packed := range frames {
		if bads[fi] {
			stats.badFrames++
		}
		if err := d.Decode(packed, bads[fi], out[:]); err != nil {
			t.Fatalf("%s frame %d Decode: %v", tc.name, fi, err)
		}
		stats.frames++
		stats.samples += frameSamples
		frameExact := true
		for si := 0; si < frameSamples; si++ {
			got := out[si]
			want := wantFrames[fi][si]
			if got == want {
				stats.exactSamples++
				continue
			}
			frameExact = false
			if stats.firstFrame < 0 {
				stats.firstFrame = fi
				stats.firstSample = si
				stats.firstGot = got
				stats.firstWant = want
			}
			delta := int(got) - int(want)
			if delta < 0 {
				delta = -delta
			}
			if delta > stats.maxAbsDelta {
				stats.maxAbsDelta = delta
			}
			delta64 := int64(delta)
			stats.sumAbsDelta += delta64
			stats.sumSqDelta += delta64 * delta64
		}
		if frameExact {
			stats.exactFrames++
		}
	}
	return stats
}

func (s *decoderITUVectorStats) add(other decoderITUVectorStats) {
	frameOffset := s.frames
	s.frames += other.frames
	s.samples += other.samples
	s.badFrames += other.badFrames
	s.exactFrames += other.exactFrames
	s.exactSamples += other.exactSamples
	if other.maxAbsDelta > s.maxAbsDelta {
		s.maxAbsDelta = other.maxAbsDelta
	}
	s.sumAbsDelta += other.sumAbsDelta
	s.sumSqDelta += other.sumSqDelta
	if s.firstFrame < 0 && other.firstFrame >= 0 {
		s.firstFrame = frameOffset + other.firstFrame
		s.firstSample = other.firstSample
		s.firstGot = other.firstGot
		s.firstWant = other.firstWant
	}
}

func (s decoderITUVectorStats) diffSamples() int {
	return s.samples - s.exactSamples
}

func (s decoderITUVectorStats) meanAbsDelta() float64 {
	if s.diffSamples() == 0 {
		return 0
	}
	return float64(s.sumAbsDelta) / float64(s.diffSamples())
}

func (s decoderITUVectorStats) rmsDelta() float64 {
	if s.diffSamples() == 0 {
		return 0
	}
	return math.Sqrt(float64(s.sumSqDelta) / float64(s.diffSamples()))
}

func (s decoderITUVectorStats) firstDiffString() string {
	if s.firstFrame < 0 {
		return "-"
	}
	return strconv.Itoa(s.firstFrame) + ":" + strconv.Itoa(s.firstSample)
}

func decoderITUPercent(num, den int) string {
	if den == 0 {
		return "-"
	}
	return strconv.FormatFloat(float64(num)*100/float64(den), 'f', 2, 64) + "%"
}

type decoderITUTraceResult struct {
	mode     string
	frame    int
	bad      bool
	hasDiff  bool
	stats    decoderITUFrameStats
	taps     Phase3DiagFrameTaps
	want     [frameSamples]int16
	vector   decoderITUValidationCase
	worstKey int64
}

type decoderITUFrameStats struct {
	exactSamples int
	firstSample  int
	firstGot     int16
	firstWant    int16
	maxAbsDelta  int
	sumAbsDelta  int64
	sumSqDelta   int64
}

func runDecoderITUVectorTraceCase(t *testing.T, tc decoderITUValidationCase, mode string) decoderITUTraceResult {
	t.Helper()
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) != len(wantFrames) {
		t.Fatalf("%s: frame count mismatch: bit=%d pst=%d", tc.name, len(frames), len(wantFrames))
	}
	if len(bads) != len(frames) {
		t.Fatalf("%s: bad flag count mismatch: bads=%d frames=%d", tc.name, len(bads), len(frames))
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", "first", "first-diff":
		mode = "first-diff"
	case "worst", "worst-frame":
		mode = "worst-frame"
	case "max", "max-sample":
		mode = "max-sample"
	default:
		t.Fatalf("unsupported G729_DECODER_ITU_VECTOR_TRACE_MODE=%q", mode)
	}

	var d Decoder
	best := decoderITUTraceResult{mode: mode, frame: -1, vector: tc}
	for fi, packed := range frames {
		taps, err := d.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("%s frame %d DecodeWithTaps: %v", tc.name, fi, err)
		}
		stats := decoderITUCompareFrame(&taps.Output, &wantFrames[fi])
		if stats.diffSamples() == 0 {
			continue
		}
		candidate := decoderITUTraceResult{
			mode:     mode,
			frame:    fi,
			bad:      bads[fi],
			hasDiff:  true,
			stats:    stats,
			taps:     taps,
			want:     wantFrames[fi],
			vector:   tc,
			worstKey: decoderITUTraceRank(mode, stats),
		}
		if mode == "first-diff" {
			return candidate
		}
		if !best.hasDiff || candidate.worstKey > best.worstKey {
			best = candidate
		}
	}
	return best
}

func decoderITUTraceRank(mode string, stats decoderITUFrameStats) int64 {
	switch mode {
	case "max-sample":
		return int64(stats.maxAbsDelta)
	default:
		return stats.sumSqDelta
	}
}

func logDecoderITUVectorTrace(t *testing.T, tc decoderITUValidationCase, trace decoderITUTraceResult) {
	t.Helper()
	stages := decoderITUTraceStages(trace.taps)
	start, end := decoderITUTraceWindow(trace.stats.firstSample, 16)

	t.Logf("%s trace mode=%s frame=%d bad=%t scope=%s note=%q",
		tc.name, trace.mode, trace.frame, trace.bad, tc.scope, tc.note)
	t.Logf("first diff sample=%d got=%d want=%d delta=%+d; frame exact=%s maxAbs=%d meanAbs=%.2f rms=%.2f",
		trace.stats.firstSample, trace.stats.firstGot, trace.stats.firstWant,
		int(trace.stats.firstGot)-int(trace.stats.firstWant),
		decoderITUPercent(trace.stats.exactSamples, frameSamples),
		trace.stats.maxAbsDelta, trace.stats.meanAbsDelta(), trace.stats.rmsDelta())
	t.Logf("indices: L0=%d L1=%d L2=%d L3=%d P1=%d P0=%d C1=%d S1=%d GA1=%d GB1=%d P2=%d C2=%d S2=%d GA2=%d GB2=%d",
		trace.taps.Frame.L0, trace.taps.Frame.L1, trace.taps.Frame.L2, trace.taps.Frame.L3,
		trace.taps.Frame.P1, trace.taps.Frame.P0, trace.taps.Frame.C1, trace.taps.Frame.S1,
		trace.taps.Frame.GA1, trace.taps.Frame.GB1, trace.taps.Frame.P2, trace.taps.Frame.C2,
		trace.taps.Frame.S2, trace.taps.Frame.GA2, trace.taps.Frame.GB2)

	t.Logf("stage comparison vs PST final domain:")
	for _, row := range []struct {
		name string
		data [frameSamples]int16
	}{
		{name: "synth_x2", data: stages.synthX2},
		{name: "postfilter_x2", data: stages.postfilterX2},
		{name: "hp_raw", data: stages.hpRaw},
		{name: "hp_x2", data: stages.hpX2},
		{name: "output", data: trace.taps.Output},
	} {
		stats := decoderITUCompareFrame(&row.data, &trace.want)
		t.Logf("  %-14s exact=%s near1=%s first=%s maxAbs=%d meanAbs=%.2f rms=%.2f",
			row.name,
			decoderITUPercent(stats.exactSamples, frameSamples),
			decoderITUPercent(decoderITUTraceNearCount(row.data[:], trace.want[:], 1), frameSamples),
			stats.firstDiffString(), stats.maxAbsDelta, stats.meanAbsDelta(), stats.rmsDelta())
	}

	pstHalf := decoderITUTracePSTHalf(trace.want)
	hpHalfStats := decoderITUCompareFrame(&stages.hpRaw, &pstHalf)
	t.Logf("stage comparison vs PST>>1 pre-scale domain:")
	t.Logf("  %-14s exact=%s near1=%s first=%s maxAbs=%d meanAbs=%.2f rms=%.2f",
		"hp_raw",
		decoderITUPercent(hpHalfStats.exactSamples, frameSamples),
		decoderITUPercent(decoderITUTraceNearCount(stages.hpRaw[:], pstHalf[:], 1), frameSamples),
		hpHalfStats.firstDiffString(), hpHalfStats.maxAbsDelta, hpHalfStats.meanAbsDelta(), hpHalfStats.rmsDelta())

	t.Logf("sample window [%d:%d):", start, end)
	t.Logf("  %-14s %s", "pst", decoderITUTraceSamples(trace.want[start:end]))
	t.Logf("  %-14s %s", "pst>>1", decoderITUTraceSamples(pstHalf[start:end]))
	t.Logf("  %-14s %s", "synth_x2", decoderITUTraceSamples(stages.synthX2[start:end]))
	t.Logf("  %-14s %s", "postfilter_x2", decoderITUTraceSamples(stages.postfilterX2[start:end]))
	t.Logf("  %-14s %s", "hp_raw", decoderITUTraceSamples(stages.hpRaw[start:end]))
	t.Logf("  %-14s %s", "hp_x2", decoderITUTraceSamples(stages.hpX2[start:end]))
	t.Logf("  %-14s %s", "output", decoderITUTraceSamples(trace.taps.Output[start:end]))
}

type decoderITUTraceFrameStages struct {
	synthX2      [frameSamples]int16
	postfilterX2 [frameSamples]int16
	hpRaw        [frameSamples]int16
	hpX2         [frameSamples]int16
}

func decoderITUTraceStages(taps Phase3DiagFrameTaps) decoderITUTraceFrameStages {
	var stages decoderITUTraceFrameStages
	for sf := 0; sf < 2; sf++ {
		off := sf * subframeLen
		sub := taps.Sub[sf]
		pcm.ScaleUpSat(sub.S[:], stages.synthX2[off:off+subframeLen])
		pcm.ScaleUpSat(sub.SPf[:], stages.postfilterX2[off:off+subframeLen])
		copy(stages.hpRaw[off:off+subframeLen], sub.HpOut[:])
		pcm.ScaleUpSat(sub.HpOut[:], stages.hpX2[off:off+subframeLen])
	}
	return stages
}

func decoderITUCompareFrame(got, want *[frameSamples]int16) decoderITUFrameStats {
	stats := decoderITUFrameStats{firstSample: -1}
	for i := 0; i < frameSamples; i++ {
		if got[i] == want[i] {
			stats.exactSamples++
			continue
		}
		if stats.firstSample < 0 {
			stats.firstSample = i
			stats.firstGot = got[i]
			stats.firstWant = want[i]
		}
		delta := int(got[i]) - int(want[i])
		if delta < 0 {
			delta = -delta
		}
		if delta > stats.maxAbsDelta {
			stats.maxAbsDelta = delta
		}
		delta64 := int64(delta)
		stats.sumAbsDelta += delta64
		stats.sumSqDelta += delta64 * delta64
	}
	return stats
}

func (s decoderITUFrameStats) diffSamples() int {
	return frameSamples - s.exactSamples
}

func (s decoderITUFrameStats) meanAbsDelta() float64 {
	if s.diffSamples() == 0 {
		return 0
	}
	return float64(s.sumAbsDelta) / float64(s.diffSamples())
}

func (s decoderITUFrameStats) rmsDelta() float64 {
	if s.diffSamples() == 0 {
		return 0
	}
	return math.Sqrt(float64(s.sumSqDelta) / float64(s.diffSamples()))
}

func (s decoderITUFrameStats) firstDiffString() string {
	if s.firstSample < 0 {
		return "-"
	}
	return strconv.Itoa(s.firstSample)
}

func decoderITUTracePSTHalf(want [frameSamples]int16) [frameSamples]int16 {
	var out [frameSamples]int16
	for i := 0; i < frameSamples; i++ {
		out[i] = int16(int32(want[i]) >> 1)
	}
	return out
}

func decoderITUTraceNearCount(got, want []int16, threshold int) int {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	count := 0
	for i := 0; i < n; i++ {
		delta := int(got[i]) - int(want[i])
		if delta < 0 {
			delta = -delta
		}
		if delta <= threshold {
			count++
		}
	}
	return count
}

func decoderITUTraceWindow(center, radius int) (int, int) {
	if center < 0 {
		center = 0
	}
	start := center - radius
	if start < 0 {
		start = 0
	}
	end := center + radius + 1
	if end > frameSamples {
		end = frameSamples
	}
	return start, end
}

func decoderITUTraceSamples(samples []int16) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, sample := range samples {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Itoa(int(sample)))
	}
	b.WriteByte(']')
	return b.String()
}
