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

type validatorMessage struct {
	yqPath      string
	filePath    string
	description string
	item        string
}

func (v validatorMessage) String() string {
	if v.filePath != "" {
		v.filePath = fmt.Sprintf(" %s", v.filePath)
	}
	if v.item != "" {
		v.item = fmt.Sprintf(" %s", v.item)
	}
	if v.filePath == "" && v.yqPath == "" && v.item == "" {
		return v.description
	}
	return fmt.Sprintf("%s%s: %s%s",
		utils.ColorWrap(v.yqPath, color.FgCyan), utils.ColorWrap(v.filePath, color.FgCyan),
		v.description, v.item)
}

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	warnings           []validatorMessage
	errors             []validatorMessage
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
	hasUnSetVarWarning bool
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasWarnings() && !v.hasErrors() {
		message.Successf("0 findings for %q", v.typedZarfPackage.Metadata.Name)
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
			connectData = append(connectData, []string{utils.ColorWrap("Warning", color.FgYellow), warning.String()})
		}
		for _, err := range v.errors {
			connectData = append(connectData, []string{utils.ColorWrap("Error", color.FgRed), err.String()})
		}
		message.Table(header, connectData)
		message.Info(v.getWarningAndErrorCount())
	}
}

func (v Validator) getWarningAndErrorCount() string {
	wordWarning := "warnings"
	if len(v.warnings) == 1 {
		wordWarning = "warning"
	}
	wordError := "errors"
	if len(v.errors) == 1 {
		wordError = "error"
	}
	return fmt.Sprintf("%d %s and %d %s in %q",
		len(v.warnings), wordWarning, len(v.errors), wordError, v.typedZarfPackage.Metadata.Name)
}

func (v Validator) hasWarnings() bool {
	return len(v.warnings) > 0
}

func (v Validator) hasErrors() bool {
	return len(v.errors) > 0
}

func (v *Validator) addWarning(vmessage validatorMessage) {
	v.warnings = append(v.warnings, vmessage)
}

func (v *Validator) addError(vMessage validatorMessage) {
	v.errors = append(v.errors, vMessage)
}
