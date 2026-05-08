package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3bgEnhancedStateCarryAudit checks whether the remaining enhanced
// decoder gap is caused by a bad state carry in one of the decoder's
// cross-frame memories. FFmpeg is used only as an executable black-box decoder
// for numeric output references.
func TestPhase3bgEnhancedStateCarryAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ENHANCED_STATE_CARRY_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ENHANCED_STATE_CARRY_AUDIT=1 to run enhanced state-carry audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	datasets := []phase3bfGainGridDataset{
		phase3bfLoadG192GainGridDataset(t, "SPEECH.BIT"),
	}
	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(rawPath); err == nil {
		datasets = append(datasets, phase3bfLoadRawGainGridDataset(t, "Asterisk", rawPath))
	} else {
		t.Logf("Asterisk enhanced state-carry audit skipped: %v", err)
	}

	variants := []phase3bgStateVariant{
		{name: "enhanced"},
		{name: "reset_lsp_each_frame", before: phase3bgMaskLSP},
		{name: "reset_gain_each_frame", before: phase3bgMaskGain},
		{name: "reset_synth_each_frame", before: phase3bgMaskSynth},
		{name: "reset_postfilter_each_frame", before: phase3bgMaskPostfilter},
		{name: "reset_hp_each_frame", before: phase3bgMaskHP},
		{name: "reset_past_exc_each_frame", before: phase3bgMaskPastExc | phase3bgMaskPrevGP},
		{name: "reset_prev_gp_each_frame", before: phase3bgMaskPrevGP},
		{name: "reset_filters_each_frame", before: phase3bgMaskSynth | phase3bgMaskPostfilter | phase3bgMaskHP},
		{name: "reset_decoder_each_frame", before: phase3bgMaskDecoder},
		{name: "reset_after_low200_filters", afterLowRMS: 200, after: phase3bgMaskSynth | phase3bgMaskPostfilter | phase3bgMaskHP},
		{name: "reset_after_low500_filters", afterLowRMS: 500, after: phase3bgMaskSynth | phase3bgMaskPostfilter | phase3bgMaskHP},
		{name: "reset_after_low200_dynamic", afterLowRMS: 200, after: phase3bgMaskGain | phase3bgMaskSynth | phase3bgMaskPostfilter | phase3bgMaskHP | phase3bgMaskPastExc | phase3bgMaskPrevGP},
		{name: "reset_after_low500_dynamic", afterLowRMS: 500, after: phase3bgMaskGain | phase3bgMaskSynth | phase3bgMaskPostfilter | phase3bgMaskHP | phase3bgMaskPastExc | phase3bgMaskPrevGP},
	}

	results := make(map[string]map[string]phase3bgStateRow)
	for _, ds := range datasets {
		rows := phase3bgEvalStateVariants(t, ds, variants)
		byName := make(map[string]phase3bgStateRow, len(rows))
		for _, row := range rows {
			byName[row.name] = row
		}
		results[ds.label] = byName
		phase3bgLogStateRows(t, ds.label, rows)
	}
	phase3bgLogCrossVerdict(t, results)
}

type phase3bgResetMask uint16

const (
	phase3bgMaskLSP phase3bgResetMask = 1 << iota
	phase3bgMaskGain
	phase3bgMaskSynth
	phase3bgMaskPostfilter
	phase3bgMaskHP
	phase3bgMaskPastExc
	phase3bgMaskPrevGP
	phase3bgMaskDecoder
)

type phase3bgStateVariant struct {
	name        string
	before      phase3bgResetMask
	afterLowRMS float64
	after       phase3bgResetMask
}

type phase3bgStateRow struct {
	name     string
	m        blackboxMetrics
	env      phase3pEnvelopeSummary
	rmsRatio float64
	clipped  int
	resets   int
}

func phase3bgEvalStateVariants(t *testing.T, ds phase3bfGainGridDataset, variants []phase3bgStateVariant) []phase3bgStateRow {
	t.Helper()
	refRMS := diag4Rms(ds.ref)
	rows := make([]phase3bgStateRow, 0, len(variants))
	for _, variant := range variants {
		out, resets := phase3bgDecodeEnhancedStateVariant(t, ds.rawPath, ds.frames, variant)
		if variant.name == "enhanced" {
			prod := phase3rDecodeRawEnhanced(t, ds.rawPath, ds.frames)
			if !phase3eEqualPCM(out, prod) {
				t.Fatalf("%s enhanced state-carry baseline diverges from DecodeEnvelopeRecovered", ds.label)
			}
		}
		m := blackboxMeasure(ds.ref, out, 40)
		ratio := 0.0
		if refRMS > 0 {
			ratio = m.rms / refRMS
		}
		rows = append(rows, phase3bgStateRow{
			name:     variant.name,
			m:        m,
			env:      phase3pEnvelopeCompare(ds.ref, out),
			rmsRatio: ratio,
			clipped:  phase3xCountClipped(out),
			resets:   resets,
		})
	}
	return rows
}

func phase3bgDecodeEnhancedStateVariant(t *testing.T, rawPath string, frames int, variant phase3bgStateVariant) ([]int16, int) {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw g729 payload: %v", err)
	}
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}

	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var resets int
	for f := 0; f < frames; f++ {
		if variant.before != 0 {
			phase3bgResetState(&dec, variant.before)
			resets++
		}
		frameRaw := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		frameOut := out[f*frameSamples : (f+1)*frameSamples]
		if err := dec.decodeEnvelopeRecoveredWithLogCorrections(frameRaw, false, frameOut, 26, 14); err != nil {
			t.Fatalf("DecodeEnvelopeRecovered[%s] frame %d: %v", variant.name, f, err)
		}
		if variant.after != 0 && variant.afterLowRMS > 0 && envelopeRMS(frameOut) < variant.afterLowRMS {
			phase3bgResetState(&dec, variant.after)
			resets++
		}
	}
	return out, resets
}

func phase3bgResetState(dec *Decoder, mask phase3bgResetMask) {
	if mask&phase3bgMaskDecoder != 0 {
		dec.Reset()
		return
	}
	if mask&phase3bgMaskLSP != 0 {
		dec.lsp.Reset()
	}
	if mask&phase3bgMaskGain != 0 {
		dec.gn.Reset()
	}
	if mask&phase3bgMaskSynth != 0 {
		dec.syn.Reset()
	}
	if mask&phase3bgMaskPostfilter != 0 {
		dec.pst.Reset()
	}
	if mask&phase3bgMaskHP != 0 {
		dec.hpX = [2]int16{}
		dec.hpY = [2]int32{}
	}
	if mask&phase3bgMaskPastExc != 0 {
		dec.pastExc = [pastExcLen]int16{}
	}
	if mask&phase3bgMaskPrevGP != 0 {
		dec.prevGpQ14 = 0
	}
}

func phase3bgLogStateRows(t *testing.T, label string, rows []phase3bgStateRow) {
	t.Helper()
	prod, ok := phase3bgFindStateRow(rows, "enhanced")
	if !ok {
		t.Fatalf("%s state-carry rows missing enhanced baseline", label)
	}
	t.Logf("Phase 3bg enhanced state-carry audit - %s", label)
	t.Logf("enhanced baseline: gSNR=%.2f seg=%.2f corr=%.3f rmsRatio=%.3f ratioMed=%.3f low<0.5=%d clipped=%d",
		prod.m.globalSNR, prod.m.segSNR, prod.m.corr, prod.rmsRatio,
		prod.env.ratioMedian, prod.env.lowRatioFrames, prod.clipped)
	t.Logf("%-30s %7s %9s %9s %8s %9s %9s %10s %8s %9s",
		"variant", "resets", "gSNR", "segSNR", "corr", "rmsRatio", "ratioMed", "low<0.5", "clipped", "deltaG")
	for _, row := range rows {
		t.Logf("%-30s %7d %9.2f %9.2f %8.3f %9.3f %9.3f %10d %8d %+9.2f",
			row.name, row.resets, row.m.globalSNR, row.m.segSNR, row.m.corr,
			row.rmsRatio, row.env.ratioMedian, row.env.lowRatioFrames, row.clipped,
			row.m.globalSNR-prod.m.globalSNR)
	}

	top := append([]phase3bgStateRow(nil), rows...)
	sort.Slice(top, func(i, j int) bool {
		if top[i].m.globalSNR == top[j].m.globalSNR {
			return top[i].m.segSNR > top[j].m.segSNR
		}
		return top[i].m.globalSNR > top[j].m.globalSNR
	})
	limit := 5
	if len(top) < limit {
		limit = len(top)
	}
	t.Logf("top by global SNR:")
	for i := 0; i < limit; i++ {
		row := top[i]
		t.Logf("  %s gSNR=%.2f seg=%.2f corr=%.3f delta=%+.2f resets=%d",
			row.name, row.m.globalSNR, row.m.segSNR, row.m.corr,
			row.m.globalSNR-prod.m.globalSNR, row.resets)
	}
}

func phase3bgLogCrossVerdict(t *testing.T, results map[string]map[string]phase3bgStateRow) {
	t.Helper()
	speechRows := results["SPEECH.BIT"]
	astRows := results["Asterisk"]
	if len(speechRows) == 0 || len(astRows) == 0 {
		t.Logf("cross-dataset verdict skipped: need both SPEECH.BIT and Asterisk")
		return
	}
	prodSpeech, okSpeech := speechRows["enhanced"]
	prodAst, okAst := astRows["enhanced"]
	if !okSpeech || !okAst {
		t.Fatalf("cross-dataset verdict missing enhanced baseline")
	}

	var candidates []phase3bgCrossCandidate
	for name, ast := range astRows {
		if name == "enhanced" {
			continue
		}
		speech, ok := speechRows[name]
		if !ok {
			continue
		}
		if ast.m.globalSNR <= prodAst.m.globalSNR+0.25 {
			continue
		}
		if ast.m.segSNR < prodAst.m.segSNR-0.10 || ast.m.corr < prodAst.m.corr-0.005 {
			continue
		}
		if speech.m.globalSNR < prodSpeech.m.globalSNR-0.25 || speech.m.segSNR < prodSpeech.m.segSNR-0.25 || speech.m.corr < prodSpeech.m.corr-0.01 {
			continue
		}
		candidates = append(candidates, phase3bgCrossCandidate{name: name, speech: speech, ast: ast})
	}
	sort.Slice(candidates, func(i, j int) bool {
		di := candidates[i].ast.m.globalSNR - prodAst.m.globalSNR
		dj := candidates[j].ast.m.globalSNR - prodAst.m.globalSNR
		if di == dj {
			return candidates[i].speech.m.globalSNR > candidates[j].speech.m.globalSNR
		}
		return di > dj
	})
	if len(candidates) == 0 {
		t.Logf("cross-dataset verdict: no state reset/carry variant materially improves Asterisk without regressing SPEECH.BIT; keep continuous enhanced decoder state")
		return
	}
	t.Logf("cross-dataset verdict: %d state-carry candidate(s) beat thresholds; inspect before promotion", len(candidates))
	for _, c := range candidates {
		t.Logf("  %s astΔG=%+.2f astΔS=%+.2f speechΔG=%+.2f speechΔS=%+.2f",
			c.name,
			c.ast.m.globalSNR-prodAst.m.globalSNR,
			c.ast.m.segSNR-prodAst.m.segSNR,
			c.speech.m.globalSNR-prodSpeech.m.globalSNR,
			c.speech.m.segSNR-prodSpeech.m.segSNR)
	}
}

type phase3bgCrossCandidate struct {
	name   string
	speech phase3bgStateRow
	ast    phase3bgStateRow
}

func phase3bgFindStateRow(rows []phase3bgStateRow, name string) (phase3bgStateRow, bool) {
	for _, row := range rows {
		if row.name == name {
			return row, true
		}
	}
	return phase3bgStateRow{}, false
}
