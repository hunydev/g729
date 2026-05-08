# Decoder Stage Handoff Retry

Date: 2026-05-07

## Status

The files currently present at:

- `/home/exedev/g729/testdata/oracle/handoff/decoder_stage_expected.csv`
- `/home/exedev/g729/testdata/oracle/handoff/decoder_stage_summary.csv`

have the requested shape and contain numeric values only, but they are
not valid external oracle artifacts. They were produced by a local repo
test that calls this repository's decoder implementation.

Current local evidence:

```sh
wc -l testdata/oracle/handoff/decoder_stage_expected.csv testdata/oracle/handoff/decoder_stage_summary.csv
```

```text
  56491 testdata/oracle/handoff/decoder_stage_expected.csv
     25 testdata/oracle/handoff/decoder_stage_summary.csv
```

The local compare harness reports that the current expected file is
almost identical to a fresh local dump, except for LSP/LP state values
after skipped SPEECH frame ranges:

```sh
G729_COMPARE_DECODER_STAGE_HANDOFF=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderStageHandoff -count=1 -v
```

```text
decoder_stage handoff: exact 55952/56490 99.05% mismatches=538 blank_expected=0 missing_got=0
field lp_a_q12: exact 1360/1540 88.31% mismatches=180
field lsf_q13: exact 1221/1400 87.21% mismatches=179
field lsp_q15: exact 1221/1400 87.21% mismatches=179
```

This means the current files cannot identify the decoder quality defect.
They should be replaced by an independent numeric verifier output.

## Second Attempt

A second handoff overwrote `decoder_stage_expected.csv` by converting the
local `decoder_stage_got.csv` file to the `expected` column shape. This
also is not a valid external oracle. It is useful only as a local
snapshot of the current implementation.

Observed compare:

```sh
G729_COMPARE_DECODER_STAGE_HANDOFF=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderStageHandoff -count=1 -v
```

```text
decoder_stage handoff: exact 56490/56490 100.00% mismatches=0 blank_expected=0 missing_got=0
```

That 100% result is expected for a self-oracle and must not be used as
decoder conformance evidence.

## Local Repo Side

Local implementation dumps must use `got`, not `expected`:

```sh
G729_DUMP_DECODER_STAGE_GOT=1 go test ./internal/decoder -run TestDecoderStageGotDump -count=1 -v
```

This writes:

- `/home/exedev/g729/testdata/oracle/handoff/decoder_stage_got.csv`
- `/home/exedev/g729/testdata/oracle/handoff/decoder_stage_got_summary.csv`

The local compare gate is:

```sh
G729_COMPARE_DECODER_STAGE_HANDOFF=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderStageHandoff -count=1 -v
```

Strict gate after a real external expected file is present:

```sh
G729_COMPARE_DECODER_STAGE_HANDOFF=1 \
G729_REJECT_DECODER_STAGE_SELF_ORACLE=1 \
G729_REQUIRE_COMPLETE_DECODER_STAGE_HANDOFF=1 \
G729_REQUIRE_EXACT_DECODER_STAGE_HANDOFF=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderStageHandoff -count=1 -v
```

## Verifier Prompt

Use this prompt for the external verifier. The verifier must not run
this repository's decoder tests to fill `expected`.

```text
You are the external numeric verifier for a clean-room Go G.729 codec.

Repository root:
/home/exedev/g729

Clean-room boundary:
- Do not write source code or implementation logic into the repository.
- Do not put function names, branch descriptions, code snippets, or
  implementation-derived prose into artifacts.
- The only allowed repository outputs are numeric CSV artifacts with the
  exact headers requested below.
- Do not run this repository's local decoder dump test to produce
  expected values. In particular, do not use:
  G729_DUMP_DECODER_STAGE_GOT=1 go test ./internal/decoder -run TestDecoderStageGotDump
  or any older local dump command that writes decoder_stage_expected.csv.

Input files:
- SPEECH bitstream:
  /home/exedev/g729/testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.BIT
- SPEECH reference PCM, if useful for final PCM cross-check only:
  /home/exedev/g729/testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.PST
- Asterisk raw RTP payload bytes:
  /home/exedev/g729/testdata/external/asterisk_payload.g729

Output files to overwrite:
- /home/exedev/g729/testdata/oracle/handoff/decoder_stage_expected.csv
- /home/exedev/g729/testdata/oracle/handoff/decoder_stage_summary.csv

decoder_stage_expected.csv header:
source,frame,sub,field,index,expected

decoder_stage_summary.csv header:
source,frames,field,exact_notes,value

Sources and frames:
- SPEECH frames: 0..19, 100..104, 1122..1126
- ASTERISK frames: 0..29
- ASTERISK_VOICED: choose 10 voiced candidate frames from the Asterisk
  payload and use the original frame numbers from that payload

For every selected speech frame, write frame-level rows with:
sub=-1,index=-1
fields:
L0,L1,L2,L3,P1,P0,C1,S1,GA1,GB1,P2,C2,S2,GA2,GB2

For each subframe, write scalar rows with index=-1:
pitch_t_int
pitch_t_frac
adaptive_gain_q14
fixed_gain_q14
fixed_gain_x1e6

For each subframe, write array rows:
lsf_q13, index 0..9
lsp_q15, index 0..9
lp_a_q12, index 0..10
adaptive_v_q0, index 0..39
fixed_c_q13, index 0..39
pitch_contrib_q0, index 0..39
fixed_contrib_q0, index 0..39
excitation_u_q0, index 0..39
synth_s_q0, index 0..39
postfilter_s_q0, index 0..39
hp_q0, index 0..39
pcm_q0, index 0..39

Summary rows required for each source:
total_frames
speech_frames
sid_frames
malformed_frames
pcm_rms_x1e6
pcm_peak
pcm_clipped_samples
unavailable_fields

Validation before reporting completion:
- decoder_stage_expected.csv has exactly 56,490 data rows plus the header.
- decoder_stage_summary.csv has the required summary rows for SPEECH,
  ASTERISK, and ASTERISK_VOICED.
- The expected column has no blank values.
- unavailable_fields is 0 for every source, unless a field is genuinely
  unavailable in your independent verifier. If any field is unavailable,
  keep the row and leave expected blank, then increment unavailable_fields.
- Report only aggregate counts and file paths back to the repo-side agent.
```
