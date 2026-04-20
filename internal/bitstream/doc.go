// Package bitstream converts G.729 frame parameters to and from their
// canonical 80-bit wire representation, and reads/writes the ITU-T
// G.192 serial bitstream file format used by the official G.729 test
// vectors.
//
// # Wire format
//
// A G.729 frame is 80 bits, transmitted MSB-first within each byte.
// Parameters are concatenated in the order declared by Frame (see the
// Frame struct field order), each contributing its declared bit width
// MSB-first. Total: 10 bytes per frame.
//
// # G.192 file format
//
// Each frame in a .bit file is 82 little-endian 16-bit words: one sync
// word, one length word (= FrameBits), and 80 data words. Data words
// encode the bit value, not pack it: 0x0081 for 1, 0x007F for 0.
//
// # References
//
//   - ITU-T G.729 and G.729 Annex A, "Coding of speech at 8 kbit/s
//     using CS-ACELP", parameter transmission tables.
//   - ITU-T G.191 Software Tools Library, Section on serial bitstream
//     (G.192) format.
package bitstream
