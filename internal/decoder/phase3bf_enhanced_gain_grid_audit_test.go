package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3bfEnhancedGainGridAudit checks whether the current enhanced
// decoder's gain-log correction point is locally optimal. FFmpeg is used only
// as an executable black-box decoder to provide numeric output references.
func TestPhase3bfEnhancedGainGridAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ENHANCED_GAIN_GRID_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ENHANCED_GAIN_GRID_AUDIT=1 to run enhanced gain-grid audit")
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
		t.Logf("Asterisk enhanced gain-grid audit skipped: %v", err)
	}

	ecQs := []int{22, 23, 24, 25, 26, 27, 28, 29, 30}
	gammaQs := []int{10, 11, 12, 13, 14, 15, 16, 17, 18}
	results := make(map[string]map[phase3bfGainGridKey]phase3bfGainGridRow)
	for _, ds := range datasets {
		rows := phase3bfEvalGainGrid(t, ds, ecQs, gammaQs)
		byKey := make(map[phase3bfGainGridKey]phase3bfGainGridRow, len(rows))
		for _, row := range rows {
			byKey[row.key] = row
		}
		results[ds.label] = byKey
		phase3bfLogGainGrid(t, ds.label, rows)
	}
	phase3bfLogGainGridCrossVerdict(t, results)
}

type phase3bfGainGridDataset struct {
	label   string
	rawPath string
	frames  int
	ref     []int16
}

type phase3bfGainGridKey struct {
	ecQ    int
	gammaQ int
}

type phase3bfGainGridRow struct {
	key      phase3bfGainGridKey
	m        blackboxMetrics
	env      phase3pEnvelopeSummary
	rmsRatio float64
	clipped  int
}

func phase3bfLoadG192GainGridDataset(t *testing.T, name string) phase3bfGainGridDataset {
	t.Helper()
	path := vectorPath(name)
	ensureTestdataPresent(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	frames := len(data) / bitstream.G192FrameBytes
	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, name+".g729")
	writeG192RawForEnvelopeAudit(t, data, frames, rawPath)
	return phase3bfGainGridDataset{
		label:   name,
		rawPath: rawPath,
		frames:  frames,
		ref:     phase3uFFmpegDecodeG192(t, data, frames, name),
	}
}

func phase3bfLoadRawGainGridDataset(t *testing.T, label, rawPath string) phase3bfGainGridDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	return phase3bfGainGridDataset{
		label:   label,
		rawPath: rawPath,
		frames:  frames,
		ref:     phase3uFFmpegDecodeRaw(t, rawPath, frames, label),
	}
}

func phase3bfEvalGainGrid(t *testing.T, ds phase3bfGainGridDataset, ecQs, gammaQs []int) []phase3bfGainGridRow {
	t.Helper()
	refRMS := diag4Rms(ds.ref)
	rows := make([]phase3bfGainGridRow, 0, len(ecQs)*len(gammaQs))
	for _, ecQ := range ecQs {
		for _, gammaQ := range gammaQs {
			out := phase3rDecodeRawEnhancedCorrections(t, ds.rawPath, ds.frames, ecQ, gammaQ)
			m := blackboxMeasure(ds.ref, out, 40)
			ratio := 0.0
			if refRMS > 0 {
				ratio = m.rms / refRMS
			}
			rows = append(rows, phase3bfGainGridRow{
				key:      phase3bfGainGridKey{ecQ: ecQ, gammaQ: gammaQ},
				m:        m,
				env:      phase3pEnvelopeCompare(ds.ref, out),
				rmsRatio: ratio,
				clipped:  phase3xCountClipped(out),
			})
		}
	}
	return rows
}

func phase3bfLogGainGrid(t *testing.T, label string, rows []phase3bfGainGridRow) {
	t.Helper()
	prod, ok := phase3bfFindGainGridRow(rows, phase3bfGainGridKey{ecQ: 26, gammaQ: 14})
	if !ok {
		t.Fatalf("%s gain grid missing production correction point", label)
	}
	t.Logf("Phase 3bf enhanced gain-grid audit - %s", label)
	t.Logf("production ecQ=%d gammaQ=%d: gSNR=%.2f seg=%.2f corr=%.3f rmsRatio=%.3f ratioMed=%.3f low<0.5=%d clipped=%d",
		prod.key.ecQ, prod.key.gammaQ, prod.m.globalSNR, prod.m.segSNR, prod.m.corr,
		prod.rmsRatio, prod.env.ratioMedian, prod.env.lowRatioFrames, prod.clipped)

	top := append([]phase3bfGainGridRow(nil), rows...)
	sort.Slice(top, func(i, j int) bool {
		if top[i].m.globalSNR == top[j].m.globalSNR {
			return top[i].m.segSNR > top[j].m.segSNR
		}
		return top[i].m.globalSNR > top[j].m.globalSNR
	})
	limit := 12
	if len(top) < limit {
		limit = len(top)
	}
	t.Logf("%-4s %-6s %9s %9s %8s %9s %9s %10s %8s %9s",
		"ecQ", "gamma", "gSNR", "segSNR", "corr", "rmsRatio", "ratioMed", "low<0.5", "clipped", "deltaG")
	for i := 0; i < limit; i++ {
		row := top[i]
		t.Logf("%-4d %-6d %9.2f %9.2f %8.3f %9.3f %9.3f %10d %8d %+9.2f",
			row.key.ecQ, row.key.gammaQ, row.m.globalSNR, row.m.segSNR, row.m.corr,
			row.rmsRatio, row.env.ratioMedian, row.env.lowRatioFrames, row.clipped,
			row.m.globalSNR-prod.m.globalSNR)
	}
}

func phase3bfLogGainGridCrossVerdict(t *testing.T, results map[string]map[phase3bfGainGridKey]phase3bfGainGridRow) {
	t.Helper()
	speechRows := results["SPEECH.BIT"]
	astRows := results["Asterisk"]
	if len(speechRows) == 0 || len(astRows) == 0 {
		t.Logf("cross-dataset verdict skipped: need both SPEECH.BIT and Asterisk")
		return
	}

	prodKey := phase3bfGainGridKey{ecQ: 26, gammaQ: 14}
	prodSpeech, okSpeech := speechRows[prodKey]
	prodAst, okAst := astRows[prodKey]
	if !okSpeech || !okAst {
		t.Fatalf("cross-dataset verdict missing production correction point")
	}

	var candidates []phase3bfGainGridCrossCandidate
	for key, ast := range astRows {
		speech, ok := speechRows[key]
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
		if ast.clipped > prodAst.clipped+10 || speech.clipped > prodSpeech.clipped+10 {
			continue
		}
		candidates = append(candidates, phase3bfGainGridCrossCandidate{
			key:    key,
			speech: speech,
			ast:    ast,
		})
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
		t.Logf("cross-dataset verdict: no gain correction point materially improves Asterisk without regressing SPEECH.BIT; keep production ecQ=26 gammaQ=14")
		return
	}
	limit := 8
	if len(candidates) < limit {
		limit = len(candidates)
	}
	t.Logf("cross-dataset verdict: %d candidate(s) beat production thresholds; inspect before promotion", len(candidates))
	t.Logf("%-4s %-6s %10s %10s %11s %11s %10s %10s",
		"ecQ", "gamma", "astDeltaG", "astDeltaS", "speechDeltaG", "speechDeltaS", "astCorr", "speechCorr")
	for i := 0; i < limit; i++ {
		c := candidates[i]
		t.Logf("%-4d %-6d %+10.2f %+10.2f %+11.2f %+11.2f %10.3f %10.3f",
			c.key.ecQ, c.key.gammaQ,
			c.ast.m.globalSNR-prodAst.m.globalSNR,
			c.ast.m.segSNR-prodAst.m.segSNR,
			c.speech.m.globalSNR-prodSpeech.m.globalSNR,
			c.speech.m.segSNR-prodSpeech.m.segSNR,
			c.ast.m.corr, c.speech.m.corr)
	}
}

type phase3bfGainGridCrossCandidate struct {
	key    phase3bfGainGridKey
	speech phase3bfGainGridRow
	ast    phase3bfGainGridRow
}

func phase3bfFindGainGridRow(rows []phase3bfGainGridRow, key phase3bfGainGridKey) (phase3bfGainGridRow, bool) {
	for _, row := range rows {
		if row.key == key {
			return row, true
		}
	}
	return phase3bfGainGridRow{}, false
}
