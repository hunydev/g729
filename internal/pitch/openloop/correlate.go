package openloop

import "github.com/exedev/g729/internal/fixed"

// correlate computes the decimated correlation of eq. A.4
// (G729E.txt §A.3.4 lines 2089-2092):
//
//	R(k) = Σ_{n=0..39} sw(2n) · sw(2n − k)
//
// over the 223-sample wsp window where wsp[143..222] holds the
// current 80-sample frame (sw(0..79)) and wsp[0..142] holds the
// 143-sample sw history (max lag k = 143 per §A.3.4 line 2098).
// Indexing convention pinned by the Phase 2b sub-plan §6 OL-1:
// "n = 0" of eq. A.4 refers to the first sample of the current
// frame, so
//
//	sw(2n)     = wsp[143 + 2n]      n = 0..39  (indices 143..221)
//	sw(2n − k) = wsp[143 + 2n − k]  k ∈ [20,143]
//
// The function returns the lag k ∈ [kMin, kMax] that maximizes R(k)
// together with the corresponding R(k) value. §A.3.4 leaves the
// sign of R(k) implicit; per Phase 2a I1 discipline the operational
// pin is "negative R(k) is treated as 0 for selection" — only
// positive-correlation (well-correlated) candidates compete. If no
// positive R(k) exists in [kMin, kMax] the function returns
// lag = kMin and rsq = 0. Tie-breaking selects the smallest k via
// strict ">" so the lower-delay candidate wins, which is consistent
// with §A.3.4 line 2110 ("favouring the delays with the values in
// the lower range").
//
// Q-format. wsp is int16 Q0; the inner accumulator is Word32
// (int32). fixed.LMac performs acc + 2·a·b with saturation at both
// the multiplication stage (only when a = b = Min16, see fixed.LMult)
// and the addition stage (LAdd), so the returned rsq carries the
// implicit ×2 product semantics standard to G.729 fixed-point. OL-2
// will apply the same ×2 scaling to the eq. A.5 energy denominator
// so the R/E ratio is invariant under the shared scale.
//
// I3 / I4: pure (reads only wsp), zero allocation.
func correlate(wsp *[223]int16, kMin, kMax int) (lag int16, rsq int32) {
	lag = int16(kMin)
	rsq = 0
	for k := kMin; k <= kMax; k++ {
		var acc fixed.Word32
		for n := 0; n < 40; n++ {
			acc = fixed.LMac(acc, wsp[143+2*n], wsp[143+2*n-k])
		}
		if acc > rsq {
			rsq = acc
			lag = int16(k)
		}
	}
	return lag, rsq
}
