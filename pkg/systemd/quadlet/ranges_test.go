package quadlet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRanges_Creation(t *testing.T) {
	empty := NewRangesEmpty()

	assert.Equal(t, empty.Length(), uint32(0))

	one := NewRanges(17, 42)
	assert.Equal(t, one.Ranges[0].Start, uint32(17))
	assert.Equal(t, one.Ranges[0].Length, uint32(42))
}

func TestRanges_Single(t *testing.T) {
	/* Before */
	r := NewRanges(10, 10)

	r.Add(0, 9)

	assert.Equal(t, len(r.Ranges), 2)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(9))
	assert.Equal(t, r.Ranges[1].Start, uint32(10))
	assert.Equal(t, r.Ranges[1].Length, uint32(10))

	/* just before */
	r = NewRanges(10, 10)

	r.Add(0, 10)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(20))

	/* before + inside */
	r = NewRanges(10, 10)

	r.Add(0, 19)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(20))

	/* before + inside, whole */
	r = NewRanges(10, 10)

	r.Add(0, 20)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(20))

	/* before + inside + after */
	r = NewRanges(10, 10)

	r.Add(0, 30)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(30))

	/* just inside */
	r = NewRanges(10, 10)

	r.Add(10, 5)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))

	/* inside */
	r = NewRanges(10, 10)

	r.Add(12, 5)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))

	/* inside at end */
	r = NewRanges(10, 10)

	r.Add(15, 5)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))

	/* inside + after */
	r = NewRanges(10, 10)

	r.Add(15, 10)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(15))

	/* just after */
	r = NewRanges(10, 10)

	r.Add(20, 10)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(20))

	/* after */
	r = NewRanges(10, 10)

	r.Add(21, 10)

	assert.Equal(t, len(r.Ranges), 2)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(21))
	assert.Equal(t, r.Ranges[1].Length, uint32(10))
}

func TestRanges_Multi(t *testing.T) {
	base := NewRanges(10, 10)
	base.Add(50, 10)
	base.Add(30, 10)

	/* Test copy */
	r := base.Copy()

	assert.Equal(t, len(r.Ranges), 3)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(30))
	assert.Equal(t, r.Ranges[1].Length, uint32(10))
	assert.Equal(t, r.Ranges[2].Start, uint32(50))
	assert.Equal(t, r.Ranges[2].Length, uint32(10))

	/* overlap everything */
	r = base.Copy()

	r.Add(0, 100)

	assert.Equal(t, len(r.Ranges), 1)
	assert.Equal(t, r.Ranges[0].Start, uint32(0))
	assert.Equal(t, r.Ranges[0].Length, uint32(100))

	/* overlap middle */
	r = base.Copy()

	r.Add(25, 10)

	assert.Equal(t, len(r.Ranges), 3)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(25))
	assert.Equal(t, r.Ranges[1].Length, uint32(15))
	assert.Equal(t, r.Ranges[2].Start, uint32(50))
	assert.Equal(t, r.Ranges[2].Length, uint32(10))

	/* overlap last */
	r = base.Copy()

	r.Add(45, 10)

	assert.Equal(t, len(r.Ranges), 3)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(30))
	assert.Equal(t, r.Ranges[1].Length, uint32(10))
	assert.Equal(t, r.Ranges[2].Start, uint32(45))
	assert.Equal(t, r.Ranges[2].Length, uint32(15))
}

func TestRanges_Remove(t *testing.T) {
	base := NewRanges(10, 10)
	base.Add(50, 10)
	base.Add(30, 10)

	/* overlap all */
	r := base.Copy()

	r.Remove(0, 100)

	assert.Equal(t, len(r.Ranges), 0)

	/* overlap middle 1 */

	r = base.Copy()

	r.Remove(25, 20)

	assert.Equal(t, len(r.Ranges), 2)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(50))
	assert.Equal(t, r.Ranges[1].Length, uint32(10))

	/* overlap middle 2 */

	r = base.Copy()

	r.Remove(25, 10)

	assert.Equal(t, len(r.Ranges), 3)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(35))
	assert.Equal(t, r.Ranges[1].Length, uint32(5))
	assert.Equal(t, r.Ranges[2].Start, uint32(50))
	assert.Equal(t, r.Ranges[2].Length, uint32(10))

	/* overlap middle 3 */
	r = base.Copy()

	r.Remove(35, 10)

	assert.Equal(t, len(r.Ranges), 3)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(30))
	assert.Equal(t, r.Ranges[1].Length, uint32(5))
	assert.Equal(t, r.Ranges[2].Start, uint32(50))
	assert.Equal(t, r.Ranges[2].Length, uint32(10))

	/* overlap middle 4 */

	r = base.Copy()

	r.Remove(34, 2)

	assert.Equal(t, len(r.Ranges), 4)
	assert.Equal(t, r.Ranges[0].Start, uint32(10))
	assert.Equal(t, r.Ranges[0].Length, uint32(10))
	assert.Equal(t, r.Ranges[1].Start, uint32(30))
	assert.Equal(t, r.Ranges[1].Length, uint32(4))
	assert.Equal(t, r.Ranges[2].Start, uint32(36))
	assert.Equal(t, r.Ranges[2].Length, uint32(4))
	assert.Equal(t, r.Ranges[3].Start, uint32(50))
	assert.Equal(t, r.Ranges[3].Length, uint32(10))
}
