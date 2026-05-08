package decoder

import (
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/synth"
)

const (
	speechPath           = "/home/exedev/g729/testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.BIT"
	asteriskPath         = "/home/exedev/g729/testdata/external/asterisk_payload.g729"
	decoderGotOut        = "/home/exedev/g729/testdata/oracle/handoff/decoder_stage_got.csv"
	decoderGotSummaryOut = "/home/exedev/g729/testdata/oracle/handoff/decoder_stage_got_summary.csv"
)

type cell struct {
	sub      int
	field    string
	index    int
	hasValue bool
	value    int64
}

type stageRow struct {
	source   string
	frame    int
	sub      int
	field    string
	index    int
	hasValue bool
	value    int64
}

type summaryState struct {
	totalFrames      int64
	speechFrames     int64
	sidFrames        int64
	malformedFrames  int64
	unavailableField int64
	sumSq            float64
	samples          int64
	peak             int
	clipped          int64
}

type speechBitFrame struct {
	data []byte
	sid  bool
}

type frameDump struct {
	frame       int
	cells       []cell
	unavail     int
	sampleSum   float64
	sampleN     int64
	peak        int
	clipped     int64
	malformed   bool
	sidFrame    bool
	voicedScore int64
	voicedGood  bool
}

func TestDecoderStageGotDump(t *testing.T) {
	if os.Getenv("G729_DUMP_DECODER_STAGE_GOT") != "1" {
		t.Skip("set G729_DUMP_DECODER_STAGE_GOT=1 to write decoder_stage_got.csv")
	}

	speechTargets := make(map[int]struct{}, 30)
	for i := 0; i <= 19; i++ {
		speechTargets[i] = struct{}{}
	}
	for i := 100; i <= 104; i++ {
		speechTargets[i] = struct{}{}
	}
	for i := 1122; i <= 1126; i++ {
		speechTargets[i] = struct{}{}
	}
	asteriskTargets := make(map[int]struct{}, 30)
	for i := 0; i < 30; i++ {
		asteriskTargets[i] = struct{}{}
	}

	maxSpeech := maxIntKey(speechTargets)
	maxAsterisk := maxIntKey(asteriskTargets)

	speechFrames, err := readSpeechFrames(speechPath)
	if err != nil {
		t.Fatalf("read speech frames: %v", err)
	}
	asteriskFrames, err := readAsteriskFrames(asteriskPath)
	if err != nil {
		t.Fatalf("read asterisk frames: %v", err)
	}

	summaryBySource := map[string]*summaryState{
		"SPEECH":   {},
		"ASTERISK": {},
	}
	records := make([]stageRow, 0, (len(speechTargets)+len(asteriskTargets))*822)

	speechArtifacts := collectSpeechArtifacts(speechFrames, speechTargets, maxSpeech, summaryBySource["SPEECH"])
	for _, frameIdx := range sortedKeys(speechTargets) {
		if frameIdx > maxSpeech {
			continue
		}
		art, ok := speechArtifacts[frameIdx]
		if !ok {
			continue
		}
		appendArtifactRows(&records, "SPEECH", frameIdx, art.cells)
	}

	asteriskArtifacts := collectAsteriskArtifacts(asteriskFrames, asteriskTargets, maxAsterisk, summaryBySource["ASTERISK"])
	for i := 0; i <= maxAsterisk; i++ {
		art, ok := asteriskArtifacts[i]
		if !ok {
			continue
		}
		appendArtifactRows(&records, "ASTERISK", i, art.cells)
	}

	voicedIdx := pickVoicedAsteriskFrames(asteriskArtifacts, 10)
	if len(voicedIdx) > 0 {
		voicedSummary := &summaryState{}
		for _, fi := range voicedIdx {
			art := asteriskArtifacts[fi]
			appendArtifactRows(&records, "ASTERISK_VOICED", fi, art.cells)
			aggregateSummaryFromArtifact(voicedSummary, &art)
			voicedSummary.totalFrames++
			voicedSummary.speechFrames++
		}
		summaryBySource["ASTERISK_VOICED"] = voicedSummary
	}

	if err := writeStageCSV(decoderGotOut, "got", records); err != nil {
		t.Fatalf("write got: %v", err)
	}
	if err := writeSummaryCSV(decoderGotSummaryOut, summaryBySource); err != nil {
		t.Fatalf("write summary: %v", err)
	}
}

func readSpeechFrames(path string) ([]speechBitFrame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var frames []speechBitFrame
	header := make([]byte, 4)
	for {
		_, err := io.ReadFull(f, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		sync := binary.LittleEndian.Uint16(header[:2])
		if sync != bitstream.G192SyncGood && sync != bitstream.G192SyncBad {
			return nil, fmt.Errorf("invalid g192 sync 0x%04x at frame %d", sync, len(frames))
		}
		lengthWords := int(binary.LittleEndian.Uint16(header[2:]))
		if lengthWords <= 0 {
			return nil, fmt.Errorf("invalid g192 payload length %d at frame %d", lengthWords, len(frames))
		}

		payload := make([]byte, 2*lengthWords)
		if _, err := io.ReadFull(f, payload); err != nil {
			return nil, err
		}
		rec := speechBitFrame{
			sid: lengthWords != bitstream.FrameBits,
		}
		if lengthWords == bitstream.FrameBits {
			rec.data = make([]byte, bitstream.FrameBytes)
			for bi := 0; bi < bitstream.FrameBits; bi++ {
				word := binary.LittleEndian.Uint16(payload[2*bi : 2*bi+2])
				switch word {
				case bitstream.G192Bit1:
					byteIdx := bi >> 3
					bitIdx := 7 - (bi & 7)
					rec.data[byteIdx] |= 1 << uint(bitIdx)
				case bitstream.G192Bit0, 0x0000:
				default:
					return nil, fmt.Errorf("invalid softbit 0x%04x at frame %d bit %d", word, len(frames), bi)
				}
			}
		}
		frames = append(frames, rec)
	}
	return frames, nil
}

func readAsteriskFrames(path string) ([][]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		return nil, fmt.Errorf("asterisk payload length %d not divisible by frame size %d", len(raw), bitstream.FrameBytes)
	}
	count := len(raw) / bitstream.FrameBytes
	frames := make([][]byte, count)
	for i := 0; i < count; i++ {
		start := i * bitstream.FrameBytes
		frame := make([]byte, bitstream.FrameBytes)
		copy(frame, raw[start:start+bitstream.FrameBytes])
		frames[i] = frame
	}
	return frames, nil
}

func collectSpeechArtifacts(frames []speechBitFrame, targets map[int]struct{}, maxFrame int, st *summaryState) map[int]frameDump {
	out := make(map[int]frameDump, len(targets))
	var dec Decoder

	for fi := 0; fi <= maxFrame; fi++ {
		_, want := targets[fi]
		if fi >= len(frames) {
			if want {
				st.totalFrames++
				st.malformedFrames++
				art := frameDump{frame: fi, sidFrame: true}
				art.cells = missingFrameCells()
				art.unavail = len(art.cells)
				st.unavailableField += int64(art.unavail)
				out[fi] = art
			}
			continue
		}

		rec := frames[fi]
		if rec.sid {
			if want {
				st.totalFrames++
				st.sidFrames++
				art := frameDump{frame: fi, sidFrame: true}
				art.cells = missingFrameCells()
				art.unavail = len(art.cells)
				st.unavailableField += int64(art.unavail)
				out[fi] = art
			}
			continue
		}

		taps, err := dec.DecodeWithTaps(rec.data)
		if err != nil {
			if want {
				st.totalFrames++
				st.malformedFrames++
				art := frameDump{frame: fi, malformed: true}
				art.cells = missingFrameCells()
				art.unavail = len(art.cells)
				st.unavailableField += int64(art.unavail)
				out[fi] = art
			}
			continue
		}

		if !want {
			continue
		}

		st.totalFrames++
		st.speechFrames++
		art, err := collectFrameFromDecoded(fi, rec.data, &taps)
		if err != nil {
			st.malformedFrames++
			art.cells = missingFrameCells()
			art.unavail = len(art.cells)
		}
		if err != nil {
			st.unavailableField += int64(art.unavail)
			out[fi] = art
			continue
		}

		st.unavailableField += int64(art.unavail)
		st.samples += art.sampleN
		st.sumSq += art.sampleSum
		st.clipped += art.clipped
		if art.peak > st.peak {
			st.peak = art.peak
		}
		out[fi] = art
	}
	return out
}

func collectAsteriskArtifacts(frames [][]byte, targets map[int]struct{}, maxFrame int, st *summaryState) map[int]frameDump {
	out := make(map[int]frameDump, len(targets))
	var dec Decoder

	maxFrame = minInt(maxFrame, len(frames)-1)
	for fi := 0; fi <= maxFrame; fi++ {
		_, want := targets[fi]
		taps, err := dec.DecodeWithTaps(frames[fi])
		if err != nil {
			if want {
				st.totalFrames++
				st.malformedFrames++
				art := frameDump{frame: fi, malformed: true}
				art.cells = missingFrameCells()
				art.unavail = len(art.cells)
				st.unavailableField += int64(art.unavail)
				out[fi] = art
			}
			continue
		}
		if !want {
			continue
		}
		st.totalFrames++
		st.speechFrames++
		art, err := collectFrameFromDecoded(fi, frames[fi], &taps)
		if err != nil {
			st.malformedFrames++
			art.cells = missingFrameCells()
			art.unavail = len(art.cells)
			st.unavailableField += int64(art.unavail)
			out[fi] = art
			continue
		}

		st.unavailableField += int64(art.unavail)
		st.samples += art.sampleN
		st.sumSq += art.sampleSum
		st.clipped += art.clipped
		if art.peak > st.peak {
			st.peak = art.peak
		}
		out[fi] = art
	}
	return out
}

func collectFrameFromDecoded(frameIdx int, packed []byte, taps *Phase3DiagFrameTaps) (frameDump, error) {
	var out frameDump
	out.frame = frameIdx
	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		return out, err
	}

	out.cells = append(out.cells, makeFrameLevelCells(int(f.L0), int(f.L1), int(f.L2), int(f.L3), int(f.P1), int(f.P0), int(f.C1), int(f.S1), int(f.GA1), int(f.GB1), int(f.P2), int(f.C2), int(f.S2), int(f.GA2), int(f.GB2))...)

	for sf := 0; sf < 2; sf++ {
		appendLspAndLSF(sf, taps.Sub[sf].A, &out)
	}

	for sf := 0; sf < 2; sf++ {
		sub := &taps.Sub[sf]
		appendCell(&out, sf, "pitch_t_int", -1, true, int64(sub.TInt))
		appendCell(&out, sf, "pitch_t_frac", -1, true, int64(sub.TFrac))
		appendCell(&out, sf, "adaptive_gain_q14", -1, true, int64(sub.GpQ14))

		gainQ14 := gainQ14FromMantExp(sub.GainTaps.GcMantQ14, sub.GainTaps.GcExp)
		appendCell(&out, sf, "fixed_gain_q14", -1, true, gainQ14)
		appendCell(&out, sf, "fixed_gain_x1e6", -1, true, q14ToX1e6(gainQ14))

		var zero [40]int16
		var pitchContrib [40]int16
		var fixedContrib [40]int16
		synth.BuildExcitation(sub.GpQ14, 0, 0, &sub.V, &zero, &pitchContrib)
		synth.BuildExcitation(0, sub.GainTaps.GcMantQ14, sub.GainTaps.GcExp, &zero, &sub.C, &fixedContrib)

		appendArray(&out, sf, "adaptive_v_q0", sub.V[:])
		appendArray(&out, sf, "fixed_c_q13", sub.C[:])
		appendArray(&out, sf, "pitch_contrib_q0", pitchContrib[:])
		appendArray(&out, sf, "fixed_contrib_q0", fixedContrib[:])
		appendArray(&out, sf, "excitation_u_q0", sub.U[:])
		appendArray(&out, sf, "synth_s_q0", sub.S[:])
		appendArray(&out, sf, "postfilter_s_q0", sub.SPf[:])
		appendArray(&out, sf, "hp_q0", sub.HpOut[:])

		off := sf * 40
		pcmRow := make([]int16, 40)
		copy(pcmRow, taps.Output[off:off+40])
		appendArrayFromInt16(&out, sf, "pcm_q0", pcmRow)
		collectSampleStatsFromPCM(&out, pcmRow)
	}
	out.voicedScore = computeVoicedScore(taps)
	out.voicedGood = isVoicedFrameCandidate(taps)
	return out, nil
}

func appendLspAndLSF(sf int, a [11]int16, art *frameDump) {
	var lspQ [10]int16
	if err := lsp.LPToLSP(&a, &lspQ); err != nil {
		for i := 0; i < 10; i++ {
			appendCell(art, sf, "lsf_q13", i, false, 0)
			appendCell(art, sf, "lsp_q15", i, false, 0)
		}
	} else {
		var lsf [10]int16
		lsp.LSPToLSF(&lspQ, &lsf)
		for i := 0; i < 10; i++ {
			appendCell(art, sf, "lsf_q13", i, true, int64(lsf[i]))
			appendCell(art, sf, "lsp_q15", i, true, int64(lspQ[i]))
		}
	}
	for i := 0; i <= 10; i++ {
		appendCell(art, sf, "lp_a_q12", i, true, int64(a[i]))
	}
}

func appendCell(art *frameDump, sub int, field string, index int, hasValue bool, value int64) {
	art.cells = append(art.cells, cell{sub: sub, field: field, index: index, hasValue: hasValue, value: value})
	if !hasValue {
		art.unavail++
	}
}

func appendArray(art *frameDump, sub int, field string, src []int16) {
	for i, v := range src {
		appendCell(art, sub, field, i, true, int64(v))
	}
}

func appendArrayFromInt16(art *frameDump, sub int, field string, src []int16) {
	for i, v := range src {
		appendCell(art, sub, field, i, true, int64(v))
	}
}

func collectSampleStatsFromPCM(art *frameDump, pcmSamples []int16) {
	for _, s := range pcmSamples {
		v := int(s)
		art.sampleN++
		art.sampleSum += float64(v) * float64(v)
		if v == 32767 || v == -32768 {
			art.clipped++
		}
		abs := v
		if abs < 0 {
			abs = -abs
		}
		if abs > art.peak {
			art.peak = abs
		}
	}
}

func makeFrameLevelCells(l0, l1, l2, l3, p1, p0, c1, s1, ga1, gb1, p2, c2, s2, ga2, gb2 int) []cell {
	out := make([]cell, 0, 15)
	appendFrameCell := func(field string, value int64) {
		out = append(out, cell{sub: -1, field: field, index: -1, hasValue: true, value: value})
	}
	appendFrameCell("L0", int64(l0))
	appendFrameCell("L1", int64(l1))
	appendFrameCell("L2", int64(l2))
	appendFrameCell("L3", int64(l3))
	appendFrameCell("P1", int64(p1))
	appendFrameCell("P0", int64(p0))
	appendFrameCell("C1", int64(c1))
	appendFrameCell("S1", int64(s1))
	appendFrameCell("GA1", int64(ga1))
	appendFrameCell("GB1", int64(gb1))
	appendFrameCell("P2", int64(p2))
	appendFrameCell("C2", int64(c2))
	appendFrameCell("S2", int64(s2))
	appendFrameCell("GA2", int64(ga2))
	appendFrameCell("GB2", int64(gb2))
	return out
}

func missingFrameCells() []cell {
	cells := make([]cell, 0, 822)
	cells = append(cells, makeFrameLevelCells(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)...)
	for sf := 0; sf < 2; sf++ {
		cells = appendCellNoValue(cells, sf, "pitch_t_int", -1)
		cells = appendCellNoValue(cells, sf, "pitch_t_frac", -1)
		cells = appendCellNoValue(cells, sf, "adaptive_gain_q14", -1)
		cells = appendCellNoValue(cells, sf, "fixed_gain_q14", -1)
		cells = appendCellNoValue(cells, sf, "fixed_gain_x1e6", -1)

		for i := 0; i < 10; i++ {
			cells = appendCellNoValue(cells, sf, "lsf_q13", i)
			cells = appendCellNoValue(cells, sf, "lsp_q15", i)
		}
		for i := 0; i <= 10; i++ {
			cells = appendCellNoValue(cells, sf, "lp_a_q12", i)
		}
		for i := 0; i < 40; i++ {
			cells = appendCellNoValue(cells, sf, "adaptive_v_q0", i)
			cells = appendCellNoValue(cells, sf, "fixed_c_q13", i)
			cells = appendCellNoValue(cells, sf, "pitch_contrib_q0", i)
			cells = appendCellNoValue(cells, sf, "fixed_contrib_q0", i)
			cells = appendCellNoValue(cells, sf, "excitation_u_q0", i)
			cells = appendCellNoValue(cells, sf, "synth_s_q0", i)
			cells = appendCellNoValue(cells, sf, "postfilter_s_q0", i)
			cells = appendCellNoValue(cells, sf, "hp_q0", i)
			cells = appendCellNoValue(cells, sf, "pcm_q0", i)
		}
	}
	return cells
}

func appendCellNoValue(dst []cell, sub int, field string, idx int) []cell {
	return append(dst, cell{sub: sub, field: field, index: idx, hasValue: false})
}

func appendArtifactRows(out *[]stageRow, source string, frame int, cells []cell) {
	for _, c := range cells {
		*out = append(*out, stageRow{
			source:   source,
			frame:    frame,
			sub:      c.sub,
			field:    c.field,
			index:    c.index,
			hasValue: c.hasValue,
			value:    c.value,
		})
	}
}

func aggregateSummaryFromArtifact(st *summaryState, art *frameDump) {
	st.unavailableField += int64(art.unavail)
	st.sumSq += art.sampleSum
	st.samples += art.sampleN
	st.clipped += art.clipped
	if art.peak > st.peak {
		st.peak = art.peak
	}
}

func pickVoicedAsteriskFrames(arts map[int]frameDump, want int) []int {
	type scored struct {
		frame int
		score int64
		good  bool
	}
	all := make([]scored, 0, len(arts))
	for f, a := range arts {
		all = append(all, scored{frame: f, score: a.voicedScore, good: a.voicedGood})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].good != all[j].good {
			return all[i].good
		}
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].frame < all[j].frame
	})
	cands := make([]int, 0, want)
	if len(all) == 0 {
		return cands
	}
	for _, s := range all {
		if len(cands) >= want {
			break
		}
		cands = append(cands, s.frame)
	}
	sort.Ints(cands)
	return cands
}

func computeVoicedScore(taps *Phase3DiagFrameTaps) int64 {
	var score int64
	for sf := 0; sf < 2; sf++ {
		sub := &taps.Sub[sf]
		if sub.TInt >= 20 && sub.TInt <= 143 {
			if sub.GpQ14 > 0 {
				score += int64(sub.GpQ14)
			}
		}
	}
	for n := 0; n < 80; n++ {
		v := int64(taps.Output[n])
		if v < 0 {
			v = -v
		}
		score += v / 512
	}
	return score
}

func isVoicedFrameCandidate(taps *Phase3DiagFrameTaps) bool {
	for sf := 0; sf < 2; sf++ {
		sub := &taps.Sub[sf]
		if sub.TInt < 20 || sub.TInt > 143 {
			return false
		}
		if sub.GpQ14 <= 0 {
			return false
		}
	}
	return true
}

func gainQ14FromMantExp(mant int16, exp int8) int64 {
	if mant == 0 {
		return 0
	}
	v := int64(mant)
	shift := int(exp)
	if shift >= 0 {
		if shift >= 62 {
			if v >= 0 {
				return math.MaxInt64
			}
			return math.MinInt64
		}
		return v << uint(shift)
	}
	shift = -shift
	if shift >= 63 {
		return 0
	}
	return v >> uint(shift)
}

func q14ToX1e6(x int64) int64 {
	if x >= 0 {
		return (x*1_000_000 + 8192) / 16384
	}
	return (x*1_000_000 - 8192) / 16384
}

func writeStageCSV(path, valueColumn string, records []stageRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"source", "frame", "sub", "field", "index", valueColumn}); err != nil {
		return err
	}
	for _, r := range records {
		val := ""
		if r.hasValue {
			val = strconv.FormatInt(r.value, 10)
		}
		if err := w.Write([]string{
			r.source,
			strconv.Itoa(r.frame),
			strconv.Itoa(r.sub),
			r.field,
			strconv.Itoa(r.index),
			val,
		}); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeSummaryCSV(path string, summary map[string]*summaryState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"source", "frames", "field", "exact_notes", "value"}); err != nil {
		return err
	}

	sources := make([]string, 0, len(summary))
	for s := range summary {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	for _, source := range sources {
		st := summary[source]
		appendSummaryRow(w, source, "total_frames", int64(st.totalFrames), "exact")
		appendSummaryRow(w, source, "speech_frames", int64(st.speechFrames), "exact")
		appendSummaryRow(w, source, "sid_frames", int64(st.sidFrames), "exact")
		appendSummaryRow(w, source, "malformed_frames", int64(st.malformedFrames), "exact")
		rmsX1e6 := int64(0)
		if st.samples > 0 {
			rmsX1e6 = int64(math.Round(math.Sqrt(st.sumSq/float64(st.samples)) * 1_000_000))
		}
		appendSummaryRow(w, source, "pcm_rms_x1e6", rmsX1e6, "exact")
		appendSummaryRow(w, source, "pcm_peak", int64(st.peak), "exact")
		appendSummaryRow(w, source, "pcm_clipped_samples", st.clipped, "exact")
		appendSummaryRow(w, source, "unavailable_fields", int64(st.unavailableField), "exact")
	}
	return w.Error()
}

func appendSummaryRow(w *csv.Writer, source, field string, value int64, note string) {
	_ = w.Write([]string{
		source,
		"",
		field,
		note,
		strconv.FormatInt(value, 10),
	})
}

func maxIntKey(m map[int]struct{}) int {
	maxv := -1
	for k := range m {
		if k > maxv {
			maxv = k
		}
	}
	return maxv
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sortedKeys(m map[int]struct{}) []int {
	k := make([]int, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	sort.Ints(k)
	return k
}
