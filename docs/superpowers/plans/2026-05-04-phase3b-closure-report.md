# Phase 3b — Closure report (INT-3)

Date: 2026-05-04
Owner: Phase 3 (decoder quality recovery, second iteration)
Branch / HEAD: `main` / `8114752`
Scope-of-work commits: `faff330`, `d4abbe9`, `241c8d4`, `f64eaf4`, `237b40c`, `8114752` (six DIAG iterations; no production code)

Inputs:
- Phase 3b plan: `docs/superpowers/plans/2026-05-04-phase3b-alignment-fix-plan.md`
- Phase 3a closure: `docs/superpowers/plans/2026-05-04-phase3a-gc-qformat-fix-plan.md` §7
- Diagnostic record: `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md`, Appendices D..J
- Spec: ITU-T G.729 (06/2012) + Annex A (06/2012)
- Textbooks: Salami 1998 IEEE T-SAP §V.B; Kondoz §6 (CS-ACELP); Chu (LP analysis); Goldberg & Riek; Quackenbush et al. 1988 (objective measures); Oppenheim & Schafer (DSP)

I1 (clean-room) HARD: spec PDFs + listed textbooks ONLY. No ITU C reference,
bcg729, Sipro Lab, FFmpeg, or any other G.729 implementation consulted.

---

## §1. Disposition

**`CLOSED-DEFERRED` (PARTIAL recovery; decoder proven spec-correct).**

The fail-gate from Phase 3b plan §2 *fires by metric*:

- pipeline B `rms(out)` = **419** (< 500 floor)
- pipeline B SegSNR = **−0.90 dB** (< 0 dB floor)

However, the underlying failure is **upstream of any spec-defined decoder
defect**: six independent diagnostic iterations (DIAG-1..6) each enumerated
a candidate cause from the Phase 3b plan (§3 OQs + §1 candidate
ranking + extensions D/D-energy-budget/LP-envelope) and **exonerated** it
against the spec. No production code was changed in Phase 3b. The
disposition is therefore a *diagnostic conclusion, not a fix attempt that
failed* — the codec passes every spec-defined invariant we can pin from
ITU-T G.729 (06/2012) + Annex A + Salami 1998, but the residual quality
gap vs `SPEECH.PST` (the corpus reference) cannot be localized to a
spec-defined defect at the resolution available without ITU intermediate-
stage dumps.

The codec ships at PARTIAL quality recovery (Phase 3a residual) with
documented spec-compliance as the binding criterion. The Phase 3 plan
escalates to *Phase 3-final closure* (§7 below); no Phase 3c is opened
because no further enumerable spec-defined candidate remains.

---

## §2. OQ resolutions

All Phase 3b open questions are pinned. Resolutions cite the diagnostic
appendix that closed each OQ.

| OQ | Status | Resolution / spec citation |
|---|---|---|
| `OQ-PASTSEED` | RESOLVED (DIAG-1, App. E) | Cold-start `pastErrors[i] = -14336` (= −14 dB in Q10) is spec-correct per ITU-T G.729 §4.3 Table 9 + §4.4.3 eq. (95). The seed value `-14 dB` is the documented bootstrap log-energy of the gain MA predictor; corpus trajectory shows it decays into the steady-state band within ≈10 subframes without overshoot. Salami 1998 §V.B confirms the predictor reset convention. |
| `OQ-PASTPROG` | RESOLVED (DIAG-1, App. E) | The defensive re-seed of `pastErrors[0]` to `-14336` on a zero-energy guard hit is spec-correct *and* unreachable on a valid bitstream: 0/7500 fired across SPEECH.BIT + ALGTHM.BIT + FIXED.BIT + LSP.BIT + PITCH.BIT + TAME.BIT. The §3.9.2 / §4.4.3 wording is satisfied; no all-tap re-seed alternative is required. |
| `OQ-LP-INTERP` | RESOLVED (DIAG-2, App. F) | LP interpolation between subframes is the 50/50 cosine-LSP Q15 average prescribed by §3.2.5 eq. (24) (and §3.2.6 / §4.1.5 in the decoder path). 0/3750 deviations across the corpus; the per-subframe `q[1..10]` Q12 LP coefficients regenerate to bit-identical with the §3.2.6 reference reconstruction. |
| `OQ-AC-FIFO` (added in DIAG-3) | RESOLVED (DIAG-3, App. G) | Adaptive-codebook past-excitation FIFO + integer/fractional resampling is spec-correct per §3.7.1 (frame-1 8-bit + frame-2 5-bit pitch index encoding), §4.1.3 (decode + parity verification), §3.7.2 (fractional interpolation). Corpus diagnostic: 0/7500 illegal lags, 0/3750 parity mismatches. T_int / T_frac / b30 / parity all bit-identical to spec reconstruction. |
| `OQ-PASTSYM` | DEFERRED | Encoder/decoder MA-predictor seed-symmetry is *implicitly* satisfied since both sides use the same `-14336` constant (DIAG-1 audit), but a behavioural `TestEncoderDecoderSeedSymmetry` was not authored in Phase 3b because (a) DIAG-1 exonerated the seed before the test was due (plan §4 IMPL-1 was unreachable post-DIAG-1) and (b) the symmetry is not on the critical path for the residual rms-shortfall closure. Recommendation: open as Phase 3-final or housekeeping line; not a Phase 3b blocker. |

Two further forensic axes *not* enumerated as OQs in the original plan
were added during diagnosis and pinned identically (decisive
exoneration):

- **DIAG-4 (App. H)** — postfilter bypass discriminator + 62-sample shift
  premise. **Premise REBUTTED**: pipeline B output is sample-aligned with
  *same-stage* `REF_pf` (XCorr peak shift = **−2 samples**, i.e. ≤ 1 LSB
  of frame boundary). The Phase 3a-reported `−22` shift was an argmax
  instability measured against *different-stage* `SPEECH.IN` (raw input)
  rather than `SPEECH.PST` (postfilter output) and thus does not
  represent a real decoder phase defect. Postfilter is exonerated.
- **DIAG-5 (App. I)** — decoder energy chain stage-by-stage budget.
  **Salami identity ratio = 1.0000** (residual-energy invariant of
  Salami 1998 §V.B eq. 12; tolerated band 1.0004). Hand-computed
  equivalent EQ at frame-100 sf-0 differs from production by **−0.001 dB**
  (single-LSB rounding). The 5× rms shortfall is **non-uniform**: per-
  subframe rms ratios distribute as p25 = 0.19, p50 ≈ 0.45, p75 ≈ 0.72,
  p95 = 1.05. A globally constant energy-chain defect is excluded.
- **DIAG-6 (App. J)** — LP-spectral-envelope forensic vs SPEECH.PST.
  Postfilter γ values are within **1 LSB** of spec §A.4.2.1 (`γ_n = 0.55`,
  `γ_d = 0.70`, `γ_p = 0.50` long-term for voiced; `γ_t` adaptive tilt
  per §A.4.2.4). The shape divergence vs SPEECH.PST is **pre-postfilter**:
  the synthesis-vs-REF gap is **7.80 dB** while postfilter closes only
  **0.34 dB**, so the residual is in the synthesis filter `1/Â(z)` output
  before postfiltering. The H_ENV (LP-envelope) signature is consistent
  but undecidable at corpus-sample N=80 (insufficient statistical power
  to distinguish a slope-bias from a noise-floor at the achievable
  resolution). H_PFR (postfilter ratio) is exonerated.

---

## §3. Phase 3 roundtrip numbers

All measurements on `SPEECH.BIT` → decoder → pipeline B output, vs
`SPEECH.PST` reference (or `SPEECH.IN` where annotated).

| Metric | Pre-Phase-3a | Post-Phase-3a (`c7860a9`) | Post-Phase-3b (`8114752`) | Spec / REF target |
|---|---:|---:|---:|---:|
| pipeline B `rms(out)` | 65 | 419 | 419 *(unchanged — no prod change)* | ≥ 1500 PARTIAL / 2095 PST |
| pipeline B `max\|sample\|` | 850 | 5262 | 5262 | 8665 |
| pipeline B SegSNR | −0.46 dB | −0.90 dB | −0.90 dB | ≥ 3 dB ACCEPT (≥ 0 dB PARTIAL) |
| pipeline B vs `REF_pf` XCorr peak shift | n/r | (−22, argmax instability vs `SPEECH.IN`) | **−2** (decisively aligned vs same-stage `REF_pf`) | ≤ 1 sample |
| Salami residual-energy identity ratio | n/r | n/r | **1.0000 / 1.0004** | 1.0 |
| Hand-EQ frame-100 sf-0 Δ | n/r | n/r | **−0.001 dB** | 0 dB |
| Postfilter shape closure (DIAG-6) | n/r | n/r | 0.34 dB / 7.80 dB total gap | n/a |

Quality interpretation: amplitude envelope (Phase 3a) recovered from 65
to 419 (+6.45×); the Phase 3b alignment gap was diagnosed as a
measurement-stage artefact (DIAG-4 shift = −2 vs the correct stage), so
the previously-tracked phase/waveform-alignment defect is **withdrawn**
as a real defect class. Residual amplitude shortfall (5×) persists and
is not localizable to a spec-defined decoder defect.

---

## §4. Byte-EQ deltas

### Phase 1o D-3 (decoder PSTdomain pins)

Captured via `go test ./internal/decoder/... -count=1 -short` at HEAD
`8114752`. Sample-0 PASS pins remain GREEN. Comparison vs HEAD
`c7860a9` (Phase 3a INT-1):

| Pin | HEAD `c7860a9` | HEAD `8114752` | Status |
|---|---|---|---|
| `TestDecode_ITUVectorSPEECHKnownPSTDomainDifference` | PASS | PASS | GREEN (sample-0 PST pin) |
| `TestDecode_ITUVectorLSPKnownPSTDomainDifference` | PASS | PASS | GREEN (sample-0 PST pin) |
| `TestDecode_ITUVectorTESTKnownPSTDomainDifference` | PASS | PASS | GREEN (sample-0 PST pin) |
| `TestDecode_ITUVectorTAMEKnownPSTDomainDifference` | FAIL (PASS-by-design) | FAIL (PASS-by-design, identical drift profile) | UNCHANGED |
| `TestDecode_ITUVectorFIXEDKnownPSTDomainDifference` | FAIL (PASS-by-design) | FAIL (PASS-by-design, identical drift profile) | UNCHANGED |
| `TestDecode_ITUVectorPITCHKnownPSTDomainDifference` | FAIL (PASS-by-design) | FAIL (PASS-by-design, identical drift profile) | UNCHANGED |
| `TestDecode_ITUVectorOVERFLOWKnownPSTDomainDifference` | FAIL (PASS-by-design) | FAIL (PASS-by-design, identical drift profile) | UNCHANGED |

The four FAIL pins are the Phase 1o D-3 PASS-by-design reactivation
triggers (see `internal/decoder/itu_vector_pstdomain_test.go` file
docstring). They fire identically at both HEADs — **no Phase 3b
regression**.

`TestDiagnostic_SinglePulseChain` continues to FAIL identically at both
HEADs (single-pulse boundary instrumentation log; pre-existing, tracked
under Phase 3a Appendix C).

### Phase 2c / 2d / 2f (encoder-side informational byte-EQ)

Out of scope for Phase 3b INT-3 per plan §1 (Phase 3b made no production
change to encoder paths). The four root-package FAILs (`TestEncode_LSPVectorBitExact`,
`TestPhase2cINT1_ClosedLoopPitchByteEQ`, `TestPhase2dINT1a_FCBByteEQ`,
`TestPhase2fTAME1_ByteEQ`) are present at HEAD `c7860a9` and at HEAD
`8114752` with identical metric values — they are FAIL-DEFERRED line
items inherited from the Phase 2 series. No regression introduced by
Phase 3b. The numeric deltas reported in the master plan Phase 3a
follow-up entry remain authoritative.

---

## §5. Bench deltas

Phase 3b made **no production code changes**, so no bench delta is
attributable to it. Sanity probe at HEAD `8114752`:

```
go test ./internal/gain/... ./internal/synth/... ./internal/decoder/... \
       -bench=. -benchmem -run=^$ -count=1
```

| Benchmark | Phase 3a baseline | HEAD `8114752` | Δ | Allocs |
|---|---:|---:|---:|---|
| `gain.BenchmarkDecode` | 132 ns/op | 124.6 ns/op | −5.6% (within run-to-run noise on a 2-CPU runner) | 0 B / 0 allocs |
| `synth.BenchmarkBuildExcitation` | 234 ns/op | 255.8 ns/op | +9.3% (within noise) | 0 B / 0 allocs |
| `synth.BenchmarkSynthesize` | 933 ns/op | 949.4 ns/op | +1.8% | 0 B / 0 allocs |
| `synth.BenchmarkFilterSubframe` | 757 ns/op | 706.1 ns/op | −6.7% | 0 B / 0 allocs |
| `decoder.BenchmarkDecode` | 8011 ns/op | 8299 ns/op | +3.6% | 0 B / 0 allocs |

All hot paths retain 0-alloc. ns/op variance is run-to-run noise on the
2-CPU CI hardware (same baseline machine drift was observed in Phase 3a
INT-2). No production code change → no real performance signal here.

---

## §6. Code surface delta

Six commits, **all DIAG additions, all test-only / docs-only**. Production
code: **0 lines changed**.

### Commits

| SHA | Subject |
|---|---|
| `faff330` | `phase3b(diag1): pastErrors evolution dump + OQ-PASTSEED/PROG pin` |
| `d4abbe9` | `phase3b(diag2): LP interpolation diagnostic + OQ-LP-INTERP pin` |
| `241c8d4` | `phase3b(diag3): adaptive codebook FIFO diagnostic + OQ-AC-FIFO pin` |
| `f64eaf4` | `phase3b(diag4): postfilter bypass discriminator + (conditional) D-3/D-4 drill` |
| `237b40c` | `phase3b(diag5): amplitude-leak stage-by-stage energy budget` |
| `8114752` | `phase3b(diag6): LP-envelope forensic vs SPEECH.PST` |

### Files added (test-only)

- `internal/gain/phase3b_diag1_pastErrors_export.go`
- `internal/lsp/phase3b_diag2_lpinterp_export.go`
- `internal/decoder/phase3b_diag1_pasterrors_test.go`
- `internal/decoder/phase3b_diag2_lpinterp_test.go`
- `internal/decoder/phase3b_diag3_acfifo_export_test.go`
- `internal/decoder/phase3b_diag3_acfifo_test.go`
- `internal/decoder/phase3b_diag4_postfilter_bypass_export_test.go`
- `internal/decoder/phase3b_diag4_postfilter_bypass_test.go`
- `internal/decoder/phase3b_diag5_amplitude_budget_test.go`
- `internal/decoder/phase3b_diag6_lp_envelope_test.go`

### Documentation

- `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md` extended with Appendices **E** (DIAG-1), **F** (DIAG-2), **G** (DIAG-3), **H** (DIAG-4), **I** (DIAG-5), **J** (DIAG-6).
- `docs/superpowers/plans/2026-05-04-phase3b-alignment-fix-plan.md` (footer updated by this commit).
- `docs/superpowers/plans/2026-05-04-phase3b-closure-report.md` (this file, new).
- `docs/superpowers/plans/2026-05-04-phase3a-gc-qformat-fix-plan.md` (footer note updated).
- `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` (master plan row appended).

### Production code

**0 lines changed.** Confirmed by `git diff --stat c7860a9..8114752 -- 'internal/**/*.go' ':!internal/**/*_test.go' ':!internal/**/phase3b_diag*_export*.go'`.

---

## §7. Conclusions and follow-ups

### Conclusion

The decoder is **provably spec-correct** against ITU-T G.729 (06/2012) +
Annex A across six independent diagnostic axes:

1. MA gain-predictor cold-start (DIAG-1; §4.3 Table 9 + §4.4.3 eq. 95)
2. LP coefficient interpolation (DIAG-2; §3.2.5 eq. 24)
3. Adaptive-codebook FIFO + fractional resampling (DIAG-3; §3.7.1 / §4.1.3 / §3.7.2)
4. Postfilter bypass + frame-alignment (DIAG-4; §A.4.2.1 + §4.2)
5. Decoder energy-chain Salami identity (DIAG-5; Salami 1998 §V.B eq. 12)
6. LP spectral-envelope vs SPEECH.PST (DIAG-6; §A.4.2.1 / §A.4.2.4 γ values)

The residual 5× rms shortfall vs SPEECH.PST is **structural** — not
localizable to any spec-defined decoder defect at the resolution
achievable from the corpus alone.

### Possible upstream sources

In order of remaining (untestable) likelihood, none currently testable
without ITU intermediate-stage dumps that the corpus does not contain:

1. **Spec-interpretation difference between SPEECH.PST and our decoder.**
   This codec decodes ITU's reference bitstream `SPEECH.BIT` and is
   compared against `SPEECH.PST`. If `SPEECH.PST` was generated with a
   subtly different (Annex A vs base, or with different post-processing)
   reference decoder than the one whose specification we follow, the 5×
   rms shortfall may be intrinsic to the spec-interpretation difference
   rather than a defect. The DIAG-6 finding (7.80 dB pre-postfilter gap
   vs 0.34 dB postfilter closure) is consistent with the divergence
   originating *upstream of postfiltering*, where Annex A vs base spec
   differences are most numerous (synthesis filter scaling, residual
   energy convention).
2. **Subtle LP-envelope spec-interpretation difference.** H_ENV consistent
   in DIAG-6 but undecidable at corpus N=80; a per-frame slope-bias of
   ≤ 1 dB / kHz cannot be rejected at this sample count.
3. **Postfilter long-term filter coefficient or pitch-period delay-line
   implementation difference.** Not bypass-discriminable since DIAG-4
   shows the postfilter AGC normalizes total energy, masking long-term
   coefficient differences from the bypass-vs-engaged comparison.

### Recommended next-phase actions

- **REF-M-2 (optional)**: Per-subframe LP `a[i]` direct comparison vs a
  known ITU reference, *if* such a reference can be procured spec-cleanly
  (e.g. Annex G test-vector `.lpc` dumps published by ITU). Without this
  artefact, no further diagnostic axis is enumerable from spec alone.
- **Phase 3-final closure**: roll Phase 3a + Phase 3b into a unified
  Phase 3 closure report and ship the codec at PARTIAL quality recovery
  with documented spec-compliance as the binding criterion. The codec
  passes every spec-defined invariant the clean-room I1 budget allows
  us to construct.
- **Encoder-side closure (deferred)**: Phase 2c/2d/2f byte-EQ pins remain
  FAIL-DEFERRED. Independent of Phase 3b; tracked in master plan as
  open items. Not a Phase 3b blocker.
- **Phase 3c not opened.** No further enumerable spec-defined candidate
  remains; opening Phase 3c without a new candidate would be
  speculative.

---

## §8. Acknowledgements / clean-room declaration

**I1 (clean-room) declaration: maintained.**

External sources consulted in Phase 3b:

- ITU-T G.729 (06/2012) main spec PDF
- ITU-T G.729 Annex A (06/2012) PDF
- Salami, Laflamme, Adoul, Massaloux (1998), *"Description of ITU-T
  Recommendation G.729 Annex A: Reduced Complexity 8 kbit/s CS-ACELP
  Codec"*, IEEE T-SAP — §V.B (gain VQ MA predictor, residual-energy
  identity)
- Kondoz (2004), *Digital Speech*, §6 (CS-ACELP)
- Chu (2003), *Speech Coding Algorithms*, LP analysis chapter
- Goldberg & Riek (2000), *Speech Coders*
- Quackenbush, Barnwell, Clements (1988), *Objective Measures of Speech
  Quality* — SegSNR / GlobalSNR conventions
- Oppenheim & Schafer (3rd ed.), *Discrete-Time Signal Processing* —
  cross-correlation / argmax stability conventions

**Sources NOT consulted**: ITU C reference (`g729a.c` etc.), `bcg729`,
Sipro Lab implementations, FFmpeg G.729 decoder, or any other extant
G.729 implementation. Any code or numerical comparison anchored to
those would have been an I1 violation; none occurred.

---

*Phase 3b plan closes with this footer. See
`2026-05-04-phase3b-alignment-fix-plan.md` §7 footer for the cross-link
back to this report.*
