package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3aeGainGammaClusterAudit scales gammaC for selected transmitted
// GA clusters before fixed-gain reconstruction and predictor update. This
// tests whether the GA-cluster signal belongs in gain magnitude rather than
// post-gain excitation scaling.
func TestPhase3aeGainGammaClusterAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_GAMMA_CLUSTER_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_GAMMA_CLUSTER_AUDIT=1 to run gain gamma-cluster audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	bitPath := vectorPath("SPEECH.BIT")
	ensureTestdataPresent(t, bitPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	speechFrames := len(bitData) / bitstream.G192FrameBytes
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	phase3aeReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk gain gamma-cluster audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3aeReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3aeGainDecoder struct {
	initialized bool
	pastErrors  [4]int16
}

func phase3aeReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	baseOut := decodeVariant(t, bitData, frames, nil, nil)
	phase3aeReport(t, label, ref, baseOut, func(s phase3adClusterScale) []int16 {
		return phase3aeDecodeG192Scale(t, bitData, frames, s)
	})
}

func phase3aeReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	baseOut := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	phase3aeReport(t, label, ref, baseOut, func(s phase3adClusterScale) []int16 {
		return phase3aeDecodeRawScale(t, raw, frames, s)
	})
}

func phase3aeReport(t *testing.T, label string, ref, baseOut []int16, decode func(phase3adClusterScale) []int16) {
	t.Helper()
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)
	rows := make([]phase3xRow, 0, len(phase3adScales()))

	t.Logf("Phase 3ae gain gamma-cluster audit - %s", label)
	t.Logf("%-12s %9s %7s %10s %10s %8s %9s %9s %9s",
		"scale", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-----", "---", "----", "------", "-----", "------", "--------", "-------", "-------")
	for _, s := range phase3adScales() {
		out := baseOut
		if s.name != "production" {
			out = decode(s)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: s.name, m: m, env: env})
		t.Logf("%-12s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			s.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
	}
	phase3adLogBest(t, phase3xRow{name: "production", m: base, env: baseEnv}, rows)
}

func phase3aeDecodeG192Scale(t *testing.T, bitData []byte, frames int, s phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aeGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", s.name, f, err)
		}
		if err := dec.decodeFramePhase3aeScale(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, s); err != nil {
			t.Fatalf("decodeFramePhase3aeScale[%s] frame %d: %v", s.name, f, err)
		}
	}
	return out
}

func phase3aeDecodeRawScale(t *testing.T, raw []byte, frames int, s phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aeGainDecoder
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3aeScale(packed, out[f*frameSamples:(f+1)*frameSamples], &gd, s); err != nil {
			t.Fatalf("decodeFramePhase3aeScale[%s] frame %d: %v", s.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3aeScale(packed []byte, out []int16, gd *phase3aeGainDecoder, s phase3adClusterScale) error {
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

	d.decodeSubframePhase3aeScale(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], gd, s)
	d.decodeSubframePhase3aeScale(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], gd, s)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3aeScale(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	gd *phase3aeGainDecoder,
	s phase3adClusterScale,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := gd.decode(gain.Indices{GA: GA, GB: GB}, &c, s)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var synthOut [subframeLen]int16
	d.syn.Filter(sfA, &u, &synthOut)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &synthOut, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func (d *phase3aeGainDecoder) decode(idx gain.Indices, c *[subframeLen]int16, scale phase3adClusterScale) (gpQ14, gcMantQ14 int16, gcExp int8) {
	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = gain.PastErrorsDefault
		}
		d.initialized = true
	}

	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		gp, _ := phase3aaDecodeVQ(idx, phase3aaGainMapProduction)
		d.shift(gain.PastErrorsDefault)
		return gp, 0, 0
	}

	predicted := gain.PredictedLogGain(&d.pastErrors)
	ecLog2Q10 := int32(gain.Log2Fixed(ecEnergy)) - 26*1024
	ecDBQ10 := (ecLog2Q10*phase3jDBPerLog2Q13 + (1 << 12)) >> 13
	ecBarDBQ10 := fixed.Saturate(fixed.Word32(ecDBQ10 - phase3jTenLog10_40Q10))
	logGainDBQ10 := predicted - int32(ecBarDBQ10)
	log2GcQ10 := (logGainDBQ10*phase3jInvDBScaleQ15 + (1 << 14)) >> 15

	gp, gammaC := phase3aaDecodeVQ(idx, phase3aaGainMapProduction)
	if scale.mask(idx.GA) {
		gammaC = phase3aeScaleWord16(gammaC, scale.num, scale.den)
	}
	gpQ14 = gp
	if gammaC > 0 {
		gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaC))) - 13*1024
		log2GcWithGammaQ10 := log2GcQ10 + gammaLog2Q10
		intPart := log2GcWithGammaQ10 >> 10
		frac := log2GcWithGammaQ10 - (intPart << 10)
		gcMantQ14 = gain.Pow2FracQ14(frac)
		switch {
		case intPart > 127:
			gcExp = 127
		case intPart < -128:
			gcExp = -128
		default:
			gcExp = int8(intPart)
		}
	}

	uCurrent := int16(gain.PastErrorsDefault)
	if gammaC > 0 {
		gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaC))) - 13*1024
		val := (gammaLog2Q10*phase3jDBPerLog2Q10 + (1 << 9)) >> 10
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		uCurrent = int16(val)
	}
	d.shift(uCurrent)
	return
}

func (d *phase3aeGainDecoder) shift(v int16) {
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = v
}

func phase3aeScaleWord16(v int16, num, den int) int16 {
	x := int64(v) * int64(num)
	if x >= 0 {
		x += int64(den / 2)
	} else {
		x -= int64(den / 2)
	}
	x /= int64(den)
	if x > 32767 {
		x = 32767
	} else if x < -32768 {
		x = -32768
	}
	return int16(x)
}
