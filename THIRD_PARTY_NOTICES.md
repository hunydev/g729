# Third-Party Notices

This repository is distributed as MIT-licensed source code for
`github.com/hunydev/g729`.

See [CLEANROOM_AUDIT.md](CLEANROOM_AUDIT.md) and
[IP_PROVENANCE.md](IP_PROVENANCE.md) for the clean-room boundary and
provenance record.

## Distributed Source

The distributed codec library source uses:

- Go standard library only.
- No vendored third-party source code.
- No third-party G.729 implementation source code.
- No generated code copied from a third-party G.729 implementation.

The repository test suite, examples, and development tools also import
`github.com/pion/rtp` to marshal generic RTP packets for `cmd/g729rtpcheck`
interoperability fixtures, the `examples/rtp_pion_packetize` RTP header
example, and the `cmd/g729rtpfixture` pcap generator. Pion RTP is a generic RTP
packet library, not a G.729 codec implementation, and it is used only as a
non-runtime test/example/tool dependency. Pion RTP and its `pion/randutil`
dependency are MIT-licensed and are not vendored in this repository.

## Distributed Documentation and Demo Assets

The GitHub Pages documentation under `docs/` includes a browser demo and
small listening samples:

- `docs/assets/wasm/wasm_exec.js` is copied from the Go toolchain. It carries
  the Go Authors copyright header and is governed by the BSD-style Go license:
  <https://go.dev/LICENSE>.
- `docs/assets/wasm/g729.wasm` is built from this repository's Go source with
  the Go toolchain.
- `docs/assets/audio/source-8k-16bit.wav` is an owner-provided 8 kHz mono
  signed 16-bit PCM WAV sample downloaded from
  `https://download.huny.dev/d/./8k_16bit.wav` for this project page.
- `docs/assets/audio/g729-encode.g729`,
  `docs/assets/audio/g729-encode-g729-decode.wav`, and
  `docs/assets/audio/g729-encode-ffmpeg-decode.wav` are generated derivatives
  of that owner-provided sample.
- `docs/assets/audio/bcg729-encode.g729`,
  `docs/assets/audio/bcg729-encode-g729-decode.wav`, and
  `docs/assets/audio/bcg729-encode-ffmpeg-decode.wav` are generated
  black-box-comparison derivatives of that owner-provided sample.
- `docs/assets/audio/arena/source-osr-us-0010-8k.wav` is
  `OSR_us_000_0010_8k.wav` from the Open Speech Repository, American English
  Harvard sentences, 16-bit PCM at 8 kHz. Source page:
  <https://www.voiptroubleshooter.com/open_speech/american.html>. The provider
  requires identifying the source of the speech materials as "Open Speech
  Repository".
- `docs/assets/audio/arena/trial-XX-bcg729-ffmpeg.wav` files are generated
  black-box-comparison derivatives of the Open Speech Repository sample.
- `docs/assets/audio/arena/trial-XX-our-loopback.wav` files are generated
  local-codec loopback derivatives of the Open Speech Repository sample.

The FFmpeg executable was used only as a black-box converter/decoder while
generating WAV derivatives. A local `bcg729` executable was used only as a
black-box encoder while generating the comparison payload. FFmpeg source code,
FFmpeg binaries, `bcg729` source code, and `bcg729` binaries are not
redistributed in this repository.

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
- Private verifier output directories such as
  `/home/exedev/g729_untracked/verifier-output/` — external conformance
  oracle outputs, including decoder final PCM oracle CSVs.
- `testdata/external/*.g729` and `testdata/external/user_quality_input.*` —
  user, customer, or external system audio/payload samples.
- Local build, transfer, and agent artifacts.

Only small prompts, schemas, diagnostic tables, or narrowly reviewed numeric
fixtures may be tracked when they satisfy the clean-room oracle policy in
[IP_PROVENANCE.md](IP_PROVENANCE.md). Large external conformance vectors and
private verifier outputs are not redistributed.

## External Trademarks and Names

Names such as ITU, G.729, FFmpeg, Asterisk, FreeSWITCH, Sangoma, Sipro, and
bcg729 may appear in documentation to describe interoperability targets,
forbidden sources, or black-box verification tools. No endorsement is implied.

## Compliance Summary

For the distributed repository:

- Project license: MIT.
- Runtime third-party code for the Go codec library: none.
- Non-runtime Go test/example/tool dependency: Pion RTP, used only for generic
  RTP packet fixture generation, example RTP header marshaling, and pcap
  fixture generation.
- Documentation demo third-party helper: Go `wasm_exec.js` from the Go
  toolchain, under the Go BSD-style license.
- Vendored codec code: none.
- Redistributed ITU reference source: none.
- Redistributed external G.729 implementation source: none.
- Redistributed private decoder exact oracle data: none.
- Redistributed speech samples: only the owner-provided public Pages demo
  sample, the Open Speech Repository arena sample, and generated derivatives
  listed above.
