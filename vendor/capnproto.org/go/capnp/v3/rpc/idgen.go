package rpc

// idgen returns a sequence of monotonically increasing IDs with
// support for replacement.  The zero value is a generator that
// starts at zero.
type idgen struct {
	i    uint32
	free uintSet
}

func (gen *idgen) next() uint32 {
	if first, ok := gen.free.min(); ok {
		gen.free.remove(first)
		return uint32(first)
	}
	i := gen.i
	if i == ^uint32(0) {
		// TODO(soon): make this abort the connection.
		// All ID generation should be under application control, not
		// remote, but remote could hypothetically retain 1<<32 exports
		// to overflow this.
		panic("overflow ID")
	}
	gen.i++
	return i
}

func (gen *idgen) remove(i uint32) {
	gen.free.add(uint(i))
}

// A uintSet is a set of unsigned integers represented by a bit set.
// This data type assumes that the integers are packed closely to zero.
// The zero value is the empty set.
type uintSet []uint64

func (s uintSet) has(i uint) bool {
	j := i / 64
	mask := uint64(1) << (i % 64)
	return j < uint(len(s)) && s[j]&mask != 0
}

func (s *uintSet) add(i uint) {
	j := i / 64
	mask := uint64(1) << (i % 64)
	if j >= uint(len(*s)) {
		s2 := make(uintSet, j+1)
		copy(s2, *s)
		*s = s2
	}
	(*s)[j] |= mask
}

func (s uintSet) remove(i uint) {
	j := i / 64
	mask := uint64(1) << (i % 64)
	if j < uint(len(s)) {
		s[j] &^= mask
	}
}

func (s uintSet) min() (_ uint, ok bool) {
	for i, x := range s {
		if x == 0 {
			continue
		}
		for j := uint(0); j < 64; j++ {
			if x&(1<<j) != 0 {
				return uint(i)*64 + j, true
			}
		}
	}
	return 0, false
}
