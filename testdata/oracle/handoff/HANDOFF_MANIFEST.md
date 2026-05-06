# Oracle Handoff Manifest

Date: 2026-05-06

This manifest identifies the clean-room verifier handoff inputs and
the current verifier-filled outputs. The pre-fill hashes identify the
blank templates originally sent to the verifier. The post-fill hashes
identify the current files after numeric `expected` cells were filled.

## Files

| File | Header | Data rows | SHA-256 |
| --- | --- | ---: | --- |
| `LSP_VERIFIER_PROMPT.md` | n/a | n/a | `d776ba0d5ed27d7261ac57dc741cf34cae64a50bf5504b9a661720cb28fdda35` |
| `lsp_tables_expected_template.csv` | `table,selector,tap,row,col,expected` | 1680 | `2979b1a9d5e599d88bc87d36984ab68653ae6dd551f7bb776ce8ef7bf6c6ed56` |
| `lsp_tables_got.csv` | `table,selector,tap,row,col,got` | 1680 | `1d9e104ef7edcecd2f51ea8695311b35e508b8f8b9078f94ac2e7b8f89603288` |
| `lsp_predictor_residual_expected_template.csv` | `frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,expected` | 22320 | `86126634d2022621b071ee5b2e8bee8ee9c60f9a2a4105d9d130a4041eb0af99` |
| `lsp_predictor_residual_got.csv` | `frame,selector,L1,L2,L3,ref_selector,ref_L1,ref_L2,ref_L3,col,got` | 22320 | `5c1bc1682c5db95e1ede9f96291819926083b615ff559b469c7075a9677e9fdd` |
| `lsp_frame0_vq_expected_template.csv` | `field,frame,selector,tap,L1,L2,L3,col,expected` | 76 | `091e87fd55dc57cc4b8fc0cf879600633e37033d0af28860596873975a993e12` |
| `lsp_frame0_vq_got.csv` | `field,frame,selector,tap,L1,L2,L3,col,got` | 76 | `dc24d27990baf779c0fd540dceb586bc2a74b0d3ac15b2219e4a393f35daa77d` |
| `lsp_frame0_source_expected_template.csv` | `field,frame,col,expected` | 8 | `ec413da828caf4b6dd3c64b56fc4e9eb3cfde7eb0e620c8a93cc13dd6d19f6a8` |
| `lsp_frame0_source_got.csv` | `field,frame,col,got` | 8 | `9fa5c22aa8fd1788902312b0663f04564bb17c68bf0e674280b27ad6ff9b006d` |
| `LSP_DECISION_VERIFIER_PROMPT.md` | n/a | n/a | `df89b6d9b8ea76be53e9f1cbd94abe3d3ea1262c389a47f4cb85868445a08724` |
| `lsp_decision_expected_template.csv` | `field,frame,tap,L0,L1,L2,L3,col,expected` | 1472 | `38a4ffad4894acf412d01048d14beb1c5b9e7ea8f033629cec1569e318489c13` |
| `lsp_decision_got.csv` | `field,frame,tap,L0,L1,L2,L3,col,got` | 1472 | `24f43d6b517b69dc58ca420cfddfcd9f6669c27909cb6a73d6353dd3e9c9a976` |
| `PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md` | n/a | n/a | `0d5de5773dc4b23b31079e2b254e08a02eaea056174a460158fd05129ac1dd01` |
| `pitch_closedloop_search_expected_template.csv` | `field,frame,sub,index,lag,frac,expected` | 3192 | `5a4b28ab4c51728c107e5cc278362a67e9f7f5a0b27d9a3bef8d2367bb31d202` |
| `pitch_closedloop_search_got.csv` | `field,frame,sub,index,lag,frac,got` | 3192 | `086a22e02010f9f9eb1292d89f4b58a06d6969061d7119385dd90c630eb314ea` |
| `TAME_GAIN_TAMING_VERIFIER_PROMPT.md` | n/a | n/a | `7a31b94f0edc9249f1de0e6522e10f136ef42c060262fbdee8d9e6f5e588d3e0` |
| `tame_gain_taming_expected_template.csv` | `field,frame,sub,index,expected` | 8962 | `4b31d19b4fb910e579ccfd78babe80337798c8a6c9a3b90d29c54bd06b18fc2a` |
| `tame_gain_taming_got.csv` | `field,frame,sub,index,got` | 8962 | `53927db37826977a1f0e6a2421f4ccb7a0a569998bae8461c479a6260c0af298` |
| `REMAINING_CONFORMANCE_VERIFIER_PROMPT.md` | n/a | n/a | `e16f930424d36dd33991954531cc7ab6affb941a773151b4f6e3910e8d00d84a` |

## Verifier-filled Files

| File | Filled `expected` cells | Post-fill SHA-256 |
| --- | ---: | --- |
| `lsp_tables_expected_template.csv` | 1680 | `ad51b42a05ba6cd30738984c00f072267cd359b09cee1e751b576ca9215fd060` |
| `lsp_predictor_residual_expected_template.csv` | 22320 | `8c74fd93df50e52b71b8485ef5e2c7a1c856bac72aef9d0087e6ca3b8dab0dee` |
| `lsp_frame0_vq_expected_template.csv` | 76 | `b53c591848ea490384d7a1acf4dc111518651b313f19a9419601510c03b4fbfb` |
| `lsp_frame0_source_expected_template.csv` | 8 | `a3689dd1b66af972673943d7fb477adb1ce7d75897d22274fb8289a41b0c7539` |
| `pitch_closedloop_search_expected_template.csv` | 3192 | `e855712427f3da636f4db1c00045782e8b8aec0b7191b0db342c7e57ccba6b8a` |
| `lsp_decision_expected_template.csv` | 1472 | `7e3b481a2263fe8ed0811ad38b7fdfd0d1e0c69c01e6fdca6a3f623cda7b7f20` |
| `tame_gain_taming_expected_template.csv` | 8962 | `ab66f67c7f498cf41858f15c45faafaf949e36dcd1319054e69ef14580b85dd4` |

## Verification Commands

Recompute hashes:

```sh
sha256sum \
  testdata/oracle/handoff/LSP_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/lsp_tables_expected_template.csv \
  testdata/oracle/handoff/lsp_tables_got.csv \
  testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv \
  testdata/oracle/handoff/lsp_predictor_residual_got.csv \
  testdata/oracle/handoff/lsp_frame0_vq_expected_template.csv \
  testdata/oracle/handoff/lsp_frame0_vq_got.csv \
  testdata/oracle/handoff/lsp_frame0_source_expected_template.csv \
  testdata/oracle/handoff/lsp_frame0_source_got.csv \
  testdata/oracle/handoff/LSP_DECISION_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/lsp_decision_expected_template.csv \
  testdata/oracle/handoff/lsp_decision_got.csv \
  testdata/oracle/handoff/PITCH_CLOSEDLOOP_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/pitch_closedloop_search_expected_template.csv \
  testdata/oracle/handoff/pitch_closedloop_search_got.csv \
  testdata/oracle/handoff/TAME_GAIN_TAMING_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/tame_gain_taming_expected_template.csv \
  testdata/oracle/handoff/tame_gain_taming_got.csv \
  testdata/oracle/handoff/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md
```

Check data row counts and headers:

```sh
awk -F, 'FNR==1 {print FILENAME ": header=" $0} FNR>1 {rows++} ENDFILE {print FILENAME ": rows=" rows; rows=0}' \
  testdata/oracle/handoff/lsp_tables_expected_template.csv \
  testdata/oracle/handoff/lsp_tables_got.csv \
  testdata/oracle/handoff/lsp_predictor_residual_expected_template.csv \
  testdata/oracle/handoff/lsp_predictor_residual_got.csv \
  testdata/oracle/handoff/lsp_frame0_vq_expected_template.csv \
  testdata/oracle/handoff/lsp_frame0_vq_got.csv \
  testdata/oracle/handoff/lsp_frame0_source_expected_template.csv \
  testdata/oracle/handoff/lsp_frame0_source_got.csv \
  testdata/oracle/handoff/lsp_decision_expected_template.csv \
  testdata/oracle/handoff/lsp_decision_got.csv \
  testdata/oracle/handoff/pitch_closedloop_search_expected_template.csv \
  testdata/oracle/handoff/pitch_closedloop_search_got.csv \
  testdata/oracle/handoff/tame_gain_taming_expected_template.csv \
  testdata/oracle/handoff/tame_gain_taming_got.csv
```

## Completion Rule

The handoff is not complete until the verifier returns filled templates
with:

- unchanged headers;
- unchanged row counts;
- unchanged row order and key columns;
- numeric `expected` cells for every required row;
- no added implementation-derived text or provenance notes.

After that, run the strict compare commands documented in `README.md`.
