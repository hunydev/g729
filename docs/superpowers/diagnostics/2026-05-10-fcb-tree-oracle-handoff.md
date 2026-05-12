# FCB Tree-Search Numeric Oracle Handoff

Date: 2026-05-10

Scope: clean-room handoff for the one fixed-codebook surface that the official
PDF text does not fully specify: the exact Annex A reduced-complexity
tree-search candidate subset.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg, Sipro, or other G.729 implementation
  source was inspected.
- This handoff exports only numeric search-surface scalars and local scalar
  results.
- A verifier may use external G.729 implementations only as black-box
  executables, never as source material, and may return only numeric
  `expected` cells in the template.

## Files

```text
testdata/oracle/handoff/FCB_TREE_SEARCH_VERIFIER_PROMPT.md
testdata/oracle/handoff/fcb_tree_search_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_got.csv
testdata/oracle/handoff/FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md
testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv
```

The handoff covers `SPEECH.IN` frames 292 through 294 and both subframes.
Each row is keyed by:

```csv
field,frame,sub,index
```

The generated row count is `10194`.

The user-audio handoff is pinned to the converted
`testdata/external/user_quality_audio.m4a` frames 292 through 294 and both
subframes. This is the reported 2.9 second problem region. It uses the same
CSV key schema and row count.

The pinned conversion metadata is:

```text
format: 8 kHz mono signed little-endian 16-bit PCM
samples: 118701
bytes: 237402
sha256: e8d783af34de25d8d7d16a84dfe92238c647e4079a07d8dffd4e715a804ca5fa
```

## Numeric Surface

The CSV rows include:

- `d_abs[0..39]`
- `sign[0..39]`
- sign-folded `phi[0..1599]`
- focused selected positions and score terms
- exhaustive `C^2/E` positions and score terms
- first-three-pulse threshold statistics and accepted-prefix count

The verifier prompt asks for:

- both the `got` CSV and the expected template to be provided to the
  verifier;
- input rows copied back numerically after import verification;
- `selected_*` rows filled with the verifier's exact reduced-complexity
  Annex A tree-search result on the provided numeric surface;
- `full_*` rows filled with exhaustive `C^2/E` results on the same numeric
  surface.

## Commands

Refresh the handoff:

```sh
G729_WRITE_FCB_TREE_SEARCH_HANDOFF=1 \
  go test -run TestOracleHandoff_WriteFCBTreeSearchHandoff -count=1 -v
```

Refresh the user-audio problem-region handoff:

```sh
G729_WRITE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
  go test -run TestOracleHandoff_WriteFCBTreeSearchUserAudioHandoff -count=1 -v
```

Compare the verifier-filled SPEECH handoff:

```sh
G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

Compare the verifier-filled user-audio problem-region handoff:

```sh
G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

Strict complete comparison:

```sh
G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

Strict complete user-audio comparison:

```sh
G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

Structural integrity is covered by:

```sh
go test -run 'TestOracleHandoff_LSPStructuralIntegrity|TestOracleHandoff_LSPManifestMatchesCurrentFiles|TestOracleHandoff_ManifestHashesMatchCurrentFiles|TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles|TestOracleHandoff_VerifierPromptsStateCleanRoomBoundary|TestOracleHandoff_FCBPromptPinsSpeechInput|TestOracleHandoff_UserAudioFCBPromptPinsConvertedSample|TestOracleHandoff_FCBTreeSearchGotMatchesCurrentSurface|TestOracleHandoff_FCBTreeSearchUserAudioGotMatchesCurrentSurface|TestOracleHandoff_IntakeScriptValidatesAndAppliesFilledOutput|TestOracleHandoff_IntakeScriptRejectsUnsafeOutput' -count=1 -v
```

The current repo-external verifier bundle is:

```text
/tmp/g729-fcb-verifier-handoff-2026-05-10.tar.gz
sha256 3cc33c03830abdbbcdf791536f762fb966bf8bc9aea2ab2ff85c1abe61645772
```

External verifiers should start with
`testdata/oracle/handoff/EXTERNAL_VERIFIER_REQUEST.md` inside the
bundle. The focused FCB templates are already verifier-filled; the only
remaining blank template in that request is the broad closed-loop stage
template.

Returned verifier CSVs should be staged in a temporary directory and
validated with `testdata/oracle/handoff/validate_verifier_output.sh`
before setting `G729_APPLY_VERIFIER_OUTPUT=1` to copy them into the
template paths. The validator accepts only the three active expected CSV
filenames and requires unchanged headers, row order, key columns, and
complete numeric `expected` cells.

## Finding

No production encoder behavior changed. The verifier-filled focused FCB
handoffs provide clean-room evidence for deciding whether the current Core
focused-search approximation differs from the exact Annex A
reduced-complexity tree subset.

`fcb_tree_search_expected_template.csv` strict-compares exactly:
`10194/10194`, mismatches `0`, blanks `0`, missing `0`.

`fcb_tree_search_user_audio_expected_template.csv` strict-compares exactly:
`10194/10194`, mismatches `0`, blanks `0`, missing `0`.
