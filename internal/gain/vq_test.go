package gain

import (
	"testing"

	"github.com/exedev/g729/internal/tables"
)

func TestDecodeVQ_SumsMatchTableEntries(t *testing.T) {
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			gp, gammaC := decodeVQ(Indices{GA: ga, GB: gb})
			wantGp := int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])
			wantGc := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
			if wantGp > 32767 {
				wantGp = 32767
			} else if wantGp < -32768 {
				wantGp = -32768
			}
			if wantGc > 32767 {
				wantGc = 32767
			} else if wantGc < -32768 {
				wantGc = -32768
			}
			if int32(gp) != wantGp {
				t.Errorf("(GA=%d, GB=%d): g_p = %d, want %d", ga, gb, gp, wantGp)
			}
			if int32(gammaC) != wantGc {
				t.Errorf("(GA=%d, GB=%d): γ̂_c = %d, want %d", ga, gb, gammaC, wantGc)
			}
		}
	}
}

func TestDecodeVQ_GPNonNegative(t *testing.T) {
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			gp, _ := decodeVQ(Indices{GA: ga, GB: gb})
			if gp < 0 {
				t.Errorf("(GA=%d, GB=%d): g_p = %d negative", ga, gb, gp)
			}
		}
	}
}
