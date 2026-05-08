# Encoder Closed-Loop Stage Verifier Prompt

You are an isolated clean-room numeric verifier. Do not inspect or
describe any G.729 implementation source code. Fill only numeric
`expected` cells in:

```text
/home/exedev/g729/testdata/oracle/handoff/encoder_closedloop_stage_expected_template.csv
```

Use the matching local-observation file only for row keys and local `got`
comparison after your independent calculation:

```text
/home/exedev/g729/testdata/oracle/handoff/encoder_closedloop_stage_got.csv
```

Do not copy `got` values into `expected`. That is a self-oracle and is
invalid.

## Source Material

Use the G.729 Annex A encoder equations and the test vector input:

```text
/home/exedev/g729/testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.IN
```

Target frames are fixed by the CSV key rows:

```text
0, 1, 2, 3, 4, 5,
100, 101,
500, 501,
1000, 1001,
1500, 1501,
2000, 2001,
2500, 2501,
2750, 2751,
3000, 3001,
3500, 3501
```

The handoff focuses on the encoder closed-loop chain after LSP
quantization:

- LP residual and target path: `r`, `x`, `h`, `xb`
- adaptive-codebook selection: `pitch_int`, `pitch_frac`, `v`, `y`,
  `unquant_gp_q14`
- fixed-codebook search: `x_prime`, `d`, `d_abs`, `sign`, `phi`,
  `fcb_position`, `fcb_position_sign`, `c`, `z`, `s_bits`, `c_bits`
- gain-search input surface: `gpc_pred_q12`,
  `gain_corr_A`, `gain_corr_B`, `gain_corr_C`, `gain_corr_D`,
  `gain_corr_F`, `ga_bits`, `gb_bits`

## CSV Contract

Header:

```csv
field,frame,sub,index,lag,frac,expected
```

Rows:

```text
100848 data rows plus header
```

Key columns:

```csv
field,frame,sub,index,lag,frac
```

Rules:

- Preserve the header exactly.
- Preserve row count, row order, and key columns exactly.
- Fill every `expected` cell with a signed decimal integer.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV.
- Do not run `G729_WRITE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1` after
  filling; that command regenerates blank expected cells.
- Treat `index=-1` as scalar/not-applicable.
- For `phi`, `index = i*40 + j`.
- Return only the filled CSV file, or a short completion note outside
  the CSV file.

## Local Strict Compare

After filling the expected template, run:

```sh
G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -count=1 -v
```

Expected outcome for a valid independent oracle is either exact PASS or
a useful mismatch report that identifies the first divergent fields.

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any
other G.729 implementation code. FFmpeg may be used only as an external
black-box executable for decode-quality checks, not as source material.
Verifier output may enter this repository only as numeric scalar oracle
artifacts, deltas, controlled notes, and aggregate histograms.
