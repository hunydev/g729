// Package pitch implements the G.729 + Annex A decoder's adaptive
// codebook: pitch delay reconstruction from the 14 pitch-related
// bits per frame (P1 8 bits, P0 1 bit, P2 5 bits) plus construction
// of the 40-sample adaptive codebook vector per subframe via
// 1/3-sample fractional interpolation on the past excitation signal.
//
// # Public API
//
//	CheckParity(p1, p0 uint8) bool
//	    Per ITU-T G.729 §3.7.2. Returns true when p0 matches the
//	    parity computed over P1's upper 6 bits.
//
//	DecodeDelaySubframe1(p1 uint8) (tInt, tFrac int)
//	    Per §3.7.1. tInt ∈ [19, 143], tFrac ∈ {-1, 0, 1} at 1/3
//	    sample resolution. The 1/3-range covers [19, 84 2/3] with
//	    full fractional granularity; the integer-only range covers
//	    [85, 143].
//
//	DecodeDelaySubframe2(p2 uint8, t1Int int) (tInt, tFrac int)
//	    Per §3.7.1. Relative encoding around int(T1) (the integer
//	    truncation of subframe-1's fractional delay) with a ±5/3
//	    sample window at 1/3 resolution.
//
//	AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16)
//	    Per §3.8 / §4.1.4. Writes v[0..39] from pastExc at the
//	    specified delay, using the 1/3-sample FIR interpolator from
//	    tables.PitchInterpFIR for fractional offsets. Integer short
//	    pitch extends by periodicity when tInt < 40; fractional short
//	    pitch evaluates current-subframe taps from previously generated
//	    adaptive-vector samples.
//
// # Past excitation convention
//
// pastExc represents past excitation in chronological order.
// pastExc[len-1] is the most recent sample (one before the current
// subframe's first output). For integer-delay calls the caller must
// supply at least tInt + L_SUBFR samples of history; for fractional
// calls the FIR also reaches forward by Linter+1 taps, so the
// simplest sufficient condition is len(pastExc) ≥ tInt + L_SUBFR +
// Linter (worst case). Phase 1g's ring buffer is sized accordingly.
//
// # State ownership
//
// This package holds no state. The past-excitation ring buffer is
// owned by the top-level decoder (Phase 1g), which updates it each
// subframe from the sum of adaptive and fixed codebook contributions
// scaled by the decoded gains.
//
// # Numerical contract
//
//	Indices:     raw integers from the bitstream.
//	Delays:      tInt ∈ [19, 143], tFrac ∈ {-1, 0, 1}.
//	FIR taps:    Q15 int16, from tables.PitchInterpFIR.
//	pastExc, v:  Q0 int16 (excitation domain).
//
// # Scratch-from-spec
//
// Algorithm derived from ITU-T G.729 §3.7 / §4.1.3 / §4.1.4 and
// Annex A. The b30 FIR coefficient table is the only data initializer
// transcribed from the ITU reference distribution under the
// merger-doctrine exception (numeric constants only — no algorithmic
// code consulted). Every arithmetic step routes through
// internal/fixed.
//
// # Concurrency
//
// All functions are pure and safe for concurrent use. The caller
// owns all state.
package pitch
