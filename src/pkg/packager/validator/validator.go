// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validator contains functions for verifying zarf yaml files are valid
package validator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

const (
	validatorInvalidPrefix = "schema is invalid:"
	validatorWarningPrefix = "zarf schema warning:"
)

// Validator is used by other packages to check their zarf packages for errors/warnings
type Validator struct {
	warnings           []string
	errors             []string
	jsonSchema         []byte
	typedZarfPackage   types.ZarfPackage
	untypedZarfPackage interface{}
}

func (v Validator) getFormatedError() error {
	if !v.hasErrors() {
		return nil
	}
	errorMessage := validatorInvalidPrefix
	for _, errorStr := range v.errors {
		errorMessage = fmt.Sprintf("%s\n - %s", errorMessage, errorStr)
	}
	return errors.New(errorMessage)
}

func (v Validator) getFormatedWarning() string {
	if !v.hasWarnings() {
		return ""
	}
	return fmt.Sprintf("%s %s", validatorWarningPrefix, strings.Join(v.warnings, ", "))
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

func (v Validator) DisplayFormattedMessage() {
	if v.hasWarnings() {
		message.Warn(v.getFormatedWarning())
	}
	if v.hasErrors() {
		message.Fatal(v.getFormatedError(), v.getFormatedError().Error())
	}
	if v.isSuccess() {
		message.Success(v.getFormatedSuccess())
	}
}

// ValidateZarfSchema validates a zarf file against the zarf schema, returns a validator with warnings or errors if they exist
func ValidateZarfSchema(path string) (Validator, error) {
	validator := Validator{}
	var err error
	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &validator.typedZarfPackage); err != nil {
		return validator, err
	}

	checkForVarInComponentImport(&validator)

	if validator.jsonSchema, err = config.GetSchemaFile(); err != nil {
		return validator, err
	}

	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &validator.untypedZarfPackage); err != nil {
		return validator, err
	}

	if err = validateSchema(&validator); err != nil {
		return validator, err
	}

	return validator, nil
}

func checkForVarInComponentImport(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			validator.warnings = append(validator.warnings, fmt.Sprintf("component.%d.import.path will not resolve ZARF_PKG_TMPL_* variables", i))
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			validator.warnings = append(validator.warnings, fmt.Sprintf("component.%d.import.url will not resolve ZARF_PKG_TMPL_* variables", i))
		}
	}

}

func validateSchema(validator *Validator) error {
	schemaLoader := gojsonschema.NewBytesLoader(validator.jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(validator.untypedZarfPackage)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			validator.errors = append(validator.errors, desc.String())
		}
	}

	return err
}
