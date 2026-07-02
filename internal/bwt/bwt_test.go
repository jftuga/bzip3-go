package bwt

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

func TestKnownVector(t *testing.T) {
	// Hand-computed with libsais conventions: T="abab" has SA=[2,0,3,1],
	// so U = "bbaa" with primary index 2.
	in := []byte("abab")
	out := make([]byte, 4)
	idx := Encode(out, in, make([]int32, 4))
	if string(out) != "bbaa" || idx != 2 {
		t.Fatalf("Encode(abab) = %q idx %d, want \"bbaa\" idx 2", out, idx)
	}
	dec := make([]byte, 4)
	if err := new(Unbwt).Decode(dec, out, make([]int32, 5), idx); err != nil {
		t.Fatal(err)
	}
	if string(dec) != "abab" {
		t.Fatalf("Decode = %q, want \"abab\"", dec)
	}
}

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(3))

	cases := map[string][]byte{
		"banana":     []byte("banana"),
		"repetitive": bytes.Repeat([]byte("abcabc"), 5000),
		"zeros":      make([]byte, 10000),
		"two":        {1, 2},
		"same":       {5, 5, 5, 5},
	}
	random := make([]byte, 100000)
	rng.Read(random)
	cases["random"] = random
	if text, err := os.ReadFile("../../testdata/shakespeare.txt"); err == nil {
		cases["shakespeare"] = text[:300000]
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			n := len(in)
			out := make([]byte, n)
			idx := Encode(out, in, make([]int32, n))
			if idx < 1 || int(idx) > n {
				t.Fatalf("primary index %d out of range [1, %d]", idx, n)
			}
			dec := make([]byte, n)
			if err := new(Unbwt).Decode(dec, out, make([]int32, n+1), idx); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(in, dec) {
				t.Fatalf("round trip mismatch (len %d)", n)
			}
		})
	}
}

func TestDecodeInvalidIndex(t *testing.T) {
	in := []byte("banana")
	for _, idx := range []int32{0, -1, 7} {
		if err := new(Unbwt).Decode(make([]byte, 6), in, make([]int32, 7), idx); err == nil {
			t.Fatalf("Decode with idx %d succeeded, want error", idx)
		}
	}
}
