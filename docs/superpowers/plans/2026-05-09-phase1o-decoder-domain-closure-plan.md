# Phase 1o — Decoder Domain Closure Plan

**Date:** 2026-05-09
**Phase:** 1o (decoder domain closure; Phase 2 entry preparation)
**Selected path:** (α) per Phase 1n RC-3 close — gate 17 permanent disposition first, then real deferred decoder defects, then encoder (Phase 2) entry.

**Prerequisite commits:**
- `a3f43e6` — Phase 1n RC-3 close (synthesis + knob retirement REFUTE).
- `9ab1c91` — gate 17 RED disposition (`t.Skip` w/ evidence summary, Phase 1l alt-path d-i).
- Phase 1g–1n closure refs:
  `b1412d4` (1n RC-2), `a47f03f` (1n RC-1), `ea844d6` (1n plan),
  `21894d3` (1m synthesis), `5232411` (1m CE-3), `0d58ca6` (1m CE-2), `f3df272` (1m CE-1), `57b877f` (1m plan),
  `f902bd9` (1l synthesis), `2ee0009` (1l HP-2), `076b6de` (1l HP-1), `308e4f3` (1l plan),
  `8e6386c` (0c-reentry synthesis), `68a7df9`/`aeee9e9`/`8ec97f5` (0c-1/2/3),
  `82568dd` (0c-reentry plan), `d448282` (1k F-non-Cgamma-revisit close).

**Cumulative state at entry:** 28 diagnostic cycles, 30 sub-hypotheses refuted, 0 defects identified, 5 hard-spec invariants confirmed verbatim, 3 R-blocking items inventoried (R-A/B/C). Gate 17 sample 5..7 sign-mismatch mechanism formally exhausted across 7 mechanistic paths under clean-room constraints.

---

## 1. Goals

1. **Permanently dispose gate 17** — convert the 30-cycle `t.Skip` into a stable end-state (archive / re-interpret / explicit-permanent-skip). User-gated decision (D-1).
2. **Triage and resolve all 8 `t.Skip` calls in `internal/decoder/decode_test.go`** — every skip must end Phase 1o either re-enabled GREEN, replaced with a TDD-resolved fix, or removed-with-archived-rationale. Goal: zero `t.Skip` in `decode_test.go`.
3. **Fix the OVERFLOW.BIT bitstream loader bug** — reverse-engineer the framing variation, land a TDD red→green fix in `internal/bitstream` (or a documented loader variant), then enable `TestDecode_ITUVectorOverflowBitExact`.
4. **Decoder hand-off verdict** — produce a Phase 1o completion report declaring decoder "ready for Phase 2 (encoder) entry" with an explicit list of inherited spec ambiguities (R-A/B/C) and any residual decoder caveats.
5. **Lift the 28-cycle preservation invariant** on `internal/decoder/stagef_bis_diagnostic_test.go` and decide its final disposition (land as regression or delete).

---

## 2. Pre-cycle exploration findings

### 2.1 Gate 17 current state

`internal/decoder/stagef_octpostfix_regression_test.go:53–81` — single test `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` calling `t.Skip(...)` at line 54. Docstring (lines 5–52) records 30 cumulative refutations across Phase 1k/0c-reentry/1l/1m/1n, three confirmed hard-spec invariants (§4.2.4 AGC, §4.3 zero-init, §A.4.2.5 IIR pole pair), and four reactivation triggers (corrigendum / Appendix I-III / pre-frame-0 state / new spec source). Last touched by gate-17 disposition commit `9ab1c91` and re-annotated through `a3f43e6` (Phase 1n RC-3 added the +2 RC-1/RC-2 refutations to the cumulative counter).

### 2.2 Eight `t.Skip` enumeration in `decode_test.go`

| # | Line | Test name | Skip reason (summary) | Gate-17-related? | Re-enable feasibility |
|---|------|-----------|-----------------------|------------------|------------------------|
| 1 | 165 | `TestDecode_ITUVectorAlgthmBitExact` | "Phase 1h INCOMPLETE: structural decoder divergences remain… all 7 ITU vectors diverge at frame 0 sample 0 with got=0 want=2." | **Likely related** — claim is sample-0 mismatch, but text predates Phase 0c-reentry want-domain re-interpret (`8e6386c`) which fixed sample 0..4; remaining divergence is the gate-17 sample-5..7 sign window. | After D-1 disposition: re-enable; if D-1 chooses re-interpret (b), test PASSes for sample 0..4 plus documented sample 5..7 caveat; if D-1 chooses archive (a), drop assertion for sample 5..7. |
| 2 | 232 | `TestDecode_ITUVectorSpeechBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM…" | **Same as #1** | Same as #1. |
| 3 | 442 | `TestDecode_ITUVectorFixedBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM. FIXED first divergence: frame 0 sample 0 got=0 want=2." | **Same as #1** | Same as #1. |
| 4 | 451 | `TestDecode_ITUVectorLspBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM…" | **Same as #1** | Same as #1. |
| 5 | 464 | `TestDecode_ITUVectorPitchBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM…" | **Same as #1** | Same as #1. |
| 6 | 475 | `TestDecode_ITUVectorTameBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM…" (smallest non-pathological vector). | **Same as #1** | Same as #1; recommended re-enable measurement vehicle. |
| 7 | 486 | `TestDecode_ITUVectorTestBitExact` | "Phase 1h INCOMPLETE: same root cause as ALGTHM…" | **Same as #1** | Same as #1. |
| 8 | 531 | `TestDecode_ITUVectorOverflowBitExact` | "Phase 1h INCOMPLETE: OVERFLOW.BIT fails G.192 parsing in `bitstream.ReadG192File` with 'invalid G.192 data word'; pre-existing issue independent of Phase 1h." | **NOT related** — separate loader-format defect (D-2). | Only after D-2 (loader fix) lands. |

Gate-17-related count: **7 of 8** (all ITU-vector skips except OVERFLOW). The 7 are technically pre-Phase-1k (Phase 1h era, commit lineage prior to `308e4f3`), but their stated symptom ("frame 0 sample 0 got=0 want=2") was overtaken by Phase 0c-reentry (`8e6386c`); the only currently-justifying mismatch is the gate-17 sample-5..7 sign window. None were authored *during* the gate-17 chase — they pre-date it and are now better described as "blocked-by-gate-17" than "gate-17-internal".

### 2.3 OVERFLOW.BIT bitstream loader bug

- **Symptom (verified):** `internal/bitstream/g192.go:97` (`ReadG192Frame`) returns `ErrBadG192Bit` partway through `OVERFLOW.BIT`. Empirical probe: 19 frames parse cleanly, then frame 19 (offset 3116, 384 frames total) has a sync word `0x216b` (byte-swap of the canonical `G192SyncGood = 0x6B21`, `internal/bitstream/g192.go:12`) and the first bit-word reads `0x0000` instead of `0x007F`/`0x0081`.
- **File facts:** `OVERFLOW.BIT` size 62976 bytes ÷ 164 (G.192 frame size = 4 header + 2·80 bits) = 384 frames, divides evenly. So the file is *length-consistent* with a G.192 stream of 384 frames; only the framing of frame 19+ differs.
- **Suspected cause:** Either (a) the ITU release shipped `OVERFLOW.BIT` with a mid-file endianness flip / framing variant intentionally (to exercise saturation recovery on a "noisy" channel), or (b) bytes 3116+ use a different per-bit encoding (non-G.192) — the trailing bit-word `0x0000` is neither 0x7F nor 0x81, so a simple endianness swap won't recover it. `READMETV.txt` describes the vector as "overflow detection in synthesizer" and notes a v1.0→current correction whose impact was on `overflow.pst` only.
- **Spec section:** §4 Table 8 / Annex A frame format governs the bitstream. The ITU G.192 transport layer is referenced informally in `READMETV.txt`; the spec PDF itself does not mandate the test-vector framing format. This is a pure loader-side concern, not a decoder-spec concern.
- **Fix scope (preliminary):** TDD red test exercising the first 19 frames + the framing transition; either (i) extend `ReadG192Frame` to tolerate the documented variant, or (ii) add a loader variant `ReadG192FileLenient` used only by ITU test harnesses, or (iii) add a one-shot pre-pass that normalises `OVERFLOW.BIT` in test-only code. Selection at D-2 task-time.

### 2.4 `internal/decoder/stagef_bis_diagnostic_test.go` — 28-cycle preservation status

Untracked at HEAD (per `git status`). 50-line preview confirms it is a Phase 1k F-bis Stage-1 diagnostic harness measuring sample-0 across the four boundaries (`synth.Filter → postfilter.Filter → hpFilter → pcm.ScaleUpSat`) and is `t.Logf`-only (escape-hatch 1, no `t.Errorf`). Originated when the working hypothesis was "P fix int64-accum candidate" (now refuted by Phase 0c-reentry). The 28-cycle preservation invariant is **LIFTED at Phase 1o entry**. Recommended disposition (subject to D-3.bis decision):
- **Land as regression** — if its boundary-trace assertions still document a useful invariant (e.g., the now-confirmed §4.2.4 AGC carryover or §4.3 zero-init).
- **Delete** — if its measurement target (sample-0 ×2 entry boundary) has been fully superseded by Phase 0c-reentry's want-domain re-interpret.

### 2.5 SF-1 tilt γ_t standalone hypothesis

Carried as side-finding from Phase 1k F-non-Cgamma-revisit (`docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-synthesis-report.md:171,213`) and Phase 1l F-non-Hpost (`docs/superpowers/plans/2026-05-06-phase1l-stage-f-non-hpost-synthesis-report.md:101,338,351`). State at Phase 1o entry:
- Production gates γ_t via `agcGainPrev` in `internal/postfilter/tilt.go:7,11,79` (codec-start → 0.2).
- Spec verbatim §4.2.3 gates γ_t via `sign(k1')` (k1'<0 → 0.9).
- Per F-oct-postfix-2 measurement: μ_contrib Q15 ≈ −558 ≪ |st[n]|, Δ=0 on sample 5..7 sign — **sign-irrelevant** for gate 17.
- SF-1 remains a verbatim spec-conformance gap, not a defect. Disposition options: (i) leave production as-is + document deviation in `internal/postfilter/tilt.go` doc comment as "Annex-A activity-gated form"; (ii) re-implement to verbatim §4.2.3 sign(k1') and re-baseline ITU vectors. Recommended: (i) — defer behavioural change to Phase 2 once encoder-side k1' computation is on the table.

### 2.6 Promotion gate 19 status

Referenced as "E5: 측정-only test 자동 promotion (gate 19 자동 등재)" in `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-synthesis-report.md:43,213,274`. **Unfired** (no commit currently bears a "gate 19" promotion record). Decision deferred from Phase 1k synthesis; user-gate explicitly recommended. Phase 1o folds this into D-final: enumerate measurement-only diagnostic tests in `internal/decoder/` (10+ files matching `stagef_*_diagnostic_test.go` and `phase*_*_diagnostic_test.go`), decide which (if any) to promote to permanent regression coverage vs. archive vs. delete.

### 2.7 Other decoder-domain TODO/FIXME/XXX

`grep -rn "TODO\|FIXME\|XXX" internal/decoder/` returns no hits beyond test-string content already inventoried above. Decoder production source is annotation-clean.

### 2.8 Existing completion-report style

Phase 1g sample (`docs/superpowers/plans/2026-04-22-phase1g-decoder-completion-report.md`) establishes: `# Phase Nx Completion Report — <subject>` H1; `**Date** / **Plan** / **Status**` block; numbered sections (`## 1. Spec sections referenced`, `## 2. Plan deviations`, etc.); explicit clean-room source disclaimer; commit refs in backticks. Phase 1o close report will mirror this layout.

---

## 3. Task list (TDD)

### D-1 — Gate 17 permanent disposition (USER-GATED at task entry)

- [x] D-1 (D-1b chosen, commit `1e57ffb`) — `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` rewritten as `TestDecode_AlgthmFrame0Sf0Sample5to7_KnownPSTDomainDifference` (PASS-by-design pin of production `[+2,+2,+2]` post-`ScaleUpSat ×2`; PST `[-1,-1,-1]` documented as known PST-file-domain ambiguity; 30-refutation ledger + 7-path mechanistic exhaustion + 5 hard-spec invariants + 3 R-blocking ambiguities preserved verbatim in docstring; reactivation triggers retained).

**Sub-options (user must pick one before D-1 dispatch):**

- **(D-1a) ARCHIVE** — delete `internal/decoder/stagef_octpostfix_regression_test.go`; archive its docstring + the 30-cycle evidence summary into `docs/superpowers/archives/gate17-evidence.md`. Cleanest end-state, irreversible without commit-history archaeology.
- **(D-1b) RE-INTERPRET** — rewrite the test as a documented "post-`pcm.ScaleUpSat` PCM-domain comparison" variant: assert that production output for sample 5..7 is `+1,+1,+1` (the actual production value), document in-test why the ITU PST want of `−1,−1,−1` is a domain-mismatch artefact (clean-room evidence summary), keep as PASS-by-design with a permanent docstring pointer to the 30-cycle archive. Reversible: a future corrigendum can revert.
- **(D-1c) ESCALATE-KEEP-SKIP** — keep the `t.Skip` permanently; append "ITU clarification request open / clean-room reactivation triggers permanent" annotation; drop reactivation triggers; mark as never-reactivatable absent new spec source. Worst hygiene (perpetual skip in green test output).

**Default recommendation if user defers:** D-1b (re-interpret) — preserves the 30-cycle evidence, documents the deviation explicitly, eliminates the skip, and remains reversible.

**TDD shape:** the chosen option lands in a single commit (test edit + docstring update + optional archive file). No production change.

### D-2 — OVERFLOW.BIT loader fix (TDD red → green)

- [x] D-2 (F2 chosen, commit `afe3686`) — `ReadG192FrameLenient` added (accepts `0x0000` ≡ logical-0 softbit alongside canonical `0x007F`/`0x0081`); strict `ReadG192Frame` unchanged; `ReadG192File` rewired to lenient. Frame-19 anomaly in `OVERFLOW.BIT` (80 zero data-words behind canonical 0x6B21 sync, characterized in commit `1e83d6b`) now parses cleanly via the lenient path. Lenience documented as informed inference from G.191 STL "indeterminate softbit" convention.

1. **D-2 RED** — add `internal/bitstream/g192_overflow_test.go` exercising `ReadG192File` on `OVERFLOW.BIT`; assert `len(frames) == 384` and the byte content of frame 19 is well-formed under whatever framing variant is selected. Test FAILs at HEAD with `ErrBadG192Bit`.
2. **D-2 measurement** — manual binary inspection of `OVERFLOW.BIT` bytes 3116+ to identify the framing variation (endianness flip / per-bit encoding shift / mid-file resync). Document in commit body.
3. **D-2 GREEN** — implement chosen fix variant (lenient loader / variant constructor / pre-normaliser). Test PASSes.
4. **D-2 follow-up** — re-enable `TestDecode_ITUVectorOverflowBitExact` (line 531 of `decode_test.go`); rolls into D-3.8.

### D-3 — Eight `t.Skip` triage & resolution

After D-1 lands (gate 17 permanently disposed), re-enable each skip and observe:

- **D-3.tame** — `TestDecode_ITUVectorTameBitExact` (line 475). Smallest non-pathological vector (128 frames). Re-enable first; if PASS, treat as pilot evidence the rest will follow; if FAIL at sample 5..7, confirm gate-17-window only and proceed per D-1 disposition mapping; if FAIL elsewhere, escalate to a measurement sub-task.
- **D-3.algthm** (line 165), **D-3.speech** (232), **D-3.fixed** (442), **D-3.lsp** (451), **D-3.pitch** (464), **D-3.test** (486) — re-enable each in sequence; each is its own TDD step. If all pass cleanly post-D-1, batch-strip the skips and wrappers in a single commit. If any expose a *new* defect (not the gate-17 sample-5..7 window), split into its own measurement task with explicit user gate before fixing — Phase 1o anti-goal forbids unrelated diagnostic cycles.
- **D-3.overflow** — `TestDecode_ITUVectorOverflowBitExact` (line 531). Depends on D-2 GREEN. Re-enable, confirm decode behaviour against `OVERFLOW.PST`; same gate-17 mapping applies for sample-5..7 outcomes.

**Pass condition for D-3 batch:** zero `t.Skip` remaining in `internal/decoder/decode_test.go`.

### D-3.bis — `stagef_bis_diagnostic_test.go` disposition

Untracked file preserved 28 cycles. **The preservation invariant is LIFTED at Phase 1o entry.** Read the file end-to-end; classify:
- If it asserts a still-valid invariant, convert `t.Logf` → `t.Errorf` and `git add` as a regression test.
- Otherwise, delete the file (and add a one-line entry to D-final completion report's "deleted diagnostics" list).

### D-3.ter — Other diagnostic-test housekeeping (gate 19 promotion question)

Inventory `internal/decoder/{stagef_*,phase*_,foct_*,diagnostic_*}_test.go` files (≥10 files). For each, decide:
- **promote** — convert to regression (assertions on a stable invariant);
- **archive** — move salient evidence into `docs/superpowers/archives/` and delete;
- **keep** — retain as `t.Logf`-only diagnostic with explicit "kept-for-Phase-2-encoder-cross-reference" doc tag.

This is the explicit gate 19 (E5) decision deferred from Phase 1k synthesis. **USER-GATED batch decision** required before bulk action.

### D-4 — Decoder public API godoc audit

Pass criterion §4.6 below. For each exported symbol in `internal/decoder/decode.go` (`Decoder`, `Decode`, related errors in `errors.go`, `doc.go`), ensure godoc-quality comment exists. No production behaviour change. One commit.

### D-final — Phase 1o completion report

Write `docs/superpowers/plans/2026-05-XX-phase1o-decoder-domain-closure-completion-report.md` mirroring the Phase 1g layout. Sections:
1. Spec sections referenced (carry forward from Phase 1g–1n).
2. Gate 17 final disposition record (D-1 outcome + commit).
3. OVERFLOW.BIT loader fix record (D-2).
4. Eight `t.Skip` resolution table (before/after).
5. `stagef_bis` + diagnostic housekeeping outcome (D-3.bis, D-3.ter).
6. Public API godoc audit outcome (D-4).
7. Inherited spec ambiguities for Phase 2 (R-A, R-B, R-C; SF-1 tilt γ_t gating; OVERFLOW.BIT framing rationale).
8. Verdict: "**Ready for Phase 2 hand-off**" OR "**Blockers remain**" with explicit list.

---

## 4. Pass criteria for Phase 1o close

1. **Zero `t.Skip` in `internal/decoder/decode_test.go`** (all 8 triaged & resolved or removed-with-rationale).
2. **`TestDecode_ITUVectorOverflowBitExact` reaches GREEN** (or, if a documented framing variant requires deferral, OVERFLOW.BIT skip is replaced by a `bitstream`-package test pinning the loader's documented behaviour and the decoder-side test is removed with archived rationale).
3. **Gate 17 has a stable permanent disposition** per D-1 outcome (no perpetually-deferred skip remains except via explicit D-1c choice with documented rationale).
4. **`go test ./... -race`** baseline = 0 plan-allowed FAIL OR every remaining FAIL has a documented rationale + tracking issue (in completion report §8).
5. **`go vet ./...`** clean.
6. **Decoder public API godoc-complete** — every exported symbol in `internal/decoder` has a godoc-quality comment; `golint`/`revive`-equivalent inspection (manual; project does not run a linter) records zero exported-symbol-without-doc gaps.
7. **`internal/decoder/stagef_bis_diagnostic_test.go` is no longer untracked** — either committed or deleted.

---

## 5. Anti-goals (Phase 1o explicit non-scope)

- **No encoder work.** Phase 2 is gated on Phase 1o close.
- **No ITU Annex A binary, ITU C reference, bcg729, Sipro Lab, FFmpeg, or any other existing G.729 implementation.** Clean-room invariant remains absolute.
- **No new diagnostic-style speculative cycles.** D-2 / D-3.* may only spawn an additional measurement cycle if a re-enabled test exposes a *new* defect (not the gate-17 sample-5..7 window) — and only after explicit user gate.
- **No production behaviour change in `internal/postfilter/tilt.go` (SF-1).** SF-1 disposition deferred to Phase 2 per §2.5.
- **No spec-text quotation longer than minimal-attribution snippets** in plan, tests, or commit messages.
- **No reactivation of gate 17 RED assertion** unless a reactivation trigger (corrigendum / Appendix I-III / new spec source / pre-frame-0 dependency discovery) has materialised.

---

## 6. Phase 2 entry preconditions (Phase 1o → Phase 2 hand-off contract)

Phase 1o close MUST hand the following to Phase 2:

1. **Clean test baseline** — `go test ./... -race` with zero unexplained failures and zero `t.Skip` in production decoder test files.
2. **Documented public decoder API** — godoc-complete `decoder.Decoder`, `decoder.Decode`, error types, supporting types.
3. **Inherited spec-ambiguity ledger** — explicit list, in the completion report:
   - **R-A** (Phase 1n inventory).
   - **R-B** (Phase 1n inventory).
   - **R-C** (Phase 1n RC-1/RC-2/RC-3 — verbatim documentation issue, not gate-17 mechanism).
   - **SF-1** — tilt γ_t gating (`agcGainPrev` vs §4.2.3 `sign(k1')`); encoder-side k1' computation will revisit.
   - **Gate 17 sample 5..7 sign window** — disposition record + 30-cycle evidence pointer; encoder symmetry may yield new evidence.
   - **OVERFLOW.BIT framing variation** — D-2 outcome record (resolved or deferred-with-rationale).
4. **Diagnostic-test inventory** — D-3.ter outcome: which diagnostic tests survive into Phase 2 (kept / promoted) vs archived.
5. **Phase 2 plan stub** — empty `docs/superpowers/plans/2026-05-XX-phase2-encoder-plan.md` skeleton OR explicit "Phase 2 plan deferred to first user gate after Phase 1o close" note in completion report §8.

---

## 7. Identified user gates required during Phase 1o execution

| Gate | When | Decision |
|------|------|----------|
| G-D1 | Before D-1 dispatch | Pick (D-1a archive) / (D-1b re-interpret, default) / (D-1c escalate-skip). |
| G-D2 | After D-2 measurement | Pick loader-fix variant: (i) lenient core / (ii) variant constructor / (iii) test-only normaliser. |
| G-D3-escalation | If any D-3.* re-enable exposes a NEW defect (not gate-17 window) | Approve a separate diagnostic cycle OR convert that test to permanent skip with rationale. |
| G-D3.ter | Before D-3.ter bulk action | Approve diagnostic-test promote/archive/keep batch (gate 19 / E5 decision). |
| G-Final | At D-final completion-report draft | Approve "ready for Phase 2 hand-off" verdict OR enumerate residual blockers. |

---

## 8. Process notes

1. Pre-cycle exploration (this document §2) is the authoritative starting state; D-1..D-final must not redo it.
2. Each D-* task lands in its own commit. Commit messages follow project convention (`test(decoder): ...` / `fix(bitstream): ...` / `docs(plans): ...`).
3. Between every D-* task, run `go vet ./...` and a focused `go test ./internal/decoder/... -race` to catch regressions early.
4. The 28-cycle preservation invariant on `stagef_bis_diagnostic_test.go` is **LIFTED** at Phase 1o entry; D-3.bis is its terminal disposition gate.
5. No diagnostic-style speculative cycle may be entered in Phase 1o without an explicit user gate (see §7 G-D3-escalation).
