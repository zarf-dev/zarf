// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// ZarfSchema is exported so main.go can embed the schema file
var ZarfSchema fs.ReadFileFS

// ValidatePackageSchemaAtPath checks the Zarf package in the current directory against the Zarf schema
func ValidatePackageSchemaAtPath(path string, setVariables map[string]string) ([]PackageFinding, error) {
	var untypedZarfPackage interface{}
	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &untypedZarfPackage); err != nil {
		return nil, err
	}
	jsonSchema, err := ZarfSchema.ReadFile("zarf.schema.json")
	if err != nil {
		return nil, err
	}
	_, err = templateZarfObj(&untypedZarfPackage, setVariables)
	if err != nil {
		return nil, err
	}
	return getSchemaFindings(jsonSchema, untypedZarfPackage)
}

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
	_, err = templateZarfObj(&untypedZarfPackage, setVariables)
	if err != nil {
		return nil, err
	}
	return getSchemaFindings(jsonSchema, untypedZarfPackage)
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

func templateZarfObj(zarfObj any, setVariables map[string]string) ([]PackageFinding, error) {
	var findings []PackageFinding
	templateMap := map[string]string{}

	setVarsAndWarn := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(zarfObj, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				findings = append(findings, PackageFinding{
					Description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key),
					Severity:    SevWarn,
				})
			}
			if _, present := setVariables[key]; !present {
				findings = append(findings, PackageFinding{
					Description: fmt.Sprintf("package template %s is not set and won't be evaluated during lint", key),
					Severity:    SevWarn,
				})
			}
		}
		for key, value := range setVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
		return nil
	}

	if err := setVarsAndWarn(v1alpha1.ZarfPackageTemplatePrefix, false); err != nil {
		return nil, err
	}

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := setVarsAndWarn(v1alpha1.ZarfPackageVariablePrefix, true); err != nil {
		return nil, err
	}

	if err := utils.ReloadYamlTemplate(zarfObj, templateMap); err != nil {
		return nil, err
	}
	return findings, nil
}
