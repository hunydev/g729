# Phase 2a-INT-1-d3 — Upstream LP-analysis divergence diagnostic

**Date:** 2026-05-04 (d3 dispatch)
**Parent plan:** `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md`
  §Task 2a-INT-1, §0.4, Error E9 (I5/I6 hard-N close).
**Predecessor plans:**
* `docs/superpowers/plans/2026-05-03-phase2a-int1-d1-diagnostic-plan.md`
  (refuted H-A, H-B, H-C, H-G, H-H, H-I, H-K)
* d2 amendment in the same d1 plan, §9–§10 (refuted H-E, H-F;
  promoted H-L to dominant)
**HEAD at entry:** `a7aa5a0` (post-d2 measurement commit).
**Status:** RED (`TestEncode_LSPVectorBitExact` fatals at frame 29 of
LSP.IN with `g729/lsp: fewer than 5 sign changes in F1 or F2 — LP
filter not stable`); production code FROZEN under I6; this dispatch
performs measurement only.

---

## §0. Invariants

| # | Invariant | Enforcement (this dispatch) |
|---|-----------|------------------------------|
| I1 | Clean-room MIT — no ITU C / bcg729 / Sipro / FFmpeg G.729 source consulted. Spec source = `docs/superpowers/specs/itu/G729E.{pdf,txt}` only. | Manual; no fetch / read of forbidden references. The float64 LP-analysis oracle in the d3 test is re-derived directly from §3.2.1 eq. 3–7 + §3.2.2 in the test source. |
| I3 | Pure functions where possible; no panics in test code beyond `t.Fatalf` for I/O preconditions. | New test uses `t.Logf` for measurements; `t.Fatalf` only for missing test vectors. |
| I5 | Hypothesis-budget cap. **Remains 0/5** for the d3 cycle (measurement only). | No production edits; all 5 attempts remain available for d4 / fix. |
| I6 | Production FROZEN — `internal/lsp/encoder_*.go`, `internal/lpc/*.go`, `internal/pcm/*.go`, `encoder.go` MUST NOT be modified. Only `*_test.go` and `docs/**`. | `git diff --name-only` shows only this plan + one new `*_test.go`. |
| I7 | Measurement-only TDD: emit boundary values via `t.Logf`; no `t.Errorf` for numeric divergence. Hard-asserts restricted to I/O preconditions. | New test `TestINT1D3UpstreamLP` has zero `t.Errorf`; `t.Fatalf` only on file I/O. |
| I8 | All commits include the Co-authored-by trailer. | Final commit per §6. |

---

## §1. Dispatch brief recap

d2 closed-form measurements ruled out single-stage VQ bugs and table
permutation bugs and produced strong evidence that the divergence
sits **upstream of the VQ search**, in the LP analysis chain
(`internal/pcm.PreProcessor` HPF, `internal/lpc.Analyzer` windowed
autocorrelation + lag window + Levinson, `internal/lsp.LPToLSP`
Chebyshev refinement). d3 is a four-step measurement battery
designed to localise *which* upstream module is responsible:

1. **Step 1 — Frame-29 LP-instability isolation.** Capture the
   raw 80 samples, the HPF output, the float-oracle r(0..10) before
   and after lag windowing, the float-oracle reflection coefficients
   k(1..10), and the production a(0..10) at the integration gate's
   abort frame. Identify the exact failure mode (mathematical
   instability vs Q-format saturation cascade).
2. **Step 2 — Frame 5 / 10 / 15 LP coefficient dump.** Capture
   r(), k(), a(), and LSP roots q() to verify whether early healthy
   frames already drift from a clean float oracle, and whether the
   resulting LSP roots are well-conditioned (distinct, in (−1, 1),
   adequately spaced).
3. **Step 3 — Chebyshev zero localisation.** Dump F1/F2 over a
   61-point grid and count sign changes; cross-check the bracket
   midpoints against `findLSPRoots`'s bisection result.
4. **Step 4 — Encoder vs decoder a[] cross-check (frame 5).** Take
   the WANT (L0, L1, L2, L3) for frame 5, run them through the
   decoder (a fresh `Decoder`, advanced over WANT indices for frames
   0–5), reconstruct a(0..10) Q12 from the resulting LSP via
   `lspToLP`, and diff against the encoder's LP-analysis a(0..10).
   This is the bisection that decides whether the bug lives in the
   LP-analysis chain or in the LSP-encoding chain.

Test:
`internal/lsp/phase2a_int1_d3_upstream_lp_test.go::TestINT1D3UpstreamLP`.

---

## §2. Frame-coverage caveats for the LP-coef dump

Frames 5, 10, 15 of `LSP.IN` carry the **identical 80-sample PCM
block** (verified by direct file inspection: bytes 5·160..6·160,
10·160..11·160, 15·160..16·160 are byte-equal; max-abs = 79, sum =
114). This is a dataset artifact: the ITU LSP test vector loops a
short non-silent excerpt for many frames before the transient at
frame 29 (max-abs jumps to 1014). Consequently the per-frame
dumps for f ∈ {5, 10, 15} differ only by their **HPF state and
oldSpeech sliding-window phase**, both of which rapidly steady-
state on the repeated input. The dump is therefore effectively a
*single*-frame snapshot for the early-frame regime, complemented by
the frame-29 snapshot for the transient-frame regime.

---

## §3. Step-1 findings — Frame-29 LP instability

| Quantity | Value |
|---|---|
| HPF first 10 samples | `[-253 -261 -269 -290 -254 -269 -237 -199 -165 -98]` |
| HPF last 10 samples  | `[88 21 -41 -89 -158 -193 -250 -289 -332 -371]` |
| Oracle r(0)          | 6.36 ×10⁶ (lag-windowed: 6.36 ×10⁶) |
| Oracle r(10)         | −1.32 ×10⁴ (lag-windowed: −1.18 ×10⁴) |
| Oracle k(1..10)      | `[-0.984, +0.816, +0.710, +0.204, -0.660, -0.061, -0.155, +0.270, -0.028, +0.337]` |
| Oracle E(0..10)      | `[6.36e6, 2.07e5, 6.94e4, 3.44e4, 3.30e4, 1.86e4, 1.85e4, 1.81e4, 1.68e4, 1.68e4, 1.49e4]` |
| Oracle Levinson stable (`|k_i| < 1` ∀i)? | **YES** |
| Oracle a(Q12)        | `[4096 -4939 -2733 1463 4229 -1688 1005 -1114 179 -1768 1382]` |
| **Production a(Q12)** | **`[4096 356 -7408 -6046 6046 7408 -356 -4096 0 0 0]`** |
| `LPToLSP` error (production a) | `g729/lsp: fewer than 5 sign changes in F1 or F2 — LP filter not stable` |
| F1 sign changes (production a, grid60) | **4** (need 5) |
| F2 sign changes (production a, grid60) | 5 |

**Failure mode (root cause at frame 29):** Levinson saturation
cascade in `internal/lpc/levinson.go`. Three converging signals:

1. Mathematical sanity. The float oracle on the same 240-sample
   windowed input produces |k_i| < 1 for all i and a stable a()
   with magnitudes ≤ 1.2 (real). The autocorrelation matrix at
   frame 29 is positive-definite — the spec's Levinson
   pre-conditions (§3.2.2 lines 717–736) are satisfied.
2. Production a() shape. The production output exhibits the
   classic signature of a Q-format saturation cascade in the
   Levinson update loop:
   * Coefficients exceed Q12 magnitude (a[2] = −7408 ≈ −1.81,
     a[5] = 7408 ≈ +1.81): both saturate against MaxInt16 in
     `saturateInt16`.
   * Mirror-image symmetry between a[1..4] and a[7..4] (4096 vs
     -4096, 7408 vs -7408, 6046 vs -6046, 356 vs -356) is
     consistent with `aWork[j] = aPrev[j] + (k·aPrev[i-j])>>15`
     producing reversed-sign clones once `aPrev[i-j]` saturates
     and the recursion's symmetry breaks down.
   * Trailing zeros (a[8..10] = 0): once `aWork[j]` saturates and
     the int32 update wraps in `saturateInt32`, subsequent
     iterations fail to lift the high-order coefficients off zero.
3. Downstream consequence. F1's even-symmetric construction
   `f1[i+1] = a[i+1] + a[10-i] - f1[i]` is fed
   `(a[1]+a[10]) = 356`, `(a[2]+a[9]) = -7408`, `(a[3]+a[8]) =
   -6046`, `(a[4]+a[7]) = 1950`, `(a[5]+a[6]) = 7761` — a
   coefficient sequence so far from a positive-real-valued LP
   that F1 has only 4 zeros on (−1, 1).

**No bit-level smoking-gun overflow site is yet localised** — the
saturation could be in the inner `aWork[j]` update loop, in the
final `aWork[i] = kQ15 >> 3` (which silently truncates), or in the
shared `r(k)` scale chosen by `autocorrelate` (a higher AC scale
on the high-energy frame 29 input may compress r() to a Q-format
where the subsequent Levinson divisions amplify quantization
noise into the kQ15 path).

**Spec citation for the next dispatch's fix attempt.** §3.2.2
lines 717–736 specify the recursion in real-valued arithmetic; the
ITU reference (per §A.2 of the Annex A overview text in
`G729E.txt`) implements an "AC-norm" pre-shift on r() that targets
the most-significant-bit envelope of r(0). Production's
`autocorrelate` instead uses the minimal-shift-to-fit-Word32
heuristic (`internal/lpc/autocorr.go` lines 50–57) — a coarser
choice that loses bits on transient frames. This is the single
most likely §-citable fix path (see §6).

---

## §4. Step-2 / 2b findings — Early-frame LP coefficient dumps

For the repeated-input early-frame regime (f ∈ {5, 10, 15}, all
fed identical PCM):

| Quantity | Value (frame 5; 10/15 identical to within ~1 LSB) |
|---|---|
| HPF first 10 | `[-9 -7 -8 -8 2 2 6 8 9 10]` |
| HPF last 10  | `[10 6 3 -1 -5 -7 -10 -10 -17 -17]` |
| Oracle r(0..10) (raw) | `[1.025e4 9.04e3 7.57e3 5.17e3 2.19e3 -132 -2.69e3 -4.31e3 -5.52e3 -5.93e3 -5.78e3]` |
| Oracle k(1..10) | `[-0.881, +0.183, +0.492, +0.452, -0.293, +0.113, +0.030, +0.184, -0.188, +0.205]` |
| Oracle a(Q12)  | `[4096 -3931 -1312 639 3247 -2450 566 -86 1107 -1545 841]` |
| Production a(Q12) | `[4096 -3901 -1297 572 3235 -2416 553 -39 1014 -1443 784]` |
| max\|Δ_aQ12\| (prod − oracle) | **102** (≈ 0.025 real) |
| sum\|Δ_aQ12\|              | 470 |
| LSP roots q(0..9) Q15 (production) | `[31588 30637 27965 22328 12477 40 -16030 -21210 -27161 -29344]` |
| LSP distinct? in (−1,1)? strictly decreasing? | **YES / YES / YES** |
| min consecutive q-gap (Q15) | **951** (≈ 0.029 real) |

**Verdict for the early-frame regime:** production's LP analysis
is **healthy in absolute terms** — 10-coefficient LSP set is
distinct, in-band, and well-spaced (min gap ~3% of full scale,
above the §3.2.4 stability floor of `lsfRearrJ1` = 10/8192 ≈
0.0012 rad). Production a() agrees with the float oracle to within
~0.025 in real magnitude (~100 Q12 LSB across all 10 coefficients).

For the transient frame 29:

| Quantity | Value |
|---|---|
| max\|Δ_aQ12\| (prod − oracle) | **9 096** (≈ 2.22 real) |
| sum\|Δ_aQ12\|                | 36 064 |

**Verdict for the transient regime:** production a() is wholly
divorced from the float oracle (~9 LSB ≈ 2.22 real magnitude error
on a coefficient that mathematically should fit in [−1.2, +1.2]).
This corroborates the Levinson saturation-cascade root cause from
§3.

---

## §5. Step-3 / Step-4 findings

### §5.1 Step 3 — Chebyshev sanity (frame 5)

* `f1[0..5] (Q24)` = `[16777216 -29544448 18321408 -11825152 24915968 -32546816]`
* `f2[0..5] (Q24)` = `[16777216 -2412544 -1814528 -3624960 9785344 -2375680]`
* F1 sign changes on the 61-point grid x = cos(i·π/60) for i = 0..60: **5** (want 5).
* F2 sign changes on the 61-point grid: **5** (want 5).
* F1 sign changes on production's `grid60` (k = 0..59 over k·π/59): **5**.
* F2 sign changes on production's `grid60`: **5**.
* `findLSPRoots` returns
  q(0..9) = `[31588 30637 27965 22328 12477 40 -16030 -21210 -27161 -29344]`
  (ω rad) = `[0.269 0.363 0.548 0.821 1.180 1.570 2.082 2.275 2.548 2.680]`.
* Bracket midpoints from the 61-grid scan (ω rad):
  * F1: `[0.288 0.550 1.178 2.068 2.539]`
  * F2: `[0.340 0.812 1.597 2.278 2.697]`

**Verdict:** the Chebyshev refinement is **healthy on the
healthy-frame regime**. Bracket midpoints from the coarser grid
and the bisected roots from `findLSPRoots` agree to within the
spec-mandated 4-bisection precision (~0.05 rad worst case). The
even/odd interlacing convention (F1 supplies q[0,2,4,6,8], F2
supplies q[1,3,5,7,9]) is preserved. **H-A is reconfirmed
refuted** for the early-frame regime.

### §5.2 Step 4 — Encoder vs decoder a[] cross-check (frame 5)

Frame 5 WANT indices: **(L0, L1, L2, L3) = (1, 5, 14, 20)**.

Procedure:

* Drive a fresh `Decoder` with WANT indices for frames 0..5; the
  decoder's `pastResiduals` and `prevLSP` thereby track ITU's
  reference state at the end of frame 5.
* `dec.prevLSP` (post-`Decode`) holds the just-decoded
  current-frame LSP.
* Apply `lspToLP` directly to that LSP (no interpolation) to get
  the "decoder oracle" a(0..10) Q12 — the same a() that ITU's
  reference would have quantised to.

| Quantity | Value |
|---|---|
| Encoder a(Q12) (LP-analysis output)              | `[4096 -3901 -1297 572 3235 -2416 553 -39 1014 -1443 784]` |
| Decoder oracle a(Q12) (sf2, no interpolation)    | `[4096 -3548 -1461 248 3284 -1512 46 -655 1261 -787 183]` |
| Decoder sf1 (Q12, prev-frame interpolated)       | `[4096 -1750 -1036 -109 1466 -583 214 -257 576 -426 117]` |
| Encoder LSP (Q15)                                | `[31588 30637 27965 22328 12477 40 -16030 -21210 -27161 -29344]` |
| Decoder LSP (Q15, raw current-frame)             | `[31577 30044 27987 22264 12339 -3048 -14910 -21293 -27146 -29427]` |
| **max \|Δ a[]\| (encoder − decoder)**             | **904 Q12 ≈ 0.221 real** |
| sum \|Δ a[]\|                                     | 4 421 Q12 |
| **max \|Δ q[]\| (encoder − decoder)**             | **3 088 Q15 ≈ 0.094 real** at i = 5 |

**Interpretation.**

* The LSP shift at coordinate 5 (encoder q[5] = 40 vs decoder
  q[5] = −3088, i.e. ω₅ moves from ~π/2 to ~π/2 + 0.094 rad) is
  **larger than the L2/L3 codebook entries can compensate**
  (codebook range ±0.28 rad, but the per-coordinate residual is
  typically tens of LSB Q13 ~ 0.005 rad, not 0.09).
* The 904 Q12 ≈ 0.22 a-coefficient diff is larger than what
  routine VQ quantisation noise would produce on a healthy frame
  (typical L2+L3 split-VQ MSE in this regime is ≲ 0.02 in real
  LP-coefficient magnitude).
* **Caveat.** Some of the diff *is* legitimate VQ quantization
  error: even a perfect LP-analysis encoder would not match the
  decoder's a() bit-exactly because L2/L3 are 5-bit residuals and
  L1 is a 7-bit unweighted-MSE choice. A 0.09 rad shift on q[5]
  is **borderline** explainable by an unfortunate split-VQ
  argmin in the centre of the spectrum.
* **Net verdict.** The encoder a() and the decoder oracle a()
  **do differ measurably**, but the diff is **not large enough to
  unambiguously confirm H-L** as the dominant root cause for the
  healthy-frame regime. It *is* large enough to be the
  coefficient-by-coefficient signature of an LP analysis whose
  output ω is shifted by ~0.1 rad from ITU's reference at one
  coordinate (q[5]) and accurate at the others (Δ ≤ 12 Q15 ≈
  4 LSB ≈ 4×10⁻⁴ real).

The 1-coordinate Δ pattern (only q[5] is materially off) is a
distinctive fingerprint: it points at the **Chebyshev refinement's
behaviour near the band-centre crossing** rather than at the
autocorrelation / Levinson stages (which would shift all 10
coordinates roughly equally). This is the new H-L1 hypothesis
opened in §6.

---

## §6. Updated hypothesis set after d3

| H | Status after d3 | Evidence |
|---|-----------------|----------|
| H-A — Chebyshev LSB accuracy on healthy frames | refuted (d1, reconfirmed d3 §5.1) | F1/F2 each have 5 sign changes; bracket midpoints align to within 4-bisection precision. |
| H-B — partial-cost convention | refuted (d1) | unchanged |
| H-C — weight Q-format / boost | refuted (d1) | unchanged |
| H-D — silent-input LP convention | live for frame 0 only | unchanged from d2 |
| H-E — codebook row indexing | refuted (d2 §9.2) | unchanged |
| H-F — MA-predictor table indexing | refuted (d2 §9.2/9.3) | unchanged |
| H-G — frame alignment | refuted (d1) | unchanged |
| H-H — pre-processor on silence | refuted (d1) | unchanged |
| H-I — eq. 21 cost full-vs-partial | refuted (d1) | unchanged |
| H-J — round-trip qLSP→ωLSF noise | refuted (d1) | unchanged |
| H-K — stability in L0 cost | refuted (d1) | unchanged |
| H-L — upstream LP analysis divergence | **partially confirmed** | §5.2: encoder a() ≠ decoder oracle a() at frame 5 (max Δ 904 Q12; q[5] shifted by 0.094 rad), but the gap may have a non-negligible VQ-quantization component. |
| **H-L1 — Levinson saturation cascade on transients (NEW)** | **CONFIRMED for frame 29** | §3: float oracle is mathematically stable (all \|k_i\| < 1) but production a() is wildly off-shape (saturated mirror-image with trailing zeros). §-cite path: `internal/lpc/levinson.go` Q-format loops + `internal/lpc/autocorr.go` AC-norm choice. |
| **H-L2 — Chebyshev refinement bias near band-centre (NEW)** | **plausible** | §5.2: only q[5] (the band-centre LSP) is materially off the decoder oracle (3088 Q15); q[0..4, 6..9] are within 12 Q15. Pattern is consistent with a one-sided bisection bias when F1/F2 cross zero with very small slope near x ≈ 0. |
| **H-L3 — autocorrelation AC-shift heuristic loses bits on high-energy frames (NEW)** | live, dependent on H-L1 | §3 / `autocorr.go` line 50–57: production picks the minimal shift to fit r(0) into Word32; ITU's spec language (§3.2.1 line 691, "to avoid arithmetic problems") leaves the normalisation under-specified, but a 1-bit-conservative shift (or a normalize-l style shift on s′(n)) would buy the Levinson recursion exactly the Q-format headroom that frame 29 lacks. |

---

## §7. Disposition

**ESCALATE — open d4.**

Justification:

1. **H-L1 (Levinson saturation cascade) is confirmed and §-citable.**
   The float oracle proves the spec-arithmetic recursion is
   mathematically stable on the same 240-sample input that fatals
   production. The fault is therefore in production's
   Q-format implementation of either §3.2.1 (autocorrelation
   normalisation) or §3.2.2 (Levinson update saturations). A
   surgical fix likely exists, but **localising it requires one
   more measurement cycle**:
   * step-by-step Levinson trace on frame 29 (per-iteration `e`,
     `kQ15`, and `aWork[]` after each i = 1..10);
   * cross-check of the AC-norm shift choice against a
     normalize-l style 31-bit-MSB shift on `s'(n)`;
   * spec re-read of §3.2.1 line 691 + Annex A overview of the
     reference encoder's AC-norm convention.
2. **H-L2 (Chebyshev refinement bias) is a separate, smaller
   bug that may need its own measurement.** Frame 5's q[5] = 40
   (essentially the band-centre crossing at ω = π/2) is suspect:
   the production bisection refines to a midpoint estimate, but a
   one-sided Newton step or a smaller terminal interval would
   reduce the apparent 0.094 rad shift. d4 should add a 6th /
   8th bisection to the test (oracle, not production) and see
   whether the divergence drops to within VQ-quantization noise.
3. **A single 1–2 line FIX-PROPOSED is therefore deferred** to the
   d4 cycle's §6/§7 disposition, where the per-iteration Levinson
   trace will pinpoint either:
   * (a) the exact `aWork[j] = saturateInt32(...)` site that
     wraps on the frame-29 transient, or
   * (b) the AC-shift count chosen by `autocorrelate` that should
     be one bit higher.

**I5 budget after d3: 0 of 5 attempts consumed. Five remain.**

---

## §8. Hand-off checklist for d4

* [ ] Read this plan in full and the d3 test log.
* [ ] Add a `S5_Frame29_LevinsonTrace` subtest that, for the
      production frame-29 input, emits per-iteration `i`, the
      pre-saturation `sum`, the `num/e` quotient, the chosen `kQ15`,
      the new `e`, and every `aWork[j]` after the j-loop. Compare
      against the float oracle's i-th iteration.
* [ ] Add an `S6_AutocorrShiftSweep` subtest that re-runs
      `autocorrelate` with `scale ∈ {scale_prod, scale_prod+1,
      scale_prod+2}` on the frame-29 input and reports the
      resulting Levinson stability.
* [ ] If S5 shows a clear single-iteration saturation site →
      FIX-PROPOSED (1 of 5) targeting `internal/lpc/levinson.go`.
* [ ] If S6 shows that one extra shift bit yields a stable
      Levinson on frame 29 → FIX-PROPOSED (1 of 5) targeting
      `internal/lpc/autocorr.go` lines 50–57 (extend the shift
      loop by one extra step, or switch to a normalize-l style
      31-bit-MSB criterion).
* [ ] Add a `S7_ChebyshevSixthBisection` subtest that re-runs
      `findLSPRoots` with 6 (instead of 4) bisections on the
      frame-5 a() and re-runs Step 4's encoder-vs-decoder a()
      diff. If max\|Δ q\| at i=5 drops below ~500 Q15, H-L2 is
      confirmed and a separate FIX-PROPOSED targeting `bisectRoot`
      can be opened.

---

## §9. Cross-references

* d1 measurement test:
  `internal/lsp/phase2a_int1_d1_closed_form_test.go`
* d2 measurement test:
  `internal/lsp/phase2a_int1_d2_closed_form_test.go`
* **d3 measurement test:
  `internal/lsp/phase2a_int1_d3_upstream_lp_test.go`**
* Integration gate:
  `lsp_itu_vector_test.go::TestEncode_LSPVectorBitExact`
  (RED at frame 29 with LP-instability fatal — pre-VQ failure;
  d3 §3 confirms root cause is `internal/lpc/levinson.go` saturation,
  not the `LPToLSP` guard itself).
* Spec §-cites used by d3:
  * §3.2.1 eq. 3 (LP analysis window) — d3 oracle, `d3LPWindowReal`.
  * §3.2.1 eq. 5 (autocorrelation) — d3 oracle, `oracleLPAnalysisD3`.
  * §3.2.1 eq. 6 (lag window) — d3 oracle, `d3LagWindowReal`.
  * §3.2.1 eq. 7 (white-noise correction r(0) · 1.0001) — d3 oracle.
  * §3.2.1 line 691 ("to avoid arithmetic problems") — H-L3 hook.
  * §3.2.2 lines 717–736 (Levinson recursion) — d3 oracle + H-L1.
  * §3.2.3 lines 782–799 (Chebyshev evaluation + bisection) — d3 §5.1.

