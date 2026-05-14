package decoder

import (
	"os"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/synth"
)

// TestDecoderTAMEExcitationOracleReplay replaces only adaptive_v_q0 with the
// verifier oracle and then rebuilds excitation_u_q0 through the local gain,
// fixed-codebook, and BuildExcitation path. If this replay is close/exact, the
// U mismatch is mostly inherited from ACB history; if not, the current
// subframe's fixed/gain/excitation construction remains suspect.
func TestDecoderTAMEExcitationOracleReplay(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_EXCITATION_ORACLE_REPLAY") != "1" {
		t.Skip("set G729_DECODER_TAME_EXCITATION_ORACLE_REPLAY=1 to run TAME excitation oracle replay")
	}

	expectedPath := os.Getenv("G729_DECODER_TAME_ACB_CHECKPOINT_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEACBCheckpointExpectedPath
	}
	expected, err := readDecoderTAMEACBCheckpointRows(expectedPath)
	if err != nil {
		t.Fatalf("read decoder TAME ACB checkpoint expected: %v", err)
	}
	oracleV := decoderTAMESubframeOverridesFromStageRows(t, expected, "adaptive_v_q0")
	oracleU := decoderTAMESubframeOverridesFromStageRows(t, expected, "excitation_u_q0")
	if len(oracleV) == 0 {
		t.Fatalf("no complete adaptive_v_q0 rows in %s", expectedPath)
	}
	if len(oracleU) == 0 {
		t.Fatalf("no complete excitation_u_q0 rows in %s", expectedPath)
	}

	keys := make(map[decoderFrameSubKey]struct{}, len(oracleV)+len(oracleU))
	for key := range oracleV {
		keys[key] = struct{}{}
	}
	for key := range oracleU {
		keys[key] = struct{}{}
	}
	taps := decoderTAMEReplayTapsByKey(t, keys)

	rows := make([]decoderTAMEExcitationReplayRow, 0, len(oracleU))
	var localAgg, replayAgg, fixedAgg decoderACBOracleAggregate
	for key, wantU := range oracleU {
		wantV, ok := oracleV[key]
		if !ok {
			continue
		}
		tap, ok := taps[key]
		if !ok {
			t.Fatalf("missing local taps for frame=%d sub=%d", key.frame, key.sub)
		}

		var zero [subframeLen]int16
		var replayU [subframeLen]int16
		var pitchFromOracleV [subframeLen]int16
		var localFixed [subframeLen]int16
		synth.BuildExcitation(tap.GpQ14, tap.GainTaps.GcMantQ14, tap.GainTaps.GcExp, &wantV, &tap.C, &replayU)
		synth.BuildExcitation(tap.GpQ14, 0, 0, &wantV, &zero, &pitchFromOracleV)
		synth.BuildExcitation(0, tap.GainTaps.GcMantQ14, tap.GainTaps.GcExp, &zero, &tap.C, &localFixed)

		impliedFixed := decoderTAMEImpliedFixedContribution(t, key, wantU, pitchFromOracleV)

		localCmp := decoderCompareACBOracle(wantU, tap.U)
		replayCmp := decoderCompareACBOracle(wantU, replayU)
		fixedCmp := decoderCompareACBOracle(impliedFixed, localFixed)
		localAgg.add(wantU, tap.U)
		replayAgg.add(wantU, replayU)
		fixedAgg.add(impliedFixed, localFixed)
		rows = append(rows, decoderTAMEExcitationReplayRow{
			key:             key,
			tInt:            tap.TInt,
			tFrac:           tap.TFrac,
			local:           localCmp,
			replayOracleV:   replayCmp,
			fixedImplied:    fixedCmp,
			localExact:      decoderInt16ArrayEqual(wantU[:], tap.U[:]),
			replayExact:     decoderInt16ArrayEqual(wantU[:], replayU[:]),
			replayFirstDiff: decoderFirstDiffInt16(wantU[:], replayU[:]),
		})
	}
	if len(rows) == 0 {
		t.Fatalf("no subframes have both complete adaptive_v_q0 and excitation_u_q0 rows")
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].key.frame != rows[j].key.frame {
			return rows[i].key.frame < rows[j].key.frame
		}
		return rows[i].key.sub < rows[j].key.sub
	})

	localAll := localAgg.finish()
	replayAll := replayAgg.finish()
	fixedAll := fixedAgg.finish()
	t.Logf("decoder TAME excitation oracle replay: path=%s subframes=%d", expectedPath, len(rows))
	t.Logf("aggregate %-16s refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		"local_u", localAll.refRMS, localAll.gotRMS, localAll.errRMS, localAll.scaledErrRMS, localAll.corr, localAll.scale, localAll.maxAbs)
	t.Logf("aggregate %-16s refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		"oracle_v_replay", replayAll.refRMS, replayAll.gotRMS, replayAll.errRMS, replayAll.scaledErrRMS, replayAll.corr, replayAll.scale, replayAll.maxAbs)
	t.Logf("aggregate %-16s refRMS=%8.2f gotRMS=%8.2f errRMS=%8.2f scaledErr=%8.2f corr=%7.4f scale=%7.4f maxAbs=%4d",
		"implied_fixed", fixedAll.refRMS, fixedAll.gotRMS, fixedAll.errRMS, fixedAll.scaledErrRMS, fixedAll.corr, fixedAll.scale, fixedAll.maxAbs)

	t.Logf("%5s %3s %8s %8s %8s %8s %8s %8s %8s %8s %7s %7s %8s",
		"frame", "sub", "T", "local", "replay", "fixImp", "lRMS", "rRMS", "fRMS", "rScErr", "rCorr", "fCorr", "rFirst")
	for _, row := range rows {
		t.Logf("%5d %3d %5d.%+d %8t %8t %8.2f %8.2f %8.2f %8.2f %8.2f %7.4f %7.4f %8d",
			row.key.frame,
			row.key.sub,
			row.tInt,
			row.tFrac,
			row.localExact,
			row.replayExact,
			row.fixedImplied.errRMS,
			row.local.errRMS,
			row.replayOracleV.errRMS,
			row.fixedImplied.refRMS,
			row.replayOracleV.scaledErrRMS,
			row.replayOracleV.corr,
			row.fixedImplied.corr,
			row.replayFirstDiff)
	}
}

type decoderTAMEExcitationReplayRow struct {
	key             decoderFrameSubKey
	tInt            int
	tFrac           int
	local           decoderACBOracleCompare
	replayOracleV   decoderACBOracleCompare
	fixedImplied    decoderACBOracleCompare
	localExact      bool
	replayExact     bool
	replayFirstDiff int
}

func decoderTAMEImpliedFixedContribution(t *testing.T, key decoderFrameSubKey, oracleU, pitchFromOracleV [subframeLen]int16) [subframeLen]int16 {
	t.Helper()
	var implied [subframeLen]int16
	for i := 0; i < subframeLen; i++ {
		v := int(oracleU[i]) - int(pitchFromOracleV[i])
		if v < -32768 || v > 32767 {
			t.Fatalf("implied fixed contribution out of int16 range: frame=%d sub=%d index=%d value=%d",
				key.frame, key.sub, i, v)
		}
		implied[i] = int16(v)
	}
	return implied
}
