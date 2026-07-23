// Package warning provides a warning error type (a "soft" multi-error).
//
// Example usage that catches and produces warnings:
//
//	func unmarshalThings(src []any) ([]thing, error) {
//		var dst []thing
//		var warns []error
//		for i, a := range src {
//			// Unmarshal can produce warnings or errors
//			var x thing
//			err := Unmarshal(a, &x)
//			if w := warning.As(err); w != nil {
//				warns = append(warns, w.Wrapf("while unmarshaling item %d of %d", i, len(src)))
//			} else if err != nil {
//				return err
//			}
//			dst = append(dst, x)
//		}
//		// Since it only had warnings, return both dst and a wrapper warning.
//		return dst, warning.Wrap(warns...)
//	}
package warning

import (
	"fmt"
	"strings"
)

// Warning is a kind of error that exists so that parsing/processing functions
// can produce warnings that do not abort part-way, but can still be reported
// via the error interface and logged.
// Warning can wrap zero or more errors (that may also be warnings).
// A Warning with zero wrapped errors is considered equivalent to a nil warning.
type Warning struct {
	message string
	errs    []error
}

// New creates a new warning that wraps one or more errors. Note that msg is *not* a format string.
func New(msg string, errs ...error) *Warning { return &Warning{message: msg, errs: errs} }

// Newf creates a new warning that wraps a single error created with fmt.Errorf.
// (This enables the use of %w for wrapping other errors.)
func Newf(f string, x ...any) *Warning { return &Warning{errs: []error{fmt.Errorf(f, x...)}} }

// Wrap returns a new warning with no message that wraps one or more errors.
// If passed no errors, it returns nil.
// If passed a single error that is a warning, it returns that warning.
// This is a convenient way to downgrade an error to a warning, and also handle
// cases where no warnings occurred.
func Wrap(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 && Is(errs[0]) {
		return errs[0]
	}
	return &Warning{errs: errs}
}

// Wrapf wraps a single error in a warning, with a message created using a format
// string.
func Wrapf(err error, f string, x ...any) error {
	return &Warning{message: fmt.Sprintf(f, x...), errs: []error{err}}
}

// Is reports if err is a *Warning. nil is not considered to be a warning.
// This is distinct from errors.Is because an error is a warning only if the top
// level of the tree is a warning (not if any of the recursively-unwrapped
// errors are).
func Is(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*Warning)
	return ok
}

// As checks if err is a *Warning, and returns it. If it is nil or not a
// *Warning, it returns nil.  This is distinct from errors.As because an error
// is a warning only if the top level of the tree is a warning (not if any of
// the recursively-unwrapped errors are).
func As(err error) *Warning {
	if err == nil {
		return nil
	}
	w, _ := err.(*Warning)
	return w
}

// Error returns a string that generally looks like:
//
//	w.message
//	  ↳ w.errs[0].Error()
//	  ↳ w.errs[1].Error()
//	  ↳ ...
//
// If the warning has no message, it is omitted. If there is no message and also
// only one child error, that child error's Error() is returned directly.
// Otherwise, Error prepends indentation to sub-error messages that span
// multiple lines to make them print nicely.
func (w *Warning) Error() string {
	if w.message == "" && len(w.errs) == 1 {
		return w.errs[0].Error()
	}
	b := new(strings.Builder)
	if w.message != "" {
		fmt.Fprintln(b, w.message)
	}
	for _, err := range w.errs {
		if err == nil {
			continue
		}
		fmt.Fprint(b, "  ↳ ")
		for i, line := range strings.Split(err.Error(), "\n") {
			if i > 0 {
				fmt.Fprint(b, "    ")
			}
			fmt.Fprintf(b, "%s\n", line)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Unwrap returns all errors directly wrapped by this warning.
func (w *Warning) Unwrap() []error { return w.errs }

// Wrapf wraps this warning in a new warning with a message using a format.
// If w doesn't already have a message, it sets the message and returns w.
func (w *Warning) Wrapf(f string, x ...any) *Warning {
	msg := fmt.Sprintf(f, x...)
	if w.message == "" {
		w.message = msg
		return w
	}
	return &Warning{message: msg, errs: []error{w}}
}

// Append appends errs as child errors of this warning.
func (w *Warning) Append(errs ...error) *Warning {
	w.errs = append(w.errs, errs...)
	return w
}
