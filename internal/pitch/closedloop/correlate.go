package closedloop

import "github.com/hunydev/g729/internal/fixed"

// PitchMinInt and PitchMaxInt are the integer-only adaptive-codebook
// delay bounds per ITU-T G.729 Annex A §A.3.7 (G729E.txt
// lines 2129–2131): "a fractional pitch delay T1 ... in the range
// [19⅔, 84⅔] and integers only in the range [85, 143]". The
// integer-lag closed-loop search itself rounds the lower fractional
// bound up to the nearest integer (20). The upper bound 143 is the
// maximum lag the past-excitation buffer must support.
const (
	PitchMinInt = 20
	PitchMaxInt = 143
)

// BackwardFilter computes the 40-sample backward-filtered target
// signal xb(n) per ITU-T G.729 Annex A §A.3.7 eq. A.7
// (G729E.txt lines 2153–2156):
//
//	"xb(n) is the backward filtered target signal (correlation
//	 between x(n) and the impulse response h(n))."
//
// Concretely, applying the standard derivation
// Σ_n x(n)·yk(n) = Σ_n xb(n)·u(n−k) where yk(n) = (u*h)(n) yields
//
//	xb(n) = Σ_{m=n..L−1} x(m)·h(m − n),   n = 0,...,L−1   (L = 40)
//
// i.e. the time-reversed correlation of x with h truncated to the
// subframe.
//
// Q-format. x is Q0 (TG-1 convention) and h is Q12 (HI-1
// convention). The pointwise product accumulates in Q12; we
// arithmetically right-shift by 12 to land xb back in Q0 so the
// downstream SearchInteger correlation Σ xb·u keeps both factors in
// the same scale. Saturation to Word16 protects pathological
// inputs; for ITU-magnitude signals the result fits comfortably.
//
// I3 / I4: pure (writes only through xb), zero allocation.
func BackwardFilter(x, h *[SubframeLen]int16, xb *[SubframeLen]int16) {
	for n := 0; n < SubframeLen; n++ {
		var acc int64
		for m := n; m < SubframeLen; m++ {
			acc += int64(x[m]) * int64(h[m-n])
		}
		xb[n] = saturateInt64ToInt16(acc >> 12)
	}
}

// SearchInteger maximises the numerator-only closed-loop pitch
// criterion of ITU-T G.729 Annex A §A.3.7 eq. A.7
// (G729E.txt lines 2151–2156):
//
//	RN(k) = Σ_{n=0..39} x(n)·yk(n) = Σ_{n=0..39} xb(n)·u(n − k)
//
// over an integer search range that depends on the subframe index:
//
//   - sub = 0 (first subframe): k ∈ Subframe1Window(centre), where
//     centre is the open-loop pitch Top (§A.3.7 line 2167: "the
//     search range is limited around a preselected value"). The
//     seven-lag window follows §3.7's tmin/tmax boundary adaptation.
//
//   - sub = 1 (second subframe): k ∈ Subframe2Window(centre), where
//     centre is the integer part of the first-subframe lag T1, per
//     §4.1.3 (G729E.txt lines 1512–1523). This is the integer search
//     window; the two extra P2 codepoints are fractional boundary delays,
//     not integer-search lags.
//
// API note (CL-2 decision): the existing single-entry SearchInteger
// signature was preserved by activating the previously-reserved sub
// parameter rather than introducing a parallel SearchIntegerInRange.
// Justification: keeps the subframe-dispatch concern co-located with
// the search itself (one symbol per algorithmic stage), and the
// encoder's per-subframe driver (INT-0) calls SearchInteger
// uniformly with sub = subframeIndex. A free SearchIntegerInRange
// helper can still be extracted later if FR-2 needs to evaluate the
// integer search at an arbitrary window.
//
// exc is the past-excitation ring buffer ordered chronologically:
// the buffer is anchored so u(0) = exc[len(exc) − SubframeLen], hence
// u(−1) = exc[len(exc) − SubframeLen − 1] and the past spans
// exc[len(exc) − SubframeLen − PitchMaxInt : len(exc) − SubframeLen].
// The trailing SubframeLen samples (exc[len(exc) − SubframeLen :
// len(exc)]) hold the LP-residual extension u(0..39) per §A.3.7
// line 2161, enabling the short-pitch case k < SubframeLen. The
// formula u(n − k) = exc[len(exc) − SubframeLen − k + n] is valid
// uniformly for all (k, n) ∈ [PitchMinInt, PitchMaxInt] × [0, 39]
// when len(exc) ≥ PitchMaxInt + SubframeLen = 183. Fractional
// candidates near the maximum lag need additional older history for
// the b30 interpolation taps; the encoder therefore prepends Linter
// extra samples while preserving the same u(0) anchor.
//
// Q-format. xb and exc are Word16 (Q0). The accumulator keeps the
// ITU implicit ×2 product scaling (cf. openloop.correlate), but uses
// int64 internally so the argmax follows the mathematical RN(k) in
// eq. A.7 rather than a saturated Word32 surrogate. The returned RNbest
// is saturated to Word32 for the legacy diagnostic surface only.
// Tie-break on equal RN(k) favours the lower k via strict ">" comparison,
// matching the openloop §A.3.4 line 2110 "favouring the delays with the
// values in the lower range" convention.
//
// I3 / I4: pure (reads xb / exc), zero allocation.
func SearchInteger(xb *[SubframeLen]int16, exc []int16, centre int16, sub int) (intLag int16, RNbest int32) {
	var kMin, kMax int
	if sub == 0 {
		tmin, tmax := Subframe1Window(centre)
		kMin = int(tmin)
		kMax = int(tmax)
	} else {
		tmin, tmax := Subframe2Window(centre)
		kMin = int(tmin)
		kMax = int(tmax)
	}

	intLag = int16(kMin)
	best := int64(-1 << 63)
	base := len(exc) - SubframeLen
	for k := kMin; k <= kMax; k++ {
		var acc int64
		excBase := base - k
		for n := 0; n < SubframeLen; n++ {
			acc += 2 * int64(xb[n]) * int64(exc[excBase+n])
		}
		if acc > best {
			best = acc
			intLag = int16(k)
		}
	}
	return intLag, saturateInt64ToInt32(best)
}

func saturateInt64ToInt32(v int64) int32 {
	if v > int64(fixed.Max32) {
		return fixed.Max32
	}
	if v < int64(fixed.Min32) {
		return fixed.Min32
	}
	return int32(v)
}
