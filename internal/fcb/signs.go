package fcb

// placePulses zeros c[] then writes ±PulseAmplitude at each of the
// four pulse positions, where the sign of pulse i is taken from bit i
// of the signs field per ITU-T G.729 §3.8.2 eq. (61):
//
//	S = s0 + 2*s1 + 4*s2 + 8*s3
//
//	sign_bit_i = 1  →  pulse amplitude = +PulseAmplitude
//	sign_bit_i = 0  →  pulse amplitude = −PulseAmplitude
//
// Clearing c[] first is a contract: the downstream pitch enhancement
// filter assumes c is the canonical 4-pulse codebook vector, zero
// everywhere else.
func placePulses(positions [4]int, signs uint8, c *[40]int16) {
	for i := range c {
		c[i] = 0
	}
	for i := 0; i < 4; i++ {
		if (signs>>uint(i))&1 == 1 {
			c[positions[i]] = PulseAmplitude
		} else {
			c[positions[i]] = -PulseAmplitude
		}
	}
}
