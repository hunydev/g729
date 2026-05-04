package g729

import (
	"errors"
	"io"

	"github.com/exedev/g729/internal/acelp"
	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcbsearch"
	"github.com/exedev/g729/internal/filter"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/gainquant"
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

	// Phase 2d INT-0: fixed-codebook + gain quantization state.
	//
	// prevGpQ14 caches the quantized adaptive-codebook gain ĝp from
	// the previous subframe so CB-4 (BuildCode) can derive the
	// harmonic-enhancement coefficient β per §3.8 eq. 47. At cold
	// start (first subframe of stream) this is 0.
	//
	// prevTaming is the §3.9.2 sticky taming flag (currently a
	// per-subframe diagnostic; the eq. 73/74 quantizer codebooks
	// are non-negative so taming is one-sided).
	//
	// s{1,2} / c{1,2} / ga{1,2} / gb{1,2} hold the per-frame FCB +
	// gain bits for Phase 2f bitstream packing (S = 4 bits; C =
	// 13 bits; GA = 3 bits; GB = 4 bits).
	prevGpQ14  int16
	prevTaming bool
	s1, s2     uint8
	c1, c2     uint16
	ga1, gb1   uint8
	ga2, gb2   uint8

	// Phase 2f PACK-1: per-frame LSP VQ indices retained from
	// lpcStep so buildBitstreamFrame can compose the §4.2.1 + Table 8
	// 80-bit packed frame. Mirrors the p1/p0/p2 + s/c/ga/gb pattern
	// above. Widths per Table 8: L0=1, L1=7, L2=5, L3=5.
	l0, l1, l2, l3 uint16

	// Phase 2f API-2: streaming Write/Flush state.
	//
	// streamBuf is the PCM tail buffer holding 0..FrameSamples-1
	// samples not yet emitted as a frame. streamBufLen counts the
	// valid samples. streamSink is the destination io.Writer (nil
	// for non-streaming Encoder instances; (*Encoder).Write returns
	// ErrNoStreamSink in that case).
	//
	// OQ-FLUSH-PAD pin: Flush zero-pads any partial trailing frame
	// to FrameSamples with 0x0000 (linear-PCM silence) and emits
	// one final 10-byte frame. Plan §1 I-2f-3 + §10 OQ-FLUSH-PAD.
	//
	// streamPacked is the per-frame packed-byte scratch reused
	// across Write/Flush; promoting it to a receiver field keeps
	// the streamSink.Write(streamPacked[:]) interface call from
	// escaping the slice header onto the heap. INT-2 zero-alloc.
	streamBuf    [FrameSamples]int16
	streamPacked [FrameBytes]byte
	streamBufLen int
	streamSink   io.Writer

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
	for i := range e.pastQuaEn {
		e.pastQuaEn[i] = gain.PastErrorsDefault
	}
	return e
}

// Reset returns the Encoder to initial state. Equivalent to using a fresh
// NewEncoder, but reuses the existing memory.
func (e *Encoder) Reset() {
	*e = Encoder{}
	lsp.InitFreqPrev(&e.freqPrev)
	lsp.InitLSPOld(&e.lspOld)
	for i := range e.pastQuaEn {
		e.pastQuaEn[i] = gain.PastErrorsDefault
	}
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
// Phase 2f API-1 wiring (plan §6 Task API-1):
//
//  1. lpcStep(pcm)            — §3.2 LP analysis + LSP VQ → l0..l3,
//     aQ12Latest, aHatSF1/2, oldSpeech slide.
//  2. openloopStep()          — §A.3.3/A.3.4 weighted speech + open-loop
//     pitch → tOp.
//  3. closedloopStep(0)       — §A.3.5–A.3.10 subframe 1 → p1, p0; calls
//     fcbStep(0) internally → s1, c1, ga1, gb1.
//  4. closedloopStep(1)       — subframe 2 → p2; calls fcbStep(1)
//     internally → s2, c2, ga2, gb2.
//  5. buildBitstreamFrame     — composes the 15 per-frame indices into
//     a stack-allocated bitstream.Frame.
//  6. bitstream.Pack          — emits the canonical 10-byte G.729
//     frame per §4.2.1 + Table 8.
//
// I3 / I4: per-frame state is mutated exactly once per call by the
// step methods; no allocation on the hot path (the bitstream.Frame
// value is stack-resident).
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
	if len(pcm) != FrameSamples {
		return ErrShortPCM
	}
	if len(out) < FrameBytes {
		return ErrShortOutput
	}

	if _, err := e.lpcStep(pcm); err != nil {
		return err
	}
	_ = e.openloopStep()
	_, _ = e.closedloopStep(0)
	_, _ = e.closedloopStep(1)

	var bsFrame bitstream.Frame
	e.buildBitstreamFrame(&bsFrame)
	return bitstream.Pack(&bsFrame, out)
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

	// Phase 2f PACK-1: retain the LSP VQ indices for §4.2.1 + Table 8
	// frame packing in buildBitstreamFrame.
	e.l0 = uint16(indices.L0)
	e.l1 = uint16(indices.L1)
	e.l2 = uint16(indices.L2)
	e.l3 = uint16(indices.L3)

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
// r(n) = s(n) + Σ_{i=1..10} â_i · s(n − i),  n = 0,...,39
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
//  1. r(n) = analysis filter Â(z) on s (§A.3.3 eq. A.3).
//  2. x(n) = target signal via 1/Â(z/γ) on r (§A.3.6 eq. cited;
//     closedloop.TargetSignal).
//  3. h(n) = impulse response of 1/Â(z/γ) (§A.3.5;
//     closedloop.ImpulseResponse).
//  4. xb(n) = backward filter of x against h (§A.3.7 eq. A.7;
//     closedloop.BackwardFilter).
//  5. intLag = closedloop.SearchInteger(xb, exc, centre, sub)
//     where centre = tOp for sub=0 and intT1 for sub=1.
//  6. frac = closedloop.RefineFraction(xb, exc, intLag, intLag<85)
//     per §A.3.7 lines 2169–2170 (fractions only when intLag<85).
//  7. v(n) = closedloop.AdaptiveVector(exc, intLag, frac) (§3.7.1
//     eq. 40).
//  8. (gp, y) = closedloop.GpAndY(x, v, h) (§3.7.3 eq. 43, 44).
//  9. P1/P0 (sub=0) or P2 (sub=1) packed via closedloop.EncodeP*
//     per §3.7.2.
//  10. Per-subframe state commit:
//     - swMemErr trail: ew(n) = x(n) − ĝp·y(n) for n = 30..39 per
//     §A.3.10 eq. A.10. Phase 2c-only placeholder: the spec
//     eq. A.10 also subtracts ĝ_c·z(n) from the fixed codebook,
//     which is not yet wired (Phase 2d task). See OQ-EXC-COMMIT
//     in plan §9.
//     - oldExc shift-by-40 + append u(n) = round(Gp·v(n)/2^14)
//     for n = 0..39. Phase 2c-only placeholder: the full
//     excitation per §A.3.9 is u(n) = ĝp·v(n) + ĝ_c·c(n);
//     Phase 2d will add the fixed-codebook contribution.
//     - lpResidualMemQ ← s(30..39) for the next subframe's r(n).
//
// Buffer convention (closedloop.SearchInteger / RefineFraction /
// AdaptiveVector): exc is sized PitchMaxInt + SubframeLen = 183
// samples; u(0) is anchored at exc[len − SubframeLen]; the past
// excitation u(−143..−1) occupies the leading 143 samples, and the
// trailing SubframeLen samples carry the LP-residual extension
// r(0..39) per §A.3.7 line 2161, supporting the short-pitch case
// k < SubframeLen.
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

	// Build the closed-loop scratch buffer: 143 past-excitation
	// samples u(−143..−1) followed by the LP-residual extension
	// r(0..39) for the current subframe per §A.3.7 line 2161.
	// Stack-allocated; total 183 samples = PitchMaxInt + SubframeLen.
	var excSearch [closedloop.PitchMaxInt + closedloop.SubframeLen]int16
	copy(excSearch[:closedloop.PitchMaxInt],
		e.oldExc[len(e.oldExc)-closedloop.PitchMaxInt:])
	copy(excSearch[closedloop.PitchMaxInt:], r[:])
	excSlice := excSearch[:]
	intLag, _ = closedloop.SearchInteger(&xb, excSlice, centre, sub)
	frac = closedloop.RefineFraction(&xb, excSlice, intLag, intLag < 85)

	closedloop.AdaptiveVector(excSlice, intLag, frac, &v)
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

	// §A.3.10 eq. A.9 / A.10 commit and FCB + gain quantization
	// driver per Phase 2d INT-0. fcbStep owns:
	//   - oldExc shift + append u(n) = ĝp·v(n) + ĝc·c(n) (eq. A.9)
	//   - swMemErr ← x(n) − ĝp·y(n) − ĝc·z(n) for n=30..39 (eq. A.10)
	//   - per-frame s/c/ga/gb output bits
	//   - pastQuaEn FIFO advance and prevGpQ14 / prevTaming carry
	e.fcbStep(sub, &x, &y, &h, &v, gp)

	// Quantized-Â analysis-filter memory advance for next subframe.
	copy(e.lpResidualMemQ[:], sFrame[30:40])

	return intLag, frac
}

// fcbStep runs the §3.8 + §3.9 fixed-codebook search + gain
// quantization chain on one subframe, and commits the §A.3.10 eq. A.9
// excitation update (oldExc) and eq. A.10 weighted-error update
// (swMemErr). Per Phase 2d sub-plan §6.1 INT-0:
//
//  1. CB-1 AdjustedTarget  : x'(n) = x(n) − gp·y(n)        (§3.8.1 eq. 50)
//  2. CB-1 CorrelationD    : d(n) = Σ x'(i)·h(i−n)         (§3.8.1 eq. 52)
//  3. CB-3 SignsFromD      : signs[n], |d(n)|              (§3.8.1)
//  4. CB-2 PhiPrime        : φ′(i,j)                       (§3.8.1 eq. 56–57)
//  5. CB-2 SearchDepthFirst: 4-pulse positions             (§A.3.8.1)
//  6. CB-4 BuildCode       : c[40] (Q13) + harmonic enh.   (§3.8 eq. 45–47)
//  7. CB-5 FilterCode      : z[40] (Q12) = c ⊛ h           (§3.9 eq. 64)
//  8. GQ-1 PredictedGcQ12  : g'c (Q12)                     (§3.9.1 eq. 71)
//  9. GQ-2 SearchConjugate : (ga, gb, ĝp Q14, ĝc Q12)      (§3.9.2 eq. 73, 74)
//  10. GQ-3 Tame           : optional ĝp clamp             (§3.9.2 taming)
//  11. ENC-1 PackS / PackC : (S, C) bit fields             (§3.8.2 eq. 61, 62)
//  12. ENC-1 PackGains     : (GA, GB) bit fields           (§3.9.3)
//  13. eq. A.10 commit     : swMemErr[n−30] = sat(x(n) −
//     (ĝp·y(n) >> 14) − (ĝc·z(n) >> 12))
//     for n=30..39 (G729E.txt line 2211)
//  14. eq. A.9  commit     : shift oldExc left by SubframeLen and
//     append u(n) = sat((ĝp·v(n) >> 14)
//     + (ĝc·c(n) >> 12)) for n=0..39 (line 2202)
//  15. GQ-3 UpdatePastQuaEn: FIFO shift; new entry = 20·log10(γ̂_c) Q10
//     (§3.9.1 eq. 72)
//  16. prevGpQ14 ← ĝp ; prevTaming ← taming
//
// Q-format reconciliation (OQ-Q-FORMAT-A10): ĝp is Q14, y is Q0;
// product is Q14, right-shift by 14 lands Q0. ĝc is Q12, z is Q12
// (FilterCode CB-5 contract); product is Q24, right-shift by 24 lands
// Q0 — but z is *stored* as int16 Q12 with the >>13 already applied, so
// effectively the product is int32 Q24 → >>12 only would land Q12 and
// then we need additional >>12 to land Q0. To keep the code aligned
// with the sub-plan §6.1 line 321 spec (>>12), the product (ĝc·z) is
// scaled by >>12 to land in Q12 (matching x), and the subtraction is
// performed in Q0 by treating z's stored Q12 representation as the
// physical sample value (z is the filtered FCB excitation, on the same
// physical scale as y per §3.9 eq. 63). Likewise for c[]: c is Q13,
// ĝc·c is Q25, >>13 → Q12. The Q12 sum is then >>12 to Q0 with sat.
// For oldExc storage (Q0 int16), c is Q13 so ĝc·c >> 13 lands in Q11;
// to land in Q0 the sub-plan §3 line 321/322 specifies >>12 on the
// final ĝc·c product which is the joint shift after squaring conventions
// reconcile (see §A.3.9 narrative).
//
// I3 (relaxed for per-subframe state): commits oldExc, swMemErr,
// pastQuaEn, prevGpQ14, prevTaming, and the per-subframe s*/c*/ga*/gb*
// output fields. I4: zero allocation — all scratch (xPrime, d, signs,
// dAbs, phi, positions, c, z) lives on the stack.
func (e *Encoder) fcbStep(
	sub int,
	x, y, h, v *[closedloop.SubframeLen]int16,
	gpUnq int16,
) {
	const N = closedloop.SubframeLen

	// 1. CB-1: x'(n) = x(n) − gp·y(n).
	var xPrime [N]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	// 2. CB-1: d(n) = Σ x'(i)·h(i−n).
	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, h, &d)

	// 3. CB-3: signs / |d|.
	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	// 4. CB-2: φ′(i,j).  ~6.4 KB on stack.
	var phi [N][N]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	// 5. CB-2: depth-first 4-pulse search.
	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	// 6. CB-4: c[40] with harmonic enhancement.
	var c [N]int16
	// intLag for harmonic enhancement: per §3.8 eq. 46/48, T is the
	// integer pitch lag of the current subframe; reuse the closed-loop
	// winner cached on the encoder.
	var intLag int16
	if sub == 0 {
		intLag = e.intT1
	} else {
		intLag = e.intT2
	}
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	// 7. CB-5: z[40] = c ⊛ h (Q12).
	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	// 8. GQ-1: g'c (Q12, NOT saturated; see PredictedGcQ12 docstring)
	// and the predictor's log2 form for §3.9.2 eq. (74) reconstruction
	// in the native (mant Q14, exp) decoder representation per REF-1.
	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)

	// 9. GQ-2: conjugate-codebook 2D VQ → (ga, gb, ĝp Q14, γ̂_c Q13).
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(x, y, &z, gpcPredQ12)

	// 10. GQ-3: taming (one-sided clamp on ĝp under predicted-overflow).
	gpTamed := gainquant.Tame(gpHatQ14, &e.oldExc)
	taming := gpTamed != gpHatQ14
	gpHatQ14 = gpTamed

	// 11. ENC-1: pack S, C.
	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)

	// 12. ENC-1: pack GA, GB (forward §3.9.3 imap).
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)

	if sub == 0 {
		e.s1 = s
		e.c1 = cPacked
		e.ga1 = gaBits
		e.gb1 = gbBits
	} else {
		e.s2 = s
		e.c2 = cPacked
		e.ga2 = gaBits
		e.gb2 = gbBits
	}

	// 12b. Reconstruct the chosen quantized g_c in the native
	// (mant Q14, exp) representation that mirrors gain.Decoder.Decode
	// bit-for-bit (REF-1 §2 / IMPL-3 step C). The §A.3.10 commits below
	// then derive a non-saturating int32 Q12 value from (mant, exp) for
	// the swMemErr / oldExc accumulator multiplies — eliminating the
	// pre-IMPL-3 int16 collapse in `gcHatQ12` that biased the encoder
	// excitation envelope versus the decoder reconstruction.
	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	gcQ12Wide := mantExpToQ12(gcMantQ14, gcExp)

	// 13. §A.3.10 eq. A.10 commit: swMemErr ← x − ĝp·y − ĝc·z.
	// Q-format: ĝp Q14 × y Q0 = Q14 (>>14 → Q0); ĝc Q12 × z Q12 = Q24
	// (>>24 → Q0). z is the filtered FCB excitation in Q12 per
	// FilterCode (CB-5) — the trailing >>12 reconciles to Q0 sample.
	for n := 30; n < N; n++ {
		gpY := (int32(gpHatQ14) * int32(y[n])) >> 14
		gcZ := int32((int64(gcQ12Wide) * int64(z[n])) >> 12)
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	// 14. §A.3.10 eq. A.9 commit: shift oldExc left by N, append
	// u(n) = ĝp·v(n) + ĝc·c(n). Q-format: ĝp Q14 × v Q0 = Q14
	// (>>14 → Q0); ĝc Q12 × c Q13 = Q25 (>>13 → Q12, then sub-plan
	// §3 line 322 specifies an additional >>1 reconciles c to v
	// scale; combined >>13 is what aligns the sample magnitudes
	// observed downstream by the long-term filter memory).
	copy(e.oldExc[:len(e.oldExc)-N], e.oldExc[N:])
	base := len(e.oldExc) - N
	for n := 0; n < N; n++ {
		gpV := (int32(gpHatQ14) * int32(v[n])) >> 14
		gcC := int32((int64(gcQ12Wide) * int64(c[n])) >> 13)
		e.oldExc[base+n] = fixed.Saturate(gpV + gcC)
	}

	// 15. GQ-3 part B: pastQuaEn FIFO advance with γ̂_c (Q13). The
	// chosen γ̂_c is now returned directly by SearchConjugate (sum of
	// GBK1[ga][1] + GBK2[gb][1]); no inversion of the saturated ĝc is
	// required (replaces the pre-IMPL-3 recoverGammaCQ13 helper).
	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)

	// 16. Carry forward.
	e.prevGpQ14 = gpHatQ14
	e.prevTaming = taming
}

// mantExpToQ12 converts the native (mantissa Q14, exponent int8) g_c
// representation into a NON-saturated int32 Q12 value suitable for the
// §A.3.10 commit accumulators. Mirrors the decoder-side path used by
// synth.BuildExcitation: gc·c(n) at Q(14+exp+13)>>(15) = Q(12+exp);
// here we reduce to the sub-plan §3 chosen Q12 sample envelope.
//
// Math:
//
//	gcQ12 = (mant << 14) >> (14 - exp + 2)   for the canonical case
//	       = mant << exp / 4                (because mant is Q14, target Q12)
//	→ if exp ≥ 2:   mant << (exp - 2)
//	  if exp <  2:  mant >> (2 - exp)
//
// All shifts kept in int64 to absorb the rare exp ≥ 8 codebook entries
// (γ̂_c·g'c can run up to ≈160 ⇒ Q12 ≈ 655 360, fits in 20 bits) without
// the pre-IMPL-3 int16 saturation that biased the §A.3.10 commit.
func mantExpToQ12(mantQ14 int16, exp int8) int32 {
	if mantQ14 == 0 {
		return 0
	}
	shift := int(exp) - 2
	if shift >= 0 {
		v := int64(mantQ14) << uint(shift)
		if v > 0x7FFFFFFF {
			return 0x7FFFFFFF
		}
		if v < -0x80000000 {
			return -0x80000000
		}
		return int32(v)
	}
	return int32(int64(mantQ14) >> uint(-shift))
}
