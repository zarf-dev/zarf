// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validator contains functions for linting the zarf.yaml
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
	zarfInvalidPrefix = "schema is invalid:"
	zarfWarningPrefix = "zarf schema warning:"
)

// ValidateZarfSchema a zarf file against the zarf schema, returns an error if the file is invalid
func ValidateZarfSchema(path string) (err error) {
	var zarfTypedData types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &zarfTypedData); err != nil {
		return err
	}

	if err := checkForVarInComponentImport(zarfTypedData); err != nil {
		message.Warn(err.Error())
	}

	zarfSchema, _ := config.GetSchemaFile()

	var zarfData interface{}
	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &zarfData); err != nil {
		return err
	}

	if err = validateSchema(zarfData, zarfSchema); err != nil {
		return err
	}

	message.Success("Validation successful")
	return nil
}

func checkForVarInComponentImport(zarfYaml types.ZarfPackage) error {
	valid := true
	errorMessage := zarfWarningPrefix
	componentWarningStart := "component."
	for i, component := range zarfYaml.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			errorMessage = fmt.Sprintf("%s %s%d.import.path will not resolve ZARF_PKG_TMPL_* variables.",
				errorMessage, componentWarningStart, i)
			valid = false
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			errorMessage = fmt.Sprintf("%s %s%d.import.url will not resolve ZARF_PKG_TMPL_* variables.",
				errorMessage, componentWarningStart, i)
			valid = false
		}
	}
	if valid {
		return nil
	}
	return errors.New(errorMessage)
}

func validateSchema(unmarshalledYaml interface{}, jsonSchema []byte) error {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(unmarshalledYaml)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		errorMessage := zarfInvalidPrefix
		for _, desc := range result.Errors() {
			errorMessage = fmt.Sprintf("%s\n - %s", errorMessage, desc.String())
		}
		err = errors.New(errorMessage)
	}

	return err
}
