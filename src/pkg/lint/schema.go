// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"io/fs"
	"regexp"

	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// ZarfSchema is exported so main.go can embed the schema file
var ZarfSchema fs.ReadFileFS

// ValidatePackageSchema checks the Zarf package in the current directory against the Zarf schema
func ValidatePackageSchema(setVariables map[string]string) ([]PackageFinding, error) {
	var untypedZarfPackage interface{}
	if err := utils.ReadYaml(layout.ZarfYAML, &untypedZarfPackage); err != nil {
		return nil, err
	}

	jsonSchema, err := ZarfSchema.ReadFile("zarf.schema.json")
	if err != nil {
		return nil, err
	}

	untypedZarfPackage, err = templateRawYaml(untypedZarfPackage, setVariables)
	if err != nil {
		return nil, err
	}

	return getSchemaFindings(jsonSchema, untypedZarfPackage)
}

func templateRawYaml(untypedPackage any, values map[string]string) (any, error) {
	templateMap := map[string]string{}

	for key, value := range values {
		templateMap[fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageVariablePrefix, key)] = value
	}

	for key, value := range values {
		templateMap[fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, key)] = value
	}

	if err := utils.ReloadYamlTemplate(&untypedPackage, templateMap); err != nil {
		return nil, err
	}
	return untypedPackage, nil
}

func makeFieldPathYqCompat(field string) string {
	if field == "(root)" {
		return field
	}
	// \b is a metacharacter that will stop at the next non-word character (including .)
	// https://regex101.com/r/pIRPk0/1
	re := regexp.MustCompile(`(\b\d+\b)`)

	wrappedField := re.ReplaceAllString(field, "[$1]")

	return fmt.Sprintf(".%s", wrappedField)
}

func getSchemaFindings(jsonSchema []byte, obj interface{}) ([]PackageFinding, error) {
	var findings []PackageFinding
	schemaErrors, err := runSchema(jsonSchema, obj)
	if err != nil {
		return nil, err
	}

	for _, schemaErr := range schemaErrors {
		findings = append(findings, PackageFinding{
			YqPath:      makeFieldPathYqCompat(schemaErr.Field()),
			Description: schemaErr.Description(),
			Severity:    SevErr,
		})
	}

	return findings, nil
}

func runSchema(jsonSchema []byte, pkg interface{}) ([]gojsonschema.ResultError, error) {
	schemaLoader := gojsonschema.NewBytesLoader(jsonSchema)
	documentLoader := gojsonschema.NewGoLoader(pkg)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, err
	}

	if !result.Valid() {
		return result.Errors(), nil
	}
	return nil, nil
}
