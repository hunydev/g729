# External Verifier Request

You are receiving a clean-room verifier handoff bundle. Treat the bundle
as the only project material you may inspect.

## Task

The focused fixed-codebook templates have already been filled by an
isolated verifier and exact-compared locally:

```text
testdata/oracle/handoff/fcb_tree_search_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv
```

Only fill this remaining blank CSV template when you can produce an
independent numeric oracle:

```text
testdata/oracle/handoff/encoder_closedloop_stage_expected_template.csv
```

Start with `testdata/oracle/handoff/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md`.
That file routes the remaining blank template to the detailed task prompt
and states the row keys, strict compare command, and clean-room rules.

## Return Contract

Return only the filled CSV templates, preserving each header, row count,
row order, and key column exactly. Every `expected` cell in a returned
CSV must be a signed decimal integer.

Do not return source code, pseudocode, implementation notes, branch
descriptions, provenance notes, comments inside CSV files, or long
explanations. A short completion note outside the CSV files is fine.

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any
other G.729 implementation source code. External G.729 tools may be used
only as black-box executables when a detailed prompt permits it. Output
that enters the repository must be limited to numeric scalar oracle
artifacts, deltas, controlled notes, and aggregate histograms.

## Important Notes

- Do not run any `G729_WRITE_*_HANDOFF=1` command after filling
  templates; those commands regenerate blank templates.
- The focused FCB tree-search templates are verifier-filled artifacts; do
  not overwrite them unless explicitly discarding verifier output.
- For the broad encoder closed-loop stage template, do not copy
  `encoder_closedloop_stage_got.csv` values into `expected`; that file is
  for row keys and later local comparison only.
