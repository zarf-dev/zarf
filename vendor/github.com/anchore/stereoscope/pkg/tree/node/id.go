package node

import "slices"

type ID string

type IDSet map[ID]struct{}

func NewIDSet(is ...ID) IDSet {
	// TODO: replace with single generic implementation that also incorporates other set implementations
	s := make(IDSet)
	s.Add(is...)
	return s
}

func (s IDSet) Size() int {
	return len(s)
}

func (s IDSet) Merge(other IDSet) {
	for _, i := range other.List() {
		s.Add(i)
	}
}

func (s IDSet) Add(ids ...ID) {
	for _, i := range ids {
		s[i] = struct{}{}
	}
}

func (s IDSet) Remove(ids ...ID) {
	for _, i := range ids {
		delete(s, i)
	}
}

func (s IDSet) Contains(i ID) bool {
	_, ok := s[i]
	return ok
}

func (s IDSet) Clear() {
	clear(s)
}

func (s IDSet) List() []ID {
	ret := make([]ID, 0, len(s))
	for i := range s {
		ret = append(ret, i)
	}
	return ret
}

func (s IDSet) Sorted() []ID {
	ids := s.List()

	slices.Sort(ids)

	return ids
}

func (s IDSet) ContainsAny(ids ...ID) bool {
	for _, i := range ids {
		_, ok := s[i]
		if ok {
			return true
		}
	}
	return false
}
