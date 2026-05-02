package lsp

// SetLSPInterpRoundModeForTest installs the Phase 1n R-C-empirical
// sf-1 LSP interpolation rounding-mode knob (`lspInterpRoundMode`) for
// the duration of a measurement test, returning a `restore` func that
// resets the knob to its prior value. Callers MUST defer `restore()`.
//
// Modes (mirrors `interpolateLSP`):
//
//	0 = floor `(prev+curr) >> 1`               ← cycle-entry default
//	1 = round-half-away-from-zero
//
// CYCLE-END REMOVAL COMMITMENT (E2' relaxation, RC-3 close): this
// setter and the underlying knob are deleted at Phase 1n RC-3 per
// `docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-plan.md`
// (Phase 7). It exists only to enable the RC-1 branch-test in
// `internal/decoder/phase1n_rc1_lspinterp_branch_diagnostic_test.go`.
//
// This setter is NOT goroutine-safe; tests using it must not run in
// parallel with any code path exercising `interpolateLSP`.
func SetLSPInterpRoundModeForTest(mode int) (restore func()) {
	prev := lspInterpRoundMode
	lspInterpRoundMode = mode
	return func() { lspInterpRoundMode = prev }
}
