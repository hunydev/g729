package fcb

// placePulses zeros c[] then writes ±PulseAmplitude at each of the
// four pulse positions, where the sign of pulse i is taken from bit
// (3-i) of the signs field per ITU-T G.729 §3.8 equation (45):
//
//	sign_bit_i = 1  →  pulse amplitude = +PulseAmplitude
//	sign_bit_i = 0  →  pulse amplitude = −PulseAmplitude
//
// (The exact bit→sign mapping is an encoder convention not pinned
// by the spec; bit-exact verification against ITU vectors happens
// in Phase 1g.)
//
// Clearing c[] first is a contract: the downstream pitch enhancement
// filter assumes c is the canonical 4-pulse codebook vector, zero
// everywhere else.
func placePulses(positions [4]int, signs uint8, c *[40]int16) {
	for i := range c {
		c[i] = 0
	}
	for i := 0; i < 4; i++ {
		if (signs>>(3-uint(i)))&1 == 1 {
			c[positions[i]] = PulseAmplitude
		} else {
			c[positions[i]] = -PulseAmplitude
		}
	}
}
