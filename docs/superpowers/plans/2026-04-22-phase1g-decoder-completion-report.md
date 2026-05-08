# Phase 1g Completion Report — internal/decoder

**Date:** 2026-04-22
**Plan:** `docs/superpowers/plans/2026-04-22-phase1g-decoder.md`
**Status:** ⚠ Wiring complete (11/11 tasks), but ITU bit-exact deferred (see §4 / §6)

---

## 1. Spec sections referenced

- ITU-T G.729 §3.2.4, §3.6 — LSP decoding, LSP→LP (consumed via `internal/lsp`)
- ITU-T G.729 §3.7.1 — Adaptive codebook FIR interpolation (consumed via `internal/pitch`; bug fix this phase)
- ITU-T G.729 §3.8 — Fixed codebook + pitch enhancement (consumed via `internal/fcb`)
- ITU-T G.729 §3.9 — Gain-VQ MA predictor (consumed via `internal/gain`)
- ITU-T G.729 §3.10 / §4.1.2 — LP synthesis filter (consumed via `internal/synth`)
- ITU-T G.729 §A.4.2.3 — Tilt compensation `μ` from autocorrelation of `A(z/γ_n)/A(z/γ_d)` impulse response (this phase)
- ITU-T G.729 §4.1.6 — Decoder block diagram / inter-stage data flow (this phase)
- ITU-T G.729 §4.2.2 — Output HP filter (this phase)
- ITU-T G.729 §4.2.3 — Output ×2 amplitude scaling (consumed via `internal/pcm`)
- ITU-T G.729 §4.3 — First-frame initial conditions

No ITU reference C, bcg729, Sipro Lab, or any other existing G.729 implementation
was consulted for algorithmic code. All constants come from the spec PDF; symbol
names derive from spec math (`γ_n`, `γ_d`, `γ_t`, `μ`, `b0..b2`/`a1`/`a2` for HP,
`pastExc`, `pastResidual`, `pastSynth`, `prevGpQ14`).

---

## 2. Plan deviations

### 2.1 `internal/pitch/firInterpolate` — zero-clamp out-of-bounds FIR reads

**Plan:** declared `internal/pitch` "complete from prior phases" and untouched.

**Implemented:** added bounds-check zero-fill inside `firInterpolate` (commit `6850bd4`).

**Why.** The §3.7.1 1/3-sample FIR has Linter=10 forward taps. With the spec-allowed
encoded delay range tInt ∈ [19, 143] × tFrac ∈ {-1, +1}, the forward index
`(len(pastExc)-k) + n + 1 + i` can exceed `len(pastExc)-1` whenever
`tInt < end + Linter` (≈ tInt < 50 in the long-pitch path; *always* in the
short-pitch path). Without clamping, the very first valid `decodeSubframe` call
with such a delay panics. The spec semantics treat the unrealised "future"
samples u(0+) as zero (the adaptive codebook is constructed *before* the
current subframe's u is produced). The fix returns 0 for any `backIdx < 0`
or `fwdIdx >= len(pastExc)` and is independent of the decoder.

### 2.2 `TestDecodeSubframe_TwoCallsDiffer` — uses non-trivial indices

**Plan, Task 5:** test passes all-zero indices `(C=0, S=0, GA=0, GB=0)`.

**Implemented:** uses `(C=0x1FFF, S=0xF, GA=7, GB=15)`.

**Why.** With all-zero indices the computed gain is small enough that the
postfilter's AGC + rounding floors the entire subframe to zero. Both calls
then produce identical zero output despite the underlying state having
advanced. Switching to non-trivial indices generates audible energy and
exercises the FIFO-slide invariant the test is meant to lock in. The test's
intent — *"two back-to-back calls produce different output"* — is preserved.

### 2.3 ITU bit-exact tasks (Tasks 9, 10) — `t.Skip` with diagnostic data

**Plan:** Tasks 9 (ALGTHM) and 10 (SPEECH) require bit-exact match. Plan's
diagnosis loop budgets ±1 LSB nudges of `γ_n`, `γ_d`, `γ_t`, `α_AGC`, sqrt
rounding, long-term division, R²·E overflow, residual Q-format, and
`pastResidual` layout.

**Implemented:** both tests are `t.Skip`'d with detailed root-cause
diagnostics. See §4 for the divergence and §6 for the Phase-1h backlog.

**Why.** Frame-0 subframe-2 of ALGTHM produces `gc=32767` (saturated to
`int16` max) from `gain.Decode` because the fixed-codebook vector decoded by
`fcb.Decode` is identically zero for indices `(C2=6134, S2=15)`, which drives
`log2(0)` through the gain VQ's energy-prediction path. The synthesis filter
then saturates to ±32767 within ~30 samples and the decoder output is three
orders of magnitude larger than ITU expects (≈±5). This is *not* a ±1 LSB
issue — it's structural in pre-Phase-1g packages that lack ITU-vector-level
unit tests. The plan's diagnosis loop is the wrong tool for this magnitude
of divergence; per the plan's explicit guidance ("If the divergence cannot
be closed within reasonable effort … commit the test as `t.Skip()`'d with a
detailed skip reason"), both tests are skipped with the precise first
divergent symptom recorded inline in the test source.

---

## 3. ITU vector verification results

| Vector              | Status      | Tolerance | First divergent point                                                    |
| ------------------- | ----------- | --------- | ------------------------------------------------------------------------ |
| ALGTHM.BIT/.PST     | ⚠ Skipped   | exact     | Frame 0 sample 0: got 0, want 2 (HP/×2 minor). By sample 30+: ±32767. |
| SPEECH.BIT/.PST     | ⚠ Skipped   | exact     | Same root cause as ALGTHM (synth saturation when gc→32767 from log2(0)).|
| ERASURE / PARITY / OVERFLOW / FIXED / LSP / PITCH / TAME / TEST | not attempted Phase 1g | — | Phase 1h tasks. |

**First divergent frame / sample (ALGTHM):** frame 0, sample 0 (off by 2 — HP filter LSB-level rounding).

**First *catastrophic* divergent point:** frame 0 subframe 2 (samples ~40–79). The synthesizer output saturates at +32767 across most of the subframe due to:

- `fcb.Decode((C=6134, S=15), tInt=20, β)` → c[0:40] all zero.
- `gain.Decode((GA=6, GB=2), c=[0..0])` → log2(0) → corrupted predicted log gain → `gc_Q12 = 32767` (int16 max).
- `synth.BuildExcitation(gp_Q14=5498, gc_Q12=32767, v, c)` produces excitation at the int16 ceiling.
- `synth.Filter(sf2A, u, s)` → `s[i]` saturates to 32767 from sample ~5 onwards.

**Suspected stage:** `internal/fcb` (pulse-position decoding for specific C indices) and/or `internal/gain` (defensive handling of zero-energy fixed codebook).

**Smallest hypothesis:** either `fcb.decodePositions` mis-decodes C=6134 (placing all four pulses at the same position so signs cancel) or `internal/gain.fixedCodebookEnergy` silently returns 0 for the all-zero case and `log2Fixed(0)` returns a sentinel that propagates upward instead of being clamped.

**Minimum reproducer:** `TestDebugStages` was used during diagnosis (removed before commit); the failing inputs are recorded above and the root cause is fully reproducible by running `TestDecode_ITUVectorAlgthmBitExact` with the `t.Skip` removed.

---

## 4. Status of each Phase 1f open item

| 1f open item                                  | 1g status      |
| --------------------------------------------- | -------------- |
| Bit-exact `computeTiltMu` (placeholder → 0)   | ✅ Fixed (Task 3, commit `49e5c11`) — 22-tap impulse-response autocorrelation, γ_t = 0.9/0.2 voicing-dependent. |
| §4.2.2 output HP filter                       | ✅ Implemented (Task 4, commit `1d3f356`) — 2-pole 2-zero at 100 Hz, Q13/Q12 fixed-point. |
| AGC sqrt rounding, γ_n/γ_d ±1 LSB nudging     | ⚠ Untested at ITU level (blocked by §3 root cause; deferred to Phase 1h). |
| Long-term division, R²·E overflow, residual Q-format, `pastResidual` layout | ⚠ Same — deferred to Phase 1h alongside per-package ITU vector tests. |

---

## 5. Benchmark numbers (verbatim)

```
$ go test -bench=. -benchmem -run='^$' ./internal/decoder/
goos: linux
goarch: amd64
pkg: github.com/hunydev/g729/internal/decoder
cpu: AMD EPYC 9554P 64-Core Processor
BenchmarkDecode-2   	  138258	      8778 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/hunydev/g729/internal/decoder	1.307s
```

`BenchmarkDecode` is **0 allocs/op** as required by the plan.

Other packages (regression check — all still 0 allocs/op):

```
internal/bitstream  BenchmarkPack-2 / BenchmarkUnpack-2 / BenchmarkParity-2          all 0 B/op 0 allocs/op
internal/fcb        BenchmarkDecode-2                                                 0 B/op 0 allocs/op
internal/fixed      (no benches that allocate)                                        0 B/op 0 allocs/op
internal/gain       BenchmarkDecode-2                          108.7 ns/op            0 B/op 0 allocs/op
internal/lsp        BenchmarkDecode-2                          570.1 ns/op            0 B/op 0 allocs/op
internal/pcm        BenchmarkPreProcessor_ProcessFrame-2       598.3 ns/op            0 B/op 0 allocs/op
internal/pcm        BenchmarkScaleUpSat_Frame-2                145.2 ns/op            0 B/op 0 allocs/op
internal/pitch      BenchmarkAdaptiveCodebookIntegerDelay-2     20.69 ns/op           0 B/op 0 allocs/op
internal/pitch      BenchmarkAdaptiveCodebookFractional-2      1379 ns/op             0 B/op 0 allocs/op
internal/pitch      BenchmarkAdaptiveCodebookShortPitch-2       24.94 ns/op           0 B/op 0 allocs/op
internal/postfilter BenchmarkFilter-2                          2078 ns/op             0 B/op 0 allocs/op
internal/synth      BenchmarkBuildExcitation-2                  192.2 ns/op           0 B/op 0 allocs/op
internal/synth      BenchmarkSynthesize-2                       768.2 ns/op           0 B/op 0 allocs/op
internal/synth      BenchmarkFilterSubframe-2                   584.0 ns/op           0 B/op 0 allocs/op
```

`go test -race ./...` — all 11 packages pass. `go vet ./...` — silent.

---

## 6. Phase 1h backlog (critical-path)

### 6.1 Critical (blocks ITU bit-exact)

1. **Per-package ITU vector unit tests.** Add bit-exact verification for `internal/lsp` (LSP.BIT/.PST), `internal/pitch` (PITCH.BIT/.PST + parity), `internal/fcb` (FIXED.BIT/.PST), `internal/gain` (gain VQ codebook tables + first-frame predictor). The current packages have only structural unit tests; the ALGTHM divergence in §3 cannot be debugged without these.
2. **`fcb.Decode` review.** Frame-0 ALGTHM `(C2=6134, S2=15)` produces an all-zero codebook vector. Either the pulse-position mapping for C=6134 collapses all four pulses to overlapping positions or the sign application is misaligned.
3. **`gain.Decode` zero-energy guard.** `log2Fixed(0)` (or its caller `fixedCodebookEnergy`) needs a defined behaviour for the all-zero-codebook case; today the propagated value drives `gc` to int16 saturation.
4. **`internal/synth` headroom audit.** `LMsu` accumulator at Q13 across 10 taps with `|a[i]| ≤ ~8192` and `|s| ≤ 32767` can reach ~5×10⁹ → Word32 saturation. ITU implementations typically scale `s` by 1/2 internally; spec the convention and align.

### 6.2 Other deferred items

5. **Erasure concealment** (§A.4.1). `Decoder.Decode` accepts the `bad` flag but ignores it; replicate previous LSP/pitch/gain with attenuation per spec.
6. **Parity error handling.** `pitch.CheckParity` is computed but its result is discarded; on parity failure the spec requires using P1 from the previous frame.
7. **Overflow handling.** OVERFLOW.BIT specifically tests that ITU-style saturating arithmetic at every stage matches the reference; some `int32`/`int64` paths in `internal/decoder` need re-audit against the `fixed` saturating primitives.
8. **Public API.** Package `g729` (top of repo) is empty; expose `Decoder`, `Decode`, error types under MIT-friendly names.
9. **Encoder.** Phase 2 — out of scope for 1h but worth noting in roadmap.
10. **Re-enable ITU vectors.** Once 6.1 (1)–(4) are resolved, remove the `t.Skip` from `TestDecode_ITUVectorAlgthmBitExact` and `TestDecode_ITUVectorSpeechBitExact`, add ERASURE/PARITY/OVERFLOW/TAME/TEST counterparts.

---

## 7. Verification table

| Verification step                               | Result                            |
| ----------------------------------------------- | --------------------------------- |
| `go test -race ./internal/decoder/`             | PASS                              |
| `go test -race ./...`                           | PASS (11 packages, 2 t.Skip)      |
| `go vet ./...`                                  | clean (silent)                    |
| `go test -bench=BenchmarkDecode -benchmem ./internal/decoder/` | 0 B/op, 0 allocs/op (8.8 µs/frame) |
| Existing benches (10 packages)                  | all still 0 allocs/op             |
| `internal/decoder/TestDecodeZeroAllocations`    | PASS                              |
| ITU ALGTHM bit-exact                            | SKIP (deferred to Phase 1h, see §3) |
| ITU SPEECH bit-exact                            | SKIP (deferred to Phase 1h, see §3) |

---

## 8. Files added in this phase

- `internal/decoder/doc.go`
- `internal/decoder/types.go`
- `internal/decoder/errors.go`
- `internal/decoder/decode.go`
- `internal/decoder/decode_test.go`
- `internal/decoder/subframe.go`
- `internal/decoder/subframe_test.go`
- `internal/decoder/hpfilter.go`
- `internal/decoder/hpfilter_test.go`
- `internal/decoder/testdata_helpers_test.go`
- `internal/decoder/alloc_test.go`
- `internal/decoder/bench_test.go`

Modified:

- `internal/synth/synthesizer.go` — added exported `Filter` method (Task 2).
- `internal/synth/synthesizer_test.go` — added `TestFilter_*` cases (Task 2).
- `internal/postfilter/tilt.go` — replaced `computeTiltMu` placeholder with bit-exact §A.4.2.3 implementation (Task 3).
- `internal/postfilter/tilt_test.go` — added `TestComputeTiltMu_*` cases (Task 3).
- `internal/pitch/adaptive.go` — bounds-check zero-fill in `firInterpolate` (deviation 2.1).

---

## 9. Full commit list

```
6e0626c test(decoder): zero-alloc Decode + BenchmarkDecode
cff17b2 test(decoder): ITU Annex A SPEECH vector — skipped pending Phase 1h sub-package validation
f81bbfd test(decoder): ITU Annex A ALGTHM vector — skipped pending Phase 1h sub-package validation
9a50a36 test(decoder): ITU Annex A test-vector loader helpers (.bit / .pst)
cd7c795 test(decoder): lock first-frame state init and Reset determinism per ITU §4.3
471ae46 feat(decoder): Decode wires bitstream→lsp→two-subframes→x2 per ITU §4.1.6/§4.2
6850bd4 fix(pitch): zero-clamp out-of-bounds FIR reads in firInterpolate
0533101 feat(decoder): decodeSubframe helper wiring pitch→fcb→gain→synth→post→HP per ITU §4.1.6
1d3f356 feat(decoder): output HP filter 2-pole 2-zero at 100Hz per ITU §4.2.2
49e5c11 feat(postfilter): bit-exact computeTiltMu via impulse-response autocorrelation per ITU §A.4.2.3
2d53a9d feat(synth): expose Filter entrypoint so decoder can observe excitation
f3a9dee feat(decoder): package skeleton + Decoder type with Reset
```

Plus the docs commit (`docs(plans): add Phase 1g completion report`) that follows.
