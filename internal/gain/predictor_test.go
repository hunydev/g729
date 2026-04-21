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
