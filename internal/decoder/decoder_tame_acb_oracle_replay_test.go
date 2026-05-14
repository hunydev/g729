package decoder

import (
	"os"
	"sort"
	"testing"
)

// TestDecoderTAMEACBOracleReplay feeds verifier-provided past_exc_pre_acb_q0
// rows into the local ACB interpolation. If replay matches adaptive_v_q0 where
// full pre-ACB history is available, the interpolation path is not the root
// cause; the mismatch is upstream in the excitation history itself.
func TestDecoderTAMEACBOracleReplay(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ACB_ORACLE_REPLAY") != "1" {
		t.Skip("set G729_DECODER_TAME_ACB_ORACLE_REPLAY=1 to run TAME ACB oracle replay")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_CHECKPOINT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	expected, err := readDecoderTAMEACBCheckpointRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}

	past := decoderTAMEPastExcOverridesFromStageRows(t, expected)
	oracleV := decoderTAMESubframeOverridesFromStageRows(t, expected, "adaptive_v_q0")
	if len(past) == 0 {
		t.Fatalf("no complete past_exc_pre_acb_q0 rows in %s", expectedPath)
	}
	if len(oracleV) == 0 {
		t.Fatalf("no complete adaptive_v_q0 rows in %s", expectedPath)
	}

	taps := decoderTAMEReplayTapsByKey(t, decoderTAMEReplayUnionKeys(past, oracleV))
	rows := make([]decoderTAMEACBReplayRow, 0, len(past))
	var aggregate decoderACBOracleAggregate
	for key, pastExc := range past {
		wantV, ok := oracleV[key]
		if !ok {
			continue
		}
		tap, ok := taps[key]
		if !ok {
			t.Fatalf("missing local taps for frame=%d sub=%d", key.frame, key.sub)
		}
		var gotV [subframeLen]int16
		decodeAdaptiveCodebook(tap.TInt, tap.TFrac, pastExc[:], &gotV)
		cmp := decoderCompareACBOracle(wantV, gotV)
		aggregate.add(wantV, gotV)
		rows = append(rows, decoderTAMEACBReplayRow{
			key:       key,
			tInt:      tap.TInt,
			tFrac:     tap.TFrac,
			compare:   cmp,
			exact:     decoderInt16ArrayEqual(wantV[:], gotV[:]),
			firstDiff: decoderFirstDiffInt16(wantV[:], gotV[:]),
		})
	}
	if len(rows) == 0 {
		t.Fatalf("no subframes have both complete past_exc_pre_acb_q0 and adaptive_v_q0 rows")
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].key.frame != rows[j].key.frame {
			return rows[i].key.frame < rows[j].key.frame
		}
		return rows[i].key.sub < rows[j].key.sub
	})
	all := aggregate.finish()
	exactRows := 0
	for _, row := range rows {
		if row.exact {
			exactRows++
		}
	}
	t.Logf("decoder TAME ACB oracle replay: path=%s subframes=%d exactSubframes=%d",
		expectedPath, len(rows), exactRows)
	t.Logf("aggregate refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		all.refRMS, all.gotRMS, all.errRMS, all.scaledErrRMS, all.corr, all.scale, all.maxAbs)
	t.Logf("%5s %3s %8s %7s %8s %8s %8s %8s %7s %7s %8s",
		"frame", "sub", "T", "exact", "errRMS", "scErr", "refRMS", "gotRMS", "corr", "scale", "first")
	for _, row := range rows {
		t.Logf("%5d %3d %5d.%+d %7t %8.2f %8.2f %8.2f %8.2f %7.4f %7.4f %8d",
			row.key.frame,
			row.key.sub,
			row.tInt,
			row.tFrac,
			row.exact,
			row.compare.errRMS,
			row.compare.scaledErrRMS,
			row.compare.refRMS,
			row.compare.gotRMS,
			row.compare.corr,
			row.compare.scale,
			row.firstDiff)
	}
}

type decoderTAMEACBReplayRow struct {
	key       decoderFrameSubKey
	tInt      int
	tFrac     int
	compare   decoderACBOracleCompare
	exact     bool
	firstDiff int
}

func decoderTAMEPastExcOverridesFromStageRows(t *testing.T, rows []stageRow) map[decoderFrameSubKey][pastExcLen]int16 {
	t.Helper()
	type build struct {
		values [pastExcLen]int16
		set    [pastExcLen]bool
	}
	builds := make(map[decoderFrameSubKey]*build)
	for _, row := range rows {
		if row.field != "past_exc_pre_acb_q0" || !row.hasValue {
			continue
		}
		if row.index < 0 || row.index >= pastExcLen {
			t.Fatalf("past_exc_pre_acb_q0 index out of range: frame=%d sub=%d index=%d", row.frame, row.sub, row.index)
		}
		if row.value < -32768 || row.value > 32767 {
			t.Fatalf("past_exc_pre_acb_q0 value out of int16 range: frame=%d sub=%d index=%d value=%d",
				row.frame, row.sub, row.index, row.value)
		}
		key := decoderFrameSubKey{frame: row.frame, sub: row.sub}
		b := builds[key]
		if b == nil {
			b = &build{}
			builds[key] = b
		}
		b.values[row.index] = int16(row.value)
		b.set[row.index] = true
	}

	out := make(map[decoderFrameSubKey][pastExcLen]int16)
	for key, b := range builds {
		complete := true
		for _, ok := range b.set {
			if !ok {
				complete = false
				break
			}
		}
		if complete {
			out[key] = b.values
		}
	}
	return out
}

func decoderTAMEReplayUnionKeys(left map[decoderFrameSubKey][pastExcLen]int16, right map[decoderFrameSubKey][subframeLen]int16) map[decoderFrameSubKey]struct{} {
	out := make(map[decoderFrameSubKey]struct{}, len(left)+len(right))
	for key := range left {
		out[key] = struct{}{}
	}
	for key := range right {
		out[key] = struct{}{}
	}
	return out
}

func decoderTAMEReplayTapsByKey(t *testing.T, keys map[decoderFrameSubKey]struct{}) map[decoderFrameSubKey]Phase3DiagSubframeTaps {
	t.Helper()
	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		t.Fatal("TAME vector case missing")
	}
	frames, _ := readG192Frames(t, vectorPath(tc.bitFile))
	maxFrame := -1
	for key := range keys {
		if key.frame > maxFrame {
			maxFrame = key.frame
		}
	}
	if maxFrame >= len(frames) {
		t.Fatalf("TAME target frame %d out of range; vector has %d frames", maxFrame, len(frames))
	}

	out := make(map[decoderFrameSubKey]Phase3DiagSubframeTaps, len(keys))
	var dec Decoder
	for frame := 0; frame <= maxFrame; frame++ {
		taps, err := dec.DecodeWithTaps(frames[frame])
		if err != nil {
			t.Fatalf("TAME frame %d DecodeWithTaps: %v", frame, err)
		}
		for sub := range taps.Sub {
			key := decoderFrameSubKey{frame: frame, sub: sub}
			if _, want := keys[key]; want {
				out[key] = taps.Sub[sub]
			}
		}
	}
	return out
}

func decoderInt16ArrayEqual(left, right []int16) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func decoderFirstDiffInt16(left, right []int16) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			return i
		}
	}
	if len(left) != len(right) {
		return limit
	}
	return -1
}
