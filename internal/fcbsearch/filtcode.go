package fcbsearch

// FilterCode computes the filtered fixed-codebook excitation z[0..39]
// per ITU-T G.729 §3.9 eq. 64 (G729E.txt §3.9, line ~1340):
//
//	z(n) = Σ_{i=0..n} c(i) · h(n − i)        n = 0,...,39
//
// This is the zero-state response of the weighted synthesis filter
// 1/Â(z/γ) (whose impulse response is h) to the algebraic excitation
// c. Like §3.7.3 eq. 44 for the adaptive vector y, the convolution
// is lower-triangular (causal, zero history per subframe boundary).
//
// Q-format. c is Q13 (CB-4 PulseAmplitude convention; ±1.0 → ±8192)
// and h is Q12 (HI-1 impulse-response convention). The product
// c(i)·h(n−i) accumulates in Q25 in an int32 acc; an arithmetic
// right-shift by 13 with Word16 saturation lands z in Q12 — the
// canonical scale used downstream by the §3.9.2 gain VQ search and
// the §A.3.10 eq. A.10 weighted-error commit (OQ-Q-FORMAT-A10
// resolution: z is in Q12).
//
// The convolution kernel mirrors closedloop.GpAndY's eq. 44 loop but
// is duplicated rather than refactored to a shared helper because
// the shift differs (13 vs 12) and the y/Gp coupling there is not
// shared here — per the merger doctrine, duplication is preferred to
// a parameterized helper for a 3-line kernel.
//
// I3 / I4: pure (writes only through z), zero allocation.
func FilterCode(c, h, z *[SubframeLen]int16) {
	for n := 0; n < SubframeLen; n++ {
		var acc int64
		for i := 0; i <= n; i++ {
			acc += int64(c[i]) * int64(h[n-i])
		}
		z[n] = saturateInt64ToInt16(roundShift64(acc, 13))
	}
}

func roundShift(v int32, shift uint) int32 {
	if shift == 0 {
		return v
	}
	add := int32(1 << (shift - 1))
	if v >= 0 {
		return (v + add) >> shift
	}
	return -(((-v) + add) >> shift)
}

func roundShift64(v int64, shift uint) int64 {
	if shift == 0 {
		return v
	}
	add := int64(1 << (shift - 1))
	if v >= 0 {
		return (v + add) >> shift
	}
	return -(((-v) + add) >> shift)
}

func saturateInt64ToInt16(v int64) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
