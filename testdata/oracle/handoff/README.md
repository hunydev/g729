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
- `fcb_tree_search_got.csv`: this implementation's fixed-codebook
  tree-search numeric surface for SPEECH.IN frames 292 through 294.
- `fcb_tree_search_expected_template.csv`: verifier-owned template
  for the exact reduced-complexity Annex A fixed-codebook tree-search
  scalar rows; currently verifier-filled and exact-compared.
- `FCB_TREE_SEARCH_VERIFIER_PROMPT.md`: copyable prompt for an isolated
  clean-room verifier that will fill only numeric `expected` cells in
  the FCB tree-search template.
- `fcb_tree_search_user_audio_got.csv`: this implementation's
  fixed-codebook tree-search numeric surface for converted user problem
  sample frames 292 through 294.
- `fcb_tree_search_user_audio_expected_template.csv`: verifier-owned
  template for the user-audio fixed-codebook tree-search scalar rows;
  currently verifier-filled and exact-compared.
- `FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier that will fill only numeric `expected`
  cells in the user-audio FCB tree-search template.
- `decoder_itu_stage_got.csv`: this implementation's selected ITU decoder
  vector stage trace for ALGTHM, TAME, and OVERFLOW localization frames.
- `decoder_itu_stage_expected_template.csv`: verifier-owned template for the
  selected decoder stage trace rows.
- `decoder_itu_stage_expected.csv`: partial verifier-filled decoder stage
  artifact. It is useful for localization, but it still contains blank
  `expected` cells and is not a complete strict gate.
- `DECODER_ITU_FCB_POSITION_CLARIFICATION_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier to clarify the three unresolved fixed-codebook
  fourth-pulse position decompositions from the partial decoder stage artifact.
- `decoder_itu_fcb_position_clarification_expected_template.csv`:
  verifier-owned template for that three-row clarification.
- `decoder_itu_fcb_position_clarification_expected.csv`: verifier-filled
  clarification artifact; currently exact-compared and resolves the three
  fixed-codebook fourth-pulse position decompositions.
- `decoder_tame_stage_wide_expected.csv`: verifier-filled wide numeric
  decoder-stage artifact for TAME frames 117 through 119. It contains
  subframe LP, adaptive/fixed gain, adaptive vector, fixed codebook,
  pitch/fixed contribution, excitation, and synthesis cells for localization.
- `decoder_tame_stage_wide_onset_got.csv`: this implementation's wide
  TAME decoder-stage trace for selected onset windows spanning frames 0..5,
  22..33, 49..60, 68..79, and 112..127.
- `decoder_tame_stage_wide_onset_expected_template.csv`: verifier-owned
  wide template for the same selected TAME onset windows. Blank cells are
  permitted when a value cannot be independently derived under the clean-room
  boundary; filled numeric cells can still be compared locally.
- `DECODER_TAME_STAGE_WIDE_ONSET_PROMPT.md`: copyable prompt for an isolated
  clean-room verifier that fills only independently derived numeric cells in
  the TAME onset wide template.
- `decoder_itu_stage_frame0_chain_expected.csv`: verifier-filled frame-0
  chain artifact for ALGTHM, TAME, and OVERFLOW. It contains subframe-0
  `fixed_c_q13`, final PST PCM, and inverse final-output-scale HP candidates;
  the `fixed_c_q13` rows exact-match after stream-start pitch sharpening uses
  the upper beta value before the first decoded pitch gain exists.
- `decoder_itu_frame0_hp_input_inverse_expected_template.csv`: verifier-filled
  frame-0 inverse HP-input ranges. It maps the verifier-owned final PST PCM
  rows to candidate `postfilter_s_q0` ranges before the decoder output HP
  filter. Local compare currently matches `170/240`, which localizes remaining
  disagreement before the HP filter input rather than to final scaling alone.
- `DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier that fills only numeric `expected` cells in the
  frame-0 HP-input inverse template.
- `decoder_tame_pre_acb_history_expected_template.csv`: verifier-owned
  template for the 153-sample TAME frame 117 subframe 0 past-excitation FIFO
  immediately before adaptive-codebook reconstruction. This is currently
  blocked unless an independent upstream excitation trace is available; the
  FIFO cannot be reconstructed uniquely from downstream `adaptive_v_q0` rows.
- `DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md`: copyable prompt for an isolated
  clean-room verifier that fills only numeric `expected` cells in the TAME
  pre-ACB history template.
- `decoder_tame_excitation_history_expected_template.csv`: verifier-owned
  template for TAME frame `0..116` decoded `excitation_u_q0` samples. This
  forward trace supplies the prior excitation needed to derive frame 117
  subframe 0 pre-ACB history. This is currently blocked until decoder support
  tables are independently verified.
- `DECODER_TAME_EXCITATION_HISTORY_PROMPT.md`: copyable prompt for an
  isolated clean-room verifier that fills only numeric `expected` cells in the
  TAME excitation-history template.
- `decoder_support_tables_expected_template.csv`: verifier-owned template for
  small decoder support tables and scalar constants required before broader
  forward traces can be independently generated. Full completion is currently
  blocked under the clean-room boundary because some gain VQ/map values are
  only available as simulation-software numeric tables, not Recommendation
  text/math.
- `DECODER_SUPPORT_TABLES_PROMPT.md`: copyable prompt for an isolated
  clean-room verifier that fills only numeric `expected` cells in the decoder
  support-table template.
- `tame_short_pitch_relation.csv`: verifier-produced numeric relation table
  derived from the TAME wide artifact. It documents the short-pitch
  `T_frac=0` relation used to justify the phase-0 FIR adaptive-codebook fix.
- `REMAINING_CONFORMANCE_VERIFIER_PROMPT.md`: consolidated verifier
  request for the currently unfilled encoder closed-loop stage
  conformance handoff template, with completed FCB status noted.
- `LSP_VERIFIER_PROMPT.md`: copyable prompt for an isolated clean-room
  verifier that will fill only numeric `expected` cells in the LSP
  templates.
- `HANDOFF_MANIFEST.md`: row counts, headers, and pre-fill SHA-256
  hashes for the LSP handoff input files.

The current LSP handoff completion audit is documented in
`docs/superpowers/plans/2026-05-06-lsp-oracle-handoff-audit.md`.

Active verifier bundle:

```sh
sh testdata/oracle/handoff/create_verifier_bundle.sh
```

The bundle intentionally contains only prompts, manifest/docs, blank
`expected` templates, and numeric `got` CSVs. It does not contain source
code or external implementation material. The repo-local helper uses
deterministic tar/gzip options (`--sort=name`, fixed `--mtime`,
`--numeric-owner`, and `gzip -n`) so the archive hash is stable for a
fixed set of input files. By default it refuses to build if the remaining
outgoing blank `expected` template already has verifier-filled cells.

The helper copies exactly these files into the bundle:

```text
testdata/oracle/handoff/HANDOFF_MANIFEST.md
testdata/oracle/handoff/README.md
testdata/oracle/handoff/EXTERNAL_VERIFIER_REQUEST.md
testdata/oracle/handoff/create_verifier_bundle.sh
testdata/oracle/handoff/validate_verifier_output.sh
testdata/oracle/handoff/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md
testdata/oracle/handoff/FCB_TREE_SEARCH_VERIFIER_PROMPT.md
testdata/oracle/handoff/FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md
testdata/oracle/handoff/ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md
testdata/oracle/handoff/fcb_tree_search_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_got.csv
testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv
testdata/oracle/handoff/encoder_closedloop_stage_expected_template.csv
testdata/oracle/handoff/encoder_closedloop_stage_got.csv
```

Filled verifier output intake:

1. Put only the verifier-returned numeric `expected` CSV files in a
   temporary directory, then validate them before copying:

   ```sh
   sh testdata/oracle/handoff/validate_verifier_output.sh /path/to/returned-csv-dir
   ```

   The validator rejects unexpected files, symlinked files, changed
   headers, changed row counts, changed key columns, blank `expected`
   cells, and non-numeric `expected` cells unless that artifact explicitly
   allows controlled note cells or partial blank cells. It is
   validation-only by default.
2. To copy validated files into their exact template paths, rerun with:

   ```sh
   G729_APPLY_VERIFIER_OUTPUT=1 \
   sh testdata/oracle/handoff/validate_verifier_output.sh /path/to/returned-csv-dir
   ```

3. Do not run any `G729_WRITE_*_HANDOFF=1` command after copying filled
   templates; those commands regenerate blank templates.
4. Do not run `create_verifier_bundle.sh` for incoming verifier output.
   That helper is for outgoing verifier bundles and refuses filled
   remaining-blank templates by default.
5. Run the strict compare command for each filled template:

   ```sh
   G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
   G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
   G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
   go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v

   G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
   G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
   G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
   go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v

   G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
   G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
   G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
   go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -count=1 -v

   G729_COMPARE_DECODER_TAME_STAGE_WIDE=1 \
   G729_REQUIRE_EXACT_DECODER_TAME_STAGE_WIDE=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEStageWide -count=1 -v

   G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse -count=1 -v

   G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1 \
   G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY=1 \
   G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEPreACBHistory -count=1 -v

   G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1 \
   G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY=1 \
   G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEExcitationHistory -count=1 -v

   G729_COMPARE_DECODER_SUPPORT_TABLES=1 \
   G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES=1 \
   G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES=1 \
   go test -run TestOracleHandoff_CompareDecoderSupportTables -count=1 -v
   ```

6. If strict compare passes, update `HANDOFF_MANIFEST.md` and the
   audit docs from "currently unfilled" to verifier-filled status, then
   rerun the default handoff guards. Until that metadata is updated,
   `TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles` is
   expected to fail because the previously blank templates are no longer
   blank.

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

FCB tree-search workflow:

1. Refresh the fixed-codebook tree-search surface and expected-value
   template:

   ```sh
   G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_WriteFCBTreeSearchHandoff -v
   ```

2. Give the verifier both `fcb_tree_search_got.csv` and
   `fcb_tree_search_expected_template.csv`, then ask it to fill
   `expected` in the template using `FCB_TREE_SEARCH_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,sub,index
   ```

4. `index=-1` means "scalar field, no element index". The `got` file
   includes `d_abs`, `sign`, `phi`, focused selected positions/scores,
   full-search positions/scores, and threshold/accepted-prefix scalars.
   The expected template intentionally keeps only the key columns and the
   blank verifier-owned `expected` column.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -v
   ```

User-audio FCB tree-search workflow:

1. Refresh the user problem-sample fixed-codebook tree-search surface
   and expected-value template. This workflow is pinned to
   `testdata/external/user_quality_audio.m4a`; do not substitute another
   sample for this template:

   ```sh
   G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 go test -run TestOracleHandoff_WriteFCBTreeSearchUserAudioHandoff -v
   ```

2. Give the verifier both `fcb_tree_search_user_audio_got.csv` and
   `fcb_tree_search_user_audio_expected_template.csv`, then ask it to fill
   `expected` in the template using
   `FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,sub,index
   ```

4. `index=-1` means "scalar field, no element index". The `got` file
   includes `d_abs`, `sign`, `phi`, focused selected positions/scores,
   full-search positions/scores, and threshold/accepted-prefix scalars.
   The expected template intentionally keeps only the key columns and the
   blank verifier-owned `expected` column.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -v
   ```

Decoder frame-0 HP-input inverse workflow:

1. Refresh the blank template from the existing verifier-filled frame-0 chain
   artifact:

   ```sh
   G729_WRITE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE_TEMPLATE=1 \
   go test ./internal/decoder -run TestDecoderITUFrame0HPInputInverseTemplate -count=1 -v
   ```

2. Give the verifier both `decoder_itu_stage_frame0_chain_expected.csv` and
   `decoder_itu_frame0_hp_input_inverse_expected_template.csv`, then ask it to
   fill `expected` using
   `DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   source,frame,sub,field,index
   ```

4. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse -count=1 -v
   ```

   Current result: complete numeric oracle, non-exact local compare
   (`170/240` within range). Use it as a localization artifact showing that
   the remaining frame-0 mismatch enters before the output HP filter.

Decoder TAME stage-wide onset workflow:

1. Refresh the local `got` CSV and blank verifier template:

   ```sh
   G729_WRITE_DECODER_TAME_STAGE_WIDE_ONSET_HANDOFF=1 \
   go test ./internal/decoder -run TestDecoderTAMEStageWideOnsetHandoffTemplate -count=1 -v
   ```

2. Give the verifier `decoder_tame_stage_wide_onset_expected_template.csv`,
   `decoder_tame_stage_wide_onset_got.csv`, `TAME.BIT`, and `TAME.PST`, then
   ask it to fill independently derived numeric cells using
   `DECODER_TAME_STAGE_WIDE_ONSET_PROMPT.md`.
3. Preserve the exact CSV shape. The wide columns cover `past_exc_pre_acb_q0`,
   `lp_a_q12`, adaptive/fixed gains, adaptive/fixed vectors, pitch/fixed
   contributions, excitation, and synthesis for selected TAME onset windows.
4. After the verifier returns a CSV, compare the filled numeric cells locally:

   ```sh
   G729_COMPARE_DECODER_TAME_STAGE_WIDE_ONSET=1 \
   G729_DECODER_TAME_STAGE_WIDE_ONSET_EXPECTED=/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_stage_wide_onset_expected_template.csv \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEStageWideOnset -count=1 -v
   ```

   Set `G729_REQUIRE_EXACT_DECODER_TAME_STAGE_WIDE_ONSET=1` only when all
   filled verifier cells must match local values. Set
   `G729_REQUIRE_COMPLETE_DECODER_TAME_STAGE_WIDE_ONSET=1` only if the verifier
   explicitly reports that every requested cell was independently derived.
   Current status: blank template, 116 data rows and 406 value columns.

Decoder TAME pre-ACB history workflow:

1. Refresh the blank template:

   ```sh
   G729_WRITE_DECODER_TAME_PRE_ACB_HISTORY_TEMPLATE=1 \
   go test ./internal/decoder -run TestDecoderTAMEPreACBHistoryTemplate -count=1 -v
   ```

2. Give the verifier `decoder_tame_pre_acb_history_expected_template.csv`,
   `decoder_tame_stage_wide_expected.csv`, and `tame_short_pitch_relation.csv`,
   then ask it to fill `expected` using
   `DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   source,frame,sub,field,index
   ```

4. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1 \
   G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY=1 \
   G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEPreACBHistory -count=1 -v
   ```

   Current status: the verifier reported this cannot be filled from only the
   provided downstream artifacts. Treat this template as blocked until a
   clean-room forward excitation trace is available.

Decoder TAME excitation-history workflow:

1. Refresh the blank template:

   ```sh
   G729_WRITE_DECODER_TAME_EXCITATION_HISTORY_TEMPLATE=1 \
   go test ./internal/decoder -run TestDecoderTAMEExcitationHistoryTemplate -count=1 -v
   ```

2. Give the verifier `decoder_tame_excitation_history_expected_template.csv`,
   `decoder_tame_stage_wide_expected.csv`, and `tame_short_pitch_relation.csv`,
   then ask it to fill `expected` using
   `DECODER_TAME_EXCITATION_HISTORY_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   source,frame,sub,field,index
   ```

4. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1 \
   G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY=1 \
   G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY=1 \
   go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEExcitationHistory -count=1 -v
   ```

   Current status: the verifier reported this cannot be filled from the
   previously provided inputs because the full forward decode also requires
   independently verified support tables and state that are not present in the
   current clean-room artifacts.

Decoder support-table workflow:

1. Refresh the blank template:

   ```sh
   G729_WRITE_DECODER_SUPPORT_TABLES_TEMPLATE=1 \
   go test -run TestOracleHandoff_WriteDecoderSupportTablesTemplate -count=1 -v
   ```

2. Give the verifier `decoder_support_tables_expected_template.csv`, then ask
   it to fill `expected` using `DECODER_SUPPORT_TABLES_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   table,row,col
   ```

4. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_DECODER_SUPPORT_TABLES=1 \
   G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES=1 \
   G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES=1 \
   go test -run TestOracleHandoff_CompareDecoderSupportTables -count=1 -v
   ```

   Current status: the verifier reported the full 264-row file cannot be
   completed under the current clean-room boundary. Spec text independently
   covers only a subset of scalar constants; the gain VQ/map tables are
   simulation-software numeric tables and should not be guessed.

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

Encoder closed-loop stage workflow:

1. Refresh the SPEECH frame subset template:

   ```sh
   G729_WRITE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 go test -run TestOracleHandoff_WriteEncoderClosedLoopStageHandoff -v
   ```

2. Ask the verifier to fill `expected` in
   `encoder_closedloop_stage_expected_template.csv` using
   `ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md`.
3. Compare only numeric scalar values keyed by:

   ```csv
   field,frame,sub,index,lag,frac
   ```

4. `index=-1` means scalar/not applicable. For `phi`,
   `index = i*40 + j`.
5. After the verifier fills numeric `expected` cells, compare locally:

   ```sh
   G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -v
   ```

   For a complete exact verdict, require every cell to be filled and
   fail on any mismatch:

   ```sh
   G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -v
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
