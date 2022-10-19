package quadlet

import (
	"math"
	"strconv"
	"strings"
)

// The Ranges abstraction efficiently keeps track of a list of non-intersecting
// ranges of uint32. You can merge these and modify them (add/remove a range).
// The primary use of these is to manage Uid/Gid ranges for re-mapping

func minUint32(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

func maxUint32(x, y uint32) uint32 {
	if x > y {
		return x
	}
	return y
}

type Range struct {
	Start  uint32
	Length uint32
}

type Ranges struct {
	Ranges []Range
}

func (r *Ranges) Add(start, length uint32) {
	// The maximum value we can store is UINT32_MAX-1, because if start
	// is 0 and length is UINT32_MAX, then the first non-range item is
	// 0+UINT32_MAX. So, we limit the start and length here so all
	// elements in the ranges are in this area.
	if start == math.MaxUint32 {
		return
	}
	length = minUint32(length, math.MaxUint32-start)

	if length == 0 {
		return
	}

	for i := 0; i < len(r.Ranges); i++ {
		current := &r.Ranges[i]
		// Check if new range starts before current
		if start < current.Start {
			// Check if new range is completely before current
			if start+length < current.Start {
				// insert new range at i
				newr := make([]Range, len(r.Ranges)+1)
				copy(newr[0:i], r.Ranges[0:i])
				newr[i] = Range{Start: start, Length: length}
				copy(newr[i+1:], r.Ranges[i:])
				r.Ranges = newr

				return // All done
			}

			// ranges overlap, extend current backward to new start
			toExtendLen := current.Start - start
			current.Start -= toExtendLen
			current.Length += toExtendLen

			// And drop the extended part from new range
			start += toExtendLen
			length -= toExtendLen

			if length == 0 {
				return // That was all
			}

			// Move on to next case
		}

		if start >= current.Start && start < current.Start+current.Length {
			// New range overlaps current
			if start+length <= current.Start+current.Length {
				return // All overlapped, we're done
			}

			// New range extends past end of current
			overlapLen := (current.Start + current.Length) - start

			// And drop the overlapped part from current range
			start += overlapLen
			length -= overlapLen

			// Move on to next case
		}

		if start == current.Start+current.Length {
			// We're extending current
			current.Length += length

			// Might have to merge some old remaining ranges
			for i+1 < len(r.Ranges) &&
				r.Ranges[i+1].Start <= current.Start+current.Length {
				next := &r.Ranges[i+1]

				newEnd := maxUint32(current.Start+current.Length, next.Start+next.Length)

				current.Length = newEnd - current.Start

				copy(r.Ranges[i+1:], r.Ranges[i+2:])
				r.Ranges = r.Ranges[:len(r.Ranges)-1]
				current = &r.Ranges[i]
			}

			return // All done
		}
	}

	// New range remaining after last old range, append
	if length > 0 {
		r.Ranges = append(r.Ranges, Range{Start: start, Length: length})
	}
}

func (r *Ranges) Remove(start, length uint32) {
	// Limit ranges, see comment in Add
	if start == math.MaxUint32 {
		return
	}
	length = minUint32(length, math.MaxUint32-start)

	if length == 0 {
		return
	}

	for i := 0; i < len(r.Ranges); i++ {
		current := &r.Ranges[i]

		end := start + length
		currentStart := current.Start
		currentEnd := current.Start + current.Length

		if end > currentStart && start < currentEnd {
			remainingAtStart := uint32(0)
			remainingAtEnd := uint32(0)

			if start > currentStart {
				remainingAtStart = start - currentStart
			}

			if end < currentEnd {
				remainingAtEnd = currentEnd - end
			}

			switch {
			case remainingAtStart == 0 && remainingAtEnd == 0:
				// Remove whole range
				copy(r.Ranges[i:], r.Ranges[i+1:])
				r.Ranges = r.Ranges[:len(r.Ranges)-1]
				i-- // undo loop iter
			case remainingAtStart != 0 && remainingAtEnd != 0:
				// Range is split

				newr := make([]Range, len(r.Ranges)+1)
				copy(newr[0:i], r.Ranges[0:i])
				copy(newr[i+1:], r.Ranges[i:])
				newr[i].Start = currentStart
				newr[i].Length = remainingAtStart
				newr[i+1].Start = currentEnd - remainingAtEnd
				newr[i+1].Length = remainingAtEnd
				r.Ranges = newr
				i++ /* double loop iter */
			case remainingAtStart != 0:
				r.Ranges[i].Start = currentStart
				r.Ranges[i].Length = remainingAtStart
			default: /* remainingAtEnd != 0 */
				r.Ranges[i].Start = currentEnd - remainingAtEnd
				r.Ranges[i].Length = remainingAtEnd
			}
		}
	}
}

func (r *Ranges) Merge(other *Ranges) {
	for _, o := range other.Ranges {
		r.Add(o.Start, o.Length)
	}
}

func (r *Ranges) Copy() *Ranges {
	rs := make([]Range, len(r.Ranges))
	copy(rs, r.Ranges)
	return &Ranges{Ranges: rs}
}

func (r *Ranges) Length() uint32 {
	length := uint32(0)
	for _, rr := range r.Ranges {
		length += rr.Length
	}
	return length
}

func NewRangesEmpty() *Ranges {
	return &Ranges{Ranges: nil}
}

func NewRanges(start, length uint32) *Ranges {
	r := NewRangesEmpty()
	r.Add(start, length)

	return r
}

func parseEndpoint(str string, defaultVal uint32) uint32 {
	str = strings.TrimSpace(str)
	intVal, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return defaultVal
	}

	if intVal < 0 {
		return uint32(0)
	}
	if intVal > math.MaxUint32 {
		return uint32(math.MaxUint32)
	}
	return uint32(intVal)
}

// Ranges are specified inclusive. I.e. 1-3 is 1,2,3
func ParseRanges(str string) *Ranges {
	r := NewRangesEmpty()

	for _, part := range strings.Split(str, ",") {
		start, end, isPair := strings.Cut(part, "-")
		startV := parseEndpoint(start, 0)
		endV := startV
		if isPair {
			endV = parseEndpoint(end, math.MaxUint32)
		}
		if endV >= startV {
			r.Add(startV, endV-startV+1)
		}
	}

	return r
}
