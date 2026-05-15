package lsp

import "github.com/hunydev/g729/internal/tables"

// lspToLSF is the inverse of lsfToLSP per ITU-T G.729 §3.2.5: given a
// Q15 LSP value q = cos(ω), return the corresponding Q13 LSF
// ω ∈ [0, π]. The conversion is implemented as a small integer binary
// search over lsfToLSP itself, keeping encoder-side inversion locked to
// the same fixed-point approximation used by the decoder.
//
// I3 / I4: pure function, no allocation, no panic.
func lspToLSF(q int16) int16 {
	qi := int32(q)
	if qi >= int32(tables.CosLSP[0]) {
		return 0
	}
	if qi <= int32(tables.CosLSP[64]) {
		return int16(lspPiQ13)
	}

	lo, hi := int32(0), lspPiQ13
	for hi-lo > 1 {
		mid := (lo + hi) >> 1
		if int32(lsfToLSP(int16(mid))) > qi {
			lo = mid
		} else {
			hi = mid
		}
	}

	bestOmega := lo
	bestDiff := absInt32(qi - int32(lsfToLSP(int16(lo))))
	start := lo - 128
	if start < 0 {
		start = 0
	}
	end := hi + 128
	if end > lspPiQ13 {
		end = lspPiQ13
	}
	for omega := start; omega <= end; omega++ {
		diff := absInt32(qi - int32(lsfToLSP(int16(omega))))
		tieBreak := (qi >= 0 && omega > bestOmega) || (qi < 0 && omega < bestOmega)
		if diff < bestDiff || (diff == bestDiff && tieBreak) {
			bestDiff = diff
			bestOmega = omega
		}
	}
	return int16(bestOmega)
}

// LSPToLSF converts the 10 LSP cosines q[0..9] (Q15) into the 10 LSF
// angles ω[0..9] (Q13) per ITU-T G.729 §3.2.5 ω_i = arccos(q_i).
// Caller owns both arrays. I3 / I4: pure, zero allocation, no panic.
func LSPToLSF(q *[10]int16, omega *[10]int16) {
	for i := 0; i < 10; i++ {
		omega[i] = lspToLSF(q[i])
	}
}

func absInt32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
