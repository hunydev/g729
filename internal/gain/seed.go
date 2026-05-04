package gain

// SeedDecoder primes a gain.Decoder with the supplied 4-tap MA-predictor
// past-error history. Used by encoder-side cross-validation tests
// (internal/gainquant.TestApply_MantissaExponent) that exercise the spec
// equivalence "encoder Apply == decoder Decode for matched indices" by
// running both sides from a known shared predictor state.
//
// After SeedDecoder, the very next Decode call skips the
// pastErrorsDefault initialization that fires on the zero-value Decoder,
// preserving the seeded taps.
func SeedDecoder(d *Decoder, past [4]int16) {
	d.pastErrors = past
	d.initialized = true
}
