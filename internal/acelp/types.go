package acelp

import "errors"

// SubframeSamples is the 5 ms subframe length (§3.8).
const SubframeSamples = 40

// PulseCount is the number of non-zero pulses per ACELP subframe (§3.8).
const PulseCount = 4

var errStub = errors.New("internal/acelp: not yet implemented")

// Result holds a single subframe search outcome.
type Result struct {
	Positions     [PulseCount]int16 // 0..39
	Signs         [PulseCount]int16 // +1 or -1
	Code          [SubframeSamples]int16
	PositionsBits uint16 // 13-bit packed C-field
	SignsBits     uint16 // 4-bit packed S-field
}

// Searcher holds per-instance scratch buffers (preallocated for zero-alloc).
type Searcher struct {
	// Phase 2d will populate. Empty by design at 2-0.
}

// Reset returns the searcher to its zero state.
func (s *Searcher) Reset() { *s = Searcher{} }

// Search runs the §A.3 ACELP search. Phase 2-0 stub.
func (s *Searcher) Search(target, impulseResp []int16, out *Result) error {
	return errStub
}
