package gain

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// decodeVQ performs the conjugate-structure two-stage codebook
// lookup per ITU-T G.729 §3.9:
//
//	g_p   = GainGBK1[GA][0] + GainGBK2[GB][0]      (Q14)
//	γ̂_c = GainGBK1[GA][1] + GainGBK2[GB][1]      (Q13)
//
// The stages are summed component-wise with Word16 saturation.  Per
// ITU-T G.729 §3.9.3, the encoder reorders GA/GB indices before
// transmission to reduce the impact of single bit errors, so the
// decoder MUST apply the inverse map (GainImap1/GainImap2) to recover
// the physical GBK entry index from the received bits.
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gaEntry := tables.GainImap1[idx.GA]
	gbEntry := tables.GainImap2[idx.GB]
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][0]), fixed.Word16(tables.GainGBK2[gbEntry][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][1]), fixed.Word16(tables.GainGBK2[gbEntry][1])))
	return
}
