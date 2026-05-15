package lsp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase1o_D3_S6_R2_LpDump — Phase 1o D-3 S-6 R-2 family
// investigation for the TAME f0 sf0 sample-1 off-by-2.
//
// Plan reference:
//
//	docs/superpowers/plans/2026-05-10-phase1o-d3-statebearing-rootcause-plan.md
//	§S-6 (FINAL diagnostic budget; fix attempt 5 of 5).
//
// PRIOR-WORK CHAIN:
//   - S-1 (aa27ad1)  state-bearing dump (defect = single common
//     state-bearing container).
//   - S-2 (0428df7)  H-1 agc-init seed REFUTED.
//   - S-3 (bd37512)  H-11 family REFUTED (defect originates inside
//     synth.Filter — s[1] = 0 vs want 1, with sf0 LP a[] = [4096 2108
//     1500 -137 399 -135 156 -55 301 256 189]).
//   - S-4 (da089b5)  R-1 synth-rounding REFUTED (G.191 basop semantics
//     byte-EQ; real-valued y[1] = u[1] − a[1]·s[0]/4096 = 1 − 2108/4096
//     = 0.485, which round_extract_h sends to 0).
//   - S-5 (be80eaf)  R-3 BuildExcitation REFUTED (u[0..7] hand-computed
//     byte-EQ to production: [1 1 1 1 0 0 0 0]).
//
// MATHEMATICAL ANCHOR (closed-form via §4.1.6 eq. (77) + G.191 basops):
//
//	s[1] = round( L_shl( L_mult(u[1], a[0]) − L_mult(a[1], s[0]), 3 ) )
//	     = round( L_shl( 2·u[1]·a[0] − 2·a[1]·s[0], 3 ) )
//
//	With u[1]=1, a[0]=4096, s[0]=1:
//	     = round( L_shl( 8192 − 2·a[1], 3 ) )
//	     = extract_h( 8·(8192 − 2·a[1]) + 0x8000 )
//	     = extract_h( 65536 − 16·a[1] + 32768 )
//	     = extract_h( 98304 − 16·a[1] )
//
//	For s[1] = 1: need 98304 − 16·a[1] ≥ 65536  ⇔  a[1] ≤ 2048.
//	For s[1] = 0: need 32768 ≤ 98304 − 16·a[1] < 65536  ⇔  2049 ≤ a[1] ≤ 4096.
//
//	Production sf0 a[1] = 2108 → 98304 − 33728 = 64576 → extract_h = 0.
//	The minimum perturbation ΔΔ to flip s[1] from 0 → 1 is a[1] ≤ 2048,
//	i.e. a delta of −60 LSB on the Q12 LP coefficient.
//
// SPEC ANCHORS (G729E.txt verbatim line numbers, ITU-T G.729 (06/2012)):
//
//	§3.2.5 (lines 901..919, "Interpolation of the LSP coefficients"):
//	  "The quantized (and unquantized) LP coefficients are used for
//	   the second subframe. For the first subframe, the quantized
//	   (and unquantized) LP coefficients are obtained by linear
//	   interpolation of the corresponding parameters in the adjacent
//	   subframes. The interpolation is done on the LSP coefficients in
//	   the cosine domain. ... The (unquantized) interpolated LSP
//	   coefficients in each of the two subframes are given by:
//	      Subframe 1 : q_i^(1) = 0.5 q_i^(previous) + 0.5 q_i^(current)
//	      Subframe 2 : q_i^(2) = q_i^(current)                    (24)"
//
//	§3.2.4 (line 843): "At start up the initial values of l̂_i^(k)
//	   are given by l̂_i = i·π/11 for all k < 0."
//
//	§4.3 Table 9 (lines 1700..1707): non-zero initialization table —
//	   l̂_i (§3.2.4) = i·π/11; q_i (§3.2.4) = arccos(i·π/11).
//	   NB the table caption says "arccos"; in the spec's notation the
//	   forward direction is q_i = cos(ω_i) so the LSP initial values
//	   that q_i takes at startup are cos(i·π/11). (See §A.3.2 / lsf_lsp.go
//	   forward map: lsfToLSP(ω) = cos(ω).)
//
// SUB-HYPOTHESIS ENUMERATION (R-2a..f):
//
//	R-2a  prevLSP first-frame init constants (cos(i·π/11) Q15) byte-EQ.
//	R-2b  sf0 interpolation formula (½·prev + ½·curr) byte-EQ.
//	R-2c  Chebyshev LSP→LP conversion bit-EQ.
//	R-2d  a[] Q-format (production declares Q12 with a[0]=4096).
//	R-2e  sf0 vs sf1 routing — does sf0 actually receive the
//	      INTERPOLATED a[], or accidentally receive sf1's curr-only a[]?
//	R-2f  a[0] = 4096 (Q12 unity) handling.
//
// METHODOLOGY: dump prevLSP, currLSP, sf0_lsp, sf1_lsp, sf0_a, sf1_a
// from the TAME frame-0 production decode chain. Hand-verify each
// sub-hypothesis byte-EQ against the spec citations above. This is a
// diagnostic test (t.Logf only, no t.Errorf in the dump section) per
// the S-2/S-3/S-4/S-5 escape-hatch convention so the suite stays
// GREEN while the defect remains live.
func TestPhase1o_D3_S6_R2_LpDump(t *testing.T) {
	bitPath := lspVectorPath("TAME.BIT")
	if _, err := os.Stat(bitPath); err != nil {
		t.Skipf("missing test vector %s: %v", bitPath, err)
	}

	frames, _ := readG192FramesForLSP(t, bitPath)
	if len(frames) == 0 {
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

	idx := Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	}

	t.Logf("=== Phase 1o D-3 S-6 R-2 LP-coefficient dump (TAME frame 0) ===")
	t.Logf("LSP bitstream indices: L0=%d L1=%d L2=%d L3=%d",
		idx.L0, idx.L1, idx.L2, idx.L3)

	// ---- R-2a: prevLSP first-frame init ----
	t.Logf("")
	t.Logf("--- R-2a: prevLSP first-frame init constants ---")
	t.Logf("Reference-execution numeric oracle: fixed codec-start prevLSP Q15")
	specPrevLSP := [10]int16{
		30000, 26000, 21000, 15000, 8000,
		0, -8000, -15000, -21000, -26000,
	}
	t.Logf("  oracle startup prevLSP            : %v", specPrevLSP)
	t.Logf("  production initialPrevLSP            : %v", initialPrevLSP)
	mismatch := false
	for i := 0; i < 10; i++ {
		if specPrevLSP[i] != initialPrevLSP[i] {
			t.Logf("  MISMATCH at i=%d: spec=%d prod=%d (Δ=%d LSB)",
				i, specPrevLSP[i], initialPrevLSP[i],
				int(initialPrevLSP[i])-int(specPrevLSP[i]))
			mismatch = true
		}
	}
	if !mismatch {
		t.Logf("  R-2a REFUTED: prevLSP init byte-EQ to spec §4.3 Table 9.")
	} else {
		t.Logf("  R-2a SURVIVING CANDIDATE: byte-mismatch on prevLSP init constants.")
	}

	// Re-run the production decoder body MANUALLY so we can capture
	// intermediate state at every step.
	var d Decoder
	d.pastResiduals[0] = initialPastResidual
	d.pastResiduals[1] = initialPastResidual
	d.pastResiduals[2] = initialPastResidual
	d.pastResiduals[3] = initialPastResidual

	var residual [10]int16
	combineResidual(idx.L1, idx.L2, idx.L3, &residual)
	t.Logf("")
	t.Logf("--- §3.2.4 step trace ---")
	t.Logf("  residual after combineResidual (Q13): %v", residual)
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	t.Logf("  residual after rearrange J1,J2  (Q13): %v", residual)

	var lsf [10]int16
	d.applyPredictor(idx.L0, &residual, &lsf)
	t.Logf("  lsf after MA predictor          (Q13): %v", lsf)
	enforceLSFStability(&lsf)
	t.Logf("  lsf after stability             (Q13): %v", lsf)

	var currLSP [10]int16
	for i := 0; i < 10; i++ {
		currLSP[i] = lsfToLSP(lsf[i])
	}
	t.Logf("  currLSP (cos)                   (Q15): %v", currLSP)
	t.Logf("  prevLSP (frame-0 init)          (Q15): %v", initialPrevLSP)

	// ---- R-2b: interpolation formula ----
	t.Logf("")
	t.Logf("--- R-2b: sf0 interpolation formula ---")
	t.Logf("Spec §3.2.5 eq. (24): sf1[i] = 0.5·prev[i] + 0.5·curr[i]   (FIRST subframe)")
	t.Logf("                       sf2[i] = curr[i]                     (SECOND subframe)")
	var sf0LSP, sf1LSP [10]int16
	interpolateLSP(&initialPrevLSP, &currLSP, &sf0LSP, &sf1LSP)
	t.Logf("  sf0_lsp = (prev+curr)>>1        (Q15): %v", sf0LSP)
	t.Logf("  sf1_lsp = curr                  (Q15): %v", sf1LSP)

	// ---- R-2c..f: LSP → LP conversion ----
	var sf0A, sf1A [11]int16
	LSPToLP(&sf0LSP, &sf0A)
	LSPToLP(&sf1LSP, &sf1A)
	t.Logf("")
	t.Logf("--- R-2c/d/f: LSP → LP conversion outputs ---")
	t.Logf("  sf0 a[0..10] (interpolated LSP) (Q12): %v", sf0A)
	t.Logf("  sf1 a[0..10] (curr LSP)         (Q12): %v", sf1A)

	// Bonus probes: a[] under alternative LSP inputs to estimate the
	// magnitude of the routing-vs-formula leverage on a[1].
	var aFromCurrOnly, aFromPrevOnly, aFromMid, aFromCurrFloor [11]int16
	LSPToLP(&currLSP, &aFromCurrOnly)
	LSPToLP(&initialPrevLSP, &aFromPrevOnly)
	var midFloat [10]int16
	for i := 0; i < 10; i++ {
		midFloat[i] = int16((int32(initialPrevLSP[i]) + int32(currLSP[i])) >> 1)
	}
	LSPToLP(&midFloat, &aFromMid)
	t.Logf("")
	t.Logf("--- R-2 routing probes ---")
	t.Logf("  a[] from PREV LSP only          (Q12): %v", aFromPrevOnly)
	t.Logf("  a[] from CURR LSP only (= sf1)  (Q12): %v", aFromCurrOnly)
	t.Logf("  a[] from MID LSP (= sf0 prod)   (Q12): %v", aFromMid)
	_ = aFromCurrFloor

	// ---- Mathematical anchor: hand-compute sf0 a[1] range ----
	t.Logf("")
	t.Logf("--- Mathematical anchor: a[1] range to flip s[1] from 0 → 1 ---")
	t.Logf("  Closed-form: extract_h(98304 − 16·a[1])")
	for _, candA1 := range []int{int(sf0A[1]), int(sf1A[1]), 2048, 2049, 2108, 1500} {
		val := 98304 - 16*candA1
		s1 := val >> 16
		t.Logf("  a[1] = %5d  →  98304 − 16·a[1] = %7d  →  extract_h = %d",
			candA1, val, int16(s1))
	}
	t.Logf("  REQUIRED: a[1] ≤ 2048 (Q12) to make s[1] = 1.")
	t.Logf("  PRODUCTION sf0 a[1] = %d  →  s[1] = %d", sf0A[1],
		int16((98304-16*int(sf0A[1]))>>16))
	t.Logf("  PRODUCTION sf1 a[1] = %d  →  s[1] would be %d if sf0 received this", sf1A[1],
		int16((98304-16*int(sf1A[1]))>>16))

	// ---- R-2e: routing check via decoder.Decode public API ----
	var dpub Decoder
	pubSF0A, pubSF1A := dpub.Decode(idx)
	t.Logf("")
	t.Logf("--- R-2e: sf0/sf1 routing via lsp.Decoder.Decode public API ---")
	t.Logf("  Decode().sf1 a[0..10] (1st subframe, samples  0..39) (Q12): %v", pubSF0A)
	t.Logf("  Decode().sf2 a[0..10] (2nd subframe, samples 40..79) (Q12): %v", pubSF1A)
	if pubSF0A != sf0A {
		t.Errorf("sf0 routing: Decode() returns %v but manual interpolated path gives %v",
			pubSF0A, sf0A)
	}
	if pubSF1A != sf1A {
		t.Errorf("sf1 routing: Decode() returns %v but manual curr-only path gives %v",
			pubSF1A, sf1A)
	}
	if pubSF0A == sf0A && pubSF1A == sf1A {
		t.Logf("  R-2e REFUTED: routing matches spec — sf0 receives interpolated, sf1 receives curr-only.")
	}

	// ---- R-2f: a[0] = 4096 unity check ----
	t.Logf("")
	t.Logf("--- R-2f: a[0] = 4096 (Q12 unity) ---")
	if pubSF0A[0] != 4096 || pubSF1A[0] != 4096 {
		t.Errorf("a[0] not 4096: sf0=%d sf1=%d", pubSF0A[0], pubSF1A[0])
	} else {
		t.Logf("  a[0] = 4096 byte-EQ for both subframes. R-2f REFUTED.")
	}

	t.Logf("")
	t.Logf("=== Verdict scaffolding (decided in plan §S-6) ===")
	t.Logf("  Production sf0 a[1] = %d", sf0A[1])
	t.Logf("  Required for s[1]=1 : a[1] ≤ 2048")
	t.Logf("  Gap                 : %d LSB", int(sf0A[1])-2048)
	t.Logf("  Routing R-2e        : %v (true ⇒ sf0/sf1 routing is spec-correct)",
		pubSF0A == sf0A && pubSF1A == sf1A)
}

func lspVectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}

func readG192FramesForLSP(tb testing.TB, path string) ([][]byte, []bool) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readG192FramesForLSP: %v", err)
	}
	frames, bads, err := bitstream.ReadG192File(bytes.NewReader(data))
	if err != nil {
		tb.Fatalf("ReadG192File(%s): %v", path, err)
	}
	return frames, bads
}

func byteReader(b []byte) *bytesReaderShim { return &bytesReaderShim{b: b} }

type bytesReaderShim struct {
	b []byte
	o int
}

func (r *bytesReaderShim) Read(p []byte) (int, error) {
	if r.o >= len(r.b) {
		return 0, ioEOF
	}
	n := copy(p, r.b[r.o:])
	r.o += n
	return n, nil
}

var ioEOF = func() error {
	return errEOF{}
}()

type errEOF struct{}

func (errEOF) Error() string { return "EOF" }
