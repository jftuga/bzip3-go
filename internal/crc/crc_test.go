package crc

import (
	"hash/crc32"
	"testing"
)

func TestTableMatchesCastagnoli(t *testing.T) {
	ref := crc32.MakeTable(crc32.Castagnoli)
	for i := range table {
		if table[i] != ref[i] {
			t.Fatalf("table[%d] = %#x, want %#x", i, table[i], ref[i])
		}
	}
}

func TestKnownValues(t *testing.T) {
	// First entries of the hardcoded table in upstream libbz3.c.
	want := []uint32{0x00000000, 0xF26B8303, 0xE13B70F7, 0x1350F3F4}
	for i, w := range want {
		if table[i] != w {
			t.Fatalf("table[%d] = %#x, want %#x", i, table[i], w)
		}
	}
	if got := Sum(1, []byte{}); got != 1 {
		t.Fatalf("Sum(1, empty) = %d, want 1", got)
	}
}
