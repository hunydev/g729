package openloop

import "github.com/hunydev/g729/internal/fixed"

// correlateAt returns R(k) for a single lag k of eq. A.4 (G729E.txt
// §A.3.4 lines 2089-2092):
//
//	R(k) = Σ_{n=0..39} sw(2n) · sw(2n − k)
//
// using the same wsp indexing convention as OL-1's correlate
// (sw(2n) = wsp[143+2n], sw(2n − k) = wsp[143+2n − k]). This per-lag
// accessor is needed by pickBestInRange because OL-1's correlate is a
// max-R-only kernel that scans a range; the eq. A.5 selection in OL-3
// requires R(k) AND E(k) per-lag to feed compareNormalized. Per the
// OL-2 closure note we expose this helper alongside `correlate` rather
// than refactor OL-1.
//
// Q-format and saturation match OL-1 exactly: fixed.LMac yields
// acc + 2·a·b with the same implicit ×2 product scale, which cancels
// against the matching ×2 OL-1 documents in correlate.go. Pure,
// zero-allocation.
func correlateAt(wsp *[223]int16, k int) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		acc = fixed.LMac(acc, wsp[143+2*n], wsp[143+2*n-k])
	}
	return acc
}

// pickBestInRange returns the lag k ∈ [kMin, kMax] that maximizes the
// eq. A.5 normalized open-loop pitch score R(k)²/E(k), together with
// its R(k) and E(k) Word32 values for downstream OL-4 sub-multiple
// merging.
//
// The three §A.3.4 delay regions (lines 2094-2097) get two scan
// strategies:
//
//   - [20,39] and [40,79]: full-stride scan over every k via
//     correlateAt + energy + compareNormalized (one (R,E) pair per
//     candidate, single comparator call per replacement).
//
//   - [80,143]: per §A.3.4 lines 2113-2114 verbatim — "in the third
//     delay region [80, 143] only the correlations at the even delays
//     are computed in the first pass, then the delays at ±1 of the
//     selected even delay are tested." The first pass scans
//     k ∈ {80, 82, …, 142} and the refinement evaluates
//     {best_even − 1, best_even, best_even + 1} clamped to [80, 143].
//
// Tie-break. Both passes scan descending and update on compareNormalized's
// ≥ relation, so when two lags tie the later (smaller-k) iteration
// replaces the earlier (larger-k) holder. This realises §A.3.4 line 2110
// "favouring the delays with the values in the lower range" at the
// per-range granularity (the inter-range lift is OL-4's responsibility).
//
// I3 / I4: pure (reads only wsp, calls only pure helpers), zero
// allocation on every path.
func pickBestInRange(wsp *[223]int16, kMin, kMax int) (lag int16, rsq fixed.Word32, en fixed.Word32) {
	if kMin == 80 && kMax == 143 {
		return pickBestEvenWithRefinement(wsp)
	}
	return pickBestFullScan(wsp, kMin, kMax)
}

// pickBestFullScan implements the full-stride scan for ranges [20,39]
// and [40,79] (§A.3.4 lines 2095-2096). Descending scan with ≥ update
// gives lower-lag tie-break.
func pickBestFullScan(wsp *[223]int16, kMin, kMax int) (lag int16, rsq, en fixed.Word32) {
	lag = int16(kMax)
	rsq = correlateAt(wsp, kMax)
	en = energy(wsp, kMax)
	for k := kMax - 1; k >= kMin; k-- {
		rk := correlateAt(wsp, k)
		ek := energy(wsp, k)
		if compareNormalized(rk, ek, rsq, en) {
			lag, rsq, en = int16(k), rk, ek
		}
	}
	return
}

// pickBestEvenWithRefinement implements the §A.3.4 lines 2113-2114
// even-first scan over [80,143] followed by the ±1 refinement around
// the even-pass winner.
func pickBestEvenWithRefinement(wsp *[223]int16) (lag int16, rsq, en fixed.Word32) {
	// Even-first pass: k ∈ {80, 82, …, 142}. Descending so on tie
	// the lower lag wins.
	lag = 142
	rsq = correlateAt(wsp, 142)
	en = energy(wsp, 142)
	for k := 140; k >= 80; k -= 2 {
		rk := correlateAt(wsp, k)
		ek := energy(wsp, k)
		if compareNormalized(rk, ek, rsq, en) {
			lag, rsq, en = int16(k), rk, ek
		}
	}
	// ±1 refinement around the even winner; clamp to [80, 143].
	bestEven := int(lag)
	hi := bestEven + 1
	if hi > 143 {
		hi = 143
	}
	lo := bestEven - 1
	if lo < 80 {
		lo = 80
	}
	// Re-scan descending [hi..lo] with ≥ update so a lower-lag tie
	// (e.g. R(109)·E(110) ties R(110)·E(109)) selects the lower lag,
	// per §A.3.4 line 2110.
	lag = int16(hi)
	rsq = correlateAt(wsp, hi)
	en = energy(wsp, hi)
	for k := hi - 1; k >= lo; k-- {
		rk := correlateAt(wsp, k)
		ek := energy(wsp, k)
		if compareNormalized(rk, ek, rsq, en) {
			lag, rsq, en = int16(k), rk, ek
		}
	}
	return
}
