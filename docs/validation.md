# Validation and Quality Gates

This document expands the validation summary in the repository README.

The project separates three different claims:

- Decoder conformance evidence.
- Encoder quality evidence.
- Standards certification.

Only the first two are project claims. ITU certification, ITU endorsement, and
encoder byte-exact conformance are not claimed.

## Summary

| Area | Evidence | Claim |
|---|---|---|
| Decoder | ITU Annex A `.BIT -> local decoder -> PCM` private oracle | `740800/740800` final PCM samples exact |
| Encoder | FFmpeg black-box quality gate | Product send path is quality-gated |
| Encoder quality | Public sample regression, private PESQ NB matrix, blind listening | Diagnostic quality evidence, not certification |
| Optional MOS-LQO | External `G729_MOS_LQO_TOOL` wrapper | Customer-facing objective score when a licensed scorer is available |
| Performance | Single-thread RTF and frame-time jitter benchmarks | Real-time streaming capacity planning evidence |
| RTP | Payload type 18, `ptime=10/20`, `annexb=no` checks | Send-path interoperability confidence |
| IP | Clean-room provenance record | No third-party codec source used |

See [performance.md](performance.md) for the real-time factor and jitter
benchmark methodology.

## ITU Decoder Vector Validation

Decoder validation is strongest when it uses fixed ITU bitstreams:
`.BIT -> local decoder -> PCM` compared directly against the companion `.PST`
reference PCM. The current strict decoder reaches exact final PCM equality in
the private oracle gate:

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
contain only prompts, schemas, aggregate results, and clean-room notes unless a
small numeric fixture is explicitly reviewed for redistribution.

## Encoder Quality Evidence

The encoder cannot be validated the same way as the decoder. A decoder has a
fixed bitstream input, so exact `.BIT -> PCM` comparison is direct. An encoder
has search freedoms and may produce a different valid G.729 bitstream for the
same PCM input, so this project uses the strongest practical quality stack
available without importing third-party codec source:

1. **Default release regression.** `go test ./...` includes
   `TestEncoderCorePagesQualityRegression`, which encodes the public Pages demo
   sample with `EncoderProfileCore`, decodes it with the exact local decoder,
   and checks SNR, correlation, RMS ratio, and clipping headroom.
2. **Black-box interoperability.** The opt-in FFmpeg gate encodes with the
   local encoder and decodes with FFmpeg as an executable-only black box. This
   catches payload compatibility and gross quality regressions without relying
   on FFmpeg source.
3. **Private sample quality matrix.** For local customer/problem samples, the
   PESQ NB diagnostic compares Core, diagnostic profiles, and the local
   `bcg729` black-box anchor through both the exact local decoder and FFmpeg.
   PESQ NB is a legacy P.862-style objective score; it remains useful for
   regression and relative ranking, but it is not current ITU certification.
4. **Blind listening.** The comparison web app keeps the release decision tied
   to A/B listening when objective scores disagree with perceived quality. The
   current product default remains `EncoderProfileCore` because recent blind
   tests preferred it over the more PESQ-led candidate, which measured well but
   sounded slightly more muffled.
5. **Optional MOS-LQO/POLQA reporting.** If a customer requires MOS-LQO, set
   `G729_MOS_LQO_TOOL` to an external scorer wrapper. The repository does not
   ship POLQA/P.863 because the standard implementation is not open source or
   free for general commercial use. MOS-LQO output is reported as an objective
   listening-quality number, not as a subjective MOS panel result and not as an
   ITU conformance certificate.

PESQ is used as a regression signal, not as the final product-quality
selector. The default encoder remains Core because recent blind listening
preferred it over the PESQ-led candidate, even when the candidate scored higher
on PESQ NB.

Representative private PESQ NB runs on the current sample set place the best
diagnostic candidate close to the local `bcg729` anchor while leaving Core as
the default for listening quality:

```text
user_quality_audio.m4a:
bcg729 -> ffmpeg decode        PESQ NB 3.6975
Core -> ffmpeg decode          PESQ NB 3.4654
PESQ candidate -> ffmpeg       PESQ NB 3.6354

user_quality_input.m4a:
bcg729 -> ffmpeg decode        PESQ NB 3.6557
Core -> ffmpeg decode          PESQ NB 3.4952
PESQ candidate -> ffmpeg       PESQ NB 3.5683
```

These numbers are release diagnostics. They support regression control and
customer-facing quality discussion, but they do not replace codec-vector
conformance evidence.

## FFmpeg Black-Box Quality Gate

The quality gate uses FFmpeg only as an external decoder executable; no
external implementation source is inspected.

```sh
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v
```

The gate passes when the local encoder decode quality is within 2 dB of the
`SPEECH.BIT -> ffmpeg` reference path on both global SNR and segmental SNR.

The inbound/local decoder Asterisk sample gate is intentionally separate from
the outbound encoder claim:

```sh
G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v
```

The local decoder gate for the local encoder stream is also separate from the
passing outbound encoder claim:

```sh
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY=1 \
go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v
```

## External Sample Diagnostics

For a user-provided problem sample, run the opt-in external sample diagnostic.
FFmpeg-readable inputs such as WAV/MP3/M4A are converted to 8 kHz mono signed
16-bit PCM through the local FFmpeg executable; raw `.pcm`, `.raw`, `.sln`,
`.s16le`, and `.in` files are assumed to already be 8 kHz mono signed
little-endian int16 PCM.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
go test -run TestExternalSampleQualityDiagnostic -count=1 -v
```

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
residual reranking (`EncoderProfileQualityPESQ`). It must beat Core by at least
`0.05` PESQ NB locally and through FFmpeg, keep the FFmpeg PESQ gap to the
local `bcg729` black-box anchor within `0.15`, and avoid local near-clips.

If a customer asks for MOS-LQO/POLQA-style objective listening-quality numbers,
provide an external scorer wrapper through `G729_MOS_LQO_TOOL`. The wrapper is
not bundled with this MIT repository because POLQA/P.863 is not generally
available as open-source/free commercial software. It must accept
`ref.wav degraded.wav` and print one finite score to stdout:

```sh
G729_MOS_LQO_TOOL=/path/to/mos-lqo-wrapper \
G729_PESQ_PYTHON=/tmp/g729-pesq-venv/bin/python \
G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 \
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v
```

## Focused Diagnostics

To separate clipping from "muffled" spectral-shape complaints:

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_SPECTRAL_TILT=1 \
go test -run TestExternalSampleSpectralTiltDiagnostic -count=1 -v
```

To compare quality variants around a specific audible artifact:

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_QUALITY_WINDOW=1 \
go test -run TestExternalSampleQualityWindowDiagnostic -count=1 -v
```

For Core-vs-Quality encoder search work, prefer the production-state gain-mode
diagnostic over the older gain-scale sweep helpers:

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_PRODUCTION_GAIN_MODE=1 \
go test -run TestExternalSampleProductionGainModeDiagnostic -count=1 -v
```

For Core near-clip localization, the patch-matrix diagnostic can be run in
Core mode. It edits transmitted fields after encoding and decodes with FFmpeg,
so it is a numeric diagnostic only, not a production fix.

```sh
G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav \
G729_EXTERNAL_SAMPLE_CORE_FFMPEG_PATCH_MATRIX=1 \
go test -run TestExternalSampleCoreFFmpegPatchMatrixDiagnostic -count=1 -v
```

For the exact Annex A reduced fixed-codebook tree-search question, use the
clean-room numeric handoff:

```sh
G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1 \
go test -run TestOracleHandoff_WriteFCBTreeSearchHandoff -count=1 -v

G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

For the same FCB tree-search question on the user problem sample's 2.9 second
region:

```sh
G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_WriteFCBTreeSearchUserAudioHandoff -count=1 -v

G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

## Test Suite Layout

| Suite | Invocation | Release gate role |
|---|---|---|
| Default release | `go test ./...` | **Binding.** Must pass at release commits. |
| Pages Core quality regression | `go test -run TestEncoderCorePagesQualityRegression -count=1 -v` | Included in default tests. Pins the public demo sample against the current Core encoder and exact local decoder. |
| Decoder final PCM oracle | `G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1 G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/private/verifier-output go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v` | **Binding for strict decoder conformance when private oracle data is available.** |
| Private PESQ candidate regression | `G729_PESQ_PYTHON=/path/to/python G729_EXTERNAL_SAMPLE_QUALITY=/path/to/input.wav G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 G729_REQUIRE_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v` | Binding when private listening samples, PESQ, FFmpeg, and the local black-box anchor are available. |
| Optional MOS-LQO reporting | `G729_MOS_LQO_TOOL=/path/to/wrapper G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v` | Informational. Requires an external licensed scorer. |
| FFmpeg quality | `G729_FFMPEG_BLACKBOX_QUALITY=1 G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v` | **Binding for outbound G.729 encoder support.** |
| Local decoder quality | `G729_FFMPEG_BLACKBOX_QUALITY=1 G729_REQUIRE_LOCAL_DECODER_FFMPEG_QUALITY=1 go test -run TestExternalFFmpegBlackboxLocalDecoderDelta_SPEECH -count=1 -v` | **Binding for strict local decoder regression coverage.** |
| Asterisk local decode quality | `G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY=1 go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v` | Binding when a local non-redistributed Asterisk-origin inbound sample is present. |
| Conformance | `go test -tags=conformance ./...` | Informational. Currently expects 0 failures. |
| Diagnostic | `go test -tags=diagnostic ./...` | Informational. Currently expects documented PST-domain drift-monitoring failures. |

The conformance and diagnostic suites do not block release; their
expected-failure inventories are catalogued in
[`releases/v0.1.0-rc1-checklist.md`](releases/v0.1.0-rc1-checklist.md).
