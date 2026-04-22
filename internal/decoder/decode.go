package decoder

import (
	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/pitch"
)

// Decode consumes one packed G.729 frame (10 bytes) and writes 80 PCM
// samples at Q0 full amplitude to out[0:80].
//
// bad: frame-erasure marker from the transport layer. Phase 1g treats
// it as a no-op. Phase 1h will add concealment per ITU-T G.729 §A.4.1.
//
// Returns ErrShortInput if len(packed) < 10 or ErrShortOutput if
// len(out) < 80. Never panics; never allocates on the heap.
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

	pcm.ScaleUpSat(out[:frameSamples], out[:frameSamples])

	return nil
}
