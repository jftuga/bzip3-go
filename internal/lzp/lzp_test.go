package lzp

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	lut := make([]int32, DictSize)

	cases := map[string][]byte{
		"repetitive": bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog. "), 500),
		"longruns":   bytes.Repeat([]byte{0}, 10000),
	}
	if text, err := os.ReadFile("../../testdata/shakespeare.txt"); err == nil {
		cases["shakespeare"] = text[:200000]
	}
	random := make([]byte, 50000)
	rng.Read(random)
	cases["random"] = random

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			enc := make([]byte, len(in))
			n := Compress(in, enc, lut)
			if n == -1 {
				t.Skip("incompressible with LZP; nothing to verify")
			}
			dec := make([]byte, len(in)+1024)
			m := Decompress(enc[:n], dec, lut)
			if m != int32(len(in)) || !bytes.Equal(dec[:m], in) {
				t.Fatalf("round trip mismatch: got %d bytes, want %d", m, len(in))
			}
		})
	}
}

func TestTooSmall(t *testing.T) {
	lut := make([]int32, DictSize)
	in := make([]byte, minMatch+31)
	if n := Compress(in, make([]byte, len(in)), lut); n != -1 {
		t.Fatalf("Compress on tiny input = %d, want -1", n)
	}
}
