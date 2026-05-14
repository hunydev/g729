# Decoder Support Tables Prompt

You are an isolated clean-room verifier. Do not inspect, return, or describe
G.729 implementation source code. Return only numeric scalar answers, or a
short incomplete note outside the CSV if the requested values cannot be
derived independently.

Fill only the blank `expected` cells in:

```text
testdata/oracle/handoff/decoder_support_tables_expected_template.csv
```

Purpose: verify the small numeric tables needed before asking for full
forward decoder traces such as TAME frame `0..116` `excitation_u_q0`. Previous
forward-trace requests were blocked because these support tables were not
included as independent clean-room numeric inputs.

Known limitation: if the values exist only in ITU simulation-software data
tables or other implementation source, and cannot be derived from ITU-T G.729
Recommendation text/math under the clean-room boundary below, do not fill
those rows. In that case return an incomplete note outside the CSV instead of
guessing.

## Required Output

Return the same filename with exactly this header and row order:

```csv
table,row,col,expected
```

For every template row:

- `table` identifies the numeric table or scalar constant.
- `row` and `col` identify the scalar position; `-1` means not applicable.
- `expected` must be a signed decimal integer.

Do not add columns, comments, source names, provenance notes, or explanatory
text inside the CSV.

## Requested Tables

The template covers:

- `CosLSP`: 65 Q15 entries for LSF-to-LSP cosine lookup.
- `PitchInterpFIR`: 31 Q15 entries for 1/3-sample pitch interpolation.
- `Pow2Table`: 33 Q14 entries for fractional `2^x` reconstruction.
- `Log2Table`: 33 Q15 entries for fractional `log2` reconstruction.
- `GainGBK1`: 8x2 gain VQ stage-1 entries.
- `GainGBK2`: 16x2 gain VQ stage-2 entries.
- `GainMap1`, `GainImap1`: 8-entry gain index permutation tables.
- `GainMap2`, `GainImap2`: 16-entry gain index permutation tables.
- `GainMAPredictor`: 4 Q13 MA predictor coefficients.
- `GainMeanEnergyQ10`: scalar mean log-energy constant.
- `GainPastErrorsDefaultQ10`: scalar decoder gain-predictor initial error.

Use only ITU-T G.729 Recommendation text and clean-room mathematical
derivation for these numeric values. If a value cannot be independently
derived from that material, do not guess; leave the CSV unfilled and return a
short incomplete note outside the CSV. Do not use ITU simulation-software
source/data files as an oracle unless a future repository policy explicitly
allows those numeric artifacts.

## Local Compare Command

After copying the filled file into `testdata/oracle/handoff`, the
implementation-side compare command is:

```sh
G729_COMPARE_DECODER_SUPPORT_TABLES=1 \
G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES=1 \
G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES=1 \
go test -run TestOracleHandoff_CompareDecoderSupportTables -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any other
G.729 implementation code. External decoder binaries are not needed for this
task; if a future task permits external tools, they may only be used as
black-box executable processes. Verifier output may enter this repository only
as numeric scalar oracle artifacts.
