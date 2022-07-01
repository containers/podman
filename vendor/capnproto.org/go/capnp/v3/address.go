package capnp

import (
	"fmt"
)

// An address is an index inside a segment's data (in bytes).
// It is bounded to [0, maxSegmentSize).
type address uint32

// String returns the address in hex format.
func (a address) String() string {
	return fmt.Sprintf("%#08x", uint64(a))
}

// GoString returns the address in hex format.
func (a address) GoString() string {
	return fmt.Sprintf("capnp.address(%#08x)", uint64(a))
}

// addSize returns the address a+sz.  ok is false if the result would
// be an invalid address.
func (a address) addSize(sz Size) (_ address, ok bool) {
	x := int64(a) + int64(sz)
	if x > int64(maxSegmentSize) {
		return 0xffffffff, false
	}
	return address(x), true
}

// addSizeUnchecked returns a+sz without any overflow checking.
func (a address) addSizeUnchecked(sz Size) address {
	return a + address(sz)
}

// element returns the address a+i*sz.  ok is false if the result would
// be an invalid address.
func (a address) element(i int32, sz Size) (_ address, ok bool) {
	x := int64(a) + int64(i)*int64(sz)
	if x > int64(maxSegmentSize) || x < 0 {
		return 0xffffffff, false
	}
	return address(x), true
}

// addOffset returns the address a+o.  It panics if o is invalid.
func (a address) addOffset(o DataOffset) address {
	if o >= 1<<19 {
		panic("data offset overflow")
	}
	return a + address(o)
}

// A Size is a size (in bytes).
type Size uint32

// wordSize is the number of bytes in a Cap'n Proto word.
const wordSize Size = 8

// String returns the size in the format "X bytes".
func (sz Size) String() string {
	if sz == 1 {
		return "1 byte"
	}
	return fmt.Sprintf("%d bytes", sz)
}

// GoString returns the size as a Go expression.
func (sz Size) GoString() string {
	return fmt.Sprintf("capnp.Size(%d)", sz)
}

// times returns the size sz*n.  ok is false if the result would be
// greater than maxSegmentSize.
func (sz Size) times(n int32) (_ Size, ok bool) {
	x := int64(sz) * int64(n)
	if x > int64(maxSegmentSize) || x < 0 {
		return 0xffffffff, false
	}
	return Size(x), true
}

// timesUnchecked returns sz*n without any overflow or negative checking.
func (sz Size) timesUnchecked(n int32) Size {
	return sz * Size(n)
}

// padToWord adds padding to sz to make it divisible by wordSize.
// The result is undefined if sz > maxSegmentSize.
func (sz Size) padToWord() Size {
	n := Size(wordSize - 1)
	return (sz + n) &^ n
}

// maxSegmentSize is the largest size representable in the Cap'n Proto
// encoding.
const maxSegmentSize Size = 1<<32 - 8

// maxAllocSize returns the largest permitted size of a single segment
// on this platform.
//
// Converting between a Size and an int can overflow both ways: on
// systems where int is 32 bits, Size to int overflows, and on systems
// where int is 64 bits, int to Size overflows.  Quantities less than
// or equal to maxAllocSize() will not overflow.
//
// This is effectively a compile-time constant, but can't be represented
// as a constant because it requires a conditional.  It is trivially
// inlinable and optimizable, so should act like one.
func maxAllocSize() Size {
	if maxInt == 0x7fffffff {
		return Size(0x7ffffff8)
	}

	return maxSegmentSize
}

// DataOffset is an offset in bytes from the beginning of a struct's
// data section.  It is bounded to [0, 1<<19).
type DataOffset uint32

// String returns the offset in the format "+X bytes".
func (off DataOffset) String() string {
	if off == 1 {
		return "+1 byte"
	}
	return fmt.Sprintf("+%d bytes", off)
}

// GoString returns the offset as a Go expression.
func (off DataOffset) GoString() string {
	return fmt.Sprintf("capnp.DataOffset(%d)", off)
}

// ObjectSize records section sizes for a struct or list.
type ObjectSize struct {
	DataSize     Size // must be <= 1<<19 - 8
	PointerCount uint16
}

// isZero reports whether sz is the zero size.
func (sz ObjectSize) isZero() bool {
	return sz.DataSize == 0 && sz.PointerCount == 0
}

// isOneByte reports whether the object size is one byte (for Text/Data element sizes).
func (sz ObjectSize) isOneByte() bool {
	return sz.DataSize == 1 && sz.PointerCount == 0
}

// isValid reports whether sz's fields are in range.
func (sz ObjectSize) isValid() bool {
	return sz.DataSize <= 0xffff*wordSize
}

// pointerSize returns the number of bytes the pointer section occupies.
func (sz ObjectSize) pointerSize() Size {
	// Guaranteed not to overflow
	return wordSize * Size(sz.PointerCount)
}

// totalSize returns the number of bytes that the object occupies.
// The range is [0, 0xffff0].
func (sz ObjectSize) totalSize() Size {
	return sz.DataSize + sz.pointerSize()
}

// dataWordCount returns the number of words in the data section.
func (sz ObjectSize) dataWordCount() int32 {
	if sz.DataSize%wordSize != 0 {
		panic("data size not aligned by word")
	}
	return int32(sz.DataSize / wordSize)
}

// totalWordCount returns the number of words that the object occupies.
func (sz ObjectSize) totalWordCount() int32 {
	return sz.dataWordCount() + int32(sz.PointerCount)
}

// String returns a short, human readable representation of the object
// size.
func (sz ObjectSize) String() string {
	return fmt.Sprintf("{datasz=%d ptrs=%d}", sz.DataSize, sz.PointerCount)
}

// GoString formats the ObjectSize as a keyed struct literal.
func (sz ObjectSize) GoString() string {
	return fmt.Sprintf("capnp.ObjectSize{DataSize: %d, PointerCount: %d}", sz.DataSize, sz.PointerCount)
}

// BitOffset is an offset in bits from the beginning of a struct's data
// section.  It is bounded to [0, 1<<22).
type BitOffset uint32

// offset returns the equivalent byte offset.
func (bit BitOffset) offset() DataOffset {
	return DataOffset(bit / 8)
}

// mask returns the bitmask for the bit.
func (bit BitOffset) mask() byte {
	return byte(1 << (bit % 8))
}

// String returns the offset in the format "bit X".
func (bit BitOffset) String() string {
	return fmt.Sprintf("bit %d", bit)
}

// GoString returns the offset as a Go expression.
func (bit BitOffset) GoString() string {
	return fmt.Sprintf("capnp.BitOffset(%d)", bit)
}
