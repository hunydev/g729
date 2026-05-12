# PESQ Candidate Quality Status

Date: 2026-05-12

Scope: status handoff for the active end-to-end G.729 quality goal after the
decoder postfilter alignment work and the PESQ-led encoder candidate.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg, Sipro, or other G.729 implementation
  source was inspected.
- `bcg729` and FFmpeg are used only as black-box executables for numeric
  decode/encode measurements.
- PESQ NB is used only as a legacy narrowband diagnostic; it is not a
  conformance claim and it does not replace blind listening.

## Candidate

The current listening-diagnostic candidate is:

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
samples while keeping decoded near-clips at zero.

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

- full Compare rows for `pesq_local` and `pesq_ffmpeg`;
- Blind 1:1 options for `PESQ candidate vs bcg729`;
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

Not complete yet.

The numeric gates are satisfied, but the active goal also requires that the
user-reported grit/smoky/mic-rub artifact becomes difficult to distinguish from
bcg729 in blind listening. That requirement needs the user's Blind 1:1 result
summary from the 8000 web app before the goal can be closed.

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
