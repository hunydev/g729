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
// PHASE 1n R-C-EMPIRICAL — TEST-ONLY KNOB (E2' relaxation)
//
// `lspInterpRoundMode` is a Phase 1n R-C-empirical disambiguation knob
// installed under the cycle plan
// `docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-plan.md`
// (Task RC-1) to empirically dispose of the R-C spec ambiguity by a
// branch-test on ALGTHM frame 0 sample 5..7.
//
// E2' INVARIANT: the default value (0 = floor `>> 1`) is byte-EQ to
// the cycle-entry production behaviour. All non-Phase-1n test gates
// (1..16 PASS, 17 SKIP, 18 PASS, 20 pending) MUST therefore remain
// byte-EQ while this knob is in place.
//
// CYCLE-END REMOVAL COMMITMENT: this knob, the test-only setter
// (`SetLSPInterpRoundModeForTest`), and the mode-1 branch are removed
// at Phase 1n RC-3 cycle close.
//   - Refute / Partial verdict → knob removed, expression reverted to
//     the cycle-entry single-line floor (production behaviour
//     unchanged).
//   - Defect-confirmed verdict → user gate G-XS7; on approval, knob
//     removed and expression replaced with the symmetric-round single
//     line in a separate fix cycle.
//
// Values:
//
//	0 = floor `(int32(prev[i]) + int32(curr[i])) >> 1`     ← DEFAULT
//	1 = round-half-away-from-zero (symmetric round)
//
// This variable is NOT goroutine-safe; the test setter is intended for
// single-goroutine measurement scaffolding only.
var lspInterpRoundMode int

func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
	for i := 0; i < 10; i++ {
		sum := int32(prev[i]) + int32(curr[i])
		switch lspInterpRoundMode {
		case 1:
			// Round-half-away-from-zero (symmetric round).
			if sum >= 0 {
				sf1[i] = int16((sum + 1) >> 1)
			} else {
				sf1[i] = int16(-((-sum + 1) >> 1))
			}
		default:
			// Floor toward −∞ via arithmetic right shift on signed
			// int32 — byte-EQ to the cycle-entry production
			// expression `int16((int32(prev[i]) + int32(curr[i])) >> 1)`.
			sf1[i] = int16(sum >> 1)
		}
		sf2[i] = curr[i]
	}
}
