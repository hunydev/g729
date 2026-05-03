# Phase 2f — Full encode wrapper + streaming API + per-vector ITU byte-EQ harness (sub-plan)

- **Date opened:** 2026-05-13
- **Status:** **IN PROGRESS — opened 2026-05-13.** Phase 2d closed-deferred at HEAD `7646337`; Phase 2f opens with all upstream encoder stages production-pinned (lpcStep / openloopStep / closedloopStep / fcbStep) and `internal/bitstream.Pack` already implemented for Phase 1 decoder use. This sub-plan wires `Encoder.EncodeFrame` to a non-`ErrNotImplemented` return path, packs the 80-bit on-wire frame per Table 8 ordering, adds a streaming convenience API (`Write`/`Flush`), and lands the per-vector ITU byte-EQ harness (PITCH / TAME / ALGTHM / SPEECH / FIXED / LSP / TEST) as the Phase 2-final compliance gate.
- **Scope:** ITU-T G.729 §4.1 (bit allocation, Table 8) + §4.2.1 (transmitted frame format) + Annex A §A.4 (Annex-A bit allocation passthrough) + design spec §4.3 (streaming API) + §7.2 (level-2 ITU vector gates). No new arithmetic — all spec-arithmetic blocks (pre-process, LPC, LSP, open-loop, closed-loop, ACELP, gain) are inherited frozen from Phases 2a–2d.
- **Inputs:**
  - Master plan: `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` §7 (Phase 2f scope) + §8 (Phase 2-final closure trigger) + §6 (Phase 2e folded — TAME.BIT harness inherited here).
  - Predecessor sub-plan (template): `docs/superpowers/plans/2026-05-11-phase2d-fixed-codebook-acelp-plan.md`.
  - Predecessor closure: `docs/superpowers/plans/2026-05-12-phase2d-closure-report.md` — entry preconditions §12, OQ register §9, I5 budget §10.
  - Spec: `docs/superpowers/specs/itu/G729E.txt` §4.1 (Table 1 bit allocation, §4.1.* per-field decoders), §4.2.1 (transmitted frame format), §A.4 (Annex A passthrough). Specifically: Table 1 (lines 396–406), Table 8 (lines 1446–1464), §4.2.1 frame-byte layout.
  - Existing bitstream packing: `internal/bitstream/pack.go:8` (`Pack(*Frame, []byte) error`), `internal/bitstream/types.go:15` (`Frame` struct, Table 8 field order), `internal/bitstream/g192.go:33` (`WriteG192Frame`).
  - Existing decoder API precedent: `decoder_root.go:7` (`Decoder` shell), `frame.go:5` (`EncodeFrame`/`DecodeFrame` top-level convenience).
  - Existing encoder skeleton: `encoder.go` — Phase 2d-complete; `EncodeFrame` still returns `ErrNotImplemented` at line 187; per-frame fields `s1, s2, c1, c2, ga1, gb1, ga2, gb2` already populated by `fcbStep`; pitch fields `p1, p0, p2` populated by `closedloopStep`.
  - LSP fields (L0/L1/L2/L3): currently not retained on `Encoder` between `lpcStep` and `EncodeFrame`. **API-1 step 1** will add per-frame `l0, l1, l2, l3` fields and have `lpcStep` write them, mirroring the existing pitch / FCB / gain field pattern.
- **Output contract:** ledger-driven TDD; **seven per-vector full-frame byte-EQ gates** (one per `*.IN`/`*.BIT` pair) closing Phase 2 cycle. Closure report as INT-3.

---

## 0. Inherited invariants

### 0.1 Cross-cutting (from master plan + Phase 2d)

| ID | Invariant | Source |
|----|-----------|--------|
| I1 | **CLEAN-ROOM.** Only `docs/superpowers/specs/itu/G729E.{pdf,txt}` and our own prior plans/docs/textbooks (Kondoz, Spanias). NO ITU-T C reference, no bcg729, no Sipro, no FFmpeg. Self-attest at every commit; spec-cite every numeric constant and Table 8 field order. | Master plan §I1 |
| I3 | Per-frame state mutation discipline. `EncodeFrame` advances all per-frame state exactly once per call (`oldSpeech` slide, `freqPrev` MA-predictor commit, `lspDec` advance, `pastQuaEn` FIFO at frame end). Per-subframe state (`oldExc`, `swMemErr`, `lpResidualMemQ`, `prevGpQ14`, `prevTaming`) commits inside `closedloopStep`+`fcbStep` per subframe. Streaming `Write`/`Flush` MUST NOT introduce mid-frame state mutation. | Phase 2d §I3 |
| I4 | **Zero-alloc on hot path.** `Encoder.EncodeFrame` allocates 0 in steady state per `testing.AllocsPerRun(128, ...)`. Streaming `Write` and `Flush` allocate 0 in steady state once their internal scratch buffer is encoder-owned. INT-2 pins the assertion at the public-API level. | Master plan §I4 + Phase 2d INT-2 precedent |
| I5 | INT-1 spend budget: ≤5 escalations per integration step. Phase 2f opens with **5/5 fresh** per-vector budget (separate gate from upstream Phase 2a/2b/2c/2d INT-1 budgets — each per-vector PASS/FAIL-DEFERRED is system-level). The Phase 2a 1/5 *Phase 2-final escape* slot remains reserved for the G.192 byte-EQ end-game per `2026-05-06-phase2a-closure-report.md` §8 (NOT consumed by Phase 2f INT-1). | Phase 2a INT-1 closure §8 + Phase 2d closure §10 |
| I6 | ITU bit-exactness for all integer ops; saturating fixed-point arithmetic via `internal/fixed`. Phase 2f adds NO new arithmetic. | Master plan §I6 |
| I8 | Single squashed commit per task with prescribed message + `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` trailer. | Master plan §I8 |
| I9 | LSP codebook discipline: `internal/tables/lsp_*.go` MUST be untouched. | Phase 2a closure |
| I10 | Encoder–decoder state isolation. Phase 2f's bitstream packer reuses `internal/bitstream.Pack` (currently consumed by the decoder for round-trip tests) — this is *read-only* reuse of a pure function on a value-typed `Frame`; no shared mutable state. The `internal/bitstream` package is in the encoder–decoder shared layer per the merger doctrine. | Phase 2d §I10 |

### 0.2 Phase-2f-new

| ID | Invariant | Definition |
|----|-----------|-----------|
| **I-2f-1** | **Frame-byte ordering: G.729 native first.** Public `Encoder.EncodeFrame` writes the canonical 10-byte G.729 frame per §4.2.1 + Table 8 via `internal/bitstream.Pack`. The G.192 ITU-test framing (164-byte = 1 sync + 1 length + 80 softbits LE16) is offered separately via `internal/bitstream.WriteG192Frame` for ITU vector comparison (PACK-2). The public `Encoder.EncodeFrame` API does NOT emit G.192 — the decision rationale (G.192 is a transport, not the codec format; the public API mirrors `Decoder.DecodeFrame` which consumes 10-byte native) is pinned here as **OQ-FRAME-FORMAT** default. |
| **I-2f-2** | **Bit ordering: MSB-first within byte, transmission order across bytes.** `internal/bitstream.Pack` uses `BitWriter.Write(value, width)` which packs MSB-first into the byte stream per §4.2.1. The Table 8 field order (L0, L1, L2, L3, P1, P0, C1, S1, GA1, GB1, P2, C2, S2, GA2, GB2) is already encoded in `Pack` and asserted by Phase 1 decoder round-trip tests; PACK-1 re-asserts via the encoder side. |
| **I-2f-3** | **Streaming framing semantics.** `(*Encoder).Write` consumes int16 PCM samples, buffers up to 79 trailing samples, and emits one 10-byte frame per 80-sample boundary. `(*Encoder).Flush` zero-pads the trailing partial frame (if any) to 80 samples and emits one final frame (or no-ops if buffer empty). Callers MUST ultimately call `Flush` to drain any tail; the convention follows `compress/flate.Writer.Flush` semantics. **OQ-FLUSH-PAD** default = zero-pad with 0x0000 (silence in linear PCM). |
| **I-2f-4** | **Stateless bitstream pack.** `Encoder.EncodeFrame` reads only per-frame fields (`l0/l1/l2/l3, p1, p0, c1, s1, ga1, gb1, p2, c2, s2, ga2, gb2`) and pure-functionally constructs a `bitstream.Frame` value, then calls `bitstream.Pack`. No bitstream-level state (no rolling sync counter, no padding state) — each frame is independent. This matches the decoder's frame-stateless `Pack`/`Unpack` API. |
| **I-2f-5** | **No new spec arithmetic.** Phase 2f wires existing pinned blocks. If a per-vector byte-EQ rate misses the plausibility floor (defined per task), the disposition is FAIL-DEFERRED routed back to the *upstream* Phase 2a/2b/2c/2d sub-plan, NOT a Phase 2f production-side fix. The five Phase 2f I5 slots are reserved exclusively for **packing-layer** OQs (frame format, flush padding, sync handling, bit ordering) and NOT for upstream arithmetic re-litigation. |

### 0.3 Master-plan scope

The master plan (`2026-05-02-phase2-encoder-plan.md` §7) defines Phase 2f as: end-to-end `EncodeFrame` wiring + bitstream pack + streaming `Write`/`Flush` + per-vector byte-EQ harness. The master plan §6 (Phase 2e — folded into 2d per `2026-05-11-phase2d-fixed-codebook-acelp-plan.md` §0.3) contributed one carryover deliverable: the **TAME.IN → TAME.BIT byte-EQ harness** (master plan §6 line 1031). That harness is owned by Phase 2f as **TAME-1**.

### 0.4 Carryover from Phase 2d (closure report §9 OQ register, §12 entry preconditions)

| ID | State at Phase 2d close | Phase 2f disposition |
|----|-------------------------|-----------------------|
| **OQ-EXC-COMMIT** | RESOLVED at INT-0 | (closed; no Phase 2f action) |
| **OQ-Q-FORMAT-A10** | RESOLVED at INT-0 step 3 | (closed; no Phase 2f action) |
| **OQ-A38-DEPTH** | PINNED at full 8192 iterations; INT-1a slot 1/5 reserved | **NOT GATING for Phase 2f bitstream pack.** Re-evaluation deferred to post-Phase-2b H-CENTER fix or Phase 2g (perf). |
| **OQ-A38-SIGNTIE** | PINNED at sign(0) = +1; slot 2/5 reserved | NOT GATING. Same disposition. |
| **OQ-TAMING-THR** | PINNED at gp 0.95 Q14 / E 2³³; slot 3/5 reserved | **TAME-1 IS THE FIRST DIRECT WITNESS.** TAME.BIT byte-EQ exercises GA*/GB* on the dedicated taming vector. If TAME-1 disposition is materially better than PITCH-1 (Phase 2d INT-1a inherited rates), OQ-TAMING-THR pin is validated. If TAME-1 GA*/GB* rates *worse* than PITCH-1, escalate **TAME-1 slot 1/5** to sweep OQ-TAMING-THR (0.9 / 0.95 / 1.0 Q14 + E threshold variants). |
| **OQ-GA-PRESELECT-METRIC** | PINNED at L1 linear; slot 4/5 reserved | NOT GATING for Phase 2f packing. |
| **OQ-GBK-INDEX-MAP** | PINNED at physical-idx + inverse-imap pack; slot 5/5 reserved | NOT GATING for Phase 2f packing. |
| **H-CENTER** (Phase 2c carryover) | LIVE-DEFERRED; INT-1b confirms residual blocker upstream Phase 2b | **STRUCTURAL CEILING for PITCH-1 / SPEECH-1.** No Phase 2f probe can move H-CENTER (open-loop tOp lives in Phase 2b). FAIL-DEFERRED for any vector capped by H-CENTER routes back to a hypothetical future Phase 2b re-entry; not gating Phase 2f closure. |
| **H-PHASE / OQ-WINDOW / OQ-XB-NORM** (Phase 2c carryover) | LIVE-DEFERRED / PINNED / UNTESTED; Phase 2c reserved I5 slots 3/5–5/5 untouched | NOT GATING for Phase 2f packing. |
| **Phase 2d INT-1a FAIL-DEFERRED (S1 5.50 / C1 0.00 / GA1 12.15 / GB1 5.29 / S2 4.20 / C2 0.00 / GA2 11.77 / GB2 4.52 %)** | FAIL-DEFERRED | **INHERITED CEILING for INT-1 (PITCH.BIT full-frame byte-EQ).** Full-frame byte-EQ is structurally bounded below by the *minimum* per-field rate (C1/C2 0 % ⇒ full-frame ≤ 0 % on PITCH; one bit-flip flips the whole frame). PITCH-1 disposition will be ACCEPT-PARTIAL or FAIL-DEFERRED with documented blocker reference back to Phase 2d INT-1a. |
| **Phase 2 perf soft-target miss** (Phase 2d INT-2 §7) | `fcbStep` ~85 µs/op exceeds 2× Phase 2c `BenchmarkClosedloopStep` budget (29928 ns/op) | **CANDIDATE FOR PHASE 2g.** INT-2 records `BenchmarkEncodeFrame` end-to-end; if soft-realtime-binding (e.g. > 5 ms / 10 ms frame budget on AMD EPYC 9554P), authorize Phase 2g entry. NOT gating Phase 2f closure. |

### 0.5 I5 budget posture at Phase 2f entry

| Gate | Budget | Reserved | Spent | Available |
|------|-------:|---------:|------:|----------:|
| Phase 2f INT-1 (per-vector full-frame byte-EQ) | 5 | 0 | 0 | **5** |
| Phase 2d INT-1a (FCB byte-EQ vs PITCH.BIT) | 5 | 0 | 0 | 5 (NOT consumed by Phase 2f) |
| Phase 2c INT-1b reserved (post-Phase-2d re-run) | 5 | 1 | 1 | 4 (NOT consumed by Phase 2f) |
| Phase 2-final escape (G.192 byte-EQ) | 1 | 1 | 0 | 0 (RESERVED for Phase 2-final, NOT Phase 2f) |

Phase 2f INT-1 escalation chain (priority order, packing-layer only per I-2f-5):

| Slot | Hypothesis | Owner |
|------|-----------|-------|
| 1/5 | OQ-FRAME-FORMAT (G.729 10-byte vs G.192 164-byte vector framing) | INT-1 first per-vector failure |
| 2/5 | OQ-FLUSH-PAD (zero-pad vs hold-last-sample on Flush) | INT-1 if SPEECH/TAME tail-frame mismatches |
| 3/5 | OQ-VECTOR-FRAME-COUNT (.IN-bytes / 160 vs .BIT-bytes / 164 frame-count source) | INT-1 if vector lengths disagree |
| 4/5 | OQ-COLD-START-CONVENTION (frame-0 oldExc / pastQuaEn / freqPrev cold-start byte-EQ pin) | INT-1 if frame-0 differs across vectors |
| 5/5 | OQ-TAMING-THR (only consumed at TAME-1 — see §0.4) | TAME-1 |

---

## 1. Spec anchors (line ranges in `docs/superpowers/specs/itu/G729E.txt`)

| § | Lines | Subject | Binding |
|---|------:|---------|:-------:|
| **4.1** | ~1444–1530 | Bit allocation; Table 1 (lines 396–406) cross-ref. | ✅ structure |
| **4.2.1** | ~1466–1490 | Transmitted frame format: 80 bits per 10 ms frame, MSB-first within byte, transmission order per Table 8. | ✅ binding (PACK-1) |
| Table 8 | 1446–1464 | Per-symbol bit count + transmission order. Already canonicalized in `internal/bitstream/types.go:15` `Frame` field order. | ✅ binding |
| Table 1 | 396–406 | L0(1) + L1(7) + L2(5) + L3(5) + P1(8) + P0(1) + C1(13) + S1(4) + GA1(3) + GB1(4) + P2(5) + C2(13) + S2(4) + GA2(3) + GB2(4) = 80 bits. | ✅ structure |
| §A.4 | 2225–2260 | "The bit allocation is the same as in clause 4.1." Annex A passthrough. | ✅ binding |
| (design spec) §4.3 | n/a | Streaming `(*Encoder).Write` / `(*Encoder).Flush` semantics. Authored in master plan §7 (line 1048). | ✅ binding (API-2) |
| (design spec) §7.2 | n/a | ITU level-2 vector gates (per-vector byte-EQ). Master plan §7 line 1052. | ✅ binding (INT-1) |

**No new spec uncertainty introduced by Phase 2f.** All packing-layer OQs (§0.5) are protocol-shape questions, NOT arithmetic uncertainty; the spec text answers them unambiguously once read carefully (the ones logged are minor ambiguity in *test-harness* convention, e.g. which vector framing to use for byte-EQ).

---

## 2. Test-vector inventory

ITU vectors at `testdata/itu/G729_Release3/g729AnnexA/test_vectors/`:

| Vector | `.IN` bytes | `.BIT` bytes | Frame count | Encoder gate | Notes |
|--------|------------:|-------------:|------------:|--------------|-------|
| **PITCH** | 293 628 | 300 940 | 1835 | INT-1 / PITCH-1 | Re-used from Phase 2c/2d. Full-frame byte-EQ inherits C1/C2 0 % ceiling. |
| **TAME** | 20 480 | 20 992 | 128 | INT-1 / TAME-1 | **First direct witness of OQ-TAMING-THR.** Folded from master-plan §6 (Phase 2e). |
| **ALGTHM** | (TBD by INT-1 step 1) | (TBD) | (computed) | INT-1 / ALGTHM-1 | Algorithmic conformance vector. |
| **SPEECH** | (TBD) | (TBD) | (computed) | INT-1 / SPEECH-1 | Real speech corpus. |
| **FIXED** | (TBD) | (TBD) | (computed) | INT-1 / FIXED-1 | Fixed-point conformance. |
| **LSP** | (TBD) | (TBD) | (computed) | INT-1 / LSP-1 | Inherits Phase 2a INT-1 ACCEPT-PARTIAL ceiling (L0=78.67 / L1=38.93 / L2=17.07 / L3=19.35 %). |
| **TEST** | (TBD) | (TBD) | (computed) | INT-1 / TEST-1 | General regression. |

**.BIT framing.** `.BIT` files are G.192 framed (1835 frames × 164 bytes/frame = 300 940 ✓ for PITCH; 128 × 164 = 20 992 ✓ for TAME). The byte-EQ harness MUST decode .BIT through `internal/bitstream.WriteG192Frame`'s inverse (a `ReadG192Frame` to be added under PACK-2 if not already present) into a 10-byte canonical G.729 frame, then byte-compare against encoder output. Frame count is derived from `.IN-bytes / (FrameSamples × 2)` per master plan §7 line 1059 (do NOT use .BIT length to avoid F2 / OVERFLOW framing dependency inherited from Phase 1o D-2).

**Decoder-only vectors NOT gating Phase 2f (per master plan §7 line 1063):** `ERASURE.BIT`, `OVERFLOW.BIT`, `PARITY.BIT` — these are bad-frame / softbit / parity test vectors with no `.IN` input source.

---

## 3. Pre-flight inventory

### 3.1 Working-tree gate

- Phase 2d CLOSED-DEFERRED per `2026-05-12-phase2d-closure-report.md`. INT-1a S1/C1/GA1/GB1/S2/C2/GA2/GB2 FAIL-DEFERRED; INT-1b P1 10.79 / P0 57.49 / P2 11.66 % FAIL-DEFERRED.
- `git status` MUST be clean before PACK-1 starts; baseline `go test ./...` count + FAIL ledger recorded in PACK-1 step 1 (must equal **6** = 5 inherited from Phase 2c/2d carryover + 1 Phase 2d INT-1a).
- `go vet ./...` MUST pass clean as gate.
- `BenchmarkPhase2dINT2_FullFramePipeline` baseline (per Phase 2d closure §7) recorded for INT-2 perf comparison (`EncodeFrame` MUST not regress beyond +5 % vs the inner pipeline benchmark — soft target).

### 3.2 Reusable symbols (per I10 merger doctrine)

| Symbol | Location | Phase 2f use |
|--------|----------|--------------|
| `bitstream.Frame` (value type) | `internal/bitstream/types.go:15` | PACK-1 constructs from `Encoder` per-frame fields. |
| `bitstream.Pack(*Frame, []byte) error` | `internal/bitstream/pack.go:8` | API-1 calls per frame; pure function, zero-alloc. |
| `bitstream.FrameBytes` (= 10) | `internal/bitstream/types.go:8` | Cross-checked against root `g729.FrameBytes` in API-1 step 2. |
| `bitstream.FrameBits` (= 80) | `internal/bitstream/types.go:6` | Cross-check Table 1 + 8 totals. |
| `bitstream.WriteG192Frame(io.Writer, []byte, bad bool) error` | `internal/bitstream/g192.go:33` | PACK-2 reuses inverse direction (a new `ReadG192Frame` may need to be added if the decoder side does not already have one — pre-flight audit at PACK-2 step 1). |
| `g729.NewEncoder() / Reset() / EncodeFrame()` | `encoder.go:144 / 156 / 180` | API-1 replaces `EncodeFrame` body (currently `ErrNotImplemented`). |
| `g729.FrameSamples` (= 80) / `g729.FrameBytes` (= 10) | `errors.go:33 / 34` | Public API constants; unchanged by Phase 2f. |
| `g729.ErrShortPCM / ErrShortOutput` | `errors.go` | Returned by API-1 on length mismatch. |
| `g729.ErrNotImplemented` | `errors.go` (Phase 2-0 transitional sentinel) | **REMOVED by API-1** per master plan §7 line 1050. Audit all references in `encoder.go` + tests. |
| `(*Encoder).lpcStep / openloopStep / closedloopStep / fcbStep` | `encoder.go` | Called in order by `EncodeFrame` per master plan §7 line 1047. |

### 3.3 New symbols (to be added by Phase 2f)

| Symbol | Owner task | Signature |
|--------|------------|-----------|
| `Encoder.l0, l1, l2, l3` (new fields, uint16) | API-1 step 1 | Per-frame LSP indices retained by `lpcStep` for `EncodeFrame` to read. |
| `Encoder.buildBitstreamFrame(out *bitstream.Frame)` | PACK-1 | Pure: copies the 15 per-frame fields into a `bitstream.Frame` value. |
| `Encoder.encodeFrameInternal(pcm *[FrameSamples]int16, out []byte) error` | API-1 | Inner driver: lpcStep → openloopStep → closedloopStep(0) → closedloopStep(1) → buildBitstreamFrame → bitstream.Pack. |
| `(*Encoder).Write(p []int16) (n int, err error)` | API-2 | Streaming: buffers PCM, emits 10-byte frames per 80-sample boundary. Returns count of int16 samples consumed (NOT bytes). |
| `(*Encoder).Flush() error` | API-2 | Drains tail (zero-pad to 80 samples); emits one final frame if buffer non-empty. |
| `Encoder.streamBuf [FrameSamples]int16` (new field) | API-2 | Streaming PCM tail buffer. |
| `Encoder.streamBufLen int8` (new field) | API-2 | Number of valid samples in `streamBuf`. |
| `Encoder.streamSink io.Writer` (new field) | API-2 | Set via `(*Encoder).SetSink(io.Writer)` constructor variant `NewStreamingEncoder(io.Writer)`. |
| `bitstream.ReadG192Frame(io.Reader, []byte) (bad bool, err error)` (NEW IF MISSING — audit at PACK-2 step 1) | PACK-2 | Inverse of `WriteG192Frame`; emits 10-byte native frame + bad-frame flag. |
| `g729.NewStreamingEncoder(w io.Writer) *Encoder` | API-2 | Constructor variant; `Write`/`Flush` emit to `w`. |

### 3.4 Encoder state-additions checklist

API-1 step 1 adds the per-frame LSP fields (placed below the existing Phase 2d block at `encoder.go:135`):

```go
// Phase 2f API-1: per-frame LSP indices retained by lpcStep so
// EncodeFrame can pack them via internal/bitstream. Phase 2a/2b/2c/2d
// retained these only as the lsp.Indices return value of lpcStep;
// Phase 2f promotes them to Encoder fields for symmetry with the
// pitch (p1/p0/p2) and FCB+gain (s1/s2/c1/c2/ga*/gb*) fields.
l0, l1, l2, l3 uint16
```

API-2 adds the streaming state (placed after the LSP fields):

```go
// Phase 2f API-2: streaming Write/Flush state.
//
// streamBuf is the PCM tail buffer holding 0..79 samples not yet
// emitted as a frame. streamBufLen counts the valid samples.
// streamSink is the destination io.Writer (nil for non-streaming
// Encoder instances; (*Encoder).Write returns ErrNoStreamSink in
// that case).
streamBuf    [FrameSamples]int16
streamBufLen int8
streamSink   io.Writer
```

`Reset()` clears all three (existing `*e = Encoder{}` already covers this; cross-check at API-2 step 4).

---

## 4. Package-layout decision

**Choice:** all Phase 2f production code lives in the **root package** (`encoder.go` + new `streaming.go` + new `pack.go` if separation is preferred). NO new `internal/*` package introduced.

**Justification:**
- `EncodeFrame` is the public entry point; it MUST live in the root `g729` package per the existing skeleton (`encoder.go:180`).
- Bitstream packing is a one-line composition (`bitstream.Pack(&frame, out)`) that does not warrant a new package.
- Streaming `Write`/`Flush` is API-shape only; no new arithmetic; co-locates with the public encoder type.
- The TAME / ALGTHM / SPEECH / FIXED / LSP / PITCH / TEST byte-EQ harnesses live as `phase2f_int1_<vector>_byteeq_test.go` files in the root, mirroring the Phase 2c/2d harness convention (`phase2c_int1_pitch_byteeq_test.go`, `phase2d_int1a_fcb_byteeq_test.go`).
- A potential `internal/bitstream.ReadG192Frame` addition (if missing) is a pure-function read primitive; it lives in `internal/bitstream` next to its `Write` counterpart.

**Alternatives rejected:**
- *New `internal/encoder/` package* — would duplicate the existing root-package encoder skeleton and break the Phase 2-0 layout decision. Rejected.
- *New `internal/streaming/` package* — over-engineering for two methods + 2 new fields. Rejected.
- *Public `Pack(*Encoder, []byte)` helper in `frame.go`* — the existing `frame.go` convenience pattern (line 5) is per-instance method delegation, not a packing primitive. The packing logic stays internal; the public surface is `EncodeFrame`. Rejected.

---

## 5. Task ledger

> Each task: 5-step TDD checklist (RED → GREEN → refactor → vet → commit). Each commit message stub MUST include `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` (I8).

### PACK-1 — G.729 10-byte frame packing from `Encoder` per-frame fields

§4.2.1 + Table 8. Composes the 15 per-frame field values into a `bitstream.Frame` and calls `bitstream.Pack`. Owns the L0/L1/L2/L3 retention work (API-1 step 1).

- [ ] Step 1 — Baseline: `go test ./... 2>&1 | tail -40` count + FAIL ledger (must equal 6); `git status` clean. Add `Encoder.l0, l1, l2, l3 uint16` fields per §3.4; modify `lpcStep` to write `e.l0 = uint16(indices.L0); e.l1 = uint16(indices.L1); e.l2 = uint16(indices.L2); e.l3 = uint16(indices.L3)` after `lsp.Quantize` (encoder.go:265).
- [ ] Step 2 — RED: `phase2f_pack1_buildframe_test.go` constructs an `Encoder` with hand-set per-frame fields (e.g. l0=1, l1=42, p1=200, c1=0x1ABC, s1=0xA, ga1=5, gb1=11, p2=27, c2=0x0123, s2=0x3, ga2=2, gb2=8), calls `e.buildBitstreamFrame(&f)`, and asserts every field on the returned `bitstream.Frame` equals the encoder field. Then calls `bitstream.Pack(&f, out[:])` and asserts the resulting 10 bytes match a hand-derived bit pattern (compute via the Table 8 layout).
- [ ] Step 3 — GREEN: implement `(*Encoder).buildBitstreamFrame(out *bitstream.Frame)` in `encoder.go` (or a new `pack.go` if the encoder file size warrants). Pure assignment; zero-alloc.
- [ ] Step 4 — `go vet ./...` ✅; `go test ./...` ✅; alloc bench (`AllocsPerRun(128, buildBitstreamFrame) == 0`).
- [ ] Step 5 — Commit `phase2f(pack): G.729 10-byte frame pack from Encoder fields per 4.2.1 + Table 8 (PACK-1)` with I8 trailer.

### PACK-2 — G.192 framing read primitive (for ITU vector harness)

§A.3 (G.192 framing convention; G.191 STL `g192.txt` reference). Adds the *inverse* of the existing `bitstream.WriteG192Frame` so the INT-1 harnesses can decode `.BIT` files into 10-byte canonical frames for byte-EQ comparison.

- [ ] Step 1 — Audit: `grep -n "ReadG192" internal/bitstream/`. If a reader already exists, this task is no-op; jump to step 5 with `phase2f(pack): G.192 reader already present (PACK-2 noop)`. Otherwise proceed.
- [ ] Step 2 — RED: `internal/bitstream/g192_test.go` round-trip test: synthesize a 10-byte `Pack` output, route through `WriteG192Frame` to a `bytes.Buffer`, then `ReadG192Frame` should recover the original 10 bytes + `bad=false`. Assert one negative case (sync 0x6B20 ⇒ bad=true).
- [ ] Step 3 — GREEN: implement `bitstream.ReadG192Frame(r io.Reader, out []byte) (bad bool, err error)` in `internal/bitstream/g192.go`. Validate sync ∈ {G192SyncGood, G192SyncBad}; validate length word == FrameBits; read 80 LE-uint16 softbits; emit MSB-first into out[0..9]. Pooled buffer per `WriteG192Frame` precedent.
- [ ] Step 4 — `go vet ./internal/bitstream/...` ✅; `go test ./internal/bitstream/...` ✅; alloc bench `AllocsPerRun(128, ReadG192Frame) == 0`.
- [ ] Step 5 — Commit `phase2f(bitstream): add G.192 ReadG192Frame for ITU vector harness (PACK-2)`.

### API-1 — Public `Encoder.EncodeFrame` end-to-end wiring

Master plan §7 line 1047.

- [ ] Step 1 — RED: `phase2f_api1_encodeframe_test.go` calls `e.EncodeFrame(pcm[:], out[:])` over PITCH.IN frame 0; asserts `err == nil` (no longer `ErrNotImplemented`) and `len(out) == FrameBytes`; asserts the 10-byte output matches a hand-recorded value derived from the existing per-frame fields populated by `closedloopStep` + `fcbStep` after the same input (cross-checked via `buildBitstreamFrame` round-trip).
- [ ] Step 2 — GREEN: replace `EncodeFrame` body in `encoder.go:180`:
    ```go
    func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
        if len(pcm) != FrameSamples {
            return ErrShortPCM
        }
        if len(out) < FrameBytes {
            return ErrShortOutput
        }
        if _, err := e.lpcStep(pcm); err != nil {
            return err
        }
        e.openloopStep()
        e.closedloopStep(0)
        e.closedloopStep(1)
        var f bitstream.Frame
        e.buildBitstreamFrame(&f)
        return bitstream.Pack(&f, out)
    }
    ```
- [ ] Step 3 — Refactor: remove `ErrNotImplemented` from `errors.go` (master plan §7 line 1050). Audit references via `grep -rn "ErrNotImplemented" .` — must be zero outside the tests asserting on its prior behaviour. Update those tests to assert success. **NOTE:** keep `ErrNotImplemented` defined as a deprecated alias for one Phase 2f cycle if external test imports break; final removal in INT-3.
- [ ] Step 4 — `go vet ./...` ✅; `go test ./...` ✅ (or RED only on pre-existing FAIL-DEFERRED tests); `go test -race ./...`; bench `BenchmarkEncodeFrame`.
- [ ] Step 5 — Commit `phase2f(encoder): wire EncodeFrame end-to-end via Pack (API-1)`.

### API-2 — Streaming `Write` / `Flush` wrapper

Master plan §7 line 1048; design spec §4.3.

- [ ] Step 1 — RED part A: `phase2f_api2_streaming_test.go` exercises:
    1. `NewStreamingEncoder(buf)` on empty buf; `Write(make([]int16, 80))` returns `(80, nil)`; `buf.Len() == 10` (one frame emitted).
    2. `Write(make([]int16, 79))` returns `(79, nil)`; `buf.Len() == 0` (no frame emitted; 79 < 80 boundary).
    3. `Write(make([]int16, 1))` returns `(1, nil)`; `buf.Len() == 10` (boundary crossed).
    4. `Write(make([]int16, 240))` returns `(240, nil)`; `buf.Len() == 30` (3 frames emitted).
    5. `Flush()` on partial-buffer state (write 50 then Flush) emits one zero-padded frame; `buf.Len() == 10`. Flush on empty buffer is a no-op (`buf.Len()` unchanged).
- [ ] Step 1 — RED part B: zero-alloc gate `phase2f_api2_streaming_zeroalloc_test.go`: `AllocsPerRun(128, func(){ e.Write(pcm80[:]) }) == 0`.
- [ ] Step 2 — GREEN: add `streamBuf`, `streamBufLen`, `streamSink` fields per §3.4; add `NewStreamingEncoder(w io.Writer) *Encoder` constructor; implement `Write` and `Flush` per I-2f-3 semantics. Use a stack-allocated `[FrameBytes]byte` scratch in `Write` for the per-frame `EncodeFrame` output before forwarding to `streamSink.Write`.
- [ ] Step 3 — Refactor: ensure `Reset()` clears the streaming state correctly (already covered by `*e = Encoder{}` per existing pattern; verify in test).
- [ ] Step 4 — `go vet ./...` ✅; `go test ./...` ✅; `go test -race ./...`; bench `BenchmarkStreamingWrite_80sample`.
- [ ] Step 5 — Commit `phase2f(encoder): streaming Write/Flush per design 4.3 (API-2)` with I8 trailer.

### TAME-1 — TAME.BIT byte-EQ harness (folded from master-plan §6)

Master plan §6 line 1031; first direct witness of OQ-TAMING-THR. **THE TAMING-VECTOR-SPECIFIC GATE.**

- [ ] Step 1 — RED: `phase2f_int1_tame_byteeq_test.go` reads `testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.IN` (20 480 bytes = 128 frames × 80 int16 samples), reads `TAME.BIT` (20 992 bytes = 128 frames × 164 G.192 bytes), uses `bitstream.ReadG192Frame` (PACK-2) to extract 128 × 10-byte canonical frames as ground-truth. Encodes TAME.IN frame-by-frame via `EncodeFrame`, byte-compares each output against ground-truth. Records per-field rates (L0/L1/L2/L3/P1/P0/C1/S1/GA1/GB1/P2/C2/S2/GA2/GB2) by `bitstream.Unpack`-ing both the encoder output and the ground truth and comparing field-wise. Reports both *full-frame match rate* and *per-field match rates*. **Plausibility floor:** GA1/GA2/GB1/GB2 rates on TAME MUST be ≥ Phase 2d INT-1a baseline (12.15 / 11.77 / 5.29 / 4.52 %) — i.e. the dedicated taming corpus does not regress vs the random-speech corpus.
- [ ] Step 2 — Disposition decision tree:
    - If full-frame ≥ 80 % → **PASS**; record disposition; close. Validates OQ-TAMING-THR pin.
    - If GA*/GB* rates ≥ Phase 2d INT-1a baseline AND full-frame < 80 % → **ACCEPT-PARTIAL**; record per-field plausibility breakdown; OQ-TAMING-THR pin holds; remaining shortfall traces back to inherited Phase 2c (P1/P2) + Phase 2d (S/C) FAIL-DEFERRED.
    - If GA*/GB* rates < Phase 2d INT-1a baseline → **escalate slot 5/5 (OQ-TAMING-THR)**: sweep gp clamp ∈ {0.9, 0.95, 1.0} × E-threshold ∈ {2³², 2³³, 2³⁴}. Record sweep results; pin best variant; re-run.
- [ ] Step 3 — Refactor: extract a shared `vectorByteEQHarness(inPath, bitPath string, expectedFrames int) HarnessResult` helper if multiple per-vector tests share the boilerplate; place in `phase2f_int1_harness_test.go`.
- [ ] Step 4 — `go vet ./...` ✅; per-vector test runs without panic.
- [ ] Step 5 — Commit `phase2f(int1): TAME.BIT byte-EQ harness — first OQ-TAMING-THR witness (TAME-1)` with disposition recorded in commit body.

### INT-1 — Per-vector full-frame byte-EQ (PITCH / ALGTHM / SPEECH / FIXED / LSP / TEST)

Master plan §7 line 1052; six per-vector tests (TAME-1 separate above due to OQ-TAMING-THR linkage).

- [ ] Step 1 — RED: author `phase2f_int1_<vector>_byteeq_test.go` for each of {pitch, algthm, speech, fixed, lsp, test}, each one mirroring the TAME-1 harness shape (PACK-2 G.192 read, frame-by-frame compare, per-field + full-frame rate report). Frame count derived from `.IN` size per master plan §7 line 1059.
- [ ] Step 2 — Disposition matrix per vector (apply in priority order, per I-2f-5):
    - **PITCH-1:** Full-frame ≤ min(C1, C2) ≤ 0 % per Phase 2d INT-1a inheritance. Disposition: **FAIL-DEFERRED** with documented routing to Phase 2d INT-1a + Phase 2b H-CENTER. NO Phase 2f I5 spent (structural blocker upstream per I-2f-5).
    - **ALGTHM-1, SPEECH-1, FIXED-1, TEST-1:** Same structural ceiling. Disposition: report rates; PASS if full-frame ≥ 80 %; ACCEPT-PARTIAL if max-per-field rate ≥ corresponding Phase 2a/2c/2d baseline; otherwise **FAIL-DEFERRED with upstream routing.**
    - **LSP-1:** Inherits Phase 2a INT-1 ACCEPT-PARTIAL ceiling (L0=78.67 / L1=38.93 / L2=17.07 / L3=19.35 %). Plausibility floor: per-field rates MUST be ≥ Phase 2a baselines; if not, escalate slot 4/5 (OQ-COLD-START-CONVENTION). Full-frame disposition is bounded by L1×L2×L3 product; ACCEPT-PARTIAL is the realistic best.
- [ ] Step 3 — I5 ledger: record per-vector escalation (if any). Maximum spend per task (PITCH-1 / ALGTHM-1 / SPEECH-1 / FIXED-1 / LSP-1 / TEST-1) is bounded by the **shared 5/5 Phase 2f INT-1 budget** (one budget across all six per-vector tests, NOT 5 each). If any per-vector escalation discovers a packing-layer bug (OQ-FRAME-FORMAT / OQ-FLUSH-PAD / OQ-VECTOR-FRAME-COUNT / OQ-COLD-START-CONVENTION), spend the slot and apply the fix to the production code (NOT the test).
- [ ] Step 4 — `go vet ./...` ✅; `go test ./...` (per-vector failures expected per dispositions; record FAIL-DEFERRED dispositions in commit body).
- [ ] Step 5 — Commit `phase2f(int1): per-vector byte-EQ harness PITCH/ALGTHM/SPEECH/FIXED/LSP/TEST (INT-1)` with full disposition table in commit body.

### INT-2 — Zero-alloc + race + bench at public API level

Master plan §I4.

- [ ] Step 1 — RED: `phase2f_int2_encode_zeroalloc_test.go` asserts `testing.AllocsPerRun(128, func(){ e.EncodeFrame(pcm80[:], out10[:]) }) == 0` for both:
    - cold-start Encoder (frame 0)
    - steady-state Encoder (frame 100, post warm-up)
- [ ] Step 2 — RED part B: streaming zero-alloc — `AllocsPerRun(128, func(){ e.Write(pcm80[:]) }) == 0` (covered by API-2 step 1 part B; double-check here at the public-API level with `NewStreamingEncoder(io.Discard)`).
- [ ] Step 3 — GREEN: convert any captured allocs to caller-owned scratch (encoder receiver fields). The likely culprit is the `bitstream.Frame` value in `EncodeFrame` — if the compiler does not stack-allocate it, promote to an `Encoder.bsFrame` field.
- [ ] Step 4 — `go test -race ./...` green (no new DATA RACE beyond Phase 2d baseline); `BenchmarkEncodeFrame` captured (per-frame ns/op + B/op + allocs/op); compare to `BenchmarkPhase2dINT2_FullFramePipeline` baseline (≤ +5 % regression acceptable). Also capture `BenchmarkStreamingWrite_80sample` and `BenchmarkStreamingWrite_800sample` (10-frame batch).
- [ ] Step 5 — Commit `phase2f(encoder+streaming): zero-alloc + race-clean (INT-2)` with bench numbers in commit body.

### INT-3 — Closure report

- [ ] Step 1 — Write `docs/superpowers/plans/YYYY-MM-DD-phase2f-closure-report.md` mirroring Phase 2d closure report sections (overview, task ledger, INT-1 per-vector dispositions in side-by-side table with PASS / ACCEPT-PARTIAL / FAIL-DEFERRED + upstream routing for each, plausibility math, OQ register, I5 budget, perf bench, Phase 2-final entry preconditions).
- [ ] Step 2 — Update master plan §7 row to CLOSED with closure-report link. Add a §7-line summary of per-vector disposition (one line per vector).
- [ ] Step 3 — Audit `ErrNotImplemented` removal completeness; if API-1 step 3 left a deprecation alias, decide retention or final removal per master plan §7 line 1050.
- [ ] Step 4 — Phase 2-final readiness check: list outstanding FAIL-DEFERRED gates (Phase 2a INT-1 LSP, Phase 2c INT-1 P1/P0/P2, Phase 2d INT-1a S/C/GA/GB, Phase 2f per-vector dispositions) and route each to Phase 2-final closure report (`2026-XX-XX-phase2-completion-report.md` per master plan §8) for final disposition. Document Phase 2g (perf) need-or-no-need based on `BenchmarkEncodeFrame` numbers.
- [ ] Step 5 — Commit `phase2f: closure report + master-plan flip + Phase 2-final entry checklist (INT-3)`.

---

## 6. Per-task contract summary

| Task | Inputs | Outputs | Spec | Test |
|------|--------|---------|------|------|
| PACK-1 | Encoder per-frame fields (l0..l3, p1, p0, c1, s1, ga1, gb1, p2, c2, s2, ga2, gb2) | bitstream.Frame value; 10-byte packed output via bitstream.Pack | §4.2.1 + Table 8 | unit golden |
| PACK-2 | G.192-framed io.Reader | 10-byte canonical frame + bad-frame flag | G.192 (STL G.191) | unit round-trip |
| API-1 | pcm[80]int16 | out[10]byte (no error) | §4.2.1 (frame format) | encoder smoke |
| API-2 | streaming pcm []int16 + io.Writer | 10-byte frames per 80-sample boundary; Flush zero-pads tail | design §4.3 | streaming behaviour matrix |
| TAME-1 | TAME.IN | TAME.BIT byte-EQ (full-frame + per-field) | §3.9.2 (taming) + §4.2.1 | corpus 128 frames |
| INT-1 | PITCH / ALGTHM / SPEECH / FIXED / LSP / TEST .IN | corresponding .BIT byte-EQ matrix | §4.2.1 + §A.4 | corpus per-vector |
| INT-2 | EncodeFrame + streaming Write | 0 allocs / race-clean | I4 | bench |
| INT-3 | results | closure report + master-plan flip + Phase 2-final entry checklist | — | doc |

---

## 7. Phase 2 closure conditions

Per master plan §7 line 1063: "Phase 2 closure trigger: All seven `*.IN`/`*.BIT` vectors PASS byte-EQ. The 3 ERASURE / OVERFLOW / PARITY decoder-only vectors are NOT encoder gates."

**Realistic Phase 2f outcome posture (given Phase 2c INT-1 + Phase 2d INT-1a FAIL-DEFERRED inheritance):**

| Vector | Realistic Phase 2f disposition | Closure path |
|--------|-------------------------------|--------------|
| PITCH | FAIL-DEFERRED (C1/C2 0 % structural) | Routed to Phase 2d INT-1a + Phase 2b H-CENTER. Recorded in INT-3 §<vector>. |
| TAME | ACCEPT-PARTIAL or PASS depending on OQ-TAMING-THR validation | If PASS, OQ-TAMING-THR closed; if ACCEPT-PARTIAL, slot 5/5 escalation gives a tighter pin or routes to upstream. |
| ALGTHM | FAIL-DEFERRED expected (same upstream blockers) | Same upstream routing. |
| SPEECH | FAIL-DEFERRED expected | Same upstream routing. |
| FIXED | FAIL-DEFERRED expected | Same upstream routing. |
| LSP | ACCEPT-PARTIAL (Phase 2a inheritance) | Routed to Phase 2a INT-1 ACCEPT-PARTIAL closure. |
| TEST | FAIL-DEFERRED expected | Same upstream routing. |

**Phase 2 cycle disposition** (NOT closed by Phase 2f alone): the master plan §7 closure trigger ("all seven PASS") is a **stretch goal**. The realistic disposition at Phase 2f close is **CLOSED-DEFERRED** with documented per-vector dispositions; the master plan §8 Phase 2-final closure report then does the final accounting (per `2026-05-02-phase2-encoder-plan.md` §8 lines 1067–1099).

**Phase 2-final readiness criteria** (recorded in INT-3):
- Public `EncodeFrame` returns no `ErrNotImplemented` (✓ via API-1).
- Streaming `Write`/`Flush` work and zero-alloc (✓ via API-2 + INT-2).
- Per-vector harnesses present and dispositioned for all 7 vectors (✓ via TAME-1 + INT-1).
- All FAIL-DEFERRED dispositions point to a documented upstream blocker (Phase 2a/2b/2c/2d closure report) or a documented Phase 2g (perf) candidate.
- I5 budget table reconciled across phases (no double-spend).

If **any** vector PASSes at full-frame ≥ 80 %, that is a meaningful Phase 2f outcome and a candidate for partial Phase 2-final closure. If **all** vectors PASS, the master plan §7 closure trigger fires and Phase 2-final is mechanical.

---

## 8. I5 budget (Phase 2f INT-1 + TAME-1) — fresh

| Slot | Hypothesis | Owner | Spent | Remaining | Result |
|------|-----------|-------|------:|----------:|--------|
| 0/5 | Baseline (PACK-1 default 10-byte G.729 frame; API-1 default lpcStep→openloopStep→2×closedloopStep→Pack chain; API-2 default zero-pad-on-Flush) | INT-1 first per-vector run | 0/5 | 5/5 | (TBD) |
| 1/5 | OQ-FRAME-FORMAT (G.729 native 10-byte vs G.192 164-byte vector framing) | INT-1 first per-vector failure | — | — | reserved |
| 2/5 | OQ-FLUSH-PAD (zero-pad vs hold-last-sample on Flush) | INT-1 SPEECH/TAME tail mismatch | — | — | reserved |
| 3/5 | OQ-VECTOR-FRAME-COUNT (.IN/160 vs .BIT/164 frame source) | INT-1 vector-length disagreement | — | — | reserved |
| 4/5 | OQ-COLD-START-CONVENTION (frame-0 byte-EQ pin) | INT-1 / LSP-1 frame-0-only mismatch | — | — | reserved |
| 5/5 | OQ-TAMING-THR (gp 0.9/0.95/1.0 × E 2³²/2³³/2³⁴ sweep) | TAME-1 GA*/GB* < Phase 2d baseline | — | — | reserved |

**Phase 2c reserved I5 (4/4 untouched), Phase 2d INT-1a (5/5 reserved), Phase 2-final escape (1/1 reserved)** — NOT consumed by Phase 2f per I-2f-5 + §0.5 budget posture.

---

## 9. Open questions / risks (OQ register)

| ID | Spec cite | Default pin | Escalation knob | Owner gate |
|----|-----------|-------------|------------------|------------|
| **OQ-FRAME-FORMAT** | §4.2.1 (G.729 native) + G.192 (STL G.191, transport) | Public `EncodeFrame` writes 10-byte G.729 native; G.192 emitted only via `bitstream.WriteG192Frame` for vector comparison | Try G.192 as the public output if downstream tooling expects it | INT-1 slot 1/5 |
| **OQ-FLUSH-PAD** | design §4.3 (silent on tail-padding semantics) | Zero-pad tail with 0x0000 (linear-PCM silence) | Hold-last-sample; abort-with-error; emit-no-frame | INT-1 slot 2/5 |
| **OQ-VECTOR-FRAME-COUNT** | master plan §7 line 1059 | Frame count = `.IN-bytes / (FrameSamples × 2)` (160 bytes/frame); `.BIT` length is *expected* but not authoritative | If `.IN` and `.BIT` disagree on frame count, halt and flag (do NOT silently truncate). Phase 1o D-2 OVERFLOW.BIT precedent applies. | INT-1 slot 3/5 |
| **OQ-COLD-START-CONVENTION** | §A.3 (encoder cold start) | `NewEncoder()` initializes `pastQuaEn` to `4 × gain.PastErrorsDefault`, `lspOld` via `lsp.InitLSPOld`, `freqPrev` via `lsp.InitFreqPrev`; all other state zero | Frame-0 byte-EQ may differ across vectors if a per-vector "preamble" convention exists; INT-1 LSP-1 step 1 records frame-0 disposition specifically. | INT-1 slot 4/5 |
| **OQ-TAMING-THR** (carryover from Phase 2d) | §3.9.2 narrative | gp clamp 0.95 Q14 = 15565; E threshold 2³³ | Sweep {0.9, 0.95, 1.0} × {2³², 2³³, 2³⁴} | TAME-1 slot 5/5 |
| **OQ-A38-DEPTH** (carryover) | §A.3.8.1 | Full 8192 iterations | Phase 2g | (NOT Phase 2f) |
| **OQ-A38-SIGNTIE** (carryover) | §3.8.1 | sign(0) = +1 | Phase 2-final escape | (NOT Phase 2f) |
| **OQ-GA-PRESELECT-METRIC** (carryover) | §3.9.2 | L1 linear | Phase 2-final escape | (NOT Phase 2f) |
| **OQ-GBK-INDEX-MAP** (carryover) | §3.9.3 | Physical-idx + inverse-imap pack | Phase 2-final escape | (NOT Phase 2f) |
| **H-CENTER** (Phase 2c carryover) | Phase 2b open-loop | LIVE-DEFERRED | Phase 2b re-entry / Phase 2-final | (structural ceiling for PITCH-1 / SPEECH-1 / TEST-1) |
| **H-PHASE / OQ-WINDOW / OQ-XB-NORM** (Phase 2c carryover) | §A.3.6 / §A.3.7 | LIVE-DEFERRED / PINNED / UNTESTED | Phase 2c reserved slots 3/5–5/5 | (NOT Phase 2f) |
| **Risk R-1** | Bitstream pack alloc surface | `bitstream.Pack` is documented zero-alloc (`internal/bitstream/pack.go:8` doc comment). Verify at INT-2 with `AllocsPerRun(128, EncodeFrame) == 0`. If `bitstream.Frame` does not stack-allocate, promote to `Encoder.bsFrame` field. | INT-2 step 3 |
| **Risk R-2** | Streaming buffer race | `Write`/`Flush` are NOT thread-safe by design (per `Encoder` doc comment line 23). Documented; not a Phase 2f bug. | (none) |
| **Risk R-3** | Phase 2-final perf budget | `EncodeFrame` may exceed soft-realtime (e.g. > 5 ms / 10 ms frame). If `BenchmarkEncodeFrame` measures > 2 ms/op on AMD EPYC 9554P, recommend Phase 2g entry in INT-3. Phase 2g pruning of ACELP search (OQ-A38-DEPTH knob) is the most likely lever. | INT-3 step 4 |
| **Risk R-4** | I5 doctrine | Risk of double-spending I5 across Phase 2a/2b/2c/2d/2f INT-1 budgets. Mitigation: §0.5 budget table is canonical; Phase 2f INT-1 spends ONLY against packing-layer OQs per I-2f-5. | (canonical table §0.5) |

---

## 10. Inheritance to Phase 2-final / Phase 2g

Phase 2f MUST hand off:
- Public `EncodeFrame` returning no `ErrNotImplemented` (API-1).
- Streaming `Write`/`Flush` API surface stable (API-2).
- Seven per-vector byte-EQ harnesses with documented dispositions (TAME-1 + INT-1).
- `BenchmarkEncodeFrame` end-to-end measurement (INT-2) for Phase 2g (perf) trigger decision.
- I5 ledger reconciled across all Phase 2 sub-phases (INT-3).

**Phase 2-final entry preconditions** (per master plan §8):
- All FAIL-DEFERRED dispositions traced to documented upstream blocker.
- `ErrNotImplemented` removed from public API.
- Public API stable (no breaking changes since Phase 2-0 except `ErrNotImplemented` removal).
- Inherited Phase 1o FAILs (3 entries: SinglePulseChain, LowEnergyCodebookIsSmooth, SucceedsAcrossAllGainIndices) re-examined with Phase 2 encoder symmetry (master plan §9 line 1109).
- R-A / R-B / R-C ambiguity ledger re-examined (master plan §9 line 1110).
- SF-1 tilt γ_t gating re-examined (master plan §9 line 1111).
- OVERFLOW.BIT framing rationale re-examined (master plan §9 line 1112).

**Phase 2g (perf) — contingent.** Authored only if Phase 2f INT-2 measures `BenchmarkEncodeFrame` as soft-realtime-binding. Phase 2g would consume **Phase 2d INT-1a slot 1/5 (OQ-A38-DEPTH)** with a pruned ACELP variant (per-depth K-best or threshold-controlled focused search per §3.8.1 K3 = 0.4 / max-180 base codec); not a precondition for Phase 2-final.

---

## 11. Self-review

- [x] Mirrors Phase 2d sub-plan structure (§§0–12 modulo Phase 2f scope).
- [x] All ITU spec references cite line ranges in `docs/superpowers/specs/itu/G729E.txt` or design-doc § (for streaming API).
- [x] §4.2.1 + Table 8 binding called out as I-2f-2 (bit ordering MSB-first transmission order).
- [x] G.729 native-vs-G.192 framing decision called out as I-2f-1 + OQ-FRAME-FORMAT.
- [x] Streaming Write/Flush semantics called out as I-2f-3 + OQ-FLUSH-PAD.
- [x] Stateless bitstream pack discipline called out as I-2f-4.
- [x] No new spec arithmetic discipline called out as I-2f-5 (FAIL-DEFERRED routes upstream).
- [x] Carryover from Phase 2d (5 PINNED OQs + Phase 2c LIVE OQs) explicitly inherited; structural ceiling for PITCH-1/SPEECH-1 documented.
- [x] Each task has TDD 5-step checklist + commit message stub + I8 trailer mandate.
- [x] TAME-1 separated from INT-1 to give OQ-TAMING-THR a dedicated harness (folds master-plan §6).
- [x] INT-1 per-vector dispositions enumerated with realistic FAIL-DEFERRED expectations + upstream routing.
- [x] I5 budget posture (§0.5) explicit with cross-phase non-double-spend invariant (R-4).
- [x] Phase 2 closure conditions (§7) realistic; Phase 2-final entry preconditions (§10) enumerated.
- [x] Reusable symbols enumerated (§3.2) per merger doctrine.
- [x] Package-layout decision (root-package only) justified with rejected alternatives (§4).
- [x] TAME.BIT vector status confirmed PRESENT (`testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.{IN,BIT}`); both files 20480 / 20992 bytes = 128 frames.

---

## 12. Execution handoff

**Next dispatch:** PACK-1 (G.729 10-byte frame packing from Encoder per-frame fields per §4.2.1 + Table 8).

**Order of execution:** PACK-1 → PACK-2 → API-1 → API-2 → TAME-1 → INT-1 (PITCH / ALGTHM / SPEECH / FIXED / LSP / TEST in any order) → INT-2 → INT-3.

**Stop conditions:**
- PACK-1 / PACK-2 / API-1 / API-2 RED after a TDD round → escalate per ledger; do not skip.
- TAME-1 GA*/GB* < Phase 2d INT-1a baseline → MUST escalate slot 5/5 (OQ-TAMING-THR) before proceeding to INT-1.
- INT-1 per-vector failure that traces to a packing-layer bug → spend the appropriate slot (1/5–4/5), apply fix, re-run all dispositioned vectors.
- INT-1 per-vector failure that traces upstream → record FAIL-DEFERRED disposition; do NOT spend Phase 2f I5 (per I-2f-5).
- INT-2 zero-alloc fail → promote `bitstream.Frame` to `Encoder.bsFrame` field (R-1 mitigation).
- INT-3 closure report MUST be authored even if all INT-1 vectors FAIL-DEFERRED — closure report is the routing mechanism to Phase 2-final.

**Phase 2 cycle status at Phase 2f close:** **CLOSED-DEFERRED** is the realistic outcome (per §7); **CLOSED-PASS** requires all 7 vectors at ≥ 80 % full-frame which is bounded above by Phase 2c/2d/2a INT-1 ceilings. Phase 2-final closure report (master plan §8) is the next dispatch after Phase 2f INT-3.

— end of Phase 2f sub-plan —
