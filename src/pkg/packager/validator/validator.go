// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validator contains functions for verifying zarf yaml files are valid
package validator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

const (
	validatorInvalidPrefix = "schema is invalid:"
	validatorWarningPrefix = "zarf schema warning:"
)

type Validator struct {
	warnings []string
	errors   []string
}

func (v Validator) getFormmatedError() error {
	if len(v.errors) == 0 {
		return nil
	}
	errorMessage := validatorInvalidPrefix
	for _, errorStr := range v.errors {
		errorMessage = fmt.Sprintf("%s\n - %s", errorMessage, errorStr)
	}
	return errors.New(errorMessage)
}

func (v Validator) getFormmatedWarning() string {
	if len(v.warnings) == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s", validatorWarningPrefix, strings.Join(v.warnings, ", "))
}

// ValidateZarfSchema a zarf file against the zarf schema, returns an error if the file is invalid
func ValidateZarfSchema(path string) (err error) {
	// var zarfTypedData types.ZarfPackage
	// if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &zarfTypedData); err != nil {
	// 	return err
	// }

	// if err := checkForVarInComponentImport(zarfTypedData); err != nil {
	// 	message.Warn("")
	// }

	// zarfSchema, _ := config.GetSchemaFile()

	// var zarfData interface{}
	// if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &zarfData); err != nil {
	// 	return err
	// }
	// var validator Validator
	// if validator, err = validateSchema(nil,zarfData, zarfSchema); err != nil {
	// 	return err
	// }

	// message.Success(fmt.Sprintf("Schema validation successful for %q", zarfTypedData.Metadata.Name))
	return nil
}

func checkForVarInComponentImport(validator Validator, zarfPackage types.ZarfPackage) Validator {
	for i, component := range zarfPackage.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			validator.warnings = append(validator.warnings, fmt.Sprintf("component.%d.import.path will not resolve ZARF_PKG_TMPL_* variables", i))
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			validator.warnings = append(validator.warnings, fmt.Sprintf("component.%d.import.url will not resolve ZARF_PKG_TMPL_* variables", i))
		}
	}

	return validator
}

func validateSchema(validator Validator, unmarshalledYaml interface{}, jsonSchema []byte) (Validator, error) {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return validator, err
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			validator.errors = append(validator.errors, desc.String())
		}
	}

	return validator, err
}
