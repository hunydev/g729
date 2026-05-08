package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// TestPhase1o_D3_S4_R1_SynthRoundingBoundary — Phase 1o D-3 S-4 R-1
// rounding-boundary investigation for the TAME f0 sf0 sample-1
// off-by-2.
//
// Per S-3 (commit bd37512) the production trace shows:
//
//	u[0..7] = [1, 1, 1, 1, 0, 0, 0, 0]
//	s[0..7] = [1, 0, 1, 1, -1, 0, 0, 0]
//	want[0..7] (= TAME.PST/2 desired hpOut) = [1, 1, 0, 0, 0, 0, 0, 0]
//
// The S-3 hand-arithmetic at sample 1 with a (Q12) =
// [4096 2108 1500 -137 399 -135 156 -55 301 256 189]:
//
//	lTemp = LMult(u[1]=1, a[0]=4096)             = 8192    (Q13)
//	lTemp = LMsu(lTemp, a[1]=2108, s[0]=1)       = 3976    (Q13)
//	lTemp = LShl(lTemp, 3)                       = 31808   (Q16)
//	s[1]  = Round(lTemp) = (31808+0x8000)>>16    = 0       (truncates 0.485 → 0)
//
// The real-valued reference is u[1] − a[1]·s[0]/4096 = 1 − 2108/4096
// = 0.4854, which round-half-to-nearest sends to 0. So PRODUCTION is
// arithmetically correct under §4.1.6 eq. (77) and the basop rounding
// definition (G729E §6.2.1, Table 10: Word16 round(Word32) = extract_h(L_add(L,
// 0x00008000))).
//
// This test enumerates the R-1 sub-hypothesis family and refutes each
// via direct arithmetic, then re-ranks for S-5. It is a diagnostic
// (t.Logf only) following the S-2 / S-3 escape-hatch convention so
// the suite stays GREEN while the defect remains live.
//
// SPEC ANCHORS:
//
//	§4.1.6 eq. (77):  ŝ(n) = u(n) − Σ_{i=1..10} âi·ŝ(n−i)
//	§6.2.1 Table 10:  Word16 round(Word32 L_var)  := extract_h(L_add(L_var, 0x00008000))
//	§6.2.1 Table 10:  Word32 L_shl(Word32, Word16) — saturating left shift
//	§6.2.1 Table 10:  Word32 L_mult(Word16, Word16) — 2·a·b with overflow only at Min16·Min16
//
// VERDICT: R-1 NO-FIX. Re-rank R-2 (LP interpolation §4.1.5) and R-3
// (BuildExcitation rounding §4.1.6 eq. (75)) for S-5.
func TestPhase1o_D3_S4_R1_SynthRoundingBoundary(t *testing.T) {
	const (
		uN_1   = int16(1)    // u[1] from production trace (S-3)
		s0     = int16(1)    // s[0] from production trace (S-3)
		a0     = int16(4096) // Q12 unity, §6.2.1
		a1     = int16(2108) // sf-0 LP coefficient (Q12), production trace
		shl3   = int16(3)    // §A.4.1 Q13→Q16 (a[i] is Q12, L_mult adds 1, total Q13; +3 → Q16)
		half16 = fixed.Word32(0x00008000)
	)

	t.Logf("=== Phase 1o D-3 S-4 R-1: synth onePass rounding boundary ===")
	t.Logf("Inputs:  u[1]=%d  s[0]=%d  a[0]=%d  a[1]=%d", uN_1, s0, a0, a1)
	t.Logf("Real-valued y[1] = u[1] − a[1]·s[0]/a[0] = 1 − 2108/4096 = %.6f", 1.0-2108.0/4096.0)

	// Production path (canonical §4.1.6 eq. (77) + G.191 basops).
	lAcc := fixed.LMult(fixed.Word16(uN_1), fixed.Word16(a0))
	t.Logf("step 1: LMult(u[1], a[0]) = %d  (Q13: 2·1·4096 = 8192)", lAcc)

	lAcc = fixed.LMsu(lAcc, fixed.Word16(a1), fixed.Word16(s0))
	t.Logf("step 2: LMsu(_, a[1], s[0]) = %d  (Q13: 8192 − 2·2108·1 = 3976)", lAcc)

	lShifted := fixed.LShl(lAcc, shl3)
	t.Logf("step 3: LShl(_, 3) = %d  (Q16: 3976·8 = 31808)", lShifted)

	rounded := fixed.Round(lShifted)
	t.Logf("step 4: Round(_) = %d  (extract_h(31808 + 0x8000) = extract_h(64576) = 0)", rounded)

	if rounded != 0 {
		t.Errorf("production trace mismatch — rounded=%d, expected 0", rounded)
	}

	// ---------------------------------------------------------------
	// R-1a: Round bias 0x00008000 vs 0x00010000 (round-half-up to next).
	// G.191 STL Table 10 fixes the bias at 0x00008000. With 0x00010000
	// the result would lift to 1, but that VIOLATES the basop spec.
	// ---------------------------------------------------------------
	withBigBias := fixed.ExtractH(fixed.LAdd(lShifted, 0x00010000))
	t.Logf("R-1a (bias=0x10000, NON-spec): ExtractH(31808 + 0x10000) = %d  → REFUTED (spec mandates 0x8000)", withBigBias)

	// ---------------------------------------------------------------
	// R-1b: LShl(4) instead of LShl(3) — implies Q12 not Q13.
	// Refuted by the existing trivial-passthrough contract test
	// (synth.TestQFormatContract_FilterSubframeAcceptsAOneQ12) where
	// a=[4096, 0, …] and u[n]=n+1 yields s[n]=n+1. That ONLY holds
	// with Round(LShl(L_mult(u[n], 4096), 3)). LShl(4) would double
	// every output sample; LShl(2) would zero them.
	// ---------------------------------------------------------------
	withShl4 := fixed.Round(fixed.LShl(lAcc, 4))
	t.Logf("R-1b (LShl=4, NON-spec):     Round(LShl(3976, 4)) = Round(63616) = %d  → BUT breaks trivial-passthrough → REFUTED", withShl4)

	// Demonstrate that LShl(4) breaks sample 0 of the contract test:
	lAcc0 := fixed.LMult(1, a0) // u[0]=1, no past
	s0New := fixed.Round(fixed.LShl(lAcc0, 4))
	t.Logf("           LShl(4) sample-0 contract: u[0]=1 → s[0]=%d (want 1) → contract violated", s0New)

	// ---------------------------------------------------------------
	// R-1c: accumulator initial value adds an implicit positive bias.
	// §4.1.6 eq. (77) is exactly ŝ(n) = u(n) − Σ a_i·ŝ(n−i); there is
	// no constant offset term. Any added bias would also affect sample
	// 0 (which currently matches want=1 → final=2). REFUTED by the
	// sample-0 trace match.
	// ---------------------------------------------------------------
	t.Logf("R-1c (implicit bias term):    no spec basis — eq. (77) has no constant; sample-0 already matches want → REFUTED")

	// ---------------------------------------------------------------
	// R-1d: equation form s(n) = u(n)·a[0] − Σ a[i]·s(n-i) vs alt form.
	// Eq. (77) explicit. Implementations that use L_deposit_h(u[n])
	// + Σ a[i]·s[n-i] (Q16 formulation) are byte-equivalent — verified:
	// ---------------------------------------------------------------
	lAccAlt := fixed.LMsu(0, a1, s0)                             // start at 0, accumulate -a·s in Q13
	lShiftedAlt := fixed.LShl(lAccAlt, 3)                        // Q13 → Q16
	lShiftedAlt = fixed.LAdd(lShiftedAlt, fixed.LDepositH(uN_1)) // add u[1] in Q16
	roundedAlt := fixed.Round(lShiftedAlt)
	t.Logf("R-1d (alt-form L_deposit_h):  Round(_) = %d (matches production %d) → byte-EQ → REFUTED", roundedAlt, rounded)

	// ---------------------------------------------------------------
	// R-1e: a[0] coefficient interpretation. a[0]=4096 (Q12 unity) is
	// asserted by tables/lpc.go and the trivial-passthrough contract
	// test. Any other a[0] (e.g., 8192 Q13) would break the contract.
	// ---------------------------------------------------------------
	t.Logf("R-1e (a[0]≠4096 Q12):         contradicts tables/lpc.go and synth qformat_contract_test → REFUTED")

	// ---------------------------------------------------------------
	// R-1f: pre-shift on u[n] before accumulation. u is Q0 by
	// construction (synth.BuildExcitation doc). Any pre-shift on u
	// would also affect sample 0 (already correct). REFUTED.
	// ---------------------------------------------------------------
	t.Logf("R-1f (u[n] pre-shift):        sample-0 trace already matches want → no pre-shift mismatch → REFUTED")

	// ---------------------------------------------------------------
	// G.191 BASOP SPOT-CHECK — verify our fixed primitives are byte-EQ
	// to the ITU-T G.729 §6.2.1 Table 10 definitions, for the exact
	// values that appear in the sample-1 trace.
	// ---------------------------------------------------------------
	t.Logf("--- G.191 basop spot-check ---")
	if got := fixed.LMult(1, 4096); got != 8192 {
		t.Errorf("LMult(1, 4096)=%d, want 8192 (G.191: 2·a·b)", got)
	}
	if got := fixed.LMult(2108, 1); got != 4216 {
		t.Errorf("LMult(2108, 1)=%d, want 4216", got)
	}
	if got := fixed.LSub(8192, 4216); got != 3976 {
		t.Errorf("LSub(8192, 4216)=%d, want 3976", got)
	}
	if got := fixed.LShl(3976, 3); got != 31808 {
		t.Errorf("LShl(3976, 3)=%d, want 31808", got)
	}
	if got := fixed.LAdd(31808, fixed.Word32(half16)); got != 64576 {
		t.Errorf("LAdd(31808, 0x8000)=%d, want 64576", got)
	}
	if got := fixed.ExtractH(64576); got != 0 {
		t.Errorf("ExtractH(64576)=%d, want 0", got)
	}
	t.Logf("All basop primitives match G.191 STL definitions for the sample-1 trace.")

	// ---------------------------------------------------------------
	// CONCLUSION + RE-RANK FOR S-5.
	// ---------------------------------------------------------------
	t.Logf("")
	t.Logf("=== R-1 verdict: NO-FIX ===")
	t.Logf("Synth onePass implements §4.1.6 eq. (77) byte-EQ to G.191 basops.")
	t.Logf("Real-valued y[1] = 0.485 rounds to 0 by spec; production output is arithmetically correct.")
	t.Logf("The off-by-2 at sample 1 must originate UPSTREAM of synth — either in")
	t.Logf("  • the LP coefficient a[1]=2108 fed to syn_filt (R-2: §4.1.5 LP interp), or")
	t.Logf("  • the excitation u[1]=1 fed to syn_filt   (R-3: §4.1.6 eq. (75) BuildExcitation).")
	t.Logf("")
	t.Logf("Sensitivity check (what input perturbation lifts s[1] to 1?):")
	for _, alt := range []struct {
		label string
		uVal  int16
		s0Val int16
		a1Val int16
	}{
		{"u[1]=2 (R-3 candidate, +1 LSB)", 2, 1, 2108},
		{"a[1]=1500 (R-2 candidate, sf-1 a[2])", 1, 1, 1500},
		{"s[0]=0 (R-1d-bis, refuted by §3.10 init)", 1, 0, 2108},
	} {
		acc := fixed.LMult(fixed.Word16(alt.uVal), fixed.Word16(a0))
		acc = fixed.LMsu(acc, fixed.Word16(alt.a1Val), fixed.Word16(alt.s0Val))
		out := fixed.Round(fixed.LShl(acc, 3))
		t.Logf("  %s → s[1]=%d", alt.label, out)
	}
	t.Logf("")
	t.Logf("Recommended S-5 dispatch (rank order):")
	t.Logf("  1. R-3 BuildExcitation rounding (§4.1.6 eq. (75)) — gc·c (Q26→Q15 via LShr(11)) bias")
	t.Logf("  2. R-2 LP coefficient interpolation routing (§4.1.5) — sf-0 a[] vs sf-1 a[]")
	t.Logf("")
	t.Logf("Cumulative refutation budget: 3 / 5 consumed (S-2 H-1, S-3 H-11, S-4 R-1).")
}
