# g729

[![Go Reference](https://pkg.go.dev/badge/github.com/hunydev/g729.svg)](https://pkg.go.dev/github.com/hunydev/g729)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Website: <https://g729.huny.dev/>. The site includes listening samples and a
browser-side WebAssembly encoder/decoder demo built from the same pure-Go code.

`github.com/hunydev/g729` is a clean-room, pure-Go, MIT-licensed
G.729A-compatible codec for `G729/8000 annexb=no` RTP send paths.

- **Encoder:** independent, quality-gated, and not byte-exact to the ITU
  reference encoder or `bcg729`.
- **Decoder:** the strict path matches private ITU Annex A reference PCM
  sample-for-sample in the current verifier run (`740800/740800`).
- **Scope:** G.729 Annex A, RTP payload type 18, `ptime=10`, and `ptime=20`.
- **Not supported:** Annex B SID/CNG/DTX, G.729.1, G.729D, and G.729E.
- **Certification:** no ITU certification, endorsement, or encoder byte-exact
  conformance claim is made.

No cgo, native codec dependency, or vendored codec source is required. The
intended deployment targets are SIP/RTP, MRCP, TTS, IVR, and server-side media
applications that need `G729/8000` with `annexb=no`.
This package targets the common RTP profile `G729/8000` with `annexb=no`.
It is not a full implementation of every G.729 family variant.

---

## Project summary

`github.com/hunydev/g729` is an independent, clean-room, pure-Go
implementation compatible with the ITU-T G.729 Annex A 8 kbps
CS-ACELP speech codec, intended primarily as a server-side
encoder/decoder for RTP payload type 18 (`G729/8000`) inside MRCP /
TTS / VoIP deployments.

The codec was implemented from public ITU-T specifications and
public textbooks only. **No** ITU reference C source, `bcg729`,
Sipro Lab implementation, FFmpeg `libavcodec/g729dec.c`, or any
other extant G.729 implementation source was consulted at any
point. See the **Clean-room statement** below.

This module is the G.729 piece of a multi-codec deployment. G.711
(µ-law / A-law) and G.722 are intentionally out of scope for this
module and should live in separate codec packages in a deployment
stack.

## Why this project exists

Many practical G.729 deployments still rely on native C libraries,
GPL/commercial licensing, or platform-specific codec modules. This
package aims to provide a small, dependency-free, MIT-licensed Go codec
for server-side RTP media paths where cgo and native codec packaging are
undesirable.

---

## Supported codec scope

| Capability | v0.1.0 status |
|---|---|
| `G729/8000` RTP payload type 18 | **Encoder/send path supported for `annexb=no`; decoder supported** |
| 10 ms frame: 80 int16 samples ↔ 10 packed bytes | **Supported** |
| `ptime=10` (one frame per RTP packet) | **Supported** |
| `ptime=20` (two frames per RTP packet) | **Supported** (caller bundles two encoder outputs) |
| `annexb=no` SDP advertisement | **Required** |
| Single-stream `Encoder` / `Decoder` | **Supported** |
| Opt-in `EncoderProfileCoreFast` | **Supported as performance trade-off profile** |
| Opt-in `DecodeFrameEnhanced` listening aid | **Experimental; not a conformance claim** |
| Streaming `Encoder.Write` / `Encoder.Flush` | **Supported** |
| Hot-path 0-allocation steady state | **Verified** |
| Single-thread RTF / jitter benchmarks | **Available** |
| Decoder ITU Annex A final PCM conformance | **Sample-exact in current private oracle gate** |
| Encoder ITU byte-exact conformance | **Not claimed** |
| G.729 Annex B (SID / CNG / DTX) | **Not supported** |
| G.729.1 (wideband / scalable) | **Not supported** |
| G.729D / G.729E | **Not supported** |
| ITU certified conformance claim | **Not made** |

---

## Installation

```sh
go get github.com/hunydev/g729
```

Module is pure Go (stdlib only). Go 1.22 or newer.

---

## Usage

Minimal frame-at-a-time encode + decode:

```go
package main

import (
    "github.com/hunydev/g729"
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
- `cmd/g729rtpcheck` — raw payload / Ethernet IPv4 UDP RTP pcap validator
- `cmd/g729wasm` — Go WebAssembly wrapper used by the project website demo

Each `Encoder` and each `Decoder` is **single-threaded**. Concurrent
calls on the same instance are a data race; one instance per stream.

`EncodeFrame` and `DecodeFrame` are zero-allocation in steady state.
`DecodeFrameEnhanced` is available as an opt-in, non-strict local
listening aid. It is not used by the default decoder and is not an ITU
conformance claim.

### Real-time performance

The repository includes single-thread benchmarks for real-time streaming
capacity planning. They pin the benchmark to one OS thread, report real-time
factor (`rtf`), codec-only `streams/core`, and per-frame processing-time jitter
against the 10 ms G.729 deadline.

```sh
GOMAXPROCS=1 go test -run=^$ \
  -bench='BenchmarkThroughput_|BenchmarkRealtimeJitter_' \
  -benchmem -count=5
```

`rtf < 1.0` means the measured codec path is faster than real time. The
`streams/core` metric is a codec-only upper bound; production sizing should
apply safety margin for RTP, scheduling, jitter buffers, and co-hosted work.

See [docs/performance.md](docs/performance.md) for metric definitions and
interpretation.

### Encoder profiles

`NewEncoder()` and `NewStreamingEncoder()` use `EncoderProfileCore`, the
product default.

`EncoderProfileCoreFast` is available as an explicit opt-in performance
trade-off for high-density deployments. Diagnostic profiles are available for
PESQ experiments, A/B listening, and regression analysis. All profiles emit
normal 10-byte G.729 frames, but non-default profiles are not conformance
claims and are not recommended as the default send path.

See [docs/encoder-profiles.md](docs/encoder-profiles.md) for details.

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
builder and `cmd/g729rtpcheck` for black-box validation of raw
payload streams or Ethernet/IPv4/UDP/RTP pcap captures.

```sh
# Validate raw one-frame-per-packet payload bytes.
go run ./cmd/g729rtpcheck -mode=payload -ptime=10 -in output.g729

# Validate payload type 18 packets in a pcap and check RTP continuity.
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -in capture.pcap
```

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
Annex B SID / CNG / DTX. Receiving SID frames is not supported in
v0.1.0 and may return an error or produce invalid audio.

---

## MRCP / TTS integration note

The codec's intended deployment target is the server-side audio egress
path of MRCP-driven TTS and IVR systems: the TTS engine produces 8 kHz
int16 PCM; this module produces RTP-shaped 10-byte G.729 frames; the
MRCP/SIP framework wraps them in RTP packets and sends them to the SIP
endpoint.

Decoder side is provided for inbound audio (e.g. ASR ingress),
loopback testing, and tooling.

Validation evidence is summarized below and detailed in
[docs/validation.md](docs/validation.md).

An experimental `DecodeFrameEnhanced` path remains available for listening
diagnostics. It is non-strict and is not used as evidence for the
`G729/8000 annexb=no` product claim.

---

## Validation summary

| Area | Evidence | Claim |
|---|---|---|
| Decoder | ITU Annex A `.BIT -> local decoder -> PCM` private oracle | `740800/740800` final PCM samples exact |
| Encoder | FFmpeg black-box quality gate | Product send path is quality-gated |
| Encoder quality | Public sample regression, private PESQ NB matrix, blind listening | Diagnostic quality evidence, not certification |
| Performance | Single-thread RTF and frame-time jitter benchmarks | Real-time streaming capacity planning evidence |
| RTP | Payload type 18, `ptime=10/20`, `annexb=no` checks | Send-path interoperability confidence |
| IP | Clean-room provenance and audit records | No third-party codec source used |

For the full test matrix, opt-in commands, PESQ notes, and private oracle
workflow, see [docs/validation.md](docs/validation.md). For release wording
boundaries, see [docs/claims-and-nonclaims.md](docs/claims-and-nonclaims.md).

---

## Known scope and limitations

This release distinguishes decoder conformance evidence, encoder quality
evidence, and standards certification:

1. **Decoder final PCM exact gate passes.** Fixed ITU Annex A bitstreams
   decoded by this package match the official reference PCM sample-for-sample
   in the current private verifier run. See the validation summary for the
   current aggregate count.
2. **The decoder claim is not an ITU certification claim.** ITU has not
   certified this implementation, and no endorsement is implied.
3. **The encoder is quality-gated, not byte-exact-gated.**
   `EncoderProfileCore` emits standard 10-byte G.729 frames and passes the
   project FFmpeg black-box outbound quality gate, but it is an independent
   encoder implementation and is not expected to match the ITU reference encoder
   or `bcg729` bit-for-bit.
4. **Verifier data is not redistributed.** Private oracle files and reference
   execution CSVs are not part of the MIT-licensed source distribution and are
   not relicensed as MIT.
5. **Annex B and other G.729 variants remain out of scope.** SID/CNG/DTX
   (`annexb=yes`), G.729.1, G.729D, and G.729E are not implemented.
6. **`DecodeFrameEnhanced` is diagnostic-only.** It is useful for listening
   experiments but is not the strict decoder path and is not used as conformance
   evidence.

---

## Clean-room statement

This project maintains a clean-room constraint. No ITU reference C,
bcg729, FFmpeg, Sipro, or other G.729 implementation source was used.
Public specifications, black-box executable behavior, private numeric
oracle outputs, and independently written tests were used. v0.1.0 claims
strict decoder final-PCM sample equality against the current private ITU
Annex A oracle gate; it does not claim ITU certification, ITU endorsement,
or encoder byte-exact conformance.
See [CLEANROOM_AUDIT.md](CLEANROOM_AUDIT.md) for the engineering clean-room
audit record, [docs/similarity-review.md](docs/similarity-review.md) for
similarity claim triage, [IP_PROVENANCE.md](IP_PROVENANCE.md) for the
distribution provenance record, and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the redistribution notice
inventory.

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

MIT. See [LICENSE](LICENSE). The repository includes an engineering
clean-room audit record in [CLEANROOM_AUDIT.md](CLEANROOM_AUDIT.md), a
provenance record in [IP_PROVENANCE.md](IP_PROVENANCE.md), claims guidance in
[docs/claims-and-nonclaims.md](docs/claims-and-nonclaims.md), and a third-party
notice inventory in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---

## Development status

This is a v0.1.0 release candidate. The encoder/decoder core, RTP send-path
tooling, decoder oracle workflow, and release packaging pass the current
project gates.

The public API (`Encoder`, `Decoder`, `EncoderProfile`, `NewEncoder`,
`NewEncoderWithProfile`, `NewDecoder`, `NewStreamingEncoder`,
`NewStreamingEncoderWithProfile`, `EncodeFrame`, `DecodeFrame`, `Reset`,
`Write`, `Flush`, sentinel errors, and frame-shape constants) is intended to be
stable across the v0.1.x line.

Detailed engineering plans and historical diagnostics remain in
[`docs/superpowers/`](docs/superpowers/) for auditability, but they are not
required reading for normal use.
