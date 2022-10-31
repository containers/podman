package capnp

// Struct is a pointer to a struct.
type Struct StructKind

// The underlying type of Struct. We expose this so that
// we can use ~StructKind as a constraint in generics to
// capture any struct type.
type StructKind = struct {
	seg        *Segment
	off        address
	size       ObjectSize
	depthLimit uint
	flags      structFlags
}

// NewStruct creates a new struct, preferring placement in s.
func NewStruct(s *Segment, sz ObjectSize) (Struct, error) {
	if !sz.isValid() {
		return Struct{}, errorf("new struct: invalid size")
	}
	sz.DataSize = sz.DataSize.padToWord()
	seg, addr, err := alloc(s, sz.totalSize())
	if err != nil {
		return Struct{}, annotatef(err, "new struct")
	}
	return Struct{
		seg:        seg,
		off:        addr,
		size:       sz,
		depthLimit: maxDepth,
	}, nil
}

// NewRootStruct creates a new struct, preferring placement in s, then sets the
// message's root to the new struct.
func NewRootStruct(s *Segment, sz ObjectSize) (Struct, error) {
	st, err := NewStruct(s, sz)
	if err != nil {
		return st, err
	}
	if err := s.msg.SetRoot(st.ToPtr()); err != nil {
		return st, err
	}
	return st, nil
}

// ToPtr converts the struct to a generic pointer.
func (p Struct) ToPtr() Ptr {
	return Ptr{
		seg:        p.seg,
		off:        p.off,
		size:       p.size,
		depthLimit: p.depthLimit,
		flags:      structPtrFlag(p.flags),
	}
}

// Segment returns the segment the referenced struct is stored in or nil
// if the pointer is invalid.
func (p Struct) Segment() *Segment {
	return p.seg
}

// Message returns the message the referenced struct is stored in or nil
// if the pointer is invalid.
func (p Struct) Message() *Message {
	if p.seg == nil {
		return nil
	}
	return p.seg.msg
}

// IsValid returns whether the struct is valid.
func (p Struct) IsValid() bool {
	return p.seg != nil
}

// Size returns the size of the struct.
func (p Struct) Size() ObjectSize {
	return p.size
}

// CopyFrom copies content from another struct.  If the other struct's
// sections are larger than this struct's, the extra data is not copied,
// meaning there is a risk of data loss when copying from messages built
// with future versions of the protocol.
func (p Struct) CopyFrom(other Struct) error {
	if err := copyStruct(p, other); err != nil {
		return annotatef(err, "copy struct")
	}
	return nil
}

// readSize returns the struct's size for the purposes of read limit
// accounting.
func (p Struct) readSize() Size {
	if p.seg == nil {
		return 0
	}
	return p.size.totalSize()
}

// Ptr returns the i'th pointer in the struct.
func (p Struct) Ptr(i uint16) (Ptr, error) {
	if p.seg == nil || i >= p.size.PointerCount {
		return Ptr{}, nil
	}
	return p.seg.readPtr(p.pointerAddress(i), p.depthLimit)
}

// HasPtr reports whether the i'th pointer in the struct is non-null.
// It does not affect the read limit.
func (p Struct) HasPtr(i uint16) bool {
	if p.seg == nil || i >= p.size.PointerCount {
		return false
	}
	return p.seg.readRawPointer(p.pointerAddress(i)) != 0
}

// SetPtr sets the i'th pointer in the struct to src.
func (p Struct) SetPtr(i uint16, src Ptr) error {
	if p.seg == nil || i >= p.size.PointerCount {
		panic("capnp: set field outside struct boundaries")
	}
	return p.seg.writePtr(p.pointerAddress(i), src, false)
}

// SetText sets the i'th pointer to a newly allocated text or null if v is empty.
func (p Struct) SetText(i uint16, v string) error {
	if v == "" {
		return p.SetPtr(i, Ptr{})
	}
	return p.SetNewText(i, v)
}

// SetNewText sets the i'th pointer to a newly allocated text.
func (p Struct) SetNewText(i uint16, v string) error {
	t, err := NewText(p.seg, v)
	if err != nil {
		return err
	}
	return p.SetPtr(i, t.ToPtr())
}

// SetTextFromBytes sets the i'th pointer to a newly allocated text or null if v is nil.
func (p Struct) SetTextFromBytes(i uint16, v []byte) error {
	if v == nil {
		return p.SetPtr(i, Ptr{})
	}
	t, err := NewTextFromBytes(p.seg, v)
	if err != nil {
		return err
	}
	return p.SetPtr(i, t.ToPtr())
}

// SetData sets the i'th pointer to a newly allocated data or null if v is nil.
func (p Struct) SetData(i uint16, v []byte) error {
	if v == nil {
		return p.SetPtr(i, Ptr{})
	}
	d, err := NewData(p.seg, v)
	if err != nil {
		return err
	}
	return p.SetPtr(i, d.ToPtr())
}

func (p Struct) pointerAddress(i uint16) address {
	// Struct already had bounds check
	ptrStart, _ := p.off.addSize(p.size.DataSize)
	a, _ := ptrStart.element(int32(i), wordSize)
	return a
}

// bitInData reports whether bit is inside p's data section.
func (p Struct) bitInData(bit BitOffset) bool {
	return p.seg != nil && bit < BitOffset(p.size.DataSize*8)
}

// Bit returns the bit that is n bits from the start of the struct.
func (p Struct) Bit(n BitOffset) bool {
	if !p.bitInData(n) {
		return false
	}
	addr := p.off.addOffset(n.offset())
	return p.seg.readUint8(addr)&n.mask() != 0
}

// SetBit sets the bit that is n bits from the start of the struct to v.
func (p Struct) SetBit(n BitOffset, v bool) {
	if !p.bitInData(n) {
		panic("capnp: set field outside struct boundaries")
	}
	addr := p.off.addOffset(n.offset())
	b := p.seg.readUint8(addr)
	if v {
		b |= n.mask()
	} else {
		b &^= n.mask()
	}
	p.seg.writeUint8(addr, b)
}

func (p Struct) dataAddress(off DataOffset, sz Size) (addr address, ok bool) {
	if p.seg == nil || Size(off)+sz > p.size.DataSize {
		return 0, false
	}
	return p.off.addOffset(off), true
}

// Uint8 returns an 8-bit integer from the struct's data section.
func (p Struct) Uint8(off DataOffset) uint8 {
	addr, ok := p.dataAddress(off, 1)
	if !ok {
		return 0
	}
	return p.seg.readUint8(addr)
}

// Uint16 returns a 16-bit integer from the struct's data section.
func (p Struct) Uint16(off DataOffset) uint16 {
	addr, ok := p.dataAddress(off, 2)
	if !ok {
		return 0
	}
	return p.seg.readUint16(addr)
}

// Uint32 returns a 32-bit integer from the struct's data section.
func (p Struct) Uint32(off DataOffset) uint32 {
	addr, ok := p.dataAddress(off, 4)
	if !ok {
		return 0
	}
	return p.seg.readUint32(addr)
}

// Uint64 returns a 64-bit integer from the struct's data section.
func (p Struct) Uint64(off DataOffset) uint64 {
	addr, ok := p.dataAddress(off, 8)
	if !ok {
		return 0
	}
	return p.seg.readUint64(addr)
}

// SetUint8 sets the 8-bit integer that is off bytes from the start of the struct to v.
func (p Struct) SetUint8(off DataOffset, v uint8) {
	addr, ok := p.dataAddress(off, 1)
	if !ok {
		panic("capnp: set field outside struct boundaries")
	}
	p.seg.writeUint8(addr, v)
}

// SetUint16 sets the 16-bit integer that is off bytes from the start of the struct to v.
func (p Struct) SetUint16(off DataOffset, v uint16) {
	addr, ok := p.dataAddress(off, 2)
	if !ok {
		panic("capnp: set field outside struct boundaries")
	}
	p.seg.writeUint16(addr, v)
}

// SetUint32 sets the 32-bit integer that is off bytes from the start of the struct to v.
func (p Struct) SetUint32(off DataOffset, v uint32) {
	addr, ok := p.dataAddress(off, 4)
	if !ok {
		panic("capnp: set field outside struct boundaries")
	}
	p.seg.writeUint32(addr, v)
}

// SetUint64 sets the 64-bit integer that is off bytes from the start of the struct to v.
func (p Struct) SetUint64(off DataOffset, v uint64) {
	addr, ok := p.dataAddress(off, 8)
	if !ok {
		panic("capnp: set field outside struct boundaries")
	}
	p.seg.writeUint64(addr, v)
}

// structFlags is a bitmask of flags for a pointer.
type structFlags uint8

// Pointer flags.
const (
	isListMember structFlags = 1 << iota
)

// copyStruct makes a deep copy of src into dst.
func copyStruct(dst, src Struct) error {
	if dst.seg == nil {
		panic("copy struct into invalid pointer")
	}
	if src.seg == nil {
		return nil
	}

	// Q: how does version handling happen here, when the
	//    destination toData[] slice can be bigger or smaller
	//    than the source data slice, which is in
	//    src.seg.Data[src.off:src.off+src.size.DataSize] ?
	//
	// A: Newer fields only come *after* old fields. Note that
	//    copy only copies min(len(src), len(dst)) size,
	//    and then we manually zero the rest in the for loop
	//    that writes toData[j] = 0.
	//

	// data section:
	srcData := src.seg.slice(src.off, src.size.DataSize)
	dstData := dst.seg.slice(dst.off, dst.size.DataSize)
	copyCount := copy(dstData, srcData)
	dstData = dstData[copyCount:]
	for j := range dstData {
		dstData[j] = 0
	}

	// ptrs section:

	// version handling: we ignore any extra-newer-pointers in src,
	// i.e. the case when srcPtrSize > dstPtrSize, by only
	// running j over the size of dstPtrSize, the destination size.
	srcPtrSect, _ := src.off.addSize(src.size.DataSize)
	dstPtrSect, _ := dst.off.addSize(dst.size.DataSize)
	numSrcPtrs := src.size.PointerCount
	numDstPtrs := dst.size.PointerCount
	for j := uint16(0); j < numSrcPtrs && j < numDstPtrs; j++ {
		srcAddr, _ := srcPtrSect.element(int32(j), wordSize)
		dstAddr, _ := dstPtrSect.element(int32(j), wordSize)
		m, err := src.seg.readPtr(srcAddr, src.depthLimit)
		if err != nil {
			return annotatef(err, "copy struct pointer %d", j)
		}
		err = dst.seg.writePtr(dstAddr, m, true)
		if err != nil {
			return annotatef(err, "copy struct pointer %d", j)
		}
	}
	for j := numSrcPtrs; j < numDstPtrs; j++ {
		// destination p is a newer version than source so these extra new pointer fields in p must be zeroed.
		addr, _ := dstPtrSect.element(int32(j), wordSize)
		dst.seg.writeRawPointer(addr, 0)
	}
	// Nothing more here: so any other pointers in srcPtrSize beyond
	// those in dstPtrSize are ignored and discarded.

	return nil
}

// s.EncodeAsPtr is equivalent to s.ToPtr(); for implementing TypeParam.
// The segment argument is ignored.
func (s Struct) EncodeAsPtr(*Segment) Ptr { return s.ToPtr() }

// DecodeFromPtr(p) is equivalent to p.Struct() (the receiver is ignored).
// for implementing TypeParam.
func (Struct) DecodeFromPtr(p Ptr) Struct { return p.Struct() }

var _ TypeParam[Struct] = Struct{}
