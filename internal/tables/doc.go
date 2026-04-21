// Package tables centralizes the ITU-T G.729 + Annex A numerical
// constants (codebooks, VQ tables, windows, predictors) used by the
// codec's analysis and synthesis blocks.
//
// # Why a dedicated package
//
// The G.729 specification defines several thousand fixed numerical
// constants: LSP codebooks, ACELP gain-VQ tables, the LP analysis
// window, the lag window, MA predictor coefficients, and so on. These
// are shared between the encoder and decoder and between multiple
// sub-blocks. Keeping them in one place:
//
//   - avoids duplication when the same table is referenced by more
//     than one internal package;
//   - makes spec-to-code review auditable — every numerical constant
//     lives at one checkable location;
//   - separates "data" from "algorithm" so the sub-packages that
//     consume the tables stay small and focused.
//
// # Conventions
//
// All tables are exported (upper-case identifier) with a doc comment
// citing the ITU-T G.729 section, table number (where applicable),
// and Q-format. Values are transcribed directly from the ITU
// specification documents; no value is copied from or cross-checked
// against any other G.729 implementation. See the repository-level
// scratch-from-spec policy for the origin rule.
//
// # When to add a table
//
// Tables are added incrementally, as a consuming sub-package (LSP
// dequantization, ACELP decoder, gain VQ, etc.) reaches the point
// where it needs them. Never add a table to this package without a
// concrete consumer ready to use it (YAGNI). Each addition lands in
// the same commit as the first test or function that exercises it.
//
// # Naming
//
// Identifiers track the ITU spec's mathematical notation, translated
// to Go's CamelCase rules. Examples of intended future additions:
//
//   - LSPCodebookL1   (§3.2.4, L1 first-stage codebook)
//   - LSPCodebookL2   (§3.2.4, L2 second-stage low split)
//   - LSPCodebookL3   (§3.2.4, L3 second-stage high split)
//   - MAPredictorsLSP (§3.2.4, LSP MA predictor, 2 sets)
//   - LPAnalysisWindow(§3.2.1, windowing of the speech signal)
//   - LagWindow        (§3.2.2, autocorrelation lag window)
//   - ACELPGainCodebookGA (§3.9.2 / Annex A §A.3.9)
//   - ACELPGainCodebookGB (§3.9.2 / Annex A §A.3.9)
//
// The Q-format of each table is documented in the comment that
// precedes the declaration.
package tables
