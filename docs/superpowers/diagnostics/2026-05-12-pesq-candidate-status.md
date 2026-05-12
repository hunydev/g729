# PESQ Candidate Quality Status

Date: 2026-05-12

Scope: final status for the active end-to-end G.729 quality goal after the
decoder postfilter alignment work and the PESQ-led encoder candidate was
promoted to the product default.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg, Sipro, or other G.729 implementation
  source was inspected.
- `bcg729` and FFmpeg are used only as black-box executables for numeric
  decode/encode measurements.
- PESQ NB is used only as a legacy narrowband diagnostic; it is not a
  conformance claim and it does not replace blind listening.

## Candidate

The current product default encoder profile is:

```text
EncoderProfileQualityPESQ
```

It emits normal 10-byte G.729 frames. It keeps the broader Quality heuristic
surface disabled, but enables:

- native reconstructed-gain residual search;
- gain clip repair;
- fixed-codebook residual reranking.

This profile was introduced for A/B testing because it is the first local
candidate in this cycle that reaches the active PESQ target on both main user
samples while keeping decoded near-clips at zero. It is now the default used by
`NewEncoder` and `NewStreamingEncoder`; the older broad
`EncoderProfileQuality` remains available as a diagnostic profile.

## Current Numeric Evidence

The 8000 web app was checked with:

```sh
curl -fsS -F 'file=@testdata/external/user_quality_audio.m4a' \
  'http://127.0.0.1:8000/api/compare?want=pesq_local,pesq_ffmpeg,external_local,external_ffmpeg'

curl -fsS -F 'file=@testdata/external/user_quality_input.m4a' \
  'http://127.0.0.1:8000/api/compare?want=pesq_local,pesq_ffmpeg,external_local,external_ffmpeg'
```

`testdata/external/user_quality_audio.m4a`:

| Path | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Lag | Peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| PESQ candidate -> local decode | 3.558455 | -0.139031 | 0 | 40 | 31978 |
| PESQ candidate -> FFmpeg decode | 3.601664 | -0.095822 | 0 | 40 | 31847 |
| bcg729 -> local decode | 3.649558 | -0.047928 | 0 | 40 | 32622 |
| bcg729 -> FFmpeg decode | 3.697486 | 0.000000 | 0 | 40 | 32614 |

`testdata/external/user_quality_input.m4a`:

| Path | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Lag | Peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| PESQ candidate -> local decode | 3.545760 | -0.109909 | 0 | 40 | 27326 |
| PESQ candidate -> FFmpeg decode | 3.555439 | -0.100230 | 0 | 40 | 30366 |
| bcg729 -> local decode | 3.605134 | -0.050535 | 0 | 40 | 26136 |
| bcg729 -> FFmpeg decode | 3.655669 | 0.000000 | 0 | 40 | 25881 |

Decoder isolation status:

- On `user_quality_audio.m4a`, `bcg729 -> local decode` is within `0.047928`
  PESQ of `bcg729 -> FFmpeg decode`.
- On `user_quality_input.m4a`, `bcg729 -> local decode` is within `0.050535`
  PESQ of `bcg729 -> FFmpeg decode`.
- This meets the active decoder-gap target of `<= 0.05~0.10` PESQ.

End-to-end status:

- Both main samples are above `3.5` PESQ NB for `PESQ candidate -> local
  decode`.
- Both main samples are within `0.15` PESQ of the `bcg729 -> FFmpeg decode`
  black-box anchor on the local-decode path.
- NearClip is `0` on all four measured paths for both samples.

## Web App Status

The 8000 app exposes:

- full Compare rows for the default encoder, Core, Core-clip, and bcg729
  black-box anchor paths;
- Blind 1:1 options for default-vs-Core, default-vs-bcg729, Core-vs-bcg729,
  and local-vs-FFmpeg decoder checks;
- trial result rows with `PESQ`, `SNR`, `Corr`, `Clip`, residual `High`, and
  residual `Worst`;
- a copyable blind-result summary text area for preserving listening results.

Health check:

```text
curl -fsS http://127.0.0.1:8000/healthz
ok
```

## Reproduction Commands

Focused PESQ candidate matrix:

```sh
G729_PESQ_PYTHON=/tmp/g729-pesq-venv/bin/python \
G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 \
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a \
go test . -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v

G729_PESQ_PYTHON=/tmp/g729-pesq-venv/bin/python \
G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 \
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_input.m4a \
go test . -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v
```

General verification:

```sh
go test ./... -count=1

(cd third-party/g729-compare-web && go test . -count=1)
```

## Completion Status

Complete.

The numeric gates are satisfied, and the blind-listening gate is now satisfied
by the user's 2026-05-12 report:

- no audible difference among `our PESQ/default encode -> local decode`,
  `our core encode -> local decode`, and `bcg729 encode -> FFmpeg decode`;
- no audible grit, muffling, or bcg729-relative roughness in the current local
  encode/decode output;
- no audible difference between `bcg729 encode -> FFmpeg decode` and
  `bcg729 encode -> local decode`.

## Addendum: PESQ-Degrit Blind Candidate

`EncoderProfileQualityPESQDegrit` was added as a listening diagnostic, not as a
replacement for `EncoderProfileQualityPESQ`. It uses the PESQ candidate's
native reconstructed-gain search, gain clip repair, and FCB residual reranking,
then also enables bounded gain MSE/noise repair to test whether lower
high-residual energy reduces the user-reported grit.

Current PESQ evidence says it is not the numeric leader:

| Sample | Path | PESQ NB | Gap vs bcg729+FFmpeg | NearClip |
| --- | --- | ---: | ---: | ---: |
| `user_quality_audio.m4a` | PESQ-degrit -> local decode | 3.483080 | -0.214406 | 0 |
| `user_quality_audio.m4a` | PESQ-degrit -> FFmpeg decode | 3.488838 | -0.208648 | 2 |
| `user_quality_input.m4a` | PESQ-degrit -> local decode | 3.3570 | -0.2987 | 0 |
| `user_quality_input.m4a` | PESQ-degrit -> FFmpeg decode | 3.3517 | -0.3040 | 0 |

The 8000 web app exposes `PESQ candidate vs PESQ-degrit candidate` and
`PESQ-degrit candidate vs bcg729` blind pairs so this tradeoff can be judged by
listening rather than promoted on PESQ alone.

## Addendum: Core Listening Candidate and Default-Gate Status

After user listening feedback, `EncoderProfileCore` became an important
subjective candidate: it disables the repository-local Quality/PESQ heuristics
and can sound less processed even when its PESQ is lower. The 8000 web app now
exposes Core-centered blind pairs, including:

- `Core local decode vs bcg729 FFmpeg`
- `Core local decode vs bcg729 local decode`
- `Core local decode vs PESQ candidate local decode`
- `Core local decode vs core-clip local decode`
- `Core local decode vs Core FFmpeg decode`

`EncoderProfileCoreClipRepair` was added as a listening diagnostic. It keeps
Core's search policy and only adds decoder-in-loop gain clip repair with a
lower pre-clip threshold. On `user_quality_audio.m4a`, this removes the
Core+FFmpeg near-clip markers but lowers PESQ/SNR, so it is not a numeric
promotion candidate by itself.

Current selected 8000 API evidence:

`testdata/external/user_quality_audio.m4a`:

| Key | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Peak |
| --- | ---: | ---: | ---: | ---: |
| `our_local` | 3.279689 | -0.417797 | 0 | 30736 |
| `our_ffmpeg` | 3.271727 | -0.425759 | 4 | 32767 |
| `core_local` | 3.403904 | -0.293582 | 0 | 30888 |
| `core_ffmpeg` | 3.444327 | -0.253159 | 2 | 32768 |
| `core_clip_local` | 3.366384 | -0.331102 | 0 | 29004 |
| `core_clip_ffmpeg` | 3.414424 | -0.283062 | 0 | 31386 |
| `pesq_local` | 3.558455 | -0.139031 | 0 | 31978 |
| `pesq_ffmpeg` | 3.601664 | -0.095822 | 0 | 31847 |
| `external_local` | 3.649558 | -0.047928 | 0 | 32622 |
| `external_ffmpeg` | 3.697486 | 0.000000 | 0 | 32614 |

`testdata/external/user_quality_input.m4a`:

| Key | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Peak |
| --- | ---: | ---: | ---: | ---: |
| `our_local` | 3.135855 | -0.519814 | 0 | 26456 |
| `our_ffmpeg` | 3.139509 | -0.516160 | 0 | 28942 |
| `core_local` | 3.466861 | -0.188808 | 0 | 25932 |
| `core_ffmpeg` | 3.489401 | -0.166268 | 0 | 26283 |
| `core_clip_local` | 3.466861 | -0.188808 | 0 | 25932 |
| `core_clip_ffmpeg` | 3.489401 | -0.166268 | 0 | 26283 |
| `pesq_local` | 3.545760 | -0.109909 | 0 | 27326 |
| `pesq_ffmpeg` | 3.555439 | -0.100230 | 0 | 30366 |
| `external_local` | 3.605134 | -0.050535 | 0 | 26136 |
| `external_ffmpeg` | 3.655669 | 0.000000 | 0 | 25881 |

Default-gate interpretation:

- `EncoderProfileQualityPESQ` still satisfies the PESQ target on both main
  samples and keeps near-clips at zero.
- The current default `EncoderProfileQuality` (`our_local`/`our_ffmpeg` in the
  web app) does not satisfy the PESQ target on these samples.
- No default promotion has been made because the active goal requires blind
  listening to confirm that the user-reported grit/smoky/mic-rub artifact is
  hard to distinguish from bcg729.

Current blocker:

- Resolved. Final blind listening reported no meaningful audible difference
  between the promoted default, Core, and the bcg729+FFmpeg anchor. The PESQ
  candidate was promoted because it satisfies the numeric release gate while
  also passing the user's subjective gate.

## Addendum: Final Default Promotion Audit

`EncoderProfileQualityPESQ` is promoted to the default encoder profile used by
the public constructors. The 8000 web app now labels this path as `our encode`
instead of exposing it as a separate PESQ candidate.

Current 8000 API evidence after default promotion:

`testdata/external/user_quality_audio.m4a`:

| Key | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Lag | Peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| `our_local` | 3.558455 | -0.139031 | 0 | 40 | 31978 |
| `our_ffmpeg` | 3.601664 | -0.095822 | 0 | 40 | 31847 |
| `external_local` | 3.649558 | -0.047928 | 0 | 40 | 32622 |
| `external_ffmpeg` | 3.697486 | 0.000000 | 0 | 40 | 32614 |

`testdata/external/user_quality_input.m4a`:

| Key | PESQ NB | Gap vs bcg729+FFmpeg | NearClip | Lag | Peak |
| --- | ---: | ---: | ---: | ---: | ---: |
| `our_local` | 3.545760 | -0.109909 | 0 | 40 | 27326 |
| `our_ffmpeg` | 3.555439 | -0.100230 | 0 | 40 | 30366 |
| `external_local` | 3.605134 | -0.050535 | 0 | 40 | 26136 |
| `external_ffmpeg` | 3.655669 | 0.000000 | 0 | 40 | 25881 |

Verification:

```sh
go test ./... -count=1
(cd third-party/g729-compare-web && go test . -count=1)
curl -fsS http://127.0.0.1:8000/healthz
```
