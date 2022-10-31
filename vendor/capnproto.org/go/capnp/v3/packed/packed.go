// Package packed provides functions to read and write the "packed"
// compression scheme described at https://capnproto.org/encoding.html#packing.
package packed

import (
	"bufio"
	"errors"
	"io"
)

const wordSize = 8

// Special case tags.
const (
	zeroTag     byte = 0x00
	unpackedTag byte = 0xff
)

// Pack appends the packed version of src to dst and returns the
// resulting slice.  len(src) must be a multiple of 8 or Pack panics.
func Pack(dst, src []byte) []byte {
	if len(src)%wordSize != 0 {
		panic("packed.Pack len(src) must be a multiple of 8")
	}
	var buf [wordSize]byte
	for len(src) > 0 {
		var hdr byte
		n := 0
		for i := uint(0); i < wordSize; i++ {
			if src[i] != 0 {
				hdr |= 1 << i
				buf[n] = src[i]
				n++
			}
		}
		dst = append(dst, hdr)
		dst = append(dst, buf[:n]...)
		src = src[wordSize:]

		switch hdr {
		case zeroTag:
			z := min(numZeroWords(src), 0xff)
			dst = append(dst, byte(z))
			src = src[z*wordSize:]
		case unpackedTag:
			i := 0
			end := min(len(src), 0xff*wordSize)
			for i < end {
				zeros := 0
				for _, b := range src[i : i+wordSize] {
					if b == 0 {
						zeros++
					}
				}

				if zeros > 1 {
					break
				}
				i += wordSize
			}

			rawWords := byte(i / wordSize)
			dst = append(dst, rawWords)
			dst = append(dst, src[:i]...)
			src = src[i:]
		}
	}
	return dst
}

// numZeroWords returns the number of leading zero words in b.
func numZeroWords(b []byte) int {
	for i, bb := range b {
		if bb != 0 {
			return i / wordSize
		}
	}
	return len(b) / wordSize
}

// Unpack appends the unpacked version of src to dst and returns the
// resulting slice.
func Unpack(dst, src []byte) ([]byte, error) {
	for len(src) > 0 {
		tag := src[0]
		src = src[1:]

		pstart := len(dst)
		dst = allocWords(dst, 1)
		p := dst[pstart : pstart+wordSize]
		if len(src) >= wordSize {
			i := 0
			nz := tag & 1
			p[0] = src[i] & -nz
			i += int(nz)
			nz = tag >> 1 & 1
			p[1] = src[i] & -nz
			i += int(nz)
			nz = tag >> 2 & 1
			p[2] = src[i] & -nz
			i += int(nz)
			nz = tag >> 3 & 1
			p[3] = src[i] & -nz
			i += int(nz)
			nz = tag >> 4 & 1
			p[4] = src[i] & -nz
			i += int(nz)
			nz = tag >> 5 & 1
			p[5] = src[i] & -nz
			i += int(nz)
			nz = tag >> 6 & 1
			p[6] = src[i] & -nz
			i += int(nz)
			nz = tag >> 7 & 1
			p[7] = src[i] & -nz
			i += int(nz)
			src = src[i:]
		} else {
			for i := uint(0); i < wordSize; i++ {
				if tag&(1<<i) == 0 {
					continue
				}
				if len(src) == 0 {
					return dst, io.ErrUnexpectedEOF
				}
				p[i] = src[0]
				src = src[1:]
			}
		}
		switch tag {
		case zeroTag:
			if len(src) == 0 {
				return dst, io.ErrUnexpectedEOF
			}
			dst = allocWords(dst, int(src[0]))
			src = src[1:]
		case unpackedTag:
			if len(src) == 0 {
				return dst, io.ErrUnexpectedEOF
			}
			start := len(dst)
			dst = allocWords(dst, int(src[0]))
			src = src[1:]
			n := copy(dst[start:], src)
			src = src[n:]
		}
	}
	return dst, nil
}

func allocWords(p []byte, n int) []byte {
	target := len(p) + n*wordSize
	if cap(p) >= target {
		pp := p[len(p):target]
		for i := range pp {
			pp[i] = 0
		}
		return p[:target]
	}
	newcap := cap(p)
	doublecap := newcap + newcap
	if target > doublecap {
		newcap = target
	} else {
		if len(p) < 1024 {
			newcap = doublecap
		} else {
			for newcap < target {
				newcap += newcap / 4
			}
		}
	}
	pp := make([]byte, target, newcap)
	copy(pp, p)
	return pp
}

// A Reader decompresses a packed byte stream.
type Reader struct {
	// ReadWord state
	rd      *bufio.Reader
	err     error
	zeroes  int
	literal int

	// Read state
	word    [wordSize]byte
	wordIdx int
}

// NewReader returns a reader that decompresses a packed stream from r.
func NewReader(r *bufio.Reader) *Reader {
	return &Reader{rd: r, wordIdx: wordSize}
}

func min(a, b int) int {
	if b < a {
		return b
	}
	return a
}

// ReadWord decompresses the next word from the underlying stream.
func (r *Reader) ReadWord(p []byte) error {
	if len(p) < wordSize {
		return errors.New("packed: read word buffer too small")
	}
	r.wordIdx = wordSize // if the caller tries to call ReadWord and Read, don't give them partial words.
	if r.err != nil {
		err := r.err
		r.err = nil
		return err
	}
	p = p[:wordSize]
	switch {
	case r.zeroes > 0:
		r.zeroes--
		for i := range p {
			p[i] = 0
		}
		return nil
	case r.literal > 0:
		r.literal--
		_, err := io.ReadFull(r.rd, p)
		return err
	}

	var tag byte
	if r.rd.Buffered() < wordSize+1 {
		var err error
		tag, err = r.rd.ReadByte()
		if err != nil {
			return err
		}
		for i := range p {
			p[i] = 0
		}
		for i := uint(0); i < wordSize; i++ {
			if tag&(1<<i) != 0 {
				p[i], err = r.rd.ReadByte()
				if err != nil {
					if err == io.EOF {
						err = io.ErrUnexpectedEOF
					}
					return err
				}
			} else {
				p[i] = 0
			}
		}
	} else {
		b, _ := r.rd.Peek(wordSize + 1)
		tag = b[0]
		i := 1
		nz := tag & 1
		p[0] = b[i] & -nz
		i += int(nz)
		nz = tag >> 1 & 1
		p[1] = b[i] & -nz
		i += int(nz)
		nz = tag >> 2 & 1
		p[2] = b[i] & -nz
		i += int(nz)
		nz = tag >> 3 & 1
		p[3] = b[i] & -nz
		i += int(nz)
		nz = tag >> 4 & 1
		p[4] = b[i] & -nz
		i += int(nz)
		nz = tag >> 5 & 1
		p[5] = b[i] & -nz
		i += int(nz)
		nz = tag >> 6 & 1
		p[6] = b[i] & -nz
		i += int(nz)
		nz = tag >> 7 & 1
		p[7] = b[i] & -nz
		i += int(nz)
		r.rd.Discard(i)
	}
	switch tag {
	case zeroTag:
		z, err := r.rd.ReadByte()
		if err == io.EOF {
			r.err = io.ErrUnexpectedEOF
			return nil
		} else if err != nil {
			r.err = err
			return nil
		}
		r.zeroes = int(z)
	case unpackedTag:
		l, err := r.rd.ReadByte()
		if err == io.EOF {
			r.err = io.ErrUnexpectedEOF
			return nil
		} else if err != nil {
			r.err = err
			return nil
		}
		r.literal = int(l)
	}
	return nil
}

// Read reads up to len(p) bytes into p.  This will decompress whole
// words at a time, so mixing calls to Read and ReadWord may lead to
// bytes missing.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.wordIdx < wordSize {
		n = copy(p, r.word[r.wordIdx:])
		r.wordIdx += n
	}
	for n < len(p) {
		if r.rd.Buffered() < wordSize+1 && n > 0 {
			return n, nil
		}
		if len(p)-n >= wordSize {
			err := r.ReadWord(p[n:])
			if err != nil {
				return n, err
			}
			n += wordSize
		} else {
			err := r.ReadWord(r.word[:])
			if err != nil {
				return n, err
			}
			r.wordIdx = copy(p[n:], r.word[:])
			n += r.wordIdx
		}
	}
	return n, nil
}

type Writer struct {
	io.Writer
	buf []byte
}

func (w *Writer) Write(b []byte) (int, error) {
	w.buf = Pack(w.buf[:0], b)
	return w.Writer.Write(w.buf)
}
