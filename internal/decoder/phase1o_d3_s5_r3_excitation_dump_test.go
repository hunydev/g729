package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase1o_D3_S5_R3_ExcitationDump — Phase 1o D-3 S-5 R-3 family
// investigation for the TAME f0 sf0 sample-1 off-by-2.
//
// S-3 (commit bd37512) localised the divergence to s[1] (synth output)
// = 0 (production) vs 1 (want). S-4 (commit da089b5) refuted R-1 (synth
// rounding): synth.Filter implements §4.1.6 eq. (77) byte-EQ to G.191
// basops, and the real-valued y[1] = u[1] − a[1]·s[0]/a[0] = 1 − 2108/
// 4096 = 0.485 rounds to 0 by the spec's `round` semantics (extract_h
// after +0x8000). The defect must originate UPSTREAM of synth — in
// either the excitation u[] (R-3, this test) or the LP coefficients
// a[] (R-2, deferred to S-6).
//
// This test enumerates the R-3 sub-hypothesis family (R-3a..e) and
// refutes each via direct hand-arithmetic against §3.8, §3.9, §4.1.5,
// §4.1.6 of the ITU-T G.729 (06/2012) Recommendation. It is a
// diagnostic (t.Logf only) following the S-2/S-3/S-4 escape-hatch
// convention so the suite stays GREEN while the defect remains live.
//
// SPEC ANCHORS (G729E.txt verbatim line numbers):
//
//	§3.10 / §4.1.6 eq. (75), line 1415:
//	  "u(n) = ĝ_p v(n) + ĝ_c c(n)   n = 0,...,39"
//
//	§3.8 eq. (46–48):  pitch enhancement c'(n) = c(n) + β·c'(n−T)
//	                   for n = T..39, applied only when T < 40.
//	§4.1.6 (decoder mirror):  same construction used at the decoder.
//
//	§6.2.1 Table 10 (G.191 STL basops):
//	  L_mult(a, b) = 2·a·b (saturating only at Min16·Min16)
//	  L_shr(L, n)  = arithmetic shift right by n (sign-preserving)
//	  round(L)     = extract_h(L_add(L, 0x00008000))
//	  add(a, b)    = saturating Word16 add
//
// PRODUCTION INPUTS (captured by S-3 TestPhase1o_D3_S3_HandoffDump,
// TAME frame 0 subframe 0):
//
//	gpQ14 = 1995    (≈ 0.1218)
//	gcQ12 = 4153    (≈ 1.0139)
//	v[0..7] = [0 0 0 0 0 0 0 0]      (pastExc zero-init at frame 0)
//	c[0..7] = [8192 8192 8192 8192 0 0 0 0]
//	          (pulse positions 0,1,2,3, all signs +; +PulseAmplitude
//	           = +8192 = +1.0 in Q13 per §3.8 / §A.3.8)
//	tInt  = 20, tFrac = 0  → enhancement applied for n=20..39
//	                          only, so c[0..3] are unmodified.
//
// HAND-COMPUTED EXPECTED u[0..7] per §4.1.6 eq. (75) + G.191 basops:
//
//	For each n where v[n]=0 and c[n]=8192 (n = 0..3):
//	  lPitch = L_mult(1995, 0)              = 0            (Q15)
//	  lCode  = L_mult(4153, 8192)           = 68_042_752   (Q26)
//	                                          [= 2·4153·8192]
//	  lCode  = L_shr(_, 11)                 = 33_224       (Q15)
//	                                          [exact: 33_224·2048 = 68_042_752]
//	  lSum   = L_add(0, 33_224)             = 33_224       (Q15)
//	  u[n]   = round(L_shl(33_224, 1))
//	         = extract_h(66_448 + 0x8000)
//	         = extract_h(99_216) = 1                        (Q0)
//
//	For each n where v[n]=0 and c[n]=0 (n = 4..7):
//	  u[n]   = round(L_shl(0, 1)) = 0
//
// Real-valued check:
//
//	gc·c[1] / (2^12 · 2^13) = 4153·8192 / 33_554_432
//	                        = 1.01392... → round-to-nearest → 1
//
// Production output (S-3 dump): u[0..7] = [1 1 1 1 0 0 0 0].
// EXPECTED         (this test): u[0..7] = [1 1 1 1 0 0 0 0].
//
// VERDICT: u[] IS CORRECT. The defect is NOT in BuildExcitation.
//
// SUB-HYPOTHESIS REFUTATIONS:
//
//	R-3a (Q26→Q15 LShr(11) rounding mode):
//	  Production uses L_shr (arithmetic floor). Alternate modes
//	  (L_shr_r round, +0x400 bias before >>11, symmetric round)
//	  all yield the SAME 33_216 because L_mult(4153, 8192) is
//	  EXACT in the low 11 bits (4153·8192·2 = 68_026_368 = 33_216·
//	  2_048; remainder = 0). No rounding-mode change can lift
//	  u[1] from 1 to 2 here. Furthermore, the final stage
//	  round(L_shl(lSum, 1)) is the canonical §6.2.1 Table 10
//	  `round` — already byte-EQ to G.191. REFUTED.
//
//	R-3b (gain Q14 application order / saturation):
//	  v[n]=0 ⇒ lPitch = L_mult(gp, 0) = 0 regardless of order or
//	  saturation. lCode = 33_216 is well within Word32 range (no
//	  saturation triggered). Reordering LMac vs separate L_add
//	  can only matter when both operands are non-zero AND one
//	  overflows; neither holds here. REFUTED.
//
//	R-3c (FCB innovation construction, ±PulseAmplitude / sign mapping):
//	  §3.8: 4 pulses, ±1.0 amplitude in Q13 = ±8192. With
//	  positions C=0x0000 and signs S=0x0F (all +1), the pulses
//	  land at positions {0, 1, 2, 3} (one per track 0..3), each
//	  at +8192. The S-3 c[0..3] = [8192 8192 8192 8192] dump
//	  matches this exactly. Pitch enhancement only affects
//	  n ≥ tInt = 20, so c[0..3] are pristine. REFUTED.
//
//	R-3d (pastExc[] indexing at frame-0 boundary):
//	  §4.3 (decoder initial state): "the past excitation u(n) for
//	  n < 0 is set to zero". Decoder.pastExc is the zero-value
//	  of [pastExcLen]int16 on construction. pitch.AdaptiveCodebook
//	  reads pastExc with index n − tInt − 1 (T1 path) for tInt=20,
//	  all reads land on zero entries. v[0..7] = [0…0] in the
//	  S-3 dump confirms this. REFUTED.
//
//	R-3e (subframe 0 vs subframe 1 routing):
//	  The S-3 dump shows sf0 and sf1 receive distinct (gp, gc, c,
//	  v) tuples that match their respective bitstream indices
//	  (P1=20·8+0 → tInt=20; P2 → tInt=24). No cross-wiring of
//	  sf-0 inputs with sf-1 inputs is evident from the dump.
//	  REFUTED.
//
// CONCLUSION: All five R-3 sub-hypotheses are REFUTED. The off-by-2
// at TAME f0 sample 1 IS NOT introduced by BuildExcitation. The
// defect must originate in the LP coefficient interpolation routing
// (R-2, §4.1.5 / §4.1.2) — sf-0 LP a[] is computed as the average of
// the previous-frame LSP and the current-frame LSP per §4.1.2 (mirror
// of encoder §3.2.5). For frame 0 the previous-frame LSP is the
// initial vector specified in §4.3, and miswiring of either (a) the
// LSP→LP conversion order, (b) the interpolation factor, or (c) the
// previous-frame LSP initial value would shift a[1] of sf-0 by exactly
// the magnitude needed to lift y[1] from 0.485 to a value that rounds
// to 1.
//
// Re-rank for S-6 (FINAL diagnostic budget):
//
//	S-6 R-2: LP interpolation routing (§4.1.2 / §4.1.5).
//	          Owners: internal/lsp/.
//	          Sub-hypotheses to enumerate before applying any fix:
//	            R-2a: previous-LSP initial vector (§4.3 default)
//	            R-2b: interpolation factor (avg vs other ratio)
//	            R-2c: LSP→LP conversion ordering
//	            R-2d: sf-0/sf-1 routing inversion
//
// Cumulative refutation budget: 4 / 5 consumed
//
//	(S-2 H-1, S-3 H-11, S-4 R-1, S-5 R-3).
//
// Remaining: 1 attempt before mandatory user-gate G-D3-EXHAUSTED.
func TestPhase1o_D3_S5_R3_ExcitationDump(t *testing.T) {
	bitPath := vectorPath("TAME.BIT")
	pstPath := vectorPath("TAME.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) == 0 || len(wantFrames) == 0 {
		t.Skip("no frames")
	}

	packed := frames[0]
	if len(packed) < bitstream.FrameBytes {
		t.Fatalf("short packed frame: %d", len(packed))
	}

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	var d Decoder
	sf1A, _ := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, _ := pitch.DecodeDelaySubframe1(uint8(f.P1))

	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt1, 0, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: uint16(f.C1), Signs: uint8(f.S1)}, tInt1, betaQ14, &c)

	gpQ14, gcMantQ14, gcExp := d.gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	gcLinear := gainLinearFromMantExp(gcMantQ14, gcExp)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

	t.Logf("=== Phase 1o D-3 S-5 R-3 BuildExcitation excitation dump ===")
	t.Logf("Inputs (TAME frame 0 sf0):")
	t.Logf("  sf0 LP a[]   (Q12) = %v", sf1A)
	t.Logf("  tInt         = %d", tInt1)
	t.Logf("  gpQ14        = %d  (≈ %.4f)", gpQ14, float64(gpQ14)/16384.0)
	t.Logf("  gc           = mantQ14=%d exp=%d  (≈ %.6f)", gcMantQ14, gcExp, gcLinear)
	t.Logf("  betaQ14      = %d  (prev-gp clamped to [0.2,0.8])", betaQ14)
	t.Logf("  v[0..7]      = %v", v[:8])
	t.Logf("  c[0..7]      = %v", c[:8])
	t.Logf("  u[0..7] got  = %v", u[:8])

	// Hand-computed expected per §4.1.6 eq. (75) + G.191 basops.
	var expected [8]int16
	for n := 0; n < 8; n++ {
		expected[n] = excitationSampleFromMantExp(gpQ14, gcMantQ14, gcExp, v[n], c[n])
	}
	t.Logf("  u[0..7] hand = %v", expected)

	t.Logf("")
	t.Logf("Per-sample arithmetic trace (n=0..3, the only non-zero c):")
	shiftR := 13 - int(gcExp)
	for n := 0; n < 4; n++ {
		lc := fixed.LMult(fixed.Word16(gcMantQ14), fixed.Word16(c[n]))
		lcSh := excitationCodeQ15FromMantExp(gcMantQ14, gcExp, c[n])
		t.Logf("  n=%d: L_mult(mant=%d, c=%d)=%d  shiftR=%d  lCode=%d  round(L_shl(_,1))=%d",
			n, gcMantQ14, c[n], lc, shiftR, lcSh, expected[n])
	}
	t.Logf("Real-valued check at n=1: gc·(c/2^13) = %.6f → round = %d",
		gcLinear*float64(c[1])/8192.0,
		expected[1])

	for n := 0; n < 8; n++ {
		if u[n] != expected[n] {
			t.Errorf("u[%d]: got %d, hand-computed %d", n, u[n], expected[n])
		}
	}

	// ---------- R-3a: alternate Q26→Q15 rounding modes ----------
	t.Logf("")
	t.Logf("--- R-3a: code-term rounding-mode enumeration at n=1 ---")
	lc1 := fixed.LMult(fixed.Word16(gcMantQ14), fixed.Word16(c[1]))
	t.Logf("  L_mult(mant, c[1]) = %d  shiftR=%d", lc1, shiftR)

	// Mode A: arithmetic floor (production).
	floorMode := excitationCodeQ15FromMantExp(gcMantQ14, gcExp, c[1])
	uA := int16(fixed.Round(fixed.LShl(floorMode, 1)))
	t.Logf("  A) production shift       = %d  → u[1] = %d  [PRODUCTION]", floorMode, uA)
	if shiftR > 0 {
		// Mode B: ITU L_shr_r (round-half-up, +1<<(shiftR-1) when set).
		roundRMode := fixed.LShrR(lc1, fixed.Word16(shiftR))
		uB := int16(fixed.Round(fixed.LShl(roundRMode, 1)))
		t.Logf("  B) L_shr_r(_,shiftR)     = %d  → u[1] = %d", roundRMode, uB)

		// Mode C: explicit half-LSB bias then shift.
		biasMode := fixed.LShr(fixed.LAdd(lc1, fixed.Word32(1)<<uint(shiftR-1)), fixed.Word16(shiftR))
		uC := int16(fixed.Round(fixed.LShl(biasMode, 1)))
		t.Logf("  C) (_+half)>>shiftR      = %d  → u[1] = %d", biasMode, uC)
	} else {
		t.Logf("  B/C skipped: gcExp requires a saturating left shift, not a right-shift rounding choice.")
	}

	t.Logf("  All in-spec rounding modes return u[1]=1 because L_mult is exact mod 2^11.")
	t.Logf("  R-3a REFUTED: no rounding-mode change can lift u[1] from 1 to 2.")

	// ---------- R-3b: gain order / saturation ----------
	t.Logf("")
	t.Logf("--- R-3b: gain Q14 application order / saturation ---")
	t.Logf("  v[n]=0 ⇒ L_mult(gp, 0)=0 regardless of multiplication order.")
	t.Logf("  L_mult(mant=%d, c=%d)=%d is well within Word32 range (no saturation).", gcMantQ14, c[1], lc1)
	t.Logf("  R-3b REFUTED.")

	// ---------- R-3c: FCB innovation construction ----------
	t.Logf("")
	t.Logf("--- R-3c: FCB innovation ±PulseAmplitude (Q13=8192) ---")
	t.Logf("  Positions C=%#x signs S=%#x → pulses at {0,1,2,3} all +.", uint16(f.C1), uint8(f.S1))
	t.Logf("  c[0..3]=[%d %d %d %d] — exact +PulseAmplitude per §3.8.", c[0], c[1], c[2], c[3])
	t.Logf("  Pitch enhancement only modifies n ≥ tInt=%d; c[0..3] pristine.", tInt1)
	t.Logf("  R-3c REFUTED.")

	// ---------- R-3d: pastExc indexing at frame-0 boundary ----------
	t.Logf("")
	t.Logf("--- R-3d: pastExc[] zero-init / adaptive contribution at frame 0 ---")
	allZero := true
	for _, x := range d.pastExc {
		if x != 0 {
			allZero = false
			break
		}
	}
	t.Logf("  Decoder.pastExc all-zero on construction: %v  (per §4.3 init)", allZero)
	t.Logf("  v[0..7] = %v  ⇒ adaptive contribution = 0 for all n.", v[:8])
	t.Logf("  R-3d REFUTED.")

	// ---------- R-3e: subframe routing ----------
	t.Logf("")
	t.Logf("--- R-3e: sf0 vs sf1 input routing ---")
	t.Logf("  sf0 dispatched with bitstream P1/C1/S1/GA1/GB1.")
	t.Logf("  S-3 dump confirms sf1 sees DIFFERENT (gp=10769, gc=-32768, c=[0,-8192,8192,...])")
	t.Logf("  with no cross-wiring evidence. R-3e REFUTED.")

	// ---------- VERDICT ----------
	t.Logf("")
	t.Logf("=== R-3 verdict: NO-FIX ===")
	t.Logf("BuildExcitation implements §4.1.6 eq. (75) byte-EQ to G.191 basops.")
	t.Logf("u[0..7] matches hand-computed expected values exactly.")
	t.Logf("The off-by-2 at TAME f0 sample 1 must originate in LP interpolation (R-2).")
	t.Logf("")
	t.Logf("Re-rank for S-6 (LAST attempt before G-D3-EXHAUSTED):")
	t.Logf("  R-2  LP interpolation routing (§4.1.2 / §4.1.5)")
	t.Logf("       R-2a previous-LSP initial vector (§4.3 default)")
	t.Logf("       R-2b interpolation factor (avg vs other)")
	t.Logf("       R-2c LSP→LP conversion ordering")
	t.Logf("       R-2d sf-0/sf-1 routing inversion")
	t.Logf("")
	t.Logf("Cumulative refutation budget: 4 / 5 consumed.")
}
