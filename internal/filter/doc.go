// Package filter implements the encoder-side perceptual weighting
// filter W(z) = A(z/γ1) / A(z/γ2) (§3.3) and the impulse response
// computation h[] used by the ACELP target derivation (§3.7-3.8).
//
// The synthesis filter 1/Â(z) used by the *decoder* lives in
// internal/synth — that distinction is intentional: the encoder's
// W(z) needs both numerator and denominator coefficients, while the
// decoder's 1/Â(z) is denominator-only.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2c (target computation) and Phase 2d (impulse response).
package filter
