package bzip3

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// cBinary returns the path to the reference C bzip3 binary, skipping the
// test when it is not installed.
func cBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("bzip3")
	if err != nil {
		t.Skip("C bzip3 binary not found; skipping cross-validation")
	}
	return path
}

func runC(t *testing.T, bin string, stdin []byte, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v: %v\n%s", bin, args, err, errb.String())
	}
	return out.Bytes()
}

// TestCrossGoEncodeCDecode verifies the C implementation decodes Go output.
func TestCrossGoEncodeCDecode(t *testing.T) {
	bin := cBinary(t)
	in, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		t.Fatal(err)
	}

	var comp bytes.Buffer
	w, err := NewWriter(&comp, MinBlockSize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(in); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	dec := runC(t, bin, comp.Bytes(), "-d", "-c")
	if !bytes.Equal(dec, in) {
		t.Fatalf("C decoded %d bytes from Go stream, want %d; contents differ", len(dec), len(in))
	}
}

// TestCrossCEncodeGoDecode verifies Go decodes fresh C output at several
// block sizes.
func TestCrossCEncodeGoDecode(t *testing.T) {
	bin := cBinary(t)
	in, err := os.ReadFile("testdata/shakespeare.txt")
	if err != nil {
		t.Fatal(err)
	}

	for _, blockMiB := range []string{"1", "2", "16"} {
		t.Run("b"+blockMiB, func(t *testing.T) {
			comp := runC(t, bin, in, "-e", "-c", "-b", blockMiB)
			dec, err := io.ReadAll(NewReader(bytes.NewReader(comp)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dec, in) {
				t.Fatalf("Go decoded %d bytes from C stream, want %d; contents differ", len(dec), len(in))
			}
		})
	}
}

// TestByteIdenticalEncode checks that Go compression output is bit-for-bit
// identical to the C implementation. BWT output is unique for a given
// input and every other stage is deterministic, so any divergence is a
// porting bug.
func TestByteIdenticalEncode(t *testing.T) {
	bin := cBinary(t)

	inputs := map[string]string{
		"shakespeare": "testdata/shakespeare.txt",
		"license":     "testdata/LICENSE.txt",
	}
	for name, path := range inputs {
		t.Run(name, func(t *testing.T) {
			in, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cOut := runC(t, bin, in, "-e", "-c", "-b", "2")

			var goOut bytes.Buffer
			w, err := NewWriter(&goOut, 2*1024*1024)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(in); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(cOut, goOut.Bytes()) {
				diff := -1
				for i := 0; i < min(len(cOut), goOut.Len()); i++ {
					if cOut[i] != goOut.Bytes()[i] {
						diff = i
						break
					}
				}
				t.Fatalf("output differs: C %d bytes, Go %d bytes, first difference at offset %d",
					len(cOut), goOut.Len(), diff)
			}
		})
	}
}

// TestCLIRoundTripLikeUpstream mirrors upstream's `make roundtrip` target:
// compress LICENSE at block size 6, decompress, compare.
func TestCLIRoundTripLikeUpstream(t *testing.T) {
	in, err := os.ReadFile("testdata/LICENSE.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()

	var comp bytes.Buffer
	w, err := NewWriter(&comp, 6*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(in); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	compPath := filepath.Join(tmp, "LICENSE.bz3")
	if err := os.WriteFile(compPath, comp.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(compPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec, err := io.ReadAll(NewReader(f))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec, in) {
		t.Fatal("round trip mismatch")
	}
}
