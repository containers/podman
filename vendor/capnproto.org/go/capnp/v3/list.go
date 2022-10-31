package capnp

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"capnproto.org/go/capnp/v3/internal/strquote"
)

// A List is a reference to an array of values.
type List ListKind

// The underlying type of List. We expose this so that
// we can use ~ListKind as a constraint in generics to
// capture any list type.
type ListKind = struct {
	seg        *Segment
	off        address    // at beginning of elements (past composite list tag word)
	length     int32      // always [0, 1<<29)
	size       ObjectSize // always valid
	depthLimit uint
	flags      listFlags
}

// newPrimitiveList allocates a new list of primitive values, preferring placement in s.
func newPrimitiveList(s *Segment, sz Size, n int32) (List, error) {
	if n < 0 || n >= 1<<29 {
		return List{}, errorf("new list: length out of range")
	}
	// sz is [0, 8] and n is [0, 1<<29).
	// Range is [0, maxSegmentSize], thus there will never be overflow.
	total := sz.timesUnchecked(n)
	s, addr, err := alloc(s, total)
	if err != nil {
		return List{}, annotatef(err, "new list")
	}
	return List{
		seg:        s,
		off:        addr,
		length:     n,
		size:       ObjectSize{DataSize: sz},
		depthLimit: maxDepth,
	}, nil
}

// NewCompositeList creates a new composite list, preferring placement
// in s.
func NewCompositeList(s *Segment, sz ObjectSize, n int32) (List, error) {
	if !sz.isValid() {
		return List{}, errorf("new composite list: invalid element size")
	}
	if n < 0 || n >= 1<<29 {
		return List{}, errorf("new composite list: length out of range")
	}
	sz.DataSize = sz.DataSize.padToWord()
	total, ok := sz.totalSize().times(n)
	if !ok || total > maxSegmentSize-wordSize {
		return List{}, errorf("new composite list: size overflow")
	}
	s, addr, err := alloc(s, wordSize+total)
	if err != nil {
		return List{}, annotatef(err, "new composite list")
	}
	// Add tag word
	s.writeRawPointer(addr, rawStructPointer(pointerOffset(n), sz))
	return List{
		seg:        s,
		off:        addr.addSizeUnchecked(wordSize),
		length:     n,
		size:       sz,
		flags:      isCompositeList,
		depthLimit: maxDepth,
	}, nil
}

// ToPtr converts the list to a generic pointer.
func (p List) ToPtr() Ptr {
	return Ptr{
		seg:        p.seg,
		off:        p.off,
		lenOrCap:   uint32(p.length),
		size:       p.size,
		depthLimit: p.depthLimit,
		flags:      listPtrFlag(p.flags),
	}
}

// Segment returns the segment the referenced list is stored in or
// nil if the pointer is invalid.
func (p List) Segment() *Segment {
	return p.seg
}

// Message returns the message the referenced list is stored in or nil
// if the pointer is invalid.
func (p List) Message() *Message {
	if p.seg == nil {
		return nil
	}
	return p.seg.msg
}

// IsValid returns whether the list is valid.
func (p List) IsValid() bool {
	return p.seg != nil
}

// readSize returns the list's size for the purposes of read limit
// accounting.
func (p List) readSize() Size {
	if p.seg == nil {
		return 0
	}
	e := p.size.totalSize()
	if e == 0 {
		e = wordSize
	}
	sz, ok := e.times(p.length)
	if !ok {
		return maxSegmentSize
	}
	return sz
}

// allocSize returns the list's size for the purpose of copying the list
// to a different message.
func (p List) allocSize() Size {
	if p.seg == nil {
		return 0
	}
	if p.flags&isBitList != 0 {
		return bitListSize(p.length)
	}
	sz, _ := p.size.totalSize().times(p.length) // size has already been validated
	if p.flags&isCompositeList == 0 {
		return sz
	}
	return sz + wordSize
}

// raw returns the equivalent raw list pointer with a zero offset.
func (p List) raw() rawPointer {
	if p.seg == nil {
		return 0
	}
	if p.flags&isCompositeList != 0 {
		return rawListPointer(0, compositeList, p.length*p.size.totalWordCount())
	}
	if p.flags&isBitList != 0 {
		return rawListPointer(0, bit1List, p.length)
	}
	if p.size.PointerCount == 1 && p.size.DataSize == 0 {
		return rawListPointer(0, pointerList, p.length)
	}
	if p.size.PointerCount != 0 {
		panic("invalid list size")
	}
	switch p.size.DataSize {
	case 0:
		return rawListPointer(0, voidList, p.length)
	case 1:
		return rawListPointer(0, byte1List, p.length)
	case 2:
		return rawListPointer(0, byte2List, p.length)
	case 4:
		return rawListPointer(0, byte4List, p.length)
	case 8:
		return rawListPointer(0, byte8List, p.length)
	default:
		panic("invalid list size")
	}
}

// Len returns the length of the list.
func (p List) Len() int {
	if p.seg == nil {
		return 0
	}
	return int(p.length)
}

// primitiveElem returns the address of the segment data for a list element.
// Calling this on a bit list returns an error.
func (p List) primitiveElem(i int, expectedSize ObjectSize) (address, error) {
	if p.seg == nil || i < 0 || i >= int(p.length) {
		// This is programmer error, not input error.
		panic("list element out of bounds")
	}
	if p.flags&isBitList != 0 || p.flags&isCompositeList == 0 && p.size != expectedSize || p.flags&isCompositeList != 0 && (p.size.DataSize < expectedSize.DataSize || p.size.PointerCount < expectedSize.PointerCount) {
		return 0, errorf("mismatched list element size")
	}
	addr, ok := p.off.element(int32(i), p.size.totalSize())
	if !ok {
		return 0, errorf("read list element %d: address overflow", i)
	}
	return addr, nil
}

// Struct returns the i'th element as a struct.
func (p List) Struct(i int) Struct {
	if p.seg == nil || i < 0 || i >= int(p.length) {
		// This is programmer error, not input error.
		panic("list element out of bounds")
	}
	if p.flags&isBitList != 0 {
		return Struct{}
	}
	addr, ok := p.off.element(int32(i), p.size.totalSize())
	if !ok {
		return Struct{}
	}
	return Struct{
		seg:        p.seg,
		off:        addr,
		size:       p.size,
		flags:      isListMember,
		depthLimit: p.depthLimit - 1,
	}
}

// SetStruct set the i'th element to the value in s.
func (p List) SetStruct(i int, s Struct) error {
	if p.flags&isBitList != 0 {
		return errorf("SetStruct called on bit list")
	}
	if err := copyStruct(p.Struct(i), s); err != nil {
		return annotatef(err, "set list element %d", i)
	}
	return nil
}

// l.EncodeAsPtr is equivalent to l.ToPtr(); for implementing TypeParam.
// The segment argument is ignored.
func (l List) EncodeAsPtr(*Segment) Ptr { return l.ToPtr() }

// DecodeFromPtr(p) is equivalent to p.List; for implementing TypeParam.
func (List) DecodeFromPtr(p Ptr) List { return p.List() }

var _ TypeParam[List] = List{}

// A BitList is a reference to a list of booleans.
type BitList List

var _ TypeParam[BitList] = BitList{}

// NewBitList creates a new bit list, preferring placement in s.
func NewBitList(s *Segment, n int32) (BitList, error) {
	if n < 0 || n >= 1<<29 {
		return BitList{}, errorf("new bit list: length out of range")
	}
	s, addr, err := alloc(s, bitListSize(n))
	if err != nil {
		return BitList{}, annotatef(err, "new %d-element bit list", n)
	}
	return BitList{
		seg:        s,
		off:        addr,
		length:     n,
		flags:      isBitList,
		depthLimit: maxDepth,
	}, nil
}

// bitListSize returns the number of bytes needed for a bit list with n
// elements.  It is only defined for n in [0, 1<<29).
func bitListSize(n int32) Size {
	return Size((n + 7) / 8)
}

// At returns the i'th bit.
func (p BitList) At(i int) bool {
	if p.seg == nil || i < 0 || i >= int(p.length) {
		// This is programmer error, not input error.
		panic("list element out of bounds")
	}
	if p.flags&isBitList == 0 {
		return false
	}
	bit := BitOffset(i)
	addr := p.off.addOffset(bit.offset())
	return p.seg.readUint8(addr)&bit.mask() != 0
}

// Set sets the i'th bit to v.
func (p BitList) Set(i int, v bool) {
	if p.seg == nil || i < 0 || i >= int(p.length) {
		// This is programmer error, not input error.
		panic("list element out of bounds")
	}
	if p.flags&isBitList == 0 {
		// Again, programmer error.  Should have used NewBitList.
		panic("BitList.Set called on a non-bit list")
	}
	bit := BitOffset(i)
	addr := p.off.addOffset(bit.offset())
	b := p.seg.slice(addr, 1)
	if v {
		b[0] |= bit.mask()
	} else {
		b[0] &^= bit.mask()
	}
}

// String returns the list in Cap'n Proto schema format (e.g. "[true, false]").
func (p BitList) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < p.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		if p.At(i) {
			buf = append(buf, "true"...)
		} else {
			buf = append(buf, "false"...)
		}
	}
	buf = append(buf, ']')
	return string(buf)
}

// A PointerList is a reference to an array of pointers.
type PointerList List

var _ TypeParam[PointerList] = PointerList{}

// NewPointerList allocates a new list of pointers, preferring placement in s.
func NewPointerList(s *Segment, n int32) (PointerList, error) {
	total, ok := wordSize.times(n)
	if !ok {
		return PointerList{}, errorf("new pointer list: size overflow")
	}
	s, addr, err := alloc(s, total)
	if err != nil {
		return PointerList{}, annotatef(err, "new %d-element pointer list", n)
	}
	return PointerList{
		seg:        s,
		off:        addr,
		length:     n,
		size:       ObjectSize{PointerCount: 1},
		depthLimit: maxDepth,
	}, nil
}

// At returns the i'th pointer in the list.
func (p PointerList) At(i int) (Ptr, error) {
	addr, err := p.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return Ptr{}, err
	}
	return p.seg.readPtr(addr, p.depthLimit)
}

// Set sets the i'th pointer in the list to v.
func (p PointerList) Set(i int, v Ptr) error {
	addr, err := p.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return err
	}
	return p.seg.writePtr(addr, v, false)
}

// TextList is an array of pointers to strings.
type TextList List

var _ TypeParam[TextList] = TextList{}

// NewTextList allocates a new list of text pointers, preferring placement in s.
func NewTextList(s *Segment, n int32) (TextList, error) {
	pl, err := NewPointerList(s, n)
	if err != nil {
		return TextList{}, err
	}
	return TextList(pl), nil
}

// At returns the i'th string in the list.
func (l TextList) At(i int) (string, error) {
	addr, err := l.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return "", err
	}
	p, err := l.seg.readPtr(addr, l.depthLimit)
	if err != nil {
		return "", err
	}
	return p.Text(), nil
}

// BytesAt returns the i'th element in the list as a byte slice.
// The underlying array of the slice is the segment data.
func (l TextList) BytesAt(i int) ([]byte, error) {
	addr, err := l.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return nil, err
	}
	p, err := l.seg.readPtr(addr, l.depthLimit)
	if err != nil {
		return nil, err
	}
	return p.TextBytes(), nil
}

// Set sets the i'th string in the list to v.
func (l TextList) Set(i int, v string) error {
	addr, err := l.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return err
	}
	if v == "" {
		return l.seg.writePtr(addr, Ptr{}, false)
	}
	p, err := NewText(l.seg, v)
	if err != nil {
		return err
	}
	return l.seg.writePtr(addr, p.ToPtr(), false)
}

// String returns the list in Cap'n Proto schema format (e.g. `["foo", "bar"]`).
func (l TextList) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		s, err := l.BytesAt(i)
		if err != nil {
			buf = append(buf, "<error>"...)
			continue
		}
		buf = strquote.Append(buf, s)
	}
	buf = append(buf, ']')
	return string(buf)
}

// DataList is an array of pointers to data.
type DataList List

var _ TypeParam[DataList] = DataList{}

// NewDataList allocates a new list of data pointers, preferring placement in s.
func NewDataList(s *Segment, n int32) (DataList, error) {
	pl, err := NewPointerList(s, n)
	if err != nil {
		return DataList{}, err
	}
	return DataList(pl), nil
}

// At returns the i'th data in the list.
func (l DataList) At(i int) ([]byte, error) {
	addr, err := l.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return nil, err
	}
	p, err := l.seg.readPtr(addr, l.depthLimit)
	if err != nil {
		return nil, err
	}
	return p.Data(), nil
}

// Set sets the i'th data in the list to v.
func (l DataList) Set(i int, v []byte) error {
	addr, err := l.primitiveElem(i, ObjectSize{PointerCount: 1})
	if err != nil {
		return err
	}
	if len(v) == 0 {
		return l.seg.writePtr(addr, Ptr{}, false)
	}
	p, err := NewData(l.seg, v)
	if err != nil {
		return err
	}
	return l.seg.writePtr(addr, p.ToPtr(), false)
}

// String returns the list in Cap'n Proto schema format (e.g. `["foo", "bar"]`).
func (l DataList) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		s, err := l.At(i)
		if err != nil {
			buf = append(buf, "<error>"...)
			continue
		}
		buf = strquote.Append(buf, s)
	}
	buf = append(buf, ']')
	return string(buf)
}

// A VoidList is a list of zero-sized elements.
type VoidList List

var _ TypeParam[VoidList] = VoidList{}

// NewVoidList creates a list of voids.  No allocation is performed;
// s is only used for Segment()'s return value.
func NewVoidList(s *Segment, n int32) VoidList {
	if n < 0 || n >= 1<<29 {
		panic("list length overflow")
	}
	return VoidList{
		seg:        s,
		length:     n,
		depthLimit: maxDepth,
	}
}

// String returns the list in Cap'n Proto schema format (e.g. "[void, void, void]").
func (l VoidList) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = append(buf, "void"...)
	}
	buf = append(buf, ']')
	return string(buf)
}

// A UInt8List is an array of UInt8 values.
type UInt8List List

var _ TypeParam[UInt8List] = UInt8List{}

// NewUInt8List creates a new list of UInt8, preferring placement in s.
func NewUInt8List(s *Segment, n int32) (UInt8List, error) {
	l, err := newPrimitiveList(s, 1, n)
	if err != nil {
		return UInt8List{}, err
	}
	return UInt8List(l), nil
}

// NewText creates a new list of UInt8 from a string.
func NewText(s *Segment, v string) (UInt8List, error) {
	// TODO(light): error if v is too long
	l, err := NewUInt8List(s, int32(len(v)+1))
	if err != nil {
		return UInt8List{}, err
	}
	copy(l.seg.slice(l.off, Size(len(v))), v)
	return l, nil
}

// NewTextFromBytes creates a NUL-terminated list of UInt8 from a byte slice.
func NewTextFromBytes(s *Segment, v []byte) (UInt8List, error) {
	// TODO(light): error if v is too long
	l, err := NewUInt8List(s, int32(len(v)+1))
	if err != nil {
		return UInt8List{}, err
	}
	copy(l.seg.slice(l.off, Size(len(v))), v)
	return l, nil
}

// NewData creates a new list of UInt8 from a byte slice.
func NewData(s *Segment, v []byte) (UInt8List, error) {
	// TODO(light): error if v is too long
	l, err := NewUInt8List(s, int32(len(v)))
	if err != nil {
		return UInt8List{}, err
	}
	copy(l.seg.slice(l.off, Size(len(v))), v)
	return l, nil
}

func isOneByteList(p Ptr) bool {
	return p.seg != nil && p.flags.ptrType() == listPtrType && p.size.isOneByte() && p.flags.listFlags()&isCompositeList == 0
}

// At returns the i'th element.
func (l UInt8List) At(i int) uint8 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 1})
	if err != nil {
		return 0
	}
	return l.seg.readUint8(addr)
}

// Set sets the i'th element to v.
func (l UInt8List) Set(i int, v uint8) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 1})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint8(addr, v)
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l UInt8List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendUint(buf, uint64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Int8List is an array of Int8 values.
type Int8List List

var _ TypeParam[Int8List] = Int8List{}

// NewInt8List creates a new list of Int8, preferring placement in s.
func NewInt8List(s *Segment, n int32) (Int8List, error) {
	l, err := newPrimitiveList(s, 1, n)
	if err != nil {
		return Int8List{}, err
	}
	return Int8List(l), nil
}

// At returns the i'th element.
func (l Int8List) At(i int) int8 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 1})
	if err != nil {
		return 0
	}
	return int8(l.seg.readUint8(addr))
}

// Set sets the i'th element to v.
func (l Int8List) Set(i int, v int8) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 1})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint8(addr, uint8(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Int8List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendInt(buf, int64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// A UInt16List is an array of UInt16 values.
type UInt16List List

var _ TypeParam[UInt16List] = UInt16List{}

// NewUInt16List creates a new list of UInt16, preferring placement in s.
func NewUInt16List(s *Segment, n int32) (UInt16List, error) {
	l, err := newPrimitiveList(s, 2, n)
	if err != nil {
		return UInt16List{}, err
	}
	return UInt16List(l), nil
}

// At returns the i'th element.
func (l UInt16List) At(i int) uint16 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 2})
	if err != nil {
		return 0
	}
	return l.seg.readUint16(addr)
}

// Set sets the i'th element to v.
func (l UInt16List) Set(i int, v uint16) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 2})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint16(addr, v)
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l UInt16List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendUint(buf, uint64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Int16List is an array of Int16 values.
type Int16List List

var _ TypeParam[Int16List] = Int16List{}

// NewInt16List creates a new list of Int16, preferring placement in s.
func NewInt16List(s *Segment, n int32) (Int16List, error) {
	l, err := newPrimitiveList(s, 2, n)
	if err != nil {
		return Int16List{}, err
	}
	return Int16List(l), nil
}

// At returns the i'th element.
func (l Int16List) At(i int) int16 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 2})
	if err != nil {
		return 0
	}
	return int16(l.seg.readUint16(addr))
}

// Set sets the i'th element to v.
func (l Int16List) Set(i int, v int16) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 2})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint16(addr, uint16(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Int16List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendInt(buf, int64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// UInt32List is an array of UInt32 values.
type UInt32List List

var _ TypeParam[UInt32List] = UInt32List{}

// NewUInt32List creates a new list of UInt32, preferring placement in s.
func NewUInt32List(s *Segment, n int32) (UInt32List, error) {
	l, err := newPrimitiveList(s, 4, n)
	if err != nil {
		return UInt32List{}, err
	}
	return UInt32List(l), nil
}

// At returns the i'th element.
func (l UInt32List) At(i int) uint32 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		return 0
	}
	return l.seg.readUint32(addr)
}

// Set sets the i'th element to v.
func (l UInt32List) Set(i int, v uint32) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint32(addr, v)
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l UInt32List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendUint(buf, uint64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Int32List is an array of Int32 values.
type Int32List List

var _ TypeParam[Int32List] = Int32List{}

// NewInt32List creates a new list of Int32, preferring placement in s.
func NewInt32List(s *Segment, n int32) (Int32List, error) {
	l, err := newPrimitiveList(s, 4, n)
	if err != nil {
		return Int32List{}, err
	}
	return Int32List(l), nil
}

// At returns the i'th element.
func (l Int32List) At(i int) int32 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		return 0
	}
	return int32(l.seg.readUint32(addr))
}

// Set sets the i'th element to v.
func (l Int32List) Set(i int, v int32) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint32(addr, uint32(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Int32List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendInt(buf, int64(l.At(i)), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// UInt64List is an array of UInt64 values.
type UInt64List List

var _ TypeParam[UInt64List] = UInt64List{}

// NewUInt64List creates a new list of UInt64, preferring placement in s.
func NewUInt64List(s *Segment, n int32) (UInt64List, error) {
	l, err := newPrimitiveList(s, 8, n)
	if err != nil {
		return UInt64List{}, err
	}
	return UInt64List(l), nil
}

// At returns the i'th element.
func (l UInt64List) At(i int) uint64 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		return 0
	}
	return l.seg.readUint64(addr)
}

// Set sets the i'th element to v.
func (l UInt64List) Set(i int, v uint64) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint64(addr, v)
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l UInt64List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendUint(buf, l.At(i), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Int64List is an array of Int64 values.
type Int64List List

var _ TypeParam[Int64List] = Int64List{}

// NewInt64List creates a new list of Int64, preferring placement in s.
func NewInt64List(s *Segment, n int32) (Int64List, error) {
	l, err := newPrimitiveList(s, 8, n)
	if err != nil {
		return Int64List{}, err
	}
	return Int64List(l), nil
}

// At returns the i'th element.
func (l Int64List) At(i int) int64 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		return 0
	}
	return int64(l.seg.readUint64(addr))
}

// Set sets the i'th element to v.
func (l Int64List) Set(i int, v int64) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint64(addr, uint64(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Int64List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendInt(buf, l.At(i), 10)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Float32List is an array of Float32 values.
type Float32List List

var _ TypeParam[Float32List] = Float32List{}

// NewFloat32List creates a new list of Float32, preferring placement in s.
func NewFloat32List(s *Segment, n int32) (Float32List, error) {
	l, err := newPrimitiveList(s, 4, n)
	if err != nil {
		return Float32List{}, err
	}
	return Float32List(l), nil
}

// At returns the i'th element.
func (l Float32List) At(i int) float32 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		return 0
	}
	return math.Float32frombits(l.seg.readUint32(addr))
}

// Set sets the i'th element to v.
func (l Float32List) Set(i int, v float32) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 4})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint32(addr, math.Float32bits(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Float32List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendFloat(buf, float64(l.At(i)), 'g', -1, 32)
	}
	buf = append(buf, ']')
	return string(buf)
}

// Float64List is an array of Float64 values.
type Float64List List

var _ TypeParam[Float64List] = Float64List{}

// NewFloat64List creates a new list of Float64, preferring placement in s.
func NewFloat64List(s *Segment, n int32) (Float64List, error) {
	l, err := newPrimitiveList(s, 8, n)
	if err != nil {
		return Float64List{}, err
	}
	return Float64List(l), nil
}

// At returns the i'th element.
func (l Float64List) At(i int) float64 {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		return 0
	}
	return math.Float64frombits(l.seg.readUint64(addr))
}

// Set sets the i'th element to v.
func (l Float64List) Set(i int, v float64) {
	addr, err := l.primitiveElem(i, ObjectSize{DataSize: 8})
	if err != nil {
		panic(err)
	}
	l.seg.writeUint64(addr, math.Float64bits(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l Float64List) String() string {
	var buf []byte
	buf = append(buf, '[')
	for i := 0; i < l.Len(); i++ {
		if i > 0 {
			buf = append(buf, ", "...)
		}
		buf = strconv.AppendFloat(buf, l.At(i), 'g', -1, 64)
	}
	buf = append(buf, ']')
	return string(buf)
}

// A list of some Cap'n Proto enum type T.
type EnumList[T ~uint16] UInt16List

var _ TypeParam[EnumList[uint16]] = EnumList[uint16]{}

// NewEnumList creates a new list of T, preferring placement in s.
func NewEnumList[T ~uint16](s *Segment, n int32) (EnumList[T], error) {
	l, err := NewUInt16List(s, n)
	return EnumList[T](l), err
}

// At returns the i'th element.
func (l EnumList[T]) At(i int) T {
	return T(UInt16List(l).At(i))
}

// Set sets the i'th element to v.
func (l EnumList[T]) Set(i int, v T) {
	UInt16List(l).Set(i, uint16(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[1, 2, 3]").
func (l EnumList[T]) String() string {
	return UInt16List(l).String()
}

// A list of some Cap'n Proto struct type T.
type StructList[T ~StructKind] List

var _ TypeParam[StructList[Struct]] = StructList[Struct]{}

// At returns the i'th element.
func (s StructList[T]) At(i int) T {
	return T(List(s).Struct(i))
}

// Set sets the i'th element to v.
func (s StructList[T]) Set(i int, v T) error {
	return List(s).SetStruct(i, Struct(v))
}

// String returns the list in Cap'n Proto schema format (e.g. "[(x = 1), (x = 2)]").
func (s StructList[T]) String() string {
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i := 0; i < s.Len(); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprint(buf, s.At(i))
	}
	buf.WriteByte(']')
	return buf.String()
}

// A list of some Cap'n Proto capability type T.
type CapList[T ~ClientKind] PointerList

func (c CapList[T]) At(i int) (T, error) {
	ptr, err := PointerList(c).At(i)
	if err != nil {
		return T{}, err
	}
	return T(ptr.Interface().Client()), nil
}

func (c CapList[T]) Set(i int, v T) error {
	pl := PointerList(c)
	seg := pl.Segment()
	capId := seg.Message().AddCap(Client(v))
	return pl.Set(i, NewInterface(seg, capId).ToPtr())
}

type listFlags uint8

const (
	isCompositeList listFlags = 1 << iota
	isBitList
)
