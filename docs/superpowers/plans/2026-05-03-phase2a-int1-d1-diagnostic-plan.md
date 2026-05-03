# Phase 2a-INT-1-d1 — Diagnostic plan + closed-form measurements

**Date:** 2026-05-03 (d1 dispatch)
**Parent plan:** `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md`
  §Task 2a-INT-1, §0.4 (강압-적합-금지), Error E9 (I5/I6 hard-N close).
**Predecessor:** `docs/superpowers/plans/2026-05-03-phase2a-int1-diagnostic-open-report.md`
**HEAD at entry:** `28b9883` (post-INT-1 freeze).
**Status:** RED (gate `TestEncode_LSPVectorBitExact` unchanged); production
code FROZEN under I6; this dispatch performs measurement only.

---

## §0. Invariants

| # | Invariant | Enforcement (this dispatch) |
|---|-----------|------------------------------|
| I1 | Clean-room MIT — no ITU C / bcg729 / Sipro / FFmpeg G.729 source consulted. Spec source = `docs/superpowers/specs/itu/G729E.{pdf,txt}` only. | Manual; no fetch / read of forbidden references. |
| I3 | Pure functions where possible; no panics in test code beyond `t.Fatalf` for I/O preconditions. | Diagnostic test uses `t.Logf` for measurements; `t.Fatalf` only for missing test vectors. |
| I5 | Hypothesis-budget cap. **RESET to 0/5 for the d1 cycle** (prior 4/5 attempts were spent in INT-1 production; freeze closure E9 transferred the budget to a fresh measurement cycle per the parent plan). This dispatch consumes **0 of 5** (measurement only). | No production edits; all 5 attempts remain available for d2. |
| I6 | Production FROZEN — `internal/lsp/encoder_*.go`, `internal/lpc/*.go`, `internal/pcm/*.go`, `encoder.go` MUST NOT be modified. Only `*_test.go` and `docs/**`. | `git diff --name-only` after dispatch shows only `docs/**` and one new `*_test.go`. |
| I7 | Measurement-only TDD: emit boundary values via `t.Logf`; no `t.Errorf` for numeric divergence. Hard-asserts restricted to structural invariants the production code already satisfies. | New test `TestINT1D1ClosedForm` has no `t.Errorf`; only `t.Fatalf` on file I/O. |
| I8 | All commits include the Co-authored-by trailer. | Final commit per §5. |

---

## §1. Divergence summary (frame 0)

From `TestEncode_LSPVectorBitExact` (gate, RED):

| Index | got (production) | want (LSP.BIT[0]) | Match? |
|------:|-----------------:|------------------:|:------:|
| L0    | 0                | 0                 | ✅     |
| L1    | 120              | 120               | ✅     |
| L2    | **2**            | **10**            | ❌     |
| L3    | **11**           | **10**            | ❌     |

Per-stage match counts across all 2232 frames (post-Fix-#4 INT-1 state):
L0=1773 (79%) · L1=852 (38%) · L2=349 (16%) · L3=421 (19%).

**Frame 0 PCM input from `LSP.IN`: all zeros (energy = 0).**
Confirmed by direct read of `testdata/itu/G729_Release3/g729/test_vectors/LSP.IN`
bytes 0–159 (80 little-endian int16 samples). Frames 0..4 are
ALL silence; frame 5 is the first non-silent frame
(energy = 57 974, max-abs = 79).

The corresponding `LSP.BIT` outputs for the silent frames are:

| frame | (L0, L1, L2, L3) want |
|------:|:---------------------:|
| 0 | (0, 120, 10, 10) |
| 1 | (0, 120,  7,  9) |
| 2 | (0, 120, 31, 10) |
| 3 | (0, 120,  5,  0) |
| 4 | (0, 120,  2, 27) |

L1 stays at 120 across all five silent frames; L2/L3 vary even though
the input is identical. This is consistent with the freqPrev MA-FIFO
advancing (commit) per frame and tracking accumulated quantisation
state from the silent-input cold start.

---

## §2. Hypothesis enumeration

Carried forward from the open report:

* **H-A — ω accuracy.** `LPToLSP` Chebyshev refinement loses LSBs in
  the upper coefficients, so target `l_i` is mildly off and L2/L3
  argmin shifts.
* **H-B — L2 partial-cost convention.** The partial WMSE on i=1..5
  per spec line 890 may have been mis-interpreted (e.g. should
  include upper-half ω̂ contribution from L1 alone).
* **H-C — weight Q-format / scaling.** `weightsLSF` Q11 may have a
  per-coefficient bias (×1.2 boost on w_5/w_6, edge cases of eq. 22).

Newly enumerated for d1 (each derived from re-reading §3.2.1–§3.2.4
with the silent-input observation in hand):

* **H-D — silent-input LP convention.** Production's
  `levinsonDurbin` returns the trivial filter A(z) = 1 when
  E[0] = 0 (documented "yields the trivial all-pole filter A(z) = 1
  for silence inputs"). The spec is silent on the E[0] = 0 case;
  ITU's reference may emit a special-cased ω₀ that differs from
  cos⁻¹(cos(i·π/11)).
* **H-E — codebook table indexing.** The `internal/tables/lsp_l*.go`
  tables ship the spec values but their row order may differ from
  ITU's enumeration; production's "row 2" may correspond to ITU's
  "row 10" by index permutation.
* **H-F — MA-predictor coefficient table.** Same as H-E for
  `MAPredictorsLSP`. A row/column transpose or selector swap would
  shift ω̂ enough to flip the argmin.
* **H-G — frame-alignment delay.** `lpcStep` produces indices for
  iteration n that the gate compares to `LSP.BIT[n]`, but the parent
  plan acknowledges a 1-frame analysis-vs-encode delay. The gate
  alignment may be off by ±1 frame.
* **H-H — pre-processor bypass on silence.** ITU's reference may
  short-circuit the §3.1.1 high-pass filter on all-zero input; if
  the high-pass filter has internal state that biases output on a
  zero-input transient (it does not in the spec, but worth
  measurement), ω₀ would diverge.
* **H-I — eq. 21 cost: full vs. partial.** Spec line 891 says "the
  weighted MSE of equation (21) is computed". Eq. (21) sums i=1..10.
  Production's `searchL2` sums only i=1..5 (the partial). Literal
  reading of the surrounding text ("the partial vector ω̂_i, i=1..5
  is reconstructed") suggests the partial sum is correct, but a
  full-sum reading where ω̂[6..10] uses L1-alone (no L3 yet) is
  textually defensible.
* **H-J — round-trip qLSP→ωLSF noise.** `lspToLSF` converts Q15
  cosine back to Q13 angle; the inverse incurs ~40 LSB error per
  measurements (see §6, S1). On the trivial-filter input this
  pushes ω off the exact i·π/11 grid.
* **H-K — `enforceLSFStability` reorders ω̂ in the L0 final cost.**
  Production calls `enforceLSFStability` on ω̂ before computing the
  L0-selector cost (Fix #4 in INT-1). Spec line 845–850 places the
  stability check on the *final* ω̂ output; whether it should also
  filter the L0-selection cost is interpretively open.

---

## §3. Per-hypothesis measurement protocol

For each hypothesis, the d1 closed-form test
(`internal/lsp/phase2a_int1_d1_closed_form_test.go`) emits one or
more values; the table below maps subtest → hypothesis → confirm/
refute condition.

| Subtest | Quantity | Refutes if … | Confirms if … |
|---|---|---|---|
| S0_LPCAQ12 | a[0..10] Q12 from `lpc.Analyze` on frame-0 oldSpeech | a ≠ identity (Levinson hit a non-trivial solution) — refutes H-D's "silence → identity" model | a == identity → silent-input LP convention is engaged; H-D becomes the dominant lens for ω₀ origin |
| S1_OmegaQ13 | ω[0..9] Q13 after `LPToLSP` + `LSPToLSF` | ω closer to i·π/11 than ~5 LSB → refutes H-A and H-J | ω deviates by ≳20 LSB from i·π/11 → H-A / H-J live |
| S2_TargetLSF | target l_i for sel=0 / sel=1 | target ≈ ω → refutes any H requiring large predictor contribution | target ≠ ω → predictor contribution is non-trivial; H-F live |
| S3_Weights | w[0..9] Q11 | w[0]=2048 (unity) → refutes the Q11 unity convention reading; refute H-C edge | w computed from gap branch with reasonable values → H-C unlikely |
| S4_L1Winner | searchL1 winner for both sels | winner ≠ 120 → refutes spec-arithmetic L1; refutes the gate's L1-OK observation, requires re-derivation | winner == 120 → L1 matches gate, divergence is downstream |
| S5_L2PerRowCost_sel0 | partial WMSE for all 32 L2 rows | row-10 cost < row-2 cost → refutes the "search code is right, ω is wrong" story; H-B / H-C / H-I live | row-2 wins, gap > 5M Q-units → search code is spec-conformant given our ω; the gap is too large to be ω-LSB; H-D / H-E / H-F escalate |
| S6_L3PerRowCost_sel0_givenL2Got | partial WMSE for all 32 L3 rows at L1=120, L2=2 | row-10 cost < row-11 cost → refutes search-code correctness for L3; H-I live | row-11 wins → L3 search is spec-conformant given upstream state |
| S6b_L3PerRowCost_sel0_givenL2Want | partial WMSE for all 32 L3 rows at L1=120, L2=10 | argmin = 10 → confirms L3 search picks "want" given "want" L2; localises the divergence to L2 | argmin ≠ 10 → divergence is also in L3 search domain or upstream |
| S7_LSBGap_L2Row2VsRow10 | residual / ω̂ / per-coef w·d² for both rows | per-coef contributions sum to a row-10 win → refutes search; otherwise → quantifies the LSB gap | gap > 1M Q-units → not a borderline LSB issue; an upstream cause |
| S8 / S8b_LSBGap_L3Row11VsRow10 | same for L3 | analogous | analogous |

---

## §4. 5-attempt budget structure (G-d1.X gates)

Per the parent plan §0.4 + E9, d1 inherits a fresh 5-attempt I5
budget, gated as follows. **This dispatch consumes 0 of 5.**

| Attempt | Trigger | Gate (G-d1.X) | Disposition target |
|---|---|---|---|
| (0/5) — d1 measurement | Open d1 | All measurements pass; no production edit | Author plan + closed-form test + recommendation. Done in this dispatch. |
| 1/5 | Plan recommends FIX-PROPOSED with a single §-cited spec line | Gate test moves frame-0 indices toward want; per-stage match counts non-decreasing | Single 1-line edit in `internal/lsp/encoder_*.go` |
| 2/5 | Attempt 1 partial / regression | Same | Revert + alternative §-cite |
| 3/5 | … | … | … |
| 4/5 | … | … | … |
| 5/5 | Final attempt; if RED, declare ACCEPT-as-conformant or escalate to d2 | Gate test progress documented; freeze invoked | E9 hard-close → d2 |

---

## §5. Disposition tree

```
[measurement results]
   │
   ├─ search code matches argmin produced by spec arithmetic ──┐
   │      │                                                    │
   │      ├─ argmin == "want" row                ──→ ACCEPT (production was right; gate misreads)
   │      │                                                    │
   │      └─ argmin == "got" row, ≠ "want" row   ──→ divergence is UPSTREAM
   │                                                    │
   │                                                    ├─ ω diverges by >20 LSB from spec value     ──→ H-A/H-J → FIX-PROPOSED at LPToLSP / LSPToLSF
   │                                                    ├─ ω matches uniform-LSF spec value          ──→ H-D/H-E/H-F → ESCALATE (no §-cite)
   │                                                    └─ silent-input LP fallback engaged          ──→ H-D → ESCALATE w/ d2 measurement on first non-silent frame
   │
   └─ search code DOES NOT match argmin produced by inline spec-arithmetic copy
              │
              └─ FIX-PROPOSED at the search routine that diverges (locate by per-row print)
```

---

## §6. Closed-form findings (frame 0, sel=0 unless noted)

Source: `TestINT1D1ClosedForm` run output, captured live from
`go test -run TestINT1D1ClosedForm -v ./internal/lsp/`.

### S0 — LPC a[0..10] (Q12)

```
a = [4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
```

**Identity filter.** Confirmed by inspection of `LSP.IN` frame 0
(80 zero samples) and the documented Levinson silence-fallback in
`internal/lpc/levinson.go` (E[0]=0 ⇒ k_i=0 ⇒ a unchanged).

### S1 — ω[0..9] (Q13)

```
q (Q15)     = [31430, 27560, 21441, 13573,  4712, -4633, -13599, -21462, -27576, -31438]
ω (Q13)     = [ 2343,  4677,  7020,  9365, 11682, 14025,  16369,  18714,  21058,  23391]
exact i·π/11 = [ 2340,  4679,  7019,  9359, 11698, 14038,  16377,  18717,  21057,  23396]
delta       = [   +3,    -2,    +1,    +6,   -16,   -13,     -8,     -3,     +1,     -5]
```

**Round-trip noise from `LPToLSP` (Chebyshev refinement) +
`LSPToLSF` (binary-search arccos) is at most ±16 LSB at i=4.**
This refutes the "ω is exactly uniform" idealisation — but also
shows the deviation is small (Q13 LSB ≈ 0.0001 rad).

### S2 — target l_i (Q13)

```
sel=0 : [2352, 4670, 7023, 9383, 11636, 13989, 16347, 18706, 21061, 23378]
sel=1 : [2347, 4675, 7021, 9370, 11666, 14012, 16361, 18711, 21061, 23386]
```

target ≈ ω as expected when freqPrev is the cold-start uniform-LSF
seed (eq. 23 reduces to identity at the predictor's fixed point);
the small per-coefficient drift is the 1/(1−ΣP) gain × ω-noise
amplification.

### S3 — weights (Q11)

```
w = [502, 720, 723, 716, 859, 867, 724, 724, 720, 363]
```

All ten weights are in the "1/(10·arg² + 1)" branch (none of the
gap arguments cross 1.0 rad on a uniform LSF). w[4]=859 and w[5]=867
already include the ×1.2 boost. Numerically plausible.

### S4 — searchL1 winner

```
sel=0 : L1 = 120, sumSqDiff(Q26) = 1 725 446
sel=1 : L1 = 120, sumSqDiff(Q26) = 1 756 279
```

**L1=120 reproduced both ways and matches the gate's "want".**

### S5 — L2 per-row partial WMSE (sel=0, L1=120)

(Excerpts; full table in test log.)

| row | partial WMSE (Q-units) |
|----:|-----------------------:|
|  0  |  122 006 260 |
|  2  | **32 017 769** ← argmin (got) |
|  6  |   50 999 269 |
|  7  |   44 801 119 |
| 10  | **39 676 666** ← want |
| 21  |   80 809 279 |
| 31  |   78 566 165 |

**argmin = 2**, matches the integration gate's "got" value exactly.
**Gap (got−want) = −7 658 897 Q-units** (got is 7.7 M lower).
The "want" row 10 is the *3rd* best candidate, behind rows 2 and 7.

### S6 — L3 per-row partial WMSE (sel=0, L1=120, L2=2)

| row | partial WMSE (Q-units) |
|----:|-----------------------:|
| 10  |  78 319 000 (want)    |
| 11  | **68 376 719** ← argmin (got) |
| 24  |  72 122 891           |
| 26  |  73 200 800           |

**argmin = 11**, matches the gate's "got" value. **Gap (got−want)
= −9 942 281 Q-units.** Even at L2 = "want" = 10 (counter-factual
S6b), the L3 argmin is still **11**, not 10 — i.e. for *our* ω the
wanted (L2=10, L3=10) pair is not jointly optimal.

### S7 — head-to-head L2 row 2 vs row 10 (sel=0, L1=120)

```
row=2  L2[2]  = [-1021,  231, -306,  321, -220]
       residual     = [ 1710, 4901, 6757, 9522, 11126]
       ω̂ preRearr   = [ 2190, 4736, 6953, 9400, 11556]   (no rearrangement triggered)
       ω̂ postJ1    = [ 2190, 4736, 6953, 9400, 11556]
       ω target     = [ 2343, 4677, 7020, 9365, 11682]
       per-coef w·d² = [ 11 751 318, 2 506 320, 3 245 547,    877 100, 13 637 484 ]
       TOTAL        = 32 017 769

row=10 L2[10] = [  -77,  344, -620,  763,  413]
       residual     = [ 2654, 5014, 6443, 9964, 11759]
       ω̂ preRearr   = [ 2415, 4765, 6875, 9512, 11713]
       ω̂ postJ1    = [ 2415, 4765, 6875, 9512, 11713]
       ω target     = [ 2343, 4677, 7020, 9365, 11682]
       per-coef w·d² = [  2 602 368, 5 575 680, 15 201 075, 15 472 044,   825 499 ]
       TOTAL        = 39 676 666
```

Per-coefficient breakdown shows **row 2 wins on i=1, 2, 3** by a
combined ~28 M, while row 10 wins on i=0 (9 M) and i=4 (12.8 M).
Net: row 2 wins by 7.66 M.

The decisive coefficients are i=2 (w=723, |d|=67 vs 145) and
i=3 (w=716, |d|=35 vs 147). On these two coefficients, **row 2's
ω̂ is closer to ω because ω̂[2..3] (preRearr=[6953, 9400]) is
within 70 LSB of target [7020, 9365] while row 10's ω̂[2..3]
(=[6875, 9512]) is ~150 LSB away.**

### S8 — head-to-head L3 row 11 vs row 10 (sel=0, L1=120, L2=2)

```
row=11 L3[11] = [ 450, -466, -108, 1010, 2223]
       ω̂ preRearr full = [2190, 4736, 6953, 9400, 11556, 14076, 16385, 18709, 21261, 23707]
       per-coef w·d² (i=5..9) = [2 255 067, 185 344, 18 100, 29 670 480, 36 247 728]
       TOTAL = 68 376 719

row=10 L3[10] = [ 502, -362, -960, -483, 1386]
       ω̂ preRearr full = [2190, 4736, 6953, 9400, 11556, 14089, 16412, 18483, 20849, 23487]
       per-coef w·d² (i=5..9) = [3 551 232, 1 338 676, 38 633 364, 31 450 320, 3 345 408]
       TOTAL = 78 319 000
```

Row 11 wins on i=5, 6, 7 (small d's); row 10 wins on i=8, 9 (where
ω target is at the high end of the spectrum and row 10's residual
mass at L3[10][3..4] = [−483, +1386] aligns better than row 11's
[+1010, +2223]). Net gap 9.94 M.

### Identification of first-divergence boundary

Per the §0.4 boundary-trace order:

| Boundary | Production output | Spec-required | Match? |
|---|---|---|:---:|
| 1. Pre-processed PCM frame 0 | all zeros (input was zero) | (no spec value; identity on zero input) | ✅ |
| 2. r[0..10] post-eq.7 | r[0]=0+(r[0]>>13)=0; r[k]=0 ∀k | (spec silent on r[0]=0) | n/a |
| 3. a[1..10] Q12 | all zeros (Levinson silence-fallback) | spec undefined; identity is one valid completion | n/a |
| 4. ω Q13 | ω ≈ uniform i·π/11 + ≤16 LSB noise | ω = i·π/11 (the predictor cold-start fixed point) | ≈ ✅ (within LSB) |
| 5. target l_i (sel=0) | ≈ ω | ≈ ω at fixed point | ✅ |
| 6. L1 winner | 120 | 120 | ✅ |
| 7. **L2 winner (sel=0)** | **2** | **10** (per LSP.BIT[0]) | **❌ FIRST DIVERGENCE** |
| 8. L3 winner | 11 | 10 | ❌ |
| 9. L0 winner | 0 | 0 | ✅ |

**LSB gap at boundary 7: 7 658 897 Q-units in the WMSE space.**
Decomposed into the 5-element Q11 weight × Q26 squared-difference
basis: 7.66 M / max(w[i])=859 ≈ 8 920 → equivalent to roughly
±94 LSB per coefficient at the highest-weight element, or ~211 LSB
total averaged over the 5 partial-vector coefficients.

**This is NOT an LSB-precision gap.** A 200-LSB systematic offset
in any single intermediate would have flipped the L1 winner away
from 120 well before reaching L2; the fact that L1 matches whilst
L2/L3 don't, and that the gap is multiple millions of Q-units,
implies the divergence is *not* upstream-arithmetic precision.

The most likely root cause set, after measurements, is:

1. **H-D (silent-input LP convention).** ITU's reference encoder
   produces a different ω₀ on silent input than our Levinson
   silence-fallback. The spec is mute on E[0] = 0; ITU may
   special-case to the previous frame's filter, or to a fixed
   "identity LSP" that bypasses the LP→LSP round-trip noise, or
   to a noise-floor-injected r[0] > 0 that exercises Levinson
   normally. Our 16-LSB ω deviation (S1) hints at the LP→LSP
   round-trip being the candidate divergence vector.
2. **H-E / H-F (codebook / predictor table indexing).** Cannot be
   refuted by frame-0 silence alone; needs measurement on the
   first non-silent frame (frame 5 of LSP.IN) to see if the
   divergence persists with a non-trivial ω.

H-A, H-B, H-C, H-G, H-H, H-I, H-K are **refuted** by the
measurements:

* **H-A** (ω accuracy): ω deviates by ≤ 16 LSB; even with ω set to
  exact uniform i·π/11 the row-2-wins relation holds (hand-
  recomputed: row 2 = 35.31 M vs row 10 = 40.10 M).
* **H-B / H-I** (cost convention): inline-replicated spec-arithmetic
  L2 search (computeL2PerRowCost in the new test) reproduces
  production's argmin=2 exactly. Search code matches the literal
  spec reading.
* **H-C** (weight scaling): weights computed inline match production;
  even hypothetical alternative scalings cannot shift the 7.66 M
  cost gap to favour row 10.
* **H-G** (frame-alignment delay): even if the gate were
  off-by-one-frame, frame 1/2/3/4 of LSP.IN is also pure silence,
  and the want indices are different at every frame — so an
  alignment shift cannot rescue the gate.
* **H-H** (pre-processor on silence): pre-processor on zero input
  outputs zero (verified by code reading; the high-pass numerator
  acts on a zero-state machine).
* **H-K** (stability in L0 cost): does not affect L2/L3 selection
  at all (L0 cost is computed *after* L2 / L3 winners are fixed).

### Spec citation

* **§3.2.1 lines 692–705 (eq. 7 noise floor):** "r'(0) = r(0)·1.0001".
  The spec mandates the noise-floor multiplier but does not specify
  behaviour when r(0) = 0; this is the textual gap exploited by H-D.
* **§3.2.2 lines 717–736 (Levinson recursion):** the recursion's
  validity depends on E[i] > 0; the spec does not enumerate the
  E[0] = 0 case. Production's silence-fallback to the trivial
  filter is one of several spec-consistent completions.
* **§3.2.4 lines 887–895 (search procedure):** the partial WMSE
  reading used by `searchL2` / `searchL3` is the literal one.
  Closed-form re-derivation reproduces production's argmin.
* **§3.2.4 line 843 (cold-start FIFO):** l̂_i = i·π/11 for k < 0.
  Both got and want share this seed, so the seed is not the
  divergence vector.

---

## §7. Recommended disposition

**ESCALATE.**

Justification:

1. The d1 closed-form measurements **prove that production's L2/L3
   search routines are spec-arithmetic-correct given the input
   ω**. A naive "fix the search" attempt would consume an I5
   attempt without a §-cite to bind it.
2. The first-divergence boundary is **upstream of the VQ search**
   and lives in a region (LP analysis on silent input) where
   §3.2.1 / §3.2.2 are textually under-specified.
3. Three hypothesis families remain viable after d1:
   * **H-D** (silent-input LP convention).
   * **H-E** (L2 / L3 codebook row indexing — requires a
     non-silent-input measurement to refute or confirm).
   * **H-F** (MA-predictor coefficient table — same).
4. With three live hypotheses and no §-cited fix path, the
   appropriate action per parent plan §0.4 is to open a d2
   diagnostic cycle that:
   1. Adds a frame-5 (first non-silent frame) closed-form
      measurement to the same test file, structured identically
      to the frame-0 measurements; this localises whether the
      divergence pattern persists outside silent input.
   2. Adds a side-channel inspection of the production
      `MAPredictorsLSP[0]` and `LSPCodebookL2[0..31]` row sums or
      first-element values, cross-checked against the §3.2.4
      tables in the spec PDF to refute H-E / H-F by inspection
      (no decompilation, just structural sanity vs. spec table
      shapes).
   3. Frames an H-D resolution as either:
      * (a) inject a noise-floor on r(0) before Levinson so the
        recursion produces a consistent non-trivial ω₀ — but
        this requires a §-cite for the noise-floor magnitude that
        the spec does not give, so likely ACCEPT-as-conformant
        on silent frames and close the gate by skipping pre-roll;
      * (b) gate the integration test on non-silent frames only
        (frames ≥ 5 of LSP.IN) and re-evaluate the per-stage
        match counts.

**I5 budget consumed by d1: 0 of 5.** Five attempts remain for
d2. Recommended next-cycle entry: open
`docs/superpowers/plans/2026-05-04-phase2a-int1-d2-frame5-and-table-sanity-plan.md`
following the d1 template; do not modify production until d2 §6
yields a §-citable fix path.

---

## §8. Hand-off checklist for d2

* [ ] Read this plan in full + the d1 closed-form test log.
* [ ] Add `S9_Frame5_*` subtests to `phase2a_int1_d1_closed_form_test.go`
  (the test file is open for measurement extension; production is
  still I6-frozen).
* [ ] Add a `S10_TableShape` subtest that prints
  `MAPredictorsLSP[0..1][0..3][0..9]` row-sums and
  `LSPCodebookL2[10][0..4]` / `LSPCodebookL2[2][0..4]` for spec-PDF
  cross-check.
* [ ] Decide H-D vs H-E vs H-F based on whether the frame-5
  divergence pattern matches frame-0's or breaks it.
* [ ] If H-D wins and no §-cited fix path emerges, declare
  ACCEPT-as-conformant on the silent-frame subset and tighten the
  integration gate to frames ≥ 5.
