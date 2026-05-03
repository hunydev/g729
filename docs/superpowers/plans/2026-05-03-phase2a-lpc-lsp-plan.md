# Phase 2a — LPC analysis + LSP quantization sub-plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Date:** 2026-05-03
**Phase:** 2a (encoder front-end: windowed autocorrelation → Levinson-Durbin → LP→LSP → 18-bit LSP VQ)
**Parent plan:** `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §2
**Master closure ref:** `docs/superpowers/plans/2026-05-03-phase2-0-scaffold-report.md`

**Goal:** Replace the `errStub` body of `internal/lpc.Analyzer.Analyze` and add encoder-side LSP quantization in `internal/lsp` so that, for every frame in `testdata/itu/G729_Release3/g729/test_vectors/LSP.IN`, the four LSP-bit-field outputs `(L0, L1, L2, L3)` are byte-EQ to the corresponding G.192-packed fields in `LSP.BIT`. This is a *partial-frame* gate — pitch / ACELP / gain bit positions remain `ErrNotImplemented`-shaped, asserted at the bit-field granularity only.

**Architecture:** Mirror the decoder topology already in `internal/lsp` (split-VQ combine + MA predictor + stability rearrangement + LSP→LP) but in reverse: from `pcm.PreProcessor` output → windowed autocorrelation → Levinson-Durbin (`internal/lpc`) → LP→LSP via Chebyshev (`internal/lsp` new symbol) → adaptive-weighted two-stage VQ search → MA-predictor selector L0 (`internal/lsp` new symbol). Encoder owns its own `freqPrev[4][10]int16` MA-predictor memory; the decoder-side `pastResiduals[4][10]` field is **not** reused (encoder and decoder are independent state machines per design §5.4).

**Tech Stack:** Go 1.22+, zero runtime dependencies, no CGo, no SIMD, no assembly. All arithmetic via `internal/fixed` (G.191 STL-equivalent saturating Word16/Word32). MIT clean-room — only ITU-T G.729 (06/2012) base recommendation as published in `docs/superpowers/specs/itu/G729E.{pdf,txt}` §3.2.1–§3.2.4, plus the existing design spec `docs/superpowers/specs/2026-04-20-g729-codec-design.md` §3.2 / §5.1 / §5.3.

**Source spec citations (mandatory traceability):**
- `docs/superpowers/specs/itu/G729E.txt` §3.2 lines 630–946 (windowing, autocorrelation, Levinson, LP→LSP, LSP-VQ).
- `docs/superpowers/specs/itu/G729E.txt` §3.2.1 lines 661–705 (eq. 3 window, eq. 5 autocorr, eq. 6 lag window, eq. 7 noise floor).
- `docs/superpowers/specs/itu/G729E.txt` §3.2.2 lines 710–736 (Levinson-Durbin recursion).
- `docs/superpowers/specs/itu/G729E.txt` §3.2.3 lines 738–799 (LP→LSP, Chebyshev recursion eq. 15–17).
- `docs/superpowers/specs/itu/G729E.txt` §3.2.4 lines 800–899 (two-stage VQ, eq. 19–23, J=0.0012/0.0006 rearrangement).
- `docs/superpowers/specs/2026-04-20-g729-codec-design.md` §3.2 + §5.1 (encoder topology), §5.3 (state-table).

**Phase 2-0 inheritance:**
- Entry HEAD `379218f` (Phase 2-0 scaffold closure).
- Encoder skeleton `encoder.go` already has the §5.3 state fields including `oldSpeech[240]`, `lspOld[10]`, `lspOldQ[10]`, `freqPrev[4][10]`.
- `internal/lpc.Analyzer{}` placeholder + `errStub` already in tree.
- `internal/lsp` decoder side already implements the *forward* arithmetic that the encoder reuses unmodified: `combineResidual`, `lsfToLSP`, `lspToLP`, `interpolateLSP`, `enforceLSFStability`, `rearrangeAdjacent`, `insertionSort10`, `MAPredictorsLSP` table consumption pattern. See §1.2 below for the exact symbol list and reuse policy.
- `internal/tables` ships all four required codebooks (`LSPCodebookL1`, `LSPCodebookL2`, `LSPCodebookL3`, `MAPredictorsLSP`) plus `CosLSP` LUT — Phase 2a Task-0 codebook ingestion is **NOT** required (already present).

---

## 0. Entry preconditions and invariants

### 0.1 Working tree gate

- [ ] **Step 0.1.1: Confirm clean tree at HEAD `379218f`**

```bash
git rev-parse --short HEAD          # expect: 379218f
git status --short                  # expect: empty
```

If either check fails, do not enter Phase 2a. Resolve drift first.

- [ ] **Step 0.1.2: Confirm baseline test counts**

```bash
go test ./... 2>&1 | tee phase2a-baseline.log
grep -c "^--- FAIL" phase2a-baseline.log     # expect: 3
grep -c "^--- SKIP" phase2a-baseline.log     # expect: 3
```

The 3 FAILs MUST be exactly:
1. `TestDiagnostic_SinglePulseChain` (`internal/decoder`)
2. `TestDecode_LowEnergyCodebookIsSmooth` (`internal/gain`)
3. `TestDecode_SucceedsAcrossAllGainIndices` (`internal/gain`)

Pass count expected: 658. Any other FAIL is a regression and blocks entry — invoke E1.

> Note: the master plan §0.1.2 cited `a372de7` / 394 PASS at Phase 2 entry. By Phase 2-0 closure (`379218f`), the pass count rose to 658 (Phase 2-0 scaffold added the new sentinel-error / encoder-skeleton / internal-package-skeleton tests). The 3 FAIL / 3 SKIP set is unchanged from `a372de7`.

### 0.2 Invariants for the Phase 2a cycle

Inherits master-plan I1..I8 verbatim (`docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §0.2). Phase 2a-specific additions:

| # | Invariant | Enforcement |
|---|-----------|-------------|
| I9 | **LSP VQ codebook citation discipline.** Every reference to `LSPCodebookL{1,2,3}` and `MAPredictorsLSP` in code or test must trace to spec §3.2.4 (codebook 1) / §3.2.4 (split L2,L3) / §3.2.4 eq. 20 (MA predictor). The "merger doctrine" comment block already in `internal/tables/lsp_l*.go` (Phase 1a vintage) MUST be preserved unmodified — Phase 2a never re-asserts the bit-pattern values. | Per-task: every commit touching `internal/tables/lsp_*.go` is forbidden; if a value lookup is needed, edit only `internal/lsp/*.go`. |
| I10 | **Encoder-decoder state isolation.** Phase 2a-new encoder LSP state (`Encoder.freqPrev`, MA-residual history) is **owned by the root `Encoder`** struct and passed by pointer into `internal/lsp` encoder helpers. The `internal/lsp.Decoder.pastResiduals` field is NOT reused, NOT exported, and NOT mutated by encoder code. | Per-task: `grep "pastResiduals" internal/lsp/encoder*.go` must return zero hits. |
| I11 | **Chebyshev grid is exactly 60 evaluation points + 4 sub-divisions per sign change.** §3.2.3 line 782–784 fixes both numbers. No "tune until it matches the vector" — if the grid is wrong, the divergence trace per E2 must point to a different §3.2.3 clause. | Per-task LP→LSP: cite line 783 in the impl comment. |
| I12 | **VQ search is exhaustive** (128 × 32 × 32 × 2 = 262 144 candidates per frame). G.729 base spec §3.2.4 lines 887–897 mandates "the entry … that minimizes" at every stage; G.729A (Annex A §A.3.2.4) introduces a smart-search variant — **Phase 2a stays on the base spec** to keep the gate vector LSP.BIT meaningful. Annex A pruning is a Phase 2a-followup, not in scope. | Per-task VQ: assert candidate-count in test, not just final index. |

### 0.3 Escape hatches

Inherits master-plan E1..E5. Phase 2a-specific additions:

| Hatch | Trigger | Action |
|-------|---------|--------|
| E6 | `LSP.IN` or `LSP.BIT` missing / unreadable from `testdata/itu/G729_Release3/g729/test_vectors/`. | Stop, do **not** synthesize a substitute vector. Surface to user with file-presence diagnostic; Phase 2a cannot close without the gate vector. |
| E7 | Bit-field extraction from `LSP.BIT` reproducibly disagrees with the G.192 frame layout used by `internal/bitstream` for the decoder side (e.g., MSB/LSB ordering, frame boundary). | Treat as a Phase 2a Task-0 sub-cycle: write a measurement-only diagnostic test under `internal/bitstream/lsp_field_layout_diagnostic_test.go`, capture the layout in a `*-report.md`, then resume Task 1. |
| E8 | LP→LSP root-finder fails to converge for a measured `LSP.IN` frame (a sign change is not detected within the 60-point grid). §3.2.3 guarantees this cannot happen for stable A(z); failure implies Levinson produced an unstable filter, which is itself a defect. Halt to root-cause Levinson before continuing VQ. | Stop, write `*-report.md` with the offending frame index and `r[0..10]`/`a[1..10]` measurements. |
| E9 | A single VQ family hypothesis (e.g., "weights w_i are wrong", "rearrangement happens before MA prediction not after") consumes >5 production-fix attempts (I5). Default close: write a measurement-only `phase2a_vq_*_diagnostic_test.go`, freeze production via I6, escalate to user with hypothesis-family table. | I5 + I6 hard-N close. |

### 0.4 강압-적합 (forced-fit) avoidance — LPC/LSP block boundaries

When a Phase 2a measurement diverges from `LSP.BIT`:

1. **Boundary-trace order** — measure, in this order:
   1. Pre-processed input (`pcm.PreProcessor.Process` output) vs. an ITU-derived reference if one is buildable from `LSP.IN` + the §3.7 pre-processor recipe.
   2. Windowed autocorrelation r[0..10] (post-§3.2.1 eq. 7).
   3. Levinson-Durbin a[1..10] (Q12).
   4. Unquantized LSP q[1..10] (Q15) and LSF ω[1..10] (Q13).
   5. Per-stage VQ residuals: target `l_i` (eq. 23), L1 winner, L2 winner partial vector after rearrangement-J1, L3 winner full vector after rearrangement-J1, post-MA-predictor ω̂, post-J2 rearrangement, post-stability LSF.
   6. Final L0/L1/L2/L3 indices.
2. **First-divergence rule.** Identify the *first* boundary in the above list where measured ≠ expected. The fix MUST cite the §3.2.x clause governing that boundary's arithmetic.
3. **No magic-constant tuning.** Q-format shifts, saturation policy, and rounding direction must each cite spec §3.2.x or Annex A §A.3.2.x. Phase 1k F-* lesson is binding.
4. **Hypothesis budget.** Per I5, max 5 production-fix attempts on a single hypothesis family before E9 close.

---

## 1. Pre-flight inventory

### 1.1 ITU intermediate vector inventory (gate vectors)

Performed via `glob testdata/itu/G729_Release3/g729/test_vectors/LSP.*` plus `wc -c`:

| File | Size (B) | Frame size (B) | Frame count | Format |
|---|---:|---:|---:|---|
| `testdata/itu/G729_Release3/g729/test_vectors/LSP.IN`  | 357 120 | 160 (80 × 2) | 2232 | Intel little-endian int16 PCM, 80 samples / frame (10 ms @ 8 kHz) |
| `testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT` | 366 048 | 164          | 2232 | G.192 framed: 4-byte sync header + 80 × 2-byte bit-words (`0x007F` / `0x0081`) |
| `testdata/itu/G729_Release3/g729/test_vectors/LSP.PST` | 357 120 | 160 (80 × 2) | 2232 | Decoder PCM output reference (NOT used by Phase 2a; carried for §A.4 cross-checking only) |

`testdata/itu/G729_Release3/g729AnnexA/test_vectors/LSP.{IN,BIT,PST}` have identical sizes and are byte-equivalent for the LSP gate (Annex A diverges in pitch/ACELP search, not in LPC/LSP). Phase 2a primary gate uses the base-codec path; Annex A vector is a redundancy check.

`READMETV.txt` in the same directory documents `lsp` as "lsp quantization" — i.e., this vector pair was specifically constructed by ITU to exercise the §3.2.1–§3.2.4 chain. Match expectation is byte-exact.

**Bit-field layout in `LSP.BIT` (per §4 Table 8 / §A.4 frame format, decoder-side `internal/bitstream` already validated this layout for the decoder gate):**
- Bits 0–17 of the 80-bit payload are L0 (1) | L1 (7) | L2 (5) | L3 (5), in transmission order (MSB-first per §A.4 Table A.4).
- G.192 wire format: each bit is one 16-bit word (`0x0081` = 1, `0x007F` = 0); per frame: 4-byte header + 160 bytes payload = 164 B. `internal/bitstream.ReadG192Frame` already handles this.

**E6 verification step (run at Task 1 entry):**

```bash
test -s testdata/itu/G729_Release3/g729/test_vectors/LSP.IN  || echo E6
test -s testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT || echo E6
```

### 1.2 `internal/lsp` reusable symbols (decoder-side, already in tree at HEAD `379218f`)

| Symbol | File | Reuse status for encoder side |
|---|---|---|
| `Indices{L0,L1,L2,L3 uint8}` | `types.go` | **Reuse as-is** — same struct serves as encoder output (will be packed into bitstream by Phase 2c onwards). |
| `combineResidual(l1,l2,l3,out)` | `codebook.go` | **Reuse during VQ inner loops** — needed to reconstruct candidate ω̂ during L2/L3 search. |
| `lsfToLSP(omega) int16` | `lsf_lsp.go` | **Reuse** — Q13 ω → Q15 q; encoder uses for unquantized + quantized chain. |
| `lspToLP(lsp,a)` | `lsp_lp.go` | **Reuse** — needed downstream to build the dequantized a_q for Phase 2b/2c, but Phase 2a may not invoke it (gate is on indices, not a_q). Mark reused but optional. |
| `interpolateLSP(prev,curr,sf1,sf2)` | `interpolate.go` | **Reuse** in Phase 2a only if the gate trace requires q^(1)/q^(2); otherwise carry through to Phase 2b. |
| `enforceLSFStability(lsf)` | `stability.go` | **Reuse** — applied to the post-MA-predictor ω̂ per §3.2.4 lines 845–850. |
| `rearrangeAdjacent(lsf,J)` | `stability.go` | **Reuse** — applied with J1=10 (Q13) then J2=5 (Q13) per §3.2.4 lines 819–830. Encoder also calls this *during* VQ search on partial reconstructed vectors (§3.2.4 lines 887–895), so this function is hot in the inner loop. |
| `insertionSort10(a)` | `stability.go` | Internal helper; stays unexported and unchanged. |
| `(d *Decoder).applyPredictor` | `predictor.go` | **DO NOT call from encoder** (I10) — the algorithm body is the template for an encoder-side `applyPredictorWithMemory(memory, selector, residual, out)` free function that does NOT mutate memory (encoder VQ search needs a non-destructive evaluator inside the L1/L2/L3 loop, only committing the FIFO advance for the winning L0). |
| `lsfMinEdge`, `lsfMinGap`, `lsfMaxEdge`, `lsfRearrJ1`, `lsfRearrJ2` | `stability.go` | **Reuse** — Q13 stability constants, identical for encoder. |
| `initialPrevLSP[10]int16`, `initialPastResidual[10]int16` | `decoder.go` | **Reuse** — same codec-start values seed the encoder's `lspOld` and `freqPrev` (per design §5.3 + §3.2.4 line 843). |
| `lspStep`, internal cosine-LUT bookkeeping | `lsf_lsp.go` | Unexported, unchanged. |

**New symbols to add to `internal/lsp` (encoder side):**

| New symbol | Purpose | Spec § |
|---|---|---|
| `LPToLSP(a *[11]int16, q *[10]int16)` | Chebyshev sign-change root finder, producing Q15 LSP coefficients from Q12 LP coefficients | §3.2.3 lines 738–799 |
| `quantize(input *Quant) Indices` (or `Quantize`) | Top-level encoder-side LSP quantizer: takes unquantized ω, MA-history, returns winning indices and updates history | §3.2.4 lines 800–899 |
| `applyPredictorWithMemory(mem *[4][10]int16, selector uint8, residual, out *[10]int16)` | Non-destructive predictor evaluation (no FIFO advance) | §3.2.4 eq. 20 |
| `commitPredictorMemory(mem *[4][10]int16, residual *[10]int16)` | FIFO advance after L0 winner is chosen | §3.2.4 eq. 20 (post-decode) |
| `weightsLSF(omega *[10]int16, w *[10]int16)` | Adaptive weights w_i (eq. 22) including ×1.2 boost on w_5/w_6 | §3.2.4 lines 863–882 |

**New symbols to add to `internal/lpc` (replacing stubs):**

| New symbol | Purpose | Spec § |
|---|---|---|
| `lpAnalysisWindow [240]int16` (Q15 LUT) | Hamming + cosine window per eq. 3 | §3.2.1 eq. 3 (lines 663–669) |
| `lagWindow [10]int16` (Q15 LUT) | 60 Hz bandwidth expansion factors per eq. 6 | §3.2.1 eq. 6 (lines 692–699) |
| `(a *Analyzer) Analyze(speech *[240]int16, aCoeffs *[11]int16) error` | Replaces stub; produces Q12 a[0..10] with a[0]=4096 | §3.2.1–§3.2.2 |
| `windowSpeech(speech *[240]int16, windowed *[240]int16)` | s'(n) = w_lp(n) · s(n) | §3.2.1 eq. 4 |
| `autocorrelate(windowed *[240]int16, r *[11]int32)` | r[k] = Σ s'(n) s'(n−k), k=0..10, with overflow scaling | §3.2.1 eq. 5 |
| `applyNoiseFloorAndLagWindow(r *[11]int32)` | r'(0)=1.0001·r(0), r'(k)=w_lag(k)·r(k) | §3.2.1 eq. 7 |
| `levinsonDurbin(r *[11]int32, a *[11]int16) (rc int)` | Recursion of §3.2.2 eq. 8 → a[1..10] Q12 | §3.2.2 lines 717–736 |

`internal/lpc/types.go` `Analyzer{}` struct grows to hold zero state today (analysis is stateless given the speech buffer); the **windowed-speech 240-sample history is owned by `Encoder.oldSpeech`**, not by `Analyzer`. This preserves the §5.4 "blocks never own cross-frame state that the root coordinator must see" rule.

### 1.3 `internal/tables` LSP VQ codebook presence (Phase 2a Task-0 codebook ingestion: NOT NEEDED)

| Codebook | File | Present? | Format | Source citation |
|---|---|:---:|---|---|
| L1 (128 × 10, first stage) | `internal/tables/lsp_l1.go` | ✅ Yes | Q13 int16 | §3.2.4 line 807 (codebook 1 size) |
| L2 (32 × 5, second stage lower) | `internal/tables/lsp_l2.go` | ✅ Yes | Q13 int16 | §3.2.4 line 808 |
| L3 (32 × 5, second stage upper) | `internal/tables/lsp_l3.go` | ✅ Yes | Q13 int16 | §3.2.4 line 808 |
| MA predictors (2 × 4 × 10) | `internal/tables/lsp_ma.go` | ✅ Yes | Q15 int16 | §3.2.4 eq. 20 |
| Cosine LUT (65 entries) | `internal/tables/lsp_cos.go` | ✅ Yes | Q15 int16 | §3.2.3 eq. 16 evaluator support |

No table ingestion task. All table identity preserved by I9.

### 1.4 Pre-flight commit (no-op, this plan only)

This plan is documentation-only. The single Phase 2a commit produced by the *current* dispatch is the plan-creation commit at the end of this task list.

---

## 2. Task family W — windowing (§3.2.1 eq. 3–4)

### Task 2a-W-1: Hamming + cosine window LUT

**Files:** Create `internal/lpc/window.go`, `internal/lpc/window_test.go`.

- [x] **Step 1: Write failing test** asserting the LUT length, endpoint values (w[0] = 0.08 in Q15 ≈ 2621, w[199] = peak at the Hamming centre, w[200] = cos(0) ≈ 32767, w[239] ≈ cos(39·2π/159)), and monotonic-decay shape on `[200,239]`. Cross-check the analytic formula at 4 hand-computed sample indices (0, 100, 199, 200) against eq. 3 with absolute tolerance ±2 in Q15.
- [x] **Step 2: Run to verify FAIL** (`undefined: lpAnalysisWindow`).
- [x] **Step 3: Write minimal implementation** — hard-coded `[240]int16` LUT generated **off-line** by a Go `init`-time computation living **only in the test file** as the oracle, with the production LUT being the literal table. The literal-vs-oracle pattern matches Phase 1a `CosLSP` precedent; we only commit the literal values, with a doc comment showing the computation.
- [x] **Step 4: Run to verify PASS**.
- [x] **Step 5: Commit.**

**Spec cite:** §3.2.1 eq. 3 (lines 663–669):
- `w_lp(n) = 0.54 − 0.46·cos(2πn/399)` for `n ∈ [0,199]`,
- `w_lp(n) = cos(2π(n−200)/159)` for `n ∈ [200,239]`.

**Q-format:** Q15 (window values are in [0,1]). Saturated at +32767 for w(200) = cos(0) = 1.0.

**Expected vs measured gate:** none (LUT is intra-package; a measurement gate appears at Task 2a-INT-1).

**Commit message:**
```
feat(lpc): Phase 2a-W-1 add 240-sample Hamming+cosine window LUT

Implements §3.2.1 eq. 3 of ITU-T G.729 (06/2012) — half Hamming on
[0,199] and quarter cosine on [200,239] in Q15. LUT is a literal
[240]int16 with an oracle test cross-checking eq. 3 at endpoints.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-W-2: Apply window to 240-sample speech buffer

**Files:** Extend `internal/lpc/window.go`, add `internal/lpc/window_apply_test.go`.

- [x] **Step 1: Write failing test** with a synthetic input (DC = 1024) and assert the windowed output equals `1024 · w_lp(n) >> 15` for each n.
- [x] **Step 2: Run to verify FAIL** (`undefined: windowSpeech`).
- [x] **Step 3: Write minimal implementation:**
  ```go
  func windowSpeech(speech *[240]int16, windowed *[240]int16) {
      for n := 0; n < 240; n++ {
          windowed[n] = fixed.Mult(speech[n], lpAnalysisWindow[n])
      }
  }
  ```
- [x] **Step 4: Run to verify PASS**.
- [x] **Step 5: Commit.**

**Spec cite:** §3.2.1 eq. 4 (line 684).
**Q-format:** speech is Q0 int16 (post-pre-processor); `fixed.Mult` (Q0·Q15→Q0 with the implicit ×2 of fractional multiply already absorbed by `fixed.Mult` semantics defined in `internal/fixed`). Windowed output is Q0 int16.

**Commit message:**
```
feat(lpc): Phase 2a-W-2 windowSpeech multiplies speech by §3.2.1 LUT

windowSpeech applies the §3.2.1 eq. 4 product s'(n) = w_lp(n)·s(n)
sample-by-sample using the Q15 LUT from W-1. Zero-allocation, in-place
output via caller-owned [240]int16 buffer.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 3. Task family AC — autocorrelation + bandwidth expansion (§3.2.1 eq. 5–7)

### Task 2a-AC-1: Autocorrelation r[0..10] with overflow scaling [x]

**Files:** Create `internal/lpc/autocorr.go`, `internal/lpc/autocorr_test.go`.

- [x] **Step 1: Write failing test** with three inputs:
  1. All-zero windowed speech → r[0..10] = 0.
  2. Constant input s'(n) = 1024 → r[k] = (240−k)·1024² for k=0..10.
  3. Sine wave at 1 kHz, sample-perfect period — assert r[0] > 0 and r[8] (period boundary) ≥ 0.99·r[0] within Q-format slack.
- [x] **Step 2: Run to verify FAIL** (`undefined: autocorrelate`).
- [x] **Step 3: Write minimal implementation** using `fixed.LMac` accumulators (Word32 = sum of Q0·Q0 products). Apply overflow-recovery scaling: if any `|s'(n)|` would overflow the Word32 accumulator across 240 taps (worst-case 240 · 32767² ≈ 2.58·10¹¹, exceeds 2³¹−1), pre-shift the windowed buffer right by 1 bit (track scale factor for downstream r[k] interpretation). §3.2.1 says "to avoid arithmetic problems" — the spec leaves the exact normalization scheme to the implementation; document the choice in a comment and keep all r[] in **Word32 with the same shared scale**.
- [x] **Step 4: Run to verify PASS**.
- [x] **Step 5: Commit.**

**Spec cite:** §3.2.1 eq. 5 (lines 686–689). Overflow-recovery shift policy: cite "to avoid arithmetic problems" remark on line 691 + design-spec §5.4 on saturating-arithmetic discipline.

**Q-format:** input Q0 int16; r[k] Word32 in Q0 with shared right-shift scale (returned alongside or recorded in a sibling `int8`).

**Expected vs measured gate:** none (covered by Task 2a-INT-1 r[] dump assertion).

**Commit message:**
```
feat(lpc): Phase 2a-AC-1 autocorrelation r[0..10] with overflow scaling

Implements §3.2.1 eq. 5 — r(k) = Σ_{n=k..239} s'(n)·s'(n-k), k=0..10 —
with shared right-shift normalization to keep all 11 values in a
common Word32 representation without saturating the long accumulator.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-AC-2: Noise floor (r(0)·1.0001) and lag window (60 Hz expansion)

**Files:** Create `internal/lpc/lagwindow.go`, `internal/lpc/lagwindow_test.go`.

- [ ] **Step 1: Write failing test** asserting:
  1. `lagWindow[k]` LUT, k=0..9 (representing k=1..10 in eq. 6), matches `exp(−0.5·(2π·60·k/8000)²)` in Q15 within ±2.
  2. Applied to a flat r[k] = 1<<24, the result divides by exactly the LUT factor.
  3. r'(0) = r(0) + (r(0) >> 13) approximation of ×1.0001 (0.0001 in Q13 ≈ 1; equivalent to multiplying by 32771 / 32768 — pick the spec-faithful Q15 representation 32771 and document the round).
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** compute LUT off-line (oracle in test, literal values committed); apply via `fixed.Mult` on each r[k] (k=1..10) and the noise-floor scalar on r[0].
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.1 eq. 6 (lines 692–699), eq. 7 (lines 700–704).

**Q-format:** lagWindow Q15 int16; r[k] stays Word32 with the AC-1 shared scale.

**Commit message:**
```
feat(lpc): Phase 2a-AC-2 lag window + 1.0001 noise floor

Adds the §3.2.1 eq. 6 60 Hz bandwidth expansion LUT (Q15) and the
eq. 7 white-noise correction factor 1.0001 on r(0). Both ITU-T G.729
(06/2012) §3.2.1 verbatim; LUT precomputed via test-only oracle.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 4. Task family LD — Levinson-Durbin (§3.2.2)

### Task 2a-LD-1: Levinson-Durbin recursion → a[0..10] Q12

**Files:** Create `internal/lpc/levinson.go`, `internal/lpc/levinson_test.go`.

- [ ] **Step 1: Write failing test** with three inputs:
  1. r' = [1, 0, 0, ..., 0] → a = [1, 0, 0, ..., 0] (Q12: a[0] = 4096).
  2. r' from a known stable AR(1) process with pole = 0.5 → a[1] ≈ −2048 (Q12), a[2..10] ≈ 0.
  3. Frame-0 r' computed from `LSP.IN` first frame (after AC-1 + AC-2) → a[1..10] is captured as a "characterisation" assertion; the **values are pinned by direct §3.2.2 arithmetic, NOT by any external implementation**, and the test starts as a `t.Logf` capture. After Task 2a-INT-1 confirms downstream L1/L2/L3 gate-EQ on the same frame, this characterisation is promoted to `t.Errorf`.
- [ ] **Step 2: Run to verify FAIL** (`undefined: levinsonDurbin`).
- [ ] **Step 3: Write minimal implementation** following §3.2.2 verbatim:
  ```
  E[0] = r'(0)
  for i = 1..10:
      k_i = - (Σ_{j=0..i-1} a^{i-1}_j · r'(i-j)) / E^{i-1}
      a^{i}_i = k_i
      for j = 1..i-1: a^{i}_j = a^{i-1}_j + k_i · a^{i-1}_{i-j}
      E^{i} = (1 - k_i²) · E^{i-1}
  ```
  Q-format: r' Word32 in the AC shared scale; a in Q12; k_i (reflection coefficient) in Q15; E[i] in Word32 sharing the r' scale. Division uses `fixed.Div32`.
- [ ] **Step 4: Run to verify PASS** on the synthetic inputs; `t.Logf` the frame-0 capture.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.2 lines 710–736 verbatim.

**Q-format pinning rationale:** a[0]=4096 fixes Q12 for the 10th-order LP coefficients; this matches `internal/lsp/lsp_lp.go`'s consumer side (`lspToLP` returns `[11]int16` with `a[0] = 4096`), preserving cross-block Q-format continuity.

**Expected vs measured gate:** **deferred** to Task 2a-INT-1; the Levinson output is intermediate and not separately gated by any LSP.* file.

**Commit message:**
```
feat(lpc): Phase 2a-LD-1 Levinson-Durbin recursion a[1..10] in Q12

Solves §3.2.2 eq. 8 via the lines-717..736 recursion. Output
[11]int16 with a[0] = 4096 (Q12). Reflection coefficients k_i kept
in Q15; prediction error E[i] in the shared Word32 r'() scale.
Three synthetic-input tests + one frame-0 characterisation log.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 5. Task family LP — LP→LSP via Chebyshev (§3.2.3)

### Task 2a-LP-1: F1, F2 polynomial coefficients (§3.2.3 eq. 15)

**Files:** Create `internal/lsp/lp_lsp.go`, `internal/lsp/lp_lsp_test.go`.

- [ ] **Step 1: Write failing test** asserting eq. 15 recursion:
  - `f1(i+1) = a_{i+1} + a_{10-i} − f1(i)`, `f1(0)=1.0`,
  - `f2(i+1) = a_{i+1} − a_{10-i} + f2(i)`, `f2(0)=1.0`,
  with three hand-traced inputs (a=[1,0,…,0]; a=[1,−1,0,…,0]; the Levinson output of constant r').
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** in-place over `[6]int32` arrays for f1 and f2; promote a from Q12 to Q24 for the recursion to retain headroom (additions only — no products yet at this step).
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.3 eq. 15 (lines 779–782), F1/F2 definitions eq. 9–14 (lines 744–771).

**Commit message:**
```
feat(lsp): Phase 2a-LP-1 F1/F2 polynomial coefficients eq. 15

Computes the symmetric/antisymmetric sum-and-difference polynomials
F1(z), F2(z) coefficients f1(0..5), f2(0..5) from Q12 a[1..10] per
§3.2.3 eq. 9–15. Promoted to Q24 internally for headroom.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-LP-2: Chebyshev evaluator C(x) (§3.2.3 eq. 17)

**Files:** Extend `internal/lsp/lp_lsp.go`, add `internal/lsp/chebyshev_test.go`.

- [ ] **Step 1: Write failing test** with f = [1,0,0,0,0,0] (so C(x) = T_5(x) + 0.5 = cos(5·acos(x)) + 0.5) sampled at x = cos(0), cos(π/8), cos(π/4), cos(π/2), cos(π).
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation** of the back-recursion §3.2.3 lines 794–799:
  ```
  b[5] = 1; b[6] = 0
  for k = 4 down to 1: b[k] = 2x·b[k+1] − b[k+2] + f(5−k)
  C(x) = x·b[1] − b[2] + f(5)/2
  ```
  Q-format: x in Q15; f in Q24 (from LP-1); b in Q24; product 2x·b is Q15·Q24 → Word32 Q24 with explicit shifts to keep sums aligned. Output C(x) in Q24 Word32.
- [ ] **Step 4: Run to verify PASS** within ±2^14 absolute tolerance (Q24).
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.3 eq. 16–17 (lines 786–799).

**Commit message:**
```
feat(lsp): Phase 2a-LP-2 Chebyshev evaluator C(x) for §3.2.3

Implements the back-recursion of §3.2.3 lines 794–799 producing
C(x) = T_5(x) + f(1)T_4(x) + ... + f(5)/2 in Q24 from Q15 x and Q24
f-coefficients. Pure function, no allocation.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-LP-3: Sign-change root finder (60-point grid + 4 sub-divisions)

**Files:** Extend `internal/lsp/lp_lsp.go`, add `internal/lsp/lp_lsp_roots_test.go`.

- [ ] **Step 1: Write failing test** on f1/f2 derived from a known LP filter where the roots ω_i can be computed analytically (e.g., Levinson on a 2-tap filter with one resonance), assert all 5 roots of C_1 and 5 roots of C_2 are recovered to ±1 LSB Q15, and assert ordering 0 < ω_1 < ω_2 < ... < ω_10 < π.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation** with **exactly** 60 grid points uniformly spaced on x ∈ [−1, +1] (the cosine domain of ω ∈ [0, π]) using `tables.CosLSP` (65 endpoints, sampled at 60 equally-spaced indices to match §3.2.3 line 783 verbatim "60 points equally spaced between 0 and π"). On each sign change between grid[k] and grid[k+1], divide the interval **4 times** (binary subdivision per line 784) before snapping to the midpoint. Track that 5 roots come from F1 and 5 from F2; interleave them in ω order. (I11.)
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.3 lines 782–799. I11 binding: 60 grid + 4 subdivisions.

**Failure behaviour:** if fewer than 5 sign changes are detected for either polynomial, return a sentinel (`ErrLPCNonStable` — new in `internal/lsp` errors) so the encoder can route to E8 (Levinson defect upstream).

**Commit message:**
```
feat(lsp): Phase 2a-LP-3 LP→LSP sign-change root finder (60+4)

Implements §3.2.3 lines 782–799 — 60 equally-spaced sign-change
detections on x = cos(ω) using tables.CosLSP, followed by 4 binary
subdivisions per detected interval. Returns 10 LSPs in ω order
(F1/F2 interleaved). ErrLPCNonStable sentinel on missing roots
gates Phase 2a E8.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-LP-4: `LPToLSP` top-level wrapper

**Files:** Extend `internal/lsp/lp_lsp.go`, add `internal/lsp/lp_lsp_top_test.go`.

- [ ] **Step 1: Write failing test** asserting the round-trip property: for every L1/L2/L3 codebook entry (sample ~100 of them), `lspToLP(lsp); LPToLSP(a)` returns `lsp` within ±4 in Q15 (the four-step subdivision tolerance dominates).
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** chains LP-1 → LP-3 with appropriate buffer plumbing.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.3 lines 738–799 (full clause).

**Commit message:**
```
feat(lsp): Phase 2a-LP-4 LPToLSP top-level wrapper

Composes LP-1 (F1/F2) → LP-2 (Chebyshev) → LP-3 (root finder) into
a single LPToLSP(a *[11]int16, q *[10]int16) entry point. Round-trip
test against lspToLP across ~100 codebook-derived LSPs validates the
chain to ±4 LSBs in Q15.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 6. Task family MA — encoder MA predictor + target (§3.2.4 eq. 20, 23)

### Task 2a-MA-1: Non-destructive predictor evaluator + memory commit

**Files:** Create `internal/lsp/encoder_predictor.go`, `internal/lsp/encoder_predictor_test.go`.

- [ ] **Step 1: Write failing test** asserting that `applyPredictorWithMemory` produces the same output as the existing decoder-side `applyPredictor` for the same `(memory, selector, residual)` triple, but **does not mutate `memory`**. Then `commitPredictorMemory(memory, residual)` advances the FIFO and now the next non-destructive evaluation matches the decoder's post-mutation state.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** copy of `(d *Decoder).applyPredictor`'s arithmetic body but reading from `mem *[4][10]int16` instead of `d.pastResiduals` and **omitting** the FIFO shift. `commitPredictorMemory` is the FIFO shift alone.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 eq. 20 (lines 836–840). I10 binding: encoder-owned memory.

**Q-format:** residual Q13, predictor Q15, output Q13 — identical to decoder side.

**Commit message:**
```
feat(lsp): Phase 2a-MA-1 non-destructive MA predictor evaluator

applyPredictorWithMemory mirrors the decoder-side §3.2.4 eq. 20
arithmetic but reads memory by pointer and does not mutate it,
enabling repeated trial evaluations during the encoder's L0 search
loop. commitPredictorMemory performs the post-decision FIFO advance.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-MA-2: Target vector l_i computation (§3.2.4 eq. 23)

**Files:** Extend `internal/lsp/encoder_predictor.go`, add `internal/lsp/encoder_target_test.go`.

- [ ] **Step 1: Write failing test** with synthetic ω(m) = i·π/11 (codec-start), zeroed memory: assert l_i = ω(m) (since predictor contribution is 0 at start). Second case: ω(m) = 0; with memory loaded with one non-zero past frame, assert the closed-form value of eq. 23.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** for each i, sum `Σ_{k=1..4} P_{i,k} · mem[k-1][i]`, subtract from ω(m), divide by `(1 − Σ_{k=1..4} P_{i,k})` using `fixed.Div32`. Q-format: ω, l_i in Q13; predictor Q15; intermediate Word32.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 eq. 23 (lines 884–886).

**Commit message:**
```
feat(lsp): Phase 2a-MA-2 target vector l_i per §3.2.4 eq. 23

computeTargetLSF derives the residual to be quantized for one MA
predictor selector: l_i = (ω_i − Σ P·history) / (1 − Σ P), in Q13.
Pure function, two synthetic-input tests cover the start-up and
nonzero-memory cases.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 7. Task family VQ — two-stage 18-bit VQ search (§3.2.4)

### Task 2a-VQ-1: Adaptive weights w_i (§3.2.4 eq. 22)

**Files:** Create `internal/lsp/encoder_weights.go`, `internal/lsp/encoder_weights_test.go`.

- [ ] **Step 1: Write failing test** with three ω vectors:
  1. Uniform spacing ω_i = i·π/11 → all branches of eq. 22 hit "if … > 0" and w = 1.0 except w_5/w_6 boosted by 1.2.
  2. Cluster ω_5 ≈ ω_6 → otherwise branch on w_5 → w_5 > 1.0.
  3. ω_2 < 0.04π → w_1 in otherwise branch, recompute by hand.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** exactly the three-branch piecewise of eq. 22 lines 864–880. ×1.2 boost on w_5/w_6 line 882. Q-format: ω in Q13, w in Q11 (so 1.0 = 2048; 1.2 = 2458). Constants 0.04π ≈ 0.1257 → 1029 (Q13), 0.92π ≈ 2.890 → 23676 (Q13), gap threshold 1.0 → 8192 (Q13). Compute (gap − 1.0) in Q13 then square to Q26, multiply by 10 → Q26, add 1.0 in Q26 (67108864), reciprocal via `fixed.Div32` to Q11 weight. Document each constant cite line-for-line.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 eq. 22 (lines 863–882).

**Commit message:**
```
feat(lsp): Phase 2a-VQ-1 adaptive weights w_i per §3.2.4 eq. 22

weightsLSF computes the 10 frequency-spacing-driven weights w_i in
Q11, including the ×1.2 boost on w_5/w_6 (line 882). Three test
inputs cover the uniform branch, a cluster forcing w_5 into the
spacing-otherwise branch, and the w_1 edge case.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-VQ-2: First-stage L1 search (unweighted MSE)

**Files:** Create `internal/lsp/encoder_vq.go`, `internal/lsp/encoder_vq_l1_test.go`.

- [ ] **Step 1: Write failing test:** for a target vector equal to row 17 of `LSPCodebookL1`, assert `searchL1` returns 17 with zero MSE.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** brute-force scan of all 128 rows; for each, compute Σ (target − row[i])² in Word32 (target Q13, row Q13, diff Q13, square Q26). Pick argmin. **No weighting at this stage** per §3.2.4 line 887 verbatim ("the entry L1 that minimizes the (unweighted) mean-squared error").
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 lines 887–888.

**Commit message:**
```
feat(lsp): Phase 2a-VQ-2 first-stage L1 unweighted MSE search

searchL1 brute-force scans all 128 rows of LSPCodebookL1 and returns
the index minimizing the unweighted Σ(target-row)² per §3.2.4 line
887. 7-bit output, exhaustive per I12.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-VQ-3: L2 lower-half search (rearrangement-J1 + weighted MSE on partial vector)

**Files:** Extend `internal/lsp/encoder_vq.go`, add `internal/lsp/encoder_vq_l2_test.go`.

- [ ] **Step 1: Write failing test:** for each of 32 rows, build the candidate partial residual r[0..4] = L1[l1][0..4] + L2[l2][0..4]; reconstruct ω̂[0..4] via the predictor, apply `rearrangeAdjacent(_, lsfRearrJ1)` on the partial 5-vector, compute Σ_{i=0..4} w_i·(ω_i − ω̂_i)², choose argmin. Validate on a synthetic case where the closed-form winner is known.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** loop over 32 candidates; per candidate use a stack-allocated `[10]int16` workspace (only first 5 elements meaningful in the partial pass, but reuse the function signature). Reuse `combineResidual` with l3 = arbitrary placeholder (the partial cost only sums i=0..4 so L3 contribution is irrelevant). Apply `applyPredictorWithMemory` (non-destructive), then `rearrangeAdjacent` on the partial, then weighted MSE on i=0..4. **Note:** the L1+L2 combine is structurally L1[0..4]+L2; a direct partial-combine without the placeholder is preferred to keep the inner loop allocation-free.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 lines 889–892 (L2 search), line 890 ("rearranged to guarantee a minimum distance of 0.0012") = `lsfRearrJ1`.

**Commit message:**
```
feat(lsp): Phase 2a-VQ-3 second-stage L2 lower-split search

searchL2 brute-force scans all 32 rows of LSPCodebookL2; for each,
reconstructs the partial ω̂[0..4] via the encoder MA predictor,
applies the J=0.0012 rearrangement (lsfRearrJ1) per §3.2.4 line 890,
and minimizes Σ_{i=0..4} w_i·(ω_i − ω̂_i)². Allocation-free.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-VQ-4: L3 upper-half search

**Files:** Extend `internal/lsp/encoder_vq.go`, add `internal/lsp/encoder_vq_l3_test.go`.

- [ ] **Step 1: Write failing test:** mirror VQ-3 over indices i=5..9, with L3 candidates over the upper 5 dimensions; rearrangement-J1 spans the full [0..9] vector now (per spec line 893: "Again the rearrangement procedure is used to guarantee a minimum distance of 0.0012").
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** brute-force over 32 L3 rows; partial sum on i=5..9 weighted MSE.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 lines 893–895.

**Commit message:**
```
feat(lsp): Phase 2a-VQ-4 second-stage L3 upper-split search

searchL3 mirrors VQ-3 for the upper half: 32 candidates, partial
weighted MSE on i=5..9, J=0.0012 rearrangement on the full
reconstructed [0..9] vector per §3.2.4 line 893.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-VQ-5: L0 selector outer loop (best of two MA predictors)

**Files:** Extend `internal/lsp/encoder_vq.go`, add `internal/lsp/encoder_vq_l0_test.go`.

- [ ] **Step 1: Write failing test:** for a hand-constructed ω where predictor 0 gives a closed-form lower MSE than predictor 1, assert L0 = 0; and the symmetric case asserts L0 = 1.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** outer loop over selector ∈ {0, 1}; for each, run VQ-2 → VQ-3 → VQ-4 to get (L1, L2, L3); reconstruct full ω̂ via predictor + J1 rearrangement; compute total weighted MSE Σ w_i·(ω_i − ω̂_i)². Pick the selector with lower MSE. Then `commitPredictorMemory` exactly once on the winning residual.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §3.2.4 lines 851 ("For each of the two MA predictors the best approximation … has to be found"), 896–897 ("the MA predictor L0 that produces the lowest weighted MSE is selected").

**I12 verification:** add a synthetic-input subtest counting that the inner loops issue exactly 2 · (128 + 32 + 32) = 384 candidate evaluations per frame (or 2 · 128 · 32 · 32 = 262 144 if the spec text is later re-read as full-cross-product — current reading is sequential-greedy per stage, line-by-line of §3.2.4 lines 887–895).

> **Spec re-read note:** §3.2.4 lines 889–892 say "**Using the selected first stage vector L1**" — this means L2 search uses the L1 winner, NOT a full cross-product. Likewise L3 uses L1+L2 winner. So total = 2 · (128 + 32 + 32) = 384 evaluations / frame, sequential-greedy. **I12 is amended:** Phase 2a uses the spec-mandated sequential-greedy search (NOT exhaustive cross-product). The phrase "exhaustive" in I12 refers to "every codebook row at each stage", which 384 evaluations satisfy.

**Commit message:**
```
feat(lsp): Phase 2a-VQ-5 L0 selector + Quantize top-level entry

Wraps VQ-2..VQ-4 in the outer selector loop per §3.2.4 lines 851,
896–897. Quantize(omega, freqPrev) returns Indices{L0,L1,L2,L3} and
calls commitPredictorMemory exactly once on the winning residual.
Sequential-greedy = 384 candidate evaluations per frame per I12.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 8. Task family INT — integration + LSP.BIT byte-EQ gate

### Task 2a-INT-0: LSP.BIT bit-field extractor (test-side helper)

**Files:** Create `internal/bitstream/lsp_field_test.go` (test-only, no production change).

> Per I6 this lives in `_test.go`; it is a test helper, not a production decoder.

- [ ] **Step 1: Write failing test** asserting that for `LSP.BIT` frame 0, the extracted (L0, L1, L2, L3) tuple equals the values manually decoded from the first 18 G.192 bit-words at the LSP positions (bit indices 0–17 of the 80-bit payload).
- [ ] **Step 2: Run to verify FAIL** (`undefined: extractLSPFields`).
- [ ] **Step 3: Write minimal implementation** of `extractLSPFields(g192Frame []byte) (l0, l1, l2, l3 uint8)` — read 18 bit-words at offset 4 (skipping G.192 sync header), MSB-first per §A.4 Table A.4, pack into the four indices. Pure test helper.
- [ ] **Step 4: Run to verify PASS** on a hand-traced first-frame oracle.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.4 Table A.4 (transmission order); G.192 transport format.

**E6 / E7 binding:** if the layout disagrees with the manual oracle, halt and write a `*-report.md` per E7.

**Commit message:**
```
test(bitstream): Phase 2a-INT-0 LSP.BIT field extractor

Adds extractLSPFields test helper that reads (L0,L1,L2,L3) from one
G.192-framed bitstream record per §A.4 Table A.4 transmission order.
Test-only; no production-code change. Validated against a hand-decoded
LSP.BIT first-frame oracle.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-INT-1: End-to-end LSP.IN → indices, gated against LSP.BIT (all 2232 frames)

**Files:** Modify `encoder.go` (wire `lpcStep` partial method), add `lsp_itu_vector_test.go` at root, modify `internal/lpc/types.go` to flesh out `Analyzer.Analyze`, modify (extend, don't replace) `internal/lpc/doc.go`.

- [ ] **Step 1: Write failing test** `TestEncode_LSPVectorBitExact` that:
  1. Loads `testdata/itu/G729_Release3/g729/test_vectors/LSP.IN` (357120 B = 2232 frames × 80 samples).
  2. Loads `testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT` and uses `extractLSPFields` per frame.
  3. For each frame: feeds 80 samples into a fresh `Encoder`, calls a new package-private method `(e *Encoder).lpcStep(pcm []int16) (lsp.Indices, error)` that runs the §3.2.1–§3.2.4 chain and returns the indices.
  4. Asserts `got == want` per (L0, L1, L2, L3); on first divergence, dumps frame index + per-boundary trace per §0.4.
- [ ] **Step 2: Run to verify FAIL** initially (`undefined: (*Encoder).lpcStep` and `errStub`-shaped Analyze).
- [ ] **Step 3: Write minimal implementation:**
  - In `internal/lpc/types.go`: Replace `Analyze` body with the full chain `windowSpeech` → `autocorrelate` → `applyNoiseFloorAndLagWindow` → `levinsonDurbin`, returning Q12 a[11] via the `out *[11]int16` (signature change: from `out []int16` slice to `out *[11]int16` pointer-to-array — done now to enforce the size contract; update `types_test.go` and any caller stubs accordingly).
  - In `encoder.go`: add `(e *Encoder) lpcStep(pcm []int16) (lsp.Indices, error)`:
    1. Shift `oldSpeech` left by 80 samples and append the 80 input samples to positions [160..239]. (Look-ahead = 40 samples per §3.2.1 line 671 means the LP analysis window for *frame n* uses 120 past + 80 present + 40 future = 240 samples. **Phase 2a accepts a 1-frame analysis-vs-encode delay**: at frame n we analyze the speech that ended at frame n-1's last sample plus 40 look-ahead from frame n. Phase 2a gate vector LSP.BIT was generated by ITU's encoder under this convention; document the buffer-shift ordering in a comment with the §3.2.1 line 671 cite.)
    2. Call `lpc.Analyze(&e.oldSpeech, &aQ12)`.
    3. Call `lsp.LPToLSP(&aQ12, &qQ15)`.
    4. Convert qQ15 → ω in Q13 via `arccos` LUT — **NEW SUB-TASK 2a-INT-1a if not already provided**: the existing `lsfToLSP` is the forward direction (ω→q); we need `lspToLSF(q, omega)`. If absent at INT-1 entry, spawn it as a strictly-test-driven micro-task before continuing.
    5. Call `lsp.Quantize(&omega, &e.freqPrev)` → `lsp.Indices`.
- [ ] **Step 4: Run to verify PASS** on all 2232 frames.
- [ ] **Step 5: Commit.**

**Spec cite:** chains §3.2.1–§3.2.4 end-to-end; gate format §A.4.

**Expected vs measured gate:** **byte-EQ on (L0,L1,L2,L3) for all 2232 frames**. On first divergence, produce a `*-report.md` per §0.4 boundary-trace order.

**Hypothesis-budget posture:** if the first divergence cannot be closed within 5 production-fix attempts, freeze production (I6) and open a `phase2a_int1_*_diagnostic_test.go` sub-cycle per E9.

**Commit message:**
```
feat(g729): Phase 2a-INT-1 LSP.IN → indices byte-EQ against LSP.BIT

End-to-end wiring: Encoder.lpcStep runs §3.2.1 windowed
autocorrelation → §3.2.2 Levinson-Durbin → §3.2.3 LP→LSP →
§3.2.4 18-bit two-stage VQ + L0 selector, returning lsp.Indices.
TestEncode_LSPVectorBitExact validates byte equality of (L0,L1,L2,L3)
across all 2232 frames of LSP.IN against LSP.BIT extracted via
G.192 field reader.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task 2a-INT-2: Zero-allocation gate + cross-package bench

**Files:** Add `lsp_alloc_test.go` at root, extend `internal/lpc/alloc_test.go`.

- [ ] **Step 1: Write failing test** asserting `testing.AllocsPerRun(100, func(){ enc.lpcStep(pcm) })` returns `0.0` (per I4) and `testing.Benchmark` reports a baseline (no assertion, just a `b.ReportAllocs()` capture).
- [ ] **Step 2: Run to verify FAIL** if any inner-loop allocation slipped in (likely VQ search workspace).
- [ ] **Step 3: Write minimal implementation:** hoist any leaked allocation onto `Encoder` as a scratch field. Candidates flagged at design-time: the partial-vector workspace inside `searchL2`/`searchL3` (5-element int16 arrays — must be stack-allocated; verify `go build -gcflags='-m'` shows `does not escape`).
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** N/A (engineering invariant I4).

**Commit message:**
```
test(g729): Phase 2a-INT-2 zero-allocation gate on Encoder.lpcStep

Pins I4: AllocsPerRun(100, lpcStep) == 0. Surfaces any escape from
the VQ inner-loop scratch buffers; current implementation passes by
keeping all candidate workspaces on the stack and reusing
Encoder.freqPrev for the only persistent state.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 9. Pass criteria for Phase 2a closure

- [ ] **C1.** All ITU `LSP.IN` → `LSP.BIT` 2232 frames produce byte-EQ (L0, L1, L2, L3) — `TestEncode_LSPVectorBitExact` GREEN. No `t.Skip`. If a divergence-vs-spec-vector path is taken (analogous to Phase 1o's PST-vs-BIT divergence handling), it MUST be documented in a `*-report.md` with §A.4-style cite, **and is not expected** for the LSP gate (no analogous PST-style spec-divergence is on record for LSP).
- [ ] **C2.** `go vet ./...` exits 0.
- [ ] **C3.** `go build ./...` exits 0.
- [ ] **C4.** Baseline 3 FAIL (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) and 3 SKIP unchanged. **Net-new PASS count expected ≥ +20** (one per VQ stage test, one per LP stage, one per AC/W/LD step, plus INT-0/1/2 = ~25 new tests across the 14 tasks).
- [ ] **C5.** Zero allocation in `Encoder.lpcStep` per I4 (`AllocsPerRun == 0`).
- [ ] **C6.** No `internal/tables/lsp_*.go` modifications across the entire Phase 2a series (I9 self-attest).
- [ ] **C7.** No `internal/lsp/decoder.go` mutation of `pastResiduals` from encoder code paths (I10 grep gate).
- [ ] **C8.** `EncodeFrame` itself still returns `ErrNotImplemented` after Phase 2a — Phase 2a wires only `lpcStep`, NOT the public `EncodeFrame` (which awaits Phase 2b/c/d/e/f). Document this in the `Encoder.EncodeFrame` doc comment.
- [ ] **C9.** Closure report `docs/superpowers/plans/YYYY-MM-DD-phase2a-completion-report.md` produced summarising:
  - Tasks closed vs. tasks deferred.
  - Each per-task expected-vs-measured table (especially INT-1's frame-by-frame gate evidence).
  - I5/I6/I9/I10/I11/I12 self-attest.
  - Inherited-FAIL re-evaluation (does an encoder LSP path surface evidence on `TestDecode_LowEnergyCodebookIsSmooth` / `TestDecode_SucceedsAcrossAllGainIndices`? Update gain-side report if yes.).

---

## 10. Inheritance to Phase 2b (open-loop pitch)

**State carry from Phase 2a → 2b:**
- `Encoder.lspOld[10]int16` (Q15) — populated by `lpcStep` after each frame; needed by Phase 2b for the `interpolateLSP` call that produces the unquantized per-subframe a coefficients used by perceptual weighting (§3.3).
- `Encoder.lspOldQ[10]int16` (Q15) — same, quantized branch. Not strictly needed by 2b's open-loop pitch (pitch uses unquantized perceptual weighting), but kept fresh for 2c/2d.
- `Encoder.freqPrev[4][10]int16` (Q13) — MA predictor history; consumed only by Phase 2a's own `Quantize` re-entry on subsequent frames. 2b does not touch it.

**Files / packages already touched at Phase 2a closure:**
- `internal/lpc/{doc.go,types.go,window.go,autocorr.go,lagwindow.go,levinson.go}` plus their test files.
- `internal/lsp/{lp_lsp.go,encoder_predictor.go,encoder_weights.go,encoder_vq.go}` plus test files. **Decoder-side files unmodified.**
- `internal/bitstream/lsp_field_test.go` (test-only).
- `encoder.go` — adds `lpcStep` private method.

**Phase 2b entry preconditions:**
- All Phase 2a C1..C9 satisfied.
- `Encoder.oldWspeech[143]int16` is **NOT** populated at Phase 2a closure (perceptual weighting filter is a Phase 2b/3.3 responsibility). Phase 2b will:
  - Build the perceptual weighting filter A(z/γ_1)/A(z/γ_2) from `Encoder.lspOld → lspToLP → weighted` (§3.3).
  - Filter the present + past speech into `oldWspeech`.
  - Run the §3.4–§3.5 open-loop pitch search.
- `Encoder.synMem`, `wMem`, `errMem` remain zeroed; their first use is in Phase 2c (target computation §3.6).

**Open issues passed to Phase 2b:**
- The 1-frame analysis-vs-encode delay introduced at Task 2a-INT-1 step 3 (look-ahead handling) interacts with `oldWspeech[143]`'s indexing convention. Phase 2b plan must explicitly re-state the buffer-shift ordering (Phase 2a doc-comments it, Phase 2b owns it).

---

## 11. Self-Review

Per master plan §10, this sub-plan was reviewed against:

1. **I1 clean-room.** All citations point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` (the bundled ITU-T G.729 (06/2012) recommendation) or to the design spec `docs/superpowers/specs/2026-04-20-g729-codec-design.md`. **No URL / file path of any external G.729 implementation appears in this plan.** Self-attest: ✅.
2. **I7 TDD discipline.** Every code-producing task has the 5-step (test → fail → impl → pass → commit) checklist. ✅.
3. **I8 Co-author trailer.** Every commit message in this plan includes the trailer. ✅.
4. **Task depth** matches Phase 1k F-* / Phase 1o D-* / Phase 2-0 plan precedent (per-task spec cite, Q-format pinning, expected-vs-measured gate where applicable). ✅.
5. **Scope.** Plan does not implement code; only describes it. ✅.

---

## 12. Execution handoff

**Recommended next dispatch:** execute Phase 2a Task 2a-W-1 (Hamming + cosine window LUT) per §2 of this plan. Open the next session with:

> "Execute Task 2a-W-1 from `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md` per the 5-step TDD checklist. Stop after the commit and report back for dispatch of Task 2a-W-2."

Sub-skill: `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`.
