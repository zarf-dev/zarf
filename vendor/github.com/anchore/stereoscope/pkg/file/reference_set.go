package file

// ReferenceSet is a set of file references
type ReferenceSet map[ID]struct{}

// NewFileReferenceSet creates a new ReferenceSet instance.
func NewFileReferenceSet() ReferenceSet {
	return make(ReferenceSet)
}

// Add the ID of the given file reference to the set.
func (s ReferenceSet) Add(ref Reference) {
	s[ref.ID()] = struct{}{}
}

// Remove the ID of the given file reference from the set.
func (s ReferenceSet) Remove(ref Reference) {
	delete(s, ref.ID())
}

// Contains indicates if the given file reference ID is already contained in this set.
func (s ReferenceSet) Contains(ref Reference) bool {
	_, ok := s[ref.ID()]
	return ok
}
