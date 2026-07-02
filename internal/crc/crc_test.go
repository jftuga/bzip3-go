package crc

import (
	"hash/crc32"
	"math/rand"
	"testing"
)

func TestTableMatchesCastagnoli(t *testing.T) {
	ref := crc32.MakeTable(crc32.Castagnoli)
	for i := range tables[0] {
		if tables[0][i] != ref[i] {
			t.Fatalf("table[%d] = %#x, want %#x", i, tables[0][i], ref[i])
		}
	}
}

func TestKnownValues(t *testing.T) {
	// First entries of the hardcoded table in upstream libbz3.c.
	want := []uint32{0x00000000, 0xF26B8303, 0xE13B70F7, 0x1350F3F4}
	for i, w := range want {
		if tables[0][i] != w {
			t.Fatalf("table[%d] = %#x, want %#x", i, tables[0][i], w)
		}
	}
	if got := Sum(1, []byte{}); got != 1 {
		t.Fatalf("Sum(1, empty) = %d, want 1", got)
	}
}

// sumBytewise is the reference byte-at-a-time formulation from upstream.
func sumBytewise(crc uint32, buf []byte) uint32 {
	for _, b := range buf {
		crc = tables[0][byte(crc)^b] ^ (crc >> 8)
	}
	return crc
}

func TestSlicingMatchesBytewise(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	data := make([]byte, 1024)
	rng.Read(data)
	for length := 0; length <= 64; length++ {
		for offset := 0; offset < 8; offset++ {
			buf := data[offset : offset+length]
			if got, want := Sum(1, buf), sumBytewise(1, buf); got != want {
				t.Fatalf("Sum(1, data[%d:%d]) = %#x, want %#x", offset, offset+length, got, want)
			}
		}
	}
	if got, want := Sum(1, data), sumBytewise(1, data); got != want {
		t.Fatalf("Sum(1, data) = %#x, want %#x", got, want)
	}
}
