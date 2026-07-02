package ld

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Validator interface should be implemented by
type Validator interface {
	Validate() error
}

// ValidateGraph recursively calls all Validator(s) on all values in the graph and returns a joined error
// of all the errors found
func ValidateGraph(graph any) error {
	var errs []error
	err := VisitObjectGraph(graph, func(path []any, validator Validator) error {
		for _, err := range flattenErrors(validator.Validate()) {
			errs = append(errs, newValidationError(err, append(path, baseType(reflect.TypeOf(validator)))...)) // ok to append to mutable path here: it is only stringified
		}
		return nil
	})
	if err != nil {
		return err
	}
	return JoinErrors(errs...)
}

// Validation functions are provided to ValidateProperty calls and will be executed if a property is set
type Validation[T any] func(value T) error

// ValidateProperty is used by generated code, typically, to validate a specific property against all defined validations
// it might have such as whether it is required, matches a pattern, has enough elements, or is in a specific defined set.
// This function needs to be called with a struct reference and a pointer to the property, e.g. ValidateProperty(obj, &obj.Prop, ...)
// due to looking up field tags and names based on the property reference. This will panic if called any other way.
func ValidateProperty[T any](object any, property *T, validations ...Validation[T]) error {
	value := reflect.ValueOf(property)

	// object is always a pointer to the base struct
	o := reflect.ValueOf(object).Elem()
	var f reflect.StructField
	for i := 0; i < o.NumField(); i++ {
		if o.Field(i).Addr() == value {
			f = o.Type().Field(i)
			break
		}
	}
	if f.Name == "" {
		panic(fmt.Sprintf("property: %v not found in object: %v", property, object))
	}

	var errs []error
	if f.Anonymous { // inherited type validation
		if validator, ok := any(property).(Validator); ok {
			errs = flattenErrors(validator.Validate())
		}
	} else {
		// value is pointer to field, which always points to a valid elem
		if value.Elem().IsZero() {
			if f.Tag.Get("required") == "true" {
				return newValidationError(fmt.Errorf("required"), f)
			}
			return nil // don't process other validators, this is simply not set and not required
		}
	}

	for _, validation := range validations {
		err := validation(*property)
		if err != nil {
			errs = append(errs, newValidationError(err, f))
		}
	}
	return JoinErrors(errs...)
}

// ValidateIRI validates the value is in the set of provided values by comparing IDs
func ValidateIRI[T any](values ...T) Validation[T] {
	ids := map[string]struct{}{}
	for _, v := range values {
		id, err := GetID(v)
		if err != nil {
			panic(err)
		}
		ids[id] = struct{}{}
	}
	return func(value T) error {
		id, err := GetID(value)
		if err != nil {
			return err
		}
		if _, ok := ids[id]; ok {
			return nil
		}
		prefix, parts := trimCommonPrefixes(sortedKeys(ids))
		return fmt.Errorf("value is not allowed: '%v', expected: %s{%s}", id, prefix, strings.Join(parts, " | "))
	}
}

// ValidateAll applies the provided validation to all elements in a slice
func ValidateAll[T any](validation Validation[T]) Validation[[]T] {
	return func(values []T) error {
		var errs []error
		for i, value := range values {
			err := validation(value)
			if err != nil {
				errs = append(errs, newValidationError(err, i))
			}
		}
		return JoinErrors(errs...)
	}
}

// ValidateMinCount validates there are a minimum number of elements present in a slice
func ValidateMinCount[S ~[]T, T any](minCount int) Validation[S] {
	return func(values S) error {
		if minCount > len(values) {
			return fmt.Errorf("must have at least: %v item(s), got: %v", minCount, len(values))
		}
		return nil
	}
}

// ValidateMaxCount validates that the maximum number of elements has not been exceeded
func ValidateMaxCount[T any](maxCount int) Validation[[]T] {
	return func(values []T) error {
		if maxCount < len(values) {
			return fmt.Errorf("must have fewer than: %v item(s), got: %v", maxCount, len(values))
		}
		return nil
	}
}

// ValidateExpression validates that the value matches the provided expression
func ValidateExpression(expression string) Validation[string] {
	return func(value string) error {
		r, err := regexp.Compile(expression)
		if err != nil {
			return fmt.Errorf("invalid expression: %s", expression)
		}
		if !r.MatchString(value) {
			return fmt.Errorf("must match expression: %s: value: %v", expression, value)
		}
		return nil
	}
}
