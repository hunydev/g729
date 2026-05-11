# G.729 Gain Preselect PDF Audit

Date: 2026-05-10

Scope: clean-room audit of the encoder gain-quantization preselect shape.

Sources used:

- ITU-T Recommendation G.729 (03/96), section 3.9.
- ITU-T Recommendation G.729 Annex A (11/96), section A.3.9.

Clean-room boundary:

- The official PDF text was inspected.
- The ITU ZIP entries named `Software.zip` were not extracted or opened.
- `bcg729`, FFmpeg, ITU reference C, Sipro, and other implementation source
  were not inspected.

## Finding

Annex A section A.3.9 delegates gain quantization to G.729 section 3.9.
G.729 section 3.9.2 defines the gain codebook search as:

- derive unquantized `gp` and `gc` optima from the weighted-error surface;
- preselect 4 of 8 first-stage vectors by closeness of the fixed-gain
  correction side;
- preselect 8 of 16 second-stage vectors by closeness of the pitch-gain side;
- run final exhaustive search only over the remaining 4x8 = 32 pairs.

Current implementation mapping:

- `internal/gainquant.SearchConjugate` keeps the Annex A/G.729 4x8 preselect
  shape.
- `internal/gainquant.SearchConjugatePreselectTargetBits` changes only the
  fixed-point precision of the unquantized preselect-center solve when used
  by `EncoderProfileCore`.
- `EncoderProfileQuality` uses `searchConjugateNativeGainWide`, which is a
  Quality heuristic: it evaluates all 128 standard gain-index pairs against
  the reconstructed-gain residual before decoder-in-loop repair.

## Classification

Promoting full 8x16 gain search into `EncoderProfileCore` is not currently a
spec-aligned fix. It would preserve the bitstream syntax, but it would no
longer follow the G.729 section 3.9.2 preselect procedure that Annex A points
to.

The Core residual near-clip evidence therefore remains classified as:

- not a broken final 4x8 cost-ordering bug;
- primarily a GA-axis preselect-breadth miss on the current problem sample;
- a valid target for Quality-profile exploration;
- not a Core-profile change unless a later PDF audit finds a different
  mismatch in the preselect-center solve, gain reconstruction, taming, or
  state commit timing.

## Numeric Context

Problem sample: `testdata/external/user_quality_audio.m4a`.

Core gain-preselect miss diagnostic:

```text
subframes=2968 selected==full 79.89%
fullInPreselect 79.89%
fullGAInTop4 81.27%
fullGBInTop8 97.71%
gaMissOnly 17.82%
gbMissOnly 1.38%
bothMiss 0.91%
meanSelectedRank 2.2
```

Focused clip-window examples:

```text
frame 292 sub0 selected 4/13 fullBest 2/11 fullIn false gaTop4 false gbTop8 true rank 7
frame 292 sub1 selected 7/13 fullBest 6/13 fullIn false gaTop4 false gbTop8 true rank 6
frame 293 sub0 selected 7/13 fullBest 2/8  fullIn false gaTop4 false gbTop8 false rank 7
frame 293 sub1 selected 7/13 fullBest 2/10 fullIn false gaTop4 false gbTop8 true rank 6
frame 294 sub0 selected 7/13 fullBest 6/13 fullIn false gaTop4 false gbTop8 true rank 5
frame 294 sub1 selected 4/13 fullBest 2/11 fullIn false gaTop4 false gbTop8 true rank 9
```

Conclusion: the observed full-search wins are mostly outside the PDF-defined
4x8 preselect set, especially on the GA axis. That explains why a post-hoc
gain patch can remove Core near-clips but does not justify a Core full-search
change.
