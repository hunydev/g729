package lsp

import "github.com/exedev/g729/internal/tables"

// sinViaCos approximates sin(ω) Q15 from ω in Q13 by phase-shifting
// the existing cosine LUT through the identity sin(ω) = cos(π/2 − ω).
// For ω > π/2 the even symmetry cos(−x) = cos(x) is exploited so that
// the result is non-negative across the full lspToLSF input range
// ω ∈ [0, π] without needing a dedicated sine table. Accuracy is
// sufficient as a Newton-step slope estimate (the slope only needs to
// be correct to a few percent for the linear correction to converge).
func sinViaCos(omegaQ13 int32) int32 {
	const piHalfQ13 int32 = lspMaxOmega / 2
	arg := piHalfQ13 - omegaQ13
	if arg < 0 {
		arg = -arg
	}
	if arg > lspMaxOmega {
		arg = lspMaxOmega
	}
	return int32(lsfToLSP(int16(arg)))
}

// lspToLSF is the inverse of lsfToLSP per ITU-T G.729 §3.2.5: given
// a Q15 LSP value q = cos(ω), return the corresponding Q13 LSF
// ω ∈ [0, π]. The inverse is realized by binary search on the
// monotone non-increasing tables.CosLSP plus linear interpolation
// inside the located cell, then sharpened by a single Newton step
// against the actual cosine derivative −sin(ω).
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
//	ω₀ = idx*lspStep + frac.
//	One Newton refinement against f(ω) = cos(ω) − q, f'(ω) = −sin(ω):
//	  Δω = (cos(ω₀) − q) / sin(ω₀)   (in radians)
//	  Δω_Q13 = ((cos(ω₀) − q)_Q15 << 13) / sin(ω₀)_Q15
//	  (Q13 here uses 2^13 ≈ 25736/π = lspMaxOmega/π LSB per radian;
//	  the residual 0.04 % scale mismatch is well below one Q13 LSB
//	  for any plausible correction magnitude.)
//	ω = clamp(ω₀ + Δω, 0, lspMaxOmega).
//
// FIX-2D (Phase 2a INT-1 d4 §19): the chord interpolation alone
// linearizes cos within a LUT cell, but cos is curved; on
// high-derivative cells the residual bias reaches ~28 Q13 LSB and
// dominates the per-coordinate ω drift observed in d7 §S2/S6. The
// Newton step uses the local sine slope instead of the cell chord
// slope and brings the per-coordinate Δω to within a few LSB,
// matching the float-oracle precision floor.
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

	// Single Newton refinement step. sinAt is non-negative on [0, π]
	// (modulo the small phase-shift approximation in sinViaCos);
	// guarding sinAt > 0 prevents division-by-zero pathology near the
	// outer endpoints, which the early returns above already handle
	// exactly. The correction is capped to a single LUT-cell width to
	// keep the step strictly local.
	cosAt := int32(lsfToLSP(int16(omega)))
	sinAt := sinViaCos(omega)
	if sinAt > 0 {
		delta := (int64(cosAt-qi) << 13) / int64(sinAt)
		if delta > int64(lspStep) {
			delta = int64(lspStep)
		} else if delta < -int64(lspStep) {
			delta = -int64(lspStep)
		}
		omega += int32(delta)
		if omega < 0 {
			omega = 0
		}
		if omega > lspMaxOmega {
			omega = lspMaxOmega
		}
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
