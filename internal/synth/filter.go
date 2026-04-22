package synth

// filterSubframe applies 1/A(z) to u, producing s, with the ITU §3.10
// two-pass saturation-recovery strategy.
//
// First pass: compute the exact L_temp accumulator in int64 to detect
// overflow that the fixed-point LMsu/LShl chain would silently
// saturate.  If the exact pre-rounding value would exceed Word32 range
// after the LShl(_, 3) (i.e. |L_temp| ≥ 2²⁸), bail out and re-run with
// u and the past-synthesis history both divided by 4; the recovered
// per-sample output is then multiplied by 4 and saturated to Word16.
// Past-state scaling is required because past-driven overflow cannot
// be cancelled by input-only scaling.  The persisted pastSynth holds
// the un-scaled output so subsequent subframes inherit the correct
// state.  No-overflow path is bit-exactly equivalent to the original
// LMult / LMsu / LShl(_, 3) / Round chain.
func (synth *Synthesizer) filterSubframe(a *[11]int16, u, s *[40]int16) {
	var work [50]int16
	copy(work[:10], synth.pastSynth[:])

	if !synth.tryFilterPass(a, u, &work, 1) {
		copy(s[:], work[10:])
		copy(synth.pastSynth[:], work[40:])
		return
	}

	var work2 [50]int16
	for i, v := range synth.pastSynth {
		work2[i] = int16(int32(v) >> 2)
	}
	var uScaled [40]int16
	for i, v := range u {
		uScaled[i] = int16(int32(v) >> 2)
	}
	synth.tryFilterPass(a, &uScaled, &work2, 4)
	copy(s[:], work2[10:])
	copy(synth.pastSynth[:], s[30:])
}

// tryFilterPass runs the 40-sample direct-form 1/A(z) loop writing into
// work[10..49].  outMult is the post-rounding output scale (1 for the
// normal pass, 4 for the recovery pass).  Returns true if the exact
// pre-shift accumulator magnitude would have saturated LShl(_, 3) on
// the normal pass; in that case work is left in a half-written state
// and the caller must restart with u scaled down.
//
// The accumulator is computed in int64 so that intermediate LMsu
// saturation cannot silently corrupt the final value.  When the int64
// result fits in the Q13 Word32 representation, this is bit-exactly
// equivalent to the original LMult / LMsu chain.
func (synth *Synthesizer) tryFilterPass(a *[11]int16, u *[40]int16, work *[50]int16, outMult int32) bool {
	const maxPreShift = int64(1 << 28)

	for n := 0; n < 40; n++ {
		// L_temp = 2·u(n)·4096 − Σ_{i=1..10} 2·a[i]·s(n−i)
		acc := int64(u[n]) << 13
		for i := 1; i <= 10; i++ {
			acc -= int64(a[i]) * int64(work[10+n-i]) << 1
		}

		if outMult == 1 && (acc >= maxPreShift || acc <= -maxPreShift) {
			return true
		}

		// LShl(L_temp, 3) — multiply by 8 with Word32 saturation.
		shifted := acc << 3
		if shifted > (1<<31)-1 {
			shifted = (1 << 31) - 1
		} else if shifted < -(1 << 31) {
			shifted = -(1 << 31)
		}

		// Round = saturating (shifted + 0x8000) >> 16.
		rounded := (shifted + (1 << 15)) >> 16
		if rounded > 32767 {
			rounded = 32767
		} else if rounded < -32768 {
			rounded = -32768
		}

		if outMult != 1 {
			rounded *= int64(outMult)
			if rounded > 32767 {
				rounded = 32767
			} else if rounded < -32768 {
				rounded = -32768
			}
		}
		work[10+n] = int16(rounded)
	}
	return false
}
