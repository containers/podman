package entities

import (
	"strings"
)

type StringSet struct {
	m map[string]struct{}
}

func NewStringSet(elem ...string) *StringSet {
	s := &StringSet{}
	s.m = make(map[string]struct{}, len(elem))
	for _, e := range elem {
		s.Add(e)
	}
	return s
}

func (s *StringSet) Add(elem string) {
	s.m[elem] = struct{}{}
}

func (s *StringSet) Remove(elem string) {
	delete(s.m, elem)
}

func (s *StringSet) Contains(elem string) bool {
	_, ok := s.m[elem]
	return ok
}

func (s *StringSet) Elements() []string {
	keys := make([]string, len(s.m))
	i := 0
	for k := range s.m {
		keys[i] = k
		i++
	}
	return keys
}

func (s *StringSet) String() string {
	return strings.Join(s.Elements(), ", ")
}
