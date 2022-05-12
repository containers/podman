//go:build amd64 && !appengine && !noasm && gc
// +build amd64,!appengine,!noasm,gc

// This file contains the specialisation of Decoder.Decompress4X
// that uses an asm implementation of its main loop.
package huff0

import (
	"errors"
	"fmt"
)

// decompress4x_main_loop_x86 is an x86 assembler implementation
// of Decompress4X when tablelog > 8.
//go:noescape
func decompress4x_main_loop_amd64(ctx *decompress4xContext)

// decompress4x_8b_loop_x86 is an x86 assembler implementation
// of Decompress4X when tablelog <= 8 which decodes 4 entries
// per loop.
//go:noescape
func decompress4x_8b_main_loop_amd64(ctx *decompress4xContext)

// fallback8BitSize is the size where using Go version is faster.
const fallback8BitSize = 800

type decompress4xContext struct {
	pbr0     *bitReaderShifted
	pbr1     *bitReaderShifted
	pbr2     *bitReaderShifted
	pbr3     *bitReaderShifted
	peekBits uint8
	out      *byte
	dstEvery int
	tbl      *dEntrySingle
	decoded  int
	limit    *byte
}

// Decompress4X will decompress a 4X encoded stream.
// The length of the supplied input must match the end of a block exactly.
// The *capacity* of the dst slice must match the destination size of
// the uncompressed data exactly.
func (d *Decoder) Decompress4X(dst, src []byte) ([]byte, error) {
	if len(d.dt.single) == 0 {
		return nil, errors.New("no table loaded")
	}
	if len(src) < 6+(4*1) {
		return nil, errors.New("input too small")
	}

	use8BitTables := d.actualTableLog <= 8
	if cap(dst) < fallback8BitSize && use8BitTables {
		return d.decompress4X8bit(dst, src)
	}

	var br [4]bitReaderShifted
	// Decode "jump table"
	start := 6
	for i := 0; i < 3; i++ {
		length := int(src[i*2]) | (int(src[i*2+1]) << 8)
		if start+length >= len(src) {
			return nil, errors.New("truncated input (or invalid offset)")
		}
		err := br[i].init(src[start : start+length])
		if err != nil {
			return nil, err
		}
		start += length
	}
	err := br[3].init(src[start:])
	if err != nil {
		return nil, err
	}

	// destination, offset to match first output
	dstSize := cap(dst)
	dst = dst[:dstSize]
	out := dst
	dstEvery := (dstSize + 3) / 4

	const tlSize = 1 << tableLogMax
	const tlMask = tlSize - 1
	single := d.dt.single[:tlSize]

	var decoded int

	if len(out) > 4*4 && !(br[0].off < 4 || br[1].off < 4 || br[2].off < 4 || br[3].off < 4) {
		ctx := decompress4xContext{
			pbr0:     &br[0],
			pbr1:     &br[1],
			pbr2:     &br[2],
			pbr3:     &br[3],
			peekBits: uint8((64 - d.actualTableLog) & 63), // see: bitReaderShifted.peekBitsFast()
			out:      &out[0],
			dstEvery: dstEvery,
			tbl:      &single[0],
			limit:    &out[dstEvery-4], // Always stop decoding when first buffer gets here to avoid writing OOB on last.
		}
		if use8BitTables {
			decompress4x_8b_main_loop_amd64(&ctx)
		} else {
			decompress4x_main_loop_amd64(&ctx)
		}

		decoded = ctx.decoded
		out = out[decoded/4:]
	}

	// Decode remaining.
	remainBytes := dstEvery - (decoded / 4)
	for i := range br {
		offset := dstEvery * i
		endsAt := offset + remainBytes
		if endsAt > len(out) {
			endsAt = len(out)
		}
		br := &br[i]
		bitsLeft := br.remaining()
		for bitsLeft > 0 {
			br.fill()
			if offset >= endsAt {
				return nil, errors.New("corruption detected: stream overrun 4")
			}

			// Read value and increment offset.
			val := br.peekBitsFast(d.actualTableLog)
			v := single[val&tlMask].entry
			nBits := uint8(v)
			br.advance(nBits)
			bitsLeft -= uint(nBits)
			out[offset] = uint8(v >> 8)
			offset++
		}
		if offset != endsAt {
			return nil, fmt.Errorf("corruption detected: short output block %d, end %d != %d", i, offset, endsAt)
		}
		decoded += offset - dstEvery*i
		err = br.close()
		if err != nil {
			return nil, err
		}
	}
	if dstSize != decoded {
		return nil, errors.New("corruption detected: short output block")
	}
	return dst, nil
}
