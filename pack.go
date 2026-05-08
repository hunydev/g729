package g729

import "github.com/hunydev/g729/internal/bitstream"

// buildBitstreamFrame composes the 15 per-frame G.729 transmission
// indices currently held on the Encoder into out.
//
// Field mapping per §4.2.1 + Table 8:
//
//	L0,L1,L2,L3 — LSP VQ indices written by lpcStep
//	P1,P0,P2    — pitch indices written by closedloopStep
//	C1/S1/GA1/GB1, C2/S2/GA2/GB2 — FCB + gain indices written by
//	    the per-subframe FCB/gain step
//
// Pure copy; never allocates. The widths declared on bitstream.Frame
// match the encoder field widths (uint16 ⊇ uint8), so this is a
// straight zero-extension.
func (e *Encoder) buildBitstreamFrame(out *bitstream.Frame) {
	out.L0 = e.l0
	out.L1 = e.l1
	out.L2 = e.l2
	out.L3 = e.l3
	out.P1 = uint16(e.p1)
	out.P0 = uint16(e.p0)
	out.C1 = e.c1
	out.S1 = uint16(e.s1)
	out.GA1 = uint16(e.ga1)
	out.GB1 = uint16(e.gb1)
	out.P2 = uint16(e.p2)
	out.C2 = e.c2
	out.S2 = uint16(e.s2)
	out.GA2 = uint16(e.ga2)
	out.GB2 = uint16(e.gb2)
}
