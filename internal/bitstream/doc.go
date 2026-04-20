// Package bitstream is the boundary between the G.729 codec's logical
// frame parameters and their wire representation.
//
// # Layers
//
// The package exposes three layers, each independently useful:
//
//  1. Frame struct — one G.729 frame as 15 named integer parameters
//     (L0..GB2) in the ITU-T transmission order.
//  2. Pack / Unpack — convert between Frame and the 10-byte packed
//     bitstream actually transmitted over RTP. Zero-allocation, safe
//     to call on every encoder / decoder frame.
//  3. G.192 file I/O — WriteG192Frame / ReadG192Frame / ReadG192File
//     read and write the ITU-T G.192 serial bitstream (.bit) format
//     used by the official G.729 test vectors. Not on the hot path;
//     allocates a small frame-size buffer per call.
//
// # Wire byte / bit ordering
//
// Within each byte, the most significant bit is transmitted first.
// Within each parameter, the most significant bit is transmitted
// first. Parameters appear in the order of the Frame struct fields
// (L0 first, GB2 last).
//
// # G.192 file format
//
// Each frame on disk is 82 little-endian 16-bit words: sync (0x6B21 or
// 0x6B20), length (= FrameBits = 80), then one word per source bit
// (0x0081 for 1, 0x007F for 0).
//
// # Parity
//
// The P0 bit is the XOR of the 6 most-significant bits of P1. Use
// Parity to compute or validate it.
//
// # References
//
//   - ITU-T G.729, section on parameter transmission (frame bit
//     allocations).
//   - ITU-T G.729 Annex A, reduced-complexity variant; bitstream
//     layout is identical.
//   - ITU-T G.191 STL, serial bitstream (G.192) format definition.
package bitstream
