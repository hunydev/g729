package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/tables"
)

// TestPhase3bDiag3_AdaptiveCodebookTrajectory drives SPEECH.BIT
// through the production decoder (via the existing DecodeWithTaps
// mirror) and captures, per subframe across the entire ITU Annex A
// SPEECH.BIT corpus, the adaptive-codebook reconstruction state
// prescribed by ITU-T G.729 (06/2012) §3.7.1 / §4.1.3:
//
//   - Pitch-lag bitstream unpacking: T_int + T_frac from P1, P0, P2
//     (§3.7.2 / §4.1.3 — eq. (41) sf-1, eq. (42) sf-2).
//   - Past-excitation FIFO read: v(n) per eq. (40), 1/3-sample b30
//     fractional FIR; integer-delay direct-copy fast path.
//   - Per-subframe FIFO advance: copy(d.pastExc[:113], d.pastExc[40:])
//     followed by copy(d.pastExc[113:], u[:]) (subframe.go:51-52).
//   - Parity bit P0 reconstruction over the six MSBs of P1 (§4.1.3).
//
// The diagnostic is informational (t.Logf only); it always passes.
// Designed to feed Appendix G of
// docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md
// (OQ-AC-FIFO pin + candidate-D-2 verdict).
//
// Spec anchors used for the prescription pin:
//
//	§3.7.1 eq. (40)  v(n) = Σ_i u(n−k−i)·b30(t+3i)
//	                       + Σ_i u(n−k+1+i)·b30(3−t+3i),  i = 0..9
//	§3.7.2           b30 = Hamming-windowed sinc, fc=3600 Hz,
//	                 oversampled by 3, |k| ≤ 29 + zero pad at ±30.
//	§4.1.3 eq. (41)  Sub-frame 1 lag from P1 (8-bit composite).
//	§4.1.3 eq. (42)  Sub-frame 2 lag from P2 (5-bit, relative to T1).
//	§4.1.3 / §3.7.2  P0 = parity over the six MSBs of P1.
func TestPhase3bDiag3_AdaptiveCodebookTrajectory(t *testing.T) {
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
		// raw bitstream
		p1, p0, p2 uint8
		// decoded lag
		tInt  int
		tFrac int
		// gains and FIFO
		gpQ14 int16
		v     [40]int16
		u     [40]int16
		// parity-recomputed bit
		parityRecomputed uint8
	}
	records := make([]subRecord, 0, totalSubframes)

	var d Decoder
	bReader := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte

	for fi := 0; fi < frames; fi++ {
		if _, rerr := bitstream.ReadG192Frame(bReader, packed[:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", fi, rerr)
		}
		taps, derr := d.DecodeWithTaps(packed[:])
		if derr != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", fi, derr)
		}
		f := taps.Frame
		// sf-1 record
		r1 := subRecord{
			p1:               uint8(f.P1),
			p0:               uint8(f.P0),
			p2:               0,
			tInt:             taps.Sub[0].TInt,
			tFrac:            taps.Sub[0].TFrac,
			gpQ14:            taps.Sub[0].GpQ14,
			v:                taps.Sub[0].V,
			u:                taps.Sub[0].U,
			parityRecomputed: pitch.Parity(uint8(f.P1)),
		}
		records = append(records, r1)
		// sf-2 record
		r2 := subRecord{
			p1:               uint8(f.P1),
			p0:               uint8(f.P0),
			p2:               uint8(f.P2),
			tInt:             taps.Sub[1].TInt,
			tFrac:            taps.Sub[1].TFrac,
			gpQ14:            taps.Sub[1].GpQ14,
			v:                taps.Sub[1].V,
			u:                taps.Sub[1].U,
			parityRecomputed: pitch.Parity(uint8(f.P1)),
		}
		records = append(records, r2)
	}

	t.Logf("Phase 3b DIAG-3 — adaptive-codebook FIFO trajectory")
	t.Logf("Corpus: SPEECH.BIT, %d frames (%d subframes)", frames, totalSubframes)
	t.Logf("Spec ref: G.729 (06/2012) §3.7.1 eq. (40); §4.1.3 eq. (41)/(42); §3.7.2 b30 def.")
	t.Logf("Code under test: internal/pitch/adaptive.go::AdaptiveCodebook (decoder use site:")
	t.Logf("  internal/decoder/subframe.go:31; FIFO advance lines 51-52);")
	t.Logf("  internal/pitch/delay.go::DecodeDelaySubframe{1,2}; internal/pitch/parity.go::Parity.")
	t.Logf("")

	// ── §G.5 First-5-frame (10-subframe) trace ─────────────────────
	t.Logf("First-5-frame trace (T_int, T_frac, ‖v‖₂, gp Q14, gp·‖v‖₂):")
	maxTrace := 10
	if maxTrace > len(records) {
		maxTrace = len(records)
	}
	for i := 0; i < maxTrace; i++ {
		r := records[i]
		nv := l2NormI16(r.v[:])
		gpFloat := float64(r.gpQ14) / 16384.0
		t.Logf("  sf %2d (frame %d/%d) T_int=%3d T_frac=%+d ‖v‖₂=%9.2f gp=%.4f gp·‖v‖=%9.2f",
			i, i/2, i%2, r.tInt, r.tFrac, nv, gpFloat, gpFloat*nv)
	}
	t.Logf("First subframe v[0..39] (raw Q0):")
	for i := 0; i < 5 && i < len(records); i++ {
		t.Logf("  sf %d v = %v", i, formatInt16Compact(records[i].v[:]))
	}
	t.Logf("")

	// ── §G.6 T_int / T_frac distribution + parity sanity ──────────
	tIntHist := make(map[int]int)
	tFracHist := make(map[int]int)
	tIntMin, tIntMax := 1 << 30, -1 << 30
	tIntOutOfRange := 0
	parityMismatches := 0
	for _, r := range records {
		tIntHist[r.tInt]++
		tFracHist[r.tFrac]++
		if r.tInt < tIntMin {
			tIntMin = r.tInt
		}
		if r.tInt > tIntMax {
			tIntMax = r.tInt
		}
		// §3.7.1 legal range: T ∈ [19+1/3, 143] ⇒ T_int ∈ [19, 143]
		// (with T_frac ∈ {-1, 0, 1}); §4.1.3 sf-2 may reach 144 with
		// frac=-1 (DecodeDelaySubframe2 doc).
		if r.tInt < 19 || r.tInt > 144 {
			tIntOutOfRange++
		}
	}
	// Parity: only sf-1 records carry a transmitted P0; recompute once
	// per frame and compare against P0.
	for fi := 0; fi < frames; fi++ {
		r := records[fi*2]
		if (r.p0 & 1) != r.parityRecomputed {
			parityMismatches++
		}
	}

	t.Logf("T_int distribution (legal range [19, 144]):")
	t.Logf("  observed range: [%d, %d]   out-of-range: %d / %d (%.4f%%)",
		tIntMin, tIntMax, tIntOutOfRange, len(records),
		100.0*float64(tIntOutOfRange)/float64(len(records)))
	// Histogram top-10
	type kv struct {
		k, v int
	}
	tIntPairs := make([]kv, 0, len(tIntHist))
	for k, v := range tIntHist {
		tIntPairs = append(tIntPairs, kv{k, v})
	}
	sort.Slice(tIntPairs, func(i, j int) bool { return tIntPairs[i].v > tIntPairs[j].v })
	t.Logf("  top-10 T_int bins (lag -> count):")
	for i := 0; i < 10 && i < len(tIntPairs); i++ {
		t.Logf("    T_int=%3d  count=%5d  (%.2f%%)",
			tIntPairs[i].k, tIntPairs[i].v,
			100.0*float64(tIntPairs[i].v)/float64(len(records)))
	}
	t.Logf("T_frac distribution (legal {-1, 0, +1}):")
	for _, k := range []int{-1, 0, 1} {
		c := tFracHist[k]
		t.Logf("  T_frac=%+d  count=%5d  (%.2f%%)", k, c,
			100.0*float64(c)/float64(len(records)))
	}
	// Any T_frac outside {-1, 0, +1}?
	otherFrac := 0
	for k, c := range tFracHist {
		if k != -1 && k != 0 && k != 1 {
			otherFrac += c
			t.Logf("  T_frac=%+d  count=%5d (UNEXPECTED)", k, c)
		}
	}
	t.Logf("  T_frac out-of-spec: %d / %d", otherFrac, len(records))
	t.Logf("Parity sanity (recomputed P0 vs transmitted P0 over six MSBs of P1):")
	t.Logf("  mismatches: %d / %d frames (%.4f%%)",
		parityMismatches, frames, 100.0*float64(parityMismatches)/float64(frames))
	t.Logf("")

	// ── §G.7 Subframe-boundary continuity ─────────────────────────
	// Inspect |u_prev[39] − u_curr[0]| distribution across all
	// subframe transitions (n = totalSubframes − 1 transitions). A
	// FIFO-advance off-by-one would overwrite u_curr[0] with the
	// previous-subframe trailing sample, producing a degenerate
	// distribution (most deltas = 0 or repeated).
	deltas := make([]int, 0, totalSubframes-1)
	zeroDeltaCount := 0
	for i := 1; i < len(records); i++ {
		d := int(records[i-1].u[39]) - int(records[i].u[0])
		if d < 0 {
			d = -d
		}
		deltas = append(deltas, d)
		if records[i-1].u[39] == records[i].u[0] {
			zeroDeltaCount++
		}
	}
	if len(deltas) > 0 {
		sumD, maxD := 0, 0
		for _, d := range deltas {
			sumD += d
			if d > maxD {
				maxD = d
			}
		}
		mean := float64(sumD) / float64(len(deltas))
		// median
		sortable := append([]int(nil), deltas...)
		sort.Ints(sortable)
		median := sortable[len(sortable)/2]
		t.Logf("Subframe-boundary continuity |u_prev[39] − u_curr[0]|:")
		t.Logf("  n=%d  mean=%.2f  median=%d  max=%d", len(deltas), mean, median, maxD)
		t.Logf("  exact-zero-delta transitions: %d / %d (%.4f%%)",
			zeroDeltaCount, len(deltas),
			100.0*float64(zeroDeltaCount)/float64(len(deltas)))
		t.Logf("  (a degenerate FIFO advance would push exact-zero close to 100%%)")
	}
	t.Logf("")

	// ── §G.7 cont. — past-exc snapshot at end of frame 0 ─────────
	// Re-run the first frame to log the final pastExc state and verify
	// the trailing 80 samples equal u_sf1 ⊕ u_sf2 of frame 0 (FIFO
	// commit-after-read invariant).
	{
		var d2 Decoder
		bReader2 := bytes.NewReader(bitData)
		var packed2 [bitstream.FrameBytes]byte
		_, _ = bitstream.ReadG192Frame(bReader2, packed2[:])
		taps0, _ := d2.DecodeWithTaps(packed2[:])
		snap := d2.PastExcSnapshot()
		L := PastExcLenForDiag()
		// Compare trailing 80 vs taps0.Sub[0].U then Sub[1].U
		mismatchSf1, mismatchSf2 := 0, 0
		for i := 0; i < 40; i++ {
			if snap[L-80+i] != taps0.Sub[0].U[i] {
				mismatchSf1++
			}
			if snap[L-40+i] != taps0.Sub[1].U[i] {
				mismatchSf2++
			}
		}
		t.Logf("FIFO commit invariant (post-frame-0 pastExc trailing 80 == U_sf1 ⊕ U_sf2):")
		t.Logf("  trailing-80 sf-1 mismatches: %d / 40", mismatchSf1)
		t.Logf("  trailing-80 sf-2 mismatches: %d / 40", mismatchSf2)
		t.Logf("  (any non-zero = read/write order or stride bug in subframe.go:51-52)")
	}
	t.Logf("")

	// ── §G.7 cont. — phase-skew probe ─────────────────────────────
	// Per subframe, scan a small lag window over u_curr cross-correlated
	// against the FIFO read region pastExc[L−tInt-20 : L−tInt+40+20]
	// reconstructed from u_prev (proxy: just measure peak lag of
	// (gp·v) vs u over the subframe). For a clean AC, peak lag = 0
	// (v IS the AC component of u within rounding/Q-scale).
	type lagStat struct {
		lag int
		cnt int
	}
	lagHist := make(map[int]int)
	for _, r := range records {
		lag := bestXcorrLag(r.v[:], r.u[:], 5)
		lagHist[lag]++
	}
	pairs := make([]lagStat, 0, len(lagHist))
	for k, v := range lagHist {
		pairs = append(pairs, lagStat{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].cnt > pairs[j].cnt })
	t.Logf("Per-subframe XCorr peak lag (v[] vs u[], window ±5):")
	for i := 0; i < len(pairs) && i < 11; i++ {
		t.Logf("  lag=%+d  count=%5d  (%.2f%%)", pairs[i].lag, pairs[i].cnt,
			100.0*float64(pairs[i].cnt)/float64(len(records)))
	}
	t.Logf("  (bias toward non-zero lag = AC reconstruction phase-shifted vs total exc;")
	t.Logf("   biased toward 0 = AC and total exc are time-aligned ⇒ no per-subframe skew)")
	t.Logf("")

	// ── §G.6 cont. — FIR table sanity ─────────────────────────────
	t.Logf("PitchInterpFIR (b30) table dump (Q15, indices 0..30):")
	for i := 0; i < 31; i++ {
		t.Logf("  PitchInterpFIR[%2d] = %6d   (= %+8.5f)", i,
			tables.PitchInterpFIR[i],
			float64(tables.PitchInterpFIR[i])/32768.0)
	}
	// Hand-recomputed Hamming-windowed sinc reference, fc=3600 Hz,
	// fs=8000 Hz, oversample by 3, window width 60 samples.
	//   h_c(t) = 2·fc/fs · sinc(2·fc·t/fs);   t = k/3 sample.
	//   ⇒ b30(k) = 0.9 · sinc(0.3·k) · w_hamming(k, N=60),  k = 0..30
	//   w_hamming(k, N=60) = 0.54 + 0.46·cos(π·k/30).
	t.Logf("Hand-recomputed reference (0.9·sinc(0.3k)·hamming(k, N=60), Q15 round):")
	maxAbsErr := 0
	sumTable := 0
	for i := 0; i < 31; i++ {
		ref := referenceB30(i)
		err := int(tables.PitchInterpFIR[i]) - ref
		if err < 0 {
			err = -err
		}
		if err > maxAbsErr {
			maxAbsErr = err
		}
		sumTable += int(tables.PitchInterpFIR[i])
		t.Logf("  k=%2d  table=%+6d  ref=%+6d  delta=%+d", i,
			tables.PitchInterpFIR[i], ref, int(tables.PitchInterpFIR[i])-ref)
	}
	t.Logf("FIR table vs hand-recomputed reference:")
	t.Logf("  max |delta| over all 31 taps: %d Q15 LSBs", maxAbsErr)
	t.Logf("  table sum (one-sided, includes b30(0)) = %d Q15  (= %+.5f)",
		sumTable, float64(sumTable)/32768.0)
	// DC-gain of the full b30 (mirrored): b30(0) + 2·Σ_{k=1..30} b30(k).
	dcSum := int(tables.PitchInterpFIR[0])
	for k := 1; k < 31; k++ {
		dcSum += 2 * int(tables.PitchInterpFIR[k])
	}
	// At full reconstruction (oversampled domain) DC ≈ 1.0 · 32768 ·
	// (oversample_factor=3) for an oversampling-3 FIR with unit DC
	// in original domain; the polyphase decomposition splits this
	// across the 3 phases, each contributing ~32768.
	t.Logf("  full mirrored DC sum (b30(0) + 2·Σ_{k=1..30}) = %d Q15", dcSum)
	t.Logf("  per-phase DC (each polyphase phase, ~32768 expected):")
	for phase := 0; phase < 3; phase++ {
		s := 0
		for i := phase; i < 31; i += 3 {
			coef := int(tables.PitchInterpFIR[i])
			if i == 0 {
				s += coef
			} else {
				s += 2 * coef
			}
		}
		t.Logf("    phase %d (taps k = %d, %d, %d, …): sum = %d Q15  (= %+.5f)",
			phase, phase, phase+3, phase+6, s, float64(s)/32768.0)
	}
	t.Logf("")

	// ── §G.8 — observed-vs-prescribed pin ─────────────────────────
	t.Logf("OQ-AC-FIFO observed semantics:")
	t.Logf("  • bitstream unpack: P1 (8b) → DecodeDelaySubframe1 → (T_int1, T_frac1)")
	t.Logf("    P0 (1b) checked but result not gated on (decode.go:65)")
	t.Logf("    P2 (5b) → DecodeDelaySubframe2(P2, T_int1) → (T_int2, T_frac2)")
	t.Logf("  • AC build:   pitch.AdaptiveCodebook(T_int, T_frac, d.pastExc[:153], &v)")
	t.Logf("    integer fast-path tFrac=0: v[n] = pastExc[L−T_int+n]   (b30(0)≈0.9 NOT applied)")
	t.Logf("    fractional path  tFrac=±1: §3.7.1 eq. (40) Σ over 20 b30 taps")
	t.Logf("    short-pitch  tInt<40   : extend by periodicity v[n]=v[n−tInt]")
	t.Logf("  • FIFO commit (subframe.go:51-52, AFTER read+synthesis+postfilter+HP):")
	t.Logf("    copy(d.pastExc[:113], d.pastExc[40:113+40])  ← shift left by 40")
	t.Logf("    copy(d.pastExc[113:153], u[:40])             ← append current u")
	t.Logf("    ⇒ read-before-write order at every subframe boundary")
	t.Logf("  • encoder uses pitch/closedloop.Interpolate3 (frac.go:53) with the SAME")
	t.Logf("    posPhase/negPhase mapping and the SAME b30 table → encoder/decoder symmetric")
}

// l2NormI16 returns ‖x‖₂ in float64 over int16 samples.
func l2NormI16(x []int16) float64 {
	var s float64
	for _, v := range x {
		f := float64(v)
		s += f * f
	}
	return math.Sqrt(s)
}

// bestXcorrLag returns argmax_τ Σ_n a[n] · b[n+τ] over τ ∈ [−w, +w].
// Lengths of a and b must match. Out-of-range b indices contribute 0.
func bestXcorrLag(a, b []int16, w int) int {
	bestLag := 0
	bestCorr := -math.MaxFloat64
	N := len(a)
	for lag := -w; lag <= w; lag++ {
		var c float64
		for n := 0; n < N; n++ {
			j := n + lag
			if j < 0 || j >= N {
				continue
			}
			c += float64(a[n]) * float64(b[j])
		}
		if c > bestCorr {
			bestCorr = c
			bestLag = lag
		}
	}
	return bestLag
}

// referenceB30 returns the spec-derived b30(k) value in Q15 (rounded
// half-away-from-zero) using the construction documented in §3.7.2:
// b30(k) = 0.9 · sinc(0.3·k) · hamming(k, N=60),  k ∈ [0, 30].
// Independently re-derived from the first-principles definition of
// a Hamming-windowed sinc lowpass (Oppenheim & Schafer §7.2; not from
// any G.729 implementation source). Used only for table sanity in
// this diagnostic.
func referenceB30(k int) int {
	x := 0.3 * float64(k)
	var sinc float64
	if k == 0 {
		sinc = 1.0
	} else {
		sinc = math.Sin(math.Pi*x) / (math.Pi * x)
	}
	w := 0.54 + 0.46*math.Cos(math.Pi*float64(k)/30.0)
	v := 0.9 * sinc * w * 32768.0
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// formatInt16Compact returns a compact bracketed list of int16 values,
// suitable for a single t.Logf line (no fancy trimming).
func formatInt16Compact(x []int16) []int16 {
	out := make([]int16, len(x))
	copy(out, x)
	return out
}
