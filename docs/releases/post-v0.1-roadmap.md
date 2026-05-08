# Post-v0.1 Productization Roadmap

This roadmap tracks work after the v0.1.0 release-candidate baseline.
It is informational; release gates remain defined in the rc checklist
and verification log.

## Current Baseline

- Default suite: expected PASS.
- Conformance suite: 0 expected failures under `-tags=conformance`.
- Diagnostic suite: 5 expected PSTdomain pins under `-tags=diagnostic`.
- Gain decoder diagnostics use native `(gcMantQ14, gcExp)` output;
  the former `internal/gain/legacy_gcq12.go` adapter has been removed.
- `cmd/g729rtpcheck` provides a stdlib-only raw payload and
  Ethernet/IPv4/UDP/RTP pcap validator for payload type 18 captures.
- Public API Reset contract is pinned by tests: codec state and
  streaming tail reset; streaming sink is preserved.

## v0.1.x Patch Scope

- Documentation, examples, and release-gate maintenance.
- `cmd/g729rtpcheck` improvements that stay tooling-only:
  better report formatting, fixture examples, and additional pcap
  link-layer support if needed.
- Decoder PSTdomain limitation disposition: keep as a permanent known
  limitation unless a new spec-derived clean-room diagnostic candidate
  appears.
- Throughput tracking through `BenchmarkThroughput_*`; no release
  should regress allocation behavior from 0 B/op, 0 allocs/op.

## v0.2 Candidate Scope

- Optional RTP packetization helper package if real integrations need
  a library API rather than the current example/tooling layer.
- Packet-loss handling / concealment API design. This must be a
  dedicated clean-room phase because it changes decoder state semantics.
- Decoder optimization after hotspot attribution is available in an
  environment with `go tool pprof` or equivalent approved tooling.
- Additional black-box interoperability scripts around SIP/MRCP stacks,
  using only captured RTP payloads and numeric observations.

## Large Dedicated Phases

These are not v0.1.x work:

- Annex B SID / CNG / DTX.
- G.729D and G.729E low/high bit-rate variants.
- G.729.1 wideband / scalable codec.
- ITU certification or byte-exact conformance claims.

Each item above requires its own clean-room plan, oracle/verifier
protocol, public API review, and release-gate expansion before any code
is accepted.

## Release Gate Additions

The post-v0.1 gate should include:

- `go test ./... -count=1`
- `go test -tags=conformance ./... -count=1`
- `go test -tags=diagnostic ./... -count=1` with exactly 4 expected
  PSTdomain pins unless a documented diagnostic cycle changes the
  inventory.
- `go test . -run=^$ -bench=BenchmarkThroughput -benchmem`
- `go test ./cmd/g729rtpcheck`
- `go build ./examples/... ./cmd/g729rtpcheck`

Clean-room boundary remains mandatory: do not inspect ITU reference C,
bcg729, FFmpeg, Sipro Lab, or other G.729 implementation code.
