package synth

import (
	"github.com/exedev/g729/internal/fixed"
)

// filterSubframe applies 1/A(z) to u in-place, producing s.
//
// Spec: ITU-T G.729 §3.10 / §4.1.2.
//
//	s(n) = u(n) − Σ_{i=1..10} a[i] · s(n−i), n ∈ [0, 39]
//
// Q-format pipeline per sample:
//
//	L_temp = LMult(u[n], 4096)                 // Q13 in Word32 (2·u·4096)
//	for i = 1..10:
//	  L_temp = LMsu(L_temp, a[i], s[n-i])      // subtract 2·a[i]·s(n-i) Q13
//	L_temp = LShl(L_temp, 3)                   // Q13 → Q16
//	s[n]   = Round(L_temp)                     // Q0 Word16
//
// Past-state feedback (s(n−i) for n−i < 0) is read from synth.pastSynth;
// on return, synth.pastSynth is updated to hold s[30..39].
//
// Implementation uses a stack-resident 50-sample working buffer to unify
// past and current state indexing; no heap allocations.
func (synth *Synthesizer) filterSubframe(a *[11]int16, u, s *[40]int16) {
	var work [50]int16
	copy(work[:10], synth.pastSynth[:])

	for n := 0; n < 40; n++ {
		lTemp := fixed.LMult(u[n], 4096)
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}

	copy(s[:], work[10:])
	copy(synth.pastSynth[:], work[40:])
}
