# Decoder TAME Pre-ACB History Prompt

You are an isolated clean-room verifier. Do not inspect, return, or describe
G.729 implementation source code. Return only numeric scalar answers, or a
short incomplete note outside the CSV if the requested values cannot be
derived independently.

Fill only the blank `expected` cells in:

```text
testdata/oracle/handoff/decoder_tame_pre_acb_history_expected_template.csv
```

The template asks for the TAME vector's past-excitation FIFO immediately before
building the adaptive-codebook vector for frame 117, subframe 0. This is the
next localization point after the completed TAME wide-stage artifact showed:

```text
adaptive_gain_q14 exact: 6/6
adaptive_v_q0 exact: 0/240
pitch_contrib_q0 exact: 0/240
excitation_u_q0 exact: 0/240
synth_s_q0 exact: 0/240
```

Purpose: determine whether the frame 117 adaptive-codebook mismatch is already
present in the incoming past-excitation history, or whether the local
adaptive-codebook interpolation itself is the next failing operation.

## Required Output

Return the same filename with exactly this header and row order:

```csv
source,frame,sub,field,index,expected
```

For every template row:

- `source` is `TAME`.
- `frame` is `117`.
- `sub` is `0`.
- `field` is `past_exc_pre_acb_q0`.
- `index` is `0..152`.
- `expected` must be the signed decimal integer Q0 past-excitation value at
  that FIFO index immediately before adaptive-codebook reconstruction.

Use the local decoder convention for this numeric artifact:

```text
index 152 is u(-1), the most recent past excitation sample.
index 0 is the oldest sample in the 153-sample FIFO.
```

Do not add columns, comments, source names, provenance notes, or explanatory
text inside the CSV.

## Inputs You May Use

Use only clean-room numeric/spec inputs, including:

```text
TAME.BIT
testdata/oracle/handoff/decoder_tame_stage_wide_expected.csv
testdata/oracle/handoff/tame_short_pitch_relation.csv
testdata/oracle/handoff/decoder_tame_pre_acb_history_expected_template.csv
```

If your clean-room trace generator can extend the same TAME decode trace
backward to earlier frames, use that trace to determine the frame 117
pre-ACB history. If these values cannot be independently derived without
using implementation-derived local `got` values or external implementation
source code, do not guess. Return a short incomplete note outside the CSV.

## Local Compare Command

After copying the filled file into `testdata/oracle/handoff`, the
implementation-side compare command is:

```sh
G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1 \
G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY=1 \
G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEPreACBHistory -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any other
G.729 implementation code. External decoder binaries are not needed for this
task; if a future task permits external tools, they may only be used as
black-box executable processes. Verifier output may enter this repository only
as numeric scalar oracle artifacts.
