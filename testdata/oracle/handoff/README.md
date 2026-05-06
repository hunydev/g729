# H-CENTER Oracle Handoff

These files are not oracle artifacts and are intentionally ignored by the optional oracle validator because they live in a subdirectory.

- `pitch_top_open_loop_got.csv`: this implementation's frame-level open-loop `T_op` values.
- `pitch_top_open_loop_expected_template.csv`: verifier-owned template for raw oracle `T_op` values.
- `lsp_tables_got.csv`: this implementation's LSP VQ table and MA-predictor scalar values.
- `lsp_tables_expected_template.csv`: verifier-owned template for LSP VQ table and MA-predictor scalar values.
- `lsp_predictor_residual_got.csv`: this implementation's committed LSP MA-predictor residual trajectory.
- `lsp_predictor_residual_expected_template.csv`: verifier-owned template for committed LSP MA-predictor residual trajectory.
- `lsp_frame0_vq_got.csv`: this implementation's frame-0 LSP VQ seed,
  target, selected-index, cost, and rank scalars.
- `lsp_frame0_vq_expected_template.csv`: verifier-owned template for
  frame-0 LSP VQ seed, target, selected-index, cost, and rank scalars.
- `lsp_frame0_source_got.csv`: this implementation's eight-value
  frame-0 source distinction dump: transmitted `LSP.BIT` tuple and
  local encoder-selected tuple.
- `lsp_frame0_source_expected_template.csv`: verifier-owned template
  for independently distinguishing transmitted frame-0 index values
  from the actual encoder-selected frame-0 index values.
- `lsp_decision_got.csv`: this implementation's first-16-frame LSP
  decision input, selected tuple, transmitted tuple, cost, and rank
  scalar dump.
- `lsp_decision_expected_template.csv`: verifier-owned template for
  the same multi-frame LSP decision scalar rows.
- `LSP_DECISION_VERIFIER_PROMPT.md`: copyable prompt for an isolated
  clean-room verifier that will fill only numeric `expected` cells in
  the multi-frame LSP decision template.
- `pitch_closedloop_search_got.csv`: this implementation's first-four
  PITCH-frame closed-loop pitch search input and RN-score scalar dump.
- `pitch_closedloop_search_expected_template.csv`: verifier-owned
  template for the same closed-loop pitch search scalar rows.
- `PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier that will fill only numeric `expected`
  cells in the pitch closed-loop search template.
- `tame_gain_taming_got.csv`: this implementation's all-frame TAME
  gain/taming path scalar dump.
- `tame_gain_taming_expected_template.csv`: verifier-owned template
  for the TAME gain/taming scalar rows.
- `TAME_GAIN_TAMING_VERIFIER_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier that will fill only numeric `expected`
  cells in the TAME gain/taming template.
- `REMAINING_CONFORMANCE_VERIFIER_PROMPT.md`: consolidated verifier
  request for the two still-unfilled conformance handoff templates.
- `LSP_VERIFIER_PROMPT.md`: copyable prompt for an isolated clean-room
  verifier that will fill only numeric `expected` cells in the LSP
  templates.
- `HANDOFF_MANIFEST.md`: row counts, headers, and pre-fill SHA-256
  hashes for the LSP handoff input files.

The current LSP handoff completion audit is documented in
`docs/superpowers/plans/2026-05-06-lsp-oracle-handoff-audit.md`.

Write-refresh tests refuse to overwrite any expected template that
already has verifier-filled cells. To intentionally discard verifier
output and regenerate a blank template, set
`G729_OVERWRITE_VERIFIER_EXPECTED=1` together with the relevant
`G729_WRITE_*_HANDOFF=1` variable.

Verifier workflow:

1. Fill `expected_top_open_loop` for every frame in the template.
2. Merge the filled template with this implementation's `got` values:

   ```sh
   G729_MERGE_ORACLE_HANDOFF=1 go test -run TestOracleHCenter_MergeTopOpenLoopHandoff -v
   ```

3. The merge writes a clean-room oracle artifact at `testdata/oracle/pitch_top_open_loop.csv` with:

   ```csv
   vector,frame,subframe,field,expected,got,delta,notes
   PITCH,0,-1,top_open_loop,<expected>,<got>,<got-expected>,mismatch
   ```

4. Use only controlled notes: `mismatch`, `out_of_window`, `range_ok`, `range_fail`, or `unknown`.
5. Do not include implementation details, code names, source locations, or explanations for oracle internals.

LSP table workflow:

1. Refresh this implementation's scalar table dump and expected-value template:

   ```sh
   G729_WRITE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_WriteLSPTableHandoff -v
   ```

2. Ask the verifier to fill `expected` in `lsp_tables_expected_template.csv`
   using `LSP_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   table,selector,tap,row,col
   ```

4. `selector` and `tap` are `-1` for non-MA tables. `row` is `-1` for
   `MAPredictorsLSP`.
5. The verifier must not add implementation names, source locations,
   branch descriptions, or provenance notes.
6. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
   ```

LSP predictor residual workflow:

1. Refresh this implementation's frame-by-frame committed residual dump
   and expected-value template:

   ```sh
   G729_WRITE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPPredictorResidualHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `lsp_predictor_residual_expected_template.csv` using
   `LSP_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   frame,col
   ```

4. The index columns `selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3`
   are included only as numeric context for the verifier.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
   ```

LSP frame-0 VQ workflow:

1. Refresh this implementation's frame-0 diagnostic scalar dump and
   expected-value template:

   ```sh
   G729_WRITE_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0VQHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `lsp_frame0_vq_expected_template.csv` using
   `LSP_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,selector,tap,L1,L2,L3,col
   ```

4. `-1` in a key column means "not applicable" for that scalar row.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
   ```

LSP frame-0 source distinction workflow:

1. Refresh the eight-value source distinction template:

   ```sh
   G729_WRITE_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0SourceHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `lsp_frame0_source_expected_template.csv` using
   `LSP_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,col
   ```

4. `bitstream_index` means the tuple decoded directly from `LSP.BIT`
   frame 0. `encoder_selected_index` means the tuple actually emitted
   by the encoder when running `coder LSP.IN LSP.BIT` for frame 0.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0SourceHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0SourceHandoff -v
   ```

LSP multi-frame decision workflow:

1. Refresh the first-16-frame LSP decision template:

   ```sh
   G729_WRITE_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPDecisionHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `lsp_decision_expected_template.csv` using
   `LSP_DECISION_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,tap,L0,L1,L2,L3,col
   ```

4. `-1` in a key column means "not applicable" for that scalar row.
   Rows include predictor memory, input LSF, weights, selector targets,
   local encoder-selected tuple, transmitted `LSP.BIT` tuple, and
   tuple cost/rank scalars.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v
   ```

Pitch closed-loop search workflow:

1. Refresh the first-four-frame search input and RN-score template:

   ```sh
   G729_WRITE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_WritePitchClosedLoopSearchInputHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `pitch_closedloop_search_expected_template.csv` using
   `PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,sub,index,lag,frac
   ```

4. `-1` in a key column means "not applicable" for that scalar row.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_COMPLETE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_EXACT_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff -v
   ```

TAME gain/taming workflow:

1. Refresh the all-frame TAME gain/taming template:

   ```sh
   G729_WRITE_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_WriteTAMEGainTamingHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `tame_gain_taming_expected_template.csv` using
   `TAME_GAIN_TAMING_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,sub,index
   ```

4. `-1` in a key column means "not applicable" for that scalar row.
   Rows include local/transmitted LSP, pitch, FCB, and gain fields,
   selected integer/fraction pitch, previous gain/taming state,
   old-excitation energy summaries, `pastQuaEn`, selected quantized
   gains, taming flag, and commit-energy summaries.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v
   ```
