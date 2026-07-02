// Package crc implements the CRC32 variant used by bzip3: the Castagnoli
// polynomial with a caller-supplied initial value (bzip3 uses 1) and no
// final XOR. This matches crc32sum() in upstream libbz3.c.
package crc

// Sum updates crc with buf and returns the new value.
func Sum(crc uint32, buf []byte) uint32 {
	for _, b := range buf {
		crc = table[byte(crc)^b] ^ (crc >> 8)
	}
	return crc
}

var table = makeTable()

func makeTable() (t [256]uint32) {
	// Reversed Castagnoli polynomial, as in upstream's hardcoded table.
	const poly = 0x82F63B78
	for i := range t {
		c := uint32(i)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = poly ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}
