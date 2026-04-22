package decoder

import "testing"

var benchOut [80]int16

func BenchmarkDecode(b *testing.B) {
	var d Decoder
	packed := [10]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Decode(packed[:], false, benchOut[:])
	}
}
