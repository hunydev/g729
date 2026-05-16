# Encoder Validation Plan

This document tracks post-`v0.1.0` encoder validation. It is engineering
evidence, not ITU certification, ITU endorsement, or an encoder byte-exact
claim.

## Current Baseline

The product encoder default is `NewEncoder()` / `EncoderProfileCore`.
Diagnostic profiles may score differently on objective metrics, but they are
not the default send path.

Initial post-release audit found that an FFmpeg quality-test helper named
`writeOurEncodedRawG729` still encoded with diagnostic `EncoderProfileQuality`
instead of the product default. The helper has been corrected to use
`NewEncoder()`, and `TestWriteOurEncodedRawG729UsesProductDefault` pins that
test-helper contract.

The next encoder audit added a receiver-mirror invariant:
`TestEncoderReceiverMirrorStateMatchesEmittedBitstream` decodes each emitted
G.729 frame through an independent local receiver-state mirror and checks that
the encoder's cached LP, pitch, gain-predictor, pitch-gain, and past-excitation
state match what the transmitted bitstream will reconstruct. That test exposed
and now pins a gain-codebook arithmetic-width bug: encoder-side `gammaCQ13`
must keep the receiver's full `GBK1+GBK2` `int32` value when updating
`pastQuaEn`, rather than an `int16`-saturated surrogate.

With the corrected product-default helper and receiver-aligned gain update, the
SPEECH/FFmpeg black-box gate still passes:

```text
SPEECH.BIT -> ffmpeg       GlobalSNR 7.04 dB   SegSNR 4.39 dB
our Core encoder -> ffmpeg GlobalSNR 6.92 dB   SegSNR 4.38 dB
delta vs reference path    GlobalSNR -0.12 dB  SegSNR -0.01 dB
```

The diagnostic `EncoderProfileQuality` scores higher on this specific numeric
SPEECH gate (`+0.43 dB` GlobalSNR, `+0.32 dB` SegSNR versus the FFmpeg
reference path), but it is not selected as the product default because recent
blind listening preferred Core on private samples.

## Validation Tiers

| Tier | Purpose | Evidence |
|---|---|---|
| API/default identity | Prevent test and docs from drifting away from the product default | `TestWriteOurEncodedRawG729UsesProductDefault` |
| Receiver-state mirror | Ensure encoder state follows the decoder reconstruction implied by its own emitted bitstream | `TestEncoderReceiverMirrorStateMatchesEmittedBitstream` |
| Public regression | Keep a redistributable sample in default `go test ./...` | `TestEncoderCorePagesQualityRegression` |
| Black-box interoperability | Verify local encoder payloads decode cleanly with FFmpeg executable black box | `TestExternalFFmpegBlackboxQuality_SPEECH` |
| Decoder separation | Compare local decoder and FFmpeg on the same local encoder payload | `TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH` |
| Profile comparison | Track Core, CoreFast, and diagnostic profile quality without changing defaults | `TestExternalFFmpegBlackboxProfileCompare_SPEECH` and external sample profile diagnostics |
| Private listening matrix | Compare Core, diagnostic profiles, `bcg729` black-box anchor, FFmpeg decode, and exact local decode on private samples | `TestExternalSamplePESQMatrixDiagnostic` and comparison web app blind arena |
| Realtime safety | Quantify encoder RTF, p95/p99 processing time, deadline ratio, and deadline misses | `cmd/g729loadtest` |

## Required Product-Default Commands

```sh
env GOCACHE=/tmp/go-build go test ./... -count=1

env GOCACHE=/tmp/go-build \
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v

env GOCACHE=/tmp/go-build \
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 \
G729_FFMPEG_BLACKBOX_REPORT=/tmp/g729-encoder-speech-ffmpeg.json \
go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v

env GOCACHE=/tmp/go-build \
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v
```

## Exploratory Commands

```sh
env GOCACHE=/tmp/go-build \
G729_FFMPEG_BLACKBOX_PROFILE_COMPARE=1 \
go test -run TestExternalFFmpegBlackboxProfileCompare_SPEECH -count=1 -v

env GOCACHE=/tmp/go-build \
G729_EXTERNAL_SAMPLE_PROFILE_COMPARE=1 \
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/private-sample.wav \
go test -run TestExternalSampleProfileCompareDiagnostic -count=1 -v

env GOCACHE=/tmp/go-build \
G729_EXTERNAL_SAMPLE_PESQ_MATRIX=1 \
G729_PESQ_PYTHON=/path/to/python-with-pesq \
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/private-sample.wav \
go test -run TestExternalSamplePESQMatrixDiagnostic -count=1 -v
```

## Next Work

- Extend the machine-readable FFmpeg black-box report from SPEECH to the private
  sample matrix, without committing private media or oracle data.
- Build a private sample manifest format that records source duration, speech
  type, language/accent, input headroom, and whether the sample is
  redistributable.
- Add CoreFast gates that explicitly compare throughput gain against bounded
  quality loss.
- Add a focused encoder validation report for the comparison web app so blind
  arena results can be recorded without exposing private audio.
