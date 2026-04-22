// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"regexp"

	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/schema"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// ValidatePackageSchemaAtPath checks the Zarf package against the Zarf schema
// If path is a directory, it will look for layout.ZarfYAML within it.
// If path is a file, it will use that file directly.
func ValidatePackageSchemaAtPath(path string, setVariables map[string]string) ([]PackageFinding, error) {
	var untypedZarfPackage interface{}

	pkgPath, err := layout.ResolvePackagePath(path)
	if err != nil {
		return nil, fmt.Errorf("unable to access path %q: %w", path, err)
	}

	if err := utils.ReadYaml(pkgPath.ManifestFile, &untypedZarfPackage); err != nil {
		return nil, err
	}
	jsonSchema := schema.GetV1Alpha1Schema()
	if err := templateZarfObj(&untypedZarfPackage, setVariables); err != nil {
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

func templateZarfObj(zarfObj any, setVariables map[string]string) error {
	templateMap := map[string]string{}

	setVars := func(templatePrefix string) error {
		for key, value := range setVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}
		return nil
	}

	if err := setVars(v1alpha1.ZarfPackageTemplatePrefix); err != nil {
		return err
	}

	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := setVars(v1alpha1.ZarfPackageVariablePrefix); err != nil {
		return err
	}

	return utils.ReloadYamlTemplate(zarfObj, templateMap)
}
