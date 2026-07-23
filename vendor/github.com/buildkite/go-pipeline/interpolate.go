package pipeline

import (
	"github.com/buildkite/go-pipeline/ordered"
	"github.com/buildkite/interpolate"
)

// This file contains helpers for recursively interpolating all the strings in
// pipeline objects.

// stringTransformer implementations mutate strings.
type stringTransformer interface {
	Transform(string) (string, error)
}

// envInterpolator returns a reusable string transform that replaces
// variables (${FOO}) with their values from a map.
type envInterpolator struct {
	env interpolate.Env
}

// Transform calls interpolate.Interpolate to transform the string.
func (e envInterpolator) Transform(s string) (string, error) {
	return interpolate.Interpolate(e.env, s)
}

// selfInterpolater describes types that can interpolate themselves in-place.
// They can use the string transformer on string fields, or use
// interpolate{Slice,Map,OrderedMap,Any} on their other contents, to do this.
type selfInterpolater interface {
	interpolate(stringTransformer) error
}

// interpolateAny interpolates most things, mostly in-place. When passed a
// string, it returns a new string. Anything it doesn't know how to interpolate
// is returned unaltered.
func interpolateAny[T any](tf stringTransformer, o T) (T, error) {
	// The box-typeswitch-unbox dance is required because the Go compiler
	// has no type switch for type parameters.
	var err error
	a := any(o)

	switch t := a.(type) {
	case selfInterpolater:
		err = t.interpolate(tf)

	case *string:
		err = interpolateString(tf, t)

	case string:
		a, err = tf.Transform(t)

	case []any:
		err = interpolateSlice(tf, t)

	case []string:
		err = interpolateSlice(tf, t)

	case map[string]any:
		err = interpolateMap(tf, t)

	case map[string]string:
		err = interpolateMap(tf, t)

	case *ordered.Map[string, any]:
		err = interpolateOrderedMap(tf, t)

	case *ordered.Map[string, string]:
		err = interpolateOrderedMap(tf, t)

	default:
		return o, nil
	}

	// This happens if T is an interface type and o was interface-nil to begin
	// with. (You can't type assert interface-nil.)
	if a == nil {
		var zt T
		return zt, err
	}
	return a.(T), err
}

// interpolateString is a helper to interpolate a string field in-place
// (requiring a pointer to the field).
func interpolateString(tf stringTransformer, p *string) error {
	if p == nil {
		return nil
	}
	s, err := tf.Transform(*p)
	if err != nil {
		return err
	}
	*p = s
	return nil
}

// interpolateSlice applies interpolateAny over any type of slice. Values in the
// slice are updated in-place.
func interpolateSlice[E any, S ~[]E](tf stringTransformer, s S) error {
	for i, e := range s {
		// It could be a string, so replace the old value with the new.
		inte, err := interpolateAny(tf, e)
		if err != nil {
			return err
		}
		s[i] = inte
	}
	return nil
}

// interpolateMapValues applies interpolateAny over the values of any type of
// map. The map is altered in-place.
func interpolateMapValues[K comparable, V any, M ~map[K]V](tf stringTransformer, m M) error {
	for k, v := range m {
		// V could be string, so be sure to replace the old value with the new.
		intv, err := interpolateAny(tf, v)
		if err != nil {
			return err
		}
		m[k] = intv
	}
	return nil
}

// interpolateMap applies interpolateAny over both keys and values of any type
// of map. The map is altered in-place.
func interpolateMap[K comparable, V any, M ~map[K]V](tf stringTransformer, m M) error {
	for k, v := range m {
		// We interpolate both keys and values.
		intk, err := interpolateAny(tf, k)
		if err != nil {
			return err
		}

		// V could be string, so be sure to replace the old value with the new.
		intv, err := interpolateAny(tf, v)
		if err != nil {
			return err
		}

		// If the key changed due to interpolation, delete the old key.
		if k != intk {
			delete(m, k)
		}
		m[intk] = intv
	}
	return nil
}

// interpolateOrderedMap applies interpolateAny over any type of ordered.Map.
// The map is altered in-place.
func interpolateOrderedMap[K comparable, V any](tf stringTransformer, m *ordered.Map[K, V]) error {
	return m.Range(func(k K, v V) error {
		// We interpolate both keys and values.
		intk, err := interpolateAny(tf, k)
		if err != nil {
			return err
		}
		intv, err := interpolateAny(tf, v)
		if err != nil {
			return err
		}

		m.Replace(k, intk, intv)
		return nil
	})
}
