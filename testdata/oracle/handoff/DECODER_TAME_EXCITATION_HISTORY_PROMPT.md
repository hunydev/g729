# Decoder TAME Excitation History Prompt

You are an isolated clean-room verifier. Do not inspect, return, or describe
G.729 implementation source code. Return only numeric scalar answers, or a
short incomplete note outside the CSV if the requested values cannot be
derived independently.

Fill only the blank `expected` cells in:

```text
testdata/oracle/handoff/decoder_tame_excitation_history_expected_template.csv
```

The template asks for the TAME vector's decoded excitation history from frame
0 through frame 116, both subframes. This is a forward-trace request, not an
inverse reconstruction request.

Purpose: provide the prior excitation needed to determine the 153-sample
past-excitation FIFO immediately before TAME frame 117 subframe 0 adaptive
codebook reconstruction. The previous
`decoder_tame_pre_acb_history_expected_template.csv` request could not be
filled from only downstream `adaptive_v_q0` rows because that inverse problem
is underdetermined.

## Required Output

Return the same filename with exactly this header and row order:

```csv
source,frame,sub,field,index,expected
```

For every template row:

- `source` is `TAME`.
- `frame` is `0..116`.
- `sub` is `0` or `1`.
- `field` is `excitation_u_q0`.
- `index` is `0..39`.
- `expected` must be the signed decimal integer Q0 excitation sample for that
  frame/subframe/sample position after the adaptive and fixed contributions
  are combined.

Do not add columns, comments, source names, provenance notes, or explanatory
text inside the CSV.

## Inputs You May Use

Use only clean-room numeric/spec inputs, including:

```text
TAME.BIT
testdata/oracle/handoff/decoder_tame_excitation_history_expected_template.csv
testdata/oracle/handoff/decoder_tame_stage_wide_expected.csv
testdata/oracle/handoff/tame_short_pitch_relation.csv
```

If your clean-room trace generator can decode TAME forward from frame 0, use
that trace to fill `excitation_u_q0`. If these values cannot be independently
derived without using implementation-derived local `got` values or external
implementation source code, do not guess. Return a short incomplete note
outside the CSV.

## Derived FIFO Check

Once frames `0..116` are filled, the frame 117 subframe 0 pre-ACB FIFO can be
assembled from the last 153 decoded excitation samples before frame 117:

```text
oldest -> newest:
frame 115 samples 7..79, then frame 116 samples 0..79
```

Using the local artifact convention, that means index `152` is `u(-1)`, the
most recent sample immediately before frame 117 subframe 0.

## Local Compare Command

After copying the filled file into `testdata/oracle/handoff`, the
implementation-side compare command is:

```sh
G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1 \
G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY=1 \
G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEExcitationHistory -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any other
G.729 implementation code. External decoder binaries are not needed for this
task; if a future task permits external tools, they may only be used as
black-box executable processes. Verifier output may enter this repository only
as numeric scalar oracle artifacts.
