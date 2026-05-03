package g729

import (
	"errors"

	"github.com/exedev/g729/internal/acelp"
	"github.com/exedev/g729/internal/filter"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/pitch/closedloop"
	"github.com/exedev/g729/internal/pitch/openloop"
)

// Encoder holds G.729 Annex A encoder state for one logical stream.
//
// All buffers are preallocated; EncodeFrame allocates 0 in steady state.
// Concurrent calls on the same Encoder are a data race; callers needing
// parallel encoding must own one Encoder per channel.
type Encoder struct {
	pre pcm.PreProcessor

	// §5.3 preallocated histories.
	oldSpeech  [240]int16
	oldWspeech [143]int16
	oldExc     [154]int16
	synMem     [10]int16
	wMem       [10]int16
	errMem     [10]int16
	lspOld     [10]int16
	lspOldQ    [10]int16
	pastQuaEn  [4]int16
	freqPrev   [4][10]int16

	// FIX-3-B (Phase 2a INT-1 d10): count of frames that triggered the
	// anti-palindromic LP guard in lpcStep (LPToLSP returned
	// ErrLPCNonStable → previous-frame LSP reuse). Diagnostic-only;
	// not gating. Per spec §3.2.6 stability-and-reuse precedent.
	lspReuseCount uint64

	// Phase 2b INT-0: open-loop pitch state.
	//
	// aQ12Latest caches the order-10 LP polynomial (Q12) produced by
	// the most recent lpcStep call so openloopStep can build A(z/γ)
	// and A'(z) per §A.3.3 without re-running LP analysis. Phase 2b
	// stand-in for Â — the quantized-LP reconstruction is OQ-2 work
	// (Phase 2b plan §1 line 42).
	//
	// lpResidualMem and swMem are the §A.3.3 filter memories owned
	// by the encoder per the I10 / Phase-2a state-isolation
	// doctrine: the openloop package's helpers are pure on these
	// pointers and the root encoder coordinates lifetime.
	//
	// tOp records the most recently computed open-loop pitch
	// T_op ∈ [20,143] so Phase 2c (closed-loop refinement) can
	// recover the search centre without re-running §A.3.4.
	aQ12Latest    [lpc.LPCOrder + 1]int16
	lpResidualMem [10]int16
	swMem         [10]int16
	tOp           int16

	// Phase 2c INT-0: closed-loop pitch state.
	//
	// lspDec mirrors the decoder-side LSP reconstruction so the
	// encoder can derive the per-subframe quantized LP polynomials
	// Â (Q12) used by the closed-loop filters per I-2c-2 (plan
	// §0). It is advanced exactly once per frame inside lpcStep
	// after Quantize selects the LSP indices.
	//
	// aHatSF1 / aHatSF2 cache those per-subframe Â vectors so that
	// closedloopStep(0) and closedloopStep(1) read the right
	// polynomial without re-running the LSP→LP chain.
	//
	// swMemErr is the §A.3.10 weighted-error memory ew(n) for
	// the target filter 1/Â(z/γ); committed at the end of every
	// subframe per eq. A.10. Phase 2c-only placeholder: the
	// fixed-codebook contribution ĝ_c·z(n) is missing and will be
	// added by Phase 2d (OQ-EXC-COMMIT, plan §9).
	//
	// lpResidualMemQ is the analysis-filter memory for residual
	// r(n) computed via the QUANTIZED Â (separate from the
	// open-loop lpResidualMem above which still uses the
	// unquantized A as the Phase-2b stand-in per OQ-2).
	//
	// oldExc holds the trailing past excitation u(-1..-LookbackExc)
	// in chronological order, with oldExc[len-1] = u(-1). Updated
	// per subframe after closedloopStep selects (intLag, frac, gp):
	// shift left by SubframeLen and append the new 40 samples
	// u(n) = Gp·v(n). Phase 2c-only placeholder per OQ-EXC-COMMIT;
	// Phase 2d will add Gc·c(n) to u(n).
	//
	// intT1 caches the subframe-1 integer winner so closedloopStep(1)
	// can centre its 10-lag P2 search window per §4.1.3 (lines
	// 1512–1523) via closedloop.Subframe2Window.
	//
	// frac1 / frac2 / intT2 + p1 / p0 / p2 store the per-frame
	// pitch-bit outputs; future bit-packing tasks (Phase 2g) will
	// drain them into the 80-bit frame layout.
	lspDec         lsp.Decoder
	aHatSF1        [lpc.LPCOrder + 1]int16
	aHatSF2        [lpc.LPCOrder + 1]int16
	swMemErr       [10]int16
	lpResidualMemQ [10]int16
	intT1          int16
	intT2          int16
	frac1          int8
	frac2          int8
	p1             uint8
	p0             uint8
	p2             uint8

	// Per-block state owners.
	lpc    lpc.Analyzer
	acelp  acelp.Searcher
	weight filter.Weighting
}

// NewEncoder returns an Encoder in initial state.
func NewEncoder() *Encoder {
	e := &Encoder{}
	lsp.InitFreqPrev(&e.freqPrev)
	lsp.InitLSPOld(&e.lspOld)
	return e
}

// Reset returns the Encoder to initial state. Equivalent to using a fresh
// NewEncoder, but reuses the existing memory.
func (e *Encoder) Reset() {
	*e = Encoder{}
	lsp.InitFreqPrev(&e.freqPrev)
	lsp.InitLSPOld(&e.lspOld)
}

// LSPReuseCount returns the running tally of frames where the
// FIX-3-B anti-palindromic LP guard fired (LPToLSP returned
// ErrLPCNonStable → previous-frame LSP reuse). Diagnostic-only;
// not part of the encoded bitstream. See §3.2.6 / §3.2.3 cite in
// lpcStep.
func (e *Encoder) LSPReuseCount() uint64 { return e.lspReuseCount }

// EncodeFrame consumes exactly FrameSamples samples and writes exactly
// FrameBytes bytes to out. Internal state is retained across calls.
//
// Phase 2-0 stub: validates lengths and returns ErrNotImplemented. Real
// encoding is wired in Phase 2a..2f. Phase 2a wires only the
// LP-analysis + LSP-quantization sub-chain via the package-private
// lpcStep helper; the public EncodeFrame remains a stub until later
// phases land the pitch / FCB / gain / packing pipeline.
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
	if len(pcm) != FrameSamples {
		return ErrShortPCM
	}
	if len(out) < FrameBytes {
		return ErrShortOutput
	}
	return ErrNotImplemented
}

// lpcStep runs the §3.2.1–§3.2.4 chain on one 80-sample PCM frame
// and returns the LSP quantizer indices (L0, L1, L2, L3) for that
// frame. Internal state advanced per call:
//
//	e.oldSpeech : 240-sample sliding analysis buffer (shifted left by
//	              80 each call; new pre-processed samples appended at
//	              [160..239]).
//	e.freqPrev  : MA-predictor FIFO; commitPredictorMemory advances
//	              it once Quantize selects the L0 winner.
//
// Buffer-shift convention (per plan §3.2.1 cite line 671). The
// 30 ms LP analysis window covers 240 = 200 past + 40 future
// samples. With the slide-by-80 layout used here the window applied
// at iteration n centres on oldSpeech[200], so the analysis output
// at iteration n corresponds to the speech segment ending at the
// first 40 samples of frame n — i.e. there is a 1-frame
// analysis-vs-encode delay between PCM-in and indices-out. Phase 2a
// gate vector LSP.BIT was generated by ITU's encoder under this
// convention; Phase 2b will own the same shift ordering for the
// pitch / FCB stages.
//
// I3 / I4: pure (apart from advancing e.oldSpeech / e.freqPrev),
// zero allocation; all per-frame scratch lives in stack arrays.
func (e *Encoder) lpcStep(pcm []int16) (lsp.Indices, error) {
	if len(pcm) != FrameSamples {
		return lsp.Indices{}, ErrShortPCM
	}

	var processed [FrameSamples]int16
	e.pre.Process(pcm, processed[:])

	copy(e.oldSpeech[0:160], e.oldSpeech[80:240])
	copy(e.oldSpeech[160:240], processed[:])

	var aQ12 [lpc.LPCOrder + 1]int16
	if err := e.lpc.Analyze(&e.oldSpeech, &aQ12); err != nil {
		return lsp.Indices{}, err
	}
	e.aQ12Latest = aQ12

	var qQ15 [10]int16
	if err := lsp.LPToLSP(&aQ12, &qQ15); err != nil {
		// FIX-3-B (Phase 2a INT-1 d10): anti-palindromic LP guard.
		//
		// Spec citation: G.729 (06/2012) §3.2.3 establishes the F1/F2
		// sum/difference polynomial construction and the 60-point
		// sign-change scan, but is silent on the degenerate case
		// where a[k] = ±a[10−k] makes one polynomial identically
		// zero (no sign changes ⇒ root extraction fails). §3.2.6
		// (LSP→LP for the synthesis filter) provides the
		// stability-and-reuse precedent: when a freshly reconstructed
		// LSP vector violates the ordering / spacing constraints,
		// the previous frame's quantized LSPs are reused. Applying
		// the same precedent here keeps the encoder graceful on
		// rare anti-palindromic transients (e.g. LSP.IN frame 596)
		// instead of fail-fast at the first non-stable frame.
		//
		// Cold-start safety: NewEncoder / Reset seed e.lspOld via
		// InitLSPOld (cos(i·π/11) Q15 per §3.2.6 / §4.1.5), so the
		// fallback is well-defined even if the very first frame
		// triggers the guard.
		if errors.Is(err, lsp.ErrLPCNonStable) {
			qQ15 = e.lspOld
			e.lspReuseCount++
		} else {
			return lsp.Indices{}, err
		}
	} else {
		// Successful extraction: cache for future-frame reuse.
		e.lspOld = qQ15
	}

	var omega [10]int16
	lsp.LSPToLSF(&qQ15, &omega)

	indices := lsp.Quantize(&omega, &e.freqPrev)

	// Phase 2c INT-0: derive per-subframe quantized LP polynomials Â
	// from the just-emitted indices. The decoder-side state in
	// e.lspDec runs in lock-step with the encoder VQ + MA-predictor
	// so the reconstructed Â matches what the receiver will see.
	// Cached on the encoder for the per-subframe closed-loop driver.
	e.aHatSF1, e.aHatSF2 = e.lspDec.Decode(indices)

	return indices, nil
}

// openloopStep runs the §A.3.3 weighted-speech construction and the
// §A.3.4 open-loop pitch search on the most recently analyzed frame.
// It must be called after lpcStep on the same frame: lpcStep populates
// e.aQ12Latest and writes the pre-processed PCM into e.oldSpeech[160:240],
// both of which openloopStep consumes.
//
// State advanced per call:
//
//	e.lpResidualMem : 10-sample s-history for eq. A.3
//	e.swMem         : 10-sample sw-history for eq. A.2
//	e.oldWspeech    : 143-sample sw history (slid in-place per
//	                  slideOldWspeech / I-2b-2)
//	e.tOp           : the per-frame open-loop pitch ∈ [20,143]
//
// I3 / I4: pure (apart from advancing the four state buffers above);
// zero allocation. The 80-sample current-frame view is taken via a
// pointer-to-array reslice over e.oldSpeech, avoiding a stack copy.
func (e *Encoder) openloopStep() int16 {
	s := (*[FrameSamples]int16)(e.oldSpeech[160:240])
	e.tOp = openloop.Step(&e.aQ12Latest, s, &e.lpResidualMem, &e.swMem, &e.oldWspeech)
	return e.tOp
}

// lpResidualSubframe computes the 40-sample LP residual r(n) for one
// subframe per ITU-T G.729 §A.3.3 eq. A.3 (G729E.txt line 2080):
//
//r(n) = s(n) + Σ_{i=1..10} â_i · s(n − i),  n = 0,...,39
//
// using the supplied 10-sample input history mem (s(-10..-1)). aHat
// is the QUANTIZED Â (Q12, leading 4096), per Phase 2c invariant
// I-2c-2. Mirrors openloop/lowpass.go lpResidual but specialised to
// SubframeLen = 40 and quantized-Â discipline.
//
// I3 / I4: pure (writes only through r), zero allocation. The caller
// is responsible for advancing mem at subframe boundaries.
func lpResidualSubframe(s *[40]int16, aHat *[lpc.LPCOrder + 1]int16, mem *[10]int16, r *[40]int16) {
for n := 0; n < 40; n++ {
sum := int32(s[n])
for i := 1; i <= 10; i++ {
var sni int16
if n-i >= 0 {
sni = s[n-i]
} else {
sni = mem[10+n-i]
}
sum += int32(fixed.Mult(aHat[i], sni))
}
r[n] = fixed.Saturate(sum)
}
}

// closedloopStep runs ITU-T G.729 Annex A §A.3.5–§A.3.10 for one
// subframe and returns the selected (intLag, frac) ∈ [20,143]×{−1,0,+1}.
//
// Pre-condition: lpcStep + openloopStep have been called for the
// current frame, so e.aHatSF{1,2} carry the quantized Â (per I-2c-2)
// and e.tOp carries the open-loop centre. Must be called twice per
// frame in order: closedloopStep(0) then closedloopStep(1). The
// subframe-1 winner is committed to e.intT1 so subframe-2's 10-lag
// P2 search window (closedloop.Subframe2Window) can centre on it
// per §4.1.3 (G729E.txt lines 1512–1523).
//
// Per-subframe pipeline:
//
//   1. r(n) = analysis filter Â(z) on s (§A.3.3 eq. A.3).
//   2. x(n) = target signal via 1/Â(z/γ) on r (§A.3.6 eq. cited;
//      closedloop.TargetSignal).
//   3. h(n) = impulse response of 1/Â(z/γ) (§A.3.5;
//      closedloop.ImpulseResponse).
//   4. xb(n) = backward filter of x against h (§A.3.7 eq. A.7;
//      closedloop.BackwardFilter).
//   5. intLag = closedloop.SearchInteger(xb, exc, centre, sub)
//      where centre = tOp for sub=0 and intT1 for sub=1.
//   6. frac = closedloop.RefineFraction(xb, exc, intLag, intLag<85)
//      per §A.3.7 lines 2169–2170 (fractions only when intLag<85).
//   7. v(n) = closedloop.AdaptiveVector(exc, intLag, frac) (§3.7.1
//      eq. 40).
//   8. (gp, y) = closedloop.GpAndY(x, v, h) (§3.7.3 eq. 43, 44).
//   9. P1/P0 (sub=0) or P2 (sub=1) packed via closedloop.EncodeP*
//      per §3.7.2.
//   10. Per-subframe state commit:
//       - swMemErr trail: ew(n) = x(n) − ĝp·y(n) for n = 30..39 per
//         §A.3.10 eq. A.10. Phase 2c-only placeholder: the spec
//         eq. A.10 also subtracts ĝ_c·z(n) from the fixed codebook,
//         which is not yet wired (Phase 2d task). See OQ-EXC-COMMIT
//         in plan §9.
//       - oldExc shift-by-40 + append u(n) = round(Gp·v(n)/2^14)
//         for n = 0..39. Phase 2c-only placeholder: the full
//         excitation per §A.3.9 is u(n) = ĝp·v(n) + ĝ_c·c(n);
//         Phase 2d will add the fixed-codebook contribution.
//       - lpResidualMemQ ← s(30..39) for the next subframe's r(n).
//
// Buffer convention (closedloop.SearchInteger): exc is sized 143
// samples with exc[len-1] = u(-1). Phase 2c INT-0 SMOKE test pins
// inputs that yield centres ≥ 40 so the residual-extension corner
// (short lags k<40 reading into u(0..39)) is not exercised; INT-1
// will revisit if PITCH.BIT byte-EQ requires it.
//
// I3 (relaxed for Phase 2c): swMemErr / lpResidualMemQ / oldExc are
// updated PER SUBFRAME (not per frame) because subframe-2's adaptive
// codebook search must observe subframe-1's freshly-committed u(n)
// (the pitch lag may reference back into the just-completed
// subframe). The frame-level state (oldSpeech, freqPrev, lspDec
// internals) remains committed only at frame boundaries via lpcStep.
//
// I4: zero allocation. All scratch (r, x, h, xb, v, y, ew) lives on
// the stack as fixed-size arrays.
func (e *Encoder) closedloopStep(sub int) (intLag int16, frac int8) {
var aHat *[lpc.LPCOrder + 1]int16
if sub == 0 {
aHat = &e.aHatSF1
} else {
aHat = &e.aHatSF2
}

sStart := 160 + 40*sub
sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

var r, x, h, xb, v, y [closedloop.SubframeLen]int16
lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
closedloop.TargetSignal(aHat, &r, &e.swMemErr, &x)
closedloop.ImpulseResponse(aHat, &h)
closedloop.BackwardFilter(&x, &h, &xb)

var centre int16
if sub == 0 {
centre = e.tOp
} else {
centre = e.intT1
}

excPast := e.oldExc[len(e.oldExc)-143:]
intLag, _ = closedloop.SearchInteger(&xb, excPast, centre, sub)
frac = closedloop.RefineFraction(&xb, excPast, intLag, intLag < 85)

closedloop.AdaptiveVector(excPast, intLag, frac, &v)
gp := closedloop.GpAndY(&x, &v, &h, &y)

if sub == 0 {
e.intT1 = intLag
e.frac1 = frac
e.p1 = closedloop.EncodeP1(intLag, frac)
e.p0 = closedloop.EncodeP0(e.p1)
} else {
tmin, _ := closedloop.Subframe2Window(e.intT1)
e.intT2 = intLag
e.frac2 = frac
e.p2 = closedloop.EncodeP2(intLag, frac, tmin)
}

// §A.3.10 eq. A.10 weighted-error commit (Phase 2c placeholder:
// fixed-codebook term ĝ_c·z(n) omitted — see OQ-EXC-COMMIT).
for n := 30; n < 40; n++ {
gpY := int32(gp) * int32(y[n]) >> 14
e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY)
}

// Excitation commit: shift past by SubframeLen and append the
// adaptive-codebook contribution Gp·v(n). Phase 2d will replace
// this with u(n) = ĝp·v(n) + ĝ_c·c(n) per §A.3.9.
copy(e.oldExc[:len(e.oldExc)-closedloop.SubframeLen],
e.oldExc[closedloop.SubframeLen:])
base := len(e.oldExc) - closedloop.SubframeLen
for n := 0; n < closedloop.SubframeLen; n++ {
gpV := int32(gp) * int32(v[n]) >> 14
e.oldExc[base+n] = fixed.Saturate(gpV)
}

// Quantized-Â analysis-filter memory advance for next subframe.
copy(e.lpResidualMemQ[:], sFrame[30:40])

return intLag, frac
}
