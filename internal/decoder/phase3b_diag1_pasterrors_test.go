package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3bDiag1_PastErrorsTrajectory drives SPEECH.BIT through the
// full decoder, capturing the per-subframe evolution of the gain
// MA-predictor FIFO Û(m-1..m-4) (G.729 §3.9.1 eq. (69)) so that
// OQ-PASTSEED (cold-start value) and OQ-PASTPROG (zero-energy guard
// re-seed strategy) — see plan
// docs/superpowers/plans/2026-05-04-phase3b-alignment-fix-plan.md §3 —
// can be pinned against corpus evidence.
//
// Test is informational (t.Logf only); always passes. Designed to feed
// Appendix E of the diagnostic report
// docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md.
func TestPhase3bDiag1_PastErrorsTrajectory(t *testing.T) {
	const bytesPerBitFrame = 164 // G.192 frame size on disk
	bitPath := filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", "SPEECH.BIT")
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	frames := len(bitData) / bytesPerBitFrame
	if frames <= 0 {
		t.Fatalf("frame count = %d, cannot proceed", frames)
	}
	totalSubframes := frames * 2

	type subRecord struct {
		preTaps   [4]int16
		predicted int32
		gpIdx     uint8
		gcIdx     uint8
		gpQ14     int16
		gcMantQ14 int16
		gcExp     int8
		gammaCQ13 int32
		uCurrent  int16
		guard     bool
	}
	records := make([]subRecord, 0, totalSubframes)

	// Drive the decoder in our own loop so we can snapshot the gain
	// predictor BEFORE each Decode call. We mirror DecodeWithTaps
	// (phase3diag_taps_export_test.go) line-for-line, splitting the
	// per-subframe block into a pre-snapshot + decodeSubframeWithTaps
	// invocation. State advancement is identical to the production
	// Decoder.Decode pathway, so the per-subframe predictor evolution
	// captured here matches what production decoding produces.
	var d Decoder
	bReader := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte
	for fi := 0; fi < frames; fi++ {
		if _, rerr := bitstream.ReadG192Frame(bReader, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", fi, rerr)
		}
		var f bitstream.Frame
		if perr := bitstream.Unpack(packed[:], &f); perr != nil {
			t.Fatalf("Unpack frame %d: %v", fi, perr)
		}

		sf1A, sf2A := d.lsp.Decode(lsp.Indices{
			L0: uint8(f.L0), L1: uint8(f.L1),
			L2: uint8(f.L2), L3: uint8(f.L3),
		})
		tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
		_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
		tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

		var out [frameSamples]int16

		for k, blk := range [2]struct {
			sfA   *[lpcOrder + 1]int16
			tInt  int
			tFrac int
			C     uint16
			S     uint8
			GA    uint8
			GB    uint8
			out   []int16
		}{
			{&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen]},
			{&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples]},
		} {
			var rec subRecord
			rec.preTaps = d.gn.PastErrorsSnapshot()
			rec.gpIdx = blk.GA
			rec.gcIdx = blk.GB

			// Build C[40] for this subframe so we can call
			// gain.Decoder.DecodeWithFullTaps via decodeSubframeWithTaps.
			var taps Phase3DiagSubframeTaps
			d.decodeSubframeWithTaps(blk.sfA, blk.tInt, blk.tFrac,
				blk.C, blk.S, blk.GA, blk.GB, blk.out, &taps)

			rec.predicted = taps.GainTaps.Predicted
			rec.gpQ14 = taps.GainTaps.GpQ14Final
			rec.gcMantQ14 = taps.GainTaps.GcMantQ14
			rec.gcExp = taps.GainTaps.GcExp
			rec.gammaCQ13 = taps.GainTaps.GammaCQ13
			rec.guard = taps.GainTaps.ZeroEnergyGuard

			// uCurrent = the value FIFO-shifted into pastErrors[0]
			// this subframe. Re-derive from the predictor's spec
			// rule so we report exactly what production code wrote
			// (matches gain/decode.go lines ~123-143 / §4.4.3 bound).
			postTaps := d.gn.PastErrorsSnapshot()
			rec.uCurrent = postTaps[0]
			_ = k
			records = append(records, rec)
		}

		pcm.ScaleUpSat(out[:frameSamples], out[:frameSamples])
	}

	// silence unused-import warnings if the helpers above ever change.
	_ = fcb.Indices{}
	_ = synth.Synthesizer{}
	_ = gain.PastErrorsDefault

	// ── Section 1: first-50-subframe trajectory ─────────────────────
	t.Logf("Phase 3b DIAG-1 — pastErrors trajectory")
	t.Logf("Corpus: SPEECH.BIT, %d frames = %d subframes", frames, totalSubframes)
	t.Logf("Spec seed Û_init = pastErrorsDefault = %d (= -14 dB Q10 per §4.3 Table 9)", gain.PastErrorsDefault)
	t.Logf("")
	t.Logf("First-50-subframe trajectory (preTaps in Q10 dB; preTaps[0]=Û(m-1) most recent):")
	t.Logf("%4s | %7s %7s %7s %7s | %8s %8s | %5s",
		"sf#", "Û(m-1)", "Û(m-2)", "Û(m-3)", "Û(m-4)", "pred", "uCur", "guard")
	first := 50
	if first > len(records) {
		first = len(records)
	}
	for i := 0; i < first; i++ {
		r := records[i]
		guardStr := ""
		if r.guard {
			guardStr = "GUARD"
		}
		t.Logf("%4d | %7d %7d %7d %7d | %8d %8d | %5s",
			i, r.preTaps[0], r.preTaps[1], r.preTaps[2], r.preTaps[3],
			r.predicted, r.uCurrent, guardStr)
	}
	t.Logf("")

	// ── Section 2: long-run statistics (sf 50..end) ─────────────────
	mean := func(xs []float64) float64 {
		if len(xs) == 0 {
			return math.NaN()
		}
		var s float64
		for _, v := range xs {
			s += v
		}
		return s / float64(len(xs))
	}
	stddev := func(xs []float64, m float64) float64 {
		if len(xs) < 2 {
			return math.NaN()
		}
		var s float64
		for _, v := range xs {
			d := v - m
			s += d * d
		}
		return math.Sqrt(s / float64(len(xs)-1))
	}

	residuals := make([]float64, 0, len(records))
	predicteds := make([]float64, 0, len(records))
	uCurrents := make([]float64, 0, len(records))
	guardCount := 0
	const skip = 50
	for i := skip; i < len(records); i++ {
		r := records[i]
		if r.guard {
			guardCount++
		}
		residuals = append(residuals, float64(r.predicted)-float64(r.uCurrent))
		predicteds = append(predicteds, float64(r.predicted))
		uCurrents = append(uCurrents, float64(r.uCurrent))
	}
	mR, mP, mU := mean(residuals), mean(predicteds), mean(uCurrents)
	sR, sP, sU := stddev(residuals, mR), stddev(predicteds, mP), stddev(uCurrents, mU)

	t.Logf("Long-run statistics (subframes %d..%d, n=%d):", skip, len(records)-1, len(residuals))
	t.Logf("  predicted - uCurrent  : mean = %+.2f Q10 dB (= %+.3f dB),  stddev = %.2f Q10",
		mR, mR/1024.0, sR)
	t.Logf("  predicted             : mean = %+.2f Q10 dB (= %+.3f dB),  stddev = %.2f Q10",
		mP, mP/1024.0, sP)
	t.Logf("  uCurrent              : mean = %+.2f Q10 dB (= %+.3f dB),  stddev = %.2f Q10",
		mU, mU/1024.0, sU)
	guardPct := 100.0 * float64(guardCount) / float64(len(residuals))
	// Also count guard hits across the WHOLE corpus including cold start.
	guardAll := 0
	for _, r := range records {
		if r.guard {
			guardAll++
		}
	}
	t.Logf("  zero-energy guard fired: %d of %d subframes in window (%.3f%%); %d of %d total (%.3f%%)",
		guardCount, len(residuals), guardPct,
		guardAll, len(records), 100.0*float64(guardAll)/float64(len(records)))
	t.Logf("")

	// ── Section 3: cold-start convergence indicators ────────────────
	const seed = int16(-14336)
	leave0 := -1
	for i, r := range records {
		if r.preTaps[0] != seed {
			leave0 = i
			break
		}
	}
	leave3 := -1
	for i, r := range records {
		if r.preTaps[3] != seed {
			leave3 = i
			break
		}
	}
	t.Logf("Cold-start convergence:")
	t.Logf("  pastErrors[0] (newest) first leaves seed (-14336) at sf# = %d", leave0)
	t.Logf("  pastErrors[3] (oldest) first leaves seed (-14336) at sf# = %d", leave3)
	t.Logf("  (expect leave0 = 1 [seed consumed by sf 0, shifted in sf 1] and leave3 = 4 if cold-start is monotone)")
	t.Logf("")

	// ── Section 4: cold-start bias check (first 10 subframes) ───────
	if len(records) >= 10 {
		first10 := make([]float64, 10)
		for i := 0; i < 10; i++ {
			first10[i] = float64(records[i].predicted)
		}
		mFirst := mean(first10)
		t.Logf("Cold-start bias check:")
		t.Logf("  mean(predicted) sf 0..9   = %+.2f Q10 dB (= %+.3f dB)", mFirst, mFirst/1024.0)
		t.Logf("  mean(predicted) sf 50..end= %+.2f Q10 dB (= %+.3f dB)", mP, mP/1024.0)
		t.Logf("  Δ (cold − long-run)       = %+.2f Q10 dB (= %+.3f dB)", mFirst-mP, (mFirst-mP)/1024.0)
		t.Logf("  Spec reference: §3.9.1 E̅ = 30 dB (tables.GainMeanEnergyQ10 = 30720). With all four")
		t.Logf("  taps at -14336 (Q10), Σ b_i·Û(m-i) = -14·(0.68+0.58+0.34+0.19) = -14·1.79 = -25.06 dB,")
		t.Logf("  predicted = E̅ + Σ = 30 - 25.06 = 4.94 dB (= 5058 Q10). This is the expected sf-0 value.")
	}
	t.Logf("")

	// ── Section 5: encoder/decoder seed parity probe ────────────────
	t.Logf("ENCODER SEED PARITY: DEFERRED to IMPL-2 symmetry test")
	t.Logf("  (encoder uses gain.PastErrorsDefault = %d as cold-start seed per gainquant/predictor.go;",
		gain.PastErrorsDefault)
	t.Logf("  byte-exact symmetry on a synthetic idx sequence is a separate test.)")
}
