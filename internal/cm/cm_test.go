package cm

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(7))

	cases := map[string][]byte{
		"text":   bytes.Repeat([]byte("hello, world! "), 1000),
		"zeros":  make([]byte, 20000),
		"single": {42},
	}
	random := make([]byte, 30000)
	rng.Read(random)
	cases["random"] = random

	var c Coder
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			enc := make([]byte, len(in)+len(in)/2+64)
			c.Begin()
			n := c.Encode(enc, in)
			dec := make([]byte, len(in))
			c.Begin()
			c.Decode(enc[:n], dec)
			if !bytes.Equal(in, dec) {
				t.Fatalf("round trip mismatch (len %d, encoded %d)", len(in), n)
			}
		})
	}
}
