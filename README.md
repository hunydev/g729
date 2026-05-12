# g729

[![Go Reference](https://pkg.go.dev/badge/github.com/hunydev/g729.svg)](https://pkg.go.dev/github.com/hunydev/g729)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Website: <https://g729.huny.dev/>. The site includes listening samples and a
browser-side WebAssembly encoder/decoder demo built from the same pure-Go code.

Pure-Go, MIT-licensed G.729A-compatible speech codec for RTP
`G729/8000` send paths.

This project provides a clean-room Go implementation of a
G.729A-compatible encoder and decoder with no cgo, no native
dependencies, and no vendored codec source. It is intended for SIP/RTP,
MRCP, TTS, IVR, and server-side media applications that need
`G729/8000` with `annexb=no`.

**Status: v0.1.0-rc1.** The outbound encoder/RTP send path is
black-box tested against FFmpeg. The decoder is included for loopback,
tooling, and limited inbound regression testing. Broad interoperability
certification and ITU byte-exact conformance are not claimed.

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
| `G729/8000` RTP payload type 18 | **Encoder/send path supported for `annexb=no`; decoder included with limited interoperability coverage** |
| 10 ms frame: 80 int16 samples ↔ 10 packed bytes | **Supported** |
| `ptime=10` (one frame per RTP packet) | **Supported** |
| `ptime=20` (two frames per RTP packet) | **Supported** (caller bundles two encoder outputs) |
| `annexb=no` SDP advertisement | **Required** |
| Single-stream `Encoder` / `Decoder` | **Supported** |
| Opt-in `DecodeFrameEnhanced` listening aid | **Experimental; not a conformance claim** |
| Streaming `Encoder.Write` / `Encoder.Flush` | **Supported** |
| Hot-path 0-allocation steady state | **Verified** |
| ITU reference byte-exact conformance | **Not claimed** (explicit decoder vector gate exists; see Known limitations) |
| ITU vector full byte-EQ | **Not passing yet** |
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

### Encoder profiles

`NewEncoder()` and `NewStreamingEncoder()` use `EncoderProfileCore`. This
profile emits normal 10-byte G.729 frames and is the current product default
because the latest blind-listening gate preferred it over both the previous
PESQ-led default and the `bcg729` black-box anchor. It is not an ITU byte-exact
encoder claim.
The Core open-loop path follows Annex A's raw-correlation per-range maxima
before the normalized three-range merge, and range-3 override checks every
lower-range submultiple with the Core `11/10` lift instead of only the current
pairwise winner.
Core closed-loop pitch refinement also evaluates the encodable P1/P2 fractional
boundary codepoints rather than silently restricting the search to only the
three fractions around the integer winner.
Its fixed-codebook path uses the K3=0.4 focused threshold search from §3.8.1
with a 180-entry frame cap across the two subframes; subframe 0 is
conservatively capped at 90 so it cannot consume the whole frame budget. Its
LSP VQ path uses the sequential Annex A search rather than the broader
diagnostic second-stage search. Its gain predictor keeps the wider int32
§3.9.1 math path and the Annex A GA/GB preselection search.

`EncoderProfileQualityPESQ` remains available as a numeric diagnostic profile.
It keeps the broader historical Quality heuristic surface disabled, and enables
native reconstructed-gain residual search, gain clip repair, and fixed-codebook
residual reranking. It has strong PESQ NB scores on the current sample set, but
the latest blind tests found it slightly more muffled than Core, so it is no
longer the default.
`EncoderProfileQuality` remains available as the older broad quality-heuristic
profile for diagnostics. Quality uses the sequential Annex A LSP VQ path by
default; the broader second-stage LSP search remains available only as an
internal diagnostic knob.
`EncoderProfileQualityAnnexALSP` is kept as an explicit alias for that LSP path
in listening diagnostics.
`EncoderProfileQualityClean` is a listening-diagnostic profile for comparing a
smoother, bcg729-like candidate against the default Quality profile. It keeps
the standard 10-byte payload shape, disables normalized closed-loop pitch
reranking, and uses stricter, high-residual-aware decoder-in-loop gain MSE
repair.
`EncoderProfileQualityCleanSNR` keeps the same pitch policy but uses the older
high-SNR gain-repair preference for clarity A/B tests.
`EncoderProfileQualityCleanSmooth` lowers the clean repair threshold and biases
more strongly toward high-residual reduction for bitstream-level smoothing
diagnostics.
`EncoderProfileQualityCleanVoiced` keeps the clean pitch policy while allowing
decoder-in-loop gain repair to prefer slightly higher adaptive gain when the
objective-score tradeoff remains bounded.
`EncoderProfileQualityCleanDegrit` keeps the clean pitch policy while allowing
gain repair to prefer lower fixed-codebook gain correction when adaptive gain
is not reduced.
`EncoderProfileQualityCleanHarmonic` keeps the clean pitch policy while
allowing voiced gain repair to trade bounded score loss for higher adaptive
gain with lower fixed-codebook correction.
`EncoderProfileQualityCleanHarmonicStrong` pushes that same gain-balance
tradeoff harder for grit-vs-muffling A/B tests.
`EncoderProfileQualityCleanHarmonicDeep` pushes the same gain-balance tradeoff
beyond the strong candidate to locate the grit-vs-muffling boundary.
`EncoderProfileQualityCleanFCBRerank` keeps the clean pitch policy while
reranking a small fixed-codebook candidate set with decoder-in-loop residual
scoring for grit/noise listening diagnostics.
For clean-room algorithm comparisons, use
`NewEncoderWithProfile(EncoderProfileCore)` or
`NewStreamingEncoderWithProfile(w, EncoderProfileCore)`. The core profile
disables those local quality heuristics while preserving the same public frame
shape and decoder compatibility. The Core preselect center solve preserves the
maximum zero-allocation-safe correlation precision used by this implementation,
but it still keeps Annex A's 4x8 GA/GB preselect breadth. Diagnostic quality
profiles may evaluate all 128 standard gain-index pairs using the exact
reconstructed-gain residual before applying decoder-in-loop clipping repair.

`EncoderProfileCoreClipRepair` is a listening-diagnostic variant of Core. It
keeps the Core LSP/FCB/gain-preselect policy and only adds decoder-in-loop gain
clip repair with a lower pre-clip threshold. Use it for A/B tests of the Core
sound with a minimal clipping safety valve, not as a conformance claim.

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

Current status: the outbound TTS/RTP send path now passes the binding
FFmpeg black-box encoder quality gate for `G729/8000 annexb=no`
payloads. This is not an ITU byte-exact or certification claim. The
strict local decoder also passes the current FFmpeg black-box regression
gates for this repository's local encoder payload and a local,
non-redistributed Asterisk-origin `.g729` payload sample. That is enough for current
tooling and loopback confidence, but it is still not broad
interoperability certification for every external G.729 sender.

An experimental `DecodeFrameEnhanced` path remains available for listening
diagnostics. It is non-strict and is not used as evidence for the
`G729/8000 annexb=no` product claim.

---

## Known limitations

This release does not claim ITU byte-exact or certified G.729
conformance. The outbound encoder/RTP send path is now black-box gated
against FFmpeg, and the strict local decoder is black-box gated against
FFmpeg for the local encoder payload plus a local, non-redistributed
Asterisk-origin payload sample.

Concretely:

1. **FFmpeg black-box encoder gate passes.** On the ITU SPEECH corpus,
   `SPEECH.BIT -> ffmpeg` tracks `SPEECH.PST` at about `GlobalSNR=7.04
   dB`, `SegSNR=4.39 dB`, while `SPEECH.IN -> our encoder -> ffmpeg`
   currently measures about `GlobalSNR=7.12 dB`, `SegSNR=4.61 dB`.
   The deltas (`+0.08 dB` global, `+0.22 dB` segmental) pass the
   project-defined `>= -2.00 dB` release gate for outbound encoder
   quality.
2. **Local decoder roundtrip gate passes against FFmpeg.** On the local
   encoder's own SPEECH payload, `our encoder -> local decoder` now tracks
   `our encoder -> ffmpeg` at about `GlobalSNR=16.12 dB`,
   `SegSNR=14.41 dB`, and RMS ratio `0.992` local-vs-FFmpeg. The
   end-to-end source quality is still bounded by the outbound encoder gate,
   not by ITU byte-exact vector certification.
3. **Local Asterisk payload decoder gate passes against FFmpeg.**
   The strict local decoder now tracks FFmpeg on a local, non-redistributed
   Asterisk-origin `.g729` payload sample at about `GlobalSNR=14.64 dB`,
   `SegSNR=15.23 dB`, `corr=0.983`, and RMS ratio `0.985`. This is a
   useful inbound regression gate for MRCP/SIP integration, but it is not a
   blanket claim that arbitrary external G.729 payloads from every sender
   have been exhaustively qualified. The non-strict enhanced listening path
   is currently worse than strict on this gate and is not conformance
   evidence.
4. **0 encoder byte-EQ expected failures** in the conformance suite.
   The LSP vector, TAME byte-EQ, former Phase 2c closed-loop pitch,
   and Phase 2d FCB pins now pass as source-divergence diagnostics
   after clean-room numeric handoff audits. These measurements remain
   informational and are not sufficient to certify audio quality.
   Excluded from the default test suite via the `conformance` build tag.
5. **Decoder ITU vector exact gate is explicit and not passing yet.**
   For decoder credibility, the direct gate is fixed ITU `.BIT` payloads
   decoded by this repository compared sample-by-sample against companion
   `.PST` reference PCM, not PESQ/MOS. The opt-in matrix currently shows
   `0.00%` exact frames and `7.14%` exact samples across the Annex A
   ordinary-good vector scope, so this remains the main decoder
   conformance blocker. Older PSTdomain PASS-by-design diagnostic pins are
   retained as history, not as a public conformance claim.
6. **`TestDiagnostic_SinglePulseChain`** is retained as a
   diagnostic-only instrumentation log and currently PASSes. Excluded
   from the default test suite via the `diagnostic` build tag.

### ITU decoder vector validation

Decoder validation is stronger when it uses fixed ITU bitstreams:
`.BIT -> local decoder -> PCM` compared directly against the companion
`.PST` reference PCM. PESQ/POLQA/MOS are useful listening-quality metrics,
but they are secondary for decoder conformance because the input bitstream is
already fixed.

Run the current sample-level matrix:

```sh
G729_DECODER_ITU_VECTOR_VALIDATION=1 \
go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
```

When the matrix reaches exact equality, promote it to a hard gate:

```sh
G729_DECODER_ITU_VECTOR_VALIDATION=1 \
G729_REQUIRE_DECODER_ITU_VECTOR_EXACT=1 \
go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
```

To localize the first or largest vector divergence without adding a default
test cost:

```sh
G729_DECODER_ITU_VECTOR_TRACE=1 \
G729_DECODER_ITU_VECTOR_TRACE_VECTOR=ALGTHM \
G729_DECODER_ITU_VECTOR_TRACE_MODE=worst-frame \
go test ./internal/decoder -run TestDecoderITUVectorFirstDiffTrace -count=1 -v
```

The default scope is `annexa-good` (`ALGTHM`, `SPEECH`, `FIXED`, `LSP`,
`PITCH`, `TAME`, `TEST`, `OVERFLOW`). Set
`G729_DECODER_ITU_VECTOR_SCOPE=all` to include `ERASURE` and `PARITY`, which
belong to the separate bad-frame concealment/parity behavior surface.

See `docs/superpowers/diagnostics/2026-05-12-decoder-vector-validation.md`
for the current matrix and why this gate supersedes PESQ for decoder
credibility.

### FFmpeg black-box quality gate

The quality gate uses FFmpeg only as an external decoder executable;
no external implementation source is inspected.

```sh
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v
```

The gate passes when the local encoder decode quality is within 2 dB of
the `SPEECH.BIT -> ffmpeg` reference path on both global SNR and
segmental SNR.

The inbound/local decoder Asterisk sample gate is intentionally separate
from the outbound encoder claim:

```sh
G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v
```

At this checkpoint the strict Asterisk sample gate passes. The enhanced
Asterisk listening gate is non-strict and is not part of the default
decoder/inbound conformance boundary:

```sh
G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
G729_REQUIRE_ENHANCED_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v
```

The local decoder gate for the local encoder stream is also separate
from the passing outbound encoder claim:

```sh
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v
```

At this checkpoint the strict local decoder gate passes. The enhanced
local listening gate is non-strict and does not change the default decoder
conformance boundary:

```sh
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_ENHANCED_LOCAL_DECODER_FFMPEG_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v
```

For a user-provided problem sample, run the opt-in external sample
diagnostic. FFmpeg-readable inputs such as WAV/MP3/M4A are converted to
8 kHz mono signed 16-bit PCM through the local FFmpeg executable; raw
`.pcm`, `.raw`, `.sln`, `.s16le`, and `.in` files are assumed to already
be 8 kHz mono signed little-endian int16 PCM.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
go test -run TestExternalSampleQualityDiagnostic -count=1 -v
```

If `G729_EXTERNAL_SAMPLE_QUALITY` is unset, the diagnostic first checks
for the current local problem sample at
`testdata/external/user_quality_audio.m4a`, then the legacy
`user_quality_input.*` samples.

This prints `input -> our encoder -> ffmpeg`,
`input -> our encoder -> local`, and `local decoder vs ffmpeg` on the
same aligned SNR scale used by the web and release diagnostics.

To reproduce the PESQ-led encoder candidate matrix, run:

```sh
G729_PESQ_PYTHON=/tmp/g729-pesq-venv/bin/python \
G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 \
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v
```

See `docs/superpowers/diagnostics/2026-05-12-pesq-candidate-status.md` for
the PESQ candidate evidence, web-app checks, and the later blind-listening
decision that returned the product default to `EncoderProfileCore`.

To separate clipping from "muffled" spectral-shape complaints, run the
spectral tilt diagnostic. It compares source, `EncoderProfileCore`,
`EncoderProfileQuality`, and the local `bcg729` black-box executable by
band-energy share and high/mid tilt; setting
`G729_EXTERNAL_SAMPLE_SPECTRAL_ABLATION=1` also prints selected quality
heuristic splits.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_SPECTRAL_TILT=1 \
go test -run TestExternalSampleSpectralTiltDiagnostic -count=1 -v
```

To compare quality variants around a specific audible artifact, run the
focused-window diagnostic. By default it measures frames `286:312`
(`2.860s..3.130s` at 8 kHz); override the frame range with
`G729_EXTERNAL_SAMPLE_QUALITY_WINDOW_FRAMES=start:end`.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_QUALITY_WINDOW=1 \
go test -run TestExternalSampleQualityWindowDiagnostic -count=1 -v
```

For Core-vs-Quality encoder search work, prefer the production-state
gain-mode diagnostic over the older gain-scale sweep helpers. It keeps the
same closed-loop and commit state shape as the encoder profiles, then varies
only the gain-search mode.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_MODE=1 \
go test -run TestExternalSampleProductionGainModeDiagnostic -count=1 -v
```

The current problem-sample result shows that full native gain search can
approach the `bcg729` black-box SNR, but it reintroduces near-clips without
the Quality profile's decoder-in-loop repair. That makes it a Quality
heuristic, not a Core spec-alignment fix.

For the remaining Core near-clip localization, the patch-matrix diagnostic can
be run in Core mode. It edits transmitted fields after encoding and decodes
with FFmpeg, so it is a numeric diagnostic only, not a production fix.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_CORE_FFMPEG_PATCH_MATRIX=1 \
go test -run TestExternalSampleCoreFFmpegPatchMatrixDiagnostic -count=1 -v
```

For the exact Annex A reduced fixed-codebook tree-search question, use the
clean-room numeric handoff. It exports only scalar search-surface values and
local scalar results in `fcb_tree_search_got.csv`; an isolated verifier must
fill only numeric `expected` cells in
`fcb_tree_search_expected_template.csv` before any comparison is treated as
evidence.

```sh
G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1 \
go test -run TestOracleHandoff_WriteFCBTreeSearchHandoff -count=1 -v

G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

For the same FCB tree-search question on the user problem sample's
2.9 second region, use the user-audio handoff pinned to
`testdata/external/user_quality_audio.m4a`:

```sh
G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_WriteFCBTreeSearchUserAudioHandoff -count=1 -v

G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

### Test suite layout

The repository ships multiple test layers, each with a distinct release
gate role:

| Suite | Invocation | Release gate role |
|---|---|---|
| Default (release) | `go test ./...` | **Binding.** Must PASS at the v0.1.0-rc1 tag commit. |
| FFmpeg quality (product) | `G729_FFMPEG_BLACKBOX_QUALITY=1 G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v` | **Binding for outbound G.729 encoder support.** Currently PASSes. |
| Local decoder quality | `G729_FFMPEG_BLACKBOX_QUALITY=1 G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY=1 go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v` | **Binding for strict local decoder regression coverage.** Currently PASSes against FFmpeg on the local encoder SPEECH payload. |
| Asterisk local decode quality | `G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY=1 go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v` | **Binding when a local non-redistributed Asterisk-origin inbound sample is present.** PASSed during rc1 verification against FFmpeg; not broad sender certification. |
| Conformance (informational) | `go test -tags=conformance ./...` | Non-blocking. Currently expects 0 failures; new failures must be triaged. |
| Diagnostic (informational) | `go test -tags=diagnostic ./...` | Non-blocking. Currently expects 5 documented PSTdomain drift-monitoring FAILs. |

The conformance and diagnostic suites do **not** block release;
their expected-failure inventories are catalogued in
[`docs/releases/v0.1.0-rc1-checklist.md`](docs/releases/v0.1.0-rc1-checklist.md).

---

## Clean-room statement

This project maintains a clean-room constraint. No ITU reference C,
bcg729, FFmpeg, Sipro, or other G.729 implementation source was used.
Public specifications, test vectors, and independently written tests
were used. v0.1.0 does not claim ITU byte-exact conformance.
See [IP_PROVENANCE.md](IP_PROVENANCE.md) for the distribution provenance
record and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the
redistribution notice inventory.

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
provenance record in [IP_PROVENANCE.md](IP_PROVENANCE.md) and a
third-party notice inventory in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---

## Development status

- **Phase 0 / 1 / 2** — encoder/decoder core implementation, completed.
  See `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`
  (master plan).
- **Phase 3 — CLOSED-PARTIAL**. Encoder and decoder diagnostics were
  closed enough to proceed to RC packaging, with the current public
  claim bounded by the FFmpeg black-box outbound encoder gate and the
  decoder limitations above. See
  [Phase 3-final closure report](docs/superpowers/plans/2026-05-04-phase3-final-closure-report.md).
- **Phase 4 — CLOSED**. Release packaging cycle for v0.1.0-rc1. See
  [Phase 4 plan](docs/superpowers/plans/2026-05-04-phase4-v0.1.0-release-packaging-plan.md).

This is a release candidate. The public API (`Encoder`, `Decoder`,
`EncoderProfile`, `NewEncoder`, `NewEncoderWithProfile`, `NewDecoder`,
`NewStreamingEncoder`, `NewStreamingEncoderWithProfile`, `EncodeFrame`,
`DecodeFrame`, `Reset`, `Write`, `Flush`, sentinel errors, frame-shape
constants) is intended to be stable across the v0.1.x line.
