package entities

import (
	"strings"
)

type stringSet struct {
	m map[string]struct{}
}

func NewStringSet(elem ...string) *stringSet {
	s := &stringSet{}
	s.m = make(map[string]struct{}, len(elem))
	for _, e := range elem {
		s.Add(e)
	}
	return s
}

func (s *stringSet) Add(elem string) {
	s.m[elem] = struct{}{}
}

func (s *stringSet) Remove(elem string) {
	delete(s.m, elem)
}

func (s *stringSet) Contains(elem string) bool {
	_, ok := s.m[elem]
	return ok
}

func (s *stringSet) Elements() []string {
	keys := make([]string, len(s.m))
	i := 0
	for k := range s.m {
		keys[i] = k
		i++
	}
	return keys
}

func (s *stringSet) String() string {
	return strings.Join(s.Elements(), ", ")
}
