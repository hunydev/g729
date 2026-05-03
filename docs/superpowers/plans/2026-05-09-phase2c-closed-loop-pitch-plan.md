# Phase 2c — Closed-loop pitch + adaptive codebook (sub-plan)

- **Date:** 2026-05-09
- **Status:** IN PROGRESS
- **Scope:** ITU §A.3.5 (impulse response), §A.3.6 (target signal), §A.3.7 (closed-loop search incl. fractional refinement via b30, eq. A.6–A.8), §A.3.8 cross-cited for fixed-codebook handoff only; base §3.7 (and §3.7.1/§3.7.2/§3.7.3) for P1/P0/P2 packing and Gp computation; §4.1.3 P1/P2 decode mechanic for byte-EQ.
- **Inputs:**
  - Master plan: `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §4
  - Predecessor plan (template): `docs/superpowers/plans/2026-05-07-phase2b-open-loop-pitch-plan.md`
  - Predecessor closure: `docs/superpowers/plans/2026-05-08-phase2b-closure-report.md`
  - Spec: `docs/superpowers/specs/itu/G729E.txt` §A.3.5–§A.3.8, §3.6/§3.7, §4.1.3, Table 8
- **Output contract:** ledger-driven TDD; one INT-1 STRICT byte-EQ gate against PITCH.BIT P1/P0/P2 fields; closure report as INT-3.

---

## 0. Inherited invariants

| ID | Invariant | Source |
|---|---|---|
| I1 | Q-format discipline: Q15 LSP/coeffs, Q12 LP coefficients (`aQ12Latest[0..10]`), Q14 weighted residual where defined; reuse phase-2a/2b types. | Phase 2a closure §I1, phase 2b §0 |
| I3 | Per-frame state mutation only at frame end; closed-loop runs per subframe but commits encoder state once per frame. | Phase 2b §0 I3 |
| I4 | Zero-alloc on the hot path (encode loop). All scratch in `[N]int16`/`[N]int32` arrays, no `make` inside subframe loop. | Phase 2b INT-2 |
| I5 | INT-1 spend budget: ≤5 escalations per integration step before mandatory ACCEPT-PARTIAL writeup. | Phase 2a INT-1 closure |
| I6 | ITU bit-exactness for all integer ops; saturating fixed-point arithmetic via `internal/fixed`. | Master plan §I6 |
| I8 | Single squashed commit per task with prescribed message + `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer. | Master plan §I8 |
| **I-2c-1** | **Annex A binding:** when §A.3.* differs from §3.* (e.g. R(k) numerator-only RN(k) per eq. A.7; b30 1/3 frac per eq. A.8), Annex A wins. Base §3.7 is consulted only for P1/P0/P2 packing (§3.7.2) and Gp formula (§3.7.3). | ITU §A.3.7 vs §3.7 |
| **I-2c-2** | **Quantized-Â discipline:** all closed-loop filters MUST run on quantized Â(z) reconstructed from quantized LSP via `internal/lsp.LSPToLP` (exported by QA-1). No drift to unquantized A(z). | Phase 2b H-OQ2 LIVE-DEFERRED → resolved here |

---

## 1. Spec anchors

| Section | Lines | Subject | Eqs |
|---|---|---|---|
A.3.5 | 2115–2118 | Impulse response h(n) of weighted synthesis 1/Â(z/γ₁)·(... already weighted in Annex A: just 1/Â(z/γ)) over subframe | — || 
| §A.3.6 | 2119–2125 | Target signal x(n): residual r(n) filtered through 1/Â(z/γ); Annex A simplification vs base §3.6 | — |
| §A.3.7 | 2126–2175 | Closed-loop pitch search; numerator-only criterion RN(k); 1/3-sample b30 fractional refinement | A.6, A.7, A.8 |
| §A.3.7.1/2/3 | 2176–2181 | Cross-references to §3.7.1/2/3 (b30 interp / P1-P0-P2 encoding / Gp) | — |
A.3.8 | 2182–2190 | **Fixed codebook (out of scope here; handed to Phase 2d)** — cited only because user-task numbering colloquially groups frac interp under "§A.3.8" | — || 
| §3.6 | 1058–1078 | Base target signal definition | — |
| §3.7 | 1079–1157 | Base closed-loop search; tmin/tmax search window; eq. 38 yk recursion; eq. 39 b12 interpolation (NOT used in Annex A) | 37, 38, 39 |
| §3.7.1 | 1158–1167 | b30 1/3 fractional interpolation table — referenced by §A.3.7 eq. A.8 | 40 |
| §3.7.2 | 1168–1185 | P1 (8 bits, fractional 1/3 in [19⅓..84⅔] then int [85..143]), P0 parity (1 bit), P2 (5 bits relative) | 41, 42 |
| §3.7.3 | 1186–1199 | Adaptive-codebook gain Gp eq. 43 bounded [0, 1.2]; filtered adaptive vector y(n) eq. 44 | 43, 44 |
| §4.1.3 | 1500–1525 | **Decoder mapping** P1→T1, P2→T2 (used here as the canonical inverse for byte-EQ INT-1) | — |
| Table 8 | 1446–1464 | Bit allocation: P1=8, P0=1, P2=5 per subframe | — |

**Spec quirk OQ-INTERP:** the user task labels fractional interpolation as "§A.3.8"; the spec actually places it in §A.3.7 eq. A.8 + §3.7.1 eq. 40. §A.3.8 is fixed codebook. Plan uses spec numbering throughout.

---

## 2. Test-vector inventory

| Vector | Path | Use |
|---|---|---|
| PITCH.IN | `testdata/itu/PITCH.IN` | Encoder input PCM (16 kHz 16-bit). Drives closed-loop search per subframe. |
| PITCH.BIT | `testdata/itu/PITCH.BIT` | ITU reference bitstream. **INT-1 byte-EQ:** decode P1/P0/P2 fields per §4.1.3 lines 1505–1510 (P1) and 1512–1523 (P2) and compare to encoder output. STRICT this iteration (no ACCEPT-PARTIAL fallback at first attempt; budget I5 ≤ 5 escalations). |
| LSP.IN/LSP.BIT | (inherited from Phase 2a) | QA-1 cross-check: reconstructed Â must match decode-side LP. |

P1 decode: if P1 < 197 → int(T1) = (P1+2)/3 + 19, frac = P1 − 3·int(T1) + 58; else int(T1) = P1 − 112, frac = 0. P2 decode: tmin = max(int(T1)−5, 20); if tmin+9 > 143 then tmin = 134; tmax = tmin+9; int(T2) = (P2+2)/3 − 1 + tmin, frac = P2 − 3·((P2+2)/3 − 1) − 2.

---

## 3. Pre-flight inventory

### 3.1 Working-tree gate

- Phase 2b CLOSED per `2026-05-08-phase2b-closure-report.md` (INT-1 ACCEPT-PARTIAL @ 53.95% plausibility, H-OQ2 + H-PHASE LIVE-DEFERRED into 2c).
- `git status` MUST be clean before QA-1 starts; baseline `go test ./...` count recorded in QA-1 step-1.
- `go vet ./...` MUST pass clean as gate.

### 3.2 Reusable symbols

| Symbol | Location | Phase 2c use |
|---|---|---|
| `lspToLP` (→ export as `LSPToLP`) | `internal/lsp/lsp_lp.go:21` (currently package-private) | QA-1 exports it; HI-1/TG-1 consume to build h(n) and x(n) from quantized LSP. |
| `internal/fixed` (mult/mac/round) | `internal/fixed/` | All closed-loop accumulators. |
| `internal/pitch/openloop` | `internal/pitch/openloop/` | Sibling precedent for new `closedloop/` package. |
| `internal/pitch/adaptive.go` | `internal/pitch/` (decoder side) | Reference for adaptive vector v(n) construction; encoder VP-1 mirrors it. |
| `internal/pitch/parity.go` | `internal/pitch/` | P0 parity computation reused as-is for ENC-1. |
| `encoder.aQ12Latest[11]` | `encoder.go:56` | Quantized LP from Phase 2a; QA-1 ensures it is the LSPToLP output. |
| `encoder.lpResidualMem[10]`, `swMem[10]` | `encoder.go:57–58` | Filter memories for h(n) and x(n). |
| `encoder.tOp` | `encoder.go:59` | Phase 2b open-loop center; CL-1 search window center for subframe 1. |
| `encoder.oldExc[154]int16` | `encoder.go:25` | Past excitation buffer; VP-1 reads, GP-1 writes. Currently zero-initialized untouched by Phase 2b. |
| `encoder.lspOldQ[10]` | `encoder.go:30` | Quantized LSP feed for QA-1. |

### 3.3 New symbols (to be added by Phase 2c)

| Symbol | Owner task | Signature |
|---|---|---|
| `lsp.LSPToLP` | QA-1 | Exported wrapper around current package-private `lspToLP`. |
| `closedloop.ImpulseResponse(aQ12 [11]int16, gamma int16, h *[40]int16)` | HI-1 | Annex A §A.3.5 |
| `closedloop.TargetSignal(aQ12 [11]int16, residual []int16, swMem *[10]int16, x *[40]int16)` | TG-1 | Annex A §A.3.6 |
| `closedloop.SearchInteger(xb [40]int16, exc []int16, center int16, sub int) (intLag int16, RNbest int32)` | CL-1 | §A.3.7 eq. A.6/A.7 |
| `closedloop.Subframe2Window(intT1 int16) (tmin, tmax int16)` | CL-2 | §4.1.3 lines 1512–1523 |
| `closedloop.RefineFraction(xb [40]int16, exc []int16, intLag int16) (frac int16)` | FR-1, FR-2 | §A.3.7 eq. A.8, b30 |
| `closedloop.AdaptiveVector(exc []int16, intLag, frac int16, v *[40]int16)` | VP-1 | §3.7.1 eq. 40 |
| `closedloop.GpAndY(x [40]int16, y0 [40]int16, h [40]int16, v [40]int16) (gp int16, y *[40]int16)` | GP-1 | §3.7.3 eq. 43, 44 |
| `closedloop.PackP1P0P2(intT1, frac1, intT2, frac2 int16) (p1 uint16, p0 uint16, p2 uint16)` | ENC-1 | §3.7.2 eq. 41/42, parity |
| `encoder.closedloopStep(sub int)` | INT-0 | Per-subframe driver invoked twice per frame after `lpcStep`/`openloopStep`. |

---

## 4. Package-layout decision

**Choice:** new package `internal/pitch/closedloop/`, sibling to existing `internal/pitch/openloop/`.

**Justification:**
- Mirrors Phase 2b precedent (kept open-loop in its own subpackage).
- Keeps closed-loop logic narrowly scoped; no entanglement with decoder-side `internal/pitch/{adaptive,delay,parity}.go`.
- Permits zero-alloc-only API surface (caller-owned scratch).

**Alternatives rejected:**
- *Place inside `internal/pitch/` directly* — collides with decoder symbols; harder to audit zero-alloc.
- *Place inside `encoder` package* — leaks Annex A pitch search into top-level; can't reuse from any future Annex E variants.
- *Reuse `internal/pitch/openloop/` (rename it)* — open-loop is a distinct algorithmic stage; conflation hurts readability.

---

## 5. Task ledger

> Each task: 5-step TDD checklist (RED → GREEN → refactor → vet → commit). Each commit message stub MUST include `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` (I8).

### QA-1 — Export `LSPToLP` and verify quantized-Â reconstruction

- [ ] Step 1 — Baseline: `go test ./...` count + `git status` clean.
- [ ] Step 2 — RED: add `internal/lsp/lsp_lp_export_test.go` asserting `lsp.LSPToLP` symbol exists and matches package-private result for a fixed LSP vector from LSP.IN.
- [ ] Step 3 — GREEN: rename `lspToLP` → `LSPToLP` in `internal/lsp/lsp_lp.go:21`; update internal callers in `internal/lsp/decoder.go`.
- [ ] Step 4 — vet + race + bench unchanged; `go test ./internal/lsp/...` green.
- [ ] Step 5 — Commit `phase2c(lsp): export LSPToLP for encoder closed-loop reuse (QA-1)` with I8 trailer.

### HI-1 — Impulse response h(n) for 1/Â(z/γ) over subframe

- [ ] RED: `closedloop_test.go` with golden h(n) vector from a hand-driven trace of one subframe.
- [ ] GREEN: implement `ImpulseResponse` in `internal/pitch/closedloop/impulse.go` using `internal/fixed`.
- [ ] Refactor: extract gamma weighting helper if shared with TG-1.
- [ ] vet + zero-alloc check via `go test -bench`.
- [ ] Commit `phase2c(closedloop): impulse response h(n) per A.3.5 (HI-1)`.

### TG-1 — Target signal x(n)

- [ ] RED: golden x(n) test from PITCH.IN frame 0 subframe 0 using known Â and `swMem`.
- [ ] GREEN: implement `TargetSignal` in `internal/pitch/closedloop/target.go`.
- [ ] Refactor: ensure `swMem` is read-only inside (mutation deferred to encoder driver per I3).
- [ ] vet + race.
- [ ] Commit `phase2c(closedloop): target signal x(n) per A.3.6 (TG-1)`.

### CL-1 — Integer-lag closed-loop search RN(k) numerator-only

- [ ] RED: golden integer winner for PITCH.IN frame 0 subframe 0 (use ITU reference T1 derived from PITCH.BIT P1).
- [ ] GREEN: `SearchInteger` per eq. A.6/A.7. Backward-filtered target xb; iterate k over [center−3, center+3] ∩ [20,143].
- [ ] Refactor: caller-owned scratch only.
- [ ] vet + bench (zero-alloc).
- [ ] Commit `phase2c(closedloop): integer-lag closed-loop search per A.3.7 (CL-1)`.

### CL-2 — Subframe-2 search window per §4.1.3

- [ ] RED: window-bounds test asserting tmin/tmax slide rule per spec lines 1512–1523.
- [ ] GREEN: `Subframe2Window`.
- [ ] Refactor: shared with ENC-1 P2 packing.
- [ ] vet.
- [ ] Commit `phase2c(closedloop): subframe-2 search window (CL-2)`.

### FR-1 — b30 fractional interpolation eq. A.8

- [ ] RED: golden fractional value (−1/0/+1 of 1/3) for one subframe.
- [ ] GREEN: import b30 table from existing decoder-side `internal/pitch/` if usable; else replicate in `internal/pitch/closedloop/frac.go`.
- [ ] Refactor: dedupe with decoder b30 table behind shared `internal/pitch/b30` if no import cycle.
- [ ] vet.
- [ ] Commit `phase2c(closedloop): b30 1/3 fractional interpolation (FR-1)`.

### FR-2 — Fractional refinement around integer winner

- [ ] RED: combined integer+frac picks T1 frac matching PITCH.BIT P1 decode.
- [ ] GREEN: `RefineFraction` evaluates RN(k) at frac ∈ {−1,0,+1}/3 around integer winner; returns frac that maximizes.
- [ ] Refactor: collapse with CL-1 if natural.
- [ ] vet.
- [ ] Commit `phase2c(closedloop): fractional refinement around integer winner (FR-2)`.

### VP-1 — Adaptive codebook vector v(n) from oldExc

- [ ] RED: golden v(n) for known (intLag, frac) using `oldExc[154]`.
- [ ] GREEN: `AdaptiveVector` in `internal/pitch/closedloop/adaptive.go` mirroring decoder-side construction (b30 frac applied to past excitation).
- [ ] Refactor: zero-alloc.
- [ ] vet + race.
- [ ] Commit `phase2c(closedloop): adaptive codebook vector v(n) (VP-1)`.

### GP-1 — Gp + y(n) per eq. 43/44

- [ ] RED: golden Gp clamped to [0, 1.2] (Q14) and y(n) update.
- [ ] GREEN: `GpAndY`.
- [ ] Refactor: re-use convolution helper from HI-1 if applicable.
- [ ] vet.
- [ ] Commit `phase2c(closedloop): adaptive-codebook gain Gp and filtered y(n) (GP-1)`.

### ENC-1 — P1/P0/P2 bit packing per §3.7.2 + Table 8

- [ ] RED: encode (intT1, frac1, intT2, frac2) → P1/P0/P2 matching PITCH.BIT for frame 0.
- [ ] GREEN: `PackP1P0P2`; reuse `internal/pitch/parity.go` for P0.
- [ ] Refactor: invariant cross-check via existing decoder-side P1→T1 round-trip.
- [ ] vet.
- [ ] Commit `phase2c(closedloop): pack P1/P0/P2 per Table 8 (ENC-1)`.

### INT-0 — Encoder wiring: `closedloopStep` ×2 per frame

- [ ] RED: `encoder_test.go` invoking encoder over PITCH.IN frame 0 expects `oldExc` and bitstream P1/P0/P2 fields populated for both subframes.
- [ ] GREEN: add `closedloopStep(sub int)` to `encoder.go`, called twice per frame after `openloopStep`. Wire QA-1 → HI-1 → TG-1 → CL-1 → FR-2 → VP-1 → GP-1 → ENC-1; commit `oldExc`/`swMem` updates only at frame end (I3).
- [ ] Refactor: scratch arrays as method receivers' fields if profiling shows alloc.
- [ ] vet + race.
- [ ] Commit `phase2c(encoder): wire closed-loop pitch per subframe (INT-0)`.

### INT-1 — STRICT byte-EQ vs PITCH.BIT P1/P0/P2

- [ ] RED: new `phase2c_int1_pitch_byteeq_test.go` decoding PITCH.BIT P1/P0/P2 per §4.1.3 (lines 1505–1510 / 1512–1523) and asserting STRICT equality with encoder output across all frames.
- [ ] Iterate up to I5 budget (5 escalations max) hunting mismatches. Track each escalation in `docs/superpowers/plans/2026-05-09-phase2c-int1-escalations.md`.
- [ ] On mismatch escalation chain: H-OQ2 (already resolved by QA-1) → H-PHASE (filter memory phasing across frame boundary; inherited from Phase 2b §H-PHASE) → H-CENTER (open-loop center off-by-one) → H-FRAC-TIE (tie-break direction in eq. A.7) → H-TMIN-EDGE.
- [ ] If still red after I5: ACCEPT-PARTIAL writeup with plausibility computation (per Phase 2a INT-1 closure template) and LIVE-DEFERRED hypotheses listed.
- [ ] Commit `phase2c(int1): closed-loop pitch byte-EQ vs PITCH.BIT (INT-1)`.

### INT-2 — Zero-alloc + race + bench

- [ ] RED: `phase2c_int2_zeroalloc_test.go` asserts `testing.AllocsPerRun` == 0 over `closedloopStep`.
- [ ] GREEN: convert any captured allocs to caller-owned scratch.
- [ ] `go test -race ./...` green; bench captured.
- [ ] Commit `phase2c(closedloop): zero-alloc + race-clean closed-loop step (INT-2)`.

### INT-3 — Closure report

- [ ] Write `docs/superpowers/plans/2026-05-10-phase2c-closure-report.md` mirroring 2b closure report sections (overview, INT-1 disposition, plausibility math if ACCEPT-PARTIAL, LIVE-DEFERRED list, Phase 2d entry preconditions).
- [ ] Update master plan §4 row to CLOSED with closure-report link.
- [ ] Commit `phase2c: closure report + master-plan flip (INT-3)`.

---

## 6. Per-task contract summary

| Task | Inputs | Outputs | Spec | Test |
|---|---|---|---|---|
| QA-1 | quantized LSP | `[11]int16` Â (Q12) | §3.2.6 | unit, golden vs decoder |
| HI-1 | Â, γ | `h[40] int16` | §A.3.5 | unit golden |
| TG-1 | Â, residual, swMem | `x[40] int16` | §A.3.6 | unit golden |
| CL-1 | xb, exc, center, sub | int lag, RN | §A.3.7 eq. A.6/A.7 | unit golden vs PITCH.BIT |
| CL-2 | int T1 | tmin/tmax | §4.1.3 1512–1523 | bounds unit |
| FR-1 | exc, intLag, frac | b30 sample | §3.7.1 eq. 40 | golden table |
| FR-2 | xb, exc, intLag | frac ∈ {−1,0,+1} | §A.3.7 eq. A.8 | unit |
| VP-1 | exc, intLag, frac | v[40] | §3.7.1 | golden |
| GP-1 | x, y0, h, v | Gp, y[40] | §3.7.3 eq. 43/44 | golden |
| ENC-1 | intT1, frac1, intT2, frac2 | P1, P0, P2 | §3.7.2, Table 8 | round-trip vs §4.1.3 |
| INT-0 | encoder | oldExc/swMem mutation | §A.3.* | encoder smoke |
| INT-1 | PITCH.IN | PITCH.BIT P1/P0/P2 byte-EQ | §4.1.3 | corpus |
| INT-2 | encoder | 0 allocs / race-clean | I4 | bench |
| INT-3 | results | closure report | — | doc |

---

## 7. Output contract for INT gates

### INT-1 STRICT report table (template)

| Frame | Sub | P1.ref | P1.enc | P0.ref | P0.enc | P2.ref | P2.enc | match |
|---|---|---|---|---|---|---|---|---|

If any row mismatches and I5 budget exhausted, switch to ACCEPT-PARTIAL:

| Field | Total | Match | Mismatch | %match | Verdict |
|---|---|---|---|---|---|

Plausibility = weighted avg over (P1 byte-EQ, P0 parity match, P2 byte-EQ); ACCEPT-PARTIAL threshold ≥ 50%.

### INT-2 zero-alloc report

| Run | AllocsPerRun | BytesPerRun | Verdict |
|---|---|---|---|

---

## 8. I5 budget tracker (INT-1)

| Escalation | Hypothesis | Spent | Remaining | Result |
|---|---|---|---|---|
| 0 | (initial pass) | 0/5 | 5/5 | TBD |

---

## 9. Open questions / risks

- **OQ-PHASE (inherited from Phase 2b H-PHASE):** filter-memory phasing across frame boundary may still misalign when closed-loop runs subframe-2 with subframe-1 GP-1 updates not yet committed (I3). Mitigation: hold subframe-1 `swMem`/`oldExc` deltas in scratch and apply at frame end.
- **OQ-INTERP:** user task labels frac interp as "§A.3.8" but spec places it in §A.3.7 eq. A.8 + §3.7.1 eq. 40. Plan uses spec numbering; flagged for code review.
- **OQ-GBOUND:** Gp upper bound 1.2 in eq. 43 — is this Q14 = 19661 or Q12 = 4915? §3.7.3 does not specify Q-format inline; verify against ITU C reference `qua_gain.c`/`pitch.c` constants in QA step of GP-1.
- **OQ-2 (inherited Phase 2a) RESOLVED via QA-1** export of LSPToLP.
- **Risk R-1:** Annex A search range `[19⅓, 84⅔] ∪ [85,143]` boundary 85-vs-86 differs from base §3.7.2 eq. 41 boundary 86. CL-1/FR-2 must follow Annex A boundary.
- **Risk R-2:** P0 parity convention — verify `internal/pitch/parity.go` matches encoder direction (decoder uses inverse).

---

## 10. Inheritance to Phase 2d (ACELP / fixed codebook)

Phase 2c MUST hand off:
- Â(z) per subframe (already in `aQ12Latest`).
- Target signal x(n) (TG-1) — Phase 2d subtracts adaptive-codebook contribution Gp·y(n) to form x2(n) for ACELP.
- Filtered adaptive vector y(n) (GP-1) and Gp.
- Updated `oldExc[154]` after each subframe (committed at frame end).
- h(n) (HI-1) is reused by ACELP backward correlation.

---

## 11. Self-review

- [x] Mirrors Phase 2b sub-plan structure (§§0–12).
- [x] All ITU spec references cite line ranges in `docs/superpowers/specs/itu/G729E.txt`.
- [x] §A.3.8 numbering quirk explicitly flagged (OQ-INTERP).
- [x] LIVE-DEFERRED hypotheses from Phase 2b (H-OQ2, H-PHASE) addressed: H-OQ2 resolved by QA-1, H-PHASE remains a Phase 2c risk in INT-1 escalation chain.
- [x] Annex-A-vs-base distinction (RN(k) numerator-only; b30 not b12) called out as I-2c-1.
- [x] Quantized-Â discipline (I-2c-2) prevents silent drift.
- [x] Each task has TDD 5-step checklist + commit message stub + I8 trailer mandate.
- [x] INT-1 STRICT-first with documented ACCEPT-PARTIAL fallback under I5 budget.

---

## 12. Execution handoff

**Next dispatch:** QA-1 (export `LSPToLP`).

**Order of execution:** QA-1 → HI-1 → TG-1 → CL-1 → CL-2 → FR-1 → FR-2 → VP-1 → GP-1 → ENC-1 → INT-0 → INT-1 → INT-2 → INT-3.

**Stop conditions:**
- INT-1 PASS or ACCEPT-PARTIAL with I5 = 5/5 spent → proceed to INT-2.
- Any earlier task RED after a TDD round → escalate per ledger; do not skip.
- Any new prerequisite from Annex A discovered mid-flight → amend this plan with a new revision header before continuing.
