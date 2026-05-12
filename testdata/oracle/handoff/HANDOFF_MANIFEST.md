# Oracle Handoff Manifest

Date: 2026-05-12

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
| `ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md` | n/a | n/a | `851facd1c72a8e8bd3cf9180a5c4a0f6ef877ad0ae023ee15896df43073dcc43` |
| `encoder_closedloop_stage_expected_template.csv` | `field,frame,sub,index,lag,frac,expected` | 100848 | `d597acf9a6040a0380dd009e8c5669905aba5ecc7222b4756c13372d4c8aace4` |
| `encoder_closedloop_stage_got.csv` | `field,frame,sub,index,lag,frac,got` | 100848 | `859bb985aad0cbd4bc18e7f65429d79d176a07e4ec16477a6f982eb407f28b3b` |
| `TAME_GAIN_TAMING_VERIFIER_PROMPT.md` | n/a | n/a | `7a31b94f0edc9249f1de0e6522e10f136ef42c060262fbdee8d9e6f5e588d3e0` |
| `tame_gain_taming_expected_template.csv` | `field,frame,sub,index,expected` | 8962 | `4b31d19b4fb910e579ccfd78babe80337798c8a6c9a3b90d29c54bd06b18fc2a` |
| `tame_gain_taming_got.csv` | `field,frame,sub,index,got` | 8962 | `53927db37826977a1f0e6a2421f4ccb7a0a569998bae8461c479a6260c0af298` |
| `FCB_TREE_SEARCH_VERIFIER_PROMPT.md` | n/a | n/a | `b327bac1381cea87d2e364876dbdc423eac89f018d1950ed906e0a41b2a351a0` |
| `fcb_tree_search_expected_template.csv` | `field,frame,sub,index,expected` | 10194 | `4644b4330f2401496de267fd2f33c33d6e245715e1460accecb1cd51a3164d40` |
| `fcb_tree_search_got.csv` | `field,frame,sub,index,got` | 10194 | `9e788a38f7df52c0ed3d02f02a484fb5425e6d19dace0b88f7d5e36321640b75` |
| `FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md` | n/a | n/a | `102cfaccf8bc984507294b2d4ddee7272cd20befdeb513765b0989fb8fa1ada5` |
| `fcb_tree_search_user_audio_expected_template.csv` | `field,frame,sub,index,expected` | 10194 | `4644b4330f2401496de267fd2f33c33d6e245715e1460accecb1cd51a3164d40` |
| `fcb_tree_search_user_audio_got.csv` | `field,frame,sub,index,got` | 10194 | `88412b695175e48192a6f1f50fedc9647d56417d3e9682623406b2d1ef904ac8` |
| `decoder_itu_stage_expected_template.csv` | `source,frame,sub,field,index,expected` | 10491 | `a06d13af94cb2d1b60c6d6f13caeb6567a559e0b6a5d6959d56a6225f1a522a7` |
| `decoder_itu_stage_got.csv` | `source,frame,sub,field,index,got` | 10491 | `8358a260b67f0551fa7a84280ca97e1106ae236bb44e5252c9fcc42ee4b4bdda` |
| `DECODER_ITU_FCB_POSITION_CLARIFICATION_PROMPT.md` | n/a | n/a | `adf86bb920b590d25103bb4b1982aba459e1380a9886e3526f8ee535d7cb6790` |
| `decoder_itu_fcb_position_clarification_expected_template.csv` | `C,i0,i1,i2,i3,jx,m0,m1,m2,m3,note` | 3 | `33b5766a587219fedc26a7d6e8b757eade3dd03f31f9394834bd4602968ca450` |
| `EXTERNAL_VERIFIER_REQUEST.md` | n/a | n/a | `d65189e31ed3189ada680e26efd2e934d71d8286023d0c27cef5499df66aa725` |
| `REMAINING_CONFORMANCE_VERIFIER_PROMPT.md` | n/a | n/a | `0da88d8dc36ccedc36906df62ce781c04056f2e5bd5597187e6ec26b3dd5eadc` |
| `create_verifier_bundle.sh` | n/a | n/a | `05460c982ec487ec894bcb31b1200be4ab0542fd335bbabd08f8a21cb0651faf` |
| `validate_verifier_output.sh` | n/a | n/a | `fcb983b16c6c4f33398a4b80ba3fa501e0de1e001e11ffc496f5a4fb45840c1c` |

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
| `fcb_tree_search_expected_template.csv` | 10194 | `9662f8e6de7eb5e4d0d99ce0fae2770e7ee43b42f61b7a25f813fc297ffd98fa` |
| `fcb_tree_search_user_audio_expected_template.csv` | 10194 | `8043447cd725636836fe496018e373171b8ad87258604b7297920f5e6d22b260` |
| `decoder_itu_fcb_position_clarification_expected.csv` | 30 | `e2a8ff28bbd291c251e9d8dad13e2d82da9985bf342246ed9c7ac9fcf145fca2` |
| `decoder_tame_stage_wide_expected.csv` | 1518 | `ca9809900a74be1345f844bcc00090d31b0bfe4c3ada6bd1d82702d5c578b35e` |

## Controlled Numeric Diagnostics

| File | Data rows | SHA-256 | Note |
| --- | ---: | --- | --- |
| `tame_short_pitch_relation.csv` | 48 | `9d73b5aea66fa153b2d26353e750d9beddae4cc1b8a2df33c35bc80fb111560c` | Verifier-produced relation table for TAME short-pitch `T_frac=0`; supports the phase-0 FIR adaptive-codebook fix. |

## Partially Filled Files

| File | Filled `expected` cells | Blank `expected` cells | SHA-256 | Next action |
| --- | ---: | ---: | --- | --- |
| `decoder_itu_stage_expected.csv` | 1562 | 8929 | `158a1c310206163f60022a00d0e3002a23ee525d3dfd9955cfd4e8b0b3be261b` | Use as a localization artifact only; fixed-codebook fourth-pulse clarification is resolved, so request additional LP/synthesis state rows before promoting to a strict gate. |

## Currently Unfilled Files

| File | Blank `expected` cells | Next action |
| --- | ---: | --- |
| `encoder_closedloop_stage_expected_template.csv` | 100848 | Fill from `ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md` if a broader closed-loop stage oracle is required. |

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
  testdata/oracle/handoff/FCB_TREE_SEARCH_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/fcb_tree_search_expected_template.csv \
  testdata/oracle/handoff/fcb_tree_search_got.csv \
  testdata/oracle/handoff/FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv \
  testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv \
  testdata/oracle/handoff/EXTERNAL_VERIFIER_REQUEST.md \
  testdata/oracle/handoff/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md \
  testdata/oracle/handoff/create_verifier_bundle.sh \
  testdata/oracle/handoff/validate_verifier_output.sh
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
  testdata/oracle/handoff/tame_gain_taming_got.csv \
  testdata/oracle/handoff/fcb_tree_search_expected_template.csv \
  testdata/oracle/handoff/fcb_tree_search_got.csv \
  testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv \
  testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv
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
