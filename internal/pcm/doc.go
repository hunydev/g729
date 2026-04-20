// Package pcm converts between raw int16 PCM samples and the scaled,
// DC-rejected samples used by the rest of the G.729 codec.
//
// # Encoder side: PreProcessor
//
// The encoder feeds each incoming 10 ms frame (80 int16 samples at
// 8 kHz) through PreProcessor.Process, which applies the ITU-T G.729
// section 3.1.1 input pre-processing:
//
//   - A second-order pole-zero high-pass filter with a cutoff of
//     ~140 Hz, removing DC and sub-audible rumble that would otherwise
//     corrupt LPC analysis.
//   - A 1/2 amplitude scaling, folded into the filter's numerator
//     coefficients. This reserves 6 dB of headroom so that downstream
//     fixed-point analysis does not overflow on loud inputs.
//
// The filter is causal and stateful: two previous input samples and
// two previous output values persist across calls, so a signal split
// into consecutive 80-sample frames produces the same result as
// processing the whole signal at once. Each encoding channel must own
// a dedicated PreProcessor; the type is not safe for concurrent use.
//
// # Decoder side: ScaleUpSat
//
// The decoder synthesizes samples in the same halved-amplitude domain
// the encoder used, and the final stage multiplies by 2 with int16
// saturation. ScaleUpSat performs this operation; it is stateless and
// safe for concurrent use on independent slices.
//
// # Fixed-point arithmetic
//
// All arithmetic goes through internal/fixed, the G.191 basic-ops
// primitives. The filter accumulator is Word32; coefficients live in
// Q13 (13 fractional bits). See coeffs.go for the exact integer
// constants and their derivation from the ITU-T real-valued
// specification.
//
// # Zero allocation
//
// PreProcessor.Process and ScaleUpSat both allocate nothing. Filter
// memory is part of the PreProcessor value; input and output buffers
// are caller-owned. This property is enforced by alloc_test.go.
package pcm
