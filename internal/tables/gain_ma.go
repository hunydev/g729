package tables

// GainMAPredictor holds the 4 MA predictor coefficients b_1..b_4
// used to predict the log-gain correction factor per ITU-T G.729
// §3.9 eq. (69):
//
//	Ẽ(m) = Σ b_i · Û(m−i)    (i = 1..4)
//
// Spec values [b_1..b_4] = [0.68, 0.58, 0.34, 0.19] in Q13:
// {5571, 4751, 2785, 1556}. Transcribed from tab_ld8a.c pred[]
// initializer under the merger-doctrine exception.
var GainMAPredictor = [4]int16{5571, 4751, 2785, 1556}

// GainMeanEnergyQ10 is the mean log-energy constant E̅ per ITU-T
// G.729 §3.9 eq. (66)-(68). Spec gives E̅ = 30 dB; in Q10 this is
// 30 · 2^10 = 30720.
const GainMeanEnergyQ10 int16 = 30720
