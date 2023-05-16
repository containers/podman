package set

import "golang.org/x/exp/maps"

// FIXME:
// - Docstrings
// - This should be in a public library somewhere

type Set[E comparable] struct {
	m map[E]struct{}
}

func New[E comparable]() *Set[E] {
	return &Set[E]{
		m: map[E]struct{}{},
	}
}

func NewWithValues[E comparable](values ...E) *Set[E] {
	s := New[E]()
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (s *Set[E]) Add(v E) {
	s.m[v] = struct{}{} // Possibly writing the same struct{}{} presence marker again.
}

func (s *Set[E]) Delete(v E) {
	delete(s.m, v)
}

func (s *Set[E]) Contains(v E) bool {
	_, ok := s.m[v]
	return ok
}

func (s *Set[E]) Empty() bool {
	return len(s.m) == 0
}

func (s *Set[E]) Values() []E {
	return maps.Keys(s.m)
}
