# G.729 Annex A Closed-Loop Pitch PDF Audit

Date: 2026-05-10

Scope: clean-room audit of `EncoderProfileCore` closed-loop pitch search
against ITU-T G.729 section 3.7 and G.729 Annex A section A.3.7.

Sources used:

- ITU-T Recommendation G.729 (03/96), sections 3.7, 3.7.1, 3.7.2, 3.7.3.
- ITU-T Recommendation G.729 Annex A (11/96), section A.3.7.

Clean-room boundary:

- Only the official PDF text was inspected.
- The `Software.zip` entries bundled in the ITU packages were not extracted
  or opened.
- No external G.729 implementation source was inspected.

## Audit Table

| Requirement | PDF source | Current code | Classification |
| --- | --- | --- | --- |
| First-subframe centre is the open-loop pitch `Top`. | G.729 3.7, Annex A A.3.7 | `encoder.closedloopStep`, `centre = e.tOp` for subframe 0 | Aligned |
| Subframe-1 integer window is `Top-3`, clamped to 20..143, with 7 integer lags. | G.729 3.7 | `closedloop.Subframe1Window` | Aligned |
| Subframe-2 integer window is based on `int(T1)-5`, clamped to 20..143, with 10 integer lags. | G.729 3.7 | `closedloop.Subframe2Window` | Aligned |
| Annex A closed-loop search maximizes the numerator-only `RN(k)` instead of normalized `R(k)`. | Annex A A.3.7 | `closedloop.BackwardFilter` plus `closedloop.SearchInteger` | Aligned |
| Search-stage excitation is extended by copying the LP residual into `u(0..39)`. | G.729 3.7, Annex A A.3.7 | `encoder.closedLoopExcitationSearch` | Aligned |
| T1 fractional range includes the encodable boundary delays `19+1/3` and `84+2/3`. | G.729 3.7 and 3.7.2 | `closedloop.RefineFractionSubframe1` adds `(19,+1)` and `(85,-1)` at boundaries | Aligned |
| T2 fractional range spans `int(T1)-5-2/3` through `int(T1)+4+2/3`. | G.729 3.7 and 3.7.2 | `closedloop.RefineFractionSubframe2` adds `(tmin-1,+1)` and `(tmax+1,-1)` | Aligned |
| Final adaptive-codebook vector for synthesis/gain uses the decoder-reconstructable past excitation, not the residual-extension search buffer. | G.729 3.7.1 and Annex A A.3.10 | `encoder.adaptiveVectorForSynthesis` uses `pitch.AdaptiveCodebook` unless a diagnostic quality knob is enabled | Aligned for Core |
| Pitch gain `gp` and filtered adaptive vector `y` are computed after the selected fractional delay is committed to the synthesis vector. | G.729 3.7.3, 3.9 | `closedloop.GpAndY` inside `commitClosedLoopPitch` | Aligned |

## Quality Heuristics Kept Out Of Core

The following are intentionally not part of `EncoderProfileCore`:

- normalized adaptive pitch search;
- full-range pitch-centre rescue;
- pitch clip repair;
- residual-extension adaptive-vector synthesis.

They remain Quality/diagnostic surfaces because they change the Annex A
closed-loop pitch search or synthesis-vector policy while preserving
standard-compatible transmitted pitch fields.

## Finding

No closed-loop pitch PDF mismatch was found in the current Core path.

The fractional boundary handling is not a heuristic: it covers pitch-delay
codepoints exposed by the PDF-defined T1 and T2 fractional ranges. Removing
those boundary candidates would make the encoder less complete with respect
to the valid pitch-index search span.

The next likely Core audit surface is fixed-codebook search and state commit,
not closed-loop pitch windowing or fractional refinement.
