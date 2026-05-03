# Phase 2a-INT-1-d4 — Levinson saturation + Chebyshev band-centre pinpoint

**Date:** 2026-05-04 (d4 dispatch)
**Parent plan:** `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md`
  §Task 2a-INT-1, §0.4, Error E9 (I5/I6 hard-N close).
**Predecessor plans:**
* `docs/superpowers/plans/2026-05-03-phase2a-int1-d1-diagnostic-plan.md`
  (refuted H-A, H-B, H-C, H-G, H-H, H-I, H-K)
* d2 amendment in the same d1 plan §9–§10 (refuted H-E, H-F)
* `docs/superpowers/plans/2026-05-04-phase2a-int1-d3-upstream-lp-plan.md`
  (confirmed H-L1 Levinson-cascade, opened H-L2 Chebyshev band-centre,
  opened H-L3 AC-shift heuristic)
**HEAD at entry:** `6e6499f` (post-d3 measurement commit).
**Status:** RED (`TestEncode_LSPVectorBitExact` fatals at frame 29 of
LSP.IN with `g729/lsp: fewer than 5 sign changes in F1 or F2 — LP
filter not stable`); production code FROZEN under I6; this dispatch
performs measurement only.

---

## §0. Invariants

| # | Invariant | Enforcement (this dispatch) |
|---|-----------|------------------------------|
| I1 | Clean-room MIT — no ITU C / bcg729 / Sipro / FFmpeg G.729 source consulted. Spec source = `docs/superpowers/specs/itu/G729E.{pdf,txt}` only. The mirrored autocorrelate / lag-window / Levinson / Chebyshev routines in the d4 test are byte-for-byte transcriptions of production bodies, themselves derived from spec text. The float Chebyshev oracle in §S8 is re-derived directly from §3.2.3 eq. 9–17. | Manual; no fetch / read of forbidden references. |
| I3 | Pure functions where possible; no panics in test code beyond `t.Fatalf` for I/O preconditions. | New test uses `t.Logf` for measurements; `t.Errorf` only on the S0 mirror-bit-exactness assertion (binary signal: trace trustworthy or not); `t.Fatalf` only on file I/O. |
| I5 | Hypothesis-budget cap. **Remains 0/5** for the d4 cycle (measurement only). | No production edits; all 5 attempts remain available for the post-d4 fix dispatch. |
| I6 | Production FROZEN — `internal/lsp/encoder_*.go`, `internal/lpc/*.go`, `internal/pcm/*.go`, `encoder.go` MUST NOT be modified. Only `*_test.go` and `docs/**`. | `git diff --name-only` shows only this plan + one new `*_test.go`. |
| I7 | Measurement-only TDD: emit boundary values via `t.Logf`; no `t.Errorf` for numeric divergence. Hard-asserts restricted to I/O preconditions and the mirror-bit-exact precondition (S0). | New test `TestINT1D4Pinpoint` has exactly one `t.Errorf`, in S0, gating the trustworthiness of every other measurement. |
| I8 | All commits include the Co-authored-by trailer. | Final commit per §8. |

---

## §1. Dispatch brief recap

d3 escalated to four targeted measurements:

* **S5** — Frame-29 per-iteration Levinson trace, with float-oracle
  cross-reference. Pinpoint *which* iteration first saturates and
  *what* Q-format pressure causes the saturation.
* **S6** — Autocorrelation shift sweep on frame 29. Re-run Levinson
  with the production AC scale, scale+1, scale+2, … and check
  whether a small AC-shift bump alone recovers stability.
* **S7** — Frame-5 6- and 8-bisection Chebyshev re-run, comparing
  against the decoder oracle q[5] gap of 3088 Q15.
* **S8** — Float-oracle upstream (Levinson + Chebyshev in real
  arithmetic) → fixed-point production VQ projection. The ultimate
  bisection: does the WANT (1, 5, 14, 20) index tuple emerge if we
  hand the production VQ a clean, ITU-reference-equivalent q[]?

Test:
`internal/lsp/phase2a_int1_d4_pinpoint_test.go::TestINT1D4Pinpoint`.

---

## §2. S0 — Mirror trustworthiness

Before any per-iteration trace can be trusted, the d4 test mirrors
production's pipeline (windowSpeech + autocorrelate + applyLagWindow
+ levinsonDurbin) byte-for-byte and asserts mirror-vs-production
a[Q12] equality across all 30 driven frames.

**Result.** `mirror vs production a[] max drift across all 30 frames
= 0 Q12`. The mirrored pipeline is bit-exact with production; every
trace emitted in §3–§6 below is therefore an exact replay of
production's internal state, not an oracle approximation.

---

## §3. S5 — Frame-29 per-iteration Levinson trace

### §3.1 Raw trace

Production AC scale chosen on frame 29 = **0** (the heuristic finds
that `Σ s²` already fits in Word32). Therefore
`r'[0..10] = [6352423 6247787 5976075 5526681 4928785 4218502
3407775 2556154 1673401 811501 -12377]`.

Per-iteration table (`e` = state value used by iteration i, i.e.
`iter[i-1].eAfter`):

| i  | fix `e` (after) | fix `kQ15` | fix `q = num/e` | fix |aWork[1..i]|max | float `E[i]` | float `k_i` | float |a[1..i]|max |
|----|-----------------|------------|-----------------|----------------------|--------------|-------------|----------------------|
|  1 | 207644          | -32228     | -32228          | 4029                 | 2.073e+05    | -0.983560   | 0.9836               |
|  2 | 69265           | +26750     | +26750          | 7319                 | 6.937e+04    | +0.815653   | 1.7858               |
|  3 | 29394           | +24861     | +24861          | 4783                 | 3.442e+04    | +0.709830   | 1.2068               |
|  4 | 25975           | +11175     | +11175          | 3724                 | 3.299e+04    | +0.204048   | 1.0620               |
|  5 | 11389           | -24555     | -24555          | 4771                 | 1.860e+04    | -0.660333   | 1.1967               |
|  6 |  5559           | -23443     | -23443          | 7097                 | 1.853e+04    | -0.060924   | 1.1565               |
|  7 |     0           | **-32768** | **-45774**      | **7408 (sat)**       | 1.809e+04    | -0.154650   | 1.1471               |
|  8 |     0           |  0         |  0              | 7408                 | 1.677e+04    | +0.270327   | 1.1889               |
|  9 |     0           |  0         |  0              | 7408                 | 1.676e+04    | -0.027926   | 1.1964               |
| 10 |     0           |  0         |  0              | 7408                 | 1.485e+04    | +0.337388   | 1.2059               |

**Cascade entry point: i = 7.**

### §3.2 Q-format diagnosis

At i = 7:

* `sum = Σ_{j=0..6} aWork[j] · r'(7−j) = 31_807_649` (int64, in
  Q12·rscale units; `aWork` is Q12, `r'` is in the AC-shared scale
  of 0).
* `num = -(sum << 3) = -254_461_192` (signed int64).
* `e = 5_559` (state inherited from i = 6's
  `eAfter = (eOld * (2³⁰ − k₆²)) >> 30`).
* `q = num / e = -45_774`. **Outside int16 range** ([-32768, +32767]),
  so `kQ15` saturates to `MinInt16 = -32768`.

The true k₇ is **-0.1546** → Q15 = -5066. The fixed-point pipeline
overshoots by ~9× because the running prediction-error state `e`
has compounded multiplicative truncation in
`e = (e * (2³⁰ − k²)) >> 30`:

| i | fix `e` after | float `E[i]` | fix/float ratio |
|---|---------------|--------------|-----------------|
| 0 | 6_352_423     | 6.36 e+06    | 1.00            |
| 1 | 207_644       | 2.07 e+05    | 1.00            |
| 2 |  69_265       | 6.94 e+04    | 1.00            |
| 3 |  29_394       | 3.44 e+04    | 0.85            |
| 4 |  25_975       | 3.30 e+04    | 0.79            |
| 5 |  11_389       | 1.86 e+04    | 0.61            |
| 6 |   5_559       | 1.85 e+04    | **0.30**        |

By i = 6 the fixed-point `e` has lost ~70 % of its float-equivalent
magnitude. At i = 7 the dividend `(num)` is correctly computed in
int64 (no overflow there: `|sum| < 2³⁵`, `|num| < 2³⁸`, well within
int64), but the *quotient* `num/e` is artificially inflated because
the divisor is too small.

### §3.3 Spec citation and §-conformant fix

Spec §3.2.2 lines 717–736 specify the recursion in **real
arithmetic** with no Q-format clipping. The spec's pre-condition is
that r' be positive-definite — confirmed by the float oracle
(|k_i| < 1 ∀ i). The fixed-point implementation must therefore
carry enough headroom on `e` and `sum` that the quotient `num/e`
**never exceeds Q15 magnitude** when the true k_i is in (−1, +1).
Production's `levinsonDurbin` does not.

The §-conformant fix has two equivalent shapes:

**FIX-1A (preferred): re-normalize `e` between iterations.** Track
an exponent alongside `e` so that after the
`e = (e * oneMinusKSq) >> 30` step `e` is always normalized to the
high bits of int32 (à la `Norm_l(e)` in any spec-style fixed-point
arithmetic library). The same re-normalization shift is applied to
the `sum << 3` numerator before the division. ITU's spec text at
§3.2.1 line 691 ("to avoid arithmetic problems") and the Annex A
overview reference precisely this discipline as standard practice.
Cost in `internal/lpc/levinson.go`: ~10 lines (Norm_l-equivalent
helper + per-iteration shift bookkeeping).

**FIX-1B (alternative): widen `e`/`sum` to int64 with no shift
truncation in the (1−k²) step.** Replace
`e = (e * oneMinusKSq) >> 30` with an int64 product whose shift is
chosen per iteration to keep `e` in the upper half of int64.
Equivalent precision; slightly more state. Same §-citation.

Both fixes require the corresponding update to the `q = num / e`
divide so that `num`'s shift matches `e`'s exponent.

**Estimated I5 cost: 1 of 5** for FIX-1A (single hypothesis,
testable: re-run the integration gate and observe whether frame 29
LP-instability fatals; the two textbook formulations of the fix are
equivalent so this counts as one hypothesis).

---

## §4. S6 — Autocorrelation shift sweep

Frame 29, sweeping `forceScale ∈ {prod, prod+1, …, prod+4}` (i.e.
`{0, 1, 2, 3, 4}`):

| forceScale | a[] (truncated) | kQ15-saturation? | max\|k_fix − k_flt\| | F1 / F2 sign-changes |
|------------|-----------------|------------------|----------------------|----------------------|
| 0 (prod)   | `[4096 356 -7408 -6046 6046 7408 -356 -4096 0 0 0]`     | **true** | 0.8454 | **4** / 5 |
| 1          | `[4096 -1048 -9289 -3505 9766 9766 -3506 -9289 -1047 4095 0]` | **true** | 1.0279 | 5 / 5 |
| 2          | `[4096 -604 -9836 -4254 10622 10622 -4255 -9836 -603 4095 0]` | **true** | 1.0279 | 5 / 5 |
| 3          | `[4096 -1890 -8204 -2125 8140 8140 -2125 -8204 -1889 4095 0]` | **true** | 1.0279 | 5 / 5 |
| 4          | `[4096 -3021 -4207 -1691 3729 3028 2548 -2629 -2836 -1324 2435]` | **false** | 0.3364 | 5 / 5 |

**Verdict.** Adding 1, 2, or 3 bits of AC-shift headroom does **not**
break the saturation cascade — `kQ15` still saturates and `a[]`
retains the mirror-symmetric shape. Stability is recovered only at
**+4 extra bits**, and even then `max|k_fix − k_flt| = 0.336` (poor
precision). Two consequences:

* **H-L3 is REFUTED** in its narrow form: the AC-shift heuristic in
  `internal/lpc/autocorr.go` lines 50–57 is *not* the dominant
  fault. Even +3 bits of headroom does not recover Levinson
  stability; the cascade is intrinsic to `levinsonDurbin`'s Q-format.
* **H-L1 is RE-CONFIRMED** as the sole upstream fault for the
  transient-frame regime. The fix must live in
  `internal/lpc/levinson.go`, not in `autocorr.go`. (Note however
  that scale=1, 2, 3 do produce 5 + 5 sign changes — the saturated
  a[] is "stable enough" by the §3.2.3 sign-change test but still
  numerically wrong. This is a coincidence, not a fix path.)

A side-observation: the F1 sign-change count at scale = 0 is **4**
(the integration-gate fatal trigger). At scale ≥ 1 it becomes 5,
which would make the fatal disappear without actually fixing the
underlying numeric error. **This rules out any "just bump the
AC-shift heuristic" patch as a legitimate fix** — it would merely
mask the LP-instability fatal while still producing wildly wrong
LP coefficients on transients.

---

## §5. S7 — Frame-5 6- and 8-bisection Chebyshev re-run

Decoder-oracle q[] (frame 5, WANT-driven) =
`[31577 30044 27987 22264 12339 -3048 -14910 -21293 -27146 -29427]`.

Sweeping the bisection iteration count on the same encoder a[]:

| variant                | q[] (full)                                                          | max\|Δq\| vs decoder | argmax | sum\|Δq\| |
|------------------------|---------------------------------------------------------------------|----------------------|--------|-----------|
| 4-bisect (production)  | `[31588 30637 27965 22328 12477 40 -16030 -21210 -27161 -29344]`    | **3088 Q15**         | i = 5  | 5217      |
| 6-bisect (mirror)      | `[31600 30623 27972 22318 12464 -1 -15995 -21221 -27185 -29362]`    | **3047 Q15**         | i = 5  | 5104      |
| 8-bisect (mirror)      | `[31597 30622 27970 22320 12454 -11 -15986 -21213 -27187 -29364]`   | **3037 Q15**         | i = 5  | 5083      |

**Verdict.** Doubling and quadrupling the bisection count moves the
q[5] estimate by a mere ~50 Q15 (4-bisect: 40 → 6-bisect: -1 →
8-bisect: -11). The decoder oracle sits at q[5] = -3048. The
encoder a[]'s F2 polynomial places its third zero at x ≈ 0
(ω ≈ π/2); the decoder a[]'s places it at x ≈ -0.093 (ω ≈ π/2 +
0.094). The 3000+ Q15 gap is **not** a bisection-precision artifact;
it is a **true coefficient-domain divergence** between encoder a[]
and decoder a[], inherent to the encoder having to *quantize* its
own LP and the decoder *reconstructing from the quantization*. The
small drift on the encoder side (from production's slight Levinson
imprecision visible in d3 §4 at ~100 Q12 LSB across the 10 a[]
coefficients) is amplified by the Chebyshev root finder's
sensitivity to the F2 third-zero slope near x = 0.

* **H-L2 is REFUTED.** Bisection iteration count is not the issue.
* The §3.2.3 spec text at lines 783–784 *literally* specifies four
  bisections ("the sign change interval is then divided four times
  to allow better tracking of the root"). Production is
  spec-conformant. Increasing the count would have been a spec
  departure for no measurable benefit.

The residual Δq[5] = 3088 Q15 is the natural consequence of the
encoder's L0-best a[] not bit-matching the decoder's reconstructed
a[] — a routine VQ-quantization signature, **not** a Chebyshev bug.

---

## §6. S8 — Float-oracle upstream → fixed-point VQ (CRITICAL)

Drive frames 0..5 with the float-oracle LP analysis (real-domain
windowSpeech + autocorrelate + lag-window + Levinson + Chebyshev
roots), quantize the resulting q[] to Q15 int16, and run **production
fixed-point** `LSPToLSF` + `Quantize` on each frame. Compare the
frame-5 produced indices against WANT = (L0, L1, L2, L3) =
(1, 5, 14, 20).

| Frame | float a[]Q12 (truncated)                            | float q[]Q15 (truncated)                                          | (L0, L1, L2, L3) |
|-------|-----------------------------------------------------|-------------------------------------------------------------------|------------------|
| 0     | `[4096 0 0 0 0 0 0 0 0 0 0]` (silence)               | `[31441 27566 21458 13612 4663 -4663 -13612 -21458 -27566 -31441]` | (0, 120, 2, 11)  |
| 1     | (silence)                                            | (silence init)                                                    | (0, 120, 14, 20) |
| 2     | (silence)                                            | (silence init)                                                    | (0, 120, 7, 11)  |
| 3     | (silence)                                            | (silence init)                                                    | (0, 120, 8, 12)  |
| 4     | (silence)                                            | (silence init)                                                    | (0, 120, 9, 11)  |
| 5     | `[4096 -3931 -1312 639 3247 -2450 566 -86 1107 -1545 841]` | `[31576 30626 27986 22462 12548 84 -15975 -21127 -27390 -29342]` | **(1, 5, 14, 23)** |
| **WANT** | —                                                | —                                                                 | **(1, 5, 14, 20)** |

**Critical verdict — three of four frame-5 indices match WANT.**

* L0 = 1 ✓ (predictor selector)
* L1 = 5 ✓ (first-stage unweighted-MSE pick)
* L2 = 14 ✓ (lower second-stage)
* L3 = **23** vs WANT 20 (residual upper-second-stage miss by 3 codebook indices)

The L1 + L2 + L0 perfect agreement is decisive: the dominant
misalignment between production and WANT lives entirely in the
**upstream LP analysis chain** (Levinson + Chebyshev + window /
autocorrelation). With clean float upstream, production's
fixed-point predictor / weights / VQ search converge on the WANT
first-stage row exactly and the WANT lower-second-stage row exactly.

The residual L3 miss of 3 indices is small. Likely contributors:

1. **Predictor-memory drift across frames 0..4.** All five "silence"
   frames produce indices `(0, 120, …)` — the trivial L0/L1 pick for
   the uniform-spaced cosine-init q[]. ITU's reference encoder may
   pick slightly different L2/L3 on these frames (ITU's WANT
   indices for frames 0..4 are not in scope of this measurement; a
   d5 follow-up could drive freqPrev with the WANT tuples for
   frames 0..4 directly).
2. **Float-oracle q[] vs ITU-reference q[] mismatch at q[5].** Our
   float q[5] = +84; production fixed q[5] = +40; decoder q[5] =
   -3048. All three sit inside the same codebook quantization cell
   on the L1+L2 axes (hence L1, L2 match) but the cell boundary on
   the L3 axis differs by 3 indices.

Neither contributor is large enough to reopen the VQ; both are
downstream consequences of the dominant H-L1 Levinson bug, which
distorts every freqPrev FIFO entry up through frame 5.

---

## §7. Updated hypothesis set

| H | Status after d4 | Evidence |
|---|-----------------|----------|
| H-A — Chebyshev LSB accuracy on healthy frames | refuted (d1, reconfirmed d3 §5.1) | unchanged |
| H-B — partial-cost convention | refuted (d1) | unchanged |
| H-C — weight Q-format / boost | refuted (d1) | unchanged |
| H-D — silent-input LP convention | live for frames 0..4 | promoted to H-L4 below |
| H-E — codebook row indexing | refuted (d2) | unchanged |
| H-F — MA-predictor table indexing | refuted (d2) | unchanged |
| H-G — frame alignment | refuted (d1) | unchanged |
| H-H — pre-processor on silence | refuted (d1) | unchanged |
| H-I — eq. 21 cost full-vs-partial | refuted (d1) | unchanged |
| H-J — round-trip qLSP→ωLSF noise | refuted (d1) | unchanged |
| H-K — stability in L0 cost | refuted (d1) | unchanged |
| H-L — upstream LP analysis divergence | **CONFIRMED as dominant** | §6 S8: float upstream → 3-of-4 WANT indices recovered. |
| **H-L1 — Levinson saturation cascade on transients** | **CONFIRMED with §-citable root cause** | §3 S5: cascade enters at i = 7, frame 29; root cause = `e` truncation in `e = (e·(2³⁰−k²))>>30` over iterations 3–6, leaving `e=5559` against true E[6]=18534 (3.3× too small), so the i=7 quotient `num/e=-45774` overflows int16. Spec §3.2.2 lines 717–736 require real-arithmetic equivalence; the Q-format implementation must re-normalize `e` between iterations. |
| **H-L2 — Chebyshev refinement bias near band-centre** | **REFUTED** | §5 S7: 6 and 8 bisections move q[5] by ≤50 Q15; the 3088 Q15 gap is encoder-vs-decoder a[] divergence (VQ quantization residual), not bisection precision. |
| **H-L3 — autocorrelation AC-shift heuristic** | **REFUTED in its narrow form** | §4 S6: scale+1, scale+2, scale+3 still saturate; only scale+4 recovers stability and even then with poor precision. The autocorr heuristic is not the bug; Levinson Q-format is. |
| **H-L4 — silence/cold-start LSP encoding** (NEW) | live, low priority | §6 S8: frames 0..4 (silence) all produce (L0=0, L1=120, …) under the float-oracle path. ITU's reference indices for these frames may differ; the resulting freqPrev drift is small (3 L3 indices at frame 5) but non-zero. Investigate in a future d5/d6 only if the H-L1 fix doesn't fully close the integration gate. |

---

## §8. Disposition

**FIX-PROPOSED — single module, single hypothesis.**

### FIX-1A (preferred): re-normalize `e` between Levinson iterations

**Module:** `internal/lpc/levinson.go`

**Lines:** 41–86 (the body of `levinsonDurbin`)

**Change shape:**
* Add a per-iteration `eShift int` exponent tracking how many bits
  `e` has been left-shifted to maintain Q-format normalization.
* After `e = (e * oneMinusKSq) >> 30`, count leading zeros of `|e|`
  and left-shift `e` to the upper bits of int32 (Norm_l-style),
  incrementing `eShift` accordingly.
* Apply the same shift correction to `sum` (or, equivalently, to
  the `num << 3` numerator) before the division so that
  `q = num / e` retains its Q15 interpretation.
* No change to the `aWork` update rule, `kQ15`-to-`aWork[i]` cast,
  or final saturation.

**Spec citation:** §3.2.2 lines 717–736 (real-arithmetic recursion);
§3.2.1 line 691 ("to avoid arithmetic problems") establishes the
spec's general acknowledgement that Q-format normalization is the
implementer's responsibility.

**I5 budget cost: 1 of 5.** Single hypothesis: re-normalize `e`
between iterations. Pass criterion: `TestEncode_LSPVectorBitExact`
no longer fatals at frame 29; mirror trace shows `kQ15` no longer
saturates at i = 7; per-iteration `e/E_float` ratio stays within
±10 % through i = 10.

### FIX-2 (NOT proposed, kept for record): Chebyshev re-fit

H-L2 is refuted; no fix proposed. Production's 4-bisection is
spec-conformant and the q[5] gap is not bisection-driven.

### Out-of-scope deferred items

* H-L4 (silence-frame predictor-memory drift). Defer to a possible
  d5 cycle iff H-L1 fix alone leaves the integration gate RED at a
  frame later than 29. Current expectation: H-L1 alone unblocks at
  least the LP-instability fatal at frame 29 and likely closes the
  bit-exact gate; the residual L3 miss observed in §6 may
  self-resolve once frames 0..4 use a Q-correct Levinson.

---

## §9. Hand-off checklist for the post-d4 fix dispatch

* [ ] **Lift I6 freeze on `internal/lpc/levinson.go` only.** All
      other production files remain frozen.
* [ ] Apply FIX-1A as documented in §8 (or its FIX-1B equivalent).
* [ ] Re-run `TestINT1D4Pinpoint` and verify:
      - S0 still passes (mirror needs to be updated to match the new
        production body — small follow-up edit in the test file).
      - S5 trace shows `kQ15` not saturating at i = 7 on frame 29;
        per-iteration `e/E_float` ratio stays within ±10 % through
        i = 10.
* [ ] Re-run `TestEncode_LSPVectorBitExact` and confirm the
      LP-instability fatal at frame 29 is resolved.
* [ ] If the integration gate now produces frame-5 indices matching
      WANT, close E9. If a residual frame-5 L3 miss remains (3
      indices per §6), open a d5 dispatch targeted at H-L4.
* [ ] **Spend 1 of 5 I5 attempts.**

---

## §10. Cross-references

* d1 measurement test:
  `internal/lsp/phase2a_int1_d1_closed_form_test.go`
* d2 measurement test:
  `internal/lsp/phase2a_int1_d2_closed_form_test.go`
* d3 measurement test:
  `internal/lsp/phase2a_int1_d3_upstream_lp_test.go`
* **d4 measurement test:
  `internal/lsp/phase2a_int1_d4_pinpoint_test.go`**
* Integration gate:
  `lsp_itu_vector_test.go::TestEncode_LSPVectorBitExact`
* Spec §-cites used by d4:
  * §3.2.1 eq. 3 (LP analysis window) — d4 mirror.
  * §3.2.1 eq. 5 (autocorrelation) — d4 mirror + S6 sweep.
  * §3.2.1 eq. 6 (lag window) — d4 mirror.
  * §3.2.1 eq. 7 (white-noise correction r(0)·1.0001) — d4 mirror.
  * §3.2.1 line 691 ("to avoid arithmetic problems") — H-L1 fix
    citation.
  * §3.2.2 lines 717–736 (Levinson recursion) — H-L1 root cause and
    fix citation.
  * §3.2.3 lines 738–784 (LP→LSP, F1/F2, 60-point grid, four
    bisections) — S7 spec-conformance check.
  * §3.2.3 eq. 9–17 (F1/F2 polynomials + Chebyshev evaluation) —
    S8 float oracle.
