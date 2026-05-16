# Contributing

This repository accepts contributions under a clean-room policy. This document
is an engineering contribution policy, not legal advice.

## Clean-Room Contribution Policy

Do not copy, translate, or derive code structure from any existing G.729
implementation. Forbidden sources include:

- ITU reference C source or related reference distribution source files.
- `bcg729` source code.
- FFmpeg G.729 decoder or encoder implementation source.
- Sipro Lab implementations.
- Asterisk or FreeSWITCH G.729 codec modules.
- Any other existing G.729 implementation source.

The restriction covers comments, helper names, function boundaries,
control-flow structure, tables, branch descriptions, test narratives, and
magic-number provenance. Do not add third-party codec source, third-party codec
binaries, official ITU vector files, private oracle dumps, or customer/user
audio samples to the repository.

Allowed references include public standards text, public speech-coding
textbooks and papers, independently written tests, local implementation
behavior, black-box executable behavior, and narrowly reviewed numeric oracle
artifacts.

## Pull Request Provenance Notes

Pull requests that change algorithmic codec behavior should include a short
provenance note describing:

- Public specification sections or public references used.
- Whether any black-box executable output or numeric oracle artifact was used.
- Confirmation that no forbidden G.729 implementation source was inspected.
- Any new numeric fixtures, their source boundary, and whether they are safe to
  redistribute.
- What project claim is affected, if any.

Example:

```text
Provenance: Implemented from public G.729 Annex A text and local tests. No
forbidden implementation source inspected. No new oracle data added.
```

## Development Checks

Before submitting a pull request, run the relevant local checks:

```sh
go test ./...
```

For release or provenance-sensitive work, also run:

```sh
go list -deps ./...
go mod graph
```

If private oracle data or black-box tools are used locally, keep them outside
the repository and document only aggregate results or reviewed numeric
artifacts.

## Similarity Concerns

If you believe this repository contains code that is too similar to another
implementation, open an IP similarity issue with exact source and local
file/line comparisons. See [CLEANROOM_AUDIT.md](CLEANROOM_AUDIT.md) and
[docs/similarity-review.md](docs/similarity-review.md).
