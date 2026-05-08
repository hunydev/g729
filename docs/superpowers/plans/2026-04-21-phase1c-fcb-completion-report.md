# Phase 1c — `internal/fcb` (ACELP Fixed Codebook) — Completion Report

**Plan:** `docs/superpowers/plans/2026-04-21-phase1c-fcb.md`
**Status:** ✅ All 7 tasks complete. Plan checkboxes flipped.

## Spec sections referenced

| Code unit                       | Spec                                                                        |
| ------------------------------- | --------------------------------------------------------------------------- |
| `decodePositions`               | ITU-T G.729 §3.8 Table 7 (4-pulse track layout, jx selects track-3 half)    |
| `placePulses`                   | ITU-T G.729 §3.8 eq. (45) `c(n) = Σ s_i · δ(n − m_i)`                       |
| `ClampPitchGainForEnhancement`  | ITU-T G.729 §3.8 eq. (47) and §4.1.5 (β = previous ĝ_p clamped to [0.2,0.8])|
| `applyPitchEnhancement`         | ITU-T G.729 §3.8 eq. (46) filter `1/(1 − β·z^−T)`, eq. (48) gating `T < 40` |
| `Decode`                        | §3.8 + §4.1.5 (position → sign → pre-emphasis pipeline)                     |
| Annex A §A.3.8                  | Fixed codebook decoder unchanged from full G.729                            |

No ITU reference C source, bcg729, Sipro Lab, or any other existing G.729 implementation was consulted. Variable names track spec symbols (`P_i`, `S_i`, `c`, `t`, `β`).

## Plan deviations / open items requiring Phase 1g verification

The plan itself flagged three points as needing bit-exact ITU-vector verification at Phase 1g. None of them required code changes during Phase 1c — the implementation matches the plan — but they remain to be checked when test vectors become available:

1. **Sign-bit convention.** Per `placePulses`: bit `(signs >> (3-i)) & 1 == 1` ⇒ `+PulseAmplitude`. The spec text in §3.8 does not pin the encoder bit-mapping convention. If Phase 1g vectors show the opposite mapping, invert in `placePulses` and update `signs_test.go` and `decode_test.go` (only test data needs to change; the structure is unaffected).

2. **β clamp endpoints.** Implemented as `betaLowerQ14 = 3277` (= round(0.2·2¹⁴)) and `betaUpperQ14 = 13107` (= round(0.8·2¹⁴)). The spec gives only the real-valued bounds 0.2 and 0.8; the Q14 rounding choice may differ by ±1 in some implementations. Adjust constants if Phase 1g vectors disagree.

3. **Pitch lag `t` for the enhancement filter.** The decoder's pitch lag is `tInt + tFrac/3` with `tFrac ∈ {-1, 0, 1}`, so `|tFrac/3| < 0.5` and the nearest-integer rule gives `t = tInt`. Callers pass `t = tInt` directly. If §4.1.5 turns out to specify a different rounding (e.g. floor vs round) for edge cases, only the call site at the top-level decoder needs adjustment — the filter itself only takes an integer `t`.

A fourth concern was anticipated but did not materialize:

4. **Filter gating `T < 40`.** ITU-T G.729 §3.8 eq. (48) explicitly states the filter applies only when `T < 40`. `applyPitchEnhancement` short-circuits via `if t < 1 || t >= 40 { return }`, which matches the spec exactly. No deviation.

No test inputs needed adjustment after spec re-reading.

## Test results

```
$ go test -race ./internal/fcb/...
ok  	github.com/hunydev/g729/internal/fcb	1.010s

$ go vet ./internal/fcb/...
(silent)

$ go test -race ./...
ok  	github.com/hunydev/g729/internal/bitstream
ok  	github.com/hunydev/g729/internal/fcb
ok  	github.com/hunydev/g729/internal/fixed
ok  	github.com/hunydev/g729/internal/lsp
ok  	github.com/hunydev/g729/internal/pcm
ok  	github.com/hunydev/g729/internal/pitch
ok  	github.com/hunydev/g729/internal/tables
```

All zero-allocation tests pass (`TestNoAllocationInClampPitchGainForEnhancement`, `TestNoAllocationInDecode_NoEnhancement`, `TestNoAllocationInDecode_WithEnhancement`).

Highlights from the unit suite:

- `TestDecodePositions_TrackMembershipExhaustive` — all 8192 13-bit codes verified: each pulse falls in its declared track, no two pulses ever collide.
- Pitch enhancement closed-form cascades validated for β=0.5 (single-tap propagation `c[20]≈4096`) and β=0.8 (cascade `c[10]≈6554`, `c[20]≈5243`, `c[30]≈4194`).
- `TestDecode_MatchesPiecewiseComposition` confirms `Decode` is exactly `decodePositions → placePulses → applyPitchEnhancement`.

## Benchmark results (zero allocations on every path)

```
goarch: amd64
pkg: github.com/hunydev/g729/internal/fcb
cpu: AMD EPYC 9554P 64-Core Processor
BenchmarkDecode_NoEnhancement-2          92,699,360   12.48 ns/op   0 B/op   0 allocs/op
BenchmarkDecode_WithEnhancement-2        28,206,894   40.40 ns/op   0 B/op   0 allocs/op
BenchmarkDecode_ShortLagEnhancement-2    14,307,518   83.34 ns/op   0 B/op   0 allocs/op
```

`ShortLagEnhancement` (T=20) is the longest cascade (filter runs 20 iterations) — even at this worst case, cost is ~83 ns and zero allocations.

## Commits (Phase 1c)

```
f31b9ce  feat(fcb): package skeleton + Indices type and PulseAmplitude constant
66e8534  feat(fcb): decode 13-bit pulse positions across 4 tracks per ITU §3.8
1e06a3d  feat(fcb): place 4 signed pulses into codebook vector per ITU §3.8
d4fb483  feat(fcb): clamp previous pitch gain to [0.2, 0.8] Q14 per ITU §4.1.5
a59915c  feat(fcb): in-place pitch pre-emphasis filter c[n]+=β·c[n-T] per ITU §3.8
742d06c  feat(fcb): wire Decode — positions + signs + pitch enhancement
4600e11  test(fcb): lock zero-alloc + per-subframe benches; polish doc
26bceda  docs(fcb): mark Phase 1c plan tasks complete
```

(8 commits — 7 task commits as planned + 1 housekeeping commit to flip plan checkboxes.)

## Next phase

Phase 1c is complete. Ready for **Phase 1d — gain VQ decoder** (`internal/gain`): two-stage 7-bit VQ for `(g_p, γ̂_c)`, MA energy prediction, and post-decode pitch-gain export for the next subframe's β.
