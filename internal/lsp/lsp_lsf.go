package lsp

import "github.com/exedev/g729/internal/tables"

// lspToLSF is the inverse of lsfToLSP per ITU-T G.729 §3.2.5: given
// a Q15 LSP value q = cos(ω), return the corresponding Q13 LSF
// ω ∈ [0, π]. The inverse is realized by binary search on the
// monotone non-increasing tables.CosLSP plus linear interpolation
// inside the located cell, mirroring the forward map's
// piecewise-linear segments exactly.
//
// Numerical contract:
//
//	q clamped to [CosLSP[64], CosLSP[0]] (≈ [-1, +1] in Q15);
//	  outside that range the routine returns 0 or lspMaxOmega.
//	cell index idx is the largest k with CosLSP[k] >= q
//	  (so CosLSP[idx] >= q > CosLSP[idx+1] for q strictly inside).
//	frac = (q - c0) * lspStep / (c1 - c0)  in Q13 LSB units;
//	  this is the algebraic inverse of the forward
//	  interp = c0 + ((c1 - c0)·frac) / lspStep used by lsfToLSP.
//	ω = idx*lspStep + frac, clamped to [0, lspMaxOmega].
//
// I3 / I4: pure function, no allocation, no panic.
func lspToLSF(q int16) int16 {
	qi := int32(q)

	if qi >= int32(tables.CosLSP[0]) {
		return 0
	}
	if qi <= int32(tables.CosLSP[64]) {
		return int16(lspMaxOmega)
	}

	// Binary search for the largest idx in [0, 64] with CosLSP[idx] >= q.
	lo, hi := 0, 64
	for hi-lo > 1 {
		mid := (lo + hi) >> 1
		if int32(tables.CosLSP[mid]) >= qi {
			lo = mid
		} else {
			hi = mid
		}
	}

	c0 := int32(tables.CosLSP[lo])
	c1 := int32(tables.CosLSP[lo+1])

	// (c1 - c0) is strictly negative inside the table interior since
	// CosLSP is strictly monotone non-increasing on [0, 64]; the
	// outer clamp above has already excluded the only flat tie point.
	frac := ((qi - c0) * lspStep) / (c1 - c0)

	omega := int32(lo)*lspStep + frac
	if omega < 0 {
		omega = 0
	}
	if omega > lspMaxOmega {
		omega = lspMaxOmega
	}
	return int16(omega)
}

// LSPToLSF converts the 10 LSP cosines q[0..9] (Q15) into the 10
// LSF angles ω[0..9] (Q13) per ITU-T G.729 §3.2.5 ω_i = arccos(q_i).
// Caller owns both arrays. I3 / I4: pure, zero allocation, no panic.
func LSPToLSF(q *[10]int16, omega *[10]int16) {
	for i := 0; i < 10; i++ {
		omega[i] = lspToLSF(q[i])
	}
}
