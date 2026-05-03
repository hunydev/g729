package lsp

import "errors"

// ErrLPCNonStable is returned by the LP→LSP root finder when fewer
// than 5 sign changes are detected for either F1(z) or F2(z) on the
// 60-point grid of §3.2.3 lines 782–784. A stable Levinson-Durbin LP
// filter always yields exactly 5 roots per polynomial; missing roots
// indicate upstream instability that the encoder routes to E8.
var ErrLPCNonStable = errors.New("g729/lsp: fewer than 5 sign changes in F1 or F2 — LP filter not stable")
