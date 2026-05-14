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

## Verifier Handoff

A focused clean-room verifier handoff now targets the remaining decision point:
does any good-frame decoder-side pitch-instability/taming limiter exist on the
stress subframes?

Files:

```text
testdata/oracle/handoff/DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md
testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv
testdata/oracle/handoff/decoder_pitch_instability_decision_got.csv
```

Template shape:

```text
source,frame,sub,field,index,expected
rows=9552
sources: TAME=1680 rows, SPEECH=256, PITCH=320, OVERFLOW=7296
fields per targeted subframe=16
```

The template is intentionally blank. The verifier should fill only numeric
`expected` cells it can independently derive. The most important rows are
`pitch_instability_flag_q0` and
`adaptive_gain_after_pitch_instability_q14`. If the Recommendation-backed
answer is "no good-frame decoder limiter", those flags should be `0` and the
after-gain should match the decoded adaptive gain wherever the before-gain is
independently known.

Verifier return:

```text
rows=9552 filled=2993 blank=6559
bitstream_ga=597/597
bitstream_gb=597/597
pitch_t_int=597/597
pitch_t_frac=597/597
pitch_instability_flag_q0=597/597, all 0
```

Local compare:

```text
exact 2987/2993 99.80%
blanks=6559
mismatches=6
```

The result closes the decoder-side cap hypothesis for good frames: the verifier
found no Recommendation-backed pitch-instability limiter on any targeted
subframe. The six mismatches are all TAME frame `117`, subframe `1` scalar
rows (`fixed_gain_q14` and RMS/ratio values). They are consistent with the
known local prior-excitation/history divergence rather than evidence for a
missing good-frame gain limiter.

## Gain EC Q25 Cross-Vector Check

After the cap hypothesis closed, the next tempting local candidate was changing
the fixed-codebook energy Q correction from `26` to `25`. This behaves like a
fixed-gain damping path and improves TAME, but the cross-vector audit rejects it
as a production formula change.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_GAIN_EC_Q25_CROSS_VECTOR=1 \
  go test ./internal/decoder -run TestDecoderTAMEGainECQ25CrossVectorAudit -count=1 -v
```

Result:

```text
vector       frames   bad    prodSNR    ec25SNR     delta    prodRMS    ec25RMS   prodCorr   ec25Corr verdict
TAME            128     0       6.50      12.54      6.04   14830.60   10358.40      0.972      0.972 improves-local-vector-only
SPEECH         3750     0      23.29      10.07    -13.22    2050.42    1448.46      0.998      0.998 regresses
PITCH          1835     0      14.14       7.58     -6.56    3796.49    2684.32      0.993      0.993 regresses
OVERFLOW        384     0      -1.58      -1.89     -0.31   10623.45   10413.38      0.434      0.374 neutral
```

Interpretation:

- The TAME improvement is an amplitude-damping side effect, not evidence that
  `ecQ=25` is the correct decoder gain formula.
- SPEECH and PITCH are good-frame vectors with no bad frames, and both regress
  sharply under `ecQ=25`.
- Keep the strict gain path at `ecQ=26`; use `ecQ=25` only as a diagnostic
  amplitude probe when explaining TAME's accumulated excitation history.

## Gain Variant Cross-Vector Check

The same cross-vector rejection now covers the other tempting gain formula
variant, `gain_gamma_q14`.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_GAIN_VARIANT_CROSS_VECTOR=1 \
  go test ./internal/decoder -run TestDecoderTAMEGainVariantCrossVectorAudit -count=1 -v
```

Result:

```text
vector     variant                gSNR     delta        rms       corr
TAME       production             6.50      0.00   14830.60      0.972
TAME       gain_ec_q25           12.54      6.04   10358.40      0.972
TAME       gain_gamma_q14         8.71      2.21    7449.08      0.971
SPEECH     production            23.29      0.00    2050.42      0.998
SPEECH     gain_ec_q25           10.07    -13.22    1448.46      0.998
SPEECH     gain_gamma_q14         5.80    -17.49    1025.95      0.998
PITCH      production            14.14      0.00    3796.49      0.993
PITCH      gain_ec_q25            7.58     -6.56    2684.32      0.993
PITCH      gain_gamma_q14         4.64     -9.50    1898.35      0.993
FIXED      production            14.14      0.00     126.15      0.986
FIXED      gain_ec_q25            8.00     -6.14      89.26      0.986
FIXED      gain_gamma_q14         4.90     -9.24      63.15      0.986
```

Interpretation:

- `gain_gamma_q14` improves TAME (`6.50 -> 8.71 dB`) because it damps the
  recurrent excitation, but it is even worse than `ecQ=25` on SPEECH, PITCH,
  and FIXED.
- Both gain variants are rejected as production decoder changes.
- The remaining TAME issue is a recurrent history/energy-balance localization
  problem, not a scalar gain formula correction.

## Onset Candidate Range Audit

The exhaustive fixed-gain-half window scans are useful but slow. The current
best diagnostic window is:

```text
fixed_gain_half, global subframes [52,239)
frames [26,120), candidate RMS 1185.18
```

A compact fixed-range audit now captures the important behavior without
rerunning the exhaustive subframe search.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_ONSET_CANDIDATE_RANGE_AUDIT=1 \
  go test ./internal/decoder -run TestDecoderTAMEOnsetCandidateRangeAudit -count=1 -v
```

Result:

```text
candidate                range         frames   refRMS   outRMS   errRMS      gSNR    segSNR    corr
production               all             128  10735.2  14830.6   5081.9      6.50     11.04   0.972
production               window-start      8  10842.2  11338.2    603.3     25.09     27.32   1.000
production               first-1.25       12  10809.8  13659.5   2890.2     11.46     11.59   0.999
production               first-1.50       12  10818.6  16298.6   5555.9      5.79      5.86   0.998
production               late-oracle      12  10821.3  19976.9   9226.5      1.38      1.42   0.997
fixed_half_sf52_239      all             128  10735.2  10757.3   1185.2     19.14     21.39   0.994
fixed_half_sf52_239      window-start      8  10842.2  10883.0   1035.7     20.40     23.54   0.995
fixed_half_sf52_239      first-1.25       12  10809.8  10554.0    852.3     22.06     23.52   0.997
fixed_half_sf52_239      first-1.50       12  10818.6  11497.9   1200.0     19.10     19.30   0.996
fixed_half_sf52_239      late-oracle      12  10821.3  12239.4   1562.3     16.81     17.11   0.998
fixed_half_frame26_120   all             128  10735.2  10750.3   1187.8     19.12     21.38   0.994
fixed_half_frame26_120   window-start      8  10842.2  10883.0   1035.7     20.40     23.54   0.995
fixed_half_frame26_120   first-1.25       12  10809.8  10554.0    852.3     22.06     23.52   0.997
fixed_half_frame26_120   first-1.50       12  10818.6  11497.9   1200.0     19.10     19.30   0.996
fixed_half_frame26_120   late-oracle      12  10821.3  12173.4   1583.6     16.69     17.04   0.997
gain_ec_q25_cutover10    all             128  10735.2  11760.7   2160.3     13.93     16.98   0.986
gain_ec_q25_cutover10    window-start      8  10842.2  10165.2    820.1     22.43     22.60   0.999
gain_ec_q25_cutover10    first-1.25       12  10809.8  10932.2    534.4     26.12     27.07   0.999
gain_ec_q25_cutover10    first-1.50       12  10818.6  12757.3   2136.0     14.09     14.35   0.997
gain_ec_q25_cutover10    late-oracle      12  10821.3  14843.8   4142.4      8.34      8.43   0.997
```

Interpretation:

- The fixed-gain-half window improves overall TAME (`6.50 -> 19.14 dB`) and
  the late oracle-adjacent range (`1.38 -> 16.81 dB`).
- The same window worsens the early window-start range (`25.09 -> 20.40 dB`),
  so it is not a direct production formula fix.
- `gain_ec_q25_cutover10` improves the first over-amplified onset range more
  strongly (`11.46 -> 26.12 dB`), but it is weaker late and already rejected by
  the cross-vector audit.
- The useful localization point is global subframe `52` / frame `26`; the next
  oracle should target excitation and gain state around frame `26`, then the
  visible onset at frames `49..72`.

## Frame-26 Onset Verifier Result

The focused clean-room request for frame `26` and visible-onset frames produced:

```text
/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_onset_frame26_expected.csv
rows=22820 filled=280 blanks=22540
```

Filled fields:

```text
bitstream_ga: 70/70
bitstream_gb: 70/70
pitch_t_int: 70/70
pitch_t_frac: 70/70
```

Local compare:

```text
decoder_tame_onset_frame26: exact 280/280 100.00% blanks=22540 mismatches=0 missing_got=0
```

The artifact is clean and exact for bitstream and decoded pitch scalar rows, but
it does not provide the needed gain/history oracle. The verifier left
`adaptive_gain_q14`, `fixed_gain_q14`, `past_exc_pre_acb_q0`,
`adaptive_v_q0`, contribution vectors, `excitation_u_q0`, and RMS summaries
blank because independent gain VQ tables, gain predictor state, and prior
excitation history were unavailable.

This means the current clean-room verifier path has reached a dependency wall
for internal decoder state. Further requests for the same internal TAME
gain/history values are unlikely to help unless the verifier gets an
independent numeric source for the missing support tables or a fully independent
forward decoder trace.

## State-Carry Reset Audit

With internal clean-room oracle blocked, a PST-only reset audit checks which
local cross-frame state carries the late TAME over-amplification. This is not a
production-fix search; one-shot resets intentionally break decoder continuity.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_STATE_CARRY_RESET_AUDIT=1 \
  go test ./internal/decoder -run TestDecoderTAMEStateCarryResetAudit -count=1 -v
```

Selected result rows:

```text
variant                       range             gSNR    deltaG      corr
production                    all               6.50     +0.00     0.972
production                    window-start     25.09     +0.00     1.000
production                    first-1.50        5.79     +0.00     0.998
production                    late-oracle       1.38     +0.00     0.997
reset_gain_f26                all               7.70     +1.21     0.973
reset_gain_f26                late-oracle       2.18     +0.80     0.998
reset_past_exc_f26            window-start      0.82    -24.27     0.462
reset_past_exc_f26            first-1.50       17.95    +12.16     0.995
reset_past_exc_f26            late-oracle       6.42     +5.04     0.998
reset_past_exc_f53            first-1.25        2.36     -9.10     0.653
reset_past_exc_f53            late-oracle      14.99    +13.60     0.993
reset_past_exc_f72            first-1.50        2.11     -3.68     0.654
reset_past_exc_f72            late-oracle      24.12    +22.74     0.999
reset_synth_f72               late-oracle       1.38     -0.00     0.997
reset_filters_f72             late-oracle       1.38     +0.00     0.997
```

Interpretation:

- One-shot `pastExc` resets strongly reduce the late TAME over-amplification,
  especially at frame `72` (`1.38 -> 24.12 dB` in the late-oracle range).
- The same resets badly damage the immediate window after the reset, so this is
  not a production-safe repair.
- Resetting synthesis/postfilter/HP state does not materially change the late
  error. The downstream filter chain is therefore not carrying the TAME growth.
- Gain predictor reset gives only a small improvement and does not explain the
  late envelope gap by itself.
- The remaining target is the upstream excitation feedback path: samples
  entering `pastExc` (`U`) and adaptive-codebook feedback around frames
  `49..72` and `115..116`.

## Feedback Component Window Audit

The next PST-only audit applies upstream component perturbations only inside
selected windows. This keeps continuity outside the window and ranks which
component most affects late TAME over-amplification.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_FEEDBACK_COMPONENT_WINDOW_AUDIT=1 \
  go test ./internal/decoder -run TestDecoderTAMEFeedbackComponentWindowAudit -count=1 -v
```

Selected result rows:

```text
variant                  scope      range          gSNR   deltaG   outRMS   errRMS   corr
production               -          all            6.50    +0.00  14830.6   5081.9  0.972
production               -          first-1.50     5.79    +0.00  16298.6   5555.9  0.998
production               -          late-oracle    1.38    +0.00  19976.9   9226.5  0.997
pitch_gain_cap_0p95      f49_72     late-oracle   27.77   +26.38  10816.2    442.5  0.999
pitch_gain_cap_0p95      f49_72     first-1.50     2.30    -3.49   2993.8   8305.7  0.880
zero_adaptive            f49_72     late-oracle   22.15   +20.76  10241.1    845.2  0.998
zero_adaptive            f49_72     first-1.50     0.54    -5.25   2152.4  10168.6  0.392
fixed_gain_half          f26_120    all           19.12   +12.63  10750.3   1187.8  0.994
fixed_gain_half          f26_120    first-1.50    19.10   +13.31  11497.9   1200.0  0.996
fixed_gain_half          f26_120    late-oracle   16.69   +15.31  12173.4   1583.6  0.997
no_fcb_pitch_enhance     f26_120    late-oracle   -2.74    -4.12  23893.1  14834.1  0.905
force_pitch_frac_zero    f26_120    late-oracle    1.38    +0.00  19976.9   9226.5  0.997
flip_pitch_frac_sign     f26_120    late-oracle    1.38    +0.00  19976.9   9226.5  0.997
```

Interpretation:

- Pitch-fraction perturbations are no-ops on this TAME surface; the issue is
  not fractional pitch phase.
- Removing or capping adaptive contribution inside frames `49..72` makes the
  late oracle range much closer, but it destroys the immediate voiced window.
  That confirms the late error is carried through adaptive feedback, not that
  zeroing/capping pitch is a valid decoder rule.
- `fixed_gain_half` over the broader frame `26..120` window gives the best
  PST-level envelope, but this is still a damping/localization probe. It works
  by changing what enters `pastExc` over a long feedback path.
- Disabling FCB pitch enhancement makes TAME worse, so the current issue is not
  an over-applied FCB pitch-sharpening loop.

## Fixed-Gain Feedback Propagation Audit

The `fixed_gain_half` probe needed one more check: does it help only by
reducing immediate fixed-codebook output, or does that reduction propagate
through the recurrent excitation history?

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_FIXED_FEEDBACK_AUDIT=1 \
  go test ./internal/decoder -run TestDecoderTAMEFixedFeedbackPropagationAudit -count=1 -v
```

PST-level result:

```text
candidate       range                 gSNR    deltaG    outRMS    errRMS      corr
production      all                   6.50     +0.00   14830.6    5081.9     0.972
production      window-start         25.09     +0.00   11338.2     603.3     1.000
production      root-49-72            9.42     +0.00   14346.9    3654.5     0.997
production      late-oracle           1.38     +0.00   19976.9    9226.5     0.997
fixed_gain_half all                  19.12    +12.63   10750.3    1187.8     0.994
fixed_gain_half window-start         20.40     -4.69   10883.0    1035.7     0.995
fixed_gain_half root-49-72           21.38    +11.96   10787.1     922.3     0.996
fixed_gain_half late-oracle          16.69    +15.31   12173.4    1583.6     0.997
```

Internal RMS propagation:

```text
window          subfrm     fixR       uR    pastR       vR   pitchR     dFix       dU       dV
pre-window           8    1.000    1.000    1.000    1.000    1.000      0.0      0.0      0.0
window-start        16    0.500    0.963    0.970    0.969    0.969     20.2      8.4      7.0
root-49-72          46    0.500    0.803    0.806    0.804    0.804     21.6     56.8     55.8
first-1.50          24    0.500    0.788    0.792    0.791    0.791     23.4     68.8     66.9
pre-late             8    0.502    0.764    0.766    0.765    0.765     21.6     87.0     85.4
late-oracle         24    0.834    0.777    0.776    0.779    0.779      7.8     89.4     87.9
full-window        188    0.500    0.813    0.817    0.815    0.815     22.1     60.4     59.1
```

Interpretation:

- At the diagnostic window start, fixed contribution is directly halved
  (`fixR=0.500`), but `U`, `pastExc`, and adaptive-vector RMS are still close
  to production (`0.96..0.97`). This also explains why the immediate
  `window-start` PST range regresses.
- By frames `49..72`, the same fixed damping has propagated through
  `U -> pastExc -> adaptive_v`; `U`, `pastExc`, `v`, and pitch contribution are
  all around `0.80x` production.
- In the late oracle range, the fixed contribution is no longer halved after
  the probe window ends, but the history/adaptive path remains around `0.78x`.
  The improvement is therefore a recurrent feedback-history effect, not a
  direct fixed-codebook output correction.
- This remains a localization probe only. It does not identify a
  Recommendation-backed production rule for halving fixed gain.

## Late ACB/Excitation Oracle Replay

The filled verifier oracle for TAME frames `117..119` separates current-subframe
gain/excitation math from accumulated history.

Commands:

```sh
env GOCACHE=/tmp/go-build \
  G729_COMPARE_DECODER_TAME_ACB_CHECKPOINT=1 \
  go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEACBCheckpoint -count=1 -v

env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_EXCITATION_ORACLE_REPLAY=1 \
  go test ./internal/decoder -run TestDecoderTAMEExcitationOracleReplay -count=1 -v
```

Key result:

```text
adaptive_gain_q14: exact 6/6
local_u:         refRMS=205.62 gotRMS=386.08 errRMS=232.78 corr=0.8639 scale=0.4601
oracle_v_replay: refRMS=205.62 gotRMS=205.67 errRMS=0.33   corr=1.0000 scale=0.9997
implied_fixed:   refRMS=58.54  gotRMS=58.81  errRMS=0.34   corr=1.0000 scale=0.9954
```

Interpretation:

- The late adaptive gain values that the verifier could independently fill are
  exact.
- If verifier `adaptive_v_q0` is injected, local excitation reconstruction
  matches verifier `excitation_u_q0` to rounding noise.
- The late `U` mismatch is therefore inherited from local `adaptive_v_q0` /
  `pastExc` history, not from current-subframe gain application, fixed-codebook
  vector construction, or `BuildExcitation`.

## PastExc Source Backtrace

The filled `past_exc_pre_acb_q0` oracle rows can be mapped from pastExc buffer
index back to the previous `U` subframe that populated each sample. This checks
whether the late oracle gives enough coverage to trace the fault backward.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_PAST_EXC_SOURCE_BACKTRACE=1 \
  go test ./internal/decoder -run TestDecoderTAMEPastExcSourceBacktraceAudit -count=1 -v
```

Aggregate result:

```text
candidate                   count   refRMS   gotRMS   errRMS    scErr    corr   scale  maxAbs
production                    546   204.01   386.68   232.02   100.48  0.8703  0.4592     442
fixed_half_f26_120            546   204.01   292.82   186.10   128.64  0.7762  0.5407     406
fixed_half_sf52_239           546   204.01   292.82   186.10   128.64  0.7762  0.5407     406
pitch_cap_f49_72              546   204.01   196.52   107.17   104.97  0.8575  0.8902     199
zero_adaptive_f49_72          546   204.01   218.11   175.99   154.30  0.6542  0.6119     332
```

Source coverage:

```text
production by source U subframe:
srcSF 234..238 only
```

Interpretation:

- `pitch_cap_f49_72` gives the closest late `pastExc` envelope, but earlier
  PST diagnostics show it damages the immediate voiced window. It remains a
  localization probe only.
- `fixed_gain_half` reduces the over-large local history but does not recover
  the oracle shape as well as the targeted pitch cap.
- The filled oracle rows only backtrace to source `U` subframes `234..238`
  (frames `117..119`). They do not cover the older source `U` subframes
  `230..233` from frames `115..116`, nor the earlier root region around
  frames `49..72`.
- This confirms the current verifier artifacts are enough to prove the late
  mismatch is inherited through `pastExc`, but not enough to independently
  locate the earlier subframe where the history first diverges.

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
- Do not change decoder gain fixed-codebook energy correction from `26` to `25`;
  it improves TAME only by damping and regresses normal good-frame vectors.
- Do not change decoder gain gamma correction to `14`; it also improves TAME
  only by damping and regresses normal good-frame vectors.
- Do not treat the incomplete 108..137 verifier CSV as a decoder oracle.
- Do not treat the frame-26 onset CSV as a gain/history oracle; it only confirms
  bitstream gain indices and decoded pitch values.
- Do not treat state resets as candidate decoder behavior; they are
  localization probes and break immediate continuity.
- Do not treat component-window perturbations as candidate decoder behavior;
  they rank sensitivity but intentionally change the excitation trajectory.
- Do not treat `fixed_gain_half` as a candidate decoder gain rule; the latest
  audit shows it helps by long-term feedback damping, not by correcting an
  independently verified scalar formula.
- Next useful work is earlier prior-excitation/history localization before
  frame `117`, focused on the upstream excitation feedback path around frames
  `49..72` and `115..116`.

## Verification

```sh
env GOCACHE=/tmp/go-build go test ./internal/decoder -count=1
env GOCACHE=/tmp/go-build G729_DECODER_PITCH_CAP_ACTIVATION_AUDIT=1 go test ./internal/decoder -run TestDecoderPitchGainCapActivationAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_PITCH_CAP_TRIGGER_GRID_CROSS_VECTOR=1 go test ./internal/decoder -run TestDecoderPitchGainCapTriggerGridCrossVectorAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_GAIN_EC_Q25_CROSS_VECTOR=1 go test ./internal/decoder -run TestDecoderTAMEGainECQ25CrossVectorAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_GAIN_VARIANT_CROSS_VECTOR=1 go test ./internal/decoder -run TestDecoderTAMEGainVariantCrossVectorAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_ONSET_CANDIDATE_RANGE_AUDIT=1 go test ./internal/decoder -run TestDecoderTAMEOnsetCandidateRangeAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_STATE_CARRY_RESET_AUDIT=1 go test ./internal/decoder -run TestDecoderTAMEStateCarryResetAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_FEEDBACK_COMPONENT_WINDOW_AUDIT=1 go test ./internal/decoder -run TestDecoderTAMEFeedbackComponentWindowAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_FIXED_FEEDBACK_AUDIT=1 go test ./internal/decoder -run TestDecoderTAMEFixedFeedbackPropagationAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_EXCITATION_ORACLE_REPLAY=1 go test ./internal/decoder -run TestDecoderTAMEExcitationOracleReplay -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_ACB_ORACLE_SHAPE=1 G729_DECODER_HISTORY_START_SUBFRAME=52 G729_DECODER_HISTORY_END_SUBFRAME=240 G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half go test ./internal/decoder -run TestDecoderTAMEACBOracleShape -count=1 -v
env GOCACHE=/tmp/go-build G729_DECODER_TAME_PAST_EXC_SOURCE_BACKTRACE=1 go test ./internal/decoder -run TestDecoderTAMEPastExcSourceBacktraceAudit -count=1 -v
env GOCACHE=/tmp/go-build G729_COMPARE_DECODER_TAME_ONSET_FRAME26=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEOnsetFrame26 -count=1 -v
env GOCACHE=/tmp/go-build G729_COMPARE_DECODER_PITCH_INSTABILITY_DECISION=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderPitchInstabilityDecision -count=1 -v
env GOCACHE=/tmp/go-build G729_COMPARE_DECODER_TAME_ACB_CHECKPOINT=1 go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEACBCheckpoint -count=1 -v
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -count=1 -v
```
