# Phase 2a-INT-1 Diagnostic Cycle Open Report

**Date:** 2026-05-03
**Plan reference:** `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md`
  §Task 2a-INT-1, §0.4 (강압-적합-금지), Error E9 (I5/I6 hard-N close).
**Status:** Production FROZEN per I6. Gate test
`TestEncode_LSPVectorBitExact` is RED. Diagnostic sub-cycle opened.

## TL;DR

The 5-step TDD cycle for Task 2a-INT-1 reached Step 4 (run-and-pass)
and stayed RED after four production-fix attempts (I5 budget). Per the
plan's E9 default close, production is frozen and a measurement-only
diagnostic test is staged at
`internal/lsp/phase2a_int1_frame0_boundary_trace_diagnostic_test.go`
to seed the next investigation cycle. This report documents what was
attempted, what the gate measurements show, and what the next cycle
should investigate.

INT-1a (`lspToLSF`) committed cleanly at `ee2df4d` and is unaffected.

## Pre-flight

* Tree state at HEAD `0209381` (post INT-1a `ee2df4d`): clean.
* `go vet ./...` / `go build ./...`: clean.
* 3 inherited FAILs confirmed and unchanged throughout this cycle:
  * `internal/decoder.TestDiagnostic_SinglePulseChain`
  * `internal/gain.TestDecode_LowEnergyCodebookIsSmooth`
  * `internal/gain.TestDecode_SucceedsAcrossAllGainIndices`

## INT-1a Outcome (auxiliary)

* SHA `ee2df4d` — `feat(lsp): Phase 2a-INT-1a lspToLSF arccos inverse
  helper`.
* All 4 inverse-map tests in `internal/lsp/lsp_lsf_test.go` PASS.
* Used by `Encoder.lpcStep` to feed `Quantize`'s ω input.

## INT-1 Outcome (primary)

### Steps 1–3 (RED, then implement)

* `lsp_itu_vector_test.go` (root) loads `LSP.IN` (357120 B = 2232 ×
  160 B PCM frames) + `LSP.BIT` (366048 B = 2232 × 164 B G.192 frames),
  runs `enc.lpcStep(pcm)` per frame, and asserts byte-EQ on
  `(L0, L1, L2, L3)` vs the G.192-extracted oracle from `LSP.BIT[n]`.
* Step 2 RED confirmed (`enc.lpcStep undefined`).
* Step 3 implementation:
  * `internal/lpc/types.go`: `Analyzer.Analyze` rewritten from stub to
    full chain (`windowSpeech → autocorrelate → applyLagWindow →
    levinsonDurbin`) on `*[240]int16 → *[11]int16`. Stub-error path
    removed.
  * `internal/lpc/types_test.go`: stub-error test replaced with
    `TestAnalyzer_Analyze_AllZeroSpeechProducesTrivialFilter`
    (a[0]=4096, a[1..10]=0).
  * `encoder.go`: `lpcStep` added — pre-process → 80-sample left-shift
    of `oldSpeech` → append at `[160:240]` → `Analyze` → `LPToLSP` →
    `LSPToLSF` → `Quantize`.

### Step 4 (RED, divergence)

After Fix #4 (see below), the gate's per-stage match counts are:

| Stage | Matches |  %    |
|-------|--------:|------:|
| L0    | 1773 / 2232 | 79% |
| L1    |  852 / 2232 | 38% |
| L2    |  349 / 2232 | 16% |
| L3    |  421 / 2232 | 19% |

First divergence is **frame 0**: `got=(L0=0, L1=120, L2=2, L3=11)`
vs `want=(L0=0, L1=120, L2=10, L3=10)`. L0 and L1 already match at
frame 0 — the divergence is **L2/L3**.

### Hypothesis-budget (I5) trail

| # | Fix | Spec citation | Effect on frame-0 / match counts | Kept? |
|---|-----|---------------|-----------------------------------|-------|
| 1 | `InitFreqPrev`: cold-start MA-predictor FIFO with `l̂_i = i·π/11` Q13 | §3.2.4 ("the same memory the decoder uses on cold start") | Frame 0 L0: 1→0 ✓ ; L1: 80→120 ✓ . Counts moved from random toward current. | **YES** |
| 2 | `Quantize` final cost: rearrange residual J1+J2 BEFORE predictor (was: rearrange ω̂ J1 only AFTER predictor) | §3.2.4 lines 819–833 + decoder.go `Decode` step 3 | Counts barely changed; encoder/decoder convention now aligned. | **YES** |
| 3 | (Reverted) `lpcStep` skip pre-processor — hypothesis that `LSP.IN` is post-pre-processor input | None — speculative | Counts moved L1 853→817 (worse). Reverted. | **NO** |
| 4 | `Quantize` final cost: `enforceLSFStability` AFTER predictor | §3.2.4 + decoder.go `Decode` step 4 | Marginal improvement (L0 1767→1773). | **YES** |

Three production fixes kept (1, 2, 4); one reverted (3). I5 budget:
**4 of 5 attempts consumed.** Per E9, the next attempt has been
withheld in favour of opening a measurement cycle, because the
remaining gap is no longer a single-spec-line correction.

## Frame-0 boundary trace (S1–S4)

From `TestDiagnostic_Phase2aInt1_Frame0BoundaryTrace`:

```
S1: initialPastResidual (Q13, i·π/11) = [2340 4679 7019 9359 11698
                                         14038 16377 18717 21057 23396]
S2: LSPCodebookL1[120]                 = [2731 4670 7063 9201 11346
                                          13735 16875 18797 20787 22360]
S3: want L2[10]   = [-77   344 -620  763  413]
    got  L2[2]    = [-1021 231 -306  321 -220]
S4: want L3[10]   = [ 502 -362 -960 -483 1386]
    got  L3[11]   = [ 450 -466 -108 1010 2223]
```

Combined want residual l̂ at frame 0:
`[2654 5014 6443 9964 11759 14237 16513 17837 20304 23746]` (Q13).

## Open hypothesis-family table for next cycle

The integration-gate evidence (L1=120 matches at frame 0; only 38%
across all frames) is consistent with three non-exclusive
families:

* **H-A: ω accuracy.** LP analysis at frame 0 yields ω close enough
  that L1=120 wins, but L2/L3 LSB-precision is off because either
  (i) the analysis-window placement (oldSpeech layout) differs from
  what generated `LSP.BIT`, or (ii) `LPToLSP` Chebyshev-root
  refinement loses a few LSBs on the upper coefficients (5..9). The
  L3 stage operates on i=5..9 and is most sensitive to this.
* **H-B: L2 partial-cost convention.** Spec line 890 says "the
  partial vector ω̂_i is reconstructed using equation (20), and
  rearranged to guarantee a minimum distance of 0.0012." The current
  `searchL2` rearranges only the first 5 components after the
  predictor. Open: should the L2 partial cost include the predictor
  contribution from the upper-half pastResidual memory (which
  mathematically contributes nothing to ω̂[0..4] — diagonal in i —
  but might matter under a non-diagonal interpretation we have
  missed)? Or should L2 search use UN-weighted MSE on residual
  (line 887 reads of L1 as "unweighted") propagated to L2?
* **H-C: weighted-MSE Q-format / weight scaling.** `weightsLSF` is
  Q11; per-term `w · d²` accumulates as `Q11 · Q26 = Q37` in int64.
  Cross-stage cost comparison (L0=0 vs L0=1 in `Quantize`) uses the
  same units, so an absolute-scale bias is harmless — but a
  per-coefficient weight bias (e.g. wrong `w5/w6 · 1.2` boost or
  wrong edge cases in eq. 22) would skew which L2/L3 row wins.

The next cycle should **dump frame-0 ω from production**
(`Encoder.lpcStep` boundary) and compute the partial weighted MSE
for `(L1=120, L2=10)` vs `(L1=120, L2=2)` by hand from the spec,
then attribute the bug to whichever side disagrees.

## Files touched / created (this cycle)

Production (FROZEN — do not modify in next cycle):

* `internal/lpc/types.go` — `Analyzer.Analyze` real implementation.
* `internal/lpc/types_test.go` — replaced stub-error test.
* `internal/lsp/encoder_init.go` (new) — `InitFreqPrev` exported.
* `internal/lsp/encoder_vq.go` — `Quantize` final-reconstruction
  pipeline aligned to decoder (residual rearrange J1+J2 → predictor
  → `enforceLSFStability` → cost).
* `internal/lsp/encoder_vq_l0_test.go` — synthetic oracle re-aligned
  to corrected production convention.
* `encoder.go` — `lpcStep` method; `NewEncoder`/`Reset` now seed
  `freqPrev` via `InitFreqPrev`.

Test infrastructure:

* `lsp_itu_vector_test.go` (root, new) — Phase 2a integration gate
  (RED, byte-EQ vs LSP.BIT for 2232 frames).
* `internal/lsp/phase2a_int1_frame0_boundary_trace_diagnostic_test.go`
  (new) — measurement-only S1–S4 trace + structural sanity.

Documentation:

* This file.

## Plan checkbox status

* `[x]` 2a-INT-1a (committed `ee2df4d`).
* `[ ]` 2a-INT-1 — **deferred** to next diagnostic cycle. Production
  frozen at the partial-fix state; integration gate remains RED.

## Hand-off

Next session should open a `2a-INT-1-d1` (diagnostic 1) plan section
following the Phase 1o D-3 pattern, run the boundary-trace test with
production-side ω injected, and decide between H-A, H-B, H-C with a
closed-form computation. Do NOT make further production edits to
`encoder_vq.go`, `encoder_predictor.go`, or `encoder.go`'s `lpcStep`
without first surfacing the hypothesis to the user (per §0.4).
