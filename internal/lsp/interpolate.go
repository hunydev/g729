package lsp

// interpolateLSP produces the per-subframe LSP vectors for one frame
// per ITU-T G.729 §3.2.5 eq. (24):
//
//	sf1[i] = 0.5·prev[i] + 0.5·curr[i]
//	sf2[i] = curr[i]
//
// The midpoint is computed in 32-bit before the shift so the average
// of two near-full-scale Q15 operands does not saturate prematurely.
// The result is always within Word16 range, so no explicit saturation
// is needed after the shift.
//
// ============================================================================
// SPEC VERBATIM CITATION (ITU-T G.729 (06/2012), PDF p. 14, §3.2.5
// "Interpolation of the LSP coefficients", lines 901..919; extracted
// via `pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -`):
//
//	"The quantized (and unquantized) LP coefficients are used for the
//	 second subframe. For the first subframe, the quantized (and
//	 unquantized) LP coefficients are obtained by linear interpolation
//	 of the corresponding parameters in the adjacent subframes. The
//	 interpolation is done on the LSP coefficients in the cosine
//	 domain. Let q_i^(current) be the LSP coefficients computed for
//	 the current 10 ms frame, and q_i^(previous) the LSP coefficients
//	 computed in the previous 10 ms frame. The (unquantized)
//	 interpolated LSP coefficients in each of the two subframes are
//	 given by:
//	    Subframe 1 : q_i^(1) = 0.5 q_i^(previous) + 0.5 q_i^(current)
//	                                       i = 1,...,10
//	    Subframe 2 : q_i^(2) = q_i^(current)        i = 1,...,10
//	                                                           (24)
//	 The same interpolation procedure is used for the interpolation
//	 of the quantized LSP coefficients by substituting q_i by q̂_i in
//	 equation (24)."
//
// R-C ambiguity (Phase 1m CE-3, commit 5232411): the spec's "0.5
// multiplication" of the half-sum is silent on the fixed-point
// rounding mode. Production uses arithmetic right shift `>> 1` which
// is floor-toward-negative-infinity on the int32 sum; for odd
// `(prev+curr)` with negative result this differs by −1 LSB from
// symmetric rounding (round-half-away-from-zero). On ALGTHM frame 0
// the i=1 and i=5 cells fall in this odd-half-sum regime.
//
// ============================================================================
// PHASE 1n RC-1 EMPIRICAL DISPOSITION (commit a47f03f)
//
// Phase 1n RC-1 installed a test-only `lspInterpRoundMode` knob (under
// E2' relaxation per
// `docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-plan.md`)
// to branch-test symmetric rounding against floor on ALGTHM frame 0
// sample 5..7. The branch-test verdict was REFUTE_unchanged: symmetric
// rounding leaves sample 5..7 byte-identical to floor. Mechanistic
// reason — only LSP cells i=1 and i=5 differ by 1 LSB under the mode
// flip, which (after Chebyshev expansion / §3.2.6) perturbs only the
// LP taps a[8..10]; at frame 0 the past synth-filter state is zero
// so a[8..10] multiply zero for n < 10 and cannot reach n=5..7.
//
// Per E2' cycle-end commitment, the knob (and its test setter
// `SetLSPInterpRoundModeForTest`) were removed at Phase 1n RC-3,
// restoring this function to its cycle-entry single-line floor
// expression. Production behaviour is byte-EQ to the cycle-entry
// state. R-C remains an unresolved verbatim documentation issue but
// is no longer a candidate gate 17 sample-5..7 mechanism. See the
// Phase 1n RC-3 synthesis report
// (`docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-synthesis-report.md`)
// for the full mechanistic argument and cumulative scoreboard.
func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
	for i := 0; i < 10; i++ {
		sf1[i] = int16((int32(prev[i]) + int32(curr[i])) >> 1)
		sf2[i] = curr[i]
	}
}
