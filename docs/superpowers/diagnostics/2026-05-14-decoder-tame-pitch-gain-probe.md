# Decoder TAME Pitch-Gain Probe

Date: 2026-05-14

## Context

The decoder TAME vector still has a large late `pastExc` amplitude gap against
the clean-room numeric oracle. The latest useful filled oracle is
`decoder_tame_stage_wide_onset_expected_template.csv`, where filled
`past_exc_pre_acb_q0` rows show production history is roughly too large before
the late `117..119` checkpoint.

## Verifier Result

The requested clean-room verifier output:

```text
/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_gain_taming_108_137_expected.csv
```

was not a complete adaptive-gain/taming oracle:

- Rows: `210`
- Filled: `60`
- Blank: `150`
- Filled fields: `bitstream_ga` and `bitstream_gb` only
- Blank fields: `adaptive_gain_before_taming_q14`, `taming_flag_q0`,
  `adaptive_gain_after_taming_q14`, `taming_error_before_q0`,
  `taming_error_after_q0`

Reason: under the clean-room boundary, the verifier did not have independent
numeric gain VQ tables or a prior accumulated taming/error state. This artifact
is therefore not worth ingesting as a decoder oracle; it only reconfirms the
bitstream gain indices.

## Local Oracle Probe

The test-only diagnostic window scan added in commit `af458fc` compares
candidate perturbations against filled `past_exc_pre_acb_q0` oracle rows.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_PAST_EXC_VARIANT_WINDOW_SCAN=1 \
  G729_DECODER_TAME_PAST_EXC_VARIANT=pitch_gain_cap_0p95 \
  G729_DECODER_TAME_PAST_EXC_SCAN_START=112 \
  G729_DECODER_TAME_PAST_EXC_SCAN_END=144 \
  G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
  go test ./internal/decoder -run TestDecoderTAMEPastExcVariantWindowScan -count=1 -v
```

Result:

```text
production: count=918 exact=0 wantRMS=204.89 gotRMS=384.97 errRMS=232.73 meanAbs=194.62 maxAbs=447 corr=0.8622 scale=0.4589
best pitch_gain_cap_0p95 window [112,132): exact=16 errRMS=83.99 meanAbs=64.34 maxAbs=176 corr=0.9380 scale=0.8108
```

For comparison, the older fixed-gain damping probe is weaker on the same
oracle surface:

```text
best fixed_gain_half window [40,72): exact=11 errRMS=153.75 meanAbs=128.23 maxAbs=273 corr=0.9604 scale=0.5795
```

## State Trigger Probe

The fixed window is only a localization tool, so the next diagnostic searched
for a runtime state trigger that reproduces the same effect without using the
known TAME frame number.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_PITCH_CAP_TRIGGER_SEARCH=1 \
  G729_DECODER_ITU_VECTOR_FRONTIER_TOP=12 \
  go test ./internal/decoder -run TestDecoderTAMEPitchGainCapTriggerSearch -count=1 -v
```

Best trigger:

```text
production: count=918 exact=0 wantRMS=204.89 gotRMS=384.97 errRMS=232.73 meanAbs=194.62 maxAbs=447 corr=0.8622 scale=0.4589
production pstRMS=5081.86
gp>cap+past>=240: applied=19 errRMS=94.30 meanAbs=79.16 maxAbs=173 corr=0.8942 scale=0.8933 pstRMS=1156.72
```

This is close to the best fixed-window `pitch_gain_cap_0p95` result and strongly
points at an adaptive-gain/history problem around the late TAME onset.

## Initial Cross-Vector Audit

The same broad trigger was then applied across the good Annex A decoder
vectors.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_PITCH_CAP_TRIGGER_CROSS_VECTOR=1 \
  go test ./internal/decoder -run TestDecoderPitchGainCapTriggerCrossVectorAudit -count=1 -v
```

Result:

```text
vector    applied    prodRMS    candRMS   prodSNR   candSNR  prodCor  candCor
ALGTHM         15    2068.11    2844.58      8.65      5.88   0.9810   0.9573
SPEECH        463     143.44     699.68     23.29      9.53   0.9978   0.9553
FIXED           0      28.05      28.05     14.14     14.14   0.9859   0.9859
LSP             0      65.13      65.13     20.20     20.20   0.9952   0.9952
PITCH        1387     888.43    1986.39     14.14      7.15   0.9925   0.9542
TAME           19    5081.86    1156.72      6.50     19.35   0.9716   0.9950
TEST            5     103.93     197.92     22.76     17.16   0.9975   0.9915
OVERFLOW      146   10396.15    8040.17     -1.58      0.66   0.4340   0.3829
```

The trigger is therefore not production-safe. It isolates the TAME failure
mode, but it overfires on normal voiced content and causes severe regressions
on `SPEECH`, `PITCH`, `ALGTHM`, and `TEST`.

## Activation Distribution

The broad trigger overfires because `gp > 0.95` is common on voiced content.
The next diagnostic summarized the activation distribution across good Annex A
vectors.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_PITCH_CAP_ACTIVATION_AUDIT=1 \
  G729_DECODER_ITU_VECTOR_FRONTIER_TOP=5 \
  go test ./internal/decoder -run TestDecoderPitchGainCapActivationAudit -count=1 -v
```

Summary:

```text
vector     events   frames     gpMed   pastMed   tailMed      vMed    fixMed    p/fMed past/fMed
ALGTHM         29       35     17839     336.1     254.9      56.1     409.2      0.86      0.89
SPEECH       1953     3750     17265     118.2     126.7     121.9      53.4      2.70      2.48
FIXED          53      120     17839      12.8       8.1       7.1      49.0      0.15      0.25
LSP          2289     2232     16985      18.8      19.1      18.8       4.4      3.95      3.93
PITCH        1899     1835     17839     542.4     517.8     652.9     246.2      2.22      2.01
TAME          245      128     16502     300.9     304.2     296.3      37.8      7.39      7.53
TEST           67      176     17943      68.9      80.3      64.2      64.5      1.32      1.01
OVERFLOW      755      384     16683    7494.9    7567.1    7545.5      31.3    234.98    231.34
```

TAME differs from normal voiced vectors by low fixed contribution and high
pitch/fixed or past/fixed ratios. However, `OVERFLOW` has the same shape at a
much larger amplitude, so this is still a stress-vector heuristic rather than a
normative decoder rule.

## Refined Trigger Grid

The refined grid searched state triggers that maximize TAME improvement while
penalizing regressions on the other good Annex A vectors.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_PITCH_CAP_TRIGGER_GRID_CROSS_VECTOR=1 \
  G729_DECODER_ITU_VECTOR_FRONTIER_TOP=12 \
  go test ./internal/decoder -run TestDecoderPitchGainCapTriggerGridCrossVectorAudit -count=1 -v
```

Best scored rows:

```text
trigger                                           score    tameD    maxReg    regVec  tameApp   othApp    pastErr   tameRMS
gp>16000+past>=220+fixed<=40                    4068.52  4081.08      4.14    SPEECH       26      176     103.00   1000.78
gp>16500+v>=220+fixed<=40                       4065.24  4070.47      1.74    SPEECH       16      104     102.43   1011.39
gp>16500+past>=220+fixed<=40                    4058.11  4065.73      2.50    SPEECH       17      128     102.08   1016.13
```

Detailed cross-vector audit for the selected diagnostic trigger:

```text
trigger=gp>16000+past>=220+fixed<=40
vector    applied    prodRMS    candRMS   prodSNR   candSNR  prodCor  candCor
ALGTHM          0    2068.11    2068.11      8.65      8.65   0.9810   0.9810
SPEECH         16     143.44     147.58     23.29     23.04   0.9978   0.9977
FIXED           0      28.05      28.05     14.14     14.14   0.9859   0.9859
LSP             0      65.13      65.13     20.20     20.20   0.9952   0.9952
PITCH          20     888.43     888.57     14.14     14.14   0.9925   0.9925
TAME           26    5081.86    1000.78      6.50     20.61   0.9716   0.9959
TEST            0     103.93     103.93     22.76     22.76   0.9975   0.9975
OVERFLOW      140   10396.15    8019.92     -1.58      0.68   0.4340   0.3873
```

This is a much sharper localization than the broad pastRMS-only trigger. It
suggests the runaway TAME history is tied to high adaptive gain while fixed
contribution is low, i.e. a pitch-feedback-dominated excitation state.

## Interpretation

`pitch_gain_cap_0p95` is a strong localization probe: limiting adaptive gain
for global subframes `112..131` makes the later TAME past-excitation history
much closer to oracle.

It is not yet a production fix. The PDF-visible decoder clause in
`docs/superpowers/specs/itu/G729E.txt` section `4.1.5` describes
reconstructing the adaptive gain from the received gain-codebook index through
equation `(73)`. It does not expose a decoder-side pitch-instability/taming
state machine. The same recommendation text lists `Taming.c` as pitch
instability control in the simulation-software inventory, but the clean-room
rule forbids inspecting that implementation source.

Therefore:

- Do not add an unconditional decoder `gp <= 0.95` clamp to production.
- Do not add the current `gp>0.95 && pastExcRMS>=240` trigger to production.
- Do not add the refined `gp>16000 && pastRMS>=220 && fixedRMS<=40` trigger to
  production without a Recommendation-backed decoder pitch-instability rule or
  independent oracle confirmation.
- Do not treat the incomplete 108..137 verifier CSV as a decoder oracle.
- Next useful work is a PDF-visible/state-machine derivation of decoder
  pitch-instability control, or a clean-room oracle that confirms the gain
  limiting decision and state update around the TAME/OVERFLOW stress surfaces.

## Verification

```sh
env GOCACHE=/tmp/go-build go test ./internal/decoder -count=1
env GOCACHE=/tmp/go-build G729_DECODER_PITCH_CAP_ACTIVATION_AUDIT=1 go test ./internal/decoder -run TestDecoderPitchGainCapActivationAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_PITCH_CAP_TRIGGER_GRID_CROSS_VECTOR=1 go test ./internal/decoder -run TestDecoderPitchGainCapTriggerGridCrossVectorAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -count=1 -v
```
