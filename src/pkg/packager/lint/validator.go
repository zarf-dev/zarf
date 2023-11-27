// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// Warning holds the location of warning and the message
type Warning struct {
	location string
	message  string
}

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	warnings           []Warning
	errors             []error
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
}

func (v Validator) Error() string {
	if !v.hasErrors() {
		return ""
	}
	errorMessage := validatorInvalidPrefix
	for _, errorStr := range v.errors {
		errorMessage = fmt.Sprintf("%s\n - %s", errorMessage, errorStr.Error())
	}
	return errorMessage
}

// func (v Validator) getFormatedWarning() string {
// 	if !v.hasWarnings() {
// 		return ""
// 	}
// 	return fmt.Sprintf("%s %s", validatorWarningPrefix, strings.Join(v.warnings, ", "))
// }

func (v Validator) printWarningTable() {
	if v.hasWarnings() {
		connectData := [][]string{}
		for _, warning := range v.warnings {
			connectData = append(connectData, []string{warning.location, warning.message})
		}
		header := []string{"Location", "Warning"}
		message.Table(header, connectData)
	}
}

func (v Validator) getFormatedSuccess() string {
	return fmt.Sprintf("Schema validation successful for %q", v.typedZarfPackage.Metadata.Name)
}

func (v Validator) hasWarnings() bool {
	return len(v.warnings) > 0
}

func (v Validator) hasErrors() bool {
	return len(v.errors) > 0
}

func (v Validator) isSuccess() bool {
	return !v.hasWarnings() && !v.hasErrors()
}

func (v *Validator) addWarning(location string, message string) {
	v.warnings = append(v.warnings, Warning{location, message})
}

func (v *Validator) addError(err error) {
	v.errors = append(v.errors, err)
}

// DisplayFormattedMessage Displays the message to the user with proper warnings, failures, or success
// Will exit if there are errors
func (v Validator) DisplayFormattedMessage() {
	if v.hasWarnings() {
		v.printWarningTable()
	}
	if v.hasErrors() {
		message.Fatal(v, v.Error())
	}
	if v.isSuccess() {
		message.Success(v.getFormatedSuccess())
	}
}
