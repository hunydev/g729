# Third-Party Notices

This repository is distributed as MIT-licensed source code for
`github.com/hunydev/g729`.

## Distributed Source

The distributed project source uses:

- Go standard library only.
- No vendored third-party source code.
- No third-party G.729 implementation source code.
- No generated code copied from a third-party G.729 implementation.

## Optional Development Tools

The following tools may be used during local development or release
verification, but they are not distributed as part of this repository:

- Go toolchain.
- FFmpeg executable, used only as a black-box converter/decoder in opt-in
  tests.
- Asterisk or FreeSWITCH servers, used only as black-box SIP/RTP peers in
  optional integration tests.
- Packet-capture tooling such as tcpdump or Wireshark.

These tools keep their own licenses. Their source code is not included here.

## Local Test Materials Not Redistributed

The following local materials are intentionally excluded from git:

- `testdata/itu/` — ITU test vectors.
- `docs/superpowers/specs/itu/` — ITU specification PDFs/text.
- `testdata/external/*.g729` and `testdata/external/user_quality_input.*` —
  user, customer, or external system audio/payload samples.
- Local build, transfer, and agent artifacts.

Only small documentation files and numeric oracle artifacts may be tracked
when they satisfy the clean-room oracle policy in [IP_PROVENANCE.md](IP_PROVENANCE.md).

## External Trademarks and Names

Names such as ITU, G.729, FFmpeg, Asterisk, FreeSWITCH, Sangoma, Sipro, and
bcg729 may appear in documentation to describe interoperability targets,
forbidden sources, or black-box verification tools. No endorsement is implied.

## Compliance Summary

For the distributed repository:

- Project license: MIT.
- Runtime third-party code: none.
- Vendored code: none.
- Redistributed ITU reference source: none.
- Redistributed external G.729 implementation source: none.
- Redistributed local speech samples: none.

