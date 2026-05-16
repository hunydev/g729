# Clean-Room Release Checklist Template

This checklist is an engineering release aid, not legal advice.

## Release Identity

- Release version:
- Release date:
- Release commit hash:
- Reviewer:

## Claim Review

- [ ] README project summary matches [docs/claims-and-nonclaims.md](../claims-and-nonclaims.md).
- [ ] README does not imply ITU certification or ITU endorsement.
- [ ] README does not imply encoder byte-exact conformance.
- [ ] README does not imply Annex B, G.729.1, G.729D, or G.729E support.
- [ ] Validation wording separates decoder evidence, encoder evidence, and
      standards certification.
- [ ] Release notes use the same claims and non-claims.

## Provenance Documents

- [ ] [IP_PROVENANCE.md](../../IP_PROVENANCE.md) is current.
- [ ] [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) is current.
- [ ] [CLEANROOM_AUDIT.md](../../CLEANROOM_AUDIT.md) is current.
- [ ] [CONTRIBUTING.md](../../CONTRIBUTING.md) clean-room policy is current.
- [ ] [docs/similarity-review.md](../similarity-review.md) is current.
- [ ] No forbidden-source language has been weakened.

## Source Distribution Audit

- [ ] No ITU reference C source is tracked.
- [ ] No `bcg729` source or binary is tracked.
- [ ] No FFmpeg G.729 implementation source or binary is tracked.
- [ ] No Sipro implementation source or binary is tracked.
- [ ] No Asterisk or FreeSWITCH G.729 codec module source or binary is tracked.
- [ ] No other third-party G.729 implementation source or binary is tracked.
- [ ] No official ITU `.BIT`, `.PST`, specification PDFs, or reference
      distribution files are tracked.
- [ ] No private verifier output directory or large oracle CSV is tracked.
- [ ] No customer, user, or external-system audio/payload sample is tracked
      outside explicitly documented public demo assets.
- [ ] Only reviewed small prompts, schemas, diagnostics, or numeric fixtures
      are tracked under the clean-room oracle policy.

## Command Checks

Run from the release commit:

```sh
go test ./...
go list -deps ./...
go mod graph
```

If distribution/oracle handoff audit tests are present, run them:

```sh
go test ./... -run 'TestMITDistributionAudit|TestOracleHandoff_' -count=1
```

If private decoder oracle data is available, run the strict decoder final PCM
gate outside the public repository:

```sh
G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 \
G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1 \
G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/private/verifier-output \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v
```

## Final Notes

- Distribution audit result:
- Oracle/private validation result, if run:
- Known excluded private materials:
- Release approval:
