# g729

Pure-Go G.729A-compatible 8 kbps speech codec for MRCP / TTS RTP send paths.

**Status: v0.1.0-rc preparation — clean-room, MIT-licensed.**

---

## Project summary

`github.com/exedev/g729` is an independent, clean-room, pure-Go
implementation of the ITU-T G.729 Annex A 8 kbps CS-ACELP speech
codec, intended primarily as a server-side encoder/decoder for
RTP payload type 18 (`G729/8000`) inside MRCP / TTS / VoIP
deployments.

The codec was implemented from public ITU-T specifications and
public textbooks only. **No** ITU reference C source, `bcg729`,
Sipro Lab implementation, FFmpeg `libavcodec/g729dec.c`, or any
other extant G.729 implementation source was consulted at any
point. See the **Clean-room statement** below.

This module is the G.729 piece of a multi-codec deployment. G.711
(µ-law / A-law) and G.722 are deliberately not included here —
those are trivially implementable from their own published
specifications and live in their own modules in a typical
deployment stack.

---

## Supported codec scope

| Capability | v0.1.0 status |
|---|---|
| `G729/8000` RTP payload type 18 | **Supported** |
| 10 ms frame: 80 int16 samples ↔ 10 packed bytes | **Supported** |
| `ptime=10` (one frame per RTP packet) | **Supported** |
| `ptime=20` (two frames per RTP packet) | **Supported** (caller bundles two encoder outputs) |
| `annexb=no` SDP advertisement | **Required** |
| Single-stream `Encoder` / `Decoder` | **Supported** |
| Streaming `Encoder.Write` / `Encoder.Flush` | **Supported** |
| Hot-path 0-allocation steady state | **Verified** |
| ITU reference byte-exact conformance | **Not claimed** (see Known limitations) |
| ITU vector full byte-EQ | **Not claimed** |
| G.729 Annex B (SID / CNG / DTX) | **Not supported** |
| G.729.1 (wideband / scalable) | **Not supported** |
| G.729D / G.729E | **Not supported** |
| ITU certified conformance claim | **Not made** |

---

## Installation

```sh
go get github.com/exedev/g729
```

Module is pure Go (stdlib only). Go 1.22 or newer.

---

## Usage

Minimal frame-at-a-time encode + decode:

```go
package main

import (
    "github.com/exedev/g729"
)

func main() {
    enc := g729.NewEncoder()
    dec := g729.NewDecoder()

    pcmIn := make([]int16, g729.FrameSamples) // 80 samples = 10 ms @ 8 kHz
    bits := make([]byte, g729.FrameBytes)     // 10 bytes
    pcmOut := make([]int16, g729.FrameSamples)

    // Fill pcmIn from your audio source (8 kHz mono int16) ...

    if err := enc.EncodeFrame(pcmIn, bits); err != nil {
        panic(err)
    }
    if err := dec.DecodeFrame(bits, pcmOut); err != nil {
        panic(err)
    }
}
```

See [`examples/`](examples/) for fuller programs:

- `examples/encode_pcm` — raw PCM int16 LE 8 kHz mono → G.729 frames
- `examples/decode_g729` — G.729 frames → raw PCM int16 LE 8 kHz mono
- `examples/streaming_encode` — `NewStreamingEncoder` + `Write` + `Flush`
- `examples/rtp_packetize` — illustrative RTP payload packetization

Each `Encoder` and each `Decoder` is **single-threaded**. Concurrent
calls on the same instance are a data race; one instance per stream.

`EncodeFrame` and `DecodeFrame` are zero-allocation in steady state.

---

## RTP packetization

G.729 frames are 10 ms each (80 samples / 10 bytes). RFC 3551
assigns static payload type 18 to `G729/8000`.

For `ptime=10`, one G.729 frame is the RTP payload (10 bytes).

For `ptime=20`, the sender concatenates two consecutive 10-byte
encoder outputs into a single 20-byte RTP payload; the receiver
hands them to two consecutive `DecodeFrame` calls.

This module does not implement RTP framing itself — the caller
owns RTP header / sequence-number / timestamp generation. See
`examples/rtp_packetize/main.go` for an illustrative payload
builder.

---

## SDP examples

`ptime=10`, single G.729 frame per RTP packet:

```
m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:10
a=maxptime:10
```

`ptime=20`, two G.729 frames bundled per RTP packet (20 bytes payload):

```
m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:20
a=maxptime:20
```

`annexb=no` MUST be advertised — this codec does not implement
Annex B SID / CNG / DTX. Receiving SID frames is undefined
behaviour in v0.1.0.

---

## MRCP / TTS integration note

The codec's intended deployment target is the server-side audio
egress path of MRCP-driven TTS and IVR systems: the TTS engine
produces 8 kHz int16 PCM; this module produces RTP-suitable 10-byte
G.729 frames; the MRCP/SIP framework wraps them in RTP packets and
sends them to the SIP endpoint.

Decoder side is provided for inbound audio (e.g. ASR ingress),
loopback testing, and tooling. The decoder is verified
spec-correct against ITU-T G.729 (06/2012) + Annex A across seven
independent diagnostic axes (see Phase 3-final closure report),
but does not byte-match the ITU `SPEECH.PST` reference — see
Known limitations.

---

## Known limitations

This release does not claim ITU byte-exact conformance. The decoder
is verified spec-correct against ITU-T G.729 (06/2012) + Annex A
across seven independent diagnostic axes, but does not byte-match
the ITU PST reference. See
[Phase 3-final closure report](docs/superpowers/plans/2026-05-04-phase3-final-closure-report.md)
for full numerical evidence.

Concretely:

1. **Pipeline B SegSNR ≈ −0.90 dB** vs `SPEECH.PST`. Decoder is
   spec-correct on seven axes, but the reference vector divergence
   is structural and not localizable to any spec-defined defect
   under the clean-room constraint. Not a release blocker for the
   v0.1.0 RTP send-path use case.
2. **4 encoder byte-EQ FAIL-DEFERRED pins** (Phase 2c / 2d / 2f
   conformance backlog). Tracked independently; not affecting the
   audio output of `EncodeFrame` for the supported scope. Excluded
   from the default test suite via the `conformance` build tag.
3. **4 decoder PSTdomain PASS-by-design FAIL pins** (Phase 1o D-3,
   sample 40-41 drift). Documented; identical pre/post Phase 3.
   Excluded from the default test suite via the `diagnostic` build
   tag.
4. **1 `TestDiagnostic_SinglePulseChain`** — diagnostic-only
   instrumentation log retained for future reference. Excluded from
   the default test suite via the `diagnostic` build tag.
5. **`internal/gain/legacy_gcq12.go`** — test-only adapter retained;
   non-blocking housekeeping item.

### Test suite layout

The repository ships three test layers, each with a distinct release
gate role:

| Suite | Invocation | Release gate role |
|---|---|---|
| Default (release) | `go test ./...` | **Binding.** Must PASS at the v0.1.0-rc1 tag commit. |
| Conformance (informational) | `go test -tags=conformance ./...` | Non-blocking. Currently expects 4 documented FAIL-DEFERRED items; new failures must be triaged. |
| Diagnostic (informational) | `go test -tags=diagnostic ./...` | Non-blocking. Currently expects 5 documented diagnostic / drift-monitoring FAILs (4 PSTdomain pins + 1 SinglePulseChain). |

The conformance and diagnostic suites do **not** block release;
their expected-failure inventories are catalogued in
[`docs/releases/v0.1.0-rc1-checklist.md`](docs/releases/v0.1.0-rc1-checklist.md).

---

## Clean-room statement

This project maintains a clean-room constraint. No ITU reference C,
bcg729, FFmpeg, Sipro, or other G.729 implementation source was used.
Public specifications, test vectors, and independently written tests
were used. v0.1.0 does not claim ITU byte-exact conformance.

Permitted reference materials, used during development:

- ITU-T Recommendation G.729 (06/2012), main body PDF
- ITU-T Recommendation G.729 Annex A (06/2012), PDF
- Salami, Laflamme, Adoul, Massaloux (1998), *Description of ITU-T
  Recommendation G.729 Annex A*, IEEE Transactions on Speech and
  Audio Processing, §V.B
- Kondoz (2004), *Digital Speech*, §6 (CS-ACELP)
- Chu (2003), *Speech Coding Algorithms*, LP analysis chapter
- Goldberg & Riek (2000), *A Practical Handbook of Speech Coders*
- Quackenbush, Barnwell, Clements (1988), *Objective Measures of
  Speech Quality*
- Oppenheim & Schafer, *Discrete-Time Signal Processing* (3rd ed.)

Forbidden sources, never consulted at any point:

- ITU C reference source files (`g729a.c`, `cb_search.c`, `dec_gain.c`,
  any other reference distribution file)
- `bcg729` (Belledonne Communications)
- Sipro Lab implementations
- FFmpeg G.729 decoder (`libavcodec/g729dec.c`)
- Any other extant G.729 implementation

Each diagnostic and design note in `docs/superpowers/diagnostics/`
and `docs/superpowers/plans/` carries its own per-document I1
declaration with citation list.

---

## License

MIT. See [LICENSE](LICENSE).

---

## Development status

- **Phase 0 / 1 / 2** — encoder/decoder core implementation, completed.
  See `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`
  (master plan).
- **Phase 3 — CLOSED-PARTIAL** at HEAD `56a0ec3`. Decoder spec-correct
  across seven independent diagnostic axes; ships under spec-compliance
  binding criterion. See
  [Phase 3-final closure report](docs/superpowers/plans/2026-05-04-phase3-final-closure-report.md).
- **Phase 4 — ACTIVE**. Release packaging cycle for v0.1.0-rc1. See
  [Phase 4 plan](docs/superpowers/plans/2026-05-04-phase4-v0.1.0-release-packaging-plan.md).

This is a release candidate. The public API (`Encoder`, `Decoder`,
`NewEncoder`, `NewDecoder`, `NewStreamingEncoder`, `EncodeFrame`,
`DecodeFrame`, `Reset`, `Write`, `Flush`, sentinel errors, frame-shape
constants) is intended to be stable across the v0.1.x line.
