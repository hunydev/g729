# Post-v0.1 Productization Roadmap

This roadmap tracks work after the v0.1.0 release-candidate baseline.
It is informational; release gates remain defined in the rc checklist
and verification log.

## Current Baseline

- Default suite: expected PASS.
- Strict decoder final PCM oracle gate: private ITU Annex A verification
  reaches `740800/740800` exact final PCM samples when
  `G729_DECODER_REFERENCE_ORACLE_DIR` points at the private verifier output.
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
- Keep README and package documentation aligned with the decoder final-PCM
  exact result while preserving the no-ITU-certification and no encoder
  byte-exact claim boundaries.
- `cmd/g729rtpcheck` improvements that stay tooling-only:
  better report formatting, fixture examples, and additional pcap
  link-layer support if needed.
- Retire or relabel older decoder PSTdomain diagnostic notes where they are
  superseded by the private final-PCM oracle gate.
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
- ITU certification, ITU endorsement, and encoder byte-exact conformance
  claims.

Each item above requires its own clean-room plan, oracle/verifier
protocol, public API review, and release-gate expansion before any code
is accepted.

## Release Gate Additions

The post-v0.1 gate should include:

- `go test ./... -count=1`
- `G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1 G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/private/verifier-output go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v` when the private oracle is available
- `G729_PESQ_PYTHON=/path/to/python G729_EXTERNAL_SAMPLE_QUALITY=/path/to/private-sample G729_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 G729_REQUIRE_EXTERNAL_SAMPLE_ENCODER_CANDIDATE_PESQ=1 go test -run TestExternalSampleEncoderCandidatePESQDiagnostic -count=1 -v` when private listening samples, PESQ, FFmpeg, and the local black-box anchor are available
- `go test -tags=conformance ./... -count=1`
- `go test -tags=diagnostic ./... -count=1` with exactly 4 expected
  PSTdomain pins unless a documented diagnostic cycle changes the
  inventory.
- `go test . -run=^$ -bench=BenchmarkThroughput -benchmem`
- `go test ./cmd/g729rtpcheck`
- `go build ./examples/... ./cmd/g729rtpcheck`

Clean-room boundary remains mandatory: do not inspect ITU reference C,
bcg729, FFmpeg, Sipro Lab, or other G.729 implementation code.
