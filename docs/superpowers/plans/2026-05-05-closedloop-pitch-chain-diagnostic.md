# Closed-loop pitch chain diagnostic

Date: 2026-05-05

## Objective

Diagnose why Phase 2c closed-loop pitch byte-EQ remains near 12% by
splitting `closedloopStep` into:

- search-window admission
- integer RN winner
- full fractional RN winner
- production integer/fraction/code output
- prior pitch-state influence

Clean-room constraint: diagnostics use only local implementation state,
public ITU test-vector bit fields, and numeric oracle artifacts. No
external G.729 implementation source was consulted.

## Evidence

Command:

```sh
go test -run 'TestOracleHCenter_ClosedLoop(StageSplit|ForcedPitchState)Diagnostic' -v
```

Baseline production-state split:

| Stage | Ref in search window | Ref integer RN best | Ref full fractional RN best | Production int match | Production frac match | Production code match |
|---|---:|---:|---:|---:|---:|---:|
| Subframe 1 | 1093/1835 = 59.56% | 491/1835 = 26.76% | 186/1835 = 10.14% | 491/1835 = 26.76% | 221/1835 = 12.04% | 221/1835 = 12.04% |
| Subframe 2 | 1123/1835 = 61.20% | 472/1835 = 25.72% | 196/1835 = 10.68% | 472/1835 = 25.72% | 215/1835 = 11.72% | 217/1835 = 11.83% |

Forced-reference pitch-state split:

| Stage | Ref in search window | Ref integer RN best | Ref full fractional RN best | Production int match | Production frac match | Production code match |
|---|---:|---:|---:|---:|---:|---:|
| Subframe 1 | 1093/1835 = 59.56% | 466/1835 = 25.40% | 192/1835 = 10.46% | 462/1835 = 25.18% | 233/1835 = 12.70% | 233/1835 = 12.70% |
| Subframe 2 | 1835/1835 = 100.00% | 638/1835 = 34.77% | 305/1835 = 16.62% | 676/1835 = 36.84% | 332/1835 = 18.09% | 332/1835 = 18.09% |

Oracle-`T_op` centre forcing:

| Variant | P1 byte-EQ | P0 byte-EQ | P2 byte-EQ |
|---|---:|---:|---:|
| Production | 221/1835 = 12.04% | 1099/1835 = 59.89% | 217/1835 = 11.83% |
| Oracle `T_op` | 225/1835 = 12.26% | 1096/1835 = 59.73% | 230/1835 = 12.53% |

Reference `int(T1)` lies inside the oracle-`T_op` closed-loop window
only 1083/1835 = 59.02%, confirming the previous P1 proxy is not a
valid 70% follow-on gate for raw `T_op`.

## Diagnosis

1. Search itself is not the primary defect. Production integer matches
   the integer RN-best count exactly in the baseline split, so
   `SearchInteger` is selecting what its current RN surface tells it to
   select.
2. Fractional refinement is not an isolated defect. Even a full
   fractional RN sweep over the whole search window ranks the reference
   code best only about 10-11% in baseline state.
3. Open-loop centre is not the dominant cause. Forcing oracle `T_op`
   barely moves P1/P2 byte-EQ.
4. Prior pitch-state divergence is only a partial contributor.
   Forcing reference pitch decisions makes subframe-2 window admission
   100% and improves P2 code match to 18.09%, but subframe-1 remains
   around 12.70% and RN-best remains low.

## Next target

The next useful diagnostic is not another open-loop/P1 proxy gate. It
should compare the RN input signals feeding the closed-loop search:

- `x` / `xb` target path from `lpResidualSubframe`, `TargetSignal`,
  `ImpulseResponse`, `BackwardFilter`
- `excSearch` path from `oldExc` and residual extension
- state commits in `fcbStep` that update `oldExc` and `swMemErr`

The likely defect class is upstream signal construction or state
commit, not `SearchInteger`, `RefineFraction`, P1/P2 packing, or
open-loop `T_op`.

## Follow-up Split: RN Inputs

Command:

```sh
go test -run 'TestOracleHCenter_ClosedLoop(SignalVariant|StateCommitVariant)Diagnostic' -v
```

### Signal variants

The table reports how often the reference full fractional lag is the
best RN candidate under different target/excitation inputs.

| Variant | Subframe 1 | Subframe 2 | Interpretation |
|---|---:|---:|---|
| baseline `xb + exc` | 186/1835 = 10.14% | 196/1835 = 10.68% | Current production RN surface. |
| `x` direct + exc | 82/1835 = 4.47% | 80/1835 = 4.36% | Bypassing backward filtering makes it worse. |
| `r` direct + exc | 104/1835 = 5.67% | 95/1835 = 5.18% | Residual direct is also worse. |
| zero `swMemErr`, `xb + exc` | 184/1835 = 10.03% | 197/1835 = 10.74% | Target memory is not the dominant issue. |
| `xb + zero oldExc` | 600/1835 = 32.70% | 581/1835 = 31.66% | Removing past excitation radically changes RN ranking. |
| `xb + no residual extension` | 196/1835 = 10.68% | 209/1835 = 11.39% | Residual extension is not the blocker. |
| `xb + oldExc/2` | 168/1835 = 9.16% | 177/1835 = 9.65% | Smaller old excitation does not help. |
| `xb + oldExc*2` | 249/1835 = 13.57% | 273/1835 = 14.88% | More old excitation helps slightly but is far from sufficient. |
| `xb + -oldExc` | 51/1835 = 2.78% | 23/1835 = 1.25% | Not a simple sign inversion. |
| `xb + -oldExc/2` | 53/1835 = 2.89% | 27/1835 = 1.47% | Confirms sign inversion is wrong. |

### Commit-state variants

| Variant | Subframe 1 code | Subframe 2 code | RN-best observation |
|---|---:|---:|---|
| production | 221/1835 = 12.04% | 217/1835 = 11.83% | Baseline. |
| zero `oldExc` after every commit | 100/1835 = 5.45% | 131/1835 = 7.14% | Code output worsens due tie/degenerate search, despite RN-best tie inflation. |
| zero `swMemErr` after every commit | 227/1835 = 12.37% | 219/1835 = 11.93% | No meaningful effect. |
| zero `oldExc + swMemErr` | 100/1835 = 5.45% | 133/1835 = 7.25% | Same as oldExc zeroing. |
| reset `pastQuaEn` after every commit | 249/1835 = 13.57% | 242/1835 = 13.19% | Small gain, not a root cause. |

## Updated Diagnosis

The dominant RN-input divergence is on the excitation side, specifically
the accumulated `oldExc` signal that feeds `excSearch`.

Rejected or weakened candidates:

- `swMemErr` / target memory: zeroing has almost no effect.
- Backward filtering: bypassing it with `x` or `r` makes RN ranking worse.
- Residual extension in `excSearch`: removing it has almost no effect.
- Simple old-excitation sign error: negating `oldExc` makes RN ranking
  much worse.
- Simple low old-excitation amplitude: halving oldExc also worsens.

Remaining likely defect class:

- `oldExc` contents are structurally different from the reference
  encoder's excitation history. This may come from fixed-codebook pulse
  selection, quantized gain reconstruction, `ĝc·c(n)` / `ĝp·v(n)` commit
  scaling, taming, or earlier pitch decisions cascading into ACELP. The
  next useful split is to audit `fcbStep` outputs (`c`, `z`, `ĝp`,
  `ĝc`, and committed `u(n)`) before revisiting closed-loop search.

## Follow-up Split: FCB/Gain Commit

Command:

```sh
go test -run TestOracleHCenter_FCBCommitSplitDiagnostic -v
```

This diagnostic replays the `fcbStep` math without mutating production
state, then compares the emitted `S/C/GA/GB` fields and the committed
excitation mix `u(n)=ĝp·v(n)+ĝc·c(n)`. It also runs two forced variants:

- forced reference pitch, with production FCB search
- forced reference pitch plus reference `C/S`, with only gain search left

| Variant | S | C | GA | GB | All fields |
|---|---:|---:|---:|---:|---:|
| production pitch sf1 | 90/1835 = 4.90% | 0/1835 = 0.00% | 222/1835 = 12.10% | 91/1835 = 4.96% | 0/1835 = 0.00% |
| production pitch sf2 | 77/1835 = 4.20% | 0/1835 = 0.00% | 217/1835 = 11.83% | 89/1835 = 4.85% | 0/1835 = 0.00% |
| forced ref pitch sf1 | 123/1835 = 6.70% | 0/1835 = 0.00% | 235/1835 = 12.81% | 108/1835 = 5.89% | 0/1835 = 0.00% |
| forced ref pitch sf2 | 86/1835 = 4.69% | 0/1835 = 0.00% | 208/1835 = 11.34% | 85/1835 = 4.63% | 0/1835 = 0.00% |
| forced ref pitch + ref `C/S` sf1 | 1835/1835 = 100.00% | 1835/1835 = 100.00% | 238/1835 = 12.97% | 116/1835 = 6.32% | 50/1835 = 2.72% |
| forced ref pitch + ref `C/S` sf2 | 1835/1835 = 100.00% | 1835/1835 = 100.00% | 223/1835 = 12.15% | 90/1835 = 4.90% | 42/1835 = 2.29% |

Commit mix:

| Variant | `abs(ĝp·v)` | `abs(ĝc·c)` | Code-dominant subframes | Taming | Saturations |
|---|---:|---:|---:|---:|---:|
| production pitch sf1 | 33,897,330 | 43,851,571 | 918/1835 = 50.03% | 0/1835 | 0 |
| production pitch sf2 | 34,792,397 | 43,398,253 | 941/1835 = 51.28% | 0/1835 | 0 |
| forced ref pitch sf1 | 25,859,694 | 40,803,150 | 1192/1835 = 64.96% | 0/1835 | 0 |
| forced ref pitch sf2 | 24,934,229 | 40,938,081 | 1245/1835 = 67.85% | 0/1835 | 0 |
| forced ref pitch + ref `C/S` sf1 | 26,958,208 | 39,410,590 | 1192/1835 = 64.96% | 0/1835 | 0 |
| forced ref pitch + ref `C/S` sf2 | 25,831,246 | 39,257,714 | 1242/1835 = 67.68% | 0/1835 | 0 |

### Interpretation

The FCB pulse index `C` is the strongest hard failure: production and
forced-reference-pitch variants both score 0/1835 for `C1` and `C2`.
Forcing reference pitch does not repair the fixed-codebook pulse search.

Injecting reference `C/S` isolates the gain search. `GA/GB` remain near
the same low 5-13% range, so gain VQ / target-energy state also diverges;
however the earliest hard byte-EQ break is already in the ACELP pulse
search surface before gain packing.

The committed excitation is codebook-term dominated in most forced-pitch
subframes, with no taming and no saturation. This weakens the taming and
commit-saturation hypotheses. The next concrete target is the FCB search
input surface: `x' = x - gp*y`, `d(n)`, `signs`, and `phi`/pulse
positions, preferably under forced reference pitch and reference `C/S`
side-by-side.

## Follow-up Split: FCB Search Surface

Command:

```sh
go test -run 'TestOracleHCenter_FCBSearch(Surface|InputVariant)Diagnostic' -v
```

The search-surface diagnostic decodes the reference `C` into pulse
positions using a local inverse of the documented pack format, then
checks whether those reference positions are best under the current
`dAbs/phi` criterion. The local inverse is guarded by a round-trip check
against `PackC`.

| Surface | C hit | S hit | Ref-position surface-S | Ref-position best | Ref C roundtrip |
|---|---:|---:|---:|---:|---:|
| forced ref pitch sf1 | 0/1835 = 0.00% | 123/1835 = 6.70% | 79/1835 = 4.31% | 0/1835 = 0.00% | 1835/1835 |
| forced ref pitch sf2 | 0/1835 = 0.00% | 86/1835 = 4.69% | 81/1835 = 4.41% | 0/1835 = 0.00% | 1835/1835 |

Per-track position hits under forced reference pitch:

| Surface | T0 | T1 | T2 | T3 |
|---|---:|---:|---:|---:|
| sf1 | 237/1835 = 12.92% | 216/1835 = 11.77% | 206/1835 = 11.23% | 150/1835 = 8.17% |
| sf2 | 243/1835 = 13.24% | 184/1835 = 10.03% | 184/1835 = 10.03% | 148/1835 = 8.07% |

Input variants then perturb only the `x'` construction under forced
reference pitch:

| Variant | Sf1 ref-position best | Sf1 ref-position surface-S | Sf2 ref-position best | Sf2 ref-position surface-S |
|---|---:|---:|---:|---:|
| baseline `x - gp*y` | 0/1835 = 0.00% | 79/1835 = 4.31% | 0/1835 = 0.00% | 81/1835 = 4.41% |
| no adaptive subtraction (`x`) | 0/1835 = 0.00% | 79/1835 = 4.31% | 0/1835 = 0.00% | 81/1835 = 4.41% |
| zero `swMemErr`, then `x - gp*y` | 0/1835 = 0.00% | 85/1835 = 4.63% | 1/1835 = 0.05% | 97/1835 = 5.29% |
| zero `oldExc` for adaptive vector | 0/1835 = 0.00% | 82/1835 = 4.47% | 0/1835 = 0.00% | 74/1835 = 4.03% |
| residual direct `r` | 0/1835 = 0.00% | 99/1835 = 5.40% | 1/1835 = 0.05% | 118/1835 = 6.43% |

### Updated Interpretation

The `C` failure is not a field packing bug: reference `C` round-trips
through the local inverse and production `PackC` for every frame.

The reference pulse positions are not winners on the current ACELP
criterion surface, even when pitch is forced to the reference. Disabling
adaptive subtraction, zeroing target memory, zeroing old excitation for
`v/y`, or using residual direct does not materially recover the
reference positions. That weakens the hypotheses that a single
`gp*y`, `swMemErr`, or `oldExc` input explains the FCB failure.

The remaining target is broader FCB search-surface construction:
`x`, `h`, `d(n)=Σx'(i)h(i-n)`, sign extraction, and `phi` scaling/tie
ordering. Since the reference positions are never best, the next useful
diagnostic is a one-frame/early-frame trace comparing current top
candidate vs reference candidate scores (`C²`, `E`, ratio), plus variant
checks for `CorrelationD` and `PhiPrime` scaling/diagonal conventions.

## Follow-up Split: FCB Score Trace and Surface Conventions

Command:

```sh
go test -run 'TestOracleHCenter_FCB(SearchScoreTrace|PhiVariant|CorrelationVariant)Diagnostic' -v
```

The score trace logs the production-best pulse tuple and the reference
`C` tuple under forced reference pitch, using the current `dAbs/phi`
surface. Early frames show the production tuple is not a tie-breaking
artifact: it has a much larger criterion ratio.

Selected examples:

| Frame/subframe | Candidate | Positions | C | S | `dSum` | `E` | `C²/E` |
|---|---|---:|---:|---:|---:|---:|---:|
| f0 sf1 | production | `[5 31 12 4]` | `0x0728` | `0xb` | 203,568,487 | 55,106,892 | 7.52e8 |
| f0 sf1 | reference | `[0 1 2 3]` | `0x0000` | `0xf` | 34,828,446 | 77,246,584 | 1.57e7 |
| f0 sf2 | production | `[10 6 2 18]` | `0x0883` | `0x0` | 168,995,060 | 95,158,951 | 3.00e8 |
| f0 sf2 | reference | `[0 26 12 34]` | `0x02ae` | `0xa` | 75,955,144 | 88,850,337 | 6.49e7 |
| f1 sf1 | production | `[0 16 2 4]` | `0x0188` | `0x5` | 218,384,187 | 79,871,643 | 5.97e8 |
| f1 sf1 | reference | `[15 1 2 14]` | `0x0c0a` | `0x8` | 136,006,461 | 82,251,121 | 2.25e8 |

`PhiPrime` convention variants:

| Variant | Sf1 ref-position best | Sf2 ref-position best |
|---|---:|---:|
| baseline | 0/1835 = 0.00% | 0/1835 = 0.00% |
| no diagonal half | 0/1835 = 0.00% | 0/1835 = 0.00% |
| no off-diagonal sign | 1/1835 = 0.05% | 0/1835 = 0.00% |
| double off-diagonal | 0/1835 = 0.00% | 0/1835 = 0.00% |

`CorrelationD` index variants:

| Variant | Sf1 ref-position best | Sf2 ref-position best |
|---|---:|---:|
| baseline | 0/1835 = 0.00% | 0/1835 = 0.00% |
| target-prefix indexing | 2/1835 = 0.11% | 0/1835 = 0.00% |
| reversed `h` | 0/1835 = 0.00% | 0/1835 = 0.00% |

### Updated Interpretation

`SearchDepthFirst`, `PackC`, and the obvious `CorrelationD` /
`PhiPrime` convention variants are no longer strong root-cause
candidates. The production pulse tuple is winning by a large numerical
margin on the current surface; the reference tuple is not close enough
for tie ordering to matter.

The next boundary to split is therefore the surface inputs before the
ACELP equations: `x` and `h`. A focused next diagnostic should compare
energy/signature variants of `TargetSignal`, `ImpulseResponse`, and the
weighted LPC coefficients feeding them. If no local variant recovers
reference-position-best, the defect likely sits upstream in weighted LP
coefficient generation rather than in ACELP search mechanics.

## Follow-up Split: `x` / `h` Input Variants

Command:

```sh
go test -run TestOracleHCenter_FCBXHVariantDiagnostic -v
```

This diagnostic keeps forced reference pitch and checks whether the
reference `C` pulse tuple becomes best when changing the FCB surface
inputs:

- quantized `aHatSF*` vs unquantized `aQ12Latest`
- residual path only vs target/impulse path only vs both
- γ = 0.75 vs no γ weighting
- identity `h` as a hard impulse-response bypass

Results:

| Variant | Sf1 ref-position best | Sf2 ref-position best |
|---|---:|---:|
| baseline quantized `aHat`, γ=0.75 | 0/1835 = 0.00% | 0/1835 = 0.00% |
| unquantized residual only | 0/1835 = 0.00% | 0/1835 = 0.00% |
| unquantized target/impulse only | 0/1835 = 0.00% | 0/1835 = 0.00% |
| unquantized all | 0/1835 = 0.00% | 0/1835 = 0.00% |
| no γ, quantized LP | 0/1835 = 0.00% | 0/1835 = 0.00% |
| no γ, unquantized LP | 0/1835 = 0.00% | 0/1835 = 0.00% |
| identity `h`, quantized target | 1/1835 = 0.05% | 2/1835 = 0.11% |
| identity `h`, unquantized target | 1/1835 = 0.05% | 2/1835 = 0.11% |

### Updated Interpretation

The obvious `x`/`h` local variants also do not recover the reference FCB
pulse tuple. That weakens these root-cause candidates:

- quantized vs unquantized LP coefficient selection
- γ=0.75 weighting application
- `ImpulseResponse` all-pole recurrence as an isolated issue
- residual-only vs target/impulse-only LP use

At this point the remaining mismatch is broader than one closed-loop or
FCB helper. The next useful boundary is to validate whether the encoder
is even being compared against a compatible FCB oracle surface:

1. Check if reference `C/S` decoded into a code vector and reference
   `GA/GB` produce a plausible lower closed-loop/fixed-codebook error
   than the production-selected `C/S/GA/GB` under the current `x/y/z`
   surface.
2. If reference fields are not better on the current surface, the
   current clean-room objective and the ITU vector's encoded choices are
   being driven by an upstream state/signal difference not isolated by
   local variants.
3. If reference fields are better after decoding/commit but not during
   `C²/E`, revisit the ACELP objective formulation beyond the simple
   `d/phi` convention checks.

## Follow-up Split: Reference Field Error on Current Surface

Command:

```sh
go test -run TestOracleHCenter_FCBReferenceFieldErrorDiagnostic -v
```

This diagnostic compares three candidates under forced reference pitch
on the current `x/y/z` surface:

- production-selected `C/S` plus production best gain
- reference `C/S/GA/GB`
- reference `C/S` plus the best gain re-searched on the current surface

The comparison uses the same weighted-error shape as the gain search /
commit boundary: `Σ(x - ĝp*y - ĝc*z)^2`.

| Surface | Ref `C/S/GA/GB` lower than production | Ref `C/S` + best gain lower than production | Summed production cost | Summed ref-field cost | Summed ref-code best-gain cost |
|---|---:|---:|---:|---:|---:|
| sf1 | 76/1835 = 4.14% | 154/1835 = 8.39% | 286,898,634,048 | 45,970,217,880,321 | 795,895,483,907 |
| sf2 | 85/1835 = 4.63% | 149/1835 = 8.12% | 295,697,118,980 | 47,929,401,026,402 | 809,333,253,595 |

### Updated Interpretation

The ITU reference FCB fields are not better choices on the current
clean-room encoder surface. Even when the reference pulse vector is
paired with the best gain searched locally, it beats production only
about 8% of the time and is worse in aggregate.

This resolves the previous branch:

- Not an ACELP objective bug that simply picks the wrong local minimum
  on an otherwise compatible surface.
- Not a gain packing issue that hides a better reference `C/S`.
- The current upstream state/signal surface is incompatible with the
  one that produced ITU `PITCH.BIT` FCB fields.

Next useful work should move out of local FCB mechanics and validate
the upstream encoder state against independent numeric boundaries:

1. LSP/LPC quantization byte-EQ and reconstructed `aHatSF1/2` trajectory.
2. LP residual and weighted target energy trajectory before FCB.
3. Whether `PITCH.BIT` FCB fields are a viable oracle for this clean-room
   encoder before upstream LSP/LP state is byte-aligned.

## Follow-up Split: PITCH LSP Upstream Boundary

Command:

```sh
go test -run TestOracleHCenter_PITCHLSPUpstreamBoundaryDiagnostic -v
```

This diagnostic extracts the LSP fields from `PITCH.BIT`, reconstructs
reference `aHatSF1/2` through the local LSP decoder, and compares them
to the encoder-produced LSP fields and `aHatSF1/2` on `PITCH.IN`.
It then repeats the FCB reference-field error comparison after forcing
the reconstructed reference LSP filters into the encoder state.

LSP boundary on `PITCH.IN/PITCH.BIT`:

| Field | Match |
|---|---:|
| L0 | 1727/1835 = 94.11% |
| L1 | 582/1835 = 31.72% |
| L2 | 257/1835 = 14.01% |
| L3 | 260/1835 = 14.17% |
| all LSP fields | 45/1835 = 2.45% |

Reconstructed LP filter equality:

| Surface | Exact | Mean absolute coefficient delta | Max absolute coefficient delta |
|---|---:|---:|---:|
| `aHatSF1` | 0/1835 = 0.00% | 224.59 | 1837 |
| `aHatSF2` | 0/1835 = 0.00% | 285.08 | 2822 |

First LSP miss:

```text
frame 0 got=(1,41,13,30) want=(1,41,5,26)
```

Reference-field error comparison:

| Surface | Ref `C/S/GA/GB` lower | Ref `C/S` + best gain lower | Production summed cost | Ref-field summed cost | Ref-code best-gain summed cost |
|---|---:|---:|---:|---:|---:|
| production LSP sf1 | 76/1835 = 4.14% | 154/1835 = 8.39% | 286,898,634,048 | 45,970,217,880,321 | 795,895,483,907 |
| production LSP sf2 | 85/1835 = 4.63% | 149/1835 = 8.12% | 295,697,118,980 | 47,929,401,026,402 | 809,333,253,595 |
| forced reference LSP sf1 | 73/1835 = 3.98% | 156/1835 = 8.50% | 286,812,416,837 | 45,430,735,381,835 | 793,904,921,781 |
| forced reference LSP sf2 | 77/1835 = 4.20% | 149/1835 = 8.12% | 293,102,892,917 | 47,416,889,548,184 | 807,608,057,979 |

### Updated Interpretation

The PITCH vector's LSP path is not byte-aligned: only 2.45% of frames
match all four LSP fields, and the reconstructed LP filters never match
exactly. This is a major upstream mismatch.

However, forcing reference `aHatSF1/2` alone does not make the ITU FCB
fields better on the current surface. The FCB reference-field lower rate
stays around 4%, and reference `C/S` with locally best gain stays around
8%.

That means the FCB mismatch is not explained by a single-frame LSP
filter substitution. The incompatible surface includes additional state:
speech/preprocess history, LP residual memory, weighted-error memory,
excitation history, and gain history. The next boundary should compare
trajectory-level state, not individual local substitutions.

## Follow-up Split: Reference Field Trajectory

Command:

```sh
go test -run TestOracleHCenter_PITCHReferenceTrajectoryDiagnostic -v
```

This diagnostic extends the previous single-frame substitution into a
trajectory substitution. It compares:

- forced reference pitch with production FCB/gain commit
- forced reference pitch with reference `C/S/GA/GB` commit
- forced reference LSP plus reference `C/S/GA/GB` commit

The reference-field commit updates `oldExc`, `swMemErr`, `pastQuaEn`,
`prevGpQ14`, and `lpResidualMemQ` using the local commit math but the
transmitted reference fields.

| Trajectory | Surface | Ref `C/S/GA/GB` lower | Ref `C/S` + best gain lower | Production summed cost | Ref-field summed cost | Ref-code best-gain summed cost |
|---|---|---:|---:|---:|---:|---:|
| production commit | sf1 | 76/1835 = 4.14% | 154/1835 = 8.39% | 286,898,634,048 | 45,970,217,880,321 | 795,895,483,907 |
| production commit | sf2 | 85/1835 = 4.63% | 149/1835 = 8.12% | 295,697,118,980 | 47,929,401,026,402 | 809,333,253,595 |
| reference field commit | sf1 | 93/1835 = 5.07% | 459/1835 = 25.01% | 10,911,295,528,166 | 1,544,986,806,756,847 | 14,454,377,611,580 |
| reference field commit | sf2 | 93/1835 = 5.07% | 471/1835 = 25.67% | 11,036,051,560,141 | 1,694,171,468,524,293 | 14,772,883,683,243 |
| reference LSP + field commit | sf1 | 94/1835 = 5.12% | 467/1835 = 25.45% | 10,874,287,735,110 | 1,539,070,430,724,887 | 14,405,826,792,795 |
| reference LSP + field commit | sf2 | 92/1835 = 5.01% | 479/1835 = 26.10% | 10,988,181,424,691 | 1,688,079,885,563,337 | 14,700,614,138,480 |

### Updated Interpretation

Forcing the transmitted reference FCB/gain fields into the encoder
trajectory does not make those fields locally optimal. The exact
reference fields remain lower than production only about 5% of the time.
Even allowing best local gains on reference `C/S` reaches only about
25-26%.

The large aggregate costs under reference-field trajectory show that
those fields are incompatible with the current reconstructed state.
This makes `PITCH.BIT` FCB fields a weak oracle for fixing local FCB
mechanics until the broader encoder trajectory is byte-aligned.

Practical next target:

- Stop using FCB byte-EQ as a local validation gate for the current
  encoder state.
- Move upstream to the earliest independently measurable trajectory:
  LSP/LPC vector conformance (`LSP.IN/LSP.BIT`) and then `PITCH.IN`
  preprocessing / LP analysis / residual state. The FCB path should be
  revisited only after those upstream boundaries are materially aligned.

## Upstream Recheck: LSP Corpus Gate

Commands:

```sh
go test -run 'TestINT1D10CorpusDiagnostic|TestINT1D8' -v
```

`LSP.IN/LSP.BIT` remains an accept-partial front-end gate, not a
byte-aligned encoder front-end:

| Corpus | L0 | L1 | L2 | L3 | all4 |
|---|---:|---:|---:|---:|---:|
| `LSP.IN/LSP.BIT` full | 78.67% | 38.93% | 17.07% | 19.35% | 3.67% |
| first 50 frames | 76.00% | 60.00% | 24.00% | 8.00% | not logged |
| last 500 frames | 80.80% | 45.80% | 21.40% | 20.00% | not logged |
| `PITCH.IN/PITCH.BIT` full | 94.11% | 31.72% | 14.01% | 14.17% | 2.45% |

Frame-offset sweep on `LSP.IN/LSP.BIT` does not support a simple
one-frame alignment fix:

| Offset | L0 | L1 | L2 | L3 | all4 |
|---|---:|---:|---:|---:|---:|
| -3 | 65.19% | 14.22% | 7.54% | 9.02% | 0.36% |
| -2 | 68.52% | 13.86% | 8.65% | 8.88% | 0.22% |
| -1 | 70.60% | 19.32% | 9.32% | 12.19% | 0.67% |
| 0 | 78.67% | 38.93% | 17.07% | 19.35% | 3.67% |
| +1 | 69.88% | 19.45% | 9.68% | 11.12% | 0.45% |
| +2 | 67.26% | 14.80% | 7.85% | 9.60% | 0.31% |
| +3 | 66.62% | 13.32% | 7.22% | 8.12% | 0.40% |

### Decision

The next target should be a front-end alignment goal, not another FCB
local tweak. The best candidate surface is Phase 2a:

- `LP analysis → LPToLSP → LSPToLSF → Quantize`
- especially L2/L3 split-vector selection, because L0/L1 are much
  healthier than L2/L3 and all4 remains below 4%.

Concrete next gate:

- Raise `LSP.IN/LSP.BIT` all4 and L2/L3 first, then re-run the closed
  loop / FCB diagnostics.
- Do not use `PITCH.BIT` FCB strict byte-EQ as a pass/fail criterion
  until the front-end gate is materially better.

## Closed-Loop Pitch Input Attribution Refresh

After the LSP source-divergence detour was closed, the pitch chain was
remeasured with `TestOracleHCenter_ClosedLoopInputAttributionDiagnostic`.
This diagnostic separates three failure classes for P1/P2:

- search-window miss: the reference delay is outside the local search
  window;
- baseline surface miss: the reference delay is inside the window but
  not the local full fractional RN winner;
- excitation/target variants that make the reference delay become the
  local winner.

Command:

```sh
go test -run TestOracleHCenter_ClosedLoopInputAttributionDiagnostic -v
```

Results:

| Subframe | Window miss | Baseline surface miss | Baseline best |
|---|---:|---:|---:|
| 1 | 742/1835 = 40.44% | 907/1835 = 49.43% | 186/1835 = 10.14% |
| 2 | 712/1835 = 38.80% | 927/1835 = 50.52% | 196/1835 = 10.68% |

Variant rescue rates:

| Variant | Subframe 1 best | Subframe 1 rescued over baseline | Subframe 2 best | Subframe 2 rescued over baseline |
|---|---:|---:|---:|---:|
| `xb + zero oldExc` | 600/1835 = 32.70% | 537/1835 = 29.26% | 581/1835 = 31.66% | 525/1835 = 28.61% |
| `x direct + zero oldExc` | 655/1835 = 35.69% | 571/1835 = 31.12% | 625/1835 = 34.06% | 551/1835 = 30.03% |
| `r direct + zero oldExc` | 655/1835 = 35.69% | 572/1835 = 31.17% | 627/1835 = 34.17% | 553/1835 = 30.14% |

Additional checks:

- `TestOracleHCenter_ClosedLoopWithOracleTopDiagnostic` shows oracle
  `T_op` barely changes byte-EQ: P1 `12.04% -> 12.26%`, P2
  `11.83% -> 12.53%`.
- `zero-swMemErr-after-commit` is also nearly neutral: P1 `12.37%`,
  P2 `11.93%`.
- `zero-oldExc-after-commit` worsens emitted code byte-EQ, but raises
  RN-surface compatibility to `32.70%` / `27.68%`; this is a
  diagnostic clue about accumulated excitation content, not a production
  fix.

Interpretation:

- Closed-loop failure is not primarily open-loop centre selection.
- It is not primarily `swMemErr` target memory.
- There are two live contributors:
  1. search-window miss caused by the local centre trajectory;
  2. surface miss caused by the local `oldExc` trajectory feeding
     `excSearch`.
- The stronger actionable signal is `oldExc`: removing it rescues about
  29-31 percentage points of reference RN-surface winners.

Next production-relevant diagnostic:

- Split `oldExc` commit into adaptive-only `gp*v`, fixed-codebook
  contribution `gc*c`, gain quantization/taming, and shift timing.
- Do this before changing closed-loop target math, because target
  variants without zeroing `oldExc` perform worse than baseline.

## L2/L3 Surface Split

Command:

```sh
go test ./internal/lsp -run TestINT1D11L23SurfaceDiagnostic -v
```

The diagnostic replays `LSP.IN/LSP.BIT` through the clean-room
`LP analysis -> LPToLSP -> LSPToLSF -> Quantize` chain, snapshots the
MA-predictor memory before quantization, and ranks the transmitted
oracle L2/L3 rows on the local search surface.

Measured field rates match the Phase 2a corpus gate:

| Field | Exact |
|---|---:|
| L0 | 1756/2232 = 78.67% |
| L1 | 869/2232 = 38.93% |
| L2 | 381/2232 = 17.07% |
| L3 | 432/2232 = 19.35% |
| all4 | 82/2232 = 3.67% |

Full-tuple local final-cost comparison:

| Metric | Result |
|---|---:|
| Oracle tuple lower than production | 36/2232 = 1.61% |
| Oracle tuple equal to production | 82/2232 = 3.67% |
| Oracle tuple within +10% of production cost | 191/2232 = 8.56% |
| Average oracle / production cost | 4.31x |

Rank diagnostics:

| Surface | Exact | Top-3 | Top-8 | Avg rank |
|---|---:|---:|---:|---:|
| L2 when L0/L1 already match | 212/729 = 29.08% | 401/729 = 55.01% | 559/729 = 76.68% | 5.62 |
| L3 when L0/L1/L2 already match | 82/212 = 38.68% | 138/212 = 65.09% | 180/212 = 84.91% | 4.54 |
| L2 under oracle L0/L1 prefix | 587/2232 = 26.30% | 1133/2232 = 50.76% | 1676/2232 = 75.09% | 6.15 |
| L3 under oracle L0/L1/L2 prefix | 630/2232 = 28.23% | 1151/2232 = 51.57% | 1718/2232 = 76.97% | 5.91 |

Interpretation:

- The transmitted oracle L2/L3 rows are often near the local surface
  but rarely the argmin.
- The full transmitted LSP tuple is locally better than production in
  only 1.61% of frames and is on average 4.31x more expensive on the
  current clean-room surface.
- Therefore the low L2/L3 byte-EQ is not evidence that the CSV/bit
  extraction is wrong, and it is not yet a narrow L2/L3 pack/index bug.
  It is more consistent with upstream `omega` / LP-analysis / MA-state
  trajectory drift before the split-vector search.

Next target after this split:

- Add an `omega`/`aQ12` trajectory diagnostic for `LSP.IN` that buckets
  frames by first divergent field (`L1`, `L2`, `L3`) and compares local
  `omega` proximity to the oracle tuple's reconstructed LSF surface.
- Only consider production changes if that diagnostic isolates a
  specific clean-room arithmetic boundary, such as LPC analysis scale,
  LP-to-LSP root placement, LSP-to-LSF inverse rounding, or MA predictor
  state update.

## Omega Trajectory Bucket Split

Command:

```sh
go test ./internal/lsp -run TestINT1D12OmegaTrajectoryDiagnostic -v
```

The diagnostic buckets every `LSP.IN/LSP.BIT` frame by the first
divergent LSP field and compares the current unquantized `omega`
against two reconstructed LSF surfaces:

- production tuple reconstructed with the pre-quantization local
  MA-predictor memory;
- transmitted oracle tuple reconstructed with the same local
  MA-predictor memory.

| Bucket | Frames | Oracle closer to local `omega` | Avg unweighted dist production | Avg unweighted dist oracle | Avg weighted dist production | Avg weighted dist oracle |
|---|---:|---:|---:|---:|---:|---:|
| all-match | 82 | 0/82 = 0.00% | 596,259 | 596,259 | 470,296,553 | 470,296,553 |
| L0-first | 476 | 40/476 = 8.40% | 1,008,832 | 2,544,441 | 969,249,491 | 3,474,204,532 |
| L1-first | 1027 | 52/1027 = 5.06% | 509,678 | 1,184,672 | 424,503,103 | 1,367,463,452 |
| L2-first | 517 | 66/517 = 12.77% | 653,747 | 941,970 | 474,862,164 | 974,189,459 |
| L3-first | 130 | 23/130 = 17.69% | 552,688 | 755,604 | 415,965,669 | 762,606,210 |

First useful anchors:

- Frame 0 is `L2-first`: production `[0 120 2 11]`, oracle
  `[0 120 10 10]`. Oracle unweighted distance is slightly lower
  (`167,768` vs `192,255`), but oracle weighted distance is higher
  (`116,891,995` vs `102,190,622`). This explains why the local
  weighted search picks L2 row 2 even though the oracle row is nearby.
- Frame 5 is `L3-first`: production `[1 5 14 23]`, oracle
  `[1 5 14 20]`. Oracle distance is much worse (`1,430,988` vs
  `353,261` unweighted; `2,441,085,217` vs `321,789,304` weighted).

Interpretation:

- Once the frame is bucketed by first divergence, oracle reconstruction
  is still usually farther from the current local `omega` than
  production reconstruction.
- The strongest divergence is upstream of L2/L3 packing. The local
  VQ search is mostly coherent with the `omega` it receives.
- The remaining clean-room target is therefore the source of `omega`
  drift: LPC analysis scaling / Levinson fixed-point behavior /
  LP-to-LSP root placement / LSP-to-LSF inverse rounding, plus the
  MA-predictor state only where the bucket says the first miss is L0.

Next diagnostic:

- Split the `omega` source into `aQ12 -> qQ15 -> omega`: log per-bucket
  max `aQ12`, root positions, and LSP-to-LSF deltas around frames 0, 5,
  6, 7, 11, and 28. The aim is to identify the first arithmetic
  boundary where a small clean-room correction would move the local
  `omega` toward the oracle surface without forcing bit-fields.

## Omega Source Frame Trace

Command:

```sh
go test ./internal/lsp -run TestINT1D13OmegaSourceFrameTrace -v
```

Selected frame anchors:

| Frame | Bucket | Production | Oracle | Key observation |
|---:|---|---|---|---|
| 0 | L2-first | `[0 120 2 11]` | `[0 120 10 10]` | `aQ12=[4096,0..]`; local `omega` is the expected uniform-LSF shape. Oracle L2 is near, but weighted search prefers row 2. |
| 5 | L3-first | `[1 5 14 23]` | `[1 5 14 20]` | Upper-split mismatch is concentrated around coordinates 5/6; oracle L3 is much farther on weighted distance. |
| 6 | L1-first | `[0 29 0 23]` | `[0 98 15 24]` | Divergence has already moved to first-stage VQ. |
| 7 | L0-first | `[1 96 30 1]` | `[0 20 13 13]` | Predictor selector and MA-state trajectory differ first. |
| 11 | L1-first | `[1 17 30 7]` | `[1 2 3 8]` | One of the rare frames where oracle full tuple has lower local final cost. |
| 28 | all-match | `[1 13 2 17]` | `[1 13 2 17]` | Even all-match frames keep a non-zero `omega - reconstructed` quantization residual, as expected. |

Frame 0 detail is the most useful clean-room boundary:

- `aQ12=[4096 0 0 0 0 0 0 0 0 0 0]`
- `qQ15=[31441 27565 21459 13613 4661 -4664 -13615 -21460 -27568 -31441]`
- `omega=[2333 4674 7014 9354 11695 14033 16374 18713 21054 23394]`
- production reconstructed LSF:
  `[2190 4736 6953 9400 11556 14076 16385 18709 21261 23707]`
- oracle reconstructed LSF:
  `[2415 4765 6875 9512 11713 14089 16412 18483 20849 23487]`

This means frame 0 does not implicate LPC analysis, LP-to-LSP root
placement, or LSP-to-LSF inverse conversion. The upstream `omega`
source is benign on that anchor. The disagreement happens inside the
VQ distortion surface: oracle row 10 is geometrically nearby, but the
current weighted partial search picks row 2.

Corpus coordinate buckets:

- Average signed `omega - oracle-reconstructed-LSF` is near zero in
  most buckets, so there is no obvious global one-sided angle bias.
- The largest Q-domain deviations occur in high-energy outliers, but
  not in a stable coordinate direction across all buckets.
- L0-first frames remain special because changing the MA selector
  changes the entire reconstructed surface before L1/L2/L3 are
  evaluated.

Updated next diagnostic:

- Test LSF weighting and partial-cost conventions as measurement-only
  variants: weighted vs unweighted L2/L3, full-vector vs split-vector
  cost, and final-cost L0 selection before/after stability. Frame 0
  should be the anchor: a correct hypothesis must explain why oracle
  L2 row 10 is not selected even with clean uniform `omega`.

## Weight Variant Diagnostic

Command:

```sh
go test ./internal/lsp -run TestINT1D14WeightVariantDiagnostic -v
```

Frame 0 anchor:

| Variant | L2 argmin | L2 oracle row | L2 best cost | L2 oracle cost | L3 argmin | L3 oracle row | L3 best cost | L3 oracle cost |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| weighted | 2 | 10 | 33,937,615 | 41,549,070 | 11 | 10 | 68,253,007 | 75,342,925 |
| no 1.2 boost | 2 | 10 | 31,155,391 | 41,502,414 | 11 | 10 | 67,986,751 | 74,891,341 |
| unweighted | 2 | 10 | 49,451 | 59,614 | 10 | 10 | 108,154 | 108,154 |

Corpus rank summary under oracle prefixes:

| Surface | Exact | Top-3 | Top-8 | Avg rank |
|---|---:|---:|---:|---:|
| L2 weighted | 587/2232 = 26.30% | 1133/2232 = 50.76% | 1676/2232 = 75.09% | 6.15 |
| L2 no 1.2 boost | 595/2232 = 26.66% | 1151/2232 = 51.57% | 1689/2232 = 75.67% | 6.04 |
| L2 unweighted | 721/2232 = 32.30% | 1315/2232 = 58.92% | 1882/2232 = 84.32% | 4.55 |
| L3 weighted | 630/2232 = 28.23% | 1151/2232 = 51.57% | 1718/2232 = 76.97% | 5.91 |
| L3 no 1.2 boost | 621/2232 = 27.82% | 1145/2232 = 51.30% | 1722/2232 = 77.15% | 5.87 |
| L3 unweighted | 719/2232 = 32.21% | 1317/2232 = 59.01% | 1850/2232 = 82.89% | 4.85 |

Interpretation:

- Removing the 1.2 boost does not explain the mismatch.
- Unweighted costs improve oracle rank by about 5-6 exact percentage
  points, but not enough to restore byte alignment.
- Frame 0 L2 remains production row 2 even under unweighted distance,
  so the L2 mismatch cannot be reduced to the adaptive weighting rule.
- Frame 0 L3 becomes an unweighted tie at oracle row 10, which suggests
  part of the upper-split difference is weighting/tie-order sensitive,
  but this does not generalize strongly enough across the corpus.

Next diagnostic:

- Test split-search sequencing: greedy `L1 -> L2 -> L3` versus a
  small exhaustive clean-room oracle over `L2 x L3` under fixed
  `(L0,L1)` for selected frames and aggregate rank. This targets the
  remaining possibility that the sequential split heuristic, not the
  scalar weighting formula alone, is where the local surface diverges
  from the transmitted tuple.

## Split-Search Exhaustive Diagnostic

Command:

```sh
G729_LSP_EXHAUSTIVE_DIAG=1 go test ./internal/lsp -run TestINT1D15SplitSearchExhaustiveDiagnostic -v
```

Frame 0 anchor:

| Surface | Exhaustive best | Oracle pair | Greedy pair | Oracle rank | Oracle cost | Best cost |
|---|---|---|---|---:|---:|---:|
| weighted | `(2,11)` | `(10,10)` | `(2,11)` | 8 | 116,891,995 | 102,190,622 |
| unweighted | `(2,10)` | `(10,10)` | `(2,10)` | 2 | 167,768 | 157,605 |

Corpus summary under oracle `(L0,L1)` prefix:

| Surface | Pair exact | Top-3 | Top-8 | Top-32 | Avg rank | Greedy exact | Exhaustive == greedy |
|---|---:|---:|---:|---:|---:|---:|---:|
| weighted | 202/2232 = 9.05% | 382/2232 = 17.11% | 620/2232 = 27.78% | 1061/2232 = 47.54% | 117.99 | 206/2232 = 9.23% | 2113/2232 = 94.67% |
| unweighted | 281/2232 = 12.59% | 553/2232 = 24.78% | 851/2232 = 38.13% | 1378/2232 = 61.74% | 69.37 | 284/2232 = 12.72% | 2116/2232 = 94.80% |

Subset where production already matches oracle `(L0,L1)`:

| Surface | Pair exact | Top-3 | Top-8 | Top-32 | Avg rank | Greedy exact | Exhaustive == greedy |
|---|---:|---:|---:|---:|---:|---:|---:|
| weighted | 80/729 = 10.97% | 146/729 = 20.03% | 244/729 = 33.47% | 413/729 = 56.65% | 90.81 | 82/729 = 11.25% | 684/729 = 93.83% |
| unweighted | 119/729 = 16.32% | 221/729 = 30.32% | 347/729 = 47.60% | 518/729 = 71.06% | 46.49 | 120/729 = 16.46% | 686/729 = 94.10% |

Interpretation:

- Greedy sequencing is not the main cause. Exhaustive `L2 x L3` agrees
  with the greedy pair about 94-95% of the time.
- Oracle pair exact remains low even under exhaustive full-pair search:
  9.05% weighted, 12.59% unweighted.
- Frame 0 confirms the narrow result: exhaustive weighted chooses the
  production pair `(2,11)`, and exhaustive unweighted still chooses
  `(2,10)` rather than oracle `(10,10)`.

Updated boundary:

- Do not change L2/L3 search sequencing.
- The mismatch sits before or around the surface definition consumed by
  the whole VQ: `(L0,L1)` trajectory, MA predictor memory, codebook/
  predictor table numeric content, or the exact clean-room reading of
  distortion weighting. The current local split-search implementation
  is internally coherent.

## Predictor Trajectory Diagnostic

Command:

```sh
go test ./internal/lsp -run TestINT1D16PredictorTrajectoryDiagnostic -v
```

This diagnostic keeps the same local `omega` but changes only the
MA-predictor memory commit policy:

- production commit: commit the locally selected LSP residual;
- oracle commit: commit the transmitted oracle LSP residual each frame.

| Commit policy | L0 | L1 | L2 | L3 | all4 | Ref lower local cost | Ref within +10% |
|---|---:|---:|---:|---:|---:|---:|---:|
| production commit | 78.67% | 38.93% | 17.07% | 19.35% | 3.67% | 1.61% | 8.56% |
| oracle commit | 83.69% | 71.59% | 36.34% | 39.52% | 16.40% | 1.84% | 25.85% |

Early-frame trace:

| Frame | Production-memory pick | Oracle | Oracle-memory pick |
|---:|---|---|---|
| 0 | `(0,120,2,11)` | `(0,120,10,10)` | `(0,120,2,11)` |
| 1 | `(0,120,14,20)` | `(0,120,7,9)` | `(0,120,7,9)` |
| 2 | `(0,120,7,11)` | `(0,120,31,10)` | `(0,120,31,10)` |
| 3 | `(0,120,8,13)` | `(0,120,5,0)` | `(0,120,5,0)` |
| 4 | `(0,120,9,26)` | `(0,120,2,27)` | `(0,120,2,27)` |
| 5 | `(1,5,14,23)` | `(1,5,14,20)` | `(1,5,14,28)` |
| 6 | `(0,29,0,23)` | `(0,98,15,24)` | `(0,105,14,23)` |
| 7 | `(1,96,30,1)` | `(0,20,13,13)` | `(0,56,23,13)` |

Interpretation:

- MA predictor memory trajectory is a major upstream contributor.
  Forcing oracle residual commits more than doubles all4
  (`3.67% -> 16.40%`) and raises L1 sharply (`38.93% -> 71.59%`).
- The very first frame still misses L2/L3 even under oracle memory,
  because both memory states are identical at cold start. That
  remaining frame-0 mismatch is a local cold-start distortion-surface
  issue, not an accumulated-memory issue.
- Reference tuple still rarely has lower local final cost
  (`1.84%`), so oracle commit improves state alignment but does not
  make the current local distortion function fully match the vector
  generator.

Updated next target:

- Treat MA predictor trajectory as the first material upstream blocker.
- Separately keep frame 0 as a cold-start VQ surface probe: since
  `aQ12` and initial memory are known, any fix for that frame must be
  about table numeric content, predictor formula, residual
  rearrangement/stability ordering, or distortion weighting, not
  prior-state drift.

## Cold-Start Surface Variant Diagnostic

Command:

```sh
G729_LSP_EXHAUSTIVE_DIAG=1 go test ./internal/lsp -run TestINT1D17ColdStartSurfaceVariantDiagnostic -v
```

This diagnostic keeps fixed `(L0,L1)` and exhaustively scores
`L2 x L3` while varying only residual/stability ordering.

Frame 0:

| Mode | Best pair | Oracle pair | Oracle rank | Oracle cost | Best cost |
|---|---|---|---:|---:|---:|
| final J1+J2 pre-predictor + stability | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| residual J1 only | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| no residual rearrange | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| post-predictor J1 | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| final without stability | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |

Corpus exact pair rates remain flat:

| Mode | Pair exact | Top-3 | Top-8 | Avg rank |
|---|---:|---:|---:|---:|
| final J1+J2 pre-predictor + stability | 202/2232 = 9.05% | 17.11% | 27.78% | 117.99 |
| residual J1 only | 202/2232 = 9.05% | 17.11% | 27.78% | 117.99 |
| no residual rearrange | 206/2232 = 9.23% | 17.20% | 27.78% | 117.73 |
| post-predictor J1 | 206/2232 = 9.23% | 17.20% | 27.78% | 117.72 |
| final without stability | 202/2232 = 9.05% | 17.20% | 27.78% | 117.99 |

Interpretation:

- Residual rearrangement and stability ordering do not explain the
  cold-start frame 0 mismatch.
- The ordering variants also barely move the corpus surface, so this
  is not a promising production-fix direction.
- The remaining clean-room explanations are narrower:
  table numeric content, MA predictor arithmetic/table content,
  bit-vector generation conditions, or a still-missing scalar
  distortion detail outside the rearrangement/stability ordering.

Current practical boundary:

- The accumulated mismatch is dominated by MA predictor trajectory.
- The cold-start mismatch is a separate VQ surface issue and should not
  be patched by changing L2/L3 sequencing or rearrangement ordering.

## Second-Stage Interpretation Diagnostic

Command:

```sh
G729_LSP_EXHAUSTIVE_DIAG=1 go test ./internal/lsp -run TestINT1D18SecondStageInterpretationDiagnostic -v
```

This diagnostic keeps fixed `(L0,L1)`, exhaustively scores `L2 x L3`,
and changes only the interpretation of the second-stage codebook rows:
current `L1 + L2/L3`, sign-negated lower/upper/both, and lower/upper
codebook swaps.

Frame 0:

| Mode | Best pair | Oracle pair | Oracle rank | Oracle cost | Best cost |
|---|---|---|---:|---:|---:|
| current `L1 + L2/L3` | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| negate lower | `(6,11)` | `(10,10)` | 92 | 176,462,775 | 85,885,562 |
| negate upper | `(2,23)` | `(10,10)` | 457 | 299,375,587 | 83,285,484 |
| negate both | `(6,23)` | `(10,10)` | 612 | 358,946,367 | 66,980,424 |
| swap L2/L3 | `(24,14)` | `(10,10)` | 197 | 222,478,476 | 88,646,109 |
| swap L2/L3 + negate both | `(16,27)` | `(10,10)` | 623 | 357,566,980 | 75,137,438 |

Corpus pair exact:

| Mode | Pair exact | Top-8 | Avg rank |
|---|---:|---:|---:|
| current `L1 + L2/L3` | 202/2232 = 9.05% | 27.78% | 117.99 |
| negate lower | 0/2232 = 0.00% | 0.81% | 440.45 |
| negate upper | 0/2232 = 0.00% | 0.27% | 545.47 |
| negate both | 0/2232 = 0.00% | 0.04% | 851.27 |
| swap L2/L3 | 1/2232 = 0.04% | 0.18% | 590.32 |
| swap L2/L3 + negate both | 5/2232 = 0.22% | 0.85% | 447.39 |

Interpretation:

- The second-stage table interpretation is not inverted, swapped, or
  sign-reversed. Every such variant is much worse than the current
  `L1 + L2/L3` interpretation.
- This removes another class of plausible clean-room implementation
  bugs without touching production behavior.

Remaining candidate set:

- MA predictor trajectory remains the only large measured contributor.
- Cold-start frame 0 remains unexplained by sequencing, weighting,
  rearrangement/stability ordering, or second-stage sign/swap
  interpretation. The remaining cold-start candidates are table numeric
  content, MA predictor coefficient arithmetic/content, or external
  vector-generation conditions.

## MA Predictor Variant Diagnostic

Command:

```sh
G729_LSP_EXHAUSTIVE_DIAG=1 go test ./internal/lsp -run TestINT1D19MAPredictorVariantDiagnostic -v
```

This diagnostic keeps fixed `(L0,L1)`, exhaustively scores `L2 x L3`,
and changes only MA predictor interpretation: selector swap, tap
reverse, `32768 - sumP` compensator, zero memory, and no predictor.

Frame 0:

| Mode | Best pair | Oracle pair | Oracle rank | Oracle cost | Best cost |
|---|---|---|---:|---:|---:|
| current predictor | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| selector swap | `(2,11)` | `(10,10)` | 9 | 437,150,947 | 362,089,121 |
| tap reverse | `(2,11)` | `(10,10)` | 8 | 116,891,995 | 102,190,622 |
| `32768 - sumP` compensator | `(2,11)` | `(10,10)` | 8 | 116,960,063 | 102,413,803 |
| zero memory | `(24,17)` | `(10,10)` | 692 | 749,601,841,588 | 714,931,929,974 |
| no predictor | `(2,11)` | `(10,10)` | 7 | 1,695,743,775 | 1,513,534,390 |

Corpus pair exact:

| Mode | Pair exact | Top-8 | Avg rank |
|---|---:|---:|---:|
| current predictor | 202/2232 = 9.05% | 27.78% | 117.99 |
| selector swap | 123/2232 = 5.51% | 17.61% | 168.42 |
| tap reverse | 91/2232 = 4.08% | 15.41% | 179.19 |
| `32768 - sumP` compensator | 202/2232 = 9.05% | 27.82% | 118.01 |
| zero memory | 0/2232 = 0.00% | 0.94% | 473.77 |
| no predictor | 111/2232 = 4.97% | 17.65% | 202.75 |

Interpretation:

- MA predictor selector/tap interpretation is not swapped or reversed.
- The 32768 compensator variant is effectively neutral and does not
  explain frame 0.
- Removing predictor memory or the predictor itself is much worse.
- Therefore the large MA predictor issue is the trajectory of committed
  residuals, not an obvious selector/tap arithmetic inversion in the
  predictor formula.

Practical next step:

- Stop searching for a production tweak in L2/L3 local mechanics. The
  clean-room evidence says the local mechanics are internally coherent.
- Move to artifact hygiene: keep these diagnostics as non-gating
  evidence, define the next implementation target as improving LSP
  predictor trajectory only when an allowed numeric oracle identifies a
  concrete table or coefficient mismatch.

## LSP Table Handoff

Command:

```sh
G729_WRITE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_WriteLSPTableHandoff -v
```

Generated handoff files:

| File | Rows | Purpose |
|---|---:|---|
| `testdata/oracle/handoff/lsp_tables_got.csv` | 1681 | Local scalar dump for LSP VQ tables and MA predictor coefficients. |
| `testdata/oracle/handoff/lsp_tables_expected_template.csv` | 1681 | Verifier-owned numeric `expected` template. |

Schema:

```csv
table,selector,tap,row,col,got
table,selector,tap,row,col,expected
```

The key is `table,selector,tap,row,col`. `selector=-1,tap=-1` for
LSP codebooks, and `row=-1` for `MAPredictorsLSP`.

After the verifier fills `expected`, compare:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

Complete exact verdict:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

Clean-room status:

- These files live under `testdata/oracle/handoff/`, so they are not
  optional validator artifacts and do not gate CI.
- They contain only numeric scalar cells and controlled table labels
  already present in the clean-room implementation.
- The next external verifier action is to fill `expected` values only.
  No source names, code snippets, branch descriptions, or provenance
  notes should be added.

Implementation implication:

- Until a filled numeric handoff identifies a concrete mismatch, the
  LSP VQ implementation should remain unchanged. The measured local
  mechanics are coherent; the unresolved risk is numeric table /
  coefficient parity or vector-generation conditions outside the local
  code path.

## LSP Predictor Residual Handoff

Command:

```sh
G729_WRITE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPPredictorResidualHandoff -v
```

Generated handoff files:

| File | Rows | Purpose |
|---|---:|---|
| `testdata/oracle/handoff/lsp_predictor_residual_got.csv` | 22321 | Local committed LSP MA-predictor residual trajectory, 2232 frames x 10 coefficients plus header. |
| `testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv` | 22321 | Verifier-owned numeric `expected` template. |

Schema:

```csv
frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,got
frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,expected
```

The comparison key is `frame,col`. Index fields are numeric context
only: local emitted LSP fields and transmitted `LSP.BIT` fields.

After the verifier fills `expected`, compare:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

Complete exact verdict:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

Implementation implication:

- This handoff directly targets the measured large contributor:
  committed LSP predictor residual trajectory.
- If table handoff matches but this residual trajectory differs, the
  next production investigation should focus on quantizer decision
  inputs and commit timing rather than table content.

## Closed-Loop OldExc Component Split

Added diagnostics:

```sh
go test -run 'TestOracleHCenter_ClosedLoopSearchPolicyFloorDiagnostic|TestOracleHCenter_ClosedLoopOldExcComponentCommitDiagnostic' -v
```

Key results:

- FCB commit mix is not a taming or saturation issue. Existing
  `TestOracleHCenter_FCBCommitSplitDiagnostic` reports `taming=0` and
  `saturations=0`; commit energy is often code-dominant, but forcing
  pitch/reference code does not make gain fields line up.
- Replacing committed `oldExc` tail with `gp*v` only or `gc*c` only
  does not explain the pitch failure:
  - production selected code: P1 `12.04%`, P2 `11.83%`
  - pitch-only tail: P1 `11.34%`, P2 `9.81%`
  - code-only tail: P1 `9.26%`, P2 `8.99%`
- The earlier zero-oldExc `full-frac-RN-best` lift is mostly a
  diagnostic artifact. With zero old excitation, many search surfaces
  have non-positive global RN, while the actual search starts from
  `RNbest=0` and will not select negative candidates:
  - subframe1 zero-oldExc: global-best `32.70%`, positive-global-best
    `2.94%`, selected-code `5.45%`, non-positive-global `30.79%`
  - subframe2 zero-oldExc: global-best `31.66%`, positive-global-best
    `1.53%`, selected-code `5.72%`, non-positive-global `31.88%`

Current implication:

- The closed-loop pitch problem is no longer isolated to one
  `oldExc` commit component. Removing history makes an equality-based
  RN oracle look better only because many surfaces collapse at or below
  zero.
- The next useful split is search-input numeric parity: the local
  target `x`, backward target `xb`, impulse response `h`, residual
  extension, and selected integer/fraction RN values need scalar
  handoff rows for the first failing PITCH frames. That should tell us
  whether the mismatch is in target construction, interpolation /
  correlation, or upstream excitation state.

## Pitch Closed-Loop Search Handoff

Added handoff tests:

```sh
G729_WRITE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_WritePitchClosedLoopSearchInputHandoff -v
G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff -v
```

Generated files:

| File | Rows | Purpose |
|---|---:|---|
| `testdata/oracle/handoff/pitch_closedloop_search_got.csv` | 3193 | Local first-four PITCH-frame closed-loop scalar dump. |
| `testdata/oracle/handoff/pitch_closedloop_search_expected_template.csv` | 3193 | Verifier-owned numeric `expected` template. |
| `testdata/oracle/handoff/PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md` | n/a | Copyable clean-room verifier prompt. |

Schema:

```csv
field,frame,sub,index,lag,frac,got
field,frame,sub,index,lag,frac,expected
```

The comparison key is `field,frame,sub,index,lag,frac`. `-1` means
"not applicable" for that scalar row.

The rows include:

- scalar window and decision fields: `centre`, `window_min`,
  `window_max`, `ref_int`, `ref_frac`, `ref_code`, `prod_int`,
  `prod_frac`, `prod_code`, `prod_rn_int`, `ref_rn_frac`;
- vector inputs: `a_hat`, `residual`, `target_x`, `impulse_h`,
  `target_xb`, `old_exc`, `exc_residual_ext`;
- score surface rows: `rn_int` and `rn_frac`.

Current pre-fill hashes:

- `pitch_closedloop_search_got.csv`:
  `086a22e02010f9f9eb1292d89f4b58a06d6969061d7119385dd90c630eb314ea`
- `pitch_closedloop_search_expected_template.csv`:
  `5a4b28ab4c51728c107e5cc278362a67e9f7f5a0b27d9a3bef8d2367bb31d202`
- `PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md`:
  `0d5de5773dc4b23b31079e2b254e08a02eaea056174a460158fd05129ac1dd01`

Verifier-filled result:

```sh
G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_COMPLETE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_EXACT_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff -v
```

Result: exact `3192/3192 100.00%`, `mismatches=0`, `blanks=0`.
Post-fill hash:
`e855712427f3da636f4db1c00045782e8b8aec0b7191b0db342c7e57ccba6b8a`.

Interpretation:

- If these `expected` values were independently computed by the
  verifier, the first-four-frame closed-loop pitch search inputs and
  RN score surface match the clean-room implementation exactly.
- That would move the PITCH.BIT byte mismatch out of local
  target/search arithmetic for these frames and toward source
  divergence: the transmitted PITCH.BIT pitch fields are not the
  argmax selected by this search surface. Example: frame 0 subframe 0
  has local/verifier `centre=74`, `window=[71,77]`, but decoded
  `ref_int=33`, `ref_rn_frac=-207227538`, and local selected code
  `154` versus `ref_code=41`.
- If the template was populated by copying `got` values rather than
  independent calculation, this only proves the handoff compare path
  and cannot be used as an oracle result.
