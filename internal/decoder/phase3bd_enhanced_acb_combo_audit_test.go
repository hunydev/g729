package decoder

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3bdEnhancedACBComboAudit checks whether the fractional ACB variants
// rejected on the strict decoder path become useful after the opt-in enhanced
// envelope recovery stage. FFmpeg is used only as an executable black-box
// decoder.
func TestPhase3bdEnhancedACBComboAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_ENHANCED_ACB_COMBO_AUDIT") != "1" {
		t.Skip("set G729_DECODER_ENHANCED_ACB_COMBO_AUDIT=1 to run enhanced ACB combo audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	datasets := []phase3bdACBDataset{
		phase3bdLoadG192ACBDataset(t, "SPEECH.BIT"),
	}
	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(rawPath); err == nil {
		datasets = append(datasets, phase3bdLoadRawACBDataset(t, "Asterisk", rawPath))
	} else {
		t.Logf("Asterisk enhanced ACB combo audit skipped: %v", err)
	}

	variants := phase3pACBVariants()
	for _, ds := range datasets {
		prod := phase3bdDecodeRawEnhancedACB(t, ds.raw, ds.frames, variants[0])
		base := blackboxMeasure(ds.ref, prod, 40)
		t.Logf("Phase 3bd enhanced ACB combo audit - %s (%d frames)", ds.label, ds.frames)
		t.Logf("baseline enhanced production ACB: gSNR=%.2f seg=%.2f corr=%.3f rms=%.1f",
			base.globalSNR, base.segSNR, base.corr, base.rms)
		t.Logf("%-28s %9s %9s %8s %9s %9s",
			"variant", "gSNR", "segSNR", "corr", "rms", "deltaG")
		best := phase3bcFilterRow{name: "production", m: base}
		for _, v := range variants {
			out := prod
			if v.mode != phase3hACBProduction {
				out = phase3bdDecodeRawEnhancedACB(t, ds.raw, ds.frames, v)
			}
			m := blackboxMeasure(ds.ref, out, 40)
			if m.globalSNR > best.m.globalSNR {
				best = phase3bcFilterRow{name: v.name, m: m}
			}
			t.Logf("%-28s %9.2f %9.2f %8.3f %9.1f %+9.2f",
				v.name, m.globalSNR, m.segSNR, m.corr, m.rms, m.globalSNR-base.globalSNR)
		}
		t.Logf("best by global SNR: %s %.2f dB (delta=%+.2f)",
			best.name, best.m.globalSNR, best.m.globalSNR-base.globalSNR)
	}
}

type phase3bdACBDataset struct {
	label  string
	raw    []byte
	frames int
	ref    []int16
}

func phase3bdLoadG192ACBDataset(t *testing.T, name string) phase3bdACBDataset {
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
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read converted raw %s: %v", rawPath, err)
	}
	return phase3bdACBDataset{
		label:  name,
		raw:    raw,
		frames: frames,
		ref:    phase3uFFmpegDecodeG192(t, data, frames, name),
	}
}

func phase3bdLoadRawACBDataset(t *testing.T, label, rawPath string) phase3bdACBDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	return phase3bdACBDataset{
		label:  label,
		raw:    raw,
		frames: frames,
		ref:    phase3uFFmpegDecodeRaw(t, rawPath, frames, label),
	}
}

func phase3bdDecodeRawEnhancedACB(t *testing.T, raw []byte, frames int, variant phase3hVariant) []int16 {
	t.Helper()
	if len(raw) < frames*bitstream.FrameBytes {
		t.Fatalf("raw g729 payload too short: got %d bytes, want %d", len(raw), frames*bitstream.FrameBytes)
	}
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		start := frame * bitstream.FrameBytes
		if err := dec.decodeFramePhase3bdEnhancedACB(raw[start:start+bitstream.FrameBytes], out[frame*frameSamples:(frame+1)*frameSamples], variant); err != nil {
			t.Fatalf("decodeFramePhase3bdEnhancedACB[%s] frame %d: %v", variant.name, frame, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3bdEnhancedACB(packed []byte, out []int16, variant phase3hVariant) error {
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

	var stats envelopeRecoveryStats
	stats.hasGA036 = envelopeRecoveryHasGA036(uint8(fr.GA1)) || envelopeRecoveryHasGA036(uint8(fr.GA2))
	d.decodeSubframePhase3bdEnhancedACB(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], &stats, variant)
	d.decodeSubframePhase3bdEnhancedACB(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], &stats, variant)

	scaleDecoderOutputForEnvelopeRecovery(out[:frameSamples])
	applyEnvelopeRecovery(out[:frameSamples], &stats)
	return nil
}

func (d *Decoder) decodeSubframePhase3bdEnhancedACB(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA, GB uint8,
	out []int16,
	stats *envelopeRecoveryStats,
	variant phase3hVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	phase3hAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v, variant.mode)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := d.gn.DecodeWithLogCorrections(gain.Indices{GA: GA, GB: GB}, &c, 26, 14)
	gcSigned := phase3bdSignedGain(gcMant, gcExp)
	gcAbs := absFloat(gcSigned)
	if gcAbs > stats.gcMax {
		stats.gcMax = gcAbs
	}

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)
	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)
	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	gp := float64(gpQ14) / 16384.0
	for n := 0; n < subframeLen; n++ {
		pitchPart := gp * float64(v[n])
		fixedPart := gcSigned * float64(c[n]) / 8192.0
		stats.pitchE += pitchPart * pitchPart
		stats.fixedE += fixedPart * fixedPart
		stats.uE += float64(u[n]) * float64(u[n])
		stats.sE += float64(s[n]) * float64(s[n])
	}

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func phase3bdSignedGain(mant int16, exp int8) float64 {
	return float64(mant) * math.Exp2(float64(exp)-14)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
