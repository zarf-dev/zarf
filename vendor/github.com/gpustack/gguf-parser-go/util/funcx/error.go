package funcx

// NoError ignores the given error,
// it is usually a nice helper for chain function calling.
func NoError[T any](t T, _ error) T {
	return t
}

// NoError2 ignores the given error,
// it is usually a nice helper for chain function calling.
func NoError2[T, U any](t T, u U, _ error) (T, U) {
	return t, u
}

// NoError3 ignores the given error,
// it is usually a nice helper for chain function calling.
func NoError3[T, U, V any](t T, u U, v V, _ error) (T, U, V) {
	return t, u, v
}

// NoError4 ignores the given error,
// it is usually a nice helper for chain function calling.
func NoError4[T, U, V, W any](t T, u U, v V, w W, _ error) (T, U, V, W) {
	return t, u, v, w
}

// MustNoError is similar to NoError,
// but it panics if the given error is not nil,
// it is usually a nice helper for chain function calling.
func MustNoError[T any](t T, e error) T {
	if e != nil {
		panic(e)
	}
	return t
}

// MustNoError2 is similar to NoError2,
// but it panics if the given error is not nil,
// it is usually a nice helper for chain function calling.
func MustNoError2[T, U any](t T, u U, e error) (T, U) {
	if e != nil {
		panic(e)
	}
	return t, u
}

// MustNoError3 is similar to NoError3,
// but it panics if the given error is not nil,
// it is usually a nice helper for chain function calling.
func MustNoError3[T, U, V any](t T, u U, v V, e error) (T, U, V) {
	if e != nil {
		panic(e)
	}
	return t, u, v
}

// MustNoError4 is similar to NoError4,
// but it panics if the given error is not nil,
// it is usually a nice helper for chain function calling.
func MustNoError4[T, U, V, W any](t T, u U, v V, w W, e error) (T, U, V, W) {
	if e != nil {
		panic(e)
	}
	return t, u, v, w
}
