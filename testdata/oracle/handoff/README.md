# H-CENTER Oracle Handoff

These files are not oracle artifacts and are intentionally ignored by the optional oracle validator because they live in a subdirectory.

- `pitch_top_open_loop_got.csv`: this implementation's frame-level open-loop `T_op` values.
- `pitch_top_open_loop_expected_template.csv`: verifier-owned template for raw oracle `T_op` values.

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
