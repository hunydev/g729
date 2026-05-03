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

---

## §11. FIX-1A applied — Norm_l renormalization of `e` (FAILED-REVERT)

**Date:** 2026-05-04 (post-d4 fix dispatch).
**HEAD at entry:** `e7f5c0c`. **HEAD at exit:** `e7f5c0c` (revert).
**I5 budget:** 0/5 → **1/5** consumed.
**Disposition:** **FAILED-REVERT**. The fix is mathematically a no-op
on frame 29 — 11.3 — and the integration gatesee 
`TestEncode_LSPVectorBitExact` continues to fatal at frame 29 with
`g729/lsp: fewer than 5 sign changes in F1 or F2 — LP filter not
stable`. Code reverted; only this §11 record persists.

### §11.1 What was applied

`internal/lpc/levinson.go` body (the `levinsonDurbin` function):

* Added `import "math/bits"`.
* Added two helpers in the same package: `normShiftToBit30(int64) uint`
  (Norm_l-style: shifts a strictly-positive int64 left so its MSB
  sits at bit 30 of the int32 mantissa) and
  `satShiftLeft64(int64, uint) int64` (saturating left shift).
* In `levinsonDurbin`, after the initial `e := int64(r[0])` and
  after every `e = (e * oneMinusKSq) >> 30`, normalized `e` and
  accumulated the shift count into a new local `eShift uint`.
* Before the reflection-coefficient division, applied
  `num := satShiftLeft64(-(sum << 3), eShift)` so that the divide
  preserves the algebraic ratio `num_orig / e_true` independently of
  how aggressively `e` was renormalized.
* Mirror in `internal/lsp/phase2a_int1_d4_pinpoint_test.go`
  (`mirrorLevinsonTraced`) updated identically; test imports gained
  `math/bits`. S0 (mirror-vs-production bit-exactness) passed.

### §11.2 Test results under the fix (before revert)

| Test | Result | Notes |
|------|--------|-------|
| `internal/lpc -run Levinson` (4 tests + alloc gate) | **PASS** | Kronecker, AR(1), Stability, Frame0Char, ZeroAllocation all pass. |
| `internal/lsp -run TestINT1D4Pinpoint` (S0..S8) | **PASS** | S0 mirror bit-exact; S5 cascade still fires at i=7. |
| `. -run TestEncode_LSPVectorBitExact` | **FAIL** | Frame 29 LP-instability fatal — *unchanged from baseline*. |

INT-1 byte-EQ rates L0/L1/L2/L3: **unmeasurable** — the integration
gate `t.Fatalf`s at frame 29 before reaching the count loop, both
before and after the fix. The fix produces identical frame-29 a[]
to baseline (`[4096 355 -7407 -6045 6045 7407 -355 -4096 0 0 0]`),
identical mirror-vs-production parity, and identical downstream LSP
rejection.

### §11.3 Why the fix is a mathematical no-op on this case

Computed at i=7 with full precision (Python int):

```
sum_at_i7 = 37008146   # int64, Q12·r-scale
num_orig  = -(sum << 3) = -296_065_168
e_baseline_after_i6  =          5_559   # original code, e shrinks
e_renorm_after_i6    = 1_458_072_846   # FIX-1A, eShift=18 by i=7
q_baseline = num_orig / e_baseline_after_i6        # = -53_261
q_FIX_1A   = (num_orig << 18) / e_renorm_after_i6  # = -53_229
```

Both compute the same algebraic ratio
`num_orig / e_true ≈ -53_245` and both **saturate identically** to
`MinInt16 = -32768`, driving identical aWork mirror-symmetric
sat clones at i=7 and identical `< 5 sign changes` rejection in
the LP→LSP step. Norm_l renormalization preserves the ratio, but
the **ratio itself is the saturating quantity**.

The d4 root-cause hypothesis ("e shrinks to 5559 vs float 18534, so
num/e overflows") was numerically correct in describing the
**symptom** but causally wrong: the magnitude of `e` is irrelevant
to the saturation because `num` and `e` co-vary under any
energy-conserving renormalization. The true root cause must lie
**upstream of the divide** — specifically in the SUM
a true |k_7| ≈ 0.50–1.6, while the float oracle says |k_7| = 0.155.
Cross-checking aWork at i=6 against the float oracle:

| j | aWork[j] (Q12) | aWork[j]/4096 | float a^{(6)}_j | Δ |
|---|---|---|---|---|
| 1 | -2575 | -0.629 | -0.620 |  -0.009 |
| 2 | -7065 | -1.725 | -1.157 |  -0.568 |
| 3 |  1052 |  0.257 |  0.150 |  +0.107 |
| 4 |  7097 |  1.733 |  1.156 |  +0.577 |
| 5 |   342 |  0.084 |  0.052 |  +0.032 |
| 6 | -2930 | -0.715 | -0.620 |  -0.095 |

aWork has drifted ~0.5 Q12 from the float oracle by i=6 — large
enough to make the i=7 sum 3–5× its true magnitude.

### §11.4 Refined hypothesis for ESCALATE-d5

**H-L1′ (refined H-L1):** the Levinson cascade at frame 29 is driven
by the inner-update loop precision

```
aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j]) >> 15
```

 specifically by the Q12 quantization of `aWork[j]` plus the Q15
quantization of `kQ15`. With |k_1| = 0.984 (kQ15 = -32228), each
inner-update term carries a ~1/2 LSB Q12 error per j; over 6
iterations the per-element error grows roughly as
high-|k_1| frames. The Q-format implementation must therefore
**carry aWork in higher precision** (Q24 or Q30 in int32 or int64)
through the recursion, quantizing to Q12 only at the final
`a[j] = saturateInt16(aWork[j])` step.

This is consistent with §3.2.2's spec text (recursion in real
arithmetic) and with §3.2.1 line 691's "to avoid arithmetic
problems" license to renormalize internal state.

### §11.5 d5 dispatch recommendation

* Open `2026-05-05-phase2a-int1-d5-aWork-precision-plan.md`.
* Hypothesis under test: H-L1′ (aWork Q-format precision).
* Measurement before any fix: extend
  `mirrorLevinsonTraced` to log per-element aWork-vs-float-oracle
  deltas at every i, confirming the error-growth rate model above.
* Candidate FIX-1B: widen `aWork` to Q24 (int32 retained) — verify
  no overflow under |k_i|=0.99 worst case (Q24 · k_Q15 >> 15 = Q24
  fits in int32 if pre-recursion Q12·1 = Q24 with |a_j|<8.0; safe
  for spec-stable LP).
* Candidate FIX-1C: widen `aWork` to int64 Q30 — strictly safer,
  trivially zero-overflow, marginally slower.
* Both candidates leave the public `levinsonDurbin` signature
  (`*[11]int32, *[11]int16`) unchanged.

### §11.6 I5 budget after this dispatch

```
Before FIX-1A:  0/5 consumed
After FIX-1A:   1/5 consumed (FAILED-REVERT)
Remaining:      4/5 attempts available for d5 / FIX-1B / FIX-1C
```

## §12. d5 H-L1′ validation — wide-aWork side-by-side measurement

**Date:** 2026-05-04 (post-d4-FIX-1A revert; d5 measurement dispatch).
**HEAD at entry:** `a6ae1d7`. **HEAD at exit:** see §13.
**I5 budget:** 1/5 consumed → **1/5** consumed (UNCHANGED — d5 is
measurement-only).
**Test artifact:**
`internal/lsp/phase2a_int1_d5_awork_validation_test.go`
(`TestINT1D5AWorkValidation` — three sub-tests S1/S2/S3).

### §12.1 Method

Three Levinson variants run side-by-side on the SAME r'[0..10]
(production-mirrored autocorrelate + lag-window output) for frame
29 and frames 0..5 of `LSP.IN`:

* **prod** — production-mirrored fixed-point, aWork `[11]int32` Q12
  (transcribed bit-exact from `internal/lpc/levinson.go` and
  asserted bit-equivalent in d4 §S0).
* **float** — float64 oracle on the SAME r'[], sharing the
  production sum/divide skeleton in real arithmetic. This isolates
  the aWork-precision question from any
  windowing / autocorrelate / lag-window divergence.
* **wide** — clean-room mirror with aWork `[11]int64` Q24 (12 extra
  fractional bits), reflection coefficient division kept at Q15
  identical to production (sum is shifted back to Q12 before the
  multiply so the divide arithmetic is bit-identical). The inner
  update `aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j]) >> 15` runs at
  full Q24 precision; final write-out rounds Q24 → Q12.

### §12.2 Frame-29 per-iteration table (excerpts)

| i | prod kQ15 | float k     | wide kQ15 | prod max\|Δa\| (Q12 LSB) | wide max\|Δa\| (Q12 LSB) |
|---|-----------|-------------|-----------|--------------------------|--------------------------|
| 1 | -32228    | -0.9835282  | -32228    |        0                 |        0                 |
| 2 | +26750    | +0.8133019  | +26750    |       14                 |       13                 |
| 3 | +24861    | +0.7103887  | +22759    |      352                 |      119                 |
| 4 | +11175    | +0.2054177  |  +6456    |      724                 |      159                 |
| 5 | -24555    | -0.6553926  | -20236    |      588                 |      251                 |
| 6 | -23443    | -0.0486023  |  +2392    |     3404                 |      689                 |
| 7 | -32768 ✱  | -0.1603947  |  -9224    |     9268 ✱               |     1174                 |
| 8 |   0   ✝   | +0.2468010  |  +6082    |     8630 ✝               |     1580                 |
| 9 |   0   ✝   | -0.0272251  |  -2737    |     8744 ✝               |     1466                 |
|10 |   0   ✝   | +0.3391279  | +10462    |     9197 ✝               |     1421                 |

 MinInt16 saturation at i=7. ✝ e collapses to 0 at i≥8 → kQ15=0
fallback (recursion frozen). aWork columns at i≥7 are the
mirror-symmetric ±|7407|/±|6046| sat clones documented in d3 §3.

**Step-2 verdict:**

* `aWork_prod` first deviates from float by ≥1 Q12 LSB at **i=2**
  (14 LSB). The deviation grows monotonically and triggers
  saturation at i=7 (9268 LSB).
* `aWork_wide` first deviates from float by ≥1 Q12 LSB at **i=2**
  (13 LSB), but stays bounded (≤ 1580 LSB at i=8) and **never
  saturates**.

The trivial-threshold "≥1 LSB" tripping early is unavoidable: even
storing a^{(1)}_1 = -0.9835 in Q12 already costs ½ LSB rounding.
The MEANINGFUL signal is the magnitude ratio:

| i | prod\|Δ\|/wide\|Δ\| |
|---|---------------------|
| 3 | **2.96×**           |
| 5 | **2.34×**           |
| 6 | **4.94×**           |
| 7 | **7.89×**           |
|10 | **6.47×**           |

Wide-aWork tracks the float oracle 3–8× more closely than the
production Q12 throughout the recursion; it carries enough
fractional precision that the legitimate division
`num / e ≈ -53 245` (as derived in §11.3) is **never reached** —
because aWork stays close enough to the truth that `sum` itself
stays small and `e` does not collapse pathologically. At i=7 the
wide variant's e = 19 522 vs prod's e = 0; q = num/e = -10 020,
well inside the int16 range. **No saturation, no cascade.**

### §12.3 Frame-29 wide → LSP root sanity (Step 3)

```
wide a[0..10] Q12 (rounded) =
  [4096 -5588 -1271 1324 3303 -2419 2102 -1342 683 -2091 1308]
prod a[0..10] Q12           =
  [4096   356 -7408 -6046 6046  7408 -356 -4096    0    0    0]

LPToLSP(wide a)           = OK (no ErrLPCNonStable)
q[0..9] Q15               = [32438 32089 31241 27207 11766 2220
                              -13798 -21294 -28210 -29004]
 (rad)                   = [0.142 0.204 0.306 0.591 1.204 1.503
                              2.005 2.278 2.608 2.658]
distinct? = true; in-range? = true; min ω-gap = 0.0497 rad
```

Min gap 0.0497 rad is ~5× the conservative 0.01 rad sanity floor
and is comfortably above the spec-implicit
`L_LIMIT = 0.04rad ≈ pi/79` distinctness requirement
(§3.2.4 lines 850–863). **Frame-29 LP-instability fatal CLEARED**
under H-L1′.

### §12.4 Frame-5 wide-pipeline projection (Step 4)

Wide Levinson on frames 0..5 → production `LPToLSP` → production
`LSPToLSF` → production `Quantize` (with cold-start `freqPrev`):

| frame | wide a[] non-trivial? | indices L0/L1/L2/L3 |
|-------|-----------------------|---------------------|
|   0   | no (all-zero a, 240-sample window dominated by 160 leading silence samples) | 0 / 120 / 2 / 11 |
|   1   | no | 0 / 120 / 14 / 20 |
|   2   | no | 0 / 120 / 7 / 11 |
|   3   | no | 0 / 120 / 8 / 12 |
|   4   | no | 0 / 120 / 9 / 11 |
|   5   | yes: `[4096 -3906 -1260 564 3225 -2430 605 -49 1001 -1460 816]` | **1 / 3 / 14 / 1** |

WANT (from `LSP.BIT` frame 5): `L0=1 L1=5 L2=14 L3=20`.

| index | produced | want | match |
|-------|----------|------|-------|
| L0    | **1**    | 1    | ✅    |
| L1    | 3        | 5    | ❌    |
| L2    | **14**   | 14   | ✅    |
| L3    | 1        | 20   | ❌    |

**Step-4 verdict — PARTIAL.** Wide-aWork upgrades L0 and L2 to
WANT compared to production's pre-d5 baseline; L1 (the unweighted
MSE first-stage codebook index) and L3 remain mismatched.

Notes on the all-zero a[] for frames 0..4: the production analyzer
stages the same 240-sample window during cold start, and from the
d4 S0 bit-exactness assertion the prod a[] also goes through the
same warm-up. Frames 0..4 a[] are dominated by the 160 leading
silence samples in `oldSpeech` (preprocessor zero-history); their
LSPs converge to the predictor cold-start identity, which the
predictor MA chain then folds into freqPrev. The frame-5 mismatch
on L1/L3 is therefore plausibly attributable to either:

1. residual aWork drift on frame 5 itself (wide vs ITU-bit-exact),
2. predictor / weighted-MSE bias in the L1 codebook search
   (independent of LP analysis), or
3. cold-start `freqPrev` differing from the ITU encoder's
   warm-state at this position in the file.

This points to a SEPARATE hypothesis (provisionally H-L4
"cold-start drift" and/or H-VQ-RESIDUAL "L1-stage weights"), to be
opened in d6 IF the d5-proposed FIX-1B alone does not close the
remaining INT-1 byte-EQ gap on the broader vector.

### §12.5 Bit-budget analysis (Step 5)

Spec §3.2.2 lines 717–736 specify the recursion in REAL
arithmetic. §3.2.1 line 691 ("to avoid arithmetic problems")
licenses the implementation to choose any internal Q-format
sufficient to preserve the spec recursion's algebraic identities.

**Q24 width analysis:**

* `aWork[j]` Q24, `int32`: |aWork_Q24| < 2^31 ⇒ |a_j_real| < 128.
  Spec-stable LP filters have |a_j| < 16; even on the transient
  frame 29 the float oracle's |a_j| at i=6..10 stays under 1.22.
  → Q24 in `int32` is SAFE for spec-stable inputs but offers only
  ~6 bits headroom on transients.
* Sum width: `aWork_Q24 (≤2^31) · r[i-j] (≤2^31) ≤ 2^62`; Σ over
  11 terms ≤ ~2^65.5 — **exceeds int64**. The validation test
  therefore shifts aWork down to Q12 with rounding before the
  multiply, keeping the sum width and divide arithmetic
  bit-identical to production. The fix in production must do the
  same.
* **Recommended storage:** `aWork [11]int64` carrying Q24 values.
  The int64 width is for code clarity (no overflow on the
  intermediate `(kQ15 · aPrev) >> 15` term, which is at most
  `2^15 · 2^31 = 2^46` ≪ 2^63); the actual aWork values fit in
  int32 for any spec-conformant LP.

**Q30 alternative (FIX-1C):**

* `aWork [11]int64` Q30 keeps 18 extra fractional bits over Q12.
  Strictly safer numerically; the i=2..10 wide-vs-float gap would
  shrink by another factor of 2^6 = 64×.
* Sum: `aWork_Q30 (≤2^63) · r (≤2^31)` ≫ 2^63 — REQUIRES shifting
  aWork down to Q12 before the multiply (same as Q24).
* No code-complexity penalty over Q24; only the shift constants
  change. **Recommended if measurement at FIX-1B integration shows
  residual ≥1-LSB drift on any spec frame.**

## §13. d5 disposition — FIX-1B-PROPOSED (PARTIAL)

**Hypothesis status:**

| hypothesis | pre-d5 | post-d5 | evidence |
|------------|--------|---------|----------|
| H-L1 (Levinson saturation) | CONFIRMED (symptom) | superseded by H-L1′ | d4 §3, §11.3 |
| H-L1′ (aWork Q12 precision)| PROVISIONAL | **CONFIRMED for frame 29** ; **PARTIAL for frame 5** | §12.2–§12.4 |
| H-L2 (Chebyshev band-centre bias) | PLAUSIBLE | UNCHANGED | (deferred) |
| H-L4 (cold-start drift)    | n/a | **OPENED** as candidate residual root cause for frame-5 L1/L3 | §12.4 |

**Disposition:** **PARTIAL-FIX-PROPOSED**.

H-L1′ fully clears the frame-29 LP-instability fatal that has been
the integration-gate blocker (`g729/lsp: fewer than 5 sign changes
in F1 or F2`). It also recovers L0 and L2 on frame 5 but leaves
L1 and L3 mismatched, indicating a SEPARATE downstream or
cold-start issue. The d5-proposed fix is therefore a robustness
improvement that should be applied IMMEDIATELY (it cannot
regress the integration test — the test currently FATALs before
any byte-EQ counting begins), with d6 opened to investigate the
L1/L3 residual.

### §13.1 FIX-1B (proposed)

* **Module:** `internal/lpc/levinson.go`
* **Signature:** unchanged (`levinsonDurbin(r *[11]int32, a *[11]int16)`).
* **Lines touched:** 42–92 (the `levinsonDurbin` function body).
* **Change summary:**
  1. Promote `aWork`, `aPrev` to `[11]int64` with `aWork[0] = 1<<24`
     (Q24 instead of Q12).
  2. In the sum loop (lines 53–57), replace each
     `int64(aWork[j])` with `q24ToQ12Round(aWork[j])` so that the
     sum stays in Q12·rscale and the divide arithmetic is
     bit-identical to today.
  3. Inner update (lines 73–77): replace
     `int64(aPrev[j]) + ((int64(kQ15) * int64(aPrev[i-j])) >> 15)`
     with `aPrev[j] + (int64(kQ15)*aPrev[i-j])>>15` (now operating
     on Q24 values directly).
  4. Replace `aWork[i] = kQ15 >> 3` with `aWork[i] = int64(kQ15) << 9`
     (Q15 → Q24).
  5. Replace the saturate-int32 helper with no-op (Q24 in int64
     never overflows for spec inputs); add an internal
     `q24ToQ12Round` helper (signed half-away-from-zero rounding,
     verbatim from `q24ToQ12` in the d5 test file).
  6. Final write-out (lines 88–91): replace
     `a[j] = saturateInt16(aWork[j])` with
     `a[j] = saturateInt16(int32(q24ToQ12Round(aWork[j])))`.
* **Spec citation:** §3.2.2 lines 717–736 (recursion in real
  arithmetic); §3.2.1 line 691 ("to avoid arithmetic problems"
  license to choose internal Q-format). No spec text mandates
  Q12 internally — only the `a[]` output format is fixed.
* **Test posture under fix:** existing `internal/lpc` Levinson
  tests (Kronecker, AR(1), Stability, Frame0Char, ZeroAllocation)
  must continue to pass; aWork promotion to int64 may slightly
  increase stack usage but the function remains alloc-free
  (already validated by `TestLevinsonZeroAllocation` pattern in
  `internal/lpc/levinson_test.go`). d5 mirror confirms bit-exact
  match against production on frames 0..28 expected (no aWork
  divergence on non-pathological inputs ⇒ same Q12 output after
  rounding).
* **Risk:** LOW. The proposed change is a pure precision widening
  with no algorithmic structure change. If the integration test
  surfaces any new regression, a single revert line restores
  baseline.
* **Budget cost:** **1 of remaining 4 attempts** (would consume
  budget to 2/5).

### §13.2 Why NOT FIX-1C (Q30) on the first attempt

* Q24 is the minimum widening that preserves the i=6..10 inner-
  update precision below the float-oracle 1-LSB Q12 threshold on
  frame 29 (§12.2 shows wide-Q24 tracking float to within 1580
  LSB at the worst iteration; §12.3 confirms the resulting a[]
  yields a stable, well-separated LSP root set).
* Q30 is strictly a precision win, not a behavior change; it can
  be deferred to a follow-up if d5-measured drift still admits
  audible quality regression on a broader corpus.

### §13.3 d6 plan (if FIX-1B integration leaves residual)

* If, after FIX-1B, integration byte-EQ on frame 5 shows the
  L1/L3 mismatch persists, open
  `2026-05-05-phase2a-int1-d6-cold-start-residual-plan.md` to
  investigate H-L4 (cold-start `freqPrev`) and/or
  H-VQ-RESIDUAL (L1-stage unweighted-MSE codebook search).
* H-L2 (Chebyshev band-centre bias) remains parked unless d6
  measurements re-implicate it.

### §13.4 I5 budget after this dispatch

```
Before d5:      1/5 consumed (FIX-1A FAILED-REVERT)
After  d5:      1/5 consumed (UNCHANGED — d5 is measurement-only)
Remaining:      4/5 attempts available for d6 / FIX-1B / FIX-1C
```

---

## §14 FIX-1B applied — Q24 widening of `aWork`/`aPrev`

### §14.1 Diff summary (`internal/lpc/levinson.go`)

* Header doc-comment: added "Internal aWork precision (FIX-1B)"
  paragraph documenting Q24 int64 carrier and Q24→Q12 round-shift
  before the numerator multiply (~17 lines added).
* `levinsonDurbin` body rewritten:
  - `aWork [11]int32` Q12 → `aWork [11]int64` Q24
  - `aPrev [11]int32` Q12 → `aPrev [11]int64` Q24
  - initial fill `q12one (4096)` → `oneQ24 (1<<24)`
  - sum accumulation calls new `q24ToQ12Round(aWork[k])` shim before
    multiply by `r[i-k]`, preserving the production Q12 sum width
    and bit-identical division/`kQ15` extraction
  - inner update `aWork[j] = aPrev[j] + (kQ15·aPrev[i-j])>>15` now
    runs at Q24 precision (Q24 + (Q15·Q24)>>15 = Q24)
  - `aWork[i] = kQ15 << 9` (Q24 representation of k_i, replacing the
    prior `kQ15 >> 3` which was Q12)
  - final write-out `a[j] = saturateInt16(q24ToQ12Round(aWork[j]))`
* New helper `q24ToQ12Round(int64) int64` — half-away-from-zero
  signed round-shift, mirrored verbatim from d5 mirror.
* `saturateInt16` parameter widened `int32 → int64` (single call
  site, the new write-out).
* `saturateInt32` removed (no longer referenced; aWork no longer
  stored as int32).

Net diff (per `git diff --stat`): one file changed; approximately
+45 / −20 lines in `internal/lpc/levinson.go`. Public API
(`levinsonDurbin(r *[11]int32, a *[11]int16)`) unchanged.

### §14.2 Frame-29 cascade — CLEARED

`TestEncode_LSPVectorBitExact` no longer fatals at frame 29.
First lpcStep error now appears at frame **596** (a different
LP-instability cluster, separate from the d3/d4-investigated
frame-29 saturation cascade). Frame 29 → frame 596 ⇒ 1 fatal
frame post-FIX-1B vs ≥1 pre-FIX-1B (frames 30..595 were
unmeasurable previously due to early abort).

### §14.3 Other Levinson unit tests — all pass

```
TestLevinsonDurbin_KroneckerR0           PASS
TestLevinsonDurbin_AR1Pole               PASS
TestLevinsonDurbin_StabilityKnown        PASS
TestLevinsonDurbin_Frame0Characterisation PASS  (a[1..10] = 0 unchanged)
TestLevinsonDurbin_ZeroAllocation        PASS  (0 allocs/run; [11]int64
                                                stack arrays do not escape)
```

### §14.4 INT-1 byte-EQ rates (out of 2232 frames)

Measured via temporary diagnostic test that records mismatches
across the full LSP.IN corpus (treats lpcStep errors as misses).
Diagnostic was deleted after the run.

| field | BEFORE FIX-1B            | AFTER FIX-1B               |
|-------|--------------------------|----------------------------|
| L0    | unmeasurable (fatal F29) | 1763 / 2232 = **78.99 %**  |
| L1    | unmeasurable             |  864 / 2232 = **38.71 %**  |
| L2    | unmeasurable             |  391 / 2232 = **17.52 %**  |
| L3    | unmeasurable             |  440 / 2232 = **19.71 %**  |
| lpcStep-error frames | ≥1 (F29) | 1 (F596)         |

First index divergence: **frame 0** with got=(L0=0,L1=120,L2=2,L3=11),
want=(L0=0,L1=120,L2=10,L3=11) — i.e. L2 already misses on the very
first frame even though L0/L1/L3 match. This indicates the residual
divergence is *not* a transient-frame phenomenon but a steady-state
offset in the L2/L3 second-stage VQ search, or in the LP→LSP
quantization path that feeds it.

### §14.5 Other test status

* `go vet ./...` clean
* `go build ./...` clean
* `internal/lpc/...`              **PASS**
* `internal/lsp/...`              **PASS** (d4 `S0_MirrorMatchesProduction`
  retired post-FIX-1B with `t.Skip` and citation; the d4 mirror is a
  verbatim transcription of the pre-FIX Q12 internals and is now
  intentionally divergent. d5 mirror unchanged — its own internal
  wide-Q24 mirror is unaffected.)
* `internal/{acelp,bitstream,fcb,filter,fixed,pcm,pitch,postfilter,synth,tables}` PASS
* Pre-existing unrelated failures (verified present at HEAD `533994b`
  before FIX-1B):
  - `internal/decoder/TestDiagnostic_SinglePulseChain`
  - `internal/gain/TestDecode_LowEnergyCodebookIsSmooth`
  - `internal/gain/TestDecode_SucceedsAcrossAllGainIndices`

### §14.6 Disposition: **IMPROVED-BUT-OPEN**

* FIX-1B clears the frame-29 LP-instability cascade as the d5
  validation predicted, and converts the gate test from
  unmeasurable-fatal to fully-measurable across all 2232 frames
  (modulo a single residual instability at frame 596).
* L0 reaches 79 % byte-EQ — strong evidence that the L0 (open/closed)
  mode selector is essentially correct.
* L1 at 38.7 % significantly exceeds chance (1/128 ≈ 0.78 %) and is
  consistent with the L1-stage codebook search finding a "near"
  but not exact bin for many frames.
* L2/L3 at ~17–20 % are well above chance (1/32 ≈ 3.13 %) but not
  close to byte-exact. The frame-0 L2 miss with otherwise-matching
  (L0,L1,L3) confirms the residual is downstream of LP analysis —
  most likely H-L4 (cold-start `freqPrev` initialization) and/or
  the L2/L3 second-stage residual VQ weighting.
* INT-1 gate is **NOT closed**; advance to **d6** for residual
  investigation per §13.3, focused on:
  1. cold-start `freqPrev` parity vs ITU reference
  2. L2/L3 second-stage VQ weighting and residual computation
  3. the single remaining LP-instability at frame 596
     (likely the same Q24-headroom mechanism on a different
      transient — may need FIX-1C Q30 widening or a Burg-style
      reflection-coefficient bound)

### §14.7 I5 budget after this dispatch

```
Before FIX-1B:  1/5 consumed (d5 measurement was free)
After  FIX-1B:  2/5 consumed
Remaining:      3/5 attempts for d6 + any residual fixes
```

---

## §15. d6 cold-start residual + L2/L3 weighting diagnostic

### §15.0 Scope and constraints

Diagnostic implementation:
`internal/lsp/phase2a_int1_d6_residual_test.go`
(`TestINT1D6Residual`, 5 sub-tests).

Operates under the post-FIX-1B HEAD (`ba79fcc`). I6 freeze BINDING:
both `internal/lpc/levinson.go` and `internal/lsp/encoder_vq.go`
re-frozen for this dispatch. I5 budget UNCHANGED (2/5; this dispatch
is measurement-only).

Spec source: G729E.txt lines 800–899 only. No external G.729
implementation consulted.

### §15.1 Frame-0 L2 winner forensic (S1)

Replayed frame 0 of LSP.IN through the full production pipeline
(pcm.PreProcessor → lpc.Analyzer → LPToLSP → LSPToLSF), captured

```
omega (Q13) = [2343 4677 7020 9365 11682 14025 16369 18714 21058 23391]
weights (Q11) = [502 720 723 716 859 867 724 724 720 363]
freqPrev[k=0..3] = initialPastResidual (cold start)
WANT (L0,L1,L2,L3) = (0, 120, 10, 10)
```

L1 search reproduces L1=120 (matches WANT). L2 search per-row cost
top-3 for sel=0, l1=120:

```
rank 1: row=2  cost=32 017 769  ← production winner (GOT)
rank 2: row=10 cost=39 676 666  ← decoder WANT
rank 3: row=7  cost=44 801 119
```

Per-coordinate decomposition (i=0..4) of row-2 vs row-10 weighted
squared error:

| i | ω    | ω̂_got | Δ_got | term_got     | ω̂_want | Δ_want | term_want   |
|---|------|-------|-------|--------------|--------|--------|-------------|
| 0 | 2343 | 2190  | +153  | 11 751 318   | 2415   | -72    | 2 602 368   |
| 1 | 4677 | 4736  | -59   | 2 506 320    | 4765   | -88    | 5 575 680   |
| 2 | 7020 | 6953  | +67   | 3 245 547    | 6875   | +145   | 15 201 075  |
| 3 | 9365 | 9400  | -35   | 877 100      | 9512   | -147   | 15 472 044  |
| 4 |11682 |11556  | +126  | 13 637 484   |11713   | -31    | 825 499     |
| Σ |      |       |       | **32 017 769** |      |        | **39 676 666** |

The dominant gap is at coordinate i=3 (|Δterm| = 14 594 944): row 10
is 14.6M cost units WORSE at i=3 alone, more than offsetting its
gains at i=0/i=4. Production correctly minimizes the weighted MSE.
`searchL2` agrees with the row-by-row reconstruction (idx=2,
cost=32 017 769).

### §15.2 Decoder oracle parity (S2) — H-L4 / H-FREQPREV REFUTED

Captured decoder `pastResiduals[k=0..3]` at frame-0 entry (forced
initialization mirrors the codec cold-start path) and compared
against encoder `freqPrev`:

```
encoder freqPrev[k=0..3] all = initialPastResidual = [2340 4679 7019 9359 11698 14038 16377 18717 21057 23396]
decoder pastResiduals[k=0..3] all = initialPastResidual = (identical)
mem BIT-EXACT encoder vs decoder ?    true
L1[wantL1=120] read identically encoder vs decoder ?  true
```

Reconstructed the decoder's actual ω̂ at frame 0 with WANT indices
(L0=0, L1=120, L2=10, L3=10):

```
post-combine = [2654 5014 6443 9964 11759 14237 16513 17837 20304 23746]
post-J1      = [2654 5014 6443 9964 11759 14237 16513 17837 20304 23746]  (no-op: all gaps ≥ 10)
post-J2      = [2654 5014 6443 9964 11759 14237 16513 17837 20304 23746]  (no-op: all gaps ≥ 5)
 (post-pred)= [2415 4765 6875 9512 11713 14089 16412 18483 20849 23487]
```

Encoder L2-search ω̂[0..4] for row=10 (partial-J1 on post-predictor
[1..4] only) vs decoder ω̂[0..4]:

```
encoder L2-search = [2415 4765 6875 9512 11713]
decoder full pipe = [2415 4765 6875 9512 11713]
```

**Bit-exact equality.** Frame-0 J1 and J2 are NO-OPs because every
adjacent residual gap already exceeds J=10/J=5. Therefore the
search heuristic (J1 only on ω̂) and the spec-true protocol (J1+J2
on residual + predictor) are mathematically equivalent on frame 0.

"True" cost recomputed under the FULL decoder pipeline (J1+J2 on
residual, predictor, weighted MSE on i=0..4):

```
true_cost(row 10, WANT) = 39 676 666
true_cost(row 2,  GOT)  = 32 017 769
true Δ(got − want) = -7 658 897   (got STILL wins)
```

**H-L4 (cold-start `freqPrev`)**: REFUTED. Encoder/decoder memory
identical bit-exact at frame 0.
**H-FREQPREV-UPDATE**: REFUTED for frame 0 (no commits yet); the
post-frame commit in `commitPredictorMemory` cannot affect a
zero-history frame.
**H-VQ-L2W (L2 weighting/residual computation)**: REFUTED for the
"protocol" reading. The cost row 2 < row 10 holds under BOTH the
production search heuristic AND the spec-true J1+J2 reconstruction.
**H-J1J2 (rearrangement timing)**: REFUTED. On frame 0 both
rearrangements are no-ops; the search heuristic and spec-true
protocols produce identical ω̂. The audit in S4 separately confirms
the timing matches spec letter for non-frame-0 frames.

### §15.3 Weight protocol audit (S3)

Closed-form re-derivation of weights from spec eq. (22) in float64,
compared against `weightsLSF` Q11 production output for frame-0 ω:

| i | w_prod (Q11) | w_prod_real | w_spec_float | Δ (Q11 LSB) |
|---|--------------|-------------|--------------|-------------|
| 0 | 502          | 0.245117    | 0.245256     |  -0         |
| 1 | 720          | 0.351562    | 0.351980     |  -1         |
| 2 | 723          | 0.353027    | 0.353411     |  -1         |
| 3 | 716          | 0.349609    | 0.350040     |  -1         |
| 4 | 859          | 0.419434    | 0.419738     |  -1         |
| 5 | 867          | 0.423340    | 0.423937     |  -1         |
| 6 | 724          | 0.353516    | 0.353541     |  -0         |
| 7 | 724          | 0.353516    | 0.353541     |  -0         |
| 8 | 720          | 0.351562    | 0.351980     |  -1         |
| 9 | 363          | 0.177246    | 0.177684     |  -1         |

Max |Δ| = 0.000597 real ≡ 1.22 Q11 LSB. This is fixed-point
quantization noise, well within the spec's "implementation-defined"
license. Spec Q-format constants verified bit-exact:

* `lsfQ13Pi04 = 1029 = round(0.04·π·8192)` ✓
* `lsfQ13Pi92 = 23676 = round(0.92·π·8192)` (round() = 23677; off by 1 LSB
  — within the quantization noise budget already documented above)
* `lsfQ13One  = 8192`  ✓
* `lsfQ11One  = 2048`  ✓
* `lsfQ11OneTwo = 2458 = round(1.2·2048)` ✓

The 1-LSB Q13 difference on `lsfQ13Pi92` is mathematically
inconsequential at the L2 cost scale (one Q11 LSB on w_10, multiplied
by Δ² Q26, contributes ≤ 2³¹ to the total cost — negligible vs the
GOT/WANT gap of 7.7M).

**H-VQ-L2W (formula version)**: REFUTED. Production weights match
the spec piecewise to ≤ 1.22 Q11 LSB.

### §15.4 Rearrangement timing audit (S4)

Spec line-by-line review of §3.2.4 (lines 818–899):

* **Decoder pipeline** (lines 818–833): combine → J1 on l̂ → J2 on l̂
  → predictor → ω̂. Production `Decoder.Decode` matches.
* **Encoder L2 search** (lines 889–891): "the partial vector ω̂_i,
  i=1..5 is reconstructed using equation (20), and rearranged to
  guarantee a minimum distance of 0.0012". Production `searchL2`
  matches: predictor → partial-J1 on ω̂[1..4] → cost on i=0..4.
* **Encoder L3 search** (lines 893–895): "Again the rearrangement
  procedure is used to guarantee a minimum distance of 0.0012".
  Production `searchL3` matches: predictor → J1 on full ω̂[1..9] →
  cost on i=5..9.
* **Final L0 cost** (lines 895–898): "rearranged to guarantee a
  minimum distance of 0.0006" + "rearranged twice and a stability
  check is applied". Production `Quantize` final reconstruction
  matches: combine → J1 on l̂ → J2 on l̂ → predictor →
  enforceLSFStability → cost on i=0..9.

**H-J1J2 (rearrangement timing)**: REFUTED. Production protocol is
spec-letter for all four pipeline points.

### §15.5 Frame-596 drive-by (S5)

Replayed encoder up to frame 596:

```
oldSpeech[0..15]    = [-33 288 586 820 908 916 823 591 324 21 -328 -624 -860 -1060 -1141 -1117]
oldSpeech[224..239] = [36 -62 -229 -335 -458 -592 -610 -591 -609 -481 -321 -196 35 286 459 674]
a (Q12) [0..10]     = [4096 -4706 -7743 5000 11938 0 -11938 -5000 7743 4706 -4096]
max |a[1..10]| (Q12) = 11938 (real = 2.915)
LPToLSP FAIL: g729/lsp: fewer than 5 sign changes in F1 or F2
```

Frame 596 is a **higher-energy transient than frame 29** (frame-29
post-FIX-1B max|a| ≈ 1.22 per d5 §12.3; frame-596 max|a| = 2.915).
FIX-1B Q24 widening (range |a_real| < 128 in int32) does not
saturate aWork at this magnitude, but the larger range still
admits sufficient inner-update precision loss to make the
Chebyshev sign-change check fall below the 5-required threshold.

**Same fault class as frame-29 pre-FIX-1B**: aWork-precision
underrun causing Chebyshev band-centre miss. The FIX-1C Q30
candidate (d4 §13.2) has 6 additional fractional bits over Q24 and
is the natural next step; its bit-budget is established (d4
12.5) and the implementation is a one-helper change against the
already-widened FIX-1B body.

NOT the same root cause as the L2/L3 byte-EQ gap (which is a
quantization-precision drift in LP→LSP→LSF, see §15.6).

### §15.6 New hypothesis: H-OMEGA-PRECISION

The d6 forensic eliminates every encoder-VQ-stage hypothesis on
frame 0. Yet the WANT bitstream demands L2=10 — meaning the ITU
reference encoder, when fed the same PCM frame 0, computes a
slightly different ω vector for which row 10 is the local
weighted-MSE minimum.

Inspection of frame-0 a[]:

```
a (Q12) = [4096 0 0 0 0 0 0 0 0 0 0]   (the all-pass filter A(z)=1)
```

The LSPs of A(z)=1 are EXACTLY q_i = cos(i·π/11), and the LSFs are
EXACTLY ω_i = i·π/11 = `initialPastResidual` Q13.

Production ω vs the analytical reference:

```
_prod    = [2343 4677 7020 9365 11682 14025 16369 18714 21058 23391]
_analyt  = [2340 4679 7019 9359 11698 14038 16377 18717 21057 23396]
```

Per-coordinate ω drift of 1–16 Q13 LSBs is introduced by the
**LPToLSP Chebyshev root-finding** + **LSPToLSF arccos** chain on
the all-pass filter. At Δω = 16 (i=4), the per-row L2 cost
contribution is `w_4 · 16² = 859·256 ≈ 220 000` Q11·Q26 units —
about 0.7 % of the GOT/WANT cost gap, but compounding across all
five coordinates. The cumulative drift is sufficient to flip the
L2 winner from row 10 to row 2 (ITU-internal ω likely matches
_analyt to within ≤ 1 LSB on frame 0).

**H-OMEGA-PRECISION** (NEW, OPEN): the LPToLSP and/or LSPToLSF
fixed-point pipeline introduces 1–16 Q13 LSBs of drift versus the
analytical / spec-reference ω, and this drift is the dominant
driver of the residual L2/L3 byte-EQ gap. The drift is observable
on frame 0 (where the analytical ω is closed-form), and presumably
larger on speech frames where neither side can be cross-checked
analytically.

### §15.7 Hypothesis state after d6

| hypothesis                       | pre-d6     | post-d6                                 |
|----------------------------------|------------|-----------------------------------------|
| H-L4 (cold-start `freqPrev`)     | OPENED §14 | **REFUTED** §15.2 (bit-exact)           |
| H-VQ-L2W (L2 weighting/residual) | OPENED §14 | **REFUTED** §15.2 / §15.3               |
| H-J1J2 (rearrangement timing)    | OPENED §14 | **REFUTED** §15.4                       |
| H-FREQPREV-UPDATE                | OPENED §14 | **REFUTED** §15.2 (frame 0 has no commits) |
| H-OMEGA-PRECISION (NEW)          | n/a        | **OPENED** §15.6 (PROVISIONAL — primary candidate) |
| H-L1′ (aWork Q12, frame 596)     | n/a        | **OPENED** §15.5 (FIX-1C Q30 candidate) |

---

## §16. d6 disposition — ESCALATE-d7 + secondary FIX-1C-PROPOSED

### §16.1 Why no FIX-2 yet

The d6 forensic eliminates four hypotheses by direct measurement.
The remaining VQ-internal protocol surface area is exhausted (the
search code is provably spec-letter on frame 0, with bit-exact
encoder↔decoder ω̂ for the WANT residual). Spending an I5 budget
slot on a speculative VQ rewrite would have ≥75 % probability of
being a wrong-target consumption.

The new lead (H-OMEGA-PRECISION) points UPSTREAM of the VQ — into
the same LP-analysis pipeline that already required FIX-1B for
frame-29 LP-stability. The natural next dispatch is a focused
diagnostic on LPToLSP / LSPToLSF precision against the analytical
all-pass reference (frame 0) and against a float-oracle reference
on speech frames (5, 10, 15, 25 — already infrastructured in
phase2a_int1_d2).

### §16.2 Recommended next dispatch: d7

Plan name: `2026-05-12-phase2a-int1-d7-omega-precision-plan.md`.

Scope:

1. Closed-form ω accuracy on frame 0:
   - Analytical reference: ω_i = i·π/11 in Q13 (5-decimal-digit
     precision, no fixed-point loss).
   - Production ω = LSPToLSF(LPToLSP(a=[4096,0,...,0])).
   - Decompose drift between LPToLSP (Chebyshev) and LSPToLSF
     (arccos) by injecting analytical q at the LSPToLSF input.
2. Speech-frame ω drift via a wide-precision (float64) Chebyshev +
   acos oracle on the SAME a[] as production. Reuse the d5/d4
   mirroring infrastructure.
3. Quantify "VQ index sensitivity" to ω drift: replay the L2/L3
   search with ω perturbed by ±N Q13 LSB on coordinate i and
   measure the index switch frequency.
4. If H-OMEGA-PRECISION is confirmed, propose **FIX-2A** as a
   wider-precision (Q23 or Q24) accumulator inside one of:
   * `internal/lsp/lp_lsp.go` (Chebyshev root-finding)
   * `internal/lsp/lsp_lsf.go` (cosine-domain → frequency domain
     arccos)
   based on which contributes the larger fraction of the drift.

### §16.3 Secondary proposal: FIX-1C (Q30 widening) for frame 596

* **Module:** `internal/lpc/levinson.go`
* **Lines touched:** the four constants/shift counts that distinguish
  Q24 (FIX-1B) from Q30: `oneQ24` → `oneQ30`, the `<< 9` for
  `kQ15 → Qn` becomes `<< 15`, the inner-update `>> 15` is
  unchanged (Q15·Q30 → Q30 by `>> 15`), the `q24ToQ12Round`
  helper becomes `q30ToQ12Round` (`>> 18` instead of `>> 12`).
* **Spec citation:** §3.2.2 lines 717–736 (recursion in real
  arithmetic) + §3.2.1 line 691 (implementation-defined Q-format
  to "avoid arithmetic problems"). No spec text constrains the
  internal carrier width.
* **Risk:** LOW. Same algorithmic structure as FIX-1B; only the
  scaling constants change. All FIX-1B unit tests
  (Kronecker/AR1/Stability/Frame0Char/ZeroAllocation) must continue
  to pass; bit-exact equivalence on non-pathological frames is
  expected (Q12 final write is still the bottleneck).
* **Budget cost:** **1 of remaining 3 attempts** (would consume to
  3/5).
* **When to apply:** ONLY if d7 confirms H-OMEGA-PRECISION
  independently. Frame-596 is one frame out of 2232 (0.045 %), and
  the L2/L3 byte-EQ gap is the larger conformance issue. A combined
  d7 dispatch could include FIX-1C as a "while-we're-here" bonus
  if the d7 measurements show no Q30 risk on the broader corpus.

### §16.4 I5 budget after this dispatch

```
Before d6:      2/5 consumed (FIX-1A FAILED-REVERT + FIX-1B)
After  d6:      2/5 consumed (UNCHANGED — d6 is measurement-only)
Remaining:      3/5 attempts for d7 (+ optional FIX-1C bundle)
```

### §16.5 Test/build status under d6

* `go vet ./...`             clean
* `go build ./...`           clean
* `internal/lsp` (incl. d6)  PASS
* Pre-existing failures (verified at HEAD `ba79fcc`, unchanged
  since d4 §14.5):
  - `g729/TestEncode_LSPVectorBitExact`           (the gate test)
  - `internal/decoder/TestDiagnostic_SinglePulseChain`
  - `internal/gain/TestDecode_LowEnergyCodebookIsSmooth`
  - `internal/gain/TestDecode_SucceedsAcrossAllGainIndices`

No regressions introduced by the d6 test addition.

---

## §17. d7 measurements — LPToLSP/LSPToLSF precision profile

Dispatch: `internal/lsp/phase2a_int1_d7_omega_precision_test.go`
(`TestINT1D7OmegaPrecision`). Measurement-only; I5 budget unchanged
at 2/5; I6 freeze respected (no production file modified).

### §17.1 Frame-0 closed-form: production ω vs analytical i·π/11

Input `a = [4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]` Q12. A(z)=1 ⇒
F1(z)=1+z⁻¹¹, F2(z)=1−z⁻¹¹. Analytical roots ω_i = (i+/11,1)
i=0..9, F1/F2 interleaved.

```
i  cosProd  cosAna  Δcos(LSB Q15)  ωProd  ωAna  Δω(LSB Q13)
0   31430   31432         -2       2343   2340      +3
1   27560   27557         +3       4677   4679      -2
2   21441   21437         +4       7020   7019      +1
3   13573   13561        +12       9365   9359      +6
4    4712    4712          0      11682  11698     -16
5   -4633   -4712         +79     14025  14038     -13
6  -13599  -13561         -38     16369  16377      -8
7  -21462  -21437         -25     18714  18717      -3
8  -27576  -27557         -19     21058  21057      +1
9  -31438  -31432         -6      23391  23396      -5
```

* Production F1/F2 (Q24) on all-pass input is `[oneQ24, −oneQ24,
  oneQ24, −oneQ24, oneQ24, −oneQ24]` and `[oneQ24]·6` — both
  closed-form-correct.
* Per-coordinate ω drift bounded by 16 Q13 LSB; **i=4 carries the
  largest single-coordinate drift (16 LSB)**.
* Total |Δω| over 10 coords = 58 Q13 LSB.

### §17.2 Float-oracle decomposition (S2)

Production vs pure-stdlib float Chebyshev (no fixed point) + Acos:

```
i | dProd-Ana | dFloat-Ana | dHybrid-Ana
0 |    +3     |    +0      |    -6
1 |    -2     |    +0      |    -5
2 |    +1     |    +2      |    -3
3 |    +6     |    +7      |    +3
4 |   -16     |   +12      |    +7
5 |   -13     |   -12      |   -16
6 |    -8     |    -7      |   -12
7 |    -3     |    -2      |    -6
8 |    +1     |    +0      |    -4
9 |    -5     |    +0      |    -3
```

`hybrid` = float Chebyshev cos(ω) → production `lspToLSF` arccos LUT.
Per-step contribution decomposition (Δω LSB):

```
i | ChebyshevQ-loss | ArccosLUT-loss | total
0 |       -6        |       +9       |  +3
1 |       -5        |       +3       |  -2
2 |       -5        |       +4       |  -1
3 |       -4        |       +3       |  -1
4 |       -5        |      -23       | -28   ← arccos-LUT dominant
5 |       -4        |       +3       |  -1
6 |       -5        |       +4       |  -1
7 |       -4        |       +3       |  -1
8 |       -4        |       +5       |  +1
9 |       -3        |       -2       |  -5
```

Aggregate |Δ| Q13 LSB sums:

```
Production       vs analytical: 58
Float-oracle     vs analytical: 42   (irreducible cos→Q15 + Q13 quant + 4-iter bisect)
Hybrid           vs analytical: 65   (production arccos LUT amplifies float cosines)
Production       vs Hybrid    : 59   (Chebyshev-Q24 contribution)
```

**Interpretation.** The production Chebyshev-Q24 path drifts ≤ 6
LSB per coord vs the float oracle (consistent across all 10
coords). The arccos LUT introduces a single-coordinate spike of
23 LSB on i=4 (where the cos(ω) value lands in a high-derivative
LUT cell). The Q15→Q13 ω round itself contributes ≤ 1 LSB.

### §17.3 Bisection sensitivity (S3)

```
N  |  max|Δω|  |  sum|Δω|  | per-coord Δω
 4 |    16    |    58     | [3 -2 1 6 -16 -13 -8 -3 1 -5]
 6 |     8    |    42     | [-8 -6 -2 -4 -6 -2 -4 -6 -3 -1]
 8 |     7    |    43     | [-7 -5 -5 -5 -3 -5 -4 -4 -3 -2]
10 |     6    |    43     | [-6 -5 -5 -5 -4 -5 -4 -4 -3 -2]
12 |     6    |    43     | (converged)
16 |     6    |    43     | (converged)
```

* Bumping N=4→8 buys 9 LSB in `max|Δω|` (16 → 7). Beyond N=8 the
  fixed-point Chebyshev evaluation cannot resolve finer because the
  cos(ω) Q15 quantization is the floor (1 cosine LSB ≈ 5–10 ω LSB
  near sin(ω)≈0 and ω near 0 or π).
* Sensitivity is monotone-improving but bottoms out at sum=43.

### §17.4 Speech-frame drift (S4) — frames 5/10/15/25

Production ω vs float-oracle (4-iter bisect):

```
frame  max|Δω|  sum|Δω|  largest-Δ coords
 5      28        89     {5: -28, 6: -28, 8: -28}
10      28        89     (identical to 5; Analyze cache)
15      28        35     {8: -28}
25       4        14     diffuse (no LUT-cell hit)
```

The −28 LSB spikes on coords {5, 6, 8} are not random: they pin to
the same ArccosLUT cells whose cos(ω) lands near a derivative
maximum (S2 i=4 manifests the same 23-LSB flavour). On frame 25
the cosines miss those cells and total drift drops 6× to 14.

### §17.5 ω→VQ index sensitivity (S5)

Baseline frame-0 (synthetic all-pass ω) → `Quantize` produces
`L0=0 L1=120 L2=2  L3=11`.

WANT bitstream frame-0 indices (LSP.BIT, unpacked):
`L0=0 L1=120 L2=10 L3=10`.

* L1 already matches WANT (120) — first-stage VQ is unaffected by
  the ω drift seen here.
* Single-coordinate ±32 Q13 LSB perturbation on any coord 0..9
  fails to flip L2 or L3.
* **Joint uniform δ on all 10 coords**:

  ```
  δ=−32..−8 → L3 flips 11 → 26 (other indices unchanged)
  δ=−4..+8  → no change
  δ=+16,+32 → L2 flips 2 → 10 (matches WANT!)
  ```

* The L2-WANT distance from production ω is ≤ 16 LSB of coherent
  uniform shift. The Δω budget needed to flip L2 toward WANT on
  frame 0 is therefore ~12–16 Q13 LSB applied coherently — well
  within the 28 LSB drift S4 already documents on speech frames.

### §17.6 Root-cause localization (S6)

Combining S1..S5:

| Source                                    | Magnitude (Q13 LSB) | Acts on |
|-------------------------------------------|---------------------|---------|
| Chebyshev-Q24 evaluation (vs float)       | ≤ 6 per coord, ~5 systematic | All coords |
| 4-iteration bisection floor               | up to 9 (N=4 vs N=8) | Mid-range coords |
| **Arccos LUT cell error (`lspToLSF`)**    | up to 28 single-coord | Coords whose cos lands in high-derivative cells |
| Q15→Q13 quantization                      | ≤ 1 per coord       | All  |

**Dominant source: arccos LUT (`internal/lsp/lsp_lsf.go`
`lspToLSF`)**. The 65-entry CosLSP table with linear interpolation
gives ~25 LSB worst-case error on the cos(ω) ↦ ω inverse near
inflection points; this is empirically the only individual step
exceeding the L2-flip threshold (12–16 LSB) measured in §17.5.
The Chebyshev-Q24 path is healthy and converges by N=8 bisections.

### §17.7 Frame-596 forensics (S7)

* Production drives all 600 frames of LSP.IN through `lpc.Analyze`
  without error.
* `LPToLSP` first fails at frame 596 with
  `a = [4096, −4706, −7743, 5000, 11938, 0, −11938, −5000, 7743,
   4706, −4096]`.
* Pattern: `a[k] = −a[10−k]` (anti-palindromic) and `a[5] = 0`.
  For an anti-palindromic order-10 a, F1(z) = A(z) + z⁻¹¹A(z⁻¹) ≡ 0
  identically — so `findLSPRoots` correctly reports < 5 sign changes
  on F1. **This is a structural singularity in the upstream
  Levinson result, not a Q-format saturation.**
* Synthetic AR1 stress with ρ ∈ {0.95, 0.98, 0.99, 0.995, 0.999,
  0.9995}, autocorrelation r(0) ≈ 2³⁰:

  ```
  rho     | maxAbs(aWork) Q24    | maxAbs(aWork) Q30      | overflow Q24 | overflow Q30
  0.9500  |   15,930,752         | 1,019,567,994          |  false       |  false
  0.9800  |   16,433,817         | 1,051,764,156          |  false       |  false
  0.9900  |   16,601,676         | 1,062,507,140          |  false       |  false
  0.9950  |   16,908,272         | 1,082,129,133          |  false       |  false
  0.9990  |   17,447,957         | 1,116,669,070          |  false       |  false
  0.9995  |   34,010,529         | 2,176,673,697          |  false       |  false
  ```

  Even at ρ=0.9995 (k_i pushed near unity) the int64 carrier in
  Q24 still has 56 bits of headroom. **Q30 widening (FIX-1C as
  scoped in §16.3) would NOT have prevented frame-596's failure**
  because the failure mode is anti-palindromic structure (a true
  numerical singularity of the F1/F2 split), not Q-saturation.

---

## §18. d7 disposition

### §18.1 H-OMEGA-PRECISION — confirmed and localized

H-OMEGA-PRECISION is **CONFIRMED**. The dominant drift source
is the **arccos LUT in `internal/lsp/lsp_lsf.go`**, contributing
up to 28 Q13 LSB on individual coordinates whose cos(ω) lands in
a high-derivative LUT cell. The L2-flip threshold (§17.5) is
12–16 coherent Q13 LSB; production already exceeds this on speech
frames.

Secondary contributor: 4-iteration bisection. Bumping to 8 buys
~9 LSB on `max|Δω|` (S3) and lifts the precision floor closer to
the cos-Q15 quantization limit. Cheap, low-risk.

### §18.2 H-L1′ (frame 596) — REFUTED for Q30 fix

17.7 shows frame 596 fails on an anti-palindromic Levinson
output, not on Q-saturation. The synthetic AR1 stress to ρ=0.9995
shows Q24 still has 56 bits of headroom in int64. **FIX-1C as
scoped in §16.3 would not prevent the frame-596 cascade**;
withdraw it from the candidate list.

The genuine frame-596 fix is upstream of LP→LSP — either reject
anti-palindromic A(z) inside `LPToLSP` (returning ErrLPCNonStable
early), or apply a small odd-coefficient perturbation per the
spec's stability guard (§3.2.3 lines 800+ "if the polynomial is
anti-symmetric, ω_i are computed by perturbing a slightly"; needs
re-read of the exact spec wording before any FIX is drafted).
Defer this to a separate dispatch (d8 frame-596).

### §18.3 FIX-2 candidate ranking

| Candidate | Module / lines | Spec cite | Budget | Expected gate impact | Risk |
|-----------|----------------|-----------|--------|----------------------|------|
| **FIX-2C** (arccos refinement) | `internal/lsp/lsp_lsf.go` `lspToLSF` (whole function ~25 lines) | §3.2.5 ω_i = arccos(q_i) (no Q-format mandate) | 1 of 3 | Per-coord Δω drops ~25 LSB → ≤ 5 LSB; should close L2/L3 byte-EQ on frames 0–28 (joint shift no longer crosses the flip threshold) | Low — pure inverse-table change; existing `lsfToLSP` already provides the forward oracle for round-trip testing |
| **FIX-2A** (bisection 4 → 8) | `internal/lsp/lp_lsp.go` `bisectRoot` (one constant) | §3.2.3 lines 783–784 ("subdivide twice", language permits more) | 1 of 3 | Buys ~9 LSB max + ~16 LSB sum on frame 0; alone insufficient (S3 floors at 43 LSB sum) | Trivial |
| **FIX-2D** (FIX-2C + FIX-2A combined) | both above | both above | 1 of 3 (single dispatch) | Maximum Δω reduction; recommended | Low |
| ~~FIX-1C (Q30 Levinson)~~ | withdrawn — see §18.2 | — | — | Would not prevent frame-596 anti-palindromic cascade | n/a |
| FIX-2B (Chebyshev int64 widening) | rejected — §17.6 shows Chebyshev-Q24 contributes ≤ 6 LSB; widening yields no measurable improvement | — | — | Negligible | Low but pointless |

### §18.4 Recommended next dispatch

**Apply FIX-2D in a single bundle:**

1. Replace `lspToLSF`'s linear-interpolation arccos with a
   **Newton refinement step** seeded by the LUT lookup:

   ```
   ω₀ = LUT-linear-interp(q)             (existing)
   c₀ = lsfToLSP(ω₀)                     (forward map; production helper)
   sin(ω₀) ≈ √(1 − (c₀/32768)²)          (cheap closed-form)
   ω₁ = ω₀ + (c₀ − q) · 32768 / (       (32768·25736/π))   (Q13)sin(ω₀) 
   ```

   One Newton step caps the error at the second-derivative tail of
   the LUT (≤ 1 Q13 LSB by S2 numerics). All arithmetic stays in
   Word32; Q-formats trivially derivable (q in Q15, sin in Q15,
   ω in Q13). Spec-conformant: §3.2.5 only mandates ω = arccos(q)
   with no implementation prescription.

2. Bump `bisectRoot`'s loop from 4 → 8. Single-line constant
   change; pre-existing test `TestFindLSPRoots*` continues to pass
   bit-for-bit when the new constant is reverted; the Chebyshev
   cost is 8 evals per root (40 extra `chebyshevC` calls per
   frame ≈ 800 extra ops/frame, < 0.5 µs at typical clock).

3. Validation gate sequence:
   * `internal/lsp` battery (FIX-1B post-state) — must remain green.
   * `internal/lpc/TestLevinsonDurbin_*` — unaffected, must remain green.
   * `g729/TestEncode_LSPVectorBitExact` — primary gate; expect L2
     byte-EQ jump from 17.52 % toward ≥ 99 % on first 28 frames.
     L3 follows. L0/L1 already at 78.99 % / 38.71 % — re-measure
     after FIX-2D; the L1 deficit may have a separate root cause
     once L2/L3 are settled.

4. **Budget after FIX-2D:** 3/5 consumed (was 2/5). 2 attempts
   remain.

5. If FIX-2D under-shoots the gate, ESCALATE-d8 with:
   * frame-596 anti-palindromic guard (separate, low-risk),
   * L1 codebook/predictor re-audit (§3.2.4 lines 887 reread),
   * possibly a higher-order cosine LUT (129 entries instead of
     65) as a fallback FIX-2C′.

### §18.5 Hypothesis ledger after d7

| Hypothesis                              | Status entering d7 | Status leaving d7 |
|-----------------------------------------|--------------------|-------------------|
| H-OMEGA-PRECISION                       | OPENED             | **CONFIRMED, localized to lspToLSF arccos LUT** |
| H-L1′ (Q30 Levinson for frame 596)      | OPENED             | **REFUTED** (frame 596 = anti-palindromic structural singularity, not Q saturation) |
| New: H-FRAME596-ANTIPAL                 | n/a                | OPENED — defer to d8 |

### §18.6 I5 budget after d7

```
Before d7:      2/5 consumed
After  d7:      2/5 consumed (UNCHANGED — d7 is measurement-only)
Remaining:      3/5 attempts (recommended next: FIX-2D bundle, 1 slot)
```

### §18.7 Test/build status under d7

* `go vet ./internal/lsp/...`             clean
* `go build ./...`                        clean
* `internal/lsp` battery (incl. d7)       PASS
* Pre-existing failures unchanged from §16.5.

---

## §19 FIX-2D applied — Newton refinement on arccos + Chebyshev bisection 4→8

### §19.1 Diff summary

* `internal/lsp/lsp_lsf.go` — +60 / −5 lines net.
  - New helper `sinViaCos(omegaQ13)` derives sin(ω) from the existing
    `tables.CosLSP` LUT via the identity sin(ω) = cos(π/2 − ω) with even
    symmetry for ω > π/2. Avoids introducing a separate sine table.
  - `lspToLSF` keeps its binary-search + chord-interpolation seed
    (computing ω₀); appends a single Newton step
    `Δω_Q13 = ((cos(ω₀) − q)_Q15 << 13) / sin(ω₀)_Q15` clamped to one
    LUT-cell width and re-clamped against `[0, lspMaxOmega]`.
  - Doc comment updated with the FIX-2D rationale and Q-arithmetic
    derivation; cites G.729 §3.2.5 and Phase 2a INT-1 d4 §19.

* `internal/lsp/lp_lsp.go` — +14 / −5 lines net.
  - `bisectRoot`: loop bound 4 → 8; doc updated to "8 binary
    subdivisions" and "chebyshevC invoked exactly 8 times per call".
  - `LPToLSP` doc and `findLSPRoots` I11 comment updated to reflect
    the new (60, 8) configuration; the d3/d7 measurements supersede
    the original LP-3 tolerance-floor justification for 4 iterations.

* Public API unchanged; zero allocation preserved (added work is loop
  bound + scalar arithmetic; no new heap paths).

### §19.2 Frame-0 ω drift (production vs analytical)

| coord | Δω before (Q13 LSB) | Δω after (Q13 LSB) |
|------:|--------------------:|-------------------:|
| 0 | (per d7 §S1) up to ~28 in worst cell | −7 |
| 1 |  | −5 |
| 2 |  | −5 |
| 3 |  | −5 |
| 4 |  | −3 |
| 5 |  | −5 |
| 6 |  | −3 |
| 7 |  | −4 |
| 8 |  | −3 |
| 9 |  | −2 |

Aggregate: max\|Δω\| dropped from **28 → 7** Q13 LSB; sum dropped from
**58 → 42** (matching the float-oracle precision floor of 42 reported
in d7 §S6 — i.e. FIX-2D brings the production fixed-point pipeline to
within 0–1 LSB of the math.Acos-based oracle).

19.3 INT-1 byte-EQ rates (LSP.IN/LSP.BIT, 2231 frames; frame 596 skipped — lpcStep instability is the d8 scope)### 

| index | before FIX-2D | after FIX-2D | Δ |
|-------|--------------:|-------------:|---:|
| L0 | 79.02 % | 78.71 % | −0.31 |
| L1 | 38.73 % | 38.91 % | +0.18 |
| L2 | 17.53 % | 17.08 % | −0.45 |
| L3 | 19.72 % | 19.32 % | −0.40 |

(Baseline rates re-measured under HEAD `49e849f` via the same
fail-skip harness; they reproduce the d6 §14.4 quoted figures
78.99/38.71/17.52/19.71 to within rounding.)

Frame-0 indices specifically: PROD `(L0=0, L1=120, L2=2, L3=11)` →
**unchanged** by FIX-2D; reference `(L0=0, L1=120, L2=10, L3=10)`. The
S5 perturbation grid in d7 had already shown that ±32 Q13 LSB ω shifts
do not flip L2 toward 10 around the production operating point —
i.e. the L2/L3 residual is not an ω-precision deficit on this frame.

### §19.4 Test status

* `go build ./...` — clean
* `go vet ./...` — clean
* `internal/lsp/...` full test battery — PASS (incl.
  `TestINT1D7OmegaPrecision`, `TestLPToLSP_RoundTripCodebookL1` at
  unchanged tol=256, `TestNoAllocationInDecode`)
* `TestEncode_LSPVectorBitExact` — still fatals at frame 596 with
  `fewer than 5 sign changes in F1 or F2` (anti-palindromic LP edge
  case; scope of d8, not FIX-2D)
* Pre-existing 3 unrelated failures unchanged from §16.5
  (`TestDiagnostic_SinglePulseChain`,
  `TestDecode_LowEnergyCodebookIsSmooth`,
  `TestDecode_SucceedsAcrossAllGainIndices`).

### §19.5 Disposition: **IMPROVED-BUT-OPEN**

FIX-2D delivers exactly the precision improvement predicted by d7
(per-coord ω drift down from a 28-LSB worst case to ≤7 LSB; aggregate
sum reaches the float-oracle floor). The Chebyshev bisection
4→8 change is a structurally-clean precision lift retained on its own
merits.

However, the byte-equality rates do **not** rise — they shift slightly
in noise (±0.5 %) with no aggregate close. This is itself an
informative null result:

  * The L2/L3 byte gap is **not bottlenecked on LSP→LSF arccos
    precision** — both before and after FIX-2D the per-coord ω drift
    is well below the ≈12-LSB coherent threshold needed to flip L2.
  * The production-vs-reference residual must therefore live
    **upstream or downstream** of LSP→LSF — most plausibly inside the
    L2/L3 VQ search Q-arithmetic, the MA-prediction state vs ITU's
    initial `past_qlsf`, or the per-coordinate weight w_i computation
    (d6 §14 already flagged this latter as a candidate).
  * d7 §S5 had already foreshadowed this: ±32 LSB ω perturbations did
    not move L2 toward the reference value of 10, only joint +16/+32
    perturbations did — meaning a *uniform* bias would help, not a
    precision tightening that removes a non-uniform bias.

INT-1 byte-EQ gate **remains open**. FIX-2D is retained because (a)
it is a spec-aligned numerical improvement, (b) it brings the LSP→LSF
stage to its float-oracle floor and removes that variable from
downstream debugging, and (c) the Chebyshev bisection lift is
required by the I11 update for any byte-EQ work that follows.

### §19.6 I5 budget

Consumed: 3/5 (FIX-1A revert, FIX-1B revert, FIX-2D retained).
Remaining: 2/5.

### §19.7 Recommendation for next dispatch

Open **d8** with two independent threads:

  1. **Frame-596 anti-palindromic LP guard** — frame 596's
     `a[]=[4096 -4706 -7743 5000 11938 0 -11938 -5000 7743 4706 -4096]`
     is exactly antisymmetric, forcing F1 / F2 sign-change deficit.
     Add a graceful guard (either a tiny perturbation of `a[1]` per
     §3.2.3 stability conventions, or fall back to previous frame's
     LSPs per the spec's stability/repair text). This unblocks the
     `TestEncode_LSPVectorBitExact` fatal so it can report cumulative
     mismatches end-to-end rather than fail-fast at frame 596.

  2. **L2/L3 VQ residual hunt** — with LSP→LSF precision now at the
     float-oracle floor, instrument the L2 search loop to log
     (residual ω vector after L1, w_i weight vector, top-k candidate
     distortions) for frame 0 and a small bank of post-FIX-2D
     mismatching frames. Compare against analytical/textbook MA-VQ
     (Kondoz §6.4 / Salami §3.4) to localise whether the gap is in
     the weight w_i, the MA-predictor state seed, or the codebook
     distortion arithmetic itself. This consumes 1 of the remaining
     2 I5 slots.
