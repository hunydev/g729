package lsp

import "testing"

func TestDecoderShape(t *testing.T) {
	var d Decoder
	sf1, sf2 := d.Decode(Indices{})
	if len(sf1) != 11 {
		t.Fatalf("subframe 1 LP length = %d, want 11", len(sf1))
	}
	if len(sf2) != 11 {
		t.Fatalf("subframe 2 LP length = %d, want 11", len(sf2))
	}
}

func TestDecodeAllZeroIndicesProducesStableLP(t *testing.T) {
	var d Decoder
	sf1, sf2 := d.Decode(Indices{L0: 0, L1: 0, L2: 0, L3: 0})

	if sf1[0] != 4096 {
		t.Errorf("sf1[0] = %d, want 4096 (Q12 1.0)", sf1[0])
	}
	if sf2[0] != 4096 {
		t.Errorf("sf2[0] = %d, want 4096 (Q12 1.0)", sf2[0])
	}
	nonZero := false
	for i := 1; i < 11; i++ {
		if sf1[i] != 0 || sf2[i] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("entire LP vector is zero — decoder pipeline is not wired")
	}
}

func TestDecodeL0SelectorChangesOutput(t *testing.T) {
	var d0, d1 Decoder
	sf1a, _ := d0.Decode(Indices{L0: 0, L1: 10, L2: 5, L3: 7})
	sf1b, _ := d1.Decode(Indices{L0: 1, L1: 10, L2: 5, L3: 7})
	if sf1a == sf1b {
		t.Fatal("decoder produced identical output for L0=0 and L0=1")
	}
}

func TestDecodeResetRestoresDeterminism(t *testing.T) {
	var d Decoder
	_, _ = d.Decode(Indices{L0: 1, L1: 42, L2: 11, L3: 3})
	d.Reset()

	afterReset, _ := d.Decode(Indices{L0: 0, L1: 0, L2: 0, L3: 0})
	var fresh Decoder
	freshOut, _ := fresh.Decode(Indices{L0: 0, L1: 0, L2: 0, L3: 0})

	if afterReset != freshOut {
		t.Error("after Reset, decoder does not match a freshly-zero-valued one")
	}
}
