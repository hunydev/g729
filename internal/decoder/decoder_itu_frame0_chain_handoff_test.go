package decoder

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"testing"
)

const decoderITUFrame0ChainExpectedPath = "/home/exedev/g729/testdata/oracle/handoff/decoder_itu_stage_frame0_chain_expected.csv"

func TestOracleHandoff_CompareDecoderITUFrame0Chain(t *testing.T) {
	if os.Getenv("G729_COMPARE_DECODER_ITU_FRAME0_CHAIN") != "1" {
		t.Skip("set G729_COMPARE_DECODER_ITU_FRAME0_CHAIN=1 to compare decoder ITU frame-0 chain artifact")
	}

	expected, err := readDecoderITUFrame0ChainRows(decoderITUFrame0ChainExpectedPath)
	if err != nil {
		t.Fatalf("read decoder ITU frame-0 chain expected: %v", err)
	}
	targets := make(map[string]map[int]struct{})
	for _, row := range expected {
		if _, ok := targets[row.source]; !ok {
			targets[row.source] = make(map[int]struct{})
		}
		targets[row.source][row.frame] = struct{}{}
	}
	got, err := collectDecoderITUStageRows(t, targets)
	if err != nil {
		t.Fatalf("collect decoder ITU frame-0 chain got rows: %v", err)
	}
	gotByKey := make(map[decoderStageKey]stageRow, len(got))
	for _, row := range got {
		gotByKey[decoderStageRowKey(row)] = row
	}

	var fixedExact, fixedTotal, pstExact, pstTotal int
	var hpRangeTotal, hpRangeOK, missingGot int
	first := make([]decoderStageMismatch, 0, 16)
	bySource := make(map[string]*decoderITUFrame0ChainStats)

	hpRanges := make(map[decoderStageKey]*decoderITUFrame0HPRange)
	for _, want := range expected {
		key := decoderStageRowKey(want.stageRow)
		switch want.field {
		case "fixed_c_q13":
			fixedTotal++
			sourceStats := decoderITUFrame0ChainStatsFor(bySource, want.source)
			sourceStats.fixedTotal++
			gotRow, ok := gotByKey[key]
			if !ok {
				missingGot++
				appendFrame0ChainMismatch(&first, key, decoderStageValueString(want.stageRow), "", "missing got")
				continue
			}
			if gotRow.hasValue && gotRow.value == want.value {
				fixedExact++
				sourceStats.fixedExact++
				continue
			}
			appendFrame0ChainMismatch(&first, key, decoderStageValueString(want.stageRow), decoderStageValueString(gotRow), "fixed_c mismatch")
		case "pst_pcm_q0":
			pstTotal++
			sourceStats := decoderITUFrame0ChainStatsFor(bySource, want.source)
			sourceStats.pstTotal++
			gotKey := key
			gotKey.field = "pcm_q0"
			gotRow, ok := gotByKey[gotKey]
			if !ok {
				missingGot++
				appendFrame0ChainMismatch(&first, gotKey, decoderStageValueString(want.stageRow), "", "missing pcm got")
				continue
			}
			if gotRow.hasValue && gotRow.value == want.value {
				pstExact++
				sourceStats.pstExact++
				continue
			}
			appendFrame0ChainMismatch(&first, gotKey, decoderStageValueString(want.stageRow), decoderStageValueString(gotRow), "pst_pcm mismatch")
		case "hp_inverse_low_q0", "hp_inverse_high_q0":
			gotKey := key
			gotKey.field = "hp_q0"
			rng := hpRanges[gotKey]
			if rng == nil {
				rng = &decoderITUFrame0HPRange{}
				hpRanges[gotKey] = rng
			}
			if want.field == "hp_inverse_low_q0" {
				rng.haveLow = true
				rng.low = want.value
			} else {
				rng.haveHigh = true
				rng.high = want.value
			}
		default:
			t.Fatalf("unexpected frame-0 chain field %q", want.field)
		}
	}

	for key, rng := range hpRanges {
		if !rng.haveLow || !rng.haveHigh {
			t.Fatalf("incomplete hp inverse range for %+v", key)
		}
		hpRangeTotal++
		sourceStats := decoderITUFrame0ChainStatsFor(bySource, key.source)
		sourceStats.hpTotal++
		gotRow, ok := gotByKey[key]
		if !ok {
			missingGot++
			appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), "", "missing hp got")
			continue
		}
		if gotRow.hasValue && gotRow.value >= rng.low && gotRow.value <= rng.high {
			hpRangeOK++
			sourceStats.hpOK++
			continue
		}
		appendFrame0ChainMismatch(&first, key, fmt.Sprintf("[%d,%d]", rng.low, rng.high), decoderStageValueString(gotRow), "hp outside inverse range")
	}

	t.Logf("decoder_itu_frame0_chain: fixed_c_q13 exact %d/%d %.2f%%",
		fixedExact, fixedTotal, percent(fixedExact, fixedTotal))
	t.Logf("decoder_itu_frame0_chain: pst_pcm_q0 exact %d/%d %.2f%%",
		pstExact, pstTotal, percent(pstExact, pstTotal))
	t.Logf("decoder_itu_frame0_chain: hp_q0 within PST-derived inverse range %d/%d %.2f%% missing_got=%d",
		hpRangeOK, hpRangeTotal, percent(hpRangeOK, hpRangeTotal), missingGot)
	for _, source := range sortedStringKeys(bySource) {
		st := bySource[source]
		t.Logf("decoder_itu_frame0_chain source %s: fixed_c=%d/%d pst=%d/%d hp_range=%d/%d",
			source, st.fixedExact, st.fixedTotal, st.pstExact, st.pstTotal, st.hpOK, st.hpTotal)
	}
	for i, m := range first {
		t.Logf("mismatch[%d]: source=%s frame=%d sub=%d field=%s index=%d expected=%s got=%s notes=%s",
			i, m.key.source, m.key.frame, m.key.sub, m.key.field, m.key.index, m.want, m.got, m.note)
	}

	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_CHAIN_FIXED_C") == "1" && fixedExact != fixedTotal {
		t.Fatalf("decoder ITU frame-0 fixed_c rows mismatch: exact %d/%d", fixedExact, fixedTotal)
	}
	if os.Getenv("G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_CHAIN") == "1" &&
		(fixedExact != fixedTotal || pstExact != pstTotal || hpRangeOK != hpRangeTotal || missingGot != 0) {
		t.Fatalf("decoder ITU frame-0 chain is not exact: fixed_c=%d/%d pst=%d/%d hp=%d/%d missing=%d",
			fixedExact, fixedTotal, pstExact, pstTotal, hpRangeOK, hpRangeTotal, missingGot)
	}
}

type decoderITUFrame0ChainRow struct {
	stageRow
	note string
}

type decoderITUFrame0HPRange struct {
	low      int64
	high     int64
	haveLow  bool
	haveHigh bool
}

type decoderITUFrame0ChainStats struct {
	fixedExact int
	fixedTotal int
	pstExact   int
	pstTotal   int
	hpOK       int
	hpTotal    int
}

func decoderITUFrame0ChainStatsFor(stats map[string]*decoderITUFrame0ChainStats, source string) *decoderITUFrame0ChainStats {
	st := stats[source]
	if st == nil {
		st = &decoderITUFrame0ChainStats{}
		stats[source] = st
	}
	return st
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readDecoderITUFrame0ChainRows(path string) ([]decoderITUFrame0ChainRow, error) {
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

	var rows []decoderITUFrame0ChainRow
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
		rows = append(rows, decoderITUFrame0ChainRow{
			stageRow: stageRow{
				source:   rec[0],
				frame:    frame,
				sub:      sub,
				field:    rec[3],
				index:    index,
				hasValue: true,
				value:    value,
			},
			note: rec[6],
		})
	}
	return rows, nil
}

func appendFrame0ChainMismatch(dst *[]decoderStageMismatch, key decoderStageKey, want, got, note string) {
	if len(*dst) == cap(*dst) {
		return
	}
	*dst = append(*dst, decoderStageMismatch{
		key:  key,
		want: want,
		got:  got,
		note: note,
	})
}
