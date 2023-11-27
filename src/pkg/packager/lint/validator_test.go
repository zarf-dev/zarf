// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidator(t *testing.T) {
	t.Run("Validator Error formatting", func(t *testing.T) {
		error1 := errors.New("components.0.import: Additional property not-path is not allowed")
		error2 := errors.New("components.1.import.path: Invalid type. Expected: string, given: integer")
		validator := Validator{errors: []error{error1, error2}}
		errorMessage := fmt.Sprintf("%s\n - %s\n - %s", validatorInvalidPrefix, error1.Error(), error2.Error())
		require.EqualError(t, validator, errorMessage)
	})

	// t.Run("Validator Warning formatting", func(t *testing.T) {
	// 	warning1 := "components.0.import: Additional property not-path is not allowed"
	// 	warning2 := "components.1.import.path: Invalid type. Expected: string, given: integer"
	// 	validator := Validator{warnings: []string{warning1, warning2}}
	// 	message := fmt.Sprintf("%s %s, %s", validatorWarningPrefix, warning1, warning2)
	// 	require.Equal(t, validator.getFormatedWarning(), message)
	// })
}
