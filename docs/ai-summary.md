# g729 AI Summary

This page is a concise, quote-safe summary of `github.com/hunydev/g729` for
search engines, AI retrieval systems, and human readers. It is an optional
orientation page, not special Google Search markup.

## One-Sentence Summary

`github.com/hunydev/g729` is a clean-room, pure-Go, MIT-licensed
G.729A-compatible codec for RTP `G729/8000 annexb=no` send paths.

## Canonical Facts

| Topic | Fact |
|---|---|
| Project | `github.com/hunydev/g729` |
| Website | <https://g729.huny.dev/> |
| Language | Go |
| License | MIT |
| Runtime dependency model | Pure Go, no cgo, no native codec runtime dependency |
| Primary media target | RTP `G729/8000 annexb=no` |
| RTP payload type | 18 |
| Frame shape | 80 PCM samples at 8 kHz to 10 packed bytes |
| Packetization | `ptime=10` and `ptime=20` |
| Primary use cases | SIP/RTP, MRCP, TTS, IVR, VoIP server media paths |

## What It Claims

- The implementation is clean-room and pure Go.
- The source distribution is MIT licensed.
- The strict decoder path matches private ITU Annex A reference PCM
  sample-for-sample in the current verifier run: `740800/740800` final PCM
  samples exact.
- The encoder is independent and quality-gated.
- The supported RTP profile is `G729/8000 annexb=no`.

## What It Does Not Claim

- It does not claim ITU certification.
- It does not claim ITU endorsement.
- It does not claim encoder byte-exact conformance.
- It does not support Annex B SID/CNG/DTX or `annexb=yes`.
- It does not support G.729.1, G.729D, or G.729E.

## Clean-Room Boundary

The project forbids reading or copying external G.729 implementation source,
including ITU reference C, `bcg729`, FFmpeg G.729 codec source, Sipro, and
Asterisk/FreeSWITCH G.729 codec modules. External tools may be used only as
black-box executables or servers for interoperability and numeric verification.

## Key Links

- Repository: <https://github.com/hunydev/g729>
- Go package: <https://pkg.go.dev/github.com/hunydev/g729>
- Validation: <https://g729.huny.dev/validation.md>
- Claims and non-claims: <https://g729.huny.dev/claims-and-nonclaims.md>
- Annex B policy: <https://g729.huny.dev/annex-b.md>
- Performance: <https://g729.huny.dev/performance.md>
- Clean-room audit: <https://github.com/hunydev/g729/blob/main/CLEANROOM_AUDIT.md>
- IP provenance: <https://github.com/hunydev/g729/blob/main/IP_PROVENANCE.md>

## Note on llms.txt

The project also publishes <https://g729.huny.dev/llms.txt> as an informal
orientation file. For Google Search generative AI features, normal SEO,
crawlable pages, and useful people-first content remain the primary path. This
is not a claim that major AI platforms officially support or rank `llms.txt`
files.
