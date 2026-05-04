package decoder

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase3bDiag4_PostfilterBypass is the Phase 3b DIAG-4 discriminator
// for candidate D-1 (postfilter long-term filter memory + pitch-period
// delay line + tilt + AGC).
//
// Background (see docs Appendix D / G):
//
//	Pipeline B (ITU encoder → our decoder) shows a corpus-wide
//	cross-correlation peak shift of −22 samples vs reference SPEECH.PST
//	while the intrinsic ITU-vs-ITU pipeline A reports +40 samples — a
//	62-sample gap that is amplitude-blind and survives both Phase 3b
//	DIAG-1 (gain MA predictor seed, EXONERATED candidate B) and DIAG-2
//	(LP interpolation, EXONERATED candidate C) and DIAG-3 (AC FIFO,
//	EXONERATED candidate D-2).
//
// Method:
//
//	Build two pipelines side-by-side over the entire SPEECH.BIT corpus:
//	  A_pf   : Decoder.Decode    — full pipeline (synthesis + postfilter
//	                               + HP + ScaleUpSat). Current default.
//	  A_raw  : Decoder.DecodeFrameNoPostfilter — synthesis output fed
//	                               directly to HP filter, postfilter
//	                               chain skipped (test-only shim;
//	                               structurally identical to Decode
//	                               except for the pst.Filter call).
//	Compare both signals against SPEECH.PST (REF_pf, ITU post-postfilter
//	reference). REF_raw (a pre-postfilter ITU reference) is NOT shipped
//	in the Annex A test_vectors directory; the discriminator therefore
//	runs against REF_pf only and the verdict is informative — a
//	pre-postfilter shift of ≈0 against a post-postfilter reference
//	would still indicate that the postfilter contributes the observed
//	alignment skew, but the absolute alignment number against a missing
//	REF_raw cannot be verified.
//
// Verdict logic (logged, not asserted):
//
//	shift(A_raw) ≈ 0 (within ±2)         → D-1 CONFIRMED
//	shift(A_raw) ≈ shift(A_pf) (Δ ≤ 2)   → D-1 EXONERATED
//	|shift(A_raw)| < |shift(A_pf)|       → D-1 PARTIALLY CONFIRMED
//
// Spec citations (clean-room): ITU-T G.729 §A.4.2 (Annex A postfilter,
// long-term + short-term + tilt + AGC), §3.10 / §4.1.6 (synthesis), and
// §4.2.2 (output HP filter). No reference C / bcg729 / FFmpeg consulted.
//
// Informational: t.Logf only.
func TestPhase3bDiag4_PostfilterBypass(t *testing.T) {
	const (
		bytesPerOutFrame = 2 * frameSamples
	)
	vecDir := filepath.Join("..", "..", "testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	bitPath := filepath.Join(vecDir, "SPEECH.BIT")
	pstPath := filepath.Join(vecDir, "SPEECH.PST")
	inPath := filepath.Join(vecDir, "SPEECH.IN")

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read SPEECH.IN: %v", err)
	}

	frames := len(pstData) / bytesPerOutFrame
	if inFrames := len(inData) / bytesPerOutFrame; inFrames < frames {
		frames = inFrames
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	totalSamples := frames * frameSamples

	refPST := make([]int16, totalSamples)
	refIN := make([]int16, totalSamples)
	for n := 0; n < totalSamples; n++ {
		refPST[n] = int16(binary.LittleEndian.Uint16(pstData[2*n : 2*n+2]))
		refIN[n] = int16(binary.LittleEndian.Uint16(inData[2*n : 2*n+2]))
	}

	// Pipeline A_pf: full default decode.
	apf := make([]int16, totalSamples)
	{
		var decFull Decoder
		var packed [bitstream.FrameBytes]byte
		r := bytes.NewReader(bitData)
		for f := 0; f < frames; f++ {
			if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
				t.Fatalf("ReadG192Frame[A_pf] frame %d: %v", f, rerr)
			}
			if derr := decFull.Decode(packed[:], false, apf[f*frameSamples:(f+1)*frameSamples]); derr != nil {
				t.Fatalf("Decode[A_pf] frame %d: %v", f, derr)
			}
		}
	}

	// Pipeline A_raw: postfilter bypassed (HP+ScaleUpSat retained).
	araw := make([]int16, totalSamples)
	{
		var decRaw Decoder
		var packed [bitstream.FrameBytes]byte
		r := bytes.NewReader(bitData)
		for f := 0; f < frames; f++ {
			if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
				t.Fatalf("ReadG192Frame[A_raw] frame %d: %v", f, rerr)
			}
			if derr := decRaw.DecodeFrameNoPostfilter(packed[:], araw[f*frameSamples:(f+1)*frameSamples]); derr != nil {
				t.Fatalf("DecodeFrameNoPostfilter[A_raw] frame %d: %v", f, derr)
			}
		}
	}

	const maxShift = 240
	shPF, gPF, sPF := diag4BestAligned(refPST, apf, maxShift)
	shRAW, gRAW, sRAW := diag4BestAligned(refPST, araw, maxShift)
	shPFin, gPFin, sPFin := diag4BestAligned(refIN, apf, maxShift)
	shRAWin, gRAWin, sRAWin := diag4BestAligned(refIN, araw, maxShift)
	// Also: ITU-PST vs SPEECH.IN (pipeline A intrinsic), as a calibration
	// anchor matching the +40-sample shift reported in Appendix D.2.
	shAA, gAA, sAA := diag4BestAligned(refIN, refPST, maxShift)

	rmsPF := diag4Rms(apf)
	rmsRAW := diag4Rms(araw)
	rmsRefPST := diag4Rms(refPST)
	rmsRefIN := diag4Rms(refIN)
	maxPF := diag4MaxAbs(apf)
	maxRAW := diag4MaxAbs(araw)

	t.Logf("Phase 3b DIAG-4 — postfilter-bypass discriminator (SPEECH corpus, %d frames, %d samples)",
		frames, totalSamples)
	t.Logf("REF_pf = SPEECH.PST (RMS=%.0f). REF_raw (pre-postfilter ITU reference) NOT shipped.", rmsRefPST)
	t.Logf("REF_in = SPEECH.IN  (RMS=%.0f) — upstream PCM, used by Appendix D's pipeline-A/B comparison.", rmsRefIN)
	t.Logf("Calibration anchor: ITU-PST vs SPEECH.IN  shift = %+d  GlobalSNR = %.2f dB  SegSNR = %.2f dB",
		shAA, gAA, sAA)
	t.Logf("")
	t.Logf("=== vs REF_pf (SPEECH.PST) ===")
	t.Logf("%-14s %8s %8s %12s %12s %14s",
		"Variant", "rms", "max|s|", "SegSNR(dB)", "XCorrShift", "GlobalSNR(dB)")
	t.Logf("%-14s %8s %8s %12s %12s %14s",
		"-------", "---", "------", "----------", "----------", "-------------")
	t.Logf("%-14s %8.0f %8d %12.2f %12d %14.2f",
		"A_pf", rmsPF, maxPF, sPF, shPF, gPF)
	t.Logf("%-14s %8.0f %8d %12.2f %12d %14.2f",
		"A_raw", rmsRAW, maxRAW, sRAW, shRAW, gRAW)
	t.Logf("%-14s %8s %8s %12s %12s %14s",
		"A_pf_only_lt", "—", "—", "deferred", "—", "—")
	t.Logf("")
	t.Logf("=== vs REF_in (SPEECH.IN) — anchor for Appendix D.2 (−22 vs +40) ===")
	t.Logf("%-14s %8s %8s %12s %12s %14s",
		"Variant", "rms", "max|s|", "SegSNR(dB)", "XCorrShift", "GlobalSNR(dB)")
	t.Logf("%-14s %8s %8s %12s %12s %14s",
		"-------", "---", "------", "----------", "----------", "-------------")
	t.Logf("%-14s %8.0f %8d %12.2f %12d %14.2f",
		"A_pf", rmsPF, maxPF, sPFin, shPFin, gPFin)
	t.Logf("%-14s %8.0f %8d %12.2f %12d %14.2f",
		"A_raw", rmsRAW, maxRAW, sRAWin, shRAWin, gRAWin)
	t.Logf("")
	t.Logf("Δ shift vs REF_pf (A_raw − A_pf) = %+d samples", shRAW-shPF)
	t.Logf("Δ shift vs REF_in (A_raw − A_pf) = %+d samples", shRAWin-shPFin)
	t.Logf("")

	const eps = 2
	verdict := ""
	// Use vs-PST shift as the primary signal — REF_pf is at the same
	// processing stage as A_pf and removes encoder/HP delay confounders.
	switch {
	case absInt(shRAW) <= eps && absInt(shPF) <= eps:
		verdict = "D-1 EXONERATED (revised) — A_pf already aligns within ±2 samples vs REF_pf; the −22 reported in Appendix D.2 is a measurement artifact of bestAlignedSNR vs SPEECH.IN at low SNR, not a postfilter defect. Postfilter is not the source of any phase skew."
	case absInt(shRAW) <= eps:
		verdict = "D-1 CONFIRMED — bypassing postfilter eliminates the alignment skew (|shift(A_raw)| ≤ 2)."
	case absInt(shRAW-shPF) <= eps:
		verdict = "D-1 EXONERATED — bypassing postfilter does NOT change the alignment skew (|Δshift| ≤ 2). Drill D-3 / D-4."
	case absInt(shRAW) < absInt(shPF):
		verdict = "D-1 PARTIALLY CONFIRMED — postfilter contributes part of the shift but a residual upstream skew remains. Drill D-3 / D-4 too."
	default:
		verdict = "INDETERMINATE — bypass made shift WORSE; postfilter is masking an upstream defect. Drill D-3 / D-4 carefully."
	}
	t.Logf("Verdict: %s", verdict)

	// =====================================================================
	// Step 3 — supplementary D-3 (synthesis 1/Â(z) memory) / D-4 (excitation
	// memory cold-start) drill, conditional on D-1 not being CONFIRMED to
	// fully account for the residual amplitude defect.
	//
	// Spec: §4.1.6 (excitation initialised to zero); §3.10 / §4.1.2
	// (synthesis filter pastSynth initialised to zero).
	//
	// Method: re-decode the first 5 frames via DecodeWithTaps, dumping
	// per-subframe rms(u), rms(s), max|u|, max|s|, plus the cold-start
	// past-excitation FIFO snapshot (which should be all-zero at frame 0
	// before any decode call per §4.1.6) verified via PastExcSnapshot if
	// available. If not, this section reports U[0]/S[0] of subframe 0 of
	// frame 0 directly — those are the literal first samples produced by
	// the spec equations and let us hand-check.
	t.Logf("")
	t.Logf("=== Supplementary D-3 / D-4 drill (first 5 frames) ===")
	{
		var dec Decoder
		var packed [bitstream.FrameBytes]byte
		r := bytes.NewReader(bitData)
		// Cold-start invariant: zero-value Decoder has pastExc all-zero
		// (verified by structural identity; the zero value of
		// [pastExcLen]int16 is all-zero per Go spec). This matches
		// ITU-T G.729 §4.1.6 / §4.3 cold-start requirement.
		var coldZero bool = true
		for i := range dec.pastExc {
			if dec.pastExc[i] != 0 {
				coldZero = false
				break
			}
		}
		t.Logf("Cold-start pastExc[0..152] == 0 : %v  (spec §4.1.6 cold-start contract)", coldZero)
		t.Logf("%5s %3s %10s %10s %8s %8s",
			"frame", "sf", "rms(u)", "rms(s)", "max|u|", "max|s|")
		for fi := 0; fi < 5; fi++ {
			if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
				t.Fatalf("ReadG192Frame[drill] frame %d: %v", fi, rerr)
			}
			taps, derr := dec.DecodeWithTaps(packed[:])
			if derr != nil {
				t.Fatalf("DecodeWithTaps[drill] frame %d: %v", fi, derr)
			}
			for sf := 0; sf < 2; sf++ {
				uR := diag4Rms(taps.Sub[sf].U[:])
				sR := diag4Rms(taps.Sub[sf].S[:])
				maxU := diag4MaxAbs(taps.Sub[sf].U[:])
				maxS := diag4MaxAbs(taps.Sub[sf].S[:])
				t.Logf("%5d %3d %10.2f %10.2f %8d %8d",
					fi, sf+1, uR, sR, maxU, maxS)
			}
			if fi == 0 {
				t.Logf("  frame0 sf1 U[0:8] = %v", taps.Sub[0].U[:8])
				t.Logf("  frame0 sf1 S[0:8] = %v", taps.Sub[0].S[:8])
			}
		}
		t.Logf("")
		t.Logf("Per-frame ratio rms(s)/rms(u) tracks 1/Â(z) gain (no cold-start blow-up expected; spec §3.10 overflow-recovery scales by ¼ on overflow).")
		t.Logf("If rms(s)/rms(u) is consistent with the LP filter spectral envelope (≈1..3 typical) the synthesis filter (D-3) is healthy; a defect would manifest as anomalous decay or amplification.")
	}
}

// diag4BestAligned mirrors the bestAlignedSNR contract used by
// phase3_roundtrip_quality_test.go (same alignment metric — search
// integer-sample shift in [-maxShift, +maxShift] that maximises
// GlobalSNR(ref, test_shifted)).
//
// Positive shift: test signal lags reference (test[k] aligns with
// ref[k-shift]).
//
// Returns (bestShift, GlobalSNR_at_shift_dB, SegSNR_at_shift_dB).
func diag4BestAligned(ref, test []int16, maxShift int) (int, float64, float64) {
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
	shifted := make([]int16, len(ref))
	for i := range ref {
		j := i + bestShift
		if j >= 0 && j < len(test) {
			shifted[i] = test[j]
		}
	}
	return bestShift, bestSNR, diag4SegSNR(ref, shifted)
}

// diag4SegSNR mirrors segSNRDB from phase3_roundtrip_quality_test.go:
// per-frame SNR averaged over 80-sample frames, clipped to [-10, +35] dB,
// silence frames (sigE < 1) skipped. Quackenbush 1988 §2.4 (textbook).
func diag4SegSNR(ref, test []int16) float64 {
	if len(ref) != len(test) || len(ref) == 0 {
		return math.NaN()
	}
	const seg = frameSamples
	var sum float64
	var count int
	for i := 0; i+seg <= len(ref); i += seg {
		var sigE, errE float64
		for j := 0; j < seg; j++ {
			s := float64(ref[i+j])
			e := s - float64(test[i+j])
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

func diag4Rms(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var e float64
	for _, v := range s {
		e += float64(v) * float64(v)
	}
	return math.Sqrt(e / float64(len(s)))
}

func diag4MaxAbs(s []int16) int {
	m := 0
	for _, v := range s {
		a := int(v)
		if a < 0 {
			a = -a
		}
		if a > m {
			m = a
		}
	}
	return m
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
