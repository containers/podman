package graphdriver

import "sync"

type minfo struct {
	check bool
	count int
}

// RefCounter is a generic counter for use by graphdriver Get/Put calls
type RefCounter struct {
	counts  map[string]*minfo
	mu      sync.Mutex
	checker Checker
}

// NewRefCounter returns a new RefCounter
func NewRefCounter(c Checker) *RefCounter {
	return &RefCounter{
		checker: c,
		counts:  make(map[string]*minfo),
	}
}

// Increment increases the ref count for the given id and returns the current count
func (c *RefCounter) Increment(path string) int {
	return c.incdec(path, func(minfo *minfo) {
		minfo.count++
	})
}

// Decrement decreases the ref count for the given id and returns the current count
func (c *RefCounter) Decrement(path string) int {
	return c.incdec(path, func(minfo *minfo) {
		minfo.count--
	})
}

func (c *RefCounter) incdec(path string, infoOp func(minfo *minfo)) int {
	c.mu.Lock()
	m := c.counts[path]
	if m == nil {
		m = &minfo{}
		c.counts[path] = m
	}
	// if we are checking this path for the first time check to make sure
	// if it was already mounted on the system and make sure we have a correct ref
	// count if it is mounted as it is in use.
	if !m.check {
		m.check = true
		if c.checker.IsMounted(path) {
			m.count++
		}
	} else if !c.checker.IsMounted(path) {
		// if the unmount was performed outside of this process (e.g. conmon cleanup)
		// the ref counter would lose track of it.  Check if it is still mounted.
		m.count = 0
	}
	infoOp(m)
	count := m.count
	if count <= 0 {
		// If the mounted path has been decremented enough have no references,
		// then its entry can be removed.
		delete(c.counts, path)
	}
	c.mu.Unlock()
	return count
}
