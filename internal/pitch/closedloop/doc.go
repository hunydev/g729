// Package closedloop implements the encoder-side §A.3.5–§A.3.7
// closed-loop pitch search used by the G.729 Annex A reduced-
// complexity encoder.
//
// The package is a sibling of internal/pitch/openloop/ (which hosts
// §A.3.3 perceptual weighting and §A.3.4 open-loop pitch search). It
// owns symbols whose state is private to the closed-loop search and
// must not leak into the decoder API surface.
//
// # Phase 2c-HI-1 surface
//
//	ImpulseResponse(aHatQ12, h)
//	    Computes the 40-tap impulse response h(n) of the weighted
//	    synthesis filter 1/Â(z/γ) with γ = 0.75, per ITU-T G.729
//	    Annex A §A.3.5 (G729E.txt lines 2114–2117). The first
//	    argument is the order-10 quantized LP polynomial Â in Q12
//	    with leading tap aHatQ12[0] = 4096 (ITU convention). The
//	    output array h is overwritten with the response in Q12.
//
// # Q-format invariants
//
//	aHatQ12 : Q12, aHatQ12[0] = 4096, |aHatQ12[i]| < 2^15.
//	h       : Q12, h[0] = 4096 (i.e. 1.0) for any normalized Â,
//	          subsequent samples bounded by Word16 saturation in the
//	          all-pole recurrence.
//
// # Spec scratch
//
// All algorithms are derived directly from ITU-T G.729 (06/2012)
// Annex A. No reference C source has been consulted; the
// implementation is clean-room per project invariant I1.
//
// Source spec line numbers refer to the project's
// docs/superpowers/specs/itu/G729E.txt transcription:
//
//	§A.3.5 lines 2114–2117 — Computation of the impulse response.
//	§A.3.3 line 2063       — γ = 0.75 (Annex A perceptual weight).
//
// # Allocation
//
// All routines are zero-allocation; scratch is held on the stack via
// fixed-size local arrays. No heap activity is required for any
// closed-loop step and a benchmark gate enforces this.
package closedloop
