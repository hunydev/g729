package g729

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/lpc"
	"github.com/hunydev/g729/internal/pitch/closedloop"
)

const (
	fcbTreeHandoffExpectedName          = "fcb_tree_search_expected_template.csv"
	fcbTreeHandoffGotName               = "fcb_tree_search_got.csv"
	fcbTreeHandoffVerifierPrompt        = "FCB_TREE_SEARCH_VERIFIER_PROMPT.md"
	fcbTreeUserAudioHandoffExpectedName = "fcb_tree_search_user_audio_expected_template.csv"
	fcbTreeUserAudioHandoffGotName      = "fcb_tree_search_user_audio_got.csv"
	fcbTreeUserAudioSamplePath          = "testdata/external/user_quality_audio.m4a"
	fcbTreeBlankExpected                = int64(-1 << 63)
)

type fcbTreeHandoffRow struct {
	field string
	frame int
	sub   int
	index int
	value int64
}

type fcbTreeHandoffKey struct {
	field string
	frame int
	sub   int
	index int
}

func TestOracleHandoff_WriteFCBTreeSearchHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_FCB_TREE_SEARCH_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1 to refresh FCB tree-search handoff files")
	}

	rows := collectFCBTreeHandoffRows(t, fcbTreeHandoffTargetFrames())
	if len(rows) == 0 {
		t.Fatal("no FCB tree-search handoff rows collected")
	}

	if err := os.MkdirAll(encoderClosedLoopHandoffDir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeHandoffExpectedName)
	gotPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeHandoffGotName)
	if fcbTreeExpectedHasFilledCells(t, expectedPath) && os.Getenv("G729_OVERWRITE_VERIFIER_EXPECTED") != "1" {
		t.Fatalf("%s already has verifier-filled expected cells; set G729_OVERWRITE_VERIFIER_EXPECTED=1 to discard them", expectedPath)
	}
	if err := writeFCBTreeHandoffCSV(expectedPath, "expected", rows, true); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
	if err := writeFCBTreeHandoffCSV(gotPath, "got", rows, false); err != nil {
		t.Fatalf("write got: %v", err)
	}
	t.Logf("wrote %d FCB tree-search handoff rows", len(rows))
}

func TestOracleHandoff_WriteFCBTreeSearchGot(t *testing.T) {
	if os.Getenv("G729_WRITE_FCB_TREE_SEARCH_GOT") != "1" {
		t.Skip("set G729_WRITE_FCB_TREE_SEARCH_GOT=1 to refresh only the FCB tree-search got file")
	}

	rows := collectFCBTreeHandoffRows(t, fcbTreeHandoffTargetFrames())
	if len(rows) == 0 {
		t.Fatal("no FCB tree-search handoff rows collected")
	}
	gotPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeHandoffGotName)
	if err := writeFCBTreeHandoffCSV(gotPath, "got", rows, false); err != nil {
		t.Fatalf("write got: %v", err)
	}
	t.Logf("wrote %d FCB tree-search got rows", len(rows))
}

func TestOracleHandoff_WriteFCBTreeSearchUserAudioHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 to refresh user-audio FCB tree-search handoff files")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	if _, err := os.Stat(fcbTreeUserAudioSamplePath); err != nil {
		t.Skipf("user-audio sample unavailable: %v", err)
	}
	samples := readExternalQualitySamples(t, fcbTreeUserAudioSamplePath)
	if rem := len(samples) % FrameSamples; rem != 0 {
		samples = append(samples, make([]int16, FrameSamples-rem)...)
	}
	rows := collectFCBTreeHandoffRowsFromSamples(t, samples, fcbTreeUserAudioSamplePath, fcbTreeHandoffTargetFrames())
	if len(rows) == 0 {
		t.Fatal("no user-audio FCB tree-search handoff rows collected")
	}

	if err := os.MkdirAll(encoderClosedLoopHandoffDir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeUserAudioHandoffExpectedName)
	gotPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeUserAudioHandoffGotName)
	if fcbTreeExpectedHasFilledCells(t, expectedPath) && os.Getenv("G729_OVERWRITE_VERIFIER_EXPECTED") != "1" {
		t.Fatalf("%s already has verifier-filled expected cells; set G729_OVERWRITE_VERIFIER_EXPECTED=1 to discard them", expectedPath)
	}
	if err := writeFCBTreeHandoffCSV(expectedPath, "expected", rows, true); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
	if err := writeFCBTreeHandoffCSV(gotPath, "got", rows, false); err != nil {
		t.Fatalf("write got: %v", err)
	}
	t.Logf("wrote %d user-audio FCB tree-search handoff rows from %s", len(rows), fcbTreeUserAudioSamplePath)
}

func TestOracleHandoff_WriteFCBTreeSearchUserAudioGot(t *testing.T) {
	if os.Getenv("G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_GOT") != "1" {
		t.Skip("set G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_GOT=1 to refresh only the user-audio FCB tree-search got file")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	if _, err := os.Stat(fcbTreeUserAudioSamplePath); err != nil {
		t.Skipf("user-audio sample unavailable: %v", err)
	}
	samples := readExternalQualitySamples(t, fcbTreeUserAudioSamplePath)
	if rem := len(samples) % FrameSamples; rem != 0 {
		samples = append(samples, make([]int16, FrameSamples-rem)...)
	}
	rows := collectFCBTreeHandoffRowsFromSamples(t, samples, fcbTreeUserAudioSamplePath, fcbTreeHandoffTargetFrames())
	if len(rows) == 0 {
		t.Fatal("no user-audio FCB tree-search handoff rows collected")
	}
	gotPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeUserAudioHandoffGotName)
	if err := writeFCBTreeHandoffCSV(gotPath, "got", rows, false); err != nil {
		t.Fatalf("write got: %v", err)
	}
	t.Logf("wrote %d user-audio FCB tree-search got rows from %s", len(rows), fcbTreeUserAudioSamplePath)
}

func TestOracleHandoff_FCBTreeSearchGotMatchesCurrentSurface(t *testing.T) {
	rows := collectFCBTreeHandoffRows(t, fcbTreeHandoffTargetFrames())
	assertFCBTreeGotMatchesRows(t, filepath.Join(encoderClosedLoopHandoffDir, fcbTreeHandoffGotName), rows)
}

func TestOracleHandoff_FCBTreeSearchUserAudioGotMatchesCurrentSurface(t *testing.T) {
	if _, err := os.Stat(fcbTreeUserAudioSamplePath); err != nil {
		t.Skipf("user-audio sample unavailable: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	samples := readExternalQualitySamples(t, fcbTreeUserAudioSamplePath)
	if rem := len(samples) % FrameSamples; rem != 0 {
		samples = append(samples, make([]int16, FrameSamples-rem)...)
	}
	rows := collectFCBTreeHandoffRowsFromSamples(t, samples, fcbTreeUserAudioSamplePath, fcbTreeHandoffTargetFrames())
	assertFCBTreeGotMatchesRows(t, filepath.Join(encoderClosedLoopHandoffDir, fcbTreeUserAudioHandoffGotName), rows)
}

func TestOracleHandoff_CompareFCBTreeSearchHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_FCB_TREE_SEARCH_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 after verifier fills fcb_tree_search_expected_template.csv")
	}

	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeHandoffExpectedName)
	expected, blanks, err := readFCBTreeExpected(expectedPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no rows")
	}

	targets := make(map[int]struct{})
	for _, row := range expected {
		targets[row.frame] = struct{}{}
	}
	gotRows := collectFCBTreeHandoffRows(t, sortedEncoderClosedLoopFrameKeys(targets))
	got := make(map[fcbTreeHandoffKey]int64, len(gotRows))
	for _, row := range gotRows {
		got[fcbTreeHandoffRowKey(row)] = row.value
	}

	var exact, mismatch, missing int
	first := make([]string, 0, 12)
	for _, want := range expected {
		if want.value == fcbTreeBlankExpected {
			continue
		}
		key := fcbTreeHandoffRowKey(want)
		have, ok := got[key]
		if !ok {
			missing++
			mismatch++
			if len(first) < cap(first) {
				first = append(first, fmt.Sprintf("missing got: %+v expected=%d", key, want.value))
			}
			continue
		}
		if have == want.value {
			exact++
			continue
		}
		mismatch++
		if len(first) < cap(first) {
			first = append(first, fmt.Sprintf("mismatch: %+v expected=%d got=%d", key, want.value, have))
		}
	}

	filled := len(expected) - blanks
	if filled == 0 {
		t.Fatalf("expected handoff has no filled numeric cells; verifier output is required before comparison")
	}
	t.Logf("FCB tree-search handoff: exact %d/%d %.2f%% mismatches=%d blanks=%d missing=%d",
		exact, filled, percent(exact, filled), mismatch, blanks, missing)
	for _, msg := range first {
		t.Log(msg)
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF") == "1" && blanks > 0 {
		t.Fatalf("expected handoff still has %d blank cells", blanks)
	}
	if os.Getenv("G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("FCB tree-search handoff has %d mismatches", mismatch)
	}
}

func TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 after verifier fills fcb_tree_search_user_audio_expected_template.csv")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, fcbTreeUserAudioHandoffExpectedName)
	expected, blanks, err := readFCBTreeExpected(expectedPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no rows")
	}

	targets := make(map[int]struct{})
	for _, row := range expected {
		targets[row.frame] = struct{}{}
	}
	if _, err := os.Stat(fcbTreeUserAudioSamplePath); err != nil {
		t.Skipf("user-audio sample unavailable: %v", err)
	}
	samples := readExternalQualitySamples(t, fcbTreeUserAudioSamplePath)
	if rem := len(samples) % FrameSamples; rem != 0 {
		samples = append(samples, make([]int16, FrameSamples-rem)...)
	}
	gotRows := collectFCBTreeHandoffRowsFromSamples(t, samples, fcbTreeUserAudioSamplePath, sortedEncoderClosedLoopFrameKeys(targets))
	got := make(map[fcbTreeHandoffKey]int64, len(gotRows))
	for _, row := range gotRows {
		got[fcbTreeHandoffRowKey(row)] = row.value
	}

	var exact, mismatch, missing int
	first := make([]string, 0, 12)
	for _, want := range expected {
		if want.value == fcbTreeBlankExpected {
			continue
		}
		key := fcbTreeHandoffRowKey(want)
		have, ok := got[key]
		if !ok {
			missing++
			mismatch++
			if len(first) < cap(first) {
				first = append(first, fmt.Sprintf("missing got: %+v expected=%d", key, want.value))
			}
			continue
		}
		if have == want.value {
			exact++
			continue
		}
		mismatch++
		if len(first) < cap(first) {
			first = append(first, fmt.Sprintf("mismatch: %+v expected=%d got=%d", key, want.value, have))
		}
	}

	filled := len(expected) - blanks
	if filled == 0 {
		t.Fatalf("expected handoff has no filled numeric cells; verifier output is required before comparison")
	}
	t.Logf("user-audio FCB tree-search handoff: exact %d/%d %.2f%% mismatches=%d blanks=%d missing=%d",
		exact, filled, percent(exact, filled), mismatch, blanks, missing)
	for _, msg := range first {
		t.Log(msg)
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF") == "1" && blanks > 0 {
		t.Fatalf("expected handoff still has %d blank cells", blanks)
	}
	if os.Getenv("G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("user-audio FCB tree-search handoff has %d mismatches", mismatch)
	}
}

func fcbTreeHandoffTargetFrames() []int {
	return []int{292, 293, 294}
}

func collectFCBTreeHandoffRows(t *testing.T, targetFrames []int) []fcbTreeHandoffRow {
	t.Helper()
	data, err := os.ReadFile(encoderClosedLoopFrameSamplePath)
	if err != nil {
		t.Fatalf("read %s: %v", encoderClosedLoopFrameSamplePath, err)
	}
	samples := handoffS16LEToSamples(data)
	return collectFCBTreeHandoffRowsFromSamples(t, samples, "SPEECH.IN", targetFrames)
}

func collectFCBTreeHandoffRowsFromSamples(t *testing.T, samples []int16, sampleLabel string, targetFrames []int) []fcbTreeHandoffRow {
	t.Helper()
	targets := make(map[int]struct{}, len(targetFrames))
	maxFrame := 0
	for _, frame := range targetFrames {
		targets[frame] = struct{}{}
		if frame > maxFrame {
			maxFrame = frame
		}
	}
	if have, want := len(samples)/FrameSamples, maxFrame+1; have < want {
		t.Fatalf("%s has %d frames, need %d", sampleLabel, have, want)
	}

	enc := NewEncoderWithProfile(EncoderProfileCore)
	rows := make([]fcbTreeHandoffRow, 0, len(targetFrames)*2*1699)
	for frame := 0; frame <= maxFrame; frame++ {
		off := frame * FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frame, err)
		}
		_ = enc.openloopStep()
		for sub := 0; sub < 2; sub++ {
			if _, ok := targets[frame]; ok {
				rows = append(rows, collectFCBTreeSubframeRows(enc, frame, sub)...)
			}
			_, _ = enc.closedloopStep(sub)
		}
	}
	return rows
}

func collectFCBTreeSubframeRows(e *Encoder, frame, sub int) []fcbTreeHandoffRow {
	const N = closedloop.SubframeLen
	aHat := &e.aHatSF1
	if sub == 1 {
		aHat = &e.aHatSF2
	}

	sStart := 120 + 40*sub
	sFrame := (*[N]int16)(e.oldSpeech[sStart : sStart+N])

	var r, x, h, xb [N]int16
	lpResidualSubframe(sFrame, (*[lpc.LPCOrder + 1]int16)(aHat), &e.lpResidualMemQ, &r)
	closedloop.TargetSignal(aHat, &r, &e.swMemErr, &x)
	closedloop.ImpulseResponse(aHat, &h)
	closedloop.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedLoopPitchSearchLen]int16
	excSlice := e.closedLoopExcitationSearch(&r, &excSearch)
	intLag, _ := closedloop.SearchInteger(&xb, excSlice, centre, sub)
	var frac int8
	if sub == 1 {
		intLag, frac = closedloop.RefineFractionSubframe2(&xb, excSlice, intLag, e.intT1)
	} else {
		intLag, frac = closedloop.RefineFractionSubframe1(&xb, excSlice, intLag)
	}

	var v, y [N]int16
	e.adaptiveVectorForSynthesis(excSlice, intLag, frac, &v)
	gp := closedloop.GpAndY(&x, &v, &h, &y)

	var xPrime [N]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)

	hSearch := h
	fcb.ApplyPitchEnhancement(&hSearch, int(intLag), fcb.ClampPitchGainForEnhancement(e.prevGpQ14))

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, &hSearch, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(&hSearch, &signs, &phi)

	limit := e.coreFCBThresholdScanLimit(sub)
	avg3, max3, threshold := fcbTreeFirstThreeStats(&dAbs)

	var selectedPos [4]int8
	var selectedSum [2]int64
	entered := fcbsearch.SearchDepthFirstThresholdScanEntered(&dAbs, &phi, &selectedPos, &selectedSum, limit)

	var fullPos [4]int8
	var fullSum [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &fullPos, &fullSum)

	rows := make([]fcbTreeHandoffRow, 0, 1699)
	add := func(field string, index int, value int64) {
		rows = append(rows, fcbTreeHandoffRow{field: field, frame: frame, sub: sub, index: index, value: value})
	}
	add("pitch_int", -1, int64(intLag))
	add("pitch_frac", -1, int64(frac))
	add("search_limit", -1, int64(limit))
	add("first3_avg_c", -1, avg3)
	add("first3_max_c", -1, max3)
	add("first3_threshold", -1, threshold)
	add("accepted_prefixes", -1, int64(entered))
	add("selected_c2", -1, selectedSum[0])
	add("selected_e", -1, selectedSum[1])
	add("full_c2", -1, fullSum[0])
	add("full_e", -1, fullSum[1])
	for i := 0; i < N; i++ {
		add("d_abs", i, int64(dAbs[i]))
		add("sign", i, int64(signs[i]))
	}
	for i := 0; i < N; i++ {
		for j := 0; j < N; j++ {
			add("phi", i*N+j, int64(phi[i][j]))
		}
	}
	for i, pos := range selectedPos {
		add("selected_position", i, int64(pos))
	}
	for i, pos := range fullPos {
		add("full_position", i, int64(pos))
	}
	return rows
}

func fcbTreeFirstThreeStats(dAbs *[closedloop.SubframeLen]int32) (avg3, max3, threshold int64) {
	var sumC, count int64
	for _, m0 := range fcbTreeTrack0 {
		d0 := int64(dAbs[m0])
		for _, m1 := range fcbTreeTrack1 {
			d01 := d0 + int64(dAbs[m1])
			for _, m2 := range fcbTreeTrack2 {
				c := d01 + int64(dAbs[m2])
				sumC += c
				count++
				if c > max3 {
					max3 = c
				}
			}
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	avg3 = sumC / count
	threshold = avg3 + (4*(max3-avg3))/10
	return avg3, max3, threshold
}

var (
	fcbTreeTrack0 = [8]int8{0, 5, 10, 15, 20, 25, 30, 35}
	fcbTreeTrack1 = [8]int8{1, 6, 11, 16, 21, 26, 31, 36}
	fcbTreeTrack2 = [8]int8{2, 7, 12, 17, 22, 27, 32, 37}
)

func writeFCBTreeHandoffCSV(path, valueColumn string, rows []fcbTreeHandoffRow, blankValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"field", "frame", "sub", "index", valueColumn}); err != nil {
		return err
	}
	for _, row := range rows {
		value := strconv.FormatInt(row.value, 10)
		if blankValue {
			value = ""
		}
		rec := []string{
			row.field,
			strconv.Itoa(row.frame),
			strconv.Itoa(row.sub),
			strconv.Itoa(row.index),
			value,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return w.Error()
}

func readFCBTreeExpected(path string) ([]fcbTreeHandoffRow, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, 0, err
	}
	if len(records) == 0 {
		return nil, 0, fmt.Errorf("empty CSV")
	}
	if got, want := strings.Join(records[0], ","), "field,frame,sub,index,expected"; got != want {
		return nil, 0, fmt.Errorf("header=%q, want %q", got, want)
	}
	rows := make([]fcbTreeHandoffRow, 0, len(records)-1)
	var blanks int
	for line, rec := range records[1:] {
		if len(rec) == 4 {
			rec = append(rec, "")
		}
		if len(rec) != 5 {
			return nil, 0, fmt.Errorf("line %d columns=%d, want 5", line+2, len(rec))
		}
		row, err := fcbTreeRowFromStrings(rec[:4])
		if err != nil {
			return nil, 0, fmt.Errorf("line %d key: %w", line+2, err)
		}
		if rec[4] == "" {
			row.value = fcbTreeBlankExpected
			blanks++
		} else {
			row.value, err = strconv.ParseInt(rec[4], 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("line %d expected=%q: %w", line+2, rec[4], err)
			}
		}
		rows = append(rows, row)
	}
	return rows, blanks, nil
}

func readFCBTreeValueCSV(path, valueColumn string) ([]fcbTreeHandoffRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV")
	}
	wantHeader := "field,frame,sub,index," + valueColumn
	if got := strings.Join(records[0], ","); got != wantHeader {
		return nil, fmt.Errorf("header=%q, want %q", got, wantHeader)
	}
	rows := make([]fcbTreeHandoffRow, 0, len(records)-1)
	seen := make(map[fcbTreeHandoffKey]struct{}, len(records)-1)
	for line, rec := range records[1:] {
		if len(rec) != 5 {
			return nil, fmt.Errorf("line %d columns=%d, want 5", line+2, len(rec))
		}
		row, err := fcbTreeRowFromStrings(rec[:4])
		if err != nil {
			return nil, fmt.Errorf("line %d key: %w", line+2, err)
		}
		key := fcbTreeHandoffRowKey(row)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("line %d duplicate key %+v", line+2, key)
		}
		seen[key] = struct{}{}
		if rec[4] == "" {
			return nil, fmt.Errorf("line %d %s is blank", line+2, valueColumn)
		}
		row.value, err = strconv.ParseInt(rec[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d %s=%q: %w", line+2, valueColumn, rec[4], err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func assertFCBTreeGotMatchesRows(t *testing.T, gotPath string, wantRows []fcbTreeHandoffRow) {
	t.Helper()
	gotRows, err := readFCBTreeValueCSV(gotPath, "got")
	if err != nil {
		t.Fatalf("read %s: %v", gotPath, err)
	}
	if len(gotRows) != len(wantRows) {
		t.Fatalf("%s rows=%d, want %d", gotPath, len(gotRows), len(wantRows))
	}
	want := make(map[fcbTreeHandoffKey]int64, len(wantRows))
	for _, row := range wantRows {
		key := fcbTreeHandoffRowKey(row)
		if _, ok := want[key]; ok {
			t.Fatalf("generated duplicate key %+v", key)
		}
		want[key] = row.value
	}
	var exact, mismatch, missing int
	first := make([]string, 0, 12)
	for _, row := range gotRows {
		key := fcbTreeHandoffRowKey(row)
		w, ok := want[key]
		if !ok {
			missing++
			mismatch++
			if len(first) < cap(first) {
				first = append(first, fmt.Sprintf("unexpected got key: %+v got=%d", key, row.value))
			}
			continue
		}
		if row.value == w {
			exact++
			continue
		}
		mismatch++
		if len(first) < cap(first) {
			first = append(first, fmt.Sprintf("mismatch: %+v got=%d want=%d", key, row.value, w))
		}
	}
	for _, msg := range first {
		t.Log(msg)
	}
	if missing > 0 || mismatch > 0 {
		t.Fatalf("%s is stale: exact=%d/%d mismatches=%d missing=%d", gotPath, exact, len(gotRows), mismatch, missing)
	}
}

func fcbTreeRowFromStrings(key []string) (fcbTreeHandoffRow, error) {
	var row fcbTreeHandoffRow
	if len(key) != 4 {
		return row, fmt.Errorf("key columns=%d, want 4", len(key))
	}
	row.field = key[0]
	var err error
	if row.frame, err = strconv.Atoi(key[1]); err != nil {
		return row, fmt.Errorf("frame: %w", err)
	}
	if row.sub, err = strconv.Atoi(key[2]); err != nil {
		return row, fmt.Errorf("sub: %w", err)
	}
	if row.index, err = strconv.Atoi(key[3]); err != nil {
		return row, fmt.Errorf("index: %w", err)
	}
	return row, nil
}

func fcbTreeHandoffRowKey(row fcbTreeHandoffRow) fcbTreeHandoffKey {
	return fcbTreeHandoffKey{field: row.field, frame: row.frame, sub: row.sub, index: row.index}
}

func fcbTreeExpectedHasFilledCells(t *testing.T, path string) bool {
	t.Helper()
	rows, blanks, err := readFCBTreeExpected(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return len(rows) > blanks
}
