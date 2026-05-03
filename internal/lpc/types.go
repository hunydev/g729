package lpc

// LPCOrder is the G.729 LP analysis order (§3.2): a[0]=1, a[1..10].
const LPCOrder = 10

// LPCWindowSamples is the 30 ms asymmetric Hamming window length
// (§3.2.1): 240 samples = 80 future-lookahead + 160 past.
const LPCWindowSamples = 240

// Analyzer holds frame-to-frame analysis state for the LP chain.
//
// At Phase 2a all per-frame scratch lives on the stack inside
// Analyze; the type currently carries no fields, but exists so the
// public surface can grow (e.g. AC scaling history) without an API
// break.
type Analyzer struct{}

// Reset returns the analyzer to its zero state.
func (a *Analyzer) Reset() { *a = Analyzer{} }

// Analyze runs the §3.2.1–§3.2.2 LP analysis chain on a 240-sample
// windowed-input speech buffer and writes the order-10 LP filter
// a[0..10] (Q12, with a[0] = 4096) into out.
//
// Pipeline (matches the spec linearly):
//
//	§3.2.1 eq. 4 :  windowSpeech         — apply w_lp(n) to s(n)
//	§3.2.1 eq. 5 :  autocorrelate        — r(0..10), shared scale
//	§3.2.1 eq. 6 :  applyLagWindow       — 60 Hz BW expansion + r'(0)·1.0001
//	§3.2.2       :  levinsonDurbin       — solve for a[1..10] in Q12
//
// I3 / I4: pure (apart from writing through out), zero allocation;
// all intermediates live in stack arrays.
func (a *Analyzer) Analyze(speech *[LPCWindowSamples]int16, out *[LPCOrder + 1]int16) error {
	var (
		windowed [LPCWindowSamples]int16
		r        [LPCOrder + 1]int32
	)
	windowSpeech(speech, &windowed)
	_ = autocorrelate(&windowed, &r)
	applyLagWindow(&r)
	levinsonDurbin(&r, out)
	return nil
}

