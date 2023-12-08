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

type ValidationType int

const (
	validationError   ValidationType = 1
	validationWarning ValidationType = 2
)

type validatorMessage struct {
	yqPath         string
	filePath       string
	description    string
	item           string
	packageName    string
	validationType ValidationType
}

func (vt ValidationType) String() string {
	if vt == validationError {
		return utils.ColorWrap("Error", color.FgRed)
	} else if vt == validationWarning {
		return utils.ColorWrap("Warning", color.FgYellow)
	} else {
		return ""
	}
}

func (v validatorMessage) String() string {
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
	findings           []validatorMessage
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
	hasUnSetVarWarning bool
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasFindings() {
		message.Successf("0 findings for %q", v.typedZarfPackage.Metadata.Name)
	}
	v.printValidationTable2()
}

// IsSuccess returns true if there are not any errors
func (v Validator) IsSuccess() bool {
	return !v.hasFindings()
}

// func (v Validator) printValidationTable() {
// 	if v.hasWarnings() || v.hasErrors() {
// 		header := []string{"Type", "Path", "Message"}
// 		connectData := [][]string{}
// 		for _, warning := range v.findings {
// 			connectData = append(connectData,
// 				[]string{utils.ColorWrap("Warning", color.FgYellow), warning.getPath(), warning.String()})
// 		}
// 		for _, validatorError := range v.errors {
// 			connectData = append(connectData,
// 				[]string{utils.ColorWrap("Error", color.FgRed), validatorError.getPath(), validatorError.String()})
// 		}
// 		message.Table(header, connectData)
// 		message.Info(v.getWarningAndErrorCount())
// 	}
// }

func (v Validator) printValidationTable2() {
	differentPaths := v.getUniquePaths()
	if v.hasFindings() {
		for _, path := range differentPaths {
			header := []string{"Type", "Path", "Message"}
			connectData := make(map[string][][]string)
			item := path
			for _, finding := range v.findings {
				if finding.filePath == path {
					if item == "" {
						item = "original"
					}
					connectData[item] = append(connectData[item],
						[]string{utils.ColorWrap(finding.validationType.String(), color.FgYellow),
							finding.getPath(), finding.String()})
				}
			}
			// for _, validatorError := range v.errors {
			// 	if validatorError.filePath == path {
			// 		if item == "" {
			// 			item = "original"
			// 		}
			// 		connectData[item] = append(connectData[item],
			// 			[]string{utils.ColorWrap("Error", color.FgRed), validatorError.getPath(), validatorError.String()})
			// 	}
			// }

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
	for _, warning := range v.findings {
		if !contains(paths, warning.filePath) {
			paths = append(paths, warning.filePath)
		}
	}
	return paths
}

func (vm validatorMessage) getPath() string {
	if vm.yqPath == "" {
		return ""
	}
	return utils.ColorWrap(vm.yqPath, color.FgCyan)
}

// func (v Validator) getWarningAndErrorCount() string {
// 	wordWarning := "warnings"
// 	if len(v.findings) == 1 {
// 		wordWarning = "warning"
// 	}
// 	wordError := "errors"
// 	if len(v.errors) == 1 {
// 		wordError = "error"
// 	}
// 	return fmt.Sprintf("%d %s and %d %s in %q",
// 		len(v.findings), wordWarning, len(v.errors), wordError, v.typedZarfPackage.Metadata.Name)
// }

func (v Validator) hasFindings() bool {
	return len(v.findings) > 0
}

func (v *Validator) addWarning(vmessage validatorMessage) {
	vmessage.validationType = validationWarning
	v.findings = append(v.findings, vmessage)
}

func (v *Validator) addError(vMessage validatorMessage) {
	vMessage.validationType = validationError
	v.findings = append(v.findings, vMessage)
}
