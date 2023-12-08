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
	// if v.filePath != "" {
	// 	v.filePath = fmt.Sprintf(" %s", v.filePath)
	// }
	if v.item != "" {
		v.item = fmt.Sprintf(" - %s", v.item)
	}
	if v.filePath == "" && v.yqPath == "" && v.item == "" {
		return v.description
	}
	return fmt.Sprintf("%s%s", v.description, v.item)
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
	v.printValidationTable2()
}

// IsSuccess returns true if there are not any errors
func (v Validator) IsSuccess() bool {
	return !v.hasErrors()
}

func (v Validator) printValidationTable() {
	if v.hasWarnings() || v.hasErrors() {
		header := []string{"Type", "Path", "Message"}
		connectData := [][]string{}
		for _, warning := range v.warnings {
			connectData = append(connectData,
				[]string{utils.ColorWrap("Warning", color.FgYellow), warning.getPath(), warning.String()})
		}
		for _, validatorError := range v.errors {
			connectData = append(connectData,
				[]string{utils.ColorWrap("Error", color.FgRed), validatorError.getPath(), validatorError.String()})
		}
		message.Table(header, connectData)
		message.Info(v.getWarningAndErrorCount())
	}
}

func (v Validator) printValidationTable2() {
	differentPaths := v.getUniquePaths()
	if v.hasWarnings() || v.hasErrors() {
		for _, path := range differentPaths {
			header := []string{"Type", "Path", "Message"}
			connectData := make(map[string][][]string)
			item := path
			for _, warning := range v.warnings {
				if warning.filePath == path {
					if item == "" {
						item = "original"
					}
					connectData[item] = append(connectData[item],
						[]string{utils.ColorWrap("Warning", color.FgYellow), warning.getPath2(), warning.String()})
				}
			}
			for _, validatorError := range v.errors {
				if validatorError.filePath == path {
					if item == "" {
						item = "original"
					}
					connectData[item] = append(connectData[item],
						[]string{utils.ColorWrap("Error", color.FgRed), validatorError.getPath2(), validatorError.String()})
				}
			}

			message.Infof("Component at path: %s", item)
			message.Table(header, connectData[item])
			//message.Info(v.getWarningAndErrorCount())
		}
	}
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func (v Validator) getUniquePaths() []string {
	paths := []string{}
	for _, warning := range v.warnings {
		if !contains(paths, warning.filePath) {
			paths = append(paths, warning.filePath)
		}
	}
	for _, validatorError := range v.errors {
		if !contains(paths, validatorError.filePath) {
			paths = append(paths, validatorError.filePath)
		}
	}
	return paths
}

func (vm validatorMessage) getPath() string {
	if vm.yqPath == "" {
		return ""
	}
	if vm.filePath != "" {
		return utils.ColorWrap(fmt.Sprintf("%s %s", vm.yqPath, vm.filePath), color.FgCyan)
	}
	return utils.ColorWrap(vm.yqPath, color.FgCyan)
}

func (vm validatorMessage) getPath2() string {
	if vm.yqPath == "" {
		return ""
	}
	// if vm.filePath != "" {
	// 	return utils.ColorWrap(fmt.Sprintf("%s %s", vm.yqPath, vm.filePath), color.FgCyan)
	// }
	return utils.ColorWrap(vm.yqPath, color.FgCyan)
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
