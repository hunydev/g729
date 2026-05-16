package postfilter

import "github.com/hunydev/g729/internal/fixed"

// tiltLen is the impulse-response truncation length per ITU-T G.729
// §A.4.2.3 — "the impulse response … is truncated after 22 samples".
const tiltLen = 22

// gammaTiltPositiveK1Q14 is Annex A γ_t = 0.8 in Q14, used when k1' > 0.
const gammaTiltPositiveK1Q14 int16 = 13107 // round(0.8·2^14)

// gammaTiltNonPositiveK1Q14 is Annex A γ_t = 0 when k1' <= 0.
const gammaTiltNonPositiveK1Q14 int16 = 0

// Legacy names retained for older diagnostics in this package.
const (
	gammaTiltActiveQ14   = gammaTiltPositiveK1Q14
	gammaTiltInactiveQ14 = gammaTiltNonPositiveK1Q14
)

// computeTiltMu derives μ for H_t(z) = 1 - μ·z⁻¹ from the
// cascade A(z/γ_n)/A(z/γ_d) per ITU-T G.729 §A.4.2.3:
//
//	h(n) = impulse response of A(z/γ_n)/A(z/γ_d), n = 0..tiltLen-1
//	r_h(i) = Σ_{n=0..tiltLen-1-i} h(n) · h(n+i),  i ∈ {0, 1}
//	k_1'  = r_h(1) / r_h(0)
//	γ_t   = 0.8 if k_1' > 0, else 0
//	μ     = γ_t · k_1'
func (pf *Postfilter) computeTiltMu(aNum, aDen *[lpcOrder + 1]int16) int16 {
	// 1. Compute h(n) for n = 0..tiltLen-1.
	var h [tiltLen]int32
	for n := 0; n < tiltLen; n++ {
		var hNum int32
		switch {
		case n == 0:
			hNum = int32(aNum[0])
		case n <= lpcOrder:
			hNum = int32(aNum[n])
		default:
			hNum = 0
		}
		acc := hNum << 12
		for k := 1; k <= lpcOrder && k <= n; k++ {
			acc -= int32(aDen[k]) * h[n-k]
		}
		h[n] = (acc + (1 << 11)) >> 12
	}

	// 2. Autocorrelation r_h(0), r_h(1). Annex A keeps these in the
	// doubled L_mult domain, then divides the high words.
	var rh0, rh1 int64
	for n := 0; n < tiltLen; n++ {
		rh0 += 2 * int64(h[n]) * int64(h[n])
	}
	for n := 0; n < tiltLen-1; n++ {
		rh1 += 2 * int64(h[n]) * int64(h[n+1])
	}

	if rh0 == 0 {
		return 0
	}
	den := int16(rh0 >> 16)
	if den <= 0 {
		return 0
	}
	var num int16
	if rh1 > 0 {
		num = int16(rh1 >> 16)
	}
	if num <= 0 {
		return 0
	}

	scaledNum := int16((int32(gammaTiltPositiveK1Q14) * int32(num)) >> 14)
	return fixed.DivS(scaledNum, den)
}

// applyTiltWithMu applies the one-tap tilt filter
//
//	s_tilt(n) = x(n) - μ · x(n-1)
//
// per ITU-T G.729 §A.4.2.3. In the Annex A decoder chain, x is the
// long-term postfilter output and s_tilt feeds the short-term synthesis
// postfilter.
//
// pastTiltInput holds x(-1) on entry; on return, holds x(39).
func (pf *Postfilter) applyTiltWithMu(sIn *[subframeLen]int16, muQ15 int16, sOut *[subframeLen]int16) {
	prev := pf.pastTiltInput
	for n := 0; n < subframeLen; n++ {
		prod := int32(muQ15) * int32(prev)
		contrib := int16(prod >> 15)
		sum := int32(sIn[n]) - int32(contrib)
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		sOut[n] = int16(sum)
		prev = sIn[n]
	}
	pf.pastTiltInput = prev
}
