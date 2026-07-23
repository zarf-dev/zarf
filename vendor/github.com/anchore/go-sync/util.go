package sync

import "iter"

// ToSeq converts a slice to an iter.Seq
func ToSeq[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

// ToIndexSeq converts a []T to an iter.Seq2[int,T] where the index is the first parameter
func ToIndexSeq[T any](values []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for index, value := range values {
			if !yield(index, value) {
				return
			}
		}
	}
}

// ToSlice takes an iter.Seq and returns a slice of the values returned
func ToSlice[T any](values iter.Seq[T]) (everything []T) {
	for v := range values {
		everything = append(everything, v)
	}
	return everything
}

// ToSeq2 converts a map[K]V to an iter.Seq2[K,V]
func ToSeq2[K comparable, V any](values map[K]V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, value := range values {
			if !yield(key, value) {
				return
			}
		}
	}
}

// keyValue is used to for Seq2 and related sequence conversions
type keyValue[K, V any] struct {
	Key   K
	Value V
}

// toKeyValueIterator converts an iter.Seq2[K,V] to an iter.Seq[keyValue[K,V]]
func toKeyValueIterator[From1, From2 any](iterator iter.Seq2[From1, From2]) iter.Seq[keyValue[From1, From2]] {
	return func(yield func(keyValue[From1, From2]) bool) {
		for key, value := range iterator {
			if !yield(keyValue[From1, From2]{Key: key, Value: value}) {
				return
			}
		}
	}
}
