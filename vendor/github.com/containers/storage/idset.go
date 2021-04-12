package storage

import (
	"github.com/containers/storage/pkg/idtools"
	"github.com/google/go-intervals/intervalset"
	"github.com/pkg/errors"
)

// idSet represents a set of integer IDs. It is stored as an ordered set of intervals.
type idSet struct {
	set *intervalset.ImmutableSet
}

func newIDSet(intervals []interval) *idSet {
	s := intervalset.Empty()
	for _, i := range intervals {
		s.Add(intervalset.NewSet([]intervalset.Interval{i}))
	}
	return &idSet{set: s.ImmutableSet()}
}

// getHostIDs returns all the host ids in the id map.
func getHostIDs(idMaps []idtools.IDMap) *idSet {
	var intervals []interval
	for _, m := range idMaps {
		intervals = append(intervals, interval{start: m.HostID, end: m.HostID + m.Size})
	}
	return newIDSet(intervals)
}

// getContainerIDs returns all the container ids in the id map.
func getContainerIDs(idMaps []idtools.IDMap) *idSet {
	var intervals []interval
	for _, m := range idMaps {
		intervals = append(intervals, interval{start: m.ContainerID, end: m.ContainerID + m.Size})
	}
	return newIDSet(intervals)
}

// subtract returns the subtraction of `s` and `t`. `s` and `t` are unchanged.
func (s *idSet) subtract(t *idSet) *idSet {
	if s == nil || t == nil {
		return s
	}
	return &idSet{set: s.set.Sub(t.set)}
}

// union returns the union of `s` and `t`. `s` and `t` are unchanged.
func (s *idSet) union(t *idSet) *idSet {
	if s == nil {
		return t
	} else if t == nil {
		return s
	}
	return &idSet{set: s.set.Union(t.set)}
}

// Methods to iterate over the intervals of the idSet. intervalset doesn't provide one :-(

// iterator to idSet. Returns nil if iteration finishes.
type iteratorFn func() *interval

// cancelFn must be called exactly once unless iteratorFn returns nil, otherwise go routine might
// leak.
type cancelFn func()

func (s *idSet) iterator() (iteratorFn, cancelFn) {
	if s == nil {
		return func() *interval { return nil }, func() {}
	}
	cancelCh := make(chan byte)
	dataCh := make(chan interval)
	go func() {
		s.set.Intervals(func(ii intervalset.Interval) bool {
			select {
			case <-cancelCh:
				return false
			case dataCh <- ii.(interval):
				return true
			}
		})
		close(dataCh)
	}()
	iterator := func() *interval {
		i, ok := <-dataCh
		if !ok {
			return nil
		}
		return &i
	}
	return iterator, func() { close(cancelCh) }
}

// size returns the total number of ids in the ID set.
func (s *idSet) size() int {
	var size int
	iterator, cancel := s.iterator()
	defer cancel()
	for i := iterator(); i != nil; i = iterator() {
		size += i.length()
	}
	return size
}

// findAvailable finds the `n` ids from `s`.
func (s *idSet) findAvailable(n int) (*idSet, error) {
	var intervals []intervalset.Interval
	iterator, cancel := s.iterator()
	defer cancel()
	for i := iterator(); n > 0 && i != nil; i = iterator() {
		i.end = minInt(i.end, i.start+n)
		intervals = append(intervals, *i)
		n -= i.length()
	}
	if n > 0 {
		return nil, errors.New("could not find enough available IDs")
	}
	return &idSet{set: intervalset.NewImmutableSet(intervals)}, nil
}

// zip creates an id map from `s` (host ids) and container ids.
func (s *idSet) zip(container *idSet) []idtools.IDMap {
	hostIterator, hostCancel := s.iterator()
	defer hostCancel()
	containerIterator, containerCancel := container.iterator()
	defer containerCancel()
	var out []idtools.IDMap
	for h, c := hostIterator(), containerIterator(); h != nil && c != nil; {
		if n := minInt(h.length(), c.length()); n > 0 {
			out = append(out, idtools.IDMap{
				ContainerID: c.start,
				HostID:      h.start,
				Size:        n,
			})
			h.start += n
			c.start += n
		}
		if h.IsZero() {
			h = hostIterator()
		}
		if c.IsZero() {
			c = containerIterator()
		}
	}
	return out
}

// interval represents an interval of integers [start, end). Note it is allowed to have
// start >= end, in which case it is treated as an empty interval. It implements interface
// intervalset.Interval.
type interval struct {
	// Start of the interval (inclusive).
	start int
	// End of the interval (exclusive).
	end int
}

func (i interval) length() int {
	return maxInt(0, i.end-i.start)
}

func (i interval) Intersect(other intervalset.Interval) intervalset.Interval {
	j := other.(interval)
	return interval{start: maxInt(i.start, j.start), end: minInt(i.end, j.end)}
}

func (i interval) Before(other intervalset.Interval) bool {
	j := other.(interval)
	return !i.IsZero() && !j.IsZero() && i.end < j.start
}

func (i interval) IsZero() bool {
	return i.length() <= 0
}

func (i interval) Bisect(other intervalset.Interval) (intervalset.Interval, intervalset.Interval) {
	j := other.(interval)
	if j.IsZero() {
		return i, interval{}
	}
	// Subtracting [j.start, j.end) is equivalent to the union of intersecting (-inf, j.start) and
	// [j.end, +inf).
	left := interval{start: i.start, end: minInt(i.end, j.start)}
	right := interval{start: maxInt(i.start, j.end), end: i.end}
	return left, right
}

func (i interval) Adjoin(other intervalset.Interval) intervalset.Interval {
	j := other.(interval)
	if !i.IsZero() && !j.IsZero() && (i.end == j.start || j.end == i.start) {
		return interval{start: minInt(i.start, j.start), end: maxInt(i.end, j.end)}
	}
	return interval{}
}

func (i interval) Encompass(other intervalset.Interval) intervalset.Interval {
	j := other.(interval)
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	default:
		return interval{start: minInt(i.start, j.start), end: maxInt(i.end, j.end)}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}
