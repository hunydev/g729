# Operational Hardening

This document records the operational checks behind the current
`G729/8000 annexb=no` release line. It is engineering evidence, not ITU
certification, ITU endorsement, or a claim to support every G.729 family
variant.

## Scope

The supported production profile is intentionally narrow:

- RTP media type: `G729/8000`
- Payload type: static RTP payload type 18, unless a deployment remaps it
- Packetization: `ptime=10` or `ptime=20`
- Frame shape: 10-byte G.729 Annex A speech frames
- SDP: advertise `a=fmtp:18 annexb=no`

The project does not implement Annex B SID/CNG/DTX, G.729.1, G.729D, or
G.729E.

The detailed Annex B / `annexb=yes` policy is recorded in
[`annex-b.md`](annex-b.md).

## Payload Boundary

The decoder accepts one 10-byte speech frame per `DecodeFrame` call. It rejects
the 2-byte RTP Annex B SID/CNG frame shape with `ErrUnsupportedAnnexB`, so a
misconfigured `annexb=yes` peer does not get decoded as speech.

Malformed payload handling is deliberately conservative:

- `len(bits) == 10`: decode as one speech frame.
- `len(bits) == 2`: return `ErrUnsupportedAnnexB`.
- Any other frame length: return `ErrShortBitstream`.
- Output buffers shorter than 80 samples return `ErrShortOutput`.

`cmd/g729rtpcheck` applies the same boundary at packet level. It accepts only
10-byte or 20-byte speech payloads for `ptime=10` and `ptime=20`, and rejects
SID/CNG-like payloads.

## RTP / SDP Checks

Use `cmd/g729rtpcheck` to validate raw payload streams or Ethernet IPv4 UDP RTP
pcaps:

```sh
go run ./cmd/g729rtpcheck -mode=payload -ptime=10 -in output.g729
go run ./cmd/g729rtpcheck -mode=payload -ptime=20 -in output.g729
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -in capture.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in capture.pcap
```

Use `cmd/g729rtpfixture` to generate small Pion RTP based pcaps for repeatable
tooling smoke tests:

```sh
go run ./cmd/g729rtpfixture -ptime=20 -packets=4 -multi-ssrc -wrong-pt -out /tmp/g729-fixture.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in /tmp/g729-fixture.pcap

go run ./cmd/g729rtpfixture -sid -out /tmp/g729-sid.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=0 -in /tmp/g729-sid.pcap
```

The `-sid` fixture is expected to fail today with `ErrUnsupportedAnnexB`; it is
kept as an explicit boundary test until an `annexb=yes` implementation exists.
See [`annex-b.md`](annex-b.md) for the required implementation scope.

The checker verifies payload length, packetized frame count, optional RTP
sequence/timestamp continuity, and optional decode traversal. The report also
counts matching SSRC streams, RTP marker packets, padding packets, extension
headers, and CSRC entries. Use `-json` for CI/release artifacts.

The `cmd/g729rtpcheck` tests include Pion RTP generated packets in addition to
hand-built packet fixtures. This keeps the RTP parser checked against a generic
Go RTP implementation while keeping the codec runtime free of RTP-library
dependencies. The Pion fixture matrix covers `ptime=20`, multi-SSRC streams,
payload-type skip behavior, RTP marker/padding/extension/CSRC counters, strict
timestamp discontinuity, and malformed padding/extension guards. The same
fixture path pins the current behavior for 2-byte Annex B SID/CNG payloads:
they are rejected with `ErrUnsupportedAnnexB` until an explicit `annexb=yes`
implementation exists.

## Encoder Quality Gate

The release-facing FFmpeg black-box gate compares:

- official `SPEECH.BIT -> ffmpeg`
- local product-default Core encoder output `-> ffmpeg`

The test can also write a machine-readable JSON report:

```sh
env GOCACHE=/tmp/go-build \
G729_FFMPEG_BLACKBOX_QUALITY=1 \
G729_REQUIRE_FFMPEG_BLACKBOX_QUALITY=1 \
G729_FFMPEG_BLACKBOX_REPORT=/tmp/g729-encoder-speech-ffmpeg.json \
go test -run TestExternalFFmpegBlackboxQuality_SPEECH -count=1 -v
```

This is a black-box interoperability and regression signal. It is not an
encoder byte-exact conformance claim.

## Real-Time Load Gates

Use `cmd/g729loadtest` for realtime smoke/soak runs. The release gate should
record `errors=0`, `codec_deadline_misses=0`, and a p99 processing deadline
ratio comfortably below `1.0`.

Single-core smoke commands:

```sh
env GOCACHE=/tmp/go-build go run ./cmd/g729loadtest \
  -mode=encode -profile=core -streams=1 -duration=5s -gomaxprocs=1 \
  -max-codec-deadline-miss-ratio=0 -max-p99-deadline-ratio=0.10 -json

env GOCACHE=/tmp/go-build go run ./cmd/g729loadtest \
  -mode=decode -streams=1 -duration=5s -gomaxprocs=1 \
  -max-codec-deadline-miss-ratio=0 -max-p99-deadline-ratio=0.05 -json

env GOCACHE=/tmp/go-build go run ./cmd/g729loadtest \
  -mode=loopback -profile=core -streams=1 -duration=5s -gomaxprocs=1 \
  -max-codec-deadline-miss-ratio=0 -max-p99-deadline-ratio=0.15 -json
```

Longer production soak runs should use the same commands with a longer
`-duration` and deployment-specific stream counts.

## Current v0.2.1 Smoke Snapshot

Environment:

```text
goos: linux
goarch: amd64
cpu: AMD EPYC 9554P 64-Core Processor
```

5-second realtime single-core smoke results:

| Mode | Streams | Errors | Codec misses | Codec RTF | Streams/core | p99 us | p99 deadline |
|---|---:|---:|---:|---:|---:|---:|---:|
| encode Core | 1 | 0 | 0 | 0.027898 | 35.85 | 334.20 | 0.033420 |
| decode | 1 | 0 | 0 | 0.003924 | 254.83 | 70.46 | 0.007046 |
| loopback Core | 1 | 0 | 0 | 0.031867 | 31.38 | 452.55 | 0.045255 |

RTP payload JSON smoke:

```text
cmd/g729rtpcheck -mode=payload -ptime=20 -json
mode=payload packets=1 frames=2 payloadBytes=20 decodedFrames=0 skipped=0 streams=1
```

Wake-late values are OS scheduling signals from the VM timer path, not codec
processing-time misses. The codec processing path stayed well inside the 10 ms
G.729 frame deadline in all three smoke runs.

## Release Interpretation

This hardening supports the following conservative statement:

> The project is suitable for `G729/8000 annexb=no` RTP speech-frame send paths
> when callers advertise `annexb=no`, reject or drop Annex B SID/CNG payloads,
> and size channel density from measured single-core RTF with deployment
> safety margin.

It does not justify claims of Annex B support, G.729.1 support, G.729D/E
support, ITU certification, ITU endorsement, or encoder byte-exact conformance.
