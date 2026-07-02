package bzip3

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"testing"
)

func TestBlockRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(11))

	cases := map[string][]byte{
		"empty":      {},
		"tiny":       []byte("hi"),
		"just-63":    bytes.Repeat([]byte{'x'}, 63),
		"just-64":    bytes.Repeat([]byte{'x'}, 64),
		"text":       bytes.Repeat([]byte("compression test data with some repetition. "), 2000),
		"zeros":      make([]byte, 100000),
		"pattern-ff": bytes.Repeat([]byte{0xf2, 0xf2, 0x00, 0xff}, 30000),
	}
	random := make([]byte, 200000)
	rng.Read(random)
	cases["random"] = random
	if text, err := os.ReadFile("testdata/shakespeare.txt"); err == nil {
		cases["shakespeare"] = text
	}

	state, err := NewState(DefaultBlockSize)
	if err != nil {
		t.Fatal(err)
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			buf := make([]byte, Bound(len(in)))
			copy(buf, in)
			cSize, err := state.EncodeBlock(buf, int32(len(in)))
			if err != nil {
				t.Fatalf("EncodeBlock: %v", err)
			}
			dSize, err := state.DecodeBlock(buf, cSize, int32(len(in)))
			if err != nil {
				t.Fatalf("DecodeBlock: %v", err)
			}
			if int(dSize) != len(in) || !bytes.Equal(buf[:dSize], in) {
				t.Fatalf("round trip mismatch: got %d bytes, want %d", dSize, len(in))
			}
		})
	}
}

func TestFrameRoundTrip(t *testing.T) {
	in, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		t.Fatal(err)
	}
	comp, err := Compress(in, MinBlockSize) // multiple blocks
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(comp)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec, in) {
		t.Fatal("frame round trip mismatch")
	}
}

func TestFrameEmpty(t *testing.T) {
	comp, err := Compress(nil, DefaultBlockSize)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decompress(comp)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) != 0 {
		t.Fatalf("got %d bytes, want 0", len(dec))
	}
}

func TestWriterReaderRoundTrip(t *testing.T) {
	in, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w, err := NewWriter(&buf, MinBlockSize)
	if err != nil {
		t.Fatal(err)
	}
	// Write in odd-sized chunks to exercise buffering.
	for off := 0; off < len(in); {
		n := 12345
		if off+n > len(in) {
			n = len(in) - off
		}
		if _, err := w.Write(in[off : off+n]); err != nil {
			t.Fatal(err)
		}
		off += n
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	dec, err := io.ReadAll(NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec, in) {
		t.Fatal("writer/reader round trip mismatch")
	}
}

// TestDecodeCReference decodes testdata/shakespeare.txt.bz3, which was
// produced by the C implementation, proving decode-direction format
// compatibility.
func TestDecodeCReference(t *testing.T) {
	comp, err := os.Open("testdata/shakespeare.txt.bz3")
	if err != nil {
		t.Fatal(err)
	}
	defer comp.Close()
	want, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(NewReader(comp))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded %d bytes, want %d; contents differ", len(got), len(want))
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Run("bad-magic", func(t *testing.T) {
		if _, err := io.ReadAll(NewReader(bytes.NewReader([]byte("NOTBZ3xxx")))); err == nil {
			t.Fatal("want error for bad magic")
		}
	})
	t.Run("truncated", func(t *testing.T) {
		var buf bytes.Buffer
		w, _ := NewWriter(&buf, MinBlockSize)
		w.Write(bytes.Repeat([]byte("data"), 10000))
		w.Close()
		trunc := buf.Bytes()[:buf.Len()-10]
		if _, err := io.ReadAll(NewReader(bytes.NewReader(trunc))); err == nil {
			t.Fatal("want error for truncated stream")
		}
	})
	t.Run("corrupt-crc", func(t *testing.T) {
		var buf bytes.Buffer
		w, _ := NewWriter(&buf, MinBlockSize)
		w.Write(bytes.Repeat([]byte("data"), 10000))
		w.Close()
		// Corrupt a byte in the middle of the entropy-coded payload; the
		// final bytes are arithmetic-coder flush and may be irrelevant to
		// decoding.
		b := buf.Bytes()
		b[len(b)/2] ^= 0xFF
		if _, err := io.ReadAll(NewReader(bytes.NewReader(b))); err == nil {
			t.Fatal("want error for corrupt data")
		}
	})
}
