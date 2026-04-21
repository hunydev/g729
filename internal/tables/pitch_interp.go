// Package tables holds pure numerical lookup tables (codebooks,
// quantizer entries, FIR coefficient sets) used by the G.729 codec.
//
// All entries in this package are flat data initializers transcribed
// from ITU-T G.729 normative tables under the "merger doctrine"
// exception: pure numerical interoperability data carries no
// creative expression. No algorithmic ITU C source code has been
// consulted; only the spec text, equation appendices, and the
// numerical initializer blocks of the reference distribution's
// `tab_ld8a.c` (data-only).
package tables

// PitchInterpFIR is the 1/3-sample fractional-delay interpolation
// filter b30 used by the adaptive codebook decoder, per ITU-T
// G.729 §3.7.1.
//
// The filter is a Hamming-windowed sinc with Linter = 10 one-sided
// taps, oversampled by 3 (cut-off −3 dB at 3600 Hz), truncated at
// ±29 and padded with a zero at ±30. By symmetry only one half of
// the response is stored; the symmetry around the center yields
// FIR_SIZE_SYN = 3·Linter + 1 = 31 unique Q15 coefficients.
//
// Indexing convention used by the pitch decoder (see
// internal/pitch/adaptive.go):
//
//	For an integer tap offset i ∈ [0, 9] and fractional offset
//	t ∈ {0, 1, 2} (representing delay-fraction 0, 1/3, 2/3),
//	the coefficient b30(t + 3i) of the spec equation (40) is
//	PitchInterpFIR[t + 3*i].  The center tap b30(0) = 1.0
//	is implicit (the integer-delay path uses a direct copy and
//	skips the FIR), so PitchInterpFIR[0] holds b30(1), the
//	first off-center positive-side tap.
//
// Table values are the Q15 integer coefficients from the ITU
// reference distribution (matching the floating-point comment
// block that accompanies the array there). Transcribed under the
// merger-doctrine exception; no algorithmic code was consulted.
var PitchInterpFIR = [31]int16{
	29443,
	25207, 14701, 3143,
	-4402, -5850, -2783,
	1211, 3130, 2259,
	0, -1652, -1666,
	-464, 756, 1099,
	550, -245, -634,
	-451, 0, 308,
	296, 78, -120,
	-165, -79, 34,
	91, 70, 0,
}
