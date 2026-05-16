# Similarity Review Guide

This guide explains how maintainers evaluate source-code similarity claims for
this clean-room G.729A-compatible implementation. It is an engineering guide,
not legal advice.

## Expected Similarities

G.729A is a standards-based codec. Independent implementations are expected to
share codec structure because the public specification defines the algorithm
and bitstream. Expected similarities include:

- 10 ms frames, 80 sample subunits, and 10-byte packed payloads.
- LPC analysis, LSP/LSF conversion, interpolation, and stability checks.
- Adaptive-codebook and fixed-codebook concepts.
- Gain prediction, gain quantization, and bit allocation.
- Synthesis, postfiltering, high-pass filtering, and clipping behavior.
- Constants, tables, equations, and field widths that are present in public
  specification materials or independently reviewed numeric fixtures.
- Similar test cases for standard bitstreams, edge cases, and RTP payload
  shape.

These similarities do not by themselves indicate copying. They are often the
expected result of implementing the same public standard.

## Concerning Similarities

The following are more concerning and should be reviewed carefully:

- Copied comments or comments with the same unusual wording.
- Matching helper decomposition, temporary-variable flow, or branch structure
  that is not required by the public specification.
- Non-spec magic constants with the same unusual values and placement.
- Identical bugs or edge-case behavior where the public spec allows multiple
  reasonable choices.
- Translated code structure from C or another language into Go.
- External implementation function names, file names, labels, or source-line
  references embedded in code, tests, or oracle artifacts.
- Large code-adjacent tables whose provenance is not a public spec,
  independent derivation, or reviewed numeric oracle.

Concerning similarity is not a legal conclusion. It is a reason to isolate the
area, review provenance, and rewrite from public materials if needed.

## Review Process

Maintainers evaluate reports using this process:

1. Confirm the report includes exact source project, source file path, source
   function or symbol, source line range, local file path, and local line
   range.
2. Identify whether the similarity is required or strongly implied by public
   G.729/G.729A specification text.
3. Check repository provenance notes, tests, and numeric oracle boundaries.
4. Avoid fetching or inspecting forbidden G.729 implementation source during
   triage.
5. If the concern is credible, freeze the affected area and rewrite from public
   specification materials or remove the disputed material.
6. Update tests, [CLEANROOM_AUDIT.md](../CLEANROOM_AUDIT.md), and release
   checklists as needed.

Vague claims without file/line comparisons may be closed as unactionable.

## Reporting a Claim

Use the IP similarity issue template and provide:

- Exact source project.
- Source file path.
- Source function or symbol name.
- Source line range.
- Corresponding repository file path.
- Corresponding repository line range.
- Type of similarity.
- Explanation of why the similarity is not explained by the public
  G.729/G.729A specification.

Do not paste large source files or proprietary material into an issue.
