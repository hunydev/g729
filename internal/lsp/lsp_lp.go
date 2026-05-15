package lsp

// LSPToLP converts a 10-element Q15 LSP vector into the 11
// coefficients of the synthesis filter A(z) in Q12 Word16, with
// a[0] = 4096 (i.e. 1.0 in Q12). Per ITU-T G.729 §3.2.6 / §4.1.6:
//
// F1(z) = Π_{i ∈ {0,2,4,6,8}} (1 − 2·q_i·z^-1 + z^-2)
// F2(z) = Π_{i ∈ {1,3,5,7,9}} (1 − 2·q_i·z^-1 + z^-2)
// A(z)  = ((1 + z^-1)·F1(z) + (1 − z^-1)·F2(z)) / 2
//
// The fixed-point path keeps the reduced F1/F2 polynomials in Q24,
// applies the (1+z^-1)/(1-z^-1) post transforms, then promotes the
// post-transform values to Q28 for the final sum. This matches the
// decoder_tame_lp_polynomial_step numeric oracle; running the
// recurrence directly in Q28 keeps extra fractional product bits and
// shifts a few half-boundary LP coefficients by 1 LSB.
func LSPToLP(lsp *[10]int16, a *[11]int16) {
	var f1, f2 [6]int64
	buildLSPPolyQ24(lsp, 0, &f1)
	buildLSPPolyQ24(lsp, 1, &f2)

	for i := 5; i >= 1; i-- {
		f1[i] += f1[i-1]
		f2[i] -= f2[i-1]
	}

	setA := func(k int, sumQ28 int64) {
		sum := (sumQ28 + (1 << 15)) >> 16
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		a[k] = int16(sum)
	}

	setA(0, 1<<28)
	for i := 1; i <= 5; i++ {
		setA(i, (f1[i]+f2[i])<<3)
		setA(11-i, (f1[i]-f2[i])<<3)
	}
}

func buildLSPPolyQ24(lsp *[10]int16, offset int, f *[6]int64) {
	*f = [6]int64{}
	f[0] = 1 << 24
	f[1] = -lspPolyProductQ24(int64(lsp[offset]), f[0])

	for step := 1; step < 5; step++ {
		q := int64(lsp[2*step+offset])
		old := *f
		var next [6]int64
		next[0] = old[0]
		next[1] = old[1] - lspPolyProductQ24(q, old[0])
		for j := step; j >= 1; j-- {
			add := old[j-1]
			if j == step {
				add <<= 1
			}
			next[j+1] = old[j+1] - lspPolyProductQ24(q, old[j]) + add
		}
		*f = next
	}
}

func lspPolyProductQ24(q, coeff int64) int64 {
	return ((q * coeff) >> 16) << 2
}
