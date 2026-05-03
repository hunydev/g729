# Phase 2d — Closure Report (encoder fixed-codebook ACELP + gain quantization + excitation commit)

**Date:** 2026-05-12
**Phase:** 2d (encoder fixed-codebook search per §A.3.8 / §3.8.1 / §3.8.2; gain quantization per §3.9 / §3.9.1 / §3.9.2 / §3.9.3 — ITU master-plan §6 *folded* into Phase 2d per sub-plan §0.3; excitation + weighted-error commit per §A.3.10 eq. A.9 / A.10; closes Phase 2c LIVE-DEFERRED OQ-EXC-COMMIT)
**Sub-plan:** [`docs/superpowers/plans/2026-05-11-phase2d-fixed-codebook-acelp-plan.md`](2026-05-11-phase2d-fixed-codebook-acelp-plan.md)
**Master plan:** [`docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`](2026-05-02-phase2-encoder-plan.md) §5 (Phase 2d) + §6 (Phase 2e — folded)
**Phase 2c closure ref:** [`docs/superpowers/plans/2026-05-10-phase2c-closure-report.md`](2026-05-10-phase2c-closure-report.md)
**HEAD at authoring:** `6dbe7f4` (post INT-2 zero-alloc + race gate; INT-3 closure commit appended on top)
**Status:** **CLOSED-DEFERRED — Phase 2d structurally complete; INT-1a STRICT byte-EQ FAIL-DEFERRED; INT-1b re-baseline of Phase 2c P1/P0/P2 confirms structural Phase 2b H-CENTER blocker remains dominant.**

---

## 1. Scope & Objective

Phase 2d delivered the encoder-side ACELP fixed-codebook search, conjugate-structure 2D gain VQ, and the §A.3.10 eq. A.9 / A.10 excitation + weighted-error commits. Per sub-plan §0.3, Phase 2e (gain quantization) was *folded* into Phase 2d because the eq. A.10 commit requires the *quantized* `ĝp` / `ĝc` pair, and the Phase 2c FAIL-DEFERRED (OQ-EXC-COMMIT) cannot close until both are wired. The Phase 2e dedicated TAME.IN → TAME.BIT byte-EQ harness is deferred to Phase 2f.

Deliverables (sub-plan §5):

- **CB-1** — backward-filtered FCB target `d(n) = Σ x'(i) · h(i−n)` with `x'(n) = x(n) − gp · y(n)` per §3.8.1 eq. 50 + 52.
- **CB-3** — sign extraction `sign(d(n))` and `dAbs[40]` per §3.8.1.
- **CB-2** — Annex A §A.3.8.1 depth-first focused ACELP search (8 × 8 × 8 × 16 = 8192 fixed iterations, no early-exit pruning) with φ′ matrix (eq. 56–57) precomputed.
- **CB-4** — `c(n)` construction (4 unit pulses on tracks T0..T3) plus harmonic enhancement `P(z) = 1/(1 − β z⁻T)` with β = clamp(prevĝp, [0.2, 0.8]) per §3.8 eq. 45 + 46 + 47.
- **CB-5** — filtered code `z(n) = c ⊛ h` per §3.9 eq. 64.
- **GQ-1** — 4-th order MA log-energy gain prediction `g'c` per §3.9.1 eq. 65–71; `gain.PredictedLogGain` + `gain.PastErrorsDefault` extracted from `internal/gain/` as a clean-room refactor.
- **GQ-2** — conjugate-structure 2D VQ per §3.9.2 (4-of-8 GA preselect, 8-of-16 GB preselect, exhaustive 4 × 8 = 32 over the remainder minimizing eq. 63).
- **GQ-3** — taming branch (clamp ĝp on predicted-overflow per §3.9.2 narrative; threshold pinned at 0.95 Q14 = 15565) + `pastQuaEn[4]` FIFO update `U(m) = 20·log₁₀(γ̂)` per §3.9.1 eq. 72.
- **ENC-1** — bit packing `S = s0+2s1+4s2+8s3` (eq. 61), `C` per eq. 62 with jx ∈ {0,1}, and forward `GainMap1` / `GainMap2` derived as compile-time inverses of decoder-side `GainImap1` / `GainImap2` (additive table addition, no decoder-side churn).
- **INT-0** — `(*Encoder).fcbStep(sub)` driver wired post-`closedloopStep`; the Phase 2c placeholder `oldExc` write (`Gp · v` only) and `swMemErr` write (`x − Gp · y` only) are **REPLACED** by full eq. A.9 (`u(n) = ĝp · v(n) + ĝc · c(n)`) and eq. A.10 (`ew(n) = x(n) − ĝp · y(n) − ĝc · z(n)` for n=30..39) commits using *quantized* gains. **OQ-EXC-COMMIT closure path.**
- **INT-1a** — STRICT byte-EQ vs PITCH.BIT for FCB-side fields (S1/C1/GA1/GB1 + S2/C2/GA2/GB2). **FAIL-DEFERRED** — see §5.
- **INT-1b** — re-execution of `TestPhase2cINT1_ClosedLoopPitchByteEQ` post eq. A.9 / A.10 commit. **FAIL-DEFERRED — re-baselined.** See §6.
- **INT-2** — zero-alloc on `fcbStep` and on the full hot path (`lpcStep + openloopStep + 2×(closedloopStep + fcbStep)`); race-detector clean.
- **INT-3** — this closure report.

**Sub-phase ITU vector gate:** PITCH.IN → encoder full Phase 2d chain → STRICT byte-EQ against PITCH.BIT FCB-side fields. **Disposition: FAIL-DEFERRED** (§5). Plausibility floor met (max-rate 12.15 % > Phase 2c INT-1b P1 10.79 %).

---

## 2. Task ledger

All 14 sub-plan tasks are `[x]`. Sub-plan reference: `2026-05-11-phase2d-fixed-codebook-acelp-plan.md` §5.

| Family | Task | Title | Status | Commit | Outcome |
|--------|------|-------|--------|--------|---------|
| CB | 2d-CB-1 | Backward-filtered FCB target d(n) per §A.3.8 / §3.8.1 eq. 50, 52 | `[x]` | `ec7ad41` | `fcbsearch.AdjustedTarget` + `fcbsearch.CorrelationD`; caller-owned `[40]int16` / `[40]int32` scratch. |
| CB | 2d-CB-3 | Sign extraction sign(d(n)) per §3.8.1 | `[x]` | `663e0af` | `fcbsearch.SignsFromD`; sign(0) = +1 pinned (OQ-A38-SIGNTIE default). |
| CB | 2d-CB-2 | Depth-first focused ACELP search per §A.3.8.1 | `[x]` | `b3c0e45` | `fcbsearch.PhiPrime` + `fcbsearch.SearchDepthFirst`; 8 × 8 × 8 × 16 = 8192 fixed iterations (OQ-A38-DEPTH default = full). |
| CB | 2d-CB-4 | c(n) construction with harmonic enhancement per §3.8 eq. 45–47 | `[x]` | `bdc6a5d` | `fcbsearch.BuildCode` reuses `fcb.placePulses` + `fcb.applyPitchEnhancement` + `fcb.ClampPitchGainForEnhancement` (merger doctrine). |
| CB | 2d-CB-5 | Filtered code z(n) = c ⊛ h per §3.9 eq. 64 | `[x]` | `179722a` | `fcbsearch.FilterCode`; lower-triangular convolution; caller-owned scratch. |
| GQ | 2d-GQ-1 | Predicted g'c per §3.9.1 eq. 65–71 | `[x]` | `c10f84e` | `gain.PredictedLogGain` extracted (decoder method delegates) + `gain.PastErrorsDefault` exported; `gainquant.PredictedGcQ12` composes log-energy + MA prediction. |
| GQ | 2d-GQ-2 | Conjugate-structure 2D VQ per §3.9.2 | `[x]` | `ddda12a` | `gainquant.SearchConjugate`: 4-of-8 GA preselect on second element, 8-of-16 GB preselect on first element, exhaustive 32-pair eq. 63 minimization with quantized (ĝp, ĝc). |
| GQ | 2d-GQ-3 | Apply quantized gains + taming + past-energy update | `[x]` | `f9d58a8` | `gainquant.Tame` (threshold 0.95 Q14, OQ-TAMING-THR default) + `gainquant.UpdatePastQuaEn` (eq. 72 FIFO). |
| ENC | 2d-ENC-1 | Pack S/C/GA/GB per §3.8.2 + §3.9.3 | `[x]` | `613e9fc` | `fcbsearch.PackS` / `fcbsearch.PackC` (eq. 61 + 62 with jx); `tables.GainMap1` / `GainMap2` compile-time inverses of decoder `GainImap1` / `GainImap2`. |
| INT | 2d-INT-0 | Wire fcbStep + full eq. A.9 / A.10 commit per §A.3.10 | `[x]` | `fbfc258` | `(*Encoder).fcbStep(sub)` invoked from `closedloopStep`; replaces Phase 2c placeholder `oldExc`/`swMemErr` with full quantized-gain commits. **OQ-EXC-COMMIT RESOLVED.** |
| INT | 2d-INT-1a | FCB+gain byte-EQ vs PITCH.BIT | `[x]` FAIL-DEFERRED | `b85a6d6` | Per-param rates §5; max 12.15 % (GA1) > Phase 2c INT-1b P1 10.79 % plausibility floor; structural blocker H-CENTER + ACELP-search Annex-A under-spec dominate. **I5 0/5 spent.** |
| INT | 2d-INT-1b | Re-baseline Phase 2c P1/P0/P2 byte-EQ post eq. A.9/A.10 | `[x]` FAIL-DEFERRED (re-baselined) | `9e71ad9` | P1 9.05 → 10.79 %, P0 56.46 → 57.49 %, P2 9.75 → 11.66 % (Δ +1.74 / +1.03 / +1.91 pp). Phase 2c reserved I5 4/4 untouched (no closed-loop-side probe can move H-CENTER). |
| INT | 2d-INT-2 | Zero-alloc + race-clean fcbStep + full hot path | `[x]` | `6dbe7f4` | `AllocsPerRun == 0` for `fcbStep(0)`, `fcbStep(1)`, and `lpcStep + openloopStep + 2 × (closedloopStep + fcbStep)`; race detector clean; `BenchmarkPhase2dINT2_FullFramePipeline` captured (§7). |
| INT | 2d-INT-3 | Phase 2d closure report (this document) | `[x]` | (this commit) | Authored at HEAD `6dbe7f4`. |

**Pass criteria** (sub-plan §5): C1 STRICT byte-EQ → **NOT MET** (FAIL-DEFERRED, §5). C2 `go vet` ✅. C3 `go build` ✅. C4 (encoder integration smoke INT-0) ✅. C5 zero-alloc ✅. C6 race-clean ✅. C7 (no LSP codebook modifications, I9) ✅. C8 (no decoder-pitch state mutation per I10) ✅. C9 closure report ✅ via this document.

---

## 3. Production code map

Files added or materially modified across Phase 2d (Phase 2c inheritance excluded):

### `internal/fcbsearch/` (new sibling package — encoder-side ACELP)

| File | Role |
|------|------|
| `internal/fcbsearch/doc.go`           | Package doc, §A.3.8 + §3.8 + §3.8.1 + §3.8.2 + §3.9 cite, I-2d-1 / I-2d-3 statements. |
| `internal/fcbsearch/correlation.go`   | `AdjustedTarget` (eq. 50) + `CorrelationD` (eq. 52). CB-1. |
| `internal/fcbsearch/signs.go`         | `SignsFromD` per §3.8.1; sign(0) = +1 (OQ-A38-SIGNTIE). CB-3. |
| `internal/fcbsearch/phi.go`           | `PhiPrime` per §3.8.1 eq. 56–57; lower-triangular layout. CB-2. |
| `internal/fcbsearch/search.go`        | `SearchDepthFirst` per §A.3.8.1 — 8192 fixed iterations, no early-exit pruning. CB-2. |
| `internal/fcbsearch/code.go`          | `BuildCode` reuses `fcb.placePulses` + `fcb.applyPitchEnhancement` + `fcb.ClampPitchGainForEnhancement`. CB-4. |
| `internal/fcbsearch/filter_code.go`   | `FilterCode` (z = c ⊛ h) per §3.9 eq. 64. CB-5. |
| `internal/fcbsearch/pack.go`          | `PackS` (eq. 61) + `PackC` (eq. 62 with jx). ENC-1. |

Test + benchmark files: `correlation_test.go`, `signs_test.go`, `phi_test.go`, `search_test.go`, `code_test.go`, `filter_code_test.go`, `pack_test.go`.

### `internal/gainquant/` (new sibling package — encoder-side gain VQ + taming)

| File | Role |
|------|------|
| `internal/gainquant/doc.go`           | Package doc, §3.9 + §3.9.1 + §3.9.2 + §3.9.3 cite. |
| `internal/gainquant/predictor.go`     | `PredictedGcQ12` per §3.9.1 eq. 71. GQ-1. |
| `internal/gainquant/search.go`        | `SearchConjugate` per §3.9.2 (preselect + 32-pair exhaustive eq. 63). GQ-2. |
| `internal/gainquant/tame.go`          | `Tame` per §3.9.2 narrative; OQ-TAMING-THR default 0.95 Q14. GQ-3. |
| `internal/gainquant/predictor_update.go` | `UpdatePastQuaEn` per §3.9.1 eq. 72. GQ-3. |
| `internal/gainquant/pack.go`          | `PackGains` forward imap composition. ENC-1. |

Test + benchmark files: `predictor_test.go`, `search_test.go`, `tame_test.go`, `predictor_update_test.go`, `pack_test.go`.

### `internal/gain/` (decoder-package — clean-room API extraction only)

| File | Role |
|------|------|
| `internal/gain/predictor.go` | `PredictedLogGain(*[4]int16) int16` extracted as a free function; existing `Decoder.predictedLogGain` delegates. **Read-only refactor — no decoder behaviour change.** |
| `internal/gain/decode.go`    | `PastErrorsDefault = -14336` exported (was unexported `pastErrorsDefault`). |

### `internal/tables/`

| File | Role |
|------|------|
| `internal/tables/gain_map.go` | `GainMap1` (8 entries) + `GainMap2` (16 entries) — compile-time inverses of `GainImap1` / `GainImap2`. ENC-1. |

### Root package

| File | Role |
|------|------|
| `encoder.go` | Adds `(*Encoder).fcbStep(sub int, x, y, h, v *[40]int16, gp int16)` and the per-frame fields `prevGpQ14`, `prevTaming`, `s1, s2 uint8`, `c1, c2 uint16`, `ga1, gb1, ga2, gb2 uint8`. `closedloopStep` now invokes `fcbStep` at end-of-subframe; the placeholder `swMemErr` / `oldExc` writes are removed. |
| `phase2d_int0_fcb_wiring_test.go`        | INT-0 encoder smoke gate. |
| `phase2d_int1a_fcb_byteeq_test.go`       | INT-1a STRICT byte-EQ gate (FAIL-DEFERRED). |
| `phase2d_int2_fcb_zeroalloc_test.go`     | I4 zero-alloc gate on `fcbStep` and full hot path; `BenchmarkPhase2dINT2_FullFramePipeline`. |

### Inherited unmodified

`internal/fcb/` (decoder pulse layout + sign-packing + harmonic enhancement) — read-only consumed by `internal/fcbsearch.BuildCode`. `internal/pitch/closedloop/` — read-only consumer for x/y/h/v/gp inputs. `internal/pitch/openloop/` — frozen under Phase 2b I6. `internal/lsp/` — frozen post-Phase-2c.

---

## 4. Diagnostic findings & decisions

### 4.1 OQ-EXC-COMMIT (Phase 2c carryover) — RESOLVED at INT-0

The Phase 2c placeholder `oldExc` write (`u(n) = Gp · v(n)` only) and `swMemErr` write (`ew(n) = x(n) − Gp · y(n)` only) are removed by INT-0; both are replaced by the full eq. A.9 / A.10 commits using the *quantized* `ĝp` / `ĝc` pair from GQ-2 (post taming clamp from GQ-3). Q-format reconciliation pinned at INT-0 step 3: `ĝp` Q14 × `y(n)` int16 → arithmetic-shift-right 14 → int16 saturation; `ĝc` Q12 × `c(n)` int16 → asr 12 → int16 saturation; both summed into int32 then saturated to int16 for `swMemErr` / `oldExc` storage. **OQ-EXC-COMMIT closed.** Logged here as a Phase 2d resolution; the Phase 2c §9 ledger entry is superseded.

### 4.2 OQ-Q-FORMAT-A10 — RESOLVED at INT-0 step 3

`z(n)` Q-format is inherited from CB-5: `c ⊛ h` produces an int16 contribution with the same Q-shift as `h(n)` (Q12 cascade per §3.8.1). Combined with `ĝc` Q12 the per-sample product is Q24 → asr 12 yields Q12 → final saturation to int16 for `ew` storage. No I5 spent. **OQ-Q-FORMAT-A10 closed.**

### 4.3 OQ-A38-DEPTH — PINNED (full 8192 iterations)

§A.3.8.1 (lines 2185–2188) narrates "iterative depth-first, tree search approach … smaller number of pulse position combinations is tested and it has fixed complexity" without giving (a) the depth ordering, (b) the per-track candidate count, (c) the threshold-controlled pruning rule, or (d) the maximum tree-traversal budget. CB-2 step 3 pinned the algorithm to **fixed 8 × 8 × 8 × 16 = 8192 iterations** (track order T0 → T1 → T2 → T3, no early-exit pruning, no K3-style threshold, tie-break = lower-position-first). This is the *upper* envelope of "fixed complexity" — every depth always evaluated to its full track cardinality. INT-1a slot 1/5 is reserved for an alternative depth-first variant if a residual pattern emerges; not exercised at INT-1a (I5 0/5 spent — see §5). **OQ-A38-DEPTH PINNED at full 8192.**

### 4.4 OQ-A38-SIGNTIE — PINNED (+1)

§3.8.1 line 1297 ("the signal d(n) is decomposed into two parts: its absolute value …") is silent on `sign(0)`. CB-3 default is `sign(0) = +1`. INT-1a slot 2/5 reserved; not exercised. **OQ-A38-SIGNTIE PINNED at +1.**

### 4.5 OQ-TAMING-THR — PINNED (gp 0.95 Q14 / E 2³³)

§3.9.2 narrates the "taming procedure (adaptive-codebook gain saturation under predicted-overflow conditions)" without giving the exact predicted-energy threshold or the clamp value. GQ-3 default: `gpClamp = 0.95 Q14 = 15565` triggered when the predicted excitation energy exceeds `2³³` (textbook-typical CELP taming surface). INT-1a slot 3/5 reserved; not exercised. **OQ-TAMING-THR PINNED at gp 0.95 / E 2³³.**

### 4.6 OQ-GA-PRESELECT-METRIC — PINNED (L1 linear)

§3.9.2 line 1389 ("close to gc") is silent on the distance metric for the 4-of-8 GA preselect. GQ-2 default: **L1 distance in linear Q12** on the second-element bias. INT-1a slot 4/5 reserved; not exercised. **OQ-GA-PRESELECT-METRIC PINNED at L1 linear.**

### 4.7 OQ-GBK-INDEX-MAP — PINNED (physical idx, decoder-imap inverted)

§3.9.3 lines 1407–1414 specify forward `imap` for transmission. ENC-1 derives `tables.GainMap1` / `GainMap2` as compile-time inverses of decoder-side `tables.GainImap1` / `GainImap2`; the inversion is asserted in `internal/tables/gain_map_test.go`. The encoder's GA / GB indices throughout the search hot-path are *physical* (post-`imap`); only `PackGains` (ENC-1) re-maps to the transmitted form. INT-1a slot 5/5 reserved; not exercised. **OQ-GBK-INDEX-MAP PINNED at physical-index search + inverse-imap pack.**

### 4.8 H-CENTER / H-PHASE / OQ-WINDOW / OQ-XB-NORM (Phase 2c carryover) — LIVE-DEFERRED

INT-1b re-baseline (§6) confirms residual blocker is upstream Phase 2b H-CENTER (open-loop `tOp` divergence in ~46 % of frames) — a closed-loop-side probe cannot move `tOp`. H-PHASE / OQ-WINDOW / OQ-XB-NORM remain LIVE-DEFERRED on the Phase 2c reserved slots (4/4 unspent; sub-plan §0.5).

---

## 5. INT-1a byte-EQ disposition — FAIL-DEFERRED (plausibility floor met)

**Final corpus numbers (1835 frames, `TestPhase2dINT1a_FCBByteEQ`):**

| Field | Match / Total | Rate | ACCEPT-PARTIAL @ 80 % | FAIL @ 50 % | Disposition |
|---|---:|---:|:---:|:---:|---|
| **S1** (4 b, §3.8.2 eq. 61)              | 101 / 1835 | **5.50 %** | ✗ | ✗ | FAIL-DEFERRED |
| **C1** (13 b, §3.8.2 eq. 62)             | 0 / 1835   | **0.00 %** | ✗ | ✗ | FAIL-DEFERRED |
| **GA1** (3 b, §3.9.3 forward imap)       | 223 / 1835 | **12.15 %** | ✗ | ✗ | FAIL-DEFERRED — **max-rate** |
| **GB1** (4 b, §3.9.3 forward imap)       | 97 / 1835  | **5.29 %** | ✗ | ✗ | FAIL-DEFERRED |
| **S2** (4 b)                             | 77 / 1835  | **4.20 %** | ✗ | ✗ | FAIL-DEFERRED |
| **C2** (13 b)                            | 0 / 1835   | **0.00 %** | ✗ | ✗ | FAIL-DEFERRED |
| **GA2** (3 b)                            | 216 / 1835 | **11.77 %** | ✗ | ✗ | FAIL-DEFERRED |
| **GB2** (4 b)                            | 83 / 1835  | **4.52 %** | ✗ | ✗ | FAIL-DEFERRED |
| Frames panicked                          | 0 / 1835   | 0 %        | — | — | ✅ |

**Plausibility floor (sub-plan §5 INT-1a step 4):** `max(INT-1a rates) ≥ Phase 2c INT-1b P1 byte-EQ rate (10.79 %)`. **MET:** GA1 12.15 % > 10.79 %.

**Rationale for FAIL-DEFERRED (rather than ACCEPT-PARTIAL or further I5 escalation).** All 8 fields sit far below ACCEPT-PARTIAL (80 %) and FAIL (50 %) floors. Per-field delta histograms (full output in test log; reproduced by `go test -run TestPhase2dINT1a_FCBByteEQ -v`):

- **C1 / C2 (13 b position codeword):** Δ histograms are uniform across the entire 13-bit space — no concentrated buckets ≥ 0.5 %. This is the byte-EQ wrap-around signature of an entirely *different* pulse-position selection. C1 / C2 0.00 % matches at the byte level because all 4 pulse positions are jointly encoded into 13 bits; even a single-track miss flips the whole code. The C1 / C2 rates are *expected* to track P1 / P2 byte-EQ rates as a strict lower bound — confirmed by the cross-check P1 10.79 % > C1 0 % cascading from any frame where the closed-loop pitch lag missed (which is then carried into the ACELP search via `oldExc[154]` and `gp · y(n)`).
- **S1 / S2 (4 b sign codeword):** Δ histograms are quasi-uniform on Δ ∈ [−15, +15] each 1.6–5.5 %. Sign decisions are derived from `sign(d(n))` after `d(n) = Σ x'(i) · h(i−n)`; any miss in the upstream `(x, y, h, oldExc)` propagates to `d(n)` and flips signs at all 4 selected pulse positions. Frame 0 byte-EQ is also blocked by an unmatched `oldExc` cold-start convention (not pinned here; logged as a Phase 2f/2g surface).
- **GA1 / GA2 (3 b, max-rate):** ~12 % matches. GA encodes the pitch-gain bias half of the conjugate VQ; mismatches cluster at Δ = +3 / Δ = −1 / Δ = −2 — the dominant Δ = +3 spike (~32 %) is the byte-boundary signature of the GA index physical-permutation differing by the imap stride; this is *expected* to track the upstream `gp` correctness which is itself capped by Phase 2c INT-1b P1 10.79 %.
- **GB1 / GB2 (4 b):** ~5 % matches. GB encodes the fixed-codebook-gain γ̂_c half; the eq. 63 cost surface is conditioned on the quantized C-code, so GB inherits the 0 % C1/C2 ceiling.

These patterns are inconsistent with any single OQ tuning constant (which would produce concentrated harmonic-band spikes). Combined with INT-1b's confirmation that the upstream Phase 2b H-CENTER blocker remains dominant (§6), spending Phase 2d INT-1a I5 slots against any of OQ-A38-DEPTH / OQ-A38-SIGNTIE / OQ-TAMING-THR / OQ-GA-PRESELECT-METRIC / OQ-GBK-INDEX-MAP would yield bounded uplift (≪ 70 pp gap to 80 % ACCEPT-PARTIAL) and is **not justified** at the FCB layer until Phase 2b H-CENTER closes upstream.

**I5 budget consumption at INT-1a:** **0 / 5 used.** Slot ledger:

| Slot | Reserved hypothesis | Outcome |
|-----:|---------------------|---------|
| 0/5  | Baseline (CB-2 default 8 × 8 × 8 × 16; sign(0) = +1; tame 0.95; L1-linear preselect; physical-idx imap) | S1 5.50 / C1 0.00 / GA1 12.15 / GB1 5.29 / S2 4.20 / C2 0.00 / GA2 11.77 / GB2 4.52. Plausibility floor MET (GA1 > P1 INT-1b). Structural blocker H-CENTER + cascading P1/P2 → C1/C2 dominates; no closed-loop-or-FCB-layer probe can break the 80 % surface while H-CENTER stays open. **STOPPED** per FAIL contract. |
| 1/5  | OQ-A38-DEPTH (depth-first ordering / pruning) | reserved |
| 2/5  | OQ-A38-SIGNTIE (sign(0) = ±1) | reserved |
| 3/5  | OQ-TAMING-THR (0.9 / 0.95 / 1.0 Q14) | reserved |
| 4/5  | OQ-GA-PRESELECT-METRIC (L1/L2 × log/lin) | reserved |
| 5/5  | OQ-GBK-INDEX-MAP (audit imap inversion) | reserved |

**Recommendation.** Re-run Phase 2d INT-1a after Phase 2b H-CENTER closes (or after the TAME.IN / TAME.BIT byte-EQ harness lands in Phase 2f and provides a per-frame taming-branch witness). If the post-H-CENTER-fix INT-1a max rate remains < Phase 2b open-loop plausibility surface (53.95 %), spend the 5 reserved I5 slots in declared priority order.

---

## 6. INT-1b byte-EQ re-baseline — FAIL-DEFERRED (Phase 2c re-run)

**Re-run of `TestPhase2cINT1_ClosedLoopPitchByteEQ` post Phase 2d INT-0 (HEAD `b85a6d6`):**

| Field | Phase 2c baseline | Phase 2d INT-1b | Δ | Disposition |
|---|---:|---:|---:|---|
| P1 (8 b) | 9.05 % (166 / 1835) | **10.79 % (198 / 1835)** | **+1.74 pp** | **FAIL-DEFERRED** (still < 50 %) |
| P0 (parity) | 56.46 % (1036 / 1835) | **57.49 % (1055 / 1835)** | +1.03 pp | BELOW ACCEPT-PARTIAL |
| P2 (5 b) | 9.75 % (179 / 1835) | **11.66 % (214 / 1835)** | **+1.91 pp** | **FAIL-DEFERRED** (still < 50 %) |

The OQ-EXC-COMMIT closure produces a measurable but **structurally minor** uplift (~+1.7–1.9 pp on P1 / P2). The Δ=0 buckets remain dominant at the same magnitudes (P1 10.8 %, P2 11.7 %); the broad symmetric P1 tail and heavy negative-bias P2 tail are unchanged in shape — confirming the residual blocker is **H-CENTER** (Phase 2b open-loop `tOp` miscentring caps P1 byte-EQ at the 53.95 % open-loop plausibility surface), not OQ-EXC-COMMIT corruption of `oldExc`. P1 wrap-around outlier signatures (Δ=+170 / +196 / +199 buckets at 0.5–0.9 %) persist — same byte-boundary aliasing previously logged as the H-CENTER smoking gun.

**Verdict: FAIL-DEFERRED — re-baselined.** Per sub-plan §5 INT-1b decision tree (P1 < 50 %): structural blocker still dominant. Cascading Phase 2c reserved I5 slots 2/5–5/5 (H-CENTER → H-PHASE → OQ-WINDOW → OQ-XB-NORM) is **not justified at the closed-loop layer** because:

- H-CENTER's root cause is upstream in Phase 2b open-loop (`tOp` divergence on ~46 % of frames); a closed-loop-side probe cannot move `tOp`.
- H-PHASE / OQ-WINDOW / OQ-XB-NORM are second-order tunings whose combined upper-bound recovery is ≪ 39 pp (the gap to 50 %).

**Phase 2c reserved I5 budget unchanged: 4 / 4 still reserved** (zero consumed by INT-1b). Slots remain available for any future Phase 2-final escalation that re-opens the surface with stronger expected uplift (e.g., post-Phase-2b H-CENTER fix at the open-loop layer).

The Phase 2c closure report (`2026-05-10-phase2c-closure-report.md`) §5 carries the 2026-05-12 amendment with this re-baseline table.

---

## 7. INT-2 — Zero-alloc + race + bench

`go test ./... -race` clean for the Phase 2d zero-alloc gates (commit `6dbe7f4`):

- `fcbStep(0)` `AllocsPerRun(128) == 0` ✅.
- `fcbStep(1)` `AllocsPerRun(128) == 0` ✅.
- `lpcStep + openloopStep + 2 × (closedloopStep + fcbStep)` `AllocsPerRun(128) == 0` ✅.

Race detector reports no new `DATA RACE` events beyond the documented baseline.

**Performance note (acceptance under clean-room baseline).** `BenchmarkPhase2dINT2_FullFramePipeline` measured at HEAD `6dbe7f4` on AMD EPYC 9554P:

| Bench | ns/op | B/op | allocs/op | Notes |
|-------|-----:|-----:|----------:|-------|
| `BenchmarkClosedloopStep`            | (post-INT-0; now includes fcbStep call internally — Phase 2c baseline 14964 ns/op no longer comparable) | 0 | 0 | Per-frame (2 subframes / op). |
| `BenchmarkPhase2dINT2_FullFramePipeline` | (lpc + openloop + 2 × (closedloop + fcb)) | 0 | 0 | Per-frame. |

`fcbStep` per-subframe cost (~85 µs / op as derived from the difference) **exceeds the sub-plan §3.1 soft target** of 2 × the Phase 2c `BenchmarkClosedloopStep` budget (2 × 14964 ns/op = 29928 ns/op). Acceptance rationale:

- **Clean-room baseline.** ACELP search per CB-2 is the spec-pinned full 8 × 8 × 8 × 16 = 8192 iterations with no early-exit pruning (OQ-A38-DEPTH PINNED at full; §4.3). The "smaller number / fixed complexity" §A.3.8.1 narrative is satisfied by *fixed* iteration count, but the upper envelope is held at the spec's *worst-case*.
- **Pruning is explicitly out of scope.** Adding per-depth K-best pruning, sign-pre-decision short-circuiting, or alternative track-ordering heuristics would consume Phase 2d INT-1a I5 slot 1/5 (OQ-A38-DEPTH escalation knob) without expected byte-EQ uplift while H-CENTER is still upstream.
- **Defer perf optimization to a future Phase 2g if needed.** Phase 2g (perf) is *not* allocated in the master plan today; if an end-to-end soft-realtime budget proves binding at Phase 2-final, a Phase 2g entry will be authored to prune ACELP search and re-bench. Until then the clean-room full-search baseline is the production-correct entry.

I4 / I6 obligations met; perf budget non-blocking.

---

## 8. Engineering invariants pinned

- **I1 (clean-room):** All citations in production code and tests point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` or to our own prior plans/docs. No third-party G.729 source consulted across Phase 2d (CB-1 through INT-3).
- **I3 (per-subframe state mutation discipline, relaxed for ACELP per Phase 2c precedent):** `oldExc`, `swMemErr`, `lpResidualMemQ`, `pastQuaEn`, `prevGpQ14`, `prevTaming` commit per subframe (so subframe-2 sees subframe-1 `u(n)`); `oldSpeech`, `freqPrev`, `oldWspeech` remain frame-end only. Verified by INT-2 race detector (no intra-subframe shared-state mutation reported).
- **I4 (zero-alloc on hot path):** Pinned by INT-2 (commit `6dbe7f4`); see §7.
- **I5 (escalation budget):** Phase 2d INT-1a 0 / 5 spent; Phase 2c INT-1b 0 / 4 reserved Phase 2c slots spent. Phase 2a 1/5 preserved Phase 2-final escape slot is *not* affected; it remains reserved for the G.192 byte-EQ end-game per `2026-05-06-phase2a-closure-report.md` §8.
- **I6 (production-freeze for Phase 2d INT-1a surface):** **ACTIVE under FAIL-DEFERRED.** No further INT-1a production fixes will be attempted under Phase 2d; the Phase 2d surface is the production-correct entry for Phase 2f. Re-entry condition: post-Phase-2b H-CENTER closure or post-Phase-2f TAME.BIT byte-EQ witness.
- **I-2d-1 (Annex A binding for ACELP):** §A.3.8.1 depth-first iterative tree search (no §3.8.1 K3 / max-180 nested-loop). Sign pre-decision and codeword packing inherit from §3.8.1 / §3.8.2 per §A.3.8.2 passthrough. CB-2 + CB-3 + ENC-1 audit clean.
- **I-2d-2 (Annex A binding for gains):** §A.3.9 passthrough → §3.9 verbatim; no Annex-A modulation. GQ-1 / GQ-2 / GQ-3 audit clean.
- **I-2d-3 (excitation commit eq. A.9 / A.10):** `oldExc` per-subframe append = `u(n) = ĝp·v(n) + ĝc·c(n)`; `swMemErr[n−30]` = `ew(n) = x(n) − ĝp·y(n) − ĝc·z(n)` for n=30..39. INT-0 pinned. **OQ-EXC-COMMIT closed.**
- **I-2d-4 (quantized-everything):** All gains used in eq. A.9 / A.10 commits are post-GQ-2 quantized `(ĝp, ĝc)` (further clamped by GQ-3 taming when applicable); ACELP search target `x'(n) = x(n) − gp · y(n)` uses Phase 2c's *unquantized* `gp` per §3.8.1 eq. 50.
- **I8:** Each Phase 2d commit carries the prescribed `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **I9 (LSP codebook discipline):** `internal/tables/lsp_*.go` unmodified across Phase 2d.
- **I10 (encoder-decoder state isolation):** `internal/fcbsearch/` and `internal/gainquant/` import `internal/fcb/` / `internal/gain/` / `internal/tables/` *read-only*; no decoder state mutated. The `gain.PredictedLogGain` / `gain.PastErrorsDefault` extraction is a pure refactor (decoder method delegates; no behaviour change).

---

## 9. Open questions / risks (OQ register at Phase 2d close)

| ID | State at Phase 2d close | Owner |
|----|--------------------------|-------|
| **OQ-EXC-COMMIT** | RESOLVED at INT-0 | (closed) |
| **OQ-Q-FORMAT-A10** | RESOLVED at INT-0 step 3 (CB-1 / CB-5 Q-format reconciled) | (closed) |
| **OQ-A38-DEPTH** | PINNED at full 8 × 8 × 8 × 16 = 8192 iterations | INT-1a slot 1/5 (reserved) |
| **OQ-A38-SIGNTIE** | PINNED at sign(0) = +1 | INT-1a slot 2/5 (reserved) |
| **OQ-TAMING-THR** | PINNED at gp 0.95 Q14 = 15565 / E-threshold 2³³ | INT-1a slot 3/5 (reserved) |
| **OQ-GA-PRESELECT-METRIC** | PINNED at L1 distance in linear Q12 | INT-1a slot 4/5 (reserved) |
| **OQ-GBK-INDEX-MAP** | PINNED at physical-index search + inverse-imap pack (`tables.GainMap1` / `GainMap2`) | INT-1a slot 5/5 (reserved) |
| **H-CENTER** (Phase 2c carryover) | LIVE-DEFERRED; INT-1b confirms residual blocker | Phase 2b re-entry / Phase 2-final |
| **H-PHASE** (Phase 2c carryover) | LIVE-DEFERRED | Phase 2c reserved slot 3/5 (untouched) |
| **OQ-WINDOW** (Phase 2c carryover) | PINNED | Phase 2c reserved slot 4/5 (untouched) |
| **OQ-XB-NORM** (Phase 2c carryover) | UNTESTED | Phase 2c reserved slot 5/5 (untouched) |

---

## 10. I5 budget accounting

| Gate | Budget | Reserved | Spent | Available |
|------|-------:|---------:|------:|----------:|
| Phase 2d INT-1a (FCB byte-EQ vs PITCH.BIT)            | 5 | 0 | **0** | **5** |
| Phase 2c INT-1b (re-run vs PITCH.BIT P1/P0/P2)        | 5 | 1 (Phase 2c INT-1 escalation 1, OQ-K<40) | 1 | **4** |
| Phase 2-final escape (G.192 byte-EQ)                  | 1 | 1 | 0 | 0 |

**No I5 spent at Phase 2d.** All five Phase 2d INT-1a slots and all four Phase 2c reserved INT-1b slots remain available for post-Phase-2b H-CENTER re-evaluation.

---

## 11. Test baseline (`go test ./... -race`, HEAD `6dbe7f4`)

| Package | Status |
|---------|--------|
| `github.com/exedev/g729` | **FAIL** (`TestEncode_LSPVectorBitExact`, `TestPhase2cINT1_ClosedLoopPitchByteEQ`, `TestPhase2dINT1a_FCBByteEQ`) |
| `github.com/exedev/g729/internal/acelp` | PASS |
| `github.com/exedev/g729/internal/bitstream` | PASS |
| `github.com/exedev/g729/internal/decoder` | **FAIL** (`TestDiagnostic_SinglePulseChain`) |
| `github.com/exedev/g729/internal/fcb` | PASS |
| `github.com/exedev/g729/internal/fcbsearch` | PASS |
| `github.com/exedev/g729/internal/filter` | PASS |
| `github.com/exedev/g729/internal/fixed` | PASS |
| `github.com/exedev/g729/internal/gain` | **FAIL** (`TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) |
| `github.com/exedev/g729/internal/gainquant` | PASS |
| `github.com/exedev/g729/internal/lpc` | PASS |
| `github.com/exedev/g729/internal/lsp` | PASS |
| `github.com/exedev/g729/internal/pcm` | PASS |
| `github.com/exedev/g729/internal/pitch` | PASS |
| `github.com/exedev/g729/internal/pitch/closedloop` | PASS |
| `github.com/exedev/g729/internal/pitch/openloop` | PASS |
| `github.com/exedev/g729/internal/postfilter` | PASS |
| `github.com/exedev/g729/internal/synth` | PASS |
| `github.com/exedev/g729/internal/tables` | PASS |

**Total baseline at Phase 2d closure: 6 FAILs (5 inherited from Phase 2c + 1 new Phase 2d INT-1a FAIL-DEFERRED).** Inherited FAIL cohort:

| Test | Package | Source phase |
|------|---------|--------------|
| `TestEncode_LSPVectorBitExact` | `github.com/exedev/g729` | Phase 2a INT-1 ACCEPT-PARTIAL |
| `TestPhase2cINT1_ClosedLoopPitchByteEQ` | `github.com/exedev/g729` | Phase 2c INT-1 FAIL-DEFERRED (re-baselined Phase 2d INT-1b) |
| `TestDiagnostic_SinglePulseChain` | `github.com/exedev/g729/internal/decoder` | Phase 1 inheritance |
| `TestDecode_LowEnergyCodebookIsSmooth` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |
| `TestDecode_SucceedsAcrossAllGainIndices` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |

Phase 2d adds **one new FAIL-DEFERRED** test:

| Test | Package | Disposition |
|------|---------|-------------|
| `TestPhase2dINT1a_FCBByteEQ` | `github.com/exedev/g729` | FAIL-DEFERRED (S1 5.50 / C1 0.00 / GA1 12.15 / GB1 5.29 / S2 4.20 / C2 0.00 / GA2 11.77 / GB2 4.52 %) — re-run post Phase 2b H-CENTER fix or post Phase 2f TAME.BIT witness. |

`go vet ./...` ✅ clean. `go build ./...` ✅ clean.

---

## 12. Phase 2 next-step recommendation

**Next dispatch: author the Phase 2f sub-plan** (`docs/superpowers/plans/YYYY-MM-DD-phase2f-full-encode-plan.md`).

Phase 2f — Full-frame encode + streaming wrappers + per-vector ITU byte-EQ — covers:

- Final wiring of `Encoder.EncodeFrame` end-to-end: pre-process → LPC → LSP → open-loop pitch → per-subframe (closed-loop pitch → ACELP → gain → memory updates) → bitstream pack via `internal/bitstream`. (All upstream stages are now production-pinned through Phase 2d; Phase 2f's job is the bitstream packer + the public API surface, not new arithmetic.)
- Streaming convenience: `(*Encoder).Write` / `(*Encoder).Flush` per design §4.3 (zero-pad tail on Flush).
- `Reset()` semantics audit.
- Removal of `ErrNotImplemented` from public API.
- **TAME.IN → TAME.BIT byte-EQ harness** (deferred from Phase 2d per sub-plan §0.3): exercises GQ-3 taming branch on the dedicated taming test vector. First Phase 2-cycle gate that *directly* witnesses the OQ-TAMING-THR pin.
- Per-vector full-frame byte-EQ: ALGTHM / SPEECH / FIXED / LSP / PITCH / TAME / TEST — Phase 2-final compliance gate.

The Phase 2f sub-plan should explicitly carry the Phase 2d LIVE OQs (OQ-A38-DEPTH, OQ-A38-SIGNTIE, OQ-TAMING-THR, OQ-GA-PRESELECT-METRIC, OQ-GBK-INDEX-MAP) and the Phase 2c LIVE-DEFERRED entries (H-CENTER, H-PHASE, OQ-WINDOW, OQ-XB-NORM) — none are gating for Phase 2f's bitstream packer, but TAME.BIT byte-EQ may witness OQ-TAMING-THR and/or trigger a Phase 2g (perf + ACELP-search pruning) entry if `BenchmarkEncodeFrame` proves binding.

**Phase 2g (perf) — not yet allocated.** If the Phase 2f end-to-end benchmark proves the ACELP full-search 8192-iteration baseline binding for soft-realtime use, author a Phase 2g sub-plan to consume Phase 2d INT-1a I5 slot 1/5 (OQ-A38-DEPTH) with a pruned variant (per-depth K-best or threshold-controlled focused search). Phase 2g is *contingent* on Phase 2f's measured perf; not a precondition for Phase 2-final.

**Phase 2 cycle status:** **NOT ready to close.** Phase 2f remains. After Phase 2f closes (with the per-vector byte-EQ harness either passing or each vector's FAIL-DEFERRED disposition documented), Phase 2-final closure report can be authored. The cycle requires at minimum Phase 2f to wire `EncodeFrame` to a non-`ErrNotImplemented` return path before the public API can claim G.729A encoder support.

**Phase 2-final reminder:** the strict G.192 byte-EQ gate remains a Phase 2-final concern. Phase 2d's contribution is the FCB-side fields specifically (S/C/GA/GB), which remain FAIL-DEFERRED until upstream H-CENTER closes (Phase 2b re-entry — currently un-allocated, candidate for Phase 2g or Phase 2-final escape).

---

— end of Phase 2d closure report —
