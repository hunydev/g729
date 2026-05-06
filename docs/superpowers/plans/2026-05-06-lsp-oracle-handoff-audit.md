# LSP Oracle Handoff Audit

Date: 2026-05-06

This audit maps the active clean-room LSP recovery goal to concrete
repository evidence. It is intentionally conservative: a requirement is
not complete unless the corresponding numeric verifier artifact exists
and the local strict comparison gate has covered it.

## Objective Restatement

Complete the clean-room LSP table and predictor oracle handoff, use
verifier-filled numeric artifacts to determine whether the mismatch is
in LSP table constants or frame-by-frame MA predictor residual
trajectory, then make only oracle-supported production changes and
rerun the default build/test/race gates.

## Prompt-to-artifact Checklist

| Requirement | Evidence | Status |
| --- | --- | --- |
| Keep clean-room boundary: no ITU reference C, bcg729, FFmpeg, Sipro, or other implementation code. | `AGENTS.md` repository rule plus `testdata/oracle/handoff/LSP_VERIFIER_PROMPT.md` forbid implementation code, implementation-derived labels, branch descriptions, source locations, and provenance notes. | Satisfied for local process; must be maintained by verifier. |
| Allowed inputs only numeric oracle artifacts, ITU vectors, public spec interpretation. | Handoff prompt restricts returned content to numeric scalar `expected` cells. Existing handoff tests consume CSV scalar values only. | Satisfied for handoff design. |
| Stabilize current LSP table handoff generation path. | `internal/tables/lsp_oracle_handoff_test.go` provides `TestOracleHandoff_WriteLSPTableHandoff`, gated by `G729_WRITE_LSP_TABLE_HANDOFF=1`. | Implemented. |
| Stabilize current LSP table compare path. | `TestOracleHandoff_CompareLSPTableHandoff` compares `lsp_tables_got.csv` and `lsp_tables_expected_template.csv`; strict gates are `G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1` and `G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1`. | Implemented. |
| Generate LSP table handoff files. | `testdata/oracle/handoff/lsp_tables_got.csv` and `testdata/oracle/handoff/lsp_tables_expected_template.csv` exist. | Implemented. |
| Identify exact handoff input version for the external verifier. | `testdata/oracle/handoff/HANDOFF_MANIFEST.md` records headers, row counts, and pre-fill SHA-256 hashes. | Implemented. |
| Guard LSP handoff template structure in the default test suite. | `TestOracleHandoff_LSPStructuralIntegrity` checks headers, row counts, key alignment, and numeric `got` cells while allowing blank-or-numeric verifier `expected` cells. `TestOracleHandoff_LSPManifestMatchesCurrentFiles` checks the manifest's row/header entries. | Implemented. |
| Compare verifier-filled `lsp_tables_expected_template.csv` and decide table/coefficient mismatch. | `G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v` reports exact `1680/1680 100.00%`, mismatches `0`, blanks `0`. | Complete: no table/coefficient mismatch found. |
| If table mismatch exists, measure impact before production table change. | Table strict compare found zero mismatches, so no production table-impact diagnostic is needed. | Not applicable. |
| If table mismatch does not exist, design LSP.BIT numeric oracle for MA predictor residual trajectory. | `internal/lsp/phase2a_int1_predictor_handoff_test.go` writes and compares `lsp_predictor_residual_*` files keyed by `frame,col`. | Implemented. |
| Generate LSP predictor residual handoff files. | `testdata/oracle/handoff/lsp_predictor_residual_got.csv` and `testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv` exist. | Implemented. |
| Compare verifier-filled predictor residual template frame by frame. | `G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v` reports exact `22320/22320 100.00%`, mismatches `0`, blanks `0`. | Complete for local emitted-index residual arithmetic. |
| Compare local emitted residual trajectory against `LSP.BIT` reference-index residual trajectory. | `go test ./internal/lsp -run TestOracleHandoff_LSPReferenceResidualTrajectoryDiagnostic -v` reports exact `2519/22320 11.29%`, frame-all10 `77/2232 3.45%`; first mismatch is frame 0 local `(0,120,2,11)` vs ref `(0,120,10,10)`. | Complete diagnostic: residual arithmetic is exonerated; VQ index selection/LP-analysis target remains divergent. |
| Remeasure LSP all4, L1/L2/L3, PITCH/FCB downstream diagnostics. | `go test -tags conformance -run 'TestEncode_LSPVectorBitExact|TestINT1D10CorpusDiagnostic' -v` reports LSP full rates `L0=78.67% L1=38.93% L2=17.07% L3=19.35%`; first divergence frame 0 got `(0,120,2,11)` want `(0,120,10,10)`. `go test -tags conformance -run 'TestPhase2cINT1_ClosedLoopPitchByteEQ|TestPhase2dINT1a_FCBByteEQ|TestPhase2fINT1_PerVectorByteEQ' -v` reports PITCH `P1=12.04% P0=59.89% P2=11.83%`; FCB max field `GA1=12.10%`, `C1/C2=0.00%`; per-vector full-frame remains `0.00%`. | Complete measurement; conformance gates still fail/defer. |
| Limit production changes to numeric-oracle-identified causes. | Verifier data exonerates LSP tables and emitted-index residual arithmetic, so no production table or predictor change was made. | Satisfied. |
| Pass `go test ./...`. | `env GOCACHE=/tmp/go-build go test ./...` passed; root package `15.428s`, `internal/lsp 0.600s`. | Complete. |
| Pass `go build ./...`. | `env GOCACHE=/tmp/go-build go build ./...` passed. | Complete. |
| Pass `go test -race ./...`. | `env GOCACHE=/tmp/go-build go test -race ./...` passed; root package `124.904s`, `internal/lsp 4.350s`. | Complete. |
| Document clear verdict for LSP predictor/table mismatch. | This audit records strict table `100.00%`, strict emitted-index residual `100.00%`, and reference-index residual trajectory `11.29%` exact / `3.45%` frame-all10. | Complete: table and emitted-index residual arithmetic exonerated; remaining mismatch is VQ decision surface/input. |

## Current Verdict

The verifier-filled LSP table handoff is complete and exact:

- LSP table/coefficient cells: `1680/1680 100.00%`
- LSP emitted-index predictor residual cells: `22320/22320 100.00%`

This rules out production LSP VQ table constants and emitted-index
residual construction/commit arithmetic as the current LSP.BIT mismatch
cause.

The remaining LSP divergence is in the VQ decision surface or its input
target. Comparing local emitted residuals against the `LSP.BIT`
reference-index residual trajectory gives only `2519/22320 11.29%`
exact cells and `77/2232 3.45%` all-10 frame exact. Frame 0 already
diverges at the second stage:

```text
local=(0,120,2,11) ref=(0,120,10,10)
```

The next productive target is therefore the LSP VQ search input:
LP-analysis/windowing, LSP-to-LSF target, weighting, and second-stage
selection cost surface. It is not a table handoff or residual FIFO
problem.

## Follow-up VQ Input Diagnostic

`TestINT1D20TargetInterpolationDiagnostic` measures whether the
`LSP.BIT` reference `(L2,L3)` pair becomes locally optimal when the
local LSF target is moved toward the reference-index reconstruction
`refHat`.

Result:

```text
alpha=  0% toward refHat: ref pair rank1 202/2232 9.05% top8 620/2232 27.78%
alpha= 10% toward refHat: ref pair rank1 284/2232 12.72% top8 739/2232 33.11%
alpha= 25% toward refHat: ref pair rank1 438/2232 19.62% top8 997/2232 44.67%
alpha= 50% toward refHat: ref pair rank1 963/2232 43.15% top8 1610/2232 72.13%
alpha= 75% toward refHat: ref pair rank1 1907/2232 85.44% top8 2112/2232 94.62%
alpha= 90% toward refHat: ref pair rank1 2205/2232 98.79% top8 2219/2232 99.42%
alpha=100% toward refHat: ref pair rank1 2232/2232 100.00% top8 2232/2232 100.00%
```

Frame 0 remains the first mismatch:

```text
got=(0,120,2,11) ref=(0,120,10,10) rank@0=8 rank@100=1
omega=[2333 4674 7014 9354 11695 14033 16374 18713 21054 23394]
refHat=[2415 4765 6875 9512 11713 14089 16412 18483 20849 23487]
```

This narrows the remaining issue further: the reference tuple is
compatible with the local codebooks and cost surface once the target is
close enough to `refHat`, but the current LP-analysis/LSP-to-LSF target
is too far from that region on most frames. The next diagnostic should
therefore focus on the LPC analysis buffer/window/cold-start path that
feeds `omega`, especially why early frames repeatedly use the same
nearly uniform target.

`TestINT1D21ColdStartAnalysisTrace` then traces frames 0..12 through
the local analysis path. The first five frames have zero PCM energy,
zero preprocessed energy, zero analysis-buffer energy, and the same
identity LPC analysis result:

```text
frame=0 pcmEnergy=0 processedEnergy=0 oldSpeechEnergy=0 reusedLSP=false
aQ12=[4096 0 0 0 0 0 0 0 0 0 0]
omega=[2333 4674 7014 9354 11695 14033 16374 18713 21054 23394]
got=(0,120,2,11) ref=(0,120,10,10)
```

Frames 1..4 repeat the same `aQ12`, `qQ15`, and `omega` while the
`LSP.BIT` reference emits different second-stage choices. Non-zero
input appears at frame 5, and the local analysis target changes at
that point.

This further narrows the next target to ITU-vector analysis alignment:
initial lookahead, LPC analysis delay, and exact cold-start framing for
`LSP.IN`. It is not an `LPToLSP` fallback issue; `reusedLSP=false` on
these traced frames.

`TestINT1D8GroundTruth` also rules out a simple global frame-offset
explanation. Offset `0` is the best alignment:

```text
offset=-1 all4=0.67%
offset=+0 all4=3.67%
offset=+1 all4=0.45%
```

So the remaining mismatch is not a constant one-frame shift in
`LSP.BIT`; it is an analysis-window/cold-start framing difference that
changes the LSP target itself, especially in the first silent frames.

`TestINT1D22AnalysisWindowOffsetDiagnostic` compares multiple 240-sample
analysis-buffer offsets over the same preprocessed `LSP.IN` stream. The
current production offset `-160` is still the best of the tested
variants:

```text
offset=-240 all4=0.45%
offset=-200 all4=1.39%
offset=-160 all4=3.67%
offset=-120 all4=1.25%
offset= -80 all4=0.67%
offset= -40 all4=0.72%
offset=  +0 all4=0.22%
offset= +40 all4=0.54%
offset= +80 all4=0.36%
```

That rules out a simple 240-sample analysis-buffer shift among the
tested multiples of 40 samples.

`TestINT1D16PredictorTrajectoryDiagnostic` shows the other half of the
cold-start picture: if predictor memory is committed with the
`LSP.BIT` reference residuals, frames 1..4 match the reference
second-stage choices, but frame 0 remains divergent:

```text
frame=0 prodMem got=(0,120,2,11) ref=(0,120,10,10) refMem got=(0,120,2,11)
frame=1 prodMem got=(0,120,14,20) ref=(0,120,7,9)  refMem got=(0,120,7,9)
frame=2 prodMem got=(0,120,7,11)  ref=(0,120,31,10) refMem got=(0,120,31,10)
frame=3 prodMem got=(0,120,8,13)  ref=(0,120,5,0)  refMem got=(0,120,5,0)
frame=4 prodMem got=(0,120,9,26)  ref=(0,120,2,27) refMem got=(0,120,2,27)
```

So the earliest actionable root is the frame-0 cold-start decision.
Once frame 0 commits the reference residual, the silent-frame MA
trajectory immediately explains frames 1..4. The next clean-room probe
should focus on the frame-0 initial MA predictor seed / target relation,
not broad table, residual, or window-shift changes.

`TestINT1D23Frame0InitialMemorySweepDiagnostic` isolates that frame-0
seed relation without changing production code. With the current
production cold-start memory, the reference tuple is close but not the
local winner:

```text
production-initPastResidual:
got=(0,120,2,11) ref=(0,120,10,10)
gotCost=102190622 refCost=116891995 refFullRank=9
L1 target rank=1 for ref L1=120
ref-prefix pair best=(2,11) ref=(10,10) rank=8
```

Zero memory and repeated reference residual memory are both much worse,
so the mismatch is not explained by those simple seed conventions:

```text
zero-memory: got=(1,80,12,17) refFullRank=140870
reference-residual-repeated: got=(1,120,7,9) refFullRank=10791
```

The frame-0 implied uniform memory, derived numerically from the
reference tuple and local frame-0 target, makes the reference tuple the
exact local winner:

```text
impliedUniformMemory=[2232 4556 7204 9148 11674 13961 16326 19031 21339 23269]
frame0-implied-uniform-memory:
got=(0,120,10,10) ref=(0,120,10,10)
gotCost=2808 refCost=2808 refFullRank=1
ref-prefix pair best=(10,10) ref=(10,10) rank=1
omega-refHat=[1 0 1 0 0 1 0 0 1 0]
```

This does not justify changing the production seed yet, because the
implied memory is a diagnostic construction. It does, however, make the
next verifier handoff very small: ask for numeric frame-0 encoder-side
MA predictor seed values, target residual values for selector 0, and
the selected L1/L2/L3 costs/ranks. That should distinguish whether the
true mismatch is initial MA memory, target residual arithmetic, or a
tie/rounding rule on the frame-0 VQ surface.

The verifier then filled `lsp_frame0_vq_expected_template.csv` completely.
The strict local compare passed:

```text
LSP frame-0 VQ handoff compare: exact 76/76 100.00% mismatches=0 blanks=0
```

Post-fill SHA-256:

```text
lsp_frame0_vq_expected_template.csv b53c591848ea490384d7a1acf4dc111518651b313f19a9419601510c03b4fbfb
```

This exonerates the local frame-0 seed, target, selected-index,
cost, and rank arithmetic as represented in the handoff. It also
creates a sharper contradiction to resolve before any production
change: within the same frame-0 numeric handoff, the selected-index
rows are `(0,120,2,11)` while the transmitted `LSP.BIT` reference-index
rows are `(0,120,10,10)`. `READMETV.txt` states that `lsp.bit` was
obtained by running `coder lsp.in lsp.bit`, so the next handoff must ask
the verifier to resolve that source-of-truth distinction explicitly:
whether `selected_index` means the encoder's actual emitted tuple for
`LSP.IN` frame 0, or merely a recomputation on the handoff's local
numeric target/cost surface.

That sharper source distinction handoff is now:

```text
lsp_frame0_source_expected_template.csv
field,frame,col,expected
```

It has only eight rows:

```text
bitstream_index,0,0..3
encoder_selected_index,0,0..3
```

The local `got` values intentionally preserve the contradiction:

```text
bitstream_index        = (0,120,10,10)
encoder_selected_index = (0,120,2,11)
```

The verifier prompt now explicitly says not to copy `got`; it asks for
`bitstream_index` from decoded `LSP.BIT` frame 0 and
`encoder_selected_index` from the actual encoder emission for `coder
LSP.IN LSP.BIT` frame 0. If the verifier returns
`encoder_selected_index=(0,120,10,10)`, the remaining root is local
front-end/VQ decision mismatch. If it returns `(0,120,2,11)`, then the
current external verification method is distinguishing a recomputed
surface from the transmitted ITU bitstream, and the source oracle must
be corrected before any production patch.

The verifier filled the source distinction template completely, and the
strict compare passed:

```text
LSP frame-0 source handoff compare: exact 8/8 100.00% mismatches=0 blanks=0
```

Post-fill SHA-256:

```text
lsp_frame0_source_expected_template.csv a3689dd1b66af972673943d7fb477adb1ce7d75897d22274fb8289a41b0c7539
```

Verifier-confirmed frame-0 source split:

```text
bitstream_index        = (0,120,10,10)
encoder_selected_index = (0,120,2,11)
```

This closes the LSP detour for production purposes: the local frame-0
VQ decision agrees with the verifier's encoder-selected tuple, while the
transmitted `LSP.BIT` tuple is a different source. Therefore the
`LSP.IN` -> `LSP.BIT` byte-EQ gate must remain an informational /
source-divergence diagnostic, not a production patch driver. The next
production-relevant target returns to the closed-loop pitch chain:
separating `closedloopStep` target, excitation, and search inputs to
explain why P1/P2 byte-EQ remains near 12%.

## Strict Commands

Refresh LSP table handoff:

```sh
G729_WRITE_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_WriteLSPTableHandoff -v
```

Strict LSP table verdict:

```sh
G729_COMPARE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_TABLE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_TABLE_HANDOFF=1 go test ./internal/tables -run TestOracleHandoff_CompareLSPTableHandoff -v
```

Refresh LSP predictor residual handoff:

```sh
G729_WRITE_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPPredictorResidualHandoff -v
```

Strict LSP predictor residual verdict:

```sh
G729_COMPARE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_PREDICTOR_HANDOFF=1 G729_REQUIRE_EXACT_LSP_PREDICTOR_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPPredictorResidualHandoff -v
```

Refresh frame-0 LSP VQ handoff:

```sh
G729_WRITE_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0VQHandoff -v
```

Strict frame-0 LSP VQ verdict:

```sh
G729_COMPARE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_VQ_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_VQ_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0VQHandoff -v
```

Refresh frame-0 source distinction handoff:

```sh
G729_WRITE_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_WriteLSPFrame0SourceHandoff -v
```

Strict frame-0 source distinction verdict:

```sh
G729_COMPARE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_FRAME0_SOURCE_HANDOFF=1 G729_REQUIRE_EXACT_LSP_FRAME0_SOURCE_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPFrame0SourceHandoff -v
```

Final gates after any verifier-supported production change:

```sh
go test ./...
go build ./...
go test -race ./...
```
