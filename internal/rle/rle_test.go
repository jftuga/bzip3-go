package rle

import (
	"bytes"
	"math/rand"
	"testing"
)

func roundTrip(t *testing.T, in []byte) {
	t.Helper()
	out := make([]byte, len(in)+len(in)/2+64)
	n := Encode(in, out)
	dec := make([]byte, len(in))
	if err := Decode(out[:n], dec); err != nil {
		t.Fatalf("Decode failed: %v (encoded %d bytes)", err, n)
	}
	if !bytes.Equal(in, dec) {
		t.Fatalf("round trip mismatch (len %d, encoded %d)", len(in), n)
	}
}

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	t.Run("runs", func(t *testing.T) {
		var in []byte
		for i := 0; i < 100; i++ {
			b := byte(rng.Intn(4))
			n := rng.Intn(1000) + 1
			in = append(in, bytes.Repeat([]byte{b}, n)...)
		}
		roundTrip(t, in)
	})

	t.Run("random", func(t *testing.T) {
		in := make([]byte, 100000)
		rng.Read(in)
		roundTrip(t, in)
	})

	t.Run("mixed", func(t *testing.T) {
		var in []byte
		for i := 0; i < 200; i++ {
			if rng.Intn(2) == 0 {
				in = append(in, bytes.Repeat([]byte{byte(rng.Intn(256))}, rng.Intn(600)+1)...)
			} else {
				chunk := make([]byte, rng.Intn(100)+1)
				rng.Read(chunk)
				in = append(in, chunk...)
			}
		}
		roundTrip(t, in)
	})

	t.Run("long-run", func(t *testing.T) {
		roundTrip(t, bytes.Repeat([]byte{0xAA}, 1<<16))
	})

	t.Run("single", func(t *testing.T) {
		roundTrip(t, []byte{7})
	})
}
