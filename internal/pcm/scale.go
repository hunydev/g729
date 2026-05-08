package pcm

import "github.com/hunydev/g729/internal/fixed"

// ScaleUpSat multiplies each sample in in by 2 with int16 saturation
// and writes the result to out. This is the decoder-side inverse of
// the 1/2 amplitude scaling applied in PreProcessor: the decoder
// synthesizes samples in the halved-amplitude domain, and this final
// step restores the original amplitude, clipping to +/-Max16 where
// the doubling would overflow.
//
// in and out may alias (in-place is safe). Processes
// min(len(in), len(out)) samples. Allocates nothing.
//
// See ITU-T G.729 section 4.2 for the decoder output-scaling
// requirement.
func ScaleUpSat(in, out []int16) {
	n := len(in)
	if len(out) < n {
		n = len(out)
	}
	for i := 0; i < n; i++ {
		out[i] = fixed.Shl(in[i], 1)
	}
}
