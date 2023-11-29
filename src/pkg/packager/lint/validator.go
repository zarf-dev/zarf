// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fatih/color"
)

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	warnings           []string
	errors             []error
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasWarnings() && !v.hasErrors() {
		message.Success(fmt.Sprintf("Schema validation successful for %q", v.typedZarfPackage.Metadata.Name))
	}
	v.printValidationTable()
}

// IsSuccess returns true if there are not any errors
func (v Validator) IsSuccess() bool {
	return !v.hasErrors()
}

func (v Validator) printValidationTable() {
	if v.hasWarnings() || v.hasErrors() {
		header := []string{"Type", "Message"}
		connectData := [][]string{}
		for _, warning := range v.warnings {
			connectData = append(connectData, []string{utils.ColorWrap("Warning", color.FgYellow), warning})
		}
		for _, err := range v.errors {
			connectData = append(connectData, []string{utils.ColorWrap("Error", color.FgRed), err.Error()})
		}
		message.Table(header, connectData)
		message.Info(fmt.Sprintf("%d warnings and %d errors in %q",
			len(v.warnings), len(v.errors), v.typedZarfPackage.Metadata.Name))
	}
}

func (v Validator) hasWarnings() bool {
	return len(v.warnings) > 0
}

func (v Validator) hasErrors() bool {
	return len(v.errors) > 0
}

func (v *Validator) addWarning(message string) {
	v.warnings = append(v.warnings, message)
}

func (v *Validator) addError(err error) {
	v.errors = append(v.errors, err)
}
