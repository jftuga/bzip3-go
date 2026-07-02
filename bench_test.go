package bzip3

import (
	"os"
	"testing"
)

func benchData(b *testing.B) []byte {
	b.Helper()
	data, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		b.Skip("testdata/shakespeare.txt not available")
	}
	return data
}

func BenchmarkEncodeBlock(b *testing.B) {
	data := benchData(b)
	state, err := NewState(DefaultBlockSize)
	if err != nil {
		b.Fatal(err)
	}
	buf := make([]byte, Bound(len(data)))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, data)
		if _, err := state.EncodeBlock(buf, int32(len(data))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeBlock(b *testing.B) {
	data := benchData(b)
	state, err := NewState(DefaultBlockSize)
	if err != nil {
		b.Fatal(err)
	}
	enc := make([]byte, Bound(len(data)))
	copy(enc, data)
	cSize, err := state.EncodeBlock(enc, int32(len(data)))
	if err != nil {
		b.Fatal(err)
	}
	buf := make([]byte, Bound(len(data)))
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf[:cSize], enc[:cSize])
		if _, err := state.DecodeBlock(buf, cSize, int32(len(data))); err != nil {
			b.Fatal(err)
		}
	}
}
