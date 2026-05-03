// Package openloop implements the encoder-side §A.3.3 perceptual
// weighting chain and the §A.3.4 decimated three-range open-loop
// pitch search used by the G.729 Annex A reduced-complexity encoder.
//
// The package is structured as a sibling of internal/pitch/ (which
// hosts the decoder-side adaptive-codebook reconstruction) so that
// the encoder-only state and intermediate buffers introduced by
// §A.3.3 / §A.3.4 do not leak into the decoder API surface. A future
// closedloop sibling will host §4.1.x fractional refinement.
//
// # Phase 2b-WS-1 surface
//
//	gammaWeightLP(a, out *[11]int16)
//	    Builds Â(z/γ) for γ = 0.75 from the unquantized 10-th order
//	    LP coefficients â (Q12, a[0] = 4096). Each tap aw[i] is
//	    obtained as fixed.Mult(a[i], gammaPow[i]) where gammaPow is a
//	    static Q15 LUT of γⁱ for i = 0..10.
//
//	combineWith07(aw, out *[11]int16)
//	    Applies the (1 − 0.7z⁻¹) low-pass factor to Â(z/γ) producing
//	    the order-10 representation A'(z) used in §A.3.3 eq. A.2.
//	    The 0.7 constant is the Q15 value 22938.
//
// # Q-format invariants
//
//	a, aw, aPrime : Q12 with leading tap = 4096 (ITU LP convention).
//	gammaPow      : Q15 LUT of γⁱ; gammaPow[0] = 32767, gammaPow[1]
//	                = 24576 (= 0.75·32768), then 0.75ⁱ rounded to
//	                nearest int16. The recurrence
//	                gammaPow[i+1] = MultR(gammaPow[i], 24576) holds
//	                modulo half-LSB rounding.
//	gamma07Q15    : 22938 = round(0.7·32768).
//
// # Spec scratch
//
// All algorithms are derived directly from ITU-T G.729 (06/2012)
// Annex A §A.3.3 (lines 2057–2081 of the project's G729E.txt
// transcription). No reference C source has been consulted; the
// implementation is clean-room per project invariant I1.
//
// # Concurrency
//
// All exported and unexported functions in this package are pure
// (write-only through their out pointers) and safe for concurrent
// invocation. The package holds no global state beyond the
// compile-time gammaPow / gamma07Q15 constants.
package openloop
