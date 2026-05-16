// Package postfilter implements the G.729 Annex A adaptive postfilter
// chain per ITU-T G.729 §A.4.2. It consumes the pre-postfilter synthesis
// s[40] from internal/synth, the LP filter coefficients a[11] (Q12), and
// the integer pitch delay t_int from internal/pitch, and produces
// postfiltered samples s_pf[40] ready for the output high-pass stage.
//
// # Pipeline
//
// Per ITU-T G.729 §A.4.2.1 / §A.4.2.2 / §A.4.2.3 / §A.4.2.4:
//
//  1. Bandwidth expansion:  a → aNum (γ_n ≈ 0.55), aDen (γ_d ≈ 0.70)
//  2. Residual FIR:         r(n) = Σ aNum[i]·s(n−i)
//  3. Pitch refinement:     T ∈ {t_int−1, t_int, t_int+1}, max cross-correlation
//  4. Long-term postfilter: r′(n) = (r(n) + g_l·r(n−T)) / (1 + g_l)
//  5. Tilt compensation:    s_tilt(n) = r′(n) − μ·r′(n−1)
//  6. Short-term synthesis: s_st(n) = s_tilt(n) − Σ aDen[i]·s_st(n−i)
//  7. Adaptive gain control: s_pf(n) = g_pf(n)·s_st(n) with smoothing
//
// Each stage carries its own state across subframes. A Postfilter's zero
// value is a valid reset state per §A.4.2 first-frame initialisation.
//
// # Numerical contract
//
//	Inputs:
//		a      — Q12 [11]int16, a[0] = 4096
//		t_int  — integer, ∈ [20, 143]
//		s      — Q0 [40]int16
//	Output:
//		s_pf   — Q0 [40]int16, saturated
//
// # Scratch-from-spec
//
// All coefficients and formulas derive from ITU-T G.729 §A.4.2 directly.
// No ITU reference C, bcg729, Sipro Lab, or any other existing G.729
// implementation was consulted for algorithmic code. Numerical primitives
// route through internal/fixed for ITU saturation semantics.
//
// # Concurrency
//
// Postfilter is not safe for concurrent use. One instance per decoder channel.
package postfilter
