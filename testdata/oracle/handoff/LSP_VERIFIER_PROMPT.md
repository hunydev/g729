# Clean-room LSP Verifier Prompt

Use this prompt for an external verifier that is isolated from this
repository's implementation work. The verifier must return only numeric
CSV cells in the provided templates.

## Prompt

You are acting as a clean-room numeric verifier for a Go G.729 encoder
project. Your task is to fill expected scalar values in the provided
CSV templates. Do not provide source code, implementation-derived names,
branch descriptions, source locations, provenance notes, or magic-number
explanations.

Clean-room boundary:

- Do not inspect this repository's Go implementation except for the
  CSV templates and any allowed numeric context columns already present
  in those templates.
- Do not send implementation code, pseudocode, branch descriptions,
  source file names, or algorithm explanations back to the repository.
- The only allowed returned artifact content is numeric scalar values in
  the `expected` column. If you cannot verify a cell, leave it blank.
- Allowed context is the G.729 specification, ITU test vectors, and the
  numeric columns present in the templates.

Fill these files:

1. `testdata/oracle/handoff/lsp_tables_expected_template.csv`
2. `testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv`
3. `testdata/oracle/handoff/lsp_frame0_vq_expected_template.csv`
4. `testdata/oracle/handoff/lsp_frame0_source_expected_template.csv`

Return the same files with the same row order and same headers. Only
the final `expected` column may change.

## File 1: LSP Table Expected Values

Input template:

```csv
table,selector,tap,row,col,expected
```

Key:

```csv
table,selector,tap,row,col
```

Fill `expected` with the verifier's signed integer scalar for that key.

Column rules:

- `table` identifies one numeric table family.
- `selector=-1,tap=-1` means the row belongs to an LSP codebook table.
- `row=-1` means the row belongs to an MA-predictor coefficient table.
- `col` is the coefficient/dimension index.
- Do not add columns.
- Do not change any key column.
- Do not add comments, notes, source names, or explanation rows.

Local strict verdict command after the file is filled:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

## File 2: LSP Predictor Residual Expected Values

Input template:

```csv
frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,expected
```

Comparison key:

```csv
frame,col
```

Fill `expected` with the verifier's committed LSP MA-predictor residual
scalar for the given frame and coefficient column.

Column rules:

- `frame` is zero-based.
- `col` is the LSP/LSF coefficient index, `0..9`.
- `selector,L1,L2,L3` are this implementation's local emitted indices.
- `ref_selector,ref_L1,ref_L2,ref_L3` are numeric context from the
  transmitted `LSP.BIT` vector for the same frame.
- The index columns are context only; the local comparison uses only
  `frame,col` as the key.
- Do not add columns.
- Do not change any context column.
- Do not add comments, notes, source names, or explanation rows.

Local strict verdict command after the file is filled:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

## File 3: LSP Frame-0 VQ Expected Values

Input template:

```csv
field,frame,selector,tap,L1,L2,L3,col,expected
```

Comparison key:

```csv
field,frame,selector,tap,L1,L2,L3,col
```

Fill `expected` with the verifier's signed integer scalar for that key.
This file is intentionally small and targets only frame 0 of `LSP.IN` /
`LSP.BIT`.

Column rules:

- `field` identifies the numeric scalar family.
- `frame` is zero-based and is `0` for every row in this template.
- `selector`, `tap`, `L1`, `L2`, `L3`, and `col` are numeric context
  keys. A value of `-1` means "not applicable" for that row.
- `initial_memory` rows request encoder-side MA predictor seed values.
- `target_lsf` rows request selector-specific target residual/LSF
  scalar values before codebook selection.
- `selected_index` rows request the verifier's emitted frame-0 LSP
  index tuple, with `col=0..3` corresponding to the four scalar index
  fields in order.
- `reference_index` rows are the transmitted `LSP.BIT` frame-0 index
  tuple and should remain numeric-only.
- `l1_cost`, `l1_rank`, `l23_pair_cost`, `l23_pair_rank`,
  `full_tuple_cost`, and `full_tuple_rank` rows request numeric costs
  or ranks on the verifier's frame-0 VQ surface for the keyed candidate.
- Do not add columns.
- Do not change any key column.
- Do not add comments, notes, source names, or explanation rows.

Local strict verdict command after the file is filled:

```sh
G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
```

## File 4: LSP Frame-0 Source Distinction Expected Values

Input template:

```csv
field,frame,col,expected
```

Comparison key:

```csv
field,frame,col
```

Fill `expected` with the verifier's signed integer scalar for that key.
This file intentionally asks only eight scalar values and is meant to
resolve the source-of-truth distinction for `LSP.IN` / `LSP.BIT` frame 0.

Column rules:

- `bitstream_index` rows request the tuple decoded directly from
  `LSP.BIT` frame 0.
- `encoder_selected_index` rows request the tuple actually emitted by
  the encoder when running `coder LSP.IN LSP.BIT` for frame 0.
- `col=0..3` corresponds to the four LSP index scalars in order:
  `L0,L1,L2,L3`.
- Do not copy this repository's `got` column into `expected`.
- If your clean-room verifier cannot independently distinguish
  `bitstream_index` from `encoder_selected_index`, leave the
  `encoder_selected_index` cells blank.
- Do not add columns.
- Do not change any key column.
- Do not add comments, notes, source names, or explanation rows.

Local strict verdict command after the file is filled:

```sh
G729_COMPARE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0SourceHandoff -v
```

## Completion Check

Before returning the filled templates:

- Every fillable `expected` cell is either a signed base-10 integer or
  intentionally blank.
- Headers are unchanged.
- Row count is unchanged.
- Row order is unchanged.
- No explanatory text was inserted into either CSV.
- No implementation source code or implementation-derived labels are
  present in the CSV files.
