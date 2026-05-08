# IP Provenance and Clean-Room Record

This document records the intellectual-property provenance used for the
MIT-licensed release of `github.com/hunydev/g729`.

This is an engineering record, not legal advice.

## Summary

`github.com/hunydev/g729` is an independently developed clean-room
implementation of a G.729A-compatible 8 kbit/s speech codec for
`G729/8000 annexb=no` RTP speech frames.

The repository is licensed under the MIT License. The distributed source
does not include ITU reference source code, bcg729, FFmpeg, Sipro Lab
implementation code, Asterisk/FreeSWITCH codec implementation code, or any
other third-party G.729 implementation source.

## License Position

- Project license: MIT.
- SPDX identifier: `MIT`.
- Copyright holder line: see [LICENSE](LICENSE).
- Runtime dependencies: Go standard library only.
- Vendored third-party source code: none.

The MIT License is an OSI-approved open source license:

- https://opensource.org/license/mit
- https://opensource.org/licenses

## Clean-Room Boundary

The implementation was developed under the repository clean-room rule:

- Do not inspect ITU reference C.
- Do not inspect bcg729.
- Do not inspect FFmpeg G.729 implementation source.
- Do not inspect Sipro Lab or other G.729 implementation source.
- Do not copy implementation-derived code, branch structure, comments,
  function names, or magic-number provenance from any external G.729
  implementation.

Allowed inputs were limited to:

- Public ITU-T G.729 and Annex A specification text.
- Public speech-coding textbooks and papers cited in project documents.
- Local implementation behavior.
- Public test-vector file formats and numeric vector contents where legally
  available to the developer, but not redistributed in this repository.
- Numeric oracle artifacts containing only scalar values, deltas, aggregate
  histograms, and controlled notes.
- Black-box executable/server behavior from tools such as FFmpeg, Asterisk,
  or FreeSWITCH, without reading their codec source code.

## Numeric Oracle Policy

Oracle handoff artifacts may enter the repository only as numbers and small
controlled labels. They must not contain:

- External implementation source snippets.
- External implementation function names or branch descriptions.
- External source-file locations.
- Magic-number provenance copied from an implementation.
- Narrative descriptions that reveal external implementation structure.

This keeps verifier output useful for debugging while avoiding source-code
contamination.

## Black-Box Verification

FFmpeg, Asterisk, FreeSWITCH, and similar tools may be used as black-box
decoders, encoders, RTP peers, or servers. The permitted operation is:

- execute the tool or server;
- feed it audio, RTP, or G.729 payloads;
- collect numeric results, decoded PCM, pcaps, logs, or pass/fail status.

The prohibited operation is reading, copying, or deriving implementation
logic from their G.729 source code.

## Public Demo Media

The GitHub Pages site under `docs/` redistributes a small owner-provided
speech sample and generated derivatives:

- `docs/assets/audio/source-8k-16bit.wav` — source WAV downloaded from
  `https://download.huny.dev/d/./8k_16bit.wav`.
- `docs/assets/audio/g729-encode.g729` — raw payload produced by this
  repository's encoder.
- `docs/assets/audio/g729-encode-g729-decode.wav` — local encoder payload
  decoded by this repository's decoder.
- `docs/assets/audio/g729-encode-ffmpeg-decode.wav` — local encoder payload
  decoded by FFmpeg as a black-box executable.

These files are documentation/demo assets, not oracle artifacts and not
conformance evidence. They do not contain external G.729 implementation source
or implementation-derived structure.

## Non-Redistributed Materials

The following materials may exist locally during development but are not
redistributed by this repository:

- ITU test vectors under `testdata/itu/`.
- ITU specification PDFs/text under `docs/superpowers/specs/itu/`.
- User, customer, or Asterisk-origin audio/payload samples under
  `testdata/external/`.
- Local build, transfer, or agent artifacts.

The public Pages demo sample listed above is intentionally excluded from this
non-redistributed bucket because it is owner-provided for publication.

The repository `.gitignore` excludes these paths, and
`TestMITDistributionAudit` checks that forbidden local materials are not
tracked by git.

## Patent and Standards Notes

Public reports state that most G.729 patent terms expired on January 1, 2017,
and remaining consortium patent rights were made available royalty-free from
that date. This project does not rely on that statement as legal advice; users
with compliance obligations should perform their own legal review.

Useful public references:

- https://lwn.net/Articles/713292/
- https://bugzilla.redhat.com/show_bug.cgi?id=1358293

This project does not claim ITU certification, ITU byte-exact conformance, or
endorsement by ITU, FFmpeg, Asterisk, FreeSWITCH, Sangoma, Sipro, or any other
third party.
