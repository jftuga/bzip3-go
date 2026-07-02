// Command bzip3 is a pure-Go port of the bzip3 command-line tool. It reads
// and writes the same file format as the C implementation and supports
// parallel block compression with -j.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	bzip3 "github.com/jftuga/bzip3-go"
)

const version = "1.0.0 (bzip3 format v1, upstream 1.5.x compatible)"

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bzip3: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	var (
		encode  = flag.Bool("e", false, "compress data (default)")
		z       = flag.Bool("z", false, "compress data (alias for -e)")
		decode  = flag.Bool("d", false, "decompress data")
		test    = flag.Bool("t", false, "verify validity of compressed data")
		stdout  = flag.Bool("c", false, "force reading/writing from standard streams")
		force   = flag.Bool("f", false, "force overwriting output if it already exists")
		keep    = flag.Bool("k", true, "keep (don't delete) input files")
		rm      = flag.Bool("rm", false, "remove input files after successful (de)compression")
		verbose = flag.Bool("v", false, "verbose mode (display effectiveness of compression)")
		showVer = flag.Bool("V", false, "display version information")
		blockMB = flag.Int("b", 16, "block size in MiB (65 KiB min effective, 511 MiB max)")
		jobs    = flag.Int("j", 0, "number of parallel workers")
	)
	flag.Parse()
	_ = keep

	if *showVer {
		fmt.Printf("bzip3-go %s\n", version)
		return
	}

	mode := "encode"
	switch {
	case *decode:
		mode = "decode"
	case *test:
		mode = "test"
	case *encode || *z:
		mode = "encode"
	}

	blockSize := int32(*blockMB) * 1024 * 1024
	if blockSize < bzip3.MinBlockSize || blockSize > bzip3.MaxBlockSize {
		fatal("block size must be between 65 KiB and 511 MiB")
	}
	if *jobs > 64 || *jobs < 0 {
		fatal("number of workers must be between 0 and 64")
	}

	input, output := resolveFiles(mode, *stdout, flag.Args())

	in := os.Stdin
	if input != "" {
		f, err := os.Open(input)
		if err != nil {
			fatal("failed to open input file: %v", err)
		}
		defer f.Close()
		in = f
	}

	var out *os.File
	switch {
	case mode == "test":
		out = nil
	case output == "":
		out = os.Stdout
	default:
		if _, err := os.Stat(output); err == nil && !*force {
			fatal("output file `%s' already exists. Use -f to force overwrite.", output)
		}
		f, err := os.Create(output)
		if err != nil {
			fatal("failed to open output file: %v", err)
		}
		defer f.Close()
		out = f
	}

	if err := process(in, out, mode, blockSize, *jobs, *verbose, input); err != nil {
		fatal("%v", err)
	}

	if *rm && input != "" {
		if err := os.Remove(input); err != nil {
			fatal("failed to remove input file: %v", err)
		}
	}
}

// resolveFiles applies the upstream CLI file-naming rules: encode appends
// .bz3, decode strips it, -c forces standard streams.
func resolveFiles(mode string, stdout bool, args []string) (input, output string) {
	if len(args) > 2 {
		fatal("too many files specified")
	}
	if len(args) == 0 {
		return "", ""
	}
	input = args[0]
	if mode == "test" {
		return input, ""
	}
	if len(args) == 2 {
		return input, args[1]
	}
	if stdout {
		return input, ""
	}
	if mode == "encode" {
		return input, input + ".bz3"
	}
	if !strings.HasSuffix(input, ".bz3") {
		fatal("file %s has an unknown extension, expected .bz3", input)
	}
	return input, strings.TrimSuffix(input, ".bz3")
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func process(in io.Reader, out io.Writer, mode string, blockSize int32, workers int, verbose bool, name string) error {
	var read, written int64
	var err error

	switch mode {
	case "encode":
		read, written, err = compress(in, out, blockSize, workers)
	case "decode":
		read, written, err = decompress(in, out, workers)
	case "test":
		read, written, err = decompress(in, io.Discard, workers)
	}
	if err != nil {
		return err
	}

	if verbose {
		if name != "" {
			fmt.Fprintf(os.Stderr, " %s:", name)
		}
		if mode == "encode" {
			fmt.Fprintf(os.Stderr, "\t%d -> %d bytes, %.2f%%, %.2f bpb\n",
				read, written, float64(written)*100/float64(read), float64(written)*8/float64(read))
		} else {
			fmt.Fprintf(os.Stderr, "\t%d -> %d bytes, %.2f%%, %.2f bpb\n",
				read, written, float64(read)*100/float64(written), float64(read)*8/float64(written))
		}
	}
	return nil
}

func compress(in io.Reader, out io.Writer, blockSize int32, workers int) (int64, int64, error) {
	cw := &countingWriter{w: out}
	if workers <= 1 {
		w, err := bzip3.NewWriter(cw, blockSize)
		if err != nil {
			return 0, 0, err
		}
		read, err := io.Copy(w, in)
		if err != nil {
			return read, cw.n, err
		}
		return read, cw.n, w.Close()
	}
	read, err := compressParallel(in, cw, blockSize, workers)
	return read, cw.n, err
}

// compressParallel mirrors the upstream pthread path: read up to `workers`
// blocks, encode them concurrently, then write them in order.
func compressParallel(in io.Reader, out io.Writer, blockSize int32, workers int) (int64, error) {
	states := make([]*bzip3.State, workers)
	buffers := make([][]byte, workers)
	for i := range states {
		s, err := bzip3.NewState(blockSize)
		if err != nil {
			return 0, err
		}
		states[i] = s
		buffers[i] = make([]byte, bzip3.Bound(int(blockSize)))
	}

	var hdr [9]byte
	copy(hdr[:], "BZ3v1")
	putS32(hdr[5:], blockSize)
	if _, err := out.Write(hdr[:]); err != nil {
		return 0, err
	}

	var read int64
	origSizes := make([]int32, workers)
	newSizes := make([]int32, workers)
	errs := make([]error, workers)

	for {
		n := 0
		for ; n < workers; n++ {
			m, err := io.ReadFull(in, buffers[n][:blockSize])
			origSizes[n] = int32(m)
			read += int64(m)
			if err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					return read, err
				}
				if m > 0 {
					n++
				}
				break
			}
		}
		if n == 0 {
			return read, nil
		}

		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				newSizes[i], errs[i] = states[i].EncodeBlock(buffers[i], origSizes[i])
			}(i)
		}
		wg.Wait()

		for i := 0; i < n; i++ {
			if errs[i] != nil {
				return read, errs[i]
			}
			var bh [8]byte
			putS32(bh[0:], newSizes[i])
			putS32(bh[4:], origSizes[i])
			if _, err := out.Write(bh[:]); err != nil {
				return read, err
			}
			if _, err := out.Write(buffers[i][:newSizes[i]]); err != nil {
				return read, err
			}
		}
		if n < workers {
			return read, nil
		}
	}
}

func decompress(in io.Reader, out io.Writer, workers int) (int64, int64, error) {
	cr := &countingReader{r: in}
	cw := &countingWriter{w: out}
	if workers <= 1 {
		r := bzip3.NewReader(cr)
		_, err := io.Copy(cw, r)
		return cr.n, cw.n, err
	}
	_, err := decompressParallel(cr, cw, workers)
	return cr.n, cw.n, err
}

func decompressParallel(in io.Reader, out io.Writer, workers int) (int64, error) {
	var hdr [9]byte
	if _, err := io.ReadFull(in, hdr[:]); err != nil {
		return 0, bzip3.ErrMalformedHeader
	}
	if string(hdr[:5]) != "BZ3v1" {
		return 0, bzip3.ErrMalformedHeader
	}
	blockSize := getS32(hdr[5:])
	if blockSize < bzip3.MinBlockSize || blockSize > bzip3.MaxBlockSize {
		return 0, bzip3.ErrMalformedHeader
	}
	bound := bzip3.Bound(int(blockSize))

	states := make([]*bzip3.State, workers)
	buffers := make([][]byte, workers)
	for i := range states {
		s, err := bzip3.NewState(blockSize)
		if err != nil {
			return 0, err
		}
		states[i] = s
		buffers[i] = make([]byte, bound)
	}

	read := int64(9)
	origSizes := make([]int32, workers)
	newSizes := make([]int32, workers)
	dSizes := make([]int32, workers)
	errs := make([]error, workers)

	for {
		n := 0
		eof := false
		for ; n < workers; n++ {
			var bh [8]byte
			if _, err := io.ReadFull(in, bh[:]); err != nil {
				if errors.Is(err, io.ErrUnexpectedEOF) {
					return read, bzip3.ErrTruncatedData
				}
				if !errors.Is(err, io.EOF) {
					return read, err
				}
				eof = true
				break
			}
			newSizes[n] = getS32(bh[0:])
			origSizes[n] = getS32(bh[4:])
			if newSizes[n] < 0 || int(newSizes[n]) > bound || origSizes[n] < 0 || int(origSizes[n]) > bound {
				return read, bzip3.ErrMalformedHeader
			}
			if _, err := io.ReadFull(in, buffers[n][:newSizes[n]]); err != nil {
				return read, bzip3.ErrTruncatedData
			}
			read += int64(8 + newSizes[n])
		}
		if n == 0 {
			return read, nil
		}

		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				dSizes[i], errs[i] = states[i].DecodeBlock(buffers[i], newSizes[i], origSizes[i])
			}(i)
		}
		wg.Wait()

		for i := 0; i < n; i++ {
			if errs[i] != nil {
				return read, errs[i]
			}
			if dSizes[i] != origSizes[i] {
				return read, bzip3.ErrMalformedHeader
			}
			if _, err := out.Write(buffers[i][:origSizes[i]]); err != nil {
				return read, err
			}
		}
		if eof {
			return read, nil
		}
	}
}

func putS32(b []byte, v int32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func getS32(b []byte) int32 {
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
}
