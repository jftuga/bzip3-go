// Package bwt implements the Burrows-Wheeler transform with the exact
// conventions of libsais_bwt/libsais_unbwt as used by bzip3: the output
// starts with the last input byte, the row corresponding to the full string
// is omitted, and the returned primary index is that row's position plus
// one (equivalently, the position of the sentinel row in the BWT of the
// sentinel-terminated string).
package bwt

import (
	"errors"

	"github.com/jftuga/bzip3-go/internal/sais"
)

// ErrInvalidIndex is returned by Decode for a primary index outside [1, n]
// (or != n when n <= 1), matching libsais_unbwt validation.
var ErrInvalidIndex = errors.New("bwt: invalid primary index")

// Encode computes the BWT of in into out (both length n) using sa as
// suffix-array scratch space (len(sa) >= n). It returns the primary index.
func Encode(out, in []byte, sa []int) int32 {
	n := len(in)
	if n == 0 {
		return 0
	}
	if n == 1 {
		out[0] = in[0]
		return 1
	}

	sa = sa[:n]
	sais.ComputeSA(in, sa)

	// Layout matches libsais_bwt: out[0] is the last input byte, the
	// SA row equal to 0 is skipped, and its position + 1 is returned.
	out[0] = in[n-1]
	idx := int32(-1)
	j := 1
	for i := 0; i < n; i++ {
		s := sa[i]
		if s == 0 {
			idx = int32(i) + 1
			continue
		}
		out[j] = in[s-1]
		j++
	}
	return idx
}

// Decode inverts the BWT: in holds the transform (length n), idx is the
// primary index returned by Encode, and out receives the original data
// (length n). lf is scratch space with len(lf) >= n.
//
// Any idx in the valid range decodes without out-of-bounds access; corrupt
// data simply yields wrong output, which the block-level CRC catches.
func Decode(out, in []byte, lf []int32, idx int32) error {
	n := len(in)
	if n <= 1 {
		if int(idx) != n {
			return ErrInvalidIndex
		}
		if n == 1 {
			out[0] = in[0]
		}
		return nil
	}
	if idx <= 0 || int(idx) > n {
		return ErrInvalidIndex
	}

	// The BWT of the sentinel-terminated string is in[:idx] ++ [$] ++
	// in[idx:]. Compute the LF mapping over it; sorted position 0 is the
	// sentinel row, so character buckets start at 1.
	var start [256]int32
	for _, c := range in {
		start[c]++
	}
	sum := int32(1)
	for c := 0; c < 256; c++ {
		t := start[c]
		start[c] = sum
		sum += t
	}
	lf = lf[:n]
	for i, c := range in {
		lf[i] = start[c]
		start[c]++
	}

	// Walk backward from the sentinel row (row 0 holds the last byte).
	p := int(idx)
	r := 0
	for k := n - 1; k >= 0; k-- {
		i := r
		if r >= p {
			i = r - 1
		}
		out[k] = in[i]
		r = int(lf[i])
	}
	return nil
}
