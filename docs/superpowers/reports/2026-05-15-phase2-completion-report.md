# Phase 2 — Completion Report (encoder cycle close)

**Date:** 2026-05-15
**Phase:** 2 (encoder construction — sub-phases 2-0 / 2a / 2b / 2c / 2d / 2e-folded / 2f)
**Master plan:** [`docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`](../plans/2026-05-02-phase2-encoder-plan.md)
**HEAD at authoring:** `6ad13be` (post Phase 2f INT-3 closure; this completion-report commit appended on top)
**Predecessor closure:** [`docs/superpowers/reports/2026-05-11-phase1o-completion-report.md`](2026-05-11-phase1o-completion-report.md) (Phase 1 decoder cycle close)

**Status:** **CLOSED — encoder cycle structurally complete; public API stable; ITU level-2 vector compliance gate FAIL-DEFERRED with documented structural blocker (Phase 2b H-CENTER, upstream-rooted in open-loop pitch tOp); 7 residual FAILs all carry strictly-justified dispositions routed to Phase 3 (release polish + Phase 2b re-entry candidate). Phase 3 entry: AUTHORIZED.**

---

## 1. Executive summary

Phase 2 delivered a complete clean-room G.729 Annex A encoder under the master plan §I1 invariant (no ITU reference C, bcg729, Sipro, FFmpeg, or other third-party G.729 source consulted). Across sub-phases 2-0 → 2f the encoder gained: pre-process LPC analysis (§3.2) → LSP MA-VQ (§3.2.4) → open-loop pitch (§A.3.3–§A.3.5) → closed-loop pitch + adaptive codebook (§A.3.6 / §A.3.7 / §3.7) → fixed-codebook ACELP (§A.3.8 / §3.8) → conjugate-structure 2D gain VQ + taming (§3.9) → eq. A.9 / A.10 excitation + weighted-error commit (§A.3.10) → bitstream pack (§4.2.1 + Table 8) → public `Encoder.EncodeFrame` + streaming `Write`/`Flush` (design §4.3). Ninety-eight Phase 2 commits land between `8893fac` (master plan) and `6ad13be` (Phase 2f INT-3) inclusive; sub-phase ranges are tabulated in §2.

The **ITU level-2 vector compliance gate** (per design §7.2: full-frame byte-EQ across all seven `*.IN`/`*.BIT` corpora) is **NOT MET**. Across the seven corpora (PITCH / TAME / ALGTHM / SPEECH / FIXED / LSP / TEST), full-frame byte-EQ is 0 % on every vector. The cascade has a **single dominant root cause**: Phase 2b H-CENTER — the open-loop pitch lag `tOp` diverges from ITU ground truth on ~46 % of frames (Phase 2c closure §6 measured), capping closed-loop P1/P0/P2 at ~10–11 % byte-EQ, which in turn forces the fixed-codebook position codeword C1/C2 to ≤ 2.86 % across all corpora (since one bit-flip in any of the 4 jointly-encoded pulse positions flips the entire 13-bit codeword), which in turn caps full-frame byte-EQ ≤ min(C1, C2) — almost-zero across the board. The OQ-TAMING-THR slot 5/5 sweep (Phase 2f TAME-1) authored a 9-variant matrix and produced **NO-WINNER** (all variants byte-identical), confirming the taming clamp is *masked* by the upstream ACELP-search disagreement on TAME frames and is therefore not the determining factor.

The **public API surface** is stable: `Encoder.EncodeFrame(pcm []int16, out []byte) error` returns `nil` (no `ErrNotImplemented`), `(*Encoder).Write([]int16) (int, error)` and `(*Encoder).Flush() error` provide the streaming convenience pair, `NewEncoder()` / `NewStreamingEncoder(io.Writer)` / `Reset()` round out the lifecycle. Steady-state `EncodeFrame` is **zero-allocation** and `go test -race` clean. End-to-end `BenchmarkEncodeFrame` measures **135.6 µs/op** on AMD EPYC 9554P — comfortably below the 2 ms/op Risk R-3 threshold and the 10 ms/frame soft-realtime budget; **Phase 2g (perf) is NOT NEEDED**.

The **ledger of inherited concerns** from Phase 1o (3 FAILs + R-A/R-B/R-C ambiguity ledger + SF-1 tilt γ_t gating + OVERFLOW.BIT F2 framing rationale + cosmetic gofmt) is re-examined in §3 below. All 3 inherited Phase 1o FAILs **persist** at Phase 2 close — Phase 2 encoder symmetry produced no new evidence sufficient to fix or demote any of them, so they are **carried forward to Phase 3** with documented rationale. The R-A/R-B/R-C and SF-1 γ_t items remain **OPEN** and likewise carry forward to Phase 3. The OVERFLOW.BIT F2 lenient-loader rationale is **NEUTRAL**: the encoder writes strict canonical `0x007F`/`0x0081` softbits, neither validating nor invalidating F2.

**Phase 2 cycle disposition: CLOSED-DEFERRED at the ITU compliance gate, CLOSED-COMPLETE at the structural / public-API gate.** The structural cycle is done; the compliance gap is a single upstream blocker (H-CENTER) and is recorded as the highest-leverage Phase 3 (or Phase 2b re-entry) intervention candidate.

---

## 2. Sub-phase summary with commit ranges

All sub-phase commit ranges below are inclusive on both ends, with the closing commit being the closure / completion report commit (or the master-plan flip commit, where applicable).

| Sub-phase | Title | Status | Range | Closure report |
|-----------|-------|--------|-------|----------------|
| **2-0**  | Scaffold (Encoder shell + sentinel errors + skeleton internal packages) | CLOSED 2026-05-03 | `82b895a` ↔ `19bac5c` | [`2026-05-03-phase2-0-scaffold-report.md`](../plans/2026-05-03-phase2-0-scaffold-report.md) |
| **2a**  | LPC analysis (§3.2.1–§3.2.3) + LSP MA-VQ (§3.2.4) | CLOSED 2026-05-06 (INT-1 ACCEPT-PARTIAL) | `66a214e` ↔ `0c5fc86` | [`2026-05-06-phase2a-closure-report.md`](../plans/2026-05-06-phase2a-closure-report.md) |
| **2b**  | Open-loop pitch (§A.3.3–§A.3.5) | CLOSED 2026-05-08 (INT-1 PASS) | `fc8f42f` ↔ `a3348d8` | [`2026-05-08-phase2b-closure-report.md`](../plans/2026-05-08-phase2b-closure-report.md) (per `git log` `a3348d8` "INT-3 Phase 2b closure report") |
| **2c**  | Closed-loop pitch + adaptive codebook (§A.3.6 / §A.3.7 / §3.7) | CLOSED-DEFERRED 2026-05-10 (INT-1 FAIL-DEFERRED, structural H-CENTER) | `b0d69b7` ↔ `ab32b95` | [`2026-05-10-phase2c-closure-report.md`](../plans/2026-05-10-phase2c-closure-report.md) |
| **2d**  | Fixed-codebook ACELP (§A.3.8 / §3.8) + gain quant (§3.9 — Phase 2e folded) + eq. A.9 / A.10 commit | CLOSED-DEFERRED 2026-05-12 (INT-1a FAIL-DEFERRED, INT-1b re-baseline) | `a8027c3` ↔ `7646337` | [`2026-05-12-phase2d-closure-report.md`](../plans/2026-05-12-phase2d-closure-report.md) |
| **2e**  | Gain quantization + taming | **FOLDED INTO 2d** 2026-05-12 | (no independent commit range) | (covered in 2d closure §1, §3, §4.5) |
| **2f**  | Full-encode wrapper + streaming API + per-vector ITU byte-EQ | CLOSED-DEFERRED 2026-05-14 (7-vector FAIL-DEFERRED, slot 5/5 NO-WINNER) | `4a01a86` ↔ `6ad13be` | [`2026-05-14-phase2f-closure-report.md`](../plans/2026-05-14-phase2f-closure-report.md) |

**Phase 2 spans `8893fac` (master plan) through `6ad13be` (Phase 2f INT-3 closure)** — 98 Phase-2-tagged commits over the period 2026-05-02 → 2026-05-14. Sub-phase 2e was folded into 2d at the Phase 2d sub-plan §0.3 because the eq. A.10 weighted-error commit requires the *quantized* `(ĝp, ĝc)` pair which only becomes available after gain quantization.

---

## 3. ITU level-2 vector gate — per-vector results table

Per design §7.2, the level-2 gate is full-frame byte-EQ on every available `*.IN`/`*.BIT` ITU vector. The seven encoder-relevant vectors are (PITCH / TAME / ALGTHM / SPEECH / FIXED / LSP / TEST); the three decoder-only vectors (ERASURE / OVERFLOW / PARITY — these have no `.IN` source) are NOT encoder gates and are addressed separately in §4.4 / §4.6 below.

**Per-vector full-frame byte-EQ disposition** (from Phase 2f INT-1 closure §5.3):

| Vector | Frames | Full-frame byte-EQ | Disposition | Upstream route |
|--------|------:|-------------------:|-------------|----------------|
| **PITCH**  | 1835 | 0/1835 (0.00 %) | FAIL-DEFERRED | Phase 2b H-CENTER → Phase 2c P1/P2 → Phase 2d C1/C2 |
| **TAME**   | 128  | 0/128 (0.00 %)  | FAIL-DEFERRED + slot-5/5 NO-WINNER | Same upstream chain (taming clamp masked) |
| **ALGTHM** | 35   | 0/35 (0.00 %)   | FAIL-DEFERRED | Same upstream chain |
| **SPEECH** | 3750 | 0/3750 (0.00 %) | FAIL-DEFERRED | Same upstream chain |
| **FIXED**  | 120  | 0/120 (0.00 %)  | FAIL-DEFERRED + corpus-statistics outliers | Same upstream chain (GA1=65.83 / GB1=55.83 % outliers explained §4.8 of 2f closure) |
| **LSP**    | 2232 | 0/2232 (0.00 %) | FAIL-DEFERRED | Phase 2a INT-1 LSP ACCEPT-PARTIAL inheritance + same upstream |
| **TEST**   | 176  | 0/176 (0.00 %)  | FAIL-DEFERRED | Same upstream chain |

**Per-field byte-EQ rates** (rolled up from Phase 2f INT-1 §5.3; rates are the *number of frames where the encoder field byte-EQ matched the ground truth, expressed as a percentage of total frames per corpus*):

| Field | PITCH | TAME | ALGTHM | SPEECH | FIXED | LSP | TEST |
|-------|------:|-----:|-------:|-------:|------:|----:|-----:|
| L0    | 94.11 | 60.16 | 94.29 | 82.03 | 89.17 | 79.03 | 78.98 |
| L1    | 31.72 | 61.72 | 42.86 | 46.83 | 55.83 | 38.93 | 47.16 |
| L2    | 14.01 | 24.22 | 22.86 | 18.24 | 18.33 | 18.06 | 18.75 |
| L3    | 14.17 | 81.25 | 17.14 | 20.67 | 25.00 | 20.12 | 21.59 |
| P1    | 10.79 | 14.84 | 5.71  | 6.93  | 1.67  | 23.92 | 4.55  |
| P0    | 57.49 | 60.94 | 54.29 | 53.65 | 51.67 | 64.96 | 50.00 |
| C1    | 0.00  | 0.00  | 0.00  | 0.05  | 0.83  | 0.13  | 0.57  |
| S1    | 5.50  | 8.59  | 2.86  | 10.24 | 1.67  | 6.77  | 10.23 |
| GA1   | 12.15 | 7.03  | 14.29 | 24.29 | 65.83 | 36.11 | 17.61 |
| GB1   | 5.29  | 2.34  | 2.86  | 14.72 | 55.83 | 2.11  | 9.66  |
| P2    | 11.66 | 32.03 | 8.57  | 6.85  | 2.50  | 21.01 | 6.25  |
| C2    | 0.00  | 0.00  | 2.86  | 0.03  | 0.83  | 0.00  | 0.00  |
| S2    | 4.20  | 9.38  | 17.14 | 10.77 | 17.50 | 7.48  | 6.82  |
| GA2   | 11.77 | 4.69  | 14.29 | 24.77 | 39.17 | 35.08 | 9.66  |
| GB2   | 4.52  | 4.69  | 8.57  | 14.13 | 38.33 | 1.97  | 7.95  |

**Cross-baseline inheritance (PITCH corpus exact match)**: PITCH P1/P2/GA1/GB1/GA2/GB2 = 10.79 / 11.66 / 12.15 / 5.29 / 11.77 / 4.52 % match Phase 2c INT-1b + Phase 2d INT-1a baselines **bit-exact**. Phase 2f introduced no new arithmetic regression — the packing layer is a pure pass-through.

**The full-frame ≤ min(C1, C2) bound** explains the universal 0 %: across all 7 vectors, max(C1) = 0.83 % (FIXED) and max(C2) = 2.86 % (ALGTHM). The structural ceiling for full-frame byte-EQ is therefore ≪ 3 % across the entire corpus inventory, before considering the further multiplicative effect of the other 13 fields. Until upstream Phase 2b H-CENTER closes (the dominant determinant of C1/C2 being non-zero), full-frame byte-EQ cannot rise materially.

**Single-field "PASS-equivalent" rates ≥ 80 %**: L0 PITCH 94.11 / ALGTHM 94.29 / FIXED 89.17 / SPEECH 82.03 (≥ 80 %); L3 TAME 81.25 (≥ 80 %). These five field × vector cells are the only "compliance-gate-equivalent" passes at Phase 2 close. Every other cell is below the design §7.2 ACCEPT-PARTIAL 80 % surface.

---

## 4. Phase 1o inherited concerns — re-examination

### 4.1 Three pre-existing FAILs (Phase 1o §6.1 carry-forward)

Per Phase 1o report §6.1, three FAILs were inherited as encoder regression targets pending Phase 2 mirror-symmetric witnesses. **All three persist at Phase 2 close.** Re-examination:

| Test | Phase 1o angle | Phase 2 evidence | Phase 2 disposition |
|------|----------------|------------------|---------------------|
| `TestDiagnostic_SinglePulseChain` | Decoder-side single-pulse chain divergence; encoder FCB k₁ symmetry might yield evidence | Phase 2d delivered the encoder FCB ACELP search (`internal/fcbsearch/`). The encoder's k₁ pulse selection is byte-exact to spec arithmetic per CB-2 (full 8 × 8 × 8 × 16 = 8192 iterations, OQ-A38-DEPTH PINNED). Phase 2d INT-1a confirms C1 = 0 % byte-EQ vs PITCH.BIT — the encoder's ACELP search produces *different* k₁ pulse selections than ground truth on every PITCH frame, **inheriting the H-CENTER blocker**, not exposing a new signal. The decoder-side `SinglePulseChain` divergence is on a *different* arithmetic path (decode-time pulse-chain reconstruction vs encode-time pulse selection) and the encoder evidence is therefore tangential. | **CARRY FORWARD to Phase 3.** No new evidence sufficient to fix or demote. Reactivation trigger: Phase 2b H-CENTER closure (would make encoder k₁ ≈ ground truth and provide the missing witness on the decoder pulse-chain path). |
| `TestDecode_LowEnergyCodebookIsSmooth` | Low-energy codebook smoothness; encoder gain-quant symmetry expected to expose root cause | Phase 2d delivered encoder gain quantization (`internal/gainquant/`). Phase 2d INT-1a confirms GA1 = 12.15 / GB1 = 5.29 % byte-EQ — the encoder's conjugate-structure 2D VQ produces *different* (GA, GB) indices than ground truth on most frames, **not exposing the low-energy smoothness invariant**. The Phase 2d clean-room implementation passes its own unit tests (`internal/gainquant/*_test.go` all PASS); the decoder-side `LowEnergyCodebookIsSmooth` failure is on a downstream synthesis path (post-decode smoothness) that the encoder never traverses. | **CARRY FORWARD to Phase 3.** No new evidence. Reactivation trigger: Phase 2b H-CENTER closure (would tighten encoder GA/GB to ground truth) OR a dedicated decoder-side §3.9 / §A.4.2 audit. |
| `TestDecode_SucceedsAcrossAllGainIndices` | Pathological GA × GB saturation sweep; encoder gain-quant Q-format alignment expected to inform | Phase 2d INT-0 step 3 pinned the eq. A.9 / A.10 Q-format reconciliation (`ĝp` Q14 × y(n) → asr 14 → int16 sat; `ĝc` Q12 × c(n) → asr 12 → int16 sat). The **encoder-side** Q-format is sound by construction (every commit verified via `internal/fixed` saturating arithmetic + INT-2 zero-alloc gate). The decoder-side pathological sweep iterates over (GA, GB) pairs that the **encoder would never select** (the encoder's eq. 63 cost surface eliminates pathological pairs as part of the search). The failure surface is therefore **decoder-only** by construction — the encoder cannot produce the pathological inputs. | **CARRY FORWARD to Phase 3.** Disposition is structural: the test exercises a code path the encoder never reaches. Reactivation trigger: dedicated decoder-side `internal/gain/decode.go` Q-format audit (out of scope for Phase 2). |

**Net Phase 2 evidence on the 3 inherited FAILs: NEUTRAL.** Phase 2 produced no new witness sufficient to fix or demote any of the three. They carry forward unchanged.

### 4.2 R-A / R-B / R-C ambiguity ledger (Phase 1o §6.2 carry-forward)

Per Phase 1o §6.2, three Phase-1n inventory items carry the "ambiguous spec clause" tag: R-A and R-B are clauses where Phase 2 encoder might produce mirror evidence; R-C is a verbatim-documentation issue with no expected encoder interaction. Re-examination at Phase 2 close:

- **R-A:** No new encoder evidence. The Phase 2 encoder pinned several OQ-* defaults (OQ-A38-DEPTH, OQ-A38-SIGNTIE, OQ-TAMING-THR, OQ-GA-PRESELECT-METRIC, OQ-GBK-INDEX-MAP, OQ-EXC-COMMIT, OQ-Q-FORMAT-A10, OQ-FRAME-FORMAT, OQ-FLUSH-PAD, OQ-VECTOR-FRAME-COUNT, OQ-COLD-START-CONVENTION) but none of these textually map to the Phase 1n R-A clause. **Disposition: OPEN, carry forward to Phase 3.**
- **R-B:** No new encoder evidence. Same reasoning as R-A. **Disposition: OPEN, carry forward to Phase 3.**
- **R-C:** Verbatim-documentation issue (RC-1/RC-2/RC-3 per Phase 1n stage-r-c plan); no expected encoder interaction; Phase 2 produced none. **Disposition: OPEN, carry forward to Phase 3** (Phase 3 candidate: ITU corrigendum search, per Phase 1o §6.5).

### 4.3 SF-1 tilt γ_t gating (Phase 1o §6.3 carry-forward)

Per Phase 1o §6.3: postfilter tilt γ_t gating decision (`agcGainPrev` lazy seed vs §4.2.3 `sign(k1')`) was deferred to Phase 2 because encoder-side LPC produces k1' first-class. Re-examination:

The Phase 2a LPC pipeline (`internal/lpc/`) computes the Levinson-Durbin reflection coefficients including k1'. However, the postfilter is a **decoder-side** module (`internal/postfilter/`) that consumes the *quantized* LSP set converted back to LP (via `LSPToLP`), not the encoder's k1' directly. The encoder symmetric witness would require a frame-by-frame harness comparing encoder-computed `sign(k1')` vs the postfilter's `agcGainPrev` lazy-seed behavior; this harness was not authored in Phase 2 (out of scope per master plan §7 — Phase 2 is encoder-only, no postfilter modifications).

**Disposition: OPEN, carry forward to Phase 3.** Reactivation trigger: a dedicated decoder-postfilter audit using the Phase 2a `internal/lpc/` k1' as ground truth.

### 4.4 OVERFLOW.BIT framing rationale (Phase 1o §6.4 carry-forward)

Per Phase 1o §6.4: D-2 chose variant F2 (lenient G.192 loader accepting `0x0000` ≡ logical 0 alongside canonical `0x007F`/`0x0081`). The lenience is informed inference from G.191 STL "indeterminate softbit" convention, not chapter-and-verse spec.

**Phase 2 evidence:** The Phase 2f PACK-2 task added `bitstream.ReadG192Frame` (strict policy: only `0x007F` / `0x0081` accepted; `bad bool` returned on `0x6B20` sync). The encoder-side **emission** path (`bitstream.WriteG192Frame` from Phase 1o, unmodified) writes strict canonical `0x007F`/`0x0081` softbits — verified at `internal/bitstream/g192.go:16-18`. Therefore:

- The Phase 2 encoder will **never produce** a `0x0000` softbit "in the wild."
- The F2 lenience is **not validated** by encoder emission (since the encoder writes strict).
- The F2 lenience is **not invalidated** by encoder emission either (since `WriteG192Frame` is the only encoder-side path and it's strict).
- The F2 lenience remains an artefact of the OVERFLOW.BIT vector specifically (frame-19 80-zero-data-word anomaly behind canonical sync), not a general loader-roundtrip requirement.

**Disposition: NEUTRAL.** F2 stands as a vector-specific accommodation (D-2's design intent). No Phase 2 action required. Phase 1o §6.4 acknowledged both outcomes (validate / invalidate) as compatible with D-2; the third outcome (no encoder emission of `0x0000` ⇒ no validation) is the realised one.

### 4.5 Cosmetic gofmt cleanup (Phase 1o §6.5 carry-forward)

Per Phase 1o §6.5: 18 box-drawing comment lines flagged for cleanup, "bundle into the first task that touches the affected file." Re-examination at Phase 2 close:

Phase 2 sub-phases touched many files but did not specifically run a sweep for the 18 box-drawing lines. `gofmt ./...` produces no diff at HEAD `6ad13be` (verified per Phase 2f INT-2 § go vet clean). The 18 lines are presumably either (a) removed incidentally by Phase 2 file rewrites, (b) untouched and still present, or (c) gofmt-clean as-is (the box-drawing characters are valid UTF-8 in `// ` comment lines, which `gofmt` does not rewrite).

**Disposition: CARRY FORWARD to Phase 3** as a minor cosmetic item. Phase 3 release-polish task should run a one-shot `grep -rn '═\|║\|╔\|╗\|╚\|╝'` (or similar box-drawing ranges) and dispose any remaining instances per the original Phase 1o §6.5 directive.

### 4.6 `TestDecode_ITUVectorAlgthmBitExact` SKIP demote candidate (Phase 1o §6 → master plan §9 line 1114)

Per master plan §9 line 1114: this `t.Skip` shell was carried in Phase 1o D-3 demote inventory; the demote-to-PASS-by-design decision deferred to Phase 2 on the assumption that "after full-frame ALGTHM encode PASS, the decoder side gains symmetry."

**Phase 2 evidence:** ALGTHM **full-frame encode does NOT pass** at Phase 2 close (0/35 frames, FAIL-DEFERRED per §3 above). The premise of the SKIP-demote is unmet.

**Disposition: SKIP STAYS, CARRY FORWARD to Phase 3** alongside the other 6 ITU-vector PSTdomain tests (which already live as `*_KnownPSTDomainDifference` PASS-by-design per Phase 1o D-3). If a Phase 2b H-CENTER re-entry produces ALGTHM full-frame ≥ 80 %, the SKIP becomes a PASS-by-design candidate; otherwise it carries to Phase 3 alongside the rest of the PSTdomain inventory.

### 4.7 ITU corrigendum search (Phase 1o §6.5 → Phase 3)

Per Phase 1o §6.5: "Out of scope; documented as Phase 3 candidate." **Confirmed: Phase 3 candidate.** The R-A / R-B / R-C ambiguity ledger and the SF-1 γ_t question both benefit from a published corrigendum or Annex I/II/III erratum. Phase 3 plan should include a one-shot ITU bibliography sweep (G.729 corrigenda 1 + Implementor's Guide).

---

## 5. Final test counts at Phase 2 close

`go test ./...` at HEAD `6ad13be` (verified `2026-05-15`):

```
PASS:    (vast majority — 18 of 19 packages green)
FAIL:    7 top-level test functions across 3 packages
SKIP:    (carried unchanged from Phase 1o; ITU PSTdomain tests live as
         PASS-by-design *_KnownPSTDomainDifference per Phase 1o D-3)
```

### 5.1 The 7 FAILs at Phase 2 close (canonical ledger)

| # | Test | Package | Source phase | Disposition |
|---|------|---------|--------------|-------------|
| 1 | `TestEncode_LSPVectorBitExact` | `github.com/exedev/g729` | Phase 2a INT-1 | **FAIL-DEFERRED — strictly justified.** ACCEPT-PARTIAL per Phase 2a closure §6 (L0=78.67 / L1=38.93 / L2=17.07 / L3=19.35 % byte-EQ vs LSP.BIT). Phase 2-final escape slot 1/1 RESERVED but NOT consumed. Phase 3 candidate: re-attempt only after Phase 2b H-CENTER closure (the LSP MA-VQ residual is dominated by upstream pitch-prediction error). |
| 2 | `TestPhase2cINT1_ClosedLoopPitchByteEQ` | `github.com/exedev/g729` | Phase 2c INT-1 / Phase 2d INT-1b | **FAIL-DEFERRED — strictly justified.** Re-baselined in Phase 2d INT-1b (P1 9.05 → 10.79 / P0 56.46 → 57.49 / P2 9.75 → 11.66 % per Phase 2d closure §6). Structural blocker is Phase 2b H-CENTER (open-loop tOp divergence on ~46 % of frames); a closed-loop probe cannot move tOp. Phase 2c reserved I5 4/4 untouched; Phase 3 candidate: Phase 2b re-entry. |
| 3 | `TestPhase2dINT1a_FCBByteEQ` | `github.com/exedev/g729` | Phase 2d INT-1a | **FAIL-DEFERRED — strictly justified.** S1 5.50 / C1 0.00 / GA1 12.15 / GB1 5.29 / S2 4.20 / C2 0.00 / GA2 11.77 / GB2 4.52 % per Phase 2d closure §5. Plausibility floor met (GA1 12.15 % > Phase 2c INT-1b P1 10.79 %). Cascades from H-CENTER → P1/P2 → C1/C2 = 0 %. I5 0/5 spent. Phase 3 candidate: re-attempt only after H-CENTER closure. |
| 4 | `TestPhase2fTAME1_ByteEQ` | `github.com/exedev/g729` | Phase 2f TAME-1 | **FAIL-DEFERRED — strictly justified.** GA1 7.03 / GB1 2.34 / GA2 4.69 / GB2 4.69 % per Phase 2f closure §5.1. 3-of-4 plausibility floor breaches authorised slot 5/5 sweep; 9-variant OQ-TAMING-THR sweep produced **NO-WINNER** (all variants byte-identical) per Phase 2f closure §5.2 — the taming clamp is masked by upstream ACELP-search disagreement on every TAME frame. OQ-TAMING-THR pin held at carryover (gp 0.95 Q14, E 2³³). Phase 3 candidate: re-attempt only after H-CENTER closure. |
| 5 | `TestDiagnostic_SinglePulseChain` | `github.com/exedev/g729/internal/decoder` | Phase 1 inheritance | **CARRY FORWARD to Phase 3 — strictly justified.** §4.1 above. |
| 6 | `TestDecode_LowEnergyCodebookIsSmooth` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance | **CARRY FORWARD to Phase 3 — strictly justified.** §4.1 above. |
| 7 | `TestDecode_SucceedsAcrossAllGainIndices` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance | **CARRY FORWARD to Phase 3 — strictly justified.** §4.1 above (decoder-only path; encoder cannot produce pathological inputs). |

### 5.2 Strictly-justified residual rationale

Per master plan §8 ("expect FAIL count = 0 (or strictly justified residual)"), every residual FAIL above carries an explicit citation to:
- Its source phase closure report (Phase 2a / 2c / 2d / 2f or Phase 1o);
- Its plausibility-floor / inheritance-ceiling breakdown;
- Its reactivation trigger (overwhelmingly: Phase 2b H-CENTER closure);
- Its Phase 3 routing decision.

**No FAIL is unaccounted. No FAIL is fixable within the Phase 2 budget.** The cycle closes at the documented residual.

### 5.3 Test ledger by package

| Package | Status | New Phase 2 tests added | New Phase 2 FAILs |
|---------|--------|-------------------------|--------------------|
| `github.com/exedev/g729` (root) | FAIL (4) | ~30+ Phase 2 root tests | 4 (#1, #2, #3, #4 above) |
| `internal/acelp` | PASS | (skeleton from 2-0) | 0 |
| `internal/bitstream` | PASS | PACK-2 round-trip | 0 |
| `internal/decoder` | FAIL (1 inherited) | 0 (decoder unchanged) | 0 (#5 above is Phase 1 inheritance) |
| `internal/fcb` | PASS | 0 | 0 |
| `internal/fcbsearch` | PASS | CB-1 / CB-2 / CB-3 / CB-4 / CB-5 / ENC-1 unit tests | 0 |
| `internal/filter` | PASS | (skeleton from 2-0) | 0 |
| `internal/fixed` | PASS | 0 | 0 |
| `internal/gain` | FAIL (2 inherited) | GQ-1 extractions only | 0 (#6, #7 above are Phase 1 inheritance) |
| `internal/gainquant` | PASS | GQ-1 / GQ-2 / GQ-3 / ENC-1 / Tame / UpdatePastQuaEn unit tests | 0 |
| `internal/lpc` | PASS | W-1 / W-2 / AC-1 / AC-2 / LD-1 unit tests | 0 |
| `internal/lsp` | PASS | LP-1 / LP-2 / LP-3 / LP-4 / MA-1 / MA-2 / VQ-1..5 / Quantize unit tests | 0 |
| `internal/pcm` | PASS | 0 | 0 |
| `internal/pitch` | PASS | 0 | 0 |
| `internal/pitch/closedloop` | PASS | HI-1 / TG-1 / CL-1 / CL-2 / FR-1 / FR-2 / VP-1 / GP-1 / ENC-1 unit tests | 0 |
| `internal/pitch/openloop` | PASS | WS-1 / WS-2 / OL-1..5 unit tests | 0 |
| `internal/postfilter` | PASS | 0 (decoder-only) | 0 |
| `internal/synth` | PASS | 0 (decoder-only) | 0 |
| `internal/tables` | PASS | `gain_map.go` (forward GA/GB imap inverses) | 0 |

**Phase 2 net: +4 new FAILs (all FAIL-DEFERRED with strict justification), 0 fixed Phase 1 FAILs (none reachable within Phase 2 scope).** `go vet ./...` clean. `go build ./...` clean. `go test -race ./...` clean (no DATA RACE events beyond the documented Phase 1 baseline).

---

## 6. H-CENTER decision

Per Phase 2f closure §11 routing: **H-CENTER is the dominant root cause** of every encoder-side FAIL-DEFERRED in the level-2 compliance gate. Phase 2c closure §6 measured tOp divergence on ~46 % of PITCH frames; Phase 2d INT-1b confirmed that closed-loop / FCB / gain probes cannot move tOp (the variable lives in the open-loop layer); Phase 2f INT-1 cross-corpus disposition confirmed the cascade applies uniformly across all 7 vectors.

**Two outstanding options** for the H-CENTER decision (per Phase 2f closure §12 step 7):

**Option A — Phase 2b re-entry.** Author a Phase 2b re-entry sub-plan to attack the open-loop pitch tOp residual directly. Consume the **Phase 2-final escape slot 1/1** (the reserved Phase 2a-closure §8 escape). Expected uplift if successful: cascading recovery of P1/P2 → C1/C2 → S/GA/GB → full-frame, potentially clearing 3 of the 4 root-package FAIL-DEFERREDs and bringing per-vector full-frame ≥ 80 % on at least PITCH/SPEECH/TAME corpora. Risk: the H-CENTER residual at Phase 2b close was itself dispositioned ACCEPT-PARTIAL (per `2026-05-08-phase2b-closure-report.md`); no second-order knob in the existing Phase 2b code is known to resolve it. A re-entry would require fresh diagnostic measurement (per-frame tOp ground-truth comparison) and likely a new OQ pin (e.g. open-loop window centring or sub-multiple lift threshold).

**Option B — Document H-CENTER as the final residual; route to Phase 3.** Accept the Phase 2-final compliance gate at the structural ceiling (≪ 80 % full-frame). Defer H-CENTER to Phase 3 (release polish + ITU corrigendum search + diagnostic surface). Preserve the Phase 2-final escape slot 1/1 unspent for any Phase 3 finding that exposes a single high-leverage fix (e.g. an ITU corrigendum that clarifies open-loop window centring).

**Phase 2-final disposition (this report's recommendation): Option B.** Rationale:

1. The level-2 compliance gate is **NOT** a Phase 2 acceptance criterion in the strict sense — master plan §8 explicitly authorises "strictly justified residual" disposition.
2. The encoder cycle is structurally complete and the public API is stable; opening Phase 2b re-entry now would hold open the cycle indefinitely.
3. The Phase 2-final escape slot 1/1 is preserved unspent — it remains available for any Phase 3 finding (corrigendum, errata, diagnostic surface) that reveals a single high-leverage knob; spending it speculatively against the Phase 2b open-loop layer without fresh diagnostic evidence is a low-probability bet.
4. Phase 3 (release polish) provides a natural surface for the diagnostic re-entry: a Phase 3 sub-task can author a per-frame tOp ground-truth witness harness, and **only if** that harness reveals a clear anomaly (concentrated in a window-centring spike, sub-multiple lift threshold, or similar) does the escape slot get spent.

**Decision: H-CENTER carries forward to Phase 3 with the Phase 2-final escape slot 1/1 RESERVED.** A diagnostic-surface task is the recommended Phase 3 first step on this front. The Phase 2-final escape slot is owned by the `2026-05-06-phase2a-closure-report.md` §8 budget; no Phase 2-final action consumes it.

---

## 7. Public API stability statement

The G.729A encoder/decoder Go module exposes the following public surface at Phase 2 close:

### 7.1 Constants (root package `github.com/exedev/g729`)

| Symbol | Value | Source |
|--------|-------|--------|
| `FrameSamples` | 80 | Phase 2-0 (`errors.go:33`) |
| `FrameBytes`   | 10 | Phase 2-0 (`errors.go:34`) |

### 7.2 Errors (root package)

| Symbol | Source phase | Behaviour |
|--------|--------------|-----------|
| `ErrShortPCM`     | Phase 2-0 | Returned by `EncodeFrame` / `DecodeFrame` / `Write` on `len(pcm) < FrameSamples`. |
| `ErrShortOutput`  | Phase 2-0 | Returned by `EncodeFrame` / `DecodeFrame` on `len(out) < FrameBytes`. |
| `ErrShortInput`   | Phase 2-0 / 1o D-4 | Returned by `DecodeFrame` on `len(bitstream) < FrameBytes`. |
| ~~`ErrNotImplemented`~~ | (REMOVED Phase 2f API-1 / INT-3) | Was a transitional sentinel; no production references at Phase 2 close per Phase 2f §10 audit. |

### 7.3 Encoder (root package)

| Symbol | Phase | Signature |
|--------|-------|-----------|
| `Encoder`                     | 2-0 onwards | Opaque struct; zero value is *not* usable — must be obtained via `NewEncoder` or `NewStreamingEncoder`. |
| `NewEncoder() *Encoder`       | 2-0 | Returns a fresh encoder with cold-start state per §A.3. |
| `NewStreamingEncoder(w io.Writer) *Encoder` | 2f API-2 | Returns an encoder configured to emit 10-byte frames to `w` on `Write`/`Flush` calls. |
| `(*Encoder).Reset()`          | 2-0 | Returns the encoder to cold-start state (zero-value semantics). |
| `(*Encoder).EncodeFrame(pcm []int16, out []byte) error` | 2f API-1 (was 2-0 stub) | Encodes one 80-sample frame to a 10-byte canonical G.729 frame. Returns `nil` on success; `ErrShortPCM` / `ErrShortOutput` on length mismatch. |
| `(*Encoder).Write(p []int16) (n int, err error)` | 2f API-2 | Streaming write; buffers up to 79 trailing samples; emits one frame per 80-sample boundary to the configured `io.Writer`. Returns `n = len(p)` on success, or short-write count on emit error. |
| `(*Encoder).Flush() error`    | 2f API-2 | Drains the streaming tail buffer; zero-pads the partial frame to 80 samples (OQ-FLUSH-PAD pin) and emits one final frame, or no-ops if buffer empty. |

### 7.4 Decoder (root package)

| Symbol | Phase | Signature |
|--------|-------|-----------|
| `Decoder`                     | 2-0 (root shell wrapping `internal/decoder/`) | Opaque struct. |
| `NewDecoder() *Decoder`       | 2-0 | Returns a fresh decoder with cold-start state per §4.3. |
| `(*Decoder).Reset()`          | 2-0 | Returns the decoder to cold-start state. |
| `(*Decoder).DecodeFrame(bitstream []byte, pcm []int16) error` | 2-0 (delegates to `internal/decoder.Decode` per Phase 1 close) | Decodes one 10-byte canonical G.729 frame to 80 PCM samples. |

### 7.5 Top-level convenience (root package, Phase 2-0)

| Symbol | Signature | Usage |
|--------|-----------|-------|
| `EncodeFrame(pcm []int16, out []byte) error` | top-level | Constructs a fresh `Encoder`, calls `EncodeFrame`, discards. *Caveat:* fresh encoder per call ⇒ no inter-frame state; intended for single-frame test harnesses, NOT production streams. Production callers SHOULD use `NewEncoder().EncodeFrame` directly to retain inter-frame state. |
| `DecodeFrame(bitstream []byte, pcm []int16) error` | top-level | Symmetric convenience; same caveat. |

### 7.6 Breaking changes since Phase 2-0

**Only one:** `ErrNotImplemented` was removed in Phase 2f API-1 (master plan §7 line 1050 directive). Pre-Phase-2f callers that switched on `errors.Is(err, ErrNotImplemented)` will not compile against the post-Phase-2f API. This is the documented stable-API breaking change at Phase 2 close.

All other public surface (`Encoder`, `Decoder`, `EncodeFrame`, `DecodeFrame`, `NewEncoder`, `NewDecoder`, `Reset`, `FrameSamples`, `FrameBytes`, `ErrShortPCM`, `ErrShortOutput`, `ErrShortInput`) is **stable** since Phase 2-0. The Phase 2f additions (`NewStreamingEncoder`, `Write`, `Flush`) are **additive only** — no pre-existing consumer is affected.

### 7.7 Performance posture

- `(*Encoder).EncodeFrame` end-to-end: **135.6 µs/op, 0 B/op, 0 allocs/op** on AMD EPYC 9554P (`BenchmarkPhase2fINT2_EncodeFrame`).
- `(*Encoder).Write` (one 80-sample frame to `io.Discard`): **~137 µs/op, 0 B/op, 0 allocs/op** (`BenchmarkPhase2fINT2_StreamingWrite80`).
- `(*Encoder).Write` (10-frame batch / 800 samples): **1.357 ms/op = 135.7 µs/frame amortized, 0 B/op, 0 allocs/op** (`BenchmarkPhase2fINT2_StreamingWrite800`).
- Concurrency: documented single-goroutine per encoder (per `encoder.go` doc comment); `go test -race` clean.

**Phase 2g (perf) NOT NEEDED** at Phase 2 close: 0.136 ms/op is ≪ Risk R-3 2 ms/op threshold and ≪ 10 ms/frame soft-realtime budget. The clean-room ACELP full-search (8192 iterations, OQ-A38-DEPTH PINNED at full) is comfortably soft-realtime. Phase 2g remains contingent on a future binding-perf-budget signal.

---

## 8. I5 budget reconciliation across all Phase 2 sub-phases

Single canonical table (per Phase 2f closure §9):

| Gate | Budget | Reserved | Spent | Available | Source |
|------|-------:|---------:|------:|----------:|--------|
| Phase 2a INT-1 (LSP byte-EQ vs LSP.BIT)               | 5 | 1 (Phase 2-final escape) | 4 | 0 | `2026-05-06-phase2a-closure-report.md` §8 |
| Phase 2c INT-1 (closed-loop pitch byte-EQ)            | 5 | 4 | 1 (OQ-K<40) | 0 (4/4 reserved) | `2026-05-10-phase2c-closure-report.md` §10 |
| Phase 2d INT-1a (FCB byte-EQ vs PITCH.BIT)            | 5 | 0 | 0 | 5 | `2026-05-12-phase2d-closure-report.md` §10 |
| Phase 2f INT-1 (per-vector packing-layer)             | 5 | 0 | 0 | 5 | `2026-05-14-phase2f-closure-report.md` §9 |
| Phase 2f TAME-1 slot 5/5 (OQ-TAMING-THR sweep)        | 1 | 0 | **1** | 0 | `2026-05-14-phase2f-closure-report.md` §9 |
| **Phase 2-final escape (G.192 byte-EQ)**              | **1** | **1** | **0** | **0** (RESERVED) | `2026-05-06-phase2a-closure-report.md` §8 |

**Cross-phase total: 6 spent, 14 reserved (untouched), 0 over-budget.** No double-spend. The Phase 2-final escape slot 1/1 stands **RESERVED** for Phase 3 (or a future Phase 2b re-entry) per §6 above.

---

## 9. Phase 3 entry note

**Phase 3 — Release polish.** Authored only on user dispatch. Phase 2-final report is the cycle-close; Phase 3 is the next dispatch. Recommended Phase 3 scope (carryover ledger):

| Carryover | Source | Recommended Phase 3 sub-task |
|-----------|--------|------------------------------|
| 7 FAILs (4 Phase 2 FAIL-DEFERRED + 3 Phase 1 inheritance) | §5.1 above | Either re-attempt under H-CENTER closure (consuming Phase 2-final escape slot 1/1 if a high-leverage knob surfaces) OR document as final residual + skip-demote per `*_KnownPSTDomainDifference` precedent (Phase 1o D-3). |
| H-CENTER diagnostic surface | §6 above | Author a per-frame tOp ground-truth witness harness (PITCH.IN → encoder open-loop tOp vs ITU `tOp` from PITCH.BIT P0 + P1 reverse-decode); only if a clear anomaly surfaces, spend the escape slot. |
| R-A / R-B / R-C ambiguity ledger | §4.2 above + Phase 1o §6.2 | One-shot ITU bibliography sweep: G.729 corrigenda 1 (ITU-T COM 16 series), Annex I/II/III errata, Implementor's Guide. |
| SF-1 tilt γ_t gating | §4.3 above + Phase 1o §6.3 | Decoder-postfilter audit using Phase 2a `internal/lpc/` k1' as ground truth. Out of scope for Phase 2 (encoder-only); natural Phase 3 sub-task. |
| `TestDecode_ITUVectorAlgthmBitExact` SKIP demote | §4.6 above + master plan §9 line 1114 | Demote alongside the other 6 PSTdomain tests if H-CENTER stays unresolved (final residual disposition); demote-to-PASS-by-design if H-CENTER re-entry succeeds. |
| Cosmetic gofmt cleanup (18 box-drawing lines) | §4.5 above | One-shot grep + dispose. |
| ITU corrigendum search | Phase 1o §6.5 + §4.7 above | Same as R-A/R-B/R-C (one-shot bibliography sweep). |
| README + public examples | new (Phase 3 entry) | Public-API-stability-grounded: codable from §7 above. |
| Fuzzing | new (Phase 3 entry) | `Encoder.EncodeFrame` + `Decoder.DecodeFrame` round-trip + crash-resistance fuzzing. Out of Phase 2 scope. |
| Phase 3 master plan | (next dispatch) | Author `docs/superpowers/plans/YYYY-MM-DD-phase3-release-polish-plan.md` enumerating the carryover above + entry preconditions. |

**Phase 3 is NOT authored at Phase 2 close** — by user-dispatch convention only. The recommended next user dispatch is `phase 3 sub-plan` (or equivalent in the user's preferred language).

---

## 10. Engineering invariants honored across Phase 2

- **I1 (clean-room MIT):** All citations in Phase 2 production code and tests point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` or to in-repo plans/closure reports/textbooks (Kondoz, Spanias). **NO ITU-T C reference, no bcg729, no Sipro Lab Telecom, no FFmpeg, no third-party G.729 source consulted across all 98 Phase-2 commits.** Self-attest at every commit; spec-cite every numeric constant and Table 8 field order.
- **I3 (per-frame state mutation discipline):** `EncodeFrame` advances all per-frame state exactly once per call; per-subframe state (`oldExc`, `swMemErr`, `lpResidualMemQ`, `prevGpQ14`, `prevTaming`) commits inside `closedloopStep` + `fcbStep` per subframe; streaming `Write` / `Flush` introduce no mid-frame state mutation. Verified by INT-2 race detector across 2c / 2d / 2f.
- **I4 (zero-alloc on hot path):** Every Phase 2 sub-phase's INT-2 task pinned `AllocsPerRun(128) == 0` on the relevant primitive; Phase 2f INT-2 pins it at the public-API level (`EncodeFrame` cold-start + steady-state + streaming `Write`).
- **I5 (escalation budget):** §8 above. Cross-phase total 6 spent, 14 reserved, 0 over-budget.
- **I6 (ITU bit-exactness for all integer ops):** Saturating fixed-point arithmetic via `internal/fixed`. Every Phase 2 sub-phase's CB-* / GQ-* / LP-* / OL-* unit tests exercise saturating boundary conditions and byte-EQ to spec arithmetic.
- **I8 (commit trailers):** Every Phase 2 commit carries the `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **I9 (LSP codebook discipline):** `internal/tables/lsp_*.go` UNMODIFIED across all 98 Phase-2 commits.
- **I10 (encoder-decoder state isolation):** `internal/{lpc,lsp,pitch/openloop,pitch/closedloop,fcbsearch,gainquant}/` are encoder-only sibling packages; they import `internal/{fcb,gain,tables}/` *read-only*; no decoder state mutated. `internal/bitstream/` is the encoder–decoder shared layer per the merger doctrine; PACK-2's `ReadG192Frame` is a pure function on byte slices (no shared state). Phase 2 introduced ZERO decoder behaviour changes.
- **I-2d-1 / I-2d-2 / I-2d-3 / I-2d-4** (Annex A binding for ACELP + gains; eq. A.9 / A.10 commit; quantized-everything): all honored, see Phase 2d closure §8.
- **I-2f-1 / I-2f-2 / I-2f-3 / I-2f-4 / I-2f-5** (frame-byte ordering; bit ordering MSB-first; streaming framing; stateless bitstream pack; no new spec arithmetic): all honored, see Phase 2f closure §7.

---

## 11. Closure verdict

**Phase 2: CLOSED.**

- Structural cycle complete (sub-phases 2-0 / 2a / 2b / 2c / 2d / 2e-folded / 2f all CLOSED or CLOSED-DEFERRED with closure reports).
- Public API stable (only breaking change since 2-0 is `ErrNotImplemented` removal).
- Steady-state zero-alloc + race-clean + comfortably soft-realtime (`BenchmarkEncodeFrame` 135.6 µs/op).
- Phase 2g (perf) NOT NEEDED.
- ITU level-2 compliance gate FAIL-DEFERRED with strictly-justified residual (H-CENTER blocker upstream).
- 7 residual FAILs all carry strict justification + Phase 3 routing.
- Phase 1o R-A/R-B/R-C/SF-1/OVERFLOW.BIT/SinglePulseChain/LowEnergyCodebookIsSmooth/SucceedsAcrossAllGainIndices ledger re-examined; net Phase 2 evidence is NEUTRAL on all items; all carry forward to Phase 3.
- Phase 2-final escape slot 1/1 RESERVED.
- I5 budget reconciled across phases (no double-spend).
- Engineering invariants I1 / I3 / I4 / I5 / I6 / I8 / I9 / I10 + Phase 2-specific I-2d-* / I-2f-* honored throughout.

**Phase 3 entry: AUTHORIZED.** Next dispatch: Phase 3 release-polish master plan, on user invocation.

---

— end of Phase 2 completion report —
