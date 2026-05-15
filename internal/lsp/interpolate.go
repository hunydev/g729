package lsp

// interpolateLSP produces the per-subframe LSP vectors for one frame
// per ITU-T G.729 §3.2.5 eq. (24):
//
//	sf1[i] = 0.5·prev[i] + 0.5·curr[i]
//	sf2[i] = curr[i]
//
// The midpoint is computed as the sum of two independently halved
// Word16 values. This is not algebraically identical to
// (prev+curr)>>1 for odd/odd pairs; the independent shifts are pinned
// by the decoder_tame_lsp_pipeline numeric oracle.
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
// R-C disposition (decoder_tame_lsp_pipeline): the spec's "0.5
// multiplication" is implemented with per-operand arithmetic shifts:
// (prev >> 1) + (curr >> 1). For odd/odd pairs this is 1 LSB smaller
// than shifting the summed pair, and that difference is visible in the
// reference LSP interpolation and LP coefficient oracles.
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
// Per E2' cycle-end commitment, the old round-mode knob (and its test
// setter `SetLSPInterpRoundModeForTest`) were removed at Phase 1n
// RC-3. The later TAME verifier artifact resolved the fixed-point
// ambiguity directly. See the Phase 1n RC-3 synthesis report
// (`docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-synthesis-report.md`)
// for the full mechanistic argument and cumulative scoreboard.
func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
	for i := 0; i < 10; i++ {
		sf1[i] = int16((int32(prev[i]) >> 1) + (int32(curr[i]) >> 1))
		sf2[i] = curr[i]
	}
}
