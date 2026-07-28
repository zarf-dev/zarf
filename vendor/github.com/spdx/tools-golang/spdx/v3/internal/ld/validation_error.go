package ld

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// JoinErrors returns errors.Join'd errors, taking into account nested joined errors, flattening these to a single joined set
func JoinErrors(errs ...error) error {
	var out []error
	for _, err := range errs {
		out = append(out, flattenErrors(err)...)
	}
	switch len(out) {
	case 0:
		return nil
	case 1:
		return out[0]
	default:
		return errors.Join(out...)
	}
}

func flattenErrors(err error) []error {
	var out []error
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range joined.Unwrap() {
			out = append(out, flattenErrors(e)...)
		}
	} else {
		if err != nil {
			return []error{err}
		}
	}
	return out
}

var validatorInterface = reflect.TypeOf((*Validator)(nil)).Elem()

type validationError struct {
	Path string
	Err  error
}

func StringifyPath(pathSlice []any) string {
	path := strings.Builder{}
	for i := 0; i < len(pathSlice); i++ {
		part := pathSlice[i]
		switch p := part.(type) {
		case int:
			_, _ = path.WriteString("[")
			_, _ = path.WriteString(strconv.Itoa(p))
			_, _ = path.WriteString("]")
		case reflect.StructField:
			if !p.Anonymous {
				_, _ = path.WriteString(".")
				_, _ = path.WriteString(p.Name)
			}
		case reflect.Type:
			_, _ = path.WriteString("<")
			_, _ = path.WriteString(p.Name())
			_, _ = path.WriteString(">")
		default:
			_, _ = path.WriteString("/")
			_, _ = path.WriteString(fmt.Sprint(p))
		}
	}
	return path.String()
}

func (v *validationError) Error() string {
	return v.Path + ": " + v.Err.Error()
}

func newValidationError(err error, path ...any) *validationError {
	// if the error is a validation error, prepend the path
	if vErr, ok := err.(*validationError); ok {
		vErr.Path = StringifyPath(path) + " " + vErr.Path
		return vErr
	}
	return &validationError{
		Path: StringifyPath(path),
		Err:  err,
	}
}
