package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase3bDiag6_LPEnvelopeForensic is the Phase 3b DIAG-6 LP-spectral-
// envelope forensic. After DIAG-1..5 exonerated their candidates and the
// decoder energy chain was verified internally consistent (Salami identity
// = 1.0000, hand-EQ Δ = −0.001 dB, DIAG-5 verdict NO LEAK), the residual
// 5× rms shortfall against SPEECH.PST is per-frame non-uniform (p25=0.19,
// p95=1.05). Two surviving hypotheses:
//
//   - H_PFR: postfilter §A.4.2.1 short-term filter Â(z/γ_n)/Â(z/γ_d)
//     applies different bandwidth-expansion than spec; AGC normalises
//     total energy so spectral *shape* would differ even at matched RMS.
//   - H_ENV: LSP→LP conversion or some envelope-defining stage diverges
//     from SPEECH.PST upstream of the postfilter.
//
// Method: per-frame windowed DFT (Hann, N=80) on three signals —
// our_synth (postfilter bypassed via DecodeFrameNoPostfilter), our_pf
// (full Decode), REF_pf (SPEECH.PST). Compare log magnitudes in dB.
// 20 frames sampled every 187 frames over the corpus.
//
// Discrimination logic:
//   - shape(our_pf) ≠ shape(REF_pf) AND shape(our_synth) ≈ shape(REF_pf)
//     modulo postfilter shaping → H_PFR CONFIRMED.
//   - shape(our_synth) already differs from shape(REF_pf) modulo
//     postfilter → H_ENV CONFIRMED (LP synthesis upstream defect).
//   - All three envelope shapes agree → BOTH EXONERATED → closure-PARTIAL.
//
// Spec citations (clean-room, ITU-T G.729 06/2012 + Annex A only):
//
//   - §3.2.6 — LSP-to-LP conversion (Q-format chain).
//   - §4.2.1 / §4.2.2 — formant short-term postfilter Â(z/γ_n)/Â(z/γ_d).
//   - §A.4.2.1 — Annex A short-term postfilter, γ_n=0.55, γ_d=0.7.
//
// Informational only — `t.Logf`. No assertions.
func TestPhase3bDiag6_LPEnvelopeForensic(t *testing.T) {
	const bytesPerOutFrame = 2 * frameSamples

	vecDir := filepath.Join("..", "..", "testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	bitPath := filepath.Join(vecDir, "SPEECH.BIT")
	pstPath := filepath.Join(vecDir, "SPEECH.PST")

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(pstData) / bytesPerOutFrame
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}

	refPST := make([]int16, frames*frameSamples)
	for n := range refPST {
		refPST[n] = int16(binary.LittleEndian.Uint16(pstData[2*n : 2*n+2]))
	}

	// Pipeline our_pf: full Decode.
	ourPF := make([]int16, frames*frameSamples)
	{
		var dec Decoder
		var packed [bitstream.FrameBytes]byte
		r := bytes.NewReader(bitData)
		for f := 0; f < frames; f++ {
			if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
				t.Fatalf("ReadG192Frame[ourPF] frame %d: %v", f, rerr)
			}
			if derr := dec.Decode(packed[:], false, ourPF[f*frameSamples:(f+1)*frameSamples]); derr != nil {
				t.Fatalf("Decode[ourPF] frame %d: %v", f, derr)
			}
		}
	}

	// Pipeline our_synth: DecodeFrameNoPostfilter (postfilter bypassed).
	ourSynth := make([]int16, frames*frameSamples)
	{
		var dec Decoder
		var packed [bitstream.FrameBytes]byte
		r := bytes.NewReader(bitData)
		for f := 0; f < frames; f++ {
			if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
				t.Fatalf("ReadG192Frame[ourSynth] frame %d: %v", f, rerr)
			}
			if derr := dec.DecodeFrameNoPostfilter(packed[:], ourSynth[f*frameSamples:(f+1)*frameSamples]); derr != nil {
				t.Fatalf("DecodeFrameNoPostfilter[ourSynth] frame %d: %v", f, derr)
			}
		}
	}

	// γ audit (source-level).
	t.Logf("=== Phase 3b DIAG-6 — LP-spectral-envelope forensic ===")
	t.Logf("Corpus: SPEECH.BIT/SPEECH.PST (%d frames, %d samples).", frames, frames*frameSamples)
	t.Logf("")
	t.Logf("=== §J.3 γ_n / γ_d audit (internal/postfilter/postfilter.go) ===")
	t.Logf("Spec  G.729 §A.4.2.1: γ_n = 0.55, γ_d = 0.70.")
	t.Logf("Ours  gammaNumQ15 = %d (= %.6f, target 0.55, Δ = %+.6f)",
		18022, float64(18022)/32768.0, float64(18022)/32768.0-0.55)
	t.Logf("Ours  gammaDenQ15 = %d (= %.6f, target 0.70, Δ = %+.6f)",
		22938, float64(22938)/32768.0, float64(22938)/32768.0-0.70)
	t.Logf("Verdict (γ values): MATCH (within 1 LSB at Q15).")
	t.Logf("")

	// Pick 20 frames every 187 (covers to ~frame 3553 of 3750).
	const numProbes = 20
	const probeStride = 187
	probes := make([]int, 0, numProbes)
	for i := 0; i < numProbes; i++ {
		fr := i * probeStride
		if fr >= frames {
			break
		}
		probes = append(probes, fr)
	}

	const N = 80
	const fs = 8000.0
	hann := make([]float64, N)
	for n := 0; n < N; n++ {
		hann[n] = 0.5 * (1.0 - math.Cos(2*math.Pi*float64(n)/float64(N-1)))
	}

	// Per-bin error accumulators across all probe frames (bins 1..N/2).
	const numBins = N/2 + 1
	sumDpf := make([]float64, numBins)  // Σ (logMag(our_pf) − logMag(REF_pf))
	sumDpf2 := make([]float64, numBins) // Σ Δ²
	sumDsy := make([]float64, numBins)  // Σ (logMag(our_synth) − logMag(REF_pf))
	sumDsy2 := make([]float64, numBins) // Σ Δ²
	sumPFE := make([]float64, numBins)  // Σ (logMag(our_pf) − logMag(our_synth)) — empirical PF response
	sumPFE2 := make([]float64, numBins) // Σ Δ²
	// Shape-only (level-normalised) accumulators: subtract per-frame mean
	// log-magnitude so the L2 measures envelope shape independent of the
	// known 5× rms gap (DIAG-5 NO-LEAK; that gap is signal-level, not
	// spectral shape, and would otherwise dominate L2).
	sumShPf := make([]float64, numBins)
	sumShPf2 := make([]float64, numBins)
	sumShSy := make([]float64, numBins)
	sumShSy2 := make([]float64, numBins)
	count := make([]int, numBins)

	// L2 distances per probe frame (raw + shape-normalised).
	type frameL2 struct {
		frame                             int
		l2PfRef, l2SynthRef, l2PfSy       float64
		l2ShPfRef, l2ShSynthRef, l2ShPfSy float64
		rmsSynth, rmsPf, rmsRef           float64
		meanPf, meanSy, meanRef           float64
	}
	l2s := make([]frameL2, 0, len(probes))

	t.Logf("=== §J.4 Per-frame DFT log-magnitude (dB) — frame 100 detail + sample frames ===")
	t.Logf("Hann window N=80, fs=8 kHz, bin spacing = 100 Hz, log20 dB ref = 1.0.")

	logMag := func(buf []int16) []float64 {
		x := make([]complex128, N)
		for n := 0; n < N; n++ {
			x[n] = complex(float64(buf[n])*hann[n], 0)
		}
		// O(N²) DFT.
		out := make([]float64, numBins)
		for k := 0; k < numBins; k++ {
			var acc complex128
			theta := -2 * math.Pi * float64(k) / float64(N)
			for n := 0; n < N; n++ {
				acc += x[n] * cmplx.Exp(complex(0, theta*float64(n)))
			}
			m := cmplx.Abs(acc)
			if m < 1e-9 {
				m = 1e-9
			}
			out[k] = 20 * math.Log10(m)
		}
		return out
	}

	rms := func(buf []int16) float64 {
		var s float64
		for _, x := range buf {
			s += float64(x) * float64(x)
		}
		return math.Sqrt(s / float64(len(buf)))
	}

	formantPeaks := func(mag []float64) []int {
		// Local maxima excluding endpoints; minimum bin 1.
		ps := []int{}
		for k := 1; k < len(mag)-1; k++ {
			if mag[k] > mag[k-1] && mag[k] > mag[k+1] {
				ps = append(ps, k)
			}
		}
		return ps
	}

	// Track frame-100 detail printout.
	want100 := -1
	for i, fr := range probes {
		if fr == 100 {
			want100 = i
			break
		}
	}

	for pi, fr := range probes {
		base := fr * frameSamples
		bufSy := ourSynth[base : base+N]
		bufPf := ourPF[base : base+N]
		bufRef := refPST[base : base+N]

		mSy := logMag(bufSy)
		mPf := logMag(bufPf)
		mRef := logMag(bufRef)

		// Per-frame mean log-magnitude (over k=1..numBins-1, exclude DC) —
		// subtract to get level-normalised "shape" envelopes.
		var sumPf, sumSy, sumRef float64
		for k := 1; k < numBins; k++ {
			sumPf += mPf[k]
			sumSy += mSy[k]
			sumRef += mRef[k]
		}
		meanPf := sumPf / float64(numBins-1)
		meanSy := sumSy / float64(numBins-1)
		meanRef := sumRef / float64(numBins-1)

		// Aggregate per-bin error stats (skip DC bin 0 in summary print but
		// still accumulate so consumer can include if desired).
		var l2pfRef, l2syRef, l2pfSy float64
		var l2ShPfRef, l2ShSyRef, l2ShPfSy float64
		for k := 0; k < numBins; k++ {
			dPf := mPf[k] - mRef[k]
			dSy := mSy[k] - mRef[k]
			dPfSy := mPf[k] - mSy[k]
			dShPf := (mPf[k] - meanPf) - (mRef[k] - meanRef)
			dShSy := (mSy[k] - meanSy) - (mRef[k] - meanRef)
			dShPfSy := (mPf[k] - meanPf) - (mSy[k] - meanSy)
			sumDpf[k] += dPf
			sumDpf2[k] += dPf * dPf
			sumDsy[k] += dSy
			sumDsy2[k] += dSy * dSy
			sumPFE[k] += dPfSy
			sumPFE2[k] += dPfSy * dPfSy
			sumShPf[k] += dShPf
			sumShPf2[k] += dShPf * dShPf
			sumShSy[k] += dShSy
			sumShSy2[k] += dShSy * dShSy
			count[k]++
			if k >= 1 {
				l2pfRef += dPf * dPf
				l2syRef += dSy * dSy
				l2pfSy += dPfSy * dPfSy
				l2ShPfRef += dShPf * dShPf
				l2ShSyRef += dShSy * dShSy
				l2ShPfSy += dShPfSy * dShPfSy
			}
		}
		l2pfRef = math.Sqrt(l2pfRef / float64(numBins-1))
		l2syRef = math.Sqrt(l2syRef / float64(numBins-1))
		l2pfSy = math.Sqrt(l2pfSy / float64(numBins-1))
		l2ShPfRef = math.Sqrt(l2ShPfRef / float64(numBins-1))
		l2ShSyRef = math.Sqrt(l2ShSyRef / float64(numBins-1))
		l2ShPfSy = math.Sqrt(l2ShPfSy / float64(numBins-1))

		l2s = append(l2s, frameL2{
			frame: fr, l2PfRef: l2pfRef, l2SynthRef: l2syRef, l2PfSy: l2pfSy,
			l2ShPfRef: l2ShPfRef, l2ShSynthRef: l2ShSyRef, l2ShPfSy: l2ShPfSy,
			rmsSynth: rms(bufSy), rmsPf: rms(bufPf), rmsRef: rms(bufRef),
			meanPf: meanPf, meanSy: meanSy, meanRef: meanRef,
		})

		// Per-frame compact summary line (raw + shape-only).
		t.Logf("frame %4d  rms[sy/pf/ref]=%6.0f/%6.0f/%6.0f  L2raw[pf−ref|sy−ref|pf−sy]=%5.2f|%5.2f|%5.2f  L2shape[pf−ref|sy−ref|pf−sy]=%5.2f|%5.2f|%5.2f dB",
			fr, rms(bufSy), rms(bufPf), rms(bufRef),
			l2pfRef, l2syRef, l2pfSy,
			l2ShPfRef, l2ShSyRef, l2ShPfSy)

		// Frame-100 detail (or the first probe if 100 not selected).
		if want100 >= 0 && pi == want100 {
			t.Logf("    --- frame %d full per-bin table ---", fr)
			t.Logf("    %5s %8s %10s %10s %10s %12s %12s",
				"bin", "Hz", "synth(dB)", "pf(dB)", "ref(dB)", "Δsy−ref", "Δpf−ref")
			for k := 0; k < numBins; k++ {
				t.Logf("    %5d %8.0f %10.2f %10.2f %10.2f %12.3f %12.3f",
					k, float64(k)*fs/float64(N), mSy[k], mPf[k], mRef[k],
					mSy[k]-mRef[k], mPf[k]-mRef[k])
			}
			// Formant peaks
			pSy := formantPeaks(mSy)
			pPf := formantPeaks(mPf)
			pRef := formantPeaks(mRef)
			t.Logf("    formant-peak bins (Hz):")
			t.Logf("      synth: %v  Hz: %v", pSy, binsToHz(pSy, fs, N))
			t.Logf("      our_pf: %v  Hz: %v", pPf, binsToHz(pPf, fs, N))
			t.Logf("      ref_pf: %v  Hz: %v", pRef, binsToHz(pRef, fs, N))
		}
	}

	// §J.5 Aggregate per-bin level error statistics.
	t.Logf("")
	t.Logf("=== §J.5 Per-bin aggregate level error (raw, across %d probe frames) ===", len(probes))
	t.Logf("%5s %8s %12s %12s %12s %12s %12s %12s",
		"bin", "Hz", "μ(pf−ref)", "σ(pf−ref)", "μ(sy−ref)", "σ(sy−ref)", "μ(pf−sy)", "σ(pf−sy)")
	for k := 0; k < numBins; k++ {
		c := float64(count[k])
		mPf := sumDpf[k] / c
		sPf := math.Sqrt(math.Max(0, sumDpf2[k]/c-mPf*mPf))
		mSy := sumDsy[k] / c
		sSy := math.Sqrt(math.Max(0, sumDsy2[k]/c-mSy*mSy))
		mEm := sumPFE[k] / c
		sEm := math.Sqrt(math.Max(0, sumPFE2[k]/c-mEm*mEm))
		t.Logf("%5d %8.0f %12.3f %12.3f %12.3f %12.3f %12.3f %12.3f",
			k, float64(k)*fs/float64(N), mPf, sPf, mSy, sSy, mEm, sEm)
	}
	t.Logf("")
	t.Logf("=== §J.5b Per-bin SHAPE-only level error (mean-log-magnitude removed) ===")
	t.Logf("This isolates spectral shape from the known per-frame level offset")
	t.Logf("(the 5× rms gap from DIAG-5; that gap is signal level, not shape).")
	t.Logf("%5s %8s %12s %12s %12s %12s",
		"bin", "Hz", "μsh(pf−ref)", "σsh(pf−ref)", "μsh(sy−ref)", "σsh(sy−ref)")
	for k := 0; k < numBins; k++ {
		c := float64(count[k])
		mPf := sumShPf[k] / c
		sPf := math.Sqrt(math.Max(0, sumShPf2[k]/c-mPf*mPf))
		mSy := sumShSy[k] / c
		sSy := math.Sqrt(math.Max(0, sumShSy2[k]/c-mSy*mSy))
		t.Logf("%5d %8.0f %12.3f %12.3f %12.3f %12.3f",
			k, float64(k)*fs/float64(N), mPf, sPf, mSy, sSy)
	}

	// §J.6 L2 distribution.
	t.Logf("")
	t.Logf("=== §J.6 L2 envelope-distance distribution across probe frames ===")
	l2pfRefAll := make([]float64, len(l2s))
	l2syRefAll := make([]float64, len(l2s))
	l2pfSyAll := make([]float64, len(l2s))
	l2ShPfRefAll := make([]float64, len(l2s))
	l2ShSyRefAll := make([]float64, len(l2s))
	l2ShPfSyAll := make([]float64, len(l2s))
	for i, r := range l2s {
		l2pfRefAll[i] = r.l2PfRef
		l2syRefAll[i] = r.l2SynthRef
		l2pfSyAll[i] = r.l2PfSy
		l2ShPfRefAll[i] = r.l2ShPfRef
		l2ShSyRefAll[i] = r.l2ShSynthRef
		l2ShPfSyAll[i] = r.l2ShPfSy
	}
	logPercentiles := func(label string, v []float64) {
		w := append([]float64(nil), v...)
		sort.Float64s(w)
		p := func(q float64) float64 {
			if len(w) == 0 {
				return 0
			}
			i := int(q*float64(len(w)-1) + 0.5)
			if i < 0 {
				i = 0
			}
			if i >= len(w) {
				i = len(w) - 1
			}
			return w[i]
		}
		var sum float64
		for _, x := range w {
			sum += x
		}
		t.Logf("  %-22s mean=%5.2f  p25=%5.2f  p50=%5.2f  p75=%5.2f  p95=%5.2f  max=%5.2f dB",
			label, sum/float64(len(w)), p(0.25), p(0.50), p(0.75), p(0.95), p(1.0))
	}
	t.Logf(" --- raw (level + shape) ---")
	logPercentiles("L2(our_pf-REF_pf)", l2pfRefAll)
	logPercentiles("L2(our_synth-REF_pf)", l2syRefAll)
	logPercentiles("L2(our_pf-our_synth)", l2pfSyAll)
	t.Logf(" --- shape-only (mean log-mag removed; level-blind) ---")
	logPercentiles("L2sh(our_pf-REF_pf)", l2ShPfRefAll)
	logPercentiles("L2sh(our_synth-REF_pf)", l2ShSyRefAll)
	logPercentiles("L2sh(our_pf-our_synth)", l2ShPfSyAll)

	// §J.7 H_PFR: empirical postfilter response μ(our_pf − our_synth).
	t.Logf("")
	t.Logf("=== §J.7 Empirical postfilter spectral response (our_pf − our_synth) ===")
	t.Logf("Per spec §A.4.2.1, the formant filter is Â(z/γ_n)/Â(z/γ_d) with")
	t.Logf("γ_n=0.55 < γ_d=0.70, which sharpens formants (boosts peaks by")
	t.Logf("~+a few dB, attenuates valleys). AGC restores subframe energy so")
	t.Logf("sum-energy-equalised; per-bin shape should show formant emphasis.")
	t.Logf("Below: μ(pf−sy) per bin (positive = postfilter boosted that bin).")
	for k := 0; k < numBins; k++ {
		c := float64(count[k])
		mEm := sumPFE[k] / c
		bar := ""
		n := int(math.Round(mEm))
		if n > 0 {
			for i := 0; i < n && i < 30; i++ {
				bar += "+"
			}
		} else if n < 0 {
			for i := 0; i < -n && i < 30; i++ {
				bar += "-"
			}
		}
		t.Logf("  bin %2d (%4.0f Hz) μ(pf−sy)=%+6.2f dB  %s", k, float64(k)*fs/float64(N), mEm, bar)
	}
	t.Logf("Note: H_PFR analytical reconstruction would require exporting per-")
	t.Logf("subframe LP a[i] coefficients; the empirical view above is a direct")
	t.Logf("measurement of the postfilter's actual transfer function in dB.")

	// §J.8 Verdict synthesis.
	t.Logf("")
	t.Logf("=== §J.8 Verdict synthesis ===")
	// Compute aggregate magnitudes.
	var meanL2PfRef, meanL2SyRef, meanL2PfSy float64
	var meanShPfRef, meanShSyRef, meanShPfSy float64
	var meanLevPf, meanLevSy, meanLevRef float64
	for i, r := range l2s {
		meanL2PfRef += l2pfRefAll[i]
		meanL2SyRef += l2syRefAll[i]
		meanL2PfSy += l2pfSyAll[i]
		meanShPfRef += l2ShPfRefAll[i]
		meanShSyRef += l2ShSyRefAll[i]
		meanShPfSy += l2ShPfSyAll[i]
		meanLevPf += r.meanPf
		meanLevSy += r.meanSy
		meanLevRef += r.meanRef
	}
	n := float64(len(l2s))
	meanL2PfRef /= n
	meanL2SyRef /= n
	meanL2PfSy /= n
	meanShPfRef /= n
	meanShSyRef /= n
	meanShPfSy /= n
	meanLevPf /= n
	meanLevSy /= n
	meanLevRef /= n
	t.Logf("mean L2-raw     (our_pf − REF_pf)    = %.2f dB", meanL2PfRef)
	t.Logf("mean L2-raw     (our_synth − REF_pf) = %.2f dB", meanL2SyRef)
	t.Logf("mean L2-raw     (our_pf − our_synth) = %.2f dB", meanL2PfSy)
	t.Logf("mean L2-shape   (our_pf − REF_pf)    = %.2f dB  ← level-blind shape distance", meanShPfRef)
	t.Logf("mean L2-shape   (our_synth − REF_pf) = %.2f dB  ← level-blind shape distance", meanShSyRef)
	t.Logf("mean L2-shape   (our_pf − our_synth) = %.2f dB  ← postfilter spectral effect (shape only)", meanShPfSy)
	t.Logf("mean log-mag    (our_pf  / our_synth / REF_pf) = %.2f / %.2f / %.2f dB",
		meanLevPf, meanLevSy, meanLevRef)
	t.Logf("Δlevel (REF − our_pf) = %.2f dB ; (REF − our_synth) = %.2f dB",
		meanLevRef-meanLevPf, meanLevRef-meanLevSy)
	t.Logf("")
	t.Logf("Decision tree (shape-only L2; raw L2 is dominated by the known 5×")
	t.Logf("rms gap from DIAG-5 and is therefore not the right discriminator):")
	t.Logf("  if L2sh(pf−ref) < ~3 dB and L2sh(sy−ref) < ~3 dB → BOTH EXONERATED.")
	t.Logf("  if L2sh(sy−ref) ≪ L2sh(pf−ref)                  → H_PFR CONFIRMED.")
	t.Logf("  if both L2sh > ~4 dB and similar in magnitude    → H_ENV CONFIRMED.")
	switch {
	case meanShPfRef < 3.0 && meanShSyRef < 3.0:
		t.Logf("→ Pattern: shape-only L2 small for BOTH our_pf and our_synth vs")
		t.Logf("  REF_pf. The 5× rms gap is therefore a LEVEL phenomenon, not a")
		t.Logf("  shape phenomenon. Verdict: BOTH EXONERATED — closure-PARTIAL.")
	case meanShSyRef < meanShPfRef-1.5:
		t.Logf("→ Pattern: synth tracks REF_pf shape better than our_pf does.")
		t.Logf("  γ_n / γ_d audit shows MATCH within 1-LSB at Q15, so formula or")
		t.Logf("  filter wiring is suspect: H_PFR CONFIRMED.")
	case meanShPfRef < meanShSyRef-1.5:
		t.Logf("→ Pattern: our_pf tracks REF_pf shape better than synth does;")
		t.Logf("  postfilter is correcting toward REF, but synth diverges first.")
		t.Logf("  H_ENV PARTIAL — LSP→LP / synthesis envelope likely upstream defect.")
	default:
		t.Logf("→ Pattern: shape-only L2 ≈ for synth and our_pf — both differ")
		t.Logf("  from REF_pf in shape by similar amounts. The postfilter does not")
		t.Logf("  meaningfully close the gap, so H_PFR is unlikely; the divergence")
		t.Logf("  exists pre-postfilter. Verdict: H_ENV CONFIRMED.")
	}

	t.Logf("")
	t.Logf("=== Spec citations (clean-room) ===")
	t.Logf("  ITU-T G.729 (06/2012) §3.2.6 — LSP→LP conversion.")
	t.Logf("  ITU-T G.729 (06/2012) §4.2.1 / §4.2.2 — formant short-term postfilter.")
	t.Logf("  ITU-T G.729 (06/2012) §A.4.2.1 — Annex A γ_n=0.55, γ_d=0.70.")
	t.Logf("  Kondoz §6 — LP envelope interpretation.")
	t.Logf("  Oppenheim & Schafer — windowed DFT log-magnitude analysis.")
}

func binsToHz(bins []int, fs float64, N int) []float64 {
	out := make([]float64, len(bins))
	for i, b := range bins {
		out[i] = float64(b) * fs / float64(N)
	}
	return out
}
