# Clean-Room Audit Record

This is an engineering audit record, not legal advice.

This repository is intended to be distributed as a clean-room, pure-Go,
MIT-licensed G.729A-compatible codec for `G729/8000 annexb=no` RTP send
paths. The project does not claim ITU certification, ITU endorsement, encoder
byte-exact conformance, Annex B support, G.729.1 support, or G.729D/E support.

## Clean-Room Claim

The implementation is maintained under a clean-room boundary:

- Codec source is written in Go for this repository.
- Public standards documents, public speech-coding references, black-box
  executable behavior, and numeric oracle outputs may be used.
- Existing G.729 implementation source code must not be read, copied,
  translated, or used as implementation structure.
- Private oracle data and third-party executable outputs are verification
  materials, not MIT-licensed project source.

The claim is limited to repository engineering practice and distributed source
contents. It is not a legal conclusion and is not a statement about patent
rights, certification, or standards-body approval.

## Forbidden Sources

Contributors and maintainers must not inspect, copy, translate, or derive code
structure from:

- ITU reference C source or related reference distribution source files.
- `bcg729` source code.
- FFmpeg G.729 decoder or encoder implementation source.
- Sipro Lab implementations.
- Asterisk or FreeSWITCH G.729 codec modules.
- Any other existing G.729 implementation source, whether open source,
  commercial, leaked, proprietary, or generated from one of the above.

This restriction includes comments, function boundaries, helper naming,
control-flow structure, table provenance, branch descriptions, and bug-for-bug
behavior from those implementations.

## Permitted Inputs

Permitted engineering inputs are limited to:

- Public ITU-T G.729 and Annex A specification text.
- Public speech-coding textbooks, papers, and general DSP references.
- Independently written tests and local implementation behavior.
- Public file-format knowledge for `.BIT`, `.PST`, RTP, WAV, and packet
  captures.
- Black-box execution of external tools or servers, without reading their G.729
  codec source.
- Numeric oracle artifacts containing only scalar values, deltas, controlled
  labels, and aggregate histograms.

Large official test vectors, private verifier output, customer/user samples,
and reference-decoder execution CSVs are not redistributed as part of this
repository.

## Black-Box and Oracle Boundary

External tools may be executed as black boxes for interoperability and quality
checks. The permitted boundary is:

- Provide audio, payload bytes, RTP packets, or bitstream files as input.
- Record decoded PCM, encoded payloads, pcaps, numeric metrics, pass/fail
  status, logs, and small controlled labels.
- Import only numeric values that are reviewed as oracle artifacts.

The prohibited boundary is:

- Reading or copying external G.729 implementation source.
- Importing source snippets, comments, helper names, branch descriptions, or
  implementation-specific narrative.
- Explaining a numeric value by pointing to a third-party implementation's
  source line.
- Committing large private oracle dumps, official ITU vector files, customer
  samples, or third-party codec binaries.

Numeric oracle artifacts are useful because they can identify arithmetic or
state mismatches without importing implementation source. They are still not
project source and should be kept narrowly scoped.

## Expected Standards-Based Similarity

Algorithmic similarity is expected in standards-based codecs. Independent
G.729A implementations will naturally share many properties because the public
specification defines the codec structure and bitstream:

- LPC, LSP/LSF conversion, interpolation, and stability handling.
- Adaptive-codebook and fixed-codebook search concepts.
- Gain quantization, gain prediction, and bit allocation.
- Frame and subframe sizes, packed bit fields, and RTP payload shape.
- Synthesis, postfiltering, high-pass filtering, and fixed-point arithmetic
  domains.
- Publicly specified constants, tables, equations, and clipping behavior.

Similarity in these areas is not, by itself, evidence of copying. It must be
evaluated against what the public specification requires or strongly implies.

## Meaningful Copying Concerns

The following similarities are meaningful concerns and should be investigated:

- Copied or lightly edited comments.
- Unusual branch structure not explained by the public specification.
- Non-spec magic constants or implementation-specific table values.
- Identical bugs, especially where the public spec allows multiple reasonable
  choices.
- Translated source structure, including matching helper decomposition,
  temporary-variable flow, or naming patterns from a forbidden source.
- Implementation-derived function names, labels, or source-file references in
  code, tests, or oracle artifacts.
- Large code-adjacent numeric blocks whose provenance is a third-party
  implementation rather than a public spec, independent derivation, or reviewed
  numeric oracle.

These concerns do not automatically prove copying, but they are actionable
engineering signals.

## Maintainer Response Process

When a similarity claim is reported, maintainers should:

1. Acknowledge the report as an engineering review item, not a legal
   determination.
2. Confirm that the report includes concrete source and local file/line ranges.
3. Classify the claimed similarity as specification-driven, independently
   derived, numeric-oracle-derived, or potentially implementation-derived.
4. Avoid fetching or inspecting forbidden implementation repositories while
   triaging the report.
5. If the report is credible, isolate the affected code path and either rewrite
   from public specification materials or remove the disputed material.
6. Update tests, provenance notes, and release checklists as needed.
7. Keep public responses factual and limited to repository evidence.

Maintainers should not make legal conclusions in issues, pull requests, or
release notes.

## Reporter Requirements

Similarity reports must include enough detail to be actionable:

- Exact source project name.
- Source file path.
- Source function or symbol name, if applicable.
- Source line range.
- Corresponding repository file path.
- Corresponding repository line range.
- Type of similarity being claimed.
- Explanation of why the similarity is not explained by the public G.729 or
  G.729A specification.

Vague statements such as "this looks like another codec" or "the algorithm is
the same" are not actionable without file/line evidence and a specification
comparison.

Use the IP similarity issue template when possible:
[.github/ISSUE_TEMPLATE/ip_similarity_claim.yml](.github/ISSUE_TEMPLATE/ip_similarity_claim.yml).
