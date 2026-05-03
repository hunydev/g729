# Phase 2b — Open-loop pitch estimation sub-plan

> **STATUS: IN PROGRESS — 2026-05-07.** TDD task ledger for Annex A §A.3.3 (low-pass weighted speech) + §A.3.4 (decimated three-range open-loop pitch search). Awaits dispatch of WS-1.

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Date:** 2026-05-07
**Phase:** 2b (encoder open-loop pitch: low-pass weighted speech → 3-range correlation/energy → integer T_op ∈ [20,143])
**Parent plan:** `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §3
**Phase 2a closure ref:** `docs/superpowers/plans/2026-05-06-phase2a-closure-report.md`
**Phase 2a sub-plan ref:** `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md`

**Goal:** Implement encoder-side open-loop pitch estimation per ITU-T G.729 Annex A §A.3.3 + §A.3.4. Two visible products:

1. A package-internal Go function (proposed: `openloop.Search(wsp *[143+80]int16) int16`) that returns the integer pitch lag T_op for one 80-sample frame.
2. A package-private encoder method `(*Encoder).openloopStep(processed *[80]int16) int16` that (a) updates the perceptual-weighted-speech sliding history `Encoder.oldWspeech[143]` after `lpcStep` has produced this frame's unquantized LP coefficients, and (b) returns T_op for downstream Phase 2c closed-loop pitch refinement.

**Architecture (high-level):**

- **Annex A path (binding for this codec).** Per the project scope (`docs/superpowers/specs/2026-04-20-g729-codec-design.md` §1 line 5: "Pure-Go G.729 Annex A"), Phase 2b targets §A.3.3 + §A.3.4 verbatim, NOT base-codec §3.3 + §3.4. The differences are spec-explicit and material:
  - Perceptual weighting filter is `W(z) = Â(z)/Â(z/γ)` with **fixed γ = 0.75** (eq. A.1) — *not* `A(z/γ₁)/A(z/γ₂)` with adaptive γ₁ ∈ {0.94, 0.98} / γ₂ ∈ [0.4, 0.7].
  - Open-loop pitch uses a low-pass filtered weighted speech `sw(n)` (eq. A.2) obtained by filtering `s(n)` through `Â(z)/[Â(z/γ)(1 − 0.7z⁻¹)]`.
  - Correlation in eq. A.4 uses **decimated even samples** `Σ sw(2n)·sw(2n−k), n=0..39`.
  - Range [80,143] is searched **only at even delays first**, then ±1 of the even winner is tested (§A.3.4 lines 2112–2114).
  - The three-range selection rule (§A.3.4 lines 2109–2111) is described as "augmenting the normalized correlations corresponding to the lower delay range if their delays are submultiples of the delays in the higher delay range" — i.e. the implementation lifts the shorter-delay normalized correlation when it is a sub-multiple of a longer-delay candidate. **This is intentionally vague in the spec text; the precise multiplier and threshold are not fixed by the spec narrative.** See §9 OQ-1 for the binding closure path.
- **Cross-package state ownership.** `Encoder.oldWspeech[143]int16` was reserved by Phase 2-0 scaffold (`encoder.go:23`). Phase 2b owns its slide-by-80 update (mirrors the `oldSpeech[240]` pattern from Phase 2a). The 143-sample history covers the maximum lag k=143 in eq. A.4 (search needs `sw[n−k]` for n=0..79 and k up to 143, so the buffer must hold the previous frame's 143 samples = 80 current + 63 already-aged — pinned by §A.3.4 line 2097 max range).
- **Separation from decoder pitch package.** `internal/pitch/` already contains *decoder-side* helpers (`adaptive.go`, `delay.go`, `parity.go`). Per Phase 2a I10 precedent ("encoder-decoder state isolation"), Phase 2b adds a *new sibling package* `internal/pitch/openloop/` so the encoder open-loop search does not import or mutate decoder pitch state. See §4 for the package-layout decision.

**Tech Stack:** Go 1.22+, zero runtime dependencies, no CGo, no SIMD, no assembly. All arithmetic via `internal/fixed` (G.191 STL-equivalent saturating Word16/Word32). MIT clean-room.

**Source spec citations (mandatory traceability):**

- `docs/superpowers/specs/itu/G729E.txt` §3.3 lines 948–1008 (full-codec perceptual weighting — *informational only* for Phase 2b; Annex A overrides).
- `docs/superpowers/specs/itu/G729E.txt` §3.4 lines 1010–1050 (full-codec open-loop pitch — *informational only*; Annex A overrides ranges, decimation, selection rule).
- `docs/superpowers/specs/itu/G729E.txt` §A.3.3 lines 2057–2083 (Annex A perceptual weighting; eq. A.1, A.2, A.3 binding).
- `docs/superpowers/specs/itu/G729E.txt` §A.3.4 lines 2084–2114 (Annex A open-loop pitch; eq. A.4, A.5, three-range selection binding).
- `docs/superpowers/specs/2026-04-20-g729-codec-design.md` §1, §3.3, §5.1 stage 5 (encoder data flow).

**Phase 2a inheritance (HEAD `e2b689e` → entry HEAD for Phase 2b):**

- `Encoder.lpcStep` is the spec-arithmetic-conformant LP analysis + LSP quantization sub-chain. Phase 2b runs *after* `lpcStep` in the same frame; the unquantized Q12 LP coefficients `aQ12[11]` produced by `lpc.Analyzer.Analyze` must be available to Phase 2b's perceptual-weighting filter. This implies a small refactor: `lpcStep` will need to surface `aQ12` (currently a local) to the caller, OR `openloopStep` will recompute it from the latest LP-domain state. **Plan defaults to surfacing.** See INT-0 for the wiring.
- `Encoder.lspOldQ[10]` (Q15 quantized LSP) is updated by `lpcStep` and is also available to `openloopStep` if §A.3.3 eq. A.1 is implemented from the *quantized* LP path (Annex A uses `Â`, the quantized filter, per line 2058). The quantized LP coefficients `âQ12` are not currently materialized by `lpcStep`; an additional `lsp.LSPToLP(&qHatQ15, &aHatQ12)` invocation is required (already in tree as `lspToLP` per Phase 2a §1.2 reuse table). See INT-0 step 3.
- All Phase 2a pinned engineering invariants (I1, I3, I4, I8, I9, I10) carry forward unmodified.

---

## 0. Inherited invariants

Inherits master-plan I1..I8 verbatim (`docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §0.2) and Phase 2a I9..I12 *only where they apply* — I9 (LSP codebook discipline), I10 (encoder/decoder state isolation), and I12 (exhaustive base-spec VQ) are LSP-specific and not gating for Phase 2b. I11 (Chebyshev grid) is LSP-specific.

Phase 2b explicit re-statement of cross-cutting invariants:

| # | Invariant | Phase 2b enforcement |
|---|-----------|----------------------|
| I1 | **CLEAN-ROOM.** Only `docs/superpowers/specs/itu/G729E.{pdf,txt}` and our own prior plans/docs. No ITU-T C reference, no bcg729, no Sipro, no FFmpeg, no other G.729 implementation. | Self-attest at every commit; spec-cite every numeric constant (γ=0.75, 0.7 LPF coefficient, 0.85 base-codec threshold for §3.4 only, decimation factor 2). |
| I3 | **Purity.** `openloopStep` is pure (mutates only `Encoder.oldWspeech` and returns `int16`); no I/O outside the existing `io.Reader/Writer` API surface. | Per-task: no `os.*`, no `fmt.Print*`, no time/random. |
| I4 | **Zero-alloc on hot path.** All per-frame scratch lives in stack-resident fixed-size arrays. | INT-2 pins `AllocsPerRun(128, openloopStep) == 0` and `AllocsPerRun(128, lpcStep+openloopStep) == 0`. |
| I5 | **Escalation budget.** ≤5 production-fix attempts per integration gate (INT-1). On exceed → invoke E9 (freeze production, write `phase2b_int1_*_diagnostic_test.go`, escalate). | Tracker §8. |
| I6 | **Production-freeze on byte-EQ shortfall.** If INT-1 byte-EQ vs. ITU PITCH vector is < 100 % and I5 is exhausted, freeze production for the open-loop pitch surface and document an ACCEPT-PARTIAL closure (Phase 2a precedent: `docs/superpowers/plans/2026-05-05-phase2a-int1-accept-partial-closure.md`). | INT-1 disposition. |
| I8 | **Commit trailer** `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` on every commit. | Per-task commit-message stub. |
| I-2b-1 | **Annex A binding.** Phase 2b implements §A.3.3 + §A.3.4, NOT §3.3 + §3.4. Every algorithm comment cites the §A.3.x line range, NOT §3.3 / §3.4 (the latter is referenced only as informational context). | Per-task: comment audit at PR time. |
| I-2b-2 | **`oldWspeech[143]` slide-by-80 ordering** mirrors `oldSpeech[240]` slide-by-80 from Phase 2a (`encoder.go:117–118`): at frame n, `oldWspeech[0:63]` ← previous-frame `oldWspeech[80:143]`, `oldWspeech[63:143]` ← current-frame freshly weighted samples. The 143 = 63 past + 80 current layout is pinned by §A.3.4 max lag k=143 (eq. A.4 needs `sw(2n − k)` for n=0..39, k up to 143, so n=0,k=143 ⇒ index −143; with 80 current samples indexed [0..79] in the local frame, the past must extend 143 samples below sample 0 of the current frame — i.e. the buffer must total 80 + 143 = 223 samples in the *concatenated view*, but the *retained history across frame boundaries* is 143). See WS-2 for the buffer-layout test. | Per-task: WS-2 step-1 test asserts the slide ordering against a synthetic ramp. |

---

## 1. Spec anchors (line ranges in `docs/superpowers/specs/itu/G729E.txt`)

| § | Lines | Content | Binding for Phase 2b? |
|---|------:|---------|:---------------------:|
| 3.3 | 948–1008 | Base-codec perceptual weighting `A(z/γ₁)/A(z/γ₂)` with adaptive γ₁/γ₂ via LAR + flat/tilted hysteresis. | ❌ informational |
| 3.4 | 1010–1050 | Base-codec open-loop pitch: ranges 80–143 / 40–79 / 20–39, eq. 34 (full-rate correlation, no decimation), eq. 35 normalization, 0.85 threshold rule "favouring the delays with the values in the lower range" (NB: this is **shorter-delay bias** in the base spec — selection starts from t1 in the longer range and is overridden by t2/t3 if those reach 0.85·R'(Top)). | ❌ informational |
| A.3.3 | 2057–2083 | Annex A perceptual weighting `W(z) = Â(z)/Â(z/γ)`, γ=0.75 (eq. A.1); LP residual `r(n) = s(n) + Σ â_i·s(n−i)` (eq. A.3); low-pass weighted speech `Sw(n) = r(n) − Σ a'_i·sw(n−i)` (eq. A.2) with `A'(z) = Â(z/γ)·(1 − 0.7z⁻¹)`. | ✅ binding |
| A.3.4 | 2084–2114 | Annex A open-loop pitch: eq. A.4 decimated correlation `R(k) = Σ sw(2n)·sw(2n−k), n=0..39`; eq. A.5 normalization; ranges i=1: 20..39, i=2: 40..79, i=3: 80..143; sub-multiple lift rule; range-3 even-only first pass + ±1 refinement. | ✅ binding |

> **Note on user-task-prompt phrasing.** The dispatch message described "best-of-3 with bias to longer lags". The spec (both §3.4 and §A.3.4) actually biases to **shorter** lags ("favouring the delays with the values in the lower range" / "augmenting the normalized correlations corresponding to the lower delay range"). The plan follows the spec, not the dispatch phrasing. This is logged here for traceability and is not an open question.

---

## 2. Test-vector inventory (gate vector for INT-1)

`testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.{IN,BIT,PST}` is byte-identical to `testdata/itu/G729_Release3/g729/test_vectors/PITCH.{IN,BIT,PST}` (verified by `wc -c`):

| File | Size (B) | Notes |
|---|---:|---|
| `PITCH.IN`  | 293 628 | Intel little-endian int16 PCM. 293628 / 2 = 146814 samples. **Not a clean multiple of 80** (146814 = 1835·80 + 14); the leftover 14 samples likely represent encoder look-ahead alignment. INT-1 step 1 must verify and document the exact framing convention (header bytes? leading look-ahead samples? trailing pad?) before promoting any T_op assertion to gating. |
| `PITCH.BIT` | 300 940 | G.192 framed: 4-byte sync header + 80 × 2-byte bit-words (`0x007F`/`0x0081`). 300940 / 164 = 1835 frames exactly. Bit positions for P1/P0 (and per Annex A §A.4 for P2 second-subframe delta) per §4 Table 8 / §A.4 Table A.4. |
| `PITCH.PST` | 293 600 | Decoder PCM output reference. 293600 / 2 = 146800 samples = 1835 × 80 exactly. Used as a sanity reference only; Phase 2b gate is on the bitstream side. |

**Field extraction for INT-1.** The `PITCH.BIT` field of interest is **P1** (first-subframe pitch delay, 8 bits = 6-bit integer + 2-bit fractional, but for open-loop the integer T_op is the *centre* of the closed-loop search range, NOT directly P1). Therefore PITCH.BIT alone does not directly expose T_op as a bit field — closed-loop refinement (Phase 2c) sits between T_op and P1. **Phase 2b INT-1 gate is therefore a *consistency* test, not a bit-field byte-EQ test:**

- **Primary gate (consistency):** Run encoder LPC + open-loop chain across all 1835 frames of PITCH.IN; assert (a) every `T_op` returned is in [20, 143], (b) zero allocations, (c) no panics. Capture each frame's `T_op` to a CSV-like artefact under `testdata/phase2b/openloop_top_capture.csv` for cross-checking once Phase 2c lands the closed-loop search and surfaces P1.
- **Secondary gate (round-trip plausibility):** For each frame's emitted `T_op`, check that `P1` extracted from PITCH.BIT lies within `[T_op − 5, T_op + 4]` (the Annex A §A.3.7 closed-loop search window) — if it doesn't, log the frame index. This is a *plausibility* check, not a strict gate, because the Annex A search window is inclusive of fractional offsets and may extend with boundary clamping.
- **Tertiary gate (deferred to Phase 2c):** Once Phase 2c lands, the combined open + closed loop produces P1 directly; the byte-EQ gate moves to Phase 2c INT-1. Phase 2b INT-1 closure does NOT block on full byte-EQ.

> **E10 (Phase 2b-specific escape hatch):** if PITCH.IN framing turns out to require non-trivial pre-processing (e.g. embedded header, decimated leading look-ahead) that is not documented in `READMETV.txt` or the §A.4 spec, halt and write a `phase2b_pitchin_framing_diagnostic_test.go` measurement-only test, then resume.

---

## 3. Pre-flight inventory

### 3.1 Working-tree gate

- [ ] **Step 3.1.1: Confirm Phase 2a CLOSED at HEAD `0c5fc86`** (current HEAD per dispatch context).
  ```bash
  git rev-parse --short HEAD          # expect: 0c5fc86 (or descendant if intervening commits)
  git status --short                  # expect: empty
  ```
- [ ] **Step 3.1.2: Confirm baseline test counts.**
  ```bash
  go test ./... 2>&1 | tee phase2b-baseline.log
  grep -c "^--- FAIL" phase2b-baseline.log     # expect: 4 (Phase 2a INT-1 ACCEPT-PARTIAL adds TestEncode_LSPVectorBitExact to the baseline FAIL set per closure report §C4)
  grep -c "^--- SKIP" phase2b-baseline.log     # expect: 3 (unchanged from Phase 2a entry)
  ```
  The 4 FAILs MUST be exactly:
  1. `TestEncode_LSPVectorBitExact` (root, Phase 2a INT-1 ACCEPT-PARTIAL)
  2. `TestDiagnostic_SinglePulseChain` (`internal/decoder`)
  3. `TestDecode_LowEnergyCodebookIsSmooth` (`internal/gain`)
  4. `TestDecode_SucceedsAcrossAllGainIndices` (`internal/gain`)

  Any other FAIL is a regression and blocks entry — invoke E1.

### 3.2 Reusable symbols inventory

| Symbol | Location | Reuse posture for Phase 2b |
|---|---|---|
| `lpc.Analyzer.Analyze(*[240]int16, *[11]int16) error` | `internal/lpc/types.go` | **Reuse unchanged** — Phase 2b consumes `aQ12` produced by INT-0 step 3 wiring. |
| `lsp.LPToLSP`, `lsp.LSPToLSF`, `lsp.Quantize`, `lsp.InitLSPOld`, `lsp.InitFreqPrev`, `lsp.ErrLPCNonStable` | `internal/lsp/*.go` | **Reuse unchanged** — Phase 2a-pinned encoder LSP surface. |
| `lspToLP(lsp, a)` | `internal/lsp/lsp_lp.go` | **Reuse for the Annex A `Â(z/γ)` weighting branch** — converts the quantized LSP back to Q12 LP for the §A.3.3 weighting filter. Note: this is the existing decoder-side helper; per Phase 2a §1.2 it was marked "reused but optional"; Phase 2b *requires* it. |
| `pcm.PreProcessor.Process` | `internal/pcm/*.go` | Already invoked from `lpcStep` (`encoder.go:115`). Phase 2b consumes the *same* `processed[80]` output, NOT a re-processed copy. |
| `internal/fixed.{Mult, MultR, LMac, LMsu, Add, Sub, ShrR, Norm32, Div32}` | `internal/fixed/*.go` | Reuse for all Q-format arithmetic. |
| `internal/filter.Weighting` | `internal/filter/types.go` | Currently a §3.3-shaped stub (base-codec adaptive γ₁/γ₂ — wrong shape for Annex A). Phase 2b **does NOT use this stub**; it lives under `internal/pitch/openloop/lowpass.go` (or sibling `internal/filter/annexa.go` — see §4). The base-codec stub is left undisturbed for a future Phase 2-final cleanup or removed by INT-0 step 5 if dead. |
| `internal/pitch.{Adaptive, Delay, Parity}` | `internal/pitch/*.go` | **DO NOT IMPORT** from encoder open-loop code per I10 precedent. Decoder pitch state is reconstruction-side; encoder open-loop search is independent. |

### 3.3 New symbols to introduce

| New symbol | Package | Purpose | Spec § |
|---|---|---|---|
| `(constant) Gamma` (Q15) | `internal/pitch/openloop` | γ = 0.75 in Q15 = 24576 | §A.3.3 line 2063 |
| `(constant) LPFCoeff` (Q15) | `internal/pitch/openloop` | 0.7 in Q15 = 22938 (for `(1 − 0.7z⁻¹)`) | §A.3.3 line 2071 |
| `gammaWeightLP(a, aWeighted *[11]int16)` | `internal/pitch/openloop` | Computes `a'_i = â_i · γⁱ` (Q12 in, Q12 out) | §A.3.3 eq. A.2 LP-domain |
| `combineWith07(aGamma, aPrime *[11]int16)` | `internal/pitch/openloop` | Convolves `Â(z/γ) · (1 − 0.7z⁻¹)` to produce A'(z) coefficients | §A.3.3 line 2071 |
| `lpResidual(aHat *[11]int16, speech *[80]int16, mem *[10]int16, residual *[80]int16)` | `internal/pitch/openloop` | Computes eq. A.3 r(n) = s(n) + Σ â_i·s(n−i) over the 80-sample frame | §A.3.3 eq. A.3 |
| `lowpassWeightedSpeech(aPrime *[11]int16, residual *[80]int16, mem *[10]int16, sw *[80]int16)` | `internal/pitch/openloop` | Computes eq. A.2 Sw(n) = r(n) − Σ a'_i·sw(n−i) | §A.3.3 eq. A.2 |
| `correlate(wsp *[223]int16, kMin, kMax int) (int16, int32)` | `internal/pitch/openloop` | Decimated correlation eq. A.4 over a single range; returns `(bestLag, bestRSquared)` | §A.3.4 eq. A.4 |
| `energy(wsp *[223]int16, k int) int32` | `internal/pitch/openloop` | Σ sw²(2n − k), n=0..39, with overflow normalization | §A.3.4 eq. A.5 denominator |
| `pickBestInRange(wsp *[223]int16, kMin, kMax int) (lag int16, rsq int32, energy int32)` | `internal/pitch/openloop` | Wraps `correlate` + `energy` for one range; for range [80,143] applies the even-first + ±1 refinement (§A.3.4 lines 2112–2114) | §A.3.4 |
| `submultipleLift(t1, r1, e1, t2, r2, e2, t3, r3, e3 …) int16` | `internal/pitch/openloop` | Three-range merger with sub-multiple lift rule per §A.3.4 lines 2109–2111. **Concrete arithmetic per OQ-1.** | §A.3.4 |
| `Search(wsp *[223]int16) int16` | `internal/pitch/openloop` | Top-level package API: takes the 80-current + 143-history concatenated buffer, returns T_op ∈ [20,143] | §A.3.4 |
| `(*Encoder).openloopStep(processed *[80]int16, aHatQ12 *[11]int16) int16` | root `encoder.go` | Wires the §A.3.3 + §A.3.4 chain into the encoder; advances `oldWspeech[143]` per I-2b-2; returns T_op | §A.3.3 + §A.3.4 |

`internal/pitch/openloop` package: **no exported state struct** (the perceptual weighting filter memories live on the encoder, passed in by pointer per the Phase 2a "encoder owns cross-frame state" doctrine).

---

## 4. Package-layout decision

**Chosen:** `internal/pitch/openloop/` (new sibling sub-package under `internal/pitch/`).

**One-line justification:** Mirrors Phase 2a's encoder-vs-decoder isolation pattern (decoder LSP in `internal/lsp/*.go`, encoder LSP in `internal/lsp/encoder_*.go`) at the package level instead of the file level, because (a) the decoder pitch package `internal/pitch/` already has its own focused decoder API (`Adaptive`, `Delay`, `Parity`), (b) Phase 2c will add a sibling `internal/pitch/closedloop/` for the fractional closed-loop search, and (c) the §A.3.3 low-pass weighted-speech filter has no decoder-side counterpart and would not naturally live in `internal/pitch/`.

Alternatives considered and rejected:

- `internal/pitch/` directly with `encoder_openloop_*.go` files (Phase 2a `internal/lsp/encoder_*.go` precedent). Rejected: closed-loop will follow in 2c and the file count grows quickly; sub-package keeps the scope narrow.
- `internal/encoder/openloop/` (a new `internal/encoder/` super-package). Rejected: would require a much larger refactor than Phase 2b warrants, and the existing `internal/lpc` / `internal/lsp` / `internal/acelp` structure does not co-locate by encoder/decoder.
- Putting the low-pass weighting filter under `internal/filter/` and the search under `internal/pitch/`. Rejected: splits a tightly-coupled §A.3.3 + §A.3.4 chain across packages and would entangle the filter with the existing base-codec `Weighting` stub (which is the wrong shape for Annex A — see §3.2 reusable-symbols table).

---

## 5. Task ledger

> **Convention:** every code-producing task follows the 5-step (test → fail → impl → pass → commit) cycle. All commits include the I8 trailer.

### Task family WS — perceptual-weighted speech (§A.3.3)

#### Task 2b-WS-1: γ-weighted LP coefficients + A'(z) construction

**Files:** Create `internal/pitch/openloop/doc.go`, `internal/pitch/openloop/weighting.go`, `internal/pitch/openloop/weighting_test.go`.

- [ ] **Step 1: Write failing test** with three inputs:
  1. `â = [4096, 0, 0, …, 0]` (Q12, identity LP) → `a' = [4096, 0, 0, …, 0]` after `gammaWeightLP`, then `[4096, −22938, 0, …, 0]` after `combineWith07` (the `(1 − 0.7z⁻¹)` factor alone, since `Â(z/γ) = 1`).
  2. `â = [4096, −2048, 0, …, 0]` (Q12, single-tap) → `gammaWeightLP` returns `[4096, −1536, 0, …, 0]` (since γ¹·(−2048) = 0.75·(−2048) = −1536), then `combineWith07` convolves with `(1, −0.7)` → `[4096, −24474, 1075, …, 0]` (verify by hand-multiply).
  3. Frame-0 `â` produced by Phase 2a's `lpcStep` on PITCH.IN frame 0 → captured as a `t.Logf` characterisation; promoted to `t.Errorf` after INT-0 wiring confirms the chain.
- [ ] **Step 2: Run to verify FAIL** (`undefined: gammaWeightLP, combineWith07`).
- [ ] **Step 3: Write minimal implementation:**
  - `gammaWeightLP`: `aWeighted[0] = a[0]; for i = 1..10: aWeighted[i] = fixed.Mult(a[i], gammaPow[i])` where `gammaPow[i] = γⁱ` precomputed as a Q15 LUT (`var gammaPow = [11]int16{32767, 24576, 18432, ...}`). Document the recurrence `gammaPow[i+1] = round(gammaPow[i] · γ / 32768)`.
  - `combineWith07`: 11-tap FIR convolution of input with `(32767, −22938)` in Q15, producing 12 taps; truncate to 11 (the highest-degree tap is dropped per §A.3.3 — A'(z) is order-10, not order-11, because the LPF factor adds one degree but is then folded into the existing 10th-order representation per the spec narrative). **OQ-2: verify with a one-pass diagnostic on PITCH.IN frame 0 that the truncation matches expected residual energy; if not, restructure as an order-11 representation.**
- [ ] **Step 4: Run to verify PASS** on the synthetic inputs; characterisation log on frame 0.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.3 lines 2063 (γ=0.75), 2071 (A'(z) construction).
**Q-format:** input `a` Q12 with `a[0]=4096`; `gammaPow` Q15; output `aWeighted` / `aPrime` Q12 with `[0]=4096`.

**Commit message:**
```
feat(pitch/openloop): Phase 2b-WS-1 γ-weighted LP and A'(z) for §A.3.3

Adds gammaWeightLP and combineWith07 for the Annex A §A.3.3 perceptual
weighting chain. γ=0.75 (Q15 24576) and (1 − 0.7z⁻¹) factor per
eq. A.2; γⁱ LUT precomputed in Q15 with hand-traced two-tap test.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-WS-2: LP residual + low-pass weighted speech + `oldWspeech[143]` slide-by-80

**Files:** Create `internal/pitch/openloop/lowpass.go`, `internal/pitch/openloop/lowpass_test.go`. Extend `encoder.go` only at INT-0 (not in this task).

- [ ] **Step 1: Write failing test** with three inputs:
  1. Zero speech, zero memory → r(n) = 0 ∀n, sw(n) = 0 ∀n.
  2. Impulse speech `s = [4096, 0, …, 0]` Q0 with identity Â and zero memory → r(n) = s(n) (since Σ â_i·s(n−i) = 0 for i≥1 and s(n)=0 for n≥1); then sw(n) = r(n) for the same reason.
  3. Buffer-slide test: pre-load `oldWspeech` with a synthetic ramp `[0,1,2,…,142]`, run `slideOldWspeech(oldWspeech, freshSw)` with `freshSw = [200,201,…,279]`, assert `oldWspeech[0:63] == [80,81,…,142]` (old [80..143)) and `oldWspeech[63:143] == [200,201,…,279]`.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:**
  - `lpResidual`: `r[n] = s[n] + Σ_{i=1..10} fixed.Mult(â[i], s[n−i])` where `s[n−i]` falls back to a 10-sample input-history `mem` for `n<i`. Q-format: input `s` Q0, â Q12, product Q12 with implicit shift via `fixed.Mult`; sum saturated to int16. **NOTE on Q-format mismatch:** `fixed.Mult(Q12, Q0) → Q12` if treated as Q15·Q0; the residual is intended to live in Q0 alongside `s`. The exact shift convention is spec-silent and must be pinned by experiment in INT-0; document the choice with a comment citing §A.3.3 line 2080 verbatim.
  - `lowpassWeightedSpeech`: `sw[n] = r[n] − Σ_{i=1..10} fixed.Mult(a'[i], sw[n−i])`, same Q-format and history-fallback discipline.
  - `slideOldWspeech(old *[143]int16, fresh *[80]int16)`: in-place `copy(old[0:63], old[80:143]); copy(old[63:143], fresh[:])` per I-2b-2.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.3 eq. A.2 (lines 2073–2076), eq. A.3 (lines 2079–2081); buffer convention from §A.3.4 line 2097 (max lag k=143).
**Q-format:** s, r, sw all int16 Q0 (within saturating-Add discipline of `internal/fixed`); mem arrays match.
**I-2b-2 binding:** the slide test is the explicit pin.

**Commit message:**
```
feat(pitch/openloop): Phase 2b-WS-2 LP residual + low-pass weighted speech

Implements §A.3.3 eq. A.2/A.3: r(n) = s(n) + Σ â_i·s(n-i) and
Sw(n) = r(n) − Σ a'_i·sw(n-i), 80 samples per frame, with caller-owned
10-sample history memories. slideOldWspeech advances the encoder's
[143]int16 history by 80 per frame (mirrors oldSpeech slide ordering
from Phase 2a; pinned by §A.3.4 max-lag=143).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task family OL — open-loop search (§A.3.4)

#### Task 2b-OL-1: Decimated correlation kernel for one lag range

**Files:** Create `internal/pitch/openloop/correlate.go`, `internal/pitch/openloop/correlate_test.go`.

- [ ] **Step 1: Write failing test** with three inputs over the concatenated `wsp[0..222]` view (143 history + 80 current):
  1. All-zero `wsp` → `correlate(wsp, 80, 143)` returns (anyLag, R²=0).
  2. `wsp[k]` = δ(k − 143) (impulse at the boundary between history and current) → R(k) for k=143 isolated to a single product term.
  3. Periodic `wsp` with period 80 → assert that `correlate(wsp, 80, 143)` returns lag = 80 (the period) with maximum R².
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:**
  ```go
  // R(k) = Σ_{n=0..39} sw(2n) * sw(2n - k)   (eq. A.4)
  // Operating on the concatenated wsp[0..222] where wsp[143..222] = current
  // 80 samples and wsp[0..142] = history. The "n=0" of eq. A.4 refers to
  // the first sample of the current frame, i.e. wsp[143]. So:
  //   sw(2n)     = wsp[143 + 2n]      for n=0..39  (range covers indices 143..221)
  //   sw(2n - k) = wsp[143 + 2n - k]  for n=0..39, k ∈ [20,143]
  func correlate(wsp *[223]int16, kMin, kMax int) (lag int16, rsq int32) {
      for k := kMin; k <= kMax; k++ {
          var acc int32
          for n := 0; n < 40; n++ {
              acc = fixed.LMac(acc, wsp[143+2*n], wsp[143+2*n-k])
          }
          // For range comparison we need R(k)·|R(k)| (signed-squared) per eq. A.5
          // intent — Phase 2b OL-3 documents the signed-vs-unsigned choice.
          if /* candidate stronger */ { … }
      }
      return …
  }
  ```
  Signed-vs-unsigned R(k) handling: §A.3.4 leaves this implicit; pin to "negative R(k) is treated as R(k)=0 for selection purposes" per the Phase 2a I1 discipline of citing the spec verbatim and documenting any operational choice as such.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 eq. A.4 (lines 2089–2092).
**Q-format:** wsp Q0 int16; R(k) Word32 in Q0² ≡ Q0 with implicit ×2 product semantics of `fixed.LMac`.

**Commit message:**
```
feat(pitch/openloop): Phase 2b-OL-1 decimated correlation kernel

Implements eq. A.4 R(k) = Σ_{n=0..39} sw(2n)·sw(2n-k) for one lag
range. Returns (bestLag, bestR²) over [kMin, kMax]. Three tests
cover zero, impulse, and period-80 inputs.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-OL-2: Energy normalization with overflow scaling

**Files:** Extend `internal/pitch/openloop/correlate.go`, add `internal/pitch/openloop/energy_test.go`.

- [ ] **Step 1: Write failing test:**
  1. Zero wsp → energy(k) = 0 ∀k.
  2. Constant wsp[i] = 1024 → energy(k) = 40 · 1024² = 41943040 for any k (40 even-indexed taps).
  3. Saturation case: wsp filled with ±32767 alternating → energy must not overflow Word32; assert returned value is the post-shift representation and `energyShift` is set so `energy << shift` reconstructs the unscaled value.
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** `energy(wsp, k)` accumulates `Σ_{n=0..39} wsp[143+2n−k]²` in Word32 with `fixed.Norm32`-driven post-shift; document the shift policy. To keep R²/E ratio comparable across lags within a range, **the same shift must be applied to R² when computing the comparison metric**; implement that as a sibling helper `compareNormalized(rsq1, e1, rsq2, e2) bool` that does a cross-multiplicative comparison `rsq1·e2 vs rsq2·e1` to avoid the explicit divide.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 eq. A.5 (lines 2102–2107).

**Commit message:**
```
feat(pitch/openloop): Phase 2b-OL-2 energy normalization for eq. A.5

Adds energy(k) = Σ sw²(2n-k) and compareNormalized for cross-
multiplicative R²/E comparison without divide. Overflow handling
via shared right-shift; saturation test pinned with ±32767 input.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-OL-3: Best-of-range selection (per range, with §A.3.4 even-first refinement for [80,143])

**Files:** Extend `internal/pitch/openloop/correlate.go`, add `internal/pitch/openloop/best_in_range_test.go`.

- [ ] **Step 1: Write failing test:**
  1. Synthetic wsp where the best lag in [20,39] is known (period 25) → `pickBestInRange(wsp, 20, 39)` returns 25.
  2. Same for [40,79] (period 64).
  3. For [80,143]: synthetic wsp where the best EVEN lag is 110 and the best OVERALL lag is 109 → `pickBestInRange(wsp, 80, 143)` returns 109 after the ±1 refinement around the even winner 110. (§A.3.4 line 2113–2114.)
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:**
  - For ranges [20,39] and [40,79]: full-stride scan via `correlate` + `compareNormalized`.
  - For range [80,143]: scan only even k ∈ {80, 82, …, 142}, find the even winner k_even, then test k_even − 1, k_even, k_even + 1 (clamping to [80,143]) and pick the best of those 3 by `compareNormalized`.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 lines 2094–2097 (range definitions), 2112–2114 (even-first + ±1 refinement).

**Commit message:**
```
feat(pitch/openloop): Phase 2b-OL-3 per-range best-lag selection

pickBestInRange handles all three §A.3.4 ranges. [80,143] uses the
Annex A even-first scan + ±1 refinement around the even winner per
lines 2113-2114; [20,39] and [40,79] use full-stride scans.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-OL-4: Three-range merger with sub-multiple lift rule

**Files:** Extend `internal/pitch/openloop/correlate.go`, add `internal/pitch/openloop/merge_test.go`.

- [ ] **Step 1: Write failing test:**
  1. Three ranges all return identical normalized correlation → merger picks the shortest-range winner (range [20,39]) per the §A.3.4 lines 2109–2111 "favouring the lower delay range" rule.
  2. Range [80,143] strongly dominates and the other two are weak → merger returns the [80,143] winner.
  3. Range [20,39] returns lag 30, range [40,79] returns lag 60 (= 2·30, exact sub-multiple), range [80,143] returns lag 90 (= 3·30) → merger lifts the 30 by the sub-multiple bonus and selects 30 as T_op. **The exact bonus arithmetic is OQ-1 (see §9); the test value is parameterised against a constant `subMultipleLift` declared in the impl with a `// OQ-1 binding constant` comment.**
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** see OQ-1 closure path. **Default first-pass implementation:** start `T_op = t1` (range [20,39] winner); if `R'(t2)` is within a tunable bonus of `R'(T_op)·multiple_factor` AND `t2` is a near-multiple of `t1`, override; same for `t3`. Tunable constants live as named `const` so they can be re-pinned post-INT-1 without touching algorithm shape.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 lines 2109–2111.

**Commit message:**
```
feat(pitch/openloop): Phase 2b-OL-4 three-range merger with sub-multiple lift

Combines the three §A.3.4 range winners into the final T_op. Sub-
multiple lift rule per lines 2109-2111 (informal in spec; binding
constants live as named consts pending OQ-1 closure).

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-OL-5: Package-level `Search` API

**Files:** Extend `internal/pitch/openloop/openloop.go` (new top-level entry file), add `internal/pitch/openloop/search_test.go`.

- [ ] **Step 1: Write failing test:** for a synthetic `wsp[223]` constructed to have a clear period-100 oscillation in the current 80 samples and a flat history → `Search(&wsp)` returns 100 (or 99/101 if the ±1 refinement nudges it).
- [ ] **Step 2: Run to verify FAIL**.
- [ ] **Step 3: Write minimal implementation:** wraps `pickBestInRange(_, 20, 39)`, `pickBestInRange(_, 40, 79)`, `pickBestInRange(_, 80, 143)` and feeds the three `(lag, rsq, energy)` triples to `submultipleLift` to return T_op.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 (full clause).

**Commit message:**
```
feat(pitch/openloop): Phase 2b-OL-5 Search top-level entry

Search(wsp *[223]int16) int16 composes OL-3 + OL-4 into the §A.3.4
open-loop pitch entry point. Returns T_op ∈ [20,143].

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

### Task family INT — encoder integration + gates

#### Task 2b-INT-0: Wire `openloopStep` into `Encoder`

**Files:** Modify `encoder.go` (add `openloopStep` method; surface `aQ12` from `lpcStep` either via an extended return signature or via an `Encoder.aQ12Latest [11]int16` cached field; pick the latter to preserve the existing `lpcStep` signature). Add a tiny `internal/pitch/openloop/integration_smoke_test.go` driving one synthetic frame end-to-end.

- [ ] **Step 1: Write failing test** at the package-internal level: feed a 1-second sine wave at 200 Hz (period = 40 samples) through `NewEncoder()` 100 times calling `lpcStep` then `openloopStep`; assert (a) returned T_op converges to a value near 40 within the first 5 frames, (b) zero allocations across the loop body, (c) `e.oldWspeech` is not all-zero after the first frame.
- [ ] **Step 2: Run to verify FAIL** (`undefined: (*Encoder).openloopStep`).
- [ ] **Step 3: Write minimal implementation:**
  ```go
  func (e *Encoder) openloopStep(processed *[FrameSamples]int16, aHatQ12 *[11]int16) int16 {
      var aGamma, aPrime [11]int16
      openloop.GammaWeightLP(aHatQ12, &aGamma)
      openloop.CombineWith07(&aGamma, &aPrime)

      var residual, freshSw [80]int16
      openloop.LPResidual(aHatQ12, processed, &e.lpResidualMem, &residual)
      openloop.LowpassWeightedSpeech(&aPrime, &residual, &e.swMem, &freshSw)

      var wsp [223]int16
      copy(wsp[:143], e.oldWspeech[:])
      copy(wsp[143:], freshSw[:])

      top := openloop.Search(&wsp)

      openloop.SlideOldWspeech(&e.oldWspeech, &freshSw)
      return top
  }
  ```
  Add new `Encoder` fields: `lpResidualMem [10]int16`, `swMem [10]int16`. Both initialized to zero by `NewEncoder` / `Reset`. Document in field-block comment that they are §A.3.3 filter memories owned by the root coordinator per the I10/Phase-2a state-isolation doctrine.
  Surface `aHatQ12` from `lpcStep` by adding a private method `(*Encoder).quantizedLPCoefficients(qHatQ15 *[10]int16) [11]int16` that calls `lsp.LSPToLP` on the (post-quantization) `qHatQ15`; Phase 2a's `lpcStep` already produces `qQ15` (the *unquantized* LSP) and the *quantized* LSP must be reconstructed by `lsp.combineResidual(L1, L2, L3, _)` + MA-predictor application. This is the only structural addition to `lpcStep`. Alternative: extend `lpcStep`'s return tuple to `(lsp.Indices, [11]int16, error)` and let the caller pass it to `openloopStep`. **Default to the tuple-extension form** as it keeps state on the stack and out of `Encoder`.
- [ ] **Step 4: Run to verify PASS**.
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.3 + §A.3.4.

**Commit message:**
```
feat(g729): Phase 2b-INT-0 wire openloopStep into Encoder

Adds (*Encoder).openloopStep that consumes the quantized LP
coefficients from the prior lpcStep call, runs the §A.3.3 low-pass
weighted-speech chain, advances oldWspeech[143] per I-2b-2 slide
ordering, and returns T_op ∈ [20,143] via openloop.Search.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-INT-1: ITU PITCH.IN consistency gate

**Files:** Add `pitch_itu_vector_test.go` at root (mirror of `lsp_itu_vector_test.go`). Create `testdata/phase2b/openloop_top_capture.csv` artefact path (gitignored if large; checked-in if compact).

- [ ] **Step 1: Write failing test** `TestEncode_OpenLoopPitchConsistency`:
  1. Verify presence of `testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.{IN,BIT}` (E10 if missing).
  2. Determine PITCH.IN framing convention (293628 B = 146814 samples ≠ 1835·80 = 146800 — the 14-sample residual must be reconciled). Document the resolution in the test setup.
  3. For each of 1835 frames: feed 80 samples through `NewEncoder()` calling `lpcStep` → `openloopStep` → `T_op`. Assert (a) `T_op ∈ [20,143]`, (b) no panic, (c) `T_op` is consistent with the P1 bit field of PITCH.BIT in the sense that `int(P1_decoded) ∈ [T_op − 5, T_op + 4]` for at least 80 % of frames (plausibility, not byte-EQ — Phase 2c owns the strict gate).
  4. Persist per-frame `T_op` to the capture artefact for Phase 2c cross-checking.
- [ ] **Step 2: Run to verify FAIL** (initially, openloopStep undefined or capture file missing).
- [ ] **Step 3: Write minimal implementation:** wire the test loop; surface any framing-convention findings as testdata documentation.
- [ ] **Step 4: Run to verify PASS** (or invoke E10 / I5 on shortfall).
- [ ] **Step 5: Commit.**

**Spec cite:** §A.3.4 (algorithm); §A.4 / §A.3.7 (P1 closed-loop search window for the plausibility check).

**Expected vs measured gate:** **consistency only** at Phase 2b. Strict byte-EQ gate is Phase 2c's responsibility. Phase 2b INT-1 disposition: PASS if (a)+(b)+(c) above hold.

**I5 budget posture:** 0/5 consumed at INT-1 entry. Sub-multiple-lift OQ-1 is the most likely I5 sink — the spec is genuinely vague here and the first PASS rate against the plausibility check may require 2–3 iterations on `subMultipleLift` constants. Each iteration consumes 1/5.

**Commit message:**
```
test(g729): Phase 2b-INT-1 PITCH.IN open-loop pitch consistency gate

TestEncode_OpenLoopPitchConsistency runs the encoder LPC + open-loop
chain on all 1835 frames of PITCH.IN, asserting T_op ∈ [20,143] and
≥80% plausibility against the PITCH.BIT P1 field within the §A.3.7
±[-5,+4] closed-loop search window.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-INT-2: Zero-alloc + race + benchmarks

**Files:** Add `phase2b_int2_openloop_zeroalloc_test.go` at root; add benchmarks under `internal/pitch/openloop/bench_test.go` for `Search`, `correlate`, `energy`, `lpResidual`, `lowpassWeightedSpeech`, `combineWith07`, `gammaWeightLP`.

- [ ] **Step 1: Write failing tests** asserting `AllocsPerRun(128, ...) == 0` on:
  - `(*Encoder).openloopStep` standalone.
  - `lpcStep + openloopStep` end-to-end for one frame.
- [ ] **Step 2: Run to verify FAIL** if any leak surfaces.
- [ ] **Step 3: Hoist any leaked scratch** onto `Encoder` as a fixed-size field per Phase 2a I4 precedent.
- [ ] **Step 4: Run `go test ./... -race`** and verify zero new race reports beyond the documented baseline FAIL set.
- [ ] **Step 5: Commit.**

**Expected output (per Phase 2a precedent):** every benchmark reports `0 B/op, 0 allocs/op`. Race-detector clean.

**Commit message:**
```
test(g729): Phase 2b-INT-2 zero-alloc + race gate on openloopStep

Pins I4: AllocsPerRun(128, openloopStep) == 0 and same on the
lpcStep + openloopStep composition. Race-detector clean across
internal/pitch/openloop and root.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

#### Task 2b-INT-3: Phase 2b closure report

**Files:** Author `docs/superpowers/plans/YYYY-MM-DD-phase2b-closure-report.md`.

- [ ] Capture: full task ledger (WS-1/2, OL-1..5, INT-0..2 status), files added, INT-1 disposition (consistency PASS / shortfall details), per-frame T_op statistics (min/max/mean/distribution), I5 budget consumed, race + alloc table, OQ-1/OQ-2 closure status, hand-off notes to Phase 2c.

**Commit message:**
```
docs(phase2b): Phase 2b-INT-3 closure report

Captures final task ledger, INT-1 plausibility gate disposition, T_op
distribution across PITCH.IN, I5 budget accounting, OQ-1/OQ-2
closures, and hand-off notes (oldExc[154], synMem/wMem/errMem
inheritance) for Phase 2c closed-loop pitch.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

---

## 6. Per-task contract summary

| Task | Input files | ITU §-cite | Q-format invariants | Test name | Acceptance |
|------|-------------|------------|---------------------|-----------|------------|
| WS-1 | (new package) | §A.3.3 lines 2063, 2071 | aQ12 in/out, gammaPow Q15 | `TestGammaWeightLP_*`, `TestCombineWith07_*` | 3 synthetic inputs PASS |
| WS-2 | WS-1 | §A.3.3 eq. A.2/A.3 | s,r,sw all Q0; mem Q0 | `TestLPResidual_*`, `TestLowpassWeightedSpeech_*`, `TestSlideOldWspeech_*` | 3 inputs PASS |
| OL-1 | WS-2 | §A.3.4 eq. A.4 | wsp Q0; rsq Word32 | `TestCorrelate_*` | 3 inputs PASS |
| OL-2 | OL-1 | §A.3.4 eq. A.5 | energy Word32 with shared shift | `TestEnergy_*`, `TestCompareNormalized_*` | 3 inputs PASS incl. ±32767 saturation |
| OL-3 | OL-2 | §A.3.4 lines 2094–2097, 2112–2114 | per-range lag int16 | `TestPickBestInRange_*` | 3 inputs PASS, [80,143] hits ±1 refinement |
| OL-4 | OL-3 | §A.3.4 lines 2109–2111 | T_op int16 | `TestSubmultipleLift_*` | 3 inputs PASS, OQ-1 constants pinned |
| OL-5 | OL-4 | §A.3.4 (full) | wsp Q0 → T_op int16 | `TestSearch_*` | 1 synthetic input PASS |
| INT-0 | OL-5 | §A.3.3 + §A.3.4 | encoder.aHatQ12 Q12 | `TestEncoder_OpenLoopStep_Smoke` | 100-frame sine 200 Hz converges to T_op ≈ 40 |
| INT-1 | INT-0 | §A.3.4 + §A.3.7 | T_op ∈ [20,143] | `TestEncode_OpenLoopPitchConsistency` | 1835/1835 frames, ≥80% plausibility vs P1 |
| INT-2 | INT-1 | (engineering) | n/a | `TestNoAllocationInOpenLoopStep`, `TestNoAllocationInLPCStepPlusOpenLoop` | 0 allocs/op; race clean |
| INT-3 | INT-2 | (closure) | n/a | n/a | report authored |

---

## 7. Output contract for INT gates

**INT-1 output table (mirrors Phase 2a INT-1 closure-report shape):**

| Metric | Target | Acceptance |
|---|---|---|
| Frames processed | 1835 | =1835 |
| Frames with T_op ∈ [20,143] | 1835 | =1835 |
| Frames with `int(P1) ∈ [T_op−5, T_op+4]` (plausibility) | ≥1468 (80%) | report value + per-frame divergence trace if <80% |
| Allocations on `openloopStep` (per frame) | 0 | =0 |
| Allocations on `lpcStep + openloopStep` (per frame) | 0 | =0 |
| Race-detector status | clean | clean |
| I5 consumed | 0/5 → ≤5/5 | report value |

**INT-2 alloc/bench table (mirror of Phase 2a INT-2-d):**

| Symbol | ns/op (informational) | B/op | allocs/op |
|---|---:|---:|---:|
| `openloop.GammaWeightLP` | TBD | 0 | 0 |
| `openloop.CombineWith07` | TBD | 0 | 0 |
| `openloop.LPResidual` | TBD | 0 | 0 |
| `openloop.LowpassWeightedSpeech` | TBD | 0 | 0 |
| `openloop.Correlate` (one range) | TBD | 0 | 0 |
| `openloop.Energy` (one range) | TBD | 0 | 0 |
| `openloop.PickBestInRange` ([80,143]) | TBD | 0 | 0 |
| `openloop.Search` (full) | TBD | 0 | 0 |
| `(*Encoder).openloopStep` | TBD | 0 | 0 |
| `lpcStep + openloopStep` end-to-end | TBD | 0 | 0 |

---

## 8. I5 budget tracker

**INT-1 starting balance: 0/5.**

| Slot | Status | Triggering hypothesis | Resolution / disposition |
|---|---|---|---|
| 1 | available | (likely OQ-1 sub-multiple lift constants tuning) | TBD at INT-1 |
| 2 | available | — | TBD |
| 3 | available | — | TBD |
| 4 | available | — | TBD |
| 5 | available | — | TBD |

On exhaust: invoke **E9** (Phase 2a precedent — freeze production, write `phase2b_int1_*_diagnostic_test.go` measurement-only test, escalate to user with hypothesis-family table). Phase 2a 1/5 preserved I5 slot is *not* available for Phase 2b — that slot is reserved for Phase 2-final per closure-report §8 line 226.

---

## 9. Open questions / risks

### OQ-1: §A.3.4 "sub-multiple lift" arithmetic is spec-vague

**Spec text (§A.3.4 lines 2109–2111):** *"The winner among the three normalized correlations is selected by favouring the delays with the values in the lower range. This is done by augmenting the normalized correlations corresponding to the lower delay range if their delays are submultiples of the delays in the higher delay range."*

**The spec does NOT specify:**
1. The numeric "augmentation" factor (Phase 2a base-codec §3.4 uses an explicit 0.85 threshold; Annex A drops the threshold and leaves the lift unspecified).
2. The "submultiple" tolerance (exact integer division? ±1 sample slack? ratio-based?).
3. Tie-break between range-1 and range-2 when both are sub-multiples of range-3.

**Closure path:** OL-4 implements the lift with **named `const` parameters** so the algorithm shape is committed first and constants are tuned at INT-1. If 80% plausibility against P1 is hit on the first attempt, OQ-1 closes as ACCEPT; if not, up to 4/5 of the I5 budget is available for constant re-tuning before E9 / ACCEPT-PARTIAL closure.

### OQ-2: §A.3.3 A'(z) order — 10 or 11?

The convolution `Â(z/γ) · (1 − 0.7z⁻¹)` is mathematically order-11. The spec (§A.3.3 line 2071) writes A'(z) without specifying its order; eq. A.2 sums `Σ_{i=1..10} a'_i·sw(n−i)` which implies order-10. **Two valid readings:**

(a) Truncate the highest-degree term of the convolution → order-10.
(b) Compute the full order-11 representation and use it in eq. A.2 with the upper bound raised to 11.

WS-1 step 3 defaults to (a). If INT-1 plausibility fails and tracing isolates the low-pass-filter shape as the cause, swap to (b) under one I5 slot.

### OQ-3: PITCH.IN framing convention

`PITCH.IN` is 293628 B = 146814 samples; `PITCH.BIT` is 1835 frames; 1835 × 80 = 146800 samples. The 14-sample residual is unaccounted for. Possibilities: (a) trailing partial frame discarded, (b) leading look-ahead samples not framed, (c) header bytes (unlikely — file is .IN, not .BIN). INT-1 step 2 must reconcile before promoting any consistency assertion.

### Risk: Phase 2a residual L1/L2/L3 disagreement may bleed into Phase 2b inputs

Phase 2a INT-1 closed ACCEPT-PARTIAL with L1 byte-EQ at 38.93 % (50× chance). The *quantized* LP coefficients `âQ12` consumed by Phase 2b §A.3.3 are derived from these indices. A different L1/L2/L3 choice produces a different Â(z), which produces different sw(n), which produces different T_op. **Phase 2b INT-1 plausibility threshold (80 %) is set deliberately loose to absorb this upstream variance.** A stricter byte-EQ gate is a Phase 2c concern.

### Risk: `internal/filter.Weighting` stub diverges from Annex A

The existing `internal/filter/types.go` describes "Weighting holds the perceptual weighting filter memory (§3.3)" — base-codec shape with adaptive γ₁/γ₂. Phase 2b does NOT use this stub (per §3.2 reusable-symbols inventory) and should NOT modify it. A Phase 2-final cleanup may either (a) repurpose it for Annex A or (b) delete it. Out of scope for Phase 2b.

---

## 10. Inheritance to Phase 2c (closed-loop pitch)

**State carry from Phase 2b → 2c:**

- `Encoder.oldWspeech[143]` — populated by every `openloopStep`; consumed by Phase 2c §A.3.6 target computation (the low-pass weighted speech is reused as the closed-loop target source per §A.3.6 line 2120).
- `Encoder.lpResidualMem[10]`, `Encoder.swMem[10]` (NEW in Phase 2b) — perceptual-weighting filter memories for the next frame; consumed by Phase 2c only if the closed-loop chain needs to re-filter (it does NOT — Phase 2c uses the existing residual r(n) directly per eq. A.3 + the Phase 2b cached output).
- `Encoder.aQ12Latest [11]int16` (introduced in INT-0 if the tuple-extension path is rejected) — Phase 2c reads it for the closed-loop search h(n) construction (§A.3.5).
- `Top` (T_op) — passed by Phase 2c as the centre of the §A.3.7 closed-loop fractional search range.

**Files / packages already touched at Phase 2b closure:**

- `internal/pitch/openloop/{doc.go, weighting.go, lowpass.go, correlate.go, openloop.go}` plus their test files.
- `internal/pitch/openloop/bench_test.go`.
- `encoder.go` — adds `openloopStep` private method, `lpResidualMem`, `swMem` fields, optional `aQ12Latest` field.
- `pitch_itu_vector_test.go` (root, INT-1).
- `phase2b_int2_openloop_zeroalloc_test.go` (root, INT-2).

**Phase 2c entry preconditions:**

- All Phase 2b acceptance criteria satisfied (T_op consistency PASS, INT-2 zero-alloc + race clean, INT-3 closure report authored).
- `Encoder.oldExc[154]int16` is **NOT** populated at Phase 2b closure (adaptive-codebook excitation buffer is a Phase 2c responsibility).
- `Encoder.synMem[10]`, `wMem[10]`, `errMem[10]` remain zeroed; first use is in Phase 2c §A.3.6 target computation.

---

## 11. Self-review

Reviewed against:

1. **I1 clean-room.** All citations point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` (the bundled ITU-T G.729 (06/2012) recommendation) or to our own prior plans/docs. **No URL / file path of any external G.729 implementation appears in this plan.** ✅
2. **TDD discipline.** Every code-producing task (WS-1/2, OL-1..5, INT-0/1/2) has the 5-step (test → fail → impl → pass → commit) checklist. ✅
3. **I8 Co-author trailer.** Every commit message stub includes the trailer. ✅
4. **Annex A binding (I-2b-1).** Every algorithm task cites §A.3.x line ranges; §3.3/§3.4 are referenced as informational only. ✅
5. **`oldWspeech[143]` slide ordering (I-2b-2).** WS-2 step-1 test 3 is the explicit pin; carried to INT-0 via `slideOldWspeech`. ✅
6. **Scope.** Plan does not implement code; only describes it. ✅

---

## 12. Execution handoff

**Recommended next dispatch:** execute Phase 2b Task 2b-WS-1 (γ-weighted LP coefficients + A'(z) construction) per §5 of this plan. Open the next session with:

> "Execute Task 2b-WS-1 from `docs/superpowers/plans/2026-05-07-phase2b-open-loop-pitch-plan.md` per the 5-step TDD checklist. Stop after the commit and report back for dispatch of Task 2b-WS-2."

Sub-skill: `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`.
