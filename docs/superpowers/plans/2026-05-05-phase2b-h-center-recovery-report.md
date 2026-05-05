# Phase 2b H-CENTER Recovery Report

**Date:** 2026-05-05
**Status:** SURRENDER — 5/5 fresh H-CENTER slots spent; plausibility remains below 70%.
**Surface:** encoder open-loop pitch `T_op` against `PITCH.IN` / `PITCH.BIT` P1 integer lag.

## 1. Objective and stop condition

Raise `TestEncode_OpenLoopPitchConsistency` plausibility from the pinned 53.95% baseline to >=70% soft target / >=80% stretch target, or prove why the remaining H-CENTER defect cannot be moved further under the allowed clean-room evidence.

Stop condition reached: **slot budget exhausted and final production baseline remains 990/1835 = 53.95% (<70%)**.

## 2. Clean-room scope

Allowed evidence used:
- `docs/superpowers/specs/itu/G729E.txt` Annex A §A.3.3 / §A.3.4.
- Existing local plans and tests.
- The ITU Annex A vector pair `PITCH.IN` / `PITCH.BIT`.

No ITU reference C source, bcg729, Sipro Lab, FFmpeg, or other G.729 implementation code was consulted.

## 3. Hypothesis cycles

### Slot 1/5 — H-OQ2: quantized Â for open-loop weighting

Problem statement + spec cite: Annex A §A.3.3 states that the perceptual weighting filter is based on quantized LP coefficients `âi`, with fixed `γ = 0.75`; current production still uses `aQ12Latest`, documented as the Phase 2b unquantized stand-in.

Measurable hypothesis: replacing the open-loop weighting input with quantized Â should raise plausibility by at least +6 pp, crossing 60%.

Diagnostic test: `TestPhase2bHCenter_HOQ2QuantizedDiagnostic`.

Result:
- unquantized production baseline: **990/1835 = 53.95%**
- quantized SF1 candidate: **993/1835 = 54.11%**
- quantized SF2 candidate: **987/1835 = 53.79%**

Disposition: **rejected**. Quantized SF1 moves only +0.16 pp, far below the H-OQ2 pass criterion. No production change retained.

### Slot 2/5 — H-PHASE: frame-boundary filter memory phasing

Problem statement + spec cite: Annex A §A.3.3 eq. A.2/A.3 define recursive filters across subframe time; a mismatch between `lpResidualMem`, `swMem`, and `oldWspeech` at the frame boundary could miscenter §A.3.4.

Measurable hypothesis: a sine-wave boundary diagnostic should expose a one-frame or 10-sample slip between memories and the searched weighted-speech history.

Diagnostic test: `TestPhase2bHCenter_HPhaseBoundaryAlignment`.

Result: PASS. After every tested frame:
- `swMem[0:10] == oldWspeech[133:143]`
- `lpResidualMem[0:10] == oldSpeech[230:240]`

Disposition: **rejected as current root cause**. The tested frame-boundary alignment is internally consistent. No production change retained.

### Slot 3/5 — H-NEW: §A.3.4 eq. A.4 decimation off-by-one

Problem statement + spec cite: Annex A §A.3.4 eq. A.4 is `R(k) = Σ sw(2n)sw(2n-k), n=0..39`; the only plausible off-by-one variant inside the 223-sample buffer is `sw(2n+1)`.

Measurable hypothesis: if the implementation was one decimated sample early, the `sw(2n+1)` candidate should materially lift plausibility.

Production candidate tested then reverted:
- `correlateAt`: `wsp[144+2*n] * wsp[144+2*n-k]`
- `energy`: `wsp[144+2*n-k]`

Result:
- production baseline candidate after change: **973/1835 = 53.02%**
- quantized SF1 under same candidate: **985/1835 = 53.68%**

Disposition: **rejected and rolled back**. The off-by-one variant regresses the corpus gate.

### Slot 4/5 — H-NEW: normalized-correlation negative handling

Problem statement + spec cite: Annex A §A.3.4 eq. A.5 normalizes `R(ti)` but does not explicitly describe negative-correlation treatment. Current code treats non-positive `R` as zero.

Measurable hypothesis: if the intended score used magnitude, `|R(k)|/sqrt(E(k))` should lift plausibility.

Production candidate tested then reverted:
- `compareNormalized`: score candidates by `abs(R)^2/E` instead of clipping `R <= 0` to zero.

Result:
- production baseline candidate after change: **962/1835 = 52.43%**
- quantized SF1 under same candidate: **969/1835 = 52.81%**

Disposition: **rejected and rolled back**. Magnitude scoring regresses the corpus gate.

### Slot 5/5 — H-NEW: γ value ambiguity

Problem statement + spec cite: Annex A §A.3.3 explicitly fixes `γ = 0.75` and says the adaptive weighting procedure from clause 3.3 is not used in the reduced-complexity version.

Measurable hypothesis: no legal production candidate remains under the allowed spec text. Changing `γ` would be parameter tuning against a binding constant, not an ambiguity closure.

Diagnostic result: `rg` over `G729E.txt` finds Annex A §A.3.3 as the binding reduced-complexity γ statement; other γ mentions are base-codec or non-Annex-A contexts.

Disposition: **closed without production change**. No clean-room/spec-compliant gamma variant was available.

## 4. Final measurement

Final production state remains the original pinned open-loop implementation:

- Range gate: **1835/1835 = 100.00%**
- Plausibility: **990/1835 = 53.95%**
- CSV artifact: `testdata/phase2b/hcenter_top_vs_t1.csv`

Δ histogram buckets >=0.5%:

`Δ=-69:10(0.5%) Δ=-59:10(0.5%) Δ=-16:10(0.5%) Δ=-15:9(0.5%) Δ=-13:12(0.7%) Δ=-11:12(0.7%) Δ=-10:10(0.5%) Δ=-8:10(0.5%) Δ=-7:11(0.6%) Δ=-4:25(1.4%) Δ=-3:57(3.1%) Δ=-2:58(3.2%) Δ=-1:130(7.1%) Δ=+0:406(22.1%) Δ=+1:144(7.8%) Δ=+2:75(4.1%) Δ=+3:65(3.5%) Δ=+4:24(1.3%) Δ=+11:9(0.5%) Δ=+18:9(0.5%) Δ=+45:10(0.5%) Δ=+106:9(0.5%)`

## 5. Verification commands

Focused gates run:

```sh
env GOCACHE=/tmp/go-build go test -run TestEncode_OpenLoopPitchConsistency -v
env GOCACHE=/tmp/go-build go test -run TestPhase2bHCenter_HOQ2QuantizedDiagnostic -v
env GOCACHE=/tmp/go-build go test -run TestPhase2bHCenter_HPhaseBoundaryAlignment -v
env GOCACHE=/tmp/go-build G729_WRITE_HCENTER_LOGS=1 go test -run TestPhase2bHCenter_WriteMeasurementArtifacts -v
```

Full regression gates run at final audit:

```sh
env GOCACHE=/tmp/go-build go test ./...
env GOCACHE=/tmp/go-build go build ./...
env GOCACHE=/tmp/go-build go test -race ./...
```

All three passed. The non-escalated build initially emitted a Go module stat-cache warning for `/home/exedev/go/pkg/mod`; the approved rerun of the same `go build ./...` command completed cleanly.

## 6. Surrender analysis

The tested hypotheses cover the remaining prompt-named H-CENTER surfaces:

- H-OQ2 is real as a spec correction, but it is not the dominant `T_op` plausibility blocker on this corpus.
- H-PHASE has no observed frame-boundary slip in the encoder-owned memories.
- The concrete decimated-index off-by-one alternative regresses.
- Magnitude-based negative-correlation scoring regresses.
- Annex A leaves no legal γ retuning surface.

The remaining Δ distribution is not explained by the allowed §A.3.3/§A.3.4 ambiguities. Further progress would require evidence outside the allowed clean-room set, reopening upstream LPC/LSP vector quantization quality, or changing the validation target itself. Under the stated constraints, the correct action is to preserve the production pin and document the failed recovery.

— end of Phase 2b H-CENTER recovery report —
