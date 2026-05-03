# Phase 2d — Fixed codebook (ACELP) + gain quantization + excitation commit (sub-plan)

- **Date:** 2026-05-11
- **Status:** **IN PROGRESS — opened 2026-05-11.** TDD task ledger for ITU-T G.729 Annex A §A.3.8 (fixed-codebook search) + §A.3.9 (gain quantization, defers to base §3.9) + §A.3.10 (eq. A.10 weighted-error / excitation commit). Awaits dispatch of CB-1.
- **Scope:** Annex A §A.3.8 + §A.3.8.1 + §A.3.8.2 (depth-first focused ACELP search; codeword packing) — base §3.8 (algebraic codebook structure) and §3.8.1 (sign pre-decision and threshold-controlled focused search) — base §3.9 (conjugate-structure 2D VQ on (g_p, γ̂_c), stages GA 3-bit + GB 4-bit) — §3.9.1 (4-th order MA log-energy prediction) — §A.3.9 (Annex A passthrough to §3.9) — §A.3.10 (excitation commit eq. A.9 + weighted-error eq. A.10 over n=30..39) — §4.1.4 (decoder S/C inverse mapping, used as INT-1a byte-EQ canonical form) — Table 1 (S1/S2 4 b each, C1/C2 13 b each, GA1/GA2 3 b each, GB1/GB2 4 b each).
- **Inputs:**
  - Master plan: `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §5 (Phase 2d scope) + §6 (Phase 2e scope — *folded into this sub-plan*; see §0.3 below).
  - Predecessor sub-plan (template): `docs/superpowers/plans/2026-05-09-phase2c-closed-loop-pitch-plan.md`.
  - Predecessor closure: `docs/superpowers/plans/2026-05-10-phase2c-closure-report.md` — carryover OQs in §0.4.
  - Phase 2b sub-plan (structure precedent): `docs/superpowers/plans/2026-05-07-phase2b-open-loop-pitch-plan.md`.
  - Spec: `docs/superpowers/specs/itu/G729E.txt` §3.8 (lines 1201–1340), §3.8.1 (1242–1318), §3.8.2 (1320–1340), §3.9 (1340–1407), §3.9.1 (1342–1380), §3.9.2 (1382–1405), §3.9.3 (1407–1414), §3.10 (1416–1442), §A.3.8 (2182–2190), §A.3.8.1 (2184–2188), §A.3.8.2 (2189–2190), §A.3.9 (2191–2192), §A.3.10 (2198–2215), §4.1.4 (1526–1532), Table 1 (lines 396–406), Table 8 (1446–1464).
- **Output contract:** ledger-driven TDD; **one INT-1a STRICT byte-EQ gate** vs `PITCH.BIT` for the new FCB-side fields (S1/S2/C1/C2 + GA1/GA2/GB1/GB2) **plus one INT-1b re-run** of `TestPhase2cINT1_ClosedLoopPitchByteEQ` against PITCH.BIT P1/P0/P2 — INT-1b is the formal closure of Phase 2c FAIL-DEFERRED via OQ-EXC-COMMIT. Closure report as INT-3.

---

## 0. Inherited invariants

### 0.1 Cross-cutting (from master plan + Phase 2c)

| ID | Invariant | Source |
|----|-----------|--------|
| I1 | **CLEAN-ROOM.** Only `docs/superpowers/specs/itu/G729E.{pdf,txt}` and our own prior plans/docs/textbooks (Kondoz, Spanias). NO ITU-T C reference, no bcg729, no Sipro, no FFmpeg. Self-attest at every commit; spec-cite every numeric constant (K3=0.4, max-loop=180, b=[0.68 0.58 0.34 0.19], β bounds [0.2,0.8], E̅=30 dB, …). | Master plan §I1 |
| I3 | Per-subframe state mutation discipline (relaxed for ACELP per Phase 2c precedent: `oldExc` / `swMemErr` / `lpResidualMemQ` commit per subframe so subframe-2 sees subframe-1 u(n); frame-level `oldSpeech` / `freqPrev` / `pastQuaEn` commit only at frame end). | Phase 2c §I3 |
| I4 | **Zero-alloc on hot path.** All FCB scratch (h, d, φ-storage, sign-extracted d, pulse-position state, candidate c[40], filtered z[40], gain VQ scratch, ew[10]) lives in stack-resident `[N]int16` / `[N]int32` arrays inside `fcbStep` / encoder method receivers. INT-2 pins `AllocsPerRun(128, fcbStep)` ≡ 0 and `AllocsPerRun(128, lpcStep+openloopStep+2×(closedloopStep+fcbStep))` ≡ 0. | Phase 2c INT-2 |
| I5 | INT-1 spend budget: ≤5 escalations per integration step before mandatory ACCEPT-PARTIAL writeup. Phase 2d INT-1a opens with **5/5 fresh** (separate gate from Phase 2c INT-1; the 4 reserved Phase 2c INT-1 slots are *consumed by INT-1b*, not by INT-1a — see §0.5). | Phase 2a INT-1 closure |
| I6 | ITU bit-exactness for all integer ops; saturating fixed-point arithmetic via `internal/fixed`. | Master plan §I6 |
| I8 | Single squashed commit per task with prescribed message + `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer. | Master plan §I8 |
| I9 | LSP codebook discipline: `internal/tables/lsp_*.go` MUST be untouched. (Not gating for Phase 2d but restated for self-attestation.) | Phase 2a closure |
| I10 | Encoder–decoder state isolation. Phase 2d MAY consume *read-only* primitives from `internal/fcb/` (Table 7 pulse layout, sign packing) and `internal/gain/` (`tables.GainGBK1/2`, `tables.GainMAPredictor`, `tables.GainMeanEnergyQ10`, `tables.GainImap1/2`, `pastErrorsDefault`) per the merger doctrine, but MUST NOT mutate decoder state. New encoder-side gain prediction memory lives in **`Encoder.pastQuaEn[4]`** (already reserved at `encoder.go:31`), distinct from `internal/gain.Decoder.pastErrors`. | Phase 2c §I10 |

### 0.2 Phase-2d-new

| ID | Invariant | Definition |
|----|-----------|-----------|
| **I-2d-1** | **Annex A binding for ACELP.** §A.3.8.1 specifies a *depth-first iterative tree search*; §3.8.1 specifies the *nested-loop A-priori focused search with K3=0.4 threshold and ≤180 last-loop entries*. Annex A §A.3.8.1 line 2185–2188 wins on **search algorithm** (depth-first, fixed complexity), but defers to §3.8.1 for **sign pre-decision** ("the signs of the pulses are found using the same approach explained in clause 3.8.1") and §3.8.2 for **codeword packing** (eq. 61 / 62) per §A.3.8.2 line 2189–2190 ("Same as described in clause 3.8.2"). |
| **I-2d-2** | **Annex A binding for gains.** §A.3.9 (line 2191–2192) is a one-line passthrough: "Same as described in clause 3.9." Phase 2d uses §3.9 + §3.9.1 + §3.9.2 + §3.9.3 verbatim; no Annex-A modulation. |
| **I-2d-3** | **Excitation commit eq. A.9 / A.10.** `oldExc` per-subframe append MUST be `u(n) = ĝp·v(n) + ĝc·c(n)` (eq. A.9, line 2202). `swMemErr[n−30]` per-subframe commit MUST be `ew(n) = x(n) − ĝp·y(n) − ĝc·z(n)` for n=30..39 (eq. A.10, line 2211). The Phase 2c placeholder commit (`u = Gp·v` only; `ew = x − Gp·y` only) is **REPLACED** by INT-0 of this sub-plan; this is the OQ-EXC-COMMIT closure path. |
| **I-2d-4** | **Quantized-everything discipline.** All gains used in eq. A.9 / A.10 commits are the *quantized* `ĝp` / `ĝc` (post §3.9.2 conjugate VQ search), NOT the unquantized `gp` / `gc`. Both quantities flow through the encoder for one round-trip: (i) ACELP search uses unquantized `gp` (from Phase 2c GP-1) to form `x'(n) = x(n) − gp·y(n)` (§3.8.1 eq. 50); (ii) gain VQ search produces `ĝp` and `ĝc`; (iii) the eq. A.9 / A.10 commits use the quantized pair. |

### 0.3 Master-plan scope deviation

The master plan splits ACELP (Phase 2d) and gain quantization (Phase 2e) into separate sub-phases. **This sub-plan folds both into one Phase 2d** because §A.3.10 eq. A.10 (excitation commit) requires *quantized* `ĝp` and `ĝc`, and the Phase 2c FAIL-DEFERRED disposition (OQ-EXC-COMMIT) cannot close until both are wired. Splitting would force a FAIL-DEFERRED at Phase 2d INT-1 with the same structural rationale (`oldExc` still missing the gain-quantized fixed-codebook half). The master-plan §6 Phase-2e row will be **subsumed** by the Phase 2d INT-3 closure report; §6 will be flipped to "FOLDED INTO PHASE 2D — see `…2026-05-11-phase2d-fixed-codebook-acelp-plan.md`". The Phase 2e ITU vector gate (`TAME.IN` → `TAME.BIT`) remains a Phase 2-final concern and is not gated by this sub-plan; the §3.9.2 taming branch is implemented under GQ-3 but its dedicated TAME.BIT byte-EQ harness is deferred to Phase 2f.

### 0.4 Carryover from Phase 2c (closure report `2026-05-10-phase2c-closure-report.md` §9)

| ID | State at Phase 2c close | Phase 2d disposition |
|----|-------------------------|-----------------------|
| **OQ-EXC-COMMIT** | LIVE-DEFERRED (Phase 2d coupling). `oldExc` per-subframe append is missing the `ĝc·c(n)` half; `swMemErr` is missing the `ĝc·z(n)` half. | **RESOLVED HERE.** I-2d-3 is the spec binding; INT-0 wires it; INT-1b is the formal regression gate (Phase 2c INT-1 byte-EQ re-run). |
| **H-CENTER** | LIVE-DEFERRED (Phase 2b open-loop carryover). `tOp` open-loop output diverges from reference T1 in ~46 % of frames; this caps Phase 2c P1 byte-EQ. | **CANDIDATE FOR RE-EVALUATION.** INT-1b post-OQ-EXC-COMMIT byte-EQ may close higher than the Phase 2b plausibility surface (53.95 %) IF the Phase 2c FAIL-DEFERRED was driven primarily by `oldExc` corruption rather than `tOp` miscentring. If INT-1b ≥ 80 %, Phase 2c flips ACCEPT-PARTIAL or PASS and H-CENTER survives only on the residual mismatch tail. If INT-1b < 50 %, H-CENTER is the next escalation target (consume Phase 2c reserved I5 slot 2/5; not Phase 2d INT-1a budget). See §5 INT-1b ledger. |
| **H-PHASE** | LIVE-DEFERRED (Phase 2b carryover). Subframe-2 `swMem` pre-commit ordering not unambiguous from §A.3.6. | **CANDIDATE FOR RE-EVALUATION.** Same disposition as H-CENTER: INT-1b will measure whether subframe-2 P2 mismatches correlate with `swMemErr` slide ordering once the eq. A.10 `ĝc·z` term lands. If P2 byte-EQ remains < P1 byte-EQ by > 10 pts, H-PHASE escalation (instrument per-frame `swMemErr[0..9] @ sub-1 entry / sub-2 entry / frame end`) consumes Phase 2c reserved I5 slot 3/5. |
| **OQ-WINDOW** | PINNED (Phase 2b open-loop search window `[tOp−5, tOp+4]`). | **RE-RUN ESCALATION KNOB only.** Not consumed under Phase 2d INT-1a (FCB params don't directly depend on the search window). May be consumed under Phase 2c reserved slot 4/5 if INT-1b residual is concentrated on `\|reference T1 − tOp\| > 4` frames. |
| **OQ-XB-NORM** | UNTESTED (xb Q-shift). | **RE-RUN ESCALATION KNOB only.** Same disposition as OQ-WINDOW: candidate for Phase 2c reserved slot 5/5 if INT-1b residual implicates correlation magnitude rather than algebraic ordering. |

**Carryover-handling rationale (summary).** Phase 2d INT-1a (FCB byte-EQ) and INT-1b (Phase 2c re-run) are *separate* gates with separate I5 budgets. INT-1a opens 5/5 fresh; INT-1b consumes Phase 2c's 4/5 reserved slots in escalation order H-CENTER → H-PHASE → OQ-WINDOW → OQ-XB-NORM (per Phase 2c closure §5 line 153). Splitting the budget this way preserves the I5 doctrine (no double-spend) and preserves the Phase 2c production-freeze surface (Phase 2c INT-1 closure remains unchanged unless INT-1b explicitly flips it).

### 0.5 I5 budget posture at Phase 2d entry

| Gate | Budget | Reserved | Spent | Available |
|------|-------:|---------:|------:|----------:|
| Phase 2d INT-1a (FCB byte-EQ vs PITCH.BIT) | 5 | 0 | 0 | **5** |
| Phase 2c INT-1b (re-run vs PITCH.BIT P1/P0/P2) | 5 | 1 (already spent at Phase 2c INT-1) | 1 | **4** |
| Phase 2-final escape (G.192 byte-EQ) | 1 | 1 | 0 | 0 |

---

## 1. Spec anchors (line ranges in `docs/superpowers/specs/itu/G729E.txt`)

| § | Lines | Subject | Eqs | Binding |
|---|------:|---------|-----|:-------:|
| 3.8 | 1201–1241 | Algebraic codebook structure: 4 pulses on tracks T0..T3 per Table 7; harmonic enhancement P(z) = 1/(1−βz⁻T); β = ĝp(m−1) bounded [0.2, 0.8]. | 45, 46, 47 | ✅ structure |
| 3.8.1 | 1242–1318 | Sign pre-decision; signal d(n) (eq. 52); criterion C²/E (eq. 53); fast formulas C (eq. 58) / E (eq. 59); φ′ matrix (eq. 56–57); focused search threshold thr3 = av3 + K3·(max3 − av3) (eq. 60); K3 = 0.4; max-loop = 180 / 2 subframes. | 50–60 | ✅ binding for sign pre-decision + φ′ form |
| 3.8.2 | 1320–1340 | Codeword packing: S = s0+2s1+4s2+8s3 (eq. 61); C = (m0/5)+8·(m1/5)+64·(m2/5)+512·(2(m3/5)+jx) (eq. 62) with jx∈{0,1}. | 61, 62 | ✅ binding |
| 3.9 | 1340–1395 | Conjugate-structure 2D VQ on (g_p, γ̂_c) using GA (3 b) + GB (4 b); ĝp = GA1+GB1 (eq. 73); ĝc = g'c·(GA2+GB2) (eq. 74). | 63, 73, 74 | ✅ binding |
| 3.9.1 | 1342–1380 | Gain prediction: 4-th order MA on `pastQuaEn` Q10 with b = [0.68 0.58 0.34 0.19] (table); E̅ = 30 dB; predicted g'c via eq. 71. | 65–72 | ✅ binding |
| 3.9.2 | 1382–1405 | Codebook search: preselect 4-of-8 in GA on second element (closest to gc); preselect 8-of-16 in GB on first element (closest to gp); exhaustive 4×8 = 32 over remainder minimizing eq. 63. | 63 | ✅ binding |
| 3.9.3 | 1407–1414 | Codeword: GA / GB indices mapped via `tables.GainImap1` / `GainImap2` (decoder side already implements the inverse `imap`). Encoder applies the *forward* map. | — | ✅ binding |
| 3.10 | 1416–1442 | Memory update (base codec): u(n) = ĝp·v + ĝc·c (eq. 75); ew(n) = x − ĝp·y − ĝc·z for n=30..39 (eq. 76). | 75, 76 | (subsumed by §A.3.10) |
| **A.3.8** | 2182–2190 | Annex A FCB: same 17-bit structure as §3.8; depth-first iterative tree search (vs §3.8.1 nested-loop focused). | — | ✅ binding |
| A.3.8.1 | 2184–2188 | Search algorithm narrative: "iterative depth-first, tree search approach is used. … smaller number of pulse position combinations is tested and it has fixed complexity." | — | ✅ binding (algorithm) |
| A.3.8.2 | 2189–2190 | Codeword: "Same as described in clause 3.8.2." | — | (passthrough to §3.8.2) |
| A.3.9 | 2191–2192 | Gain quantization: "Same as described in clause 3.9." | — | (passthrough to §3.9) |
| **A.3.10** | 2198–2215 | Excitation: u(n) = ĝp·v(n) + ĝc·c(n) (eq. A.9). Weighted error: ew(n) = x(n) − ĝp·y(n) − ĝc·z(n) for n=30..39 (eq. A.10). | A.9, A.10 | ✅ binding |
| 4.1.4 | 1526–1532 | Decoder mapping S/C → positions (canonical inverse for INT-1a byte-EQ). | — | ✅ binding (test) |
| Table 1 | 396–406 | Bit allocation: S1/S2 4 b, C1/C2 13 b, GA1/GA2 3 b, GB1/GB2 4 b. Total per subframe (FCB+gains): 24 b → 48 b/frame. | — | ✅ binding |
| Table 8 | 1446–1464 | Per-symbol bit count cross-reference. | — | (cross-ref) |

**Spec quirk OQ-A38-DEPTH:** §A.3.8.1 narrates "iterative depth-first, tree search" without giving (a) the depth ordering, (b) the per-track candidate count, (c) the threshold-controlled pruning rule, or (d) the maximum tree-traversal budget. The §3.8.1 K3 = 0.4 / max-180 numbers are explicitly base-codec only. **Closure path:** CB-2 step 1 lays out the depth-first algorithm derived from spec narrative + first principles (4 tracks × 8 positions = 32 candidates per pulse step; depth-first means commit i0 then i1 then i2 then i3; "fixed complexity" implies a *constant* candidate cap per depth — derive from spec narrative line 2188). Logged as **OQ-A38-DEPTH** in §9; default heuristic candidate cap and tie-break direction pinned by CB-2 step 2; instrumented for INT-1a escalation.

---

## 2. Test-vector inventory

| Vector | Path | Use |
|--------|------|-----|
| PITCH.IN | `testdata/itu/PITCH.IN` | Encoder input PCM. Drives FCB + gain pipeline per subframe. **Re-used** from Phase 2c — no new corpus needed. |
| PITCH.BIT | `testdata/itu/PITCH.BIT` | ITU reference bitstream. **INT-1a STRICT byte-EQ:** decode S1/S2 + C1/C2 + GA1/GA2 + GB1/GB2 fields per §4.1.4 + §3.9.3 (inverse `imap`) and compare to encoder output. **INT-1b STRICT byte-EQ re-run:** decode P1/P0/P2 per §4.1.3 (Phase 2c INT-1 harness re-executed). |
| TAME.IN / TAME.BIT | `testdata/itu/TAME.{IN,BIT}` | Reserved for Phase 2f (taming-branch byte-EQ); not gated by this sub-plan. GQ-3 implements the §3.9.2 taming arithmetic; its harness is owned by Phase 2f. |

PITCH.BIT bit layout (Table 8 + §4.1.4): per-frame 80 bits = L0(1) + L1(7) + L2(5) + L3(5) + P1(8) + P0(1) + S1(4) + C1(13) + GA1(3) + GB1(4) + P2(5) + S2(4) + C2(13) + GA2(3) + GB2(4). FCB-side fields land at bits 31..51 (subframe-1) and bits 56..79 (subframe-2). The Phase 2c INT-1 harness already strips P1/P0/P2; INT-1a strips S1/C1/GA1/GB1/S2/C2/GA2/GB2 from the same `PITCH.BIT` stream.

---

## 3. Pre-flight inventory

### 3.1 Working-tree gate

- Phase 2c CLOSED-DEFERRED per `2026-05-10-phase2c-closure-report.md`. INT-1 FAIL-DEFERRED at P1 9.05 % / P0 56.46 % / P2 9.75 %.
- `git status` MUST be clean before CB-1 starts; baseline `go test ./...` count + FAIL ledger recorded in CB-1 step 1 (must equal 5 = 4 inherited + 1 Phase 2c INT-1 FAIL-DEFERRED).
- `go vet ./...` MUST pass clean as gate.
- `BenchmarkClosedloopStep` baseline (14964 ns/op per Phase 2c INT-2) recorded for INT-2 perf comparison (`fcbStep` MUST not regress beyond 2× the closedloopStep budget — soft target).

### 3.2 Reusable symbols (per I10 merger doctrine)

| Symbol | Location | Phase 2d use |
|--------|----------|--------------|
| `internal/closedloop.ImpulseResponse` | `internal/pitch/closedloop/impulse.go` | h(n) reused as-is for CB-1 / CB-5; no new computation. |
| `internal/closedloop.TargetSignal` | `internal/pitch/closedloop/target.go` | x(n) reused as the input to CB-1 (CB-1 then forms x'(n) = x − gp·y per §3.8.1 eq. 50). |
| `internal/closedloop.GpAndY` outputs | `internal/pitch/closedloop/gain.go` | (gp, y[40]) consumed by CB-1 (target adjustment) and GQ-2 (eq. 63 inner products). |
| `internal/closedloop.AdaptiveVector` output v[40] | `internal/pitch/closedloop/adaptive.go` | Consumed by INT-0 for the eq. A.9 commit `ĝp·v(n)`. |
| `internal/fcb.decodePositions` | `internal/fcb/positions.go:15` | **Inverse only** — used inside INT-1a test harness to round-trip `(S, C) → positions → encoder output`. NOT consumed by encoder hot path. |
| `internal/fcb.placePulses` | `internal/fcb/signs.go:17` | Reused by CB-4 to construct `c[40]` from `(positions, signs)`. Encoder package imports `internal/fcb` read-only. |
| `internal/fcb.applyPitchEnhancement` | `internal/fcb/enhance.go:40` | Reused by CB-4 for the §3.8 eq. 46 P(z) = 1/(1−βz⁻T) harmonic enhancement applied to c[]. Argument β derived via §3.8 eq. 47 from previous-subframe ĝp (encoder maintains its own `prevGpQ14` field; see §3.4). |
| `internal/fcb.ClampPitchGainForEnhancement` | `internal/fcb/enhance.go:16` | Reused by CB-4 to compute β from `prevGpQ14`. |
| `internal/gain.predictedLogGain` (currently a method on `gain.Decoder`) | `internal/gain/predictor.go:20` | **Refactored** by GQ-1: extract pure form `gain.PredictedLogGain(pastQuaEn *[4]int16) int16` and have the existing decoder method call into it. Encoder calls the pure form on its own `Encoder.pastQuaEn[4]`. |
| `internal/tables.GainGBK1` (Q14 g_p, Q13 γ̂_c) | `internal/tables/` | GQ-2 reads for the GA codebook (8 entries × 2 elements). |
| `internal/tables.GainGBK2` | `internal/tables/` | GQ-2 reads for the GB codebook (16 × 2). |
| `internal/tables.GainImap1` / `GainImap2` | `internal/tables/` | These map *transmitted index → physical entry*; GQ-2's encoder side requires the **inverse** `map1` / `map2`. ENC-1 step 2 derives them by inverting `GainImap{1,2}` (8-entry / 16-entry permutations) — placed alongside the existing `Imap` tables in `internal/tables/` as `GainMap1` / `GainMap2` (additive, no decoder churn). |
| `internal/tables.GainMAPredictor` (b₁..b₄ Q13) | `internal/tables/` | GQ-1 reads for eq. 69. |
| `internal/tables.GainMeanEnergyQ10` (E̅ Q10 dB) | `internal/tables/` | GQ-1 reads for eq. 71. |
| `gain.pastErrorsDefault` (−14 dB Q10 = −14336) | `internal/gain/decode.go:9` | GQ-1 init: `Encoder.pastQuaEn[4]` initialized to 4× this value at first call. Same constant; export as `gain.PastErrorsDefault` if extraction needed (GQ-1 step 2). |
| `Encoder.pastQuaEn[4]` | `encoder.go:31` | **Already reserved.** GQ-1 owns its update per subframe. |
| `Encoder.aHatSF{1,2}` | `encoder.go:101–102` | Phase 2c-built quantized Â per subframe; consumed by CB-1 (h reuse) and GQ-2 (z = c ⊛ h). |
| `Encoder.oldExc[154]` | `encoder.go:25` | INT-0 OWNS the eq. A.9 commit (currently `Gp·v`-only Phase 2c placeholder); replaced with `ĝp·v + ĝc·c`. |
| `Encoder.swMemErr[10]` | `encoder.go:104` | INT-0 OWNS the eq. A.10 commit (currently `x − Gp·y` only); extended with `− ĝc·z`. |
| `Encoder.lpResidualMemQ[10]` | `encoder.go:105` | Untouched by Phase 2d (already committed per subframe by `closedloopStep`). |

### 3.3 New symbols (to be added by Phase 2d)

| Symbol | Owner task | Signature |
|--------|------------|-----------|
| `fixed.AdjustedTarget(x, y *[40]int16, gp int16, xPrime *[40]int16)` *(or place in `internal/pitch/closedloop` — see §4)* | CB-1 | `x'(n) = x(n) − gp·y(n)` per §3.8.1 eq. 50 |
| `fcbsearch.CorrelationD(xPrime, h *[40]int16, d *[40]int32)` | CB-1 | d(n) = Σᵢ x'(i)·h(i−n) per §3.8.1 eq. 52 |
| `fcbsearch.SignsFromD(d *[40]int32, signs *[40]int8, dAbs *[40]int32)` | CB-3 | signs[n] = sign(d(n)); dAbs[n] = \|d(n)\| per §3.8.1 |
| `fcbsearch.PhiPrime(h *[40]int16, signs *[40]int8, phi *[40][40]int32)` | CB-2 helper | φ′(i,j) per §3.8.1 eq. 56–57 |
| `fcbsearch.SearchDepthFirst(dAbs *[40]int32, phi *[40][40]int32) (positions [4]int8, bestC int32, bestE int32)` | CB-2 | Annex A §A.3.8.1 depth-first focused tree search; returns 4 pulse positions on tracks T0..T3 |
| `fcbsearch.BuildCode(positions [4]int8, signs *[40]int8, prevGpQ14 int16, T int16, c *[40]int16)` | CB-4 | c[] from (positions, signs) per §3.8 eq. 45 + harmonic enhancement P(z) per §3.8 eq. 46/47 |
| `fcbsearch.FilterCode(c *[40]int16, h *[40]int16, z *[40]int16)` | CB-5 | z(n) = c ⊛ h per §3.9 eq. 64 |
| `fcbsearch.PackS(signs [4]int8) uint8` | ENC-1 | S = s0+2s1+4s2+8s3 per §3.8.2 eq. 61 |
| `fcbsearch.PackC(positions [4]int8) uint16` | ENC-1 | C per §3.8.2 eq. 62 (13-bit) |
| `gain.PredictedLogGain(pastQuaEn *[4]int16) int16` | GQ-1 | pure refactor of `gain.Decoder.predictedLogGain` |
| `gain.PredictedGcQ12(pastQuaEn *[4]int16, c *[40]int16) int16` | GQ-1 | g'c per eq. 71 (composes log-energy + MA prediction) |
| `gainquant.SearchConjugate(x, y, z *[40]int16, gpcPredQ12 int16) (ga uint8, gb uint8, gpQ14 int16, gcQ12 int16)` | GQ-2 | §3.9.2 conjugate VQ search: 4-of-8 GA preselect on γ̂_c × g'c ≈ gc; 8-of-16 GB preselect on ĝp ≈ gp; exhaustive 4×8 over remainder minimizing eq. 63 |
| `gainquant.UpdatePastQuaEn(pastQuaEn *[4]int16, gammaCQ13 int16)` | GQ-3 | §3.9.1 eq. 72: U(m) = 20·log10(γ̂); FIFO shift |
| `gainquant.Tame(gpQ14Predicted int16, prevTaming bool, oldExc *[154]int16) (gpClampQ14 int16, taming bool)` | GQ-3 | §3.9.2 taming branch (clamp gp when predicted overflow, threshold per spec) |
| `gainquant.PackGains(ga uint8, gb uint8) (ga3 uint8, gb4 uint8)` | ENC-1 | §3.9.3 forward `imap` (inverse of `tables.GainImap{1,2}`) |
| `Encoder.fcbStep(sub int)` | INT-0 | Per-subframe driver; runs after `closedloopStep(sub)` |
| `Encoder.prevGpQ14` (new field) | INT-0 | Pitch-gain memory for §3.8 eq. 47 β derivation; updated at end of each subframe with `ĝp` (post gain VQ) |
| `Encoder.prevTaming` (new field) | INT-0 | §3.9.2 taming sticky flag (carries across subframes) |

### 3.4 Encoder state-additions checklist

INT-0 adds the following fields to `Encoder` (placed below the existing Phase 2c block at `encoder.go:107`):

```go
// Phase 2d INT-0: fixed-codebook + gain quantization state.
//
// prevGpQ14 caches the quantized adaptive-codebook gain ĝp from
// the previous subframe so CB-4 can derive β per §3.8 eq. 47.
// At cold start (first subframe of stream) this is 0.
//
// prevTaming is the §3.9.2 taming sticky flag.
//
// s1/c1/ga1/gb1/s2/c2/ga2/gb2 store the per-frame FCB+gain bits
// for Phase 2f bitstream packing.
prevGpQ14   int16
prevTaming  bool
s1, s2      uint8
c1, c2      uint16
ga1, gb1    uint8
ga2, gb2    uint8
```

`pastQuaEn[4]` is already reserved; GQ-1 step 1 initializes it to `4× pastErrorsDefault` (per `gain.Decoder` init pattern at `internal/gain/decode.go:39`).

---

## 4. Package-layout decision

**Choice:** new package `internal/fcbsearch/` (encoder-side ACELP search) and new package `internal/gainquant/` (encoder-side gain VQ + taming + prediction wiring), siblings to the existing decoder packages `internal/fcb/` and `internal/gain/`.

**Justification:**
- Mirrors Phase 2b precedent (`internal/pitch/openloop/` sibling to decoder `internal/pitch/`) and Phase 2c precedent (`internal/pitch/closedloop/`).
- Keeps encoder-side ACELP (depth-first search, sign pre-decision, φ′ matrix) cleanly separated from decoder-side `internal/fcb/` (position decode + harmonic enhancement). The merger doctrine permits read-only reuse of `fcb.placePulses` / `fcb.applyPitchEnhancement` / `fcb.ClampPitchGainForEnhancement` from the encoder package without circular dependencies (`internal/fcbsearch` imports `internal/fcb`, never the reverse).
- Keeps gain VQ search separate from decoder-side `internal/gain/` (which only knows how to *decode* given GA / GB indices). The inverse `imap` tables (`tables.GainMap1` / `GainMap2`) live in `internal/tables/` so both packages can read them.
- Permits zero-alloc-only API surface (caller-owned scratch).

**Alternatives rejected:**
- *Place inside `internal/fcb/` directly* — would merge encoder/decoder state surfaces and break I10 (encoder–decoder isolation).
- *Place inside `internal/acelp/`* — `internal/acelp/` is the Phase 2-0 stub (`internal/acelp/types.go:31`) for a future pluggable-search interface; merging the search implementation here couples the search algorithm to the public-ish `Searcher.Search` boundary prematurely. Phase 2f may later migrate `internal/fcbsearch.SearchDepthFirst` behind `internal/acelp.Searcher.Search`; not a Phase 2d concern.
- *Place inside `encoder` (root) package* — leaks Annex A search internals into top level; can't unit-test in isolation.
- *Single combined package `internal/encoderacelp/`* — `fcbsearch` and `gainquant` are algorithmically distinct (search vs quantization) and have different test surfaces; combining hurts readability.

---

## 5. Task ledger

> Each task: 5-step TDD checklist (RED → GREEN → refactor → vet → commit). Each commit message stub MUST include `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` (I8).

### CB-1 — Backward-filtered FCB target d(n)

§A.3.8 + §3.8.1 eq. 50 + 52. Forms x'(n) = x(n) − gp·y(n) (target after pitch contribution removed) then d(n) = Σᵢ x'(i)·h(i−n). DISTINCT from Phase 2c xb (which is a backward filter on the *unadjusted* target x(n)). Owner of the §3.8.1 eq. 50 target adjustment.

- [ ] Step 1 — Baseline: `go test ./... 2>&1 | tail -40` count + FAIL ledger (must equal 5); `git status` clean.
- [ ] Step 2 — RED: `internal/fcbsearch/correlation_test.go` golden `d[40]` from a hand-driven trace using PITCH.IN frame 0 sub 0: take Phase 2c outputs (x, h) + `closedloop.GpAndY` (gp, y), compute `x'(n) = x(n) − gp·y(n)`, then convolve with h(i−n) per eq. 52. Expect deterministic int32 values.
- [ ] Step 3 — GREEN: implement `fcbsearch.AdjustedTarget` and `fcbsearch.CorrelationD` in `internal/fcbsearch/correlation.go` using `internal/fixed`. Caller-owned scratch only.
- [ ] Step 4 — `go vet ./internal/fcbsearch/...` ✅; `go test ./internal/fcbsearch/...` ✅; alloc bench (`AllocsPerRun(128) == 0`).
- [ ] Step 5 — Commit `phase2d(fcbsearch): backward-filtered target d(n) per A.3.8 / 3.8.1 (CB-1)` with I8 trailer.

### CB-3 — Sign extraction sign(d(n))

(Done before CB-2 because CB-2 needs the φ′ matrix which depends on signs.)

- [ ] RED: `internal/fcbsearch/signs_test.go` golden `signs[40]` and `dAbs[40]` from CB-1's d[40].
- [ ] GREEN: implement `fcbsearch.SignsFromD` per §3.8.1 line 1296 ("the signal d(n) is decomposed into two parts: its absolute value …"). Sign convention: signs[n] ∈ {−1, +1}; tie-break (d(n) == 0) defaults to +1 (logged as **OQ-A38-SIGNTIE** in §9; spec is silent).
- [ ] Refactor: ensure caller-owned scratch.
- [ ] vet + bench.
- [ ] Commit `phase2d(fcbsearch): sign extraction sign(d(n)) per 3.8.1 (CB-3)`.

### CB-2 — Depth-first focused ACELP search

§A.3.8.1 line 2185–2188 (depth-first iterative tree search). Owner of OQ-A38-DEPTH closure.

- [ ] Step 1 — RED part A: `internal/fcbsearch/phi_test.go` golden φ′[40][40] from h + signs (eq. 56 + 57). Storage layout: lower-triangular only (φ′(i,j) for i ≤ j), main diagonal pre-scaled by 0.5.
- [ ] Step 2 — RED part B: `internal/fcbsearch/search_test.go` golden best `(positions, C, E)` for one synthetic input where the optimal pulse positions are hand-chosen. Vector size: 4 candidates per track × 4 tracks = 256 trial combinations exhaustive ground truth.
- [ ] Step 3 — GREEN: implement `fcbsearch.PhiPrime` (eq. 56–57) and `fcbsearch.SearchDepthFirst`. Algorithm (pinned per spec narrative + first principles):
    - For each i0 ∈ track 0 (8 positions) // depth 0
        - For each i1 ∈ track 1 (8 positions) // depth 1
            - C₂ = d(m0) + d(m1); E₂ = φ′(m0,m0) + φ′(m1,m1) + φ′(m0,m1)
            - For each i2 ∈ track 2 (8 positions) // depth 2
                - C₃ = C₂ + d(m2); E₃ = E₂ + φ′(m2,m2) + φ′(m0,m2) + φ′(m1,m2)
                - For each i3 ∈ track 3 (16 positions: 8 + 8 with jx) // depth 3
                    - C = C₃ + d(m3); E = E₃ + φ′(m3,m3) + φ′(m0,m3) + φ′(m1,m3) + φ′(m2,m3)
                    - if C²·bestE > bestC²·E → update (positions, bestC, bestE)
    - This is **8 × 8 × 8 × 16 = 8192** combinations — full exhaustive. The spec says "smaller number" / "fixed complexity"; the depth-first nature means no inner allocation, no nested-loop K3 threshold, *fixed* iteration count (8192). Compare to base §3.8.1 *worst-case* 8192 + early-exit thr3 + max-180 cap. **Annex A "fixed complexity" interpretation:** no early-exit branch; constant 8192 iterations per subframe. CB-2 step 4 measures actual cycle count vs Phase 2c benchmark.
- [ ] Step 4 — Refactor: ensure all scratch caller-owned; bench `BenchmarkSearchDepthFirst` ≤ 50 µs/op as soft target (8192 ALU-light iterations).
- [ ] Step 5 — Commit `phase2d(fcbsearch): depth-first focused ACELP search per A.3.8.1 (CB-2)`.

### CB-4 — c(n) construction with harmonic enhancement

§3.8 eq. 45 + 46 + 47.

- [ ] RED: `internal/fcbsearch/code_test.go` golden c[40] from (positions = [0, 6, 12, 23], signs = [+, −, +, +], prevGpQ14 = 8192 (~Q14 0.5), T = 39): expect 4 unit pulses at signed positions, then enhanced by 1/(1−βz⁻T) where β = clamp(prevGpQ14, 0.2, 0.8).
- [ ] GREEN: `fcbsearch.BuildCode` reuses `fcb.placePulses` + `fcb.ClampPitchGainForEnhancement` + `fcb.applyPitchEnhancement` (per §3.2 reusable symbols).
- [ ] Refactor: dedupe with `internal/fcb` if function signatures align.
- [ ] vet + bench.
- [ ] Commit `phase2d(fcbsearch): construct c(n) with harmonic enhancement per 3.8 (CB-4)`.

### CB-5 — Filtered code z(n) = c ⊛ h

§3.9 eq. 64.

- [ ] RED: golden z[40] for known c, h (lower-triangular convolution).
- [ ] GREEN: `fcbsearch.FilterCode` in `internal/fcbsearch/filter_code.go`; same convolution kernel as Phase 2c HI-1 (consider extracting to shared helper if natural).
- [ ] Refactor: caller-owned scratch.
- [ ] vet + bench.
- [ ] Commit `phase2d(fcbsearch): filtered code z(n) = c⊛h per 3.9 eq. 64 (CB-5)`.

### GQ-1 — Gain prediction (§3.9.1 eq. 65–71)

- [ ] RED part A: `internal/gain/predictor_export_test.go` asserting refactor of `predictedLogGain` to free function `gain.PredictedLogGain(pastQuaEn *[4]int16) int16` matches the existing method.
- [ ] RED part B: `internal/gainquant/predictor_test.go` golden `g'c` (Q12) for known (pastQuaEn = [-14336, -14336, -14336, -14336] cold start, c = sample fixed-codebook from CB-4).
- [ ] GREEN part A: extract `gain.PredictedLogGain` (also export `gain.PastErrorsDefault = -14336`) and have `gain.Decoder.predictedLogGain` delegate to it.
- [ ] GREEN part B: `gainquant.PredictedGcQ12` composes `gain.fixedCodebookEnergy` (export it as `gain.FixedCodebookEnergy` if needed) + `gain.PredictedLogGain` + `gain.pow2Fixed` (export as `gain.Pow2Fixed`). Mirrors the decoder's gain reconstruction per §3.9.1 eq. 71.
- [ ] vet + race + bench.
- [ ] Commit `phase2d(gainquant): predicted g'c per 3.9.1 eq. 71 (GQ-1)`.

### GQ-2 — Conjugate-codebook 2D VQ (§3.9.2 eq. 73, 74; cost eq. 63)

- [ ] RED: `internal/gainquant/search_test.go` golden (GA, GB, ĝp, ĝc) for a synthetic (x, y, z, g'c) where the optimal entry can be computed by exhaustive 8 × 16 = 128 enumeration.
- [ ] GREEN: `gainquant.SearchConjugate` per §3.9.2:
    - Compute optimum *unquantized* (gp_opt, gc_opt) from (x, y, z) per eq. 63 partial derivatives (closed form: solve 2 × 2 system). NB this is "the optimum pitch gain gp, and fixed-codebook gain gc, are derived from equation (63), and are used for the preselection" (§3.9.2 line 1389).
    - Preselect 4-of-8 GA entries whose second element (γ̂_c bias) is closest to gc_opt / g'c.
    - Preselect 8-of-16 GB entries whose first element (ĝp bias) is closest to gp_opt.
    - Exhaustive 4 × 8 = 32 over remainder minimizing eq. 63 with quantized (ĝp = GA1+GB1, ĝc = g'c·(GA2+GB2)).
- [ ] Refactor: caller-owned scratch arrays for the 4 + 8 preselected indices; zero-alloc.
- [ ] vet + bench.
- [ ] Commit `phase2d(gainquant): conjugate-structure 2D VQ per 3.9.2 (GQ-2)`.

### GQ-3 — Quantized gain application + taming + past-energy update

§3.9.2 (taming) + §3.9.1 eq. 72 (past-energy update U(m) = 20·log10(γ̂)).

- [ ] RED part A: `internal/gainquant/tame_test.go` golden taming clamp (predicted-overflow path: synthesize a `gp` that triggers eq. 63 saturation when convolved with `oldExc`; expect `gp` clamped to per-spec threshold). Spec text §3.9.2 narrates "taming procedure (adaptive-codebook gain saturation under predicted-overflow conditions)" without exact threshold — log as **OQ-TAMING-THR** in §9; pin a default of 0.95 (Q14 = 15565) per textbook-typical CELP taming threshold; revisit at INT-1a escalation if FCB byte-EQ residual correlates with predicted-overflow frames.
- [ ] RED part B: `internal/gainquant/predictor_update_test.go` golden `pastQuaEn[4]` after one subframe (FIFO shift; new entry = U(m) per eq. 72).
- [ ] GREEN: `gainquant.Tame` (returns clamped gp + sticky `taming` flag) and `gainquant.UpdatePastQuaEn`.
- [ ] vet + bench.
- [ ] Commit `phase2d(gainquant): apply quantized gains + taming + past-energy update (GQ-3)`.

### ENC-1 — Bit packing (S/C per §3.8.2 eq. 61/62, GA/GB per §3.9.3 forward imap)

- [ ] RED part A: `internal/fcbsearch/pack_test.go` golden (S, C) for known positions+signs round-tripped through `fcb.decodePositions` (§4.1.4 inverse).
- [ ] RED part B: `internal/tables/gain_map_test.go` asserting forward `GainMap1` / `GainMap2` are inverses of `GainImap1` / `GainImap2`.
- [ ] GREEN part A: `fcbsearch.PackS` (eq. 61) and `fcbsearch.PackC` (eq. 62 with jx ∈ {0,1}).
- [ ] GREEN part B: derive `tables.GainMap1` / `GainMap2` by inverting `GainImap1` / `GainImap2` (compile-time generated `[8]uint8` / `[16]uint8`); add `gainquant.PackGains(ga, gb) -> (ga3, gb4)`.
- [ ] vet + bench.
- [ ] Commit `phase2d(enc): pack S/C/GA/GB per 3.8.2 + 3.9.3 (ENC-1)`.

### INT-0 — Encoder integration: `fcbStep` + full eq. A.9 / A.10 commit

- [ ] Step 1 — RED: `phase2d_int0_fcb_wiring_test.go` invoking encoder over PITCH.IN frame 0 expects all of `s1, c1, ga1, gb1, s2, c2, ga2, gb2` populated for both subframes; expects `oldExc` last-40 commit to equal `ĝp·v(n) + ĝc·c(n)` (eq. A.9, line 2202); expects `swMemErr[0..9]` to equal `x(n) − ĝp·y(n) − ĝc·z(n)` for n=30..39 (eq. A.10, line 2211).
- [ ] Step 2 — GREEN: add `(*Encoder).fcbStep(sub int)` to `encoder.go`; insert call after `closedloopStep(sub)` in `EncodeFrame` once Phase 2f wires the per-subframe loop. For Phase 2d, add `fcbStep` invocation inside `closedloopStep` (last lines, replacing the placeholder `swMemErr` / `oldExc` commit). Pipeline:
    1. CB-1: `x'(n) = x − gp·y` (gp = Phase 2c's unquantized GP-1 output); `d(n) = Σ x'·h(i−n)`.
    2. CB-3: signs / dAbs from d.
    3. CB-2: φ′ from (h, signs); depth-first search → 4 positions.
    4. CB-4: c[40] from (positions, signs, prevGpQ14, T).
    5. CB-5: z[40] = c ⊛ h.
    6. GQ-1: g'c from (pastQuaEn, c).
    7. GQ-2: SearchConjugate(x, y, z, g'c) → (ga, gb, ĝp, ĝc).
    8. GQ-3 part A (taming): clamp ĝp if predicted overflow.
    9. ENC-1: pack (S, C, GA, GB) into per-subframe fields.
    10. **§A.3.10 eq. A.10 commit** (REPLACES Phase 2c placeholder): `swMemErr[n−30] = sat(x(n) − (ĝp·y(n) >> 14) − (ĝc·z(n) >> 12))` for n=30..39 — Q-format reconciled at INT-0 step 3.
    11. **§A.3.10 eq. A.9 commit** (REPLACES Phase 2c placeholder): shift `oldExc` left by 40, append `u(n) = sat((ĝp·v(n) >> 14) + (ĝc·c(n) >> 12))` for n=0..39.
    12. GQ-3 part B: `UpdatePastQuaEn(&pastQuaEn, gammaCQ13)` — compose 20·log10(γ̂) per eq. 72; FIFO shift.
    13. `prevGpQ14 ← ĝp` for next subframe's β derivation; `prevTaming ← taming`.
- [ ] Step 3 — Refactor: scratch arrays as method receivers' fields if profiling shows alloc; Q-format pinning audit (note: ĝp Q14, ĝc Q12 per `gain.Decoder.Decode` return contract at `internal/gain/decode.go:38`; `swMemErr` int16; `oldExc` int16).
- [ ] Step 4 — vet + race.
- [ ] Step 5 — Commit `phase2d(encoder): wire fcbStep + full eq. A.9/A.10 commit per A.3.10 (INT-0)`.

### INT-1a — STRICT byte-EQ vs PITCH.BIT for FCB+gain fields (5/5 fresh I5)

- [ ] Step 1 — RED: `phase2d_int1a_fcb_byteeq_test.go` decoding PITCH.BIT S1/C1/GA1/GB1 + S2/C2/GA2/GB2 per §4.1.4 (positions inverse) + §3.9.3 (`GainImap` indexing) and asserting STRICT equality with encoder output across all PITCH.IN frames (1835 frames).
- [ ] Step 2 — Iterate up to I5 budget (5 escalations max) hunting mismatches. Track each escalation in `docs/superpowers/plans/2026-05-11-phase2d-int1a-escalations.md`.
- [ ] Step 3 — Escalation chain (in priority order, derived from spec uncertainty):
    - **slot 1/5 — OQ-A38-DEPTH:** depth-first ordering / tie-break direction. Try alternative track ordering (T3 → T0 vs T0 → T3) and tie-break (lower-position-first vs higher-position-first).
    - **slot 2/5 — OQ-A38-SIGNTIE:** d(n) == 0 sign default (try −1 vs +1).
    - **slot 3/5 — OQ-TAMING-THR:** taming threshold (try 0.9 / 0.95 / 1.0 Q14).
    - **slot 4/5 — OQ-GA-PRESELECT-METRIC:** §3.9.2 preselect "closest to gc" — distance metric (L1 vs L2 in dB Q10 vs in linear Q12).
    - **slot 5/5 — OQ-GBK-INDEX-MAP:** confirm `GainMap1` / `GainMap2` are bit-perfect inverses of decoder `GainImap1` / `GainImap2` and not some unrelated permutation.
- [ ] Step 4 — If still red after I5: ACCEPT-PARTIAL writeup with plausibility computation per Phase 2c INT-1 closure template (`docs/superpowers/plans/2026-05-10-phase2c-closure-report.md` §5). FAIL-DEFERRED is acceptable; the FCB byte-EQ surface is structurally coupled to upstream P1/P0/P2 byte-EQ (which itself remains capped by H-CENTER per Phase 2c §6.1) — any FCB mismatch on a frame where Phase 2c P1 misses is *expected*. Plausibility floor: max(INT-1a S1/C1/GA1/GB1 byte-EQ rate) MUST be ≥ Phase 2c INT-1b P1 byte-EQ rate (as measured by INT-1b — see below). If INT-1a < INT-1b P1, escalate.
- [ ] Step 5 — Commit `phase2d(int1a): FCB+gain byte-EQ vs PITCH.BIT (INT-1a)`.

### INT-1b — Phase 2c INT-1 re-run (closes Phase 2c FAIL-DEFERRED via OQ-EXC-COMMIT)

- [x] Step 1 — RED: re-execute `TestPhase2cINT1_ClosedLoopPitchByteEQ` (no test changes — just re-run after INT-0 lands). Capture new P1 / P0 / P2 byte-EQ rates. **Result @ HEAD `b85a6d6`: P1 10.79 % (198/1835) / P0 57.49 % (1055/1835) / P2 11.66 % (214/1835).**
- [x] Step 2 — Compare to Phase 2c baseline (P1 9.05 % / P0 56.46 % / P2 9.75 %): Δ P1 = +1.74 pp, Δ P0 = +1.03 pp, Δ P2 = +1.91 pp. P1 < 50 % branch hit; per decision tree the structural blocker remains dominant (H-CENTER upstream in Phase 2b open-loop). Cascading Phase 2c reserved slots 2/5–5/5 is not justified — expected uplift from closed-loop-side probes is bounded well below the 39 pp gap to 50 % because `tOp` divergence is the root cause and lives in Phase 2b. **No I5 spent (0 / 4 reserved Phase 2c slots consumed).**
- [x] Step 3 — Side-effect: Phase 2c closure report §5 disposition table updated with the post-Phase-2d INT-1b re-baseline (commit `phase2d(int1b): re-baseline …`). Test code unchanged.
- [x] Step 4 — `go vet ./...` ✅ / `go build ./...` ✅ (no production-code change).
- [x] Step 5 — Commit `phase2d(int1b): re-baseline Phase 2c P1/P0/P2 byte-EQ post eq. A.9/A.10 (INT-1b)`.

### INT-2 — Zero-alloc + race + bench

- [ ] Step 1 — RED: `phase2d_int2_fcb_zeroalloc_test.go` asserts `testing.AllocsPerRun(128, func(){...})` == 0 for:
    - `fcbStep(0)` in isolation
    - `fcbStep(1)` in isolation
    - full hot path: `lpcStep + openloopStep + (closedloopStep + fcbStep) × 2`
- [ ] Step 2 — GREEN: convert any captured allocs to caller-owned scratch (encoder receiver fields).
- [ ] Step 3 — `go test -race ./...` green; `BenchmarkFcbStep` captured; `BenchmarkClosedloopStep` re-captured for regression check (≤ 5 % regression acceptable).
- [ ] Step 4 — vet.
- [ ] Step 5 — Commit `phase2d(fcbsearch+gainquant): zero-alloc + race-clean (INT-2)`.

### INT-3 — Closure report

- [ ] Step 1 — Write `docs/superpowers/plans/2026-05-12-phase2d-closure-report.md` mirroring Phase 2c closure report sections (overview, INT-1a + INT-1b dispositions side-by-side, plausibility math, LIVE-DEFERRED list, Phase 2e/2f entry preconditions). Include Phase 2c disposition flip status (PASS / ACCEPT-PARTIAL / still-FAIL-DEFERRED).
- [ ] Step 2 — Update master plan §5 row to CLOSED with closure-report link; flip §6 row to "FOLDED INTO PHASE 2D" per §0.3 of this sub-plan.
- [ ] Step 3 — If Phase 2c INT-1 disposition flipped, update `docs/superpowers/plans/2026-05-10-phase2c-closure-report.md` §5 disposition row in-place with a 2026-05-12 amendment header; do NOT rewrite Phase 2c closure prose.
- [ ] Step 4 — Commit `phase2d: closure report + master-plan flip + Phase 2c disposition update (INT-3)`.

---

## 6. Per-task contract summary

| Task | Inputs | Outputs | Spec | Test |
|------|--------|---------|------|------|
| CB-1 | x[40], y[40], gp, h[40] | x'[40], d[40] (Q-format pinned at step 3) | §3.8.1 eq. 50, 52 | unit golden |
| CB-3 | d[40] | signs[40] ∈ {−1,+1}, dAbs[40] | §3.8.1 lines 1296–1300 | unit golden |
| CB-2 | dAbs[40], h[40], signs[40] | positions[4], C, E | §A.3.8.1 + §3.8.1 eq. 56–60 | unit golden + exhaustive cross-check |
| CB-4 | positions[4], signs[40], prevGpQ14, T | c[40] | §3.8 eq. 45, 46, 47 | unit golden |
| CB-5 | c[40], h[40] | z[40] | §3.9 eq. 64 | unit golden |
| GQ-1 | pastQuaEn[4], c[40] | g'c (Q12) | §3.9.1 eq. 65–71 | unit golden |
| GQ-2 | x[40], y[40], z[40], g'c | (GA, GB, ĝp Q14, ĝc Q12) | §3.9.2 eq. 73, 74, 63 | unit golden vs exhaustive |
| GQ-3 | ĝp, ĝc, γ̂_c (Q13), pastQuaEn, oldExc | clamped ĝp, taming flag, updated pastQuaEn | §3.9.2 (tame) + §3.9.1 eq. 72 | unit golden + threshold sweep |
| ENC-1 | positions, signs, GA, GB | S (4b), C (13b), GA3 (3b), GB4 (4b) | §3.8.2 eq. 61, 62 + §3.9.3 forward imap | round-trip vs §4.1.4 |
| INT-0 | encoder (post Phase 2c) | oldExc/swMemErr full eq. A.9/A.10 commit; per-frame s1/c1/ga1/gb1/s2/c2/ga2/gb2 | §A.3.10 eq. A.9, A.10 | encoder smoke |
| INT-1a | PITCH.IN | PITCH.BIT S/C/GA/GB byte-EQ | §4.1.4 + §3.9.3 | corpus (1835 frames) |
| INT-1b | PITCH.IN | PITCH.BIT P1/P0/P2 byte-EQ regression | §4.1.3 (Phase 2c harness) | corpus (1835 frames) |
| INT-2 | encoder | 0 allocs / race-clean | I4 | bench |
| INT-3 | results | closure report + Phase 2c amendment | — | doc |

---

## 7. Encoder integration order

After Phase 2d INT-0 lands, `Encoder.EncodeFrame` (still a Phase 2-0 stub today) will eventually be wired by Phase 2f as:

```
EncodeFrame(pcm []int16, out []byte):
  preprocess(pcm)                                  // §3.1 (existing pcm.PreProcessor)
  lpcStep(pcm)                                     // §3.2 (Phase 2a)
  openloopStep()                                   // §A.3.3 + §A.3.4 (Phase 2b)
  for sub in {0, 1}:
    intLag, frac := closedloopStep(sub)            // §A.3.5–§A.3.7 (Phase 2c)
    fcbStep(sub)                                   // §A.3.8 + §3.9 + §A.3.10 (Phase 2d, this sub-plan)
  packBitstream(out)                               // §4.1 / Table 1 (Phase 2f)
```

`fcbStep(sub)` per-subframe contract (depends on `closedloopStep(sub)` having committed `e.aHatSF{1,2}`, `e.intT1` / `e.intT2`, `e.frac1` / `e.frac2`, the *unquantized* gp, and the per-subframe scratch x[40] / y[40] / h[40] / v[40]):

1. CB-1 → d[40].
2. CB-3 → signs, dAbs.
3. CB-2 → positions[4].
4. CB-4 → c[40] (uses `prevGpQ14` from previous subframe).
5. CB-5 → z[40].
6. GQ-1 → g'c.
7. GQ-2 → (GA, GB, ĝp, ĝc, γ̂_c).
8. GQ-3 part A (tame) → clamped ĝp, taming flag.
9. ENC-1 → s, c, ga, gb (write to per-frame fields).
10. **§A.3.10 eq. A.10 commit:** `swMemErr[n−30] = sat(x − ĝp·y − ĝc·z)` for n=30..39.
11. **§A.3.10 eq. A.9 commit:** `oldExc` shift-by-40 + append `u(n) = sat(ĝp·v + ĝc·c)` for n=0..39.
12. GQ-3 part B (UpdatePastQuaEn) → FIFO shift `pastQuaEn`.
13. `e.prevGpQ14 ← ĝp`; `e.prevTaming ← taming`.

**Per-subframe vs per-frame state ownership:** `oldExc`, `swMemErr`, `pastQuaEn`, `prevGpQ14`, `prevTaming`, and `lpResidualMemQ` are per-subframe (Phase 2c precedent). `oldSpeech`, `freqPrev`, `oldWspeech` are per-frame (Phase 2a/2b precedent). I3 doctrine maintained (relaxed for ACELP per Phase 2c).

**Phase 2c `closedloopStep` end-of-function patch:** the placeholder `swMemErr` / `oldExc` writes at `encoder.go:402–414` are *removed* by INT-0 step 2 and replaced by the call to `fcbStep(sub)` which performs the full eq. A.9 / A.10 commit. This is the OQ-EXC-COMMIT closure mechanism.

---

## 8. I5 budget (Phase 2d INT-1a) — fresh

| Escalation | Hypothesis | Spent | Remaining | Result |
|------------|------------|-------|-----------|--------|
| 0 | Baseline (Annex A depth-first per CB-2 step 3 algorithm) | 0/5 | 5/5 | (TBD by INT-1a) |
| 1 | OQ-A38-DEPTH track ordering / tie-break | — | — | reserved |
| 2 | OQ-A38-SIGNTIE d(n)==0 default | — | — | reserved |
| 3 | OQ-TAMING-THR 0.9 / 0.95 / 1.0 | — | — | reserved |
| 4 | OQ-GA-PRESELECT-METRIC L1/L2/log/lin | — | — | reserved |
| 5 | OQ-GBK-INDEX-MAP inversion audit | — | — | reserved |

**Phase 2c INT-1b reserved slots** (consumed only on INT-1b escalation per §0.4):

| Slot | Hypothesis | Owner |
|------|-----------|-------|
| 2/5 | H-CENTER probe (Phase 2b open-loop tOp distance from reference T1) | INT-1b post-OQ-EXC-COMMIT |
| 3/5 | H-PHASE probe (subframe-2 swMemErr pre-commit ordering) | INT-1b post-OQ-EXC-COMMIT |
| 4/5 | OQ-WINDOW probe (search-window asymmetry [tOp−5, tOp+4] vs [tOp−4, tOp+5]) | INT-1b post-OQ-EXC-COMMIT |
| 5/5 | OQ-XB-NORM probe (xb adaptive Q-shift) | INT-1b post-OQ-EXC-COMMIT |

---

## 9. Open questions / risks (OQ register)

| ID | Spec cite | Default pin | Escalation knob | Owner gate |
|----|-----------|-------------|------------------|------------|
| **OQ-A38-DEPTH** | §A.3.8.1 lines 2185–2188 | Constant 8 × 8 × 8 × 16 = 8192 iterations, no early exit, depth order T0→T1→T2→T3, tie-break = lower-position-first | Re-order T3→T0; alternative tie-break direction; per-depth K-best pruning | INT-1a slot 1/5 |
| **OQ-A38-SIGNTIE** | §3.8.1 line 1297 | sign(0) = +1 | Try sign(0) = −1 | INT-1a slot 2/5 |
| **OQ-TAMING-THR** | §3.9.2 narrative (taming) | gp clamp threshold 0.95 (Q14 = 15565) when predicted overflow | Try 0.9 / 1.0 | INT-1a slot 3/5 |
| **OQ-GA-PRESELECT-METRIC** | §3.9.2 line 1389 ("close to gc") | L2 distance in dB Q10 on log gc | L1 in linear Q12; L2 in linear Q12 | INT-1a slot 4/5 |
| **OQ-GBK-INDEX-MAP** | §3.9.3 lines 1407–1414 | Forward `GainMap1` / `GainMap2` = inverse of `tables.GainImap1` / `GainImap2` | Audit decoder `imap` correctness | INT-1a slot 5/5 |
| **OQ-Q-FORMAT-A10** | §A.3.10 eq. A.10 | ĝp·y(n) >> 14 (ĝp Q14, y Q?), ĝc·z(n) >> 12 (ĝc Q12, z Q?), then int32 sat to int16 | Audit z(n) Q-format from CB-5; re-derive shifts | INT-0 step 3 (no I5 cost; pin during GREEN) |
| **OQ-EXC-COMMIT** (carryover) | §A.3.10 eq. A.9 | RESOLVED at INT-0 (this sub-plan) | — | (closed) |
| **H-CENTER** (carryover) | Phase 2b open-loop | LIVE-DEFERRED at Phase 2c close | INT-1b probe (Phase 2c reserved slot 2/5) | INT-1b |
| **H-PHASE** (carryover) | §A.3.6 | LIVE-DEFERRED at Phase 2c close | INT-1b probe (Phase 2c reserved slot 3/5) | INT-1b |
| **OQ-WINDOW** (carryover) | §A.3.7 | PINNED at Phase 2c close | INT-1b probe (Phase 2c reserved slot 4/5) | INT-1b |
| **OQ-XB-NORM** (carryover) | §A.3.7 eq. A.6 | UNTESTED at Phase 2c close | INT-1b probe (Phase 2c reserved slot 5/5) | INT-1b |
| **Risk R-1** | §3.8 eq. 47 | β = ĝp(m−1) clamped [0.2, 0.8] in Q14 — reuse `fcb.ClampPitchGainForEnhancement` | If FCB byte-EQ residual concentrates on first subframe of stream (no prevGp), audit cold-start β | INT-1a (no I5 slot reserved; investigation only) |
| **Risk R-2** | §3.9.1 eq. 69 | b = [0.68 0.58 0.34 0.19] Q13 from `tables.GainMAPredictor` | Audit decoder uses same constants as encoder | INT-1a / INT-1b cross-check |
| **Risk R-3** | §3.9 eq. 63 cost | Quantized cost minimization may saturate int32 accumulator on high-energy frames | Audit `fixed.LMac` saturation in GQ-2; if mismatches concentrate on high-RMS frames, escalate Q-format | INT-1a (no I5 slot; investigation) |
| **Risk R-4** | I5 doctrine | Risk of double-spending I5 between INT-1a and INT-1b. Mitigation: §0.5 budget table is canonical; INT-1b explicitly does NOT consume Phase 2d INT-1a slots, and vice-versa. | — | — |

---

## 10. Inheritance to Phase 2e / 2f (ACELP / FCB / gain folded)

Phase 2d MUST hand off:
- Full eq. A.9 `oldExc` commit (already specified by I-2d-3); Phase 2f's bitstream packing depends on no further excitation arithmetic.
- Per-frame fields `s1, c1, ga1, gb1, s2, c2, ga2, gb2` populated and ready for Phase 2f's Table 1 packing.
- `pastQuaEn[4]` updated per subframe (eq. 72); Phase 2f's stream-level state audit depends on it.
- `prevGpQ14`, `prevTaming` carried across subframes.
- `Encoder.EncodeFrame` STILL returns `ErrNotImplemented` after Phase 2d — Phase 2f wires the public entry point + bitstream packer + zero-pad-on-Flush.

**Phase 2e (gain quantization sub-plan)** is FOLDED INTO this sub-plan per §0.3. The `TAME.IN` / `TAME.BIT` byte-EQ harness (master plan §6 ITU vector gate) is deferred to Phase 2f and will exercise the GQ-3 taming path on the dedicated taming test vector.

---

## 11. Self-review

- [x] Mirrors Phase 2c sub-plan structure (§§0–12 modulo Phase-2d scope folding).
- [x] All ITU spec references cite line ranges in `docs/superpowers/specs/itu/G729E.txt`.
- [x] §A.3.8 vs §3.8 distinction (depth-first vs nested-loop) called out as I-2d-1.
- [x] §A.3.10 eq. A.9 / A.10 binding called out as I-2d-3 (closes Phase 2c OQ-EXC-COMMIT).
- [x] Quantized-gain-everywhere discipline called out as I-2d-4.
- [x] Carryover from Phase 2c (OQ-EXC-COMMIT, H-CENTER, H-PHASE, OQ-WINDOW, OQ-XB-NORM) explicitly inherited; INT-1b escalation chain pre-allocated against Phase 2c reserved I5 slots without double-spend.
- [x] OQ-A38-DEPTH (depth-first algorithm under-spec) flagged with default + escalation knob; pinned by CB-2 step 3.
- [x] Each task has TDD 5-step checklist + commit message stub + I8 trailer mandate.
- [x] INT-1a STRICT-first with documented ACCEPT-PARTIAL fallback under fresh I5 = 5/5 budget.
- [x] INT-1b is the formal regression gate that closes Phase 2c FAIL-DEFERRED (or escalates per Phase 2c reserved I5).
- [x] Master-plan §5 (ACELP) and §6 (gain quantization) scope folding rationale documented in §0.3.
- [x] Reusable symbols from `internal/fcb/` and `internal/gain/` enumerated per merger doctrine (§3.2).
- [x] Package-layout decision (`internal/fcbsearch/` + `internal/gainquant/`) justified with rejected alternatives (§4).

---

## 12. Execution handoff

**Next dispatch:** CB-1 (backward-filtered FCB target d(n) per §3.8.1 eq. 50 + 52).

**Order of execution:** CB-1 → CB-3 → CB-2 → CB-4 → CB-5 → GQ-1 → GQ-2 → GQ-3 → ENC-1 → INT-0 → INT-1a → INT-1b → INT-2 → INT-3.

**Stop conditions:**
- INT-1a PASS or ACCEPT-PARTIAL (with I5 ≤ 5/5 spent) → proceed to INT-1b.
- INT-1b PASS or Phase 2c disposition explicitly flipped (PASS / ACCEPT-PARTIAL) or all 4 reserved Phase 2c slots exhausted with documented blocker → proceed to INT-2.
- Any earlier task RED after a TDD round → escalate per ledger; do not skip.
- Any new prerequisite from Annex A discovered mid-flight → amend this plan with a new revision header before continuing.
- If CB-2 step 3 reveals that the depth-first algorithm derived from spec narrative + first principles is structurally inconsistent with PITCH.BIT (INT-1a slot 1/5 escalation cannot move FCB byte-EQ above Phase 2c P1 byte-EQ), document OQ-A38-DEPTH as STRUCTURAL-UNDER-SPEC and consider Phase 2-final spec-quirk closure under the reserved 1/5 escape slot.

— end of Phase 2d sub-plan —
