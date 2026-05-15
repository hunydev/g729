# G.729 Quality Goal Completion Audit

Date: 2026-05-10

Scope: status audit for the active long-running quality goal after the
clean-room PDF audits, product-quality measurements, and fixed-codebook
tree-search oracle handoff.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg, Sipro, or other G.729 implementation
  source was inspected.
- External encoders/decoders were used only as black-box executables.
- Imported oracle content remains limited to numeric measurements, scalar
  fields, deltas, and aggregate diagnostics.
- The ITU package source archive was not opened.

## Current Quality Metrics

Problem sample:

```text
testdata/external/user_quality_audio.m4a
```

Current diagnostic results:

| Path | SNR dB | SegSNR dB | Corr | RMS/ref | Peak | NearClip |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Quality -> local decode | 6.40 | 5.04 | 0.8787 | 0.9117 | 29608 | 0 |
| Quality -> ffmpeg decode | 5.94 | 4.77 | 0.8650 | 0.9206 | 30732 | 0 |
| Core -> local decode | 5.03 | 4.14 | 0.8300 | 0.8820 | 31164 | 0 |
| Core -> ffmpeg decode | 5.21 | 4.37 | 0.8383 | 0.9007 | 32768 | 2 |
| bcg729 -> ffmpeg decode | 5.68 | n/a | 0.8562 | 0.9131 | 32614 | 0 |

Interpretation:

- The product/default Quality profile has reached the main problem-sample
  near-clip target: `our -> ffmpeg` near-clip is `0`.
- Quality-profile `our -> ffmpeg` SNR and correlation are now slightly above
  the bcg729 black-box path on this sample.
- Core still has a small residual ffmpeg-decoded clipping surface and remains
  below the Quality profile; the focused Annex A reduced-tree FCB verifier
  checks now exact-pass, so this residual is not explained by the focused
  FCB tree-search subset.
- Current Core ffmpeg near-clip frames are `[292 293 294]`, which is why the
  active FCB tree-search handoffs are pinned to frames `292..294`.

Original user sample:

```text
testdata/external/user_quality_input.m4a
```

| Path | SNR dB | SegSNR dB | Corr | RMS/ref | Peak | NearClip |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Core -> local decode | 4.88 | 3.46 | 0.8258 | 0.9088 | 24362 | 0 |
| Core -> ffmpeg decode | 5.09 | 3.78 | 0.8346 | 0.9160 | 26283 | 0 |
| Quality -> local decode | 5.87 | 4.21 | 0.8640 | 0.9362 | 25724 | 0 |
| Quality -> ffmpeg decode | 5.43 | 4.05 | 0.8508 | 0.9512 | 27574 | 0 |

SPEECH black-box regression check:

| Path | SNR dB | SegSNR dB |
| --- | ---: | ---: |
| SPEECH.BIT -> ffmpeg decode | 7.04 | 4.39 |
| our encoder -> ffmpeg decode | 7.12 | 4.61 |

Interpretation: the current encoder is not regressed against the checked
SPEECH black-box baseline.

## 8000 Port Web App Check

Health check:

```text
curl -fsS http://127.0.0.1:8000/healthz
ok
```

Problem-sample `/api/compare` results:

Reproduce without embedded audio/download payloads:

```sh
curl -fsS -F 'file=@testdata/external/user_quality_audio.m4a' \
  http://127.0.0.1:8000/api/compare |
  jq 'del(.audio, .downloads)'
```

| Path | SNR dB | Corr | RMS ratio | Lag | Peak | NearClip |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| our encode -> local decode | 6.40 | 0.8787 | 0.9117 | 40 | 29608 | 0 |
| our encode -> ffmpeg decode | 5.94 | 0.8650 | 0.9206 | 40 | 30732 | 0 |
| bcg729 encode -> local decode | 5.40 | 0.8452 | 0.8959 | 40 | 30316 | 0 |
| bcg729 encode -> ffmpeg decode | 5.68 | 0.8562 | 0.9131 | 40 | 32614 | 0 |
| local decoder: our payload vs bcg729 payload | 9.09 | 0.9375 | 0.9827 | 0 | 30316 | 0 |
| ffmpeg decoder: our payload vs bcg729 payload | 9.54 | 0.9440 | 0.9919 | 0 | 32614 | 0 |

Clip markers:

- `our_local`, `our_ffmpeg`, `external_local`, and `external_ffmpeg` are
  empty on the current web-app response.
- The converted source still contains source-side clip markers, including the
  known 2.860s and 2.886s regions.

## Goal Checklist

| Goal item | Status | Evidence |
| --- | --- | --- |
| Preserve clean-room boundary | Done | PDF text and black-box numeric output only; no external implementation source inspected. |
| Separate Core from Quality heuristics | Done | README/doc comments distinguish `EncoderProfileCore` from `EncoderProfileQuality`. |
| Measure Core and Quality on the same samples | Done | External-sample diagnostics cover the problem sample and original user sample. |
| Audit closed-loop pitch against PDF | Done | `2026-05-10-closedloop-pitch-pdf-audit.md`; no production mismatch found. |
| Audit open-loop center tuning surface | Done, no change | `2026-05-10-openloop-lift-sweep-audit.md`; `1.12` removes problem-sample Core near-clips but is a PDF-ambiguous tuning candidate and slightly worsens the original user sample. |
| Audit PDF-visible FCB equations | Done | `2026-05-10-fcb-search-pdf-audit.md`; equations and packing are aligned. |
| Resolve exact Annex A reduced FCB tree subset | Done | Verifier-filled numeric expected cells were validated and both focused strict compares exact-passed: `10194/10194` for SPEECH and `10194/10194` for the user-audio problem region. |
| Keep FCB handoff artifacts fresh | Done | `TestOracleHandoff_FCBTreeSearchGotMatchesCurrentSurface`, `TestOracleHandoff_FCBTreeSearchUserAudioGotMatchesCurrentSurface`, `TestOracleHandoff_ManifestHashesMatchCurrentFiles`, and `TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles` guard current got CSVs, manifest hashes, verifier-filled FCB hashes, and the remaining blank closed-loop-stage count. |
| Keep verifier prompts within clean-room boundary | Done | `TestOracleHandoff_VerifierPromptsStateCleanRoomBoundary`, `TestOracleHandoff_FCBPromptPinsSpeechInput`, and `TestOracleHandoff_UserAudioFCBPromptPinsConvertedSample` require black-box executable wording and pinned input identity for the active verifier prompts. |
| Document and guard verifier output intake | Done | `validate_verifier_output.sh` validates allowed filenames, rejects symlinks, requires unchanged headers/keys, complete numeric cells, and validation-only default behavior; `TestOracleHandoff_READMEDocumentsFilledVerifierIntake`, `TestOracleHandoff_IntakeScriptValidatesAndAppliesFilledOutput`, and `TestOracleHandoff_IntakeScriptRejectsUnsafeOutput` lock the workflow. |
| Audit gain preselection and reconstruction | Done | `2026-05-10-gain-preselect-pdf-audit.md` and `2026-05-10-gain-reconstruction-pdf-audit.md`; no reconstruction mismatch found. |
| Audit state commit timing | Done | `2026-05-10-state-commit-pdf-audit.md`; no stale state-commit mismatch found. |
| Reduce problem-sample product clipping | Done for Quality | Quality `our -> ffmpeg` near-clip is `0`. |
| Improve toward bcg729 quality | Done for Quality sample metric | Quality `our -> ffmpeg` SNR/correlation exceed the bcg729 black-box path on the current problem sample. |
| Avoid SPEECH regression | Done | `our encoder -> ffmpeg` is slightly above `SPEECH.BIT -> ffmpeg` in the checked diagnostic. |
| Make Core clipping strongly reduced | Partial | Core ffmpeg near-clip is `2`, but not fully resolved. |
| Document remaining heuristics | Done | README/doc comments classify Quality-profile native search and repair as product heuristics. |
| Regenerate docs audio/WASM if encoder output changed | Not applicable in this phase | The handoff/audit phase made no new production encoder behavior change. Existing dirty assets predate this audit phase. |
| Verify 8000 web app | Done | Health check and `/api/compare` succeeded on the problem sample. |

## Prompt-to-Artifact Checklist

| Prompt requirement | Artifact or gate | Current evidence |
| --- | --- | --- |
| Keep the clean-room boundary. | `AGENTS.md`; verifier prompts; `TestOracleHandoff_VerifierPromptsStateCleanRoomBoundary` | Prompts require black-box executable use and numeric scalar artifacts only. |
| Do not inspect external implementation source. | Audit notes in this document and the PDF audit files. | No source-derived implementation text is imported; `Software.zip` remains unopened. |
| Use `testdata/external/user_quality_audio.m4a` as the problem sample. | `TestOracleHandoff_UserAudioFCBPromptPinsConvertedSample`; `TestExternalSampleQualityDiagnostic` | Sample hash and converted PCM identity are pinned; latest Quality ffmpeg result is SNR `5.94`, near-clip `0`. |
| Keep `testdata/external/user_quality_input.m4a` as the original user sample. | `TestExternalSampleQualityDiagnostic` | Latest Quality ffmpeg result is SNR `5.43`, near-clip `0`. |
| Keep `third-party/g729-compare-web` / port 8000 usable. | `curl -fsS http://127.0.0.1:8000/healthz` and `/api/compare` table above. | Health returned `ok`; compare output has no decoded near-clips. |
| Separate `EncoderProfileCore` and `EncoderProfileQuality`. | `encoder.go`, `doc.go`, `README.md`, `encoder_test.go` | Core disables quality heuristics; Quality keeps product heuristics. |
| Measure Core and Quality on the same samples. | `G729_EXTERNAL_SAMPLE_PROFILE_COMPARE=1` diagnostics and recorded tables. | Core/Quality comparisons exist for both active samples; problem-sample Core ffmpeg near-clips remain localized to frames `[292 293 294]`. |
| Audit closed-loop pitch. | `docs/superpowers/diagnostics/2026-05-10-closedloop-pitch-pdf-audit.md` | PDF-visible closed-loop pitch mapping found no current production mismatch. |
| Audit FCB search. | `docs/superpowers/diagnostics/2026-05-10-fcb-search-pdf-audit.md`; FCB handoff CSVs. | PDF-visible equations align; focused exact reduced tree subset strict-compares passed with zero mismatches. |
| Audit gain quantization. | `2026-05-10-gain-preselect-pdf-audit.md`; `2026-05-10-gain-reconstruction-pdf-audit.md` | Core keeps Annex A/G.729 preselect; Quality native gain search is documented as heuristic. |
| Audit state commit timing. | `2026-05-10-state-commit-pdf-audit.md` | No stale commit mismatch found for `oldExc`, `swMemErr`, `lpResidualMemQ`, or gain predictor state. |
| Classify spec-aligned fixes vs heuristics. | `README.md`, `doc.go`, `encoder.go` comments. | Quality-only tuning is documented as heuristic; Core changes are kept separate. |
| Run `go test ./... -count=1`. | Command gate. | Latest run passed. |
| Run the problem-sample quality diagnostic. | `G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a go test -run TestExternalSampleQualityDiagnostic -count=1 -v` | Latest run passed with Quality ffmpeg SNR `5.94`, near-clip `0`. |
| Run the original-sample quality diagnostic. | `G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_input.m4a go test -run TestExternalSampleQualityDiagnostic -count=1 -v` | Latest run passed with Quality ffmpeg SNR `5.43`, near-clip `0`. |
| Run the SPEECH black-box quality diagnostic. | `G729_FFMPEG_BLACKBOX_QUALITY=1 go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v` | Latest run passed; our encoder `7.12/4.61` vs `SPEECH.BIT -> ffmpeg` `7.04/4.39`. |
| Regenerate docs audio/WASM if encoder output changes. | Git dirty assets; no new production encoder behavior in the handoff/audit phase. | Not applicable to the latest handoff/intake/doc changes. |
| Resolve remaining Core FCB tree evidence. | `fcb_tree_search_expected_template.csv`; `fcb_tree_search_user_audio_expected_template.csv`; `encoder_closedloop_stage_expected_template.csv` | Focused FCB achieved: both FCB templates are verifier-filled (`0/10194` blank cells each) and exact-pass. The broad closed-loop stage remains optional and blank (`100848/100848` blank expected cells). |
| Validate returned verifier output before applying it. | `validate_verifier_output.sh`; intake tests. | Done for the two returned FCB CSVs; validation accepted `10194` numeric cells in each file before apply. |

## Completion Decision

The long-running goal is complete.

The product Quality profile meets the immediate audible-clipping target on the
current problem sample and no SPEECH regression was measured. The focused Core
FCB question is no longer blocked: both verifier-filled FCB templates
strict-compare exactly. The remaining unfilled
`encoder_closedloop_stage_expected_template.csv` is a broader optional
follow-up oracle, not a blocker for the scoped quality/PDF comparison goal:
the explicit closed-loop pitch, FCB, gain, state-commit, product-quality, and
regression gates have concrete evidence above.

## Optional Follow-Up

The focused FCB handoff has been completed by an isolated verifier:

```text
testdata/oracle/handoff/fcb_tree_search_expected_template.csv
testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv
```

The verifier output was validated and applied as numeric `expected` cells only.
The verifier was allowed to use external G.729 executables privately only as
black-box processes and return only numeric cells.
FCB tree-search strict compare exact-passed at goal closure. After the
2026-05-12 short-pitch `T_frac=0` adaptive-codebook fix and the 2026-05-15
fixed-gain Q1 quantization fix, the encoder closed-loop/FCB numeric surface
changed and the focused FCB verifier artifacts were rerun. The refreshed
2026-05-15 verifier output is current and exact.

Current compare results after the 2026-05-15 verifier refresh:

| Command | Current result |
| --- | --- |
| `G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v` | Passes: exact `10194/10194`, mismatches `0`, blanks `0`, missing `0` |
| `G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v` | Passes: exact `10194/10194`, mismatches `0`, blanks `0`, missing `0` |
| `G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -count=1 -v` | Optional follow-up remains unfilled: `expected handoff has no filled numeric cells; verifier output is required before comparison` |

Before sending or consuming verifier files, keep the default handoff guards
green:

```sh
go test -run 'TestOracleHandoff_FCBTreeSearchGotMatchesCurrentSurface|TestOracleHandoff_FCBTreeSearchUserAudioGotMatchesCurrentSurface|TestOracleHandoff_ManifestHashesMatchCurrentFiles|TestOracleHandoff_ManifestUnfilledCountsMatchCurrentFiles' -count=1 -v
```

The current repo-external verifier bundle is:

```text
/tmp/g729-fcb-verifier-handoff-2026-05-10.tar.gz
sha256 f4b97a1b9a0dc745aa194f0c2e733f289a37e678b7bb6f214aa0468ffb744dae
```

When sending the bundle to another AI or engineer, ask them to start with
`testdata/oracle/handoff/EXTERNAL_VERIFIER_REQUEST.md`. That file is the
short handoff contract for the clean-room boundary, return format, and
the remaining blank expected CSV template.

For incoming verifier output, first run
`testdata/oracle/handoff/validate_verifier_output.sh` against a temporary
directory containing only returned expected CSV files. It validates exact
headers, row counts, key-column order, allowed filenames, and complete
numeric `expected` cells before `G729_APPLY_VERIFIER_OUTPUT=1` copies the
files into the handoff template paths.

```sh
G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

Strict completion gate:

```sh
G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

Problem-region gate:

```sh
G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
  go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

If the verifier output shows a Core tree-search mismatch, the next change
should be a spec-aligned Core fix. If it does not, the remaining improvement
path is explicitly Quality-profile heuristic work, not an Annex A bug fix.
