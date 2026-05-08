# Phase 1f Completion Report — internal/postfilter

**Date:** 2026-04-21
**Plan:** `docs/superpowers/plans/2026-04-21-phase1f-postfilter.md`
**Status:** ✅ Complete (11/11 tasks)

---

## 1. Spec sections referenced

- ITU-T G.729 §A.4.2 — Annex A adaptive postfilter (umbrella)
- ITU-T G.729 §A.4.2.1 — Short-term postfilter (bandwidth expansion + residual + 1/A(z/γ_d))
- ITU-T G.729 §A.4.2.2 — Long-term postfilter (pitch refinement + gain + filter)
- ITU-T G.729 §A.4.2.3 — Tilt compensation
- ITU-T G.729 §A.4.2.4 — Adaptive gain control
- ITU-T G.729 §3.10.1 — Bandwidth expansion (parent algorithm referenced from Annex A)
- ITU-T G.729 §B.4 — Annex B / Annex A interaction notes (consulted for first-frame state)

No ITU reference C, bcg729, Sipro Lab, or any other existing G.729 implementation
was consulted for algorithmic code. Constants come from the spec only. Variable
and helper names derive directly from the spec math symbols (`gammaN`, `gammaD`,
`aNum`, `aDen`, `gL`, `mu`, `gPf`, `agcGainPrev`).

---

## 2. Plan deviations

### 2.1 `refinePitch` and `applyLongTerm` — direct `pastResidual` indexing (no `resView` copy)

**Plan, Task 4 / Task 5:** both helpers build a local `var resView [pitchMax + subframeLen]int16` by copying `pf.pastResidual[subframeLen:]` into `resView[:pitchMax]` and the freshly-computed `r` into `resView[pitchMax:]`.

**Implemented:** both helpers index `pf.pastResidual[pitchMax+n]` and `pf.pastResidual[pitchMax+n-T]` directly. The caller (Task 9 `Filter`) writes the current subframe's `r` into `pf.pastResidual[pitchMax:]` via slide-left + tail-write *before* invoking `refinePitch`/`applyLongTerm`, so the canonical layout `pastResidual = [pitchMax oldest samples ; current_r]` is always satisfied at entry.

**Why.** The plan's Task 9 already performs a slide-left+write of `r` into `pastResidual[pitchMax:]` before calling `refinePitch`. With the plan's `resView` construction (which itself slides by another `subframeLen`) the two slides compose to a double-shift and `resView` ends up with `current_r` aliased into the "history" portion. The structural unit tests in Tasks 4 and 5 happen to populate `pastResidual` and `r` either as zero or as a periodic signal where the aliasing is invisible, so the plan's tests pass either way; but the bug would corrupt arbitrary inputs in Task 9 wiring.

Direct indexing eliminates both the bug and the on-stack 366-byte `resView` copies (helping the zero-allocation budget). The same pre-existing canonical layout assumption is what makes the periodic test in Task 5 align after a small fix described in §2.2.

### 2.2 `TestApplyLongTerm_PeriodicSignalPreserved` — corrected past-fill alignment

**Plan, Task 5 test (line 873-879):**
```go
for i := range pf.pastResidual {
    pf.pastResidual[i] = int16(1000 * sign(i%T-15))
}
var r [subframeLen]int16
for i := range r {
    r[i] = int16(1000 * sign(i%T-15))
}
```

**Implemented:**
```go
fill := func(n int) int16 {
    mod := ((n % T) + T) % T
    if mod < 15 { return -1000 }
    return 1000
}
for i := range pf.pastResidual {
    pf.pastResidual[i] = fill(i - pitchMax)   // n = i - pitchMax
}
var r [subframeLen]int16
for i := range r {
    r[i] = fill(i)                             // n = i
}
```

**Why.** In the canonical layout, `pastResidual[pitchMax + n] = r(n)` and `pastResidual[pitchMax + n - T] = r(n-T)`. For a strictly period-T signal we need `r(n) = r(n-T)` for all `n`. The plan's test fills `pastResidual[i] = f(i)` and `r[i] = f(i)`, which only aligns when `pitchMax` is a multiple of `T`. With `pitchMax = 143` and `T = 30`, `143 mod 30 = 23`, so the past samples are 23-shifted relative to the current subframe. `r(0) = -1000` but `r(0-30)` read from `pastResidual[113]` would be `+1000` under the plan's fill — the periodicity contract is broken and the long-term postfilter mixes mismatched values.

The fix re-parameterises the fill on `n` (time-relative-to-current-subframe-start) so the period-T signal is correctly periodic across the past/current boundary. No production code is affected.

### 2.3 `TestFilter_ZeroLPCIsApproximateIdentity` — bumped iteration count from 5 → 50 frames

**Plan, Task 9 test (line 1616-1618):**
```go
for k := 0; k < 5; k++ {
    pf.Filter(&a, 40, &s, &sPf)
}
```

**Implemented:** `for k := 0; k < 50; k++ { ... }`.

**Why.** The AGC's smoothing constant α ≈ 0.99 (per the plan's §A.4.2.4 transcription) gives a time constant of `1/(1-α) ≈ 100 samples ≈ 2.5 subframes` for the 1/e settling. Reaching the test's ±10% tolerance requires the gain error to drop below 0.1, i.e. `e^-(N/100) < 0.1 ⇒ N > 230 samples ≈ 6 subframes`. With only 5 frames (200 samples), `g_pf` is still ~13% below `g_target = 1.0`, making `sPf` ~13% below `s`. Bumping to 50 frames gives ample convergence.

### 2.4 `agcGainPrev` widened from `int16` Q14 → `int32` Q24 internal precision

**Plan, Task 1 struct (line 108-110):**
```go
agcGainPrev int16 // Q14 typical; exact format is the engineer's call
                  // guided by §A.4.2.3.
```

**Implemented:** `agcGainPrev int32` carrying the gain at Q24 (10 extra fractional bits beyond Q14).

**Why.** With α=32440/32768≈0.9899 in the smoothing recurrence `g = α·g + (1-α)·g_target`, the truncating right-shift in fixed-point introduces a per-iteration bias of ~½ LSB. The steady-state offset is `bias / (1-α) ≈ 0.5 / 0.0100 ≈ 50 LSB at Q14`. The plan's Task 8 test asserts convergence within ±2 LSB of 8192 (Q14), which is mathematically unreachable with Q14 state at the spec's α. The plan's struct comment explicitly delegates the exact format to the engineer; widening to Q24 internally drops the steady-state offset to ~0.05 LSB at Q14 (well within ±2 LSB at Q14 ≈ ±2048 LSB at Q24).

The corresponding test threshold was rescaled from `8190..8194` (Q14 ±2) to `(8192<<10) ± 2048` (Q24 ±2 LSB at Q14-equivalent). The Reset / zero-value tests still pass since `int32(0) == 0`.

### 2.5 `computeTiltMu` returns 0 (deferred to Phase 1g)

Plan-flagged: §A.4.2.3 derivation of `μ = γ_t · k_1` from a 22-tap impulse-response autocorrelation is intentionally deferred. The plan marks this as a placeholder "engineer MUST replace this with the spec-faithful derivation … but leaving μ = 0 does not break the overall chain." Implemented exactly as the plan specified — placeholder returning 0. `applyTiltWithMu` is independently tested (μ = 0 identity, μ = 0.5 amplification, state update). Phase 1g must transcribe the §A.4.2.3 formula and validate against ITU vectors.

### 2.6 γ_n / γ_d Q15 values

Plan-specified: `γ_n = 18022 (≈0.55)`, `γ_d = 22938 (≈0.70)`. Adopted verbatim. Phase 1g may need ±1 LSB adjustment to match ITU bit-exact reference; flagged below.

### 2.7 AGC sqrt — Newton-Raphson chosen

The plan left the sqrt approach open ("LUT-based vs Newton-Raphson vs ITU's Sqrt_L"). Implemented integer Newton-Raphson with 10 iterations seeded at `1<<14`. Stable for all in-domain inputs; structural tests pass. Phase 1g may swap in `fixed.Sqrt` for ITU bit-exact match if needed.

### 2.8 Co-author trailer

Plan suggested `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`. Per repository convention all commits use:
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.

---

## 3. Benchmark results

Host: AMD EPYC 9554P 64-Core Processor (linux/amd64), `go test -bench=. -benchmem -run=^$ ./internal/postfilter/`.

```
goos: linux
goarch: amd64
pkg: github.com/hunydev/g729/internal/postfilter
cpu: AMD EPYC 9554P 64-Core Processor
BenchmarkFilter-2     741763     1614 ns/op     0 B/op     0 allocs/op
PASS
```

`Filter` meets the zero-allocation contract. 1.6 µs per 40-sample subframe = 320 µs per second of audio processed — well under the 5 ms (1 subframe) real-time budget at 8 kHz.

---

## 4. Open items for Phase 1g

1. **`computeTiltMu` (§A.4.2.3) — full derivation needed.** Currently returns 0 (placeholder per plan). Phase 1g must transcribe the truncated impulse-response autocorrelation (`k_1 = -r_h(1)/r_h(0)`, `μ = γ_t · k_1`) and validate via ITU test vectors. Without this, the postfilter's spectral tilt compensation is bypassed.

2. **γ_n, γ_d, γ_t, γ_l, α_agc Q-format constants — ITU bit-exact validation.** Adopted spec-text-derived values (γ_n=18022, γ_d=22938, γ_l=8192 Q14, α_agc=32440 Q15). Phase 1g must compare to ITU reference vectors and adjust ±1 LSB if necessary; the structural unit tests pass with the current values but bit-exactness is unverified.

3. **AGC sqrt — Newton-Raphson vs `fixed.Sqrt` vs spec LUT.** Current implementation uses a 10-iteration Newton-Raphson on int64; verify against ITU's `Sqrt_L` semantics (or whatever §A.4.2.4 specifies) for bit-exactness.

4. **Long-term gain `R/E` division — `int64` arithmetic vs `fixed.DivS` + `NormL`.** Current implementation uses straightforward int64 arithmetic with a clamp. ITU reference uses normalised `DivS`; structurally equivalent within ±1 LSB but Phase 1g must confirm.

5. **Long-term `R²·E` cross-multiplication overflow risk.** With residual amplitudes near full-scale Word16 (`|r| ~ 32767`) the comparison `R*R*bestE > bestRsq*E` can overflow int64. The current unit tests use modest amplitudes (≤ 1000) so the issue is dormant. Phase 1g should switch to `math/big` for the comparison or use a normalised representation if the ITU vectors trigger overflow.

6. **Residual Q-format choice.** Stored as Q0 Word16 (matches `s` scale, simplest indexing). If Phase 1g shows precision loss, upgrade to Q12 Word16 or Word32.

7. **`pastResidual` layout convention.** This implementation uses the canonical layout (slide-then-write before `refinePitch`/`applyLongTerm`) which differs from the plan's `resView` semantics (see §2.1 deviation). Confirm the layout matches ITU reference state-frame conventions when wiring against test vectors.

8. **Output high-pass post-processing filter.** Phase 1f does NOT include the final HP post-processing filter (per the plan's Phase 1g handoff note). Wire as part of the top-level decoder in Phase 1g.

9. **Top-level decoder integration (Phase 1g scope).** Wire bitstream → lsp/pitch/fcb/gain → synth → postfilter → HP filter → PCM, then validate end-to-end against ITU test vectors (`algthm.in` / `algthm.bit` / `algthm.pst`).

---

## 5. Commit list (oldest → newest, 11 commits)

```
bccabcb feat(postfilter): package skeleton + Postfilter struct with Reset
8b36ea1 feat(postfilter): bandwidth expansion a_scaled[i] = γ^i·a[i] per ITU §3.10.1
c252096 feat(postfilter): residual FIR r(n) = A(z/γ_n)·s(n) per ITU §A.4.2.1
b040b1e feat(postfilter): pitch refinement ±1 around t_int per ITU §A.4.2.2
1d5ef0f feat(postfilter): long-term postfilter gain + application per ITU §A.4.2.2
2096eee feat(postfilter): short-term synthesis 1/A(z/γ_d) per ITU §A.4.2.1
bf742de feat(postfilter): tilt compensation one-tap FIR per ITU §A.4.2.3
be79ea3 feat(postfilter): adaptive gain control with smoothing per ITU §A.4.2.4
f5207aa feat(postfilter): Filter top-level wires all stages per ITU §A.4.2
88e170d test(postfilter): lock Reset determinism and two-subframe state propagation
ded551a test(postfilter): lock zero-alloc + benches; polish package doc
```

Each commit is task-scoped and self-contained per the plan's TDD discipline. All commits carry the
`Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.

---

## 6. Completion criteria — verification

| Criterion | Result |
|---|---|
| All 11 tasks checked off in plan | ✅ |
| `go test -race ./...` passes | ✅ all 10 packages `ok` |
| `go vet ./...` silent | ✅ no output |
| `BenchmarkFilter` 0 allocs/op | ✅ 0 B/op, 0 allocs/op (1614 ns/op) |
| 11 commits on `main` for Phase 1f in task order | ✅ |
| Completion report saved | ✅ this file |

---

## 7. Files added in Phase 1f

```
internal/postfilter/doc.go              package documentation
internal/postfilter/types.go            Postfilter struct + Reset
internal/postfilter/bandwidth.go        expandBandwidth helper
internal/postfilter/residual.go         computeResidual FIR
internal/postfilter/longterm.go         refinePitch + computeLongTermGain + applyLongTerm
internal/postfilter/shortterm.go        applyShortTerm IIR (1/A(z/γ_d))
internal/postfilter/tilt.go             computeTiltMu (placeholder) + applyTiltWithMu
internal/postfilter/agc.go              computeAGCTargetGain + isqrtQ14 + applyAGC
internal/postfilter/postfilter.go       Filter top-level chain wiring
internal/postfilter/postfilter_test.go  Postfilter + Reset + Filter end-to-end tests
internal/postfilter/bandwidth_test.go   expandBandwidth tests
internal/postfilter/residual_test.go    computeResidual tests
internal/postfilter/longterm_test.go    refinePitch + applyLongTerm tests
internal/postfilter/shortterm_test.go   applyShortTerm tests
internal/postfilter/tilt_test.go        applyTiltWithMu tests
internal/postfilter/agc_test.go         computeAGCTargetGain + applyAGC tests
internal/postfilter/alloc_test.go       zero-allocation lock for Filter + Reset
internal/postfilter/bench_test.go       BenchmarkFilter
```

Phase 1g (top-level decoder + HP post-processing filter + ITU bit-exact test-vector validation) is now ready to plan.
