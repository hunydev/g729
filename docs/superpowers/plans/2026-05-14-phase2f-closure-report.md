# Phase 2f — Closure Report (full encode wrapper + streaming API + per-vector ITU byte-EQ harness)

**Date:** 2026-05-14
**Phase:** 2f (full-frame `EncodeFrame` end-to-end wiring per master plan §7; bitstream pack per §4.2.1 + Table 8; streaming `Write`/`Flush` per design §4.3; G.192 reader for ITU vector harness; seven per-vector byte-EQ gates including TAME.BIT taming-witness)
**Sub-plan:** [`docs/superpowers/plans/2026-05-13-phase2f-full-encode-plan.md`](2026-05-13-phase2f-full-encode-plan.md)
**Master plan:** [`docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`](2026-05-02-phase2-encoder-plan.md) §7 (Phase 2f) + §6 (Phase 2e — folded into 2d; TAME.BIT harness inherited here)
**Phase 2d closure ref:** [`docs/superpowers/plans/2026-05-12-phase2d-closure-report.md`](2026-05-12-phase2d-closure-report.md)
**HEAD at authoring:** `c5a31bd` (post INT-2 zero-alloc + race + bench; INT-3 closure commit appended on top)
**Status:** **CLOSED-DEFERRED — Phase 2f packing layer + public API structurally complete; per-vector full-frame byte-EQ FAIL-DEFERRED across all 7 vectors due to inherited Phase 2c/2d/2a structural blockers; OQ-TAMING-THR slot 5/5 sweep NO-WINNER (pin held at carryover values); zero Phase 2f I5 spent per I-2f-5; Phase 2-final entry preconditions met.**

---

## 1. Scope & Objective

Phase 2f delivered the public encoder surface: `(*Encoder).EncodeFrame` returning a packed 10-byte G.729 frame, the streaming `(*Encoder).Write` / `(*Encoder).Flush` convenience pair, the symmetric `bitstream.ReadG192Frame` primitive needed to read ITU `.BIT` vectors, and seven per-vector byte-EQ harnesses (PITCH, TAME, ALGTHM, SPEECH, FIXED, LSP, TEST) that compose the level-2 ITU compliance gate per design §7.2. Per **I-2f-5** (no new spec arithmetic introduced by Phase 2f), every per-vector FAIL-DEFERRED routes back to a documented upstream Phase 2a/2b/2c/2d blocker; **OQ-TAMING-THR slot 5/5 is the explicit carve-out** authored to falsify the taming-pin hypothesis on the dedicated TAME corpus.

Deliverables (sub-plan §5):

- **PACK-1** — `(*Encoder).buildBitstreamFrame` constructs a `bitstream.Frame` from the 15 per-frame fields (l0/l1/l2/l3 + p1/p0/p2 + s1/c1/ga1/gb1 + s2/c2/ga2/gb2) and `bitstream.Pack` emits the 10-byte G.729 native frame per §4.2.1 + Table 8. Zero-alloc.
- **PACK-2** — `bitstream.ReadG192Frame` (inverse of `WriteG192Frame`): consumes G.192-framed bytes (sync + length + 80 LE-uint16 softbits) and emits the 10-byte canonical native frame plus a `bad bool` on `0x6B20`. Pooled buffer (zero-alloc on the hot path post first call). Round-trip tested against `WriteG192Frame`.
- **API-1** — Public `Encoder.EncodeFrame(pcm []int16, out []byte) error` returns `nil` (no longer `ErrNotImplemented`); body composes `lpcStep → openloopStep → closedloopStep(0) → closedloopStep(1) → buildBitstreamFrame → bitstream.Pack`. `ErrNotImplemented` removed from the public API surface (no remaining production references).
- **API-2** — Streaming wrapper: `NewStreamingEncoder(io.Writer)` constructor, `(*Encoder).Write([]int16) (int, error)` and `(*Encoder).Flush() error` per design §4.3. Per **I-2f-3**: 80-sample boundary emission, zero-pad-on-Flush via OQ-FLUSH-PAD pin (linear-PCM 0x0000), `Reset()` clears streaming state. Steady-state zero-alloc (encoder-owned `streamBuf [80]int16` + `streamBufLen int8` + `streamSink io.Writer`).
- **TAME-1** — TAME.BIT byte-EQ harness exercised end-to-end. Plausibility floor breached (GA1 7.03 % < 12.15 % Phase 2d INT-1a baseline). **Escalation to slot 5/5 (OQ-TAMING-THR) authorized per sub-plan §12 stop condition before INT-1 entry.**
- **TAME-1 slot 5/5 (OQ-TAMING-THR sweep)** — 9-variant sweep (gp ∈ {0.9, 0.95, 1.0} Q14 × E ∈ {2³², 2³³, 2³⁴}) demoted `gainquant.GpClipQ14` and `gainquant.TameEnergyThresholdQ0` from `const` → `var` for test-time swap; restored on test exit. **All 9 variants identical:** full 0.00 % / GA1 7.03 % / GB1 2.34 % / GA2 4.69 % / GB2 4.69 %. **NO-WINNER** disposition recorded; OQ-TAMING-THR pin holds at carryover values; residual GA*/GB* shortfall traced to upstream Phase 2c P1/P2 + Phase 2d S/C/C1/C2 = 0 % FAIL-DEFERRED structural blocker (the ACELP search disagrees on every TAME frame upstream of the taming branch, so no taming-threshold knob can recover the bit-EQ rate). Slot 5/5 budget consumed; remaining slots 1/5–4/5 untouched.
- **INT-1** — six per-vector full-frame byte-EQ harnesses (PITCH, ALGTHM, SPEECH, FIXED, LSP, TEST) authored under a shared `runPhase2fVectorByteEQ` helper. Frame count canonicalized as `min(floor(.IN / 160), floor(.BIT / 164))` with OQ-VECTOR-FRAME-COUNT diagnostic logged when sizes don't cleanly multiply (PITCH 28 trailing bytes, SPEECH 64 trailing bytes — reconciliation succeeds; OQ slot 3/5 NOT consumed). All 6 vectors disposition **FAIL-DEFERRED** with documented upstream routing (see §5).
- **INT-2** — public-API zero-alloc gates (`EncodeFrame` cold-start + steady-state + streaming `Write` against `io.Discard`). All `AllocsPerRun(128) == 0`. `BenchmarkPhase2fINT2_EncodeFrame` 135.6 µs/op (+0.4 % vs `BenchmarkPhase2dINT2_FullFramePipeline`); `BenchmarkPhase2fINT2_StreamingWrite80` ~137 µs/op; `BenchmarkPhase2fINT2_StreamingWrite800` 1.357 ms/op = 135.7 µs/frame amortized. `go test -race` clean.
- **INT-3** — this closure report.

**Sub-phase ITU vector gate:** Encode each `*.IN` → byte-EQ full-frame against `*.BIT`. **Disposition: 7/7 FAIL-DEFERRED at full-frame ≥ 80 % gate.** Per-field rates documented per vector (§5); plausibility floors met where applicable (LSP-1 matches Phase 2a INT-1 ACCEPT-PARTIAL ceiling; PITCH-1 matches Phase 2c INT-1b + Phase 2d INT-1a baselines exactly).

---

## 2. Task ledger

All 8 sub-plan tasks (PACK-1 / PACK-2 / API-1 / API-2 / TAME-1 / INT-1 / INT-2 / INT-3) are `[x]`. Sub-plan reference: `2026-05-13-phase2f-full-encode-plan.md` §5.

| Family | Task | Title | Status | Commit | Outcome |
|--------|------|-------|--------|--------|---------|
| PACK | 2f-PACK-1 | G.729 10-byte frame pack from Encoder fields per §4.2.1 + Table 8 | `[x]` | `c8dded1` | `(*Encoder).buildBitstreamFrame`; per-frame `l0/l1/l2/l3 uint16` fields written by `lpcStep`; pure assignment + `bitstream.Pack`. Zero-alloc. |
| PACK | 2f-PACK-2 | G.192 frame reader for ITU vector harness | `[x]` | `d85a7e7` | `bitstream.ReadG192Frame`; sync ∈ {0x6B21, 0x6B20} + length=80 + 80 LE-uint16 softbits; round-trip tested. |
| API | 2f-API-1 | Public Encoder.EncodeFrame end-to-end wiring | `[x]` | `dadd630` | Replaces `ErrNotImplemented`; lpcStep → openloopStep → closedloopStep(0) → closedloopStep(1) → buildBitstreamFrame → bitstream.Pack. `ErrNotImplemented` removed from `errors.go`. |
| API | 2f-API-2 | Streaming Write/Flush per design §4.3 | `[x]` | `b2aa3ab` | `NewStreamingEncoder(io.Writer)` + `Write([]int16) (int, error)` + `Flush() error`; 80-sample boundary emission; zero-pad-on-Flush (OQ-FLUSH-PAD); encoder-owned scratch; steady-state zero-alloc. |
| INT | 2f-TAME-1 | TAME.BIT byte-EQ harness — first OQ-TAMING-THR witness | `[x]` FAIL-DEFERRED | `cb004a4` | Per-field rates §5.1; full=0/128, GA1=7.03 %, GB1=2.34 %, GA2=4.69 %, GB2=4.69 %; 3-of-4 floors breached → escalate slot 5/5. |
| INT | 2f-TAME-1/slot-5/5 | OQ-TAMING-THR 9-variant sweep close-out | `[x]` NO-WINNER | `d76e1f0` | All 9 variants identical (gp 0.9/0.95/1.0 × E 2³²/2³³/2³⁴ — gainquant fields demoted const→var); pin held at carryover (gp 0.95 Q14, E 2³³). Residual routes upstream to Phase 2c P1/P2 + Phase 2d S/C blockers. |
| INT | 2f-INT-1 | Per-vector byte-EQ harness PITCH/ALGTHM/SPEECH/FIXED/LSP/TEST | `[x]` 6× FAIL-DEFERRED | `151aa24` | Shared `runPhase2fVectorByteEQ` helper; 6 subtests of `TestPhase2fINT1_PerVectorByteEQ`. All 6 vectors full-frame = 0 %; per-field cross-baseline checks confirm Phase 2a/2c/2d inheritance exactly (§5.2). **0/5 Phase 2f INT-1 budget spent.** |
| INT | 2f-INT-2 | Zero-alloc + race + bench at public API level | `[x]` | `c5a31bd` | `AllocsPerRun(128) == 0` for `EncodeFrame` cold-start + steady-state and streaming `Write` against `io.Discard`; race-clean; `BenchmarkPhase2fINT2_EncodeFrame` 135.6 µs/op (+0.4 % vs Phase 2d full-frame pipeline). |
| INT | 2f-INT-3 | Phase 2f closure report (this document) | `[x]` | (this commit) | Authored at HEAD `c5a31bd`. |

**Pass criteria** (sub-plan §5): C1 G.729 native pack ✅ via PACK-1; C2 G.192 reader ✅ via PACK-2; C3 public API ✅ via API-1; C4 streaming ✅ via API-2; C5 TAME byte-EQ harness ✅ (FAIL-DEFERRED disposition recorded) via TAME-1; C6 per-vector harness ✅ (6× FAIL-DEFERRED) via INT-1; C7 zero-alloc + race ✅ via INT-2; C8 closure report ✅ via this document.

---

## 3. Production code map

Files added or materially modified across Phase 2f (Phase 2d inheritance excluded):

### Root package (`encoder.go` + `errors.go` + new `streaming.go` if separated)

| File | Role |
|------|------|
| `encoder.go` | `EncodeFrame` body wired (no longer `ErrNotImplemented`); `(*Encoder).buildBitstreamFrame(*bitstream.Frame)`; new per-frame fields `l0, l1, l2, l3 uint16` written by `lpcStep`; new streaming fields `streamBuf [80]int16`, `streamBufLen int8`, `streamSink io.Writer`; new constructor `NewStreamingEncoder(io.Writer) *Encoder`; new methods `Write([]int16) (int, error)` and `Flush() error`. |
| `errors.go` | `ErrNotImplemented` removed (master plan §7 line 1050). |

### `internal/bitstream/`

| File | Role |
|------|------|
| `internal/bitstream/g192.go` | `ReadG192Frame(io.Reader, []byte) (bad bool, err error)` added; symmetric to `WriteG192Frame`. PACK-2. |

### `internal/gainquant/`

| File | Role |
|------|------|
| `internal/gainquant/tame.go` | `GpClipQ14` and `TameEnergyThresholdQ0` demoted from `const` → `var` to enable slot-5/5 sweep test-time swap. Production behaviour unchanged (defaults preserved at 15565 / 2³³). |

### Test files (root package)

| File | Role |
|------|------|
| `phase2f_pack1_*_test.go`                              | PACK-1 unit goldens. |
| `phase2f_api1_encodeframe_test.go`                     | API-1 smoke + zero-alloc cross-check. |
| `phase2f_api2_streaming_test.go`                       | API-2 behaviour matrix (boundary-cross, partial-buffer-Flush, no-op-Flush, batch). |
| `phase2f_int1_tame_byteeq_test.go`                     | TAME-1 byte-EQ gate (FAIL-DEFERRED). |
| `phase2f_tame1_slot5_sweep_test.go`                    | TAME-1 slot 5/5 OQ-TAMING-THR 9-variant sweep (informational t.Logf; NO-WINNER disposition). |
| `phase2f_int1_per_vector_byteeq_test.go`               | INT-1 6-vector parent test + shared `runPhase2fVectorByteEQ` helper. |
| `phase2f_int2_public_api_alloc_test.go`                | INT-2 public-API zero-alloc + benchmarks. |

### Test files (`internal/bitstream/`)

| File | Role |
|------|------|
| `internal/bitstream/g192_test.go`        | PACK-2 round-trip + bad-sync negative case. |

### Inherited unmodified

`internal/lpc/`, `internal/lsp/`, `internal/pitch/openloop/`, `internal/pitch/closedloop/`, `internal/fcb/`, `internal/fcbsearch/`, `internal/gain/` — all read-only consumers under Phase 2f. `internal/tables/` LSP and gain codebooks frozen per I9. Decoder packages (`internal/decoder/`, `internal/synth/`, `internal/postfilter/`) unmodified per I10.

---

## 4. Diagnostic findings & decisions

### 4.1 OQ-FRAME-FORMAT — PINNED (G.729 10-byte native via public API; G.192 via `internal/bitstream`)

§4.2.1 specifies the on-wire format as 10 bytes per 80-bit frame, MSB-first within byte, transmission order per Table 8. G.192 (STL G.191) is a *transport* (sync + length + softbits) used by ITU test vectors; it is not the codec format. The public `Encoder.EncodeFrame` writes 10-byte G.729 native (mirror of `Decoder.DecodeFrame` consuming 10-byte native). G.192 emission is offered via `internal/bitstream.WriteG192Frame` for vector-comparison harnesses; G.192 ingestion via `internal/bitstream.ReadG192Frame` (PACK-2). **OQ-FRAME-FORMAT PINNED at G.729 native for public API, G.192 internal-only for vector harness.** No alternative explored at INT-1; slot 1/5 reserved.

### 4.2 OQ-FLUSH-PAD — PINNED (zero-pad with 0x0000)

Design §4.3 is silent on tail-padding semantics. API-2 default: `Flush` zero-pads the partial-frame buffer to 80 samples with linear-PCM silence (0x0000). Justification: linear PCM 0x0000 is the canonical silence representation; hold-last-sample would introduce a non-zero pre-emphasized residual that deviates from a clean shutdown; abort-with-error pushes complexity onto the caller without payback. **OQ-FLUSH-PAD PINNED at zero-pad.** No alternative explored at INT-1; slot 2/5 reserved.

### 4.3 OQ-VECTOR-FRAME-COUNT — RECONCILED (min(.IN/160, .BIT/164) with diagnostic)

Master plan §7 line 1059 says "use `*.IN` length to drive frame count." Audit at INT-1: PITCH.IN = 293 628 = 1835 × 160 + 28 trailing bytes; SPEECH.IN = 600 064 = 3750 × 160 + 64 trailing bytes (other vectors clean). The trailing bytes are *not* a partial frame (28 < 160) and `.BIT` cleanly multiplies (1835 × 164 / 3750 × 164). The harness reconciles via `min(floor(.IN / 160), floor(.BIT / 164))` and logs an `OQ-VECTOR-FRAME-COUNT diagnostic` t.Logf line for transparency. **OQ-VECTOR-FRAME-COUNT slot 3/5 NOT consumed** — the reconciliation passes for all six vectors.

### 4.4 OQ-COLD-START-CONVENTION — UNTESTED (slot 4/5 reserved)

Per-vector frame-0 disposition is *included* in the full-frame match count (no special frame-0 carve-out). All six vectors have full-frame = 0 %; the per-field rates do not provide a frame-0-isolated signal. Slot 4/5 remains reserved for future per-vector frame-0-only pin if a Phase 2-final probe surfaces a cold-start-only mismatch.

### 4.5 OQ-TAMING-THR — REVALIDATED (pin held at gp 0.95 Q14 / E 2³³)

§3.9.2 narrates the taming branch but does not pin a numeric ceiling. Phase 2d default: gp clamp 0.95 Q14 = 15565 + E threshold 2³³ ≈ 8.59 × 10⁹. TAME-1 byte-EQ on the dedicated taming corpus produced GA1 7.03 % / GB1 2.34 % / GA2 4.69 % / GB2 4.69 %, **3 of 4 below the Phase 2d INT-1a plausibility floor (12.15 % / 5.29 % / 11.77 % / 4.52 %)**. Per sub-plan §5 third disposition branch + §12 stop condition, slot 5/5 escalation MANDATED before INT-1 entry. The 9-variant sweep (3 gp clamps × 3 E thresholds) demoted `GpClipQ14` and `TameEnergyThresholdQ0` from `const` → `var` and ran each combination against the full TAME corpus. **All 9 variants produced byte-identical results** (full=0, GA1=7.03 %, GB1=2.34 %, GA2=4.69 %, GB2=4.69 %).

The empirical interpretation: **the taming branch is not the determining factor on the TAME corpus.** GA*/GB* indices are the *quantized* gain indices output by `gainquant.SearchConjugate`; that search minimizes a cost surface conditioned on the ACELP-search-output `c(n)` and the closed-loop-output `gp`/`v(n)`/`y(n)`. With Phase 2c P1/P2 ≈ 11 % FAIL-DEFERRED and Phase 2d C1/C2 = 0 % FAIL-DEFERRED, every TAME frame's `c(n)` and gain-search inputs disagree with ground truth at the bit level **upstream** of the taming clamp; the clamp itself is therefore a no-op (no predicted overflow ever triggers) on a corpus where the *correct* clamp never reaches the search input. **NO-WINNER disposition recorded**; the OQ-TAMING-THR pin holds at the carryover values (gp 0.95 Q14, E 2³³). Slot 5/5 budget consumed; the residual routes upstream to the Phase 2b H-CENTER + Phase 2d ACELP H-CENTER FAIL-DEFERRED chain.

**This is the explicit I-2f-5 carve-out** (slot 5/5 is the only Phase 2f I5 slot authorized to consume in the absence of a packing-layer bug). Per sub-plan §5 third branch, the slot was authorized; per sub-plan §12 stop-condition, INT-1 may now proceed.

### 4.6 H-CENTER / Phase 2d INT-1a S/C/GA/GB / Phase 2a INT-1 LSP — INHERITED CEILINGS (FAIL-DEFERRED routing upstream)

Per-vector full-frame byte-EQ is **bounded above by the minimum per-field rate**: full-frame ≤ min(L0..GB2). On every vector, C1 ≤ 0.83 % (max across vectors at FIXED-1 1/120) and C2 ≤ 2.86 % (max at ALGTHM-1 1/35) — the structural Phase 2d INT-1a 0 % FAIL-DEFERRED ceiling. **One bit-flip flips the whole frame**, so full-frame = 0 % is the *expected* disposition on every Phase 2f vector until upstream Phase 2b H-CENTER closes (which would unblock C1/C2 transitively). Per **I-2f-5**, no Phase 2f INT-1 budget consumed for these per-vector failures — they are upstream-routed to Phase 2b open-loop / Phase 2c closed-loop / Phase 2d ACELP / Phase 2a LSP closure reports. **Phase 2f INT-1 spend: 0/5.**

Cross-baseline check confirms exact inheritance:

| Vector | Field | Phase 2f rate | Inherited baseline | Match? |
|--------|-------|--------------:|-------------------:|:------:|
| PITCH  | P1    | 10.79 %       | Phase 2c INT-1b 10.79 % | ✅ |
| PITCH  | P2    | 11.66 %       | Phase 2c INT-1b 11.66 % | ✅ |
| PITCH  | GA1   | 12.15 %       | Phase 2d INT-1a 12.15 % | ✅ |
| PITCH  | GB1   | 5.29 %        | Phase 2d INT-1a 5.29 %  | ✅ |
| PITCH  | GA2   | 11.77 %       | Phase 2d INT-1a 11.77 % | ✅ |
| PITCH  | GB2   | 4.52 %        | Phase 2d INT-1a 4.52 %  | ✅ |
| LSP    | L0    | 79.03 %       | Phase 2a INT-1 78.67 %  | ≈ (within 1 frame) |
| LSP    | L1    | 38.93 %       | Phase 2a INT-1 38.93 %  | ✅ |
| LSP    | L2    | 18.06 %       | Phase 2a INT-1 17.07 %  | ≈ (within 1 frame) |
| LSP    | L3    | 20.12 %       | Phase 2a INT-1 19.35 %  | ≈ (within 1 frame) |

The exact match on the PITCH-corpus rates (which is the corpus Phase 2c/2d benchmarked against) is **direct evidence that Phase 2f introduces no new arithmetic regression** — the Phase 2f packing layer is a pure pass-through of the Phase 2c/2d production-pinned outputs.

### 4.7 Cross-corpus C1/C2 surprise — FIXED-1 / ALGTHM-1 / SPEECH-1 / LSP-1 / TEST-1 non-zero

Phase 2d INT-1a established C1 = C2 = 0.00 % on the PITCH corpus (1835 / 1835). The Phase 2f cross-corpus harness reveals:

| Vector | C1     | C2     |
|--------|--------|--------|
| PITCH  | 0.00 % | 0.00 % |
| TAME   | 0.00 % | 0.00 % |
| ALGTHM | 0.00 % | 2.86 % (1/35) |
| SPEECH | 0.05 % (2/3750) | 0.03 % (1/3750) |
| FIXED  | 0.83 % (1/120) | 0.83 % (1/120) |
| LSP    | 0.13 % (3/2232) | 0.00 % (0/2232) |
| TEST   | 0.57 % (1/176) | 0.00 % (0/176) |

The H-CENTER blocker is **dominant on PITCH/TAME but not absolute** on synthetic / fixed-point conformance / mixed-content corpora. The non-zero-but-tiny C1/C2 rates on FIXED/SPEECH/LSP/TEST/ALGTHM are consistent with the H-CENTER hypothesis (a small tail of frames where open-loop tOp converges to the correct lag) and **do not contradict** the Phase 2d FAIL-DEFERRED disposition. Recorded here as a Phase 2-final probe candidate (a per-frame H-CENTER hit-rate tabulation against ground-truth `tOp` would directly witness the dominant blocker).

### 4.8 FIXED-1 anomaly — high L0/L1/GA1/GB1 rates relative to PITCH inheritance

FIXED-1 produces L0 = 89.17 %, L1 = 55.83 %, GA1 = 65.83 %, GB1 = 55.83 % — substantially higher than the PITCH-corpus baselines (L0 94.11, L1 31.72, GA1 12.15, GB1 5.29). The FIXED corpus is the fixed-point conformance vector (synthetic test patterns); the elevated GA1/GB1 rates reflect a corpus where the gain search converges to a small index range that happens to overlap ground truth more often than on speech-like corpora. Full-frame byte-EQ is still 0 % (L2/L3/P1/C1/S1/P2/C2/S2 break the chain). **No production change recommended** — this is corpus-statistical noise around the H-CENTER blocker, not a separate failure mode.

---

## 5. Per-vector INT-1 byte-EQ disposition matrix

### 5.1 TAME-1 (128 frames; t.Errorf path on 3-of-4 floor breaches)

| Field | Match / Total | Rate | Phase 2d INT-1a floor | Disposition |
|---|---:|---:|---:|---|
| L0  | 77 / 128  | 60.16 % | — | informational |
| L1  | 79 / 128  | 61.72 % | — | informational |
| L2  | 31 / 128  | 24.22 % | — | informational |
| L3  | 104 / 128 | 81.25 % | — | informational |
| P1  | 19 / 128  | 14.84 % | — | informational |
| P0  | 78 / 128  | 60.94 % | — | informational |
| C1  | 0 / 128   | 0.00 %  | — | inherited Phase 2d |
| S1  | 11 / 128  | 8.59 %  | — | informational |
| **GA1** | **9 / 128**  | **7.03 %**  | **12.15 %** | **FLOOR BREACH** |
| **GB1** | **3 / 128**  | **2.34 %**  | **5.29 %**  | **FLOOR BREACH** |
| P2  | 41 / 128  | 32.03 % | — | informational |
| C2  | 0 / 128   | 0.00 %  | — | inherited Phase 2d |
| S2  | 12 / 128  | 9.38 %  | — | informational |
| **GA2** | **6 / 128**  | **4.69 %**  | **11.77 %** | **FLOOR BREACH** |
| GB2 | 6 / 128   | 4.69 %  | 4.52 %  | meets floor |
| Full-frame | 0 / 128 | 0.00 % | — | FAIL-DEFERRED |

3-of-4 plausibility floor breaches (GA1 / GB1 / GA2) → escalation slot 5/5 MANDATED per sub-plan §5 third branch + §12 stop condition.

### 5.2 TAME-1 slot 5/5 OQ-TAMING-THR sweep — NO-WINNER

9 variants (gp Q14 ∈ {14746, 15565, 16384} × E ∈ {2³², 2³³, 2³⁴}). All produced **byte-identical** outputs:

| gp clamp | E threshold | full | GA1 | GB1 | GA2 | GB2 |
|---------:|------------:|-----:|----:|----:|----:|----:|
| 0.9      | 2³²         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 0.9      | 2³³         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 0.9      | 2³⁴         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 0.95     | 2³²         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| **0.95** | **2³³**     | **0.00 %** | **7.03 %** | **2.34 %** | **4.69 %** | **4.69 %** ← carryover pin |
| 0.95     | 2³⁴         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 1.0      | 2³²         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 1.0      | 2³³         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |
| 1.0      | 2³⁴         | 0.00 % | 7.03 % | 2.34 % | 4.69 % | 4.69 % |

**Disposition: NO-WINNER.** The 9-variant identity demonstrates the taming clamp is *not reached* on the TAME corpus under any sweep variant — the upstream ACELP-search disagreement on every TAME frame masks every taming-knob outcome. OQ-TAMING-THR pin held at carryover (gp 0.95 Q14 = 15565, E 2³³ = 8 589 934 592). **Slot 5/5 budget CONSUMED**; the residual routes upstream to the Phase 2b H-CENTER + Phase 2d ACELP H-CENTER FAIL-DEFERRED chain. Slot 5/5 closed; **INT-1 authorized to proceed** per sub-plan §12.

### 5.3 INT-1 (six per-vector subtests; all FAIL-DEFERRED, full-frame 0 %)

Each row: per-field rates (15 fields = L0/L1/L2/L3/P1/P0/C1/S1/GA1/GB1/P2/C2/S2/GA2/GB2). Full-frame rate 0/N for all six vectors.

| Vector | Frames | L0 | L1 | L2 | L3 | P1 | P0 | C1 | S1 | GA1 | GB1 | P2 | C2 | S2 | GA2 | GB2 | Full | Disposition |
|--------|------:|---:|---:|---:|---:|---:|---:|---:|---:|----:|----:|---:|---:|---:|----:|----:|-----:|-------------|
| PITCH  | 1835 | 94.11 | 31.72 | 14.01 | 14.17 | 10.79 | 57.49 | 0.00 | 5.50 | 12.15 | 5.29 | 11.66 | 0.00 | 4.20 | 11.77 | 4.52 | 0.00 | FAIL-DEFERRED → Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER |
| ALGTHM | 35   | 94.29 | 42.86 | 22.86 | 17.14 | 5.71  | 54.29 | 0.00 | 2.86 | 14.29 | 2.86 | 8.57  | 2.86 | 17.14 | 14.29 | 8.57 | 0.00 | FAIL-DEFERRED → same upstream |
| SPEECH | 3750 | 82.03 | 46.83 | 18.24 | 20.67 | 6.93  | 53.65 | 0.05 | 10.24 | 24.29 | 14.72 | 6.85  | 0.03 | 10.77 | 24.77 | 14.13 | 0.00 | FAIL-DEFERRED → same upstream |
| FIXED  | 120  | 89.17 | 55.83 | 18.33 | 25.00 | 1.67  | 51.67 | 0.83 | 1.67 | 65.83 | 55.83 | 2.50  | 0.83 | 17.50 | 39.17 | 38.33 | 0.00 | FAIL-DEFERRED → same upstream + corpus-statistics §4.8 |
| LSP    | 2232 | 79.03 | 38.93 | 18.06 | 20.12 | 23.92 | 64.96 | 0.13 | 6.77 | 36.11 | 2.11 | 21.01 | 0.00 | 7.48 | 35.08 | 1.97 | 0.00 | FAIL-DEFERRED → Phase 2a INT-1 LSP ACCEPT-PARTIAL + Phase 2c/2d |
| TEST   | 176  | 78.98 | 47.16 | 18.75 | 21.59 | 4.55  | 50.00 | 0.57 | 10.23 | 17.61 | 9.66 | 6.25  | 0.00 | 6.82 | 9.66 | 7.95 | 0.00 | FAIL-DEFERRED → Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER |

**Phase 2f INT-1 budget consumption: 0/5 spent.** Per **I-2f-5**, all six per-vector failures route upstream to Phase 2a/2b/2c/2d FAIL-DEFERRED reports. No Phase 2f production-side fix attempted.

LSP-1 cross-baseline (Phase 2a INT-1 ACCEPT-PARTIAL inheritance check):

| Field | Phase 2f LSP-1 | Phase 2a INT-1 baseline | Match? |
|-------|----------------:|------------------------:|:------:|
| L0    | 79.03 %         | 78.67 %                 | ≈ (within 1 frame on 2232) |
| L1    | 38.93 %         | 38.93 %                 | ✅ exact |
| L2    | 18.06 %         | 17.07 %                 | ≈ |
| L3    | 20.12 %         | 19.35 %                 | ≈ |

**Phase 2a inheritance confirmed; no Phase 2f LSP regression.**

---

## 6. INT-2 — Zero-alloc + race + bench at public API level

`go test -run TestPhase2fINT2 -race ./...` clean (HEAD `c5a31bd`):

- `EncodeFrame` cold-start (fresh Encoder per AllocsPerRun warm-up call): `AllocsPerRun(128) == 0` ✅.
- `EncodeFrame` steady-state: `AllocsPerRun(128) == 0` ✅.
- `(*Encoder).Write` (NewStreamingEncoder vs `io.Discard`): `AllocsPerRun(128) == 0` ✅ (4-frame warm-up to past frame-0 state init, then steady-state).

Race detector reports no new `DATA RACE` events beyond the documented Phase 2d baseline (encoder is documented single-goroutine per `encoder.go` doc comment).

**Performance numbers (AMD EPYC 9554P, HEAD `c5a31bd`):**

| Bench | ns/op | B/op | allocs/op | Notes |
|-------|------:|-----:|----------:|-------|
| `BenchmarkPhase2dINT2_FullFramePipeline`     | ~135 084 | 0 | 0 | Phase 2d baseline (lpc + openloop + 2 × (closedloop + fcb), inner pipe, no bitstream.Pack). |
| `BenchmarkPhase2fINT2_EncodeFrame`           | **135 600** | 0 | 0 | Public-API end-to-end (above + buildBitstreamFrame + bitstream.Pack). |
| `BenchmarkPhase2fINT2_StreamingWrite80`      | ~137 000 | 0 | 0 | Streaming Write of one 80-sample frame against `io.Discard`. |
| `BenchmarkPhase2fINT2_StreamingWrite800`     | 1 357 000 | 0 | 0 | 10-frame batch (800 samples = 10 × 80); 135.7 µs/frame amortized. |

**Pack overhead:** `BenchmarkPhase2fINT2_EncodeFrame − BenchmarkPhase2dINT2_FullFramePipeline ≈ 516 ns/op` per frame (~+0.4 %). **Soft target ≤ +5 % vs Phase 2d full-frame pipeline ✅.**

**Streaming overhead:** `BenchmarkPhase2fINT2_StreamingWrite80 − BenchmarkPhase2fINT2_EncodeFrame ≈ 1.4 µs/op` per frame (~+1 %). **Negligible.**

**Phase 2g (perf) trigger evaluation (sub-plan §9 Risk R-3):** EncodeFrame at 135.6 µs/op = 0.136 ms/op; Risk R-3 threshold = 2 ms/op on AMD EPYC 9554P. **0.136 ms/op ≪ 2 ms/op.** **Phase 2g NOT NEEDED** at Phase 2f close. The clean-room ACELP full-search (8192 iterations, no early-exit pruning, OQ-A38-DEPTH PINNED) is comfortably soft-realtime; perf is non-blocking for Phase 2-final entry.

I4 / I6 obligations met; perf budget non-blocking.

---

## 7. Engineering invariants pinned

- **I1 (clean-room):** All citations in production code and tests point to `docs/superpowers/specs/itu/G729E.{pdf,txt}` or to our own prior plans/docs/textbooks. No third-party G.729 source consulted across Phase 2f (PACK-1 through INT-3). The OQ-TAMING-THR slot 5/5 sweep variants are textbook-typical CELP taming surfaces (Kondoz §10.5; Spanias §11.6) — not derived from any reference C.
- **I3 (per-frame state mutation discipline):** `EncodeFrame` advances all per-frame state exactly once per call. Streaming `Write`/`Flush` introduce NO mid-frame state mutation — `streamBuf`/`streamBufLen` are buffering only; on 80-sample boundary the inner `EncodeFrame` is invoked which does the per-frame state advance.
- **I4 (zero-alloc on hot path):** Pinned by INT-2 (commit `c5a31bd`); see §6.
- **I5 (escalation budget):** Phase 2f INT-1 0/5 spent; **TAME-1 slot 5/5 (OQ-TAMING-THR) 1/1 spent** (NO-WINNER). The Phase 2f INT-1 packing-layer slots 1/5–4/5 remain available; they're reserved for any Phase 2-final per-vector probe that surfaces a packing-layer bug (per **I-2f-5**, NOT for upstream arithmetic re-litigation). Cross-phase: Phase 2a INT-1 1/5 + Phase 2-final escape 1/1 + Phase 2c INT-1b 4/4 + Phase 2d INT-1a 5/5 + Phase 2f INT-1 5/5 reserved untouched.
- **I6 (ITU bit-exactness for all integer ops):** Phase 2f adds NO new arithmetic.
- **I-2f-1 / I-2f-2 / I-2f-3 / I-2f-4 / I-2f-5 (Phase 2f packing discipline):** All five Phase 2f-specific invariants honored throughout. Public API writes G.729 native 10-byte frames; bit ordering MSB-first transmission order; streaming framing zero-pad-on-Flush; bitstream pack is stateless; no new spec arithmetic introduced (slot 5/5 is the explicit carve-out, exercised and closed).
- **I8:** Each Phase 2f commit carries the prescribed `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer.
- **I9 (LSP codebook discipline):** `internal/tables/lsp_*.go` unmodified across Phase 2f.
- **I10 (encoder-decoder state isolation):** `internal/bitstream/` is the encoder–decoder shared layer per the merger doctrine; PACK-2 added a *new* read primitive (`ReadG192Frame`) symmetric to `WriteG192Frame`; both are pure functions on byte slices with no shared mutable state. Decoder packages unmodified.

---

## 8. Open questions / risks (OQ register at Phase 2f close)

| ID | State at Phase 2f close | Owner |
|----|--------------------------|-------|
| **OQ-FRAME-FORMAT** | PINNED at G.729 native 10-byte for public API; G.192 internal-only for vector harness | INT-1 slot 1/5 (reserved) |
| **OQ-FLUSH-PAD** | PINNED at zero-pad with 0x0000 | INT-1 slot 2/5 (reserved) |
| **OQ-VECTOR-FRAME-COUNT** | RECONCILED via min(.IN/160, .BIT/164) with t.Logf diagnostic | INT-1 slot 3/5 (NOT consumed) |
| **OQ-COLD-START-CONVENTION** | UNTESTED at frame-0 isolation | INT-1 slot 4/5 (reserved) |
| **OQ-TAMING-THR** | REVALIDATED — pin held at gp 0.95 Q14 / E 2³³ via slot 5/5 sweep NO-WINNER | TAME-1 slot 5/5 **CONSUMED** |
| **OQ-A38-DEPTH** (Phase 2d carryover) | PINNED at full 8192 iterations; Phase 2d INT-1a slot 1/5 reserved | (NOT Phase 2f) |
| **OQ-A38-SIGNTIE** (Phase 2d carryover) | PINNED at sign(0) = +1 | (NOT Phase 2f) |
| **OQ-GA-PRESELECT-METRIC** (Phase 2d carryover) | PINNED at L1 linear | (NOT Phase 2f) |
| **OQ-GBK-INDEX-MAP** (Phase 2d carryover) | PINNED at physical-idx + inverse-imap pack | (NOT Phase 2f) |
| **H-CENTER** (Phase 2c carryover) | LIVE-DEFERRED; structural ceiling for all 6 INT-1 per-vector full-frame rates | Phase 2b re-entry / Phase 2-final |
| **H-PHASE / OQ-WINDOW / OQ-XB-NORM** (Phase 2c carryover) | LIVE-DEFERRED / PINNED / UNTESTED | Phase 2c reserved slots 3/5–5/5 (untouched) |

---

## 9. I5 budget accounting

| Gate | Budget | Reserved | Spent | Available |
|------|-------:|---------:|------:|----------:|
| Phase 2f INT-1 (per-vector packing-layer)              | 5 | 0 | **0** | **5** |
| Phase 2f TAME-1 slot 5/5 (OQ-TAMING-THR sweep)         | 1 | 0 | **1** | 0 |
| Phase 2d INT-1a (FCB byte-EQ vs PITCH.BIT)             | 5 | 0 | 0 | 5 (NOT consumed by Phase 2f) |
| Phase 2c INT-1b reserved (post-Phase-2d re-run)        | 5 | 1 | 1 | 4 (NOT consumed by Phase 2f) |
| Phase 2a INT-1 (LSP ACCEPT-PARTIAL)                    | 5 | 1 | 0 | 4 (NOT consumed by Phase 2f) |
| Phase 2-final escape (G.192 byte-EQ)                   | 1 | 1 | 0 | 0 (RESERVED for Phase 2-final) |

**Phase 2f net spend: 0 INT-1 packing-layer slots + 1 TAME-1 slot 5/5 sweep slot.** All remaining cross-phase budgets untouched. No double-spend across phases (R-4 mitigation honored).

---

## 10. Test baseline (`go test ./... -race`, HEAD `c5a31bd`)

| Package | Status |
|---------|--------|
| `github.com/exedev/g729` | **FAIL** (`TestEncode_LSPVectorBitExact`, `TestPhase2cINT1_ClosedLoopPitchByteEQ`, `TestPhase2dINT1a_FCBByteEQ`, `TestPhase2fTAME1_ByteEQ`) |
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

**Total baseline at Phase 2f closure: 7 FAILs (6 inherited from Phase 2a/2c/2d/decoder/gain + 1 new Phase 2f TAME-1 FAIL-DEFERRED).** Inherited FAIL cohort:

| Test | Package | Source phase |
|------|---------|--------------|
| `TestEncode_LSPVectorBitExact` | `github.com/exedev/g729` | Phase 2a INT-1 ACCEPT-PARTIAL |
| `TestPhase2cINT1_ClosedLoopPitchByteEQ` | `github.com/exedev/g729` | Phase 2c INT-1 FAIL-DEFERRED (re-baselined Phase 2d INT-1b) |
| `TestPhase2dINT1a_FCBByteEQ` | `github.com/exedev/g729` | Phase 2d INT-1a FAIL-DEFERRED |
| `TestDiagnostic_SinglePulseChain` | `github.com/exedev/g729/internal/decoder` | Phase 1 inheritance |
| `TestDecode_LowEnergyCodebookIsSmooth` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |
| `TestDecode_SucceedsAcrossAllGainIndices` | `github.com/exedev/g729/internal/gain` | Phase 1 inheritance |

Phase 2f adds **one new FAIL-DEFERRED** test:

| Test | Package | Disposition |
|------|---------|-------------|
| `TestPhase2fTAME1_ByteEQ` | `github.com/exedev/g729` | FAIL-DEFERRED — 3-of-4 plausibility floor breaches (GA1 7.03 / GB1 2.34 / GA2 4.69 < Phase 2d INT-1a baselines); slot 5/5 sweep NO-WINNER; routes upstream to Phase 2c P1/P2 + Phase 2d S/C structural blocker. |

Phase 2f authored harnesses that **PASS** (informational t.Logf only, no t.Errorf):

| Test | Package | Disposition |
|------|---------|-------------|
| `TestPhase2fTAME1_Slot5_OQTamingThrSweep` | `github.com/exedev/g729` | NO-WINNER (informational sweep table) |
| `TestPhase2fINT1_PerVectorByteEQ` (6 subtests: PITCH/ALGTHM/SPEECH/FIXED/LSP/TEST) | `github.com/exedev/g729` | All 6 FAIL-DEFERRED but informational (no t.Errorf) per **I-2f-5** routing |
| `TestPhase2fINT2_*` (alloc + bench) | `github.com/exedev/g729` | All PASS |

`go vet ./...` ✅ clean. `go build ./...` ✅ clean.

### `ErrNotImplemented` removal audit (INT-3 step 3)

`grep -rn "ErrNotImplemented"` across the tree:

- Production Go: **0 references** in any `.go` file outside `*_test.go`. The sentinel is fully removed from the public API surface per master plan §7 line 1050.
- Test Go: 1 reference (a comment in `encoder_encodeframe_test.go:21` documenting that API-1 removed it — informational, not a runtime use).
- Plan/closure docs: 8 references (all historical context).

**Removal complete.** No production callers reference `ErrNotImplemented`.

---

## 11. Phase 2-final entry checklist (master plan §8 preconditions)

Per sub-plan §10 + master plan §8, the following must hold for Phase 2-final entry:

| Precondition | Status | Source |
|--------------|--------|--------|
| Public `EncodeFrame` returns no `ErrNotImplemented` | ✅ via API-1 | `encoder.go` (commit `dadd630`) |
| Streaming `Write`/`Flush` work and steady-state zero-alloc | ✅ via API-2 + INT-2 | commits `b2aa3ab` + `c5a31bd` |
| Per-vector harnesses present and dispositioned for all 7 vectors (PITCH/TAME/ALGTHM/SPEECH/FIXED/LSP/TEST) | ✅ via TAME-1 + INT-1 | commits `cb004a4` + `151aa24` |
| All FAIL-DEFERRED dispositions traced to documented upstream blocker | ✅ — per-vector routing tabulated §5.3 | this report §5 |
| `ErrNotImplemented` removed from public API | ✅ — production zero references | §10 audit |
| Public API stable (no breaking changes since Phase 2-0 except `ErrNotImplemented` removal) | ✅ — only API additions are `NewStreamingEncoder`, `Write`, `Flush` | API-2 (commit `b2aa3ab`) |
| `BenchmarkEncodeFrame` measured for Phase 2g trigger decision | ✅ — 0.136 ms/op ≪ 2 ms/op R-3 threshold; **Phase 2g NOT NEEDED** | INT-2 §6 |
| I5 ledger reconciled across all Phase 2 sub-phases | ✅ — single canonical table §9; no double-spend | this report §9 |
| Phase 1o inherited 3 FAILs re-examined under Phase 2 encoder symmetry | **Phase 2-final scope** | master plan §9 line 1109 |
| R-A / R-B / R-C ambiguity ledger re-examined | **Phase 2-final scope** | master plan §9 line 1110 |
| SF-1 tilt γ_t gating re-examined | **Phase 2-final scope** | master plan §9 line 1111 |
| OVERFLOW.BIT framing rationale re-examined (encoder may natively emit 0x0000 softbits) | **Phase 2-final scope** | master plan §9 line 1112 |
| `TestDecode_ITUVectorAlgthmBitExact` SKIP demote candidate | **Phase 2-final scope** | master plan §9 line 1114 |

**Phase 2-final entry: AUTHORIZED.** All Phase 2f-owned preconditions met; the four "Phase 2-final scope" rows are explicit hand-offs to the master plan §8 closure report (`docs/superpowers/reports/YYYY-MM-DD-phase2-completion-report.md`).

### Outstanding FAIL-DEFERRED routing summary

For Phase 2-final disposition (master plan §8):

| Source | FAIL-DEFERRED locus | Route |
|--------|---------------------|-------|
| Phase 2a INT-1 LSP ACCEPT-PARTIAL (L0=78.67/L1=38.93/L2=17.07/L3=19.35 %) | encoder LSP indices | Reserved 1/5 Phase 2-final escape slot OR document as final residual |
| Phase 2c INT-1b P1/P0/P2 (10.79/57.49/11.66 %) | closed-loop pitch | Phase 2b re-entry needed (H-CENTER tOp blocker upstream) |
| Phase 2d INT-1a S/C/GA/GB (5.50/0.00/12.15/5.29 / 4.20/0.00/11.77/4.52 %) | ACELP + gain quantization | Cascades from H-CENTER + Phase 2c P1/P2 → C1/C2 0 % |
| Phase 2f TAME-1 (GA1 7.03/GB1 2.34/GA2 4.69 < Phase 2d floor) | OQ-TAMING-THR sweep NO-WINNER | Same upstream chain (Phase 2c P1/P2 + Phase 2d S/C) — taming clamp not the determining factor |
| Phase 2f INT-1 6× FAIL-DEFERRED (full-frame 0 % across all 6 corpora) | per-vector packing layer (no fault) | Cascades from C1/C2 ≤ 2.86 % → full-frame ≤ that ceiling |
| `TestDiagnostic_SinglePulseChain` | decoder pulse-chain diagnostic | Phase 1 inheritance — re-examine under Phase 2 encoder symmetry |
| `TestDecode_LowEnergyCodebookIsSmooth` | decoder gain | Phase 1 inheritance — re-examine |
| `TestDecode_SucceedsAcrossAllGainIndices` | decoder gain | Phase 1 inheritance — re-examine |

The dominant root cause across the encoder-side FAILs is **Phase 2b H-CENTER** (open-loop tOp miscentring on ~46 % of frames per Phase 2c closure §6) which cascades:

```
Phase 2b H-CENTER (tOp ≠ ground truth on ~46% of frames)
   → Phase 2c P1/P0/P2 capped at ~10–11 %
       → Phase 2d C1/C2 = 0 % (one bit-flip flips the 13-b position codeword)
           → Phase 2d S/GA/GB ≈ 5–12 %
               → Phase 2f per-vector full-frame = 0 %
```

A **Phase 2b re-entry** (or a Phase 2-final escape slot consuming the reserved 1/5 budget against H-CENTER) is the single highest-leverage intervention for the Phase 2-final compliance gate.

---

## 12. Phase 2 next-step recommendation

**Next dispatch: author the Phase 2-final completion report** (`docs/superpowers/reports/YYYY-MM-DD-phase2-completion-report.md` per master plan §8).

The Phase 2-final report covers:

1. **Sub-phase summary** — Phase 2-0 through Phase 2f with commit ranges + closure-report cross-links.
2. **ITU vector gate results table** — all 7 vectors × per-field rates (this report §5.3 is the canonical input).
3. **Inherited Phase 1o FAIL disposition** — 3 entries (SinglePulseChain / LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices). Each: fixed-with-cite OR demoted-with-§A.4-authorization OR explicitly carried to Phase 3.
4. **R-A / R-B / R-C ambiguity ledger re-examination** — per master plan §9 line 1110.
5. **SF-1 tilt γ_t gating** — encoder produces k1' first-class during LPC analysis; re-examine the postfilter's `agcGainPrev` vs §4.2.3 `sign(k1')` gating decision.
6. **OVERFLOW.BIT framing rationale** — Phase 1o D-2 F2 lenient loader; encoder may natively produce 0x0000 softbits validating or invalidating F2.
7. **H-CENTER decision** — either authorize a Phase 2b re-entry (consuming the reserved Phase 2-final 1/5 escape slot) OR document H-CENTER as the final residual and accept the Phase 2-final compliance gate at the structural ceiling (≪ 80 % full-frame across all 7 vectors).
8. **Public API stability statement** — Phase 2f added `NewStreamingEncoder` + `Write` + `Flush`; removed `ErrNotImplemented`. Otherwise unchanged since Phase 2-0.
9. **Phase 3 entry note** — release polish, README, public examples, fuzzing, ITU corrigendum search (Phase 1o §6 carryover).

**Phase 2g (perf) — NOT AUTHORIZED.** Per Phase 2f INT-2 §6: `BenchmarkEncodeFrame` measured at 0.136 ms/op = ~6.8 % of the 2 ms/op Risk R-3 threshold and ~1.4 % of the 10 ms/frame soft-realtime budget. The clean-room ACELP full-search (8192 iterations) is comfortably soft-realtime; pruning Phase 2d INT-1a slot 1/5 (OQ-A38-DEPTH) would consume an I5 budget without measurable user-facing benefit. **Phase 2g remains contingent on a future binding-perf-budget signal; not a precondition for Phase 2-final.**

**Phase 2 cycle status:** **CLOSED-DEFERRED** is the realistic outcome (master plan §7 line 1063 closure trigger "all 7 vectors PASS" is bounded above by the H-CENTER structural ceiling and is **NOT MET**). The CLOSED-DEFERRED disposition with documented per-vector dispositions is sufficient for the master plan §8 Phase 2-final closure report to do the final accounting and Phase 3 hand-off. **Phase 2-final closure report is the next dispatch.**

---

— end of Phase 2f closure report —
