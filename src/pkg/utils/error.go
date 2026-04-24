// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"errors"
	"fmt"
)

// FilterErr error expects an error type and an error that implements the Unwrap() method
// Any errors included in the wrapped error that match the error type T are removed from the final joined error
func FilterErr[T error](joined error) (error, error) {
	var target T
	var filteredErrs []error

	unwrapped, ok := joined.(interface{ Unwrap() []error })

	if !ok {
		return nil, fmt.Errorf("error of type %T does not have method Unwrap()", joined)
	}

	errs := unwrapped.Unwrap()

	for _, err := range errs {
		if !errors.As(err, &target) {
			filteredErrs = append(filteredErrs, err)
		}
	}

	return errors.Join(filteredErrs...), nil
}
