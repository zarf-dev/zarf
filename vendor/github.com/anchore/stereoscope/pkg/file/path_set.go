package file

import (
	"slices"
)

type PathSet map[Path]struct{}

func NewPathSet(is ...Path) PathSet {
	// TODO: replace with single generic implementation that also incorporates other set implementations
	s := make(PathSet)
	s.Add(is...)
	return s
}

func (s PathSet) Size() int {
	return len(s)
}

func (s PathSet) Merge(other PathSet) {
	for _, i := range other.List() {
		s.Add(i)
	}
}

func (s PathSet) Add(ids ...Path) {
	for _, i := range ids {
		s[i] = struct{}{}
	}
}

func (s PathSet) Remove(ids ...Path) {
	for _, i := range ids {
		delete(s, i)
	}
}

func (s PathSet) Contains(i Path) bool {
	_, ok := s[i]
	return ok
}

func (s PathSet) Clear() {
	clear(s)
}

func (s PathSet) List() []Path {
	ret := make([]Path, 0, len(s))
	for i := range s {
		ret = append(ret, i)
	}
	return ret
}

func (s PathSet) Sorted() []Path {
	ids := s.List()

	slices.Sort(ids)

	return ids
}

func (s PathSet) ContainsAny(ids ...Path) bool {
	for _, i := range ids {
		_, ok := s[i]
		if ok {
			return true
		}
	}
	return false
}

type PathCountSet map[Path]int

func NewPathCountSet(is ...Path) PathCountSet {
	s := make(PathCountSet)
	s.Add(is...)
	return s
}

func (s PathCountSet) Add(ids ...Path) {
	for _, i := range ids {
		if _, ok := s[i]; !ok {
			s[i] = 1
			continue
		}
		s[i]++
	}
}

func (s PathCountSet) Remove(ids ...Path) {
	for _, i := range ids {
		if _, ok := s[i]; !ok {
			continue
		}

		s[i]--
		if s[i] <= 0 {
			delete(s, i)
		}
	}
}

func (s PathCountSet) Contains(i Path) bool {
	count, ok := s[i]
	return ok && count > 0
}
