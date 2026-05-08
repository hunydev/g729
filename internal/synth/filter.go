package synth

import "github.com/hunydev/g729/internal/fixed"

// filterSubframe applies 1/A(z) to u, producing s, with the ITU §3.10
// two-pass saturation-recovery strategy.
//
// Pass 1: clear fixed.Overflow, run the 40-sample LMult / LMsu / LShl /
// Round chain in fixed primitives, check Overflow at end. If NOT set,
// persist the output as-is.
//
// Pass 2 (on Overflow): scale both u and pastSynth by ¼, re-run, then
// scale the resulting Word16 per-sample output by ×4 with Word16
// saturation. The persisted pastSynth holds the post-recovery output
// so subsequent subframes inherit the correct state.
//
// Past-state scaling is required because past-driven overflow cannot
// be cancelled by input-only scaling.
func (synth *Synthesizer) filterSubframe(a *[11]int16, u, s *[40]int16) {
	var work [50]int16
	copy(work[:10], synth.pastSynth[:])

	fixed.ClearOverflow()
	synth.onePass(a, u, &work)
	if !fixed.Overflow() {
		copy(s[:], work[10:])
		copy(synth.pastSynth[:], work[40:])
		return
	}

	// Pass 2: scale input and past state by 1/2.
	var work2 [50]int16
	for i, v := range synth.pastSynth {
		work2[i] = int16(int32(v) >> 1)
	}
	var uScaled [40]int16
	for i, v := range u {
		uScaled[i] = int16(int32(v) >> 1)
	}
	fixed.ClearOverflow()
	synth.onePass(a, &uScaled, &work2)

	// Scale back up by ×2 with Word16 saturation.
	for i := 10; i < 50; i++ {
		v := int32(work2[i]) << 1
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		work2[i] = int16(v)
	}
	copy(s[:], work2[10:])
	copy(synth.pastSynth[:], work2[40:])
}

// onePass runs the 40-sample direct-form 1/A(z) loop using fixed-point
// primitives so that fixed.Overflow is set whenever any LMsu/LShl/Round
// step saturates. Writes outputs into work[10..49].
func (synth *Synthesizer) onePass(a *[11]int16, u *[40]int16, work *[50]int16) {
	for n := 0; n < 40; n++ {
		lTemp := fixed.LMult(u[n], a[0])
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}
}
