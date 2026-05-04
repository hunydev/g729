package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/lsp"
)

// TestPhase3bDiag2_LPInterpolationTrajectory drives SPEECH.BIT through
// the production decoder, capturing per-frame the LSP state used by
// the inter-subframe interpolation prescribed by ITU-T G.729 §3.2.5 eq.
// (24) (the same equation referenced from §4.1.5 for the decoder side
// via the substitution q_i → q̂_i):
//
//	Subframe 1 : q̂_i^(1) = 0.5·q̂_i^(prev) + 0.5·q̂_i^(curr)   i = 1..10
//	Subframe 2 : q̂_i^(2) = q̂_i^(curr)                          i = 1..10
//
// Spec semantics pinned by this diagnostic (see Appendix F of
// docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md):
//
//   - domain     : LSP cosine-domain (q_i = cos ω̂_i), Q15
//   - sf-1 input : 50/50 unweighted average of q̂_i^(prev) and q̂_i^(curr)
//   - sf-2 input : q̂_i^(curr) (the freshly-decoded LSP for this frame)
//   - quantized substitution : same equation, q_i → q̂_i (§3.2.5 final
//     paragraph; §4.1.5 pulls this in by reference for the decoder)
//
// Code site under test: internal/lsp/interpolate.go::interpolateLSP
// (called from internal/lsp/decoder.go::Decoder.Decode step 7).
//
// Test is informational (t.Logf only); always passes. Designed to
// feed Appendix F (OQ-LP-INTERP pin + candidate-C verdict).
func TestPhase3bDiag2_LPInterpolationTrajectory(t *testing.T) {
	const bytesPerBitFrame = 164
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

	type frameRec struct {
		prevLSP [10]int16 // q̂_i^(m-1) Q15 (cosine domain), as fed to interp
		currLSP [10]int16 // q̂_i^(m)   Q15 — the freshly-decoded LSP
		sf1LSP  [10]int16 // interpolated LSP for subframe 1 Q15
		sf2LSP  [10]int16 // = currLSP, by spec
		sf1A    [11]int16 // a[0..10] Q12 from sf1LSP
		sf2A    [11]int16 // a[0..10] Q12 from sf2LSP (= currLSP)
		// alternative reconstructions of sf-1 LP (for §F.7 dump):
		sf0AltCurr [11]int16 // alt-1: sf-0 uses q̂_i^(m)   (no interp)
		sf0AltPrev [11]int16 // alt-2: sf-0 uses q̂_i^(m-1) (previous frame)
	}
	records := make([]frameRec, 0, frames)

	var d Decoder
	bReader := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte

	for fi := 0; fi < frames; fi++ {
		if _, rerr := bitstream.ReadG192Frame(bReader, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", fi, rerr)
		}

		// Snapshot prevLSP BEFORE Decode. On frame 0 this is the
		// zero-state (replaced inside Decode by initialPrevLSP =
		// cos(i·π/11) Q15 per §3.2.4 / §4.1.5 lazy init). We
		// substitute the spec init here when uninitialised so the
		// "what was actually fed to interpolateLSP" record is
		// faithful.
		prevSnap := d.lsp.PrevLSPSnapshot()
		init := d.lsp.InitializedForDiag()

		// Drive production Decode (advances state identically to the
		// roundtrip pipeline — uses the same internal lsp.Decoder.Decode
		// pathway).
		var out [frameSamples]int16
		if derr := d.Decode(packed[:], false, out[:]); derr != nil {
			t.Fatalf("Decode frame %d: %v", fi, derr)
		}

		// Snapshot AFTER Decode: lsp.Decoder saves the freshly-
		// decoded LSP into prevLSP (decoder.go:108), so the
		// post-snapshot equals q̂_i^(m) of frame fi.
		currSnap := d.lsp.PrevLSPSnapshot()

		// Replicate the spec interpolation outside the production
		// decoder so we can dump the inputs and the resulting Â(z)
		// for both subframes plus the two alternative sf-0
		// reconstructions for §F.7.
		var rec frameRec
		if init {
			rec.prevLSP = prevSnap
		} else {
			// First-frame: production code uses the spec init
			// (cos(i·π/11) Q15, §3.2.4 / §4.1.5). Mirror that here
			// so the recorded "prevLSP fed to interp" matches what
			// production actually used.
			rec.prevLSP = initialPrevLSPMirror
		}
		rec.currLSP = currSnap
		for i := 0; i < 10; i++ {
			rec.sf1LSP[i] = int16((int32(rec.prevLSP[i]) + int32(rec.currLSP[i])) >> 1)
			rec.sf2LSP[i] = rec.currLSP[i]
		}
		lsp.LSPToLP(&rec.sf1LSP, &rec.sf1A)
		lsp.LSPToLP(&rec.sf2LSP, &rec.sf2A)
		// Alternatives for sf-0 LP (interp domain alt-1: no interp;
		// alt-2: previous frame's LSP frozen for one extra subframe):
		altCurr := rec.currLSP
		altPrev := rec.prevLSP
		lsp.LSPToLP(&altCurr, &rec.sf0AltCurr)
		lsp.LSPToLP(&altPrev, &rec.sf0AltPrev)

		records = append(records, rec)
	}

	t.Logf("Phase 3b DIAG-2 — LP interpolation trajectory")
	t.Logf("Corpus: SPEECH.BIT, %d frames", frames)
	t.Logf("Spec ref: G.729 (06/2012) §3.2.5 eq. (24); §4.1.5 (decoder-side)")
	t.Logf("Code under test: internal/lsp/interpolate.go::interpolateLSP")
	t.Logf("  sf1[i] = (prev[i] + curr[i]) >> 1   (Q15, cosine domain, floor rounding)")
	t.Logf("  sf2[i] = curr[i]")
	t.Logf("")

	// ── §F.5 First-10-frame trace ───────────────────────────────────
	t.Logf("First-10-frame LP trace (a[1..10] Q12; a[0]=4096 omitted):")
	maxTrace := 10
	if maxTrace > len(records) {
		maxTrace = len(records)
	}
	for i := 0; i < maxTrace; i++ {
		r := records[i]
		t.Logf("  frame %d sf-0 a[1..10] = %v", i, r.sf1A[1:])
		t.Logf("           sf-1 a[1..10] = %v", r.sf2A[1:])
	}
	t.Logf("")

	// ── §F.6 Stability + monotonicity statistics ────────────────────
	monoViolSF0 := 0
	monoViolSF1 := 0
	unstableSF0 := 0
	unstableSF1 := 0
	for _, r := range records {
		if !lspMonotoneDecreasing(&r.sf1LSP) {
			monoViolSF0++
		}
		if !lspMonotoneDecreasing(&r.sf2LSP) {
			monoViolSF1++
		}
		if !azStableSchurCohn(&r.sf1A) {
			unstableSF0++
		}
		if !azStableSchurCohn(&r.sf2A) {
			unstableSF1++
		}
	}
	t.Logf("LSP monotonicity (cos-domain strictly decreasing q[0]>...>q[9]):")
	t.Logf("  sf-0 (interpolated) violations : %d / %d (%.4f%%)",
		monoViolSF0, len(records), 100.0*float64(monoViolSF0)/float64(len(records)))
	t.Logf("  sf-1 (current LSP) violations  : %d / %d (%.4f%%)",
		monoViolSF1, len(records), 100.0*float64(monoViolSF1)/float64(len(records)))
	t.Logf("Â(z) stability (Schur–Cohn step-down on Q12 a[1..10]; |k_m|<1 ∀m):")
	t.Logf("  sf-0 (interpolated) unstable    : %d / %d (%.4f%%)",
		unstableSF0, len(records), 100.0*float64(unstableSF0)/float64(len(records)))
	t.Logf("  sf-1 (current LSP) unstable     : %d / %d (%.4f%%)",
		unstableSF1, len(records), 100.0*float64(unstableSF1)/float64(len(records)))
	t.Logf("")

	// ── §F.6 Frame-to-frame LSP velocity (long-run, sf 0..end) ──────
	velocities := make([]float64, 0, len(records))
	for _, r := range records {
		velocities = append(velocities, l2DeltaQ15(&r.currLSP, &r.prevLSP))
	}
	mV := mean64(velocities)
	sV := stddev64(velocities, mV)
	t.Logf("Frame-to-frame LSP velocity ||q̂(m) − q̂(m-1)||₂ in Q15 cosine units:")
	t.Logf("  mean   = %.2f Q15 (= %.5f normalised)", mV, mV/32768.0)
	t.Logf("  stddev = %.2f Q15", sV)
	t.Logf("")

	// ── §F.7 Subframe-boundary delta analysis ───────────────────────
	// For each frame m: compute
	//   D_within = ||a_sf0(m) − a_sf1(m)||₂   (interp vs current)
	//   D_across = ||a_sf1(m) − a_sf1(m-1)||₂ (current vs prev current)
	// If interpolation is doing its job, sf-0 should sit between
	// sf-1(m-1) and sf-1(m) — i.e. closer to sf-1(m-1) than sf-1(m) is.
	// Probe:
	//   D_sf0_to_prevSf1 = ||a_sf0(m) − a_sf1(m-1)||₂
	//   D_sf1_to_prevSf1 = ||a_sf1(m) − a_sf1(m-1)||₂
	// Expectation: D_sf0_to_prevSf1 < D_sf1_to_prevSf1 on average.
	dWithin := make([]float64, 0, len(records))
	dAcross := make([]float64, 0, len(records)-1)
	dSf0ToPrev := make([]float64, 0, len(records)-1)
	dSf1ToPrev := make([]float64, 0, len(records)-1)
	for i, r := range records {
		dWithin = append(dWithin, l2DeltaA(&r.sf1A, &r.sf2A))
		if i == 0 {
			continue
		}
		prev := records[i-1]
		dAcross = append(dAcross, l2DeltaA(&r.sf2A, &prev.sf2A))
		dSf0ToPrev = append(dSf0ToPrev, l2DeltaA(&r.sf1A, &prev.sf2A))
		dSf1ToPrev = append(dSf1ToPrev, l2DeltaA(&r.sf2A, &prev.sf2A))
	}
	t.Logf("Subframe-boundary LP-coefficient deltas (Q12, L2 over a[1..10]):")
	t.Logf("  mean ||a_sf0(m) − a_sf1(m)||₂   (within-frame)        = %.2f", mean64(dWithin))
	t.Logf("  mean ||a_sf1(m) − a_sf1(m-1)||₂ (across-frame)        = %.2f", mean64(dAcross))
	t.Logf("  mean ||a_sf0(m) − a_sf1(m-1)||₂ (sf-0 vs prev sf-1)   = %.2f", mean64(dSf0ToPrev))
	t.Logf("  mean ||a_sf1(m) − a_sf1(m-1)||₂ (sf-1 vs prev sf-1)   = %.2f", mean64(dSf1ToPrev))
	t.Logf("  Interpretation: sf-0 should be roughly midway between")
	t.Logf("  sf-1(m-1) and sf-1(m). Expect mean(sf-0 vs prev sf-1) ≈")
	t.Logf("  mean(sf-1 vs prev sf-1)/2 if interp is well-formed.")
	t.Logf("")

	// ── §F.7 (d) 5 representative voiced frames — simplified dump ───
	// Pick by largest frame-to-frame LSP velocity (a robust proxy for
	// active voiced content where LP coefficients move fastest).
	// Sample-resolution alternatives are dumped as L2 distances; full
	// in-test waveform reconstruction is deferred (would require
	// re-running synth.Filter with three different a-vectors and
	// matching pastSynth memory — invasive and out of scope per the
	// DIAG-2 task description).
	type rank struct {
		idx int
		vel float64
	}
	ranks := make([]rank, len(velocities))
	for i, v := range velocities {
		ranks[i] = rank{i, v}
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i].vel > ranks[j].vel })
	pickN := 5
	if pickN > len(ranks) {
		pickN = len(ranks)
	}
	t.Logf("5 highest-velocity (voiced-proxy) frames — sf-0 LP under three reconstructions:")
	t.Logf("  base = current production interp (sf1[i] = (prev+curr)>>1)")
	t.Logf("  alt1 = sf-0 uses q̂(m)   (no interpolation)")
	t.Logf("  alt2 = sf-0 uses q̂(m-1) (previous frame held)")
	t.Logf("  L2 distances over a[1..10] Q12:")
	for k := 0; k < pickN; k++ {
		i := ranks[k].idx
		r := records[i]
		dBaseAlt1 := l2DeltaA(&r.sf1A, &r.sf0AltCurr)
		dBaseAlt2 := l2DeltaA(&r.sf1A, &r.sf0AltPrev)
		dAlt1Alt2 := l2DeltaA(&r.sf0AltCurr, &r.sf0AltPrev)
		t.Logf("  frame %4d (vel=%.0f Q15): ||base−alt1||=%.2f  ||base−alt2||=%.2f  ||alt1−alt2||=%.2f",
			i, ranks[k].vel, dBaseAlt1, dBaseAlt2, dAlt1Alt2)
		t.Logf("       sf-0 base a[1..10] = %v", r.sf1A[1:])
		t.Logf("       sf-0 alt1 a[1..10] = %v", r.sf0AltCurr[1:])
		t.Logf("       sf-0 alt2 a[1..10] = %v", r.sf0AltPrev[1:])
	}
	t.Logf("")
	t.Logf("(d) status: SIMPLIFIED — LP-coefficient L2 distances only;")
	t.Logf("    full per-alternative synthesis cross-correlation deferred")
	t.Logf("    (would require re-driving synth.Filter with three a-vectors")
	t.Logf("    on identical pastSynth memory; invasive vs DIAG scope).")
}

// initialPrevLSPMirror duplicates internal/lsp.initialPrevLSP (the
// codec-start LSP init q_i = cos(i·π/11) Q15 per §3.2.4 / §4.1.5) so
// the diagnostic can faithfully record what production-code feeds to
// interpolateLSP on the very first frame. Kept package-local to the
// test rather than promoted to public API.
var initialPrevLSPMirror = [10]int16{
	31441, 27566, 21458, 13612, 4663,
	-4663, -13612, -21458, -27566, -31441,
}

// lspMonotoneDecreasing reports whether the cosine-domain LSP vector
// is strictly decreasing (well-formed: cos is monotone-decreasing on
// (0, π) and the LSF roots ω_1<...<ω_10 satisfy 0<ω<π).
func lspMonotoneDecreasing(q *[10]int16) bool {
	for i := 1; i < 10; i++ {
		if q[i] >= q[i-1] {
			return false
		}
	}
	return true
}

// l2DeltaQ15 returns ||a − b||₂ for two Q15 LSP vectors as a float64
// in Q15 units (i.e. range roughly 0..√10·65536 ≈ 207k).
func l2DeltaQ15(a, b *[10]int16) float64 {
	var s float64
	for i := 0; i < 10; i++ {
		d := float64(a[i]) - float64(b[i])
		s += d * d
	}
	return math.Sqrt(s)
}

// l2DeltaA returns ||a[1..10] − b[1..10]||₂ for two Q12 LP vectors.
func l2DeltaA(a, b *[11]int16) float64 {
	var s float64
	for i := 1; i <= 10; i++ {
		d := float64(a[i]) - float64(b[i])
		s += d * d
	}
	return math.Sqrt(s)
}

// azStableSchurCohn implements the Schur–Cohn step-down on a monic
// Q12 LP polynomial to test minimum-phase stability. Returns true iff
// every reflection coefficient |k_m| < 1 for m = 10..1, equivalent to
// all roots of A(z) lying strictly inside the unit disk. Adapted from
// the existing in-package check in internal/lsp/stability_test.go
// (TestALGTHMFrame0SF0_AzStability) — clean-room, no external impl.
func azStableSchurCohn(a *[11]int16) bool {
	work := make([]float64, 11)
	for i := 0; i <= 10; i++ {
		work[i] = float64(a[i]) / 4096.0
	}
	if math.Abs(work[0]-1.0) > 1e-9 {
		return false
	}
	for m := 10; m >= 1; m-- {
		k := work[m]
		if math.Abs(k) >= 1.0 {
			return false
		}
		denom := 1.0 - k*k
		next := make([]float64, m)
		next[0] = 1.0
		for i := 1; i < m; i++ {
			next[i] = (work[i] - k*work[m-i]) / denom
		}
		copy(work[:m], next)
	}
	return true
}

func mean64(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

func stddev64(xs []float64, m float64) float64 {
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
