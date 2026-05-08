package fcbsearch

import "github.com/hunydev/g729/internal/fcb"

// BuildCode constructs the ACELP fixed-codebook excitation c[0..39]
// per ITU-T G.729 §3.8 equations (45), (46), and (47):
//
//	c(n)  = Σ_{i=0..3} sign_i · δ(n − m_i)                     (eq. 45)
//	c'(n) = c(n) + β · c'(n − T)         for n = T..39         (eq. 46)
//	β     = clamp(ĝ_p^(m−1), 0.2, 0.8)                         (eq. 47)
//
// The four pulse positions are taken from positions[0..3] and the
// per-position signs are read from signs[positions[i]] in the §3.8.1
// sign-decomposition convention (signs ∈ {−1,+1}, indexed by absolute
// position) — i.e. the same shape produced by SignsFromD (CB-3) and
// consumed by SearchDepthFirst (CB-2). Positive sign places +1.0 in
// Q13 (= fcb.PulseAmplitude); non-positive places −1.0 in Q13.
//
// intLag is the integer pitch lag T of the current subframe (eq. 46);
// the harmonic enhancement is bypassed when T ≥ 40 per eq. 48 / §3.8
// boundary. prevGpQ14 is the previous subframe's quantized adaptive-
// codebook gain ĝ_p^(m−1) in Q14, clamped to [0.2, 0.8] via eq. 47
// (delegated to fcb.ClampPitchGainForEnhancement). The IIR
// enhancement loop itself is delegated to fcb.ApplyPitchEnhancement —
// the same implementation used by the decoder per §4.1.5, honoring
// the merger doctrine (Phase 2d sub-plan §3.2).
//
// Output c[] is Q13. I3 / I4: pure (writes only through c), zero
// allocation.
func BuildCode(positions *[4]int8, signs *[SubframeLen]int16, intLag int16, prevGpQ14 int16, c *[SubframeLen]int16) {
	BuildSparseCode(positions, signs, c)
	betaQ14 := fcb.ClampPitchGainForEnhancement(prevGpQ14)
	fcb.ApplyPitchEnhancement(c, int(intLag), betaQ14)
}

// BuildSparseCode constructs the unfiltered algebraic code vector of
// §3.8 eq. (45), before the pitch-enhancement prefilter of eq. (46).
func BuildSparseCode(positions *[4]int8, signs *[SubframeLen]int16, c *[SubframeLen]int16) {
	for i := range c {
		c[i] = 0
	}
	for i := 0; i < 4; i++ {
		p := positions[i]
		if signs[p] > 0 {
			c[p] = fcb.PulseAmplitude
		} else {
			c[p] = -fcb.PulseAmplitude
		}
	}
}
