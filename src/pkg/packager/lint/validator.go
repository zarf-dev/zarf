// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fatih/color"
	"github.com/pterm/pterm"
)

type validationType int

const (
	validationError   validationType = 1
	validationWarning validationType = 2
)

type packageKey struct {
	path string
	name string
}

type validatorMessage struct {
	yqPath         string
	description    string
	item           string
	packageKey     packageKey
	validationType validationType
}

func (vt validationType) String() string {
	if vt == validationError {
		return utils.ColorWrap("Error", color.FgRed)
	} else if vt == validationWarning {
		return utils.ColorWrap("Warning", color.FgYellow)
	}
	return ""
}

func (vm validatorMessage) String() string {
	if vm.item != "" {
		vm.item = fmt.Sprintf(" - %s", vm.item)
	}
	return fmt.Sprintf("%s%s", vm.description, vm.item)
}

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	findings           []validatorMessage
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasFindings() {
		message.Successf("0 findings for %q", v.typedZarfPackage.Metadata.Name)
	}
	v.printValidationTable()
}

// IsSuccess returns true if there are not any errors
func (v Validator) IsSuccess() bool {
	for _, finding := range v.findings {
		if finding.validationType == validationError {
			return false
		}
	}
	return true
}

func (v Validator) printValidationTable() {
	if !v.hasFindings() {
		return
	}

	packageKeys := helpers.Unique(v.getUniquePackageKeys())
	connectData := make(map[packageKey][][]string)

	for _, finding := range v.findings {
		connectData[finding.packageKey] = append(connectData[finding.packageKey],
			[]string{finding.validationType.String(), finding.getPath(), finding.String()})
	}

	header := []string{"Type", "Path", "Message"}
	for _, packageKey := range packageKeys {
		//We should probably move this println into info
		pterm.Println()
		if packageKey.path != "" {
			message.Infof("Linting package %q at %s", packageKey.name, packageKey.path)
		} else {
			message.Infof("Linting package %q", packageKey.name)
		}

		message.Table(header, connectData[packageKey])
		message.Info(v.getFormattedFindingCount(packageKey))
	}
}

func (v Validator) getUniquePackageKeys() []packageKey {
	uniqueKeys := make(map[packageKey]bool)
	var pks []packageKey

	for _, finding := range v.findings {
		if _, exists := uniqueKeys[finding.packageKey]; !exists {
			uniqueKeys[finding.packageKey] = true
			pks = append(pks, finding.packageKey)
		}
	}

	return pks
}

func (v Validator) getFormattedFindingCount(pk packageKey) string {
	warningCount := 0
	errorCount := 0
	for _, finding := range v.findings {
		if finding.packageKey != pk {
			continue
		}
		if finding.validationType == validationWarning {
			warningCount++
		}
		if finding.validationType == validationError {
			errorCount++
		}
	}
	wordWarning := "warnings"
	if warningCount == 1 {
		wordWarning = "warning"
	}
	wordError := "errors"
	if errorCount == 1 {
		wordError = "error"
	}
	return fmt.Sprintf("%d %s and %d %s in %q",
		warningCount, wordWarning, errorCount, wordError, pk.name)
}

func (vm validatorMessage) getPath() string {
	if vm.yqPath == "" {
		return ""
	}
	return utils.ColorWrap(vm.yqPath, color.FgCyan)
}

func (v Validator) hasFindings() bool {
	return len(v.findings) > 0
}

func (v *Validator) addWarning(vmessage validatorMessage) {
	vmessage.validationType = validationWarning
	v.findings = helpers.Unique(append(v.findings, vmessage))
}

func (v *Validator) addError(vMessage validatorMessage) {
	vMessage.validationType = validationError
	v.findings = helpers.Unique(append(v.findings, vMessage))
}
