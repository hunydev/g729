package lpc

import "errors"

// LPCOrder is the G.729 LP analysis order (§3.2): a[0]=1, a[1..10].
const LPCOrder = 10

// LPCWindowSamples is the 30 ms asymmetric Hamming window length
// (§3.2.1): 240 samples = 80 future-lookahead + 160 past.
const LPCWindowSamples = 240

// errStub is returned by every Phase 2-0 stub method. Replaced per
// sub-phase when real arithmetic lands.
var errStub = errors.New("internal/lpc: not yet implemented")

// Analyzer holds frame-to-frame analysis state.
type Analyzer struct {
	// Phase 2a will populate. Empty by design at 2-0.
}

// Reset returns the analyzer to its zero state.
func (a *Analyzer) Reset() { *a = Analyzer{} }

// Analyze produces order-10 LPC coefficients from a windowed speech
// buffer. Phase 2-0 returns a sentinel; Phase 2a wires the real
// autocorrelation + Levinson recursion.
func (a *Analyzer) Analyze(speech []int16, out []int16) error {
	return errStub
}
