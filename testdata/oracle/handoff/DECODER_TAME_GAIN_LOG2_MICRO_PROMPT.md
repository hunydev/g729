# Decoder TAME Gain Log2 Micro Oracle Prompt

Run this in the external verifier workspace, not inside `/home/exedev/g729`:

```text
/home/exedev/g729_untracked
```

The repository may receive only numeric oracle artifacts. Do not copy source code, pseudocode, branch descriptions, table provenance, or implementation notes into `/home/exedev/g729`.

## Goal

Generate a focused numeric oracle for the TAME decoder gain path around the current first exact mismatch frontier:

```text
fixed-codebook energy -> Log2 -> Ebar_c -> log gain -> Log2/Pow2 gain reconstruction
```

Existing artifact:

```text
/home/exedev/g729_untracked/verifier-output/decoder_tame_gain_internals_expected.csv
```

Current local diagnostics show:

- bitstream GA/GB and gamma VQ indices match.
- For subframes where local `fixed_codebook_energy_q26` already matches, local `ec_bar_q10` still mismatches.
- Simple constant and rounding sweeps do not make `ec_bar_q10` exact.

So the next artifact must expose the integer `Log2` micro-steps used by the verifier/reference execution.

## Required Output

Write:

```text
/home/exedev/g729_untracked/verifier-output/decoder_tame_gain_log2_micro_expected.csv
```

CSV schema:

```csv
source,frame,sub,field,index,expected,note
```

Rules:

- `source` must be `TAME`.
- `frame` should cover TAME frames `0..127`.
- `sub` must be `0` or `1`.
- `field` must be one of the field names below.
- `index` must be `0`.
- `expected` must be a signed decimal integer.
- `note` must be the controlled value `reference_decoder_execution`.
- No blank expected cells.
- Do not include comments or extra columns in the CSV.

## Fields

Emit these fields for each TAME frame/subframe:

```text
ec_energy_q26
ec_log2_input_q0
ec_log2_norm_shift_q0
ec_log2_norm_x_q0
ec_log2_int_part_q0
ec_log2_frac30_q0
ec_log2_table_index_q0
ec_log2_fraction_q0
ec_log2_table0_q15
ec_log2_table1_q15
ec_log2_interp_product_q30
ec_log2_frac_q15
ec_log2_raw_q10
ec_log2_corrected_q10
ec_db_q10
ec_bar_db_q10
gamma_q13
gamma_log2_input_q0
gamma_log2_norm_shift_q0
gamma_log2_norm_x_q0
gamma_log2_int_part_q0
gamma_log2_frac30_q0
gamma_log2_table_index_q0
gamma_log2_fraction_q0
gamma_log2_table0_q15
gamma_log2_table1_q15
gamma_log2_interp_product_q30
gamma_log2_frac_q15
gamma_log2_raw_q10
gamma_log2_corrected_q10
predicted_q10
log_gain_q10
log2_gc_q10
gc0_q14
fixed_gain_q14
u_current_q10
```

If the reference implementation uses a differently named but equivalent intermediate, map it to the field above by numeric meaning. Return numeric values only.

## Validation To Report

Report:

- output path
- row count
- filled expected count
- blank expected count
- non-integer expected count
- SHA256 of the CSV
- confirmation that TAME final PCM still matches official TAME.PST exactly
- blockers, if any

## Local Compare Command

After the artifact exists, run this in `/home/exedev/g729`:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO=1 \
  go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceTAMEGainLog2Micro -count=1 -v
```

For a strict gate:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO=1 \
  G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_GAIN_LOG2_MICRO=1 \
  go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceTAMEGainLog2Micro -count=1 -v
```
