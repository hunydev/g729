package lsp

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// combineResidual builds the 10-element Q13 residual LSF vector for
// one frame from the three split-VQ indices, per ITU-T G.729 §3.2.4
// equation (19):
//
//	r[i] = L1[l1][i] + L2[l2][i]       for i in [0, 5)
//	r[i] = L1[l1][i] + L3[l3][i-5]     for i in [5, 10)
//
// All values are Q13 int16. fixed.Add provides Word16 saturation;
// realistic codebook entries do not approach saturation but the
// guard is part of the package's defensive arithmetic contract.
func combineResidual(l1, l2, l3 uint8, out *[10]int16) {
	for i := 0; i < 5; i++ {
		out[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL2[l2][i])
	}
	for i := 5; i < 10; i++ {
		out[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL3[l3][i-5])
	}
}
