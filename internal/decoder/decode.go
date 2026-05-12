package decoder

import (
	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
)

// Decode consumes one packed G.729 / Annex A frame and writes one frame of
// linear PCM to out.
//
// Input contract:
//   - packed must hold at least 10 bytes (80 bits) carrying the 15
//     index fields defined in ITU-T G.729 §4 / §A.4 (encoded MSB-first
//     by the bitstream package).
//   - bad is the frame-erasure marker supplied by the transport layer.
//     The current implementation treats it as a no-op; full concealment
//     per G.729 §4.4 / §A.4.1 is not yet wired in.
//   - out must hold at least 80 int16 samples; only out[0:80] is written
//     (Q0 samples post-HP-filter and final amplitude recovery).
//
// Errors:
//   - ErrShortInput  — len(packed) < 10.
//   - ErrShortOutput — len(out)    < 80.
//   - Errors propagated from bitstream.Unpack (e.g. parity / range
//     violations on the 15 index fields).
//
// Decode advances the Decoder's internal state (LSP MA predictor, gain
// MA predictor, synthesizer memory, postfilter memory, HP-filter memory,
// and the past-excitation FIFO). Callers must invoke Decode strictly in
// frame order on a single Decoder; concurrent calls are not supported.
//
// Decode never panics and performs no heap allocations on the steady-state
// path.
//
// Conformance caveat: this decoder is not currently claimed to be ITU-vector
// byte-exact. Use TestDecoderITUVectorValidation for the current sample-level
// matrix against the Annex A .BIT/.PST vectors. Bad-frame concealment and
// parity handling are also not fully wired yet.
func (d *Decoder) Decode(packed []byte, bad bool, out []int16) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}
	_ = bad // Phase 1g ignores; Phase 1h implements concealment.

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0),
		L1: uint8(f.L1),
		L2: uint8(f.L2),
		L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))

	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	d.decodeSubframe(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen])
	d.decodeSubframe(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples])

	scaleDecoderOutput(out[:frameSamples])

	return nil
}

func scaleDecoderOutput(out []int16) {
	pcm.ScaleUpSat(out, out)
}

func scaleDecoderOutputForEnvelopeRecovery(out []int16) {
	scaleDecoderOutput(out)
	pcm.ScaleUpSat(out, out)
}
