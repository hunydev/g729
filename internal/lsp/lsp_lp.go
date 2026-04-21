package lsp

import "github.com/exedev/g729/internal/fixed"

// lspToLP converts a 10-element Q15 LSP vector into the 11
// coefficients of the synthesis filter A(z) in Q12 Word16, with
// a[0] = 4096 (i.e. 1.0 in Q12). Per ITU-T G.729 §3.2.6 / §4.1.6:
//
//	F1(z) = Π_{i ∈ {0,2,4,6,8}} (1 − 2·q_i·z^-1 + z^-2)
//	F2(z) = Π_{i ∈ {1,3,5,7,9}} (1 − 2·q_i·z^-1 + z^-2)
//	A(z)  = ((1 + z^-1)·F1(z) + (1 − z^-1)·F2(z)) / 2
//
// F1 and F2 are accumulated in Q28 Word32. Each factor is applied via
// the recurrence f_new[j] = f[j] − 2·q·f[j−1] + f[j−2] with j swept
// from high to low so the in-place update is safe.
func lspToLP(lsp *[10]int16, a *[11]int16) {
	var f1, f2 [11]fixed.Word32
	f1[0] = 1 << 28
	f2[0] = 1 << 28

	for step := 0; step < 5; step++ {
		q1 := lsp[2*step]
		q2 := lsp[2*step+1]

		top := 2*step + 2
		for j := top; j >= 2; j-- {
			f1[j] = polyStep(f1[j], q1, f1[j-1], f1[j-2])
			f2[j] = polyStep(f2[j], q2, f2[j-1], f2[j-2])
		}
		f1[1] = polyStep(f1[1], q1, f1[0], 0)
		f2[1] = polyStep(f2[1], q2, f2[0], 0)
	}

	// Assemble A(z): a[k] = (F1[k] + F1[k-1] + F2[k] − F2[k-1]) / 2,
	// then convert Q28 → Q12 with rounding (>>17 with bias 1<<16).
	for k := 0; k <= 10; k++ {
		var prev1, prev2 fixed.Word32
		if k > 0 {
			prev1 = f1[k-1]
			prev2 = f2[k-1]
		}
		sum := int64(f1[k]) + int64(prev1) + int64(f2[k]) - int64(prev2)
		sum = (sum + (1 << 16)) >> 17
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		a[k] = int16(sum)
	}
}

// polyStep computes one Chebyshev recurrence update:
//
//	f_new = f − 2·q·f_prev1 + f_prev2
//
// q is Q15; f / f_prev1 / f_prev2 are Q28. The 2·q·f_prev1 product
// is formed in int64 and brought back to Q28 with a >>14 shift. The
// final accumulation uses fixed.LSub / fixed.LAdd so any Word32
// overflow saturates rather than wraps.
func polyStep(f fixed.Word32, q int16, fPrev1, fPrev2 fixed.Word32) fixed.Word32 {
	prod := (int64(q) * int64(fPrev1)) >> 14
	if prod > int64(fixed.Max32) {
		prod = int64(fixed.Max32)
	} else if prod < int64(fixed.Min32) {
		prod = int64(fixed.Min32)
	}
	result := fixed.LSub(f, fixed.Word32(prod))
	result = fixed.LAdd(result, fPrev2)
	return result
}
