# Decoder ITU Frame-0 HP Input Inverse Prompt

You are an isolated clean-room verifier. Do not inspect, return, or describe
G.729 implementation source code. Return only the filled numeric CSV artifact.

Fill only the blank `expected` cells in:

```text
testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv
```

Use these existing numeric oracle inputs:

```text
testdata/oracle/handoff/decoder_itu_stage_frame0_chain_expected.csv
testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv
```

The frame-0 chain file provides verifier-owned final PST PCM values for
ALGTHM, TAME, and OVERFLOW frame 0 subframes 0 and 1. The new template asks for
the integer Q0 postfilter-output sample range that could have entered the
decoder output high-pass filter before the final `x2` PCM scale.

## Required Output

Return the same filename with exactly this header and row order:

```csv
source,frame,sub,field,index,expected
```

For every template key, fill `expected` as a signed decimal integer:

- `postfilter_inverse_low_q0`: the lowest integer Q0 HP-filter input sample that
  can produce the corresponding PST final PCM sample under the recurrence below.
- `postfilter_inverse_high_q0`: the highest integer Q0 HP-filter input sample
  that can produce the corresponding PST final PCM sample under the recurrence
  below.

Do not add columns, comments, notes, provenance, source names, or explanatory
text inside the CSV.

## HP Filter Model

Use the G.729 decoder output HP filter from section 4.2.2:

```text
H(z) = (b0 + b1*z^-1 + b2*z^-2) / (1 + a1*z^-1 + a2*z^-2)

b0 = +0.93980581
b1 = -1.8795834
b2 = +0.93980581
a1 = -1.9330735
a2 = +0.93589199
```

Use the equivalent fixed-point coefficients:

```text
b0_q13 = 7699
b1_q13 = -15399
b2_q13 = 7699
neg_a1_q12 = 7918
a2_q13 = 7667
```

For each sample `n`, with HP input `x[n]` as signed Q0 integer, previous input
state `x1=x[n-1]`, `x2=x[n-2]`, previous accumulator states `y1`, `y2` as Q12
integers, compute:

```text
ff_q13 = b0_q13*x[n] + b1_q13*x1 + b2_q13*x2
ff_q12 = arithmetic_shift_right(ff_q13, 1)

fb_q12 = arithmetic_shift_right(neg_a1_q12*y1, 12)
       - arithmetic_shift_right(a2_q13*y2, 13)

acc_q12 = ff_q12 + fb_q12
hp_q0 = clamp_to_int16(arithmetic_shift_right(acc_q12 + 2048, 12))

x2 = x1
x1 = x[n]
y2 = y1
y1 = acc_q12
```

The first sample of frame 0 starts with zero HP state:

```text
x1 = 0
x2 = 0
y1 = 0
y2 = 0
```

The final PST PCM in `decoder_itu_stage_frame0_chain_expected.csv` is the output
after the final saturating scale by 2:

```text
pst_pcm_q0 = clamp_to_int16(2 * hp_q0)
```

Therefore each final PST PCM sample maps to a possible `hp_q0` set. For example,
non-saturated even outputs normally identify one HP output exactly, while odd
or saturated final PCM values may imply a small range. Then use the HP
recurrence above to find the lowest and highest integer Q0 `x[n]` values that
can produce any allowed `hp_q0`, while preserving the already-derived feasible
state range from prior samples.

If a sample range is wider than expected, still return the mathematically
valid lowest and highest integer Q0 values. If the range cannot be narrowed
with the allowed inputs, return the full signed 16-bit range:

```text
-32768
32767
```

Do not leave any `expected` cell blank.

## Local Compare Command

After copying the filled file into `testdata/oracle/handoff`, the
implementation-side compare command is:

```sh
G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any other
G.729 implementation code. External decoder binaries are not needed for this
task; if a future task permits external tools, they may only be used as
black-box executable processes. Verifier output may enter this repository only
as numeric scalar oracle artifacts.
