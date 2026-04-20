package bitstream

// Pack serializes f into the first FrameBytes bytes of out. The bytes
// are cleared first, so pre-existing content in out[:FrameBytes] is
// overwritten.
//
// Returns ErrShortOutput if len(out) < FrameBytes. Never allocates.
func Pack(f *Frame, out []byte) error {
	if len(out) < FrameBytes {
		return ErrShortOutput
	}
	buf := out[:FrameBytes]
	for i := range buf {
		buf[i] = 0
	}

	var w BitWriter
	w.Init(buf)
	w.Write(f.L0, 1)
	w.Write(f.L1, 7)
	w.Write(f.L2, 5)
	w.Write(f.L3, 5)
	w.Write(f.P1, 8)
	w.Write(f.P0, 1)
	w.Write(f.C1, 13)
	w.Write(f.S1, 4)
	w.Write(f.GA1, 3)
	w.Write(f.GB1, 4)
	w.Write(f.P2, 5)
	w.Write(f.C2, 13)
	w.Write(f.S2, 4)
	w.Write(f.GA2, 3)
	w.Write(f.GB2, 4)

	return nil
}
