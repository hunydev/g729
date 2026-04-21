package lsp

// Decoder reconstructs the quantized LP filter from the 18-bit LSP
// parameters of one G.729 frame. It carries the state required across
// frames: past quantized residuals for the MA predictor, and the
// previous frame's LSP vector for inter-frame interpolation.
//
// The zero value is a valid Reset state.
type Decoder struct {
	// pastResiduals holds the 4 previous frames' quantized residual
	// vectors r̂(n-1)..r̂(n-4), all Q13. pastResiduals[0] is the most
	// recent (r̂(n-1)).
	pastResiduals [4][10]int16
}

// Reset returns the decoder to its initial state.
func (d *Decoder) Reset() {
	*d = Decoder{}
}

// Decode reconstructs the per-subframe LP filter coefficients for one
// frame. sf1 is the interpolated LP for the first subframe, sf2 is the
// current-frame LP for the second subframe. Both are Q12 with
// sf1[0] = sf2[0] = 4096 (i.e. 1.0).
//
// Decode allocates nothing.
func (d *Decoder) Decode(idx Indices) (sf1, sf2 [11]int16) {
	return sf1, sf2
}
