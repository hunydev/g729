package openloop

import "github.com/hunydev/g729/internal/fixed"

// lpResidual computes the LP residual r(n) per §A.3.3 eq. A.3
// (G729E.txt line 2080):
//
//	r(n) = s(n) + Σ_{i=1..10} â_i · s(n − i),  n = 0,...,79
//
// for one 80-sample frame. The 10-sample input history mem holds
// s(−10..−1) so that s(n − i) for n < i resolves to mem[10 + n − i].
//
// Q-format. s and r are int16 Q0; â is Q12 with the leading 1.0 at
// â[0] (= 4096). Products use the same LMult / LShl(3) / Round scaling
// as the synthesis filter so a Q12 coefficient of 4096 behaves as
// exact unity.
//
// I3 / I4: pure (writes only through r), zero allocation.
func lpResidual(s *[80]int16, aHat *[11]int16, mem *[10]int16, r *[80]int16) {
	for n := 0; n < 80; n++ {
		acc := fixed.LMult(s[n], aHat[0])
		for i := 1; i <= 10; i++ {
			var sni int16
			if n-i >= 0 {
				sni = s[n-i]
			} else {
				sni = mem[10+n-i]
			}
			acc = fixed.LMac(acc, aHat[i], sni)
		}
		r[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func lpResidualSubframe(s *[40]int16, aHat *[11]int16, mem *[10]int16, r *[40]int16) {
	for n := 0; n < 40; n++ {
		acc := fixed.LMult(s[n], aHat[0])
		for i := 1; i <= 10; i++ {
			var sni int16
			if n-i >= 0 {
				sni = s[n-i]
			} else {
				sni = mem[10+n-i]
			}
			acc = fixed.LMac(acc, aHat[i], sni)
		}
		r[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

// lowpassWeightedSpeech computes the low-pass filtered weighted
// speech sw(n) per §A.3.3 eq. A.2 (G729E.txt line 2074):
//
//	sw(n) = r(n) − Σ_{i=1..10} a'_i · sw(n − i),  n = 0,...,79
//
// for one 80-sample frame, with caller-owned 10-sample sw history
// mem (sw(−10..−1)). aPrime is the order-10 A'(z) produced by WS-1's
// combineWith07; per WS-1 the leading aPrime[0] = 4096 is excluded
// from the i ≥ 1 sum and the i = 1 tap carries the (1 − 0.7z⁻¹)
// scaling baked into combineWith07.
//
// Q-format. r, sw int16 Q0; aPrime Q12 (with the WS-1 hybrid tap-1
// convention). Products use the same Q12 scaling as lpResidual.
//
// I3 / I4: pure (writes only through sw), zero allocation.
func lowpassWeightedSpeech(r *[80]int16, aPrime *[11]int16, mem *[10]int16, sw *[80]int16) {
	for n := 0; n < 80; n++ {
		acc := fixed.LMult(r[n], aPrime[0])
		for i := 1; i <= 10; i++ {
			var swni int16
			if n-i >= 0 {
				swni = sw[n-i]
			} else {
				swni = mem[10+n-i]
			}
			acc = fixed.LMsu(acc, aPrime[i], swni)
		}
		sw[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

func lowpassWeightedSpeechSubframe(r *[40]int16, aPrime *[11]int16, mem *[10]int16, sw *[40]int16) {
	for n := 0; n < 40; n++ {
		acc := fixed.LMult(r[n], aPrime[0])
		for i := 1; i <= 10; i++ {
			var swni int16
			if n-i >= 0 {
				swni = sw[n-i]
			} else {
				swni = mem[10+n-i]
			}
			acc = fixed.LMsu(acc, aPrime[i], swni)
		}
		sw[n] = fixed.Round(fixed.LShl(acc, 3))
	}
}

// slideOldWspeech advances the encoder's 143-element sw history by
// one 80-sample frame, mirroring oldSpeech's slide-by-80 from
// Phase 2a (encoder.go:117–118). After the call:
//
//	old[0:63]   == previous old[80:143]   (retained tail)
//	old[63:143] == fresh[0:80]            (current-frame sw samples)
//
// Pinned by I-2b-2 and §A.3.4 max-lag k = 143.
//
// I3 / I4: pure (writes only through old), zero allocation.
func slideOldWspeech(old *[143]int16, fresh *[80]int16) {
	copy(old[0:63], old[80:143])
	copy(old[63:143], fresh[:])
}
