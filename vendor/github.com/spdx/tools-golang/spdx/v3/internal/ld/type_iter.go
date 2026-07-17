package ld

import "iter"

func SliceOf[To, From any, List ~[]From](values List) []To {
	var out []To
	for _, v := range values {
		if cast, ok := any(v).(To); ok {
			out = append(out, cast)
		}
	}
	return out
}

type TypeSeq[Element, View any] iter.Seq2[Element, View]

func (s TypeSeq[Element, View]) Len() int {
	cnt := 0
	for range s {
		cnt++
	}
	return cnt
}

func NewTypeSeq[T any, E any](values []E, cast func(any) *T) TypeSeq[E, *T] {
	if values == nil {
		return func(yield func(E, *T) bool) {}
	}
	return func(yield func(E, *T) bool) {
		for _, value := range values {
			v := cast(value)
			if v != nil {
				if !yield(value, v) {
					return
				}
			}
		}
	}
}
