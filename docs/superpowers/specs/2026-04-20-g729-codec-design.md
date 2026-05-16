# G.729A Pure-Go Codec — Design Spec

- **Date:** 2026-04-20
- **Status:** Approved for planning
- **Scope:** Pure-Go G.729 Annex A encoder and decoder library, to be published as open source under MIT.

---

## 1. Goals and context

Build a pure-Go implementation of the ITU-T G.729 Annex A speech codec. The primary consumer is a TTS pipeline that already produces 8 kHz 16-bit LE mono PCM and needs to stream it as G.729 over RTP to an MRCP endpoint. The team already operates μ-law, A-law, and G.722 implementations; G.729 fills the remaining common VoIP codec gap.

Existing open-source G.729 libraries are GPL or LGPL, which is incompatible with the intended downstream usage (commercial binary distribution of products that embed this library). The library itself will be released as open source, so the implementation must also be clean-room with respect to copyright: written from the ITU-T specification documents only, without reference to ITU's C source, bcg729, or any other existing implementation.

G.729 patents expired in 2017, so there are no patent obstacles. The constraint is purely code-copyright: the spec is referenceable, existing code is not.

### Non-goals

- Annex B (VAD / DTX / CNG). Out of scope for this spec; a follow-up spec may add it.
- Full G.729 (non-Annex-A) with its higher-complexity analyses. Annex A is bitstream-compatible with G.729 and is what VoIP deployments use.
- Internal resampling. Input must be 8 kHz; callers handle rate conversion upstream.
- Any form of SIMD or CGo. Pure portable Go only.

---

## 2. Decisions summary

| Area | Decision |
|---|---|
| Variant | G.729 Annex A only |
| Initial spec scope | Encoder focus; decoder included in implementation for round-trip testing |
| Input PCM | 8 kHz / 16-bit LE signed / mono, mandatory |
| Public API | Strict 80-sample frame core + streaming `Write`/`Flush` wrapper |
| Concurrency | Per-instance isolation; no internal locks; caller enforces one goroutine per instance |
| Numeric representation | 16-bit fixed-point, Q-format per ITU specification |
| Implementation source | Scratch from ITU-T G.729 + Annex A specification documents; no reference to existing code |
| Completion criterion | All official ITU test vectors pass bit/sample exact (both encoder and decoder) |
| Release gate (outside this spec) | Bit-exact tests + live MRCP interop + PESQ quality thresholds |
| Performance target | Hundreds of concurrent channels; zero allocation in steady-state hot path |
| Dependencies | Go 1.22+, no runtime dependencies, no CGo, no SIMD, no assembly |
| License | MIT |

---

## 3. Repository and package layout

Single Go module `g729` (module path TBD by owner at publish time, e.g. `github.com/<owner>/g729`). Internal DSP units live under `internal/` so the public API surface stays minimal and stable.

```
g729/
├── go.mod                          (Go 1.22+, zero runtime deps)
├── go.sum
├── README.md
├── LICENSE                         (MIT)
├── doc.go                          (package doc with usage examples)
│
├── encoder.go                      (public: Encoder, NewEncoder, Write, Flush, Reset)
├── decoder.go                      (public: Decoder, NewDecoder, Decode, Reset, ConcealFrame)
├── frame.go                        (public: EncodeFrame, DecodeFrame strict-frame API)
├── errors.go                       (sentinel errors)
│
├── internal/
│   ├── fixed/                      Q-format arithmetic (L_mult, L_add, add, sub,
│   │                               shl, shr, saturate, round, norm ... all ITU
│   │                               basic ops renamed to idiomatic Go)
│   ├── tables/                     ITU lookup tables (LSP VQ codebook,
│   │                               fixed codebook structure, gain codebook,
│   │                               analysis/lag windows, Chebyshev grid)
│   ├── lpc/                        Autocorrelation, Levinson-Durbin
│   ├── lsp/                        LPC <-> LSP conversion, 4-stage split VQ,
│   │                               MA predictor state
│   ├── pitch/                      Open-loop + closed-loop pitch, adaptive codebook
│   ├── acelp/                      Algebraic codebook search (G.729A fast variant)
│   ├── gain/                       Gain quantization, gain prediction, taming
│   ├── filter/                     Weighting filter, synthesis filter, post-filter
│   ├── bitstream/                  80-bit frame <-> []byte packing, ITU .bit format
│   └── pcm/                        int16 <-> Q-format, high-pass pre-processing
│
└── testdata/
    └── itu/                        ITU official test vectors (.in / .bit / .pst)
```

### Boundary rationale

- Each DSP unit is small enough to hold in mind (roughly 50-500 LOC), has a single responsibility, and in most cases has a matching ITU intermediate test vector. This is essential for fixed-point debugging: when a final encoded frame diverges from the expected bitstream, we need to be able to isolate which block drifted.
- `internal/fixed` is the foundation. It must be rock-solid and independently tested before any higher block is written; otherwise every block's bug potentially hides a Q-format bug.
- `internal/tables` exists to isolate the large static data. Tables are transcribed from the ITU specification and verified by size and checksum; they never mix with live computation code.
- The root package `g729` holds no DSP. It only owns instance state, coordinates the per-frame flow across DSP blocks, and exposes the public API.

---

## 4. Public API

The public API is small by design: two types (`Encoder`, `Decoder`), a strict-frame core, and a streaming convenience layer.

### 4.1 Constants and errors

```go
package g729

const (
    SampleRate   = 8000 // Hz, fixed
    FrameSamples = 80   // 10 ms at 8 kHz
    FrameBytes   = 10   // 80 bits packed per frame
)

var (
    ErrShortPCM       = errors.New("g729: input PCM length not multiple of frame size (80)")
    ErrShortOutput    = errors.New("g729: output buffer too small")
    ErrShortBitstream = errors.New("g729: bitstream length not multiple of 10 bytes")
)
```

### 4.2 Strict frame API (zero-alloc core)

```go
// EncodeFrame consumes exactly 80 samples and writes exactly 10 bytes into
// out. len(pcm) != 80 returns ErrShortPCM; len(out) < 10 returns
// ErrShortOutput. Internal state is retained across calls.
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error

// DecodeFrame consumes exactly 10 bytes and writes exactly 80 samples into
// out. Mirror constraints of EncodeFrame.
func (d *Decoder) DecodeFrame(bits []byte, out []int16) error
```

ITU test-vector tests run directly against these. No allocation occurs inside either function in steady state.

### 4.3 Streaming wrapper (convenience)

```go
type Encoder struct{ /* unexported */ }

func NewEncoder() *Encoder
func (e *Encoder) Reset()

// Write accumulates PCM samples. Each time the internal buffer crosses an
// 80-sample boundary, that frame is encoded and its 10 bytes are appended
// to out. Any remaining tail (< 80 samples) is buffered for the next call.
// The returned slice is the append result; callers may pass a reusable
// backing buffer to achieve zero allocation.
func (e *Encoder) Write(pcm []int16, out []byte) ([]byte, error)

// Flush zero-pads any tail buffer (< 80 samples) to produce one final frame
// and appends its 10 bytes to out. If no tail is present, out is returned
// unchanged. Frame-to-frame predictor state (LPC, pitch, etc.) is retained;
// use Reset to fully reinitialize.
func (e *Encoder) Flush(out []byte) ([]byte, error)

type Decoder struct{ /* unexported */ }

func NewDecoder() *Decoder
func (d *Decoder) Reset()

// Decode consumes a whole number of 10-byte frames and appends the decoded
// PCM to out. No partial-frame buffering: callers align to 10-byte frames.
func (d *Decoder) Decode(bits []byte, out []int16) ([]int16, error)

// ConcealFrame synthesizes one frame (80 samples) of concealment PCM using
// the standard G.729 erasure procedure (repeat last parameters with gain
// attenuation). Call on RTP packet loss.
func (d *Decoder) ConcealFrame(out []int16) error
```

### 4.4 API conventions

- `out`-parameter-plus-returned-slice pattern matches the Go stdlib `Append*` family (`time.AppendFormat`, `strconv.AppendInt`, `hex.AppendEncode`). Callers control allocation; the library never owns output buffers.
- `Encoder` and `Decoder` contain no locks. Concurrent calls on the same instance are a data race and are documented as undefined. Multi-channel callers own one instance per channel, matching how μ-law/A-law/G.722 are used in the existing pipeline.
- No `Close` method; both types are pure value containers reclaimed by the garbage collector. `Reset` returns an instance to initial state so callers can pool instances if desired.
- No logging from the library. Errors are propagated only via return values.

---

## 5. Internal data flow

### 5.1 Encoder, one frame (10 ms, 80 samples)

```
pcm[80] int16
  │
  ▼  (1) Pre-processing         internal/pcm
  │      HPF (~140 Hz cutoff), scale int16 -> Q-format
  │
  ▼  (2) LPC analysis           internal/lpc
  │      windowed autocorrelation, lag window, Levinson-Durbin -> a[10]
  │
  ▼  (3) LSP quantization       internal/lsp
  │      a -> LSP (Chebyshev), 4-stage split VQ (18 bits), MA predictor
  │      update, dequantized a_q[10]
  │
  ▼  (per-subframe loop x2, 40 samples each)
  │
  ▼  (4) Open-loop pitch        internal/pitch   (once per frame)
  │      weighted speech, three-range autocorrelation max -> T_op
  │
  ▼  (5) Closed-loop pitch      internal/pitch
  │      fractional-lag search around T_op, adaptive codebook v[40],
  │      pitch delay index (8 + 5 bits)
  │
  ▼  (6) Target computation     internal/filter
  │      perceptual weighting, zero-input response removal,
  │      adaptive contribution removal -> fixed-codebook target x2[40]
  │
  ▼  (7) ACELP fixed-codebook   internal/acelp
  │      17-bit algebraic codebook, 4 pulses x +/-1,
  │      G.729A fast depth-first search
  │
  ▼  (8) Gain quantization      internal/gain
  │      2D VQ (7 bits), MA gain prediction, taming
  │
  ▼  (9) Memory update          within each block
  │      adaptive codebook, synthesis-filter, weighting-filter state
  │
  ▼  (end subframe loop)
  │
  ▼  (10) Bitstream pack        internal/bitstream
  │      LSP 18 | sub1: pitch 8+1 + ACELP 13+4 + gain 7 = 33
  │               sub2: pitch 5 + ACELP 13+4 + gain 7 = 29
  │       total 80 bits -> 10 bytes
  │
out[10] byte
```

### 5.2 Decoder, one frame

Substantially simpler (no search required):

1. Unpack 80 bits from `bits[10]`.
2. Dequantize LSP via MA predictor -> `a_q[10]`.
3. Per subframe:
   1. Look up adaptive codebook at pitch lag, scale by `g_p`.
   2. Reconstruct fixed-codebook pulses from positions/signs, scale by `g_c`.
   3. Sum to total excitation.
   4. Run through synthesis filter `1/A_q(z)`.
4. Apply adaptive post-filter (long-term + short-term + tilt compensation + AGC).
5. Undo pre-processing scaling; clip to `int16`.

Erasure concealment (`ConcealFrame`) reuses the last valid frame's parameters with standardized gain attenuation and LSP aging.

### 5.3 Encoder state (pre-allocated struct fields)

| Field | Size | Purpose |
|---|---|---|
| `hpIn`, `hpOut` | small | High-pass filter memory |
| `oldSpeech` | `[240]int16` | Past samples for LPC analysis window |
| `oldWspeech` | `[143]int16` | Weighted-speech history for open-loop pitch |
| `oldExc` | `[154]int16` | Adaptive codebook memory |
| `synMem` | `[10]int16` | Synthesis filter state |
| `wMem` | `[10]int16` | Weighting filter state |
| `errMem` | `[10]int16` | Zero-input response state |
| `lspOld`, `lspOldQ` | `[10]int16` x2 | LSP history |
| `pastQuaEn` | `[4]int16` | Gain predictor MA memory |
| `freqPrev` | `[4][10]int16` | LSP MA predictor memory |
| per-block scratch | varies | Reused workspace, no per-call allocation |

Total per-instance memory: roughly 1-2 KB. 1000 instances stay under 2 MB.

### 5.4 Block interface style

- Stateless computations are pure functions with explicit output buffers, never allocating: e.g. `func Autocorrelate(window []int16, out []int32)`.
- Stateful blocks are small structs owned by `Encoder`/`Decoder` as fields (composition, not embedding). Blocks never allocate in their methods.
- The root-package `Encoder`/`Decoder` is the only component that coordinates cross-block flow.

---

## 6. Error handling

G.729 DSP routines do not fail at runtime in any interesting sense. Errors are limited to contract violations at the API boundary:

| Situation | Behavior |
|---|---|
| `EncodeFrame` with `len(pcm) != 80` | Return `ErrShortPCM`. |
| Output buffer smaller than required | Return `ErrShortOutput` / `ErrShortBitstream`. |
| `DecodeFrame` with `len(bits) != 10` | Return `ErrShortBitstream`. |
| Numeric overflow inside DSP | No error. Fixed-point saturating arithmetic absorbs per spec. |
| "Invalid" bit combinations on decode | No error. G.729 defines decode for any 10-byte input; there is no invalid-frame concept at the codec level. |
| RTP packet loss | Caller invokes `ConcealFrame` explicitly. `DecodeFrame` is never called with a missing frame. |

No panics. No logging. The library never writes to stdout, stderr, or any logger.

---

## 7. Testing

Tests form a three-level pyramid. Levels 1 and 2 gate the project's "done" state; level 3 is the release gate and lives outside this repository.

### 7.1 Level 1 — Unit tests (per internal package)

- `internal/fixed`: every primitive (`L_mult`, `L_add`, `saturate`, rounding, `norm`, ...) tested against example values from the ITU basic-operations specification, including boundary values (`INT16_MAX`, `INT16_MIN`) and saturation behavior.
- `internal/lpc`, `lsp`, `pitch`, `acelp`, `gain`, `filter`: each block has golden tests, and where ITU intermediate vectors expose the block's output (see 7.2) those are used directly.
- `internal/bitstream`: pack/unpack round-trip plus ITU `.bit` format compatibility.
- `internal/tables`: size checks and checksums to catch transcription errors.

### 7.2 Level 2 — ITU test vectors (root package)

This is the completion gate. The ITU distributes per-algorithm test vectors for G.729 Annex A (`ALGTHM`, `SPEECH`, `FIXED`, `LSP`, `PITCH`, `TAME`, and a few more). Each provides:

- `*.in` — 8 kHz 16-bit PCM input.
- `*.bit` — exact bitstream the encoder must produce.
- `*.pst` — exact PCM the decoder must produce from `.bit`.

Tests:

```go
func TestITUEncoder(t *testing.T) {
    // For each vector: encode *.in via EncodeFrame and require byte-exact match to *.bit.
}

func TestITUDecoder(t *testing.T) {
    // For each vector: decode *.bit via DecodeFrame and require sample-exact match to *.pst.
}

func TestRoundTrip(t *testing.T) {
    // Helpful internal check during implementation; loose SNR threshold,
    // not a compliance test.
}
```

Vectors are committed to `testdata/itu/` so tests are reproducible without external fetches.

### 7.3 Level 3 — Interop and quality (release gate, outside this repo)

- Send live G.729 RTP to a real MRCP endpoint and confirm recognition/playback.
- Cross-decode our output with a reference decoder (e.g. bcg729 in a disposable test environment) and measure PESQ NB plus listening diagnostics against the original PCM.
- Long-running streaming stability: memory growth, allocation rate, latency distribution over hours of concurrent channels.

### 7.4 Test infrastructure

- `go test ./...` runs the full suite in under 30 seconds, including ITU vectors.
- Benchmarks: `BenchmarkEncodeFrame`, `BenchmarkDecodeFrame` report ns/op and use `b.ReportAllocs()`. A separate assertion test enforces that steady-state `EncodeFrame` / `DecodeFrame` produce zero allocations.
- `go test -race ./...` with a multi-instance test confirms the per-instance isolation contract.
- Fuzz test on `DecodeFrame` with arbitrary 10-byte inputs: must never panic or loop.

---

## 8. Phased delivery

Each phase is independently planned and executed (via the `writing-plans` skill in later steps). This spec sets only the phase boundaries and their gates.

### Phase 0 — Foundation
Primitives only; not yet a codec.

- `internal/fixed` Q-format primitives with unit tests.
- `internal/bitstream` 80-bit pack/unpack plus ITU `.bit` reader/writer.
- `internal/pcm` HPF and scaling.
- `internal/tables` populated from the ITU specification (either hand-transcribed or extracted via a script).

**Gate:** all primitive unit tests pass.

### Phase 1 — Decoder first
A complete G.729A decoder that passes sample-exact ITU vectors.

- LSP dequantization, adaptive codebook read, ACELP pulse reconstruction, gain dequantization.
- Synthesis filter, post-filter, erasure concealment.
- Public `Decoder` API.

Decoder before encoder because: (a) substantially simpler — no codebook search; (b) once available it makes encoder debugging far easier ("encode, then listen / round-trip"); (c) delivers standalone value to the ecosystem first.

**Gate:** `TestITUDecoder` passes sample-exact on all vectors.

### Phase 2 — Encoder, block by block
Use ITU's per-block intermediate vectors to stage encoder bring-up:

- **2a** LPC and LSP quantization — verify against `lsp.*` vector.
- **2b** Open-loop pitch — verify `T_op` against `pitch.*`.
- **2c** Closed-loop pitch / adaptive codebook — verify P1/P2 bit fields.
- **2d** ACELP search — verify C1/C2/S1/S2 bit fields via `fixed.*`.
- **2e** Gain quantization and taming — verify via `tame.*`.
- **2f** Full encoder — verify `speech.*` byte-exact.

Each sub-gate is a bit-exact match on just that block's fields. Full-frame bit-exactness only at 2f.

**Gate:** `TestITUEncoder` passes byte-exact on all vectors.

### Phase 3 — Public API, streaming, performance
- Streaming `Write` / `Flush` / `Reset` on `Encoder`.
- `Decoder.Decode` convenience wrapper.
- Public-facing `EncodeFrame` / `DecodeFrame` (cores already exist from phases 1-2).
- Documentation, examples, `doc.go`.

**Gate:**
- Boundary tests for partial-frame buffering (79, 81, 160 samples across `Write` calls).
- `BenchmarkEncodeFrame` reports zero allocations.
- Multi-instance benchmark confirms steady-state allocation rate stays flat.
- `go test -race ./...` passes.

### Phase 4 — Release readiness
- godoc coverage on every public symbol.
- README with usage, performance numbers, license, references.
- CI (GitHub Actions): test, race, benchmark regression.
- `v0.1.0` release.

**Gate:** project is comfortable to accept external pull requests.

### Schedule sense

Detailed estimates will go into each phase's implementation plan. Rough ordering:

- Phase 0: 1-2 weeks
- Phase 1: 2-3 weeks
- Phase 2: 4-8 weeks (longest; codebook search and fixed-point debugging concentrate here)
- Phase 3: 1 week
- Phase 4: 1 week

Roughly 2-4 months of focused full-time work, considerably more part-time.

---

## 9. References

- ITU-T Recommendation G.729 (06/2012), "Coding of speech at 8 kbit/s using conjugate-structure algebraic-code-excited linear prediction (CS-ACELP)".
- ITU-T Recommendation G.729 Annex A (11/1996), "Reduced complexity 8 kbit/s CS-ACELP speech codec".
- ITU-T G.191 Software Tools library (for `.bit` G.192 file format).
- A. M. Kondoz, "Digital Speech: Coding for Low Bit Rate Communication Systems", 2nd ed.
- A. Spanias et al., "Audio Signal Processing and Coding" / related speech coding texts.

Implementation must not consult ITU-T reference C source, bcg729, Sipro Lab code, or any other existing G.729 implementation.
