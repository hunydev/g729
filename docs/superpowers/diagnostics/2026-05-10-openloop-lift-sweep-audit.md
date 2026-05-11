# Open-Loop Submultiple-Lift Sweep Audit

Date: 2026-05-10

Scope: clean-room audit of whether changing the Core open-loop
submultiple-lift factor is a valid next step for the remaining
problem-sample Core near-clips.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg source, Sipro, or other G.729
  implementation source was inspected.
- FFmpeg was used only as an external black-box decoder executable.
- The existing `testdata/oracle/pitch_top_open_loop.csv` artifact was
  used only as numeric scalar evidence.

## PDF Context

Annex A section A.3.4 says the winner among the three normalized
open-loop pitch correlations is selected by favoring lower delay ranges
when they are submultiples of higher delays. The PDF-visible text does
not specify a numeric lift factor.

The current Core implementation therefore uses a clean-room
spec-ambiguity choice: a global `11/10` submultiple lift. This is not a
byte-exact claim.

## Oracle State

The raw `T_op` artifact exists at:

```text
testdata/oracle/pitch_top_open_loop.csv
```

Current diagnostic:

```sh
go test -run 'TestOracleHCenter_TopOpenLoopOptionalDiagnostic|TestOracleHCenter_TopOpenLoopExactGate' -count=1 -v
```

Result:

```text
exact 1292/1835 70.41%
window +/-1: 1366/1835 74.44%
window +/-2: 1372/1835 74.77%
window +/-5: 1411/1835 76.89%
window +/-10: 1451/1835 79.07%
```

Interpretation: the open-loop center has numeric oracle evidence, but it
is not fully exact. The current exact gate is intentionally partial and
does not by itself justify changing the Core lift factor.

## Problem Sample Sweep

Command:

```sh
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a \
G729_EXTERNAL_SAMPLE_OPENLOOP_LIFT_SWEEP=1 \
  go test -run TestExternalSampleOpenLoopSubmultipleLiftSweepDiagnostic -count=1 -v
```

Selected results:

| Lift | Changed Top | SNR dB | SegSNR dB | Corr | Peak | NearClip | Local SNR dB | Local NearClip |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1.05 | 31 | 4.96 | 4.28 | 0.8291 | 32767 | 10 | 4.92 | 0 |
| 1.10 | 0 | 5.21 | 4.37 | 0.8383 | 32768 | 2 | 5.03 | 0 |
| 1.12 | 6 | 5.21 | 4.42 | 0.8379 | 31970 | 0 | 4.99 | 0 |
| 1.15 | 21 | 5.11 | 4.38 | 0.8340 | 32767 | 6 | 4.98 | 0 |
| 1.20 | 32 | 5.12 | 4.36 | 0.8368 | 32768 | 6 | 5.04 | 0 |
| 2.00 | 123 | 5.02 | 4.31 | 0.8307 | 32768 | 8 | 4.95 | 0 |

Problem-sample interpretation:

- `1.12` removes the two Core ffmpeg near-clips and slightly improves
  segmental SNR.
- It does not improve global SNR or correlation.
- It is a narrow tuning candidate, not a PDF-derived fix.

## Original User Sample Sweep

Command:

```sh
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_input.m4a \
G729_EXTERNAL_SAMPLE_OPENLOOP_LIFT_SWEEP=1 \
  go test -run TestExternalSampleOpenLoopSubmultipleLiftSweepDiagnostic -count=1 -v
```

Selected results:

| Lift | Changed Top | SNR dB | SegSNR dB | Corr | Peak | NearClip |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1.10 | 0 | 5.09 | 3.78 | 0.8346 | 26283 | 0 |
| 1.12 | 7 | 5.06 | 3.76 | 0.8332 | 26283 | 0 |
| 1.15 | 22 | 5.07 | 3.77 | 0.8336 | 26283 | 0 |
| 2.00 | 193 | 5.03 | 3.74 | 0.8319 | 29418 | 0 |

Original-sample interpretation: the `1.12` candidate slightly worsens
SNR, segmental SNR, and correlation versus current Core.

## SPEECH Black-Box Sweep

Command:

```sh
G729_FFMPEG_BLACKBOX_OPENLOOP_LIFT_SWEEP=1 \
  go test -run TestExternalFFmpegBlackboxOpenLoopLiftSweep_SPEECH -count=1 -v
```

Selected results:

| Path | Changed Top | SNR dB | SegSNR dB | Corr | Peak | NearClip |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| SPEECH.BIT -> ffmpeg | n/a | 7.04 | 4.39 | 0.8971 | 32767 | 1 |
| Core lift 1.10 | 0 | 6.87 | 4.33 | 0.8928 | 32319 | 0 |
| Core lift 1.12 | 8 | 6.87 | 4.32 | 0.8928 | 32319 | 0 |
| Core lift 1.15 | 17 | 6.87 | 4.32 | 0.8929 | 32319 | 0 |
| Core lift 2.00 | 146 | 6.72 | 4.33 | 0.8900 | 32410 | 0 |

SPEECH interpretation: `1.12` does not create an obvious aggregate
SPEECH regression versus current Core, but it also does not improve the
aggregate.

## Disposition

No production encoder change.

Changing Core from `11/10` to a nearby tuned value such as `1.12` would
be a Quality heuristic because the PDF text does not expose the exact
numeric lift factor. The active goal explicitly keeps tuning changes out
of `EncoderProfileCore`.

The product/default Quality profile already meets the problem-sample
near-clip target. The remaining Core Annex A alignment work is therefore
still the FCB tree-search numeric oracle handoff, not an open-loop lift
tuning patch.
