// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/xeipuuv/gojsonschema"
)

// ZarfSchema is exported so main.go can embed the schema file
var ZarfSchema embed.FS

func getSchemaFile() ([]byte, error) {
	return ZarfSchema.ReadFile("zarf.schema.json")
}

// ValidateZarfSchema validates a zarf file against the zarf schema, returns *validator with warnings or errors if they exist
// along with an error if the validation itself failed
func ValidateZarfSchema(path string) (*Validator, error) {
	validator := Validator{}
	var err error
	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &validator.typedZarfPackage); err != nil {
		return nil, err
	}

	checkForVarInComponentImport(&validator)

	if validator.jsonSchema, err = getSchemaFile(); err != nil {
		return nil, err
	}

	if err := utils.ReadYaml(filepath.Join(path, layout.ZarfYAML), &validator.untypedZarfPackage); err != nil {
		return nil, err
	}

	if err = validateSchema(&validator); err != nil {
		return nil, err
	}

	return &validator, nil
}

func checkForVarInComponentImport(validator *Validator) {
	for i, component := range validator.typedZarfPackage.Components {
		if strings.Contains(component.Import.Path, types.ZarfPackageTemplatePrefix) {
			validator.addWarning(fmt.Sprintf(".component.[%d].import.path: Will not resolve ZARF_PKG_TMPL_* variables", i))
		}
		if strings.Contains(component.Import.URL, types.ZarfPackageTemplatePrefix) {
			validator.addWarning(fmt.Sprintf(".component.[%d].import.url: Will not resolve ZARF_PKG_TMPL_* variables", i))
		}
	}

}

func makeFieldYqEval(field string) string {
	if field == "(root)" {
		return field
	}
	// . is a non-word chacter (\b) so this gets digits between two .
	re := regexp.MustCompile(`\b\d+\b`)

	wrappedField := re.ReplaceAllStringFunc(field, func(match string) string {
		return "[" + match + "]"
	})

	return fmt.Sprintf(".%s", wrappedField)
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
			err := fmt.Errorf(
				"%s: %s", makeFieldYqEval(desc.Field()), desc.Description())
			validator.addError(err)
		}
	}

	return err
}
