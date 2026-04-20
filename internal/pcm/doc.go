// Package pcm implements the encoder pre-processing (140 Hz high-pass
// filter with 1/2 amplitude scaling folded in) and the decoder output
// scaling (×2 with int16 saturation) for the G.729 codec.
//
// Everything in the codec outside this package assumes its int16 input
// has been through PreProcessor.Process and its int16 output will go
// through ScaleUpSat. See ITU-T G.729 §3.1.1 (pre-processing) for the
// filter transfer function and §4.2 for the decoder output scaling.
package pcm
