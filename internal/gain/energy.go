package gain

import "github.com/exedev/g729/internal/fixed"

// fixedCodebookEnergy returns Σ c[n]² as a non-negative Q0 Word32.
//
// Per ITU-T G.729 §3.9 eq. (66), the mean energy used downstream is
// E̅_c = 10·log10((1/40)·Σc²); this helper provides the inner sum in
// Q0 so that subsequent log2 conversion can apply the (1/40) and the
// dB scaling without intermediate precision loss.
//
// Each pulse magnitude is bounded by ±8192 (Q13 unit pulse), so the
// 40-term sum cannot exceed 40·8192² < 2³¹ and never saturates.
func fixedCodebookEnergy(c *[40]int16) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		// LMult returns 2·a·b in Q1; LShr by 1 recovers a·b in Q0.
		acc = fixed.LAdd(acc, fixed.LShr(fixed.LMult(fixed.Word16(c[n]), fixed.Word16(c[n])), 1))
	}
	return acc
}
