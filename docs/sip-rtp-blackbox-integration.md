# SIP/RTP Black-Box Integration Plan

This document defines the optional black-box SIP/RTP integration workflow for
`G729/8000 annexb=no` deployments. It is an engineering validation plan, not
ITU certification, ITU endorsement, legal advice, or a claim to support every
G.729 family variant.

The goal is to validate that this package interoperates with real SIP/RTP
media paths while preserving the repository clean-room boundary.

## Scope

The supported integration target is intentionally narrow:

- RTP media profile: `G729/8000`
- RTP payload type: static payload type 18 unless a deployment remaps it
- Packetization: `ptime=10` or `ptime=20`
- SDP: `a=rtpmap:18 G729/8000` and `a=fmtp:18 annexb=no`
- Payload shape: 10-byte G.729 Annex A speech frames

Annex B SID/CNG/DTX, `annexb=yes`, G.729.1, G.729D, and G.729E are outside the
current product claim. See [annex-b.md](annex-b.md).

## Non-Goals

- Do not claim ITU certification or endorsement.
- Do not claim encoder byte-exact conformance.
- Do not add a SIP stack to this codec package.
- Do not redistribute third-party SIP/RTP server binaries, codec modules, or
  private captures.
- Do not inspect, copy, translate, or summarize any third-party G.729
  implementation source.

## Clean-Room Boundary

Permitted black-box operations:

- Configure and run an external SIP/RTP peer using its documented administrator
  interface.
- Place calls, exchange SDP, send RTP, receive RTP, and capture packets.
- Feed captured G.729 payloads into this repository's tools.
- Record numeric results, decoded PCM, audio hashes, packet statistics, logs,
  SDP text, and pass/fail status.

Prohibited operations:

- Reading G.729 codec source from ITU reference C, `bcg729`, FFmpeg, Sipro,
  Asterisk, FreeSWITCH, Sangoma modules, or any other implementation.
- Copying implementation structure, comments, branch behavior, function names,
  or magic-number provenance from any external codec.
- Committing third-party G.729 binaries, codec modules, server packages, or
  private call captures.
- Treating a black-box peer's observed behavior as a license to copy its
  implementation.

Black-box evidence may enter this repository only as small reviewed summaries,
commands, schemas, numeric reports, or documentation. Raw pcaps, private audio,
third-party binaries, and large oracle outputs stay outside git.

## Test Topologies

Use whichever topology is available in the deployment lab:

| Topology | Purpose |
|---|---|
| local encoder -> RTP -> black-box SIP/RTP peer | Outbound send-path interoperability |
| black-box SIP/RTP peer -> RTP -> local decoder | Inbound payload and decoder interoperability |
| local encoder -> peer loopback/echo -> local decoder | End-to-end timing and packetization smoke |
| peer playback/MOH -> local capture | Receive-path payload shape and Annex B boundary checks |

The peer may be Asterisk, FreeSWITCH, a carrier SBC, a media gateway, or another
SIP/RTP endpoint, provided it is used only as a black box.

## Preconditions

- The peer is installed and licensed outside this repository if licensing is
  required.
- The peer is configured through documented runtime/admin configuration only.
- The call advertises `annexb=no`.
- Captures are stored outside the repository, for example under `/tmp` or a
  private lab artifact directory.
- The local commit hash, Go version, OS, peer product/version, and relevant SDP
  snippets are recorded with the result.

## Positive Capture Workflow

Capture RTP from a real call and validate packet shape:

```sh
tcpdump -i any -s 0 -w /tmp/g729-call.pcap udp
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in /tmp/g729-call.pcap
go run ./cmd/g729rtpreport -in /tmp/g729-call.pcap -pt=18 -ptime=20 -out /tmp/g729-call-report.json
```

Use `-ptime=10` when the call sends one speech frame per RTP packet. The
expected positive result is:

- payload type 18 packets are counted;
- payload bytes are multiples of 10 for the negotiated `ptime`;
- RTP sequence and timestamp continuity pass when `-strict-ts` is used;
- no 2-byte SID/CNG payload is accepted as speech;
- the reported stream count, marker count, skipped packets, and decoded frame
  count match the captured call shape.

If the deployment remaps payload type 18 dynamically, pass the negotiated
payload type with `-pt`.

`cmd/g729rtpreport` wraps the same pcap analysis with decoded PCM smoke
metrics and integration metadata fields. Use optional flags such as `-peer`,
`-peer-role`, `-topology`, `-sdp-offer`, `-sdp-answer`, and `-notes` to record
the black-box peer context without committing private captures.

## Local Fixture Workflow

Use the Pion RTP fixture generator for repeatable local tooling checks before
using a real SIP/RTP peer:

```sh
go run ./cmd/g729rtpfixture -ptime=20 -packets=4 -multi-ssrc -wrong-pt -out /tmp/g729-fixture.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in /tmp/g729-fixture.pcap
```

The fixture is not a full SIP call. It checks that the pcap parser, RTP header
handling, payload-type filtering, multi-SSRC accounting, and timestamp logic are
working before external integration begins.

The generated fixture can also exercise the report path:

```sh
go run ./cmd/g729rtpreport -in /tmp/g729-fixture.pcap -pt=18 -ptime=20 -out /tmp/g729-fixture-report.json
```

## Negative Boundary Workflow

Annex B is intentionally unsupported in the current release line. A 2-byte
SID/CNG-shaped payload must fail:

```sh
go run ./cmd/g729rtpfixture -sid -out /tmp/g729-sid.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=0 -in /tmp/g729-sid.pcap
```

The expected result is `ErrUnsupportedAnnexB`. If a real peer sends SID/CNG
payloads, the integration result is a configuration or scope failure for the
current `annexb=no` release line, not a decoder bug.

## Audio Quality Smoke

For outbound calls, record the peer-side audio if available and compare it
against the local source in a private lab artifact. For inbound calls, extract
matching RTP payloads and decode them with this repository's decoder.

Useful evidence:

- duration and sample-rate match;
- RMS and peak are in plausible range;
- no near-clip bursts are introduced by the codec path;
- local decode and any licensed/private quality scorer remain stable across
  repeated runs;
- blind listening does not reveal gross packetization, timestamp, or decode
  defects.

These checks are interoperability and regression evidence. They are not
standards-body certification.

## Evidence Record Template

Record this information for each integration run:

```text
date:
repository commit:
go version:
os/arch:
peer product/version:
peer role:
call topology:
SDP offer:
SDP answer:
ptime:
payload type:
annexb setting:
pcap path:
rtpcheck command:
rtpcheck JSON summary:
audio artifact paths:
audio metrics:
pass/fail:
notes:
```

Do not commit private pcaps, private audio, server binaries, codec modules, or
license files.

## Release-Gate Interpretation

The mandatory open-source release gates remain `go test ./...`, distribution
hygiene tests, RTP fixture/checker tests, and the documented encoder/decoder
quality gates.

The SIP/RTP black-box gate is optional and environment-dependent. Passing it
supports this conservative statement:

```text
The current commit was exercised against a black-box SIP/RTP peer for
G729/8000 annexb=no packetization and payload-shape interoperability.
```

It does not support stronger claims such as ITU certification, Annex B support,
or encoder byte-exact conformance.

## Future Automation

Future work may add an env-gated command or test that consumes externally
captured pcaps and enforces site-specific acceptance thresholds over the
`cmd/g729rtpreport` JSON output. Such automation must still keep pcaps and
private audio outside git.
