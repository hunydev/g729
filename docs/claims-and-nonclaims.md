# Claims and Non-Claims

This document keeps release wording aligned across the README, provenance
records, validation docs, and notices. It is an engineering communication
record, not legal advice.

## Project Claims

- `github.com/hunydev/g729` is a clean-room, pure-Go, MIT-licensed
  G.729A-compatible codec for `G729/8000 annexb=no` RTP send paths.
- The codec library uses Go source and the Go standard library; it does not
  require cgo, native codec libraries, or vendored G.729 implementation source.
- `NewEncoder()` and `NewStreamingEncoder()` use `EncoderProfileCore`, the
  product default.
- The encoder emits normal 10-byte G.729 frames for the supported Annex A
  send path and is quality-gated by project tests and diagnostics.
- The encoder is independent and quality-gated; it is not claimed to be
  byte-exact to the ITU reference encoder, `bcg729`, FFmpeg, or any other
  encoder.
- The strict decoder path matches private ITU Annex A reference PCM
  sample-for-sample in the current verifier run: `740800/740800` final PCM
  samples exact.
- RTP send-path tooling targets payload type 18, `ptime=10`, `ptime=20`, and
  `annexb=no`.
- RTP Annex B SID/CNG payloads are explicitly rejected at the decoder boundary;
  they are not decoded as speech or treated as Annex B support.
- Private oracle data, official ITU vectors, customer/user samples, and
  external G.729 implementation source or binaries are not redistributed as
  part of the MIT-licensed source distribution.

## Non-Claims

- No ITU certification is claimed.
- No ITU endorsement is claimed.
- No encoder byte-exact conformance is claimed.
- No support is claimed for Annex B SID/CNG/DTX or `annexb=yes`.
- No support is claimed for G.729.1.
- No support is claimed for G.729D or G.729E.
- No legal advice, patent clearance, or licensing opinion is provided.
- No third-party codec source or binary redistribution rights are claimed.
- Decoder exactness is an engineering validation result from a private oracle
  gate; it is not standards-body certification or third-party endorsement.

## Wording Guidance

Prefer:

```text
clean-room, pure-Go, MIT-licensed G.729A-compatible codec for G729/8000
annexb=no RTP send paths
```

Prefer for the encoder:

```text
independent, quality-gated, and not byte-exact to the ITU reference encoder or
bcg729
```

Prefer for the decoder:

```text
strict decoder path matches private ITU Annex A reference PCM
sample-for-sample in the current verifier run
```

Avoid:

- "ITU certified"
- "ITU approved"
- "reference compatible" without explaining the exact scope
- "full G.729 family support"
- "Annex B support"
- "byte-exact encoder"
