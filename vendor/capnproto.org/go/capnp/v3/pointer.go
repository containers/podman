package capnp

import (
	"bytes"
)

// A Ptr is a reference to a Cap'n Proto struct, list, or interface.
// The zero value is a null pointer.
type Ptr struct {
	seg        *Segment
	off        address
	lenOrCap   uint32
	size       ObjectSize
	depthLimit uint
	flags      ptrFlags
}

// Struct converts p to a Struct. If p does not hold a Struct pointer,
// the zero value is returned.
func (p Ptr) Struct() Struct {
	if p.flags.ptrType() != structPtrType {
		return Struct{}
	}
	return Struct{
		seg:        p.seg,
		off:        p.off,
		size:       p.size,
		flags:      p.flags.structFlags(),
		depthLimit: p.depthLimit,
	}
}

// StructDefault attempts to convert p into a struct, reading the
// default value from def if p is not a struct.
func (p Ptr) StructDefault(def []byte) (Struct, error) {
	s := p.Struct()
	if s.seg == nil {
		if def == nil {
			return Struct{}, nil
		}
		defp, err := unmarshalDefault(def)
		if err != nil {
			return Struct{}, err
		}
		return defp.Struct(), nil
	}
	return s, nil
}

// List converts p to a List. If p does not hold a List pointer,
// the zero value is returned.
func (p Ptr) List() List {
	if p.flags.ptrType() != listPtrType {
		return List{}
	}
	return List{
		seg:        p.seg,
		off:        p.off,
		length:     int32(p.lenOrCap),
		size:       p.size,
		flags:      p.flags.listFlags(),
		depthLimit: p.depthLimit,
	}
}

// ListDefault attempts to convert p into a list, reading the default
// value from def if p is not a list.
func (p Ptr) ListDefault(def []byte) (List, error) {
	l := p.List()
	if l.seg == nil {
		if def == nil {
			return List{}, nil
		}
		defp, err := unmarshalDefault(def)
		if err != nil {
			return List{}, err
		}
		return defp.List(), nil
	}
	return l, nil
}

// Interface converts p to an Interface. If p does not hold a List
// pointer, the zero value is returned.
func (p Ptr) Interface() Interface {
	if p.flags.ptrType() != interfacePtrType {
		return Interface{}
	}
	return Interface{
		seg: p.seg,
		cap: CapabilityID(p.lenOrCap),
	}
}

// Text attempts to convert p into Text, returning an empty string if
// p is not a valid 1-byte list pointer.
func (p Ptr) Text() string {
	b, ok := p.text()
	if !ok {
		return ""
	}
	return string(b)
}

// TextDefault attempts to convert p into Text, returning def if p is
// not a valid 1-byte list pointer.
func (p Ptr) TextDefault(def string) string {
	b, ok := p.text()
	if !ok {
		return def
	}
	return string(b)
}

// TextBytes attempts to convert p into Text, returning nil if p is not
// a valid 1-byte list pointer.  It returns a slice directly into the
// segment.
func (p Ptr) TextBytes() []byte {
	b, ok := p.text()
	if !ok {
		return nil
	}
	return b
}

// TextBytesDefault attempts to convert p into Text, returning def if p
// is not a valid 1-byte list pointer.  It returns a slice directly into
// the segment.
func (p Ptr) TextBytesDefault(def string) []byte {
	b, ok := p.text()
	if !ok {
		return []byte(def)
	}
	return b
}

func (p Ptr) text() (b []byte, ok bool) {
	if !isOneByteList(p) {
		return nil, false
	}
	l := p.List()
	b = l.seg.slice(l.off, Size(l.length))
	if len(b) == 0 || b[len(b)-1] != 0 {
		// Text must be null-terminated.
		return nil, false
	}
	return b[: len(b)-1 : len(b)], true
}

// Data attempts to convert p into Data, returning nil if p is not a
// valid 1-byte list pointer.
func (p Ptr) Data() []byte {
	return p.DataDefault(nil)
}

// DataDefault attempts to convert p into Data, returning def if p is
// not a valid 1-byte list pointer.
func (p Ptr) DataDefault(def []byte) []byte {
	if !isOneByteList(p) {
		return def
	}
	l := p.List()
	b := l.seg.slice(l.off, Size(l.length))
	if b == nil {
		return def
	}
	return b
}

// IsValid reports whether p is valid.
func (p Ptr) IsValid() bool {
	return p.seg != nil
}

// Segment returns the segment that the referenced data is stored in
// or nil if the pointer is invalid.
func (p Ptr) Segment() *Segment {
	return p.seg
}

// Message returns the message the referenced data is stored in or nil
// if the pointer is invalid.
func (p Ptr) Message() *Message {
	if p.seg == nil {
		return nil
	}
	return p.seg.msg
}

// Default returns p if it is valid, otherwise it unmarshals def.
func (p Ptr) Default(def []byte) (Ptr, error) {
	if !p.IsValid() {
		return unmarshalDefault(def)
	}
	return p, nil
}

// SamePtr reports whether p and q refer to the same object.
func SamePtr(p, q Ptr) bool {
	return p.seg == q.seg && p.off == q.off
}

// EncodeAsPtr returns the receiver; for implementing TypeParam.
// The segment argument is ignored.
func (p Ptr) EncodeAsPtr(*Segment) Ptr { return p }

// DecodeFromPtr returns its argument; for implementing TypeParam.
func (Ptr) DecodeFromPtr(p Ptr) Ptr { return p }

var _ TypeParam[Ptr] = Ptr{}

func unmarshalDefault(def []byte) (Ptr, error) {
	msg, err := Unmarshal(def)
	if err != nil {
		return Ptr{}, annotatef(err, "read default")
	}
	p, err := msg.Root()
	if err != nil {
		return Ptr{}, annotatef(err, "read default")
	}
	return p, nil
}

type ptrFlags uint8

const interfacePtrFlag ptrFlags = interfacePtrType << 6

func structPtrFlag(f structFlags) ptrFlags {
	return structPtrType<<6 | ptrFlags(f)&ptrLowerMask
}

func listPtrFlag(f listFlags) ptrFlags {
	return listPtrType<<6 | ptrFlags(f)&ptrLowerMask
}

const (
	structPtrType = iota
	listPtrType
	interfacePtrType
)

func (f ptrFlags) ptrType() int {
	return int(f >> 6)
}

const ptrLowerMask ptrFlags = 0x3f

func (f ptrFlags) listFlags() listFlags {
	return listFlags(f & ptrLowerMask)
}

func (f ptrFlags) structFlags() structFlags {
	return structFlags(f & ptrLowerMask)
}

func isZeroFilled(b []byte) bool {
	for _, bb := range b {
		if bb != 0 {
			return false
		}
	}
	return true
}

// Equal returns true iff p1 and p2 are equal.
//
// Equality is defined to be:
//
//	- Two structs are equal iff all of their fields are equal.  If one
//	  struct has more fields than the other, the extra fields must all be
//		zero.
//	- Two lists are equal iff they have the same length and their
//	  corresponding elements are equal.  If one list is a list of
//	  primitives and the other is a list of structs, then the list of
//	  primitives is treated as if it was a list of structs with the
//	  element value as the sole field.
//	- Two interfaces are equal iff they point to a capability created by
//	  the same call to NewClient or they are referring to the same
//	  capability table index in the same message.  The latter is
//	  significant when the message's capability table has not been
//	  populated.
//	- Two null pointers are equal.
//	- All other combinations of things are not equal.
func Equal(p1, p2 Ptr) (bool, error) {
	if !p1.IsValid() && !p2.IsValid() {
		return true, nil
	}
	if !p1.IsValid() || !p2.IsValid() {
		return false, nil
	}
	pt := p1.flags.ptrType()
	if pt != p2.flags.ptrType() {
		return false, nil
	}
	switch pt {
	case structPtrType:
		s1, s2 := p1.Struct(), p2.Struct()
		data1 := s1.seg.slice(s1.off, s1.size.DataSize)
		data2 := s2.seg.slice(s2.off, s2.size.DataSize)
		switch {
		case len(data1) < len(data2):
			if !bytes.Equal(data1, data2[:len(data1)]) {
				return false, nil
			}
			if !isZeroFilled(data2[len(data1):]) {
				return false, nil
			}
		case len(data1) > len(data2):
			if !bytes.Equal(data1[:len(data2)], data2) {
				return false, nil
			}
			if !isZeroFilled(data1[len(data2):]) {
				return false, nil
			}
		default:
			if !bytes.Equal(data1, data2) {
				return false, nil
			}
		}
		n := int(s1.size.PointerCount)
		if n2 := int(s2.size.PointerCount); n2 < n {
			n = n2
		}
		for i := 0; i < n; i++ {
			sp1, err := s1.Ptr(uint16(i))
			if err != nil {
				return false, annotatef(err, "equal")
			}
			sp2, err := s2.Ptr(uint16(i))
			if err != nil {
				return false, annotatef(err, "equal")
			}
			if ok, err := Equal(sp1, sp2); !ok || err != nil {
				return false, err
			}
		}
		for i := n; i < int(s1.size.PointerCount); i++ {
			if s1.HasPtr(uint16(i)) {
				return false, nil
			}
		}
		for i := n; i < int(s2.size.PointerCount); i++ {
			if s2.HasPtr(uint16(i)) {
				return false, nil
			}
		}
		return true, nil
	case listPtrType:
		l1, l2 := p1.List(), p2.List()
		if l1.Len() != l2.Len() {
			return false, nil
		}
		if l1.flags&isCompositeList == 0 && l2.flags&isCompositeList == 0 && l1.size != l2.size {
			return false, nil
		}
		if l1.size.PointerCount == 0 && l2.size.PointerCount == 0 && l1.size.DataSize == l2.size.DataSize {
			// Optimization: pure data lists can be compared bytewise.
			sz, _ := l1.size.totalSize().times(l1.length) // both list bounds have been validated
			return bytes.Equal(l1.seg.slice(l1.off, sz), l2.seg.slice(l2.off, sz)), nil
		}
		for i := 0; i < l1.Len(); i++ {
			e1, e2 := l1.Struct(i), l2.Struct(i)
			if ok, err := Equal(e1.ToPtr(), e2.ToPtr()); err != nil {
				return false, annotatef(err, "equal: list element %d", i)
			} else if !ok {
				return false, nil
			}
		}
		return true, nil
	case interfacePtrType:
		i1, i2 := p1.Interface(), p2.Interface()
		if i1.Message() == i2.Message() {
			if i1.Capability() == i2.Capability() {
				return true, nil
			}
			ntab := len(i1.Message().CapTable)
			if int64(i1.Capability()) >= int64(ntab) || int64(i2.Capability()) >= int64(ntab) {
				return false, nil
			}
		}
		return i1.Client().IsSame(i2.Client()), nil
	default:
		panic("unreachable")
	}
}
