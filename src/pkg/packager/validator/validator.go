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

	message.Success(fmt.Sprintf("Schema validation successful for %q", zarfTypedData.Metadata.Name))
	return nil
}

func checkForVarInComponentImport(zarfYaml types.ZarfPackage) error {
	var errorMessages []string
	for i, component := range zarfYaml.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			errorMessages = append(errorMessages, fmt.Sprintf("component.%d.import.path will not resolve ZARF_PKG_TMPL_* variables", i))
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			errorMessages = append(errorMessages, fmt.Sprintf("component.%d.import.url will not resolve ZARF_PKG_TMPL_* variables", i))
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("%s %s", zarfWarningPrefix, strings.Join(errorMessages, ", "))
	}

	return nil
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
