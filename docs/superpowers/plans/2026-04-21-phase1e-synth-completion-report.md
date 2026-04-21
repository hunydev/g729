# Phase 1e Completion Report — internal/synth

**Date:** 2026-04-21
**Plan:** `docs/superpowers/plans/2026-04-21-phase1e-synth.md`
**Status:** ✅ Complete (11/11 tasks)

---

## 1. Spec sections referenced

- ITU-T G.729 §3.10 — LP synthesis filter (decoder side)
- ITU-T G.729 §4.1.2 — Synthesis filter recurrence + 10-sample state memory
- ITU-T G.729 §4.1.6 — Excitation composition `u(n) = g_p·v(n) + g_c·c(n)` (eq. 75)
- ITU-T G.729 §4.3 — First-frame initialisation (zero-state)

No ITU reference C, bcg729, Sipro Lab, or any other existing G.729 implementation
was consulted for algorithmic code. Constants come from the spec only.

---

## 2. Plan deviations

### 2.1 `TestSynthesize_StatePropagatesAcrossSubframes` — coefficient changed

**Plan, line 1164:**
```go
a := [11]int16{4096, 2000, 0, ...}   // a_1 = 0.488
v1[0] = 4000
```

**Implemented:**
```go
a := [11]int16{4096, 4000, 0, ...}   // a_1 = 0.977
v1[0] = 4000
```

**Why.** With `a_1 = 2000` (≈0.488), the IIR decay rate is so fast that
`4000 · 0.488^N` falls below 1 LSB by `N ≈ 13`. By the end of subframe 1
(N = 39) the signal has been quantised to 0 for ~25 samples, so
`pastSynth` carries an all-zero state into subframe 2 — `s2` is
legitimately all-zero, and the test (which asserts `s2` non-zero to prove
state propagation) fails for a reason unrelated to state-propagation
correctness.

`a_1 = 4000` (≈0.977) keeps the magnitude sequence of `s1[30..39]`
clearly in the 100s–1000s range, so a non-zero `s2` is a meaningful
witness of state propagation. The IIR is still strictly stable
(|a_1| < 1).

This is a test-only adjustment; no production code differs from the plan.

### 2.2 No other deviations

- `Q-format of the code-gain contribution` — followed the plan as written
  (11-bit down-shift). No tolerance/precision issue surfaced in any of
  the unit tests; the real bit-exact check is queued for Phase 1g.
- `Round` half-LSB direction — plan's ±1 LSB tolerances absorbed all
  observed results.
- No heap escape encountered. `var u [40]int16` inside `Synthesize` and
  `var work [50]int16` inside `filterSubframe` both stay on the stack;
  `TestNoAllocationInSynthesize` and `TestNoAllocationInBuildExcitation`
  pass with `0 allocs/op` on first run.
- No two-pass overflow guard introduced. None of the existing tests
  required it.

### 2.3 Co-author trailer

Plan suggested `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`.
Per repository convention all commits use:
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.

---

## 3. Benchmark results

Host: AMD EPYC 9554P 64-Core Processor (linux/amd64), `go test -bench=. -benchmem -run=^$ ./internal/synth/`.

```
BenchmarkBuildExcitation-2     6,413,994     187.6 ns/op     0 B/op   0 allocs/op
BenchmarkSynthesize-2          1,437,874     769.3 ns/op     0 B/op   0 allocs/op
BenchmarkFilterSubframe-2      2,076,852     582.6 ns/op     0 B/op   0 allocs/op
```

All three primitives meet the zero-allocation contract.
`Synthesize ≈ BuildExcitation + FilterSubframe` (188 + 583 ≈ 771 ns) — no
hidden overhead in the wiring. The 769 ns/subframe figure leaves ample
budget within the 5 ms (40-sample at 8 kHz) real-time window.

---

## 4. Open items for Phase 1g

1. **Bit-exact ITU test-vector validation.** All numerical decisions in
   §3.10 / §4.1.6 (rounding direction, saturation behaviour at
   intermediate stages, code-gain Q-format down-shift) are validated
   structurally by unit tests but have not been compared against the
   ITU reference test vectors. Phase 1g must run end-to-end with the
   official `algthm.in` / `algthm.bit` pairs and confirm exact match.

2. **11-bit down-shift on code-gain contribution.** `BuildExcitation`
   loses 11 bits of `gcQ12` precision when aligning Q26 → Q15. If
   Phase 1g shows audible degradation or bit-mismatch tied to the
   fixed-codebook path, options are (a) revising `internal/gain` to
   emit `gcQ1` instead, or (b) widening the alignment to Q21 with a
   compensating `LShl` on the pitch half. Document the choice.

3. **Two-pass overflow guard for `1/A(z)`.** The synthesis filter uses a
   single direct-form pass. ITU-T G.729 §3.10 mentions an alternative
   two-pass formulation that re-runs with scaled inputs if `LShl(L_temp, 3)`
   saturates. None of the current unit tests trip the saturation
   threshold; verify with the full ITU test-vector suite, and add the
   two-pass guard if any vector overflows.

4. **First-frame initial conditions.** Phase 1e initialises `pastSynth`
   to all zeros via the zero-value `Synthesizer{}` (and `Reset()`).
   Confirm Annex A does not specify a different initial state for the
   reduced-complexity decoder.

5. **Postfilter coupling (Phase 1f).** The synthesis output `s[n]` is
   the pre-postfilter signal. Phase 1f (Annex A postfilter) will read
   `s[n]` and the same `a[]` coefficients; the call interface designed
   here (`Synthesize(a, v, c, gp, gc, s)`) must compose cleanly with
   the postfilter's input contract.

---

## 5. Commit list (oldest → newest, 11 commits)

```
23804b4 feat(synth): package skeleton + Synthesizer type with Reset
4ff5ce1 feat(synth): BuildExcitation pitch contribution per ITU §4.1.6
8002746 feat(synth): add fixed codebook contribution to BuildExcitation
3c9bbb4 test(synth): saturation coverage for BuildExcitation extremes
8c9d99c feat(synth): LP synthesis filter direct-form skeleton per ITU §3.10
257196a test(synth): lock 1st-order impulse response for LP synthesis filter
8fe3a21 test(synth): lock past-state feedback and state update for LP synthesis
86128ad feat(synth): Synthesize public entry composes excitation + filter
29c3fc0 test(synth): lock Reset determinism and two-subframe state propagation
6ba90c6 test(synth): lock zero-allocation on BuildExcitation, Synthesize, Reset
512b05f test(synth): lock per-subframe benches; polish package doc
```

Each commit is task-scoped and self-contained per the plan's TDD
discipline. All commits carry the
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
trailer.

---

## 6. Completion criteria — verification

| Criterion | Result |
|---|---|
| All 11 tasks checked off in plan | ✅ |
| `go test -race ./...` passes | ✅ all 9 packages `ok` |
| `go vet ./...` silent | ✅ no output |
| `BenchmarkBuildExcitation` 0 allocs/op | ✅ 0 B/op, 0 allocs/op |
| `BenchmarkSynthesize` 0 allocs/op | ✅ 0 B/op, 0 allocs/op |
| `BenchmarkFilterSubframe` 0 allocs/op | ✅ 0 B/op, 0 allocs/op |
| 11 commits on `main` for Phase 1e in task order | ✅ |
| Completion report saved | ✅ this file |

---

## 7. Files added in Phase 1e

```
internal/synth/doc.go              package documentation
internal/synth/types.go            Synthesizer struct + Reset
internal/synth/excitation.go       BuildExcitation
internal/synth/filter.go           filterSubframe (private)
internal/synth/synthesizer.go      Synthesize public entry
internal/synth/synthesizer_test.go Synthesizer + Synthesize tests
internal/synth/excitation_test.go  BuildExcitation tests (pitch/code/sat)
internal/synth/filter_test.go      filterSubframe tests
internal/synth/alloc_test.go       zero-allocation lock
internal/synth/bench_test.go       benchmarks
```

Phase 1f (Annex A postfilter) is now ready to plan.
