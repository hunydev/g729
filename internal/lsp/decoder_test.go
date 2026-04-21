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
