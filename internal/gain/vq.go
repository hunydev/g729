package gain

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// decodeVQ performs the conjugate-structure two-stage codebook
// lookup per ITU-T G.729 §3.9:
//
//	g_p   = GainGBK1[GA][0] + GainGBK2[GB][0]      (Q14)
//	γ̂_c = GainGBK1[GA][1] + GainGBK2[GB][1]      (Q13)
//
// The stages are summed component-wise with Word16 saturation.  The
// codebooks are indexed directly by the received bits (GA, GB); the
// optional reorder tables (Map/Imap) live in tables for the encoder
// search routine and play no role at the decoder.
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][0]), fixed.Word16(tables.GainGBK2[idx.GB][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][1]), fixed.Word16(tables.GainGBK2[idx.GB][1])))
	return
}
