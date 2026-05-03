package filter

import "errors"

// LPCOrder mirrors lpc.LPCOrder for type-safety inside this package.
const LPCOrder = 10

var errStub = errors.New("internal/filter: not yet implemented")

// Weighting holds the perceptual weighting filter memory (§3.3).
type Weighting struct {
	// Phase 2c will populate (residual + numerator/denominator memory).
}

// Reset returns the weighting filter to zero memory state.
func (w *Weighting) Reset() { *w = Weighting{} }

// Apply runs W(z) = A(z/γ1) / A(z/γ2) on a 40-sample subframe.
// Phase 2-0 stub.
func (w *Weighting) Apply(a, in, out []int16) error {
	return errStub
}
