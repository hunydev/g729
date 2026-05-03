package lsp

// LSPToLP converts a 10-element Q15 LSP vector into the 11
// coefficients of the synthesis filter A(z) in Q12 Word16, with
// a[0] = 4096 (i.e. 1.0 in Q12). Per ITU-T G.729 §3.2.6 / §4.1.6:
//
// F1(z) = Π_{i ∈ {0,2,4,6,8}} (1 − 2·q_i·z^-1 + z^-2)
// F2(z) = Π_{i ∈ {1,3,5,7,9}} (1 − 2·q_i·z^-1 + z^-2)
// A(z)  = ((1 + z^-1)·F1(z) + (1 − z^-1)·F2(z)) / 2
//
// The §3.2.6 recurrence
//
// F_i(j) = F_i(j) − 2·q_i·F_i(j−1) + F_i(j−2)
//
// is exact arithmetic — the spec does not authorise saturation on the
// intermediate F polynomials. Their middle-stage |F| can transiently
// exceed the Q28 Word32 envelope (~|F| ≤ 7.999) while the final
// symmetric/antisymmetric A polynomial remains in Q12 Word16 range.
// We therefore keep f1, f2 in int64 and only apply Word16 saturation
// on the final a[k] output (which is the §3.2.6 output domain).
func LSPToLP(lsp *[10]int16, a *[11]int16) {
	var f1, f2 [11]int64
	const oneQ28 int64 = 1 << 28
	f1[0] = oneQ28
	f2[0] = oneQ28

	for step := 0; step < 5; step++ {
		q1 := int64(lsp[2*step])
		q2 := int64(lsp[2*step+1])

		top := 2*step + 2
		for j := top; j >= 2; j-- {
			f1[j] = polyStepExact(f1[j], q1, f1[j-1], f1[j-2])
			f2[j] = polyStepExact(f2[j], q2, f2[j-1], f2[j-2])
		}
		f1[1] = polyStepExact(f1[1], q1, f1[0], 0)
		f2[1] = polyStepExact(f2[1], q2, f2[0], 0)
	}

	// Assemble A(z): a[k] = (F1[k] + F1[k-1] + F2[k] − F2[k-1]) / 2,
	// then convert Q28 → Q12 with rounding (>>17 with bias 1<<16).
	for k := 0; k <= 10; k++ {
		var prev1, prev2 int64
		if k > 0 {
			prev1 = f1[k-1]
			prev2 = f2[k-1]
		}
		sum := f1[k] + prev1 + f2[k] - prev2
		sum = (sum + (1 << 16)) >> 17
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		a[k] = int16(sum)
	}
}

// polyStepExact computes one Chebyshev recurrence update in exact
// int64 arithmetic, with no saturation:
//
// f_new = f − 2·q·f_prev1 + f_prev2
//
// q is Q15; f / f_prev1 / f_prev2 are Q28. The 2·q·f_prev1 product
// is formed as q·f_prev1 (Q15·Q28 = Q43) and shifted right by 14 to
// land back in Q28 (the factor of 2 is absorbed by the asymmetric
// shift, mirroring the original polyStep convention).
func polyStepExact(f, q, fPrev1, fPrev2 int64) int64 {
	prod := (q * fPrev1) >> 14
	return f - prod + fPrev2
}
