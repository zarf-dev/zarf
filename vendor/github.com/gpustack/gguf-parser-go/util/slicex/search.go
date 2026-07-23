package slicex

import "golang.org/x/exp/constraints"

// UpperBound returns an index of the first element that is greater than value.
func UpperBound[T constraints.Integer | constraints.Float](s []T, e T) int {
	l, r := 0, len(s)
	for l < r {
		m := l + (r-l)/2
		if s[m] <= e {
			l = m + 1
		} else {
			r = m
		}
	}
	return l
}
