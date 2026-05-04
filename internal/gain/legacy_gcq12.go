package gain

// LegacyGcQ12FromMantExp converts the new (mantissa Q14, exponent int8)
// g_c representation back to the legacy single Q12 int16 form with
// saturation.
//
// Math: linear g_c = mantQ14 · 2^(exp - 14); the Q12 form is g_c · 2^12
// = mantQ14 · 2^(exp - 14 + 12) = mantQ14 · 2^(exp - 2). Positive shifts
// saturate at the int16 envelope; negative shifts arithmetic-right with
// floor-towards-negative semantics. mant=0 short-circuits to 0.
//
// TEMPORARY shim used only by Phase 3a IMPL-1 to keep decoder /
// diagnostic tests building while the new mantissa+exponent triple
// propagates through the call graph. Removed in Phase 3a INT-1 once
// IMPL-2 (synth.BuildExcitation) and IMPL-3/4 (encoder/decoder call
// sites) have landed.
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
