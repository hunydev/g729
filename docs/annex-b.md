# Annex B Policy

This document records the project policy for G.729 Annex B, SID/CNG/DTX, and
`annexb=no` RTP deployments. It is an engineering scope document, not a claim
of Annex B support and not legal advice.

## Current Release Scope

The supported RTP profile is:

- RTP media type: `G729/8000`
- Payload type: static payload type 18 unless remapped by the application
- Packetization: `ptime=10` or `ptime=20`
- Payload shape: one or two 10-byte G.729 speech frames
- SDP: advertise `a=fmtp:18 annexb=no`

The project does not implement Annex B SID, CNG, DTX, VAD, or comfort-noise
state. It also does not implement G.729.1, G.729D, or G.729E.

## Runtime Boundary

The strict decoder accepts exactly one 10-byte speech frame per `DecodeFrame`
call. A 2-byte RTP Annex B SID/CNG-shaped payload is rejected with
`ErrUnsupportedAnnexB`.

The same boundary is applied by the RTP tooling:

- `cmd/g729rtpcheck` rejects 2-byte SID/CNG-like RTP payloads.
- `cmd/g729rtpfixture -sid` generates a SID/CNG-like pcap that is expected to
  fail through `cmd/g729rtpcheck`.
- The RTP web tool and SDP checker reject `annexb=yes` offers.

This behavior is intentional. A SID/CNG payload is not a malformed speech
frame; it belongs to an unsupported codec mode. Rejecting it explicitly avoids
silently decoding comfort-noise control data as speech.

## Caller Guidance

Applications using this package should:

- Advertise `a=fmtp:18 annexb=no` in SDP.
- Do not generate SID/CNG/DTX payloads from this encoder.
- Reject, drop, or renegotiate peers that send `annexb=yes` or 2-byte
  SID/CNG-like payloads.
- Keep one `Encoder` or `Decoder` instance per RTP stream/channel.
- Use `cmd/g729rtpcheck` and `cmd/g729rtpfixture` for integration smoke tests.

Minimal SDP shape:

```text
m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:20
a=maxptime:20
```

## Verification Gates

Current Annex B boundary checks are intentionally negative tests:

```sh
go test ./... -count=1
go test ./cmd/g729rtpcheck ./cmd/g729rtpfixture -count=1

go run ./cmd/g729rtpfixture -sid -out /tmp/g729-sid.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=0 -in /tmp/g729-sid.pcap
```

The final command should fail with `ErrUnsupportedAnnexB`. If it starts
passing before an explicit Annex B implementation exists, that is a regression.

Positive RTP speech-frame tooling checks remain separate:

```sh
go run ./cmd/g729rtpfixture -ptime=20 -packets=4 -multi-ssrc -wrong-pt -out /tmp/g729-fixture.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in /tmp/g729-fixture.pcap
```

## What Annex B Would Require

Adding `annexb=yes` is not a small payload-size switch. It should be treated as
a separate feature with its own design, tests, release notes, and claims.

Minimum implementation areas:

- VAD decision path for the encoder.
- DTX state machine for speech-to-silence transitions.
- SID frame generation and parsing.
- Comfort-noise parameter encoding/decoding.
- Comfort-noise synthesis on the decoder side.
- State interaction between SID/CNG frames, ordinary speech frames, packet
  loss concealment, and decoder memories.
- SDP negotiation behavior for `annexb=yes` and `annexb=no`.
- RTP packet validation for mixed speech and SID/CNG payloads.
- Quality and interoperability gates against black-box RTP peers.
- Clean-room oracle/vector strategy that does not import implementation source.

Until those pieces exist, accepting `annexb=yes` would overstate the supported
codec scope.

## Non-Claims

This project currently does not claim:

- Annex B support.
- SID support.
- CNG support.
- DTX support.
- `annexb=yes` interoperability.
- Any G.729.1, G.729D, or G.729E support.

The current claim remains bounded to `G729/8000 annexb=no` speech-frame RTP
send paths and the strict decoder validation described in
[`validation.md`](validation.md).
