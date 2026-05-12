package fcb

import "testing"

func TestDecodePositions_AllZero(t *testing.T) {
	got := decodePositions(0)
	want := [4]int{0, 1, 2, 3}
	if got != want {
		t.Fatalf("decodePositions(0) = %v, want %v", got, want)
	}
}

func TestDecodePositions_AllMax(t *testing.T) {
	got := decodePositions(0x1FFF)
	want := [4]int{35, 36, 37, 39}
	if got != want {
		t.Fatalf("decodePositions(0x1FFF) = %v, want %v", got, want)
	}
}

func TestDecodePositions_Jx0SelectsLowerHalfOfTrack3(t *testing.T) {
	got := decodePositions(uint16(5 << 10))
	if got[3] != 28 {
		t.Fatalf("pos[3] = %d, want 28 (track 3a for jx=0, i3=5)", got[3])
	}
}

func TestDecodePositions_Jx1SelectsUpperHalfOfTrack3(t *testing.T) {
	got := decodePositions(uint16((5 << 10) | (1 << 9)))
	if got[3] != 29 {
		t.Fatalf("pos[3] = %d, want 29 (track 3b for jx=1, i3=5)", got[3])
	}
}

func TestDecodePositions_TAMEVerifierClarificationCodes(t *testing.T) {
	tests := []struct {
		code uint16
		want [4]int
	}{
		{code: 4099, want: [4]int{15, 1, 2, 23}},
		{code: 3587, want: [4]int{15, 1, 2, 19}},
		{code: 4183, want: [4]int{35, 11, 7, 23}},
	}

	for _, tc := range tests {
		got := decodePositions(tc.code)
		if got != tc.want {
			t.Fatalf("decodePositions(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestDecodePositions_TrackMembershipExhaustive(t *testing.T) {
	track0 := [8]int{0, 5, 10, 15, 20, 25, 30, 35}
	track1 := [8]int{1, 6, 11, 16, 21, 26, 31, 36}
	track2 := [8]int{2, 7, 12, 17, 22, 27, 32, 37}
	track3a := [8]int{3, 8, 13, 18, 23, 28, 33, 38}
	track3b := [8]int{4, 9, 14, 19, 24, 29, 34, 39}

	contains := func(arr [8]int, v int) bool {
		for _, x := range arr {
			if x == v {
				return true
			}
		}
		return false
	}

	for code := uint16(0); code < (1 << 13); code++ {
		got := decodePositions(code)
		if !contains(track0, got[0]) {
			t.Errorf("code=0x%04x: pos[0]=%d not in track 0", code, got[0])
		}
		if !contains(track1, got[1]) {
			t.Errorf("code=0x%04x: pos[1]=%d not in track 1", code, got[1])
		}
		if !contains(track2, got[2]) {
			t.Errorf("code=0x%04x: pos[2]=%d not in track 2", code, got[2])
		}
		jx := (code >> 9) & 1
		if jx == 0 {
			if !contains(track3a, got[3]) {
				t.Errorf("code=0x%04x jx=0: pos[3]=%d not in track 3a", code, got[3])
			}
		} else {
			if !contains(track3b, got[3]) {
				t.Errorf("code=0x%04x jx=1: pos[3]=%d not in track 3b", code, got[3])
			}
		}
		for a := 0; a < 4; a++ {
			for b := a + 1; b < 4; b++ {
				if got[a] == got[b] {
					t.Errorf("code=0x%04x: positions %d and %d collide at %d", code, a, b, got[a])
				}
			}
		}
	}
}
