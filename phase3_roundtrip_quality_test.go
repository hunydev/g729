package g729

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase3RoundTripQuality_SPEECH measures three SNR profiles to isolate
// the perceptual contribution of our encoder vs our decoder vs the codec's
// intrinsic G.729A loss. Authored as the Phase 3 entry diagnostic per
// post-Phase-2 closure dispatch (2026-05-15) — its result determines
// whether path A (clean-room continuation, byte-EQ deferred) is acceptable
// or whether path B (wall-protocol reference consultation) / path C
// (two-team clean-room) must be activated to recover ITU level-2 byte-EQ.
//
// Reference signal: SPEECH.IN (original PCM input). NOT SPEECH.PST —
// Phase 1o D-3 disposed PSTdomain ambiguity max|Δ|=32104 between our
// decoder and SPEECH.PST as PASS-by-design (see
// internal/decoder/itu_vector_pstdomain_test.go), so SPEECH.PST is
// contaminated as a perceptual reference relative to our decoder. The
// canonical reference for perceptual quality is the original PCM input.
//
// Three pipelines compared against SPEECH.IN:
//
//	A. SPEECH.PST                            (ITU enc + ITU dec: intrinsic G.729A loss)
//	B. SPEECH.BIT  → ourDecoder              (ITU enc + our dec: decoder-only path)
//	C. SPEECH.IN   → ourEncoder → ourDecoder (full round-trip)
//
// (C-SegSNR − B-SegSNR) ≈ encoder perceptual contribution.
// (B-SegSNR − A-SegSNR) ≈ decoder perceptual contribution
//                         (incorporates Phase 1o D-3 PSTdomain Δ).
//
// Metrics (clean-room, public-textbook references only):
//
//	Global SNR : 10·log10(Σs² / Σ(s−ŝ)²)            single number across all samples.
//	SegSNR     : average of per-frame SNR over 80-sample (10ms) frames,
//	             clipped to [-10, +35] dB, silence frames skipped.
//
// SegSNR formulation per Quackenbush / Barnwell / Clements,
// "Objective Measures of Speech Quality", Prentice-Hall, 1988, §2.4
// (clean-room legitimate textbook reference). Same formulation reported
// in Salami et al., "Design and Description of CS-ACELP", IEEE T-SAP 1998
// §V.B Table II (G.729 reference paper).
//
// Reference baselines (G.729A on clean speech, public literature):
//
//	SegSNR    ~ 12–14 dB (Salami 1998 Table II)
//	GlobalSNR ~  8–12 dB
//
// Decision criteria (informational; the operator decides):
//
//	C-SegSNR ≥ A-SegSNR − 2 dB → encoder perceptual loss negligible. Path A acceptable.
//	C-SegSNR ≥ A-SegSNR − 5 dB → bounded perceptual loss. Path B candidate.
//	C-SegSNR <  A-SegSNR − 5 dB → destructive perceptual loss. Path B/C forced.
//
// Test is INFORMATIONAL (t.Logf only). Fails only on infrastructure
// errors (missing files, decode panics). Quality numbers feed the
// path-decision dispatch authored by the operator.
func TestPhase3RoundTripQuality_SPEECH(t *testing.T) {
	const (
		bytesPerInFrame  = 2 * FrameSamples
		bytesPerBitFrame = 164
	)
	vecDir := "testdata/itu/G729_Release3/g729AnnexA/test_vectors"
	inPath := filepath.Join(vecDir, "SPEECH.IN")
	bitPath := filepath.Join(vecDir, "SPEECH.BIT")
	pstPath := filepath.Join(vecDir, "SPEECH.PST")

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	// Frame count reconciliation per existing convention
	// (phase2f_int1_per_vector_byteeq_test.go). SPEECH.IN has 32-byte
	// trailing samples (encoder lookahead margin); use the smallest
	// of the three stream frame counts.
	frames := len(inData) / bytesPerInFrame
	if bf := len(bitData) / bytesPerBitFrame; bf < frames {
		frames = bf
	}
	if pf := len(pstData) / bytesPerInFrame; pf < frames {
		frames = pf
	}
	if frames <= 0 {
		t.Fatalf("frame count reconciled to %d; cannot proceed", frames)
	}
	totalSamples := frames * FrameSamples

	src := make([]int16, totalSamples)
	pst := make([]int16, totalSamples)
	for n := 0; n < totalSamples; n++ {
		src[n] = int16(binary.LittleEndian.Uint16(inData[2*n : 2*n+2]))
		pst[n] = int16(binary.LittleEndian.Uint16(pstData[2*n : 2*n+2]))
	}

	// Pipeline B: ITU canonical bitstream → our decoder.
	bReader := bytes.NewReader(bitData)
	decB := NewDecoder()
	pipelineB := make([]int16, totalSamples)
	var packed [FrameBytes]byte
	for f := 0; f < frames; f++ {
		if _, rerr := bitstream.ReadG192Frame(bReader, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, rerr)
		}
		if derr := decB.DecodeFrame(packed[:], pipelineB[f*FrameSamples:(f+1)*FrameSamples]); derr != nil {
			t.Fatalf("pipeline-B decode frame %d: %v", f, derr)
		}
	}

	// Pipeline C: our encoder → our decoder (full round-trip).
	enc := NewEncoder()
	decC := NewDecoder()
	pipelineC := make([]int16, totalSamples)
	for f := 0; f < frames; f++ {
		var rt [FrameBytes]byte
		if eerr := enc.EncodeFrame(src[f*FrameSamples:(f+1)*FrameSamples], rt[:]); eerr != nil {
			t.Fatalf("pipeline-C encode frame %d: %v", f, eerr)
		}
		if derr := decC.DecodeFrame(rt[:], pipelineC[f*FrameSamples:(f+1)*FrameSamples]); derr != nil {
			t.Fatalf("pipeline-C decode frame %d: %v", f, derr)
		}
	}

	// G.729A algorithmic delay is ~15 ms (1 frame + 5 ms lookahead = up
	// to 120 samples). The PST file may be aligned to encoder-input
	// frame index or to first-decoded-output, and our pipelines may
	// align differently. Search a ±240-sample offset window per pipeline
	// and report the best-aligned SNR. Best-shift SHOULD be consistent
	// across pipelines if all use the same canonical alignment.
	const maxShift = 240
	shA, gA, sA := bestAlignedSNR(src, pst, maxShift)
	shB, gB, sB := bestAlignedSNR(src, pipelineB, maxShift)
	shC, gC, sC := bestAlignedSNR(src, pipelineC, maxShift)

	// Sanity: detect catastrophic-silence failures (output amplitude
	// negligible). Helps distinguish "alignment fixed it" from
	// "pipeline produced silence".
	rmsA := rmsAmp(pst)
	rmsB := rmsAmp(pipelineB)
	rmsC := rmsAmp(pipelineC)
	rmsRef := rmsAmp(src)

	t.Logf("Phase 3 round-trip quality report — SPEECH corpus (%d frames, %d samples)",
		frames, totalSamples)
	t.Logf("Reference: SPEECH.IN (clean-room — SPEECH.PST excluded as relative reference per Phase 1o D-3 PSTdomain ambiguity).")
	t.Logf("Reference RMS amplitude: %.0f", rmsRef)
	t.Logf("")
	t.Logf("%-58s %6s %10s %10s %10s",
		"Pipeline", "shift", "RMS", "GlobalSNR", "SegSNR")
	t.Logf("%-58s %6s %10s %10s %10s",
		"--------", "(samp)", "       ", "  (dB)   ", "  (dB)  ")
	t.Logf("%-58s %6d %10.0f %10.2f %10.2f",
		"A: SPEECH.PST            (ITU enc + ITU dec — intrinsic)", shA, rmsA, gA, sA)
	t.Logf("%-58s %6d %10.0f %10.2f %10.2f",
		"B: SPEECH.BIT → ourDec   (ITU enc + our dec)", shB, rmsB, gB, sB)
	t.Logf("%-58s %6d %10.0f %10.2f %10.2f",
		"C: SPEECH.IN  → ourEnc → ourDec  (full round-trip)", shC, rmsC, gC, sC)
	t.Logf("")
	t.Logf("Encoder perceptual contribution (C − B): GlobalSNR Δ = %+.2f dB ; SegSNR Δ = %+.2f dB",
		gC-gB, sC-sB)
	t.Logf("Decoder perceptual contribution (B − A): GlobalSNR Δ = %+.2f dB ; SegSNR Δ = %+.2f dB",
		gB-gA, sB-sA)
	t.Logf("")
	t.Logf("Reference baselines (Salami 1998 IEEE T-SAP §V.B Table II): G.729A SegSNR ~12–14 dB on clean speech.")
	t.Logf("Decision rule (informational): C-SegSNR ≥ A-SegSNR − 2 dB → encoder perceptual loss negligible (path A acceptable).")
	t.Logf("                              C-SegSNR ≥ A-SegSNR − 5 dB → bounded perceptual loss   (path B candidate).")
	t.Logf("                              C-SegSNR <  A-SegSNR − 5 dB → destructive perceptual loss (path B/C forced).")
}

// globalSNRDB returns 10·log10(Σref² / Σ(ref−test)²) in dB.
// Quackenbush/Barnwell/Clements 1988 §2.3 (textbook).
func globalSNRDB(ref, test []int16) float64 {
	if len(ref) != len(test) || len(ref) == 0 {
		return math.NaN()
	}
	var sigE, errE float64
	for i := range ref {
		s := float64(ref[i])
		e := float64(ref[i]) - float64(test[i])
		sigE += s * s
		errE += e * e
	}
	if errE <= 0 {
		return math.Inf(+1)
	}
	if sigE <= 0 {
		return 0
	}
	return 10 * math.Log10(sigE/errE)
}

// segSNRDB returns the segmental SNR averaged over 80-sample (10ms)
// frames. Per-frame value clipped to [-10, +35] dB; silence frames
// (signal energy < 1) skipped. Quackenbush 1988 §2.4 (textbook).
func segSNRDB(ref, test []int16) float64 {
	if len(ref) != len(test) || len(ref) == 0 {
		return math.NaN()
	}
	const seg = FrameSamples
	var sum float64
	var count int
	for i := 0; i+seg <= len(ref); i += seg {
		var sigE, errE float64
		for j := 0; j < seg; j++ {
			s := float64(ref[i+j])
			e := float64(ref[i+j]) - float64(test[i+j])
			sigE += s * s
			errE += e * e
		}
		if sigE < 1 {
			continue
		}
		var snr float64
		if errE < 1 {
			snr = 35
		} else {
			snr = 10 * math.Log10(sigE/errE)
		}
		if snr < -10 {
			snr = -10
		}
		if snr > 35 {
			snr = 35
		}
		sum += snr
		count++
	}
	if count == 0 {
		return math.NaN()
	}
	return sum / float64(count)
}

func snrPair(ref, test []int16) (float64, float64) {
	return globalSNRDB(ref, test), segSNRDB(ref, test)
}

// bestAlignedSNR searches for the integer-sample shift in
// [-maxShift, +maxShift] that maximises GlobalSNR(ref, test_shifted),
// then returns (bestShift, GlobalSNR_at_shift, SegSNR_at_shift).
// Positive shift means test signal lags reference (test[k] aligns with
// ref[k-shift]). Used to compensate for G.729 algorithmic delay
// (typically ≤120 samples).
func bestAlignedSNR(ref, test []int16, maxShift int) (int, float64, float64) {
	bestShift := 0
	bestSNR := math.Inf(-1)
	for shift := -maxShift; shift <= maxShift; shift++ {
		var sigE, errE float64
		for i := 0; i < len(ref); i++ {
			j := i + shift
			if j < 0 || j >= len(test) {
				continue
			}
			s := float64(ref[i])
			e := s - float64(test[j])
			sigE += s * s
			errE += e * e
		}
		if errE <= 0 {
			return shift, math.Inf(+1), math.Inf(+1)
		}
		snr := 10 * math.Log10(sigE/errE)
		if snr > bestSNR {
			bestSNR = snr
			bestShift = shift
		}
	}
	// Recompute SegSNR at the discovered shift, using the overlapping
	// region only (truncate ref/test to align).
	shifted := make([]int16, len(ref))
	for i := range ref {
		j := i + bestShift
		if j >= 0 && j < len(test) {
			shifted[i] = test[j]
		}
	}
	return bestShift, bestSNR, segSNRDB(ref, shifted)
}

// rmsAmp returns the RMS amplitude of a signal in raw int16 units.
func rmsAmp(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var e float64
	for _, v := range s {
		e += float64(v) * float64(v)
	}
	return math.Sqrt(e / float64(len(s)))
}
