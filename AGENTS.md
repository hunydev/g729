# Repository Instructions

You are running in an exe.dev VM.

https://exe.dev/docs/proxy.md has details about the exe.dev HTTPS proxy.

Only use documented exe.dev features (see https://exe.dev/docs.md). Undocumented local endpoints are internal infrastructure: unstable and unsupported.

## Clean-room G.729 Oracle Rule

This repository is developed under a clean-room boundary. Do not inspect ITU reference C, bcg729, FFmpeg, Sipro Lab, or other G.729 implementation code.

Verifier output may enter this repository only as numeric oracle artifacts. Allowed artifact content is limited to vector/frame/subframe/field scalar values, deltas, controlled notes, and aggregate histograms. Do not add implementation-derived names, code snippets, branch descriptions, or magic-number provenance to artifacts.
