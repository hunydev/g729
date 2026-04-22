package gain

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

func TestPredictedLogGain_AllZeroPastErrors(t *testing.T) {
	var d Decoder
	got := d.predictedLogGain()
	if got != tables.GainMeanEnergyQ10 {
		t.Errorf("predictedLogGain(pastErrors=0) = %d, want %d (= E̅)", got, tables.GainMeanEnergyQ10)
	}
}

func TestPredictedLogGain_KnownPastErrors(t *testing.T) {
	d := Decoder{pastErrors: [4]int16{1024, 1024, 1024, 1024}}
	got := d.predictedLogGain()
	const want = 32553
	if diff := got - want; diff > 4 || diff < -4 {
		t.Errorf("predictedLogGain = %d, want ≈%d (±4)", got, want)
	}
}

func TestPredictedLogGain_OnlyFirstTapContributes(t *testing.T) {
	d := Decoder{pastErrors: [4]int16{1024, 0, 0, 0}}
	got := d.predictedLogGain()
	const want = 31416
	if diff := got - want; diff > 4 || diff < -4 {
		t.Errorf("predictedLogGain = %d, want ≈%d (±4)", got, want)
	}
}

// TestPastErrorsDefault_MatchesSpec asserts that the initial value of
// the MA-predictor's past-errors FIFO is -14·2^10 Q10 = -14336, per
// ITU-T G.729 §3.9.1 / §4.1.6 (initialization to -14 dB).
func TestPastErrorsDefault_MatchesSpec(t *testing.T) {
const wantQ10 = -14336
if pastErrorsDefault != wantQ10 {
t.Errorf("pastErrorsDefault = %d; want %d (= -14 dB Q10)", pastErrorsDefault, wantQ10)
}
}

// TestMAPredictor_EvolutionFollowsSpec drives the predictor through a
// single-subframe step and asserts the FIFO shifts the new entry into
// slot [0] while sliding the old entries down (slot [3] dropped).
//
// Initial: pastErrors = [-14336, -14336, -14336, -14336]
// After 1 subframe: pastErrors = [U(m), -14336, -14336, -14336]
// (the three older slots are former [0..2], all initialized identically
// so they remain at pastErrorsDefault.)
func TestMAPredictor_EvolutionFollowsSpec(t *testing.T) {
var d Decoder
for i := range d.pastErrors {
d.pastErrors[i] = pastErrorsDefault
}
d.initialized = true

var c [40]int16
c[0] = 8192
_, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)

if d.pastErrors[1] != pastErrorsDefault {
t.Errorf("pastErrors[1] after 1 subframe = %d; want %d (= pastErrorsDefault)",
d.pastErrors[1], pastErrorsDefault)
}
if d.pastErrors[2] != pastErrorsDefault {
t.Errorf("pastErrors[2] after 1 subframe = %d; want %d", d.pastErrors[2], pastErrorsDefault)
}
if d.pastErrors[3] != pastErrorsDefault {
t.Errorf("pastErrors[3] after 1 subframe = %d; want %d", d.pastErrors[3], pastErrorsDefault)
}
}
