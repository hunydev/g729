# Clean-room Oracle Artifacts

This directory is for optional verifier artifacts. The main test suite must pass when no artifact files are present.

Artifacts may be CSV (`*.csv`) or JSONL (`*.jsonl`). They must contain only numeric scalar comparisons and controlled notes.

Files under `testdata/oracle/handoff/` are verifier handoff material, not oracle artifacts. They are ignored by the optional artifact validator until a verifier produces a completed `.csv` or `.jsonl` file directly under `testdata/oracle/`.

## CSV Schema

Required header:

```csv
vector,frame,subframe,field,expected,got,delta,notes
```

Example:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
PITCH,1,-1,top_open_loop,82,85,3,mismatch
```

## JSONL Schema

Each line is one object with the same fields:

```json
{"vector":"PITCH","frame":0,"subframe":-1,"field":"top_open_loop","expected":74,"got":74,"delta":0,"notes":"range_ok"}
```

## Field Rules

- `vector`: non-empty short vector identifier.
- `frame`: zero-based frame index.
- `subframe`: `-1` for frame-level fields, otherwise `0` or `1`.
- `field`: non-empty scalar field identifier such as `P1`, `P2`, or `top_open_loop`.
- `expected`: verifier oracle scalar.
- `got`: this implementation's scalar.
- `delta`: `got - expected`.
- `notes`: one of `mismatch`, `out_of_window`, `range_ok`, `range_fail`, `unknown`.

## Forbidden Content

Artifacts must not include implementation code, implementation-derived names, source locations, branch descriptions, magic-number explanations, or names/URLs of external G.729 implementations. The Go validator rejects artifact files containing high-risk tokens.

## H-CENTER Raw `T_op`

For future raw open-loop pitch verification, use:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
```

`expected` is the verifier-provided raw open-loop pitch. `got` is this implementation's `T_op`. The optional H-CENTER diagnostic reports exact, `±1`, `±2`, `±5`, and `±10` rates plus a delta histogram.

To refresh the handoff files for an external verifier:

```sh
G729_WRITE_ORACLE_HANDOFF=1 go test -run TestOracleHCenter_WriteTopOpenLoopHandoff -v
```

After the verifier fills `testdata/oracle/handoff/pitch_top_open_loop_expected_template.csv`, merge it into a validator-ready artifact:

```sh
G729_MERGE_ORACLE_HANDOFF=1 go test -run TestOracleHCenter_MergeTopOpenLoopHandoff -v
```

## LSP Table / Predictor Numeric Handoff

For LSP cold-start and predictor-trajectory diagnostics, use the
handoff-only table files:

```sh
G729_WRITE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_WriteLSPTableHandoff -v
```

This writes:

- `testdata/oracle/handoff/lsp_tables_got.csv`
- `testdata/oracle/handoff/lsp_tables_expected_template.csv`

These files are intentionally not validator artifacts. They are for an
external clean-room verifier to fill numeric scalar `expected` values
for table and coefficient cells. The key columns are:

```csv
table,selector,tap,row,col
```

Use `selector=-1,tap=-1` for LSP codebooks, and `row=-1` for
`MAPredictorsLSP`. Do not include source names, code snippets,
provenance notes, or implementation details.

Use `testdata/oracle/handoff/LSP_VERIFIER_PROMPT.md` as the verifier
handoff prompt, and use `testdata/oracle/handoff/HANDOFF_MANIFEST.md`
to verify row counts, headers, and pre-fill hashes. The current local
completion audit is documented in
`docs/superpowers/plans/2026-05-06-lsp-oracle-handoff-audit.md`.

After the verifier fills numeric `expected` cells, compare the handoff:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

For a complete exact verdict:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

## LSP Predictor Residual Numeric Handoff

For frame-by-frame LSP MA-predictor residual trajectory verification,
use:

```sh
G729_WRITE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPPredictorResidualHandoff -v
```

This writes:

- `testdata/oracle/handoff/lsp_predictor_residual_got.csv`
- `testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv`

The verifier fills numeric `expected` residual cells keyed by:

```csv
frame,col
```

The other columns are numeric context only: local emitted LSP indices
and the transmitted `LSP.BIT` indices for the same frame. After the
verifier fills `expected`, compare:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

For a complete exact verdict:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

## LSP Frame-0 VQ Numeric Handoff

For the narrowed frame-0 LSP cold-start decision, use:

```sh
G729_WRITE_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0VQHandoff -v
```

This writes:

- `testdata/oracle/handoff/lsp_frame0_vq_got.csv`
- `testdata/oracle/handoff/lsp_frame0_vq_expected_template.csv`

The verifier fills numeric `expected` cells keyed by:

```csv
field,frame,selector,tap,L1,L2,L3,col
```

Rows cover only frame 0 and include seed memory, selector targets,
emitted/reference indices, and selected VQ cost/rank scalars. After the
verifier fills `expected`, compare:

```sh
G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
```

For a complete exact verdict:

```sh
G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
```

## LSP Frame-0 Source Distinction Handoff

To resolve whether frame 0's transmitted `LSP.BIT` tuple and the
encoder-selected tuple are being treated as the same source, use:

```sh
G729_WRITE_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0SourceHandoff -v
```

This writes:

- `testdata/oracle/handoff/lsp_frame0_source_got.csv`
- `testdata/oracle/handoff/lsp_frame0_source_expected_template.csv`

The verifier fills numeric `expected` cells keyed by:

```csv
field,frame,col
```

Compare after fill:

```sh
G729_COMPARE_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0SourceHandoff -v
```
