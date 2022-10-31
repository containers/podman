package capnp

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"capnproto.org/go/capnp/v3/packed"
)

// Security limits. Matches C++ implementation.
const (
	defaultTraverseLimit = 64 << 20 // 64 MiB
	defaultDepthLimit    = 64

	maxStreamSegments = 512

	defaultDecodeLimit = 64 << 20 // 64 MiB
)

const maxDepth = ^uint(0)

// A Message is a tree of Cap'n Proto objects, split into one or more
// segments of contiguous memory.  The only required field is Arena.
// A Message is safe to read from multiple goroutines.
type Message struct {
	// rlimit must be first so that it is 64-bit aligned.
	// See sync/atomic docs.
	rlimit     uint64
	rlimitInit sync.Once

	Arena Arena

	// CapTable is the indexed list of the clients referenced in the
	// message.  Capability pointers inside the message will use this table
	// to map pointers to Clients.  The table is usually populated by the
	// RPC system.
	//
	// See https://capnproto.org/encoding.html#capabilities-interfaces for
	// more details on the capability table.
	CapTable []Client

	// TraverseLimit limits how many total bytes of data are allowed to be
	// traversed while reading.  Traversal is counted when a Struct or
	// List is obtained.  This means that calling a getter for the same
	// sub-struct multiple times will cause it to be double-counted.  Once
	// the traversal limit is reached, pointer accessors will report
	// errors. See https://capnproto.org/encoding.html#amplification-attack
	// for more details on this security measure.
	//
	// If not set, this defaults to 64 MiB.
	TraverseLimit uint64

	// DepthLimit limits how deeply-nested a message structure can be.
	// If not set, this defaults to 64.
	DepthLimit uint

	// mu protects the following fields:
	mu       sync.Mutex
	segs     map[SegmentID]*Segment
	firstSeg Segment // Preallocated first segment. msg is non-nil once initialized.
}

// NewMessage creates a message with a new root and returns the first
// segment.  It is an error to call NewMessage on an arena with data in it.
func NewMessage(arena Arena) (msg *Message, first *Segment, err error) {
	msg = &Message{Arena: arena}
	switch arena.NumSegments() {
	case 0:
		first, err = msg.allocSegment(wordSize)
		if err != nil {
			return nil, nil, annotatef(err, "new message")
		}
	case 1:
		first, err = msg.Segment(0)
		if err != nil {
			return nil, nil, annotatef(err, "new message")
		}
		if len(first.data) > 0 {
			return nil, nil, errorf("new message: arena not empty")
		}
	default:
		return nil, nil, errorf("new message: arena not empty")
	}
	if first.ID() != 0 {
		return nil, nil, errorf("new message: arena allocated first segment with non-zero ID")
	}
	seg, _, err := alloc(first, wordSize) // allocate root
	if err != nil {
		return nil, nil, annotatef(err, "new message")
	}
	if seg != first {
		return nil, nil, errorf("new message: arena allocated first word outside first segment")
	}
	return msg, first, nil
}

// NewSingleSegmentMessage(b) is equivalent to NewMessage(SingleSegment(b)), except
// that it panics instead of returning an error. This can only happen if the passed
// slice contains data, so the caller is responsible for ensuring that it has a length
// of zero.
func NewSingleSegmentMessage(b []byte) (msg *Message, first *Segment) {
	msg, first, err := NewMessage(SingleSegment(b))
	if err != nil {
		panic(err)
	}
	return msg, first
}

// Analogous to NewSingleSegmentMessage, but using MutliSegment.
func NewMultiSegmentMessage(b [][]byte) (msg *Message, first *Segment) {
	msg, first, err := NewMessage(MultiSegment(b))
	if err != nil {
		panic(err)
	}
	return msg, first
}

// Reset resets a message to use a different arena, allowing a single
// Message to be reused for reading multiple messages.  This invalidates
// any existing pointers in the Message, so use with caution.  All
// clients in the message's capability table will be released.
func (m *Message) Reset(arena Arena) {
	m.mu.Lock()
	m.segs = nil
	m.firstSeg = Segment{}
	m.mu.Unlock()

	m.Arena = arena
	for _, c := range m.CapTable {
		c.Release()
	}
	m.CapTable = nil
	m.rlimitInit.Do(func() {})
	m.initReadLimit()
}

func (m *Message) initReadLimit() {
	if m.TraverseLimit == 0 {
		atomic.StoreUint64(&m.rlimit, defaultTraverseLimit)
		return
	}
	atomic.StoreUint64(&m.rlimit, m.TraverseLimit)
}

// canRead reports whether the amount of bytes can be stored safely.
func (m *Message) canRead(sz Size) bool {
	m.rlimitInit.Do(m.initReadLimit)
	for {
		curr := atomic.LoadUint64(&m.rlimit)
		ok := curr >= uint64(sz)
		var new uint64
		if ok {
			new = curr - uint64(sz)
		} else {
			new = 0
		}
		if atomic.CompareAndSwapUint64(&m.rlimit, curr, new) {
			return ok
		}
	}
}

// ResetReadLimit sets the number of bytes allowed to be read from this message.
func (m *Message) ResetReadLimit(limit uint64) {
	m.rlimitInit.Do(func() {})
	atomic.StoreUint64(&m.rlimit, limit)
}

// Unread increases the read limit by sz.
func (m *Message) Unread(sz Size) {
	m.rlimitInit.Do(m.initReadLimit)
	atomic.AddUint64(&m.rlimit, uint64(sz))
}

// Root returns the pointer to the message's root object.
func (m *Message) Root() (Ptr, error) {
	s, err := m.Segment(0)
	if err != nil {
		return Ptr{}, annotatef(err, "read root")
	}
	p, err := s.root().At(0)
	if err != nil {
		return Ptr{}, annotatef(err, "read root")
	}
	return p, nil
}

// SetRoot sets the message's root object to p.
func (m *Message) SetRoot(p Ptr) error {
	s, err := m.Segment(0)
	if err != nil {
		return annotatef(err, "set root")
	}
	if err := s.root().Set(0, p); err != nil {
		return annotatef(err, "set root")
	}
	return nil
}

// AddCap appends a capability to the message's capability table and
// returns its ID.  It "steals" c's reference: the Message will release
// the client when calling Reset.
func (m *Message) AddCap(c Client) CapabilityID {
	n := CapabilityID(len(m.CapTable))
	m.CapTable = append(m.CapTable, c)
	return n
}

// Compute the total size of the message in bytes, when serialized as
// a stream. This is the same as the length of the slice returned by
// m.Marshal()
func (m *Message) TotalSize() (uint64, error) {
	nsegs := uint64(m.NumSegments())
	totalSize := (nsegs/2 + 1) * 8
	for i := uint64(0); i < nsegs; i++ {
		seg, err := m.Segment(SegmentID(i))
		if err != nil {
			return 0, err
		}
		totalSize += uint64(len(seg.Data()))
	}
	return totalSize, nil
}

func (m *Message) depthLimit() uint {
	if m.DepthLimit != 0 {
		return m.DepthLimit
	}
	return defaultDepthLimit
}

// NumSegments returns the number of segments in the message.
func (m *Message) NumSegments() int64 {
	return int64(m.Arena.NumSegments())
}

// Segment returns the segment with the given ID.
func (m *Message) Segment(id SegmentID) (*Segment, error) {
	if int64(id) >= m.Arena.NumSegments() {
		return nil, errorf("segment %d: out of bounds", id)
	}
	m.mu.Lock()
	seg, err := m.segment(id)
	m.mu.Unlock()
	return seg, err
}

// segment returns the segment with the given ID, with no bounds
// checking.  The caller must be holding m.mu.
func (m *Message) segment(id SegmentID) (*Segment, error) {
	if m.segs == nil && id == 0 && m.firstSeg.msg != nil {
		return &m.firstSeg, nil
	}
	if s := m.segs[id]; s != nil {
		return s, nil
	}
	if len(m.segs) == maxInt {
		return nil, errorf("segment %d: number of loaded segments exceeds int", id)
	}
	data, err := m.Arena.Data(id)
	if err != nil {
		return nil, errorf("load segment %d: %v", id, err)
	}
	s := m.setSegment(id, data)
	return s, nil
}

// setSegment creates or updates the Segment with the given ID.
// The caller must be holding m.mu.
func (m *Message) setSegment(id SegmentID, data []byte) *Segment {
	if m.segs == nil {
		if id == 0 {
			m.firstSeg = Segment{
				id:   id,
				msg:  m,
				data: data,
			}
			return &m.firstSeg
		}
		m.segs = make(map[SegmentID]*Segment)
		if m.firstSeg.msg != nil {
			m.segs[0] = &m.firstSeg
		}
	} else if seg := m.segs[id]; seg != nil {
		seg.data = data
		return seg
	}
	seg := &Segment{
		id:   id,
		msg:  m,
		data: data,
	}
	m.segs[id] = seg
	return seg
}

// allocSegment creates or resizes an existing segment such that
// cap(seg.Data) - len(seg.Data) >= sz.  The caller must not be holding
// onto m.mu.
func (m *Message) allocSegment(sz Size) (*Segment, error) {
	if sz > maxAllocSize() {
		return nil, errorf("allocation: too large")
	}
	m.mu.Lock()
	if len(m.segs) == maxInt {
		m.mu.Unlock()
		return nil, errorf("allocation: number of loaded segments exceeds int")
	}
	if m.segs == nil && m.firstSeg.msg != nil {
		// Transition from sole segment to segment map.
		m.segs = make(map[SegmentID]*Segment)
		m.segs[0] = &m.firstSeg
	}
	id, data, err := m.Arena.Allocate(sz, m.segs)
	if err != nil {
		m.mu.Unlock()
		return nil, errorf("allocation: %v", err)
	}
	seg := m.setSegment(id, data)
	m.mu.Unlock()
	return seg, nil
}

// alloc allocates sz zero-filled bytes.  It prefers using s, but may
// use a different segment in the same message if there's not sufficient
// capacity.
func alloc(s *Segment, sz Size) (*Segment, address, error) {
	if sz > maxAllocSize() {
		return nil, 0, errorf("allocation: too large")
	}
	sz = sz.padToWord()

	if !hasCapacity(s.data, sz) {
		var err error
		s, err = s.msg.allocSegment(sz)
		if err != nil {
			return nil, 0, err
		}
	}

	addr := address(len(s.data))
	end, ok := addr.addSize(sz)
	if !ok {
		return nil, 0, errorf("allocation: address overflow")
	}
	space := s.data[len(s.data):end]
	s.data = s.data[:end]
	for i := range space {
		space[i] = 0
	}
	return s, addr, nil
}

func (m *Message) WriteTo(w io.Writer) (int64, error) {
	wc := &writeCounter{Writer: w}
	err := NewEncoder(wc).Encode(m)
	return wc.N, err
}

type writeCounter struct {
	N int64
	io.Writer
}

func (wc *writeCounter) Write(b []byte) (n int, err error) {
	n, err = wc.Writer.Write(b)
	wc.N += int64(n)
	return
}

// An Arena loads and allocates segments for a Message.
type Arena interface {
	// NumSegments returns the number of segments in the arena.
	// This must not be larger than 1<<32.
	NumSegments() int64

	// Data loads the data for the segment with the given ID.  IDs are in
	// the range [0, NumSegments()).
	// must be tightly packed in the range [0, NumSegments()).
	Data(id SegmentID) ([]byte, error)

	// Allocate selects a segment to place a new object in, creating a
	// segment or growing the capacity of a previously loaded segment if
	// necessary.  If Allocate does not return an error, then the
	// difference of the capacity and the length of the returned slice
	// must be at least minsz.  segs is a map of segments keyed by ID
	// using arrays returned by the Data method (although the length of
	// these slices may have changed by previous allocations).  Allocate
	// must not modify segs.
	//
	// If Allocate creates a new segment, the ID must be one larger than
	// the last segment's ID or zero if it is the first segment.
	//
	// If Allocate returns an previously loaded segment's ID, then the
	// arena is responsible for preserving the existing data in the
	// returned byte slice.
	Allocate(minsz Size, segs map[SegmentID]*Segment) (SegmentID, []byte, error)
}

// SingleSegmentArena is an Arena implementation that stores message data
// in a continguous slice.  Allocation is performed by first allocating a
// new slice and copying existing data. SingleSegment arena does not fail
// unless the caller attempts to access another segment.
type SingleSegmentArena []byte

// SingleSegment constructs a SingleSegmentArena from b.  b MAY be nil.
// Callers MAY use b to populate the segment for reading, or to reserve
// memory of a specific size.
func SingleSegment(b []byte) *SingleSegmentArena {
	return (*SingleSegmentArena)(&b)
}

func (ssa SingleSegmentArena) NumSegments() int64 {
	return 1
}

func (ssa SingleSegmentArena) Data(id SegmentID) ([]byte, error) {
	if id != 0 {
		return nil, errorf("segment %d requested in single segment arena", id)
	}
	return ssa, nil
}

func (ssa *SingleSegmentArena) Allocate(sz Size, segs map[SegmentID]*Segment) (SegmentID, []byte, error) {
	data := []byte(*ssa)
	if segs[0] != nil {
		data = segs[0].data
	}
	if len(data)%int(wordSize) != 0 {
		return 0, nil, errorf("segment size is not a multiple of word size")
	}
	if hasCapacity(data, sz) {
		return 0, data, nil
	}
	inc, err := nextAlloc(int64(len(data)), int64(maxAllocSize()), sz)
	if err != nil {
		return 0, nil, err
	}
	buf := make([]byte, len(data), cap(data)+inc)
	copy(buf, data)
	*ssa = buf
	return 0, *ssa, nil
}

func (ssa SingleSegmentArena) String() string {
	return fmt.Sprintf("single-segment arena [len=%d cap=%d]", len(ssa), cap(ssa))
}

type roSingleSegment []byte

func (ss roSingleSegment) NumSegments() int64 {
	return 1
}

func (ss roSingleSegment) Data(id SegmentID) ([]byte, error) {
	if id != 0 {
		return nil, errorf("segment %d requested in single segment arena", id)
	}
	return ss, nil
}

func (ss roSingleSegment) Allocate(sz Size, segs map[SegmentID]*Segment) (SegmentID, []byte, error) {
	return 0, nil, errorf("arena is read-only")
}

func (ss roSingleSegment) String() string {
	return fmt.Sprintf("read-only single-segment arena [len=%d]", len(ss))
}

// MultiSegment is an arena that stores object data across multiple []byte
// buffers, allocating new buffers of exponentially-increasing size when
// full. This avoids the potentially-expensive slice copying of SingleSegment.
type MultiSegmentArena [][]byte

// MultiSegment returns a new arena that allocates new segments when
// they are full.  b MAY be nil.  Callers MAY use b to populate the
// buffer for reading or to reserve memory of a specific size.
func MultiSegment(b [][]byte) *MultiSegmentArena {
	return (*MultiSegmentArena)(&b)
}

// demuxArena slices b into a multi-segment arena.  It assumes that
// len(data) >= hdr.totalSize().
func demuxArena(hdr streamHeader, data []byte) (Arena, error) {
	maxSeg := hdr.maxSegment()
	if int64(maxSeg) > int64(maxInt-1) {
		return nil, errorf("number of segments overflows int")
	}
	segs := make([][]byte, int(maxSeg+1))
	for i := range segs {
		sz, err := hdr.segmentSize(SegmentID(i))
		if err != nil {
			return nil, err
		}
		segs[i], data = data[:sz:sz], data[sz:]
	}
	return MultiSegment(segs), nil
}

func (msa *MultiSegmentArena) NumSegments() int64 {
	return int64(len(*msa))
}

func (msa *MultiSegmentArena) Data(id SegmentID) ([]byte, error) {
	if int64(id) >= int64(len(*msa)) {
		return nil, errorf("segment %d requested (arena only has %d segments)", id, len(*msa))
	}
	return (*msa)[id], nil
}

func (msa *MultiSegmentArena) Allocate(sz Size, segs map[SegmentID]*Segment) (SegmentID, []byte, error) {
	var total int64
	for i, data := range *msa {
		id := SegmentID(i)
		if s := segs[id]; s != nil {
			data = s.data
		}
		if hasCapacity(data, sz) {
			return id, data, nil
		}
		total += int64(cap(data))
		if total < 0 {
			// Overflow.
			return 0, nil, errorf("alloc %d bytes: message too large", sz)
		}
	}
	n, err := nextAlloc(total, 1<<63-1, sz)
	if err != nil {
		return 0, nil, err
	}
	buf := make([]byte, 0, n)
	id := SegmentID(len(*msa))
	*msa = append(*msa, buf)
	return id, buf, nil
}

func (msa *MultiSegmentArena) String() string {
	return fmt.Sprintf("multi-segment arena [%d segments]", len(*msa))
}

// nextAlloc computes how much more space to allocate given the number
// of bytes allocated in the entire message and the requested number of
// bytes.  It will always return a multiple of wordSize.  max must be a
// multiple of wordSize.  The sum of curr and the returned size will
// always be less than max.
func nextAlloc(curr, max int64, req Size) (int, error) {
	if req == 0 {
		return 0, nil
	}
	if req > maxAllocSize() {
		return 0, errorf("alloc %v: too large", req)
	}
	padreq := req.padToWord()
	want := curr + int64(padreq)
	if want <= curr || want > max {
		return 0, errorf("alloc %v: message size overflow", req)
	}
	new := curr
	double := new + new
	switch {
	case want < 1024:
		next := (1024 - curr + 7) &^ 7
		if next < curr {
			return int((curr + 7) &^ 7), nil
		}
		return int(next), nil
	case want > double:
		return int(padreq), nil
	default:
		for 0 < new && new < want {
			new += new / 4
		}
		if new <= 0 {
			return int(padreq), nil
		}
		delta := new - curr
		if delta > int64(maxAllocSize()) {
			return int(maxAllocSize()), nil
		}
		return int((delta + 7) &^ 7), nil
	}
}

// A Decoder represents a framer that deserializes a particular Cap'n
// Proto input stream.
type Decoder struct {
	r io.Reader

	wordbuf [wordSize]byte
	hdrbuf  []byte

	reuse bool
	buf   []byte
	msg   Message
	arena roSingleSegment

	// Maximum number of bytes that can be read per call to Decode.
	// If not set, a reasonable default is used.
	MaxMessageSize uint64
}

// NewDecoder creates a new Cap'n Proto framer that reads from r.
// The returned decoder will only read as much data as necessary to
// decode the message.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// NewPackedDecoder creates a new Cap'n Proto framer that reads from a
// packed stream r.  The returned decoder may read more data than
// necessary from r.
func NewPackedDecoder(r io.Reader) *Decoder {
	return NewDecoder(packed.NewReader(bufio.NewReader(r)))
}

// Decode reads a message from the decoder stream.  The error is io.EOF
// only if no bytes were read.
func (d *Decoder) Decode() (*Message, error) {
	maxSize := d.MaxMessageSize
	if maxSize == 0 {
		maxSize = defaultDecodeLimit
	} else if maxSize < uint64(len(d.wordbuf)) {
		return nil, errorf("decode: max message size is smaller than header size")
	}

	// Read first word (number of segments and first segment size).
	// For single-segment messages, this will be sufficient.
	if _, err := io.ReadFull(d.r, d.wordbuf[:]); err == io.EOF {
		return nil, io.EOF
	} else if err != nil {
		return nil, errorf("decode: read header: %v", err)
	}
	maxSeg := SegmentID(binary.LittleEndian.Uint32(d.wordbuf[:]))
	if maxSeg > maxStreamSegments {
		return nil, errorf("decode: too many segments to decode")
	}

	// Read the rest of the header if more than one segment.
	var hdr streamHeader
	if maxSeg == 0 {
		hdr = streamHeader{d.wordbuf[:]}
	} else {
		hdrSize := streamHeaderSize(maxSeg)
		if hdrSize > maxSize || hdrSize > uint64(maxInt) {
			return nil, errorf("decode: message too large")
		}
		d.hdrbuf = resizeSlice(d.hdrbuf, int(hdrSize))
		copy(d.hdrbuf, d.wordbuf[:])
		if _, err := io.ReadFull(d.r, d.hdrbuf[len(d.wordbuf):]); err != nil {
			return nil, errorf("decode: read header: %v", err)
		}
		hdr = streamHeader{d.hdrbuf}
	}
	total, err := hdr.totalSize()
	if err != nil {
		return nil, annotatef(err, "decode")
	}
	// TODO(someday): if total size is greater than can fit in one buffer,
	// attempt to allocate buffer per segment.
	if total > maxSize-uint64(len(hdr.b)) || total > uint64(maxInt) {
		return nil, errorf("decode: message too large")
	}

	// Read segments.
	if !d.reuse {
		buf := make([]byte, int(total))
		if _, err := io.ReadFull(d.r, buf); err != nil {
			return nil, errorf("decode: read segments: %v", err)
		}
		arena, err := demuxArena(hdr, buf)
		if err != nil {
			return nil, annotatef(err, "decode")
		}
		return &Message{Arena: arena}, nil
	}
	d.buf = resizeSlice(d.buf, int(total))
	if _, err := io.ReadFull(d.r, d.buf); err != nil {
		return nil, errorf("decode: read segments: %v", err)
	}
	var arena Arena
	if maxSeg == 0 {
		d.arena = d.buf[:len(d.buf):len(d.buf)]
		arena = &d.arena
	} else {
		var err error
		arena, err = demuxArena(hdr, d.buf)
		if err != nil {
			return nil, annotatef(err, "decode")
		}
	}
	d.msg.Reset(arena)
	return &d.msg, nil
}

func resizeSlice(b []byte, size int) []byte {
	if cap(b) < size {
		return make([]byte, size)
	}
	return b[:size]
}

// ReuseBuffer causes the decoder to reuse its buffer on subsequent decodes.
// The decoder may return messages that cannot handle allocations.
func (d *Decoder) ReuseBuffer() {
	d.reuse = true
}

// Unmarshal reads an unpacked serialized stream into a message.  No
// copying is performed, so the objects in the returned message read
// directly from data.
func Unmarshal(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, io.EOF
	}
	if len(data) < int(wordSize) {
		return nil, errorf("unmarshal: short header section")
	}
	maxSeg := SegmentID(binary.LittleEndian.Uint32(data))
	hdrSize := streamHeaderSize(maxSeg)
	if uint64(len(data)) < hdrSize {
		return nil, errorf("unmarshal: short header section")
	}
	hdr := streamHeader{data[:hdrSize]}
	data = data[hdrSize:]
	if total, err := hdr.totalSize(); err != nil {
		return nil, annotatef(err, "unmarshal")
	} else if total > uint64(len(data)) {
		return nil, errorf("unmarshal: short data section")
	}
	arena, err := demuxArena(hdr, data)
	if err != nil {
		return nil, annotatef(err, "unmarshal")
	}
	return &Message{Arena: arena}, nil
}

// UnmarshalPacked reads a packed serialized stream into a message.
func UnmarshalPacked(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, io.EOF
	}
	data, err := packed.Unpack(nil, data)
	if err != nil {
		return nil, errorf("unmarshal: %v", err)
	}
	return Unmarshal(data)
}

// MustUnmarshalRoot reads an unpacked serialized stream and returns
// its root pointer.  If there is any error, it panics.
func MustUnmarshalRoot(data []byte) Ptr {
	msg, err := Unmarshal(data)
	if err != nil {
		panic(err)
	}
	p, err := msg.Root()
	if err != nil {
		panic(err)
	}
	return p
}

// An Encoder represents a framer for serializing a particular Cap'n
// Proto stream.
type Encoder struct {
	w      io.Writer
	hdrbuf []byte
	bufs   [][]byte
}

// NewEncoder creates a new Cap'n Proto framer that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// NewPackedEncoder creates a new Cap'n Proto framer that writes to a
// packed stream w.
func NewPackedEncoder(w io.Writer) *Encoder {
	return NewEncoder(&packed.Writer{Writer: w})
}

// Encode writes a message to the encoder stream.
func (e *Encoder) Encode(m *Message) error {
	nsegs := m.NumSegments()
	if nsegs == 0 {
		return errorf("encode: message has no segments")
	}
	e.bufs = append(e.bufs[:0], nil) // first element is placeholder for header
	maxSeg := SegmentID(nsegs - 1)
	hdrSize := streamHeaderSize(maxSeg)
	if hdrSize > uint64(maxInt) {
		return errorf("encode: header size overflows int")
	}
	e.hdrbuf = resizeSlice(e.hdrbuf, int(hdrSize))
	e.hdrbuf = appendUint32(e.hdrbuf[:0], uint32(maxSeg))
	for i := int64(0); i < nsegs; i++ {
		s, err := m.Segment(SegmentID(i))
		if err != nil {
			return annotatef(err, "encode")
		}
		n := len(s.data)
		if int64(n) > int64(maxSegmentSize) {
			return errorf("encode: segment %d too large", i)
		}
		e.hdrbuf = appendUint32(e.hdrbuf, uint32(Size(n)/wordSize))
		e.bufs = append(e.bufs, s.data)
	}
	if len(e.hdrbuf)%int(wordSize) != 0 {
		e.hdrbuf = appendUint32(e.hdrbuf, 0)
	}
	e.bufs[0] = e.hdrbuf

	if err := e.write(e.bufs); err != nil {
		return errorf("encode: %v", err)
	}

	return nil
}

// Marshal concatenates the segments in the message into a single byte
// slice including framing.
func (m *Message) Marshal() ([]byte, error) {
	// Compute buffer size.
	nsegs := m.NumSegments()
	if nsegs == 0 {
		return nil, errorf("marshal: message has no segments")
	}
	hdrSize := streamHeaderSize(SegmentID(nsegs - 1))
	if hdrSize > uint64(maxInt) {
		return nil, errorf("marshal: header size overflows int")
	}
	var dataSize uint64
	m.mu.Lock()
	for i := int64(0); i < nsegs; i++ {
		s, err := m.segment(SegmentID(i))
		if err != nil {
			m.mu.Unlock()
			return nil, annotatef(err, "marshal")
		}
		n := uint64(len(s.data))
		if n%uint64(wordSize) != 0 {
			m.mu.Unlock()
			return nil, errorf("marshal: segment %d not word-aligned", i)
		}
		if n > uint64(maxSegmentSize) {
			m.mu.Unlock()
			return nil, errorf("marshal: segment %d too large", i)
		}
		dataSize += n
		if dataSize > uint64(maxInt) {
			m.mu.Unlock()
			return nil, errorf("marshal: message size overflows int")
		}
	}
	total := hdrSize + dataSize
	if total > uint64(maxInt) {
		m.mu.Unlock()
		return nil, errorf("marshal: message size overflows int")
	}

	// Fill buffer.
	buf := make([]byte, int(hdrSize), int(total))
	binary.LittleEndian.PutUint32(buf, uint32(nsegs-1))
	for i := int64(0); i < nsegs; i++ {
		s, err := m.segment(SegmentID(i))
		if err != nil {
			m.mu.Unlock()
			return nil, annotatef(err, "marshal")
		}
		if len(s.data)%int(wordSize) != 0 {
			m.mu.Unlock()
			return nil, errorf("marshal: segment %d not word-aligned", i)
		}
		binary.LittleEndian.PutUint32(buf[int(i+1)*4:], uint32(len(s.data)/int(wordSize)))
		buf = append(buf, s.data...)
	}
	m.mu.Unlock()
	return buf, nil
}

// MarshalPacked marshals the message in packed form.
func (m *Message) MarshalPacked() ([]byte, error) {
	data, err := m.Marshal()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, len(data))
	buf = packed.Pack(buf, data)
	return buf, nil
}

// streamHeaderSize returns the size of the header, given the lower 32
// bits of the first word of the header (the number of segments minus
// one).
func streamHeaderSize(maxSeg SegmentID) uint64 {
	return ((uint64(maxSeg)+2)*4 + 7) &^ 7
}

// appendUint32 appends a uint32 to a byte slice and returns the
// new slice.
func appendUint32(b []byte, v uint32) []byte {
	b = append(b, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(b[len(b)-4:], v)
	return b
}

// streamHeader holds the framing words at the beginning of a serialized
// Cap'n Proto message.
type streamHeader struct {
	b []byte
}

// maxSegment returns the number of segments in the message minus one.
func (h streamHeader) maxSegment() SegmentID {
	return SegmentID(binary.LittleEndian.Uint32(h.b))
}

// segmentSize returns the size of segment i, returning an error if the
// segment overflows maxSegmentSize.
func (h streamHeader) segmentSize(i SegmentID) (Size, error) {
	s := binary.LittleEndian.Uint32(h.b[4+i*4:])
	sz, ok := wordSize.times(int32(s))
	if !ok {
		return 0, errorf("segment %d: overflow size", i)
	}
	return sz, nil
}

// totalSize returns the sum of all the segment sizes.  The sum will
// be in the range [0, 0xfffffff800000000].
func (h streamHeader) totalSize() (uint64, error) {
	var sum uint64
	for i := uint64(0); i <= uint64(h.maxSegment()); i++ {
		x, err := h.segmentSize(SegmentID(i))
		if err != nil {
			return sum, err
		}
		sum += uint64(x)
	}
	return sum, nil
}

func hasCapacity(b []byte, sz Size) bool {
	return sz <= Size(cap(b)-len(b))
}

const maxInt = int(^uint(0) >> 1)
