package gainquant

// GpClipQ14 is the upper bound on the quantized adaptive-codebook gain
// ĝp under the §3.9.2 taming branch ("adaptive-codebook gain
// saturation under predicted-overflow conditions").
//
// Spec §3.9.2 narrates the branch but does not pin a numeric ceiling;
// OQ-TAMING-THR resolves to 0.95 (Q14 = 15565) as the clean-room
// textbook-typical CELP taming threshold (Kondoz §10.5; Spanias
// §11.6). Revisitable at Phase 2f TAME.BIT byte-EQ.
const GpClipQ14 int16 = 15565

// TameEnergyThresholdQ0 is the trip energy for the §3.9.2 taming
// branch: when Σ oldExc[i]² (over the full 154-sample lookback) exceeds
// this value, an ĝp above GpClipQ14 is judged unsafe (predicted
// overflow when convolved into the long-term excitation memory) and
// clamped.
//
// OQ-TAMING-THR pins the energy threshold at 2^33 (≈ 8.59e9): low
// enough that sustained voiced speech energy trips it but high enough
// that quiescent / unvoiced segments do not. Boundary semantics are
// strict-greater (Σx² > threshold) so the threshold itself is
// non-tripping. Revisitable at Phase 2f TAME.BIT byte-EQ.
const TameEnergyThresholdQ0 int64 = 1 << 33

// Tame implements the §3.9.2 adaptive-codebook gain taming branch:
//
//  1. Compute the energy E = Σ oldExc[i]² over the long-term excitation
//     memory (caller-owned [154]int16).
//  2. If E > TameEnergyThresholdQ0 AND ĝp > GpClipQ14, clamp ĝp to
//     GpClipQ14; otherwise return ĝp unchanged.
//
// Sign discipline: §3.9.2 codebooks (GBK1 / GBK2) are non-negative, so
// ĝp is structurally ≥ 0; the clamp is a one-sided ceiling. Negative
// inputs (test fixtures or future codebook revisions) bypass the clamp
// since they cannot exceed the positive ceiling.
//
// Q-format: gpQ14 in / out; oldExc in Q0 (excitation samples per
// §A.3.10 eq. A.9 commit). Energy accumulator is int64 (exact for 154
// int16² sums: max 154·2^30 < 2^38, fits in int63).
//
// I4 (zero allocation): the only state is a stack-resident int64.
func Tame(gpQ14 int16, oldExc *[154]int16) int16 {
	var energy int64
	for i := 0; i < len(oldExc); i++ {
		s := int64(oldExc[i])
		energy += s * s
	}
	if energy > TameEnergyThresholdQ0 && gpQ14 > GpClipQ14 {
		return GpClipQ14
	}
	return gpQ14
}
