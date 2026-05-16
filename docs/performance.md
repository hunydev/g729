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

## Load and Soak Test Command

`cmd/g729loadtest` is the release-facing smoke/soak driver. It creates one
independent codec instance per simulated stream and can pace each stream at one
10 ms frame per tick. Use it to check that the codec hot path does not miss the
G.729 frame deadline under the chosen single-core or multi-core load.
The current release hardening checklist and latest smoke snapshot are recorded
in [`operational-hardening.md`](operational-hardening.md).

Single-core encode soak:

```sh
go run ./cmd/g729loadtest \
  -mode=encode -profile=core -streams=1 -gomaxprocs=1 -duration=60s \
  -max-p99-deadline-ratio=1 -json
```

Single-core multi-stream capacity probe:

```sh
go run ./cmd/g729loadtest \
  -mode=encode -profile=core -streams=64 -gomaxprocs=1 -duration=60s \
  -max-p99-deadline-ratio=1 -json
```

Loopback stress, useful as a conservative upper-bound check:

```sh
go run ./cmd/g729loadtest \
  -mode=loopback -profile=core -streams=32 -gomaxprocs=1 -duration=60s \
  -max-p99-deadline-ratio=1 -json
```

The command reports:

- `codec_rtf`: summed codec processing time divided by summed media duration.
- `codec_streams_per_core`: `1 / codec_rtf`, a codec-only capacity estimate.
- `processing_us p99`: p99 per-frame codec processing time.
- `codec_deadline_misses`: frames whose codec processing time exceeded 10 ms.
- `p99_deadline`: p99 processing time divided by the 10 ms frame deadline.
- `wake_late_*`: scheduler/timer wake-up lateness in realtime mode.

Use `-json` when recording release or CI artifacts. Duration fields in the
JSON report are emitted as nanoseconds with companion microsecond stats for
the processing and wake-late distributions.

Treat wake lateness as an OS scheduling signal, not codec work. A loaded VM can
wake goroutines late even when codec processing is well below the 10 ms budget.
For release gating, prefer `p99_deadline < 1.0` and zero codec errors; record
max/outlier deadline misses separately rather than treating one VM scheduling
outlier as an automatic codec failure.

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
| EncodeFrame Core | 101083 | 101.1 | 0.01011 | 98.9x | 98.9 | 0 |
| EncodeFrame CoreFast | 92241 | 92.24 | 0.009224 | 108.4x | 108.4 | 0 |
| DecodeFrame | 11902 | 11.90 | 0.001190 | 840x | 840 | 0 |
| StreamingWrite800 Core | 1011006 per 10 frames | 101.1 | 0.01011 | 98.9x | 98.9 | 0 |
| StreamingWrite800 CoreFast | 923959 per 10 frames | 92.40 | 0.009240 | 108.2x | 108.2 | 0 |

Median per-frame processing-time distribution from the same five-run sample:

| Path | mean us | p50 us | p95 us | p99 us | p99 deadline | observed max us |
|---|---:|---:|---:|---:|---:|---:|
| EncodeFrame Core | 100.8 | 99.99 | 108.6 | 117.2 | 0.01172 | 147 |
| EncodeFrame CoreFast | 92.37 | 91.07 | 98.62 | 106.3 | 0.01063 | 183 |
| DecodeFrame | 11.83 | 11.67 | 11.72 | 18.35 | 0.001835 | 81.5 |
| EncodeFrame + DecodeFrame | 112.9 | 112.0 | 120.3 | 133.8 | 0.01338 | 688 |

The observed max column is sensitive to OS scheduling. The p99 deadline column
is the more useful codec-hot-path jitter signal: on this VM, p99 processing
time stays far below the 10 ms G.729 frame budget.

CoreFast is an opt-in encoder profile. On the same public Pages sample used by
the default quality regression, the measured local-decode metrics were close to
Core while improving encoder throughput:

```text
Core:     globalSNR=2.12 corr=0.6782 rms/ref=0.9506 peak=8493 nearClip=0
CoreFast: globalSNR=2.13 corr=0.6769 rms/ref=0.9417 peak=8376 nearClip=0
```

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
