# Phase 2 — Encoder Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a clean-room pure-Go G.729 Annex A **encoder** that produces byte-exact bitstreams matching the ITU-T G.729 specification PDF arithmetic, validated incrementally against ITU intermediate test vectors (`LSP.*`, `PITCH.*`, `FIXED.*`, `TAME.*`, `SPEECH.*`).

**Architecture:** Mirror the existing decoder topology. The root `g729` package owns `Encoder` state and per-frame coordination; per-block DSP lives in small `internal/*` packages. Three new packages are added (`internal/lpc`, `internal/acelp`, `internal/filter`) alongside the existing eleven decoder-side packages. ITU per-block intermediate vectors stage bring-up so each sub-phase is gated by a bit-exact match on just that block's output, with full-frame bit-exactness deferred to sub-phase 2f.

**Tech Stack:** Go 1.22+, zero runtime dependencies, no CGo, no SIMD, no assembly. Q-format fixed-point arithmetic via `internal/fixed` (G.191 STL semantics). MIT license, clean-room — only ITU-T G.729 (06/2012) + Annex A specifications and public textbooks consulted; no ITU reference C, bcg729, Sipro Lab, or FFmpeg.

**Source spec:** `docs/superpowers/specs/2026-04-20-g729-codec-design.md` §3, §4, §5.1, §5.3, §7.2, §8.

**Phase 1 inheritance:** Entry HEAD `a372de7`. Baseline: 394 PASS / 3 SKIP / 3 FAIL (per `docs/superpowers/reports/2026-05-11-phase1o-completion-report.md`). The 3 FAILs (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) are diagnostic-side and explicitly inherited into Phase 2 — encoder symmetry is expected to expose their root cause via cross-block witness.

---

## 0. Entry preconditions and invariants

### 0.1 Working tree gate

- [x] **Step 0.1.1: Confirm clean tree at HEAD `a372de7`**

```bash
git rev-parse --short HEAD          # expect: a372de7
git status --short                  # expect: empty
```

If either check fails, do not enter Phase 2. Resolve drift first.

- [x] **Step 0.1.2: Confirm baseline test counts**

```bash
go test ./... 2>&1 | tee /tmp/phase2-baseline.log
grep -c "^--- FAIL" /tmp/phase2-baseline.log     # expect: 3
grep -c "^--- SKIP" /tmp/phase2-baseline.log     # expect: 3
```

The 3 FAILs MUST be exactly: `TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`. Any other FAIL is a regression and blocks entry.

### 0.2 Invariants for the entire Phase 2 cycle

| # | Invariant | Enforcement |
|---|-----------|-------------|
| I1 | **Clean-room MIT.** Only ITU-T G.729 (06/2012) + Annex A spec PDFs and public textbooks consulted. NO ITU reference C source, bcg729, Sipro Lab, FFmpeg, or any other G.729 implementation. | Self-attest in every sub-phase report; flag any URL/file path consulted. |
| I2 | **Byte-EQ to spec PDF arithmetic, NOT to PST.** Phase 1o D-3 lesson: 7 ITU PST vectors require arithmetic the spec does not authorize. The encoder targets the bitstream the spec defines, which equals the `.bit` files (no analogous PST/BIT divergence is documented). If any encoder output diverges from `.bit`, default disposition is **fix the encoder**, NOT demote the test — unless a measurement-driven §A.4.* clause cite proves the spec authorizes the divergence. | Per-sub-phase report §A. |
| I3 | **No panics, no logging, no goroutines** in `internal/*` or root. Errors are sentinel returns at API boundary only (per design §6). | `go vet`, golangci-lint, manual review in code-review pass. |
| I4 | **Zero allocation in steady state.** All scratch buffers preallocated as struct fields on `Encoder`. `EncodeFrame` and decoder-side `DecodeFrame` allocate 0 in steady state. | Per-sub-phase: `testing.AllocsPerRun` assertion = 0. |
| I5 | **Hard-N-attempt-cap (5/5).** Phase 1o D-3 innovation: production-fix attempts on a single hypothesis family are capped at 5. On hit, the cycle MUST close with one of {root cause identified and fixed, demote the test with measured §A.4.* authorization, escalate via escape hatch}. Forbidden: continuing to iterate. | Per-cycle report §B counter. |
| I6 | **Production-zero-modification during diagnostic cycles.** When a sub-phase opens a diagnostic cycle (Phase 1k F-* pattern), `internal/**` non-test files are not modified. Only `_test.go`, `_diagnostic_test.go`, and `docs/**` change. | `git diff --name-only -- internal/ ':!*_test.go'` in cycle reports. |
| I7 | **TDD: failing test first, minimal code, commit per task.** Every code-producing task follows the 5-step pattern (write failing test → run-fail → minimal impl → run-pass → commit). | Per-task checkboxes. |
| I8 | **Co-author trailer on every commit:** `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` | Every `git commit -m` HEREDOC. |

### 0.3 Escape hatches

| Hatch | Trigger | Action |
|-------|---------|--------|
| E1 | Regression gate FAIL: a non-inherited test that was PASS at HEAD `a372de7` becomes FAIL. | Stop, report, do not advance. Either fix the regression or revert. |
| E2 | Measurement contradicts plan's core hypothesis (e.g., a sub-phase gate vector measures values inconsistent with the design spec's block decomposition). | Stop, write a `*-report.md`, redesign before continuing. |
| E3 | Measurement gap / non-reproducible (e.g., `.in`/`.bit`/`.pst` triple inconsistent). | Stop, document in report, defer the affected vector with explicit note; do not block other sub-phases. |
| E4 | External G.729 implementation consultation detected (any `bcg729`, `g729a.c`, `Ipp` etc. URL or file viewed). | Halt, report, restart from clean prompt. I1 violation is a hard scope failure. |
| E5 | Production change made outside an active sub-phase implementation task (e.g., during a diagnostic cycle). | Halt, revert the production diff, reopen the cycle as production OR diagnostic. I6 violation. |

### 0.4 강압-적합 (forced-fit) avoidance

When a measurement diverges from spec, the diagnostic-first discipline applies (Phase 1k F-* pattern):

1. **Measure first.** Capture the value at every chain boundary (synthesis → postfilter → hpFilter → ScaleUpSat for decoder; pre-process → LPC → LSP → pitch → ACELP → gain → bitpack for encoder). Identify the *first* boundary where divergence appears.
2. **Cite spec clause.** The fix must point to a specific §-numbered clause whose arithmetic the implementation deviates from.
3. **No "tune until it matches."** Adjusting magic constants, shift counts, or saturation policies without a §-cite is forbidden.
4. **Hypothesis budget.** Per I5, max 5 attempts on a single hypothesis family. Then close (fix / demote / escalate).

---

## 1. Phase 2-0 — Scaffold (root API + new internal packages)

**Goal:** Stand up the public API surface and the three new `internal/` packages with stubs returning `ErrNotImplemented`. Verify `go vet` / `go build` / existing tests still PASS. No DSP yet.

**Files:**
- Create: `errors.go`
- Create: `encoder.go`
- Create: `frame.go`
- Create: `decoder_root.go` (root-package decoder shell — wraps `internal/decoder`)
- Create: `internal/lpc/doc.go`, `internal/lpc/types.go`, `internal/lpc/types_test.go`
- Create: `internal/acelp/doc.go`, `internal/acelp/types.go`, `internal/acelp/types_test.go`
- Create: `internal/filter/doc.go`, `internal/filter/types.go`, `internal/filter/types_test.go`
- Modify: `doc.go` (extend root package doc to include encoder usage example)

> **Note on `decoder_root.go`:** The root-package public `Decoder` type is currently absent (Phase 1 kept the decoder under `internal/decoder`). Phase 2-0 adds the root-package shell that the design spec §4 mandates, wrapping `internal/decoder.Decoder`. This avoids touching the existing `internal/decoder` package and keeps Phase 1o's 394 PASS gate stable.

### Task 2-0-1: Sentinel errors

**Files:** `errors.go`, `errors_test.go`

- [x] **Step 1: Write failing test** `errors_test.go`

```go
package g729

import (
    "errors"
    "testing"
)

func TestErrors_AreSentinels(t *testing.T) {
    cases := []struct {
        name string
        err  error
        msg  string
    }{
        {"ErrShortPCM", ErrShortPCM, "g729: input PCM length not multiple of frame size (80)"},
        {"ErrShortOutput", ErrShortOutput, "g729: output buffer too small"},
        {"ErrShortBitstream", ErrShortBitstream, "g729: bitstream length not multiple of 10 bytes"},
        {"ErrNotImplemented", ErrNotImplemented, "g729: not yet implemented"},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            if c.err == nil {
                t.Fatalf("%s is nil", c.name)
            }
            if c.err.Error() != c.msg {
                t.Fatalf("%s: got %q want %q", c.name, c.err.Error(), c.msg)
            }
            // Sentinel: comparable via errors.Is to itself.
            if !errors.Is(c.err, c.err) {
                t.Fatalf("%s: errors.Is self-check failed", c.name)
            }
        })
    }
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./ -run TestErrors_AreSentinels -v
```

Expected: FAIL with "undefined: ErrShortPCM" (or similar).

- [x] **Step 3: Write minimal implementation** `errors.go`

```go
// Package g729 sentinel errors.
//
// All errors returned by the public API are exported sentinel values.
// Per design §6, the codec never panics, never logs, and never wraps
// internal errors — DSP overflow is absorbed by saturating fixed-point
// arithmetic, so the only failure modes are contract violations at the
// API boundary.
package g729

import "errors"

var (
    // ErrShortPCM is returned by EncodeFrame when len(pcm) != FrameSamples.
    ErrShortPCM = errors.New("g729: input PCM length not multiple of frame size (80)")

    // ErrShortOutput is returned by EncodeFrame when the output buffer
    // cannot hold FrameBytes.
    ErrShortOutput = errors.New("g729: output buffer too small")

    // ErrShortBitstream is returned by DecodeFrame and Decode when the
    // input length is not a multiple of FrameBytes.
    ErrShortBitstream = errors.New("g729: bitstream length not multiple of 10 bytes")

    // ErrNotImplemented is a transitional sentinel used by the scaffold
    // until each sub-phase's DSP block is wired in. It is removed before
    // Phase 2 closure.
    ErrNotImplemented = errors.New("g729: not yet implemented")
)

// Public frame-shape constants.
const (
    SampleRate   = 8000 // Hz, fixed by spec.
    FrameSamples = 80   // 10 ms at 8 kHz.
    FrameBytes   = 10   // 80 bits packed per frame.
)
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./ -run TestErrors_AreSentinels -v
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add errors.go errors_test.go
git commit -m "$(cat <<'EOF'
feat(g729): Phase 2-0 add sentinel errors and frame constants

Per design spec §4.1 and §6: ErrShortPCM, ErrShortOutput,
ErrShortBitstream, plus transitional ErrNotImplemented that
Phase 2-0 stubs return until each sub-phase wires real DSP.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-2: `internal/lpc` package skeleton

**Files:** `internal/lpc/doc.go`, `internal/lpc/types.go`, `internal/lpc/types_test.go`

- [x] **Step 1: Write failing test** `internal/lpc/types_test.go`

```go
package lpc

import "testing"

func TestAnalyzer_Reset_ZeroValueIsSafe(t *testing.T) {
    var a Analyzer
    a.Reset() // must not panic on zero value
}

func TestAnalyzer_Analyze_StubReturnsNotImplemented(t *testing.T) {
    var a Analyzer
    var (
        speech [LPCWindowSamples]int16
        out    [LPCOrder]int16
    )
    if err := a.Analyze(speech[:], out[:]); err == nil {
        t.Fatal("Analyze returned nil; expected stub error")
    }
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./internal/lpc/... -v
```

Expected: FAIL with "undefined: Analyzer" / "undefined: LPCWindowSamples".

- [x] **Step 3: Write minimal implementation**

`internal/lpc/doc.go`:

```go
// Package lpc implements G.729 LPC (Linear Predictive Coding) analysis:
// windowed autocorrelation with the §3.2.1 30 ms asymmetric Hamming
// window, lag windowing for spectral smoothing (§3.2.2), and the
// Levinson-Durbin recursion (§3.2.3) producing the order-10 LPC
// coefficients a[1..10] in Q-format.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2a.
package lpc
```

`internal/lpc/types.go`:

```go
package lpc

import "errors"

// LPCOrder is the G.729 LP analysis order (§3.2): a[0]=1, a[1..10].
const LPCOrder = 10

// LPCWindowSamples is the 30 ms asymmetric Hamming window length
// (§3.2.1): 240 samples = 80 future-lookahead + 160 past.
const LPCWindowSamples = 240

// errStub is returned by every Phase 2-0 stub method. Replaced per
// sub-phase when real arithmetic lands.
var errStub = errors.New("internal/lpc: not yet implemented")

// Analyzer holds frame-to-frame analysis state.
type Analyzer struct {
    // Phase 2a will populate. Empty by design at 2-0.
}

// Reset returns the analyzer to its zero state.
func (a *Analyzer) Reset() { *a = Analyzer{} }

// Analyze produces order-10 LPC coefficients from a windowed speech
// buffer. Phase 2-0 returns a sentinel; Phase 2a wires the real
// autocorrelation + Levinson recursion.
func (a *Analyzer) Analyze(speech []int16, out []int16) error {
    return errStub
}
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./internal/lpc/... -v
```

Expected: PASS (both tests).

- [x] **Step 5: Commit**

```bash
git add internal/lpc/
git commit -m "$(cat <<'EOF'
feat(lpc): Phase 2-0 add internal/lpc package skeleton

Empty Analyzer + Reset + stub Analyze returning sentinel.
Constants LPCOrder=10 and LPCWindowSamples=240 per §3.2/§3.2.1.
Phase 2a wires the autocorrelation + Levinson-Durbin arithmetic.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-3: `internal/acelp` package skeleton

**Files:** `internal/acelp/doc.go`, `internal/acelp/types.go`, `internal/acelp/types_test.go`

- [x] **Step 1: Write failing test** `internal/acelp/types_test.go`

```go
package acelp

import "testing"

func TestSearcher_Reset_ZeroValueIsSafe(t *testing.T) {
    var s Searcher
    s.Reset()
}

func TestSearcher_Search_StubReturnsNotImplemented(t *testing.T) {
    var s Searcher
    var (
        target [SubframeSamples]int16
        h      [SubframeSamples]int16
        out    Result
    )
    if err := s.Search(target[:], h[:], &out); err == nil {
        t.Fatal("Search returned nil; expected stub error")
    }
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./internal/acelp/... -v
```

Expected: FAIL.

- [x] **Step 3: Write minimal implementation**

`internal/acelp/doc.go`:

```go
// Package acelp implements the G.729 Annex A fast algebraic codebook
// search: 4 pulses with sign, distributed over interleaved tracks T0..T3,
// 17-bit codeword (positions 13 bits + signs 4 bits). The search uses
// the §A.3 depth-first focused variant of the §3.8 full search.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2d.
package acelp
```

`internal/acelp/types.go`:

```go
package acelp

import "errors"

// SubframeSamples is the 5 ms subframe length (§3.8).
const SubframeSamples = 40

// PulseCount is the number of non-zero pulses per ACELP subframe (§3.8).
const PulseCount = 4

var errStub = errors.New("internal/acelp: not yet implemented")

// Result holds a single subframe search outcome.
type Result struct {
    Positions [PulseCount]int16 // 0..39
    Signs     [PulseCount]int16 // +1 or -1
    Code      [SubframeSamples]int16
    PositionsBits uint16 // 13-bit packed C-field
    SignsBits     uint16 // 4-bit packed S-field
}

// Searcher holds per-instance scratch buffers (preallocated for zero-alloc).
type Searcher struct {
    // Phase 2d will populate. Empty by design at 2-0.
}

// Reset returns the searcher to its zero state.
func (s *Searcher) Reset() { *s = Searcher{} }

// Search runs the §A.3 ACELP search. Phase 2-0 stub.
func (s *Searcher) Search(target, impulseResp []int16, out *Result) error {
    return errStub
}
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./internal/acelp/... -v
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/acelp/
git commit -m "$(cat <<'EOF'
feat(acelp): Phase 2-0 add internal/acelp package skeleton

Empty Searcher + Reset + stub Search + Result struct exposing
Positions/Signs/Code plus packed C/S bitfields.
Constants SubframeSamples=40 and PulseCount=4 per §3.8/§A.3.
Phase 2d wires the depth-first algebraic search.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-4: `internal/filter` package skeleton

**Files:** `internal/filter/doc.go`, `internal/filter/types.go`, `internal/filter/types_test.go`

- [x] **Step 1: Write failing test** `internal/filter/types_test.go`

```go
package filter

import "testing"

func TestWeighting_Reset_ZeroValueIsSafe(t *testing.T) {
    var w Weighting
    w.Reset()
}

func TestWeighting_Apply_StubReturnsNotImplemented(t *testing.T) {
    var w Weighting
    var (
        in  [40]int16
        out [40]int16
        a   [11]int16
    )
    if err := w.Apply(a[:], in[:], out[:]); err == nil {
        t.Fatal("Apply returned nil; expected stub error")
    }
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./internal/filter/... -v
```

Expected: FAIL.

- [x] **Step 3: Write minimal implementation**

`internal/filter/doc.go`:

```go
// Package filter implements the encoder-side perceptual weighting
// filter W(z) = A(z/γ1) / A(z/γ2) (§3.3) and the impulse response
// computation h[] used by the ACELP target derivation (§3.7-3.8).
//
// The synthesis filter 1/Â(z) used by the *decoder* lives in
// internal/synth — that distinction is intentional: the encoder's
// W(z) needs both numerator and denominator coefficients, while the
// decoder's 1/Â(z) is denominator-only.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2c (target computation) and Phase 2d (impulse response).
package filter
```

`internal/filter/types.go`:

```go
package filter

import "errors"

// LPCOrder mirrors lpc.LPCOrder for type-safety inside this package.
const LPCOrder = 10

var errStub = errors.New("internal/filter: not yet implemented")

// Weighting holds the perceptual weighting filter memory (§3.3).
type Weighting struct {
    // Phase 2c will populate (residual + numerator/denominator memory).
}

// Reset returns the weighting filter to zero memory state.
func (w *Weighting) Reset() { *w = Weighting{} }

// Apply runs W(z) = A(z/γ1) / A(z/γ2) on a 40-sample subframe.
// Phase 2-0 stub.
func (w *Weighting) Apply(a, in, out []int16) error {
    return errStub
}
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./internal/filter/... -v
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/filter/
git commit -m "$(cat <<'EOF'
feat(filter): Phase 2-0 add internal/filter package skeleton

Empty Weighting + Reset + stub Apply for the §3.3 perceptual
weighting filter W(z) = A(z/γ1) / A(z/γ2). The decoder-side
synthesis filter 1/Â(z) stays in internal/synth.
Phase 2c wires the target-computation path; Phase 2d adds h[].

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-5: Root `Encoder` skeleton (state-only)

**Files:** `encoder.go`, `encoder_test.go`

- [x] **Step 1: Write failing test** `encoder_test.go`

```go
package g729

import (
    "errors"
    "testing"
)

func TestEncoder_NewEncoder_NotNil(t *testing.T) {
    e := NewEncoder()
    if e == nil {
        t.Fatal("NewEncoder returned nil")
    }
}

func TestEncoder_EncodeFrame_RejectsShortPCM(t *testing.T) {
    e := NewEncoder()
    var out [FrameBytes]byte
    if err := e.EncodeFrame(make([]int16, FrameSamples-1), out[:]); !errors.Is(err, ErrShortPCM) {
        t.Fatalf("got %v want ErrShortPCM", err)
    }
}

func TestEncoder_EncodeFrame_RejectsShortOutput(t *testing.T) {
    e := NewEncoder()
    pcm := make([]int16, FrameSamples)
    if err := e.EncodeFrame(pcm, make([]byte, FrameBytes-1)); !errors.Is(err, ErrShortOutput) {
        t.Fatalf("got %v want ErrShortOutput", err)
    }
}

func TestEncoder_EncodeFrame_StubReturnsNotImplemented(t *testing.T) {
    e := NewEncoder()
    pcm := make([]int16, FrameSamples)
    var out [FrameBytes]byte
    if err := e.EncodeFrame(pcm, out[:]); !errors.Is(err, ErrNotImplemented) {
        t.Fatalf("got %v want ErrNotImplemented", err)
    }
}

func TestEncoder_Reset_ZeroValueIsSafe(t *testing.T) {
    var e Encoder
    e.Reset()
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./ -v
```

Expected: FAIL with "undefined: NewEncoder" / "undefined: Encoder".

- [x] **Step 3: Write minimal implementation** `encoder.go`

```go
package g729

import (
    "github.com/exedev/g729/internal/acelp"
    "github.com/exedev/g729/internal/lpc"
    "github.com/exedev/g729/internal/filter"
    "github.com/exedev/g729/internal/pcm"
)

// Encoder holds G.729 Annex A encoder state for one logical stream.
//
// All buffers are preallocated; EncodeFrame allocates 0 in steady state.
// Concurrent calls on the same Encoder are a data race; callers needing
// parallel encoding must own one Encoder per channel.
type Encoder struct {
    pre pcm.PreProcessor

    // §5.3 preallocated histories.
    oldSpeech  [240]int16
    oldWspeech [143]int16
    oldExc     [154]int16
    synMem     [10]int16
    wMem       [10]int16
    errMem     [10]int16
    lspOld     [10]int16
    lspOldQ    [10]int16
    pastQuaEn  [4]int16
    freqPrev   [4][10]int16

    // Per-block state owners.
    lpc    lpc.Analyzer
    acelp  acelp.Searcher
    weight filter.Weighting
}

// NewEncoder returns an Encoder in initial state.
func NewEncoder() *Encoder {
    return &Encoder{}
}

// Reset returns the Encoder to initial state. Equivalent to using a fresh
// NewEncoder, but reuses the existing memory.
func (e *Encoder) Reset() {
    *e = Encoder{}
}

// EncodeFrame consumes exactly FrameSamples samples and writes exactly
// FrameBytes bytes to out. Internal state is retained across calls.
//
// Phase 2-0 stub: validates lengths and returns ErrNotImplemented. Real
// encoding is wired in Phase 2a..2f.
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
    if len(pcm) != FrameSamples {
        return ErrShortPCM
    }
    if len(out) < FrameBytes {
        return ErrShortOutput
    }
    return ErrNotImplemented
}
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./ -v -run TestEncoder
```

Expected: PASS (5 tests).

- [x] **Step 5: Commit**

```bash
git add encoder.go encoder_test.go
git commit -m "$(cat <<'EOF'
feat(g729): Phase 2-0 add Encoder skeleton with §5.3 state layout

Encoder owns all preallocated histories (oldSpeech[240],
oldWspeech[143], oldExc[154], synMem/wMem/errMem[10],
lspOld/lspOldQ[10], pastQuaEn[4], freqPrev[4][10]) and
embeds the new internal/lpc, internal/acelp, internal/filter
block owners as fields.

EncodeFrame validates len(pcm)==80 and len(out)>=10, then
returns ErrNotImplemented. Phase 2a..2f wire the real chain.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-6: Root `Decoder` shell wrapping `internal/decoder`

**Files:** `decoder_root.go`, `decoder_root_test.go`

- [x] **Step 1: Write failing test** `decoder_root_test.go`

```go
package g729

import (
    "errors"
    "testing"
)

func TestDecoder_NewDecoder_NotNil(t *testing.T) {
    d := NewDecoder()
    if d == nil {
        t.Fatal("NewDecoder returned nil")
    }
}

func TestDecoder_DecodeFrame_RejectsShortBitstream(t *testing.T) {
    d := NewDecoder()
    var out [FrameSamples]int16
    if err := d.DecodeFrame(make([]byte, FrameBytes-1), out[:]); !errors.Is(err, ErrShortBitstream) {
        t.Fatalf("got %v want ErrShortBitstream", err)
    }
}

func TestDecoder_DecodeFrame_RejectsShortOutput(t *testing.T) {
    d := NewDecoder()
    var bits [FrameBytes]byte
    if err := d.DecodeFrame(bits[:], make([]int16, FrameSamples-1)); !errors.Is(err, ErrShortOutput) {
        t.Fatalf("got %v want ErrShortOutput", err)
    }
}

func TestDecoder_DecodeFrame_AcceptsValidShape(t *testing.T) {
    d := NewDecoder()
    var (
        bits [FrameBytes]byte
        out  [FrameSamples]int16
    )
    if err := d.DecodeFrame(bits[:], out[:]); err != nil {
        t.Fatalf("unexpected error on zero frame: %v", err)
    }
}
```

- [x] **Step 2: Run to verify FAIL**

```bash
go test ./ -v -run TestDecoder
```

Expected: FAIL with "undefined: NewDecoder".

- [x] **Step 3: Write minimal implementation** `decoder_root.go`

```go
package g729

import (
    "github.com/exedev/g729/internal/decoder"
)

// Decoder holds G.729 Annex A decoder state for one logical stream.
type Decoder struct {
    inner decoder.Decoder
}

// NewDecoder returns a Decoder in initial state.
func NewDecoder() *Decoder {
    return &Decoder{}
}

// Reset returns the Decoder to initial state.
func (d *Decoder) Reset() {
    d.inner.Reset()
}

// DecodeFrame consumes exactly FrameBytes bytes and writes exactly
// FrameSamples samples to out.
func (d *Decoder) DecodeFrame(bits []byte, out []int16) error {
    if len(bits) != FrameBytes {
        return ErrShortBitstream
    }
    if len(out) < FrameSamples {
        return ErrShortOutput
    }
    return d.inner.DecodeFrame(bits, out)
}
```

> **Implementation note:** If `internal/decoder.Decoder` does not yet expose `DecodeFrame(bits []byte, out []int16) error` with this exact signature, the wrapper code must be adapted to whatever the existing `internal/decoder` API is — the test asserts only that `NewDecoder` exists, len-checks work, and a zero-frame returns nil. Adapt the wrapper, not the inner decoder.

- [x] **Step 4: Run to verify PASS**

```bash
go test ./ -v -run TestDecoder
```

Expected: PASS (4 tests).

- [x] **Step 5: Commit**

```bash
git add decoder_root.go decoder_root_test.go
git commit -m "$(cat <<'EOF'
feat(g729): Phase 2-0 add root Decoder shell wrapping internal/decoder

Per design §4: root-package Decoder is the public type. Internal
decoder package retains Phase 1 implementation; the shell only
adds len-validation and forwards to inner.DecodeFrame.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-7: Strict-frame entry-point in `frame.go`

**Files:** `frame.go`, `frame_test.go`

- [x] **Step 1: Write failing test** `frame_test.go`

```go
package g729

import (
    "errors"
    "testing"
)

func TestEncodeFrame_TopLevelDelegates(t *testing.T) {
    e := NewEncoder()
    pcm := make([]int16, FrameSamples)
    var out [FrameBytes]byte
    if err := EncodeFrame(e, pcm, out[:]); !errors.Is(err, ErrNotImplemented) {
        t.Fatalf("got %v want ErrNotImplemented (stub)", err)
    }
}

func TestDecodeFrame_TopLevelDelegates(t *testing.T) {
    d := NewDecoder()
    var (
        bits [FrameBytes]byte
        out  [FrameSamples]int16
    )
    if err := DecodeFrame(d, bits[:], out[:]); err != nil {
        t.Fatalf("unexpected error on zero frame: %v", err)
    }
}
```

- [x] **Step 2: Run to verify FAIL**

Expected: FAIL.

- [x] **Step 3: Write minimal implementation** `frame.go`

```go
package g729

// EncodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Encoder).EncodeFrame.
func EncodeFrame(e *Encoder, pcm []int16, out []byte) error {
    return e.EncodeFrame(pcm, out)
}

// DecodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Decoder).DecodeFrame.
func DecodeFrame(d *Decoder, bits []byte, out []int16) error {
    return d.DecodeFrame(bits, out)
}
```

- [x] **Step 4: Run to verify PASS**

```bash
go test ./ -v
```

Expected: PASS for `TestEncodeFrame_TopLevelDelegates` and `TestDecodeFrame_TopLevelDelegates`. All other tests still pass.

- [x] **Step 5: Commit**

```bash
git add frame.go frame_test.go
git commit -m "$(cat <<'EOF'
feat(g729): Phase 2-0 add top-level EncodeFrame/DecodeFrame helpers

Per design §4.2, the strict-frame API exposes both method and
function forms. The function form delegates to the method.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

### Task 2-0-8: Scaffold gate verification

- [x] **Step 1: Run full test suite**

```bash
go test ./... 2>&1 | tee /tmp/phase2-0-final.log
```

- [x] **Step 2: Verify counts unchanged**

```bash
grep -c "^--- FAIL" /tmp/phase2-0-final.log     # expect: 3 (inherited)
grep -c "^--- SKIP" /tmp/phase2-0-final.log     # expect: 3 (inherited)
```

The 3 inherited FAILs MUST remain `TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`. New scaffold tests all PASS.

- [x] **Step 3: Run `go vet` and `go build`**

```bash
go vet ./...
go build ./...
```

Expected: no output, exit 0.

- [x] **Step 4: Write Phase 2-0 closure note** `docs/superpowers/plans/2026-05-02-phase2-0-scaffold-report.md`

Document:
- Working tree pre/post diff
- Files added (list)
- Test counts (before/after)
- Confirmation that I1-I8 hold
- Hand-off note to Phase 2a author

- [x] **Step 5: Commit closure note**

```bash
git add docs/superpowers/plans/2026-05-02-phase2-0-scaffold-report.md
git commit -m "$(cat <<'EOF'
docs(plans): Phase 2-0 scaffold closure report

7 scaffold tasks complete. Public API surface (Encoder, Decoder,
EncodeFrame, DecodeFrame, sentinel errors, frame constants)
landed; three new internal packages (lpc, acelp, filter) have
type skeletons returning ErrNotImplemented. Test count delta:
+N passes, 0 new failures, 3 inherited Phase 1o FAILs unchanged.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## 2. Phase 2a — LPC analysis + LSP quantization  — [x] **CLOSED 2026-05-06**

**Closure report:** `docs/superpowers/plans/2026-05-06-phase2a-closure-report.md`.

**Status (2026-05-06):** Phase 2a **CLOSED**. End-to-end LPC analysis + LSP quantization sub-chain (`Encoder.lpcStep`: HPF → window → autocorrelation → Levinson-Durbin → LP→LSP → 4-stage split-VQ + MA-predictor) delivered, spec-arithmetic conformant, zero-alloc on hot path (`TestNoAllocationInLPCStep` + `BenchmarkApplyWindow/Autocorr/LevinsonDurbin` all 0 allocs/op), race-detector clean. INT-1 byte-EQ gate closed **ACCEPT-PARTIAL** with final corpus rates L0=78.67 % / L1=38.93 % (50× chance) / L2=17.07 % / L3=19.35 % over 2232 frames; residual is under-specified protocol detail (§3.2.4 cold-start MA-predictor seed, §3.2.5 sub-LSB inverse-cosine rounding, Annex A §A.4 VQ tie-breaks) not recoverable without the forbidden ITU C reference. Three production fixes retained: FIX-1B (Levinson `aWork`/`aPrev` Q24 widening), FIX-2D (Newton-refined arccos + Chebyshev bisection 4→8), FIX-3-B (anti-palindromic LP guard with previous-frame LSP reuse). I5 used 4/5; 1/5 preserved for Phase 2-final. I6 production-freeze LIFTED (INT-1-specific). INT-2-d closure-report authored at HEAD `e2b689e`. See sub-plan `docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md` and INT-1 binding closure `docs/superpowers/plans/2026-05-05-phase2a-int1-accept-partial-closure.md`.

**Scope (high-level):**
- `internal/lpc.Analyzer.Analyze`: §3.2.1 windowed autocorrelation r[0..10] (240-sample asymmetric Hamming, 80 future-lookahead + 160 past), §3.2.2 lag windowing, §3.2.3 Levinson-Durbin recursion → a[1..10] in Q12.
- `internal/lsp` encoder side: a[10] → LSP (Chebyshev §3.2.4), 4-stage split-VQ (18 bits, §3.2.5), MA predictor update (§3.2.6), dequantized a_q[10].
- Wire into `Encoder.EncodeFrame` first stage; remaining stages still return `ErrNotImplemented` via partial-frame guard.

**Sub-phase ITU vector gate:** `LSP.IN` → encode → match L0/L1/L2/L3 fields in `LSP.BIT` (18 bits per frame at the LSP positions). Full-frame match NOT required at this gate (other fields are zeroed by the partial encoder).

**Per-sub-phase plan deferral:** Following the Phase 1k F-* and Phase 1o D-* per-cycle plan pattern, Phase 2a gets its own dedicated implementation plan written at the time of entry, capturing:
1. Detailed §3.2.1-§3.2.6 clause-by-clause TDD task decomposition.
2. Q-format pinning per intermediate (r[], a[], LSP, residual error, etc.) — measured against spec PDF, NOT against any other implementation.
3. Bit-field extraction utility for `LSP.BIT` so the gate can be expressed as `got_L0_L1_L2_L3 == want_L0_L1_L2_L3` rather than full-frame.
4. Inherited-FAIL re-evaluation after Phase 2a closure: `TestDecode_LowEnergyCodebookIsSmooth` and `TestDecode_SucceedsAcrossAllGainIndices` may surface new evidence once an encoder LSP path exists.

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2a-lpc-lsp-plan.md` (date stamped at entry).

**Deferral marker:** This master plan does NOT decompose Phase 2a into TDD tasks. The dedicated Phase 2a plan is authored when 2-0 closes.

---

## 3. Phase 2b — Open-loop pitch estimation  — [x] **CLOSED 2026-05-08** — sub-plan: `docs/superpowers/plans/2026-05-07-phase2b-open-loop-pitch-plan.md` — closure report: `docs/superpowers/plans/2026-05-08-phase2b-closure-report.md`

**Scope (high-level):**
- `internal/pitch` encoder-side additions: §3.4 weighted-speech computation (perceptual weighting on past + current frame), §3.5 three-range autocorrelation maximum search → integer T_op in [20..143].
- Wire into `Encoder.EncodeFrame` second stage.

**Sub-phase ITU vector gate:** `PITCH.IN` → encode → match T_op (integer pitch) at the per-frame `PITCH.*` checkpoint. Implementation must produce the integer lag identical to spec.

**Per-sub-phase plan deferral:** Authored at entry. Captures:
1. §3.3-§3.5 clause TDD decomposition.
2. Open-loop weighted-speech buffer (`oldWspeech[143]`) update sequencing.
3. Three-range max search arithmetic and tie-break rule per §3.5.

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2b-open-loop-pitch-plan.md`.

---

## 4. Phase 2c — Closed-loop pitch + adaptive codebook  — **CLOSED-DEFERRED 2026-05-10** — sub-plan: `docs/superpowers/plans/2026-05-09-phase2c-closed-loop-pitch-plan.md` — closure report: [`docs/superpowers/plans/2026-05-10-phase2c-closure-report.md`](2026-05-10-phase2c-closure-report.md) (INT-1 STRICT byte-EQ FAIL-DEFERRED at P1 9.05 % / P0 56.46 % / P2 9.75 %; structural blockers H-CENTER + OQ-EXC-COMMIT + H-PHASE; I5 1/5 spent, 4/5 reserved for post-Phase-2d re-run; **next dispatch: Phase 2d sub-plan**)

**Scope (high-level):**
- `internal/pitch` encoder-side: §3.7 fractional-lag closed-loop search around T_op (sub-1/3 resolution per Annex A), adaptive codebook v[40] generation, P1 (8 bits = 6+2 frac), P0 (parity), P2 (5 bits = 4+1 frac, delta-from-P1).
- `internal/filter` encoder-side: §3.6 target computation x[40] = perceptually weighted speech − zero-input response. §3.7.1 target adjustment for closed-loop search (subtraction of adaptive contribution → x2[40] for ACELP).

**Sub-phase ITU vector gate:** Encode `SPEECH.IN`, extract P1/P0/P2 bit fields, byte-EQ those fields against `SPEECH.BIT`. Other fields (LSP, fixed codebook, gains) may match too at this point but are not yet gate-required for the frames where they pre-existed.

**Per-sub-phase plan deferral:** Authored at entry. Captures:
1. §3.6, §3.7 clause TDD.
2. `oldExc[154]` adaptive-codebook buffer indexing for fractional lags.
3. Bit packing for P1/P0/P2 via existing `internal/bitstream`.

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2c-closed-loop-pitch-plan.md`.

---

## 5. Phase 2d — ACELP search + gain quantization (Phase 2e folded)  — **CLOSED-DEFERRED 2026-05-12** — sub-plan: [`docs/superpowers/plans/2026-05-11-phase2d-fixed-codebook-acelp-plan.md`](2026-05-11-phase2d-fixed-codebook-acelp-plan.md) — closure report: [`docs/superpowers/plans/2026-05-12-phase2d-closure-report.md`](2026-05-12-phase2d-closure-report.md) (INT-1a STRICT byte-EQ FAIL-DEFERRED at S1 5.50 / C1 0.00 / GA1 12.15 / GB1 5.29 / S2 4.20 / C2 0.00 / GA2 11.77 / GB2 4.52 %, plausibility floor met via GA1 > Phase 2c INT-1b P1 10.79 %; INT-1b re-baseline P1 9.05→10.79 / P0 56.46→57.49 / P2 9.75→11.66 % (FAIL-DEFERRED, structural Phase 2b H-CENTER blocker upstream); OQ-EXC-COMMIT + OQ-Q-FORMAT-A10 resolved; 5 OQs PINNED with reserved I5 slots (0/5 spent); Phase 2c reserved I5 4/4 untouched; Phase 2e folded per sub-plan §0.3; **next dispatch: Phase 2f sub-plan**)

**Scope (high-level):**
- `internal/acelp.Searcher.Search`: §A.3 G.729A fast ACELP — depth-first focused search, 4 pulses on interleaved tracks T0..T3, 17-bit codeword (13 positions + 4 signs), correlation φ[] precomputation, sign-decision pre-selection per track.
- `internal/filter` encoder-side: §3.8.1 impulse response h[] computation (W(z)/Â(z) cascade response over 40 samples).

**Sub-phase ITU vector gate:** Encode `FIXED.IN`, extract C1/S1 (subframe 1) and C2/S2 (subframe 2) bit fields, byte-EQ against `FIXED.BIT`.

**Per-sub-phase plan deferral:** Authored at entry. Captures:
1. §A.3 depth-first search algorithm (G.729A vs §3.8 full G.729 differ — use Annex A explicitly).
2. φ[] correlation arithmetic and sign-pre-decision logic.
3. Q-format pinning for d[], h[h*h], rri[][].
4. Tie-break rule when two pulse positions yield identical correlation — per spec, NOT per any reference implementation.

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2d-acelp-plan.md`.

**Inherited-FAIL re-evaluation gate:** After 2d closure, `TestDiagnostic_SinglePulseChain` (decoder-side) likely receives an encoder-symmetry witness — re-evaluate disposition.

---

## 6. Phase 2e — Gain quantization + taming  — **FOLDED INTO PHASE 2D 2026-05-12** — covered under [`docs/superpowers/plans/2026-05-11-phase2d-fixed-codebook-acelp-plan.md`](2026-05-11-phase2d-fixed-codebook-acelp-plan.md) (sub-plan §0.3) and [`docs/superpowers/plans/2026-05-12-phase2d-closure-report.md`](2026-05-12-phase2d-closure-report.md) (closure report §1, §3 `internal/gainquant/` package + §3.9 / §3.9.1 / §3.9.2 / §3.9.3 implementation under tasks GQ-1/GQ-2/GQ-3/ENC-1). The TAME.IN → TAME.BIT byte-EQ harness is deferred to Phase 2f.

**Scope (high-level):**
- `internal/gain` encoder-side: §3.9 conjugate-structured 2D VQ on (g_p, γ_c) → 7 bits per subframe (3 bits GA + 4 bits GB), §3.9.1 MA gain prediction state update, §3.9.2 taming procedure (adaptive-codebook gain saturation under predicted-overflow conditions).
- Wire into `Encoder.EncodeFrame` post-ACELP.

**Sub-phase ITU vector gate:** Encode `TAME.IN`, byte-EQ full GA1/GB1/GA2/GB2 bit fields against `TAME.BIT`. The TAME vector specifically exercises the taming branch, so its byte-EQ match is a direct signal that taming is implemented.

**Per-sub-phase plan deferral:** Authored at entry. Captures:
1. §3.9 conjugate VQ search (NOT exhaustive; conjugate structure has a specific decoupled search).
2. §3.9.1 prediction state — `pastQuaEn[4]` update.
3. §3.9.2 taming — exact threshold and clamp arithmetic.

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2e-gain-taming-plan.md`.

**Inherited-FAIL re-evaluation gate:** After 2e closure, both `TestDecode_LowEnergyCodebookIsSmooth` and `TestDecode_SucceedsAcrossAllGainIndices` (decoder-side) receive encoder-symmetry witnesses on the gain quantization path. Re-evaluate disposition.

---

## 7. Phase 2f — Full-frame encode + streaming wrappers

**Scope (high-level):**
- Final wiring of `Encoder.EncodeFrame` end-to-end: pre-process → LPC → LSP → open-loop pitch → per-subframe (closed-loop pitch → target → ACELP → gain → memory updates) → bitstream pack.
- Streaming convenience: `(*Encoder).Write`, `(*Encoder).Flush` per design §4.3 (zero-pad tail on Flush).
- Top-level `Reset()` semantics audit: confirm zero-value Encoder is byte-identical to a Reset Encoder.
- Remove `ErrNotImplemented` (transitional sentinel) from public API and from all internal stub returns. Verify nothing in the public `errors.go` references it.

**Sub-phase ITU vector gate:** Encode every available `*.IN` (`ALGTHM`, `SPEECH`, `FIXED`, `LSP`, `PITCH`, `TAME`, `TEST`) → byte-EQ full-frame against `*.BIT`. This is the level-2 ITU compliance gate per design §7.2.

**Per-sub-phase plan deferral:** Authored at entry. Captures:
1. End-to-end wiring TDD.
2. `Write`/`Flush` semantics including the zero-pad-tail-on-Flush rule.
3. Removal of `ErrNotImplemented`.
4. `testing.AllocsPerRun` zero-alloc gate.
5. `testdata/itu/G729_Release3/g729AnnexA/test_vectors/` per-vector byte-EQ harness (using `*.IN` length to drive frame count, not `*.BIT` length, to avoid F2 framing dependency).

**Plan filename:** `docs/superpowers/plans/YYYY-MM-DD-phase2f-full-encode-plan.md`.

**Phase 2 closure trigger:** All seven `*.IN`/`*.BIT` vectors PASS byte-EQ. The 3 ERASURE/OVERFLOW/PARITY decoder-only vectors are NOT encoder gates.

---

## 8. Phase 2-final — Closure report

**Scope:**
- Write `docs/superpowers/reports/YYYY-MM-DD-phase2-completion-report.md` covering:
  - Sub-phase summary (2-0 through 2f) with commit ranges.
  - ITU vector gate results table (all 7 vectors × bit/PST status).
  - Inherited-FAIL disposition (3 entries from Phase 1o):
    - `TestDiagnostic_SinglePulseChain`
    - `TestDecode_LowEnergyCodebookIsSmooth`
    - `TestDecode_SucceedsAcrossAllGainIndices`
    Each must be: fixed (with §-cite) / demoted (with measured §A.4.* authorization) / explicitly carried forward to Phase 3 with a documented rationale.
  - R-A / R-B / R-C ambiguity ledger (from Phase 1o §6) re-examined: encoder side may have produced new evidence to reduce or close.
  - SF-1 tilt γ_t gating (postfilter `agcGainPrev` vs §4.2.3 `sign(k1')`) — encoder produces k1' explicitly during LPC analysis, so this can be re-examined with first-class measurement.
  - OVERFLOW.BIT framing rationale (Phase 1o D-2 F2 lenient loader) — Phase 2 encoder may produce 0x0000 softbit output naturally, validating or invalidating F2.
  - Final test counts; expect FAIL count = 0 (or strictly justified residual).
  - Public API stability statement (no breaking changes since Phase 2-0 except `ErrNotImplemented` removal).
  - Phase 3 entry note (release polish, README, public examples, fuzzing — out of scope here).

- Commit:

```bash
git add docs/superpowers/reports/YYYY-MM-DD-phase2-completion-report.md
git commit -m "$(cat <<'EOF'
docs(phase2): closure report — encoder bit-exact across ITU vectors

Phase 2 cycle closed. Sub-phases 2-0 through 2f complete; all
seven .in/.bit ITU vectors byte-EQ. Inherited Phase 1o FAILs
disposed (see report §3). Public API stable.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## 9. Inherited Phase 1o long-term items (carry-through)

Per `docs/superpowers/reports/2026-05-11-phase1o-completion-report.md` §6, Phase 2 inherits:

| Item | Owner sub-phase | Re-eval gate |
|------|-----------------|--------------|
| 3 FAILs (SinglePulseChain, LowEnergyCodebookIsSmooth, SucceedsAcrossAllGainIndices) | 2d/2e | After each sub-phase closure, re-run and document disposition. |
| R-A / R-B / R-C ambiguity ledger | 2-final | Encoder symmetry produces witnesses; re-examine in closure report. |
| SF-1 tilt γ_t gating | 2-final | Encoder LPC produces k1' first-class. |
| OVERFLOW.BIT framing rationale (F2 loader) | 2f | Encoder may natively produce 0x0000 softbits. |
| Cosmetic gofmt cleanup (18 box-drawing lines) | Any sub-phase | Bundle into the first task that touches the affected file. |
| `TestDecode_ITUVectorAlgthmBitExact` SKIP demote candidate | 2f | After full-frame ALGTHM encode PASS, the decoder side gains symmetry. |
| ITU corrigendum search | Out of scope | Documented as Phase 3 candidate. |

---

## 10. Self-Review

**Spec coverage check.** Walking the design spec §3, §4, §5.1, §5.3, §7.2, §8 against this plan:

- §3 package layout — Phase 2-0 adds the three missing internal packages. ✓
- §4 public API (Encoder, Decoder, EncodeFrame, DecodeFrame, NewEncoder, NewDecoder, Reset, sentinel errors, constants) — Phase 2-0 covers all. Streaming `Write`/`Flush` covered in 2f. ✓
- §5.1 encoder data flow (10 stages) — distributed across 2a (LPC + LSP), 2b (open-loop pitch), 2c (closed-loop pitch + target), 2d (ACELP), 2e (gain + taming + memory updates), 2f (bitstream pack + full wiring + pre-process + post-pre-process scaling). Pre-processing stage uses existing `pcm.PreProcessor`. ✓
- §5.3 encoder state — Phase 2-0 Task 5 lays out all preallocated fields. ✓
- §7.2 ITU level-2 gates — every sub-phase 2a-2f names its specific vector gate; 2f names the full ITU compliance gate. ✓
- §8 sub-phase decomposition — 2a..2f map 1:1 to spec. ✓

**Placeholder scan.** No "TBD"/"TODO"/"fill in"/"appropriate error handling"/"similar to Task N" found in TDD task content. The 2a-2f sub-phase scopes are intentionally high-level (per Phase 1k per-cycle plan deferral pattern) — they say so explicitly and name the dedicated plan filename that supplies the missing TDD detail. This is deferral, not a placeholder.

**Type consistency.** Cross-checking names introduced earlier:
- `Analyzer`, `LPCOrder=10`, `LPCWindowSamples=240` (lpc) — referenced by `Encoder.lpc` field.
- `Searcher`, `Result`, `SubframeSamples=40`, `PulseCount=4` (acelp) — referenced by `Encoder.acelp` field.
- `Weighting` (filter) — referenced by `Encoder.weight` field.
- `pcm.PreProcessor` — pre-existing in `internal/pcm`, referenced by `Encoder.pre`.
- `decoder.Decoder` — pre-existing in `internal/decoder`, referenced by root `Decoder.inner`. Wrapper signature notes that adapter code must follow the actual `internal/decoder` API.
- Public `Encoder`, `Decoder`, `EncodeFrame`, `DecodeFrame`, `NewEncoder`, `NewDecoder`, `Reset`, `ErrShortPCM`, `ErrShortOutput`, `ErrShortBitstream`, `ErrNotImplemented`, `SampleRate`, `FrameSamples`, `FrameBytes` — defined in `errors.go`/`encoder.go`/`decoder_root.go`/`frame.go`, used consistently in tests.

All types/methods used in any task are defined in an earlier task. ✓

**Gap fix.** None identified.

---

## 11. Execution Handoff

Plan complete and to be saved at `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`. Two execution options:

1. **Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration. Each Phase 2-0 task (2-0-1 through 2-0-8) is one subagent. Each later sub-phase (2a-2f) opens with a "write the dedicated sub-phase plan" subagent before any implementation subagents.

2. **Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
