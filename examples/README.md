# Examples

Minimal example programs for `github.com/hunydev/g729`. Each example
uses only this module's public API and the Go standard library — no
external G.729 implementation source is consulted (clean-room I1
constraint).

## Programs

| Path | Purpose |
|---|---|
| `encode_pcm/` | Raw int16 LE 8 kHz mono PCM from stdin → packed 10-byte G.729 frames to stdout |
| `decode_g729/` | Packed 10-byte G.729 frames from stdin → raw int16 LE 8 kHz mono PCM to stdout |
| `streaming_encode/` | Same as `encode_pcm` but uses `NewStreamingEncoder` + `Write` + `Flush` (handles non-frame-aligned chunks) |
| `rtp_packetize/` | Illustrative RTP payload packetization (`-ptime=10` or `-ptime=20`); emits hex-dump lines (no real RTP header generation) |
| `../cmd/g729rtpcheck/` | Black-box raw payload / Ethernet IPv4 UDP RTP pcap validator for payload type 18 captures |

## Running

Each example is a standalone `main` package under the parent module:

```sh
# Encode a raw PCM file:
go run ./examples/encode_pcm < input.pcm > output.g729

# Decode a G.729 byte stream:
go run ./examples/decode_g729 < output.g729 > roundtrip.pcm

# Streaming encode (buffered, handles partial frames):
go run ./examples/streaming_encode < input.pcm > output.g729

# Illustrative RTP packetization (ptime=10 or ptime=20):
go run ./examples/rtp_packetize -ptime=10 < output.g729
go run ./examples/rtp_packetize -ptime=20 < output.g729

# Validate raw payload bytes or an RTP pcap:
go run ./cmd/g729rtpcheck -mode=payload -ptime=10 -in output.g729
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -in capture.pcap
go run ./cmd/g729rtpcheck -mode=pcap -pt=18 -ptime=20 -strict-ts -json -in capture.pcap
```

Build all example binaries:

```sh
go build ./examples/... ./cmd/g729rtpcheck
```

## SDP examples for RTP integration

`ptime=10`, single G.729 frame per RTP packet:

```
m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:10
a=maxptime:10
```

`ptime=20`, two G.729 frames bundled per RTP packet (20 bytes payload):

```
m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:20
a=maxptime:20
```

`annexb=no` MUST be advertised — this codec does not implement
Annex B SID / CNG / DTX. RTP Annex B SID/CNG frames are rejected with
`ErrUnsupportedAnnexB` rather than decoded as speech.

## Notes

- The `rtp_packetize` example is illustrative only. A production RTP
  sender (RFC 3550 / RFC 3551) handles RTP header construction,
  timestamp continuity, sequence numbers, SSRC, and jitter buffering;
  none of that belongs in this codec module.
- Each `g729.Encoder` and `g729.Decoder` instance is single-threaded.
  Concurrent calls on the same instance are a data race; create one
  instance per RTP stream / channel.
- `EncodeFrame` and `DecodeFrame` are zero-allocation in steady state
  (verified by hot-path benchmarks; see release verification log).
