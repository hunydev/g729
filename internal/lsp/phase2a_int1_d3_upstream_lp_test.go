package lsp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
)

// d3FrameSnapshot bundles every per-frame measurement collected by
// the d3 sequential mirror. Used as a value type by every Step
// subtest below.
type d3FrameSnapshot struct {
	hpfFirst10   [10]int16
	hpfLast10    [10]int16
	aProdQ12     [11]int16
	aOracleQ12   [11]int16
	rOracle      [11]float64
	rLagOracle   [11]float64
	kOracle      [11]float64 // index 0 unused
	eOracle      [11]float64
	oracleStable bool
	lpToLSPErr   error
	qProdQ15     [10]int16
}

// TestINT1D3UpstreamLP — Phase 2a-INT-1-d3 (upstream LP-analysis
// divergence diagnostic).
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-04-phase2a-int1-d3-upstream-lp-plan.md
//
// d2 refuted H-A,B,C,E,F,G,H,I,J,K and promoted H-L (upstream
// LP-analysis divergence) to dominant. d3 measures the upstream LP
// chain directly and bisects the encoder vs decoder a[] question.
//
// Subtest map:
//
//	S1_Frame29_LPInstability       — capture the LP-instability fault site
//	S2_Frame{5,10,15}_LPCoefDump   — r/k/a/q for early frames
//	S2b_ProdVsOracle_AQ12          — production a[] vs float oracle a[]
//	S3_ChebyshevSanity_Frame5      — F1/F2 sign-change count on a 61-grid
//	S4_EncoderVsDecoder_Frame5     — encoder a[] vs decoder oracle a[] (KEY)
//
// ABSOLUTE CONSTRAINTS (parent plan §0.4 + d1 §0):
//   - Clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 source.
//     Spec source = G729E.{pdf,txt} only.
//   - I6 binding: zero production-file changes.
//   - Measurement-only: t.Logf for every numeric value; t.Errorf only
//     where it provides a clear binary-pass/fail signal documented in
//     the d3 plan.
func TestINT1D3UpstreamLP(t *testing.T) {
	if testing.Short() {
		t.Skip("d3 upstream-LP battery; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		framesToDrive    = 30 // 0..29 inclusive (29 = LP-instability fault)
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read LSP.BIT: %v", err)
	}
	if len(inData) < framesToDrive*bytesPerInFrame {
		t.Fatalf("LSP.IN too short: %d", len(inData))
	}
	if len(bitData) < framesToDrive*bytesPerBitFrame {
		t.Fatalf("LSP.BIT too short: %d", len(bitData))
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	InitFreqPrev(&freqPrev)

	snaps := make(map[int]d3FrameSnapshot)
	target := map[int]bool{5: true, 10: true, 15: true, 29: true}

	for f := 0; f < framesToDrive; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(
				inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])

		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var snap d3FrameSnapshot
		copy(snap.hpfFirst10[:], processed[:10])
		copy(snap.hpfLast10[:], processed[70:80])

		var aProd [lpc.LPCOrder + 1]int16
		_ = an.Analyze(&oldSpeech, &aProd)
		snap.aProdQ12 = aProd

		var aOracle [11]float64
		oracleLPAnalysisD3(&oldSpeech, &aOracle,
			&snap.rOracle, &snap.rLagOracle, &snap.kOracle, &snap.eOracle)
		snap.oracleStable = true
		for i := 1; i <= 10; i++ {
			if math.Abs(snap.kOracle[i]) >= 1.0 {
				snap.oracleStable = false
			}
		}
		snap.aOracleQ12[0] = 4096
		for i := 1; i <= 10; i++ {
			v := math.Round(aOracle[i] * 4096.0)
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			snap.aOracleQ12[i] = int16(v)
		}

		var qProd [10]int16
		err := LPToLSP(&aProd, &qProd)
		snap.lpToLSPErr = err
		snap.qProdQ15 = qProd

		if target[f] {
			snaps[f] = snap
		}

		// Production VQ — advances production's freqPrev. Skip if
		// the LP→LSP path failed (frame 29).
		if err == nil {
			var omega [10]int16
			LSPToLSF(&qProd, &omega)
			_ = Quantize(&omega, &freqPrev)
		}
	}

	// --- Step 1: frame-29 LP-instability isolation -----------------
	t.Run("S1_Frame29_LPInstability", func(t *testing.T) {
		s := snaps[29]
		t.Logf("=== Frame 29 LP-instability isolation ===")
		t.Logf("HPF first 10 samples = %v", s.hpfFirst10)
		t.Logf("HPF last  10 samples = %v", s.hpfLast10)
		t.Logf("oracle r[0..10] (raw)          = %s", fmtFloats(s.rOracle[:], "%.4e"))
		t.Logf("oracle r[0..10] (lag+r0·1.0001)= %s", fmtFloats(s.rLagOracle[:], "%.4e"))
		t.Logf("oracle k[1..10]                = %s", fmtFloats(s.kOracle[1:], "%+.6f"))
		t.Logf("oracle E[0..10]                = %s", fmtFloats(s.eOracle[:], "%.4e"))
		t.Logf("oracle a[] Q12                 = %v", s.aOracleQ12)
		t.Logf("prod   a[] Q12                 = %v", s.aProdQ12)
		t.Logf("oracle Levinson stable (|k|<1 ∀k)? %v", s.oracleStable)
		if !s.oracleStable {
			for i := 1; i <= 10; i++ {
				if math.Abs(s.kOracle[i]) >= 1.0 {
					t.Logf("oracle: |k_%d| = %.6f ≥ 1 → autocorrelation matrix not",
						i, s.kOracle[i])
					t.Logf("        positive-definite at frame 29; LP filter NOT stable")
					t.Logf("        in the underlying mathematics, not just in Q-format.")
					break
				}
			}
		}
		t.Logf("prod LPToLSP error = %v", s.lpToLSPErr)
		if s.lpToLSPErr == nil {
			t.Logf("prod q[] Q15 = %v", s.qProdQ15)
		}

		// Sign-change scan on the production a[] (regardless of error).
		var f1, f2 [6]int32
		computeF1F2(&s.aProdQ12, &f1, &f2)
		nF1 := countSignChangesGrid60(&f1)
		nF2 := countSignChangesGrid60(&f2)
		t.Logf("prod F1 sign changes on grid60 = %d (need 5)", nF1)
		t.Logf("prod F2 sign changes on grid60 = %d (need 5)", nF2)
		if errors.Is(s.lpToLSPErr, ErrLPCNonStable) && (nF1 < 5 || nF2 < 5) {
			t.Logf("CONFIRMED: production aborts because F1/F2 sign-change scan")
			t.Logf("  yields fewer than 5 zeros for at least one polynomial.")
			t.Logf("  Failure mode = the polynomial A(z) returned by Levinson")
			t.Logf("  has its even/odd Chebyshev split with <5 real roots in (-1,1).")
		}
	})

	// --- Step 2: frame 5/10/15 LP coefficient dumps ----------------
	for _, f := range []int{5, 10, 15} {
		f := f
		t.Run(fmt.Sprintf("S2_Frame%d_LPCoefDump", f), func(t *testing.T) {
			s := snaps[f]
			t.Logf("=== Frame %d LP-coef dump ===", f)
			t.Logf("HPF first 10 samples = %v", s.hpfFirst10)
			t.Logf("HPF last  10 samples = %v", s.hpfLast10)
			t.Logf("oracle r[0..10] (raw)         = %s", fmtFloats(s.rOracle[:], "%.4e"))
			t.Logf("oracle r[0..10] (lag-windowed)= %s", fmtFloats(s.rLagOracle[:], "%.4e"))
			t.Logf("oracle k[1..10]               = %s", fmtFloats(s.kOracle[1:], "%+.6f"))
			t.Logf("oracle E[0..10]               = %s", fmtFloats(s.eOracle[:], "%.4e"))
			t.Logf("oracle a[] Q12                = %v", s.aOracleQ12)
			t.Logf("prod   a[] Q12                = %v", s.aProdQ12)
			t.Logf("oracle Levinson stable? %v", s.oracleStable)
			t.Logf("prod q[0..9] Q15              = %v", s.qProdQ15)
			t.Logf("prod LPToLSP error            = %v", s.lpToLSPErr)
			if s.lpToLSPErr == nil {
				distinct, inBounds, monoDec := true, true, true
				for i := 0; i < 10; i++ {
					if s.qProdQ15[i] <= -32768 || s.qProdQ15[i] >= 32767 {
						inBounds = false
					}
					if i > 0 && s.qProdQ15[i] >= s.qProdQ15[i-1] {
						monoDec = false
					}
					for j := i + 1; j < 10; j++ {
						if s.qProdQ15[i] == s.qProdQ15[j] {
							distinct = false
						}
					}
				}
				t.Logf("LSP roots: distinct=%v, in (-1,1)=%v, strictly decreasing=%v",
					distinct, inBounds, monoDec)
				var minGap int32 = 1 << 30
				for i := 1; i < 10; i++ {
					g := int32(s.qProdQ15[i-1]) - int32(s.qProdQ15[i])
					if g < minGap {
						minGap = g
					}
				}
				t.Logf("LSP min consecutive q-gap (Q15) = %d (≈ %.5f real)",
					minGap, float64(minGap)/32768.0)
			}
		})
	}

	// --- Step 2b: prod-vs-oracle a[] gap, frames 5/10/15/29 -------
	t.Run("S2b_ProdVsOracle_AQ12", func(t *testing.T) {
		for _, f := range []int{5, 10, 15, 29} {
			s := snaps[f]
			var maxD, sumD int32
			for i := 0; i <= 10; i++ {
				d := int32(s.aProdQ12[i]) - int32(s.aOracleQ12[i])
				if d < 0 {
					d = -d
				}
				if d > maxD {
					maxD = d
				}
				sumD += d
			}
			t.Logf("frame %2d  prod   a[]    = %v", f, s.aProdQ12)
			t.Logf("frame %2d  oracle a[] Q12= %v", f, s.aOracleQ12)
			t.Logf("frame %2d  |Δ| max=%d sum=%d Q12 (oracle-stable=%v)",
				f, maxD, sumD, s.oracleStable)
		}
	})

	// --- Step 3: Chebyshev sanity on frame 5 a[] -------------------
	t.Run("S3_ChebyshevSanity_Frame5", func(t *testing.T) {
		s := snaps[5]
		var f1, f2 [6]int32
		computeF1F2(&s.aProdQ12, &f1, &f2)
		t.Logf("=== Chebyshev sanity (frame 5 a[]) ===")
		t.Logf("a[] Q12        = %v", s.aProdQ12)
		t.Logf("f1[0..5] (Q24) = %v", f1)
		t.Logf("f2[0..5] (Q24) = %v", f2)

		// 61-grid: x = cos(i·π/60), i = 0..60. Spec §3.2.3 line 783
		// reads as "60 points equally spaced between 0 and π"; the
		// 61-point reading (i=0..60) is the alternative endpoint
		// inclusion. Production uses i=0..59 over k·π/59 instead.
		var x61 [61]int16
		for i := 0; i <= 60; i++ {
			v := math.Round(math.Cos(float64(i)*math.Pi/60.0) * 32768.0)
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			x61[i] = int16(v)
		}
		var c1, c2 [61]int32
		for i := 0; i <= 60; i++ {
			c1[i] = chebyshevC(x61[i], &f1)
			c2[i] = chebyshevC(x61[i], &f2)
		}
		var n1, n2 int
		var locF1, locF2 [12]float64
		for i := 1; i <= 60; i++ {
			if (c1[i-1] < 0) != (c1[i] < 0) {
				if n1 < len(locF1) {
					locF1[n1] = (math.Acos(float64(x61[i-1])/32768.0) +
						math.Acos(float64(x61[i])/32768.0)) / 2.0
				}
				n1++
			}
			if (c2[i-1] < 0) != (c2[i] < 0) {
				if n2 < len(locF2) {
					locF2[n2] = (math.Acos(float64(x61[i-1])/32768.0) +
						math.Acos(float64(x61[i])/32768.0)) / 2.0
				}
				n2++
			}
		}
		t.Logf("F1 sign changes on 61-grid = %d (want 5)", n1)
		t.Logf("F2 sign changes on 61-grid = %d (want 5)", n2)
		t.Logf("F1 bracket midpoints (ω rad) = %v", locF1[:capN(n1, len(locF1))])
		t.Logf("F2 bracket midpoints (ω rad) = %v", locF2[:capN(n2, len(locF2))])

		// Production's grid60 result for cross-reference.
		nP1 := countSignChangesGrid60(&f1)
		nP2 := countSignChangesGrid60(&f2)
		t.Logf("F1 sign changes on production grid60 = %d", nP1)
		t.Logf("F2 sign changes on production grid60 = %d", nP2)

		var qProd [10]int16
		if err := LPToLSP(&s.aProdQ12, &qProd); err != nil {
			t.Logf("findLSPRoots error: %v", err)
			return
		}
		t.Logf("findLSPRoots q[0..9] Q15 = %v", qProd)
		var omegaProd [10]float64
		for i := 0; i < 10; i++ {
			omegaProd[i] = math.Acos(float64(qProd[i]) / 32768.0)
		}
		t.Logf("findLSPRoots ω[0..9] rad = %v", omegaProd)

		if n1 == 5 && n2 == 5 {
			t.Logf("Chebyshev sanity PASS: each polynomial has exactly 5 zeros.")
		} else {
			t.Logf("Chebyshev sanity SUSPECT: F1=%d F2=%d (want 5 each).", n1, n2)
		}
	})

	// --- Step 4: encoder vs decoder a[] for frame 5 (KEY) ---------
	t.Run("S4_EncoderVsDecoder_Frame5", func(t *testing.T) {
		encoderAQ12 := snaps[5].aProdQ12
		encoderQ := snaps[5].qProdQ15

		var dec Decoder
		var sf1, sf2 [11]int16
		var rawLSP [10]int16
		var wantTuple [4]uint8
		for f := 0; f <= 5; f++ {
			l0, l1, l2, l3 := extractLSPFieldsD3(
				bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
			if f == 5 {
				wantTuple = [4]uint8{l0, l1, l2, l3}
			}
			sf1, sf2 = dec.Decode(Indices{L0: l0, L1: l1, L2: l2, L3: l3})
			rawLSP = dec.prevLSP // post-Decode this holds the just-
			// decoded current-frame LSP (Decoder.Decode step 9).
		}

		var aDecRaw [11]int16
		lspToLP(&rawLSP, &aDecRaw)

		t.Logf("=== Frame 5 encoder vs decoder a[0..10] cross-check ===")
		t.Logf("WANT indices (frame 5)            : L0=%d L1=%d L2=%d L3=%d",
			wantTuple[0], wantTuple[1], wantTuple[2], wantTuple[3])
		t.Logf("Encoder a[] Q12 (from LP analysis)= %v", encoderAQ12)
		t.Logf("Decoder a[] Q12 (WANT idx, sf2 raw)= %v", aDecRaw)
		t.Logf("Decoder sf1 Q12 (interpolated)    = %v", sf1)
		t.Logf("Decoder sf2 Q12 (current-frame)   = %v", sf2)
		t.Logf("Encoder LSP Q15                   = %v", encoderQ)
		t.Logf("Decoder LSP Q15 (raw, current)    = %v", rawLSP)

		var maxAbsDiff int32
		var sumAbsDiff int32
		for i := 0; i <= 10; i++ {
			d := int32(encoderAQ12[i]) - int32(aDecRaw[i])
			if d < 0 {
				d = -d
			}
			if d > maxAbsDiff {
				maxAbsDiff = d
			}
			sumAbsDiff += d
		}
		t.Logf("a[] diff: max-abs = %d Q12 (≈ %.4f real), sum-abs = %d Q12",
			maxAbsDiff, float64(maxAbsDiff)/4096.0, sumAbsDiff)

		var maxLSPDiff int32
		for i := 0; i < 10; i++ {
			d := int32(encoderQ[i]) - int32(rawLSP[i])
			if d < 0 {
				d = -d
			}
			if d > maxLSPDiff {
				maxLSPDiff = d
			}
		}
		t.Logf("LSP diff: max-abs = %d Q15 (≈ %.5f real)",
			maxLSPDiff, float64(maxLSPDiff)/32768.0)

		switch {
		case maxAbsDiff <= 50:
			t.Logf("CLASSIFICATION: encoder a[] ≈ decoder oracle a[] (Δ≤50 Q12).")
			t.Logf("  Implication: LP analysis is ~bit-exact with reference;")
			t.Logf("  divergence lives DOWNSTREAM (LSP→LSF, weights, predictor,")
			t.Logf("  or VQ search). REOPEN window-by-window the LSP encoding chain.")
		case maxAbsDiff <= 400:
			t.Logf("CLASSIFICATION: small but non-trivial drift (50 < Δ ≤ 400 Q12).")
			t.Logf("  Implication: LP analysis has minor Q-format / lag-window")
			t.Logf("  imprecision; could compound through freqPrev across frames.")
		default:
			t.Logf("CLASSIFICATION: large divergence (Δ > 400 Q12).")
			t.Logf("  Implication: LP analysis itself is materially off ITU's")
			t.Logf("  reference. H-L confirmed; next dispatch must localise the")
			t.Logf("  fault inside windowSpeech / autocorrelate / applyLagWindow")
			t.Logf("  / levinsonDurbin.")
		}
	})
}

// ---------------------------------------------------------------------
// Float64 LP-analysis oracle (clean-room; spec §3.2.1 eq. 4–7 + §3.2.2).
// Independent re-derivation, used only as a measurement reference.
// ---------------------------------------------------------------------

func oracleLPAnalysisD3(speech *[240]int16,
	aOut *[11]float64, rRaw, rLag, k, ePred *[11]float64) {

	var sPrime [240]float64
	for n := 0; n < 240; n++ {
		sPrime[n] = d3LPWindowReal[n] * float64(speech[n])
	}
	for kk := 0; kk <= 10; kk++ {
		var s float64
		for n := kk; n < 240; n++ {
			s += sPrime[n] * sPrime[n-kk]
		}
		rRaw[kk] = s
	}
	rLag[0] = rRaw[0] * 1.0001
	for kk := 1; kk <= 10; kk++ {
		rLag[kk] = rRaw[kk] * d3LagWindowReal[kk-1]
	}

	var a [11]float64
	var aPrev [11]float64
	a[0] = 1.0
	ePred[0] = rLag[0]
	for i := 1; i <= 10; i++ {
		var sum float64
		for j := 0; j < i; j++ {
			sum += a[j] * rLag[i-j]
		}
		var ki float64
		if ePred[i-1] != 0 {
			ki = -sum / ePred[i-1]
		}
		k[i] = ki
		copy(aPrev[:i], a[:i])
		for j := 1; j < i; j++ {
			a[j] = aPrev[j] + ki*aPrev[i-j]
		}
		a[i] = ki
		ePred[i] = (1.0 - ki*ki) * ePred[i-1]
	}
	*aOut = a
}

// d3LPWindowReal is the §3.2.1 eq. 3 30 ms asymmetric LP analysis
// window in real-valued doubles (oracle-side; production ships the
// Q15 LUT in internal/lpc/window.go).
var d3LPWindowReal = func() [240]float64 {
	var w [240]float64
	twoPi := 2.0 * math.Pi
	for n := 0; n < 200; n++ {
		w[n] = 0.54 - 0.46*math.Cos(twoPi*float64(n)/399.0)
	}
	for n := 200; n < 240; n++ {
		w[n] = math.Cos(twoPi * float64(n-200) / 159.0)
	}
	return w
}()

// d3LagWindowReal is the §3.2.1 eq. 6 60 Hz BW-expansion coefficient
// for k = 1..10 in real-valued doubles (oracle-side).
var d3LagWindowReal = func() [10]float64 {
	var w [10]float64
	const f0, fs = 60.0, 8000.0
	c := 2.0 * math.Pi * f0 / fs
	for kk := 1; kk <= 10; kk++ {
		x := c * float64(kk)
		w[kk-1] = math.Exp(-0.5 * x * x)
	}
	return w
}()

// ---------------------------------------------------------------------
// Misc helpers
// ---------------------------------------------------------------------

// countSignChangesGrid60 mirrors production's grid60 sign-change scan.
func countSignChangesGrid60(f *[6]int32) int {
	c0 := chebyshevC(grid60[0], f)
	n := 0
	for k := 1; k < 60; k++ {
		c1 := chebyshevC(grid60[k], f)
		if (c0 < 0) != (c1 < 0) {
			n++
		}
		c0 = c1
	}
	return n
}

func capN(n, m int) int {
	if n > m {
		return m
	}
	return n
}

func fmtFloats(xs []float64, format string) string {
	out := "["
	for i, v := range xs {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf(format, v)
	}
	return out + "]"
}

// extractLSPFieldsD3 mirrors the d2 helper. Duplicated to avoid
// cross-test-file coupling.
func extractLSPFieldsD3(g192Frame []byte) (l0, l1, l2, l3 uint8) {
	const g192Bit1 uint16 = 0x0081
	bit := func(i int) uint8 {
		off := 4 + 2*i
		if binary.LittleEndian.Uint16(g192Frame[off:off+2]) == g192Bit1 {
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
	l0 = pack(0, 1)
	l1 = pack(1, 7)
	l2 = pack(8, 5)
	l3 = pack(13, 5)
	return
}
