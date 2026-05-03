package lsp

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/tables"
)

// TestINT1D7OmegaPrecision — Phase 2a-INT-1-d7
// LPToLSP/LSPToLSF precision diagnostic.
//
// Parent plan:
//
//	docs/superpowers/plans/2026-05-04-phase2a-int1-d4-pinpoint-plan.md
//	(d6 §15 confirms H-OMEGA-PRECISION as the dominant residual
//	source; this dispatch profiles where the drift originates.)
//
// ABSOLUTE CONSTRAINTS:
//   - Clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg consulted.
//     Spec source = G729E.{pdf,txt}.
//   - I6 binding: zero production-file changes (this file is a
//     measurement-only _test.go).
//   - I5 budget: NOT consumed (still 2/5 after this dispatch).
//
// Subtest map:
//
//	S1_Frame0_ProdVsAnalytical          — production ω vs i·π/11 reference
//	S2_Frame0_FloatOracleVsProduction   — pure-stdlib Chebyshev + acos oracle
//	S3_BisectionSensitivity             — frame 0 bisection iter count {4,6,8,10}
//	S4_SpeechFrameDrift                 — frames 5/10/15/25 prod vs float oracle
//	S5_OmegaPerturbationToVQ            — δ-LSB injection on frame-0 ω
//	S6_RootCauseLocalization            — combine S1..S5
//	S7_Frame596_LevinsonProjection      — Q24 vs Q30 vs float headroom
func TestINT1D7OmegaPrecision(t *testing.T) {
	if testing.Short() {
		t.Skip("d7 omega-precision diagnostic; -short")
	}

	t.Run("S1_Frame0_ProdVsAnalytical", subS1Frame0Analytical)
	t.Run("S2_Frame0_FloatOracleVsProduction", subS2FloatOracle)
	t.Run("S3_BisectionSensitivity", subS3Bisection)
	t.Run("S4_SpeechFrameDrift", subS4Speech)
	t.Run("S5_OmegaPerturbationToVQ", subS5VQ)
	t.Run("S6_RootCauseLocalization", subS6RootCause)
	t.Run("S7_Frame596_LevinsonProjection", subS7Frame596)
}

// -----------------------------------------------------------------
// Common analytical reference for a = [4096, 0, ..., 0] Q12.
// A(z) = 1 ⇒ F1(z) = 1 + z⁻¹¹, F2(z) = 1 − z⁻¹¹.
// On the unit circle, F1=0 at z = e^{j(2k+1)π/11} and F2=0 at
// z = e^{j 2kπ/11}. After excluding z = ±1, the 10 LSP angles are
// ω_i = (i+1)·π/11 for i = 0..9, interleaved F2/F1.
// -----------------------------------------------------------------

func analyticalOmegaQ13() (out [10]int16) {
	const piQ13 float64 = 25736
	for i := 0; i < 10; i++ {
		w := float64(i+1) * piQ13 / 11.0
		out[i] = int16(math.Round(w))
	}
	return
}

func analyticalCosQ15() (out [10]int16) {
	for i := 0; i < 10; i++ {
		c := math.Cos(float64(i+1) * math.Pi / 11.0)
		v := math.Round(c * 32768.0)
		switch {
		case v > 32767:
			v = 32767
		case v < -32768:
			v = -32768
		}
		out[i] = int16(v)
	}
	return
}

// -----------------------------------------------------------------
// S1: production ω vs analytical i·π/11
// -----------------------------------------------------------------

func subS1Frame0Analytical(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	// Production LP→LSP (Chebyshev path).
	var qProd [10]int16
	if err := LPToLSP(&a, &qProd); err != nil {
		t.Fatalf("LPToLSP failed: %v", err)
	}
	var omegaProd [10]int16
	LSPToLSF(&qProd, &omegaProd)

	qAna := analyticalCosQ15()
	omAna := analyticalOmegaQ13()

	t.Logf("F1/F2 closed-form check (Q24 oneQ24=%d):", int32(1)<<24)
	var f1, f2 [6]int32
	computeF1F2(&a, &f1, &f2)
	// Analytical: F1(z) = 1 + z⁻¹¹ ⇒ Chebyshev coefficients f1[i] for
	// i=1..5 are all 0 (since the 1+z⁻¹¹ factor splits evenly so the
	// reduced symmetric polynomial is purely the boundary contribution).
	// Same for F2. Document the production output here for the record.
	t.Logf("  f1[0..5] (Q24) = %v", f1)
	t.Logf("  f2[0..5] (Q24) = %v", f2)

	// 60-point grid evaluations of F1.
	t.Logf("Grid60 evaluations (F1, Q24):")
	for k := 0; k < 60; k++ {
		c := chebyshevC(grid60[k], &f1)
		t.Logf("  k=%2d x=%6d  C_F1(x)=%d", k, grid60[k], c)
	}

	// Per-coordinate drift ω.
	t.Logf("ω drift (frame 0, all-pass):")
	t.Logf("  i  cosProd  cosAna  Δcos(LSB Q15)  ωProd  ωAna  Δω(LSB Q13)")
	for i := 0; i < 10; i++ {
		dCos := int32(qProd[i]) - int32(qAna[i])
		dOm := int32(omegaProd[i]) - int32(omAna[i])
		t.Logf("  %d  %7d  %7d  %+6d         %5d  %5d  %+5d",
			i, qProd[i], qAna[i], dCos, omegaProd[i], omAna[i], dOm)
	}

	// Aggregate stats.
	maxOm := int32(0)
	sumAbs := int32(0)
	for i := 0; i < 10; i++ {
		d := int32(omegaProd[i]) - int32(analyticalOmegaQ13()[i])
		if d < 0 {
			d = -d
		}
		if d > maxOm {
			maxOm = d
		}
		sumAbs += d
	}
	t.Logf("Frame-0 ω drift: max|Δ|=%d Q13 LSB, mean|Δ|=%.2f", maxOm, float64(sumAbs)/10)
}

// -----------------------------------------------------------------
// S2: pure-stdlib float oracle for Chebyshev grid + bisection + acos.
// -----------------------------------------------------------------

// chebyshevCFloat evaluates C(x) per spec §3.2.3 eq. 17 in float64.
// The f[1..5] real-valued Chebyshev coefficients are derived from
// the same recursion as computeF1F2 but kept in real arithmetic.
func computeF1F2Float(a *[11]int16) (f1, f2 [6]float64) {
	f1[0] = 1.0
	f2[0] = 1.0
	for i := 0; i < 5; i++ {
		ai1 := float64(a[i+1]) / 4096.0
		a10i := float64(a[10-i]) / 4096.0
		f1[i+1] = ai1 + a10i - f1[i]
		f2[i+1] = ai1 - a10i + f2[i]
	}
	return
}

func chebyshevCFloat(x float64, f *[6]float64) float64 {
	// Same back-recursion as production, but in float64.
	bk1 := 1.0
	bk2 := 0.0
	for k := 4; k >= 1; k-- {
		bk := 2*x*bk1 - bk2 + f[5-k]
		bk2 = bk1
		bk1 = bk
	}
	return x*bk1 - bk2 + f[5]/2
}

// findRootsFloat replicates the production root scan (60-point grid,
// then bisection) but entirely in float64. nBisect controls bisection
// iteration count.
func findRootsFloat(f1, f2 *[6]float64, nBisect int) (q [10]float64, ok bool) {
	var rF1, rF2 [5]float64
	var nF1, nF2 int
	xPrev := 1.0
	cPrev1 := chebyshevCFloat(xPrev, f1)
	cPrev2 := chebyshevCFloat(xPrev, f2)

	for k := 1; k < 60; k++ {
		omega := float64(k) * math.Pi / 59.0
		x := math.Cos(omega)
		c1 := chebyshevCFloat(x, f1)
		c2 := chebyshevCFloat(x, f2)
		if nF1 < 5 && (c1 < 0) != (cPrev1 < 0) {
			rF1[nF1] = bisectFloat(xPrev, x, cPrev1, c1, f1, nBisect)
			nF1++
		}
		if nF2 < 5 && (c2 < 0) != (cPrev2 < 0) {
			rF2[nF2] = bisectFloat(xPrev, x, cPrev2, c2, f2, nBisect)
			nF2++
		}
		xPrev, cPrev1, cPrev2 = x, c1, c2
	}
	if nF1 < 5 || nF2 < 5 {
		return q, false
	}
	for i := 0; i < 5; i++ {
		q[2*i] = rF1[i]
		q[2*i+1] = rF2[i]
	}
	return q, true
}

func bisectFloat(xLo, xHi, cLo, cHi float64, f *[6]float64, n int) float64 {
	for i := 0; i < n; i++ {
		mid := (xLo + xHi) / 2
		cMid := chebyshevCFloat(mid, f)
		if (cLo < 0) != (cMid < 0) {
			xHi, cHi = mid, cMid
		} else {
			xLo, cLo = mid, cMid
		}
	}
	_ = cHi
	return (xLo + xHi) / 2
}

// floatToOmegaQ13 converts cos(ω) (float64) → ω in Q13 via
// math.Acos (the float "oracle" inverse).
func floatToOmegaQ13(c float64) int16 {
	if c > 1 {
		c = 1
	}
	if c < -1 {
		c = -1
	}
	w := math.Acos(c)
	v := math.Round(w * (25736.0 / math.Pi))
	if v < 0 {
		v = 0
	}
	if v > 25736 {
		v = 25736
	}
	return int16(v)
}

func subS2FloatOracle(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	// Production
	var qProd [10]int16
	_ = LPToLSP(&a, &qProd)
	var omProd [10]int16
	LSPToLSF(&qProd, &omProd)

	// Float oracle: Chebyshev + 4-iter bisection + Acos.
	f1F, f2F := computeF1F2Float(&a)
	qFloat, ok := findRootsFloat(&f1F, &f2F, 4)
	if !ok {
		t.Fatalf("float oracle did not find 5+5 roots")
	}
	var qFloatQ15 [10]int16
	var omFloat [10]int16
	for i := 0; i < 10; i++ {
		qF := math.Round(qFloat[i] * 32768.0)
		if qF > 32767 {
			qF = 32767
		} else if qF < -32768 {
			qF = -32768
		}
		qFloatQ15[i] = int16(qF)
		omFloat[i] = floatToOmegaQ13(qFloat[i])
	}

	// Hybrid: float Chebyshev → production lspToLSF (isolates the
	// arccos-LUT contribution).
	var omHybrid [10]int16
	for i := 0; i < 10; i++ {
		omHybrid[i] = lspToLSF(qFloatQ15[i])
	}

	omAna := analyticalOmegaQ13()
	t.Logf("Frame 0 — production vs float-oracle vs analytical")
	t.Logf("i | qProd qFloat ΔqLSB | ωProd ωFloat ωHybrid ωAna | dProd-Ana dFloat-Ana dHybrid-Ana")
	for i := 0; i < 10; i++ {
		dq := int32(qProd[i]) - int32(qFloatQ15[i])
		dProd := int32(omProd[i]) - int32(omAna[i])
		dFloat := int32(omFloat[i]) - int32(omAna[i])
		dHy := int32(omHybrid[i]) - int32(omAna[i])
		t.Logf("%d | %6d %6d %+5d | %5d %5d %5d %5d | %+4d %+4d %+4d",
			i, qProd[i], qFloatQ15[i], dq,
			omProd[i], omFloat[i], omHybrid[i], omAna[i],
			dProd, dFloat, dHy)
	}

	// Decomposition: how much of (ωProd − ωAna) is from production
	// Chebyshev path, vs production lspToLSF arccos LUT?
	// "ChebyshevContrib" ≈ ωHybrid − ωFloat (same arccos, different
	// cos source); "ArccosContrib" ≈ ωProd − ωFloat-with-prod-arccos
	// approximated by (ωProd − ωHybrid).
	t.Logf("Per-step decomposition (Q13 LSB):")
	for i := 0; i < 10; i++ {
		cheb := int32(omHybrid[i]) - int32(omFloat[i])
		arccos := int32(omProd[i]) - int32(omHybrid[i])
		t.Logf("  i=%d  ChebyshevQ-loss≈%+d  ArccosLUT-loss≈%+d  total≈%+d",
			i, cheb, arccos, cheb+arccos)
	}
}

// -----------------------------------------------------------------
// S3: bisection iteration sensitivity.
// Re-implement the production findLSPRoots with configurable
// bisection iterations (cannot mutate production under I6).
// -----------------------------------------------------------------

func bisectRootN(xLo, xHi int16, cLo, cHi int32, f *[6]int32, n int) int16 {
	for i := 0; i < n; i++ {
		mid := int16((int32(xLo) + int32(xHi)) >> 1)
		cMid := chebyshevC(mid, f)
		if (cLo < 0) != (cMid < 0) {
			xHi, cHi = mid, cMid
		} else {
			xLo, cLo = mid, cMid
		}
	}
	_ = cHi
	return int16((int32(xLo) + int32(xHi)) >> 1)
}

func findLSPRootsN(f1, f2 *[6]int32, q *[10]int16, n int) error {
	var rF1, rF2 [5]int16
	var nF1, nF2 int
	xPrev := grid60[0]
	cPrev1 := chebyshevC(xPrev, f1)
	cPrev2 := chebyshevC(xPrev, f2)
	for k := 1; k < 60; k++ {
		x := grid60[k]
		c1 := chebyshevC(x, f1)
		c2 := chebyshevC(x, f2)
		if nF1 < 5 && signsDiffer(cPrev1, c1) {
			rF1[nF1] = bisectRootN(xPrev, x, cPrev1, c1, f1, n)
			nF1++
		}
		if nF2 < 5 && signsDiffer(cPrev2, c2) {
			rF2[nF2] = bisectRootN(xPrev, x, cPrev2, c2, f2, n)
			nF2++
		}
		xPrev, cPrev1, cPrev2 = x, c1, c2
	}
	if nF1 < 5 || nF2 < 5 {
		return ErrLPCNonStable
	}
	for i := 0; i < 5; i++ {
		q[2*i] = rF1[i]
		q[2*i+1] = rF2[i]
	}
	return nil
}

func subS3Bisection(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var f1, f2 [6]int32
	computeF1F2(&a, &f1, &f2)
	omAna := analyticalOmegaQ13()
	t.Logf("Bisection sensitivity (frame 0, all-pass):")
	t.Logf("  N | maxAbsΔω | sumAbsΔω | per-coord Δω")
	for _, n := range []int{4, 6, 8, 10, 12, 16} {
		var q [10]int16
		if err := findLSPRootsN(&f1, &f2, &q, n); err != nil {
			t.Logf("  N=%d: %v", n, err)
			continue
		}
		var om [10]int16
		LSPToLSF(&q, &om)
		var maxAbs, sumAbs int32
		ds := make([]int32, 10)
		for i := 0; i < 10; i++ {
			d := int32(om[i]) - int32(omAna[i])
			ds[i] = d
			if d < 0 {
				d = -d
			}
			if d > maxAbs {
				maxAbs = d
			}
			sumAbs += d
		}
		t.Logf("  %2d | %8d | %8d | %v", n, maxAbs, sumAbs, ds)
	}
}

// -----------------------------------------------------------------
// S4: speech-frame drift (production vs float oracle).
// -----------------------------------------------------------------

func subS4Speech(t *testing.T) {
	const samples = 80
	const bytesPerInFrame = 2 * samples
	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	target := map[int]bool{5: true, 10: true, 15: true, 25: true}
	maxF := 26
	for f := 0; f < maxF; f++ {
		var pcmFrame [samples]int16
		off := f * bytesPerInFrame
		for i := 0; i < samples; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}
		var processed [samples]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: Analyze: %v", f, err)
		}
		if !target[f] {
			continue
		}
		var qProd [10]int16
		if err := LPToLSP(&aQ12, &qProd); err != nil {
			t.Logf("frame %d: LPToLSP error: %v", f, err)
			continue
		}
		var omProd [10]int16
		LSPToLSF(&qProd, &omProd)

		f1F, f2F := computeF1F2Float(&aQ12)
		qFloat, ok := findRootsFloat(&f1F, &f2F, 4)
		if !ok {
			t.Logf("frame %d: float oracle could not locate 5+5 roots", f)
			continue
		}
		var omFloat [10]int16
		for i := 0; i < 10; i++ {
			omFloat[i] = floatToOmegaQ13(qFloat[i])
		}

		var maxAbs, sumAbs int32
		for i := 0; i < 10; i++ {
			d := int32(omProd[i]) - int32(omFloat[i])
			if d < 0 {
				d = -d
			}
			if d > maxAbs {
				maxAbs = d
			}
			sumAbs += d
		}
		t.Logf("frame %d: a[0..10]=%v", f, aQ12)
		t.Logf("frame %d: ωProd  =%v", f, omProd)
		t.Logf("frame %d: ωFloat =%v", f, omFloat)
		var diffs [10]int32
		for i := 0; i < 10; i++ {
			diffs[i] = int32(omProd[i]) - int32(omFloat[i])
		}
		t.Logf("frame %d: Δ Q13 =%v   max=%d sum=%d", f, diffs, maxAbs, sumAbs)
	}
}

// -----------------------------------------------------------------
// S5: ω-perturbation → L2/L3 index sensitivity, frame 0.
// -----------------------------------------------------------------

func subS5VQ(t *testing.T) {
	// Baseline: production frame-0 ω used as "exact" target. We
	// perturb each coordinate independently by δ Q13 LSBs and report
	// at what |δ| the encoder L1/L2/L3/L0 indices flip.
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var qProd [10]int16
	_ = LPToLSP(&a, &qProd)
	var omegaBase [10]int16
	LSPToLSF(&qProd, &omegaBase)

	// Cold-start freqPrev (matches encoder.go).
	var freqPrevBase [4][10]int16
	InitFreqPrev(&freqPrevBase)

	baseline := func() Indices {
		var fp = freqPrevBase
		var w = omegaBase
		return Quantize(&w, &fp)
	}()
	t.Logf("Baseline frame-0 indices (using production ω): L0=%d L1=%d L2=%d L3=%d",
		baseline.L0, baseline.L1, baseline.L2, baseline.L3)

	// Sweep δ on each coordinate independently.
	deltas := []int32{-32, -16, -8, -4, -2, -1, 0, 1, 2, 4, 8, 16, 32}
	for coord := 0; coord < 10; coord++ {
		var changes []string
		for _, d := range deltas {
			fp := freqPrevBase
			w := omegaBase
			v := int32(w[coord]) + d
			if v < 0 {
				v = 0
			}
			if v > 25736 {
				v = 25736
			}
			w[coord] = int16(v)
			idx := Quantize(&w, &fp)
			flag := ""
			if idx.L0 != baseline.L0 {
				flag += "L0"
			}
			if idx.L1 != baseline.L1 {
				flag += "L1"
			}
			if idx.L2 != baseline.L2 {
				flag += "L2"
			}
			if idx.L3 != baseline.L3 {
				flag += "L3"
			}
			if flag == "" {
				flag = "-"
			}
			changes = append(changes, fmt.Sprintf("δ=%+3d→%s(L0=%d L1=%d L2=%d L3=%d)", d, flag,
				idx.L0, idx.L1, idx.L2, idx.L3))
		}
		t.Logf("coord=%d  %v", coord, changes)
	}

	// Joint perturbation: apply the SAME δ to all coords (worst-case
	// uniform tilt) — when does any index flip?
	t.Logf("Joint uniform perturbation:")
	for _, d := range deltas {
		fp := freqPrevBase
		w := omegaBase
		for c := 0; c < 10; c++ {
			v := int32(w[c]) + d
			if v < 0 {
				v = 0
			}
			if v > 25736 {
				v = 25736
			}
			w[c] = int16(v)
		}
		idx := Quantize(&w, &fp)
		t.Logf("  δ=%+3d → L0=%d L1=%d L2=%d L3=%d", d, idx.L0, idx.L1, idx.L2, idx.L3)
	}

	// Cross-check: what does the WANT bitstream say? L1=10, L2=10
	// (per d4 §14). Reconstruct the WANT ω from the WANT indices and
	// measure |Δ| vs production ω — this tells us how far the WANT
	// ω lies from production ω in Q13 LSBs.
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
	bitData, err := os.ReadFile(bitPath)
	if err == nil && len(bitData) >= 164 {
		l0, l1, l2, l3 := extractWantIndices(bitData[:164])
		t.Logf("Frame-0 WANT indices: L0=%d L1=%d L2=%d L3=%d", l0, l1, l2, l3)
		t.Logf("Frame-0 PROD indices: L0=%d L1=%d L2=%d L3=%d",
			baseline.L0, baseline.L1, baseline.L2, baseline.L3)
	}
}

// extractWantIndices unpacks the LSP-quantizer indices from the
// 164-byte G.192 bit-frame header (per ITU annex / d2 helper).
// Bit packing: prm[0]=L0 (1), prm[1]=L1 (7), prm[2]=L2 (5), prm[3]=L3 (5).
// Each prm bit is encoded as 0x007F (0) / 0x0081 (1) byte pair.
func extractWantIndices(frame []byte) (l0, l1, l2, l3 uint8) {
	// Skip header: bytes 0/1 = sync, bytes 2/3 = nbits=80, then 80
	// "softbit" 16-bit words follow (each word stored little-endian
	// over 2 bytes). Total 164 bytes: 4 header + 160 payload.
	read := func(idx int) uint16 {
		off := 4 + idx*2
		return binary.LittleEndian.Uint16(frame[off : off+2])
	}
	bit := func(idx int) uint8 {
		if read(idx) == 0x0081 {
			return 1
		}
		return 0
	}
	pack := func(start, n int) uint8 {
		var v uint8
		for i := 0; i < n; i++ {
			v = (v << 1) | bit(start+i)
		}
		return v
	}
	l0 = bit(0)
	l1 = pack(1, 7)
	l2 = pack(8, 5)
	l3 = pack(13, 5)
	return
}

// -----------------------------------------------------------------
// S6: aggregate root-cause localization.
// -----------------------------------------------------------------

func subS6RootCause(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var qProd [10]int16
	_ = LPToLSP(&a, &qProd)
	var omProd [10]int16
	LSPToLSF(&qProd, &omProd)

	// Float oracle.
	f1F, f2F := computeF1F2Float(&a)
	qFloat, _ := findRootsFloat(&f1F, &f2F, 4)
	var qFloatQ15 [10]int16
	for i := 0; i < 10; i++ {
		v := math.Round(qFloat[i] * 32768.0)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		qFloatQ15[i] = int16(v)
	}
	var omFloat, omHybrid [10]int16
	for i := 0; i < 10; i++ {
		omFloat[i] = floatToOmegaQ13(qFloat[i])
		omHybrid[i] = lspToLSF(qFloatQ15[i])
	}

	omAna := analyticalOmegaQ13()

	// Aggregate sums.
	var sumProdAna, sumFloatAna, sumHybridAna, sumProdHybrid int32
	for i := 0; i < 10; i++ {
		sumProdAna += absI32d7(int32(omProd[i]) - int32(omAna[i]))
		sumFloatAna += absI32d7(int32(omFloat[i]) - int32(omAna[i]))
		sumHybridAna += absI32d7(int32(omHybrid[i]) - int32(omAna[i]))
		sumProdHybrid += absI32d7(int32(omProd[i]) - int32(omHybrid[i]))
	}
	t.Logf("Aggregate |Δ| Q13 LSB (sum over 10 coords):")
	t.Logf("  Production      vs analytical: %d", sumProdAna)
	t.Logf("  Float-oracle    vs analytical: %d", sumFloatAna)
	t.Logf("  Hybrid(prod-acos+float-cos) vs analytical: %d", sumHybridAna)
	t.Logf("  Production      vs Hybrid    : %d   (≈ Chebyshev contribution)", sumProdHybrid)
	t.Logf("  Hybrid          vs Float     : (arccos LUT vs math.Acos contribution)")
}

func absI32d7(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// -----------------------------------------------------------------
// S7: frame-596 Levinson Q-format projection.
// Drive the production analyzer through frame 596 and observe
// whether LPToLSP succeeds; then project Q24 vs Q30 vs float64
// Levinson on a synthetic stress r[] designed to mimic near-singular
// AR conditions.
// -----------------------------------------------------------------

func subS7Frame596(t *testing.T) {
	const samples = 80
	const bytesPerInFrame = 2 * samples
	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	if len(inData) < 600*bytesPerInFrame {
		t.Skipf("LSP.IN only %d bytes (<600 frames)", len(inData))
	}
	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16

	var firstFail = -1
	var firstFailA [11]int16
	for f := 0; f < 600; f++ {
		var pcmFrame [samples]int16
		off := f * bytesPerInFrame
		for i := 0; i < samples; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}
		var processed [samples]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: Analyze: %v", f, err)
		}
		var q [10]int16
		err := LPToLSP(&aQ12, &q)
		if err != nil {
			if firstFail < 0 {
				firstFail = f
				firstFailA = aQ12
			}
		}
		if f == 596 {
			t.Logf("frame 596 a[]=%v  LPToLSP-err=%v", aQ12, err)
		}
	}
	if firstFail >= 0 {
		t.Logf("first LPToLSP failure: frame %d, a[]=%v", firstFail, firstFailA)
	} else {
		t.Logf("no LPToLSP failure across the first 600 frames (post FIX-1B)")
	}

	// Synthetic stress: AR1 process with pole approaching 1 to push
	// reflection coefficients toward ±1 and Levinson aWork toward
	// the int64-Q24 / int64-Q30 saturation rails. We project the
	// max-magnitude predictor coefficient at each order under three
	// arithmetics (Q24-fixed, Q30-fixed, float64) for several pole
	// radii.
	t.Logf("Levinson width projection (synthetic AR1, varying pole radius):")
	t.Logf("  rho |  maxQ24 |  maxQ30 |  maxFloat | Q24-overflow? | Q30-overflow?")
	for _, rho := range []float64{0.95, 0.98, 0.99, 0.995, 0.999, 0.9995} {
		// AR1 autocorr: r(k) = ρ^|k| (in some rscale). Choose rscale
		// so r(0) maps near 2^31 to mirror production worst case.
		const rscale = float64(1 << 30)
		var r [11]int32
		for k := 0; k <= 10; k++ {
			r[k] = int32(math.Round(math.Pow(rho, float64(k)) * rscale))
		}
		maxQ24, ovQ24 := simulateLevinsonQ(r, 24)
		maxQ30, ovQ30 := simulateLevinsonQ(r, 30)
		maxF := simulateLevinsonFloat(r)
		t.Logf("  %0.4f | %7d | %7d | %9.5f |     %v     |     %v",
			rho, maxQ24, maxQ30, maxF, ovQ24, ovQ30)
	}
}

// simulateLevinsonQ runs a clean-room §3.2.2 Levinson recursion at
// the requested fractional bit width on the supplied autocorrelation
// r[], returning the maximum |aWork[j]| in the requested Q-format
// across all stages and a "would-overflow-int64" flag. Mirrors the
// production update structure but parameterizes the carrier width.
func simulateLevinsonQ(r [11]int32, qBits uint) (maxAbsQ int64, overflowed bool) {
	const order = 10
	one := int64(1) << qBits
	var aWork, aPrev [order + 1]int64
	aWork[0] = one
	e := int64(r[0])

	q12Round := func(v int64) int64 {
		shift := int64(1) << (qBits - 12 - 1)
		if v >= 0 {
			return (v + shift) >> (qBits - 12)
		}
		return -((-v + shift) >> (qBits - 12))
	}

	for i := 1; i <= order; i++ {
		var sum int64
		sum = q12Round(aWork[0]) * int64(r[i])
		for j := 1; j < i; j++ {
			sum += q12Round(aWork[j]) * int64(r[i-j])
		}
		var kQ15 int32
		if e > 0 {
			num := -(sum << 3)
			q := num / e
			switch {
			case q > math.MaxInt16:
				kQ15 = math.MaxInt16
			case q < math.MinInt16:
				kQ15 = math.MinInt16
			default:
				kQ15 = int32(q)
			}
		}
		copy(aPrev[:i], aWork[:i])
		for j := 1; j < i; j++ {
			// (Q15 · Qn) >> 15 = Qn
			term := (int64(kQ15) * aPrev[i-j]) >> 15
			aWork[j] = aPrev[j] + term
			// Overflow check: |aWork[j]| approaching int64 rail.
			if aWork[j] > (int64(1)<<62) || aWork[j] < -(int64(1)<<62) {
				overflowed = true
			}
		}
		aWork[i] = int64(kQ15) << (qBits - 15)

		kSq := int64(kQ15) * int64(kQ15)
		if kSq > (int64(1) << 30) {
			kSq = int64(1) << 30
		}
		oneMinusKSq := (int64(1) << 30) - kSq
		e = (e * oneMinusKSq) >> 30
	}

	for j := 1; j <= order; j++ {
		v := aWork[j]
		if v < 0 {
			v = -v
		}
		if v > maxAbsQ {
			maxAbsQ = v
		}
	}
	return
}

// simulateLevinsonFloat returns the maximum |a_j| in real arithmetic
// after the order-10 Levinson recursion.
func simulateLevinsonFloat(r [11]int32) (maxAbs float64) {
	const order = 10
	rscale := float64(r[0])
	if rscale == 0 {
		return 0
	}
	var rr [order + 1]float64
	for i := 0; i <= order; i++ {
		rr[i] = float64(r[i]) / rscale
	}
	var aWork, aPrev [order + 1]float64
	aWork[0] = 1
	e := rr[0]
	for i := 1; i <= order; i++ {
		var sum float64
		for j := 0; j < i; j++ {
			sum += aWork[j] * rr[i-j]
		}
		var k float64
		if e > 0 {
			k = -sum / e
		}
		copy(aPrev[:i], aWork[:i])
		for j := 1; j < i; j++ {
			aWork[j] = aPrev[j] + k*aPrev[i-j]
		}
		aWork[i] = k
		e = (1 - k*k) * e
	}
	for j := 1; j <= order; j++ {
		v := math.Abs(aWork[j])
		if v > maxAbs {
			maxAbs = v
		}
	}
	return
}

// Sanity: silence the "unused tables/helpers" linter when the spec
// PDF reference is the only consumer.
var _ = tables.LSPCodebookL1
