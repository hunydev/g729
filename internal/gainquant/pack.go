package gainquant

import "github.com/exedev/g729/internal/tables"

// PackGains applies the §3.9.3 forward index permutation that converts
// the physical conjugate-codebook indices (ga, gb) returned by
// SearchConjugate (GQ-2) into the transmitted (GA, GB) indices placed
// in the bitstream. The permutation reduces the perceptual impact of
// single-bit channel errors by clustering bit-adjacent codes onto
// nearby codebook entries (see §3.9.3 narrative).
//
// The forward maps tables.GainMap1 and tables.GainMap2 are the
// inverses of the decoder-side tables.GainImap1 / GainImap2 already
// used by gain.Decoder (verified by TestGainMap{1,2}AndImap{1,2}-
// AreMutualInverses in internal/tables/gain_map_test.go).
//
// I3 / I4: pure, zero allocation.
func PackGains(ga, gb uint8) (ga3, gb4 uint8) {
	return tables.GainMap1[ga], tables.GainMap2[gb]
}
