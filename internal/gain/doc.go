// Package gain implements the G.729 + Annex A decoder's conjugate-
// structure gain VQ: from 7 bits per subframe (GA 3 bits + GB 4 bits)
// and the Phase 1c fixed codebook vector c[40], decode the pitch gain
// g_p (Q14) and the fixed codebook gain g_c (Q12) consumed by the
// Phase 1e excitation sum u(n) = g_p·v(n) + g_c·c(n).
//
// # Public API
//
//	Indices{GA, GB uint8}
//	    Bit-field indices delivered by the bitstream unpacker
//	    (GA1/GB1 or GA2/GB2 from bitstream.Frame).
//
//	Decoder
//	    Per-instance struct holding the 4-tap MA predictor state
//	    for log-gain correction errors. Zero value is a valid
//	    initial state (pastErrors is populated lazily on the
//	    first Decode call with the spec's −14 dB Q10 default).
//
//	Decoder.Decode(idx, c) → (gpQ14, gcQ12 int16)
//	    Per ITU-T G.729 §3.9 / §4.1.6: compute E_c from c, form
//	    the MA-predicted log gain, look up the two-stage VQ,
//	    assemble g_c = γ̂_c · 10^((Ê − Ē_c)/20), and update the
//	    predictor state with U(m) = 20·log10(γ̂_c).
//
//	Decoder.Reset()
//	    Returns the Decoder to its zero-value state.
//
// # State ownership
//
// The MA predictor's tap line (past 4 log-gain correction errors) is
// the only state this package holds. It must NOT be shared across
// independent decoding sessions. Phase 1g will allocate one Decoder
// per active stream.
//
// # Numerical contract
//
//	Indices:       raw integers from the bitstream.
//	c (input):     Q13 int16 (from internal/fcb).
//	g_p (output):  Q14 int16 ∈ [0, ~1.2].
//	g_c (output):  Q12 int16 (chosen so typical magnitudes in (0, 8)
//	               map to non-zero int16; final Q-format alignment
//	               with Phase 1e excitation sum is settled in Phase 1g).
//	pastErrors:    Q10 dB, initialized lazily to −14 dB (= −14336).
//	Tables:        Q14/Q13 GBK1/GBK2, Q13 MAPredictor, Q14 Pow2Table,
//	               Q15 Log2Table, Q10 mean energy.
//
// # Scratch-from-spec
//
// Algorithm derived solely from ITU-T G.729 §3.9 + §4.1.6 + Annex A
// §A.3.9 (decoder tables are unchanged from full G.729). Numerical
// tables (GBK1, GBK2, MA predictor + mean energy, Pow2Table, Log2Table)
// are transcribed from ITU's tab_ld8a.c data-array initializers under
// the merger-doctrine exception. No algorithmic ITU C source has been
// consulted. Every arithmetic step routes through internal/fixed
// primitives or explicit int32 widening with rounding/saturation.
//
// # Concurrency
//
// A Decoder is not safe for concurrent use. Each active stream
// requires its own Decoder instance. Individual methods do not spawn
// goroutines; all work is synchronous on the caller.
package gain
