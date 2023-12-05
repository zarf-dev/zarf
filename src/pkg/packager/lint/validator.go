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

type ValidatorMessage struct {
	yqPath      string
	filePath    string
	description string
	item        string
}

func (v ValidatorMessage) String() string {
	if v.filePath != "" {
		v.filePath = fmt.Sprintf(" %s", v.filePath)
	}
	if v.item != "" {
		v.item = fmt.Sprintf(" %s", v.item)
	}
	return fmt.Sprintf("%s%s: %s%s",
		v.yqPath, v.filePath, v.description, v.item)
}

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	warnings           []string
	warnings2          []ValidatorMessage
	errors             []error
	errors2            []ValidatorMessage
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasWarnings() && !v.hasErrors() {
		message.Successf("Schema validation successful for %q", v.typedZarfPackage.Metadata.Name)
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
		for _, warning := range v.warnings2 {
			connectData = append(connectData, []string{utils.ColorWrap("Warning", color.FgYellow), warning.String()})
		}
		for _, err := range v.errors2 {
			connectData = append(connectData, []string{utils.ColorWrap("Error", color.FgRed), err.String()})
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

func (v *Validator) addWarning2(vmessage ValidatorMessage) {
	v.warnings2 = append(v.warnings2, vmessage)
}

func (v *Validator) addError(err error) {
	v.errors = append(v.errors, err)
}

func (v *Validator) addError2(vMessage ValidatorMessage) {
	v.errors2 = append(v.errors2, vMessage)
}
