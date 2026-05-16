package g729

import (
	"math"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	pitchcore "github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

func TestEncoderReceiverMirrorStateMatchesEmittedBitstream(t *testing.T) {
	pcm := receiverMirrorTestPCM(320)
	enc := NewEncoder()
	var mirror encoderReceiverMirror

	for frame := 0; frame < len(pcm)/FrameSamples; frame++ {
		in := pcm[frame*FrameSamples : (frame+1)*FrameSamples]
		var packed [FrameBytes]byte
		if err := enc.EncodeFrame(in, packed[:]); err != nil {
			t.Fatalf("EncodeFrame frame %d: %v", frame, err)
		}

		var bf bitstream.Frame
		if err := bitstream.Unpack(packed[:], &bf); err != nil {
			t.Fatalf("Unpack encoded frame %d: %v", frame, err)
		}
		assertEncoderPackedFields(t, frame, enc, bf)

		taps := mirror.decodeFrame(t, packed[:])

		if enc.aHatSF1 != taps.a1 {
			t.Fatalf("frame %d sub0 receiver LP mirror mismatch: encoder=%v decoder=%v", frame, enc.aHatSF1, taps.a1)
		}
		if enc.aHatSF2 != taps.a2 {
			t.Fatalf("frame %d sub1 receiver LP mirror mismatch: encoder=%v decoder=%v", frame, enc.aHatSF2, taps.a2)
		}
		if int(enc.intT1) != taps.t1 || int(enc.frac1) != taps.frac1 {
			t.Fatalf("frame %d sub0 pitch mirror mismatch: encoder=(%d,%d) decoder=(%d,%d)",
				frame, enc.intT1, enc.frac1, taps.t1, taps.frac1)
		}
		if int(enc.intT2) != taps.t2 || int(enc.frac2) != taps.frac2 {
			t.Fatalf("frame %d sub1 pitch mirror mismatch: encoder=(%d,%d) decoder=(%d,%d)",
				frame, enc.intT2, enc.frac2, taps.t2, taps.frac2)
		}
		if enc.prevGpQ14 != mirror.prevGpQ14 {
			t.Fatalf("frame %d previous pitch gain mismatch: encoder=%d decoder=%d",
				frame, enc.prevGpQ14, mirror.prevGpQ14)
		}
		if enc.pastQuaEn != mirror.gain.PredictorErrors() {
			t.Fatalf("frame %d gain predictor mirror mismatch: encoder=%v decoder=%v",
				frame, enc.pastQuaEn, mirror.gain.PredictorErrors())
		}
		gotPastExc := encoderPastExcReceiverWindow(enc)
		if gotPastExc != mirror.pastExc {
			t.Fatalf("frame %d past excitation mirror mismatch: encoder tail=%v decoder=%v",
				frame, gotPastExc, mirror.pastExc)
		}
	}
}

func receiverMirrorTestPCM(frames int) []int16 {
	out := make([]int16, frames*FrameSamples)
	for i := range out {
		t := float64(i) / float64(SampleRate)
		voice := 1200*math.Sin(2*math.Pi*180*t) +
			650*math.Sin(2*math.Pi*360*t+0.4) +
			300*math.Sin(2*math.Pi*720*t+1.1)
		// Add a deterministic low-amplitude non-periodic component so the
		// encoder search sees more than a pure harmonic tone.
		dither := float64(((i*1103515245 + 12345) >> 16) & 0x7f)
		out[i] = int16(voice + dither - 64)
	}
	return out
}

func assertEncoderPackedFields(t *testing.T, frame int, enc *Encoder, bf bitstream.Frame) {
	t.Helper()
	check := func(name string, got, want uint16) {
		if got != want {
			t.Fatalf("frame %d %s packed field mismatch: bitstream=%d encoder=%d", frame, name, got, want)
		}
	}
	check("L0", bf.L0, enc.l0)
	check("L1", bf.L1, enc.l1)
	check("L2", bf.L2, enc.l2)
	check("L3", bf.L3, enc.l3)
	check("P1", bf.P1, uint16(enc.p1))
	check("P0", bf.P0, uint16(enc.p0))
	check("P2", bf.P2, uint16(enc.p2))
	check("C1", bf.C1, enc.c1)
	check("S1", bf.S1, uint16(enc.s1))
	check("GA1", bf.GA1, uint16(enc.ga1))
	check("GB1", bf.GB1, uint16(enc.gb1))
	check("C2", bf.C2, enc.c2)
	check("S2", bf.S2, uint16(enc.s2))
	check("GA2", bf.GA2, uint16(enc.ga2))
	check("GB2", bf.GB2, uint16(enc.gb2))
}

func encoderPastExcReceiverWindow(enc *Encoder) [153]int16 {
	var out [153]int16
	copy(out[:], enc.oldExc[len(enc.oldExc)-len(out):])
	return out
}

type encoderReceiverMirror struct {
	lsp        lsp.Decoder
	gain       gain.Decoder
	synth      synth.Synthesizer
	pastExc    [153]int16
	prevGpQ14  int16
	havePrevGp bool
}

type encoderReceiverMirrorFrame struct {
	a1, a2       [11]int16
	t1, t2       int
	frac1, frac2 int
}

func (m *encoderReceiverMirror) decodeFrame(t *testing.T, packed []byte) encoderReceiverMirrorFrame {
	t.Helper()
	var bf bitstream.Frame
	if err := bitstream.Unpack(packed, &bf); err != nil {
		t.Fatalf("receiver mirror unpack: %v", err)
	}

	a1, a2 := m.lsp.Decode(lsp.Indices{
		L0: uint8(bf.L0),
		L1: uint8(bf.L1),
		L2: uint8(bf.L2),
		L3: uint8(bf.L3),
	})

	t1, frac1 := pitchcore.DecodeDelaySubframe1(uint8(bf.P1))
	if !pitchcore.CheckParity(uint8(bf.P1), uint8(bf.P0)) {
		t.Fatalf("receiver mirror parity mismatch for P1=%d P0=%d", bf.P1, bf.P0)
	}
	t2, frac2 := pitchcore.DecodeDelaySubframe2(uint8(bf.P2), t1)

	m.decodeSubframe(&a1, t1, frac1, bf.C1, uint8(bf.S1), uint8(bf.GA1), uint8(bf.GB1))
	m.decodeSubframe(&a2, t2, frac2, bf.C2, uint8(bf.S2), uint8(bf.GA2), uint8(bf.GB2))

	return encoderReceiverMirrorFrame{
		a1: a1, a2: a2,
		t1: t1, t2: t2,
		frac1: frac1, frac2: frac2,
	}
}

func (m *encoderReceiverMirror) decodeSubframe(a *[11]int16, tInt, tFrac int, cIdx uint16, signs, ga, gb uint8) {
	beta := int16(fcb.InitialPitchEnhancementQ14)
	if m.havePrevGp {
		beta = fcb.ClampPitchGainForEnhancement(m.prevGpQ14)
	}

	var v [40]int16
	pitchcore.AdaptiveCodebook(tInt, tFrac, m.pastExc[:], &v)

	var c [40]int16
	fcb.Decode(fcb.Indices{Positions: cIdx, Signs: signs}, tInt, beta, &c)

	gpQ14, gcMantQ14, gcExp := m.gain.Decode(gain.Indices{GA: ga, GB: gb}, &c)
	var u [40]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	var s [40]int16
	m.synth.Filter(a, &u, &s)
	commitU := u
	if shift := m.synth.LastExcitationScaleShift(); shift > 0 {
		for i, sample := range m.pastExc {
			m.pastExc[i] = int16(int32(sample) >> shift)
		}
		for i, sample := range commitU {
			commitU[i] = int16(int32(sample) >> shift)
		}
	}

	copy(m.pastExc[:len(m.pastExc)-FrameSamples/2], m.pastExc[FrameSamples/2:])
	copy(m.pastExc[len(m.pastExc)-FrameSamples/2:], commitU[:])
	m.prevGpQ14 = gpQ14
	m.havePrevGp = true
}
