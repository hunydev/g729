package gain

// KEPT-WITH-TEST-CONSUMERS: Phase 3a INT-1 (IMPL-4 finalization).
//
// IMPL-4 audit (INT-1, post-c7fcc06): production-side consumers of
// LegacyGcQ12FromMantExp = 0. Verified by repo-wide grep:
//
//	grep -rn 'LegacyGcQ12FromMantExp' --include='*.go' . \
//	  | grep -v '_test.go'
//
// returns only this file and a back-reference comment in
// internal/gain/phase3a_diag1_export.go (DocComment, not a call site).
//
// Native (gpQ14, gcMantQ14 Q14, gcExp int8) triple is end-to-end:
//   - decoder: internal/decoder/subframe.go:36-39 calls
//     d.gn.Decode(...) → (gpQ14, gcMant, gcExp) and feeds them
//     directly to synth.BuildExcitation.
//   - encoder: internal/encoder.go derives a non-saturating int32 Q12
//     from (mant, exp) via mantExpToQ12 for its §A.3.10 commit
//     accumulators; internal/gainquant.SearchConjugate returns γ̂_c
//     Q13 instead of the pre-IMPL-3 saturated ĝ_c Q12.
//
// Remaining test-only callers at INT-1 landing (10 files; exceeds the
// IMPL-4 plan's "≤ 3 tests inlined" budget for adapter excision, so
// the adapter is KEPT rather than inlined; revisited under Phase 3b
// once decoder amplitude-recovery follow-up lands and the decoder
// diagnostic test surface is rewritten):
//   - internal/gain/legacy_gcq12_test.go (adapter self-test)
//   - internal/gain/decode_test.go (Q12 logging in legacy harness)
//   - internal/decoder/decode_test.go (logging)
//   - internal/decoder/diagnostic_singlepulse_test.go (logging)
//   - internal/decoder/diagnostic_multipulse_test.go (logging)
//   - internal/decoder/phase1o_d3_s3_handoff_dump_test.go (logging)
//   - internal/decoder/phase1o_d3_s5_r3_excitation_dump_test.go (logging)
//   - internal/decoder/stagef_quart_diagnostic_test.go (productionGainProbe)
//   - internal/decoder/stagef_sext_diagnostic_test.go (logging)
//   - internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go
//     (constant comparison)
//
// All of the above are diagnostic / dump probes, not production-
// surface tests. They cross-reference historical Q12-formatted log
// lines for byte-comparable trace artefacts; none feed a production
// pipeline. The adapter therefore has zero binary footprint in
// shipped code paths and zero impact on the IMPL-1..3 arithmetic
// IMPL-4 / INT-1 has finalized.
//
// LegacyGcQ12FromMantExp converts the new (mantissa Q14, exponent int8)
// g_c representation back to the legacy single Q12 int16 form with
// saturation.
//
// Math: linear g_c = mantQ14 · 2^(exp - 14); the Q12 form is g_c · 2^12
// = mantQ14 · 2^(exp - 14 + 12) = mantQ14 · 2^(exp - 2). Positive shifts
// saturate at the int16 envelope; negative shifts arithmetic-right with
// floor-towards-negative semantics. mant=0 short-circuits to 0.
func LegacyGcQ12FromMantExp(mantQ14 int16, exp int8) int16 {
	if mantQ14 == 0 {
		return 0
	}
	shift := int(exp) - 2
	switch {
	case shift >= 0:
		if shift >= 16 {
			if mantQ14 > 0 {
				return 32767
			}
			return -32768
		}
		v := int32(mantQ14) << uint(shift)
		if v > 32767 {
			return 32767
		}
		if v < -32768 {
			return -32768
		}
		return int16(v)
	default:
		s := -shift
		if s >= 31 {
			return 0
		}
		return int16(int32(mantQ14) >> uint(s))
	}
}
