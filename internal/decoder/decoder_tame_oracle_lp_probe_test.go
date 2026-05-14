package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestDecoderTAMEOracleLPCoeffProbe injects only the verifier-provided numeric
// lp_a_q12 rows from the TAME wide-stage artifact. It is a localization probe:
// if the final PST agreement moves, LP coefficient drift is causally relevant
// for the late TAME envelope; if not, focus remains on gain/excitation history.
func TestDecoderTAMEOracleLPCoeffProbe(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ORACLE_LP_PROBE") != "1" {
		t.Skip("set G729_DECODER_TAME_ORACLE_LP_PROBE=1 to run TAME oracle LP probe")
	}

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		t.Fatal("TAME vector case missing")
	}
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.pstFile, err)
	}
	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)

	expectedPath := os.Getenv("G729_DECODER_TAME_STAGE_WIDE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideExpectedPath
	}
	overrides := decoderTAMEWideLPOverrides(t, expectedPath)

	production := decodeVariant(t, bitData, frames, nil, nil)
	oracleLP := decodeTAMEWithLPOverrides(t, bitData, frames, overrides)

	prodMetrics := blackboxMeasure(ref, production, 40)
	oracleMetrics := blackboxMeasure(ref, oracleLP, 40)
	t.Logf("TAME oracle LP coeff probe: overrides=%d", len(overrides))
	t.Logf("%-18s %8s %8s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "diffSamp")
	t.Logf("%-18s %8.2f %8d %10.2f %10.2f %8.3f %9.2f %9.3f %9d",
		"production", prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR, prodMetrics.corr,
		0.0, 0.0, 0)
	t.Logf("%-18s %8.2f %8d %10.2f %10.2f %8.3f %9.2f %9.3f %9d",
		"oracle_lp", oracleMetrics.rms, oracleMetrics.peak, oracleMetrics.globalSNR, oracleMetrics.segSNR, oracleMetrics.corr,
		oracleMetrics.globalSNR-prodMetrics.globalSNR, oracleMetrics.corr-prodMetrics.corr,
		phase3fDiffSamples(production, oracleLP))
}

// TestDecoderTAMEOracleACBVectorProbe injects only adaptive_v_q0 rows from the
// TAME wide-stage artifact. This isolates whether the remaining late TAME
// envelope is materially driven by past-excitation/adaptive-codebook history.
func TestDecoderTAMEOracleACBVectorProbe(t *testing.T) {
	if os.Getenv("G729_DECODER_TAME_ORACLE_ACB_PROBE") != "1" {
		t.Skip("set G729_DECODER_TAME_ORACLE_ACB_PROBE=1 to run TAME oracle ACB-vector probe")
	}

	tc, ok := decoderITUValidationCaseByName("TAME")
	if !ok {
		t.Fatal("TAME vector case missing")
	}
	bitPath := vectorPath(tc.bitFile)
	pstPath := vectorPath(tc.pstFile)
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.bitFile, err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read %s: %v", tc.pstFile, err)
	}
	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)

	expectedPath := os.Getenv("G729_DECODER_TAME_STAGE_WIDE_EXPECTED")
	if expectedPath == "" {
		expectedPath = decoderTAMEStageWideExpectedPath
	}
	overrides := decoderTAMEWideSubframeOverrides(t, expectedPath, "adaptive_v_q0")

	production := decodeVariant(t, bitData, frames, nil, nil)
	oracleACB := decodeTAMEWithACBOverrides(t, bitData, frames, overrides)

	prodMetrics := blackboxMeasure(ref, production, 40)
	oracleMetrics := blackboxMeasure(ref, oracleACB, 40)
	t.Logf("TAME oracle ACB-vector probe: overrides=%d", len(overrides))
	t.Logf("%-18s %8s %8s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "diffSamp")
	t.Logf("%-18s %8.2f %8d %10.2f %10.2f %8.3f %9.2f %9.3f %9d",
		"production", prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR, prodMetrics.corr,
		0.0, 0.0, 0)
	t.Logf("%-18s %8.2f %8d %10.2f %10.2f %8.3f %9.2f %9.3f %9d",
		"oracle_acb", oracleMetrics.rms, oracleMetrics.peak, oracleMetrics.globalSNR, oracleMetrics.segSNR, oracleMetrics.corr,
		oracleMetrics.globalSNR-prodMetrics.globalSNR, oracleMetrics.corr-prodMetrics.corr,
		phase3fDiffSamples(production, oracleACB))
}

type decoderFrameSubKey struct {
	frame int
	sub   int
}

func decoderTAMEWideLPOverrides(t *testing.T, path string) map[decoderFrameSubKey][lpcOrder + 1]int16 {
	t.Helper()
	rows, err := readDecoderTAMEStageWideRows(path)
	if err != nil {
		t.Fatalf("read decoder TAME stage wide expected: %v", err)
	}
	type build struct {
		values [lpcOrder + 1]int16
		set    [lpcOrder + 1]bool
	}
	builds := make(map[decoderFrameSubKey]*build)
	for _, row := range rows {
		if row.field != "lp_a_q12" || !row.hasValue {
			continue
		}
		if row.index < 0 || row.index > lpcOrder {
			t.Fatalf("lp_a_q12 index out of range: frame=%d sub=%d index=%d", row.frame, row.sub, row.index)
		}
		if row.value < -32768 || row.value > 32767 {
			t.Fatalf("lp_a_q12 value out of int16 range: frame=%d sub=%d index=%d value=%d",
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

	out := make(map[decoderFrameSubKey][lpcOrder + 1]int16)
	for key, b := range builds {
		for i, ok := range b.set {
			if !ok {
				t.Fatalf("incomplete lp_a_q12 override: frame=%d sub=%d missing index=%d", key.frame, key.sub, i)
			}
		}
		out[key] = b.values
	}
	return out
}

func decoderTAMEWideSubframeOverrides(t *testing.T, path, field string) map[decoderFrameSubKey][subframeLen]int16 {
	t.Helper()
	rows, err := readDecoderTAMEStageWideRows(path)
	if err != nil {
		t.Fatalf("read decoder TAME stage wide expected: %v", err)
	}
	type build struct {
		values [subframeLen]int16
		set    [subframeLen]bool
	}
	builds := make(map[decoderFrameSubKey]*build)
	for _, row := range rows {
		if row.field != field || !row.hasValue {
			continue
		}
		if row.index < 0 || row.index >= subframeLen {
			t.Fatalf("%s index out of range: frame=%d sub=%d index=%d", field, row.frame, row.sub, row.index)
		}
		if row.value < -32768 || row.value > 32767 {
			t.Fatalf("%s value out of int16 range: frame=%d sub=%d index=%d value=%d",
				field, row.frame, row.sub, row.index, row.value)
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

	out := make(map[decoderFrameSubKey][subframeLen]int16)
	for key, b := range builds {
		for i, ok := range b.set {
			if !ok {
				t.Fatalf("incomplete %s override: frame=%d sub=%d missing index=%d", field, key.frame, key.sub, i)
			}
		}
		out[key] = b.values
	}
	return out
}

func decodeTAMEWithLPOverrides(t *testing.T, bitData []byte, frames int, overrides map[decoderFrameSubKey][lpcOrder + 1]int16) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[oracle_lp] frame %d: %v", frame, err)
		}
		if err := dec.decodeFrameWithLPOverride(frame, packed[:], out[frame*frameSamples:(frame+1)*frameSamples], overrides); err != nil {
			t.Fatalf("decodeFrameWithLPOverride frame %d: %v", frame, err)
		}
	}
	return out
}

func decodeTAMEWithACBOverrides(t *testing.T, bitData []byte, frames int, overrides map[decoderFrameSubKey][subframeLen]int16) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[oracle_acb] frame %d: %v", frame, err)
		}
		if err := dec.decodeFrameWithACBOverride(frame, packed[:], out[frame*frameSamples:(frame+1)*frameSamples], overrides); err != nil {
			t.Fatalf("decodeFrameWithACBOverride frame %d: %v", frame, err)
		}
	}
	return out
}

func (d *Decoder) decodeFrameWithLPOverride(
	frameIndex int,
	packed []byte,
	out []int16,
	overrides map[decoderFrameSubKey][lpcOrder + 1]int16,
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
	if a, ok := overrides[decoderFrameSubKey{frame: frameIndex, sub: 0}]; ok {
		sf1A = a
	}
	if a, ok := overrides[decoderFrameSubKey{frame: frameIndex, sub: 1}]; ok {
		sf2A = a
	}

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	d.decodeSubframe(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen])
	d.decodeSubframe(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples])
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeFrameWithACBOverride(
	frameIndex int,
	packed []byte,
	out []int16,
	overrides map[decoderFrameSubKey][subframeLen]int16,
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

	d.decodeSubframeWithACBOverride(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1),
		out[:subframeLen], overrides[decoderFrameSubKey{frame: frameIndex, sub: 0}])
	d.decodeSubframeWithACBOverride(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2),
		out[subframeLen:frameSamples], overrides[decoderFrameSubKey{frame: frameIndex, sub: 1}])
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframeWithACBOverride(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	vOverride [subframeLen]int16,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	if vOverride != ([subframeLen]int16{}) {
		v = vOverride
	} else {
		decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)
	}

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.rememberPitchGain(gpQ14)
}
