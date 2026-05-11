# G.729 Fixed-Codebook Search PDF Audit

Date: 2026-05-10

Scope: clean-room audit of the Core fixed-codebook search path against the
PDF-visible parts of ITU-T G.729 section 3.8 and G.729 Annex A section A.3.8.

Sources used:

- ITU-T Recommendation G.729 (03/96), sections 3.8, 3.8.1, 3.8.2.
- ITU-T Recommendation G.729 Annex A (11/96), section A.3.8.

Clean-room boundary:

- Only the official PDF text was inspected.
- The `Software.zip` entries bundled in the ITU packages were not extracted
  or opened.
- No external G.729 implementation source was inspected.

## PDF-Visible Mapping

| Requirement | PDF source | Current code | Classification |
| --- | --- | --- | --- |
| Four-pulse ISPP track structure, including the 16-position fourth track. | G.729 Table 7 | `internal/fcbsearch/search.go` track tables | Aligned |
| Build sparse code vector from four signed unit pulses. | G.729 eq. 45 | `fcbsearch.BuildSparseCode` | Aligned |
| Pitch enhancement uses `P(z)=1/(1-beta z^-T)` with `beta` from previous quantized pitch gain clamped to 0.2..0.8. | G.729 eqs. 46-48 | `fcb.ClampPitchGainForEnhancement`, `fcb.ApplyPitchEnhancement`, `fcbsearch.BuildCode` | Aligned |
| Search incorporates pitch enhancement by modifying impulse response `h`. | G.729 eq. 49 | `encoder.fcbStep` applies enhancement to `hSearch` before `CorrelationD` and `PhiPrime` | Aligned |
| Adjust target by subtracting adaptive contribution. | G.729 eq. 50 | `fcbsearch.AdjustedTarget` | Aligned |
| Compute `d(n)` as backward correlation of adjusted target and impulse response. | G.729 eq. 52 | `fcbsearch.CorrelationD` | Aligned |
| Precompute signed `phi'` matrix with sign folding and half-scaled diagonal. | G.729 eqs. 56-57 | `fcbsearch.SignsFromD`, `fcbsearch.PhiPrime` | Aligned |
| Pulse criterion is maximize `C^2/E`, with `C` from absolute correlations and `E/2` from signed `phi'`. | G.729 eqs. 53, 58-59 | `fcbsearch.SearchDepthFirst`, `SearchDepthFirstThresholdScanEntered` | Aligned to PDF-visible criterion |
| Pack signs and pulse positions into `S` and `C` fields. | G.729 eqs. 61-62 | `fcbsearch.PackS`, `fcbsearch.PackC` | Aligned |
| Filter final fixed-codebook vector through original impulse response for gain search. | G.729 eq. 64 | `fcbsearch.FilterCode` | Aligned |

## Annex A Tree-Search Limitation

Annex A section A.3.8 keeps the same 17-bit codebook structure, but says the
pulse positions are found with a more efficient iterative depth-first tree
search that tests a smaller fixed-complexity subset. The PDF text does not
fully specify that subset.

Because the implementation source bundled with the ITU package is outside the
clean-room boundary, the exact Annex A tree-search ordering and candidate
budget cannot be imported from source. The current Core path therefore uses
the PDF-visible G.729 focused-search threshold surface:

- `K3 = 0.4`;
- frame-level fourth-loop entry cap `180`;
- same `C^2/E` criterion and codeword construction.

This is treated as a clean-room approximation to the PDF-visible FCB search
surface, not as a byte-exact Annex A tree-search claim.

## Exhaustive Baseline Check

On `testdata/external/user_quality_audio.m4a`, the existing diagnostic
compares the current Core threshold scan with the same gain-preselect target
and an exhaustive FCB `C^2/E` search:

```text
norm24-core  GlobalSNR 5.21 dB SegSNR 4.37 dB Corr 0.8383 RMS/ref 0.9007 Peak 32768 NearClip 2
norm24-wide  GlobalSNR 5.06 dB SegSNR 4.33 dB Corr 0.8327 RMS/ref 0.9071 Peak 32767 NearClip 7
```

The exhaustive PDF-visible criterion is not a quality improvement for this
problem sample. It also does not resolve the remaining Core clips.

## Finding

No FCB equation-level mismatch was found in the PDF-visible parts of the
current Core path.

The unverified surface is the exact Annex A reduced-complexity tree-search
candidate subset. That gap cannot be closed by inspecting bundled ITU source
under the repository clean-room rule. Future work should treat this as either:

- a clean-room oracle-handoff problem using numeric candidate/rank artifacts
  only; or
- a Quality heuristic exploration area, if changing the candidate subset is
  driven by listening metrics rather than PDF evidence.
