// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fatih/color"
)

type Category int

const (
	CategoryError   Category = 1
	CategoryWarning Category = 2
)

// ValidatorMessage is used to denote a finding
type ValidatorMessage struct {
	YqPath         string
	Description    string
	Item           string
	PackageRelPath string
	PackageName    string
	Category       Category
}

func (c Category) String() string {
	if c == CategoryError {
		return message.ColorWrap("Error", color.FgRed)
	} else if c == CategoryWarning {
		return message.ColorWrap("Warning", color.FgYellow)
	}
	return ""
}

func (vm ValidatorMessage) itemizedDescription() string {
	if vm.Item != "" {
		return fmt.Sprintf("%s - %s", vm.Description, vm.Item)
	}
	return vm.Description
}

// Validator holds the warnings/errors and messaging that we get from validation
type Validator struct {
	findings           []ValidatorMessage
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
	baseDir            string
}

// DisplayFormattedMessage message sent to user based on validator results
func (v Validator) DisplayFormattedMessage() {
	if !v.hasFindings() {
		message.Successf("0 findings for %q", v.typedZarfPackage.Metadata.Name)
	}
	v.printValidationTable()
}

func DisplayFormattedMessage(findings []ValidatorMessage) {
	v := Validator{}
	v.findings = findings
	if !v.hasFindings() {
		message.Successf("0 findings for %q", v.typedZarfPackage.Metadata.Name)
	}
	v.printValidationTable()
}

// IsSuccess returns true if there are not any errors
func (v Validator) IsSuccess() bool {
	for _, finding := range v.findings {
		if finding.Category == CategoryError {
			return false
		}
	}
	return true
}

func (v Validator) packageRelPathToUser(vm ValidatorMessage) string {
	if helpers.IsOCIURL(vm.PackageRelPath) {
		return vm.PackageRelPath
	}
	return filepath.Join(v.baseDir, vm.PackageRelPath)
}

func (v Validator) printValidationTable() {
	if !v.hasFindings() {
		return
	}

	mapOfFindingsByPath := make(map[string][]ValidatorMessage)
	for _, finding := range v.findings {
		mapOfFindingsByPath[finding.PackageRelPath] = append(mapOfFindingsByPath[finding.PackageRelPath], finding)
	}

	header := []string{"Type", "Path", "Message"}

	for packageRelPath, findings := range mapOfFindingsByPath {
		lintData := [][]string{}
		for _, finding := range findings {
			lintData = append(lintData, []string{finding.Category.String(), finding.getPath(), finding.itemizedDescription()})
		}
		message.Notef("Linting package %q at %s", findings[0].PackageName, v.packageRelPathToUser(findings[0]))
		message.Table(header, lintData)
		message.Info(v.getFormattedFindingCount(packageRelPath, findings[0].PackageName))
	}
}

func (v Validator) getFormattedFindingCount(relPath string, packageName string) string {
	warningCount := 0
	errorCount := 0
	for _, finding := range v.findings {
		if finding.PackageRelPath != relPath {
			continue
		}
		if finding.Category == CategoryWarning {
			warningCount++
		}
		if finding.Category == CategoryError {
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
		warningCount, wordWarning, errorCount, wordError, packageName)
}

func (vm ValidatorMessage) getPath() string {
	if vm.YqPath == "" {
		return ""
	}
	return message.ColorWrap(vm.YqPath, color.FgCyan)
}

func (v Validator) hasFindings() bool {
	return len(v.findings) > 0
}

func (v *Validator) addError(vMessage ValidatorMessage) {
	vMessage.Category = CategoryError
	v.findings = helpers.Unique(append(v.findings, vMessage))
}
