package ordered

// Tuple is used for storing values in Map.
type Tuple[K comparable, V any] struct {
	Key   K
	Value V

	deleted bool
}

// TupleSS is a convenience alias to reduce keyboard wear.
type TupleSS = Tuple[string, string]

// TupleSA is a convenience alias to reduce keyboard wear.
type TupleSA = Tuple[string, any]
