package sync

import "iter"

type List[T comparable] struct {
	Locking
	values []T
}

// ----------------- Collection functions -----------------

func (s *List[T]) Append(value T) {
	defer s.Lock()()
	s.values = append(s.values, value)
}

func (s *List[T]) Remove(value T) {
	defer s.Lock()()
	idx := s.indexOf(value)
	if idx >= 0 {
		_, _ = s.removeIndex(idx)
	}
}

func (s *List[T]) Contains(value T) bool {
	defer s.RLock()()
	return s.indexOf(value) >= 0
}

func (s *List[T]) Len() int {
	defer s.RLock()()
	return len(s.values)
}

// ----------------- Queue functions -----------------

func (s *List[T]) Enqueue(value T) {
	s.Append(value)
}

func (s *List[T]) Dequeue() (value T, ok bool) {
	defer s.Lock()()
	if len(s.values) == 0 {
		return value, false
	}
	value = (s.values)[0]
	_, _ = s.removeIndex(0)
	return value, true
}

// ----------------- Stack functions -----------------

func (s *List[T]) Push(value T) {
	s.Append(value)
}

func (s *List[T]) Pop() (value T, ok bool) {
	defer s.Lock()()
	last := len(s.values) - 1
	if last >= 0 {
		v := (s.values)[last]
		s.values = (s.values)[0:last]
		return v, true
	}
	return value, false
}

func (s *List[T]) Peek() (value T, ok bool) {
	defer s.RLock()()
	last := len(s.values) - 1
	if last >= 0 {
		return (s.values)[last], true
	}
	return value, false
}

// ----------------- Iterator functions -----------------

// Seq is an iter.Seq compatible iterator function with a read lock, as such it is not possible to
// modify this list during the loop -- use Values() to obtain a copy for those purposes
func (s *List[T]) Seq(fn func(value T) bool) {
	defer s.RLock()()
	for _, v := range s.values {
		if !fn(v) {
			return
		}
	}
}

// ----------------- other utility functions -----------------

// Values returns a slice containing all the values at the time of the call, this should be used
// sparingly as it is only a snapshot of the current values
func (s *List[T]) Values() []T {
	defer s.RLock()()
	return s.copyValues()
}

// copyValues creates a copy of the values and returns it, without any locking
func (s *List[T]) copyValues() []T {
	out := make([]T, len(s.values))
	copy(out, s.values)
	return out
}

// Clear removes all values
func (s *List[T]) Clear() {
	defer s.Lock()()
	s.values = nil
}

func (s *List[T]) RemoveAll(values iter.Seq[T]) {
	defer s.Lock()()
	for value := range values {
		s.Remove(value)
	}
}

func (s *List[T]) Update(updater func(values []T) []T) {
	defer s.Lock()()
	s.values = updater(s.values)
}

// removeIndex removes the index from the list, returns the value and true if a value was removed
func (s *List[T]) removeIndex(index int) (value T, ok bool) {
	last := len(s.values) - 1
	if index < 0 || index > last {
		return value, false
	}
	value = (s.values)[index]
	switch index {
	case 0:
		if last == 0 {
			s.values = nil
		} else {
			s.values = (s.values)[1:]
		}
	case last:
		s.values = (s.values)[:last]
	default:
		s.values = append((s.values)[0:index], (s.values)[index+1:]...)
	}
	return value, true
}

func (s *List[T]) indexOf(value T) (index int) {
	for i, v := range s.values {
		if value == v {
			return i
		}
	}
	return -1
}

var _ interface {
	Lockable
	Collection[int]
	Queue[int]
	Stack[int]
} = (*List[int])(nil)
