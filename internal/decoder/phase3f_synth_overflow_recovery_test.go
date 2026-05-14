package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3fSynthOverflowRecoveryAudit_SPEECH audits the §3.10 overflow
// recovery branch in the synthesis filter. Production uses the quarter/x4
// recovery path; the legacy half/x2 and no-recovery paths are kept only as
// diagnostic probes.
func TestPhase3fSynthOverflowRecoveryAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_SYNTH_OVERFLOW_AUDIT") != "1" {
		t.Skip("set G729_DECODER_SYNTH_OVERFLOW_AUDIT=1 to audit synth overflow recovery")
	}

	tc := phase3eSelectedITUVector(t, "G729_DECODER_SYNTH_OVERFLOW_VECTOR", "SPEECH")
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

	production := decodeVariant(t, bitData, frames, nil, nil)
	quarter, quarterStats := phase3fDecodeWithSynthRecovery(t, bitData, frames, 2)
	if !phase3eEqualPCM(production, quarter) {
		t.Fatalf("phase3f quarter-recovery decoder diverges from production Decode")
	}
	half, halfStats := phase3fDecodeWithSynthRecovery(t, bitData, frames, 1)
	none, noneStats := phase3fDecodeWithSynthRecovery(t, bitData, frames, 0)

	variants := []struct {
		name  string
		out   []int16
		stats phase3fSynthStats
	}{
		{name: "production_quarter_x4", out: quarter, stats: quarterStats},
		{name: "legacy_half_x2", out: half, stats: halfStats},
		{name: "no_recovery_pass1_sat", out: none, stats: noneStats},
	}

	prodMetrics := blackboxMeasure(ref, production, 40)
	t.Logf("Phase 3f synth overflow recovery audit — %s/%s (%d frames)", tc.bitFile, tc.pstFile, frames)
	t.Logf("production baseline: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR, prodMetrics.corr)
	t.Logf("")
	t.Logf("%-24s %8s %8s %10s %10s %8s %9s %9s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "pass1Ov", "pass2Ov", "dGsnr", "dCorr", "diffSamp")
	t.Logf("%-24s %8s %8s %10s %10s %8s %9s %9s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "-------", "-------", "-----", "-----", "--------")
	for _, v := range variants {
		m := blackboxMeasure(ref, v.out, 40)
		t.Logf("%-24s %8.2f %8d %10.2f %10.2f %8.3f %9d %9d %9.2f %9.3f %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			v.stats.pass1Overflow, v.stats.pass2Overflow,
			m.globalSNR-prodMetrics.globalSNR, m.corr-prodMetrics.corr,
			phase3fDiffSamples(production, v.out))
	}
	t.Logf("verdict: %s", phase3fOverflowVerdict(quarterStats))
}

type phase3fSynthStats struct {
	pass1Overflow int
	pass2Overflow int
}

type phase3fSynth struct {
	pastSynth [lpcOrder]int16
	stats     phase3fSynthStats
}

func phase3fDecodeWithSynthRecovery(t *testing.T, bitData []byte, frames int, recoveryShift uint) ([]int16, phase3fSynthStats) {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var syn phase3fSynth
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for fr := 0; fr < frames; fr++ {
		if _, err := bitstream.ReadG192FrameLenient(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[phase3f] frame %d: %v", fr, err)
		}
		if err := dec.decodeFramePhase3f(packed[:], out[fr*frameSamples:(fr+1)*frameSamples], &syn, recoveryShift); err != nil {
			t.Fatalf("decodeFramePhase3f frame %d: %v", fr, err)
		}
	}
	return out, syn.stats
}

func (d *Decoder) decodeFramePhase3f(packed []byte, out []int16, syn *phase3fSynth, recoveryShift uint) error {
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

	d.decodeSubframePhase3f(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], syn, recoveryShift)
	d.decodeSubframePhase3f(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], syn, recoveryShift)

	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3f(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
	syn *phase3fSynth,
	recoveryShift uint,
) {
	betaQ14 := d.pitchEnhancementBetaQ14()

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMantQ14, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	syn.filter(sfA, &u, &s, recoveryShift)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.rememberPitchGain(gpQ14)
}

func (s *phase3fSynth) filter(a *[lpcOrder + 1]int16, u, out *[subframeLen]int16, recoveryShift uint) {
	var work [lpcOrder + subframeLen]int16
	copy(work[:lpcOrder], s.pastSynth[:])

	fixed.ClearOverflow()
	phase3fSynthOnePass(a, u, &work)
	if !fixed.Overflow() || recoveryShift == 0 {
		if fixed.Overflow() {
			s.stats.pass1Overflow++
		}
		copy(out[:], work[lpcOrder:])
		copy(s.pastSynth[:], work[subframeLen:])
		return
	}
	s.stats.pass1Overflow++

	var work2 [lpcOrder + subframeLen]int16
	for i, v := range s.pastSynth {
		work2[i] = int16(int32(v) >> recoveryShift)
	}
	var uScaled [subframeLen]int16
	for i, v := range u {
		uScaled[i] = int16(int32(v) >> recoveryShift)
	}

	fixed.ClearOverflow()
	phase3fSynthOnePass(a, &uScaled, &work2)
	if fixed.Overflow() {
		s.stats.pass2Overflow++
	}

	for i := lpcOrder; i < lpcOrder+subframeLen; i++ {
		v := int32(work2[i]) << recoveryShift
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		work2[i] = int16(v)
	}
	copy(out[:], work2[lpcOrder:])
	copy(s.pastSynth[:], work2[subframeLen:])
}

func phase3fSynthOnePass(a *[lpcOrder + 1]int16, u *[subframeLen]int16, work *[lpcOrder + subframeLen]int16) {
	for n := 0; n < subframeLen; n++ {
		lTemp := fixed.LMult(u[n], a[0])
		for i := 1; i <= lpcOrder; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[lpcOrder+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[lpcOrder+n] = fixed.Round(lTemp)
	}
}

func phase3fDiffSamples(a, b []int16) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var diff int
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			diff++
		}
	}
	return diff
}

func phase3fOverflowVerdict(prodStats phase3fSynthStats) string {
	if prodStats.pass1Overflow == 0 {
		return "synth overflow recovery is inactive on the selected vector"
	}
	if prodStats.pass2Overflow > 0 {
		return "quarter recovery still overflows on pass2; need narrower synthesis arithmetic audit"
	}
	return "production quarter recovery is active; if envelope still grows, inspect synthesis input/state rather than toggling recovery scale"
}
