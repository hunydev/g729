package lsp

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

func TestOracleHandoff_CompareTAMELSPPipeline(t *testing.T) {
	if os.Getenv("G729_COMPARE_TAME_LSP_PIPELINE") != "1" {
		t.Skip("set G729_COMPARE_TAME_LSP_PIPELINE=1 to compare the TAME LSP pipeline oracle")
	}

	path := os.Getenv("G729_TAME_LSP_PIPELINE_EXPECTED")
	if path == "" {
		path = "/home/exedev/g729_untracked/verifier-output/decoder_tame_lsp_pipeline_expected.csv"
	}

	expected, order, frames, err := readTAMELSPPipelineExpected(path)
	if err != nil {
		t.Fatalf("read TAME LSP pipeline oracle: %v", err)
	}
	got := collectTAMELSPPipelineGot(t, frames)

	type stat struct {
		exact    int
		total    int
		missing  int
		mismatch int
	}
	stats := map[string]*stat{}
	fieldOrder := make([]string, 0)
	seenField := map[string]bool{}
	firstMismatches := make([]string, 0, 12)

	for _, k := range order {
		if !seenField[k.field] {
			seenField[k.field] = true
			fieldOrder = append(fieldOrder, k.field)
		}
		s := stats[k.field]
		if s == nil {
			s = &stat{}
			stats[k.field] = s
		}
		s.total++
		want := expected[k]
		have, ok := got[k]
		switch {
		case !ok:
			s.missing++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d sub=%d field=%s index=%d missing got want=%d",
					k.source, k.frame, k.sub, k.field, k.index, want,
				))
			}
		case have == want:
			s.exact++
		default:
			s.mismatch++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d sub=%d field=%s index=%d got=%d want=%d delta=%d",
					k.source, k.frame, k.sub, k.field, k.index, have, want, have-want,
				))
			}
		}
	}

	var exact, total, missing, mismatches int
	t.Logf("TAME LSP pipeline oracle: %s", path)
	for _, field := range fieldOrder {
		s := stats[field]
		exact += s.exact
		total += s.total
		missing += s.missing
		mismatches += s.mismatch
		t.Logf("  %-31s exact %4d/%4d  mismatches=%4d  missing=%4d",
			field, s.exact, s.total, s.mismatch, s.missing)
	}
	t.Logf("  TOTAL exact %d/%d %.2f%%  mismatches=%d  missing=%d",
		exact, total, 100.0*float64(exact)/float64(total), mismatches, missing)
	for _, m := range firstMismatches {
		t.Logf("  first mismatch: %s", m)
	}

	if os.Getenv("G729_REQUIRE_EXACT_TAME_LSP_PIPELINE") == "1" && exact != total {
		t.Fatalf("TAME LSP pipeline oracle mismatch: exact=%d total=%d mismatches=%d missing=%d",
			exact, total, mismatches, missing)
	}
}

type tameLSPPipelineKey struct {
	source string
	frame  int
	sub    int
	field  string
	index  int
}

func readTAMELSPPipelineExpected(path string) (map[tameLSPPipelineKey]int64, []tameLSPPipelineKey, map[int]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil, fmt.Errorf("empty csv")
	}
	wantHeader := []string{"source", "frame", "sub", "field", "index", "expected", "note"}
	if len(rows[0]) != len(wantHeader) {
		return nil, nil, nil, fmt.Errorf("header width = %d, want %d", len(rows[0]), len(wantHeader))
	}
	for i, want := range wantHeader {
		if rows[0][i] != want {
			return nil, nil, nil, fmt.Errorf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}

	expected := make(map[tameLSPPipelineKey]int64, len(rows)-1)
	order := make([]tameLSPPipelineKey, 0, len(rows)-1)
	frames := make(map[int]bool)
	for rowNum, row := range rows[1:] {
		if len(row) != len(wantHeader) {
			return nil, nil, nil, fmt.Errorf("row %d width = %d, want %d", rowNum+2, len(row), len(wantHeader))
		}
		frame, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d frame: %w", rowNum+2, err)
		}
		sub, err := strconv.Atoi(row[2])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d sub: %w", rowNum+2, err)
		}
		index, err := strconv.Atoi(row[4])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d index: %w", rowNum+2, err)
		}
		value, err := strconv.ParseInt(row[5], 10, 64)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d expected: %w", rowNum+2, err)
		}
		k := tameLSPPipelineKey{
			source: row[0],
			frame:  frame,
			sub:    sub,
			field:  row[3],
			index:  index,
		}
		if _, exists := expected[k]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate key at row %d: %+v", rowNum+2, k)
		}
		expected[k] = value
		order = append(order, k)
		frames[frame] = true
	}
	return expected, order, frames, nil
}

func collectTAMELSPPipelineGot(t *testing.T, frames map[int]bool) map[tameLSPPipelineKey]int64 {
	t.Helper()

	maxFrame := 0
	for frame := range frames {
		if frame > maxFrame {
			maxFrame = frame
		}
	}

	bitPath := filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", "TAME.BIT")
	raw, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}
	packedFrames, bads, err := bitstream.ReadG192File(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadG192File(%s): %v", bitPath, err)
	}
	if maxFrame >= len(packedFrames) {
		t.Fatalf("oracle references frame %d but TAME.BIT has %d frames", maxFrame, len(packedFrames))
	}

	got := make(map[tameLSPPipelineKey]int64)
	add := func(frame, sub int, field string, index int, value int64) {
		if !frames[frame] {
			return
		}
		got[tameLSPPipelineKey{
			source: "TAME",
			frame:  frame,
			sub:    sub,
			field:  field,
			index:  index,
		}] = value
	}
	add10 := func(frame, sub int, field string, values *[10]int16) {
		for i, v := range values {
			add(frame, sub, field, i, int64(v))
		}
	}
	add11 := func(frame, sub int, field string, values *[11]int16) {
		for i, v := range values {
			add(frame, sub, field, i, int64(v))
		}
	}

	var d Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		if bads[frame] {
			t.Fatalf("TAME frame %d marked bad", frame)
		}
		var bf bitstream.Frame
		if err := bitstream.Unpack(packedFrames[frame], &bf); err != nil {
			t.Fatalf("Unpack TAME frame %d: %v", frame, err)
		}
		idx := Indices{
			L0: uint8(bf.L0),
			L1: uint8(bf.L1),
			L2: uint8(bf.L2),
			L3: uint8(bf.L3),
		}

		add(frame, -1, "bitstream_l0", 0, int64(idx.L0))
		add(frame, -1, "bitstream_l1", 0, int64(idx.L1))
		add(frame, -1, "bitstream_l2", 0, int64(idx.L2))
		add(frame, -1, "bitstream_l3", 0, int64(idx.L3))

		if !d.initialized {
			for k := 0; k < 4; k++ {
				d.pastResiduals[k] = initialPastResidual
			}
		}

		var residual [10]int16
		combineResidual(idx.L1, idx.L2, idx.L3, &residual)
		add10(frame, -1, "residual_combined_q13", &residual)
		rearrangeAdjacent(&residual, lsfRearrJ1)
		rearrangeAdjacent(&residual, lsfRearrJ2)
		add10(frame, -1, "residual_rearranged_q13", &residual)

		var lsf [10]int16
		d.applyPredictor(idx.L0, &residual, &lsf)
		add10(frame, -1, "lsf_after_predictor_q13", &lsf)
		enforceLSFStability(&lsf)
		add10(frame, -1, "lsf_after_stability_q13", &lsf)

		var currLSP [10]int16
		for i := 0; i < 10; i++ {
			currLSP[i] = lsfToLSP(lsf[i])
		}

		if !d.initialized {
			d.prevLSP = initialPrevLSP
			d.initialized = true
		}
		prevLSP := d.prevLSP
		add10(frame, -1, "prev_lsp_before_interp_q15", &prevLSP)
		add10(frame, -1, "curr_lsp_q15", &currLSP)

		var sf0LSP, sf1LSP [10]int16
		interpolateLSP(&prevLSP, &currLSP, &sf0LSP, &sf1LSP)
		add10(frame, 0, "interp_lsp_q15", &sf0LSP)
		add10(frame, 0, "subframe_lsp_q15", &sf0LSP)
		add10(frame, 1, "subframe_lsp_q15", &sf1LSP)

		var sf0A, sf1A [11]int16
		LSPToLP(&sf0LSP, &sf0A)
		LSPToLP(&sf1LSP, &sf1A)
		add11(frame, 0, "lp_a_q12", &sf0A)
		add11(frame, 1, "lp_a_q12", &sf1A)

		d.prevLSP = currLSP
	}

	keys := make([]tameLSPPipelineKey, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.frame != b.frame {
			return a.frame < b.frame
		}
		if a.sub != b.sub {
			return a.sub < b.sub
		}
		if a.field != b.field {
			return a.field < b.field
		}
		return a.index < b.index
	})

	return got
}
