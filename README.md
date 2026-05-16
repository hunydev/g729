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
black-box tested against FFmpeg. The decoder has a stronger conformance
result: in the current private verifier run, fixed ITU Annex A
bitstreams decoded by this package match the official reference PCM
sample-for-sample (`740800/740800` final PCM samples exact). This is a
decoder correctness claim, not ITU certification and not an encoder
byte-exact claim.

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
| Opt-in `DecodeFrameEnhanced` listening aid | **Experimental; not a conformance claim** |
| Streaming `Encoder.Write` / `Encoder.Flush` | **Supported** |
| Hot-path 0-allocation steady state | **Verified** |
| Decoder ITU Annex A final PCM conformance | **Sample-exact in current private oracle gate: `740800/740800` final PCM samples** |
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

Current status: the outbound TTS/RTP send path passes the binding FFmpeg
black-box encoder quality gate for `G729/8000 annexb=no` payloads. The
strict local decoder also passes the strongest decoder gate available to
this project: fixed ITU Annex A `.BIT` streams decoded by this package match
the companion reference `.PST` PCM sample-for-sample in the private oracle
run (`740800/740800` final PCM samples exact). This gives the decoder a
concrete conformance basis beyond MOS/PESQ-style quality scores. It is still
not ITU certification, not an endorsement by ITU, and not a claim that the
encoder is byte-exact to the ITU reference encoder.

An experimental `DecodeFrameEnhanced` path remains available for listening
diagnostics. It is non-strict and is not used as evidence for the
`G729/8000 annexb=no` product claim.

---

## Known scope and limitations

This release distinguishes decoder conformance evidence, encoder quality
evidence, and standards certification:

1. **Decoder final PCM exact gate passes.** Fixed ITU Annex A bitstreams
   decoded by this package match the official reference PCM sample-for-sample
   in the current private verifier run: `740800/740800` final PCM samples
   exact across `ALGTHM`, `ERASURE`, `FIXED`, `LSP`, `OVERFLOW`, `PARITY`,
   `PITCH`, `SPEECH`, `TAME`, and `TEST`. For a decoder, this is stronger
   evidence than PESQ/MOS because the input bitstream is fixed and the output
   can be compared directly.
2. **The decoder claim is not an ITU certification claim.** ITU has not
   certified this implementation, and no endorsement is implied. The claim is
   limited to the private, reproducible verifier result described above.
3. **The encoder is not claimed byte-exact.** `EncoderProfileCore` emits
   standard 10-byte G.729 frames and passes the project FFmpeg black-box
   outbound quality gate, but it is an independent encoder implementation and
   is not expected to match the ITU reference encoder or `bcg729` bit-for-bit.
4. **Verifier data is not redistributed.** The private oracle directory
   contains numeric outputs derived from external conformance materials. Those
   data files are not part of the MIT-licensed source distribution and are not
   relicensed as MIT. Public documentation records aggregate pass/fail counts
   and clean-room process only.
5. **Annex B and other G.729 variants remain out of scope.** SID/CNG/DTX
   (`annexb=yes`), G.729.1, G.729D, and G.729E are not implemented.
6. **`DecodeFrameEnhanced` is diagnostic-only.** It is useful for listening
   experiments but is not the strict decoder path and is not used as
   conformance evidence.

### ITU decoder vector validation

Decoder validation is strongest when it uses fixed ITU bitstreams:
`.BIT -> local decoder -> PCM` compared directly against the companion
`.PST` reference PCM. The current strict decoder reaches exact final PCM
equality in the private oracle gate:

```text
ALGTHM    2800/2800 exact
ERASURE  24000/24000 exact
FIXED    9600/9600 exact
LSP      178560/178560 exact
OVERFLOW 30720/30720 exact
PARITY   24000/24000 exact
PITCH    146800/146800 exact
SPEECH   300000/300000 exact
TAME     10240/10240 exact
TEST     14080/14080 exact
TOTAL    740800/740800 exact
```

The hard gate is opt-in because the large oracle data is intentionally kept
outside the public repository:

```sh
G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 \
G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1 \
G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/private/verifier-output \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v
```

Stage and arithmetic oracles are also kept private. They are used to localize
bugs without importing implementation source; public repo artifacts should
contain only prompts, schemas, aggregate results, and clean-room notes unless
a small numeric fixture is explicitly reviewed for redistribution.

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

To turn that matrix into a hard regression gate for private listening/PESQ
samples, add `G729_REQUIRE_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1`. The gate
currently pins the narrow decoder-exact-informed candidate path: native
reconstructed-gain search + decoder-in-loop gain clip repair + fixed-codebook
residual reranking (`EncoderProfileQualityPESQ`). It must beat Core by at
least `0.05` PESQ NB locally and through FFmpeg, keep the FFmpeg PESQ gap to
the local `bcg729` black-box anchor within `0.15`, and avoid local near-clips.

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
| Pages Core quality regression | `go test -run TestEncoderCorePagesQualityRegression -count=1 -v` | Included in default tests. Pins the public demo sample against the current Core encoder and exact local decoder so obvious SNR/correlation/headroom regressions are caught without external tools. |
| Decoder final PCM oracle | `G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1 G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/private/verifier-output go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v` | **Binding for strict decoder conformance when private oracle data is available.** Current private run PASSes `740800/740800` final PCM samples exact. |
| Private PESQ candidate regression | `G729_PESQ_PYTHON=/path/to/python G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 G729_REQUIRE_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v` | Binding when private listening samples, PESQ, FFmpeg, and the local black-box anchor are available. Pins the decoder-exact-informed candidate against Core and `bcg729` without changing the product default. |
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
Public specifications, black-box executable behavior, private numeric
oracle outputs, and independently written tests were used. v0.1.0 claims
strict decoder final-PCM sample equality against the current private ITU
Annex A oracle gate; it does not claim ITU certification, ITU endorsement,
or encoder byte-exact conformance.
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
- **Phase 3 — CLOSED**. Decoder exactness work reached strict final-PCM
  equality against the private ITU Annex A oracle gate (`740800/740800`
  samples exact). Encoder quality remains bounded by the FFmpeg black-box
  outbound gate and listening diagnostics; encoder byte-exact conformance is
  not claimed.
- **Phase 4 — CLOSED**. Release packaging cycle for v0.1.0-rc1. See
  [Phase 4 plan](docs/superpowers/plans/2026-05-04-phase4-v0.1.0-release-packaging-plan.md).

This is a release candidate. The public API (`Encoder`, `Decoder`,
`EncoderProfile`, `NewEncoder`, `NewEncoderWithProfile`, `NewDecoder`,
`NewStreamingEncoder`, `NewStreamingEncoderWithProfile`, `EncodeFrame`,
`DecodeFrame`, `Reset`, `Write`, `Flush`, sentinel errors, frame-shape
constants) is intended to be stable across the v0.1.x line.
