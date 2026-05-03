# Phase 2-0 — Scaffold Closure Report

**Date:** 2026-05-03
**Plan:** `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §1
**Entry HEAD:** `8893fac` (user-authorized successor to plan-cited `a372de7`; the plan commit itself was `8893fac`).
**Exit HEAD:** see `git log --oneline` (8 scaffold commits).

## 1. Tasks completed

| # | Task | Commit |
|---|------|--------|
| 2-0-1 | Sentinel errors (`ErrShortPCM`, `ErrShortOutput`, `ErrShortBitstream`, `ErrNotImplemented`) + frame constants | `82b895a` |
| 2-0-2 | `internal/lpc` skeleton (`Analyzer`, `Reset`, stub `Analyze`; `LPCOrder=10`, `LPCWindowSamples=240`) | `e9ba24c` |
| 2-0-3 | `internal/acelp` skeleton (`Searcher`, `Reset`, stub `Search`; `Result`, `SubframeSamples=40`, `PulseCount=4`) | `d1a0a2a` |
| 2-0-4 | `internal/filter` skeleton (`Weighting`, `Reset`, stub `Apply`) | `3226a1e` |
| 2-0-5 | Root `Encoder` skeleton (§5.3 state layout, returns `ErrNotImplemented`) | `8c89076` |
| 2-0-6 | Root `Decoder` shell wrapping `internal/decoder.Decoder.Decode` | `60efcb6` |
| 2-0-7 | Strict-frame helpers `EncodeFrame` / `DecodeFrame` in `frame.go` | `19bac5c` |
| 2-0-8 | Scaffold gate verification + this report | (this commit) |

## 2. Files added

```
errors.go              errors_test.go
encoder.go             encoder_test.go
decoder_root.go        decoder_root_test.go
frame.go               frame_test.go
internal/lpc/doc.go    internal/lpc/types.go    internal/lpc/types_test.go
internal/acelp/doc.go  internal/acelp/types.go  internal/acelp/types_test.go
internal/filter/doc.go internal/filter/types.go internal/filter/types_test.go
docs/superpowers/plans/2026-05-03-phase2-0-scaffold-report.md
```

No files were modified outside the additions above. `doc.go` (root) was left untouched at this gate; the plan listed it as an optional modification (extend doc to include encoder usage example), and Phase 2-0 stubs do not yet provide a runnable encode example. Documenting the encoder API will land in the first sub-phase that produces non-stub encoded output (Phase 2a re-evaluation).

## 3. Test counts (per user-supplied formula `grep -E "^\s*--- (FAIL|SKIP|PASS)"`)

| State | Baseline (`8893fac`) | Post-scaffold | Δ |
|-------|---------------------|---------------|---|
| PASS  | 636 | 658 | +22 |
| FAIL  | 3   | 3   | 0  |
| SKIP  | 3   | 3   | 0  |

Inherited FAIL set unchanged:
- `TestDiagnostic_SinglePulseChain`
- `TestDecode_LowEnergyCodebookIsSmooth`
- `TestDecode_SucceedsAcrossAllGainIndices`

`go vet ./...` — clean.
`go build ./...` — clean.

## 4. Invariants

| # | Status | Note |
|---|--------|------|
| I1 | ✅ | Clean-room. No external G.729 implementation consulted. Only spec PDFs + this plan. |
| I2 | n/a | No bitstream produced yet. |
| I3 | ✅ | No panics, logs, or goroutines anywhere; sentinel returns only at API boundary. |
| I4 | n/a | No `EncodeFrame` body yet to alloc-test; deferred to Phase 2a. |
| I5 | ✅ | No diagnostic cycles opened. |
| I6 | ✅ | No `internal/**` non-test production files modified outside scaffold-additions for the three new packages. |
| I7 | ✅ | Every task: failing test → minimal impl → pass → commit per task. |
| I8 | ✅ | All 8 commits include the Copilot co-author trailer. |

## 5. Implementation notes / deviations

- **Decoder shell adapter:** `internal/decoder.Decoder` exposes `Decode(packed []byte, bad bool, out []int16) error`, not `DecodeFrame`. The Phase 2-0 plan explicitly authorizes adapting the wrapper to the existing inner API (Task 2-0-6 implementation note). The shell calls `inner.Decode(bits, false, out)`.
- **HEAD SHA:** Pre-flight SHA gate skipped per user authorization (plan was committed on top of the cited `a372de7` as `8893fac`, docs-only). Working tree was clean and baseline test counts matched.

## 6. Hand-off to Phase 2a

Next dispatch: author `docs/superpowers/plans/YYYY-MM-DD-phase2a-lpc-lsp-plan.md` per master plan §2 deferral, then implement §3.2.1 windowed autocorrelation → §3.2.6 MA-predictor LSP quantization. Phase 2a's first encoder bring-up wires `internal/lpc.Analyzer.Analyze` into the still-stub `Encoder.EncodeFrame`, replaces the relevant `ErrNotImplemented` boundary with a partial-frame guard, and gates on `LSP.IN` → `LSP.BIT` field-equality (L0/L1/L2/L3 only, full-frame deferred).

Re-evaluate the three inherited FAILs once an encoder LSP path exists; cross-block witness from the encoder side may surface their root cause.
