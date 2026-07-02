package bzip3

// Go-native ports of upstream's AFL fuzz harnesses: fuzz-round-trip.c,
// fuzz-decompress.c and fuzz-decode-block.c. Memory safety is enforced by
// the runtime; these check for panics, hangs and round-trip integrity.

import (
	"bytes"
	"testing"
)

func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("hello world"))
	f.Add(bytes.Repeat([]byte{0}, 100))
	f.Add(bytes.Repeat([]byte("abc"), 500))
	f.Add([]byte{0xf2, 0xf2, 0xf2, 0xf2})

	state, err := NewState(MinBlockSize)
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > MinBlockSize {
			data = data[:MinBlockSize]
		}
		buf := make([]byte, Bound(len(data)))
		copy(buf, data)
		cSize, err := state.EncodeBlock(buf, int32(len(data)))
		if err != nil {
			t.Fatalf("EncodeBlock: %v", err)
		}
		dSize, err := state.DecodeBlock(buf, cSize, int32(len(data)))
		if err != nil {
			t.Fatalf("DecodeBlock: %v", err)
		}
		if int(dSize) != len(data) || !bytes.Equal(buf[:dSize], data) {
			t.Fatalf("round trip mismatch: %d bytes, want %d", dSize, len(data))
		}
	})
}

func FuzzDecompressFrame(f *testing.F) {
	valid, err := Compress([]byte("some sample data to build a valid frame around"), MinBlockSize)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte("BZ3v1"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic; errors are expected for malformed input.
		out, err := Decompress(data)
		_ = out
		_ = err
	})
}

func FuzzDecodeBlock(f *testing.F) {
	// Seed with a valid encoded block.
	state, err := NewState(MinBlockSize)
	if err != nil {
		f.Fatal(err)
	}
	seed := bytes.Repeat([]byte("block fuzz seed data. "), 100)
	buf := make([]byte, Bound(len(seed)))
	copy(buf, seed)
	cSize, err := state.EncodeBlock(buf, int32(len(seed)))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf[:cSize], int32(len(seed)))
	f.Add([]byte{}, int32(0))

	f.Fuzz(func(t *testing.T, block []byte, origSize int32) {
		if origSize < 0 || origSize > MinBlockSize {
			return
		}
		if len(block) > MinBlockSize {
			block = block[:MinBlockSize]
		}
		dbuf := make([]byte, Bound(int(origSize))+len(block))
		copy(dbuf, block)
		// Must never panic or write out of bounds; errors are expected.
		_, _ = state.DecodeBlock(dbuf, int32(len(block)), origSize)
	})
}
