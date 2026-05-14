package decoder

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3eUpstreamVariantLocalization_SPEECH pushes the Phase 3d verdict
// one layer upstream. It runs whole-pipeline perturbations that selectively
// remove or scale adaptive-codebook, fixed-codebook, gain, pitch-fraction, and
// synthesis/state surfaces, then compares final PCM against SPEECH.PST.
//
// This is intentionally opt-in and informational. The variants are diagnostic
// probes, not production candidates.
func TestPhase3eUpstreamVariantLocalization_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_UPSTREAM_LOCALIZE") != "1" {
		t.Skip("set G729_DECODER_UPSTREAM_LOCALIZE=1 to run upstream decoder localization")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_UPSTREAM_VECTOR", "SPEECH")
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
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)

	productionViaVariant := phase3eDecodeVariant(t, bitData, frames, phase3eVariant{name: "production_variant"})
	productionViaDecode := decodeVariant(t, bitData, frames, nil, nil)
	if !phase3eEqualPCM(productionViaVariant, productionViaDecode) {
		t.Fatalf("phase3e production_variant diverges from Decoder.Decode baseline")
	}
	prodMetrics := blackboxMeasure(ref, productionViaVariant, 40)

	variants := []phase3eVariant{
		{name: "production"},
		{name: "zero_adaptive_contribution", zeroAdaptive: true},
		{name: "zero_fixed_contribution", zeroFixed: true},
		{name: "no_fcb_pitch_enhancement", noFCBEnhancement: true},
		{name: "gain_unenhanced_c", gainUnenhancedC: true},
		{name: "pitch_gain_tame_energy", pitchTameEnergy: true},
		{name: "pitch_gain_cap_0p95", pitchCapQ14: 15565},
		{name: "pitch_gain_cap_0p90", pitchCapQ14: 14746},
		{name: "pitch_gain_half", pitchScaleNum: 1, pitchScaleDen: 2},
		{name: "pitch_gain_double", pitchScaleNum: 2, pitchScaleDen: 1},
		{name: "fixed_gain_half", fixedExpDelta: -1},
		{name: "fixed_gain_double", fixedExpDelta: +1},
		{name: "fixed_gain_quad", fixedExpDelta: +2},
		{name: "fractional_acb_interpolation", useFractionalACB: true},
		{name: "force_pitch_frac_zero", forceTFracZero: true},
		{name: "flip_pitch_frac_sign", flipTFracSign: true},
		{name: "synth_identity_hp_x2", synthMode: phase3eSynthIdentityHP},
		{name: "synth_identity_pf_hp_x2", synthMode: phase3eSynthIdentityPFHP},
		{name: "reset_synth_each_frame", resetSynthEachFrame: true},
		{name: "reset_past_exc_each_frame", resetPastExcEachFrame: true},
		{name: "reset_gain_each_frame", resetGainEachFrame: true},
		{name: "reset_lsp_each_frame", resetLSPEachFrame: true},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3e upstream variant localization — %s/%s (%d frames)", tc.bitFile, tc.pstFile, frames)
	t.Logf("baseline production: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR)
	t.Logf("")
	t.Logf("%-30s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "lagSNR", "bestSNR")
	t.Logf("%-30s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "------", "-------")
	for _, v := range variants {
		out := productionViaVariant
		if v.name != "production" {
			out = phase3eDecodeVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		rows = append(rows, row{name: v.name, m: m})
		t.Logf("%-30s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %10d %10.2f",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			m.globalSNR-prodMetrics.globalSNR, m.corr-prodMetrics.corr,
			m.bestSNRLag, m.bestSNR)
	}

	best := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
	}
	bestCorr := rows[0]
	for _, r := range rows[1:] {
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
	}
	t.Logf("")
	t.Logf("best by gSNR@0: %s %.2f dB (Δ=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (Δ=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("verdict: %s", phase3eVerdict(prodMetrics, best, bestCorr))
}

func phase3eSelectedITUVector(t *testing.T, envName, defaultName string) decoderITUValidationCase {
	t.Helper()
	name := strings.TrimSpace(os.Getenv(envName))
	if name == "" {
		name = defaultName
	}
	tc, ok := decoderITUValidationCaseByName(name)
	if !ok {
		t.Fatalf("unknown decoder ITU vector %q", name)
	}
	return tc
}

type phase3eSynthMode int

const (
	phase3eSynthProduction phase3eSynthMode = iota
	phase3eSynthIdentityHP
	phase3eSynthIdentityPFHP
)

type phase3eVariant struct {
	name                  string
	zeroAdaptive          bool
	zeroFixed             bool
	noFCBEnhancement      bool
	gainUnenhancedC       bool
	pitchTameEnergy       bool
	pitchCapQ14           int16
	pitchScaleNum         int
	pitchScaleDen         int
	fixedExpDelta         int
	forceTFracZero        bool
	flipTFracSign         bool
	useFractionalACB      bool
	synthMode             phase3eSynthMode
	resetSynthEachFrame   bool
	resetPastExcEachFrame bool
	resetGainEachFrame    bool
	resetLSPEachFrame     bool
}

func phase3eDecodeVariant(t *testing.T, bitData []byte, frames int, variant phase3eVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if variant.resetSynthEachFrame {
			dec.syn.Reset()
		}
		if variant.resetPastExcEachFrame {
			dec.pastExc = [pastExcLen]int16{}
			dec.prevGpQ14 = 0
			dec.havePrevGpQ14 = false
		}
		if variant.resetGainEachFrame {
			dec.gn.Reset()
		}
		if variant.resetLSPEachFrame {
			dec.lsp.Reset()
		}
		if err := dec.decodeFramePhase3eVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3eVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3eVariant(packed []byte, out []int16, variant phase3eVariant) error {
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
	tFrac1 = phase3eAdjustTFrac(tFrac1, variant)
	tFrac2 = phase3eAdjustTFrac(tFrac2, variant)

	d.decodeSubframePhase3eVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], variant)
	d.decodeSubframePhase3eVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], variant)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3eVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	variant phase3eVariant,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()
	if variant.noFCBEnhancement {
		betaQ14 = 0
	}

	var v [subframeLen]int16
	if variant.useFractionalACB {
		pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)
	} else {
		decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)
	}

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gainC := &c
	if variant.gainUnenhancedC {
		var cRaw [subframeLen]int16
		fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, 0, &cRaw)
		gainC = &cRaw
	}

	gainTaps := d.gn.DecodeWithFullTaps(gain.Indices{GA: GA, GB: GB}, gainC)
	gpQ14 := gainTaps.GpQ14Final
	gcMantQ14 := gainTaps.GcMantQ14
	gcExp := gainTaps.GcExp
	if variant.zeroAdaptive {
		gpQ14 = 0
	}
	if variant.pitchTameEnergy &&
		phase3ePastExcEnergy(d.pastExc[:]) > gainquant.TameEnergyThresholdQ0 &&
		gpQ14 > gainquant.GpClipQ14 {
		gpQ14 = gainquant.GpClipQ14
	}
	if variant.pitchCapQ14 > 0 && gpQ14 > variant.pitchCapQ14 {
		gpQ14 = variant.pitchCapQ14
	}
	if variant.pitchScaleNum != 0 && variant.pitchScaleDen != 0 {
		gpQ14 = phase3eScaleWord16(gpQ14, variant.pitchScaleNum, variant.pitchScaleDen)
	}
	if variant.zeroFixed {
		gcMantQ14 = 0
		gcExp = 0
	}
	if variant.fixedExpDelta != 0 && gcMantQ14 != 0 {
		gcExp = phase3eClampExp(int(gcExp) + variant.fixedExpDelta)
	}

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var hpOut [subframeLen]int16
	switch variant.synthMode {
	case phase3eSynthIdentityHP:
		d.hpFilter(&u, hpOut[:])
	case phase3eSynthIdentityPFHP:
		var sPf [subframeLen]int16
		d.pst.Filter(sfA, tInt, &u, &sPf)
		d.hpFilter(&sPf, hpOut[:])
	default:
		var s [subframeLen]int16
		d.syn.Filter(sfA, &u, &s)
		var sPf [subframeLen]int16
		d.pst.Filter(sfA, tInt, &s, &sPf)
		d.hpFilter(&sPf, hpOut[:])
	}
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	// Keep the pitch-enhancement state tied to the decoded gain so
	// current-subframe perturbations do not double-count the beta path.
	d.rememberPitchGain(gainTaps.GpQ14Final)
}

func phase3eAdjustTFrac(tFrac int, variant phase3eVariant) int {
	if variant.forceTFracZero {
		return 0
	}
	if variant.flipTFracSign {
		return -tFrac
	}
	return tFrac
}

func phase3eScaleWord16(v int16, num, den int) int16 {
	if den == 0 {
		return v
	}
	x := int64(v) * int64(num)
	if x >= 0 {
		x = (x + int64(den)/2) / int64(den)
	} else {
		x = (x - int64(den)/2) / int64(den)
	}
	if x > 32767 {
		x = 32767
	} else if x < -32768 {
		x = -32768
	}
	return int16(x)
}

func phase3ePastExcEnergy(samples []int16) int64 {
	var energy int64
	for _, sample := range samples {
		s := int64(sample)
		energy += s * s
	}
	return energy
}

func phase3eClampExp(v int) int8 {
	if v > 127 {
		return 127
	}
	if v < -128 {
		return -128
	}
	return int8(v)
}

func phase3eEqualPCM(a, b []int16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func phase3eVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.m.corr-prod.corr > 0.05 {
		return "variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no simple component removal/scale/state reset improves final PCM; likely a numeric reconstruction defect within pitch/gain/FCB/synthesis rather than a missing whole-stage bypass"
}
