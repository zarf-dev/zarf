package phpserialize

import (
	"reflect"
	"strings"
)

// numericalValue returns the float64 representation of a value if it is a
// numerical type - integer, unsigned integer or float. If the value is not a
// numerical type then the second argument is false and the value returned
// should be disregarded.
func numericalValue(value reflect.Value) (float64, bool) {
	switch value.Type().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64:
		return float64(value.Int()), true

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(value.Uint()), true

	case reflect.Float32, reflect.Float64:
		return value.Float(), true

	default:
		return 0, false
	}
}

// lessValue compares two reflect.Value instances and returns true if a is
// considered to be less than b.
//
// This function is used to sort keys for what amounts to associate arrays and
// objects in PHP. These are represented as slices and maps in Go. Since Go
// randomised map iterators we need to make sure we always return the keys of an
// associative array or object in a predicable order.
//
// The keys can be numerical, strings or a combination of both. We treat numbers
// (integers, unsigned integers and floats) as always less than strings. Numbers
// are ordered by magnitude (ignoring types) and strings are orders
// lexicographically.
//
// If keys are of any other type the behavior of the comparison is undefined. If
// there is a legitimate reason why keys could be other types then this function
// should be updated accordingly.
func lessValue(a, b reflect.Value) bool {
	aValue, aNumerical := numericalValue(a)
	bValue, bNumerical := numericalValue(b)

	if aNumerical && bNumerical {
		return aValue < bValue
	}

	if !aNumerical && !bNumerical {
		// In theory this should mean they are both strings. In reality
		// they could be any other type and the String() representation
		// will be something like "<bool>" if it is not a string. Since
		// distinct values of non-strings still return the same value
		// here that's what makes the ordering undefined.
		return strings.Compare(a.String(), b.String()) < 0
	}

	// Numerical values are always treated as less than other types
	// (including strings that might represent numbers themselves). The
	// inverse is also true.
	return aNumerical && !bNumerical
}
