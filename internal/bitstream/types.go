package bitstream

// Frame format constants.
const (
	// FrameBits is the number of bits per G.729 frame on the wire.
	FrameBits = 80
	// FrameBytes is the number of bytes per packed G.729 frame.
	FrameBytes = FrameBits / 8
)

// Frame is one decoded G.729 frame's transmitted parameters. Field order
// matches the ITU-T G.729 transmission order (Annex A Table 8). All fields
// hold unsigned integer indices; the declared range of each is given in
// the doc comment and enforced implicitly by the bit width in Pack.
type Frame struct {
	L0  uint16 // 1 bit  — LSP MA predictor switch
	L1  uint16 // 7 bits — LSP 1st stage codebook index
	L2  uint16 // 5 bits — LSP 2nd stage lower split
	L3  uint16 // 5 bits — LSP 2nd stage upper split
	P1  uint16 // 8 bits — pitch delay subframe 1
	P0  uint16 // 1 bit  — parity of upper 6 bits of P1
	C1  uint16 // 13 bits — ACELP positions subframe 1
	S1  uint16 // 4 bits — ACELP signs subframe 1
	GA1 uint16 // 3 bits — gain stage 1 subframe 1
	GB1 uint16 // 4 bits — gain stage 2 subframe 1
	P2  uint16 // 5 bits — pitch delay subframe 2
	C2  uint16 // 13 bits — ACELP positions subframe 2
	S2  uint16 // 4 bits — ACELP signs subframe 2
	GA2 uint16 // 3 bits — gain stage 1 subframe 2
	GB2 uint16 // 4 bits — gain stage 2 subframe 2
}
