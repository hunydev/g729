# Phase 2c — Closure Report (encoder closed-loop pitch + adaptive codebook)

**Date:** 2026-05-10
**Phase:** 2c (encoder closed-loop pitch refinement: §A.3.5 impulse response → §A.3.6 target signal → §A.3.7 integer + b30 fractional pitch search → §3.7.1 adaptive codebook v(n) → §3.7.3 Gp + filtered y(n) → §3.7.2 / Table 8 P1/P0/P2 packing → encoder per-subframe wiring)
**Sub-plan:** `docs/superpowers/plans/2026-05-09-phase2c-closed-loop-pitch-plan.md`
**Master plan:** `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §4
**Phase 2b closure ref:** `docs/superpowers/plans/2026-05-08-phase2b-closure-report.md`
**HEAD at authoring:** `730bf43` (post INT-2 zero-alloc + race gate; closure commit appended on top)
**Status:** **CLOSED-DEFERRED — Phase 2c structurally complete; INT-1 byte-EQ FAIL-DEFERRED pending Phase 2d ENC-INT closure.**

---

## 1. Scope & Objective

Phase 2c delivered the encoder-side closed-loop pitch refinement and adaptive-codebook chain per ITU-T G.729 Annex A §A.3.5–§A.3.7 + base §3.7.1/§3.7.2/§3.7.3 + §4.1.3, on top of the Phase 2b open-loop centre `tOp`:

- **QA-1 — quantized Â discipline.** `internal/lsp.LSPToLP` exported (was package-private `lspToLP`); the encoder closed-loop chain runs all filters on quantized Â reconstructed from `lspOldQ`. Closes Phase 2b carryover **H-OQ2**.
- **HI-1 — impulse response h(n).** `internal/pitch/closedloop.ImpulseResponse(aQ12, gamma, h*[40])` per §A.3.5.
- **TG-1 — target signal x(n).** `internal/pitch/closedloop.TargetSignal(aQ12, residual, swMem, x*[40])` per §A.3.6 — caller-owned `swMem`, mutation deferred to encoder driver per I3.
- **CL-1 — integer-lag closed-loop search.** `closedloop.SearchInteger(xb, exc, center, sub) -> (intLag, RNbest)` with backward-filtered target `xb` and Annex A numerator-only `RN(k)` per §A.3.7 eq. A.6/A.7.
- **CL-2 — subframe-2 search window.** `closedloop.Subframe2Window(intT1) -> (tmin, tmax)` per §4.1.3 lines 1512–1523.
- **FR-1 — b30 1/3-sample interpolation.** Table from §3.7.1 eq. 40 (referenced by §A.3.7 eq. A.8) — `closedloop/frac.go`.
- **FR-2 — fractional refinement.** `closedloop.RefineFraction(xb, exc, intLag) -> frac ∈ {−1,0,+1}` evaluating RN(k) at three fractional positions per §A.3.7 eq. A.8.
- **VP-1 — adaptive codebook vector v(n).** `closedloop.AdaptiveVector(exc, intLag, frac, v*[40])` per §3.7.1 eq. 40.
- **GP-1 — Gp + filtered y(n).** `closedloop.GpAndY(x, y0, h, v) -> (gp, y*[40])` per §3.7.3 eq. 43/44, Gp clamped to [0, 1.2] inclusive (Q14, OQ-GBOUND interpretation).
- **ENC-1 — P1/P0/P2 bit packing.** `closedloop.PackP1P0P2` per §3.7.2 eq. 41/42 + Table 8; reuses `internal/pitch/parity.go` for P0.
- **INT-0 — encoder wiring.** `(*Encoder).closedloopStep(sub)` invoked twice per frame after `lpcStep`/`openloopStep`; commits `oldExc`/`swMem` updates only at frame end (I3).
- **INT-2 — zero-alloc + race-clean.** `closedloopStep` itself and the full hot path (`lpcStep + openloopStep + 2× closedloopStep`) report `AllocsPerRun == 0`; race detector clean.

**Sub-phase ITU vector gate:** `PITCH.IN` → encoder LPC + open-loop + closed-loop chain → STRICT byte-EQ against `PITCH.BIT` P1/P0/P2 fields per §4.1.3. **Disposition: FAIL-DEFERRED** (see §5).

---

## 2. Task ledger

All 14 sub-plan tasks are `[x]`. Sub-plan reference: `docs/superpowers/plans/2026-05-09-phase2c-closed-loop-pitch-plan.md` §5.

| Family | Task | Title | Status | Commit | Outcome |
|--------|------|-------|--------|--------|---------|
| QA  | 2c-QA-1   | Export `LSPToLP` for encoder closed-loop reuse                | `[x]` | `1db809e` | `internal/lsp.LSPToLP` exported; decoder-side caller updated; closes H-OQ2. |
| HI  | 2c-HI-1   | Impulse response h(n) for 1/Â(z/γ) over subframe              | `[x]` | `ac76a73` | `closedloop.ImpulseResponse` per §A.3.5 with caller-owned `[40]int16` scratch. |
| TG  | 2c-TG-1   | Target signal x(n)                                            | `[x]` | `8c095ef` | `closedloop.TargetSignal` per §A.3.6; `swMem` read-only inside, mutation deferred. |
| CL  | 2c-CL-1   | Integer-lag closed-loop search per §A.3.7                     | `[x]` | `8e9ef86` | `closedloop.SearchInteger`; numerator-only `RN(k)` over `[center−3, center+3] ∩ [20,143]`. |
| CL  | 2c-CL-2   | Subframe-2 search window per §4.1.3                           | `[x]` | `ebfc451` | `closedloop.Subframe2Window` slide rule per spec lines 1512–1523. |
| FR  | 2c-FR-1   | b30 1/3 fractional interpolation table                        | `[x]` | `2f2b4b4` | b30 table + interpolation kernel per §3.7.1 eq. 40 / §A.3.7 eq. A.8. |
| FR  | 2c-FR-2   | Fractional refinement around integer winner                   | `[x]` | `a0695e2` | `closedloop.RefineFraction` evaluates RN(k) at frac ∈ {−1,0,+1}. |
| VP  | 2c-VP-1   | Adaptive codebook vector v(n) from oldExc                     | `[x]` | `8c2297f` | `closedloop.AdaptiveVector` mirrors decoder construction; zero-alloc. |
| GP  | 2c-GP-1   | Adaptive-codebook gain Gp and filtered y(n)                   | `[x]` | `439daf1` | `closedloop.GpAndY` per §3.7.3 eq. 43/44; Gp clamp [0, 1.2] Q14 inclusive. |
| ENC | 2c-ENC-1  | Pack P1/P0/P2 per Table 8                                     | `[x]` | `3895c4b` | `closedloop.PackP1P0P2`; reuses `internal/pitch/parity.go`. |
| INT | 2c-INT-0  | Wire closed-loop pitch per subframe                           | `[x]` | `1c4e49a` | `(*Encoder).closedloopStep`; QA-1 → HI-1 → TG-1 → CL-1 → FR-2 → VP-1 → GP-1 → ENC-1 chain; oldExc/swMem committed at frame end (I3). |
| INT | 2c-INT-1  | STRICT byte-EQ vs PITCH.BIT P1/P0/P2                          | `[x]` FAIL-DEFERRED | `4c0f500`, `be0128d` | Baseline 2.07/51.28/3.71 → escalation 1 (OQ-K<40 LP-residual extension): **9.05 / 56.46 / 9.75**. STRUCTURAL — see §5. |
| INT | 2c-INT-2  | Zero-alloc + race-clean closed-loop step                      | `[x]` | `730bf43` | `AllocsPerRun == 0` for `closedloopStep(0/1)` and full hot-path; race detector clean; `BenchmarkClosedloopStep` 14964 ns/op. |
| INT | 2c-INT-3  | Phase 2c closure report (this document)                       | `[x]` | (this commit) | Authored at HEAD `730bf43`. |

**Pass criteria** (sub-plan §5/§6/§7): C1 STRICT byte-EQ → **NOT MET** (FAIL-DEFERRED, §5). C2 `go vet` ✅. C3 `go build` ✅. C4 (encoder integration smoke) ✅. C5 zero-alloc ✅. C6 race-clean ✅. C7 (no LSP codebook modifications) ✅. C8 (no decoder-pitch state mutation per I10) ✅. C9 closure report ✅ via this document.

---

## 3. Production code map

Files added or materially modified across Phase 2c (Phase 2a/2b inheritance excluded):

### `internal/pitch/closedloop/` (new sibling package, all Phase 2c-new)

| File | Role |
|------|------|
| `internal/pitch/closedloop/doc.go`           | Package doc, §A.3.5–§A.3.7 + §3.7.1/§3.7.2/§3.7.3 + §4.1.3 cite, I-2c-1 / I-2c-2 statement. |
| `internal/pitch/closedloop/impulse.go`       | `ImpulseResponse` per §A.3.5 (Task HI-1). |
| `internal/pitch/closedloop/target.go`        | `TargetSignal` per §A.3.6 (Task TG-1). |
| `internal/pitch/closedloop/correlate.go`     | Backward-filtered target `xb` and `RN(k)` numerator (Task CL-1 helper). |
| `internal/pitch/closedloop/encode.go`        | `SearchInteger` integer-lag scanner (Task CL-1) and full subframe driver. |
| `internal/pitch/closedloop/window.go`        | `Subframe2Window` per §4.1.3 (Task CL-2). |
| `internal/pitch/closedloop/frac.go`          | b30 1/3-sample interpolation table (Task FR-1). |
| `internal/pitch/closedloop/refine.go`        | `RefineFraction` (Task FR-2). |
| `internal/pitch/closedloop/adaptive.go`      | `AdaptiveVector` per §3.7.1 (Task VP-1). |
| `internal/pitch/closedloop/gain.go`          | `GpAndY` per §3.7.3 (Task GP-1). |
| `internal/pitch/closedloop/encode.go`        | `PackP1P0P2` per §3.7.2 + Table 8 (Task ENC-1). |

Test + benchmark files: `impulse_test.go`, `impulse_bench_test.go`, `target_test.go`, `target_bench_test.go`, `correlate_test.go`, `window_test.go`, `frac_test.go`, `refine_test.go`, `adaptive_test.go`, `gain_test.go`, `encode_test.go`.

### `internal/lsp/`

| File | Role |
|------|------|
| `internal/lsp/lsp_lp.go` | `lspToLP` → exported as `LSPToLP` (Task QA-1). |
| `internal/lsp/decoder.go` | Internal callers updated to exported name. |

### Root package

| File | Role |
|------|------|
| `encoder.go` | Adds `(*Encoder).closedloopStep(sub int)` and per-subframe scratch fields under the existing `// §5.3 preallocated histories` block; `oldExc`/`swMem` commit deferred to frame end per I3. |
| `phase2c_int0_closedloop_wiring_test.go` | INT-0 encoder smoke gate. |
| `phase2c_int1_pitch_byteeq_test.go` | INT-1 STRICT byte-EQ gate (FAIL-DEFERRED at 9.05 / 56.46 / 9.75 %). |
| `phase2c_int2_closedloop_zeroalloc_test.go` | I4 zero-alloc gate on `closedloopStep` and hot path. |

### Inherited unmodified

`internal/pitch/{adaptive,delay,parity}.go` (decoder pitch package) — `parity.go` re-used as a callee from ENC-1; the decoder pitch package itself is untouched per I10. `internal/pitch/openloop/` — frozen under Phase 2b I6.

---

## 4. Diagnostic findings & decisions

### 4.1 H-OQ2 (Phase 2b carryover) — RESOLVED at QA-1

Phase 2b LIVE-DEFERRED **H-OQ2** ("`aQ12Latest` is the unquantized Â stand-in") was resolved by Task QA-1: `internal/lsp.LSPToLP` is exported, and `closedloopStep` reconstructs quantized Â from `lspOldQ` for every subframe before invoking HI-1/TG-1. Phase 2b INT-1 plausibility is **not** retroactively re-measured — Phase 2b's `internal/pitch/openloop/` surface remains frozen under I6 and `aQ12Latest` is still the Phase 2b open-loop input.

### 4.2 OQ-GBOUND — pinned at GP-1 inclusive 1.2 Q14

§3.7.3 eq. 43 specifies the Gp upper bound as `1.2` without inline Q-format. GP-1 pins **inclusive `1.2` Q14 = 19661** (i.e. Gp ∈ [0, 19661]). The boundary case Gp == 19661 occurs in PITCH.IN at low frequencies; both inclusive and exclusive variants were dry-tested — the inclusive interpretation matches the Phase 2c plausibility floor better and is consistent with the §3.9 gain quantizer's two-stage clamp. **CLOSED inclusive 1.2.**

### 4.3 OQ-K<40 — escalation 1 LP-residual extension refactor

CL-1's backward correlation `RN(k)` over `[center−3, center+3] ∩ [20,143]` requires past excitation `exc[len−k .. len]` for `k ∈ [20, 39]`. The Phase 2c-INT-1 baseline (escalation 0) used a **`minSafeCentre = 45` workaround** that biased open-loop centres `tOp < 45` to a synthetic floor — yielding P1 2.07 % / P0 51.28 % / P2 3.71 %. Escalation 1 (commit `be0128d`) refactored the LP-residual extension: anchor `u(0)` at `exc[len − SubframeLen]` and fill the trailing 40 samples with the current-subframe residual `r(n)`, removing the `minSafeCentre` bias. Result: **P1 9.05 % / P0 56.46 % / P2 9.75 %** — Δ=0 buckets now dominant (≈9-10 %) but still FAIL.

### 4.4 OQ-WINDOW — still pinned (carryover)

§A.3.7 search window `int(T1) ∈ [tOp−5, tOp+4]` is implemented verbatim. The Phase 2b plausibility (`int(T1) ∈ [tOp−5, tOp+4]` over PITCH.BIT) was 53.95 %; the Phase 2c INT-1 P1 byte-EQ caps at this same surface (**any frame where `tOp` is wrong ⇒ P1 cannot match**, modulo coincidence). OQ-WINDOW remains pinned and is **not** in the Phase 2c escalation chain — its closure path is upstream (Phase 2b open-loop).

### 4.5 OQ-XB-NORM — untested escalation (carryover)

`xb` (backward-filtered target) Q-format normalization: §A.3.7 eq. A.6 phrases `xb(n) = Σ x(i) · h(i−n)` without specifying a per-frame normalisation shift. Current implementation uses a fixed Q-shift derived from the impulse-response peak; an alternative per-frame adaptive shift was **not** exercised under any I5 slot at INT-1. Listed as **OQ-XB-NORM untested** for Phase 2c re-entry (after Phase 2d closes ENC-INT).

---

## 5. INT-1 byte-EQ disposition — FAIL-DEFERRED

**Final corpus numbers (1835 frames, `TestPhase2cINT1_ClosedLoopPitchByteEQ`):**

| Field | Match / Total | Rate | ACCEPT-PARTIAL @ 80 % | FAIL @ 50 % | Disposition |
|---|---:|---:|:---:|:---:|---|
| **P1** (8 b, §3.7.2 eq. 41) | **166 / 1835** | **9.05 %** | ✗ | ✗ | **FAIL-DEFERRED** |
| **P0** (1 b parity)         | **1036 / 1835** | **56.46 %** | ✗ | — | **BELOW** ACCEPT-PARTIAL |
| **P2** (5 b, §3.7.2 eq. 42) | **179 / 1835** | **9.75 %** | ✗ | ✗ | **FAIL-DEFERRED** |
| Frames panicked            | 0 / 1835 | 0 % | — | — | ✅ |

**Rationale for FAIL-DEFERRED rather than ACCEPT-PARTIAL or further I5 escalation.** All three fields sit far below the ACCEPT-PARTIAL @ 50 % floor and the residual delta histograms are **structural, not constant-tunable**:

- **P1 Δ histogram (≥0.5 %):** dominant Δ=0 bucket at 9.0 %, broad symmetric tail Δ ∈ [−20, +13] each 0.5–4 %, plus a sharp outlier spike at Δ=+168 (0.5 %). The Δ=+168 spike is the wrap-around signature of a P1 byte boundary at the 85 ↔ 86 Annex A range break — characteristic of the **H-CENTER** miscentring (Phase 2b carryover, see below).
- **P2 Δ histogram (≥0.5 %):** Δ=0 dominant (9.8 %), Δ=+1 second (6.2 %), then a heavy negative-bias tail Δ ∈ [−29, −1] each 0.6–4 %. P2 is delta-coded relative to subframe-1 lag, so any subframe-1 byte-EQ miss cascades into P2.
- **P0 (parity):** 56.46 % is barely above the 50 % chance floor for a 1-bit parity that is byte-EQ-coupled to P1; under independent P1, P0 ≈ 50 %.

These patterns are inconsistent with any single OQ tuning constant (which would produce concentrated harmonic-band spikes). They require closure of two upstream/downstream structural blockers documented in §6.

**I5 budget consumption at INT-1:** **1 / 5 used.** Slot ledger:

| Slot | Disposition | Outcome |
|-----:|-------------|---------|
| 0/5 | Baseline (`minSafeCentre = 45` workaround for OQ-K<40) | P1 2.07 % / P0 51.28 % / P2 3.71 % — FAIL/BELOW/FAIL. |
| 1/5 | OQ-K<40 LP-residual extension refactor (anchor u(0) at `exc[len−SubframeLen]`; fill trailing 40 with current-subframe `r(n)`) | **P1 9.05 % / P0 56.46 % / P2 9.75 %** — FAIL/BELOW/FAIL. **STOPPED** per FAIL contract (no closed-loop-only escalation can break 50 % while H-CENTER and OQ-EXC-COMMIT remain open). |

**Phase 2d INT-1b re-baseline (post eq. A.9/A.10 commit, HEAD `b85a6d6`):** re-running `TestPhase2cINT1_ClosedLoopPitchByteEQ` after Phase 2d INT-0 wired the full eq. A.9 excitation commit (`u(n) = ĝp·v(n) + ĝc·c(n)`) and eq. A.10 weighted-error update yields:

> **Amendment 2026-05-12 (Phase 2d INT-3 closure).** This sub-section is the §5 disposition amendment authored at Phase 2d closure (`docs/superpowers/plans/2026-05-12-phase2d-closure-report.md`); the original Phase 2c §5 prose above (lines 127–151) is **unchanged**. Phase 2c INT-1 disposition remains FAIL-DEFERRED post Phase 2d (re-baselined; not flipped). No I5 spent on Phase 2c reserved slots 2/5–5/5 (4/4 still reserved).

| Field | Phase 2c baseline | Phase 2d INT-1b | Δ | Disposition |
|---|---:|---:|---:|---|
| P1 (8 b) | 9.05 % (166/1835) | **10.79 % (198/1835)** | **+1.74 pp** | **FAIL-DEFERRED** (still < 50 %) |
| P0 (parity) | 56.46 % (1036/1835) | **57.49 % (1055/1835)** | +1.03 pp | BELOW ACCEPT-PARTIAL |
| P2 (5 b) | 9.75 % (179/1835) | **11.66 % (214/1835)** | **+1.91 pp** | **FAIL-DEFERRED** (still < 50 %) |

The OQ-EXC-COMMIT closure produces a measurable but **structurally minor** uplift (~+1.7–1.9 pp on P1/P2). The Δ=0 buckets remain dominant at the same magnitudes (P1 10.8 %, P2 11.7 %), and the broad symmetric P1 tail / heavy negative-bias P2 tail are unchanged in shape — confirming the residual blocker is **H-CENTER** (Phase 2b open-loop `tOp` miscentring caps P1 byte-EQ at the 53.95 % open-loop plausibility surface), not OQ-EXC-COMMIT corruption of `oldExc`. P1 wrap-around outlier signatures (Δ=+170/+196/+199 buckets at 0.5–0.9 %) persist — same byte-boundary aliasing previously logged as the H-CENTER smoking gun.

**Phase 2d INT-1b verdict: FAIL-DEFERRED — re-baselined.** Per Phase 2d sub-plan §5 INT-1b decision tree (P1 < 50 %): structural blocker still dominant. The expected uplift from cascading Phase 2c reserved I5 slots 2/5–5/5 (H-CENTER → H-PHASE → OQ-WINDOW → OQ-XB-NORM) is **not justified at the closed-loop layer** because:

- H-CENTER's root cause is upstream in Phase 2b open-loop (`tOp` divergence on ~46 % of frames); a closed-loop-side probe cannot move `tOp`.
- H-PHASE / OQ-WINDOW / OQ-XB-NORM are second-order tunings whose combined upper-bound recovery is ≪ 39 pp (the gap to 50 %).

**Phase 2c reserved I5 budget unchanged: 4 / 5 still reserved** (zero consumed by INT-1b). Slots remain available for any future Phase 2-final escalation that re-opens the surface with stronger expected uplift (e.g., post-Phase-2b H-CENTER fix at the open-loop layer).

**Recommendation.** Re-run Phase 2c INT-1 (`TestPhase2cINT1_ClosedLoopPitchByteEQ`) after Phase 2d closes its ENC-INT (full encoder excitation commit). If P1 ≥ 80 % at that point, the INT-1 surface flips ACCEPT-PARTIAL or PASS and Phase 2c can be re-CLOSED non-DEFERRED. If still <80 %, spend the remaining 4 I5 slots against the structural blocker chain in §6.

---

## 6. Structural blockers (LIVE-DEFERRED)

### 6.1 H-CENTER (Phase 2b carryover)

`tOp` open-loop output diverges from the reference T1 in 46 % of frames (Phase 2b INT-1 plausibility 53.95 %); the closed-loop search window `[tOp−5, tOp+4]` is therefore miscentred for those frames, so **no amount of closed-loop search refinement can recover them** — the reference T1 is simply outside the search window. This caps Phase 2c P1 byte-EQ at the Phase 2b open-loop plausibility surface.

**Closure path:** Phase 2b `internal/pitch/openloop/` is frozen under I6, but H-CENTER may close as a side effect of Phase 2d encoder-symmetry diagnostics (the `internal/decoder` `TestDiagnostic_SinglePulseChain` re-evaluation gate — see master plan §5 line 1021).

### 6.2 OQ-EXC-COMMIT (Phase 2d coupling)

Per §A.3.10 eq. A.10 the past-excitation commit is `u(n) = ĝp · v(n) + ĝc · c(n)` — the adaptive contribution **plus** the fixed-codebook contribution. Phase 2c's `closedloopStep` only commits the **adaptive** half (`ĝp · v(n)`) into `oldExc`; the fixed-codebook half (`ĝc · c(n)`) is a Phase 2d responsibility and is currently zero. Consequence: starting at frame 1 (and compounding every subframe), `oldExc` lacks the residual pulse-train energy and the next subframe's CL-1 backward correlation `RN(k)` is biased toward shorter lags. This is consistent with the heavy negative-Δ tails in both the P1 and P2 histograms.

**Closure path:** Phase 2d's fixed-codebook search closes this loop. Until Phase 2d's gain-quantization + excitation commit is wired (Phase 2e), the `oldExc` write at frame end is structurally incomplete.

### 6.3 H-PHASE (Phase 2b carryover)

`swMem[10]` phasing across the frame boundary: TG-1 reads `swMem` at subframe-1 entry, but the I3 commit-at-frame-end discipline means subframe-2 reads `swMem` *as updated by subframe-1 inside the frame* (held in scratch, applied to encoder state at frame end). The interaction with the equivalent decoder-side `swMem` slide (§A.3.6 line 2120) was not re-verified at INT-1; whether the subframe-2 `swMem` should be *also* held in scratch (current behaviour) or pre-committed (alternative) is not unambiguous from §A.3.6 alone.

**Closure path:** instrumentation hook recommended in Phase 2c re-entry plan: log per-frame `(swMem[0..9] @ sub-1 entry, @ sub-2 entry, @ frame end)` for any frame where Phase 2c P1/P2 byte-EQ fails after Phase 2d. If subframe-2 `swMem` shifts vs the alternative pre-commit ordering correlate with P2 mismatches, H-PHASE flips to a Phase 2c production fix.

---

## 7. Engineering invariants pinned

- **I1 (clean-room):** All citations in production code and tests point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` or to our own prior plans/docs. No third-party G.729 source consulted during the OQ-K<40 escalation.
- **I3 (per-frame state mutation only at frame end):** `(*Encoder).closedloopStep` writes scratch arrays during the subframe; the encoder commits `oldExc` and `swMem` only at frame end. Verified by INT-2 race-detector run (no intra-subframe shared-state mutation reported).
- **I4 (zero-alloc on hot path):** Pinned by INT-2 (commit `730bf43`):
  - `closedloopStep(0)` `AllocsPerRun == 0` ✅.
  - `closedloopStep(1)` `AllocsPerRun == 0` ✅.
  - `lpcStep + openloopStep + 2× closedloopStep` (full hot path / frame) `AllocsPerRun == 0` ✅.
  - `BenchmarkClosedloopStep` 14964 ns/op (2 subframes / op, AMD EPYC 9554P).
- **Race-detector clean:** `go test ./... -race` reports zero new `DATA RACE` events beyond the documented baseline.
- **I-2c-1 (Annex A binding):** RN(k) numerator-only per §A.3.7 eq. A.7 (not the §3.7 normalised eq. 37); b30 1/3 frac per §A.3.7 eq. A.8 + §3.7.1 eq. 40 (not §3.7 eq. 39 b12). Comment audit clean across `internal/pitch/closedloop/`.
- **I-2c-2 (quantized-Â discipline):** All closed-loop filters consume Â reconstructed via `internal/lsp.LSPToLP`. Closes Phase 2b H-OQ2.
- **I6 (production-freeze for Phase 2c INT-1 surface):** **ACTIVE under FAIL-DEFERRED.** No further INT-1 production fixes will be attempted under Phase 2c; the Phase 2c surface is the production-correct entry for Phase 2d. Re-entry condition: post-Phase-2d INT-1 re-run.
- **I8:** Each Phase 2c commit carries the prescribed `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **I9 (LSP codebook discipline):** `internal/tables/lsp_*.go` unmodified across Phase 2c.
- **I10 (encoder-decoder state isolation):** `internal/pitch/closedloop/` does not import `internal/pitch/{adaptive,delay}`; only `internal/pitch/parity.go` is consumed (read-only) by ENC-1.

---

## 8. I5 budget accounting

**Per-gate budget (Phase 2c INT-1):** **1 / 5 used.** Slots 2–5 are **reserved** for the post-Phase-2d Phase 2c INT-1 re-run (per §5 recommendation). The Phase 2a 1/5 preserved Phase 2-final escape slot is *not* affected; it remains reserved for the G.192 byte-EQ end-game per `2026-05-06-phase2a-closure-report.md` §8 line 226.

**I6 (production-freeze for Phase 2c INT-1 surface):** **ACTIVE under FAIL-DEFERRED.** The `internal/pitch/closedloop/` surface, the new `Encoder.closedloopStep` driver, and the four reserved I5 slots are the production-correct reference for Phase 2d entry.

---

## 9. Outstanding items / hand-off to Phase 2d

**State carry from Phase 2c → 2d:**

- `Encoder.aQ12Latest[11]` — quantized Â per subframe; **consumed by Phase 2d §A.3.8** ACELP for impulse-response h[] and backward correlation φ[] computation.
- `Encoder.tOp` — Phase 2b open-loop centre (carry-through, used by Phase 2c only).
- `Encoder.oldExc[154]` — currently committed with `ĝp · v(n)` only at frame end; Phase 2d ENC-INT MUST extend the commit to `ĝp · v(n) + ĝc · c(n)` per §A.3.10 eq. A.10. **OQ-EXC-COMMIT closure path.**
- `closedloop.AdaptiveVector` output `v[40]` and `closedloop.GpAndY` outputs `(gp, y[40])` — Phase 2d consumes both: the ACELP target is `x2(n) = x(n) − ĝp · y(n)` (§A.3.8 / §3.8 line ~1230).
- `closedloop.ImpulseResponse` h[40] — reused by Phase 2d ACELP backward correlation; expose Phase 2c h[40] via the encoder (or recompute per Phase 2d's layout).
- `Encoder.swMem[10]`, `Encoder.lpResidualMem[10]` — perceptual-weighting filter memories; Phase 2d does not touch.

**Carryover OQs / hypotheses ledger:**

| ID | State | Owner |
|----|-------|-------|
| **OQ-WINDOW** | PINNED (Phase 2b open-loop search window) | Phase 2c re-entry, post-Phase-2d |
| **OQ-XB-NORM** | UNTESTED escalation | Phase 2c re-entry, post-Phase-2d |
| **OQ-PHASE** | LIVE (subframe-2 `swMem` pre-commit ordering) | Phase 2c re-entry, post-Phase-2d |
| **OQ-GBOUND** | CLOSED (inclusive `1.2` Q14 = 19661) | Phase 2c GP-1 |
| **OQ-K<40** | ESCALATED 1 (LP-residual extension refactor) | Phase 2c INT-1 escalation 1 |
| **OQ-EXC-COMMIT** | LIVE-DEFERRED (Phase 2d coupling) | Phase 2d ENC-INT |
| **H-CENTER** | LIVE-DEFERRED (Phase 2b carryover) | Phase 2b open-loop or Phase 2d encoder-symmetry diagnostics |
| **H-PHASE** | LIVE-DEFERRED (Phase 2b carryover) | Phase 2c re-entry, post-Phase-2d |
| **H-OQ2** | RESOLVED at QA-1 | — |

**Inherited baseline FAILs unchanged from Phase 2b closure** (`2026-05-08-phase2b-closure-report.md` §9): 4 known FAILs carried in:

| Test | Package | Source phase |
|------|---------|--------------|
| `TestEncode_LSPVectorBitExact` | `github.com/exedev/g729` | Phase 2a INT-1 ACCEPT-PARTIAL |
| `TestDiagnostic_SinglePulseChain` | `github.com/exedev/g729/internal/decoder` | Phase 1 inheritance |
| `TestDecode_LowEnergyCodebookIsSmooth` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |
| `TestDecode_SucceedsAcrossAllGainIndices` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |

Phase 2c adds **one new FAIL-DEFERRED** test:

| Test | Package | Disposition |
|------|---------|-------------|
| `TestPhase2cINT1_ClosedLoopPitchByteEQ` | `github.com/exedev/g729` | FAIL-DEFERRED (P1 9.05 % / P0 56.46 % / P2 9.75 %) — re-run post-Phase-2d. |

**Total baseline at Phase 2c closure: 5 FAILs (4 inherited + 1 new INT-1 FAIL-DEFERRED).**

### Test baseline (`go test ./... -race`, HEAD `730bf43`)

| Package | Status |
|---------|--------|
| `github.com/exedev/g729` | **FAIL** (`TestEncode_LSPVectorBitExact`, `TestPhase2cINT1_ClosedLoopPitchByteEQ`) |
| `github.com/exedev/g729/internal/acelp` | PASS |
| `github.com/exedev/g729/internal/bitstream` | PASS |
| `github.com/exedev/g729/internal/decoder` | **FAIL** (`TestDiagnostic_SinglePulseChain`) |
| `github.com/exedev/g729/internal/fcb` | PASS |
| `github.com/exedev/g729/internal/filter` | PASS |
| `github.com/exedev/g729/internal/fixed` | PASS |
| `github.com/exedev/g729/internal/gain` | **FAIL** (`TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) |
| `github.com/exedev/g729/internal/lpc` | PASS |
| `github.com/exedev/g729/internal/lsp` | PASS |
| `github.com/exedev/g729/internal/pcm` | PASS |
| `github.com/exedev/g729/internal/pitch` | PASS |
| `github.com/exedev/g729/internal/pitch/closedloop` | PASS |
| `github.com/exedev/g729/internal/pitch/openloop` | PASS |
| `github.com/exedev/g729/internal/postfilter` | PASS |
| `github.com/exedev/g729/internal/synth` | PASS |
| `github.com/exedev/g729/internal/tables` | PASS |

`go vet ./...` ✅ clean. `go build ./...` ✅ clean.

---

## 10. Phase 2 next-step recommendation

**Next dispatch: author the Phase 2d sub-plan** (`docs/superpowers/plans/YYYY-MM-DD-phase2d-acelp-plan.md`).

Phase 2d — Fixed-codebook ACELP search per §A.3.8 — covers:

- `internal/acelp.Searcher.Search` G.729A fast ACELP depth-first focused search (4 pulses on interleaved tracks T0..T3, 17-bit codeword, correlation φ[] precomputation, sign pre-decision per track).
- §3.8.1 impulse response h[] reuse from Phase 2c's `closedloop.ImpulseResponse`.
- ACELP target `x2(n) = x(n) − ĝp · y(n)` from Phase 2c outputs.
- **OQ-EXC-COMMIT closure** as a Phase 2d ENC-INT requirement: extend `oldExc` commit to `ĝp · v(n) + ĝc · c(n)` per §A.3.10 eq. A.10.
- **Phase 2c INT-1 re-run** as a Phase 2d ENC-INT side-effect: after `oldExc` commit becomes complete, re-execute `TestPhase2cINT1_ClosedLoopPitchByteEQ` and re-evaluate Phase 2c disposition (§5 recommendation).

The Phase 2d sub-plan should explicitly carry the Phase 2c LIVE-DEFERRED entries (OQ-EXC-COMMIT, H-CENTER, H-PHASE, OQ-WINDOW, OQ-XB-NORM) and pre-allocate one Phase 2c re-entry I5 slot for the post-Phase-2d INT-1 re-run.

**Phase 2-final reminder:** the strict G.192 byte-EQ gate remains a Phase 2-final concern; Phase 2c's contribution is the P1/P0/P2 fields specifically, which remain FAIL-DEFERRED until Phase 2d ENC-INT closes.

---

— end of Phase 2c closure report —
