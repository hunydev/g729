# LSP Decision Verifier Prompt

You are an isolated clean-room verifier. Do not return or describe
implementation code. Fill only numeric `expected` cells in:

```text
testdata/oracle/handoff/lsp_decision_expected_template.csv
```

Use the scalar keys exactly as provided:

```csv
field,frame,tap,L0,L1,L2,L3,col
```

Rules:

- Preserve the header, row count, row order, and key columns.
- Fill every `expected` cell with a signed decimal integer.
- Do not run `G729_WRITE_LSP_DECISION_HANDOFF=1` after filling; it
  regenerates a blank expected template.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV.
- Treat `-1` in a key column as "not applicable" for that scalar.
- The file covers the first 16 LSP-vector frames.
- Values are scalar snapshots of the LSP decision surface:
  predictor memory, input LSF `omega`, LSF weights, selector targets,
  local encoder-selected index tuple, transmitted `LSP.BIT` tuple,
  local tuple cost/rank, and transmitted tuple cost/rank.

After filling the template, the implementation-side compare command is:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v
```
