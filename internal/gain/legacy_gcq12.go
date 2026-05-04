package gain

// SCHEDULED FOR REMOVAL: Phase 3a INT-1.
//
// After IMPL-3 (this commit), no production consumer of
// LegacyGcQ12FromMantExp remains. IMPL-2 removed the synth-side use
// (BuildExcitation/Synthesize take (gcMantQ14, gcExp) natively per
// REF-1 §2); IMPL-3 removes the encoder-side use (encoder.go fcbStep
// now derives a non-saturating int32 Q12 directly from (mant, exp) for
// its §A.3.10 commit accumulators — see encoder.go mantExpToQ12 — and
// internal/gainquant.SearchConjugate returns γ̂_c Q13 instead of the
// pre-IMPL-3 saturated ĝc Q12). The remaining callers are diagnostic
// test files in internal/decoder that emit gcQ12-formatted log lines
// for cross-comparison with historic numbers; they do not feed any
// production path.
//
// Remaining call sites at IMPL-3 landing:
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
// INT-1 will excise these probes (replacing them with (mant, exp)
// readouts) and delete this helper.
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
