# Phase 1o D-3 — State-Bearing Root-Cause Diagnostic Plan

**Date:** 2026-05-10
**Phase:** 1o sub-cycle D-3 (state-bearing root cause for the common TAME-shaped defect)
**Parent plan:** `docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md` §D-3.
**Anchor commits:** `654ffe4` (D-3.tame escalate) + `b43c689` (D-3 batch measurement record, 6-vector matrix).
**Reference test (read-only, untracked):** `internal/decoder/stagef_bis_diagnostic_test.go` — preservation invariant **lifted at Phase 1o entry**; this cycle does NOT touch it.
**Sources (absolute):** ITU-T G.729 PDF (`docs/superpowers/specs/itu/G729E.pdf`) + `READMETV.txt` + Kondoz / Spanias textbooks ONLY. ITU C reference / bcg729 / Sipro / FFmpeg / any other G.729 implementation = forbidden surface.

---

## 0. Problem statement (one paragraph)

All 6 newly-measured ITU vectors (`SPEECH`, `FIXED`, `LSP`, `PITCH`, `TEST`, `OVERFLOW`) plus the TAME reference share one diff signature: first divergence at frame 0 sample 0/1/40 with |Δ| ∈ {2,4,6}, growing within frame to 10²–10⁵, and a monotonic cross-frame cascade (frame-N sample-0 |Δ| grows with N). 100 % of frames in every vector are corrupted. Per `b43c689` measurement the verdict is **single common state-bearing defect** in a container that (a) is read during frame-0 sample-0/1, AND (b) carries wrong values across subframe / frame boundaries with positive feedback. Gate 17 is **NOT** this surface (DISPOSED PASS-by-design in `6633b28`; its 7-path mechanistic exhaustion in `2026-05-08-phase1n-stage-r-c-empirical-synthesis-report.md` §5 covered only the narrow ≤±3 frame-0 sf0 sample-5..7 sign window — none of those paths produce the cross-frame growth signature observed here).

---

## 1. Anti-precedent acknowledgement (gate-17 28-cycle pattern)

Gate 17 burned 30 sub-hypothesis refutations across 28 cycles (Phase 1k → Phase 1n) before being disposed PASS-by-design. The dominant failure modes were:
(a) sub-tasks defined as "investigate/measure further" with no falsification predicate,
(b) speculative spec re-reading without a byte-EQ assertion that could refute the candidate,
(c) cycle-end without a numeric stop rule.

This plan **forbids** all three patterns. Every sub-task below has:
- a single named state container,
- a SPEC-derived expected value (with §-ref),
- a byte-EQ assertion that REFUTES the hypothesis on PASS,
- a hard cap of **5 refutations before mandatory user-gate escalation** (§5.2).

---

## 2. Hypothesis enumeration (ranked priority)

Priority rule: (i) earliest-frame influence on the observed first-divergence sample first, then (ii) largest cascade multiplier (Q-format leverage × number of feedback paths). Per b43c689 every first-divergence is at frame 0 → only containers READ during frame-0 sample-0/1 can be the proximate cause; containers that further compound across frames raise the rank.

| Rank | Hypothesis ID | State container | File:line | Why ranked here |
|------|---------------|-----------------|-----------|-----------------|
| **H-1** | `agc-init-seed` | `postfilter.Postfilter.agcGainPrev int32` (Q24) | `internal/postfilter/types.go:22`, `internal/postfilter/agc.go:53–56,74` | Lazy-init seeds `agcGainPrev = gTargetQ24` on first call (`agc.go:54`). If spec mandates zero-seed (§A.4.2.4 / §4.3 catch-all), every output sample is multiplied by a Q24 gain that is already off-spec at frame-0 sf0 sample 0 — directly explains early ±2 + growth (Q24 multiplier × per-sample one-pole feedback at α=0.99 = exact "growing within frame" shape). Also gates γ_t in `tilt.go:66` (the SF-1 deviation already on the R-ledger). Highest leverage. |
| **H-2** | `agc-feedback-loop` | same `agcGainPrev` carried sample-to-sample inside `applyAGC` | `internal/postfilter/agc.go:58–74` | One-pole α=32440/32768 (Q15). Per-sample compounding is mathematically exactly the within-frame growth shape. Even if init seed is correct (H-1 PASS), an off-by-one in α or rounding term `+ (1<<14)` produces the same envelope. |
| **H-3** | `gain-ma-init` | `gain.Decoder.pastErrors [4]int16`, lazy seed `-14336` (Q10) | `internal/gain/types.go:15`, `internal/gain/decode.go:7,9,39–43,60–63,113–118` | First-call init fires INSIDE `Decode` after flag check (line 39); if seed value or order vs predictor read is off, gp/gc for frame-0 sf0 are wrong → excitation wrong → all downstream samples wrong → cascade. Q10 dB error → linear-domain multiplier. |
| **H-4** | `lsp-firstframe-prevlsp` | `lsp.Decoder.prevLSP [10]int16` seeded to `cos(i·π/11) Q15` after Decode body | `internal/lsp/decoder.go:21,26–32,94–97,108` | The seed is written at line 95 INSIDE the `if !d.initialized` branch placed AFTER `interpolateLSP` consumers (line 101). For frame 0, `interpolateLSP` is called with the pre-init zero-value `prevLSP` → sf-1 LP wrong → sample 0..39 wrong, then `prevLSP = lsp` (line 108) carries the wrong frame-0 LSP into frame 1 → cascade. Already partially examined for sample-5..7 by RC-1 (`a47f03f`) but NOT for sample 0/1 cascade. |
| **H-5** | `lsp-ma-pastresiduals-init` | `lsp.Decoder.pastResiduals [4][10]int16` seeded to `i·π/11 Q13` | `internal/lsp/decoder.go:15,37–48,62–66` | I-4 hard-spec invariant per Phase 1n CE-3 (`5232411`) — confirmed byte-EQ in 63 cells but only at the LSP residual level. If the predictor-FIFO advance order is wrong, frame-0 ω̂ is computed against pre-shift slot values. Lower probability than H-4 but same surface. |
| **H-6** | `pastexc-zero-init` | `decoder.Decoder.pastExc [153]int16` zero-init | `internal/decoder/types.go:27`, `internal/decoder/subframe.go:31,51–52` | `pitch.AdaptiveCodebook` reads `pastExc` for every sample of every subframe. `pastExc` advance happens at lines 51–52 AFTER `hpFilter` and AFTER `pst.Filter` — order is "post-output" and spec ordering must be confirmed. Zero-init is spec-correct for frame 0 (§4.3) but a buffer-update timing bug → frame-1 sf-0 reads pastExc populated from frame-0 sf-1 in wrong slot positions. |
| **H-7** | `hp-biquad-zero-init` | `decoder.hpX [2]int16, hpY [2]int32` zero-init | `internal/decoder/types.go:31–32`, `internal/decoder/hpfilter.go:27–30,61–64` | I-3 IIR pole-pair invariant (1.93/-0.94) means non-zero past ⇒ exponential transient. Zero-init at codec start is spec (§4.2.2 / §4.3 catch-all, I-2 / I-3). Verify: dump (hpX, hpY) entering frame-0 sf-0 = (0,0,0,0). Failure here would explain early ±2 + slow decay over many frames; but observed signature has ABRUPT growth not exp decay → low rank. |
| **H-8** | `synth-pastsynth-init` | `synth.Synthesizer.pastSynth [10]int16` zero-init | `internal/synth/types.go:9–13`, `internal/synth/filter.go:21,27,54` | Spec §4.3 mandates zero. Verify entry to frame-0 sf-0. Same failure mode shape as H-7 but through the LP IIR (10-tap). |
| **H-9** | `pf-pastResidual-init` | `postfilter.pastResidual [pitchMax+subframeLen]int16` zero-init | `internal/postfilter/types.go:15`, `internal/postfilter/postfilter.go:33–34`, `internal/postfilter/longterm.go:29–30,108–109` | Long-term postfilter reads `pastResidual[pitchMax+n−T]` for T ∈ [20,143]. Frame-0 sf-0 with T=20 indexes index 123 (zero-init ⇒ contribution 0, g_l clamped to 0 ⇒ rOut = r). If init non-zero, spurious correlation → wrong T selected by `refinePitch` → wrong rOut → cascade. |
| **H-10** | `pf-pastS-init` | `postfilter.pastS [lpcOrder]int16` zero-init | `internal/postfilter/residual.go:15–17,29` | FIR computeResidual reads pastS[0..9]. Zero-init spec-correct; verify. |
| **H-11** | `pf-pastSynthPost-init` | `postfilter.pastSynthPost [lpcOrder]int16` zero-init | `internal/postfilter/shortterm.go:14–15,27` | Short-term IIR memory; same as H-10 for the IIR. |
| **H-12** | `pf-pastTiltInput-init` | `postfilter.pastTiltInput int16` zero-init | `internal/postfilter/tilt.go:87,100` | tilt s_st(-1) at frame-0 sf-0 must be 0; verify. |
| **H-13** | `subframe-init-order` | sf-0 vs sf-1 ordering inside `decoder.Decode` | `internal/decoder/decode.go:32–47` | Order: lsp.Decode → pitch delays → decodeSubframe(sf-0) → decodeSubframe(sf-1) → ScaleUpSat. Spec (§4.1) demands lsp.Decode produce sf-1 LP from interpolation BEFORE sf-0 runs (sf-0 actually uses sf1A in current code: `d.decodeSubframe(&sf1A, …)` for first half). Verify naming/routing: is `sf1A` actually subframe-0 LP per spec (§4.1.5 sf-1 = first subframe = samples 0..39)? If swapped, frame-0 samples 0..39 use the WRONG LP → first-divergence sample 0/1, magnitude bounded for low-energy frames, growing for high-energy. |
| **H-14** | `frame-boundary-pastexc-swap` | `pastExc` slide direction at `subframe.go:51–52` | `internal/decoder/subframe.go:51–52` | `copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])` slides OLD samples LEFT (drop oldest 40), then writes current `u[:]` at the tail. Verify direction matches spec convention "pastExc[len-1] = u(-1) most recent" (`pitch/adaptive.go:29`). A direction-swap would put wrong samples in the AdaptiveCodebook indexing window → frame-1 sf-0 sample 0 wrong → cascade. |
| **H-15** | `gain-prev-gp-init` | `decoder.prevGpQ14 int16` zero-init + `ClampPitchGainForEnhancement` consumption | `internal/decoder/types.go:29`, `internal/decoder/subframe.go:28,54` | Used by FCB `betaQ14` clamp on the NEXT subframe. Frame-0 sf-0 reads zero (spec OK for codec-start); frame-0 sf-1 reads sf-0's gpQ14. Verify spec init ≡ 0. |

**Hypothesis count: 15.** Priority order above is the chase order for §3.

---

## 3. Falsification predicates (one byte-EQ per hypothesis)

Each predicate is a single byte-EQ assertion that, **if it PASSES (got == spec-expected)**, REFUTES the hypothesis. All are computable from the ITU PDF + already-verified internal helpers; no external G.729 implementation is consulted. All measurements run on TAME (smallest non-pathological vector per parent plan §2.2 row 6).

| H-ID | Falsification predicate (PASS ⇒ REFUTE) |
|------|------------------------------------------|
| H-1 | At entry to `applyAGC` for TAME frame 0 sf-0 sample 0, dump `pf.agcGainPrev` BEFORE the lazy-init branch executes. Spec-expected per §4.3 catch-all + §A.4.2.4 init: `0` (zero-init). PASS = `agcGainPrev == 0` AND post-init seed equals the formula `gTargetQ14 << 10` byte-EQ to the value computed from sTilt/s of frame-0 sf-0. |
| H-2 | After `applyAGC` completes for TAME frame 0 sf-0, dump `pf.agcGainPrev` (Q24). Compute the spec-expected closed-form `g(40) = α·g(39) + (1−α)·gTargetQ24`-iterated 40 times from seed and gTarget (single subframe, no per-sample sTilt dependence in g-update path). PASS = byte-EQ. |
| H-3 | At entry to `gain.Decoder.Decode` for TAME frame 0 sf-0 (first call), dump `d.pastErrors[0..3]`. Spec-expected (§4.3 / Annex A): `[-14336, -14336, -14336, -14336]` (Q10 dB) — verbatim per existing const `pastErrorsDefault`. PASS = byte-EQ. Then dump after Decode and verify FIFO advance: `pastErrors[0] = uCurrent` (computed from GA,GB indices) byte-EQ to spec eq.(72). |
| H-4 | At entry to `interpolateLSP` for TAME frame 0, dump `d.prevLSP[0..9]`. Spec-expected per §3.2.4 / §4.1.5 + I-4: `initialPrevLSP` constants (`internal/lsp/decoder.go:29–32`). PASS = byte-EQ. **Critical:** assert this is true BEFORE `d.prevLSP = lsp` line 108 runs, i.e., at the call site line 101 ON THE FIRST FRAME. |
| H-5 | At entry to `applyPredictor` for TAME frame 0, dump `d.pastResiduals[0..3][0..9]`. Spec-expected per §3.2.4 + I-4: each row = `initialPastResidual` (40 cells). PASS = byte-EQ. Then after Decode, verify FIFO ring advance (slot 0 ← residual just computed). |
| H-6 | At entry to TAME frame-0 sf-0 `pitch.AdaptiveCodebook`, dump `d.pastExc[0..152]`. Spec-expected per §4.3 catch-all (I-2): all 153 cells = 0. PASS = byte-EQ. Then at entry to TAME frame-0 sf-1, dump again and assert: pastExc[0..112] = 0; pastExc[113..152] = the `u` vector written by sf-0 (byte-EQ to recomputed `u` from BuildExcitation). |
| H-7 | At entry to TAME frame-0 sf-0 `hpFilter`, dump `d.hpX[0], d.hpX[1], d.hpY[0], d.hpY[1]`. Spec-expected per §4.2.2 + I-2/I-3: `(0, 0, 0, 0)`. PASS = byte-EQ. |
| H-8 | At entry to TAME frame-0 sf-0 `synth.Filter` (called via decodeSubframe), dump `d.syn.pastSynth[0..9]`. Spec-expected per §4.3 + I-2: all zero. PASS = byte-EQ. |
| H-9 | At entry to TAME frame-0 sf-0 `pst.Filter`, dump `pf.pastResidual[0..(pitchMax+subframeLen-1)]`. Spec-expected: all zero (§4.3 catch-all). PASS = byte-EQ. |
| H-10 | At entry to TAME frame-0 sf-0 `computeResidual`, dump `pf.pastS[0..9]`. Spec-expected: all zero. PASS = byte-EQ. |
| H-11 | At entry to TAME frame-0 sf-0 `applyShortTerm`, dump `pf.pastSynthPost[0..9]`. Spec-expected: all zero. PASS = byte-EQ. |
| H-12 | At entry to TAME frame-0 sf-0 `applyTiltWithMu`, dump `pf.pastTiltInput`. Spec-expected: `0`. PASS = byte-EQ. |
| H-13 | At entry to TAME frame-0 sf-0 `decodeSubframe`, dump the `sfA` argument's coefficients `sfA[0..10]`. Spec-expected per §4.1.5: subframe-1 (samples 0..39) uses the **interpolated** LP filter `(prevLSP + currLSP) / 2` → LSF → LP. PASS = byte-EQ to recomputed value from `interpolateLSP(initialPrevLSP, lsp_TAME_frame0)` → `lspToLP`. (This refutes a "sf-0 / sf-1 LP swap" only.) |
| H-14 | After TAME frame-0 sf-0 `decodeSubframe` returns, dump `d.pastExc[113..152]` and compare byte-EQ to the locally-recomputed `u` vector for sf-0. PASS = byte-EQ at every index. Slide-direction inversion fails this. |
| H-15 | At entry to TAME frame-0 sf-0 `decodeSubframe`, dump `d.prevGpQ14`. Spec-expected per §4.3: `0`. PASS = byte-EQ. Then at entry to TAME frame-0 sf-1, dump `d.prevGpQ14` and compare byte-EQ to the `gpQ14` returned by sf-0's `gn.Decode` call (recomputed locally). |

All dumps live in **one new t.Logf-only diagnostic test** (S-1 below). No production source change in S-1; no `t.Errorf` until S-2+.

---

## 4. Initial measurement strategy (sub-task S-1)

**S-1 — TAME frame 0/1 boundary state dump.**

- New file: `internal/decoder/phase1o_d3_s1_state_dump_diagnostic_test.go` (untracked until staged).
- Test name: `TestPhase1o_D3_S1_TameFrame01StateBoundaryDump`.
- Body: instantiate a fresh `Decoder`, feed TAME packed frames 0 and 1 via the existing harness helper, but BEFORE Decode of each frame and at fixed instrumentation points dump every state container in the rank-1..15 list.
- Output: ONE table per hypothesis row, written via `t.Logf`, formatted `H-N | container | got | spec-expected | Δ | PASS|FAIL`.
- The test ends with `t.Skip` summarizing "measurement-only — see logs". No assertion, so it does NOT error.
- Run command: `go test ./internal/decoder/ -run TestPhase1o_D3_S1 -v`. Capture full output.
- Deliverable: a table embedded in the S-1 commit message ranking hypotheses by `|Δ|` (largest first) and by "earliest instrumentation-point at which Δ ≠ 0" (earliest first).
- The intersection of those two rankings = chase order for S-2..S-K.

**S-1 stop:** as soon as a single H-N row shows non-zero Δ, S-1 logs that row + completes the rest of the table for completeness. Multiple non-zero Δ rows are EXPECTED (cascade); the chase order in §5.1 is "smallest-frame-position first, then largest |Δ|".

**Constraint:** S-1 produces ONE commit (`test(decoder): Phase 1o D-3 S-1 boundary state dump`). It must NOT modify production source and must NOT touch `decode_test.go`'s skips.

---

## 5. Stop rules

### 5.1 Single-hypothesis-explains-all rule
After S-1 dump, rank surviving hypotheses by (i) earliest non-zero-Δ frame position, then (ii) largest |Δ|. Take the rank-1 hypothesis to S-2 (production fix per §6 template). After the fix lands and TAME PASSes (`TestDecode_ITUVectorTameBitExact` byte-EQ green), **immediately** re-run all 5 remaining ITU vectors (`SPEECH`, `FIXED`, `LSP`, `PITCH`, `TEST`) plus `OVERFLOW` (D-2 dependency permitting). If ALL 6 PASS without any further change → STOP, write D-3 completion record, dispose 6 of the 7 `t.Skip` entries (per parent plan §D-3).

### 5.2 Hard refutation cap
**Maximum 5 sub-task refutations (S-2 through S-6) before mandatory user-gate escalation.** A "refutation" = a sub-task whose spec-derived byte-EQ predicate PASSes (i.e., the hypothesis was the wrong one). On the 5th refutation, the cycle MUST halt and produce a `2026-05-XX-phase1o-d3-statebearing-exhausted-report.md` document listing:
- the 15 enumerated hypotheses + the 5 refuted byte-EQ records,
- residual rank-6..15 hypotheses (deferred to next user gate),
- the hypothesis that was NOT yet falsified but cannot be tested without a spec-text source we do not yet possess (R-D candidate),
- recommendation for the user gate (corrigendum / Appendix I-III search vs deferral to Phase 2 encoder-side cross-check).

This cap is the explicit anti-precedent of the gate-17 28-cycle marathon.

### 5.3 Anti-speculation rule
A sub-task may be dispatched ONLY if it has all three of:
(a) a single named state container,
(b) a SPEC-derived expected value with a `§<n>` PDF citation in the sub-task description,
(c) a byte-EQ assertion that compiles without conditional knobs.

Sub-tasks of the form "investigate further", "trace the AGC behaviour", "compare across vectors" are FORBIDDEN.

### 5.4 No new ITU-impl reference
Reaffirm: ITU C reference / bcg729 / Sipro / FFmpeg / any other G.729 implementation MUST NOT be cited or executed at any point in this cycle.

---

## 6. Production fix sub-task template (S-2)

Once §5.1 selects the surviving hypothesis H-N, S-2 is dispatched as a TDD red→green:

1. **RED step** — un-skip `TestDecode_ITUVectorTameBitExact` (line 475 of `decode_test.go`) in the SAME commit as the failing assertion that captures the byte-EQ predicate violation found in S-1. Confirm `go test ./internal/decoder/ -run TestDecode_ITUVectorTameBitExact` FAILs on `main` (current symptom: first divergence per b43c689 row).
2. **GREEN step** — minimal production change to the named container (per H-N's file:line). Cite the §-section of the PDF that mandates the corrected behaviour in a `// Spec: §x.y.z (G729E.pdf p.NN) ...` comment on the changed line. The change MUST be the smallest diff that flips the byte-EQ predicate from FAIL to PASS for TAME and that does not regress any currently-PASSing test.
3. **Validation** — run, in this exact order, in one commit:
   - `go vet ./...`
   - `go test ./internal/decoder/ -run TestDecode_ITUVectorTameBitExact -count=1 -race` → PASS.
   - **Cross-validation spot check (mandatory, in same commit message)**: `go test ./internal/decoder/ -run TestDecode_ITUVectorOverflowBitExact -count=1` (after D-2 land) AND `go test ./internal/decoder/ -run 'TestDecode_ITUVector(Speech|Pitch|Lsp|Fixed|Test)BitExact' -count=1`. Record per-vector pass/fail in the commit body.
4. **Commit message form** — `fix(<package>): Phase 1o D-3 S-2 <H-N short name>` with a body that lists the spec citation and the per-vector cross-validation outcome.

---

## 7. Cross-validation criteria

Single-fix success is binary AND comprehensive:

| Vector | Required outcome after S-2 |
|--------|----------------------------|
| TAME | `TestDecode_ITUVectorTameBitExact` byte-EQ PASS over 80×128 = 10 240 samples. |
| SPEECH | byte-EQ PASS over 3750 frames × 80 samples. |
| FIXED | byte-EQ PASS over 120 frames × 80 samples. |
| LSP | byte-EQ PASS over 2232 frames × 80 samples. |
| PITCH | byte-EQ PASS over 1835 frames × 80 samples. |
| TEST | byte-EQ PASS over 176 frames × 80 samples. |
| OVERFLOW | byte-EQ PASS over 384 frames × 80 samples (only after D-2 loader fix lands; if D-2 is still pending, OVERFLOW is conditionally deferred and must PASS as soon as D-2 closes). |

If TAME PASSes but ANY non-OVERFLOW vector FAILs, the fix is **PARTIAL**:
- record the partial outcome,
- the residual delta becomes a NEW first-divergence signature,
- re-run S-1 dump on the FAILing vector with the smallest residual |Δ| (likely FIXED or LSP per `b43c689` magnitudes 2144 / 11774),
- restart §3 against the surviving hypothesis ranks.
- The hard cap §5.2 applies cumulatively: PARTIAL fixes count toward the 5-refutation budget if they leave ≥1 vector failing.

---

## 8. Risk register & mitigations

| # | Risk | Likelihood | Symptom that the cycle is becoming "gate-17-shaped" | Mitigation |
|---|------|------------|------------------------------------------------------|------------|
| **R-1** | Spec ambiguity on initial value of one of `agcGainPrev` / `pastTiltInput` / `pastResidual` (§4.3 catch-all is broad, §A.4.2.4 says "initialized to g_target" which is the current code's lazy seed). | MEDIUM | An H-N predicate cannot be expressed as byte-EQ because the spec does not pin a numeric init value. | Per §5.3, that H-N is NOT dispatched; instead it is logged as an `R-D` candidate (new R-blocking inventory item) and the next-rank hypothesis is taken. After 5 such non-dispatchable hypotheses, hard escalation per §5.2. |
| **R-2** | Multiple containers all show non-zero Δ at S-1 (cascade-amplified noise hides the proximate cause). | MEDIUM-HIGH | The "rank by earliest non-zero Δ" tie-breaker resolves to the same frame position on ≥3 hypotheses. | Force the chase to the container WHOSE NON-ZERO Δ APPEARS WITH THE SMALLEST ABSOLUTE VALUE first (a true root cause has SMALL Δ at the boundary; large Δs are downstream amplifications). Document the tie-break decision in the S-1 commit. |
| **R-3** | Production fix in `applyAGC` accidentally re-opens the disposed gate-17 sample-5..7 sign window (the 7-path enumeration and `pf.agcGainPrev` are mechanistically intertwined). | LOW-MEDIUM | After S-2, `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` (currently `t.Skip`) starts FAIL when un-skipped. | S-2 cross-validation §6.3 also runs `TestDecode_AlgthmFrame0Sf0Sample5to7_*` with `t.Skip` REMOVED in a scratch branch (NOT committed) to observe gate-17 behaviour delta. If gate-17 PASSes by-design AND the 6 vectors PASS → keep fix. If gate-17 regresses → re-evaluate as R-1. |
| **R-4** | The 5-refutation budget is exhausted because the defect lives in a state container we forgot to enumerate (e.g., `fcb` package or `fixed.Overflow` sticky flag). | LOW | After 5 refutations, all 15 enumerated containers byte-EQ to spec-expected. | Mandatory user gate (§5.2) opens a new enumeration cycle covering `internal/fcb` (`fixed.Overflow` sticky), `internal/pitch` (interpolation FIR taps), `internal/synth/excitation.go` (BuildExcitation rounding). NO new chase without the user gate. |
| **R-5** | Cross-frame cascade is caused by interaction between TWO containers each individually byte-EQ to spec, but in combination (e.g., subframe init order H-13 + pastExc swap H-14). | LOW | TAME PASSes after fixing H-N, but SPEECH or PITCH still FAILs with reduced |Δ|. | Treated as PARTIAL fix per §7; restart §3 on the residual signature. |
| **R-6** | Open-ended speculation pull (gate-17 trap). | MITIGATED BY DESIGN | A sub-task description contains the words "investigate", "trace", "explore", "characterise". | §5.3 anti-speculation rule rejects the dispatch. |

---

## 9. Confirmation — `stagef_bis_diagnostic_test.go`

Untracked at HEAD (`?? internal/decoder/stagef_bis_diagnostic_test.go`). 28-cycle preservation invariant **lifted at Phase 1o entry** (parent plan §2.4). This D-3 sub-cycle does **NOT** touch the file; its disposition is handled by D-3.bis per parent plan §D-3.bis, not by this sub-cycle. `git add` for this commit names ONLY the new plan markdown.

---

## 10. Recommended first sub-task

**S-1: TAME frame 0/1 boundary state dump** — see §4. Single commit, untracked-then-staged single new test file, `t.Logf`-only, no production change. Output ranks hypotheses for S-2 dispatch.

---

## 11. Sub-task pipeline summary

| Sub-task | Type | Predicate | Hard cap counted? |
|----------|------|-----------|-------------------|
| S-1 | measurement (boundary dump) | none — produces ranking | NO |
| S-2 | production fix on rank-1 hypothesis | TAME byte-EQ + 5-vector cross-validation | YES if PARTIAL or refuted |
| S-3 | production fix on rank-2 (only if S-2 refuted/partial) | same template | YES |
| S-4 | production fix on rank-3 | same | YES |
| S-5 | production fix on rank-4 | same | YES |
| S-6 | production fix on rank-5 | same | YES → on FAIL/PARTIAL, **MANDATORY USER GATE** per §5.2 |

Total budget: **1 measurement + 5 fix attempts**. Beyond that, escalation only.

---

## 12. S-1 completion record

**Status:** ✅ S-1 completed — see commit (test(decoder): Phase 1o D-3 S-1 TAME frame 0/1 state dump).

**Test:** `internal/decoder/phase1o_d3_s1_state_dump_diagnostic_test.go` ⇒ `TestPhase1o_D3_S1_TameFrame01StateBoundaryDump` PASSes (measurement-only, `t.Logf` dumps + final ranking summary). Cross-package private-state access via `reflect+unsafe` from a same-package test file (no production source change in any package).

**Measured first-divergence (TAME, fresh decoder):**
- IP-E (end of frame 0): sample **1**, got=0 want=2, **|Δ|=2**.
- IP-G (end of frame 1): sample **0**, got=0 want=−23, **|Δ|=23** (cross-frame cascade reproduced).

**IP-A boundary dump (live values):**
- All 15 containers byte-EQ to **zero** at IP-A (raw struct zero-value). Lazy seeds for `lsp.prevLSP`, `lsp.pastResiduals`, `gain.pastErrors`, `pf.agcGainPrev`, `pf.initialized`, `gain.initialized`, `lsp.initialized` are all **deferred to the first call inside their respective `Decode` bodies**. After IP-after-sf0 (frame 0):
  - `pf.agcGainPrev` = **11 863 040** (Q24, lazy seed = `gTargetQ14 << 10`); `pf.initialized=true`.
  - `gain.pastErrors` = `[-15009 -14336 -14336 -14336]` (slot 0 ← computed `U(0)`, FIFO advanced; slots 1..3 = `pastErrorsDefault`).
  - `lsp.prevLSP` = decoded LSPs (no longer the `initialPrevLSP` seed, because line 108 overwrites at end of `lsp.Decode`).
  - `lsp.pastResiduals[0]` = `[4171 6434 …]` (current residual at slot 0); `[1..3]` = `initialPastResidual`.
  - `pastExc head[0..9]` and `tail[143..152]` = zero (sf-0's tiny excitation u didn't visibly populate the head; rotation correct).
  - `hpY` = `[-94 -106]` (non-zero from sf-0 HP run; spec).
  - `pf.pastResidual / pastS / pastSynthPost / pastTiltInput` and `synth.pastSynth` = zero (per their per-call write semantics at this point).
  - `d.prevGpQ14` = **1995** (Q14 pitch-gain from sf-0).

**H-1..H-15 ranking table (live, by earliest non-zero deviation from spec-init then |Δ|):**

| Rank | H-ID | Container | Earliest non-zero Δ point | Magnitude | Spec-baseline citation |
|------|------|-----------|---------------------------|-----------|------------------------|
| 1 | H-1 | `pf.agcGainPrev` (Q24) | IP-after-sf0 (lazy-seed = `gTargetQ24` ≈ 1.19e7) | Q24-large | §4.3 catch-all "all memories ← 0"; §A.4.2.4 init clause is ambiguous re. seed — **REFUTED-BY-S-2** (see §13) |
| 2 | H-2 | `applyAGC` iteration `g` | IP-after-sf0 (compounded from H-1 via α=32440/32768 Q15) | derived | §A.4.2.4 one-pole α |
| 3 | H-13 | sf-0/sf-1 LP routing | sfA[0..10] for sf-0 = interpolated LP per §4.1.5 (matches; **REFUTED**) | 0 | §4.1.5 sf-1 = first half |
| 4 | H-4 | `lsp.prevLSP` | IP-A=0, but seeded INSIDE `lsp.Decode` BEFORE `interpolateLSP` consumer (line 95 < line 101); **REFUTED** | 0 | §3.2.4 / §4.1.5 |
| 5 | H-5 | `lsp.pastResiduals` | IP-A=0, seeded INSIDE `lsp.Decode` BEFORE `applyPredictor`; **REFUTED** | 0 | §3.2.4 |
| 6 | H-3 | `gain.pastErrors` | IP-A=0, seeded INSIDE `gain.Decoder.Decode` to `[-14336]×4` BEFORE first read; **REFUTED** | 0 | §3.9 / §4.1.6 |
| 7 | H-6 | `pastExc` | zero at IP-A; tail = `u` from sf-0 with correct slide direction; **REFUTED** | 0 | §4.3 catch-all |
| 8 | H-7 | `hpX/hpY` | (0,0,0,0) at IP-A; **REFUTED** | 0 | §4.2.2 / §4.3 |
| 9 | H-8 | `synth.pastSynth` | zero ×10 at IP-A; **REFUTED** | 0 | §4.3 |
| 10 | H-9 | `pf.pastResidual` | zero ×183 at IP-A; **REFUTED** | 0 | §4.3 |
| 11 | H-10 | `pf.pastS` | zero ×10 at IP-A; **REFUTED** | 0 | §4.3 |
| 12 | H-11 | `pf.pastSynthPost` | zero ×10 at IP-A; **REFUTED** | 0 | §4.3 |
| 13 | H-12 | `pf.pastTiltInput` | 0 at IP-A; **REFUTED** | 0 | §4.3 |
| 14 | H-14 | `pastExc` slide direction | sf-0 `u` recomputed locally byte-EQ to `pastExc[113..152]`; **REFUTED** | 0 | adaptive-codebook indexing convention (§3.7) |
| 15 | H-15 | `d.prevGpQ14` | 0 at IP-A; **REFUTED** | 0 | §4.3 |

**Top-3 candidates (post-S-1):**
1. **H-1** — `pf.agcGainPrev` lazy-seed = `gTargetQ24` instead of 0. The Q24 magnitude (≈ 1.19e7) directly seeds the one-pole α=0.99 feedback at sf-0 sample 0; for low-energy frames this overshoots the true ITU-spec g(0). The within-frame growth + cross-frame cascade signature on TAME maps EXACTLY to a per-sample multiplier walking from a wrong initial value through the α=0.99 lowpass.
2. **H-2** — `applyAGC` iteration internal. Co-falsified surface with H-1; if S-2 zero-seeds `agcGainPrev` and TAME still FAILs with reduced |Δ|, the residual bug lives in the iteration update (`(α·g + (1−α)·gTarget + (1<<14)) >> 15` rounding term).
3. **H-13** — sf-0/sf-1 LP routing kept on the rank list as an ordering-sanity anchor; first-look IP-B byte-EQ shows the correct interpolated LP is fed to sf-0 — refuted as a primary cause but worth re-checking if H-1 + H-2 both refute.

**Recommended S-2 first-fix hypothesis:** **H-1** — `internal/postfilter/agc.go` lines 53–56: drop the lazy-init branch, seed `agcGainPrev = 0` via the existing zero-value (§4.3 catch-all "all filter memories shall be initialised to zero"). Predicate: `TestDecode_ITUVectorTameBitExact` flips from FAIL to PASS without regressing any currently-passing test.

---

## 13. S-2 completion record (REFUTATION)

**Status:** ❌ S-2 REFUTED (NO-FIX) — fix attempt 1 of 5 consumed against the §5.2 hard cap.

**Test:** `internal/decoder/phase1o_d3_s2_h1_fix_test.go` ⇒ `TestPhase1o_D3_S2_H1_TameByteExact` preserved as `t.Skip` refutation record (full pre-/post-fix divergence table embedded in source comment).

**Production change:** none committed. The seed-removal patch
(`internal/postfilter/agc.go` lines 53–56 deleted, `gTargetQ24`
comment updated to cite §4.3) was applied for measurement only and
fully reverted before commit. The lazy-init branch (`if !pf.initialized
{ pf.agcGainPrev = int32(gTargetQ24); pf.initialized = true }`)
remains in tree, untouched.

**Cross-validation outcome (all 6 ITU vectors with seed REMOVED):**

| Vector | Pre-fix first divergence | Post-fix first divergence | Verdict |
|--------|--------------------------|---------------------------|---------|
| TAME | f0 s1 got=0 want=2 (Δ=-2) | **f0 s0 got=0 want=2 (Δ=-2)** — shifted EARLIER | FAIL |
| SPEECH | f0 s0 got=0 want=2 family | f0 s1 Δ=-2; f1 sf0 Δ=-6 | FAIL |
| FIXED | f0 s0 family | f0 s0 Δ=-2; f1 sf0 Δ=+235 | FAIL |
| LSP | f0 s0 family | f0 s0 Δ=-2; f1 sf0 Δ=+2 | FAIL |
| PITCH | f0 s0 family | f0 s0 Δ=-2; f1 sf0 Δ=-16 | FAIL |
| TEST | f0 s0 family | (same family signature) | FAIL |
| OVERFLOW | f0 s0 family | f0 s0 Δ=-2; f1 sf0 Δ=-5 | FAIL |

**Interpretation (per §8 R-1 spec ambiguity):** Pre-fix, sample 0 of
frame 0 had been *accidentally* matching the ITU reference because the
Q24 seed value (`gTargetQ14 << 10` ≈ 1.19e7) produced a sample-0
output coincidentally close to the spec output. Removing the seed
exposes that the sample-0 match was non-causal — there is a *separate*
divergence at sample 0 that the seed had been masking. Per R-1, the
§A.4.2.4 init clause ("initialized to g_target") is therefore the
*binding* clause for `agcGainPrev`, not the §4.3 catch-all; the
current implementation honours §A.4.2.4 correctly.

**Hypothesis ledger update:** H-1 marked **REFUTED-BY-S-2** in §12
ranking table. Surviving rank-1 candidate is **H-2** (`applyAGC`
iteration internals: α=32440/32768 Q15 rounding term, `+(1<<14)`
bias). The empirical signature (sample-0 mismatch on every vector with
identical Δ=-2 magnitude in the trivially-low-energy startup window)
points away from `applyAGC` arithmetic per se and toward an upstream
producer of `sTilt`/`s` at sf-0 sample 0 — cf. H-12 (`pastTiltInput`
init), H-11 (`pastSynthPost` init), or pre-postfilter signal scaling
(synth output → postfilter input handoff). S-3 will re-rank with
"smallest |Δ| at smallest frame position" first, on the FIXED.BIT
vector (smallest cross-frame cascade magnitude).

**Cumulative refutation budget:** 1 / 5 consumed. 4 fix attempts
remain before mandatory user gate per §5.2.

**Anchor commits:** S-1 = `aa27ad1`; S-2 = (this commit, see message).

## §14. S-3 stage-localisation outcome (H-11 family REFUTED, NO-FIX)

**Method.** A measurement-only sub-test (`internal/decoder/phase1o_d3_
s3_handoff_dump_test.go`) replays TAME frame 0 with the H-1 lazy seed
RESTORED (current production state) and dumps, for samples 0..7 of sf0
and 40..47 of sf1, the per-stage values: `u` (excitation),
`s` (synth), `sPf` (full postfilter), `hpOut` (HP filter), and `final`
(post-`pcm.ScaleUpSat`). Output is compared against `TAME.PST`.

**Per-stage table (TAME frame 0 sf0, samples 0..5):**

| idx | u | s | sPf | hpOut | final | want | Δfinal |
|----:|--:|--:|----:|------:|------:|-----:|-------:|
| 0 | 1 | 1 | 1 | 1 | 2 | 2 | 0 |
| 1 | 1 | **0** | 0 | 0 | 0 | 2 | **−2** ← first diff |
| 2 | 1 | 1 | 1 | 1 | 2 | 0 | +2 |
| 3 | 1 | 1 | 1 | 1 | 2 | 0 | +2 |
| 4 | 0 | −1 | −1 | −1 | −2 | 0 | −2 |
| 5 | 0 | 0 | 1 | 1 | 2 | 0 | +2 |

LP coefficients for sf0 (Q12): `[4096 2108 1500 -137 399 -135 156 -55
301 256 189]`. Inputs: `tInt=20 tFrac=0 gpQ14=1995 gcQ12=4153
v[0..7]=0 c[0..3]=8192`.

**Stage that introduces the −2.** Synth output `s[1]=0` is already
wrong (needs to be 1 to back-compute to `final=2`). Postfilter and HP
are pure pass-through on the {0, 1, −1} small-magnitude regime here;
AGC scales sample-1's input of 0 to 0 regardless of `agcGainPrev`
seed value. Hence the off-by-2 originates **inside `synth.Filter`
(1/A(z))**, BEFORE the synth → postfilter handoff.

**Hand-arithmetic at sample 1** with `a[1]=2108`, `pastSynth=0`:

```
lTemp = LMult(u[1]=1, a[0]=4096)             = 8192
lTemp = LMsu(lTemp, a[1]=2108, s[0]=1)       = 8192 - 4216 = 3976
lTemp = LShl(lTemp, 3)                        = 31808
s[1]  = Round(lTemp) = (31808+32768)>>16     = 0     (truncates 0.485 → 0)
```

Real-valued reference: `y[1] = u[1] − a[1]·y[0]/a[0] = 1 − 2108/4096 =
0.485`. The ITU reference apparently rounds (or uses a different
intermediate Q-format) such that the value lifts to 1. This is purely
a synth-stage rounding boundary, not a handoff issue.

**H-11 sub-hypothesis verdicts (all REFUTED):**

| sub-H | claim | refutation |
|-------|-------|------------|
| H-11a | `pastSynthPost` initial value | irrelevant — postfilter is pass-through on sample 1 |
| H-11b | postfilter consumes `s` via stale-by-one indexing | `sPf[n]==s[n]` for n∈{0..4}, no shift |
| H-11c | postfilter pre-emphasis Q14 floor vs symmetric rounding | tilt of magnitude<1 cannot lift 0 to non-zero |
| H-11d | synth `mem[]` zero-init OR off-by-one mem update timing | `pastSynth` is zero per §3.10/§4.3; mem write order is canonical |
| H-11e | HP input has off-by-one DC handling | HP[0]=1 matches; HP[1]=0 ⇐ correctly receives 0 from synth |

**Re-rank for S-4** (smallest spec delta first):

| rank | id | hypothesis | spec |
|-----:|----|------------|------|
| 1 | R-1 | Synth `onePass` rounding boundary — `Round((LShl(L,3))` may need different bias or `LShl(4)+Round` per spec semantics (§3.10 / §A.3.10 + G.191 basop ref for `round`/`L_shl`) | §3.10 |
| 2 | R-2 | LP-coefficient interpolation routes sf-0 LP to sf-1 slot or vice versa (§4.1.5) | §4.1.5 |
| 3 | R-3 | `BuildExcitation` rounding/scaling for `u[1]` (gc·c contribution at Q26→Q15 via LShr(11)) | §4.1.6 eq.(75) |

Recommended S-4 dispatch: **R-1** (synth rounding) — the hand-arithmetic
0.485 → 0 vs 1 directly evidences a rounding boundary; G.191 basop
semantics for `round`/`L_shl` are unambiguous in the spec.

**Cumulative refutation budget:** 2 / 5 consumed. 3 fix attempts
remain before mandatory user gate per §5.2.

**Anchor commits:** S-1 = `aa27ad1`; S-2 = `0428df7`; S-3 = (this
commit, see message).

---

### S-4: R-1 synth rounding boundary — REFUTED (NO-FIX)

**Procedure.** Per task brief, R-1 was decomposed into six sub-hypotheses
(R-1a..R-1f) covering Round bias, LShl shift count, accumulator initial
bias, equation form, a[0] interpretation, and u[n] pre-shift. Each was
arithmetically refuted against §4.1.6 eq. (77) and the G.191 STL basop
definitions in §6.2.1 Table 10. Diagnostic test:
`internal/decoder/phase1o_d3_s4_r1_synth_test.go`.

**R-1 sub-hypothesis verdicts (all REFUTED):**

| sub-H | claim | refutation |
|-------|-------|------------|
| R-1a | `Round` bias should be `0x00010000` (or other) instead of `0x00008000` | G.191 STL Table 10 fixes the bias at `0x00008000`. With `0x00010000` the result lifts to 1 but VIOLATES the basop spec. |
| R-1b | `LShl(L, 4)` instead of `LShl(L, 3)` | Breaks the trivial-passthrough contract test (`synth.TestQFormatContract_FilterSubframeAcceptsAOneQ12`): with a=[4096,0,…] and u[n]=n+1, `LShl(4)` doubles every output (sample-0 becomes 2 instead of 1). |
| R-1c | accumulator carries an implicit positive bias | §4.1.6 eq. (77) has no constant term; any added bias would also lift sample 0 (already correct → final=2 matches want=2). |
| R-1d | alt equation form `L_deposit_h(u[n]) + Σ −a[i]·s[n-i]` | Verified byte-EQ to current `L_mult(u[n], a[0]) + Σ −a[i]·s[n-i]` form (both produce 31808 → 0). |
| R-1e | `a[0] ≠ 4096 Q12` | Contradicts `internal/tables/lpc.go` and existing `synth.TestQFormatContract_FilterSubframeAcceptsAOneQ12` contract. |
| R-1f | u[n] pre-shift before accumulation | Sample-0 trace already matches want; any pre-shift on u would break sample 0. |

**G.191 basop spot-check (sample-1 chain):**

```
LMult(1, 4096)              = 8192
LMult(2108, 1)              = 4216
LSub(8192, 4216)            = 3976
LShl(3976, 3)               = 31808
LAdd(31808, 0x8000)         = 64576
ExtractH(64576)             = 0
```

All values match the G.191 STL definitions (§6.2.1 Table 10) bit-for-bit.

**Sensitivity check (what input perturbation lifts s[1] to 1?):**

| perturbation | s[1] | maps to |
|--------------|-----:|---------|
| `u[1]=2` (instead of 1, +1 LSB) | 1 | R-3 BuildExcitation rounding |
| `a[1]=1500` (instead of 2108) | 1 | R-2 LP interp routing |
| `s[0]=0` (instead of 1) | 1 | refuted: §3.10 mandates zero-init pastSynth, sample-0 trace s[0]=1 follows from u[0]=1 trivially |

**Re-rank for S-5** (smallest spec delta first):

| rank | id | hypothesis | spec |
|-----:|----|------------|------|
| 1 | R-3 | `BuildExcitation` rounding/scaling — `gc·c` (Q26 → Q15 via `LShr(11)`) at +1 LSB boundary (real `u[1]=4153/4096=1.014` rounds to 1 in production but a different Q-shift order may produce 2 byte-EQ to ITU) | §4.1.6 eq. (75) |
| 2 | R-2 | LP-coefficient interpolation routing — sf-0 a[] vs sf-1 a[] swap or wrong MA-predictor branch yields wrong a[1] (a[1]=1500 would also lift s[1] to 1) | §4.1.5 |

Recommended S-5 dispatch: **R-3** first (smallest spec delta — single
rounding/shift call site in `synth.BuildExcitation`).

**Cumulative refutation budget:** 3 / 5 consumed. 2 fix attempts
remain before mandatory user gate per §5.2.

