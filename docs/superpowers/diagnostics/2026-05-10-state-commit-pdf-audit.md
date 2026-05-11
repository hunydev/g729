# G.729 Encoder State Commit PDF Audit

Date: 2026-05-10

Scope: clean-room audit of encoder state updates across subframes and frame
boundaries.

Sources used:

- ITU-T Recommendation G.729 (03/96), sections 3.9.1 and 3.10.
- ITU-T Recommendation G.729 Annex A (11/96), section A.3.10.

Clean-room boundary:

- Only the official PDF text was inspected.
- The ITU package `Software.zip` entries were not extracted or opened.
- No external G.729 implementation source was inspected.

## Audit Table

| State | PDF requirement | Current code | Classification |
| --- | --- | --- | --- |
| `oldExc` | After quantized gains, compute `u(n) = gp_hat*v(n) + gc_hat*c(n)` for the present subframe. | `encoder.fcbStep` shifts `oldExc` by 40 and appends `synth.BuildExcitation(...)`. | Aligned |
| `swMemErr` | Annex A updates weighted-synthesis filter state by computing `ew(n)=x(n)-gp_hat*y(n)-gc_hat*z(n)` for `n=30..39`. | `encoder.fcbStep` writes `e.swMemErr[0..9]` from samples 30..39 after gain quantization. | Aligned |
| `lpResidualMemQ` | Next subframe residual uses the preceding speech samples through the analysis filter memory. | `commitClosedLoopPitch` copies `sFrame[30:40]` after each subframe. | Aligned |
| `pastQuaEn` | Gain predictor FIFO advances with `U(m)=20*log10(gamma_hat)` after gain quantization. | `gainquant.UpdatePastQuaEn` is called once per subframe after the chosen gain pair is committed. | Aligned |
| `prevGpQ14` | Fixed-codebook harmonic enhancement uses previous subframe's quantized pitch gain. | `fcbStep` updates `prevGpQ14` after FCB/gain commit, so the next subframe observes it. | Aligned |
| `intT1` / `P1` | Subframe-2 pitch range and P2 packing use the integer part of subframe-1 T1. | `commitClosedLoopPitch` commits `intT1` before subframe 2, and `Subframe2Window(e.intT1)` is used for P2. | Aligned |
| LSP and frame speech buffers | LP/LSP analysis and quantized LP state advance once per 80-sample frame. | `lpcStep` advances `oldSpeech`, LSP predictor, and decoded `aHatSF1/aHatSF2` before the two subframes. | Aligned |

## Subframe Ordering

The frame driver calls:

```text
lpcStep
openloopStep
closedloopStep(0)
closedloopStep(1)
buildBitstreamFrame
```

This means subframe 2 sees subframe 1's freshly committed:

- `oldExc`;
- `swMemErr`;
- `lpResidualMemQ`;
- `pastQuaEn`;
- `prevGpQ14`;
- `intT1`.

That ordering is necessary for Annex A's analysis-by-synthesis loop and for
the gain-predictor MA state.

## Quality Heuristics Kept Out Of Core

`EncoderProfileQuality` may use decoder-in-loop gain/pitch repair and native
gain search. Those changes can alter the selected transmitted fields, but the
resulting state is still committed through the same per-subframe state owners.
They are not part of `EncoderProfileCore`.

## Finding

No state-commit timing mismatch was found in the current Core path.

The remaining Core near-clips are therefore not explained by delayed
subframe-1 state, missing `pastQuaEn` update, stale harmonic-enhancement
`prevGpQ14`, or frame-level instead of subframe-level `swMemErr` update.

The next useful clean-room surfaces are:

- gain quantization Q-format and reconstruction details not already covered
  by the gain-preselect audit;
- clean-room numeric oracle artifacts for the Annex A reduced-complexity FCB
  tree subset, if exact Core alignment is required.
