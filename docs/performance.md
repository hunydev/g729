# Real-Time Performance Metrics

This page documents the performance metrics used to justify real-time
streaming claims for `github.com/hunydev/g729`.

## Definitions

G.729 processes 10 ms frames: 80 samples at 8 kHz produce one 10-byte payload.
The real-time budget is therefore 10 ms per frame.

| Metric | Meaning |
|---|---|
| `rtf` | Real-time factor: codec processing wall time divided by media duration. `rtf < 1.0` means faster than real time. |
| `x-realtime` | Media duration divided by processing wall time. This is `1 / rtf`. |
| `streams/core` | Theoretical single-core stream count before application, RTP, scheduler, and safety-margin overhead. Same numeric value as `x-realtime`. |
| `p95-us`, `p99-us`, `max-us` | Per-frame codec wall time distribution. |
| `p99-deadline`, `max-deadline` | Per-frame time divided by the 10 ms frame deadline. Values below `1.0` stay inside the frame budget. |
| `p99-jitter-us`, `max-jitter-us` | Processing-time spread relative to p50. This is codec processing-time jitter, not network RTP jitter. |

## Method

The benchmark path is intentionally single-threaded:

- The benchmark sets `GOMAXPROCS(1)`.
- The benchmark locks the benchmark goroutine to one OS thread.
- One `Encoder` or `Decoder` instance is used per measured stream.
- The steady-state hot path is warmed up before timing.
- No RTP socket, filesystem, FFmpeg, PESQ, or network I/O is included.
- `StreamingWrite800` writes 10 frames at a time to `io.Discard`, so it
  measures the streaming encoder wrapper without sink backpressure.

This makes the result useful for estimating multi-channel deployments: divide
available CPU budget by the single-stream RTF, then apply an operational safety
margin for RTP packetization, jitter buffers, scheduling, and co-hosted work.

## Command

Run the single-thread real-time benchmarks:

```sh
GOMAXPROCS=1 go test -run=^$ \
  -bench='BenchmarkThroughput_|BenchmarkRealtimeJitter_' \
  -benchmem -count=5
```

For a release note, prefer reporting the median of several runs and include
the CPU/VM type. Performance numbers are hardware- and load-dependent.

## Current exe.dev VM Snapshot

Command:

```sh
GOMAXPROCS=1 go test -run=^$ \
  -bench='BenchmarkThroughput_|BenchmarkRealtimeJitter_' \
  -benchmem -count=5
```

Environment:

```text
goos: linux
goarch: amd64
cpu: AMD EPYC 9554P 64-Core Processor
```

Median throughput from the five-run sample:

| Path | ns/op | us/frame | RTF | x-realtime | Codec-only streams/core | Allocs |
|---|---:|---:|---:|---:|---:|---:|
| EncodeFrame | 100811 | 100.8 | 0.01008 | 99.2x | 99.2 | 0 |
| DecodeFrame | 11902 | 11.90 | 0.001190 | 840x | 840 | 0 |
| StreamingWrite800 | 1008700 per 10 frames | 100.9 | 0.01009 | 99.1x | 99.1 | 0 |

Median per-frame processing-time distribution from the same five-run sample:

| Path | mean us | p50 us | p95 us | p99 us | p99 deadline | observed max us |
|---|---:|---:|---:|---:|---:|---:|
| EncodeFrame | 100.8 | 99.83 | 108.0 | 118.2 | 0.01183 | 206 |
| DecodeFrame | 11.83 | 11.67 | 11.72 | 18.35 | 0.001835 | 81.5 |
| EncodeFrame + DecodeFrame | 112.9 | 112.0 | 120.3 | 133.8 | 0.01338 | 688 |

The observed max column is sensitive to OS scheduling. The p99 deadline column
is the more useful codec-hot-path jitter signal: on this VM, p99 processing
time stays far below the 10 ms G.729 frame budget.

## Interpretation

For a single 10 ms G.729 stream:

- `rtf < 1.0` means the codec can keep up with real time.
- `rtf = 0.10` means one stream consumes roughly 10% of one core for the codec
  operation being measured.
- `streams/core = 10` is the theoretical codec-only upper bound for that
  operation on one core.
- `p99-deadline < 1.0` means at least 99% of measured codec frames finish
  before the 10 ms media deadline.
- `max-deadline` can show operating-system scheduling outliers. Treat it as a
  diagnostic signal, not as a network jitter measurement.

Release-facing claims should use conservative wording such as:

> On the measured single-thread VM, the codec hot path runs below real-time
> RTF with zero steady-state allocations. Multi-channel capacity should be
> sized from measured `streams/core` with deployment-specific safety margin.

Do not present these numbers as a guarantee for all hardware or all runtime
loads.
