package gain

// Indices are the gain-VQ bit-field values delivered per subframe
// by the bitstream unpacker. GA and GB are the first- and second-
// stage conjugate-structure VQ indices (ITU-T G.729 §3.9).
type Indices struct {
	GA uint8 // 3 bits — first-stage codebook index (0..7)
	GB uint8 // 4 bits — second-stage codebook index (0..15)
}

// Decoder holds the MA-predictor state used by the gain VQ decoder
// across subframes. The zero value is a valid initial state; the
// first Decode call populates pastErrors with the spec's default.
type Decoder struct {
	pastErrors  [4]int16 // Û(m-1)..Û(m-4), Q10 dB log-gain prediction errors
	initialized bool
}

// Reset returns the Decoder to its zero-value state.
func (d *Decoder) Reset() {
	*d = Decoder{}
}
