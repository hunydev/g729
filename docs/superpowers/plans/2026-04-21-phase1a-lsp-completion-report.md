# Phase 1a (LSP Decoder) — Completion Report

**Date:** 2026-04-21
**Plan:** `docs/superpowers/plans/2026-04-21-phase1a-lsp.md`
**Status:** ✅ All 14 tasks complete; all completion criteria met.

---

## Commits (one per task, in order)

```
09301da chore: ignore ITU test vectors and spec PDF working copies     (pre-Task 1 housekeeping)
<task1>  feat(lsp): package skeleton for LSP decoder                    (combined into earlier setup)
7635e61 feat(tables): add L1 LSF codebook from ITU §3.2.4 Table 7       (Task 2)
272b92e feat(tables): add L2 LSF codebook from ITU §3.2.4 Table 8       (Task 3)
de13837 feat(tables): add L3 LSF codebook from ITU §3.2.4 Table 9       (Task 4)
8d18c7a feat(tables): add MA LSF predictor coefficients from ITU §3.2.4 Table 6   (Task 5)
73441d7 feat(tables): add cosine LUT from ITU §3.2.5                    (Task 6)
4e6d139 feat(lsp): split-VQ residual combiner (L1 + L2/L3)              (Task 7)
2a0bd67 feat(lsp): order-4 MA predictor with past-residual state        (Task 8)
6edff5e feat(lsp): LSF stability enforcement (monotonic + min gap)      (Task 9)
edc5505 feat(lsp): LSF→LSP conversion via cosine LUT                    (Task 10)
8c57f02 feat(lsp): per-subframe LSP interpolation between frames        (Task 11)
a127c7e feat(lsp): Chebyshev LSP→LP polynomial expansion                (Task 12)
159355c feat(lsp): wire Decoder pipeline end-to-end                     (Task 13)
c4d3b52 test(lsp): lock zero-alloc Decode + per-frame bench; polish doc (Task 14)
```

(Task 1's package-skeleton commit landed earlier in the same session under `feat(lsp): package skeleton …`; the SHAs above include all 13 subsequent feature commits.)

---

## Completion Criteria — Verified

| Criterion | Result |
|---|---|
| `go test -race ./internal/lsp/... ./internal/tables/...` | ✅ PASS (19 tests in `lsp`, all `tables` tests) |
| `go vet ./internal/lsp/... ./internal/tables/...` | ✅ silent |
| `BenchmarkDecode` zero-alloc | ✅ **0 B/op, 0 allocs/op @ 610.6 ns/frame** |
| One commit per task | ✅ |
| Plan checkboxes flipped | ✅ all `- [x]` |

```
BenchmarkDecode-2   2160015   610.6 ns/op   0 B/op   0 allocs/op
```

---

## Spec sections referenced

| Code | Spec section |
|---|---|
| `internal/tables/lsp_l1.go`, `lsp_l2.go`, `lsp_l3.go` | §3.2.4 Tables 7–9 (codebook indices) |
| `internal/tables/lsp_ma.go` | §3.2.4 Table 6 (MA predictor coefficients) |
| `internal/tables/lsp_cos.go` | §3.2.5 (cosine LUT) |
| `internal/lsp/codebook.go` | §3.2.4 eq (19) — split-VQ combine |
| `internal/lsp/predictor.go` | §3.2.4 eq (20) — switched MA predictor |
| `internal/lsp/stability.go` | §3.2.4 — pre-predictor pair-rearrangement (J=0.0012/0.0006); §3.2.4 post-predictor stability (sort, edges 0.005/3.135, gap 0.0391) |
| `internal/lsp/lsf_lsp.go` | §3.2.5 — q_i = cos(ω_i) |
| `internal/lsp/interpolate.go` | §4.1.2 — per-subframe LSP interpolation |
| `internal/lsp/lsp_lp.go` | §3.2.6, §4.1.6 — Chebyshev LSP→LP |
| `internal/lsp/decoder.go` | §3.2.4–§3.2.6, §4.1.1–§4.1.2 — full LSP pipeline |

---

## Deviations from the plan (with rationale)

### 1. Cosine LUT range (Task 6 / Task 10)

- **Plan said:** LUT covers `[0, π/2]` with 65 entries, monotone positive.
- **Spec actually says (§3.2.5 Table 8 of the C distribution / `tab_ld8a.c` `table[65]`):** LUT covers the **full range `[0, π]`** with 65 entries. Index 0 = +32767 (cos 0), index 32 = 0 (cos π/2), index 64 = -32768 (cos π).
- **Decision:** follow the spec. No quadrant fold is needed in `lsfToLSP`, which simplifies the algorithm.
- **Constants used in `lsfToLSP`:**
  - `lspStep = 402` (= floor(π_Q13 / 64) = floor(25736 / 64))
  - `lspMaxOmega = 25728` (clamp upper edge before division)
  - Linear interp: `c0 + ((c1 − c0) · frac) / lspStep`

  The plan's `step = 201` (matching the half-range LUT) and the suggested `halfPi = 12868` were not used because they assume a different LUT.

### 2. Stability function — two-pass spec model (Task 9)

- **Plan said:** single forward pass, looking only at minimum gap (`minGap = 10`, i.e. `J = 0.0012` rad).
- **Spec actually has two separate passes:**
  - **Pre-predictor** pair-rearrangement on the residual l̂ (§3.2.4): J = 0.0012 then J = 0.0006, applied via `(l̂_i + l̂_{i-1} ± J) / 2`.
  - **Post-predictor** stability on ω̂ (§3.2.4 enumerated rules): sort, lower edge 0.005, minimum adjacent gap 0.0391, upper edge 3.135.
- **Decision:** Implement **both** for spec fidelity:
  - `enforceLSFStability(lsf)` — post-predictor: insertion-sort + clamp + min-gap with back-propagated upper clamp.
  - `rearrangeAdjacent(lsf, J)` — pre-predictor pair-rearrangement; called twice in `Decode` with `lsfRearrJ1=10` and `lsfRearrJ2=5`.
- **Q13 constants used:** `lsfMinEdge=41`, `lsfMinGap=320`, `lsfMaxEdge=25682`, `lsfRearrJ1=10`, `lsfRearrJ2=5`.
- **Test impact:** the plan's `TestStabilityTooClose` expected `minGap = 10` but I used `minGap = 320` (the spec's post-predictor value). Because the test inputs use gaps of 1 (well below either threshold), it still passes with the stricter constant.

### 3. Chebyshev LSP→LP (Task 12)

- Implementation matches the plan's derivation; no spec deviation. Constants used:
  - F1/F2 accumulator in **Q28** Word32 (init `1 << 28`).
  - Per-step shift in `polyStep`: `(int64(q) * int64(fPrev1)) >> 14` for `2·q·f_prev1` in Q28 (derivation: Q15·Q28 → Q43, ·2 → Q44, → Q28 needs `>>14`).
  - Final A-assembly: `(F1[k] + F1[k-1] + F2[k] − F2[k-1])` then **rounded right shift by 17** with bias `1<<16`.
  - Saturation to Word16 at the end.

### 4. Past-residual initialization (Task 8 / Task 13)

- **Spec says (§3.2.4):** "the initial values of l̂_i(k) are given by l̂_i = i·π/11 for all k < 0".
- **Plan implementation of `Reset()`:** zeros the FIFO.
- **Decision:** zero-init keeps `Reset()` semantically the zero value; populate `iπ/11` lazily on the first `Decode` call via an `initialized` flag and a `var initialPastResidual = [10]int16{2340, 4679, …, 23396}` Q13 constant. This matches the spec's intent without breaking the "zero value is valid Reset state" invariant the rest of the codebase relies on.

### 5. Interpolation saturation (Task 11)

- **Plan suggested:** `fixed.Shr(fixed.Add(prev[i], curr[i]), 1)` (saturated add then shift).
- **Test failed because:** saturated `Add` clipped sums above 32767 *before* the divide-by-2, producing the wrong midpoint for large LSP values.
- **Fix used:** widen to int32: `int16((int32(prev[i]) + int32(curr[i])) >> 1)`. This is allocation-free and correct because the average of two Word16 values always fits in Word16.

---

## Test tolerances

No test tolerances were relaxed. All plan-supplied tests pass as-written, plus the additional `TestRearrangeAdjacent*` tests for the pre-predictor pass.

---

## Licensing posture (carried forward from the user's option-A decision)

- Algorithmic code in `internal/lsp` was written from the ITU-T G.729 specification PDF and Annex A text only.
- Numerical table values in `internal/tables/lsp_*.go` were transcribed from the **data-array initializers** of `tab_ld8a.c` in the ITU reference distribution. Per the merger doctrine, the bitstream-interoperability requirement leaves no creative space in those numerical constants. Each table file carries this disclaimer in its doc comment.
- **No** other ITU C source file (`pred_lt3.c`, `lpcfunc.c`, `qua_lsp.c`, etc.) was opened, read, or referenced. The quarantine path containing these files is `/home/exedev/g729-itu-software-quarantine/` and is outside the repo.
- No bcg729, no Sipro Lab, no other G.729 implementation was consulted.

---

## Files added / modified

**Created (algorithmic code):**
- `internal/lsp/codebook.go`, `predictor.go`, `stability.go`, `lsf_lsp.go`, `interpolate.go`, `lsp_lp.go`, `bench_test.go`, `alloc_test.go`
- `internal/lsp/codebook_test.go`, `predictor_test.go`, `stability_test.go`, `lsf_lsp_test.go`, `interpolate_test.go`, `lsp_lp_test.go`

**Created (data tables):**
- `internal/tables/lsp_l1.go`, `lsp_l2.go`, `lsp_l3.go`, `lsp_ma.go`, `lsp_cos.go`
- `internal/tables/lsp_tables_test.go`

**Modified:**
- `internal/lsp/decoder.go` — full pipeline + lazy iπ/11 init + `prevLSP` state
- `internal/lsp/decoder_test.go` — integration tests
- `internal/lsp/doc.go` — pipeline overview rewrite

---

## What's next

Phase 1a is sealed. Subsequent phases (each independently planned and executed):

- **Phase 1b** — pitch / adaptive codebook
- **Phase 1c** — ACELP fixed codebook
- **Phase 1d** — gain VQ
- **Phase 1e** — synthesis filter
- **Phase 1f** — post-filter
- **Phase 1g** — top-level Decoder + first end-to-end ITU test-vector run (`LSP.*`, `SPEECH.*`)
- **Phase 1h** — erasure concealment

Until Phase 1g wires bitstream → LSP → pitch → ACELP → gain → synth → post, the LSP decoder's correctness rests on the property tests in this phase (boundary cases, monotonicity, stability invariants, leading-coefficient identity, allocation contract). Bit-exactness vs. ITU test vectors will be measured then.
