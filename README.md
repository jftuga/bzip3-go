# bzip3-go

A pure-Go port of the [bzip3](https://github.com/iczelia/bzip3) compression
algorithm by Kamila Szewczyk. No cgo. Fully file-format compatible with the
C implementation, and verified to produce bit-for-bit identical compressed
output.

The codec pipeline is, per independent block:

1. RLE - per-byte-value run collapsing, applied only when it shrinks the block
2. LZP - hash-table match prediction (2^18 entries, minimum match 40), applied only when it shrinks the block
3. BWT - Burrows-Wheeler transform via a linear-time SA-IS suffix array
4. An order-0 context-mixing arithmetic coder

## Library

```go
import bzip3 "github.com/jftuga/bzip3-go"
```

Streaming (the bzip3 file format, same as the `bzip3` CLI tool):

```go
w, _ := bzip3.NewWriter(dst, bzip3.DefaultBlockSize)
io.Copy(w, src)
w.Close()

r := bzip3.NewReader(src)
io.Copy(dst, r)
```

One-shot frame API (equivalent to upstream `bz3_compress`/`bz3_decompress`):

```go
compressed, err := bzip3.Compress(data, bzip3.DefaultBlockSize)
original, err := bzip3.Decompress(compressed)
```

Low-level block API (equivalent to `bz3_encode_block`/`bz3_decode_block`),
for parallel or custom-container use:

```go
state, _ := bzip3.NewState(blockSize)
n, err := state.EncodeBlock(buf, dataSize) // in place; len(buf) >= bzip3.Bound(dataSize)
n, err := state.DecodeBlock(buf, compressedSize, origSize)
```

A `State` is not safe for concurrent use; create one per goroutine.

## CLI

```console
$ go install github.com/jftuga/bzip3-go/cmd/bzip3@latest

$ bzip3 -e file          # -> file.bz3
$ bzip3 -d file.bz3      # -> file
$ bzip3 -t file.bz3      # integrity test
$ bzip3 -e -b 32 -j 8 -c < input > output.bz3   # 32 MiB blocks, 8 workers
```

Flags mirror the C tool: `-e`/`-z` encode, `-d` decode, `-t` test, `-b`
block size in MiB (default 16), `-j` parallel workers, `-c` standard
streams, `-f` force overwrite, `-v` verbose, `-rm` remove input, `-V`
version. Note Go's flag parsing does not support combined short options
(use `-f -e -b 6`, not `-feb 6`).

## Compatibility and testing

- Decodes streams produced by C bzip3, and C bzip3 decodes streams produced
  by this port (verified in CI against the Debian `bzip3` package).
- Compressed output is byte-identical to C bzip3 1.5.3 for both the
  sequential and parallel paths. BWT output is unique for a given input and
  every other stage is deterministic, so this is expected to hold for all
  inputs; round-trip integrity is what the test suite guarantees.
- Upstream's test targets are ported: decoding `shakespeare.txt.bz3` (made
  by the C implementation) and the LICENSE round trip.
- Native Go fuzz targets port upstream's AFL harnesses (round trip, frame
  decompression, block decoding against adversarial input).

## Performance

On an Apple M4, throughput is competitive with C bzip3 1.5.3; on a 21 MiB
single-block input, Go encode measured slightly faster than C (0.37s vs
0.45s user), and decode is within ~20%. These are single-file measurements,
not a rigorous benchmark, but there is no order-of-magnitude gap. See
`PERFORMANCE_IMPROVEMENT.md` for profiled hotspots and optimization ideas.

## Differences from the C implementation

- Malformed streams whose blocks decode to a size other than the advertised
  original size are rejected (`ErrMalformedHeader`); upstream copies stale
  buffer bytes in some of these cases.
- The suffix array uses `[]int` (8 bytes per entry on 64-bit platforms)
  versus C's `int32_t`, so encoding needs roughly `12 * blockSize` bytes of
  scratch versus C's ~8. This is a memory difference, not a speed one;
  converting the SA-IS port to `int32` is a possible future optimization.
- `bz3_recover` mode and the batch (`-B`) CLI mode are not implemented.

## License

LGPL-3.0-or-later, as this is a derivative work of libbz3 (Copyright
Kamila Szewczyk). The SA-IS suffix array construction in `internal/sais`
is adapted from [dsnet/compress](https://github.com/dsnet/compress)
(BSD 3-clause, Joe Tsai), itself a port of sais-lite by Yuta Mori (MIT);
see `internal/sais/LICENSE.md`.
