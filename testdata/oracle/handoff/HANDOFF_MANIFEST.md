# Oracle Handoff Manifest

Date: 2026-05-14

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
| `fcb_tree_search_got.csv` | `field,frame,sub,index,got` | 10194 | `7d8358f1e21c8b0b8ed605d7ea1cd71dbbfa9806e3d9d535b46a09f76814b8ce` |
| `FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md` | n/a | n/a | `102cfaccf8bc984507294b2d4ddee7272cd20befdeb513765b0989fb8fa1ada5` |
| `fcb_tree_search_user_audio_expected_template.csv` | `field,frame,sub,index,expected` | 10194 | `4644b4330f2401496de267fd2f33c33d6e245715e1460accecb1cd51a3164d40` |
| `fcb_tree_search_user_audio_got.csv` | `field,frame,sub,index,got` | 10194 | `70b70b6f76e224172edd54a3863ebbc3c11e9f8859419af4f9964afe0169f3ae` |
| `decoder_itu_stage_expected_template.csv` | `source,frame,sub,field,index,expected` | 10491 | `a06d13af94cb2d1b60c6d6f13caeb6567a559e0b6a5d6959d56a6225f1a522a7` |
| `decoder_itu_stage_got.csv` | `source,frame,sub,field,index,got` | 10491 | `4ec339672a8abd05d94b9aee060f696b75416d40b1dcc1158128ffc4270a4492` |
| `DECODER_ITU_FCB_POSITION_CLARIFICATION_PROMPT.md` | n/a | n/a | `adf86bb920b590d25103bb4b1982aba459e1380a9886e3526f8ee535d7cb6790` |
| `decoder_itu_fcb_position_clarification_expected_template.csv` | `C,i0,i1,i2,i3,jx,m0,m1,m2,m3,note` | 3 | `33b5766a587219fedc26a7d6e8b757eade3dd03f31f9394834bd4602968ca450` |
| `DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md` | n/a | n/a | `b5cf97ca846b7f164b50b3dc65bad54ff0379cba7912b2de55e701ae341951ea` |
| `decoder_itu_frame0_hp_input_inverse_expected_template.csv` | `source,frame,sub,field,index,expected` | 480 | `c821398c859954af18c4c9f448c85bfbb923a366906692dccd9011f2675bd814` |
| `DECODER_TAME_STAGE_WIDE_ONSET_PROMPT.md` | n/a | n/a | `3e2ee7526730795b970a34761228730ab7b1555e2669ff55c287f040f42926e8` |
| `decoder_tame_stage_wide_onset_expected_template.csv` | `frame,sub,past_exc_pre_acb_q0_0..152,lp_a_q12_0..10,adaptive_gain_q14,fixed_gain_q14,adaptive_v_q0_0..39,fixed_c_q13_0..39,pitch_contrib_q0_0..39,fixed_contrib_q0_0..39,excitation_u_q0_0..39,synth_s_q0_0..39` | 116 | `22df2b7c1616fa61469f5ac9fec6f9b74801a57c6cffc90cbb39ebf1f2874e3c` |
| `decoder_tame_stage_wide_onset_got.csv` | `frame,sub,past_exc_pre_acb_q0_0..152,lp_a_q12_0..10,adaptive_gain_q14,fixed_gain_q14,adaptive_v_q0_0..39,fixed_c_q13_0..39,pitch_contrib_q0_0..39,fixed_contrib_q0_0..39,excitation_u_q0_0..39,synth_s_q0_0..39` | 116 | `61738779e6f21b5414c16af26510072453eb307d6459fc3d3cd1a352c5809706` |
| `DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md` | n/a | n/a | `0829d37f52081b42e02ff82d568bd37008d9db8fef710012d0356e592edf04d2` |
| `decoder_tame_pre_acb_history_expected_template.csv` | `source,frame,sub,field,index,expected` | 153 | `e1533e9a2a6b67d534cb287254e57c51a11c0ae0326242ec9cf07516ec9160ad` |
| `DECODER_TAME_EXCITATION_HISTORY_PROMPT.md` | n/a | n/a | `2a6075561facc1bfc4cf1219c5c92582f0994274c92cb0084e41d0a7a1639b02` |
| `decoder_tame_excitation_history_expected_template.csv` | `source,frame,sub,field,index,expected` | 9360 | `51953e40c067649d593128fcf042bb39061fcec74c59b010c74554614d44607a` |
| `DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md` | n/a | n/a | `cf9a7f373bb1f1ea2dd79cb539fe8588c54658c0d4abc7b30d98e67718b82de8` |
| `decoder_pitch_instability_decision_expected_template.csv` | `source,frame,sub,field,index,expected` | 9552 | `116b9736c5c2fbe239c3fa0803bba5db7a6d3bfb3cec09af9a13539133bdd4ad` |
| `decoder_pitch_instability_decision_got.csv` | `source,frame,sub,field,index,got` | 9552 | `052a91da9ad66f6b1d9748d0fca966e18cd315b97187688be21c1a818b07194b` |
| `DECODER_SUPPORT_TABLES_PROMPT.md` | n/a | n/a | `c0121878fcbbfc351e42120e28ad0268a8d8fdaa88f47c543e408f1ac049e6b9` |
| `decoder_support_tables_expected_template.csv` | `table,row,col,expected` | 264 | `dd0a3d086c4938fdef7b664961de726097fc8f622674706f94bfab921dcba28f` |
| `EXTERNAL_VERIFIER_REQUEST.md` | n/a | n/a | `d65189e31ed3189ada680e26efd2e934d71d8286023d0c27cef5499df66aa725` |
| `REMAINING_CONFORMANCE_VERIFIER_PROMPT.md` | n/a | n/a | `0da88d8dc36ccedc36906df62ce781c04056f2e5bd5597187e6ec26b3dd5eadc` |
| `create_verifier_bundle.sh` | n/a | n/a | `7e8e88c53952c08be61e16af8282eb9902d87d3610a31e2879231ef3a6ff1f9c` |
| `validate_verifier_output.sh` | n/a | n/a | `99af1ad486a42bf970a9520d8c3fd7ad4ee9141e67e01efb29a185362949047d` |

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
| `fcb_tree_search_expected_template.csv` | 10194 | `3de52bccab39e2fd75d4e62af80d9d54d523441c2bc1c3ae16905584fd2cc2c5` |
| `fcb_tree_search_user_audio_expected_template.csv` | 10194 | `4fb6ff3d96ab786166970ed65919441cd1986a5fbdc3393faa240c65a0239bf2` |
| `decoder_itu_fcb_position_clarification_expected.csv` | 30 | `e2a8ff28bbd291c251e9d8dad13e2d82da9985bf342246ed9c7ac9fcf145fca2` |
| `decoder_tame_stage_wide_expected.csv` | 1518 | `ca9809900a74be1345f844bcc00090d31b0bfe4c3ada6bd1d82702d5c578b35e` |
| `decoder_itu_frame0_hp_input_inverse_expected_template.csv` | 480 | `5dbfcd17059df81a630a4c0391094937c99bb1373a04af12e5b05362f31578cf` |

## Controlled Numeric Diagnostics

| File | Data rows | SHA-256 | Note |
| --- | ---: | --- | --- |
| `tame_short_pitch_relation.csv` | 48 | `9d73b5aea66fa153b2d26353e750d9beddae4cc1b8a2df33c35bc80fb111560c` | Verifier-produced relation table for TAME short-pitch `T_frac=0`; supports the phase-0 FIR adaptive-codebook fix. |
| `decoder_itu_stage_frame0_chain_expected.csv` | 840 | `4841ce9825263051d33a96234ef5f3a578e4be25785cb2153a9ccbda04eefcab` | Verifier-produced frame-0 chain artifact for ALGTHM, TAME, and OVERFLOW; fixed-codebook subframe-0 rows exact-match after the stream-start pitch-sharpening fix. |

## Partially Filled Files

| File | Filled `expected` cells | Blank `expected` cells | SHA-256 | Next action |
| --- | ---: | ---: | --- | --- |
| `decoder_itu_stage_expected.csv` | 1562 | 8929 | `158a1c310206163f60022a00d0e3002a23ee525d3dfd9955cfd4e8b0b3be261b` | Use as a localization artifact only; fixed-codebook fourth-pulse clarification is resolved, so request additional LP/synthesis state rows before promoting to a strict gate. |
| `decoder_tame_stage_wide_onset_expected_template.csv` | 2436 | 44660 | `a8cb11efbed61d9c5de107b619420cce342356b2e9b8c733e89bf427f07b51f5` | Use as a localization artifact only. Local compare is exact `444/2436`; all filled `past_exc_pre_acb_q0` rows mismatch, confirming that late TAME ACB disagreement is inherited from prior excitation history. |
| `decoder_pitch_instability_decision_expected_template.csv` | 2993 | 6559 | `f49b996365a34a5a49c1962135bafd11c0ad1eaf7f3fa839e952c392ed7618c7` | Use as a decision artifact: `pitch_instability_flag_q0` is filled `597/597` and all values are `0`, so the verifier found no good-frame decoder-side pitch-instability limiter. Local compare is exact `2987/2993`; the six TAME `117/1` scalar mismatches are localization evidence for prior excitation/history divergence. |

## Currently Unfilled Files

| File | Blank `expected` cells | Next action |
| --- | ---: | --- |
| `encoder_closedloop_stage_expected_template.csv` | 100848 | Fill from `ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md` if a broader closed-loop stage oracle is required. |
| `decoder_tame_pre_acb_history_expected_template.csv` | 153 | Blocked unless an independent prior-excitation trace is available; `adaptive_v_q0` rows alone do not uniquely determine the FIFO. |
| `decoder_tame_excitation_history_expected_template.csv` | 9360 | Blocked; full forward decode requires numeric support tables and state not available from the current clean-room inputs. |
| `decoder_support_tables_expected_template.csv` | 264 | Blocked for full completion under current clean-room inputs; spec text covers only a subset, while gain VQ/map tables are simulation-software numeric tables. |

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
  testdata/oracle/handoff/DECODER_ITU_FRAME0_HP_INPUT_INVERSE_PROMPT.md \
  testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv \
  testdata/oracle/handoff/DECODER_TAME_STAGE_WIDE_ONSET_PROMPT.md \
  testdata/oracle/handoff/decoder_tame_stage_wide_onset_expected_template.csv \
  testdata/oracle/handoff/decoder_tame_stage_wide_onset_got.csv \
  testdata/oracle/handoff/DECODER_TAME_PRE_ACB_HISTORY_PROMPT.md \
  testdata/oracle/handoff/decoder_tame_pre_acb_history_expected_template.csv \
  testdata/oracle/handoff/DECODER_TAME_EXCITATION_HISTORY_PROMPT.md \
  testdata/oracle/handoff/decoder_tame_excitation_history_expected_template.csv \
  testdata/oracle/handoff/DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md \
  testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv \
  testdata/oracle/handoff/decoder_pitch_instability_decision_got.csv \
  testdata/oracle/handoff/DECODER_SUPPORT_TABLES_PROMPT.md \
  testdata/oracle/handoff/decoder_support_tables_expected_template.csv \
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
  testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv \
  testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv \
  testdata/oracle/handoff/decoder_tame_stage_wide_onset_expected_template.csv \
  testdata/oracle/handoff/decoder_tame_stage_wide_onset_got.csv \
  testdata/oracle/handoff/decoder_tame_pre_acb_history_expected_template.csv \
  testdata/oracle/handoff/decoder_tame_excitation_history_expected_template.csv \
  testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv \
  testdata/oracle/handoff/decoder_pitch_instability_decision_got.csv \
  testdata/oracle/handoff/decoder_support_tables_expected_template.csv
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
